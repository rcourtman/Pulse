package notifications

import (
	"errors"
	"net/url"
	"strings"
	"testing"
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
