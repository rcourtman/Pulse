package unifiedresources

import (
	"sort"
	"strings"
)

// Pre-repair matching and grouping are independent content oracles. Broad
// matching must keep every confidence/review result; grouping may only avoid
// candidates its existing threshold/priority rules already reject.
func (m *IdentityMatcher) findCandidatesReference(identity ResourceIdentity) []MatchCandidate {
	candidates := make(map[string]MatchCandidate)

	// Machine ID match
	if identity.MachineID != "" {
		if existing, ok := m.byMachineID[strings.TrimSpace(identity.MachineID)]; ok {
			candidates[existing] = MatchCandidate{ID: existing, Confidence: 1.0, Reason: "machine_id"}
		}
	}

	// DMI UUID match
	if identity.DMIUUID != "" {
		if existing, ok := m.byDMIUUID[strings.TrimSpace(identity.DMIUUID)]; ok {
			candidates[existing] = promoteCandidate(candidates[existing], MatchCandidate{ID: existing, Confidence: 0.99, Reason: "dmi_uuid"})
		}
	}

	hostnameIDs := m.collectIDs(m.byHostname, identity.Hostnames, NormalizeHostname)
	ipIDs := m.collectIDs(m.byIP, identity.IPAddresses, NormalizeIP)
	macIDs := m.collectIDs(m.byMAC, identity.MACAddresses, NormalizeMAC)

	// Hostname + MAC overlap
	for id := range intersectIDs(hostnameIDs, macIDs) {
		candidates[id] = promoteCandidate(candidates[id], MatchCandidate{ID: id, Confidence: 0.90, Reason: "hostname+mac"})
	}

	// Hostname + IP overlap
	for id := range intersectIDs(hostnameIDs, ipIDs) {
		candidates[id] = promoteCandidate(candidates[id], MatchCandidate{
			ID:             id,
			Confidence:     0.80,
			Reason:         "hostname+ip",
			RequiresReview: true,
		})
	}

	// Hostname only
	for id := range hostnameIDs {
		candidates[id] = promoteCandidate(candidates[id], MatchCandidate{ID: id, Confidence: 0.50, Reason: "hostname", RequiresReview: true})
	}

	// IP only
	for id := range ipIDs {
		candidates[id] = promoteCandidate(candidates[id], MatchCandidate{ID: id, Confidence: 0.40, Reason: "ip", RequiresReview: true})
	}

	list := make([]MatchCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		list = append(list, candidate)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Confidence == list[j].Confidence {
			return list[i].ID < list[j].ID
		}
		return list[i].Confidence > list[j].Confidence
	})

	return list
}

func resolveTopLevelSystemsReference(resources []Resource) TopLevelSystemResolver {
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
		matches := matcher.findCandidatesReference(nodes[i].identity)
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
		hostFormOwners := buildTopLevelSystemHostFormOwners(initialGroups)
		attached := false

		for groupRoot, group := range initialGroups {
			if !group.attachByHost {
				continue
			}
			target, ok := uniqueBetterTopLevelSystemTargetReference(groupRoot, group, hostOwners, ipOwners, hostFormOwners, initialGroups)
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

func uniqueBetterTopLevelSystemTargetReference(
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
			candidateRoots[root] = struct{}{}
		}
		if short := NormalizeHostname(comparable); short != "" && short != comparable {
			for root := range hostFormOwners[short] {
				candidateRoots[root] = struct{}{}
			}
		}
	}

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
