package tempproxy

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestErrorType_Constants(t *testing.T) {
	// Verify enum values are distinct
	values := map[ErrorType]bool{}
	errorTypes := []ErrorType{
		ErrorTypeUnknown,
		ErrorTypeTransport,
		ErrorTypeAuth,
		ErrorTypeSSH,
		ErrorTypeSensor,
		ErrorTypeTimeout,
		ErrorTypeNode,
	}

	for _, et := range errorTypes {
		if values[et] {
			t.Errorf("Duplicate ErrorType value: %d", et)
		}
		values[et] = true
	}

	// ErrorTypeUnknown should be 0 (default)
	if ErrorTypeUnknown != 0 {
		t.Errorf("ErrorTypeUnknown = %d, want 0", ErrorTypeUnknown)
	}
}

func TestProxyError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      ProxyError
		contains string
	}{
		{
			name: "with wrapped error",
			err: ProxyError{
				Message: "connection failed",
				Wrapped: errors.New("dial tcp: connection refused"),
			},
			contains: "connection failed: dial tcp: connection refused",
		},
		{
			name: "without wrapped error",
			err: ProxyError{
				Message: "operation timed out",
			},
			contains: "operation timed out",
		},
		{
			name: "with type and retryable",
			err: ProxyError{
				Type:      ErrorTypeSSH,
				Message:   "SSH connectivity issue",
				Retryable: true,
				Wrapped:   errors.New("ssh: handshake failed"),
			},
			contains: "SSH connectivity issue: ssh: handshake failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.err.Error()
			if result != tc.contains {
				t.Errorf("Error() = %q, want %q", result, tc.contains)
			}
		})
	}
}

func TestProxyError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	err := &ProxyError{
		Message: "outer error",
		Wrapped: inner,
	}

	unwrapped := err.Unwrap()
	if unwrapped != inner {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, inner)
	}
}

func TestProxyError_Unwrap_Nil(t *testing.T) {
	err := &ProxyError{
		Message: "error without wrapped",
	}

	unwrapped := err.Unwrap()
	if unwrapped != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrapped)
	}
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name        string
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{
			name:        "attempt 0",
			attempt:     0,
			minExpected: 90 * time.Millisecond,  // initialBackoff - 10% jitter
			maxExpected: 110 * time.Millisecond, // initialBackoff + 10% jitter
		},
		{
			name:        "attempt 1",
			attempt:     1,
			minExpected: 180 * time.Millisecond, // 200ms * 0.9
			maxExpected: 220 * time.Millisecond, // 200ms * 1.1
		},
		{
			name:        "attempt 2",
			attempt:     2,
			minExpected: 360 * time.Millisecond, // 400ms * 0.9
			maxExpected: 440 * time.Millisecond, // 400ms * 1.1
		},
		{
			name:        "negative attempt (treated as 0)",
			attempt:     -1,
			minExpected: 90 * time.Millisecond,
			maxExpected: 110 * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Run multiple times to account for jitter
			for i := 0; i < 10; i++ {
				result := calculateBackoff(tc.attempt)
				if result < tc.minExpected || result > tc.maxExpected {
					t.Errorf("calculateBackoff(%d) = %v, want between %v and %v",
						tc.attempt, result, tc.minExpected, tc.maxExpected)
				}
			}
		})
	}
}

func TestCalculateBackoff_CappedAtMax(t *testing.T) {
	// High attempt number should be capped at maxBackoff (10s) + jitter
	result := calculateBackoff(100)

	maxWithJitter := maxBackoff + time.Duration(float64(maxBackoff)*jitterFraction)
	if result > maxWithJitter {
		t.Errorf("calculateBackoff(100) = %v, should be capped at ~%v", result, maxBackoff)
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substrs  []string
		expected bool
	}{
		{
			name:     "single match",
			s:        "connection refused",
			substrs:  []string{"refused"},
			expected: true,
		},
		{
			name:     "multiple substrs first match",
			s:        "ssh connection failed",
			substrs:  []string{"ssh", "timeout"},
			expected: true,
		},
		{
			name:     "multiple substrs second match",
			s:        "operation timeout",
			substrs:  []string{"ssh", "timeout"},
			expected: true,
		},
		{
			name:     "no match",
			s:        "success",
			substrs:  []string{"error", "failed"},
			expected: false,
		},
		{
			name:     "case insensitive upper",
			s:        "CONNECTION REFUSED",
			substrs:  []string{"connection"},
			expected: true,
		},
		{
			name:     "case insensitive mixed",
			s:        "SSH Error",
			substrs:  []string{"ssh error"},
			expected: true,
		},
		{
			name:     "case insensitive upper substr",
			s:        "connection refused",
			substrs:  []string{"REFUSED"},
			expected: true,
		},
		{
			name:     "empty string",
			s:        "",
			substrs:  []string{"test"},
			expected: false,
		},
		{
			name:     "empty substrs",
			s:        "test",
			substrs:  []string{},
			expected: false,
		},
		{
			name:     "exact match",
			s:        "error",
			substrs:  []string{"error"},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := contains(tc.s, tc.substrs...)
			if result != tc.expected {
				t.Errorf("contains(%q, %v) = %v, want %v", tc.s, tc.substrs, result, tc.expected)
			}
		})
	}
}

