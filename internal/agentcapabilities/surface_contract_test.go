package agentcapabilities

import (
	"reflect"
	"testing"
)

func TestDefaultSurfaceAffordancesPinSupportedSurfaces(t *testing.T) {
	assistant := DefaultSurfaceAffordancesForID(SurfaceIDPulseAssistant)
	if !assistant.Tools || !assistant.InteractiveQuestions {
		t.Fatalf("Pulse Assistant affordances = %+v, want tools and interactive questions", assistant)
	}
	if assistant.Resources || assistant.Prompts || assistant.CapabilityMetadata {
		t.Fatalf("Pulse Assistant must not default to MCP-only affordances: %+v", assistant)
	}

	mcp := DefaultSurfaceAffordancesForID(SurfaceIDPulseMCP)
	if !mcp.Tools || !mcp.Resources || !mcp.Prompts || !mcp.CapabilityMetadata {
		t.Fatalf("Pulse MCP affordances = %+v, want tools/resources/prompts/capability metadata", mcp)
	}
	if mcp.InteractiveQuestions {
		t.Fatalf("Pulse MCP must not default to native interactive questions: %+v", mcp)
	}
}

func TestNormalizeSurfaceAffordancesPreservesExplicitManifestValues(t *testing.T) {
	surface := OperatorSurfaceContract{
		ID: SurfaceIDPulseMCP,
		Affordances: SurfaceAffordanceContract{
			Tools: true,
		},
	}

	got := NormalizeSurfaceAffordances(surface)
	if !got.Tools {
		t.Fatalf("explicit tools affordance lost: %+v", got)
	}
	if got.Resources || got.Prompts || got.CapabilityMetadata || got.InteractiveQuestions {
		t.Fatalf("explicit affordances must not be expanded by defaults: %+v", got)
	}
}

func TestResolveSurfaceAffordanceContractResolvesLabelsAndDefaults(t *testing.T) {
	canonical, ok := ResolveSurfaceAffordanceContract(CanonicalManifest().SurfaceContract, SurfaceIDPulseMCP, "")
	if !ok {
		t.Fatal("canonical Pulse MCP surface must resolve from manifest")
	}
	if canonical.SurfaceID != SurfaceIDPulseMCP || canonical.SurfaceLabel != "Pulse MCP" {
		t.Fatalf("canonical Pulse MCP surface identity = %+v", canonical)
	}
	if !canonical.Affordances.Tools || !canonical.Affordances.Resources || !canonical.Affordances.Prompts || !canonical.Affordances.CapabilityMetadata {
		t.Fatalf("canonical Pulse MCP affordances = %+v", canonical.Affordances)
	}

	legacyAssistant, ok := ResolveSurfaceAffordanceContract(SurfaceContract{}, SurfaceIDPulseAssistant, "")
	if ok {
		t.Fatal("legacy Assistant fallback must not report a declared surface")
	}
	if legacyAssistant.SurfaceLabel != "Pulse Assistant" || !legacyAssistant.Affordances.Tools || !legacyAssistant.Affordances.InteractiveQuestions {
		t.Fatalf("legacy Assistant fallback = %+v", legacyAssistant)
	}
	if legacyAssistant.Affordances.Resources || legacyAssistant.Affordances.Prompts || legacyAssistant.Affordances.CapabilityMetadata {
		t.Fatalf("legacy Assistant fallback must not inherit MCP affordances: %+v", legacyAssistant.Affordances)
	}

	custom, ok := ResolveSurfaceAffordanceContract(SurfaceContract{}, "custom_external_agent", "")
	if ok {
		t.Fatal("unknown custom surface fallback must not report a declared surface")
	}
	if custom.SurfaceLabel != "custom_external_agent" || SurfaceAffordancesDeclared(custom.Affordances) {
		t.Fatalf("unknown custom surface fallback = %+v, want id label and empty affordances", custom)
	}
}

