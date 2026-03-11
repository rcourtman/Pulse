// Package metrics provides persistent storage for time-series metrics data
// using SQLite for durability across restarts.
package metrics

import (
	"database/sql"
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

	pdb "github.com/rcourtman/pulse-go-rewrite/pkg/db"
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
	RetentionRaw    time.Duration // How long to keep raw data
	RetentionMinute time.Duration // How long to keep minute data
	RetentionHourly time.Duration // How long to keep hourly data
	RetentionDaily  time.Duration // How long to keep daily data
}

// DefaultConfig returns sensible defaults for metrics storage
func DefaultConfig(dataDir string) StoreConfig {
	return StoreConfig{
		DBPath:          filepath.Join(dataDir, "metrics.db"),
		WriteBufferSize: 100,
		FlushInterval:   5 * time.Second,
		RetentionRaw:    2 * time.Hour,
		RetentionMinute: 24 * time.Hour,
		RetentionHourly: 7 * 24 * time.Hour,
		RetentionDaily:  90 * 24 * time.Hour,
	}
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

// WriteMetric represents a metric sample to be written synchronously.
type WriteMetric struct {
	ResourceType string
	ResourceID   string
	MetricType   string
	Value        float64
	Timestamp    time.Time
	Tier         Tier
}

// Store provides persistent metrics storage
type Store struct {
	db     *pdb.InstrumentedDB
	config StoreConfig

	// Write buffer
	bufferMu sync.Mutex
	buffer   []bufferedMetric

	// Background workers
	writeCh  chan []bufferedMetric
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
	stopping atomic.Bool
}

// NewStore creates a new metrics store with the given configuration
func NewStore(config StoreConfig) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(config.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metrics directory: %w", err)
	}

	// Open database with pragmas in DSN so every pool connection is configured
	dsn := config.DBPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
			"auto_vacuum(INCREMENTAL)",
			"wal_autocheckpoint(500)",
		},
	}.Encode()
	rawDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open metrics database: %w", err)
	}

	// Configure connection pool (SQLite works best with single writer)
	rawDB.SetMaxOpenConns(1)
	rawDB.SetMaxIdleConns(1)
	rawDB.SetConnMaxLifetime(0)

	db := pdb.Wrap(rawDB, "metrics")

	store := &Store{
		db:      db,
		config:  config,
		buffer:  make([]bufferedMetric, 0, config.WriteBufferSize),
		writeCh: make(chan []bufferedMetric, 100), // Buffer for write batches
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	store.migrateLegacyHostResourceType()

	// Clean up stale data from previous runs before starting the background worker.
	// This prevents accumulation if Pulse was restarted before hourly retention ran.
	// Runs BEFORE auto-vacuum migration so the VACUUM operates on a much smaller
	// dataset (e.g. 60MB of live data instead of 5GB of stale + live).
	store.runRetention()

	// Migrate existing databases to incremental auto-vacuum. This is a one-time
	// operation that restructures the file so deleted pages can be reclaimed.
	store.migrateAutoVacuum()

	// Start background workers
	go store.backgroundWorker()

	log.Info().
		Str("path", config.DBPath).
		Int("bufferSize", config.WriteBufferSize).
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

		-- Index for efficient queries by resource and time
		CREATE INDEX IF NOT EXISTS idx_metrics_lookup 
		ON metrics(resource_type, resource_id, metric_type, tier, timestamp);

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

	if err := s.ensureMetricsUniqueIndex(); err != nil {
		return err
	}

	log.Debug().Msg("Metrics schema initialized")
	return nil
}

