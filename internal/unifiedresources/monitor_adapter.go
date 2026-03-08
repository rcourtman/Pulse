package unifiedresources

import (
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// MonitorAdapter exposes a ResourceRegistry through the monitoring
// package's legacy resource-store contract.
type MonitorAdapter struct {
	registry *ResourceRegistry

	mu            sync.RWMutex
	activeAlerts  []models.Alert
	lastRebuiltAt time.Time
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

func (a *MonitorAdapter) currentRegistry() *ResourceRegistry {
	if a == nil {
		return nil
	}

	a.mu.RLock()
	registry := a.registry
	a.mu.RUnlock()
	return registry
}

func (a *MonitorAdapter) replaceRegistry(snapshot models.StateSnapshot, recordsBySource map[DataSource][]IngestRecord) {
	registry := a.currentRegistry()
	if registry == nil {
		return
	}

	rebuilt := NewRegistry(registry.store)
	rebuilt.IngestSnapshot(snapshot)
	for source, records := range recordsBySource {
		if len(records) == 0 || strings.TrimSpace(string(source)) == "" {
			continue
		}
		rebuilt.IngestRecords(source, records)
	}
	rebuiltAt := time.Now().UTC()
	if !snapshot.LastUpdate.IsZero() && snapshot.LastUpdate.After(rebuiltAt) {
		rebuiltAt = snapshot.LastUpdate
	}

	a.mu.Lock()
	a.registry = rebuilt
	a.activeAlerts = append([]models.Alert(nil), snapshot.ActiveAlerts...)
	a.lastRebuiltAt = rebuiltAt
	a.mu.Unlock()
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
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}

	return registry.List()
}

func (a *MonitorAdapter) unifiedAIAdapter() *UnifiedAIAdapter {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return &UnifiedAIAdapter{registry: registry}
}

// GetInfrastructure returns infrastructure resources for unified AI context.
func (a *MonitorAdapter) GetInfrastructure() []Resource {
	adapter := a.unifiedAIAdapter()
	if adapter == nil {
		return nil
	}
	return adapter.GetInfrastructure()
}

// GetWorkloads returns workload resources for unified AI context.
func (a *MonitorAdapter) GetWorkloads() []Resource {
	adapter := a.unifiedAIAdapter()
	if adapter == nil {
		return nil
	}
	return adapter.GetWorkloads()
}

// GetByType returns resources filtered by type for MCP tools.
func (a *MonitorAdapter) GetByType(t ResourceType) []Resource {
	adapter := a.unifiedAIAdapter()
	if adapter == nil {
		return nil
	}
	return adapter.GetByType(t)
}

// GetStats returns aggregate stats for unified AI context.
func (a *MonitorAdapter) GetStats() ResourceStats {
	adapter := a.unifiedAIAdapter()
	if adapter == nil {
		return ResourceStats{
			ByType:   make(map[ResourceType]int),
			ByStatus: make(map[ResourceStatus]int),
			BySource: make(map[DataSource]int),
		}
	}
	return adapter.GetStats()
}

// GetTopByCPU returns top CPU resources for unified AI context.
func (a *MonitorAdapter) GetTopByCPU(limit int, types []ResourceType) []Resource {
	adapter := a.unifiedAIAdapter()
	if adapter == nil {
		return nil
	}
	return adapter.GetTopByCPU(limit, types)
}

// GetTopByMemory returns top memory resources for unified AI context.
func (a *MonitorAdapter) GetTopByMemory(limit int, types []ResourceType) []Resource {
	adapter := a.unifiedAIAdapter()
	if adapter == nil {
		return nil
	}
	return adapter.GetTopByMemory(limit, types)
}

// GetTopByDisk returns top disk resources for unified AI context.
func (a *MonitorAdapter) GetTopByDisk(limit int, types []ResourceType) []Resource {
	adapter := a.unifiedAIAdapter()
	if adapter == nil {
		return nil
	}
	return adapter.GetTopByDisk(limit, types)
}

// GetRelated returns related resources for unified AI context.
func (a *MonitorAdapter) GetRelated(resourceID string) map[string][]Resource {
	adapter := a.unifiedAIAdapter()
	if adapter == nil {
		return map[string][]Resource{}
	}
	return adapter.GetRelated(resourceID)
}

// FindContainerHost resolves a container host for unified AI context.
func (a *MonitorAdapter) FindContainerHost(containerNameOrID string) string {
	adapter := a.unifiedAIAdapter()
	if adapter == nil {
		return ""
	}
	return adapter.FindContainerHost(containerNameOrID)
}

// PopulateFromSnapshot ingests a fresh state snapshot into the registry.
func (a *MonitorAdapter) PopulateFromSnapshot(snapshot models.StateSnapshot) {
	a.replaceRegistry(snapshot, nil)
}

// PopulateSnapshotAndSupplemental atomically rebuilds the registry from a
// snapshot plus source-native supplemental records before exposing the result.
func (a *MonitorAdapter) PopulateSnapshotAndSupplemental(snapshot models.StateSnapshot, recordsBySource map[DataSource][]IngestRecord) {
	a.replaceRegistry(snapshot, recordsBySource)
}

// PopulateSupplementalRecords ingests source-native records emitted outside the
// legacy state snapshot pipeline.
func (a *MonitorAdapter) PopulateSupplementalRecords(source DataSource, records []IngestRecord) {
	registry := a.currentRegistry()
	if registry == nil || len(records) == 0 || strings.TrimSpace(string(source)) == "" {
		return
	}
	registry.IngestRecords(source, records)
	a.mu.Lock()
	a.lastRebuiltAt = time.Now().UTC()
	a.mu.Unlock()
}

// MetricsTargetForResource resolves the history/metrics target for a canonical
// resource currently held by the adapter.
func (a *MonitorAdapter) MetricsTargetForResource(resourceID string) *MetricsTarget {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.MetricsTarget(resourceID)
}

// UnifiedResourceFreshness returns the most recent point at which the adapter
// replaced or mutated its canonical registry contents.
func (a *MonitorAdapter) UnifiedResourceFreshness() time.Time {
	if a == nil {
		return time.Time{}
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastRebuiltAt
}

// VMs returns cached VM views for AI/read-state consumers.
func (a *MonitorAdapter) VMs() []*VMView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.VMs()
}

// Containers returns cached LXC container views for AI/read-state consumers.
func (a *MonitorAdapter) Containers() []*ContainerView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.Containers()
}

