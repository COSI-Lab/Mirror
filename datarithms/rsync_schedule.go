package datarithms

import (
	"fmt"
	"time"
)

// Schedule is a struct that holds a list of tasks and their corresponding target time to run
// The invariant is that the target time must be increasing in the jobs list.
// So the excutation algorithm is trivial. Run the task, sleep until the next target time, repeat.
type Schedule struct {
	jobs     []Job
	iterator int
}

type Job struct {
	short       string
	target_time float32
}

// Returns the next job to run and how long to sleep until the next job
func (s *Schedule) NextJob() (short string, dt time.Duration) {
	//                v
	// [ ] -> [ ] -> [ ]
	// run this job and sleep until midnight
	if s.iterator == len(s.jobs)-1 {
		// we're at the end of the schedule
		defer s.Reset()

		now := time.Now()
		dt = time.Until(time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC))
		return s.jobs[s.iterator].short, dt
	}

	//         v current job
	// [ ] -> [ ] -> [ ]
	//                ^ next job (will never overflow)
	// run this job sleep until the next job and change to the next job
	current := s.jobs[s.iterator]
	s.iterator++
	next := s.jobs[s.iterator]

	// compute the time until the next job
	dt = time.Duration((next.target_time - current.target_time) * float32(24*time.Hour))
	return current.short, dt
}

// Resets the iterator to the beginning of the schedule
func (s *Schedule) Reset() {
	s.iterator = 0
}

// fed as input to the scheduling algorithm
type Task struct {
	// Short name of the project
	Short string

	// How many times does the project sync per day
	Syncs int
}

// Scheduling algorithm
func BuildSchedule(tasks []Task) *Schedule {
	total_jobs := 0
	for _, task := range tasks {
		total_jobs += task.Syncs
	}

	// compute least common multiple of all sync frequencies
	lcm := 1
	for _, task := range tasks {
		// compute the greatest common divisor of best known LCM and sync frequency of the current task
		var (
			a int
			b int
		)
		if lcm > task.Syncs {
			a = lcm
			b = task.Syncs
		} else {
			a = task.Syncs
			b = lcm
		}
		for b != 0 {
			rem := a % b
			a = b
			b = rem
		}
		// now a is the GCD; we can compute the next LCM
		// FIXME: check for overflow in multiplication
		lcm = lcm * task.Syncs / a
	}

	jobs := make([]Job, total_jobs)
	var interval float32 = 1.0 / float32(total_jobs)
	c := 0
	for i := 0; i < lcm; i++ {
		for _, task := range tasks {
			if i%(lcm/task.Syncs) == 0 {
				// emit a job
				jobs[c].short = task.Short
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
		syncs[task.Short] = 0
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
		if syncs[task.Short] != task.Syncs {
			fmt.Println(task.Short, "was expecting", task.Syncs, "syncs but found", syncs[task.Short])
			return false
		}
	}

	return true
}
