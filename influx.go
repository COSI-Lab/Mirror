package main

import (
	"crypto/tls"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

var writer api.WriteAPI
var reader api.QueryAPI

// SetupInfluxClients connects to influxdb and sets up the db clients
func SetupInfluxClients(token string) {
	// create new client with default option for server url authenticate by token
	options := influxdb2.DefaultOptions()
	options.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})

	client := influxdb2.NewClientWithOptions("https://mirror.clarkson.edu:8086", token, options)

	writer = client.WriteAPI("COSI", "stats")
	reader = client.QueryAPI("COSI")
}
