package monitoring

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

// This is a local HTTP transport receipt, not proof of an installed provider
// delivering to a person's device. Use real monitor callbacks and notification
// rendering rather than invoking the transport directly.
func TestPBSMetricObservationGapNotificationReceipts(t *testing.T) {
	receipts := make(chan []byte, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read webhook: %v", err)
		}
		receipts <- body
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	notifier := notifications.NewNotificationManagerWithDataDir("http://pulse.example", t.TempDir())
	t.Cleanup(notifier.Stop)
	if err := notifier.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"); err != nil {
		t.Fatal(err)
	}
	notifier.AddWebhook(notifications.WebhookConfig{ID: "pbs-receipt", Name: "pbs-receipt", URL: server.URL, Enabled: true, Service: "generic"})
	notifier.SetNotifyOnResolve(true)
	notifier.SetGroupingWindow(0)

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	cfg := manager.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = alerts.ActivationActive
	cfg.Schedule.QuietHours.Enabled = false
	cfg.TimeThresholds["pbs"] = 0
	cfg.PBSDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
	manager.UpdateConfig(cfg)
	monitor := &Monitor{alertManager: manager, notificationMgr: notifier}
	manager.SetAlertCallback(monitor.handleAlertFired)
	manager.SetResolvedCallback(monitor.handleAlertResolved)

	receive := func() []byte {
		t.Helper()
		select {
		case body := <-receipts:
			return body
		case <-time.After(5 * time.Second):
			t.Fatal("no HTTP webhook receipt")
			return nil
		}
	}
	quiet := func() {
		t.Helper()
		select {
		case body := <-receipts:
			t.Fatalf("unexpected webhook: %s", body)
		case <-time.After(200 * time.Millisecond):
		}
	}
	firing := func() string {
		t.Helper()
		var payload struct {
			Grouped bool `json:"grouped"`
			Alerts  []struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"alerts"`
		}
		if err := json.Unmarshal(receive(), &payload); err != nil {
			t.Fatal(err)
		}
		if !payload.Grouped || len(payload.Alerts) != 1 || payload.Alerts[0].Type != "cpu" {
			t.Fatalf("want one CPU firing, got %+v", payload)
		}
		active := manager.GetActiveAlerts()
		if len(active) != 1 || active[0].ID != payload.Alerts[0].ID {
			t.Fatalf("receipt does not match active alert: %+v", active)
		}
		return payload.Alerts[0].ID
	}

	pbs := models.PBSInstance{ID: "pbs-receipt", Name: "backup", Host: "pbs.example.invalid", Status: "online", ConnectionHealth: "healthy", CPU: 99}
	manager.CheckPBS(pbs)
	id := firing()

	// A failed metrics endpoint supplies zero values but is not recovery.
	pbs.CPU = 0
	pbs.NodeMetricsUnavailable = true
	for range 3 {
		manager.CheckPBS(pbs)
	}
	quiet()
	if active := manager.GetActiveAlerts(); len(active) != 1 || active[0].ID != id {
		t.Fatalf("observation gap lost the active incident: %+v", active)
	}

	pbs.NodeMetricsUnavailable = false
	manager.CheckPBS(pbs)
	var recovery struct {
		Event string `json:"event"`
		ID    string `json:"alertIdentifier"`
	}
	if err := json.Unmarshal(receive(), &recovery); err != nil {
		t.Fatal(err)
	}
	if recovery.Event != "resolved" || recovery.ID != id {
		t.Fatalf("wrong recovery receipt: %+v", recovery)
	}
	if active := manager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("healthy sample left active alerts: %+v", active)
	}

	// A distinct breach clears the default two-point minimum-delta spam guard.
	pbs.CPU = 95
	manager.CheckPBS(pbs)
	firing()
	quiet()
}
