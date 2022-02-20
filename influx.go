package main

import (
	"context"
	"fmt"
	"time"

	"github.com/COSI_Lab/Mirror/logging"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

type DataPoint *write.Point

var writer api.WriteAPI
var reader api.QueryAPI

func SetupInfluxClients(token string) {
	// create new client with default option for server url authenticate by token
	client := influxdb2.NewClient("https://mirror.clarkson.edu:8086", token)

	writer = client.WriteAPI("COSI", "stats")
	reader = client.QueryAPI("COSI")
}

// Statistics measured for each distribution
type DistroStat struct {
	BytesSent int
	BytesRecv int
	Requests  int
}

type NGINXStatistics map[string]*DistroStat

// Sends the latest NGINX stats to the database
func SendTotalBytesByDistro(statistics NGINXStatistics) {
	// Measure time
	t := time.Now()

	// Create and send points
	for short, stat := range statistics {
		p := influxdb2.NewPoint("mirror",
			map[string]string{"distro": short},
			map[string]interface{}{
				"bytes_sent": stat.BytesSent,
				"bytes_recv": stat.BytesRecv,
				"requests":   stat.Requests,
			}, t)
		writer.WritePoint(p)
	}

	logging.Info("Sent nginx stats")
}

// Loads the latest NGINX stats from the database
// Returns a map of distro short names to total bytes sent and total in the map
func QueryTotalBytesByDistro(projects map[string]*Project) NGINXStatistics {
	// Map from short names to bytes sent
	statistics := make(NGINXStatistics)

	for short := range projects {
		statistics[short] = &DistroStat{}
	}
	statistics["other"] = &DistroStat{}

	// You can paste this into the influxdb data explorer
	/*
		from(bucket: "stats")
		    |> range(start: 0, stop: now())
		    |> filter(fn: (r) => r["_measurement"] == "mirror")
		    |> filter(fn: (r) => r["_field"] == "bytes_sent" or r["_field"] == "bytes_recv" or r["_field"] == "requests")
		    |> last()
		    |> group(columns: ["distro"], mode: "by")
	*/
	result, err := reader.Query(context.Background(), "from(bucket: \"stats\") |> range(start: 0, stop: now()) |> filter(fn: (r) => r[\"_measurement\"] == \"mirror\") |> filter(fn: (r) => r[\"_field\"] == \"bytes_sent\" or r[\"_field\"] == \"bytes_recv\" or r[\"_field\"] == \"requests\") |> last() |> group(columns: [\"distro\"], mode: \"by\")")

	if err != nil {
		logging.Error("Error querying influxdb", err)
	}

	for result.Next() {
		if result.Err() == nil {
			// Get the data point
			dp := result.Record()

			// Get the distro short name
			distro, ok := dp.ValueByKey("distro").(string)
			if !ok {
				logging.Warn("Error getting distro short name")
				fmt.Printf("%T %v\n", distro, distro)
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
				statistics[distro].BytesSent = int(sent)
			case "bytes_recv":
				received, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting bytes recv")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				statistics[distro].BytesRecv = int(received)
			case "requests":
				requests, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting requests")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				statistics[distro].Requests = int(requests)
			}
		} else {
			logging.Warn("InitNGINXStats Flux Query Error", result.Err())
		}
	}

	return statistics
}
