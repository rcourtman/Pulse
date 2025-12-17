package metrics

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreWriteBatchAndQuery(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics-test.db")
	cfg.FlushInterval = time.Hour
	cfg.RetentionRaw = 10 * time.Second
	cfg.RetentionMinute = 20 * time.Second
	cfg.RetentionHourly = 30 * time.Second
	cfg.RetentionDaily = 40 * time.Second

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	ts := time.Unix(1000, 0)
	store.writeBatch([]bufferedMetric{
		{resourceType: "vm", resourceID: "vm-101", metricType: "cpu", value: 1.5, timestamp: ts},
		{resourceType: "vm", resourceID: "vm-101", metricType: "cpu", value: 2.5, timestamp: ts.Add(1 * time.Second)},
	})

	points, err := store.Query("vm", "vm-101", "cpu", ts.Add(-1*time.Second), ts.Add(2*time.Second))
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	if points[0].Value != 1.5 || points[1].Value != 2.5 {
		t.Fatalf("unexpected query values: %+v", points)
	}

	all, err := store.QueryAll("vm", "vm-101", ts.Add(-1*time.Second), ts.Add(2*time.Second))
	if err != nil {
		t.Fatalf("QueryAll returned error: %v", err)
	}
	if len(all["cpu"]) != 2 {
		t.Fatalf("expected QueryAll to return 2 cpu points, got %+v", all)
	}
}

func TestStoreSelectTierAndStats(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics-test.db")
	cfg.FlushInterval = time.Hour
	cfg.RetentionRaw = 10 * time.Second
	cfg.RetentionMinute = 20 * time.Second
	cfg.RetentionHourly = 30 * time.Second
	cfg.RetentionDaily = 40 * time.Second

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	if store.selectTier(5*time.Second) != TierRaw {
		t.Fatalf("expected raw tier")
	}
	if store.selectTier(15*time.Second) != TierMinute {
		t.Fatalf("expected minute tier")
	}
	if store.selectTier(25*time.Second) != TierHourly {
		t.Fatalf("expected hourly tier")
	}
	if store.selectTier(35*time.Second) != TierDaily {
		t.Fatalf("expected daily tier")
	}

	// Insert one point for each tier to verify stats aggregation.
	ts := int64(1000)
	_, err = store.db.Exec(
		`INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier) VALUES
		('vm','vm-101','cpu',1.0,?, 'raw'),
		('vm','vm-101','cpu',2.0,?, 'minute'),
		('vm','vm-101','cpu',3.0,?, 'hourly'),
		('vm','vm-101','cpu',4.0,?, 'daily')`,
		ts, ts, ts, ts,
	)
	if err != nil {
		t.Fatalf("insert metrics returned error: %v", err)
	}

	stats := store.GetStats()
	if stats.RawCount != 1 || stats.MinuteCount != 1 || stats.HourlyCount != 1 || stats.DailyCount != 1 {
		t.Fatalf("unexpected tier counts: %+v", stats)
	}
	if stats.DBPath == "" || stats.DBSize <= 0 {
		t.Fatalf("expected stats DB info to be populated: %+v", stats)
	}
}
