package agentcapabilities

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestCanonicalManifestOwnsAgentSurface(t *testing.T) {
	manifest := CanonicalManifest()
	if manifest.Version != "v1" {
		t.Fatalf("Version = %q, want v1", manifest.Version)
	}
	if len(manifest.Capabilities) == 0 {
		t.Fatal("CanonicalManifest must declare agent capabilities")
	}

	for _, name := range []string{
		FleetContextCapabilityName,
		ResourceContextCapabilityName,
		ListResourceCapabilitiesCapabilityName,
		OperationsLoopStatusCapabilityName,
		SetOperatorStateCapabilityName,
		ListFindingsCapabilityName,
		PlanActionCapabilityName,
		ExecuteActionCapabilityName,
	} {
		if _, ok := FindCapability(manifest.Capabilities, name); !ok {
			t.Fatalf("CanonicalManifest missing %q", name)
		}
	}
}

func TestCanonicalManifestPinsPatrolControlStatusCapability(t *testing.T) {
	if PatrolControlStatusCapabilityName != "get_patrol_control_status" {
		t.Fatalf("PatrolControlStatusCapabilityName = %q, want get_patrol_control_status", PatrolControlStatusCapabilityName)
	}
	if PatrolControlStatusCapabilityPath != "/api/agent/patrol-control/status" {
		t.Fatalf("PatrolControlStatusCapabilityPath = %q, want /api/agent/patrol-control/status", PatrolControlStatusCapabilityPath)
	}
	if OperationsLoopStatusCapabilityName != PatrolControlStatusCapabilityName {
		t.Fatalf("OperationsLoopStatusCapabilityName must remain a source-compatible alias for %q", PatrolControlStatusCapabilityName)
	}
	if OperationsLoopStatusCapabilityPath != PatrolControlStatusCapabilityPath {
		t.Fatalf("OperationsLoopStatusCapabilityPath must remain a source-compatible alias for %q", PatrolControlStatusCapabilityPath)
	}
	if OperationsLoopStatusCompatibilityPath != "/api/agent/operations-loop/status" {
		t.Fatalf("OperationsLoopStatusCompatibilityPath = %q, want /api/agent/operations-loop/status", OperationsLoopStatusCompatibilityPath)
	}

	manifest := CanonicalManifest()
	cap, ok := FindCapability(manifest.Capabilities, PatrolControlStatusCapabilityName)
	if !ok {
		t.Fatalf("CanonicalManifest missing %q", PatrolControlStatusCapabilityName)
	}
	if cap.Path != PatrolControlStatusCapabilityPath {
		t.Fatalf("%s path = %q, want %q", PatrolControlStatusCapabilityName, cap.Path, PatrolControlStatusCapabilityPath)
	}
	if _, ok := FindCapability(manifest.Capabilities, "get_operations_loop_status"); ok {
		t.Fatal("CanonicalManifest must not publish legacy get_operations_loop_status as a primary capability")
	}
}

func TestCanonicalManifestUsesSharedResourceContextAddressing(t *testing.T) {
	if ResourceIDArgumentName != "resourceId" {
		t.Fatalf("ResourceIDArgumentName = %q, want resourceId", ResourceIDArgumentName)
	}

	manifest := CanonicalManifest()
	for name, path := range map[string]string{
		FleetContextCapabilityName:             FleetContextCapabilityPath,
		ResourceContextCapabilityName:          ResourceContextCapabilityPath,
		ListResourceCapabilitiesCapabilityName: ListResourceCapabilitiesCapabilityPath,
		OperationsLoopStatusCapabilityName:     OperationsLoopStatusCapabilityPath,
		GetOperatorStateCapabilityName:         OperatorStateCapabilityPath,
		SetOperatorStateCapabilityName:         OperatorStateCapabilityPath,
		ClearOperatorStateCapabilityName:       OperatorStateCapabilityPath,
	} {
		cap, ok := FindCapability(manifest.Capabilities, name)
		if !ok {
			t.Fatalf("CanonicalManifest missing %q", name)
		}
		if cap.Path != path {
			t.Fatalf("%s path = %q, want %q", name, cap.Path, path)
		}
	}

	for _, name := range []string{GetOperatorStateCapabilityName, SetOperatorStateCapabilityName, ClearOperatorStateCapabilityName, PlanActionCapabilityName} {
		cap, ok := FindCapability(manifest.Capabilities, name)
		if !ok {
			t.Fatalf("CanonicalManifest missing %q", name)
		}
		var schema map[string]any
		if err := json.Unmarshal(ToolInputSchema(cap), &schema); err != nil {
			t.Fatalf("%s projected inputSchema must be valid JSON: %v", name, err)
		}
		properties, _ := schema["properties"].(map[string]any)
		if _, ok := properties[ResourceIDArgumentName]; !ok {
			t.Fatalf("%s projected inputSchema properties missing %q: %v", name, ResourceIDArgumentName, properties)
		}
		if !schemaRequiredContains(schema, ResourceIDArgumentName) {
			t.Fatalf("%s projected inputSchema required = %v, want %q", name, schema["required"], ResourceIDArgumentName)
		}
	}
}

