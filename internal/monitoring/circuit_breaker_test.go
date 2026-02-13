package monitoring

import (
	"testing"
	"time"
)

func TestCircuitBreaker_NewDefaults(t *testing.T) {
	tests := []struct {
		name              string
		openThreshold     int
		retryInterval     time.Duration
		maxDelay          time.Duration
		halfOpenWindow    time.Duration
		wantOpenThreshold int
		wantRetryInterval time.Duration
		wantMaxDelay      time.Duration
		wantHalfOpen      time.Duration
	}{
		{
			name:              "valid config",
			openThreshold:     3,
			retryInterval:     5 * time.Second,
			maxDelay:          5 * time.Minute,
			halfOpenWindow:    30 * time.Second,
			wantOpenThreshold: 3,
			wantRetryInterval: 5 * time.Second,
			wantMaxDelay:      5 * time.Minute,
			wantHalfOpen:      30 * time.Second,
		},
		{
			name:              "zero threshold defaults to 3",
			openThreshold:     0,
			retryInterval:     5 * time.Second,
			maxDelay:          5 * time.Minute,
			halfOpenWindow:    30 * time.Second,
			wantOpenThreshold: 3,
			wantRetryInterval: 5 * time.Second,
			wantMaxDelay:      5 * time.Minute,
			wantHalfOpen:      30 * time.Second,
		},
		{
			name:              "zero retry interval defaults to 5s",
			openThreshold:     3,
			retryInterval:     0,
			maxDelay:          5 * time.Minute,
			halfOpenWindow:    30 * time.Second,
			wantOpenThreshold: 3,
			wantRetryInterval: 5 * time.Second,
			wantMaxDelay:      5 * time.Minute,
			wantHalfOpen:      30 * time.Second,
		},
		{
			name:              "zero max delay defaults to 5min",
			openThreshold:     3,
			retryInterval:     5 * time.Second,
			maxDelay:          0,
			halfOpenWindow:    30 * time.Second,
			wantOpenThreshold: 3,
			wantRetryInterval: 5 * time.Second,
			wantMaxDelay:      5 * time.Minute,
			wantHalfOpen:      30 * time.Second,
		},
		{
			name:              "zero half-open window defaults to 30s",
			openThreshold:     3,
			retryInterval:     5 * time.Second,
			maxDelay:          5 * time.Minute,
			halfOpenWindow:    0,
			wantOpenThreshold: 3,
			wantRetryInterval: 5 * time.Second,
			wantMaxDelay:      5 * time.Minute,
			wantHalfOpen:      30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := newCircuitBreaker(tt.openThreshold, tt.retryInterval, tt.maxDelay, tt.halfOpenWindow)
			if cb.openThreshold != tt.wantOpenThreshold {
				t.Errorf("openThreshold = %d, want %d", cb.openThreshold, tt.wantOpenThreshold)
			}
			if cb.retryInterval != tt.wantRetryInterval {
				t.Errorf("retryInterval = %v, want %v", cb.retryInterval, tt.wantRetryInterval)
			}
			if cb.maxDelay != tt.wantMaxDelay {
				t.Errorf("maxDelay = %v, want %v", cb.maxDelay, tt.wantMaxDelay)
			}
			if cb.halfOpenWindow != tt.wantHalfOpen {
				t.Errorf("halfOpenWindow = %v, want %v", cb.halfOpenWindow, tt.wantHalfOpen)
			}
			if cb.state != breakerClosed {
				t.Errorf("initial state = %v, want closed", cb.state)
			}
		})
	}
}

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
	now := time.Now()

	// Breaker should start closed
	if !cb.allow(now) {
		t.Error("closed breaker should allow requests")
	}

	// First two failures should keep it closed
	cb.recordFailure(now)
	if !cb.allow(now) {
		t.Error("breaker should still be closed after 1 failure (threshold 3)")
	}

	cb.recordFailure(now.Add(time.Second))
	if !cb.allow(now.Add(time.Second)) {
		t.Error("breaker should still be closed after 2 failures (threshold 3)")
	}

	// Third failure should trip it
	cb.recordFailure(now.Add(2 * time.Second))
	if cb.allow(now.Add(2 * time.Second)) {
		t.Error("breaker should be open after 3 failures")
	}

	// Verify state
	state, failures, _ := cb.State()
	if state != "open" {
		t.Errorf("state = %s, want open", state)
	}
	if failures != 3 {
		t.Errorf("failures = %d, want 3", failures)
	}
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
	now := time.Now()

	// Trip the breaker
	for i := 0; i < 3; i++ {
		cb.recordFailure(now)
	}

	// Should be open
	if cb.allow(now) {
		t.Error("breaker should be open")
	}

	// With 3 failures, retry interval is 5s << 3 = 40s
	// Before retry interval, should still be open
	if cb.allow(now.Add(39 * time.Second)) {
		t.Error("breaker should still be open before retry interval")
	}

	// After retry interval (40s), should transition to half-open and allow
	if !cb.allow(now.Add(41 * time.Second)) {
		t.Error("breaker should transition to half-open after retry interval")
	}

	state, _, _ := cb.State()
	if state != "half_open" {
		t.Errorf("state = %s, want half_open", state)
	}
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
	now := time.Now()

	// Trip the breaker
	for i := 0; i < 3; i++ {
		cb.recordFailure(now)
	}

	// Transition to half-open (retry interval is 5s << 3 = 40s)
	cb.allow(now.Add(41 * time.Second))

	// Success should close it
	cb.recordSuccess()

	state, failures, _ := cb.State()
	if state != "closed" {
		t.Errorf("state = %s, want closed", state)
	}
	if failures != 0 {
		t.Errorf("failures = %d, want 0", failures)
	}

	// Should allow requests again
	if !cb.allow(now.Add(50 * time.Second)) {
		t.Error("closed breaker should allow requests")
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
	now := time.Now()

	// Trip the breaker
	for i := 0; i < 3; i++ {
		cb.recordFailure(now)
	}

	// Transition to half-open (retry interval is 5s << 3 = 40s)
	cb.allow(now.Add(41 * time.Second))

	// Failure should trip it back to open
	cb.recordFailure(now.Add(42 * time.Second))

	state, _, _ := cb.State()
	if state != "open" {
		t.Errorf("state = %s, want open", state)
	}

	// Should not allow requests (need to wait for new retry interval)
	if cb.allow(now.Add(50 * time.Second)) {
		t.Error("breaker should be open after failure in half-open state")
	}
}

