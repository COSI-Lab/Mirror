package main

import (
	"crypto/tls"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

// SetupInfluxClients connects to influxdb and sets up the db clients
func SetupInfluxClients(token string) (reader api.QueryAPI, writer api.WriteAPI) {
	// create new client with default option for server url authenticate by token
	options := influxdb2.DefaultOptions()
	options.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})

	client := influxdb2.NewClientWithOptions("https://mirror.clarkson.edu:8086", token, options)
	return client.QueryAPI("COSI"), client.WriteAPI("COSI", "stats")
}
