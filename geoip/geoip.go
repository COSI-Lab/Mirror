package geoip

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/IncSW/geoip2"
)

// GeoIPHandler will keep itself up-to-date with the latest GeoIP database by updating once every 24 hours
// Provides methods to lookup IP addresses and return the associated latitude and longitude
// The structure should be created with it's maxmind license key
type GeoIPHandler struct {
	sync.RWMutex

	// The stop channel is used to end the goroutine that updates the database
	stop chan struct{}
	// Maxmind license key can be created at https://www.maxmind.com
	licenseKey string
	// underlying database object
	db         *geoip2.CityReader
}

// NewGeoIPHandler creates a new GeoIPHandler with the given license key
func NewGeoIPHandler(licenseKey string) (*GeoIPHandler, error) {
	// Download the database
	bytes, err := downloadAndCheckHash(licenseKey)
	if err != nil {
		return nil, err
	}

	// Create the database
	db, err := geoip2.NewCityReader(bytes)
	if err != nil {
		return nil, err
	}

	// Create the handler
	handler := &GeoIPHandler{
		stop: 	 make(chan struct{}),
		licenseKey: licenseKey,
		db:         db,
	}

	// update the database every 24 hours
	go handler.update(handler.stop)

	return handler, nil
}

func (g *GeoIPHandler) update(stop chan struct{}) {
	// update the database every 24 hours
	for {
		select {
		case <-stop:
			return
		case <-time.After(24 * time.Hour):
			// Lock the database
			g.Lock()

			// Download the database
			bytes, err := downloadAndCheckHash(g.licenseKey)
			if err != nil {
				fmt.Println(err)
				g.Unlock()
				continue
			}

			// Create the database
			db, err := geoip2.NewCityReader(bytes)

			if err != nil {
				fmt.Println(err)
				g.Unlock()
				continue
			}

			// Create the handler
			g.db = db

			g.Unlock()
		}
	}
}

func (g *GeoIPHandler) Close() {
	g.stop <- struct{}{}
}

func (g *GeoIPHandler) Lookup(ip net.IP) (*geoip2.CityResult, error) {
	return g.db.Lookup(ip)
}
