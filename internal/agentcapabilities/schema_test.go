package agentcapabilities

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestStrictObjectInputSchemaBuildsClosedManifestSchema(t *testing.T) {
	raw := StrictObjectInputSchema([]string{"resourceId"}, map[string]any{
		"resourceId": map[string]any{"type": "string"},
	})

	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schema["additionalProperties"] != false {
		t.Fatalf("strict schema additionalProperties = %v, want false", schema["additionalProperties"])
	}
	required, _ := schema["required"].([]any)
	if len(required) != 1 || required[0] != "resourceId" {
		t.Fatalf("strict schema required = %v, want [resourceId]", required)
	}
}

func TestObjectInputSchemaCanBuildPermissiveAdapterFallback(t *testing.T) {
	raw := ObjectInputSchema(nil, map[string]any{
		"body": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		},
	}, true)

	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schema["additionalProperties"] != true {
		t.Fatalf("permissive schema additionalProperties = %v, want true", schema["additionalProperties"])
	}
	if _, hasRequired := schema["required"]; hasRequired {
		t.Fatalf("schema without required args must omit required, got %v", schema["required"])
	}
}

func TestProviderInputSchemaProjectsStructuredToolSchema(t *testing.T) {
	required := []string{"mode"}
	enum := []string{"summary", "detail"}

	schema := InputSchema{
		Type: "object",
		Properties: map[string]PropertySchema{
			"mode": {
				Type:        "string",
				Description: "Response detail level",
				Enum:        enum,
				Default:     "summary",
			},
			"limit": {
				Type:    "number",
				Default: 25,
			},
		},
		Required: required,
	}

	projected := ProviderInputSchema(schema)
	if projected["type"] != "object" {
		t.Fatalf("projected type = %v, want object", projected["type"])
	}
	if projected["additionalProperties"] != false {
		t.Fatalf("provider schema additionalProperties = %v, want false", projected["additionalProperties"])
	}

	props, ok := projected["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("projected properties type = %T, want map[string]interface{}", projected["properties"])
	}
	mode, ok := props["mode"].(map[string]interface{})
	if !ok {
		t.Fatalf("mode property type = %T, want map[string]interface{}", props["mode"])
	}
	if mode["type"] != "string" {
		t.Fatalf("mode type = %v, want string", mode["type"])
	}
	if mode["description"] != "Response detail level" {
		t.Fatalf("mode description = %v, want Response detail level", mode["description"])
	}
	if !reflect.DeepEqual(mode["enum"], []string{"summary", "detail"}) {
		t.Fatalf("mode enum = %#v, want summary/detail", mode["enum"])
	}
	if mode["default"] != "summary" {
		t.Fatalf("mode default = %v, want summary", mode["default"])
	}

	limit, ok := props["limit"].(map[string]interface{})
	if !ok {
		t.Fatalf("limit property type = %T, want map[string]interface{}", props["limit"])
	}
	if limit["default"] != 25 {
		t.Fatalf("limit default = %v, want 25", limit["default"])
	}

	if !reflect.DeepEqual(projected["required"], []string{"mode"}) {
		t.Fatalf("projected required = %#v, want mode", projected["required"])
	}
}

func TestParseProviderToolInputRejectsInternalApprovalMetadata(t *testing.T) {
	if input, ok := ParseProviderToolInput(`{"command":"uptime","_approval_id":"fabricated"}`); ok || input != nil {
		t.Fatalf("provider input with internal metadata = %#v, ok=%v; want rejected", input, ok)
	}
}

func TestValidateDeclaredToolArgumentsRejectsUnknownProviderProperties(t *testing.T) {
	schema := InputSchema{
		Type: "object",
		Properties: map[string]PropertySchema{
			"command": {Type: "string"},
		},
		Required: []string{"command"},
	}
	if err := ValidateDeclaredToolArguments(schema, map[string]any{
		"command": "uptime",
		"trusted": true,
	}); err == nil || !strings.Contains(err.Error(), "undeclared tool argument") {
		t.Fatalf("unknown provider argument error = %v, want undeclared tool argument", err)
	}
	if err := ValidateDeclaredToolArguments(schema, WithApprovalArgument(map[string]any{
		"command": "uptime",
	}, "server-owned")); err != nil {
		t.Fatalf("server-owned internal approval metadata must remain valid: %v", err)
	}
}

func TestProjectProviderToolsProjectsRegistryTools(t *testing.T) {
	tools := []Tool{
		{
			Name:        "pulse_read",
			Description: "Read infrastructure",
			InputSchema: InputSchema{
				Properties: map[string]PropertySchema{
					"target_host": {Type: "string", Description: "Host to inspect"},
				},
				Required: []string{"target_host"},
			},
		},
		{
			Name:        "pulse_query",
			Description: "Query inventory",
		},
	}

	projected := ProjectProviderTools(tools)
	if len(projected) != 2 {
		t.Fatalf("ProjectProviderTools length = %d, want 2", len(projected))
	}
	if projected[0].Name != "pulse_read" || projected[0].Description != "Read infrastructure" {
		t.Fatalf("first provider tool = %+v", projected[0])
	}
	if projected[0].InputSchema["type"] != "object" {
		t.Fatalf("provider input schema type = %v, want object", projected[0].InputSchema["type"])
	}
	props := projected[0].InputSchema["properties"].(map[string]interface{})
	targetHost := props["target_host"].(map[string]interface{})
	if targetHost["description"] != "Host to inspect" {
		t.Fatalf("target_host description = %v", targetHost["description"])
	}
	if !reflect.DeepEqual(projected[0].InputSchema["required"], []string{"target_host"}) {
		t.Fatalf("provider required = %#v", projected[0].InputSchema["required"])
	}
	if len(projected[1].InputSchema["properties"].(map[string]interface{})) != 0 {
		t.Fatalf("empty registry schema should project empty properties: %#v", projected[1].InputSchema)
	}
}