// Nodes returns cached Proxmox node views for AI/read-state consumers.
func (a *MonitorAdapter) Nodes() []*NodeView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.Nodes()
}

// Hosts returns cached agent-host views for AI/read-state consumers.
func (a *MonitorAdapter) Hosts() []*HostView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.Hosts()
}

// DockerHosts returns cached Docker host views for AI/read-state consumers.
func (a *MonitorAdapter) DockerHosts() []*DockerHostView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.DockerHosts()
}

// DockerContainers returns cached Docker container views for AI/read-state consumers.
func (a *MonitorAdapter) DockerContainers() []*DockerContainerView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.DockerContainers()
}

// StoragePools returns cached storage pool views for AI/read-state consumers.
func (a *MonitorAdapter) StoragePools() []*StoragePoolView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.StoragePools()
}

// PBSInstances returns cached PBS instance views for AI/read-state consumers.
func (a *MonitorAdapter) PBSInstances() []*PBSInstanceView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.PBSInstances()
}

// PMGInstances returns cached PMG instance views for AI/read-state consumers.
func (a *MonitorAdapter) PMGInstances() []*PMGInstanceView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.PMGInstances()
}

// K8sClusters returns cached Kubernetes cluster views for AI/read-state consumers.
func (a *MonitorAdapter) K8sClusters() []*K8sClusterView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.K8sClusters()
}

// K8sNodes returns cached Kubernetes node views for AI/read-state consumers.
func (a *MonitorAdapter) K8sNodes() []*K8sNodeView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.K8sNodes()
}

// Pods returns cached pod views for AI/read-state consumers.
func (a *MonitorAdapter) Pods() []*PodView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.Pods()
}

// K8sDeployments returns cached deployment views for AI/read-state consumers.
func (a *MonitorAdapter) K8sDeployments() []*K8sDeploymentView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.K8sDeployments()
}

// Workloads returns cached polymorphic workload views for AI/read-state consumers.
func (a *MonitorAdapter) Workloads() []*WorkloadView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.Workloads()
}

// Infrastructure returns cached polymorphic infrastructure views for AI/read-state consumers.
func (a *MonitorAdapter) Infrastructure() []*InfrastructureView {
	registry := a.currentRegistry()
	if registry == nil {
		return nil
	}
	return registry.Infrastructure()
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
