package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/COSI-Lab/logging"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

type NetStat struct {
	BytesSent int
	BytesRecv int
	Requests  int
}
type DistroStatistics map[string]*NetStat
type Statistics struct {
	sync.RWMutex
	nginx    DistroStatistics
	clarkson DistroStatistics
	rsyncd   NetStat
}

var statistics Statistics
var clarksonIPv4net *net.IPNet
var clarksonIPv6net *net.IPNet

// Prepare filters and regular expressions
func init() {
	var err error
	_, clarksonIPv4net, err = net.ParseCIDR("128.153.0.0/16")
	if err != nil {
		logging.Panic(err)
		os.Exit(1)
	}
	_, clarksonIPv6net, err = net.ParseCIDR("2605:6480::/32")
	if err != nil {
		logging.Panic(err)
		os.Exit(1)
	}
}

// HandleStatistics recieves parsed log entries over channels and tracks the useful information
// The statistics object should be created before this function can be run.
func HandleStatistics(nginxEntries chan *NginxLogEntry, rsyncdEntries chan *RsyncdLogEntry) {
	// We send the latest stats to influxdb every minute
	ticker := time.NewTicker(1 * time.Minute)

	for {
		select {
		case <-ticker.C:
			Sendstatistics()
		case entry := <-nginxEntries:
			statistics.Lock()
			// Track all distro usage
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

			// Additionally track usage from within the clarkson network
			if clarksonIPv4net.Contains(entry.IP) || clarksonIPv6net.Contains(entry.IP) {
				if _, ok := statistics.clarkson[entry.Distro]; ok {
					statistics.clarkson[entry.Distro].BytesSent += entry.BytesSent
					statistics.clarkson[entry.Distro].BytesRecv += entry.BytesRecv
					statistics.clarkson[entry.Distro].Requests++
				} else {
					statistics.clarkson["other"].BytesSent += entry.BytesSent
					statistics.clarkson["other"].BytesRecv += entry.BytesRecv
					statistics.clarkson["other"].Requests++
				}
				statistics.clarkson["total"].BytesSent += entry.BytesSent
				statistics.clarkson["total"].BytesRecv += entry.BytesRecv
				statistics.clarkson["total"].Requests++
			}
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

// Sends the latest statistics to the database
func Sendstatistics() {
	if influxReadOnly {
		logging.Info("INFLUX_READ_ONLY is set, not sending data to influx")
		return
	}

	t := time.Now()

	statistics.RLock()
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
	for short, stat := range statistics.clarkson {
		p := influxdb2.NewPoint("clarkson",
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

	// Never forget to release the lock.
	// Just to be safe let go of this lock before attempting to log.
	statistics.RUnlock()

	logging.Info("Sent statistics")
}

// InitStatistics queries the database for the all of the latest statistics
// In general everything in `statistics` should be monotonically increasing
// lastUpdated should be the same no matter where we check
func InitStatistics(projects map[string]*Project) (lastUpdated time.Time, err error) {
	// Map from short names to bytes sent
	statistics = Statistics{}

	lastUpdated, statistics.nginx, err = QueryDistroStatistics(projects, "nginx")
	if err != nil {
		return lastUpdated, err
	}
	lastUpdated, statistics.clarkson, err = QueryDistroStatistics(projects, "clarkson")
	if err != nil {
		return lastUpdated, err
	}

	statistics.rsyncd, err = QueryRsyncdStatistics()
	if err != nil {
		return lastUpdated, err
	}

	return lastUpdated, nil
}

// measurement is the particular filter you want `DistroStatistics` from
// current "clarkson" and "nginx" (all) are supported
func QueryDistroStatistics(projects map[string]*Project, measurement string) (lastUpdated time.Time, stats DistroStatistics, err error) {
	// You can paste this into the influxdb data explorer
	// Replace MEASUREMENT with "nginx" or "clarkson"
	/*
		from(bucket: "stats")
		    |> range(start: 0, stop: now())
		    |> filter(fn: (r) => r["_measurement"] == "MEASUREMENT")
		    |> filter(fn: (r) => r["_field"] == "bytes_sent" or r["_field"] == "bytes_recv" or r["_field"] == "requests")
		    |> last()
		    |> group(columns: ["distro"], mode: "by")
	*/
	request := fmt.Sprintf("from(bucket: \"stats\") |> range(start: 0, stop: now()) |> filter(fn: (r) => r[\"_measurement\"] == \"%s\") |> filter(fn: (r) => r[\"_field\"] == \"bytes_sent\" or r[\"_field\"] == \"bytes_recv\" or r[\"_field\"] == \"requests\") |> last() |> group(columns: [\"distro\"], mode: \"by\")", measurement)

	// try the query at most 5 times
	var result *api.QueryTableResult
	for i := 0; i < 5; i++ {
		result, err = reader.Query(context.Background(), request)

		if err != nil {
			logging.Warn("Failed to querying influxdb nginx statistics", err)
			// Sleep for some time before retrying
			time.Sleep(time.Duration(i) * time.Second)
			continue
		}

		break
	}

	if err != nil {
		return lastUpdated, stats, errors.New("Error querying influxdb")
	}

	stats = make(DistroStatistics)
	for short := range projects {
		stats[short] = &NetStat{}
	}
	stats["other"] = &NetStat{}
	stats["total"] = &NetStat{}

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

			if stats[distro] == nil {
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
				stats[distro].BytesSent = int(sent)
			case "bytes_recv":
				received, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting bytes recv")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				stats[distro].BytesRecv = int(received)
			case "requests":
				requests, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting requests")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				stats[distro].Requests = int(requests)
			}
		} else {
			logging.Warn("QueryDistroStatistics Flux Query Error", result.Err())
		}
	}
	result.Close()

	return lastUpdated, stats, nil
}

func QueryRsyncdStatistics() (stat NetStat, err error) {
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
	var result *api.QueryTableResult
	for i := 0; i < 5; i++ {
		result, err = reader.Query(context.Background(), request)

		if err != nil {
			logging.Warn("Failed to querying influxdb rsyncd statistics", err)
			// Sleep for some time before retrying
			time.Sleep(time.Duration(i) * time.Second)
			continue
		}

		break
	}

	if result == nil {
		return stat, errors.New("Error querying influxdb for rsyncd stat")
	}

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

	return stat, nil
}