func TestProjectProviderToolsWithGovernanceAppendsSharedGovernanceDescriptions(t *testing.T) {
	tools := []Tool{
		{
			Name:        "pulse_read",
			Description: "Read infrastructure.",
		},
		{
			Name:        "pulse_control",
			Description: "Run controlled actions.",
		},
		{
			Name:        "pulse_extra",
			Description: "Provider-only helper.",
		},
	}
	governance := []ToolGovernanceDescriptor{
		NewToolGovernanceDescriptor("pulse_read", "Read infrastructure.", false, ToolGovernance{
			ActionMode:      ActionModeRead,
			ApprovalPolicy:  ApprovalPolicyScopeOnly,
			ApprovalSummary: "no approval required",
			Summary:         "Read infrastructure.",
		}),
		NewToolGovernanceDescriptor("pulse_control", "Run controlled actions.", true, ToolGovernance{
			ActionMode:      ActionModeWrite,
			ApprovalPolicy:  ApprovalPolicyActionPlan,
			ApprovalSummary: "approval required in controlled mode",
			Summary:         "State-changing action.",
		}),
	}

	projected := ProjectProviderToolsWithGovernance(tools, governance)
	if len(projected) != 3 {
		t.Fatalf("ProjectProviderToolsWithGovernance length = %d, want 3", len(projected))
	}
	if !strings.Contains(projected[0].Description, "Read infrastructure.") ||
		!strings.Contains(projected[0].Description, "Pulse governance: mode=read; approval=scope_only (no approval required); Read infrastructure.") {
		t.Fatalf("read provider description missing shared governance: %q", projected[0].Description)
	}
	assertProviderToolBehaviorHints(t, projected[0].BehaviorHints, true, false, true, true)
	if projected[0].PulseGovernance == nil || projected[0].PulseGovernance.Name != "pulse_read" || projected[0].PulseGovernance.ActionMode != ActionModeRead {
		t.Fatalf("read provider governance metadata = %+v, want normalized pulse_read/read", projected[0].PulseGovernance)
	}
	if !strings.Contains(projected[1].Description, "Run controlled actions.") ||
		!strings.Contains(projected[1].Description, "Pulse governance: mode=write; approval=action_plan (approval required in controlled mode); State-changing action.") {
		t.Fatalf("write provider description missing shared governance: %q", projected[1].Description)
	}
	assertProviderToolBehaviorHints(t, projected[1].BehaviorHints, false, true, false, true)
	if projected[1].PulseGovernance == nil || projected[1].PulseGovernance.Name != "pulse_control" || projected[1].PulseGovernance.ApprovalPolicy != ApprovalPolicyActionPlan {
		t.Fatalf("write provider governance metadata = %+v, want normalized pulse_control/action_plan", projected[1].PulseGovernance)
	}
	if strings.Contains(projected[2].Description, "Pulse governance:") {
		t.Fatalf("provider tool without matching governance must keep original description, got %q", projected[2].Description)
	}
	if projected[2].BehaviorHints != nil || projected[2].PulseGovernance != nil {
		t.Fatalf("provider tool without matching governance must not invent metadata, got hints=%+v governance=%+v", projected[2].BehaviorHints, projected[2].PulseGovernance)
	}
}

func assertProviderToolBehaviorHints(t *testing.T, hints *ToolBehaviorHints, readOnly, destructive, idempotent, openWorld bool) {
	t.Helper()
	if hints == nil {
		t.Fatal("provider tool behavior hints missing")
	}
	assertBoolPtr := func(label string, got *bool, want bool) {
		t.Helper()
		if got == nil || *got != want {
			t.Fatalf("%s hint = %v, want %v", label, got, want)
		}
	}
	assertBoolPtr("readOnly", hints.ReadOnlyHint, readOnly)
	assertBoolPtr("destructive", hints.DestructiveHint, destructive)
	assertBoolPtr("idempotent", hints.IdempotentHint, idempotent)
	assertBoolPtr("openWorld", hints.OpenWorldHint, openWorld)
}

func TestProjectAssistantProviderToolsOwnsNativeQuestionToolComposition(t *testing.T) {
	registryTools := []Tool{{
		Name:        "pulse_query",
		Description: "Query inventory.",
	}}
	governance := []ToolGovernanceDescriptor{
		NewToolGovernanceDescriptor("pulse_query", "Query inventory.", false, ToolGovernance{
			ActionMode:      ActionModeRead,
			ApprovalPolicy:  ApprovalPolicyScopeOnly,
			ApprovalSummary: "no approval required",
			Summary:         "Read inventory.",
		}),
	}

	interactive := ProjectAssistantProviderTools(registryTools, governance, AssistantProviderToolOptions{
		IncludeQuestionTool: true,
	})
	if len(interactive) != 2 {
		t.Fatalf("interactive Assistant tools length = %d, want 2", len(interactive))
	}
	if interactive[0].Name != "pulse_query" || !strings.Contains(interactive[0].Description, "Pulse governance: mode=read; approval=scope_only (no approval required); Read inventory.") {
		t.Fatalf("registry tool must retain shared governance projection, got %+v", interactive[0])
	}
	if !reflect.DeepEqual(interactive[1], NewPulseQuestionProviderTool()) {
		t.Fatalf("interactive Assistant surface must append shared question tool, got %+v", interactive[1])
	}

	nonInteractive := ProjectAssistantProviderTools(registryTools, governance, AssistantProviderToolOptions{})
	if len(nonInteractive) != 1 || nonInteractive[0].Name != "pulse_query" {
		t.Fatalf("non-interactive Assistant tools must exclude question tool, got %+v", nonInteractive)
	}
}

