package agentcapabilities

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestIsRequestResponseCapabilitySkipsEventSubscription(t *testing.T) {
	if IsRequestResponseCapability(Capability{Name: EventSubscriptionCapabilityName}) {
		t.Fatal("subscribe_events must stay out of request/response tool projections")
	}
	if !IsRequestResponseCapability(Capability{Name: "get_resource_context"}) {
		t.Fatal("ordinary manifest capabilities must project as request/response tools")
	}
}

func TestFindCapabilityReturnsNamedCapability(t *testing.T) {
	capabilities := []Capability{
		{Name: "get_fleet_context", Path: "/api/agent/fleet-context"},
		{Name: "get_resource_context", Path: "/api/agent/resource-context/{resourceId}"},
	}

	got, ok := FindCapability(capabilities, "get_resource_context")
	if !ok {
		t.Fatal("FindCapability did not find get_resource_context")
	}
	if got.Path != "/api/agent/resource-context/{resourceId}" {
		t.Fatalf("FindCapability path = %q, want resource context path", got.Path)
	}

	if _, ok := FindCapability(capabilities, "missing"); ok {
		t.Fatal("FindCapability unexpectedly found missing capability")
	}
}

func TestFindCapabilityReturnsDetachedCapability(t *testing.T) {
	rawSchema := json.RawMessage(`{"type":"object","properties":{"mode":{"type":"string"}}}`)
	rawOutputSchema := json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`)
	wantSchema := string(rawSchema)
	wantOutputSchema := string(rawOutputSchema)
	capabilities := []Capability{
		{
			Name:         "get_resource_context",
			InputSchema:  rawSchema,
			OutputSchema: rawOutputSchema,
			ErrorCodes:   []string{"resource_not_found"},
		},
	}

	got, ok := FindCapability(capabilities, "get_resource_context")
	if !ok {
		t.Fatal("FindCapability did not find get_resource_context")
	}

	got.InputSchema[0] = '['
	got.OutputSchema[0] = '['
	got.ErrorCodes[0] = "mutated"

	if string(capabilities[0].InputSchema) != wantSchema {
		t.Fatalf("FindCapability returned aliased input schema: source=%s got=%s", capabilities[0].InputSchema, got.InputSchema)
	}
	if string(capabilities[0].OutputSchema) != wantOutputSchema {
		t.Fatalf("FindCapability returned aliased output schema: source=%s got=%s", capabilities[0].OutputSchema, got.OutputSchema)
	}
	if capabilities[0].ErrorCodes[0] != "resource_not_found" {
		t.Fatalf("FindCapability returned aliased error codes: source=%v got=%v", capabilities[0].ErrorCodes, got.ErrorCodes)
	}
}

func TestResolveRequestResponseCapabilityRejectsStreamingCapabilities(t *testing.T) {
	capabilities := []Capability{
		{Name: EventSubscriptionCapabilityName, Path: "/api/agent/events", Method: http.MethodGet},
		{Name: ResourceContextCapabilityName, Path: "/api/agent/resource-context/{resourceId}", Method: http.MethodGet},
	}
	if _, err := ResolveRequestResponseCapability(capabilities, ResourceContextCapabilityName); err != nil {
		t.Fatalf("ResolveRequestResponseCapability rejected resource context: %v", err)
	}
	if _, err := ResolveRequestResponseCapability(capabilities, EventSubscriptionCapabilityName); err == nil {
		t.Fatal("ResolveRequestResponseCapability must reject streaming subscription capability")
	} else if lookupErr, ok := err.(CapabilityLookupError); !ok || lookupErr.Name != EventSubscriptionCapabilityName {
		t.Fatalf("streaming capability error = %T %[1]v, want CapabilityLookupError for %s", err, EventSubscriptionCapabilityName)
	}
}

func TestProjectToolsProjectsRequestResponseCapabilities(t *testing.T) {
	tools := ProjectTools([]Capability{
		{
			Name:        "get_resource_context",
			Title:       "Inspect resource",
			Description: "Read focused resource context.",
			Category:    "context",
			Method:      http.MethodGet,
			Path:        "/api/agent/resource-context/{resourceId}",
			Scope:       "monitoring:read",
			ActionMode:  ActionModeRead,
			OutputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"canonicalId": { "type": "string" }
				},
				"required": ["canonicalId"],
				"additionalProperties": true
			}`),
		},
		{
			Name:        EventSubscriptionCapabilityName,
			Description: "Stream Pulse Intelligence events.",
			Method:      http.MethodGet,
			Path:        "/api/agent/events",
		},
		{
			Name:           SetOperatorStateCapabilityName,
			Title:          "Set operator state",
			Description:    "Set operator state.",
			Method:         http.MethodPut,
			Path:           OperatorStateCapabilityPath,
			Scope:          "monitoring:write",
			ActionMode:     ActionModeWrite,
			ApprovalPolicy: ApprovalPolicyScopeOnly,
		},
	})

	if len(tools) != 2 {
		t.Fatalf("ProjectTools length = %d, want 2 request/response tools", len(tools))
	}
	if tools[0].Name != "get_resource_context" {
		t.Fatalf("first projected tool = %q, want get_resource_context", tools[0].Name)
	}
	if tools[0].Title != "Inspect resource" {
		t.Fatalf("first projected tool title = %q, want manifest title", tools[0].Title)
	}
	if len(tools[0].OutputSchema) == 0 {
		t.Fatal("first projected tool must carry manifest-owned outputSchema")
	}
	if tools[1].Name != SetOperatorStateCapabilityName {
		t.Fatalf("second projected tool = %q, want %s", tools[1].Name, SetOperatorStateCapabilityName)
	}
	if strings.Contains(tools[1].Description, EventSubscriptionCapabilityName) {
		t.Fatalf("request/response projection leaked streaming capability description: %q", tools[1].Description)
	}
	if !strings.Contains(tools[1].Description, "required scope: monitoring:write") {
		t.Fatalf("projected tool description missing manifest metadata: %q", tools[1].Description)
	}
	meta, ok := tools[1].Meta[ToolMetaPulseCapabilityKey].(map[string]any)
	if !ok {
		t.Fatalf("projected tool missing structured Pulse capability metadata: %#v", tools[1].Meta)
	}
	if meta["scope"] != "monitoring:write" {
		t.Fatalf("projected metadata scope = %#v, want monitoring:write", meta["scope"])
	}
	route, _ := meta["route"].(map[string]any)
	if route["method"] != http.MethodPut || route["path"] != OperatorStateCapabilityPath {
		t.Fatalf("projected metadata route = %#v, want PUT operator-state route", route)
	}
	governance, _ := meta["governance"].(map[string]any)
	if governance["actionMode"] != string(ActionModeWrite) || governance["approvalPolicy"] != string(ApprovalPolicyScopeOnly) {
		t.Fatalf("projected metadata governance = %#v, want write/scope_only", governance)
	}
	assertToolAnnotations(t, tools[0].Annotations, true, false, true, true)
	assertToolAnnotations(t, tools[1].Annotations, false, true, false, true)

	var schema map[string]any
	if err := json.Unmarshal(tools[1].InputSchema, &schema); err != nil {
		t.Fatalf("projected tool input schema must be JSON: %v", err)
	}
	props, _ := schema["properties"].(map[string]any)
	if _, hasResourceID := props[ResourceIDArgumentName]; !hasResourceID {
		t.Fatalf("projected tool input schema missing path argument: %s", string(tools[1].InputSchema))
	}
}

