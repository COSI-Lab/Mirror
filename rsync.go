package main

import (
	"os"
	"os/exec"
	"time"

	"github.com/COSI_Lab/Mirror/datarithms"
	"github.com/COSI_Lab/Mirror/logging"
)

func rsync(project Project) ([]byte, *os.ProcessState) {
	var command exec.Cmd

	command.Path = "/usr/bin/rsync"
	command.Args = []string{project.Rsync.Options, "--dry-run", project.Rsync.Host + "::" + project.Rsync.Src, project.Rsync.Dest}
	b, err := command.CombinedOutput()

	if err != nil {
		logging.Log(logging.Warn, "Combined output call failed", err)
	}

	return b, command.ProcessState
}

func handleRSYNC(config ConfigFile) {
	rsyncStatus := make(map[string]*datarithms.CircularQueue, len(config.Mirrors))

	for _, mirror := range config.Mirrors {
		if mirror.Rsync.SyncsPerDay > 0 {
			rsyncStatus[mirror.Short] = datarithms.CircularQueueInit(7 * mirror.Rsync.SyncsPerDay)
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

	// run the schedule
	for {
		short, sleep := schedule.NextJob()

		if short == "" {
			logging.Log(logging.Error, "Reached end of schedule, restarting")
			schedule.Reset()
			continue
		}

		logging.Log(logging.Info, "Running job rsync: "+short)

		// TODO find project using a map instead
		for _, project := range config.Mirrors {
			if project.Short == short {
				go func() {
					b, state := rsync(project)
					rsyncStatus[short].Push(string(b))

					if state != nil && state.Success() {
						logging.Log(logging.Success, "Job rsync:", short, "finished successfully")
					} else {
						logging.Log(logging.Warn, "Job rsync:", short, "failed", state)
					}
				}()

				break
			}
		}

		logging.Log(logging.Info, "Sleeping for "+time.Duration(sleep).String())
		time.Sleep(sleep)
	}
}
