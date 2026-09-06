// Package metrics provides persistent storage for time-series metrics data
// using SQLite for durability across restarts.
package metrics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	pdb "github.com/rcourtman/pulse-go-rewrite/pkg/db"
)

const (
	privateDirPerm  = 0o700
	privateFilePerm = 0o600
)

// Tier represents the granularity of stored metrics
type Tier string

const (
	TierRaw    Tier = "raw"    // Raw data, ~5s intervals
	TierMinute Tier = "minute" // 1-minute averages
	TierHourly Tier = "hourly" // 1-hour averages
	TierDaily  Tier = "daily"  // 1-day averages
)

// MetricPoint represents a single metric data point
type MetricPoint struct {
	Timestamp time.Time
	Value     float64
	Min       float64 // For aggregated data
	Max       float64 // For aggregated data
}

// StoreConfig holds configuration for the metrics store
type StoreConfig struct {
	DBPath          string
	WriteBufferSize int           // Number of records to buffer before batch write
	FlushInterval   time.Duration // Max time between flushes
	RollupInterval  time.Duration // How often to aggregate raw samples into coarser tiers
	RetentionRaw    time.Duration // How long to keep raw data
	RetentionMinute time.Duration // How long to keep minute data
	RetentionHourly time.Duration // How long to keep hourly data
	RetentionDaily  time.Duration // How long to keep daily data
}

const (
	minRollupInterval     = 5 * time.Minute
	defaultRollupInterval = 15 * time.Minute
	maxRollupChunkWindow  = 5 * time.Minute
	maxRollupChunksPerRun = 128
)

// DefaultConfig returns sensible defaults for metrics storage
func DefaultConfig(dataDir string) StoreConfig {
	dbPath := filepath.Join(dataDir, "metrics.db")
	if resolvedDBPath, err := resolveStoreDBPath(dbPath); err == nil {
		dbPath = resolvedDBPath
	}

	return StoreConfig{
		DBPath: dbPath,
		// Large installs can enqueue hundreds of metric points per poll cycle.
		// A larger buffer keeps those writes inside a single SQLite transaction
		// more often, which materially reduces WAL churn on SSD-backed setups.
		WriteBufferSize: 500,
		FlushInterval:   5 * time.Second,
		RollupInterval:  defaultRollupInterval,
		RetentionRaw:    2 * time.Hour,
		RetentionMinute: 24 * time.Hour,
		RetentionHourly: 7 * 24 * time.Hour,
		RetentionDaily:  90 * 24 * time.Hour,
	}
}

func resolveStoreDBPath(dbPath string) (string, error) {
	trimmedPath := strings.TrimSpace(dbPath)
	if trimmedPath == "" {
		return "", fmt.Errorf("metrics database path is required")
	}

	cleanedPath := filepath.Clean(trimmedPath)
	dir, err := securityutil.NormalizeStorageDir(filepath.Dir(cleanedPath))
	if err != nil {
		return "", fmt.Errorf("resolve metrics database directory: %w", err)
	}

	resolvedPath, err := securityutil.JoinStorageLeaf(dir, filepath.Base(cleanedPath))
	if err != nil {
		return "", fmt.Errorf("resolve metrics database path: %w", err)
	}

	return resolvedPath, nil
}

func normalizeRollupInterval(interval, rawRetention time.Duration) time.Duration {
	if interval <= 0 {
		interval = defaultRollupInterval
	}
	if interval < minRollupInterval {
		interval = minRollupInterval
	}
	if rawRetention <= 0 {
		return interval
	}

	maxInterval := rawRetention / 2
	if maxInterval < minRollupInterval {
		maxInterval = minRollupInterval
	}
	if interval > maxInterval {
		return maxInterval
	}
	return interval
}

// bufferedMetric holds a metric waiting to be written
type bufferedMetric struct {
	resourceType string
	resourceID   string
	metricType   string
	value        float64
	timestamp    time.Time
	tier         Tier
}

type writeRequest struct {
	metrics             []bufferedMetric
	availability        []AvailabilityObservation
	availabilityDeletes []string
	done                chan struct{}
}

type metricBatchKey struct {
	resourceType  string
	resourceID    string
	metricType    string
	timestampUnix int64
	tier          Tier
}

// WriteMetric represents a metric sample to be written synchronously.
type WriteMetric struct {
	ResourceType string
	ResourceID   string
	MetricType   string
	Value        float64
	Timestamp    time.Time
	Tier         Tier
}

// SeriesKey identifies one stored metric series in the normalized form the
// write path persists (lower-cased resource/metric types, trimmed id).
type SeriesKey struct {
	ResourceType string
	ResourceID   string
	MetricType   string
}

// NormalizedSeriesKey builds the SeriesKey the write path would store for the
// given identifiers, so callers can match MaxTimestampsForTier results.
func NormalizedSeriesKey(resourceType, resourceID, metricType string) SeriesKey {
	return SeriesKey{
		ResourceType: normalizeMetricResourceType(resourceType),
		ResourceID:   normalizeMetricIdentifier(resourceID),
		MetricType:   normalizeMetricType(metricType),
	}
}

type maintenanceRequest struct {
	run  func()
	done chan struct{}
}

var (
	startupMaintenanceHook       func()
	metricsIdentityMigrationHook func()
)

// Store provides persistent metrics storage
type Store struct {
	db     *pdb.InstrumentedDB
	config StoreConfig

	// Cache compiled read SQL only. Results always come from the current
	// database snapshot. Bound the number of parameter-count shapes retained.
	readMu         sync.Mutex
	readStatements map[string]*pdb.InstrumentedStmt

	// SQL templates contain only parameter positions, never query values.
	// Keep this lock separate from statement preparation, which may wait for
	// a database connection while another caller owns a read transaction.
	queryMu         sync.Mutex
	readQueryShapes map[retainedQueryShape]string

	// Write buffer
	bufferMu sync.Mutex
	buffer   []bufferedMetric

	// Background workers
	writeCh                    chan writeRequest
	maintenanceCh              chan maintenanceRequest
	stopCh                     chan struct{}
	doneCh                     chan struct{}
	maintenanceDoneCh          chan struct{}
	stopOnce                   sync.Once
	stopping                   atomic.Bool
	syncWaitWarnNano           atomic.Int64
	identityMigrationPending   atomic.Bool
	commercialRetentionSeconds atomic.Int64
	commercialPurgeEligibleAt  atomic.Int64

	// startupHook is captured once at construction so a store that outlives
	// the test that built it never invokes a later test's hook.
	startupHook func()
}

// SetCommercialHistoryRetention applies a delayed commercial ceiling to the
// configured physical retention. A zero or negative day count clears the
// ceiling. Access control is enforced elsewhere immediately; this method only
// governs when excess history becomes physically purgeable.
func (s *Store) SetCommercialHistoryRetention(days int, purgeEligibleAt time.Time) {
	if s == nil || days <= 0 || purgeEligibleAt.IsZero() {
		if s != nil {
			s.commercialRetentionSeconds.Store(0)
			s.commercialPurgeEligibleAt.Store(0)
		}
		return
	}
	s.commercialRetentionSeconds.Store(int64(time.Duration(days) * 24 * time.Hour / time.Second))
	s.commercialPurgeEligibleAt.Store(purgeEligibleAt.UTC().Unix())
}

func (s *Store) effectiveRetention(base time.Duration, now time.Time) time.Duration {
	seconds := s.commercialRetentionSeconds.Load()
	eligibleAt := s.commercialPurgeEligibleAt.Load()
	if seconds <= 0 || eligibleAt <= 0 || now.UTC().Unix() < eligibleAt {
		return base
	}
	commercial := time.Duration(seconds) * time.Second
	if commercial < base {
		return commercial
	}
	return base
}

// NewStore creates a new metrics store with the given configuration
func NewStore(config StoreConfig) (*Store, error) {
	resolvedDBPath, err := resolveStoreDBPath(config.DBPath)
	if err != nil {
		return nil, err
	}
	config.DBPath = resolvedDBPath
	config.RollupInterval = normalizeRollupInterval(config.RollupInterval, config.RetentionRaw)

	dir := filepath.Dir(config.DBPath)
	if err := ensureOwnerOnlyDir(dir); err != nil {
		return nil, fmt.Errorf("failed to prepare metrics directory: %w", err)
	}

	if err := rejectSymlinkOrNonRegular(config.DBPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	// Open database with pragmas in DSN so every pool connection is configured
	dsn := config.DBPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
			// auto_vacuum is deliberately NOT set here. It is a persistent
			// database property that migrateAutoVacuum establishes once at
			// startup; as a per-connection pragma it replays as a header
			// write on every new pool connection, which blocks connection
			// creation behind the active writer (#1601).
			// Checkpoint less aggressively so high-cardinality installs don't
			// keep rewriting tiny WAL segments back into the main DB file.
			"wal_autocheckpoint(4000)",
		},
	}.Encode()
	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open metrics database: %w", err)
	}

	// Configure connection pool. Writes are already funnelled: the flush,
	// rollup, retention, and maintenance paths run on the single background
	// worker goroutine, and the WriteBatchSync poller path serializes on the
	// WAL write lock via busy_timeout. What a pool of one actually did was
	// queue every UI history read behind those commits, which froze charts
	// whenever a commit picked up a WAL checkpoint (#1601). WAL mode reads
	// snapshot-isolated on their own connections, so give readers room.
	rawDB.SetMaxOpenConns(4)
	rawDB.SetMaxIdleConns(4)
	rawDB.SetConnMaxLifetime(0)

	db := pdb.Wrap(rawDB, "metrics")

	store := &Store{
		db:                db,
		config:            config,
		buffer:            make([]bufferedMetric, 0, config.WriteBufferSize),
		writeCh:           make(chan writeRequest, 100), // Buffer for write batches
		maintenanceCh:     make(chan maintenanceRequest, 1),
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
		maintenanceDoneCh: make(chan struct{}),
		startupHook:       startupMaintenanceHook,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	store.migrateLegacyHostResourceType()

	if err := hardenSQLiteArtifacts(config.DBPath); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to secure metrics db files: %w", err)
	}

	// Keep metric ingestion independent from rollup, retention, and one-time
	// startup maintenance. SQLite remains the write-serialization boundary,
	// but long maintenance scheduling no longer prevents the write queue from
	// being drained (#1601).
	go store.backgroundWorker()
	go store.maintenanceWorker()
	store.enqueueMaintenance(store.runStartupMaintenance)

	log.Info().
		Str("path", config.DBPath).
		Int("bufferSize", config.WriteBufferSize).
		Dur("rollupInterval", config.RollupInterval).
		Msg("Metrics store initialized")

	return store, nil
}

