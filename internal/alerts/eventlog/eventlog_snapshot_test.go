package eventlog

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// A pre-snapshot database must upgrade in place and keep its rows readable.
func TestOpenUpgradesPreSnapshotSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, eventLogFileName)
	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE alert_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at TEXT NOT NULL,
			event_type TEXT NOT NULL,
			alert_id TEXT NOT NULL,
			resource_id TEXT NOT NULL DEFAULT '',
			resource_name TEXT NOT NULL DEFAULT '',
			alert_type TEXT NOT NULL DEFAULT '',
			level TEXT NOT NULL DEFAULT '',
			reason TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			details TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO alert_events (occurred_at, event_type, alert_id)
		VALUES ('2026-08-01T00:00:00Z', 'fired', 'legacy-alert');
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(dir)
	if err != nil {
		t.Fatalf("open pre-snapshot db: %v", err)
	}
	defer store.Close()

	events, err := store.Query(Filter{AlertID: "legacy-alert"})
	if err != nil {
		t.Fatalf("query upgraded db: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected the legacy row to survive, got %d events", len(events))
	}
	if len(events[0].Snapshot) != 0 {
		t.Fatalf("legacy row grew a snapshot: %s", events[0].Snapshot)
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	snapshot := json.RawMessage(`{"id":"a1","value":95.5}`)
	store.Append(Event{
		OccurredAt: time.Now(),
		Type:       TypeFired,
		AlertID:    "a1",
		Snapshot:   snapshot,
	})
	store.Flush()

	events, err := store.Query(Filter{AlertID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if string(events[0].Snapshot) != string(snapshot) {
		t.Fatalf("snapshot round trip: got %s want %s", events[0].Snapshot, snapshot)
	}
}

func TestEventRetentionDoesNotExpireActiveState(t *testing.T) {
	store := newTestStore(t)
	startedAt := time.Now().Add(-180 * 24 * time.Hour).UTC()
	if err := store.AppendDurable(testActiveEvent(t, TypeFired, "long-running-alert", startedAt, startedAt, 91)); err != nil {
		t.Fatal(err)
	}
	store.pruneEventsBefore(time.Now().Add(-90 * 24 * time.Hour))

	events, err := store.Query(Filter{AlertID: "long-running-alert"})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("retained events = %d, want 0", len(events))
	}
	active, err := store.LoadActiveState()
	if err != nil || len(active) != 1 {
		t.Fatalf("active state = %+v, %v; long-running alert expired with history", active, err)
	}
}

func TestUnchangedDeliveryDecisionIsOneEpisode(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	started := time.Date(2026, 8, 27, 20, 0, 0, 0, time.UTC)
	base := Event{
		OccurredAt:   started,
		Type:         TypeNotificationSuppressed,
		AlertID:      "alert-1",
		ResourceID:   "node/pve-1",
		ResourceName: "pve-1",
		AlertType:    "cpu",
		Level:        "warning",
		Reason:       "notifications_inactive",
		Message:      "Notification suppressed: alert delivery is not turned on.",
		Details:      map[string]string{"activationState": "pending_review"},
	}
	store.Append(base)
	store.Flush()
	repeated := base
	repeated.OccurredAt = started.Add(10 * time.Second)
	store.Append(repeated)
	store.Flush()
	if got := store.appended.Load(); got != 1 {
		t.Fatalf("unchanged suppression admitted %d diagnostic events, want one", got)
	}

	events, err := store.Query(Filter{AlertID: base.AlertID})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("unchanged suppression produced %d events, want one episode", len(events))
	}
	if !events[0].OccurredAt.Equal(started) {
		t.Fatalf("episode timestamp = %s, want first decision at %s", events[0].OccurredAt, started)
	}

	changed := repeated
	changed.OccurredAt = started.Add(20 * time.Second)
	changed.Reason = "quiet_hours:critical"
	changed.Message = "Notification deferred by quiet hours."
	changed.Type = TypeNotificationDeferred
	changed.Details = map[string]string{"replayAt": started.Add(time.Hour).Format(time.RFC3339)}
	store.Append(changed)
	store.Flush()
	repeatedDeferred := changed
	repeatedDeferred.OccurredAt = started.Add(25 * time.Second)
	store.Append(repeatedDeferred)
	store.Append(Event{
		OccurredAt: started.Add(30 * time.Second),
		Type:       TypeNotificationDispatched,
		AlertID:    base.AlertID,
		Reason:     "ready",
	})
	resuppressed := base
	resuppressed.OccurredAt = started.Add(40 * time.Second)
	store.Append(resuppressed)
	store.Flush()
	if got := store.appended.Load(); got != 4 {
		t.Fatalf("changed delivery lifecycle admitted %d diagnostic events, want four", got)
	}

	events, err = store.Query(Filter{AlertID: base.AlertID})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("changed delivery lifecycle produced %d events, want four distinct episodes", len(events))
	}
	if events[0].Type != TypeNotificationSuppressed ||
		events[1].Type != TypeNotificationDispatched ||
		events[2].Type != TypeNotificationDeferred ||
		events[3].Type != TypeNotificationSuppressed {
		t.Fatalf("delivery episode order = %+v", events)
	}
}

