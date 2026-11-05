package notifications

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

// Receipt lifecycle is positive evidence: absence from a snapshot (including a
// missing/unreadable/stopped alert owner) never establishes resolution. The
// existing occurrence cancellation callback records resolution explicitly.
func (nq *NotificationQueue) migrateDeliveryReceiptContinuity() error {
	columns, err := nq.tableColumns("notification_delivery_receipts")
	if err != nil {
		return err
	}
	for _, column := range []string{"highest_severity", "delivered_nanos", "resolved_at"} {
		if !columns[column] {
			if _, err := nq.db.Exec(`ALTER TABLE notification_delivery_receipts ADD COLUMN ` + column + ` INTEGER NOT NULL DEFAULT 0`); err != nil {
				return fmt.Errorf("add receipt %s: %w", column, err)
			}
		}
	}
	_, err = nq.db.Exec(`CREATE TABLE IF NOT EXISTS notification_receipt_retirements (
		destination_key TEXT PRIMARY KEY, retired_at INTEGER NOT NULL
	)`)
	return err
}

func (nq *NotificationQueue) recordDeliveryReceipt(alertID string, start time.Time, destination string, completed time.Time, level alerts.AlertLevel) error {
	if nq == nil || strings.TrimSpace(alertID) == "" || start.IsZero() || strings.TrimSpace(destination) == "" {
		return fmt.Errorf("alert occurrence and destination are required for a delivery receipt")
	}
	if completed.IsZero() {
		completed = time.Now()
	}
	nq.mu.Lock()
	defer nq.mu.Unlock()
	_, err := nq.db.Exec(`INSERT INTO notification_delivery_receipts
		(alert_id, alert_start_nanos, destination_key, delivered_at, delivered_nanos, highest_severity)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(alert_id, alert_start_nanos, destination_key) DO UPDATE SET
			delivered_at = MAX(delivered_at, excluded.delivered_at),
			delivered_nanos = MAX(delivered_nanos, excluded.delivered_nanos),
			highest_severity = MAX(highest_severity, excluded.highest_severity)`,
		alertID, start.UnixNano(), destination, completed.Unix(), completed.UnixNano(), notificationSeverityRank(level))
	return err
}

// Called under the manager's admission lease and mutex, before storage can be
// closed by Stop. Only current destinations may contribute restart cooldown.
func (nq *NotificationQueue) deliveryCooldown(id string, start time.Time, destinations map[string]bool) (notificationRecord, bool, error) {
	record := notificationRecord{alertStart: start}
	if id == "" || start.IsZero() || len(destinations) == 0 {
		return record, false, nil
	}
	nq.mu.RLock()
	defer nq.mu.RUnlock()
	rows, err := nq.db.Query(`SELECT destination_key, delivered_at, delivered_nanos, highest_severity
		FROM notification_delivery_receipts WHERE alert_id = ? AND alert_start_nanos = ?`, id, start.UnixNano())
	if err != nil {
		return record, false, err
	}
	defer rows.Close()
	var highest int
	for rows.Next() {
		var destination string
		var seconds, nanos int64
		var severity int
		if err := rows.Scan(&destination, &seconds, &nanos, &severity); err != nil {
			return notificationRecord{}, false, err
		}
		// Legacy receipts have no severity history. They still authorise
		// recovery, but cannot suppress potentially new severity information.
		if !destinations[destination] || severity < 1 || severity > 3 {
			continue
		}
		if nanos == 0 {
			nanos = seconds * int64(time.Second)
		}
		completed := time.Unix(0, nanos)
		if completed.After(record.lastSent) {
			record.lastSent = completed
		}
		if severity > highest {
			highest = severity
		}
	}
	if err := rows.Err(); err != nil {
		return notificationRecord{}, false, err
	}
	switch highest {
	case 1:
		record.level = alerts.AlertLevelInfo
	case 2:
		record.level = alerts.AlertLevelWarning
	case 3:
		record.level = alerts.AlertLevelCritical
	}
	return record, highest > 0, nil
}

func (nq *NotificationQueue) resolveDeliveryReceipts(id string, start *time.Time, at time.Time) error {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	query := `UPDATE notification_delivery_receipts SET resolved_at = ? WHERE alert_id = ? AND resolved_at = 0`
	args := []any{at.Unix(), id}
	if start != nil {
		query += ` AND alert_start_nanos = ?`
		args = append(args, start.UnixNano())
	}
	_, err := nq.db.Exec(query, args...)
	return err
}

func (n *NotificationManager) receiptDestinationKeysLocked() map[string]bool {
	keys := make(map[string]bool)
	add := func(job notificationDeliveryJob, enabled bool) {
		if key := notificationDeliveryDestinationKey(job); enabled && key != "" {
			keys[key] = true
		}
	}
	add(notificationDeliveryJob{Type: "email", EmailConfig: &n.emailConfig}, n.emailConfig.Enabled)
	add(notificationDeliveryJob{Type: "apprise", AppriseConfig: &n.appriseConfig}, n.appriseConfig.Enabled)
	for _, webhook := range n.webhooks {
		add(notificationDeliveryJob{Type: "webhook", WebhookConfig: &webhook}, webhook.Enabled)
	}
	return keys
}

func receiptDestinationScope(keys map[string]bool) string {
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	if len(ordered) == 0 {
		return "-"
	}
	return strings.Join(ordered, ",")
}

