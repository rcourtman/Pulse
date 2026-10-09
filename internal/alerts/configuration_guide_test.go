package alerts

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Exercise the actual copied JSON with the delivery owner's policy, without
// opening stores, starting background workers or sending a notification.
func documentedQuietHours(t *testing.T) *Manager {
	t.Helper()
	guide, err := os.ReadFile("../../docs/CONFIGURATION.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(guide), "## 🔔 Alerts (`alerts.json`)")
	if !ok {
		t.Fatal("missing alert configuration section")
	}
	section, _, _ = strings.Cut(section, "\n---\n")
	block := regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```").FindStringSubmatch(section)
	if len(block) != 2 {
		t.Fatal("missing alert configuration JSON example")
	}
	var config AlertConfig
	if err := json.Unmarshal([]byte(block[1]), &config); err != nil {
		t.Fatal(err)
	}
	loc := time.Local
	if zone := config.Schedule.QuietHours.Timezone; zone != "" {
		loc, err = time.LoadLocation(zone)
		if err != nil {
			t.Fatal(err)
		}
	}
	return &Manager{config: config, quietHoursLoc: loc}
}

func TestDocumentedQuietHoursHoldWarningsOnEveryDay(t *testing.T) {
	m := documentedQuietHours(t)
	policy := m.QuietHoursNotificationPolicy()
	alert := &Alert{ID: "guide", Type: "cpu", Level: AlertLevelWarning}
	for day := 5; day <= 11; day++ { // Monday through Sunday, in summer time.
		for _, hour := range []int{1, 22} {
			now := time.Date(2026, time.October, day, hour, 30, 0, 0, m.quietHoursLoc)
			if at := policy(alert, now); at == nil || !at.After(now) {
				t.Errorf("documented %s %02d:30 schedule did not hold warning: %v", now.Weekday(), hour, at)
			}
		}
	}
}

func TestDocumentedQuietHoursClockAndEndMinute(t *testing.T) {
	m := documentedQuietHours(t)
	policy := m.QuietHoursNotificationPolicy()
	alert := &Alert{Type: "cpu", Level: AlertLevelWarning}
	for _, tc := range []struct {
		name string
		now  time.Time
		held bool
	}{
		{"before start in BST", time.Date(2026, 10, 5, 20, 59, 59, 0, time.UTC), false},
		{"start in BST", time.Date(2026, 10, 5, 21, 0, 0, 0, time.UTC), true},
		{"last second of end minute", time.Date(2026, 10, 6, 5, 0, 59, 0, time.UTC), true},
		{"after end minute", time.Date(2026, 10, 6, 5, 1, 0, 0, time.UTC), false},
		{"daytime", time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC), false},
		{"before start in GMT", time.Date(2026, 12, 7, 21, 59, 59, 0, time.UTC), false},
		{"start in GMT", time.Date(2026, 12, 7, 22, 0, 0, 0, time.UTC), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if at := policy(alert, tc.now); (at != nil) != tc.held {
				t.Fatalf("held=%t, want %t; re-evaluation=%v", at != nil, tc.held, at)
			}
		})
	}
	now := time.Date(2026, 10, 5, 21, 30, 0, 0, time.UTC)
	want := time.Date(2026, 10, 6, 5, 1, 0, 0, time.UTC)
	if at := policy(alert, now); at == nil || !at.Equal(want) {
		t.Fatalf("re-evaluation=%v, want %v", at, want)
	}
}

func TestDocumentedQuietHoursKeepCriticalCategoriesEligible(t *testing.T) {
	m := documentedQuietHours(t)
	now := time.Date(2026, 10, 5, 22, 30, 0, 0, m.quietHoursLoc)
	for _, kind := range []string{"cpu", "disk-health", "connectivity"} {
		alert := &Alert{ID: "guide-" + kind, Type: kind, Level: AlertLevelCritical,
			Acknowledged: true, Metadata: map[string]interface{}{"occurrence": "retained"}}
		before := alert.Clone()
		if at := m.QuietHoursNotificationPolicy()(alert, now); at != nil {
			t.Errorf("example held critical %s until %v", kind, at)
		}
		m.config.Schedule.QuietHours.Suppress = QuietHoursSuppression{Performance: true, Storage: true, Offline: true}
		if at := m.QuietHoursNotificationPolicy()(alert, now); at == nil {
			t.Errorf("category opt-in did not hold critical %s", kind)
		}
		m.config.Schedule.QuietHours.Suppress = QuietHoursSuppression{}
		if !reflect.DeepEqual(before, alert) {
			t.Error("notification hold changed alert lifecycle or occurrence")
		}
	}
}

func TestDocumentedQuietHoursDaysAreLocalCalendarDays(t *testing.T) {
	m := documentedQuietHours(t)
	alert := &Alert{Type: "cpu", Level: AlertLevelWarning}
	m.config.Schedule.QuietHours.Days = map[string]bool{"monday": true}
	policy := m.QuietHoursNotificationPolicy()
	for _, tc := range []struct {
		day  int
		hour int
		held bool
	}{{5, 1, true}, {5, 22, true}, {6, 1, false}} {
		now := time.Date(2026, 10, tc.day, tc.hour, 30, 0, 0, m.quietHoursLoc)
		if at := policy(alert, now); (at != nil) != tc.held {
			t.Errorf("%s %02d:30 held=%t, want %t", now.Weekday(), tc.hour, at != nil, tc.held)
		}
	}
	// The original incomplete example omitted days entirely. Enabled alone
	// must not be mistaken for an active notification hold.
	m.config.Schedule.QuietHours.Days = nil
	now := time.Date(2026, 10, 5, 22, 30, 0, 0, m.quietHoursLoc)
	if at := m.QuietHoursNotificationPolicy()(alert, now); at != nil {
		t.Errorf("no selected days invented a hold: %v", at)
	}
}
