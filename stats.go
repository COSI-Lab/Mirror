package main

import (
	"log"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

var bytes_by_distro map[string]int

// Loads the latest NGINX stats from the database
func InitNGINXStats() {

}

// NGINX statisitcs
func HandleNGINXStats(shorts []string, entries chan *LogEntry, writer api.WriteAPI) {
	// Map from short names to bytes sent
	bytes_by_distro = make(map[string]int)

	for i := 0; i < len(shorts); i++ {
		bytes_by_distro[shorts[i]] = 0
	}

	timer := time.NewTimer(10 * time.Second)

LOOP:
	for {
		select {
		case <-timer.C:
			// Measure time
			t := time.Now()

			// Create points
			for short, bytes := range bytes_by_distro {
				p := influxdb2.NewPoint("mirror", map[string]string{"distro": short}, map[string]interface{}{"bytes_sent": bytes}, t)
				writer.WritePoint(p)
			}

			log.Print("[INFO] Distro bytes sent")
			timer.Reset(10 * time.Second)
		case entry, ok := <-entries:
			// TODO this shouldn't ever happen, keeping this in while we're testing
			if !ok {
				break LOOP
			}

			if _, ok := bytes_by_distro[entry.Distro]; ok {
				bytes_by_distro[entry.Distro] += entry.BytesSent
			} else {
				bytes_by_distro["other"] += entry.BytesSent
			}
		}

		// TODO: Remove sleep condition
		time.Sleep(200 * time.Millisecond)
	}
}
