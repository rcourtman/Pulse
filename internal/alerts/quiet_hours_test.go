package alerts

import (
	"sync"
	"testing"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
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

	t.Run("public suppression helper defers quiet hours through queue metadata", func(t *testing.T) {
		m := newManagerWithQuietHoursSuppress(QuietHoursSuppression{Offline: true})
		alert := &Alert{ID: "offline-public", Type: "connectivity", Level: AlertLevelCritical}
		if m.ShouldSuppressNotification(alert) {
			t.Fatal("expected public quiet-hours helper to defer rather than drop")
		}
		if alert.Metadata[MetadataQuietHoursSuppressed] != true {
			t.Fatalf("expected quiet-hours replay metadata, got %#v", alert.Metadata)
		}
		if alert.Metadata[MetadataQuietHoursSuppressionReason] != "offline" {
			t.Fatalf("expected offline suppression reason, got %#v", alert.Metadata[MetadataQuietHoursSuppressionReason])
		}
	})

	t.Run("dispatch annotates quiet-hours replay instead of dropping callback", func(t *testing.T) {
		m := newManagerWithQuietHoursSuppress(QuietHoursSuppression{})
		m.config.ActivationState = ActivationActive
		var dispatched *Alert
		m.SetAlertCallback(func(alert *Alert) {
			dispatched = alert
		})

		alert := &Alert{ID: "warn-deferred", Type: "cpu", Level: AlertLevelWarning}
		if !m.dispatchAlert(alert, false) {
			t.Fatal("expected quiet-hours suppressed alert to dispatch for queued replay")
		}
		if dispatched == nil {
			t.Fatal("expected alert callback to receive quiet-hours deferred alert")
		}
		if dispatched.Metadata[MetadataQuietHoursSuppressed] != true {
			t.Fatalf("expected quiet-hours suppression metadata on dispatched alert, got %#v", dispatched.Metadata)
		}
		if dispatched.Metadata[MetadataQuietHoursSuppressionReason] != "non-critical" {
			t.Fatalf("expected non-critical suppression reason, got %#v", dispatched.Metadata[MetadataQuietHoursSuppressionReason])
		}
		rawReplayAt, ok := dispatched.Metadata[MetadataQuietHoursReplayAt].(string)
		if !ok || rawReplayAt == "" {
			t.Fatalf("expected replay timestamp metadata, got %#v", dispatched.Metadata[MetadataQuietHoursReplayAt])
		}
		replayAt, err := time.Parse(time.RFC3339, rawReplayAt)
		if err != nil {
			t.Fatalf("expected RFC3339 replay timestamp, got %q: %v", rawReplayAt, err)
		}
		if !replayAt.After(m.now()) {
			t.Fatalf("expected replay timestamp after quiet-hours suppression time, got %s", replayAt)
		}
	})

	t.Run("resolved notifications are suppressed for acknowledged alerts", func(t *testing.T) {
		m := newManagerWithQuietHoursSuppress(QuietHoursSuppression{})
		alert := &Alert{
			ID:           "resolved-ack",
			Type:         "cpu",
			Level:        AlertLevelCritical,
			Acknowledged: true,
			LastNotified: ptrTime(time.Now().Add(-time.Minute)),
		}
		if !m.ShouldSuppressResolvedNotification(alert) {
			t.Fatal("expected recovery notification to be suppressed for acknowledged alert")
		}
	})

	t.Run("resolved notifications are suppressed when the firing alert was never notified", func(t *testing.T) {
		m := newManagerWithQuietHoursSuppress(QuietHoursSuppression{})
		alert := &Alert{
			ID:    "resolved-unnotified",
			Type:  "cpu",
			Level: AlertLevelCritical,
		}
		if !m.ShouldSuppressResolvedNotification(alert) {
			t.Fatal("expected recovery notification to be suppressed when LastNotified is nil")
		}
	})

	t.Run("resolved notifications are not dropped when firing alert was deferred for quiet hours", func(t *testing.T) {
		m := newManagerWithQuietHoursSuppress(QuietHoursSuppression{})
		alert := &Alert{
			ID:    "resolved-deferred",
			Type:  "cpu",
			Level: AlertLevelWarning,
			Metadata: map[string]interface{}{
				MetadataQuietHoursSuppressed:        true,
				MetadataQuietHoursSuppressionReason: "non-critical",
				MetadataQuietHoursReplayAt:          m.now().Add(time.Hour).UTC().Format(time.RFC3339),
			},
		}
		if m.ShouldSuppressResolvedNotification(alert) {
			t.Fatal("expected recovery notification to enter quiet-hours replay queue")
		}
	})

	t.Run("manual re-dispatch of offline alert keeps quiet-hours replay metadata", func(t *testing.T) {
		m := newManagerWithQuietHoursSuppress(QuietHoursSuppression{Offline: true})
		m.config.ActivationState = ActivationActive

		received := make(chan *Alert, 1)
		m.SetAlertCallback(func(alert *Alert) {
			received <- alert
		})

		state, alert := testNewCanonicalAlert("pbs1", canonicalConnectivitySpecID("pbs1"), string(alertspecs.AlertSpecKindConnectivity), "offline")
		alert.Level = AlertLevelCritical
		alert.Metadata = map[string]interface{}{
			"resourceType": "pbs",
		}

		m.mu.Lock()
		m.setActiveAlertNoLock(state, alert)
		m.mu.Unlock()

		m.NotifyExistingAlert(state)

		select {
		case dispatched := <-received:
			if dispatched.Metadata[MetadataQuietHoursSuppressed] != true {
				t.Fatalf("expected quiet-hours replay metadata, got %#v", dispatched.Metadata)
			}
			if dispatched.Metadata[MetadataQuietHoursSuppressionReason] != "offline" {
				t.Fatalf("expected offline suppression reason, got %#v", dispatched.Metadata[MetadataQuietHoursSuppressionReason])
			}
			rawReplayAt, ok := dispatched.Metadata[MetadataQuietHoursReplayAt].(string)
			if !ok || rawReplayAt == "" {
				t.Fatalf("expected quiet-hours replay timestamp, got %#v", dispatched.Metadata[MetadataQuietHoursReplayAt])
			}
			replayAt, err := time.Parse(time.RFC3339, rawReplayAt)
			if err != nil {
				t.Fatalf("expected RFC3339 replay timestamp, got %q: %v", rawReplayAt, err)
			}
			if !replayAt.After(m.now()) {
				t.Fatalf("expected replay timestamp after quiet-hours dispatch time, got %s", replayAt)
			}
		case <-time.After(time.Second):
			t.Fatal("expected quiet-hours annotated re-dispatch callback")
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

// Local-clock schedules apply to both copies of a repeated minute. Missing
// minutes do not move a configured start or end into some other wall-clock hour.
func TestQuietHoursDaylightSavingClockAndReplay(t *testing.T) {
	for _, tc := range []struct {
		name, zone, start, end, now, replay string
		quiet                               bool
	}{
		{"london-first-repeated-hour", "Europe/London", "01:00", "02:00", "2026-10-25T00:30:00Z", "2026-10-25T02:01:00Z", true},
		{"london-second-repeated-hour", "Europe/London", "01:00", "02:00", "2026-10-25T01:30:00Z", "2026-10-25T02:01:00Z", true},
		{"london-first-repeated-end", "Europe/London", "00:00", "01:30", "2026-10-25T00:15:00Z", "2026-10-25T00:31:00Z", true},
		{"london-second-repeated-end", "Europe/London", "00:00", "01:30", "2026-10-25T01:15:00Z", "2026-10-25T01:31:00Z", true},
		{"london-after-first-end", "Europe/London", "00:00", "01:30", "2026-10-25T00:45:00Z", "", false},
		{"london-after-second-end", "Europe/London", "00:00", "01:30", "2026-10-25T01:45:00Z", "", false},
		{"london-missing-start", "Europe/London", "01:30", "02:30", "2026-03-29T01:15:00Z", "2026-03-29T01:31:00Z", true},
		{"london-missing-end-replay", "Europe/London", "00:00", "01:30", "2026-03-29T00:55:30Z", "2026-03-29T01:00:00Z", true},
		{"london-after-missing-end", "Europe/London", "00:00", "01:30", "2026-03-29T01:15:00Z", "", false},
		{"london-overnight-missing-end", "Europe/London", "22:00", "01:30", "2026-03-28T23:00:00Z", "2026-03-29T01:00:00Z", true},
		{"london-overnight-repeated-end", "Europe/London", "22:00", "01:30", "2026-10-24T23:15:00Z", "2026-10-25T00:31:00Z", true},
		{"london-end-minute-before-jump", "Europe/London", "00:00", "00:59", "2026-03-29T00:59:59Z", "2026-03-29T01:00:00Z", true},
		{"new-york-first-repeated-end", "America/New_York", "00:00", "01:30", "2026-11-01T05:15:00Z", "2026-11-01T05:31:00Z", true},
		{"new-york-second-repeated-end", "America/New_York", "00:00", "01:30", "2026-11-01T06:15:00Z", "2026-11-01T06:31:00Z", true},
		{"new-york-after-first-end", "America/New_York", "00:00", "01:30", "2026-11-01T05:45:00Z", "", false},
		{"new-york-after-second-end", "America/New_York", "00:00", "01:30", "2026-11-01T06:45:00Z", "", false},
		{"new-york-missing-start", "America/New_York", "02:30", "03:30", "2026-03-08T07:15:00Z", "2026-03-08T07:31:00Z", true},
		{"new-york-missing-end", "America/New_York", "00:00", "02:30", "2026-03-08T06:55:00Z", "2026-03-08T07:00:00Z", true},
		{"new-york-after-missing-end", "America/New_York", "00:00", "02:30", "2026-03-08T07:15:00Z", "", false},
		{"lord-howe-first-repeated-half-hour", "Australia/Lord_Howe", "01:30", "02:00", "2026-04-04T14:40:00Z", "2026-04-04T15:31:00Z", true},
		{"lord-howe-second-repeated-half-hour", "Australia/Lord_Howe", "01:30", "02:00", "2026-04-04T15:10:00Z", "2026-04-04T15:31:00Z", true},
		{"lord-howe-missing-start", "Australia/Lord_Howe", "02:00", "02:45", "2026-10-03T15:35:00Z", "2026-10-03T15:46:00Z", true},
		{"lord-howe-after-missing-end", "Australia/Lord_Howe", "00:00", "02:15", "2026-10-03T15:40:00Z", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			m := newTestManager(t)
			m.now = func() time.Time { return now }
			cfg := m.GetConfig()
			cfg.Schedule.QuietHours = QuietHours{Enabled: true, Start: tc.start, End: tc.end, Timezone: tc.zone,
				Days: map[string]bool{"monday": true, "tuesday": true, "wednesday": true, "thursday": true, "friday": true, "saturday": true, "sunday": true}}
			m.UpdateConfig(cfg)
			m.mu.Lock()
			defer m.mu.Unlock()
			if got := m.isInQuietHours(); got != tc.quiet {
				t.Errorf("quiet at %s for %s %s-%s = %v, want %v", now, tc.zone, tc.start, tc.end, got, tc.quiet)
			}
			if tc.replay == "" {
				return
			}
			want, err := time.Parse(time.RFC3339, tc.replay)
			if err != nil {
				t.Fatal(err)
			}
			got := m.quietHoursReplayAt()
			if !got.Equal(want) {
				t.Errorf("replay = %s, want first eligible real minute %s", got, want)
			}
			if !got.After(now) {
				t.Error("replay must be in the future")
			}
			now = want.Add(-time.Nanosecond)
			if !m.isInQuietHours() {
				t.Error("quiet hours ended before the final included minute finished")
			}
			now = want
			if m.isInQuietHours() {
				t.Error("computed replay minute is still quiet")
			}
		})
	}
}

func TestQuietHoursOvernightSelectedCalendarDays(t *testing.T) {
	m := newTestManager(t)
	now := time.Date(2026, 10, 2, 23, 30, 0, 0, time.UTC) // Friday
	m.now = func() time.Time { return now }
	cfg := m.GetConfig()
	cfg.Schedule.QuietHours = QuietHours{Enabled: true, Start: "22:00", End: "06:00", Timezone: "UTC", Days: map[string]bool{"friday": true}}
	m.UpdateConfig(cfg)
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.isInQuietHours() {
		t.Fatal("selected Friday evening must be quiet")
	}
	want := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	if got := m.quietHoursReplayAt(); !got.Equal(want) {
		t.Fatalf("unselected Saturday must end suppression at midnight: got %s, want %s", got, want)
	}
	now = want
	if m.isInQuietHours() {
		t.Fatal("overnight settings must not silently enable an unselected day")
	}
	now = now.AddDate(0, 0, 6).Add(6*time.Hour + 59*time.Second) // Friday's inclusive 06:00 minute
	if !m.isInQuietHours() || !m.quietHoursReplayAt().Equal(now.Truncate(time.Minute).Add(time.Minute)) {
		t.Fatal("selected morning must keep the inclusive end minute")
	}
}

func TestQuietHoursReplaySkipsNonexistentGap(t *testing.T) {
	// This overnight window has a one-minute daily gap, 01:29. London's
	// spring jump skips that entire gap, so the next real exit is Monday.
	now := time.Date(2026, 3, 28, 23, 0, 0, 0, time.UTC)
	m := fixedQuietHoursTestManager(now, QuietHours{Enabled: true, Start: "01:30", End: "01:28", Timezone: "Europe/London",
		Days: map[string]bool{"saturday": true, "sunday": true, "monday": true}})
	want := time.Date(2026, 3, 30, 0, 29, 0, 0, time.UTC)
	if !m.isInQuietHours() || !m.quietHoursReplayAt().Equal(want) {
		t.Fatalf("skipped gap must not release a notification into quiet hours: replay=%s want=%s", m.quietHoursReplayAt(), want)
	}
}

func TestQuietHoursFullDayRetainsDailyReplayBoundary(t *testing.T) {
	for _, tc := range []struct{ start, end, now, replay string }{
		{"00:00", "23:59", "2026-10-01T12:30:00Z", "2026-10-02T00:00:00Z"},
		{"22:00", "21:59", "2026-10-01T23:30:00Z", "2026-10-02T22:00:00Z"},
		{"22:00", "21:59", "2026-10-01T12:30:00Z", "2026-10-01T22:00:00Z"},
	} {
		t.Run(tc.start+"/"+tc.now, func(t *testing.T) {
			now, _ := time.Parse(time.RFC3339, tc.now)
			want, _ := time.Parse(time.RFC3339, tc.replay)
			m := fixedQuietHoursTestManager(now, QuietHours{Enabled: true, Start: tc.start, End: tc.end, Timezone: "UTC", Days: map[string]bool{"thursday": true, "friday": true}})
			if !m.isInQuietHours() || !m.quietHoursReplayAt().Equal(want) {
				t.Fatalf("daily replay changed: %s, want %s", m.quietHoursReplayAt(), want)
			}
		})
	}
}

func TestQuietHoursClockDispatchAndDiagnosis(t *testing.T) {
	for _, tc := range []struct {
		name, now, start, end, replay string
		quiet                         bool
	}{
		{"repeated-hour", "2026-10-25T00:30:00Z", "01:00", "02:00", "2026-10-25T02:01:00Z", true},
		{"skipped-end", "2026-03-29T00:55:00Z", "00:00", "01:30", "2026-03-29T01:00:00Z", true},
		{"after-skipped-end", "2026-03-29T01:15:00Z", "00:00", "01:30", "", false},
	} {
		for _, critical := range []bool{false, true} {
			name := tc.name + "/warning"
			if critical {
				name = tc.name + "/unsuppressed-critical"
			}
			t.Run(name, func(t *testing.T) {
				now, _ := time.Parse(time.RFC3339, tc.now)
				m := newTestManager(t)
				m.now = func() time.Time { return now }
				cfg := m.GetConfig()
				cfg.Enabled = true
				cfg.ActivationState = ActivationActive
				cfg.FlappingEnabled = false
				cfg.Schedule.QuietHours = QuietHours{Enabled: true, Start: tc.start, End: tc.end, Timezone: "Europe/London", Days: map[string]bool{"sunday": true}}
				m.UpdateConfig(cfg)
				_, alert := testNewCanonicalAlert("vm-100", "vm-100-cpu", "vm", "cpu")
				alert.Level = AlertLevelWarning
				if critical {
					alert.Level = AlertLevelCritical
				}
				alert.StartTime = now
				alert.LastSeen = now
				// Old metadata must be cleared when quiet hours no longer apply.
				alert.Metadata = map[string]interface{}{MetadataQuietHoursSuppressed: true, MetadataQuietHoursReplayAt: "2099-01-01T00:00:00Z"}
				m.mu.Lock()
				m.setActiveAlertNoLock(alert.ID, alert)
				m.mu.Unlock()
				deferred := tc.quiet && !critical
				diagnosis, found := m.DiagnoseAlertDelivery(alert.ID)
				if !found {
					t.Fatal("active occurrence unavailable")
				}
				if deferred {
					want, _ := time.Parse(time.RFC3339, tc.replay)
					if diagnosis.Status != AlertDeliveryStatusDeferred || diagnosis.Reason != AlertDeliveryReasonQuietHours || diagnosis.QuietHoursReplayAt == nil || !diagnosis.QuietHoursReplayAt.Equal(want) {
						t.Fatalf("diagnosis disagrees with local-clock policy: %+v", diagnosis)
					}
				} else if diagnosis.Status != AlertDeliveryStatusWouldSend || diagnosis.Reason != AlertDeliveryReasonReady {
					t.Fatalf("eligible alert was diagnosed as held: %+v", diagnosis)
				}
				var dispatched *Alert
				m.SetAlertCallback(func(a *Alert) { dispatched = a })
				m.mu.Lock()
				admitted := m.dispatchAlert(alert, false)
				m.mu.Unlock()
				if !admitted || dispatched == nil {
					t.Fatal("quiet-hours replay must enter the existing delivery pipeline, not be dropped")
				}
				if deferred {
					if dispatched.Metadata[MetadataQuietHoursSuppressed] != true || dispatched.Metadata[MetadataQuietHoursReplayAt] != tc.replay {
						t.Fatalf("pipeline replay timestamp = %#v, want %s", dispatched.Metadata, tc.replay)
					}
				} else if hasQuietHoursNotificationReplay(dispatched) {
					t.Fatalf("eligible alert kept stale deferral metadata: %#v", dispatched.Metadata)
				}
				// Escalation/bypass sends consult this same public helper.
				bypass := cloneAlertForOutput(dispatched)
				if m.ShouldSuppressNotification(bypass) || hasQuietHoursNotificationReplay(bypass) != deferred {
					t.Fatal("public bypass helper did not apply the same clock policy")
				}
			})
		}
	}
}

func TestQuietHoursConcurrentUncachedReaders(t *testing.T) {
	now := time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC)
	m := fixedQuietHoursTestManager(now, QuietHours{Enabled: true, Start: "01:00", End: "02:00", Timezone: "Europe/London", Days: map[string]bool{"sunday": true}})
	start := make(chan struct{})
	var wg sync.WaitGroup
	for worker := 0; worker < 32; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < 25; i++ {
				alert := &Alert{ID: "uncached-reader", Type: "cpu", Level: AlertLevelWarning}
				if m.ShouldSuppressNotification(alert) || alert.Metadata[MetadataQuietHoursReplayAt] != "2026-10-25T02:01:00Z" {
					t.Error("concurrent reader disagrees with quiet-hours replay")
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	if m.quietHoursLoc != nil {
		t.Fatal("read-locked evaluation mutated the configuration-owned location cache")
	}
}
