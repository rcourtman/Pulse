package unifiedresources

import (
	"sort"
	"strings"
)

// UnifiedAIAdapter exposes ResourceRegistry through a unified-resource-native
// AI-facing provider contract.
type UnifiedAIAdapter struct {
	registry *ResourceRegistry
}

// NewUnifiedAIAdapter creates an AI-facing unified adapter around a registry.
// If registry is nil, a new in-memory registry is created.
func NewUnifiedAIAdapter(registry *ResourceRegistry) *UnifiedAIAdapter {
	if registry == nil {
		registry = NewRegistry(nil)
	}
	return &UnifiedAIAdapter{registry: registry}
}

func (a *UnifiedAIAdapter) GetAll() []Resource {
	return a.listResources()
}

func (a *UnifiedAIAdapter) GetInfrastructure() []Resource {
	all := a.listResources()
	out := make([]Resource, 0, len(all))
	for _, resource := range all {
		if isUnifiedInfrastructure(resource.Type) {
			out = append(out, resource)
		}
	}
	return out
}

func (a *UnifiedAIAdapter) GetWorkloads() []Resource {
	all := a.listResources()
	out := make([]Resource, 0, len(all))
	for _, resource := range all {
		if isUnifiedWorkload(resource.Type) {
			out = append(out, resource)
		}
	}
	return out
}

func (a *UnifiedAIAdapter) GetByType(t ResourceType) []Resource {
	all := a.listResources()
	out := make([]Resource, 0, len(all))
	for _, resource := range all {
		if resource.Type == t {
			out = append(out, resource)
		}
	}
	return out
}

func (a *UnifiedAIAdapter) GetStats() ResourceStats {
	if a == nil || a.registry == nil {
		return ResourceStats{
			ByType:   make(map[ResourceType]int),
			ByStatus: make(map[ResourceStatus]int),
			BySource: make(map[DataSource]int),
		}
	}
	return a.registry.Stats()
}

func (a *UnifiedAIAdapter) GetTopByCPU(limit int, types []ResourceType) []Resource {
	return a.getTopByMetric(limit, types, func(resource Resource) float64 {
		if resource.Metrics == nil {
			return 0
		}
		return metricPercent(resource.Metrics.CPU)
	})
}

func (a *UnifiedAIAdapter) GetTopByMemory(limit int, types []ResourceType) []Resource {
	return a.getTopByMetric(limit, types, func(resource Resource) float64 {
		if resource.Metrics == nil {
			return 0
		}
		return metricPercent(resource.Metrics.Memory)
	})
}

func (a *UnifiedAIAdapter) GetTopByDisk(limit int, types []ResourceType) []Resource {
	return a.getTopByMetric(limit, types, func(resource Resource) float64 {
		if resource.Metrics == nil {
			return 0
		}
		return metricPercent(resource.Metrics.Disk)
	})
}

func (a *UnifiedAIAdapter) GetRelated(resourceID string) map[string][]Resource {
	result := make(map[string][]Resource)
	if strings.TrimSpace(resourceID) == "" {
		return result
	}

	all := a.listResources()
	if len(all) == 0 {
		return result
	}

	byID := make(map[string]Resource, len(all))
	for _, resource := range all {
		byID[resource.ID] = resource
	}

	resource, ok := byID[resourceID]
	if !ok {
		return result
	}

	if resource.ParentID != nil && strings.TrimSpace(*resource.ParentID) != "" {
		if parent, ok := byID[*resource.ParentID]; ok {
			result["parent"] = []Resource{parent}
		}
	}

	children := make([]Resource, 0)
	if a != nil && a.registry != nil {
		children = a.registry.GetChildren(resourceID)
	}
	if len(children) > 0 {
		result["children"] = children
	}

	siblings := make([]Resource, 0)
	if resource.ParentID != nil && strings.TrimSpace(*resource.ParentID) != "" {
		for _, other := range all {
			if other.ID == resourceID || other.ParentID == nil {
				continue
			}
			if *other.ParentID == *resource.ParentID {
				siblings = append(siblings, other)
			}
		}
	}
	if len(siblings) > 0 {
		result["siblings"] = siblings
	}

	return result
}

