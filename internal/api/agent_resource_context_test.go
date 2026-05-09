package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// staticFindingsProvider is a tiny test double for AgentFindingsProvider
// — keyed by resource id so each test can stage the snapshot it
// expects to see in the bundle.
type staticFindingsProvider struct {
	byResource map[string][]AgentResourceFindingSnapshot
}

func (s staticFindingsProvider) ActiveFindingsForResource(resourceID string) []AgentResourceFindingSnapshot {
	if s.byResource == nil {
		return nil
	}
	return s.byResource[resourceID]
}

// staticApprovalsProvider is a tiny test double for
// AgentApprovalsProvider — keyed by (resource id, org id) so each
// test can stage the pending approvals it expects to see in the
// bundle without setting up a global approval store.
type staticApprovalsProvider struct {
	byResource map[string][]AgentResourceApprovalSummary
}

func (s staticApprovalsProvider) PendingApprovalsForResource(resourceID, _ string) []AgentResourceApprovalSummary {
	if s.byResource == nil {
		return nil
	}
	return s.byResource[resourceID]
}

// agentContextFixtureHandlers wires a ResourceHandlers around a
// pre-built unified-resource seed (using the same
// `resourceUnifiedSeedProvider` pattern from resources_test.go) plus
// the agent context handler with a findings-provider stub. The seed
// provider bypasses the snapshot → unified adapter, letting us stage
// exactly the resource the test expects to see in the bundle.
func agentContextFixtureHandlers(t *testing.T, resourceID string) (*AgentContextHandler, *staticFindingsProvider) {
	t.Helper()
	cfg := &config.Config{DataPath: t.TempDir()}
	resources := NewResourceHandlers(cfg)
	resources.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot: models.StateSnapshot{LastUpdate: time.Now()},
		resources: []unified.Resource{
			{
				ID:   resourceID,
				Type: "vm",
				Name: "db-01",
			},
		},
	})
	provider := &staticFindingsProvider{byResource: map[string][]AgentResourceFindingSnapshot{}}
	h := NewAgentContextHandler(resources)
	h.SetFindingsProvider(provider)
	return h, provider
}

func TestHandleAgentResourceContext_BundlesIdentityOperatorStateAndFindings(t *testing.T) {
	h, findings := agentContextFixtureHandlers(t, "vm:101")

	// Stage operator state — server should compute
	// MaintenanceWindowActive once and surface it on the wire so agents
	// don't re-evaluate timestamps client-side.
	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:101",
		NeverAutoRemediate: true,
		MaintenanceStartAt: &start,
		MaintenanceEndAt:   &end,
		MaintenanceReason:  "Q3 storage upgrade",
		Criticality:        unified.CriticalityHigh,
		SetAt:              time.Now().UTC(),
		SetBy:              "operator:richard",
	}); err != nil {
		t.Fatalf("SetResourceOperatorState: %v", err)
	}

	// Stage two active findings the provider would return.
	findings.byResource["vm:101"] = []AgentResourceFindingSnapshot{
		{
			ID:              "f-cpu",
			Title:           "CPU saturated",
			Severity:        "warning",
			Category:        "performance",
			Impact:          "Workload stalls until pressure clears.",
			Recommendation:  "Restart workload service.",
			RegressionCount: 2,
			Confidence:      "medium",
		},
		{
			ID:       "f-disk",
			Title:    "Disk pressure",
			Severity: "critical",
		},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:101", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "default"))
	h.HandleResourceContext(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var bundle AgentResourceContext
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}

	// Identity round-trips.
	if bundle.CanonicalID != "vm:101" {
		t.Errorf("canonical id: got %q want vm:101", bundle.CanonicalID)
	}
	if bundle.ResourceName != "db-01" {
		t.Errorf("resource name: got %q want db-01", bundle.ResourceName)
	}
	if bundle.ResourceType != "vm" {
		t.Errorf("resource type: got %q want vm", bundle.ResourceType)
	}

	// Operator-state projected with computed maintenance-active flag.
	if bundle.OperatorState == nil {
		t.Fatal("operator state must be present")
	}
	if !bundle.OperatorState.NeverAutoRemediate {
		t.Error("never_auto_remediate must round-trip")
	}
	if !bundle.OperatorState.MaintenanceWindowActive {
		t.Error("MaintenanceWindowActive must be true when window covers now (computed server-side, not by the agent)")
	}
	if bundle.OperatorState.Criticality != "high" {
		t.Errorf("criticality: got %q want high", bundle.OperatorState.Criticality)
	}

	// Findings come through the provider verbatim — no leakage of
	// internal Finding shape.
	if len(bundle.ActiveFindings) != 2 {
		t.Fatalf("expected 2 findings; got %d", len(bundle.ActiveFindings))
	}
	if bundle.ActiveFindings[0].Confidence != "medium" {
		t.Errorf("confidence must round-trip; got %q", bundle.ActiveFindings[0].Confidence)
	}
	if bundle.ActiveFindings[0].RegressionCount != 2 {
		t.Errorf("regression count must round-trip; got %d", bundle.ActiveFindings[0].RegressionCount)
	}

	// Generated timestamp populated so agents can age the data.
	if bundle.GeneratedAt.IsZero() {
		t.Error("GeneratedAt must be populated")
	}
}

