package datarithms

import (
	"fmt"
	"time"
)

// Schedule is a struct that holds a list of tasks and their corresponding sleeps
// The invariant is that the total number of sleeps is equal to 24 * time.Hour
// So the excutation algorithm is trivial. Run a task, sleep, repeat.
type Schedule struct {
	jobs     []Job
	iterator int
}

type Job struct {
	short string
	sleep time.Duration
}

// Returns the next job to run
func (s *Schedule) NextJob() *Job {
	s.iterator = (s.iterator + 1) % len(s.jobs)
	return &s.jobs[s.iterator]
}

// fed as input to the scheduling algorithm
type Task struct {
	// short name of the project
	short string

	// How many times does the project sync per day
	syncs int
}

// Scheduling algorithm
func BuildSchedule(task []Task) *Schedule {
	return &Schedule{iterator: 0}
}

// Verifies that the schedule has 24 * time.Hour worth of sleeps
// and that each task will be synced the correct number of times
func Verify(s *Schedule, tasks []Task) bool {
	// Setup trackers
	var total time.Duration
	total = 0

	syncs := make(map[string]int)
	for _, task := range tasks {
		syncs[task.short] = 0
	}

	for _, job := range s.jobs {
		total += job.sleep
		syncs[job.short]++
	}

	// Check
	if total != 24*time.Hour {
		return false
	}

	for _, task := range tasks {
		if syncs[task.short] != task.syncs {
			fmt.Println(task.short, "was expecting", task.syncs, "syncs but found", syncs[task.short])
			return false
		}
	}

	return true
}
