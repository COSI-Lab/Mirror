package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/COSI-Lab/Mirror/logging"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/nxadm/tail"
)

type RsyncdAggregator struct {
	stat NetStat
}

func NewRSYNCProjectAggregator() *RsyncdAggregator {
	return &RsyncdAggregator{}
}

func (a *RsyncdAggregator) Init(reader api.QueryAPI) (lastUpdated time.Time, err error) {
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
			time.Sleep(time.Second)
			continue
		}

		break
	}

	if result == nil {
		return time.Time{}, errors.New("Error querying influxdb for rsyncd stat")
	}

	for result.Next() {
		if result.Err() == nil {
			// Get the data point
			dp := result.Record()
			lastUpdated = dp.Time()

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
				a.stat.BytesSent = sent
			case "bytes_recv":
				received, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting bytes recv")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				a.stat.BytesRecv = received
			case "requests":
				requests, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting requests")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				a.stat.Requests = requests
			}
		} else {
			logging.Warn("Error querying influxdb for rsyncd stat", result.Err())
		}
	}

	return lastUpdated, nil
}

func (a *RsyncdAggregator) Aggregate(entry RsyncdLogEntry) {
	a.stat.BytesSent += entry.sent
	a.stat.BytesRecv += entry.recv
	a.stat.Requests++
}

func (a *RsyncdAggregator) Send(writer api.WriteAPI) {
	t := time.Now()

	p := influxdb2.NewPoint("rsyncd", map[string]string{}, map[string]interface{}{
		"bytes_sent": a.stat.BytesSent,
		"bytes_recv": a.stat.BytesRecv,
		"requests":   a.stat.Requests,
	}, t)
	writer.WritePoint(p)
}

type RsyncdLogEntry struct {
	time time.Time
	sent int64
	recv int64
}

func TailRSYNCLogFile(logFile string, lastUpdated time.Time, channels []chan<- RsyncdLogEntry) {
	// Find the offset of the line where the date is past lastUpdated
	start := time.Now()

	f, err := os.Open(logFile)
	if err != nil {
		logging.Error(err)
		return
	}

	// Preforms a linear scan of the log file to find the offset to continue tailing from
	offset := int64(0)
	s := bufio.NewScanner(f)
	for s.Scan() {
		tm, err := parseRSYNCDate(s.Text())
		if err == nil && tm.After(lastUpdated) {
			break
		}
		offset += int64(len(s.Text()) + 1)
	}
	logging.Info("Found rsyncd log offset in", time.Since(start))

	// Tail the log file `tail -F` starting at the offset
	seek := tail.SeekInfo{
		Offset: offset,
		Whence: io.SeekStart,
	}
	tail, err := tail.TailFile(logFile, tail.Config{Follow: true, ReOpen: true, MustExist: true, Location: &seek})
	if err != nil {
		logging.Error("Failed to start tailing `rsyncd.log`:", err)
		return
	}

	logging.Success("Tailing rsyncd log file")

	// Parse each line as we receive it
	for line := range tail.Lines {
		entry, err := parseRsyncdLine(line.Text)

		if err == nil {
			for ch := range channels {
				channels[ch] <- entry
			}
		}
	}
}

type ParseLineError struct{}

func (e ParseLineError) Error() string {
	return "Failed to parse line"
}

func parseRSYNCDate(line string) (time.Time, error) {
	// Split the line over whitespace
	parts := strings.Split(line, " ")

	if len(parts) < 2 {
		return time.Time{}, ParseLineError{}
	}

	// The 1st part is the date
	dt := parts[0]
	// 2nd part is the time
	tm := parts[1]

	// make the time.Time object
	t, err := time.Parse("2006/01/02 15:04:05", dt+" "+tm)
	if err != nil {
		return time.Time{}, err
	}

	return t, nil
}

func parseRsyncdLine(line string) (entry RsyncdLogEntry, err error) {
	// 2022/04/20 20:00:10 [pid] sent XXX bytes  received XXX bytes  total size XXX

	// Split the line over whitespace
	parts := strings.Split(line, " ")

	// the line we want has 14 parts
	if len(parts) != 14 {
		return entry, ParseLineError{}
	}

	// the 4th part is "sent"
	if parts[3] != "sent" {
		return entry, ParseLineError{}
	}

	// The 1st part is the date
	dt := parts[0]
	// 2nd part is the time
	tm := parts[1]

	// make the time.Time object
	entry.time, err = time.Parse("2006/01/02 15:04:05", dt+" "+tm)
	if err != nil {
		return entry, err
	}

	// part 5 is the number of bytes sent
	entry.sent, err = strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		fmt.Println(err)
		return entry, ParseLineError{}
	}

	// part 9 is the number of bytes received
	entry.recv, err = strconv.ParseInt(parts[8], 10, 64)
	if err != nil {
		fmt.Println(err)
		return entry, ParseLineError{}
	}

	return entry, nil
}
