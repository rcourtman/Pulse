package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
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
