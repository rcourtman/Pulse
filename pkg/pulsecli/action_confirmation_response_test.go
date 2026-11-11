package pulsecli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func actionConfirmationCommands() []cliRedirectCommand {
	return cliRedirectCommands()[1:4] // The registered plan, decide and execute commands.
}

func confirmationEnvelope(command string) map[string]any {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	plan := unified.ActionPlan{
		ActionID: "synthetic-action", RequestID: "synthetic-request", Allowed: true,
		PlannedAt: now, ExpiresAt: now.Add(5 * time.Minute), PlanHash: "sha256:synthetic-plan",
	}
	approval := unified.ActionApprovalRecord{Outcome: unified.OutcomeApproved, Timestamp: now, Method: unified.MethodAPI}
	audit := unified.ActionAuditRecord{ID: plan.ActionID, State: unified.ActionStateApproved, Plan: plan}
	var response any = plan
	switch command {
	case "decide":
		response = actionDecisionResponse{ActionID: audit.ID, State: audit.State, Approval: approval, Audit: audit}
	case "execute":
		audit.State = unified.ActionStateCompleted
		audit.Result = &unified.ExecutionResult{Success: true, Output: "Synthetic dispatch result"}
		response = actionExecutionResponse{ActionID: audit.ID, State: audit.State, Result: audit.Result, Audit: audit}
	}
	encoded, _ := json.Marshal(response)
	var envelope map[string]any
	_ = json.Unmarshal(encoded, &envelope)
	return envelope
}

