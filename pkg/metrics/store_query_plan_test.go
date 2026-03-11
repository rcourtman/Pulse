package metrics

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestQueryPlansUseIndexes validates that all critical metrics-store SQL queries
// use indexed lookups (SEARCH) rather than full table scans (SCAN TABLE).
// This catches index regressions — if a schema migration drops an index or a
// query change breaks index eligibility, these tests fail immediately.
//
// Each sub-test runs EXPLAIN QUERY PLAN on the exact SQL the store uses at
// runtime. Queries are classified by how they should access the table:
//   - searchRequired: the plan must use SEARCH (B-tree point/range lookup)
//   - allowCoveringIndexScan: the plan may use a covering-index scan, which
//     reads only the index without touching the table — acceptable when no
//     index has the query's filter columns as a prefix
//
// In all cases, a bare "SCAN TABLE metrics" (full table scan) is rejected.
//
// NOTE: The schema and SQL here are intentionally duplicated from store.go.
// If store.go's schema or queries change, these tests must be updated in
// lockstep — a mismatch means they validate stale SQL.
func TestQueryPlansUseIndexes(t *testing.T) {
	db := newPlanTestDB(t)

	// farFuture is an int64 timestamp safe on 32-bit platforms.
	const farFuture int64 = 9999999999

	tests := []struct {
		name  string
		query string
		args  []any
		// wantIndex, if non-empty, asserts this index name appears on the
		// same plan line as a SEARCH on the metrics table.
		wantIndex string
		// allowCoveringIndexScan permits "SCAN metrics USING COVERING INDEX ..."
		// for queries where the planner correctly chooses a covering-index
		// scan over a table scan. When false, only SEARCH is accepted.
		allowCoveringIndexScan bool
		// allowIndexScan permits "SCAN metrics USING INDEX ..." (index-ordered
		// scan) for queries where the planner uses an index to satisfy GROUP BY
		// or ORDER BY without a separate sort, but still reads table rows.
		allowIndexScan bool
	}{
		{
			name: "single metric lookup",
			query: `SELECT timestamp, value, COALESCE(min_value, value), COALESCE(max_value, value)
				FROM metrics
				WHERE resource_type = ? AND resource_id = ? AND metric_type = ? AND tier = ?
				AND timestamp >= ? AND timestamp <= ?
				ORDER BY timestamp ASC`,
			args:      []any{"vm", "vm-1", "cpu", "raw", int64(0), farFuture},
			wantIndex: "idx_metrics_lookup",
		},
		{
			name: "single metric with downsampling",
			query: `SELECT
				(timestamp / ?) * ? + (? / 2) as bucket_ts,
				AVG(value),
				MIN(COALESCE(min_value, value)),
				MAX(COALESCE(max_value, value))
				FROM metrics
				WHERE resource_type = ? AND resource_id = ? AND metric_type = ? AND tier = ?
				AND timestamp >= ? AND timestamp <= ?
				GROUP BY bucket_ts
				ORDER BY bucket_ts ASC`,
			args:      []any{int64(60), int64(60), int64(60), "vm", "vm-1", "cpu", "raw", int64(0), farFuture},
			wantIndex: "idx_metrics_lookup",
		},
		{
			name: "multi-metric lookup (QueryAll)",
			query: `SELECT metric_type, timestamp, value, COALESCE(min_value, value), COALESCE(max_value, value)
				FROM metrics
				WHERE resource_type = ? AND resource_id = ? AND tier = ?
				AND timestamp >= ? AND timestamp <= ?
				ORDER BY metric_type, timestamp ASC`,
			args: []any{"vm", "vm-1", "raw", int64(0), farFuture},
			// The planner uses an indexed SEARCH — it may pick idx_metrics_query_all,
			// idx_metrics_unique, or idx_metrics_lookup depending on statistics.
			// All are valid; the key invariant is that it does a SEARCH, not a SCAN.
		},
		{
			name: "multi-metric with downsampling (QueryAll)",
			query: `SELECT
				metric_type,
				(timestamp / ?) * ? + (? / 2) as bucket_ts,
				AVG(value),
				MIN(COALESCE(min_value, value)),
				MAX(COALESCE(max_value, value))
				FROM metrics
				WHERE resource_type = ? AND resource_id = ? AND tier = ?
				AND timestamp >= ? AND timestamp <= ?
				GROUP BY metric_type, bucket_ts
				ORDER BY metric_type, bucket_ts ASC`,
			args: []any{int64(60), int64(60), int64(60), "vm", "vm-1", "raw", int64(0), farFuture},
		},
		{
			name: "multi-resource multi-metric lookup (QueryAllBatch)",
			query: `SELECT resource_id, metric_type, timestamp, value, COALESCE(min_value, value), COALESCE(max_value, value)
				FROM metrics
				WHERE resource_type = ? AND resource_id IN (?, ?, ?) AND tier = ?
				AND timestamp >= ? AND timestamp <= ?
				ORDER BY resource_id, metric_type, timestamp ASC`,
			args: []any{"vm", "vm-1", "vm-2", "vm-3", "raw", int64(0), farFuture},
			// QueryAllBatch is the anti-N+1 dashboard path. The planner may choose
			// idx_metrics_query_all, idx_metrics_unique, or idx_metrics_lookup
			// depending on statistics; the invariant is an indexed SEARCH rather
			// than a full table scan.
		},
		{
			name: "multi-resource multi-metric with downsampling (QueryAllBatch)",
			query: `SELECT
				resource_id,
				metric_type,
				(timestamp / ?) * ? + (? / 2) as bucket_ts,
				AVG(value),
				MIN(COALESCE(min_value, value)),
				MAX(COALESCE(max_value, value))
				FROM metrics
				WHERE resource_type = ? AND resource_id IN (?, ?, ?) AND tier = ?
				AND timestamp >= ? AND timestamp <= ?
				GROUP BY resource_id, metric_type, bucket_ts
				ORDER BY resource_id, metric_type, bucket_ts ASC`,
			args: []any{int64(60), int64(60), int64(60), "vm", "vm-1", "vm-2", "vm-3", "raw", int64(0), farFuture},
		},
		{
			name:      "retention delete by tier+time",
			query:     `DELETE FROM metrics WHERE tier = ? AND timestamp < ?`,
			args:      []any{"raw", farFuture},
			wantIndex: "idx_metrics_tier_time",
		},
		{
			name: "rollup candidate discovery (legacy per-candidate path)",
			query: `SELECT DISTINCT resource_type, resource_id, metric_type
				FROM metrics
				WHERE tier = ? AND timestamp >= ? AND timestamp < ?`,
			args: []any{"raw", int64(0), farFuture},
			// This query filters on (tier, timestamp) which aren't the leading
			// columns of most indexes. The planner correctly uses a covering-
			// index scan — reading only the index, never the table.
			allowCoveringIndexScan: true,
		},
		{
			name: "rollup aggregation insert (legacy per-candidate path)",
			query: `INSERT OR IGNORE INTO metrics (resource_type, resource_id, metric_type, value, min_value, max_value, timestamp, tier)
				SELECT
					resource_type, resource_id, metric_type,
					AVG(value) as value,
					MIN(value) as min_value,
					MAX(value) as max_value,
					(timestamp / ?) * ? as bucket_ts,
					?
				FROM metrics
				WHERE resource_type = ? AND resource_id = ? AND metric_type = ?
				AND tier = ? AND timestamp >= ? AND timestamp < ?
				GROUP BY resource_type, resource_id, metric_type, bucket_ts`,
			args:      []any{int64(60), int64(60), "minute", "vm", "vm-1", "cpu", "raw", int64(0), farFuture},
			wantIndex: "idx_metrics_lookup",
		},
		{
			name: "batched rollup aggregation insert",
			query: `INSERT OR IGNORE INTO metrics (resource_type, resource_id, metric_type, value, min_value, max_value, timestamp, tier)
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
				GROUP BY resource_type, resource_id, metric_type, bucket_ts`,
			args: []any{int64(60), int64(60), "minute", "raw", int64(0), farFuture},
			// The SELECT filters on (tier, timestamp) without resource/metric
			// columns. SQLite uses an index-ordered scan (SCAN USING INDEX)
			// on idx_metrics_unique. A TEMP B-TREE may still be used for
			// GROUP BY. The INSERT side uses idx_metrics_unique for ON CONFLICT.
			allowIndexScan: true,
		},
		{
			name:      "max timestamp for tier (rollup scheduling)",
			query:     `SELECT MAX(timestamp) FROM metrics WHERE tier = ?`,
			args:      []any{"raw"},
			wantIndex: "idx_metrics_tier_time",
		},
		{
			name:  "tier count stats (GetStats)",
			query: `SELECT tier, COUNT(*) FROM metrics GROUP BY tier`,
			args:  []any{},
			// GROUP BY tier scans all rows — a covering-index scan on
			// idx_metrics_tier_time reads only the index (tier, timestamp),
			// avoiding the full table row fetch. A SEARCH is also acceptable
			// if the planner chooses a different strategy.
			allowCoveringIndexScan: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := explainQueryPlan(t, db, tt.query, tt.args)

			// Reject a bare full table scan — this is always bad.
			if containsFullTableScan(plan) {
				t.Errorf("query uses full table scan on metrics; expected indexed access\nPlan:\n%s", plan)
			}

			if tt.allowIndexScan {
				// Must use SEARCH, covering-index scan, OR index-ordered scan.
				if !containsMetricsSearch(plan) && !containsCoveringIndexScan(plan) && !containsIndexScan(plan) {
					t.Errorf("query plan has neither SEARCH on metrics nor index scan\nPlan:\n%s", plan)
				}
			} else if tt.allowCoveringIndexScan {
				// Must use EITHER a SEARCH on metrics or a covering-index scan on metrics.
				if !containsMetricsSearch(plan) && !containsCoveringIndexScan(plan) {
					t.Errorf("query plan has neither SEARCH on metrics nor covering-index scan\nPlan:\n%s", plan)
				}
			} else {
				// Must contain at least one SEARCH on the metrics table.
				// We scope to "metrics" to avoid false passes from constraint-
				// handling SEARCH nodes in INSERT ... ON CONFLICT plans.
				if !containsMetricsSearch(plan) {
					t.Errorf("query plan does not contain SEARCH on metrics table; may not be using an index\nPlan:\n%s", plan)
				}
			}

			// If a specific index is expected, verify that the same plan line
			// that does a SEARCH on metrics also references that index.
			// This prevents false passes where one node SEARCHes via
			// conflict handling and a different node uses the expected index.
			if tt.wantIndex != "" && !containsMetricsSearchWithIndex(plan, tt.wantIndex) {
				t.Errorf("expected SEARCH on metrics using index %q not found in plan\nPlan:\n%s", tt.wantIndex, plan)
			}
		})
	}
}

