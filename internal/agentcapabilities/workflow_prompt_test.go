package agentcapabilities

import (
	"strings"
	"testing"
)

func TestProjectPulseWorkflowPromptsProjectsManifestCapabilities(t *testing.T) {
	got := ProjectPulseWorkflowPrompts([]Capability{
		{Name: OperationsLoopStatusCapabilityName},
		{Name: FleetContextCapabilityName},
		{Name: ResourceContextCapabilityName},
		{Name: ListFindingsCapabilityName},
		{Name: PlanActionCapabilityName},
		{Name: DecideActionCapabilityName},
		{Name: ExecuteActionCapabilityName},
		{Name: ResolveFindingCapabilityName},
	})
	if len(got) != 4 {
		t.Fatalf("workflow prompts = %+v, want four manifest-backed prompts", got)
	}

	names := map[string]PulseWorkflowPrompt{}
	for _, prompt := range got {
		names[prompt.Name] = prompt
	}
	if _, ok := names[PulseWorkflowPromptTriageFleet]; !ok {
		t.Fatalf("workflow prompts missing %s: %+v", PulseWorkflowPromptTriageFleet, got)
	}
	if names[PulseWorkflowPromptTriageFleet].Label != "Triage fleet" {
		t.Fatalf("fleet workflow prompt label = %q", names[PulseWorkflowPromptTriageFleet].Label)
	}
	if names[PulseWorkflowPromptTriageFleet].PresentationKind != PulseWorkflowPromptPresentationFleet {
		t.Fatalf("fleet workflow prompt presentation kind = %q", names[PulseWorkflowPromptTriageFleet].PresentationKind)
	}
	resourcePrompt := names[PulseWorkflowPromptInvestigateResource]
	if resourcePrompt.Label != "Investigate resource" {
		t.Fatalf("resource workflow prompt label = %q", resourcePrompt.Label)
	}
	if resourcePrompt.PresentationKind != PulseWorkflowPromptPresentationResource {
		t.Fatalf("resource workflow prompt presentation kind = %q", resourcePrompt.PresentationKind)
	}
	if len(resourcePrompt.Arguments) != 1 || resourcePrompt.Arguments[0].Name != ResourceIDArgumentName || !resourcePrompt.Arguments[0].Required {
		t.Fatalf("resource prompt arguments = %+v", resourcePrompt.Arguments)
	}
	findingPrompt := names[PulseWorkflowPromptReviewFinding]
	if findingPrompt.Label != "Review finding" {
		t.Fatalf("finding workflow prompt label = %q", findingPrompt.Label)
	}
	if findingPrompt.PresentationKind != PulseWorkflowPromptPresentationFinding {
		t.Fatalf("finding workflow prompt presentation kind = %q", findingPrompt.PresentationKind)
	}
	if len(findingPrompt.Arguments) != 1 || findingPrompt.Arguments[0].Name != FindingIDArgumentName || !findingPrompt.Arguments[0].Required {
		t.Fatalf("finding prompt arguments = %+v", findingPrompt.Arguments)
	}
	loopPrompt := names[PulseWorkflowPromptOperationsLoop]
	if loopPrompt.Label != "Ask Patrol to handle an issue" {
		t.Fatalf("operations-loop workflow prompt label = %q", loopPrompt.Label)
	}
	if loopPrompt.PresentationKind != PulseWorkflowPromptPresentationWorkflow {
		t.Fatalf("operations-loop workflow prompt presentation kind = %q", loopPrompt.PresentationKind)
	}
	if len(loopPrompt.Arguments) != 0 {
		t.Fatalf("operations-loop prompt arguments = %+v, want none", loopPrompt.Arguments)
	}
}

func TestProjectPulseWorkflowPromptsRequiresFullLoopCapabilitiesForOperationsLoop(t *testing.T) {
	partial := ProjectPulseWorkflowPrompts([]Capability{
		{Name: OperationsLoopStatusCapabilityName},
		{Name: FleetContextCapabilityName},
		{Name: ResourceContextCapabilityName},
		{Name: ListFindingsCapabilityName},
		{Name: PlanActionCapabilityName},
		{Name: DecideActionCapabilityName},
		{Name: ExecuteActionCapabilityName},
	})
	for _, prompt := range partial {
		if prompt.Name == PulseWorkflowPromptOperationsLoop {
			t.Fatalf("operations-loop prompt must not appear without resolve_finding: %+v", partial)
		}
	}
}