func TestHandleAgentResourceContext_OperatorStateAbsentWhenNoEntry(t *testing.T) {
	// A resource with no operator state recorded must omit the
	// operatorState field entirely (omitempty), distinguishing "no
	// operator overrides" from "operator overrides happen to be all
	// zero." Agents branch on field presence rather than value.
	h, _ := agentContextFixtureHandlers(t, "vm:fresh")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:fresh", nil)
	h.HandleResourceContext(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d body=%s", rec.Code, rec.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, has := raw["operatorState"]; has {
		t.Errorf("operatorState must be omitted when no entry exists; got %v", raw["operatorState"])
	}
	// Active findings must be an empty array (not null), so agents can
	// iterate without nil-checking the field.
	if findings, ok := raw["activeFindings"].([]any); !ok {
		t.Errorf("activeFindings must be an array; got %T", raw["activeFindings"])
	} else if len(findings) != 0 {
		t.Errorf("expected empty findings array; got %d", len(findings))
	}
}

func TestHandleAgentResourceContext_404OnUnknownResource(t *testing.T) {
	h, _ := agentContextFixtureHandlers(t, "vm:101")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:nonexistent", nil)
	h.HandleResourceContext(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404; got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	if body["error"] != "resource_not_found" {
		t.Errorf("expected error=resource_not_found; got %v", body)
	}
}

func TestHandleAgentResourceContext_RejectsNonGet(t *testing.T) {
	h, _ := agentContextFixtureHandlers(t, "vm:101")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agent/resource-context/vm:101", nil)
	h.HandleResourceContext(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405; got %d", rec.Code)
	}
}

func TestHandleAgentResourceContext_RecentActionsCarryRefusalTokens(t *testing.T) {
	// Refused dispatches (resource_remediation_locked:, plan_drift:)
	// must surface verbatim in the recent-actions slice so agents can
	// branch on the stable token without parsing human messages. Pin
	// this to keep the agent-paradigm contract honest — Pulse already
	// records these refusals as Failed audit records (slices 17, 33);
	// this test verifies the bundle exposes them.
	h, _ := agentContextFixtureHandlers(t, "vm:locked")

	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	now := time.Now().UTC()
	plan := unified.ActionPlan{
		ActionID:  "act-refused",
		RequestID: "req-refused",
		PlannedAt: now,
		ExpiresAt: now.Add(5 * time.Minute),
	}
	if err := store.RecordActionAudit(unified.ActionAuditRecord{
		ID:        "act-refused",
		CreatedAt: now,
		UpdatedAt: now,
		State:     unified.ActionStateFailed,
		Request: unified.ActionRequest{
			RequestID:      "req-refused",
			ResourceID:     "vm:locked",
			CapabilityName: "pulse_control",
			Reason:         "restart workload",
			RequestedBy:    "pulse_patrol",
			Params:         map[string]any{"command": "systemctl restart workload"},
		},
		Plan: plan,
		Result: &unified.ExecutionResult{
			Success:      false,
			ErrorMessage: "resource_remediation_locked: resource is operator-locked against automated remediation",
		},
	}); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:locked", nil)
	h.HandleResourceContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d", rec.Code)
	}
	var bundle AgentResourceContext
	if err := json.Unmarshal(rec.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("unmarshal bundle: %v", err)
	}
	if len(bundle.RecentActions) != 1 {
		t.Fatalf("expected 1 recent action; got %d", len(bundle.RecentActions))
	}
	action := bundle.RecentActions[0]
	if action.State != string(unified.ActionStateFailed) {
		t.Errorf("state: got %q want failed", action.State)
	}
	if action.Success {
		t.Error("Success must be false on refused dispatch")
	}
	if action.ErrorMessage == "" || action.Command == "" {
		t.Errorf("error message and command must round-trip; got %+v", action)
	}
	// Stable token preservation — agents branch on the prefix.
	if !strings.Contains(action.ErrorMessage, "resource_remediation_locked:") {
		t.Errorf("ErrorMessage must preserve the stable refusal token; got %q", action.ErrorMessage)
	}
}

