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
		"discord":                          {input: "https://discord.com/api/webhooks/123/discord-secret", want: "https://discord.com/api/webhooks/REDACTED"},
		"discord versioned":                {input: "https://discord.com/api/v10/webhooks/123/discord-secret", want: "https://discord.com/api/v10/webhooks/REDACTED"},
		"discord legacy":                   {input: "https://discordapp.com/api/webhooks/123/discord-secret", want: "https://discordapp.com/api/webhooks/REDACTED"},
		"discord encoded":                  {input: "https://user:password@DISCORD.COM:443/api/webhooks/123/discord%2Dsecret", want: "https://REDACTED@DISCORD.COM:443/api/webhooks/REDACTED"},
		"discord query":                    {input: "https://discord.com/api/webhooks/123/discord-secret?wait=true&token=query-secret", want: "https://discord.com/api/webhooks/REDACTED?wait=true&token=REDACTED"},
		"discord lookalike":                {input: "https://discord.com.example.org/api/webhooks/status", want: "https://discord.com.example.org/api/webhooks/status"},
		"discord unrelated":                {input: "https://discord.com/api/status", want: "https://discord.com/api/status"},
		"slack":                            {input: "https://hooks.slack.com/services/T-test/B-test/slack-secret", want: "https://hooks.slack.com/services/REDACTED"},
		"gov slack":                        {input: "https://hooks.slack-gov.com/services/T-test/B-test/slack-secret?token=query-secret&channel=ops", want: "https://hooks.slack-gov.com/services/REDACTED?token=REDACTED&channel=ops"},
		"slack encoded path and authority": {input: "https://user:password@HOOKS.SLACK.COM:443/serv%69ces/T-test/B-test/slack%2Dsecret", want: "https://REDACTED@HOOKS.SLACK.COM:443/services/REDACTED"},
		"slack legacy path":                {input: "https://hooks.slack.com/T-test/B-test/slack-secret", want: "https://hooks.slack.com/REDACTED"},
		"unrelated services path":          {input: "https://example.com/services/status", want: "https://example.com/services/status"},
		"slack lookalike":                  {input: "https://hooks.slack.com.example.org/services/status", want: "https://hooks.slack.com.example.org/services/status"},
		"basic auth": {
			input: "https://hook-user:hook-password@example.com/hook",
			want:  "https://REDACTED@example.com/hook",
		},
		"username only": {
			input: "https://hook-user@example.com/hook",
			want:  "https://REDACTED@example.com/hook",
		},
		"encoded credentials with other secrets": {
			input: "https://hook%40user:p%40ss@example.com/bot123:secret/send?token=query-secret&channel=ops",
			want:  "https://REDACTED@example.com/botREDACTED/send?token=REDACTED&channel=ops",
		},
		"malformed credential URL": {
			input: "https://hook-user:hook-password@example.com/%zz",
			want:  "[invalid webhook URL]",
		},
		"at sign outside authority": {
			input: "https://example.com/hooks/@ops?channel=team@ops",
			want:  "https://example.com/hooks/@ops?channel=team@ops",
		},
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
		URL: "https://hook-user:hook-password@gotify.example/message?token=gotify-secret",
		Err: cause,
	}

	redacted := redactWebhookTransportError(original)
	if strings.Contains(redacted.Error(), "gotify-secret") || strings.Contains(redacted.Error(), "hook-password") || strings.Contains(redacted.Error(), "hook-user") {
		t.Fatalf("redacted transport error exposed token: %v", redacted)
	}
	if !strings.Contains(redacted.Error(), "token=REDACTED") {
		t.Fatalf("redacted transport error omitted diagnostic URL shape: %v", redacted)
	}
	if original.URL != "https://hook-user:hook-password@gotify.example/message?token=gotify-secret" {
		t.Fatal("redaction mutated the original transport error")
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
	webhookURL := "https://hook-user:hook-password@gotify.example/message?token=" + secret

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
	if strings.Contains(out, secret) || strings.Contains(out, "hook-password") || strings.Contains(out, "hook-user") {
		t.Fatalf("webhook token leaked into logs: %q", out)
	}
	if !strings.Contains(out, "token=REDACTED") {
		t.Fatalf("expected redacted url in logs, got %q", out)
	}
}

func TestSlackWebhookDiagnosticsRedactPath(t *testing.T) {
	const webhookURL = "https://hooks.slack.com/services/T-test/B-test/slack-secret"
	cause := errors.New("connection refused")
	original := &url.Error{Op: "Post", URL: webhookURL, Err: cause}
	redacted := redactWebhookTransportError(original)
	if strings.Contains(redacted.Error(), "slack-secret") || !strings.Contains(redacted.Error(), "/services/REDACTED") {
		t.Fatalf("unsafe transport diagnostic: %v", redacted)
	}
	if original.URL != webhookURL || !errors.Is(redacted, cause) {
		t.Fatal("transport error identity or cause changed")
	}
	var captured bytes.Buffer
	logger := log.Logger
	log.Logger = zerolog.New(&captured)
	t.Cleanup(func() { log.Logger = logger })
	nm := &NotificationManager{webhookRateLimits: make(map[string]*webhookRateLimit)}
	for range WebhookRateLimitMax + 2 {
		nm.checkWebhookRateLimit(webhookURL)
	}
	out := captured.String()
	if !strings.Contains(out, "rate limit exceeded") || !strings.Contains(out, "/services/REDACTED") || strings.Contains(out, "slack-secret") {
		t.Fatalf("unsafe rate-limit diagnostic: %s", out)
	}
}
func TestDiscordWebhookDiagnosticsRedactPath(t *testing.T) {
	const webhookURL = "https://discord.com/api/webhooks/123/discord-secret"
	cause := errors.New("connection refused")
	original := &url.Error{Op: "Post", URL: webhookURL, Err: cause}
	redacted := redactWebhookTransportError(original)
	if strings.Contains(redacted.Error(), "discord-secret") || !strings.Contains(redacted.Error(), "/api/webhooks/REDACTED") {
		t.Fatalf("unsafe transport diagnostic: %v", redacted)
	}
	if original.URL != webhookURL || !errors.Is(redacted, cause) {
		t.Fatal("transport error identity or cause changed")
	}
	var captured bytes.Buffer
	logger := log.Logger
	log.Logger = zerolog.New(&captured)
	t.Cleanup(func() { log.Logger = logger })
	nm := &NotificationManager{webhookRateLimits: make(map[string]*webhookRateLimit)}
	for range WebhookRateLimitMax + 2 {
		nm.checkWebhookRateLimit(webhookURL)
	}
	out := captured.String()
	if !strings.Contains(out, "rate limit exceeded") || !strings.Contains(out, "/api/webhooks/REDACTED") || strings.Contains(out, "discord-secret") {
		t.Fatalf("unsafe rate-limit diagnostic: %s", out)
	}
}
