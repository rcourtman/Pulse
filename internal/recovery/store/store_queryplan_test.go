package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

// TestQueryPlansUseRecoveryIndexes validates that the critical recovery list
// queries use indexed lookups on recovery_points instead of regressing to full
// table scans. The SQL is intentionally duplicated from store.go so the test
// fails if the runtime query shape changes without updating the guardrail.
func TestQueryPlansUseRecoveryIndexes(t *testing.T) {
	t.Parallel()

	store := newRecoveryPlanTestStore(t)

	tests := []struct {
		name      string
		query     string
		args      []any
		wantIndex string
	}{
		{
			name:      "list points count by cluster label",
			query:     `SELECT COUNT(*) FROM recovery_points WHERE cluster_label = ?`,
			args:      []any{"cluster-01"},
			wantIndex: "idx_recovery_points_cluster_completed",
		},
		{
			name: "list points page by cluster label",
			query: `
				SELECT
					id, provider, kind, mode, outcome,
					started_at_ms, completed_at_ms, size_bytes,
					verified, encrypted, immutable,
					subject_resource_id, repository_resource_id,
					subject_ref_json, repository_ref_json, details_json,
					subject_label, subject_type, is_workload,
					cluster_label, node_host_label, namespace_label, entity_id_label,
					repository_label, details_summary
				FROM recovery_points
				WHERE cluster_label = ?
				ORDER BY (completed_at_ms IS NULL) ASC, completed_at_ms DESC, updated_at_ms DESC
				LIMIT ? OFFSET ?
			`,
			args:      []any{"cluster-01", 50, 0},
			wantIndex: "idx_recovery_points_cluster_completed",
		},
		{
			name: "list rollups count by subject key",
			query: `
				WITH filtered AS (
					SELECT subject_key
					FROM recovery_points
					WHERE subject_key IS NOT NULL AND subject_key != '' AND subject_key = ?
				)
				SELECT COUNT(*) FROM (SELECT subject_key FROM filtered GROUP BY subject_key)
			`,
			args:      []any{"res:vm-010"},
			wantIndex: "idx_recovery_points_subject_key_completed",
		},
		{
			name: "list rollups page by subject key",
			query: `
				WITH filtered AS (
					SELECT
						subject_key,
						subject_resource_id,
						subject_ref_json,
						provider,
						outcome,
						COALESCE(completed_at_ms, started_at_ms, created_at_ms) AS ts_ms,
						updated_at_ms,
						id
					FROM recovery_points
					WHERE subject_key IS NOT NULL AND subject_key != '' AND subject_key = ?
				),
				agg AS (
					SELECT
						subject_key,
						MAX(ts_ms) AS last_attempt_ms,
						MAX(CASE WHEN outcome = 'success' THEN ts_ms END) AS last_success_ms
					FROM filtered
					GROUP BY subject_key
				),
				ranked AS (
					SELECT
						subject_key,
						outcome,
						ROW_NUMBER() OVER (PARTITION BY subject_key ORDER BY ts_ms DESC, updated_at_ms DESC, id DESC) AS rn
					FROM filtered
				),
				latest AS (
					SELECT subject_key, subject_resource_id, subject_ref_json
					FROM (
						SELECT
							subject_key,
							subject_resource_id,
							subject_ref_json,
							ROW_NUMBER() OVER (PARTITION BY subject_key ORDER BY ts_ms DESC, updated_at_ms DESC, id DESC) AS rn
						FROM filtered
					) x
					WHERE x.rn = 1
				),
				providers AS (
					SELECT subject_key, GROUP_CONCAT(DISTINCT provider) AS providers_csv
					FROM filtered
					GROUP BY subject_key
				)
				SELECT
					agg.subject_key,
					latest.subject_resource_id,
					latest.subject_ref_json,
					agg.last_attempt_ms,
					agg.last_success_ms,
					(SELECT outcome FROM ranked r WHERE r.subject_key = agg.subject_key AND r.rn = 1) AS last_outcome,
					providers.providers_csv
				FROM agg
				LEFT JOIN latest USING(subject_key)
				LEFT JOIN providers USING(subject_key)
				ORDER BY (agg.last_attempt_ms IS NULL) ASC, agg.last_attempt_ms DESC, agg.subject_key ASC
				LIMIT ? OFFSET ?
			`,
			args:      []any{"res:vm-010", 50, 0},
			wantIndex: "idx_recovery_points_subject_key_completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := explainRecoveryQueryPlan(t, store.db, tt.query, tt.args...)

			if containsRecoveryFullTableScan(plan) {
				t.Fatalf("query uses full table scan on recovery_points\nPlan:\n%s", plan)
			}
			if !containsRecoverySearchWithIndex(plan, tt.wantIndex) {
				t.Fatalf("expected SEARCH on recovery_points using index %q\nPlan:\n%s", tt.wantIndex, plan)
			}
		})
	}
}

