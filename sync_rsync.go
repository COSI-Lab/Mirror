package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/COSI-Lab/Mirror/config"
	"github.com/COSI-Lab/Mirror/logging"
)

var rsyncErrorCodes map[int]string

func init() {
	rsyncErrorCodes = make(map[int]string)
	rsyncErrorCodes[0] = "Success"
	rsyncErrorCodes[1] = "Syntax or usage error"
	rsyncErrorCodes[2] = "Protocol incompatibility"
	rsyncErrorCodes[3] = "Errors selecting input/output files, dirs"
	rsyncErrorCodes[4] = "Requested action not supported: an attempt was made to manipulate 64-bit files on a platform that cannot support them; or an option was specified that is supported by the client and not by the server."
	rsyncErrorCodes[5] = "Error starting client-server protocol"
	rsyncErrorCodes[6] = "Daemon unable to append to log-file"
	rsyncErrorCodes[10] = "Error in socket I/O"
	rsyncErrorCodes[11] = "Error in file I/O"
	rsyncErrorCodes[12] = "Error in rsync protocol data stream"
	rsyncErrorCodes[13] = "Errors with program diagnostics"
	rsyncErrorCodes[14] = "Error in IPC code"
	rsyncErrorCodes[20] = "Received SIGUSR1 or SIGINT"
	rsyncErrorCodes[21] = "Some error returned by waitpid()"
	rsyncErrorCodes[22] = "Error allocating core memory buffers"
	rsyncErrorCodes[23] = "Partial transfer due to error"
	rsyncErrorCodes[24] = "Partial transfer due to vanished source files"
	rsyncErrorCodes[25] = "The --max-delete limit stopped deletions"
	rsyncErrorCodes[30] = "Timeout in data send/receive"
	rsyncErrorCodes[35] = "Timeout waiting for daemon connection"
}

// RSYNCErrorCodeToString converts an rsync error code to a string
// If the error code is not known, it returns "Unknown"
func RSYNCErrorCodeToString(code int) string {
	if msg, ok := rsyncErrorCodes[code]; ok {
		return msg
	}

	return "Unknown"
}

// RSYNCTask implements the Task interface from `scheduler`
type RSYNCTask struct {
	// Project `short` name
	short    string
	args     []string
	stages   []string
	password string
}

// NewRSYNCTask creates a new RsyncTask from a config.Rsync
func NewRSYNCTask(declaration *config.Rsync, short string) *RSYNCTask {
	args := make([]string, 0)

	if declaration.User != "" {
		args = append(args, fmt.Sprintf("%s@%s::%s", declaration.User, declaration.Host, declaration.Src))
	} else {
		args = append(args, fmt.Sprintf("%s::%s", declaration.Host, declaration.Src))
	}
	args = append(args, declaration.Dest)

	// Add the password if it exists
	var password []byte
	var err error
	if declaration.PasswordFile != "" {
		password, err = os.ReadFile(declaration.PasswordFile)
		if err != nil {
			logging.Error("Failed to read password file:", err)
		}

		return &RSYNCTask{
			short:    short,
			args:     args,
			stages:   declaration.Stages,
			password: string(password),
		}
	}

	return &RSYNCTask{
		short:    short,
		args:     args,
		stages:   declaration.Stages,
		password: "",
	}
}

// Run runs the script, blocking until it finishes
func (r *RSYNCTask) Run(ctx context.Context, stdout, stderr io.Writer, status chan<- logging.LogEntry) TaskStatus {
	status <- logging.InfoLogEntry(fmt.Sprintf("%s: Starting rsync", r.short))

	for i := 0; i < len(r.stages); i++ {
		status := r.RunStage(ctx, stdout, stderr, status, i)
		if status != TaskStatusSuccess {
			return status
		}
	}

	return TaskStatusSuccess
}

// RunStage runs a single stage of the rsync task
func (r *RSYNCTask) RunStage(ctx context.Context, stdout, stderr io.Writer, status chan<- logging.LogEntry, stage int) TaskStatus {
	// join r.args and r.stages[stage]
	args := make([]string, len(r.args))
	copy(args, r.args)
	args = append(args, r.stages[stage])

	cmd := exec.Command("rsync", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if r.password != "" {
		cmd.Env = append(os.Environ(), "RSYNC_PASSWORD="+r.password)
	}

	status <- logging.InfoLogEntry("Running: " + cmd.String())

	err := cmd.Start()
	if err != nil {
		status <- logging.ErrorLogEntry(fmt.Sprintf("%s: Stage %d failed to start: %s", r.short, stage, err.Error()))
		return TaskStatusFailure
	}

	c := make(chan struct{})
	go func() {
		cmd.Wait()
		close(c)
	}()

	select {
	case <-c:
		break
	case <-ctx.Done():
		cmd.Process.Kill()
		status <- logging.InfoLogEntry(fmt.Sprintf("%s: Stage %d stopped", r.short, stage))
		return TaskStatusStopped
	}

	// Report the exit code
	if cmd.ProcessState.Success() {
		status <- logging.SuccessLogEntry(fmt.Sprintf("%s: Stage %d completed successfully", r.short, stage))
		return TaskStatusSuccess
	}

	status <- logging.ErrorLogEntry(fmt.Sprintf("%s: Stage %d failed with exit code %d (%s)", r.short, stage, cmd.ProcessState.ExitCode(), RSYNCErrorCodeToString(cmd.ProcessState.ExitCode())))
	return TaskStatusFailure
}