func TestCircuitBreaker_HalfOpenWindow(t *testing.T) {
	cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 10*time.Second)
	now := time.Now()

	// Trip and transition to half-open
	for i := 0; i < 3; i++ {
		cb.recordFailure(now)
	}
	// Retry interval is 5s << 3 = 40s
	checkTime := now.Add(41 * time.Second)
	if !cb.allow(checkTime) {
		t.Error("breaker should transition to half-open and allow first request")
	}

	// Subsequent requests within half-open window should be denied
	if cb.allow(checkTime.Add(1 * time.Second)) {
		t.Error("second request within half-open window should be denied")
	}
	if cb.allow(checkTime.Add(5 * time.Second)) {
		t.Error("third request within half-open window should be denied")
	}

	// After window expires (10s), another request should be allowed
	if !cb.allow(checkTime.Add(11 * time.Second)) {
		t.Error("request after half-open window should be allowed")
	}
}

func TestCircuitBreaker_RetryIntervalBackoff(t *testing.T) {
	cb := newCircuitBreaker(3, 5*time.Second, 1*time.Minute, 30*time.Second)
	now := time.Now()

	// Trip the breaker (3 failures)
	for i := 0; i < 3; i++ {
		cb.recordFailure(now)
	}

	// After first trip, retry interval is 5s << 3 = 40s
	if cb.retryInterval != 40*time.Second {
		t.Errorf("retryInterval after first trip = %v, want 40s", cb.retryInterval)
	}

	// Transition to half-open and fail again (4th failure)
	cb.allow(now.Add(41 * time.Second))
	cb.recordFailure(now.Add(41 * time.Second))

	// After second trip with 4 failures, retry interval is 40s << 4 = 640s = 10m40s
	// But it should be capped at maxDelay (1 minute)
	if cb.retryInterval != 1*time.Minute {
		t.Errorf("retryInterval after second trip = %v, want %v (capped at maxDelay)", cb.retryInterval, 1*time.Minute)
	}

	// All subsequent failures should keep it at maxDelay
	for i := 0; i < 5; i++ {
		cb.allow(now.Add(time.Duration(42+i*2) * time.Minute))
		cb.recordFailure(now.Add(time.Duration(42+i*2) * time.Minute))
	}

	if cb.retryInterval != 1*time.Minute {
		t.Errorf("retryInterval = %v, should remain capped at %v", cb.retryInterval, 1*time.Minute)
	}
}

