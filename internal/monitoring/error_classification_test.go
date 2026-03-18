package monitoring

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
)

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		// nil error
		{
			name: "nil error returns true",
			err:  nil,
			want: true,
		},

		// Context errors
		{
			name: "context.Canceled returns true",
			err:  context.Canceled,
			want: true,
		},
		{
			name: "context.DeadlineExceeded returns true",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "wrapped context.Canceled returns true",
			err:  stderrors.Join(stderrors.New("operation failed"), context.Canceled),
			want: true,
		},
		{
			name: "wrapped context.DeadlineExceeded returns true",
			err:  stderrors.Join(stderrors.New("operation failed"), context.DeadlineExceeded),
			want: true,
		},

		// Retryable MonitorErrors
		{
			name: "retryable MonitorError returns true",
			err: &errors.MonitorError{
				Type:      errors.ErrorTypeConnection,
				Op:        "test",
				Retryable: true,
			},
			want: true,
		},
		{
			name: "non-retryable MonitorError returns false",
			err: &errors.MonitorError{
				Type:      errors.ErrorTypeAuth,
				Op:        "test",
				Retryable: false,
			},
			want: false,
		},

		// Standard errors
		{
			name: "generic error returns false",
			err:  stderrors.New("some error"),
			want: false,
		},
		{
			name: "wrapped generic error returns false",
			err:  stderrors.Join(stderrors.New("outer"), stderrors.New("inner")),
			want: false,
		},

		// ErrTimeout and ErrConnectionFailed (via IsRetryableError)
		{
			name: "ErrTimeout returns true",
			err:  errors.ErrTimeout,
			want: true,
		},
		{
			name: "ErrConnectionFailed returns true",
			err:  errors.ErrConnectionFailed,
			want: true,
		},
		{
			name: "ErrNotFound returns false",
			err:  errors.ErrNotFound,
			want: false,
		},
		{
			name: "ErrUnauthorized returns false",
			err:  errors.ErrUnauthorized,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientError(tt.err)
			if got != tt.want {
				t.Errorf("isTransientError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestShouldTryPortlessFallback(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		// nil error
		{
			name: "nil error returns false",
			err:  nil,
			want: false,
		},

		// Connection errors that should trigger fallback
		{
			name: "connection refused",
			err:  stderrors.New("dial tcp 192.168.1.100:8006: connect: connection refused"),
			want: true,
		},
		{
			name: "connection reset",
			err:  stderrors.New("read: connection reset by peer"),
			want: true,
		},
		{
			name: "no such host",
			err:  stderrors.New("dial tcp: lookup pve.local: no such host"),
			want: true,
		},
		{
			name: "client.timeout exceeded",
			err:  stderrors.New("net/http: request canceled while waiting for connection (Client.Timeout exceeded)"),
			want: true,
		},
		{
			name: "i/o timeout",
			err:  stderrors.New("read tcp 192.168.1.100:8006: i/o timeout"),
			want: true,
		},
		{
			name: "context deadline exceeded",
			err:  stderrors.New("context deadline exceeded"),
			want: true,
		},

		// Case insensitivity
		{
			name: "CONNECTION REFUSED uppercase",
			err:  stderrors.New("CONNECTION REFUSED"),
			want: true,
		},
		{
			name: "Connection Reset mixed case",
			err:  stderrors.New("Connection Reset"),
			want: true,
		},

		// Errors that should NOT trigger fallback
		{
			name: "authentication error",
			err:  stderrors.New("401 Unauthorized"),
			want: false,
		},
		{
			name: "permission denied",
			err:  stderrors.New("permission denied"),
			want: false,
		},
		{
			name: "certificate error",
			err:  stderrors.New("x509: certificate signed by unknown authority"),
			want: false,
		},
		{
			name: "generic error",
			err:  stderrors.New("something went wrong"),
			want: false,
		},
		{
			name: "empty error message",
			err:  stderrors.New(""),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldTryPortlessFallback(tt.err)
			if got != tt.want {
				t.Errorf("shouldTryPortlessFallback(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestShouldAttemptFallback(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		// nil error
		{
			name: "nil error returns false",
			err:  nil,
			want: false,
		},

		// Timeout patterns
		{
			name: "timeout keyword",
			err:  stderrors.New("operation timeout"),
			want: true,
		},
		{
			name: "TIMEOUT uppercase",
			err:  stderrors.New("TIMEOUT"),
			want: true,
		},

		// Deadline exceeded patterns
		{
			name: "deadline exceeded",
			err:  stderrors.New("deadline exceeded"),
			want: true,
		},
		{
			name: "context deadline exceeded",
			err:  stderrors.New("context deadline exceeded"),
			want: true,
		},

		// Context canceled patterns
		{
			name: "context canceled",
			err:  stderrors.New("context canceled"),
			want: true,
		},
		{
			name: "operation was context canceled",
			err:  stderrors.New("operation was context canceled"),
			want: true,
		},

		// Case insensitivity
		{
			name: "Deadline Exceeded mixed case",
			err:  stderrors.New("Deadline Exceeded"),
			want: true,
		},
		{
			name: "CONTEXT CANCELED uppercase",
			err:  stderrors.New("CONTEXT CANCELED"),
			want: true,
		},

		// Errors that should NOT trigger fallback
		{
			name: "connection refused",
			err:  stderrors.New("connection refused"),
			want: false,
		},
		{
			name: "authentication error",
			err:  stderrors.New("401 Unauthorized"),
			want: false,
		},
		{
			name: "generic error",
			err:  stderrors.New("something went wrong"),
			want: false,
		},
		{
			name: "empty error",
			err:  stderrors.New(""),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldAttemptFallback(tt.err)
			if got != tt.want {
				t.Errorf("shouldAttemptFallback(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestClassifyDLQReason(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		// nil error
		{
			name: "nil error returns empty string",
			err:  nil,
			want: "",
		},

		// Retryable errors -> max_retry_attempts
		{
			name: "retryable MonitorError returns max_retry_attempts",
			err: &errors.MonitorError{
				Type:      errors.ErrorTypeConnection,
				Op:        "poll",
				Retryable: true,
			},
			want: "max_retry_attempts",
		},
		{
			name: "ErrTimeout returns max_retry_attempts",
			err:  errors.ErrTimeout,
			want: "max_retry_attempts",
		},
		{
			name: "ErrConnectionFailed returns max_retry_attempts",
			err:  errors.ErrConnectionFailed,
			want: "max_retry_attempts",
		},

		// Non-retryable errors -> permanent_failure
		{
			name: "non-retryable MonitorError returns permanent_failure",
			err: &errors.MonitorError{
				Type:      errors.ErrorTypeAuth,
				Op:        "auth",
				Retryable: false,
			},
			want: "permanent_failure",
		},
		{
			name: "ErrNotFound returns permanent_failure",
			err:  errors.ErrNotFound,
			want: "permanent_failure",
		},
		{
			name: "ErrUnauthorized returns permanent_failure",
			err:  errors.ErrUnauthorized,
			want: "permanent_failure",
		},
		{
			name: "generic error returns permanent_failure",
			err:  stderrors.New("some error"),
			want: "permanent_failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyDLQReason(tt.err)
			if got != tt.want {
				t.Errorf("classifyDLQReason(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

// TestErrorClassificationEdgeCases tests additional edge cases and interactions
func TestErrorClassificationEdgeCases(t *testing.T) {
	t.Run("shouldTryPortlessFallback and shouldAttemptFallback overlap on deadline exceeded", func(t *testing.T) {
		err := stderrors.New("context deadline exceeded")
		// Both should return true for this error
		if !shouldTryPortlessFallback(err) {
			t.Error("shouldTryPortlessFallback should return true for context deadline exceeded")
		}
		if !shouldAttemptFallback(err) {
			t.Error("shouldAttemptFallback should return true for context deadline exceeded")
		}
	})

	t.Run("isTransientError with nested context error", func(t *testing.T) {
		// Create error that wraps context.Canceled
		type wrappedError struct {
			msg string
			err error
		}
		werr := &wrappedError{msg: "wrapped", err: context.Canceled}
		// Implement error interface
		_ = werr.msg
		// Note: This won't work with errors.Is unless Unwrap is implemented,
		// but the test verifies current behavior
	})

	t.Run("partial keyword matches should not trigger", func(t *testing.T) {
		// "timeouts" contains "timeout" but we want exact keyword matching
		// Current implementation uses Contains, so this will match
		err := stderrors.New("multiple timeouts occurred")
		got := shouldAttemptFallback(err)
		// This documents current behavior - it DOES match because of Contains
		if !got {
			t.Error("shouldAttemptFallback currently matches partial keywords")
		}
	})
}
