package utils

import (
	"sync"
)

// SafeMap is a generic thread-safe map
type SafeMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

// NewSafeMap creates a new thread-safe map
func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{
		m: make(map[K]V),
	}
}

// NewSafeMapWithCapacity creates a new thread-safe map with initial capacity
func NewSafeMapWithCapacity[K comparable, V any](capacity int) *SafeMap[K, V] {
	return &SafeMap[K, V]{
		m: make(map[K]V, capacity),
	}
}

// Get retrieves a value from the map
func (sm *SafeMap[K, V]) Get(key K) (V, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	val, ok := sm.m[key]
	return val, ok
}

// Set sets a value in the map
func (sm *SafeMap[K, V]) Set(key K, value V) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m[key] = value
}

// Delete removes a key from the map
func (sm *SafeMap[K, V]) Delete(key K) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.m, key)
}

// Len returns the number of items in the map
func (sm *SafeMap[K, V]) Len() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.m)
}

// Keys returns all keys in the map
func (sm *SafeMap[K, V]) Keys() []K {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	keys := make([]K, 0, len(sm.m))
	for k := range sm.m {
		keys = append(keys, k)
	}
	return keys
}

// Values returns all values in the map
func (sm *SafeMap[K, V]) Values() []V {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	values := make([]V, 0, len(sm.m))
	for _, v := range sm.m {
		values = append(values, v)
	}
	return values
}

// Clear removes all items from the map
func (sm *SafeMap[K, V]) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m = make(map[K]V)
}

// Range iterates over the map with a callback function
func (sm *SafeMap[K, V]) Range(f func(key K, value V) bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for k, v := range sm.m {
		if !f(k, v) {
			break
		}
	}
}

// CircularBuffer is a generic circular buffer for storing time-series data
type CircularBuffer[T any] struct {
	mu       sync.RWMutex
	data     []T
	capacity int
	head     int
	tail     int
	size     int
}

// NewCircularBuffer creates a new circular buffer
func NewCircularBuffer[T any](capacity int) *CircularBuffer[T] {
	return &CircularBuffer[T]{
		data:     make([]T, capacity),
		capacity: capacity,
	}
}

// Push adds an item to the buffer
func (cb *CircularBuffer[T]) Push(item T) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.data[cb.tail] = item
	cb.tail = (cb.tail + 1) % cb.capacity

	if cb.size < cb.capacity {
		cb.size++
	} else {
		cb.head = (cb.head + 1) % cb.capacity
	}
}

// GetAll returns all items in the buffer in order
func (cb *CircularBuffer[T]) GetAll() []T {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.size == 0 {
		return []T{}
	}

	result := make([]T, cb.size)
	if cb.head < cb.tail {
		copy(result, cb.data[cb.head:cb.tail])
	} else {
		n := copy(result, cb.data[cb.head:])
		copy(result[n:], cb.data[:cb.tail])
	}

	return result
}

// Size returns the current number of items in the buffer
func (cb *CircularBuffer[T]) Size() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.size
}

// Clear empties the buffer
func (cb *CircularBuffer[T]) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.head = 0
	cb.tail = 0
	cb.size = 0
}

// SliceContains checks if a slice contains a value
func SliceContains[T comparable](slice []T, value T) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// SliceMap applies a function to each element of a slice
func SliceMap[T any, U any](slice []T, f func(T) U) []U {
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

// SliceFilter filters a slice based on a predicate
func SliceFilter[T any](slice []T, predicate func(T) bool) []T {
	result := make([]T, 0, len(slice))
	for _, v := range slice {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

// SliceReduce reduces a slice to a single value
func SliceReduce[T any, U any](slice []T, initial U, reducer func(U, T) U) U {
	result := initial
	for _, v := range slice {
		result = reducer(result, v)
	}
	return result
}

// SliceUnique returns unique elements from a slice
func SliceUnique[T comparable](slice []T) []T {
	seen := make(map[T]struct{})
	result := make([]T, 0, len(slice))
	for _, v := range slice {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// Ptr returns a pointer to the given value
func Ptr[T any](v T) *T {
	return &v
}

// ValueOr returns the value if not nil, otherwise returns the default
func ValueOr[T any](ptr *T, defaultValue T) T {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

// Result represents a result that can be either a value or an error
type Result[T any] struct {
	value T
	err   error
}

// NewResult creates a new Result
func NewResult[T any](value T, err error) Result[T] {
	return Result[T]{value: value, err: err}
}

// Ok creates a successful Result
func Ok[T any](value T) Result[T] {
	return Result[T]{value: value}
}

// Err creates an error Result
func Err[T any](err error) Result[T] {
	var zero T
	return Result[T]{value: zero, err: err}
}

// Value returns the value and error
func (r Result[T]) Value() (T, error) {
	return r.value, r.err
}

// IsOk returns true if the result is successful
func (r Result[T]) IsOk() bool {
	return r.err == nil
}

// IsErr returns true if the result is an error
func (r Result[T]) IsErr() bool {
	return r.err != nil
}

// Unwrap returns the value or panics if there's an error
// Note: Use UnwrapOr for safer error handling
func (r Result[T]) Unwrap() T {
	if r.err != nil {
		panic(r.err)
	}
	return r.value
}

// UnwrapSafe returns the value and error without panicking
func (r Result[T]) UnwrapSafe() (T, error) {
	return r.value, r.err
}

// UnwrapOr returns the value or a default if there's an error
func (r Result[T]) UnwrapOr(defaultValue T) T {
	if r.err != nil {
		return defaultValue
	}
	return r.value
}

// Map transforms the value if successful
func (r Result[T]) Map(f func(T) T) Result[T] {
	if r.err != nil {
		return r
	}
	return Ok(f(r.value))
}
