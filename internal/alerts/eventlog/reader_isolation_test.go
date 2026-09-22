package eventlog

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestHistoryReaderCannotBlockDurableLifecycleWrite(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.readDB.SetMaxOpenConns(1)
	tx, err := store.readDB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var before int
	if err := tx.QueryRow("SELECT COUNT(*) FROM alert_events").Scan(&before); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- store.AppendDurable(Event{OccurredAt: time.Now(), Type: TypeFired, AlertID: "during-history-read"})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("an open history snapshot blocked the durable writer")
	}
	var during int
	if err := tx.QueryRow("SELECT COUNT(*) FROM alert_events").Scan(&during); err != nil || during != before {
		t.Fatalf("reader snapshot changed: count=%d before=%d error=%v", during, before, err)
	}
	if _, err := tx.Exec("DELETE FROM alert_events"); err == nil {
		t.Fatal("history reader accepted a mutation")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	events, err := store.Query(Filter{AlertID: "during-history-read"})
	if err != nil || len(events) != 1 {
		t.Fatalf("committed event unavailable to new reader: count=%d error=%v", len(events), err)
	}
}

func TestHistoryReplayBoundaryExcludesLaterAppendsAndTracksRetention(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	at := time.Now().UTC().Add(-time.Hour)
	if err := store.AppendDurable(Event{OccurredAt: at, Type: TypeFired, AlertID: "old"}); err != nil {
		t.Fatal(err)
	}
	before, err := store.ReplayBoundary()
	if err != nil || before.LastID == 0 {
		t.Fatalf("initial boundary: %+v, %v", before, err)
	}
	if err := store.AppendDurable(Event{OccurredAt: at.Add(time.Minute), Type: TypeResolved, AlertID: "new"}); err != nil {
		t.Fatal(err)
	}
	var ids []int64
	if err := store.WalkOldest(Filter{ThroughID: before.LastID}, func(e Event) error {
		ids = append(ids, e.ID)
		return nil
	}); err != nil || len(ids) != 1 || ids[0] != before.LastID {
		t.Fatalf("bounded replay: %v, %v", ids, err)
	}
	appended, err := store.ReplayBoundary()
	if err != nil || appended.LastID <= before.LastID || appended.RetentionRevision != before.RetentionRevision {
		t.Fatalf("append boundary: %+v, %v", appended, err)
	}
	store.pruneEventsBefore(at.Add(time.Second))
	pruned, err := store.ReplayBoundary()
	if err != nil || pruned.LastID != appended.LastID || pruned.RetentionRevision <= appended.RetentionRevision {
		t.Fatalf("old-row removal must invalidate history without changing its newest ID: %+v, %v", pruned, err)
	}
	store.pruneEventsBefore(at.Add(time.Second))
	unchanged, err := store.ReplayBoundary()
	if err != nil || unchanged != pruned {
		t.Fatalf("empty prune changed boundary: %+v, %v", unchanged, err)
	}
}

func TestDiskHistoryWalkPreservesFilteredPagesAndCursor(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	at := time.Now().UTC().Add(-time.Hour)
	events := make([]Event, 0, 2406)
	for i := 0; i < 1203; i++ {
		for _, kind := range []string{TypeResolved, TypeNotificationSuppressed} {
			events = append(events, Event{OccurredAt: at, Type: kind, AlertID: fmt.Sprintf("event-%04d", i)})
		}
	}
	if err := store.ImportEvents(events); err != nil {
		t.Fatal(err)
	}
	var seen []Event
	if err := store.WalkOldest(Filter{Types: []string{TypeResolved}, AfterID: 2, Limit: 1100}, func(e Event) error {
		seen = append(seen, e)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1100 || seen[0].AlertID != "event-0001" || seen[1099].AlertID != "event-1100" {
		t.Fatalf("filtered page walk lost ordering or bounds: count=%d", len(seen))
	}
	for i := 1; i < len(seen); i++ {
		if seen[i].ID <= seen[i-1].ID || seen[i].Type != TypeResolved {
			t.Fatalf("page repeated or admitted a filtered event at %d", i)
		}
	}
}

func TestFilteredHistoryPagesSeekTheirExplicitBounds(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, tc := range []struct {
		name   string
		filter Filter
		seek   string
	}{
		{"replay watermark", Filter{AfterID: 50000, Types: []string{TypeFired, TypeResolved}}, "SEARCH alert_events USING INTEGER PRIMARY KEY (rowid>?)"},
		{"one alert", Filter{AlertID: "incident", Types: []string{TypeFired, TypeResolved}}, "SEARCH alert_events USING INDEX idx_alert_events_alert (alert_id=?)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query, args := historyPageQuery(tc.filter, "", 0, maxQueryLimit)
			rows, err := store.readDB.Query("EXPLAIN QUERY PLAN "+query, args...)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			seek := false
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(detail, "SCAN alert_events") {
					t.Fatalf("filtered replay scans retained history: %s", detail)
				}
				seek = seek || strings.Contains(detail, tc.seek)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !seek {
				t.Fatalf("history page did not seek its explicit bound: %s", tc.seek)
			}
		})
	}
}
