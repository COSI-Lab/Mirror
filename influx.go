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
func SendTotalBytesByDistro() {
	if influxReadOnly {
		logging.Info("INFLUX_READ_ONLY is set, not sending data to influx")
		return
	}

	// Measure time
	t := time.Now()

	// Create and send points
	statisitcsLock.RLock()
	for short, stat := range statisitcs {
		p := influxdb2.NewPoint("mirror",
			map[string]string{"distro": short},
			map[string]interface{}{
				"bytes_sent": stat.BytesSent,
				"bytes_recv": stat.BytesRecv,
				"requests":   stat.Requests,
			}, t)
		writer.WritePoint(p)
	}
	statisitcsLock.RUnlock()

	logging.Info("Sent nginx stats")
}

// Loads the latest NGINX stats from the database
// Returns a map of distro short names to total bytes sent and total in the map
func QueryNGINXStatistics(projects map[string]*Project) NGINXStatistics {
	// Map from short names to bytes sent
	statistics := make(NGINXStatistics)

	for short := range projects {
		statistics[short] = &DistroStat{}
	}
	statistics["other"] = &DistroStat{}
	statistics["total"] = &DistroStat{}

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

			if statistics[distro] == nil {
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

// Gets the bytes sent for each project in the last 24 hours
// Returns a sorted list of bytes sent for each project
func QueryBytesSentByProject() (map[string]int64, error) {
	// Map from short names to bytes sent
	bytesSent := make(map[string]int64)

	// You can paste this into the influxdb data explorer
	/*
		from(bucket: "stats")
			|> range(start: -24h, stop: now())
			|> filter(fn: (r) => r["_measurement"] == "mirror")
			|> filter(fn: (r) => r["_field"] == "bytes_sent")
			|> spread()
			|> yield(name: "spread")
	*/
	result, err := reader.Query(context.Background(), "from(bucket: \"stats\") |> range(start: -24h, stop: now()) |> filter(fn: (r) => r[\"_measurement\"] == \"mirror\") |> filter(fn: (r) => r[\"_field\"] == \"bytes_sent\") |> spread() |> yield(name: \"spread\")")

	if err != nil {
		return nil, err
	}

	for result.Next() {
		if result.Err() == nil {
			// Get the data point
			dp := result.Record()

			// Get the project short name
			project, ok := dp.ValueByKey("distro").(string)
			if !ok {
				logging.Warn("Error getting distro short name")
				fmt.Printf("%T %v\n", project, project)
				continue
			}

			// Get the bytes sent
			sent, ok := dp.ValueByKey("_value").(int64)
			if !ok {
				logging.Warn("Error getting bytes sent")
				fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
				continue
			}

			bytesSent[project] = sent
		} else {
			logging.Warn("InitNGINXStats Flux Query Error", result.Err())
		}
	}

	return bytesSent, nil
}

// Gets the total network bytes sent and recieved for the last week in 1 hour blocks
func QueryWeeklyNetStats() (sent []float64, recv []float64, times []int64, err error) {
	// You can paste this into the influxdb data explorer
	/*
		from(bucket: "system")
		  |> range(start: -7d, stop: now())
		  |> filter(fn: (r) => r["_measurement"] == "net")
		  |> filter(fn: (r) => r["_field"] == "bytes_sent" or r["_field"] == "bytes_recv")
		  |> derivative(unit: 1s, nonNegative: true)
		  |> aggregateWindow(every: 1h, fn: mean)
		  |> yield(name: "nonnegative derivative")
	*/
	result, err := reader.Query(context.Background(), "from(bucket: \"system\") |> range(start: -7d, stop: now()) |> filter(fn: (r) => r[\"_measurement\"] == \"net\") |> filter(fn: (r) => r[\"_field\"] == \"bytes_sent\" or r[\"_field\"] == \"bytes_recv\") |> derivative(unit: 1s, nonNegative: true) |> aggregateWindow(every: 1h, fn: mean) |> yield(name: \"nonnegative derivative\")")

	if err != nil {
		return nil, nil, nil, err
	}

	sent = make([]float64, 0)
	recv = make([]float64, 0)
	times = make([]int64, 0)

	for result.Next() {
		if result.Err() == nil {
			// Get the data point
			dp := result.Record()

			// Get the field
			field, ok := dp.ValueByKey("_field").(string)
			if !ok {
				logging.Warn("Error getting field")
				fmt.Printf("%T %v\n", field, field)
				continue
			}

			// Get the value
			value, ok := dp.ValueByKey("_value").(float64)
			if !ok {
				logging.Warn("Error getting value")
				fmt.Printf("%T %v\n", value, value)
				continue
			}

			switch field {
			case "bytes_sent":
				sent = append(sent, value)
			case "bytes_recv":
				recv = append(recv, value)
				times = append(times, dp.Time().Unix())
			}
		} else {
			logging.Warn("InitNGINXStats Flux Query Error", result.Err())
		}
	}
	return sent, recv, times, nil
}
