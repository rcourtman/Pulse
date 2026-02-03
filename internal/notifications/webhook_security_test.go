package notifications

import (
	"strings"
	"testing"
)

func TestSendWebhookRequestRevalidatesURL(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	webhook := WebhookConfig{
		Name: "blocked",
		URL:  "http://127.0.0.1/webhook",
	}

	err := nm.sendWebhookRequest(webhook, []byte(`{}`), "alert")
	if err == nil {
		t.Fatalf("expected validation error for localhost webhook URL")
	}
	if !strings.Contains(err.Error(), "webhook URL validation failed") {
		t.Fatalf("expected validation error, got %v", err)
	}
}
