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

func (a *MonitorAdapter) unifiedAIAdapter() *UnifiedAIAdapter {
	if a == nil || a.registry == nil {
		return nil
	}
	return &UnifiedAIAdapter{registry: a.registry}
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

// VMs returns cached VM views for AI/read-state consumers.
func (a *MonitorAdapter) VMs() []*VMView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.VMs()
}

// Containers returns cached LXC container views for AI/read-state consumers.
func (a *MonitorAdapter) Containers() []*ContainerView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.Containers()
}

// Nodes returns cached Proxmox node views for AI/read-state consumers.
func (a *MonitorAdapter) Nodes() []*NodeView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.Nodes()
}

// Hosts returns cached host-agent views for AI/read-state consumers.
func (a *MonitorAdapter) Hosts() []*HostView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.Hosts()
}

// DockerHosts returns cached Docker host views for AI/read-state consumers.
func (a *MonitorAdapter) DockerHosts() []*DockerHostView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.DockerHosts()
}

// DockerContainers returns cached Docker container views for AI/read-state consumers.
func (a *MonitorAdapter) DockerContainers() []*DockerContainerView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.DockerContainers()
}

// StoragePools returns cached storage pool views for AI/read-state consumers.
func (a *MonitorAdapter) StoragePools() []*StoragePoolView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.StoragePools()
}

// PBSInstances returns cached PBS instance views for AI/read-state consumers.
func (a *MonitorAdapter) PBSInstances() []*PBSInstanceView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.PBSInstances()
}

// PMGInstances returns cached PMG instance views for AI/read-state consumers.
func (a *MonitorAdapter) PMGInstances() []*PMGInstanceView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.PMGInstances()
}

// K8sClusters returns cached Kubernetes cluster views for AI/read-state consumers.
func (a *MonitorAdapter) K8sClusters() []*K8sClusterView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.K8sClusters()
}

// K8sNodes returns cached Kubernetes node views for AI/read-state consumers.
func (a *MonitorAdapter) K8sNodes() []*K8sNodeView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.K8sNodes()
}

// Pods returns cached pod views for AI/read-state consumers.
func (a *MonitorAdapter) Pods() []*PodView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.Pods()
}

// K8sDeployments returns cached deployment views for AI/read-state consumers.
func (a *MonitorAdapter) K8sDeployments() []*K8sDeploymentView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.K8sDeployments()
}

// Workloads returns cached polymorphic workload views for AI/read-state consumers.
func (a *MonitorAdapter) Workloads() []*WorkloadView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.Workloads()
}

// Infrastructure returns cached polymorphic infrastructure views for AI/read-state consumers.
func (a *MonitorAdapter) Infrastructure() []*InfrastructureView {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.Infrastructure()
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
