package unifiedresources

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestExistingStoreIndexesGlobalRecentHistory(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	// Reopen an existing canonical database without the global time index,
	// matching installations that already have resource-specific indexes.
	if _, err := store.db.Exec("DROP INDEX idx_resource_changes_time"); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rows, err := store.db.Query("EXPLAIN QUERY PLAN SELECT id FROM resource_changes WHERE observed_at >= ? ORDER BY observed_at DESC LIMIT 256", time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	indexed := false
	defer rows.Close()
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(detail, "TEMP B-TREE") || strings.Contains(detail, "SCAN resource_changes") {
			t.Fatalf("bounded global history scans or sorts retained history: %s", detail)
		}
		indexed = indexed || strings.Contains(detail, "idx_resource_changes_time")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if !indexed {
		t.Fatal("global history query did not use its chronological index")
	}
	at := time.Now().UTC()
	for i, offset := range []time.Duration{3 * time.Minute, time.Minute, 2 * time.Minute} {
		if err := store.RecordChange(ResourceChange{ID: fmt.Sprintf("history-%d", i), ResourceID: fmt.Sprintf("host-%d", i), ObservedAt: at.Add(-offset), Kind: ChangeKind("status_changed")}); err != nil {
			t.Fatal(err)
		}
	}
	changes, err := store.GetRecentChanges("", at.Add(-time.Hour), 2)
	if err != nil || len(changes) != 2 || changes[0].ID != "history-1" || changes[1].ID != "history-2" {
		t.Fatalf("bounded cross-resource chronology changed: %+v error=%v", changes, err)
	}
}