// Explicit removal/disable/rotation starts a 30-day grace. An exact revert
// during grace restores old obligations. Merely booting with no configuration
// is not removal evidence. Historic orphans without removal evidence remain
// uncertain, rather than being silently expired. Late in-flight receipts use
// the same durable retirement marker as receipts already present at removal.
func (n *NotificationManager) updateReceiptDestinationsLocked(previous map[string]bool) {
	if n.queue == nil || !n.lifecycle.join() {
		return
	}
	defer n.lifecycle.release()
	current := n.receiptDestinationKeysLocked()
	if receiptDestinationScope(previous) == receiptDestinationScope(current) {
		return
	}
	if err := n.queue.updateReceiptDestinations(previous, current, time.Now()); err != nil {
		log.Error().Err(err).Msg("Could not retain destination receipt retirement; obligations preserved")
	}
}

func (nq *NotificationQueue) updateReceiptDestinations(previous, current map[string]bool, at time.Time) error {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	tx, err := nq.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key := range previous {
		if !current[key] {
			if _, err := tx.Exec(`INSERT INTO notification_receipt_retirements(destination_key, retired_at) VALUES (?, ?)
				ON CONFLICT(destination_key) DO NOTHING`, key, at.Unix()); err != nil {
				return err
			}
		}
	}
	for key := range current {
		if !previous[key] {
			if _, err := tx.Exec(`DELETE FROM notification_receipt_retirements WHERE destination_key = ?`, key); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// Pending/retryable work pins only its exact destination/occurrences. Failure
// to read or decode it is uncertainty, never permission for age-only deletion.
func (nq *NotificationQueue) pendingReceiptKeysNoLock() (map[string]bool, map[string]bool, error) {
	rows, err := nq.db.Query(`SELECT type, alerts, config FROM notification_queue WHERE status IN ('pending', 'sending', 'failed', 'dlq')`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	keys, destinations := make(map[string]bool), make(map[string]bool)
	for rows.Next() {
		var kind string
		var payload, config []byte
		if err := rows.Scan(&kind, &payload, &config); err != nil {
			return nil, nil, err
		}
		base, _ := normalizeQueueType(kind)
		job := notificationDeliveryJob{Type: base}
		var decodeErr error
		switch base {
		case "email":
			job.EmailConfig = &EmailConfig{}
			decodeErr = json.Unmarshal(config, job.EmailConfig)
		case "webhook":
			job.WebhookConfig = &WebhookConfig{}
			decodeErr = json.Unmarshal(config, job.WebhookConfig)
		case "apprise":
			job.AppriseConfig = &AppriseConfig{}
			decodeErr = json.Unmarshal(config, job.AppriseConfig)
		default:
			return nil, nil, fmt.Errorf("unknown pending notification type")
		}
		if decodeErr != nil {
			return nil, nil, fmt.Errorf("unreadable pending notification destination")
		}
		if err := json.Unmarshal(payload, &job.Alerts); err != nil {
			return nil, nil, fmt.Errorf("unreadable pending notification occurrences")
		}
		destination := notificationDeliveryDestinationKey(job)
		if destination == "" {
			return nil, nil, fmt.Errorf("unknown pending notification destination")
		}
		destinations[destination] = true
		for _, alert := range job.Alerts {
			if key := notificationDeliveryReceiptKey(alert, destination); key != "" {
				keys[key] = true
			} else {
				return nil, nil, fmt.Errorf("unknown pending notification occurrence")
			}
		}
	}
	return keys, destinations, rows.Err()
}

func (nq *NotificationQueue) cleanupDeliveryReceiptsNoLock(cutoff int64) error {
	pinned, pendingDestinations, err := nq.pendingReceiptKeysNoLock()
	if err != nil {
		return err
	}
	rows, err := nq.db.Query(`SELECT r.alert_id, r.alert_start_nanos, r.destination_key
		FROM notification_delivery_receipts r
		LEFT JOIN notification_receipt_retirements d ON d.destination_key = r.destination_key
		WHERE r.delivered_at < ? AND ((r.resolved_at > 0 AND r.resolved_at < ?) OR (d.retired_at > 0 AND d.retired_at < ?))`, cutoff, cutoff, cutoff)
	if err != nil {
		return err
	}
	type receipt struct {
		id, destination string
		start           int64
	}
	var removable []receipt
	for rows.Next() {
		var r receipt
		if err := rows.Scan(&r.id, &r.start, &r.destination); err != nil {
			rows.Close()
			return err
		}
		key := notificationDeliveryReceiptKey(&alerts.Alert{ID: r.id, StartTime: time.Unix(0, r.start)}, r.destination)
		if !pinned[key] {
			removable = append(removable, r)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, r := range removable {
		if _, err := nq.db.Exec(`DELETE FROM notification_delivery_receipts WHERE alert_id = ? AND alert_start_nanos = ? AND destination_key = ?`, r.id, r.start, r.destination); err != nil {
			return err
		}
	}
	// Prune retirement markers too, unless a late queued attempt can still
	// produce a receipt for this destination.
	rows, err = nq.db.Query(`SELECT destination_key FROM notification_receipt_retirements d WHERE retired_at < ?
		AND NOT EXISTS (SELECT 1 FROM notification_delivery_receipts r WHERE r.destination_key = d.destination_key)`, cutoff)
	if err != nil {
		return err
	}
	var retired []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return err
		}
		if !pendingDestinations[key] {
			retired = append(retired, key)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, key := range retired {
		if _, err := nq.db.Exec(`DELETE FROM notification_receipt_retirements WHERE destination_key = ?`, key); err != nil {
			return err
		}
	}
	return nil
}
