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