func TestCanonicalManifestUsesSharedOperatorStateVocabulary(t *testing.T) {
	for name, want := range map[string]string{
		GetOperatorStateCapabilityName:   "get_operator_state",
		SetOperatorStateCapabilityName:   "set_operator_state",
		ClearOperatorStateCapabilityName: "clear_operator_state",
	} {
		if name != want {
			t.Fatalf("operator-state capability constant = %q, want %q", name, want)
		}
	}

	manifest := CanonicalManifest()
	for _, name := range []string{
		GetOperatorStateCapabilityName,
		SetOperatorStateCapabilityName,
		ClearOperatorStateCapabilityName,
	} {
		cap, ok := FindCapability(manifest.Capabilities, name)
		if !ok {
			t.Fatalf("CanonicalManifest missing %q", name)
		}
		if cap.Path != OperatorStateCapabilityPath {
			t.Fatalf("%s path = %q, want %q", name, cap.Path, OperatorStateCapabilityPath)
		}
	}
}

func TestCanonicalManifestUsesSharedFindingVocabulary(t *testing.T) {
	if FindingIDArgumentName != "finding_id" {
		t.Fatalf("FindingIDArgumentName = %q, want finding_id", FindingIDArgumentName)
	}
	if ResolutionNoteArgumentName != "resolution_note" {
		t.Fatalf("ResolutionNoteArgumentName = %q, want resolution_note", ResolutionNoteArgumentName)
	}
	if NoteArgumentName != "note" {
		t.Fatalf("NoteArgumentName = %q, want note", NoteArgumentName)
	}

	manifest := CanonicalManifest()
	for _, name := range []string{
		ListFindingsCapabilityName,
		AcknowledgeFindingCapabilityName,
		SnoozeFindingCapabilityName,
		DismissFindingCapabilityName,
		ResolveFindingCapabilityName,
	} {
		if _, ok := FindCapability(manifest.Capabilities, name); !ok {
			t.Fatalf("CanonicalManifest missing %q", name)
		}
	}

	for _, name := range []string{
		AcknowledgeFindingCapabilityName,
		SnoozeFindingCapabilityName,
		DismissFindingCapabilityName,
		ResolveFindingCapabilityName,
	} {
		cap, ok := FindCapability(manifest.Capabilities, name)
		if !ok {
			t.Fatalf("CanonicalManifest missing %q", name)
		}
		var schema map[string]any
		if err := json.Unmarshal(cap.InputSchema, &schema); err != nil {
			t.Fatalf("%s inputSchema must be valid JSON: %v", name, err)
		}
		properties, _ := schema["properties"].(map[string]any)
		if _, ok := properties[FindingIDArgumentName]; !ok {
			t.Fatalf("%s inputSchema properties missing %q: %v", name, FindingIDArgumentName, properties)
		}
		if !schemaRequiredContains(schema, FindingIDArgumentName) {
			t.Fatalf("%s inputSchema required = %v, want %q", name, schema["required"], FindingIDArgumentName)
		}
	}

	resolve, ok := FindCapability(manifest.Capabilities, ResolveFindingCapabilityName)
	if !ok {
		t.Fatalf("CanonicalManifest missing %q", ResolveFindingCapabilityName)
	}
	var resolveSchema map[string]any
	if err := json.Unmarshal(resolve.InputSchema, &resolveSchema); err != nil {
		t.Fatalf("%s inputSchema must be valid JSON: %v", ResolveFindingCapabilityName, err)
	}
	resolveProps, _ := resolveSchema["properties"].(map[string]any)
	if _, ok := resolveProps[ResolutionNoteArgumentName]; !ok {
		t.Fatalf("%s inputSchema properties missing %q: %v", ResolveFindingCapabilityName, ResolutionNoteArgumentName, resolveProps)
	}
	if schemaRequiredContains(resolveSchema, ResolutionNoteArgumentName) {
		t.Fatalf("%s inputSchema required = %v, %q must remain optional", ResolveFindingCapabilityName, resolveSchema["required"], ResolutionNoteArgumentName)
	}

	dismiss, ok := FindCapability(manifest.Capabilities, DismissFindingCapabilityName)
	if !ok {
		t.Fatalf("CanonicalManifest missing %q", DismissFindingCapabilityName)
	}
	var dismissSchema map[string]any
	if err := json.Unmarshal(dismiss.InputSchema, &dismissSchema); err != nil {
		t.Fatalf("%s inputSchema must be valid JSON: %v", DismissFindingCapabilityName, err)
	}
	dismissProps, _ := dismissSchema["properties"].(map[string]any)
	if _, ok := dismissProps[NoteArgumentName]; !ok {
		t.Fatalf("%s inputSchema properties missing %q: %v", DismissFindingCapabilityName, NoteArgumentName, dismissProps)
	}
	if schemaRequiredContains(dismissSchema, NoteArgumentName) {
		t.Fatalf("%s inputSchema required = %v, %q must remain optional", DismissFindingCapabilityName, dismissSchema["required"], NoteArgumentName)
	}
}