func newRecoveryPlanTestStore(t *testing.T) *Store {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "recovery.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	base := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	points := make([]recovery.RecoveryPoint, 0, 512)

	for i := 0; i < 512; i++ {
		started := base.Add(time.Duration(i) * time.Minute)
		completed := started.Add(2 * time.Minute)
		points = append(points, recovery.RecoveryPoint{
			ID:                makeRecoveryPointID(i),
			Provider:          recovery.ProviderKubernetes,
			Kind:              recovery.KindBackup,
			Mode:              recovery.ModeRemote,
			Outcome:           recoveryOutcomeForIndex(i),
			StartedAt:         &started,
			CompletedAt:       &completed,
			SubjectResourceID: makeRecoveryResourceID(i),
			Details: map[string]any{
				"sequence": i,
			},
			Display: &recovery.RecoveryPointDisplay{
				SubjectLabel:    "subject-" + twoDigit(i%32),
				SubjectType:     recoverySubjectTypeForIndex(i),
				IsWorkload:      i%2 == 1,
				ClusterLabel:    "cluster-" + twoDigit(i%16),
				NodeHostLabel:   "node-" + twoDigit(i%24),
				NamespaceLabel:  "ns-" + twoDigit(i%12),
				EntityIDLabel:   "entity-" + twoDigit(i),
				RepositoryLabel: "repo-" + twoDigit(i%6),
				DetailsSummary:  "summary-" + twoDigit(i),
			},
		})
	}

	if err := store.UpsertPoints(context.Background(), points); err != nil {
		t.Fatalf("UpsertPoints() error = %v", err)
	}
	if _, err := store.db.Exec(`ANALYZE`); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}

	return store
}

func explainRecoveryQueryPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
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

func containsRecoverySearchWithIndex(plan, indexName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") &&
			recoveryLineRefersToPoints(line) &&
			strings.Contains(line, indexName) {
			return true
		}
	}
	return false
}

func containsRecoveryFullTableScan(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if !strings.Contains(line, "SCAN") || !recoveryLineRefersToPoints(line) {
			continue
		}
		if strings.Contains(line, "USING") && strings.Contains(line, "INDEX") {
			continue
		}
		return true
	}
	return false
}

func recoveryLineRefersToPoints(line string) bool {
	for _, needle := range []string{"recovery_points ", "recovery_points("} {
		if strings.Contains(line, needle) {
			return true
		}
	}
	return strings.HasSuffix(strings.TrimSpace(line), "recovery_points")
}

func makeRecoveryPointID(i int) string {
	return fmt.Sprintf("point-%03d", i)
}

func makeRecoveryResourceID(i int) string {
	return fmt.Sprintf("vm-%03d", i%40)
}

func recoverySubjectTypeForIndex(i int) string {
	if i%2 == 0 {
		return "vm"
	}
	return "pod"
}

func recoveryOutcomeForIndex(i int) recovery.Outcome {
	switch i % 4 {
	case 0:
		return recovery.OutcomeSuccess
	case 1:
		return recovery.OutcomeFailed
	case 2:
		return recovery.OutcomeRunning
	default:
		return recovery.OutcomeSuccess
	}
}

func twoDigit(v int) string {
	return fmt.Sprintf("%02d", v)
}