func TestManifestSurfaceAffordancesResolvesDeclaredAndDefaultSurfaces(t *testing.T) {
	defaults, ok := ManifestSurfaceAffordances(Manifest{}, SurfaceIDPulseMCP)
	if ok {
		t.Fatal("legacy manifest fallback must report compatibility default, not a declared surface")
	}
	if !defaults.Tools || !defaults.Resources || !defaults.Prompts || !defaults.CapabilityMetadata || defaults.InteractiveQuestions {
		t.Fatalf("Pulse MCP default affordances = %+v", defaults)
	}

	manifest := Manifest{SurfaceContract: SurfaceContract{OperatorSurfaces: []OperatorSurfaceContract{{
		ID:              "custom_external_agent",
		Label:           "Custom external agent",
		ExternalAdapter: true,
		Affordances: SurfaceAffordanceContract{
			Tools:     true,
			Resources: true,
		},
	}}}}
	declared, ok := ManifestSurfaceAffordances(manifest, "custom_external_agent")
	if !ok {
		t.Fatal("declared custom surface was not found")
	}
	if !declared.Tools || !declared.Resources || declared.Prompts || declared.CapabilityMetadata || declared.InteractiveQuestions {
		t.Fatalf("declared custom surface affordances = %+v", declared)
	}

	if unknown, ok := ManifestSurfaceAffordances(Manifest{}, "unknown_surface"); ok || SurfaceAffordancesDeclared(unknown) {
		t.Fatalf("unknown surface affordances = %+v ok=%v, want empty", unknown, ok)
	}
}

func TestSurfaceAffordanceLabelsAreStable(t *testing.T) {
	got := SurfaceAffordanceLabels(SurfaceAffordanceContract{
		Tools:                true,
		Resources:            true,
		Prompts:              true,
		CapabilityMetadata:   true,
		InteractiveQuestions: true,
	})
	want := []string{"tools", "resources", "prompts", "capability metadata", "interactive questions"}
	if len(got) != len(want) {
		t.Fatalf("labels = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("labels = %#v, want %#v", got, want)
		}
	}
}

func TestFindOperatorSurfaceContractMatchesIDOrLabel(t *testing.T) {
	contract := CanonicalManifest().SurfaceContract
	if surface, ok := FindOperatorSurfaceContract(contract, SurfaceIDPulseAssistant, ""); !ok || surface.Label != "Pulse Assistant" {
		t.Fatalf("find by id = %+v ok=%v", surface, ok)
	}
	if surface, ok := FindOperatorSurfaceContract(contract, "", "Pulse MCP"); !ok || surface.ID != SurfaceIDPulseMCP {
		t.Fatalf("find by label = %+v ok=%v", surface, ok)
	}
}

func TestSurfaceToolContractNormalizeCollectionsDetachesNames(t *testing.T) {
	source := []string{PulseQueryToolName}
	contract := SurfaceToolContract{ToolNames: source}.NormalizeCollections()
	if len(contract.ToolNames) != 1 || contract.ToolNames[0] != PulseQueryToolName {
		t.Fatalf("normalized names = %#v", contract.ToolNames)
	}
	contract.ToolNames[0] = PulseReadToolName
	if source[0] != PulseQueryToolName {
		t.Fatalf("NormalizeCollections returned aliased tool names: source=%#v normalized=%#v", source, contract.ToolNames)
	}

	empty := SurfaceToolContract{}.NormalizeCollections()
	if empty.ToolNames == nil || len(empty.ToolNames) != 0 {
		t.Fatalf("empty tool names = %#v, want stable empty list", empty.ToolNames)
	}
}

func TestCloneSurfaceToolContractsDetachesNames(t *testing.T) {
	if empty := CloneSurfaceToolContracts(nil); empty == nil || len(empty) != 0 {
		t.Fatalf("empty surface tool contracts = %#v, want stable empty list", empty)
	}

	source := []SurfaceToolContract{{
		SurfaceID:         SurfaceIDPulseMCP,
		ToolNames:         []string{FleetContextCapabilityName},
		CapabilityNames:   []string{FleetContextCapabilityName},
		RegistryToolNames: []string{PulseReadToolName},
		NativeToolNames:   []string{PulseQuestionToolName},
	}}

	cloned := CloneSurfaceToolContracts(source)
	source[0].ToolNames[0] = "mutated"
	source[0].CapabilityNames[0] = "mutated"
	source[0].RegistryToolNames[0] = "mutated"
	source[0].NativeToolNames[0] = "mutated"

	if !reflect.DeepEqual(cloned[0].ToolNames, []string{FleetContextCapabilityName}) {
		t.Fatalf("cloned tool names = %#v", cloned[0].ToolNames)
	}
	if !reflect.DeepEqual(cloned[0].CapabilityNames, []string{FleetContextCapabilityName}) {
		t.Fatalf("cloned capability names = %#v", cloned[0].CapabilityNames)
	}
	if !reflect.DeepEqual(cloned[0].RegistryToolNames, []string{PulseReadToolName}) {
		t.Fatalf("cloned registry names = %#v", cloned[0].RegistryToolNames)
	}
	if !reflect.DeepEqual(cloned[0].NativeToolNames, []string{PulseQuestionToolName}) {
		t.Fatalf("cloned native names = %#v", cloned[0].NativeToolNames)
	}
}