// initSchema creates the database schema if it doesn't exist
func (s *Store) initSchema() error {
	schema := `
		-- Main metrics table
		CREATE TABLE IF NOT EXISTS metrics (
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

		-- Index for retention pruning
		CREATE INDEX IF NOT EXISTS idx_metrics_tier_time 
		ON metrics(tier, timestamp);

		-- Covering index for Unified History (QueryAll) performance
		CREATE INDEX IF NOT EXISTS idx_metrics_query_all
		ON metrics(resource_type, resource_id, tier, timestamp, metric_type);

		-- Metadata table for tracking rollup state
		CREATE TABLE IF NOT EXISTS metrics_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	if err := s.ensureMetricsIdentityIndex(); err != nil {
		return err
	}
	if err := s.initAvailabilityHistorySchema(); err != nil {
		return err
	}

	log.Debug().Msg("Metrics schema initialized")
	return nil
}

var metricsIdentityColumns = []string{"resource_type", "resource_id", "metric_type", "tier", "timestamp"}

// ensureMetricsIdentityIndex keeps one B-tree for both metric identity and
// single-series range queries. Older databases have two indexes containing the
// same five columns in different orders: idx_metrics_lookup serves reads while
// idx_metrics_unique enforces identity. Every insert dirties both trees, which
// materially amplifies WAL and checkpoint writes.
func (s *Store) ensureMetricsIdentityIndex() error {
	lookupCurrent, err := s.metricsIndexMatches("idx_metrics_lookup", true, metricsIdentityColumns)
	if err != nil {
		return fmt.Errorf("inspect metrics identity index: %w", err)
	}
	legacyUniqueExists, err := s.metricsIndexExists("idx_metrics_unique")
	if err != nil {
		return fmt.Errorf("inspect legacy metrics unique index: %w", err)
	}

	if legacyUniqueExists {
		// v6.1.1 and earlier already have an authoritative unique index. Keep
		// serving with that crash-safe schema and defer the O(rows) rebuild to
		// the startup-maintenance worker so NewStore latency stays bounded.
		s.identityMigrationPending.Store(true)
		log.Info().Msg("Scheduled metrics identity index consolidation")
		return nil
	}
	if lookupCurrent {
		return nil
	}

	return s.migrateMetricsIdentityIndex()
}

func (s *Store) migrateMetricsIdentityIndex() error {
	if err := s.replaceMetricsIdentityIndex(); err == nil {
		log.Info().Msg("Consolidated metrics lookup and identity indexes")
		return nil
	} else if !isUniqueConstraintError(err) {
		return fmt.Errorf("consolidate metrics identity index: %w", err)
	}

	log.Warn().Msg("Duplicate metrics detected; deduplicating before consolidating identity index")
	if err := s.deduplicateMetrics(); err != nil {
		return err
	}
	if err := s.replaceMetricsIdentityIndex(); err != nil {
		return fmt.Errorf("consolidate metrics identity index after dedupe: %w", err)
	}

	log.Info().Msg("Metrics deduplicated and identity index consolidated")
	return nil
}

func (s *Store) replaceMetricsIdentityIndex() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin identity index migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DROP INDEX IF EXISTS idx_metrics_lookup`); err != nil {
		return fmt.Errorf("drop old lookup index: %w", err)
	}
	if metricsIdentityMigrationHook != nil {
		metricsIdentityMigrationHook()
	}
	if _, err := tx.Exec(`
		CREATE UNIQUE INDEX idx_metrics_lookup
		ON metrics(resource_type, resource_id, metric_type, tier, timestamp)
	`); err != nil {
		return fmt.Errorf("create consolidated lookup index: %w", err)
	}
	if _, err := tx.Exec(`DROP INDEX IF EXISTS idx_metrics_unique`); err != nil {
		return fmt.Errorf("drop old unique index: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit identity index migration: %w", err)
	}
	return nil
}

func (s *Store) deduplicateMetrics() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin metrics dedupe transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
		DELETE FROM metrics
		WHERE rowid NOT IN (
			SELECT MIN(rowid)
			FROM metrics
			GROUP BY resource_type, resource_id, metric_type, tier, timestamp
		)
	`); err != nil {
		return fmt.Errorf("dedupe metrics: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit metrics dedupe: %w", err)
	}
	return nil
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unique") ||
		strings.Contains(lower, "constraint") ||
		strings.Contains(lower, "duplicate")
}

func (s *Store) metricsIndexExists(name string) (bool, error) {
	rows, err := s.db.Query(`PRAGMA index_list(metrics)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			sequence int
			index    string
			unique   int
			origin   string
			partial  int
		)
		if err := rows.Scan(&sequence, &index, &unique, &origin, &partial); err != nil {
			return false, err
		}
		if index == name {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) metricsIndexMatches(name string, wantUnique bool, wantColumns []string) (bool, error) {
	rows, err := s.db.Query(`PRAGMA index_list(metrics)`)
	if err != nil {
		return false, err
	}

	found := false
	for rows.Next() {
		var (
			sequence int
			index    string
			unique   int
			origin   string
			partial  int
		)
		if err := rows.Scan(&sequence, &index, &unique, &origin, &partial); err != nil {
			_ = rows.Close()
			return false, err
		}
		if index == name {
			found = (unique == 1) == wantUnique && partial == 0
			break
		}
	}
	if err := rows.Close(); err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	columnRows, err := s.db.Query(`
		SELECT name
		FROM pragma_index_info(?)
		ORDER BY seqno
	`, name)
	if err != nil {
		return false, err
	}
	defer columnRows.Close()

	columns := make([]string, 0, len(wantColumns))
	for columnRows.Next() {
		var columnName string
		if err := columnRows.Scan(&columnName); err != nil {
			return false, err
		}
		columns = append(columns, columnName)
	}
	if err := columnRows.Err(); err != nil {
		return false, err
	}
	if len(columns) != len(wantColumns) {
		return false, nil
	}
	for i := range columns {
		if columns[i] != wantColumns[i] {
			return false, nil
		}
	}
	return true, nil
}

// migrateAutoVacuum ensures the database uses incremental auto-vacuum.
// SQLite cannot switch from NONE to INCREMENTAL without a full VACUUM to
// restructure the file, so we detect and convert on first run after upgrade.
func (s *Store) migrateAutoVacuum() {
	var mode int
	if err := s.db.QueryRow("PRAGMA auto_vacuum").Scan(&mode); err != nil {
		log.Debug().Err(err).Msg("Failed to check auto_vacuum mode")
		return
	}
	if mode == 2 { // already INCREMENTAL
		return
	}

	log.Info().Int("current_mode", mode).Msg("Converting metrics database to incremental auto-vacuum (one-time migration)")
	start := time.Now()

	// Set the desired mode then VACUUM to restructure the file.
	if _, err := s.db.Exec("PRAGMA auto_vacuum = INCREMENTAL"); err != nil {
		log.Warn().Err(err).Msg("Failed to set auto_vacuum mode")
		return
	}
	if _, err := s.db.Exec("VACUUM"); err != nil {
		log.Warn().Err(err).Msg("Auto-vacuum migration VACUUM failed (will retry next restart)")
		return
	}

	log.Info().Dur("duration", time.Since(start)).Msg("Metrics database auto-vacuum migration complete")
}