func TestProjectPulseAssistantProviderToolsUsesSurfaceAffordances(t *testing.T) {
	registryTools := []Tool{{
		Name:        PulseQueryToolName,
		Description: "Query inventory.",
	}}
	governance := []ToolGovernanceDescriptor{
		NewToolGovernanceDescriptor(PulseQueryToolName, "Query inventory.", false, ToolGovernance{ActionMode: ActionModeRead}),
	}

	manifest := CanonicalManifest()
	canonical := ProjectPulseAssistantProviderTools(manifest, registryTools, governance, AssistantProviderToolOptions{
		IncludeQuestionTool: true,
	})
	if names := ProviderToolNames(canonical); !reflect.DeepEqual(names, []string{PulseQueryToolName, PulseQuestionToolName}) {
		t.Fatalf("canonical Assistant provider tools = %#v, want registry plus question", names)
	}

	toolsDisabled := manifest
	for i := range toolsDisabled.SurfaceContract.OperatorSurfaces {
		if toolsDisabled.SurfaceContract.OperatorSurfaces[i].ID == SurfaceIDPulseAssistant {
			toolsDisabled.SurfaceContract.OperatorSurfaces[i].Affordances = SurfaceAffordanceContract{InteractiveQuestions: true}
		}
	}
	if got := ProjectPulseAssistantProviderTools(toolsDisabled, registryTools, governance, AssistantProviderToolOptions{IncludeQuestionTool: true}); len(got) != 0 {
		t.Fatalf("tools-disabled Assistant provider tools = %+v, want none", got)
	}

	questionsDisabled := manifest
	for i := range questionsDisabled.SurfaceContract.OperatorSurfaces {
		if questionsDisabled.SurfaceContract.OperatorSurfaces[i].ID == SurfaceIDPulseAssistant {
			questionsDisabled.SurfaceContract.OperatorSurfaces[i].Affordances = SurfaceAffordanceContract{Tools: true}
		}
	}
	nonInteractive := ProjectPulseAssistantProviderTools(questionsDisabled, registryTools, governance, AssistantProviderToolOptions{
		IncludeQuestionTool: true,
	})
	if names := ProviderToolNames(nonInteractive); !reflect.DeepEqual(names, []string{PulseQueryToolName}) {
		t.Fatalf("non-interactive Assistant provider tools = %#v, want registry tools only", names)
	}
}

func TestLegacyAssistantUtilityProviderToolsOwnCompatibilitySchemas(t *testing.T) {
	tools := LegacyAssistantUtilityProviderTools()
	if len(tools) != 2 {
		t.Fatalf("legacy Assistant utility tools = %+v, want two non-command compatibility tools", tools)
	}

	byName := map[string]ProviderTool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}

	if _, ok := byName[LegacyAssistantRunCommandToolName]; ok {
		t.Fatal("retired run_command alias must not be projected to providers")
	}

	fetchURL := byName[LegacyAssistantFetchURLToolName]
	if !reflect.DeepEqual(fetchURL.InputSchema["required"], []string{LegacyAssistantURLArgumentName}) {
		t.Fatalf("fetch_url required = %#v, want url", fetchURL.InputSchema["required"])
	}

	setResourceURL := byName[LegacyAssistantSetResourceURLToolName]
	if !reflect.DeepEqual(setResourceURL.InputSchema["required"], []string{LegacyAssistantResourceTypeArgumentName, LegacyAssistantResourceIDArgumentName}) {
		t.Fatalf("set_resource_url required = %#v, want resource_type/resource_id", setResourceURL.InputSchema["required"])
	}
	setProps := setResourceURL.InputSchema["properties"].(map[string]interface{})
	resourceType := setProps[LegacyAssistantResourceTypeArgumentName].(map[string]interface{})
	if !reflect.DeepEqual(resourceType["enum"], []string{"vm", "system-container", "oci-container", "app-container", "agent", "node", "docker-host"}) {
		t.Fatalf("set_resource_url resource_type enum = %#v", resourceType["enum"])
	}
}

func TestProviderToolGovernanceDescriptorsProjectOfferedMetadata(t *testing.T) {
	descriptors, ok := ProviderToolGovernanceDescriptors([]ProviderTool{
		{
			Name: PulseQuestionToolName,
		},
		{
			Name: "pulse_query",
			PulseGovernance: &ToolGovernanceDescriptor{
				Name:           "pulse_query",
				Description:    "Query inventory.",
				ActionMode:     ActionModeRead,
				ApprovalPolicy: ApprovalPolicyScopeOnly,
				Summary:        "Inventory read.",
			},
		},
		{
			Name: "pulse_query",
			PulseGovernance: &ToolGovernanceDescriptor{
				Name:       "pulse_query",
				ActionMode: ActionModeWrite,
			},
		},
	})
	if !ok {
		t.Fatal("ProviderToolGovernanceDescriptors rejected complete offered metadata")
	}
	if len(descriptors) != 1 {
		t.Fatalf("descriptors length = %d, want one registry descriptor", len(descriptors))
	}
	if descriptors[0].Name != "pulse_query" || descriptors[0].ActionMode != ActionModeRead || descriptors[0].Summary != "Inventory read." {
		t.Fatalf("projected descriptor = %+v, want normalized pulse_query read metadata", descriptors[0])
	}

	descriptors[0].Name = "mutated"
	again, ok := ProviderToolGovernanceDescriptors([]ProviderTool{{
		Name: "pulse_query",
		PulseGovernance: &ToolGovernanceDescriptor{
			Name:       "pulse_query",
			ActionMode: ActionModeRead,
		},
	}})
	if !ok || again[0].Name != "pulse_query" {
		t.Fatalf("ProviderToolGovernanceDescriptors must return detached descriptors, got %+v ok=%v", again, ok)
	}
}

