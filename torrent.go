package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/COSI-Lab/logging"
	"github.com/gocolly/colly"
)

// HandleTorrents periodically downloads remote torrents and extracts torrents from disk
func HandleTorrents(config *ConfigFile, torrentDir, downloadDir string) {
	err := os.MkdirAll(downloadDir, 0755)
	if err != nil {
		logging.Error("Failed to create torrents downloadDir: ", err)
		return
	}

	err = os.MkdirAll(torrentDir, 0755)
	if err != nil {
		logging.Error("Failed to create torrents torrentDir: ", err)
		return
	}

	// On startup, and then every day at midnight
	// - scrape torrents from upstreams (such as linuxmint)
	// - search disk for torrent files and corresponding downloads
	// - sync downloadDir
	// - sync torrentDir
	go scrapeTorrents(config.Torrents, torrentDir)
	go syncTorrents(config, torrentDir, downloadDir)

	// Sleep until midnight
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.Local)
	time.Sleep(time.Until(midnight))
	go scrapeTorrents(config.Torrents, torrentDir)
	go syncTorrents(config, torrentDir, downloadDir)

	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		go scrapeTorrents(config.Torrents, torrentDir)
		go syncTorrents(config, torrentDir, downloadDir)
	}
}

// syncTorrents goes over all projects, finds their torrent files, the corresponding source
// files and then creates hardlinks in the download and torrent directories
func syncTorrents(config *ConfigFile, torrentDir, ourDir string) {
	for _, project := range config.GetProjects() {
		if len(project.Torrents) > 0 {
			logging.Info("Syncing " + project.Name + " torrents")
		}

		for _, searchPath := range project.Torrents {
			go func(searchPath string) {
				cmd := exec.Command("find", searchPath)
				out, err := cmd.Output()

				if err != nil {
					logging.Error("Failed to find torrent files: ", err)
					return
				}

				for _, torrentPath := range strings.Split(string(out), "\n") {
					filepath := strings.TrimSuffix(torrentPath, ".torrent")

					torrentName := path.Base(torrentPath)
					fileName := path.Base(filepath)

					_, err := os.Stat(downloadDir + "/" + fileName)
					if err != nil {
						if os.IsNotExist(err) {
							// Create a hardlink
							err = os.Link(torrentPath, downloadDir+"/"+torrentName)
							if err != nil {
								logging.Warn("Failed to create hardlink: ", err)
								continue
							}
						} else {
							logging.Error("Failed to stat a torrented file: ", err)
						}
					}

					_, err = os.Stat(torrentDir + "/" + torrentName)
					if err != nil {
						if os.IsNotExist(err) {
							// Create a hardlink
							err = os.Link(torrentPath, torrentDir+"/"+torrentName)
							if err != nil {
								logging.Warn("Failed to create hardlink: ", err)
								continue
							}
						} else {
							logging.Error("Failed to stat a torrent file: ", err)
						}
					}
				}
			}(searchPath)
		}
	}
}

// scrapeTorrents downloads all torrents from upstreams
func scrapeTorrents(torrents []*Torrent, downloadDir string) {
	for _, upstream := range torrents {
		go scrape(upstream.Depth, upstream.Delay, upstream.Url, downloadDir)
	}
}

// Visits a url and downloads all torrents to outdir
//
// Torrents with a name that already exists are skipped if
// the upstream file has the same file size as the one on disk
func scrape(depth, delay int, url, outdir string) {
	logging.Info("Scraping " + url)

	// Instantiate default collector
	c := colly.NewCollector(
		// MaxDepth is 1, so only the links on the scraped page
		// is visited, and no further links are followed
		colly.MaxDepth(depth + 1),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob: "*",
		Delay:      time.Duration(delay) * time.Second,
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
		}
	})

	c.Visit(url)
	logging.Success("Finished scraping " + url)
}

// Downloads the file at `r` and saves it to `target` on disk
func download(r *colly.Request, target string) error {
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
