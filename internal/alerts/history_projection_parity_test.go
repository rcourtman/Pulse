package alerts

// Parity: the event-log history projection must reproduce the JSON-backed
// history manager's output for the same lifecycle run
// (docs/ALERT_ENGINE_EVOLUTION.md — the event log becomes the sole history
// authority only once this holds). The JSON history characterizes the
// projection: a divergence is a projection bug unless investigation proves
// a history-manager defect, which is then fixed first.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
)

func TestHistoryProjectionIncrementalReadersPreserveChronologyAndIsolation(t *testing.T) {
	m := newHistoryParityManager(t)
	store := m.eventLogStore()
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	appendEvent := func(kind, id string, occurred time.Time, value float64) {
		t.Helper()
		snapshot, err := json.Marshal(Alert{ID: id, ResourceID: "vm-" + id, StartTime: at, LastSeen: occurred, Value: value})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.AppendDurable(eventlog.Event{Type: kind, AlertID: id, OccurredAt: occurred, Snapshot: snapshot}); err != nil {
			t.Fatal(err)
		}
	}
	read := func() []Alert {
		t.Helper()
		result, ok := m.AlertHistoryFromEvents(time.Time{}, 0)
		if !ok {
			t.Fatal("history unavailable")
		}
		return result
	}
	compareFresh := func() {
		t.Helper()
		incremental := read()
		m.historyProjectionMu.Lock()
		m.historyProjection = nil
		m.historyProjectionMu.Unlock()
		if fresh := read(); !reflect.DeepEqual(incremental, fresh) {
			t.Fatalf("incremental history differs from a complete chronological replay:\n%+v\n%+v", incremental, fresh)
		}
	}
	appendEvent(eventlog.TypeFired, "one", at, 90)
	first := read()
	first[0].Value = -1
	if got := read(); got[0].Value != 90 {
		t.Fatal("caller mutation contaminated the durable history fold")
	}
	appendEvent(eventlog.TypeResolved, "one", at.Add(2*time.Minute), 30)
	compareFresh()
	// A delayed earlier event must be placed before the resolution, not
	// overwrite it just because its durable insertion ID is newer.
	appendEvent(eventlog.TypeFired, "one", at.Add(time.Minute), 99)
	compareFresh()
	if got := read(); got[0].Value != 30 {
		t.Fatalf("late event replaced the later resolution: %+v", got)
	}
	if err := store.AppendDurable(eventlog.Event{Type: eventlog.TypeHistoryCleared, OccurredAt: at.Add(3 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if got := read(); len(got) != 0 {
		t.Fatalf("clear left cached occurrences: %+v", got)
	}
	appendEvent(eventlog.TypeFired, "two", at.Add(4*time.Minute), 85)
	compareFresh()
	if _, ok := m.AlertHistoryFromEvents(at.Add(4*time.Minute), 5); !ok {
		t.Fatal("windowed history unavailable")
	}
	want := read()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, ok := m.AlertHistoryFromEvents(time.Time{}, 5)
			if !ok || !reflect.DeepEqual(got, want) {
				t.Errorf("concurrent history: %+v, available=%v", got, ok)
			}
			if len(got) > 0 {
				got[0].Value = -2
			}
		}()
	}
	wg.Wait()
	if got := read(); !reflect.DeepEqual(got, want) {
		t.Fatal("parallel callers contaminated the retained fold")
	}
}

func newHistoryParityManager(t *testing.T) *Manager {
	t.Helper()
	m := newTestManager(t)
	m.UpdateConfig(contractTestConfig(m))
	store, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory event log: %v", err)
	}
	m.SetEventLog(store)
	t.Cleanup(func() { m.SetEventLog(nil) })
	return m
}

