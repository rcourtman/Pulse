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
		{resourceType: "vm", resourceID: "vm-101", metricType: "cpu", value: 1.5, timestamp: ts, tier: TierRaw},
		{resourceType: "vm", resourceID: "vm-101", metricType: "cpu", value: 2.5, timestamp: ts.Add(1 * time.Second), tier: TierRaw},
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

	if store.selectTier(30*time.Minute) != TierRaw {
		t.Fatalf("expected raw tier")
	}
	if store.selectTier(3*time.Hour) != TierMinute {
		t.Fatalf("expected minute tier")
	}
	if store.selectTier(48*time.Hour) != TierHourly {
		t.Fatalf("expected hourly tier")
	}
	if store.selectTier(10*24*time.Hour) != TierDaily {
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

func TestStoreQueryFallbacksToRaw(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics-test.db")
	cfg.FlushInterval = time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	ts := time.Now()
	store.WriteBatchSync([]WriteMetric{{
		ResourceType: "vm",
		ResourceID:   "vm-101",
		MetricType:   "cpu",
		Value:        42.0,
		Timestamp:    ts,
		Tier:         TierRaw,
	}})

	points, err := store.Query("vm", "vm-101", "cpu", ts.Add(-24*time.Hour), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}

	all, err := store.QueryAll("vm", "vm-101", ts.Add(-24*time.Hour), ts.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("QueryAll returned error: %v", err)
	}
	if len(all["cpu"]) != 1 {
		t.Fatalf("expected QueryAll to return 1 cpu point, got %+v", all)
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
	if countRaw != 2 {
		t.Fatalf("expected raw metrics to be retained, got %d", countRaw)
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

// TestStoreRollupTierMultiResource verifies that the batched rollup produces
// correct aggregations when multiple resources and metric types exist in the
// same time window. This exercises the GROUP BY partitioning that replaced
// the previous per-candidate N+1 loop.
func TestStoreRollupTierMultiResource(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics-rollup-multi.db")
	cfg.FlushInterval = time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	base := time.Now().Add(-2 * time.Minute).Truncate(time.Minute)
	ts := base.Unix()

	// Insert data for 3 resources × 2 metric types, all in the same minute bucket.
	_, err = store.db.Exec(`
		INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier) VALUES
		('vm','vm-1','cpu', 10.0, ?, 'raw'),
		('vm','vm-1','cpu', 20.0, ?, 'raw'),
		('vm','vm-1','mem', 50.0, ?, 'raw'),
		('vm','vm-1','mem', 70.0, ?, 'raw'),
		('vm','vm-2','cpu', 30.0, ?, 'raw'),
		('vm','vm-2','cpu', 40.0, ?, 'raw'),
		('node','node-1','cpu', 80.0, ?, 'raw'),
		('node','node-1','cpu', 90.0, ?, 'raw')
	`, ts, ts+10, ts, ts+10, ts, ts+10, ts, ts+10)
	if err != nil {
		t.Fatalf("insert metrics returned error: %v", err)
	}

	store.rollupTier(TierRaw, TierMinute, time.Minute, 0)

	// Verify each resource/metric produced correct aggregations.
	type rollupResult struct {
		resourceType, resourceID, metricType string
		value, minVal, maxVal                float64
	}
	rows, err := store.db.Query(`
		SELECT resource_type, resource_id, metric_type, value, min_value, max_value
		FROM metrics WHERE tier = 'minute'
		ORDER BY resource_type, resource_id, metric_type
	`)
	if err != nil {
		t.Fatalf("query minute tier: %v", err)
	}
	defer rows.Close()

	var results []rollupResult
	for rows.Next() {
		var r rollupResult
		if err := rows.Scan(&r.resourceType, &r.resourceID, &r.metricType, &r.value, &r.minVal, &r.maxVal); err != nil {
			t.Fatalf("scan: %v", err)
		}
		results = append(results, r)
	}

	expected := []rollupResult{
		{"node", "node-1", "cpu", 85.0, 80.0, 90.0},
		{"vm", "vm-1", "cpu", 15.0, 10.0, 20.0},
		{"vm", "vm-1", "mem", 60.0, 50.0, 70.0},
		{"vm", "vm-2", "cpu", 35.0, 30.0, 40.0},
	}

	if len(results) != len(expected) {
		t.Fatalf("expected %d rollup rows, got %d", len(expected), len(results))
	}
	for i, e := range expected {
		r := results[i]
		if r.resourceType != e.resourceType || r.resourceID != e.resourceID || r.metricType != e.metricType {
			t.Fatalf("row %d: expected (%s,%s,%s), got (%s,%s,%s)", i, e.resourceType, e.resourceID, e.metricType, r.resourceType, r.resourceID, r.metricType)
		}
		if r.value != e.value || r.minVal != e.minVal || r.maxVal != e.maxVal {
			t.Fatalf("row %d (%s/%s/%s): expected value=%.1f min=%.1f max=%.1f, got value=%.1f min=%.1f max=%.1f",
				i, e.resourceType, e.resourceID, e.metricType, e.value, e.minVal, e.maxVal, r.value, r.minVal, r.maxVal)
		}
	}
}

// TestStoreRollupTierEmptyWindowPreservesCheckpoint verifies that rollupTier
// does not advance the checkpoint when no source rows exist in the rollup
// window. This ensures late or backfilled samples are still rolled up when
// they arrive after an empty rollup cycle.
func TestStoreRollupTierEmptyWindowPreservesCheckpoint(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics-rollup-empty.db")
	cfg.FlushInterval = time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	// Run rollup on an empty store — checkpoint should NOT be set.
	store.rollupTier(TierRaw, TierMinute, time.Minute, 0)

	metaKey := "rollup:raw:minute"
	if _, ok := store.getMetaInt(metaKey); ok {
		t.Fatal("expected rollup checkpoint to remain unset after empty rollup")
	}

	// Now backfill a sample into the window that would have been skipped.
	base := time.Now().Add(-2 * time.Minute).Truncate(time.Minute)
	ts := base.Unix()
	_, err = store.db.Exec(
		`INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier) VALUES
		('vm','vm-late','cpu', 42.0, ?, 'raw')`, ts,
	)
	if err != nil {
		t.Fatalf("insert backfilled metric: %v", err)
	}

	// Run rollup again — should now process the backfilled sample.
	store.rollupTier(TierRaw, TierMinute, time.Minute, 0)

	var value float64
	err = store.db.QueryRow(
		`SELECT value FROM metrics WHERE tier = 'minute' AND resource_id = 'vm-late'`,
	).Scan(&value)
	if err != nil {
		t.Fatalf("expected minute-tier row for backfilled sample, got error: %v", err)
	}
	if value != 42.0 {
		t.Fatalf("expected rollup value 42.0, got %v", value)
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
			{resourceType: "vm", resourceID: "v1", metricType: "cpu", value: float64(i * 10), timestamp: start.Add(time.Duration(i) * time.Minute), tier: TierRaw},
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
	if len(points) != 3 {
		t.Fatalf("expected 3 bucketed points, got %d", len(points))
	}
}

func TestQueryAllBatch(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics-batch.db")
	cfg.FlushInterval = time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ts := time.Unix(1000, 0)
	store.writeBatch([]bufferedMetric{
		// disk-1: smart_temp with 2 points
		{resourceType: "disk", resourceID: "disk-1", metricType: "smart_temp", value: 35, timestamp: ts, tier: TierRaw},
		{resourceType: "disk", resourceID: "disk-1", metricType: "smart_temp", value: 36, timestamp: ts.Add(time.Second), tier: TierRaw},
		// disk-2: smart_temp with 1 point, power_on_hours with 1 point
		{resourceType: "disk", resourceID: "disk-2", metricType: "smart_temp", value: 40, timestamp: ts, tier: TierRaw},
		{resourceType: "disk", resourceID: "disk-2", metricType: "smart_power_on_hours", value: 1000, timestamp: ts, tier: TierRaw},
		// disk-3: no data (will not appear in results)
	})

	start := ts.Add(-time.Second)
	end := ts.Add(2 * time.Second)

	t.Run("returns data grouped by resource and metric", func(t *testing.T) {
		result, err := store.QueryAllBatch("disk", []string{"disk-1", "disk-2", "disk-3"}, start, end, 0)
		if err != nil {
			t.Fatalf("QueryAllBatch: %v", err)
		}

		// disk-1 should have smart_temp with 2 points
		if got := len(result["disk-1"]["smart_temp"]); got != 2 {
			t.Fatalf("disk-1 smart_temp: expected 2 points, got %d", got)
		}
		if result["disk-1"]["smart_temp"][0].Value != 35 || result["disk-1"]["smart_temp"][1].Value != 36 {
			t.Fatalf("disk-1 smart_temp unexpected values: %+v", result["disk-1"]["smart_temp"])
		}

		// disk-2 should have smart_temp with 1 point and power_on_hours with 1 point
		if got := len(result["disk-2"]["smart_temp"]); got != 1 {
			t.Fatalf("disk-2 smart_temp: expected 1 point, got %d", got)
		}
		if got := len(result["disk-2"]["smart_power_on_hours"]); got != 1 {
			t.Fatalf("disk-2 power_on_hours: expected 1 point, got %d", got)
		}

		// disk-3 should have no entry
		if _, ok := result["disk-3"]; ok {
			t.Fatalf("disk-3 should not appear in results: %+v", result["disk-3"])
		}
	})

	t.Run("empty resource IDs returns empty map", func(t *testing.T) {
		result, err := store.QueryAllBatch("disk", nil, start, end, 0)
		if err != nil {
			t.Fatalf("QueryAllBatch: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("single resource ID matches QueryAll", func(t *testing.T) {
		batch, err := store.QueryAllBatch("disk", []string{"disk-1"}, start, end, 0)
		if err != nil {
			t.Fatalf("QueryAllBatch: %v", err)
		}
		single, err := store.QueryAll("disk", "disk-1", start, end, 0)
		if err != nil {
			t.Fatalf("QueryAll: %v", err)
		}

		if len(batch["disk-1"]["smart_temp"]) != len(single["smart_temp"]) {
			t.Fatalf("batch (%d points) != single (%d points)",
				len(batch["disk-1"]["smart_temp"]), len(single["smart_temp"]))
		}
	})

	t.Run("duplicate resource IDs are deduplicated", func(t *testing.T) {
		result, err := store.QueryAllBatch("disk", []string{"disk-1", "disk-1", "disk-2", "disk-1"}, start, end, 0)
		if err != nil {
			t.Fatalf("QueryAllBatch: %v", err)
		}
		// Should still return correct results despite dupes
		if got := len(result["disk-1"]["smart_temp"]); got != 2 {
			t.Fatalf("disk-1 smart_temp: expected 2 points after dedup, got %d", got)
		}
		if got := len(result["disk-2"]["smart_temp"]); got != 1 {
			t.Fatalf("disk-2 smart_temp: expected 1 point after dedup, got %d", got)
		}
	})

	t.Run("per-resource fallback stops at first non-empty tier like QueryAll", func(t *testing.T) {
		tsMinute := ts.Add(-24 * time.Hour)
		store.writeBatch([]bufferedMetric{
			{resourceType: "disk", resourceID: "disk-raw", metricType: "smart_temp", value: 41, timestamp: ts, tier: TierRaw},
			{resourceType: "disk", resourceID: "disk-raw", metricType: "smart_temp", value: 39, timestamp: tsMinute, tier: TierMinute},
			{resourceType: "disk", resourceID: "disk-raw", metricType: "smart_power_on_hours", value: 1234, timestamp: tsMinute, tier: TierMinute},
			{resourceType: "disk", resourceID: "disk-minute", metricType: "smart_temp", value: 37, timestamp: tsMinute, tier: TierMinute},
		})

		rangeStart := ts.Add(-90 * time.Minute)
		rangeEnd := ts.Add(time.Minute)

		batch, err := store.QueryAllBatch("disk", []string{"disk-raw", "disk-minute"}, rangeStart, rangeEnd, 0)
		if err != nil {
			t.Fatalf("QueryAllBatch: %v", err)
		}

		singleRaw, err := store.QueryAll("disk", "disk-raw", rangeStart, rangeEnd, 0)
		if err != nil {
			t.Fatalf("QueryAll(disk-raw): %v", err)
		}
		singleMinute, err := store.QueryAll("disk", "disk-minute", rangeStart, rangeEnd, 0)
		if err != nil {
			t.Fatalf("QueryAll(disk-minute): %v", err)
		}

		if got := len(batch["disk-raw"]["smart_temp"]); got != len(singleRaw["smart_temp"]) {
			t.Fatalf("disk-raw smart_temp points = %d, want %d", got, len(singleRaw["smart_temp"]))
		}
		if _, exists := batch["disk-raw"]["smart_power_on_hours"]; exists {
			t.Fatalf("disk-raw unexpectedly merged minute-tier metrics: %+v", batch["disk-raw"])
		}
		if got := len(batch["disk-minute"]["smart_temp"]); got != len(singleMinute["smart_temp"]) {
			t.Fatalf("disk-minute smart_temp points = %d, want %d", got, len(singleMinute["smart_temp"]))
		}
	})
}