func TestProviderToolGovernanceDescriptorsPreserveOfferedToolSemantics(t *testing.T) {
	if descriptors, ok := ProviderToolGovernanceDescriptors(nil); ok || descriptors != nil {
		t.Fatalf("nil offered tools must not claim complete metadata, got %+v ok=%v", descriptors, ok)
	}
	if descriptors, ok := ProviderToolGovernanceDescriptors([]ProviderTool{}); !ok || len(descriptors) != 0 {
		t.Fatalf("empty offered tools must be a valid no-tools manifest, got %+v ok=%v", descriptors, ok)
	}
	if descriptors, ok := ProviderToolGovernanceDescriptors([]ProviderTool{{Name: PulseQuestionToolName}}); !ok || len(descriptors) != 0 {
		t.Fatalf("question-only offered tools must be valid Assistant-native metadata, got %+v ok=%v", descriptors, ok)
	}
	for _, tools := range [][]ProviderTool{
		{{Name: "pulse_query"}},
		{{Name: "pulse_query", PulseGovernance: &ToolGovernanceDescriptor{Name: "pulse_read"}}},
	} {
		if descriptors, ok := ProviderToolGovernanceDescriptors(tools); ok || descriptors != nil {
			t.Fatalf("incomplete offered registry metadata must fall back, got %+v ok=%v", descriptors, ok)
		}
	}
}

func TestAssistantNativeProviderToolNamesUseSharedQuestionTool(t *testing.T) {
	tools := AssistantNativeProviderTools()
	if len(tools) != 1 || tools[0].Name != PulseQuestionToolName {
		t.Fatalf("native Assistant provider tools = %+v, want shared question tool", tools)
	}

	names := AssistantNativeProviderToolNames()
	if !reflect.DeepEqual(names, []string{PulseQuestionToolName}) {
		t.Fatalf("native Assistant provider tool names = %#v, want [%s]", names, PulseQuestionToolName)
	}

	names[0] = "mutated"
	if got := AssistantNativeProviderToolNames(); !reflect.DeepEqual(got, []string{PulseQuestionToolName}) {
		t.Fatalf("native Assistant provider tool names must be detached, got %#v", got)
	}
}

func TestProviderToolNamesPreservesOfferedToolSemantics(t *testing.T) {
	if got := ProviderToolNames(nil); got != nil {
		t.Fatalf("nil tools must preserve nil offered-name semantics, got %#v", got)
	}

	empty := ProviderToolNames([]ProviderTool{})
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty tools must preserve explicit no-tools semantics, got %#v", empty)
	}

	got := ProviderToolNames([]ProviderTool{
		{Name: " pulse_query "},
		{Name: "pulse_query"},
		{Name: ""},
		{Name: PulseQuestionToolName},
	})
	want := []string{"pulse_query", PulseQuestionToolName}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ProviderToolNames = %#v, want %#v", got, want)
	}
}

func TestProviderToolNameCatalogNormalizesExactAndPrefixLookups(t *testing.T) {
	catalog := NewProviderToolNameCatalog(
		[]string{" pulse_query ", "pulse_query", "", "pulse_read"},
		[]string{"patrol_report_finding"},
	)

	if !reflect.DeepEqual(catalog.Names(), []string{"pulse_query", "pulse_read", "patrol_report_finding"}) {
		t.Fatalf("catalog names = %#v", catalog.Names())
	}
	if !catalog.Has("pulse_query") || !catalog.Has("pulse_read") || !catalog.Has("patrol_report_finding") {
		t.Fatalf("catalog missing expected exact names: %#v", catalog.Names())
	}
	for _, name := range []string{"", " pulse_query ", "PULSE_QUERY", "unknown"} {
		if catalog.Has(name) {
			t.Fatalf("catalog Has(%q) = true, want false", name)
		}
	}
	for _, prefix := range []string{"p", "pulse_", "pulse_q", "patrol_"} {
		if !catalog.HasPrefix(prefix) {
			t.Fatalf("catalog HasPrefix(%q) = false, want true", prefix)
		}
	}
	for _, prefix := range []string{"", "Pulse_", "helper"} {
		if catalog.HasPrefix(prefix) {
			t.Fatalf("catalog HasPrefix(%q) = true, want false", prefix)
		}
	}

	names := catalog.Names()
	names[0] = "mutated"
	if catalog.Has("mutated") || !catalog.Has("pulse_query") {
		t.Fatalf("catalog Names() must return a detached copy, got %#v", catalog.Names())
	}
}

func TestAssistantProviderToolNameCatalogIncludesNativeQuestionTool(t *testing.T) {
	catalog := NewAssistantProviderToolNameCatalog([]string{"pulse_query", PulseQuestionToolName})
	want := []string{"pulse_query", PulseQuestionToolName}
	if !reflect.DeepEqual(catalog.Names(), want) {
		t.Fatalf("assistant provider tool catalog names = %#v, want %#v", catalog.Names(), want)
	}
	if !catalog.Has(PulseQuestionToolName) || !catalog.HasPrefix("pulse_q") {
		t.Fatalf("assistant provider tool catalog must include shared native question tool: %#v", catalog.Names())
	}
}