func TestCanonicalManifestUsesSharedActionVocabulary(t *testing.T) {
	for name, want := range map[string]string{
		RequestIDArgumentName:      "requestId",
		ActionIDArgumentName:       "actionId",
		CapabilityNameArgumentName: "capabilityName",
		ReasonArgumentName:         "reason",
		RequestedByArgumentName:    "requestedBy",
		OutcomeArgumentName:        "outcome",
	} {
		if name != want {
			t.Fatalf("action argument constant = %q, want %q", name, want)
		}
	}

	manifest := CanonicalManifest()
	for name, path := range map[string]string{
		PlanActionCapabilityName:    PlanActionCapabilityPath,
		DecideActionCapabilityName:  ActionDecisionCapabilityPath,
		ExecuteActionCapabilityName: ActionExecutionCapabilityPath,
	} {
		cap, ok := FindCapability(manifest.Capabilities, name)
		if !ok {
			t.Fatalf("CanonicalManifest missing %q", name)
		}
		if cap.Path != path {
			t.Fatalf("%s path = %q, want %q", name, cap.Path, path)
		}
	}

	for name, required := range map[string][]string{
		PlanActionCapabilityName:    {RequestIDArgumentName, ResourceIDArgumentName, CapabilityNameArgumentName, ReasonArgumentName, RequestedByArgumentName},
		DecideActionCapabilityName:  {ActionIDArgumentName, OutcomeArgumentName},
		ExecuteActionCapabilityName: {ActionIDArgumentName},
	} {
		cap, ok := FindCapability(manifest.Capabilities, name)
		if !ok {
			t.Fatalf("CanonicalManifest missing %q", name)
		}
		var schema map[string]any
		if err := json.Unmarshal(cap.InputSchema, &schema); err != nil {
			t.Fatalf("%s inputSchema must be valid JSON: %v", name, err)
		}
		properties, _ := schema["properties"].(map[string]any)
		for _, field := range required {
			if _, ok := properties[field]; !ok {
				t.Fatalf("%s inputSchema properties missing %q: %v", name, field, properties)
			}
			if !schemaRequiredContains(schema, field) {
				t.Fatalf("%s inputSchema required = %v, want %q", name, schema["required"], field)
			}
		}
	}
}

func TestCanonicalManifestPinsPulseIntelligenceSurfaceContract(t *testing.T) {
	manifest := CanonicalManifest()
	contract := manifest.SurfaceContract

	if contract.Core.ID != "pulse_intelligence_core" || contract.Core.Label != "Pulse Intelligence Core" {
		t.Fatalf("manifest core contract = %+v", contract.Core)
	}
	if !strings.Contains(contract.Core.Description, "Canonical context, governed actions, safety gates, approval state, action audit, and verification") {
		t.Fatalf("manifest core description does not pin shared core ownership: %q", contract.Core.Description)
	}
	if contract.ProactiveEngine.ID != "pulse_patrol" || contract.ProactiveEngine.Label != "Pulse Patrol" {
		t.Fatalf("manifest Patrol operator component = %+v", contract.ProactiveEngine)
	}
	if !strings.Contains(contract.ProactiveEngine.Description, "first-party operations surface") ||
		!strings.Contains(contract.ProactiveEngine.Description, "Patrol mode") ||
		!strings.Contains(contract.ProactiveEngine.Description, "verifies outcomes") {
		t.Fatalf("manifest Patrol description must describe Patrol as the primary built-in operator, got %q", contract.ProactiveEngine.Description)
	}

	if len(contract.OperatorSurfaces) != 2 {
		t.Fatalf("access path count = %d, want Assistant and MCP", len(contract.OperatorSurfaces))
	}
	surfaces := map[string]OperatorSurfaceContract{}
	for _, surface := range contract.OperatorSurfaces {
		surfaces[surface.ID] = surface
		if surface.ID == "pulse_patrol" {
			t.Fatal("Pulse Patrol must stay the primary built-in operator component, not be flattened into the compatibility access-path list")
		}
	}

	assistant := surfaces["pulse_assistant"]
	if assistant.Label != "Pulse Assistant" || !assistant.Native || assistant.ExternalAdapter {
		t.Fatalf("Pulse Assistant surface contract = %+v", assistant)
	}
	if !strings.Contains(assistant.Description, "contextual explanation") ||
		!strings.Contains(assistant.Description, "Patrol findings") {
		t.Fatalf("Pulse Assistant description must preserve contextual access-path role, got %q", assistant.Description)
	}
	if !assistant.Affordances.Tools || !assistant.Affordances.InteractiveQuestions ||
		assistant.Affordances.Resources || assistant.Affordances.Prompts || assistant.Affordances.CapabilityMetadata {
		t.Fatalf("Pulse Assistant affordance contract = %+v", assistant.Affordances)
	}

	mcp := surfaces["pulse_mcp"]
	if mcp.Label != "Pulse MCP" || mcp.Native || !mcp.ExternalAdapter {
		t.Fatalf("Pulse MCP surface contract = %+v", mcp)
	}
	if !strings.Contains(mcp.Description, "external-agent adapter") {
		t.Fatalf("Pulse MCP description must preserve adapter role, got %q", mcp.Description)
	}
	if !mcp.Affordances.Tools || !mcp.Affordances.Resources || !mcp.Affordances.Prompts || !mcp.Affordances.CapabilityMetadata ||
		mcp.Affordances.InteractiveQuestions {
		t.Fatalf("Pulse MCP affordance contract = %+v", mcp.Affordances)
	}
}

