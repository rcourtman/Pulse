package unifiedresources

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// TopLevelSystemResolver groups infrastructure resources into canonical
// top-level monitored systems using one shared identity contract.
type TopLevelSystemResolver struct {
	groups          []topLevelSystemResolvedGroup
	resourceToGroup map[string]string
}

type topLevelSystemResolvedGroup struct {
	id           string
	resources    []*Resource
	exactHosts   map[string]struct{}
	exactIPs     map[string]struct{}
	strongIDs    map[string]struct{}
	priority     int
	attachByHost bool
	explanation  MonitoredSystemGroupingExplanation
}

type topLevelSystemNode struct {
	resource     *Resource
	identity     ResourceIdentity
	strongIDs    map[string]struct{}
	exactHosts   map[string]struct{}
	exactIPs     map[string]struct{}
	priority     int
	attachByHost bool
}

type topLevelSystemGroupingEvidence struct {
	kind     string
	signal   string
	value    string
	fromType string
	toType   string
}

type topLevelSystemFallbackTarget struct {
	root     int
	evidence topLevelSystemGroupingEvidence
}

// ResolveTopLevelSystems builds the shared top-level system grouping used by
// monitored-system counting and connected-infrastructure inventory surfaces.
func ResolveTopLevelSystems(resources []Resource) TopLevelSystemResolver {
	if len(resources) == 0 {
		return TopLevelSystemResolver{
			groups:          nil,
			resourceToGroup: map[string]string{},
		}
	}

	nodes := make([]topLevelSystemNode, 0, len(resources))
	for i := range resources {
		node := buildTopLevelSystemNode(&resources[i])
		if node.resource == nil {
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return TopLevelSystemResolver{
			groups:          nil,
			resourceToGroup: map[string]string{},
		}
	}

	parent := make([]int, len(nodes))
	for i := range parent {
		parent[i] = i
	}
	groupEvidence := make(map[int][]topLevelSystemGroupingEvidence, len(nodes))

	find := func(id int) int {
		for parent[id] != id {
			parent[id] = parent[parent[id]]
			id = parent[id]
		}
		return id
	}

	union := func(left, right int, evidence topLevelSystemGroupingEvidence) {
		leftRoot := find(left)
		rightRoot := find(right)
		if leftRoot == rightRoot {
			return
		}
		if existing := groupEvidence[leftRoot]; len(existing) > 0 {
			groupEvidence[rightRoot] = append(groupEvidence[rightRoot], existing...)
			delete(groupEvidence, leftRoot)
		}
		parent[leftRoot] = rightRoot
		if evidence.kind != "" {
			groupEvidence[rightRoot] = append(groupEvidence[rightRoot], evidence)
		}
	}

	strongIDOwners := make(map[string]int)
	for i := range nodes {
		for _, strongID := range topLevelSystemOrderedStrongIDs(nodes[i].strongIDs) {
			if existing, ok := strongIDOwners[strongID]; ok {
				union(i, existing, topLevelSystemStrongIDEvidence(nodes[i].resource, nodes[existing].resource, strongID))
				continue
			}
			strongIDOwners[strongID] = i
		}
	}

	matcher := NewIdentityMatcher()
	nodeIDByMatcherID := make(map[string]int, len(nodes))
	for i := range nodes {
		matcherID := topLevelSystemMatcherID(i)
		nodeIDByMatcherID[matcherID] = i
		matcher.Add(matcherID, nodes[i].identity)
	}
	for i := range nodes {
		matches := matcher.FindCandidates(nodes[i].identity)
		for _, match := range matches {
			if match.Confidence < HighConfidenceThreshold {
				continue
			}
			existingIndex, ok := nodeIDByMatcherID[match.ID]
			if !ok || existingIndex == i {
				continue
			}
			union(i, existingIndex, topLevelSystemIdentityMatchEvidence(nodes[i].resource, nodes[existingIndex].resource, match.Reason))
		}
	}

	for {
		initialGroups := buildTopLevelSystemResolvedGroups(nodes, parent, groupEvidence)
		hostOwners, ipOwners := buildTopLevelSystemFallbackOwners(initialGroups)
		attached := false

		for groupRoot, group := range initialGroups {
			if !group.attachByHost {
				continue
			}
			target, ok := uniqueBetterTopLevelSystemTarget(groupRoot, group, hostOwners, ipOwners, initialGroups)
			if !ok {
				continue
			}
			union(groupRoot, target.root, target.evidence)
			attached = true
		}

		if !attached {
			finalGroups := buildTopLevelSystemResolvedGroups(nodes, parent, groupEvidence)
			orderedGroups := make([]topLevelSystemResolvedGroup, 0, len(finalGroups))
			for _, group := range finalGroups {
				orderedGroups = append(orderedGroups, group)
			}
			sort.Slice(orderedGroups, func(i, j int) bool {
				if orderedGroups[i].priority != orderedGroups[j].priority {
					return orderedGroups[i].priority < orderedGroups[j].priority
				}
				if len(orderedGroups[i].resources) == 0 || len(orderedGroups[j].resources) == 0 {
					return orderedGroups[i].id < orderedGroups[j].id
				}
				left := monitoredSystemDisplayName(orderedGroups[i].resources, preferredMonitoredSystemResource(orderedGroups[i].resources))
				right := monitoredSystemDisplayName(orderedGroups[j].resources, preferredMonitoredSystemResource(orderedGroups[j].resources))
				if left == right {
					return orderedGroups[i].id < orderedGroups[j].id
				}
				return left < right
			})

			resourceToGroup := make(map[string]string, len(nodes))
			for _, group := range orderedGroups {
				for _, resource := range group.resources {
					if resource == nil || strings.TrimSpace(resource.ID) == "" {
						continue
					}
					resourceToGroup[strings.TrimSpace(resource.ID)] = group.id
				}
			}

			return TopLevelSystemResolver{
				groups:          orderedGroups,
				resourceToGroup: resourceToGroup,
			}
		}
	}
}

// GroupIDForResource returns the canonical top-level system group id for a
// resource, or empty when the resource was not part of this resolver.
func (r TopLevelSystemResolver) GroupIDForResource(resource Resource) string {
	return r.resourceToGroup[strings.TrimSpace(resource.ID)]
}

// Count returns the number of grouped top-level monitored systems.
func (r TopLevelSystemResolver) Count() int {
	return len(r.groups)
}

func (r TopLevelSystemResolver) records() []MonitoredSystemRecord {
	records := make([]MonitoredSystemRecord, 0, len(r.groups))
	for _, group := range r.groups {
		records = append(records, monitoredSystemRecord(monitoredSystemGroup{
			keys:        cloneStringSet(group.strongIDs),
			resources:   group.resources,
			explanation: group.explanation,
		}))
	}
	return records
}

func (r TopLevelSystemResolver) HasMatchingCandidate(candidate MonitoredSystemCandidate) bool {
	strongIDs := monitoredSystemCandidateStrongIDs(candidate)
	for _, group := range r.groups {
		if topLevelSystemSetsOverlap(group.strongIDs, strongIDs) {
			return true
		}
	}

	if !monitoredSystemCandidateAllowsHostAttachment(candidate) {
		return false
	}

	groupByID := make(map[string]topLevelSystemResolvedGroup, len(r.groups))
	hostOwners := make(map[string]map[string]struct{})
	ipOwners := make(map[string]map[string]struct{})
	for _, group := range r.groups {
		groupByID[group.id] = group
		for host := range group.exactHosts {
			addTopLevelSystemOwner(hostOwners, host, group.id)
		}
		for ip := range group.exactIPs {
			addTopLevelSystemOwner(ipOwners, ip, group.id)
		}
	}

	targetIDs := candidateExactTargetGroups(candidate, hostOwners, ipOwners, groupByID, monitoredSystemCandidatePriority(candidate))
	return len(targetIDs) == 1
}

func buildTopLevelSystemResolvedGroups(
	nodes []topLevelSystemNode,
	parent []int,
	groupEvidence map[int][]topLevelSystemGroupingEvidence,
) map[int]topLevelSystemResolvedGroup {
	find := func(id int) int {
		for parent[id] != id {
			parent[id] = parent[parent[id]]
			id = parent[id]
		}
		return id
	}

	groupIndex := make(map[int]*topLevelSystemResolvedGroup)
	for i := range nodes {
		root := find(i)
		group := groupIndex[root]
		if group == nil {
			group = &topLevelSystemResolvedGroup{
				exactHosts: make(map[string]struct{}),
				exactIPs:   make(map[string]struct{}),
				strongIDs:  make(map[string]struct{}),
				priority:   nodes[i].priority,
			}
			groupIndex[root] = group
		}
		group.resources = append(group.resources, nodes[i].resource)
		if nodes[i].priority < group.priority {
			group.priority = nodes[i].priority
		}
		if nodes[i].attachByHost {
			group.attachByHost = true
		}
		for host := range nodes[i].exactHosts {
			group.exactHosts[host] = struct{}{}
		}
		for ip := range nodes[i].exactIPs {
			group.exactIPs[ip] = struct{}{}
		}
		for strongID := range nodes[i].strongIDs {
			group.strongIDs[strongID] = struct{}{}
		}
	}

	groups := make(map[int]topLevelSystemResolvedGroup, len(groupIndex))
	for root, group := range groupIndex {
		group.id = topLevelSystemGroupID(*group)
		group.explanation = topLevelSystemGroupingExplanation(*group, groupEvidence[root])
		groups[root] = *group
	}

	return groups
}

func buildTopLevelSystemNode(resource *Resource) topLevelSystemNode {
	if resource == nil {
		return topLevelSystemNode{}
	}

	exactHosts := topLevelSystemExactHosts(*resource)
	exactIPs := topLevelSystemExactIPs(*resource)
	identity := cloneResourceIdentity(resource.Identity)
	identity.Hostnames = uniqueStrings(append(cloneStringSlice(identity.Hostnames), topLevelSystemSortedSet(exactHosts)...))
	identity.IPAddresses = uniqueStrings(append(cloneStringSlice(identity.IPAddresses), topLevelSystemSortedSet(exactIPs)...))

	return topLevelSystemNode{
		resource:     resource,
		identity:     identity,
		strongIDs:    topLevelSystemStrongIDs(*resource),
		exactHosts:   exactHosts,
		exactIPs:     exactIPs,
		priority:     monitoredSystemResourcePriority(resource),
		attachByHost: monitoredSystemResourceAllowsHostAttachment(resource),
	}
}

// When adding a new top-level monitored-system source, update:
// - topLevelSystemStrongIDs
// - topLevelSystemExactHosts
// - topLevelSystemExactIPs
// - monitoredSystemCandidateStrongIDs when candidate matching is exposed
// - TestResolveTopLevelSystemsTopLevelSourceMatrix
// - TestResolveTopLevelSystemsMixedEnvironmentCharacterization
func topLevelSystemStrongIDs(resource Resource) map[string]struct{} {
	ids := make(map[string]struct{})

	if canonical := resource.Canonical; canonical != nil {
		if primaryID := strings.TrimSpace(canonical.PrimaryID); primaryID != "" {
			ids["primary:"+primaryID] = struct{}{}
		}
	}

	if resourceID := strings.TrimSpace(resource.ID); resourceID != "" {
		ids["resource:"+resourceID] = struct{}{}
	}
	if machineID := strings.TrimSpace(resource.Identity.MachineID); machineID != "" {
		ids["machine:"+machineID] = struct{}{}
	}
	if resource.Agent != nil {
		if agentID := strings.TrimSpace(resource.Agent.AgentID); agentID != "" {
			ids["agent:"+agentID] = struct{}{}
		}
	}
	if resource.Docker != nil {
		if hostSourceID := strings.TrimSpace(resource.Docker.HostSourceID); hostSourceID != "" {
			ids["docker:"+hostSourceID] = struct{}{}
		}
		if agentID := strings.TrimSpace(resource.Docker.AgentID); agentID != "" {
			ids["agent:"+agentID] = struct{}{}
		}
	}
	if resource.Proxmox != nil {
		if sourceID := strings.TrimSpace(resource.Proxmox.SourceID); sourceID != "" {
			ids["proxmox:"+sourceID] = struct{}{}
		}
	}
	if resource.PBS != nil {
		if instanceID := strings.TrimSpace(resource.PBS.InstanceID); instanceID != "" {
			ids["pbs:"+instanceID] = struct{}{}
		}
	}
	if resource.PMG != nil {
		if instanceID := strings.TrimSpace(resource.PMG.InstanceID); instanceID != "" {
			ids["pmg:"+instanceID] = struct{}{}
		}
	}
	if resource.Kubernetes != nil {
		if clusterID := strings.TrimSpace(resource.Kubernetes.ClusterID); clusterID != "" {
			ids["k8s:"+clusterID] = struct{}{}
		}
	}

	return ids
}

func topLevelSystemExactHosts(resource Resource) map[string]struct{} {
	hosts := make(map[string]struct{})
	for _, candidate := range []string{
		canonicalAgentHostname(resource),
		canonicalDockerHostname(resource),
		canonicalPBSHostname(resource),
		canonicalPMGHostname(resource),
		canonicalTrueNASHostname(resource),
		canonicalProxmoxNodeName(resource),
		extractHostname(firstTrimmed(topLevelSystemProxmoxHostURL(resource))),
		extractHostname(firstTrimmed(topLevelSystemPBSHostURL(resource))),
		extractHostname(firstTrimmed(topLevelSystemKubernetesServer(resource))),
		firstTrimmed(topLevelSystemCanonicalHostname(resource)),
	} {
		if normalized := topLevelSystemNormalizeHost(candidate); normalized != "" {
			hosts[normalized] = struct{}{}
		}
	}
	return hosts
}

func topLevelSystemExactIPs(resource Resource) map[string]struct{} {
	ips := make(map[string]struct{})
	for _, candidate := range resource.Identity.IPAddresses {
		if normalized := NormalizeIP(candidate); normalized != "" && !isNonUniqueIP(normalized) {
			ips[normalized] = struct{}{}
		}
	}
	for _, raw := range []string{
		firstTrimmed(topLevelSystemProxmoxHostURL(resource)),
		firstTrimmed(topLevelSystemPBSHostURL(resource)),
		firstTrimmed(topLevelSystemKubernetesServer(resource)),
	} {
		host := extractHostname(raw)
		if normalized := NormalizeIP(host); normalized != "" && !isNonUniqueIP(normalized) {
			ips[normalized] = struct{}{}
		}
	}
	return ips
}

func topLevelSystemMatcherID(index int) string {
	return "top-level-system:" + strconv.Itoa(index)
}

func buildTopLevelSystemFallbackOwners(
	groups map[int]topLevelSystemResolvedGroup,
) (map[string]map[int]struct{}, map[string]map[int]struct{}) {
	hostOwners := make(map[string]map[int]struct{})
	ipOwners := make(map[string]map[int]struct{})
	for groupRoot, group := range groups {
		for host := range group.exactHosts {
			bucket := hostOwners[host]
			if bucket == nil {
				bucket = make(map[int]struct{})
				hostOwners[host] = bucket
			}
			bucket[groupRoot] = struct{}{}
		}
		for ip := range group.exactIPs {
			bucket := ipOwners[ip]
			if bucket == nil {
				bucket = make(map[int]struct{})
				ipOwners[ip] = bucket
			}
			bucket[groupRoot] = struct{}{}
		}
	}
	return hostOwners, ipOwners
}

func uniqueBetterTopLevelSystemTarget(
	groupRoot int,
	group topLevelSystemResolvedGroup,
	hostOwners map[string]map[int]struct{},
	ipOwners map[string]map[int]struct{},
	groups map[int]topLevelSystemResolvedGroup,
) (topLevelSystemFallbackTarget, bool) {
	targets := make(map[int]topLevelSystemGroupingEvidence)

	for _, host := range topLevelSystemSortedSet(group.exactHosts) {
		for _, targetRoot := range topLevelSystemSortedRoots(hostOwners[host]) {
			if targetRoot == groupRoot {
				continue
			}
			if groups[targetRoot].priority >= group.priority {
				continue
			}
			if _, ok := targets[targetRoot]; !ok {
				targets[targetRoot] = topLevelSystemAttachmentEvidence(
					group,
					groups[targetRoot],
					"exact-host-attachment",
					"exact-host",
					host,
				)
			}
		}
	}
	for _, ip := range topLevelSystemSortedSet(group.exactIPs) {
		for _, targetRoot := range topLevelSystemSortedRoots(ipOwners[ip]) {
			if targetRoot == groupRoot {
				continue
			}
			if groups[targetRoot].priority >= group.priority {
				continue
			}
			if _, ok := targets[targetRoot]; !ok {
				targets[targetRoot] = topLevelSystemAttachmentEvidence(
					group,
					groups[targetRoot],
					"exact-ip-attachment",
					"exact-ip",
					ip,
				)
			}
		}
	}

	if len(targets) != 1 {
		return topLevelSystemFallbackTarget{}, false
	}
	for targetRoot, evidence := range targets {
		return topLevelSystemFallbackTarget{
			root:     targetRoot,
			evidence: evidence,
		}, true
	}
	return topLevelSystemFallbackTarget{}, false
}

func candidateExactTargetGroups(
	candidate MonitoredSystemCandidate,
	hostOwners map[string]map[string]struct{},
	ipOwners map[string]map[string]struct{},
	groups map[string]topLevelSystemResolvedGroup,
	priority int,
) map[string]struct{} {
	targets := make(map[string]struct{})
	for host := range monitoredSystemCandidateExactHosts(candidate) {
		for groupID := range hostOwners[host] {
			if groups[groupID].priority >= priority {
				continue
			}
			targets[groupID] = struct{}{}
		}
	}
	for ip := range monitoredSystemCandidateExactIPs(candidate) {
		for groupID := range ipOwners[ip] {
			if groups[groupID].priority >= priority {
				continue
			}
			targets[groupID] = struct{}{}
		}
	}
	return targets
}

func monitoredSystemCandidateStrongIDs(candidate MonitoredSystemCandidate) map[string]struct{} {
	ids := make(map[string]struct{})
	if resourceID := strings.TrimSpace(candidate.ResourceID); resourceID != "" {
		ids["resource:"+resourceID] = struct{}{}
	}
	if machineID := strings.TrimSpace(candidate.MachineID); machineID != "" {
		ids["machine:"+machineID] = struct{}{}
	}
	if candidate.Type != ResourceTypeK8sCluster {
		if agentID := strings.TrimSpace(candidate.AgentID); agentID != "" {
			ids["agent:"+agentID] = struct{}{}
		}
	}
	return ids
}

func monitoredSystemCandidateExactHosts(candidate MonitoredSystemCandidate) map[string]struct{} {
	hosts := make(map[string]struct{})
	for _, value := range []string{candidate.Hostname, extractHostname(candidate.HostURL)} {
		if normalized := topLevelSystemNormalizeHost(value); normalized != "" {
			hosts[normalized] = struct{}{}
		}
	}
	return hosts
}

func monitoredSystemCandidateExactIPs(candidate MonitoredSystemCandidate) map[string]struct{} {
	ips := make(map[string]struct{})
	for _, value := range []string{candidate.Hostname, extractHostname(candidate.HostURL)} {
		if normalized := NormalizeIP(value); normalized != "" && !isNonUniqueIP(normalized) {
			ips[normalized] = struct{}{}
		}
	}
	return ips
}

func monitoredSystemCandidatePriority(candidate MonitoredSystemCandidate) int {
	switch candidate.Type {
	case ResourceTypePBS:
		return 10
	case ResourceTypePMG:
		return 11
	case ResourceTypeK8sCluster:
		return 12
	default:
		return 3
	}
}

func monitoredSystemCandidateAllowsHostAttachment(candidate MonitoredSystemCandidate) bool {
	if candidate.Type == ResourceTypeK8sCluster {
		return false
	}
	return len(monitoredSystemCandidateExactHosts(candidate)) > 0 || len(monitoredSystemCandidateExactIPs(candidate)) > 0
}

func monitoredSystemResourceAllowsHostAttachment(resource *Resource) bool {
	if resource == nil || CanonicalResourceType(resource.Type) == ResourceTypeK8sCluster {
		return false
	}
	return len(topLevelSystemExactHosts(*resource)) > 0 || len(topLevelSystemExactIPs(*resource)) > 0
}

func topLevelSystemGroupID(group topLevelSystemResolvedGroup) string {
	orderedStrongIDs := topLevelSystemOrderedStrongIDs(group.strongIDs)
	if len(orderedStrongIDs) > 0 {
		return orderedStrongIDs[0]
	}

	orderedHosts := topLevelSystemSortedSet(group.exactHosts)
	if len(orderedHosts) > 0 {
		return "host:" + orderedHosts[0]
	}
	orderedIPs := topLevelSystemSortedSet(group.exactIPs)
	if len(orderedIPs) > 0 {
		return "ip:" + orderedIPs[0]
	}

	preferred := preferredMonitoredSystemResource(group.resources)
	if preferred != nil {
		if resourceID := strings.TrimSpace(preferred.ID); resourceID != "" {
			return "resource:" + resourceID
		}
	}
	return "resource:unknown"
}

func topLevelSystemOrderedStrongIDs(values map[string]struct{}) []string {
	ordered := topLevelSystemSortedSet(values)
	sort.SliceStable(ordered, func(i, j int) bool {
		leftPrefix, leftValue := topLevelSystemSplitStrongID(ordered[i])
		rightPrefix, rightValue := topLevelSystemSplitStrongID(ordered[j])
		if topLevelSystemStrongIDPriority(leftPrefix) != topLevelSystemStrongIDPriority(rightPrefix) {
			return topLevelSystemStrongIDPriority(leftPrefix) < topLevelSystemStrongIDPriority(rightPrefix)
		}
		return leftValue < rightValue
	})
	return ordered
}

func topLevelSystemStrongIDPriority(prefix string) int {
	switch prefix {
	case "machine":
		return 0
	case "agent":
		return 1
	case "primary":
		return 2
	case "proxmox":
		return 3
	case "docker":
		return 4
	case "pbs":
		return 5
	case "pmg":
		return 6
	case "k8s":
		return 7
	case "resource":
		return 8
	default:
		return 99
	}
}

func topLevelSystemSplitStrongID(value string) (string, string) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return value, ""
	}
	return parts[0], parts[1]
}

