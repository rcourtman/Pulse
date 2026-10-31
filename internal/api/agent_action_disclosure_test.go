package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// These strings deliberately match no credential-shaped redactor. Command and
// output privacy must not depend on guessing whether their contents are secrets.
const agentPrivateDetail = "scope-only-private-detail"

func agentDisclosureRecord(t *testing.T, legacy bool) unified.ActionAuditRecord {
	t.Helper()
	now := time.Now().UTC()
	record := unified.ActionAuditRecord{
		ID: "disclosure-action", State: unified.ActionStateFailed, CreatedAt: now, UpdatedAt: now,
		Request: unified.ActionRequest{
			ResourceID: "vm:101", CapabilityName: "restart", RequestedBy: "operator",
			Params: map[string]any{"command": "printf " + agentPrivateDetail},
		},
		Result: &unified.ExecutionResult{
			ErrorMessage: "plan_drift: " + agentPrivateDetail,
			Verification: &unified.ActionVerificationResult{
				Ran: true, Success: false, Command: "check " + agentPrivateDetail,
				Output: agentPrivateDetail, Note: agentPrivateDetail, RanAt: now,
			},
		},
	}
	if legacy {
		return record
	}
	truth := apiActionResultTruth(t, unified.ActionExecutionFailed, unified.ActionVerificationContradicted, unified.ActionEvidenceIndependent)
	truth.Execution.ReasonCode = agentPrivateDetail
	truth.Execution.Summary = agentPrivateDetail
	truth.Verification.ReasonCode = agentPrivateDetail
	truth.Verification.Summary = agentPrivateDetail
	truth.Verification.Evidence[0].ID = agentPrivateDetail
	truth.Verification.Evidence[0].Method = agentPrivateDetail
	truth.Verification.Evidence[0].Summary = agentPrivateDetail
	truth.Verification.Evidence[0].ReasonCode = agentPrivateDetail
	truth.Verification.Evidence[0].Refs = []unified.ActionEvidenceRef{{ID: agentPrivateDetail, Kind: agentPrivateDetail, Digest: "sha256:" + strings.Repeat("b", 64)}}
	truth.Verification.Evidence[0].Digest = ""
	truth.Compensation = unified.ActionCompensationTruth{
		Support: unified.ActionCompensationDeclared, Status: unified.ActionCompensationSucceeded,
		Strategy: agentPrivateDetail, Trigger: agentPrivateDetail, ReasonCode: agentPrivateDetail,
		Summary: agentPrivateDetail, AttemptID: agentPrivateDetail, StepID: agentPrivateDetail,
		StartedAt: &now, CompletedAt: &now,
		Execution: &unified.ActionExecutionTruth{Status: unified.ActionExecutionSucceeded, Summary: agentPrivateDetail},
		Verification: &unified.ActionVerificationTruth{
			Status: unified.ActionVerificationConfirmed, EvidenceClass: unified.ActionEvidenceIndependent,
			Summary: agentPrivateDetail, Evidence: copyAgentActionEvidence(truth.Verification.Evidence),
		},
		Evidence: copyAgentActionEvidence(truth.Verification.Evidence),
		RestoredState: &unified.ActionRestoredStateTruth{
			SubjectID: agentPrivateDetail, ExpectedDigest: "sha256:" + strings.Repeat("a", 64),
			ObservedDigest: "sha256:" + strings.Repeat("a", 64), ObservedAt: now,
		},
	}
	normalized, err := unified.NormalizeActionResultV2(truth)
	if err != nil {
		t.Fatal(err)
	}
	record.Result.ActionResultV2 = &normalized
	return record
}

func assertAgentDisclosureWire(t *testing.T, body []byte, restricted bool) {
	t.Helper()
	if restricted {
		if strings.Contains(string(body), agentPrivateDetail) || strings.Contains(string(body), `"actionResultV2"`) {
			t.Errorf("restricted action response retained command/output details: %s", body)
		}
		for _, want := range []string{`"actionResultSummary":`, `"detailsRedacted":true`, `"commandRedacted":true`, `"errorMessage":"plan_drift:"`} {
			if !strings.Contains(string(body), want) {
				t.Errorf("restricted response lost %s: %s", want, body)
			}
		}
	} else if !strings.Contains(string(body), agentPrivateDetail) || !strings.Contains(string(body), `"actionResultV2"`) || strings.Contains(string(body), `"actionResultSummary"`) {
		t.Errorf("authorized response changed its detail contract: %s", body)
	}
}

