package auth

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestQueryPlansUseIndexes validates that all critical RBAC SQL queries use
// indexed lookups (SEARCH) rather than full table scans (SCAN TABLE).
// These queries run on every authenticated API request, so regressions
// directly impact per-request latency.
//
// NOTE: The schema and SQL here are intentionally duplicated from
// sqlite_manager.go. If the production schema or queries change, these tests
// must be updated in lockstep — a mismatch means they validate stale SQL.
func TestQueryPlansUseIndexes(t *testing.T) {
	db := newRBACPlanTestDB(t)

	tests := []struct {
		name  string
		query string
		args  []any
		// table is the table name to check for indexed access.
		table string
		// wantIndex, if non-empty, asserts this index name appears on the
		// same plan line as a SEARCH on the target table.
		wantIndex string
		// allowIndexAssistedScan permits an index-assisted scan (SCAN USING INDEX
		// or SCAN USING COVERING INDEX) but still rejects a bare full table scan.
		allowIndexAssistedScan bool
		// allowScan permits any plan — used for queries that intentionally
		// scan the whole table (e.g., GetRoles with ORDER BY name on a small table).
		allowScan bool
	}{
		// --- Per-request hot path ---
		{
			name: "load role permissions by role_id",
			query: `SELECT action, resource, effect, conditions
				FROM rbac_permissions
				WHERE role_id = ?`,
			args:      []any{"admin"},
			table:     "rbac_permissions",
			wantIndex: "idx_rbac_perm_role",
		},
		{
			name: "get user assignment by username",
			query: `SELECT role_id, updated_at
				FROM rbac_user_assignments
				WHERE username = ?`,
			args:      []any{"admin"},
			table:     "rbac_user_assignments",
			wantIndex: "idx_rbac_assign_user",
		},
		{
			name: "get role by ID (primary key)",
			query: `SELECT id, name, description, parent_id, is_built_in, priority, created_at, updated_at
				FROM rbac_roles
				WHERE id = ?`,
			args:  []any{"admin"},
			table: "rbac_roles",
			// Primary key lookup — SQLite reports as SEARCH with pk index.
		},
		{
			name:  "check role exists by ID",
			query: `SELECT COUNT(*) FROM rbac_roles WHERE id = ?`,
			args:  []any{"admin"},
			table: "rbac_roles",
		},
		{
			name:  "check is_built_in by ID",
			query: `SELECT is_built_in FROM rbac_roles WHERE id = ?`,
			args:  []any{"admin"},
			table: "rbac_roles",
		},
		// --- Changelog queries (exact SQL from GetChangeLogsForEntity) ---
		{
			name: "changelog by entity (GetChangeLogsForEntity)",
			query: `SELECT id, action, entity_type, entity_id, old_value, new_value, user, timestamp
				FROM rbac_changelog
				WHERE entity_type = ? AND entity_id = ?
				ORDER BY timestamp DESC`,
			args:  []any{"role", "admin"},
			table: "rbac_changelog",
			// With small tables the planner may choose a covering-index scan
			// on idx_rbac_changelog_time instead of idx_rbac_changelog_entity.
			// Both avoid a bare full table scan — the key invariant.
			allowIndexAssistedScan: true,
		},
		{
			name: "changelog paginated (GetChangeLogs)",
			query: `SELECT id, action, entity_type, entity_id, old_value, new_value, user, timestamp
				FROM rbac_changelog
				ORDER BY timestamp DESC
				LIMIT ? OFFSET ?`,
			args:  []any{100, 0},
			table: "rbac_changelog",
			// Planner uses idx_rbac_changelog_time for ORDER BY + LIMIT.
			allowIndexAssistedScan: true,
		},
		// --- Permission deletion (inside role save transaction) ---
		{
			name:      "delete permissions by role_id",
			query:     `DELETE FROM rbac_permissions WHERE role_id = ?`,
			args:      []any{"admin"},
			table:     "rbac_permissions",
			wantIndex: "idx_rbac_perm_role",
		},
		// --- Full table scans (acceptable for small tables) ---
		{
			name: "get all roles (full scan, small table)",
			query: `SELECT id, name, description, parent_id, is_built_in, priority, created_at, updated_at
				FROM rbac_roles
				ORDER BY name`,
			args:      []any{},
			table:     "rbac_roles",
			allowScan: true,
		},
		{
			name:      "get distinct usernames (full scan, small table)",
			query:     `SELECT DISTINCT username FROM rbac_user_assignments`,
			args:      []any{},
			table:     "rbac_user_assignments",
			allowScan: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := rbacExplainQueryPlan(t, db, tt.query, tt.args)

			if tt.allowScan {
				// No assertions — these queries intentionally scan small tables.
				t.Logf("Plan (allowed scan):\n%s", plan)
				return
			}

			// Always reject a bare full table scan on the target table.
			if rbacContainsFullTableScan(plan, tt.table) {
				t.Errorf("query uses full table scan on %s; expected indexed access\nPlan:\n%s", tt.table, plan)
			}

			if tt.allowIndexAssistedScan {
				// Must use EITHER a SEARCH or an index-assisted scan.
				if !rbacContainsTableSearch(plan, tt.table) && !rbacContainsIndexAssistedScan(plan, tt.table) {
					t.Errorf("query plan has neither SEARCH nor index-assisted scan on %s\nPlan:\n%s", tt.table, plan)
				}
			} else {
				// Must contain a SEARCH on the target table.
				if !rbacContainsTableSearch(plan, tt.table) {
					t.Errorf("query plan does not contain SEARCH on %s table; may not be using an index\nPlan:\n%s", tt.table, plan)
				}
			}

			// If a specific index is expected, verify it's used on a SEARCH or covering scan.
			if tt.wantIndex != "" {
				if !rbacContainsTableSearchWithIndex(plan, tt.table, tt.wantIndex) && !rbacContainsIndexAssistedScanWithIndex(plan, tt.table, tt.wantIndex) {
					t.Errorf("expected indexed access on %s using index %q not found in plan\nPlan:\n%s", tt.table, tt.wantIndex, plan)
				}
			}
		})
	}
}

