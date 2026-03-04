package audit

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestQueryPlansUseIndexes validates that critical audit log SQL queries use
// indexed lookups rather than full table scans. The audit_events table grows
// continuously, so unindexed queries would degrade over time.
//
// NOTE: Schema and SQL are intentionally duplicated from sqlite_logger.go.
// If the production schema or queries change, these tests must be updated
// in lockstep.
func TestQueryPlansUseIndexes(t *testing.T) {
	db := newAuditPlanTestDB(t)

	tests := []struct {
		name  string
		query string
		args  []any
		// wantIndex, if non-empty, asserts this index name appears on the
		// same plan line as a SEARCH on audit_events.
		wantIndex string
		// allowCoveringIndexScan permits a covering-index scan
		// but still rejects a bare full table scan.
		allowCoveringIndexScan bool
		// allowScan permits any plan including bare full table scans.
		allowScan bool
	}{
		// --- Query paths (exact SQL shapes from sqlite_logger.go Query()) ---
		{
			name: "query by timestamp range (most common filter)",
			query: `SELECT id, timestamp, event_type, user, ip, path, success, details, signature
				FROM audit_events WHERE 1=1
				AND timestamp >= ? AND timestamp <= ?
				ORDER BY timestamp DESC
				LIMIT ?`,
			args:      []any{int64(0), int64(9999999999), 100},
			wantIndex: "idx_audit_timestamp",
		},
		{
			name: "query by event_type + timestamp",
			query: `SELECT id, timestamp, event_type, user, ip, path, success, details, signature
				FROM audit_events WHERE 1=1
				AND timestamp >= ? AND timestamp <= ?
				AND event_type = ?
				ORDER BY timestamp DESC
				LIMIT ?`,
			args: []any{int64(0), int64(9999999999), "auth_login", 100},
			// Planner may use idx_audit_event_type or idx_audit_timestamp —
			// either avoids a full table scan.
		},
		{
			name: "query by user + timestamp",
			query: `SELECT id, timestamp, event_type, user, ip, path, success, details, signature
				FROM audit_events WHERE 1=1
				AND timestamp >= ? AND timestamp <= ?
				AND user = ?
				ORDER BY timestamp DESC
				LIMIT ?`,
			args: []any{int64(0), int64(9999999999), "admin", 100},
			// Planner may use idx_audit_user or idx_audit_timestamp.
		},
		// --- Count paths (exact SQL shapes from sqlite_logger.go Count()) ---
		{
			name: "count by timestamp range",
			query: `SELECT COUNT(*) FROM audit_events WHERE 1=1
				AND timestamp >= ? AND timestamp <= ?`,
			args:      []any{int64(0), int64(9999999999)},
			wantIndex: "idx_audit_timestamp",
		},
		{
			name: "count by event_type + timestamp",
			query: `SELECT COUNT(*) FROM audit_events WHERE 1=1
				AND timestamp >= ? AND timestamp <= ?
				AND event_type = ?`,
			args: []any{int64(0), int64(9999999999), "auth_login"},
		},
		{
			name: "count by success + timestamp",
			query: `SELECT COUNT(*) FROM audit_events WHERE 1=1
				AND timestamp >= ? AND timestamp <= ?
				AND success = ?`,
			args: []any{int64(0), int64(9999999999), int64(1)},
			// success is a binary column (0/1) with very low selectivity.
			// The planner correctly prefers a full scan over index lookup
			// when most rows match. This is acceptable for a low-frequency
			// admin query.
			allowScan: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := auditExplainQueryPlan(t, db, tt.query, tt.args)

			if tt.allowScan {
				t.Logf("Plan (allowed scan):\n%s", plan)
				return
			}

			// Reject bare full table scan on audit_events.
			if auditContainsFullTableScan(plan) {
				t.Errorf("query uses full table scan on audit_events; expected indexed access\nPlan:\n%s", plan)
			}

			if tt.allowCoveringIndexScan {
				if !auditContainsSearch(plan) && !auditContainsCoveringIndexScan(plan) {
					t.Errorf("query plan has neither SEARCH nor covering-index scan on audit_events\nPlan:\n%s", plan)
				}
			} else {
				if !auditContainsSearch(plan) {
					t.Errorf("query plan does not contain SEARCH on audit_events; may not be using an index\nPlan:\n%s", plan)
				}
			}

			if tt.wantIndex != "" && !auditContainsSearchWithIndex(plan, tt.wantIndex) {
				t.Errorf("expected SEARCH on audit_events using index %q not found in plan\nPlan:\n%s", tt.wantIndex, plan)
			}
		})
	}
}

