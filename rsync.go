package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/COSI_Lab/Mirror/datarithms"
	"github.com/COSI_Lab/Mirror/logging"
)

var rysncErrorCodes map[int]string
var rsyncLocks map[string]bool

type RSYNCStatus struct {
	sync.RWMutex
	Status map[string]*datarithms.CircularQueue
}

func rsync(project Project) ([]byte, *os.ProcessState) {
	// split up the options TODO maybe precompute this?
	args := strings.Split(project.Rsync.Options, " ")
	args = append(args, project.Rsync.Host+"::"+project.Rsync.Src)
	args = append(args, project.Rsync.Dest)

	command := exec.Command("rsync", args...)
	output, err := command.CombinedOutput()

	if err != nil {
		logging.Warn("Combined output call failed", err)
	}

	return output, command.ProcessState
}

func initRSYNC(config ConfigFile) {
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

	// Create the rsync lock map
	rsyncLocks = make(map[string]bool)
	for _, project := range config.Mirrors {
		rsyncLocks[project.Short] = false
	}
}

func appendToLogFile(short string, data []byte) {
	// Get month
	month := fmt.Sprintf("%02d", time.Now().UTC().Month())

	// Open the log file TODO load path from env
	path := "/tmp/mirror/" + short + "-" + month + ".log"
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		logging.Warn("failed to open log file ", path, err)
	}

	// Write to the log file
	_, err = file.Write(data)
	if err != nil {
		logging.Warn("failed to write to log file ", path, err)
	}
}

func handleRSYNC(config ConfigFile, status *RSYNCStatus) {
	status.Lock()
	for _, mirror := range config.Mirrors {
		if mirror.Rsync.SyncsPerDay > 0 {
			status.Status[mirror.Short] = datarithms.CircularQueueInit(7 * mirror.Rsync.SyncsPerDay)
		}
	}
	status.Unlock()

	// prepare the tasks
	tasks := make([]datarithms.Task, 0, len(config.Mirrors))
	for _, mirror := range config.Mirrors {
		if mirror.Rsync.Host != "" {
			tasks = append(tasks, datarithms.Task{
				Short: mirror.Short,
				Syncs: mirror.Rsync.SyncsPerDay,
			})
		}
	}

	// build the schedule
	schedule := datarithms.BuildSchedule(tasks)

	// run the schedule
	for {
		short, sleep := schedule.NextJob()

		logging.Info("Running job rsync: " + short)

		// TODO find project using a map instead
		for _, project := range config.Mirrors {
			if project.Short == short {
				go func() {
					// Lock the project
					if rsyncLocks[short] {
						logging.Warn("rsync is already running for ", short)
						return
					}
					rsyncLocks[short] = true

					b, state := rsync(project)

					// track the status for the API
					status.Lock()
					status.Status[short].Push(string(b))
					status.Unlock()

					// append status to its log file
					appendToLogFile(short, b)

					// check if the process exited with an error
					if state != nil && state.Success() {
						logging.Success("Job rsync:", short, "finished successfully")
					} else {
						// We have some human readable error descriptions
						if meaning, ok := rysncErrorCodes[state.ExitCode()]; ok {
							logging.Warn("Job rsync:", short, "failed. Exit code:", state.ExitCode(), meaning)
						} else {
							logging.Warn("Job rsync:", short, "failed. Exit code:", state.ExitCode())
						}
					}

					// Unlock the project
					rsyncLocks[short] = false
				}()

				break
			}
		}

		logging.Info("Sleeping for " + time.Duration(sleep).String())
		time.Sleep(sleep)
	}
}