// migrateLegacyHostResourceType rewrites legacy v5 `resource_type=host` rows to
// canonical v6 `resource_type=agent`. This keeps reads/writes agent-only while
// preserving historical data.
func (s *Store) migrateLegacyHostResourceType() {
	tx, err := s.db.Begin()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to start legacy host->agent metrics migration")
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Reinsert legacy rows with canonical type, keeping any existing canonical
	// records when duplicates collide with the unique index.
	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO metrics (
			resource_type, resource_id, metric_type, value, min_value, max_value, timestamp, tier
		)
		SELECT
			'agent', resource_id, metric_type, value, min_value, max_value, timestamp, tier
		FROM metrics
		WHERE resource_type = 'host'
	`); err != nil {
		log.Warn().Err(err).Msg("Failed to copy legacy host metrics rows")
		return
	}

	deleteResult, err := tx.Exec(`DELETE FROM metrics WHERE resource_type = 'host'`)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to delete legacy host metrics rows")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Warn().Err(err).Msg("Failed to commit legacy host->agent metrics migration")
		return
	}

	rowsDeleted, err := deleteResult.RowsAffected()
	if err != nil {
		log.Debug().Err(err).Msg("Failed to read affected rows for legacy host->agent migration")
		return
	}
	if rowsDeleted > 0 {
		log.Info().Int64("rows", rowsDeleted).Msg("Migrated legacy host metrics rows to agent")
	}
}

func normalizeMetricResourceType(resourceType string) string {
	return strings.ToLower(strings.TrimSpace(resourceType))
}

func normalizeMetricType(metricType string) string {
	return strings.ToLower(strings.TrimSpace(metricType))
}

func normalizeMetricIdentifier(value string) string {
	return strings.TrimSpace(value)
}

func isLegacyMetricResourceType(resourceType string) bool {
	return strings.EqualFold(strings.TrimSpace(resourceType), "host")
}

func isSupportedMetricTier(tier Tier) bool {
	switch tier {
	case TierRaw, TierMinute, TierHourly, TierDaily:
		return true
	default:
		return false
	}
}

func validateMetricWrite(resourceType, resourceID, metricType string, tier Tier) (string, string, string, bool, string) {
	normalizedType := normalizeMetricResourceType(resourceType)
	if normalizedType == "" {
		return "", "", "", false, "empty resource type"
	}
	if isLegacyMetricResourceType(normalizedType) {
		return "", "", "", false, `unsupported legacy resource type "host"`
	}

	normalizedID := normalizeMetricIdentifier(resourceID)
	if normalizedID == "" {
		return "", "", "", false, "empty resource id"
	}

	normalizedMetric := normalizeMetricType(metricType)
	if normalizedMetric == "" {
		return "", "", "", false, "empty metric type"
	}

	if !isSupportedMetricTier(tier) {
		return "", "", "", false, fmt.Sprintf("unsupported metric tier %q", tier)
	}

	return normalizedType, normalizedID, normalizedMetric, true, ""
}

// Write adds a metric to the write buffer with the 'raw' tier by default
func (s *Store) Write(resourceType, resourceID, metricType string, value float64, timestamp time.Time) {
	s.WriteWithTier(resourceType, resourceID, metricType, value, timestamp, TierRaw)
}

// WriteWithTier adds a metric to the write buffer with a specific tier
func (s *Store) WriteWithTier(resourceType, resourceID, metricType string, value float64, timestamp time.Time, tier Tier) {
	if s.stopping.Load() {
		return
	}

	normalizedType, normalizedID, normalizedMetric, ok, reason := validateMetricWrite(resourceType, resourceID, metricType, tier)
	if !ok {
		log.Warn().
			Str("resource_type", resourceType).
			Str("resource_id", resourceID).
			Str("metric_type", metricType).
			Str("tier", string(tier)).
			Str("reason", reason).
			Msg("Dropping invalid metrics write")
		return
	}

	s.bufferMu.Lock()

	if s.stopping.Load() {
		s.bufferMu.Unlock()
		return
	}

	s.buffer = append(s.buffer, bufferedMetric{
		resourceType: normalizedType,
		resourceID:   normalizedID,
		metricType:   normalizedMetric,
		value:        value,
		timestamp:    timestamp,
		tier:         tier,
	})

	// Flush if buffer is full
	var toWrite []bufferedMetric
	if len(s.buffer) >= s.config.WriteBufferSize {
		toWrite = s.detachBufferLocked()
	}
	s.bufferMu.Unlock()

	s.enqueueWrite(writeRequest{metrics: toWrite})
}

// prepareWriteBatch validates and normalizes a caller batch, logging and
// dropping invalid entries. Shared by the synchronous and bounded batch paths.
func (s *Store) prepareWriteBatch(metrics []WriteMetric) []bufferedMetric {
	if len(metrics) == 0 {
		return nil
	}

	batch := make([]bufferedMetric, 0, len(metrics))
	droppedInvalid := 0
	for _, metric := range metrics {
		normalizedType, normalizedID, normalizedMetric, ok, reason := validateMetricWrite(
			metric.ResourceType,
			metric.ResourceID,
			metric.MetricType,
			metric.Tier,
		)
		if !ok {
			droppedInvalid++
			log.Warn().
				Str("resource_type", metric.ResourceType).
				Str("resource_id", metric.ResourceID).
				Str("metric_type", metric.MetricType).
				Str("tier", string(metric.Tier)).
				Str("reason", reason).
				Msg("Dropping invalid metrics write from batch")
			continue
		}
		batch = append(batch, bufferedMetric{
			resourceType: normalizedType,
			resourceID:   normalizedID,
			metricType:   normalizedMetric,
			value:        metric.Value,
			timestamp:    metric.Timestamp,
			tier:         metric.Tier,
		})
	}
	if droppedInvalid > 0 {
		log.Warn().
			Int("dropped", droppedInvalid).
			Msg("Dropped invalid metrics writes from batch")
	}
	return batch
}

// WriteBatchSync bypasses the in-memory sample buffer but still serializes the
// batch through the ingestion worker, waiting for the commit however long it
// takes. Callers rely on read-your-writes: mock seeding reads store coverage
// straight back, and the write-path invariant tests count committed rows. The
// live monitoring pipeline must NOT use this — it calls WriteBatchBounded so a
// slow metrics disk can never stall polling (#1437).
func (s *Store) WriteBatchSync(metrics []WriteMetric) {
	batch := s.prepareWriteBatch(metrics)
	if len(batch) == 0 {
		return
	}
	s.enqueueAndWait(writeRequest{metrics: batch})
}

// WriteBatchBounded is the monitoring-pipeline variant of WriteBatchSync: it
// hands the batch to the ingestion worker but never blocks the caller past
// syncWriteWaitTimeout. Within the budget it behaves like WriteBatchSync; past
// it the batch stays queued (or, if the queue cannot even accept it, is
// dropped with a warning) and the caller moves on.
func (s *Store) WriteBatchBounded(metrics []WriteMetric) {
	batch := s.prepareWriteBatch(metrics)
	if len(batch) == 0 {
		return
	}
	s.boundedEnqueueAndWait(writeRequest{metrics: batch})
}

func (s *Store) enqueueMaintenance(run func()) {
	if run == nil {
		return
	}

	select {
	case s.maintenanceCh <- maintenanceRequest{run: run}:
	default:
		log.Debug().Msg("Metrics maintenance queue full, skipping duplicate request")
	}
}

// WaitForMaintenance blocks until all queued maintenance work has completed.
// Tests and benchmarks use this to measure steady-state hot paths without
// asynchronous startup maintenance distorting the results.
func (s *Store) WaitForMaintenance(timeout time.Duration) error {
	if s == nil {
		return nil
	}
	if s.stopping.Load() {
		return fmt.Errorf("metrics store is stopping")
	}

	done := make(chan struct{})
	barrier := maintenanceRequest{done: done}

	if timeout <= 0 {
		s.maintenanceCh <- barrier
		<-done
		return nil
	}

	queueTimer := time.NewTimer(timeout)
	defer queueTimer.Stop()

	select {
	case s.maintenanceCh <- barrier:
	case <-queueTimer.C:
		return fmt.Errorf("timed out queueing metrics maintenance barrier after %v", timeout)
	}

	waitTimer := time.NewTimer(timeout)
	defer waitTimer.Stop()

	select {
	case <-done:
		return nil
	case <-waitTimer.C:
		return fmt.Errorf("timed out waiting for metrics maintenance after %v", timeout)
	}
}

func (s *Store) runStartupMaintenance() {
	start := time.Now()
	if s.startupHook != nil {
		s.startupHook()
	}

	if s.identityMigrationPending.Swap(false) {
		if err := s.migrateMetricsIdentityIndex(); err != nil {
			log.Error().Err(err).Msg("Deferred metrics identity index consolidation failed")
		}
	}

	// Run retention before the deferred auto-vacuum conversion so restart
	// cleanup trims stale rows and redundant-index pages before SQLite
	// potentially rewrites the file.
	s.runRetention()
	s.runAvailabilityRetention()
	s.migrateAutoVacuum()

	log.Info().Dur("duration", time.Since(start)).Msg("Deferred metrics startup maintenance completed")
}

// detachBufferLocked returns the current in-memory buffer and resets it.
// Caller must hold bufferMu.
func (s *Store) detachBufferLocked() []bufferedMetric {
	if len(s.buffer) == 0 {
		return nil
	}

	// Copy buffer for writing
	toWrite := make([]bufferedMetric, len(s.buffer))
	copy(toWrite, s.buffer)
	s.buffer = s.buffer[:0]

	return toWrite
}

// flush writes buffered metrics to the database (caller must hold bufferMu)
func (s *Store) flushLocked() {
	s.enqueueWrite(writeRequest{metrics: s.detachBufferLocked()})
}

func (s *Store) enqueueWrite(req writeRequest) {
	if len(req.metrics) == 0 && len(req.availability) == 0 && len(req.availabilityDeletes) == 0 && req.done == nil {
		return
	}

	select {
	case s.writeCh <- req:
	default:
		log.Warn().
			Str("component", "metrics_store").
			Str("action", "drop_write_batch").
			Int("batch_size", len(req.metrics)+len(req.availability)+len(req.availabilityDeletes)).
			Int("write_queue_depth", len(s.writeCh)).
			Int("write_queue_capacity", cap(s.writeCh)).
			Msg("Metrics write channel full, dropping batch")
		if req.done != nil {
			close(req.done)
		}
	}
}

// syncWriteWaitTimeout bounds how long a WriteBatchBounded call blocks on the
// ingestion worker. The monitoring pipeline writes inline (state broadcast,
// agent ingest, poll publish), so an unbounded wait lets a slow metrics disk
// starve polling entirely: on #1437's instance SQLite commits ran for seconds
// to minutes and the monitor froze after its first cycle while history writes
// queued behind retention maintenance. Within the budget the call keeps
// read-your-writes; past it the caller moves on and history lands whenever
// the worker catches up.
const syncWriteWaitTimeout = 2 * time.Second

// enqueueAndWait hands the batch to the ingestion worker and waits for its
// commit with no deadline. WriteBatchSync callers depend on read-your-writes
// regardless of disk speed. Only store shutdown releases the wait early.
func (s *Store) enqueueAndWait(req writeRequest) {
	if req.done == nil {
		req.done = make(chan struct{})
	}

	select {
	case s.writeCh <- req:
	case <-s.stopCh:
		return
	}

	select {
	case <-req.done:
	case <-s.stopCh:
	}
}

// boundedEnqueueAndWait hands the batch to the ingestion worker and waits for
// its commit, but never longer than syncWriteWaitTimeout in total. If the
// queue cannot even accept the batch within the budget the batch is dropped,
// matching enqueueWrite's saturation behavior. If only the commit is
// outstanding the batch stays queued and is not lost.
func (s *Store) boundedEnqueueAndWait(req writeRequest) {
	if req.done == nil {
		req.done = make(chan struct{})
	}

	timer := time.NewTimer(syncWriteWaitTimeout)
	defer timer.Stop()

	select {
	case s.writeCh <- req:
	case <-s.stopCh:
		return
	case <-timer.C:
		log.Warn().
			Str("component", "metrics_store").
			Str("action", "drop_sync_write_batch").
			Int("batch_size", len(req.metrics)+len(req.availability)+len(req.availabilityDeletes)).
			Int("write_queue_depth", len(s.writeCh)).
			Int("write_queue_capacity", cap(s.writeCh)).
			Msg("Metrics write queue saturated, dropping bounded batch to keep monitoring live")
		return
	}

	select {
	case <-req.done:
	case <-s.stopCh:
	case <-timer.C:
		s.warnSyncWriteBacklog(len(req.metrics) + len(req.availability) + len(req.availabilityDeletes))
	}
}

// warnSyncWriteBacklog reports a lagging ingestion worker at most once per
// 30-second window. Unlike a dropped batch this is not data loss, so the
// per-call signal is redundant while the condition persists.
func (s *Store) warnSyncWriteBacklog(batchSize int) {
	now := time.Now().UnixNano()
	last := s.syncWaitWarnNano.Load()
	if now-last < int64(30*time.Second) || !s.syncWaitWarnNano.CompareAndSwap(last, now) {
		return
	}
	log.Warn().
		Str("component", "metrics_store").
		Str("action", "sync_write_backlogged").
		Int("batch_size", batchSize).
		Int("write_queue_depth", len(s.writeCh)).
		Msg("Metrics write worker lagging, batch left queued and monitoring continues")
}

func (s *Store) drainBuffer() []bufferedMetric {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()
	return s.detachBufferLocked()
}

func (s *Store) flushBufferedAsync() {
	s.enqueueWrite(writeRequest{metrics: s.drainBuffer()})
}

func (s *Store) processWriteRequests(requests []writeRequest) {
	if len(requests) == 0 {
		return
	}

	combined := make([]bufferedMetric, 0)
	doneChans := make([]chan struct{}, 0)
	for _, req := range requests {
		if len(req.metrics) > 0 {
			combined = append(combined, req.metrics...)
		}
		if req.done != nil {
			doneChans = append(doneChans, req.done)
		}
	}

	if len(combined) > 0 {
		s.writeBatch(combined)
	}
	for _, req := range requests {
		if len(req.availability) > 0 {
			s.writeAvailabilityBatch(req.availability)
		}
		if len(req.availabilityDeletes) > 0 {
			s.deleteAvailabilityTargets(req.availabilityDeletes)
		}
	}
	for _, done := range doneChans {
		close(done)
	}
}

// writeBatch writes a batch of metrics to the database
func (s *Store) writeBatch(metrics []bufferedMetric) {
	if len(metrics) == 0 {
		return
	}

	inputCount := len(metrics)
	metrics = coalesceMetricBatch(metrics)
	if len(metrics) == 0 {
		return
	}
	if len(metrics) < inputCount {
		log.Debug().
			Int("input_count", inputCount).
			Int("count", len(metrics)).
			Msg("Coalesced duplicate metrics before write")
	}

	var tx *pdb.InstrumentedTx
	var err error

	// Retry on SQLITE_BUSY with exponential backoff
	for i := 0; i < 5; i++ {
		tx, err = s.db.Begin()
		if err == nil {
			break
		}
		if i < 4 && (err.Error() == "database is locked" || err.Error() == "sql: database is closed") {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
			continue
		}
		log.Error().Err(err).
			Str("component", "metrics_store").
			Str("action", "begin_write_tx").
			Int("batch_size", len(metrics)).
			Msg("Failed to begin metrics transaction")
		return
	}

	stmt, err := tx.Prepare(`
		INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(resource_type, resource_id, metric_type, tier, timestamp)
		DO UPDATE SET
			value = excluded.value,
			min_value = excluded.min_value,
			max_value = excluded.max_value
	`)
	if err != nil {
		_ = tx.Rollback()
		log.Error().Err(err).
			Str("component", "metrics_store").
			Str("action", "prepare_write_stmt").
			Int("batch_size", len(metrics)).
			Msg("Failed to prepare metrics insert")
		return
	}
	defer stmt.Close()

	for _, m := range metrics {
		_, err := stmt.Exec(m.resourceType, m.resourceID, m.metricType, m.value, m.timestamp.Unix(), string(m.tier))
		if err != nil {
			log.Warn().Err(err).
				Str("component", "metrics_store").
				Str("action", "insert_metric").
				Str("resource_type", m.resourceType).
				Str("resource_id", m.resourceID).
				Str("metric_type", m.metricType).
				Str("tier", string(m.tier)).
				Msg("Failed to insert metric")
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error().Err(err).
			Str("component", "metrics_store").
			Str("action", "commit_write_tx").
			Int("batch_size", len(metrics)).
			Msg("Failed to commit metrics batch")
		return
	}

	log.Debug().Int("input_count", inputCount).Int("count", len(metrics)).Msg("Wrote metrics batch")
}

func coalesceMetricBatch(metrics []bufferedMetric) []bufferedMetric {
	if len(metrics) < 2 {
		return metrics
	}

	indexes := make(map[metricBatchKey]int, len(metrics))
	coalesced := make([]bufferedMetric, 0, len(metrics))
	for _, metric := range metrics {
		key := metricBatchKey{
			resourceType:  metric.resourceType,
			resourceID:    metric.resourceID,
			metricType:    metric.metricType,
			timestampUnix: metric.timestamp.Unix(),
			tier:          metric.tier,
		}
		if index, ok := indexes[key]; ok {
			coalesced[index] = metric
			continue
		}

		indexes[key] = len(coalesced)
		coalesced = append(coalesced, metric)
	}

	return coalesced
}

// coalesceQueuedRequests drains any already-queued write requests so the worker
// can commit pending metrics in a single SQLite transaction while preserving
// Flush completion barriers.
func (s *Store) coalesceQueuedRequests(initial writeRequest) []writeRequest {
	if len(initial.metrics) == 0 && len(initial.availability) == 0 && len(initial.availabilityDeletes) == 0 && initial.done == nil {
		return nil
	}

	combined := []writeRequest{initial}
	for {
		select {
		case next, ok := <-s.writeCh:
			if !ok {
				return combined
			}
			if len(next.metrics) == 0 && len(next.availability) == 0 && len(next.availabilityDeletes) == 0 && next.done == nil {
				continue
			}
			combined = append(combined, next)
		default:
			return combined
		}
	}
}

// Query retrieves one metric using the same tier reconciliation as fleet reads.
func (s *Store) Query(resourceType, resourceID, metricType string, start, end time.Time, stepSecs int64) ([]MetricPoint, error) {
	result, err := s.queryBatch(resourceType, []string{resourceID}, []string{metricType}, start, end, stepSecs)
	if err != nil {
		return nil, err
	}
	return result[normalizeMetricIdentifier(resourceID)][normalizeMetricType(metricType)], nil
}

// QueryAll retrieves all metric types, filling observation times missing from
// the preferred tier before applying optional downsampling.
func (s *Store) QueryAll(resourceType, resourceID string, start, end time.Time, stepSecs int64) (map[string][]MetricPoint, error) {
	result, err := s.queryBatch(resourceType, []string{resourceID}, nil, start, end, stepSecs)
	if err != nil {
		return nil, err
	}
	metrics := result[normalizeMetricIdentifier(resourceID)]
	if metrics == nil {
		metrics = make(map[string][]MetricPoint)
	}
	return metrics, nil
}

// queryAllBatchChunkSize limits the number of resource IDs per SQL IN clause
// to stay well within SQLite's host-parameter ceiling and keep individual
// queries fast.
const queryAllBatchChunkSize = 500

// QueryAllBatch retrieves all metric types for multiple resources of the same
// type in a single query. Returns map[resourceID]map[metricType][]MetricPoint.
// This avoids N+1 query patterns when loading charts for many resources.
// Resource IDs are deduplicated and chunked to stay within SQLite limits.
func (s *Store) QueryAllBatch(resourceType string, resourceIDs []string, start, end time.Time, stepSecs int64) (map[string]map[string][]MetricPoint, error) {
	return s.queryBatch(resourceType, resourceIDs, nil, start, end, stepSecs)
}

// QueryMetricTypesBatch retrieves only the requested metric types for multiple
// resources of the same type in a single query. Passing no metricTypes falls
// back to QueryAllBatch semantics.
func (s *Store) QueryMetricTypesBatch(
	resourceType string,
	resourceIDs []string,
	metricTypes []string,
	start, end time.Time,
	stepSecs int64,
) (map[string]map[string][]MetricPoint, error) {
	return s.queryBatch(resourceType, resourceIDs, metricTypes, start, end, stepSecs)
}

func (s *Store) queryBatch(
	resourceType string,
	resourceIDs []string,
	metricTypes []string,
	start, end time.Time,
	stepSecs int64,
) (map[string]map[string][]MetricPoint, error) {
	resourceType = normalizeMetricResourceType(resourceType)
	// Deduplicate resource IDs.
	seen := make(map[string]struct{}, len(resourceIDs))
	unique := make([]string, 0, len(resourceIDs))
	for _, id := range resourceIDs {
		normalizedID := normalizeMetricIdentifier(id)
		if normalizedID == "" {
			continue
		}
		if _, dup := seen[normalizedID]; !dup {
			seen[normalizedID] = struct{}{}
			unique = append(unique, normalizedID)
		}
	}
	if len(unique) == 0 {
		return map[string]map[string][]MetricPoint{}, nil
	}
	normalizedMetricTypes := normalizeMetricTypes(metricTypes)

	tiers := s.tierFallbacks(end.Sub(start))
	if len(tiers) == 0 {
		return map[string]map[string][]MetricPoint{}, nil
	}
	if len(unique) <= queryAllBatchChunkSize {
		return s.queryRetainedChunk(resourceType, unique, normalizedMetricTypes, start, end, stepSecs, tiers)
	}

	result := make(map[string]map[string][]MetricPoint, len(unique))
	// Reconcile every series in one snapshot per chunk. A resource with one
	// aggregate must not hide another metric or a newer tail in a different tier.
	for lo := 0; lo < len(unique); lo += queryAllBatchChunkSize {
		hi := min(lo+queryAllBatchChunkSize, len(unique))
		chunkResult, err := s.queryRetainedChunk(resourceType, unique[lo:hi], normalizedMetricTypes, start, end, stepSecs, tiers)
		if err != nil {
			return nil, err
		}
		for id, metrics := range chunkResult {
			result[id] = metrics
		}
	}

	return result, nil
}

func normalizeMetricTypes(metricTypes []string) []string {
	if len(metricTypes) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(metricTypes))
	normalized := make([]string, 0, len(metricTypes))
	for _, metricType := range metricTypes {
		canonical := normalizeMetricType(metricType)
		if canonical == "" {
			continue
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		normalized = append(normalized, canonical)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

type retainedQueryShape struct {
	resources, metrics     int
	tiers                  [4]Tier
	aggregate, groupSeries bool
}

func retainedQueryParameters(resourceType string, resourceIDs, metricTypes []string, start, end time.Time, stepSecs int64, tiers []Tier) []interface{} {
	params := make([]interface{}, 0, len(resourceIDs)+len(metricTypes)+len(tiers)+4)
	params = append(params, resourceType)
	for _, id := range resourceIDs {
		params = append(params, id)
	}
	for _, metric := range metricTypes {
		params = append(params, metric)
	}
	params = append(params, start.Unix(), end.Unix())
	for _, tier := range tiers {
		params = append(params, string(tier))
	}
	if stepSecs > 1 {
		params = append(params, stepSecs)
	}
	// Alphabetic names avoid the SQLite driver's repeated ordinal-to-string
	// conversions while matching numbered parameters in large scopes.
	for i, value := range params {
		params[i] = sql.Named("p"+strconv.Itoa(i+1), value)
	}
	return params
}

func (s *Store) retainedQuerySQL(resourceType string, resourceIDs, metricTypes []string, start, end time.Time, stepSecs int64, tiers []Tier, groupSeries bool) (string, []interface{}) {
	shape := retainedQueryShape{resources: len(resourceIDs), metrics: len(metricTypes), aggregate: stepSecs > 1, groupSeries: groupSeries}
	cacheable := len(tiers) <= len(shape.tiers)
	copy(shape.tiers[:], tiers)
	if cacheable {
		s.queryMu.Lock()
		query := s.readQueryShapes[shape]
		s.queryMu.Unlock()
		if query != "" {
			return query, retainedQueryParameters(resourceType, resourceIDs, metricTypes, start, end, stepSecs, tiers)
		}
	}
	query, params := retainedQuerySQL(resourceType, resourceIDs, metricTypes, start, end, stepSecs, tiers, groupSeries)
	if cacheable {
		s.queryMu.Lock()
		if len(s.readQueryShapes) < maxRetainedReadStatements {
			if s.readQueryShapes == nil {
				s.readQueryShapes = make(map[retainedQueryShape]string)
			}
			s.readQueryShapes[shape] = query
		}
		s.queryMu.Unlock()
	}
	return query, params
}

// retainedQuerySQL reconciles overlapping storage buckets before any display
// aggregation. The preferred tier owns its bucket, lower tiers fill uncovered
// buckets, and a coarser fallback is omitted if a preferred point overlaps it.
// Presence never establishes continuous collection or per-series coverage.
// Every probe is restricted to the requested identities and timestamp window.
func retainedQuerySQL(resourceType string, resourceIDs, metricTypes []string, start, end time.Time, stepSecs int64, tiers []Tier, groupSeries bool) (string, []interface{}) {
	params := retainedQueryParameters(resourceType, resourceIDs, metricTypes, start, end, stepSecs, tiers)
	identityColumns := ""
	if len(resourceIDs) != 1 {
		identityColumns += "resource_id, "
	}
	if len(metricTypes) != 1 {
		identityColumns += "metric_type, "
	}
	// Reuse SQLite's named bindings across every branch and overlap probe.
	// Each identity, timestamp and display step is bound only once per read.
	slots := func(first, count int) string {
		values := make([]string, count)
		for i := range values {
			values[i] = fmt.Sprintf(":p%d", first+i)
		}
		return strings.Join(values, ",")
	}
	idSlots := slots(2, len(resourceIDs))
	metricSlots := slots(2+len(resourceIDs), len(metricTypes))
	startParam := 2 + len(resourceIDs) + len(metricTypes)
	endParam := startParam + 1
	tierParam := endParam + 1
	stepParam := tierParam + len(tiers)
	index := "idx_metrics_query_all"
	if len(metricTypes) > 0 {
		index = "idx_metrics_lookup"
	}
	scope := func(alias string, tierIndex int) string {
		clause := alias + ".resource_type = :p1 AND " + alias + ".resource_id IN (" + idSlots + ")"
		if len(metricTypes) > 0 {
			clause += " AND " + alias + ".metric_type IN (" + metricSlots + ")"
		}
		return clause + fmt.Sprintf(" AND %s.tier = :p%d AND %s.timestamp >= :p%d AND %s.timestamp <= :p%d", alias, tierParam+tierIndex, alias, startParam, alias, endParam)
	}
	projection := identityColumns + `m.timestamp, m.value,
   COALESCE(m.min_value, m.value) AS min_value, COALESCE(m.max_value, m.value) AS max_value`
	directAggregate := stepSecs > 1 && len(tiers) == 1
	bucketExpression := fmt.Sprintf("(timestamp / :p%d) * :p%d + (:p%d / 2)", stepParam, stepParam, stepParam)
	if directAggregate {
		projection = identityColumns + bucketExpression + ` AS bucket_ts,
   AVG(m.value), MIN(COALESCE(m.min_value, m.value)), MAX(COALESCE(m.max_value, m.value))`
	}
	branches := make([]string, 0, len(tiers))
	for i, tier := range tiers {
		branch := "SELECT " + projection + " FROM metrics AS m INDEXED BY " + index + " WHERE " + scope("m", i)
		for j, preferred := range tiers[:i] {
			// SQLite evaluates the uncorrelated existence check once. An empty
			// preferred tier must not incur a correlated index probe for every
			// fallback observation. Both checks use this statement's snapshot.
			branch += " AND (NOT EXISTS (SELECT 1 FROM metrics AS coverage INDEXED BY " + index + " WHERE " + scope("coverage", j) + ") OR NOT EXISTS ("
			bucket := max(tierBucketSeconds(tier), tierBucketSeconds(preferred))
			branch += fmt.Sprintf(`
    SELECT 1 FROM metrics AS h
    WHERE h.resource_type = m.resource_type AND h.resource_id = m.resource_id
    AND h.metric_type = m.metric_type AND h.tier = :p%d
    AND h.timestamp >= MAX(:p%d, (m.timestamp / %d) * %d)
    AND h.timestamp <= :p%d AND h.timestamp < (m.timestamp / %d) * %d + %d
   ))`, tierParam+j, startParam, bucket, bucket, endParam, bucket, bucket, bucket)
		}
		branches = append(branches, branch)
	}
	query := strings.Join(branches, " UNION ALL ")
	if directAggregate {
		query += " GROUP BY " + identityColumns + "bucket_ts ORDER BY " + identityColumns + "bucket_ts ASC"
	} else if stepSecs > 1 {
		query = "SELECT " + identityColumns + bucketExpression + ` AS bucket_ts,
        AVG(value), MIN(min_value), MAX(max_value)
        FROM (` + query + ") GROUP BY " + identityColumns + "bucket_ts ORDER BY " + identityColumns + "bucket_ts ASC"
	} else {
		orderColumns := identityColumns
		if !groupSeries && len(metricTypes) == 0 {
			orderColumns = ""
			if len(resourceIDs) != 1 {
				orderColumns = "resource_id, "
			}
		}
		query += " ORDER BY " + orderColumns + "timestamp ASC"
	}
	return query, params
}

func tierBucketSeconds(tier Tier) int64 {
	switch tier {
	case TierMinute:
		return 60
	case TierHourly:
		return 3600
	case TierDaily:
		return 86400
	default:
		return 1
	}
}

// The database owns statement closure. Once the bounded set is full, uncommon
// parameter-count shapes run uncached. No result, tier presence or time window
// is cached, and no eviction can close a statement another reader is binding.
const maxRetainedReadStatements = 32

func (s *Store) retainedReadStatement(query string) (*pdb.InstrumentedStmt, error) {
	s.readMu.Lock()
	defer s.readMu.Unlock()
	if stmt := s.readStatements[query]; stmt != nil {
		return stmt, nil
	}
	if len(s.readStatements) >= maxRetainedReadStatements {
		return nil, nil
	}
	stmt, err := s.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	if s.readStatements == nil {
		s.readStatements = make(map[string]*pdb.InstrumentedStmt)
	}
	s.readStatements[query] = stmt
	return stmt, nil
}

func (s *Store) queryRetainedChunk(resourceType string, resourceIDs []string, metricTypes []string, start, end time.Time, stepSecs int64, tiers []Tier) (map[string]map[string][]MetricPoint, error) {
	var tx *pdb.InstrumentedTx
	var err error
	if stepSecs > 1 {
		// Determine which tiers exist in the same read snapshot used below. An
		// all-raw series should cost a direct range read, not a UNION and an empty
		// overlap probe for every observation. Presence does not imply coverage:
		// every tier with any matching observations still participates.
		// EXISTS needs only indexed identity and time columns. Do not build or
		// order the value projection for each presence probe.
		idSlots := strings.TrimSuffix(strings.Repeat("?,", len(resourceIDs)), ",")
		presenceScope := "resource_type = ? AND resource_id IN (" + idSlots + ")"
		index := "idx_metrics_query_all"
		if len(metricTypes) > 0 {
			presenceScope += " AND metric_type IN (" + strings.TrimSuffix(strings.Repeat("?,", len(metricTypes)), ",") + ")"
			index = "idx_metrics_lookup"
		}
		presenceScope += " AND tier = ? AND timestamp >= ? AND timestamp <= ?"
		check := "EXISTS (SELECT 1 FROM metrics INDEXED BY " + index + " WHERE " + presenceScope + ")"
		checks := make([]string, len(tiers))
		checkParams := make([]interface{}, 0, len(tiers)*(len(resourceIDs)+len(metricTypes)+4))
		present := make([]bool, len(tiers))
		checkDestinations := make([]interface{}, len(tiers))
		for i, tier := range tiers {
			checks[i] = check
			checkParams = append(checkParams, resourceType)
			for _, id := range resourceIDs {
				checkParams = append(checkParams, id)
			}
			for _, metric := range metricTypes {
				checkParams = append(checkParams, metric)
			}
			checkParams = append(checkParams, string(tier), start.Unix(), end.Unix())
			checkDestinations[i] = &present[i]
		}
		presenceQuery := "SELECT " + strings.Join(checks, ", ")
		statement, err := s.retainedReadStatement(presenceQuery)
		if err != nil {
			return nil, fmt.Errorf("prepare retained metrics presence: %w", err)
		}
		// Prepare before acquiring the transaction so a one-connection pool never
		// waits for itself while preparing a database-level statement.
		tx, err = s.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return nil, fmt.Errorf("begin retained metrics snapshot: %w", err)
		}
		defer tx.Rollback()
		var presenceRow *sql.Row
		if statement != nil {
			bound := tx.Stmt(statement)
			defer bound.Close()
			presenceRow = bound.QueryRow(checkParams...)
		} else {
			presenceRow = tx.QueryRow(presenceQuery, checkParams...)
		}
		if err := presenceRow.Scan(checkDestinations...); err != nil {
			return nil, fmt.Errorf("inspect retained metrics tiers: %w", err)
		}
		available := make([]Tier, 0, len(tiers))
		for i, tier := range tiers {
			if present[i] {
				available = append(available, tier)
			}
		}
		if len(available) == 0 {
			return make(map[string]map[string][]MetricPoint), nil
		}
		tiers = available
	}
	// Fleet results are already ordered by series and time. Stream their
	// display buckets to avoid SQLite's fleet-wide temporary GROUP BY tree.
	// Single-resource charts aggregate in SQLite to bound rows crossing Go.
	streamBuckets := stepSecs > 1 && len(resourceIDs) > 1
	queryStep := stepSecs
	if streamBuckets {
		queryStep = 0
	}
	sqlQuery, params := s.retainedQuerySQL(resourceType, resourceIDs, metricTypes, start, end, queryStep, tiers, streamBuckets)

	queryRows := func() (*sql.Rows, error) { return tx.Query(sqlQuery, params...) }
	if tx == nil {
		// One SQLite statement already owns a consistent read snapshot. Plain
		// retained reads need neither a separate presence probe nor a transaction
		// wrapper. Compile the canonical reconciliation query once per shape.
		statement, prepareErr := s.retainedReadStatement(sqlQuery)
		if prepareErr != nil {
			return nil, fmt.Errorf("prepare retained metrics query: %w", prepareErr)
		}
		if statement != nil {
			queryRows = func() (*sql.Rows, error) { return statement.Query(params...) }
		} else {
			queryRows = func() (*sql.Rows, error) { return s.db.Query(sqlQuery, params...) }
		}
	}

	// Retry on SQLITE_BUSY
	var rows *sql.Rows
	for i := 0; i < 5; i++ {
		rows, err = queryRows()
		if err == nil {
			break
		}
		if i < 4 && (err.Error() == "database is locked" || err.Error() == "sql: database is closed") {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
			continue
		}
		return nil, fmt.Errorf("failed to batch query metrics: %w", err)
	}
	defer rows.Close()

	result := make(map[string]map[string][]MetricPoint, len(resourceIDs))
	seriesCapacity := estimateQueryAllBatchSeriesCapacity(start, end, stepSecs)

	// Consecutive points in one series append directly to its slice. Flush on
	// a series change so interleaved metrics can resume their existing slices
	// without performing repeated nested-map lookups for every observation.
	var outputResource, outputMetric string
	var outputMetrics map[string][]MetricPoint
	var outputPoints []MetricPoint
	flushSeries := func() {
		if outputMetrics != nil {
			outputMetrics[outputMetric] = outputPoints
		}
	}
	appendPoint := func(resourceID, metricType string, point MetricPoint) {
		if outputMetrics == nil || outputResource != resourceID || outputMetric != metricType {
			flushSeries()
			if outputMetrics == nil || outputResource != resourceID {
				outputResource = resourceID
				outputMetrics = result[resourceID]
				if outputMetrics == nil {
					outputMetrics = make(map[string][]MetricPoint, 8)
					result[resourceID] = outputMetrics
				}
			}
			outputMetric = metricType
			outputPoints = outputMetrics[metricType]
			if outputPoints == nil {
				outputPoints = make([]MetricPoint, 0, seriesCapacity)
			}
		}
		outputPoints = append(outputPoints, point)
	}
	var bucketResource, bucketMetric string
	var bucketStart int64
	var bucketPoint MetricPoint
	var bucketCount int
	flushBucket := func() {
		if bucketCount == 0 {
			return
		}
		bucketPoint.Value /= float64(bucketCount)
		bucketPoint.Timestamp = time.Unix(bucketStart+stepSecs/2, 0)
		appendPoint(bucketResource, bucketMetric, bucketPoint)
		bucketCount = 0
	}
	var resourceID, metricType string
	var ts int64
	var p MetricPoint
	destinations := make([]interface{}, 0, 6)
	if len(resourceIDs) == 1 {
		resourceID = resourceIDs[0]
	} else {
		destinations = append(destinations, &resourceID)
	}
	if len(metricTypes) == 1 {
		metricType = metricTypes[0]
	} else {
		destinations = append(destinations, &metricType)
	}
	destinations = append(destinations, &ts, &p.Value, &p.Min, &p.Max)
	for rows.Next() {
		if err := rows.Scan(destinations...); err != nil {
			log.Warn().Err(err).Msg("Failed to scan batch metric row")
			continue
		}
		if !streamBuckets {
			p.Timestamp = time.Unix(ts, 0)
			appendPoint(resourceID, metricType, p)
			continue
		}
		start := (ts / stepSecs) * stepSecs
		if bucketCount > 0 && (bucketResource != resourceID || bucketMetric != metricType || bucketStart != start) {
			flushBucket()
		}
		if bucketCount == 0 {
			bucketResource, bucketMetric, bucketStart = resourceID, metricType, start
			bucketPoint = p
		} else {
			bucketPoint.Value += p.Value
			bucketPoint.Min = min(bucketPoint.Min, p.Min)
			bucketPoint.Max = max(bucketPoint.Max, p.Max)
		}
		bucketCount++
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	flushBucket()
	flushSeries()
	return result, nil
}

func estimateQueryAllBatchSeriesCapacity(start, end time.Time, stepSecs int64) int {
	if stepSecs <= 1 {
		return 0
	}
	if !end.After(start) {
		return 1
	}

	stepDuration := time.Duration(stepSecs) * time.Second
	buckets := int(end.Sub(start)/stepDuration) + 2
	if buckets < 1 {
		return 1
	}
	return buckets
}

// selectTier chooses the appropriate data tier based on time range
// Tier selection defines preferred resolution, not a guarantee of coverage:
// - Raw: up to 2 hours (high-resolution real-time data)
// - Minute: up to 24 hours (recent detailed data)
// - Hourly: up to 7 days (medium-term history)
// - Daily: beyond 7 days (long-term historical data)
func (s *Store) selectTier(duration time.Duration) Tier {
	const (
		rawThreshold    = 2 * time.Hour
		minuteThreshold = 24 * time.Hour
		hourlyThreshold = 7 * 24 * time.Hour
	)

	switch {
	case duration <= rawThreshold:
		return TierRaw
	case duration <= minuteThreshold:
		return TierMinute
	case duration <= hourlyThreshold:
		return TierHourly
	default:
		return TierDaily
	}
}

func (s *Store) tierFallbacks(duration time.Duration) []Tier {
	switch s.selectTier(duration) {
	case TierRaw:
		// Coarser buckets fill times with no preferred raw observations.
		return []Tier{TierRaw, TierMinute, TierHourly}
	case TierMinute:
		// Raw observations fill missing minute buckets before hourly fallback.
		return []Tier{TierMinute, TierRaw, TierHourly}
	case TierHourly:
		return []Tier{TierHourly, TierMinute, TierRaw}
	case TierDaily:
		return []Tier{TierDaily, TierHourly, TierMinute, TierRaw}
	default:
		return []Tier{TierRaw}
	}
}

// backgroundWorker owns buffered metric ingestion. Maintenance work runs on a
// separate scheduler so a long startup migration, rollup, or retention pass
// cannot leave write requests unconsumed until the bounded channel overflows.
func (s *Store) backgroundWorker() {
	defer close(s.doneCh)

	flushTicker := time.NewTicker(s.config.FlushInterval)
	defer flushTicker.Stop()

	for {
		select {
		case <-s.stopCh:
			var remaining []writeRequest
			if batch := s.drainBuffer(); len(batch) > 0 {
				remaining = append(remaining, writeRequest{metrics: batch})
			}
			close(s.writeCh)
			for req := range s.writeCh {
				remaining = append(remaining, req)
			}
			s.processWriteRequests(remaining)
			return

		case req, ok := <-s.writeCh:
			if !ok {
				return
			}
			s.processWriteRequests(s.coalesceQueuedRequests(req))

		case <-flushTicker.C:
			s.flushBufferedAsync()
		}
	}
}

// maintenanceWorker owns startup maintenance and periodic rollup/retention
// scheduling. Its database writes may still wait behind live metric writes,
// but its CPU or read work cannot stop the ingestion worker from draining the
// write channel.
func (s *Store) maintenanceWorker() {
	defer close(s.maintenanceDoneCh)

	rollupTicker := time.NewTicker(s.config.RollupInterval)
	retentionTicker := time.NewTicker(1 * time.Hour)
	defer rollupTicker.Stop()
	defer retentionTicker.Stop()

	for {
		select {
		case <-s.stopCh:
			return

		case maintenance := <-s.maintenanceCh:
			if maintenance.run != nil {
				maintenance.run()
			}
			if maintenance.done != nil {
				close(maintenance.done)
			}

		case <-rollupTicker.C:
			s.runRollup()

		case <-retentionTicker.C:
			s.runRetention()
			s.runAvailabilityRetention()
		}
	}
}

// Flush writes any buffered metrics to the database and waits for all queued
// writes ahead of the flush barrier to become visible.
func (s *Store) Flush() {
	if s == nil || s.stopping.Load() {
		return
	}
	s.enqueueAndWait(writeRequest{metrics: s.drainBuffer()})
}

// runRollup aggregates raw data into higher tiers
func (s *Store) runRollup() {
	start := time.Now()

	// Rollup raw -> minute (for data older than 5 minutes)
	s.rollupTier(TierRaw, TierMinute, time.Minute, 5*time.Minute)

	// Rollup minute -> hourly (for data older than 1 hour)
	s.rollupTier(TierMinute, TierHourly, time.Hour, time.Hour)

	// Rollup hourly -> daily (for data older than 24 hours)
	s.rollupTier(TierHourly, TierDaily, 24*time.Hour, 24*time.Hour)

	log.Debug().Dur("duration", time.Since(start)).Msg("Metrics rollup completed")
}

// rollupTier aggregates data from one tier to another.
// Uses a single INSERT...SELECT...GROUP BY to batch-rollup all resource/metric
// combinations at once, replacing the previous N+1 pattern (candidate discovery
// query + per-candidate INSERT transaction).
func (s *Store) rollupTier(fromTier, toTier Tier, bucketSize, minAge time.Duration) {
	cutoff := time.Now().Add(-minAge).Unix()
	bucketSecs := int64(bucketSize.Seconds())
	if bucketSecs <= 0 {
		return
	}
	cutoffBucket := (cutoff / bucketSecs) * bucketSecs
	if cutoffBucket <= 0 {
		return
	}

	metaKey := fmt.Sprintf("rollup:%s:%s", fromTier, toTier)
	lastBucket, ok := s.getMetaInt(metaKey)
	if !ok {
		if maxTs, ok := s.getMaxTimestampForTier(toTier); ok {
			lastBucket = (maxTs / bucketSecs) * bucketSecs
			_ = s.setMetaInt(metaKey, lastBucket)
		}
	}

	if cutoffBucket <= lastBucket {
		return
	}

	chunkWindow := rollupChunkWindow(bucketSize)
	chunkSecs := int64(chunkWindow.Seconds())
	if chunkSecs <= 0 {
		chunkSecs = bucketSecs
	}

	windowStart := lastBucket
	processedChunks := 0
	processedAny := false

	for windowStart < cutoffBucket {
		nextBucket, hasSource := s.nextSourceRollupBucket(fromTier, windowStart, cutoffBucket, bucketSecs)
		if !hasSource {
			if processedAny {
				if err := s.setMetaInt(metaKey, cutoffBucket); err != nil {
					log.Warn().Err(err).Str("tier", string(fromTier)).Msg("Failed to persist rollup checkpoint")
				}
			}
			return
		}

		if nextBucket > windowStart {
			windowStart = nextBucket
		}
		windowEnd := windowStart + chunkSecs
		if windowEnd > cutoffBucket {
			windowEnd = cutoffBucket
		}
		if windowEnd <= windowStart {
			return
		}

		if !s.rollupTierWindow(fromTier, toTier, bucketSecs, windowStart, windowEnd) {
			return
		}

		windowStart = windowEnd
		processedChunks++
		processedAny = true

		if err := s.setMetaInt(metaKey, windowStart); err != nil {
			log.Warn().Err(err).Str("tier", string(fromTier)).Msg("Failed to persist rollup checkpoint")
			return
		}
		if processedChunks >= maxRollupChunksPerRun && windowStart < cutoffBucket {
			log.Debug().
				Str("from", string(fromTier)).
				Str("to", string(toTier)).
				Int("chunks", processedChunks).
				Msg("Metrics rollup paused after bounded chunk budget")
			return
		}
	}
}

func rollupChunkWindow(bucketSize time.Duration) time.Duration {
	if bucketSize <= 0 {
		return maxRollupChunkWindow
	}
	if bucketSize >= maxRollupChunkWindow {
		return bucketSize
	}
	buckets := int64(maxRollupChunkWindow / bucketSize)
	if buckets < 1 {
		buckets = 1
	}
	return time.Duration(buckets) * bucketSize
}

func (s *Store) nextSourceRollupBucket(tier Tier, startBucket, cutoffBucket, bucketSecs int64) (int64, bool) {
	var next sql.NullInt64
	if err := s.db.QueryRow(`
		SELECT MIN(timestamp)
		FROM metrics
		WHERE tier = ? AND timestamp >= ? AND timestamp < ?
	`, string(tier), startBucket, cutoffBucket).Scan(&next); err != nil {
		log.Warn().Err(err).Str("tier", string(tier)).Msg("Failed to locate next rollup source window")
		return 0, false
	}
	if !next.Valid {
		return 0, false
	}
	nextBucket := (next.Int64 / bucketSecs) * bucketSecs
	if nextBucket < startBucket {
		nextBucket = startBucket
	}
	return nextBucket, true
}

func (s *Store) rollupTierWindow(fromTier, toTier Tier, bucketSecs, startTs, endTs int64) bool {
	tx, err := s.db.Begin()
	if err != nil {
		log.Error().Err(err).Str("tier", string(fromTier)).Msg("Failed to begin rollup transaction")
		return false
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
		INSERT OR IGNORE INTO metrics (resource_type, resource_id, metric_type, value, min_value, max_value, timestamp, tier)
		SELECT
			resource_type,
			resource_id,
			metric_type,
			AVG(value) as value,
			MIN(COALESCE(min_value, value)) as min_value,
			MAX(COALESCE(max_value, value)) as max_value,
			(timestamp / ?) * ? as bucket_ts,
			?
		FROM metrics
		WHERE tier = ? AND timestamp >= ? AND timestamp < ?
		GROUP BY resource_type, resource_id, metric_type, bucket_ts
	`, bucketSecs, bucketSecs, string(toTier), string(fromTier), startTs, endTs)
	if err != nil {
		log.Warn().Err(err).
			Str("from", string(fromTier)).
			Str("to", string(toTier)).
			Msg("Failed to batch rollup metrics")
		return false
	}

	if err := tx.Commit(); err != nil {
		log.Warn().Err(err).
			Str("from", string(fromTier)).
			Str("to", string(toTier)).
			Msg("Failed to commit rollup transaction")
		return false
	}
	return true
}