// lineRefersToMetrics returns true if a plan line references the "metrics"
// table (not metrics_meta). SQLite may emit "SEARCH metrics USING ...",
// "SEARCH TABLE metrics ...", "SCAN metrics ...", or "SCAN TABLE metrics ...".
func lineRefersToMetrics(line string) bool {
	// Match "metrics " (with space after), "metrics(" (function-like),
	// or "metrics" at end of line. Reject "metrics_meta".
	for _, needle := range []string{"metrics ", "metrics("} {
		if strings.Contains(line, needle) && !strings.Contains(line, "metrics_meta") {
			return true
		}
	}
	// Handle "metrics" at end of line (e.g., bare "SCAN metrics").
	if strings.HasSuffix(strings.TrimSpace(line), "metrics") && !strings.Contains(line, "metrics_meta") {
		return true
	}
	return false
}

// containsMetricsSearch returns true if any plan line contains a SEARCH
// specifically on the metrics table (e.g., "SEARCH metrics USING INDEX ..."
// or "SEARCH TABLE metrics USING INDEX ...").
func containsMetricsSearch(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") && lineRefersToMetrics(line) {
			return true
		}
	}
	return false
}

// containsMetricsSearchWithIndex returns true if a single plan line contains
// a SEARCH on the metrics table AND the given index name. This ensures the
// expected index is used on the metrics-table read path, not on a separate node.
func containsMetricsSearchWithIndex(plan, indexName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") && lineRefersToMetrics(line) && strings.Contains(line, indexName) {
			return true
		}
	}
	return false
}

