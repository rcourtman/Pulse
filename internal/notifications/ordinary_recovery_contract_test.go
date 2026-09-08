package notifications

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Exercise normal notification entry points, not manually inserted queue jobs or
// manufactured successful-delivery receipts. This is loopback source acceptance,
// not installed alert-engine or real-provider qualification.
func TestOrdinaryNotificationRecoveryUsesDeliveredReceipt(t *testing.T) {
	for _, restart := range []bool{false, true} {
		name := "same_process"
		if restart {
			name = "manager_restart"
		}
		t.Run(name, func(t *testing.T) {
			type message struct{ title, body string }
			received := make(chan message, 8)
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read notification: %v", err)
				}
				select {
				case received <- message{r.Header.Get("Title"), string(body)}:
				default:
					t.Error("unexpected excess notifications")
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
				m.SetGroupingConfig(false, 0, false, false)
				m.SetNotifyOnResolve(true)
				m.AddWebhook(webhook)
				t.Cleanup(m.Stop)
				return m
			}
			m := open()
			alert := &alerts.Alert{ID: "ordinary-cpu", ResourceName: "ordinary-node", Type: "cpu",
				Level: alerts.AlertLevelCritical, Message: "CPU above threshold", StartTime: time.Now().Add(-time.Minute)}
			recoveryJob := notificationDeliveryJob{Type: "webhook", Event: eventResolved, Alerts: []*alerts.Alert{alert}, WebhookConfig: &webhook}
			waitReceipt := func(want bool) {
				t.Helper()
				deadline := time.Now().Add(10 * time.Second)
				for {
					got := len(m.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{recoveryJob})) == 1
					if got == want {
						return
					}
					if time.Now().After(deadline) {
						t.Fatalf("recovery receipt present=%t, want %t", got, want)
					}
					time.Sleep(time.Millisecond)
				}
			}
			next := func() message {
				t.Helper()
				select {
				case msg := <-received:
					return msg
				case <-time.After(10 * time.Second):
					t.Fatal("notification not received")
					return message{}
				}
			}
			m.SendAlert(alert)
			firing := next()
			if !strings.Contains(firing.title, alert.ResourceName) || !strings.Contains(firing.body, alert.Message) {
				t.Fatalf("incorrect firing content: %+v", firing)
			}
			waitReceipt(true)
			if restart {
				m.Stop()
				m = open()
				waitReceipt(true)
			}
			// This is the cancellation gate used before ordinary recovery dispatch.
			if m.CancelAlert(alert.ID) {
				t.Fatal("delivered firing incorrectly treated as never delivered")
			}
			m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: alert, ResolvedTime: time.Now()})
			recovery := next()
			if recovery.title != "RESOLVED: ordinary-node" || !strings.Contains(recovery.body, "is now healthy") {
				t.Fatalf("incorrect recovery content: %+v", recovery)
			}
			waitReceipt(false)
			m.Stop()
			select {
			case extra := <-received:
				t.Fatalf("unexpected notification: %+v", extra)
			default:
			}
		})
	}
}
