package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/COSI-Lab/logging"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

type NginxStatistics map[string]*NginxDistroStat
type NginxDistroStat struct {
	BytesSent int
	BytesRecv int
	Requests  int
}

type Statistics struct {
	sync.RWMutex
	nginx  NginxStatistics
	rsyncd struct {
		BytesSent int
		BytesRecv int
		Requests  int
	}
}

var statistics Statistics

// Loads the latest statistics from the database
func InitStatistics(projects map[string]*Project) (lastUpdated time.Time, err error) {
	return QueryStatistics(projects)
}

// NGINX statistics
func HandleStatistics(nginxEntries chan *NginxLogEntry, rsyncdEntries chan *RsyncdLogEntry) {
	// We send the latest stats to influxdb every minute
	ticker := time.NewTicker(1 * time.Minute)

	for {
		select {
		case <-ticker.C:
			Sendstatistics()
		case entry := <-nginxEntries:
			statistics.Lock()
			if _, ok := statistics.nginx[entry.Distro]; ok {
				statistics.nginx[entry.Distro].BytesSent += entry.BytesSent
				statistics.nginx[entry.Distro].BytesRecv += entry.BytesRecv
				statistics.nginx[entry.Distro].Requests++
			} else {
				statistics.nginx["other"].BytesSent += entry.BytesSent
				statistics.nginx["other"].BytesRecv += entry.BytesRecv
				statistics.nginx["other"].Requests++
			}
			statistics.nginx["total"].BytesSent += entry.BytesSent
			statistics.nginx["total"].BytesRecv += entry.BytesRecv
			statistics.nginx["total"].Requests++
			statistics.Unlock()
		case entry := <-rsyncdEntries:
			statistics.Lock()
			statistics.rsyncd.BytesSent += entry.sent
			statistics.rsyncd.BytesRecv += entry.recv
			statistics.rsyncd.Requests++
			statistics.Unlock()
		}
	}
}

// Sends the latest NGINX stats to the database
func Sendstatistics() {
	if influxReadOnly {
		logging.Info("INFLUX_READ_ONLY is set, not sending data to influx")
		return
	}

	// Measure time
	t := time.Now()

	// Create and send points
	statistics.RLock()
	defer statistics.RUnlock()

	for short, stat := range statistics.nginx {
		p := influxdb2.NewPoint("nginx",
			map[string]string{"distro": short},
			map[string]interface{}{
				"bytes_sent": stat.BytesSent,
				"bytes_recv": stat.BytesRecv,
				"requests":   stat.Requests,
			}, t)
		writer.WritePoint(p)
	}

	p := influxdb2.NewPoint("rsyncd", map[string]string{}, map[string]interface{}{
		"bytes_sent": statistics.rsyncd.BytesSent,
		"bytes_recv": statistics.rsyncd.BytesRecv,
		"requests":   statistics.rsyncd.Requests,
	}, t)
	writer.WritePoint(p)

	logging.Info("Sent statistics")
}