// containsFullTableScan returns true if the plan contains a full table scan
// on the metrics table — "SCAN TABLE metrics" or "SCAN metrics" without a
// "USING ... INDEX" qualifier. A covering-index scan is not a full table scan.
func containsFullTableScan(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if !strings.Contains(line, "SCAN") || !lineRefersToMetrics(line) {
			continue
		}
		// A covering-index scan is acceptable — it reads only the index.
		if strings.Contains(line, "USING") && strings.Contains(line, "INDEX") {
			continue
		}
		// Bare SCAN TABLE metrics or SCAN metrics without index = full scan.
		return true
	}
	return false
}

// containsCoveringIndexScan returns true if any plan line shows a covering-
// index scan on the metrics table (e.g., "SCAN metrics USING COVERING INDEX ...").
func containsCoveringIndexScan(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SCAN") && lineRefersToMetrics(line) && strings.Contains(line, "COVERING INDEX") {
			return true
		}
	}
	return false
}

// containsIndexScan returns true if any plan line shows an index-ordered scan
// on the metrics table (e.g., "SCAN metrics USING INDEX idx_metrics_unique").
// This differs from a covering-index scan in that it reads table rows via the
// index, but still avoids a full table scan.
func containsIndexScan(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SCAN") && lineRefersToMetrics(line) && strings.Contains(line, "USING") && strings.Contains(line, "INDEX") {
			return true
		}
	}
	return false
}

