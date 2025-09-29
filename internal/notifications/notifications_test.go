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
