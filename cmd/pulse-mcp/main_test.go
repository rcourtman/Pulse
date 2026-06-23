package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

type httpDoerFunc func(*http.Request) (*http.Response, error)

func (f httpDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAgentCapabilityUsesSharedWireContract(t *testing.T) {
	rawSchema := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	rawOutputSchema := json.RawMessage(`{"type":"object","properties":{"resources":{"type":"array"}}}`)
	capability := agentCapability{
		Name:         "get_fleet_context",
		Method:       http.MethodGet,
		Path:         "/api/agent/fleet-context",
		ActionMode:   agentcapabilities.ActionModeRead,
		InputSchema:  rawSchema,
		OutputSchema: rawOutputSchema,
	}
	manifest := agentCapabilitiesManifest{
		Version:      "v1",
		Capabilities: []agentcapabilities.Capability{capability},
	}
	var shared agentCapability = manifest.Capabilities[0]

	if shared.Name != capability.Name {
		t.Fatalf("shared manifest capability name = %q, want %q", shared.Name, capability.Name)
	}
	if shared.ActionMode != agentcapabilities.ActionModeRead {
		t.Fatalf("shared manifest action mode = %q, want %q", shared.ActionMode, agentcapabilities.ActionModeRead)
	}
	if string(shared.InputSchema) != string(rawSchema) {
		t.Fatalf("shared manifest input schema = %s, want %s", shared.InputSchema, rawSchema)
	}
	if string(shared.OutputSchema) != string(rawOutputSchema) {
		t.Fatalf("shared manifest output schema = %s, want %s", shared.OutputSchema, rawOutputSchema)
	}
}

func TestMissingAPITokenMessageUsesManifestScopeSummary(t *testing.T) {
	msg := missingAPITokenMessage("PULSE_API_TOKEN", &agentCapabilitiesManifest{
		RequiredScopes: []string{
			"settings:write",
			"monitoring:read",
			"ai:execute",
			"monitoring:read",
		},
		Capabilities: []agentCapability{
			{Name: "legacy_extra", Scope: "monitoring:write"},
		},
	})

	for _, want := range []string{
		"env var PULSE_API_TOKEN is empty",
		"current manifest scopes: monitoring:read, settings:write, ai:execute",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing token message missing %q in %q", want, msg)
		}
	}
	if strings.Count(msg, "monitoring:read") != 1 {
		t.Fatalf("missing token message must deduplicate scopes; got %q", msg)
	}
}

func TestMissingAPITokenMessageFallsBackForLegacyManifest(t *testing.T) {
	msg := missingAPITokenMessage("PULSE_API_TOKEN", &agentCapabilitiesManifest{
		Capabilities: []agentCapability{
			{Name: "write_settings", Scope: "settings:write"},
			{Name: "read_monitoring", Scope: "monitoring:read"},
		},
	})

	if !strings.Contains(msg, "current manifest scopes: monitoring:read, settings:write") {
		t.Fatalf("missing token message should fall back to capability scopes for legacy manifests; got %q", msg)
	}
}

func TestMainTokenGuidanceUsesSharedManifestScopeSummary(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "agentcapabilities.ManifestRequiredScopeList(manifest)") {
		t.Fatal("pulse-mcp token guidance must consume the manifest-owned requiredScopes summary")
	}
	if strings.Contains(text, "RequiredCapabilityScopeList(capabilities)") {
		t.Fatal("pulse-mcp token guidance must not recompute current manifest scopes from capability rows")
	}
	if strings.Contains(text, "monitoring:read scope (and monitoring:write") {
		t.Fatal("pulse-mcp must not hardcode partial token-scope guidance in startup errors")
	}
}

func TestMainUsesSharedMCPAdapterRuntimeDefaults(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		`flag.String("base-url", agentcapabilities.DefaultMCPAdapterDefaultBaseURL`,
		`flag.String("token-env", agentcapabilities.DefaultMCPAdapterTokenEnv`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("pulse-mcp runtime defaults must come from the shared MCP adapter contract; missing %s", want)
		}
	}
}

func TestReadmePresentsSharedMCPClientConfig(t *testing.T) {
	source, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		agentcapabilities.MCPReadmeClientConfigStartMarker,
		agentcapabilities.MCPReadmeClientConfigEndMarker,
		"Most MCP clients need the same manifest-owned runtime facts",
		"server name `pulse`, command `pulse-mcp`, base URL flag `--base-url`",
		"currently declared config families: `OpenCode`, `Claude-style clients`, and `custom MCP clients`",
		"Uses OpenCode's top-level mcp object.",
		`"type": "local"`,
		`"command": ["pulse-mcp", "--base-url", "http://localhost:7655"]`,
		`"environment": {`,
		"Uses the common mcpServers object supported by Claude Desktop and Claude Code.",
		"Use command `pulse-mcp`, pass `--base-url http://localhost:7655`",
		"Restart your client after saving the config.",
		"Claude Desktop",
		"Claude Code",
		"OpenCode",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("README.md must include shared MCP client setup copy %q", want)
		}
	}
	if strings.Contains(text, "Restart Claude Desktop. The Pulse tools appear") {
		t.Fatal("README.md must not present Pulse MCP setup as Claude Desktop-only")
	}
	if strings.Contains(text, "The examples below use the common\n`mcpServers` shape") {
		t.Fatal("README.md must not present OpenCode as a generic mcpServers wrapper")
	}
}

func testPulseMCPSurfaceToolContracts(toolNames ...string) []agentcapabilities.SurfaceToolContract {
	return []agentcapabilities.SurfaceToolContract{
		{
			SurfaceID:   agentcapabilities.SurfaceIDPulseMCP,
			ToolSource:  agentcapabilities.SurfaceToolSourceCapabilityManifest,
			ToolNames:   append([]string(nil), toolNames...),
			Affordances: agentcapabilities.DefaultSurfaceAffordancesForID(agentcapabilities.SurfaceIDPulseMCP),
		},
	}
}

// TestToolInputSchema_PathPlaceholdersBecomeRequiredStringProps
// pins the rule that turns capability paths into MCP tool input
// schemas: every {name} segment in the path becomes a required
// string property the agent must supply, with a description that
// hints at the canonical shape ("vm:101", "container:web-1") so
// the LLM forms the right id without back-and-forth.
func TestToolInputSchema_PathPlaceholdersBecomeRequiredStringProps(t *testing.T) {
	cap := agentCapability{
		Name:   "get_resource_context",
		Path:   "/api/agent/resource-context/{resourceId}",
		Method: http.MethodGet,
	}
	raw := agentcapabilities.ToolInputSchema(cap)
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties missing or wrong type: %v", schema["properties"])
	}
	if _, ok := props["resourceId"]; !ok {
		t.Fatalf("schema must declare resourceId property; got %v", props)
	}
	required, _ := schema["required"].([]any)
	found := false
	for _, r := range required {
		if r == "resourceId" {
			found = true
		}
	}
	if !found {
		t.Errorf("resourceId must be required so the agent can't omit it; got required=%v", required)
	}
}

