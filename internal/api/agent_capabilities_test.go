package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleAgentCapabilitiesManifest_ReturnsStableShape(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/capabilities", nil)
	HandleAgentCapabilitiesManifest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: got %q want application/json", got)
	}
	// Cacheable so agents can hold the manifest in memory across
	// requests; 5 minutes mirrors the typical agent session length.
	if got := rec.Header().Get("Cache-Control"); got == "" {
		t.Error("manifest must be cacheable; Cache-Control header missing")
	}

	var manifest AgentCapabilitiesManifest
	if err := json.Unmarshal(rec.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if manifest.Version != "v1" {
		t.Errorf("version pin: got %q want v1", manifest.Version)
	}
	if manifest.SurfaceContract.Core.Label != "Pulse Intelligence Core" {
		t.Fatalf("manifest must expose Pulse Intelligence Core surface contract, got %+v", manifest.SurfaceContract)
	}
	if len(manifest.SurfaceContract.OperatorSurfaces) != 2 {
		t.Fatalf("manifest access path count = %d, want Assistant and MCP", len(manifest.SurfaceContract.OperatorSurfaces))
	}
	surfaceIDs := map[string]bool{}
	for _, surface := range manifest.SurfaceContract.OperatorSurfaces {
		surfaceIDs[surface.ID] = true
	}
	if !surfaceIDs["pulse_assistant"] || !surfaceIDs["pulse_mcp"] || surfaceIDs["pulse_patrol"] {
		t.Fatalf("manifest compatibility access paths must be Assistant and MCP; Patrol is the primary built-in operator component, got %+v", manifest.SurfaceContract.OperatorSurfaces)
	}
	if len(manifest.SurfaceToolContracts) != 1 {
		t.Fatalf("manifest surfaceToolContracts = %+v, want static Pulse MCP tool posture", manifest.SurfaceToolContracts)
	}
	mcpSurfaceTools := manifest.SurfaceToolContracts[0]
	if mcpSurfaceTools.SurfaceID != agentcapabilities.SurfaceIDPulseMCP || mcpSurfaceTools.ToolSource != agentcapabilities.SurfaceToolSourceCapabilityManifest {
		t.Fatalf("manifest MCP surface tool contract = %+v", mcpSurfaceTools)
	}
	for _, name := range mcpSurfaceTools.ToolNames {
		if name == agentcapabilities.EventSubscriptionCapabilityName || name == agentcapabilities.PulseQuestionToolName {
			t.Fatalf("manifest MCP surface tool names must exclude streaming/native tools, got %#v", mcpSurfaceTools.ToolNames)
		}
	}
	if len(manifest.Capabilities) == 0 {
		t.Fatal("manifest must declare at least one capability")
	}
	if len(manifest.Categories) == 0 {
		t.Fatal("manifest must declare category presentation metadata")
	}
	wantPrompts := agentcapabilities.ProjectPulseWorkflowPrompts(manifest.Capabilities)
	if len(manifest.WorkflowPrompts) == 0 {
		t.Fatal("manifest must declare shared workflow prompts")
	}
	if len(manifest.WorkflowPrompts) != len(wantPrompts) {
		t.Fatalf("manifest workflowPrompts length = %d, want %d", len(manifest.WorkflowPrompts), len(wantPrompts))
	}
	for i := range manifest.WorkflowPrompts {
		if manifest.WorkflowPrompts[i].Name != wantPrompts[i].Name {
			t.Fatalf("manifest workflowPrompts[%d].name = %q, want %q", i, manifest.WorkflowPrompts[i].Name, wantPrompts[i].Name)
		}
	}
	wantScopes := agentcapabilities.RequiredCapabilityScopes(manifest.Capabilities)
	if strings.Join(manifest.RequiredScopes, "\n") != strings.Join(wantScopes, "\n") {
		t.Fatalf("manifest requiredScopes = %v, want %v", manifest.RequiredScopes, wantScopes)
	}
	knownCategories := map[string]bool{}
	for _, category := range manifest.Categories {
		if category.ID == "" || category.Label == "" {
			t.Fatalf("manifest category metadata must declare id and label, got %+v", category)
		}
		knownCategories[category.ID] = true
	}

	foundPlanAction := false
	foundFleetContextOutputSchema := false
	foundListNodesOutputSchema := false
	missingOutputSchemas := []string{}
	for _, cap := range manifest.Capabilities {
		if !knownCategories[cap.Category] {
			t.Errorf("capability %q category %q missing from manifest category metadata", cap.Name, cap.Category)
		}
		if agentcapabilities.IsRequestResponseCapability(cap) && strings.TrimSpace(cap.ResponseShape) != "" && len(cap.OutputSchema) == 0 {
			missingOutputSchemas = append(missingOutputSchemas, cap.Name)
		}
		if len(cap.OutputSchema) > 0 {
			var schema map[string]any
			if err := json.Unmarshal(cap.OutputSchema, &schema); err != nil {
				t.Errorf("capability %q outputSchema must be valid JSON Schema: %v", cap.Name, err)
			}
			if schema["type"] != "object" {
				t.Errorf("capability %q outputSchema type = %v, want object for MCP structuredContent", cap.Name, schema["type"])
			}
		}
		if cap.Name == agentcapabilities.FleetContextCapabilityName && strings.Contains(string(cap.OutputSchema), `"resources"`) {
			foundFleetContextOutputSchema = true
		}
		if cap.Name == "list_nodes" && strings.Contains(string(cap.OutputSchema), `"items"`) && strings.Contains(string(cap.OutputSchema), `"count"`) {
			foundListNodesOutputSchema = true
		}
		if cap.Name != agentcapabilities.PlanActionCapabilityName {
			continue
		}
		foundPlanAction = true
		if cap.ActionMode != agentcapabilities.ActionModeWrite {
			t.Errorf("plan_action actionMode: got %q want %q", cap.ActionMode, agentcapabilities.ActionModeWrite)
		}
		if cap.ApprovalPolicy != agentcapabilities.ApprovalPolicyActionPlan {
			t.Errorf("plan_action approvalPolicy: got %q want %q", cap.ApprovalPolicy, agentcapabilities.ApprovalPolicyActionPlan)
		}
	}
	if !foundPlanAction {
		t.Fatalf("manifest must expose %s capability", agentcapabilities.PlanActionCapabilityName)
	}
	if !foundFleetContextOutputSchema {
		t.Fatalf("manifest must expose %s outputSchema so structured MCP results are typed", agentcapabilities.FleetContextCapabilityName)
	}
	if len(missingOutputSchemas) > 0 {
		t.Fatalf("manifest request/response capabilities with responseShape must expose outputSchema: %v", missingOutputSchemas)
	}
	if !foundListNodesOutputSchema {
		t.Fatal("manifest must expose list_nodes outputSchema for array structuredContent wrapper")
	}
}