// newAuditPlanTestDB creates an in-memory SQLite database with the audit schema.
func newAuditPlanTestDB(t *testing.T) *sql.DB {
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
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// Schema matches sqlite_logger.go initSchema()
	schema := `
		CREATE TABLE IF NOT EXISTS audit_events (
			id TEXT PRIMARY KEY,
			timestamp INTEGER NOT NULL,
			event_type TEXT NOT NULL,
			user TEXT,
			ip TEXT,
			path TEXT,
			success INTEGER NOT NULL,
			details TEXT,
			signature TEXT NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_events(timestamp);
		CREATE INDEX IF NOT EXISTS idx_audit_event_type ON audit_events(event_type);
		CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_events(user) WHERE user != '';
		CREATE INDEX IF NOT EXISTS idx_audit_success ON audit_events(success);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	// Seed data so the query planner has real statistics. Errors are checked
	// to catch schema drift or constraint violations early.
	eventTypes := []string{"auth_login", "auth_logout", "settings_change", "node_added", "alert_fired"}
	users := []string{"admin", "operator", "viewer", "", ""}
	for i := 0; i < 200; i++ {
		if _, err := db.Exec(
			`INSERT INTO audit_events (id, timestamp, event_type, user, ip, path, success, details, signature)
			 VALUES (?, ?, ?, ?, '127.0.0.1', '/api/test', 1, '{}', 'sig')`,
			fmt.Sprintf("evt-%03d", i),
			int64(1700000000+i*5),
			eventTypes[i%len(eventTypes)],
			users[i%len(users)],
		); err != nil {
			t.Fatalf("seed event %d: %v", i, err)
		}
	}

	if _, err := db.Exec("ANALYZE"); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	return db
}

// auditExplainQueryPlan runs EXPLAIN QUERY PLAN and returns the full plan.
func auditExplainQueryPlan(t *testing.T, db *sql.DB, query string, args []any) string {
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

// auditLineRefersToEvents returns true if a plan line references audit_events
// (but not audit_config or other tables).
func auditLineRefersToEvents(line string) bool {
	for _, sep := range []string{" ", "(", ")"} {
		if strings.Contains(line, "audit_events"+sep) {
			return true
		}
	}
	return strings.HasSuffix(strings.TrimSpace(line), "audit_events")
}

// auditContainsSearch returns true if any plan line has SEARCH on audit_events.
func auditContainsSearch(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") && auditLineRefersToEvents(line) {
			return true
		}
	}
	return false
}

// auditContainsSearchWithIndex returns true if a plan line has SEARCH on
// audit_events with the given index name.
func auditContainsSearchWithIndex(plan, indexName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") && auditLineRefersToEvents(line) && strings.Contains(line, indexName) {
			return true
		}
	}
	return false
}

// auditContainsCoveringIndexScan returns true if any plan line shows a
// covering-index scan on audit_events.
func auditContainsCoveringIndexScan(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SCAN") && auditLineRefersToEvents(line) && strings.Contains(line, "INDEX") {
			return true
		}
	}
	return false
}

// auditContainsFullTableScan returns true if the plan has a bare full table
// scan on audit_events (SCAN without USING INDEX).
func auditContainsFullTableScan(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if !strings.Contains(line, "SCAN") || !auditLineRefersToEvents(line) {
			continue
		}
		if strings.Contains(line, "USING") && strings.Contains(line, "INDEX") {
			continue
		}
		return true
	}
	return false
}
