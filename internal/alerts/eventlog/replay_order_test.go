package eventlog

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// recentBase keeps fixtures inside the 90-day retention window: the writer
// prunes older rows when it starts, which would race the fixture import.
func recentBase() time.Time {
	return time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second).Add(123456789)
}

// importDisordered writes n events whose occurrence times do not follow their
// durable IDs: even positions carry later times than odd positions after them,
// as a legacy-history import does for long-running occurrences.
func importDisordered(t *testing.T, store *Store, n int, eventType string) []Event {
	t.Helper()
	base := recentBase()
	events := make([]Event, n)
	for i := range events {
		at := base.Add(time.Duration(i) * time.Second)
		if i%2 == 0 {
			at = base.Add(time.Duration(n+i) * time.Second)
		}
		events[i] = Event{OccurredAt: at, Type: eventType, AlertID: fmt.Sprintf("alert-%d", i%5), ResourceID: fmt.Sprintf("res-%d", i)}
	}
	for start := 0; start < n; start += 500 {
		end := min(start+500, n)
		if err := store.ImportEvents(append([]Event(nil), events[start:end]...)); err != nil {
			t.Fatalf("import events: %v", err)
		}
	}
	return events
}

func walkIDs(t *testing.T, store *Store, filter Filter) []Event {
	t.Helper()
	var visited []Event
	if err := store.WalkByID(filter, func(event Event) error {
		visited = append(visited, event)
		return nil
	}); err != nil {
		t.Fatalf("WalkByID(%+v): %v", filter, err)
	}
	return visited
}

func requireAscendingIDs(t *testing.T, events []Event) {
	t.Helper()
	for i := 1; i < len(events); i++ {
		if events[i].ID <= events[i-1].ID {
			t.Fatalf("event %d has id %d after id %d; want strictly ascending durable ids", i, events[i].ID, events[i-1].ID)
		}
	}
}