func topLevelSystemGroupingExplanation(
	group topLevelSystemResolvedGroup,
	evidence []topLevelSystemGroupingEvidence,
) MonitoredSystemGroupingExplanation {
	reasons := topLevelSystemGroupingReasons(evidence)
	if len(reasons) == 0 {
		return monitoredSystemStandaloneExplanation(group.resources)
	}

	return normalizeMonitoredSystemGroupingExplanation(MonitoredSystemGroupingExplanation{
		Summary:  topLevelSystemGroupingSummary(group.resources, reasons),
		Reasons:  reasons,
		Surfaces: monitoredSystemGroupingSurfaces(group.resources),
	})
}

func topLevelSystemGroupingReasons(
	evidence []topLevelSystemGroupingEvidence,
) []MonitoredSystemGroupingReason {
	reasons := make([]MonitoredSystemGroupingReason, 0, len(evidence))
	seen := make(map[string]struct{}, len(evidence))
	for _, item := range evidence {
		reason := topLevelSystemReasonFromEvidence(item)
		if reason.Summary == "" {
			continue
		}
		key := reason.Kind + "\x00" + reason.Signal + "\x00" + reason.Summary
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		reasons = append(reasons, reason)
	}
	if reasons == nil {
		return []MonitoredSystemGroupingReason{}
	}
	return reasons
}

func topLevelSystemReasonFromEvidence(
	evidence topLevelSystemGroupingEvidence,
) MonitoredSystemGroupingReason {
	switch evidence.kind {
	case "shared-identity":
		return MonitoredSystemGroupingReason{
			Kind:    evidence.kind,
			Signal:  evidence.signal,
			Summary: topLevelSystemMergedViewSummary(evidence.fromType, evidence.toType, topLevelSystemStrongIDBasis(evidence.signal)),
		}
	case "identity-match":
		return MonitoredSystemGroupingReason{
			Kind:    evidence.kind,
			Signal:  evidence.signal,
			Summary: topLevelSystemMergedViewSummary(evidence.fromType, evidence.toType, topLevelSystemIdentityMatchBasis(evidence.signal)),
		}
	case "exact-host-attachment":
		return MonitoredSystemGroupingReason{
			Kind:    evidence.kind,
			Signal:  evidence.signal,
			Summary: topLevelSystemAttachmentSummary(evidence.fromType, evidence.toType, "a unique exact hostname match", evidence.value),
		}
	case "exact-ip-attachment":
		return MonitoredSystemGroupingReason{
			Kind:    evidence.kind,
			Signal:  evidence.signal,
			Summary: topLevelSystemAttachmentSummary(evidence.fromType, evidence.toType, "a unique exact IP match", evidence.value),
		}
	default:
		return MonitoredSystemGroupingReason{}
	}
}

