package notifications

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// prepareQuietHoursDelivery runs under the original batch's delivery gates,
// before claiming a provider attempt. Reloading the row preserves cancellation
// rewrites made after GetPending. Held work keeps the original row; a mixed
// batch gets one ready child, committed atomically with the held remainder.
// A hold is scheduling, not an attempt, failure, receipt or lifecycle change.
func (nq *NotificationQueue) prepareQuietHoursDelivery(
	notif *QueuedNotification,
	policy func(*alerts.Alert, time.Time) *time.Time,
) (bool, error) {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	now := time.Now()
	if nq.now != nil {
		now = nq.now()
	}
	current, err := nq.scanNotification(nq.db.QueryRow(`
  SELECT id, type, method, status, alerts, config, attempts, max_attempts,
         last_attempt, last_error, created_at, next_retry_at, completed_at,
         payload_bytes, operational_links
  FROM notification_queue WHERE id = ? AND status = 'pending'`, notif.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reload quiet-hours notification: %w", err)
	}
	// Another worker may already have postponed this row since GetPending.
	if current.NextRetryAt != nil && current.NextRetryAt.After(now) {
		return false, nil
	}
	var ready, held []*alerts.Alert
	var replayAt *time.Time
	for _, alert := range current.Alerts {
		at := policy(alert, now)
		if at == nil || !at.After(now) {
			ready = append(ready, alert)
			continue
		}
		held = append(held, alert)
		if replayAt == nil || at.Before(*replayAt) {
			value := at.UTC()
			replayAt = &value
		}
	}
	if len(held) == 0 {
		*notif = *current
		return true, nil
	}
	// The queue stores second precision. Never round a policy deadline down
	// into a provider send (or a busy retry loop) before it becomes eligible.
	deadline := replayAt.Truncate(time.Second)
	if deadline.Before(*replayAt) {
		deadline = deadline.Add(time.Second)
	}
	method := current.Method
	if strings.HasPrefix(method, "resolved-group") {
		// This batch has reached its admitted grouping deadline. It must not
		// reopen that window and absorb later recoveries while quiet-held.
		method = "quiet-hours-replay"
	}
	if len(ready) == 0 {
		_, err := nq.db.Exec(`UPDATE notification_queue
  SET next_retry_at = ?, method = ? WHERE id = ? AND status = 'pending'`, deadline.Unix(), method, current.ID)
		if err != nil {
			return false, fmt.Errorf("postpone quiet-hours notification: %w", err)
		}
		current.NextRetryAt = &deadline
		current.Method = method
		*notif = *current
		return false, nil
	}

	readyJSON, err := json.Marshal(ready)
	if err != nil {
		return false, fmt.Errorf("encode ready quiet-hours alerts: %w", err)
	}
	heldJSON, err := json.Marshal(held)
	if err != nil {
		return false, fmt.Errorf("encode held quiet-hours alerts: %w", err)
	}
	// Include the parent identity and exact ready payload. Restarts and duplicate
	// worker snapshots cannot generate a second child for the same partition.
	sum := sha256.Sum256(append([]byte(current.ID+"\x00"), readyJSON...))
	childID := current.ID + ":quiet:" + hex.EncodeToString(sum[:8])
	readyLinks, heldLinks, err := partitionQuietHoursLinks(current.Links, ready, held, childID)
	if err != nil {
		return false, err
	}
	readyLinksJSON, err := json.Marshal(readyLinks)
	if err != nil {
		return false, fmt.Errorf("encode ready quiet-hours links: %w", err)
	}
	heldLinksJSON, err := json.Marshal(heldLinks)
	if err != nil {
		return false, fmt.Errorf("encode held quiet-hours links: %w", err)
	}

	tx, err := nq.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin quiet-hours partition: %w", err)
	}
	defer tx.Rollback()
	// Copy the admitted destination bytes and attempt budget, not the current
	// destination configuration. Previous attempts remain in the original audit
	// trail; splitting never resets the retry budget or fabricates a new audit.
	_, err = tx.Exec(`INSERT INTO notification_queue
  (id, type, method, status, alerts, operational_links, config, attempts,
   max_attempts, last_attempt, last_error, created_at, next_retry_at,
   completed_at, payload_bytes)
  SELECT ?, type, method, status, ?, ?, config, attempts, max_attempts,
         last_attempt, last_error, created_at, NULL, completed_at, payload_bytes
  FROM notification_queue WHERE id = ? AND status = 'pending'`, childID, string(readyJSON), string(readyLinksJSON), current.ID)
	if err != nil {
		return false, fmt.Errorf("persist ready quiet-hours partition: %w", err)
	}
	_, err = tx.Exec(`UPDATE notification_queue
  SET alerts = ?, operational_links = ?, next_retry_at = ?, method = ?
  WHERE id = ? AND status = 'pending'`, string(heldJSON), string(heldLinksJSON), deadline.Unix(), method, current.ID)
	if err != nil {
		return false, fmt.Errorf("persist held quiet-hours partition: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit quiet-hours partition: %w", err)
	}
	current.ID = childID
	current.Alerts = ready
	current.Links = readyLinks
	current.NextRetryAt = nil
	*notif = *current
	return true, nil
}

func partitionQuietHoursLinks(
	links []operationaltrust.NotificationLink,
	ready, held []*alerts.Alert,
	childID string,
) ([]operationaltrust.NotificationLink, []operationaltrust.NotificationLink, error) {
	keys := func(batch []*alerts.Alert) map[string]bool {
		result := make(map[string]bool, len(batch))
		for _, alert := range batch {
			if alert != nil && alert.OperationalRecord != nil && alert.LatestTransition != nil {
				result[alert.OperationalRecord.ID+"\x00"+alert.LatestTransition.ID] = true
			}
		}
		return result
	}
	readyKeys, heldKeys := keys(ready), keys(held)
	var readyLinks, heldLinks []operationaltrust.NotificationLink
	for _, link := range links {
		key := link.OperationalRecordID + "\x00" + link.TransitionID
		switch {
		case readyKeys[key] && !heldKeys[key]:
			link = link.Clone()
			link.NotificationID = childID
			readyLinks = append(readyLinks, link)
		case heldKeys[key] && !readyKeys[key]:
			heldLinks = append(heldLinks, link.Clone())
		default:
			// Do not silently discard or misattribute a historical linkage.
			return nil, nil, fmt.Errorf("quiet-hours partition has an ambiguous operational link")
		}
	}
	return readyLinks, heldLinks, nil
}