func TestProjectManifestSurfaceToolsUsesPublishedSurfaceContract(t *testing.T) {
	manifest := Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		SurfaceToolContracts: []SurfaceToolContract{
			{
				SurfaceID:   SurfaceIDPulseMCP,
				ToolSource:  SurfaceToolSourceCapabilityManifest,
				ToolNames:   []string{SetOperatorStateCapabilityName, ResourceContextCapabilityName, EventSubscriptionCapabilityName, "missing_tool", SetOperatorStateCapabilityName},
				Affordances: DefaultSurfaceAffordancesForID(SurfaceIDPulseMCP),
			},
		},
		Capabilities: []Capability{
			{Name: ResourceContextCapabilityName, Path: ResourceContextCapabilityPath, Method: http.MethodGet, Description: "depth"},
			{Name: SetOperatorStateCapabilityName, Path: OperatorStateCapabilityPath, Method: http.MethodPut, Description: "intent"},
			{Name: EventSubscriptionCapabilityName, Path: "/api/agent/events", Method: http.MethodGet, Description: "stream"},
			{Name: FleetContextCapabilityName, Path: FleetContextCapabilityPath, Method: http.MethodGet, Description: "fleet"},
		},
	}

	contract, ok := FindManifestSurfaceToolContract(manifest, SurfaceIDPulseMCP)
	if !ok {
		t.Fatal("published Pulse MCP surface contract was not found")
	}
	if len(contract.ToolNames) != 5 || contract.ToolNames[0] != SetOperatorStateCapabilityName {
		t.Fatalf("published surface contract names = %#v", contract.ToolNames)
	}

	capabilities := ManifestSurfaceToolCapabilities(manifest, SurfaceIDPulseMCP)
	if len(capabilities) != 2 {
		t.Fatalf("surface capabilities = %#v, want two request/response capabilities from published contract", capabilities)
	}
	if capabilities[0].Name != SetOperatorStateCapabilityName || capabilities[1].Name != ResourceContextCapabilityName {
		t.Fatalf("surface capability order = %#v, want published contract order", []string{capabilities[0].Name, capabilities[1].Name})
	}

	capabilities[0].Name = "mutated"
	second := ManifestSurfaceToolCapabilities(manifest, SurfaceIDPulseMCP)
	if second[0].Name != SetOperatorStateCapabilityName {
		t.Fatalf("surface capability projection returned aliased capabilities: %#v", second)
	}

	tools := ProjectManifestSurfaceTools(manifest, SurfaceIDPulseMCP)
	if len(tools) != 2 {
		t.Fatalf("surface tools = %#v, want two projected tools from published contract", tools)
	}
	if tools[0].Name != SetOperatorStateCapabilityName || tools[1].Name != ResourceContextCapabilityName {
		t.Fatalf("surface tool order = %#v, want published contract order", []string{tools[0].Name, tools[1].Name})
	}
}