func topLevelSystemGroupingSummary(
	resources []*Resource,
	reasons []MonitoredSystemGroupingReason,
) string {
	bases := make([]string, 0, len(reasons))
	seen := make(map[string]struct{}, len(reasons))
	for _, reason := range reasons {
		basis := topLevelSystemReasonBasis(reason)
		if basis == "" {
			continue
		}
		if _, ok := seen[basis]; ok {
			continue
		}
		seen[basis] = struct{}{}
		bases = append(bases, basis)
	}
	if len(bases) > 3 {
		bases = append(bases[:2], "other canonical identity signals")
	}
	return fmt.Sprintf(
		"Counts as one monitored system because Pulse merged %d top-level views into one canonical system using %s.",
		topLevelSystemMaxInt(2, len(monitoredSystemGroupingSurfaces(resources))),
		topLevelSystemHumanList(bases),
	)
}

func topLevelSystemReasonBasis(reason MonitoredSystemGroupingReason) string {
	switch reason.Kind {
	case "shared-identity":
		return topLevelSystemStrongIDBasis(reason.Signal)
	case "identity-match":
		return topLevelSystemIdentityMatchBasis(reason.Signal)
	case "exact-host-attachment":
		return "a unique exact hostname attachment"
	case "exact-ip-attachment":
		return "a unique exact IP attachment"
	default:
		return ""
	}
}

func topLevelSystemStrongIDEvidence(
	left *Resource,
	right *Resource,
	strongID string,
) topLevelSystemGroupingEvidence {
	prefix, _ := topLevelSystemSplitStrongID(strongID)
	return topLevelSystemGroupingEvidence{
		kind:     "shared-identity",
		signal:   topLevelSystemStrongIDSignal(prefix),
		value:    strongID,
		fromType: monitoredSystemType(left),
		toType:   monitoredSystemType(right),
	}
}

func topLevelSystemIdentityMatchEvidence(
	left *Resource,
	right *Resource,
	reason string,
) topLevelSystemGroupingEvidence {
	return topLevelSystemGroupingEvidence{
		kind:     "identity-match",
		signal:   strings.TrimSpace(reason),
		value:    strings.TrimSpace(reason),
		fromType: monitoredSystemType(left),
		toType:   monitoredSystemType(right),
	}
}

func topLevelSystemAttachmentEvidence(
	fromGroup topLevelSystemResolvedGroup,
	toGroup topLevelSystemResolvedGroup,
	kind string,
	signal string,
	value string,
) topLevelSystemGroupingEvidence {
	return topLevelSystemGroupingEvidence{
		kind:     kind,
		signal:   signal,
		value:    strings.TrimSpace(value),
		fromType: topLevelSystemPreferredGroupType(fromGroup),
		toType:   topLevelSystemPreferredGroupType(toGroup),
	}
}

func topLevelSystemPreferredGroupType(group topLevelSystemResolvedGroup) string {
	return monitoredSystemType(preferredMonitoredSystemResource(group.resources))
}

func topLevelSystemStrongIDSignal(prefix string) string {
	switch prefix {
	case "machine":
		return "machine-id"
	case "agent":
		return "agent-id"
	case "primary":
		return "canonical-primary-id"
	case "proxmox":
		return "proxmox-source-id"
	case "docker":
		return "docker-host-id"
	case "pbs":
		return "pbs-instance-id"
	case "pmg":
		return "pmg-instance-id"
	case "k8s":
		return "kubernetes-cluster-id"
	case "resource":
		return "resource-id"
	default:
		return strings.TrimSpace(prefix)
	}
}

func topLevelSystemStrongIDBasis(signal string) string {
	switch signal {
	case "machine-id":
		return "shared machine identity"
	case "agent-id":
		return "shared agent identity"
	case "canonical-primary-id":
		return "shared canonical primary identity"
	case "proxmox-source-id":
		return "shared Proxmox source identity"
	case "docker-host-id":
		return "shared Docker host identity"
	case "pbs-instance-id":
		return "shared PBS instance identity"
	case "pmg-instance-id":
		return "shared PMG instance identity"
	case "kubernetes-cluster-id":
		return "shared Kubernetes cluster identity"
	case "resource-id":
		return "shared canonical resource identity"
	default:
		return "shared canonical identity"
	}
}

func topLevelSystemIdentityMatchBasis(signal string) string {
	switch signal {
	case "dmi_uuid":
		return "high-confidence DMI identity"
	case "hostname+mac":
		return "shared hostname and MAC identity"
	default:
		return "high-confidence canonical identity"
	}
}

func topLevelSystemMergedViewSummary(leftType, rightType, basis string) string {
	leftLabel := monitoredSystemGroupingTypeLabel(leftType)
	rightLabel := monitoredSystemGroupingTypeLabel(rightType)
	if leftLabel == "" || rightLabel == "" || leftLabel == rightLabel {
		return "Merged top-level views using " + basis + "."
	}
	labels := []string{leftLabel, rightLabel}
	sort.Slice(labels, func(i, j int) bool {
		return strings.ToLower(labels[i]) < strings.ToLower(labels[j])
	})
	return fmt.Sprintf("Merged %s and %s views using %s.", labels[0], labels[1], basis)
}

func topLevelSystemAttachmentSummary(fromType, toType, basis, value string) string {
	fromLabel := monitoredSystemGroupingTypeLabel(fromType)
	if fromLabel == "" {
		fromLabel = "top-level view"
	}
	toLabel := monitoredSystemGroupingTypeLabel(toType)
	if toLabel == "" {
		toLabel = "canonical system"
	}
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return fmt.Sprintf("Attached %s to %s using %s on %s.", fromLabel, toLabel, basis, trimmed)
	}
	return fmt.Sprintf("Attached %s to %s using %s.", fromLabel, toLabel, basis)
}

