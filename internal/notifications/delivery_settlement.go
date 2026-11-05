package notifications

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

type notificationAcknowledgementError struct{ cause error }

func (e *notificationAcknowledgementError) Error() string {
	return "persist notification provider acknowledgement: " + e.cause.Error()
}
func (e *notificationAcknowledgementError) Unwrap() error { return e.cause }

// The existing successful provider audit is the durable acknowledgement.
// Keeping one authority avoids a second marker that could disagree with it.
func (q *NotificationQueue) providerAcknowledgedAtLocked(id string) (*time.Time, error) {
	var at int64
	err := q.db.QueryRow(`SELECT timestamp FROM notification_audit WHERE notification_id=? AND success=1 ORDER BY id LIMIT 1`, id).Scan(&at)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if now := time.Now().Unix(); at > now {
		if _, err := q.db.Exec(`UPDATE notification_audit SET timestamp=? WHERE notification_id=? AND success=1 AND timestamp>?`, now, id, now); err != nil {
			return nil, err
		}
		at = now
	}
	value := time.Unix(at, 0)
	return &value, nil
}

func (q *NotificationQueue) recoverAcknowledgedSending() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	rows, err := q.db.Query(`SELECT id FROM notification_queue WHERE status IN ('sending','failed','dlq') AND EXISTS(SELECT 1 FROM notification_audit a WHERE a.notification_id=notification_queue.id AND a.success=1)`)
	if err != nil {
		return err
	}
	var abandoned []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if _, owned := q.processingNotifications[id]; !owned {
			abandoned = append(abandoned, id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range abandoned {
		if _, err := q.db.Exec(`UPDATE notification_queue SET status='pending',last_error=?,next_retry_at=COALESCE(next_retry_at,?),completed_at=NULL WHERE id=? AND status IN ('sending','failed','dlq')`, notificationSettlementPendingMessage, time.Now().Unix(), id); err != nil {
			return err
		}
	}
	return nil
}

func (q *NotificationQueue) providerAcknowledgedAt(id string) (*time.Time, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.providerAcknowledgedAtLocked(id)
}

func (q *NotificationQueue) acceptedLinksLocked(id string) ([]operationaltrust.NotificationLink, bool, error) {
	var raw []byte
	err := q.db.QueryRow(`SELECT operational_links FROM notification_audit WHERE notification_id=? AND success=1 ORDER BY id LIMIT 1`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var links []operationaltrust.NotificationLink
	err = json.Unmarshal(raw, &links)
	return links, true, err
}

// Accepted occurrence facts are children of the successful audit, committed in
// the same transaction. Queue payloads remain provider snapshots, not the source
// of truth for an outcome that the provider has already accepted.
type acceptedDeliveryFact struct {
	auditID                   int64
	id                        string
	start                     int64
	destination, epoch, event string
	at                        int64
	severity                  int
	resolved                  int64
	consumed                  bool
}

type notificationSettlementStore interface {
	notificationAuditExecutor
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

func retainedDeliveryJob(kind string, config []byte) (notificationDeliveryJob, error) {
	base, event := normalizeQueueType(kind)
	job := notificationDeliveryJob{Type: base, Event: event}
	var err error
	switch base {
	case "email":
		job.EmailConfig = &EmailConfig{}
		err = json.Unmarshal(config, job.EmailConfig)
	case "webhook":
		job.WebhookConfig = &WebhookConfig{}
		err = json.Unmarshal(config, job.WebhookConfig)
	case "apprise":
		job.AppriseConfig = &AppriseConfig{}
		err = json.Unmarshal(config, job.AppriseConfig)
	default:
		return job, fmt.Errorf("unknown retained notification provider")
	}
	if err != nil {
		return job, err
	}
	identifiable := true
	switch base {
	case "webhook":
		identifiable = strings.TrimSpace(job.WebhookConfig.URL) != ""
	case "email":
		identifiable = notificationDeliveryDestinationKey(job) != ""
	case "apprise":
		identifiable = len(job.AppriseConfig.Targets) > 0 || strings.TrimSpace(job.AppriseConfig.ServerURL) != "" || strings.TrimSpace(job.AppriseConfig.ConfigKey) != ""
	}
	if !identifiable {
		return job, fmt.Errorf("retained notification receiver is unavailable")
	}
	return job, nil
}

func hasReceiptOccurrence(list []*alerts.Alert) bool {
	for _, alert := range list {
		if alert != nil && alert.ID != "" && !alert.StartTime.IsZero() {
			return true
		}
	}
	return false
}

func recordAcceptedDeliveryFacts(store notificationAuditExecutor, auditID int64, row *QueuedNotification, at time.Time) error {
	// Queue clients can send diagnostics without any occurrence obligation.
	// They need an acknowledgement, but no receiver-scoped receipt facts.
	if row.ReceiptEpoch != nil && hasReceiptOccurrence(row.Alerts) {
		if err := validateReceiptEpoch(*row.ReceiptEpoch); err != nil {
			return err
		}
		job, err := retainedDeliveryJob(row.Type, row.Config)
		if err != nil {
			return err
		}
		destination := notificationDeliveryDestinationKey(job)
		for _, alert := range row.Alerts {
			if alert == nil || alert.ID == "" || alert.StartTime.IsZero() || destination == "" {
				continue
			}
			if _, err := store.Exec(`INSERT INTO notification_delivery_facts(audit_id,alert_id,alert_start_nanos,destination_key,delivery_epoch,event,delivered_nanos,severity) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(audit_id,alert_id,alert_start_nanos) DO UPDATE SET severity=MAX(severity,excluded.severity)`, auditID, alert.ID, alert.StartTime.UnixNano(), destination, *row.ReceiptEpoch, string(job.Event), at.UnixNano(), notificationSeverityRank(alert.Level)); err != nil {
				return err
			}
		}
	}
	_, err := store.Exec(`UPDATE notification_audit SET receipt_facts_complete=1 WHERE id=?`, auditID)
	return err
}

func (q *NotificationQueue) migrateDeliveryFacts() error {
	columns, err := q.tableColumns("notification_audit")
	if err != nil {
		return err
	}
	if !columns["receipt_facts_complete"] {
		if _, err = q.db.Exec(`ALTER TABLE notification_audit ADD COLUMN receipt_facts_complete INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	_, err = q.db.Exec(`CREATE TABLE IF NOT EXISTS notification_delivery_facts(
 audit_id INTEGER NOT NULL REFERENCES notification_audit(id) ON DELETE CASCADE,
 alert_id TEXT NOT NULL,alert_start_nanos INTEGER NOT NULL,destination_key TEXT NOT NULL,delivery_epoch TEXT NOT NULL,event TEXT NOT NULL,
 delivered_nanos INTEGER NOT NULL,severity INTEGER NOT NULL,resolved_nanos INTEGER NOT NULL DEFAULT 0,consumed INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(audit_id,alert_id,alert_start_nanos));
 CREATE INDEX IF NOT EXISTS idx_delivery_facts_occurrence ON notification_delivery_facts(destination_key,delivery_epoch,alert_id,alert_start_nanos);
 CREATE INDEX IF NOT EXISTS idx_delivery_facts_disposition ON notification_delivery_facts(alert_id,alert_start_nanos);`)
	if err != nil {
		return err
	}
	factColumns, err := q.tableColumns("notification_delivery_facts")
	if err != nil {
		return err
	}
	if !factColumns["resolution_order"] {
		if _, err = q.db.Exec(`ALTER TABLE notification_delivery_facts ADD COLUMN resolution_order INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	// Earlier runtimes could record success after a failed final queue update.
	// Backfill only readable retained snapshots; unreadable accepted data remains
	// explicitly incomplete rather than being declared settled without its facts.
	rows, err := q.db.Query(`SELECT a.id,q.id,q.type,q.alerts,q.config,q.receipt_epoch,a.timestamp FROM notification_audit a JOIN notification_queue q ON q.id=a.notification_id WHERE a.success=1 AND a.receipt_facts_complete=0 AND q.status IN ('pending','sending','failed','dlq')`)
	if err != nil {
		return err
	}
	type legacy struct {
		auditID, at int64
		row         QueuedNotification
		alerts      []byte
	}
	var legacyRows []legacy
	for rows.Next() {
		var item legacy
		var rawConfig []byte
		if err := rows.Scan(&item.auditID, &item.row.ID, &item.row.Type, &item.alerts, &rawConfig, &item.row.ReceiptEpoch, &item.at); err != nil {
			rows.Close()
			return err
		}
		item.row.Config = rawConfig
		legacyRows = append(legacyRows, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range legacyRows {
		if json.Unmarshal(item.alerts, &item.row.Alerts) != nil {
			continue
		}
		if hasReceiptOccurrence(item.row.Alerts) {
			if _, err := retainedDeliveryJob(item.row.Type, item.row.Config); err != nil {
				continue
			}
		}
		if item.row.ReceiptEpoch == nil {
			epoch := ""
			item.row.ReceiptEpoch = &epoch
		}
		at := cooldownReceiptTime(item.at)
		tx, err := q.db.Begin()
		if err != nil {
			return err
		}
		if err = recordAcceptedDeliveryFacts(tx, item.auditID, &item.row, at); err == nil {
			_, err = tx.Exec(`UPDATE notification_delivery_facts SET resolved_nanos=COALESCE((SELECT r.resolved_at FROM notification_delivery_receipts r WHERE r.alert_id=notification_delivery_facts.alert_id AND r.alert_start_nanos=notification_delivery_facts.alert_start_nanos AND r.destination_key=notification_delivery_facts.destination_key AND r.delivery_epoch=notification_delivery_facts.delivery_epoch),0) WHERE audit_id=? AND event=?`, item.auditID, string(eventAlert))
		}
		if err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func acceptedFacts(store notificationSettlementStore, queueID string) ([]acceptedDeliveryFact, bool, error) {
	var auditID int64
	var complete bool
	err := store.QueryRow(`SELECT id,receipt_facts_complete FROM notification_audit WHERE notification_id=? AND success=1 ORDER BY id LIMIT 1`, queueID).Scan(&auditID, &complete)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !complete {
		return nil, true, fmt.Errorf("accepted notification occurrence facts are unavailable")
	}
	rows, err := store.Query(`SELECT audit_id,alert_id,alert_start_nanos,destination_key,delivery_epoch,event,delivered_nanos,severity,resolved_nanos,consumed FROM notification_delivery_facts WHERE audit_id=?`, auditID)
	if err != nil {
		return nil, true, err
	}
	defer rows.Close()
	var facts []acceptedDeliveryFact
	for rows.Next() {
		var f acceptedDeliveryFact
		if err := rows.Scan(&f.auditID, &f.id, &f.start, &f.destination, &f.epoch, &f.event, &f.at, &f.severity, &f.resolved, &f.consumed); err != nil {
			return nil, true, err
		}
		facts = append(facts, f)
	}
	return facts, true, rows.Err()
}

func (q *NotificationQueue) acknowledgedAlertIdentifiers(id string) ([]string, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	facts, _, err := acceptedFacts(q.db, id)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(facts))
	for _, fact := range facts {
		ids = append(ids, fact.id)
	}
	return ids, nil
}

// The delivery worker already holds these occurrences' read gates. Re-read
// disposition inside each projection write, so synchronous resolution cannot
// be undone by an older worker snapshot. No provider configuration is decoded.
func (q *NotificationQueue) settleAcknowledgedNotification(id string) (bool, error) {
	q.mu.RLock()
	facts, found, err := acceptedFacts(q.db, id)
	q.mu.RUnlock()
	if err != nil || !found {
		return found, err
	}
	for _, fact := range facts {
		start := time.Unix(0, fact.start)
		if fact.event == string(eventResolved) {
			err = q.deleteDeliveryReceiptEpoch(fact.id, start, fact.destination, fact.epoch, id)
		} else {
			err = q.recordDeliveryReceipt(fact.id, start, fact.destination, time.Unix(0, fact.at), fact.severity, fact.epoch, id)
		}
		if err != nil {
			return true, err
		}
	}
	return true, nil
}

const metadataReceiptRecovered = "_receipt_recovered"

func activationAfterRetainedResolution(activation, resolved time.Time) bool {
	if resolved.IsZero() {
		return false
	}
	if resolved.Nanosecond() == 0 {
		activation = activation.Truncate(time.Second)
	}
	return activation.After(resolved)
}

func receiptDisposition(alert *alerts.Alert) (time.Time, bool, error) {
	var at time.Time
	if value, exists := alert.Metadata[metadataResolvedAt]; exists {
		switch value := value.(type) {
		case string:
			var err error
			at, err = time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return at, false, err
			}
		case time.Time:
			at = value
		default:
			return at, false, fmt.Errorf("invalid retained notification resolution disposition")
		}
	}
	return at, false, nil
}

func updateAcknowledgedFirings(store notificationSettlementStore, id string, start time.Time, destination, epoch string, resolved *time.Time, recovered bool, maxOrder ...int64) error {
	query := `UPDATE notification_delivery_facts SET `
	var args []any
	if recovered {
		query += `consumed=1`
	} else if resolved != nil {
		query += `resolved_nanos=MAX(resolved_nanos,?)`
		args = append(args, resolved.UnixNano())
	} else {
		return nil
	}
	query += ` WHERE alert_id=? AND alert_start_nanos=? AND event=?`
	args = append(args, id, start.UnixNano(), string(eventAlert))
	if destination != "" {
		query += ` AND destination_key=? AND delivery_epoch=?`
		args = append(args, destination, epoch)
	}
	if len(maxOrder) > 0 {
		query += ` AND audit_id<=?`
		args = append(args, maxOrder[0])
	}
	_, err := store.Exec(query, args...)
	return err
}

func (q *NotificationQueue) queuedReceiptDispositionLocked(queueID, id string, start time.Time) (time.Time, bool, int64, error) {
	if queueID == "" {
		return time.Time{}, false, 0, nil
	}
	var resolved, resolutionOrder int64
	var consumed bool
	err := q.db.QueryRow(`SELECT COALESCE(MAX(f.resolved_nanos),0),COALESCE(MAX(f.consumed),0),COALESCE(MAX(f.resolution_order),0) FROM notification_delivery_facts f JOIN notification_audit a ON a.id=f.audit_id WHERE a.notification_id=? AND a.success=1 AND f.alert_id=? AND f.alert_start_nanos=? AND f.event=?`, queueID, id, start.UnixNano(), string(eventAlert)).Scan(&resolved, &consumed, &resolutionOrder)
	if err != nil {
		return time.Time{}, false, 0, err
	}
	if resolved == 0 {
		return time.Time{}, consumed, resolutionOrder, nil
	}
	return time.Unix(0, resolved), consumed, resolutionOrder, nil
}

func (q *NotificationQueue) acknowledgedReceiptLocked(alert *alerts.Alert, destination string) (notificationRecord, string, bool, bool, bool, error) {
	var epoch string
	err := q.db.QueryRow(`SELECT epoch FROM notification_destination_state WHERE destination_key=?`, destination).Scan(&epoch)
	if err == sql.ErrNoRows {
		return notificationRecord{}, "", false, false, false, nil
	}
	if err != nil {
		return notificationRecord{}, "", false, false, false, err
	}
	rows, err := q.db.Query(`SELECT f.audit_id,f.event,f.delivered_nanos,f.severity,f.resolved_nanos,f.consumed FROM notification_delivery_facts f JOIN notification_audit a ON a.id=f.audit_id AND a.success=1 JOIN notification_queue q ON q.id=a.notification_id WHERE f.destination_key=? AND f.delivery_epoch=? AND f.alert_id=? AND f.alert_start_nanos=? AND q.status IN ('pending','sending') ORDER BY f.audit_id`, destination, epoch, alert.ID, alert.StartTime.UnixNano())
	if err != nil {
		return notificationRecord{}, epoch, false, false, false, err
	}
	defer rows.Close()
	var record notificationRecord
	var recoveryOrder int64
	found, future := false, false
	levels := map[int]alerts.AlertLevel{1: alerts.AlertLevelInfo, 2: alerts.AlertLevelWarning, 3: alerts.AlertLevelCritical}
	for rows.Next() {
		var order, at, resolved int64
		var event string
		var rank int
		var consumed bool
		if err := rows.Scan(&order, &event, &at, &rank, &resolved, &consumed); err != nil {
			return record, epoch, false, false, false, err
		}
		if event == string(eventResolved) {
			recoveryOrder = order
			record = notificationRecord{}
			found = false
			continue
		}
		if consumed {
			continue
		}
		found = true
		future = future || at > time.Now().UnixNano()
		completed := time.Unix(0, at)
		if completed.After(record.lastSent) {
			record.lastSent = completed
		}
		if rank > notificationSeverityRank(record.level) {
			record.level = levels[rank]
		}
		if order > record.successOrder {
			record.successOrder = order
			record.resolved = resolved > 0
		}
		record.alertStart = alert.StartTime
	}
	if err := rows.Err(); err != nil {
		return record, epoch, false, false, false, err
	}
	recovered := recoveryOrder > 0 && recoveryOrder >= record.successOrder
	if recovered {
		record.successOrder = recoveryOrder
	}
	return record, epoch, found, recovered, future, nil
}

// Audit IDs provide a durable completion order independent of host clock
// corrections. Direct completions without an audit reserve a sequence value
// without inventing an audit entry, so late queued settlement stays older.
func (q *NotificationQueue) receiptSuccessOrderLocked(queueID string) (int64, error) {
	if queueID != "" {
		var order int64
		err := q.db.QueryRow(`SELECT id FROM notification_audit WHERE notification_id=? AND success=1 ORDER BY id LIMIT 1`, queueID).Scan(&order)
		if err == nil {
			return order, nil
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}
	if _, err := q.db.Exec(`INSERT INTO sqlite_sequence(name,seq) SELECT 'notification_audit',0 WHERE NOT EXISTS(SELECT 1 FROM sqlite_sequence WHERE name='notification_audit')`); err != nil {
		return 0, err
	}
	var order int64
	err := q.db.QueryRow(`UPDATE sqlite_sequence SET seq=MAX(COALESCE(seq,0),(SELECT COALESCE(MAX(id),0) FROM notification_audit))+1 WHERE name='notification_audit' RETURNING seq`).Scan(&order)
	return order, err
}

// Record provider success before receipt settlement. A retry reads this same
// audit and performs only local bookkeeping, including across process restart.
// If acknowledgement itself cannot persist, ordinary bounded provider retries
// remain the honest crash boundary rather than an unlimited resend loop.
func (q *NotificationQueue) acknowledgeProvider(row *QueuedNotification, at time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	previous, err := q.providerAcknowledgedAtLocked(row.ID)
	if err != nil {
		return &notificationAcknowledgementError{err}
	}
	if previous != nil {
		row.ProviderAcceptedAt = previous
		return nil
	}
	var exists int
	err = q.db.QueryRow(`SELECT 1 FROM notification_queue WHERE id=?`, row.ID).Scan(&exists)
	if err == sql.ErrNoRows {
		// Direct processor invocations have no durable queue row to replay.
		row.ProviderAcceptedAt = &at
		return nil
	}
	if err != nil {
		return &notificationAcknowledgementError{err}
	}
	accepted := *row
	accepted.Status = QueueStatusSent
	accepted.Links = transitionNotificationLinks(row.Links, operationaltrust.NotificationDelivered, at)
	tx, err := q.db.Begin()
	if err != nil {
		return &notificationAcknowledgementError{err}
	}
	defer tx.Rollback()
	if err := q.recordAuditWithExecutor(tx, &accepted, true, "", ""); err != nil {
		return &notificationAcknowledgementError{err}
	}
	if err := tx.Commit(); err != nil {
		return &notificationAcknowledgementError{err}
	}
	row.ProviderAcceptedAt = &at
	row.Links = accepted.Links
	operationaltrust.GetMetrics().ObserveNotificationDelivery(deliveryOutcome(QueueStatusSent, true, row.Attempts))
	return nil
}

func (q *NotificationQueue) deferSettlement(row *QueuedNotification, claimed bool, previousAttempt ...*time.Time) error {
	return q.deferWithReason(row, claimed, notificationSettlementPendingMessage, previousAttempt...)
}

func (q *NotificationQueue) deferWithReason(row *QueuedNotification, claimed bool, message string, previousAttempt ...*time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	state := operationaltrust.NotificationQueued
	decrement := claimed && row.ProviderAcceptedAt == nil
	var previous *time.Time
	if len(previousAttempt) > 0 {
		previous = previousAttempt[0]
	}
	var previousSeconds *int64
	if previous != nil {
		seconds := previous.Unix()
		previousSeconds = &seconds
	}
	if row.ProviderAcceptedAt != nil {
		state = operationaltrust.NotificationDelivered
	}
	links := row.Links
	if !claimed {
		// Cancellation may have rewritten a grouped row after GetPending. An
		// unclaimed retry must preserve that current membership and its links.
		var err error
		links, err = q.readNotificationLinksNoLock(row.ID)
		if err != nil {
			return err
		}
	}
	if row.ProviderAcceptedAt == nil {
		links = transitionNotificationLinks(links, state, time.Now())
	}
	encoded, err := json.Marshal(links)
	if err != nil {
		return err
	}
	_, err = q.db.Exec(`UPDATE notification_queue SET status='pending',attempts=MAX(0,attempts-?),last_attempt=CASE WHEN ? THEN ? ELSE last_attempt END,last_error=?,next_retry_at=?,completed_at=NULL,operational_links=? WHERE id=? AND (status IN ('pending','sending') OR (? AND status IN ('cancelled','failed','dlq')))`, decrement, decrement, previousSeconds, message, time.Now().Add(30*time.Second).Unix(), string(encoded), row.ID, row.ProviderAcceptedAt != nil)
	if err != nil {
		return fmt.Errorf("defer notification settlement: %w", err)
	}
	if decrement {
		row.Attempts = max(0, row.Attempts-1)
		row.LastAttempt = previous
	}
	return nil
}

func (n *NotificationManager) deliverQueuedNotificationJob(row *QueuedNotification, job notificationDeliveryJob) error {
	if row.ProviderAcceptedAt != nil {
		return nil
	}
	if err := n.deliverNotificationJob(job); err != nil {
		return err
	}
	if q := n.GetQueue(); q != nil {
		return q.acknowledgeProvider(row, time.Now())
	}
	return nil
}