func (s *Store) ensureMetricsUniqueIndex() error {
	const createUniqueIndex = `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_metrics_unique
		ON metrics(resource_type, resource_id, metric_type, timestamp, tier);
	`

	_, err := s.db.Exec(createUniqueIndex)
	if err == nil {
		return nil
	}

	// If the DB already contains duplicates (from older versions), creating the unique index
	// will fail. Deduplicate and retry once.
	lower := strings.ToLower(err.Error())
	if !strings.Contains(lower, "unique") && !strings.Contains(lower, "constraint") && !strings.Contains(lower, "duplicate") {
		return fmt.Errorf("failed to create unique index for metrics rollups: %w", err)
	}

	log.Warn().Err(err).Msg("Duplicate metrics detected; deduplicating before creating unique index")

	tx, txErr := s.db.Begin()
	if txErr != nil {
		return fmt.Errorf("begin dedupe transaction: %w", txErr)
	}
	defer tx.Rollback()

	_, txErr = tx.Exec(`
		DELETE FROM metrics
		WHERE id NOT IN (
			SELECT MIN(id)
			FROM metrics
			GROUP BY resource_type, resource_id, metric_type, timestamp, tier
		)
	`)
	if txErr != nil {
		return fmt.Errorf("dedupe metrics: %w", txErr)
	}

	if txErr := tx.Commit(); txErr != nil {
		return fmt.Errorf("commit dedupe: %w", txErr)
	}

	if _, err := s.db.Exec(createUniqueIndex); err != nil {
		return fmt.Errorf("failed to create unique index after dedupe: %w", err)
	}

	log.Info().Msg("Metrics deduplicated and unique index created")
	return nil
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
	defer tx.Rollback()

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
	return strings.TrimSpace(resourceType)
}

func isLegacyMetricResourceType(resourceType string) bool {
	return strings.EqualFold(strings.TrimSpace(resourceType), "host")
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
	if isLegacyMetricResourceType(resourceType) {
		log.Warn().
			Str("resource_type", resourceType).
			Str("resource_id", resourceID).
			Msg(`Dropping legacy metrics write for unsupported resource type "host"; use "agent"`)
		return
	}

	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()

	if s.stopping.Load() {
		return
	}

	s.buffer = append(s.buffer, bufferedMetric{
		resourceType: normalizeMetricResourceType(resourceType),
		resourceID:   resourceID,
		metricType:   metricType,
		value:        value,
		timestamp:    timestamp,
		tier:         tier,
	})

	// Flush if buffer is full
	if len(s.buffer) >= s.config.WriteBufferSize {
		s.flushLocked()
	}
}

// WriteBatchSync writes metrics directly to the database without buffering.
func (s *Store) WriteBatchSync(metrics []WriteMetric) {
	if len(metrics) == 0 {
		return
	}

	batch := make([]bufferedMetric, 0, len(metrics))
	droppedLegacyHost := 0
	for _, metric := range metrics {
		if isLegacyMetricResourceType(metric.ResourceType) {
			droppedLegacyHost++
			continue
		}
		batch = append(batch, bufferedMetric{
			resourceType: normalizeMetricResourceType(metric.ResourceType),
			resourceID:   metric.ResourceID,
			metricType:   metric.MetricType,
			value:        metric.Value,
			timestamp:    metric.Timestamp,
			tier:         metric.Tier,
		})
	}
	if droppedLegacyHost > 0 {
		log.Warn().
			Int("dropped", droppedLegacyHost).
			Msg(`Dropped legacy metrics writes for unsupported resource type "host"; use "agent"`)
	}
	if len(batch) == 0 {
		return
	}

	s.writeBatch(batch)
}

// flush writes buffered metrics to the database (caller must hold bufferMu)
func (s *Store) flushLocked() {
	if len(s.buffer) == 0 {
		return
	}

	// Copy buffer for writing
	toWrite := make([]bufferedMetric, len(s.buffer))
	copy(toWrite, s.buffer)
	s.buffer = s.buffer[:0]

	// Send to serialized write channel
	select {
	case s.writeCh <- toWrite:
	default:
		log.Warn().
			Str("component", "metrics_store").
			Str("action", "drop_write_batch").
			Int("batch_size", len(toWrite)).
			Int("write_queue_depth", len(s.writeCh)).
			Int("write_queue_capacity", cap(s.writeCh)).
			Msg("Metrics write channel full, dropping batch")
	}
}

