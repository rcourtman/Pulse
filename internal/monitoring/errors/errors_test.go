package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestMonitorError_Error(t *testing.T) {
	baseErr := fmt.Errorf("base error")

	tests := []struct {
		name     string
		err      *MonitorError
		expected string
	}{
		{
			name: "with node",
			err: &MonitorError{
				Op:       "poll_nodes",
				Instance: "pve1",
				Node:     "node1",
				Err:      baseErr,
			},
			expected: "poll_nodes failed on pve1/node1: base error",
		},
		{
			name: "with instance only",
			err: &MonitorError{
				Op:       "get_vms",
				Instance: "pve1",
				Err:      baseErr,
			},
			expected: "get_vms failed on pve1: base error",
		},
		{
			name: "operation only",
			err: &MonitorError{
				Op:  "sync",
				Err: baseErr,
			},
			expected: "sync failed: base error",
		},
		{
			name: "sanitizes control characters in context and message",
			err: &MonitorError{
				Op:       "poll\nnodes",
				Instance: "pve1\r\nprod",
				Node:     "node\t1",
				Err:      fmt.Errorf("request failed\r\nheader injection"),
			},
			expected: "poll nodes failed on pve1 prod/node 1: request failed header injection",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.err.Error()
			if result != tc.expected {
				t.Errorf("Error() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestMonitorError_Unwrap(t *testing.T) {
	baseErr := fmt.Errorf("wrapped error")
	monErr := &MonitorError{
		Op:       "test",
		Instance: "instance",
		Err:      baseErr,
	}

	unwrapped := monErr.Unwrap()
	if unwrapped != baseErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, baseErr)
	}
}

func TestMonitorError_NilReceiverSafety(t *testing.T) {
	var monErr *MonitorError

	if got := monErr.Error(); got != "<nil>" {
		t.Errorf("Error() = %q, want %q", got, "<nil>")
	}
	if got := monErr.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
	if monErr.Is(ErrTimeout) {
		t.Error("Is() should return false for nil receiver")
	}
	if monErr.WithNode("node1") != nil {
		t.Error("WithNode() should return nil for nil receiver")
	}
	if monErr.WithStatusCode(500) != nil {
		t.Error("WithStatusCode() should return nil for nil receiver")
	}
}

func TestMonitorError_Is(t *testing.T) {
	tests := []struct {
		name     string
		err      *MonitorError
		target   error
		expected bool
	}{
		{
			name:     "not found type matches ErrNotFound",
			err:      &MonitorError{Type: ErrorTypeNotFound},
			target:   ErrNotFound,
			expected: true,
		},
		{
			name:     "auth type matches ErrUnauthorized",
			err:      &MonitorError{Type: ErrorTypeAuth},
			target:   ErrUnauthorized,
			expected: true,
		},
		{
			name:     "auth type matches ErrForbidden",
			err:      &MonitorError{Type: ErrorTypeAuth},
			target:   ErrForbidden,
			expected: true,
		},
		{
			name:     "timeout type matches ErrTimeout",
			err:      &MonitorError{Type: ErrorTypeTimeout},
			target:   ErrTimeout,
			expected: true,
		},
		{
			name:     "connection type matches ErrConnectionFailed",
			err:      &MonitorError{Type: ErrorTypeConnection},
			target:   ErrConnectionFailed,
			expected: true,
		},
		{
			name:     "wrapped error matches",
			err:      &MonitorError{Type: ErrorTypeInternal, Err: ErrInvalidInput},
			target:   ErrInvalidInput,
			expected: true,
		},
		{
			name:     "type mismatch",
			err:      &MonitorError{Type: ErrorTypeInternal},
			target:   ErrNotFound,
			expected: false,
		},
		{
			name:     "nil target",
			err:      &MonitorError{Type: ErrorTypeNotFound},
			target:   nil,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.err.Is(tc.target)
			if result != tc.expected {
				t.Errorf("Is(%v) = %v, want %v", tc.target, result, tc.expected)
			}
		})
	}
}

func TestNewMonitorError(t *testing.T) {
	baseErr := fmt.Errorf("test error")

	err := NewMonitorError(ErrorTypeConnection, "poll", "instance1", baseErr)

	if err.Type != ErrorTypeConnection {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeConnection)
	}
	if err.Op != "poll" {
		t.Errorf("Op = %v, want %v", err.Op, "poll")
	}
	if err.Instance != "instance1" {
		t.Errorf("Instance = %v, want %v", err.Instance, "instance1")
	}
	if err.Err != baseErr {
		t.Errorf("Err = %v, want %v", err.Err, baseErr)
	}
	if err.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestNewMonitorError_SanitizesContext(t *testing.T) {
	baseErr := fmt.Errorf("test error")
	err := NewMonitorError(ErrorTypeConnection, "poll\nnodes", "inst\tone", baseErr)

	if err.Op != "poll nodes" {
		t.Errorf("Op = %q, want %q", err.Op, "poll nodes")
	}
	if err.Instance != "inst one" {
		t.Errorf("Instance = %q, want %q", err.Instance, "inst one")
	}
}

func TestMonitorError_WithNode(t *testing.T) {
	err := NewMonitorError(ErrorTypeConnection, "poll", "instance1", nil)
	result := err.WithNode("node1")

	if result.Node != "node1" {
		t.Errorf("Node = %v, want %v", result.Node, "node1")
	}
	// Should return same instance for chaining
	if result != err {
		t.Error("WithNode() should return same instance")
	}
}

func TestMonitorError_WithNode_SanitizesInput(t *testing.T) {
	err := NewMonitorError(ErrorTypeConnection, "poll", "instance1", nil)
	err.WithNode("node\t1\r\nprod")

	if err.Node != "node 1 prod" {
		t.Errorf("Node = %q, want %q", err.Node, "node 1 prod")
	}
}

func TestMonitorError_WithStatusCode(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		wantRetryable bool
	}{
		{
			name:          "500 error is retryable",
			statusCode:    500,
			wantRetryable: true,
		},
		{
			name:          "502 error is retryable",
			statusCode:    502,
			wantRetryable: true,
		},
		{
			name:          "429 rate limit is retryable",
			statusCode:    429,
			wantRetryable: true,
		},
		{
			name:          "408 timeout is retryable",
			statusCode:    408,
			wantRetryable: true,
		},
		{
			name:          "400 bad request not retryable",
			statusCode:    400,
			wantRetryable: false,
		},
		{
			name:          "401 unauthorized not retryable",
			statusCode:    401,
			wantRetryable: false,
		},
		{
			name:          "403 forbidden not retryable",
			statusCode:    403,
			wantRetryable: false,
		},
		{
			name:          "404 not found not retryable",
			statusCode:    404,
			wantRetryable: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := NewMonitorError(ErrorTypeAPI, "request", "instance", nil)
			result := err.WithStatusCode(tc.statusCode)

			if result.StatusCode != tc.statusCode {
				t.Errorf("StatusCode = %v, want %v", result.StatusCode, tc.statusCode)
			}
			if result.Retryable != tc.wantRetryable {
				t.Errorf("Retryable = %v, want %v", result.Retryable, tc.wantRetryable)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name          string
		errorType     ErrorType
		err           error
		wantRetryable bool
	}{
		{
			name:          "connection error is retryable",
			errorType:     ErrorTypeConnection,
			err:           nil,
			wantRetryable: true,
		},
		{
			name:          "timeout error is retryable",
			errorType:     ErrorTypeTimeout,
			err:           nil,
			wantRetryable: true,
		},
		{
			name:          "auth error not retryable",
			errorType:     ErrorTypeAuth,
			err:           nil,
			wantRetryable: false,
		},
		{
			name:          "validation error not retryable",
			errorType:     ErrorTypeValidation,
			err:           nil,
			wantRetryable: false,
		},
		{
			name:          "not found error not retryable",
			errorType:     ErrorTypeNotFound,
			err:           nil,
			wantRetryable: false,
		},
		{
			name:          "internal error retryable by default",
			errorType:     ErrorTypeInternal,
			err:           nil,
			wantRetryable: true,
		},
		{
			name:          "internal with invalid input not retryable",
			errorType:     ErrorTypeInternal,
			err:           ErrInvalidInput,
			wantRetryable: false,
		},
		{
			name:          "internal with forbidden not retryable",
			errorType:     ErrorTypeInternal,
			err:           ErrForbidden,
			wantRetryable: false,
		},
		{
			name:          "API error retryable by default",
			errorType:     ErrorTypeAPI,
			err:           nil,
			wantRetryable: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isRetryable(tc.errorType, tc.err)
			if result != tc.wantRetryable {
				t.Errorf("isRetryable(%v, %v) = %v, want %v", tc.errorType, tc.err, result, tc.wantRetryable)
			}
		})
	}
}

func TestWrapConnectionError(t *testing.T) {
	baseErr := fmt.Errorf("connection refused")
	err := WrapConnectionError("connect", "pve1", baseErr)

	var monErr *MonitorError
	if !errors.As(err, &monErr) {
		t.Fatal("WrapConnectionError() did not return MonitorError")
	}

	if monErr.Type != ErrorTypeConnection {
		t.Errorf("Type = %v, want %v", monErr.Type, ErrorTypeConnection)
	}
	if monErr.Op != "connect" {
		t.Errorf("Op = %v, want %v", monErr.Op, "connect")
	}
	if monErr.Instance != "pve1" {
		t.Errorf("Instance = %v, want %v", monErr.Instance, "pve1")
	}
}

func TestWrapAPIError(t *testing.T) {
	baseErr := fmt.Errorf("server error")
	err := WrapAPIError("request", "pve1", baseErr, 500)

	var monErr *MonitorError
	if !errors.As(err, &monErr) {
		t.Fatal("WrapAPIError() did not return MonitorError")
	}

	if monErr.Type != ErrorTypeAPI {
		t.Errorf("Type = %v, want %v", monErr.Type, ErrorTypeAPI)
	}
	if monErr.StatusCode != 500 {
		t.Errorf("StatusCode = %v, want %v", monErr.StatusCode, 500)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "MonitorError retryable",
			err:      NewMonitorError(ErrorTypeConnection, "test", "instance", nil),
			expected: true,
		},
		{
			name:     "MonitorError not retryable",
			err:      NewMonitorError(ErrorTypeAuth, "test", "instance", nil),
			expected: false,
		},
		{
			name:     "ErrTimeout is retryable",
			err:      ErrTimeout,
			expected: true,
		},
		{
			name:     "ErrConnectionFailed is retryable",
			err:      ErrConnectionFailed,
			expected: true,
		},
		{
			name:     "wrapped ErrTimeout is retryable",
			err:      fmt.Errorf("wrapped: %w", ErrTimeout),
			expected: true,
		},
		{
			name: "typed nil MonitorError is not retryable",
			err: func() error {
				var monErr *MonitorError
				return monErr
			}(),
			expected: false,
		},
		{
			name:     "regular error not retryable",
			err:      fmt.Errorf("regular error"),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsRetryableError(tc.err)
			if result != tc.expected {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tc.err, result, tc.expected)
			}
		})
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "MonitorError auth type",
			err:      NewMonitorError(ErrorTypeAuth, "login", "instance", nil),
			expected: true,
		},
		{
			name:     "MonitorError with 401 status",
			err:      NewMonitorError(ErrorTypeAPI, "request", "instance", nil).WithStatusCode(401),
			expected: true,
		},
		{
			name:     "MonitorError with 403 status",
			err:      NewMonitorError(ErrorTypeAPI, "request", "instance", nil).WithStatusCode(403),
			expected: true,
		},
		{
			name:     "ErrUnauthorized",
			err:      ErrUnauthorized,
			expected: true,
		},
		{
			name:     "ErrForbidden",
			err:      ErrForbidden,
			expected: true,
		},
		{
			name:     "wrapped ErrUnauthorized",
			err:      fmt.Errorf("wrapped: %w", ErrUnauthorized),
			expected: true,
		},
		{
			name:     "error with 'unauthorized' in message",
			err:      fmt.Errorf("request unauthorized"),
			expected: true,
		},
		{
			name:     "error with 'forbidden' in message",
			err:      fmt.Errorf("access forbidden"),
			expected: true,
		},
		{
			name:     "error with 'authentication failed' in message",
			err:      fmt.Errorf("authentication failed"),
			expected: true,
		},
		{
			name:     "error with 'authentication error' in message",
			err:      fmt.Errorf("authentication error occurred"),
			expected: true,
		},
		{
			name:     "error with '401' in message",
			err:      fmt.Errorf("HTTP 401 response"),
			expected: true,
		},
		{
			name:     "error with '403' in message",
			err:      fmt.Errorf("HTTP 403 response"),
			expected: true,
		},
		{
			name: "typed nil MonitorError",
			err: func() error {
				var monErr *MonitorError
				return monErr
			}(),
			expected: false,
		},
		{
			name:     "regular error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
		{
			name:     "status substring without boundary is not auth",
			err:      fmt.Errorf("HTTP 1403 response"),
			expected: false,
		},
		{
			name:     "uppercase unauthorized is detected",
			err:      fmt.Errorf("UNAUTHORIZED request"),
			expected: true,
		},
		{
			name:     "status code adjacent to token text is not auth",
			err:      fmt.Errorf("status403"),
			expected: false,
		},
		{
			name:     "MonitorError connection type not auth",
			err:      NewMonitorError(ErrorTypeConnection, "connect", "instance", nil),
			expected: false,
		},
		{
			name:     "oversized error with auth marker beyond cap is not matched",
			err:      fmt.Errorf("%sunauthorized", strings.Repeat("a", maxAuthMatchLength+16)),
			expected: false,
		},
		{
			name:     "oversized error with auth marker in prefix is matched",
			err:      fmt.Errorf("forbidden%s", strings.Repeat("b", maxAuthMatchLength+16)),
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsAuthError(tc.err)
			if result != tc.expected {
				t.Errorf("IsAuthError(%v) = %v, want %v", tc.err, result, tc.expected)
			}
		})
	}
}

