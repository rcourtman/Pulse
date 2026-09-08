package notifications

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Provider rejection must preserve the firing receipt needed for recovery,
// including when an operator retries the recovery after reopening SQLite.
func TestQueuedNtfyRecoveryAfterProviderOutageAndRestart(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		class  NotificationFailureClass
	}{
		{"unavailable", http.StatusServiceUnavailable, NotificationFailureServerError},
		{"authentication", http.StatusUnauthorized, NotificationFailureAuthentication},
		{"rate_limited", http.StatusTooManyRequests, NotificationFailureRateLimited},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testQueuedNtfyRecoveryAfterProviderRejection(t, tc.status, tc.class)
		})
	}
}

func testQueuedNtfyRecoveryAfterProviderRejection(t *testing.T, status int, class NotificationFailureClass) {
	var unavailable atomic.Bool
	var accepted atomic.Int32
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		if unavailable.Load() {
			w.WriteHeader(status)
			return
		}
		if accepted.Add(1) == 2 {
			if r.Header.Get("Priority") != "default" || r.Header.Get("Title") != "RESOLVED: database" ||
				!strings.Contains(string(body), "is now healthy") {
				t.Errorf("incorrect recovery: headers=%v body=%q", r.Header, body)
			}
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	dir := t.TempDir()
	webhook := WebhookConfig{ID: "ops", Name: "ops", URL: server.URL + "/topic", Enabled: true, Service: "ntfy"}
	open := func() *NotificationManager {
		m := NewNotificationManagerWithDataDir("", dir)
		m.webhookClient = server.Client()
		if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
			t.Fatal(err)
		}
		m.AddWebhook(webhook)
		t.Cleanup(m.Stop)
		return m
	}
	m := open()
	config, err := json.Marshal(webhook)
	if err != nil {
		t.Fatal(err)
	}
	alert := &alerts.Alert{ID: "cpu", ResourceName: "database", Type: "cpu", Level: alerts.AlertLevelCritical,
		Message: "CPU above threshold", StartTime: time.Now().Add(-time.Minute)}
	resolved := notificationDeliveryJob{Type: "webhook", Event: eventResolved, Alerts: []*alerts.Alert{alert}, WebhookConfig: &webhook}
	wait := func(m *NotificationManager, id string, want NotificationQueueStatus, wantAudits int) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for {
			var status string
			var audits int
			if err := m.queue.db.QueryRow("SELECT status FROM notification_queue WHERE id = ?", id).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if err := m.queue.db.QueryRow("SELECT count(*) FROM notification_audit WHERE notification_id = ?", id).Scan(&audits); err != nil {
				t.Fatal(err)
			}
			if status == string(want) && audits == wantAudits {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: status=%s audits=%d, want %s with audit", id, status, audits, want)
			}
			time.Sleep(time.Millisecond)
		}
	}
	enqueue := func(id, kind string, payload *alerts.Alert) {
		t.Helper()
		if err := m.queue.Enqueue(&QueuedNotification{ID: id, Type: kind, Status: QueueStatusPending,
			Config: config, Alerts: []*alerts.Alert{payload}, MaxAttempts: 1}); err != nil {
			t.Fatal(err)
		}
	}
	enqueue("firing", "webhook", alert.Clone())
	wait(m, "firing", QueueStatusSent, 1)
	unavailable.Store(true)
	recovery := alert.Clone()
	annotateResolvedMetadata(recovery, time.Now())
	enqueue("recovery", "webhook_resolved", recovery)
	wait(m, "recovery", QueueStatusDLQ, 1)
	if got := m.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{resolved}); len(got) != 1 {
		t.Fatal("failed recovery consumed the firing receipt")
	}
	m.Stop()
	m = open()
	assertFailureClass := func() {
		t.Helper()
		var got string
		if err := m.queue.db.QueryRow("SELECT failure_class FROM notification_audit WHERE notification_id = 'recovery' AND success = 0").Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != string(class) {
			t.Fatalf("retained failure class = %q, want %q", got, class)
		}
	}
	assertFailureClass()
	if got := m.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{resolved}); len(got) != 1 {
		t.Fatal("restart lost the firing receipt")
	}
	unavailable.Store(false)
	if count, err := m.queue.RetryTerminalFailures(); err != nil || count != 1 {
		t.Fatalf("retry = %d, %v; want one recovery", count, err)
	}
	wait(m, "recovery", QueueStatusSent, 2)
	if got := m.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{resolved}); len(got) != 0 {
		t.Fatal("successful recovery did not consume the firing receipt")
	}
	if got := accepted.Load(); got != 2 {
		t.Fatalf("accepted HTTP requests = %d, want firing and recovery only", got)
	}
	var failures, successes int
	if err := m.queue.db.QueryRow("SELECT count(*) FROM notification_audit WHERE notification_id = 'recovery' AND success = 0").Scan(&failures); err != nil {
		t.Fatal(err)
	}
	if err := m.queue.db.QueryRow("SELECT count(*) FROM notification_audit WHERE notification_id = 'recovery' AND success = 1").Scan(&successes); err != nil {
		t.Fatal(err)
	}
	if failures != 1 || successes != 1 {
		t.Fatalf("recovery audit failures=%d successes=%d, want 1 each", failures, successes)
	}
	assertFailureClass()
}
