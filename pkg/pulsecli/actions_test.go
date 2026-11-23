package pulsecli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/spf13/cobra"
)

func TestActionsRedirectPreservesStricterCallerPolicy(t *testing.T) {
	for _, stop := range []error{errors.New("caller redirect denied"), http.ErrUseLastResponse} {
		t.Run(stop.Error(), func(t *testing.T) {
			var requests, callbacks atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				http.Redirect(w, r, "/accepted", http.StatusTemporaryRedirect)
			}))
			defer server.Close()
			client := server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error {
				callbacks.Add(1)
				return stop
			}
			_, _, err := executeCLIRedirectCommand(t, cliRedirectCommands()[3], server.URL, client)
			if err == nil || (stop != http.ErrUseLastResponse && !errors.Is(err, stop)) {
				t.Fatalf("caller refusal was lost: %v", err)
			}
			if stop == http.ErrUseLastResponse && !strings.Contains(err.Error(), "307 Temporary Redirect") {
				t.Fatalf("last response was not handled as an HTTP refusal: %v", err)
			}
			if requests.Load() != 1 || callbacks.Load() != 1 {
				t.Fatalf("requests=%d callbacks=%d, want one of each", requests.Load(), callbacks.Load())
			}
			if client.CheckRedirect(nil, nil) != stop {
				t.Fatal("caller redirect policy was mutated")
			}
		})
	}
}

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

