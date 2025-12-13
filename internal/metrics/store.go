// Package metrics provides persistent storage for time-series metrics data
// using SQLite for durability across restarts.
package metrics

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
}

// Store provides persistent metrics storage
type Store struct {
	db     *sql.DB
	config StoreConfig

	// Write buffer
	bufferMu sync.Mutex
	buffer   []bufferedMetric

	// Background workers
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

	// Open database with WAL mode for better concurrent access
	db, err := sql.Open("sqlite", config.DBPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open metrics database: %w", err)
	}

	// Configure connection pool (SQLite works best with single writer)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &Store{
		db:     db,
		config: config,
		buffer: make([]bufferedMetric, 0, config.WriteBufferSize),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
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

// Write adds a metric to the write buffer
func (s *Store) Write(resourceType, resourceID, metricType string, value float64, timestamp time.Time) {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()

	s.buffer = append(s.buffer, bufferedMetric{
		resourceType: resourceType,
		resourceID:   resourceID,
		metricType:   metricType,
		value:        value,
		timestamp:    timestamp,
	})

	// Flush if buffer is full
	if len(s.buffer) >= s.config.WriteBufferSize {
		s.flushLocked()
	}
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

	// Write in background to not block callers
	go s.writeBatch(toWrite)
}

// writeBatch writes a batch of metrics to the database
func (s *Store) writeBatch(metrics []bufferedMetric) {
	if len(metrics) == 0 {
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		log.Error().Err(err).Msg("Failed to begin metrics transaction")
		return
	}

	stmt, err := tx.Prepare(`
		INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier)
		VALUES (?, ?, ?, ?, ?, 'raw')
	`)
	if err != nil {
		tx.Rollback()
		log.Error().Err(err).Msg("Failed to prepare metrics insert")
		return
	}
	defer stmt.Close()

	for _, m := range metrics {
		_, err := stmt.Exec(m.resourceType, m.resourceID, m.metricType, m.value, m.timestamp.Unix())
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

// Query retrieves metrics for a resource within a time range
func (s *Store) Query(resourceType, resourceID, metricType string, start, end time.Time) ([]MetricPoint, error) {
	// Select appropriate tier based on time range
	tier := s.selectTier(end.Sub(start))

	rows, err := s.db.Query(`
		SELECT timestamp, value, COALESCE(min_value, value), COALESCE(max_value, value)
		FROM metrics
		WHERE resource_type = ? AND resource_id = ? AND metric_type = ? AND tier = ?
		AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp ASC
	`, resourceType, resourceID, metricType, string(tier), start.Unix(), end.Unix())
	if err != nil {
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

// QueryAll retrieves all metric types for a resource within a time range
func (s *Store) QueryAll(resourceType, resourceID string, start, end time.Time) (map[string][]MetricPoint, error) {
	tier := s.selectTier(end.Sub(start))

	rows, err := s.db.Query(`
		SELECT metric_type, timestamp, value, COALESCE(min_value, value), COALESCE(max_value, value)
		FROM metrics
		WHERE resource_type = ? AND resource_id = ? AND tier = ?
		AND timestamp >= ? AND timestamp <= ?
		ORDER BY metric_type, timestamp ASC
	`, resourceType, resourceID, string(tier), start.Unix(), end.Unix())
	if err != nil {
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
func (s *Store) selectTier(duration time.Duration) Tier {
	switch {
	case duration <= s.config.RetentionRaw:
		return TierRaw
	case duration <= s.config.RetentionMinute:
		return TierMinute
	case duration <= s.config.RetentionHourly:
		return TierHourly
	default:
		return TierDaily
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
			return

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

	// Find distinct resource/metric combinations that need rollup
	rows, err := s.db.Query(`
		SELECT DISTINCT resource_type, resource_id, metric_type
		FROM metrics
		WHERE tier = ? AND timestamp < ?
	`, string(fromTier), cutoff)
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
		s.rollupCandidate(c.resourceType, c.resourceID, c.metricType, fromTier, toTier, bucketSecs, cutoff)
	}
}

// rollupCandidate aggregates a single resource/metric from one tier to another
func (s *Store) rollupCandidate(resourceType, resourceID, metricType string, fromTier, toTier Tier, bucketSecs, cutoff int64) {
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
		AND tier = ? AND timestamp < ?
		GROUP BY resource_type, resource_id, metric_type, bucket_ts
	`, bucketSecs, bucketSecs, string(toTier), resourceType, resourceID, metricType, string(fromTier), cutoff)

	if err != nil {
		log.Warn().Err(err).
			Str("resource", resourceID).
			Str("from", string(fromTier)).
			Str("to", string(toTier)).
			Msg("Failed to rollup metrics")
		return
	}

	// Delete rolled-up raw data
	_, err = tx.Exec(`
		DELETE FROM metrics
		WHERE resource_type = ? AND resource_id = ? AND metric_type = ?
		AND tier = ? AND timestamp < ?
	`, resourceType, resourceID, metricType, string(fromTier), cutoff)

	if err != nil {
		log.Warn().Err(err).Msg("Failed to delete rolled-up metrics")
		return
	}

	tx.Commit()
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

// Stats holds metrics store statistics
type Stats struct {
	DBPath        string    `json:"dbPath"`
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
	stats := Stats{
		DBPath: s.config.DBPath,
	}

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
