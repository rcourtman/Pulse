package notifications

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// The guide must not equate a settings change with remote containment. Wait
// until the real receiver has the request before changing policy; a subsequent
// queued attempt must be skipped, but the original request can still succeed.
func TestQueuedWebhookPolicyChangeCannotRecallInFlight(t *testing.T) {
	for _, policy := range []string{"global-pause", "destination-disable", "destination-remove"} {
		t.Run(policy, func(t *testing.T) {
			started, release := make(chan struct{}, 1), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			var requests atomic.Int32
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				started <- struct{}{}
				<-release
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			defer unblock() // Also drain the receiver on an assertion failure.
			m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
			defer m.Stop()
			if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
				t.Fatal(err)
			}
			hook := WebhookConfig{ID: "guide-ops", Enabled: true, Service: "generic", URL: server.URL}
			m.AddWebhook(hook)
			config, err := json.Marshal(hook)
			if err != nil {
				t.Fatal(err)
			}
			job := &QueuedNotification{ID: "in-flight", Type: "webhook", Config: config,
				Alerts: []*alerts.Alert{{ID: "guide-incident", ResourceName: "synthetic-node", Type: "cpu",
					Level: alerts.AlertLevelWarning, StartTime: time.Now().Add(-time.Minute)}}}
			done := make(chan error, 1)
			go func() { done <- m.ProcessQueuedNotification(job) }()
			select {
			case <-started:
			case err := <-done:
				t.Fatalf("request ended before receiver admission: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("receiver did not observe request")
			}
			switch policy {
			case "global-pause":
				m.SetEnabled(false)
			case "destination-disable":
				hook.Enabled = false
				if err := m.UpdateWebhook(hook.ID, hook); err != nil {
					t.Fatal(err)
				}
			case "destination-remove":
				if err := m.DeleteWebhook(hook.ID); err != nil {
					t.Fatal(err)
				}
			}
			if err := m.ProcessQueuedNotification(job); !errors.Is(err, ErrNotificationDeliverySkipped) {
				t.Fatalf("later attempt = %v; want policy skip", err)
			}
			unblock()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("original in-flight request did not complete: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("original request did not drain")
			}
			if got := requests.Load(); got != 1 {
				t.Fatalf("receiver requests = %d; want exactly the original request", got)
			}
			t.Log("policy blocks a later attempt, but receiver accepts the already in-flight request")
		})
	}
}

// Global pause is cancellation, unlike a quiet-hours hold. Exercise real
// enqueue, buffered grouping, pause, resume and persistent reconstruction;
// neither resume nor the retained-failure retry button restores cancelled work.
func TestGlobalNotificationPauseDoesNotCatchUpCancelledWork(t *testing.T) {
	dir := t.TempDir()
	open := func() *NotificationManager {
		m := NewNotificationManagerWithDeferredQueue("", dir)
		m.AddWebhook(WebhookConfig{ID: "guide-ops", Enabled: true, Service: "generic", URL: "https://example.test/hook"})
		t.Cleanup(m.Stop)
		return m
	}
	m := open()
	m.SetGroupingConfig(false, 0, false, false)
	alert := &alerts.Alert{ID: "pending-before-pause", ResourceName: "synthetic-node", Type: "cpu",
		Level: alerts.AlertLevelWarning, StartTime: time.Now().Add(-time.Minute)}
	m.SendAlert(alert)
	rows, err := m.queue.GetPending(10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("pending before pause = %d, %v; want one delivery", len(rows), err)
	}
	for _, kind := range []string{"webhook_resolved", "email", "email_resolved", "apprise", "apprise_resolved"} {
		if err := m.queue.Enqueue(&QueuedNotification{ID: kind, Type: kind, Config: []byte(`{}`),
			Alerts: []*alerts.Alert{alert}, CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.queue.Enqueue(&QueuedNotification{ID: "retained-failure", Type: "webhook", Status: QueueStatusDLQ,
		Config: []byte(`{}`), Alerts: []*alerts.Alert{alert}, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	m.SetGroupingConfig(true, 3600, false, false)
	buffered := alert.Clone()
	buffered.ID = "buffered-before-pause"
	m.SendAlert(buffered)
	if len(m.pendingAlerts) != 1 {
		t.Fatal("fixture did not buffer one alert group")
	}
	m.SetEnabled(false)
	if len(m.pendingAlerts) != 0 || m.groupTimer != nil {
		t.Fatal("pause did not clear buffered groups and their timer")
	}
	paused := alert.Clone()
	paused.ID = "during-pause"
	m.SendAlert(paused)
	m.SetEnabled(true)
	if len(m.pendingAlerts) != 0 {
		t.Fatal("resume recreated a buffered group")
	}
	m.Stop()
	m = open() // Saved queue, fully enabled policy, no automatic workers.
	rows, err = m.queue.GetPending(10)
	if err != nil || len(rows) != 0 {
		t.Fatalf("pending after resume/reconstruction = %d, %v; want none", len(rows), err)
	}
	stats, err := m.queue.GetQueueStats()
	if err != nil || stats["cancelled"] != 6 || stats["dlq"] != 1 || len(stats) != 2 {
		t.Fatalf("retained queue = %v, %v; want six cancelled deliveries and one separate retained failure", stats, err)
	}
	if count, err := m.queue.RetryTerminalFailures(); err != nil || count != 1 {
		t.Fatalf("retained-failure retry = %d, %v; want only the separate failure", count, err)
	}
	rows, err = m.queue.GetPending(10)
	if err != nil || len(rows) != 1 || rows[0].ID != "retained-failure" {
		t.Fatalf("retry rows = %+v, %v; want only the original retained failure, no cancelled replay", rows, err)
	}
	t.Log("pause cancels pending work and drops buffered/paused alerts; resume/restart/retry creates no catch-up")
}
