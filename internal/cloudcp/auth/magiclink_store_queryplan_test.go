package auth

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestMagicLinkStoreQueryPlansUseIndexes(t *testing.T) {
	t.Parallel()

	store, tokenHash, now := newMagicLinkPlanTestStore(t)

	tests := []struct {
		name      string
		query     string
		args      []any
		wantIndex string
	}{
		{
			name:      "consume lookup uses token primary key",
			query:     `SELECT email, tenant_id, expires_at, used FROM magic_link_tokens WHERE token_hash = ?`,
			args:      []any{hex.EncodeToString(tokenHash)},
			wantIndex: "sqlite_autoindex_magic_link_tokens_1",
		},
		{
			name:      "consume update uses token primary key",
			query:     `UPDATE magic_link_tokens SET used = 1, used_at = ? WHERE token_hash = ? AND used = 0`,
			args:      []any{now.Unix(), hex.EncodeToString(tokenHash)},
			wantIndex: "sqlite_autoindex_magic_link_tokens_1",
		},
		{
			name:      "delete expired uses expires index",
			query:     `DELETE FROM magic_link_tokens WHERE expires_at < ?`,
			args:      []any{now.Unix()},
			wantIndex: "idx_cp_ml_expires_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := explainMagicLinkQueryPlan(t, store.db, tt.query, tt.args...)
			if containsMagicLinkFullTableScan(plan) {
				t.Fatalf("query uses full table scan on magic_link_tokens\nPlan:\n%s", plan)
			}
			if !containsMagicLinkSearchWithIndex(plan, tt.wantIndex) {
				t.Fatalf("expected SEARCH using index %q\nPlan:\n%s", tt.wantIndex, plan)
			}
		})
	}
}

func newMagicLinkPlanTestStore(t *testing.T) (*Store, []byte, time.Time) {
	t.Helper()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { store.Close() })

	base := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	var sampleHash []byte

	for i := 0; i < 128; i++ {
		tokenHash := []byte(fmt.Sprintf("%032d", i))
		rec := &TokenRecord{
			Email:     fmt.Sprintf("user%02d@example.com", i),
			TenantID:  fmt.Sprintf("tenant-%02d", i%12),
			ExpiresAt: base.Add(time.Duration(i-64) * time.Minute),
		}
		if err := store.Put(tokenHash, rec); err != nil {
			t.Fatalf("Put(%d): %v", i, err)
		}
		if i == 42 {
			sampleHash = append([]byte(nil), tokenHash...)
		}
	}

	if _, err := store.db.Exec(`ANALYZE`); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}
	return store, sampleHash, base
}

func explainMagicLinkQueryPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
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

func containsMagicLinkSearchWithIndex(plan, indexName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") &&
			strings.Contains(line, "magic_link_tokens") &&
			strings.Contains(line, indexName) {
			return true
		}
	}
	return false
}

func containsMagicLinkFullTableScan(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if !strings.Contains(line, "SCAN") || !strings.Contains(line, "magic_link_tokens") {
			continue
		}
		if strings.Contains(line, "USING") && strings.Contains(line, "INDEX") {
			continue
		}
		return true
	}
	return false
}