func TestQueryProjectsLegacyRepeatedDeliveryRowsAsEpisodes(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	started := time.Date(2026, 8, 27, 20, 0, 0, 0, time.UTC)
	insert := func(alertID string, occurredAt time.Time, eventType, reason string) {
		t.Helper()
		if _, err := store.db.Exec(`
			INSERT INTO alert_events
				(occurred_at, event_type, alert_id, resource_id, resource_name, alert_type, level, reason, message, details, snapshot)
			VALUES (?, ?, ?, 'node/pve-1', 'pve-1', 'cpu', 'warning', ?, 'held', '{}', '')
		`, occurredAt.Format(time.RFC3339Nano), eventType, alertID, reason); err != nil {
			t.Fatal(err)
		}
	}

	insert("older-alert", started.Add(-time.Minute), TypeNotificationDeferred, "quiet_hours")
	insert("legacy-alert", started, TypeNotificationSuppressed, "notifications_inactive")
	insert("legacy-alert", started.Add(10*time.Second), TypeNotificationSuppressed, "notifications_inactive")
	insert("legacy-alert", started.Add(20*time.Second), TypeNotificationDispatched, "ready")
	insert("legacy-alert", started.Add(30*time.Second), TypeNotificationSuppressed, "notifications_inactive")
	insert("legacy-alert", started.Add(40*time.Second), TypeNotificationSuppressed, "notifications_inactive")

	events, err := store.Query(Filter{Limit: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("legacy delivery rows projected as %d events, want four distinct episodes", len(events))
	}
	if !events[0].OccurredAt.Equal(started.Add(30*time.Second)) ||
		events[1].Type != TypeNotificationDispatched ||
		!events[2].OccurredAt.Equal(started) ||
		events[3].AlertID != "older-alert" {
		t.Fatalf("legacy delivery episode order = %+v", events)
	}
}

func TestDeliveryEpisodeCoalescingSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 8, 27, 20, 0, 0, 0, time.UTC)
	event := Event{
		OccurredAt: started,
		Type:       TypeNotificationSuppressed,
		AlertID:    "restart-alert",
		Reason:     "notifications_inactive",
		Message:    "held",
		Details:    map[string]string{"activationState": "pending_review"},
	}
	store.Append(event)
	if err := store.Flush(); err != nil {
		t.Fatal(err)
	}
	store.Close()

	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	repeated := event
	repeated.OccurredAt = started.Add(10 * time.Second)
	reopened.Append(repeated)
	if err := reopened.Flush(); err != nil {
		t.Fatal(err)
	}

	var rows int
	if err := reopened.db.QueryRow(
		`SELECT COUNT(*) FROM alert_events WHERE alert_id = ?`,
		event.AlertID,
	).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("restart appended %d unchanged suppression rows, want one episode", rows)
	}
}

func TestConcurrentUnchangedDeliveryDecisionAdmitsOneEpisode(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	event := Event{
		OccurredAt: time.Date(2026, 8, 27, 20, 0, 0, 0, time.UTC),
		Type:       TypeNotificationSuppressed,
		AlertID:    "concurrent-alert",
		Reason:     "acknowledged",
		Message:    "held",
	}
	var writers sync.WaitGroup
	for range 100 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			store.Append(event)
		}()
	}
	writers.Wait()
	if err := store.Flush(); err != nil {
		t.Fatal(err)
	}

	if got := store.appended.Load(); got != 1 {
		t.Fatalf("concurrent reevaluations admitted %d diagnostic events, want one", got)
	}
	events, err := store.Query(Filter{AlertID: event.AlertID})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("concurrent reevaluations wrote %d episodes, want one", len(events))
	}
}
