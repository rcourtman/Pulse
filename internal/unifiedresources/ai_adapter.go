package unifiedresources

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// AIAdapter exposes a ResourceRegistry through the AI package's
// legacy-compatible ResourceProvider contract.
type AIAdapter struct {
	registry *ResourceRegistry

	mu           sync.RWMutex
	activeAlerts []models.Alert
}

// NewAIAdapter creates an AI-facing adapter around a registry.
// If registry is nil, a new in-memory registry is created.
func NewAIAdapter(registry *ResourceRegistry) *AIAdapter {
	if registry == nil {
		registry = NewRegistry(nil)
	}
	return &AIAdapter{registry: registry}
}

// PopulateFromSnapshot ingests a fresh state snapshot and updates active alerts.
func (a *AIAdapter) PopulateFromSnapshot(snapshot models.StateSnapshot) {
	if a == nil || a.registry == nil {
		return
	}

	a.registry.IngestSnapshot(snapshot)

	a.mu.Lock()
	a.activeAlerts = append([]models.Alert(nil), snapshot.ActiveAlerts...)
	a.mu.Unlock()
}

// SetActiveAlerts updates the active alert set used during legacy projection.
func (a *AIAdapter) SetActiveAlerts(alerts []models.Alert) {
	if a == nil {
		return
	}

	a.mu.Lock()
	a.activeAlerts = append([]models.Alert(nil), alerts...)
	a.mu.Unlock()
}

func (a *AIAdapter) GetAll() []LegacyResource {
	return a.listLegacyResources()
}

func (a *AIAdapter) GetInfrastructure() []LegacyResource {
	all := a.listLegacyResources()
	out := make([]LegacyResource, 0, len(all))
	for _, resource := range all {
		if resource.IsInfrastructure() {
			out = append(out, resource)
		}
	}
	return out
}

func (a *AIAdapter) GetWorkloads() []LegacyResource {
	all := a.listLegacyResources()
	out := make([]LegacyResource, 0, len(all))
	for _, resource := range all {
		if resource.IsWorkload() {
			out = append(out, resource)
		}
	}
	return out
}

func (a *AIAdapter) GetByType(t LegacyResourceType) []LegacyResource {
	all := a.listLegacyResources()
	out := make([]LegacyResource, 0, len(all))
	for _, resource := range all {
		if resource.Type == t {
			out = append(out, resource)
		}
	}
	return out
}

func (a *AIAdapter) GetStats() LegacyStoreStats {
	all := a.listLegacyResources()
	stats := LegacyStoreStats{
		TotalResources:      len(all),
		SuppressedResources: 0,
		ByType:              make(map[LegacyResourceType]int),
		ByPlatform:          make(map[LegacyPlatformType]int),
		ByStatus:            make(map[LegacyResourceStatus]int),
		WithAlerts:          0,
		LastUpdated:         time.Now().UTC().Format(time.RFC3339),
	}

	for _, resource := range all {
		stats.ByType[resource.Type]++
		stats.ByPlatform[resource.PlatformType]++
		stats.ByStatus[resource.Status]++
		if len(resource.Alerts) > 0 {
			stats.WithAlerts++
		}
	}

	return stats
}

func (a *AIAdapter) GetTopByCPU(limit int, types []LegacyResourceType) []LegacyResource {
	return a.getTopByMetric(limit, types, func(resource LegacyResource) float64 {
		return resource.CPUPercent()
	})
}

func (a *AIAdapter) GetTopByMemory(limit int, types []LegacyResourceType) []LegacyResource {
	return a.getTopByMetric(limit, types, func(resource LegacyResource) float64 {
		return resource.MemoryPercent()
	})
}

func (a *AIAdapter) GetTopByDisk(limit int, types []LegacyResourceType) []LegacyResource {
	return a.getTopByMetric(limit, types, func(resource LegacyResource) float64 {
		return resource.DiskPercent()
	})
}

func (a *AIAdapter) GetRelated(resourceID string) map[string][]LegacyResource {
	result := make(map[string][]LegacyResource)
	if strings.TrimSpace(resourceID) == "" {
		return result
	}

	all := a.listLegacyResources()
	if len(all) == 0 {
		return result
	}

	byID := make(map[string]LegacyResource, len(all))
	for _, resource := range all {
		byID[resource.ID] = resource
	}

	resource, ok := byID[resourceID]
	if !ok {
		return result
	}

	if resource.ParentID != "" {
		if parent, ok := byID[resource.ParentID]; ok {
			result["parent"] = []LegacyResource{parent}
		}
	}

	children := make([]LegacyResource, 0)
	siblings := make([]LegacyResource, 0)
	clusterMembers := make([]LegacyResource, 0)
	for _, other := range all {
		if other.ParentID == resourceID {
			children = append(children, other)
		}
		if resource.ParentID != "" && other.ID != resourceID && other.ParentID == resource.ParentID {
			siblings = append(siblings, other)
		}
		if resource.ClusterID != "" && other.ID != resourceID && other.ClusterID == resource.ClusterID {
			clusterMembers = append(clusterMembers, other)
		}
	}

	if len(children) > 0 {
		result["children"] = children
	}
	if len(siblings) > 0 {
		result["siblings"] = siblings
	}
	if len(clusterMembers) > 0 {
		result["cluster_members"] = clusterMembers
	}

	return result
}

