package agentcapabilities

import (
	"strings"
	"testing"
)

func TestBuildToolGovernancePromptSectionProjectsSharedManifest(t *testing.T) {
	manifest := []ToolGovernanceDescriptor{
		{
			Name:            "pulse_query",
			Description:     "Query inventory.\nSecond line.",
			ActionMode:      ActionModeRead,
			ApprovalPolicy:  ApprovalPolicyScopeOnly,
			ApprovalSummary: "no approval required",
			Summary:         "Read inventory.",
		},
		{
			Name:            "pulse_control",
			Description:     "Run controlled actions.",
			ActionMode:      ActionModeWrite,
			ApprovalPolicy:  ApprovalPolicyActionPlan,
			ApprovalSummary: "approval required in controlled mode",
		},
	}

	prompt := BuildToolGovernancePromptSection(manifest, ToolGovernancePromptOptions{})

	for _, expected := range []string{
		"## AVAILABLE TOOL GOVERNANCE",
		"Pulse's governed tool registry",
		"pulse_query: mode=read; approval=scope_only (no approval required); Read inventory.",
		"pulse_control: mode=write; approval=action_plan (approval required in controlled mode); Run controlled actions.",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected prompt to include %q, got %q", expected, prompt)
		}
	}
}

func TestBuildToolGovernancePromptSectionHonorsOfferedTools(t *testing.T) {
	manifest := []ToolGovernanceDescriptor{
		{Name: "pulse_query", ActionMode: ActionModeRead, ApprovalPolicy: ApprovalPolicyScopeOnly},
		{Name: "pulse_control", ActionMode: ActionModeWrite, ApprovalPolicy: ApprovalPolicyActionPlan},
	}

	prompt := BuildToolGovernancePromptSection(manifest, ToolGovernancePromptOptions{
		OfferedToolNames: []string{" pulse_query ", "pulse_query", "provider_extra"},
	})

	if !strings.Contains(prompt, "pulse_query: mode=read; approval=scope_only") {
		t.Fatalf("expected offered manifest tool, got %q", prompt)
	}
	if strings.Contains(prompt, "pulse_control:") {
		t.Fatalf("did not expect non-offered manifest tool, got %q", prompt)
	}
	if !strings.Contains(prompt, "provider_extra: mode=read; approval=scope_only (no approval required); Offered by Pulse for this turn.") {
		t.Fatalf("expected fallback offered-tool line, got %q", prompt)
	}
	if strings.Count(prompt, "pulse_query:") != 1 {
		t.Fatalf("expected duplicate offered tool names to be collapsed, got %q", prompt)
	}
}

func TestBuildToolGovernancePromptSectionAppendsAdditionalSurfaceTools(t *testing.T) {
	prompt := BuildToolGovernancePromptSection([]ToolGovernanceDescriptor{
		{Name: "pulse_query", ActionMode: ActionModeRead, ApprovalPolicy: ApprovalPolicyScopeOnly},
	}, ToolGovernancePromptOptions{
		OfferedToolNames: []string{"pulse_query", PulseQuestionToolName},
		AdditionalToolGovernanceLines: []string{
			" " + PulseQuestionToolGovernancePromptLine() + " ",
			PulseQuestionToolGovernancePromptLine(),
		},
	})

	if !strings.Contains(prompt, "pulse_query: mode=read; approval=scope_only") {
		t.Fatalf("expected offered manifest tool, got %q", prompt)
	}
	if strings.Count(prompt, PulseQuestionToolName+":") != 1 {
		t.Fatalf("expected one shared question-tool governance line, got %q", prompt)
	}
	if !strings.Contains(prompt, "mode=interactive; approval=user answer required") {
		t.Fatalf("expected interactive question-tool governance, got %q", prompt)
	}
}

func TestBuildToolGovernancePromptSectionQuestionOnlyDoesNotUseNoToolsGuidance(t *testing.T) {
	prompt := BuildToolGovernancePromptSection(nil, ToolGovernancePromptOptions{
		OfferedToolNames: []string{},
		AdditionalToolGovernanceLines: []string{
			PulseQuestionToolGovernancePromptLine(),
		},
	})

	if strings.Contains(prompt, "No Pulse tools are offered for this turn") {
		t.Fatalf("question-only prompt must not use no-tools guidance, got %q", prompt)
	}
	if !strings.Contains(prompt, PulseQuestionToolGovernancePromptLine()) {
		t.Fatalf("expected shared question-tool governance line, got %q", prompt)
	}
}

