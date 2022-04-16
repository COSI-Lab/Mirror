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

func NewGeoIPHandler(licenseKey string) *GeoIPHandler {
	g := &GeoIPHandler{licenseKey: licenseKey}

	// Load the database from disk
	db, err := geoip2.Open("GeoLite2-City.mmdb")
	if err != nil {
		logging.Warn("Error opening database:", err)
	} else {
		logging.Success("Loaded GeoIP database from disk")
	}
	g.db = db

	if g.licenseKey == "" {
		if g.db == nil {
			logging.Warn("No license key found and no database was found on disk. GeoIP will not be available")
		} else {
			logging.Warn("No license key found. GeoIP database will never be updated")
		}
	} else {
		// update the database every 24 hours
		go func() {
			for {
				g.updateDatabase()
				time.Sleep(24 * time.Hour)
			}
		}()
	}

	return g
}

func (g *GeoIPHandler) GetGeoIP(ip net.IP) (city *geoip2.City) {
	g.RLock()

	defer func() {
		g.RUnlock()
		if err := recover(); err != nil {
			logging.Warn("GeoIP caused panic while looking up:", ip)
		}
	}()

	if g.db == nil {
		return nil
	}

	city, err := g.db.City(ip)
	if err != nil {
		return nil
	}

	return city
}

// handleDatabases checks for new databases and downloads them
func (g *GeoIPHandler) updateDatabase() {
	if g.licenseKey == "" {
		logging.Warn("No license key provided, skipping database update")
		return
	}

	logging.Info("Checking for new MaxMind GeoLite2-City database")

	if !g.checkForNewDatabase() {
		logging.Info("No new GeoIP database found")
	} else {
		start := time.Now()
		db, err := g.downloadNewDatabase()

		if err != nil {
			logging.Error("Error while downloading new GeoIP database:", err)
		} else {
			logging.Success("Found new GeoIP database and downloaded it in", time.Since(start))

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
			// write the file to disk (open in write mode)
			f, err := os.OpenFile("GeoLite2-City.mmdb", os.O_WRONLY|os.O_CREATE, 0644)

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

			f.Close()

			// TODO: instead opening the database from disk just load the byte slice instead
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