func TestHandleAgentResourceContext_PendingApprovalsAreScopedToResource(t *testing.T) {
	// pendingApprovals must list every still-pending approval
	// targeting the bundle's resource. The provider is the seam that
	// scopes by (resource id, org id) — the bundle handler trusts
	// the provider's filtering. Pin the contract: if the provider
	// returns N entries, the bundle returns N entries with the same
	// agent-stable shape (id, command, riskLevel, requestedBy,
	// requestedAt, expiresAt) and an empty array (never null) when
	// the provider returns nothing.
	h, _ := agentContextFixtureHandlers(t, "vm:101")
	now := time.Now().UTC()
	expires := now.Add(5 * time.Minute)
	h.SetApprovalsProvider(staticApprovalsProvider{
		byResource: map[string][]AgentResourceApprovalSummary{
			"vm:101": {
				{
					ID:          "appr-1",
					Command:     "systemctl restart nginx",
					RiskLevel:   "medium",
					RequestedBy: "ai:patrol",
					RequestedAt: now,
					ExpiresAt:   expires,
				},
			},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:101", nil)
	h.HandleResourceContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var bundle AgentResourceContext
	if err := json.NewDecoder(rec.Body).Decode(&bundle); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(bundle.PendingApprovals) != 1 {
		t.Fatalf("PendingApprovals len = %d; want 1; body=%s", len(bundle.PendingApprovals), rec.Body.String())
	}
	got := bundle.PendingApprovals[0]
	if got.ID != "appr-1" {
		t.Errorf("ID: got %q, want %q", got.ID, "appr-1")
	}
	if got.Command != "systemctl restart nginx" {
		t.Errorf("Command did not round-trip: got %q", got.Command)
	}
	if got.RiskLevel != "medium" {
		t.Errorf("RiskLevel did not round-trip: got %q", got.RiskLevel)
	}
	if !got.ExpiresAt.Equal(expires) {
		t.Errorf("ExpiresAt did not round-trip: got %v, want %v", got.ExpiresAt, expires)
	}
}

func TestHandleAgentResourceContext_PendingApprovalsEmptyArrayWhenNone(t *testing.T) {
	// Absent or empty must surface as an empty array, not as a
	// missing field — agents iterate without nil-checking. This
	// mirrors the contract for ActiveFindings and RecentActions.
	h, _ := agentContextFixtureHandlers(t, "vm:404")
	// No approvals provider wired — same observable shape as a
	// provider that returns nil for this resource.

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:404", nil)
	h.HandleResourceContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"pendingApprovals":[]`) {
		t.Errorf("expected pendingApprovals to surface as empty array; body=%s", body)
	}
}

// fleetFixtureHandlers seeds the registry with a multi-resource set
// so the fleet sweep has something to walk. Mirrors the per-resource
// fixture but with N resources and an explicit findings/approvals
// stub for each.
func fleetFixtureHandlers(t *testing.T, resources []unified.Resource) (*AgentContextHandler, *staticFindingsProvider, *staticApprovalsProvider) {
	t.Helper()
	cfg := &config.Config{DataPath: t.TempDir()}
	rh := NewResourceHandlers(cfg)
	rh.SetStateProvider(resourceUnifiedSeedProvider{
		snapshot:  models.StateSnapshot{LastUpdate: time.Now()},
		resources: resources,
	})
	findings := &staticFindingsProvider{byResource: map[string][]AgentResourceFindingSnapshot{}}
	approvals := &staticApprovalsProvider{byResource: map[string][]AgentResourceApprovalSummary{}}
	h := NewAgentContextHandler(rh)
	h.SetFindingsProvider(findings)
	h.SetApprovalsProvider(approvals)
	return h, findings, approvals
}

func TestHandleAgentFleetContext_RollsUpEveryResource(t *testing.T) {
	// The fleet view must surface one entry per resource in the
	// registry so an agent can scan the whole org in one read. Pin
	// the contract: identity round-trips, count is 1:1 with the
	// registry, list is always an array (never null).
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
		{ID: "container:web-1", Type: "container", Name: "web-1", Technology: "docker"},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	h.HandleFleetContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var fleet AgentFleetContext
	if err := json.NewDecoder(rec.Body).Decode(&fleet); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(fleet.Resources) != 2 {
		t.Fatalf("Resources len = %d; want 2; body=%s", len(fleet.Resources), rec.Body.String())
	}
	byID := map[string]AgentFleetResourceSummary{}
	for _, s := range fleet.Resources {
		byID[s.CanonicalID] = s
	}
	if got, ok := byID["vm:101"]; !ok || got.ResourceName != "db-01" || got.ResourceType != "vm" {
		t.Errorf("vm:101 entry missing or wrong: %+v", got)
	}
	if got, ok := byID["container:web-1"]; !ok || got.Technology != "docker" {
		t.Errorf("container:web-1 entry missing or wrong technology: %+v", got)
	}
}

func TestHandleAgentFleetContext_CountsFindingsBySeverity(t *testing.T) {
	// Per-severity finding counts are the headline number agents use
	// to triage. Pin the contract: counts roll up correctly across
	// (critical, warning, info) and total is the sum.
	h, findings, _ := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
	})
	findings.byResource["vm:101"] = []AgentResourceFindingSnapshot{
		{ID: "f1", Severity: "critical"},
		{ID: "f2", Severity: "warning"},
		{ID: "f3", Severity: "warning"},
		{ID: "f4", Severity: "info"},
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	h.HandleFleetContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	var fleet AgentFleetContext
	if err := json.NewDecoder(rec.Body).Decode(&fleet); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(fleet.Resources) != 1 {
		t.Fatalf("Resources len = %d; want 1", len(fleet.Resources))
	}
	got := fleet.Resources[0].Findings
	want := AgentFleetFindingCounts{Total: 4, Critical: 1, Warning: 2, Info: 1}
	if got != want {
		t.Errorf("findings counts = %+v; want %+v", got, want)
	}
}

func TestHandleAgentFleetContext_PropagatesPendingApprovalCount(t *testing.T) {
	// pendingApprovalCount must reflect the provider's per-resource
	// count so an agent triaging the fleet sees governance-blocked
	// resources at a glance.
	h, _, approvals := fleetFixtureHandlers(t, []unified.Resource{
		{ID: "vm:101", Type: "vm", Name: "db-01"},
		{ID: "container:web-1", Type: "container", Name: "web-1"},
	})
	approvals.byResource["vm:101"] = []AgentResourceApprovalSummary{
		{ID: "appr-1"},
		{ID: "appr-2"},
	}
	// container:web-1 has no pending approvals — should surface as 0.

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	h.HandleFleetContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	var fleet AgentFleetContext
	if err := json.NewDecoder(rec.Body).Decode(&fleet); err != nil {
		t.Fatalf("decode: %v", err)
	}
	byID := map[string]AgentFleetResourceSummary{}
	for _, s := range fleet.Resources {
		byID[s.CanonicalID] = s
	}
	if got := byID["vm:101"].PendingApprovalCount; got != 2 {
		t.Errorf("vm:101 pendingApprovalCount = %d; want 2", got)
	}
	if got := byID["container:web-1"].PendingApprovalCount; got != 0 {
		t.Errorf("container:web-1 pendingApprovalCount = %d; want 0", got)
	}
}

func TestHandleAgentFleetContext_EmptyArrayWhenRegistryEmpty(t *testing.T) {
	// Empty registry must surface as `resources: []`, never null —
	// agents iterate without nil-checking. This mirrors the
	// iteration-safe contract the per-resource bundle's sections
	// already follow.
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/fleet-context", nil)
	h.HandleFleetContext(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"resources":[]`) {
		t.Errorf("expected resources to surface as empty array; body=%s", body)
	}
}

func TestHandleAgentFleetContext_RejectsNonGet(t *testing.T) {
	h, _, _ := fleetFixtureHandlers(t, []unified.Resource{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agent/fleet-context", nil)
	h.HandleFleetContext(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d, want 405", rec.Code)
	}
}

func TestExtractAgentResourceContextID(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/api/agent/resource-context/vm:101", "vm:101"},
		{"/api/agent/resource-context/vm:101/", "vm:101"},
		{"/api/agent/resource-context/instance:node:200", "instance:node:200"},
		{"/api/agent/resource-context/", ""},
	}
	for _, tc := range cases {
		if got := extractAgentResourceContextID(tc.path); got != tc.want {
			t.Errorf("extractAgentResourceContextID(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
