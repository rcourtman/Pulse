package api

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestJTIReplayStoreQueryPlanUsesExpiryIndex(t *testing.T) {
	t.Parallel()

	store, now := newJTIReplayPlanTestStore(t)

	plan := explainHandoffQueryPlan(t, store.db, `DELETE FROM handoff_jti WHERE expires_at <= ?`, now.Unix())

	if containsHandoffFullTableScan(plan) {
		t.Fatalf("cleanup query uses full table scan on handoff_jti\nPlan:\n%s", plan)
	}
	if !containsHandoffSearchWithIndex(plan, "idx_handoff_jti_expires_at") {
		t.Fatalf("expected SEARCH on handoff_jti using idx_handoff_jti_expires_at\nPlan:\n%s", plan)
	}
}

func newJTIReplayPlanTestStore(t *testing.T) (*jtiReplayStore, time.Time) {
	t.Helper()

	configDir := t.TempDir()
	store := &jtiReplayStore{configDir: configDir}
	now := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 512; i++ {
		expiresAt := now.Add(time.Duration(i+1) * time.Hour)
		if i < 24 {
			expiresAt = now.Add(-time.Duration(i+1) * time.Minute)
		}
		stored, err := store.checkAndStore("jti-"+twoDigitHandoff(i), expiresAt)
		if err != nil {
			t.Fatalf("checkAndStore(%d): %v", i, err)
		}
		if !stored {
			t.Fatalf("checkAndStore(%d) returned duplicate unexpectedly", i)
		}
	}

	t.Cleanup(func() {
		if store.db != nil {
			_ = store.db.Close()
		}
	})

	if _, err := store.db.Exec(`ANALYZE`); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}
	return store, now
}

func explainHandoffQueryPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()

	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v\nQuery: %s", err, query)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var id, parent, aux int
		var detail string
		if err := rows.Scan(&id, &parent, &aux, &detail); err != nil {
			t.Fatalf("scan plan row: %v", err)
		}
		lines = append(lines, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	return strings.Join(lines, "\n")
}

func containsHandoffSearchWithIndex(plan, indexName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") &&
			strings.Contains(line, "handoff_jti") &&
			strings.Contains(line, indexName) {
			return true
		}
	}
	return false
}

func containsHandoffFullTableScan(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if !strings.Contains(line, "SCAN") || !strings.Contains(line, "handoff_jti") {
			continue
		}
		if strings.Contains(line, "USING") && strings.Contains(line, "INDEX") {
			continue
		}
		return true
	}
	return false
}

func twoDigitHandoff(v int) string {
	return fmt.Sprintf("%03d", v)
}