func TestProviderToolDescriptionWithGovernanceHandlesEmptyDescription(t *testing.T) {
	got := ProviderToolDescriptionWithGovernance("", ToolGovernanceDescriptor{
		Name:           "pulse_query",
		ActionMode:     ActionModeRead,
		ApprovalPolicy: ApprovalPolicyScopeOnly,
	})

	want := "Pulse governance: mode=read; approval=scope_only"
	if got != want {
		t.Fatalf("ProviderToolDescriptionWithGovernance = %q, want %q", got, want)
	}
}

func TestToolNormalizeCollectionsReturnsIndependentInputSchema(t *testing.T) {
	required := []string{"mode"}
	enum := []string{"summary", "detail"}
	defaultValue := map[string]interface{}{
		"filters": []interface{}{"active"},
	}
	properties := map[string]PropertySchema{
		"mode": {
			Type:    "string",
			Enum:    enum,
			Default: defaultValue,
		},
	}

	normalized := (Tool{
		Name: "pulse_query",
		InputSchema: InputSchema{
			Properties: properties,
			Required:   required,
		},
	}).NormalizeCollections()

	if normalized.InputSchema.Type != "object" {
		t.Fatalf("normalized schema type = %q, want object", normalized.InputSchema.Type)
	}
	if normalized.InputSchema.Properties == nil {
		t.Fatal("normalized schema properties must be an initialized object")
	}

	normalized.InputSchema.Required[0] = "changed"
	mode := normalized.InputSchema.Properties["mode"]
	mode.Enum[0] = "changed"
	mode.Default.(map[string]interface{})["filters"].([]interface{})[0] = "changed"
	normalized.InputSchema.Properties["mode"] = mode

	if required[0] != "mode" {
		t.Fatalf("source required was mutated to %q", required[0])
	}
	if enum[0] != "summary" {
		t.Fatalf("source enum was mutated to %q", enum[0])
	}
	if defaultValue["filters"].([]interface{})[0] != "active" {
		t.Fatalf("source default was mutated to %#v", defaultValue)
	}
	if properties["mode"].Enum[0] != "summary" {
		t.Fatalf("source property enum was mutated to %q", properties["mode"].Enum[0])
	}
}

func TestProviderToolNormalizeCollectionsKeepsInputSchemaObject(t *testing.T) {
	tool := EmptyProviderTool()
	if tool.InputSchema == nil {
		t.Fatal("EmptyProviderTool must initialize input_schema")
	}
	if len(tool.InputSchema) != 0 {
		t.Fatalf("EmptyProviderTool input_schema = %#v, want empty object", tool.InputSchema)
	}

	normalized := (ProviderTool{Name: "diagnose"}).NormalizeCollections()
	if normalized.InputSchema == nil {
		t.Fatal("NormalizeCollections must initialize input_schema")
	}

	inputSchema := map[string]interface{}{
		"properties": map[string]interface{}{
			"mode": map[string]interface{}{
				"enum": []interface{}{"summary", "detail"},
			},
		},
	}
	normalized = (ProviderTool{Name: "diagnose", InputSchema: inputSchema}).NormalizeCollections()
	properties := normalized.InputSchema["properties"].(map[string]interface{})
	mode := properties["mode"].(map[string]interface{})
	mode["enum"].([]interface{})[0] = "changed"
	if inputSchema["properties"].(map[string]interface{})["mode"].(map[string]interface{})["enum"].([]interface{})[0] != "summary" {
		t.Fatalf("NormalizeCollections must not alias provider input_schema: source=%#v normalized=%#v", inputSchema, normalized.InputSchema)
	}

	readOnly := true
	sourceGovernance := &ToolGovernanceDescriptor{
		Name:       "pulse_query",
		ActionMode: ActionModeRead,
	}
	withMetadata := ProviderTool{
		Name: "pulse_query",
		BehaviorHints: &ToolBehaviorHints{
			ReadOnlyHint: &readOnly,
		},
		PulseGovernance: sourceGovernance,
	}.NormalizeCollections()
	*withMetadata.BehaviorHints.ReadOnlyHint = false
	withMetadata.PulseGovernance.Name = "mutated"
	if !readOnly {
		t.Fatal("NormalizeCollections must detach provider behavior hints")
	}
	if sourceGovernance.Name != "pulse_query" {
		t.Fatalf("NormalizeCollections must detach provider governance metadata, source name = %q", sourceGovernance.Name)
	}
}