// rollupCandidate aggregates a single resource/metric from one tier to another
func (s *Store) rollupCandidate(resourceType, resourceID, metricType string, fromTier, toTier Tier, bucketSecs, startTs, endTs int64) {
	if startTs >= endTs {
		return
	}
	itx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer func() { _ = itx.Rollback() }()

	// Aggregate data into buckets
	_, err = itx.Exec(`
		INSERT OR IGNORE INTO metrics (resource_type, resource_id, metric_type, value, min_value, max_value, timestamp, tier)
		SELECT 
			resource_type, 
			resource_id, 
			metric_type,
			AVG(value) as value,
			MIN(COALESCE(min_value, value)) as min_value,
			MAX(COALESCE(max_value, value)) as max_value,
			(timestamp / ?) * ? as bucket_ts,
			?
		FROM metrics
		WHERE resource_type = ? AND resource_id = ? AND metric_type = ? 
		AND tier = ? AND timestamp >= ? AND timestamp < ?
		GROUP BY resource_type, resource_id, metric_type, bucket_ts
	`, bucketSecs, bucketSecs, string(toTier), resourceType, resourceID, metricType, string(fromTier), startTs, endTs)

	if err != nil {
		log.Warn().Err(err).
			Str("resource", resourceID).
			Str("from", string(fromTier)).
			Str("to", string(toTier)).
			Msg("Failed to rollup metrics")
		return
	}

	if err := itx.Commit(); err != nil {
		log.Warn().Err(err).
			Str("resource", resourceID).
			Str("from", string(fromTier)).
			Str("to", string(toTier)).
			Msg("Failed to commit rollup transaction")
	}
}