// newPlanTestDB creates an in-memory SQLite database with the metrics store
// schema. It uses the same schema and indexes as store.initSchema().
func newPlanTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := ":memory:?" + url.Values{
		"_pragma": []string{
			"busy_timeout(5000)",
			"journal_mode(WAL)",
		},
	}.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	// Pin to a single connection — a second connection to :memory: would get
	// a different database, causing "no such table" failures.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	schema := `
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

		CREATE INDEX IF NOT EXISTS idx_metrics_lookup
		ON metrics(resource_type, resource_id, metric_type, tier, timestamp);

		CREATE INDEX IF NOT EXISTS idx_metrics_tier_time
		ON metrics(tier, timestamp);

		CREATE INDEX IF NOT EXISTS idx_metrics_query_all
		ON metrics(resource_type, resource_id, tier, timestamp, metric_type);

		CREATE UNIQUE INDEX IF NOT EXISTS idx_metrics_unique
		ON metrics(resource_type, resource_id, metric_type, timestamp, tier);

		CREATE TABLE IF NOT EXISTS metrics_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	// Insert a small amount of seed data so the query planner has statistics
	// to reason about. Without data, SQLite may choose different plans.
	for i := 0; i < 100; i++ {
		_, err := db.Exec(
			`INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier) VALUES (?, ?, ?, ?, ?, ?)`,
			"vm", fmt.Sprintf("vm-%d", i%10), "cpu", float64(i), int64(1000000+i*5), "raw",
		)
		if err != nil {
			t.Fatalf("seed data: %v", err)
		}
	}

	// Run ANALYZE so the planner has real statistics.
	if _, err := db.Exec("ANALYZE"); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	return db
}

// explainQueryPlan runs EXPLAIN QUERY PLAN on the given query and returns
// the full plan output as a single string.
func explainQueryPlan(t *testing.T, db *sql.DB, query string, args []any) string {
	t.Helper()

	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v\nQuery: %s", err, query)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan explain row: %v", err)
		}
		lines = append(lines, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("explain rows: %v", err)
	}

	return strings.Join(lines, "\n")
}