func TestCanonicalManifestPublishesExternalSurfaceToolContracts(t *testing.T) {
	manifest := CanonicalManifest()
	if len(manifest.SurfaceToolContracts) != 1 {
		t.Fatalf("surface tool contracts = %+v, want static Pulse MCP contract only", manifest.SurfaceToolContracts)
	}

	mcp := manifest.SurfaceToolContracts[0]
	if mcp.SurfaceID != SurfaceIDPulseMCP || mcp.ToolSource != SurfaceToolSourceCapabilityManifest {
		t.Fatalf("surface tool contract identity = %+v", mcp)
	}
	if len(mcp.ToolNames) == 0 || !reflect.DeepEqual(mcp.ToolNames, mcp.CapabilityNames) {
		t.Fatalf("MCP surface tools must be manifest-published capability names, got tools=%#v capabilities=%#v", mcp.ToolNames, mcp.CapabilityNames)
	}
	want := canonicalPulseMCPSurfaceToolNames()
	if !reflect.DeepEqual(mcp.ToolNames, want) {
		t.Fatalf("MCP surface tools = %#v, want explicit canonical allowlist %#v", mcp.ToolNames, want)
	}
	for _, name := range mcp.ToolNames {
		switch name {
		case EventSubscriptionCapabilityName, PulseQuestionToolName:
			t.Fatalf("static external surface tools must not include %s: %#v", name, mcp.ToolNames)
		}
		if _, err := ResolveRequestResponseCapability(manifest.Capabilities, name); err != nil {
			t.Fatalf("static external surface tool %q must resolve to a request/response manifest capability: %v", name, err)
		}
	}
	if len(mcp.RegistryToolNames) != 0 || len(mcp.NativeToolNames) != 0 {
		t.Fatalf("static external surface must not expose Assistant runtime buckets: %+v", mcp)
	}
}

func TestCanonicalManifestPinsPulseMCPResolvedOperationsLoopCapabilities(t *testing.T) {
	manifest := CanonicalManifest()

	contract, ok := FindManifestSurfaceToolContract(manifest, SurfaceIDPulseMCP)
	if !ok {
		t.Fatal("Pulse MCP must publish a static surface tool contract")
	}
	if contract.ToolSource != SurfaceToolSourceCapabilityManifest {
		t.Fatalf("Pulse MCP tool source = %q, want %q", contract.ToolSource, SurfaceToolSourceCapabilityManifest)
	}

	surfaceCapabilities := ManifestSurfaceToolCapabilities(manifest, SurfaceIDPulseMCP)
	surfaceCapabilityNames := map[string]bool{}
	for _, cap := range surfaceCapabilities {
		surfaceCapabilityNames[cap.Name] = true
	}
	surfaceTools := ProjectManifestSurfaceTools(manifest, SurfaceIDPulseMCP)
	surfaceToolsByName := map[string]ProjectedTool{}
	for _, tool := range surfaceTools {
		surfaceToolsByName[tool.Name] = tool
	}

	requiredLoop := []struct {
		name       string
		category   string
		method     string
		path       string
		scope      string
		mode       ActionMode
		approval   ApprovalPolicy
		errorCodes []string
	}{
		{
			name:     FleetContextCapabilityName,
			category: "context",
			method:   http.MethodGet,
			path:     FleetContextCapabilityPath,
			scope:    auth.ScopeMonitoringRead,
			mode:     ActionModeRead,
			approval: ApprovalPolicyScopeOnly,
		},
		{
			name:     OperationsLoopStatusCapabilityName,
			category: "context",
			method:   http.MethodGet,
			path:     OperationsLoopStatusCapabilityPath,
			scope:    auth.ScopeMonitoringRead,
			mode:     ActionModeRead,
			approval: ApprovalPolicyScopeOnly,
		},
		{
			name:       ResourceContextCapabilityName,
			category:   "context",
			method:     http.MethodGet,
			path:       ResourceContextCapabilityPath,
			scope:      auth.ScopeMonitoringRead,
			mode:       ActionModeRead,
			approval:   ApprovalPolicyScopeOnly,
			errorCodes: []string{AgentErrCodeResourceNotFound},
		},
		{
			name:       ListResourceCapabilitiesCapabilityName,
			category:   "context",
			method:     http.MethodGet,
			path:       ListResourceCapabilitiesCapabilityPath,
			scope:      auth.ScopeMonitoringRead,
			mode:       ActionModeRead,
			approval:   ApprovalPolicyScopeOnly,
			errorCodes: []string{AgentErrCodeResourceNotFound},
		},
		{
			name:     ListFindingsCapabilityName,
			category: "finding",
			method:   http.MethodGet,
			path:     "/api/ai/patrol/findings",
			scope:    auth.ScopeAIExecute,
			mode:     ActionModeRead,
			approval: ApprovalPolicyScopeOnly,
		},
		{
			name:     ResolveFindingCapabilityName,
			category: "finding",
			method:   http.MethodPost,
			path:     "/api/ai/patrol/resolve",
			scope:    auth.ScopeAIExecute,
			mode:     ActionModeWrite,
			approval: ApprovalPolicyScopeOnly,
			errorCodes: []string{
				AgentErrCodeInvalidFindingRequest,
				AgentErrCodeFindingNotFound,
				AgentErrCodeFindingActionNotAllowed,
				AgentErrCodePatrolUnavailable,
			},
		},
		{
			name:     PlanActionCapabilityName,
			category: "action",
			method:   http.MethodPost,
			path:     PlanActionCapabilityPath,
			scope:    auth.ScopeActionsPlan,
			mode:     ActionModeWrite,
			approval: ApprovalPolicyActionPlan,
			errorCodes: []string{
				AgentErrCodeInvalidActionRequest,
				AgentErrCodeActionActorUnavailable,
				AgentErrCodeResourceNotFound,
				AgentErrCodeCapabilityNotFound,
				AgentErrCodeActionExecutionUnavailable,
			},
		},
		{
			name:     DecideActionCapabilityName,
			category: "action",
			method:   http.MethodPost,
			path:     ActionDecisionCapabilityPath,
			scope:    auth.ScopeActionsApprove,
			mode:     ActionModeWrite,
			approval: ApprovalPolicyActionPlan,
			errorCodes: []string{
				AgentErrCodeMissingID,
				AgentErrCodeInvalidID,
				AgentErrCodeInvalidActionDecision,
				AgentErrCodeActionNotFound,
				AgentErrCodeActionNotPending,
				AgentErrCodeActionPlanExpired,
				AgentErrCodeActionActorUnavailable,
				AgentErrCodeActionApprovalForbidden,
				AgentErrCodeActionStepUpUnavailable,
				AgentErrCodeActionDecisionConflict,
				AgentErrCodeActionSeparationRequired,
				AgentErrCodeActionReplanRequired,
			},
		},
		{
			name:     ExecuteActionCapabilityName,
			category: "action",
			method:   http.MethodPost,
			path:     ActionExecutionCapabilityPath,
			scope:    auth.ScopeActionsExecute,
			mode:     ActionModeWrite,
			approval: ApprovalPolicyActionPlan,
			errorCodes: []string{
				AgentErrCodeMissingID,
				AgentErrCodeInvalidID,
				AgentErrCodeInvalidActionExecution,
				AgentErrCodeActionNotFound,
				AgentErrCodeActionNotApproved,
				AgentErrCodeActionAlreadyExecuting,
				AgentErrCodeActionExecutionFinal,
				AgentErrCodeActionDryRunOnly,
				AgentErrCodeActionPlanExpired,
				AgentErrCodeActionPlanDrift,
				AgentErrCodeResourceRemediationLocked,
				AgentErrCodeActionExecutorUnavailable,
				AgentErrCodeActionActorUnavailable,
				AgentErrCodeActionExecutionForbidden,
				AgentErrCodeActionNotExecuting,
				AgentErrCodeActionReplanRequired,
			},
		},
	}

	for _, want := range requiredLoop {
		if !containsString(contract.ToolNames, want.name) {
			t.Fatalf("Pulse MCP surface contract missing resolved-operations loop capability %q: %#v", want.name, contract.ToolNames)
		}
		if !surfaceCapabilityNames[want.name] {
			t.Fatalf("Pulse MCP surface capability projection missing %q: %#v", want.name, surfaceCapabilities)
		}
		tool, ok := surfaceToolsByName[want.name]
		if !ok {
			t.Fatalf("Pulse MCP tools/list projection missing %q: %#v", want.name, surfaceTools)
		}

		cap, ok := FindCapability(manifest.Capabilities, want.name)
		if !ok {
			t.Fatalf("CanonicalManifest missing %q", want.name)
		}
		if cap.Category != want.category || cap.Method != want.method || cap.Path != want.path || cap.Scope != want.scope {
			t.Fatalf("%s manifest route contract = category:%q method:%q path:%q scope:%q, want category:%q method:%q path:%q scope:%q",
				want.name, cap.Category, cap.Method, cap.Path, cap.Scope, want.category, want.method, want.path, want.scope)
		}
		if cap.ActionMode != want.mode || cap.ApprovalPolicy != want.approval {
			t.Fatalf("%s governance = %s/%s, want %s/%s", want.name, cap.ActionMode, cap.ApprovalPolicy, want.mode, want.approval)
		}
		for _, errorCode := range want.errorCodes {
			if !containsString(cap.ErrorCodes, errorCode) {
				t.Fatalf("%s errorCodes = %#v, missing %q", want.name, cap.ErrorCodes, errorCode)
			}
		}

		meta, ok := tool.Meta[ToolMetaPulseCapabilityKey].(map[string]any)
		if !ok {
			t.Fatalf("%s projected MCP tool missing Pulse capability metadata: %#v", want.name, tool.Meta)
		}
		if meta["name"] != want.name || meta["category"] != want.category || meta["scope"] != want.scope {
			t.Fatalf("%s projected MCP metadata = %#v, want name/category/scope from manifest", want.name, meta)
		}
		route, _ := meta["route"].(map[string]any)
		if route["method"] != want.method || route["path"] != want.path {
			t.Fatalf("%s projected MCP route metadata = %#v, want %s %s", want.name, route, want.method, want.path)
		}
		governance, _ := meta["governance"].(map[string]any)
		if governance["actionMode"] != string(want.mode) || governance["approvalPolicy"] != string(want.approval) {
			t.Fatalf("%s projected MCP governance metadata = %#v, want %s/%s", want.name, governance, want.mode, want.approval)
		}
	}
}