// assertHistoryParity compares the projection against the JSON history on
// the fields the history API exposes.
func assertHistoryParity(t *testing.T, m *Manager) {
	t.Helper()
	// Read the JSON-backed side directly: GetAlertHistory itself now
	// returns the projection, so going through it would compare the
	// projection with itself.
	legacy := m.applyCurrentNodeDisplayNames(canonicalizeAlertHistoryForOutput(m.historyManager.GetAllHistory(0)))
	projected, ok := m.AlertHistoryFromEvents(time.Time{}, 0)
	if !ok {
		t.Fatal("projection unavailable despite an enabled event log")
	}
	if len(projected) != len(legacy) {
		t.Fatalf("projection has %d entries, JSON history has %d\nprojected: %+v\nlegacy: %+v",
			len(projected), len(legacy), summarizeHistory(projected), summarizeHistory(legacy))
	}
	for i := range legacy {
		l, p := legacy[i], projected[i]
		if l.ID != p.ID {
			t.Errorf("entry %d: ID diverges: legacy=%q projected=%q", i, l.ID, p.ID)
			continue
		}
		if l.ResourceID != p.ResourceID {
			t.Errorf("entry %d (%s): ResourceID legacy=%q projected=%q", i, l.ID, l.ResourceID, p.ResourceID)
		}
		if l.Level != p.Level {
			t.Errorf("entry %d (%s): Level legacy=%q projected=%q", i, l.ID, l.Level, p.Level)
		}
		if !l.StartTime.Truncate(time.Second).Equal(p.StartTime.Truncate(time.Second)) {
			t.Errorf("entry %d (%s): StartTime legacy=%v projected=%v", i, l.ID, l.StartTime, p.StartTime)
		}
		if l.Acknowledged != p.Acknowledged {
			t.Errorf("entry %d (%s): Acknowledged legacy=%v projected=%v", i, l.ID, l.Acknowledged, p.Acknowledged)
		}
		if l.Value != p.Value {
			t.Errorf("entry %d (%s): Value legacy=%v projected=%v", i, l.ID, l.Value, p.Value)
		}
		if l.ResourceName != p.ResourceName {
			t.Errorf("entry %d (%s): ResourceName legacy=%q projected=%q", i, l.ID, l.ResourceName, p.ResourceName)
		}
		if l.Node != p.Node {
			t.Errorf("entry %d (%s): Node legacy=%q projected=%q", i, l.ID, l.Node, p.Node)
		}
		if l.Message != p.Message {
			t.Errorf("entry %d (%s): Message legacy=%q projected=%q", i, l.ID, l.Message, p.Message)
		}
		if l.Type != p.Type {
			t.Errorf("entry %d (%s): Type legacy=%q projected=%q", i, l.ID, l.Type, p.Type)
		}
		// The two sides stamp resolve-time LastSeen at slightly different
		// instants; hold them within a small tolerance.
		if delta := l.LastSeen.Sub(p.LastSeen); delta > 2*time.Second || delta < -2*time.Second {
			t.Errorf("entry %d (%s): LastSeen legacy=%v projected=%v", i, l.ID, l.LastSeen, p.LastSeen)
		}
	}
}

func summarizeHistory(entries []Alert) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.ID+"@"+e.StartTime.Format(time.RFC3339))
	}
	return out
}

func TestHistoryProjectionParityActiveAlert(t *testing.T) {
	m := newHistoryParityManager(t)
	contractRaiseGuestCPUAlert(t, m, "hp-vm-1", 95)
	assertHistoryParity(t, m)
}

func TestHistoryProjectionParityResolvedAlert(t *testing.T) {
	m := newHistoryParityManager(t)
	cfg := m.GetConfig()
	contractRaiseGuestCPUAlert(t, m, "hp-vm-2", 95)
	// A clearing observation resolves through the real evaluation path.
	m.checkMetric("hp-vm-2", "Contract VM hp-vm-2", "node1", "inst1", "guest", "cpu", 50, cfg.GuestDefaults.CPU, nil)
	assertHistoryParity(t, m)
}

func TestHistoryProjectionParityAcknowledgedAlert(t *testing.T) {
	m := newHistoryParityManager(t)
	alert := contractRaiseGuestCPUAlert(t, m, "hp-vm-3", 95)
	if err := m.AcknowledgeAlert(alert.ID, "parity-user"); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	assertHistoryParity(t, m)
}