func TestProjectManifestSurfaceToolsUsesNormalizedContractAndToolAffordance(t *testing.T) {
	manifest := Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		SurfaceToolContracts: []SurfaceToolContract{
			{
				SurfaceID:       SurfaceIDPulseMCP,
				ToolSource:      SurfaceToolSourceCapabilityManifest,
				CapabilityNames: []string{ResourceContextCapabilityName},
			},
		},
		Capabilities: []Capability{
			{Name: ResourceContextCapabilityName, Path: ResourceContextCapabilityPath, Method: http.MethodGet, Description: "depth"},
		},
	}

	capabilities := ManifestSurfaceToolCapabilities(manifest, SurfaceIDPulseMCP)
	if len(capabilities) != 1 || capabilities[0].Name != ResourceContextCapabilityName {
		t.Fatalf("surface capabilities = %#v, want normalized capabilityNames fallback", capabilities)
	}

	tools := ProjectManifestSurfaceTools(manifest, SurfaceIDPulseMCP)
	if len(tools) != 1 || tools[0].Name != ResourceContextCapabilityName {
		t.Fatalf("surface tools = %#v, want normalized capabilityNames fallback", tools)
	}

	disabledManifest := manifest
	disabledManifest.SurfaceContract = CloneSurfaceContract(manifest.SurfaceContract)
	for i := range disabledManifest.SurfaceContract.OperatorSurfaces {
		if disabledManifest.SurfaceContract.OperatorSurfaces[i].ID == SurfaceIDPulseMCP {
			disabledManifest.SurfaceContract.OperatorSurfaces[i].Affordances = SurfaceAffordanceContract{
				Resources:          true,
				Prompts:            true,
				CapabilityMetadata: true,
			}
		}
	}
	disabledManifest.SurfaceToolContracts = CloneSurfaceToolContracts(manifest.SurfaceToolContracts)
	disabledManifest.SurfaceToolContracts[0].Affordances = DefaultSurfaceAffordancesForID(SurfaceIDPulseMCP)

	if got := ManifestSurfaceToolCapabilities(disabledManifest, SurfaceIDPulseMCP); len(got) != 0 {
		t.Fatalf("surface with tools affordance disabled returned capabilities: %#v", got)
	}
	if got := ProjectManifestSurfaceTools(disabledManifest, SurfaceIDPulseMCP); len(got) != 0 {
		t.Fatalf("surface with tools affordance disabled returned tools: %#v", got)
	}
}

