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
	timer := time.NewTimer(1 * time.Minute)

LOOP:
	for {
		select {
		case <-timer.C:
			SendTotalBytesByDistro(BytesByDistro)
		case entry, ok := <-entries:
			// TODO this shouldn't ever happen, but I'm keeping this in while we're testing
			if !ok {
				break LOOP
			}

			if _, ok := BytesByDistro[entry.Distro]; ok {
				BytesByDistro[entry.Distro] += entry.BytesSent
			} else {
				BytesByDistro["other"] += entry.BytesSent
			}
		}

		// TODO: Remove sleep condition
		time.Sleep(200 * time.Millisecond)
	}

	// This loop should never break out to here, if we hit this state then we're no longer sending distro usage stats
	logging.Panic("HandleNGINXStats stop sending distro bytes")
}
