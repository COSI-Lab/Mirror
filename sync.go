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
	// TaskStatusTimeout indicates that the task failed to complete within the allotted time
	TaskStatusTimeout
	// TaskStatusStopped indicates that the task was stopped before it could complete by the scheduler
	TaskStatusStopped
)

// Tasks are the units of work to be preformed by the scheduler
//
// Each task runs in its own go-routine and the scheduler ensures that only one instance of task `Run` will be called at a time
type Task interface {
	Run(stdout io.Writer, stderr io.Writer, status chan<- logging.LogEntryT, context context.Context) TaskStatus
}

type sync_result_t struct {
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

	queue   *datarithms.CircularQueue[logging.LogEntryT]
	results *datarithms.CircularQueue[sync_result_t]

	channel chan logging.LogEntryT
	stdout  *bufio.Writer
	stderr  *bufio.Writer
	task    Task
}

func NewScheduler(config *config.File, ctx context.Context) Scheduler {
	failed := false
	month := time.Now().UTC().Month()

	tasks := make([]*SchedulerTask, 0, len(config.Projects))
	timesPerDay := make([]uint, 0, len(config.Projects))

	for short, project := range config.Projects {
		var task Task
		var syncsPerDay uint

		switch project.SyncStyle {
		case "rsync":
			task = NewRsyncTask(project.Rsync, short)
			syncsPerDay = project.Rsync.SyncsPerDay
		case "script":
			task = NewScriptTask(project.Script, short)
			syncsPerDay = project.Script.SyncsPerDay
		default:
			continue
		}

		q := datarithms.NewCircularQueue[logging.LogEntryT](64)
		results := datarithms.NewCircularQueue[sync_result_t](64)

		channel := make(chan logging.LogEntryT, 64)
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
			logging.Error("Failed to open stdout file for project ", short, ": ", err)
			failed = true
		}
		stderr, err := os.OpenFile(fmt.Sprintf("/var/log/mirror/%s-%s.err", short, month), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logging.Error("Failed to open stderr file for project ", short, ": ", err)
			failed = true
		}

		tasks = append(tasks, &SchedulerTask{
			running: false,
			short:   short,
			queue:   q,
			results: results,
			channel: channel,
			stdout:  bufio.NewWriter(stdout),
			stderr:  bufio.NewWriter(stderr),
			task:    task,
		})
		timesPerDay = append(timesPerDay, syncsPerDay)
	}

	if failed {
		logging.Error("One or more errors occurred while setting up the scheduler")
		os.Exit(1)
	}

	return Scheduler{
		ctx:      ctx,
		calendar: scheduler.BuildCalendar[*SchedulerTask](tasks, timesPerDay),
	}
}

// Start begins the scheduler and blocks until the context is canceled
//
// manual is a channel that can be used to manually trigger a project sync
func (sc *Scheduler) Start(manual <-chan string) {
	timer := time.NewTimer(0)
	month := time.NewTimer(waitMonth())

	for {
		select {
		case <-sc.ctx.Done():
			return
		case <-month.C:
			month.Reset(waitMonth())
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
						logging.Error("Failed to open stdout file for project ", t.short, ": ", err)
					} else {
						t.stdout.Reset(stdout)
					}
					stderr, err := os.OpenFile(fmt.Sprintf("/var/log/mirror/%s-%s.err", t.short, month), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					if err != nil {
						logging.Error("Failed to open stderr file for project ", t.short, ": ", err)
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
		status := t.task.Run(t.stdout, t.stderr, t.channel, ctx)
		t.stdout.Flush()
		t.stderr.Flush()
		end := time.Now()
		t.results.Push(sync_result_t{
			start:  start,
			end:    end,
			status: status,
		})
		t.Lock()
		t.running = false
		t.Unlock()
	}()
}

// waitMonth returns a timer that will fire at the beginning of the next month
func waitMonth() time.Duration {
	now := time.Now().UTC()
	return time.Until(time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.Local))
}
