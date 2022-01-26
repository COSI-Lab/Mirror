package main

import (
	"os"
	"os/exec"
)

func rsync(project Project) ([]byte, *os.ProcessState) {
	var command exec.Cmd

	command.Path = "/usr/local/bin/rsync"
	command.Args = []string{project.Rsync.Options, "--dry-run", project.Rsync.Host + "::" + project.Rsync.Src, project.Rsync.Dest}
	b, _ := command.CombinedOutput()

	return b, command.ProcessState
}
