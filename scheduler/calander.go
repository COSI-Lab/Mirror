package scheduler

import "time"

// Calendar is a struct that holds a list of tasks and their corresponding time to run [0, 1)
//
// The invariant is that the time must be increasing.
// So the algorithm is trivial. Run the task, sleep until the next start time, repeat.
type Calendar[T any] struct {
	tasks    []T
	times    []float32
	iterator int
}

// Returns the job to run and how long to sleep until the next job
func (s *Calendar[T]) NextJob() (task T, dt time.Duration) {
	// Calculate the time since midnight
	now := time.Now().UTC()
	pos := time.Since(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC))

	// Convert time to position in the schedule	(0.0 <= t <= 1.0)
	t := float32(pos) / float32(24*time.Hour)

	// Find the first job that is greater than the current time
	for s.iterator < len(s.tasks) && s.times[s.iterator] <= t {
		s.iterator++
	}

	// If we are at the end of the schedule, sleep until midnight
	if s.iterator == len(s.tasks) {
		s.iterator = 0
		dt = time.Until(time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC))
		return s.tasks[len(s.tasks)-1], dt
	}

	// Time to sleep until the next job
	dt = time.Duration((s.times[s.iterator] - t) * float32(24*time.Hour))

	return s.tasks[s.iterator-1], dt
}

// Scheduling algorithm
func BuildCalendar[T any](tasks []T, timesPerDay []uint) Calendar[T] {
	total_jobs := uint(0)
	for _, n := range timesPerDay {
		total_jobs += n
	}

	// Compute least common multiple of all sync frequencies
	lcm := uint(1)
	for _, n := range timesPerDay {
		// compute the greatest common divisor of best known LCM and sync frequency of the current task
		var (
			a uint
			b uint
		)

		if lcm > n {
			a = lcm
			b = n
		} else {
			a = n
			b = lcm
		}
		for b != 0 {
			rem := a % b
			a = b
			b = rem
		}

		// now a is the GCD; we can compute the next LCM
		// TODO: check for overflow in multiplication
		lcm = lcm * n / a
	}

	jobs := make([]T, total_jobs)
	times := make([]float32, total_jobs)

	var interval float32 = 1.0 / float32(total_jobs)
	c := 0
	for i := uint(0); i < lcm; i++ {
		for idx, task := range tasks {
			n := timesPerDay[idx]
			if i%(lcm/n) == 0 {
				// emit a job
				tasks[c] = task
				times[c] = interval * float32(c)
				c += 1
			}
		}
	}

	return Calendar[T]{
		tasks:    jobs,
		times:    times,
		iterator: 0,
	}
}

// Applies a function to each task in the calendar
func (s *Calendar[T]) ForEach(f func(*T)) {
	for i := range s.tasks {
		f(&s.tasks[i])
	}
}

// Finds the first task that satisfies the predicate
func (s *Calendar[T]) Find(f func(T) bool) *T {
	for i := range s.tasks {
		if f(s.tasks[i]) {
			return &s.tasks[i]
		}
	}
	return nil
}