func TestBuildPulseWorkflowPromptRendersSharedWorkflowText(t *testing.T) {
	got, err := BuildPulseWorkflowPrompt([]Capability{{Name: FleetContextCapabilityName}}, PulseWorkflowPromptParams{Name: PulseWorkflowPromptTriageFleet})
	if err != nil {
		t.Fatalf("BuildPulseWorkflowPrompt: %v", err)
	}
	if got.Description != "Pulse fleet triage" || !strings.Contains(got.Text, "get_fleet_context") || !strings.Contains(got.Text, "safest next governed step") {
		t.Fatalf("fleet workflow prompt = %+v", got)
	}

	resource, err := BuildPulseWorkflowPrompt([]Capability{{Name: ResourceContextCapabilityName}}, PulseWorkflowPromptParams{
		Name:      PulseWorkflowPromptInvestigateResource,
		Arguments: map[string]string{ResourceIDArgumentName: " vm:101 "},
	})
	if err != nil {
		t.Fatalf("BuildPulseWorkflowPrompt resource: %v", err)
	}
	for _, want := range []string{`"vm:101"`, ResourceContextCapabilityName, ResourceIDArgumentName, "Do not execute write tools"} {
		if !strings.Contains(resource.Text, want) {
			t.Fatalf("resource workflow prompt missing %q: %q", want, resource.Text)
		}
	}

	loop, err := BuildPulseWorkflowPrompt(
		[]Capability{
			{Name: OperationsLoopStatusCapabilityName},
			{Name: FleetContextCapabilityName},
			{Name: ResourceContextCapabilityName},
			{Name: ListFindingsCapabilityName},
			{Name: PlanActionCapabilityName},
			{Name: DecideActionCapabilityName},
			{Name: ExecuteActionCapabilityName},
			{Name: ResolveFindingCapabilityName},
		},
		PulseWorkflowPromptParams{Name: PulseWorkflowPromptOperationsLoop},
	)
	if err != nil {
		t.Fatalf("BuildPulseWorkflowPrompt operations loop: %v", err)
	}
	for _, want := range []string{
		"Patrol issue handling",
		OperationsLoopStatusCapabilityName,
		FleetContextCapabilityName,
		ListFindingsCapabilityName,
		ResourceContextCapabilityName,
		PlanActionCapabilityName,
		"ask the operator only when policy requires approval",
		DecideActionCapabilityName,
		ExecuteActionCapabilityName,
		ResolveFindingCapabilityName,
		"without executing",
	} {
		if !strings.Contains(loop.Text, want) && !strings.Contains(loop.Description, want) {
			t.Fatalf("operations-loop workflow prompt missing %q: description=%q text=%q", want, loop.Description, loop.Text)
		}
	}
}

func TestBuildPulseWorkflowPromptFromManifestHonorsWorkflowPrompts(t *testing.T) {
	got, err := BuildPulseWorkflowPromptFromManifest(Manifest{
		WorkflowPrompts: []PulseWorkflowPrompt{{
			Name: PulseWorkflowPromptTriageFleet,
		}},
	}, PulseWorkflowPromptParams{Name: PulseWorkflowPromptTriageFleet})
	if err != nil {
		t.Fatalf("BuildPulseWorkflowPromptFromManifest declared prompt: %v", err)
	}
	if got.Description != "Pulse fleet triage" || !strings.Contains(got.Text, "get_fleet_context") {
		t.Fatalf("manifest-declared prompt = %+v", got)
	}

	_, err = BuildPulseWorkflowPromptFromManifest(Manifest{
		Capabilities:    []Capability{{Name: FleetContextCapabilityName}},
		WorkflowPrompts: []PulseWorkflowPrompt{},
	}, PulseWorkflowPromptParams{Name: PulseWorkflowPromptTriageFleet})
	if err == nil || err.Error() != "unknown prompt: pulse_triage_fleet" {
		t.Fatalf("explicit empty workflowPrompts must suppress capability fallback; err=%v", err)
	}
}

func TestBuildPulseWorkflowPromptSupportsSurfaceResourceInstructions(t *testing.T) {
	got, err := BuildPulseWorkflowPromptWithOptions([]Capability{{Name: ResourceContextCapabilityName}}, PulseWorkflowPromptParams{
		Name:      PulseWorkflowPromptInvestigateResource,
		Arguments: map[string]string{ResourceIDArgumentName: "vm:101"},
	}, PulseWorkflowPromptRenderOptions{
		ResourceContextInstruction: func(resourceID string) string {
			return "Use resources/read for " + resourceID
		},
	})
	if err != nil {
		t.Fatalf("BuildPulseWorkflowPromptWithOptions: %v", err)
	}
	if !strings.Contains(got.Text, "Use resources/read for vm:101") {
		t.Fatalf("surface resource instruction missing from prompt: %q", got.Text)
	}
}

func TestBuildPulseWorkflowPromptValidatesAvailabilityAndArguments(t *testing.T) {
	_, err := BuildPulseWorkflowPrompt([]Capability{{Name: ResourceContextCapabilityName}}, PulseWorkflowPromptParams{Name: PulseWorkflowPromptInvestigateResource})
	if err == nil || err.Error() != "prompt argument "+ResourceIDArgumentName+" is required" {
		t.Fatalf("missing resourceId error = %v", err)
	}

	_, err = BuildPulseWorkflowPrompt(nil, PulseWorkflowPromptParams{Name: PulseWorkflowPromptTriageFleet})
	if err == nil || err.Error() != "unknown prompt: pulse_triage_fleet" {
		t.Fatalf("unknown prompt error = %v", err)
	}

	finding, err := BuildPulseWorkflowPrompt([]Capability{{Name: ListFindingsCapabilityName}}, PulseWorkflowPromptParams{
		Name:      PulseWorkflowPromptReviewFinding,
		Arguments: map[string]string{FindingIDArgumentName: " finding-1 "},
	})
	if err != nil {
		t.Fatalf("BuildPulseWorkflowPrompt finding: %v", err)
	}
	for _, want := range []string{`"finding-1"`, ListFindingsCapabilityName, "name the evidence"} {
		if !strings.Contains(finding.Text, want) {
			t.Fatalf("finding workflow prompt missing %q: %q", want, finding.Text)
		}
	}
}

func TestPulseWorkflowPromptParamsNormalizeDetachesArguments(t *testing.T) {
	source := map[string]string{ResourceIDArgumentName: "vm:101"}
	params := PulseWorkflowPromptParams{
		Name:      " " + PulseWorkflowPromptInvestigateResource + " ",
		Arguments: source,
	}.NormalizeCollections()
	if params.Name != PulseWorkflowPromptInvestigateResource || params.Arguments[ResourceIDArgumentName] != "vm:101" {
		t.Fatalf("normalized prompt params = %+v", params)
	}
	params.Arguments[ResourceIDArgumentName] = "vm:102"
	if source[ResourceIDArgumentName] != "vm:101" {
		t.Fatalf("normalized prompt params aliased source: source=%+v params=%+v", source, params.Arguments)
	}
}