func TestCircuitBreaker_SuccessInClosedState(t *testing.T) {
	cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)

	// Record a failure first
	cb.recordFailure(time.Now())
	if cb.failureCount != 1 {
		t.Errorf("failureCount = %d, want 1", cb.failureCount)
	}

	// Success in closed state should not change state but should reset if not already closed
	cb.recordSuccess()

	state, _, _ := cb.State()
	if state != "closed" {
		t.Errorf("state = %s, want closed", state)
	}
	if cb.failureCount != 0 {
		t.Errorf("failureCount = %d, want 0 after success", cb.failureCount)
	}
}

func TestCircuitBreaker_NonConsecutiveFailuresDoNotTrip(t *testing.T) {
	cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
	now := time.Now()

	cb.recordFailure(now)
	cb.recordSuccess()
	cb.recordFailure(now.Add(time.Second))
	cb.recordFailure(now.Add(2 * time.Second))

	// Breaker should remain closed because failures were not consecutive across the success.
	if !cb.allow(now.Add(2 * time.Second)) {
		t.Fatal("breaker should remain closed after non-consecutive failures")
	}
	state, failures, _ := cb.State()
	if state != "closed" {
		t.Fatalf("state = %s, want closed", state)
	}
	if failures != 2 {
		t.Fatalf("failures = %d, want 2 consecutive failures since last success", failures)
	}

	// Third consecutive failure should now trip the breaker.
	cb.recordFailure(now.Add(3 * time.Second))
	if cb.allow(now.Add(3 * time.Second)) {
		t.Fatal("breaker should open on third consecutive failure")
	}
}

func TestCircuitBreaker_State(t *testing.T) {
	cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
	now := time.Now()

	// Test closed state
	state, failures, retryAt := cb.State()
	if state != "closed" {
		t.Errorf("initial state = %s, want closed", state)
	}
	if failures != 0 {
		t.Errorf("initial failures = %d, want 0", failures)
	}
	if !retryAt.IsZero() {
		t.Error("retryAt should be zero for closed state")
	}

	// Trip to open
	for i := 0; i < 3; i++ {
		cb.recordFailure(now)
	}

	state, failures, retryAt = cb.State()
	if state != "open" {
		t.Errorf("state = %s, want open", state)
	}
	if failures != 3 {
		t.Errorf("failures = %d, want 3", failures)
	}
	// Retry interval is 5s << 3 = 40s
	expectedRetry := now.Add(40 * time.Second)
	if retryAt.Before(expectedRetry.Add(-time.Millisecond)) || retryAt.After(expectedRetry.Add(time.Millisecond)) {
		t.Errorf("retryAt = %v, want ~%v", retryAt, expectedRetry)
	}

	// Transition to half-open
	cb.allow(now.Add(41 * time.Second))
	state, _, retryAt = cb.State()
	if state != "half_open" {
		t.Errorf("state = %s, want half_open", state)
	}
	if retryAt.IsZero() {
		t.Error("retryAt should not be zero for half_open state")
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
	now := time.Now()

	// Test that concurrent access doesn't panic
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			cb.allow(now)
			cb.recordFailure(now)
			cb.recordSuccess()
			cb.State()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestCircuitBreaker_Allow(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(cb *circuitBreaker, now time.Time)
		timeOffset     time.Duration
		want           bool
		wantStateAfter breakerState
	}{
		{
			name:           "closed state always allows",
			setup:          func(cb *circuitBreaker, now time.Time) {},
			timeOffset:     0,
			want:           true,
			wantStateAfter: breakerClosed,
		},
		{
			name: "open state denies before retry interval",
			setup: func(cb *circuitBreaker, now time.Time) {
				cb.state = breakerOpen
				cb.openedAt = now
				cb.retryInterval = 10 * time.Second
			},
			timeOffset:     5 * time.Second,
			want:           false,
			wantStateAfter: breakerOpen,
		},
		{
			name: "open state allows and transitions to half-open after retry interval",
			setup: func(cb *circuitBreaker, now time.Time) {
				cb.state = breakerOpen
				cb.openedAt = now
				cb.retryInterval = 10 * time.Second
			},
			timeOffset:     10 * time.Second,
			want:           true,
			wantStateAfter: breakerHalfOpen,
		},
		{
			name: "half-open state denies during window",
			setup: func(cb *circuitBreaker, now time.Time) {
				cb.state = breakerHalfOpen
				cb.lastAttempt = now
				cb.halfOpenWindow = 30 * time.Second
			},
			timeOffset:     15 * time.Second,
			want:           false,
			wantStateAfter: breakerHalfOpen,
		},
		{
			name: "half-open state allows after window passed",
			setup: func(cb *circuitBreaker, now time.Time) {
				cb.state = breakerHalfOpen
				cb.lastAttempt = now
				cb.halfOpenWindow = 30 * time.Second
			},
			timeOffset:     30 * time.Second,
			want:           true,
			wantStateAfter: breakerHalfOpen,
		},
		{
			name: "unknown state allows (default branch)",
			setup: func(cb *circuitBreaker, now time.Time) {
				cb.state = breakerState(99)
			},
			timeOffset:     0,
			want:           true,
			wantStateAfter: breakerState(99),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
			now := time.Now()
			tt.setup(cb, now)

			got := cb.allow(now.Add(tt.timeOffset))

			if got != tt.want {
				t.Errorf("allow() = %v, want %v", got, tt.want)
			}
			if cb.state != tt.wantStateAfter {
				t.Errorf("state after allow() = %v, want %v", cb.state, tt.wantStateAfter)
			}
		})
	}
}

