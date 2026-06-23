package agentcapabilities

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testPulseMCPSurfaceToolContracts(toolNames ...string) []SurfaceToolContract {
	return []SurfaceToolContract{
		{
			SurfaceID:   SurfaceIDPulseMCP,
			ToolSource:  SurfaceToolSourceCapabilityManifest,
			ToolNames:   append([]string(nil), toolNames...),
			Affordances: DefaultSurfaceAffordancesForID(SurfaceIDPulseMCP),
		},
	}
}

func TestToolResultHelpersBuildSharedEnvelope(t *testing.T) {
	empty := EmptyToolResult()
	if empty.Content == nil {
		t.Fatal("EmptyToolResult must initialize content")
	}
	if len(empty.Content) != 0 {
		t.Fatalf("EmptyToolResult content = %#v, want empty list", empty.Content)
	}

	text := NewToolTextResult("ok")
	if text.IsError {
		t.Fatal("NewToolTextResult must default to success")
	}
	if len(text.Content) != 1 || text.Content[0].Type != "text" || text.Content[0].Text != "ok" {
		t.Fatalf("unexpected text result: %+v", text)
	}

	failed := NewToolErrorResult(errors.New("nope"))
	if !failed.IsError {
		t.Fatal("NewToolErrorResult must mark the result as an error")
	}
	if failed.Content[0].Text != "nope" {
		t.Fatalf("error text = %q, want nope", failed.Content[0].Text)
	}

	payload := NewToolJSONResultWithIsError(map[string]any{"error": "policy_blocked"}, true)
	if !payload.IsError {
		t.Fatal("explicit error JSON result must preserve isError=true")
	}
	if !strings.Contains(payload.Content[0].Text, `"error":"policy_blocked"`) {
		t.Fatalf("JSON result did not marshal payload: %+v", payload)
	}
	if payload.StructuredContent["error"] != "policy_blocked" {
		t.Fatalf("JSON result structuredContent = %+v, want policy_blocked error", payload.StructuredContent)
	}
	wire, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal tool result: %v", err)
	}
	if !strings.Contains(string(wire), `"structuredContent":{"error":"policy_blocked"}`) {
		t.Fatalf("JSON result must expose structuredContent in the shared wire envelope: %s", string(wire))
	}
}

func TestToolResultNormalizeCollectionsDetachesContent(t *testing.T) {
	sourceContent := []ToolContent{{Type: "text", Text: "ok"}}
	sourceStructured := map[string]any{
		"nested": map[string]any{"value": "original"},
		"items":  []any{map[string]any{"value": "first"}},
	}
	normalized := (ToolResult{Content: sourceContent, StructuredContent: sourceStructured}).NormalizeCollections()
	if normalized.Content == nil {
		t.Fatal("NormalizeCollections must initialize content")
	}
	if normalized.Content[0].Text != "ok" {
		t.Fatalf("normalized content = %+v", normalized.Content)
	}

	normalized.Content[0].Text = "changed"
	if sourceContent[0].Text != "ok" {
		t.Fatalf("NormalizeCollections returned aliased content: source=%+v normalized=%+v", sourceContent, normalized.Content)
	}

	normalized.StructuredContent["nested"].(map[string]any)["value"] = "changed"
	normalized.StructuredContent["items"].([]any)[0].(map[string]any)["value"] = "changed"
	if sourceStructured["nested"].(map[string]any)["value"] != "original" {
		t.Fatalf("NormalizeCollections returned aliased structured nested map: source=%+v normalized=%+v", sourceStructured, normalized.StructuredContent)
	}
	if sourceStructured["items"].([]any)[0].(map[string]any)["value"] != "first" {
		t.Fatalf("NormalizeCollections returned aliased structured nested list: source=%+v normalized=%+v", sourceStructured, normalized.StructuredContent)
	}

	empty := (ToolResult{}).NormalizeCollections()
	if empty.Content == nil {
		t.Fatal("NormalizeCollections must preserve empty content as [] rather than nil")
	}
}

func TestToolHTTPTextResultMarksNon2xxAsError(t *testing.T) {
	ok := NewToolHTTPTextResult([]byte(`{"ok":true}`), 200)
	if ok.IsError {
		t.Fatal("2xx upstream response must become isError=false")
	}
	if ok.Content[0].Text != `{"ok":true}` {
		t.Fatalf("2xx content = %q", ok.Content[0].Text)
	}
	if ok.StructuredContent["ok"] != true {
		t.Fatalf("2xx structuredContent = %+v, want ok=true", ok.StructuredContent)
	}

	errResult := NewToolHTTPTextResult([]byte(`{"error":"resource_not_found"}`), 404)
	if !errResult.IsError {
		t.Fatal("non-2xx upstream response must become isError=true")
	}
	if errResult.Content[0].Text != `{"error":"resource_not_found"}` {
		t.Fatalf("non-2xx content = %q", errResult.Content[0].Text)
	}
	if errResult.StructuredContent["error"] != "resource_not_found" {
		t.Fatalf("non-2xx structuredContent = %+v, want resource_not_found error", errResult.StructuredContent)
	}

	arrayResult := NewToolHTTPTextResult([]byte(`[{"ok":true},{"ok":false}]`), 200)
	if arrayResult.Content[0].Text != `[{"ok":true},{"ok":false}]` {
		t.Fatalf("array content text changed: %q", arrayResult.Content[0].Text)
	}
	if arrayResult.StructuredContent["count"] != 2 {
		t.Fatalf("array structuredContent count = %+v, want 2", arrayResult.StructuredContent)
	}
	items, itemsOK := arrayResult.StructuredContent["items"].([]any)
	if !itemsOK || len(items) != 2 {
		t.Fatalf("array structuredContent items = %+v, want two items", arrayResult.StructuredContent)
	}
	first, _ := items[0].(map[string]any)
	if first["ok"] != true {
		t.Fatalf("array structuredContent first item = %+v, want ok=true", first)
	}

	for _, body := range [][]byte{
		[]byte(`plain text`),
		[]byte(`{"ok":true}{"extra":true}`),
	} {
		result := NewToolHTTPTextResult(body, 200)
		if result.StructuredContent != nil {
			t.Fatalf("body %q produced structuredContent %+v, want text-only result", string(body), result.StructuredContent)
		}
	}
}

func TestCapabilityHTTPToolResultUsesSharedHTTPResponseState(t *testing.T) {
	ok := NewCapabilityHTTPToolResult(HTTPCallResponse{
		Method:     "GET",
		Path:       "/api/agent/fleet-context",
		StatusCode: 202,
		Body:       []byte(`{"ok":true}`),
	})
	if ok.IsError {
		t.Fatal("2xx capability HTTP response must become isError=false")
	}
	if ok.Content[0].Text != `{"ok":true}` {
		t.Fatalf("2xx content = %q", ok.Content[0].Text)
	}
	if ok.StructuredContent["ok"] != true {
		t.Fatalf("2xx structuredContent = %+v, want ok=true", ok.StructuredContent)
	}

	failed := NewCapabilityHTTPToolResult(HTTPCallResponse{
		Method:     "GET",
		Path:       "/api/agent/resource-context/vm%3A101",
		StatusCode: 403,
		Body:       []byte(`{"error":"scope_required","message":"monitoring:read is required"}`),
	})
	if !failed.IsError {
		t.Fatal("non-2xx capability HTTP response must become isError=true")
	}
	if failed.Content[0].Text != `{"error":"scope_required","message":"monitoring:read is required"}` {
		t.Fatalf("non-2xx content = %q", failed.Content[0].Text)
	}
	if failed.StructuredContent["error"] != "scope_required" {
		t.Fatalf("non-2xx structuredContent = %+v, want scope_required error", failed.StructuredContent)
	}
}

func TestToolResultTextFlattensTextBlocks(t *testing.T) {
	result := ToolResult{
		Content: []ToolContent{
			{Type: "text", Text: "first"},
			{Type: "resource", URI: "file://ignored"},
			{Type: "text"},
			{Type: "text", Text: "second"},
		},
	}
	if got := ToolResultText(result); got != "first\nsecond" {
		t.Fatalf("ToolResultText returned %q", got)
	}

	if got := ToolResultText(ToolResult{}); got != "" {
		t.Fatalf("empty ToolResultText returned %q, want empty", got)
	}
}

func TestInterpretToolResultFlattensTextAndDetectsSharedMarkers(t *testing.T) {
	approval := InterpretToolResult(ToolResult{
		Content: []ToolContent{
			NewToolTextContent(ApprovalRequiredToolMarker("qm start 101", "tool-1", "approval required", "approval-1", "Approve it.")),
		},
	})
	if approval.Text == "" || !approval.ApprovalRequired || approval.PolicyBlocked || approval.IsError {
		t.Fatalf("approval interpretation = %+v", approval)
	}

	policy := InterpretToolResult(ToolResult{
		Content: []ToolContent{
			NewToolTextContent(PolicyBlockedToolMarker("rm -rf /", "blocked by policy")),
		},
		IsError: true,
	})
	if policy.Text == "" || !policy.PolicyBlocked || policy.ApprovalRequired || !policy.IsError {
		t.Fatalf("policy interpretation = %+v", policy)
	}

	plain := InterpretToolResult(ToolResult{
		Content: []ToolContent{
			NewToolTextContent("first"),
			{Type: "resource", URI: "file://ignored"},
			NewToolTextContent("second"),
		},
	})
	if plain.Text != "first\nsecond" || plain.ApprovalRequired || plain.PolicyBlocked || plain.IsError {
		t.Fatalf("plain interpretation = %+v", plain)
	}
}