func TestNewPulseQuestionProviderToolBuildsSharedAssistantTool(t *testing.T) {
	tool := NewPulseQuestionProviderTool()

	if tool.Name != PulseQuestionToolName {
		t.Fatalf("question provider tool name = %q, want %q", tool.Name, PulseQuestionToolName)
	}
	if !strings.Contains(tool.Description, "Ask the user for missing information") {
		t.Fatalf("question provider tool description = %q", tool.Description)
	}
	if !strings.Contains(tool.Description, "Never use it to ask for internal identifiers") {
		t.Fatalf("question provider tool description must forbid asking for internal identifiers, got %q", tool.Description)
	}
	if tool.InputSchema["type"] != "object" {
		t.Fatalf("question provider input_schema type = %v, want object", tool.InputSchema["type"])
	}
	properties := tool.InputSchema["properties"].(map[string]interface{})
	questions := properties["questions"].(map[string]interface{})
	if questions["type"] != "array" {
		t.Fatalf("questions schema type = %v, want array", questions["type"])
	}
	items := questions["items"].(map[string]interface{})
	itemProperties := items["properties"].(map[string]interface{})
	if itemProperties["question"].(map[string]interface{})["type"] != "string" {
		t.Fatalf("question field schema = %#v", itemProperties["question"])
	}
	typeField := itemProperties["type"].(map[string]interface{})
	if !reflect.DeepEqual(typeField["enum"], PulseQuestionToolTypeValues()) {
		t.Fatalf("question type enum = %#v, want text/select", typeField["enum"])
	}
	if !reflect.DeepEqual(items["required"], []string{"id", "question"}) {
		t.Fatalf("question item required = %#v, want id/question", items["required"])
	}
	if !reflect.DeepEqual(tool.InputSchema["required"], []string{"questions"}) {
		t.Fatalf("question tool required = %#v, want questions", tool.InputSchema["required"])
	}
}

