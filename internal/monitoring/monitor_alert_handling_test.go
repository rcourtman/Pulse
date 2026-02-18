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
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

func TestMonitor_HandleAlertFired_Extra(t *testing.T) {
	// 1. Alert is nil
	m1 := &Monitor{}
	m1.handleAlertFired(nil) // Should return safely

	// 2. Alert is not nil, with Hub and NotificationMgr
	hub := websocket.NewHub(nil)
	notifMgr := notifications.NewNotificationManager("dummy")

	// mock incidentStore - but it is an interface or struct?
	// In monitor.go: func (m *Monitor) GetIncidentStore() *incidents.Store
	// It's a pointer to struct, so hard to mock unless we set it to nil or real store.
	// We can set it to nil for this test to avoid disk I/O.

	m2 := &Monitor{
		wsHub:           hub,
		notificationMgr: notifMgr,
		incidentStore:   nil,
	}

	alert := &alerts.Alert{
		ID:    "test-alert",
		Level: alerts.AlertLevelWarning,
	}

	m2.handleAlertFired(alert)
	// We are just verifying it doesn't crash and calls methods.
	// Hub doesn't expose way to check broadcasts easily without client.
	// NotificationMgr might spin up goroutine.
}

func TestMonitor_HandleAlertResolved_Detailed_Extra(t *testing.T) {
	// 1. With Hub and NotificationMgr and Resolve Notify ON
	hub := websocket.NewHub(nil)
	notifMgr := notifications.NewNotificationManager("dummy")

	// Enable resolve notifications
	// Notifications config needs to be updated?
	// notificationMgr.GetNotifyOnResolve() reads config.
	// But NotificationManager struct doesn't export Config update easily without SetConfig?
	// The constructor initializes defaults.

	m := &Monitor{
		wsHub:           hub,
		notificationMgr: notifMgr,
		alertManager:    alerts.NewManager(),
	}

	// This should run safely
	m.handleAlertResolved("alert-id")
}

func TestHandleAlertResolved_QuietHoursDoesNotSuppressRecovery(t *testing.T) {
	received := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	notifMgr := notifications.NewNotificationManagerWithDataDir("http://pulse.example", t.TempDir())
	if err := notifMgr.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"); err != nil {
		t.Fatalf("UpdateAllowedPrivateCIDRs: %v", err)
	}
	notifMgr.AddWebhook(notifications.WebhookConfig{
		ID:      "test-webhook",
		Name:    "test-webhook",
		URL:     srv.URL,
		Enabled: true,
		Service: "generic",
	})
	notifMgr.SetNotifyOnResolve(true)

	alertMgr := alerts.NewManager()
	cfg := alertMgr.GetConfig()
	cfg.Enabled = true
	cfg.GuestDefaults.PoweredOffSeverity = alerts.AlertLevelWarning
	cfg.Schedule.QuietHours.Enabled = true
	cfg.Schedule.QuietHours.Timezone = "UTC"
	cfg.Schedule.QuietHours.Days = map[string]bool{
		"monday":    true,
		"tuesday":   true,
		"wednesday": true,
		"thursday":  true,
		"friday":    true,
		"saturday":  true,
		"sunday":    true,
	}
	now := time.Now().UTC()
	cfg.Schedule.QuietHours.Start = now.Add(-1 * time.Hour).Format("15:04")
	cfg.Schedule.QuietHours.End = now.Add(1 * time.Hour).Format("15:04")
	alertMgr.UpdateConfig(cfg)

	m := &Monitor{
		alertManager:    alertMgr,
		notificationMgr: notifMgr,
	}
	alertMgr.SetResolvedCallback(m.handleAlertResolved)

	vm := models.VM{
		ID:       "vm-1",
		Name:     "test-vm",
		Node:     "node-1",
		Instance: "inst-1",
		Status:   "stopped",
		Memory:   models.Memory{Usage: 0},
		Disk:     models.Disk{Usage: 0},
	}

	// Two consecutive stopped polls are required to trigger the powered-off alert.
	alertMgr.CheckGuest(vm, vm.Instance)
	alertMgr.CheckGuest(vm, vm.Instance)

	alertID := "guest-powered-off-" + vm.ID

	// Recover while quiet hours are active.
	vm.Status = "running"
	alertMgr.CheckGuest(vm, vm.Instance)

	resolved := alertMgr.GetResolvedAlert(alertID)
	if resolved == nil || resolved.Alert == nil {
		t.Fatalf("expected resolved alert %q to exist", alertID)
	}
	if !alertMgr.ShouldSuppressResolvedNotification(resolved.Alert) {
		t.Fatalf("expected quiet hours suppression to be active for non-critical resolved alert %q (test precondition)", alertID)
	}

	select {
	case body := <-received:
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to parse webhook payload: %v", err)
		}
		if payload["event"] != "resolved" {
			t.Fatalf("expected webhook event=resolved, got %v", payload["event"])
		}
		if payload["alertId"] != alertID {
			t.Fatalf("expected webhook alertId=%q, got %v", alertID, payload["alertId"])
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for resolved notification webhook (should not be suppressed by quiet hours)")
	}
}
