package scheduler

// CalendarBuilder is a builder pattern for the Calendar struct
type CalendarBuilder[T any] struct {
	tasks []T
	times []uint
}

// NewCalendarBuilder creates a new CalendarBuilder
func NewCalendarBuilder[T any]() CalendarBuilder[T] {
	return CalendarBuilder[T]{
		tasks: make([]T, 0),
		times: make([]uint, 0),
	}
}

// AddTask adds a task to the CalendarBuilder
func (b *CalendarBuilder[T]) AddTask(task T, timesPerDay uint) {
	b.tasks = append(b.tasks, task)
	b.times = append(b.times, timesPerDay)
}

// Build builds the Calendar
func (b *CalendarBuilder[T]) Build() Calendar[T] {
	return buildCalendar(b.tasks, b.times)
}
