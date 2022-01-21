package main

import (
	"context"
	"log"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

var bytes_by_distro map[string]int

// Loads the latest NGINX stats from the database
func InitNGINXStats(shorts []string, reader api.QueryAPI) {
	// Map from short names to bytes sent
	bytes_by_distro = make(map[string]int)

	for i := 0; i < len(shorts); i++ {
		bytes_by_distro[shorts[i]] = 0
	}

	/*
		from(bucket: \"test\")
			|> range(start: -7d)
			|> filter(fn: (r) => r[\"_measurement\"] == \"mirror\" and  r[\"_field\"] == \"bytes_sent\")
			|> last()
	*/
	result, err := reader.Query(context.Background(), "from(bucket: \"test\") |> range(start: -7d) |> filter(fn: (r) => r[\"_measurement\"] == \"mirror\" and  r[\"_field\"] == \"bytes_sent\") |> last()")

	if err != nil {
		log.Println("\x1B[31m[Error]\x1B[0m", err)
	} else {
		for result.Next() {
			if result.Err() == nil {
				distro, ok := result.Record().ValueByKey("distro").(string)
				if !ok {
					log.Println("\x1B[33m[WARN]\x1B[0m InitNGINXStats can not parse distro to string")
					continue
				}

				bytes, ok := result.Record().Value().(int64)
				if !ok {
					log.Printf("\x1B[33m[WARN]\x1B[0m InitNGINXStats can not parse %s bytes to int\n%s\n", distro, result.Record().String())
					continue
				}

				if _, ok := bytes_by_distro[distro]; ok {
					bytes_by_distro[distro] = int(bytes)
				}
			} else {
				log.Println("\x1B[33m[WARN]\x1B[0m InitNGINXStats Flux Query Error", result.Err().Error())
			}
		}
	}

	log.Println("[INFO] InitNGINXStats successfully loaded previous stats from influxdb")
}

// NGINX statisitcs
func HandleNGINXStats(entries chan *LogEntry, writer api.WriteAPI) {
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

			timer.Reset(10 * time.Second)
		case entry, ok := <-entries:
			// TODO this shouldn't ever happen, but I'm keeping this in while we're testing
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

	// This loop should never break out to here, if we hit this state then we're no longer sending distro usage stats
	// TODO add fail detection so we can restart this loop
	log.Print("\x1B[31m[Error]\x1B[0m HandleNGINXStats stop sending distro bytes")
}
