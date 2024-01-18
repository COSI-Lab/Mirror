package aggregator

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/COSI-Lab/Mirror/logging"
	"github.com/COSI-Lab/geoip"
	"github.com/IncSW/geoip2"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/nxadm/tail"
)

// It is critical that NGINX uses the following log format:
/*
 * log_format config '"$time_local" "$remote_addr" "$request" "$status" "$body_bytes_sent" "$request_length" "$http_user_agent"';
 * access_log /var/log/nginx/access.log config;
 */

// ProjectStatistics is a shorthand for map[string]*NetStat
type ProjectStatistics map[string]*NetStat

// NGINXProjectAggregator measures the popularity of each project (bytes sent, bytes received, and number of requests)
//
// It is given a stream of NGINXLogEntries and aggregates the statistics for each project
type NGINXProjectAggregator struct {
	stats map[string]ProjectStatistics

	// filter function
	filters map[string]func(NGINXLogEntry) bool
}

// NewNGINXProjectAggregator creates a new NGINXProjectAggregator with no measurements
func NewNGINXProjectAggregator() *NGINXProjectAggregator {
	return &NGINXProjectAggregator{
		stats:   map[string]ProjectStatistics{},
		filters: map[string]func(NGINXLogEntry) bool{},
	}
}

// AddMeasurement adds a measurement to the aggregator
// measurement is the name of the measurement in influxdb (e.g. "nginx" or "clarkson")
// filter is a function that checks if an entry should be counted for this measurement
func (aggregator *NGINXProjectAggregator) AddMeasurement(measurement string, filter func(NGINXLogEntry) bool) {
	aggregator.stats[measurement] = make(ProjectStatistics)
	aggregator.filters[measurement] = filter
}

// Init initializes the aggregator by querying the database for the latest statistics
func (aggregator *NGINXProjectAggregator) Init(reader api.QueryAPI) (lastUpdated time.Time, err error) {
	for measurement := range aggregator.filters {
		// You can paste this into the influxdb data explorer
		// Replace MEASUREMENT with "nginx" or "clarkson"
		/*
			from(bucket: "stats")
				|> range(start: 0, stop: now())
				|> filter(fn: (r) => r["_measurement"] == "MEASUREMENT")
				|> filter(fn: (r) => r["_field"] == "bytes_sent" or r["_field"] == "bytes_recv" or r["_field"] == "requests")
				|> last()
				|> group(columns: ["Project"], mode: "by")
		*/
		request := fmt.Sprintf("from(bucket: \"stats\") |> range(start: 0, stop: now()) |> filter(fn: (r) => r[\"_measurement\"] == \"%s\") |> filter(fn: (r) => r[\"_field\"] == \"bytes_sent\" or r[\"_field\"] == \"bytes_recv\" or r[\"_field\"] == \"requests\") |> last() |> group(columns: [\"Project\"], mode: \"by\")", measurement)

		// try the query at most 5 times
		var result *api.QueryTableResult
		for i := 0; i < 5; i++ {
			result, err = reader.Query(context.Background(), request)

			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			break
		}

		if err != nil {
			return lastUpdated, err
		}

		stats := make(ProjectStatistics)

		for result.Next() {
			if result.Err() != nil {
				logging.Warn("QueryProjectStatistics Flux Query Error", result.Err())
				continue
			}

			dp := result.Record()
			lastUpdated = dp.Time()

			// Get the Project short name
			Project, ok := dp.ValueByKey("Project").(string)
			if !ok {
				logging.Warn("Error getting Project short name")
				fmt.Printf("%T %v\n", Project, Project)
				continue
			}

			// Create a new NetStat for the project if it doesn't exist
			if _, ok := stats[Project]; !ok {
				stats[Project] = &NetStat{}
			}

			field, ok := dp.ValueByKey("_field").(string)
			if !ok {
				logging.Warn("Error getting field")
				fmt.Printf("%T %v\n", field, field)
				continue
			}

			switch field {
			case "bytes_sent":
				sent, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting bytes sent")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				stats[Project].BytesSent = sent
			case "bytes_recv":
				received, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting bytes recv")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				stats[Project].BytesRecv = received
			case "requests":
				requests, ok := dp.ValueByKey("_value").(int64)
				if !ok {
					logging.Warn("Error getting requests")
					fmt.Printf("%T %v\n", dp.ValueByKey("_value"), dp.ValueByKey("_value"))
					continue
				}
				stats[Project].Requests = requests
			}
		}
		result.Close()

		// Add "other" and "total" to the stats if they don't exist
		if _, ok := stats["other"]; !ok {
			stats["other"] = &NetStat{}
		}

		if _, ok := stats["total"]; !ok {
			stats["total"] = &NetStat{}
		}
	}

	return lastUpdated, nil
}

// Aggregate adds a NGINXLogEntry to the aggregator
func (aggregator *NGINXProjectAggregator) Aggregate(entry NGINXLogEntry) {
	for measurement, filter := range aggregator.filters {
		if !filter(entry) {
			return
		}

		stat := aggregator.stats[measurement]

		if _, ok := aggregator.stats[entry.Project]; ok {
			stat[entry.Project].BytesSent += entry.BytesSent
			stat[entry.Project].BytesRecv += entry.BytesRecv
			stat[entry.Project].Requests++
		} else {
			stat["other"].BytesSent += entry.BytesSent
			stat["other"].BytesRecv += entry.BytesRecv
			stat["other"].Requests++
		}

		stat["total"].BytesSent += entry.BytesSent
		stat["total"].BytesRecv += entry.BytesRecv
		stat["total"].Requests++
	}
}