func TestAgentActionDisclosureAcrossProducersAndScopes(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, scopes := range [][]string{
			{config.ScopeMonitoringRead},
			{config.ScopeMonitoringRead, config.ScopeActionsPlan},
			{config.ScopeMonitoringRead, config.ScopeActionsApprove},
			{config.ScopeMonitoringRead, config.ScopeAIExecute},
			{"*"}, {}, nil,
		} {
			name := strings.Join(scopes, "+")
			if legacy {
				name = "legacy/" + name
			}
			t.Run(name, func(t *testing.T) {
				record := agentDisclosureRecord(t, legacy)
				before, _ := json.Marshal(record)
				req := httptest.NewRequest(http.MethodGet, "/api/agent/events", nil)
				if scopes != nil {
					attachAPITokenRecord(req, &config.APITokenRecord{Scopes: scopes})
				}
				restricted := !requestCanReadAgentCommandPayloads(req)
				payload, ok := projectAgentActionCompletedPayload(record)
				if !ok {
					t.Fatal("terminal record did not project")
				}
				broadcaster := NewAgentEventBroadcaster()
				events, unsubscribe := broadcaster.Subscribe()
				defer unsubscribe()
				broadcaster.PublishActionCompletedRecord(record)
				for _, event := range []AgentEvent{
					{Kind: AgentEventActionCompleted, Payload: payload},
					{Kind: AgentEventActionCompleted, Payload: &payload},
					<-events,
				} {
					got := redactAgentEventCommandsForRequest(event, req)
					if !restricted && !reflect.DeepEqual(got, event) {
						t.Error("authorized event payload was changed")
					}
					body, err := json.Marshal(got)
					if err != nil {
						t.Fatal(err)
					}
					assertAgentDisclosureWire(t, body, restricted)
				}
				bundle := AgentResourceContext{RecentActions: projectAgentResourceActions([]unified.ActionAuditRecord{record})}
				redactAgentResourceContextCommandsForRequest(&bundle, req)
				body, err := json.Marshal(bundle)
				if err != nil {
					t.Fatal(err)
				}
				assertAgentDisclosureWire(t, body, restricted)
				after, _ := json.Marshal(record)
				if string(before) != string(after) {
					t.Fatal("per-request projection mutated the original action evidence")
				}
			})
		}
	}
}

func TestAgentActionDisclosureRetainsIndependentTruthAxes(t *testing.T) {
	for _, execution := range []unified.ActionExecutionStatus{unified.ActionExecutionNotRun, unified.ActionExecutionSucceeded, unified.ActionExecutionFailed, unified.ActionExecutionInconclusive} {
		for verification, classes := range map[unified.ActionVerificationStatus][]unified.ActionEvidenceClass{
			unified.ActionVerificationNotAttempted: {unified.ActionEvidenceNone},
			unified.ActionVerificationConfirmed:    {unified.ActionEvidenceAgentAttested, unified.ActionEvidenceIndependent},
			unified.ActionVerificationContradicted: {unified.ActionEvidenceAgentAttested, unified.ActionEvidenceIndependent},
			unified.ActionVerificationInconclusive: {unified.ActionEvidenceNone, unified.ActionEvidenceAgentAttested, unified.ActionEvidenceIndependent},
		} {
			for _, class := range classes {
				t.Run(string(execution)+"/"+string(verification)+"/"+string(class), func(t *testing.T) {
					truth := apiActionResultTruth(t, execution, verification, class)
					got := summarizeAgentActionResult(&truth)
					if !got.DetailsRedacted || got.Execution.Status != execution || got.Verification.Status != verification || got.Verification.EvidenceClass != class || got.Compensation.Support != truth.Compensation.Support || got.Compensation.Status != truth.Compensation.Status {
						t.Fatalf("truth axes changed: %#v", got)
					}
				})
			}
		}
	}
	truth := agentDisclosureRecord(t, false).Result.ActionResultV2
	got := summarizeAgentActionResult(truth)
	if got.Compensation.Status != unified.ActionCompensationSucceeded || got.Compensation.Execution.Status != unified.ActionExecutionSucceeded || got.Compensation.Verification.Status != unified.ActionVerificationConfirmed || got.Compensation.Verification.EvidenceClass != unified.ActionEvidenceIndependent {
		t.Fatalf("compensation truth collapsed into primary failure: %#v", got)
	}
	if summarizeAgentActionResult(nil) != nil {
		t.Fatal("absent canonical result became a fabricated outcome")
	}
	truth.Verification.Evidence[0].Digest = "sha256:" + strings.Repeat("0", 64)
	got = summarizeAgentActionResult(truth)
	if got.Execution.Status != unified.ActionExecutionInconclusive || got.Verification.Status != unified.ActionVerificationInconclusive || got.Verification.EvidenceClass != unified.ActionEvidenceNone {
		t.Fatalf("invalid stored evidence became a successful summary: %#v", got)
	}
}

