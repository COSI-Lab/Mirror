package main

import (
	"time"

	"github.com/COSI_Lab/Mirror/logging"
)

var BytesByDistro map[string]int

// Loads the latest NGINX stats from the database
func InitNGINXStats(projects map[string]*Project) {
	// Query influxdb for the latest stats
	var total int
	BytesByDistro, total = QueryTotalBytesByDistro(projects)

	logging.Info("Loaded", total, "bytes from influxdb")
}

// NGINX statisitcs
func HandleNGINXStats(entries chan *LogEntry) {
	ticker := time.NewTicker(1 * time.Minute)

	for {
		select {
		case <-ticker.C:
			SendTotalBytesByDistro(BytesByDistro)
		case entry := <-entries:
			if _, ok := BytesByDistro[entry.Distro]; ok {
				BytesByDistro[entry.Distro] += entry.BytesSent
			} else {
				// logging.Info("Unknown distro", entry.Distro)
				BytesByDistro["other"] += entry.BytesSent
			}
		}
	}
}
