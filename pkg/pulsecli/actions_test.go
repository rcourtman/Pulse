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