func TestProjectPulseAssistantSurfaceToolContractSeparatesRegistryAndNativeTools(t *testing.T) {
	contract, ok := ProjectPulseAssistantSurfaceToolContract(CanonicalManifest().SurfaceContract, []ProviderTool{
		{Name: PulseQueryToolName},
		{Name: PulseControlToolName},
		NewPulseQuestionProviderTool(),
	})
	if !ok {
		t.Fatal("canonical manifest must declare Pulse Assistant surface")
	}
	if contract.SurfaceID != SurfaceIDPulseAssistant || contract.SurfaceLabel != "Pulse Assistant" {
		t.Fatalf("Assistant surface identity = %+v", contract)
	}
	if contract.ToolSource != SurfaceToolSourceAssistantRegistry {
		t.Fatalf("Assistant tool source = %q, want %s", contract.ToolSource, SurfaceToolSourceAssistantRegistry)
	}
	if !reflect.DeepEqual(contract.ToolNames, []string{PulseQueryToolName, PulseControlToolName, PulseQuestionToolName}) {
		t.Fatalf("Assistant tool names = %#v", contract.ToolNames)
	}
	if !reflect.DeepEqual(contract.RegistryToolNames, []string{PulseQueryToolName, PulseControlToolName}) {
		t.Fatalf("Assistant registry names = %#v", contract.RegistryToolNames)
	}
	if !reflect.DeepEqual(contract.NativeToolNames, []string{PulseQuestionToolName}) {
		t.Fatalf("Assistant native names = %#v", contract.NativeToolNames)
	}
	if len(contract.CapabilityNames) != 0 {
		t.Fatalf("Assistant contract must not pretend registry tools are manifest capabilities: %#v", contract.CapabilityNames)
	}
	if !contract.Affordances.Tools || !contract.Affordances.InteractiveQuestions || contract.Affordances.Resources {
		t.Fatalf("Assistant affordances = %+v", contract.Affordances)
	}

	legacy, ok := ProjectPulseAssistantSurfaceToolContract(SurfaceContract{}, []ProviderTool{
		NewPulseQuestionProviderTool(),
	})
	if ok {
		t.Fatal("legacy Assistant surface fallback must not report a declared surface")
	}
	if legacy.SurfaceID != SurfaceIDPulseAssistant || legacy.SurfaceLabel != "Pulse Assistant" {
		t.Fatalf("legacy Assistant surface identity = %+v", legacy)
	}
	if !legacy.Affordances.Tools || !legacy.Affordances.InteractiveQuestions || legacy.Affordances.Resources {
		t.Fatalf("legacy Assistant affordances = %+v", legacy.Affordances)
	}
}