func TestCanonicalManifestProjectsWorkflowPromptsFromCapabilities(t *testing.T) {
	manifest := CanonicalManifest()
	want := ProjectPulseWorkflowPrompts(manifest.Capabilities)
	if len(manifest.WorkflowPrompts) == 0 {
		t.Fatal("CanonicalManifest must expose shared Pulse Intelligence workflow prompts")
	}
	if len(manifest.WorkflowPrompts) != len(want) {
		t.Fatalf("CanonicalManifest WorkflowPrompts length = %d, want %d", len(manifest.WorkflowPrompts), len(want))
	}

	gotNames := make([]string, 0, len(manifest.WorkflowPrompts))
	wantNames := make([]string, 0, len(want))
	for i := range manifest.WorkflowPrompts {
		got := manifest.WorkflowPrompts[i]
		expected := want[i]
		gotNames = append(gotNames, got.Name)
		wantNames = append(wantNames, expected.Name)
		if got.Name != expected.Name || got.Description != expected.Description {
			t.Fatalf("workflow prompt[%d] = %+v, want %+v", i, got, expected)
		}
		if len(got.Arguments) != len(expected.Arguments) {
			t.Fatalf("workflow prompt[%d] argument count = %d, want %d", i, len(got.Arguments), len(expected.Arguments))
		}
	}

	for _, name := range []string{
		PulseWorkflowPromptTriageFleet,
		PulseWorkflowPromptInvestigateResource,
		PulseWorkflowPromptReviewFinding,
		PulseWorkflowPromptOperationsLoop,
	} {
		if !containsString(gotNames, name) {
			t.Fatalf("CanonicalManifest workflow prompts = %v, missing %q; projected prompts = %v", gotNames, name, wantNames)
		}
	}
}

