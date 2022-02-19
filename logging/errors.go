package logging

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

var hookURL string
var loggingLock sync.Mutex

type MessageType int

const (
	// Info is the type for informational messages
	INFO MessageType = iota
	// Warn is the type for warning messages
	WARN
	// Error is for when we lose funcitonality but it's fairly understood what went wrong
	ERROR
	// Panic is the type for fatal error messages and will print the stack trace
	PANIC
	// Success is the type for successful messages
	SUCCESS
)

func Setup() error {
	hookURL = os.Getenv("HOOK_URL")
	if hookURL == "" {
		return errors.New("missing .env variable HOOK_URL, not interfacing with discord")
	}

	return nil
}

func log(messageType MessageType, v ...interface{}) {
	loggingLock.Lock()
	fmt.Print(time.Now().Format("2006/01/02 15:04:05 "))

	switch messageType {
	case INFO:
		fmt.Print("\033[1m[INFO]    \033[0m| ")
	case WARN:
		fmt.Print("\033[1m\033[33m[WARN]    \033[0m| ")
	case ERROR:
		fmt.Print("\033[1m\033[31m[ERROR]   \033[0m| ")
	case PANIC:
		fmt.Print("\033[1m\033[34m[PANIC]  \033[0m| ")
	case SUCCESS:
		fmt.Print("\033[1m\033[32m[SUCCESS] \033[0m| ")
	}

	fmt.Println(v...)
	loggingLock.Unlock()
}

func Info(v ...interface{}) {
	log(INFO, v...)
}

func Warn(v ...interface{}) {
	log(WARN, v...)
}

func Error(v ...interface{}) {
	log(ERROR, v...)
}

func Panic(v ...interface{}) {
	log(PANIC, v...)
}

func Success(v ...interface{}) {
	log(SUCCESS, v...)
}
