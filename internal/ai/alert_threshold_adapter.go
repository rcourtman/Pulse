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

// getThreshold is a shared helper that extracts a trigger value from the alert config.
// extract should return the trigger value and true if the config field is present.
func (a *AlertThresholdAdapter) getThreshold(extract func(alerts.AlertConfig) (float64, bool), defaultVal float64) float64 {
	if a.manager == nil {
		return defaultVal
	}
	cfg := a.manager.GetConfig()
	if trigger, ok := extract(cfg); ok && trigger > 0 {
		return trigger
	}
	return defaultVal
}

// GetNodeCPUThreshold returns the CPU alert trigger threshold for nodes (0-100%)
func (a *AlertThresholdAdapter) GetNodeCPUThreshold() float64 {
	return a.getThreshold(func(cfg alerts.AlertConfig) (float64, bool) {
		if cfg.NodeDefaults.CPU != nil {
			return cfg.NodeDefaults.CPU.Trigger, true
		}
		return 0, false
	}, 80)
}

// GetNodeMemoryThreshold returns the memory alert trigger threshold for nodes (0-100%)
func (a *AlertThresholdAdapter) GetNodeMemoryThreshold() float64 {
	return a.getThreshold(func(cfg alerts.AlertConfig) (float64, bool) {
		if cfg.NodeDefaults.Memory != nil {
			return cfg.NodeDefaults.Memory.Trigger, true
		}
		return 0, false
	}, 85)
}

// GetGuestMemoryThreshold returns the memory alert trigger threshold for guests (0-100%)
func (a *AlertThresholdAdapter) GetGuestMemoryThreshold() float64 {
	return a.getThreshold(func(cfg alerts.AlertConfig) (float64, bool) {
		if cfg.GuestDefaults.Memory != nil {
			return cfg.GuestDefaults.Memory.Trigger, true
		}
		return 0, false
	}, 85)
}

// GetGuestDiskThreshold returns the disk alert trigger threshold for guests (0-100%)
func (a *AlertThresholdAdapter) GetGuestDiskThreshold() float64 {
	return a.getThreshold(func(cfg alerts.AlertConfig) (float64, bool) {
		if cfg.GuestDefaults.Disk != nil {
			return cfg.GuestDefaults.Disk.Trigger, true
		}
		return 0, false
	}, 90)
}

// GetStorageThreshold returns the usage alert trigger threshold for storage (0-100%)
func (a *AlertThresholdAdapter) GetStorageThreshold() float64 {
	return a.getThreshold(func(cfg alerts.AlertConfig) (float64, bool) {
		return cfg.StorageDefault.Trigger, true
	}, 85)
}
