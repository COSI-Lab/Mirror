package main

import (
	"time"

	"github.com/influxdata/influxdb-client-go/v2/api"
)

// Aggregator is an interface for aggregating a metric `T`
type Aggregator[T any] interface {
	// Initialize the aggregator with a starting value from influxdb
	Init(reader api.QueryAPI) (lastUpdated time.Time, err error)

	// Aggregate adds metric T into the aggregator
	Aggregate(entry T)

	// Send the aggregated statistics to influxdb
	Send(writer api.WriteAPI)
}

// StartAggregator starts the aggregator with the given Aggregator implementation, channel of type T, influxdb QueryAPI and WriteAPI.
// It returns the lastUpdated time and an error if any occurred during initialization.
func StartAggregator[T any](aggregator Aggregator[T], c <-chan T, reader api.QueryAPI, writer api.WriteAPI) (lastUpdated time.Time, err error) {
	lastUpdated, err = aggregator.Init(reader)
	if err != nil {
		return lastUpdated, err
	}

	go func() {
		ticker := time.NewTicker(time.Minute)

		for {
			select {
			case <-ticker.C:
				aggregator.Send(writer)
			case entry := <-c:
				aggregator.Aggregate(entry)
			}
		}
	}()

	return lastUpdated, nil
}

// NetStat is a commonly used struct for aggregating network statistics
type NetStat struct {
	BytesSent int64
	BytesRecv int64
	Requests  int64
}

// ParseLineError is an error type when parsing a line in the rsyncd or nginx feed
type ParseLineError struct{}

// Error returns the error message
func (e ParseLineError) Error() string {
	return "Failed to parse line"
}