func TestActionsPlanCommandAllowsOmittedRequestedByCompatibilityLabel(t *testing.T) {
	var received map[string]json.RawMessage
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(unified.ActionPlan{
			ActionID: "act_optional", RequestID: "req-optional", Allowed: true,
			PlannedAt: time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
			ExpiresAt: time.Date(2026, 5, 3, 12, 5, 0, 0, time.UTC), PlanHash: "sha256:optional",
		})
	}))
	defer server.Close()

	cmd := newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   server.URL,
	})
	cmd.SetArgs([]string{
		"actions", "plan",
		"--request-id", "req-optional",
		"--resource-id", "vm:42",
		"--capability", "restart",
		"--reason", "Recover",
	})
	cmd.SetOut(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute actions plan without requested-by: %v", err)
	}
	if receivedAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", receivedAuth)
	}
	if _, present := received["requestedBy"]; present {
		t.Fatalf("optional compatibility label unexpectedly emitted: %s", received["requestedBy"])
	}
	planCmd, _, err := cmd.Find([]string{"actions", "plan"})
	if err != nil {
		t.Fatalf("find actions plan command: %v", err)
	}
	flag := planCmd.Flags().Lookup("requested-by")
	if flag == nil || !strings.Contains(flag.Usage, "deprecated compatibility label") || !strings.Contains(flag.Usage, "server derives requester identity") {
		t.Fatalf("requested-by flag does not document non-authoritative compatibility semantics: %#v", flag)
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

func TestActionsExecuteCommandPostsExecutionRequest(t *testing.T) {
	now := time.Date(2026, 5, 4, 14, 45, 0, 0, time.UTC)
	var receivedAuth string
	var received struct {
		Reason string `json:"reason"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/actions/act_test/execute" {
			t.Fatalf("path = %s, want /api/actions/act_test/execute", r.URL.Path)
		}
		receivedAuth = r.Header.Get("Authorization")
		if got := r.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(actionExecutionResponse{
			ActionID: "act_test",
			State:    unified.ActionStateCompleted,
			Result:   &unified.ExecutionResult{Success: true, Output: "restart dispatched"},
			Audit: unified.ActionAuditRecord{
				ID:        "act_test",
				CreatedAt: now.Add(-time.Minute),
				UpdatedAt: now,
				State:     unified.ActionStateCompleted,
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
				Result: &unified.ExecutionResult{Success: true, Output: "restart dispatched"},
			},
		})
	}))
	defer server.Close()

	cmd := newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   server.URL + "/api",
	})
	cmd.SetArgs([]string{
		"actions", "execute",
		"--action-id", "act_test",
		"--reason", "inside approved maintenance window",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute actions execute: %v", err)
	}
	if receivedAuth != "Bearer test-token" {
		t.Fatalf("Authorization = %q", receivedAuth)
	}
	if received.Reason != "inside approved maintenance window" {
		t.Fatalf("received execution request = %#v", received)
	}

	var execution actionExecutionResponse
	if err := json.Unmarshal(out.Bytes(), &execution); err != nil {
		t.Fatalf("decode command output: %v\n%s", err, out.String())
	}
	if execution.ActionID != "act_test" || execution.State != unified.ActionStateCompleted || execution.Result == nil || execution.Result.Output != "restart dispatched" {
		t.Fatalf("execution output = %+v", execution)
	}
}

func TestActionsExecuteCommandValidatesRequiredFields(t *testing.T) {
	cmd := newTestActionsRootCommand(nil)
	cmd.SetArgs([]string{
		"actions", "execute",
		"--api-url", "http://127.0.0.1:7655",
		"--action-id", "act_test",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "api token is required") {
		t.Fatalf("expected token error, got %v", err)
	}

	cmd = newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   "http://127.0.0.1:7655",
	})
	cmd.SetArgs([]string{"actions", "execute"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "actionId is required") {
		t.Fatalf("expected action id error, got %v", err)
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
	var receivedBody map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request: %v", err)
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if err := json.Unmarshal(body, &receivedBody); err != nil {
			t.Fatalf("decode raw request: %v", err)
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
		"requestedBy": "agent:file",
		"actor": {
			"subjectId": "forged-user",
			"kind": "user",
			"credentialId": "session:forged",
			"orgId": "other-org"
		}
	}`))
	cmd.SetOut(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute actions plan with stdin: %v", err)
	}
	if received.RequestID != "req-file" || received.RequestedBy != "agent:file" {
		t.Fatalf("received request = %+v", received)
	}
	if _, present := receivedBody["actor"]; present || received.Actor.SubjectID != "" {
		t.Fatalf("server-owned actor leaked from request-file input: body=%s actor=%+v", receivedBody["actor"], received.Actor)
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

func TestActionAPIDefaultsSeedsFromEnvironment(t *testing.T) {
	deps := &ActionsDeps{Getenv: func(key string) string {
		switch key {
		case "PULSE_API_URL":
			return "  https://pulse.example.test  "
		case "PULSE_API_TOKEN":
			return "  token-123  "
		}
		return ""
	}}
	apiURL, token := actionAPIDefaults(deps)
	if apiURL != "https://pulse.example.test" {
		t.Fatalf("apiURL = %q, want trimmed env value", apiURL)
	}
	if token != "token-123" {
		t.Fatalf("token = %q, want trimmed env value", token)
	}

	empty := &ActionsDeps{Getenv: func(string) string { return "" }}
	apiURL, token = actionAPIDefaults(empty)
	if apiURL != defaultPulseAPIURL {
		t.Fatalf("apiURL = %q, want default %q when env is unset", apiURL, defaultPulseAPIURL)
	}
	if token != "" {
		t.Fatalf("token = %q, want empty when env is unset", token)
	}
}

func TestActionsApproveAndRejectShorthandsSendPlanHashAndCLISurface(t *testing.T) {
	now := time.Date(2026, 11, 23, 14, 0, 0, 0, time.UTC)
	var seenSurface string
	var seenAuth string
	var seenBody struct {
		Outcome  unified.ApprovalOutcome `json:"outcome"`
		Reason   string                  `json:"reason"`
		PlanHash string                  `json:"planHash"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenSurface = r.Header.Get(agentcapabilities.AgentSurfaceHeader)
		seenAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		state := unified.ActionStateApproved
		if seenBody.Outcome == unified.OutcomeRejected {
			state = unified.ActionStateRejected
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(actionDecisionResponse{
			ActionID: "act_shorthand",
			State:    state,
			Approval: unified.ActionApprovalRecord{
				Actor:     "operator",
				Method:    unified.MethodAPI,
				Timestamp: now,
				Outcome:   seenBody.Outcome,
				Reason:    seenBody.Reason,
			},
			Audit: unified.ActionAuditRecord{ID: "act_shorthand", State: state},
		})
	}))
	defer server.Close()

	store := newCLITestAuthStore(t)
	if _, err := store.SaveProfile("default", server.URL, "saved-action-token", "operator"); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	run := func(args ...string) error {
		cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
			Actions: &ActionsDeps{
				HTTPClient: server.Client(),
				Store:      store,
				Getenv:     func(string) string { return "" },
			},
		})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		return cmd.Execute()
	}

	if err := run("actions", "approve", "act_shorthand", "--plan-hash", "sha256:reviewed", "--reason", "verified"); err != nil {
		t.Fatalf("actions approve failed: %v", err)
	}
	if seenSurface != agentcapabilities.AgentSurfacePulseCLI {
		t.Fatalf("surface = %q, want %q", seenSurface, agentcapabilities.AgentSurfacePulseCLI)
	}
	if seenAuth != "Bearer saved-action-token" {
		t.Fatalf("Authorization = %q, want saved profile token", seenAuth)
	}
	if seenBody.Outcome != unified.OutcomeApproved || seenBody.PlanHash != "sha256:reviewed" || seenBody.Reason != "verified" {
		t.Fatalf("approve payload = %+v", seenBody)
	}

	if err := run("actions", "reject", "act_shorthand", "--plan-hash", "sha256:reviewed", "--reason", "not now"); err != nil {
		t.Fatalf("actions reject failed: %v", err)
	}
	if seenBody.Outcome != unified.OutcomeRejected || seenBody.PlanHash != "sha256:reviewed" || seenBody.Reason != "not now" {
		t.Fatalf("reject payload = %+v", seenBody)
	}
}

func TestActionsExecuteRenders402LicenseRequiredWithoutRetry(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = io.WriteString(w, `{"error":"license_required","message":"Fix execution requires Pulse Pro","feature":"ai_autofix","upgrade_url":"https://pulserelay.pro/upgrade"}`)
	}))
	defer server.Close()

	cmd := newTestActionsRootCommand(map[string]string{
		"PULSE_API_TOKEN": "test-token",
		"PULSE_API_URL":   server.URL,
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"actions", "execute", "--action-id", "act_402", "--plan-hash", "sha256:abc"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected HTTP 402 error")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("HTTP 402 retried: calls = %d, want 1", got)
	}
	for _, want := range []string{"Pulse Pro license", "ai_autofix", "Fix execution requires Pulse Pro", "https://pulserelay.pro/upgrade"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("402 error %q missing %q", err.Error(), want)
		}
	}
}

func newTestActionsClientCommand(server *httptest.Server, token string, tty bool) (*cobra.Command, *bytes.Buffer) {
	env := map[string]string{
		"PULSE_API_TOKEN": token,
	}
	if server != nil {
		env["PULSE_API_URL"] = server.URL
	}
	var client HTTPDoer
	if server != nil {
		client = server.Client()
	}
	cmd := NewClientRootCommand(CommandSpec{Use: "pulse"}, ClientDeps{
		Actions: &ActionsDeps{
			HTTPClient: client,
			Getenv: func(key string) string {
				return env[key]
			},
			IsTerminal: func(io.Writer) bool { return tty },
		},
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	return cmd, &out
}

func TestActionsListCommandHumanAndJSON(t *testing.T) {
	expires := time.Date(2026, 11, 23, 15, 30, 0, 0, time.UTC)
	var seenView, seenLimit, seenSurface, seenAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/actions" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		seenView = r.URL.Query().Get("view")
		seenLimit = r.URL.Query().Get("limit")
		seenSurface = r.Header.Get(agentcapabilities.AgentSurfaceHeader)
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(actionListResponse{
			View:  seenView,
			Count: 1,
			Actions: []actionAuditView{
				{
					ID:    "act_list_1",
					State: unified.ActionStatePending,
					Request: actionRequestView{
						ResourceID:     "vm:101",
						CapabilityName: "restart",
						RequestedBy:    "token:tok_1",
						Actor: actionActorView{
							Kind:            "api_token",
							SubjectID:       "token:tok_1",
							CredentialLabel: "oncall-agent",
						},
					},
					Resource: &actionResourceView{ID: "vm:101", Name: "web-01", Type: "vm"},
					Origin:   &unified.ActionOrigin{Surface: "pulse_mcp"},
					Plan: unified.ActionPlan{
						ActionID:  "act_list_1",
						ExpiresAt: expires,
						PlanHash:  "sha256:list1",
					},
				},
			},
		})
	}))
	defer server.Close()

	// Human TTY table output
	cmd, out := newTestActionsClientCommand(server, "list-token", true)
	cmd.SetArgs([]string{"actions", "list", "--view", "all", "--limit", "25"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("actions list human failed: %v", err)
	}
	if seenView != "all" || seenLimit != "25" {
		t.Fatalf("query view=%q limit=%q, want all/25", seenView, seenLimit)
	}
	if seenSurface != agentcapabilities.AgentSurfacePulseCLI {
		t.Fatalf("surface = %q, want %q", seenSurface, agentcapabilities.AgentSurfacePulseCLI)
	}
	if seenAuth != "Bearer list-token" {
		t.Fatalf("Authorization = %q", seenAuth)
	}
	humanOut := out.String()
	for _, want := range []string{
		"ACTION ID", "STATE", "CAPABILITY", "RESOURCE", "ORIGIN", "REQUESTER", "EXPIRES",
		"act_list_1", "pending_approval", "restart", "web-01 (vm:101)", "pulse_mcp", "token:tok_1 (oncall-agent)", "2026-11-23T15:30:00Z",
	} {
		if !strings.Contains(humanOut, want) {
			t.Fatalf("actions list human output missing %q:\n%s", want, humanOut)
		}
	}
	if strings.Contains(humanOut, "—") || strings.Contains(humanOut, ";") {
		t.Fatalf("actions list human output must not contain em-dash or semicolon:\n%s", humanOut)
	}

	// JSON output with --json on TTY
	cmdJSON, outJSON := newTestActionsClientCommand(server, "list-token", true)
	cmdJSON.SetArgs([]string{"actions", "list", "--view", "settled", "--json"})
	if err := cmdJSON.Execute(); err != nil {
		t.Fatalf("actions list json failed: %v", err)
	}
	var decoded actionListResponse
	if err := json.Unmarshal(outJSON.Bytes(), &decoded); err != nil {
		t.Fatalf("decode list json: %v\n%s", err, outJSON.String())
	}
	if decoded.View != "settled" || decoded.Count != 1 || decoded.Actions[0].ID != "act_list_1" {
		t.Fatalf("decoded list = %+v", decoded)
	}

	// Validation errors
	badView, _ := newTestActionsClientCommand(server, "list-token", true)
	badView.SetArgs([]string{"actions", "list", "--view", "bogus"})
	if err := badView.Execute(); err == nil || !strings.Contains(err.Error(), "view must be pending, settled, or all") {
		t.Fatalf("expected invalid view error, got %v", err)
	}

	badLimit, _ := newTestActionsClientCommand(server, "list-token", true)
	badLimit.SetArgs([]string{"actions", "list", "--limit", "501"})
	if err := badLimit.Execute(); err == nil || !strings.Contains(err.Error(), "limit must be between 1 and 500") {
		t.Fatalf("expected invalid limit error, got %v", err)
	}
}

func TestActionsGetCommandHumanJSONAndForms(t *testing.T) {
	now := time.Date(2026, 11, 23, 15, 0, 0, 0, time.UTC)
	var seenPaths []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		seenPaths = append(seenPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(actionDetailViewResponse{
			Audit: actionAuditView{
				ID:        "act_get_1",
				State:     unified.ActionStateCompleted,
				UpdatedAt: now,
				Request: actionRequestView{
					ResourceID:     "vm:202",
					CapabilityName: "restart",
					RequestedBy:    "token:tok_9",
					Actor: actionActorView{
						Kind:            "api_token",
						SubjectID:       "token:tok_9",
						CredentialLabel: "codex-ops",
					},
				},
				Resource: &actionResourceView{ID: "vm:202", Name: "db-primary", Type: "vm"},
				Origin:   &unified.ActionOrigin{Surface: "pulse_cli"},
				Plan: unified.ActionPlan{
					ActionID:             "act_get_1",
					PlanHash:             "sha256:gethash",
					ExpiresAt:            now.Add(30 * time.Minute),
					PredictedBlastRadius: []string{"vm:202", "service:postgres"},
				},
				Result: &unified.ExecutionResult{
					Success: true,
					Output:  "service restarted",
				},
				VerificationOutcome: unified.VerificationOutcome{
					Status:          unified.VerificationVerified,
					EvidenceSummary: "systemd active confirmed",
				},
			},
			Events: []unified.ActionLifecycleEvent{
				{ActionID: "act_get_1", State: unified.ActionStatePending, Timestamp: now},
				{ActionID: "act_get_1", State: unified.ActionStateCompleted, Timestamp: now.Add(time.Second)},
			},
			Readiness: &actionReadinessView{
				Ready:   true,
				Code:    "ready",
				Message: "Target agent connected",
			},
			Receipt: map[string]any{"id": "rcpt_42", "status": "completed"},
		})
	}))
	defer server.Close()

	// Positional form in human mode
	cmd, out := newTestActionsClientCommand(server, "get-token", true)
	cmd.SetArgs([]string{"actions", "get", "act_get_1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("actions get positional failed: %v", err)
	}
	humanOut := out.String()
	for _, want := range []string{
		"Action ID:", "act_get_1",
		"State:", "completed",
		"Capability:", "restart",
		"Resource:", "db-primary (vm:202)",
		"Origin:", "pulse_cli",
		"Requester:", "token:tok_9 (codex-ops)",
		"Plan hash:", "sha256:gethash",
		"Expires at:", "2026-11-23T15:30:00Z",
		"Blast radius:", "vm:202, service:postgres",
		"Readiness:", "ready (ready): Target agent connected",
		"Outcome:", "success (service restarted)",
		"Verification:", "verified (systemd active confirmed)",
		"Events:", "2",
		"Receipt:", "rcpt_42",
	} {
		if !strings.Contains(humanOut, want) {
			t.Fatalf("actions get human output missing %q:\n%s", want, humanOut)
		}
	}
	if strings.Contains(humanOut, "—") || strings.Contains(humanOut, ";") {
		t.Fatalf("actions get human output must not contain em-dash or semicolon:\n%s", humanOut)
	}

	// Flag form in JSON mode with inherited --api-url
	cmdFlag, outFlag := newTestActionsClientCommand(nil, "get-token", true)
	cmdFlag.SetArgs([]string{"--api-url", server.URL, "--json", "actions", "get", "--action-id", "act_get_1"})
	if err := cmdFlag.Execute(); err != nil {
		t.Fatalf("actions get --action-id failed: %v", err)
	}
	var decoded actionDetailViewResponse
	if err := json.Unmarshal(outFlag.Bytes(), &decoded); err != nil {
		t.Fatalf("decode get json: %v\n%s", err, outFlag.String())
	}
	if decoded.Audit.ID != "act_get_1" || decoded.Audit.Plan.PlanHash != "sha256:gethash" {
		t.Fatalf("decoded get = %+v", decoded)
	}
	if len(seenPaths) != 2 || seenPaths[0] != "/api/actions/act_get_1" || seenPaths[1] != "/api/actions/act_get_1" {
		t.Fatalf("seenPaths = %v", seenPaths)
	}
}

func TestActionsRefreshCommandPostsPlanHashAndRendersOutput(t *testing.T) {
	newExpiry := time.Date(2026, 11, 23, 16, 0, 0, 0, time.UTC)
	var seenPath string
	var seenBody struct {
		PlanHash string `json:"planHash"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		seenPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decode refresh body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(actionDetailViewResponse{
			Audit: actionAuditView{
				ID:    "act_refreshed_2",
				State: unified.ActionStatePending,
				Plan: unified.ActionPlan{
					ActionID:  "act_refreshed_2",
					PlanHash:  "sha256:freshhash",
					ExpiresAt: newExpiry,
				},
			},
			Events: []unified.ActionLifecycleEvent{},
		})
	}))
	defer server.Close()

	// Human output via positional action ID
	cmd, out := newTestActionsClientCommand(server, "refresh-token", true)
	cmd.SetArgs([]string{"actions", "refresh", "act_expired_1", "--plan-hash", "sha256:oldhash"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("actions refresh human failed: %v", err)
	}
	if seenPath != "/api/actions/act_expired_1/refresh" || seenBody.PlanHash != "sha256:oldhash" {
		t.Fatalf("refresh request path=%q body=%+v", seenPath, seenBody)
	}
	humanOut := out.String()
	for _, want := range []string{"act_refreshed_2", "sha256:freshhash", "2026-11-23T16:00:00Z"} {
		if !strings.Contains(humanOut, want) {
			t.Fatalf("refresh human output missing %q:\n%s", want, humanOut)
		}
	}
	if strings.Contains(humanOut, "—") || strings.Contains(humanOut, ";") {
		t.Fatalf("refresh human output must not contain em-dash or semicolon:\n%s", humanOut)
	}

	// JSON output via --action-id
	cmdJSON, outJSON := newTestActionsClientCommand(server, "refresh-token", true)
	cmdJSON.SetArgs([]string{"actions", "refresh", "--action-id", "act_expired_1", "--plan-hash", "sha256:oldhash", "--json"})
	if err := cmdJSON.Execute(); err != nil {
		t.Fatalf("actions refresh json failed: %v", err)
	}
	var decoded actionDetailViewResponse
	if err := json.Unmarshal(outJSON.Bytes(), &decoded); err != nil {
		t.Fatalf("decode refresh json: %v", err)
	}
	if decoded.Audit.ID != "act_refreshed_2" || decoded.Audit.Plan.PlanHash != "sha256:freshhash" {
		t.Fatalf("decoded refresh = %+v", decoded)
	}

	// Missing --plan-hash fails cleanly
	badCmd, _ := newTestActionsClientCommand(server, "refresh-token", true)
	badCmd.SetArgs([]string{"actions", "refresh", "act_expired_1"})
	if err := badCmd.Execute(); err == nil || !strings.Contains(err.Error(), "planHash is required") {
		t.Fatalf("expected missing planHash error, got %v", err)
	}
}

func TestActionsWatchCommandPollsTransitionsTerminalDecidedAndTimeout(t *testing.T) {
	now := time.Date(2026, 11, 23, 15, 0, 0, 0, time.UTC)

	t.Run("terminal transitions emitted once in human and NDJSON modes", func(t *testing.T) {
		states := []unified.ActionState{
			unified.ActionStatePending,
			unified.ActionStatePending, // duplicate must not print twice
			unified.ActionStateApproved,
			unified.ActionStateExecuting,
			unified.ActionStateCompleted,
		}
		var pollCount atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/actions/act_watch_1" {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			idx := int(pollCount.Add(1)) - 1
			if idx >= len(states) {
				idx = len(states) - 1
			}
			state := states[idx]
			audit := actionAuditView{
				ID:        "act_watch_1",
				State:     state,
				UpdatedAt: now.Add(time.Duration(idx) * time.Second),
				Request: actionRequestView{
					ResourceID:     "vm:101",
					CapabilityName: "restart",
				},
				Plan: unified.ActionPlan{ActionID: "act_watch_1", PlanHash: "sha256:w1"},
			}
			if state == unified.ActionStateCompleted {
				audit.Result = &unified.ExecutionResult{Success: true, Output: "done"}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(actionDetailViewResponse{Audit: audit})
		}))
		defer server.Close()

		// Human mode
		cmd, out := newTestActionsClientCommand(server, "watch-token", true)
		cmd.SetArgs([]string{"actions", "watch", "act_watch_1", "--interval", "5ms", "--timeout", "5s"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("actions watch human failed: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != 4 {
			t.Fatalf("got %d lines, want 4 unique transitions:\n%s", len(lines), out.String())
		}
		for i, wantState := range []string{"pending_approval", "approved", "executing", "completed"} {
			if !strings.Contains(lines[i], wantState) {
				t.Fatalf("line[%d] = %q, want state %q", i, lines[i], wantState)
			}
		}
		if strings.Contains(out.String(), "—") || strings.Contains(out.String(), ";") {
			t.Fatalf("watch human output must not contain em-dash or semicolon:\n%s", out.String())
		}

		// NDJSON mode with --json
		pollCount.Store(0)
		cmdJSON, outJSON := newTestActionsClientCommand(server, "watch-token", true)
		cmdJSON.SetArgs([]string{"actions", "watch", "act_watch_1", "--interval", "5ms", "--timeout", "5s", "--json"})
		if err := cmdJSON.Execute(); err != nil {
			t.Fatalf("actions watch json failed: %v", err)
		}
		jsonLines := strings.Split(strings.TrimSpace(outJSON.String()), "\n")
		if len(jsonLines) != 4 {
			t.Fatalf("got %d NDJSON lines, want 4:\n%s", len(jsonLines), outJSON.String())
		}
		for i, wantState := range []unified.ActionState{
			unified.ActionStatePending,
			unified.ActionStateApproved,
			unified.ActionStateExecuting,
			unified.ActionStateCompleted,
		} {
			var ev actionWatchEvent
			if err := json.Unmarshal([]byte(jsonLines[i]), &ev); err != nil {
				t.Fatalf("decode NDJSON line[%d]: %v", i, err)
			}
			if ev.ActionID != "act_watch_1" || ev.State != wantState {
				t.Fatalf("NDJSON line[%d] = %+v, want state %q", i, ev, wantState)
			}
		}
	})

	t.Run("until decided stops on approved", func(t *testing.T) {
		states := []unified.ActionState{
			unified.ActionStatePending,
			unified.ActionStateApproved,
			unified.ActionStateCompleted,
		}
		var pollCount atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			idx := int(pollCount.Add(1)) - 1
			if idx >= len(states) {
				idx = len(states) - 1
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(actionDetailViewResponse{
				Audit: actionAuditView{
					ID:        "act_watch_decided",
					State:     states[idx],
					UpdatedAt: now,
				},
			})
		}))
		defer server.Close()

		cmd, out := newTestActionsClientCommand(server, "watch-token", false)
		cmd.SetArgs([]string{"actions", "watch", "act_watch_decided", "--until", "decided", "--interval", "5ms", "--timeout", "5s"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("actions watch --until decided failed: %v", err)
		}
		if got := pollCount.Load(); got != 2 {
			t.Fatalf("pollCount = %d, want 2 (stopped on approved)", got)
		}
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != 2 || !strings.Contains(lines[1], `"approved"`) {
			t.Fatalf("decided output = %s", out.String())
		}
	})

	t.Run("timeout and context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(actionDetailViewResponse{
				Audit: actionAuditView{
					ID:    "act_stuck",
					State: unified.ActionStatePending,
				},
			})
		}))
		defer server.Close()

		cmd, _ := newTestActionsClientCommand(server, "watch-token", true)
		cmd.SetArgs([]string{"actions", "watch", "act_stuck", "--interval", "10ms", "--timeout", "30ms"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "timed out waiting for action act_stuck") {
			t.Fatalf("expected timeout error, got %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cmdCancel, _ := newTestActionsClientCommand(server, "watch-token", true)
		cmdCancel.SetArgs([]string{"actions", "watch", "act_stuck", "--interval", "10ms", "--timeout", "5s"})
		err = cmdCancel.ExecuteContext(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}

func TestActionsPositionalAndFlagFormsForApproveRejectExecute(t *testing.T) {
	now := time.Date(2026, 11, 23, 15, 0, 0, 0, time.UTC)
	var seenPaths []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPaths = append(seenPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/decision"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/actions/"), "/decision")
			_ = json.NewEncoder(w).Encode(actionDecisionResponse{
				ActionID: id,
				State:    unified.ActionStateApproved,
				Approval: unified.ActionApprovalRecord{Actor: "op", Method: unified.MethodAPI, Timestamp: now, Outcome: unified.OutcomeApproved},
				Audit:    unified.ActionAuditRecord{ID: id, State: unified.ActionStateApproved},
			})
		case strings.HasSuffix(r.URL.Path, "/execute"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/actions/"), "/execute")
			_ = json.NewEncoder(w).Encode(actionExecutionResponse{
				ActionID: id,
				State:    unified.ActionStateCompleted,
				Audit:    unified.ActionAuditRecord{ID: id, State: unified.ActionStateCompleted},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cases := [][]string{
		{"actions", "approve", "act_pos_1"},
		{"actions", "approve", "--action-id", "act_flag_1"},
		{"actions", "reject", "act_pos_2"},
		{"actions", "reject", "--action-id", "act_flag_2"},
		{"actions", "execute", "act_pos_3", "--plan-hash", "sha256:exec3"},
		{"actions", "execute", "--action-id", "act_flag_3", "--plan-hash", "sha256:exec3"},
	}
	for _, args := range cases {
		cmd, _ := newTestActionsClientCommand(server, "op-token", false)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("args %v failed: %v", args, err)
		}
	}
	wantPaths := []string{
		"/api/actions/act_pos_1/decision",
		"/api/actions/act_flag_1/decision",
		"/api/actions/act_pos_2/decision",
		"/api/actions/act_flag_2/decision",
		"/api/actions/act_pos_3/execute",
		"/api/actions/act_flag_3/execute",
	}
	if strings.Join(seenPaths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("seenPaths = %v, want %v", seenPaths, wantPaths)
	}

	conflictCmd, _ := newTestActionsClientCommand(server, "op-token", false)
	conflictCmd.SetArgs([]string{"actions", "execute", "act_a", "--action-id", "act_b"})
	if err := conflictCmd.Execute(); err == nil || !strings.Contains(err.Error(), "conflicting action IDs") {
		t.Fatalf("expected conflicting action IDs error, got %v", err)
	}
}

func TestActionsPlanHumanAndJSONOutput(t *testing.T) {
	plannedAt := time.Date(2026, 11, 23, 15, 0, 0, 0, time.UTC)
	expiresAt := plannedAt.Add(30 * time.Minute)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(unified.ActionPlan{
			ActionID:             "act_plan_human",
			RequestID:            "req-human-1",
			Allowed:              true,
			RequiresApproval:     true,
			ApprovalPolicy:       unified.ApprovalAdmin,
			PredictedBlastRadius: []string{"vm:42", "service:api"},
			PlannedAt:            plannedAt,
			ExpiresAt:            expiresAt,
			ResourceVersion:      "rv1",
			PolicyVersion:        "pv1",
			PlanHash:             "sha256:planhuman",
		})
	}))
	defer server.Close()

	// Human TTY mode
	cmd, out := newTestActionsClientCommand(server, "plan-token", true)
	cmd.SetArgs([]string{
		"actions", "plan",
		"--request-id", "req-human-1",
		"--resource-id", "vm:42",
		"--capability", "restart",
		"--reason", "Recover service",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("actions plan human failed: %v", err)
	}
	humanOut := out.String()
	for _, want := range []string{
		"Action ID:", "act_plan_human",
		"Origin:", "pulse_cli",
		"Plan hash:", "sha256:planhuman",
		"Expires at:", "2026-11-23T15:30:00Z",
		"Blast radius:", "vm:42, service:api",
	} {
		if !strings.Contains(humanOut, want) {
			t.Fatalf("actions plan human output missing %q:\n%s", want, humanOut)
		}
	}
	if strings.Contains(humanOut, "—") || strings.Contains(humanOut, ";") {
		t.Fatalf("actions plan human output must not contain em-dash or semicolon:\n%s", humanOut)
	}

	// Explicit --json on TTY
	cmdJSON, outJSON := newTestActionsClientCommand(server, "plan-token", true)
	cmdJSON.SetArgs([]string{
		"actions", "plan", "--json",
		"--request-id", "req-human-1",
		"--resource-id", "vm:42",
		"--capability", "restart",
		"--reason", "Recover service",
	})
	if err := cmdJSON.Execute(); err != nil {
		t.Fatalf("actions plan --json failed: %v", err)
	}
	var plan unified.ActionPlan
	if err := json.Unmarshal(outJSON.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan json: %v\n%s", err, outJSON.String())
	}
	if plan.ActionID != "act_plan_human" || plan.PlanHash != "sha256:planhuman" {
		t.Fatalf("decoded plan = %+v", plan)
	}
}
