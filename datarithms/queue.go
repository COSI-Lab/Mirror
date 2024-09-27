package datarithms

import (
	"fmt"
	"sync"
)

// Thread Safe circular queue implmentation using a slice for byte slices
type CircularQueue[T any] struct {
	lock     sync.RWMutex
	queue    []T
	capacity int
	start    int
	end      int
	length   int
}

// Creates a new circular queue of given capacity
func CircularQueueInit[T any](capacity int) *CircularQueue[T] {
	q := new(CircularQueue[T])

	q.queue = make([]T, capacity)
	q.capacity = capacity
	q.start = 0
	q.end = 0
	q.length = 0
	q.lock = sync.RWMutex{}

	return q
}

// Adds a new element to the queue
func (q *CircularQueue[T]) Push(element T) {
	q.lock.Lock()
	q.queue[q.end] = element
	q.end = (q.end + 1) % q.capacity
	// If the queue is full, start overwriting from the beginning
	if q.length == q.capacity {
		q.start = (q.start + 1) % q.capacity
	} else {
		q.length++
	}
	q.lock.Unlock()
}

// Pops the element at the front of the queue
// If the queue is empty, returns the zero value followed by an error
func (q *CircularQueue[T]) Pop() (element T, err error) {
	q.lock.Lock()
	// If the queue is empty, return nil
	if q.length == 0 {
		q.lock.Unlock()
		return element, fmt.Errorf("CircularQueue is empty")
	}
	element = q.queue[q.start]
	q.start = (q.start + 1) % q.capacity
	q.length--
	q.lock.Unlock()
	return element, nil
}

// Returns the element at the front of the queue
func (q *CircularQueue[T]) Front() T {
	q.lock.RLock()
	result := q.queue[q.start]
	q.lock.RUnlock()
	return result
}

// Returns the number of elements in the queue
func (q *CircularQueue[T]) Len() int {
	q.lock.RLock()
	result := q.length
	q.lock.RUnlock()
	return result
}

// Returns the capacity of the queue
func (q *CircularQueue[T]) Capacity() int {
	q.lock.RLock()
	result := q.capacity
	q.lock.RUnlock()
	return result
}

// Returns all the elements of the queue
func (q *CircularQueue[T]) All() []T {
	q.lock.RLock()
	result := make([]T, 0, q.length)

	// From start to end
	for i := q.start; i != q.end; i = (i + 1) % q.capacity {
		result = append(result, q.queue[i])
	}

	q.lock.RUnlock()
	return result
}
