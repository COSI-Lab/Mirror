package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/COSI-Lab/Mirror/logging"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

// QueryDailyNginxStats gets the hourly nginx statistics from influxdb
// You can paste this into the influxdb data explorer
/*
from(bucket: "public")
    |> range(start: -25h, stop: now())
    |> filter(fn: (r) => r["_measurement"] == "nginx")
    |> filter(fn: (r) => r["_field"] == "bytes_sent")
    |> derivative(unit: 1h, nonNegative: true)
*/
func QueryDailyNginxStats() (*api.QueryTableResult, error) {
	const request = "from(bucket: \"public\") |> range(start: -25h, stop: now()) |> filter(fn: (r) => r[\"_measurement\"] == \"nginx\") |> filter(fn: (r) => r[\"_field\"] == \"bytes_sent\") |> derivative(unit: 1h, nonNegative: true)"

	// try the query at most 5 times
	for i := 0; i < 5; i++ {
		result, err := reader.Query(context.Background(), request)

		if err != nil {
			logging.Warn("Failed to querying influxdb nginx statistics", err)
			// Sleep for some time before retrying
			time.Sleep(time.Duration(i) * time.Second)
			continue
		}

		return result, nil
	}

	return nil, errors.New("Error querying influxdb")
}

// TimeSentPair is a simple product type for storing a time and the number of bytes sent
type TimeSentPair struct {
	t    time.Time
	sent int64
}

// PrepareDailySendStats prepares the daily send statistics for each distro
//
// For each distro return a slice of (time, bytes_sent) pairs for each hour in the last 24 hours
// It should be expected that the returned slices will be of length 24, but this is not guaranteed
// It is guaranteed that the returned time slices will be in chronological order
func PrepareDailySendStats() (map[string][]TimeSentPair, error) {
	result, err := QueryDailyNginxStats()
	if err != nil {
		return nil, err
	}

	// Create a map of distro -> [(time, bytes_sent)]
	distroMap := make(map[string][]TimeSentPair)

	// Iterate over the results
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

			if distroMap[distro] == nil {
				distroMap[distro] = make([]TimeSentPair, 0)
			}

			// Get the time
			t := dp.Time()
			sent := int64(dp.Value().(float64))

			// Add the data point to the map
			distroMap[distro] = append(distroMap[distro], TimeSentPair{t, sent})
		} else {
			logging.Warn("InitNGINXStats Flux Query Error", result.Err())
		}
	}

	// Sort each slice in the map by time
	for _, v := range distroMap {
		sort.Slice(v, func(i, j int) bool {
			return v[i].t.Before(v[j].t)
		})
	}

	return distroMap, nil
}

// CreateBarChart uses the go-chart library to create a bar chart from the given data
func CreateBarChart(timeSentPairs []TimeSentPair, project string) chart.BarChart {
	style := chart.Style{
		FillColor:   drawing.ColorFromHex("#00bcd4"),
		StrokeColor: drawing.ColorFromHex("#00bcd4"),
		StrokeWidth: 0,
	}

	max := float64(0)
	values := make([]chart.Value, 0)
	for _, v := range timeSentPairs {
		values = append(values, chart.Value{Style: style, Label: fmt.Sprint(v.t.Hour()), Value: float64(v.sent / 1000000000)})
		if float64(v.sent/1000000000) > max {
			max = float64(v.sent / 1000000000)
		}
	}

	graph := chart.BarChart{
		Title: fmt.Sprintf("Bytes sent per hour for \"%s\" | %s", project, time.Now().Format("Jan 02 2006")),
		Background: chart.Style{
			Padding: chart.Box{
				Top:    40,
				Left:   10,
				Right:  10,
				Bottom: 20,
			},
			FillColor: drawing.ColorFromHex("efefef"),
		},
		YAxis: chart.YAxis{
			Range: &chart.ContinuousRange{
				Min: 0,
				Max: max,
			},
			ValueFormatter: func(v interface{}) string {
				if vf, isFloat := v.(float64); isFloat {
					return fmt.Sprintf("%0.fG", vf)
				}
				return ""
			},
		},
		Height:   600,
		Width:    600 * 16 / 9,
		BarWidth: 16,
		Bars:     values,
	}

	return graph
}
