package ai

import (
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// AlertManagerAdapter adapts the alerts.Manager to the AI's AlertProvider interface
type AlertManagerAdapter struct {
	manager alertManager
}

type alertManager interface {
	GetActiveAlerts() []alerts.Alert
	GetRecentlyResolved() []models.ResolvedAlert
	ClearAlert(alertID string) bool
}

// NewAlertManagerAdapter creates a new adapter for the alert manager
func NewAlertManagerAdapter(manager alertManager) *AlertManagerAdapter {
	return &AlertManagerAdapter{manager: manager}
}

// GetActiveAlerts returns all currently active alerts
func (a *AlertManagerAdapter) GetActiveAlerts() []AlertInfo {
	if a.manager == nil {
		return nil
	}

	activeAlerts := a.manager.GetActiveAlerts()
	result := make([]AlertInfo, 0, len(activeAlerts))

	for _, alert := range activeAlerts {
		result = append(result, convertAlertFromManager(&alert))
	}

	return result
}

// GetRecentlyResolved returns alerts resolved in the last N minutes
func (a *AlertManagerAdapter) GetRecentlyResolved(minutes int) []ResolvedAlertInfo {
	if a.manager == nil {
		return nil
	}

	resolvedAlerts := a.manager.GetRecentlyResolved()
	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)
	result := make([]ResolvedAlertInfo, 0)

	for _, resolved := range resolvedAlerts {
		if resolved.ResolvedTime.After(cutoff) {
			info := ResolvedAlertInfo{
				AlertInfo:    convertAlertFromModels(&resolved.Alert),
				ResolvedTime: resolved.ResolvedTime,
				Duration:     formatDuration(resolved.ResolvedTime.Sub(resolved.StartTime)),
			}
			result = append(result, info)
		}
	}

	return result
}

// GetAlertsByResource returns active alerts for a specific resource
func (a *AlertManagerAdapter) GetAlertsByResource(resourceID string) []AlertInfo {
	if a.manager == nil {
		return nil
	}

	activeAlerts := a.manager.GetActiveAlerts()
	result := make([]AlertInfo, 0)

	for _, alert := range activeAlerts {
		if alert.ResourceID == resourceID {
			result = append(result, convertAlertFromManager(&alert))
		}
	}

	return result
}

// GetAlertHistory returns historical alerts for a resource
func (a *AlertManagerAdapter) GetAlertHistory(resourceID string, limit int) []ResolvedAlertInfo {
	if a.manager == nil {
		return nil
	}

	// Get from recently resolved and filter by resource
	resolvedAlerts := a.manager.GetRecentlyResolved()
	result := make([]ResolvedAlertInfo, 0)

	for _, resolved := range resolvedAlerts {
		if resolved.ResourceID == resourceID {
			info := ResolvedAlertInfo{
				AlertInfo:    convertAlertFromModels(&resolved.Alert),
				ResolvedTime: resolved.ResolvedTime,
				Duration:     formatDuration(resolved.ResolvedTime.Sub(resolved.StartTime)),
			}
			result = append(result, info)
			if len(result) >= limit {
				break
			}
		}
	}

	return result
}

// convertAlertFromManager converts an alerts.Alert to AI's AlertInfo
func convertAlertFromManager(alert *alerts.Alert) AlertInfo {
	if alert == nil {
		return AlertInfo{}
	}

	resourceType := inferResourceType(alert.Type, alert.Metadata)

	return AlertInfo{
		ID:           alert.ID,
		Type:         alert.Type,
		Level:        string(alert.Level),
		ResourceID:   alert.ResourceID,
		ResourceName: alert.ResourceName,
		ResourceType: resourceType,
		Node:         alert.Node,
		Instance:     alert.Instance,
		Message:      alert.Message,
		Value:        alert.Value,
		Threshold:    alert.Threshold,
		StartTime:    alert.StartTime,
		Duration:     formatDuration(time.Since(alert.StartTime)),
		Acknowledged: alert.Acknowledged,
	}
}

// convertAlertFromModels converts a models.Alert to AI's AlertInfo
func convertAlertFromModels(alert *models.Alert) AlertInfo {
	if alert == nil {
		return AlertInfo{}
	}

	resourceType := inferResourceType(alert.Type, nil)

	return AlertInfo{
		ID:           alert.ID,
		Type:         alert.Type,
		Level:        alert.Level,
		ResourceID:   alert.ResourceID,
		ResourceName: alert.ResourceName,
		ResourceType: resourceType,
		Node:         alert.Node,
		Instance:     alert.Instance,
		Message:      alert.Message,
		Value:        alert.Value,
		Threshold:    alert.Threshold,
		StartTime:    alert.StartTime,
		Duration:     formatDuration(time.Since(alert.StartTime)),
		Acknowledged: alert.Acknowledged,
	}
}

// inferResourceType infers resource type from alert type
func inferResourceType(alertType string, metadata map[string]interface{}) string {
	if metadata != nil {
		if rt, ok := metadata["resourceType"].(string); ok {
			return rt
		}
	}

	switch {
	case alertType == "node_offline" || alertType == "node_cpu" || alertType == "node_memory" || alertType == "node_temperature":
		return "node"
	case alertType == "storage_usage" || alertType == "storage":
		return "storage"
	case alertType == "docker_cpu" || alertType == "docker_memory" || alertType == "docker_restart" || alertType == "docker_offline":
		return "docker"
	case alertType == "host_cpu" || alertType == "host_memory" || alertType == "host_offline" || alertType == "host_disk":
		return "host"
	case alertType == "pmg" || alertType == "pmg_queue" || alertType == "pmg_quarantine":
		return "pmg"
	case alertType == "backup" || alertType == "backup_missing":
		return "backup"
	case alertType == "snapshot" || alertType == "snapshot_age":
		return "snapshot"
	default:
		return "guest"
	}
}

// ResolveAlert clears an active alert. Returns true if the alert was found and cleared.
func (a *AlertManagerAdapter) ResolveAlert(alertID string) bool {
	if a.manager == nil {
		return false
	}
	return a.manager.ClearAlert(alertID)
}

// formatDuration returns a human-readable duration string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "< 1 min"
	} else if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 min"
		}
		return fmt.Sprintf("%d mins", mins)
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if mins > 0 {
			return fmt.Sprintf("%dh %dm", hours, mins)
		}
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	if hours > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}
