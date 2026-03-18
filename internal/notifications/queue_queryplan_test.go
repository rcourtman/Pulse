package notifications

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// TestNotificationQueueQueryPlansUseIndexes validates that the queue's
// highest-volume read and cleanup queries keep using the intended indexes.
// The SQL is intentionally duplicated from queue.go so changes to the runtime
// query shapes must update this guardrail in lockstep.
func TestNotificationQueueQueryPlansUseIndexes(t *testing.T) {
	t.Parallel()

	nq, now := newNotificationPlanTestQueue(t)

	tests := []struct {
		name                string
		query               string
		args                []any
		wantTableIndexes    map[string]string
		allowOrderTempBTree bool
	}{
		{
			name: "get pending uses status index",
			query: `
				SELECT id, type, method, status, alerts, config, attempts, max_attempts,
				       last_attempt, last_error, created_at, next_retry_at, completed_at, payload_bytes
				FROM notification_queue
				WHERE status = 'pending'
				  AND (next_retry_at IS NULL OR next_retry_at <= ?)
				ORDER BY created_at ASC
				LIMIT ?
			`,
			args: []any{now.Unix(), 20},
			wantTableIndexes: map[string]string{
				"notification_queue": "idx_status",
			},
			allowOrderTempBTree: true,
		},
		{
			name: "get dlq uses status completed index",
			query: `
				SELECT id, type, method, status, alerts, config, attempts, max_attempts,
				       last_attempt, last_error, created_at, next_retry_at, completed_at, payload_bytes
				FROM notification_queue
				WHERE status = 'dlq'
				ORDER BY completed_at DESC
				LIMIT ?
			`,
			args: []any{20},
			wantTableIndexes: map[string]string{
				"notification_queue": "idx_status_completed",
			},
		},
		{
			name:  "cleanup completed queue rows uses status completed index",
			query: `DELETE FROM notification_queue WHERE status IN ('sent', 'failed', 'cancelled') AND completed_at < ?`,
			args:  []any{now.Add(-7 * 24 * time.Hour).Unix()},
			wantTableIndexes: map[string]string{
				"notification_queue": "idx_status_completed",
			},
		},
		{
			name: "cleanup audit rows uses queue and audit indexes",
			query: `
				DELETE FROM notification_audit WHERE notification_id IN (
					SELECT id FROM notification_queue WHERE status IN ('sent', 'failed', 'cancelled') AND completed_at < ?
				)
			`,
			args: []any{now.Add(-7 * 24 * time.Hour).Unix()},
			wantTableIndexes: map[string]string{
				"notification_queue": "idx_status_completed",
				"notification_audit": "idx_audit_notification_id",
			},
		},
		{
			name:  "cleanup old audit rows uses audit timestamp index",
			query: `DELETE FROM notification_audit WHERE timestamp < ?`,
			args:  []any{now.Add(-30 * 24 * time.Hour).Unix()},
			wantTableIndexes: map[string]string{
				"notification_audit": "idx_audit_timestamp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := explainNotificationQueryPlan(t, nq.db, tt.query, tt.args...)

			if containsNotificationFullTableScan(plan) {
				t.Fatalf("query uses full table scan on notification tables\nPlan:\n%s", plan)
			}
			for tableName, indexName := range tt.wantTableIndexes {
				if !containsNotificationSearchWithIndex(plan, tableName, indexName) {
					t.Fatalf("expected SEARCH on %s using index %q\nPlan:\n%s", tableName, indexName, plan)
				}
			}
			if strings.Contains(plan, "USE TEMP B-TREE FOR ORDER BY") && !tt.allowOrderTempBTree {
				t.Fatalf("unexpected temp B-tree order spill\nPlan:\n%s", plan)
			}
		})
	}
}

func newNotificationPlanTestQueue(t *testing.T) (*NotificationQueue, time.Time) {
	t.Helper()

	nq, err := NewNotificationQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewNotificationQueue() error = %v", err)
	}
	t.Cleanup(func() { _ = nq.Stop() })

	base := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	alertPayload, err := json.Marshal([]*alerts.Alert{
		{ID: "alert-seed"},
	})
	if err != nil {
		t.Fatalf("marshal alerts: %v", err)
	}

	tx, err := nq.db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	queueStmt, err := tx.Prepare(`
		INSERT INTO notification_queue
		(id, type, method, status, alerts, config, attempts, max_attempts, created_at, next_retry_at, completed_at, payload_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		t.Fatalf("prepare queue insert: %v", err)
	}
	defer queueStmt.Close()

	auditStmt, err := tx.Prepare(`
		INSERT INTO notification_audit
		(notification_id, type, method, status, alert_identifiers, alert_count, attempts, success, error_message, payload_size, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		t.Fatalf("prepare audit insert: %v", err)
	}
	defer auditStmt.Close()

	statuses := []NotificationQueueStatus{
		QueueStatusPending,
		QueueStatusPending,
		QueueStatusSent,
		QueueStatusFailed,
		QueueStatusDLQ,
		QueueStatusCancelled,
	}

	for i := 0; i < 512; i++ {
		status := statuses[i%len(statuses)]
		createdAt := base.Add(time.Duration(i) * time.Minute).Unix()

		var nextRetryAt any
		if status == QueueStatusPending {
			switch {
			case i%6 == 0:
				nextRetryAt = nil
			case i%2 == 0:
				nextRetryAt = base.Add(-2 * time.Minute).Unix()
			default:
				nextRetryAt = base.Add(2 * time.Minute).Unix()
			}
		}

		var completedAt any
		if status != QueueStatusPending {
			completedAt = base.Add(time.Duration(i-900) * time.Minute).Unix()
		}

		id := fmt.Sprintf("notif-%03d", i)
		if _, err := queueStmt.Exec(
			id,
			"email",
			"smtp",
			status,
			string(alertPayload),
			`{"target":"ops@example.com"}`,
			i%3,
			3,
			createdAt,
			nextRetryAt,
			completedAt,
			256,
		); err != nil {
			t.Fatalf("insert queue row %d: %v", i, err)
		}

		if _, err := auditStmt.Exec(
			id,
			"email",
			"smtp",
			status,
			`["alert-seed"]`,
			1,
			i%3,
			status == QueueStatusSent,
			"",
			256,
			base.Add(time.Duration(i-1200)*time.Minute).Unix(),
		); err != nil {
			t.Fatalf("insert audit row %d: %v", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit seed data: %v", err)
	}
	if _, err := nq.db.Exec(`ANALYZE`); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}

	return nq, base
}

func explainNotificationQueryPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
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

func containsNotificationSearchWithIndex(plan, tableName, indexName string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if strings.Contains(line, "SEARCH") &&
			notificationLineRefersToTable(line, tableName) &&
			strings.Contains(line, indexName) {
			return true
		}
	}
	return false
}

func containsNotificationFullTableScan(plan string) bool {
	for _, line := range strings.Split(plan, "\n") {
		if !strings.Contains(line, "SCAN") {
			continue
		}
		if !notificationLineRefersToTable(line, "notification_queue") &&
			!notificationLineRefersToTable(line, "notification_audit") {
			continue
		}
		if strings.Contains(line, "USING") && strings.Contains(line, "INDEX") {
			continue
		}
		return true
	}
	return false
}

func notificationLineRefersToTable(line, tableName string) bool {
	for _, needle := range []string{tableName + " ", tableName + "("} {
		if strings.Contains(line, needle) {
			return true
		}
	}
	return strings.HasSuffix(strings.TrimSpace(line), tableName)
}