func TestProjectManifestSurfaceToolsRequiresPublishedPulseMCPContract(t *testing.T) {
	manifest := Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		Capabilities: []Capability{
			{Name: ResourceContextCapabilityName, Path: ResourceContextCapabilityPath, Method: http.MethodGet, Description: "depth"},
			{Name: EventSubscriptionCapabilityName, Path: "/api/agent/events", Method: http.MethodGet, Description: "stream"},
		},
	}

	if tools := ProjectManifestSurfaceTools(manifest, SurfaceIDPulseMCP); len(tools) != 0 {
		t.Fatalf("missing Pulse MCP surface tool contract exposed tools: %#v", tools)
	}
	if capabilities := ManifestSurfaceToolCapabilities(manifest, SurfaceIDPulseMCP); len(capabilities) != 0 {
		t.Fatalf("missing Pulse MCP surface tool contract exposed capabilities: %#v", capabilities)
	}

	if got := ProjectManifestSurfaceTools(Manifest{}, "custom_external_agent"); len(got) != 0 {
		t.Fatalf("missing custom external surface fallback = %#v, want no tools", got)
	}
}

func TestProjectToolsReturnsDetachedInputAndOutputSchemas(t *testing.T) {
	rawSchema := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	rawOutputSchema := json.RawMessage(`{"type":"object","properties":{"accepted":{"type":"boolean"}}}`)
	wantSchema := string(rawSchema)
	wantOutputSchema := string(rawOutputSchema)
	capabilities := []Capability{
		{
			Name:         DecideActionCapabilityName,
			Description:  "Decide an action.",
			Method:       http.MethodPost,
			Path:         ActionDecisionCapabilityPath,
			InputSchema:  rawSchema,
			OutputSchema: rawOutputSchema,
		},
	}

	tools := ProjectTools(capabilities)
	if len(tools) != 1 {
		t.Fatalf("ProjectTools length = %d, want 1", len(tools))
	}

	tools[0].InputSchema[0] = '['
	tools[0].OutputSchema[0] = '['
	if string(capabilities[0].InputSchema) != wantSchema {
		t.Fatalf("ProjectTools returned aliased input schema: source=%s projected=%s", capabilities[0].InputSchema, tools[0].InputSchema)
	}
	if string(capabilities[0].OutputSchema) != wantOutputSchema {
		t.Fatalf("ProjectTools returned aliased output schema: source=%s projected=%s", capabilities[0].OutputSchema, tools[0].OutputSchema)
	}

	second := ProjectTools(capabilities)
	if string(second[0].InputSchema) != wantSchema {
		t.Fatalf("ProjectTools mutation leaked into later projection: %s", second[0].InputSchema)
	}
	if string(second[0].OutputSchema) != wantOutputSchema {
		t.Fatalf("ProjectTools output schema mutation leaked into later projection: %s", second[0].OutputSchema)
	}
}

