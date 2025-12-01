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

func flushPending(n *NotificationManager) {
	n.mu.Lock()
	if n.queue != nil {
		// Tests don't rely on the persistent queue; shutting it down ensures sends happen synchronously.
		_ = n.queue.Stop()
		n.queue = nil
	}
	if n.groupTimer != nil {
		n.groupTimer.Stop()
		n.groupTimer = nil
	}
	n.mu.Unlock()
	n.sendGroupedAlerts()
}

func TestNormalizeAppriseConfig(t *testing.T) {
	original := AppriseConfig{
		Enabled:        true,
		Targets:        []string{"  discord://token  ", "", "DISCORD://TOKEN"},
		CLIPath:        " ",
		TimeoutSeconds: -5,
		APIKeyHeader:   "",
	}

	normalized := NormalizeAppriseConfig(original)

	if normalized.Mode != AppriseModeCLI {
		t.Fatalf("expected default mode cli, got %q", normalized.Mode)
	}

	if normalized.CLIPath != "apprise" {
		t.Fatalf("expected default CLI path 'apprise', got %q", normalized.CLIPath)
	}

	if normalized.TimeoutSeconds != 15 {
		t.Fatalf("expected timeout of 15 seconds, got %d", normalized.TimeoutSeconds)
	}

	if !normalized.Enabled {
		t.Fatalf("expected config to remain enabled when targets exist")
	}

	if len(normalized.Targets) != 1 || normalized.Targets[0] != "discord://token" {
		t.Fatalf("unexpected targets normalization result: %#v", normalized.Targets)
	}

	if normalized.APIKeyHeader != "X-API-KEY" {
		t.Fatalf("expected default API key header, got %q", normalized.APIKeyHeader)
	}

	// When all targets removed, enabled should reset to false
	empty := NormalizeAppriseConfig(AppriseConfig{Enabled: true})
	if empty.Enabled {
		t.Fatalf("expected enabled to be false when no targets configured")
	}

	httpConfig := NormalizeAppriseConfig(AppriseConfig{
		Enabled:        true,
		Mode:           AppriseModeHTTP,
		ServerURL:      "https://apprise.example.com/api/",
		APIKey:         "  secret ",
		APIKeyHeader:   "  X-Token ",
		TimeoutSeconds: 200,
	})

	if httpConfig.Mode != AppriseModeHTTP {
		t.Fatalf("expected HTTP mode, got %q", httpConfig.Mode)
	}
	if httpConfig.ServerURL != "https://apprise.example.com/api" {
		t.Fatalf("expected server URL to be trimmed, got %q", httpConfig.ServerURL)
	}
	if httpConfig.APIKey != "secret" {
		t.Fatalf("expected API key to be trimmed, got %q", httpConfig.APIKey)
	}
	if httpConfig.APIKeyHeader != "X-Token" {
		t.Fatalf("expected API key header to be trimmed, got %q", httpConfig.APIKeyHeader)
	}
	if httpConfig.TimeoutSeconds != 120 {
		t.Fatalf("expected timeout to clamp to 120, got %d", httpConfig.TimeoutSeconds)
	}
	if !httpConfig.Enabled {
		t.Fatalf("expected HTTP config with server URL to remain enabled")
	}

	disabledHTTP := NormalizeAppriseConfig(AppriseConfig{
		Enabled: true,
		Mode:    AppriseModeHTTP,
	})
	if disabledHTTP.Enabled {
		t.Fatalf("expected HTTP config without server URL to disable notifications")
	}
}

func TestSetCooldownClampsNegativeValues(t *testing.T) {
	nm := NewNotificationManager("")
	nm.SetCooldown(-10)

	nm.mu.RLock()
	if nm.cooldown != 0 {
		nm.mu.RUnlock()
		t.Fatalf("expected cooldown to clamp to zero, got %s", nm.cooldown)
	}
	nm.mu.RUnlock()

	nm.SetCooldown(5)
	nm.mu.RLock()
	if nm.cooldown != 5*time.Minute {
		nm.mu.RUnlock()
		t.Fatalf("expected cooldown of five minutes, got %s", nm.cooldown)
	}
	nm.mu.RUnlock()
}

func TestSetGroupingWindowClampsNegativeValues(t *testing.T) {
	nm := NewNotificationManager("")
	nm.SetGroupingWindow(-60)

	nm.mu.RLock()
	if nm.groupWindow != 0 {
		nm.mu.RUnlock()
		t.Fatalf("expected grouping window to clamp to zero, got %s", nm.groupWindow)
	}
	nm.mu.RUnlock()

	nm.SetGroupingWindow(120)
	nm.mu.RLock()
	if nm.groupWindow != 120*time.Second {
		nm.mu.RUnlock()
		t.Fatalf("expected grouping window of 120 seconds, got %s", nm.groupWindow)
	}
	nm.mu.RUnlock()
}

func TestSendGroupedAppriseInvokesExecutor(t *testing.T) {
	nm := NewNotificationManager("")
	nm.SetGroupingWindow(0)
	nm.SetEmailConfig(EmailConfig{Enabled: false})

	done := make(chan struct{})
	var capturedArgs []string

	nm.appriseExec = func(ctx context.Context, path string, args []string) ([]byte, error) {
		if path != "apprise" {
			t.Fatalf("expected CLI path 'apprise', got %q", path)
		}
		capturedArgs = append([]string(nil), args...)
		close(done)
		return []byte("success"), nil
	}

	nm.SetAppriseConfig(AppriseConfig{
		Enabled:        true,
		Targets:        []string{"discord://token"},
		TimeoutSeconds: 10,
	})

	alert := &alerts.Alert{
		ID:           "test",
		Type:         "cpu",
		Level:        alerts.AlertLevelCritical,
		ResourceID:   "vm-100",
		ResourceName: "vm-100",
		Message:      "CPU usage high",
		Value:        95,
		Threshold:    90,
		StartTime:    time.Now().Add(-time.Minute),
		LastSeen:     time.Now(),
	}

	nm.mu.Lock()
	nm.pendingAlerts = append(nm.pendingAlerts, alert)
	nm.mu.Unlock()

	nm.sendGroupedAlerts()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for Apprise executor to run")
	}

	if len(capturedArgs) == 0 {
		t.Fatalf("expected Apprise executor to receive arguments")
	}

	if capturedArgs[len(capturedArgs)-1] != "discord://token" {
		t.Fatalf("expected target URL as last argument, got %v", capturedArgs)
	}
}

