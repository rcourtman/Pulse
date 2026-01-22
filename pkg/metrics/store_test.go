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

	points, err := store.Query("vm", "vm-101", "cpu", ts.Add(-1*time.Second), ts.Add(2*time.Second), 0)
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	if points[0].Value != 1.5 || points[1].Value != 2.5 {
		t.Fatalf("unexpected query values: %+v", points)
	}

	all, err := store.QueryAll("vm", "vm-101", ts.Add(-1*time.Second), ts.Add(2*time.Second), 0)
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
	if stats.DBSize <= 0 {
		t.Fatalf("expected stats DB info to be populated: %+v", stats)
	}
}

func TestStoreRollupTier(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics-rollup.db")
	cfg.FlushInterval = time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	base := time.Now().Add(-2 * time.Minute).Truncate(time.Minute)
	ts := base.Unix()

	_, err = store.db.Exec(
		`INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier) VALUES
		('vm','vm-101','cpu',1.0,?, 'raw'),
		('vm','vm-101','cpu',3.0,?, 'raw')`,
		ts, base.Add(10*time.Second).Unix(),
	)
	if err != nil {
		t.Fatalf("insert metrics returned error: %v", err)
	}

	store.rollupTier(TierRaw, TierMinute, time.Minute, 0)

	var countRaw int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics WHERE tier = 'raw'`).Scan(&countRaw); err != nil {
		t.Fatalf("query raw count: %v", err)
	}
	if countRaw != 0 {
		t.Fatalf("expected raw metrics to be rolled up, got %d", countRaw)
	}

	var value, minValue, maxValue float64
	var bucketTs int64
	if err := store.db.QueryRow(
		`SELECT value, min_value, max_value, timestamp FROM metrics WHERE tier = 'minute'`,
	).Scan(&value, &minValue, &maxValue, &bucketTs); err != nil {
		t.Fatalf("query minute tier: %v", err)
	}

	expectedBucket := (ts / 60) * 60
	if bucketTs != expectedBucket {
		t.Fatalf("expected bucket %d, got %d", expectedBucket, bucketTs)
	}
	if value != 2.0 || minValue != 1.0 || maxValue != 3.0 {
		t.Fatalf("unexpected rollup values: value=%v min=%v max=%v", value, minValue, maxValue)
	}
}

func TestStoreRetentionPrunesOldData(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics-retention.db")
	cfg.RetentionRaw = time.Minute
	cfg.RetentionMinute = time.Minute
	cfg.RetentionHourly = time.Minute
	cfg.RetentionDaily = time.Minute
	cfg.FlushInterval = time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	oldTs := time.Now().Add(-2 * time.Hour).Unix()
	newTs := time.Now().Unix()

	_, err = store.db.Exec(
		`INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier) VALUES
		('vm','vm-101','cpu',1.0,?, 'raw'),
		('vm','vm-101','cpu',2.0,?, 'minute'),
		('vm','vm-101','cpu',3.0,?, 'hourly'),
		('vm','vm-101','cpu',4.0,?, 'daily'),
		('vm','vm-101','cpu',5.0,?, 'raw')`,
		oldTs, oldTs, oldTs, oldTs, newTs,
	)
	if err != nil {
		t.Fatalf("insert metrics returned error: %v", err)
	}

	store.runRetention()

	var rawCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics WHERE tier = 'raw'`).Scan(&rawCount); err != nil {
		t.Fatalf("query raw count: %v", err)
	}
	if rawCount != 1 {
		t.Fatalf("expected 1 raw metric after retention, got %d", rawCount)
	}
	var total int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&total); err != nil {
		t.Fatalf("query total count: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected only newest metric to remain, got %d", total)
	}
}

func TestStoreWriteFlushesBuffer(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics-buffer.db")
	cfg.WriteBufferSize = 1
	cfg.FlushInterval = time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	ts := time.Now().Add(-time.Second)
	store.Write("vm", "vm-101", "cpu", 1.5, ts)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		points, err := store.Query("vm", "vm-101", "cpu", ts.Add(-time.Second), ts.Add(time.Second), 0)
		if err == nil && len(points) == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("expected buffered metric to flush to database")
}

func TestStoreQueryDownsampling(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	start := time.Unix(1000, 0)
	for i := 0; i < 10; i++ {
		store.writeBatch([]bufferedMetric{
			{resourceType: "vm", resourceID: "v1", metricType: "cpu", value: float64(i * 10), timestamp: start.Add(time.Duration(i) * time.Minute)},
		})
	}

	// Query with 5m step
	points, err := store.Query("vm", "v1", "cpu", start.Add(-1*time.Hour), start.Add(1*time.Hour), 300)
	if err != nil {
		t.Fatalf("Query downsampled failed: %v", err)
	}

	// 10 minutes of data at 1m resolution (10 points)
	// Bucketed by 5m (300s):
	// Buckets: [1000-1300), [1300-1600), [1600-1900)
	// Points at: 1000, 1060, 1120, 1180, 1240 (5 points) -> Bucket 1000
	// Points at: 1300, 1360, 1420, 1480, 1540 (5 points) -> Bucket 1300
	if len(points) != 2 {
		t.Fatalf("expected 2 bucketed points, got %d", len(points))
	}
}