func TestProjectToolsReturnsDetachedStructuredMetadata(t *testing.T) {
	capabilities := []Capability{
		{
			Name:           PlanActionCapabilityName,
			Description:    "Plan an action.",
			Category:       "action",
			Method:         http.MethodPost,
			Path:           PlanActionCapabilityPath,
			Scope:          "ai:execute",
			ActionMode:     ActionModeWrite,
			ApprovalPolicy: ApprovalPolicyActionPlan,
			ResponseShape:  "ActionPlan",
			ErrorCodes:     []string{"invalid_action_request"},
		},
	}

	tools := ProjectTools(capabilities)
	if len(tools) != 1 {
		t.Fatalf("ProjectTools length = %d, want 1", len(tools))
	}
	meta, ok := tools[0].Meta[ToolMetaPulseCapabilityKey].(map[string]any)
	if !ok {
		t.Fatalf("projected metadata missing pulse capability object: %#v", tools[0].Meta)
	}
	errors, _ := meta["errorCodes"].([]string)
	if len(errors) != 1 || errors[0] != "invalid_action_request" {
		t.Fatalf("metadata errorCodes = %#v, want invalid_action_request", meta["errorCodes"])
	}

	errors[0] = "mutated"
	route, _ := meta["route"].(map[string]any)
	route["path"] = "/mutated"

	if capabilities[0].ErrorCodes[0] != "invalid_action_request" {
		t.Fatalf("projected metadata aliases manifest error codes: %v", capabilities[0].ErrorCodes)
	}
	second := ProjectTools(capabilities)
	secondMeta, _ := second[0].Meta[ToolMetaPulseCapabilityKey].(map[string]any)
	secondErrors, _ := secondMeta["errorCodes"].([]string)
	secondRoute, _ := secondMeta["route"].(map[string]any)
	if secondErrors[0] != "invalid_action_request" {
		t.Fatalf("metadata mutation leaked into later projection: %#v", secondErrors)
	}
	if secondRoute["path"] != "/api/actions/plan" {
		t.Fatalf("metadata route mutation leaked into later projection: %#v", secondRoute)
	}
}

func TestToolAnnotationsProjectsCapabilityActionModes(t *testing.T) {
	tests := []struct {
		name        string
		cap         Capability
		readOnly    bool
		destructive bool
		idempotent  bool
		openWorld   bool
	}{
		{
			name: "read capability",
			cap: Capability{
				Method:     http.MethodGet,
				ActionMode: ActionModeRead,
			},
			readOnly:    true,
			destructive: false,
			idempotent:  true,
			openWorld:   true,
		},
		{
			name: "mixed scan capability",
			cap: Capability{
				Method:     http.MethodPost,
				ActionMode: ActionModeMixed,
			},
			readOnly:    false,
			destructive: false,
			idempotent:  false,
			openWorld:   true,
		},
		{
			name: "write capability",
			cap: Capability{
				Method:     http.MethodPost,
				ActionMode: ActionModeWrite,
			},
			readOnly:    false,
			destructive: true,
			idempotent:  false,
			openWorld:   true,
		},
		{
			name: "missing mode defaults GET to read",
			cap: Capability{
				Method: http.MethodGet,
			},
			readOnly:    true,
			destructive: false,
			idempotent:  true,
			openWorld:   true,
		},
		{
			name: "missing mode defaults non-GET to write",
			cap: Capability{
				Method: http.MethodPut,
			},
			readOnly:    false,
			destructive: true,
			idempotent:  false,
			openWorld:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertToolAnnotations(t, ToolAnnotations(tc.cap), tc.readOnly, tc.destructive, tc.idempotent, tc.openWorld)
		})
	}
}

func TestCapabilityTitleFallsBackToProgrammaticName(t *testing.T) {
	if got := CapabilityTitle(Capability{Name: "refresh_node_cluster_membership"}); got != "Refresh node cluster membership" {
		t.Fatalf("fallback capability title = %q", got)
	}
}