func TestProjectPulseAssistantSurfaceToolContractHonorsSurfaceAffordances(t *testing.T) {
	providerTools := []ProviderTool{
		{Name: PulseQueryToolName},
		NewPulseQuestionProviderTool(),
	}

	toolsDisabled := CanonicalManifest().SurfaceContract
	for i := range toolsDisabled.OperatorSurfaces {
		if toolsDisabled.OperatorSurfaces[i].ID == SurfaceIDPulseAssistant {
			toolsDisabled.OperatorSurfaces[i].Affordances = SurfaceAffordanceContract{InteractiveQuestions: true}
		}
	}
	withoutTools, _ := ProjectPulseAssistantSurfaceToolContract(toolsDisabled, providerTools)
	if len(withoutTools.ToolNames) != 0 || len(withoutTools.RegistryToolNames) != 0 || len(withoutTools.NativeToolNames) != 0 {
		t.Fatalf("tools-disabled Assistant surface contract = %+v, want no published tools", withoutTools)
	}

	questionsDisabled := CanonicalManifest().SurfaceContract
	for i := range questionsDisabled.OperatorSurfaces {
		if questionsDisabled.OperatorSurfaces[i].ID == SurfaceIDPulseAssistant {
			questionsDisabled.OperatorSurfaces[i].Affordances = SurfaceAffordanceContract{Tools: true}
		}
	}
	withoutQuestions, _ := ProjectPulseAssistantSurfaceToolContract(questionsDisabled, providerTools)
	if !reflect.DeepEqual(withoutQuestions.ToolNames, []string{PulseQueryToolName}) {
		t.Fatalf("questions-disabled Assistant tool names = %#v, want registry tool only", withoutQuestions.ToolNames)
	}
	if len(withoutQuestions.NativeToolNames) != 0 {
		t.Fatalf("questions-disabled Assistant native names = %#v, want none", withoutQuestions.NativeToolNames)
	}
	if !reflect.DeepEqual(withoutQuestions.RegistryToolNames, []string{PulseQueryToolName}) {
		t.Fatalf("questions-disabled Assistant registry names = %#v", withoutQuestions.RegistryToolNames)
	}
}

func TestIntersectSurfaceAffordancesCannotEnableDisabledSurfaceAffordance(t *testing.T) {
	allowed := SurfaceAffordanceContract{
		Resources:          true,
		Prompts:            true,
		CapabilityMetadata: true,
	}
	requested := SurfaceAffordanceContract{
		Tools:              true,
		Resources:          true,
		Prompts:            true,
		CapabilityMetadata: true,
	}

	got := IntersectSurfaceAffordances(allowed, requested)
	if got.Tools {
		t.Fatalf("intersection re-enabled disabled tools affordance: %+v", got)
	}
	if !got.Resources || !got.Prompts || !got.CapabilityMetadata {
		t.Fatalf("intersection dropped allowed requested affordances: %+v", got)
	}
	if got.InteractiveQuestions {
		t.Fatalf("intersection enabled disabled interactive questions affordance: %+v", got)
	}
}

func TestProjectManifestSurfaceToolContractUsesPublishedSurfaceToolContract(t *testing.T) {
	manifest := Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		SurfaceToolContracts: []SurfaceToolContract{{
			SurfaceID:  SurfaceIDPulseMCP,
			ToolSource: SurfaceToolSourceCapabilityManifest,
			ToolNames: []string{
				SetOperatorStateCapabilityName,
				ResourceContextCapabilityName,
				EventSubscriptionCapabilityName,
				"missing_tool",
				SetOperatorStateCapabilityName,
			},
		}},
		Capabilities: []Capability{
			{Name: FleetContextCapabilityName, Description: "Read fleet context."},
			{Name: EventSubscriptionCapabilityName, Description: "Stream events."},
			{Name: ResourceContextCapabilityName, Description: "Read resource context."},
			{Name: SetOperatorStateCapabilityName, Description: "Set operator state."},
		},
	}

	contract, ok := ProjectManifestSurfaceToolContract(manifest, SurfaceIDPulseMCP)
	if !ok {
		t.Fatal("canonical manifest must declare Pulse MCP surface")
	}
	if contract.SurfaceID != SurfaceIDPulseMCP || contract.SurfaceLabel != "Pulse MCP" {
		t.Fatalf("MCP surface identity = %+v", contract)
	}
	if contract.ToolSource != SurfaceToolSourceCapabilityManifest {
		t.Fatalf("MCP tool source = %q, want %s", contract.ToolSource, SurfaceToolSourceCapabilityManifest)
	}
	want := []string{SetOperatorStateCapabilityName, ResourceContextCapabilityName}
	if !reflect.DeepEqual(contract.ToolNames, want) {
		t.Fatalf("MCP tool names = %#v, want %#v", contract.ToolNames, want)
	}
	if !reflect.DeepEqual(contract.CapabilityNames, want) {
		t.Fatalf("MCP capability names = %#v, want %#v", contract.CapabilityNames, want)
	}
	if len(contract.RegistryToolNames) != 0 || len(contract.NativeToolNames) != 0 {
		t.Fatalf("MCP contract must not expose Assistant registry/native tools: %+v", contract)
	}
	if !contract.Affordances.Tools || !contract.Affordances.Resources || !contract.Affordances.Prompts || !contract.Affordances.CapabilityMetadata || contract.Affordances.InteractiveQuestions {
		t.Fatalf("MCP affordances = %+v", contract.Affordances)
	}

}

func TestProjectManifestSurfaceToolContractCannotReenableDisabledSurfaceTools(t *testing.T) {
	surfaceContract := CloneSurfaceContract(CanonicalManifest().SurfaceContract)
	for i := range surfaceContract.OperatorSurfaces {
		if surfaceContract.OperatorSurfaces[i].ID == SurfaceIDPulseMCP {
			surfaceContract.OperatorSurfaces[i].Affordances = SurfaceAffordanceContract{
				Resources:          true,
				Prompts:            true,
				CapabilityMetadata: true,
			}
		}
	}
	manifest := Manifest{
		SurfaceContract: surfaceContract,
		SurfaceToolContracts: []SurfaceToolContract{{
			SurfaceID:   SurfaceIDPulseMCP,
			ToolSource:  SurfaceToolSourceCapabilityManifest,
			ToolNames:   []string{FleetContextCapabilityName},
			Affordances: DefaultSurfaceAffordancesForID(SurfaceIDPulseMCP),
		}},
		Capabilities: []Capability{
			{Name: FleetContextCapabilityName, Description: "Read fleet context."},
		},
	}

	contract, ok := ProjectManifestSurfaceToolContract(manifest, SurfaceIDPulseMCP)
	if !ok {
		t.Fatal("published Pulse MCP surface tool contract should still resolve")
	}
	if contract.Affordances.Tools {
		t.Fatalf("disabled surface tools affordance was re-enabled: %+v", contract.Affordances)
	}
	if len(contract.ToolNames) != 0 || len(contract.CapabilityNames) != 0 {
		t.Fatalf("tools-disabled external surface exposed tool names: %+v", contract)
	}
	if !contract.Affordances.Resources || !contract.Affordances.Prompts || !contract.Affordances.CapabilityMetadata {
		t.Fatalf("allowed non-tool affordances were lost: %+v", contract.Affordances)
	}
}

func TestProjectManifestSurfaceToolContractRequiresPublishedSurfaceToolContract(t *testing.T) {
	manifest := Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		Capabilities: []Capability{
			{Name: FleetContextCapabilityName, Description: "Read fleet context."},
			{Name: EventSubscriptionCapabilityName, Description: "Stream events."},
			{Name: ResourceContextCapabilityName, Description: "Read resource context."},
		},
	}

	if contract, ok := ProjectManifestSurfaceToolContract(manifest, SurfaceIDPulseMCP); ok {
		t.Fatalf("missing Pulse MCP surface tool contract must fail closed, got %+v", contract)
	}
	if contracts := ProjectManifestSurfaceToolContracts(manifest); len(contracts) != 0 {
		t.Fatalf("missing external surface tool contract projected tools: %+v", contracts)
	}
}

func TestProjectManifestSurfaceToolContractRejectsNativeOrMissingSurfaces(t *testing.T) {
	manifest := Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		Capabilities: []Capability{
			{Name: FleetContextCapabilityName, Description: "Read fleet context."},
		},
	}

	if contract, ok := ProjectManifestSurfaceToolContract(manifest, SurfaceIDPulseAssistant); ok {
		t.Fatalf("native Assistant surface must not project static manifest tools: %+v", contract)
	}
	if contract, ok := ProjectManifestSurfaceToolContract(manifest, "missing_surface"); ok {
		t.Fatalf("missing surface must not project static manifest tools: %+v", contract)
	}
}