func encodeConfirmation(t *testing.T, envelope map[string]any) string {
	t.Helper()
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func runActionConfirmationFixture(t *testing.T, command cliRedirectCommand, body string, status int, chunked bool) (string, string, error) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != command.path || r.Header.Get("Authorization") != "Bearer synthetic-cli-token" {
			t.Error("action confirmation request changed method, path or authentication")
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode action request: %v", err)
		}
		if request["reason"] != "synthetic-private-action" {
			t.Error("action reason changed")
		}
		switch command.name {
		case "plan":
			if request["requestId"] != "synthetic-request" || request["resourceId"] != "vm:42" || request["capabilityName"] != "restart" {
				t.Error("action planning request binding changed")
			}
		case "decide":
			if request["outcome"] != "approved" {
				t.Error("approval outcome changed")
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if chunked {
			w.(http.Flusher).Flush()
		}
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()
	stdout, stderr, err := executeCLIRedirectCommand(t, command, server.URL, server.Client())
	if requests.Load() != 1 {
		t.Fatalf("action confirmation made %d requests, want one without POST retry", requests.Load())
	}
	return stdout, stderr, err
}

func TestActionsConfirmationRejectsUnboundOrIncompleteResponses(t *testing.T) {
	for _, command := range actionConfirmationCommands() {
		t.Run(command.name, func(t *testing.T) {
			cases := []struct{ name, body string }{
				{"null-envelope", `null`}, {"empty-object", `{}`},
				{"error-envelope", `{"error":"synthetic-unrelated-response"}`},
				{"empty-body", ``}, {"array-envelope", `[]`},
				{"truncated", `{"actionId":`},
				{"trailing-json", encodeConfirmation(t, confirmationEnvelope(command.name)) + ` null`},
			}
			badTimestamp := confirmationEnvelope(command.name)
			if command.name == "plan" {
				badTimestamp["plannedAt"] = "synthetic-unrelated-response"
			} else {
				badTimestamp["audit"].(map[string]any)["createdAt"] = "synthetic-unrelated-response"
			}
			cases = append(cases, struct{ name, body string }{"private-invalid-timestamp", encodeConfirmation(t, badTimestamp)})
			fields := []string{"actionId", "state"}
			if command.name == "plan" {
				fields = []string{"actionId", "requestId", "allowed"}
			}
			for _, field := range fields {
				for _, value := range []struct {
					name  string
					value any
					omit  bool
				}{{"missing", nil, true}, {"null", nil, false}, {"wrong-type", []any{}, false}} {
					envelope := confirmationEnvelope(command.name)
					if value.omit {
						delete(envelope, field)
					} else {
						envelope[field] = value.value
					}
					cases = append(cases, struct{ name, body string }{value.name + "-" + field, encodeConfirmation(t, envelope)})
				}
				if field == "allowed" {
					continue
				}
				for _, value := range []string{"", "   "} {
					envelope := confirmationEnvelope(command.name)
					envelope[field] = value
					cases = append(cases, struct{ name, body string }{fmt.Sprintf("blank-%s-%d", field, len(value)), encodeConfirmation(t, envelope)})
				}
				if field == "requestId" || (field == "actionId" && command.name != "plan") {
					for _, value := range []string{"synthetic-unrelated-response", " synthetic-request ", " synthetic-action "} {
						envelope := confirmationEnvelope(command.name)
						envelope[field] = value
						cases = append(cases, struct{ name, body string }{"mismatch-" + field + "-" + value, encodeConfirmation(t, envelope)})
					}
				}
			}
			for _, framing := range []string{"fixed", "chunked"} {
				t.Run(framing, func(t *testing.T) {
					for _, tc := range cases {
						t.Run(tc.name, func(t *testing.T) {
							stdout, stderr, err := runActionConfirmationFixture(t, command, tc.body, http.StatusOK, framing == "chunked")
							if err == nil || !strings.Contains(err.Error(), "response") {
								t.Errorf("unconfirmed action must return a response error, got %v", err)
							}
							if stdout != "" {
								t.Error("unconfirmed action emitted successful JSON")
							}
							combined := stderr + fmt.Sprint(err)
							if strings.Contains(combined, "synthetic-unrelated-response") || strings.Contains(combined, "synthetic-cli-token") || strings.Contains(combined, "synthetic-private-action") {
								t.Error("unconfirmed response diagnostic disclosed response or request contents")
							}
							if err != nil && (!strings.Contains(err.Error(), "unconfirmed") || !strings.Contains(err.Error(), "audit before retrying")) {
								t.Error("unconfirmed POST must advise audit inspection, not a blind retry")
							}
						})
					}
				})
			}
		})
	}
}

func TestActionsConfirmationPreservesBoundOutcomes(t *testing.T) {
	for _, command := range actionConfirmationCommands() {
		t.Run(command.name, func(t *testing.T) {
			outcomes := []string{"true", "false"}
			if command.name != "plan" {
				outcomes = []string{"approved", "pending_approval", "rejected", "executing", "completed", "failed", "future_state"}
			}
			for _, outcome := range outcomes {
				for _, framing := range []string{"fixed", "chunked"} {
					t.Run(outcome+"/"+framing, func(t *testing.T) {
						envelope := confirmationEnvelope(command.name)
						envelope["futureEnvelopeField"] = true
						if command.name == "plan" {
							envelope["allowed"] = outcome == "true"
						} else {
							envelope["state"] = outcome
							audit := envelope["audit"].(map[string]any)
							audit["state"] = outcome
							if command.name == "decide" {
								// A replay may return the last decision from a different
								// approver, including a rejection after our approval.
								approval := envelope["approval"].(map[string]any)
								approval["actor"] = "synthetic-other-approver"
								if outcome == "rejected" {
									approval["outcome"] = "rejected"
								}
							}
							if command.name == "execute" && outcome != "completed" {
								delete(envelope, "result")
								delete(audit, "result")
								if outcome == "failed" {
									result := map[string]any{"success": false, "errorMessage": "Synthetic execution failure"}
									envelope["result"], audit["result"] = result, result
								}
							}
						}
						body := encodeConfirmation(t, envelope)
						stdout, stderr, err := runActionConfirmationFixture(t, command, body, http.StatusOK, framing == "chunked")
						if err != nil || stderr != "" {
							t.Fatalf("bound action outcome rejected: %v; stderr=%q", err, stderr)
						}
						var want any = &unified.ActionPlan{}
						switch command.name {
						case "decide":
							want = &actionDecisionResponse{}
						case "execute":
							want = &actionExecutionResponse{}
						}
						if err := json.Unmarshal([]byte(body), want); err != nil {
							t.Fatal(err)
						}
						encoded, err := json.MarshalIndent(want, "", "  ")
						if err != nil || stdout != string(encoded)+"\n" {
							t.Fatal("bound confirmation output changed")
						}
					})
				}
			}
		})
	}
}

func TestActionsConfirmationPreservesHTTPRefusals(t *testing.T) {
	for _, command := range actionConfirmationCommands() {
		for _, status := range []int{401, 403, 409, 503} {
			t.Run(fmt.Sprintf("%s/%d", command.name, status), func(t *testing.T) {
				stdout, _, err := runActionConfirmationFixture(t, command, `{"error":"synthetic refusal"}`, status, false)
				if err == nil || !strings.Contains(err.Error(), http.StatusText(status)) || stdout != "" {
					t.Fatalf("HTTP refusal was not preserved: error=%v; stdout=%q", err, stdout)
				}
			})
		}
	}
}
