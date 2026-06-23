package tools

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

func TestCanonicalToolGovernanceMirrorsRegisteredRegistry(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})
	registryGovernance := exec.registry.ListToolGovernance(ControlLevelControlled)
	canonical := CanonicalToolGovernance(ControlLevelControlled)

	if len(canonical) != len(registryGovernance) {
		t.Fatalf("canonical governance length = %d, want %d", len(canonical), len(registryGovernance))
	}
	for i := range registryGovernance {
		if canonical[i] != registryGovernance[i] {
			t.Fatalf("canonical governance[%d] = %#v, want registry descriptor %#v", i, canonical[i], registryGovernance[i])
		}
	}
}

func TestCanonicalToolGovernanceForAssistantSurfaceMirrorsRegisteredRegistry(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})
	registryGovernance := exec.registry.ListToolGovernance(ControlLevelControlled)
	canonical := CanonicalToolGovernanceForSurface(ControlLevelControlled, ToolGovernanceSurfacePulseAssistant)

	filtered := make([]ToolGovernanceDescriptor, 0, len(registryGovernance))
	for _, tool := range registryGovernance {
		switch tool.Name {
		case agentcapabilities.PatrolReportFindingToolName, agentcapabilities.PatrolResolveFindingToolName, agentcapabilities.PatrolGetFindingsToolName:
			continue
		default:
			filtered = append(filtered, tool)
		}
	}

	if len(canonical) != len(filtered) {
		t.Fatalf("canonical assistant governance length = %d, want %d", len(canonical), len(filtered))
	}
	for i := range filtered {
		if canonical[i] != filtered[i] {
			t.Fatalf("canonical assistant governance[%d] = %#v, want registry descriptor %#v", i, canonical[i], filtered[i])
		}
	}
}

func TestCanonicalToolGovernanceForManifestSurfaceUsesSurfaceAffordances(t *testing.T) {
	manifest := agentcapabilities.CanonicalManifest()
	assistant := CanonicalToolGovernanceForManifestSurface(
		ControlLevelControlled,
		manifest,
		agentcapabilities.SurfaceIDPulseAssistant,
	)
	canonicalAssistant := CanonicalToolGovernanceForSurface(ControlLevelControlled, ToolGovernanceSurfacePulseAssistant)
	if len(assistant) != len(canonicalAssistant) {
		t.Fatalf("manifest assistant governance length = %d, want %d", len(assistant), len(canonicalAssistant))
	}
	for i := range canonicalAssistant {
		if assistant[i] != canonicalAssistant[i] {
			t.Fatalf("manifest assistant governance[%d] = %#v, want %#v", i, assistant[i], canonicalAssistant[i])
		}
	}

	toolsDisabled := manifest
	for i := range toolsDisabled.SurfaceContract.OperatorSurfaces {
		if toolsDisabled.SurfaceContract.OperatorSurfaces[i].ID == agentcapabilities.SurfaceIDPulseAssistant {
			toolsDisabled.SurfaceContract.OperatorSurfaces[i].Affordances = agentcapabilities.SurfaceAffordanceContract{
				InteractiveQuestions: true,
			}
		}
	}
	if disabled := CanonicalToolGovernanceForManifestSurface(ControlLevelControlled, toolsDisabled, agentcapabilities.SurfaceIDPulseAssistant); len(disabled) != 0 {
		t.Fatalf("tools-disabled manifest assistant governance = %+v, want none", disabled)
	}
}

func TestCanonicalToolGovernanceForSurfaceFiltersAssistantOnlyRuntime(t *testing.T) {
	assistant := CanonicalToolGovernanceForSurface(ControlLevelControlled, ToolGovernanceSurfacePulseAssistant)
	patrol := CanonicalToolGovernanceForSurface(ControlLevelControlled, ToolGovernanceSurfacePulsePatrol)
	core := CanonicalToolGovernanceForSurface(ControlLevelControlled, ToolGovernanceSurfacePulseIntelligence)

	if len(patrol) != len(core) {
		t.Fatalf("patrol fallback governance length = %d, want full core registry length %d", len(patrol), len(core))
	}

	foundPatrolToolInCore := false
	for _, tool := range core {
		switch tool.Name {
		case agentcapabilities.PatrolReportFindingToolName, agentcapabilities.PatrolResolveFindingToolName, agentcapabilities.PatrolGetFindingsToolName:
			foundPatrolToolInCore = true
		}
	}
	if !foundPatrolToolInCore {
		t.Fatal("expected core governance to include Patrol runtime tools")
	}

	for _, tool := range assistant {
		switch tool.Name {
		case agentcapabilities.PatrolReportFindingToolName, agentcapabilities.PatrolResolveFindingToolName, agentcapabilities.PatrolGetFindingsToolName:
			t.Fatalf("Assistant fallback governance exposed Patrol runtime tool %s", tool.Name)
		}
	}
}

func TestCanonicalToolGovernanceForAssistantSurfaceHonorsControlLevel(t *testing.T) {
	governance := CanonicalToolGovernanceForSurface(ControlLevelReadOnly, ToolGovernanceSurfacePulseAssistant)
	for _, tool := range governance {
		if tool.RequireControl {
			t.Fatalf("read-only assistant governance exposed control tool %#v", tool)
		}
		if tool.Name == agentcapabilities.PulseControlToolName || tool.Name == agentcapabilities.PulseFileEditToolName {
			t.Fatalf("read-only assistant governance exposed %s", tool.Name)
		}
	}
}
