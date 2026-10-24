package notifications

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

func TestQueuedWebhookEligibilityUsesSavedIDBeforeURL(t *testing.T) {
	original := WebhookConfig{ID: "removed", URL: "https://notify.example.test/topic", Enabled: true}
	sibling := original
	sibling.ID = "replacement"
	idless := original
	idless.ID = ""
	disabled := original
	disabled.Enabled = false
	edited := original
	edited.URL = "https://notify.example.test/new-topic"
	tests := []struct {
		name    string
		queued  WebhookConfig
		current []WebhookConfig
		want    bool
	}{
		{"same identity", original, []WebhookConfig{original}, true},
		{"same identity edited URL preserves snapshot", original, []WebhookConfig{edited}, true},
		{"removed ID cannot borrow sibling URL", original, []WebhookConfig{sibling}, false},
		{"removed ID cannot borrow ID-less URL", original, []WebhookConfig{idless}, false},
		{"disabled ID cannot borrow enabled sibling", original, []WebhookConfig{sibling, disabled}, false},
		{"disabled ID first cannot borrow sibling", original, []WebhookConfig{disabled, sibling}, false},
		{"queued disabled", disabled, []WebhookConfig{original}, false},
		{"empty current list", original, nil, false},
		{"legacy ID-less matching URL", idless, []WebhookConfig{sibling}, true},
		{"legacy ID-less disabled URL", idless, []WebhookConfig{disabled}, false},
		{"legacy ID-less different URL", idless, []WebhookConfig{edited}, false},
		{"legacy missing URL", WebhookConfig{Enabled: true}, []WebhookConfig{sibling}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manager := &NotificationManager{webhooks: tc.current}
			if got := manager.isQueuedWebhookStillEnabled(tc.queued); got != tc.want {
				t.Errorf("eligible=%v, want %v", got, tc.want)
			}
		})
	}
}

