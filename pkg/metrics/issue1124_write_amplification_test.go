package metrics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sqlite3 "modernc.org/sqlite"
)

const issue1124ProfileEnv = "PULSE_METRICS_WRITE_PROFILE"

type issue1124DBStat struct {
	Name         string `json:"name"`
	Pages        int64  `json:"pages"`
	PageBytes    int64  `json:"page_bytes"`
	PayloadBytes int64  `json:"payload_bytes"`
}

type issue1124ProfileReport struct {
	SchemaMode           string            `json:"schema_mode"`
	AutoCheckpoint       bool              `json:"auto_checkpoint"`
	Ticks                int               `json:"ticks"`
	Resources            int               `json:"resources"`
	Samples              int               `json:"samples"`
	WriteCalls           int               `json:"write_calls"`
	LogicalPayloadBytes  int64             `json:"logical_payload_bytes"`
	Elapsed              time.Duration     `json:"elapsed"`
	WALBytes             int64             `json:"wal_bytes"`
	WALFrames            int64             `json:"wal_frames"`
	CacheWritesBefore    int64             `json:"cache_writes_before_checkpoint"`
	CacheWritesAfter     int64             `json:"cache_writes_after_checkpoint"`
	CacheSpills          int64             `json:"cache_spills"`
	PhysicalWriteBytes   int64             `json:"physical_write_bytes"`
	CheckpointWriteBytes int64             `json:"checkpoint_write_bytes"`
	CheckpointedFrames   int64             `json:"checkpointed_frames"`
	MainBytesBefore      int64             `json:"main_bytes_before_checkpoint"`
	MainBytesAfter       int64             `json:"main_bytes_after_checkpoint"`
	PageSize             int64             `json:"page_size"`
	PageCount            int64             `json:"page_count"`
	FreelistCount        int64             `json:"freelist_count"`
	ReadCount            int64             `json:"concurrent_read_count"`
	MaxReadLatency       time.Duration     `json:"max_read_latency"`
	RestartElapsed       time.Duration     `json:"restart_elapsed"`
	RestartMainBytes     int64             `json:"restart_main_bytes"`
	RestartPageCount     int64             `json:"restart_page_count"`
	RestartFreelist      int64             `json:"restart_freelist_count"`
	RetentionElapsed     time.Duration     `json:"retention_elapsed"`
	RetentionRows        int               `json:"retention_rows"`
	RetentionMainBytes   int64             `json:"retention_main_bytes"`
	RetentionPageCount   int64             `json:"retention_page_count"`
	RetentionFreelist    int64             `json:"retention_freelist_count"`
	DBStats              []issue1124DBStat `json:"db_stats"`
}

type issue1124Series struct {
	resourceType string
	resourceID   string
	metricTypes  []string
}

func TestStoreConsolidatesLegacyMetricsIndexes(t *testing.T) {
	suppressTestLogs(t)
	dbPath := filepath.Join(t.TempDir(), "metrics.db")
	issue1124CreateLegacyStore(t, dbPath, 2_000, false)

	cfg := DefaultConfig(filepath.Dir(dbPath))
	cfg.DBPath = dbPath
	cfg.RetentionRaw = 10 * 365 * 24 * time.Hour
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("migrate legacy store: %v", err)
	}
	if err := store.WaitForMaintenance(30 * time.Second); err != nil {
		_ = store.Close()
		t.Fatalf("wait for post-migration maintenance: %v", err)
	}
	issue1124AssertConsolidatedIndexes(t, store)

	var rows int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&rows); err != nil {
		_ = store.Close()
		t.Fatalf("count migrated rows: %v", err)
	}
	if rows != 2_000 {
		_ = store.Close()
		t.Fatalf("migrated rows=%d, want 2000", rows)
	}

	ts := time.Unix(1_700_001_999, 0)
	store.WriteBatchSync([]WriteMetric{{
		ResourceType: "vm",
		ResourceID:   "legacy-1999",
		MetricType:   "cpu",
		Value:        99,
		Timestamp:    ts,
		Tier:         TierRaw,
	}})
	var value float64
	if err := store.db.QueryRow(`
		SELECT value FROM metrics
		WHERE resource_type = 'vm' AND resource_id = 'legacy-1999'
			AND metric_type = 'cpu' AND tier = 'raw' AND timestamp = ?
	`, ts.Unix()).Scan(&value); err != nil {
		_ = store.Close()
		t.Fatalf("query migrated upsert: %v", err)
	}
	if value != 99 {
		_ = store.Close()
		t.Fatalf("migrated upsert value=%v, want 99", value)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close migrated store: %v", err)
	}

	restarted, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("restart migrated store: %v", err)
	}
	issue1124AssertConsolidatedIndexes(t, restarted)
	if err := restarted.Close(); err != nil {
		t.Fatalf("close restarted store: %v", err)
	}
}

