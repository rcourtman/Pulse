package errors

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Base error types
var (
	ErrNotFound         = errors.New("not found")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrTimeout          = errors.New("timeout")
	ErrInvalidInput     = errors.New("invalid input")
	ErrConnectionFailed = errors.New("connection failed")
	ErrInternalError    = errors.New("internal error")
)

// ErrorType represents the category of error
type ErrorType string

const (
	ErrorTypeConnection ErrorType = "connection"
	ErrorTypeAuth       ErrorType = "auth"
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeNotFound   ErrorType = "not_found"
	ErrorTypeInternal   ErrorType = "internal"
	ErrorTypeAPI        ErrorType = "api"
	ErrorTypeTimeout    ErrorType = "timeout"
)

// MonitorError is a structured error for monitoring operations
type MonitorError struct {
	Type       ErrorType
	Op         string // Operation that failed (e.g., "poll_nodes", "get_vms")
	Instance   string // Instance name where error occurred
	Node       string // Node name if applicable
	Err        error  // Underlying error
	StatusCode int    // HTTP status code if applicable
	Timestamp  time.Time
	Retryable  bool
}

func (e *MonitorError) Error() string {
	if e.Node != "" {
		return fmt.Sprintf("%s failed on %s/%s: %v", e.Op, e.Instance, e.Node, e.Err)
	}
	if e.Instance != "" {
		return fmt.Sprintf("%s failed on %s: %v", e.Op, e.Instance, e.Err)
	}
	return fmt.Sprintf("%s failed: %v", e.Op, e.Err)
}

func (e *MonitorError) Unwrap() error {
	return e.Err
}

// Is implements errors.Is interface
func (e *MonitorError) Is(target error) bool {
	if target == nil {
		return false
	}

	// Check base error types
	switch target {
	case ErrNotFound:
		return e.Type == ErrorTypeNotFound
	case ErrUnauthorized, ErrForbidden:
		return e.Type == ErrorTypeAuth
	case ErrTimeout:
		return e.Type == ErrorTypeTimeout
	case ErrConnectionFailed:
		return e.Type == ErrorTypeConnection
	}

	// Check wrapped error
	return errors.Is(e.Err, target)
}

// NewMonitorError creates a new MonitorError
func NewMonitorError(errorType ErrorType, op, instance string, err error) *MonitorError {
	return &MonitorError{
		Type:      errorType,
		Op:        op,
		Instance:  instance,
		Err:       err,
		Timestamp: time.Now(),
		Retryable: isRetryable(errorType, err),
	}
}

// WithNode adds node information to the error
func (e *MonitorError) WithNode(node string) *MonitorError {
	e.Node = node
	return e
}

// WithStatusCode adds HTTP status code to the error
func (e *MonitorError) WithStatusCode(code int) *MonitorError {
	e.StatusCode = code
	// Update retryable based on status code
	if code >= 500 || code == 429 || code == 408 {
		e.Retryable = true
	} else if code >= 400 && code < 500 {
		e.Retryable = false
	}
	return e
}

// isRetryable determines if an error should be retried
func isRetryable(errorType ErrorType, err error) bool {
	switch errorType {
	case ErrorTypeConnection, ErrorTypeTimeout:
		return true
	case ErrorTypeAuth, ErrorTypeValidation, ErrorTypeNotFound:
		return false
	default: // ErrorTypeInternal, ErrorTypeAPI
		// Check the underlying error
		if err != nil {
			return !errors.Is(err, ErrInvalidInput) && !errors.Is(err, ErrForbidden)
		}
		return true
	}
}

// Helper functions

// WrapConnectionError wraps a connection error with context
func WrapConnectionError(op, instance string, err error) error {
	return NewMonitorError(ErrorTypeConnection, op, instance, err)
}

// WrapAuthError wraps an authentication error with context
func WrapAuthError(op, instance string, err error) error {
	return NewMonitorError(ErrorTypeAuth, op, instance, err)
}

// WrapAPIError wraps an API error with context
func WrapAPIError(op, instance string, err error, statusCode int) error {
	return NewMonitorError(ErrorTypeAPI, op, instance, err).WithStatusCode(statusCode)
}

// IsRetryableError checks if an error should be retried
func IsRetryableError(err error) bool {
	var monErr *MonitorError
	if errors.As(err, &monErr) {
		return monErr.Retryable
	}

	// Check for wrapped standard errors
	return errors.Is(err, ErrTimeout) || errors.Is(err, ErrConnectionFailed)
}

// IsAuthError checks if an error is an authentication error
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}

	var monErr *MonitorError
	if errors.As(err, &monErr) {
		// Check type
		if monErr.Type == ErrorTypeAuth {
			return true
		}
		// Check status code for 401/403
		if monErr.StatusCode == 401 || monErr.StatusCode == 403 {
			return true
		}
	}

	// Check for wrapped standard errors
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden) {
		return true
	}

	// Check error message for authentication indicators
	errMsg := err.Error()
	return strings.Contains(errMsg, "authentication error") ||
		strings.Contains(errMsg, "authentication failed") ||
		strings.Contains(errMsg, "401") ||
		strings.Contains(errMsg, "403") ||
		strings.Contains(errMsg, "unauthorized") ||
		strings.Contains(errMsg, "forbidden")
}