func TestHandleAgentCapabilitiesManifest_RejectsNonGet(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agent/capabilities", nil)
	HandleAgentCapabilitiesManifest(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405; got %d", rec.Code)
	}
}

func TestAgentCapabilitiesManifest_NamesAreUniqueAndSnakeCase(t *testing.T) {
	// Capability names are agent-stable identifiers — duplicates
	// would silently mask one capability behind another, and
	// non-snake_case would break the convention agents use for tool
	// names. Pin both invariants.
	seen := map[string]bool{}
	manifest := agentcapabilities.CanonicalManifest()
	for _, cap := range manifest.Capabilities {
		if seen[cap.Name] {
			t.Errorf("duplicate capability name %q — names are agent-stable identifiers", cap.Name)
		}
		seen[cap.Name] = true

		if cap.Name == "" {
			t.Error("capability name must not be empty")
			continue
		}
		for _, ch := range cap.Name {
			if !(ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')) {
				t.Errorf("capability name %q must be snake_case (lowercase letters, digits, underscores only); got rune %q", cap.Name, ch)
				break
			}
		}
	}
}

func TestAgentCapabilitiesManifest_EveryCapabilityDeclaresMethodPathScope(t *testing.T) {
	// Without method/path/scope, an agent can't actually call the
	// capability. These three are the minimum viable contract.
	manifest := agentcapabilities.CanonicalManifest()
	for _, cap := range manifest.Capabilities {
		if cap.Method == "" {
			t.Errorf("capability %q missing method", cap.Name)
		}
		if cap.Path == "" {
			t.Errorf("capability %q missing path", cap.Name)
		}
		if cap.Scope == "" {
			t.Errorf("capability %q missing scope", cap.Name)
		}
		if cap.Description == "" {
			t.Errorf("capability %q missing description", cap.Name)
		}
		if cap.Category == "" {
			t.Errorf("capability %q missing category — agents filter the manifest by category", cap.Name)
		}
	}
}

