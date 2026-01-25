package circuit

import (
	"errors"
	"testing"
	"time"
)

func TestStateString_Unknown(t *testing.T) {
	if got := State(99).String(); got != "unknown" {
		t.Fatalf("expected unknown, got %s", got)
	}
}

func TestNewBreaker_DefaultsApplied(t *testing.T) {
	b := NewBreaker("defaults", Config{})

	if b.config.FailureThreshold != 3 {
		t.Fatalf("expected default FailureThreshold, got %d", b.config.FailureThreshold)
	}
	if b.config.SuccessThreshold != 2 {
		t.Fatalf("expected default SuccessThreshold, got %d", b.config.SuccessThreshold)
	}
	if b.config.InitialBackoff != time.Second {
		t.Fatalf("expected default InitialBackoff, got %s", b.config.InitialBackoff)
	}
	if b.config.MaxBackoff != 5*time.Minute {
		t.Fatalf("expected default MaxBackoff, got %s", b.config.MaxBackoff)
	}
	if b.config.BackoffMultiplier != 2.0 {
		t.Fatalf("expected default BackoffMultiplier, got %.1f", b.config.BackoffMultiplier)
	}
	if b.config.HalfOpenTimeout != 30*time.Second {
		t.Fatalf("expected default HalfOpenTimeout, got %s", b.config.HalfOpenTimeout)
	}
}

func TestBreaker_CanAllow_DoesNotTransition(t *testing.T) {
	b := NewBreaker("test", DefaultConfig())
	b.mu.Lock()
	b.state = StateOpen
	b.currentBackoff = time.Hour
	b.openedAt = time.Now().Add(-2 * time.Hour)
	b.mu.Unlock()

	if !b.CanAllow() {
		t.Fatalf("expected CanAllow to return true after backoff")
	}
	if b.State() != StateOpen {
		t.Fatalf("expected state to remain open on CanAllow")
	}
}

func TestBreaker_RecordFailure_InvalidDoesNotTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 1
	b := NewBreaker("test", cfg)

	b.RecordFailureWithCategory(errors.New("bad request"), ErrorCategoryInvalid)
	if b.State() != StateClosed {
		t.Fatalf("expected invalid error not to trip circuit")
	}
}

func TestBreaker_RecordFailure_RateLimitTrips(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 5
	b := NewBreaker("test", cfg)

	b.RecordFailureWithCategory(errors.New("rate limit"), ErrorCategoryRateLimit)
	if b.State() != StateOpen {
		t.Fatalf("expected rate limit error to trip circuit")
	}
}

func TestBreaker_RecordFailure_HalfOpenBackoffCaps(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBackoff = 100 * time.Millisecond
	cfg.BackoffMultiplier = 2.0
	b := NewBreaker("test", cfg)

	b.mu.Lock()
	b.state = StateHalfOpen
	b.currentBackoff = 80 * time.Millisecond
	b.mu.Unlock()

	b.RecordFailureWithCategory(errors.New("fail"), ErrorCategoryTransient)
	if b.State() != StateOpen {
		t.Fatalf("expected state to be open after half-open failure")
	}
	if b.currentBackoff != cfg.MaxBackoff {
		t.Fatalf("expected backoff to cap at max, got %s", b.currentBackoff)
	}
}