func TestCanonicalManifestReturnsDetachedCopy(t *testing.T) {
	first := CanonicalManifest()
	if len(first.Capabilities) == 0 {
		t.Fatal("CanonicalManifest must declare capabilities")
	}

	first.Version = "mutated"
	first.SurfaceContract.Core.Label = "mutated"
	if len(first.SurfaceContract.OperatorSurfaces) == 0 {
		t.Fatal("CanonicalManifest must declare operator surfaces")
	}
	first.SurfaceContract.OperatorSurfaces[0].Label = "mutated"
	if len(first.SurfaceToolContracts) == 0 {
		t.Fatal("CanonicalManifest must declare static external surface tool contracts")
	}
	if len(first.SurfaceToolContracts[0].ToolNames) == 0 || len(first.SurfaceToolContracts[0].CapabilityNames) == 0 {
		t.Fatal("CanonicalManifest must declare static external surface tool names")
	}
	first.SurfaceToolContracts[0].ToolNames[0] = "mutated"
	first.SurfaceToolContracts[0].CapabilityNames[0] = "mutated"
	first.MCPAdapter.Command = "mutated"
	if len(first.MCPAdapter.ConfigFamilies) == 0 {
		t.Fatal("CanonicalManifest must declare MCP adapter config families")
	}
	first.MCPAdapter.ConfigFamilies[0].Label = "mutated"
	if len(first.MCPAdapter.ConfigFamilies[0].FileHints) > 0 {
		first.MCPAdapter.ConfigFamilies[0].FileHints[0] = "mutated"
	}
	if len(first.MCPAdapter.ConfigFamilies[0].ClientLabels) > 0 {
		first.MCPAdapter.ConfigFamilies[0].ClientLabels[0] = "mutated"
	}
	first.Capabilities[0].Name = "mutated"
	if len(first.Capabilities[0].ErrorCodes) > 0 {
		first.Capabilities[0].ErrorCodes[0] = "mutated"
	}
	if len(first.RequiredScopes) == 0 {
		t.Fatal("CanonicalManifest must declare required scopes")
	}
	first.RequiredScopes[0] = "mutated"
	if len(first.Categories) == 0 {
		t.Fatal("CanonicalManifest must declare capability categories")
	}
	first.Categories[0].Label = "mutated"
	if len(first.WorkflowPrompts) == 0 {
		t.Fatal("CanonicalManifest must declare workflow prompts")
	}
	first.WorkflowPrompts[0].Name = "mutated"
	workflowPromptArgumentIndex := -1
	for i := range first.WorkflowPrompts {
		if len(first.WorkflowPrompts[i].Arguments) > 0 {
			workflowPromptArgumentIndex = i
			first.WorkflowPrompts[i].Arguments[0].Name = "mutated"
			break
		}
	}
	if workflowPromptArgumentIndex < 0 {
		t.Fatal("CanonicalManifest must declare at least one workflow prompt argument")
	}
	schemaIndex := -1
	for i := range first.Capabilities {
		if len(first.Capabilities[i].InputSchema) > 0 {
			schemaIndex = i
			first.Capabilities[i].InputSchema[0] = '['
			break
		}
	}
	if schemaIndex < 0 {
		t.Fatal("CanonicalManifest must include at least one authored input schema")
	}

	second := CanonicalManifest()
	if second.Version != "v1" {
		t.Fatalf("CanonicalManifest version was mutated: %q", second.Version)
	}
	if second.SurfaceContract.Core.Label == "mutated" {
		t.Fatal("CanonicalManifest returned aliased surface contract component")
	}
	if second.SurfaceContract.OperatorSurfaces[0].Label == "mutated" {
		t.Fatal("CanonicalManifest returned aliased operator surfaces")
	}
	if second.SurfaceToolContracts[0].ToolNames[0] == "mutated" {
		t.Fatal("CanonicalManifest returned aliased surface tool contract tool names")
	}
	if second.SurfaceToolContracts[0].CapabilityNames[0] == "mutated" {
		t.Fatal("CanonicalManifest returned aliased surface tool contract capability names")
	}
	if second.MCPAdapter.Command == "mutated" {
		t.Fatal("CanonicalManifest returned aliased MCP adapter contract")
	}
	if second.MCPAdapter.ConfigFamilies[0].Label == "mutated" {
		t.Fatal("CanonicalManifest returned aliased MCP adapter config families")
	}
	if len(second.MCPAdapter.ConfigFamilies[0].FileHints) > 0 && second.MCPAdapter.ConfigFamilies[0].FileHints[0] == "mutated" {
		t.Fatal("CanonicalManifest returned aliased MCP adapter config-family file hints")
	}
	if len(second.MCPAdapter.ConfigFamilies[0].ClientLabels) > 0 && second.MCPAdapter.ConfigFamilies[0].ClientLabels[0] == "mutated" {
		t.Fatal("CanonicalManifest returned aliased MCP adapter config-family client labels")
	}
	if second.Capabilities[0].Name == "mutated" {
		t.Fatal("CanonicalManifest returned aliased capability slice")
	}
	if len(first.Capabilities[0].ErrorCodes) > 0 && second.Capabilities[0].ErrorCodes[0] == "mutated" {
		t.Fatal("CanonicalManifest returned aliased error code slice")
	}
	if second.RequiredScopes[0] == "mutated" {
		t.Fatal("CanonicalManifest returned aliased required scope slice")
	}
	if second.Categories[0].Label == "mutated" {
		t.Fatal("CanonicalManifest returned aliased capability category slice")
	}
	if second.WorkflowPrompts[0].Name == "mutated" {
		t.Fatal("CanonicalManifest returned aliased workflow prompt slice")
	}
	if second.WorkflowPrompts[workflowPromptArgumentIndex].Arguments[0].Name == "mutated" {
		t.Fatal("CanonicalManifest returned aliased workflow prompt argument slice")
	}
	if string(second.Capabilities[schemaIndex].InputSchema) == string(first.Capabilities[schemaIndex].InputSchema) {
		t.Fatal("CanonicalManifest returned aliased input schema")
	}
}

