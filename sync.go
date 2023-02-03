package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/COSI-Lab/datarithms"
	"github.com/COSI-Lab/logging"
)

type Status struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`
	ExitCode  int   `json:"exitCode"`
}
type RSYNCStatus map[string]*datarithms.CircularQueue[Status]

var rysncErrorCodes map[int]string
var syncLock sync.Mutex
var syncLocks = make(map[string]bool)

func init() {
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
	if syncLogs != "" {
		err := os.MkdirAll(syncLogs, 0755)

		if err != nil {
			logging.Warn("failed to create RSYNC_LOGS directory", syncLogs, err, "not saving rsync logs")
			syncLogs = ""
		} else {
			logging.Success("opened RSYNC_LOGS directory", syncLogs)
		}
	}
}

func rsync(project *Project, options string) ([]byte, *os.ProcessState) {
	// split up the options TODO maybe precompute this?
	// actually in hindsight this whole thing can be precomputed
	args := strings.Split(options, " ")

	// Run with dry run if specified
	if syncDryRun {
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

	output, _ := command.CombinedOutput()

	return output, command.ProcessState
}

func appendToLogFile(short string, data []byte) {
	// Get month
	month := fmt.Sprintf("%02d", time.Now().UTC().Month())

	// Open the log file
	path := syncLogs + "/" + short + "-" + month + ".log"
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0640)
	if err != nil {
		logging.Warn("failed to open log file ", path, err)
	}

	if admGroup != 0 {
		// Set the file to be owned by the adm group
		err = file.Chown(os.Getuid(), admGroup)
		if err != nil {
			logging.Warn("failed to set log file ownership", path, err)
		}
	}

	// Write to the log file
	_, err = file.Write(data)
	if err != nil {
		logging.Warn("failed to write to log file ", path, err)
	}
}

func syncProject(config *ConfigFile, status RSYNCStatus, short string) {
	logging.Info("Running job: SYNC", short)

	// Lock the project
	syncLock.Lock()
	if syncLocks[short] {
		syncLock.Unlock()
		logging.Warn("Sync is already running for ", short)
		return
	}
	syncLocks[short] = true
	syncLock.Unlock()

	start := time.Now()

	if config.Mirrors[short].SyncStyle == "rsync" {
		// 1 stage syncs are the norm
		output1, state1 := rsync(config.Mirrors[short], config.Mirrors[short].Rsync.Options)
		status[short].Push(Status{StartTime: start.Unix(), EndTime: time.Now().Unix(), ExitCode: state1.ExitCode()})

		// append stage 1 to its log file
		if syncLogs != "" {
			appendToLogFile(short, []byte("\n\n"+start.Format(time.RFC1123)+"\n"))
			appendToLogFile(short, output1)
		}

		checkRSYNCState(short, state1, output1)

		// 2 stage syncs happen sometimes
		if config.Mirrors[short].Rsync.Second != "" {
			start = time.Now()
			output2, state2 := rsync(config.Mirrors[short], config.Mirrors[short].Rsync.Second)
			status[short].Push(Status{StartTime: start.Unix(), EndTime: time.Now().Unix(), ExitCode: state2.ExitCode()})

			if syncLogs != "" {
				appendToLogFile(short, []byte("\n\n"+start.Format(time.RFC1123)+"\n"))
				appendToLogFile(short, output2)
			}

			checkRSYNCState(short, state2, output2)
		}

		// A few mirrors are 3 stage syncs
		if config.Mirrors[short].Rsync.Third != "" {
			start = time.Now()
			output3, state3 := rsync(config.Mirrors[short], config.Mirrors[short].Rsync.Third)
			status[short].Push(Status{StartTime: start.Unix(), EndTime: time.Now().Unix(), ExitCode: state3.ExitCode()})

			if syncLogs != "" {
				appendToLogFile(short, []byte("\n\n"+start.Format(time.RFC1123)+"\n"))
				appendToLogFile(short, output3)
			}

			checkRSYNCState(short, state3, output3)
		}
	} else if config.Mirrors[short].SyncStyle == "script" {
		if syncDryRun {
			logging.Info("Did not sync", short, "because --dry-run was specified")
			return
		}

		// Execute the script
		logging.Info(config.Mirrors[short].Script.Command, config.Mirrors[short].Script.Arguments)
		command := exec.Command(config.Mirrors[short].Script.Command, config.Mirrors[short].Script.Arguments...)
		output, _ := command.CombinedOutput()

		if syncLogs != "" {
			appendToLogFile(short, []byte("\n\n"+start.Format(time.RFC1123)+"\n"))
			appendToLogFile(short, output)
		}
	}

	// Unlock the project
	syncLock.Lock()
	syncLocks[short] = false
	syncLock.Unlock()
}

// handleSyncs is the main scheduler
// It builds a schedule of when to sync projects in such a way they are equaly spaced across the day
// tasks are run in a separate goroutine and there is a lock to prevent the same project from being synced simultaneously
// the stop channel gracefully stops the scheduler after all active rsync tasks have completed
// the manual channel is used to manually sync a project, assuming it is not already currently syncing
func handleSyncs(config *ConfigFile, status RSYNCStatus, manual <-chan string, stop chan struct{}) {
	for _, mirror := range config.Mirrors {
		if mirror.Rsync.SyncsPerDay > 0 {
			// Store a weeks worth of status messages in memory
			status[mirror.Short] = datarithms.CircularQueueInit[Status](7 * mirror.Rsync.SyncsPerDay)
		}
	}

	// prepare the tasks
	tasks := make([]datarithms.Task, 0, len(config.Mirrors))
	for _, mirror := range config.Mirrors {
		if mirror.SyncStyle == "rsync" {
			tasks = append(tasks, datarithms.Task{
				Short: mirror.Short,
				Syncs: mirror.Rsync.SyncsPerDay,
			})
		} else if mirror.SyncStyle == "script" {
			tasks = append(tasks, datarithms.Task{
				Short: mirror.Short,
				Syncs: mirror.Script.SyncsPerDay,
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
	syncLock = sync.Mutex{}
	syncLocks = make(map[string]bool)
	for _, project := range config.Mirrors {
		syncLocks[project.Short] = false
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
				syncLock.Lock()
				allDone := true
				for _, running := range syncLocks {
					if running {
						allDone = false
						break
					}
				}
				syncLock.Unlock()

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

			go syncProject(config, status, short)
		case short := <-manual:
			syncLock.Lock()
			if !syncLocks[short] {
				syncLocks[short] = true
				go syncProject(config, status, short)
			}
			syncLock.Unlock()
		}
	}
}

func checkRSYNCState(short string, state *os.ProcessState, output []byte) {
	if state != nil && state.Success() {
		logging.Success("Job rsync:", short, "finished successfully")
	} else {
		if state.ExitCode() == 23 || state.ExitCode() == 24 {
			// states 23 "Partial transfer due to error" and 24 "Partial transfer" are not considered important enough to message discord
			logging.Error("Job rsync: ", short, " failed. Exit code: ", state.ExitCode(), " ", rysncErrorCodes[state.ExitCode()])
		} else {
			// We have some human readable error descriptions
			if meaning, ok := rysncErrorCodes[state.ExitCode()]; ok {
				logging.ErrorWithAttachment(output, "Job rsync: ", short, " failed. Exit code: ", state.ExitCode(), " ", meaning)
			} else {
				logging.ErrorWithAttachment(output, "Job rsync: ", short, " failed. Exit code: ", state.ExitCode())
			}
		}
	}
}

// On start up then once a week checks and deletes all logs older than 3 months
func checkOldLogs() {
	ticker := time.NewTicker(168 * time.Hour)
	deleteOldLogs()

	for range ticker.C {
		deleteOldLogs()
	}
}

// deletes all logs older than 3 months
func deleteOldLogs() {
	logFiles, err := os.ReadDir(syncLogs)
	if err != nil {
		logging.Error(err)
	} else {
		for _, logFile := range logFiles {
			path := syncLogs + "/" + logFile.Name()
			fileStat, err := os.Stat(path)
			if err != nil {
				logging.Warn(err)
			} else {
				modTime := fileStat.ModTime()
				if modTime.Before(time.Now().Add(-2160 * time.Hour)) {
					err = os.Remove(path)
					if err != nil {
						logging.Warn(err)
					} else {
						logging.Info("removed " + path)
					}
				}
			}
		}
	}
}
