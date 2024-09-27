package geoip

import (
	"encoding/hex"
	"net"
	"os"
	"testing"

	"github.com/IncSW/geoip2"
)

// GeoIP tests
func TestGetSHA256(t *testing.T) {
	// Get the license key through environment variables
	licenseKey := os.Getenv("MAXMIND_LICENSE_KEY")
	if licenseKey == "" {
		t.Error("MAXMIND_LICENSE_KEY environment variable not set")
		return
	}

	// Download the sha256 checksum
	sha256, err := downloadHash(licenseKey)
	if err != nil {
		t.Error(err)
	}

	// the checksum should be a hex string that is 64 characters long
	if len(sha256) != 64 {
		t.Error("sha256 checksum is not 64 characters long")
	}

	// decode the sha256 checksum
	_, err = hex.DecodeString(sha256)
	if err != nil {
		t.Error(err)
	}
}

// Download a new database
func TestDownloadDatabase(t *testing.T) {
	// Get the license key through environment variables
	licenseKey := os.Getenv("MAXMIND_LICENSE_KEY")
	if licenseKey == "" {
		t.Error("MAXMIND_LICENSE_KEY environment variable not set")
		return
	}

	// Prepare the checksum
	sha256, err := downloadHash(licenseKey)
	if err != nil {
		t.Error(err)
	}

	// Download the database
	bytes, err := downloadDatabase(licenseKey, sha256)
	if err != nil {
		t.Error(err)
	}

	// Verify that the database can be opened
	_, err = geoip2.NewCityReader(bytes)
	if err != nil {
		t.Error(err)
	}
}

func TestLookups(t *testing.T) {
	// Get the license key through environment variables
	licenseKey := os.Getenv("MAXMIND_LICENSE_KEY")
	if licenseKey == "" {
		t.Error("MAXMIND_LICENSE_KEY environment variable not set")
		return
	}

	geoip, err := NewGeoIPHandler(licenseKey)
	if err != nil {
		t.Error(err)
		return
	}

	// Lookup some IP addresses
	ips := []string{"128.153.145.19", "2605:6480:c051:100::1"}
	for _, ip := range ips {
		_, err := geoip.Lookup(net.ParseIP(ip))
		if err != nil {
			t.Error(ip, err)
		}
	}

	// TODO: Add a test that ensures maxmind knows where mirror is

	geoip.Close()
}