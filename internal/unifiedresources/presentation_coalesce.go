package unifiedresources

import (
	"strings"
	"time"
)

// CoalescePresentationHostResources collapses split top-level host views for
// API and broadcast presentation. The registry keeps source-native records so
// raw provenance remains available; presentation surfaces should show one
// monitored host when a runtime/platform view and the Pulse agent view share a
// canonical hostname.
func CoalescePresentationHostResources(resources []Resource) []Resource {
	return CoalescePresentationHostResourcesWithExclusions(resources, nil)
}

// CoalescePresentationHostResourcesWithExclusions applies the presentation
// host coalesce while honoring caller-owned split decisions.
func CoalescePresentationHostResourcesWithExclusions(
	resources []Resource,
	excluded func(left, right Resource) bool,
) []Resource {
	coalesced := coalescePresentationHostResourcesOnce(resources, excluded)
	for len(coalesced) < len(resources) {
		next := coalescePresentationHostResourcesOnce(coalesced, excluded)
		if len(next) == len(coalesced) {
			return refreshPresentationProxmoxChildActionAgents(next)
		}
		resources = coalesced
		coalesced = next
	}
	return refreshPresentationProxmoxChildActionAgents(coalesced)
}

func coalescePresentationHostResourcesOnce(
	resources []Resource,
	excluded func(left, right Resource) bool,
) []Resource {
	if len(resources) == 0 {
		return resources
	}

	coalesced := make([]Resource, 0, len(resources))
	indexByHostKey := make(map[string]int, len(resources))
	parentRedirects := make(map[string]string)
	for _, resource := range resources {
		resource.Type = CanonicalResourceType(resource.Type)
		hostKey := presentationHostMergeKey(resource)
		if hostKey == "" {
			coalesced = append(coalesced, resource)
			continue
		}

		existingIndex, ok := indexByHostKey[hostKey]
		if !ok {
			indexByHostKey[hostKey] = len(coalesced)
			coalesced = append(coalesced, resource)
			continue
		}

		existing := coalesced[existingIndex]
		if excluded != nil && excluded(existing, resource) {
			coalesced = append(coalesced, resource)
			continue
		}
		if !shouldMergePresentationHostResources(existing, resource) {
			coalesced = append(coalesced, resource)
			continue
		}
		merged := mergePresentationHostResources(existing, resource)
		coalesced[existingIndex] = merged
		addPresentationParentRedirect(parentRedirects, existing.ID, merged.ID)
		addPresentationParentRedirect(parentRedirects, resource.ID, merged.ID)
	}

	applyPresentationParentRedirects(coalesced, parentRedirects)
	return coalesced
}

func addPresentationParentRedirect(redirects map[string]string, fromID, toID string) {
	fromID = CanonicalResourceID(strings.TrimSpace(fromID))
	toID = CanonicalResourceID(strings.TrimSpace(toID))
	if fromID == "" || toID == "" || fromID == toID {
		return
	}
	redirects[fromID] = toID
}

func applyPresentationParentRedirects(resources []Resource, redirects map[string]string) {
	if len(redirects) == 0 {
		return
	}
	for i := range resources {
		if resources[i].ParentID == nil {
			continue
		}
		parentID := CanonicalResourceID(strings.TrimSpace(*resources[i].ParentID))
		if redirectedID := redirects[parentID]; redirectedID != "" {
			resources[i].ParentID = &redirectedID
		}
	}
}

func refreshPresentationProxmoxChildActionAgents(resources []Resource) []Resource {
	parentByID := make(map[string]int, len(resources))
	for i := range resources {
		resourceID := CanonicalResourceID(strings.TrimSpace(resources[i].ID))
		if resourceID != "" {
			parentByID[resourceID] = i
		}
	}

	for i := range resources {
		switch CanonicalResourceType(resources[i].Type) {
		case ResourceTypeVM, ResourceTypeSystemContainer:
		default:
			continue
		}
		if resources[i].Proxmox == nil || resources[i].ParentID == nil {
			continue
		}
		parentID := CanonicalResourceID(strings.TrimSpace(*resources[i].ParentID))
		parentIndex, ok := parentByID[parentID]
		if !ok {
			continue
		}
		attachLinkedAgentID(resources[i].Proxmox, linkedAgentIDFromResource(&resources[parentIndex]))
	}
	return resources
}

func presentationHostMergeKey(resource Resource) string {
	if CanonicalResourceType(resource.Type) != ResourceTypeAgent {
		return ""
	}

	candidates := []string{}
	if resource.Canonical != nil {
		candidates = append(candidates, resource.Canonical.PlatformID, resource.Canonical.Hostname)
	}
	candidates = append(candidates, resource.Identity.Hostnames...)
	if resource.Agent != nil {
		candidates = append(candidates, resource.Agent.Hostname)
	}
	if resource.Proxmox != nil {
		candidates = append(candidates, resource.Proxmox.NodeName)
	}
	candidates = append(candidates, resource.Name)

	for _, candidate := range candidates {
		normalized := NormalizeHostname(candidate)
		if normalized != "" {
			return "agent:" + normalized
		}
	}
	return ""
}