func TestStoreIdentityMigrationDeduplicatesLegacyRows(t *testing.T) {
	suppressTestLogs(t)
	dbPath := filepath.Join(t.TempDir(), "metrics.db")
	issue1124CreateLegacyStore(t, dbPath, 2, true)

	cfg := DefaultConfig(filepath.Dir(dbPath))
	cfg.DBPath = dbPath
	cfg.RetentionRaw = 10 * 365 * 24 * time.Hour
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("migrate duplicate legacy store: %v", err)
	}
	defer store.Close()
	issue1124AssertConsolidatedIndexes(t, store)

	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&count); err != nil {
		t.Fatalf("count deduplicated rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("deduplicated row count=%d, want 1", count)
	}

	ts := time.Unix(1_700_000_000, 0)
	store.WriteBatchSync([]WriteMetric{{
		ResourceType: "vm",
		ResourceID:   "legacy-0000",
		MetricType:   "cpu",
		Value:        42,
		Timestamp:    ts,
		Tier:         TierRaw,
	}})
	var value float64
	if err := store.db.QueryRow(`SELECT value FROM metrics`).Scan(&value); err != nil {
		t.Fatalf("query deduplicated value: %v", err)
	}
	if value != 42 {
		t.Fatalf("upsert after dedupe value=%v, want 42", value)
	}
}

func TestStoreIdentityMigrationRecoversAfterCrash(t *testing.T) {
	if os.Getenv("PULSE_METRICS_IDENTITY_CRASH_CHILD") == "1" {
		metricsIdentityMigrationHook = func() {
			os.Exit(88)
		}
		dbPath := os.Getenv("PULSE_METRICS_IDENTITY_CRASH_DB")
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

	cmd := exec.Command(os.Args[0], "-test.run=^TestStoreIdentityMigrationRecoversAfterCrash$")
	cmd.Env = append(
		os.Environ(),
		"PULSE_METRICS_IDENTITY_CRASH_CHILD=1",
		"PULSE_METRICS_IDENTITY_CRASH_DB="+dbPath,
	)
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 88 {
		t.Fatalf("migration crash child error=%v, want exit status 88", err)
	}

	cfg := DefaultConfig(filepath.Dir(dbPath))
	cfg.DBPath = dbPath
	cfg.RetentionRaw = 10 * 365 * 24 * time.Hour
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("recover store after interrupted migration: %v", err)
	}
	defer store.Close()
	if err := store.WaitForMaintenance(30 * time.Second); err != nil {
		t.Fatalf("wait for crash-recovery migration: %v", err)
	}
	issue1124AssertConsolidatedIndexes(t, store)

	var rows int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&rows); err != nil {
		t.Fatalf("count rows after crash recovery: %v", err)
	}
	if rows != 10_000 {
		t.Fatalf("rows after crash recovery=%d, want 10000", rows)
	}
	var integrity string
	if err := store.db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatalf("integrity check after crash recovery: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("integrity check after crash recovery=%q, want ok", integrity)
	}
}

func TestStoreIdentityMigrationBackupRestore(t *testing.T) {
	suppressTestLogs(t)
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "legacy.db")
	issue1124CreateLegacyStore(t, legacyPath, 5_000, false)

	legacyBackup := filepath.Join(dir, "legacy-backup.db")
	issue1124CopyFile(t, legacyPath, legacyBackup)
	cfg := DefaultConfig(dir)
	cfg.DBPath = legacyBackup
	cfg.RetentionRaw = 10 * 365 * 24 * time.Hour
	restored, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("restore legacy backup: %v", err)
	}
	if err := restored.WaitForMaintenance(30 * time.Second); err != nil {
		_ = restored.Close()
		t.Fatalf("legacy backup maintenance: %v", err)
	}
	issue1124AssertConsolidatedIndexes(t, restored)
	var rows int
	if err := restored.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&rows); err != nil {
		_ = restored.Close()
		t.Fatalf("count restored legacy backup: %v", err)
	}
	if rows != 5_000 {
		_ = restored.Close()
		t.Fatalf("restored legacy rows=%d, want 5000", rows)
	}
	var integrity string
	if err := restored.db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		_ = restored.Close()
		t.Fatalf("legacy backup integrity check: %v", err)
	}
	if integrity != "ok" {
		_ = restored.Close()
		t.Fatalf("legacy backup integrity=%q, want ok", integrity)
	}
	if err := restored.Close(); err != nil {
		t.Fatalf("close restored legacy backup: %v", err)
	}

	consolidatedBackup := filepath.Join(dir, "consolidated-backup.db")
	issue1124CopyFile(t, legacyBackup, consolidatedBackup)
	cfg.DBPath = consolidatedBackup
	reopened, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("restore consolidated backup: %v", err)
	}
	defer reopened.Close()
	issue1124AssertConsolidatedIndexes(t, reopened)
	if err := reopened.db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatalf("consolidated backup integrity check: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("consolidated backup integrity=%q, want ok", integrity)
	}
}

