package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

// This is local integration proof, not installed-artifact or off-host delivery
// qualification. Both the synthetic PBS and receiver live in this test process.
func TestPBSPartialMetricsWebhookLifecycle(t *testing.T) {
	type payload struct {
		Event  string         `json:"event"`
		Alerts []alerts.Alert `json:"alerts"`
	}
	received := make(chan payload, 16)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p payload
		if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&p) != nil {
			t.Error("invalid webhook request")
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		select {
		case received <- p:
		default:
			t.Error("unexpected webhook flood")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer receiver.Close()
	nm := notifications.NewNotificationManagerWithDataDir("", t.TempDir())
	defer nm.Stop()
	nm.SetGroupingWindow(0)
	nm.SetNotifyOnResolve(true)
	if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	nm.AddWebhook(notifications.WebhookConfig{ID: "local-receiver", Name: "local-receiver", URL: receiver.URL, Enabled: true})

	fixture := newPBSHealthTestServer(t)
	instance := config.PBSInstance{Name: "pbs-webhook", Host: fixture.server.URL, MonitorDatastores: true}
	monitor := newPBSHealthAuthorityMonitor([]config.PBSInstance{instance})
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	defer manager.Stop()
	monitor.alertManager, monitor.notificationMgr = manager, nm
	manager.UpdateConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive,
		TimeThresholds: map[string]int{"pbs": 0}, PBSDefaults: alerts.ThresholdConfig{
			Memory: &alerts.HysteresisThreshold{Trigger: 40, Clear: 30},
		}})
	monitor.wireExternalAlertCallbacks(nil)
	client := newPBSHealthTestClient(t, instance.Host)
	poll := func() { monitor.pollPBSInstance(context.Background(), instance.Name, client) }
	receive := func() payload {
		t.Helper()
		select {
		case p := <-received:
			return p
		case <-time.After(5 * time.Second):
			t.Fatal("webhook not received")
			return payload{}
		}
	}
	// Wait for the queue's completed delivery audit, not merely an HTTP request:
	// recovery eligibility is recorded after the receiver responds successfully.
	waitSent := func(want int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			entries, err := nm.GetDeliveryLog(time.Time{}, 20)
			if err != nil {
				t.Fatal(err)
			}
			sent := 0
			for _, e := range entries {
				if e.Success {
					sent++
				}
			}
			if sent == want {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("did not record %d successful deliveries", want)
	}
	poll()
	firing := receive()
	if len(firing.Alerts) != 1 || firing.Alerts[0].Type != "memory" || firing.Event == "resolved" {
		t.Fatalf("unexpected firing payload: %+v", firing)
	}
	incident := firing.Alerts[0]
	waitSent(1)
	for _, mode := range []pbsHealthTestMode{pbsHealthTestNodeDenied, pbsHealthTestNodeGatewayFailure} {
		fixture.setMode(mode)
		for range 5 {
			poll()
		}
		active := manager.GetActiveAlerts()
		if len(active) != 1 || active[0].ID != incident.ID || !active[0].StartTime.Equal(incident.StartTime) {
			t.Fatal("partial failure changed the active incident")
		}
		if len(manager.GetRecentlyResolved()) != 0 {
			t.Fatal("partial failure fabricated recovery history")
		}
		select {
		case p := <-received:
			t.Fatalf("partial failure sent webhook: %+v", p)
		case <-time.After(150 * time.Millisecond):
		}
	}
	fixture.setMode(pbsHealthTestLowMemory)
	for range 5 {
		poll()
	}
	recovery := receive()
	if recovery.Event != "resolved" || len(recovery.Alerts) != 1 ||
		recovery.Alerts[0].ID != incident.ID || !recovery.Alerts[0].StartTime.Equal(incident.StartTime) {
		t.Fatalf("recovery does not identify delivered incident: %+v", recovery)
	}
	waitSent(2)
	if len(manager.GetActiveAlerts()) != 0 || len(manager.GetRecentlyResolved()) != 1 {
		t.Fatal("genuine recovery did not update active/history state")
	}
	for range 5 {
		poll()
	}
	select {
	case p := <-received:
		t.Fatalf("duplicate webhook after recovery: %+v", p)
	case <-time.After(150 * time.Millisecond):
	}
}