// TestToolInputSchema_NonGetCapabilitiesAcceptBody pins that
// non-GET/DELETE tools expose a `body` property the agent fills
// with the request payload. GET tools must NOT advertise a body
// property so the agent doesn't try to send one (which would be
// dropped by net/http anyway, but advertising it would be
// misleading).
func TestToolInputSchema_NonGetCapabilitiesAcceptBody(t *testing.T) {
	get := agentCapability{Path: "/api/foo", Method: http.MethodGet}
	post := agentCapability{
		Path:             "/api/foo",
		Method:           http.MethodPost,
		RequestBodyShape: "{ id: string }",
	}
	put := agentCapability{Path: "/api/resources/{id}/operator-state", Method: http.MethodPut}
	del := agentCapability{Path: "/api/resources/{id}/operator-state", Method: http.MethodDelete}

	for name, tc := range map[string]struct {
		cap     agentCapability
		hasBody bool
	}{
		"GET":    {get, false},
		"POST":   {post, true},
		"PUT":    {put, true},
		"DELETE": {del, false},
	} {
		t.Run(name, func(t *testing.T) {
			raw := agentcapabilities.ToolInputSchema(tc.cap)
			var schema map[string]any
			if err := json.Unmarshal(raw, &schema); err != nil {
				t.Fatalf("unmarshal schema: %v", err)
			}
			props, _ := schema["properties"].(map[string]any)
			_, has := props["body"]
			if has != tc.hasBody {
				t.Errorf("%s: body property presence = %v, want %v", name, has, tc.hasBody)
			}
		})
	}
}

func TestToolInputSchema_FallbackUsesSharedPermissiveEnvelope(t *testing.T) {
	raw := agentcapabilities.ToolInputSchema(agentCapability{
		Path:   "/api/resources/{id}/operator-state",
		Method: http.MethodPut,
	})
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schema["additionalProperties"] != true {
		t.Fatalf("fallback schema additionalProperties = %v, want true", schema["additionalProperties"])
	}
	props, _ := schema["properties"].(map[string]any)
	if _, hasID := props["id"]; !hasID {
		t.Fatalf("fallback schema must include path argument id: %v", props)
	}
	if _, hasBody := props["body"]; !hasBody {
		t.Fatalf("fallback schema must include body property for non-GET methods: %v", props)
	}
}

