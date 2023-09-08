package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"sort"

	"github.com/COSI-Lab/Mirror/logging"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

var writer api.WriteAPI
var reader api.QueryAPI

func SetupInfluxClients(token string) {
	// create new client with default option for server url authenticate by token
	options := influxdb2.DefaultOptions()
	options.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})

	client := influxdb2.NewClientWithOptions("https://mirror.clarkson.edu:8086", token, options)

	if !influxReadOnly {
		writer = client.WriteAPI("COSI", "stats")
	}
	reader = client.QueryAPI("COSI")
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

// implements the sort interface
type LineChart struct {
	Sent  []float64
	Recv  []float64
	Times []int64
}

func (l LineChart) Len() int {
	return len(l.Sent)
}

func (l LineChart) Swap(i, j int) {
	l.Sent[i], l.Sent[j] = l.Sent[j], l.Sent[i]
	l.Recv[i], l.Recv[j] = l.Recv[j], l.Recv[i]
	l.Times[i], l.Times[j] = l.Times[j], l.Times[i]
}

func (l LineChart) Less(i, j int) bool {
	return l.Times[i] < l.Times[j]
}

// Gets the total network bytes sent and received for the last week in 1 hour blocks
func QueryWeeklyNetStats() (line LineChart, err error) {
	// You can paste this into the influxdb data explorer
	/*
		from(bucket: "system")
			|> range(start: -7d, stop: now())
			|> filter(fn: (r) => r["_measurement"] == "net" and r["interface"] == "enp8s0")
			|> filter(fn: (r) => r["_field"] == "bytes_sent" or r["_field"] == "bytes_recv")
			|> aggregateWindow(every: 1h, fn: last)
			|> derivative(unit: 1s, nonNegative: true)
			|> yield(name: "nonnegative derivative")
	*/
	result, err := reader.Query(context.Background(), "from(bucket: \"system\") |> range(start: -7d, stop: now()) |> filter(fn: (r) => r[\"_measurement\"] == \"net\" and r[\"interface\"] == \"enp8s0\") |> filter(fn: (r) => r[\"_field\"] == \"bytes_sent\" or r[\"_field\"] == \"bytes_recv\") |> aggregateWindow(every: 1h, fn: last) |> derivative(unit: 1s, nonNegative: true) |> yield(name: \"nonnegative derivative\")")

	if err != nil {
		return LineChart{}, err
	}

	sent := make([]float64, 0)
	recv := make([]float64, 0)
	times := make([]int64, 0)

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

	line = LineChart{
		Sent:  sent,
		Recv:  recv,
		Times: times,
	}

	fmt.Println(len(sent), len(recv), len(times))

	sort.Sort(line)

	return line, nil
}
