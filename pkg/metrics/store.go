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
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
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
	db     *sql.DB
	config StoreConfig

	// Write buffer
	bufferMu sync.Mutex
	buffer   []bufferedMetric

	// Background workers
	writeCh  chan []bufferedMetric
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
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
		},
	}.Encode()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open metrics database: %w", err)
	}

	// Configure connection pool (SQLite works best with single writer)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

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

	log.Debug().Msg("Metrics schema initialized")
	return nil
}

// Write adds a metric to the write buffer with the 'raw' tier by default
func (s *Store) Write(resourceType, resourceID, metricType string, value float64, timestamp time.Time) {
	s.WriteWithTier(resourceType, resourceID, metricType, value, timestamp, TierRaw)
}

// WriteWithTier adds a metric to the write buffer with a specific tier
func (s *Store) WriteWithTier(resourceType, resourceID, metricType string, value float64, timestamp time.Time, tier Tier) {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()

	s.buffer = append(s.buffer, bufferedMetric{
		resourceType: resourceType,
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

	batch := make([]bufferedMetric, len(metrics))
	for i, metric := range metrics {
		batch[i] = bufferedMetric{
			resourceType: metric.ResourceType,
			resourceID:   metric.ResourceID,
			metricType:   metric.MetricType,
			value:        metric.Value,
			timestamp:    metric.Timestamp,
			tier:         metric.Tier,
		}
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
		log.Warn().Msg("Metrics write channel full, dropping batch")
	}
}

// writeBatch writes a batch of metrics to the database
func (s *Store) writeBatch(metrics []bufferedMetric) {
	if len(metrics) == 0 {
		return
	}

	var tx *sql.Tx
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
		log.Error().Err(err).Msg("Failed to begin metrics transaction")
		return
	}

	stmt, err := tx.Prepare(`
		INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		log.Error().Err(err).Msg("Failed to prepare metrics insert")
		return
	}
	defer stmt.Close()

	for _, m := range metrics {
		_, err := stmt.Exec(m.resourceType, m.resourceID, m.metricType, m.value, m.timestamp.Unix(), string(m.tier))
		if err != nil {
			log.Warn().Err(err).
				Str("resource", m.resourceID).
				Str("metric", m.metricType).
				Msg("Failed to insert metric")
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error().Err(err).Msg("Failed to commit metrics batch")
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

// rollupTier aggregates data from one tier to another
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

	// Find distinct resource/metric combinations that need rollup
	rows, err := s.db.Query(`
		SELECT DISTINCT resource_type, resource_id, metric_type
		FROM metrics
		WHERE tier = ? AND timestamp >= ? AND timestamp < ?
	`, string(fromTier), lastBucket, cutoffBucket)
	if err != nil {
		log.Error().Err(err).Str("tier", string(fromTier)).Msg("Failed to find rollup candidates")
		return
	}

	var candidates []struct {
		resourceType string
		resourceID   string
		metricType   string
	}

	for rows.Next() {
		var c struct {
			resourceType string
			resourceID   string
			metricType   string
		}
		if err := rows.Scan(&c.resourceType, &c.resourceID, &c.metricType); err == nil {
			candidates = append(candidates, c)
		}
	}
	rows.Close()

	if len(candidates) == 0 {
		return
	}

	// Process each candidate
	for _, c := range candidates {
		s.rollupCandidate(c.resourceType, c.resourceID, c.metricType, fromTier, toTier, bucketSecs, lastBucket, cutoffBucket)
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
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	// Aggregate data into buckets
	_, err = tx.Exec(`
		INSERT INTO metrics (resource_type, resource_id, metric_type, value, min_value, max_value, timestamp, tier)
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

	tx.Commit()
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