func TestBuildAssistantToolGovernancePromptSectionProjectsQuestionAsInteractiveSurfaceTool(t *testing.T) {
	prompt := BuildAssistantToolGovernancePromptSection([]ToolGovernanceDescriptor{
		{Name: "pulse_query", ActionMode: ActionModeRead, ApprovalPolicy: ApprovalPolicyScopeOnly},
	}, []string{" pulse_query ", PulseQuestionToolName, PulseQuestionToolName})

	if !strings.Contains(prompt, "pulse_query: mode=read; approval=scope_only") {
		t.Fatalf("expected offered manifest tool, got %q", prompt)
	}
	if strings.Count(prompt, PulseQuestionToolName+":") != 1 {
		t.Fatalf("expected one Assistant question governance line, got %q", prompt)
	}
	if !strings.Contains(prompt, PulseQuestionToolGovernancePromptLine()) {
		t.Fatalf("expected Assistant question tool to use shared interactive governance, got %q", prompt)
	}
	if strings.Contains(prompt, PulseQuestionToolName+": mode=read") {
		t.Fatalf("question tool must not fall back to generic read governance, got %q", prompt)
	}
}

func TestAssistantGovernanceOfferedToolNamesSplitsNativeSurfaceTools(t *testing.T) {
	filtered, includeQuestion := AssistantGovernanceOfferedToolNames([]string{
		" pulse_query ",
		PulseQuestionToolName,
		"pulse_query",
		"",
	})
	if !includeQuestion {
		t.Fatal("expected Assistant question tool to be detected")
	}
	if len(filtered) != 1 || filtered[0] != "pulse_query" {
		t.Fatalf("filtered manifest names = %#v, want [pulse_query]", filtered)
	}

	allNames, includeQuestion := AssistantGovernanceOfferedToolNames(nil)
	if allNames != nil || !includeQuestion {
		t.Fatalf("nil offered names must preserve all-manifest plus interactive-question semantics, got %#v include=%v", allNames, includeQuestion)
	}

	none, includeQuestion := AssistantGovernanceOfferedToolNames([]string{})
	if none == nil || len(none) != 0 || includeQuestion {
		t.Fatalf("empty offered names must mean no tools, got %#v include=%v", none, includeQuestion)
	}
}

func TestBuildToolGovernancePromptSectionNoOfferedTools(t *testing.T) {
	prompt := BuildToolGovernancePromptSection([]ToolGovernanceDescriptor{
		{Name: "pulse_query", ActionMode: ActionModeRead, ApprovalPolicy: ApprovalPolicyScopeOnly},
	}, ToolGovernancePromptOptions{OfferedToolNames: []string{}})

	if !strings.Contains(prompt, "No Pulse tools are offered for this turn") {
		t.Fatalf("expected no-tools guidance, got %q", prompt)
	}
	if strings.Contains(prompt, "pulse_query:") {
		t.Fatalf("expected no manifest tools when offered list is empty, got %q", prompt)
	}
}

func TestToolGovernancePromptLineUsesSharedDefaults(t *testing.T) {
	got := ToolGovernancePromptLine(ToolGovernanceDescriptor{
		Name:        " pulse_read ",
		Description: "Read data.\nLonger details.",
	})

	want := "pulse_read: mode=read; approval=scope_only; Read data."
	if got != want {
		t.Fatalf("ToolGovernancePromptLine = %q, want %q", got, want)
	}
}

func TestToolGovernancePromptDescriptionOmitsToolNameForProviderDeclarations(t *testing.T) {
	got := ToolGovernancePromptDescription(ToolGovernanceDescriptor{
		Name:            "pulse_control",
		Description:     "Run controlled actions.",
		ActionMode:      ActionModeWrite,
		ApprovalPolicy:  ApprovalPolicyActionPlan,
		ApprovalSummary: "approval required in controlled mode",
	})

	want := "mode=write; approval=action_plan (approval required in controlled mode); Run controlled actions."
	if got != want {
		t.Fatalf("ToolGovernancePromptDescription = %q, want %q", got, want)
	}
	if strings.Contains(got, "pulse_control:") {
		t.Fatalf("provider-facing governance description must not include tool name prefix: %q", got)
	}
}

func TestPulseQuestionToolGovernancePromptLineNamesInteractiveBoundary(t *testing.T) {
	line := PulseQuestionToolGovernancePromptLine()

	for _, expected := range []string{
		PulseQuestionToolName,
		"mode=interactive",
		"approval=user answer required",
		"interactive chat only",
	} {
		if !strings.Contains(line, expected) {
			t.Fatalf("expected question-tool governance line to include %q, got %q", expected, line)
		}
	}
}

func TestBuildPulseIntelligenceOperatingInstructionsNamesGovernedActionPosture(t *testing.T) {
	instructions := BuildPulseIntelligenceOperatingInstructions()

	for _, expected := range []string{
		"## PULSE INTELLIGENCE OPERATING MODEL",
		"governed infrastructure-operations surface",
		"offered tools, resources, prompts, and capability metadata",
		"Prefer read-only context first",
		"plan, approval, and execute flow",
		"approval-required, policy-blocked, and unavailable-tool results",
	} {
		if !strings.Contains(instructions, expected) {
			t.Fatalf("expected shared operating instructions to include %q, got %q", expected, instructions)
		}
	}
}

