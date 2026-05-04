package pulsecli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/spf13/cobra"
)

func TestActionsPlanCommandPostsCanonicalRequest(t *testing.T) {
	plannedAt := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	var received unified.ActionRequest
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/actions/plan" {
			t.Fatalf("path = %s, want /api/actions/plan", r.URL.Path)
		}
		receivedAuth = r.Header.Get("Authorization")

		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if err := decoder.Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		plan := unified.ActionPlan{
			ActionID:          "act_test",
			RequestID:         received.RequestID,
			Allowed:           true,
			ApprovalPolicy:    unified.ApprovalAdmin,
			RollbackAvailable: false,
			PlannedAt:         plannedAt,
			ExpiresAt:         plannedAt.Add(5 * time.Minute),
			ResourceVersion:   "resource:sha256:test",
			PolicyVersion:     "policy:sha256:test",
			PlanHash:          "sha256:test",
			Preflight: &unified.ActionPreflight{
				Target:          received.ResourceID,
				DryRunAvailable: false,
				GeneratedAt:     plannedAt,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(plan); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	cmd := newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   server.URL + "/api",
	})
	cmd.SetArgs([]string{
		"actions", "plan",
		"--request-id", "req-1",
		"--resource-id", "vm:42",
		"--capability", "restart",
		"--reason", "Recover after confirmed outage",
		"--requested-by", "agent:oncall-helper",
		"--params-json", `{"mode":"graceful","retries":1}`,
		"--param", "force=true",
		"--param", "note=manual",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute actions plan: %v", err)
	}
	if receivedAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", receivedAuth)
	}
	if received.RequestID != "req-1" || received.ResourceID != "vm:42" || received.CapabilityName != "restart" {
		t.Fatalf("received request identity = %+v", received)
	}
	if received.Reason != "Recover after confirmed outage" || received.RequestedBy != "agent:oncall-helper" {
		t.Fatalf("received audit fields = %+v", received)
	}
	if received.Params["mode"] != "graceful" || received.Params["force"] != true || received.Params["note"] != "manual" {
		t.Fatalf("received params = %#v", received.Params)
	}
	if got, ok := received.Params["retries"].(json.Number); !ok || got.String() != "1" {
		t.Fatalf("received retries = %#v", received.Params["retries"])
	}

	var plan unified.ActionPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("decode command output: %v\n%s", err, out.String())
	}
	if plan.ActionID != "act_test" || plan.RequestID != "req-1" {
		t.Fatalf("plan output = %+v", plan)
	}
}

