package circuit

import (
	"errors"
	"testing"
	"time"
)

func TestBreaker_InitialState(t *testing.T) {
	b := NewBreaker("test", DefaultConfig())

	if b.State() != StateClosed {
		t.Errorf("Expected initial state to be Closed, got %s", b.State())
	}

	if !b.Allow() {
		t.Error("Expected Allow() to return true in Closed state")
	}
}

func TestBreaker_TransitionToOpen(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 3
	b := NewBreaker("test", cfg)

	// Record failures
	for i := 0; i < 3; i++ {
		b.RecordFailure(errors.New("test error"))
	}

	if b.State() != StateOpen {
		t.Errorf("Expected state to be Open after %d failures, got %s", cfg.FailureThreshold, b.State())
	}

	if b.Allow() {
		t.Error("Expected Allow() to return false in Open state")
	}
}

func TestBreaker_RecordSuccess_ResetFailures(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 3
	b := NewBreaker("test", cfg)

	// Record some failures but not enough to trip
	b.RecordFailure(errors.New("error 1"))
	b.RecordFailure(errors.New("error 2"))

	// Success should reset
	b.RecordSuccess()

	// Now failures should need to start from 0
	b.RecordFailure(errors.New("error 1"))
	b.RecordFailure(errors.New("error 2"))

	if b.State() != StateClosed {
		t.Error("Expected state to remain Closed after success reset")
	}
}

func TestBreaker_HalfOpen(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 2
	cfg.InitialBackoff = 10 * time.Millisecond
	cfg.MaxBackoff = 10 * time.Millisecond
	b := NewBreaker("test", cfg)

	// Trip the breaker
	b.RecordFailure(errors.New("error 1"))
	b.RecordFailure(errors.New("error 2"))

	if b.State() != StateOpen {
		t.Fatalf("Expected state to be Open, got %s", b.State())
	}

	// Wait for backoff
	time.Sleep(15 * time.Millisecond)

	// Should transition to half-open and allow one request
	if !b.Allow() {
		t.Error("Expected Allow() to return true after backoff period")
	}

	if b.State() != StateHalfOpen {
		t.Errorf("Expected state to be HalfOpen, got %s", b.State())
	}
}

func TestBreaker_HalfOpen_Success(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 2
	cfg.SuccessThreshold = 1
	cfg.InitialBackoff = 10 * time.Millisecond
	cfg.MaxBackoff = 10 * time.Millisecond
	b := NewBreaker("test", cfg)

	// Trip the breaker
	b.RecordFailure(errors.New("error 1"))
	b.RecordFailure(errors.New("error 2"))

	// Wait for backoff
	time.Sleep(15 * time.Millisecond)

	// Allow (transitions to half-open)
	b.Allow()

	// Success in half-open should close the circuit
	b.RecordSuccess()

	if b.State() != StateClosed {
		t.Errorf("Expected state to be Closed after success in HalfOpen, got %s", b.State())
	}
}

func TestBreaker_HalfOpen_Failure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 2
	cfg.InitialBackoff = 10 * time.Millisecond
	cfg.MaxBackoff = 100 * time.Millisecond
	b := NewBreaker("test", cfg)

	// Trip the breaker
	b.RecordFailure(errors.New("error 1"))
	b.RecordFailure(errors.New("error 2"))

	// Wait for backoff
	time.Sleep(15 * time.Millisecond)

	// Allow (transitions to half-open)
	b.Allow()

	// Failure in half-open should re-open with increased backoff
	b.RecordFailure(errors.New("another error"))

	if b.State() != StateOpen {
		t.Errorf("Expected state to be Open after failure in HalfOpen, got %s", b.State())
	}
}

func TestBreaker_Execute_Success(t *testing.T) {
	b := NewBreaker("test", DefaultConfig())

	err := b.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if b.State() != StateClosed {
		t.Error("Expected state to remain Closed")
	}
}

func TestBreaker_Execute_Failure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 1
	b := NewBreaker("test", cfg)

	testErr := errors.New("operation failed")
	err := b.Execute(func() error {
		return testErr
	})

	if err != testErr {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}

	if b.State() != StateOpen {
		t.Errorf("Expected state to be Open, got %s", b.State())
	}
}

func TestBreaker_Execute_CircuitOpen(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 1
	cfg.InitialBackoff = time.Hour // Long backoff so it stays open
	b := NewBreaker("test", cfg)

	// Trip the breaker
	b.RecordFailure(errors.New("error"))

	// Try to execute - should be blocked
	err := b.Execute(func() error {
		return nil
	})

	if !IsCircuitOpen(err) {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}
}

func TestBreaker_Stats(t *testing.T) {
	b := NewBreaker("test", DefaultConfig())

	// Record some activity
	b.RecordSuccess()
	b.RecordSuccess()
	b.RecordFailure(errors.New("error"))

	status := b.GetStatus()

	if status.State != "closed" {
		t.Errorf("Expected state 'closed', got %s", status.State)
	}

	if status.TotalSuccesses != 2 {
		t.Errorf("Expected 2 successes, got %d", status.TotalSuccesses)
	}

	if status.TotalFailures != 1 {
		t.Errorf("Expected 1 failure, got %d", status.TotalFailures)
	}
}

func TestBreaker_Reset(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FailureThreshold = 2
	b := NewBreaker("test", cfg)

	// Trip the breaker
	b.RecordFailure(errors.New("error 1"))
	b.RecordFailure(errors.New("error 2"))

	if b.State() != StateOpen {
		t.Fatal("Expected state to be Open")
	}

	// Reset
	b.Reset()

	if b.State() != StateClosed {
		t.Errorf("Expected state to be Closed after reset, got %s", b.State())
	}

	if !b.Allow() {
		t.Error("Expected Allow() to return true after reset")
	}
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorCategory
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: ErrorCategoryTransient,
		},
		{
			name:     "timeout error",
			err:      errors.New("context deadline exceeded"),
			expected: ErrorCategoryTransient,
		},
		{
			name:     "rate limit error",
			err:      errors.New("rate limit exceeded"),
			expected: ErrorCategoryRateLimit,
		},
		{
			name:     "invalid request",
			err:      errors.New("invalid request format"),
			expected: ErrorCategoryInvalid,
		},
		{
			name:     "generic error",
			err:      errors.New("something went wrong"),
			expected: ErrorCategoryTransient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CategorizeError(tt.err)
			if got != tt.expected {
				t.Errorf("CategorizeError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsCircuitOpen(t *testing.T) {
	if !IsCircuitOpen(ErrCircuitOpen) {
		t.Error("Expected IsCircuitOpen to return true for ErrCircuitOpen")
	}

	if IsCircuitOpen(errors.New("other error")) {
		t.Error("Expected IsCircuitOpen to return false for other errors")
	}

	if IsCircuitOpen(nil) {
		t.Error("Expected IsCircuitOpen to return false for nil")
	}
}
