package notifications

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

func TestNotificationSeverityCooldown(t *testing.T) {
	for _, cooldown := range []time.Duration{0, time.Hour} {
		for _, tc := range []struct {
			from, to alerts.AlertLevel
			send     bool
		}{
			{alerts.AlertLevelInfo, alerts.AlertLevelWarning, true},
			{alerts.AlertLevelWarning, alerts.AlertLevelCritical, true},
			{alerts.AlertLevelCritical, alerts.AlertLevelCritical, false},
			{alerts.AlertLevelCritical, alerts.AlertLevelWarning, false},
			{alerts.AlertLevelWarning, alerts.AlertLevelWarning, false},
			{"", alerts.AlertLevelCritical, false},
			{alerts.AlertLevelWarning, "unknown", false},
		} {
			t.Run(cooldown.String()+"/"+string(tc.from)+"-"+string(tc.to), func(t *testing.T) {
				nm := NewNotificationManagerWithDataDir("", t.TempDir())
				defer nm.Stop()
				nm.cooldown = cooldown
				nm.SetGroupingWindow(3600)
				a := &alerts.Alert{ID: "severity", StartTime: time.Now(), Level: tc.from}
				nm.markAlertsNotified([]*alerts.Alert{a}, time.Now())
				a.Level = tc.to
				nm.SendAlert(a)
				nm.mu.RLock()
				count := len(nm.pendingAlerts)
				nm.mu.RUnlock()
				if tc.send {
					require.Equal(t, 1, count)
				} else {
					require.Zero(t, count)
				}
			})
		}
	}
}

func TestNotificationSeverityReceiptOrder(t *testing.T) {
	nm := NewNotificationManagerWithDataDir("", t.TempDir())
	defer nm.Stop()
	a := &alerts.Alert{ID: "severity", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	nm.markAlertsNotified([]*alerts.Alert{a}, time.Now())
	a.Level = alerts.AlertLevelWarning
	nm.markAlertsNotified([]*alerts.Alert{a}, time.Now())
	require.Equal(t, alerts.AlertLevelCritical, nm.lastNotified[a.ID].level)
	// A new occurrence starts its own severity history.
	a.StartTime = a.StartTime.Add(time.Minute)
	nm.markAlertsNotified([]*alerts.Alert{a}, time.Now())
	require.Equal(t, alerts.AlertLevelWarning, nm.lastNotified[a.ID].level)
}