func TestStoreIdentityMigrationKeepsReadersAvailable(t *testing.T) {
	suppressTestLogs(t)
	dbPath := filepath.Join(t.TempDir(), "metrics.db")
	issue1124CreateLegacyStore(t, dbPath, 10_000, false)

	migrationEntered := make(chan struct{})
	releaseMigration := make(chan struct{})
	var hookOnce sync.Once
	metricsIdentityMigrationHook = func() {
		hookOnce.Do(func() { close(migrationEntered) })
		<-releaseMigration
	}
	t.Cleanup(func() {
		metricsIdentityMigrationHook = nil
	})

	cfg := DefaultConfig(filepath.Dir(dbPath))
	cfg.DBPath = dbPath
	cfg.RetentionRaw = 10 * 365 * 24 * time.Hour
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("open legacy store: %v", err)
	}
	defer store.Close()

	select {
	case <-migrationEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("deferred identity migration did not start")
	}

	queryDone := make(chan error, 1)
	go func() {
		_, err := store.Query(
			"vm",
			"legacy-0001",
			"cpu",
			time.Unix(1_699_999_999, 0),
			time.Unix(1_700_000_002, 0),
			0,
		)
		queryDone <- err
	}()
	select {
	case err := <-queryDone:
		if err != nil {
			t.Fatalf("query during identity migration: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("query blocked behind deferred identity migration")
	}

	writeDone := make(chan struct{})
	go func() {
		store.WriteBatchSync([]WriteMetric{{
			ResourceType: "vm",
			ResourceID:   "during-migration",
			MetricType:   "cpu",
			Value:        42,
			Timestamp:    time.Unix(1_700_020_000, 0),
			Tier:         TierRaw,
		}})
		close(writeDone)
	}()
	close(releaseMigration)

	select {
	case <-writeDone:
	case <-time.After(35 * time.Second):
		t.Fatal("write did not complete after identity migration released the WAL writer")
	}
	if err := store.WaitForMaintenance(35 * time.Second); err != nil {
		t.Fatalf("wait for identity migration: %v", err)
	}
	issue1124AssertConsolidatedIndexes(t, store)

	var value float64
	if err := store.db.QueryRow(`
		SELECT value FROM metrics
		WHERE resource_type = 'vm' AND resource_id = 'during-migration'
			AND metric_type = 'cpu' AND tier = 'raw' AND timestamp = 1700020000
	`).Scan(&value); err != nil {
		t.Fatalf("query write completed during migration: %v", err)
	}
	if value != 42 {
		t.Fatalf("write completed during migration value=%v, want 42", value)
	}
}

// TestMetricsWriteAmplificationInvariant persists 157,452 deterministic
// multi-provider samples, enough to force repeated B-tree splits and expose an
// accidentally restored identity index. The frame ceiling includes 14% margin
// over the consolidated schema's measured 35,030 frames; the v6.1.1 schema
// produces 50,516 frames and fails this invariant.
func TestMetricsWriteAmplificationInvariant(t *testing.T) {
	suppressTestLogs(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "metrics.db")
	cfg := DefaultConfig(dir)
	cfg.DBPath = dbPath
	cfg.FlushInterval = time.Hour
	cfg.RollupInterval = time.Hour
	cfg.RetentionRaw = 10 * 365 * 24 * time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	if err := store.WaitForMaintenance(30 * time.Second); err != nil {
		t.Fatalf("startup maintenance: %v", err)
	}
	issue1124AssertConsolidatedIndexes(t, store)

	store.db.SetMaxOpenConns(1)
	store.db.SetMaxIdleConns(1)
	if _, err := store.db.Exec(`PRAGMA wal_autocheckpoint=0`); err != nil {
		t.Fatalf("disable auto-checkpoint: %v", err)
	}
	if _, err := store.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatalf("reset WAL: %v", err)
	}

	const ticks = 12
	series := issue1124Estate()
	batches := issue1124ProviderBatches(series)
	base := time.Unix(1_700_000_000, 0).UTC()
	samples := 0
	for tick := 0; tick < ticks; tick++ {
		ts := base.Add(time.Duration(tick) * 10 * time.Second)
		for _, template := range batches {
			batch := make([]WriteMetric, len(template))
			for i, metric := range template {
				metric.Timestamp = ts
				metric.Value = float64((tick+i)%10_000) / 100
				batch[i] = metric
			}
			store.WriteBatchSync(batch)
			samples += len(batch)
		}
	}

	if samples != 157_452 {
		t.Fatalf("profile samples=%d, want 157452", samples)
	}
	var persisted int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&persisted); err != nil {
		t.Fatalf("count persisted metrics: %v", err)
	}
	if persisted != samples {
		t.Fatalf("persisted rows=%d, want %d", persisted, samples)
	}

	pageSize := issue1124PragmaInt(t, store, "page_size")
	walBytes := issue1124FileSize(t, dbPath+"-wal")
	walFrames := (walBytes - 32) / (pageSize + 24)
	const maxWALFrames = 40_000
	if walFrames > maxWALFrames {
		t.Fatalf(
			"WAL frames=%d for %d samples, limit=%d; redundant indexes or transaction churn regressed write amplification",
			walFrames,
			samples,
			maxWALFrames,
		)
	}

	var busy, logFrames, checkpointed int64
	if err := store.db.QueryRow(`PRAGMA wal_checkpoint(PASSIVE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if busy != 0 || logFrames != walFrames || checkpointed != walFrames {
		t.Fatalf(
			"checkpoint counters busy=%d log=%d checkpointed=%d, want busy=0 log=checkpointed=%d",
			busy,
			logFrames,
			checkpointed,
			walFrames,
		)
	}
}

// TestIssue1124WriteAmplificationProfile is an opt-in measurement harness for
// comparing released schemas and candidate changes under the same deterministic
// multi-provider estate. It reports WAL frames and per-B-tree dbstat pages
// separately, so retained database size is never used as a write-volume proxy.
//
// Run with PULSE_METRICS_WRITE_PROFILE=full (v6.1.1 layout) or consolidated
// and optionally PULSE_METRICS_WRITE_PROFILE_TICKS=N. The default 30 ticks
// persist 393,630 samples across 2,197 resources. Set
// PULSE_METRICS_WRITE_PROFILE_AUTOCHECKPOINT=1 for production checkpoint/fsync
// tracing, PULSE_METRICS_WRITE_PROFILE_READERS=1 for a paced concurrent reader,
// or PULSE_METRICS_WRITE_PROFILE_RETENTION=1 for retention/compaction counters.
func TestIssue1124WriteAmplificationProfile(t *testing.T) {
	mode := os.Getenv(issue1124ProfileEnv)
	if mode == "" {
		t.Skip("set " + issue1124ProfileEnv + " to run the write-amplification profile")
	}
	if mode != "full" && mode != "consolidated" && mode != "full-batched" &&
		mode != "consolidated-batched" && mode != "without-rowid" {
		t.Fatalf("%s must be full, consolidated, full-batched, consolidated-batched, or without-rowid, got %q", issue1124ProfileEnv, mode)
	}

	ticks := 30
	if raw := os.Getenv("PULSE_METRICS_WRITE_PROFILE_TICKS"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 720 {
			t.Fatalf("PULSE_METRICS_WRITE_PROFILE_TICKS must be between 1 and 720, got %q", raw)
		}
		ticks = parsed
	}

	suppressTestLogs(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "metrics.db")
	cfg := DefaultConfig(dir)
	cfg.DBPath = dbPath
	cfg.FlushInterval = time.Hour
	cfg.RollupInterval = time.Hour
	cfg.RetentionRaw = 10 * 365 * 24 * time.Hour

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.WaitForMaintenance(30 * time.Second); err != nil {
		_ = store.Close()
		t.Fatalf("startup maintenance: %v", err)
	}

	// Pin profiling to one configured connection so wal_autocheckpoint is
	// deterministic. Production's four-connection concurrency has separate SLO
	// coverage; this profile isolates bytes/pages written per logical sample.
	store.db.SetMaxOpenConns(1)
	store.db.SetMaxIdleConns(1)
	autoCheckpoint := os.Getenv("PULSE_METRICS_WRITE_PROFILE_AUTOCHECKPOINT") == "1"
	if !autoCheckpoint {
		if _, err := store.db.Exec(`PRAGMA wal_autocheckpoint=0`); err != nil {
			_ = store.Close()
			t.Fatalf("disable auto-checkpoint: %v", err)
		}
	}

	if mode == "full" || mode == "full-batched" {
		// Reconstruct the v6.1.1 schema so released and consolidated layouts
		// remain comparable after the production migration lands.
		if _, err := store.db.Exec(`
			DROP INDEX idx_metrics_lookup;
			CREATE INDEX idx_metrics_lookup
			ON metrics(resource_type, resource_id, metric_type, tier, timestamp);
			CREATE UNIQUE INDEX idx_metrics_unique
			ON metrics(resource_type, resource_id, metric_type, timestamp, tier);
		`); err != nil {
			_ = store.Close()
			t.Fatalf("install v6.1.1 index layout: %v", err)
		}
	}
	if mode == "without-rowid" {
		if _, err := store.db.Exec(`
			DROP TABLE metrics;
			CREATE TABLE metrics (
				resource_type TEXT NOT NULL,
				resource_id TEXT NOT NULL,
				metric_type TEXT NOT NULL,
				value REAL NOT NULL,
				min_value REAL,
				max_value REAL,
				timestamp INTEGER NOT NULL,
				tier TEXT NOT NULL DEFAULT 'raw',
				PRIMARY KEY(resource_type, resource_id, metric_type, tier, timestamp)
			) WITHOUT ROWID;
			CREATE INDEX idx_metrics_tier_time ON metrics(tier, timestamp);
			CREATE INDEX idx_metrics_query_all
			ON metrics(resource_type, resource_id, tier, timestamp, metric_type);
		`); err != nil {
			_ = store.Close()
			t.Fatalf("install WITHOUT ROWID schema: %v", err)
		}
	}

	if _, err := store.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		_ = store.Close()
		t.Fatalf("reset WAL: %v", err)
	}
	_ = issue1124DBStatus(t, store, sqlite3.DBStatusCacheWrite, true)
	_ = issue1124DBStatus(t, store, sqlite3.DBStatusCacheSpill, true)
	processWritesBefore := issue1124ProcessWriteBytes()

	series := issue1124Estate()
	batches := issue1124ProviderBatches(series)
	samplesPerTick := 0
	for _, batch := range batches {
		samplesPerTick += len(batch)
	}

	var readCount atomic.Int64
	var maxReadNanos atomic.Int64
	var readStop chan struct{}
	var readDone chan struct{}
	var readErr atomic.Value
	base := time.Unix(1_700_000_000, 0).UTC()
	if os.Getenv("PULSE_METRICS_WRITE_PROFILE_READERS") == "1" {
		readStop = make(chan struct{})
		readDone = make(chan struct{})
		go func() {
			defer close(readDone)
			for {
				select {
				case <-readStop:
					return
				default:
				}
				started := time.Now()
				_, queryErr := store.Query(
					"dockerContainer",
					"docker-container-0000",
					"cpu",
					base.Add(-time.Minute),
					base.Add(time.Duration(ticks+1)*10*time.Second),
					0,
				)
				if queryErr != nil {
					readErr.Store(queryErr)
					return
				}
				readCount.Add(1)
				elapsed := time.Since(started).Nanoseconds()
				for {
					previous := maxReadNanos.Load()
					if elapsed <= previous || maxReadNanos.CompareAndSwap(previous, elapsed) {
						break
					}
				}
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	logicalBytes := int64(0)
	started := time.Now()
	for tick := 0; tick < ticks; tick++ {
		ts := base.Add(time.Duration(tick) * 10 * time.Second)
		var tickBatch []WriteMetric
		for _, template := range batches {
			batch := make([]WriteMetric, len(template))
			for i, metric := range template {
				metric.Timestamp = ts
				metric.Value = float64((tick+i)%10_000) / 100
				batch[i] = metric
				logicalBytes += issue1124LogicalBytes(metric)
			}
			if mode == "full-batched" || mode == "consolidated-batched" {
				tickBatch = append(tickBatch, batch...)
			} else {
				store.WriteBatchSync(batch)
			}
		}
		if len(tickBatch) > 0 {
			store.WriteBatchSync(tickBatch)
		}
	}
	writeElapsed := time.Since(started)
	if readStop != nil {
		close(readStop)
		<-readDone
	}
	if value := readErr.Load(); value != nil {
		_ = store.Close()
		t.Fatalf("concurrent query: %v", value)
	}

	pageSize := issue1124PragmaInt(t, store, "page_size")
	pageCount := issue1124PragmaInt(t, store, "page_count")
	freelistCount := issue1124PragmaInt(t, store, "freelist_count")
	walBytes := issue1124FileSize(t, dbPath+"-wal")
	mainBefore := issue1124FileSize(t, dbPath)
	cacheWritesBefore := issue1124DBStatus(t, store, sqlite3.DBStatusCacheWrite, false)
	cacheSpills := issue1124DBStatus(t, store, sqlite3.DBStatusCacheSpill, false)
	processWritesAfterWAL := issue1124ProcessWriteBytes()
	walFrames := int64(0)
	if walBytes >= 32 {
		walFrames = (walBytes - 32) / (pageSize + 24)
	}

	var busy, logFrames, checkpointed int64
	if err := store.db.QueryRow(`PRAGMA wal_checkpoint(PASSIVE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		_ = store.Close()
		t.Fatalf("checkpoint: %v", err)
	}
	if busy != 0 {
		_ = store.Close()
		t.Fatalf("checkpoint remained busy: busy=%d log=%d checkpointed=%d", busy, logFrames, checkpointed)
	}
	mainAfter := issue1124FileSize(t, dbPath)
	cacheWritesAfter := issue1124DBStatus(t, store, sqlite3.DBStatusCacheWrite, false)
	processWritesAfterCheckpoint := issue1124ProcessWriteBytes()
	dbStats := issue1124ReadDBStats(t, store)

	expectedSamples := ticks * samplesPerTick
	var rowCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&rowCount); err != nil {
		_ = store.Close()
		t.Fatalf("count persisted metrics: %v", err)
	}
	if rowCount != expectedSamples {
		_ = store.Close()
		t.Fatalf("persisted rows=%d, expected=%d", rowCount, expectedSamples)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	restartStarted := time.Now()
	restarted, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("restart store: %v", err)
	}
	if err := restarted.WaitForMaintenance(30 * time.Second); err != nil {
		_ = restarted.Close()
		t.Fatalf("restart maintenance: %v", err)
	}
	var integrity string
	if err := restarted.db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		_ = restarted.Close()
		t.Fatalf("restart integrity check: %v", err)
	}
	if integrity != "ok" {
		_ = restarted.Close()
		t.Fatalf("restart integrity check returned %q", integrity)
	}
	var restartedRows int
	if err := restarted.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&restartedRows); err != nil {
		_ = restarted.Close()
		t.Fatalf("restart count: %v", err)
	}
	if restartedRows != expectedSamples {
		_ = restarted.Close()
		t.Fatalf("restart rows=%d, expected=%d", restartedRows, expectedSamples)
	}
	restartElapsed := time.Since(restartStarted)
	restartMainBytes := issue1124FileSize(t, dbPath)
	restartPageCount := issue1124PragmaInt(t, restarted, "page_count")
	restartFreelist := issue1124PragmaInt(t, restarted, "freelist_count")
	var (
		retentionElapsed   time.Duration
		retentionRows      = restartedRows
		retentionMainBytes = restartMainBytes
		retentionPageCount = restartPageCount
		retentionFreelist  = restartFreelist
	)
	if os.Getenv("PULSE_METRICS_WRITE_PROFILE_RETENTION") == "1" {
		restarted.config.RetentionRaw = time.Nanosecond
		restarted.config.RetentionMinute = time.Nanosecond
		restarted.config.RetentionHourly = time.Nanosecond
		restarted.config.RetentionDaily = time.Nanosecond
		retentionStarted := time.Now()
		restarted.runRetention()
		retentionElapsed = time.Since(retentionStarted)
		if err := restarted.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&retentionRows); err != nil {
			_ = restarted.Close()
			t.Fatalf("count rows after retention: %v", err)
		}
		retentionMainBytes = issue1124FileSize(t, dbPath)
		retentionPageCount = issue1124PragmaInt(t, restarted, "page_count")
		retentionFreelist = issue1124PragmaInt(t, restarted, "freelist_count")
	}
	if err := restarted.Close(); err != nil {
		t.Fatalf("close restarted store: %v", err)
	}

	report := issue1124ProfileReport{
		SchemaMode:           mode,
		AutoCheckpoint:       autoCheckpoint,
		Ticks:                ticks,
		Resources:            len(series),
		Samples:              expectedSamples,
		WriteCalls:           ticks * len(batches),
		LogicalPayloadBytes:  logicalBytes,
		Elapsed:              writeElapsed,
		WALBytes:             walBytes,
		WALFrames:            walFrames,
		CacheWritesBefore:    cacheWritesBefore,
		CacheWritesAfter:     cacheWritesAfter,
		CacheSpills:          cacheSpills,
		PhysicalWriteBytes:   issue1124CounterDelta(processWritesBefore, processWritesAfterWAL),
		CheckpointWriteBytes: issue1124CounterDelta(processWritesAfterWAL, processWritesAfterCheckpoint),
		CheckpointedFrames:   checkpointed,
		MainBytesBefore:      mainBefore,
		MainBytesAfter:       mainAfter,
		PageSize:             pageSize,
		PageCount:            pageCount,
		FreelistCount:        freelistCount,
		ReadCount:            readCount.Load(),
		MaxReadLatency:       time.Duration(maxReadNanos.Load()),
		RestartElapsed:       restartElapsed,
		RestartMainBytes:     restartMainBytes,
		RestartPageCount:     restartPageCount,
		RestartFreelist:      restartFreelist,
		RetentionElapsed:     retentionElapsed,
		RetentionRows:        retentionRows,
		RetentionMainBytes:   retentionMainBytes,
		RetentionPageCount:   retentionPageCount,
		RetentionFreelist:    retentionFreelist,
		DBStats:              dbStats,
	}
	if mode == "full-batched" || mode == "consolidated-batched" {
		report.WriteCalls = ticks
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	t.Logf("ISSUE1124_PROFILE %s", encoded)
}