func topLevelSystemHumanList(values []string) string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	switch len(filtered) {
	case 0:
		return "canonical identity signals"
	case 1:
		return filtered[0]
	case 2:
		return filtered[0] + " and " + filtered[1]
	default:
		return strings.Join(filtered[:len(filtered)-1], ", ") + ", and " + filtered[len(filtered)-1]
	}
}

func topLevelSystemSortedRoots(values map[int]struct{}) []int {
	out := make([]int, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func topLevelSystemMaxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func topLevelSystemSortedSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func topLevelSystemSetsOverlap(left, right map[string]struct{}) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	if len(left) > len(right) {
		left, right = right, left
	}
	for value := range left {
		if _, ok := right[value]; ok {
			return true
		}
	}
	return false
}

func addTopLevelSystemOwner(index map[string]map[string]struct{}, key, owner string) {
	bucket := index[key]
	if bucket == nil {
		bucket = make(map[string]struct{})
		index[key] = bucket
	}
	bucket[owner] = struct{}{}
}

func topLevelSystemNormalizeHost(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}
	if NormalizeIP(trimmed) != "" {
		return ""
	}
	return trimmed
}

func topLevelSystemProxmoxHostURL(resource Resource) string {
	if resource.Proxmox == nil {
		return ""
	}
	return strings.TrimSpace(resource.Proxmox.HostURL)
}

func topLevelSystemPBSHostURL(resource Resource) string {
	if resource.PBS == nil {
		return ""
	}
	return strings.TrimSpace(resource.PBS.HostURL)
}

func topLevelSystemKubernetesServer(resource Resource) string {
	if resource.Kubernetes == nil {
		return ""
	}
	return strings.TrimSpace(resource.Kubernetes.Server)
}

func topLevelSystemCanonicalHostname(resource Resource) string {
	if resource.Canonical == nil {
		return ""
	}
	return strings.TrimSpace(resource.Canonical.Hostname)
}