func TestErrorTypes(t *testing.T) {
	// Verify error types are defined correctly
	types := []ErrorType{
		ErrorTypeConnection,
		ErrorTypeAuth,
		ErrorTypeValidation,
		ErrorTypeNotFound,
		ErrorTypeInternal,
		ErrorTypeAPI,
		ErrorTypeTimeout,
	}

	for _, errorType := range types {
		if errorType == "" {
			t.Error("ErrorType should not be empty")
		}
	}
}

func TestBaseErrors(t *testing.T) {
	// Verify base errors are defined
	baseErrors := []error{
		ErrNotFound,
		ErrUnauthorized,
		ErrForbidden,
		ErrTimeout,
		ErrInvalidInput,
		ErrConnectionFailed,
	}

	for _, err := range baseErrors {
		if err == nil {
			t.Error("Base error should not be nil")
		}
		if err.Error() == "" {
			t.Error("Base error should have non-empty message")
		}
	}
}

func TestMonitorError_ErrorsIs(t *testing.T) {
	// Test using errors.Is with MonitorError
	monErr := NewMonitorError(ErrorTypeNotFound, "get", "instance", nil)

	if !errors.Is(monErr, ErrNotFound) {
		t.Error("errors.Is() should return true for matching error type")
	}
}

func TestMonitorError_ErrorsAs(t *testing.T) {
	// Test using errors.As with MonitorError
	originalErr := NewMonitorError(ErrorTypeConnection, "connect", "pve1", fmt.Errorf("refused"))
	wrappedErr := fmt.Errorf("outer: %w", originalErr)

	var monErr *MonitorError
	if !errors.As(wrappedErr, &monErr) {
		t.Error("errors.As() should extract MonitorError from wrapped error")
	}

	if monErr.Op != "connect" {
		t.Errorf("Op = %v, want %v", monErr.Op, "connect")
	}
}
