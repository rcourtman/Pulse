package notifications

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

var errNotificationSettlementPending = errors.New("notification storage settlement pending")
var errNotificationOccurrenceUnavailable = errors.New("alert occurrence authority unavailable")
var errNotificationDispositionChanged = errors.New("alert occurrence disposition changed")

const notificationOccurrencePendingMessage = "Alert occurrence authority is unavailable; delivery remains pending."

const notificationSettlementPendingMessage = "Notification storage is unavailable; delivery settlement remains pending."

func notificationSettlementError(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(errNotificationSettlementPending, err)
}

// Additive migration: severity zero means unknown, not a fabricated historical
// level. A first known severity remains deliverable after migration.
func (q *NotificationQueue) migrateDeliveryReceipts() error {
	columns, err := q.tableColumns("notification_delivery_receipts")
	if err != nil {
		return err
	}
	if _, err := q.db.Exec(`CREATE TABLE IF NOT EXISTS notification_destination_state(destination_key TEXT PRIMARY KEY, epoch TEXT NOT NULL, fingerprint TEXT NOT NULL DEFAULT '',legacy_uncertain INTEGER NOT NULL DEFAULT 0)`); err != nil {
		return err
	}
	stateColumns, err := q.tableColumns("notification_destination_state")
	if err != nil {
		return err
	}
	if !stateColumns["legacy_uncertain"] {
		if _, err := q.db.Exec(`ALTER TABLE notification_destination_state ADD COLUMN legacy_uncertain INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	for _, column := range []string{"severity", "delivered_nanos", "retired_at", "resolved_at", "lifecycle_revision", "success_order", "resolution_order"} {
		if !columns[column] {
			if _, err := q.db.Exec("ALTER TABLE notification_delivery_receipts ADD COLUMN " + column + " INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
		}
	}
	if !columns["delivery_epoch"] {
		tx, err := q.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		// The earlier three-key continuity schema shipped severity and completion
		// precision under these names. Preserve those facts and convert its
		// second-resolution disposition before rebuilding the primary key.
		if columns["highest_severity"] {
			if _, err = tx.Exec(`UPDATE notification_delivery_receipts SET severity=MAX(severity,highest_severity),resolved_at=CASE WHEN resolved_at BETWEEN 1 AND 9223372036 THEN resolved_at*1000000000 ELSE resolved_at END`); err != nil {
				return err
			}
		}
		_, err = tx.Exec(`ALTER TABLE notification_delivery_receipts RENAME TO notification_delivery_receipts_legacy;
		CREATE TABLE notification_delivery_receipts (
		 alert_id TEXT NOT NULL, alert_start_nanos INTEGER NOT NULL, destination_key TEXT NOT NULL,
		 delivered_at INTEGER NOT NULL, delivered_nanos INTEGER NOT NULL DEFAULT 0, severity INTEGER NOT NULL DEFAULT 0, retired_at INTEGER NOT NULL DEFAULT 0,
		 resolved_at INTEGER NOT NULL DEFAULT 0, lifecycle_revision INTEGER NOT NULL DEFAULT 0, success_order INTEGER NOT NULL DEFAULT 0, resolution_order INTEGER NOT NULL DEFAULT 0, delivery_epoch TEXT NOT NULL DEFAULT '',
		 PRIMARY KEY(alert_id, alert_start_nanos, destination_key, delivery_epoch));
		INSERT INTO notification_delivery_receipts SELECT alert_id,alert_start_nanos,destination_key,delivered_at,delivered_nanos,severity,retired_at,resolved_at,lifecycle_revision,success_order,resolution_order,'' FROM notification_delivery_receipts_legacy;
		DROP TABLE notification_delivery_receipts_legacy;
		CREATE INDEX idx_delivery_receipts_delivered_at ON notification_delivery_receipts(delivered_at);
`)
		if err != nil {
			return err
		}
		var retirements int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='notification_receipt_retirements'`).Scan(&retirements); err != nil {
			return err
		}
		if retirements > 0 {
			if _, err = tx.Exec(`UPDATE notification_delivery_receipts SET retired_at=COALESCE((SELECT retired_at FROM notification_receipt_retirements d WHERE d.destination_key=notification_delivery_receipts.destination_key),0); DROP TABLE notification_receipt_retirements`); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(`INSERT OR IGNORE INTO notification_destination_state(destination_key,epoch) SELECT DISTINCT destination_key,delivery_epoch FROM notification_delivery_receipts WHERE retired_at=0`); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	_, err = q.db.Exec(`CREATE INDEX IF NOT EXISTS idx_delivery_receipts_destination ON notification_delivery_receipts(destination_key);`)
	if err == nil {
		// A corrected host clock must not re-arm an impossible future receipt
		// on every process start. Normal successful completion ordering remains
		// monotonic; this repairs invalid wall-clock evidence explicitly.
		now := time.Now()
		_, err = q.db.Exec(`UPDATE notification_delivery_receipts SET delivered_at=?,delivered_nanos=?,lifecycle_revision=lifecycle_revision+1 WHERE delivered_at>? OR delivered_nanos>?`, now.Unix(), now.UnixNano(), now.Unix(), now.UnixNano())
	}
	return err
}

func (q *NotificationQueue) deliveryReceipt(alert *alerts.Alert, destination string) (notificationRecord, bool, error) {
	record, _, found, err := q.deliveryReceiptWithEpoch(alert, destination)
	return record, found, err
}

func (q *NotificationQueue) deliveryReceiptWithEpoch(alert *alerts.Alert, destination string) (notificationRecord, string, bool, error) {
	q.mu.RLock()
	var at, nanos, resolved, successOrder int64
	var rank int
	var epoch string
	err := q.db.QueryRow(`SELECT r.delivered_at,r.delivered_nanos,r.severity,r.delivery_epoch,r.resolved_at,r.success_order FROM notification_delivery_receipts r JOIN notification_destination_state s ON s.destination_key=r.destination_key AND s.epoch=r.delivery_epoch WHERE r.alert_id=? AND r.alert_start_nanos=? AND r.destination_key=? AND r.retired_at=0 LIMIT 1`, alert.ID, alert.StartTime.UnixNano(), destination).Scan(&at, &nanos, &rank, &epoch, &resolved, &successOrder)
	found := err == nil
	if err == sql.ErrNoRows {
		err = nil
	}
	var record notificationRecord
	future := found && (at > time.Now().Unix() || nanos > time.Now().UnixNano())
	if found {
		levels := map[int]alerts.AlertLevel{1: alerts.AlertLevelInfo, 2: alerts.AlertLevelWarning, 3: alerts.AlertLevelCritical}
		record = notificationRecord{successOrder: successOrder, lastSent: cooldownReceiptTime(at), level: levels[rank], alertStart: alert.StartTime, resolved: resolved > 0}
		if nanos > 0 && time.Unix(0, nanos).After(record.lastSent) {
			record.lastSent = time.Unix(0, nanos)
		}
	}
	if err == nil {
		ack, ackEpoch, ackFound, recovered, ackFuture, ackErr := q.acknowledgedReceiptLocked(alert, destination)
		err = ackErr
		future = future || ackFuture
		if recovered && ack.successOrder >= record.successOrder {
			found = false
			record = notificationRecord{}
		} else if ackFound {
			found = true
			epoch = ackEpoch
			if ack.lastSent.After(record.lastSent) {
				record.lastSent = ack.lastSent
			}
			if notificationSeverityRank(ack.level) > notificationSeverityRank(record.level) {
				record.level = ack.level
			}
			record.alertStart = alert.StartTime
			if ack.successOrder > record.successOrder {
				record.resolved = ack.resolved
				record.successOrder = ack.successOrder
			}
		}
	}
	q.mu.RUnlock()
	if err != nil {
		return notificationRecord{}, "", false, err
	}
	if future {
		// Normalize invalid wall-clock evidence once in storage, rather than
		// re-arming a full cooldown on every authoritative read.
		q.mu.Lock()
		now := time.Now()
		_, err = q.db.Exec(`UPDATE notification_delivery_receipts SET delivered_at=?,delivered_nanos=?,lifecycle_revision=lifecycle_revision+1 WHERE alert_id=? AND alert_start_nanos=? AND destination_key=? AND (delivered_at>? OR delivered_nanos>?)`, now.Unix(), now.UnixNano(), alert.ID, alert.StartTime.UnixNano(), destination, now.Unix(), now.UnixNano())
		if err == nil {
			_, err = q.db.Exec(`UPDATE notification_audit SET timestamp=? WHERE success=1 AND timestamp>? AND EXISTS(SELECT 1 FROM notification_queue n WHERE n.id=notification_audit.notification_id AND n.status IN ('pending','sending'))`, now.Unix(), now.Unix())
		}
		if err == nil {
			_, err = q.db.Exec(`UPDATE notification_delivery_facts SET delivered_nanos=? WHERE delivered_nanos>?`, now.UnixNano(), now.UnixNano())
		}
		q.mu.Unlock()
		if err != nil {
			return notificationRecord{}, "", false, err
		}
		return q.deliveryReceiptWithEpoch(alert, destination)
	}
	return record, epoch, found, nil
}

func receiptInCooldown(record notificationRecord, alert *alerts.Alert, cooldown time.Duration) bool {
	rank, incoming := notificationSeverityRank(record.level), notificationSeverityRank(alert.Level)
	if incoming > rank {
		return false
	}
	return !record.resolved && record.alertStart.Equal(alert.StartTime) && (cooldown <= 0 || time.Since(record.lastSent) < cooldown)
}

// Storage is read under the caller's admission lease, without holding the
// manager mutex. SQLite is authoritative for durable delivery; the map is only
// the queue-less fallback. Read failures never establish successful delivery.
func (n *NotificationManager) destinationInCooldown(alert *alerts.Alert, destination string, cooldown time.Duration) bool {
	key := notificationDeliveryReceiptKey(alert, destination)
	n.mu.RLock()
	record, exists := n.deliveryCooldown[key]
	queue := n.queue
	if key == "" {
		record, exists = n.lastNotified[alert.ID]
	}
	n.mu.RUnlock()
	if key == "" {
		return exists && receiptInCooldown(record, alert, cooldown)
	}
	if queue != nil {
		var err error
		record, exists, err = queue.deliveryReceipt(alert, destination)
		if err != nil {
			log.Error().Err(err).Msg("Failed to read notification cooldown evidence")
			return false
		}
	}

	return exists && receiptInCooldown(record, alert, cooldown)
}

func (n *NotificationManager) firingInCooldown(alert *alerts.Alert, options alertSendOptions) (bool, uint64) {
	n.destinationMu.Lock()
	defer n.destinationMu.Unlock()
	n.mu.RLock()
	revision, cooldown := n.destinationRevision, n.cooldown
	jobs := buildNotificationDeliveryJobsForSelection(n.emailConfig, n.webhooks, n.appriseConfig, []*alerts.Alert{alert}, eventAlert, time.Time{}, options.target, options.destinationIDs)
	record, exists := n.lastNotified[alert.ID]
	n.mu.RUnlock()
	if len(jobs) == 0 {
		increased := notificationSeverityRank(record.level) > 0 && notificationSeverityRank(alert.Level) > notificationSeverityRank(record.level)
		return exists && !increased && record.alertStart.Equal(alert.StartTime) && (cooldown <= 0 || time.Since(record.lastSent) < cooldown), revision
	}
	for _, job := range jobs {
		if !n.destinationInCooldown(alert, notificationDeliveryDestinationKey(job), cooldown) {
			return false, revision
		}
	}
	return true, revision
}

func (n *NotificationManager) filterFiringJobsByCooldown(jobs []notificationDeliveryJob) []notificationDeliveryJob {
	n.destinationMu.Lock()
	defer n.destinationMu.Unlock()
	result := make([]notificationDeliveryJob, 0, len(jobs))
	for _, job := range jobs {
		job = n.bindDeliveryEpochsLocked([]notificationDeliveryJob{job})[0]
		n.mu.RLock()
		cooldown := n.cooldown
		n.mu.RUnlock()
		kept := make([]*alerts.Alert, 0, len(job.Alerts))
		for _, alert := range job.Alerts {
			if !n.destinationInCooldown(alert, notificationDeliveryDestinationKey(job), cooldown) {
				kept = append(kept, alert)
			}
		}
		if len(kept) > 0 {
			job.Alerts = kept
			result = append(result, job)
		}
	}
	return result
}

func (n *NotificationManager) cacheDeliveryReceipt(alert *alerts.Alert, destination string, at time.Time, event notificationEvent) {
	n.mu.Lock()
	defer n.mu.Unlock()
	key := notificationDeliveryReceiptKey(alert, destination)
	if n.queue != nil {
		delete(n.deliveryCooldown, key)
		return
	}
	if event == eventResolved {
		delete(n.deliveryCooldown, key)
		return
	}
	if n.deliveryCooldown == nil {
		n.deliveryCooldown = make(map[string]notificationRecord)
	}
	record := n.deliveryCooldown[key]
	if at.After(record.lastSent) {
		record.lastSent = at
	}
	if notificationSeverityRank(alert.Level) > notificationSeverityRank(record.level) {
		record.level = alert.Level
	}
	record.alertStart = alert.StartTime
	n.deliveryCooldown[key] = record
}

// A destination edit is an explicit retirement of the previous identity.
// Old pending recoveries keep their admitted authority, but an exact later
// revert does not resurrect the retired receipt for new recovery admission.
// destinationMu serializes this with binding and other configuration changes.
// Complete retirement before publishing config; queue I/O must never hold the
// manager mutex.
func (n *NotificationManager) retireDestination(old, current string, aliases ...string) error {
	keys := append([]string(nil), aliases...)
	if old != "" && old != current {
		keys = append(keys, old)
	}
	if len(keys) == 0 {
		return nil
	}
	n.mu.RLock()
	queue := n.queue
	admitted := n.lifecycle.join()
	n.mu.RUnlock()
	if !admitted {
		return fmt.Errorf("notification manager stopping")
	}
	defer n.lifecycle.release()
	if queue != nil {
		if err := queue.retireDeliveryDestinations(keys); err != nil {
			return err
		}
	}
	n.mu.Lock()
	n.retireMemoryDestinationsLocked(keys)
	n.destinationRevision++
	n.mu.Unlock()
	return nil
}

func (n *NotificationManager) retireMemoryDestinationsLocked(keys []string) {
	for _, old := range keys {
		for key := range n.deliveryCooldown {
			if strings.HasSuffix(key, "\x00"+old) {
				delete(n.deliveryCooldown, key)
			}
		}
		for key := range n.deliveryReceipts {
			if strings.HasSuffix(key, "\x00"+old) {
				delete(n.deliveryReceipts, key)
			}
		}
	}
}

// Lifecycle knowledge is supplied by the alerts owner. Unknown, unhealthy,
// missing or shutting-down authority never establishes occurrence completion.
func (n *NotificationManager) SetOccurrenceLifecycleProvider(provider func(string, time.Time) alerts.NotificationOccurrenceLifecycle) {
	n.mu.RLock()
	queue := n.queue
	admitted := queue != nil && n.lifecycle.join()
	n.mu.RUnlock()
	if admitted {
		defer n.lifecycle.release()
		queue.mu.Lock()
		queue.occurrenceLifecycle = provider
		queue.mu.Unlock()
	}
}

// Never invoke the alerts owner while holding queue locks. Revalidate the
// receipt under a write lock before deletion, protecting concurrent completions.
func (q *NotificationQueue) cleanupDeliveryReceipts() {
	q.mu.RLock()
	provider := q.occurrenceLifecycle
	rows, err := q.db.Query(`SELECT alert_id, alert_start_nanos, destination_key, delivered_at, retired_at, resolved_at, lifecycle_revision, delivery_epoch FROM notification_delivery_receipts`)
	type receipt struct {
		epoch                                  string
		id                                     string
		start                                  int64
		dest                                   string
		delivered, retired, resolved, revision int64
		observedResolution                     int64
		eligible                               bool
	}
	var receipts []receipt
	if err == nil {
		for rows.Next() {
			var r receipt
			if err = rows.Scan(&r.id, &r.start, &r.dest, &r.delivered, &r.retired, &r.resolved, &r.revision, &r.epoch); err != nil {
				break
			}
			receipts = append(receipts, r)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	q.mu.RUnlock()
	if err != nil {
		log.Error().Err(err).Msg("Failed to inspect notification receipt retention")
		return
	}
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	states := make(map[pendingFiringOccurrence]alerts.NotificationOccurrenceLifecycle)
	var decisions []receipt
	for _, r := range receipts {
		if q.stopping() {
			return
		}
		eligible := r.retired > 0
		if r.retired == 0 && provider != nil {
			occurrence := pendingFiringOccurrence{r.id, time.Unix(0, r.start).UTC()}
			state, found := states[occurrence]
			if !found {
				state = provider(r.id, occurrence.start)
				states[occurrence] = state
			}
			switch state.State {
			case alerts.NotificationOccurrenceResolved:
				if !state.ResolvedAt.IsZero() && state.ResolvedAt.UnixNano() > r.resolved {
					r.observedResolution = state.ResolvedAt.UnixNano()
				}
				eligible = !state.ResolvedAt.IsZero() && state.ResolvedAt.Before(cutoff)
			case alerts.NotificationOccurrenceNoRecord:
				eligible = r.resolved > 0 && time.Unix(0, r.resolved).Before(cutoff) && (state.LatestActivationAt.IsZero() || time.Unix(0, r.resolved).After(state.LatestActivationAt))
			case alerts.NotificationOccurrenceUnavailable:
				// Complete current absence plus the receipt's own positive resolution
				// can settle a completed obligation even when historical gaps persist.
				// Absence alone never supplies a resolution or an expiry timestamp.
				eligible = state.CurrentInactive && r.resolved > 0 && time.Unix(0, r.resolved).Before(cutoff)
			}
		}
		if eligible || r.observedResolution > 0 {
			r.eligible = eligible
			decisions = append(decisions, r)
		}
	}
	if len(decisions) == 0 || q.stopping() {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	pins, err := q.recoveryPinsLocked()
	if err != nil {
		log.Error().Err(err).Msg("Failed to inspect pending recovery authority; retaining receipts")
		return
	}
	tx, err := q.db.Begin()
	if err != nil {
		log.Error().Err(err).Msg("Failed to begin receipt cleanup")
		return
	}
	defer tx.Rollback()
	for _, r := range decisions {
		if q.stopping() {
			return
		}
		if r.observedResolution > 0 {
			result, updateErr := tx.Exec(`UPDATE notification_delivery_receipts SET resolved_at=?,lifecycle_revision=lifecycle_revision+1 WHERE alert_id=? AND alert_start_nanos=? AND destination_key=? AND delivery_epoch=? AND delivered_at=? AND retired_at=? AND resolved_at=? AND lifecycle_revision=?`, r.observedResolution, r.id, r.start, r.dest, r.epoch, r.delivered, r.retired, r.resolved, r.revision)
			if updateErr != nil {
				log.Error().Err(updateErr).Msg("Failed to retain observed occurrence resolution")
				return
			}
			affected, updateErr := result.RowsAffected()
			if updateErr != nil || affected == 0 {
				continue
			}
			resolvedAt := time.Unix(0, r.observedResolution)
			if err := updateAcknowledgedFirings(tx, r.id, time.Unix(0, r.start), r.dest, r.epoch, &resolvedAt, false); err != nil {
				log.Error().Err(err).Msg("Failed to retain accepted occurrence resolution")
				return
			}
			r.resolved = r.observedResolution
			r.revision++
		}
		if !r.eligible {
			continue
		}
		key := notificationReceiptPin{r.id, r.start, r.dest, r.epoch}
		if pins.exact[key] || pins.destinations[notificationReceiptPin{dest: r.dest, epoch: r.epoch}] || pins.destinations[notificationReceiptPin{dest: r.dest, epoch: "*"}] || pins.types[strings.SplitN(r.dest, ":", 2)[0]] {
			continue
		}
		if _, err = tx.Exec(`DELETE FROM notification_delivery_receipts WHERE alert_id=? AND alert_start_nanos=? AND destination_key=? AND delivered_at=? AND retired_at=? AND resolved_at=? AND lifecycle_revision=? AND delivery_epoch=?`, r.id, r.start, r.dest, r.delivered, r.retired, r.resolved, r.revision, r.epoch); err != nil {
			log.Error().Err(err).Msg("Failed to settle receipt retention")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		log.Error().Err(err).Msg("Failed to commit receipt cleanup")
	}
}

type notificationReceiptPin struct {
	id          string
	start       int64
	dest, epoch string
}
type notificationRecoveryPins struct {
	exact        map[notificationReceiptPin]bool
	destinations map[notificationReceiptPin]bool
	types        map[string]bool
}

// Build once per pass. Unreadable rows conservatively pin their identifiable
// destination or provider type, rather than poisoning unrelated providers or
// pretending corrupt pending recovery data contains no obligations.
func (q *NotificationQueue) recoveryPinsLocked() (notificationRecoveryPins, error) {
	pins := notificationRecoveryPins{make(map[notificationReceiptPin]bool), make(map[notificationReceiptPin]bool), make(map[string]bool)}
	rows, err := q.db.Query(`SELECT type,alerts,config,receipt_epoch FROM notification_queue WHERE type LIKE '%_resolved' AND status IN ('pending','sending','dlq','failed')`)
	if err != nil {
		return pins, err
	}
	defer rows.Close()
	for rows.Next() {
		if q.stopping() {
			return pins, fmt.Errorf("notification queue stopping")
		}
		var kind string
		var payload, config []byte
		var epoch *string
		if err := rows.Scan(&kind, &payload, &config, &epoch); err != nil {
			return pins, err
		}
		job := notificationDeliveryJob{Event: eventResolved}
		job.Type, _ = normalizeQueueType(kind)
		switch job.Type {
		case "webhook":
			job.WebhookConfig = &WebhookConfig{}
			err = json.Unmarshal(config, job.WebhookConfig)
		case "email":
			job.EmailConfig = &EmailConfig{}
			err = json.Unmarshal(config, job.EmailConfig)
		case "apprise":
			job.AppriseConfig = &AppriseConfig{}
			err = json.Unmarshal(config, job.AppriseConfig)
		default:
			log.Warn().Msg("Unknown recovery provider retained for operator disposition")
			continue
		}
		if err != nil {
			pins.types[job.Type] = true
			log.Error().Msg("Unreadable recovery configuration pins provider receipts")
			continue
		}
		destination := notificationDeliveryDestinationKey(job)
		if destination == "" {
			pins.types[job.Type] = true
			continue
		}
		pinDestination := func(generation string) {
			pins.destinations[notificationReceiptPin{dest: destination, epoch: generation}] = true
			if job.WebhookConfig != nil {
				legacy, _ := legacyWebhookDestination(*job.WebhookConfig)
				pins.destinations[notificationReceiptPin{dest: legacy, epoch: generation}] = true
			}
		}
		if epoch == nil {
			pinDestination("*")
			continue
		}
		if validateReceiptEpoch(*epoch) != nil {
			pins.types[job.Type] = true
			continue
		}
		var snapshots []*alerts.Alert
		if err = json.Unmarshal(payload, &snapshots); err != nil {
			pinDestination(*epoch)
			log.Error().Msg("Unreadable recovery payload pins destination receipts")
			continue
		}
		completeIdentity := len(snapshots) > 0
		for _, alert := range snapshots {
			if alert != nil && alert.ID != "" && !alert.StartTime.IsZero() {
				pins.exact[notificationReceiptPin{alert.ID, alert.StartTime.UnixNano(), destination, *epoch}] = true
				if job.WebhookConfig != nil {
					legacy, _ := legacyWebhookDestination(*job.WebhookConfig)
					pins.exact[notificationReceiptPin{alert.ID, alert.StartTime.UnixNano(), legacy, *epoch}] = true
				}
			} else {
				completeIdentity = false
			}
		}
		if !completeIdentity {
			pinDestination(*epoch)
		}
	}
	return pins, rows.Err()
}

func (q *NotificationQueue) retireDeliveryDestination(destination string) error {
	return q.retireDeliveryDestinations([]string{destination})
}

func (q *NotificationQueue) retireDeliveryDestinations(destinations []string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	tx, err := q.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, destination := range destinations {
		if _, err = tx.Exec(`UPDATE notification_delivery_receipts SET retired_at=?,lifecycle_revision=lifecycle_revision+1 WHERE destination_key=? AND retired_at=0`, time.Now().Unix(), destination); err != nil {
			return err
		}
		if _, err = tx.Exec(`DELETE FROM notification_destination_state WHERE destination_key=?`, destination); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func webhookInventoryIdentity(hook WebhookConfig) string {
	if id := strings.TrimSpace(hook.ID); id != "" {
		return "id:" + id
	}
	// Legacy ID-less entries remain correlated to their existing endpoint key.
	// Assigning new IDs here would retire their pre-upgrade delivery authority.
	job := notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook}
	return "legacy:" + notificationDeliveryDestinationKey(job) + ":" + notificationDeliveryConfigurationFingerprint(job)
}

// ValidateWebhookInventory applies the same identity rule at every inventory
// creation and archive publication boundary. Readable legacy startup inventory
// is not rejected by a newly introduced validation rule.
func ValidateWebhookInventory(webhooks []WebhookConfig) error {
	seen := make(map[string]bool, len(webhooks))
	for _, hook := range webhooks {
		if strings.TrimSpace(hook.ID) == "" {
			continue
		}
		identity := webhookInventoryIdentity(hook)
		if seen[identity] {
			return fmt.Errorf("duplicate webhook identity in destination inventory")
		}
		seen[identity] = true
	}
	return nil
}

// ReplaceWebhooks applies a restored inventory as one recipient transition.
// Identical entries retain their generations; a failed retirement publishes
// none of the proposed inventory and cannot create duplicate logical IDs.
func (n *NotificationManager) ReplaceWebhooks(webhooks []WebhookConfig) error {
	if err := n.ConfigurationError(); err != nil {
		return err
	}
	n.destinationMu.Lock()
	defer n.destinationMu.Unlock()
	if err := ValidateWebhookInventory(webhooks); err != nil {
		return err
	}
	prepared := make([]WebhookConfig, 0, len(webhooks))
	byID := make(map[string]notificationDeliveryJob, len(webhooks))
	for _, hook := range webhooks {
		hook = copyWebhookConfig(NormalizeWebhookConfig(hook))
		prepared = append(prepared, hook)
		copy := hook
		byID[webhookInventoryIdentity(hook)] = notificationDeliveryJob{Type: "webhook", WebhookConfig: &copy}
	}
	n.mu.RLock()
	previous := copyWebhookConfigs(n.webhooks)
	queue := n.queue
	admitted := n.lifecycle.join()
	n.mu.RUnlock()
	if !admitted {
		return fmt.Errorf("notification manager stopping")
	}
	defer n.lifecycle.release()
	var retired []string
	currentLegacy := make(map[string]bool)
	for _, hook := range prepared {
		if hook.Enabled {
			key, _ := legacyWebhookDestination(hook)
			currentLegacy[key] = true
		}
	}
	for _, hook := range previous {
		old := notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook}
		key := configuredDeliveryDestinationKey(old)
		if key == "" {
			continue
		}
		current, exists := byID[webhookInventoryIdentity(hook)]
		if !exists || configuredDeliveryDestinationKey(current) != key || notificationDeliveryConfigurationFingerprint(current) != notificationDeliveryConfigurationFingerprint(old) {
			retired = append(retired, key)
		}
		legacy, _ := legacyWebhookDestination(hook)
		if !currentLegacy[legacy] {
			retired = append(retired, legacy)
		}
	}
	if queue != nil && len(retired) > 0 {
		if err := queue.retireDeliveryDestinations(retired); err != nil {
			return err
		}
	}
	n.mu.Lock()
	n.webhooks = prepared
	n.destinationRevision++
	n.retireMemoryDestinationsLocked(retired)
	n.mu.Unlock()
	return nil
}

// Older receipts identify only the logical ID and endpoint. A unique receiver
// can inherit that evidence once. Several accounts sharing the endpoint remain
// explicitly uncertain; removing one later must not make old evidence precise.
func (q *NotificationQueue) migrateLegacyWebhookDestinations(hooks []WebhookConfig) (map[string]bool, error) {
	type endpoint struct {
		identity  string
		receivers map[string]string
	}
	endpoints := make(map[string]*endpoint)
	for _, hook := range hooks {
		if !hook.Enabled {
			continue
		}
		job := notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook}
		old, identity := legacyWebhookDestination(hook)
		item := endpoints[old]
		if item == nil {
			item = &endpoint{identity: identity, receivers: make(map[string]string)}
			endpoints[old] = item
		}
		item.receivers[notificationDeliveryDestinationKey(job)] = notificationDeliveryConfigurationFingerprint(job)
	}
	held := make(map[string]bool)
	if len(endpoints) == 0 {
		return held, nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	tx, err := q.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Retained provider snapshots can contradict the current receiver even
	// when only one is configured now. They are historical evidence, not absent
	// configuration. Read them once, before assigning any legacy authority.
	type legacyRecovery struct {
		key   string
		id    string
		start int64
	}
	evidence := make(map[string]map[string]bool)
	recoveries := make(map[string][]legacyRecovery)
	unreadableSnapshot := false
	rows, err := tx.Query(`SELECT type,status,config,alerts,receipt_epoch FROM notification_queue WHERE type IN ('webhook','webhook_resolved') AND (receipt_epoch IS NULL OR receipt_epoch='')`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kind, status string
		var rawConfig, rawAlerts []byte
		var bound sql.NullString
		if err := rows.Scan(&kind, &status, &rawConfig, &rawAlerts, &bound); err != nil {
			rows.Close()
			return nil, err
		}
		var hook WebhookConfig
		if json.Unmarshal(rawConfig, &hook) != nil {
			unreadableSnapshot = true
			continue
		}
		if strings.TrimSpace(hook.URL) == "" {
			unreadableSnapshot = true
			continue
		}
		old, _ := legacyWebhookDestination(hook)
		if endpoints[old] == nil {
			continue
		}
		key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
		if evidence[old] == nil {
			evidence[old] = make(map[string]bool)
		}
		evidence[old][key] = true
		if kind != "webhook_resolved" || (bound.Valid && bound.String != "") || (status != "pending" && status != "sending" && status != "failed" && status != "dlq") {
			continue
		}
		var list []*alerts.Alert
		if json.Unmarshal(rawAlerts, &list) != nil {
			continue
		}
		for _, a := range list {
			if a != nil && a.ID != "" && !a.StartTime.IsZero() {
				recoveries[old] = append(recoveries[old], legacyRecovery{key, a.ID, a.StartTime.UnixNano()})
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for old, item := range endpoints {
		var epoch, fingerprint string
		var uncertain bool
		stateErr := tx.QueryRow(`SELECT epoch,fingerprint,legacy_uncertain FROM notification_destination_state WHERE destination_key=?`, old).Scan(&epoch, &fingerprint, &uncertain)
		if stateErr != nil && stateErr != sql.ErrNoRows {
			return nil, stateErr
		}
		var receipts int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts WHERE destination_key=?`, old).Scan(&receipts); err != nil {
			return nil, err
		}
		if stateErr == sql.ErrNoRows && receipts == 0 {
			continue
		}
		contradictory := unreadableSnapshot
		for historical := range evidence[old] {
			if _, current := item.receivers[historical]; !current {
				contradictory = true
			}
		}
		if uncertain || (fingerprint == "" && (len(item.receivers) > 1 || contradictory)) {
			held[old] = true
			// Preserve exact already-admitted legacy recoveries as retired copies at
			// their immutable receiver. They cannot authorise new recoveries, but pins
			// and completion now refer to the same exact key. Remaining shared evidence
			// stays explicitly uncertain rather than being donated to today's receiver.
			for _, recovery := range recoveries[old] {
				_, err = tx.Exec(`INSERT INTO notification_delivery_receipts(alert_id,alert_start_nanos,destination_key,delivered_at,delivered_nanos,severity,retired_at,resolved_at,lifecycle_revision,delivery_epoch,success_order,resolution_order)
 SELECT alert_id,alert_start_nanos,?,delivered_at,delivered_nanos,severity,MAX(retired_at,?),resolved_at,lifecycle_revision,delivery_epoch,success_order,resolution_order FROM notification_delivery_receipts WHERE destination_key=? AND alert_id=? AND alert_start_nanos=? AND delivery_epoch=''
 ON CONFLICT(alert_id,alert_start_nanos,destination_key,delivery_epoch) DO UPDATE SET retired_at=MAX(retired_at,excluded.retired_at),delivered_at=MAX(delivered_at,excluded.delivered_at),delivered_nanos=MAX(delivered_nanos,excluded.delivered_nanos),severity=MAX(severity,excluded.severity),resolved_at=MAX(resolved_at,excluded.resolved_at),success_order=MAX(success_order,excluded.success_order),resolution_order=MAX(resolution_order,excluded.resolution_order),lifecycle_revision=lifecycle_revision+1`, recovery.key, time.Now().Unix(), old, recovery.id, recovery.start)
				if err != nil {
					return nil, err
				}
			}
			if stateErr == nil {
				if _, err := tx.Exec(`UPDATE notification_destination_state SET legacy_uncertain=1 WHERE destination_key=?`, old); err != nil {
					return nil, err
				}
			}
			continue
		}
		current := ""
		if fingerprint != "" {
			current = hashDeliveryDestination("webhook", item.identity+"\x1f"+fingerprint)
		} else {
			for key := range item.receivers {
				current = key
			}
		}
		var currentEpoch string
		currentErr := tx.QueryRow(`SELECT epoch FROM notification_destination_state WHERE destination_key=?`, current).Scan(&currentEpoch)
		if currentErr != nil && currentErr != sql.ErrNoRows {
			return nil, currentErr
		}
		if currentErr == sql.ErrNoRows && stateErr == nil {
			if _, err := tx.Exec(`INSERT INTO notification_destination_state(destination_key,epoch,fingerprint) VALUES(?,?,?)`, current, epoch, fingerprint); err != nil {
				return nil, err
			}
			currentEpoch = epoch
			currentErr = nil
		}
		_, err = tx.Exec(`INSERT INTO notification_delivery_receipts(alert_id,alert_start_nanos,destination_key,delivered_at,delivered_nanos,severity,retired_at,resolved_at,lifecycle_revision,delivery_epoch,success_order,resolution_order)
   SELECT alert_id,alert_start_nanos,?,delivered_at,delivered_nanos,severity,retired_at,resolved_at,lifecycle_revision,delivery_epoch,success_order,resolution_order FROM notification_delivery_receipts WHERE destination_key=? AND (delivery_epoch=? OR ?)
   ON CONFLICT(alert_id,alert_start_nanos,destination_key,delivery_epoch) DO UPDATE SET delivered_at=MAX(delivered_at,excluded.delivered_at),delivered_nanos=MAX(delivered_nanos,excluded.delivered_nanos),severity=MAX(severity,excluded.severity),retired_at=MAX(retired_at,excluded.retired_at),resolved_at=CASE WHEN excluded.success_order>success_order THEN excluded.resolved_at ELSE resolved_at END,success_order=MAX(success_order,excluded.success_order),resolution_order=MAX(resolution_order,excluded.resolution_order),lifecycle_revision=MAX(lifecycle_revision,excluded.lifecycle_revision)+1`, current, old, epoch, stateErr == sql.ErrNoRows)
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(`DELETE FROM notification_delivery_receipts WHERE destination_key=? AND (delivery_epoch=? OR ?)`, old, epoch, stateErr == sql.ErrNoRows); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(`UPDATE notification_delivery_facts SET destination_key=? WHERE destination_key=? AND delivery_epoch=?`, current, old, epoch); err != nil {
			return nil, err
		}
		if currentErr == nil {
			if _, err = tx.Exec(`UPDATE notification_delivery_receipts SET retired_at=?,lifecycle_revision=lifecycle_revision+1 WHERE destination_key=? AND delivery_epoch<>? AND retired_at=0`, time.Now().Unix(), current, currentEpoch); err != nil {
				return nil, err
			}
		}
		if _, err = tx.Exec(`DELETE FROM notification_destination_state WHERE destination_key=?`, old); err != nil {
			return nil, err
		}
	}
	return held, tx.Commit()
}

// Bootstrap calls this only after the complete saved destination set is loaded.
// Global delivery pause is not retirement. Disabled/removed destinations are.
func (n *NotificationManager) reconcileDeliveryDestinations() error {
	n.mu.RLock()
	q := n.queue
	keys := n.currentDeliveryFingerprintsLocked()
	webhooks := copyWebhookConfigs(n.webhooks)
	n.mu.RUnlock()
	if q == nil || !n.lifecycle.join() {
		return nil
	}
	defer n.lifecycle.release()
	legacyHeld, err := q.migrateLegacyWebhookDestinations(webhooks)
	if err != nil {
		return err
	}
	for key := range legacyHeld {
		keys[key] = ""
	}
	q.mu.Lock()
	rows, err := q.db.Query(`SELECT destination_key FROM notification_destination_state UNION SELECT destination_key FROM notification_delivery_receipts WHERE retired_at=0`)
	var retired []string
	if err == nil {
		for rows.Next() {
			var key string
			if err = rows.Scan(&key); err != nil {
				break
			}
			if _, active := keys[key]; !active {
				retired = append(retired, key)
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	q.mu.Unlock()
	if err != nil {
		return err
	}
	for _, key := range retired {
		if err = q.retireDeliveryDestination(key); err != nil {
			return err
		}
	}
	for key, fingerprint := range keys {
		if legacyHeld[key] {
			continue
		}
		if _, err = q.activateDeliveryDestination(key, fingerprint); err != nil {
			return err
		}
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for cacheKey := range n.deliveryCooldown {
		for _, key := range retired {
			if strings.HasSuffix(cacheKey, "\x00"+key) {
				delete(n.deliveryCooldown, cacheKey)
			}
		}
	}
	return nil
}

func configuredDeliveryDestinationKey(job notificationDeliveryJob) string {
	switch job.Type {
	case "webhook":
		if job.WebhookConfig == nil || !job.WebhookConfig.Enabled {
			return ""
		}
	case "email":
		if job.EmailConfig == nil || !job.EmailConfig.Enabled {
			return ""
		}
	case "apprise":
		if job.AppriseConfig == nil || !job.AppriseConfig.Enabled {
			return ""
		}
	}
	return notificationDeliveryDestinationKey(job)
}

// This is the disposition of the receipt obligation, supplied by an explicit
// resolution callback. It survives history compaction and does not infer an end
// from absence. A later firing admission invalidates stale cleanup snapshots;
// only a newer successful firing replaces this positive disposition.
func (q *NotificationQueue) markReceiptOccurrenceResolved(id string, start time.Time, at time.Time) error {
	if id == "" || start.IsZero() {
		return nil
	}
	release := q.acquireAlertDeliveryGates([]string{id}, true)
	defer release()
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.markReceiptOccurrenceResolvedLocked(id, start, at)
}

func (q *NotificationQueue) markReceiptOccurrenceResolvedLocked(id string, start time.Time, at time.Time) error {
	var exists bool
	if err := q.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM notification_delivery_receipts WHERE alert_id=? AND alert_start_nanos=? UNION ALL SELECT 1 FROM notification_delivery_facts WHERE alert_id=? AND alert_start_nanos=?)`, id, start.UnixNano(), id, start.UnixNano()).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	}
	// Resolution order is independent of wall-clock corrections. Reserve from
	// the existing durable outcome counter without inventing a provider audit.
	order, err := q.receiptSuccessOrderLocked("")
	if err != nil {
		return err
	}
	tx, err := q.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE notification_delivery_receipts SET resolved_at=MAX(resolved_at,?),resolution_order=MAX(resolution_order,?),lifecycle_revision=lifecycle_revision+1 WHERE alert_id=? AND alert_start_nanos=?`, at.UnixNano(), order, id, start.UnixNano()); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE notification_delivery_facts SET resolved_nanos=MAX(resolved_nanos,?),resolution_order=MAX(resolution_order,?) WHERE alert_id=? AND alert_start_nanos=? AND event=?`, at.UnixNano(), order, id, start.UnixNano(), string(eventAlert)); err != nil {
		return err
	}
	return tx.Commit()
}

func (q *NotificationQueue) receiptResolutionLocked(id string, start time.Time) (int64, int64, error) {
	var at, order int64
	err := q.db.QueryRow(`SELECT COALESCE(MAX(at),0),COALESCE(MAX(sequence),0) FROM (SELECT resolved_at AS at,resolution_order AS sequence FROM notification_delivery_receipts WHERE alert_id=? AND alert_start_nanos=? UNION ALL SELECT resolved_nanos AS at,resolution_order AS sequence FROM notification_delivery_facts WHERE alert_id=? AND alert_start_nanos=?)`, id, start.UnixNano(), id, start.UnixNano()).Scan(&at, &order)
	return at, order, err
}

func (q *NotificationQueue) reopenReceiptOccurrence(id string, start time.Time, rowIDs ...string) error {
	_, err := q.reopenReceiptOccurrenceWithToken(id, start, false, rowIDs...)
	return err
}

func (q *NotificationQueue) reopenReceiptOccurrenceWithToken(id string, start time.Time, processingRecovery bool, rowIDs ...string) (int64, error) {
	if id == "" || start.IsZero() {
		return 0, nil
	}
	q.mu.Lock()
	provider := q.occurrenceLifecycle
	resolved, order, err := q.receiptResolutionLocked(id, start)
	if err == nil {
		_, err = q.db.Exec(`UPDATE notification_delivery_receipts SET lifecycle_revision=lifecycle_revision+1 WHERE alert_id=? AND alert_start_nanos=?`, id, start.UnixNano())
	}
	q.mu.Unlock()
	if err != nil {
		return 0, err
	}
	// Without retained resolution evidence this is an ordinary firing, including
	// findings owned outside alerts.Manager. Absence from that owner's active
	// inventory cannot cancel another producer's newly admitted notification.
	if resolved <= 0 && !processingRecovery {
		return order, nil
	}
	state := alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceUnavailable}
	if provider != nil {
		state = provider(id, start)
	}
	if state.State == alerts.NotificationOccurrenceResolved || state.CurrentInactive {
		return order, ErrNotificationDeliverySkipped
	}
	if resolved <= 0 && state.State != alerts.NotificationOccurrenceActive {
		return order, nil
	}
	if state.State != alerts.NotificationOccurrenceActive && !activationAfterRetainedResolution(state.LatestActivationAt, time.Unix(0, resolved)) {
		return order, errNotificationOccurrenceUnavailable
	}
	guard := func() (bool, error) {
		currentAt, currentOrder, err := q.receiptResolutionLocked(id, start)
		if err != nil {
			return false, err
		}
		if currentOrder > order || (currentOrder == order && currentAt > resolved) {
			return false, errNotificationDispositionChanged
		}
		if len(rowIDs) > 0 {
			var status string
			err = q.db.QueryRow(`SELECT status FROM notification_queue WHERE id=?`, rowIDs[0]).Scan(&status)
			if err == sql.ErrNoRows {
				return false, nil
			}
			return status == string(QueueStatusPending), err
		}
		return true, nil
	}
	// The monotonic disposition guard bounds later resolutions. Snapshot wall
	// clocks are not an ordering token: callers and clock steps can put the
	// recovery timestamp after the local cancellation callback's timestamp.
	_, err = q.cancelOccurrenceNotificationsGuarded([]string{id}, func(alert *alerts.Alert) bool {
		return alert.ID == id && alert.StartTime.Equal(start)
	}, eventResolved, guard, func() error {
		_, err := q.db.Exec(`UPDATE notification_delivery_receipts SET lifecycle_revision=lifecycle_revision+1 WHERE alert_id=? AND alert_start_nanos=? AND resolved_at>0`, id, start.UnixNano())
		return err
	})
	return order, err
}

// The registry retains only current destinations. Fresh UUID generations make
// deletion/recreation safe without an unbounded retired-destination catalogue.
func (q *NotificationQueue) activateDeliveryDestination(key, fingerprint string) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	tx, err := q.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var epoch, previous string
	err = tx.QueryRow(`SELECT epoch,fingerprint FROM notification_destination_state WHERE destination_key=?`, key).Scan(&epoch, &previous)
	if err == nil && previous == fingerprint {
		return epoch, tx.Commit()
	}
	if err == nil && previous != "" && previous != fingerprint {
		if _, err = tx.Exec(`UPDATE notification_delivery_receipts SET retired_at=?,lifecycle_revision=lifecycle_revision+1 WHERE destination_key=? AND retired_at=0`, time.Now().Unix(), key); err != nil {
			return "", err
		}
		epoch = uuid.NewString()
	} else if err == sql.ErrNoRows {
		if _, err = tx.Exec(`UPDATE notification_delivery_receipts SET retired_at=?,lifecycle_revision=lifecycle_revision+1 WHERE destination_key=? AND retired_at=0`, time.Now().Unix(), key); err != nil {
			return "", err
		}
		epoch = uuid.NewString()
	} else if err != nil {
		return "", err
	}
	_, err = tx.Exec(`INSERT INTO notification_destination_state(destination_key,epoch,fingerprint) VALUES(?,?,?) ON CONFLICT(destination_key) DO UPDATE SET epoch=excluded.epoch,fingerprint=excluded.fingerprint`, key, epoch, fingerprint)
	if err != nil {
		return "", err
	}
	return epoch, tx.Commit()
}

func (n *NotificationManager) bindDeliveryEpochs(jobs []notificationDeliveryJob) []notificationDeliveryJob {
	n.destinationMu.Lock()
	defer n.destinationMu.Unlock()
	return n.bindDeliveryEpochsLocked(jobs)
}

func (n *NotificationManager) bindDeliveryEpochsLocked(jobs []notificationDeliveryJob) []notificationDeliveryJob {
	n.mu.RLock()
	queue := n.queue
	keys := n.currentDeliveryFingerprintsLocked()
	n.mu.RUnlock()
	for i := range jobs {
		if jobs[i].ReceiptEpochBound {
			continue
		}
		if queue != nil {
			key := notificationDeliveryDestinationKey(jobs[i])
			if current, active := keys[key]; !active || current != notificationDeliveryConfigurationFingerprint(jobs[i]) {
				jobs[i].ReceiptEpoch = uuid.NewString()
			} else {
				epoch, err := queue.activateDeliveryDestination(key, notificationDeliveryConfigurationFingerprint(jobs[i]))
				if err != nil {
					log.Error().Err(err).Msg("Failed to bind notification destination generation")
					// Keep the queued obligation unbound. The processor retries the
					// registry write before contacting the provider; a fabricated
					// generation would falsely settle it as a retired delivery.
					continue
				}
				jobs[i].ReceiptEpoch = epoch
			}
		}
		jobs[i].ReceiptEpochBound = true
	}
	return jobs
}

func deliveryJobEpoch(job notificationDeliveryJob) *string {
	if !job.ReceiptEpochBound {
		return nil
	}
	epoch := job.ReceiptEpoch
	return &epoch
}

func queuedReceiptEpoch(row *QueuedNotification) string {
	if row.ReceiptEpoch == nil {
		return ""
	}
	return *row.ReceiptEpoch
}

// Raw queue callers can admit recovery without going through the manager's
// receipt filter. Capture its generation at enqueue, never at later replay.
// Ambiguous batches must be split by the caller rather than consume a newer
// generation on an old recovery's completion.
func (q *NotificationQueue) bindQueuedRecoveryEpochLocked(row *QueuedNotification) error {
	if row.ReceiptEpoch != nil {
		return nil
	}
	kind, event := normalizeQueueType(row.Type)
	if event != eventResolved {
		return nil
	}
	job := notificationDeliveryJob{Type: kind}
	switch kind {
	case "email":
		job.EmailConfig = &EmailConfig{}
		if json.Unmarshal(row.Config, job.EmailConfig) != nil {
			return nil
		}
	case "webhook":
		job.WebhookConfig = &WebhookConfig{}
		if json.Unmarshal(row.Config, job.WebhookConfig) != nil {
			return nil
		}
	case "apprise":
		job.AppriseConfig = &AppriseConfig{}
		if json.Unmarshal(row.Config, job.AppriseConfig) != nil {
			return nil
		}
	default:
		return nil
	}
	key := notificationDeliveryDestinationKey(job)
	epoch, found := "", false
	for _, alert := range row.Alerts {
		if alert == nil || alert.StartTime.IsZero() {
			continue
		}
		var current string
		err := q.db.QueryRow(`SELECT delivery_epoch FROM notification_delivery_receipts WHERE alert_id=? AND alert_start_nanos=? AND destination_key=? AND retired_at=0 LIMIT 1`, alert.ID, alert.StartTime.UnixNano(), key).Scan(&current)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return fmt.Errorf("bind queued recovery generation: %w", err)
		}
		if found && current != epoch {
			return fmt.Errorf("recovery batch contains multiple destination generations")
		}
		epoch, found = current, true
	}
	row.ReceiptEpoch = &epoch
	return nil
}

// Existing recovery work was admitted against legacy generation zero. Firings
// bind before their first post-migration attempt, without changing provider bytes.
func (q *NotificationQueue) migrateQueueReceiptEpoch() error {
	columns, err := q.tableColumns("notification_queue")
	if err != nil {
		return err
	}
	if columns["receipt_epoch"] {
		return nil
	}
	tx, err := q.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`ALTER TABLE notification_queue ADD COLUMN receipt_epoch TEXT;
 UPDATE notification_queue SET receipt_epoch='' WHERE type IN ('email_resolved','webhook_resolved','apprise_resolved');`); err != nil {
		return err
	}
	return tx.Commit()
}

func (q *NotificationQueue) deleteDeliveryReceiptEpoch(id string, start time.Time, destination, epoch string, queueIDs ...string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	queueID := ""
	if len(queueIDs) > 0 {
		queueID = queueIDs[0]
	}
	order, err := q.receiptSuccessOrderLocked(queueID)
	if err != nil {
		return err
	}
	tx, err := q.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := updateAcknowledgedFirings(tx, id, start, destination, epoch, nil, true, order); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM notification_delivery_receipts WHERE alert_id=? AND alert_start_nanos=? AND destination_key=? AND delivery_epoch=? AND success_order<=?`, id, start.UnixNano(), destination, epoch, order); err != nil {
		return err
	}
	return tx.Commit()
}

func cooldownReceiptTime(seconds int64) time.Time {
	now := time.Now()
	if seconds > now.Unix() {
		return now
	}
	return time.Unix(seconds, 0)
}

func notificationDeliveryConfigurationFingerprint(job notificationDeliveryJob) string {
	var value any
	switch job.Type {
	case "webhook":
		if job.WebhookConfig == nil {
			return ""
		}
		c := NormalizeWebhookConfig(copyWebhookConfig(*job.WebhookConfig))
		c.Name = ""
		c.ID = strings.TrimSpace(c.ID)
		c.URL = strings.TrimSpace(c.URL)
		c.Service = strings.ToLower(strings.TrimSpace(c.Service))
		c.Method = strings.ToUpper(strings.TrimSpace(c.Method))
		if c.Method == "" {
			c.Method = "POST"
		}
		c.Enabled = false
		c.TagFilter = nil
		c.TagMode = ""
		c.MinimumSeverity = ""
		c.Mention = ""
		c.SigningSecret = ""
		headers := make(map[string]string, len(c.Headers))
		headerKeys := make([]string, 0, len(c.Headers))
		for key := range c.Headers {
			headerKeys = append(headerKeys, key)
		}
		sort.Strings(headerKeys)
		for _, key := range headerKeys {
			headers[strings.ToLower(key)] = c.Headers[key]
		}
		c.Headers = headers
		value = c
	case "email":
		if job.EmailConfig == nil {
			return ""
		}
		c := copyEmailConfig(*job.EmailConfig)
		c.Enabled = false
		for i := range c.To {
			c.To[i] = strings.ToLower(strings.TrimSpace(c.To[i]))
		}
		sort.Strings(c.To)
		c.Password = ""
		c.Provider = ""
		c.SMTPHost = strings.ToLower(c.SMTPHost)
		c.TLS = false
		c.StartTLS = false
		c.RateLimit = 0
		c.TagFilter = nil
		c.TagMode = ""
		c.MinimumSeverity = ""
		value = c
	case "apprise":
		if job.AppriseConfig == nil {
			return ""
		}
		c := copyAppriseConfig(*job.AppriseConfig)
		c.Enabled = false
		for i := range c.Targets {
			c.Targets[i] = strings.TrimSpace(c.Targets[i])
		}
		sort.Strings(c.Targets)
		c.APIKeyHeader = strings.ToLower(strings.TrimSpace(c.APIKeyHeader))
		c.TimeoutSeconds = 0
		c.SkipTLSVerify = false
		c.MinimumSeverity = ""
		value = c
	default:
		return ""
	}
	payload, _ := json.Marshal(value)
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func (n *NotificationManager) currentDeliveryFingerprintsLocked() map[string]string {
	result := make(map[string]string)
	jobs := []notificationDeliveryJob{{Type: "email", EmailConfig: &n.emailConfig}, {Type: "apprise", AppriseConfig: &n.appriseConfig}}
	for i := range n.webhooks {
		jobs = append(jobs, notificationDeliveryJob{Type: "webhook", WebhookConfig: &n.webhooks[i]})
	}
	for _, job := range jobs {
		if key := configuredDeliveryDestinationKey(job); key != "" {
			result[key] = notificationDeliveryConfigurationFingerprint(job)
		}
	}
	return result
}

// Legacy pending firings adopt their first post-migration generation before
// attempting a provider. Persisting the binding prevents retries and crash
// replay from adopting a later revert. Legacy recoveries retain epoch zero.
func (n *NotificationManager) prepareQueuedReceiptEpoch(row *QueuedNotification, job notificationDeliveryJob) (notificationDeliveryJob, error) {
	if job.ReceiptEpochBound {
		return job, nil
	}
	if job.Event == eventResolved {
		job.ReceiptEpochBound = true
	} else {
		job = n.bindDeliveryEpochs([]notificationDeliveryJob{job})[0]
		if !job.ReceiptEpochBound {
			return job, notificationSettlementError(fmt.Errorf("notification destination generation unavailable"))
		}
	}
	queue := n.GetQueue()
	if queue != nil {
		queue.mu.Lock()
		_, err := queue.db.Exec(`UPDATE notification_queue SET receipt_epoch=? WHERE id=?`, job.ReceiptEpoch, row.ID)
		queue.mu.Unlock()
		if err != nil {
			return job, notificationSettlementError(fmt.Errorf("persist notification destination generation: %w", err))
		}
	}
	row.ReceiptEpoch = deliveryJobEpoch(job)
	return job, nil
}

func validateReceiptEpoch(epoch string) error {
	if epoch != "" {
		parsed, err := uuid.Parse(epoch)
		if err != nil {
			return err
		}
		if parsed.String() != epoch {
			return fmt.Errorf("noncanonical notification destination generation")
		}
	}
	return nil
}
