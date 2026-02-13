package utils

import (
	"sync"
	"testing"
)

func TestNewQueue(t *testing.T) {
	q := NewQueue[int](5)
	if q.capacity != 5 {
		t.Errorf("expected capacity 5, got %d", q.capacity)
	}
	if q.Len() != 0 {
		t.Errorf("expected empty queue, got len %d", q.Len())
	}
}

func TestNewSanitizesNonPositiveCapacity(t *testing.T) {
	tests := []int{0, -5}

	for _, capacity := range tests {
		q := New[int](capacity)
		if q.capacity != 1 {
			t.Fatalf("expected sanitized capacity 1 for input %d, got %d", capacity, q.capacity)
		}

		q.Push(1)
		q.Push(2)
		if q.Len() != 1 {
			t.Fatalf("expected len 1 for sanitized queue, got %d", q.Len())
		}

		val, ok := q.Pop()
		if !ok || val != 2 {
			t.Fatalf("expected (2, true) after overwrite, got (%d, %v)", val, ok)
		}
	}
}

func TestPushPop(t *testing.T) {
	q := NewQueue[int](3)

	q.Push(1)
	q.Push(2)
	q.Push(3)

	if q.Len() != 3 {
		t.Errorf("expected len 3, got %d", q.Len())
	}

	val, ok := q.Pop()
	if !ok || val != 1 {
		t.Errorf("expected (1, true), got (%d, %v)", val, ok)
	}

	val, ok = q.Pop()
	if !ok || val != 2 {
		t.Errorf("expected (2, true), got (%d, %v)", val, ok)
	}

	val, ok = q.Pop()
	if !ok || val != 3 {
		t.Errorf("expected (3, true), got (%d, %v)", val, ok)
	}

	val, ok = q.Pop()
	if ok {
		t.Errorf("expected (_, false), got (%d, %v)", val, ok)
	}
}

func TestPushDropsOldest(t *testing.T) {
	q := NewQueue[int](3)

	q.Push(1)
	q.Push(2)
	q.Push(3)
	q.Push(4) // should drop 1

	if q.Len() != 3 {
		t.Errorf("expected len 3, got %d", q.Len())
	}

	val, ok := q.Pop()
	if !ok || val != 2 {
		t.Errorf("expected (2, true), got (%d, %v)", val, ok)
	}

	val, ok = q.Pop()
	if !ok || val != 3 {
		t.Errorf("expected (3, true), got (%d, %v)", val, ok)
	}

	val, ok = q.Pop()
	if !ok || val != 4 {
		t.Errorf("expected (4, true), got (%d, %v)", val, ok)
	}
}

func TestPeek(t *testing.T) {
	q := NewQueue[string](2)

	_, ok := q.Peek()
	if ok {
		t.Error("expected Peek on empty queue to return false")
	}

	q.Push("a")
	q.Push("b")

	val, ok := q.Peek()
	if !ok || val != "a" {
		t.Errorf("expected (a, true), got (%s, %v)", val, ok)
	}

	// Peek should not remove
	if q.Len() != 2 {
		t.Errorf("Peek should not modify queue, len is %d", q.Len())
	}
}

func TestIsEmpty(t *testing.T) {
	q := NewQueue[int](2)

	if !q.IsEmpty() {
		t.Error("new queue should be empty")
	}

	q.Push(1)
	if q.IsEmpty() {
		t.Error("queue with item should not be empty")
	}

	q.Pop()
	if !q.IsEmpty() {
		t.Error("queue after pop should be empty")
	}
}

func TestConcurrentAccess(t *testing.T) {
	q := NewQueue[int](100)
	var wg sync.WaitGroup

	// Concurrent pushes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				q.Push(n*20 + j)
			}
		}(i)
	}

	// Concurrent pops
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				q.Pop()
			}
		}()
	}

	wg.Wait()

	// Should not panic and queue should be in valid state
	_ = q.Len()
	_ = q.IsEmpty()
}

func TestCapacityOne(t *testing.T) {
	q := NewQueue[int](1)

	q.Push(1)
	q.Push(2) // drops 1

	if q.Len() != 1 {
		t.Errorf("expected len 1, got %d", q.Len())
	}

	val, ok := q.Pop()
	if !ok || val != 2 {
		t.Errorf("expected (2, true), got (%d, %v)", val, ok)
	}
}

func TestPushSanitizesInvalidManualQueueCapacity(t *testing.T) {
	q := &Queue[int]{capacity: 0}

	q.Push(1)
	q.Push(2)

	if q.Len() != 0 {
		t.Errorf("expected len 0, got %d", q.Len())
	}

	if _, ok := q.Pop(); ok {
		t.Error("expected pop on zero-capacity queue to return false")
	}
}

func TestNegativeCapacityDoesNotPanicAndBehavesAsZero(t *testing.T) {
	q := New[int](-5)

	q.Push(1)

	if q.Len() != 0 {
		t.Errorf("expected len 0, got %d", q.Len())
	}

	if _, ok := q.Peek(); ok {
		t.Error("expected peek on negative-capacity queue to return false")
	}
}