func TestToolInputSchema_UsesManifestProvidedSchema(t *testing.T) {
	rawSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"type": { "enum": ["pve", "pbs", "pmg"] },
			"host": { "type": "string" }
		},
		"required": ["type", "host"],
		"additionalProperties": false
	}`)
	cap := agentCapability{
		Name:        "add_node",
		Path:        "/api/config/nodes",
		Method:      http.MethodPost,
		InputSchema: rawSchema,
	}

	got := agentcapabilities.ToolInputSchema(cap)
	var schema map[string]any
	if err := json.Unmarshal(got, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schema["additionalProperties"] != false {
		t.Errorf("manifest-provided schema must be preserved; got %v", schema)
	}
	if _, hasBody := schema["properties"].(map[string]any)["body"]; hasBody {
		t.Errorf("manifest-provided schema should not be wrapped in body: %s", string(got))
	}
}

func TestToolInputSchema_PreservesManifestActionArgumentSchema(t *testing.T) {
	rawSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"actionId": { "type": "string", "pattern": "^[a-zA-Z0-9_-]+$" },
			"outcome": { "type": "string", "enum": ["approved", "rejected"] },
			"reason": { "type": "string" }
		},
		"required": ["actionId", "outcome"],
		"additionalProperties": false
	}`)
	cap := agentCapability{
		Name:        "decide_action",
		Path:        "/api/actions/{actionId}/decision",
		Method:      http.MethodPost,
		InputSchema: rawSchema,
	}

	got := agentcapabilities.ToolInputSchema(cap)
	var schema map[string]any
	if err := json.Unmarshal(got, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	props, _ := schema["properties"].(map[string]any)
	if _, hasActionID := props["actionId"]; !hasActionID {
		t.Fatalf("manifest-provided action schema must preserve path argument actionId: %s", string(got))
	}
	if _, hasBody := props["body"]; hasBody {
		t.Fatalf("manifest-provided action schema must not be replaced by generic body wrapper: %s", string(got))
	}

	operatorStateSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"resourceId": { "type": "string" },
			"intentionallyOffline": { "type": "boolean" },
			"neverAutoRemediate": { "type": "boolean" }
		},
		"required": ["resourceId", "intentionallyOffline", "neverAutoRemediate"],
		"additionalProperties": false
	}`)
	operatorCap := agentCapability{
		Name:        agentcapabilities.SetOperatorStateCapabilityName,
		Path:        agentcapabilities.OperatorStateCapabilityPath,
		Method:      http.MethodPut,
		InputSchema: operatorStateSchema,
	}

	got = agentcapabilities.ToolInputSchema(operatorCap)
	if err := json.Unmarshal(got, &schema); err != nil {
		t.Fatalf("unmarshal operator-state schema: %v", err)
	}
	props, _ = schema["properties"].(map[string]any)
	if _, hasResourceID := props["resourceId"]; !hasResourceID {
		t.Fatalf("manifest-provided operator-state schema must preserve resourceId path argument: %s", string(got))
	}
	if _, hasBody := props["body"]; hasBody {
		t.Fatalf("manifest-provided operator-state schema must not be replaced by generic body wrapper: %s", string(got))
	}
}

func TestSubstitutePathParameters_FillsAllPlaceholders(t *testing.T) {
	got, err := agentcapabilities.SubstitutePathParameters(
		"/api/agent/resource-context/{resourceId}",
		map[string]any{"resourceId": "vm:101"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/api/agent/resource-context/vm%3A101" {
		t.Errorf("got %q, want /api/agent/resource-context/vm%%3A101", got)
	}
}

func TestSubstitutePathParameters_EscapesReservedCharacters(t *testing.T) {
	got, err := agentcapabilities.SubstitutePathParameters(
		"/api/config/nodes/{nodeId}/test",
		map[string]any{"nodeId": "pve/lab node"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/api/config/nodes/pve%2Flab%20node/test" {
		t.Errorf("got %q, want escaped node id in a single path segment", got)
	}
}

func TestSubstitutePathParameters_MissingPlaceholderIsAStableError(t *testing.T) {
	// The agent must get a clear error when it forgets a path
	// argument — better to fail with "missing path argument
	// resourceId" than to send a literal `{resourceId}` URL to
	// Pulse and get a confusing 404.
	_, err := agentcapabilities.SubstitutePathParameters(
		"/api/agent/resource-context/{resourceId}",
		map[string]any{},
	)
	if err == nil {
		t.Fatal("expected error for missing path arg; got nil")
	}
	if !strings.Contains(err.Error(), "resourceId") {
		t.Errorf("error must name the missing param; got %v", err)
	}
}

func TestSubstitutePathParameters_NonStringIsAnError(t *testing.T) {
	_, err := agentcapabilities.SubstitutePathParameters(
		"/api/resources/{id}/operator-state",
		map[string]any{"id": 12345},
	)
	if err == nil {
		t.Fatal("expected error for non-string path arg; got nil")
	}
}

// TestServer_DispatchInitializeReturnsToolsCapability is the
// MCP-handshake contract: clients call `initialize` first, branch
// on the advertised capabilities, and only call `tools/list` if
// `tools` is present. The server must advertise tools so Claude
// (Desktop / Code) bothers to enumerate the surface.
func TestServer_DispatchInitializeReturnsToolsCapability(t *testing.T) {
	manifest := agentcapabilities.CanonicalManifest()
	s := &mcpServer{manifest: &manifest}
	resp := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
	})
	if resp.Error != nil {
		t.Fatalf("initialize: error = %+v", resp.Error)
	}
	result, _ := resp.Result.(agentcapabilities.MCPInitializeResult)
	if result.Capabilities.Tools == nil {
		t.Fatal("initialize must advertise tools capability so MCP clients enumerate the tool surface")
	}
	if !strings.Contains(result.Instructions, "governed infrastructure-operations surface") {
		t.Fatalf("initialize must include shared Pulse Intelligence operating instructions, got %q", result.Instructions)
	}
	if result.ServerInfo.Name != "pulse-mcp" {
		t.Errorf("serverInfo.name = %v, want pulse-mcp", result.ServerInfo.Name)
	}
	expected := agentcapabilities.NewMCPToolServerInitializeResult(pulseMCPServerName, pulseMCPServerVersion, false)
	if result.ProtocolVersion != expected.ProtocolVersion || result.ServerInfo != expected.ServerInfo {
		t.Fatalf("initialize result must use shared MCP constructor; got %+v want %+v", result, expected)
	}
}

func TestServer_ResourcesListAndReadProjectContextCapabilities(t *testing.T) {
	type captured struct {
		paths []string
		token string
	}
	var got captured
	mu := sync.Mutex{}
	pulse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		got.paths = append(got.paths, r.URL.EscapedPath())
		got.token = r.Header.Get(agentcapabilities.AgentAPITokenHeader)

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
					}
				]
			}`))
		case "/api/agent/resource-context/vm%3A101":
			_, _ = w.Write([]byte(`{"canonicalId":"vm:101","resourceName":"Database VM"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer pulse.Close()

	s := &mcpServer{
		baseURL: pulse.URL,
		token:   "resource-test-token",
		manifest: &agentCapabilitiesManifest{
			Version: "v1",
			Capabilities: []agentCapability{
				{Name: agentcapabilities.FleetContextCapabilityName, Path: "/api/agent/fleet-context", Method: http.MethodGet, Description: "triage"},
				{Name: agentcapabilities.ResourceContextCapabilityName, Path: "/api/agent/resource-context/{resourceId}", Method: http.MethodGet, Description: "depth"},
			},
		},
		http: pulse.Client(),
	}

	init := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  agentcapabilities.MCPMethodInitialize,
	})
	if init.Error != nil {
		t.Fatalf("initialize: error = %+v", init.Error)
	}
	initResult, ok := init.Result.(agentcapabilities.MCPInitializeResult)
	if !ok {
		t.Fatalf("initialize result type = %T, want shared MCPInitializeResult", init.Result)
	}
	if initResult.Capabilities.Resources == nil {
		t.Fatalf("initialize must advertise resources when both context capabilities exist: %+v", initResult.Capabilities)
	}

	list := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  agentcapabilities.MCPMethodResourcesList,
	})
	if list.Error != nil {
		t.Fatalf("resources/list: error = %+v", list.Error)
	}
	listResult, ok := list.Result.(agentcapabilities.MCPListResourcesResult)
	if !ok {
		t.Fatalf("resources/list result type = %T, want shared MCPListResourcesResult", list.Result)
	}
	if len(listResult.Resources) != 1 {
		t.Fatalf("resources/list returned %+v, want one resource", listResult.Resources)
	}
	resource := listResult.Resources[0]
	if resource.URI != agentcapabilities.MCPResourceURI("vm:101") || resource.Name != "Database VM" || resource.MimeType != agentcapabilities.MCPResourceContextMIMEType {
		t.Fatalf("projected MCP resource = %+v", resource)
	}
	if !strings.Contains(resource.Description, "virtual-machine") || !strings.Contains(resource.Description, "pending approvals: 1") {
		t.Fatalf("projected resource description must carry fleet context summary; got %q", resource.Description)
	}

	params, _ := json.Marshal(agentcapabilities.MCPReadResourceParams{URI: resource.URI})
	read := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  agentcapabilities.MCPMethodResourcesRead,
		Params:  params,
	})
	if read.Error != nil {
		t.Fatalf("resources/read: error = %+v", read.Error)
	}
	readResult, ok := read.Result.(agentcapabilities.MCPReadResourceResult)
	if !ok {
		t.Fatalf("resources/read result type = %T, want shared MCPReadResourceResult", read.Result)
	}
	if len(readResult.Contents) != 1 {
		t.Fatalf("resources/read contents = %+v, want one JSON content block", readResult.Contents)
	}
	content := readResult.Contents[0]
	if content.URI != resource.URI || content.MimeType != agentcapabilities.MCPResourceContextMIMEType {
		t.Fatalf("resources/read content = %+v", content)
	}
	if content.Text != `{"canonicalId":"vm:101","resourceName":"Database VM"}` {
		t.Fatalf("resources/read content text = %q", content.Text)
	}

	mu.Lock()
	defer mu.Unlock()
	if got.token != "resource-test-token" {
		t.Fatalf("upstream token = %q, want resource-test-token", got.token)
	}
	if strings.Join(got.paths, ",") != "/api/agent/fleet-context,/api/agent/resource-context/vm%3A101" {
		t.Fatalf("upstream paths = %+v", got.paths)
	}
}

func TestServer_PromptsListAndGetProjectManifestWorkflowPrompts(t *testing.T) {
	s := &mcpServer{
		manifest: &agentCapabilitiesManifest{
			Version: "v1",
			Capabilities: []agentCapability{
				{Name: agentcapabilities.FleetContextCapabilityName, Path: "/api/agent/fleet-context", Method: http.MethodGet, Description: "triage"},
				{Name: agentcapabilities.ResourceContextCapabilityName, Path: "/api/agent/resource-context/{resourceId}", Method: http.MethodGet, Description: "depth"},
				{Name: "list_findings", Path: "/api/ai/patrol/findings", Method: http.MethodGet, Description: "findings"},
			},
		},
	}

	init := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  agentcapabilities.MCPMethodInitialize,
	})
	if init.Error != nil {
		t.Fatalf("initialize: error = %+v", init.Error)
	}
	initResult, ok := init.Result.(agentcapabilities.MCPInitializeResult)
	if !ok {
		t.Fatalf("initialize result type = %T, want shared MCPInitializeResult", init.Result)
	}
	if initResult.Capabilities.Prompts == nil {
		t.Fatalf("initialize must advertise prompts when workflow capabilities exist: %+v", initResult.Capabilities)
	}

	list := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  agentcapabilities.MCPMethodPromptsList,
	})
	if list.Error != nil {
		t.Fatalf("prompts/list: error = %+v", list.Error)
	}
	listResult, ok := list.Result.(agentcapabilities.MCPListPromptsResult)
	if !ok {
		t.Fatalf("prompts/list result type = %T, want shared MCPListPromptsResult", list.Result)
	}
	if len(listResult.Prompts) != 3 {
		t.Fatalf("prompts/list returned %+v, want three prompts", listResult.Prompts)
	}

	params, _ := json.Marshal(agentcapabilities.MCPGetPromptParams{
		Name:      agentcapabilities.MCPPromptReviewFinding,
		Arguments: map[string]string{"finding_id": "finding-1"},
	})
	get := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  agentcapabilities.MCPMethodPromptsGet,
		Params:  params,
	})
	if get.Error != nil {
		t.Fatalf("prompts/get: error = %+v", get.Error)
	}
	getResult, ok := get.Result.(agentcapabilities.MCPGetPromptResult)
	if !ok {
		t.Fatalf("prompts/get result type = %T, want shared MCPGetPromptResult", get.Result)
	}
	if len(getResult.Messages) != 1 || getResult.Messages[0].Role != "user" {
		t.Fatalf("prompts/get messages = %+v", getResult.Messages)
	}
	if !strings.Contains(getResult.Messages[0].Content.Text, `"finding-1"`) || !strings.Contains(getResult.Messages[0].Content.Text, "list_findings") {
		t.Fatalf("finding prompt text = %q", getResult.Messages[0].Content.Text)
	}
}

func TestServer_PromptsGetRecordsWorkflowPromptActivity(t *testing.T) {
	var got struct {
		path    string
		token   string
		surface string
		name    string
	}
	client := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		got.path = r.URL.Path
		got.token = r.Header.Get(agentcapabilities.AgentAPITokenHeader)
		got.surface = r.Header.Get(agentcapabilities.AgentSurfaceHeader)
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode activity payload: %v", err)
		}
		got.name = payload["name"]
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})

	s := &mcpServer{
		baseURL: "http://pulse.local",
		token:   "test-token",
		http:    client,
		manifest: &agentCapabilitiesManifest{
			Version: "v1",
			WorkflowPrompts: []agentcapabilities.PulseWorkflowPrompt{{
				Name:        agentcapabilities.PulseWorkflowPromptOperationsLoop,
				Label:       "Ask Patrol to handle an issue",
				Description: "Have Patrol investigate, follow policy, take approved actions, verify, and record the result.",
			}},
		},
	}
	params, _ := json.Marshal(agentcapabilities.MCPGetPromptParams{Name: agentcapabilities.MCPPromptOperationsLoop})
	resp := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  agentcapabilities.MCPMethodPromptsGet,
		Params:  params,
	})
	if resp.Error != nil {
		t.Fatalf("prompts/get: error = %+v", resp.Error)
	}

	if got.path != agentcapabilities.AgentWorkflowPromptActivityPath {
		t.Fatalf("activity path = %q, want %q", got.path, agentcapabilities.AgentWorkflowPromptActivityPath)
	}
	if got.token != "test-token" {
		t.Fatalf("%s = %q, want test-token", agentcapabilities.AgentAPITokenHeader, got.token)
	}
	if got.surface != agentcapabilities.AgentSurfacePulseMCP {
		t.Fatalf("%s = %q, want %s", agentcapabilities.AgentSurfaceHeader, got.surface, agentcapabilities.AgentSurfacePulseMCP)
	}
	if got.name != agentcapabilities.PulseWorkflowPromptOperationsLoop {
		t.Fatalf("activity prompt name = %q, want %s", got.name, agentcapabilities.PulseWorkflowPromptOperationsLoop)
	}
}

func TestServerDispatchUsesSharedMCPDispatcher(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	text := string(src)

	if !strings.Contains(text, "agentcapabilities.DispatchMCPToolServerRequest(") {
		t.Fatal("pulse-mcp dispatch must delegate MCP method semantics to agentcapabilities")
	}
	for _, required := range []string{
		"s.manifestToolServer().Handlers(func()",
		"agentcapabilities.MCPManifestToolServer{",
		"ServerName:                   pulseMCPServerName",
		"SurfaceID:                    agentcapabilities.SurfaceIDPulseMCP",
		"Manifest:                     manifest",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("pulse-mcp must project manifest-backed tool semantics through agentcapabilities; missing %s", required)
		}
	}
	for _, required := range []string{
		"agentcapabilities.ServeJSONRPCLines(",
		"agentcapabilities.WriteJSONRPCMessage(out, v)",
		"agentcapabilities.StreamMCPEventNotifications(",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("pulse-mcp must delegate JSON-RPC request handling to agentcapabilities; missing %s", required)
		}
	}
	for _, forbidden := range []string{
		"bufio.NewScanner(in)",
		"scanner.Scan()",
		"agentcapabilities.DecodeJSONRPCRequest(",
		"agentcapabilities.NewJSONRPCParseErrorResponse(",
		"agentcapabilities.JSONRPCRequestExpectsResponse(",
		"agentcapabilities.StreamAgentSSERecords(",
		"agentcapabilities.NewMCPEventNotification(",
		"func (s *mcpServer) maybeEmitNotification(",
		"json.Unmarshal([]byte(line)",
		"agentcapabilities.NewJSONRPCErrorResponse(\n\t\t\t\tnil",
		"agentcapabilities.JSONRPCErrorParse",
		"len(req.ID) == 0",
		"string(req.ID) == \"null\"",
		"switch req.Method",
		"type jsonRPCError = agentcapabilities.JSONRPCError",
		"agentcapabilities.MCPToolServerHandlers{",
		"agentcapabilities.NewJSONRPCResponse(req.ID, nil)",
		"agentcapabilities.JSONRPCErrorInternal",
		"agentcapabilities.JSONRPCErrorMethodNotFound",
		"func (s *mcpServer) handleInitialize(",
		"func (s *mcpServer) handleToolsList(",
		"func (s *mcpServer) handleToolsCall(",
		"agentcapabilities.NewMCPToolServerInitializeResult(pulseMCPServerName",
		"agentcapabilities.ProjectTools(s.manifest.Capabilities)",
		"agentcapabilities.ExecuteMCPToolHTTP(",
		"agentcapabilities.ExecuteMCPManifestSurfaceToolHTTP(ctx, s.http, s.baseURL, s.token, s.manifest",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("pulse-mcp must not own MCP method dispatch or error mapping; found %s", forbidden)
		}
	}
}

// TestServer_ToolsListProjectsPulseMCPSurfaceContract pins the auto-generation
// rule: tools/list must surface the manifest-owned Pulse MCP tool contract,
// not every raw manifest capability. Streaming capabilities and capabilities
// omitted from the Pulse MCP contract stay out of the request/response tool
// surface.
func TestServer_ToolsListProjectsPulseMCPSurfaceContract(t *testing.T) {
	s := &mcpServer{manifest: &agentCapabilitiesManifest{
		Version:              "v1",
		SurfaceContract:      agentcapabilities.CanonicalManifest().SurfaceContract,
		SurfaceToolContracts: testPulseMCPSurfaceToolContracts(agentcapabilities.ResourceContextCapabilityName, agentcapabilities.SetOperatorStateCapabilityName),
		Capabilities: []agentCapability{
			{Name: "get_resource_context", Title: "Inspect resource", Path: "/api/agent/resource-context/{resourceId}", Method: http.MethodGet, Description: "depth", ActionMode: agentcapabilities.ActionModeRead, OutputSchema: json.RawMessage(`{"type":"object","properties":{"canonicalId":{"type":"string"}}}`)},
			{Name: "get_fleet_context", Title: "Triage fleet", Path: "/api/agent/fleet-context", Method: http.MethodGet, Description: "triage", ActionMode: agentcapabilities.ActionModeRead},
			{Name: "subscribe_events", Title: "Subscribe events", Path: "/api/agent/events", Method: http.MethodGet, Description: "stream", ActionMode: agentcapabilities.ActionModeRead},
			{Name: agentcapabilities.SetOperatorStateCapabilityName, Title: "Set operator state", Path: agentcapabilities.OperatorStateCapabilityPath, Method: http.MethodPut, Scope: "monitoring:write", Description: "write intent", ActionMode: agentcapabilities.ActionModeWrite},
		},
	}}
	resp := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/list",
	})
	if resp.Error != nil {
		t.Fatalf("tools/list: error = %+v", resp.Error)
	}
	result, _ := resp.Result.(agentcapabilities.MCPProjectedToolsResult)
	tools := result.Tools
	if len(tools) != 2 {
		t.Fatalf("tools/list len = %d, want 2 Pulse MCP surface tools", len(tools))
	}
	names := map[string]bool{}
	titles := map[string]string{}
	descriptions := map[string]string{}
	outputSchemas := map[string]json.RawMessage{}
	annotations := map[string]*agentcapabilities.MCPToolAnnotations{}
	metadata := map[string]map[string]any{}
	for _, tool := range tools {
		names[tool.Name] = true
		titles[tool.Name] = tool.Title
		descriptions[tool.Name] = tool.Description
		outputSchemas[tool.Name] = tool.OutputSchema
		annotations[tool.Name] = tool.Annotations
		if meta, ok := tool.Meta[agentcapabilities.ToolMetaPulseCapabilityKey].(map[string]any); ok {
			metadata[tool.Name] = meta
		}
	}
	for _, want := range []string{agentcapabilities.ResourceContextCapabilityName, agentcapabilities.SetOperatorStateCapabilityName} {
		if !names[want] {
			t.Errorf("tools/list missing %q", want)
		}
	}
	if names[agentcapabilities.FleetContextCapabilityName] {
		t.Errorf("%s must not be exposed when omitted from the Pulse MCP surface contract", agentcapabilities.FleetContextCapabilityName)
	}
	if !strings.Contains(descriptions[agentcapabilities.SetOperatorStateCapabilityName], "write intent") {
		t.Errorf("tools/list must preserve manifest description; got %q", descriptions[agentcapabilities.SetOperatorStateCapabilityName])
	}
	if titles[agentcapabilities.ResourceContextCapabilityName] != "Inspect resource" {
		t.Errorf("tools/list must project manifest title; got %q", titles[agentcapabilities.ResourceContextCapabilityName])
	}
	if !strings.Contains(string(outputSchemas[agentcapabilities.ResourceContextCapabilityName]), `"canonicalId"`) {
		t.Errorf("tools/list must project manifest outputSchema; got %s", string(outputSchemas[agentcapabilities.ResourceContextCapabilityName]))
	}
	if !strings.Contains(descriptions[agentcapabilities.SetOperatorStateCapabilityName], "required scope: ") {
		t.Errorf("tools/list must project manifest capability metadata; got %q", descriptions[agentcapabilities.SetOperatorStateCapabilityName])
	}
	setOperatorMeta := metadata[agentcapabilities.SetOperatorStateCapabilityName]
	if setOperatorMeta["scope"] != "monitoring:write" {
		t.Errorf("tools/list must project structured Pulse scope metadata; got %#v", setOperatorMeta)
	}
	route, _ := setOperatorMeta["route"].(map[string]any)
	if route["method"] != http.MethodPut || route["path"] != agentcapabilities.OperatorStateCapabilityPath {
		t.Errorf("tools/list must project structured Pulse route metadata; got %#v", route)
	}
	governance, _ := setOperatorMeta["governance"].(map[string]any)
	if governance["actionMode"] != string(agentcapabilities.ActionModeWrite) {
		t.Errorf("tools/list must project structured Pulse governance metadata; got %#v", governance)
	}
	assertMCPToolAnnotations(t, annotations[agentcapabilities.ResourceContextCapabilityName], true, false, true, true)
	assertMCPToolAnnotations(t, annotations[agentcapabilities.SetOperatorStateCapabilityName], false, true, false, true)
	if names["subscribe_events"] {
		t.Error("subscribe_events must NOT be exposed as an MCP tool — SSE streams don't fit the request/response shape")
	}
}

func assertMCPToolAnnotations(t *testing.T, annotations *agentcapabilities.MCPToolAnnotations, readOnly, destructive, idempotent, openWorld bool) {
	t.Helper()
	if annotations == nil {
		t.Fatal("tool annotations are nil")
	}
	assertMCPBoolRef(t, "readOnlyHint", annotations.ReadOnlyHint, readOnly)
	assertMCPBoolRef(t, "destructiveHint", annotations.DestructiveHint, destructive)
	assertMCPBoolRef(t, "idempotentHint", annotations.IdempotentHint, idempotent)
	assertMCPBoolRef(t, "openWorldHint", annotations.OpenWorldHint, openWorld)
}

func assertMCPBoolRef(t *testing.T, name string, got *bool, want bool) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s is nil, want %v", name, want)
	}
	if *got != want {
		t.Fatalf("%s = %v, want %v", name, *got, want)
	}
}

func TestFormatToolDescriptionProjectsCapabilityMetadata(t *testing.T) {
	desc := agentcapabilities.ToolDescription(agentCapability{
		Name:             "plan_action",
		Description:      "Plan an action against a resource.",
		Category:         "action",
		Method:           http.MethodPost,
		Path:             "/api/actions/plan",
		Scope:            "ai:execute",
		ActionMode:       "write",
		ApprovalPolicy:   "action_plan",
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

func TestServer_ToolsCallRejectsUnknownManifestCapability(t *testing.T) {
	s := &mcpServer{manifest: &agentCapabilitiesManifest{
		Version: "v1",
		Capabilities: []agentCapability{
			{Name: "get_fleet_context", Path: "/api/agent/fleet-context", Method: http.MethodGet},
		},
	}}

	params, _ := json.Marshal(map[string]any{
		"name":      "missing_capability",
		"arguments": map[string]any{},
	})
	resp := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  params,
	})

	if resp.Error == nil {
		t.Fatal("unknown manifest capability must produce a JSON-RPC error")
	}
	if !strings.Contains(resp.Error.Message, "unknown tool: missing_capability") {
		t.Fatalf("unknown capability error = %q, want stable unknown tool message", resp.Error.Message)
	}
}

func TestServer_ToolsCallRejectsMalformedParamsWithSharedDecodeMessage(t *testing.T) {
	s := &mcpServer{manifest: &agentCapabilitiesManifest{Version: "v1"}}
	resp := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":`),
	})

	if resp.Error == nil {
		t.Fatal("malformed tools/call params must produce a JSON-RPC error")
	}
	if resp.Error.Code != agentcapabilities.JSONRPCErrorInternal {
		t.Fatalf("error code = %d, want JSONRPCErrorInternal", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "decode tools/call params") {
		t.Fatalf("decode error = %q, want shared decode message", resp.Error.Message)
	}

	resp = s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"  ","arguments":{}}`),
	})
	if resp.Error == nil {
		t.Fatal("invalid tools/call params must produce a JSON-RPC error")
	}
	if !strings.Contains(resp.Error.Message, "decode tools/call params: tool name is required") {
		t.Fatalf("invalid params error = %q, want shared validation message", resp.Error.Message)
	}
}

// TestServer_ToolsCallProxiesToPulseAndPreservesErrorEnvelope
// pins the substantive contract: a tools/call invocation makes
// the right HTTP request to Pulse with the bearer token, and
// preserves the substrate's stable error envelope verbatim so
// agents on the MCP side branch on the same `error` code they
// would on the wire.
func TestServer_ToolsCallProxiesToPulseAndPreservesErrorEnvelope(t *testing.T) {
	type captured struct {
		method  string
		path    string
		token   string
		surface string
		body    string
	}
	var got captured
	mu := sync.Mutex{}
	const errorBody = `{"error":"resource_not_found","message":"No resource is registered with this canonical id."}`
	client := httpDoerFunc(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		got.method = r.Method
		got.path = r.URL.Path
		got.token = r.Header.Get("X-API-Token")
		got.surface = r.Header.Get(agentcapabilities.AgentSurfaceHeader)
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			got.body = string(b)
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(errorBody)),
			Request:    r,
		}, nil
	})

	s := &mcpServer{
		baseURL: "http://pulse.test",
		token:   "test-token",
		manifest: &agentCapabilitiesManifest{
			Version:              "v1",
			SurfaceContract:      agentcapabilities.CanonicalManifest().SurfaceContract,
			SurfaceToolContracts: testPulseMCPSurfaceToolContracts(agentcapabilities.ResourceContextCapabilityName),
			Capabilities: []agentCapability{
				{
					Name:   "get_resource_context",
					Path:   "/api/agent/resource-context/{resourceId}",
					Method: http.MethodGet,
					Scope:  "monitoring:read",
				},
			},
		},
		http: client,
	}

	params, _ := json.Marshal(map[string]any{
		"name":      "get_resource_context",
		"arguments": map[string]any{"resourceId": "vm:does-not-exist"},
	})
	resp := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  params,
	})
	if resp.Error != nil {
		t.Fatalf("tools/call: rpc error = %+v", resp.Error)
	}

	if got.method != http.MethodGet {
		t.Errorf("upstream method = %q, want GET", got.method)
	}
	if got.path != "/api/agent/resource-context/vm:does-not-exist" {
		t.Errorf("upstream path = %q; placeholder must be substituted", got.path)
	}
	if got.token != "test-token" {
		t.Errorf("upstream token header = %q, want test-token", got.token)
	}
	if got.surface != agentcapabilities.AgentSurfacePulseMCP {
		t.Errorf("upstream agent surface header = %q, want %q", got.surface, agentcapabilities.AgentSurfacePulseMCP)
	}

	result, ok := resp.Result.(agentcapabilities.MCPToolResult)
	if !ok {
		t.Fatalf("tools/call result type = %T, want shared MCPToolResult", resp.Result)
	}
	if !result.IsError {
		t.Errorf("non-2xx upstream must surface as MCP isError=true; got %v", result.IsError)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content len = %d, want 1", len(result.Content))
	}
	expected := agentcapabilities.NewCapabilityHTTPToolResult(agentcapabilities.HTTPCallResponse{
		Method:     http.MethodGet,
		Path:       "/api/agent/resource-context/vm:does-not-exist",
		StatusCode: http.StatusNotFound,
		Body:       []byte(errorBody),
	})
	if result.IsError != expected.IsError || result.Content[0].Text != expected.Content[0].Text {
		t.Fatalf("tools/call result must use shared HTTP-to-MCP result semantics; got %+v want %+v", result, expected)
	}
	if result.StructuredContent["error"] != "resource_not_found" {
		t.Fatalf("tools/call structuredContent = %+v, want resource_not_found error", result.StructuredContent)
	}
	if !strings.Contains(result.Content[0].Text, `"error":"resource_not_found"`) {
		t.Errorf("MCP content must preserve substrate error envelope verbatim; got %v", result.Content[0].Text)
	}
}

func TestServer_DispatchUnknownMethodReturnsMethodNotFound(t *testing.T) {
	s := &mcpServer{manifest: &agentCapabilitiesManifest{}}
	resp := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "this/is/not/a/real/method",
	})
	if resp.Error == nil {
		t.Fatal("unknown method must produce a JSON-RPC error so MCP clients fail loudly rather than hang")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601 (method not found)", resp.Error.Code)
	}
}

// TestServer_NotificationGetsNoResponse pins the JSON-RPC 2.0
// rule that notifications (id absent) produce no response. MCP
// uses notifications for things like progress updates — we don't
// initiate any, but the server must still handle them silently
// when a client sends one.
func TestServer_NotificationGetsNoResponse(t *testing.T) {
	s := &mcpServer{manifest: &agentCapabilitiesManifest{}}
	in := bytes.NewReader([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"))
	out := &bytes.Buffer{}
	s.serve(in, out)
	if out.Len() > 0 {
		t.Errorf("notification produced output; want silent. got: %s", out.String())
	}
}

// TestServer_InitializeAdvertisesNotificationsCapabilityWhenEnabled
// pins the discovery contract for the SSE bridge: when
// --emit-notifications is on, the initialize response advertises
// the kinds an MCP client can expect under
// experimental.pulseNotifications.kinds. Drift in either
// direction (advertising when disabled, or omitting when enabled)
// breaks client expectations about whether to wait for pushes.
func TestServer_InitializeAdvertisesNotificationsCapabilityWhenEnabled(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		s := &mcpServer{manifest: &agentCapabilitiesManifest{}}
		result := s.manifestToolServer().Initialize()
		if len(result.Capabilities.Experimental) > 0 {
			t.Error("initialize must NOT advertise pulseNotifications when --emit-notifications is off")
		}
	})

	t.Run("advertised when enabled", func(t *testing.T) {
		s := &mcpServer{manifest: &agentCapabilitiesManifest{}, emitNotifications: true}
		result := s.manifestToolServer().Initialize()
		exp := result.Capabilities.Experimental
		if len(exp) == 0 {
			t.Fatal("initialize must advertise experimental block when --emit-notifications is on")
		}
		pn, ok := exp[agentcapabilities.MCPPulseNotificationsExperimentalKey].(agentcapabilities.MCPPulseNotificationsCapability)
		if !ok {
			t.Fatal("experimental block must contain pulseNotifications descriptor")
		}
		kinds := pn.Kinds
		if len(kinds) == 0 {
			t.Fatalf("pulseNotifications.kinds must list the SSE event kinds; got %v", pn.Kinds)
		}
		want := map[string]bool{}
		for _, kind := range agentcapabilities.AgentActionableEventKinds() {
			want[kind] = false
		}
		for _, k := range kinds {
			if _, exists := want[k]; exists {
				want[k] = true
			}
		}
		for k, seen := range want {
			if !seen {
				t.Errorf("pulseNotifications.kinds missing %q", k)
			}
		}
	})
}

// TestServer_StreamSSEOnceTranslatesEventsToNotifications is the
// integration test for the bridge: spin up a fake SSE server
// emitting the substrate's wire format, point the consumer at it,
// and assert each non-transport event lands as a JSON-RPC
// notification on the configured out writer.
func TestServer_StreamSSEOnceTranslatesEventsToNotifications(t *testing.T) {
	sseBody := strings.Join([]string{
		": sync comment uses standard SSE comment framing",
		"event: " + string(agentcapabilities.EventKindStreamConnected),
		"data: {}",
		"",
		"event: " + string(agentcapabilities.EventKindFindingCreated),
		"data: {\"findingId\":\"f1\",\"severity\":\"critical\"}",
		"",
		"event: " + string(agentcapabilities.EventKindHeartbeat),
		"",
		"event: " + string(agentcapabilities.EventKindActionCompleted),
		"data: {\"actionId\":\"x1\",\"success\":true,\"verification\":{\"ran\":true,\"success\":true,\"commandRedacted\":true}}",
		"",
	}, "\r\n") + "\r\n"

	pulse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(agentcapabilities.AgentAPITokenHeader) != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Accept") != agentcapabilities.AgentSSEAccept {
			t.Errorf("Accept header = %q, want %q", r.Header.Get("Accept"), agentcapabilities.AgentSSEAccept)
		}
		if r.Header.Get(agentcapabilities.AgentSurfaceHeader) != agentcapabilities.AgentSurfacePulseMCP {
			t.Errorf("%s header = %q, want %q", agentcapabilities.AgentSurfaceHeader, r.Header.Get(agentcapabilities.AgentSurfaceHeader), agentcapabilities.AgentSurfacePulseMCP)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sseBody))
	}))
	defer pulse.Close()

	out := &bytes.Buffer{}
	s := &mcpServer{
		baseURL: pulse.URL,
		token:   "test-token",
		http:    pulse.Client(),
		out:     out,
	}
	if err := s.streamSSEOnce(context.Background(), agentcapabilities.AgentEventsPath); err != nil {
		t.Fatalf("streamSSEOnce: %v", err)
	}

	body := out.String()
	if !strings.Contains(body, `"method":"notifications/finding.created"`) {
		t.Errorf("missing finding.created notification; got %s", body)
	}
	if !strings.Contains(body, `"method":"notifications/action.completed"`) {
		t.Errorf("missing action.completed notification; got %s", body)
	}
	if !strings.Contains(body, `"verification":{"ran":true,"success":true,"commandRedacted":true}`) {
		t.Errorf("action.completed verification must round-trip through MCP notification params; got %s", body)
	}
	if strings.Contains(body, string(agentcapabilities.EventKindStreamConnected)) {
		t.Errorf("stream.connected must be filtered out as transport plumbing; got %s", body)
	}
	if strings.Contains(body, `"method":"notifications/heartbeat"`) {
		t.Errorf("heartbeat must be filtered out; got %s", body)
	}
	// The payload data must round-trip verbatim so agents see the
	// substrate's wire shape unchanged.
	if !strings.Contains(body, `"findingId":"f1"`) {
		t.Errorf("notification params must round-trip the SSE data field; got %s", body)
	}
}

// TestServer_ToolsCallSendsPutBodyForWriteCapabilities pins the
// write path the existing tools/call test (read-side, GET only)
// did not cover: when an agent calls a non-GET/DELETE capability
// like set_operator_state, the bridge must (a) substitute the
// path placeholder, (b) marshal the supplied body into the
// request body, (c) set Content-Type: application/json, and
// (d) return the upstream success body in the MCP content block
// with isError=false. Drift in any of those would either drop
// the agent's data on the floor or surface success as failure
// (and vice versa).
func TestServer_ToolsCallSendsPutBodyForWriteCapabilities(t *testing.T) {
	type captured struct {
		method      string
		path        string
		token       string
		contentType string
		body        string
	}
	var got captured
	mu := sync.Mutex{}
	pulse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		got.method = r.Method
		got.path = r.URL.Path
		got.token = r.Header.Get("X-API-Token")
		got.contentType = r.Header.Get("Content-Type")
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			got.body = string(b)
		}
		// Mirror Pulse's PUT response: 200 with the canonical
		// state shape, including server-populated attribution
		// (setAt) the client cannot spoof.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"canonicalId":"vm:101","intentionallyOffline":true,"setAt":"2026-05-10T10:00:00Z","setBy":"agent:test-token"}`))
	}))
	defer pulse.Close()

	s := &mcpServer{
		baseURL: pulse.URL,
		token:   "write-test-token",
		manifest: &agentCapabilitiesManifest{
			Version:              "v1",
			SurfaceContract:      agentcapabilities.CanonicalManifest().SurfaceContract,
			SurfaceToolContracts: testPulseMCPSurfaceToolContracts(agentcapabilities.SetOperatorStateCapabilityName),
			Capabilities: []agentCapability{
				{
					Name:             agentcapabilities.SetOperatorStateCapabilityName,
					Path:             agentcapabilities.OperatorStateCapabilityPath,
					Method:           http.MethodPut,
					Scope:            "monitoring:write",
					RequestBodyShape: "ResourceOperatorStateInput",
					ErrorCodes:       []string{"operator_state_invalid"},
				},
			},
		},
		http: pulse.Client(),
	}

	params, _ := json.Marshal(map[string]any{
		"name": agentcapabilities.SetOperatorStateCapabilityName,
		"arguments": map[string]any{
			"resourceId": "vm:101",
			"body": map[string]any{
				"intentionallyOffline": true,
				"note":                 "decommissioned for hardware refresh",
			},
		},
	})
	resp := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  params,
	})
	if resp.Error != nil {
		t.Fatalf("tools/call: rpc error = %+v", resp.Error)
	}

	// Upstream request shape.
	if got.method != http.MethodPut {
		t.Errorf("upstream method = %q, want PUT", got.method)
	}
	if got.path != "/api/resources/vm:101/operator-state" {
		t.Errorf("upstream path = %q; placeholder must be substituted with the resourceId arg", got.path)
	}
	if got.token != "write-test-token" {
		t.Errorf("upstream X-API-Token = %q, want write-test-token", got.token)
	}
	if got.contentType != "application/json" {
		t.Errorf("upstream Content-Type = %q, want application/json", got.contentType)
	}

	// The body must carry the agent-supplied JSON object verbatim.
	// We don't pin exact whitespace because json.Marshal is free
	// to reorder map keys, but the field-value pairs must all be
	// present and parseable.
	var sentBody map[string]any
	if err := json.Unmarshal([]byte(got.body), &sentBody); err != nil {
		t.Fatalf("upstream body must be parseable JSON; got %q (%v)", got.body, err)
	}
	if sentBody["intentionallyOffline"] != true {
		t.Errorf("upstream body must round-trip intentionallyOffline=true; got %v", sentBody["intentionallyOffline"])
	}
	if sentBody["note"] != "decommissioned for hardware refresh" {
		t.Errorf("upstream body must round-trip note verbatim; got %v", sentBody["note"])
	}

	// MCP result shape: 2xx upstream becomes isError=false with the upstream
	// body in a text content block plus shared structuredContent for clients
	// that can branch on machine-readable tool output.
	result, ok := resp.Result.(agentcapabilities.MCPToolResult)
	if !ok {
		t.Fatalf("tools/call result type = %T, want shared MCPToolResult", resp.Result)
	}
	if result.IsError {
		t.Errorf("2xx upstream must surface as isError=false; got %v", result.IsError)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content len = %d, want 1", len(result.Content))
	}
	text := result.Content[0].Text
	if !strings.Contains(text, `"canonicalId":"vm:101"`) {
		t.Errorf("MCP content must carry upstream response body; got %q", text)
	}
	if result.StructuredContent["canonicalId"] != "vm:101" {
		t.Fatalf("MCP structuredContent = %+v, want canonicalId vm:101", result.StructuredContent)
	}
	// Server-populated attribution must reach the agent so it
	// can see WHO the substrate recorded the write under (not the
	// supplied token, but the resolved actor id).
	if !strings.Contains(text, `"setBy":"agent:test-token"`) {
		t.Errorf("MCP content must surface server-populated setBy field; got %q", text)
	}
}