// Send the aggregated statistics to influxdb
func (aggregator *NGINXProjectAggregator) Send(writer api.WriteAPI) {
	t := time.Now()

	for measurement, stats := range aggregator.stats {
		for short, stat := range stats {
			p := influxdb2.NewPoint(measurement,
				map[string]string{"Project": short},
				map[string]interface{}{
					"bytes_sent": stat.BytesSent,
					"bytes_recv": stat.BytesRecv,
					"requests":   stat.Requests,
				}, t)
			writer.WritePoint(p)
		}
	}
}

// NGINXLogEntry represents a parsed nginx log entry
type NGINXLogEntry struct {
	IP        net.IP
	City      *geoip2.CityResult
	Time      time.Time
	Method    string
	Project   string
	URL       string
	Version   string
	Status    int
	BytesSent int64
	BytesRecv int64
	Agent     string
}

var reQuotes = regexp.MustCompile(`"(.*?)"`)

// TailNGINXLogFile tails a log file and sends the parsed log entries to the specified channels
func TailNGINXLogFile(logFile string, lastUpdated time.Time, channels []chan<- NGINXLogEntry, geoipHandler *geoip.GeoIPHandler) {
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
		tm, err := parseNginxDate(s.Text())
		if err == nil && tm.After(lastUpdated) {
			break
		}
		offset += int64(len(s.Text()) + 1)
	}
	logging.Info("Found nginx log offset in", time.Since(start))

	// Tail the log file `tail -F` starting at the offset
	seek := tail.SeekInfo{
		Offset: offset,
		Whence: io.SeekStart,
	}
	tail, err := tail.TailFile(logFile, tail.Config{Follow: true, ReOpen: true, MustExist: true, Location: &seek})
	if err != nil {
		logging.Error("Failed to start tailing `nginx.log`:", err)
		return
	}

	logging.Success("Tailing nginx log file")

	// Parse each line as we receive it
	for line := range tail.Lines {
		entry, err := parseNginxLine(geoipHandler, line.Text)

		if err == nil {
			for ch := range channels {
				channels[ch] <- entry
			}
		}
	}
}

// parseNginxDate parses a single line of the nginx log file and returns the time.Time of the line
func parseNginxDate(line string) (time.Time, error) {
	return time.Parse("\"02/Jan/2006:15:04:05 -0700\"", reQuotes.FindString(line))
}

// parseNginxLine parses a single line of the nginx log file
// It's critical the log file uses the correct format found at the top of this file
// If the log file is not in the correct format or if some other part of the parsing fails
// this function will return an error
func parseNginxLine(geoipHandler *geoip.GeoIPHandler, line string) (entry NGINXLogEntry, err error) {
	// "$time_local" "$remote_addr" "$request" "$status" "$body_bytes_sent" "$request_length" "$http_user_agent";
	quoteList := reQuotes.FindAllString(line, -1)

	if len(quoteList) != 7 {
		return NGINXLogEntry{}, errors.New("invalid number of parameters in log entry")
	}

	// Trim quotation marks
	for i := 0; i < len(quoteList); i++ {
		quoteList[i] = quoteList[i][1 : len(quoteList[i])-1]
	}

	// Time
	entry.Time, err = time.Parse("02/Jan/2006:15:04:05 -0700", quoteList[0])
	if err != nil {
		return entry, err
	}

	// IPv4 or IPv6 address
	entry.IP = net.ParseIP(quoteList[1])
	if entry.IP == nil {
		return entry, errors.New("failed to parse ip")
	}

	// GeoIP lookup
	if geoipHandler == nil {
		city, err := geoipHandler.Lookup(entry.IP)
		if err != nil {
			entry.City = nil
		} else {
			entry.City = city
		}
	} else {
		entry.City = nil
	}

	// method url http version
	split := strings.Split(quoteList[2], " ")
	if len(split) != 3 {
		// this should never fail
		return entry, errors.New("invalid number of strings in request")
	}
	entry.Method = split[0]
	entry.URL = split[1]
	entry.Version = split[2]

	// Project is the top part of the URL path
	u, err := url.Parse(entry.URL)
	if err != nil {
		log.Fatal(err)
	}
	// Parse the path
	path := path.Clean(u.EscapedPath())
	// Return the first part of the path
	if pathSplit := strings.Split(path, "/"); len(pathSplit) > 1 {
		project := pathSplit[1]
		entry.Project = project
	} else {
		return entry, errors.New("could not parse project name")
	}

	// HTTP response status
	status, err := strconv.Atoi(quoteList[3])
	if err != nil {
		// this should never fail
		return entry, errors.New("could not parse http response status")
	}
	entry.Status = status

	// Bytes sent int64
	bytesSent, err := strconv.ParseInt(quoteList[4], 10, 64)
	if err != nil {
		// this should never fail
		return entry, errors.New("could not parse bytes_sent")
	}
	entry.BytesSent = bytesSent

	// Bytes received
	bytesRecv, err := strconv.ParseInt(quoteList[5], 10, 64)
	if err != nil {
		return entry, errors.New("could not parse bytes_recv")
	}
	entry.BytesRecv = bytesRecv

	// User agent
	entry.Agent = quoteList[6]

	return entry, nil
}
