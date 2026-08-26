package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
)

func newEventLogManager(t *testing.T) *Manager {
	t.Helper()

	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)

	store, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory event log: %v", err)
	}
	manager.SetEventLog(store)

	cfg := manager.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = ActivationActive
	cfg.Schedule.Cooldown = 5
	manager.UpdateConfig(cfg)

	return manager
}

func addEventLogAlert(manager *Manager, alert *Alert) *Alert {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.activeAlerts[alert.ID] = alert
	return alert
}

func queryAlertEvents(t *testing.T, manager *Manager, filter eventlog.Filter) []eventlog.Event {
	t.Helper()
	events, err := manager.AlertEvents(filter)
	if err != nil {
		t.Fatalf("AlertEvents: %v", err)
	}
	return events
}

func TestEventLogRecordsAcknowledgeAndUnacknowledge(t *testing.T) {
	manager := newEventLogManager(t)
	addEventLogAlert(manager, &Alert{
		ID:           "ack-alert",
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   "node-1",
		ResourceName: "node-1",
		StartTime:    time.Now().Add(-10 * time.Minute),
	})

	if err := manager.AcknowledgeAlert("ack-alert", "richard"); err != nil {
		t.Fatalf("AcknowledgeAlert: %v", err)
	}
	if err := manager.UnacknowledgeAlert("ack-alert"); err != nil {
		t.Fatalf("UnacknowledgeAlert: %v", err)
	}

	acked := queryAlertEvents(t, manager, eventlog.Filter{Types: []string{eventlog.TypeAcknowledged}})
	if len(acked) != 1 {
		t.Fatalf("acknowledged events = %d, want 1", len(acked))
	}
	if acked[0].AlertID != "ack-alert" || acked[0].Details["user"] != "richard" {
		t.Fatalf("acknowledged event = %+v, want ack-alert by richard", acked[0])
	}
	unacked := queryAlertEvents(t, manager, eventlog.Filter{Types: []string{eventlog.TypeUnacknowledged}})
	if len(unacked) != 1 {
		t.Fatalf("unacknowledged events = %d, want 1", len(unacked))
	}
}

func TestEventLogRecordsResolvedOnClear(t *testing.T) {
	manager := newEventLogManager(t)
	addEventLogAlert(manager, &Alert{
		ID:           "resolve-alert",
		Type:         "memory",
		Level:        AlertLevelCritical,
		ResourceID:   "node-2",
		ResourceName: "node-2",
		StartTime:    time.Now().Add(-10 * time.Minute),
	})

	if !manager.ClearAlert("resolve-alert") {
		t.Fatal("ClearAlert returned false")
	}

	events := queryAlertEvents(t, manager, eventlog.Filter{
		AlertID: "resolve-alert",
		Types:   []string{eventlog.TypeResolved},
	})
	if len(events) != 1 {
		t.Fatalf("resolved events = %d, want 1", len(events))
	}
	if events[0].ResourceName != "node-2" || events[0].Level != string(AlertLevelCritical) {
		t.Fatalf("resolved event = %+v, want node-2/critical context", events[0])
	}
}

func TestEventLogRecordsDispatchDecisions(t *testing.T) {
	manager := newEventLogManager(t)
	manager.SetAlertCallback(func(alert *Alert) {})

	dispatch := func(alert *Alert) bool {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		return manager.dispatchAlert(alert, false)
	}

	sent := addEventLogAlert(manager, &Alert{
		ID:           "dispatch-alert",
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   "node-1",
		ResourceName: "node-1",
		StartTime:    time.Now().Add(-10 * time.Minute),
	})
	if !dispatch(sent) {
		t.Fatal("dispatchAlert returned false for eligible alert")
	}

	suppressedAlert := addEventLogAlert(manager, &Alert{
		ID:           "suppressed-alert",
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   "node-1",
		ResourceName: "node-1",
		StartTime:    time.Now().Add(-10 * time.Minute),
		Acknowledged: true,
	})
	if dispatch(suppressedAlert) {
		t.Fatal("dispatchAlert returned true for acknowledged alert")
	}

	dispatched := queryAlertEvents(t, manager, eventlog.Filter{Types: []string{eventlog.TypeNotificationDispatched}})
	if len(dispatched) != 1 || dispatched[0].AlertID != "dispatch-alert" {
		t.Fatalf("dispatched events = %+v, want one for dispatch-alert", dispatched)
	}

	suppressed := queryAlertEvents(t, manager, eventlog.Filter{Types: []string{eventlog.TypeNotificationSuppressed}})
	if len(suppressed) != 1 || suppressed[0].AlertID != "suppressed-alert" {
		t.Fatalf("suppressed events = %+v, want one for suppressed-alert", suppressed)
	}
	if suppressed[0].Reason != AlertDeliveryReasonAcknowledged {
		t.Fatalf("suppressed reason = %q, want %q", suppressed[0].Reason, AlertDeliveryReasonAcknowledged)
	}
}

func TestEventLogRecordsInactiveActivationSuppression(t *testing.T) {
	manager := newEventLogManager(t)
	manager.SetAlertCallback(func(alert *Alert) {})

	cfg := manager.GetConfig()
	cfg.ActivationState = ActivationPending
	manager.UpdateConfig(cfg)

	alert := addEventLogAlert(manager, &Alert{
		ID:           "pending-alert",
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   "node-1",
		ResourceName: "node-1",
		StartTime:    time.Now().Add(-10 * time.Minute),
	})

	manager.mu.Lock()
	dispatched := manager.dispatchAlert(alert, false)
	manager.mu.Unlock()
	if dispatched {
		t.Fatal("dispatchAlert returned true while activation is pending")
	}

	events := queryAlertEvents(t, manager, eventlog.Filter{Types: []string{eventlog.TypeNotificationSuppressed}})
	if len(events) != 1 {
		t.Fatalf("suppressed events = %d, want 1", len(events))
	}
	if events[0].Reason != AlertDeliveryReasonNotificationsInactive {
		t.Fatalf("reason = %q, want %q", events[0].Reason, AlertDeliveryReasonNotificationsInactive)
	}
}

func TestEventLogDisabledManagerRecordsNothing(t *testing.T) {
	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)

	addEventLogAlert(manager, &Alert{
		ID:        "no-log-alert",
		Type:      "cpu",
		Level:     AlertLevelWarning,
		StartTime: time.Now(),
	})
	if err := manager.AcknowledgeAlert("no-log-alert", "richard"); err != nil {
		t.Fatalf("AcknowledgeAlert: %v", err)
	}

	events, err := manager.AlertEvents(eventlog.Filter{})
	if err != nil {
		t.Fatalf("AlertEvents: %v", err)
	}
	if events != nil {
		t.Fatalf("events = %+v, want nil with no event log", events)
	}
}
