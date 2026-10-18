package unifiedresources

import (
	"sort"
	"strings"
)

// Exact production grouping before pass-local owner indexes at
// 70c71ad5b8baa519a6ae5ab255a78d9d5187300e. Keep the immediate-parent
// algorithm for cost comparisons, not the older broad matcher reference.

func resolveTopLevelSystemsBeforeOwnerIndex(resources []Resource) TopLevelSystemResolver {
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
		matches := matcher.findCandidatesAtLeast(nodes[i].identity, HighConfidenceThreshold)
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
		bestPriority := int(^uint(0) >> 1)
		for _, group := range initialGroups {
			if group.priority < bestPriority {
				bestPriority = group.priority
			}
		}
		hostOwners, ipOwners := buildTopLevelSystemFallbackOwners(initialGroups)
		hostFormOwners := buildTopLevelSystemHostFormOwners(initialGroups)
		attached := false

		for groupRoot, group := range initialGroups {
			if !group.attachByHost || group.priority <= bestPriority {
				continue
			}
			target, ok := uniqueBetterTopLevelSystemTargetBeforeOwnerIndex(groupRoot, group, hostOwners, ipOwners, hostFormOwners, initialGroups)
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

func buildTopLevelSystemHostFormOwners(
	groups map[int]topLevelSystemResolvedGroup,
) map[string]map[int]struct{} {
	owners := make(map[string]map[int]struct{})
	add := func(key string, root int) {
		if key == "" {
			return
		}
		bucket := owners[key]
		if bucket == nil {
			bucket = make(map[int]struct{})
			owners[key] = bucket
		}
		bucket[root] = struct{}{}
	}
	for groupRoot, group := range groups {
		for host := range group.exactHosts {
			comparable := normalizeComparableHostname(host)
			if comparable == "" {
				continue
			}
			add(comparable, groupRoot)
			if short := NormalizeHostname(comparable); short != "" && short != comparable {
				add(short, groupRoot)
			}
		}
	}
	return owners
}

func uniqueBetterTopLevelSystemTargetBeforeOwnerIndex(
	groupRoot int,
	group topLevelSystemResolvedGroup,
	hostOwners map[string]map[int]struct{},
	ipOwners map[string]map[int]struct{},
	hostFormOwners map[string]map[int]struct{},
	groups map[int]topLevelSystemResolvedGroup,
) (topLevelSystemFallbackTarget, bool) {
	targets := make(map[int]topLevelSystemGroupingEvidence)
	candidateRoots := make(map[int]struct{})
	for host := range group.exactHosts {
		comparable := normalizeComparableHostname(host)
		if comparable == "" {
			continue
		}
		for root := range hostFormOwners[comparable] {
			if root != groupRoot && groups[root].priority < group.priority {
				candidateRoots[root] = struct{}{}
			}
		}
		if short := NormalizeHostname(comparable); short != "" && short != comparable {
			for root := range hostFormOwners[short] {
				if root != groupRoot && groups[root].priority < group.priority {
					candidateRoots[root] = struct{}{}
				}
			}
		}
	}

	for _, host := range topLevelSystemSortedSet(group.exactHosts) {
		for _, targetRoot := range topLevelSystemBetterRoots(hostOwners[host], groups, group.priority) {
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
	for _, targetRoot := range topLevelSystemSortedRoots(candidateRoots) {
		if targetRoot == groupRoot {
			continue
		}
		if groups[targetRoot].priority >= group.priority {
			continue
		}
		if _, ok := targets[targetRoot]; ok {
			continue
		}
		if value, ok := topLevelSystemShortFormHostMatchValue(group.exactHosts, groups[targetRoot].exactHosts); ok {
			targets[targetRoot] = topLevelSystemAttachmentEvidence(
				group,
				groups[targetRoot],
				"hostname-form-attachment",
				"short-hostname",
				value,
			)
		}
	}
	for _, ip := range topLevelSystemSortedSet(group.exactIPs) {
		for _, targetRoot := range topLevelSystemBetterRoots(ipOwners[ip], groups, group.priority) {
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

func topLevelSystemSortedRoots(values map[int]struct{}) []int {
	out := make([]int, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func topLevelSystemBetterRoots(values map[int]struct{}, groups map[int]topLevelSystemResolvedGroup, priority int) []int {
	var out []int
	for root := range values {
		if groups[root].priority < priority {
			out = append(out, root)
		}
	}
	sort.Ints(out)
	return out
}
