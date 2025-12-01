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