func (s *Store) getMetaInt(key string) (int64, bool) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM metrics_meta WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, false
	}
	if err != nil {
		log.Warn().Err(err).Str("key", key).Msg("Failed to read metrics metadata")
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		log.Warn().Err(err).Str("key", key).Msg("Invalid metrics metadata value")
		return 0, false
	}
	return parsed, true
}

func (s *Store) setMetaInt(key string, value int64) error {
	_, err := s.db.Exec(`
		INSERT INTO metrics_meta (key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, strconv.FormatInt(value, 10))
	return err
}

func (s *Store) getMaxTimestampForTier(tier Tier) (int64, bool) {
	var maxTs sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(timestamp) FROM metrics WHERE tier = ?`, string(tier)).Scan(&maxTs); err != nil {
		log.Warn().Err(err).Str("tier", string(tier)).Msg("Failed to read metrics max timestamp")
		return 0, false
	}
	if !maxTs.Valid || maxTs.Int64 <= 0 {
		return 0, false
	}
	return maxTs.Int64, true
}

// MaxTimestampsForTier returns the newest stored timestamp for every series
// present in the given tier. Backfill seeders use it to write only the gap
// between existing coverage and now instead of re-writing whole windows.
func (s *Store) MaxTimestampsForTier(tier Tier) (map[SeriesKey]time.Time, error) {
	rows, err := s.db.Query(`
		SELECT resource_type, resource_id, metric_type, MAX(timestamp)
		FROM metrics
		WHERE tier = ?
		GROUP BY resource_type, resource_id, metric_type
	`, string(tier))
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics coverage for tier %q: %w", tier, err)
	}
	defer rows.Close()

	coverage := make(map[SeriesKey]time.Time)
	for rows.Next() {
		var key SeriesKey
		var ts int64
		if err := rows.Scan(&key.ResourceType, &key.ResourceID, &key.MetricType, &ts); err != nil {
			return nil, fmt.Errorf("failed to scan metrics coverage row: %w", err)
		}
		coverage[key] = time.Unix(ts, 0).UTC()
	}
	return coverage, rows.Err()
}