// TestServer_ToolsCallTopLevelArgsMakeUpRequestBody pins the
// flexibility on the body argument: when an agent passes the
// body fields at the top level of arguments (no nested "body"
// key), the bridge collects everything other than path
// placeholders into the request body. This is the shape MCP
// clients tend to generate when they read the input schema as
// "object with these fields"; the bridge must accept both shapes
// so neither "body": {...} wrappers NOR top-level fields fail.
func TestServer_ToolsCallTopLevelArgsMakeUpRequestBody(t *testing.T) {
	var captured string
	pulse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		captured = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer pulse.Close()

	s := &mcpServer{
		baseURL: pulse.URL,
		token:   "test",
		manifest: &agentCapabilitiesManifest{
			SurfaceContract:      agentcapabilities.CanonicalManifest().SurfaceContract,
			SurfaceToolContracts: testPulseMCPSurfaceToolContracts(agentcapabilities.AcknowledgeFindingCapabilityName),
			Capabilities: []agentCapability{
				{
					Name:   agentcapabilities.AcknowledgeFindingCapabilityName,
					Path:   "/api/ai/patrol/acknowledge",
					Method: http.MethodPost,
				},
			},
		},
		http: pulse.Client(),
	}

	params, _ := json.Marshal(map[string]any{
		"name": "acknowledge_finding",
		"arguments": map[string]any{
			// No "body" key — fields are top-level.
			"finding_id": "f-123",
		},
	})
	resp := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  params,
	})
	if resp.Error != nil {
		t.Fatalf("tools/call: rpc error = %+v", resp.Error)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(captured), &sent); err != nil {
		t.Fatalf("captured body must be parseable JSON; got %q", captured)
	}
	if sent["finding_id"] != "f-123" {
		t.Errorf("top-level finding_id must reach the upstream body; got %v", sent["finding_id"])
	}
}

