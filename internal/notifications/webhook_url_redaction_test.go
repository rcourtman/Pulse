package notifications

import (
	"bytes"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestRedactWebhookURLSecrets(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"gotify token": {
			input: "https://gotify.example/message?token=gotify-secret",
			want:  "https://gotify.example/message?token=REDACTED",
		},
		"telegram path and query": {
			input: "https://api.telegram.org/bot123:secret/send?token=query-secret",
			want:  "https://api.telegram.org/botREDACTED/send?token=REDACTED",
		},
		"unrelated parameters": {
			input: "https://example.com/hook?extra_token=visible&channel=ops",
			want:  "https://example.com/hook?extra_token=visible&channel=ops",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := RedactWebhookURLSecrets(test.input); got != test.want {
				t.Fatalf("RedactWebhookURLSecrets() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRedactWebhookTransportErrorPreservesBehaviorWithoutToken(t *testing.T) {
	cause := errors.New("connection refused")
	original := &url.Error{
		Op:  "Post",
		URL: "https://gotify.example/message?token=gotify-secret",
		Err: cause,
	}

	redacted := redactWebhookTransportError(original)
	if strings.Contains(redacted.Error(), "gotify-secret") {
		t.Fatalf("redacted transport error exposed token: %v", redacted)
	}
	if !strings.Contains(redacted.Error(), "token=REDACTED") {
		t.Fatalf("redacted transport error omitted diagnostic URL shape: %v", redacted)
	}
	if !errors.Is(redacted, cause) {
		t.Fatal("redacted transport error no longer unwraps to its original cause")
	}
}

// TestWebhookRateLimitLogsRedactURLSecrets captures the actual log output of
// the rate-limit drop paths. Both of these logged the raw webhook URL until
// this test existed: the redaction sweep converted five call sites but missed
// checkWebhookRateLimit and the enhanced sender, and no test asserted log
// content, so a token in the URL still reached the logs on the one event most
// likely to fire repeatedly.
func TestWebhookRateLimitLogsRedactURLSecrets(t *testing.T) {
	const secret = "gotify-secret"
	webhookURL := "https://gotify.example/message?token=" + secret

	var captured bytes.Buffer
	original := log.Logger
	log.Logger = zerolog.New(&captured)
	t.Cleanup(func() { log.Logger = original })

	nm := &NotificationManager{
		webhookRateLimits: make(map[string]*webhookRateLimit),
	}

	// Exhaust the window so the drop path logs.
	for range WebhookRateLimitMax + 2 {
		nm.checkWebhookRateLimit(webhookURL)
	}

	out := captured.String()
	if !strings.Contains(out, "rate limit exceeded") {
		t.Fatalf("expected the rate-limit drop to be logged, got %q", out)
	}
	if strings.Contains(out, secret) {
		t.Fatalf("webhook token leaked into logs: %q", out)
	}
	if !strings.Contains(out, "token=REDACTED") {
		t.Fatalf("expected redacted url in logs, got %q", out)
	}
}
