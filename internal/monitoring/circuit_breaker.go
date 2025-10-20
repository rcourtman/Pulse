package monitoring

import (
	"sync"
	"time"
)

type breakerState int

const (
	breakerClosed breakerState = iota
	breakerOpen
	breakerHalfOpen
)

type circuitBreaker struct {
	mu             sync.Mutex
	state          breakerState
	failureCount   int
	openedAt       time.Time
	lastAttempt    time.Time
	retryInterval  time.Duration
	maxDelay       time.Duration
	openThreshold  int
	halfOpenWindow time.Duration
}

func newCircuitBreaker(openThreshold int, retryInterval, maxDelay, halfOpenWindow time.Duration) *circuitBreaker {
	if openThreshold <= 0 {
		openThreshold = 3
	}
	if retryInterval <= 0 {
		retryInterval = 5 * time.Second
	}
	if maxDelay <= 0 {
		maxDelay = 5 * time.Minute
	}
	if halfOpenWindow <= 0 {
		halfOpenWindow = 30 * time.Second
	}
	return &circuitBreaker{
		state:          breakerClosed,
		retryInterval:  retryInterval,
		maxDelay:       maxDelay,
		openThreshold:  openThreshold,
		halfOpenWindow: halfOpenWindow,
	}
}

func (b *circuitBreaker) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case breakerClosed:
		return true
	case breakerOpen:
		if now.Sub(b.openedAt) >= b.retryInterval {
			b.state = breakerHalfOpen
			b.lastAttempt = now
			return true
		}
		return false
	case breakerHalfOpen:
		if now.Sub(b.lastAttempt) >= b.halfOpenWindow {
			b.lastAttempt = now
			return true
		}
		return false
	default:
		return true
	}
}

func (b *circuitBreaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state != breakerClosed {
		b.state = breakerClosed
		b.failureCount = 0
	}
}

func (b *circuitBreaker) recordFailure(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failureCount++
	b.lastAttempt = now

	switch b.state {
	case breakerHalfOpen:
		b.trip(now)
	case breakerClosed:
		if b.failureCount >= b.openThreshold {
			b.trip(now)
		}
	}
}

func (b *circuitBreaker) trip(now time.Time) {
	b.state = breakerOpen
	delay := b.retryInterval << uint(b.failureCount)
	if delay > b.maxDelay {
		delay = b.maxDelay
	}
	b.retryInterval = delay
	b.openedAt = now
}
