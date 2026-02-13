package utils

import (
	"sync"
)

// Queue is a thread-safe bounded FIFO queue. When full, the oldest item is dropped.
type Queue[T any] struct {
	mu       sync.Mutex
	data     []T
	capacity int
}

// NewQueue creates a new Queue with the specified capacity.
func NewQueue[T any](capacity int) *Queue[T] {
	if capacity < 0 {
		capacity = 0
	}

	return &Queue[T]{
		data:     make([]T, 0, capacity),
		capacity: capacity,
	}
}

// Push adds an item to the queue. If the queue is full, the oldest item is dropped.
func (q *Queue[T]) Push(item T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Non-positive capacity means the queue is effectively disabled.
	if q.capacity <= 0 {
		return
	}

	if len(q.data) >= q.capacity {
		// Drop oldest (shift left)
		q.data = q.data[1:]
	}
	q.data = append(q.data, item)
}

// Pop removes and returns the oldest item from the queue.
// Returns zero value and false if empty.
func (q *Queue[T]) Pop() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.data) == 0 {
		var zero T
		return zero, false
	}

	item := q.data[0]
	q.data = q.data[1:]
	return item, true
}

// Peek returns the oldest item without removing it.
func (q *Queue[T]) Peek() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.data) == 0 {
		var zero T
		return zero, false
	}
	return q.data[0], true
}

// Len returns the current number of items.
func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.data)
}

// IsEmpty returns true if the queue is empty.
func (q *Queue[T]) IsEmpty() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.data) == 0
}
