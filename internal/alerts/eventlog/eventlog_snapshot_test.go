package eventlog

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// A pre-snapshot database must upgrade in place and keep its rows readable.
func TestOpenUpgradesPreSnapshotSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, eventLogFileName)
	// Keep the legacy row within normal retention. A fixed calendar date
	// eventually tests expiry instead of whether the schema upgrade keeps it.
	occurredAt := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
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
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO alert_events (occurred_at, event_type, alert_id)
		VALUES (?, 'fired', 'legacy-alert');
	`, occurredAt.Format(time.RFC3339)); err != nil {
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
	if got := events[0]; got.Type != TypeFired || got.AlertID != "legacy-alert" || !got.OccurredAt.Equal(occurredAt) {
		t.Fatalf("legacy row changed during schema upgrade: %+v", got)
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

func TestSnapshotVolumeBoundPrunesOldestAndAdvancesRetention(t *testing.T) {
	store := newTestStore(t)
	base := time.Now().Add(-time.Hour).UTC()
	const eventCount = 8
	payload := bytes.Repeat([]byte("x"), 1024)
	for i := 0; i < eventCount; i++ {
		snapshot, err := json.Marshal(map[string]any{"id": fmt.Sprintf("a%d", i), "blob": string(payload)})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.AppendDurable(Event{
			OccurredAt: base.Add(time.Duration(i) * time.Minute),
			Type:       TypeFired,
			AlertID:    fmt.Sprintf("a%d", i),
			Snapshot:   snapshot,
		}); err != nil {
			t.Fatal(err)
		}
	}
	before, err := store.ReplayBoundary()
	if err != nil {
		t.Fatal(err)
	}
	// Keep roughly three events' payload; the older rows must be removed.
	store.pruneSnapshotVolume(3 * 1200)
	after, err := store.ReplayBoundary()
	if err != nil {
		t.Fatal(err)
	}
	if after.RetentionRevision <= before.RetentionRevision {
		t.Fatalf("volume prune did not advance retention revision: before=%+v after=%+v", before, after)
	}
	if after.LastID != before.LastID {
		t.Fatalf("volume prune changed the newest id: before=%+v after=%+v", before, after)
	}
	remaining, err := store.Query(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) == 0 || len(remaining) >= eventCount {
		t.Fatalf("volume prune left %d of %d events", len(remaining), eventCount)
	}
	for _, event := range remaining {
		if event.AlertID == "a0" {
			t.Fatalf("oldest event survived volume prune: %+v", remaining)
		}
	}
	// A second prune already within the cap must not change the boundary.
	store.pruneSnapshotVolume(3 * 1200)
	again, err := store.ReplayBoundary()
	if err != nil {
		t.Fatal(err)
	}
	if again != after {
		t.Fatalf("prune within the cap changed the boundary: %+v -> %+v", after, again)
	}
}

// One source writing more than the cap must lose its own oldest events, not
// the older history of a source that stores little.
func TestSnapshotVolumeBoundKeepsHistoryOfOtherSources(t *testing.T) {
	store := newTestStore(t)
	base := time.Now().Add(-time.Hour).UTC()
	payload := bytes.Repeat([]byte("x"), 1024)
	appendSnapshot := func(at time.Time, eventType, resourceID string) {
		t.Helper()
		snapshot, err := json.Marshal(map[string]any{"resourceId": resourceID, "blob": string(payload)})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.AppendDurable(Event{
			OccurredAt: at,
			Type:       eventType,
			AlertID:    resourceID + "-oom-kill",
			ResourceID: resourceID,
			Snapshot:   snapshot,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// The quiet host's incident is the oldest history in the log.
	const quietResource = "docker:nas01/postgres"
	appendSnapshot(base, TypeFired, quietResource)
	appendSnapshot(base.Add(time.Minute), TypeResolved, quietResource)
	// The loud host then reports a new container for every alert.
	const loudCount = 8
	for i := 0; i < loudCount; i++ {
		appendSnapshot(base.Add(time.Duration(i+2)*time.Minute), TypeFired, fmt.Sprintf("docker:loud/job-%d", i))
	}
	before, err := store.ReplayBoundary()
	if err != nil {
		t.Fatal(err)
	}
	// Room for roughly five of the ten snapshots.
	const maxBytes = 5 * 1200
	store.pruneSnapshotVolume(maxBytes)
	after, err := store.ReplayBoundary()
	if err != nil {
		t.Fatal(err)
	}
	if after.RetentionRevision <= before.RetentionRevision {
		t.Fatalf("volume prune did not advance retention revision: before=%+v after=%+v", before, after)
	}
	remaining, err := store.Query(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var total, quiet, loud int
	for _, event := range remaining {
		total += len(event.Snapshot)
		if event.ResourceID == quietResource {
			quiet++
			continue
		}
		loud++
		if event.ResourceID == "docker:loud/job-0" {
			t.Fatalf("loud source kept its oldest event: %+v", remaining)
		}
	}
	if total > maxBytes {
		t.Fatalf("volume prune left %d snapshot bytes, cap %d", total, maxBytes)
	}
	if quiet != 2 {
		t.Fatalf("quiet source kept %d of 2 events after another source exceeded the cap", quiet)
	}
	if loud == 0 || loud >= loudCount {
		t.Fatalf("loud source kept %d of %d events", loud, loudCount)
	}
	if newest := remaining[0]; newest.ResourceID != fmt.Sprintf("docker:loud/job-%d", loudCount-1) {
		t.Fatalf("loud source lost its newest event: %+v", newest)
	}
}

// The clear-history tombstone hides every older event. A volume prune keeps the
// older events of a source within its share, so it must keep the tombstone
// that hides them.
func TestSnapshotVolumeBoundKeepsHistoryClearedTombstone(t *testing.T) {
	store := newTestStore(t)
	base := time.Now().Add(-time.Hour).UTC()
	payload := bytes.Repeat([]byte("x"), 1024)
	appendSnapshot := func(at time.Time, resourceID string) {
		t.Helper()
		snapshot, err := json.Marshal(map[string]any{"resourceId": resourceID, "blob": string(payload)})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.AppendDurable(Event{
			OccurredAt: at,
			Type:       TypeFired,
			AlertID:    resourceID + "-oom-kill",
			ResourceID: resourceID,
			Snapshot:   snapshot,
		}); err != nil {
			t.Fatal(err)
		}
	}
	const clearedResource = "docker:nas01/postgres"
	appendSnapshot(base, clearedResource)
	if err := store.ImportEvents([]Event{{
		OccurredAt: base.Add(time.Minute),
		Type:       TypeHistoryCleared,
		AlertID:    "history",
	}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		appendSnapshot(base.Add(time.Duration(i+2)*time.Minute), fmt.Sprintf("docker:loud/job-%d", i))
	}
	store.pruneSnapshotVolume(3 * 1200)
	remaining, err := store.Query(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var cleared, tombstones, loud int
	for _, event := range remaining {
		switch {
		case event.Type == TypeHistoryCleared:
			tombstones++
		case event.ResourceID == clearedResource:
			cleared++
		default:
			loud++
		}
	}
	if loud == 0 || loud >= 4 {
		t.Fatalf("volume prune left %d of 4 events of the source over its share", loud)
	}
	if cleared != 1 || tombstones != 1 {
		t.Fatalf("volume prune left %d cleared events and %d tombstones, want 1 and 1", cleared, tombstones)
	}
}

// A Docker host reporting an ID with a slash ("nas01/x") records hostId
// "nas01/x" in its snapshot metadata and must not charge its bytes to "nas01".
func TestSnapshotVolumeBoundSeparatesSlashPrefixedHostIDs(t *testing.T) {
	store := newTestStore(t)
	base := time.Now().Add(-time.Hour).UTC()
	payload := bytes.Repeat([]byte("x"), 1024)
	appendHostSnapshot := func(at time.Time, hostID, resourceID string) {
		t.Helper()
		snapshot, err := json.Marshal(map[string]any{
			"resourceId": resourceID,
			"metadata":   map[string]any{"hostId": hostID},
			"blob":       string(payload),
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.AppendDurable(Event{
			OccurredAt: at,
			Type:       TypeFired,
			AlertID:    resourceID + "-oom-kill",
			ResourceID: resourceID,
			Snapshot:   snapshot,
		}); err != nil {
			t.Fatal(err)
		}
	}

	const quietResource = "docker:nas01/postgres"
	appendHostSnapshot(base, "nas01", quietResource)
	for i := 0; i < 8; i++ {
		appendHostSnapshot(base.Add(time.Duration(i+1)*time.Minute), "nas01/x", fmt.Sprintf("docker:nas01/x/job-%d", i))
	}

	store.pruneSnapshotVolume(4 * 1200)
	remaining, err := store.Query(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	foundQuiet := false
	for _, event := range remaining {
		if event.ResourceID == quietResource {
			foundQuiet = true
		}
	}
	if !foundQuiet {
		t.Fatal("slash-prefixed hostId evicted another host's alert snapshot")
	}
}