func TestAgentCapabilitiesManifest_ScopesMatchAPIAuthConstants(t *testing.T) {
	expected := map[string]string{
		agentcapabilities.ResourceContextCapabilityName:          config.ScopeMonitoringRead,
		agentcapabilities.ListResourceCapabilitiesCapabilityName: config.ScopeMonitoringRead,
		agentcapabilities.FleetContextCapabilityName:             config.ScopeMonitoringRead,
		agentcapabilities.OperationsLoopStatusCapabilityName:     config.ScopeMonitoringRead,
		"list_nodes":                      config.ScopeSettingsRead,
		"add_node":                        config.ScopeSettingsWrite,
		"update_node":                     config.ScopeSettingsWrite,
		"remove_node":                     config.ScopeSettingsWrite,
		"test_node_credentials":           config.ScopeSettingsWrite,
		"test_node_connection":            config.ScopeSettingsWrite,
		"refresh_node_cluster_membership": config.ScopeSettingsWrite,
		"discover_lan":                    config.ScopeSettingsWrite,
		agentcapabilities.GetOperatorStateCapabilityName:   config.ScopeMonitoringRead,
		agentcapabilities.SetOperatorStateCapabilityName:   config.ScopeMonitoringWrite,
		agentcapabilities.ClearOperatorStateCapabilityName: config.ScopeMonitoringWrite,
		agentcapabilities.EventSubscriptionCapabilityName:  config.ScopeMonitoringRead,
		agentcapabilities.ListFindingsCapabilityName:       config.ScopeAIExecute,
		agentcapabilities.AcknowledgeFindingCapabilityName: config.ScopeAIExecute,
		agentcapabilities.SnoozeFindingCapabilityName:      config.ScopeAIExecute,
		agentcapabilities.DismissFindingCapabilityName:     config.ScopeAIExecute,
		agentcapabilities.ResolveFindingCapabilityName:     config.ScopeAIExecute,
		agentcapabilities.PlanActionCapabilityName:         config.ScopeActionsPlan,
		agentcapabilities.DecideActionCapabilityName:       config.ScopeActionsApprove,
		agentcapabilities.ExecuteActionCapabilityName:      config.ScopeActionsExecute,
	}

	manifest := agentcapabilities.CanonicalManifest()
	if len(manifest.Capabilities) != len(expected) {
		t.Fatalf("expected scope map for every manifest capability: manifest=%d expected=%d", len(manifest.Capabilities), len(expected))
	}

	for _, cap := range manifest.Capabilities {
		want, ok := expected[cap.Name]
		if !ok {
			t.Errorf("capability %q missing from expected API scope map", cap.Name)
			continue
		}
		if cap.Scope != want {
			t.Errorf("capability %q scope = %q, want API auth scope %q", cap.Name, cap.Scope, want)
		}
	}
}

func TestAgentCapabilitiesManifest_PatrolFindingScopesMatchAPIRoutes(t *testing.T) {
	manifest := agentcapabilities.CanonicalManifest()
	byName := map[string]agentcapabilities.Capability{}
	for _, cap := range manifest.Capabilities {
		byName[cap.Name] = cap
	}

	expected := map[string]relayMobileRuntimeRouteID{
		agentcapabilities.ListFindingsCapabilityName:       relayMobileRoutePatrolFindingsList,
		agentcapabilities.AcknowledgeFindingCapabilityName: relayMobileRoutePatrolAcknowledge,
		agentcapabilities.SnoozeFindingCapabilityName:      relayMobileRoutePatrolSnooze,
		agentcapabilities.DismissFindingCapabilityName:     relayMobileRoutePatrolDismiss,
	}
	for capabilityName, routeID := range expected {
		capability, ok := byName[capabilityName]
		if !ok {
			t.Fatalf("manifest missing Patrol finding capability %q", capabilityName)
		}
		route := relayMobileRuntimeRouteSpecFor(routeID)
		if capability.Method != route.method || capability.Path != route.path || capability.Scope != route.requiredScope {
			t.Fatalf("%s manifest route/scope = %s %s => %s, want API route %s %s => %s",
				capabilityName, capability.Method, capability.Path, capability.Scope, route.method, route.path, route.requiredScope)
		}
	}

	resolve, ok := byName[agentcapabilities.ResolveFindingCapabilityName]
	if !ok {
		t.Fatalf("manifest missing Patrol finding capability %q", agentcapabilities.ResolveFindingCapabilityName)
	}
	if resolve.Method != http.MethodPost || resolve.Path != "/api/ai/patrol/resolve" || resolve.Scope != config.ScopeAIExecute {
		t.Fatalf("%s manifest route/scope = %s %s => %s, want API route POST /api/ai/patrol/resolve => %s",
			agentcapabilities.ResolveFindingCapabilityName, resolve.Method, resolve.Path, resolve.Scope, config.ScopeAIExecute)
	}
}

