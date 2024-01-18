package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/COSI-Lab/Mirror/config"
	"github.com/COSI-Lab/Mirror/datarithms"
	"github.com/COSI-Lab/Mirror/logging"
	"github.com/COSI-Lab/Mirror/scheduler"
)

// TaskStatus is an enum of possible return statuses for a task
type TaskStatus int

const (
	// TaskStatusSuccess indicates that the task completed successfully
	TaskStatusSuccess TaskStatus = iota
	// TaskStatusFailure indicates that the task failed to complete
	TaskStatusFailure
	// TaskStatusStopped indicates that the task was stopped before it could complete by the scheduler
	TaskStatusStopped
)

// Task is the units of work to be preformed by the scheduler
//
// Each task runs in its own go-routine and the scheduler ensures that only one instance of task `Run` will be called at a time
type Task interface {
	Run(context context.Context, stdout io.Writer, stderr io.Writer, status chan<- logging.LogEntry) TaskStatus
}

type syncResult struct {
	start  time.Time
	end    time.Time
	status TaskStatus
}

// Scheduler is the main task scheduler. It's passed a context that can be used to stop all associated tasks
type Scheduler struct {
	ctx context.Context

	calendar scheduler.Calendar[*SchedulerTask]
}

// SchedulerTask wraps a `task` to provide storage for stdout, stderr, and a channel for logging
type SchedulerTask struct {
	sync.Mutex
	running bool

	short string

	queue   *datarithms.CircularQueue[logging.LogEntry]
	results *datarithms.CircularQueue[syncResult]

	channel chan logging.LogEntry
	stdout  *bufio.Writer
	stderr  *bufio.Writer
	task    Task
}

// NewScheduler creates a new scheduler from a config.File
func NewScheduler(ctx context.Context, config *config.File) (Scheduler, error) {
	month := time.Now().UTC().Month()

	builer := scheduler.NewCalendarBuilder[*SchedulerTask]()

	// Create the log directory if it doesn't exist
	if _, err := os.Stat("/var/log/mirror"); os.IsNotExist(err) {
		err := os.Mkdir("/var/log/mirror", 0755)
		if err != nil {
			return Scheduler{}, fmt.Errorf("failed to create log directory: %s", err)
		}
	}

	for short, project := range config.Projects {
		var task Task
		var syncsPerDay uint

		switch project.SyncStyle {
		case "rsync":
			task = NewRSYNCTask(project.Rsync, short)
			syncsPerDay = project.Rsync.SyncsPerDay
		case "script":
			task = NewScriptTask(project.Script, short)
			syncsPerDay = project.Script.SyncsPerDay
		default:
			continue
		}

		q := datarithms.NewCircularQueue[logging.LogEntry](64)
		results := datarithms.NewCircularQueue[syncResult](64)

		channel := make(chan logging.LogEntry, 64)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case entry := <-channel:
					q.Push(entry)
				}
			}
		}()

		stdout, err := os.OpenFile(fmt.Sprintf("/var/log/mirror/%s-%s.log", short, month), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return Scheduler{}, fmt.Errorf("failed to open stdout file for %q: %s", short, err)
		}
		stderr, err := os.OpenFile(fmt.Sprintf("/var/log/mirror/%s-%s.err", short, month), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return Scheduler{}, fmt.Errorf("failed to open stderr file for %q: %s", short, err)
		}

		builer.AddTask(&SchedulerTask{
			running: false,
			short:   short,
			queue:   q,
			results: results,
			channel: channel,
			stdout:  bufio.NewWriter(stdout),
			stderr:  bufio.NewWriter(stderr),
			task:    task,
		}, syncsPerDay)
	}

	return Scheduler{
		ctx:      ctx,
		calendar: builer.Build(),
	}, nil
}

// Start begins the scheduler and blocks until the context is canceled
func (sc *Scheduler) Start(manual <-chan string) {
	timer := time.NewTimer(0)
	month := time.NewTimer(timeToNextMonth())

	for {
		select {
		case <-sc.ctx.Done():
			return
		case <-month.C:
			month.Reset(timeToNextMonth())
			month := time.Now().Local().Month()
			sc.calendar.ForEach(
				func(task **SchedulerTask) {
					t := *task
					t.Lock()
					t.stdout.Flush()
					t.stderr.Flush()
					// Create new files for the next month
					stdout, err := os.OpenFile(fmt.Sprintf("/var/log/mirror/%s-%s.log", t.short, month), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					if err != nil {
						logging.Error("Failed to open stdout file for %q: %s", t.short, err)
					} else {
						t.stdout.Reset(stdout)
					}
					stderr, err := os.OpenFile(fmt.Sprintf("/var/log/mirror/%s-%s.err", t.short, month), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					if err != nil {
						logging.Error("Failed to open stderr file for %q: %s", t.short, err)
					} else {
						t.stderr.Reset(stderr)
					}
					t.Unlock()
				})
		case <-timer.C:
			t, dt := sc.calendar.NextJob()
			timer.Reset(dt)
			t.runTask(sc.ctx)
		case short := <-manual:
			t := *sc.calendar.Find(func(t *SchedulerTask) bool {
				return t.short == short
			})
			t.runTask(sc.ctx)
		}
	}
}

// runTask handles locking and unlocking the task and logging the results
func (t *SchedulerTask) runTask(ctx context.Context) {
	t.Lock()
	if t.running {
		t.Unlock()
		return
	}
	t.running = true
	t.Unlock()

	go func() {
		start := time.Now()
		status := t.task.Run(ctx, t.stdout, t.stderr, t.channel)
		t.stdout.Flush()
		t.stderr.Flush()
		end := time.Now()
		t.results.Push(syncResult{
			start:  start,
			end:    end,
			status: status,
		})
		t.Lock()
		t.running = false
		t.Unlock()
	}()
}

// timeToNextMonth returns the duration until the next month
func timeToNextMonth() time.Duration {
	now := time.Now().UTC()
	return time.Until(time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.Local))
}