func issue1124Estate() []issue1124Series {
	const (
		dockerContainers = 975
		proxmoxGuests    = 400
		agentHosts       = 120
		kubernetesPods   = 300
		storageTargets   = 250
		physicalDisks    = 152
	)

	series := make([]issue1124Series, 0, dockerContainers+proxmoxGuests+agentHosts+kubernetesPods+storageTargets+physicalDisks)
	appendSeries := func(resourceType, prefix string, count int, metricTypes ...string) {
		for i := 0; i < count; i++ {
			series = append(series, issue1124Series{
				resourceType: resourceType,
				resourceID:   fmt.Sprintf("%s-%04d", prefix, i),
				metricTypes:  metricTypes,
			})
		}
	}
	appendSeries("dockerContainer", "docker-container", dockerContainers, "cpu", "memory", "disk", "netin", "netout", "diskread", "diskwrite")
	appendSeries("vm", "proxmox-guest", proxmoxGuests, "cpu", "memory", "disk", "netin", "netout", "diskread", "diskwrite")
	appendSeries("agent", "agent-host", agentHosts, "cpu", "memory", "disk", "netin", "netout", "diskread", "diskwrite")
	appendSeries("pod", "kubernetes-pod", kubernetesPods, "cpu", "memory", "netin", "netout")
	appendSeries("storage", "storage-target", storageTargets, "usage", "used", "total", "avail")
	appendSeries("disk", "physical-disk", physicalDisks, "temperature", "wearout", "power_on_hours")
	return series
}

