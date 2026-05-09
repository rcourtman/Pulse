package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// TestAgentSubstrate_DiscoveryToTriageToDepthFlowsThroughHTTPBoundary is
// the end-to-end contract proof for the agent-paradigm read substrate.
// Unit tests cover each piece in isolation; this test boots the full
// router stack and proves the discovery → triage → depth chain works
// as one substrate through the actual HTTP boundary an external agent
// would hit.
//
// Specifically:
//   - GET /api/agent/capabilities returns the v1 manifest with
//     get_resource_context, get_fleet_context, and subscribe_events
//     declared. This is the discovery contract — agents fetch this
//     once at startup and learn what's available.
//   - The manifest's declared path for get_fleet_context actually
//     resolves and returns the iteration-safe AgentFleetContext shape
//     (resources is an array, never null). This is the triage entry
//     point: "where do I focus?"
//   - The manifest's declared path for get_resource_context returns
//     the stable resource_not_found error code for an unknown id,
//     not a generic 404 with human text. This is the depth entry
//     point's documented error contract — agents branch on the code,
//     not on the human message.
//   - All three endpoints sit behind monitoring:read auth, exercised
//     via the X-API-Token transport.
//
// If any of these break, an external agent walking the substrate the
// way the manifest documents will silently fail in a way the
// per-piece unit tests can't catch.
func TestAgentSubstrate_DiscoveryToTriageToDepthFlowsThroughHTTPBoundary(t *testing.T) {
	rawToken := "agent-substrate-e2e-test.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	// --- 1. Discovery: fetch the capabilities manifest. ---
	// Intentionally unauthenticated — the manifest itself is public
	// (the underlying capabilities have their own auth scopes); the
	// discovery contract is that an agent can introspect Pulse before
	// holding a token.
	req := httptest.NewRequest(http.MethodGet, "/api/agent/capabilities", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("capabilities GET: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("capabilities Content-Type: got %q, want application/json", got)
	}
	var manifest AgentCapabilitiesManifest
	if err := json.NewDecoder(rec.Body).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.Version != "v1" {
		t.Fatalf("manifest version: got %q, want v1", manifest.Version)
	}

	// Index by name so we can pull the declared path / shape for the
	// next steps without depending on slice ordering.
	byName := map[string]AgentCapability{}
	for _, c := range manifest.Capabilities {
		byName[c.Name] = c
	}
	required := []string{
		"get_resource_context",
		"get_fleet_context",
		"subscribe_events",
		"get_operator_state",
		"set_operator_state",
		"clear_operator_state",
		"list_findings",
		"acknowledge_finding",
		"snooze_finding",
		"dismiss_finding",
		"resolve_finding",
	}
	for _, name := range required {
		if _, ok := byName[name]; !ok {
			t.Errorf("manifest missing required capability %q — discovery contract broken", name)
		}
	}

	// --- 2. Triage: call the path the manifest declares for the
	// fleet view. The declared shape is "AgentFleetContext", which
	// must round-trip through the actual handler. ---
	fleetCap := byName["get_fleet_context"]
	if fleetCap.Path != "/api/agent/fleet-context" {
		t.Fatalf("get_fleet_context path: got %q, want /api/agent/fleet-context", fleetCap.Path)
	}
	if fleetCap.Method != http.MethodGet {
		t.Fatalf("get_fleet_context method: got %q, want GET", fleetCap.Method)
	}
	if fleetCap.Scope != "monitoring:read" {
		t.Fatalf("get_fleet_context scope: got %q, want monitoring:read", fleetCap.Scope)
	}
	if fleetCap.ResponseShape != "AgentFleetContext" {
		t.Fatalf("get_fleet_context response shape: got %q, want AgentFleetContext", fleetCap.ResponseShape)
	}

	// Unauthenticated call must be rejected — the underlying
	// capability's own auth scope holds even though discovery is
	// unauthenticated.
	req = httptest.NewRequest(http.MethodGet, fleetCap.Path, nil)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("fleet GET unauth: status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}

	// Authenticated call must serve the iteration-safe shape.
	req = httptest.NewRequest(http.MethodGet, fleetCap.Path, nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("fleet GET auth: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// Empty registry (no monitor wired) must surface as `resources:
	// []`, never null. Pin the wire form rather than the decoded
	// length so a regression that turns the empty slice into null is
	// caught here even though Go would happily decode either.
	if !strings.Contains(body, `"resources":[]`) {
		t.Errorf("fleet response must surface resources as empty array; body = %s", body)
	}
	var fleet AgentFleetContext
	if err := json.Unmarshal([]byte(body), &fleet); err != nil {
		t.Fatalf("decode fleet: %v", err)
	}
	if fleet.Resources == nil {
		t.Error("fleet.Resources must never be nil — agents iterate without nil-checks")
	}
	if fleet.GeneratedAt.IsZero() {
		t.Error("fleet.GeneratedAt must be populated so agents can reason about freshness")
	}

	// --- 3. Depth: call the path the manifest declares for the
	// per-resource bundle. The manifest also declares the stable
	// error code "resource_not_found" — exercise the unknown-id path
	// to confirm the error code reaches the wire verbatim. ---
	contextCap := byName["get_resource_context"]
	if contextCap.Path != "/api/agent/resource-context/{resourceId}" {
		t.Fatalf("get_resource_context path: got %q, want /api/agent/resource-context/{resourceId}", contextCap.Path)
	}
	hasNotFoundCode := false
	for _, code := range contextCap.ErrorCodes {
		if code == "resource_not_found" {
			hasNotFoundCode = true
			break
		}
	}
	if !hasNotFoundCode {
		t.Fatalf("get_resource_context manifest must declare resource_not_found error code; got %v", contextCap.ErrorCodes)
	}

	// Substitute a deliberately-unknown canonical id so the handler
	// has to take the not-found branch and write the stable error
	// token. The substrate's contract is that this token reaches the
	// wire intact so agents branch on it rather than parsing human
	// text.
	unknownPath := strings.Replace(contextCap.Path, "{resourceId}", "vm:e2e-unknown-99", 1)
	req = httptest.NewRequest(http.MethodGet, unknownPath, nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("resource-context unknown: status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
	body = rec.Body.String()
	// The error code lives under the canonical `"error"` key (the
	// shared writeJSONError shape used across the API surface), with
	// human-readable text under `"message"`. Pin both: the stable
	// token agents branch on, and the message field's presence so
	// agents can surface human text without losing the code.
	if !strings.Contains(body, `"error":"resource_not_found"`) {
		t.Errorf("resource-context unknown body must carry stable error code resource_not_found under the \"error\" key; body = %s", body)
	}
	var errEnvelope map[string]string
	if err := json.Unmarshal([]byte(body), &errEnvelope); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if errEnvelope["error"] != "resource_not_found" {
		t.Errorf("error envelope code: got %q, want resource_not_found", errEnvelope["error"])
	}
	if strings.TrimSpace(errEnvelope["message"]) == "" {
		t.Error("error envelope must carry a human-readable message alongside the stable code")
	}

	// --- 4. The SSE-stream subscribe_events capability: assert the
	// declared path is wired (a HEAD/GET probe is enough — we don't
	// hold the connection open). The substrate proof here is that
	// discovery's claim about /api/agent/events is honest: the path
	// resolves (auth-gated) rather than 404'ing. ---
	streamCap := byName["subscribe_events"]
	if streamCap.Path != "/api/agent/events" {
		t.Fatalf("subscribe_events path: got %q, want /api/agent/events", streamCap.Path)
	}
	req = httptest.NewRequest(http.MethodGet, streamCap.Path, nil)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	// Without a token, the stream must reject at the auth boundary
	// rather than 404 — the path is registered, just gated.
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("subscribe_events unauth: status = %d, want 401 (path must be registered, just gated); body = %s", rec.Code, rec.Body.String())
	}
}
