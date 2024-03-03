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

// ScriptTask is a task that runs a script project
type ScriptTask struct {
	short     string
	env       map[string]string
	command   string
	arguments []string
}

// NewScriptTask creates a new ScriptTask from a config.Script
func NewScriptTask(declaration *config.Script, short string) *ScriptTask {
	return &ScriptTask{
		short:     short,
		env:       declaration.Env,
		command:   declaration.Command,
		arguments: declaration.Arguments,
	}
}

// Run runs the script, blocking until it finishes
func (s *ScriptTask) Run(ctx context.Context, stdout, stderr io.Writer, status chan<- logging.LogEntry) TaskStatus {
	status <- logging.InfoLogEntry(fmt.Sprintf("%s: Starting script", s.short))
	cmd := exec.Command(s.command, s.arguments...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// set environment variables
	env := os.Environ()
	for key, value := range s.env {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = env

	status <- logging.InfoLogEntry("Running: " + cmd.String())

	err := cmd.Start()
	if err != nil {
		status <- logging.ErrorLogEntry(fmt.Sprintf("%s: Failed to start script: %s", s.short, err.Error()))
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
		status <- logging.InfoLogEntry(fmt.Sprintf("%s: Script stopped", s.short))
		return TaskStatusStopped
	}

	if cmd.ProcessState.Success() {
		status <- logging.SuccessLogEntry(fmt.Sprintf("%s: Script finished successfully", s.short))
		return TaskStatusSuccess
	}

	status <- logging.ErrorLogEntry(fmt.Sprintf("%s: Script failed with exit code %d", s.short, cmd.ProcessState.ExitCode()))
	return TaskStatusFailure
}
