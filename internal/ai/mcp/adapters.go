package mcp

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// AlertManagerMCPAdapter adapts alerts.Manager to MCP AlertProvider interface
type AlertManagerMCPAdapter struct {
	manager AlertManager
}

// AlertManager interface matches what alerts.Manager provides
type AlertManager interface {
	GetActiveAlerts() []alerts.Alert
}

// NewAlertManagerMCPAdapter creates a new adapter for alert manager
func NewAlertManagerMCPAdapter(manager AlertManager) *AlertManagerMCPAdapter {
	if manager == nil {
		return nil
	}
	return &AlertManagerMCPAdapter{manager: manager}
}

// GetActiveAlerts implements mcp.AlertProvider
func (a *AlertManagerMCPAdapter) GetActiveAlerts() []ActiveAlert {
	if a.manager == nil {
		return nil
	}

	activeAlerts := a.manager.GetActiveAlerts()
	result := make([]ActiveAlert, 0, len(activeAlerts))

	for _, alert := range activeAlerts {
		result = append(result, ActiveAlert{
			ID:           alert.ID,
			ResourceID:   alert.ResourceID,
			ResourceName: alert.ResourceName,
			Type:         alert.Type,
			Severity:     string(alert.Level),
			Value:        alert.Value,
			Threshold:    alert.Threshold,
			StartTime:    alert.StartTime,
			Message:      alert.Message,
		})
	}

	return result
}
