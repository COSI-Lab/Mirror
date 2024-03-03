package datarithms

import (
	"fmt"
	"sync"
)

// CircularQueue is a thread-safe queue with a fixed capacity
type CircularQueue[T any] struct {
	lock     sync.RWMutex
	queue    []T
	capacity int
	start    int
	end      int
	length   int
}

// NewCircularQueue creates a new CircularQueue with the given capacity
func NewCircularQueue[T any](capacity int) *CircularQueue[T] {
	return &CircularQueue[T]{
		lock:     sync.RWMutex{},
		queue:    make([]T, capacity),
		capacity: capacity,
		start:    0,
		end:      0,
		length:   0,
	}
}

// Push adds an element to the end of the queue
// If the queue is full, the oldest element is overwritten
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

// Pop removes an element from the front of the queue
// If the queue is empty, returns error
func (q *CircularQueue[T]) Pop() (element T, err error) {
	q.lock.Lock()
	// If the queue is empty, return nil
	if q.length == 0 {
		q.lock.Unlock()
		return element, fmt.Errorf("circularQueue is empty")
	}
	element = q.queue[q.start]
	q.start = (q.start + 1) % q.capacity
	q.length--
	q.lock.Unlock()
	return element, nil
}

// Front returns the element at the start of the queue
func (q *CircularQueue[T]) Front() T {
	q.lock.RLock()
	result := q.queue[q.start]
	q.lock.RUnlock()
	return result
}

// Len returns the number of elements in the queue
func (q *CircularQueue[T]) Len() int {
	q.lock.RLock()
	result := q.length
	q.lock.RUnlock()
	return result
}

// Capacity returns the capacity of the queue
func (q *CircularQueue[T]) Capacity() int {
	q.lock.RLock()
	result := q.capacity
	q.lock.RUnlock()
	return result
}

// All builds a slice of all elements in the queue
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

// Fold folds the queue into a single value given a function
func Fold[T any, R any](q *CircularQueue[T], f func(R, T) R, init R) R {
	q.lock.RLock()
	result := init
	for i := q.start; i != q.end; i = (i + 1) % q.capacity {
		result = f(result, q.queue[i])
	}
	q.lock.RUnlock()
	return result
}
