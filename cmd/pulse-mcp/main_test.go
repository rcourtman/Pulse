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
		"data: {\"actionId\":\"x1\",\"success\":true}",
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
