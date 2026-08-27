package eventlog

import (
	"math"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory store: %v", err)
	}
	t.Cleanup(store.Close)
	return store
}

func TestAppendAndQueryRoundTrip(t *testing.T) {
	store := newTestStore(t)
	occurred := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)

	store.Append(Event{
		OccurredAt:   occurred,
		Type:         TypeNotificationSuppressed,
		AlertID:      "alert-1",
		ResourceID:   "node/pve-1",
		ResourceName: "pve-1",
		AlertType:    "cpu",
		Level:        "warning",
		Reason:       "flapping",
		Message:      "Alert suppressed due to flapping",
		Details:      map[string]string{"trackingKey": "node/pve-1/cpu"},
	})
	store.Flush()

	events, err := store.Query(Filter{AlertID: "alert-1"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	event := events[0]
	if event.Type != TypeNotificationSuppressed || event.Reason != "flapping" {
		t.Fatalf("event = %+v, want suppressed/flapping", event)
	}
	if !event.OccurredAt.Equal(occurred) {
		t.Fatalf("OccurredAt = %v, want %v", event.OccurredAt, occurred)
	}
	if event.Details["trackingKey"] != "node/pve-1/cpu" {
		t.Fatalf("Details = %v, want trackingKey preserved", event.Details)
	}
}

func TestQueryFiltersByTypeAndTime(t *testing.T) {
	store := newTestStore(t)
	base := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)

	store.Append(Event{OccurredAt: base, Type: TypeResolved, AlertID: "a1"})
	store.Append(Event{OccurredAt: base.Add(time.Minute), Type: TypeAcknowledged, AlertID: "a1"})
	store.Append(Event{OccurredAt: base.Add(2 * time.Minute), Type: TypeResolved, AlertID: "a2"})
	store.Flush()

	byType, err := store.Query(Filter{Types: []string{TypeResolved}})
	if err != nil {
		t.Fatalf("query by type: %v", err)
	}
	if len(byType) != 2 {
		t.Fatalf("len(byType) = %d, want 2", len(byType))
	}
	// Newest first.
	if byType[0].AlertID != "a2" || byType[1].AlertID != "a1" {
		t.Fatalf("order = %s,%s, want a2,a1", byType[0].AlertID, byType[1].AlertID)
	}

	windowed, err := store.Query(Filter{Since: base.Add(30 * time.Second), Until: base.Add(90 * time.Second)})
	if err != nil {
		t.Fatalf("query by window: %v", err)
	}
	if len(windowed) != 1 || windowed[0].Type != TypeAcknowledged {
		t.Fatalf("windowed = %+v, want the acknowledged event only", windowed)
	}
}

func TestQueryLimitCaps(t *testing.T) {
	store := newTestStore(t)
	base := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		store.Append(Event{OccurredAt: base.Add(time.Duration(i) * time.Second), Type: TypeResolved, AlertID: "a1"})
	}
	store.Flush()

	events, err := store.Query(Filter{Limit: 3})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}
}

func TestQueryOversizedLimitDoesNotControlAllocation(t *testing.T) {
	store := newTestStore(t)
	store.Append(Event{OccurredAt: time.Now(), Type: TypeResolved, AlertID: "a1"})
	store.Flush()

	events, err := store.Query(Filter{Limit: math.MaxInt})
	if err != nil {
		t.Fatalf("query with oversized limit: %v", err)
	}
	if len(events) != 1 || events[0].AlertID != "a1" {
		t.Fatalf("events = %+v, want the one stored event", events)
	}
}

func TestNilStoreIsSafe(t *testing.T) {
	var store *Store
	store.Append(Event{Type: TypeResolved, AlertID: "a1"})
	store.Flush()
	store.Close()
	if got := store.Dropped(); got != 0 {
		t.Fatalf("Dropped() = %d, want 0", got)
	}
	events, err := store.Query(Filter{})
	if err != nil || events != nil {
		t.Fatalf("Query = %v, %v, want nil, nil", events, err)
	}
}

func TestCloseDrainsBufferedEvents(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory store: %v", err)
	}
	for i := 0; i < 50; i++ {
		store.Append(Event{Type: TypeNotificationDispatched, AlertID: "a1"})
	}
	store.Close()

	// Query after Close is not supported; reopen semantics do not apply to
	// the in-memory store, so assert via the counters instead.
	if store.Dropped() != 0 {
		t.Fatalf("Dropped() = %d, want 0", store.Dropped())
	}
	if store.written.Load() != 50 {
		t.Fatalf("written = %d, want 50", store.written.Load())
	}
}

func TestOpenRequiresExistingDirectory(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("Open(\"\") should fail")
	}
	if _, err := Open(t.TempDir() + "/missing"); err == nil {
		t.Fatal("Open on a missing directory should fail")
	}

	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(%q): %v", dir, err)
	}
	store.Append(Event{Type: TypeResolved, AlertID: "a1"})
	store.Flush()
	events, err := store.Query(Filter{})
	if err != nil || len(events) != 1 {
		t.Fatalf("Query = %v, %v, want one event", events, err)
	}
	store.Close()
}