func TestCanonicalManifestPinsMCPAdapterSetupContract(t *testing.T) {
	adapter := CanonicalManifest().MCPAdapter
	if adapter.ServerName != DefaultMCPAdapterServerName {
		t.Fatalf("MCP adapter server name = %q, want %q", adapter.ServerName, DefaultMCPAdapterServerName)
	}
	if adapter.Command != DefaultMCPAdapterCommand {
		t.Fatalf("MCP adapter command = %q, want %q", adapter.Command, DefaultMCPAdapterCommand)
	}
	if adapter.BaseURLFlag != DefaultMCPAdapterBaseURLFlag {
		t.Fatalf("MCP adapter base URL flag = %q, want %q", adapter.BaseURLFlag, DefaultMCPAdapterBaseURLFlag)
	}
	if adapter.DefaultBaseURL != DefaultMCPAdapterDefaultBaseURL {
		t.Fatalf("MCP adapter default base URL = %q, want %q", adapter.DefaultBaseURL, DefaultMCPAdapterDefaultBaseURL)
	}
	if adapter.TokenEnv != DefaultMCPAdapterTokenEnv {
		t.Fatalf("MCP adapter token env = %q, want %q", adapter.TokenEnv, DefaultMCPAdapterTokenEnv)
	}

	families := map[string]MCPAdapterConfigFamily{}
	for _, family := range adapter.ConfigFamilies {
		if family.ID == "" || family.Label == "" || family.Shape == "" {
			t.Fatalf("MCP adapter config family must declare id, label, and shape: %+v", family)
		}
		families[family.Shape] = family
	}
	for _, shape := range []string{
		MCPAdapterConfigShapeOpenCodeMCP,
		MCPAdapterConfigShapeMCPServers,
		MCPAdapterConfigShapeCustom,
	} {
		if _, ok := families[shape]; !ok {
			t.Fatalf("MCP adapter config family shape %q missing from manifest contract", shape)
		}
	}

	opencode := families[MCPAdapterConfigShapeOpenCodeMCP]
	if opencode.Label != "OpenCode" || !containsString(opencode.FileHints, "opencode.json") {
		t.Fatalf("OpenCode MCP adapter family = %+v", opencode)
	}
	mcpServers := families[MCPAdapterConfigShapeMCPServers]
	if !containsString(mcpServers.ClientLabels, "Claude Desktop") || !containsString(mcpServers.ClientLabels, "Claude Code") {
		t.Fatalf("mcpServers adapter family must cover Claude Desktop and Claude Code, got %+v", mcpServers)
	}
	custom := families[MCPAdapterConfigShapeCustom]
	if custom.Label != "custom MCP clients" {
		t.Fatalf("custom MCP adapter family = %+v", custom)
	}
}

func TestCanonicalManifestDeclaresCategoryPresentation(t *testing.T) {
	manifest := CanonicalManifest()
	if len(manifest.Categories) == 0 {
		t.Fatal("CanonicalManifest must declare category presentation metadata")
	}

	byID := map[string]CapabilityCategory{}
	for _, category := range manifest.Categories {
		if category.ID == "" {
			t.Fatal("category id must not be empty")
		}
		if category.Label == "" {
			t.Fatalf("category %q must declare a label", category.ID)
		}
		if category.Description == "" {
			t.Fatalf("category %q must declare a description", category.ID)
		}
		if _, exists := byID[category.ID]; exists {
			t.Fatalf("duplicate category id %q", category.ID)
		}
		byID[category.ID] = category
	}

	for _, cap := range manifest.Capabilities {
		if _, ok := byID[cap.Category]; !ok {
			t.Fatalf("capability %q category %q missing from manifest category metadata", cap.Name, cap.Category)
		}
	}
}

func TestCanonicalManifestDeclaresCapabilityTitles(t *testing.T) {
	for _, cap := range CanonicalManifest().Capabilities {
		if strings.TrimSpace(cap.Title) == "" {
			t.Fatalf("capability %q must declare a human-readable title", cap.Name)
		}
		if strings.EqualFold(strings.TrimSpace(cap.Title), strings.TrimSpace(cap.Name)) {
			t.Fatalf("capability %q title must be display copy, got %q", cap.Name, cap.Title)
		}
	}
}

