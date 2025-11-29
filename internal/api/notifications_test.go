package api

import "testing"

func TestRedactSecretsFromURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// No secrets - should pass through unchanged
		{
			name:     "no secrets in URL",
			input:    "https://example.com/webhook",
			expected: "https://example.com/webhook",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "URL with unrelated query params",
			input:    "https://example.com/api?foo=bar&baz=qux",
			expected: "https://example.com/api?foo=bar&baz=qux",
		},

		// Telegram bot token patterns
		{
			name:     "telegram bot token with sendMessage",
			input:    "https://api.telegram.org/bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11/sendMessage",
			expected: "https://api.telegram.org/botREDACTED/sendMessage",
		},
		{
			name:     "telegram bot token no trailing path",
			input:    "https://api.telegram.org/bot123456:ABC-token",
			expected: "https://api.telegram.org/botREDACTED",
		},
		{
			name:     "telegram bot token with query string",
			input:    "https://api.telegram.org/bot123456:ABC-token?chat_id=123",
			expected: "https://api.telegram.org/botREDACTED?chat_id=123",
		},
		{
			name:     "telegram bot token with path and query",
			input:    "https://api.telegram.org/bot123456:token/sendMessage?chat_id=123",
			expected: "https://api.telegram.org/botREDACTED/sendMessage?chat_id=123",
		},

		// Query parameter secrets
		{
			name:     "token query param",
			input:    "https://example.com/webhook?token=secret123",
			expected: "https://example.com/webhook?token=REDACTED",
		},
		{
			name:     "apikey query param",
			input:    "https://example.com/api?apikey=xyz123",
			expected: "https://example.com/api?apikey=REDACTED",
		},
		{
			name:     "api_key query param with underscore",
			input:    "https://example.com/api?api_key=xyz123",
			expected: "https://example.com/api?api_key=REDACTED",
		},
		{
			name:     "key query param",
			input:    "https://example.com/api?key=mykey123",
			expected: "https://example.com/api?key=REDACTED",
		},
		{
			name:     "secret query param",
			input:    "https://example.com/api?secret=mysecret",
			expected: "https://example.com/api?secret=REDACTED",
		},
		{
			name:     "password query param",
			input:    "https://example.com/api?password=pass123",
			expected: "https://example.com/api?password=REDACTED",
		},

		// Multiple parameters
		{
			name:     "secret param with other params before",
			input:    "https://example.com/api?foo=bar&token=secret",
			expected: "https://example.com/api?foo=bar&token=REDACTED",
		},
		{
			name:     "secret param with other params after",
			input:    "https://example.com/api?token=secret&foo=bar",
			expected: "https://example.com/api?token=REDACTED&foo=bar",
		},
		{
			name:     "multiple different secret params",
			input:    "https://example.com/api?token=tok&apikey=key",
			expected: "https://example.com/api?token=REDACTED&apikey=REDACTED",
		},

		// Edge cases
		{
			name:     "secret param with fragment",
			input:    "https://example.com/api?token=secret#section",
			expected: "https://example.com/api?token=REDACTED#section",
		},
		{
			name:     "bot in path but not telegram pattern",
			input:    "https://example.com/robots.txt",
			expected: "https://example.com/robots.txt",
		},
		{
			name:     "combined telegram and query param secrets",
			input:    "https://api.telegram.org/bot123:token/send?extra_token=abc",
			expected: "https://api.telegram.org/botREDACTED/send?extra_token=REDACTED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSecretsFromURL(tt.input)
			if result != tt.expected {
				t.Errorf("redactSecretsFromURL(%q)\ngot:  %q\nwant: %q", tt.input, result, tt.expected)
			}
		})
	}
}
