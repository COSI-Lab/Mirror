package datarithms

import "sync"

// Thread Safe circular queue implmentation using a slice for byte slices
type CircularQueue struct {
	lock sync.RWMutex
	// TODO replace interface{} with generics in the future
	queue    []interface{}
	capacity int
	start    int
	end      int
	length   int
}

// Creates a new circular queue of given capacity
func CircularQueueInit(capacity int) *CircularQueue {
	q := new(CircularQueue)

	q.queue = make([]interface{}, capacity)
	q.capacity = capacity
	q.start = 0
	q.end = 0
	q.length = 0
	q.lock = sync.RWMutex{}

	return q
}

// Adds a new element to the queue
func (q *CircularQueue) Push(element interface{}) {
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
func (q *CircularQueue) Pop() interface{} {
	q.lock.Lock()
	// If the queue is empty, return nil
	if q.length == 0 {
		q.lock.Unlock()
		return nil
	}
	element := q.queue[q.start]
	q.start = (q.start + 1) % q.capacity
	q.length--
	q.lock.Unlock()
	return element
}

// Returns the element at the front of the queue
func (q *CircularQueue) Front() interface{} {
	q.lock.RLock()
	result := q.queue[q.start]
	q.lock.RUnlock()
	return result
}

// Returns the number of elements in the queue
func (q *CircularQueue) Len() int {
	q.lock.RLock()
	result := q.length
	q.lock.RUnlock()
	return result
}

// Returns the capacity of the queue
func (q *CircularQueue) Capacity() int {
	q.lock.RLock()
	result := q.capacity
	q.lock.RUnlock()
	return result
}
