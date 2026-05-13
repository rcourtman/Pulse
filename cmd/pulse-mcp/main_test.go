package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// TestBuildInputSchema_PathPlaceholdersBecomeRequiredStringProps
// pins the rule that turns capability paths into MCP tool input
// schemas: every {name} segment in the path becomes a required
// string property the agent must supply, with a description that
// hints at the canonical shape ("vm:101", "container:web-1") so
// the LLM forms the right id without back-and-forth.
func TestBuildInputSchema_PathPlaceholdersBecomeRequiredStringProps(t *testing.T) {
	cap := agentCapability{
		Name:   "get_resource_context",
		Path:   "/api/agent/resource-context/{resourceId}",
		Method: http.MethodGet,
	}
	raw := buildInputSchema(cap)
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

// TestBuildInputSchema_NonGetCapabilitiesAcceptBody pins that
// non-GET/DELETE tools expose a `body` property the agent fills
// with the request payload. GET tools must NOT advertise a body
// property so the agent doesn't try to send one (which would be
// dropped by net/http anyway, but advertising it would be
// misleading).
func TestBuildInputSchema_NonGetCapabilitiesAcceptBody(t *testing.T) {
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
			raw := buildInputSchema(tc.cap)
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

func TestSubstitutePathParams_FillsAllPlaceholders(t *testing.T) {
	got, err := substitutePathParams(
		"/api/agent/resource-context/{resourceId}",
		map[string]any{"resourceId": "vm:101"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/api/agent/resource-context/vm:101" {
		t.Errorf("got %q, want /api/agent/resource-context/vm:101", got)
	}
}

func TestSubstitutePathParams_MissingPlaceholderIsAStableError(t *testing.T) {
	// The agent must get a clear error when it forgets a path
	// argument — better to fail with "missing path argument
	// resourceId" than to send a literal `{resourceId}` URL to
	// Pulse and get a confusing 404.
	_, err := substitutePathParams(
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

func TestSubstitutePathParams_NonStringIsAnError(t *testing.T) {
	_, err := substitutePathParams(
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
	s := &mcpServer{manifest: &agentCapabilitiesManifest{Version: "v1"}}
	resp := s.dispatch(context.Background(), &jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
	})
	if resp.Error != nil {
		t.Fatalf("initialize: error = %+v", resp.Error)
	}
	result, _ := resp.Result.(map[string]any)
	caps, _ := result["capabilities"].(map[string]any)
	if _, ok := caps["tools"]; !ok {
		t.Fatal("initialize must advertise tools capability so MCP clients enumerate the tool surface")
	}
	info, _ := result["serverInfo"].(map[string]any)
	if info["name"] != "pulse-mcp" {
		t.Errorf("serverInfo.name = %v, want pulse-mcp", info["name"])
	}
}

// TestServer_ToolsListProjectsManifestSkippingSubscribeEvents
// pins the auto-generation rule: tools/list must surface every
// manifest capability except subscribe_events (which is a stream,
// not a tool). Adding a capability to the manifest must
// automatically make it visible to MCP clients without changes
// here.
func TestServer_ToolsListProjectsManifestSkippingSubscribeEvents(t *testing.T) {
	s := &mcpServer{manifest: &agentCapabilitiesManifest{
		Version: "v1",
		Capabilities: []agentCapability{
			{Name: "get_resource_context", Path: "/api/agent/resource-context/{resourceId}", Method: http.MethodGet, Description: "depth"},
			{Name: "get_fleet_context", Path: "/api/agent/fleet-context", Method: http.MethodGet, Description: "triage"},
			{Name: "subscribe_events", Path: "/api/agent/events", Method: http.MethodGet, Description: "stream"},
			{Name: "set_operator_state", Path: "/api/resources/{resourceId}/operator-state", Method: http.MethodPut, Description: "write intent"},
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
	result, _ := resp.Result.(map[string]any)
	tools, _ := result["tools"].([]mcpTool)
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"get_resource_context", "get_fleet_context", "set_operator_state"} {
		if !names[want] {
			t.Errorf("tools/list missing %q", want)
		}
	}
	if names["subscribe_events"] {
		t.Error("subscribe_events must NOT be exposed as an MCP tool — SSE streams don't fit the request/response shape; future slices can layer notifications")
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
		method string
		path   string
		token  string
		body   string
	}
	var got captured
	mu := sync.Mutex{}
	pulse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		got.method = r.Method
		got.path = r.URL.Path
		got.token = r.Header.Get("X-API-Token")
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			got.body = string(b)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"resource_not_found","message":"No resource is registered with this canonical id."}`))
	}))
	defer pulse.Close()

	s := &mcpServer{
		baseURL: pulse.URL,
		token:   "test-token",
		manifest: &agentCapabilitiesManifest{
			Version: "v1",
			Capabilities: []agentCapability{
				{
					Name:   "get_resource_context",
					Path:   "/api/agent/resource-context/{resourceId}",
					Method: http.MethodGet,
					Scope:  "monitoring:read",
				},
			},
		},
		http: pulse.Client(),
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

	result, _ := resp.Result.(map[string]any)
	if result["isError"] != true {
		t.Errorf("non-2xx upstream must surface as MCP isError=true; got %v", result["isError"])
	}
	content, _ := result["content"].([]map[string]any)
	if len(content) != 1 {
		t.Fatalf("content len = %d, want 1", len(content))
	}
	if !strings.Contains(content[0]["text"].(string), `"error":"resource_not_found"`) {
		t.Errorf("MCP content must preserve substrate error envelope verbatim; got %v", content[0]["text"])
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
		result := s.handleInitialize().(map[string]any)
		caps := result["capabilities"].(map[string]any)
		if _, ok := caps["experimental"]; ok {
			t.Error("initialize must NOT advertise pulseNotifications when --emit-notifications is off")
		}
	})

	t.Run("advertised when enabled", func(t *testing.T) {
		s := &mcpServer{manifest: &agentCapabilitiesManifest{}, emitNotifications: true}
		result := s.handleInitialize().(map[string]any)
		caps := result["capabilities"].(map[string]any)
		exp, ok := caps["experimental"].(map[string]any)
		if !ok {
			t.Fatal("initialize must advertise experimental block when --emit-notifications is on")
		}
		pn, ok := exp["pulseNotifications"].(map[string]any)
		if !ok {
			t.Fatal("experimental block must contain pulseNotifications descriptor")
		}
		kinds, ok := pn["kinds"].([]string)
		if !ok || len(kinds) == 0 {
			t.Fatalf("pulseNotifications.kinds must list the SSE event kinds; got %v", pn["kinds"])
		}
		want := map[string]bool{"finding.created": false, "approval.pending": false, "action.completed": false}
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

// TestServer_MaybeEmitNotificationFiltersTransportEvents pins that
// the bridge skips events that are pure transport plumbing
// (stream.connected, heartbeat). Forwarding those would surface
// noise an agent has no useful action on, and would teach
// downstream code to filter them client-side instead of trusting
// the substrate's "doorbell, not transport" intent.
func TestServer_MaybeEmitNotificationFiltersTransportEvents(t *testing.T) {
	cases := []struct {
		name         string
		event, data  string
		shouldEmit   bool
		methodPrefix string
	}{
		{"finding.created passes through", "finding.created", `{"findingId":"f1"}`, true, "notifications/finding.created"},
		{"approval.pending passes through", "approval.pending", `{"approvalId":"a1"}`, true, "notifications/approval.pending"},
		{"action.completed passes through", "action.completed", `{"actionId":"x1"}`, true, "notifications/action.completed"},
		{"stream.connected is filtered", "stream.connected", `{}`, false, ""},
		{"heartbeat is filtered", "heartbeat", "", false, ""},
		{"empty event is filtered", "", `{"x":1}`, false, ""},
		{"empty data is filtered", "finding.created", "", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			s := &mcpServer{out: out}
			s.maybeEmitNotification(tc.event, tc.data)
			if tc.shouldEmit {
				if out.Len() == 0 {
					t.Fatalf("expected notification on stdout for %q; got nothing", tc.event)
				}
				body := out.String()
				if !strings.Contains(body, `"method":"`+tc.methodPrefix+`"`) {
					t.Errorf("expected method %q; got %s", tc.methodPrefix, body)
				}
				if !strings.Contains(body, `"jsonrpc":"2.0"`) {
					t.Errorf("notification must be JSON-RPC 2.0; got %s", body)
				}
				// Notifications must NOT carry an id field per
				// JSON-RPC 2.0 spec — clients that see an id treat
				// the message as a request and may try to respond.
				if strings.Contains(body, `"id":`) {
					t.Errorf("notification must omit the id field; got %s", body)
				}
			} else {
				if out.Len() != 0 {
					t.Errorf("expected silence for %q event; got %s", tc.event, out.String())
				}
			}
		})
	}
}

// TestServer_StreamSSEOnceTranslatesEventsToNotifications is the
// integration test for the bridge: spin up a fake SSE server
// emitting the substrate's wire format, point the consumer at it,
// and assert each non-transport event lands as a JSON-RPC
// notification on the configured out writer.
func TestServer_StreamSSEOnceTranslatesEventsToNotifications(t *testing.T) {
	sseBody := strings.Join([]string{
		"event: stream.connected",
		"data: {}",
		"",
		"event: finding.created",
		"data: {\"findingId\":\"f1\",\"severity\":\"critical\"}",
		"",
		"event: heartbeat",
		"",
		"event: action.completed",
		"data: {\"actionId\":\"x1\",\"success\":true,\"verification\":{\"ran\":true,\"success\":true,\"commandRedacted\":true}}",
		"",
	}, "\n") + "\n"

	pulse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Token") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
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
	if err := s.streamSSEOnce(context.Background(), pulse.URL+"/api/agent/events"); err != nil {
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
	if strings.Contains(body, "stream.connected") {
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
			Version: "v1",
			Capabilities: []agentCapability{
				{
					Name:             "set_operator_state",
					Path:             "/api/resources/{resourceId}/operator-state",
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
		"name": "set_operator_state",
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

	// MCP result shape: 2xx upstream becomes isError=false with
	// the upstream body in a text content block.
	result, _ := resp.Result.(map[string]any)
	if result["isError"] != false {
		t.Errorf("2xx upstream must surface as isError=false; got %v", result["isError"])
	}
	content, _ := result["content"].([]map[string]any)
	if len(content) != 1 {
		t.Fatalf("content len = %d, want 1", len(content))
	}
	text, _ := content[0]["text"].(string)
	if !strings.Contains(text, `"canonicalId":"vm:101"`) {
		t.Errorf("MCP content must carry upstream response body; got %q", text)
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
			Capabilities: []agentCapability{
				{
					Name:   "acknowledge_finding",
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
		path string
		body string
	}
	var got captured
	pulse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
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
			Capabilities: []agentCapability{
				{
					Name:   "set_operator_state",
					Path:   "/api/resources/{resourceId}/operator-state",
					Method: http.MethodPut,
				},
			},
		},
		http: pulse.Client(),
	}

	params, _ := json.Marshal(map[string]any{
		"name": "set_operator_state",
		"arguments": map[string]any{
			"resourceId":           "vm:101",
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

	if got.path != "/api/resources/vm:101/operator-state" {
		t.Errorf("path-placeholder substitution failed; got %q", got.path)
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