// writeBatch writes a batch of metrics to the database
func (s *Store) writeBatch(metrics []bufferedMetric) {
	if len(metrics) == 0 {
		return
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
	`)
	if err != nil {
		tx.Rollback()
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

	log.Debug().Int("count", len(metrics)).Msg("Wrote metrics batch")
}

// Query retrieves metrics for a resource within a time range, with optional downsampling
func (s *Store) Query(resourceType, resourceID, metricType string, start, end time.Time, stepSecs int64) ([]MetricPoint, error) {
	tiers := s.tierFallbacks(end.Sub(start))
	if len(tiers) == 0 {
		return []MetricPoint{}, nil
	}

	for i, tier := range tiers {
		points, err := s.queryWithTier(resourceType, resourceID, metricType, start, end, stepSecs, tier)
		if err != nil {
			return nil, err
		}
		if len(points) > 0 || i == len(tiers)-1 {
			return points, nil
		}

		log.Debug().
			Str("resourceType", resourceType).
			Str("resourceId", resourceID).
			Str("metric", metricType).
			Str("fromTier", string(tier)).
			Str("toTier", string(tiers[i+1])).
			Msg("Metrics query empty; falling back to more detailed tier")
	}

	return []MetricPoint{}, nil
}

func (s *Store) queryWithTier(resourceType, resourceID, metricType string, start, end time.Time, stepSecs int64, tier Tier) ([]MetricPoint, error) {
	var rows *sql.Rows
	var err error

	sqlQuery := `
		SELECT timestamp, value, COALESCE(min_value, value), COALESCE(max_value, value)
		FROM metrics
		WHERE resource_type = ? AND resource_id = ? AND metric_type = ? AND tier = ?
		AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`
	queryParams := []interface{}{resourceType, resourceID, metricType, string(tier), start.Unix(), end.Unix()}

	if stepSecs > 1 {
		sqlQuery = `
			SELECT 
				(timestamp / ?) * ? + (? / 2) as bucket_ts, 
				AVG(value), 
				MIN(COALESCE(min_value, value)), 
				MAX(COALESCE(max_value, value))
			FROM metrics
			WHERE resource_type = ? AND resource_id = ? AND metric_type = ? AND tier = ?
			AND timestamp >= ? AND timestamp <= ?
			GROUP BY bucket_ts
			ORDER BY bucket_ts ASC
		`
		queryParams = []interface{}{
			stepSecs, stepSecs, stepSecs,
			resourceType, resourceID, metricType, string(tier), start.Unix(), end.Unix(),
		}
	}

	// Retry on SQLITE_BUSY
	for i := 0; i < 5; i++ {
		rows, err = s.db.Query(sqlQuery, queryParams...)

		if err == nil {
			break
		}
		if i < 4 && (err.Error() == "database is locked" || err.Error() == "sql: database is closed") {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
			continue
		}
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	var points []MetricPoint
	for rows.Next() {
		var ts int64
		var p MetricPoint
		if err := rows.Scan(&ts, &p.Value, &p.Min, &p.Max); err != nil {
			log.Warn().Err(err).Msg("Failed to scan metric row")
			continue
		}
		p.Timestamp = time.Unix(ts, 0)
		points = append(points, p)
	}

	return points, rows.Err()
}

// QueryAll retrieves all metric types for a resource within a time range, with optional downsampling
func (s *Store) QueryAll(resourceType, resourceID string, start, end time.Time, stepSecs int64) (map[string][]MetricPoint, error) {
	tiers := s.tierFallbacks(end.Sub(start))
	if len(tiers) == 0 {
		return map[string][]MetricPoint{}, nil
	}

	result := make(map[string][]MetricPoint)
	for i, tier := range tiers {
		tierResult, err := s.queryAllWithTier(resourceType, resourceID, start, end, stepSecs, tier)
		if err != nil {
			return nil, err
		}
		if len(tierResult) == 0 {
			if i < len(tiers)-1 && len(result) == 0 {
				log.Debug().
					Str("resourceType", resourceType).
					Str("resourceId", resourceID).
					Str("fromTier", string(tier)).
					Str("toTier", string(tiers[i+1])).
					Msg("Metrics query empty; falling back to more detailed tier")
			}
			continue
		}

		// Merge in any metrics missing from higher tier results.
		added := 0
		for metric, points := range tierResult {
			if len(points) == 0 {
				continue
			}
			if existing, ok := result[metric]; !ok || len(existing) == 0 {
				result[metric] = points
				added++
			}
		}

		// If we already have some metrics and this tier didn't add anything new,
		// keep going in case lower tiers have newly introduced metrics.
		if added == 0 && i < len(tiers)-1 && len(result) == 0 {
			log.Debug().
				Str("resourceType", resourceType).
				Str("resourceId", resourceID).
				Str("fromTier", string(tier)).
				Str("toTier", string(tiers[i+1])).
				Msg("Metrics query empty; falling back to more detailed tier")
		}
	}

	return result, nil
}

func (s *Store) queryAllWithTier(resourceType, resourceID string, start, end time.Time, stepSecs int64, tier Tier) (map[string][]MetricPoint, error) {
	var rows *sql.Rows
	var err error

	sqlQuery := `
		SELECT metric_type, timestamp, value, COALESCE(min_value, value), COALESCE(max_value, value)
		FROM metrics
		WHERE resource_type = ? AND resource_id = ? AND tier = ?
		AND timestamp >= ? AND timestamp <= ?
		ORDER BY metric_type, timestamp ASC
	`
	queryParams := []interface{}{resourceType, resourceID, string(tier), start.Unix(), end.Unix()}

	if stepSecs > 1 {
		sqlQuery = `
			SELECT 
				metric_type,
				(timestamp / ?) * ? + (? / 2) as bucket_ts, 
				AVG(value), 
				MIN(COALESCE(min_value, value)), 
				MAX(COALESCE(max_value, value))
			FROM metrics
			WHERE resource_type = ? AND resource_id = ? AND tier = ?
			AND timestamp >= ? AND timestamp <= ?
			GROUP BY metric_type, bucket_ts
			ORDER BY metric_type, bucket_ts ASC
		`
		queryParams = []interface{}{
			stepSecs, stepSecs, stepSecs,
			resourceType, resourceID, string(tier), start.Unix(), end.Unix(),
		}
	}

	// Retry on SQLITE_BUSY
	for i := 0; i < 5; i++ {
		rows, err = s.db.Query(sqlQuery, queryParams...)

		if err == nil {
			break
		}
		if i < 4 && (err.Error() == "database is locked" || err.Error() == "sql: database is closed") {
			time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
			continue
		}
		return nil, fmt.Errorf("failed to query all metrics: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]MetricPoint)
	for rows.Next() {
		var metricType string
		var ts int64
		var p MetricPoint
		if err := rows.Scan(&metricType, &ts, &p.Value, &p.Min, &p.Max); err != nil {
			log.Warn().Err(err).Msg("Failed to scan metric row")
			continue
		}
		p.Timestamp = time.Unix(ts, 0)
		result[metricType] = append(result[metricType], p)
	}

	return result, rows.Err()
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
	// Deduplicate resource IDs.
	seen := make(map[string]struct{}, len(resourceIDs))
	unique := make([]string, 0, len(resourceIDs))
	for _, id := range resourceIDs {
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return map[string]map[string][]MetricPoint{}, nil
	}

	tiers := s.tierFallbacks(end.Sub(start))
	if len(tiers) == 0 {
		return map[string]map[string][]MetricPoint{}, nil
	}

	result := make(map[string]map[string][]MetricPoint, len(unique))
	unresolved := unique

	for _, tier := range tiers {
		if len(unresolved) == 0 {
			break
		}

		nextUnresolved := make([]string, 0, len(unresolved))

		// Process in chunks to stay within SQLite parameter limits.
		for lo := 0; lo < len(unresolved); lo += queryAllBatchChunkSize {
			hi := lo + queryAllBatchChunkSize
			if hi > len(unresolved) {
				hi = len(unresolved)
			}
			chunk := unresolved[lo:hi]
			tierResult, err := s.queryAllBatchWithTier(resourceType, chunk, start, end, stepSecs, tier)
			if err != nil {
				return nil, err
			}

			// Match QueryAll semantics per resource: once any tier returns data
			// for a resource, stop falling back for that resource.
			for _, resID := range chunk {
				metricMap, found := tierResult[resID]
				if !found || len(metricMap) == 0 {
					nextUnresolved = append(nextUnresolved, resID)
					continue
				}
				result[resID] = metricMap
			}
		}

		unresolved = nextUnresolved
	}

	return result, nil
}

func (s *Store) queryAllBatchWithTier(resourceType string, resourceIDs []string, start, end time.Time, stepSecs int64, tier Tier) (map[string]map[string][]MetricPoint, error) {
	placeholders := make([]string, len(resourceIDs))
	for i := range resourceIDs {
		placeholders[i] = "?"
	}
	inClause := strings.Join(placeholders, ",")

	var params []interface{}
	var sqlQuery string

	if stepSecs > 1 {
		params = make([]interface{}, 0, len(resourceIDs)+7)
		params = append(params, stepSecs, stepSecs, stepSecs, resourceType)
		for _, id := range resourceIDs {
			params = append(params, id)
		}
		params = append(params, string(tier), start.Unix(), end.Unix())

		sqlQuery = fmt.Sprintf(`
			SELECT
				resource_id,
				metric_type,
				(timestamp / ?) * ? + (? / 2) as bucket_ts,
				AVG(value),
				MIN(COALESCE(min_value, value)),
				MAX(COALESCE(max_value, value))
			FROM metrics
			WHERE resource_type = ? AND resource_id IN (%s) AND tier = ?
			AND timestamp >= ? AND timestamp <= ?
			GROUP BY resource_id, metric_type, bucket_ts
			ORDER BY resource_id, metric_type, bucket_ts ASC
		`, inClause)
	} else {
		params = make([]interface{}, 0, len(resourceIDs)+4)
		params = append(params, resourceType)
		for _, id := range resourceIDs {
			params = append(params, id)
		}
		params = append(params, string(tier), start.Unix(), end.Unix())

		sqlQuery = fmt.Sprintf(`
			SELECT resource_id, metric_type, timestamp, value, COALESCE(min_value, value), COALESCE(max_value, value)
			FROM metrics
			WHERE resource_type = ? AND resource_id IN (%s) AND tier = ?
			AND timestamp >= ? AND timestamp <= ?
			ORDER BY resource_id, metric_type, timestamp ASC
		`, inClause)
	}

	// Retry on SQLITE_BUSY
	var rows *sql.Rows
	var err error
	for i := 0; i < 5; i++ {
		rows, err = s.db.Query(sqlQuery, params...)
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

	result := make(map[string]map[string][]MetricPoint)
	for rows.Next() {
		var resourceID, metricType string
		var ts int64
		var p MetricPoint
		if err := rows.Scan(&resourceID, &metricType, &ts, &p.Value, &p.Min, &p.Max); err != nil {
			log.Warn().Err(err).Msg("Failed to scan batch metric row")
			continue
		}
		p.Timestamp = time.Unix(ts, 0)

		if _, exists := result[resourceID]; !exists {
			result[resourceID] = make(map[string][]MetricPoint)
		}
		result[resourceID][metricType] = append(result[resourceID][metricType], p)
	}

	return result, rows.Err()
}

// selectTier chooses the appropriate data tier based on time range
// Note: Tier selection uses fixed thresholds to ensure queries use tiers with complete data:
// - Raw: up to 2 hours (high-resolution real-time data)
// - Minute: up to 24 hours (recent detailed data)
// - Hourly: up to 7 days (medium-term with mock/seeded data coverage)
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
		// Fall back to coarser tiers when raw is empty (e.g., mock mode with seeded data)
		return []Tier{TierRaw, TierMinute, TierHourly}
	case TierMinute:
		// Fall back to coarser tiers when minute is empty (e.g., mock mode with seeded data)
		return []Tier{TierMinute, TierRaw, TierHourly}
	case TierHourly:
		return []Tier{TierHourly, TierMinute, TierRaw}
	case TierDaily:
		return []Tier{TierDaily, TierHourly, TierMinute, TierRaw}
	default:
		return []Tier{TierRaw}
	}
}

// backgroundWorker runs periodic tasks
func (s *Store) backgroundWorker() {
	defer close(s.doneCh)

	flushTicker := time.NewTicker(s.config.FlushInterval)
	rollupTicker := time.NewTicker(5 * time.Minute)
	retentionTicker := time.NewTicker(1 * time.Hour)

	defer flushTicker.Stop()
	defer rollupTicker.Stop()
	defer retentionTicker.Stop()

	for {
		select {
		case <-s.stopCh:
			// Final flush before stopping
			s.Flush()
			// Process remaining writes
			close(s.writeCh)
			for batch := range s.writeCh {
				s.writeBatch(batch)
			}
			return

		case batch := <-s.writeCh:
			s.writeBatch(batch)

		case <-flushTicker.C:
			s.Flush()

		case <-rollupTicker.C:
			s.runRollup()

		case <-retentionTicker.C:
			s.runRetention()
		}
	}
}

// Flush writes any buffered metrics to the database
func (s *Store) Flush() {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()
	s.flushLocked()
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

	// Fast check: skip rollup and preserve the checkpoint if the source tier
	// has no data in this window. This matches the old per-candidate path's
	// len(candidates)==0 early return, preventing the checkpoint from advancing
	// past windows where late or backfilled data may arrive later.
	var hasSource bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM metrics WHERE tier = ? AND timestamp >= ? AND timestamp < ?)`,
		string(fromTier), lastBucket, cutoffBucket).Scan(&hasSource); err != nil {
		log.Warn().Err(err).Str("tier", string(fromTier)).Msg("Failed to check rollup source data")
		return
	}
	if !hasSource {
		return
	}

	// Batch rollup: aggregate ALL resource/metric combinations in a single
	// SQL statement. The GROUP BY naturally partitions results by
	// (resource_type, resource_id, metric_type, bucket_ts), producing the
	// same output as the previous per-candidate loop but in one query and
	// one transaction instead of N+1.
	tx, err := s.db.Begin()
	if err != nil {
		log.Error().Err(err).Str("tier", string(fromTier)).Msg("Failed to begin rollup transaction")
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT OR IGNORE INTO metrics (resource_type, resource_id, metric_type, value, min_value, max_value, timestamp, tier)
		SELECT
			resource_type,
			resource_id,
			metric_type,
			AVG(value) as value,
			MIN(value) as min_value,
			MAX(value) as max_value,
			(timestamp / ?) * ? as bucket_ts,
			?
		FROM metrics
		WHERE tier = ? AND timestamp >= ? AND timestamp < ?
		GROUP BY resource_type, resource_id, metric_type, bucket_ts
	`, bucketSecs, bucketSecs, string(toTier), string(fromTier), lastBucket, cutoffBucket)
	if err != nil {
		log.Warn().Err(err).
			Str("from", string(fromTier)).
			Str("to", string(toTier)).
			Msg("Failed to batch rollup metrics")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Warn().Err(err).
			Str("from", string(fromTier)).
			Str("to", string(toTier)).
			Msg("Failed to commit rollup transaction")
		return
	}

	if err := s.setMetaInt(metaKey, cutoffBucket); err != nil {
		log.Warn().Err(err).Str("tier", string(fromTier)).Msg("Failed to persist rollup checkpoint")
	}
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
	defer itx.Rollback()

	// Aggregate data into buckets
	_, err = itx.Exec(`
		INSERT OR IGNORE INTO metrics (resource_type, resource_id, metric_type, value, min_value, max_value, timestamp, tier)
		SELECT 
			resource_type, 
			resource_id, 
			metric_type,
			AVG(value) as value,
			MIN(value) as min_value,
			MAX(value) as max_value,
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

	itx.Commit()
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

// runRetention deletes data older than retention period
func (s *Store) runRetention() {
	start := time.Now()
	now := time.Now()

	// Delete old data for each tier
	tiers := []struct {
		tier      Tier
		retention time.Duration
	}{
		{TierRaw, s.config.RetentionRaw},
		{TierMinute, s.config.RetentionMinute},
		{TierHourly, s.config.RetentionHourly},
		{TierDaily, s.config.RetentionDaily},
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

		// Reclaim disk space from deleted rows. Without this, the database
		// file never shrinks — a setup with 50+ resources can bloat to 5GB+
		// while only holding ~60MB of live data.
		if _, err := s.db.Exec(`PRAGMA incremental_vacuum(5000)`); err != nil {
			log.Debug().Err(err).Msg("Incremental vacuum failed")
		}
		if _, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
			log.Debug().Err(err).Msg("WAL checkpoint failed")
		}
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

	// Wait for background worker to finish
	select {
	case <-s.doneCh:
	case <-time.After(5 * time.Second):
		log.Warn().Msg("Metrics store shutdown timed out")
	}

	return s.db.Close()
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
