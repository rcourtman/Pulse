package notifications

import (
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Model the durable state left by an interrupted send, not a process kill.
// A receiver may have accepted that interrupted send already: recovery is
// at-least-once, and this test deliberately makes no exactly-once claim.
func TestNotificationQueueRestartResumesOnlyEligibleJobs(t *testing.T) {
	dir := t.TempDir()
	open := func() *NotificationQueue {
		t.Helper()
		q, err := NewNotificationQueue(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := q.Stop(); err != nil {
				t.Error(err)
			}
		})
		return q
	}
	q := open()
	future := time.Now().Add(time.Hour).Truncate(time.Second)
	states := map[string]NotificationQueueStatus{
		"pending": QueueStatusPending, "interrupted": QueueStatusSending,
		"delayed": QueueStatusPending, "sent": QueueStatusSent,
		"cancelled": QueueStatusCancelled, "failed": QueueStatusFailed, "dlq": QueueStatusDLQ,
	}
	for id, status := range states {
		n := &QueuedNotification{
			ID: id, Type: "webhook", Status: QueueStatusPending, Attempts: 2, MaxAttempts: 8,
			Config: []byte(`{"event":"resolved"}`),
			Alerts: []*alerts.Alert{{ID: "incident-" + id, StartTime: future.Add(-2 * time.Hour)}},
		}
		if id == "delayed" {
			n.NextRetryAt = &future
		}
		if err := q.Enqueue(n); err != nil {
			t.Fatal(err)
		}
		if status != QueueStatusPending {
			if err := q.UpdateStatus(id, status, ""); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := q.Stop(); err != nil {
		t.Fatal(err)
	}
	q = open()
	// Startup must not consume attempts before a processor is installed.
	for id, status := range states {
		if status == QueueStatusSending {
			status = QueueStatusPending
		}
		var got NotificationQueueStatus
		var attempts int
		if err := q.db.QueryRow(`SELECT status, attempts FROM notification_queue WHERE id = ?`, id).Scan(&got, &attempts); err != nil {
			t.Fatal(err)
		}
		if got != status || attempts != 2 {
			t.Fatalf("%s restored as %s/%d, want %s/2", id, got, attempts, status)
		}
	}
	var mu sync.Mutex
	calls := map[string]int{}
	q.SetProcessor(func(n *QueuedNotification) error {
		mu.Lock()
		defer mu.Unlock()
		calls[n.ID]++
		if len(n.Alerts) != 1 || n.Alerts[0].ID != "incident-"+n.ID || !n.Alerts[0].StartTime.Equal(future.Add(-2*time.Hour)) || string(n.Config) != `{"event":"resolved"}` {
			t.Errorf("durable payload changed for %s", n.ID)
		}
		return nil
	})
	deadline := time.Now().Add(3 * time.Second)
	for {
		var sent int
		if err := q.db.QueryRow(`SELECT count(*) FROM notification_queue WHERE id IN ('pending','interrupted') AND status = 'sent'`).Scan(&sent); err != nil {
			t.Fatal(err)
		}
		if sent == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("eligible jobs did not resume after processor installation")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Synchronously exercise another batch, rather than relying on a sleep to
	// check that terminal rows and a retry in the future cannot be selected.
	q.processBatch()
	if err := q.Stop(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(calls) != 2 || calls["pending"] != 1 || calls["interrupted"] != 1 {
		t.Errorf("processor calls = %v", calls)
	}
	mu.Unlock()
	q = open()
	for id, status := range states {
		wantAttempts := 2
		if id == "pending" || id == "interrupted" {
			status, wantAttempts = QueueStatusSent, 3
		}
		var got NotificationQueueStatus
		var attempts int
		if err := q.db.QueryRow(`SELECT status, attempts FROM notification_queue WHERE id = ?`, id).Scan(&got, &attempts); err != nil {
			t.Fatal(err)
		}
		if got != status || attempts != wantAttempts {
			t.Errorf("%s second restart = %s/%d, want %s/%d", id, got, attempts, status, wantAttempts)
		}
	}
	var retry int64
	if err := q.db.QueryRow(`SELECT next_retry_at FROM notification_queue WHERE id = 'delayed'`).Scan(&retry); err != nil {
		t.Fatal(err)
	}
	if retry != future.Unix() {
		t.Errorf("retry deadline = %v, want %v", retry, future)
	}
	pending, err := q.GetPending(20)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("second restart exposed %d jobs for replay", len(pending))
	}
}