func TestSendGroupedAppriseHTTP(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	nm := NewNotificationManager("https://pulse.local")
	defer nm.Stop()
	nm.SetGroupingWindow(0)
	nm.SetEmailConfig(EmailConfig{Enabled: false})

	type apprisePayload struct {
		Body  string   `json:"body"`
		Title string   `json:"title"`
		Type  string   `json:"type"`
		URLs  []string `json:"urls"`
	}

	type capturedRequest struct {
		Method      string
		Path        string
		ContentType string
		APIKey      string
		Payload     apprisePayload
	}

	requests := make(chan capturedRequest, 1)
	errs := make(chan error, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			errs <- err
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var payload apprisePayload
		if err := json.Unmarshal(body, &payload); err != nil {
			errs <- err
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		requests <- capturedRequest{
			Method:      r.Method,
			Path:        r.URL.Path,
			ContentType: r.Header.Get("Content-Type"),
			APIKey:      r.Header.Get("X-Test-Key"),
			Payload:     payload,
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	// Allow localhost for test server (SSRF protection normally blocks this)
	if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.1"); err != nil {
		t.Fatalf("failed to configure allowlist: %v", err)
	}

	nm.SetAppriseConfig(AppriseConfig{
		Enabled:        true,
		Mode:           AppriseModeHTTP,
		ServerURL:      server.URL,
		ConfigKey:      "primary",
		APIKey:         "secret",
		APIKeyHeader:   "X-Test-Key",
		Targets:        []string{"discord://token"},
		TimeoutSeconds: 10,
	})

	alert := &alerts.Alert{
		ID:           "test",
		Type:         "cpu",
		Level:        alerts.AlertLevelCritical,
		ResourceID:   "vm-100",
		ResourceName: "vm-100",
		Message:      "CPU usage high",
		Value:        95,
		Threshold:    90,
		StartTime:    time.Now().Add(-time.Minute),
		LastSeen:     time.Now(),
	}

	nm.mu.Lock()
	nm.pendingAlerts = append(nm.pendingAlerts, alert)
	nm.mu.Unlock()

	nm.sendGroupedAlerts()

	var req capturedRequest
	select {
	case req = <-requests:
	case err := <-errs:
		t.Fatalf("server error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for Apprise API request")
	}

	if req.Method != http.MethodPost {
		t.Fatalf("expected POST request, got %s", req.Method)
	}
	if req.Path != "/notify/primary" {
		t.Fatalf("expected notify path with config key, got %s", req.Path)
	}
	if req.ContentType != "application/json" {
		t.Fatalf("expected JSON content type, got %s", req.ContentType)
	}
	if req.APIKey != "secret" {
		t.Fatalf("expected API key header to be set, got %q", req.APIKey)
	}
	if req.Payload.Title != "Pulse alert: vm-100" {
		t.Fatalf("unexpected title: %s", req.Payload.Title)
	}
	if req.Payload.Type != "failure" {
		t.Fatalf("expected failure notification type, got %s", req.Payload.Type)
	}
	if len(req.Payload.URLs) != 1 || req.Payload.URLs[0] != "discord://token" {
		t.Fatalf("unexpected URLs in payload: %#v", req.Payload.URLs)
	}
	if !strings.Contains(req.Payload.Body, "CPU usage high") {
		t.Fatalf("expected alert message in payload body, got %s", req.Payload.Body)
	}
	if !strings.Contains(req.Payload.Body, "Dashboard: https://pulse.local") {
		t.Fatalf("expected dashboard link in payload body, got %s", req.Payload.Body)
	}
}

func TestNotificationCooldownAllowsNewAlertInstance(t *testing.T) {
	nm := NewNotificationManager("")
	nm.SetCooldown(1)          // 1 minute cooldown
	nm.SetGroupingWindow(3600) // keep timer from firing immediately

	alertStart := time.Now().Add(-time.Minute)
	alertA := &alerts.Alert{
		ID:        "vm-100-memory",
		Type:      "memory",
		Level:     alerts.AlertLevelWarning,
		StartTime: alertStart,
	}

	nm.SendAlert(alertA)
	flushPending(nm)

	nm.mu.RLock()
	firstRecord, ok := nm.lastNotified[alertA.ID]
	nm.mu.RUnlock()
	if !ok {
		t.Fatalf("first notification not recorded")
	}

	nm.SendAlert(alertA)

	nm.mu.RLock()
	pendingAfter := len(nm.pendingAlerts)
	nm.mu.RUnlock()
	if pendingAfter != 0 {
		t.Fatalf("cooldown alert should not be queued, found %d pending", pendingAfter)
	}

	alertRestart := &alerts.Alert{
		ID:        "vm-100-memory",
		Type:      "memory",
		Level:     alerts.AlertLevelWarning,
		StartTime: alertStart.Add(time.Minute),
	}

	nm.SendAlert(alertRestart)
	flushPending(nm)

	nm.mu.RLock()
	recordAfter := nm.lastNotified[alertRestart.ID]
	nm.mu.RUnlock()

	if !recordAfter.alertStart.Equal(alertRestart.StartTime) {
		t.Fatalf("expected alertStart %v, got %v", alertRestart.StartTime, recordAfter.alertStart)
	}
	if !recordAfter.lastSent.After(firstRecord.lastSent) {
		t.Fatalf("lastSent was not updated for new alert instance")
	}
}

func TestCancelAlertRemovesPending(t *testing.T) {
	nm := NewNotificationManager("")
	nm.SetGroupingWindow(120)

	alertA := &alerts.Alert{
		ID:        "vm-100-disk",
		Type:      "disk",
		Level:     alerts.AlertLevelWarning,
		StartTime: time.Now(),
	}
	alertB := &alerts.Alert{
		ID:        "vm-101-disk",
		Type:      "disk",
		Level:     alerts.AlertLevelWarning,
		StartTime: time.Now(),
	}

	nm.SendAlert(alertA)
	nm.SendAlert(alertB)

	nm.CancelAlert(alertA.ID)

	nm.mu.RLock()
	remaining := make([]string, 0, len(nm.pendingAlerts))
	for _, pending := range nm.pendingAlerts {
		if pending != nil {
			remaining = append(remaining, pending.ID)
		}
	}
	groupTimerActive := nm.groupTimer != nil
	nm.mu.RUnlock()

	if len(remaining) != 1 || remaining[0] != alertB.ID {
		t.Fatalf("expected only %s to remain pending, got %v", alertB.ID, remaining)
	}
	if !groupTimerActive {
		t.Fatalf("expected grouping timer to remain active while other alerts pending")
	}

	nm.CancelAlert(alertB.ID)

	nm.mu.RLock()
	if len(nm.pendingAlerts) != 0 {
		nm.mu.RUnlock()
		t.Fatalf("expected no pending alerts after cancelling all, found %d", len(nm.pendingAlerts))
	}
	timerStopped := nm.groupTimer == nil
	nm.mu.RUnlock()

	if !timerStopped {
		t.Fatalf("expected grouping timer to be cleared when no alerts remain")
	}
}

func TestConvertWebhookCustomFields(t *testing.T) {
	if result := convertWebhookCustomFields(nil); result != nil {
		t.Fatalf("expected nil for empty input, got %#v", result)
	}

	original := map[string]string{
		"app_token":  "abc123",
		"user_token": "user456",
	}

	converted := convertWebhookCustomFields(original)
	if len(converted) != len(original) {
		t.Fatalf("expected %d keys, got %d", len(original), len(converted))
	}

	for key, value := range original {
		if got, ok := converted[key]; !ok || got != value {
			t.Fatalf("expected %s=%s, got %v (present=%v)", key, value, got, ok)
		}
	}

	// Mutate original map and ensure converted copy remains unchanged
	original["extra"] = "new-value"
	if _, ok := converted["extra"]; ok {
		t.Fatalf("expected converted map to be independent of original mutations")
	}
}

func TestRenderWebhookURL_PathEncoding(t *testing.T) {
	data := WebhookPayloadData{
		Message: "CPU spike detected",
	}

	result, err := renderWebhookURL("https://example.com/alerts/{{.Message}}", data)
	if err != nil {
		t.Fatalf("expected no error rendering URL template, got %v", err)
	}

	expected := "https://example.com/alerts/CPU%20spike%20detected"
	if result != expected {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestRenderWebhookURL_QueryEncoding(t *testing.T) {
	data := WebhookPayloadData{
		Message: "CPU & Memory > 90%",
	}

	result, err := renderWebhookURL("https://hooks.example.com?msg={{urlquery .Message}}", data)
	if err != nil {
		t.Fatalf("expected no error rendering URL template, got %v", err)
	}

	expected := "https://hooks.example.com?msg=CPU+%26+Memory+%3E+90%25"
	if result != expected {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

func TestRenderWebhookURL_InvalidTemplate(t *testing.T) {
	_, err := renderWebhookURL("https://example.com/{{.Missing", WebhookPayloadData{})
	if err == nil {
		t.Fatalf("expected error for invalid URL template, got nil")
	}
}

func TestRenderWebhookURL_ErrorPaths(t *testing.T) {
	tests := []struct {
		name        string
		urlTemplate string
		data        WebhookPayloadData
		wantErr     string
	}{
		{
			name:        "empty URL template",
			urlTemplate: "",
			data:        WebhookPayloadData{},
			wantErr:     "webhook URL cannot be empty",
		},
		{
			name:        "whitespace-only URL template",
			urlTemplate: "   \t\n  ",
			data:        WebhookPayloadData{},
			wantErr:     "webhook URL cannot be empty",
		},
		{
			name:        "invalid template syntax",
			urlTemplate: "https://example.com/{{.Unclosed",
			data:        WebhookPayloadData{},
			wantErr:     "invalid webhook URL template",
		},
		{
			name:        "template execution error - undefined function",
			urlTemplate: "https://example.com/{{undefined_func .Message}}",
			data:        WebhookPayloadData{Message: "test"},
			wantErr:     "invalid webhook URL template",
		},
		{
			name:        "template produces empty URL",
			urlTemplate: "{{if false}}https://example.com{{end}}",
			data:        WebhookPayloadData{},
			wantErr:     "webhook URL template produced empty URL",
		},
		{
			name:        "template renders to missing scheme",
			urlTemplate: "{{.Message}}/path",
			data:        WebhookPayloadData{Message: "example.com"},
			wantErr:     "missing scheme or host",
		},
		{
			name:        "template renders to missing host",
			urlTemplate: "{{.Message}}://",
			data:        WebhookPayloadData{Message: "https"},
			wantErr:     "missing scheme or host",
		},
		{
			name:        "template renders to relative path",
			urlTemplate: "/{{.Message}}/webhook",
			data:        WebhookPayloadData{Message: "api"},
			wantErr:     "missing scheme or host",
		},
		{
			name:        "template renders to unparseable URL - malformed IPv6",
			urlTemplate: "http://[{{.Message}}",
			data:        WebhookPayloadData{Message: "::1"},
			wantErr:     "invalid URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderWebhookURL(tt.urlTemplate, tt.data)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil (result: %q)", tt.wantErr, result)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestRenderWebhookURL_SuccessCases(t *testing.T) {
	tests := []struct {
		name        string
		urlTemplate string
		data        WebhookPayloadData
		want        string
	}{
		{
			name:        "static URL - no template",
			urlTemplate: "https://example.com/webhook",
			data:        WebhookPayloadData{},
			want:        "https://example.com/webhook",
		},
		{
			name:        "URL with whitespace trimmed",
			urlTemplate: "  https://example.com/webhook  ",
			data:        WebhookPayloadData{},
			want:        "https://example.com/webhook",
		},
		{
			name:        "URL with template variable in path",
			urlTemplate: "https://example.com/{{.ResourceType}}/alert",
			data:        WebhookPayloadData{ResourceType: "vm"},
			want:        "https://example.com/vm/alert",
		},
		{
			name:        "URL with urlquery encoding",
			urlTemplate: "https://example.com?msg={{urlquery .Message}}",
			data:        WebhookPayloadData{Message: "hello world"},
			want:        "https://example.com?msg=hello+world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderWebhookURL(tt.urlTemplate, tt.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

func TestSendTestNotificationApprise(t *testing.T) {
	nm := NewNotificationManager("")
	nm.SetEmailConfig(EmailConfig{Enabled: false})

	// Test 1: Apprise not enabled should return error
	nm.SetAppriseConfig(AppriseConfig{
		Enabled: false,
		Targets: []string{"discord://token"},
	})

	err := nm.SendTestNotification("apprise")
	if err == nil {
		t.Fatalf("expected error when Apprise is disabled, got nil")
	}
	if !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("expected 'not enabled' error, got: %v", err)
	}

	// Test 2: Apprise enabled with CLI mode should invoke executor
	done := make(chan struct{})
	var capturedArgs []string

	nm.appriseExec = func(ctx context.Context, path string, args []string) ([]byte, error) {
		if path != "apprise" {
			t.Fatalf("expected CLI path 'apprise', got %q", path)
		}
		capturedArgs = append([]string(nil), args...)
		close(done)
		return []byte("success"), nil
	}

	nm.SetAppriseConfig(AppriseConfig{
		Enabled:        true,
		Targets:        []string{"discord://token"},
		TimeoutSeconds: 10,
	})

	err = nm.SendTestNotification("apprise")
	if err != nil {
		t.Fatalf("expected no error when testing Apprise, got: %v", err)
	}

	// Wait for the executor to be called
	select {
	case <-done:
		// Success - executor was called
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for Apprise executor to be called")
	}

	// Verify the arguments contain the target
	foundTarget := false
	for _, arg := range capturedArgs {
		if arg == "discord://token" {
			foundTarget = true
			break
		}
	}
	if !foundTarget {
		t.Fatalf("expected target 'discord://token' in args, got: %v", capturedArgs)
	}
}

func TestSendTestAppriseWithConfig(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	// Disabled config should fail
	err := nm.SendTestAppriseWithConfig(AppriseConfig{
		Enabled: false,
		Targets: []string{"discord://token"},
	})
	if err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("expected not enabled error, got %v", err)
	}

	done := make(chan struct{})
	var cliPath string

	nm.appriseExec = func(ctx context.Context, path string, args []string) ([]byte, error) {
		cliPath = path
		close(done)
		return []byte("ok"), nil
	}

	err = nm.SendTestAppriseWithConfig(AppriseConfig{
		Enabled: true,
		Mode:    AppriseModeCLI,
		Targets: []string{"discord://token"},
	})
	if err != nil {
		t.Fatalf("expected no error for valid Apprise config, got %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for Apprise test execution")
	}

	if cliPath != "apprise" {
		t.Fatalf("expected default CLI path 'apprise', got %q", cliPath)
	}
}

func TestSendTestNotificationAppriseHTTP(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	nm := NewNotificationManager("")
	defer nm.Stop()
	nm.SetEmailConfig(EmailConfig{Enabled: false})

	type apprisePayload struct {
		Body  string   `json:"body"`
		Title string   `json:"title"`
		Type  string   `json:"type"`
		URLs  []string `json:"urls"`
	}

	requests := make(chan apprisePayload, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var payload apprisePayload
		if err := json.Unmarshal(body, &payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		requests <- payload
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	// Allow localhost for test server (SSRF protection normally blocks this)
	if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.1"); err != nil {
		t.Fatalf("failed to configure allowlist: %v", err)
	}

	nm.SetAppriseConfig(AppriseConfig{
		Enabled:        true,
		Mode:           AppriseModeHTTP,
		ServerURL:      server.URL,
		ConfigKey:      "test-key",
		TimeoutSeconds: 10,
	})

	err := nm.SendTestNotification("apprise")
	if err != nil {
		t.Fatalf("expected no error when testing Apprise HTTP, got: %v", err)
	}

	// Wait for the HTTP request
	select {
	case payload := <-requests:
		// Verify the payload contains test alert information
		if payload.Title == "" {
			t.Fatalf("expected non-empty title in Apprise payload")
		}
		if payload.Body == "" {
			t.Fatalf("expected non-empty body in Apprise payload")
		}
		if !strings.Contains(payload.Body, "test alert") && !strings.Contains(payload.Body, "Test Resource") {
			t.Fatalf("expected test alert content in body, got: %s", payload.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for Apprise HTTP request")
	}
}

func TestPublicURL(t *testing.T) {
	t.Run("set and get URL", func(t *testing.T) {
		nm := NewNotificationManager("")
		nm.SetPublicURL("https://pulse.example.com")

		got := nm.GetPublicURL()
		if got != "https://pulse.example.com" {
			t.Fatalf("expected https://pulse.example.com, got %q", got)
		}
	})

	t.Run("empty string is no-op", func(t *testing.T) {
		nm := NewNotificationManager("")
		nm.SetPublicURL("https://pulse.example.com")
		nm.SetPublicURL("")

		got := nm.GetPublicURL()
		if got != "https://pulse.example.com" {
			t.Fatalf("expected URL to remain unchanged, got %q", got)
		}
	})

	t.Run("trailing slash is trimmed", func(t *testing.T) {
		nm := NewNotificationManager("")
		nm.SetPublicURL("https://pulse.example.com/")

		got := nm.GetPublicURL()
		if got != "https://pulse.example.com" {
			t.Fatalf("expected trailing slash to be trimmed, got %q", got)
		}
	})

	t.Run("whitespace is trimmed", func(t *testing.T) {
		nm := NewNotificationManager("")
		nm.SetPublicURL("  https://pulse.example.com  ")

		got := nm.GetPublicURL()
		if got != "https://pulse.example.com" {
			t.Fatalf("expected whitespace to be trimmed, got %q", got)
		}
	})

	t.Run("same URL twice is no-op", func(t *testing.T) {
		nm := NewNotificationManager("")
		nm.SetPublicURL("https://pulse.example.com")

		nm.mu.RLock()
		urlBefore := nm.publicURL
		nm.mu.RUnlock()

		nm.SetPublicURL("https://pulse.example.com")

		nm.mu.RLock()
		urlAfter := nm.publicURL
		nm.mu.RUnlock()

		if urlBefore != urlAfter {
			t.Fatalf("expected URL to remain unchanged")
		}
	})

	t.Run("whitespace-only is no-op", func(t *testing.T) {
		nm := NewNotificationManager("")
		nm.SetPublicURL("https://pulse.example.com")
		nm.SetPublicURL("   ")

		got := nm.GetPublicURL()
		if got != "https://pulse.example.com" {
			t.Fatalf("expected URL to remain unchanged after whitespace-only set, got %q", got)
		}
	})
}

func TestGetAppriseConfigReturnsCopy(t *testing.T) {
	nm := NewNotificationManager("")
	nm.SetAppriseConfig(AppriseConfig{
		Enabled:        true,
		Targets:        []string{"discord://token1", "slack://token2"},
		TimeoutSeconds: 30,
	})

	// Get a copy of the config
	configCopy := nm.GetAppriseConfig()

	// Modify the returned copy
	configCopy.Targets = append(configCopy.Targets, "telegram://token3")
	configCopy.Enabled = false
	configCopy.TimeoutSeconds = 60

	// Get another copy and verify the internal state wasn't affected
	configAfter := nm.GetAppriseConfig()

	if !configAfter.Enabled {
		t.Fatalf("modifying returned copy should not affect internal enabled state")
	}
	if configAfter.TimeoutSeconds != 30 {
		t.Fatalf("expected timeout 30, got %d", configAfter.TimeoutSeconds)
	}
	if len(configAfter.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(configAfter.Targets))
	}
	if configAfter.Targets[0] != "discord://token1" || configAfter.Targets[1] != "slack://token2" {
		t.Fatalf("internal targets were modified: %v", configAfter.Targets)
	}
}

func TestNotifyOnResolve(t *testing.T) {
	t.Run("default value is true", func(t *testing.T) {
		nm := NewNotificationManager("")

		if !nm.GetNotifyOnResolve() {
			t.Fatalf("expected default notifyOnResolve to be true")
		}
	})

	t.Run("set true and get", func(t *testing.T) {
		nm := NewNotificationManager("")
		nm.SetNotifyOnResolve(true)

		if !nm.GetNotifyOnResolve() {
			t.Fatalf("expected notifyOnResolve to be true after setting")
		}
	})

	t.Run("set false and get", func(t *testing.T) {
		nm := NewNotificationManager("")
		nm.SetNotifyOnResolve(false)

		if nm.GetNotifyOnResolve() {
			t.Fatalf("expected notifyOnResolve to be false after setting")
		}
	})
}

func TestGroupingOptions(t *testing.T) {
	t.Run("byNode=true, byGuest=false", func(t *testing.T) {
		nm := NewNotificationManager("")
		nm.SetGroupingOptions(true, false)

		nm.mu.RLock()
		byNode := nm.groupByNode
		byGuest := nm.groupByGuest
		nm.mu.RUnlock()

		if !byNode {
			t.Fatalf("expected groupByNode to be true")
		}
		if byGuest {
			t.Fatalf("expected groupByGuest to be false")
		}
	})

	t.Run("byNode=false, byGuest=true", func(t *testing.T) {
		nm := NewNotificationManager("")
		nm.SetGroupingOptions(false, true)

		nm.mu.RLock()
		byNode := nm.groupByNode
		byGuest := nm.groupByGuest
		nm.mu.RUnlock()

		if byNode {
			t.Fatalf("expected groupByNode to be false")
		}
		if !byGuest {
			t.Fatalf("expected groupByGuest to be true")
		}
	})

	t.Run("both true", func(t *testing.T) {
		nm := NewNotificationManager("")
		nm.SetGroupingOptions(true, true)

		nm.mu.RLock()
		byNode := nm.groupByNode
		byGuest := nm.groupByGuest
		nm.mu.RUnlock()

		if !byNode {
			t.Fatalf("expected groupByNode to be true")
		}
		if !byGuest {
			t.Fatalf("expected groupByGuest to be true")
		}
	})

	t.Run("both false", func(t *testing.T) {
		nm := NewNotificationManager("")
		nm.SetGroupingOptions(false, false)

		nm.mu.RLock()
		byNode := nm.groupByNode
		byGuest := nm.groupByGuest
		nm.mu.RUnlock()

		if byNode {
			t.Fatalf("expected groupByNode to be false")
		}
		if byGuest {
			t.Fatalf("expected groupByGuest to be false")
		}
	})
}

func TestWebhookAddAndGet(t *testing.T) {
	t.Run("add webhook and retrieve", func(t *testing.T) {
		nm := NewNotificationManager("")

		webhook := WebhookConfig{
			ID:      "webhook-1",
			Name:    "Test Webhook",
			URL:     "https://example.com/hook",
			Method:  "POST",
			Enabled: true,
			Service: "generic",
		}
		nm.AddWebhook(webhook)

		webhooks := nm.GetWebhooks()
		if len(webhooks) != 1 {
			t.Fatalf("expected 1 webhook, got %d", len(webhooks))
		}
		if webhooks[0].ID != "webhook-1" {
			t.Fatalf("expected webhook ID 'webhook-1', got %q", webhooks[0].ID)
		}
		if webhooks[0].Name != "Test Webhook" {
			t.Fatalf("expected webhook name 'Test Webhook', got %q", webhooks[0].Name)
		}
	})

	t.Run("add multiple webhooks", func(t *testing.T) {
		nm := NewNotificationManager("")

		nm.AddWebhook(WebhookConfig{ID: "webhook-1", Name: "First", URL: "https://example.com/1"})
		nm.AddWebhook(WebhookConfig{ID: "webhook-2", Name: "Second", URL: "https://example.com/2"})
		nm.AddWebhook(WebhookConfig{ID: "webhook-3", Name: "Third", URL: "https://example.com/3"})

		webhooks := nm.GetWebhooks()
		if len(webhooks) != 3 {
			t.Fatalf("expected 3 webhooks, got %d", len(webhooks))
		}

		ids := make(map[string]bool)
		for _, wh := range webhooks {
			ids[wh.ID] = true
		}
		if !ids["webhook-1"] || !ids["webhook-2"] || !ids["webhook-3"] {
			t.Fatalf("missing expected webhook IDs: %v", ids)
		}
	})

	t.Run("get webhooks returns empty slice when none", func(t *testing.T) {
		nm := NewNotificationManager("")

		webhooks := nm.GetWebhooks()
		if webhooks == nil {
			t.Fatalf("expected empty slice, got nil")
		}
		if len(webhooks) != 0 {
			t.Fatalf("expected 0 webhooks, got %d", len(webhooks))
		}
	})
}

func TestWebhookUpdate(t *testing.T) {
	t.Run("update existing webhook", func(t *testing.T) {
		nm := NewNotificationManager("")

		nm.AddWebhook(WebhookConfig{
			ID:      "webhook-1",
			Name:    "Original Name",
			URL:     "https://example.com/original",
			Enabled: true,
		})

		err := nm.UpdateWebhook("webhook-1", WebhookConfig{
			ID:      "webhook-1",
			Name:    "Updated Name",
			URL:     "https://example.com/updated",
			Enabled: false,
		})
		if err != nil {
			t.Fatalf("expected no error updating webhook, got %v", err)
		}

		webhooks := nm.GetWebhooks()
		if len(webhooks) != 1 {
			t.Fatalf("expected 1 webhook, got %d", len(webhooks))
		}
		if webhooks[0].Name != "Updated Name" {
			t.Fatalf("expected name 'Updated Name', got %q", webhooks[0].Name)
		}
		if webhooks[0].URL != "https://example.com/updated" {
			t.Fatalf("expected URL 'https://example.com/updated', got %q", webhooks[0].URL)
		}
		if webhooks[0].Enabled {
			t.Fatalf("expected enabled to be false")
		}
	})

	t.Run("update non-existent webhook returns error", func(t *testing.T) {
		nm := NewNotificationManager("")

		err := nm.UpdateWebhook("non-existent", WebhookConfig{
			ID:   "non-existent",
			Name: "Test",
		})
		if err == nil {
			t.Fatalf("expected error updating non-existent webhook, got nil")
		}
		if !strings.Contains(err.Error(), "webhook not found") {
			t.Fatalf("expected 'webhook not found' error, got: %v", err)
		}
	})
}

func TestWebhookDelete(t *testing.T) {
	t.Run("delete existing webhook", func(t *testing.T) {
		nm := NewNotificationManager("")

		nm.AddWebhook(WebhookConfig{ID: "webhook-1", Name: "First"})
		nm.AddWebhook(WebhookConfig{ID: "webhook-2", Name: "Second"})

		err := nm.DeleteWebhook("webhook-1")
		if err != nil {
			t.Fatalf("expected no error deleting webhook, got %v", err)
		}

		webhooks := nm.GetWebhooks()
		if len(webhooks) != 1 {
			t.Fatalf("expected 1 webhook after delete, got %d", len(webhooks))
		}
		if webhooks[0].ID != "webhook-2" {
			t.Fatalf("expected remaining webhook ID 'webhook-2', got %q", webhooks[0].ID)
		}
	})

	t.Run("delete non-existent webhook returns error", func(t *testing.T) {
		nm := NewNotificationManager("")

		err := nm.DeleteWebhook("non-existent")
		if err == nil {
			t.Fatalf("expected error deleting non-existent webhook, got nil")
		}
		if !strings.Contains(err.Error(), "webhook not found") {
			t.Fatalf("expected 'webhook not found' error, got: %v", err)
		}
	})

	t.Run("delete from middle of list", func(t *testing.T) {
		nm := NewNotificationManager("")

		nm.AddWebhook(WebhookConfig{ID: "webhook-1", Name: "First"})
		nm.AddWebhook(WebhookConfig{ID: "webhook-2", Name: "Second"})
		nm.AddWebhook(WebhookConfig{ID: "webhook-3", Name: "Third"})

		err := nm.DeleteWebhook("webhook-2")
		if err != nil {
			t.Fatalf("expected no error deleting middle webhook, got %v", err)
		}

		webhooks := nm.GetWebhooks()
		if len(webhooks) != 2 {
			t.Fatalf("expected 2 webhooks after delete, got %d", len(webhooks))
		}

		ids := make(map[string]bool)
		for _, wh := range webhooks {
			ids[wh.ID] = true
		}
		if !ids["webhook-1"] || !ids["webhook-3"] {
			t.Fatalf("expected webhook-1 and webhook-3 to remain, got: %v", ids)
		}
		if ids["webhook-2"] {
			t.Fatalf("webhook-2 should have been deleted")
		}
	})
}

func TestTemplateFuncMap(t *testing.T) {
	funcs := templateFuncMap()

	t.Run("title function", func(t *testing.T) {
		titleFn := funcs["title"].(func(string) string)

		// Empty string returns empty
		if got := titleFn(""); got != "" {
			t.Fatalf("expected empty string, got %q", got)
		}

		// Single character uppercased
		if got := titleFn("a"); got != "A" {
			t.Fatalf("expected 'A', got %q", got)
		}

		// Already uppercase single character
		if got := titleFn("Z"); got != "Z" {
			t.Fatalf("expected 'Z', got %q", got)
		}

		// Multi-character: first upper, rest lower
		if got := titleFn("HELLO"); got != "Hello" {
			t.Fatalf("expected 'Hello', got %q", got)
		}

		if got := titleFn("hello"); got != "Hello" {
			t.Fatalf("expected 'Hello', got %q", got)
		}

		if got := titleFn("hElLo"); got != "Hello" {
			t.Fatalf("expected 'Hello', got %q", got)
		}
	})

	t.Run("upper function", func(t *testing.T) {
		upperFn := funcs["upper"].(func(string) string)

		if got := upperFn("hello"); got != "HELLO" {
			t.Fatalf("expected 'HELLO', got %q", got)
		}

		if got := upperFn(""); got != "" {
			t.Fatalf("expected empty string, got %q", got)
		}

		if got := upperFn("Hello World"); got != "HELLO WORLD" {
			t.Fatalf("expected 'HELLO WORLD', got %q", got)
		}
	})

	t.Run("lower function", func(t *testing.T) {
		lowerFn := funcs["lower"].(func(string) string)

		if got := lowerFn("HELLO"); got != "hello" {
			t.Fatalf("expected 'hello', got %q", got)
		}

		if got := lowerFn(""); got != "" {
			t.Fatalf("expected empty string, got %q", got)
		}

		if got := lowerFn("Hello World"); got != "hello world" {
			t.Fatalf("expected 'hello world', got %q", got)
		}
	})

	t.Run("printf function", func(t *testing.T) {
		printfFn := funcs["printf"].(func(string, ...any) string)

		if got := printfFn("hello %s", "world"); got != "hello world" {
			t.Fatalf("expected 'hello world', got %q", got)
		}

		if got := printfFn("value: %d", 42); got != "value: 42" {
			t.Fatalf("expected 'value: 42', got %q", got)
		}

		if got := printfFn("%.2f%%", 95.5); got != "95.50%" {
			t.Fatalf("expected '95.50%%', got %q", got)
		}
	})

	t.Run("urlquery function", func(t *testing.T) {
		urlqueryFn := funcs["urlquery"].(func(...any) string)

		if got := urlqueryFn("hello world"); got != "hello+world" {
			t.Fatalf("expected 'hello+world', got %q", got)
		}

		if got := urlqueryFn("a=b&c=d"); got != "a%3Db%26c%3Dd" {
			t.Fatalf("expected 'a%%3Db%%26c%%3Dd', got %q", got)
		}

		if got := urlqueryFn("special: +/?#"); got != "special%3A+%2B%2F%3F%23" {
			t.Fatalf("expected 'special%%3A+%%2B%%2F%%3F%%23', got %q", got)
		}
	})

	t.Run("urlencode function (alias)", func(t *testing.T) {
		urlencodeFn := funcs["urlencode"].(func(...any) string)

		// Should behave identically to urlquery
		if got := urlencodeFn("hello world"); got != "hello+world" {
			t.Fatalf("expected 'hello+world', got %q", got)
		}

		if got := urlencodeFn("test@example.com"); got != "test%40example.com" {
			t.Fatalf("expected 'test%%40example.com', got %q", got)
		}
	})

	t.Run("urlpath function", func(t *testing.T) {
		urlpathFn := funcs["urlpath"].(func(string) string)

		// Spaces encoded as %20, not +
		if got := urlpathFn("hello world"); got != "hello%20world" {
			t.Fatalf("expected 'hello%%20world', got %q", got)
		}

		// Slashes encoded
		if got := urlpathFn("path/to/file"); got != "path%2Fto%2Ffile" {
			t.Fatalf("expected 'path%%2Fto%%2Ffile', got %q", got)
		}

		if got := urlpathFn(""); got != "" {
			t.Fatalf("expected empty string, got %q", got)
		}
	})

	t.Run("pathescape function", func(t *testing.T) {
		pathescapeFn := funcs["pathescape"].(func(string) string)

		// Should behave identically to urlpath
		if got := pathescapeFn("hello world"); got != "hello%20world" {
			t.Fatalf("expected 'hello%%20world', got %q", got)
		}

		if got := pathescapeFn("segment/with/slashes"); got != "segment%2Fwith%2Fslashes" {
			t.Fatalf("expected 'segment%%2Fwith%%2Fslashes', got %q", got)
		}

		// Special characters
		if got := pathescapeFn("test?query=1"); got != "test%3Fquery=1" {
			t.Fatalf("expected 'test%%3Fquery=1', got %q", got)
		}
	})
}

func TestGetEmailConfig(t *testing.T) {
	nm := NewNotificationManager("")

	config := EmailConfig{
		Enabled:  true,
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		Username: "user@example.com",
		Password: "secret",
		From:     "alerts@example.com",
		To:       []string{"admin@example.com", "ops@example.com"},
		StartTLS: true,
	}
	nm.SetEmailConfig(config)

	got := nm.GetEmailConfig()

	if !got.Enabled {
		t.Fatalf("expected enabled to be true")
	}
	if got.SMTPHost != "smtp.example.com" {
		t.Fatalf("expected host 'smtp.example.com', got %q", got.SMTPHost)
	}
	if got.SMTPPort != 587 {
		t.Fatalf("expected port 587, got %d", got.SMTPPort)
	}
	if got.Username != "user@example.com" {
		t.Fatalf("expected username 'user@example.com', got %q", got.Username)
	}
	if got.From != "alerts@example.com" {
		t.Fatalf("expected from 'alerts@example.com', got %q", got.From)
	}
	if len(got.To) != 2 {
		t.Fatalf("expected 2 recipients, got %d", len(got.To))
	}
	if !got.StartTLS {
		t.Fatalf("expected startTLS to be true")
	}
}

func TestBuildResolvedNotificationContent(t *testing.T) {
	t.Run("nil alert list returns empty strings", func(t *testing.T) {
		title, htmlBody, textBody := buildResolvedNotificationContent(nil, time.Now(), "")
		if title != "" || htmlBody != "" || textBody != "" {
			t.Fatalf("expected empty strings for nil list, got title=%q, htmlBody=%q, textBody=%q", title, htmlBody, textBody)
		}
	})

	t.Run("empty alert list returns empty strings", func(t *testing.T) {
		title, htmlBody, textBody := buildResolvedNotificationContent([]*alerts.Alert{}, time.Now(), "")
		if title != "" || htmlBody != "" || textBody != "" {
			t.Fatalf("expected empty strings for empty list, got title=%q, htmlBody=%q, textBody=%q", title, htmlBody, textBody)
		}
	})

	t.Run("list with only nil alerts returns empty strings", func(t *testing.T) {
		title, htmlBody, textBody := buildResolvedNotificationContent([]*alerts.Alert{nil, nil, nil}, time.Now(), "")
		if title != "" || htmlBody != "" || textBody != "" {
			t.Fatalf("expected empty strings for nil-only list, got title=%q, htmlBody=%q, textBody=%q", title, htmlBody, textBody)
		}
	})

	t.Run("single alert generates correct title and body", func(t *testing.T) {
		startTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		resolvedAt := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

		alert := &alerts.Alert{
			ID:           "test-alert-1",
			Type:         "cpu",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "vm-100",
			Message:      "CPU usage exceeded threshold",
			StartTime:    startTime,
			Node:         "pve1",
			Instance:     "vm-100",
			Threshold:    80,
			Value:        95.5,
		}

		title, htmlBody, textBody := buildResolvedNotificationContent([]*alerts.Alert{alert}, resolvedAt, "")

		expectedTitle := "Pulse alert resolved: vm-100"
		if title != expectedTitle {
			t.Fatalf("expected title %q, got %q", expectedTitle, title)
		}

		// Check text body contains expected elements
		if !strings.Contains(textBody, "Resolved at 2024-01-15T11:00:00Z") {
			t.Fatalf("expected resolved timestamp in body, got: %s", textBody)
		}
		if !strings.Contains(textBody, "[WARNING] vm-100") {
			t.Fatalf("expected alert level and resource name in body, got: %s", textBody)
		}
		if !strings.Contains(textBody, "CPU usage exceeded threshold") {
			t.Fatalf("expected message in body, got: %s", textBody)
		}
		if !strings.Contains(textBody, "Started: 2024-01-15T10:30:00Z") {
			t.Fatalf("expected start time in body, got: %s", textBody)
		}
		if !strings.Contains(textBody, "Cleared: 2024-01-15T11:00:00Z") {
			t.Fatalf("expected cleared time in body, got: %s", textBody)
		}
		if !strings.Contains(textBody, "Node: pve1") {
			t.Fatalf("expected node in body, got: %s", textBody)
		}
		if !strings.Contains(textBody, "Last value 95.50 (threshold 80.00)") {
			t.Fatalf("expected threshold/value in body, got: %s", textBody)
		}

		// Check HTML body wraps in pre tag
		if !strings.Contains(htmlBody, "<pre style=") {
			t.Fatalf("expected HTML body to start with <pre> tag, got: %s", htmlBody)
		}
		if !strings.Contains(htmlBody, "</pre>") {
			t.Fatalf("expected HTML body to end with </pre> tag, got: %s", htmlBody)
		}
	})

	t.Run("multiple alerts generate plural title", func(t *testing.T) {
		alert1 := &alerts.Alert{
			ID:           "alert-1",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "vm-100",
		}
		alert2 := &alerts.Alert{
			ID:           "alert-2",
			Level:        alerts.AlertLevelCritical,
			ResourceName: "vm-101",
		}
		alert3 := &alerts.Alert{
			ID:           "alert-3",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "vm-102",
		}

		title, _, textBody := buildResolvedNotificationContent([]*alerts.Alert{alert1, alert2, alert3}, time.Now(), "")

		expectedTitle := "Pulse alerts resolved (3)"
		if title != expectedTitle {
			t.Fatalf("expected title %q, got %q", expectedTitle, title)
		}

		// Verify all alerts are in the body
		if !strings.Contains(textBody, "[WARNING] vm-100") {
			t.Fatalf("expected alert1 in body, got: %s", textBody)
		}
		if !strings.Contains(textBody, "[CRITICAL] vm-101") {
			t.Fatalf("expected alert2 in body, got: %s", textBody)
		}
		if !strings.Contains(textBody, "[WARNING] vm-102") {
			t.Fatalf("expected alert3 in body, got: %s", textBody)
		}
	})

	t.Run("zero resolvedAt uses current time", func(t *testing.T) {
		alert := &alerts.Alert{
			ID:           "test-alert",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "vm-100",
		}

		beforeCall := time.Now()
		_, _, textBody := buildResolvedNotificationContent([]*alerts.Alert{alert}, time.Time{}, "")
		afterCall := time.Now()

		// The resolved timestamp should be between beforeCall and afterCall
		if !strings.Contains(textBody, "Resolved at") {
			t.Fatalf("expected 'Resolved at' in body, got: %s", textBody)
		}

		// Extract the timestamp from the body and verify it's reasonable
		// The format is "Resolved at 2024-01-15T11:00:00Z" or similar
		lines := strings.Split(textBody, "\n")
		if len(lines) == 0 {
			t.Fatalf("expected at least one line in body")
		}
		firstLine := lines[0]
		if !strings.HasPrefix(firstLine, "Resolved at ") {
			t.Fatalf("expected first line to start with 'Resolved at ', got: %s", firstLine)
		}
		timestampStr := strings.TrimPrefix(firstLine, "Resolved at ")
		parsedTime, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			t.Fatalf("failed to parse timestamp %q: %v", timestampStr, err)
		}
		if parsedTime.Before(beforeCall.Add(-time.Second)) || parsedTime.After(afterCall.Add(time.Second)) {
			t.Fatalf("expected timestamp between %v and %v, got %v", beforeCall, afterCall, parsedTime)
		}
	})

	t.Run("public URL is appended when provided", func(t *testing.T) {
		alert := &alerts.Alert{
			ID:           "test-alert",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "vm-100",
		}

		_, _, textBody := buildResolvedNotificationContent([]*alerts.Alert{alert}, time.Now(), "https://pulse.example.com")

		if !strings.Contains(textBody, "Dashboard: https://pulse.example.com") {
			t.Fatalf("expected dashboard URL in body, got: %s", textBody)
		}
	})

	t.Run("public URL is not appended when empty", func(t *testing.T) {
		alert := &alerts.Alert{
			ID:           "test-alert",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "vm-100",
		}

		_, _, textBody := buildResolvedNotificationContent([]*alerts.Alert{alert}, time.Now(), "")

		if strings.Contains(textBody, "Dashboard:") {
			t.Fatalf("expected no dashboard URL in body, got: %s", textBody)
		}
	})

	t.Run("HTML body properly escapes content", func(t *testing.T) {
		alert := &alerts.Alert{
			ID:           "test-alert",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "<script>alert('xss')</script>",
			Message:      "Value > threshold & alert triggered",
		}

		_, htmlBody, _ := buildResolvedNotificationContent([]*alerts.Alert{alert}, time.Now(), "")

		// Check that HTML special characters are escaped
		if strings.Contains(htmlBody, "<script>") {
			t.Fatalf("expected <script> to be escaped in HTML body, got: %s", htmlBody)
		}
		if !strings.Contains(htmlBody, "&lt;script&gt;") {
			t.Fatalf("expected &lt;script&gt; in HTML body, got: %s", htmlBody)
		}
		if strings.Contains(htmlBody, "& alert") {
			t.Fatalf("expected & to be escaped in HTML body, got: %s", htmlBody)
		}
		if !strings.Contains(htmlBody, "&amp; alert") {
			t.Fatalf("expected &amp; in HTML body, got: %s", htmlBody)
		}
		if strings.Contains(htmlBody, "> threshold") {
			t.Fatalf("expected > to be escaped in HTML body, got: %s", htmlBody)
		}
		if !strings.Contains(htmlBody, "&gt; threshold") {
			t.Fatalf("expected &gt; in HTML body, got: %s", htmlBody)
		}
	})

	t.Run("mixed nil and valid alerts filters correctly", func(t *testing.T) {
		alert1 := &alerts.Alert{
			ID:           "alert-1",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "vm-100",
		}
		alert2 := &alerts.Alert{
			ID:           "alert-2",
			Level:        alerts.AlertLevelCritical,
			ResourceName: "vm-101",
		}

		title, _, textBody := buildResolvedNotificationContent([]*alerts.Alert{nil, alert1, nil, alert2, nil}, time.Now(), "")

		expectedTitle := "Pulse alerts resolved (2)"
		if title != expectedTitle {
			t.Fatalf("expected title %q, got %q", expectedTitle, title)
		}

		if !strings.Contains(textBody, "[WARNING] vm-100") {
			t.Fatalf("expected alert1 in body, got: %s", textBody)
		}
		if !strings.Contains(textBody, "[CRITICAL] vm-101") {
			t.Fatalf("expected alert2 in body, got: %s", textBody)
		}
	})

	t.Run("instance not shown when same as node", func(t *testing.T) {
		alert := &alerts.Alert{
			ID:           "test-alert",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "pve1",
			Node:         "pve1",
			Instance:     "pve1", // Same as node
		}

		_, _, textBody := buildResolvedNotificationContent([]*alerts.Alert{alert}, time.Now(), "")

		if !strings.Contains(textBody, "Node: pve1") {
			t.Fatalf("expected node in body, got: %s", textBody)
		}
		// Instance line should not appear when same as node
		if strings.Contains(textBody, "Instance: pve1") {
			t.Fatalf("expected instance to be omitted when same as node, got: %s", textBody)
		}
	})

	t.Run("instance shown when different from node", func(t *testing.T) {
		alert := &alerts.Alert{
			ID:           "test-alert",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "vm-100",
			Node:         "pve1",
			Instance:     "vm-100", // Different from node
		}

		_, _, textBody := buildResolvedNotificationContent([]*alerts.Alert{alert}, time.Now(), "")

		if !strings.Contains(textBody, "Node: pve1") {
			t.Fatalf("expected node in body, got: %s", textBody)
		}
		if !strings.Contains(textBody, "Instance: vm-100") {
			t.Fatalf("expected instance in body when different from node, got: %s", textBody)
		}
	})

	t.Run("threshold and value only shown when non-zero", func(t *testing.T) {
		alertWithValues := &alerts.Alert{
			ID:           "test-alert-1",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "vm-100",
			Threshold:    80,
			Value:        95,
		}
		alertWithoutValues := &alerts.Alert{
			ID:           "test-alert-2",
			Level:        alerts.AlertLevelWarning,
			ResourceName: "vm-101",
			Threshold:    0,
			Value:        0,
		}

		_, _, textBodyWith := buildResolvedNotificationContent([]*alerts.Alert{alertWithValues}, time.Now(), "")
		_, _, textBodyWithout := buildResolvedNotificationContent([]*alerts.Alert{alertWithoutValues}, time.Now(), "")

		if !strings.Contains(textBodyWith, "Last value 95.00 (threshold 80.00)") {
			t.Fatalf("expected threshold/value in body with values, got: %s", textBodyWith)
		}
		if strings.Contains(textBodyWithout, "Last value") {
			t.Fatalf("expected no threshold/value in body without values, got: %s", textBodyWithout)
		}
	})
}

func TestCheckWebhookRateLimit(t *testing.T) {
	t.Run("first request to new URL returns true and creates entry", func(t *testing.T) {
		nm := NewNotificationManager("")

		result := nm.checkWebhookRateLimit("https://example.com/webhook1")
		if !result {
			t.Fatalf("expected first request to return true")
		}

		nm.webhookRateMu.Lock()
		entry, exists := nm.webhookRateLimits["https://example.com/webhook1"]
		nm.webhookRateMu.Unlock()

		if !exists {
			t.Fatalf("expected entry to be created for webhook URL")
		}
		if entry.sentCount != 1 {
			t.Fatalf("expected sentCount to be 1, got %d", entry.sentCount)
		}
	})

	t.Run("multiple requests within window and under limit return true", func(t *testing.T) {
		nm := NewNotificationManager("")

		url := "https://example.com/webhook2"

		// Make multiple requests, all under the limit
		for i := 1; i <= WebhookRateLimitMax-1; i++ {
			result := nm.checkWebhookRateLimit(url)
			if !result {
				t.Fatalf("request %d should return true (under limit)", i)
			}
		}

		nm.webhookRateMu.Lock()
		entry := nm.webhookRateLimits[url]
		count := entry.sentCount
		nm.webhookRateMu.Unlock()

		if count != WebhookRateLimitMax-1 {
			t.Fatalf("expected sentCount to be %d, got %d", WebhookRateLimitMax-1, count)
		}
	})

	t.Run("requests at limit return false", func(t *testing.T) {
		nm := NewNotificationManager("")

		url := "https://example.com/webhook3"

		// Use up all allowed requests
		for i := 1; i <= WebhookRateLimitMax; i++ {
			result := nm.checkWebhookRateLimit(url)
			if !result {
				t.Fatalf("request %d should return true (at or under limit)", i)
			}
		}

		// Next request should be rate limited
		result := nm.checkWebhookRateLimit(url)
		if result {
			t.Fatalf("expected request beyond limit to return false")
		}

		nm.webhookRateMu.Lock()
		entry := nm.webhookRateLimits[url]
		count := entry.sentCount
		nm.webhookRateMu.Unlock()

		// Count should remain at max since rate-limited requests don't increment
		if count != WebhookRateLimitMax {
			t.Fatalf("expected sentCount to remain at %d, got %d", WebhookRateLimitMax, count)
		}
	})

	t.Run("requests after window expiry reset counter and return true", func(t *testing.T) {
		nm := NewNotificationManager("")

		url := "https://example.com/webhook4"

		// Make first request to create entry
		nm.checkWebhookRateLimit(url)

		// Manually set lastSent to a time beyond the window
		nm.webhookRateMu.Lock()
		entry := nm.webhookRateLimits[url]
		entry.lastSent = time.Now().Add(-WebhookRateLimitWindow - time.Second)
		entry.sentCount = WebhookRateLimitMax // Simulate being at the limit
		nm.webhookRateMu.Unlock()

		// Request after window expiry should succeed and reset counter
		result := nm.checkWebhookRateLimit(url)
		if !result {
			t.Fatalf("expected request after window expiry to return true")
		}

		nm.webhookRateMu.Lock()
		count := nm.webhookRateLimits[url].sentCount
		nm.webhookRateMu.Unlock()

		if count != 1 {
			t.Fatalf("expected sentCount to reset to 1, got %d", count)
		}
	})

	t.Run("different URLs have independent rate limits", func(t *testing.T) {
		nm := NewNotificationManager("")

		url1 := "https://example.com/webhook-a"
		url2 := "https://example.com/webhook-b"

		// Exhaust rate limit for url1
		for i := 1; i <= WebhookRateLimitMax; i++ {
			nm.checkWebhookRateLimit(url1)
		}

		// url1 should be rate limited
		if nm.checkWebhookRateLimit(url1) {
			t.Fatalf("expected url1 to be rate limited")
		}

		// url2 should still work (independent limit)
		if !nm.checkWebhookRateLimit(url2) {
			t.Fatalf("expected url2 to not be rate limited")
		}

		nm.webhookRateMu.Lock()
		count1 := nm.webhookRateLimits[url1].sentCount
		count2 := nm.webhookRateLimits[url2].sentCount
		nm.webhookRateMu.Unlock()

		if count1 != WebhookRateLimitMax {
			t.Fatalf("expected url1 sentCount to be %d, got %d", WebhookRateLimitMax, count1)
		}
		if count2 != 1 {
			t.Fatalf("expected url2 sentCount to be 1, got %d", count2)
		}
	})
}