// A URL is a transport address, not a destination identity. Removing a known
// destination must revoke its pending and operator-retried work even when a
// different destination uses that same URL, before or after reopening the queue.
func TestQueuedWebhookDeletionDoesNotTransferWorkToSameURL(t *testing.T) {
	for _, service := range []string{"generic", "ntfy"} {
		for _, suffix := range []string{"", "_resolved"} {
			t.Run(service+suffix, func(t *testing.T) {
				var mu sync.Mutex
				var received []string
				server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, r.Body)
					mu.Lock()
					received = append(received, r.Header.Get("Authorization"))
					mu.Unlock()
					if r.Header.Get("Authorization") == "Bearer synthetic-removed" {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					w.WriteHeader(http.StatusAccepted)
				}))
				defer server.Close()
				original := WebhookConfig{ID: "removed", Name: "Removed destination", Enabled: true,
					Service: service, URL: server.URL + "/topic", Headers: map[string]string{"Authorization": "Bearer synthetic-removed"}}
				sibling := copyWebhookConfig(original)
				sibling.ID, sibling.Name = "replacement", "Replacement destination"
				sibling.Headers["Authorization"] = "Bearer synthetic-replacement"
				dir, now := t.TempDir(), time.Now()
				open := func(hooks []WebhookConfig) *NotificationManager {
					q := newQuietReplayQueue(t, dir, &now)
					m := &NotificationManager{enabled: true, queue: q, webhooks: copyWebhookConfigs(hooks),
						webhookClient: server.Client(), webhookRateLimits: make(map[string]*webhookRateLimit),
						lastNotified: make(map[string]notificationRecord), deliveryReceipts: make(map[string]struct{})}
					if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
						t.Fatal(err)
					}
					q.SetProcessor(m.ProcessQueuedNotification)
					return m
				}
				m := open([]WebhookConfig{original, sibling})
				config, err := json.Marshal(original)
				if err != nil {
					t.Fatal(err)
				}
				for _, id := range []string{"waiting", "previous-failure"} {
					a := quietReplayAlert(id, alerts.AlertLevelWarning, now.Add(-time.Minute))
					n := &QueuedNotification{ID: id, Type: "webhook" + suffix, Status: QueueStatusPending,
						Config: config, MaxAttempts: 1, DestinationID: "webhook:removed", Alerts: []*alerts.Alert{a}}
					if err := m.queue.Enqueue(n); err != nil {
						t.Fatal(err)
					}
				}
				m.queue.processNotification(loadQuietReplay(t, m.queue, "previous-failure"))
				if row := loadQuietReplay(t, m.queue, "previous-failure"); row.Status != QueueStatusDLQ {
					t.Fatalf("initial status=%s", row.Status)
				}
				if err := m.DeleteWebhook(original.ID); err != nil {
					t.Fatal(err)
				}
				saved := m.GetWebhooks()
				if err := m.queue.db.Close(); err != nil {
					t.Fatal(err)
				}
				m = open(saved)
				if count, err := m.queue.RetryTerminalFailures(); err != nil || count != 1 {
					t.Fatalf("retry=%d, %v; want 1", count, err)
				}
				callbacks := 0
				m.queue.SetDeliveryHealthChangedCallback(func() { callbacks++ })
				for _, id := range []string{"waiting", "previous-failure"} {
					m.queue.processNotification(loadQuietReplay(t, m.queue, id))
					row := loadQuietReplay(t, m.queue, id)
					if row.Status != QueueStatusCancelled || row.CompletedAt == nil {
						t.Errorf("%s: status=%s completed=%v; want cancellation", id, row.Status, row.CompletedAt)
					}
					if !bytes.Equal(row.Config, config) {
						t.Errorf("%s: retained configuration rewritten", id)
					}
					if len(row.Links) != 1 || row.Links[0].DeliveryState != operationaltrust.NotificationCancelled || row.Links[0].DestinationID != "webhook:removed" {
						t.Errorf("%s: cancellation lost original link: %+v", id, row.Links)
					}
					var successes, audits, receipts int
					for query, into := range map[string]*int{
						"SELECT count(*) FROM notification_audit WHERE notification_id = ?":      &audits,
						"SELECT count(*) FROM notification_delivery_receipts WHERE alert_id = ?": &receipts,
					} {
						if err := m.queue.db.QueryRow(query, id).Scan(into); err != nil {
							t.Fatal(err)
						}
					}
					if err := m.queue.db.QueryRow("SELECT count(*) FROM notification_audit WHERE notification_id = ? AND success = 1", id).Scan(&successes); err != nil {
						t.Fatal(err)
					}
					wantAudits := 0
					if id == "previous-failure" {
						wantAudits = 1
					}
					if successes != 0 || audits != wantAudits || receipts != 0 {
						t.Errorf("%s: audit=%d successes=%d receipts=%d; want %d/0/0", id, audits, successes, receipts, wantAudits)
					}
				}
				if callbacks != 2 {
					t.Errorf("health callbacks=%d, want 2", callbacks)
				}
				if count, err := m.queue.RetryTerminalFailures(); err != nil || count != 0 {
					t.Errorf("cancelled work retried: %d, %v", count, err)
				}
				stats, err := m.queue.GetQueueStats()
				if err != nil || stats["cancelled"] != 2 || stats["sent"] != 0 || !m.DeliveryHealth().Healthy {
					t.Errorf("post-deletion stats=%v error=%v health=%+v", stats, err, m.DeliveryHealth())
				}
				// Normal work for the still-enabled destination must continue.
				m.SendAlert(quietReplayAlert("new-incident", alerts.AlertLevelWarning, now))
				pending, err := m.queue.GetPending(10)
				if err != nil || len(pending) != 1 {
					t.Fatalf("fresh pending=%d, %v", len(pending), err)
				}
				m.queue.processNotification(pending[0])
				if row := loadQuietReplay(t, m.queue, pending[0].ID); row.Status != QueueStatusSent {
					t.Errorf("replacement's new work=%s, want sent", row.Status)
				}
				mu.Lock()
				actual := append([]string(nil), received...)
				mu.Unlock()
				if want := []string{"Bearer synthetic-removed", "Bearer synthetic-replacement"}; !reflect.DeepEqual(actual, want) {
					t.Errorf("receiver credentials=%v, want %v", actual, want)
				}
				t.Log("original failure retained; deleted pending/retried work cancelled across restart without requests, audits or success receipts; new replacement work delivered")
			})
		}
	}
}