// runRetention deletes data older than retention period
func (s *Store) runRetention() {
	start := time.Now()
	now := time.Now()

	// Delete old data for each tier
	tiers := []struct {
		tier      Tier
		retention time.Duration
	}{
		{TierRaw, s.effectiveRetention(s.config.RetentionRaw, now)},
		{TierMinute, s.effectiveRetention(s.config.RetentionMinute, now)},
		{TierHourly, s.effectiveRetention(s.config.RetentionHourly, now)},
		{TierDaily, s.effectiveRetention(s.config.RetentionDaily, now)},
	}

	var totalDeleted int64
	for _, t := range tiers {
		cutoff := now.Add(-t.retention).Unix()
		result, err := s.db.Exec(`DELETE FROM metrics WHERE tier = ? AND timestamp < ?`, string(t.tier), cutoff)
		if err != nil {
			log.Warn().Err(err).Str("tier", string(t.tier)).Msg("Failed to prune metrics")
			continue
		}
		if affected, _ := result.RowsAffected(); affected > 0 {
			totalDeleted += affected
		}
	}

	if totalDeleted > 0 {
		log.Info().
			Int64("deleted", totalDeleted).
			Dur("duration", time.Since(start)).
			Msg("Metrics retention cleanup completed")
	}

	// Reclaim freed pages every cycle, even in an hour where nothing was
	// deleted, so a pre-existing freelist backlog still drains over time.
	s.reclaimFreePages()
}

