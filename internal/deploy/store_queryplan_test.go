package deploy

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDeployStoreQueryPlansUseIndexes validates that the main deploy-store list
// and cleanup queries keep using their intended indexes rather than regressing
// to temp-sort or table-scan plans.
//
// The SQL is intentionally duplicated from store.go so the guardrail tracks the
// real runtime queries, not an approximation.
func TestDeployStoreQueryPlansUseIndexes(t *testing.T) {
	t.Parallel()

	s, now := newDeployPlanTestStore(t)

	tests := []struct {
		name      string
		query     string
		args      []any
		wantIndex string
	}{
		{
			name: "list jobs by org uses org created index",
			query: `
				SELECT id, cluster_id, cluster_name, source_agent_id, source_node_id, org_id, status, max_parallel, retry_max, created_at, updated_at, completed_at
				FROM deploy_jobs WHERE org_id = ? ORDER BY created_at DESC LIMIT ?
			`,
			args:      []any{"org-03", 50},
			wantIndex: "idx_deploy_jobs_org_created",
		},
		{
			name: "get targets for job uses job created index",
			query: `
				SELECT id, job_id, node_id, node_name, node_ip, arch, status, error_message, attempts, created_at, updated_at
				FROM deploy_targets WHERE job_id = ? ORDER BY created_at ASC
			`,
			args:      []any{"job-010"},
			wantIndex: "idx_deploy_targets_job_created",
		},
		{
			name: "list job events uses job created index",
			query: `
				SELECT id, job_id, target_id, type, message, data, created_at
				FROM deploy_events WHERE job_id = ? ORDER BY created_at ASC
			`,
			args:      []any{"job-010"},
			wantIndex: "idx_deploy_events_job_created",
		},
		{
			name: "list target events uses target created index",
			query: `
				SELECT id, job_id, target_id, type, message, data, created_at
				FROM deploy_events WHERE target_id = ? ORDER BY created_at ASC
			`,
			args:      []any{"target-010"},
			wantIndex: "idx_deploy_events_target_created",
		},
		{
			name:      "cleanup old jobs uses completed index",
			query:     `DELETE FROM deploy_jobs WHERE completed_at IS NOT NULL AND completed_at < ?`,
			args:      []any{now.Add(-24 * time.Hour).Format(time.RFC3339)},
			wantIndex: "idx_deploy_jobs_completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := explainDeployQueryPlan(t, s.db, tt.query, tt.args...)

			if containsDeployFullTableScan(plan) {
				t.Fatalf("query uses full table scan on deploy tables\nPlan:\n%s", plan)
			}
			if strings.Contains(plan, "USE TEMP B-TREE FOR ORDER BY") {
				t.Fatalf("query regressed to temp B-tree ordering\nPlan:\n%s", plan)
			}
			if !containsDeploySearchWithIndex(plan, tt.wantIndex) {
				t.Fatalf("expected SEARCH using index %q\nPlan:\n%s", tt.wantIndex, plan)
			}
		})
	}
}

func newDeployPlanTestStore(t *testing.T) (*Store, time.Time) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "deploy.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	base := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 180; i++ {
		id := fmt.Sprintf("job-%03d", i)
		job := &Job{
			ID:            id,
			ClusterID:     fmt.Sprintf("cluster-%02d", i%12),
			ClusterName:   fmt.Sprintf("Cluster %02d", i%12),
			SourceAgentID: fmt.Sprintf("agent-%03d", i),
			SourceNodeID:  fmt.Sprintf("node-%03d", i),
			OrgID:         fmt.Sprintf("org-%02d", i%8),
			Status:        JobRunning,
			MaxParallel:   2,
			RetryMax:      3,
			CreatedAt:     base.Add(time.Duration(i) * time.Minute),
			UpdatedAt:     base.Add(time.Duration(i) * time.Minute),
		}
		if i%5 == 0 {
			job.Status = JobSucceeded
			completedAt := base.Add(time.Duration(i+30) * time.Minute)
			job.CompletedAt = &completedAt
		}
		if err := s.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob(%d): %v", i, err)
		}
	}

	for i := 0; i < 480; i++ {
		target := &Target{
			ID:        fmt.Sprintf("target-%03d", i),
			JobID:     fmt.Sprintf("job-%03d", i%180),
			NodeID:    fmt.Sprintf("node-%03d", i),
			NodeName:  fmt.Sprintf("Node %03d", i),
			NodeIP:    fmt.Sprintf("10.0.0.%d", i%255),
			Status:    TargetPending,
			Attempts:  i % 3,
			CreatedAt: base.Add(time.Duration(i) * time.Second),
			UpdatedAt: base.Add(time.Duration(i) * time.Second),
			Arch:      "",
		}
		if err := s.CreateTarget(ctx, target); err != nil {
			t.Fatalf("CreateTarget(%d): %v", i, err)
		}
	}

	for i := 0; i < 640; i++ {
		event := &Event{
			ID:        fmt.Sprintf("event-%03d", i),
			JobID:     fmt.Sprintf("job-%03d", i%180),
			TargetID:  fmt.Sprintf("target-%03d", i%480),
			Type:      EventTargetStatusChanged,
			Message:   "deploy event",
			Data:      "",
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		if err := s.AppendEvent(ctx, event); err != nil {
			t.Fatalf("AppendEvent(%d): %v", i, err)
		}
	}

	if _, err := s.db.Exec(`ANALYZE`); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}

	return s, base
}

func explainDeployQueryPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
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

func containsDeploySearchWithIndex(plan, indexName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") && strings.Contains(line, indexName) {
			return true
		}
	}
	return false
}

func containsDeployFullTableScan(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if !strings.Contains(line, "SCAN") {
			continue
		}
		if !(strings.Contains(line, "deploy_jobs") || strings.Contains(line, "deploy_targets") || strings.Contains(line, "deploy_events")) {
			continue
		}
		if strings.Contains(line, "USING") && strings.Contains(line, "INDEX") {
			continue
		}
		return true
	}
	return false
}