func TestClassifyError_NodeErrors(t *testing.T) {
	tests := []struct {
		name       string
		respError  string
		expectType ErrorType
		retryable  bool
	}{
		{
			name:       "rejected by validator",
			respError:  "rejected by validator: invalid node",
			expectType: ErrorTypeNode,
			retryable:  false,
		},
		{
			name:       "not in allowlist",
			respError:  "node not in allowlist",
			expectType: ErrorTypeNode,
			retryable:  false,
		},
		{
			name:       "node quote pattern",
			respError:  "node \"server1\" not found",
			expectType: ErrorTypeNode,
			retryable:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := classifyError(nil, tc.respError)
			if result == nil {
				t.Fatal("classifyError returned nil")
			}
			if result.Type != tc.expectType {
				t.Errorf("Type = %v, want %v", result.Type, tc.expectType)
			}
			if result.Retryable != tc.retryable {
				t.Errorf("Retryable = %v, want %v", result.Retryable, tc.retryable)
			}
		})
	}
}

func TestClassifyError_AuthErrors(t *testing.T) {
	tests := []struct {
		respError string
	}{
		{"unauthorized"},
		{"method requires host-level privileges"},
		{"method requires admin capability"},
		{"missing admin capability for operation"},
	}

	for _, tc := range tests {
		t.Run(tc.respError, func(t *testing.T) {
			result := classifyError(nil, tc.respError)
			if result == nil {
				t.Fatal("classifyError returned nil")
			}
			if result.Type != ErrorTypeAuth {
				t.Errorf("Type = %v, want ErrorTypeAuth", result.Type)
			}
			if result.Retryable {
				t.Error("Auth errors should not be retryable")
			}
		})
	}
}

func TestClassifyError_SSHErrors(t *testing.T) {
	tests := []struct {
		respError string
	}{
		{"ssh: handshake failed"},
		{"connection reset by peer"},
		{"operation timeout exceeded"},
	}

	for _, tc := range tests {
		t.Run(tc.respError, func(t *testing.T) {
			result := classifyError(nil, tc.respError)
			if result == nil {
				t.Fatal("classifyError returned nil")
			}
			if result.Type != ErrorTypeSSH {
				t.Errorf("Type = %v, want ErrorTypeSSH", result.Type)
			}
			if !result.Retryable {
				t.Error("SSH errors should be retryable")
			}
		})
	}
}

func TestClassifyError_SensorErrors(t *testing.T) {
	tests := []struct {
		respError string
	}{
		{"sensor command failed"},
		{"temperature sensor unavailable"},
	}

	for _, tc := range tests {
		t.Run(tc.respError, func(t *testing.T) {
			result := classifyError(nil, tc.respError)
			if result == nil {
				t.Fatal("classifyError returned nil")
			}
			if result.Type != ErrorTypeSensor {
				t.Errorf("Type = %v, want ErrorTypeSensor", result.Type)
			}
			if result.Retryable {
				t.Error("Sensor errors should not be retryable")
			}
		})
	}
}

func TestClassifyError_RateLimitErrors(t *testing.T) {
	result := classifyError(nil, "rate limit exceeded")
	if result == nil {
		t.Fatal("classifyError returned nil")
	}
	if result.Type != ErrorTypeTransport {
		t.Errorf("Type = %v, want ErrorTypeTransport", result.Type)
	}
	if result.Retryable {
		t.Error("Rate limit errors should not be retryable")
	}
}

