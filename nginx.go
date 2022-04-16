package main

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/COSI_Lab/Mirror/logging"
	"github.com/nxadm/tail"
	"github.com/oschwald/geoip2-golang"
)

// It is critical that NGINX uses the following log format:
/*
 * log_format config '"$remote_addr" "$time_local" "$request" "$status" "$body_bytes_sent" "$request_length" "$http_user_agent"';
 * access_log /var/log/nginx/access.log config;
 */

// LogEntry is a struct that represents a parsed nginx log entry
type LogEntry struct {
	IP        net.IP
	City      *geoip2.City
	Time      time.Time
	Method    string
	Distro    string
	Url       string
	Version   string
	Status    int
	BytesSent int
	BytesRecv int
	Agent     string
}

var reQuotes = regexp.MustCompile(`"(.*?)"`)

// ReadLogFile is a testing function that simulates tailing a log file by reading it line by line with some delay between lines
func ReadLogFile(logFile string, channels ...chan *LogEntry) (err error) {
	if reQuotes == nil {
		logging.Warn("regexp is nil")
	}

	for {
		f, err := os.Open(logFile)
		if err != nil {
			return err
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			entry, err := ParseLine(scanner.Text())
			if err == nil {
				// Send a pointer to the entry down each channel
				for ch := range channels {
					select {
					case channels[ch] <- entry:
					default:
					}
				}
			}

			time.Sleep(100 * time.Millisecond)
		}

		f.Close()
	}
}

// ReadLogs tails a log file and sends the parsed log entries to the specified channels
func ReadLogs(logFile string, lastUpdated time.Time, channels ...chan *LogEntry) {
	// Find the offset of the line in which the date is past lastUpdated
	start := time.Now()
	offset, err := findOffset(logFile, lastUpdated)

	if err != nil {
		logging.Warn("ReadLogs failed to find offset", err)
		return
	} else {
		logging.Info("Found log file offset in", time.Since(start))
	}

	seek := tail.SeekInfo{
		Offset: offset,
		Whence: io.SeekStart,
	}

	// Tail the log file `tail -F`
	tail, err := tail.TailFile(logFile, tail.Config{Follow: true, ReOpen: true, MustExist: true, Location: &seek})
	if err != nil {
		logging.Warn("TailFile failed to start: ", err)
		return
	}

	logging.Success("Tailing nginx log file")

	for line := range tail.Lines {
		entry, err := ParseLine(line.Text)

		if err == nil {
			// Send a pointer to the entry down each channel
			for _, ch := range channels {
				select {
				case ch <- entry:
				default:
					// TODO - right now we're just dropping the log entries if the channels are full.
					// We should probably do something more intelligent here.
				}
			}
		}
	}

	// Tailing this file should never end
	logging.Panic("No longer reading log file", tail.Err())
}

// findOffset returns the offset of the log entry which has the smallest date past lastUpdated
// Since parsing a line is very expensive we use a memory vs time tradeoff
// First we quickly scan the file line by line to calculate each line's seek offset
// Then we preform a binary search on the first line that has a date after lastUpdated
func findOffset(logFile string, lastUpdated time.Time) (offset int64, err error) {
	f, err := os.Open(logFile)
	if err != nil {
		return 0, err
	}

	// Load all of the line offsets into memory
	lines := make([]int64, 0)

	// Precalculate the seek offset of each line
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, offset)
		offset += int64(len(scanner.Bytes())) + 1
	}

	// Run a binary search for the first line which is past the lastUpdated
	start := 0
	end := len(lines) - 1
	for start < end {
		mid := (start + end) / 2

		// Get the text of the line
		f.Seek(lines[mid], io.SeekStart)
		scanner := bufio.NewScanner(f)
		scanner.Scan()
		line := scanner.Text()

		tm, err := ParseDate(line)
		if err != nil {
			return 0, err
		}

		if tm.After(lastUpdated) {
			end = mid
		} else {
			start = mid + 1
		}
	}

	return lines[start], nil
}

// ParseDate parses a single line of the nginx log file and returns the time.Time of the line
func ParseDate(line string) (time.Time, error) {
	// "$remote_addr" "$time_local" "$request" "$status" "$body_bytes_sent" "$request_length" "$http_user_agent";
	quoteList := reQuotes.FindAllString(line, -1)

	if len(quoteList) != 7 {
		return time.Time{}, errors.New("invalid number of quotes")
	}

	// Time
	t := "\"02/Jan/2006:15:04:05 -0700\""
	tm, err := time.Parse(t, quoteList[1])
	if err != nil {
		return time.Time{}, err
	}

	return tm, nil
}

// ParseLine parses a single line of the nginx log file
// It's critical the log file uses the correct format found at the top of this file
// If the log file is not in the correct format or if some other part of the parsing fails
// this function will return an error
func ParseLine(line string) (*LogEntry, error) {
	// "$remote_addr" "$time_local" "$request" "$status" "$body_bytes_sent" "$request_length" "$http_user_agent";
	quoteList := reQuotes.FindAllString(line, -1)

	if len(quoteList) != 7 {
		return nil, errors.New("invalid number of parameters in log entry")
	}

	// Trim quotation marks
	for i := 0; i < len(quoteList); i++ {
		quoteList[i] = quoteList[i][1 : len(quoteList[i])-1]
	}

	var entry LogEntry
	var err error

	// IPv4 or IPv6 address
	entry.IP = net.ParseIP(quoteList[0])
	if entry.IP == nil {
		return nil, errors.New("failed to parse ip")
	}

	// Optional GeoIP lookup
	if geoip != nil {
		entry.City = geoip.GetGeoIP(entry.IP)
	} else {
		entry.City = nil
	}

	// Time
	t := "02/Jan/2006:15:04:05 -0700"
	tm, err := time.Parse(t, quoteList[1])
	if err != nil {
		return nil, err
	}
	entry.Time = tm

	// Method url http version
	split := strings.Split(quoteList[2], " ")
	if len(split) != 3 {
		// this should never fail
		return nil, errors.New("invalid number of strings in request")
	}
	entry.Method = split[0]
	entry.Url = split[1]
	entry.Version = split[2]

	// Distro is the top level of the URL path
	split = strings.Split(entry.Url, "/")

	if len(split) >= 2 {
		entry.Distro = split[1]
	} else {
		return nil, errors.New("invalid number of parts in url")
	}

	// HTTP response status
	status, err := strconv.Atoi(quoteList[3])
	if err != nil {
		// this should never fail
		return nil, errors.New("could not parse http response status")
	}
	entry.Status = status

	// Bytes sent
	bytesSent, err := strconv.Atoi(quoteList[4])
	if err != nil {
		// this should never fail
		return nil, errors.New("could not parse bytes_sent")
	}
	entry.BytesSent = bytesSent

	// Bytes received
	bytesRecv, err := strconv.Atoi(quoteList[5])
	if err != nil {
		return nil, errors.New("could not parse bytes_recv")
	}
	entry.BytesRecv = bytesRecv

	// User agent
	entry.Agent = quoteList[6]

	return &entry, nil
}
