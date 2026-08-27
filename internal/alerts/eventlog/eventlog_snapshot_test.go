package eventlog

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
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