func TestBreaker_Callbacks(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 1
	b := NewBreaker("test", cfg)

	stateCh := make(chan State, 1)
	tripCh := make(chan error, 1)
	b.SetOnStateChange(func(_, to State) {
		stateCh <- to
	})
	b.SetOnTrip(func(err error) {
		tripCh <- err
	})

	testErr := errors.New("boom")
	b.RecordFailure(testErr)

	select {
	case state := <-stateCh:
		if state != StateOpen {
			t.Fatalf("expected state change to open, got %s", state.String())
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected state change callback to fire")
	}

	select {
	case err := <-tripCh:
		if err == nil || err.Error() != testErr.Error() {
			t.Fatalf("expected trip callback with error")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected trip callback to fire")
	}
}

func TestBreaker_GetStatus_TimeUntilRetry(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 1
	cfg.InitialBackoff = 50 * time.Millisecond
	b := NewBreaker("test", cfg)

	b.RecordFailure(errors.New("fail"))
	status := b.GetStatus()

	if status.State != "open" {
		t.Fatalf("expected status open, got %s", status.State)
	}
	if status.LastError == "" {
		t.Fatalf("expected last error to be set")
	}
	if status.TimeUntilRetry <= 0 {
		t.Fatalf("expected time until retry to be set")
	}
}

func TestBreaker_ExecuteWithCategory_InvalidDoesNotTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 1
	b := NewBreaker("test", cfg)

	err := b.ExecuteWithCategory(func() (error, ErrorCategory) {
		return errors.New("invalid"), ErrorCategoryInvalid
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if b.State() != StateClosed {
		t.Fatalf("expected state to remain closed")
	}
}

func TestIsCircuitOpen_Additional(t *testing.T) {
	if !IsCircuitOpen(ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen to be recognized")
	}
	if IsCircuitOpen(errors.New("other")) {
		t.Fatalf("expected non-circuit error to be false")
	}
}

func TestCategorizeError_Additional(t *testing.T) {
	tests := []struct {
		err      error
		expected ErrorCategory
	}{
		{errors.New("rate limit exceeded"), ErrorCategoryRateLimit},
		{errors.New("429 too many requests"), ErrorCategoryRateLimit},
		{errors.New("400 bad request"), ErrorCategoryInvalid},
		{errors.New("unauthorized api key"), ErrorCategoryFatal},
		{errors.New("payment required"), ErrorCategoryFatal},
		{errors.New("random failure"), ErrorCategoryTransient},
		{nil, ErrorCategoryTransient},
	}

	for _, tt := range tests {
		if got := CategorizeError(tt.err); got != tt.expected {
			t.Fatalf("expected %v, got %v for %v", tt.expected, got, tt.err)
		}
	}
}

func TestBreaker_IsOpenClosed(t *testing.T) {
	b := NewBreaker("test", DefaultConfig())
	if !b.IsClosed() || b.IsOpen() {
		t.Fatalf("expected breaker to start closed")
	}

	b.mu.Lock()
	b.state = StateOpen
	b.mu.Unlock()

	if !b.IsOpen() || b.IsClosed() {
		t.Fatalf("expected breaker to report open")
	}
}

func TestCircuitOpenErrorMessage(t *testing.T) {
	err := circuitOpenError{}
	if err.Error() != "circuit breaker is open" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestBreaker_CanAllow_Branches(t *testing.T) {
	b := NewBreaker("test", DefaultConfig())
	if !b.CanAllow() {
		t.Fatalf("expected CanAllow true for closed")
	}

	b.mu.Lock()
	b.state = StateOpen
	b.openedAt = time.Now()
	b.currentBackoff = time.Hour
	b.mu.Unlock()

	if b.CanAllow() {
		t.Fatalf("expected CanAllow false before backoff elapses")
	}

	b.mu.Lock()
	b.openedAt = time.Now().Add(-2 * time.Hour)
	b.mu.Unlock()
	if !b.CanAllow() {
		t.Fatalf("expected CanAllow true after backoff")
	}

	b.mu.Lock()
	b.state = StateHalfOpen
	b.mu.Unlock()
	if !b.CanAllow() {
		t.Fatalf("expected CanAllow true for half-open")
	}
}

func TestBreaker_ExecuteWithCategory_SuccessAndOpen(t *testing.T) {
	b := NewBreaker("test", DefaultConfig())
	if err := b.ExecuteWithCategory(func() (error, ErrorCategory) {
		return nil, ErrorCategoryTransient
	}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	cfg := DefaultConfig()
	cfg.FailureThreshold = 1
	cfg.InitialBackoff = time.Hour
	b = NewBreaker("test", cfg)
	b.RecordFailure(errors.New("fail"))
	if err := b.ExecuteWithCategory(func() (error, ErrorCategory) {
		return nil, ErrorCategoryTransient
	}); err != ErrCircuitOpen {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestStateString_All(t *testing.T) {
	cases := map[State]string{
		StateClosed:   "closed",
		StateOpen:     "open",
		StateHalfOpen: "half-open",
	}
	for state, expected := range cases {
		if state.String() != expected {
			t.Fatalf("expected %s for state %d", expected, state)
		}
	}
}

func TestBreaker_Allow_TransitionsOpenToHalfOpen(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialBackoff = 10 * time.Millisecond
	b := NewBreaker("test", cfg)

	b.mu.Lock()
	b.state = StateOpen
	b.openedAt = time.Now().Add(-time.Second)
	b.currentBackoff = 10 * time.Millisecond
	b.mu.Unlock()

	if !b.Allow() {
		t.Fatalf("expected Allow to return true after backoff")
	}
	if b.State() != StateHalfOpen {
		t.Fatalf("expected state to transition to half-open")
	}
}

func TestBreaker_TransitionTo_NoOp(t *testing.T) {
	b := NewBreaker("test", DefaultConfig())
	b.transitionTo(StateClosed)
	if b.State() != StateClosed {
		t.Fatalf("expected state to remain closed")
	}
}

func TestBreaker_Allow_BlocksBeforeBackoff(t *testing.T) {
	b := NewBreaker("test", DefaultConfig())
	b.mu.Lock()
	b.state = StateOpen
	b.openedAt = time.Now()
	b.currentBackoff = time.Hour
	b.mu.Unlock()

	if b.Allow() {
		t.Fatalf("expected Allow to return false before backoff elapses")
	}
	if b.State() != StateOpen {
		t.Fatalf("expected state to remain open")
	}
}

func TestBreaker_Allow_HalfOpen(t *testing.T) {
	b := NewBreaker("test", DefaultConfig())
	b.mu.Lock()
	b.state = StateHalfOpen
	b.mu.Unlock()
	if !b.Allow() {
		t.Fatalf("expected Allow true in half-open")
	}
}

func TestToLower(t *testing.T) {
	if toLower("AbC123") != "abc123" {
		t.Fatalf("expected toLower to normalize casing")
	}
	if toLower("lower") != "lower" {
		t.Fatalf("expected lowercase input to remain unchanged")
	}
}