func TestClassifyError_TimeoutNetError(t *testing.T) {
	// Create a mock timeout error
	timeoutErr := &mockNetError{timeout: true}

	result := classifyError(timeoutErr, "")
	if result == nil {
		t.Fatal("classifyError returned nil")
	}
	if result.Type != ErrorTypeTimeout {
		t.Errorf("Type = %v, want ErrorTypeTimeout", result.Type)
	}
	if !result.Retryable {
		t.Error("Timeout errors should be retryable")
	}
}

func TestClassifyError_OpError(t *testing.T) {
	opErr := &net.OpError{
		Op:  "dial",
		Net: "unix",
		Err: errors.New("connection refused"),
	}

	result := classifyError(opErr, "")
	if result == nil {
		t.Fatal("classifyError returned nil")
	}
	if result.Type != ErrorTypeTransport {
		t.Errorf("Type = %v, want ErrorTypeTransport", result.Type)
	}
	if !result.Retryable {
		t.Error("Transport errors should be retryable")
	}
}

func TestClassifyError_NilInputs(t *testing.T) {
	result := classifyError(nil, "")
	if result != nil {
		t.Errorf("classifyError(nil, \"\") = %v, want nil", result)
	}
}

func TestClassifyError_UnknownError(t *testing.T) {
	unknownErr := errors.New("some unknown error")

	result := classifyError(unknownErr, "")
	if result == nil {
		t.Fatal("classifyError returned nil")
	}
	if result.Type != ErrorTypeUnknown {
		t.Errorf("Type = %v, want ErrorTypeUnknown", result.Type)
	}
	if result.Retryable {
		t.Error("Unknown errors should not be retryable")
	}
}

func TestRPCRequest_Fields(t *testing.T) {
	req := RPCRequest{
		Method: "get_temperature",
		Params: map[string]interface{}{
			"node": "server1",
		},
	}

	if req.Method != "get_temperature" {
		t.Errorf("Method = %q, want get_temperature", req.Method)
	}
	if req.Params["node"] != "server1" {
		t.Errorf("Params[node] = %v, want server1", req.Params["node"])
	}
}

func TestRPCResponse_Fields(t *testing.T) {
	resp := RPCResponse{
		Success: true,
		Data: map[string]interface{}{
			"temperature": "45.0",
		},
		Error: "",
	}

	if !resp.Success {
		t.Error("Success should be true")
	}
	if resp.Data["temperature"] != "45.0" {
		t.Errorf("Data[temperature] = %v, want 45.0", resp.Data["temperature"])
	}
	if resp.Error != "" {
		t.Errorf("Error = %q, want empty", resp.Error)
	}
}

func TestRPCResponse_ErrorCase(t *testing.T) {
	resp := RPCResponse{
		Success: false,
		Error:   "unauthorized",
	}

	if resp.Success {
		t.Error("Success should be false")
	}
	if resp.Error != "unauthorized" {
		t.Errorf("Error = %q, want unauthorized", resp.Error)
	}
}

func TestClient_Fields(t *testing.T) {
	client := &Client{
		socketPath: "/test/socket.sock",
		timeout:    60 * time.Second,
	}

	if client.socketPath != "/test/socket.sock" {
		t.Errorf("socketPath = %q, want /test/socket.sock", client.socketPath)
	}
	if client.timeout != 60*time.Second {
		t.Errorf("timeout = %v, want 60s", client.timeout)
	}
}

func TestProxyError_AllFields(t *testing.T) {
	wrapped := errors.New("wrapped error")
	err := ProxyError{
		Type:      ErrorTypeSSH,
		Message:   "SSH connectivity issue",
		Retryable: true,
		Wrapped:   wrapped,
	}

	if err.Type != ErrorTypeSSH {
		t.Errorf("Type = %v, want ErrorTypeSSH", err.Type)
	}
	if err.Message != "SSH connectivity issue" {
		t.Errorf("Message = %q, want 'SSH connectivity issue'", err.Message)
	}
	if !err.Retryable {
		t.Error("Retryable should be true")
	}
	if err.Wrapped != wrapped {
		t.Errorf("Wrapped = %v, want %v", err.Wrapped, wrapped)
	}
}

// mockNetError implements net.Error for testing
type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return "mock network error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }
