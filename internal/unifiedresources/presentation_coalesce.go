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
	indexesByHostKey := make(map[string][]int, len(resources))
	parentRedirects := make(map[string]string)
	guardedHostKeys := presentationAmbiguousProxmoxHostKeys(resources)
	for _, resource := range resources {
		resource.Type = CanonicalResourceType(resource.Type)
		hostKey := presentationHostMergeKey(resource)
		if hostKey == "" {
			coalesced = append(coalesced, resource)
			continue
		}

		// The host key is the short hostname, so distinct machines with
		// dotted hostnames (cloud.rnd-lax1 vs cloud.gce-or1) share a bucket;
		// the hostname-compatibility check keeps them from merging while a
		// short name still pairs with its own FQDN (web01 vs web01.lan).
		merged := false
		for _, existingIndex := range indexesByHostKey[hostKey] {
			existing := coalesced[existingIndex]
			if excluded != nil && excluded(existing, resource) {
				continue
			}
			if presentationHostIdentitiesDistinct(existing, resource) {
				continue
			}
			if guardedHostKeys[hostKey] && !presentationGuardedMergeAllowed(existing, resource) {
				continue
			}
			if !presentationHostnamesCompatible(existing, resource) {
				continue
			}
			if !shouldMergePresentationHostResources(existing, resource) {
				continue
			}
			mergedResource := mergePresentationHostResources(existing, resource)
			coalesced[existingIndex] = mergedResource
			addPresentationParentRedirect(parentRedirects, existing.ID, mergedResource.ID)
			addPresentationParentRedirect(parentRedirects, resource.ID, mergedResource.ID)
			merged = true
			break
		}
		if !merged {
			indexesByHostKey[hostKey] = append(indexesByHostKey[hostKey], len(coalesced))
			coalesced = append(coalesced, resource)
		}
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

	for _, candidate := range presentationHostnameCandidates(resource) {
		normalized := NormalizeHostname(candidate)
		if normalized != "" {
			return "agent:" + normalized
		}
	}
	return ""
}

func presentationHostnameCandidates(resource Resource) []string {
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
	return candidates
}

// presentationMachineIDs returns the hardware machine identifiers a host row
// carries.
func presentationMachineIDs(resource Resource) []string {
	candidates := []string{resource.Identity.MachineID}
	if resource.Agent != nil {
		candidates = append(candidates, resource.Agent.MachineID)
	}
	ids := candidates[:0]
	for _, id := range candidates {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func presentationMachineIDsOverlap(left, right []string) bool {
	for _, leftID := range left {
		for _, rightID := range right {
			if strings.EqualFold(leftID, rightID) {
				return true
			}
		}
	}
	return false
}

// presentationHostIdentitiesDistinct reports whether two host views carry
// identity proving they are different machines. Distinct machines share a
// merge bucket whenever both report only a short hostname (#1753: a pve01 in
// staging and a pve01 in production), so a differing machine ID, DMI UUID, or
// Proxmox cluster membership must veto the merge no matter what the hostname
// comparison concludes. Equal cluster names deliberately do not veto: the same
// cluster added under two connections still describes the same machines.
func presentationHostIdentitiesDistinct(left, right Resource) bool {
	leftIDs, rightIDs := presentationMachineIDs(left), presentationMachineIDs(right)
	if len(leftIDs) > 0 && len(rightIDs) > 0 && !presentationMachineIDsOverlap(leftIDs, rightIDs) {
		return true
	}
	if presentationIdentityValuesConflict(left.Identity.DMIUUID, right.Identity.DMIUUID) {
		return true
	}
	if left.Proxmox != nil && right.Proxmox != nil &&
		presentationIdentityValuesConflict(left.Proxmox.ClusterName, right.Proxmox.ClusterName) {
		return true
	}
	if presentationProxmoxNodeScopesDistinct(left.Proxmox, right.Proxmox) {
		return true
	}
	return false
}

// presentationProxmoxNodeScopesDistinct reports whether two Proxmox node
// facets describe different provider connections without any same-machine
// proof. Standalone connections are provider scopes too, and both machines in
// two hand-added sites are commonly just "pve" (#1753), so a shared short
// hostname must not fold one site's node row into the other's. The proof
// mirrors the state-layer rule: the same connection instance, the same node
// identity, or the same endpoint host still merge; anything less keeps the
// rows apart. Cluster names are operator-chosen display labels and are not
// globally unique, so an equal label across provider instances is not
// same-machine proof.
func presentationProxmoxNodeScopesDistinct(left, right *ProxmoxData) bool {
	if left == nil || right == nil {
		return false
	}
	if strings.TrimSpace(left.NodeName) == "" || strings.TrimSpace(right.NodeName) == "" {
		return false
	}
	if !presentationIdentityValuesConflict(left.Instance, right.Instance) {
		return false
	}
	if presentationIdentityValuesEqual(left.NodeIdentity, right.NodeIdentity) {
		return false
	}
	if presentationIdentityValuesEqual(extractHostname(left.HostURL), extractHostname(right.HostURL)) {
		return false
	}
	return true
}

func presentationIdentityValuesEqual(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && right != "" && strings.EqualFold(left, right)
}

// presentationAmbiguousProxmoxHostKeys marks merge buckets that contain node
// facets from two or more distinct Proxmox provider scopes. Inside such a
// bucket a bare shared hostname no longer identifies a machine, so
// hostname-only merges of an agent row into a node row must fail closed and
// only the state-layer agent link may attach one (#1753: the Agent badge and
// per-node stats jumped between two sites named "pve" on every refresh).
func presentationAmbiguousProxmoxHostKeys(resources []Resource) map[string]bool {
	facetsByKey := make(map[string][]*ProxmoxData)
	guarded := make(map[string]bool)
	for i := range resources {
		facet := resources[i].Proxmox
		if facet == nil || strings.TrimSpace(facet.NodeName) == "" {
			continue
		}
		hostKey := presentationHostMergeKey(resources[i])
		if hostKey == "" || guarded[hostKey] {
			continue
		}
		for _, existing := range facetsByKey[hostKey] {
			if presentationProxmoxNodeScopesDistinct(existing, facet) {
				guarded[hostKey] = true
				break
			}
		}
		facetsByKey[hostKey] = append(facetsByKey[hostKey], facet)
	}
	return guarded
}

// presentationGuardedMergeAllowed gates merges inside an ambiguous bucket.
// A pair of node facets is already governed by the scope veto, and agent-only
// pairs never satisfy the runtime-platform source requirement, so the case
// that matters is an agent row meeting a node row: it may only attach to the
// node whose state-layer agent link names it.
func presentationGuardedMergeAllowed(left, right Resource) bool {
	leftFacet := left.Proxmox != nil && strings.TrimSpace(left.Proxmox.NodeName) != ""
	rightFacet := right.Proxmox != nil && strings.TrimSpace(right.Proxmox.NodeName) != ""
	if leftFacet == rightFacet {
		return true
	}
	node, agent := left, right
	if rightFacet {
		node, agent = right, left
	}
	if agent.Agent == nil {
		return false
	}
	return presentationIdentityValuesEqual(node.Proxmox.LinkedAgentID, agent.Agent.AgentID)
}

func presentationIdentityValuesConflict(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && right != "" && !strings.EqualFold(left, right)
}

// presentationHostnamesCompatible reports whether two host views may describe
// the same machine. Sharing a short hostname is not enough: distinct dotted
// hostnames (cloud.rnd-lax1 vs cloud.gce-or1) belong to distinct machines,
// while a short name still pairs with its own FQDN (web01 vs web01.lan).
func presentationHostnamesCompatible(left, right Resource) bool {
	for _, leftName := range presentationHostnameCandidates(left) {
		leftFull := NormalizeFullHostname(leftName)
		if leftFull == "" {
			continue
		}
		for _, rightName := range presentationHostnameCandidates(right) {
			rightFull := NormalizeFullHostname(rightName)
			if rightFull == "" {
				continue
			}
			if leftFull == rightFull || HostnamesEquivalent(leftFull, rightFull) {
				return true
			}
		}
	}
	return false
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
		// The Proxmox node row's name is display-name aware (the configured
		// Node Name), while an agent-backed primary is named after the bare
		// reported hostname. Keep the configured name on the merged row
		// (#1753: "Node Name field not observed").
		if merged.Proxmox != nil && strings.TrimSpace(secondary.Name) != "" {
			merged.Name = secondary.Name
		}
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
	if merged.VirtualMachine == nil {
		merged.VirtualMachine = secondary.VirtualMachine
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
		merged[source] = SourceStatus{Status: sourceSightingStatus(lastSeen), LastSeen: lastSeen}
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