func TestCircuitBreaker_StateDetails(t *testing.T) {
	t.Run("closed state", func(t *testing.T) {
		cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
		beforeCreate := time.Now().Add(-time.Millisecond)

		state, failures, retryAt, since, lastTransition := cb.stateDetails()

		if state != "closed" {
			t.Errorf("state = %s, want closed", state)
		}
		if failures != 0 {
			t.Errorf("failures = %d, want 0", failures)
		}
		if !retryAt.IsZero() {
			t.Errorf("retryAt = %v, want zero time", retryAt)
		}
		if since.Before(beforeCreate) {
			t.Errorf("since = %v, should be after %v", since, beforeCreate)
		}
		if lastTransition.Before(beforeCreate) {
			t.Errorf("lastTransition = %v, should be after %v", lastTransition, beforeCreate)
		}
	})

	t.Run("open state", func(t *testing.T) {
		cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
		now := time.Now()

		// Trip the breaker to open state
		for i := 0; i < 3; i++ {
			cb.recordFailure(now)
		}

		state, failures, retryAt, since, lastTransition := cb.stateDetails()

		if state != "open" {
			t.Errorf("state = %s, want open", state)
		}
		if failures != 3 {
			t.Errorf("failures = %d, want 3", failures)
		}
		// retryInterval after 3 failures is 5s << 3 = 40s
		expectedRetryAt := now.Add(40 * time.Second)
		if retryAt.Sub(expectedRetryAt).Abs() > time.Millisecond {
			t.Errorf("retryAt = %v, want %v", retryAt, expectedRetryAt)
		}
		// since should be set to when breaker was tripped
		if since.Sub(now).Abs() > time.Millisecond {
			t.Errorf("since = %v, want ~%v", since, now)
		}
		if lastTransition.Sub(now).Abs() > time.Millisecond {
			t.Errorf("lastTransition = %v, want ~%v", lastTransition, now)
		}
	})

	t.Run("half_open state", func(t *testing.T) {
		cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)
		now := time.Now()

		// Trip the breaker
		for i := 0; i < 3; i++ {
			cb.recordFailure(now)
		}

		// Transition to half-open (retry interval is 5s << 3 = 40s)
		halfOpenTime := now.Add(41 * time.Second)
		cb.allow(halfOpenTime)

		state, failures, retryAt, since, lastTransition := cb.stateDetails()

		if state != "half_open" {
			t.Errorf("state = %s, want half_open", state)
		}
		if failures != 3 {
			t.Errorf("failures = %d, want 3", failures)
		}
		// retryAt should be lastAttempt + halfOpenWindow (30s)
		expectedRetryAt := halfOpenTime.Add(30 * time.Second)
		if retryAt.Sub(expectedRetryAt).Abs() > time.Millisecond {
			t.Errorf("retryAt = %v, want %v", retryAt, expectedRetryAt)
		}
		// since should be when we transitioned to half-open
		if since.Sub(halfOpenTime).Abs() > time.Millisecond {
			t.Errorf("since = %v, want ~%v", since, halfOpenTime)
		}
		if lastTransition.Sub(halfOpenTime).Abs() > time.Millisecond {
			t.Errorf("lastTransition = %v, want ~%v", lastTransition, halfOpenTime)
		}
	})

	t.Run("unknown state (invalid internal state)", func(t *testing.T) {
		cb := newCircuitBreaker(3, 5*time.Second, 5*time.Minute, 30*time.Second)

		// Directly set state to an invalid value to trigger default case
		cb.state = breakerState(99)

		state, _, _, _, _ := cb.stateDetails()

		if state != "unknown" {
			t.Errorf("state = %s, want unknown", state)
		}
	})
}