func TestNormalizePulseQuestionToolTypeUsesSharedDefaults(t *testing.T) {
	tests := []struct {
		name       string
		rawType    string
		hasOptions bool
		want       PulseQuestionToolType
		wantErr    string
	}{
		{name: "empty without options defaults to text", want: PulseQuestionToolTypeText},
		{name: "empty with options defaults to select", hasOptions: true, want: PulseQuestionToolTypeSelect},
		{name: "explicit text trims and lowercases", rawType: " TEXT ", want: PulseQuestionToolTypeText},
		{name: "explicit select with options", rawType: "select", hasOptions: true, want: PulseQuestionToolTypeSelect},
		{name: "explicit select requires options", rawType: "select", wantErr: "select questions must include options"},
		{name: "unknown type rejected", rawType: "binary", wantErr: "question.type must be 'text' or 'select'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePulseQuestionToolType(tt.rawType, tt.hasOptions)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("NormalizePulseQuestionToolType error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizePulseQuestionToolType returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizePulseQuestionToolType = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPulseQuestionProviderInputSchemaReturnsIndependentSchema(t *testing.T) {
	first := PulseQuestionProviderInputSchema()
	second := PulseQuestionProviderInputSchema()

	firstProperties := first["properties"].(map[string]interface{})
	firstQuestions := firstProperties["questions"].(map[string]interface{})
	firstQuestions["type"] = "changed"

	secondProperties := second["properties"].(map[string]interface{})
	secondQuestions := secondProperties["questions"].(map[string]interface{})
	if secondQuestions["type"] != "array" {
		t.Fatalf("question schema mutation leaked across calls: %#v", secondQuestions)
	}
}

func TestProviderToolCallNormalizeCollectionsKeepsInputObject(t *testing.T) {
	toolCall := EmptyProviderToolCall()
	if toolCall.Input == nil {
		t.Fatal("EmptyProviderToolCall must initialize input")
	}
	if len(toolCall.Input) != 0 {
		t.Fatalf("EmptyProviderToolCall input = %#v, want empty object", toolCall.Input)
	}

	normalized := (ProviderToolCall{Name: "diagnose"}).NormalizeCollections()
	if normalized.Input == nil {
		t.Fatal("NormalizeCollections must initialize input")
	}

	input := map[string]interface{}{"resource_id": "vm/100"}
	normalized = (ProviderToolCall{Name: "diagnose", Input: input}).NormalizeCollections()
	normalized.Input["resource_id"] = "vm/101"
	if input["resource_id"] != "vm/100" {
		t.Fatalf("NormalizeCollections must not alias provider input: source=%#v normalized=%#v", input, normalized.Input)
	}

	signature := json.RawMessage(`{"provider":"gemini"}`)
	normalized = (ProviderToolCall{
		Name:             "diagnose",
		ThoughtSignature: signature,
	}).NormalizeCollections()
	normalized.ThoughtSignature[0] = '['
	if string(signature) != `{"provider":"gemini"}` {
		t.Fatalf("NormalizeCollections must not alias provider thought signature: source=%s normalized=%s", signature, normalized.ThoughtSignature)
	}
}

func TestProjectProviderToolCallToToolCallNormalizesNameAndInput(t *testing.T) {
	input := map[string]interface{}{
		"resource_id": "vm/100",
		"body": map[string]any{
			"note": "maintenance",
			"tags": []any{"planned"},
		},
	}
	projected := ProjectProviderToolCallToToolCall(ProviderToolCall{
		ID:    "call-1",
		Name:  " pulse_read ",
		Input: input,
	})

	if projected.Name != "pulse_read" {
		t.Fatalf("projected name = %q, want pulse_read", projected.Name)
	}
	if !reflect.DeepEqual(projected.Arguments, input) {
		t.Fatalf("projected arguments = %#v, want %#v", projected.Arguments, input)
	}
	projected.Arguments["resource_id"] = "vm/101"
	if input["resource_id"] != "vm/100" {
		t.Fatalf("projected arguments must not alias provider input: source=%#v projected=%#v", input, projected.Arguments)
	}
	projectedBody := projected.Arguments["body"].(map[string]any)
	projectedBody["note"] = "changed"
	projectedBody["tags"].([]any)[0] = "changed"
	inputBody := input["body"].(map[string]any)
	if inputBody["note"] != "maintenance" || inputBody["tags"].([]any)[0] != "planned" {
		t.Fatalf("projected nested arguments must not alias provider input: source=%#v projected=%#v", input, projected.Arguments)
	}

	empty := ProjectProviderToolCallToToolCall(ProviderToolCall{Name: "pulse_read"})
	if empty.Arguments == nil || len(empty.Arguments) != 0 {
		t.Fatalf("nil provider input must normalize to empty arguments: %#v", empty.Arguments)
	}
}

func TestNormalizeProviderToolCallForExecutionUsesSharedToolsCallProjection(t *testing.T) {
	input := map[string]interface{}{
		"resource_id": "vm/100",
		"body": map[string]any{
			"note": "maintenance",
			"tags": []any{"planned"},
		},
	}
	signature := json.RawMessage(`{"provider":"gemini"}`)
	normalized := NormalizeProviderToolCallForExecution(ProviderToolCall{
		ID:               "call-1",
		Name:             " pulse_read ",
		Input:            input,
		ThoughtSignature: signature,
	})

	if normalized.ID != "call-1" {
		t.Fatalf("normalized id = %q, want call-1", normalized.ID)
	}
	if normalized.Name != "pulse_read" {
		t.Fatalf("normalized name = %q, want pulse_read", normalized.Name)
	}
	if !reflect.DeepEqual(normalized.Input, input) {
		t.Fatalf("normalized input = %#v, want %#v", normalized.Input, input)
	}
	if string(normalized.ThoughtSignature) != `{"provider":"gemini"}` {
		t.Fatalf("normalized thought signature = %s", normalized.ThoughtSignature)
	}

	normalized.Input["resource_id"] = "vm/101"
	if input["resource_id"] != "vm/100" {
		t.Fatalf("normalized input must not alias provider input: source=%#v normalized=%#v", input, normalized.Input)
	}
	normalizedBody := normalized.Input["body"].(map[string]any)
	normalizedBody["note"] = "changed"
	normalizedBody["tags"].([]any)[0] = "changed"
	inputBody := input["body"].(map[string]any)
	if inputBody["note"] != "maintenance" || inputBody["tags"].([]any)[0] != "planned" {
		t.Fatalf("normalized nested input must not alias provider input: source=%#v normalized=%#v", input, normalized.Input)
	}
	normalized.ThoughtSignature[0] = '['
	if string(signature) != `{"provider":"gemini"}` {
		t.Fatalf("normalized thought signature must not alias source: source=%s normalized=%s", signature, normalized.ThoughtSignature)
	}

	calls := NormalizeProviderToolCallsForExecution([]ProviderToolCall{
		{Name: " first ", Input: nil},
		{Name: " second ", Input: map[string]interface{}{"value": "ok"}},
	})
	if len(calls) != 2 || calls[0].Name != "first" || calls[1].Name != "second" {
		t.Fatalf("normalized calls = %+v", calls)
	}
	if calls[0].Input == nil || len(calls[0].Input) != 0 {
		t.Fatalf("nil provider input must normalize to empty input: %#v", calls[0].Input)
	}
}

func TestProviderToolInputParsingOwnsStreamAndFinalFallbacks(t *testing.T) {
	parsed, ok := ParseProviderToolInput(`{"resource_id":"vm/100","count":2}`)
	if !ok {
		t.Fatal("ParseProviderToolInput rejected complete JSON object")
	}
	if parsed["resource_id"] != "vm/100" || parsed["count"] != float64(2) {
		t.Fatalf("parsed input = %#v", parsed)
	}

	for _, raw := range []string{"", "   ", `{"resource_id"`, `null`, `[]`} {
		if parsed, ok := ParseProviderToolInput(raw); ok || parsed != nil {
			t.Fatalf("ParseProviderToolInput(%q) = %#v, %v; want nil,false", raw, parsed, ok)
		}
		fallback := ProviderToolInputOrRaw(raw)
		if fallback["raw"] != raw {
			t.Fatalf("ProviderToolInputOrRaw(%q) = %#v, want raw fallback", raw, fallback)
		}
	}

	final := ProviderToolInputOrRaw(`{"host":"nas01"}`)
	if final["host"] != "nas01" {
		t.Fatalf("ProviderToolInputOrRaw parsed complete JSON as %#v", final)
	}
}

func TestProviderToolResultShape(t *testing.T) {
	payload, err := json.Marshal(ProviderToolResult{
		ToolUseID: "call-1",
		Content:   "done",
		IsError:   true,
	})
	if err != nil {
		t.Fatalf("marshal provider tool result: %v", err)
	}
	if string(payload) != `{"tool_use_id":"call-1","content":"done","is_error":true}` {
		t.Fatalf("provider tool result JSON = %s", payload)
	}
}

func TestProviderToolResultContextProjectionPreservesTranscriptAndTruncatesModel(t *testing.T) {
	content := strings.Repeat("a", 12)
	projection := NewProviderToolResultContextProjection("call-1", content, false, ProviderToolResultContextOptions{
		MaxModelContentChars: 5,
	})

	if projection.Transcript.ToolUseID != "call-1" || projection.Transcript.Content != content || projection.Transcript.IsError {
		t.Fatalf("transcript result = %+v", projection.Transcript)
	}
	if projection.Model.ToolUseID != "call-1" || projection.Model.IsError {
		t.Fatalf("model result metadata = %+v", projection.Model)
	}
	if !strings.HasPrefix(projection.Model.Content, "aaaaa\n\n---\n[TRUNCATED: 7 characters cut.") {
		t.Fatalf("model content = %q, want shared truncation notice", projection.Model.Content)
	}
	if !projection.Truncation.Applied || projection.Truncation.OriginalChars != 12 || projection.Truncation.MaxChars != 5 || projection.Truncation.TruncatedChars != 7 {
		t.Fatalf("truncation = %+v", projection.Truncation)
	}

	unlimited := NewProviderToolResultContextProjection("call-2", content, true, ProviderToolResultContextOptions{})
	if unlimited.Transcript.Content != content || unlimited.Model.Content != content || !unlimited.Model.IsError || unlimited.Truncation.Applied {
		t.Fatalf("unlimited projection = %+v", unlimited)
	}
}

func TestProviderToolResultContextProjectionFromToolResultUsesSharedInterpretation(t *testing.T) {
	projection := NewProviderToolResultContextProjectionFromToolResult("call-1", ToolResult{
		Content: []ToolContent{
			NewToolTextContent("first"),
			{Type: "resource", URI: "file://ignored"},
			NewToolTextContent("second"),
		},
		IsError: true,
	}, ProviderToolResultContextOptions{MaxModelContentChars: 8})

	if projection.Transcript.Content != "first\nsecond" || !projection.Transcript.IsError {
		t.Fatalf("transcript result = %+v", projection.Transcript)
	}
	if !strings.HasPrefix(projection.Model.Content, "first\nse\n\n---\n[TRUNCATED: 4 characters cut.") || !projection.Model.IsError {
		t.Fatalf("model result = %+v", projection.Model)
	}
}

func TestProviderInputSchemaIncludesEmptyPropertiesObject(t *testing.T) {
	projected := ProviderInputSchema(InputSchema{})
	if projected["type"] != "object" {
		t.Fatalf("projected type = %v, want object", projected["type"])
	}

	props, ok := projected["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("projected properties type = %T, want map[string]interface{}", projected["properties"])
	}
	if len(props) != 0 {
		t.Fatalf("projected properties length = %d, want 0", len(props))
	}
	if _, ok := projected["required"]; ok {
		t.Fatalf("projected required present for empty schema: %#v", projected["required"])
	}
}

func TestProviderInputSchemaFromRawPreservesManifestOptionality(t *testing.T) {
	raw := StrictObjectInputSchema([]string{FindingIDArgumentName}, map[string]any{
		FindingIDArgumentName:      map[string]any{"type": "string"},
		ResolutionNoteArgumentName: map[string]any{"type": "string"},
	})

	projected := ProviderInputSchemaFromRaw(raw)
	props, ok := projected["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("projected properties type = %T, want map[string]interface{}", projected["properties"])
	}
	if _, ok := props[ResolutionNoteArgumentName]; !ok {
		t.Fatalf("projected properties missing %q: %#v", ResolutionNoteArgumentName, props)
	}
	required, ok := projected["required"].([]string)
	if !ok {
		t.Fatalf("projected required type = %T, want []string", projected["required"])
	}
	if !containsString(required, FindingIDArgumentName) {
		t.Fatalf("projected required = %v, want %q", required, FindingIDArgumentName)
	}
	if containsString(required, ResolutionNoteArgumentName) {
		t.Fatalf("projected required = %v, %q must remain optional", required, ResolutionNoteArgumentName)
	}

	required[0] = "changed"
	props[ResolutionNoteArgumentName] = map[string]interface{}{"type": "integer"}
	second := ProviderInputSchemaFromRaw(raw)
	secondRequired := second["required"].([]string)
	secondProps := second["properties"].(map[string]interface{})
	if !containsString(secondRequired, FindingIDArgumentName) {
		t.Fatalf("ProviderInputSchemaFromRaw returned aliased required slice: %v", secondRequired)
	}
	resolutionNote := secondProps[ResolutionNoteArgumentName].(map[string]interface{})
	if resolutionNote["type"] != "string" {
		t.Fatalf("ProviderInputSchemaFromRaw returned aliased properties: %#v", resolutionNote)
	}
}

func TestProviderInputSchemaReturnsIndependentSchemaValues(t *testing.T) {
	defaultValue := map[string]interface{}{
		"filters": []interface{}{"active"},
	}
	schema := InputSchema{
		Properties: map[string]PropertySchema{
			"mode": {Type: "string", Enum: []string{"summary", "detail"}, Default: defaultValue},
		},
		Required: []string{"mode"},
	}

	projected := ProviderInputSchema(schema)
	projected["required"].([]string)[0] = "changed"
	props := projected["properties"].(map[string]interface{})
	mode := props["mode"].(map[string]interface{})
	mode["enum"].([]string)[0] = "changed"
	mode["default"].(map[string]interface{})["filters"].([]interface{})[0] = "changed"

	if schema.Required[0] != "mode" {
		t.Fatalf("source required was mutated to %q", schema.Required[0])
	}
	if schema.Properties["mode"].Enum[0] != "summary" {
		t.Fatalf("source enum was mutated to %q", schema.Properties["mode"].Enum[0])
	}
	if defaultValue["filters"].([]interface{})[0] != "active" {
		t.Fatalf("source default was mutated to %#v", defaultValue)
	}
}

func TestProviderPropertySchemaReturnsIndependentDefault(t *testing.T) {
	defaultValue := map[string]interface{}{
		"labels": []interface{}{"prod"},
	}

	projected := ProviderPropertySchema(PropertySchema{
		Type:    "object",
		Default: defaultValue,
	})
	projectedDefault := projected["default"].(map[string]interface{})
	projectedDefault["labels"].([]interface{})[0] = "changed"

	if defaultValue["labels"].([]interface{})[0] != "prod" {
		t.Fatalf("provider property default aliased source: source=%#v projected=%#v", defaultValue, projected)
	}
}
