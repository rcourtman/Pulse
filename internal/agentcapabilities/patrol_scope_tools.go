package agentcapabilities

import "strings"

// IsPatrolInfrastructureEvidenceToolName reports whether a provider tool can
// return fresh canonical or provider-observed infrastructure evidence for a
// Patrol investigation. Planning, synthesis, retained knowledge, and finding
// lifecycle reads are deliberately outside this closed vocabulary: they may
// help explain evidence, but cannot satisfy the grounding gate or unlock a
// typed remediation proposal by themselves.
func IsPatrolInfrastructureEvidenceToolName(name string) bool {
	switch strings.TrimSpace(name) {
	case PulseQueryToolName,
		PulseDiscoveryToolName,
		PulseMetricsToolName,
		PulseStorageToolName,
		PulseDockerToolName,
		PulseKubernetesToolName,
		PulseReadToolName,
		PulsePMGToolName:
		return true
	default:
		return false
	}
}

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
	return patrolEvidenceToolNamesForResourceTypes(resourceTypes, true)
}

func patrolEvidenceToolNamesForResourceTypes(resourceTypes []string, includeDeepRead bool) ([]string, bool) {
	if len(resourceTypes) == 0 {
		return nil, false
	}

	names := []string{PulseQueryToolName}
	seen := map[string]bool{
		PulseQueryToolName: true,
	}
	if includeDeepRead {
		names = append(names, PulseReadToolName)
		seen[PulseReadToolName] = true
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
	// Watch owns symptom detection from the canonical seed and provider reads.
	// Agent-routed file/log/exec inspection belongs to the separate finding
	// investigation, where runtime availability can be evaluated explicitly.
	names, ok := patrolEvidenceToolNamesForResourceTypes(resourceTypes, false)
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
