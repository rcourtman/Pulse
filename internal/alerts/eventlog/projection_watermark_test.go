package eventlog

import (
	"testing"
	"time"
)

func TestProjectionWatermarkRoundTripAndReset(t *testing.T) {
	store := newTestStore(t)

	if got := store.ProjectionWatermark("timelines"); got != 0 {
		t.Fatalf("unset watermark = %d, want 0", got)
	}
	if err := store.SetProjectionWatermark("timelines", 42); err != nil {
		t.Fatalf("set watermark: %v", err)
	}
	if got := store.ProjectionWatermark("timelines"); got != 42 {
		t.Fatalf("watermark = %d, want 42", got)
	}
	if err := store.SetProjectionWatermark("timelines", 99); err != nil {
		t.Fatalf("advance watermark: %v", err)
	}
	if got := store.ProjectionWatermark("timelines"); got != 99 {
		t.Fatalf("advanced watermark = %d, want 99", got)
	}
	// Consumers are independent.
	if got := store.ProjectionWatermark("other"); got != 0 {
		t.Fatalf("unrelated consumer watermark = %d, want 0", got)
	}
	// Lowering (including to zero) is allowed so a rebuilt projection store
	// can force a full replay.
	if err := store.SetProjectionWatermark("timelines", 0); err != nil {
		t.Fatalf("reset watermark: %v", err)
	}
	if got := store.ProjectionWatermark("timelines"); got != 0 {
		t.Fatalf("reset watermark = %d, want 0", got)
	}

	if err := store.SetProjectionWatermark("", 5); err == nil {
		t.Fatal("set watermark with empty name should fail")
	}
	if err := store.SetProjectionWatermark("timelines", -1); err == nil {
		t.Fatal("set negative watermark should fail")
	}

	var nilStore *Store
	if got := nilStore.ProjectionWatermark("timelines"); got != 0 {
		t.Fatalf("nil store watermark = %d, want 0", got)
	}
	if err := nilStore.SetProjectionWatermark("timelines", 7); err != nil {
		t.Fatalf("nil store set watermark should be a no-op, got %v", err)
	}
}

func TestProjectionWatermarkSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.SetProjectionWatermark("timelines", 17); err != nil {
		store.Close()
		t.Fatalf("set watermark: %v", err)
	}
	store.Close()

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(reopened.Close)
	if got := reopened.ProjectionWatermark("timelines"); got != 17 {
		t.Fatalf("watermark after reopen = %d, want 17", got)
	}
}

func TestWalkOldestAfterIDVisitsOnlyTail(t *testing.T) {
	store := newTestStore(t)
	base := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		store.Append(Event{OccurredAt: base.Add(time.Duration(i) * time.Minute), Type: TypeFired, AlertID: "a1"})
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	var all []int64
	if err := store.WalkOldest(Filter{Types: []string{TypeFired}}, func(event Event) error {
		all = append(all, event.ID)
		return nil
	}); err != nil {
		t.Fatalf("walk all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("walked %d events, want 3", len(all))
	}

	var tail []int64
	if err := store.WalkOldest(Filter{AfterID: all[0], Types: []string{TypeFired}}, func(event Event) error {
		tail = append(tail, event.ID)
		return nil
	}); err != nil {
		t.Fatalf("walk tail: %v", err)
	}
	if len(tail) != 2 || tail[0] != all[1] || tail[1] != all[2] {
		t.Fatalf("tail = %v, want %v", tail, all[1:])
	}

	var none []int64
	if err := store.WalkOldest(Filter{AfterID: all[2], Types: []string{TypeFired}}, func(event Event) error {
		none = append(none, event.ID)
		return nil
	}); err != nil {
		t.Fatalf("walk beyond newest: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("walk beyond newest visited %v, want none", none)
	}
}