func TestAgentActionDisclosureErrorAndNoteWithoutCommand(t *testing.T) {
	for _, message := range []string{
		agentPrivateDetail, agentPrivateDetail + ": failure", "plan_drift_suffix: " + agentPrivateDetail,
		"plan_drift: " + agentPrivateDetail, "resource_remediation_locked: " + agentPrivateDetail,
		"action_plan_expired: " + agentPrivateDetail, "action_dry_run_only: " + agentPrivateDetail,
	} {
		got := redactAgentEventActionCompletedCommand(AgentEventActionCompletedPayload{
			ErrorMessage: message, Verification: &AgentResourceActionVerification{Ran: true, Note: agentPrivateDetail},
		})
		body, _ := json.Marshal(got)
		if strings.Contains(string(body), agentPrivateDetail) || !got.CommandRedacted || !got.Verification.CommandRedacted || !got.Verification.Ran {
			t.Errorf("error or note bypassed command-free redaction: %s", body)
		}
		if strings.HasPrefix(message, "plan_drift:") && got.ErrorMessage != "plan_drift:" {
			t.Errorf("known refusal prefix changed: %q", got.ErrorMessage)
		}
	}
}

func TestAgentActionDisclosureContextHTTPPreservesStoredEvidence(t *testing.T) {
	h, _ := agentContextFixtureHandlers(t, "vm:101")
	store, err := h.resources.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	record := agentDisclosureRecord(t, false)
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatal(err)
	}
	before, found, err := store.GetActionAudit(record.ID)
	if err != nil || !found {
		t.Fatalf("saved record: %v, %v", found, err)
	}
	// The session read after the scoped read must still have original evidence.
	for _, restricted := range []bool{true, false, true} {
		req := httptest.NewRequest(http.MethodGet, "/api/agent/resource-context/vm:101", nil)
		if restricted {
			attachAPITokenRecord(req, &config.APITokenRecord{Scopes: []string{config.ScopeMonitoringRead}})
		}
		res := httptest.NewRecorder()
		h.HandleResourceContext(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("context HTTP %d: %s", res.Code, res.Body.String())
		}
		assertAgentDisclosureWire(t, res.Body.Bytes(), restricted)
	}
	after, _, err := store.GetActionAudit(record.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("scoped HTTP read rewrote stored canonical evidence", err)
	}
}

func TestAgentActionDisclosureConcurrentSSESubscribers(t *testing.T) {
	b := NewAgentEventBroadcaster()
	readToken := newTokenRecord(t, "fixture-read-token", []string{config.ScopeMonitoringRead}, nil)
	actionToken := newTokenRecord(t, "fixture-action-token", []string{config.ScopeMonitoringRead, config.ScopeAIExecute}, nil)
	cfg := newTestConfigWithTokens(t, readToken, actionToken)
	server := httptest.NewServer(RequireAuth(cfg, RequireScope(config.ScopeMonitoringRead, b.HandleAgentEvents)))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type subscriber struct {
		body       *http.Response
		reader     *bufio.Reader
		restricted bool
	}
	var clients []subscriber
	for _, token := range []string{"fixture-read-token", "fixture-action-token"} {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
		req.Header.Set("X-API-Token", token)
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("authenticated SSE status %d", resp.StatusCode)
		}
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if line == "\n" {
				break
			}
		}
		clients = append(clients, subscriber{resp, reader, token == "fixture-read-token"})
	}
	if b.SubscriberCount() != 2 {
		t.Fatal("both streams must subscribe before publish")
	}
	record := agentDisclosureRecord(t, false)
	b.PublishActionCompletedRecord(record)
	var wg sync.WaitGroup
	for _, client := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				line, err := client.reader.ReadString('\n')
				if err != nil {
					t.Error(err)
					return
				}
				if strings.HasPrefix(line, "data: ") {
					assertAgentDisclosureWire(t, []byte(line), client.restricted)
					return
				}
			}
		}()
	}
	wg.Wait()
}
