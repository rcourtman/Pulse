package alerts

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
)

type replayedLifecycle struct {
	id    int64
	event LifecycleEvent
}

func replayOrderManager(t *testing.T) (*Manager, *eventlog.Store, *atomic.Int64) {
	t.Helper()
	m := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(m.Stop)
	store, err := eventlog.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.SetEventLog(store) // the manager owns Close
	var deliveries atomic.Int64
	m.SetAlertCallback(func(*Alert) { deliveries.Add(1) })
	m.SetResolvedCallback(func(string) { deliveries.Add(1) })
	return m, store, &deliveries
}

func replayAll(t *testing.T, m *Manager, afterID int64) []replayedLifecycle {
	t.Helper()
	var replayed []replayedLifecycle
	if err := m.ReplayLifecycleEvents(afterID, func(id int64, event LifecycleEvent) error {
		replayed = append(replayed, replayedLifecycle{id: id, event: event})
		return nil
	}); err != nil {
		t.Fatalf("ReplayLifecycleEvents(%d): %v", afterID, err)
	}
	return replayed
}

func snapshotEvent(t *testing.T, eventType string, alert Alert, at time.Time) eventlog.Event {
	t.Helper()
	payload, err := json.Marshal(alert)
	if err != nil {
		t.Fatal(err)
	}
	return eventlog.Event{Type: eventType, AlertID: alert.ID, ResourceID: alert.ResourceID, OccurredAt: at, Snapshot: payload}
}