func shouldMergePresentationHostResources(left, right Resource) bool {
	if CanonicalResourceType(left.Type) != ResourceTypeAgent ||
		CanonicalResourceType(right.Type) != ResourceTypeAgent {
		return false
	}
	sources := mergePresentationSources(presentationResourceSources(left), presentationResourceSources(right))
	return presentationHasSource(sources, SourceAgent) && presentationHasRuntimePlatformSource(sources)
}

func presentationResourceSources(resource Resource) []DataSource {
	sources := append([]DataSource(nil), resource.Sources...)
	for source := range resource.SourceStatus {
		sources = append(sources, source)
	}
	if resource.Agent != nil {
		sources = append(sources, SourceAgent)
	}
	if resource.Proxmox != nil {
		sources = append(sources, SourceProxmox)
	}
	if resource.Docker != nil {
		sources = append(sources, SourceDocker)
	}
	if resource.Kubernetes != nil {
		sources = append(sources, SourceK8s)
	}
	if resource.VMware != nil {
		sources = append(sources, SourceVMware)
	}
	if resource.TrueNAS != nil {
		sources = append(sources, SourceTrueNAS)
	}
	return mergePresentationSources(nil, sources)
}

func presentationHasRuntimePlatformSource(sources []DataSource) bool {
	for _, source := range []DataSource{
		SourceProxmox,
		SourceDocker,
		SourceK8s,
		SourceVMware,
		SourceTrueNAS,
	} {
		if presentationHasSource(sources, source) {
			return true
		}
	}
	return false
}

func presentationHasSource(sources []DataSource, target DataSource) bool {
	for _, source := range sources {
		if source == target {
			return true
		}
	}
	return false
}

func mergePresentationSources(left, right []DataSource) []DataSource {
	merged := make([]DataSource, 0, len(left)+len(right))
	seen := make(map[DataSource]struct{}, len(left)+len(right))
	for _, source := range append(append([]DataSource(nil), left...), right...) {
		if strings.TrimSpace(string(source)) == "" {
			continue
		}
		if _, ok := seen[source]; ok {
			continue
		}
		seen[source] = struct{}{}
		merged = append(merged, source)
	}
	return merged
}

func mergePresentationHostResources(left, right Resource) Resource {
	primary, secondary := left, right
	if preferPresentationHostPrimary(right, left) {
		primary, secondary = right, left
	}

	merged := primary
	merged.Sources = mergePresentationSources(presentationResourceSources(primary), presentationResourceSources(secondary))
	merged.SourceStatus = mergePresentationSourceStatus(primary.SourceStatus, secondary.SourceStatus, merged.Sources, primary.LastSeen, secondary.LastSeen)
	merged.Identity = mergePresentationIdentity(primary.Identity, secondary.Identity)

	if merged.Agent == nil {
		merged.Agent = secondary.Agent
	}
	if merged.Proxmox == nil {
		merged.Proxmox = secondary.Proxmox
	}
	if merged.Docker == nil {
		merged.Docker = secondary.Docker
	}
	if merged.Kubernetes == nil {
		merged.Kubernetes = secondary.Kubernetes
	}
	if merged.VMware == nil {
		merged.VMware = secondary.VMware
	}
	if merged.TrueNAS == nil {
		merged.TrueNAS = secondary.TrueNAS
	}
	if merged.Storage == nil {
		merged.Storage = secondary.Storage
	}
	if merged.Metrics == nil {
		merged.Metrics = secondary.Metrics
	} else if secondary.Metrics != nil {
		now := time.Now().UTC()
		combinedStatus := make(map[DataSource]SourceStatus, len(primary.SourceStatus)+len(secondary.SourceStatus))
		for source, status := range secondary.SourceStatus {
			combinedStatus[source] = status
		}
		for source, status := range primary.SourceStatus {
			combinedStatus[source] = status
		}
		stale := func(source DataSource) bool {
			return presentationSourceStale(now, combinedStatus, source)
		}
		merged.Metrics = mergePresentationMetrics(merged.Metrics, secondary.Metrics, stale)
	}
	if merged.DiscoveryTarget == nil {
		merged.DiscoveryTarget = secondary.DiscoveryTarget
	}
	if merged.MetricsTarget == nil {
		merged.MetricsTarget = secondary.MetricsTarget
	}
	if merged.Canonical == nil {
		merged.Canonical = secondary.Canonical
	}
	merged.Tags = uniquePresentationStrings(append(append([]string(nil), secondary.Tags...), primary.Tags...))
	merged.Incidents = append(append([]ResourceIncident(nil), secondary.Incidents...), primary.Incidents...)
	if secondary.LastSeen.After(merged.LastSeen) {
		merged.LastSeen = secondary.LastSeen
	}
	if secondary.UpdatedAt.After(merged.UpdatedAt) {
		merged.UpdatedAt = secondary.UpdatedAt
	}
	merged.Status = betterPresentationStatus(merged.Status, secondary.Status)
	return merged
}

