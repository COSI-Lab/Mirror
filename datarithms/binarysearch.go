package datarithms

import (
	"bufio"
	"io"
	"os"
	"time"
)

// BinarySearchFileByDate returns the offset of the first line which date is after lastUpdated
// this could panic if the parse function isn't guaranteed to work on every line of the file
func BinarySearchFileByDate(file string, lastUpdated time.Time, parse func(line string) (time.Time, error)) (offset int64, err error) {
	// Open the log file
	f, err := os.Open(file)
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

		tm, err := parse(line)
		if err != nil {
			// if for some reason we can't parse the line we increment start and try again
			start = mid + 1
			continue
		}

		if tm.After(lastUpdated) {
			end = mid
		} else {
			start = mid + 1
		}
	}

	return lines[start], nil
}
