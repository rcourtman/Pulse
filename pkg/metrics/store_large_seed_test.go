package metrics

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// Exercise the ingestion shape from the workloads-summary benchmark without
// its HTTP, reflection or monitor fixtures. A historical benchmark crashed in
// SQLite during the second synchronous seed; this is a diagnostic invariant,
// not a reproducer or a claim that the unexplained crash has been repaired.
func TestStoreLargeSummarySeedSurvivesReopen(t *testing.T) {
	cfg := DefaultConfig(t.TempDir())
	cfg.DBPath = filepath.Join(filepath.Dir(cfg.DBPath), "summary-seed.db")
	cfg.FlushInterval = time.Hour
	cfg.WriteBufferSize = 10_000
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if store != nil {
			if err := store.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	base := time.Now().Add(-4 * time.Hour).UTC().Truncate(time.Second)
	for _, group := range []struct {
		kind  string
		count int
	}{{"vm", 30}, {"container", 20}, {"dockerContainer", 20}} {
		batch := make([]WriteMetric, 0, group.count*5*240)
		for r := 0; r < group.count; r++ {
			for _, metric := range []string{"cpu", "memory", "disk", "netin", "netout"} {
				for p := 0; p < 240; p++ {
					batch = append(batch, WriteMetric{
						ResourceType: group.kind, ResourceID: fmt.Sprintf("%s-%d", group.kind, r),
						MetricType: metric, Value: float64((r + p) % 100),
						Timestamp: base.Add(time.Duration(p) * time.Minute), Tier: TierMinute,
					})
				}
			}
		}
		store.WriteBatchSync(batch)
	}
	check := func() {
		t.Helper()
		var count int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM metrics WHERE tier = ?", string(TierMinute)).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 84_000 {
			t.Fatalf("persisted minute rows = %d, want 84000", count)
		}
		rows, err := store.db.Query("PRAGMA integrity_check")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var results int
		for rows.Next() {
			var result string
			if err := rows.Scan(&result); err != nil {
				t.Fatal(err)
			}
			if result != "ok" {
				t.Errorf("integrity_check: %s", result)
			}
			results++
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if results != 1 {
			t.Fatalf("integrity_check returned %d rows, want one ok", results)
		}
	}
	check()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = nil
	store, err = NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WaitForMaintenance(10 * time.Second); err != nil {
		t.Fatal(err)
	}
	check()
}
