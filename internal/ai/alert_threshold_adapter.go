package ai

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// AlertThresholdAdapter adapts alerts.Manager to the ThresholdProvider interface
// This allows the patrol service to use user-configured alert thresholds
type AlertThresholdAdapter struct {
	manager *alerts.Manager
}

// NewAlertThresholdAdapter creates a new adapter for the alerts manager
func NewAlertThresholdAdapter(manager *alerts.Manager) *AlertThresholdAdapter {
	return &AlertThresholdAdapter{manager: manager}
}

// GetNodeCPUThreshold returns the CPU alert trigger threshold for nodes (0-100%)
func (a *AlertThresholdAdapter) GetNodeCPUThreshold() float64 {
	if a.manager == nil {
		return 80 // default
	}
	cfg := a.manager.GetConfig()
	if cfg.NodeDefaults.CPU != nil && cfg.NodeDefaults.CPU.Trigger > 0 {
		return cfg.NodeDefaults.CPU.Trigger
	}
	return 80 // default
}

// GetNodeMemoryThreshold returns the memory alert trigger threshold for nodes (0-100%)
func (a *AlertThresholdAdapter) GetNodeMemoryThreshold() float64 {
	if a.manager == nil {
		return 85 // default
	}
	cfg := a.manager.GetConfig()
	if cfg.NodeDefaults.Memory != nil && cfg.NodeDefaults.Memory.Trigger > 0 {
		return cfg.NodeDefaults.Memory.Trigger
	}
	return 85 // default
}

// GetGuestMemoryThreshold returns the memory alert trigger threshold for guests (0-100%)
func (a *AlertThresholdAdapter) GetGuestMemoryThreshold() float64 {
	if a.manager == nil {
		return 85 // default
	}
	cfg := a.manager.GetConfig()
	if cfg.GuestDefaults.Memory != nil && cfg.GuestDefaults.Memory.Trigger > 0 {
		return cfg.GuestDefaults.Memory.Trigger
	}
	return 85 // default
}

// GetGuestDiskThreshold returns the disk alert trigger threshold for guests (0-100%)
func (a *AlertThresholdAdapter) GetGuestDiskThreshold() float64 {
	if a.manager == nil {
		return 90 // default
	}
	cfg := a.manager.GetConfig()
	if cfg.GuestDefaults.Disk != nil && cfg.GuestDefaults.Disk.Trigger > 0 {
		return cfg.GuestDefaults.Disk.Trigger
	}
	return 90 // default
}

// GetStorageThreshold returns the usage alert trigger threshold for storage (0-100%)
func (a *AlertThresholdAdapter) GetStorageThreshold() float64 {
	if a.manager == nil {
		return 85 // default
	}
	cfg := a.manager.GetConfig()
	if cfg.StorageDefault.Trigger > 0 {
		return cfg.StorageDefault.Trigger
	}
	return 85 // default
}
