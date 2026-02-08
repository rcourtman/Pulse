package unifiedresources

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// LegacyAdapter projects unified resources into the legacy V1-compatible shape.
// It is used by websocket broadcasting, AI context building, and legacy /api/resources.
type LegacyAdapter struct {
	mu        sync.RWMutex
	store     ResourceStore
	resources map[string]LegacyResource
	order     []string
}

func NewLegacyAdapter(store ResourceStore) *LegacyAdapter {
	return &LegacyAdapter{
		store:     store,
		resources: make(map[string]LegacyResource),
		order:     make([]string, 0),
	}
}

// Upsert allows tests to insert resources directly without a snapshot conversion pass.
func (a *LegacyAdapter) Upsert(resource LegacyResource) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if resource.SchemaVersion == 0 {
		resource.SchemaVersion = LegacySchemaVersion
	}
	if _, exists := a.resources[resource.ID]; !exists {
		a.order = append(a.order, resource.ID)
	}
	a.resources[resource.ID] = resource
	a.sortLocked()
	return resource.ID
}

func (a *LegacyAdapter) PopulateFromSnapshot(snapshot models.StateSnapshot) {
	registry := NewRegistry(a.store)
	registry.IngestSnapshot(snapshot)
	legacy := convertRegistryToLegacy(registry, snapshot.ActiveAlerts)

	a.mu.Lock()
	defer a.mu.Unlock()

	a.resources = make(map[string]LegacyResource, len(legacy))
	a.order = a.order[:0]
	for _, resource := range legacy {
		a.resources[resource.ID] = resource
		a.order = append(a.order, resource.ID)
	}
	a.sortLocked()
}

func (a *LegacyAdapter) Get(id string) (*LegacyResource, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	resource, ok := a.resources[id]
	if !ok {
		return nil, false
	}
	copy := resource
	return &copy, true
}

func (a *LegacyAdapter) GetAll() []LegacyResource {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.copyInOrderLocked(nil)
}

func (a *LegacyAdapter) GetInfrastructure() []LegacyResource {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.copyInOrderLocked(func(resource LegacyResource) bool {
		return resource.IsInfrastructure()
	})
}

func (a *LegacyAdapter) GetWorkloads() []LegacyResource {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.copyInOrderLocked(func(resource LegacyResource) bool {
		return resource.IsWorkload()
	})
}

func (a *LegacyAdapter) GetByType(t LegacyResourceType) []LegacyResource {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.copyInOrderLocked(func(resource LegacyResource) bool {
		return resource.Type == t
	})
}

func (a *LegacyAdapter) ShouldSkipAPIPolling(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "" {
		return false
	}

	recommendations := a.GetPollingRecommendations()
	multiplier, ok := recommendations[hostname]
	if !ok {
		return false
	}
	return multiplier == 0
}

func (a *LegacyAdapter) GetPollingRecommendations() map[string]float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	recommendations := make(map[string]float64)
	for _, resource := range a.resources {
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
			recommendations[key] = 0.5
			continue
		}
		recommendations[key] = 0
	}
	return recommendations
}

func (a *LegacyAdapter) GetStats() LegacyStoreStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := LegacyStoreStats{
		TotalResources:      len(a.resources),
		SuppressedResources: 0,
		ByType:              make(map[LegacyResourceType]int),
		ByPlatform:          make(map[LegacyPlatformType]int),
		ByStatus:            make(map[LegacyResourceStatus]int),
		WithAlerts:          0,
		LastUpdated:         time.Now().UTC().Format(time.RFC3339),
	}

	for _, resource := range a.resources {
		stats.ByType[resource.Type]++
		stats.ByPlatform[resource.PlatformType]++
		stats.ByStatus[resource.Status]++
		if len(resource.Alerts) > 0 {
			stats.WithAlerts++
		}
	}

	return stats
}

func (a *LegacyAdapter) GetTopByCPU(limit int, types []LegacyResourceType) []LegacyResource {
	return a.getTopByMetric(limit, types, func(resource LegacyResource) float64 {
		return resource.CPUPercent()
	})
}

func (a *LegacyAdapter) GetTopByMemory(limit int, types []LegacyResourceType) []LegacyResource {
	return a.getTopByMetric(limit, types, func(resource LegacyResource) float64 {
		return resource.MemoryPercent()
	})
}

func (a *LegacyAdapter) GetTopByDisk(limit int, types []LegacyResourceType) []LegacyResource {
	return a.getTopByMetric(limit, types, func(resource LegacyResource) float64 {
		return resource.DiskPercent()
	})
}

