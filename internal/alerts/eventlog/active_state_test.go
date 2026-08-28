package eventlog

import (
	"encoding/json"
	"testing"
	"time"
)

func testActiveEvent(t *testing.T, eventType, alertID string, startedAt, occurredAt time.Time, value float64) Event {
	t.Helper()
	snapshot, err := json.Marshal(map[string]any{
		"id":        alertID,
		"startTime": startedAt,
		"lastSeen":  occurredAt,
		"value":     value,
	})
	if err != nil {
		t.Fatal(err)
	}
	return Event{
		Type:       eventType,
		AlertID:    alertID,
		OccurredAt: occurredAt,
		Snapshot:   snapshot,
	}
}

func TestLifecycleEventsTransactionallyProjectActiveState(t *testing.T) {
	store := newTestStore(t)
	startedAt := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)

	if err := store.AppendDurable(testActiveEvent(t, TypeFired, "alert-1", startedAt, startedAt, 91)); err != nil {
		t.Fatalf("append fired: %v", err)
	}
	initialized, err := store.ActiveStateInitialized()
	if err != nil || !initialized {
		t.Fatalf("ActiveStateInitialized = %v, %v; want true, nil", initialized, err)
	}
	active, err := store.LoadActiveState()
	if err != nil || len(active) != 1 {
		t.Fatalf("LoadActiveState = %+v, %v; want one alert", active, err)
	}
	if active[0].AlertID != "alert-1" || !active[0].OccurrenceStartedAt.Equal(startedAt) {
		t.Fatalf("active state = %+v", active[0])
	}

	acknowledgedAt := startedAt.Add(10 * time.Minute)
	if err := store.AppendDurable(testActiveEvent(t, TypeAcknowledged, "alert-1", startedAt, acknowledgedAt, 96)); err != nil {
		t.Fatalf("append acknowledgement: %v", err)
	}
	active, err = store.LoadActiveState()
	if err != nil || len(active) != 1 {
		t.Fatalf("active state after acknowledgement = %+v, %v", active, err)
	}
	var acknowledged map[string]any
	if err := json.Unmarshal(active[0].Snapshot, &acknowledged); err != nil {
		t.Fatal(err)
	}
	if acknowledged["value"] != float64(96) {
		t.Fatalf("acknowledged snapshot value = %v, want 96", acknowledged["value"])
	}

	if err := store.AppendDurable(testActiveEvent(t, TypeResolved, "alert-1", startedAt, acknowledgedAt.Add(time.Minute), 40)); err != nil {
		t.Fatalf("append resolution: %v", err)
	}
	active, err = store.LoadActiveState()
	if err != nil || len(active) != 0 {
		t.Fatalf("active state after resolution = %+v, %v; want empty", active, err)
	}
}

func TestDelayedResolutionCannotDeleteNewerOccurrence(t *testing.T) {
	store := newTestStore(t)
	oldStart := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	newStart := oldStart.Add(time.Hour)

	if err := store.AppendDurable(testActiveEvent(t, TypeFired, "reused-alert", oldStart, oldStart, 91)); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendDurable(testActiveEvent(t, TypeFired, "reused-alert", newStart, newStart, 94)); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendDurable(testActiveEvent(t, TypeResolved, "reused-alert", oldStart, newStart.Add(time.Minute), 20)); err != nil {
		t.Fatal(err)
	}

	active, err := store.LoadActiveState()
	if err != nil || len(active) != 1 {
		t.Fatalf("LoadActiveState = %+v, %v; want newer occurrence", active, err)
	}
	if !active[0].OccurrenceStartedAt.Equal(newStart) {
		t.Fatalf("remaining occurrence = %v, want %v", active[0].OccurrenceStartedAt, newStart)
	}
}

func TestStaleCheckpointCannotOverwriteLifecycleTransaction(t *testing.T) {
	store := newTestStore(t)
	startedAt := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	activeEvent := testActiveEvent(t, TypeFired, "cas-alert", startedAt, startedAt, 91)
	initial := ActiveStateSnapshot{
		AlertID:             activeEvent.AlertID,
		OccurrenceStartedAt: startedAt,
		Snapshot:            activeEvent.Snapshot,
	}
	if err := store.ReplaceActiveState([]ActiveStateSnapshot{initial}); err != nil {
		t.Fatal(err)
	}
	revision, err := store.ActiveStateRevision()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendDurable(testActiveEvent(t, TypeResolved, "cas-alert", startedAt, startedAt.Add(time.Minute), 20)); err != nil {
		t.Fatal(err)
	}

	replaced, err := store.ReplaceActiveStateIfRevision([]ActiveStateSnapshot{initial}, revision)
	if err != nil {
		t.Fatal(err)
	}
	if replaced {
		t.Fatal("stale checkpoint replaced a newer lifecycle transaction")
	}
	active, err := store.LoadActiveState()
	if err != nil || len(active) != 0 {
		t.Fatalf("active state = %+v, %v; resolved alert was resurrected", active, err)
	}
}
