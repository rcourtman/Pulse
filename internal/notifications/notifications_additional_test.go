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

func TestSendResolvedWebhookNtfySingleAlert(t *testing.T) {
	var gotMethod string
	var gotBody string
	var gotHeaders http.Header

	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotHeaders = r.Header.Clone()
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	manager := NewNotificationManager("")
	defer manager.Stop()
	manager.webhookClient = server.Client()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	resolvedAt := time.Date(2026, 2, 11, 12, 30, 0, 0, time.UTC)
	alertList := []*alerts.Alert{
		{
			ID:           "a1",
			Type:         "cpu",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "r1",
			ResourceName: "db-1",
			Node:         "node-1",
			Message:      "cpu high",
			Value:        91,
			Threshold:    80,
			StartTime:    time.Now().Add(-time.Minute),
		},
	}

	webhook := WebhookConfig{
		Name:    "resolved-ntfy",
		URL:     server.URL + "/ntfy",
		Enabled: true,
		Service: "ntfy",
		Headers: map[string]string{
			"X-Static":   "keep-me",
			"X-Template": "{{.ResourceName}}",
		},
	}

	if err := manager.sendResolvedWebhookNtfy(webhook, alertList, resolvedAt); err != nil {
		t.Fatalf("sendResolvedWebhookNtfy error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST method by default, got %q", gotMethod)
	}
	if gotHeaders.Get("Content-Type") != "text/plain" {
		t.Fatalf("expected text/plain content type, got %q", gotHeaders.Get("Content-Type"))
	}
	if gotHeaders.Get("Title") != "RESOLVED: db-1" {
		t.Fatalf("expected resolved title, got %q", gotHeaders.Get("Title"))
	}
	if gotHeaders.Get("X-Static") != "keep-me" {
		t.Fatalf("expected static header to be forwarded")
	}
	if gotHeaders.Get("X-Template") != "" {
		t.Fatalf("expected templated header value to be skipped, got %q", gotHeaders.Get("X-Template"))
	}
	if !strings.Contains(gotBody, "Resolved: db-1 on node-1 is now healthy") {
		t.Fatalf("unexpected body: %q", gotBody)
	}
}

func TestSendResolvedWebhookNtfyMultipleAlertsHTTPError(t *testing.T) {
	var gotMethod string
	var gotTitle string
	var gotBody string

	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotTitle = r.Header.Get("Title")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("temporarily unavailable"))
	}))
	defer server.Close()

	manager := NewNotificationManager("")
	defer manager.Stop()
	manager.webhookClient = server.Client()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	webhook := WebhookConfig{
		Name:    "resolved-ntfy",
		URL:     server.URL + "/ntfy",
		Method:  http.MethodPut,
		Enabled: true,
		Service: "ntfy",
	}

	alertList := []*alerts.Alert{
		{ResourceName: "db-1", Node: "node-1"},
		{ResourceName: "db-2", Node: "node-2"},
	}

	err := manager.sendResolvedWebhookNtfy(webhook, alertList, time.Date(2026, 2, 11, 12, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatalf("expected non-2xx response error")
	}
	if !strings.Contains(err.Error(), "ntfy webhook returned HTTP 503") {
		t.Fatalf("expected HTTP status in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "temporarily unavailable") {
		t.Fatalf("expected response body in error, got %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Fatalf("expected configured method PUT, got %q", gotMethod)
	}
	if gotTitle != "RESOLVED: 2 alerts" {
		t.Fatalf("expected grouped resolved title, got %q", gotTitle)
	}
	if !strings.Contains(gotBody, "2 alerts resolved") {
		t.Fatalf("expected grouped resolved body, got %q", gotBody)
	}
}

func TestSendResolvedWebhookNtfyValidationAndRateLimit(t *testing.T) {
	manager := NewNotificationManager("")
	defer manager.Stop()

	validAlert := []*alerts.Alert{{ResourceName: "vm-1", Node: "node-1"}}

	err := manager.sendResolvedWebhookNtfy(
		WebhookConfig{Name: "invalid-url", URL: "://bad-url", Enabled: true, Service: "ntfy"},
		validAlert,
		time.Now(),
	)
	if err == nil || !strings.Contains(err.Error(), "webhook URL validation failed") {
		t.Fatalf("expected URL validation failure, got %v", err)
	}

	rateLimitedURL := "http://127.0.0.1:1/ntfy"
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}
	manager.webhookRateLimits[rateLimitedURL] = &webhookRateLimit{
		lastSent:  time.Now(),
		sentCount: WebhookRateLimitMax,
	}

	err = manager.sendResolvedWebhookNtfy(
		WebhookConfig{Name: "rate-limited", URL: rateLimitedURL, Enabled: true, Service: "ntfy"},
		validAlert,
		time.Now(),
	)
	if err == nil || !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Fatalf("expected rate limit failure, got %v", err)
	}
}