func TestBuildPulseAssistantOperatingInstructionsNamesOnlyAssistantAffordances(t *testing.T) {
	instructions := BuildPulseAssistantOperatingInstructions()

	for _, expected := range []string{
		"This surface is Pulse Assistant.",
		"Use offered tools and interactive questions as the source of truth",
		"Pulse Assistant is the native Pulse surface over Pulse Intelligence Core",
		"Pulse MCP is the external-agent adapter on the same core",
		"Surface affordances: Pulse Assistant exposes tools and interactive questions.",
		"Pulse Patrol is the primary built-in operator on Pulse Intelligence Core",
		"governed infrastructure-operations surface",
		"plan, approval, and execute flow",
	} {
		if !strings.Contains(instructions, expected) {
			t.Fatalf("expected Assistant operating instructions to include %q, got %q", expected, instructions)
		}
	}
	for _, forbidden := range []string{
		"resources, prompts",
		"capability metadata",
	} {
		if strings.Contains(instructions, forbidden) {
			t.Fatalf("Assistant operating instructions must not advertise MCP-only affordance %q: %q", forbidden, instructions)
		}
	}
}

func TestBuildPulseMCPOperatingInstructionsReflectsAdvertisedAffordances(t *testing.T) {
	minimal := BuildPulseMCPOperatingInstructions(PulseMCPOperatingInstructionOptions{
		SupportsTools: true,
	})
	if !strings.Contains(minimal, "This surface is Pulse MCP.") ||
		!strings.Contains(minimal, "Use offered tools as the source of truth") {
		t.Fatalf("expected minimal MCP instructions to name tool-only MCP affordance, got %q", minimal)
	}
	if strings.Contains(minimal, "resources") ||
		strings.Contains(minimal, "prompts") ||
		strings.Contains(minimal, "capability metadata") {
		t.Fatalf("minimal MCP instructions must not advertise unsupported affordances: %q", minimal)
	}

	manifestBacked := BuildPulseMCPOperatingInstructions(PulseMCPOperatingInstructionOptions{
		SupportsTools:              true,
		SupportsResources:          true,
		SupportsPrompts:            true,
		SupportsCapabilityMetadata: true,
		SurfaceContract:            CanonicalManifest().SurfaceContract,
	})
	if !strings.Contains(manifestBacked, "Use offered tools, resources, prompts, and capability metadata as the source of truth") {
		t.Fatalf("manifest-backed MCP instructions must include advertised resources/prompts/metadata, got %q", manifestBacked)
	}
	for _, expected := range []string{
		"Pulse MCP is the external-agent adapter over Pulse Intelligence Core",
		"Pulse Assistant is the native Pulse surface on the same core",
		"Surface affordances: Pulse MCP exposes tools, resources, prompts, and capability metadata.",
		"Pulse Patrol is the primary built-in operator on Pulse Intelligence Core",
	} {
		if !strings.Contains(manifestBacked, expected) {
			t.Fatalf("manifest-backed MCP instructions must include surface contract %q, got %q", expected, manifestBacked)
		}
	}

	noTools := BuildPulseMCPOperatingInstructions(PulseMCPOperatingInstructionOptions{
		SupportsResources: true,
		SurfaceContract:   CanonicalManifest().SurfaceContract,
	})
	if strings.Contains(noTools, "offered tools") {
		t.Fatalf("MCP instructions must not advertise tools when initialize omitted the tools capability: %q", noTools)
	}
}

func TestBuildPulseMCPOperatingInstructionsUsesRequestedSurfaceContract(t *testing.T) {
	contract := SurfaceContract{
		Core:            CanonicalManifest().SurfaceContract.Core,
		ProactiveEngine: CanonicalManifest().SurfaceContract.ProactiveEngine,
		OperatorSurfaces: []OperatorSurfaceContract{
			{
				ID:              SurfaceIDPulseMCP,
				Label:           "Pulse MCP",
				ExternalAdapter: true,
				Affordances: SurfaceAffordanceContract{
					Tools:              true,
					Resources:          true,
					Prompts:            true,
					CapabilityMetadata: true,
				},
			},
			{
				ID:              "custom_external_agent",
				Label:           "Custom external agent",
				ExternalAdapter: true,
				Affordances: SurfaceAffordanceContract{
					Tools:     true,
					Resources: true,
				},
			},
		},
	}

	instructions := BuildPulseMCPOperatingInstructions(PulseMCPOperatingInstructionOptions{
		SurfaceID:                  "custom_external_agent",
		SupportsTools:              true,
		SupportsResources:          true,
		SupportsPrompts:            true,
		SupportsCapabilityMetadata: true,
		SurfaceContract:            contract,
	})

	for _, expected := range []string{
		"This surface is Custom external agent.",
		"Use offered tools and resources as the source of truth",
		"Surface affordances: Custom external agent exposes tools and resources.",
	} {
		if !strings.Contains(instructions, expected) {
			t.Fatalf("expected custom-surface instructions to include %q, got %q", expected, instructions)
		}
	}
	for _, forbidden := range []string{
		"Use offered tools, resources, prompts",
		"Custom external agent exposes tools, resources, prompts",
		"capability metadata",
	} {
		if strings.Contains(instructions, forbidden) {
			t.Fatalf("custom-surface instructions must not fall back to Pulse MCP affordance %q: %q", forbidden, instructions)
		}
	}
}