func (a *LegacyAdapter) GetRelated(resourceID string) map[string][]LegacyResource {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string][]LegacyResource)
	resource, ok := a.resources[resourceID]
	if !ok {
		return result
	}

	if resource.ParentID != "" {
		if parent, ok := a.resources[resource.ParentID]; ok {
			result["parent"] = []LegacyResource{parent}
		}
	}

	children := make([]LegacyResource, 0)
	siblings := make([]LegacyResource, 0)
	clusterMembers := make([]LegacyResource, 0)

	for _, other := range a.resources {
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

func (a *LegacyAdapter) GetResourceSummary() LegacyResourceSummary {
	a.mu.RLock()
	defer a.mu.RUnlock()

	summary := LegacyResourceSummary{
		ByType:     make(map[LegacyResourceType]LegacyTypeSummary),
		ByPlatform: make(map[LegacyPlatformType]LegacyPlatformSummary),
	}

	for _, resource := range a.resources {
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

func (a *LegacyAdapter) FindContainerHost(containerNameOrID string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	query := strings.TrimSpace(containerNameOrID)
	if query == "" {
		return ""
	}
	queryLower := strings.ToLower(query)

	bestID := ""
	bestScore := 0
	for _, resource := range a.resources {
		if !isHostResolvableWorkload(resource.Type) {
			continue
		}

		score := 0
		if strings.EqualFold(resource.ID, query) || strings.EqualFold(resource.Name, query) || strings.EqualFold(resource.DisplayName, query) {
			score = 3
		} else if strings.Contains(strings.ToLower(resource.ID), queryLower) || strings.Contains(strings.ToLower(resource.Name), queryLower) || strings.Contains(strings.ToLower(resource.DisplayName), queryLower) {
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

	resource := a.resources[bestID]
	if resource.ParentID == "" {
		return ""
	}
	parent, ok := a.resources[resource.ParentID]
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

func (a *LegacyAdapter) copyInOrderLocked(include func(LegacyResource) bool) []LegacyResource {
	out := make([]LegacyResource, 0, len(a.order))
	for _, id := range a.order {
		resource, ok := a.resources[id]
		if !ok {
			continue
		}
		if include != nil && !include(resource) {
			continue
		}
		out = append(out, resource)
	}
	return out
}

func (a *LegacyAdapter) getTopByMetric(limit int, types []LegacyResourceType, metric func(LegacyResource) float64) []LegacyResource {
	a.mu.RLock()
	defer a.mu.RUnlock()

	includeTypes := make(map[LegacyResourceType]struct{}, len(types))
	for _, t := range types {
		includeTypes[t] = struct{}{}
	}

	results := make([]LegacyResource, 0, len(a.resources))
	for _, resource := range a.resources {
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
		return metric(results[i]) > metric(results[j])
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

func (a *LegacyAdapter) sortLocked() {
	sort.Slice(a.order, func(i, j int) bool {
		left := a.resources[a.order[i]]
		right := a.resources[a.order[j]]
		leftName := strings.ToLower(left.EffectiveDisplayName())
		rightName := strings.ToLower(right.EffectiveDisplayName())
		if leftName == rightName {
			return left.ID < right.ID
		}
		return leftName < rightName
	})
}

func isHostResolvableWorkload(t LegacyResourceType) bool {
	switch t {
	case LegacyResourceTypeDockerContainer, LegacyResourceTypeContainer, LegacyResourceTypeVM, LegacyResourceTypeOCIContainer:
		return true
	default:
		return false
	}
}

func convertRegistryToLegacy(registry *ResourceRegistry, activeAlerts []models.Alert) []LegacyResource {
	resources := registry.List()
	legacy := make([]LegacyResource, 0, len(resources))
	targetsByResourceID := make(map[string][]SourceTarget, len(resources))
	legacyIDByResourceID := make(map[string]string, len(resources))

	for _, resource := range resources {
		targets := registry.SourceTargets(resource.ID)
		targetsByResourceID[resource.ID] = targets
		legacyType := mapLegacyType(resource)
		legacyIDByResourceID[resource.ID] = mapLegacyID(resource, legacyType, targets)
	}

	for _, resource := range resources {
		targets := targetsByResourceID[resource.ID]
		parentLegacyID := ""
		if resource.ParentID != nil {
			parentLegacyID = legacyIDByResourceID[*resource.ParentID]
		}
		legacyResource := convertUnifiedResource(resource, targets, legacyIDByResourceID[resource.ID], parentLegacyID)
		legacyResource.Alerts = projectAlertsForLegacyResource(resource, legacyResource, targets, activeAlerts)
		legacy = append(legacy, legacyResource)
	}

	sort.Slice(legacy, func(i, j int) bool {
		left := strings.ToLower(legacy[i].EffectiveDisplayName())
		right := strings.ToLower(legacy[j].EffectiveDisplayName())
		if left == right {
			return legacy[i].ID < legacy[j].ID
		}
		return left < right
	})
	return legacy
}

func convertUnifiedResource(resource Resource, targets []SourceTarget, legacyID string, parentLegacyID string) LegacyResource {
	legacyType := mapLegacyType(resource)
	if legacyID == "" {
		legacyID = resource.ID
	}
	name, displayName := mapLegacyNames(resource, legacyType, legacyID)
	platformType := mapLegacyPlatformType(resource, legacyType)
	platformID := mapLegacyPlatformID(resource, legacyType, targets)
	sourceType := mapLegacySourceType(resource.Sources)
	status := mapLegacyStatus(resource, legacyType)
	if parentLegacyID == "" {
		parentLegacyID = valueOrEmpty(resource.ParentID)
	}
	clusterID := strings.TrimSpace(resource.Identity.ClusterName)
	if clusterID == "" && resource.Proxmox != nil {
		clusterID = strings.TrimSpace(resource.Proxmox.ClusterName)
	}
	if clusterID == "" && resource.Kubernetes != nil {
		clusterID = strings.TrimSpace(resource.Kubernetes.ClusterName)
	}

	legacy := LegacyResource{
		ID:            legacyID,
		Type:          legacyType,
		Name:          name,
		DisplayName:   displayName,
		PlatformID:    platformID,
		PlatformType:  platformType,
		SourceType:    sourceType,
		ParentID:      parentLegacyID,
		ClusterID:     clusterID,
		Status:        status,
		CPU:           convertLegacyMetric(resource.MetricsValueCPU()),
		Memory:        convertLegacyMetric(resource.MetricsValueMemory()),
		Disk:          convertLegacyMetric(resource.MetricsValueDisk()),
		Network:       convertLegacyNetwork(resource.MetricsValueNetIn(), resource.MetricsValueNetOut()),
		Temperature:   extractTemperature(resource),
		Uptime:        extractUptime(resource),
		Tags:          append([]string(nil), resource.Tags...),
		Labels:        extractLabels(resource),
		LastSeen:      resource.LastSeen,
		PlatformData:  buildLegacyPlatformData(resource, legacyType, platformID),
		Identity:      convertLegacyIdentity(resource, name),
		SchemaVersion: LegacySchemaVersion,
	}

	if legacyType == LegacyResourceTypeDockerContainer && legacy.ParentID != "" {
		if sourceID := strings.TrimSpace(sourceIDFor(targets, SourceDocker)); sourceID != "" && !strings.Contains(sourceID, "/") {
			legacy.ID = legacy.ParentID + "/" + sourceID
		}
	}

	if legacy.LastSeen.IsZero() {
		legacy.LastSeen = time.Now().UTC()
	}
	return legacy
}

func mapLegacyType(resource Resource) LegacyResourceType {
	switch resource.Type {
	case ResourceTypeVM:
		return LegacyResourceTypeVM
	case ResourceTypeLXC:
		return LegacyResourceTypeContainer
	case ResourceTypeContainer:
		return LegacyResourceTypeDockerContainer
	case ResourceTypeK8sCluster:
		return LegacyResourceTypeK8sCluster
	case ResourceTypeK8sNode:
		return LegacyResourceTypeK8sNode
	case ResourceTypePod:
		return LegacyResourceTypePod
	case ResourceTypeK8sDeployment:
		return LegacyResourceTypeK8sDeployment
	case ResourceTypePBS:
		return LegacyResourceTypePBS
	case ResourceTypePMG:
		return LegacyResourceTypePMG
	case ResourceTypeStorage:
		return LegacyResourceTypeStorage
	case ResourceTypeCeph:
		return LegacyResourceTypePool
	case ResourceTypeHost:
		if resource.Proxmox != nil {
			return LegacyResourceTypeNode
		}
		if resource.Docker != nil {
			return LegacyResourceTypeDockerHost
		}
		return LegacyResourceTypeHost
	default:
		return LegacyResourceType(resource.Type)
	}
}

func mapLegacyNames(resource Resource, legacyType LegacyResourceType, fallbackID string) (name string, displayName string) {
	name = strings.TrimSpace(resource.Name)
	displayName = ""

	switch legacyType {
	case LegacyResourceTypeNode:
		if resource.Proxmox != nil && resource.Proxmox.NodeName != "" && !strings.EqualFold(resource.Proxmox.NodeName, name) {
			displayName = name
			name = resource.Proxmox.NodeName
		}
	case LegacyResourceTypeHost:
		if resource.Agent != nil && resource.Agent.Hostname != "" && !strings.EqualFold(resource.Agent.Hostname, name) {
			displayName = name
			name = resource.Agent.Hostname
		}
	case LegacyResourceTypeDockerHost:
		if resource.Docker != nil && resource.Docker.Hostname != "" && !strings.EqualFold(resource.Docker.Hostname, name) {
			displayName = name
			name = resource.Docker.Hostname
		}
	}

	if name == "" {
		name = fallbackID
	}
	if name == "" {
		name = resource.ID
	}
	return name, strings.TrimSpace(displayName)
}

func mapLegacyPlatformType(resource Resource, legacyType LegacyResourceType) LegacyPlatformType {
	switch legacyType {
	case LegacyResourceTypeNode, LegacyResourceTypeVM, LegacyResourceTypeContainer, LegacyResourceTypeStorage, LegacyResourceTypePool:
		return LegacyPlatformProxmoxPVE
	case LegacyResourceTypeDockerHost, LegacyResourceTypeDockerContainer:
		return LegacyPlatformDocker
	case LegacyResourceTypeK8sCluster, LegacyResourceTypeK8sNode, LegacyResourceTypePod, LegacyResourceTypeK8sDeployment:
		return LegacyPlatformKubernetes
	case LegacyResourceTypePBS:
		return LegacyPlatformProxmoxPBS
	case LegacyResourceTypePMG:
		return LegacyPlatformProxmoxPMG
	case LegacyResourceTypeHost:
		return LegacyPlatformHostAgent
	default:
		if hasSource(resource.Sources, SourceK8s) {
			return LegacyPlatformKubernetes
		}
		if hasSource(resource.Sources, SourceDocker) {
			return LegacyPlatformDocker
		}
		if hasSource(resource.Sources, SourcePBS) {
			return LegacyPlatformProxmoxPBS
		}
		if hasSource(resource.Sources, SourcePMG) {
			return LegacyPlatformProxmoxPMG
		}
		if hasSource(resource.Sources, SourceAgent) {
			return LegacyPlatformHostAgent
		}
		return LegacyPlatformProxmoxPVE
	}
}

func mapLegacyPlatformID(resource Resource, legacyType LegacyResourceType, targets []SourceTarget) string {
	switch legacyType {
	case LegacyResourceTypeNode, LegacyResourceTypeVM, LegacyResourceTypeContainer:
		if resource.Proxmox != nil && strings.TrimSpace(resource.Proxmox.Instance) != "" {
			return strings.TrimSpace(resource.Proxmox.Instance)
		}
		if sourceID := sourceIDFor(targets, SourceProxmox); sourceID != "" {
			return sourceID
		}
	case LegacyResourceTypeHost:
		if sourceID := sourceIDFor(targets, SourceAgent); sourceID != "" {
			return sourceID
		}
		if resource.Agent != nil && strings.TrimSpace(resource.Agent.AgentID) != "" {
			return strings.TrimSpace(resource.Agent.AgentID)
		}
	case LegacyResourceTypeDockerHost:
		if sourceID := sourceIDFor(targets, SourceDocker); sourceID != "" {
			return sourceID
		}
		if resource.Docker != nil && strings.TrimSpace(resource.Docker.Hostname) != "" {
			return strings.TrimSpace(resource.Docker.Hostname)
		}
	case LegacyResourceTypeDockerContainer:
		if resource.Docker != nil && strings.TrimSpace(resource.Docker.Hostname) != "" {
			return strings.TrimSpace(resource.Docker.Hostname)
		}
		if resource.ParentID != nil {
			return strings.TrimSpace(*resource.ParentID)
		}
	case LegacyResourceTypeK8sCluster, LegacyResourceTypeK8sNode, LegacyResourceTypePod, LegacyResourceTypeK8sDeployment:
		if resource.Kubernetes != nil && strings.TrimSpace(resource.Kubernetes.AgentID) != "" {
			return strings.TrimSpace(resource.Kubernetes.AgentID)
		}
		if sourceID := sourceIDFor(targets, SourceK8s); sourceID != "" {
			return sourceID
		}
	case LegacyResourceTypePBS:
		if resource.PBS != nil && strings.TrimSpace(resource.PBS.Hostname) != "" {
			return strings.TrimSpace(resource.PBS.Hostname)
		}
		if sourceID := sourceIDFor(targets, SourcePBS); sourceID != "" {
			return sourceID
		}
	case LegacyResourceTypePMG:
		if resource.PMG != nil && strings.TrimSpace(resource.PMG.Hostname) != "" {
			return strings.TrimSpace(resource.PMG.Hostname)
		}
		if sourceID := sourceIDFor(targets, SourcePMG); sourceID != "" {
			return sourceID
		}
	}

	if sourceID := firstSourceID(targets); sourceID != "" {
		return sourceID
	}
	return resource.ID
}

func mapLegacyID(resource Resource, legacyType LegacyResourceType, targets []SourceTarget) string {
	switch legacyType {
	case LegacyResourceTypeNode, LegacyResourceTypeVM, LegacyResourceTypeContainer, LegacyResourceTypeOCIContainer, LegacyResourceTypeStorage, LegacyResourceTypePool:
		if sourceID := sourceIDFor(targets, SourceProxmox); sourceID != "" {
			return sourceID
		}
	case LegacyResourceTypeHost:
		if sourceID := sourceIDFor(targets, SourceAgent); sourceID != "" {
			return sourceID
		}
	case LegacyResourceTypeDockerHost, LegacyResourceTypeDockerContainer, LegacyResourceTypeDockerService:
		if sourceID := sourceIDFor(targets, SourceDocker); sourceID != "" {
			return sourceID
		}
	case LegacyResourceTypeK8sCluster, LegacyResourceTypeK8sNode, LegacyResourceTypePod, LegacyResourceTypeK8sDeployment, LegacyResourceTypeK8sService:
		if sourceID := sourceIDFor(targets, SourceK8s); sourceID != "" {
			return sourceID
		}
	case LegacyResourceTypePBS, LegacyResourceTypeDatastore:
		if sourceID := sourceIDFor(targets, SourcePBS); sourceID != "" {
			return sourceID
		}
	case LegacyResourceTypePMG:
		if sourceID := sourceIDFor(targets, SourcePMG); sourceID != "" {
			return sourceID
		}
	}

	if sourceID := firstSourceID(targets); sourceID != "" {
		return sourceID
	}
	return resource.ID
}

func mapLegacySourceType(sources []DataSource) LegacySourceType {
	if len(sources) > 1 {
		return LegacySourceHybrid
	}
	if len(sources) == 1 {
		switch sources[0] {
		case SourceAgent, SourceDocker, SourceK8s:
			return LegacySourceAgent
		default:
			return LegacySourceAPI
		}
	}
	return LegacySourceAPI
}

func mapLegacyStatus(resource Resource, legacyType LegacyResourceType) LegacyResourceStatus {
	switch legacyType {
	case LegacyResourceTypeDockerContainer:
		// Docker container runtime state is not explicitly preserved in v2;
		// map from unified status while keeping legacy vocabulary.
		switch resource.Status {
		case StatusOnline:
			return LegacyStatusRunning
		case StatusOffline:
			return LegacyStatusStopped
		case StatusWarning:
			return LegacyStatusDegraded
		}
	case LegacyResourceTypePod:
		if resource.Kubernetes != nil {
			phase := strings.ToLower(strings.TrimSpace(resource.Kubernetes.PodPhase))
			switch phase {
			case "running":
				return LegacyStatusRunning
			case "pending", "unknown":
				return LegacyStatusDegraded
			case "succeeded", "failed":
				return LegacyStatusStopped
			}
		}
	}

	switch resource.Status {
	case StatusOnline:
		if isHostResolvableWorkload(legacyType) || legacyType == LegacyResourceTypePod {
			return LegacyStatusRunning
		}
		return LegacyStatusOnline
	case StatusOffline:
		if isHostResolvableWorkload(legacyType) || legacyType == LegacyResourceTypePod {
			return LegacyStatusStopped
		}
		return LegacyStatusOffline
	case StatusWarning:
		return LegacyStatusDegraded
	default:
		return LegacyStatusUnknown
	}
}

func convertLegacyMetric(metric *MetricValue) *LegacyMetricValue {
	if metric == nil {
		return nil
	}

	current := metric.Percent
	if current == 0 {
		current = metric.Value
	}
	if metric.Percent != 0 && metric.Value != 0 {
		current = math.Max(metric.Percent, metric.Value)
	}

	result := &LegacyMetricValue{Current: current}
	if metric.Total != nil {
		total := *metric.Total
		result.Total = &total
	}
	if metric.Used != nil {
		used := *metric.Used
		result.Used = &used
	}
	if result.Total != nil && result.Used != nil {
		free := *result.Total - *result.Used
		result.Free = &free
	}
	return result
}

func convertLegacyNetwork(in *MetricValue, out *MetricValue) *LegacyNetworkMetric {
	if in == nil && out == nil {
		return nil
	}

	rx := int64(0)
	tx := int64(0)
	if in != nil {
		rx = int64(math.Round(in.Value))
	}
	if out != nil {
		tx = int64(math.Round(out.Value))
	}
	return &LegacyNetworkMetric{RXBytes: rx, TXBytes: tx}
}

func extractTemperature(resource Resource) *float64 {
	if resource.Agent != nil && resource.Agent.Temperature != nil {
		value := *resource.Agent.Temperature
		return &value
	}
	if resource.Proxmox != nil && resource.Proxmox.Temperature != nil {
		value := *resource.Proxmox.Temperature
		return &value
	}
	if resource.Docker != nil && resource.Docker.Temperature != nil {
		value := *resource.Docker.Temperature
		return &value
	}
	if resource.Kubernetes != nil && resource.Kubernetes.Temperature != nil {
		value := *resource.Kubernetes.Temperature
		return &value
	}
	return nil
}

func extractUptime(resource Resource) *int64 {
	if resource.Agent != nil && resource.Agent.UptimeSeconds > 0 {
		value := resource.Agent.UptimeSeconds
		return &value
	}
	if resource.Proxmox != nil && resource.Proxmox.Uptime > 0 {
		value := resource.Proxmox.Uptime
		return &value
	}
	if resource.Docker != nil && resource.Docker.UptimeSeconds > 0 {
		value := resource.Docker.UptimeSeconds
		return &value
	}
	if resource.Kubernetes != nil && resource.Kubernetes.UptimeSeconds > 0 {
		value := resource.Kubernetes.UptimeSeconds
		return &value
	}
	if resource.PBS != nil && resource.PBS.UptimeSeconds > 0 {
		value := resource.PBS.UptimeSeconds
		return &value
	}
	if resource.PMG != nil && resource.PMG.UptimeSeconds > 0 {
		value := resource.PMG.UptimeSeconds
		return &value
	}
	return nil
}

func extractLabels(resource Resource) map[string]string {
	if resource.Kubernetes != nil && len(resource.Kubernetes.Labels) > 0 {
		labels := make(map[string]string, len(resource.Kubernetes.Labels))
		for key, value := range resource.Kubernetes.Labels {
			labels[key] = value
		}
		return labels
	}
	return nil
}

func convertLegacyIdentity(resource Resource, fallbackName string) *LegacyIdentity {
	hostname := ""
	if resource.Agent != nil {
		hostname = strings.TrimSpace(resource.Agent.Hostname)
	}
	if hostname == "" && resource.Docker != nil {
		hostname = strings.TrimSpace(resource.Docker.Hostname)
	}
	if hostname == "" && resource.Proxmox != nil {
		hostname = strings.TrimSpace(resource.Proxmox.NodeName)
	}
	if hostname == "" && len(resource.Identity.Hostnames) > 0 {
		hostname = strings.TrimSpace(resource.Identity.Hostnames[0])
	}
	if hostname == "" {
		hostname = fallbackName
	}

	if hostname == "" && resource.Identity.MachineID == "" && len(resource.Identity.IPAddresses) == 0 {
		return nil
	}

	ips := make([]string, 0, len(resource.Identity.IPAddresses))
	for _, ip := range resource.Identity.IPAddresses {
		trimmed := strings.TrimSpace(ip)
		if trimmed == "" {
			continue
		}
		ips = append(ips, trimmed)
	}

	return &LegacyIdentity{
		Hostname:  hostname,
		MachineID: strings.TrimSpace(resource.Identity.MachineID),
		IPs:       ips,
	}
}

func buildLegacyPlatformData(resource Resource, legacyType LegacyResourceType, platformID string) json.RawMessage {
	var payload interface{}

	switch legacyType {
	case LegacyResourceTypeNode:
		if resource.Proxmox != nil {
			payload = map[string]interface{}{
				"instance":         resource.Proxmox.Instance,
				"host":             "",
				"guestURL":         "",
				"pveVersion":       resource.Proxmox.PVEVersion,
				"kernelVersion":    resource.Proxmox.KernelVersion,
				"cpuInfo":          resource.Proxmox.CPUInfo,
				"loadAverage":      []float64{},
				"isClusterMember":  resource.Proxmox.ClusterName != "",
				"clusterName":      resource.Proxmox.ClusterName,
				"connectionHealth": firstSourceStatus(resource, SourceProxmox),
			}
		}
	case LegacyResourceTypeVM:
		if resource.Proxmox != nil {
			payload = map[string]interface{}{
				"vmid":        resource.Proxmox.VMID,
				"node":        resource.Proxmox.NodeName,
				"instance":    resource.Proxmox.Instance,
				"cpus":        resource.Proxmox.CPUs,
				"template":    resource.Proxmox.Template,
				"networkIn":   metricInt64(resource.Metrics, func(m *ResourceMetrics) *MetricValue { return m.NetIn }),
				"networkOut":  metricInt64(resource.Metrics, func(m *ResourceMetrics) *MetricValue { return m.NetOut }),
				"diskRead":    metricInt64(resource.Metrics, func(m *ResourceMetrics) *MetricValue { return m.DiskRead }),
				"diskWrite":   metricInt64(resource.Metrics, func(m *ResourceMetrics) *MetricValue { return m.DiskWrite }),
				"lastBackup":  resource.Proxmox.LastBackup,
				"ipAddresses": append([]string(nil), resource.Identity.IPAddresses...),
			}
		}
	case LegacyResourceTypeContainer, LegacyResourceTypeOCIContainer:
		if resource.Proxmox != nil {
			payload = map[string]interface{}{
				"vmid":        resource.Proxmox.VMID,
				"node":        resource.Proxmox.NodeName,
				"instance":    resource.Proxmox.Instance,
				"cpus":        resource.Proxmox.CPUs,
				"template":    resource.Proxmox.Template,
				"networkIn":   metricInt64(resource.Metrics, func(m *ResourceMetrics) *MetricValue { return m.NetIn }),
				"networkOut":  metricInt64(resource.Metrics, func(m *ResourceMetrics) *MetricValue { return m.NetOut }),
				"diskRead":    metricInt64(resource.Metrics, func(m *ResourceMetrics) *MetricValue { return m.DiskRead }),
				"diskWrite":   metricInt64(resource.Metrics, func(m *ResourceMetrics) *MetricValue { return m.DiskWrite }),
				"lastBackup":  resource.Proxmox.LastBackup,
				"ipAddresses": append([]string(nil), resource.Identity.IPAddresses...),
			}
		}
	case LegacyResourceTypeHost:
		if resource.Agent != nil {
			payload = map[string]interface{}{
				"platform":      resource.Agent.Platform,
				"osName":        resource.Agent.OSName,
				"osVersion":     resource.Agent.OSVersion,
				"kernelVersion": resource.Agent.KernelVersion,
				"architecture":  resource.Agent.Architecture,
				"agentVersion":  resource.Agent.AgentVersion,
				"interfaces":    resource.Agent.NetworkInterfaces,
				"disks":         resource.Agent.Disks,
			}
		}
	case LegacyResourceTypeDockerHost:
		if resource.Docker != nil {
			payload = map[string]interface{}{
				"agentId":        platformID,
				"runtime":        resource.Docker.Runtime,
				"runtimeVersion": resource.Docker.RuntimeVersion,
				"dockerVersion":  resource.Docker.DockerVersion,
				"os":             resource.Docker.OS,
				"kernelVersion":  resource.Docker.KernelVersion,
				"architecture":   resource.Docker.Architecture,
				"agentVersion":   resource.Docker.AgentVersion,
				"swarm":          resource.Docker.Swarm,
				"interfaces":     resource.Docker.NetworkInterfaces,
				"disks":          resource.Docker.Disks,
			}
		}
	case LegacyResourceTypeDockerContainer:
		if resource.Docker != nil {
			payload = map[string]interface{}{
				"hostId":    valueOrEmpty(resource.ParentID),
				"hostName":  resource.Docker.Hostname,
				"image":     resource.Docker.Image,
				"state":     strings.ToLower(string(resource.Status)),
				"status":    strings.ToLower(string(resource.Status)),
				"health":    "",
				"createdAt": time.Time{},
			}
		}
	case LegacyResourceTypeK8sCluster:
		if resource.Kubernetes != nil {
			payload = map[string]interface{}{
				"agentId":           resource.Kubernetes.AgentID,
				"server":            resource.Kubernetes.Server,
				"context":           resource.Kubernetes.Context,
				"version":           resource.Kubernetes.Version,
				"customDisplayName": "",
				"hidden":            false,
				"pendingUninstall":  resource.Kubernetes.PendingUninstall,
				"nodeCount":         resource.ChildCount,
			}
		}
	case LegacyResourceTypeK8sNode:
		if resource.Kubernetes != nil {
			payload = map[string]interface{}{
				"clusterId":               resource.Kubernetes.ClusterID,
				"ready":                   resource.Kubernetes.Ready,
				"unschedulable":           resource.Kubernetes.Unschedulable,
				"kubeletVersion":          resource.Kubernetes.KubeletVersion,
				"containerRuntimeVersion": resource.Kubernetes.ContainerRuntimeVersion,
				"osImage":                 resource.Kubernetes.OSImage,
				"kernelVersion":           resource.Kubernetes.KernelVersion,
				"architecture":            resource.Kubernetes.Architecture,
				"capacityCpuCores":        resource.Kubernetes.CapacityCPU,
				"capacityMemoryBytes":     resource.Kubernetes.CapacityMemoryBytes,
				"capacityPods":            resource.Kubernetes.CapacityPods,
				"allocatableCpuCores":     resource.Kubernetes.AllocCPU,
				"allocatableMemoryBytes":  resource.Kubernetes.AllocMemoryBytes,
				"allocatablePods":         resource.Kubernetes.AllocPods,
				"roles":                   append([]string(nil), resource.Kubernetes.Roles...),
			}
		}
	case LegacyResourceTypePod:
		if resource.Kubernetes != nil {
			payload = map[string]interface{}{
				"clusterId": resource.Kubernetes.ClusterID,
				"namespace": resource.Kubernetes.Namespace,
				"nodeName":  resource.Kubernetes.NodeName,
				"phase":     resource.Kubernetes.PodPhase,
				"restarts":  resource.Kubernetes.Restarts,
				"ownerKind": resource.Kubernetes.OwnerKind,
				"ownerName": resource.Kubernetes.OwnerName,
			}
		}
	case LegacyResourceTypeK8sDeployment:
		if resource.Kubernetes != nil {
			payload = map[string]interface{}{
				"clusterId":         resource.Kubernetes.ClusterID,
				"namespace":         resource.Kubernetes.Namespace,
				"desiredReplicas":   resource.Kubernetes.DesiredReplicas,
				"updatedReplicas":   resource.Kubernetes.UpdatedReplicas,
				"readyReplicas":     resource.Kubernetes.ReadyReplicas,
				"availableReplicas": resource.Kubernetes.AvailableReplicas,
			}
		}
	case LegacyResourceTypePBS:
		if resource.PBS != nil {
			payload = map[string]interface{}{
				"host":             resource.PBS.Hostname,
				"version":          resource.PBS.Version,
				"connectionHealth": resource.PBS.ConnectionHealth,
				"memoryUsed":       metricUsed(resource.MetricsValueMemory()),
				"memoryTotal":      metricTotal(resource.MetricsValueMemory()),
				"numDatastores":    resource.PBS.DatastoreCount,
			}
		}
	case LegacyResourceTypeDatastore:
		payload = map[string]interface{}{
			"pbsInstanceId":   valueOrEmpty(resource.ParentID),
			"pbsInstanceName": resource.ParentName,
			"content":         "backup",
		}
	case LegacyResourceTypePMG:
		if resource.PMG != nil {
			payload = map[string]interface{}{
				"host":             resource.PMG.Hostname,
				"version":          resource.PMG.Version,
				"connectionHealth": resource.PMG.ConnectionHealth,
				"nodeCount":        resource.PMG.NodeCount,
				"queueActive":      resource.PMG.QueueActive,
				"queueDeferred":    resource.PMG.QueueDeferred,
				"queueHold":        resource.PMG.QueueHold,
				"queueIncoming":    resource.PMG.QueueIncoming,
				"queueTotal":       resource.PMG.QueueTotal,
				"mailCountTotal":   resource.PMG.MailCountTotal,
				"spamIn":           resource.PMG.SpamIn,
				"virusIn":          resource.PMG.VirusIn,
				"lastUpdated":      resource.PMG.LastUpdated,
			}
		}
	case LegacyResourceTypeStorage, LegacyResourceTypePool:
		payload = map[string]interface{}{
			"instance": platformID,
			"node":     valueOrEmpty(resource.ParentID),
			"type":     "",
			"content":  "",
			"shared":   false,
			"enabled":  true,
			"active":   resource.Status == StatusOnline,
		}
	}

	if payload == nil {
		return nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return encoded
}

func projectAlertsForLegacyResource(resource Resource, legacy LegacyResource, targets []SourceTarget, alerts []models.Alert) []LegacyAlert {
	if len(alerts) == 0 {
		return nil
	}

	matchIDs := make(map[string]struct{}, len(targets)+2)
	matchIDs[resource.ID] = struct{}{}
	for _, target := range targets {
		if sourceID := strings.TrimSpace(target.SourceID); sourceID != "" {
			matchIDs[sourceID] = struct{}{}
		}
		if candidateID := strings.TrimSpace(target.CandidateID); candidateID != "" {
			matchIDs[candidateID] = struct{}{}
		}
	}
	if resource.Proxmox != nil && resource.Proxmox.VMID > 0 {
		matchIDs[strconv.Itoa(resource.Proxmox.VMID)] = struct{}{}
	}

	out := make([]LegacyAlert, 0)
	seen := make(map[string]struct{})
	for _, alert := range alerts {
		if alertMatchesLegacyResource(alert, legacy, resource, matchIDs) {
			if _, exists := seen[alert.ID]; exists {
				continue
			}
			seen[alert.ID] = struct{}{}
			out = append(out, LegacyAlert{
				ID:        alert.ID,
				Type:      alert.Type,
				Level:     alert.Level,
				Message:   alert.Message,
				Value:     alert.Value,
				Threshold: alert.Threshold,
				StartTime: alert.StartTime,
			})
		}
	}
	return out
}

func alertMatchesLegacyResource(alert models.Alert, legacy LegacyResource, resource Resource, matchIDs map[string]struct{}) bool {
	resourceID := strings.TrimSpace(alert.ResourceID)
	if resourceID != "" {
		if _, ok := matchIDs[resourceID]; ok {
			return true
		}
	}

	resourceName := strings.TrimSpace(alert.ResourceName)
	if resourceName != "" {
		if strings.EqualFold(resourceName, legacy.Name) || strings.EqualFold(resourceName, legacy.DisplayName) || strings.EqualFold(resourceName, legacy.EffectiveDisplayName()) {
			return true
		}
	}

	node := strings.TrimSpace(alert.Node)
	if node != "" {
		if strings.EqualFold(node, legacy.Name) || strings.EqualFold(node, legacy.DisplayName) {
			return true
		}
		if resource.Proxmox != nil && strings.EqualFold(node, resource.Proxmox.NodeName) {
			return true
		}
		if legacy.Identity != nil && strings.EqualFold(node, legacy.Identity.Hostname) {
			return true
		}
	}

	return false
}

func sourceIDFor(targets []SourceTarget, source DataSource) string {
	for _, target := range targets {
		if target.Source == source {
			return strings.TrimSpace(target.SourceID)
		}
	}
	return ""
}

func hasSource(sources []DataSource, source DataSource) bool {
	for _, s := range sources {
		if s == source {
			return true
		}
	}
	return false
}

func firstSourceID(targets []SourceTarget) string {
	for _, target := range targets {
		if id := strings.TrimSpace(target.SourceID); id != "" {
			return id
		}
	}
	return ""
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstSourceStatus(resource Resource, source DataSource) string {
	if resource.SourceStatus == nil {
		return ""
	}
	if status, ok := resource.SourceStatus[source]; ok {
		return status.Status
	}
	return ""
}

func metricInt64(metrics *ResourceMetrics, pick func(*ResourceMetrics) *MetricValue) int64 {
	if metrics == nil {
		return 0
	}
	metric := pick(metrics)
	if metric == nil {
		return 0
	}
	return int64(math.Round(metric.Value))
}

func metricUsed(metric *MetricValue) int64 {
	if metric == nil || metric.Used == nil {
		return 0
	}
	return *metric.Used
}

func metricTotal(metric *MetricValue) int64 {
	if metric == nil || metric.Total == nil {
		return 0
	}
	return *metric.Total
}

func (r Resource) MetricsValueCPU() *MetricValue {
	if r.Metrics == nil {
		return nil
	}
	return r.Metrics.CPU
}

func (r Resource) MetricsValueMemory() *MetricValue {
	if r.Metrics == nil {
		return nil
	}
	return r.Metrics.Memory
}

func (r Resource) MetricsValueDisk() *MetricValue {
	if r.Metrics == nil {
		return nil
	}
	return r.Metrics.Disk
}

func (r Resource) MetricsValueNetIn() *MetricValue {
	if r.Metrics == nil {
		return nil
	}
	return r.Metrics.NetIn
}

func (r Resource) MetricsValueNetOut() *MetricValue {
	if r.Metrics == nil {
		return nil
	}
	return r.Metrics.NetOut
}
