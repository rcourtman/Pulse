package licensing

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestConversionStoreQueryPlansUseIndexes validates that the runtime
// conversion-store read paths continue to use indexed lookups on
// conversion_events instead of regressing to full table scans.
//
// The SQL here is intentionally duplicated from conversion_store.go so a query
// shape change must update the guardrail in lockstep.
func TestConversionStoreQueryPlansUseIndexes(t *testing.T) {
	t.Parallel()

	store := newConversionPlanTestStore(t)

	tests := []struct {
		name                  string
		query                 string
		args                  []any
		wantIndex             string
		allowTempBTreeGroupBy bool
	}{
		{
			name: "query by org ordered by time",
			query: `
				SELECT
					id,
					org_id,
					event_type,
					surface,
					capability,
					idempotency_key,
					CAST(strftime('%s', created_at) AS INTEGER) AS created_at_unix
				FROM conversion_events
				WHERE org_id = ?
				ORDER BY created_at ASC, id ASC
			`,
			args:      []any{"org-03"},
			wantIndex: "idx_conversion_events_org_time",
		},
		{
			name: "query by org time range and type",
			query: `
				SELECT
					id,
					org_id,
					event_type,
					surface,
					capability,
					idempotency_key,
					CAST(strftime('%s', created_at) AS INTEGER) AS created_at_unix
				FROM conversion_events
				WHERE org_id = ? AND event_type = ? AND created_at >= ? AND created_at < ?
				ORDER BY created_at ASC, id ASC
			`,
			args: []any{
				"org-03",
				EventTrialStarted,
				time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
				time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
			},
			wantIndex: "idx_conversion_events_org_time",
		},
		{
			name: "funnel summary by org and time range",
			query: `
				SELECT event_type, COUNT(1)
				FROM conversion_events
				WHERE created_at >= ? AND created_at < ? AND org_id = ?
				GROUP BY event_type
			`,
			args: []any{
				time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
				time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
				"org-03",
			},
			wantIndex:             "idx_conversion_events_org_time",
			allowTempBTreeGroupBy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := explainConversionQueryPlan(t, store.db, tt.query, tt.args...)

			if containsConversionFullTableScan(plan) {
				t.Fatalf("query uses full table scan on conversion_events\nPlan:\n%s", plan)
			}
			if !containsConversionSearchWithIndex(plan, tt.wantIndex) {
				t.Fatalf("expected SEARCH on conversion_events using index %q\nPlan:\n%s", tt.wantIndex, plan)
			}
			if strings.Contains(plan, "USE TEMP B-TREE FOR GROUP BY") && !tt.allowTempBTreeGroupBy {
				t.Fatalf("unexpected temp B-tree group-by spill\nPlan:\n%s", plan)
			}
		})
	}
}

func newConversionPlanTestStore(t *testing.T) *ConversionStore {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "conversion.db")
	store, err := NewConversionStore(dbPath)
	if err != nil {
		t.Fatalf("NewConversionStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	eventTypes := []string{
		EventPaywallViewed,
		EventTrialStarted,
		EventUpgradeClicked,
		EventCheckoutCompleted,
	}

	for i := 0; i < 512; i++ {
		event := StoredConversionEvent{
			OrgID:          "org-" + twoDigit(i%12),
			EventType:      eventTypes[i%len(eventTypes)],
			Surface:        "settings",
			Capability:     "pro",
			IdempotencyKey: "evt-" + threeDigit(i),
			CreatedAt:      base.Add(time.Duration(i) * time.Minute),
		}
		if err := store.Record(event); err != nil {
			t.Fatalf("Record(%d) error = %v", i, err)
		}
	}

	if _, err := store.db.Exec(`ANALYZE`); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}

	return store
}

func explainConversionQueryPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()

	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v\nQuery: %s", err, query)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var (
			id     int
			parent int
			aux    int
			detail string
		)
		if err := rows.Scan(&id, &parent, &aux, &detail); err != nil {
			t.Fatalf("scan plan row: %v", err)
		}
		lines = append(lines, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("EXPLAIN QUERY PLAN returned no rows")
	}
	return strings.Join(lines, "\n")
}

func containsConversionSearchWithIndex(plan, indexName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") &&
			conversionLineRefersToEvents(line) &&
			strings.Contains(line, indexName) {
			return true
		}
	}
	return false
}

func containsConversionFullTableScan(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if !strings.Contains(line, "SCAN") || !conversionLineRefersToEvents(line) {
			continue
		}
		if strings.Contains(line, "USING") && strings.Contains(line, "INDEX") {
			continue
		}
		return true
	}
	return false
}

func conversionLineRefersToEvents(line string) bool {
	for _, needle := range []string{"conversion_events ", "conversion_events("} {
		if strings.Contains(line, needle) {
			return true
		}
	}
	return strings.HasSuffix(strings.TrimSpace(line), "conversion_events")
}

func twoDigit(v int) string {
	return fmt.Sprintf("%02d", v)
}

func threeDigit(v int) string {
	return fmt.Sprintf("%03d", v)
}
