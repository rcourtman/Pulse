package notifications

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func flushPending(n *NotificationManager) {
	n.mu.Lock()
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
	}

	normalized := NormalizeAppriseConfig(original)

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

	// When all targets removed, enabled should reset to false
	empty := NormalizeAppriseConfig(AppriseConfig{Enabled: true})
	if empty.Enabled {
		t.Fatalf("expected enabled to be false when no targets configured")
	}
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
