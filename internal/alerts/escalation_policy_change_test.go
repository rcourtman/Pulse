package alerts

import (
	"testing"
	"time"
)

func TestEscalationPolicyChangeBeforeFormerDeadline(t *testing.T) {
	for _, action := range []string{"disable", "resolve"} {
		t.Run(action, func(t *testing.T) {
			m := newTestManager(t)
			now := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)
			m.now = func() time.Time { return now }
			cfg := m.GetConfig()
			cfg.Enabled = true
			cfg.ActivationState = ActivationActive
			cfg.Schedule.Escalation = EscalationConfig{Enabled: true, Levels: []EscalationLevel{{After: 180, Notify: "email"}}}
			m.UpdateConfig(cfg)
			a := &Alert{ID: "short-lived", StartTime: now, Level: AlertLevelWarning}
			m.mu.Lock()
			m.setActiveAlertNoLock(a.ID, a)
			m.mu.Unlock()
			now = now.Add(time.Minute)
			m.checkEscalations()
			if action == "disable" {
				cfg.Schedule.Escalation.Enabled = false
				m.UpdateConfig(cfg)
			} else if !m.ClearAlert(a.ID) {
				t.Fatal("clear failed")
			}
			now = now.Add(180 * time.Minute)
			m.checkEscalations()
			// checkEscalations records admission synchronously before spawning delivery.
			if a.LastEscalation != 0 || len(a.EscalationTimes) != 0 {
				t.Fatalf("obsolete escalation admitted: %+v", a)
			}
		})
	}
}

func TestPrepareEscalationNotificationUsesCurrentOccurrenceAndDetachedTarget(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()
	m.mu.Lock()
	m.config.Enabled = true
	m.config.ActivationState = ActivationActive
	m.config.Schedule.Escalation = EscalationConfig{Enabled: true, Levels: []EscalationLevel{{After: 180, Notify: "email", DestinationIDs: []string{"email"}}}}
	a := &Alert{ID: "current-escalation", Type: "cpu", ResourceID: "node/test", StartTime: now, Level: AlertLevelWarning, Value: 90}
	m.setActiveAlertNoLock(a.ID, a)
	snapshot := cloneAlertForOutput(a)
	a.Value = 95
	m.config.Schedule.Escalation.Levels[0].Notify = "webhook"
	m.config.Schedule.Escalation.Levels[0].DestinationIDs = []string{"webhook:ops"}
	m.mu.Unlock()
	current, target, ok := m.PrepareEscalationNotification(snapshot, 1)
	if !ok || current.Value != 95 || target.Notify != "webhook" || target.DestinationIDs[0] != "webhook:ops" {
		t.Fatalf("current=%+v target=%+v ok=%v", current, target, ok)
	}
	current.Value = 0
	target.DestinationIDs[0] = "mutated"
	current, target, ok = m.PrepareEscalationNotification(snapshot, 1)
	if !ok || current.Value != 95 || target.DestinationIDs[0] != "webhook:ops" {
		t.Fatal("returned data aliases manager state")
	}
	for _, level := range []int{0, -1, 2} {
		if _, _, ok := m.PrepareEscalationNotification(snapshot, level); ok {
			t.Fatalf("invalid level %d admitted", level)
		}
	}
	if _, _, ok := m.PrepareEscalationNotification(nil, 1); ok {
		t.Fatal("nil snapshot admitted")
	}
	if err := m.SnoozeAlert(snapshot.ID, "test", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := m.PrepareEscalationNotification(snapshot, 1); ok {
		t.Fatal("snoozed occurrence admitted")
	}

}
