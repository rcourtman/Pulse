package notifications

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// The delivery log is the per-attempt record behind the delivery health
// verdict. Health says whether anything is wrong; the log is the evidence a
// user can read to see what fired, where it went, and what happened to it.
// It reads the same audit rows RecordAudit writes, so the log and the
// aggregate verdicts cannot disagree about what was attempted.

// Delivery outcomes as surfaced to callers. These name what happened to one
// attempt, not the row's live queue status: a pending row with attempts is a
// retry in flight, not a success or a terminal failure.
const (
	DeliveryOutcomeSent       = "sent"
	DeliveryOutcomeRetry      = "retry"
	DeliveryOutcomeFailed     = "failed"
	DeliveryOutcomeDeadLetter = "dead_letter"
	DeliveryOutcomeCancelled  = "cancelled"
)

// maxDeliveryLogEntries bounds one read so an install with a large retained
// audit trail cannot be asked for an unbounded payload.
const maxDeliveryLogEntries = 200

// defaultDeliveryLogEntries is the page size when the caller does not ask for
// a specific limit.
const defaultDeliveryLogEntries = 50

// DeliveryLogEntry is one recorded delivery attempt outcome.
type DeliveryLogEntry struct {
	NotificationID string    `json:"notificationId"`
	Type           string    `json:"type"`
	Method         string    `json:"method,omitempty"`
	DestinationID  string    `json:"destinationId,omitempty"`
	Outcome        string    `json:"outcome"`
	AlertIDs       []string  `json:"alertIds"`
	AlertCount     int       `json:"alertCount"`
	Attempts       int       `json:"attempts"`
	Success        bool      `json:"success"`
	ErrorMessage   string    `json:"errorMessage,omitempty"`
	FailureClass   string    `json:"failureClass,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// deliveryOutcome is the single rule turning an audit row's status into the
// outcome callers act on. RecordAudit uses it for the operational-trust
// metric and GetDeliveryLog uses it for the user-facing log, so the two can
// never label the same attempt differently.
func deliveryOutcome(status NotificationQueueStatus, success bool, attempts int) string {
	switch {
	case success && status == QueueStatusSent:
		return DeliveryOutcomeSent
	case status == QueueStatusDLQ:
		return DeliveryOutcomeDeadLetter
	case status == QueueStatusCancelled:
		return DeliveryOutcomeCancelled
	case status == QueueStatusPending && attempts > 0:
		return DeliveryOutcomeRetry
	}
	return DeliveryOutcomeFailed
}

// GetDeliveryLog returns recorded delivery attempts on or after since, newest
// first. Callers can use the longest queue retention window: audit cleanup
// removes sent, failed, and cancelled rows with their seven-day queue records,
// while dead-letter rows remain available with their queue records for 30
// days. The API names both windows so that mixed retention is not presented as
// uniformly complete history.
func (nq *NotificationQueue) GetDeliveryLog(since time.Time, limit int) ([]DeliveryLogEntry, error) {
	if nq == nil {
		return nil, fmt.Errorf("notification queue not initialized")
	}
	if since.IsZero() {
		since = time.Now().Add(-7 * 24 * time.Hour)
	}
	if limit <= 0 {
		limit = defaultDeliveryLogEntries
	}
	if limit > maxDeliveryLogEntries {
		limit = maxDeliveryLogEntries
	}

	nq.mu.RLock()
	defer nq.mu.RUnlock()

	rows, err := nq.db.Query(`
		SELECT notification_id, type, method, status, alert_identifiers,
			alert_count, operational_links, attempts, success, error_message,
			failure_class, destination_id, timestamp
		FROM notification_audit
		WHERE timestamp >= ?
		ORDER BY timestamp DESC, id DESC
		LIMIT ?
	`, since.UTC().Unix(), limit)
	if err != nil {
		return nil, fmt.Errorf("read notification delivery log: %w", err)
	}
	defer rows.Close()

	entries := make([]DeliveryLogEntry, 0, limit)
	for rows.Next() {
		var (
			entry            DeliveryLogEntry
			method           sql.NullString
			status           sql.NullString
			alertIdentifiers sql.NullString
			alertCount       sql.NullInt64
			operationalLinks sql.NullString
			attempts         sql.NullInt64
			success          sql.NullBool
			errorMessage     sql.NullString
			failureClass     sql.NullString
			destinationID    sql.NullString
			timestamp        int64
		)
		if err := rows.Scan(
			&entry.NotificationID,
			&entry.Type,
			&method,
			&status,
			&alertIdentifiers,
			&alertCount,
			&operationalLinks,
			&attempts,
			&success,
			&errorMessage,
			&failureClass,
			&destinationID,
			&timestamp,
		); err != nil {
			return nil, fmt.Errorf("scan notification delivery log row: %w", err)
		}

		entry.Method = method.String
		entry.Attempts = int(attempts.Int64)
		entry.Success = success.Bool
		entry.ErrorMessage = errorMessage.String
		entry.FailureClass = failureClass.String
		entry.Timestamp = time.Unix(timestamp, 0).UTC()
		entry.Outcome = deliveryOutcome(
			NotificationQueueStatus(status.String), success.Bool, int(attempts.Int64),
		)

		entry.AlertIDs = decodeAuditAlertIdentifiers(alertIdentifiers.String)
		entry.AlertCount = int(alertCount.Int64)
		if entry.AlertCount == 0 {
			entry.AlertCount = len(entry.AlertIDs)
		}
		// Rows written before the destination_id column existed still carry the
		// destination inside their operational links.
		entry.DestinationID = strings.TrimSpace(destinationID.String)
		if entry.DestinationID == "" {
			entry.DestinationID = decodeAuditDestinationID(operationalLinks.String)
		}

		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification delivery log: %w", err)
	}
	return entries, nil
}

func decodeAuditAlertIdentifiers(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var identifiers []string
	if err := json.Unmarshal([]byte(raw), &identifiers); err != nil {
		return []string{}
	}
	if identifiers == nil {
		return []string{}
	}
	return identifiers
}

func decodeAuditDestinationID(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var links []operationaltrust.NotificationLink
	if err := json.Unmarshal([]byte(raw), &links); err != nil {
		return ""
	}
	for _, link := range links {
		if destination := strings.TrimSpace(link.DestinationID); destination != "" {
			return destination
		}
	}
	return ""
}

// GetDeliveryLog exposes the queue's delivery log through the manager. A
// missing queue is an error, not an empty log: silence must not read as "no
// attempts were made".
func (n *NotificationManager) GetDeliveryLog(since time.Time, limit int) ([]DeliveryLogEntry, error) {
	if n == nil {
		return nil, fmt.Errorf("notification manager not initialized")
	}
	n.mu.RLock()
	queue := n.queue
	n.mu.RUnlock()

	if queue == nil {
		return nil, fmt.Errorf("notification queue not initialized")
	}
	return queue.GetDeliveryLog(since, limit)
}
