package agentcapabilities

import (
	"slices"
	"testing"
)

func TestPatrolScopedToolProjectionUsesTypedLeastManifest(t *testing.T) {
	dockerEvidence, ok := PatrolEvidenceToolNamesForResourceTypes([]string{"app-container"})
	if !ok {
		t.Fatal("app-container scope was not recognized")
	}
	if !slices.Equal(dockerEvidence, []string{PulseQueryToolName, PulseReadToolName, PulseDockerToolName}) {
		t.Fatalf("Docker evidence tools = %v", dockerEvidence)
	}
	for _, forbidden := range []string{PulseStorageToolName, PulseKubernetesToolName, PulsePMGToolName, PulseAlertsToolName, PulseKnowledgeToolName, PulseSummarizeToolName} {
		if slices.Contains(dockerEvidence, forbidden) {
			t.Fatalf("Docker evidence surface included unrelated tool %q: %v", forbidden, dockerEvidence)
		}
	}

	storageInvestigation, ok := PatrolInvestigationToolNamesForResourceTypes([]string{"physical-disk"})
	if !ok {
		t.Fatal("physical-disk scope was not recognized")
	}
	for _, required := range []string{PulseQueryToolName, PulseReadToolName, PulseMetricsToolName, PulseStorageToolName, PatrolActionCapabilitiesToolName, PatrolProposeActionToolName} {
		if !slices.Contains(storageInvestigation, required) {
			t.Fatalf("storage investigation tools missing %q: %v", required, storageInvestigation)
		}
	}
}

func TestPatrolScopedToolProjectionFailsOpenToGovernedProfileForUnknownType(t *testing.T) {
	if names, ok := PatrolEvidenceToolNamesForResourceTypes(nil); ok || names != nil {
		t.Fatalf("empty scope = (%v, %v), want no projection", names, ok)
	}
	if names, ok := PatrolEvidenceToolNamesForResourceTypes([]string{"future-resource-kind"}); ok || names != nil {
		t.Fatalf("unknown scope = (%v, %v), want no projection", names, ok)
	}
	if names, ok := PatrolEvidenceToolNamesForResourceTypes([]string{"app-container", "future-resource-kind"}); ok || names != nil {
		t.Fatalf("mixed unknown scope = (%v, %v), want no partial projection", names, ok)
	}
}

func TestPatrolScopedToolProjectionComposesProfileOwnedTools(t *testing.T) {
	detection, ok := PatrolDetectionToolNamesForResourceTypes([]string{"network-endpoint"})
	if !ok {
		t.Fatal("network-endpoint scope was not recognized")
	}
	for _, required := range []string{PulseQueryToolName, PulseDiscoveryToolName, PatrolGetFindingsToolName, PatrolReportFindingToolName, PatrolAssessFindingToolName} {
		if !slices.Contains(detection, required) {
			t.Fatalf("detection surface missing %q: %v", required, detection)
		}
	}
	if slices.Contains(detection, PatrolResolveFindingToolName) {
		t.Fatalf("detection surface exposed the legacy direct resolver: %v", detection)
	}
	if slices.Contains(detection, PulseReadToolName) {
		t.Fatalf("detection surface included optional deep-read authority: %v", detection)
	}
	if slices.Contains(detection, PatrolProposeActionToolName) {
		t.Fatalf("detection surface gained investigation proposal authority: %v", detection)
	}
}