// maxReclaimPages bounds how many freed SQLite pages reclaimFreePages returns
// to the OS per retention cycle (~50k pages * 4KiB ≈ 200MiB). A large backlog
// is drained over several hourly cycles instead of holding the write lock for
// minutes at once, while steady-state freelists (well under the cap) drain
// fully every cycle.
const maxReclaimPages = 50000

// reclaimFreePages returns freed pages to the OS. auto_vacuum is INCREMENTAL,
// so deleted-row pages sit on the freelist until reclaimed here. A previous
// fixed 5000-page batch could not keep up on busy instances: when an hourly
// retention pass frees more pages than the batch reclaims, the freelist grows
// net-positive every cycle and the database file bloats unboundedly (5GB+ of
// free pages over ~60MB of live data) even though row retention is working.
// Draining proportionally to the freelist, capped, fixes that.
func (s *Store) reclaimFreePages() {
	var freelist int64
	if err := s.db.QueryRow(`PRAGMA freelist_count`).Scan(&freelist); err != nil {
		log.Debug().Err(err).Msg("Failed to read freelist_count")
		return
	}
	if freelist == 0 {
		// Nothing to reclaim: skip the checkpoint too so steady-state WAL
		// cadence is no more aggressive than before this drained every cycle.
		return
	}
	pages := freelist
	if pages > maxReclaimPages {
		pages = maxReclaimPages
	}
	if _, err := s.db.Exec(fmt.Sprintf(`PRAGMA incremental_vacuum(%d)`, pages)); err != nil {
		log.Debug().Err(err).Msg("Incremental vacuum failed")
	}
	if _, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		log.Debug().Err(err).Msg("WAL checkpoint failed")
	}
}

