package notifications

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// The recovery guide must not imply that a successful Test after editing a
// destination repairs retained work. Exercise enqueue, actual HTTP rejection,
// edit/Test, persistent restart, operator retry and a new normal alert together.
// This records existing snapshot semantics, not a new replay policy.
func TestQueuedWebhookDestinationEditDoesNotRewriteRetainedDelivery(t *testing.T) {
	for _, policy := range []string{"enabled", "destination-disabled", "removed", "globally-paused"} {
		t.Run(policy, func(t *testing.T) {
			type request struct {
				path, credential, generation string
			}
			var mu sync.Mutex
			var received []request
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				var payload struct {
					Generation string `json:"generation"`
				}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Errorf("invalid request body: %v", err)
				}
				mu.Lock()
				received = append(received, request{r.URL.Path, r.Header.Get("Authorization"), payload.Generation})
				mu.Unlock()
				if r.URL.Path == "/original" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			original := WebhookConfig{ID: "ops", Name: "ops", Enabled: true, Service: "generic", Method: "POST",
				URL: server.URL + "/original", Headers: map[string]string{"Authorization": "Bearer synthetic-original"},
				Template: `{"generation":"original","text":"{{.Message | jsonString}}"}`}
			edited := copyWebhookConfig(original)
			edited.URL = server.URL + "/edited"
			edited.Headers["Authorization"] = "Bearer synthetic-edited"
			edited.Template = `{"generation":"edited","text":"{{.Message | jsonString}}"}`
			dir := t.TempDir()
			open := func(config WebhookConfig) *NotificationManager {
				m := NewNotificationManagerWithDeferredQueue("", dir)
				if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
					t.Fatal(err)
				}
				m.SetGroupingConfig(false, 0, false, false)
				m.AddWebhook(config)
				t.Cleanup(m.Stop)
				return m
			}
			pending := func(m *NotificationManager) *QueuedNotification {
				t.Helper()
				rows, err := m.queue.GetPending(10)
				if err != nil || len(rows) != 1 {
					t.Fatalf("pending = %d, %v; want one newly queued delivery", len(rows), err)
				}
				return rows[0]
			}
			wait := func(m *NotificationManager, id string, want NotificationQueueStatus, wantAudits int) {
				t.Helper()
				deadline := time.Now().Add(10 * time.Second)
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
						t.Fatalf("status=%s audits=%d; want %s/%d", status, audits, want, wantAudits)
					}
					time.Sleep(time.Millisecond)
				}
			}
			m := open(original)
			alert := &alerts.Alert{ID: "before-edit", ResourceName: "test-node", Type: "cpu",
				Level: alerts.AlertLevelWarning, Message: "synthetic alert", StartTime: time.Now().Add(-time.Minute)}
			m.SendAlert(alert)
			old := pending(m)
			originalConfig := append([]byte(nil), old.Config...)
			m.StartQueueProcessing()
			wait(m, old.ID, QueueStatusDLQ, 1)
			if err := m.UpdateWebhook(original.ID, edited); err != nil {
				t.Fatal(err)
			}
			if err := m.SendTestWebhook(m.GetWebhooks()[0]); err != nil {
				t.Fatalf("Test with edited destination: %v", err)
			}
			wait(m, old.ID, QueueStatusDLQ, 1) // Test neither retries nor removes the failure.
			m.Stop()
			m = open(edited)
			var retainedConfig []byte
			if err := m.queue.db.QueryRow("SELECT config FROM notification_queue WHERE id = ?", old.ID).Scan(&retainedConfig); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(retainedConfig, originalConfig) {
				t.Fatal("editing/restarting rewrote retained destination settings")
			}
			fresh := alert.Clone()
			fresh.ID = "after-edit"
			m.SendAlert(fresh)
			newJob := pending(m)
			var newConfig WebhookConfig
			if err := json.Unmarshal(newJob.Config, &newConfig); err != nil {
				t.Fatal(err)
			}
			if newConfig.URL != edited.URL || newConfig.Headers["Authorization"] != edited.Headers["Authorization"] || newConfig.Template != edited.Template {
				t.Fatal("new alert did not capture the edited destination")
			}
			switch policy {
			case "destination-disabled":
				edited.Enabled = false
				if err := m.UpdateWebhook(edited.ID, edited); err != nil {
					t.Fatal(err)
				}
			case "removed":
				if err := m.DeleteWebhook(edited.ID); err != nil {
					t.Fatal(err)
				}
			case "globally-paused":
				m.SetEnabled(false)
			}
			if count, err := m.queue.RetryTerminalFailures(); err != nil || count != 1 {
				t.Fatalf("retry = %d, %v; want one retained failure", count, err)
			}
			m.StartQueueProcessing()
			if policy == "enabled" {
				wait(m, old.ID, QueueStatusDLQ, 2)
				wait(m, newJob.ID, QueueStatusSent, 1)
			} else {
				wait(m, old.ID, QueueStatusCancelled, 1)
				wait(m, newJob.ID, QueueStatusCancelled, 0)
			}
			m.Stop() // Drain before inspecting the complete receiver/audit counts.
			mu.Lock()
			defer mu.Unlock()
			counts := map[request]int{}
			for _, r := range received {
				counts[r]++
			}
			oldRequests, newRequests := 1, 1 // Initial rejection and successful Test.
			if policy == "enabled" {
				oldRequests, newRequests = 2, 2 // Old retry still fails; new normal alert succeeds.
			}
			if len(counts) != 2 || counts[request{"/original", "Bearer synthetic-original", "original"}] != oldRequests ||
				counts[request{"/edited", "Bearer synthetic-edited", "edited"}] != newRequests {
				t.Fatalf("receiver counts = %v; want original=%d edited=%d", counts, oldRequests, newRequests)
			}
			var failed, succeeded int
			audit := open(edited)
			if err := audit.queue.db.QueryRow("SELECT count(*) FROM notification_audit WHERE notification_id = ? AND success = 0", old.ID).Scan(&failed); err != nil {
				t.Fatal(err)
			}
			if err := audit.queue.db.QueryRow("SELECT count(*) FROM notification_audit WHERE notification_id = ? AND success = 1", old.ID).Scan(&succeeded); err != nil {
				t.Fatal(err)
			}
			if failed != oldRequests || succeeded != 0 {
				t.Fatalf("old audit failures=%d successes=%d; want %d/0", failed, succeeded, oldRequests)
			}
			t.Logf("%s: Test/new work uses edited URL/header/template; retained retry keeps original settings and failure history; current disable/removal/pause remains effective", policy)
		})
	}
}