func TestAgentCapabilitiesManifest_EveryCapabilityDeclaresGovernance(t *testing.T) {
	// External agents need to know whether a capability is only a
	// read, a non-persistent check/scan, or a write before choosing
	// a tool. Keep that posture on the manifest instead of letting
	// adapters infer it from HTTP method alone.
	allowedActionModes := map[agentcapabilities.ActionMode]bool{
		agentcapabilities.ActionModeRead:  true,
		agentcapabilities.ActionModeMixed: true,
		agentcapabilities.ActionModeWrite: true,
	}
	allowedApprovalPolicies := map[agentcapabilities.ApprovalPolicy]bool{
		agentcapabilities.ApprovalPolicyScopeOnly:  true,
		agentcapabilities.ApprovalPolicyActionPlan: true,
	}

	manifest := agentcapabilities.CanonicalManifest()
	for _, cap := range manifest.Capabilities {
		if !allowedActionModes[cap.ActionMode] {
			t.Errorf("capability %q has unknown actionMode %q", cap.Name, cap.ActionMode)
		}
		if !allowedApprovalPolicies[cap.ApprovalPolicy] {
			t.Errorf("capability %q has unknown approvalPolicy %q", cap.Name, cap.ApprovalPolicy)
		}
		if cap.Method == http.MethodGet && cap.ActionMode != agentcapabilities.ActionModeRead {
			t.Errorf("GET capability %q actionMode = %q, want read", cap.Name, cap.ActionMode)
		}
		if cap.Category == "action" && cap.ApprovalPolicy != agentcapabilities.ApprovalPolicyActionPlan {
			t.Errorf("action capability %q approvalPolicy = %q, want action_plan", cap.Name, cap.ApprovalPolicy)
		}
		if cap.Category != "action" && cap.ApprovalPolicy == agentcapabilities.ApprovalPolicyActionPlan {
			t.Errorf("non-action capability %q must not claim action_plan approval policy", cap.Name)
		}
	}
}

func TestAgentCapabilitiesManifest_CategoriesAreClosed(t *testing.T) {
	// Agents filter the manifest by category. Keep the set closed
	// so a typo in a future capability doesn't fragment the surface
	// (e.g. "operator-state" vs "operator_state" would split into
	// two categories an agent might miss).
	expectedOrder := []string{
		"context",
		"operator-state",
		"finding",
		"action",
		"provisioning",
	}
	manifest := agentcapabilities.CanonicalManifest()
	if len(manifest.Categories) != len(expectedOrder) {
		t.Fatalf("manifest categories = %v, want %v", manifest.Categories, expectedOrder)
	}
	allowed := map[string]bool{}
	for i, category := range manifest.Categories {
		if category.ID != expectedOrder[i] {
			t.Fatalf("manifest category order[%d] = %q, want %q", i, category.ID, expectedOrder[i])
		}
		if category.Label == "" || category.Description == "" {
			t.Fatalf("manifest category %q must include presentation label and description", category.ID)
		}
		allowed[category.ID] = true
	}
	for _, cap := range manifest.Capabilities {
		if !allowed[cap.Category] {
			t.Errorf("capability %q has unknown category %q — extend the allowlist deliberately", cap.Name, cap.Category)
		}
	}
}

