package notifications

// Regression from the independent retention experiment. Only delivered_at is
// backdated; real cleanup, persistent reopen and actual HTTP delivery decide
// whether the owed recovery survives. This is age simulation, not a 31-day run.

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

type retentionProofReceipt struct {
	Event  string         `json:"event"`
	Alerts []alerts.Alert `json:"alerts"`
}

func TestReceiptCleanupPreservesRecoveryOfLongActiveFiring(t *testing.T) {
	for _, tc := range []struct {
		name           string
		receiptAgeDays int
	}{
		{name: "control receipt one day old", receiptAgeDays: 1},
		{name: "receipt older than thirty days while occurrence still active", receiptAgeDays: 31},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var got []retentionProofReceipt
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				var receipt retentionProofReceipt
				if err := json.NewDecoder(r.Body).Decode(&receipt); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				mu.Lock()
				got = append(got, receipt)
				mu.Unlock()
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			count := func(event string) int {
				mu.Lock()
				defer mu.Unlock()
				n := 0
				for _, receipt := range got {
					if receipt.Event == event {
						n++
					}
				}
				return n
			}
			dataDir := t.TempDir()
			webhook := WebhookConfig{ID: "ops", Name: "ops", URL: server.URL, Enabled: true, Service: "generic"}
			open := func() *NotificationManager {
				m := NewNotificationManagerWithDataDir("", dataDir)
				m.webhookClient = server.Client()
				if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
					t.Fatal(err)
				}
				m.SetGroupingConfig(false, 0, false, false)
				m.SetCooldown(30)
				m.SetNotifyOnResolve(true)
				m.AddWebhook(webhook)
				return m
			}
			drain := func(m *NotificationManager) {
				deadline := time.Now().Add(10 * time.Second)
				for {
					stats, err := m.GetQueueStats()
					if err != nil {
						t.Fatal(err)
					}
					if stats["pending"]+stats["sending"] == 0 {
						return
					}
					if time.Now().After(deadline) {
						t.Fatalf("queue did not drain: %v", stats)
					}
					time.Sleep(5 * time.Millisecond)
				}
			}
			alert := &alerts.Alert{
				ID: "storage-pbs-primary-backups-usage", Type: "usage", Level: alerts.AlertLevelCritical,
				ResourceID: "pbs-primary-backups", ResourceName: "backups", Node: "pbs-primary",
				Message: "Storage usage 95%", Value: 95, Threshold: 80,
				StartTime: time.Now().Add(-time.Duration(tc.receiptAgeDays) * 24 * time.Hour).Truncate(time.Second),
			}

			m := open()
			m.SendAlert(alert.Clone())
			deadline := time.Now().Add(10 * time.Second)
			for count("") < 1 {
				if time.Now().After(deadline) {
					t.Fatal("firing not received")
				}
				time.Sleep(5 * time.Millisecond)
			}
			drain(m)
			q := m.GetQueue()
			if ok, err := q.HasDeliveryReceipt(alert.ID, alert.StartTime, notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &webhook})); err != nil || !ok {
				t.Fatalf("firing receipt missing after delivery: ok=%v err=%v", ok, err)
			}

			// Elapsed-time simulation: only the receipt's delivered_at is aged.
			deliveredAt := time.Now().Add(-time.Duration(tc.receiptAgeDays) * 24 * time.Hour).Unix()
			q.mu.Lock()
			_, err := q.db.Exec(`UPDATE notification_delivery_receipts SET delivered_at = ? WHERE alert_id = ?`, deliveredAt, alert.ID)
			q.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			q.performCleanup() // the real hourly cleanup routine
			var remaining int
			q.mu.RLock()
			_ = q.db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts WHERE alert_id = ?`, alert.ID).Scan(&remaining)
			q.mu.RUnlock()
			t.Logf("receipt age %dd: receipts remaining after cleanup = %d (occurrence still active, no repeat sent)", tc.receiptAgeDays, remaining)

			// The occurrence stays active across a graceful reopen, then recovers.
			m.Stop()
			m = open()
			t.Cleanup(m.Stop)
			m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: alert.Clone(), ResolvedTime: time.Now()})
			drain(m)
			deadline = time.Now().Add(3 * time.Second)
			for count("resolved") < 1 && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			t.Logf("receipt age %dd: firing deliveries = %d, recovery deliveries = %d", tc.receiptAgeDays, count(""), count("resolved"))
			if count("resolved") != 1 {
				t.Errorf("FINDING: destination that received the firing did not receive its recovery (receipt age %dd)", tc.receiptAgeDays)
			}
		})
	}
}
