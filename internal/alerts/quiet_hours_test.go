package alerts

import (
	"testing"
	"time"
)

func fixedQuietHoursTestManager(now time.Time, quietHours QuietHours) *Manager {
	m := NewManager()
	m.now = func() time.Time { return now }
	m.config.Schedule.QuietHours = quietHours
	return m
}

func newManagerWithQuietHoursSuppress(s QuietHoursSuppression) *Manager {
	return fixedQuietHoursTestManager(
		time.Date(2026, time.April, 12, 12, 0, 0, 0, time.UTC),
		QuietHours{
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
		},
	)
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

func TestIsInQuietHours(t *testing.T) {
	// t.Parallel()

	t.Run("disabled returns false", func(t *testing.T) {
		m := fixedQuietHoursTestManager(
			time.Date(2026, time.April, 12, 12, 0, 0, 0, time.UTC),
			QuietHours{Enabled: false},
		)
		result := m.isInQuietHours()

		if result {
			t.Errorf("isInQuietHours() = true, want false when disabled")
		}
	})

	t.Run("invalid timezone falls back to local", func(t *testing.T) {
		m := fixedQuietHoursTestManager(
			time.Date(2026, time.April, 12, 12, 0, 0, 0, time.Local),
			QuietHours{
				Enabled:  true,
				Start:    "00:00",
				End:      "23:59",
				Timezone: "Invalid/Timezone",
				Days: map[string]bool{
					"monday": true, "tuesday": true, "wednesday": true,
					"thursday": true, "friday": true, "saturday": true, "sunday": true,
				},
			},
		)

		result := m.isInQuietHours()
		if !result {
			t.Errorf("isInQuietHours() = false, want true (invalid timezone should fall back to local)")
		}
	})

	t.Run("day not enabled returns false", func(t *testing.T) {
		now := time.Date(2026, time.April, 13, 12, 0, 0, 0, time.UTC)
		currentDay := now.Format("Monday")
		m := fixedQuietHoursTestManager(now, QuietHours{
			Enabled:  true,
			Start:    "00:00",
			End:      "23:59",
			Timezone: "UTC",
			Days:     map[string]bool{}, // No days enabled
		})
		result := m.isInQuietHours()

		if result {
			t.Errorf("isInQuietHours() = true, want false (day %s not enabled)", currentDay)
		}
	})

	t.Run("invalid start time returns false", func(t *testing.T) {
		m := fixedQuietHoursTestManager(
			time.Date(2026, time.April, 12, 12, 0, 0, 0, time.UTC),
			QuietHours{
				Enabled:  true,
				Start:    "invalid",
				End:      "23:59",
				Timezone: "UTC",
				Days: map[string]bool{
					"monday": true, "tuesday": true, "wednesday": true,
					"thursday": true, "friday": true, "saturday": true, "sunday": true,
				},
			},
		)
		result := m.isInQuietHours()

		if result {
			t.Errorf("isInQuietHours() = true, want false (invalid start time)")
		}
	})

	t.Run("invalid end time returns false", func(t *testing.T) {
		m := fixedQuietHoursTestManager(
			time.Date(2026, time.April, 12, 12, 0, 0, 0, time.UTC),
			QuietHours{
				Enabled:  true,
				Start:    "00:00",
				End:      "invalid",
				Timezone: "UTC",
				Days: map[string]bool{
					"monday": true, "tuesday": true, "wednesday": true,
					"thursday": true, "friday": true, "saturday": true, "sunday": true,
				},
			},
		)
		result := m.isInQuietHours()

		if result {
			t.Errorf("isInQuietHours() = true, want false (invalid end time)")
		}
	})

	t.Run("overnight quiet hours spanning midnight", func(t *testing.T) {
		m := fixedQuietHoursTestManager(
			time.Date(2026, time.April, 12, 23, 30, 0, 0, time.UTC),
			QuietHours{
				Enabled:  true,
				Start:    "22:00",
				End:      "06:00", // End before start = overnight
				Timezone: "UTC",
				Days: map[string]bool{
					"monday": true, "tuesday": true, "wednesday": true,
					"thursday": true, "friday": true, "saturday": true, "sunday": true,
				},
			},
		)
		if !m.isInQuietHours() {
			t.Errorf("isInQuietHours() = false, want true for overnight quiet hours")
		}
	})

	t.Run("normal daytime quiet hours", func(t *testing.T) {
		m := fixedQuietHoursTestManager(
			time.Date(2026, time.April, 12, 10, 0, 0, 0, time.UTC),
			QuietHours{
				Enabled:  true,
				Start:    "09:00",
				End:      "17:00",
				Timezone: "UTC",
				Days: map[string]bool{
					"monday": true, "tuesday": true, "wednesday": true,
					"thursday": true, "friday": true, "saturday": true, "sunday": true,
				},
			},
		)
		if !m.isInQuietHours() {
			t.Errorf("isInQuietHours() = false, want true for daytime quiet hours")
		}
	})

	t.Run("outside quiet hours window", func(t *testing.T) {
		m := fixedQuietHoursTestManager(
			time.Date(2026, time.April, 12, 4, 0, 0, 0, time.UTC),
			QuietHours{
				Enabled:  true,
				Start:    "03:00",
				End:      "03:01",
				Timezone: "UTC",
				Days: map[string]bool{
					"monday": true, "tuesday": true, "wednesday": true,
					"thursday": true, "friday": true, "saturday": true, "sunday": true,
				},
			},
		)
		result := m.isInQuietHours()
		if result {
			t.Errorf("isInQuietHours() = true, want false outside the configured window")
		}
	})

	t.Run("end minute remains inclusive", func(t *testing.T) {
		m := fixedQuietHoursTestManager(
			time.Date(2026, time.April, 12, 23, 59, 31, 0, time.UTC),
			QuietHours{
				Enabled:  true,
				Start:    "00:00",
				End:      "23:59",
				Timezone: "UTC",
				Days: map[string]bool{
					"monday": true, "tuesday": true, "wednesday": true,
					"thursday": true, "friday": true, "saturday": true, "sunday": true,
				},
			},
		)
		if !m.isInQuietHours() {
			t.Errorf("isInQuietHours() = false, want true through the configured end minute")
		}
	})
}