func TestAgentCapabilitiesManifest_DeclaresNodeProvisioningSurface(t *testing.T) {
	byName := map[string]AgentCapability{}
	manifest := agentcapabilities.CanonicalManifest()
	for _, cap := range manifest.Capabilities {
		byName[cap.Name] = cap
	}

	required := map[string]struct {
		method string
		path   string
		scope  string
	}{
		"list_nodes":                      {http.MethodGet, "/api/config/nodes", "settings:read"},
		"add_node":                        {http.MethodPost, "/api/config/nodes", "settings:write"},
		"update_node":                     {http.MethodPut, "/api/config/nodes/{nodeId}", "settings:write"},
		"remove_node":                     {http.MethodDelete, "/api/config/nodes/{nodeId}", "settings:write"},
		"test_node_credentials":           {http.MethodPost, "/api/config/nodes/test-config", "settings:write"},
		"test_node_connection":            {http.MethodPost, "/api/config/nodes/{nodeId}/test", "settings:write"},
		"refresh_node_cluster_membership": {http.MethodPost, "/api/config/nodes/{nodeId}/refresh-cluster", "settings:write"},
		"discover_lan":                    {http.MethodPost, "/api/discover", "settings:write"},
	}

	for name, want := range required {
		cap, ok := byName[name]
		if !ok {
			t.Fatalf("manifest missing node provisioning capability %q", name)
		}
		if cap.Category != "provisioning" {
			t.Errorf("%s category = %q, want provisioning", name, cap.Category)
		}
		if cap.Method != want.method || cap.Path != want.path || cap.Scope != want.scope {
			t.Errorf("%s method/path/scope = %s %s %s, want %s %s %s",
				name, cap.Method, cap.Path, cap.Scope, want.method, want.path, want.scope)
		}
	}

	for _, name := range []string{"add_node", "update_node", "test_node_credentials", "discover_lan"} {
		cap := byName[name]
		if cap.InputSchema == nil {
			t.Fatalf("%s must publish an inputSchema so agent clients get typed onboarding arguments", name)
		}
		raw, err := json.Marshal(cap.InputSchema)
		if err != nil {
			t.Fatalf("%s inputSchema marshal: %v", name, err)
		}
		text := string(raw)
		for _, fragment := range []string{`"type":"object"`, `"additionalProperties":false`} {
			if !strings.Contains(text, fragment) {
				t.Errorf("%s inputSchema missing %s: %s", name, fragment, text)
			}
		}
	}

	addSchema, _ := json.Marshal(byName["add_node"].InputSchema)
	for _, fragment := range []string{`"enum":["pve","pbs","pmg"]`, `"required":["type","name","host"]`, `"tokenValue"`} {
		if !strings.Contains(string(addSchema), fragment) {
			t.Errorf("add_node inputSchema missing %s: %s", fragment, string(addSchema))
		}
	}

	updateSchema, _ := json.Marshal(byName["update_node"].InputSchema)
	if !strings.Contains(string(updateSchema), `"nodeId"`) {
		t.Errorf("update_node inputSchema must include nodeId path argument: %s", string(updateSchema))
	}

	discoverSchema, _ := json.Marshal(byName["discover_lan"].InputSchema)
	for _, fragment := range []string{`"subnet"`, `"use_cache"`} {
		if !strings.Contains(string(discoverSchema), fragment) {
			t.Errorf("discover_lan inputSchema missing %s: %s", fragment, string(discoverSchema))
		}
	}
}

func TestAgentCapabilitiesManifest_DeclaresTypedFindingAndActionSchemas(t *testing.T) {
	byName := map[string]AgentCapability{}
	manifest := agentcapabilities.CanonicalManifest()
	for _, cap := range manifest.Capabilities {
		byName[cap.Name] = cap
	}

	assertInputSchemaFragments(t, byName, agentcapabilities.SetOperatorStateCapabilityName, []string{
		requiredSchemaFragment(agentcapabilities.ResourceIDArgumentName, "intentionallyOffline", "neverAutoRemediate"),
		`"dependencies":{"maintenanceEndAt":["maintenanceStartAt"],"maintenanceStartAt":["maintenanceEndAt"]}`,
		`"enum":["high","medium","low",""]`,
	})
	assertInputSchemaFragments(t, byName, agentcapabilities.AcknowledgeFindingCapabilityName, []string{
		requiredSchemaFragment(agentcapabilities.FindingIDArgumentName),
		`"additionalProperties":false`,
	})
	assertInputSchemaFragments(t, byName, agentcapabilities.SnoozeFindingCapabilityName, []string{
		requiredSchemaFragment(agentcapabilities.FindingIDArgumentName, "duration_hours"),
		`"minimum":1`,
		`"maximum":168`,
	})
	assertInputSchemaFragments(t, byName, agentcapabilities.DismissFindingCapabilityName, []string{
		requiredSchemaFragment(agentcapabilities.FindingIDArgumentName, agentcapabilities.ReasonArgumentName),
		`"enum":["not_an_issue","expected_behavior","will_fix_later"]`,
		`"` + agentcapabilities.NoteArgumentName + `"`,
	})
	assertInputSchemaFragments(t, byName, agentcapabilities.ResolveFindingCapabilityName, []string{
		requiredSchemaFragment(agentcapabilities.FindingIDArgumentName),
		`"` + agentcapabilities.ResolutionNoteArgumentName + `"`,
	})
	assertInputSchemaFragments(t, byName, agentcapabilities.PlanActionCapabilityName, []string{
		requiredSchemaFragment(
			agentcapabilities.RequestIDArgumentName,
			agentcapabilities.ResourceIDArgumentName,
			agentcapabilities.CapabilityNameArgumentName,
			agentcapabilities.ReasonArgumentName,
			agentcapabilities.RequestedByArgumentName,
		),
		`"params"`,
		`"additionalProperties":true`,
	})
	assertInputSchemaFragments(t, byName, agentcapabilities.DecideActionCapabilityName, []string{
		requiredSchemaFragment(agentcapabilities.ActionIDArgumentName, agentcapabilities.OutcomeArgumentName),
		`"enum":["approved","rejected"]`,
		`"pattern":"^[a-zA-Z0-9_-]+$"`,
	})
	assertInputSchemaFragments(t, byName, agentcapabilities.ExecuteActionCapabilityName, []string{
		requiredSchemaFragment(agentcapabilities.ActionIDArgumentName),
		`"maxLength":128`,
	})
}

