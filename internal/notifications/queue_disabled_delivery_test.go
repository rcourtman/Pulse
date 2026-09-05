package notifications

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// Policy skips are neither provider receipts nor delivery failures. Exercise
// the real queue processor: checking only its return value hid false success.
func TestQueueDisabledDeliveryIsCancelledNotSent(t *testing.T) {
	for _, kind := range []string{"email", "webhook", "apprise"} {
		for _, suffix := range []string{"", "_resolved"} {
			for _, enabled := range []bool{false, true} {
				name := kind + suffix + "/global-disabled"
				if enabled {
					name = kind + suffix + "/destination-disabled"
				}
				t.Run(name, func(t *testing.T) {
					q, err := NewNotificationQueue(t.TempDir())
					if err != nil {
						t.Fatal(err)
					}
					defer q.Stop()
					nm := &NotificationManager{enabled: enabled}
					if !enabled {
						nm.emailConfig.Enabled = true
						nm.appriseConfig.Enabled = true
						nm.webhooks = []WebhookConfig{{ID: "ops", Enabled: true}}
					}
					n := &QueuedNotification{ID: "disabled", Type: kind + suffix, Status: QueueStatusPending,
						Links:  []operationaltrust.NotificationLink{{DestinationID: "ops", OperationalRecordID: "incident", TransitionID: "firing", LifecycleState: operationaltrust.OperationalOpen, CauseKey: "cpu"}},
						Config: []byte(`{"enabled":true,"id":"ops"}`), MaxAttempts: 3, Alerts: []*alerts.Alert{{ID: "incident"}}}
					if err := q.Enqueue(n); err != nil {
						t.Fatal(err)
					}
					// Do not start background workers; process one persisted row synchronously.
					q.processor = nm.ProcessQueuedNotification
					callbacks := 0
					q.SetDeliveryHealthChangedCallback(func() {
						callbacks++
						release := q.acquireAlertDeliveryGates([]string{"incident"}, true)
						defer release()
						if _, err := q.GetQueueStats(); err != nil {
							t.Error(err)
						}
					})
					q.processNotification(n)
					var status string
					var completed *int64
					if err := q.db.QueryRow(`SELECT status, completed_at FROM notification_queue WHERE id = ?`, n.ID).Scan(&status, &completed); err != nil {
						t.Fatal(err)
					}
					if status != string(QueueStatusCancelled) || completed == nil {
						t.Errorf("status=%s completed=%v; want terminal cancellation", status, completed)
					}
					links, err := q.getNotificationLinks(n.ID)
					if err != nil {
						t.Fatal(err)
					}
					if len(links) != 1 || links[0].DeliveryState != operationaltrust.NotificationCancelled {
						t.Errorf("links = %+v, want cancelled", links)
					}
					for _, table := range []string{"notification_audit", "notification_delivery_receipts"} {
						var count int
						if err := q.db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil {
							t.Fatal(err)
						}
						if count != 0 {
							t.Errorf("%s has %d rows for a policy skip", table, count)
						}
					}
					if callbacks != 1 {
						t.Errorf("health callbacks = %d, want 1", callbacks)
					}
					if count, err := q.RetryTerminalFailures(); err != nil || count != 0 {
						t.Errorf("retry = %d, %v; want no replay", count, err)
					}
				})
			}
		}
	}
}
