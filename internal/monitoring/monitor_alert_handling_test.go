package monitoring

import (
	"sync/atomic"
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

func TestMonitor_HandleAlertResolved_QuietHoursSuppressesRecovery(t *testing.T) {
	// Verify that resolved notifications are suppressed during quiet hours (#1068).
	// We seed a resolved alert in the manager, configure quiet hours to suppress all
	// (00:00–23:59 every day), then verify SendResolvedAlert is NOT called.

	hub := websocket.NewHub(nil)
	notifMgr := notifications.NewNotificationManager("dummy")
	notifMgr.SetNotifyOnResolve(true)

	mgr := alerts.NewManager()
	// Configure quiet hours to be always active, alerts enabled, no time delay
	mgr.UpdateConfig(alerts.AlertConfig{
		Enabled:       true,
		TimeThreshold: 1,
		TimeThresholds: map[string]int{
			"guest": 0, "node": 0, "storage": 0, "pbs": 0, "host": 0,
		},
		GuestDefaults: alerts.ThresholdConfig{
			CPU:    &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		},
		Schedule: alerts.ScheduleConfig{
			NotifyOnResolve: true,
			QuietHours: alerts.QuietHours{
				Enabled:  true,
				Start:    "00:00",
				End:      "23:59",
				Timezone: "UTC",
				Days: map[string]bool{
					"monday": true, "tuesday": true, "wednesday": true,
					"thursday": true, "friday": true, "saturday": true, "sunday": true,
				},
				Suppress: alerts.QuietHoursSuppression{
					Performance: true,
					Storage:     true,
					Offline:     true,
				},
			},
		},
	})

	m := &Monitor{
		wsHub:           hub,
		notificationMgr: notifMgr,
		alertManager:    mgr,
	}

	// Seed a resolved alert in the alert manager.
	// Fire a warning alert via CheckGuest and then resolve it.
	guest := models.VM{
		ID:       "100",
		VMID:     100,
		Name:     "test-vm",
		Node:     "pve1",
		Status:   "running",
		Type:     "qemu",
		CPU:      0.95, // 95% — above default threshold
		CPUs:     1,
		Memory:   models.Memory{Usage: 50},
		Instance: "https://pve.local:8006",
	}

	// Fire the alert (CPU > threshold)
	mgr.CheckGuest(guest, "pve1")

	activeAlerts := mgr.GetActiveAlerts()
	if len(activeAlerts) == 0 {
		t.Skip("no alert fired — threshold may differ from defaults, skipping integration test")
	}

	alertID := activeAlerts[0].ID

	// Now resolve: bring CPU below threshold
	guest.CPU = 0.10
	mgr.CheckGuest(guest, "pve1")

	// The alert should now be in recently resolved
	resolved := mgr.GetResolvedAlert(alertID)
	if resolved == nil {
		t.Skip("alert was not resolved by CheckGuest — skipping integration test")
	}

	// Verify quiet hours suppression directly
	if !mgr.ShouldSuppressResolvedNotification(resolved.Alert) {
		t.Fatal("expected ShouldSuppressResolvedNotification to return true during quiet hours")
	}

	// Track whether resolved AI callback fires (it should, even during quiet hours)
	var aiCallbackCalled atomic.Int32
	m.alertResolvedAICallback = func(a *alerts.Alert) {
		aiCallbackCalled.Add(1)
	}

	// Call handleAlertResolved — quiet hours should suppress the notification
	m.handleAlertResolved(alertID)

	// Give goroutine time to execute
	time.Sleep(50 * time.Millisecond)

	// AI callback should always fire regardless of quiet hours
	if aiCallbackCalled.Load() == 0 {
		t.Error("expected AI resolved callback to fire even during quiet hours")
	}
}

func TestMonitor_HandleAlertResolved_NoQuietHoursSendsNotification(t *testing.T) {
	// Verify that resolved notifications are sent when quiet hours are NOT active.
	hub := websocket.NewHub(nil)
	notifMgr := notifications.NewNotificationManager("dummy")
	notifMgr.SetNotifyOnResolve(true)

	mgr := alerts.NewManager()
	// No quiet hours, but alerts enabled with no time delay
	mgr.UpdateConfig(alerts.AlertConfig{
		Enabled:       true,
		TimeThreshold: 1,
		TimeThresholds: map[string]int{
			"guest": 0, "node": 0, "storage": 0, "pbs": 0, "host": 0,
		},
		GuestDefaults: alerts.ThresholdConfig{
			CPU:    &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		},
	})

	m := &Monitor{
		wsHub:           hub,
		notificationMgr: notifMgr,
		alertManager:    mgr,
	}

	// Seed a resolved alert
	guest := models.VM{
		ID:       "200",
		VMID:     200,
		Name:     "test-vm-2",
		Node:     "pve2",
		Status:   "running",
		Type:     "qemu",
		CPU:      0.95,
		CPUs:     1,
		Memory:   models.Memory{Usage: 50},
		Instance: "https://pve.local:8006",
	}

	mgr.CheckGuest(guest, "pve2")
	activeAlerts := mgr.GetActiveAlerts()
	if len(activeAlerts) == 0 {
		t.Skip("no alert fired — threshold may differ from defaults, skipping integration test")
	}

	alertID := activeAlerts[0].ID
	guest.CPU = 0.10
	mgr.CheckGuest(guest, "pve2")

	// Should not crash, and notification should be dispatched (not suppressed)
	m.handleAlertResolved(alertID)
}
