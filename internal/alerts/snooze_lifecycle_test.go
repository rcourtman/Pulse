package alerts

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

func TestAlertSnoozeIsDurableAndEnforcedByDeliveryPolicy(t *testing.T) {
	m := newTestManager(t)
	store, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	m.SetEventLog(store)

	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = ActivationActive
	m.UpdateConfig(cfg)

	alert := &Alert{
		ID: "cpu:vm/100", CanonicalState: "cpu:vm/100", CanonicalSpecID: "cpu",
		Type: "cpu", Level: AlertLevelCritical, ResourceID: "vm/100", ResourceName: "vm-100",
		StartTime: now.Add(-time.Hour), LastSeen: now, Message: "CPU is critical.",
	}
	m.mu.Lock()
	m.setActiveAlertNoLock(alert.ID, alert)
	m.mu.Unlock()

	deliveries := 0
	m.SetAlertCallback(func(*Alert) { deliveries++ })
	until := now.Add(2 * time.Hour)
	if err := m.SnoozeAlert(alert.ID, "operator@example.com", until); err != nil {
		t.Fatalf("SnoozeAlert() error = %v", err)
	}
	m.NotifyExistingAlert(alert.ID)
	if deliveries != 0 {
		t.Fatalf("snoozed alert delivered %d notifications, want 0", deliveries)
	}

	diagnosis, ok := m.DiagnoseAlertDelivery(alert.ID)
	if !ok || diagnosis.Reason != AlertDeliveryReasonSnoozed {
		t.Fatalf("diagnosis = %+v, exists=%v", diagnosis, ok)
	}
	if diagnosis.SuppressedUntil == nil || !diagnosis.SuppressedUntil.Equal(until) {
		t.Fatalf("suppressedUntil = %v, want %v", diagnosis.SuppressedUntil, until)
	}

	snapshots, err := store.LoadActiveState()
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("LoadActiveState() = %d snapshots, %v", len(snapshots), err)
	}
	var durable Alert
	if err := json.Unmarshal(snapshots[0].Snapshot, &durable); err != nil {
		t.Fatal(err)
	}
	if durable.OperationalRecord == nil || durable.OperationalRecord.State != operationaltrust.OperationalSuppressed {
		t.Fatalf("durable operational record = %+v", durable.OperationalRecord)
	}

	if err := m.UnsnoozeAlert(alert.ID, "operator@example.com"); err != nil {
		t.Fatal(err)
	}
	resumed := activeAlert(t, m, alert.ID)
	if resumed.OperationalRecord.State != operationaltrust.OperationalOpen || resumed.OperationalRecord.Suppression != nil {
		t.Fatalf("resumed record = %+v", resumed.OperationalRecord)
	}
	events, err := m.AlertEvents(eventlog.Filter{AlertID: alert.ID})
	if err != nil {
		t.Fatal(err)
	}
	seenSnoozed, seenUnsnoozed := false, false
	for _, event := range events {
		seenSnoozed = seenSnoozed || event.Type == eventlog.TypeSnoozed
		seenUnsnoozed = seenUnsnoozed || event.Type == eventlog.TypeUnsnoozed
	}
	if !seenSnoozed || !seenUnsnoozed {
		t.Fatalf("snooze lifecycle events = %+v", events)
	}
}

func TestUnsnoozeRestoresAcknowledgedState(t *testing.T) {
	m := newTestManager(t)
	now := time.Now().UTC()
	alert := &Alert{ID: "memory:vm/100", CanonicalState: "memory:vm/100", CanonicalSpecID: "memory", Type: "memory", Level: AlertLevelWarning, ResourceID: "vm/100", ResourceName: "vm-100", StartTime: now, LastSeen: now}
	m.mu.Lock()
	m.setActiveAlertNoLock(alert.ID, alert)
	m.mu.Unlock()
	if err := m.AcknowledgeAlert(alert.ID, "operator@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := m.SnoozeAlert(alert.ID, "operator@example.com", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := m.UnsnoozeAlert(alert.ID, "operator@example.com"); err != nil {
		t.Fatal(err)
	}
	got := activeAlert(t, m, alert.ID)
	if got.OperationalRecord.State != operationaltrust.OperationalAcknowledged || !got.Acknowledged {
		t.Fatalf("restored state = %q, acknowledged=%v", got.OperationalRecord.State, got.Acknowledged)
	}
}

func TestSnoozeExpiryResumesEscalationWithoutReplayingMissedLevels(t *testing.T) {
	m := newTestManager(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = ActivationActive
	cfg.Schedule.Escalation.Enabled = true
	cfg.Schedule.Escalation.Levels = []EscalationLevel{{After: 1, Notify: "all"}}
	m.UpdateConfig(cfg)
	alert := &Alert{ID: "disk:vm/100", CanonicalState: "disk:vm/100", CanonicalSpecID: "disk", Type: "disk", Level: AlertLevelCritical, ResourceID: "vm/100", ResourceName: "vm-100", StartTime: now.Add(-time.Hour), LastSeen: now}
	m.mu.Lock()
	m.setActiveAlertNoLock(alert.ID, alert)
	m.mu.Unlock()

	escalations := make(chan struct{}, 2)
	m.SetEscalateCallback(func(*Alert, int) { escalations <- struct{}{} })
	until := now.Add(2 * time.Hour)
	if err := m.SnoozeAlert(alert.ID, "operator@example.com", until); err != nil {
		t.Fatal(err)
	}
	now = until.Add(time.Second)
	m.checkEscalations()
	select {
	case <-escalations:
		t.Fatal("expiry replayed a missed escalation level")
	case <-time.After(100 * time.Millisecond):
	}
	if activeAlert(t, m, alert.ID).OperationalRecord.State != operationaltrust.OperationalOpen {
		t.Fatal("expired snooze did not reopen the alert")
	}

	now = now.Add(2 * time.Minute)
	m.checkEscalations()
	select {
	case <-escalations:
	case <-time.After(time.Second):
		t.Fatal("post-resume escalation did not run")
	}
}
