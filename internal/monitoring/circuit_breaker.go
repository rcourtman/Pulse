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
	mu                sync.Mutex
	state             breakerState
	failureCount      int
	openedAt          time.Time
	lastAttempt       time.Time
	retryInterval     time.Duration
	baseRetryInterval time.Duration
	maxDelay          time.Duration
	openThreshold     int
	halfOpenWindow    time.Duration
	stateSince        time.Time
	lastTransition    time.Time
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
	now := time.Now()
	return &circuitBreaker{
		state:             breakerClosed,
		retryInterval:     retryInterval,
		baseRetryInterval: retryInterval,
		maxDelay:          maxDelay,
		openThreshold:     openThreshold,
		halfOpenWindow:    halfOpenWindow,
		stateSince:        now,
		lastTransition:    now,
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
			b.stateSince = now
			b.lastTransition = now
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
		now := time.Now()
		b.state = breakerClosed
		b.failureCount = 0
		b.retryInterval = b.baseRetryInterval
		b.stateSince = now
		b.lastTransition = now
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
	b.stateSince = now
	b.lastTransition = now
}

// BreakerSnapshot represents the current state of a circuit breaker.
type BreakerSnapshot struct {
	Instance string    `json:"instance"`
	Type     string    `json:"type"`
	State    string    `json:"state"`
	Failures int       `json:"failures"`
	RetryAt  time.Time `json:"retryAt,omitempty"`
}

// State returns a snapshot of the circuit breaker state for API exposure.
func (b *circuitBreaker) State() (state string, failures int, retryAt time.Time) {
	state, failures, retryAt, _, _ = b.stateDetails()
	return
}

func (b *circuitBreaker) stateDetails() (state string, failures int, retryAt time.Time, since time.Time, lastTransition time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case breakerClosed:
		state = "closed"
	case breakerOpen:
		state = "open"
		retryAt = b.openedAt.Add(b.retryInterval)
	case breakerHalfOpen:
		state = "half_open"
		retryAt = b.lastAttempt.Add(b.halfOpenWindow)
	default:
		state = "unknown"
	}

	return state, b.failureCount, retryAt, b.stateSince, b.lastTransition
}
