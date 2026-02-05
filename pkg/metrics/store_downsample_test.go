package metrics

import (
	"math"
	"testing"
	"time"
)

func TestStoreQueryAllDownsampling(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	start := time.Unix(1000, 0)
	batch := make([]bufferedMetric, 0, 20)
	for i := 0; i < 10; i++ {
		ts := start.Add(time.Duration(i) * time.Minute)
		batch = append(batch,
			bufferedMetric{resourceType: "vm", resourceID: "v1", metricType: "cpu", value: float64(i), timestamp: ts, tier: TierRaw},
			bufferedMetric{resourceType: "vm", resourceID: "v1", metricType: "mem", value: float64(100 + i), timestamp: ts, tier: TierRaw},
		)
	}
	store.writeBatch(batch)

	result, err := store.QueryAll("vm", "v1", start.Add(-time.Hour), start.Add(time.Hour), 300)
	if err != nil {
		t.Fatalf("QueryAll downsampled failed: %v", err)
	}

	cpu := result["cpu"]
	mem := result["mem"]
	if len(cpu) != 3 || len(mem) != 3 {
		t.Fatalf("expected 3 bucketed points per metric, got cpu=%d mem=%d", len(cpu), len(mem))
	}

	assertPoint := func(point MetricPoint, ts int64, value, min, max float64) {
		t.Helper()
		if point.Timestamp.Unix() != ts {
			t.Fatalf("expected bucket timestamp %d, got %d", ts, point.Timestamp.Unix())
		}
		if math.Abs(point.Value-value) > 0.0001 {
			t.Fatalf("expected value %v, got %v", value, point.Value)
		}
		if math.Abs(point.Min-min) > 0.0001 {
			t.Fatalf("expected min %v, got %v", min, point.Min)
		}
		if math.Abs(point.Max-max) > 0.0001 {
			t.Fatalf("expected max %v, got %v", max, point.Max)
		}
	}

	assertPoint(cpu[0], 1050, 1.5, 0, 3)
	assertPoint(cpu[1], 1350, 6, 4, 8)
	assertPoint(cpu[2], 1650, 9, 9, 9)

	assertPoint(mem[0], 1050, 101.5, 100, 103)
	assertPoint(mem[1], 1350, 106, 104, 108)
	assertPoint(mem[2], 1650, 109, 109, 109)
}

func TestStoreTierFallbacks(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	tests := []struct {
		name     string
		duration time.Duration
		expected []Tier
	}{
		{"raw", 30 * time.Minute, []Tier{TierRaw, TierMinute, TierHourly}},
		{"minute", 3 * time.Hour, []Tier{TierMinute, TierRaw, TierHourly}},
		{"hourly", 2 * 24 * time.Hour, []Tier{TierHourly, TierMinute, TierRaw}},
		{"daily", 30 * 24 * time.Hour, []Tier{TierDaily, TierHourly, TierMinute, TierRaw}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := store.tierFallbacks(tc.duration)
			if len(got) != len(tc.expected) {
				t.Fatalf("expected %d tiers, got %d (%v)", len(tc.expected), len(got), got)
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Fatalf("expected %v, got %v", tc.expected, got)
				}
			}
		})
	}
}

func TestStoreMetadataHelpers(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	if value, ok := store.getMetaInt("missing"); ok {
		t.Fatalf("expected missing meta to return ok=false, got %d", value)
	}

	if ts, ok := store.getMaxTimestampForTier(TierRaw); ok || ts != 0 {
		t.Fatalf("expected no max timestamp, got %d (ok=%t)", ts, ok)
	}

	_, err = store.db.Exec(
		`INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier) VALUES
		('vm','vm-1','cpu',1.0,?, 'raw'),
		('vm','vm-1','cpu',2.0,?, 'raw')`,
		100, 200,
	)
	if err != nil {
		t.Fatalf("insert metrics returned error: %v", err)
	}

	if ts, ok := store.getMaxTimestampForTier(TierRaw); !ok || ts != 200 {
		t.Fatalf("expected max timestamp 200, got %d (ok=%t)", ts, ok)
	}
}

func TestStoreQueryDownsamplingStats(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(DefaultConfig(dir))
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	defer store.Close()

	start := time.Unix(1000, 0)
	store.writeBatch([]bufferedMetric{
		{resourceType: "vm", resourceID: "v2", metricType: "cpu", value: 10, timestamp: start, tier: TierRaw},
		{resourceType: "vm", resourceID: "v2", metricType: "cpu", value: 30, timestamp: start.Add(20 * time.Second), tier: TierRaw},
		{resourceType: "vm", resourceID: "v2", metricType: "cpu", value: 20, timestamp: start.Add(50 * time.Second), tier: TierRaw},
	})

	points, err := store.Query("vm", "v2", "cpu", start.Add(-time.Minute), start.Add(time.Minute), 120)
	if err != nil {
		t.Fatalf("Query downsampled failed: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 bucketed point, got %d", len(points))
	}

	point := points[0]
	if point.Timestamp.Unix() != 1020 {
		t.Fatalf("expected bucket timestamp 1020, got %d", point.Timestamp.Unix())
	}
	if point.Value != 20 || point.Min != 10 || point.Max != 30 {
		t.Fatalf("unexpected stats: value=%v min=%v max=%v", point.Value, point.Min, point.Max)
	}
}