func TestProjectToolSerializesFalseAnnotationHints(t *testing.T) {
	tool, ok := ProjectTool(Capability{
		Name:       SetOperatorStateCapabilityName,
		Method:     http.MethodPut,
		Path:       OperatorStateCapabilityPath,
		ActionMode: ActionModeWrite,
	})
	if !ok {
		t.Fatal("write capability did not project")
	}
	body, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal projected tool: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		`"title":"Set operator state"`,
		`"annotations":{`,
		`"readOnlyHint":false`,
		`"destructiveHint":true`,
		`"idempotentHint":false`,
		`"openWorldHint":true`,
		`"_meta":{`,
		`"pulse.capability":{`,
		`"actionMode":"write"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("projected tool JSON missing %s: %s", want, text)
		}
	}
}

func TestToolDescriptionProjectsCapabilityMetadata(t *testing.T) {
	desc := ToolDescription(Capability{
		Name:             PlanActionCapabilityName,
		Description:      "Plan an action against a resource.",
		Category:         "action",
		Method:           http.MethodPost,
		Path:             PlanActionCapabilityPath,
		Scope:            "ai:execute",
		ActionMode:       ActionModeWrite,
		ApprovalPolicy:   ApprovalPolicyActionPlan,
		RequestBodyShape: "ActionRequest",
		ResponseShape:    "ActionPlan",
		ErrorCodes:       []string{"invalid_action_request", "resource_not_found"},
	})

	for _, want := range []string{
		"Plan an action against a resource.",
		"Pulse capability metadata:",
		"category: action",
		"route: POST /api/actions/plan",
		"required scope: ai:execute",
		"action mode: write",
		"approval policy: action_plan",
		"request body: ActionRequest",
		"response: ActionPlan",
		"stable error codes: invalid_action_request, resource_not_found",
	} {
		if !strings.Contains(desc, want) {
			t.Fatalf("ToolDescription missing %q in %q", want, desc)
		}
	}
}

func TestToolMetaProjectsStructuredPulseCapabilityMetadata(t *testing.T) {
	meta := ToolMeta(Capability{
		Name:             PlanActionCapabilityName,
		Description:      "Plan an action against a resource.",
		Category:         "action",
		Method:           http.MethodPost,
		Path:             PlanActionCapabilityPath,
		Scope:            "ai:execute",
		ActionMode:       ActionModeWrite,
		ApprovalPolicy:   ApprovalPolicyActionPlan,
		RequestBodyShape: "ActionRequest",
		ResponseShape:    "ActionPlan",
		ErrorCodes:       []string{"invalid_action_request", "resource_not_found"},
	})

	capability, ok := meta[ToolMetaPulseCapabilityKey].(map[string]any)
	if !ok {
		t.Fatalf("ToolMeta missing %s object: %#v", ToolMetaPulseCapabilityKey, meta)
	}
	for key, want := range map[string]any{
		"name":             PlanActionCapabilityName,
		"category":         "action",
		"scope":            "ai:execute",
		"requestBodyShape": "ActionRequest",
		"responseShape":    "ActionPlan",
	} {
		if got := capability[key]; got != want {
			t.Fatalf("ToolMeta[%s] = %#v, want %#v", key, got, want)
		}
	}
	route, _ := capability["route"].(map[string]any)
	if route["method"] != http.MethodPost || route["path"] != "/api/actions/plan" {
		t.Fatalf("ToolMeta route = %#v, want POST /api/actions/plan", route)
	}
	governance, _ := capability["governance"].(map[string]any)
	if governance["actionMode"] != "write" || governance["approvalPolicy"] != "action_plan" {
		t.Fatalf("ToolMeta governance = %#v, want write/action_plan", governance)
	}
	errors, _ := capability["errorCodes"].([]string)
	if len(errors) != 2 || errors[0] != "invalid_action_request" || errors[1] != "resource_not_found" {
		t.Fatalf("ToolMeta errorCodes = %#v, want stable manifest errors", errors)
	}
}

func TestToolDescriptionProjectsNormalizedCapabilityGovernance(t *testing.T) {
	desc := ToolDescription(Capability{
		Name:        "legacy_write",
		Description: "Legacy write capability.",
		Method:      http.MethodPost,
		Path:        "/api/legacy/write",
	})

	for _, want := range []string{
		"Legacy write capability.",
		"action mode: write",
		"approval policy: scope_only",
	} {
		if !strings.Contains(desc, want) {
			t.Fatalf("ToolDescription missing normalized governance %q in %q", want, desc)
		}
	}
}

func TestToolInputSchemaDerivesPermissiveFallback(t *testing.T) {
	raw := ToolInputSchema(Capability{
		Path:             OperatorStateCapabilityPath,
		Method:           http.MethodPut,
		RequestBodyShape: "ResourceOperatorStateInput",
	})

	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schema["additionalProperties"] != true {
		t.Fatalf("fallback schema additionalProperties = %v, want true", schema["additionalProperties"])
	}
	props, _ := schema["properties"].(map[string]any)
	if _, hasResourceID := props[ResourceIDArgumentName]; !hasResourceID {
		t.Fatalf("fallback schema must include path argument resourceId: %v", props)
	}
	body, _ := props["body"].(map[string]any)
	if !strings.Contains(body["description"].(string), "ResourceOperatorStateInput") {
		t.Fatalf("fallback body description must carry request-body shape hint: %v", body)
	}
	required, _ := schema["required"].([]any)
	if len(required) != 1 || required[0] != ResourceIDArgumentName {
		t.Fatalf("fallback schema required = %v, want [resourceId]", required)
	}
}

func TestToolInputSchemaPreservesManifestSchema(t *testing.T) {
	rawSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"actionId": { "type": "string" },
			"outcome": { "enum": ["approved", "rejected"] }
		},
		"required": ["actionId", "outcome"],
		"additionalProperties": false
	}`)
	wantSchema := string(rawSchema)

	got := ToolInputSchema(Capability{
		Name:        DecideActionCapabilityName,
		Path:        ActionDecisionCapabilityPath,
		Method:      http.MethodPost,
		InputSchema: rawSchema,
	})

	if string(got) != wantSchema {
		t.Fatalf("manifest-owned schema must be forwarded verbatim, got %s want %s", got, wantSchema)
	}

	got[0] = '['
	if string(rawSchema) != wantSchema {
		t.Fatalf("ToolInputSchema must not alias manifest-owned schema: source=%s got=%s", rawSchema, got)
	}
}

