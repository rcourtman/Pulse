package unifiedresources

import (
	"strings"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// MonitorAdapter exposes a ResourceRegistry through the monitoring
// package's legacy resource-store contract.
type MonitorAdapter struct {
	registry *ResourceRegistry

	mu           sync.RWMutex
	activeAlerts []models.Alert
}

// NewMonitorAdapter creates a monitor-facing adapter around a registry.
// If registry is nil, a new in-memory registry is created.
func NewMonitorAdapter(registry *ResourceRegistry) *MonitorAdapter {
	if registry == nil {
		registry = NewRegistry(nil)
	}

	return &MonitorAdapter{
		registry: registry,
	}
}

// ShouldSkipAPIPolling returns true when agent coverage indicates API
// polling for the hostname should be skipped entirely.
func (a *MonitorAdapter) ShouldSkipAPIPolling(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "" {
		return false
	}

	multiplier, ok := a.GetPollingRecommendations()[hostname]
	if !ok {
		return false
	}
	return multiplier == 0
}

// GetPollingRecommendations returns hostname -> polling multiplier.
// 0 means skip API polling; 0.5 means reduced frequency.
func (a *MonitorAdapter) GetPollingRecommendations() map[string]float64 {
	recommendations := make(map[string]float64)

	for _, resource := range a.GetAll() {
		if resource.SourceType != LegacySourceAgent && resource.SourceType != LegacySourceHybrid {
			continue
		}

		hostname := ""
		if resource.Identity != nil {
			hostname = strings.TrimSpace(resource.Identity.Hostname)
		}
		if hostname == "" {
			hostname = strings.TrimSpace(resource.Name)
		}
		if hostname == "" {
			continue
		}

		key := strings.ToLower(hostname)
		if resource.SourceType == LegacySourceHybrid {
			if existing, exists := recommendations[key]; !exists || existing != 0 {
				recommendations[key] = 0.5
			}
			continue
		}
		recommendations[key] = 0
	}

	return recommendations
}

// GetAll returns all resources in legacy shape for monitor broadcast usage.
func (a *MonitorAdapter) GetAll() []LegacyResource {
	if a.registry == nil {
		return nil
	}

	a.mu.RLock()
	alerts := append([]models.Alert(nil), a.activeAlerts...)
	a.mu.RUnlock()

	return convertRegistryToLegacy(a.registry, alerts)
}

// PopulateFromSnapshot ingests a fresh state snapshot into the registry.
func (a *MonitorAdapter) PopulateFromSnapshot(snapshot models.StateSnapshot) {
	if a.registry == nil {
		return
	}

	a.registry.IngestSnapshot(snapshot)

	a.mu.Lock()
	a.activeAlerts = append([]models.Alert(nil), snapshot.ActiveAlerts...)
	a.mu.Unlock()
}
