package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"sync"
	"time"
)

type MessageType int

const (
	typeInfo MessageType = iota
	typeWarning
	typeError
	typePanic
	typeSuccess
)

var logger = threadSafeLogger{}

type threadSafeLogger struct {
	sync.Mutex
	sendHooks      bool
	discordHookURL string
	discordPingID  string
}

// Setup initialize the variables for calling webhooks
// TODO: Make this threadsafe so it can be reloadable with sighup
func Setup(hookURL string, pingID string) {
	logger.Lock()
	logger.discordHookURL = hookURL
	logger.discordPingID = pingID
	logger.sendHooks = hookURL != "" && pingID != ""
	logger.Unlock()
}

// sendFile creates a multipart form message and sends it to the specified URL
// with the specified content as a file attachment
func sendFile(content []byte) {
	logger.Lock()
	defer logger.Unlock()

	if !logger.sendHooks {
		return
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add a file attachment to the multipart writer
	part, err := writer.CreateFormFile("text", "attachment.txt")
	if err != nil {
		fmt.Println(time.Now().Format("2006/01/02 15:04:05 "), "\033[1m\033[31m[ERROR]   \033[0m| ", err)
		return
	}
	part.Write(content)
	writer.Close()

	// Build the request
	request, err := http.NewRequest("POST", logger.discordHookURL, body)
	if err != nil {
		fmt.Println(time.Now().Format("2006/01/02 15:04:05 "), "\033[1m\033[31m[ERROR]   \033[0m| ", err)
		return
	}
	request.Header.Add("Content-Type", writer.FormDataContentType())

	// Execute the request
	client := &http.Client{}
	response, err := client.Do(request)
	response.Body.Close()

	if err != nil {
		fmt.Println(time.Now().Format("2006/01/02 15:04:05 "), "\033[1m\033[31m[ERROR]   \033[0m| ", err)
		return
	}
}

func sendHook(ping bool, content ...interface{}) {
	logger.Lock()
	defer logger.Unlock()

	if !logger.sendHooks {
		return
	}

	var values map[string]string
	if ping {
		values = map[string]string{"content": fmt.Sprintf("<@%s> PANIC: %v", logger.discordPingID, fmt.Sprintf("%s", content...))}
	} else {
		values = map[string]string{"content": fmt.Sprintf("ERROR: %v", fmt.Sprint(content...))}
	}
	json_data, err := json.Marshal(values)
	if err != nil {
		fmt.Println(time.Now().Format("2006/01/02 15:04:05 "), "\033[1m\033[31m[ERROR]   \033[0m| ", err)
		return
	}

	send := bytes.NewBuffer(json_data)
	_, err = http.Post(logger.discordHookURL, "application/json", send)
	if err != nil {
		fmt.Println(time.Now().Format("2006/01/02 15:04:05 "), "\033[1m\033[31m[ERROR]   \033[0m| ", err)
		return
	}
}

func log(messageType MessageType, v ...interface{}) {
	logger.Lock()
	fmt.Print(time.Now().Format("2006/01/02 15:04:05 "))

	switch messageType {
	case typeInfo:
		fmt.Print("\033[1m[INFO]    \033[0m| ")
	case typeWarning:
		fmt.Print("\033[1m\033[33m[WARN]    \033[0m| ")
	case typeError:
		fmt.Print("\033[1m\033[31m[ERROR]   \033[0m| ")
		go sendHook(false, v...)
	case typePanic:
		fmt.Print("\033[1m\033[34m[PANIC]  \033[0m| ")
		go sendHook(true, v...)
	case typeSuccess:
		fmt.Print("\033[1m\033[32m[SUCCESS] \033[0m| ")
	}

	fmt.Println(v...)
	logger.Unlock()
}

func logWithAttachment(messageType MessageType, attachment []byte, message ...interface{}) {
	logger.Lock()
	fmt.Print(time.Now().Format("2006/01/02 15:04:05 "))

	switch messageType {
	case typeInfo:
		fmt.Print("\033[1m[INFO]    \033[0m| ")
	case typeWarning:
		fmt.Print("\033[1m\033[33m[WARN]    \033[0m| ")
	case typeError:
		fmt.Print("\033[1m\033[31m[ERROR]   \033[0m| ")
		go func() {
			// TODO handle error returned by sendFile
			sendFile(attachment)
			sendHook(false, message...)
		}()
	case typePanic:
		fmt.Print("\033[1m\033[34m[PANIC]  \033[0m| ")
		go func() {
			// TODO handle error returned by sendFile
			sendFile(attachment)
			sendHook(true, message...)
		}()
	case typeSuccess:
		fmt.Print("\033[1m\033[32m[SUCCESS] \033[0m| ")
	}

	fmt.Println(message...)
	logger.Unlock()
}

func Info(v ...interface{}) {
	log(typeInfo, v...)
}

func Warn(v ...interface{}) {
	log(typeWarning, v...)
}

func Error(v ...interface{}) {
	log(typeError, v...)
}

func ErrorWithAttachment(attachment []byte, v ...interface{}) {
	logWithAttachment(typeError, attachment, v...)
}

func Panic(v ...interface{}) {
	log(typePanic, v...)
}

func PanicWithAttachment(attachment []byte, v ...interface{}) {
	logWithAttachment(typePanic, attachment, v...)
}

func Success(v ...interface{}) {
	log(typeSuccess, v...)
}