func issue1124ProviderBatches(series []issue1124Series) [][]WriteMetric {
	byProvider := make(map[string][]WriteMetric)
	order := []string{"dockerContainer", "vm", "agent", "pod", "storage", "disk"}
	for _, resource := range series {
		for _, metricType := range resource.metricTypes {
			byProvider[resource.resourceType] = append(byProvider[resource.resourceType], WriteMetric{
				ResourceType: resource.resourceType,
				ResourceID:   resource.resourceID,
				MetricType:   metricType,
				Tier:         TierRaw,
			})
		}
	}
	batches := make([][]WriteMetric, 0, len(order))
	for _, provider := range order {
		batches = append(batches, byProvider[provider])
	}
	return batches
}

func issue1124LogicalBytes(metric WriteMetric) int64 {
	// Application payload only: encoded strings plus value and timestamp.
	// SQLite page headers, record headers, rowids, indexes, WAL headers, and
	// checkpoint rewrites are intentionally excluded from this denominator.
	return int64(len(metric.ResourceType)+len(metric.ResourceID)+len(metric.MetricType)+len(metric.Tier)) + 16
}

func issue1124PragmaInt(t *testing.T, store *Store, name string) int64 {
	t.Helper()
	var value int64
	if err := store.db.QueryRow("PRAGMA " + name).Scan(&value); err != nil {
		t.Fatalf("PRAGMA %s: %v", name, err)
	}
	return value
}

