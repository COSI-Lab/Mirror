package geoip

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

// Creates a new geoip2 reader by downloading the database from MaxMind.
// We also preform a sha256 check to ensure the database is not corrupt.

const CHECKSUM_URL string = "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&suffix=tar.gz.sha256&license_key="
const DATABASE_URL string = "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&suffix=tar.gz&license_key="

// Uses the MaxMind permalink to download the most recent sha256 checksum of the database.
func downloadHash(licenseKey string) (string, error) {
	resp, err := http.Get(CHECKSUM_URL + licenseKey)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Check the status code
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP Status %d while trying to download the sha256 checksum", resp.StatusCode)
	}

	// Return the sha256 checksum
	return string(body[:64]), nil
}

func downloadDatabase(licenseKey, checksum string) ([]byte, error) {
	resp, err := http.Get(DATABASE_URL + licenseKey)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP Status %d while trying to download the database", resp.StatusCode)
	}

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// Calculate the sha256 checksum of the tarball
	calculatedHash := sha256.Sum256(body)
	calculatedHashString := fmt.Sprintf("%x", calculatedHash)

	// Check the checksum
	if checksum != calculatedHashString {
		return nil, fmt.Errorf("checksum mismatch. Expected %s, got %s", checksum, calculatedHashString)
	}

	// Here we have a tar.gz file. We need to extract it.
	gzr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		fmt.Println("Error creating gzip reader:", err)
		return nil, err
	}
	defer gzr.Close()

	// Read the files names of the things in the tar file
	tarReader := tar.NewReader(gzr)
	for {
		header, err := tarReader.Next()

		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Name ends with "GeoLite2-City.mmdb"
		if strings.HasSuffix(header.Name, "GeoLite2-City.mmdb") {
			// We found the database file. Read it.
			return ioutil.ReadAll(tarReader)
		}
	}

	// Return the database
	return nil, fmt.Errorf("database not found in the tarball")
}

func downloadAndCheckHash(licenseKey string) ([]byte, error) {
	// Download the hash
	hash, err := downloadHash(licenseKey)
	if err != nil {
		return nil, err
	}

	// Download the database
	return downloadDatabase(licenseKey, hash)
}