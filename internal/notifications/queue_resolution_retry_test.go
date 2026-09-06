package notifications

import (
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// A repaired destination must not replay obsolete firing alerts. Exercise the
// operator retry after reopening the database, not just the cancellation query.
func TestResolvedTerminalFiringIsNotReplayedAfterRestart(t *testing.T) {
	for _, terminal := range []NotificationQueueStatus{QueueStatusFailed, QueueStatusDLQ} {
		t.Run(string(terminal), func(t *testing.T) {
			dir := t.TempDir()
			q, err := NewNotificationQueue(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = q.Stop() }()
			for _, n := range []*QueuedNotification{
				{ID: "obsolete", Type: "webhook", Alerts: []*alerts.Alert{{ID: "healthy"}}},
				{ID: "group", Type: "webhook", Alerts: []*alerts.Alert{{ID: "healthy"}, {ID: "still-firing"}}},
				{ID: "recovery", Type: "webhook_resolved", Alerts: []*alerts.Alert{{ID: "healthy"}}},
			} {
				n.Config = []byte("{}")
				n.Status = QueueStatusPending
				n.MaxAttempts = 3
				if err := q.Enqueue(n); err != nil {
					t.Fatal(err)
				}
				if err := q.UpdateStatus(n.ID, terminal, "destination unavailable"); err != nil {
					t.Fatal(err)
				}
				if err := q.RecordAudit(n, false, "destination unavailable"); err != nil {
					t.Fatal(err)
				}
			}
			healthCallbacks := 0
			q.SetDeliveryHealthChangedCallback(func() {
				healthCallbacks++
				// Callback must run after both the DB mutex and alert gate
				// are released, and see the committed cancellation.
				release := q.acquireAlertDeliveryGates([]string{"healthy"}, true)
				defer release()
				stats, err := q.GetQueueStats()
				if err != nil {
					t.Error(err)
					return
				}
				if stats[string(terminal)] != 2 {
					t.Errorf("remaining terminal rows = %d, want 2", stats[string(terminal)])
				}
			})
			count, err := q.CancelByAlertIdentifiers([]string{"healthy"})
			if err != nil {
				t.Fatal(err)
			}
			q.SetDeliveryHealthChangedCallback(nil)
			if healthCallbacks != 1 {
				t.Errorf("health callbacks = %d, want 1", healthCallbacks)
			}
			if count != 0 {
				t.Fatalf("pending suppression count = %d, want 0 for terminal rows", count)
			}
			if err := q.Stop(); err != nil {
				t.Fatal(err)
			}
			q, err = NewNotificationQueue(dir)
			if err != nil {
				t.Fatal(err)
			}
			retried, err := q.RetryTerminalFailures()
			if err != nil {
				t.Fatal(err)
			}
			if retried != 2 {
				t.Errorf("retried %d rows, want only surviving group and recovery", retried)
			}
			// Alert IDs identify a resource/condition and can recur. Resolution
			// must suppress the old queue row, not permanently mute that ID.
			if err := q.Enqueue(&QueuedNotification{
				ID: "new-incident", Type: "webhook", Status: QueueStatusPending,
				Config: []byte("{}"), MaxAttempts: 3,
				Alerts: []*alerts.Alert{{ID: "healthy"}},
			}); err != nil {
				t.Fatal(err)
			}
			delivered := map[string][]string{}
			var mu sync.Mutex
			q.SetProcessor(func(n *QueuedNotification) error {
				mu.Lock()
				defer mu.Unlock()
				for _, a := range n.Alerts {
					delivered[n.ID] = append(delivered[n.ID], a.ID)
				}
				return nil
			})
			q.processBatch()
			deadline := time.Now().Add(3 * time.Second)
			for {
				var pending int
				if err := q.db.QueryRow("SELECT count(*) FROM notification_queue WHERE status IN ('pending', 'sending')").Scan(&pending); err != nil {
					t.Fatal(err)
				}
				if pending == 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("queue did not drain")
				}
				time.Sleep(time.Millisecond)
			}
			mu.Lock()
			if len(delivered) != 3 || len(delivered["new-incident"]) != 1 ||
				delivered["new-incident"][0] != "healthy" || len(delivered["group"]) != 1 ||
				delivered["group"][0] != "still-firing" ||
				len(delivered["recovery"]) != 1 || delivered["recovery"][0] != "healthy" {
				t.Errorf("replayed payloads = %v, want still-firing, genuine recovery and the new incident", delivered)
			}
			mu.Unlock()
			var failures int
			if err := q.db.QueryRow("SELECT count(*) FROM notification_audit WHERE success = 0").Scan(&failures); err != nil {
				t.Fatal(err)
			}
			if failures != 3 {
				t.Errorf("retained failed attempts = %d, want 3", failures)
			}
		})
	}
}

// A stale per-item retry request must not bypass resolution or resend a
// successful notification. The same scheduler handles transient send failures.
func TestScheduleRetryRejectsCancelledAndSent(t *testing.T) {
	q, err := NewNotificationQueue(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = q.Stop() }()
	for _, status := range []NotificationQueueStatus{
		QueueStatusCancelled, QueueStatusSent, QueueStatusPending,
		QueueStatusSending, QueueStatusFailed, QueueStatusDLQ,
	} {
		t.Run(string(status), func(t *testing.T) {
			id := string(status)
			n := &QueuedNotification{ID: id, Type: "webhook", Status: QueueStatusPending, Config: []byte("{}")}
			if err := q.Enqueue(n); err != nil {
				t.Fatal(err)
			}
			if err := q.UpdateStatus(id, status, ""); err != nil {
				t.Fatal(err)
			}
			err := q.ScheduleRetry(id, 0)
			blocked := status == QueueStatusCancelled || status == QueueStatusSent
			if (err != nil) != blocked {
				t.Fatalf("retry error = %v, blocked = %v", err, blocked)
			}
			want := QueueStatusPending
			if blocked {
				want = status
			}
			var got NotificationQueueStatus
			if err := q.db.QueryRow("SELECT status FROM notification_queue WHERE id = ?", id).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Errorf("status = %s, want %s", got, want)
			}
		})
	}
}