func issue1124DBStatus(t *testing.T, store *Store, op sqlite3.DBStatusOp, reset bool) int64 {
	t.Helper()
	conn, err := store.db.DB.Conn(context.Background())
	if err != nil {
		t.Fatalf("open SQLite status connection: %v", err)
	}
	defer conn.Close()

	var current int
	if err := conn.Raw(func(driverConn any) error {
		status, ok := driverConn.(sqlite3.DBStatus)
		if !ok {
			return fmt.Errorf("SQLite driver does not expose DBStatus")
		}
		value, _, err := status.Status(op, reset)
		current = value
		return err
	}); err != nil {
		t.Fatalf("read SQLite DB status %d: %v", op, err)
	}
	return int64(current)
}

func issue1124FileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Size()
}

func issue1124ProcessWriteBytes() int64 {
	data, err := os.ReadFile("/proc/self/io")
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "write_bytes" {
			continue
		}
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err == nil {
			return parsed
		}
	}
	return -1
}

func issue1124CounterDelta(before, after int64) int64 {
	if before < 0 || after < before {
		return -1
	}
	return after - before
}

func issue1124CopyFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read backup source %s: %v", source, err)
	}
	if err := os.WriteFile(destination, data, 0o600); err != nil {
		t.Fatalf("write backup destination %s: %v", destination, err)
	}
}