func (a *UnifiedAIAdapter) FindContainerHost(containerNameOrID string) string {
	query := strings.TrimSpace(containerNameOrID)
	if query == "" {
		return ""
	}

	all := a.listResources()
	if len(all) == 0 {
		return ""
	}

	queryLower := strings.ToLower(query)
	byID := make(map[string]Resource, len(all))
	bestID := ""
	bestScore := 0

	for _, resource := range all {
		byID[resource.ID] = resource
		if !isUnifiedHostResolvableWorkload(resource.Type) {
			continue
		}

		containerID := ""
		if resource.Docker != nil {
			containerID = strings.TrimSpace(resource.Docker.ContainerID)
		}

		score := 0
		if strings.EqualFold(resource.ID, query) ||
			strings.EqualFold(resource.Name, query) ||
			(containerID != "" && strings.EqualFold(containerID, query)) {
			score = 3
		} else if strings.Contains(strings.ToLower(resource.ID), queryLower) ||
			strings.Contains(strings.ToLower(resource.Name), queryLower) ||
			(containerID != "" && strings.Contains(strings.ToLower(containerID), queryLower)) {
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
	if resource.ParentID == nil || strings.TrimSpace(*resource.ParentID) == "" {
		return ""
	}

	parent, ok := byID[*resource.ParentID]
	if !ok {
		return ""
	}
	if parent.Agent != nil && strings.TrimSpace(parent.Agent.Hostname) != "" {
		return strings.TrimSpace(parent.Agent.Hostname)
	}
	for _, hostname := range parent.Identity.Hostnames {
		if strings.TrimSpace(hostname) != "" {
			return strings.TrimSpace(hostname)
		}
	}
	if strings.TrimSpace(parent.Name) != "" {
		return strings.TrimSpace(parent.Name)
	}
	return parent.ID
}

func (a *UnifiedAIAdapter) listResources() []Resource {
	if a == nil || a.registry == nil {
		return nil
	}
	return a.registry.List()
}

func (a *UnifiedAIAdapter) getTopByMetric(limit int, types []ResourceType, metric func(Resource) float64) []Resource {
	all := a.listResources()
	includeTypes := make(map[ResourceType]struct{}, len(types))
	for _, t := range types {
		includeTypes[t] = struct{}{}
	}

	results := make([]Resource, 0, len(all))
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
			return strings.ToLower(unifiedDisplayName(results[i])) < strings.ToLower(unifiedDisplayName(results[j]))
		}
		return left > right
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

func metricPercent(metric *MetricValue) float64 {
	if metric == nil {
		return 0
	}
	if metric.Percent > 0 {
		return metric.Percent
	}
	if metric.Value > 0 {
		return metric.Value
	}
	if metric.Total != nil && metric.Used != nil && *metric.Total > 0 {
		return (float64(*metric.Used) / float64(*metric.Total)) * 100
	}
	return 0
}

func unifiedDisplayName(resource Resource) string {
	if strings.TrimSpace(resource.Name) != "" {
		return resource.Name
	}
	return resource.ID
}

func isUnifiedInfrastructure(t ResourceType) bool {
	switch t {
	case ResourceTypeHost, ResourceTypeK8sNode, ResourceTypeK8sCluster:
		return true
	default:
		return false
	}
}

func isUnifiedWorkload(t ResourceType) bool {
	switch t {
	case ResourceTypeVM, ResourceTypeSystemContainer, ResourceTypeAppContainer, ResourceTypePod, ResourceTypeK8sDeployment:
		return true
	default:
		return false
	}
}

func isUnifiedHostResolvableWorkload(t ResourceType) bool {
	switch t {
	case ResourceTypeAppContainer, ResourceTypeSystemContainer, ResourceTypeVM:
		return true
	default:
		return false
	}
}