func TestWalkByIDFollowsDurableIDsWhileWalkOldestStaysChronological(t *testing.T) {
	store := newTestStore(t)
	written := importDisordered(t, store, 9, TypeAcknowledged)

	byID := walkIDs(t, store, Filter{})
	if len(byID) != len(written) {
		t.Fatalf("WalkByID visited %d events, want %d", len(byID), len(written))
	}
	requireAscendingIDs(t, byID)
	for i, event := range byID {
		// Immutable timestamps: the walk returns each event's stored time,
		// not a time derived from its position.
		if !event.OccurredAt.Equal(written[i].OccurredAt) || event.ResourceID != written[i].ResourceID {
			t.Fatalf("id %d returned (%s, %s), want stored (%s, %s)", event.ID,
				event.OccurredAt, event.ResourceID, written[i].OccurredAt, written[i].ResourceID)
		}
	}

	var chronological []Event
	if err := store.WalkOldest(Filter{}, func(event Event) error {
		chronological = append(chronological, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(chronological) != len(written) {
		t.Fatalf("WalkOldest visited %d events, want %d", len(chronological), len(written))
	}
	for i := 1; i < len(chronological); i++ {
		if chronological[i].OccurredAt.Before(chronological[i-1].OccurredAt) {
			t.Fatalf("WalkOldest is no longer chronological at position %d", i)
		}
	}
	if chronological[0].ID == byID[0].ID {
		t.Fatal("fixture is not disordered: both walks start with the same event")
	}
}

func TestWalkByIDAppliesEveryFilterAndLimit(t *testing.T) {
	store := newTestStore(t)
	base := recentBase()
	types := []string{TypeFired, TypeNotificationDispatched, TypeResolved, TypeEscalated}
	var events []Event
	for i := 0; i < 60; i++ {
		// Reverse time against ID so every bound is exercised on disordered data.
		events = append(events, Event{
			OccurredAt: base.Add(time.Duration(60-i) * time.Minute),
			Type:       types[i%len(types)],
			AlertID:    fmt.Sprintf("alert-%d", i%3),
		})
	}
	if err := store.ImportEvents(events); err != nil {
		t.Fatal(err)
	}
	all := walkIDs(t, store, Filter{})
	if len(all) != len(events) {
		t.Fatalf("unfiltered walk visited %d, want %d", len(all), len(events))
	}

	since, until := base.Add(15*time.Minute), base.Add(45*time.Minute)
	afterID, throughID := all[9].ID, all[49].ID
	cases := []struct {
		name   string
		filter Filter
		keep   func(Event) bool
		limit  int
	}{
		{"types", Filter{Types: []string{TypeFired, TypeResolved}}, func(e Event) bool { return e.Type == TypeFired || e.Type == TypeResolved }, 0},
		{"alert", Filter{AlertID: " alert-1 "}, func(e Event) bool { return e.AlertID == "alert-1" }, 0},
		{"since", Filter{Since: since}, func(e Event) bool { return !e.OccurredAt.Before(since) }, 0},
		{"until", Filter{Until: until}, func(e Event) bool { return !e.OccurredAt.After(until) }, 0},
		{"after", Filter{AfterID: afterID}, func(e Event) bool { return e.ID > afterID }, 0},
		{"through", Filter{ThroughID: throughID}, func(e Event) bool { return e.ID <= throughID }, 0},
		{"combined", Filter{Types: []string{TypeFired, TypeNotificationDispatched}, AlertID: "alert-2", Since: since, Until: until, AfterID: afterID, ThroughID: throughID},
			func(e Event) bool {
				return (e.Type == TypeFired || e.Type == TypeNotificationDispatched) && e.AlertID == "alert-2" &&
					!e.OccurredAt.Before(since) && !e.OccurredAt.After(until) && e.ID > afterID && e.ID <= throughID
			}, 0},
		{"limit", Filter{Types: []string{TypeResolved}, Limit: 4}, func(e Event) bool { return e.Type == TypeResolved }, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var want []int64
			for _, event := range all {
				if tc.keep(event) {
					want = append(want, event.ID)
				}
			}
			if tc.limit > 0 && len(want) > tc.limit {
				want = want[:tc.limit]
			}
			got := walkIDs(t, store, tc.filter)
			if len(got) != len(want) || len(want) == 0 {
				t.Fatalf("visited %d events, want %d (non-empty)", len(got), len(want))
			}
			for i := range want {
				if got[i].ID != want[i] {
					t.Fatalf("position %d visited id %d, want %d", i, got[i].ID, want[i])
				}
			}
		})
	}
}

func TestWalkByIDPagesAcrossKeysetBoundaries(t *testing.T) {
	for _, n := range []int{maxQueryLimit - 1, maxQueryLimit, maxQueryLimit + 1, 2*maxQueryLimit + 1} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			store := newTestStore(t)
			importDisordered(t, store, n, TypeAcknowledged)
			visited := walkIDs(t, store, Filter{})
			if len(visited) != n {
				t.Fatalf("visited %d events, want %d", len(visited), n)
			}
			requireAscendingIDs(t, visited)
			for _, limit := range []int{maxQueryLimit, maxQueryLimit + 1} {
				want := min(limit, n)
				if got := len(walkIDs(t, store, Filter{Limit: limit})); got != want {
					t.Fatalf("limit %d visited %d, want %d", limit, got, want)
				}
			}
		})
	}

	// Sparse matches must keep paging past ID ranges that hold no matches.
	store := newTestStore(t)
	var events []Event
	for i := 0; i < 3*maxQueryLimit+7; i++ {
		eventType := TypeNotificationDispatched
		if i%3 == 0 {
			eventType = TypeAcknowledged
		}
		events = append(events, Event{OccurredAt: recentBase().Add(-time.Duration(i) * time.Second), Type: eventType, AlertID: "sparse"})
	}
	if err := store.ImportEvents(events); err != nil {
		t.Fatal(err)
	}
	sparse := walkIDs(t, store, Filter{Types: []string{TypeAcknowledged}})
	if want := (len(events) + 2) / 3; len(sparse) != want {
		t.Fatalf("sparse walk visited %d, want %d", len(sparse), want)
	}
	requireAscendingIDs(t, sparse)
}