func issue1124ReadDBStats(t *testing.T, store *Store) []issue1124DBStat {
	t.Helper()
	rows, err := store.db.Query(`
		SELECT name, COUNT(*), SUM(pgsize), SUM(payload)
		FROM dbstat
		WHERE name IN (
			'metrics',
			'sqlite_sequence',
			'idx_metrics_lookup',
			'idx_metrics_identity',
			'idx_metrics_tier_time',
			'idx_metrics_query_all',
			'idx_metrics_unique'
		)
		GROUP BY name
		ORDER BY name
	`)
	if err != nil {
		t.Fatalf("query dbstat: %v", err)
	}
	defer rows.Close()

	stats := make([]issue1124DBStat, 0, 7)
	for rows.Next() {
		var stat issue1124DBStat
		if err := rows.Scan(&stat.Name, &stat.Pages, &stat.PageBytes, &stat.PayloadBytes); err != nil {
			t.Fatalf("scan dbstat: %v", err)
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("dbstat rows: %v", err)
	}
	return stats
}

func issue1124CreateLegacyStore(t *testing.T, dbPath string, rows int, duplicates bool) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy store: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			metric_type TEXT NOT NULL,
			value REAL NOT NULL,
			min_value REAL,
			max_value REAL,
			timestamp INTEGER NOT NULL,
			tier TEXT NOT NULL DEFAULT 'raw'
		);
		CREATE INDEX idx_metrics_lookup
		ON metrics(resource_type, resource_id, metric_type, tier, timestamp);
		CREATE INDEX idx_metrics_tier_time ON metrics(tier, timestamp);
		CREATE INDEX idx_metrics_query_all
		ON metrics(resource_type, resource_id, tier, timestamp, metric_type);
		CREATE TABLE metrics_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if !duplicates {
		if _, err := db.Exec(`
			CREATE UNIQUE INDEX idx_metrics_unique
			ON metrics(resource_type, resource_id, metric_type, timestamp, tier)
		`); err != nil {
			t.Fatalf("create legacy unique index: %v", err)
		}
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin legacy seed: %v", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO metrics(resource_type, resource_id, metric_type, value, timestamp, tier)
		VALUES('vm', ?, 'cpu', ?, ?, 'raw')
	`)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("prepare legacy seed: %v", err)
	}
	for i := 0; i < rows; i++ {
		resourceIndex := i
		timestamp := int64(1_700_000_000 + i)
		if duplicates {
			resourceIndex = 0
			timestamp = 1_700_000_000
		}
		if _, err := stmt.Exec(fmt.Sprintf("legacy-%04d", resourceIndex), float64(i), timestamp); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			t.Fatalf("seed legacy row %d: %v", i, err)
		}
	}
	if err := stmt.Close(); err != nil {
		_ = tx.Rollback()
		t.Fatalf("close legacy seed statement: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit legacy seed: %v", err)
	}
}

func issue1124AssertConsolidatedIndexes(t *testing.T, store *Store) {
	t.Helper()
	matches, err := store.metricsIndexMatches("idx_metrics_lookup", true, metricsIdentityColumns)
	if err != nil {
		t.Fatalf("inspect consolidated lookup index: %v", err)
	}
	if !matches {
		t.Fatal("idx_metrics_lookup is not the expected unique identity/range index")
	}
	exists, err := store.metricsIndexExists("idx_metrics_unique")
	if err != nil {
		t.Fatalf("inspect obsolete unique index: %v", err)
	}
	if exists {
		t.Fatal("obsolete idx_metrics_unique still exists")
	}

	rows, err := store.db.Query(`PRAGMA index_list(metrics)`)
	if err != nil {
		t.Fatalf("list consolidated indexes: %v", err)
	}
	defer rows.Close()
	indexes := make(map[string]bool)
	for rows.Next() {
		var (
			sequence int
			name     string
			unique   int
			origin   string
			partial  int
		)
		if err := rows.Scan(&sequence, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan consolidated index: %v", err)
		}
		indexes[name] = unique == 1
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("consolidated index rows: %v", err)
	}
	want := map[string]bool{
		"idx_metrics_lookup":    true,
		"idx_metrics_query_all": false,
		"idx_metrics_tier_time": false,
	}
	if len(indexes) != len(want) {
		t.Fatalf("metrics indexes=%v, want %v", indexes, want)
	}
	for name, unique := range want {
		if got, ok := indexes[name]; !ok || got != unique {
			t.Fatalf("metrics index %s unique=%v present=%v, want unique=%v", name, got, ok, unique)
		}
	}
}