func TestMCPToolResultWireAliasesUseNeutralHelpers(t *testing.T) {
	result := MCPToolResult{
		Content: []MCPContent{
			NewToolTextContent("ok"),
		},
	}
	blocked := NewToolBlockedError(ErrCodeActionNotAllowed, "not available here", nil)
	neutral := NewToolResponseResult(blocked)
	if !neutral.IsError || ToolResultText(neutral) == "" {
		t.Fatalf("neutral tool response result = %+v", neutral)
	}
	if got := ToolResultText(result); got != "ok" {
		t.Fatalf("MCP wire alias text = %q", got)
	}
	structured := NewToolJSONResult(map[string]any{"ok": true})
	alias := MCPToolResult(structured)
	wire, err := json.Marshal(alias)
	if err != nil {
		t.Fatalf("marshal MCP alias: %v", err)
	}
	if !strings.Contains(string(wire), `"structuredContent":{"ok":true}`) {
		t.Fatalf("MCP alias must carry shared structuredContent on the wire: %s", string(wire))
	}
	interpreted := InterpretToolResult(result)
	if interpreted.Text != "ok" || interpreted.IsError || interpreted.ApprovalRequired || interpreted.PolicyBlocked {
		t.Fatalf("MCP wire alias interpretation = %+v", interpreted)
	}
}

func TestInterpretDirectToolExecutionMapsSharedOutcomes(t *testing.T) {
	opts := DirectToolExecutionOptions{
		FailurePrefix:           "command execution failed",
		ApprovalRequiredMessage: "command requires approval (unexpected in autonomous mode)",
		PolicyBlockedMessage:    "command blocked by security policy",
	}

	failed, err := InterpretDirectToolExecution(ToolResult{
		Content: []ToolContent{NewToolTextContent("stderr")},
		IsError: true,
	}, opts)
	if err == nil || err.Error() != "command execution failed: stderr" {
		t.Fatalf("failure error = %v", err)
	}
	if failed.OutputText != "stderr" || !failed.Interpretation.IsError {
		t.Fatalf("failure outcome = %+v", failed)
	}

	approval, err := InterpretDirectToolExecution(ToolResult{
		Content: []ToolContent{
			NewToolTextContent(ApprovalRequiredToolMarker("qm start 101", "tool-1", "approval required", "approval-1", "Approve it.")),
		},
	}, opts)
	if err == nil || err.Error() != "command requires approval (unexpected in autonomous mode)" {
		t.Fatalf("approval error = %v", err)
	}
	if approval.OutputText != "" || !approval.Interpretation.ApprovalRequired {
		t.Fatalf("approval outcome = %+v", approval)
	}

	policy, err := InterpretDirectToolExecution(ToolResult{
		Content: []ToolContent{
			NewToolTextContent(PolicyBlockedToolMarker("rm -rf /", "blocked by policy")),
		},
	}, opts)
	if err == nil || err.Error() != "command blocked by security policy" {
		t.Fatalf("policy error = %v", err)
	}
	if policy.OutputText != "" || !policy.Interpretation.PolicyBlocked {
		t.Fatalf("policy outcome = %+v", policy)
	}

	success, err := InterpretDirectToolExecution(ToolResult{
		Content: []ToolContent{
			NewToolTextContent("first"),
			{Type: "resource", URI: "file://ignored"},
			NewToolTextContent("second"),
		},
	}, opts)
	if err != nil || success.OutputText != "first\nsecond" {
		t.Fatalf("success outcome = %+v err=%v", success, err)
	}
}

func TestNewProviderToolResultFromToolResultUsesSharedInterpretation(t *testing.T) {
	result := NewProviderToolResultFromToolResult("call-1", ToolResult{
		Content: []ToolContent{
			NewToolTextContent("first"),
			{Type: "resource", URI: "file://ignored"},
			NewToolTextContent("second"),
		},
		IsError: true,
	})

	if result.ToolUseID != "call-1" || result.Content != "first\nsecond" || !result.IsError {
		t.Fatalf("provider result = %+v", result)
	}
}

func TestNewProviderToolResultHelpersBuildProviderShape(t *testing.T) {
	result := NewProviderToolResult("call-1", "ok", false)
	if result.ToolUseID != "call-1" || result.Content != "ok" || result.IsError {
		t.Fatalf("provider result = %+v", result)
	}

	failed := NewProviderToolErrorResult("call-2", "failed")
	if failed.ToolUseID != "call-2" || failed.Content != "failed" || !failed.IsError {
		t.Fatalf("provider error result = %+v", failed)
	}
}

func TestJSONRPCHelpersPreserveProtocolAndNotificationShape(t *testing.T) {
	resp := NewJSONRPCErrorResponse(json.RawMessage(`"abc"`), JSONRPCErrorMethodNotFound, "missing", nil)
	if resp.JSONRPC != JSONRPCVersion {
		t.Fatalf("JSONRPC = %q, want %q", resp.JSONRPC, JSONRPCVersion)
	}
	if string(resp.ID) != `"abc"` {
		t.Fatalf("response id = %s", resp.ID)
	}
	if resp.Error == nil || resp.Error.Code != JSONRPCErrorMethodNotFound || resp.Error.Message != "missing" {
		t.Fatalf("unexpected error response: %+v", resp)
	}

	notification := NewJSONRPCNotification(MCPNotificationMethod("finding.created"), json.RawMessage(`{"findingId":"f1"}`))
	wire, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}
	body := string(wire)
	if !strings.Contains(body, `"jsonrpc":"2.0"`) || !strings.Contains(body, `"method":"notifications/finding.created"`) {
		t.Fatalf("notification missing protocol or method: %s", body)
	}
	if strings.Contains(body, `"id"`) {
		t.Fatalf("notification must not carry an id: %s", body)
	}
}

func TestJSONRPCRequestDecodeAndResponsePolicyUseSharedSemantics(t *testing.T) {
	req, err := DecodeJSONRPCRequest([]byte(`{"jsonrpc":"2.0","id":"turn-1","method":"tools/list"}`))
	if err != nil {
		t.Fatalf("DecodeJSONRPCRequest: %v", err)
	}
	if req.Method != MCPMethodToolsList || string(req.ID) != `"turn-1"` {
		t.Fatalf("decoded request = %+v", req)
	}
	if !JSONRPCRequestExpectsResponse(req) {
		t.Fatal("request with string id must expect a response")
	}

	if JSONRPCRequestExpectsResponse(JSONRPCRequest{Method: "notifications/initialized"}) {
		t.Fatal("request without id must be treated as a notification")
	}
	if JSONRPCRequestExpectsResponse(JSONRPCRequest{ID: json.RawMessage(`null`), Method: "notifications/initialized"}) {
		t.Fatal("request with null id must be treated as a notification")
	}
	if !JSONRPCRequestExpectsResponse(JSONRPCRequest{ID: json.RawMessage(`0`), Method: MCPMethodPing}) {
		t.Fatal("request with numeric zero id must still expect a response")
	}

	_, err = DecodeJSONRPCRequest([]byte(`{"jsonrpc":"2.0","method":`))
	if err == nil || !strings.Contains(err.Error(), "malformed JSON-RPC request") {
		t.Fatalf("decode error = %v, want stable malformed JSON-RPC message", err)
	}
	parseResp := NewJSONRPCParseErrorResponse(err)
	if parseResp.JSONRPC != JSONRPCVersion || parseResp.Error == nil || parseResp.Error.Code != JSONRPCErrorParse {
		t.Fatalf("parse error response = %+v", parseResp)
	}
	if !strings.Contains(parseResp.Error.Message, "malformed JSON-RPC request") {
		t.Fatalf("parse error message = %q", parseResp.Error.Message)
	}
}

func TestServeJSONRPCLinesDispatchesResponsesAndSuppressesNotifications(t *testing.T) {
	input := strings.NewReader("\n" +
		`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":` + "\n")

	var methods []string
	var responses []JSONRPCResponse
	err := ServeJSONRPCLines(
		context.Background(),
		input,
		func(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
			methods = append(methods, req.Method)
			return NewJSONRPCResponse(req.ID, map[string]any{"method": req.Method})
		},
		func(resp JSONRPCResponse) error {
			responses = append(responses, resp)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ServeJSONRPCLines: %v", err)
	}
	if len(methods) != 2 || methods[0] != "ping" || methods[1] != "notifications/initialized" {
		t.Fatalf("dispatched methods = %#v", methods)
	}
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want ping response and parse error", len(responses))
	}
	if responses[0].Error != nil || string(responses[0].ID) != "1" {
		t.Fatalf("first response = %+v, want successful ping response", responses[0])
	}
	if responses[1].Error == nil || responses[1].Error.Code != JSONRPCErrorParse {
		t.Fatalf("second response = %+v, want shared parse error", responses[1])
	}
}

