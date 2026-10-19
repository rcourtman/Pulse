package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

// Store samples in the range-read B-tree itself. The old rowid table required
// a second B-tree lookup for every value after finding its identity index entry.
// Keep the time-major key shared by ingestion, reconciliation and range reads.
const clusteredMetricsIndex = "sqlite_autoindex_metrics_1"

// The tier/time tree covers resource identity, but fetching every value through
// it would seek the composite primary tree once per source point. Discover
// only the resources in this bounded window, then read their contiguous primary
// ranges. Group by a statement-local resource number to avoid comparing both
// identity strings for every input row; the number is unique within this query
// and never becomes a persisted or externally visible resource identity.
// Reuse the ordinary rollup's six binding slots in both read stages. Repeating
// the tier/window arguments would allocate another argument slice per window.
const clusteredMetricsRollupSQL = `
	WITH selected_resources AS MATERIALIZED (
		SELECT DISTINCT resource_type, resource_id FROM metrics
		WHERE tier = ?4 AND timestamp >= ?5 AND timestamp < ?6
	), resources AS MATERIALIZED (
		SELECT ROW_NUMBER() OVER (ORDER BY resource_type, resource_id) AS resource_number,
			resource_type, resource_id FROM selected_resources
	)
	INSERT OR IGNORE INTO metrics
		(resource_type, resource_id, metric_type, value, min_value, max_value, timestamp, tier)
	SELECT r.resource_type, r.resource_id, m.metric_type, AVG(m.value),
		MIN(COALESCE(m.min_value, m.value)), MAX(COALESCE(m.max_value, m.value)),
		(m.timestamp / ?1) * ?2 AS bucket_ts, ?3
	FROM resources AS r CROSS JOIN metrics AS m
	WHERE m.resource_type = r.resource_type AND m.resource_id = r.resource_id
		AND m.tier = ?4 AND m.timestamp >= ?5 AND m.timestamp < ?6
	GROUP BY r.resource_number, bucket_ts, m.metric_type
	ORDER BY r.resource_number, bucket_ts, m.metric_type
`

const metricsTableColumns = `
	resource_type TEXT NOT NULL,
	resource_id TEXT NOT NULL,
	metric_type TEXT NOT NULL,
	value REAL NOT NULL,
	min_value REAL,
	max_value REAL,
	timestamp INTEGER NOT NULL,
	tier TEXT NOT NULL DEFAULT 'raw',
	PRIMARY KEY(resource_type, resource_id, tier, timestamp, metric_type)
`

type metricsSchemaReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// Inspect the primary key rather than a schema version marker: a restored
// backup or another Store handle can change the actual physical schema.
func metricsTableIsClustered(ctx context.Context, db metricsSchemaReader) (bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT wr FROM pragma_table_list WHERE schema = 'main' AND name = 'metrics'`)
	if err != nil {
		return false, err
	}
	var withoutRowid bool
	if rows.Next() {
		err = rows.Scan(&withoutRowid)
	} else {
		err = rows.Err()
		if err == nil {
			err = fmt.Errorf("metrics table is missing")
		}
	}
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil || !withoutRowid {
		return false, err
	}
	rows, err = db.QueryContext(ctx, `PRAGMA index_info(`+clusteredMetricsIndex+`)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var sequence, cid int
		var name string
		if err := rows.Scan(&sequence, &cid, &name); err != nil {
			return false, err
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return strings.Join(columns, ",") == strings.Join(metricsIdentityColumns, ","), nil
}

func (s *Store) metricsReadIndex() string {
	if s.clusteredMetrics.Load() {
		return clusteredMetricsIndex
	}
	return metricsIdentityIndex
}

// Reserve room for the replacement pages and the transaction's WAL before
// copying pages. Keep the readable legacy layout if the filesystem cannot
// safely hold a copy; a later startup can retry without a partial migration.
func (s *Store) checkClusteredMigrationSpace(ctx context.Context, conn *sql.Conn) error {
	var pages, freePages, pageSize int64
	for _, item := range []struct {
		query string
		dest  *int64
	}{{"PRAGMA page_count", &pages}, {"PRAGMA freelist_count", &freePages}, {"PRAGMA page_size", &pageSize}} {
		if err := conn.QueryRowContext(ctx, item.query).Scan(item.dest); err != nil {
			return err
		}
	}
	usedPages := pages - freePages
	const margin = int64(16 << 20)
	if usedPages < 0 || pageSize <= 0 || usedPages > (math.MaxInt64-margin)/3/pageSize {
		return fmt.Errorf("invalid metrics page accounting")
	}
	required := uint64(usedPages*pageSize*3 + margin)
	usage, err := disk.UsageWithContext(ctx, filepath.Dir(s.config.DBPath))
	if err != nil {
		return fmt.Errorf("inspect metrics migration free space: %w", err)
	}
	if usage.Free < required {
		return fmt.Errorf("metrics clustered migration needs %d free bytes, available %d", required, usage.Free)
	}
	return nil
}

// Copy and swap in one WAL transaction. Existing readers keep their old
// snapshot throughout the copy; queued writes resume against the new table
// after commit. An interruption rolls back the table, indexes and every row.
// BEGIN IMMEDIATE also serializes independent Store handles before inspection.
func (s *Store) migrateClusteredMetrics(ctx context.Context) error {
	if s.clusteredMetrics.Load() {
		return s.cleanupClusteredMetricsIndexes(ctx)
	}
	conn, err := s.db.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("begin clustered metrics migration: %w", err)
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), `ROLLBACK`) }()
	clustered, err := metricsTableIsClustered(ctx, conn)
	if err != nil {
		return err
	}
	if !clustered {
		if err := s.checkClusteredMigrationSpace(ctx, conn); err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, `CREATE TABLE metrics_clustered_upgrade (`+metricsTableColumns+`) WITHOUT ROWID`); err != nil {
			return fmt.Errorf("create clustered metrics replacement: %w", err)
		}
		if s.clusteredMigrationHook != nil {
			s.clusteredMigrationHook("before-copy")
		}
		if _, err := conn.ExecContext(ctx, `
			INSERT INTO metrics_clustered_upgrade
			(resource_type, resource_id, metric_type, value, min_value, max_value, timestamp, tier)
			SELECT resource_type, resource_id, metric_type, value, min_value, max_value, timestamp, tier
			FROM metrics ORDER BY resource_type, resource_id, tier, timestamp, metric_type
		`); err != nil {
			return fmt.Errorf("copy clustered metrics: %w", err)
		}
		if s.clusteredMigrationHook != nil {
			s.clusteredMigrationHook("after-copy")
		}
		if _, err := conn.ExecContext(ctx, `
			DROP TABLE metrics;
			ALTER TABLE metrics_clustered_upgrade RENAME TO metrics;
			CREATE INDEX idx_metrics_tier_time ON metrics(tier, timestamp);
		`); err != nil {
			return fmt.Errorf("swap clustered metrics: %w", err)
		}
	}
	if s.clusteredMigrationHook != nil {
		s.clusteredMigrationHook("before-commit")
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("commit clustered metrics migration: %w", err)
	}
	s.clusteredMetrics.Store(true)
	return nil
}

// A previous executable can reopen this row-compatible table and recreate its
// old read index. Remove only those known redundant trees on a later startup.
// The ordinary clustered restart is a read-only probe, without a writer lock.
func (s *Store) cleanupClusteredMetricsIndexes(ctx context.Context) error {
	rows, err := s.db.DB.QueryContext(ctx, `SELECT name FROM pragma_index_list('metrics')
		WHERE name IN ('idx_metrics_query_all', 'idx_metrics_lookup', 'idx_metrics_unique')`)
	if err != nil {
		return err
	}
	var indexes []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return err
		}
		indexes = append(indexes, name)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil || len(indexes) == 0 {
		return err
	}
	conn, err := s.db.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return err
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), `ROLLBACK`) }()
	clustered, err := metricsTableIsClustered(ctx, conn)
	if err != nil || !clustered {
		return fmt.Errorf("cannot prune identity indexes without the clustered primary key: %v", err)
	}
	for _, index := range indexes {
		// Names come from the fixed whitelist above, never user input.
		if _, err := conn.ExecContext(ctx, `DROP INDEX IF EXISTS `+index); err != nil {
			return err
		}
	}
	_, err = conn.ExecContext(ctx, `COMMIT`)
	return err
}

// A different Store can commit the migration between preparing and executing
// a legacy read. Retry that missing-index error once, after verifying the new
// primary key, with a new snapshot and the same canonical read semantics.
func (s *Store) refreshClusteredMetrics(err error) bool {
	if !strings.Contains(err.Error(), "no such index: "+metricsIdentityIndex) {
		return false
	}
	clustered, inspectErr := metricsTableIsClustered(context.Background(), s.db.DB)
	if inspectErr != nil || !clustered {
		return false
	}
	s.clusteredMetrics.Store(true)
	return true
}