// Queries the database for the latest statistics
func QueryStatistics(projects map[string]*Project) (lastUpdated time.Time, err error) {
	// Map from short names to bytes sent
	statistics = Statistics{
		nginx: make(NginxStatistics),
	}

	for short := range projects {
		statistics.nginx[short] = &NginxDistroStat{}
	}
	statistics.nginx["other"] = &NginxDistroStat{}
	statistics.nginx["total"] = &NginxDistroStat{}

	result, err := QueryNginxStatistics()
	if err != nil {
		return lastUpdated, err
	}

	for result.Next() {
		if result.Err() == nil {
			// Get the data point
			dp := result.Record()

			// Update the time of the measurement
			lastUpdated = dp.Time()

			// Get the distro short name
			distro, ok := dp.ValueByKey("distro").(string)
			if !ok {
				logging.Warn("Error getting distro short name")
				fmt.Printf("%T %v\n", distro, distro)
				continue
			}

			if statistics.nginx[distro] == nil {
				continue
			}

			// Get the field
			field, ok := dp.ValueByKey("_field").(string)
			if !ok {
				logging.Warn("Error getting field")
				fmt.Printf("%T %v\n", field, field)
				continue
			}

			// Switch on the field
			switch field {
			case "bytes_sent":
				sent, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting bytes sent")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				statistics.nginx[distro].BytesSent = int(sent)
			case "bytes_recv":
				received, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting bytes recv")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				statistics.nginx[distro].BytesRecv = int(received)
			case "requests":
				requests, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting requests")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				statistics.nginx[distro].Requests = int(requests)
			}
		} else {
			logging.Warn("InitNGINXStats Flux Query Error", result.Err())
		}
	}
	result.Close()

	result, err = QueryRsyncdStatistics()
	if err != nil {
		return lastUpdated, err
	}

	for result.Next() {
		if result.Err() == nil {
			// Get the data point
			dp := result.Record()

			// Update the time of the measurement
			lastUpdated = dp.Time()

			// Get the field
			field, ok := dp.ValueByKey("_field").(string)
			if !ok {
				logging.Warn("Error getting field")
				fmt.Printf("%T %v\n", field, field)
				continue
			}

			// Switch on the field
			switch field {
			case "bytes_sent":
				sent, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting bytes sent")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				statistics.rsyncd.BytesSent = int(sent)
			case "bytes_recv":
				received, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting bytes recv")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				statistics.rsyncd.BytesRecv = int(received)
			case "requests":
				requests, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting requests")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				statistics.rsyncd.Requests = int(requests)
			}
		} else {
			logging.Warn("InitNGINXStats Flux Query Error", result.Err())
		}
	}

	return lastUpdated, nil
}

func QueryNginxStatistics() (*api.QueryTableResult, error) {
	// You can paste this into the influxdb data explorer
	/*
		from(bucket: "stats")
		    |> range(start: 0, stop: now())
		    |> filter(fn: (r) => r["_measurement"] == "nginx")
		    |> filter(fn: (r) => r["_field"] == "bytes_sent" or r["_field"] == "bytes_recv" or r["_field"] == "requests")
		    |> last()
		    |> group(columns: ["distro"], mode: "by")
	*/
	const request = "from(bucket: \"stats\") |> range(start: 0, stop: now()) |> filter(fn: (r) => r[\"_measurement\"] == \"nginx\") |> filter(fn: (r) => r[\"_field\"] == \"bytes_sent\" or r[\"_field\"] == \"bytes_recv\" or r[\"_field\"] == \"requests\") |> last() |> group(columns: [\"distro\"], mode: \"by\")"

	// try the query at most 5 times
	for i := 0; i < 5; i++ {
		result, err := reader.Query(context.Background(), request)

		if err != nil {
			logging.Warn("Failed to querying influxdb nginx statistics", err)
			continue
		}

		return result, nil
	}

	return nil, errors.New("Error querying influxdb")
}

func QueryRsyncdStatistics() (result *api.QueryTableResult, err error) {
	// You can paste this into the influxdb data explorer
	/*
		from(bucket: "stats")
		    |> range(start: 0, stop: now())
		    |> filter(fn: (r) => r["_measurement"] == "rsyncd")
		    |> filter(fn: (r) => r["_field"] == "bytes_sent" or r["_field"] == "bytes_recv" or r["_field"] == "requests")
		    |> last()
	*/
	const request = "from(bucket: \"stats\") |> range(start: 0, stop: now()) |> filter(fn: (r) => r[\"_measurement\"] == \"rsyncd\") |> filter(fn: (r) => r[\"_field\"] == \"bytes_sent\" or r[\"_field\"] == \"bytes_recv\") |> last()"

	// try the query at most 5 times
	for i := 0; i < 5; i++ {
		result, err = reader.Query(context.Background(), request)

		if err != nil {
			logging.Warn("Failed to querying influxdb rsyncd statistics", err)
			continue
		}

		return result, nil
	}

	return nil, err
}