// TestServer_ToolsCallTopLevelArgsExcludesPathPlaceholders pins
// the disambiguation rule when both a path placeholder and a
// body field happen to be present at the top level of arguments:
// the placeholder value goes ONLY in the URL, never duplicated
// into the request body. Otherwise an agent that follows the
// auto-derived schema (which lists path placeholders as
// properties) would accidentally send {"resourceId": "vm:101",
// ...} as the PUT body and confuse a server that doesn't expect
// canonicalId duplication.
func TestServer_ToolsCallTopLevelArgsExcludesPathPlaceholders(t *testing.T) {
	type captured struct {
		path       string
		requestURI string
		body       string
	}
	var got captured
	pulse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.requestURI = r.RequestURI
		b, _ := io.ReadAll(r.Body)
		got.body = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer pulse.Close()

	s := &mcpServer{
		baseURL: pulse.URL,
		token:   "test",
		manifest: &agentCapabilitiesManifest{
			SurfaceContract:      agentcapabilities.CanonicalManifest().SurfaceContract,
			SurfaceToolContracts: testPulseMCPSurfaceToolContracts(agentcapabilities.SetOperatorStateCapabilityName),
			Capabilities: []agentCapability{
				{
					Name:   agentcapabilities.SetOperatorStateCapabilityName,
					Path:   agentcapabilities.OperatorStateCapabilityPath,
					Method: http.MethodPut,
				},
			},
		},
		http: pulse.Client(),
	}

	params, _ := json.Marshal(map[string]any{
		"name": agentcapabilities.SetOperatorStateCapabilityName,
		"arguments": map[string]any{
			"resourceId":           "vm/special 101",
			"intentionallyOffline": true,
		},
	})
	resp := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  params,
	})
	if resp.Error != nil {
		t.Fatalf("tools/call: rpc error = %+v", resp.Error)
	}

	if got.requestURI != "/api/resources/vm%2Fspecial%20101/operator-state" {
		t.Errorf("path-placeholder substitution failed; got request URI %q", got.requestURI)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(got.body), &sent); err != nil {
		t.Fatalf("body must be JSON; got %q", got.body)
	}
	if _, has := sent["resourceId"]; has {
		t.Errorf("path placeholder must NOT be duplicated into the body; got %v", sent)
	}
	if sent["intentionallyOffline"] != true {
		t.Errorf("non-placeholder fields must reach the body; got %v", sent)
	}
}
