package agentcapabilities

import "strings"

// PatrolEvidenceToolNamesForResourceTypes returns the smallest useful
// read-only evidence surface for an exact, core-resolved Patrol resource
// scope. The bool is false when the scope is empty or contains an unknown
// resource type; callers must then retain their normal governed profile
// rather than guessing from prompt text or accidentally hiding a capability.
//
// pulse_query and pulse_read are the cross-resource escape hatches: a focused
// investigation can still follow canonical topology and inspect a causal
// dependency without receiving every subsystem-specific schema up front.
func PatrolEvidenceToolNamesForResourceTypes(resourceTypes []string) ([]string, bool) {
	if len(resourceTypes) == 0 {
		return nil, false
	}

	names := []string{PulseQueryToolName, PulseReadToolName}
	seen := map[string]bool{
		PulseQueryToolName: true,
		PulseReadToolName:  true,
	}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	known := 0
	for _, raw := range resourceTypes {
		resourceType := strings.ToLower(strings.TrimSpace(raw))
		if resourceType == "" {
			continue
		}
		known++
		switch resourceType {
		case "app-container", "app_container", "docker-container", "docker_container", "docker-host", "docker":
			add(PulseDockerToolName)
		case "storage", "storage-pool", "physical-disk", "physical_disk", "pbs", "backup":
			add(PulseMetricsToolName)
			add(PulseStorageToolName)
		case "k8s-cluster", "k8s-node", "k8s-pod", "k8s-deployment", "kubernetes", "kubernetes-cluster":
			add(PulseKubernetesToolName)
		case "pmg":
			add(PulsePMGToolName)
		case "network-endpoint", "endpoint", "discovery":
			add(PulseDiscoveryToolName)
		case "agent", "host", "truenas", "node", "vm", "system-container", "container", "service":
			add(PulseMetricsToolName)
		default:
			return nil, false
		}
	}
	if known == 0 {
		return nil, false
	}
	return names, true
}

// PatrolDetectionToolNamesForResourceTypes composes the scoped evidence
// surface with the finding lifecycle owned by the detection profile. It may
// only reduce the already-governed profile manifest.
func PatrolDetectionToolNamesForResourceTypes(resourceTypes []string) ([]string, bool) {
	names, ok := PatrolEvidenceToolNamesForResourceTypes(resourceTypes)
	if !ok {
		return nil, false
	}
	return append(names,
		PatrolGetFindingsToolName,
		PatrolReportFindingToolName,
		PatrolAssessFindingToolName,
		PatrolResolveFindingToolName,
	), true
}

// PatrolInvestigationToolNamesForResourceTypes composes scoped evidence with
// the side-effect-free capability lookup and typed proposal sink. Planning,
// approval, execution, and verification remain outside this surface.
func PatrolInvestigationToolNamesForResourceTypes(resourceTypes []string) ([]string, bool) {
	names, ok := PatrolEvidenceToolNamesForResourceTypes(resourceTypes)
	if !ok {
		return nil, false
	}
	return append(names,
		PatrolActionCapabilitiesToolName,
		PatrolProposeActionToolName,
	), true
}
