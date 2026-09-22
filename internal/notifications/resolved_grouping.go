package notifications

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// enqueueResolvedGroup persists the window on the first resolution and appends
// subsequent occurrences to that not-yet-due delivery. Only untouched pending
// groups with identical destination configuration and quiet-hours schedule can
// merge. Attempts/retries are immutable batches. The queue mutex also serializes
// claims, so no delivery can be claimed while its payload is being extended.
// Method is queue scheduling metadata, not the HTTP method in Config.
func (nq *NotificationQueue) enqueueResolvedGroup(notif *QueuedNotification, window time.Duration) error {
	if notif == nil || window <= 0 {
		return nq.Enqueue(notif)
	}
	nq.mu.Lock()
	defer nq.mu.Unlock()
	now := time.Now()
	method := "resolved-group"
	if notif.NextRetryAt != nil {
		method += ":" + notif.NextRetryAt.UTC().Format(time.RFC3339Nano)
	}
	var id string
	var raw []byte
	err := nq.db.QueryRow(`SELECT id, alerts FROM notification_queue
  WHERE type = ? AND config = ? AND method = ? AND status = 'pending'
   AND attempts = 0 AND next_retry_at > ? AND created_at >= ?
  ORDER BY created_at, id LIMIT 1`, notif.Type, string(notif.Config), method, now.Unix(), now.Add(-window).Unix()).Scan(&id, &raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read pending resolved group: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		// Queue deadlines have second precision: round up rather than sending early.
		deadline := now.Add(window).Truncate(time.Second).Add(time.Second)
		if notif.NextRetryAt == nil || notif.NextRetryAt.Before(deadline) {
			notif.NextRetryAt = &deadline
		}
		notif.Method = method
		return nq.enqueueLocked(notif)
	}
	var batch []*alerts.Alert
	if err := json.Unmarshal(raw, &batch); err != nil {
		return fmt.Errorf("decode pending resolved group: %w", err)
	}
	for _, incoming := range notif.Alerts {
		if incoming == nil {
			continue
		}
		duplicate := false
		for _, existing := range batch {
			if existing != nil && existing.ID == incoming.ID && existing.StartTime.Equal(incoming.StartTime) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			batch = append(batch, incoming)
		}
	}
	links, err := normalizeNotificationLinks(id, notif.DestinationID, notificationLinksForAlerts(batch, notif.DestinationID), notificationDeliveryStateForQueueStatus(QueueStatusPending), nil, nil)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	encodedLinks, err := json.Marshal(links)
	if err != nil {
		return err
	}
	if _, err := nq.db.Exec(`UPDATE notification_queue SET alerts = ?, operational_links = ? WHERE id = ?`, string(encoded), string(encodedLinks), id); err != nil {
		return fmt.Errorf("extend pending resolved group: %w", err)
	}
	return nil
}