func TestHistoryProjectionParityMultipleResources(t *testing.T) {
	m := newHistoryParityManager(t)
	cfg := m.GetConfig()
	contractRaiseGuestCPUAlert(t, m, "hp-vm-4", 92)
	contractRaiseGuestCPUAlert(t, m, "hp-vm-5", 93)
	contractRaiseGuestCPUAlert(t, m, "hp-vm-6", 94)
	m.checkMetric("hp-vm-5", "Contract VM hp-vm-5", "node1", "inst1", "guest", "cpu", 40, cfg.GuestDefaults.CPU, nil)
	assertHistoryParity(t, m)
}

func TestHistoryProjectionParitySecondOccurrence(t *testing.T) {
	m := newHistoryParityManager(t)
	cfg := m.GetConfig()

	contractRaiseGuestCPUAlert(t, m, "hp-vm-7", 95)
	m.checkMetric("hp-vm-7", "Contract VM hp-vm-7", "node1", "inst1", "guest", "cpu", 40, cfg.GuestDefaults.CPU, nil)

	// Age the resolved occurrence past the refire window in both the
	// manager's ledger and the core so the next trigger opens a genuinely
	// new occurrence (a new history row) instead of a reactivation.
	m.resolvedMutex.Lock()
	for key, record := range m.recentlyResolved {
		record.ResolvedTime = record.ResolvedTime.Add(-10 * time.Minute)
		m.recentlyResolved[key] = record
	}
	m.resolvedMutex.Unlock()
	m.mu.Lock()
	m.core.ShiftResolved(-10 * time.Minute)
	m.mu.Unlock()

	contractRaiseGuestCPUAlert(t, m, "hp-vm-7", 97)
	assertHistoryParity(t, m)
}

func TestHistoryProjectionWalksEveryLifecycleEventBeyondQueryCap(t *testing.T) {
	m := newTestManager(t)
	store, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory event log: %v", err)
	}
	m.SetEventLog(store)
	t.Cleanup(func() { m.SetEventLog(nil) })

	const occurrenceCount = 1205
	base := time.Now().Add(-occurrenceCount * time.Second).UTC()
	events := make([]eventlog.Event, 0, occurrenceCount)
	for i := 0; i < occurrenceCount; i++ {
		occurredAt := base.Add(time.Duration(i) * time.Second)
		id := fmt.Sprintf("history-cap-%04d", i)
		snapshot, marshalErr := json.Marshal(Alert{
			ID:           id,
			Type:         "cpu",
			Level:        AlertLevelWarning,
			ResourceID:   id,
			ResourceName: id,
			StartTime:    occurredAt,
			LastSeen:     occurredAt,
		})
		if marshalErr != nil {
			t.Fatalf("marshal fixture %d: %v", i, marshalErr)
		}
		events = append(events, eventlog.Event{
			OccurredAt: occurredAt,
			Type:       eventlog.TypeHistoryImported,
			AlertID:    id,
			Snapshot:   snapshot,
		})
	}
	if err := store.ImportEvents(events); err != nil {
		t.Fatalf("import history fixtures: %v", err)
	}

	projected, ok := m.AlertHistoryFromEvents(time.Time{}, 0)
	if !ok {
		t.Fatal("projection unavailable")
	}
	if len(projected) != occurrenceCount {
		t.Fatalf("projection returned %d occurrences, want %d", len(projected), occurrenceCount)
	}
	if projected[0].ID != "history-cap-1204" || projected[len(projected)-1].ID != "history-cap-0000" {
		t.Fatalf("projection order = %q ... %q, want newest through oldest",
			projected[0].ID, projected[len(projected)-1].ID)
	}

	limited, ok := m.AlertHistoryFromEvents(time.Time{}, 25)
	if !ok || len(limited) != 25 {
		t.Fatalf("limited projection = %d entries, ok=%v; want 25, true", len(limited), ok)
	}
	if limited[0].ID != "history-cap-1204" || limited[24].ID != "history-cap-1180" {
		t.Fatalf("limited projection order = %q ... %q", limited[0].ID, limited[24].ID)
	}
}
