package monitoring

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

// Synthetic storage observations and a local receiver exercise the production
// callback/queue path. This does not qualify an installed PBS poller or image.
func TestStorageEmptyRecoveryWebhook(t *testing.T) {
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

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	defer manager.Stop()
	manager.UpdateConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive,
		TimeThresholds: map[string]int{"storage": 0}, StorageDefault: alerts.HysteresisThreshold{Trigger: 80, Clear: 70}})
	monitor := newPBSHealthAuthorityMonitor(nil)
	monitor.alertManager, monitor.notificationMgr = manager, nm
	monitor.wireExternalAlertCallbacks(nil)
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
	storage := models.Storage{ID: "pbs-test-backups", Name: "backups", Type: "pbs", Status: "online", Total: 1000, Used: 850, Free: 150, Usage: 85}
	for range 5 {
		manager.CheckStorage(storage)
	}
	firing := receive()
	if len(firing.Alerts) != 1 || firing.Alerts[0].Type != "usage" || firing.Event == "resolved" {
		t.Fatalf("unexpected firing: %+v", firing)
	}
	incident := firing.Alerts[0]
	waitSent(1)
	// Missing free capacity must not masquerade as a measured empty store.
	storage.Usage, storage.Used, storage.Free = 0, 0, 0
	for range 5 {
		manager.CheckStorage(storage)
	}
	if len(manager.GetActiveAlerts()) != 1 || manager.GetResolvedAlert(incident.ID) != nil {
		t.Fatal("missing capacity fabricated recovery")
	}
	select {
	case p := <-received:
		t.Fatalf("missing capacity sent webhook: %+v", p)
	case <-time.After(150 * time.Millisecond):
	}
	storage.Free = 1000
	for range 5 {
		manager.CheckStorage(storage)
	}
	recovery := receive()
	if recovery.Event != "resolved" || len(recovery.Alerts) != 1 {
		t.Fatalf("unexpected recovery: %+v", recovery)
	}
	resolved := recovery.Alerts[0]
	if resolved.ID != incident.ID || !resolved.StartTime.Equal(incident.StartTime) || resolved.Value != 0 || !resolved.LastSeen.After(incident.LastSeen) || !strings.Contains(resolved.Message, "0.0%") || !strings.HasPrefix(resolved.Message, "Resolved: ") {
		t.Fatalf("incorrect clearing payload: %+v", resolved)
	}
	waitSent(2)
	history := manager.GetResolvedAlert(incident.ID)
	if history == nil || history.Value != 0 || history.Message != resolved.Message {
		t.Fatalf("incorrect history: %+v", history)
	}
	for range 5 {
		manager.CheckStorage(storage)
	}
	select {
	case p := <-received:
		t.Fatalf("duplicate webhook: %+v", p)
	case <-time.After(150 * time.Millisecond):
	}
}
