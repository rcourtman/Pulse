package notifications

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func newDLQRecoveryTestQueue(t *testing.T, clockNano *atomic.Int64) *NotificationQueue {
	t.Helper()
	q, err := NewNotificationQueue(t.TempDir())
	if err != nil {
		t.Fatalf("NewNotificationQueue: %v", err)
	}
	t.Cleanup(func() { _ = q.Stop() })
	q.mu.Lock()
	q.now = func() time.Time { return time.Unix(0, clockNano.Load()).UTC() }
	q.mu.Unlock()
	return q
}

func waitForNotificationStatus(t *testing.T, q *NotificationQueue, id string, want NotificationQueueStatus, wantAudits int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var status string
		var audits int
		if err := q.db.QueryRow(`SELECT status FROM notification_queue WHERE id = ?`, id).Scan(&status); err != nil {
			t.Fatalf("read status for %s: %v", id, err)
		}
		if err := q.db.QueryRow(`SELECT COUNT(*) FROM notification_audit WHERE notification_id = ?`, id).Scan(&audits); err != nil {
			t.Fatalf("read audits for %s: %v", id, err)
		}
		if status == string(want) && audits == wantAudits {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: status=%s audits=%d, want %s/%d", id, status, audits, want, wantAudits)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitForStatusCounts(t *testing.T, q *NotificationQueue, wantSent, wantDLQ int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var sentCount, dlqCount int
		if err := q.db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE status = 'sent'`).Scan(&sentCount); err != nil {
			t.Fatalf("read sent count: %v", err)
		}
		if err := q.db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE status = 'dlq'`).Scan(&dlqCount); err != nil {
			t.Fatalf("read dlq count: %v", err)
		}
		if sentCount == wantSent && dlqCount == wantDLQ {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("status counts: sent=%d dlq=%d, want sent=%d dlq=%d", sentCount, dlqCount, wantSent, wantDLQ)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAutoReplayDLQOnDestinationRecovery_ReplaysRetryableSameDestinationOnly(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Second)
	var clock atomic.Int64
	clock.Store(base.UnixNano())
	q := newDLQRecoveryTestQueue(t, &clock)

	destA, _ := json.Marshal(WebhookConfig{ID: "wh-ha", Name: "Home Assistant", URL: "https://ha.local/api/webhook/pulse", Enabled: true})
	destB, _ := json.Marshal(WebhookConfig{ID: "wh-other", Name: "Other", URL: "https://other.local/hook", Enabled: true})
	destAEdited, _ := json.Marshal(WebhookConfig{ID: "wh-ha", Name: "Home Assistant", URL: "https://ha-new.local/api/webhook/pulse", Enabled: true})

	var failMode atomic.Int32 // 0=ok, 1=503 retryable, 2=401 auth non-retryable, 3=400 rejected non-retryable
	q.SetProcessor(func(n *QueuedNotification) error {
		switch failMode.Load() {
		case 1:
			return errors.New("webhook returned status 503 service unavailable")
		case 2:
			return errors.New("webhook returned status 401 unauthorized")
		case 3:
			return errors.New("webhook returned status 400 bad request")
		default:
			return nil
		}
	})

	enqueue := func(id, typ string, cfg []byte, created time.Time) {
		t.Helper()
		err := q.Enqueue(&QueuedNotification{
			ID:          id,
			Type:        typ,
			Status:      QueueStatusPending,
			Config:      cfg,
			MaxAttempts: 1,
			CreatedAt:   created,
			Alerts: []*alerts.Alert{
				{ID: "alert-" + id, ResourceName: "pve1", Type: "cpu", Level: alerts.AlertLevelWarning, StartTime: created},
			},
		})
		if err != nil {
			t.Fatalf("Enqueue %s: %v", id, err)
		}
	}

	// 1. Seed retryable DLQ alert + resolved on destA, sibling destB, edited destA, and >24h stale destA.
	failMode.Store(1)
	enqueue("dlq-retryable-alert", "webhook", destA, base.Add(-30*time.Minute))
	enqueue("dlq-retryable-resolved", "webhook_resolved", destA, base.Add(-20*time.Minute))
	enqueue("dlq-other-dest", "webhook", destB, base.Add(-15*time.Minute))
	enqueue("dlq-edited-config", "webhook", destAEdited, base.Add(-15*time.Minute))
	enqueue("dlq-stale-25h", "webhook", destA, base.Add(-25*time.Hour))

	for _, id := range []string{"dlq-retryable-alert", "dlq-retryable-resolved", "dlq-other-dest", "dlq-edited-config", "dlq-stale-25h"} {
		waitForNotificationStatus(t, q, id, QueueStatusDLQ, 1)
	}
	// Backdate completed_at for the stale row beyond 24h.
	if _, err := q.db.Exec(`UPDATE notification_queue SET completed_at = ? WHERE id = 'dlq-stale-25h'`, base.Add(-25*time.Hour).Unix()); err != nil {
		t.Fatalf("backdate stale row: %v", err)
	}

	// 2. Seed non-retryable auth (401) and rejected (400) DLQ alerts on destA.
	failMode.Store(2)
	enqueue("dlq-auth-401", "webhook", destA, base.Add(-10*time.Minute))
	waitForNotificationStatus(t, q, "dlq-auth-401", QueueStatusDLQ, 1)

	failMode.Store(3)
	enqueue("dlq-rejected-400", "webhook", destA, base.Add(-10*time.Minute))
	waitForNotificationStatus(t, q, "dlq-rejected-400", QueueStatusDLQ, 1)

	// 3. Destination A recovers and a live alert succeeds.
	failMode.Store(0)
	enqueue("live-recovery-probe", "webhook", destA, base)
	waitForNotificationStatus(t, q, "live-recovery-probe", QueueStatusSent, 1)

	// Recent retryable alert + resolved on destA must auto-replay and succeed.
	waitForNotificationStatus(t, q, "dlq-retryable-alert", QueueStatusSent, 2)
	waitForNotificationStatus(t, q, "dlq-retryable-resolved", QueueStatusSent, 2)

	// Non-retryable, sibling, edited-config, and >24h stale DLQ rows must stay untouched in DLQ.
	for _, untouched := range []string{"dlq-other-dest", "dlq-edited-config", "dlq-stale-25h", "dlq-auth-401", "dlq-rejected-400"} {
		waitForNotificationStatus(t, q, untouched, QueueStatusDLQ, 1)
	}
}

func TestAutoReplayDLQOnDestinationRecovery_SingleReplayCapPreventsFlapLoop(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Second)
	var clock atomic.Int64
	clock.Store(base.UnixNano())
	q := newDLQRecoveryTestQueue(t, &clock)

	destA, _ := json.Marshal(WebhookConfig{ID: "wh-flap", Name: "Flappy", URL: "https://flap.local/hook", Enabled: true})

	var failID atomic.Value
	failID.Store("")
	q.SetProcessor(func(n *QueuedNotification) error {
		if n.ID == failID.Load().(string) {
			return errors.New("connection refused")
		}
		return nil
	})

	enqueue := func(id string) {
		t.Helper()
		err := q.Enqueue(&QueuedNotification{
			ID:          id,
			Type:        "webhook",
			Status:      QueueStatusPending,
			Config:      destA,
			MaxAttempts: 1,
			CreatedAt:   base,
			Alerts: []*alerts.Alert{
				{ID: "alert-" + id, ResourceName: "pve1", Type: "cpu", Level: alerts.AlertLevelWarning, StartTime: base},
			},
		})
		if err != nil {
			t.Fatalf("Enqueue %s: %v", id, err)
		}
	}

	// Row "flappy-dlq" fails initially -> DLQ (1 audit).
	failID.Store("flappy-dlq")
	enqueue("flappy-dlq")
	waitForNotificationStatus(t, q, "flappy-dlq", QueueStatusDLQ, 1)

	// First live success triggers the one allowed automatic replay; "flappy-dlq"
	// fails again -> returns to DLQ with 2 audits (> max_attempts=1).
	enqueue("live-success-1")
	waitForNotificationStatus(t, q, "live-success-1", QueueStatusSent, 1)
	waitForNotificationStatus(t, q, "flappy-dlq", QueueStatusDLQ, 2)

	// Advance past cooldown and send a second live success; "flappy-dlq" must
	// NOT auto-replay a second time because its audit count already exceeds max_attempts.
	clock.Store(base.Add(autoReplayDLQCooldown + time.Minute).UnixNano())
	enqueue("live-success-2")
	waitForNotificationStatus(t, q, "live-success-2", QueueStatusSent, 1)
	time.Sleep(50 * time.Millisecond)
	waitForNotificationStatus(t, q, "flappy-dlq", QueueStatusDLQ, 2)

	// Explicit manual operator retry still works unconstrained.
	failID.Store("")
	count, err := q.RetryTerminalFailures()
	if err != nil || count != 1 {
		t.Fatalf("RetryTerminalFailures = %d, %v; want 1", count, err)
	}
	waitForNotificationStatus(t, q, "flappy-dlq", QueueStatusSent, 3)
}

