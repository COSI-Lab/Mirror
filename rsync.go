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

type Status struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`
	ExitCode  int   `json:"exitCode"`
}
type RSYNCStatus map[string]*datarithms.CircularQueue[Status]

func init() {
	logging.Info("Initializing rsync error codes")

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

	// Create the log directory
	if rsyncLogs != "" {
		err := os.MkdirAll(rsyncLogs, 0755)

		if err != nil {
			logging.Error("failed to create RSYNC_LOGS directory", rsyncLogs, err, "not saving rsync logs")
			rsyncLogs = ""
		} else {
			logging.Success("opened RSYNC_LOGS directory", rsyncLogs)
		}
	}
}

func rsync(project *Project, options string) ([]byte, *os.ProcessState) {
	// split up the options TODO maybe precompute this?
	args := strings.Split(options, " ")

	// Run with dry run if specified
	if rsyncDryRun {
		args = append(args, "--dry-run")
		logging.Info("Syncing", project.Short, "with --dry-run")
	}

	// Set the source and destination
	if project.Rsync.User != "" {
		args = append(args, fmt.Sprintf("%s@%s::%s", project.Rsync.User, project.Rsync.Host, project.Rsync.Src))
	} else {
		args = append(args, fmt.Sprintf("%s::%s", project.Rsync.Host, project.Rsync.Src))
	}
	args = append(args, project.Rsync.Dest)

	command := exec.Command("rsync", args...)

	// Add the password environment variable if needed
	if project.Rsync.Password != "" {
		command.Env = append(os.Environ(), "RSYNC_PASSWORD="+project.Rsync.Password)
	}

	logging.Info(command)

	output, err := command.CombinedOutput()

	if err != nil {
		logging.Warn("Combined output call failed", err)
	}

	return output, command.ProcessState
}

func appendToLogFile(short string, data []byte) {
	// Get month
	month := fmt.Sprintf("%02d", time.Now().UTC().Month())

	// Open the log file
	path := rsyncLogs + "/" + short + "-" + month + ".log"
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

// handleRSYNC is the main rsync scheduler
// It builds a schedule of when to sync projects in such a way they are equaly spaced across the day
// rsync tasks are run in a separate goroutine to avoid blocking this one
// Status for the API is saved `status`
func handleRSYNC(config *ConfigFile, status RSYNCStatus, stop chan struct{}) {
	for _, mirror := range config.Mirrors {
		if mirror.Rsync.SyncsPerDay > 0 {
			// Store a weeks worth of status messages
			status[mirror.Short] = datarithms.CircularQueueInit[Status](7 * mirror.Rsync.SyncsPerDay)
		}
	}

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

	// error checking on the schedule
	if !datarithms.Verify(schedule, tasks) {
		// A "warn" should do because a human should always be watching this when it's called
		logging.Warn("RSYNC schedule fails verification")
	}

	// a project can only be syncing once at a time
	rsyncLock := sync.Mutex{}
	rsyncLocks := make(map[string]bool)
	for _, project := range config.Mirrors {
		rsyncLocks[project.Short] = false
	}

	// skip the first job
	_, sleep := schedule.NextJob()
	timer := time.NewTimer(sleep)

	logging.Success("RSYNC scheduler started, next sync in", sleep)

	// run the schedule
	for {
		select {
		case <-stop:
			logging.Info("RSYNC scheduler stopping...")
			timer.Stop()

			// Wait for all the rsync tasks to finish
			for {
				// Check if all the rsync tasks are done
				rsyncLock.Lock()
				allDone := true
				for _, running := range rsyncLocks {
					if running {
						allDone = false
						break
					}
				}
				rsyncLock.Unlock()

				// If all the rsync tasks are done, break
				if allDone {
					break
				}

				time.Sleep(time.Second)
			}

			// Respond to the stop signal
			stop <- struct{}{}
			return
		case <-timer.C:
			short, sleep := schedule.NextJob()
			timer.Reset(sleep + time.Second)

			go func() {
				logging.Info("Running job: rsync", short)

				// Lock the project
				rsyncLock.Lock() // start critical section
				if rsyncLocks[short] {
					rsyncLock.Unlock() // end critical section
					logging.Warn("rsync is already running for ", short)
					return
				}
				rsyncLocks[short] = true
				rsyncLock.Unlock() // end critical section

				start := time.Now()

				// 1 stage syncs are the norm
				output1, state1 := rsync(config.Mirrors[short], config.Mirrors[short].Rsync.Options)
				status[short].Push(Status{StartTime: start.Unix(), EndTime: time.Now().Unix(), ExitCode: state1.ExitCode()})

				// append stage 1 to its log file
				if rsyncLogs != "" {
					appendToLogFile(short, []byte("\n\n"+start.Format(time.RFC1123)+"\n"))
					appendToLogFile(short, output1)
				}

				checkState(short, state1)

				// 2 stage syncs happen sometimes
				if config.Mirrors[short].Rsync.Second != "" {
					start = time.Now()
					output2, state2 := rsync(config.Mirrors[short], config.Mirrors[short].Rsync.Second)
					status[short].Push(Status{StartTime: start.Unix(), EndTime: time.Now().Unix(), ExitCode: state2.ExitCode()})

					if rsyncLogs != "" {
						appendToLogFile(short, []byte("\n\n"+start.Format(time.RFC1123)+"\n"))
						appendToLogFile(short, output2)
					}

					checkState(short, state2)
				}

				// A few mirrors are 3 stage syncs
				if config.Mirrors[short].Rsync.Third != "" {
					start = time.Now()
					output3, state3 := rsync(config.Mirrors[short], config.Mirrors[short].Rsync.Third)
					status[short].Push(Status{StartTime: start.Unix(), EndTime: time.Now().Unix(), ExitCode: state3.ExitCode()})

					if rsyncLogs != "" {
						appendToLogFile(short, []byte("\n\n"+start.Format(time.RFC1123)+"\n"))
						appendToLogFile(short, output3)
					}

					checkState(short, state3)
				}

				// Unlock the project
				rsyncLock.Lock()
				rsyncLocks[short] = false
				rsyncLock.Unlock()
			}()
		}
	}
}

func checkState(short string, state *os.ProcessState) {
	if state != nil && state.Success() {
		logging.Success("Job rsync:", short, "finished successfully")
	} else {
		// We have some human readable error descriptions
		if meaning, ok := rysncErrorCodes[state.ExitCode()]; ok {
			logging.Error("Job rsync: ", short, "failed. Exit code: ", state.ExitCode(), meaning)
		} else {
			logging.Error("Job rsync: ", short, "failed. Exit code: ", state.ExitCode())
		}
	}
}
