package notifications

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsRetryableWebhookError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		// Network errors - should be retryable
		{
			name:     "timeout error",
			err:      errors.New("connection timeout"),
			expected: true,
		},
		{
			name:     "timeout uppercase",
			err:      errors.New("TIMEOUT"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("dial tcp: connection refused"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      errors.New("read: connection reset by peer"),
			expected: true,
		},
		{
			name:     "no such host",
			err:      errors.New("lookup webhook.example.com: no such host"),
			expected: true,
		},
		{
			name:     "network unreachable",
			err:      errors.New("dial tcp: network unreachable"),
			expected: true,
		},
		// HTTP 5xx errors - should be retryable
		{
			name:     "status 500 internal server error",
			err:      errors.New("webhook returned status 500: Internal Server Error"),
			expected: true,
		},
		{
			name:     "status 502 bad gateway",
			err:      errors.New("webhook returned status 502: Bad Gateway"),
			expected: true,
		},
		{
			name:     "status 503 service unavailable",
			err:      errors.New("webhook returned status 503: Service Unavailable"),
			expected: true,
		},
		{
			name:     "status 504 gateway timeout",
			err:      errors.New("webhook returned status 504: Gateway Timeout"),
			expected: true,
		},
		{
			name:     "status 520 cloudflare error",
			err:      errors.New("webhook returned status 520"),
			expected: true,
		},
		{
			name:     "status 522 connection timed out",
			err:      errors.New("webhook returned status 522: Connection timed out"),
			expected: true,
		},
		{
			name:     "status 599 network connect timeout",
			err:      errors.New("webhook returned status 599"),
			expected: true,
		},
		// HTTP 429 rate limiting - should be retryable
		{
			name:     "status 429 too many requests",
			err:      errors.New("webhook returned status 429: Too Many Requests"),
			expected: true,
		},
		// HTTP 4xx client errors - should NOT be retryable
		{
			name:     "status 400 bad request",
			err:      errors.New("webhook returned status 400: Bad Request"),
			expected: false,
		},
		{
			name:     "status 401 unauthorized",
			err:      errors.New("webhook returned status 401: Unauthorized"),
			expected: false,
		},
		{
			name:     "status 403 forbidden",
			err:      errors.New("webhook returned status 403: Forbidden"),
			expected: false,
		},
		{
			name:     "status 404 not found",
			err:      errors.New("webhook returned status 404: Not Found"),
			expected: false,
		},
		{
			name:     "status 405 method not allowed",
			err:      errors.New("webhook returned status 405: Method Not Allowed"),
			expected: false,
		},
		{
			name:     "status 410 gone",
			err:      errors.New("webhook returned status 410: Gone"),
			expected: false,
		},
		{
			name:     "status 413 payload too large",
			err:      errors.New("webhook returned status 413: Payload Too Large"),
			expected: false,
		},
		{
			name:     "status 415 unsupported media type",
			err:      errors.New("webhook returned status 415: Unsupported Media Type"),
			expected: false,
		},
		{
			name:     "status 422 unprocessable entity",
			err:      errors.New("webhook returned status 422: Unprocessable Entity"),
			expected: false,
		},
		// Note: 429 is handled specially as retryable (checked before 4xx loop)
		// Edge cases
		{
			name:     "unknown error defaults to retryable",
			err:      errors.New("something unexpected happened"),
			expected: true,
		},
		{
			name:     "empty error message",
			err:      errors.New(""),
			expected: true,
		},
		{
			name:     "TLS handshake error",
			err:      errors.New("tls: handshake failure"),
			expected: true,
		},
		{
			name:     "EOF error",
			err:      errors.New("unexpected EOF"),
			expected: true,
		},
		{
			name:     "DNS resolution error",
			err:      errors.New("lookup failed: no such host"),
			expected: true,
		},
		// Case insensitivity tests
		{
			name:     "TIMEOUT uppercase",
			err:      errors.New("REQUEST TIMEOUT"),
			expected: true,
		},
		{
			name:     "Connection Refused mixed case",
			err:      errors.New("Connection Refused"),
			expected: true,
		},
		{
			name:     "Network Unreachable mixed case",
			err:      errors.New("Network Unreachable"),
			expected: true,
		},
		// Real-world error formats
		{
			name:     "dial tcp timeout format",
			err:      errors.New("dial tcp 192.168.1.1:443: i/o timeout"),
			expected: true,
		},
		{
			name:     "context deadline exceeded",
			err:      errors.New("context deadline exceeded"),
			expected: true,
		},
		{
			name:     "wrapped timeout error",
			err:      fmt.Errorf("request failed: %w", errors.New("timeout")),
			expected: true,
		},
		{
			name:     "status code in message format",
			err:      errors.New("HTTP error: status 503"),
			expected: true,
		},
		{
			name:     "status code only",
			err:      errors.New("status 401"),
			expected: false,
		},
		// Boundary tests for status code ranges
		{
			name:     "status 499 last 4xx",
			err:      errors.New("webhook returned status 499"),
			expected: false,
		},
		{
			name:     "status 500 first 5xx",
			err:      errors.New("webhook returned status 500"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableWebhookError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableWebhookError(%q) = %v, want %v", tt.err.Error(), result, tt.expected)
			}
		})
	}
}

// TestIsRetryableWebhookError_AllStatusCodes verifies status code classification
func TestIsRetryableWebhookError_AllStatusCodes(t *testing.T) {
	// Test all 4xx codes are not retryable (except 429)
	for code := 400; code <= 499; code++ {
		err := fmt.Errorf("webhook returned status %d", code)
		result := isRetryableWebhookError(err)

		if code == 429 {
			// 429 is special - should be retryable
			if !result {
				t.Errorf("status %d should be retryable (rate limited)", code)
			}
		} else {
			// Other 4xx should not be retryable
			if result {
				t.Errorf("status %d should NOT be retryable (client error)", code)
			}
		}
	}

	// Test all 5xx codes are retryable
	for code := 500; code <= 599; code++ {
		err := fmt.Errorf("webhook returned status %d", code)
		result := isRetryableWebhookError(err)
		if !result {
			t.Errorf("status %d should be retryable (server error)", code)
		}
	}
}

// TestIsRetryableWebhookError_NetworkPatterns tests various network error patterns
func TestIsRetryableWebhookError_NetworkPatterns(t *testing.T) {
	networkErrors := []string{
		"dial tcp 10.0.0.1:443: connect: connection refused",
		"dial tcp: lookup api.example.com: no such host",
		"read tcp 192.168.1.1:52345->10.0.0.1:443: read: connection reset by peer",
		"write tcp 192.168.1.1:52345->10.0.0.1:443: write: broken pipe",
		"dial tcp 10.0.0.1:443: i/o timeout",
		"net/http: request canceled while waiting for connection (Client.Timeout exceeded)",
		"dial tcp [::1]:443: connect: network unreachable",
		"Post \"https://api.example.com/webhook\": dial tcp: lookup api.example.com: no such host",
	}

	for _, errStr := range networkErrors {
		t.Run(errStr[:min(50, len(errStr))], func(t *testing.T) {
			err := errors.New(errStr)
			if !isRetryableWebhookError(err) {
				t.Errorf("network error %q should be retryable", errStr)
			}
		})
	}
}
