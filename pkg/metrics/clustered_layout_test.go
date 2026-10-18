package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	pdb "github.com/rcourtman/pulse-go-rewrite/pkg/db"
)

// Pause deferred maintenance, not the constructor, so tests can exercise the
// installed legacy schema and the cutover without racing a background upgrade.
func clusteredLegacyStore(t *testing.T) (*Store, func()) {
	t.Helper()
	suppressTestLogs(t)
	dbPath := filepath.Join(t.TempDir(), "metrics.db")
	issue1124CreateLegacyStore(t, dbPath, 100, false)
	entered, release := make(chan struct{}), make(chan struct{})
	previous := startupMaintenanceHook
	startupMaintenanceHook = func() { close(entered); <-release }
	cfg := DefaultConfig(filepath.Dir(dbPath))
	cfg.DBPath = dbPath
	cfg.FlushInterval, cfg.RollupInterval = time.Hour, time.Hour
	cfg.RetentionRaw = 10 * 365 * 24 * time.Hour
	store, err := NewStore(cfg)
	startupMaintenanceHook = previous
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	if err != nil {
		unblock()
		t.Fatal(err)
	}
	t.Cleanup(func() { unblock(); _ = store.Close() })
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("deferred maintenance did not start")
	}
	return store, unblock
}

func assertClusteredStore(t *testing.T, store *Store) {
	t.Helper()
	clustered, err := metricsTableIsClustered(context.Background(), store.db.DB)
	if err != nil || !clustered {
		t.Fatalf("clustered schema=%t, error=%v", clustered, err)
	}
	issue1124AssertConsolidatedIndexes(t, store)
	var integrity string
	if err := store.db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity=%q, error=%v", integrity, err)
	}
}

