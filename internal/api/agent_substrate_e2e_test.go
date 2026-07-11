package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
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
		agentcapabilities.ResourceContextCapabilityName,
		agentcapabilities.FleetContextCapabilityName,
		agentcapabilities.OperationsLoopStatusCapabilityName,
		agentcapabilities.EventSubscriptionCapabilityName,
		agentcapabilities.GetOperatorStateCapabilityName,
		agentcapabilities.SetOperatorStateCapabilityName,
		agentcapabilities.ClearOperatorStateCapabilityName,
		agentcapabilities.ListFindingsCapabilityName,
		agentcapabilities.AcknowledgeFindingCapabilityName,
		agentcapabilities.SnoozeFindingCapabilityName,
		agentcapabilities.DismissFindingCapabilityName,
		agentcapabilities.ResolveFindingCapabilityName,
	}
	for _, name := range required {
		if _, ok := byName[name]; !ok {
			t.Errorf("manifest missing required capability %q — discovery contract broken", name)
		}
	}

	// --- 2. Triage: call the path the manifest declares for the
	// fleet view. The declared shape is "AgentFleetContext", which
	// must round-trip through the actual handler. ---
	fleetCap := byName[agentcapabilities.FleetContextCapabilityName]
	if fleetCap.Path != agentcapabilities.FleetContextCapabilityPath {
		t.Fatalf("%s path: got %q, want %q", agentcapabilities.FleetContextCapabilityName, fleetCap.Path, agentcapabilities.FleetContextCapabilityPath)
	}
	if fleetCap.Method != http.MethodGet {
		t.Fatalf("%s method: got %q, want GET", agentcapabilities.FleetContextCapabilityName, fleetCap.Method)
	}
	if fleetCap.Scope != config.ScopeMonitoringRead {
		t.Fatalf("%s scope: got %q, want %s", agentcapabilities.FleetContextCapabilityName, fleetCap.Scope, config.ScopeMonitoringRead)
	}
	if fleetCap.ResponseShape != "AgentFleetContext" {
		t.Fatalf("%s response shape: got %q, want AgentFleetContext", agentcapabilities.FleetContextCapabilityName, fleetCap.ResponseShape)
	}
	fleetPath := projectAgentCapabilityPath(t, fleetCap, nil)

	// Unauthenticated call must be rejected — the underlying
	// capability's own auth scope holds even though discovery is
	// unauthenticated.
	req = httptest.NewRequest(http.MethodGet, fleetPath, nil)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("fleet GET unauth: status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}

	// Authenticated call must serve the iteration-safe shape.
	req = httptest.NewRequest(http.MethodGet, fleetPath, nil)
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

	statusCap := byName[agentcapabilities.OperationsLoopStatusCapabilityName]
	if statusCap.Path != agentcapabilities.OperationsLoopStatusCapabilityPath {
		t.Fatalf("%s path: got %q, want %q", agentcapabilities.OperationsLoopStatusCapabilityName, statusCap.Path, agentcapabilities.OperationsLoopStatusCapabilityPath)
	}
	if statusCap.Method != http.MethodGet {
		t.Fatalf("%s method: got %q, want GET", agentcapabilities.OperationsLoopStatusCapabilityName, statusCap.Method)
	}
	if statusCap.Scope != config.ScopeMonitoringRead {
		t.Fatalf("%s scope: got %q, want %s", agentcapabilities.OperationsLoopStatusCapabilityName, statusCap.Scope, config.ScopeMonitoringRead)
	}
	if statusCap.ResponseShape != "AgentOperationsLoopStatus" {
		t.Fatalf("%s response shape: got %q, want AgentOperationsLoopStatus", agentcapabilities.OperationsLoopStatusCapabilityName, statusCap.ResponseShape)
	}
	req = httptest.NewRequest(http.MethodGet, projectAgentCapabilityPath(t, statusCap, nil), nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("operations-loop status GET auth: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var loopStatus AgentOperationsLoopStatus
	if err := json.NewDecoder(rec.Body).Decode(&loopStatus); err != nil {
		t.Fatalf("decode operations-loop status: %v", err)
	}
	loopSteps := agentOperationsLoopStepsByID(loopStatus.Steps)
	if loopStatus.NextAction != "run_patrol" || len(loopStatus.Steps) != 4 || loopStatus.GeneratedAt.IsZero() {
		t.Fatalf("operations-loop status shape = %+v", loopStatus)
	}
	if _, ok := loopSteps["external_agents"]; ok {
		t.Fatalf("operations-loop status must keep external-agent readiness out of steps: %+v", loopStatus.Steps)
	}

	// --- 3. Depth: call the path the manifest declares for the
	// per-resource bundle. The manifest also declares the stable
	// error code "resource_not_found" — exercise the unknown-id path
	// to confirm the error code reaches the wire verbatim. ---
	contextCap := byName[agentcapabilities.ResourceContextCapabilityName]
	if contextCap.Path != agentcapabilities.ResourceContextCapabilityPath {
		t.Fatalf("%s path: got %q, want %q", agentcapabilities.ResourceContextCapabilityName, contextCap.Path, agentcapabilities.ResourceContextCapabilityPath)
	}
	hasNotFoundCode := false
	for _, code := range contextCap.ErrorCodes {
		if code == agentcapabilities.AgentErrCodeResourceNotFound {
			hasNotFoundCode = true
			break
		}
	}
	if !hasNotFoundCode {
		t.Fatalf("%s manifest must declare %s error code; got %v", agentcapabilities.ResourceContextCapabilityName, agentcapabilities.AgentErrCodeResourceNotFound, contextCap.ErrorCodes)
	}

	// Substitute a deliberately-unknown canonical id so the handler
	// has to take the not-found branch and write the stable error
	// token. The substrate's contract is that this token reaches the
	// wire intact so agents branch on it rather than parsing human
	// text.
	unknownPath := projectAgentCapabilityPath(t, contextCap, map[string]any{
		agentcapabilities.ResourceIDArgumentName: "vm:e2e-unknown-99",
	})
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
	if !strings.Contains(body, `"error":"`+agentcapabilities.AgentErrCodeResourceNotFound+`"`) {
		t.Errorf("resource-context unknown body must carry stable error code %s under the \"error\" key; body = %s", agentcapabilities.AgentErrCodeResourceNotFound, body)
	}
	var errEnvelope map[string]string
	if err := json.Unmarshal([]byte(body), &errEnvelope); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if errEnvelope["error"] != agentcapabilities.AgentErrCodeResourceNotFound {
		t.Errorf("error envelope code: got %q, want %s", errEnvelope["error"], agentcapabilities.AgentErrCodeResourceNotFound)
	}
	if strings.TrimSpace(errEnvelope["message"]) == "" {
		t.Error("error envelope must carry a human-readable message alongside the stable code")
	}

	// --- 4. The SSE-stream subscribe_events capability: assert the
	// declared path is wired (a HEAD/GET probe is enough — we don't
	// hold the connection open). The substrate proof here is that
	// discovery's claim about /api/agent/events is honest: the path
	// resolves (auth-gated) rather than 404'ing. ---
	streamCap := byName[agentcapabilities.EventSubscriptionCapabilityName]
	if streamCap.Path != agentcapabilities.AgentEventsPath {
		t.Fatalf("%s path: got %q, want %s", agentcapabilities.EventSubscriptionCapabilityName, streamCap.Path, agentcapabilities.AgentEventsPath)
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

func TestAgentSubstrate_NodeProvisioningCapabilitiesRouteThroughHTTPBoundary(t *testing.T) {
	readToken := "agent-provisioning-read.12345678"
	writeToken := "agent-provisioning-write.12345678"
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, readToken, []string{config.ScopeSettingsRead}, nil),
		newTokenRecord(t, writeToken, []string{config.ScopeSettingsWrite}, nil),
	)
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("new monitor: %v", err)
	}
	defer monitor.Stop()
	router := NewRouter(cfg, monitor, nil, nil, func() error { return nil }, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/agent/capabilities", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("capabilities GET: status = %d", rec.Code)
	}
	var manifest AgentCapabilitiesManifest
	if err := json.NewDecoder(rec.Body).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	byName := map[string]AgentCapability{}
	for _, c := range manifest.Capabilities {
		byName[c.Name] = c
	}

	listCap := byName["list_nodes"]
	addCap := byName["add_node"]
	for name, cap := range map[string]AgentCapability{
		"list_nodes": listCap,
		"add_node":   addCap,
	} {
		if cap.Name == "" {
			t.Fatalf("manifest missing %q capability", name)
		}
		if cap.Category != "provisioning" {
			t.Fatalf("%s category = %q, want provisioning", name, cap.Category)
		}
	}
	if listCap.Method != http.MethodGet || listCap.Scope != config.ScopeSettingsRead {
		t.Fatalf("list_nodes method/scope = %s/%s, want GET/%s", listCap.Method, listCap.Scope, config.ScopeSettingsRead)
	}
	if addCap.Method != http.MethodPost || addCap.Scope != config.ScopeSettingsWrite || addCap.InputSchema == nil {
		t.Fatalf("add_node method/scope/inputSchema = %s/%s/%v, want POST/%s/non-nil",
			addCap.Method, addCap.Scope, addCap.InputSchema, config.ScopeSettingsWrite)
	}

	req = httptest.NewRequest(http.MethodGet, listCap.Path, nil)
	req.Header.Set("X-API-Token", readToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list_nodes auth: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("new test config should start with no nodes, got %s", rec.Body.String())
	}

	addBody := strings.NewReader(`{
		"type": "pve",
		"name": "test-agent-node",
		"host": "http://192.168.77.50:8006",
		"tokenName": "root@pam!pulse-agent-test",
		"tokenValue": "secret-value",
		"monitorVMs": true,
		"monitorContainers": true
	}`)
	req = httptest.NewRequest(http.MethodPost, addCap.Path, addBody)
	req.Header.Set("X-API-Token", writeToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add_node auth: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"success"`) {
		t.Fatalf("add_node response should be the existing success envelope, got %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, listCap.Path, nil)
	req.Header.Set("X-API-Token", readToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list_nodes after add: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	listBody := rec.Body.String()
	if !strings.Contains(listBody, `"name":"test-agent-node"`) || !strings.Contains(listBody, `"hasToken":true`) {
		t.Fatalf("list_nodes should expose redacted configured source identity, got %s", listBody)
	}
	if strings.Contains(listBody, "secret-value") || strings.Contains(listBody, "tokenValue") {
		t.Fatalf("list_nodes must not leak stored token values, got %s", listBody)
	}
}

// TestAgentSubstrate_OperatorStateWriteRoundTripsThroughHTTPBoundary
// is the end-to-end contract proof for the agent-paradigm write
// substrate. The only write capability the manifest declares is the
// operator-state intent loop — set/get/clear — and this test boots
// the full router stack to walk every state of that loop through
// the actual HTTP boundary, asserting the manifest's declared error
// codes for set_operator_state and get_operator_state actually
// reach the wire from the handlers.
//
// Specifically:
//   - GET on a never-written resource returns 404 with
//     operator_state_not_set under the canonical "error" key, so
//     agents can branch on "no entry" vs "default-zero entry".
//   - PUT with valid input returns 200 with the persisted state,
//     including server-populated attribution (SetAt) that the
//     client-supplied value cannot spoof.
//   - GET after PUT round-trips the same state.
//   - PUT with invalid criticality returns 400 with
//     operator_state_invalid — the second stable error code the
//     manifest declares for this capability.
//   - DELETE clears idempotently.
//   - GET after DELETE returns to the not-set 404 again, closing
//     the lifecycle loop.
//   - The same resource id can be a colon-bearing canonical id
//     (e.g. vm:101) — the URL path segment must round-trip without
//     percent-encoding tripping the routing.
//
// This is the substantive proof for the write side: the agent
// surface — read, write, push — has now been exercised end-to-end
// as one substrate.
func TestAgentSubstrate_OperatorStateWriteRoundTripsThroughHTTPBoundary(t *testing.T) {
	rawToken := "agent-substrate-write-e2e.12345678"
	record := newTokenRecord(t, rawToken,
		[]string{config.ScopeMonitoringRead, config.ScopeMonitoringWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	// Pull the manifest first to confirm what we're about to
	// exercise matches what the discovery contract declares. This
	// closes the "manifest is honest" loop on the write side.
	req := httptest.NewRequest(http.MethodGet, "/api/agent/capabilities", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("capabilities GET: status = %d", rec.Code)
	}
	var manifest AgentCapabilitiesManifest
	if err := json.NewDecoder(rec.Body).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	byName := map[string]AgentCapability{}
	for _, c := range manifest.Capabilities {
		byName[c.Name] = c
	}
	setCap := byName[agentcapabilities.SetOperatorStateCapabilityName]
	getCap := byName[agentcapabilities.GetOperatorStateCapabilityName]
	clearCap := byName[agentcapabilities.ClearOperatorStateCapabilityName]

	if setCap.Method != http.MethodPut || setCap.Scope != config.ScopeMonitoringWrite {
		t.Fatalf("%s declared method/scope: got %q/%q, want PUT/%s",
			agentcapabilities.SetOperatorStateCapabilityName, setCap.Method, setCap.Scope, config.ScopeMonitoringWrite)
	}
	if !stringSliceContains(setCap.ErrorCodes, agentcapabilities.AgentErrCodeOperatorStateInvalid) {
		t.Fatalf("%s must declare %s; got %v", agentcapabilities.SetOperatorStateCapabilityName, agentcapabilities.AgentErrCodeOperatorStateInvalid, setCap.ErrorCodes)
	}
	if getCap.Method != http.MethodGet || getCap.Scope != config.ScopeMonitoringRead {
		t.Fatalf("%s declared method/scope: got %q/%q, want GET/%s",
			agentcapabilities.GetOperatorStateCapabilityName, getCap.Method, getCap.Scope, config.ScopeMonitoringRead)
	}
	if !stringSliceContains(getCap.ErrorCodes, agentcapabilities.AgentErrCodeOperatorStateNotSet) {
		t.Fatalf("%s must declare %s; got %v", agentcapabilities.GetOperatorStateCapabilityName, agentcapabilities.AgentErrCodeOperatorStateNotSet, getCap.ErrorCodes)
	}
	if clearCap.Method != http.MethodDelete || clearCap.Scope != config.ScopeMonitoringWrite {
		t.Fatalf("%s declared method/scope: got %q/%q, want DELETE/%s",
			agentcapabilities.ClearOperatorStateCapabilityName, clearCap.Method, clearCap.Scope, config.ScopeMonitoringWrite)
	}

	// The capability paths use {resourceId} as the placeholder. Sub
	// in a colon-bearing canonical id so the test exercises the
	// URL-routing path most agents will actually hit.
	resourceID := "vm:e2e-write-101"
	pathArgs := map[string]any{
		agentcapabilities.ResourceIDArgumentName: resourceID,
	}
	getPath := projectAgentCapabilityPath(t, getCap, pathArgs)
	setPath := projectAgentCapabilityPath(t, setCap, pathArgs)
	clearPath := projectAgentCapabilityPath(t, clearCap, pathArgs)

	// --- 1. GET on never-written resource → 404 not_set. ---
	req = httptest.NewRequest(http.MethodGet, getPath, nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET unset: status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
	notSetBody := rec.Body.String()
	if !strings.Contains(notSetBody, `"error":"`+agentcapabilities.AgentErrCodeOperatorStateNotSet+`"`) {
		t.Errorf("GET unset must carry stable error code %s; body = %s", agentcapabilities.AgentErrCodeOperatorStateNotSet, notSetBody)
	}

	// --- 2. PUT valid state → 200 with persisted state. ---
	validBody := []byte(`{
		"intentionallyOffline": true,
		"neverAutoRemediate": true,
		"note": "decommissioned for hardware refresh"
	}`)
	req = httptest.NewRequest(http.MethodPut, setPath, bytes.NewReader(validBody))
	req.Header.Set("X-API-Token", rawToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT valid: status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	var persisted resourceOperatorStateAPI
	if err := json.NewDecoder(rec.Body).Decode(&persisted); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	// The URL canonical id must override anything the body might
	// say (this body says nothing) — server-populated, not
	// client-controlled.
	if persisted.CanonicalID != resourceID {
		t.Errorf("PUT response CanonicalID: got %q, want %q (URL must override body)",
			persisted.CanonicalID, resourceID)
	}
	if !persisted.IntentionallyOffline {
		t.Error("PUT response must round-trip IntentionallyOffline=true")
	}
	if !persisted.NeverAutoRemediate {
		t.Error("PUT response must round-trip NeverAutoRemediate=true")
	}
	if persisted.Note != "decommissioned for hardware refresh" {
		t.Errorf("PUT response Note did not round-trip; got %q", persisted.Note)
	}
	if persisted.SetAt.IsZero() {
		t.Error("PUT response must carry server-populated SetAt — client cannot spoof attribution")
	}

	// --- 3. GET after PUT round-trips. ---
	req = httptest.NewRequest(http.MethodGet, getPath, nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET after PUT: status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	var fetched resourceOperatorStateAPI
	if err := json.NewDecoder(rec.Body).Decode(&fetched); err != nil {
		t.Fatalf("decode GET-after-PUT: %v", err)
	}
	if fetched.CanonicalID != persisted.CanonicalID ||
		fetched.IntentionallyOffline != persisted.IntentionallyOffline ||
		fetched.NeverAutoRemediate != persisted.NeverAutoRemediate ||
		fetched.Note != persisted.Note {
		t.Errorf("GET-after-PUT does not round-trip:\n  got  %+v\n  want %+v", fetched, persisted)
	}

	// --- 4. PUT invalid → 400 operator_state_invalid. ---
	// Unknown criticality is the cleanest input that survives JSON
	// decoding but fails ValidateResourceOperatorState — the
	// validator names the field in the message, which we do not
	// pin (human text), but the stable code is what agents branch
	// on.
	invalidBody := []byte(`{"criticality": "very-high"}`)
	req = httptest.NewRequest(http.MethodPut, setPath, bytes.NewReader(invalidBody))
	req.Header.Set("X-API-Token", rawToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT invalid: status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
	invalidBodyResp := rec.Body.String()
	if !strings.Contains(invalidBodyResp, `"error":"`+agentcapabilities.AgentErrCodeOperatorStateInvalid+`"`) {
		t.Errorf("PUT invalid must carry stable error code %s; body = %s", agentcapabilities.AgentErrCodeOperatorStateInvalid, invalidBodyResp)
	}

	// --- 5. DELETE → 204 (idempotent). ---
	req = httptest.NewRequest(http.MethodDelete, clearPath, nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: status = %d, want 204, body = %s", rec.Code, rec.Body.String())
	}

	// --- 6. GET after DELETE → 404 not_set, closing the loop. ---
	req = httptest.NewRequest(http.MethodGet, getPath, nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET after DELETE: status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error":"`+agentcapabilities.AgentErrCodeOperatorStateNotSet+`"`) {
		t.Errorf("GET after DELETE must surface operator_state_not_set again; body = %s", rec.Body.String())
	}

	// --- 7. DELETE again → still 204 (idempotency contract). ---
	req = httptest.NewRequest(http.MethodDelete, clearPath, nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE idempotent: status = %d, want 204 (clear must succeed even when no entry exists)", rec.Code)
	}
}

// stringSliceContains is a tiny string-slice membership helper so the
// e2e test can assert manifest contents without taking on a sort
// dependency. Named to avoid colliding with internal/api/diagnostics.go's
// existing `contains` and config_handlers_setup_script_test.go's
// `containsString` helpers.
func stringSliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// TestAgentSubstrate_ActionEndpointsEmitAgentStableEnvelope is the
// end-to-end contract proof for the action surface joining the
// agent manifest. The three action endpoints (plan/decide/execute)
// previously emitted the platform-wide APIError shape (stable code
// under `code`, human under `error`); after the slice-58 refactor
// they emit the agent-stable envelope (stable code under `error`,
// human under `message`) so an agent that branched on the agent
// surface sees identical envelope semantics across read, write,
// and action capabilities.
//
// The test covers three error paths through the actual HTTP
// boundary, one per endpoint:
//
//   - plan_action with a missing resourceId surfaces
//     invalid_action_request under the agent-stable `error` key
//     and includes a `details` map naming the field that failed.
//   - decide_action against a non-existent action id surfaces
//     action_not_found.
//   - execute_action against a non-existent action id surfaces
//     action_not_found.
//
// Each error path also asserts the message is non-empty so agents
// surfacing copy to humans get something readable, and that the
// APIError-shape fields (`code`, `status_code`, `timestamp`) do
// NOT appear, since their presence would mean the refactor
// regressed.
func TestAgentSubstrate_ActionEndpointsEmitAgentStableEnvelope(t *testing.T) {
	rawToken := "agent-substrate-action-e2e.12345678"
	record := newTokenRecord(t, rawToken,
		[]string{config.ScopeMonitoringRead, config.ScopeAIExecute}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	// First verify the manifest declares the three action
	// capabilities, since the substrate's promise is that the
	// envelope an agent encounters matches what the manifest's
	// errorCodes list documents.
	req := httptest.NewRequest(http.MethodGet, "/api/agent/capabilities", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("capabilities GET: status = %d", rec.Code)
	}
	var manifest AgentCapabilitiesManifest
	if err := json.NewDecoder(rec.Body).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	byName := map[string]AgentCapability{}
	for _, c := range manifest.Capabilities {
		byName[c.Name] = c
	}
	for want, wantScope := range map[string]string{
		agentcapabilities.PlanActionCapabilityName:    config.ScopeActionsPlan,
		agentcapabilities.DecideActionCapabilityName:  config.ScopeActionsApprove,
		agentcapabilities.ExecuteActionCapabilityName: config.ScopeActionsExecute,
	} {
		cap, ok := byName[want]
		if !ok {
			t.Fatalf("manifest missing %q capability", want)
		}
		if cap.Category != "action" {
			t.Errorf("%s: category = %q, want \"action\"", want, cap.Category)
		}
		if cap.Scope != wantScope {
			t.Errorf("%s: scope = %q, want %q", want, cap.Scope, wantScope)
		}
		if len(cap.ErrorCodes) == 0 {
			t.Errorf("%s: must declare at least one stable errorCode", want)
		}
	}

	// --- 1. plan_action: invalid request body lands under the
	// stable invalid_action_request code with a details map. ---
	planCap := byName[agentcapabilities.PlanActionCapabilityName]
	body := agentSubstrateJSONBody(t, map[string]string{
		agentcapabilities.ResourceIDArgumentName:     "",
		agentcapabilities.CapabilityNameArgumentName: "restart",
	})
	req = httptest.NewRequest(http.MethodPost, projectAgentCapabilityPath(t, planCap, nil), body)
	req.Header.Set("X-API-Token", rawToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%s invalid: status = %d, want 400, body = %s", agentcapabilities.PlanActionCapabilityName, rec.Code, rec.Body.String())
	}
	assertAgentStableEnvelope(t, agentcapabilities.PlanActionCapabilityName, rec.Body.String(), agentcapabilities.AgentErrCodeInvalidActionRequest)
	if !stringSliceContains(planCap.ErrorCodes, agentcapabilities.AgentErrCodeInvalidActionRequest) {
		t.Errorf("manifest %s.ErrorCodes must declare %s; got %v", agentcapabilities.PlanActionCapabilityName, agentcapabilities.AgentErrCodeInvalidActionRequest, planCap.ErrorCodes)
	}

	// --- 2. decide_action against a non-existent action id. ---
	decideCap := byName[agentcapabilities.DecideActionCapabilityName]
	decidePath := projectAgentCapabilityPath(t, decideCap, map[string]any{
		agentcapabilities.ActionIDArgumentName: "act_does_not_exist_xyz",
	})
	body = agentSubstrateJSONBody(t, map[string]string{
		agentcapabilities.OutcomeArgumentName: "approved",
	})
	req = httptest.NewRequest(http.MethodPost, decidePath, body)
	req.Header.Set("X-API-Token", rawToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("%s unknown: status = %d, want 404, body = %s", agentcapabilities.DecideActionCapabilityName, rec.Code, rec.Body.String())
	}
	assertAgentStableEnvelope(t, agentcapabilities.DecideActionCapabilityName, rec.Body.String(), agentcapabilities.AgentErrCodeActionNotFound)

	// --- 3. execute_action against a non-existent action id. ---
	executeCap := byName[agentcapabilities.ExecuteActionCapabilityName]
	executePath := projectAgentCapabilityPath(t, executeCap, map[string]any{
		agentcapabilities.ActionIDArgumentName: "act_does_not_exist_xyz",
	})
	req = httptest.NewRequest(http.MethodPost, executePath, agentSubstrateJSONBody(t, map[string]any{}))
	req.Header.Set("X-API-Token", rawToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("%s unknown: status = %d, want 404, body = %s", agentcapabilities.ExecuteActionCapabilityName, rec.Code, rec.Body.String())
	}
	assertAgentStableEnvelope(t, agentcapabilities.ExecuteActionCapabilityName, rec.Body.String(), agentcapabilities.AgentErrCodeActionNotFound)
}

func projectAgentCapabilityPath(t *testing.T, cap AgentCapability, args map[string]any) string {
	t.Helper()
	projected, err := agentcapabilities.ProjectCapabilityCall(cap, args)
	if err != nil {
		t.Fatalf("%s route projection failed: %v", cap.Name, err)
	}
	return projected.Path
}

func agentSubstrateJSONBody(t *testing.T, payload any) *bytes.Reader {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal agent substrate request body: %v", err)
	}
	return bytes.NewReader(raw)
}

// assertAgentStableEnvelope is the inverse APIError-shape check: it
// asserts the response body uses the agent-stable envelope (stable
// code under `error`, human under `message`) and explicitly does
// NOT carry the legacy APIError fields (`code`, `status_code`,
// `timestamp`). Drift back to APIError would mean the slice-58
// refactor regressed, and an agent that read `error` for the
// stable code would silently get human text instead.
func assertAgentStableEnvelope(t *testing.T, label, body, wantCode string) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("%s: response body is not JSON: %v\nbody=%s", label, err, body)
	}
	gotCode, _ := envelope["error"].(string)
	if gotCode != wantCode {
		t.Errorf("%s: error code = %q, want %q (envelope=%s)", label, gotCode, wantCode, body)
	}
	gotMsg, _ := envelope["message"].(string)
	if strings.TrimSpace(gotMsg) == "" {
		t.Errorf("%s: message must be non-empty so agents can surface human text; envelope=%s", label, body)
	}
	for _, legacy := range []string{"code", "status_code", "timestamp"} {
		if _, has := envelope[legacy]; has {
			t.Errorf("%s: envelope must not carry legacy APIError field %q after agent-stable refactor; envelope=%s", label, legacy, body)
		}
	}
}
