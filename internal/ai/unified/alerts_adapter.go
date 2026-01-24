// Package unified provides a unified alert/finding system.
package unified

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// AlertManagerAdapter adapts alerts.Manager to the AlertProvider interface
type AlertManagerAdapter struct {
	manager *alerts.Manager
}

// NewAlertManagerAdapter creates a new adapter for alerts.Manager
func NewAlertManagerAdapter(manager *alerts.Manager) *AlertManagerAdapter {
	return &AlertManagerAdapter{manager: manager}
}

// GetActiveAlerts returns all currently active alerts as AlertAdapters
func (a *AlertManagerAdapter) GetActiveAlerts() []AlertAdapter {
	if a.manager == nil {
		return nil
	}

	activeAlerts := a.manager.GetActiveAlerts()
	result := make([]AlertAdapter, len(activeAlerts))
	for i := range activeAlerts {
		result[i] = &alertWrapper{alert: &activeAlerts[i]}
	}
	return result
}

// GetAlert returns a specific alert by ID
func (a *AlertManagerAdapter) GetAlert(alertID string) AlertAdapter {
	if a.manager == nil {
		return nil
	}

	// Get all active alerts and find the one with matching ID
	activeAlerts := a.manager.GetActiveAlerts()
	for i := range activeAlerts {
		if activeAlerts[i].ID == alertID {
			return &alertWrapper{alert: &activeAlerts[i]}
		}
	}
	return nil
}

// SetAlertCallback sets the callback for new alerts
func (a *AlertManagerAdapter) SetAlertCallback(cb func(AlertAdapter)) {
	if a.manager == nil {
		return
	}

	a.manager.SetAlertCallback(func(alert *alerts.Alert) {
		if cb != nil && alert != nil {
			cb(&alertWrapper{alert: alert})
		}
	})
}

// SetResolvedCallback sets the callback for resolved alerts
func (a *AlertManagerAdapter) SetResolvedCallback(cb func(alertID string)) {
	if a.manager == nil {
		return
	}

	a.manager.SetResolvedCallback(cb)
}

// alertWrapper wraps an alerts.Alert to implement AlertAdapter
type alertWrapper struct {
	alert *alerts.Alert
}

func (w *alertWrapper) GetAlertID() string {
	if w.alert == nil {
		return ""
	}
	return w.alert.ID
}

func (w *alertWrapper) GetAlertType() string {
	if w.alert == nil {
		return ""
	}
	return w.alert.Type
}

func (w *alertWrapper) GetAlertLevel() string {
	if w.alert == nil {
		return ""
	}
	return string(w.alert.Level)
}

func (w *alertWrapper) GetResourceID() string {
	if w.alert == nil {
		return ""
	}
	return w.alert.ResourceID
}

func (w *alertWrapper) GetResourceName() string {
	if w.alert == nil {
		return ""
	}
	return w.alert.ResourceName
}

func (w *alertWrapper) GetNode() string {
	if w.alert == nil {
		return ""
	}
	return w.alert.Node
}

func (w *alertWrapper) GetMessage() string {
	if w.alert == nil {
		return ""
	}
	return w.alert.Message
}

func (w *alertWrapper) GetValue() float64 {
	if w.alert == nil {
		return 0
	}
	return w.alert.Value
}

func (w *alertWrapper) GetThreshold() float64 {
	if w.alert == nil {
		return 0
	}
	return w.alert.Threshold
}

func (w *alertWrapper) GetStartTime() time.Time {
	if w.alert == nil {
		return time.Time{}
	}
	return w.alert.StartTime
}

func (w *alertWrapper) GetLastSeen() time.Time {
	if w.alert == nil {
		return time.Time{}
	}
	return w.alert.LastSeen
}

func (w *alertWrapper) GetMetadata() map[string]interface{} {
	if w.alert == nil {
		return nil
	}
	return w.alert.Metadata
}