func TestToolOutputSchemaPreservesManifestSchemaWithoutFallback(t *testing.T) {
	rawSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"accepted": { "type": "boolean" }
		},
		"required": ["accepted"],
		"additionalProperties": false
	}`)
	wantSchema := string(rawSchema)

	got := ToolOutputSchema(Capability{
		Name:          "accept_action",
		ResponseShape: "ActionDecisionResponse",
		OutputSchema:  rawSchema,
	})

	if string(got) != wantSchema {
		t.Fatalf("manifest-owned output schema must be forwarded verbatim, got %s want %s", got, wantSchema)
	}

	got[0] = '['
	if string(rawSchema) != wantSchema {
		t.Fatalf("ToolOutputSchema must not alias manifest-owned schema: source=%s got=%s", rawSchema, got)
	}

	fallback := ToolOutputSchema(Capability{Name: "legacy_shape", ResponseShape: "LegacyResponse"})
	if fallback != nil {
		t.Fatalf("ToolOutputSchema must not derive a schema from responseShape; got %s", string(fallback))
	}
}

func assertToolAnnotations(t *testing.T, annotations *ToolBehaviorHints, readOnly, destructive, idempotent, openWorld bool) {
	t.Helper()
	if annotations == nil {
		t.Fatal("annotations are nil")
	}
	assertBoolRef(t, "readOnlyHint", annotations.ReadOnlyHint, readOnly)
	assertBoolRef(t, "destructiveHint", annotations.DestructiveHint, destructive)
	assertBoolRef(t, "idempotentHint", annotations.IdempotentHint, idempotent)
	assertBoolRef(t, "openWorldHint", annotations.OpenWorldHint, openWorld)
}

func assertBoolRef(t *testing.T, name string, got *bool, want bool) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s is nil, want %v", name, want)
	}
	if *got != want {
		t.Fatalf("%s = %v, want %v", name, *got, want)
	}
}

func TestSubstitutePathParametersEscapesAsSinglePathSegments(t *testing.T) {
	got, err := SubstitutePathParameters(
		"/api/config/nodes/{nodeId}/test/{resourceId}",
		map[string]any{
			"nodeId":     "pve/lab node",
			"resourceId": "vm:101",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/api/config/nodes/pve%2Flab%20node/test/vm%3A101" {
		t.Fatalf("got %q, want path params escaped as single segments", got)
	}
}

func TestPathParameterSetNamesDeclaredPathArgs(t *testing.T) {
	got := PathParameterSet("/api/resources/{resourceId}/actions/{actionId}")
	for _, want := range []string{ResourceIDArgumentName, ActionIDArgumentName} {
		if !got[want] {
			t.Fatalf("PathParameterSet missing %q in %v", want, got)
		}
	}
}

func TestProjectCapabilityCallProjectsPathAndNestedBody(t *testing.T) {
	got, err := ProjectCapabilityCall(Capability{
		Path:   OperatorStateCapabilityPath,
		Method: http.MethodPut,
	}, map[string]any{
		ResourceIDArgumentName: "vm:101",
		"body": map[string]any{
			"intentionallyOffline": true,
			"note":                 "maintenance",
		},
	})
	if err != nil {
		t.Fatalf("ProjectCapabilityCall: %v", err)
	}
	if got.Path != "/api/resources/vm%3A101/operator-state" {
		t.Fatalf("projected path = %q, want escaped resource id", got.Path)
	}
	if !got.HasBody {
		t.Fatal("PUT capability must project a JSON body")
	}
	var body map[string]any
	if err := json.Unmarshal(got.Body, &body); err != nil {
		t.Fatalf("projected body must be JSON: %v", err)
	}
	if body["intentionallyOffline"] != true || body["note"] != "maintenance" {
		t.Fatalf("nested body did not round-trip: %v", body)
	}
}

func TestProjectCapabilityCallTopLevelArgsBecomeBodyWithoutPathArgs(t *testing.T) {
	got, err := ProjectCapabilityCall(Capability{
		Path:   OperatorStateCapabilityPath,
		Method: http.MethodPut,
	}, map[string]any{
		ResourceIDArgumentName: "vm:101",
		"intentionallyOffline": true,
	})
	if err != nil {
		t.Fatalf("ProjectCapabilityCall: %v", err)
	}
	if got.Path != "/api/resources/vm%3A101/operator-state" {
		t.Fatalf("projected path = %q, want escaped resource id", got.Path)
	}
	var body map[string]any
	if err := json.Unmarshal(got.Body, &body); err != nil {
		t.Fatalf("projected body must be JSON: %v", err)
	}
	if _, hasPathArg := body[ResourceIDArgumentName]; hasPathArg {
		t.Fatalf("path argument must not be duplicated into request body: %v", body)
	}
	if body["intentionallyOffline"] != true {
		t.Fatalf("non-placeholder top-level argument missing from body: %v", body)
	}
}

func TestProjectCapabilityCallDoesNotLeakInternalArgumentsIntoBody(t *testing.T) {
	got, err := ProjectCapabilityCall(Capability{
		Path:   OperatorStateCapabilityPath,
		Method: http.MethodPut,
	}, WithApprovalArgument(map[string]any{
		ResourceIDArgumentName: "vm:101",
		"intentionallyOffline": true,
	}, "approval-1"))
	if err != nil {
		t.Fatalf("ProjectCapabilityCall: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(got.Body, &body); err != nil {
		t.Fatalf("projected body must be JSON: %v", err)
	}
	if _, leaked := body[ApprovalArgumentKey]; leaked {
		t.Fatalf("internal approval argument leaked into public request body: %v", body)
	}
	if body["intentionallyOffline"] != true {
		t.Fatalf("public top-level argument missing from body: %v", body)
	}
}

func TestProjectCapabilityCallReadCapabilityHasNoBody(t *testing.T) {
	got, err := ProjectCapabilityCall(Capability{
		Path:   ResourceContextCapabilityPath,
		Method: http.MethodGet,
	}, map[string]any{ResourceIDArgumentName: "vm:101"})
	if err != nil {
		t.Fatalf("ProjectCapabilityCall: %v", err)
	}
	if got.Path != "/api/agent/resource-context/vm%3A101" {
		t.Fatalf("projected path = %q, want escaped resource id", got.Path)
	}
	if got.HasBody || len(got.Body) != 0 {
		t.Fatalf("GET capability must not project body, got hasBody=%v body=%s", got.HasBody, got.Body)
	}
}
