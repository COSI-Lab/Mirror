package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/oschwald/geoip2-golang"
)

const MAX_MIND_URL string = "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&suffix=tar.gz&license_key="

type GeoIPHandler struct {
	sync.RWMutex
	db         *geoip2.Reader
	licenseKey string
}

func NewGeoIPHandler(licenseKey string) (*GeoIPHandler, error) {
	g := &GeoIPHandler{
		licenseKey: licenseKey,
	}

	// Load the database from disk or download it
	db, err := geoip2.Open("GeoLite2-City.mmdb")
	if err != nil {
		db, err = g.downloadNewDatabase()

		if err != nil {
			return nil, err
		}
	}

	g.db = db

	// update the database every 24 hours
	g.updateDatabase()
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			g.updateDatabase()
		}
	}()

	return g, nil
}

func (g *GeoIPHandler) GetGeoIP(ip net.IP) (city *geoip2.City, err error) {
	g.RLock()
	if g.db == nil {
		return nil, nil
	}

	city, err = g.db.City(ip)
	g.RUnlock()

	return city, err
}

// handleDatabases checks for new databases and downloads them
func (g *GeoIPHandler) updateDatabase() {
	if g.licenseKey == "" {
		logging.Warn("No license key provided, skipping database update")
		return
	}

	logging.Info("Checking for new MaxMind GeoLite2-City database")

	if !g.checkForNewDatabase() {
		logging.Info("No new database found")
	} else {
		start := time.Now()
		db, err := g.downloadNewDatabase()

		if err != nil {
			logging.Error("Error while downloading new database:", err)
		} else {
			logging.Info("Found new database and downloaded it in", time.Since(start))

			// Update the database
			g.Lock()
			g.db = db
			g.Unlock()
		}
	}
}

func (g *GeoIPHandler) checkForNewDatabase() bool {
	// Make a HEAD request to MaxMind
	url, err := url.Parse(MAX_MIND_URL + g.licenseKey)

	if err != nil {
		return false
	}

	req := http.Request{Method: http.MethodHead, URL: url}
	resp, err := http.DefaultClient.Do(&req)

	if err != nil {
		logging.Error("Error checking for new database:", err)
		return false
	}

	// Get last modified header
	lastModified := resp.Header.Get("Last-Modified")

	// Load the last modified date from the file
	lastModifiedFile, err := os.Open("last_modified")

	if err != nil {
		// create the file
		lastModifiedFile, err = os.Create("last_modified")

		if err != nil {
			logging.Error("Error creating last_modified file:", err)
			return false
		}
	}

	defer lastModifiedFile.Close()

	// Read the last modified date from the file
	lastModifiedFileBytes, err := io.ReadAll(lastModifiedFile)

	if err != nil {
		logging.Error("Error reading last_modified file:", err)
		return false
	}

	return lastModified != string(lastModifiedFileBytes)
}

func (g *GeoIPHandler) downloadNewDatabase() (db *geoip2.Reader, err error) {
	// Download the database from maxmind
	url, err := url.Parse(MAX_MIND_URL + g.licenseKey)

	if err != nil {
		return nil, err
	}

	req := http.Request{Method: http.MethodGet, URL: url}
	resp, err := http.DefaultClient.Do(&req)

	if err != nil {
		return nil, err
	}

	// Extract the tarball
	gzr, err := gzip.NewReader(resp.Body)

	if err != nil {
		return nil, err
	}

	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()

		if err != nil {
			log.Println("Error reading tarball:", err)
			return nil, err
		}

		if strings.Split(header.Name, "/")[1] == "GeoLite2-City.mmdb" {
			// write the file to disk
			f, err := os.Create("GeoLite2-City.mmdb")

			if err != nil {
				return nil, err
			}

			defer f.Close()

			// write the file to byte slice
			b := make([]byte, header.Size)
			_, err = io.ReadFull(tr, b)

			if err != nil {
				return nil, err
			}

			// write the byte slice to the file
			_, err = f.Write(b)

			if err != nil {
				return nil, err
			}

			// load the byte slice as the database
			db, err = geoip2.Open("GeoLite2-City.mmdb")

			if err != nil {
				return nil, err
			}

			break
		}
	}

	// Write the last modified date to the file
	lastModifiedFile, err := os.Create("last_modified")
	if err != nil {
		return nil, err
	}
	_, err = lastModifiedFile.Write([]byte(resp.Header.Get("Last-Modified")))
	defer lastModifiedFile.Close()

	if err != nil {
		return nil, err
	}

	return db, nil
}