func TestActionsPlanCommandRequiresToken(t *testing.T) {
	cmd := newTestActionsRootCommand(nil)
	cmd.SetArgs([]string{
		"actions", "plan",
		"--api-url", "http://127.0.0.1:7655",
		"--request-id", "req-1",
		"--resource-id", "vm:42",
		"--capability", "restart",
		"--reason", "Recover",
		"--requested-by", "agent:oncall-helper",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "api token is required") {
		t.Fatalf("expected token error, got %v", err)
	}
}

func TestActionsDecideCommandPostsApprovalDecision(t *testing.T) {
	now := time.Date(2026, 5, 4, 14, 30, 0, 0, time.UTC)
	var receivedAuth string
	var received struct {
		Outcome unified.ApprovalOutcome `json:"outcome"`
		Reason  string                  `json:"reason"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/actions/act_test/decision" {
			t.Fatalf("path = %s, want /api/actions/act_test/decision", r.URL.Path)
		}
		receivedAuth = r.Header.Get("Authorization")
		if got := r.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(actionDecisionResponse{
			ActionID: "act_test",
			State:    unified.ActionStateApproved,
			Approval: unified.ActionApprovalRecord{
				Actor:     "operator@example.com",
				Method:    unified.MethodAPI,
				Timestamp: now,
				Outcome:   unified.OutcomeApproved,
				Reason:    received.Reason,
			},
			Audit: unified.ActionAuditRecord{
				ID:        "act_test",
				CreatedAt: now.Add(-time.Minute),
				UpdatedAt: now,
				State:     unified.ActionStateApproved,
				Request: unified.ActionRequest{
					RequestID:      "req-1",
					ResourceID:     "vm:42",
					CapabilityName: "restart",
					Reason:         "Recover",
					RequestedBy:    "agent:oncall-helper",
				},
				Plan: unified.ActionPlan{
					ActionID:        "act_test",
					RequestID:       "req-1",
					ExpiresAt:       now.Add(5 * time.Minute),
					ResourceVersion: "resource:sha256:test",
					PolicyVersion:   "policy:sha256:test",
					PlanHash:        "sha256:test",
				},
			},
		})
	}))
	defer server.Close()

	cmd := newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   server.URL + "/api",
	})
	cmd.SetArgs([]string{
		"actions", "decide",
		"--action-id", "act_test",
		"--outcome", "approved",
		"--reason", "inside maintenance window",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute actions decide: %v", err)
	}
	if receivedAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", receivedAuth)
	}
	if received.Outcome != unified.OutcomeApproved || received.Reason != "inside maintenance window" {
		t.Fatalf("received decision = %#v", received)
	}

	var decision actionDecisionResponse
	if err := json.Unmarshal(out.Bytes(), &decision); err != nil {
		t.Fatalf("decode command output: %v\n%s", err, out.String())
	}
	if decision.ActionID != "act_test" || decision.State != unified.ActionStateApproved || decision.Approval.Outcome != unified.OutcomeApproved {
		t.Fatalf("decision output = %+v", decision)
	}
}

func TestActionsDecideCommandRequiresToken(t *testing.T) {
	cmd := newTestActionsRootCommand(nil)
	cmd.SetArgs([]string{
		"actions", "decide",
		"--api-url", "http://127.0.0.1:7655",
		"--action-id", "act_test",
		"--outcome", "approved",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "api token is required") {
		t.Fatalf("expected token error, got %v", err)
	}
}

func TestActionsDecideCommandValidatesDecisionFields(t *testing.T) {
	cmd := newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   "http://127.0.0.1:7655",
	})
	cmd.SetArgs([]string{"actions", "decide", "--outcome", "approved"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "actionId is required") {
		t.Fatalf("expected action id error, got %v", err)
	}

	cmd = newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   "http://127.0.0.1:7655",
	})
	cmd.SetArgs([]string{"actions", "decide", "--action-id", "act_test", "--outcome", "maybe"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "outcome must be approved or rejected") {
		t.Fatalf("expected outcome error, got %v", err)
	}
}

func TestActionsCapabilitiesCommandFetchesResourceFacets(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/resources/vm:42/facets" {
			t.Fatalf("path = %s, want /api/resources/vm:42/facets", r.URL.Path)
		}
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resourceId": "vm:42",
			"capabilities": []map[string]any{
				{
					"name":                 "restart",
					"type":                 unified.CapabilityTypeCommon,
					"description":          "Restart the VM",
					"minimumApprovalLevel": unified.ApprovalAdmin,
					"internalHandler":      "proxmox.vm.restart",
					"params": []map[string]any{
						{"name": "mode", "type": "string", "enum": []string{"graceful", "force"}, "defaultValue": "graceful"},
					},
				},
			},
			"recentChanges": []any{},
			"counts":        map[string]any{"recentChanges": 0},
		})
	}))
	defer server.Close()

	cmd := newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   server.URL + "/api",
	})
	cmd.SetArgs([]string{
		"actions", "capabilities",
		"--resource-id", "vm:42",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute actions capabilities: %v", err)
	}
	if receivedAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", receivedAuth)
	}
	if strings.Contains(out.String(), "proxmox.vm.restart") {
		t.Fatalf("capabilities output exposed internal handler: %s", out.String())
	}

	var capabilities actionCapabilitiesResponse
	if err := json.Unmarshal(out.Bytes(), &capabilities); err != nil {
		t.Fatalf("decode command output: %v\n%s", err, out.String())
	}
	if capabilities.ResourceID != "vm:42" || capabilities.Count != 1 {
		t.Fatalf("capabilities output = %+v", capabilities)
	}
	if capabilities.Capabilities[0].Name != "restart" || capabilities.Capabilities[0].Params[0].Name != "mode" {
		t.Fatalf("capabilities output = %+v", capabilities)
	}
}

func TestActionsCapabilitiesCommandRequiresToken(t *testing.T) {
	cmd := newTestActionsRootCommand(nil)
	cmd.SetArgs([]string{
		"actions", "capabilities",
		"--api-url", "http://127.0.0.1:7655",
		"--resource-id", "vm:42",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "api token is required") {
		t.Fatalf("expected token error, got %v", err)
	}
}

func TestActionsAuditCommandFetchesActionAudits(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/audit/actions" {
			t.Fatalf("path = %s, want /api/audit/actions", r.URL.Path)
		}
		if got := r.URL.Query().Get("resourceId"); got != "vm:42" {
			t.Fatalf("resourceId query = %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Fatalf("limit query = %q", got)
		}
		if got := r.URL.Query().Get("since"); got != "2026-05-03T10:00:00Z" {
			t.Fatalf("since query = %q", got)
		}
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(actionAuditListResponse{
			Audits: []unified.ActionAuditRecord{
				{
					ID:        "act_test",
					CreatedAt: now,
					UpdatedAt: now,
					State:     unified.ActionStatePlanned,
					Request: unified.ActionRequest{
						RequestID:      "req-1",
						ResourceID:     "vm:42",
						CapabilityName: "restart",
						Reason:         "Recover",
						RequestedBy:    "agent:oncall-helper",
					},
					Plan: unified.ActionPlan{
						ActionID:        "act_test",
						RequestID:       "req-1",
						Allowed:         true,
						ApprovalPolicy:  unified.ApprovalAdmin,
						PlannedAt:       now,
						ExpiresAt:       now.Add(5 * time.Minute),
						ResourceVersion: "resource:sha256:test",
						PolicyVersion:   "policy:sha256:test",
						PlanHash:        "sha256:test",
					},
				},
			},
			Count:      1,
			ResourceID: "vm:42",
		})
	}))
	defer server.Close()

	cmd := newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   server.URL + "/api",
	})
	cmd.SetArgs([]string{
		"actions", "audit",
		"--resource-id", "vm:42",
		"--limit", "5",
		"--since", "2026-05-03T11:00:00+01:00",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute actions audit: %v", err)
	}
	if receivedAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", receivedAuth)
	}

	var audits actionAuditListResponse
	if err := json.Unmarshal(out.Bytes(), &audits); err != nil {
		t.Fatalf("decode command output: %v\n%s", err, out.String())
	}
	if audits.ResourceID != "vm:42" || audits.Count != 1 || audits.Audits[0].ID != "act_test" {
		t.Fatalf("audit output = %+v", audits)
	}
}

func TestActionsEventsCommandFetchesLifecycleEvents(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/audit/actions/act_test/events" {
			t.Fatalf("path = %s, want /api/audit/actions/act_test/events", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "2" {
			t.Fatalf("limit query = %q", got)
		}
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(actionLifecycleEventsResponse{
			ActionID: "act_test",
			Events: []unified.ActionLifecycleEvent{
				{
					ActionID:  "act_test",
					Timestamp: now,
					State:     unified.ActionStatePlanned,
					Actor:     "agent:oncall-helper",
					Message:   "Plan created",
				},
			},
			Count: 1,
		})
	}))
	defer server.Close()

	cmd := newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   server.URL + "/api",
	})
	cmd.SetArgs([]string{
		"actions", "events",
		"--action-id", "act_test",
		"--limit", "2",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute actions events: %v", err)
	}
	if receivedAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", receivedAuth)
	}

	var events actionLifecycleEventsResponse
	if err := json.Unmarshal(out.Bytes(), &events); err != nil {
		t.Fatalf("decode command output: %v\n%s", err, out.String())
	}
	if events.ActionID != "act_test" || events.Count != 1 || events.Events[0].Message != "Plan created" {
		t.Fatalf("events output = %+v", events)
	}
}

func TestActionsEventsCommandRequiresActionID(t *testing.T) {
	cmd := newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   "http://127.0.0.1:7655",
	})
	cmd.SetArgs([]string{"actions", "events"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "actionId is required") {
		t.Fatalf("expected action id error, got %v", err)
	}
}

func TestActionsPlanCommandUsesRequestFileFromStdin(t *testing.T) {
	var received unified.ActionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(unified.ActionPlan{
			ActionID:        "act_stdin",
			RequestID:       "req-file",
			Allowed:         true,
			ApprovalPolicy:  unified.ApprovalNone,
			PlannedAt:       time.Date(2026, 5, 3, 11, 0, 0, 0, time.UTC),
			ExpiresAt:       time.Date(2026, 5, 3, 11, 5, 0, 0, time.UTC),
			ResourceVersion: "resource:sha256:test",
			PolicyVersion:   "policy:sha256:test",
			PlanHash:        "sha256:test",
		})
	}))
	defer server.Close()

	cmd := newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   server.URL,
	})
	cmd.SetArgs([]string{"actions", "plan", "--request-file", "-"})
	cmd.SetIn(strings.NewReader(`{
		"requestId": "req-file",
		"resourceId": "vm:42",
		"capabilityName": "restart",
		"reason": "Recover",
		"requestedBy": "agent:file"
	}`))
	cmd.SetOut(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute actions plan with stdin: %v", err)
	}
	if received.RequestID != "req-file" || received.RequestedBy != "agent:file" {
		t.Fatalf("received request = %+v", received)
	}
}

func newTestActionsRootCommand(env map[string]string) *cobra.Command {
	return NewRootCommand(
		CommandSpec{
			Use:     "pulse",
			Short:   "Pulse",
			Long:    "Pulse",
			Version: "1.2.3",
		},
		RuntimeSpec{},
		CommandDeps{
			Actions: &ActionsDeps{
				Getenv: func(key string) string {
					if env == nil {
						return ""
					}
					return env[key]
				},
			},
		},
	)
}
