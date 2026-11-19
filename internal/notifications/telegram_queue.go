package notifications

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

const telegramRateHoldMessage = "Waiting for the local Telegram webhook send limit"

type telegramRateHold struct{ retryAt time.Time }

func (*telegramRateHold) Error() string { return telegramRateHoldMessage }

// prepareTelegramBatchLocked runs before the first attempt, under the same
// queue lock and delivery gates as claim. Do not partition sent, attempted, or
// operator-retried work: their retained attempts belong to the original batch.
func (q *NotificationQueue) prepareTelegramBatchLocked(row *QueuedNotification, now time.Time) (bool, error) {
	kind, event := normalizeQueueType(row.Type)
	if kind != "webhook" || len(row.Alerts) < 2 || row.Attempts != 0 || row.LastAttempt != nil {
		return true, nil
	}
	var hook WebhookConfig
	if err := json.Unmarshal(row.Config, &hook); err != nil || !hook.Enabled || !sizesTelegramDeliveries(hook) {
		return true, nil // retain existing validation and failure behaviour
	}

	// The pending snapshot can predate a concurrent cancellation or hold.
	current, err := q.scanNotification(q.db.QueryRow(`
		SELECT id, type, method, status, alerts, config, attempts, max_attempts,
		       last_attempt, last_error, created_at, next_retry_at, completed_at,
		       payload_bytes, operational_links, receipt_epoch
		FROM notification_queue WHERE id = ? AND status = 'pending'`, row.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reload Telegram batch: %w", err)
	}
	if (current.NextRetryAt != nil && current.NextRetryAt.After(now)) ||
		current.Attempts != 0 || current.LastAttempt != nil ||
		current.Type != row.Type || !bytes.Equal(current.Config, row.Config) {
		return false, nil
	}
	*row = *current
	batches := telegramAlertBatches(current.Alerts, event)
	if len(batches) <= 1 {
		return true, nil
	}
	if err := q.persistTelegramBatchesLocked(current, batches); err != nil {
		return false, err
	}
	*row = *current
	select {
	case q.notifyChan <- struct{}{}: // children are independently due
	default:
	}
	return true, nil
}

// Retain the original ID for the first batch and preserve config and attempt
// budget for each child. No provider call occurs unless the entire partition
// is durable. Each successful response can settle only its own membership.
func (q *NotificationQueue) persistTelegramBatchesLocked(row *QueuedNotification, batches [][]*alerts.Alert) error {
	tx, err := q.db.Begin()
	if err != nil {
		return fmt.Errorf("begin Telegram batch partition: %w", err)
	}
	defer tx.Rollback()
	ids := make([]string, len(batches))
	ids[0] = row.ID
	for i := 1; i < len(ids); i++ {
		// A part held before its first attempt can lose its leader and need
		// another partition. Positions from a prior split still belong to
		// retained siblings and their audit/settlement records.
		ids[i] = "telegram-" + uuid.NewString()
	}
	links, err := telegramBatchLinks(row.Links, batches, ids)
	if err != nil {
		return err
	}
	method := row.Method
	if strings.HasPrefix(method, "resolved-group") {
		method = "telegram-sized" // this recovery group has reached its deadline
	}
	for i, batch := range batches {
		body, err := json.Marshal(batch)
		if err != nil {
			return fmt.Errorf("encode Telegram batch: %w", err)
		}
		linkBody, err := json.Marshal(links[i])
		if err != nil {
			return fmt.Errorf("encode Telegram batch links: %w", err)
		}
		if i == 0 {
			_, err = tx.Exec(`UPDATE notification_queue SET alerts = ?, operational_links = ?, method = ? WHERE id = ?`, string(body), string(linkBody), method, row.ID)
		} else {
			_, err = tx.Exec(`INSERT INTO notification_queue
				(id, type, method, status, alerts, operational_links, config, attempts,
				 max_attempts, last_attempt, last_error, created_at, next_retry_at,
				 completed_at, payload_bytes, receipt_epoch)
				SELECT ?, type, method, status, ?, ?, config, attempts,
				       max_attempts, last_attempt, last_error, created_at, next_retry_at,
				       completed_at, payload_bytes, receipt_epoch
				FROM notification_queue WHERE id = ?`, ids[i], string(body), string(linkBody), row.ID)
		}
		if err != nil {
			return fmt.Errorf("persist Telegram batch partition: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Telegram batch partition: %w", err)
	}
	row.Alerts = batches[0]
	row.Links = links[0]
	row.Method = method
	return nil
}

func telegramBatchLinks(links []operationaltrust.NotificationLink, batches [][]*alerts.Alert, ids []string) ([][]operationaltrust.NotificationLink, error) {
	result := make([][]operationaltrust.NotificationLink, len(batches))
	if len(links) == 0 {
		return result, nil
	}
	type key struct{ record, transition string }
	owners := make(map[key]int, len(links))
	for i, batch := range batches {
		for _, alert := range batch {
			if alert.OperationalRecord == nil || alert.LatestTransition == nil {
				continue
			}
			k := key{alert.OperationalRecord.ID, alert.LatestTransition.ID}
			if prior, ok := owners[k]; ok && prior != i {
				owners[k] = -1
			} else {
				owners[k] = i
			}
		}
	}
	for _, link := range links {
		i, ok := owners[key{link.OperationalRecordID, link.TransitionID}]
		if !ok || i < 0 {
			return nil, fmt.Errorf("Telegram batch partition has an ambiguous operational link")
		}
		link = link.Clone()
		link.NotificationID = ids[i]
		result[i] = append(result[i], link)
	}
	return result, nil
}
