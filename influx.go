package main

import (
	"context"
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

	writer = client.WriteAPI("COSI", "test")
	reader = client.QueryAPI("COSI")
}

// Sends the latest NGINX stats to the database
func SendTotalBytesByDistro(bytesByDistro map[string]int) {
	// Measure time
	t := time.Now()

	// Create and send points
	for short, bytes := range bytesByDistro {
		p := influxdb2.NewPoint("mirror", map[string]string{"distro": short}, map[string]interface{}{"bytes_sent": bytes}, t)
		writer.WritePoint(p)
	}
}

// Loads the latest NGINX stats from the database
// Returns a map of distro short names to total bytes sent and total in the map
func QueryTotalBytesByDistro(shorts []string) (map[string]int, int) {
	// Map from short names to bytes sent
	bytesByDistro := make(map[string]int)

	for i := 0; i < len(shorts); i++ {
		bytesByDistro[shorts[i]] = 0
	}

	/*
		from(bucket: \"test\")
			|> range(start: -7d)
			|> filter(fn: (r) => r[\"_measurement\"] == \"mirror\" and  r[\"_field\"] == \"bytes_sent\")
			|> last()
	*/
	result, err := reader.Query(context.Background(), "from(bucket: \"test\") |> range(start: -7d) |> filter(fn: (r) => r[\"_measurement\"] == \"mirror\" and  r[\"_field\"] == \"bytes_sent\") |> last()")

	if err != nil {
		logging.Log(logging.Error, "Error querying influxdb", err)
	}

	total := 0
	for result.Next() {
		if result.Err() == nil {
			distro, ok := result.Record().ValueByKey("distro").(string)
			if !ok {
				logging.Log(logging.Warn, "InitNGINXStats can not parse distro to string: ", distro)
				continue
			}

			bytes, ok := result.Record().Value().(int64)
			if !ok {
				logging.Log(logging.Warn, "InitNGINXStats can not parse ", distro, " bytes to int ", distro+result.Record().String())
				continue
			}

			if _, ok := bytesByDistro[distro]; ok {
				bytesByDistro[distro] = int(bytes)
				total += int(bytes)
			}
		} else {
			logging.Log(logging.Warn, "InitNGINXStats Flux Query Error", result.Err())
		}
	}

	return bytesByDistro, total
}
