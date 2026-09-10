package notifications

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Initial routing changes apply to new sends, not already admitted work.
// Resolution, independently, must retire that old firing work before retry.
func TestQueuedEmailRoutingChangeAndResolution(t *testing.T) {
	for _, resolve := range []bool{false, true} {
		name := "still-active"
		if resolve {
			name = "resolved"
		}
		t.Run(name, func(t *testing.T) {
			var deliveries int32
			stubSMTPDialSuccess(t, &deliveries)
			q, err := NewNotificationQueue(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer q.Stop()
			n := &NotificationManager{enabled: true, queue: q, initialTarget: notificationDeliveryTargetEmail,
				emailConfig:  EmailConfig{Enabled: true, SMTPHost: "smtp.example.com", SMTPPort: 25, From: "pulse@example.test", To: []string{"ops@example.test"}},
				lastNotified: make(map[string]notificationRecord), deliveryReceipts: make(map[string]struct{}),
			}
			alert := &alerts.Alert{ID: "routing-change", ResourceName: "test-node", Level: alerts.AlertLevelWarning, StartTime: time.Now().Add(-time.Minute)}
			n.SendAlert(alert)
			pending, err := q.GetPending(10)
			if err != nil || len(pending) != 1 || pending[0].Type != "email" {
				t.Fatalf("queued email = %v, %v", pending, err)
			}
			old := pending[0]
			n.SetInitialNotifyTarget("webhook")
			// A fetched worker item must not defeat resolution cancellation.
			if resolve {
				n.CancelAlert(alert.ID)
			}
			q.SetProcessor(n.ProcessQueuedNotification)
			q.processNotification(old)
			want := int32(1)
			if resolve {
				want = 0
			}
			if got := atomic.LoadInt32(&deliveries); got != want {
				t.Fatalf("SMTP acceptances = %d, want %d", got, want)
			}
		})
	}
}