func TestProjectManifestSurfaceToolContractsReturnsExternalAdaptersInManifestOrder(t *testing.T) {
	surfaceContract := CanonicalManifest().SurfaceContract
	surfaceContract.OperatorSurfaces = append(surfaceContract.OperatorSurfaces, OperatorSurfaceContract{
		ID:              "pulse_cli_agent",
		Label:           "Pulse CLI Agent",
		ExternalAdapter: true,
		Affordances: SurfaceAffordanceContract{
			Tools:              true,
			CapabilityMetadata: true,
		},
	})
	manifest := Manifest{
		SurfaceContract: surfaceContract,
		SurfaceToolContracts: []SurfaceToolContract{
			{
				SurfaceID:  SurfaceIDPulseMCP,
				ToolSource: SurfaceToolSourceCapabilityManifest,
				ToolNames:  []string{FleetContextCapabilityName},
			},
			{
				SurfaceID:  "pulse_cli_agent",
				ToolSource: SurfaceToolSourceCapabilityManifest,
				ToolNames:  []string{PlanActionCapabilityName},
			},
		},
		Capabilities: []Capability{
			{Name: FleetContextCapabilityName, Description: "Read fleet context."},
			{Name: EventSubscriptionCapabilityName, Description: "Stream events."},
			{Name: PlanActionCapabilityName, Description: "Plan a governed action."},
		},
	}

	contracts := ProjectManifestSurfaceToolContracts(manifest)
	if len(contracts) != 2 {
		t.Fatalf("manifest surface contracts = %+v, want MCP and custom external adapter", contracts)
	}
	if contracts[0].SurfaceID != SurfaceIDPulseMCP || contracts[1].SurfaceID != "pulse_cli_agent" {
		t.Fatalf("manifest external surface order = %+v", contracts)
	}
	if !reflect.DeepEqual(contracts[0].ToolNames, []string{FleetContextCapabilityName}) {
		t.Fatalf("MCP surface tool names = %#v", contracts[0].ToolNames)
	}
	if !reflect.DeepEqual(contracts[1].ToolNames, []string{PlanActionCapabilityName}) {
		t.Fatalf("custom external surface tool names = %#v", contracts[1].ToolNames)
	}
	for _, contract := range contracts {
		if contract.ToolSource != SurfaceToolSourceCapabilityManifest {
			t.Fatalf("external surface source = %q", contract.ToolSource)
		}
		for _, name := range contract.ToolNames {
			if name == EventSubscriptionCapabilityName {
				t.Fatalf("external surface tool names included streaming capability: %#v", contract.ToolNames)
			}
		}
	}
}

func TestProjectPulseIntelligenceSurfaceToolContractsPreservesManifestSurfaceOrder(t *testing.T) {
	surfaceContract := CanonicalManifest().SurfaceContract
	surfaceContract.OperatorSurfaces = append(surfaceContract.OperatorSurfaces, OperatorSurfaceContract{
		ID:              "pulse_cli_agent",
		Label:           "Pulse CLI Agent",
		ExternalAdapter: true,
		Affordances: SurfaceAffordanceContract{
			Tools: true,
		},
	})
	manifest := Manifest{
		SurfaceContract: surfaceContract,
		SurfaceToolContracts: []SurfaceToolContract{
			{
				SurfaceID:  SurfaceIDPulseMCP,
				ToolSource: SurfaceToolSourceCapabilityManifest,
				ToolNames:  []string{FleetContextCapabilityName},
			},
			{
				SurfaceID:  "pulse_cli_agent",
				ToolSource: SurfaceToolSourceCapabilityManifest,
				ToolNames:  []string{},
			},
		},
		Capabilities: []Capability{
			{Name: FleetContextCapabilityName, Description: "Read fleet context."},
			{Name: PlanActionCapabilityName, Description: "Plan a governed action."},
			{Name: EventSubscriptionCapabilityName, Description: "Stream events."},
		},
	}
	contracts := ProjectPulseIntelligenceSurfaceToolContracts(manifest, []ProviderTool{
		{Name: PulseQueryToolName},
		NewPulseQuestionProviderTool(),
	})

	if len(contracts) != 3 {
		t.Fatalf("surface contracts = %+v, want Assistant, MCP, and custom external adapter", contracts)
	}
	if contracts[0].SurfaceID != SurfaceIDPulseAssistant || contracts[1].SurfaceID != SurfaceIDPulseMCP || contracts[2].SurfaceID != "pulse_cli_agent" {
		t.Fatalf("surface contract order = %+v", contracts)
	}
	if !reflect.DeepEqual(contracts[0].ToolNames, []string{PulseQueryToolName, PulseQuestionToolName}) {
		t.Fatalf("Assistant projected names = %#v", contracts[0].ToolNames)
	}
	if !reflect.DeepEqual(contracts[1].ToolNames, []string{FleetContextCapabilityName}) {
		t.Fatalf("MCP projected names = %#v", contracts[1].ToolNames)
	}
	if !reflect.DeepEqual(contracts[2].ToolNames, []string{}) {
		t.Fatalf("custom external adapter projected names = %#v", contracts[2].ToolNames)
	}
}
