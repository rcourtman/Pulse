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
	"math/rand"
	"reflect"
	"runtime"
	"sort"
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

// Windowed and limited history reads are answered from the one shared
// full-history fold (#1803). Before, every read deep-copied the whole fold and
// every Since read re-decoded each snapshot in its window. These tests hold
// the shared answer equal to an independent walk of only the window's events,
// which is the original algorithm.

// referenceWindowedHistory is the original read path: a fresh fold of only
// the events the log's Since filter admits, the live overlay, then the sort.
func referenceWindowedHistory(t *testing.T, m *Manager, since time.Time, limit int) []Alert {
	t.Helper()
	type occurrence struct {
		alert      Alert
		firstEvent time.Time
	}
	occurrences := map[string]*occurrence{}
	var order []string
	err := m.eventLogStore().WalkOldest(eventlog.Filter{
		Types: []string{
			eventlog.TypeFired, eventlog.TypeRefired, eventlog.TypeResolved,
			eventlog.TypeAcknowledged, eventlog.TypeUnacknowledged,
			eventlog.TypeSnoozed, eventlog.TypeUnsnoozed, eventlog.TypeEscalated,
			eventlog.TypeHistoryImported, eventlog.TypeHistoryCleared,
		},
		Since: since,
	}, func(event eventlog.Event) error {
		if event.Type == eventlog.TypeHistoryCleared {
			occurrences = map[string]*occurrence{}
			order = order[:0]
			return nil
		}
		var snapshot Alert
		if len(event.Snapshot) == 0 || json.Unmarshal(event.Snapshot, &snapshot) != nil {
			return nil
		}
		key := historyOccurrenceKey(event.AlertID, &snapshot)
		occ, exists := occurrences[key]
		if !exists {
			occ = &occurrence{alert: snapshot, firstEvent: event.OccurredAt}
			occurrences[key] = occ
			order = append(order, key)
		} else {
			occ.alert = mergeHistoryAlertSnapshots(occ.alert, snapshot)
		}
		if event.Type == eventlog.TypeResolved && event.OccurredAt.After(occ.alert.LastSeen) {
			occ.alert.LastSeen = event.OccurredAt
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reference walk: %v", err)
	}
	m.mu.RLock()
	var active []*Alert
	for storageKey, alert := range m.activeAlerts {
		exported := cloneAlertForOutput(alert)
		if exported.ID == "" {
			exported.ID = effectiveAlertID(alert, storageKey)
		}
		active = append(active, exported)
	}
	m.mu.RUnlock()
	for _, alert := range active {
		key := historyOccurrenceKey(alert.ID, alert)
		if occ, exists := occurrences[key]; exists {
			ack, ackTime, ackUser := occ.alert.Acknowledged, occ.alert.AckTime, occ.alert.AckUser
			occ.alert = *alert
			if ack && !occ.alert.Acknowledged {
				occ.alert.Acknowledged, occ.alert.AckTime, occ.alert.AckUser = true, ackTime, ackUser
			}
		} else if since.IsZero() || alert.StartTime.After(since) {
			occurrences[key] = &occurrence{alert: *alert, firstEvent: alert.StartTime}
			order = append(order, key)
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		return occurrences[order[i]].firstEvent.Before(occurrences[order[j]].firstEvent)
	})
	results := []Alert{}
	for i := len(order) - 1; i >= 0; i-- {
		if limit > 0 && len(results) >= limit {
			break
		}
		results = append(results, occurrences[order[i]].alert)
	}
	return m.applyCurrentNodeDisplayNames(canonicalizeAlertHistoryForOutput(results))
}

// writeRandomHistory appends a lifecycle history with acknowledgements on
// both sides of window boundaries, out-of-order LastSeen values, uneven
// fractional seconds (so RFC3339Nano text order and time order can differ),
// same-instant events and a history clear. It returns every event time.
func writeRandomHistory(t *testing.T, m *Manager, seed int64) []time.Time {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	store := m.eventLogStore()
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	kinds := []string{
		eventlog.TypeFired, eventlog.TypeRefired, eventlog.TypeAcknowledged,
		eventlog.TypeUnacknowledged, eventlog.TypeSnoozed, eventlog.TypeEscalated, eventlog.TypeResolved,
	}
	fractions := []time.Duration{0, 500 * time.Millisecond, 50 * time.Millisecond, 123456789}
	var times []time.Time
	at := base
	for i := 0; i < 120; i++ {
		if rng.Intn(4) != 0 {
			at = at.Add(time.Duration(rng.Intn(3)) * time.Second)
		}
		occurred := at.Add(fractions[rng.Intn(len(fractions))])
		times = append(times, occurred)
		if i == 60 {
			if err := store.AppendDurable(eventlog.Event{Type: eventlog.TypeHistoryCleared, AlertID: "history", OccurredAt: occurred}); err != nil {
				t.Fatal(err)
			}
			continue
		}
		id := fmt.Sprintf("alert-%d", rng.Intn(5))
		snapshot := Alert{
			ID:         id,
			ResourceID: "vm-" + id,
			StartTime:  base.Add(time.Duration(rng.Intn(3)) * time.Hour),
			LastSeen:   occurred.Add(time.Duration(rng.Intn(240)-120) * time.Second),
			Value:      float64(i),
		}
		kind := kinds[rng.Intn(len(kinds))]
		if kind == eventlog.TypeAcknowledged || (kind != eventlog.TypeUnacknowledged && rng.Intn(5) == 0) {
			ackTime := occurred
			snapshot.Acknowledged, snapshot.AckTime, snapshot.AckUser = true, &ackTime, fmt.Sprintf("user-%d", i)
		}
		payload, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.AppendDurable(eventlog.Event{Type: kind, AlertID: id, OccurredAt: occurred, Snapshot: payload}); err != nil {
			t.Fatal(err)
		}
	}
	return times
}

func TestWindowedHistoryMatchesAWalkOfOnlyTheWindow(t *testing.T) {
	for seed := int64(1); seed <= 25; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			m := newHistoryParityManager(t)
			times := writeRandomHistory(t, m, seed)
			// Live overlay: one alert the log knows (with a fresh value) and
			// one it never saw fire.
			m.mu.Lock()
			m.activeAlerts["live-known"] = &Alert{ID: "alert-1", ResourceID: "vm-alert-1",
				StartTime: time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC), LastSeen: times[len(times)-1], Value: 999}
			m.activeAlerts["live-new"] = &Alert{ID: "alert-new", ResourceID: "vm-new",
				StartTime: times[len(times)/2], LastSeen: times[len(times)-1], Value: 7}
			m.mu.Unlock()

			windows := []time.Time{{}, times[0].Add(-time.Hour), times[len(times)-1].Add(time.Hour)}
			for i := 0; i < len(times); i += 7 {
				windows = append(windows, times[i], times[i].Add(time.Nanosecond), times[i].Add(-time.Nanosecond))
			}
			for _, since := range windows {
				for _, limit := range []int{0, 1, 3} {
					got, ok := m.AlertHistoryFromEvents(since, limit)
					if !ok {
						t.Fatal("history unavailable")
					}
					want := referenceWindowedHistory(t, m, since, limit)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("since=%s limit=%d: shared fold differs from a walk of only the window\n got: %+v\nwant: %+v",
							since.Format(time.RFC3339Nano), limit, got, want)
					}
				}
			}
		})
	}
}

