package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/COSI-Lab/datarithms"
	"github.com/COSI-Lab/logging"
	"github.com/nxadm/tail"
)

type RsyncdLogEntry struct {
	time time.Time
	sent int
	recv int
}

func ReadRsyncdLogFile(logFile string, ch chan *RsyncdLogEntry) (err error) {
	for {
		f, err := os.Open(logFile)
		if err != nil {
			return err
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			entry, err := parseRsyncdLine(scanner.Text())
			if err == nil {
				// Send a pointer to the entry down the channel
				ch <- entry
			}

			time.Sleep(10 * time.Millisecond)
		}

		f.Close()
	}
}

func TailRSyncdLogFile(logFile string, lastUpdated time.Time, ch chan *RsyncdLogEntry) {
	// Find the offset of the line where the date is past lastUpdated
	start := time.Now()
	offset, err := datarithms.BinarySearchFileByDate(logFile, lastUpdated, parseRsyncdDate)
	if err != nil {
		logging.Error(err)
		return
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
			// Send a pointer to the entry down each channel
			ch <- entry
		}
	}
}

type ParseLineError struct{}

func (e ParseLineError) Error() string {
	return "Failed to parse line"
}

func parseRsyncdDate(line string) (time.Time, error) {
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

func parseRsyncdLine(line string) (*RsyncdLogEntry, error) {
	// 2022/04/20 20:00:10 [pid] sent XXX bytes  received XXX bytes  total size XXX

	// Split the line over whitespace
	parts := strings.Split(line, " ")

	// the line we want has 14 parts
	if len(parts) != 14 {
		return nil, ParseLineError{}
	}

	// the 4th part is "sent"
	if parts[3] != "sent" {
		return nil, ParseLineError{}
	}

	// The 1st part is the date
	dt := parts[0]
	// 2nd part is the time
	tm := parts[1]

	// make the time.Time object
	t, err := time.Parse("2006/01/02 15:04:05", dt+" "+tm)
	if err != nil {
		return nil, err
	}

	// part 5 is the number of bytes sent
	sent, err := strconv.Atoi(parts[4])
	if err != nil {
		fmt.Println(err)
		return nil, ParseLineError{}
	}

	recv, err := strconv.Atoi(parts[8])
	if err != nil {
		fmt.Println(err)
		return nil, ParseLineError{}
	}

	return &RsyncdLogEntry{sent: sent, recv: recv, time: t}, nil
}
