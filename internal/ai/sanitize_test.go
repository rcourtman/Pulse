package ai

import (
	"errors"
	"testing"
)

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected string
	}{
		{
			name:     "nil error",
			input:    nil,
			expected: "",
		},
		{
			name:     "i/o timeout with TCP details",
			input:    errors.New("failed to send command: write tcp 192.168.0.123:7655->192.168.0.134:58004: i/o timeout"),
			expected: "connection to agent timed out - the agent may be disconnected or unreachable",
		},
		{
			name:     "read tcp i/o timeout",
			input:    errors.New("read tcp 192.168.1.100:8006: i/o timeout"),
			expected: "network timeout - the target may be unreachable",
		},
		{
			name:     "connection refused with TCP details",
			input:    errors.New("dial tcp 192.168.1.50:9090: connection refused"),
			expected: "connection refused - the agent may not be running on the target host",
		},
		{
			name:     "no such host",
			input:    errors.New("dial tcp: lookup unknown-host: no such host"),
			expected: "host not found - verify the hostname is correct and DNS is working",
		},
		{
			name:     "context deadline exceeded",
			input:    errors.New("context deadline exceeded"),
			expected: "operation timed out - the command may have taken too long",
		},
		{
			name:     "regular error passes through",
			input:    errors.New("invalid command"),
			expected: "invalid command",
		},
		{
			name:     "agent not connected error passes through",
			input:    errors.New("agent delly not connected"),
			expected: "agent delly not connected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeError(tt.input)
			if tt.input == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if result.Error() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.Error())
			}
		})
	}
}
