package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

var hookURL string
var loggingLock sync.Mutex
var PING_ID string

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

func sendHook(ping bool, content ...interface{}) {
	if hookURL != "" {
		var values map[string]string
		if ping {
			values = map[string]string{"content": fmt.Sprintf("<@%s> %s", PING_ID, fmt.Sprint(content...))}
		} else {
			values = map[string]string{"content": fmt.Sprint(content...)}
		}
		json_data, err := json.Marshal(values)

		if err != nil {
			fmt.Print(err)
		}

		resp, err := http.Post(hookURL, "application/json", bytes.NewBuffer(json_data))

		if err != nil {
			fmt.Print(err)
		}

		var res map[string]interface{}

		json.NewDecoder(resp.Body).Decode((&res))
	}
}

func Setup() error {
	hookURL = os.Getenv("HOOK_URL")
	PING_ID = os.Getenv("PING_ID")
	if hookURL == "" || PING_ID == "" {
		return errors.New("missing .env variable HOOK_URL or PING_ID, not interfacing with discord")
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
		sendHook(false, fmt.Sprintf("ERROR: %s", v...))
	case PANIC:
		fmt.Print("\033[1m\033[34m[PANIC]  \033[0m| ")
		sendHook(true, fmt.Sprintf("PANIC: %s", v...))
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
