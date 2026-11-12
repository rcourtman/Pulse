package notifications

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/require"
)

// Drive the actual threshold producer through ordinary queued delivery and
// recovery. A failed earlier consumer must not lose or rewrite the alert that
// the notification subscriber admits. No provider or installed estate is used.
func TestAlertSubscriberIsolationPreservesQueuedFiringAndRecovery(t *testing.T) {
	for _, fault := range []string{"panic", "mutation", "mutation-and-panic"} {
		t.Run(fault, func(t *testing.T) {
			type message struct {
				Event  string          `json:"event"`
				Alerts []*alerts.Alert `json:"alerts"`
			}
			received := make(chan message, 4)
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				var msg message
				if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				select {
				case received <- msg:
				default:
					t.Error("unexpected excess notifications")
				}
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(server.Close)
			hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
			n := NewNotificationManagerWithDataDir("", t.TempDir())
			t.Cleanup(n.Stop)
			n.SetGroupingConfig(false, 0, false, false)
			n.SetNotifyOnResolve(true)
			require.NoError(t, n.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
			n.AddWebhook(hook)

			producer := alerts.NewManagerWithDataDir(t.TempDir())
			t.Cleanup(producer.Stop) // Stop producer callbacks before their consumer.
			cfg := producer.GetConfig()
			cfg.ActivationState = alerts.ActivationActive
			cfg.TimeThresholds["node"] = 0
			producer.UpdateConfig(cfg)
			producer.SetAlertCallback(func(snapshot *alerts.Alert) {
				if fault != "panic" {
					snapshot.ID = "wrong-occurrence"
					snapshot.ResourceName = "wrong-node"
					snapshot.Metadata["subscriber-edit"] = true
				}
				if fault != "mutation" {
					panic("synthetic earlier-consumer panic")
				}
			})
			producer.SubscribeAlertCallback(n.SendAlert)
			producer.SetResolvedAlertCallback(n.SendResolvedAlert)
			node := models.Node{ID: "fanout-node", Name: "original-node", Status: "online", CPU: .99}
			producer.CheckNode(node)
			active := producer.GetActiveAlerts()
			require.Len(t, active, 1)
			original := active[0]
			waitReceipt := func(want bool) {
				t.Helper()
				job := notificationDeliveryJob{Type: "webhook", Event: eventResolved,
					Alerts: []*alerts.Alert{&original}, WebhookConfig: &hook}
				require.Eventually(t, func() bool {
					return (len(n.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{job})) == 1) == want
				}, 5*time.Second, time.Millisecond, "original occurrence receipt present=%t", want)
			}
			next := func(event string) {
				t.Helper()
				select {
				case msg := <-received:
					require.Equal(t, event, msg.Event)
					require.Len(t, msg.Alerts, 1)
					require.Equal(t, original.ID, msg.Alerts[0].ID)
					require.Equal(t, original.ResourceName, msg.Alerts[0].ResourceName)
					require.True(t, original.StartTime.Equal(msg.Alerts[0].StartTime))
					require.NotContains(t, msg.Alerts[0].Metadata, "subscriber-edit")
				case <-time.After(5 * time.Second):
					t.Fatalf("ordinary %s did not reach the destination after the earlier consumer's %s", event, fault)
				}
			}
			next("alert")
			waitReceipt(true)
			node.CPU = .20
			producer.CheckNode(node)
			require.Empty(t, producer.GetActiveAlerts())
			next("resolved")
			waitReceipt(false)
			producer.Stop()
			n.Stop()
			select {
			case extra := <-received:
				t.Fatalf("duplicate notification after draining: %+v", extra)
			default:
			}
		})
	}
}
