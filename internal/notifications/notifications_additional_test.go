package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestSendResolvedWebhookUsesServiceTemplate(t *testing.T) {
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	manager := NewNotificationManager("https://pulse.local")
	defer manager.Stop()
	manager.webhookClient = server.Client()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	alertList := []*alerts.Alert{
		{
			ID:           "a1",
			Type:         "cpu",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "vm100",
			ResourceName: "web-server",
			Node:         "pve1",
			Message:      "CPU usage high",
			Value:        95,
			Threshold:    90,
			StartTime:    time.Now().Add(-5 * time.Minute),
		},
	}

	webhook := WebhookConfig{
		Name:    "discord-test",
		URL:     server.URL + "/discord",
		Enabled: true,
		Service: "discord",
	}

	if err := manager.sendResolvedWebhook(webhook, alertList, time.Now()); err != nil {
		t.Fatalf("sendResolvedWebhook error: %v", err)
	}

	// Discord payloads must contain "embeds"
	var payload map[string]interface{}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if _, ok := payload["embeds"]; !ok {
		t.Fatalf("expected Discord payload to contain 'embeds', got keys: %v", payload)
	}

	// The embed title should contain "Resolved"
	embeds, ok := payload["embeds"].([]interface{})
	if !ok || len(embeds) == 0 {
		t.Fatalf("expected non-empty embeds array, got: %v", payload["embeds"])
	}
	embed, ok := embeds[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected embed to be an object, got: %T", embeds[0])
	}
	title, _ := embed["title"].(string)
	if !strings.Contains(title, "Resolved") {
		t.Fatalf("expected embed title to contain 'Resolved', got: %q", title)
	}

	// Should NOT contain the generic "event" key
	if _, ok := payload["event"]; ok {
		t.Fatal("expected service-specific payload, but found generic 'event' key")
	}
}

func TestSendResolvedWebhookGenericFallback(t *testing.T) {
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			Type:         "cpu",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "web-server",
			StartTime:    time.Now().Add(-time.Minute),
		},
	}

	// Generic webhook (no service) should still use the old generic payload
	webhook := WebhookConfig{
		Name:    "generic-hook",
		URL:     server.URL + "/generic",
		Enabled: true,
	}

	if err := manager.sendResolvedWebhook(webhook, alertList, time.Now()); err != nil {
		t.Fatalf("sendResolvedWebhook error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if payload["event"] != "resolved" {
		t.Fatalf("expected generic payload with event=resolved, got: %v", payload["event"])
	}
}

func TestSendResolvedWebhookSlackTemplate(t *testing.T) {
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	manager := NewNotificationManager("https://pulse.local")
	defer manager.Stop()
	manager.webhookClient = server.Client()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	alertList := []*alerts.Alert{
		{
			ID:           "a1",
			Type:         "memory",
			Level:        alerts.AlertLevelCritical,
			ResourceID:   "vm200",
			ResourceName: "db-server",
			Node:         "pve2",
			Message:      "Memory usage critical",
			Value:        98,
			Threshold:    95,
			StartTime:    time.Now().Add(-10 * time.Minute),
		},
	}

	webhook := WebhookConfig{
		Name:    "slack-test",
		URL:     server.URL + "/slack",
		Enabled: true,
		Service: "slack",
	}

	if err := manager.sendResolvedWebhook(webhook, alertList, time.Now()); err != nil {
		t.Fatalf("sendResolvedWebhook error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	// Slack payloads must contain "blocks" or "text"
	_, hasBlocks := payload["blocks"]
	_, hasText := payload["text"]
	if !hasBlocks && !hasText {
		t.Fatalf("expected Slack payload to contain 'blocks' or 'text', got keys: %v", payload)
	}
}

func TestSendResolvedWebhookPagerDutyResolve(t *testing.T) {
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	manager := NewNotificationManager("https://pulse.local")
	defer manager.Stop()
	manager.webhookClient = server.Client()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	alertList := []*alerts.Alert{
		{
			ID:           "a1",
			Type:         "cpu",
			Level:        alerts.AlertLevelCritical,
			ResourceID:   "vm100",
			ResourceName: "web-server",
			Node:         "pve1",
			Message:      "CPU usage critical",
			Value:        99,
			Threshold:    95,
			StartTime:    time.Now().Add(-15 * time.Minute),
		},
	}

	webhook := WebhookConfig{
		Name:    "pagerduty-test",
		URL:     server.URL,
		Enabled: true,
		Service: "pagerduty",
		Headers: map[string]string{
			"routing_key": "test-routing-key-123",
		},
	}

	if err := manager.sendResolvedWebhook(webhook, alertList, time.Now()); err != nil {
		t.Fatalf("sendResolvedWebhook error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	// PagerDuty resolved must use "resolve" event_action, not "trigger"
	if payload["event_action"] != "resolve" {
		t.Fatalf("expected event_action 'resolve', got: %v", payload["event_action"])
	}

	// routing_key must be populated from webhook headers
	if payload["routing_key"] != "test-routing-key-123" {
		t.Fatalf("expected routing_key 'test-routing-key-123', got: %v", payload["routing_key"])
	}
}

func TestSendResolvedWebhookNilAlertReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	manager := NewNotificationManager("")
	defer manager.Stop()
	manager.webhookClient = server.Client()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	// First alert is nil — should not panic, should return an error
	alertList := []*alerts.Alert{nil}

	webhook := WebhookConfig{
		Name:    "discord-nil",
		URL:     server.URL,
		Enabled: true,
		Service: "discord",
	}

	err := manager.sendResolvedWebhook(webhook, alertList, time.Now())
	if err == nil {
		t.Fatal("expected error for nil alert in service webhook, got nil")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Fatalf("expected error about nil alert, got: %v", err)
	}
}

func TestSendResolvedWebhookCustomTemplate(t *testing.T) {
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	manager := NewNotificationManager("https://pulse.local")
	defer manager.Stop()
	manager.webhookClient = server.Client()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	alertList := []*alerts.Alert{
		{
			ID:           "a1",
			Type:         "cpu",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "vm100",
			ResourceName: "web-server",
			Node:         "pve1",
			Message:      "CPU high",
			Value:        95,
			Threshold:    90,
			StartTime:    time.Now().Add(-5 * time.Minute),
		},
	}

	// Custom template should take precedence over service template
	webhook := WebhookConfig{
		Name:     "custom-discord",
		URL:      server.URL,
		Enabled:  true,
		Service:  "discord",
		Template: `{"content": "Custom resolved: {{.ResourceName}} - {{.Level}}"}`,
	}

	if err := manager.sendResolvedWebhook(webhook, alertList, time.Now()); err != nil {
		t.Fatalf("sendResolvedWebhook error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	// Custom template should have rendered, not the service template
	content, ok := payload["content"].(string)
	if !ok {
		t.Fatalf("expected 'content' field from custom template, got keys: %v", payload)
	}
	if !strings.Contains(content, "Custom resolved") {
		t.Fatalf("expected custom template content, got: %q", content)
	}
	if !strings.Contains(content, "resolved") {
		t.Fatalf("expected Level to be 'resolved' in custom template, got: %q", content)
	}

	// Should NOT have embeds (that would mean service template was used instead)
	if _, hasEmbeds := payload["embeds"]; hasEmbeds {
		t.Fatal("expected custom template to take precedence over service template, but got embeds")
	}
}