func TestWriteJSONRPCMessageUsesStableEncoderSettings(t *testing.T) {
	var out strings.Builder
	if err := WriteJSONRPCMessage(&out, NewJSONRPCResponse(json.RawMessage(`"id-1"`), map[string]string{
		"html": "<tag>&value",
	})); err != nil {
		t.Fatalf("WriteJSONRPCMessage: %v", err)
	}
	body := out.String()
	if !strings.Contains(body, `"jsonrpc":"2.0"`) || !strings.Contains(body, `"<tag>&value"`) {
		t.Fatalf("encoded message did not preserve protocol or unescaped text: %s", body)
	}
}

func TestNewMCPEventNotificationProjectsOnlyProductEvents(t *testing.T) {
	notification, ok := NewMCPEventNotification(string(EventKindFindingCreated), json.RawMessage(`{"findingId":"f1"}`))
	if !ok {
		t.Fatal("finding.created with data must become an MCP notification")
	}
	if notification.JSONRPC != JSONRPCVersion {
		t.Fatalf("JSONRPC = %q, want %q", notification.JSONRPC, JSONRPCVersion)
	}
	if notification.Method != "notifications/finding.created" {
		t.Fatalf("method = %q, want notifications/finding.created", notification.Method)
	}
	if string(notification.Params) != `{"findingId":"f1"}` {
		t.Fatalf("params = %s, want raw SSE data", notification.Params)
	}
	if len(notification.ID) != 0 {
		t.Fatalf("notification must not carry id: %+v", notification)
	}

	custom, ok := NewMCPEventNotification("custom.product.event", json.RawMessage(`{"x":1}`))
	if !ok {
		t.Fatal("unknown non-transport product events must still project")
	}
	if custom.Method != "notifications/custom.product.event" {
		t.Fatalf("custom method = %q", custom.Method)
	}

	for _, tc := range []struct {
		name   string
		kind   string
		params json.RawMessage
	}{
		{"empty kind", "", json.RawMessage(`{"x":1}`)},
		{"empty params", string(EventKindFindingCreated), nil},
		{"stream connected", string(EventKindStreamConnected), json.RawMessage(`{}`)},
		{"heartbeat", string(EventKindHeartbeat), json.RawMessage(`{}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := NewMCPEventNotification(tc.kind, tc.params); ok {
				t.Fatalf("NewMCPEventNotification(%q, %s) = %+v, true; want false", tc.kind, tc.params, got)
			}
		})
	}
}

func TestNewMCPToolServerInitializeResultAdvertisesToolsAndOptionalPulseNotifications(t *testing.T) {
	disabled := NewMCPToolServerInitializeResult("pulse-mcp", "0.1.0", false)
	if disabled.ProtocolVersion != MCPProtocolVersion {
		t.Fatalf("protocolVersion = %q, want %q", disabled.ProtocolVersion, MCPProtocolVersion)
	}
	if disabled.ServerInfo.Name != "pulse-mcp" || disabled.ServerInfo.Version != "0.1.0" {
		t.Fatalf("serverInfo = %+v", disabled.ServerInfo)
	}
	if disabled.Capabilities.Tools == nil {
		t.Fatal("initialize result must advertise tools capability")
	}
	if len(disabled.Capabilities.Experimental) != 0 {
		t.Fatalf("notifications disabled must not advertise experimental capabilities: %+v", disabled.Capabilities.Experimental)
	}
	if !strings.Contains(disabled.Instructions, "governed infrastructure-operations surface") ||
		!strings.Contains(disabled.Instructions, "plan, approval, and execute flow") {
		t.Fatalf("initialize result must advertise shared Pulse Intelligence operating instructions, got %q", disabled.Instructions)
	}
	if !strings.Contains(disabled.Instructions, "This surface is Pulse MCP.") ||
		!strings.Contains(disabled.Instructions, "Use offered tools as the source of truth") {
		t.Fatalf("generic MCP initialize instructions must describe the tool-only surface, got %q", disabled.Instructions)
	}
	for _, forbidden := range []string{"resources", "prompts", "capability metadata"} {
		if strings.Contains(disabled.Instructions, forbidden) {
			t.Fatalf("generic MCP initialize instructions must not advertise unsupported %q: %q", forbidden, disabled.Instructions)
		}
	}

	enabled := NewMCPToolServerInitializeResult("pulse-mcp", "0.1.0", true)
	exp := enabled.Capabilities.Experimental
	if len(exp) == 0 {
		t.Fatal("notifications enabled must advertise experimental capabilities")
	}
	pulseNotifications, ok := exp[MCPPulseNotificationsExperimentalKey].(MCPPulseNotificationsCapability)
	if !ok {
		t.Fatalf("experimental[%q] = %T, want MCPPulseNotificationsCapability", MCPPulseNotificationsExperimentalKey, exp[MCPPulseNotificationsExperimentalKey])
	}
	if len(pulseNotifications.Kinds) == 0 {
		t.Fatal("Pulse notification capability must list actionable event kinds")
	}
	for _, want := range AgentActionableEventKinds() {
		found := false
		for _, got := range pulseNotifications.Kinds {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Pulse notification capability missing %q: %+v", want, pulseNotifications.Kinds)
		}
	}
}

func TestNewMCPManifestToolServerInitializeResultAdvertisesResourcesWhenContextCapabilitiesExist(t *testing.T) {
	capabilities := []Capability{
		{Name: FleetContextCapabilityName, Path: FleetContextCapabilityPath, Method: http.MethodGet},
		{Name: ResourceContextCapabilityName, Path: ResourceContextCapabilityPath, Method: http.MethodGet},
		{Name: EventSubscriptionCapabilityName, Path: "/api/agent/events", Method: http.MethodGet},
	}
	workflowPrompts := ProjectPulseWorkflowPrompts(capabilities)

	result := NewMCPManifestToolServerInitializeResult("pulse-mcp", "0.1.0", false, Manifest{
		SurfaceContract:      CanonicalManifest().SurfaceContract,
		SurfaceToolContracts: testPulseMCPSurfaceToolContracts(FleetContextCapabilityName, ResourceContextCapabilityName),
		Capabilities:         capabilities,
		WorkflowPrompts:      workflowPrompts,
	})
	if result.Capabilities.Tools == nil {
		t.Fatal("manifest-backed initialize result must keep tools capability")
	}
	if result.Capabilities.Resources == nil {
		t.Fatal("manifest-backed initialize result must advertise resources when context capabilities exist")
	}
	if result.Capabilities.Prompts == nil {
		t.Fatal("manifest-backed initialize result must advertise prompts when workflow capabilities exist")
	}
	if !strings.Contains(result.Instructions, "Use offered tools, resources, prompts, and capability metadata as the source of truth") {
		t.Fatalf("manifest-backed initialize instructions must name advertised resource/prompt/metadata affordances, got %q", result.Instructions)
	}
	for _, expected := range []string{
		"Pulse MCP is the external-agent adapter over Pulse Intelligence Core",
		"Pulse Assistant is the native Pulse surface on the same core",
		"Surface affordances: Pulse MCP exposes tools, resources, prompts, and capability metadata.",
		"Pulse Patrol is the primary built-in operator on Pulse Intelligence Core",
	} {
		if !strings.Contains(result.Instructions, expected) {
			t.Fatalf("manifest-backed initialize instructions must include surface contract line %q, got %q", expected, result.Instructions)
		}
	}

	withoutResourceContext := NewMCPManifestToolServerInitializeResult("pulse-mcp", "0.1.0", false, Manifest{
		Capabilities:    capabilities[:1],
		WorkflowPrompts: ProjectPulseWorkflowPrompts(capabilities[:1]),
	})
	if withoutResourceContext.Capabilities.Tools != nil {
		t.Fatalf("initialize result must not advertise tools without a published surface tool contract: %+v", withoutResourceContext.Capabilities.Tools)
	}
	if withoutResourceContext.Capabilities.Resources != nil {
		t.Fatalf("initialize result must not advertise resources without both context capabilities: %+v", withoutResourceContext.Capabilities.Resources)
	}
	if withoutResourceContext.Capabilities.Prompts == nil {
		t.Fatal("fleet-context-only manifests should still advertise the fleet triage prompt")
	}
	if strings.Contains(withoutResourceContext.Instructions, "resources") {
		t.Fatalf("initialize instructions must not advertise resources when resources capability is absent: %q", withoutResourceContext.Instructions)
	}
	if !strings.Contains(withoutResourceContext.Instructions, "prompts and capability metadata") {
		t.Fatalf("initialize instructions must keep prompt/metadata affordances when they are advertised, got %q", withoutResourceContext.Instructions)
	}
	if strings.Contains(withoutResourceContext.Instructions, "offered tools") {
		t.Fatalf("initialize instructions must not advertise tools without a published surface tool contract: %q", withoutResourceContext.Instructions)
	}
	if strings.Contains(withoutResourceContext.Instructions, "Surface contract") {
		t.Fatalf("initialize instructions must not invent a surface contract when the manifest lacks one: %q", withoutResourceContext.Instructions)
	}

	withoutPrompts := NewMCPManifestToolServerInitializeResult("pulse-mcp", "0.1.0", false, Manifest{
		Capabilities:    capabilities,
		WorkflowPrompts: []PulseWorkflowPrompt{},
	})
	if withoutPrompts.Capabilities.Prompts != nil {
		t.Fatalf("initialize result must not advertise prompts when workflowPrompts is explicitly empty: %+v", withoutPrompts.Capabilities.Prompts)
	}
}

func TestNewMCPManifestToolServerInitializeResultRequiresPublishedSurfaceToolContractForTools(t *testing.T) {
	capabilities := []Capability{
		{Name: FleetContextCapabilityName, Path: FleetContextCapabilityPath, Method: http.MethodGet},
		{Name: ResourceContextCapabilityName, Path: ResourceContextCapabilityPath, Method: http.MethodGet},
	}

	result := NewMCPManifestToolServerInitializeResult("pulse-mcp", "0.1.0", false, Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		Capabilities:    capabilities,
		WorkflowPrompts: ProjectPulseWorkflowPrompts(capabilities),
	})

	if result.Capabilities.Tools != nil {
		t.Fatalf("initialize result must not advertise tools without a published surface tool contract: %+v", result.Capabilities.Tools)
	}
	if result.Capabilities.Resources == nil {
		t.Fatal("resource affordance can still advertise when canonical context capabilities exist")
	}
	if strings.Contains(result.Instructions, "offered tools") {
		t.Fatalf("initialize instructions must not name offered tools without a published surface tool contract: %q", result.Instructions)
	}
	if !strings.Contains(result.Instructions, "resources") {
		t.Fatalf("initialize instructions must still name advertised non-tool affordances, got %q", result.Instructions)
	}
}

func TestMCPManifestToolServerHonorsSurfaceAffordanceGates(t *testing.T) {
	capabilities := []Capability{
		{Name: FleetContextCapabilityName, Path: FleetContextCapabilityPath, Method: http.MethodGet, Description: "triage"},
		{Name: ResourceContextCapabilityName, Path: ResourceContextCapabilityPath, Method: http.MethodGet, Description: "depth"},
		{Name: ListFindingsCapabilityName, Path: "/api/ai/patrol/findings", Method: http.MethodGet, Description: "findings"},
	}
	surfaceContract := CanonicalManifest().SurfaceContract
	for i := range surfaceContract.OperatorSurfaces {
		if surfaceContract.OperatorSurfaces[i].ID == SurfaceIDPulseMCP {
			surfaceContract.OperatorSurfaces[i].Affordances = SurfaceAffordanceContract{Tools: true}
		}
	}
	server := MCPManifestToolServer{
		ServerName:    "pulse-mcp",
		ServerVersion: "0.1.0",
		Manifest: Manifest{
			SurfaceContract:      surfaceContract,
			SurfaceToolContracts: testPulseMCPSurfaceToolContracts(ListFindingsCapabilityName),
			Capabilities:         capabilities,
			WorkflowPrompts:      ProjectPulseWorkflowPrompts(capabilities),
		},
	}

	init := server.Initialize()
	if init.Capabilities.Tools == nil {
		t.Fatal("surface with tool affordance must still advertise tools")
	}
	if init.Capabilities.Resources != nil || init.Capabilities.Prompts != nil {
		t.Fatalf("surface-disabled resources/prompts must not be advertised: %+v", init.Capabilities)
	}
	if strings.Contains(init.Instructions, "resources") ||
		strings.Contains(init.Instructions, "prompts") ||
		strings.Contains(init.Instructions, "capability metadata") {
		t.Fatalf("initialize instructions must not advertise disabled MCP affordances: %q", init.Instructions)
	}

	if _, err := server.ResourcesList(context.Background()); err == nil || !strings.Contains(err.Error(), "resources are not enabled") {
		t.Fatalf("ResourcesList disabled error = %v", err)
	}
	rawRead, err := json.Marshal(MCPReadResourceParams{URI: MCPResourceURI("vm:101")})
	if err != nil {
		t.Fatalf("marshal read params: %v", err)
	}
	if _, err := server.ResourcesRead(context.Background(), rawRead); err == nil || !strings.Contains(err.Error(), "resources are not enabled") {
		t.Fatalf("ResourcesRead disabled error = %v", err)
	}
	if prompts := server.PromptsList(); len(prompts.Prompts) != 0 {
		t.Fatalf("PromptsList disabled result = %+v, want empty list", prompts)
	}
	rawPrompt, err := json.Marshal(MCPGetPromptParams{Name: MCPPromptTriageFleet})
	if err != nil {
		t.Fatalf("marshal prompt params: %v", err)
	}
	if _, err := server.PromptsGet(context.Background(), rawPrompt); err == nil || !strings.Contains(err.Error(), "prompts are not enabled") {
		t.Fatalf("PromptsGet disabled error = %v", err)
	}
}

func TestDispatchMCPToolServerRequestRoutesMethodsAndErrors(t *testing.T) {
	initializeCount := 0
	handlerContent := []MCPContent{{Type: "text", Text: "ok"}}
	handlerResources := []MCPResource{{URI: MCPResourceURI("vm:101"), Name: "VM 101"}}
	handlerContents := []MCPResourceContent{{URI: MCPResourceURI("vm:101"), MimeType: MCPResourceContextMIMEType, Text: `{"canonicalId":"vm:101"}`}}
	handlerPrompts := []MCPPrompt{{Name: MCPPromptTriageFleet, Description: "triage"}}
	handlerPromptMessages := []MCPPromptMessage{{Role: "user", Content: NewToolTextContent("triage now")}}
	handlers := MCPToolServerHandlers{
		Initialize: func() MCPInitializeResult {
			return NewMCPToolServerInitializeResult("pulse-mcp", "0.1.0", false)
		},
		ToolsList: func() MCPProjectedToolsResult {
			return MCPProjectedToolsResult{Tools: []ProjectedTool{{Name: "get_fleet_context"}}}
		},
		ToolsCall: func(ctx context.Context, raw json.RawMessage) (MCPToolResult, error) {
			if string(raw) != `{"name":"get_fleet_context"}` {
				t.Fatalf("tools/call raw params = %s", raw)
			}
			return MCPToolResult{Content: handlerContent}, nil
		},
		ResourcesList: func(ctx context.Context) (MCPListResourcesResult, error) {
			return MCPListResourcesResult{Resources: handlerResources}, nil
		},
		ResourcesRead: func(ctx context.Context, raw json.RawMessage) (MCPReadResourceResult, error) {
			if string(raw) != `{"uri":"`+MCPResourceURI("vm:101")+`"}` {
				t.Fatalf("resources/read raw params = %s", raw)
			}
			return MCPReadResourceResult{Contents: handlerContents}, nil
		},
		PromptsList: func() MCPListPromptsResult {
			return MCPListPromptsResult{Prompts: handlerPrompts}
		},
		PromptsGet: func(ctx context.Context, raw json.RawMessage) (MCPGetPromptResult, error) {
			if string(raw) != `{"name":"`+MCPPromptTriageFleet+`"}` {
				t.Fatalf("prompts/get raw params = %s", raw)
			}
			return MCPGetPromptResult{Description: "Triage", Messages: handlerPromptMessages}, nil
		},
		OnInitialize: func() {
			initializeCount++
		},
	}

	initResp := DispatchMCPToolServerRequest(context.Background(), JSONRPCRequest{
		ID:     json.RawMessage(`1`),
		Method: MCPMethodInitialize,
	}, handlers)
	if initResp.Error != nil || initializeCount != 1 {
		t.Fatalf("initialize response = %+v initializeCount=%d", initResp, initializeCount)
	}
	if _, ok := initResp.Result.(MCPInitializeResult); !ok {
		t.Fatalf("initialize result type = %T, want MCPInitializeResult", initResp.Result)
	}

	listResp := DispatchMCPToolServerRequest(context.Background(), JSONRPCRequest{
		ID:     json.RawMessage(`2`),
		Method: MCPMethodToolsList,
	}, handlers)
	listResult, ok := listResp.Result.(MCPProjectedToolsResult)
	if listResp.Error != nil || !ok || len(listResult.Tools) != 1 {
		t.Fatalf("tools/list response = %+v", listResp)
	}

	callResp := DispatchMCPToolServerRequest(context.Background(), JSONRPCRequest{
		ID:     json.RawMessage(`3`),
		Method: MCPMethodToolsCall,
		Params: json.RawMessage(`{"name":"get_fleet_context"}`),
	}, handlers)
	callResult, ok := callResp.Result.(MCPToolResult)
	if callResp.Error != nil || !ok || ToolResultText(callResult) != "ok" {
		t.Fatalf("tools/call response = %+v", callResp)
	}
	callResult.Content[0].Text = "changed"
	if handlerContent[0].Text != "ok" {
		t.Fatalf("tools/call response must detach handler content: source=%+v result=%+v", handlerContent, callResult.Content)
	}

	resourcesListResp := DispatchMCPToolServerRequest(context.Background(), JSONRPCRequest{
		ID:     json.RawMessage(`7`),
		Method: MCPMethodResourcesList,
	}, handlers)
	resourcesList, ok := resourcesListResp.Result.(MCPListResourcesResult)
	if resourcesListResp.Error != nil || !ok || len(resourcesList.Resources) != 1 {
		t.Fatalf("resources/list response = %+v", resourcesListResp)
	}
	resourcesList.Resources[0].Name = "changed"
	if handlerResources[0].Name != "VM 101" {
		t.Fatalf("resources/list response must detach handler resources: source=%+v result=%+v", handlerResources, resourcesList.Resources)
	}

	resourcesReadResp := DispatchMCPToolServerRequest(context.Background(), JSONRPCRequest{
		ID:     json.RawMessage(`8`),
		Method: MCPMethodResourcesRead,
		Params: json.RawMessage(`{"uri":"` + MCPResourceURI("vm:101") + `"}`),
	}, handlers)
	resourcesRead, ok := resourcesReadResp.Result.(MCPReadResourceResult)
	if resourcesReadResp.Error != nil || !ok || len(resourcesRead.Contents) != 1 {
		t.Fatalf("resources/read response = %+v", resourcesReadResp)
	}
	resourcesRead.Contents[0].Text = "changed"
	if handlerContents[0].Text != `{"canonicalId":"vm:101"}` {
		t.Fatalf("resources/read response must detach handler contents: source=%+v result=%+v", handlerContents, resourcesRead.Contents)
	}

	promptsListResp := DispatchMCPToolServerRequest(context.Background(), JSONRPCRequest{
		ID:     json.RawMessage(`9`),
		Method: MCPMethodPromptsList,
	}, handlers)
	promptsList, ok := promptsListResp.Result.(MCPListPromptsResult)
	if promptsListResp.Error != nil || !ok || len(promptsList.Prompts) != 1 {
		t.Fatalf("prompts/list response = %+v", promptsListResp)
	}
	promptsList.Prompts[0].Description = "changed"
	if handlerPrompts[0].Description != "triage" {
		t.Fatalf("prompts/list response must detach handler prompts: source=%+v result=%+v", handlerPrompts, promptsList.Prompts)
	}

	promptsGetResp := DispatchMCPToolServerRequest(context.Background(), JSONRPCRequest{
		ID:     json.RawMessage(`10`),
		Method: MCPMethodPromptsGet,
		Params: json.RawMessage(`{"name":"` + MCPPromptTriageFleet + `"}`),
	}, handlers)
	promptResult, ok := promptsGetResp.Result.(MCPGetPromptResult)
	if promptsGetResp.Error != nil || !ok || len(promptResult.Messages) != 1 {
		t.Fatalf("prompts/get response = %+v", promptsGetResp)
	}
	promptResult.Messages[0].Content.Text = "changed"
	if handlerPromptMessages[0].Content.Text != "triage now" {
		t.Fatalf("prompts/get response must detach handler messages: source=%+v result=%+v", handlerPromptMessages, promptResult.Messages)
	}

	pingResp := DispatchMCPToolServerRequest(context.Background(), JSONRPCRequest{
		ID:     json.RawMessage(`4`),
		Method: MCPMethodPing,
	}, handlers)
	if pingResp.Error != nil {
		t.Fatalf("ping response = %+v", pingResp)
	}
	if _, ok := pingResp.Result.(map[string]any); !ok {
		t.Fatalf("ping result type = %T, want map[string]any", pingResp.Result)
	}

	callError := DispatchMCPToolServerRequest(context.Background(), JSONRPCRequest{
		ID:     json.RawMessage(`5`),
		Method: MCPMethodToolsCall,
	}, MCPToolServerHandlers{
		ToolsCall: func(ctx context.Context, raw json.RawMessage) (MCPToolResult, error) {
			return MCPToolResult{}, errors.New("failed")
		},
	})
	if callError.Error == nil || callError.Error.Code != JSONRPCErrorInternal || callError.Error.Message != "failed" {
		t.Fatalf("tools/call error response = %+v", callError)
	}

	unknown := DispatchMCPToolServerRequest(context.Background(), JSONRPCRequest{
		ID:     json.RawMessage(`6`),
		Method: "missing/method",
	}, handlers)
	if unknown.Error == nil || unknown.Error.Code != JSONRPCErrorMethodNotFound {
		t.Fatalf("unknown method response = %+v", unknown)
	}
}

func TestMCPResourceURIEncodesAndValidatesPulseResourceIDs(t *testing.T) {
	resourceID := "container:web/api"
	uri := MCPResourceURI(resourceID)
	if !strings.HasPrefix(uri, "pulse://resource/") {
		t.Fatalf("resource URI = %q, want pulse://resource/ prefix", uri)
	}
	got, err := ResourceIDFromMCPResourceURI(uri)
	if err != nil {
		t.Fatalf("ResourceIDFromMCPResourceURI(%q): %v", uri, err)
	}
	if got != resourceID {
		t.Fatalf("decoded resource id = %q, want %q", got, resourceID)
	}

	for _, rawURI := range []string{
		"",
		"https://pulse.example/api/agent/resource-context/vm:101",
		"pulse://resource/",
		"pulse://resource/vm:101?debug=true",
		"pulse://other/vm:101",
	} {
		if _, err := ResourceIDFromMCPResourceURI(rawURI); err == nil {
			t.Fatalf("ResourceIDFromMCPResourceURI(%q) unexpectedly succeeded", rawURI)
		}
	}
}

func TestMCPManifestToolServerProjectsManifestAndExecutesTools(t *testing.T) {
	type captured struct {
		method string
		path   string
		token  string
	}
	var got captured
	pulse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.path = r.URL.Path
		got.token = r.Header.Get(AgentAPITokenHeader)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":true}`))
	}))
	defer pulse.Close()

	server := MCPManifestToolServer{
		ServerName:             "pulse-mcp",
		ServerVersion:          "0.1.0",
		EmitPulseNotifications: true,
		Client:                 pulse.Client(),
		BaseURL:                pulse.URL,
		Token:                  "test-token",
		Manifest: Manifest{
			SurfaceContract:      CanonicalManifest().SurfaceContract,
			SurfaceToolContracts: testPulseMCPSurfaceToolContracts(ResourceContextCapabilityName),
			Capabilities: []Capability{
				{Name: ResourceContextCapabilityName, Path: ResourceContextCapabilityPath, Method: http.MethodGet, Description: "depth"},
				{Name: SetOperatorStateCapabilityName, Path: OperatorStateCapabilityPath, Method: http.MethodPut, Description: "intent"},
				{Name: "subscribe_events", Path: "/api/agent/events", Method: http.MethodGet, Description: "stream"},
			},
		},
	}

	init := server.Initialize()
	if init.ServerInfo.Name != "pulse-mcp" || init.Capabilities.Tools == nil || len(init.Capabilities.Experimental) == 0 {
		t.Fatalf("initialize result = %+v", init)
	}
	if !strings.Contains(init.Instructions, "Pulse MCP is the external-agent adapter over Pulse Intelligence Core") {
		t.Fatalf("initialize instructions must carry manifest surface contract, got %q", init.Instructions)
	}

	list := server.ToolsList()
	if len(list.Tools) != 1 || list.Tools[0].Name != ResourceContextCapabilityName {
		t.Fatalf("tools/list result = %+v", list)
	}

	blockedRaw, err := json.Marshal(MCPCallToolParams{
		Name:      SetOperatorStateCapabilityName,
		Arguments: map[string]any{ResourceIDArgumentName: "vm:101"},
	})
	if err != nil {
		t.Fatalf("marshal blocked params: %v", err)
	}
	if _, err := server.ToolsCall(context.Background(), blockedRaw); err == nil || !strings.Contains(err.Error(), "unknown tool: "+SetOperatorStateCapabilityName) {
		t.Fatalf("surface contract must block tools absent from Pulse MCP allowlist, err=%v", err)
	}

	raw, err := json.Marshal(MCPCallToolParams{
		Name:      ResourceContextCapabilityName,
		Arguments: map[string]any{ResourceIDArgumentName: "vm:101"},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	result, err := server.ToolsCall(context.Background(), raw)
	if err != nil {
		t.Fatalf("ToolsCall: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/api/agent/resource-context/vm:101" || got.token != "test-token" {
		t.Fatalf("upstream call = %+v", got)
	}
	if result.IsError || ToolResultText(result) != `{"accepted":true}` {
		t.Fatalf("tools/call result = %+v", result)
	}
	if result.StructuredContent["accepted"] != true {
		t.Fatalf("tools/call structuredContent = %+v, want accepted=true", result.StructuredContent)
	}

	onInitializeCount := 0
	handlers := server.Handlers(func() {
		onInitializeCount++
	})
	resp := DispatchMCPToolServerRequest(context.Background(), JSONRPCRequest{
		ID:     json.RawMessage(`1`),
		Method: MCPMethodInitialize,
	}, handlers)
	if resp.Error != nil || onInitializeCount != 1 {
		t.Fatalf("dispatch initialize response = %+v onInitialize=%d", resp, onInitializeCount)
	}
}

func TestMCPManifestToolServerProjectsResourcesFromContextCapabilities(t *testing.T) {
	type captured struct {
		paths []string
		token string
	}
	var got captured
	pulse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.paths = append(got.paths, r.URL.EscapedPath())
		got.token = r.Header.Get(AgentAPITokenHeader)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case "/api/agent/fleet-context":
			_, _ = w.Write([]byte(`{
				"resources": [
					{
						"canonicalId": "vm:101",
						"resourceType": "virtual-machine",
						"resourceName": "Database VM",
						"technology": "proxmox",
						"pendingApprovalCount": 1,
						"findings": { "total": 2, "critical": 1, "warning": 1, "info": 0 }
					},
					{ "canonicalId": "", "resourceName": "ignored" }
				],
				"generatedAt": "2026-06-18T08:00:00Z"
			}`))
		case "/api/agent/resource-context/vm%3A101":
			_, _ = w.Write([]byte(`{"canonicalId":"vm:101","resourceName":"Database VM"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer pulse.Close()

	server := MCPManifestToolServer{
		ServerName:    "pulse-mcp",
		ServerVersion: "0.1.0",
		Client:        pulse.Client(),
		BaseURL:       pulse.URL,
		Token:         "test-token",
		Manifest: Manifest{
			Capabilities: []Capability{
				{Name: FleetContextCapabilityName, Path: "/api/agent/fleet-context", Method: http.MethodGet, Description: "triage"},
				{Name: ResourceContextCapabilityName, Path: "/api/agent/resource-context/{resourceId}", Method: http.MethodGet, Description: "depth"},
			},
		},
	}

	init := server.Initialize()
	if init.Capabilities.Resources == nil {
		t.Fatalf("initialize result must advertise resources for context-capable manifest: %+v", init.Capabilities)
	}

	list, err := server.ResourcesList(context.Background())
	if err != nil {
		t.Fatalf("ResourcesList: %v", err)
	}
	if len(list.Resources) != 1 {
		t.Fatalf("resources/list = %+v, want one non-empty resource", list.Resources)
	}
	resource := list.Resources[0]
	if resource.URI != MCPResourceURI("vm:101") || resource.Name != "Database VM" || resource.MimeType != MCPResourceContextMIMEType {
		t.Fatalf("projected resource = %+v", resource)
	}
	for _, want := range []string{"virtual-machine", "proxmox", "findings: 2 total", "pending approvals: 1"} {
		if !strings.Contains(resource.Description, want) {
			t.Fatalf("resource description missing %q: %q", want, resource.Description)
		}
	}

	raw, err := json.Marshal(MCPReadResourceParams{URI: resource.URI})
	if err != nil {
		t.Fatalf("marshal read params: %v", err)
	}
	read, err := server.ResourcesRead(context.Background(), raw)
	if err != nil {
		t.Fatalf("ResourcesRead: %v", err)
	}
	if len(read.Contents) != 1 {
		t.Fatalf("resources/read contents = %+v", read.Contents)
	}
	content := read.Contents[0]
	if content.URI != resource.URI || content.MimeType != MCPResourceContextMIMEType {
		t.Fatalf("resource content = %+v", content)
	}
	if content.Text != `{"canonicalId":"vm:101","resourceName":"Database VM"}` {
		t.Fatalf("resource content text = %q", content.Text)
	}
	if got.token != "test-token" {
		t.Fatalf("upstream token = %q, want test-token", got.token)
	}
	if strings.Join(got.paths, ",") != "/api/agent/fleet-context,/api/agent/resource-context/vm%3A101" {
		t.Fatalf("upstream paths = %+v", got.paths)
	}
}

func TestMCPManifestResourceProjectionUsesSurfaceAffordanceAndContextCapabilities(t *testing.T) {
	capabilities := []Capability{
		{Name: FleetContextCapabilityName, Path: FleetContextCapabilityPath, Method: http.MethodGet},
		{Name: ResourceContextCapabilityName, Path: ResourceContextCapabilityPath, Method: http.MethodGet},
	}
	if !MCPManifestResourceProjectionSupported(Manifest{Capabilities: capabilities}, SurfaceIDPulseMCP) {
		t.Fatal("legacy Pulse MCP manifests with both context capabilities should support resources")
	}
	if got := ManifestSurfaceResourceCapabilities(Manifest{Capabilities: capabilities}, SurfaceIDPulseMCP); len(got) != 2 {
		t.Fatalf("surface resource capabilities = %+v, want fleet and resource context", got)
	}

	surfaceContract := CanonicalManifest().SurfaceContract
	for i := range surfaceContract.OperatorSurfaces {
		if surfaceContract.OperatorSurfaces[i].ID == SurfaceIDPulseMCP {
			surfaceContract.OperatorSurfaces[i].Affordances = SurfaceAffordanceContract{Tools: true}
		}
	}
	disabled := Manifest{SurfaceContract: surfaceContract, Capabilities: capabilities}
	if MCPManifestResourceProjectionSupported(disabled, SurfaceIDPulseMCP) {
		t.Fatal("surface-disabled resource affordance must block MCP resources")
	}
	if got := ManifestSurfaceResourceCapabilities(disabled, SurfaceIDPulseMCP); len(got) != 0 {
		t.Fatalf("surface-disabled resource capabilities = %+v, want none", got)
	}

	missingResourceContext := Manifest{Capabilities: capabilities[:1]}
	if MCPManifestResourceProjectionSupported(missingResourceContext, SurfaceIDPulseMCP) {
		t.Fatal("MCP resources must require both fleet and resource context capabilities")
	}
}

func TestMCPManifestToolServerProjectsWorkflowPromptsFromManifest(t *testing.T) {
	capabilities := []Capability{
		{Name: FleetContextCapabilityName, Path: FleetContextCapabilityPath, Method: http.MethodGet, Description: "triage"},
		{Name: ResourceContextCapabilityName, Path: ResourceContextCapabilityPath, Method: http.MethodGet, Description: "depth"},
		{Name: ListFindingsCapabilityName, Path: "/api/ai/patrol/findings", Method: http.MethodGet, Description: "findings"},
	}
	recordedPrompts := []string{}
	server := MCPManifestToolServer{
		ServerName:    "pulse-mcp",
		ServerVersion: "0.1.0",
		Manifest: Manifest{
			Capabilities: capabilities,
			WorkflowPrompts: []PulseWorkflowPrompt{{
				Name:        PulseWorkflowPromptInvestigateResource,
				Label:       "Inspect this resource",
				Description: "Investigate one resource from manifest metadata.",
				Arguments: []PulseWorkflowPromptArgument{{
					Name:        ResourceIDArgumentName,
					Description: "Canonical resource id.",
					Required:    true,
				}},
			}},
		},
		RecordWorkflowPromptActivity: func(_ context.Context, promptName string) {
			recordedPrompts = append(recordedPrompts, promptName)
		},
	}

	init := server.Initialize()
	if init.Capabilities.Prompts == nil {
		t.Fatalf("initialize result must advertise prompts for prompt-capable manifests: %+v", init.Capabilities)
	}

	list := server.PromptsList()
	if len(list.Prompts) != 1 {
		t.Fatalf("prompts/list = %+v, want one manifest-backed prompt", list.Prompts)
	}
	names := map[string]MCPPrompt{}
	for _, prompt := range list.Prompts {
		names[prompt.Name] = prompt
	}
	if _, ok := names[MCPPromptTriageFleet]; ok {
		t.Fatalf("prompts/list must honor manifest workflowPrompts instead of reprojecting capabilities: %+v", list.Prompts)
	}
	resourcePrompt := names[MCPPromptInvestigateResource]
	if resourcePrompt.Title != "Inspect this resource" {
		t.Fatalf("resource prompt title = %q", resourcePrompt.Title)
	}
	if len(resourcePrompt.Arguments) != 1 || resourcePrompt.Arguments[0].Name != ResourceIDArgumentName || !resourcePrompt.Arguments[0].Required {
		t.Fatalf("resource prompt arguments = %+v", resourcePrompt.Arguments)
	}

	raw, err := json.Marshal(MCPGetPromptParams{
		Name:      MCPPromptInvestigateResource,
		Arguments: map[string]string{ResourceIDArgumentName: "vm:101"},
	})
	if err != nil {
		t.Fatalf("marshal prompt params: %v", err)
	}
	result, err := server.PromptsGet(context.Background(), raw)
	if err != nil {
		t.Fatalf("PromptsGet: %v", err)
	}
	if result.Description != "Pulse resource investigation" || len(result.Messages) != 1 {
		t.Fatalf("prompts/get result = %+v", result)
	}
	if len(recordedPrompts) != 1 || recordedPrompts[0] != MCPPromptInvestigateResource {
		t.Fatalf("recorded prompts = %+v, want [%s]", recordedPrompts, MCPPromptInvestigateResource)
	}
	message := result.Messages[0]
	if message.Role != "user" || message.Content.Type != "text" {
		t.Fatalf("prompt message = %+v", message)
	}
	for _, want := range []string{`"vm:101"`, MCPResourceURI("vm:101"), ResourceContextCapabilityName, ResourceIDArgumentName, "Do not execute write tools"} {
		if !strings.Contains(message.Content.Text, want) {
			t.Fatalf("resource prompt missing %q: %q", want, message.Content.Text)
		}
	}

	raw, err = json.Marshal(MCPGetPromptParams{Name: MCPPromptTriageFleet})
	if err != nil {
		t.Fatalf("marshal prompt params: %v", err)
	}
	if _, err := server.PromptsGet(context.Background(), raw); err == nil || !strings.Contains(err.Error(), "unknown prompt") {
		t.Fatalf("PromptsGet must reject prompts not declared by workflowPrompts; err=%v", err)
	}
	if len(recordedPrompts) != 1 {
		t.Fatalf("failed prompts/get must not record activity, got %+v", recordedPrompts)
	}
}

func TestMCPManifestSurfacePromptProjectionSupportedHonorsSurfaceAffordances(t *testing.T) {
	manifest := Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		Capabilities: []Capability{{
			Name:   FleetContextCapabilityName,
			Path:   FleetContextCapabilityPath,
			Method: http.MethodGet,
		}},
		WorkflowPrompts: []PulseWorkflowPrompt{{
			Name: PulseWorkflowPromptTriageFleet,
		}},
	}
	if !MCPManifestSurfacePromptProjectionSupported(manifest, SurfaceIDPulseMCP) {
		t.Fatal("Pulse MCP surface with prompt affordance and workflow prompts must support prompt projection")
	}

	promptsDisabled := manifest
	for i := range promptsDisabled.SurfaceContract.OperatorSurfaces {
		if promptsDisabled.SurfaceContract.OperatorSurfaces[i].ID == SurfaceIDPulseMCP {
			promptsDisabled.SurfaceContract.OperatorSurfaces[i].Affordances = SurfaceAffordanceContract{Tools: true}
		}
	}
	if MCPManifestSurfacePromptProjectionSupported(promptsDisabled, SurfaceIDPulseMCP) {
		t.Fatal("surface without prompt affordance must not support prompt projection")
	}

	withoutPrompts := manifest
	withoutPrompts.WorkflowPrompts = []PulseWorkflowPrompt{}
	if MCPManifestSurfacePromptProjectionSupported(withoutPrompts, SurfaceIDPulseMCP) {
		t.Fatal("surface with prompt affordance but no workflow prompts must not support prompt projection")
	}
}

func TestMCPManifestToolServerPromptsGetUsesSurfacePromptProjectionGate(t *testing.T) {
	rawPrompt, err := json.Marshal(MCPGetPromptParams{Name: MCPPromptTriageFleet})
	if err != nil {
		t.Fatalf("marshal prompt params: %v", err)
	}
	server := MCPManifestToolServer{
		ServerName:    "pulse-mcp",
		ServerVersion: "0.1.0",
		Manifest: Manifest{
			SurfaceContract: CanonicalManifest().SurfaceContract,
			Capabilities: []Capability{{
				Name:   FleetContextCapabilityName,
				Path:   FleetContextCapabilityPath,
				Method: http.MethodGet,
			}},
			WorkflowPrompts: []PulseWorkflowPrompt{},
		},
	}

	if init := server.Initialize(); init.Capabilities.Prompts != nil {
		t.Fatalf("initialize must not advertise prompts for an explicitly empty workflow prompt catalogue: %+v", init.Capabilities.Prompts)
	}
	if prompts := server.PromptsList(); len(prompts.Prompts) != 0 {
		t.Fatalf("PromptsList = %+v, want empty prompts", prompts)
	}
	if _, err := server.PromptsGet(context.Background(), rawPrompt); err == nil || !strings.Contains(err.Error(), "prompts are not enabled") {
		t.Fatalf("PromptsGet must use the same surface prompt projection gate as initialize/list; err=%v", err)
	}
}

func TestMCPPromptParamsNormalizeAndRejectInvalidShape(t *testing.T) {
	source := map[string]string{ResourceIDArgumentName: "vm:101"}
	params := MCPGetPromptParams{
		Name:      " " + MCPPromptInvestigateResource + " ",
		Arguments: source,
	}.NormalizeCollections()
	if params.Name != MCPPromptInvestigateResource || params.Arguments[ResourceIDArgumentName] != "vm:101" {
		t.Fatalf("normalized prompt params = %+v", params)
	}
	params.Arguments[ResourceIDArgumentName] = "vm:102"
	if source[ResourceIDArgumentName] != "vm:101" {
		t.Fatalf("normalized prompt params aliased source: source=%+v params=%+v", source, params.Arguments)
	}

	decoded, err := DecodeMCPGetPromptParams(json.RawMessage(`{"name":" ` + MCPPromptTriageFleet + ` "}`))
	if err != nil {
		t.Fatalf("DecodeMCPGetPromptParams: %v", err)
	}
	if decoded.Name != MCPPromptTriageFleet || decoded.Arguments == nil || len(decoded.Arguments) != 0 {
		t.Fatalf("decoded prompt params = %+v", decoded)
	}

	if _, err := DecodeMCPGetPromptParams(nil); err == nil || !strings.Contains(err.Error(), "prompt name is required") {
		t.Fatalf("missing prompt name error = %v", err)
	}

	declared, err := BuildMCPPromptFromManifest(Manifest{
		WorkflowPrompts: []PulseWorkflowPrompt{{
			Name: MCPPromptTriageFleet,
		}},
	}, MCPGetPromptParams{Name: MCPPromptTriageFleet})
	if err != nil {
		t.Fatalf("manifest-declared MCP prompt: %v", err)
	}
	if len(declared.Messages) != 1 || !strings.Contains(declared.Messages[0].Content.Text, FleetContextCapabilityName) {
		t.Fatalf("manifest-declared MCP prompt result = %+v", declared)
	}

	_, err = BuildMCPPromptFromManifest(Manifest{
		Capabilities: []Capability{{Name: ResourceContextCapabilityName}},
		WorkflowPrompts: []PulseWorkflowPrompt{{
			Name: MCPPromptInvestigateResource,
		}},
	}, MCPGetPromptParams{Name: MCPPromptInvestigateResource})
	if err == nil || err.Error() != "prompt argument "+ResourceIDArgumentName+" is required" {
		t.Fatalf("missing resourceId error = %v", err)
	}

	_, err = BuildMCPPromptFromManifest(Manifest{
		Capabilities: []Capability{{Name: FleetContextCapabilityName}},
		WorkflowPrompts: []PulseWorkflowPrompt{
			{Name: MCPPromptInvestigateResource},
		},
	}, MCPGetPromptParams{Name: MCPPromptTriageFleet})
	if err == nil || err.Error() != "unknown prompt: pulse_triage_fleet" {
		t.Fatalf("unknown prompt error = %v", err)
	}
}

func TestMCPMethodPayloadsUseSharedContracts(t *testing.T) {
	init := NewMCPToolServerInitializeResult("pulse-mcp", "0.1.0", true)
	body, err := json.Marshal(init)
	if err != nil {
		t.Fatalf("marshal initialize result: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		`"protocolVersion":"2025-06-18"`,
		`"tools":{}`,
		`"pulseNotifications":{"kinds":`,
		`"finding.created"`,
		`"serverInfo":{"name":"pulse-mcp","version":"0.1.0"}`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("initialize result missing %s: %s", want, text)
		}
	}

	call := MCPCallToolParams{
		Name:      "get_resource_context",
		Arguments: map[string]any{"resourceId": "vm:101"},
	}
	body, err = json.Marshal(call)
	if err != nil {
		t.Fatalf("marshal call params: %v", err)
	}
	var decoded MCPCallToolParams
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal call params: %v", err)
	}
	if decoded.Name != call.Name || decoded.Arguments["resourceId"] != "vm:101" {
		t.Fatalf("call params did not round-trip through shared shape: %+v", decoded)
	}
}

func TestToolCallParamsNormalizeAndValidateSharedContract(t *testing.T) {
	source := map[string]any{
		"resourceId": "vm:101",
		"body": map[string]any{
			"note": "maintenance",
			"tags": []any{"planned"},
		},
	}
	normalized := NormalizeToolCallParams(ToolCallParams{
		Name:      " get_resource_context ",
		Arguments: source,
	})
	if normalized.Name != "get_resource_context" {
		t.Fatalf("normalized name = %q, want get_resource_context", normalized.Name)
	}
	if normalized.Arguments["resourceId"] != "vm:101" {
		t.Fatalf("normalized arguments = %#v", normalized.Arguments)
	}
	normalized.Arguments["resourceId"] = "vm:102"
	if source["resourceId"] != "vm:101" {
		t.Fatalf("normalized arguments aliased source: source=%#v normalized=%#v", source, normalized.Arguments)
	}
	normalizedBody := normalized.Arguments["body"].(map[string]any)
	normalizedBody["note"] = "changed"
	normalizedBody["tags"].([]any)[0] = "changed"
	sourceBody := source["body"].(map[string]any)
	if sourceBody["note"] != "maintenance" || sourceBody["tags"].([]any)[0] != "planned" {
		t.Fatalf("normalized nested arguments aliased source: source=%#v normalized=%#v", source, normalized.Arguments)
	}

	emptyArgs := NormalizeToolCallParams(ToolCallParams{Name: "ping"})
	if emptyArgs.Arguments == nil || len(emptyArgs.Arguments) != 0 {
		t.Fatalf("missing arguments must normalize to empty object: %#v", emptyArgs.Arguments)
	}

	if err := ValidateToolCallParams(ToolCallParams{Name: "  "}); err == nil || err.Error() != "tool name is required" {
		t.Fatalf("blank name validation error = %v", err)
	}
}

func TestPrepareToolRegistryExecutionUsesSharedFailureResults(t *testing.T) {
	source := map[string]any{
		"resourceId": "vm:101",
		"body":       map[string]any{"note": "maintenance"},
	}
	params, invalid, ok := PrepareToolRegistryExecution(" read_resource ", source)
	if !ok {
		t.Fatalf("valid registry execution returned invalid result: %+v", invalid)
	}
	if params.Name != "read_resource" || params.Arguments["resourceId"] != "vm:101" {
		t.Fatalf("params = %+v", params)
	}
	params.Arguments["resourceId"] = "vm:102"
	params.Arguments["body"].(map[string]any)["note"] = "changed"
	if source["resourceId"] != "vm:101" || source["body"].(map[string]any)["note"] != "maintenance" {
		t.Fatalf("registry execution params aliased source: source=%#v params=%#v", source, params)
	}

	_, invalid, ok = PrepareToolRegistryExecution(" ", nil)
	if ok {
		t.Fatalf("blank tool name unexpectedly prepared")
	}
	interpreted := InterpretToolResult(invalid)
	if !interpreted.IsError || interpreted.Text != "invalid tools/call params: tool name is required" {
		t.Fatalf("invalid result = %+v interpreted=%+v", invalid, interpreted)
	}

	unknown := InterpretToolResult(NewUnknownToolResult(" missing "))
	if !unknown.IsError || unknown.Text != "unknown tool: missing" {
		t.Fatalf("unknown result = %+v", unknown)
	}

	disabled := InterpretToolResult(NewControlToolsDisabledToolResult())
	if disabled.IsError || disabled.Text != ControlToolsDisabledMessage {
		t.Fatalf("control-disabled result = %+v", disabled)
	}
}

func TestDecodeMCPCallToolParamsNormalizesAndRejectsInvalidShape(t *testing.T) {
	decoded, err := DecodeMCPCallToolParams(json.RawMessage(`{"name":" get_fleet_context "}`))
	if err != nil {
		t.Fatalf("DecodeMCPCallToolParams: %v", err)
	}
	if decoded.Name != "get_fleet_context" {
		t.Fatalf("decoded name = %q, want get_fleet_context", decoded.Name)
	}
	if decoded.Arguments == nil || len(decoded.Arguments) != 0 {
		t.Fatalf("decoded arguments = %#v, want empty object", decoded.Arguments)
	}

	for _, raw := range []json.RawMessage{
		json.RawMessage(`{}`),
		json.RawMessage(`{"name":"   ","arguments":{}}`),
	} {
		if _, err := DecodeMCPCallToolParams(raw); err == nil || !strings.Contains(err.Error(), "decode tools/call params: tool name is required") {
			t.Fatalf("invalid params %s error = %v", raw, err)
		}
	}
}

func TestExecuteCapabilityToolHTTPProjectsManifestCallAndWrapsSharedResult(t *testing.T) {
	var got struct {
		Method      string
		Path        string
		Token       string
		ContentType string
		Body        string
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Method = r.Method
		got.Path = r.URL.EscapedPath()
		got.Token = r.Header.Get(AgentAPITokenHeader)
		got.ContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		got.Body = string(body)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":true}`))
	}))
	defer server.Close()

	result, err := ExecuteCapabilityToolHTTP(context.Background(), server.Client(), server.URL, "test-token", []Capability{{
		Name:   SetOperatorStateCapabilityName,
		Method: http.MethodPut,
		Path:   OperatorStateCapabilityPath,
	}}, ToolCallParams{
		Name: SetOperatorStateCapabilityName,
		Arguments: map[string]any{
			ResourceIDArgumentName: "vm:101/console",
			"intentionallyOffline": true,
			"neverAutoRemediate":   false,
			"maintenanceReference": "ticket-1",
		},
	})
	if err != nil {
		t.Fatalf("ExecuteCapabilityToolHTTP: %v", err)
	}

	if got.Method != http.MethodPut {
		t.Fatalf("method = %s, want PUT", got.Method)
	}
	if got.Path != "/api/resources/vm%3A101%2Fconsole/operator-state" {
		t.Fatalf("path = %s", got.Path)
	}
	if got.Token != "test-token" {
		t.Fatalf("%s = %q", AgentAPITokenHeader, got.Token)
	}
	if got.ContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got.ContentType)
	}
	if strings.Contains(got.Body, ResourceIDArgumentName) {
		t.Fatalf("body must not duplicate path arg: %s", got.Body)
	}
	for _, want := range []string{`"intentionallyOffline":true`, `"neverAutoRemediate":false`, `"maintenanceReference":"ticket-1"`} {
		if !strings.Contains(got.Body, want) {
			t.Fatalf("body missing %s: %s", want, got.Body)
		}
	}
	if result.IsError {
		t.Fatalf("2xx upstream response must become shared tool success: %+v", result)
	}
	if len(result.Content) != 1 || result.Content[0].Text != `{"accepted":true}` {
		t.Fatalf("tool result = %+v", result)
	}
}