func TestWindowedHistoryReusesTheFullHistoryFold(t *testing.T) {
	m := newHistoryParityManager(t)
	times := writeRandomHistory(t, m, 7)
	if _, ok := m.AlertHistoryFromEvents(time.Time{}, 0); !ok {
		t.Fatal("history unavailable")
	}
	m.historyProjectionMu.Lock()
	full := m.historyProjection
	m.historyProjectionMu.Unlock()
	if full == nil {
		t.Fatal("a full-history read did not keep its fold")
	}
	for _, since := range []time.Time{times[10], times[90], times[len(times)-1]} {
		if _, ok := m.AlertHistoryFromEvents(since, 0); !ok {
			t.Fatal("history unavailable")
		}
		m.historyProjectionMu.Lock()
		current := m.historyProjection
		m.historyProjectionMu.Unlock()
		if current != full {
			t.Fatalf("a Since read at %s replaced the shared fold instead of reusing it", since)
		}
	}
}

func TestHistoryReadResultsDoNotAliasTheSharedFold(t *testing.T) {
	m := newHistoryParityManager(t)
	times := writeRandomHistory(t, m, 11)
	m.mu.Lock()
	m.activeAlerts["live-known"] = &Alert{ID: "alert-2", ResourceID: "vm-alert-2",
		StartTime: time.Date(2026, 9, 1, 2, 0, 0, 0, time.UTC), LastSeen: times[len(times)-1]}
	m.mu.Unlock()
	for _, since := range []time.Time{{}, times[30], times[100]} {
		first, _ := m.AlertHistoryFromEvents(since, 0)
		before := referenceWindowedHistory(t, m, since, 0)
		for i := range first {
			first[i].Value = -1
			first[i].LastSeen = time.Time{}
			if first[i].AckTime != nil {
				*first[i].AckTime = time.Time{}
			}
		}
		if again, _ := m.AlertHistoryFromEvents(since, 0); !reflect.DeepEqual(again, before) {
			t.Fatalf("since=%s: mutating a result changed the next read", since)
		}
	}
}

// A limited read copies only the rows it returns, not the whole fold.
func TestLimitedHistoryReadCopiesOnlyReturnedRows(t *testing.T) {
	m := newHistoryParityManager(t)
	store := m.eventLogStore()
	at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	metadata := map[string]interface{}{}
	for i := 0; i < 40; i++ {
		metadata[fmt.Sprintf("key-%d", i)] = fmt.Sprintf("value-%d-with-some-detail", i)
	}
	for i := 0; i < 2000; i++ {
		occurred := at.Add(time.Duration(i) * time.Minute)
		snapshot, err := json.Marshal(Alert{ID: fmt.Sprintf("alert-%d", i), ResourceID: "vm", StartTime: occurred,
			LastSeen: occurred, Message: "disk usage above threshold on a long resource name", Metadata: metadata})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.AppendDurable(eventlog.Event{Type: eventlog.TypeFired, AlertID: fmt.Sprintf("alert-%d", i),
			OccurredAt: occurred, Snapshot: snapshot}); err != nil {
			t.Fatal(err)
		}
	}
	allocated := func(limit int) uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		if got, ok := m.AlertHistoryFromEvents(time.Time{}, limit); !ok || (limit > 0 && len(got) != limit) {
			t.Fatalf("history read with limit %d failed", limit)
		}
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}
	allocated(0) // build the shared fold
	everything, ten := allocated(0), allocated(10)
	if ten*5 > everything {
		t.Fatalf("a 10-row read allocated %d bytes against %d for all 2000 rows; it must not copy the whole fold", ten, everything)
	}
}
