package mirrorErrors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

var rysncErrorCodes map[int]string
var hookURL string

func setup() {
	hookURL = os.Getenv("HOOK_URL")

	rysncErrorCodes = make(map[int]string)

	rysncErrorCodes[0] = "Success"
	rysncErrorCodes[1] = "Syntax or usage error"
	rysncErrorCodes[2] = "Protocol incompatibility"
	rysncErrorCodes[3] = "Errors selecting input/output files, dirs"
	rysncErrorCodes[4] = "Requested action not supported: an attempt was made to manipulate 64-bit files on a platform that cannot support them; or an option was specified that is supported by the client and not by the server."
	rysncErrorCodes[5] = "Error starting client-server protocol"
	rysncErrorCodes[6] = "Daemon unable to append to log-file"
	rysncErrorCodes[10] = "Error in socket I/O"
	rysncErrorCodes[11] = "Error in file I/O"
	rysncErrorCodes[12] = "Error in rsync protocol data stream"
	rysncErrorCodes[13] = "Errors with program diagnostics"
	rysncErrorCodes[14] = "Error in IPC code"
	rysncErrorCodes[20] = "Received SIGUSR1 or SIGINT"
	rysncErrorCodes[21] = "Some error returned by waitpid()"
	rysncErrorCodes[22] = "Error allocating core memory buffers"
	rysncErrorCodes[23] = "Partial transfer due to error"
	rysncErrorCodes[24] = "Partial transfer due to vanished source files"
	rysncErrorCodes[25] = "The --max-delete limit stopped deletions"
	rysncErrorCodes[30] = "Timeout in data send/receive"
	rysncErrorCodes[35] = "Timeout waiting for daemon connection"
}

func sendHook(content string, url string) {
	values := map[string]string{"content": content}
	json_data, err := json.Marshal(values)

	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.Post(url, "application/json",
		bytes.NewBuffer(json_data))

	if err != nil {
		log.Fatal(err)
	}

	var res map[string]interface{}

	json.NewDecoder(resp.Body).Decode(&res)

	fmt.Println(res["json"])
}

func Error(message string, errorType string) {
	// TODO: Have this handle logging to console and send hook
	if hookURL == "" {
		setup()
	}
	if errorType == "info" {
		log.Printf("[INFO] %s", message)
	} else if errorType == "warn" {
		log.Printf("\x1B[WARN]\x1B %s", message)
	} else if errorType == "error" {
		log.Printf("\x1B[ERROR]\x1B %s", message)
		sendHook(message, hookURL)
	} else if errorType == "panic" {
		log.Printf("[PANIC] %s", message)
		sendHook(message, hookURL)
	} else {
		log.Printf("[DEBUG] %s", message)
	}
}
