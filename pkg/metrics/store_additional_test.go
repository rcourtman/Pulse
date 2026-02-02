package metrics

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreWriteBatchSync(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	ts := time.Unix(1000, 0)
	metrics := []WriteMetric{
		{ResourceType: "vm", ResourceID: "v1", MetricType: "cpu", Value: 10.0, Timestamp: ts, Tier: TierRaw},
		{ResourceType: "vm", ResourceID: "v1", MetricType: "mem", Value: 50.0, Timestamp: ts, Tier: TierRaw},
	}

	store.WriteBatchSync(metrics)

	// Verify data was written
	points, err := store.Query("vm", "v1", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(points) != 1 || points[0].Value != 10.0 {
		t.Fatalf("expected 1 point with value 10.0, got %v", points)
	}
}

func TestStoreClear(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	// Write some data
	store.Write("vm", "v1", "cpu", 10.0, time.Now())
	store.Flush()

	// Write some data
	store.Write("vm", "v1", "cpu", 10.0, time.Now())
	store.Flush()

	// Wait for data to be written (Flush is async via channel)
	deadline := time.Now().Add(2 * time.Second)
	var stats Stats
	for time.Now().Before(deadline) {
		stats = store.GetStats()
		if stats.RawCount > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if stats.RawCount == 0 {
		t.Fatal("expected data to be written before clearing")
	}

	// clear
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify empty
	stats = store.GetStats()
	if stats.RawCount != 0 {
		t.Fatalf("expected empty store, got %d raw records", stats.RawCount)
	}
}

func TestStoreSetMaxOpenConns(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	// Just verifying it doesn't panic
	store.SetMaxOpenConns(5)
}

func TestStoreRunRollupManually(t *testing.T) {
	// Tests the runRollup function wrapper which was showing 0% coverage
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	// Create separate DB for this test
	cfg.DBPath = filepath.Join(dir, "metrics-rollup-manual.db")

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	// Trigger manual rollup - should not panic
	store.runRollup()
}

func TestStoreGetMetaIntInvalid(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	// Insert invalid int value
	_, err = store.db.Exec("INSERT INTO metrics_meta (key, value) VALUES (?, ?)", "bad_key", "not_an_int")
	if err != nil {
		t.Fatalf("failed to insert invalid meta: %v", err)
	}

	val, ok := store.getMetaInt("bad_key")
	if ok {
		t.Fatalf("expected getMetaInt to fail for invalid int, got %d", val)
	}
}
