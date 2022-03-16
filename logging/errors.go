package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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

type fileHook struct {
	content string
	file    []byte
}

func sendFile(url string, file []byte) []byte {
	// f, _ := ioutil.TempFile("", "logging")
	f, err := os.CreateTemp("", "logging")
	if err != nil {
		fmt.Print(time.Now().Format("2006/01/02 15:04:05 "))
		fmt.Print("\033[1m\033[31m[ERROR]   \033[0m| ")
		fmt.Println(err)
	}

	defer f.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("text", filepath.Base(f.Name()))
	if err != nil {
		fmt.Print(time.Now().Format("2006/01/02 15:04:05 "))
		fmt.Print("\033[1m\033[31m[ERROR]   \033[0m| ")
		fmt.Println(err)
	}

	part.Write(file)
	writer.Close()
	request, err := http.NewRequest("POST", url, body)
	if err != nil {
		fmt.Print(time.Now().Format("2006/01/02 15:04:05 "))
		fmt.Print("\033[1m\033[31m[ERROR]   \033[0m| ")
		fmt.Println(err)
	}

	request.Header.Add("Content-Type", writer.FormDataContentType())
	client := &http.Client{}

	response, err := client.Do(request)
	if err != nil {
		fmt.Print(time.Now().Format("2006/01/02 15:04:05 "))
		fmt.Print("\033[1m\033[31m[ERROR]   \033[0m| ")
		fmt.Println(err)
	}

	defer response.Body.Close()

	content, _ := io.ReadAll(response.Body)
	f.Close()
	os.Remove(f.Name())

	return content
}

func sendHook(ping bool, content ...interface{}) {
	if hookURL != "" {
		var values map[string]string
		if ping {
			values = map[string]string{"content": fmt.Sprintf("<@%s> PANIC: %v", PING_ID, fmt.Sprintf("%s", content...))}
		} else {
			values = map[string]string{"content": fmt.Sprintf("ERROR: %v", fmt.Sprint(content...))}
		}
		json_data, err := json.Marshal(values)

		if err != nil {
			fmt.Print(err)
		}

		send := bytes.NewBuffer(json_data)

		resp, err := http.Post(hookURL, "application/json", send)

		fmt.Println(send)

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
		go sendHook(false, v...)
	case PANIC:
		fmt.Print("\033[1m\033[34m[PANIC]  \033[0m| ")
		go sendHook(true, v...)
	case SUCCESS:
		fmt.Print("\033[1m\033[32m[SUCCESS] \033[0m| ")
	}

	fmt.Println(v...)
	loggingLock.Unlock()
}

func sendHookWithFile(ping bool, attachment []byte, content ...interface{}) {
	if hookURL != "" {
		var values map[string]string
		if ping {
			values = map[string]string{"content": fmt.Sprintf("<@%s> PANIC: %v", PING_ID, fmt.Sprintf("%s", content...))}
		} else {
			values = map[string]string{"content": fmt.Sprintf("ERROR: %v", fmt.Sprint(content...))}
		}
		json_data, err := json.Marshal(values)
		sendFile(hookURL, attachment)

		if err != nil {
			fmt.Print(err)
		}

		send := bytes.NewBuffer(json_data)

		resp, err := http.Post(hookURL, "application/json", send)

		fmt.Println(send)

		if err != nil {
			fmt.Print(err)
		}

		var res map[string]interface{}

		json.NewDecoder(resp.Body).Decode((&res))
	}
}

func logWithAttachment(messageType MessageType, attachment []byte, message ...interface{}) {
	loggingLock.Lock()
	fmt.Print(time.Now().Format("2006/01/02 15:04:05 "))

	switch messageType {
	case INFO:
		fmt.Print("\033[1m[INFO]    \033[0m| ")
	case WARN:
		fmt.Print("\033[1m\033[33m[WARN]    \033[0m| ")
	case ERROR:
		fmt.Print("\033[1m\033[31m[ERROR]   \033[0m| ")
		go sendHookWithFile(false, attachment, message...)
	case PANIC:
		fmt.Print("\033[1m\033[34m[PANIC]  \033[0m| ")
		go sendHookWithFile(true, attachment, message...)
	case SUCCESS:
		fmt.Print("\033[1m\033[32m[SUCCESS] \033[0m| ")
	}

	fmt.Println(message...)
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

func ErrorWithAttachment(attachment []byte, v ...interface{}) {
	logWithAttachment(ERROR, attachment, v...)
}

func Panic(v ...interface{}) {
	log(PANIC, v...)
}

func PanicWithAttachment(attachment []byte, v ...interface{}) {
	logWithAttachment(PANIC, attachment, v...)
}

func Success(v ...interface{}) {
	log(SUCCESS, v...)
}
