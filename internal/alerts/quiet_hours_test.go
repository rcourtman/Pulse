package alerts

import (
	"testing"
	"time"
)

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

func TestIsInQuietHours(t *testing.T) {
	// t.Parallel()

	t.Run("disabled returns false", func(t *testing.T) {
		// t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.QuietHours.Enabled = false
		m.mu.Unlock()

		m.mu.RLock()
		result := m.isInQuietHours()
		m.mu.RUnlock()

		if result {
			t.Errorf("isInQuietHours() = true, want false when disabled")
		}
	})

	t.Run("invalid timezone falls back to local", func(t *testing.T) {
		// t.Parallel()
		m := NewManager()
		m.mu.Lock()
		m.config.Schedule.QuietHours = QuietHours{
			Enabled:  true,
			Start:    "00:00",
			End:      "23:59",
			Timezone: "Invalid/Timezone",
			Days: map[string]bool{
				"monday": true, "tuesday": true, "wednesday": true,
				"thursday": true, "friday": true, "saturday": true, "sunday": true,
			},
		}
		m.mu.Unlock()

		// Should not panic, should fall back to local time
		m.mu.RLock()
		result := m.isInQuietHours()
		m.mu.RUnlock()

		// With all days enabled and 00:00-23:59, it should be true
		if !result {
			t.Errorf("isInQuietHours() = false, want true (invalid timezone should fall back to local)")
		}
	})

	t.Run("day not enabled returns false", func(t *testing.T) {
		// t.Parallel()
		m := NewManager()
		now := time.Now()
		currentDay := now.Format("Monday")

		m.mu.Lock()
		m.config.Schedule.QuietHours = QuietHours{
			Enabled:  true,
			Start:    "00:00",
			End:      "23:59",
			Timezone: "UTC",
			Days:     map[string]bool{}, // No days enabled
		}
		m.mu.Unlock()

		m.mu.RLock()
		result := m.isInQuietHours()
		m.mu.RUnlock()

		if result {
			t.Errorf("isInQuietHours() = true, want false (day %s not enabled)", currentDay)
		}
	})

	t.Run("invalid start time returns false", func(t *testing.T) {
		// t.Parallel()
		m := NewManager()

		m.mu.Lock()
		m.config.Schedule.QuietHours = QuietHours{
			Enabled:  true,
			Start:    "invalid",
			End:      "23:59",
			Timezone: "UTC",
			Days: map[string]bool{
				"monday": true, "tuesday": true, "wednesday": true,
				"thursday": true, "friday": true, "saturday": true, "sunday": true,
			},
		}
		m.mu.Unlock()

		m.mu.RLock()
		result := m.isInQuietHours()
		m.mu.RUnlock()

		if result {
			t.Errorf("isInQuietHours() = true, want false (invalid start time)")
		}
	})

	t.Run("invalid end time returns false", func(t *testing.T) {
		// t.Parallel()
		m := NewManager()

		m.mu.Lock()
		m.config.Schedule.QuietHours = QuietHours{
			Enabled:  true,
			Start:    "00:00",
			End:      "invalid",
			Timezone: "UTC",
			Days: map[string]bool{
				"monday": true, "tuesday": true, "wednesday": true,
				"thursday": true, "friday": true, "saturday": true, "sunday": true,
			},
		}
		m.mu.Unlock()

		m.mu.RLock()
		result := m.isInQuietHours()
		m.mu.RUnlock()

		if result {
			t.Errorf("isInQuietHours() = true, want false (invalid end time)")
		}
	})

	t.Run("overnight quiet hours spanning midnight", func(t *testing.T) {
		// t.Parallel()
		m := NewManager()

		// Set up overnight quiet hours (22:00 to 06:00)
		m.mu.Lock()
		m.config.Schedule.QuietHours = QuietHours{
			Enabled:  true,
			Start:    "22:00",
			End:      "06:00", // End before start = overnight
			Timezone: "UTC",
			Days: map[string]bool{
				"monday": true, "tuesday": true, "wednesday": true,
				"thursday": true, "friday": true, "saturday": true, "sunday": true,
			},
		}
		m.mu.Unlock()

		// This tests the overnight branch - exact result depends on current time
		// but should not panic
		m.mu.RLock()
		_ = m.isInQuietHours()
		m.mu.RUnlock()
	})

	t.Run("normal daytime quiet hours", func(t *testing.T) {
		// t.Parallel()
		m := NewManager()

		// Set up daytime quiet hours (09:00 to 17:00)
		m.mu.Lock()
		m.config.Schedule.QuietHours = QuietHours{
			Enabled:  true,
			Start:    "09:00",
			End:      "17:00",
			Timezone: "UTC",
			Days: map[string]bool{
				"monday": true, "tuesday": true, "wednesday": true,
				"thursday": true, "friday": true, "saturday": true, "sunday": true,
			},
		}
		m.mu.Unlock()

		// Should not panic, result depends on current time
		m.mu.RLock()
		_ = m.isInQuietHours()
		m.mu.RUnlock()
	})

	t.Run("outside quiet hours window", func(t *testing.T) {
		// t.Parallel()
		m := NewManager()

		// Use a time window that's definitely not now (narrow window in far past/future time)
		// This creates a window from 03:00-03:01 which is unlikely to match current time
		m.mu.Lock()
		m.config.Schedule.QuietHours = QuietHours{
			Enabled:  true,
			Start:    "03:00",
			End:      "03:01", // 1 minute window
			Timezone: "UTC",
			Days: map[string]bool{
				"monday": true, "tuesday": true, "wednesday": true,
				"thursday": true, "friday": true, "saturday": true, "sunday": true,
			},
		}
		m.mu.Unlock()

		m.mu.RLock()
		result := m.isInQuietHours()
		m.mu.RUnlock()

		// Very narrow window, likely false but depends on exact test execution time
		// Main purpose is to exercise the "outside window" return false path
		_ = result
	})
}