func TestCanonicalManifestDeclaresStructuredOutputSchemasForJSONResponses(t *testing.T) {
	manifest := CanonicalManifest()
	byName := map[string]Capability{}
	for _, cap := range manifest.Capabilities {
		byName[cap.Name] = cap
		if !IsRequestResponseCapability(cap) || strings.TrimSpace(cap.ResponseShape) == "" {
			continue
		}
		if len(cap.OutputSchema) == 0 {
			t.Errorf("capability %q responseShape %q must declare outputSchema for structured tool results", cap.Name, cap.ResponseShape)
			continue
		}
		var schema map[string]any
		if err := json.Unmarshal(cap.OutputSchema, &schema); err != nil {
			t.Errorf("capability %q outputSchema must be valid JSON: %v", cap.Name, err)
			continue
		}
		if schema["type"] != "object" {
			t.Errorf("capability %q outputSchema type = %v, want object", cap.Name, schema["type"])
		}
		props, _ := schema["properties"].(map[string]any)
		if strings.HasSuffix(strings.TrimSpace(cap.ResponseShape), "[]") {
			if _, ok := props["items"]; !ok {
				t.Errorf("array-returning capability %q must declare items for the structured content wrapper: %v", cap.Name, props)
			}
			if _, ok := props["count"]; !ok {
				t.Errorf("array-returning capability %q must declare count for the structured content wrapper: %v", cap.Name, props)
			}
			if !schemaRequiredContains(schema, "items") || !schemaRequiredContains(schema, "count") {
				t.Errorf("array-returning capability %q outputSchema required = %v, want items and count", cap.Name, schema["required"])
			}
		}
	}

	for name, fragments := range map[string][]string{
		"list_nodes":                      {`"items"`, `"count"`},
		"discover_lan":                    {`"servers"`, `"structured_errors"`, `"cached"`},
		"test_node_credentials":           {`"status"`, `"message"`},
		"refresh_node_cluster_membership": {`"clusterNodes"`, `"nodesAdded"`},
		ListFindingsCapabilityName:        {`"items"`, `"count"`},
		AcknowledgeFindingCapabilityName:  {`"success"`, `"message"`},
		OperationsLoopStatusCapabilityName: {
			`"steps"`,
			`Four-stage Patrol, Assistant, governance, and verification status rollup`,
			`Assistant counts contextual collaboration`,
			`External-agent readiness is exposed separately through externalAgentReady`,
			`Count-only evidence that the Patrol control journey reached a verified Patrol work result`,
			`Compatibility alias for patrolControlResolvedOperationsLoopCount`,
			`one non-expired API token covers every scope required by the published Pulse MCP Patrol work capability set`,
		},
		PlanActionCapabilityName:    {`"actionId"`, `"planHash"`},
		DecideActionCapabilityName:  {`"approval"`, `"audit"`},
		ExecuteActionCapabilityName: {`"result"`, `"audit"`},
	} {
		cap, ok := byName[name]
		if !ok {
			t.Fatalf("manifest missing capability %q", name)
		}
		raw, err := json.Marshal(cap.OutputSchema)
		if err != nil {
			t.Fatalf("%s outputSchema marshal: %v", name, err)
		}
		text := string(raw)
		for _, fragment := range fragments {
			if !strings.Contains(text, fragment) {
				t.Errorf("%s outputSchema missing %s: %s", name, fragment, text)
			}
		}
	}
}

func TestCanonicalManifestRequiredScopesReflectCapabilities(t *testing.T) {
	manifest := CanonicalManifest()
	want := RequiredCapabilityScopes(manifest.Capabilities)
	if strings.Join(manifest.RequiredScopes, "\n") != strings.Join(want, "\n") {
		t.Fatalf("CanonicalManifest RequiredScopes = %v, want %v", manifest.RequiredScopes, want)
	}
}

func TestCanonicalManifestScopesUseAuthVocabulary(t *testing.T) {
	known := map[string]bool{}
	for _, scope := range auth.AllKnownScopes {
		known[scope] = true
	}

	for _, cap := range CanonicalManifest().Capabilities {
		if !known[cap.Scope] {
			t.Errorf("capability %q scope = %q, want auth-owned scope vocabulary", cap.Name, cap.Scope)
		}
	}
}

func TestCanonicalManifestScopeDeclarationsStayPinnedToAuthConstants(t *testing.T) {
	source, err := os.ReadFile("manifest.go")
	if err != nil {
		t.Fatalf("read manifest.go: %v", err)
	}
	text := string(source)

	for _, required := range []string{
		`agentCapabilityScopeMonitoringRead  = auth.ScopeMonitoringRead`,
		`agentCapabilityScopeMonitoringWrite = auth.ScopeMonitoringWrite`,
		`agentCapabilityScopeSettingsRead    = auth.ScopeSettingsRead`,
		`agentCapabilityScopeSettingsWrite   = auth.ScopeSettingsWrite`,
		`agentCapabilityScopeAIExecute       = auth.ScopeAIExecute`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("agent capabilities manifest must declare scope aliases from pkg/auth; missing %s", required)
		}
	}

	if strings.Contains(text, `Scope:          "`) || strings.Contains(text, `Scope:            "`) {
		t.Fatal("agent capabilities manifest scopes must use auth-owned constants, not literal strings")
	}
}

func schemaRequiredContains(schema map[string]any, want string) bool {
	required, _ := schema["required"].([]any)
	for _, raw := range required {
		if raw == want {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
