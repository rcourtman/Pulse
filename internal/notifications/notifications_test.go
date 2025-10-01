package notifications

import (
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
