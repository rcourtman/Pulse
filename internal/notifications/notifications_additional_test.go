package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestSendResolvedAppriseCLI(t *testing.T) {
	manager := NewNotificationManager("")
	defer manager.Stop()

	var called bool
	manager.appriseExec = func(ctx context.Context, path string, args []string) ([]byte, error) {
		called = true
		if path != "apprise" {
			t.Fatalf("expected CLI path apprise, got %q", path)
		}
		if len(args) == 0 || args[len(args)-1] != "target-1" {
			t.Fatalf("expected target to be passed, got %v", args)
		}
		if !containsArg(args, "-t") || !containsArg(args, "-b") {
			t.Fatalf("expected title/body args, got %v", args)
		}
		return nil, nil
	}

	config := AppriseConfig{
		Enabled:        true,
		Mode:           AppriseModeCLI,
		Targets:        []string{"target-1"},
		CLIPath:        "apprise",
		TimeoutSeconds: 1,
	}

	alertList := []*alerts.Alert{
		{
			ID:           "a1",
			Type:         "cpu",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "r1",
			ResourceName: "db-1",
			Message:      "cpu high",
			Value:        91,
			Threshold:    80,
			StartTime:    time.Now().Add(-time.Minute),
		},
	}

	if err := manager.sendResolvedApprise(config, alertList, time.Now()); err != nil {
		t.Fatalf("sendResolvedApprise error: %v", err)
	}
	if !called {
		t.Fatalf("expected apprise exec to be called")
	}
}

func TestSendGroupedWebhookGeneric(t *testing.T) {
	var gotMethod string
	var gotBody []byte

	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	manager := NewNotificationManager("")
	defer manager.Stop()
	manager.webhookClient = server.Client()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	alertsList := []*alerts.Alert{
		{
			ID:           "a1",
			Type:         "cpu",
			Level:        alerts.AlertLevelCritical,
			ResourceID:   "r1",
			ResourceName: "db-1",
			Message:      "cpu critical",
			Value:        99,
			Threshold:    90,
			StartTime:    time.Now().Add(-2 * time.Minute),
		},
		{
			ID:           "a2",
			Type:         "mem",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "r2",
			ResourceName: "cache-1",
			Message:      "memory high",
			Value:        85,
			Threshold:    80,
			StartTime:    time.Now().Add(-time.Minute),
		},
	}

	webhook := WebhookConfig{
		Name:    "generic",
		URL:     server.URL + "/hook",
		Enabled: true,
	}

	if err := manager.sendGroupedWebhook(webhook, alertsList); err != nil {
		t.Fatalf("sendGroupedWebhook error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}

	var payload map[string]any
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if grouped, ok := payload["grouped"].(bool); !ok || !grouped {
		t.Fatalf("expected grouped payload, got %v", payload["grouped"])
	}
	if count, ok := payload["count"].(float64); !ok || int(count) != len(alertsList) {
		t.Fatalf("expected count %d, got %v", len(alertsList), payload["count"])
	}
}

func TestSendResolvedWebhookHTTP(t *testing.T) {
	var gotBody []byte
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	manager := NewNotificationManager("")
	defer manager.Stop()
	manager.webhookClient = server.Client()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	alertList := []*alerts.Alert{
		{
			ID:           "a1",
			Type:         "disk",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "r1",
			ResourceName: "storage-1",
			Message:      "disk high",
			Value:        92,
			Threshold:    90,
			StartTime:    time.Now().Add(-time.Minute),
		},
	}

	webhook := WebhookConfig{
		Name:    "resolved",
		URL:     server.URL + "/resolved",
		Enabled: true,
	}

	if err := manager.sendResolvedWebhook(webhook, alertList, time.Now()); err != nil {
		t.Fatalf("sendResolvedWebhook error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["event"] != "resolved" {
		t.Fatalf("expected event resolved, got %v", payload["event"])
	}
	if payload["alertId"] != "a1" {
		t.Fatalf("expected alertId a1, got %v", payload["alertId"])
	}
}

func containsArg(args []string, value string) bool {
	for _, arg := range args {
		if strings.TrimSpace(arg) == value {
			return true
		}
	}
	return false
}

func TestNotificationManagerStop_Idempotent(t *testing.T) {
	manager := NewNotificationManager("")

	manager.Stop()
	manager.Stop()
}