func TestClusteredMetricsFreshAndRestart(t *testing.T) {
	cfg := DefaultConfig(t.TempDir())
	for i := 0; i < 2; i++ {
		store, err := NewStore(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.WaitForMaintenance(30 * time.Second); err != nil {
			t.Fatal(err)
		}
		assertClusteredStore(t, store)
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestClusteredMetricsPreservesObservationsAndMetadata(t *testing.T) {
	store, _ := clusteredLegacyStore(t)
	base := time.Unix(1_700_000_000, 0)
	for i, tier := range []Tier{TierRaw, TierMinute, TierHourly, TierDaily} {
		for j, resource := range []string{"vm", "dockercontainer"} {
			for k, bounds := range [][2]any{{nil, nil}, {0.0, nil}, {nil, 0.0}, {-4.0, 12.0}} {
				_, err := store.db.Exec(`INSERT INTO metrics
					(resource_type, resource_id, metric_type, value, min_value, max_value, timestamp, tier)
					VALUES (?, ?, 'cpu', ?, ?, ?, ?, ?)`, resource, fmt.Sprintf("case-%d", j),
					float64(k+1), bounds[0], bounds[1], base.Add(time.Duration(i*86400+k*60)*time.Second).Unix(), tier)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if _, err := store.db.Exec(`INSERT INTO metrics_meta VALUES ('rollup_sentinel', '1700000000')`); err != nil {
		t.Fatal(err)
	}
	// The exact stored nullable bounds, observation names, tiers and values
	// must survive, independently of any reader's normalization or fallback.
	read := func() []string {
		rows, err := store.db.Query(`SELECT resource_type, resource_id, metric_type,
			quote(value), quote(min_value), quote(max_value), timestamp, tier
			FROM metrics ORDER BY resource_type, resource_id, tier, timestamp, metric_type`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var result []string
		for rows.Next() {
			var resource, id, metric, value, low, high, tier string
			var ts int64
			if err := rows.Scan(&resource, &id, &metric, &value, &low, &high, &ts, &tier); err != nil {
				t.Fatal(err)
			}
			result = append(result, fmt.Sprint(resource, id, metric, value, low, high, ts, tier))
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := read()
	if err := store.migrateClusteredMetrics(context.Background()); err != nil {
		t.Fatal(err)
	}
	if after := read(); !reflect.DeepEqual(before, after) {
		t.Fatalf("stored observations changed across clustered migration")
	}
	var sentinel string
	if err := store.db.QueryRow(`SELECT value FROM metrics_meta WHERE key='rollup_sentinel'`).Scan(&sentinel); err != nil || sentinel != "1700000000" {
		t.Fatalf("rollup metadata=%q, error=%v", sentinel, err)
	}
	assertClusteredStore(t, store)
}

func TestClusteredMetricsReadersAndQueuedWritesSurviveCutover(t *testing.T) {
	store, _ := clusteredLegacyStore(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	store.clusteredMigrationHook = func(stage string) {
		if stage == "after-copy" {
			close(entered)
			<-release
		}
	}
	// An already open SQLite snapshot must retain its original schema while
	// the other connection swaps and drops the physical table.
	tx, err := store.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM metrics INDEXED BY idx_metrics_query_all`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	migrated := make(chan error, 1)
	go func() { migrated <- store.migrateClusteredMetrics(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("migration did not copy")
	}
	points, err := store.Query("vm", "legacy-0001", "cpu", time.Unix(1_699_999_999, 0), time.Unix(1_700_001_000, 0), 0)
	if err != nil || len(points) != 1 {
		t.Fatalf("reader during copy points=%d, error=%v", len(points), err)
	}
	written := make(chan struct{})
	go func() {
		store.WriteBatchSync([]WriteMetric{{ResourceType: "vm", ResourceID: "queued", MetricType: "cpu", Value: 42, Timestamp: time.Unix(1_700_020_000, 0), Tier: TierRaw}})
		close(written)
	}()
	unblock()
	if err := <-migrated; err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM metrics INDEXED BY idx_metrics_query_all`).Scan(&count); err != nil || count != 100 {
		t.Fatalf("old read snapshot count=%d, error=%v", count, err)
	}
	_ = tx.Rollback()
	select {
	case <-written:
	case <-time.After(35 * time.Second):
		t.Fatal("queued write did not resume")
	}
	var value float64
	if err := store.db.QueryRow(`SELECT value FROM metrics WHERE resource_id='queued'`).Scan(&value); err != nil || value != 42 {
		t.Fatalf("queued write=%v, error=%v", value, err)
	}
	assertClusteredStore(t, store)
}

func TestClusteredMetricsCancellationRollsBackSwap(t *testing.T) {
	store, _ := clusteredLegacyStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.clusteredMigrationHook = func(stage string) {
		if stage == "before-commit" {
			cancel()
		}
	}
	if err := store.migrateClusteredMetrics(ctx); err == nil {
		t.Fatal("canceled migration succeeded")
	}
	clustered, err := metricsTableIsClustered(context.Background(), store.db.DB)
	if err != nil || clustered {
		t.Fatalf("canceled swap changed layout=%t, error=%v", clustered, err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&count); err != nil || count != 100 {
		t.Fatalf("rollback rows=%d, error=%v", count, err)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM sqlite_schema WHERE name='metrics_clustered_upgrade'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback replacement tables=%d, error=%v", count, err)
	}
	store.clusteredMigrationHook = nil
	if err := store.migrateClusteredMetrics(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertClusteredStore(t, store)
}

func TestClusteredMetricsPreservesConflictingReplacementTable(t *testing.T) {
	store, _ := clusteredLegacyStore(t)
	if _, err := store.db.Exec(`CREATE TABLE metrics_clustered_upgrade(value TEXT);
		INSERT INTO metrics_clustered_upgrade VALUES ('preserve')`); err != nil {
		t.Fatal(err)
	}
	if err := store.migrateClusteredMetrics(context.Background()); err == nil {
		t.Fatal("conflicting table was overwritten")
	}
	var value string
	if err := store.db.QueryRow(`SELECT value FROM metrics_clustered_upgrade`).Scan(&value); err != nil || value != "preserve" {
		t.Fatalf("conflicting table value=%q, error=%v", value, err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&count); err != nil || count != 100 {
		t.Fatalf("original rows=%d, error=%v", count, err)
	}
}

func TestClusteredMetricsCrashRollsBackSwap(t *testing.T) {
	if dbPath := os.Getenv("PULSE_CLUSTERED_CRASH_DB"); dbPath != "" {
		metricsClusteredMigrationHook = func(stage string) {
			if stage == "before-commit" {
				os.Exit(88)
			}
		}
		cfg := DefaultConfig(filepath.Dir(dbPath))
		cfg.DBPath = dbPath
		cfg.RetentionRaw = 10 * 365 * 24 * time.Hour
		store, err := NewStore(cfg)
		if err != nil {
			os.Exit(90)
		}
		_ = store.WaitForMaintenance(30 * time.Second)
		os.Exit(89)
	}
	suppressTestLogs(t)
	dbPath := filepath.Join(t.TempDir(), "metrics.db")
	issue1124CreateLegacyStore(t, dbPath, 10_000, false)
	cmd := exec.Command(os.Args[0], "-test.run=^TestClusteredMetricsCrashRollsBackSwap$")
	cmd.Env = append(os.Environ(), "PULSE_CLUSTERED_CRASH_DB="+dbPath)
	err := cmd.Run()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 88 {
		t.Fatalf("crash child=%v, want exit 88", err)
	}
	cfg := DefaultConfig(filepath.Dir(dbPath))
	cfg.DBPath = dbPath
	cfg.RetentionRaw = 10 * 365 * 24 * time.Hour
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.WaitForMaintenance(30 * time.Second); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&count); err != nil || count != 10_000 {
		t.Fatalf("recovered rows=%d, error=%v", count, err)
	}
	assertClusteredStore(t, store)
}

func TestClusteredMetricsRefreshesAnotherStoreAndCachedShapes(t *testing.T) {
	store, _ := clusteredLegacyStore(t)
	// A second handle has its own physical-schema bit and bounded SQL caches.
	release := make(chan struct{})
	previous := startupMaintenanceHook
	startupMaintenanceHook = func() { <-release }
	other, err := NewStore(store.config)
	startupMaintenanceHook = previous
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	t.Cleanup(func() { close(release); _ = other.Close() })
	start, end := time.Unix(1_699_999_999, 0), time.Unix(1_700_001_000, 0)
	for _, step := range []int64{0, 60} {
		if _, err := other.QueryAllBatch("vm", []string{"legacy-0001", "legacy-0002"}, start, end, step); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.migrateClusteredMetrics(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, step := range []int64{0, 60} {
		// Force the other handle's stale layout, including a cached presence
		// statement, so both snapshot paths exercise the real missing-index error.
		other.clusteredMetrics.Store(false)
		points, err := other.QueryAllBatch("vm", []string{"legacy-0001", "legacy-0002"}, start, end, step)
		if err != nil || len(points) != 2 || !other.clusteredMetrics.Load() {
			t.Fatalf("stale full reader step=%d: resources=%d, refreshed=%t, error=%v", step, len(points), other.clusteredMetrics.Load(), err)
		}
		other.clusteredMetrics.Store(false)
		values, err := other.QueryMetricValuesBatch("vm", []string{"legacy-0001", "legacy-0002"}, nil, start, end, step)
		if err != nil || len(values) != 2 || !other.clusteredMetrics.Load() {
			t.Fatalf("stale value reader step=%d: resources=%d, refreshed=%t, error=%v", step, len(values), other.clusteredMetrics.Load(), err)
		}
	}
}

func TestClusteredMetricsReopeningLegacyExecutableIndexes(t *testing.T) {
	cfg := DefaultConfig(t.TempDir())
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WaitForMaintenance(30 * time.Second); err != nil {
		t.Fatal(err)
	}
	// Reproduce the previous runtime's initSchema/unique-index behavior on
	// the new table. Its unchanged upsert conflict target remains functional.
	if _, err := store.db.Exec(`CREATE UNIQUE INDEX idx_metrics_query_all
		ON metrics(resource_type, resource_id, tier, timestamp, metric_type);
		INSERT INTO metrics(resource_type, resource_id, metric_type, value, timestamp, tier)
		VALUES ('vm', 'downgrade', 'cpu', 1, 2000000000, 'raw')
		ON CONFLICT(resource_type, resource_id, metric_type, tier, timestamp) DO UPDATE SET value=excluded.value;
		INSERT INTO metrics(resource_type, resource_id, metric_type, value, timestamp, tier)
		VALUES ('vm', 'downgrade', 'cpu', 2, 2000000000, 'raw')
		ON CONFLICT(resource_type, resource_id, metric_type, tier, timestamp) DO UPDATE SET value=excluded.value;`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.WaitForMaintenance(30 * time.Second); err != nil {
		t.Fatal(err)
	}
	assertClusteredStore(t, reopened)
	var value float64
	if err := reopened.db.QueryRow(`SELECT value FROM metrics WHERE resource_id='downgrade'`).Scan(&value); err != nil || value != 2 {
		t.Fatalf("legacy-compatible upsert value=%v, error=%v", value, err)
	}
}

func TestClusteredRetainedPlansUsePrimaryTable(t *testing.T) {
	db := newPlanTestDB(t)
	store := &Store{db: pdb.Wrap(db, "metrics"), config: DefaultConfig(t.TempDir())}
	if err := store.migrateClusteredMetrics(context.Background()); err != nil {
		t.Fatal(err)
	}
	end := time.Unix(2_000_000_040, 0)
	for _, valuesOnly := range []bool{false, true} {
		for _, window := range []time.Duration{time.Hour, 24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour} {
			for _, ids := range [][]string{{"vm-1"}, {"vm-1", "vm-2"}} {
				for _, metrics := range [][]string{nil, {"cpu"}, {"cpu", "memory"}} {
					for _, step := range []int64{0, 60} {
						t.Run(fmt.Sprintf("values_%t/%s/resources_%d/metrics_%d/step_%d", valuesOnly, window, len(ids), len(metrics), step), func(t *testing.T) {
							query, args := store.retainedQuerySQL("vm", ids, metrics, end.Add(-window), end, step, store.tierFallbacks(window), false, valuesOnly)
							plan := explainQueryPlan(t, db, query, args)
							for _, line := range strings.Split(plan, "\n") {
								if strings.Contains(line, "SCAN m ") || strings.Contains(line, "SCAN h ") {
									t.Fatalf("unbounded clustered read: %s", plan)
								}
								if strings.Contains(line, "SEARCH m ") || strings.Contains(line, "SEARCH h ") {
									for _, constraint := range []string{"USING PRIMARY KEY", "resource_type=?", "resource_id=?", "tier=?", "timestamp>?", "timestamp<?"} {
										if !strings.Contains(line, constraint) {
											t.Fatalf("clustered lookup missing %s: %s", constraint, plan)
										}
									}
								}
								if step == 0 && strings.Contains(line, "TEMP B-TREE") {
									t.Fatalf("clustered unaggregated read sorts: %s", plan)
								}
							}
							if !strings.Contains(plan, "SEARCH m USING PRIMARY KEY") {
								t.Fatalf("missing primary table lookup: %s", plan)
							}
						})
					}
				}
			}
		}
	}
}
