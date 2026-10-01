package metrics

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// The issue-1124 invariant starts from an empty table, where each series'
// few samples share leaf pages with its neighbours whichever column order the
// identity tree uses. A running install retains hours of history per series,
// so a metric-major identity tree gives every series its own insertion page
// and each commit rewrites one page per series. These tests seed that retained
// history before measuring.

const (
	steadyStateResources = 60
	steadyStateHistory   = 360 // one hour of 10-second raw samples per series
	steadyStateTicks     = 30
)

var steadyStateMetricTypes = []string{"cpu", "memory", "disk", "netin", "netout", "diskread", "diskwrite", "temperature"}

var steadyStateBase = time.Unix(1_700_000_000, 0).UTC()

// newSteadyStateStore returns a store holding one hour of raw history for
// every series. layout "metric-major" reinstalls the retired unique
// idx_metrics_lookup beside a non-unique time-major index; any other value
// keeps the store's own schema.
func newSteadyStateStore(tb testing.TB, layout string) (*Store, string) {
	tb.Helper()
	dir := tb.TempDir()
	cfg := DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics.db")
	cfg.FlushInterval = time.Hour
	cfg.RollupInterval = time.Hour
	cfg.RetentionRaw = 10 * 365 * 24 * time.Hour
	store, err := NewStore(cfg)
	if err != nil {
		tb.Fatalf("NewStore: %v", err)
	}
	tb.Cleanup(func() { _ = store.Close() })
	if err := store.WaitForMaintenance(30 * time.Second); err != nil {
		tb.Fatalf("startup maintenance: %v", err)
	}
	if layout == "metric-major" {
		if _, err := store.db.Exec(`
			DROP INDEX idx_metrics_query_all;
			CREATE INDEX idx_metrics_query_all
			ON metrics(resource_type, resource_id, tier, timestamp, metric_type);
			CREATE UNIQUE INDEX idx_metrics_lookup
			ON metrics(resource_type, resource_id, metric_type, tier, timestamp);
		`); err != nil {
			tb.Fatalf("install metric-major identity index: %v", err)
		}
	}

	store.db.SetMaxOpenConns(1)
	store.db.SetMaxIdleConns(1)
	tx, err := store.db.Begin()
	if err != nil {
		tb.Fatalf("begin history seed: %v", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO metrics(resource_type, resource_id, metric_type, value, timestamp, tier)
		VALUES('vm', ?, ?, ?, ?, 'raw')
	`)
	if err != nil {
		_ = tx.Rollback()
		tb.Fatalf("prepare history seed: %v", err)
	}
	for sample := 0; sample < steadyStateHistory; sample++ {
		ts := steadyStateBase.Add(time.Duration(sample) * 10 * time.Second).Unix()
		for resource := 0; resource < steadyStateResources; resource++ {
			for i, metric := range steadyStateMetricTypes {
				if _, err := stmt.Exec(steadyStateResourceID(resource), metric, float64((sample+i)%100), ts); err != nil {
					_ = stmt.Close()
					_ = tx.Rollback()
					tb.Fatalf("seed history: %v", err)
				}
			}
		}
	}
	if err := stmt.Close(); err != nil {
		_ = tx.Rollback()
		tb.Fatalf("close history seed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		tb.Fatalf("commit history seed: %v", err)
	}
	return store, cfg.DBPath
}

func steadyStateResourceID(resource int) string {
	return fmt.Sprintf("cluster-a:pve-%d:%d", resource%4, 100+resource)
}

// steadyStateWALFrames writes steadyStateTicks polls, one commit each, through
// the production ingestion path and returns the WAL frames they produced.
func steadyStateWALFrames(t *testing.T, layout string) int64 {
	t.Helper()
	store, dbPath := newSteadyStateStore(t, layout)
	if _, err := store.db.Exec(`PRAGMA wal_autocheckpoint=0`); err != nil {
		t.Fatalf("disable auto-checkpoint: %v", err)
	}
	if _, err := store.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatalf("reset WAL: %v", err)
	}

	for tick := 0; tick < steadyStateTicks; tick++ {
		ts := steadyStateBase.Add(time.Duration(steadyStateHistory+tick) * 10 * time.Second)
		batch := make([]WriteMetric, 0, steadyStateResources*len(steadyStateMetricTypes))
		for resource := 0; resource < steadyStateResources; resource++ {
			for i, metric := range steadyStateMetricTypes {
				batch = append(batch, WriteMetric{
					ResourceType: "vm",
					ResourceID:   steadyStateResourceID(resource),
					MetricType:   metric,
					Value:        float64((tick + i) % 100),
					Timestamp:    ts,
					Tier:         TierRaw,
				})
			}
		}
		store.WriteBatchSync(batch)
	}

	var persisted int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&persisted); err != nil {
		t.Fatalf("count persisted metrics: %v", err)
	}
	series := steadyStateResources * len(steadyStateMetricTypes)
	if want := series * (steadyStateHistory + steadyStateTicks); persisted != want {
		t.Fatalf("persisted rows=%d, want %d", persisted, want)
	}
	pageSize := issue1124PragmaInt(t, store, "page_size")
	return (issue1124FileSize(t, dbPath+"-wal") - 32) / (pageSize + 24)
}

// TestMetricsIdentityIndexSteadyStateWrites pins the steady-state write cost of
// the time-major identity index against the retired metric-major one. Measured
// on 2026-09-24: 25,310 WAL frames for the metric-major layout and 6,022 for
// the time-major layout over 30 polls of 480 series.
func TestMetricsIdentityIndexSteadyStateWrites(t *testing.T) {
	suppressTestLogs(t)
	metricMajor := steadyStateWALFrames(t, "metric-major")
	timeMajor := steadyStateWALFrames(t, "")
	t.Logf("WAL frames over %d polls: metric-major=%d time-major=%d", steadyStateTicks, metricMajor, timeMajor)
	if timeMajor*100 > metricMajor*35 {
		t.Fatalf(
			"time-major identity index wrote %d WAL frames against %d for the metric-major layout; want at most 35%%",
			timeMajor,
			metricMajor,
		)
	}
}

// BenchmarkQuerySingleMetricOfWideResource reads one metric of a resource that
// carries eight. A time-major identity index reads the resource's other
// metrics in the window as well, so this is the read the index order can slow.
func BenchmarkQuerySingleMetricOfWideResource(b *testing.B) {
	logger := log.Logger
	log.Logger = zerolog.Nop()
	b.Cleanup(func() { log.Logger = logger })
	store, _ := newSteadyStateStore(b, "")
	store.db.SetMaxOpenConns(4)
	store.db.SetMaxIdleConns(4)
	start := steadyStateBase.Add(-time.Second)
	end := steadyStateBase.Add(time.Duration(steadyStateHistory) * 10 * time.Second)
	points, err := store.Query("vm", steadyStateResourceID(7), "memory", start, end, 0)
	if err != nil {
		b.Fatalf("Query: %v", err)
	}
	if len(points) != steadyStateHistory {
		b.Fatalf("points=%d, want %d", len(points), steadyStateHistory)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Query("vm", steadyStateResourceID(7), "memory", start, end, 0); err != nil {
			b.Fatalf("Query: %v", err)
		}
	}
}
