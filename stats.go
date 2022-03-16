package main

import (
	"sync"
	"time"

	"github.com/COSI_Lab/Mirror/logging"
)

var statisitcs NGINXStatistics
var statisitcsLock = &sync.RWMutex{}

// Loads the latest NGINX stats from the database
func InitNGINXStats(projects map[string]*Project) {
	statisitcsLock.Lock()

	// Query influxdb for the latest stats
	statisitcs = QueryTotalBytesByDistro(projects)
	logging.Info("Loaded responses from influxdb")

	statisitcsLock.Unlock()
}

// NGINX statisitcs
func HandleNGINXStats(entries chan *LogEntry) {
	ticker := time.NewTicker(1 * time.Minute)

	for {
		select {
		case <-ticker.C:
			SendTotalBytesByDistro()
		case entry := <-entries:
			statisitcsLock.Lock()
			if _, ok := statisitcs[entry.Distro]; ok {
				statisitcs[entry.Distro].BytesSent += entry.BytesSent
				statisitcs[entry.Distro].BytesRecv += entry.BytesRecv
				statisitcs[entry.Distro].Requests++
			} else {
				statisitcs["other"].BytesSent += entry.BytesSent
				statisitcs["other"].BytesRecv += entry.BytesRecv
				statisitcs["other"].Requests++
			}

			statisitcs["total"].BytesSent += entry.BytesSent
			statisitcs["total"].BytesRecv += entry.BytesRecv
			statisitcs["total"].Requests++
			statisitcsLock.Unlock()
		}
	}
}