func TestExecuteCapabilityToolHTTPPreservesNon2xxAsSharedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"scope_required","message":"monitoring:write is required"}`))
	}))
	defer server.Close()

	result, err := ExecuteCapabilityToolHTTP(context.Background(), server.Client(), server.URL, "test-token", []Capability{{
		Name:   SetOperatorStateCapabilityName,
		Method: http.MethodPut,
		Path:   OperatorStateCapabilityPath,
	}}, ToolCallParams{
		Name:      SetOperatorStateCapabilityName,
		Arguments: map[string]any{ResourceIDArgumentName: "vm:101", "body": map[string]any{"offline": true}},
	})
	if err != nil {
		t.Fatalf("ExecuteCapabilityToolHTTP: %v", err)
	}
	if !result.IsError {
		t.Fatalf("non-2xx upstream response must become shared tool error: %+v", result)
	}
	if len(result.Content) != 1 || result.Content[0].Text != `{"error":"scope_required","message":"monitoring:write is required"}` {
		t.Fatalf("tool result = %+v", result)
	}
}

func TestExecuteCapabilityToolHTTPRejectsStreamingCapabilities(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		t.Fatalf("streaming capability must not be invoked through request/response tool execution")
	}))
	defer server.Close()

	_, err := ExecuteCapabilityToolHTTP(context.Background(), server.Client(), server.URL, "test-token", []Capability{{
		Name:   EventSubscriptionCapabilityName,
		Method: http.MethodGet,
		Path:   AgentEventsPath,
	}}, ToolCallParams{Name: "subscribe_events", Arguments: map[string]any{}})
	if err == nil {
		t.Fatal("expected streaming capability to be rejected")
	}
	if err.Error() != "unknown tool: subscribe_events" {
		t.Fatalf("error = %q, want unknown tool", err.Error())
	}
	if called {
		t.Fatal("streaming capability reached upstream HTTP server")
	}
}

func TestExecuteCapabilityToolHTTPReturnsStableValidationAndLookupErrors(t *testing.T) {
	_, err := ExecuteCapabilityToolHTTP(context.Background(), nil, "http://pulse.local", "token", nil, ToolCallParams{Name: "  ", Arguments: map[string]any{}})
	if err == nil || err.Error() != "tool name is required" {
		t.Fatalf("invalid params error = %v", err)
	}

	_, err = ExecuteCapabilityToolHTTP(context.Background(), nil, "http://pulse.local", "token", []Capability{{
		Name:   "get_fleet_context",
		Method: http.MethodGet,
		Path:   "/api/agent/fleet-context",
	}}, ToolCallParams{Name: "missing_capability", Arguments: map[string]any{}})
	if err == nil || err.Error() != "unknown tool: missing_capability" {
		t.Fatalf("unknown tool error = %v", err)
	}
}

func TestExecuteMCPManifestSurfaceToolHTTPReturnsStableDecodeErrorsAndRequiresSurfaceContract(t *testing.T) {
	_, err := ExecuteMCPManifestSurfaceToolHTTP(context.Background(), nil, "http://pulse.local", "token", Manifest{}, SurfaceIDPulseMCP, json.RawMessage(`{"name":`))
	if err == nil || !strings.Contains(err.Error(), "decode tools/call params") {
		t.Fatalf("decode error = %v", err)
	}

	_, err = ExecuteMCPManifestSurfaceToolHTTP(context.Background(), nil, "http://pulse.local", "token", Manifest{}, SurfaceIDPulseMCP, json.RawMessage(`{"name":"  ","arguments":{}}`))
	if err == nil || !strings.Contains(err.Error(), "decode tools/call params: tool name is required") {
		t.Fatalf("invalid params error = %v", err)
	}

	manifest := Manifest{
		SurfaceContract: CanonicalManifest().SurfaceContract,
		Capabilities: []Capability{{
			Name:   ResourceContextCapabilityName,
			Method: http.MethodGet,
			Path:   ResourceContextCapabilityPath,
		}},
	}
	_, err = ExecuteMCPManifestSurfaceToolHTTP(context.Background(), nil, "http://pulse.local", "token", manifest, SurfaceIDPulseMCP, json.RawMessage(`{"name":"get_resource_context","arguments":{"resourceId":"vm:101"}}`))
	if err == nil || err.Error() != "unknown tool: get_resource_context" {
		t.Fatalf("missing surface contract error = %v, want unknown tool", err)
	}

	manifest.SurfaceToolContracts = []SurfaceToolContract{{
		SurfaceID:   SurfaceIDPulseMCP,
		ToolSource:  SurfaceToolSourceCapabilityManifest,
		ToolNames:   []string{FleetContextCapabilityName},
		Affordances: DefaultSurfaceAffordancesForID(SurfaceIDPulseMCP),
	}}
	manifest.Capabilities = []Capability{{
		Name:   FleetContextCapabilityName,
		Method: http.MethodGet,
		Path:   "/api/agent/fleet-context",
	}}
	_, err = ExecuteMCPManifestSurfaceToolHTTP(context.Background(), nil, "http://pulse.local", "token", manifest, SurfaceIDPulseMCP, json.RawMessage(`{"name":"missing_capability","arguments":{}}`))
	if err == nil || err.Error() != "unknown tool: missing_capability" {
		t.Fatalf("unknown tool error = %v", err)
	}
}