func TestAutoReplayDLQOnDestinationRecovery_RespectsBatchLimitAndCooldown(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Second)
	var clock atomic.Int64
	clock.Store(base.UnixNano())
	q := newDLQRecoveryTestQueue(t, &clock)

	destA, _ := json.Marshal(WebhookConfig{ID: "wh-batch", Name: "Batch", URL: "https://batch.local/hook", Enabled: true})

	var failing atomic.Bool
	failing.Store(true)
	q.SetProcessor(func(n *QueuedNotification) error {
		if strings.HasPrefix(n.ID, "older-auth-") {
			return errors.New("webhook returned status 401 unauthorized")
		}
		if failing.Load() {
			return errors.New("dial tcp 192.0.2.10:8123: i/o timeout")
		}
		return nil
	})

	// A full batch of deterministically ineligible rows precedes every
	// eligible one. One legacy row still needs classification from its error.
	ineligible := autoReplayDLQBatchLimit
	for i := 0; i < ineligible; i++ {
		id := fmt.Sprintf("older-auth-%02d", i)
		if err := q.Enqueue(&QueuedNotification{
			ID: id, Type: "webhook", Config: destA, MaxAttempts: 1, CreatedAt: base,
		}); err != nil {
			t.Fatal(err)
		}
		waitForNotificationStatus(t, q, id, QueueStatusDLQ, 1)
	}
	if _, err := q.db.Exec(`UPDATE notification_queue SET completed_at=? WHERE id LIKE 'older-auth-%'`, base.Add(-time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := q.db.Exec(`UPDATE notification_audit SET failure_class='' WHERE notification_id='older-auth-00'`); err != nil {
		t.Fatal(err)
	}

	total := autoReplayDLQBatchLimit + 5
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("batch-dlq-%02d", i)
		created := base.Add(-time.Duration(total-i) * time.Minute)
		if err := q.Enqueue(&QueuedNotification{
			ID:          id,
			Type:        "webhook",
			Status:      QueueStatusPending,
			Config:      destA,
			MaxAttempts: 1,
			CreatedAt:   created,
			Alerts: []*alerts.Alert{
				{ID: "alert-" + id, ResourceName: "pve1", Type: "cpu", Level: alerts.AlertLevelWarning, StartTime: created},
			},
		}); err != nil {
			t.Fatalf("Enqueue %s: %v", id, err)
		}
	}
	waitForStatusCounts(t, q, 0, ineligible+total)

	// Recovery triggers exactly one bounded batch (25 rows); cooldown prevents
	// those 25 successes from immediately cascading into the remaining 5 rows.
	failing.Store(false)
	if err := q.Enqueue(&QueuedNotification{
		ID:          "live-trigger-1",
		Type:        "webhook",
		Status:      QueueStatusPending,
		Config:      destA,
		MaxAttempts: 1,
		CreatedAt:   base,
		Alerts: []*alerts.Alert{
			{ID: "alert-live-1", ResourceName: "pve1", Type: "cpu", Level: alerts.AlertLevelWarning, StartTime: base},
		},
	}); err != nil {
		t.Fatalf("Enqueue live-trigger-1: %v", err)
	}
	waitForStatusCounts(t, q, 1+autoReplayDLQBatchLimit, ineligible+5)
	time.Sleep(50 * time.Millisecond)
	waitForStatusCounts(t, q, 1+autoReplayDLQBatchLimit, ineligible+5)

	// Once cooldown elapses, the next live delivery smoothly drains the remaining 5 rows.
	clock.Store(base.Add(autoReplayDLQCooldown + time.Minute).UnixNano())
	if err := q.Enqueue(&QueuedNotification{
		ID:          "live-trigger-2",
		Type:        "webhook",
		Status:      QueueStatusPending,
		Config:      destA,
		MaxAttempts: 1,
		CreatedAt:   base.Add(autoReplayDLQCooldown + time.Minute),
		Alerts: []*alerts.Alert{
			{ID: "alert-live-2", ResourceName: "pve1", Type: "cpu", Level: alerts.AlertLevelWarning, StartTime: base},
		},
	}); err != nil {
		t.Fatalf("Enqueue live-trigger-2: %v", err)
	}
	waitForStatusCounts(t, q, 2+total, ineligible)
}
