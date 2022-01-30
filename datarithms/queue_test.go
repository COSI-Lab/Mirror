package datarithms_test

import (
	"testing"

	queue "github.com/COSI_Lab/Mirror/datarithms"
)

// Test the queue
func TestQueue(t *testing.T) {
	// Create a new queue
	q := queue.CircularQueueInit(5)

	if q.Capacity() != 5 {
		t.Error("Capacity is not 5")
	}

	// Push some elements
	q.Push(1)
	q.Push(2)
	q.Push(3)

	// Check the length
	if q.Len() != 3 {
		t.Error("Expected 3, got", q.Len())
	}

	var element interface{}

	// Pop the first element
	if element = q.Pop(); element != 1 {
		t.Error("Expected 1, got", element)
	}

	// Check the length
	if q.Len() != 2 {
		t.Error("Expected 2, got", q.Len())
	}

	// Pop the second element
	if element = q.Pop(); element != 2 {
		t.Error("Expected 2, got", element)
	}

	// Check the length
	if q.Len() != 1 {
		t.Error("Expected 1, got", q.Len())
	}

	// Pop the third element
	if element = q.Pop(); element != 3 {
		t.Error("Expected 3, got", element)
	}

	// Check the length
	if q.Len() != 0 {
		t.Error("Expected 0, got", q.Len())
	}

	// Pop the fourth element
	if element = q.Pop(); element != nil {
		t.Error("Expected nil, got", element)
	}

	// Check the length
	if q.Len() != 0 {
		t.Error("Expected 0, got", q.Len())
	}

	// Push more elements than capacity
	for i := 0; i < 10; i++ {
		q.Push(i)
	}

	// Check the length
	if q.Len() != 5 {
		t.Error("Expected 5, got", q.Len())
	}

	// Pop the first element
	if element = q.Pop(); element != 5 {
		t.Error("Expected 5, got", element)
	}

	// Check the length
	if q.Len() != 4 {
		t.Error("Expected 4, got", q.Len())
	}

	// Pop the second element
	if element = q.Pop(); element != 6 {
		t.Error("Expected 6, got", element)
	}

	// Check the length
	if q.Len() != 3 {
		t.Error("Expected 3, got", q.Len())
	}

	// Pop the third element
	if element = q.Pop(); element != 7 {
		t.Error("Expected 7, got", element)
	}

	// Check the length
	if q.Len() != 2 {
		t.Error("Expected 2, got", q.Len())
	}

	// Pop the fourth element
	if element = q.Pop(); element != 8 {
		t.Error("Expected 8, got", element)
	}

	// Check the length
	if q.Len() != 1 {
		t.Error("Expected 1, got", q.Len())
	}

	// Pop the fifth element
	if element = q.Pop(); element != 9 {
		t.Error("Expected 9, got", element)
	}

	// Check the length
	if q.Len() != 0 {
		t.Error("Expected 0, got", q.Len())
	}
}
