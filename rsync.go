package main

import (
	"log"
	"os"
	"os/exec"

	"github.com/COSI_Lab/Mirror/datarithms"
)

func rsync(project Project) ([]byte, *os.ProcessState) {
	var command exec.Cmd

	command.Path = "/usr/local/bin/rsync"
	command.Args = []string{project.Rsync.Options, "--dry-run", project.Rsync.Host + "::" + project.Rsync.Src, project.Rsync.Dest}
	b, _ := command.CombinedOutput()

	return b, command.ProcessState
}

func handleRSYNC(config ConfigFile) {
	rsyncStatus := make(map[string]*datarithms.CircularQueue, len(config.Mirrors))

	for _, mirror := range config.Mirrors {
		if mirror.Rsync.SyncsPerDay > 0 {
			rsyncStatus[mirror.Short] = datarithms.CircularQueueInit(7 * mirror.Rsync.SyncsPerDay)
		}
	}

	for _, mirror := range config.Mirrors {
		if mirror.Rsync.Host != "" {
			b, _ := rsync(mirror)
			// TODO check if the state is ok
			rsyncStatus[mirror.Short].Push(b)
		}
	}

	for _, mirror := range config.Mirrors {
		if mirror.Rsync.SyncsPerDay > 0 {
			log.Println(mirror.Short, rsyncStatus[mirror.Short].Len())
		}
	}
}