// SetMaxOpenConns sets the maximum number of open connections to the database.
func (s *Store) SetMaxOpenConns(n int) {
	s.db.SetMaxOpenConns(n)
	s.db.SetMaxIdleConns(n)
}

// Close shuts down the store gracefully
func (s *Store) Close() error {
	s.stopOnce.Do(func() {
		s.stopping.Store(true)
		close(s.stopCh)
	})

	// Wait for both the ingestion and maintenance workers to finish before
	// closing the shared database pool.
	shutdownTimer := time.NewTimer(5 * time.Second)
	defer shutdownTimer.Stop()

	select {
	case <-s.doneCh:
	case <-shutdownTimer.C:
		log.Warn().Msg("Metrics store ingestion shutdown timed out")
		return s.db.Close()
	}

	select {
	case <-s.maintenanceDoneCh:
	case <-shutdownTimer.C:
		log.Warn().Msg("Metrics store maintenance shutdown timed out")
	}

	return s.db.Close()
}

func ensureOwnerOnlyDir(dir string) error {
	cleanedDir := filepath.Clean(dir)
	if filepath.IsAbs(cleanedDir) && filepath.Dir(cleanedDir) == cleanedDir {
		return fmt.Errorf("metrics database must use a dedicated subdirectory, not filesystem root %q", cleanedDir)
	}

	info, err := os.Lstat(cleanedDir)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(cleanedDir, privateDirPerm); err != nil {
			return err
		}
		info, err = os.Lstat(cleanedDir)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe metrics directory %q: symlink is not allowed", cleanedDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("unsafe metrics directory %q: not a directory", cleanedDir)
	}
	if info.Mode()&os.ModeSticky != 0 && info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("metrics directory %q is shared; choose a dedicated Pulse-owned subdirectory", cleanedDir)
	}
	// Do not try to harden shared parents such as /tmp or /dev/shm. Apart from
	// usually failing for the unprivileged service user, a root-run container
	// could otherwise make the shared directory inaccessible to the host. A
	// dedicated child remains safe and lets Pulse enforce owner-only access.
	if !directoryOwnedByCurrentUser(info) {
		return fmt.Errorf("metrics directory %q is not owned by the current user; choose a dedicated Pulse-owned subdirectory", cleanedDir)
	}
	if err := os.Chmod(cleanedDir, privateDirPerm); err != nil {
		return fmt.Errorf("secure metrics directory %q to %04o (ensure it is writable in the service sandbox): %w", cleanedDir, privateDirPerm, err)
	}
	return nil
}

func rejectSymlinkOrNonRegular(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe sqlite path %q: symlink is not allowed", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe sqlite path %q: non-regular file is not allowed", path)
	}
	return nil
}

func hardenSQLiteFile(path string) error {
	if err := rejectSymlinkOrNonRegular(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return os.Chmod(path, privateFilePerm)
}

func hardenSQLiteArtifacts(dbPath string) error {
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := hardenSQLiteFile(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
	}
	return nil
}

// Clear removes all stored metrics data.
func (s *Store) Clear() error {
	s.Flush()
	if _, err := s.db.Exec("DELETE FROM metrics"); err != nil {
		return err
	}
	_, _ = s.db.Exec("DELETE FROM metrics_meta")
	return nil
}

// Stats holds metrics store statistics
type Stats struct {
	DBSize        int64     `json:"dbSize"`
	RawCount      int64     `json:"rawCount"`
	MinuteCount   int64     `json:"minuteCount"`
	HourlyCount   int64     `json:"hourlyCount"`
	DailyCount    int64     `json:"dailyCount"`
	TotalWrites   int64     `json:"totalWrites"`
	BufferSize    int       `json:"bufferSize"`
	LastFlush     time.Time `json:"lastFlush"`
	LastRollup    time.Time `json:"lastRollup"`
	LastRetention time.Time `json:"lastRetention"`
}

// GetStats returns storage statistics
func (s *Store) GetStats() Stats {
	stats := Stats{}

	// Count by tier
	rows, err := s.db.Query(`SELECT tier, COUNT(*) FROM metrics GROUP BY tier`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var tier string
			var count int64
			if err := rows.Scan(&tier, &count); err == nil {
				switch tier {
				case "raw":
					stats.RawCount = count
				case "minute":
					stats.MinuteCount = count
				case "hourly":
					stats.HourlyCount = count
				case "daily":
					stats.DailyCount = count
				}
			}
		}
	}

	// Get database size
	if fi, err := os.Stat(s.config.DBPath); err == nil {
		stats.DBSize = fi.Size()
	}

	// Get buffer size
	s.bufferMu.Lock()
	stats.BufferSize = len(s.buffer)
	s.bufferMu.Unlock()

	return stats
}
