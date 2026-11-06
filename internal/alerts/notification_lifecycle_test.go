package alerts

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

func TestNotificationOccurrenceLifecycleRequiresPositiveAuthority(t *testing.T) {
	start := time.Now()
	m := &Manager{activeAlerts: map[string]*Alert{"key": {ID: "same-id", StartTime: start}}}
	if state := m.NotificationOccurrenceLifecycle("same-id", start); state.State != NotificationOccurrenceActive {
		t.Fatal("explicit active occurrence not recognised")
	}
	if state := m.NotificationOccurrenceLifecycle("same-id", start.Add(-time.Second)); state.State != NotificationOccurrenceUnavailable {
		t.Fatal("absence of older occurrence treated as completion")
	}
	if state := m.NotificationOccurrenceLifecycle("missing", start); state.State != NotificationOccurrenceUnavailable {
		t.Fatal("missing storage treated as completion")
	}
	m.stopping = true
	if state := m.NotificationOccurrenceLifecycle("same-id", start); state.State != NotificationOccurrenceUnavailable {
		t.Fatal("shutdown supplied lifecycle authority")
	}
}

func TestNotificationOccurrenceLifecycleHealthyHistoryAndRefire(t *testing.T) {
	m := newEventLogManager(t)
	m.activeStateAuthoritative.Store(true)
	m.eventHistoryAuthoritative.Store(true)
	start := time.Now().Add(-40 * 24 * time.Hour).UTC()
	a := &Alert{ID: "lifecycle", StartTime: start, Level: AlertLevelCritical}
	appendEvent := func(kind string, at time.Time, snapshot *Alert) {
		t.Helper()
		data, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if err := m.eventLogStore().AppendDurable(eventlog.Event{OccurredAt: at, Type: kind, AlertID: a.ID, Snapshot: data}); err != nil {
			t.Fatal(err)
		}
	}
	if got := m.NotificationOccurrenceLifecycle(a.ID, start); got.State != NotificationOccurrenceNoRecord {
		t.Fatalf("healthy empty history: %+v", got)
	}
	appendEvent(eventlog.TypeFired, start, a)
	if got := m.NotificationOccurrenceLifecycle(a.ID, start); got.State != NotificationOccurrenceNoRecord || !got.LatestActivationAt.Equal(start) {
		t.Fatalf("positive firing without live active snapshot: %+v", got)
	}
	resolved := start.Add(time.Hour)
	appendEvent(eventlog.TypeResolved, resolved, a)
	if got := m.NotificationOccurrenceLifecycle(a.ID, start); got.State != NotificationOccurrenceResolved || !got.ResolvedAt.Equal(resolved) {
		t.Fatalf("positive resolution: %+v", got)
	}
	refired := resolved.Add(time.Hour)
	appendEvent(eventlog.TypeRefired, refired, a)
	if got := m.NotificationOccurrenceLifecycle(a.ID, start); got.State != NotificationOccurrenceNoRecord || !got.LatestActivationAt.Equal(refired) {
		t.Fatalf("refire lost activation authority: %+v", got)
	}
	if got := m.NotificationOccurrenceLifecycle(a.ID, start.Add(time.Nanosecond)); got.State != NotificationOccurrenceNoRecord || !got.LatestActivationAt.IsZero() {
		t.Fatalf("different occurrence inherited history: %+v", got)
	}
	a.OperationalRecord = &operationaltrust.OperationalRecord{State: operationaltrust.OperationalResolved}
	appendEvent(eventlog.TypeHistoryImported, refired.Add(time.Hour), a)
	if got := m.NotificationOccurrenceLifecycle(a.ID, start); got.State != NotificationOccurrenceResolved {
		t.Fatalf("explicit imported resolution lost: %+v", got)
	}
	m.eventHistoryAuthoritative.Store(false)
	if got := m.NotificationOccurrenceLifecycle(a.ID, start); got.State != NotificationOccurrenceUnavailable {
		t.Fatalf("incomplete migration became authority: %+v", got)
	}
}

func TestNotificationOccurrenceLifecycleMalformedSnapshotAndClosedStore(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithDataDir(dir)
	t.Cleanup(m.Stop)
	diskStore, openErr := eventlog.Open(filepath.Join(dir, "alerts"))
	if openErr != nil {
		t.Fatal(openErr)
	}
	m.SetEventLog(diskStore)
	m.activeStateAuthoritative.Store(true)
	m.eventHistoryAuthoritative.Store(true)
	start := time.Now().UTC()
	store := m.eventLogStore()
	valid, err := json.Marshal(&Alert{ID: "unreadable", StartTime: start})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendDurable(eventlog.Event{OccurredAt: start, Type: eventlog.TypeResolved, AlertID: "unreadable", Snapshot: valid}); err != nil {
		t.Fatal(err)
	}
	if !store.Healthy() {
		t.Fatal("valid fixture did not start healthy")
	}
	if got := m.NotificationOccurrenceLifecycle("unreadable", start); got.State != NotificationOccurrenceResolved {
		t.Fatalf("valid retained resolution was not authoritative before corruption: %+v", got)
	}
	// Simulate a retained legacy/corrupt row below the append API. This must
	// exercise the lifecycle reader, not merely AppendDurable validation.
	db, err := sql.Open("sqlite", filepath.Join(dir, "alerts", "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`UPDATE alert_events SET snapshot='"invalid alert snapshot"' WHERE alert_id='unreadable'`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	if got := m.NotificationOccurrenceLifecycle("unreadable", start); got.State != NotificationOccurrenceUnavailable {
		t.Fatalf("unreadable retained history became lifecycle authority: %+v", got)
	}
	store.Close()
	if got := m.NotificationOccurrenceLifecycle("missing", start); got.State != NotificationOccurrenceUnavailable {
		t.Fatalf("closed store became healthy absence: %+v", got)
	}
}

func TestNotificationOccurrenceLifecycleCurrentAbsenceDoesNotResolveHistory(t *testing.T) {
	m := newEventLogManager(t)
	m.activeStateAuthoritative.Store(true)
	m.eventHistoryAuthoritative.Store(false)
	state := m.NotificationOccurrenceLifecycle("not-current", time.Now())
	if state.State != NotificationOccurrenceUnavailable || !state.CurrentInactive {
		t.Fatalf("current absence and unknown history were conflated: %+v", state)
	}
	m.activeStateAuthoritative.Store(false)
	state = m.NotificationOccurrenceLifecycle("not-current", time.Now())
	if state.CurrentInactive {
		t.Fatalf("incomplete active inventory supplied current absence: %+v", state)
	}
}