func preferPresentationHostPrimary(candidate, other Resource) bool {
	candidateHasAgent := presentationHasSource(presentationResourceSources(candidate), SourceAgent)
	otherHasAgent := presentationHasSource(presentationResourceSources(other), SourceAgent)
	if candidateHasAgent != otherHasAgent {
		return candidateHasAgent
	}
	if candidate.LastSeen.Equal(other.LastSeen) {
		return strings.TrimSpace(candidate.ID) < strings.TrimSpace(other.ID)
	}
	return candidate.LastSeen.After(other.LastSeen)
}

func mergePresentationSourceStatus(
	left, right map[DataSource]SourceStatus,
	sources []DataSource,
	leftLastSeen time.Time,
	rightLastSeen time.Time,
) map[DataSource]SourceStatus {
	merged := make(map[DataSource]SourceStatus, len(sources))
	for source, status := range right {
		merged[source] = status
	}
	for source, status := range left {
		merged[source] = status
	}
	for _, source := range sources {
		if _, ok := merged[source]; ok {
			continue
		}
		lastSeen := leftLastSeen
		if rightLastSeen.After(lastSeen) {
			lastSeen = rightLastSeen
		}
		merged[source] = SourceStatus{Status: "online", LastSeen: lastSeen}
	}
	return merged
}

func mergePresentationIdentity(left, right ResourceIdentity) ResourceIdentity {
	merged := left
	if merged.MachineID == "" {
		merged.MachineID = right.MachineID
	}
	if merged.DMIUUID == "" {
		merged.DMIUUID = right.DMIUUID
	}
	if merged.ClusterName == "" {
		merged.ClusterName = right.ClusterName
	}
	merged.Hostnames = uniquePresentationStrings(append(append([]string(nil), left.Hostnames...), right.Hostnames...))
	merged.IPAddresses = uniquePresentationStrings(append(append([]string(nil), left.IPAddresses...), right.IPAddresses...))
	merged.MACAddresses = uniquePresentationStrings(append(append([]string(nil), left.MACAddresses...), right.MACAddresses...))
	return merged
}

func uniquePresentationStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		unique = append(unique, trimmed)
	}
	return unique
}

// presentationSourceStale reports whether a source's most recent report is
// older than its stale threshold. A zero/unknown last-seen is treated as NOT
// stale so coalescing never demotes a source on missing information.
func presentationSourceStale(now time.Time, status map[DataSource]SourceStatus, source DataSource) bool {
	if status == nil {
		return false
	}
	st, ok := status[source]
	if !ok || st.LastSeen.IsZero() {
		return false
	}
	threshold, ok := defaultStaleThresholds[source]
	if !ok {
		threshold = 60 * time.Second
	}
	return now.Sub(st.LastSeen) > threshold
}

// mergePresentationMetric keeps the primary (left) value, except that a metric
// from a stale source must never win over a metric from a live one. Without
// this, a Proxmox node whose Pulse Agent has gone offline shows the agent's
// last (usually 0) CPU instead of the live PVE value, because the agent is the
// presentation primary and its 0 reading is non-nil.
func mergePresentationMetric(left, right *MetricValue, stale func(DataSource) bool) *MetricValue {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	if stale(left.Source) && !stale(right.Source) {
		return right
	}
	return left
}

func mergePresentationMetrics(left, right *ResourceMetrics, stale func(DataSource) bool) *ResourceMetrics {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	merged := *left
	merged.CPU = mergePresentationMetric(left.CPU, right.CPU, stale)
	merged.Memory = mergePresentationMetric(left.Memory, right.Memory, stale)
	merged.Disk = mergePresentationMetric(left.Disk, right.Disk, stale)
	merged.NetIn = mergePresentationMetric(left.NetIn, right.NetIn, stale)
	merged.NetOut = mergePresentationMetric(left.NetOut, right.NetOut, stale)
	merged.DiskRead = mergePresentationMetric(left.DiskRead, right.DiskRead, stale)
	merged.DiskWrite = mergePresentationMetric(left.DiskWrite, right.DiskWrite, stale)
	return &merged
}

func betterPresentationStatus(left, right ResourceStatus) ResourceStatus {
	rank := map[ResourceStatus]int{
		StatusOnline:  4,
		StatusWarning: 3,
		StatusUnknown: 2,
		StatusOffline: 1,
	}
	if rank[right] > rank[left] {
		return right
	}
	return left
}
