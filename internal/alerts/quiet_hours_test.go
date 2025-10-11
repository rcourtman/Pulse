package alerts

import "testing"

func newManagerWithQuietHoursSuppress(s QuietHoursSuppression) *Manager {
	m := NewManager()
	m.config.Schedule.QuietHours = QuietHours{
		Enabled:  true,
		Start:    "00:00",
		End:      "23:59",
		Timezone: "UTC",
		Days: map[string]bool{
			"monday":    true,
			"tuesday":   true,
			"wednesday": true,
			"thursday":  true,
			"friday":    true,
			"saturday":  true,
			"sunday":    true,
		},
		Suppress: s,
	}
	return m
}

func TestShouldSuppressNotificationQuietHours(t *testing.T) {
	t.Run("non-critical alerts suppressed by default", func(t *testing.T) {
		m := newManagerWithQuietHoursSuppress(QuietHoursSuppression{})
		alert := &Alert{ID: "warn", Type: "cpu", Level: AlertLevelWarning}
		suppressed, reason := m.shouldSuppressNotification(alert)
		if !suppressed || reason != "non-critical" {
			t.Fatalf("expected non-critical alert to be suppressed, got suppressed=%t reason=%q", suppressed, reason)
		}
	})

	t.Run("critical offline alerts suppressed when configured", func(t *testing.T) {
		m := newManagerWithQuietHoursSuppress(QuietHoursSuppression{Offline: true})
		alert := &Alert{ID: "offline", Type: "connectivity", Level: AlertLevelCritical}
		suppressed, reason := m.shouldSuppressNotification(alert)
		if !suppressed || reason != "offline" {
			t.Fatalf("expected offline alert suppression, got suppressed=%t reason=%q", suppressed, reason)
		}
	})

	t.Run("critical performance alerts require opt-in", func(t *testing.T) {
		m := newManagerWithQuietHoursSuppress(QuietHoursSuppression{})
		alert := &Alert{ID: "perf", Type: "cpu", Level: AlertLevelCritical}
		suppressed, reason := m.shouldSuppressNotification(alert)
		if suppressed {
			t.Fatalf("expected performance alert not to be suppressed, got reason=%q", reason)
		}
	})

	t.Run("critical performance alerts suppressed when enabled", func(t *testing.T) {
		m := newManagerWithQuietHoursSuppress(QuietHoursSuppression{Performance: true})
		alert := &Alert{ID: "perf-enabled", Type: "cpu", Level: AlertLevelCritical}
		suppressed, reason := m.shouldSuppressNotification(alert)
		if !suppressed || reason != "performance" {
			t.Fatalf("expected performance alert suppression, got suppressed=%t reason=%q", suppressed, reason)
		}
	})

	t.Run("critical storage alerts suppressed when enabled", func(t *testing.T) {
		m := newManagerWithQuietHoursSuppress(QuietHoursSuppression{Storage: true})
		alert := &Alert{ID: "storage", Type: "usage", Level: AlertLevelCritical}
		suppressed, reason := m.shouldSuppressNotification(alert)
		if !suppressed || reason != "storage" {
			t.Fatalf("expected storage alert suppression, got suppressed=%t reason=%q", suppressed, reason)
		}
	})
}