func TestReplayLifecycleEventsFollowsDurableIDsAndKeepsStoredFacts(t *testing.T) {
	m, store, deliveries := replayOrderManager(t)
	base := time.Now().UTC().Add(-6 * time.Hour).Truncate(time.Second)
	ack := base.Add(time.Minute)

	// A legacy import whose first entry stayed open longest: its final-state
	// time is the latest even though its durable id is the lowest.
	longOpen := Alert{ID: "import-long", ResourceID: "res-long", Type: "cpu", Level: AlertLevelCritical,
		StartTime: base, LastSeen: base.Add(5 * time.Hour), Acknowledged: true, AckTime: &ack, AckUser: "operator"}
	brief := Alert{ID: "import-brief", ResourceID: "res-brief", Type: "cpu", Level: AlertLevelWarning,
		StartTime: base.Add(time.Hour), LastSeen: base.Add(time.Hour + time.Minute)}
	if err := store.ImportEvents([]eventlog.Event{
		snapshotEvent(t, eventlog.TypeHistoryImported, longOpen, longOpen.LastSeen),
		snapshotEvent(t, eventlog.TypeHistoryImported, brief, brief.LastSeen),
		// Not lifecycle facts: the replay must not visit them.
		{Type: eventlog.TypeNotificationDispatched, AlertID: brief.ID, OccurredAt: base},
		snapshotEvent(t, eventlog.TypeEscalated, brief, base.Add(2*time.Minute)),
		// A lifecycle label without a snapshot, and an unreadable snapshot.
		{Type: eventlog.TypeResolved, AlertID: "no-snapshot", OccurredAt: base},
		{Type: eventlog.TypeHistoryImported, AlertID: "corrupt", OccurredAt: base, Snapshot: json.RawMessage(`"not an alert object"`)},
	}); err != nil {
		t.Fatal(err)
	}
	live := Alert{ID: "live", ResourceID: "res-live", Type: "memory", Level: AlertLevelWarning,
		StartTime: base.Add(30 * time.Minute), LastSeen: base.Add(30 * time.Minute)}
	if err := store.AppendDurable(snapshotEvent(t, eventlog.TypeFired, live, live.StartTime)); err != nil {
		t.Fatal(err)
	}
	resolvedLive := live
	resolvedLive.LastSeen = base.Add(40 * time.Minute)
	if err := store.AppendDurable(snapshotEvent(t, eventlog.TypeResolved, resolvedLive, resolvedLive.LastSeen)); err != nil {
		t.Fatal(err)
	}

	replayed := replayAll(t, m, 0)
	wantOrder := []string{"import-long", "import-brief", "live", "live"}
	wantTypes := []string{eventlog.TypeHistoryImported, eventlog.TypeHistoryImported, eventlog.TypeFired, eventlog.TypeResolved}
	if len(replayed) != len(wantOrder) {
		t.Fatalf("replayed %d events, want %d lifecycle facts with readable snapshots", len(replayed), len(wantOrder))
	}
	stored, err := m.AlertEvents(eventlog.Filter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	storedAt := make(map[int64]time.Time, len(stored))
	for _, event := range stored {
		storedAt[event.ID] = event.OccurredAt
	}
	for i, item := range replayed {
		if i > 0 && item.id <= replayed[i-1].id {
			t.Fatalf("replay position %d has id %d after %d; want durable-id order", i, item.id, replayed[i-1].id)
		}
		if item.event.Alert == nil || item.event.Alert.ID != wantOrder[i] || item.event.Type != wantTypes[i] {
			t.Fatalf("replay position %d = %s %+v, want %s %s", i, item.event.Type, item.event.Alert, wantTypes[i], wantOrder[i])
		}
		if !item.event.Persisted {
			t.Fatalf("replayed event %d is not marked persisted", item.id)
		}
		if !item.event.OccurredAt.Equal(storedAt[item.id]) {
			t.Fatalf("replayed event %d time %s, want its stored time %s", item.id, item.event.OccurredAt, storedAt[item.id])
		}
	}
	first := replayed[0].event.Alert
	if !first.Acknowledged || first.AckUser != "operator" || first.AckTime == nil || !first.AckTime.Equal(ack) ||
		!first.LastSeen.Equal(longOpen.LastSeen) {
		t.Fatalf("imported final acknowledgement facts were not preserved: %+v", first)
	}

	// A watermark is a contiguous prefix: resuming after the first id returns
	// exactly the remaining facts, including the lower-timestamp import.
	tail := replayAll(t, m, replayed[0].id)
	if len(tail) != len(replayed)-1 {
		t.Fatalf("resume after %d visited %d events, want %d", replayed[0].id, len(tail), len(replayed)-1)
	}
	if tail[0].id != replayed[1].id {
		t.Fatalf("resume after %d started at id %d, want %d", replayed[0].id, tail[0].id, replayed[1].id)
	}
	if got := deliveries.Load(); got != 0 {
		t.Fatalf("replay invoked %d delivery callbacks", got)
	}
}

func TestReplayLifecycleEventsReturnsVisitorErrorAndBoundsConcurrentAppends(t *testing.T) {
	m, store, deliveries := replayOrderManager(t)
	base := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Second)
	var events []eventlog.Event
	for i := 0; i < 1203; i++ {
		alert := Alert{ID: fmt.Sprintf("bulk-%d", i), ResourceID: "res-bulk", Type: "cpu", Level: AlertLevelWarning,
			StartTime: base, LastSeen: base.Add(time.Duration(1203-i) * time.Second)}
		events = append(events, snapshotEvent(t, eventlog.TypeHistoryImported, alert, alert.LastSeen))
	}
	if err := store.ImportEvents(events); err != nil {
		t.Fatal(err)
	}
	boundary, err := store.ReplayBoundary()
	if err != nil {
		t.Fatal(err)
	}

	// Each visit commits another lifecycle fact. The pass still ends at the
	// high-water id captured when it began.
	visits := 0
	appended := 0
	if err := m.ReplayLifecycleEvents(0, func(id int64, event LifecycleEvent) error {
		if id > boundary.LastID {
			return fmt.Errorf("replayed id %d beyond the starting high-water id %d", id, boundary.LastID)
		}
		visits++
		alert := Alert{ID: fmt.Sprintf("during-%d", visits), ResourceID: "res-new", Type: "cpu", Level: AlertLevelWarning,
			StartTime: base, LastSeen: base}
		appended++
		return store.AppendDurable(snapshotEvent(t, eventlog.TypeAcknowledged, alert, base))
	}); err != nil {
		t.Fatal(err)
	}
	if visits != len(events) {
		t.Fatalf("visited %d, want the %d events committed before the pass", visits, len(events))
	}
	if next := replayAll(t, m, boundary.LastID); len(next) != appended {
		t.Fatalf("next pass visited %d, want the %d events appended during the first", len(next), appended)
	}

	sentinel := errors.New("canonical write failed")
	stopped := 0
	err = m.ReplayLifecycleEvents(0, func(int64, LifecycleEvent) error {
		stopped++
		if stopped == 1001 {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) || stopped != 1001 {
		t.Fatalf("replay returned %v after %d visits, want the visitor error at 1001", err, stopped)
	}
	if got := deliveries.Load(); got != 0 {
		t.Fatalf("replay invoked %d delivery callbacks", got)
	}
}

func TestLifecycleWatermarkIgnoresLegacyTimeOrderedCursor(t *testing.T) {
	m, store, _ := replayOrderManager(t)
	const consumer = "upgrade-consumer"

	// A release that replayed in occurrence-time order left its cursor under
	// the bare consumer name. It may sit above unprojected lower ids.
	if err := store.SetProjectionWatermark(consumer, 900); err != nil {
		t.Fatal(err)
	}
	if err := store.SetProjectionWatermark("unrelated-consumer", 7); err != nil {
		t.Fatal(err)
	}
	if got := m.LifecycleProjectionWatermark(consumer); got != 0 {
		t.Fatalf("ID-ordered lifecycle cursor = %d, want 0: the legacy cursor must not be trusted", got)
	}

	m.StoreLifecycleProjectionWatermark(consumer, 42)
	if got := m.LifecycleProjectionWatermark(consumer); got != 42 {
		t.Fatalf("ID-ordered lifecycle cursor = %d after store, want 42", got)
	}
	if got := store.ProjectionWatermark(lifecycleIDOrderWatermarkNamespace + consumer); got != 42 {
		t.Fatalf("namespaced cursor = %d, want 42", got)
	}
	if got := store.ProjectionWatermark(consumer); got != 900 {
		t.Fatalf("legacy generic cursor = %d, want it left untouched at 900", got)
	}
	if got := store.ProjectionWatermark("unrelated-consumer"); got != 7 {
		t.Fatalf("unrelated generic cursor = %d, want 7", got)
	}

	// Resetting to zero is the supported full-replay request.
	m.StoreLifecycleProjectionWatermark(consumer, 0)
	if got := m.LifecycleProjectionWatermark(consumer); got != 0 {
		t.Fatalf("reset lifecycle cursor = %d, want 0", got)
	}
	if got := store.ProjectionWatermark(consumer); got != 900 {
		t.Fatalf("reset changed the legacy cursor to %d", got)
	}
}

func TestLifecycleWatermarkPreservesEmptyNameValidation(t *testing.T) {
	m, store, _ := replayOrderManager(t)
	if err := store.SetProjectionWatermark("", 5); err == nil {
		t.Fatal("generic store accepted an empty consumer name")
	}
	if got := m.LifecycleProjectionWatermark(""); got != 0 {
		t.Fatalf("empty consumer cursor = %d, want 0", got)
	}
	m.StoreLifecycleProjectionWatermark("", 5) // rejected by the event log and logged
	if got := store.ProjectionWatermark(lifecycleIDOrderWatermarkNamespace); got != 0 {
		t.Fatalf("empty consumer wrote the bare namespace key with %d", got)
	}
	if got := m.LifecycleProjectionWatermark(""); got != 0 {
		t.Fatalf("empty consumer cursor = %d after store, want 0", got)
	}

	var nilManager *Manager
	if got := nilManager.LifecycleProjectionWatermark("consumer"); got != 0 {
		t.Fatalf("nil manager cursor = %d", got)
	}
	nilManager.StoreLifecycleProjectionWatermark("consumer", 3)
	withoutLog := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(withoutLog.Stop)
	withoutLog.StoreLifecycleProjectionWatermark("consumer", 3)
	if got := withoutLog.LifecycleProjectionWatermark("consumer"); got != 0 {
		t.Fatalf("manager without an event log reported cursor %d", got)
	}
}