func TestWalkByIDReleasesEachPageReaderBeforeVisiting(t *testing.T) {
	// The in-memory store has exactly one connection for reads and writes, so
	// any database call from a visitor would block forever if the page cursor
	// were still open.
	store := newTestStore(t)
	importDisordered(t, store, maxQueryLimit+3, TypeAcknowledged)
	done := make(chan error, 1)
	go func() {
		visits := 0
		done <- store.WalkByID(Filter{}, func(event Event) error {
			visits++
			if _, err := store.ReplayBoundary(); err != nil {
				return err
			}
			if visits == 1 || visits == maxQueryLimit+1 {
				return store.AppendDurable(Event{Type: TypeNotificationDispatched, AlertID: "from-visitor"})
			}
			return nil
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("walk with database work in its visitor: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("visitor blocked on the store: the page reader was still open during callbacks")
	}
}

func TestWalkByIDStopsAtCapturedHighWaterDuringConcurrentAppends(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	importDisordered(t, store, 2*maxQueryLimit+5, TypeAcknowledged)
	boundary, err := store.ReplayBoundary()
	if err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	var writers sync.WaitGroup
	var background int
	writers.Add(1)
	go func() {
		defer writers.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if err := store.AppendDurable(Event{Type: TypeAcknowledged, AlertID: "concurrent"}); err != nil {
				t.Errorf("concurrent append: %v", err)
				return
			}
			background++
		}
	}()

	inVisitor := 0
	visited := 0
	done := make(chan error, 1)
	go func() {
		done <- store.WalkByID(Filter{}, func(event Event) error {
			if event.ID > boundary.LastID {
				return fmt.Errorf("visited id %d beyond the high-water mark %d captured at start", event.ID, boundary.LastID)
			}
			visited++
			// Every visit commits another matching event. Without a captured
			// high-water mark this walk would never finish.
			inVisitor++
			return store.AppendDurable(Event{Type: TypeAcknowledged, AlertID: "visitor"})
		})
	}()
	var walkErr error
	select {
	case walkErr = <-done:
	case <-time.After(60 * time.Second):
		walkErr = errors.New("walk did not finish while appends continued")
	}
	close(stop)
	writers.Wait()
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	if visited != int(boundary.LastID) {
		t.Fatalf("visited %d events, want exactly the %d committed before the walk", visited, boundary.LastID)
	}

	// The next pass from the stored cursor picks up everything appended since.
	tail := walkIDs(t, store, Filter{AfterID: boundary.LastID})
	if want := inVisitor + background; len(tail) != want {
		t.Fatalf("next pass visited %d events, want the %d appended during the first", len(tail), want)
	}
	requireAscendingIDs(t, tail)
}

func TestWalkByIDReturnsVisitorErrorAndStops(t *testing.T) {
	store := newTestStore(t)
	importDisordered(t, store, 2*maxQueryLimit, TypeAcknowledged)
	stopAt := maxQueryLimit + 3 // inside the second page
	sentinel := errors.New("projection failed")
	visits := 0
	err := store.WalkByID(Filter{}, func(event Event) error {
		visits++
		if visits == stopAt {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WalkByID returned %v, want the visitor's error", err)
	}
	if visits != stopAt {
		t.Fatalf("visited %d events after the error, want the walk to stop at %d", visits, stopAt)
	}
}

func TestWalkByIDSeeksPrimaryKeyWithoutSorting(t *testing.T) {
	store := newTestStore(t)
	importDisordered(t, store, 50, TypeAcknowledged)
	query, args := idPageQuery(Filter{Types: []string{TypeFired, TypeAcknowledged}, AlertID: "alert-1",
		Since: time.Unix(0, 0)}, 7, 40, maxQueryLimit)
	rows, err := store.readDB.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	joined := strings.Join(plan, " | ")
	if !strings.Contains(joined, "INTEGER PRIMARY KEY") || strings.Contains(joined, "TEMP B-TREE") {
		t.Fatalf("id page plan %q must seek the primary key range without a sort", joined)
	}
}

func TestWalkByIDNilStoreAndVisitor(t *testing.T) {
	var nilStore *Store
	if err := nilStore.WalkByID(Filter{}, func(Event) error { return errors.New("unexpected visit") }); err != nil {
		t.Fatalf("nil store walk: %v", err)
	}
	if err := newTestStore(t).WalkByID(Filter{}, nil); err == nil {
		t.Fatal("WalkByID accepted a nil visitor")
	}
}
