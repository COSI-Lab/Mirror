package logging

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

var rysncErrorCodes map[int]string
var hookURL string

var loggingLock sync.Mutex

type MessageType int

const (
	// Info is the type for informational messages
	Info MessageType = iota
	// Warn is the type for warning messages
	Warn
	// Error is for when we lose funcitonality but it's fairly understood what went wrong
	Error
	// Panic is the type for fatal error messages and will print the stack trace
	Panic
	// Success is the type for successful messages
	Success
)

func Setup() error {
	hookURL = os.Getenv("HOOK_URL")
	if hookURL == "" {
		return errors.New("missing .env envirnment variable HOOK_URL, not interfacing with discord")
	}

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

	return nil
}

func Log(messageType MessageType, v ...interface{}) {
	loggingLock.Lock()
	defer loggingLock.Unlock()

	fmt.Print(time.Now().Format("2006/01/02 15:04:05 "))

	switch messageType {
	case Info:
		fmt.Print("\033[1m[INFO]    \033[0m| ")
	case Warn:
		fmt.Print("\033[1m\033[33m[WARN]   \033[0m| ")
	case Error:
		fmt.Print("\033[1m\033[31m[ERROR]   \033[0m| ")
	case Panic:
		fmt.Print("\033[1m\033[34m[PANIC]  \033[0m| ")
	case Success:
		fmt.Print("\033[1m\033[32m[SUCCESS] \033[0m| ")
	}

	fmt.Println(v...)
}