func TestSendResolvedNotificationsDirectDispatchesEnabledTargets(t *testing.T) {
	origSpawn := spawnAsync
	spawnAsync = func(f func()) { f() }
	t.Cleanup(func() { spawnAsync = origSpawn })

	webhookHits := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	manager := NewNotificationManager("")
	defer manager.Stop()
	manager.webhookClient = server.Client()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	manager.emailManager = NewEnhancedEmailManager(EmailProviderConfig{
		EmailConfig: EmailConfig{
			From:     "old@example.com",
			To:       []string{"old@example.com"},
			SMTPHost: "invalid.localhost.test",
			SMTPPort: 25,
		},
		MaxRetries: 0,
		RetryDelay: 0,
		RateLimit:  0,
	})

	appriseCalled := false
	manager.appriseExec = func(ctx context.Context, path string, args []string) ([]byte, error) {
		appriseCalled = true
		return nil, nil
	}

	alertList := []*alerts.Alert{
		{
			ID:           "a1",
			Type:         "cpu",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   "r1",
			ResourceName: "db-1",
			Node:         "node-1",
			Message:      "cpu warning",
			Value:        87,
			Threshold:    80,
			StartTime:    time.Now().Add(-2 * time.Minute),
		},
	}

	manager.sendResolvedNotificationsDirect(
		EmailConfig{
			Enabled: true,
			From:    "new@example.com",
			To:      []string{"new@example.com"},
		},
		[]WebhookConfig{
			{Name: "enabled", URL: server.URL + "/ok", Enabled: true, Service: "ntfy"},
			{Name: "disabled", URL: server.URL + "/skip", Enabled: false, Service: "ntfy"},
		},
		AppriseConfig{
			Enabled:        true,
			Mode:           AppriseModeCLI,
			Targets:        []string{"discord://token"},
			CLIPath:        "apprise",
			TimeoutSeconds: 1,
		},
		alertList,
		time.Now(),
	)

	if webhookHits != 1 {
		t.Fatalf("expected exactly one resolved webhook request, got %d", webhookHits)
	}
	if !appriseCalled {
		t.Fatalf("expected resolved Apprise branch to be called")
	}
	if manager.emailManager.config.EmailConfig.From != "new@example.com" {
		t.Fatalf("expected email manager config update from resolved-email send, got %q", manager.emailManager.config.EmailConfig.From)
	}
}

func TestSendResolvedNotificationsDirectNoopForEmptyAlerts(t *testing.T) {
	origSpawn := spawnAsync
	spawned := 0
	spawnAsync = func(f func()) { spawned++ }
	t.Cleanup(func() { spawnAsync = origSpawn })

	manager := NewNotificationManager("")
	defer manager.Stop()

	manager.sendResolvedNotificationsDirect(
		EmailConfig{Enabled: true},
		[]WebhookConfig{{Enabled: true}},
		AppriseConfig{Enabled: true},
		nil,
		time.Now(),
	)

	if spawned != 0 {
		t.Fatalf("expected no async dispatch for empty resolved alert batch, got %d", spawned)
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

func TestNotificationManagerStopIsIdempotent(t *testing.T) {
	manager := NewNotificationManagerWithDataDir("", t.TempDir())

	manager.Stop()
	manager.Stop()
}
