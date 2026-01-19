package api

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

// AlertMonitorWrapper wraps *monitoring.Monitor to satisfy AlertMonitor interface.
type AlertMonitorWrapper struct {
	m *monitoring.Monitor
}

// NewAlertMonitorWrapper creates a new wrapper for AlertMonitor.
func NewAlertMonitorWrapper(m *monitoring.Monitor) AlertMonitor {
	if m == nil {
		return nil
	}
	return &AlertMonitorWrapper{m: m}
}

func (w *AlertMonitorWrapper) GetAlertManager() AlertManager {
	return w.m.GetAlertManager()
}

func (w *AlertMonitorWrapper) GetConfigPersistence() ConfigPersistence {
	return w.m.GetConfigPersistence()
}

func (w *AlertMonitorWrapper) GetIncidentStore() *memory.IncidentStore {
	return w.m.GetIncidentStore()
}

func (w *AlertMonitorWrapper) GetNotificationManager() *notifications.NotificationManager {
	return w.m.GetNotificationManager()
}

func (w *AlertMonitorWrapper) SyncAlertState() {
	w.m.SyncAlertState()
}

func (w *AlertMonitorWrapper) GetState() models.StateSnapshot {
	return w.m.GetState()
}

// NotificationMonitorWrapper wraps *monitoring.Monitor to satisfy NotificationMonitor interface.
type NotificationMonitorWrapper struct {
	m *monitoring.Monitor
}

// NewNotificationMonitorWrapper creates a new wrapper for NotificationMonitor.
func NewNotificationMonitorWrapper(m *monitoring.Monitor) NotificationMonitor {
	if m == nil {
		return nil
	}
	return &NotificationMonitorWrapper{m: m}
}

func (w *NotificationMonitorWrapper) GetNotificationManager() NotificationManager {
	return w.m.GetNotificationManager()
}

func (w *NotificationMonitorWrapper) GetConfigPersistence() NotificationConfigPersistence {
	return w.m.GetConfigPersistence()
}

func (w *NotificationMonitorWrapper) GetState() models.StateSnapshot {
	return w.m.GetState()
}