func (a *AIAdapter) GetResourceSummary() LegacyResourceSummary {
	all := a.listLegacyResources()
	summary := LegacyResourceSummary{
		ByType:     make(map[LegacyResourceType]LegacyTypeSummary),
		ByPlatform: make(map[LegacyPlatformType]LegacyPlatformSummary),
	}

	for _, resource := range all {
		summary.TotalResources++
		switch resource.Status {
		case LegacyStatusOnline, LegacyStatusRunning:
			summary.Healthy++
		case LegacyStatusDegraded:
			summary.Degraded++
		default:
			summary.Offline++
		}
		if len(resource.Alerts) > 0 {
			summary.WithAlerts++
		}

		typeSummary := summary.ByType[resource.Type]
		typeSummary.Count++
		if resource.CPU != nil {
			typeSummary.TotalCPUPercent += resource.CPUPercent()
		}
		if resource.Memory != nil {
			typeSummary.TotalMemoryPercent += resource.MemoryPercent()
		}
		summary.ByType[resource.Type] = typeSummary

		platformSummary := summary.ByPlatform[resource.PlatformType]
		platformSummary.Count++
		summary.ByPlatform[resource.PlatformType] = platformSummary
	}

	for t, typeSummary := range summary.ByType {
		if typeSummary.Count == 0 {
			continue
		}
		typeSummary.AvgCPUPercent = typeSummary.TotalCPUPercent / float64(typeSummary.Count)
		typeSummary.AvgMemoryPercent = typeSummary.TotalMemoryPercent / float64(typeSummary.Count)
		summary.ByType[t] = typeSummary
	}

	return summary
}

func (a *AIAdapter) FindContainerHost(containerNameOrID string) string {
	query := strings.TrimSpace(containerNameOrID)
	if query == "" {
		return ""
	}

	all := a.listLegacyResources()
	if len(all) == 0 {
		return ""
	}

	queryLower := strings.ToLower(query)
	byID := make(map[string]LegacyResource, len(all))
	bestID := ""
	bestScore := 0

	for _, resource := range all {
		byID[resource.ID] = resource
		if !isHostResolvableWorkload(resource.Type) {
			continue
		}

		score := 0
		if strings.EqualFold(resource.ID, query) || strings.EqualFold(resource.Name, query) || strings.EqualFold(resource.DisplayName, query) {
			score = 3
		} else if strings.Contains(strings.ToLower(resource.ID), queryLower) ||
			strings.Contains(strings.ToLower(resource.Name), queryLower) ||
			strings.Contains(strings.ToLower(resource.DisplayName), queryLower) {
			score = 1
		}
		if score > bestScore {
			bestScore = score
			bestID = resource.ID
		}
	}

	if bestID == "" {
		return ""
	}

	resource := byID[bestID]
	if resource.ParentID == "" {
		return ""
	}

	parent, ok := byID[resource.ParentID]
	if !ok {
		return ""
	}
	if parent.Identity != nil && parent.Identity.Hostname != "" {
		return parent.Identity.Hostname
	}
	if parent.Name != "" {
		return parent.Name
	}
	return parent.ID
}

func (a *AIAdapter) listLegacyResources() []LegacyResource {
	if a == nil || a.registry == nil {
		return nil
	}

	a.mu.RLock()
	alerts := append([]models.Alert(nil), a.activeAlerts...)
	a.mu.RUnlock()

	return convertRegistryToLegacy(a.registry, alerts)
}

func (a *AIAdapter) getTopByMetric(limit int, types []LegacyResourceType, metric func(LegacyResource) float64) []LegacyResource {
	all := a.listLegacyResources()
	includeTypes := make(map[LegacyResourceType]struct{}, len(types))
	for _, t := range types {
		includeTypes[t] = struct{}{}
	}

	results := make([]LegacyResource, 0, len(all))
	for _, resource := range all {
		if len(includeTypes) > 0 {
			if _, ok := includeTypes[resource.Type]; !ok {
				continue
			}
		}
		if metric(resource) <= 0 {
			continue
		}
		results = append(results, resource)
	}

	sort.Slice(results, func(i, j int) bool {
		left := metric(results[i])
		right := metric(results[j])
		if left == right {
			return strings.ToLower(results[i].EffectiveDisplayName()) < strings.ToLower(results[j].EffectiveDisplayName())
		}
		return left > right
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}
