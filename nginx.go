package main

import (
	"bufio"
	"errors"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nxadm/tail"
	"github.com/oschwald/geoip2-golang"
)

type LogEntry struct {
	IP        net.IP
	Country   string
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

// It is critical that NGINX uses the following log format:
// "$remote_addr" "$time_local" "$request" "$status" "$body_bytes_sent" "$request_length" "$http_user_agent";

var reQuotes *regexp.Regexp
var db *geoip2.Reader

func InitDb() (err error) {
	db, err = geoip2.Open("GeoLite2-City.mmdb")
	if err != nil {
		return err
	}

	return nil
}

// Compiles regular expressions
func InitRegex() (err error) {
	reQuotes, err = regexp.Compile(`"(.*?)"`)
	if err != nil {
		return err
	}

	return nil
}

func ReadLogFile(logFile string, ch1 chan *LogEntry, ch2 chan *LogEntry) (err error) {
	if reQuotes == nil {
		if InitRegex() != nil {
			log.Println("[ERROR] could not compile nginx log parsing regex")
		}
	}

	if db == nil {
		if InitDb() != nil {
			log.Println("[ERROR] could not initilze geolite city db")
		}
	}

	f, err := os.Open(logFile)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		entry, err := ParseLine(scanner.Text())
		if err != nil {
			log.Printf("[WARN] failed to parse line %s %s", scanner.Text(), err.Error())
		} else {
			// Send a pointer to the entry down each channel
			select {
			case ch1 <- entry:
			case ch2 <- entry:
			default:
				// TODO: Warn that a channel is starting to hang and remove sleep
				time.Sleep(1 * time.Second)
			}
		}
	}

	return nil
}

func ReadLogs(logFile string, ch1 chan *LogEntry, ch2 chan *LogEntry) (err error) {
	if reQuotes == nil {
		if InitRegex() != nil {
			log.Println("[ERROR] could not compile nginx log parsing regex")
		}
	}

	if db == nil {
		if InitDb() != nil {
			log.Println("[ERROR] could not initilze geolite city db")
		}
	}

	// Tail the log file `tail -F`
	tail, err := tail.TailFile(logFile, tail.Config{Follow: true, ReOpen: true})
	if err != nil {
		return err
	}

	for line := range tail.Lines {
		entry, err := ParseLine(line.Text)
		if err != nil {
			log.Printf("[WARN] failed to parse line %s | %s", line.Text, err.Error())
		} else {
			// Send a pointer to the entry down each channel
			select {
			case ch1 <- entry:
			case ch2 <- entry:
			default:
				// TODO: Warn that a channel is starting to hang
			}
		}
	}

	log.Println("[ERROR] Closing ReadLogs *LogEntry channel for unknown reason. This should not happen!")
	close(ch1)
	close(ch2)

	return nil
}

func ParseLine(line string) (*LogEntry, error) {
	// "$remote_addr" "$time_local" "$request" "$status" "$body_bytes_sent" "$request_length" "$http_user_agent";
	quoteList := reQuotes.FindAllString(line, -1)

	// Trim quotation marks
	for i := 0; i < len(quoteList); i++ {
		quoteList[i] = quoteList[i][1 : len(quoteList[i])-1]
	}

	if len(quoteList) != 7 {
		return nil, errors.New("invalid number of parameters in log")
	}

	var entry LogEntry

	// IPv4 or IPv6 address
	entry.IP = net.ParseIP(quoteList[0])
	if entry.IP == nil {
		return nil, errors.New("failed to parse ip")
	}

	dbResult, err := db.City(entry.IP)
	if err != nil {
		return nil, err
	}
	entry.Country = dbResult.Country.IsoCode
	entry.City = dbResult

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
	entry.Distro = strings.Split(entry.Url, "/")[1]

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
