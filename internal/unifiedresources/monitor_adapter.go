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
		sourceType := monitorSourceType(resource.Sources)
		if sourceType != "agent" && sourceType != "hybrid" {
			continue
		}

		hostname := monitorHostname(resource)
		if hostname == "" {
			hostname = strings.TrimSpace(resource.Name)
		}
		if hostname == "" {
			continue
		}

		key := strings.ToLower(hostname)
		if sourceType == "hybrid" {
			if existing, exists := recommendations[key]; !exists || existing != 0 {
				recommendations[key] = 0.5
			}
			continue
		}
		recommendations[key] = 0
	}

	return recommendations
}

// GetAll returns all unified resources for monitor broadcast usage.
func (a *MonitorAdapter) GetAll() []Resource {
	if a.registry == nil {
		return nil
	}

	return a.registry.List()
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

// PopulateSupplementalRecords ingests source-native records emitted outside the
// legacy state snapshot pipeline.
func (a *MonitorAdapter) PopulateSupplementalRecords(source DataSource, records []IngestRecord) {
	if a.registry == nil || len(records) == 0 || strings.TrimSpace(string(source)) == "" {
		return
	}
	a.registry.IngestRecords(source, records)
}

func monitorSourceType(sources []DataSource) string {
	if len(sources) > 1 {
		return "hybrid"
	}
	if len(sources) == 1 {
		switch sources[0] {
		case SourceAgent, SourceDocker, SourceK8s:
			return "agent"
		default:
			return "api"
		}
	}
	return "api"
}

func monitorHostname(resource Resource) string {
	if resource.Agent != nil {
		if hostname := strings.TrimSpace(resource.Agent.Hostname); hostname != "" {
			return hostname
		}
	}
	if resource.Docker != nil {
		if hostname := strings.TrimSpace(resource.Docker.Hostname); hostname != "" {
			return hostname
		}
	}
	if resource.Proxmox != nil {
		if hostname := strings.TrimSpace(resource.Proxmox.NodeName); hostname != "" {
			return hostname
		}
	}
	for _, hostname := range resource.Identity.Hostnames {
		if trimmed := strings.TrimSpace(hostname); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
