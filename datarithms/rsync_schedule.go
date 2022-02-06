package datarithms

import (
	"fmt"
)

// Schedule is a struct that holds a list of tasks and their corresponding target time to run
// The invariant is that the target time must be increasing in the jobs list.
// So the excutation algorithm is trivial. Run a task, sleep, repeat.
type Schedule struct {
	jobs     []Job
	iterator int
}

type Job struct {
	short       string
	target_time float32
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
func BuildSchedule(tasks []Task) *Schedule {
	total_jobs := 0
	for _, task := range tasks {
		total_jobs += task.syncs
	}

	// compute least common multiple of all sync frequencies
	lcm := 1
	for _, task := range tasks {
		// compute the greatest common divisor of best known LCM and sync frequency of the current task
		var (
			a int
			b int
		)
		if lcm > task.syncs {
			a = lcm
			b = task.syncs
		} else {
			a = task.syncs
			b = lcm
		}
		for b != 0 {
			rem := a % b
			a = b
			b = rem
		}
		// now a is the GCD; we can compute the next LCM
		// FIXME: check for overflow in multiplication
		lcm = lcm * task.syncs / a
	}

	jobs := make([]Job, total_jobs)
	var interval float32 = 1.0 / float32(total_jobs)
	c := 0
	for i := 0; i < lcm; i++ {
		for _, task := range tasks {
			if lcm%task.syncs == 0 {
				// emit a job
				jobs[c].short = task.short
				jobs[c].target_time = interval * float32(c)
				c += 1
			}
		}
	}

	return &Schedule{iterator: 0, jobs: jobs}
}

// Verifies that the schedule has increasing target times, all of them are
// within the cycle (0.0 <= t <= 1.0), and that each task will be synced the
// correct number of times
func Verify(s *Schedule, tasks []Task) bool {
	syncs := make(map[string]int)
	for _, task := range tasks {
		syncs[task.short] = 0
	}

	var last_time float32 = 0.0
	for _, job := range s.jobs {
		if job.target_time < last_time || job.target_time > 1.0 {
			return false
		}
		last_time = job.target_time
		syncs[job.short]++
	}

	for _, task := range tasks {
		if syncs[task.short] != task.syncs {
			fmt.Println(task.short, "was expecting", task.syncs, "syncs but found", syncs[task.short])
			return false
		}
	}

	return true
}
