package main

import (
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

type DataPoint *write.Point

func SetupWriteClient(token string) api.WriteAPI {
	// create new client with default option for server url authenticate by token
	client := influxdb2.NewClient("https://mirror.clarkson.edu:8086", token)
	return client.WriteAPI("COSI", "test")
}