// newRBACPlanTestDB creates an in-memory SQLite database with the RBAC schema.
func newRBACPlanTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := ":memory:?" + url.Values{
		"_pragma": []string{
			"busy_timeout(5000)",
			"journal_mode(WAL)",
			"foreign_keys(ON)",
		},
	}.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	// Schema matches sqlite_manager.go initSchema()
	schema := `
		CREATE TABLE IF NOT EXISTS rbac_roles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			parent_id TEXT,
			is_built_in INTEGER NOT NULL DEFAULT 0,
			priority INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY (parent_id) REFERENCES rbac_roles(id) ON DELETE SET NULL
		);

		CREATE TABLE IF NOT EXISTS rbac_permissions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			role_id TEXT NOT NULL,
			action TEXT NOT NULL,
			resource TEXT NOT NULL,
			effect TEXT NOT NULL DEFAULT 'allow',
			conditions TEXT,
			FOREIGN KEY (role_id) REFERENCES rbac_roles(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS rbac_user_assignments (
			username TEXT NOT NULL,
			role_id TEXT NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (username, role_id),
			FOREIGN KEY (role_id) REFERENCES rbac_roles(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS rbac_changelog (
			id TEXT PRIMARY KEY,
			action TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			old_value TEXT,
			new_value TEXT,
			user TEXT,
			timestamp INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_rbac_perm_role ON rbac_permissions(role_id);
		CREATE INDEX IF NOT EXISTS idx_rbac_assign_user ON rbac_user_assignments(username);
		CREATE INDEX IF NOT EXISTS idx_rbac_changelog_time ON rbac_changelog(timestamp);
		CREATE INDEX IF NOT EXISTS idx_rbac_changelog_entity ON rbac_changelog(entity_type, entity_id);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	// Seed data so the query planner has statistics. Errors are checked
	// to catch schema drift or constraint violations early.
	now := int64(1700000000)
	roles := []string{"admin", "operator", "viewer", "auditor"}
	for _, id := range roles {
		if _, err := db.Exec(
			`INSERT INTO rbac_roles (id, name, description, is_built_in, priority, created_at, updated_at)
			 VALUES (?, ?, ?, 1, 0, ?, ?)`,
			id, id, "Built-in "+id, now, now,
		); err != nil {
			t.Fatalf("seed role %s: %v", id, err)
		}
		if _, err := db.Exec(
			`INSERT INTO rbac_permissions (role_id, action, resource, effect)
			 VALUES (?, 'read', '*', 'allow')`,
			id,
		); err != nil {
			t.Fatalf("seed permission for %s: %v", id, err)
		}
	}
	for i := 0; i < 10; i++ {
		username := fmt.Sprintf("user%d", i)
		if _, err := db.Exec(
			`INSERT INTO rbac_user_assignments (username, role_id, updated_at) VALUES (?, 'viewer', ?)`,
			username, now,
		); err != nil {
			t.Fatalf("seed user assignment %s: %v", username, err)
		}
	}
	for i := 0; i < 20; i++ {
		if _, err := db.Exec(
			`INSERT INTO rbac_changelog (id, action, entity_type, entity_id, timestamp)
			 VALUES (?, 'created', 'role', ?, ?)`,
			fmt.Sprintf("cl-%03d", i), "admin", now+int64(i),
		); err != nil {
			t.Fatalf("seed changelog %d: %v", i, err)
		}
	}

	if _, err := db.Exec("ANALYZE"); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	return db
}

// rbacExplainQueryPlan runs EXPLAIN QUERY PLAN and returns the full plan.
func rbacExplainQueryPlan(t *testing.T, db *sql.DB, query string, args []any) string {
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

// rbacContainsTableSearch returns true if any plan line contains a SEARCH
// on the specified table.
func rbacContainsTableSearch(plan, tableName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") && rbacLineRefersToTable(line, tableName) {
			return true
		}
	}
	return false
}

// rbacContainsTableSearchWithIndex returns true if a single plan line
// contains a SEARCH on the specified table AND the given index name.
func rbacContainsTableSearchWithIndex(plan, tableName, indexName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") && rbacLineRefersToTable(line, tableName) && strings.Contains(line, indexName) {
			return true
		}
	}
	return false
}

// rbacContainsIndexAssistedScan returns true if any plan line shows an
// index-assisted scan on the specified table (SCAN ... USING INDEX or
// SCAN ... USING COVERING INDEX). Both are acceptable — they use the
// index for ordering/filtering rather than scanning the full table.
func rbacContainsIndexAssistedScan(plan, tableName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SCAN") && rbacLineRefersToTable(line, tableName) && strings.Contains(line, "USING") && strings.Contains(line, "INDEX") {
			return true
		}
	}
	return false
}

// rbacContainsIndexAssistedScanWithIndex returns true if any plan line shows
// an index-assisted scan on the specified table using the given index name.
func rbacContainsIndexAssistedScanWithIndex(plan, tableName, indexName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SCAN") && rbacLineRefersToTable(line, tableName) && strings.Contains(line, indexName) {
			return true
		}
	}
	return false
}

// rbacContainsFullTableScan returns true if the plan contains a bare full
// table scan on the specified table (SCAN without USING INDEX).
func rbacContainsFullTableScan(plan, tableName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if !strings.Contains(line, "SCAN") || !rbacLineRefersToTable(line, tableName) {
			continue
		}
		if strings.Contains(line, "USING") && strings.Contains(line, "INDEX") {
			continue
		}
		return true
	}
	return false
}

// rbacLineRefersToTable returns true if a plan line references the specified
// table name (with a word boundary — avoids matching partial names).
func rbacLineRefersToTable(line, tableName string) bool {
	for _, sep := range []string{" ", "(", ")"} {
		if strings.Contains(line, tableName+sep) {
			return true
		}
	}
	return strings.HasSuffix(strings.TrimSpace(line), tableName)
}
