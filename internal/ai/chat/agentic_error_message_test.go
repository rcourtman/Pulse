package chat

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeProviderStreamErrorForUser(t *testing.T) {
	// The exact shape the user hit: an OpenRouter 402 with a JSON body and a
	// workspace-key dashboard URL. None of that may reach the chat.
	openRouter402 := `stream error: API error (402): {"error":{"message":"This request requires more credits, or fewer max_tokens. You requested up to 65536 tokens, but can only afford 40. To increase, visit https://openrouter.ai/workspaces/default/keys/69abc"}}`

	cases := []struct {
		name        string
		in          string
		wantContain string
		wantAbsent  []string
	}{
		{
			name:        "billing 402 maps to a clean actionable message",
			in:          openRouter402,
			wantContain: "billing or quota",
			wantAbsent:  []string{"{", "http", "max_tokens", "402", "openrouter.ai"},
		},
		{
			name:        "auth error",
			in:          `API error (401): {"error":"invalid api key"}`,
			wantContain: "credentials",
			wantAbsent:  []string{"{", "401", "invalid api key"},
		},
		{
			name:        "rate limit",
			in:          "API error (429): too many requests",
			wantContain: "rate limiting",
			wantAbsent:  []string{"429"},
		},
		{
			name:        "timeout",
			in:          "context deadline exceeded",
			wantContain: "timed out",
		},
		{
			name:        "canceled",
			in:          "context canceled",
			wantContain: "canceled",
		},
		{
			name:        "generic error strips JSON body but keeps the summary",
			in:          `API error (500): {"error":"internal"}`,
			wantContain: "API error (500)",
			wantAbsent:  []string{"{", "internal"},
		},
		{
			name:        "generic error strips trailing URL",
			in:          "upstream failed see https://example.com/secret",
			wantContain: "upstream failed",
			wantAbsent:  []string{"http", "secret"},
		},
		{
			name:        "empty falls back to default",
			in:          "   ",
			wantContain: "interrupted before completion",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeProviderStreamErrorForUser(tc.in)
			if !strings.Contains(got, tc.wantContain) {
				t.Fatalf("expected %q to contain %q", got, tc.wantContain)
			}
			lowerGot := strings.ToLower(got)
			for _, absent := range tc.wantAbsent {
				if strings.Contains(lowerGot, strings.ToLower(absent)) {
					t.Fatalf("sanitized message %q must not contain %q", got, absent)
				}
			}
		})
	}
}

func TestFallbackProviderStreamErrorMessage_NilAndError(t *testing.T) {
	if got := fallbackProviderStreamErrorMessage(nil); !strings.Contains(got, "interrupted before completion") {
		t.Fatalf("nil error should use the default message, got %q", got)
	}
	got := fallbackProviderStreamErrorMessage(errors.New(`API error (402): {"x":"https://o.ai/keys/1"}`))
	for _, absent := range []string{"{", "http", "402"} {
		if strings.Contains(strings.ToLower(got), absent) {
			t.Fatalf("message %q must not leak %q", got, absent)
		}
	}
}
