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
	// PanicT is used for logging panic [PANIC] messages
	PanicT
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
	case PanicT:
		return "\033[1m\033[34m[PANIC]   \033[0m| "
	case SuccessT:
		return "\033[1m\033[32m[SUCCESS] \033[0m| "
	default:
		return ""
	}
}

// LogEntryT enables programmatic creation of log entries
type LogEntryT struct {
	typ     messageType
	time    time.Time
	message string
}

// NewLogEntry creates a new LogEntryT with the current time
func NewLogEntry(mt messageType, message string) LogEntryT {
	return LogEntryT{
		typ:     mt,
		time:    time.Now(),
		message: message,
	}
}

// InfoLogEntry creates a new LogEntryT with the current time and [INFO] tag
func InfoLogEntry(message string) LogEntryT {
	return NewLogEntry(InfoT, message)
}

// WarnLogEntry creates a new LogEntryT with the current time and [WARN] tag
func WarnLogEntry(message string) LogEntryT {
	return NewLogEntry(WarnT, message)
}

// ErrorLogEntry creates a new LogEntryT with the current time and [ERROR] tag
func ErrorLogEntry(message string) LogEntryT {
	return NewLogEntry(ErrorT, message)
}

// PanicLogEntry creates a new LogEntryT with the current time and [PANIC] tag
func PanicLogEntry(message string) LogEntryT {
	return NewLogEntry(PanicT, message)
}

// SuccessLogEntry creates a new LogEntryT with the current time and [SUCCESS] tag
func SuccessLogEntry(message string) LogEntryT {
	return NewLogEntry(SuccessT, message)
}

func (le LogEntryT) String() string {
	if le.message[len(le.message)-1] != '\n' {
		return fmt.Sprintf("%s %s %s\n", time.Now().Format(tm), le.typ.String(), le.message)
	}

	return fmt.Sprintf("%s %s %s", time.Now().Format(tm), le.typ.String(), le.message)
}

func logf(mt messageType, format string, v ...interface{}) {
	logger.Lock()
	if format[len(format)-1] != '\n' {
		fmt.Printf("%s %s %s\n", time.Now().Format(tm), mt.String(), fmt.Sprintf(format, v...))
	} else {
		fmt.Printf("%s %s %s", time.Now().Format(tm), mt.String(), fmt.Sprintf(format, v...))
	}
	logger.Unlock()
}

func logln(mt messageType, v ...interface{}) {
	logger.Lock()
	fmt.Printf("%s %s %s\n", time.Now().Format(tm), mt.String(), fmt.Sprint(v...))
	logger.Unlock()
}

// Infof formats a message and logs it with [INFO] tag, it adds a newline if the message didn't end with one
func Infof(format string, v ...interface{}) {
	logf(InfoT, format, v...)
}

// Info logs a message with [INFO] tag and a newline
func Info(v ...interface{}) {
	logln(InfoT, v...)
}

// WarnF formats a message and logs it with [WARN] tag, it adds a newline if the message didn't end with one
func WarnF(format string, v ...interface{}) {
	logf(WarnT, format, v...)
}

// Warn logs a message with [WARN] tag and a newline
func Warn(v ...interface{}) {
	logln(WarnT, v...)
}

// Errorf formats a message and logs it with [ERROR] tag, it adds a newline if the message didn't end with one
func Errorf(format string, v ...interface{}) {
	logf(ErrorT, format, v...)
}

// Error logs a message with [ERROR] tag and a newline
func Error(v ...interface{}) {
	logln(ErrorT, v...)
}

// Panicf formats a message and logs it with [PANIC] tag, it adds a newline if the message didn't end with one
// Note: this function does not call panic() or otherwise stops the program
func Panicf(format string, v ...interface{}) {
	logf(PanicT, format, v...)
}

// Panic logs a message with [PANIC] tag and a newline
// Note: this function does not call panic() or otherwise stops the program
func Panic(v ...interface{}) {
	logln(PanicT, v...)
}

// Successf formats a message and logs it with [SUCCESS] tag, it adds a newline if the message didn't end with one
func Successf(format string, v ...interface{}) {
	logf(SuccessT, format, v...)
}

// Success logs a message with [SUCCESS] tag and a newline
func Success(v ...interface{}) {
	logln(SuccessT, v...)
}

// Logf formats a message and logs it with provided tag, it adds a newline if the message didn't end with one
func Logf(mt messageType, format string, v ...interface{}) {
	logf(mt, format, v...)
}

// Log logs a message with provided tag and a newline
func Log(mt messageType, v ...interface{}) {
	logln(mt, v...)
}

// LogEntry logs a LogEntryT
func LogEntry(le LogEntryT) {
	logger.Lock()
	fmt.Print(le.String())
	logger.Unlock()
}
