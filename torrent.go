package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/COSI-Lab/logging"
	"github.com/gocolly/colly"
)

// On startup, and then every day at midnight scrape torrents from upstreams and
// save to files to outdir. The purpose is to seed commonly used torrents
func ScheduleTorrents(torrents []*Torrent, outdir string) {
	scrapeTorrents(torrents, outdir)

	// Sleep until midnight
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.Local)
	time.Sleep(time.Until(midnight))
	scrapeTorrents(torrents, outdir)

	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		scrapeTorrents(torrents, outdir)
	}
}

func scrapeTorrents(torrents []*Torrent, outdir string) {
	logging.Info("Scraping torrents")

	// create outdir if it doesn't exist
	err := os.MkdirAll(outdir, os.ModePerm)
	if err != nil {
		logging.Error("Failed to create scrapeTorrents outdir: ", err)
	}

	for _, torrent := range torrents {
		go scrape(torrent.Depth, torrent.Delay, torrent.Url, outdir)
	}
}

// Visits url and downloads all torrents to outdir to a certian depth
//
// Torrents with a name that already exists in outdir are skipped if
// the upstream file has the same file size as the one on disk
func scrape(depth, delay int, url, outdir string) {
	// Instantiate default collector
	c := colly.NewCollector(
		// MaxDepth is 1, so only the links on the scraped page
		// is visited, and no further links are followed
		colly.MaxDepth(depth + 1),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       time.Duration(delay) * time.Second,
	})

	// On every a element which has href attribute call callback
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")

		// Visit link found on page
		e.Request.Visit(link)
	})

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		pos := strings.LastIndex(r.URL.Path, ".")
		if pos != -1 && r.URL.Path[pos+1:len(r.URL.Path)] == "torrent" {
			// Check if we already have this file by name
			name := path.Base(r.URL.Path)
			file := outdir + "/" + name
			_, err := os.Stat(file)

			if err != nil {
				if os.IsNotExist(err) {
					// Download
					download(r, file)
				} else {
					// Unrecoverable error
					logging.Warn(err)
				}
			}
		} else {
			logging.Info("Visiting", r.URL.String())
		}
	})

	c.Visit(url)
}

// Downloads the file at `r` and saves it to `target` on disk
func download(r *colly.Request, target string) error {
	// Save this file to ourdir
	fmt.Println("GET", r.URL)

	res, err := http.Get(r.URL.String())
	if err != nil {
		return err
	}

	f, err := os.Create(target)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, res.Body)
	if err != nil {
		return err
	}

	return res.Body.Close()
}
