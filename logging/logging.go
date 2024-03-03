package logging

import (
	"fmt"
	"sync"
	"time"
)

type threadSafeLogger struct {
	sync.Mutex
}

var logger = threadSafeLogger{}

type messageType int

const (
	// InfoT is used for logging informational [INFO] messages
	InfoT messageType = iota
	// WarnT is used for logging warning [WARN] messages
	WarnT
	// ErrorT is used for logging error [ERROR] messages
	ErrorT
	// SuccessT is used for logging success [SUCCESS] messages
	SuccessT
)

const tm = "2006/01/02 15:04:05"

func (mt messageType) String() string {
	switch mt {
	case InfoT:
		return "\033[1m[INFO]    \033[0m| "
	case WarnT:
		return "\033[1m\033[33m[WARN]    \033[0m| "
	case ErrorT:
		return "\033[1m\033[31m[ERROR]   \033[0m| "
	case SuccessT:
		return "\033[1m\033[32m[SUCCESS] \033[0m| "
	default:
		return ""
	}
}

// LogEntry enables programmatic creation of log entries
type LogEntry struct {
	typ     messageType
	time    time.Time
	message string
}

// NewLogEntry creates a new LogEntry with the current time
func NewLogEntry(mt messageType, message string) LogEntry {
	return LogEntry{
		typ:     mt,
		time:    time.Now(),
		message: message,
	}
}

// InfoLogEntry creates a new LogEntry with the current time and [INFO] tag
func InfoLogEntry(message string) LogEntry {
	return NewLogEntry(InfoT, message)
}

// WarnLogEntry creates a new LogEntry with the current time and [WARN] tag
func WarnLogEntry(message string) LogEntry {
	return NewLogEntry(WarnT, message)
}

// ErrorLogEntry creates a new LogEntry with the current time and [ERROR] tag
func ErrorLogEntry(message string) LogEntry {
	return NewLogEntry(ErrorT, message)
}

// SuccessLogEntry creates a new LogEntry with the current time and [SUCCESS] tag
func SuccessLogEntry(message string) LogEntry {
	return NewLogEntry(SuccessT, message)
}

func (le LogEntry) String() string {
	if le.message[len(le.message)-1] != '\n' {
		return fmt.Sprintf("%s %s %s\n", time.Now().Format(tm), le.typ.String(), le.message)
	}

	return fmt.Sprintf("%s %s %s", time.Now().Format(tm), le.typ.String(), le.message)
}

// Log prints the log entry to stdout
func (le LogEntry) Log() {
	logger.Lock()
	fmt.Print(le.String())
	logger.Unlock()
}

// Joins a `v ...any` slice into a string with spaces between each element
func join(v ...any) string {
	s := ""
	for i := 0; i < len(v); i++ {
		s += fmt.Sprint(v[i])
		if i != len(v)-1 {
			s += " "
		}
	}
	return s
}

func logf(mt messageType, format string, v ...any) {
	logger.Lock()
	if format[len(format)-1] != '\n' {
		fmt.Printf("%s %s %s\n", time.Now().Format(tm), mt.String(), fmt.Sprintf(format, v...))
	} else {
		fmt.Printf("%s %s %s", time.Now().Format(tm), mt.String(), fmt.Sprintf(format, v...))
	}
	logger.Unlock()
}

func logln(mt messageType, v ...any) {
	logger.Lock()
	fmt.Printf("%s %s %s\n", time.Now().Format(tm), mt.String(), join(v...))
	logger.Unlock()
}

// Infof formats a message and logs it with [INFO] tag, it adds a newline if the message didn't end with one
func Infof(format string, v ...any) {
	logf(InfoT, format, v...)
}

// Info logs a message with [INFO] tag and a newline
func Info(v ...any) {
	logln(InfoT, v...)
}

// Warnf formats a message and logs it with [WARN] tag, it adds a newline if the message didn't end with one
func Warnf(format string, v ...any) {
	logf(WarnT, format, v...)
}

// Warn logs a message with [WARN] tag and a newline
func Warn(v ...any) {
	logln(WarnT, v...)
}

// Errorf formats a message and logs it with [ERROR] tag, it adds a newline if the message didn't end with one
func Errorf(format string, v ...any) {
	logf(ErrorT, format, v...)
}

// Error logs a message with [ERROR] tag and a newline
func Error(v ...any) {
	logln(ErrorT, v...)
}

// Successf formats a message and logs it with [SUCCESS] tag, it adds a newline if the message didn't end with one
func Successf(format string, v ...any) {
	logf(SuccessT, format, v...)
}

// Success logs a message with [SUCCESS] tag and a newline
func Success(v ...any) {
	logln(SuccessT, v...)
}

// Logf formats a message and logs it with provided tag, it adds a newline if the message didn't end with one
func Logf(mt messageType, format string, v ...any) {
	logf(mt, format, v...)
}

// Log logs a message with provided tag and a newline
func Log(mt messageType, v ...any) {
	logln(mt, v...)
}
