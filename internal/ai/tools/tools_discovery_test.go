package tools

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		// Transient errors — should return true
		{"nil error", nil, false},
		{"rate limit 429", errors.New("API returned status 429"), true},
		{"503 service unavailable", errors.New("HTTP 503 Service Unavailable"), true},
		{"rate_limit underscore", errors.New("rate_limit: too many requests"), true},
		{"rate limit space", errors.New("rate limit exceeded"), true},
		{"ratelimit single word", errors.New("ratelimit error from provider"), true},
		{"too many requests", errors.New("too many requests, slow down"), true},
		{"timeout", errors.New("request timeout after 30s"), true},
		{"context deadline", errors.New("context deadline exceeded"), true},
		{"failed after retries", errors.New("failed after 3 retries"), true},
		{"temporarily unavailable", errors.New("service temporarily unavailable"), true},
		{"server overloaded", errors.New("server overloaded, try later"), true},
		{"service unavailable text", errors.New("the service is service unavailable"), true},
		{"connection refused", errors.New("dial tcp: connection refused"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"broken pipe", errors.New("write: broken pipe"), true},
		{"i/o timeout", errors.New("i/o timeout"), true},
		{"network unreachable", errors.New("network unreachable"), true},

		// Anthropic-style rate limit
		{"anthropic rate limit", errors.New("Error: 429 {\"type\":\"error\",\"error\":{\"type\":\"rate_limit_error\"}}"), true},
		// OpenAI-style
		{"openai rate limit", errors.New("Rate limit reached for gpt-4"), true},
		// Gemini-style
		{"gemini quota", errors.New("429 Too Many Requests"), true},

		// Non-transient errors — should return false
		{"resource not found", errors.New("resource not found"), false},
		{"permission denied", errors.New("permission denied"), false},
		{"invalid argument", errors.New("invalid resource_type: foo"), false},
		{"generic error", errors.New("something went wrong"), false},
		{"empty error", errors.New(""), false},
		{"auth error", errors.New("authentication failed"), false},
		{"not found", errors.New("404 not found"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTransientError(tt.err)
			assert.Equal(t, tt.expected, result, "error: %v", tt.err)
		})
	}
}