func requiredSchemaFragment(fields ...string) string {
	raw, _ := json.Marshal(fields)
	return `"required":` + string(raw)
}

func assertInputSchemaFragments(t *testing.T, byName map[string]AgentCapability, name string, fragments []string) {
	t.Helper()
	cap, ok := byName[name]
	if !ok {
		t.Fatalf("manifest missing capability %q", name)
	}
	if cap.InputSchema == nil {
		t.Fatalf("%s must publish an inputSchema", name)
	}
	raw, err := json.Marshal(cap.InputSchema)
	if err != nil {
		t.Fatalf("%s inputSchema marshal: %v", name, err)
	}
	text := string(raw)
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			t.Errorf("%s inputSchema missing %s: %s", name, fragment, text)
		}
	}
}

func TestAgentCapabilitiesManifest_CarriesStableErrorCodes(t *testing.T) {
	// The error-code surface is the agent-branching contract. The
	// codes I've shipped this session must appear on the
	// corresponding capability so agents can branch on them. Pin a
	// few of the most consequential codes.
	wantErrorCodes := map[string][]string{
		agentcapabilities.ResourceContextCapabilityName:  {agentcapabilities.AgentErrCodeResourceNotFound},
		agentcapabilities.GetOperatorStateCapabilityName: {agentcapabilities.AgentErrCodeOperatorStateNotSet},
		agentcapabilities.SetOperatorStateCapabilityName: {agentcapabilities.AgentErrCodeOperatorStateInvalid},
		agentcapabilities.AcknowledgeFindingCapabilityName: {
			agentcapabilities.AgentErrCodeInvalidFindingRequest,
			agentcapabilities.AgentErrCodeFindingNotFound,
			agentcapabilities.AgentErrCodeFindingActionNotAllowed,
			agentcapabilities.AgentErrCodePatrolUnavailable,
		},
		agentcapabilities.SnoozeFindingCapabilityName: {
			agentcapabilities.AgentErrCodeInvalidFindingRequest,
			agentcapabilities.AgentErrCodeFindingNotFound,
			agentcapabilities.AgentErrCodeFindingActionNotAllowed,
			agentcapabilities.AgentErrCodePatrolUnavailable,
		},
		agentcapabilities.DismissFindingCapabilityName: {
			agentcapabilities.AgentErrCodeInvalidFindingRequest,
			agentcapabilities.AgentErrCodeFindingNotFound,
			agentcapabilities.AgentErrCodeFindingActionNotAllowed,
			agentcapabilities.AgentErrCodePatrolUnavailable,
		},
		agentcapabilities.ResolveFindingCapabilityName: {
			agentcapabilities.AgentErrCodeInvalidFindingRequest,
			agentcapabilities.AgentErrCodeFindingNotFound,
			agentcapabilities.AgentErrCodeFindingActionNotAllowed,
			agentcapabilities.AgentErrCodePatrolUnavailable,
		},
	}
	byName := map[string]AgentCapability{}
	manifest := agentcapabilities.CanonicalManifest()
	for _, cap := range manifest.Capabilities {
		byName[cap.Name] = cap
	}
	for name, expected := range wantErrorCodes {
		cap, ok := byName[name]
		if !ok {
			t.Errorf("capability %q missing from manifest", name)
			continue
		}
		for _, code := range expected {
			found := false
			for _, declared := range cap.ErrorCodes {
				if declared == code {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("capability %q must declare error code %q so agents can branch on it; declared codes: %v", name, code, cap.ErrorCodes)
			}
		}
	}
}
