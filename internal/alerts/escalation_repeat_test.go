package alerts

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
)

func TestCriticalEscalationRepeatsAtFinalLevelUntilAcknowledged(t *testing.T) {
	m := newTestManager(t)
	store, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory event log: %v", err)
	}
	m.SetEventLog(store)
	now := time.Date(2026, time.August, 27, 18, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }

	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = ActivationActive
	cfg.Schedule.Escalation = EscalationConfig{
		Enabled:        true,
		RepeatCritical: true,
		RepeatEvery:    15,
		Levels: []EscalationLevel{{
			After:          5,
			Notify:         "webhook",
			DestinationIDs: []string{"webhook:pager"},
		}},
	}
	m.UpdateConfig(cfg)

	var mu sync.Mutex
	callbacks := 0
	m.SetEscalateCallback(func(*Alert, int) {
		mu.Lock()
		callbacks++
		mu.Unlock()
	})

	alert := &Alert{
		ID:              "critical-repeat",
		Level:           AlertLevelCritical,
		ResourceID:      "node-1",
		CanonicalSpecID: "connectivity",
		StartTime:       now.Add(-6 * time.Minute),
		LastSeen:        now,
	}
	m.mu.Lock()
	m.setActiveAlertNoLock(alert.ID, alert)
	m.mu.Unlock()

	m.checkEscalations()
	waitForEscalationCallbacks(t, &mu, &callbacks, 1)

	now = now.Add(14 * time.Minute)
	m.checkEscalations()
	waitForEscalationCallbacks(t, &mu, &callbacks, 1)

	now = now.Add(2 * time.Minute)
	m.checkEscalations()
	waitForEscalationCallbacks(t, &mu, &callbacks, 2)

	m.mu.RLock()
	active := testRequireActiveAlert(t, m, "critical-repeat")
	if active.LastEscalation != 1 || len(active.EscalationTimes) != 2 {
		t.Fatalf("repeat lifecycle = %+v", active)
	}
	canonicalAlertID := exportedAlertID(active, active.ID)
	m.mu.RUnlock()

	if err := m.AcknowledgeAlert(canonicalAlertID, "operator"); err != nil {
		t.Fatalf("AcknowledgeAlert: %v", err)
	}
	now = now.Add(20 * time.Minute)
	m.checkEscalations()
	waitForEscalationCallbacks(t, &mu, &callbacks, 2)

	events, err := m.AlertEvents(eventlog.Filter{Types: []string{eventlog.TypeEscalated}, Limit: 10})
	if err != nil {
		t.Fatalf("list escalation events: %v", err)
	}
	foundRepeat := false
	for _, event := range events {
		if event.Details["repeat"] == "true" {
			foundRepeat = true
		}
	}
	if !foundRepeat {
		t.Fatalf("repeat escalation was not distinguished in event log: %+v", events)
	}
}

func TestEscalationRepeatSkipsWarningAlerts(t *testing.T) {
	m := newTestManager(t)
	now := time.Date(2026, time.August, 27, 18, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	m.mu.Lock()
	m.config.Enabled = true
	m.config.ActivationState = ActivationActive
	m.config.Schedule.Escalation = EscalationConfig{
		Enabled: true, RepeatCritical: true, RepeatEvery: 5,
		Levels: []EscalationLevel{{After: 5, Notify: "email"}},
	}
	m.activeAlerts["warning"] = &Alert{
		ID: "warning", Level: AlertLevelWarning, StartTime: now.Add(-30 * time.Minute),
		LastEscalation: 1, EscalationTimes: []time.Time{now.Add(-10 * time.Minute)},
	}
	m.mu.Unlock()

	var callbacks atomic.Int32
	m.SetEscalateCallback(func(*Alert, int) { callbacks.Add(1) })
	m.checkEscalations()
	time.Sleep(20 * time.Millisecond)
	if got := callbacks.Load(); got != 0 {
		t.Fatalf("warning alert repeated %d times", got)
	}
}

func waitForEscalationCallbacks(t *testing.T, mu *sync.Mutex, callbacks *int, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		got := *callbacks
		mu.Unlock()
		if got == want {
			return
		}
		if got > want || time.Now().After(deadline) {
			t.Fatalf("escalation callbacks = %d, want %d", got, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
