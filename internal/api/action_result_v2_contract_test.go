package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

func apiActionResultTruth(t *testing.T, execution unified.ActionExecutionStatus, verification unified.ActionVerificationStatus, class unified.ActionEvidenceClass) unified.ActionResultV2 {
	t.Helper()
	now := time.Date(2026, 7, 11, 16, 0, 0, 0, time.UTC)
	truth := unified.ActionResultV2{
		Version:   unified.ActionResultV2Version,
		Execution: unified.ActionExecutionTruth{Status: execution},
		Verification: unified.ActionVerificationTruth{
			Status: verification, EvidenceClass: class,
		},
		Compensation: unified.ActionCompensationTruth{Support: unified.ActionCompensationUnavailable, Status: unified.ActionCompensationNotAvailable},
	}
	if execution != unified.ActionExecutionSucceeded {
		truth.Execution.ReasonCode = "execution_reason"
	}
	if verification == unified.ActionVerificationInconclusive {
		truth.Verification.ReasonCode = "observer_timeout"
	}
	if class != unified.ActionEvidenceNone {
		observerDomain := "agent:node-1"
		observerKind := "agent"
		if class == unified.ActionEvidenceIndependent {
			observerDomain = "provider:api"
			observerKind = "provider_api"
		}
		truth.Verification.Evidence = []unified.ActionEvidence{{
			Version: unified.ActionEvidenceVersion, ID: "provider-read-1", ObserverID: "provider-api",
			ObserverKind: observerKind, ObserverTrustDomain: observerDomain, ExecutorTrustDomain: "agent:node-1",
			Method: "generation_read", SubjectID: "vm:42", ObservedAt: now, ReceivedAt: now.Add(time.Second),
		}}
	}
	normalized, err := unified.NormalizeActionResultV2(truth)
	if err != nil {
		t.Fatal(err)
	}
	return normalized
}

func TestActionResultV2EventFindingContextTelemetryProjectionMatrix(t *testing.T) {
	executions := []unified.ActionExecutionStatus{unified.ActionExecutionNotRun, unified.ActionExecutionSucceeded, unified.ActionExecutionFailed, unified.ActionExecutionInconclusive}
	verificationClasses := map[unified.ActionVerificationStatus][]unified.ActionEvidenceClass{
		unified.ActionVerificationNotAttempted: {unified.ActionEvidenceNone},
		unified.ActionVerificationConfirmed:    {unified.ActionEvidenceAgentAttested, unified.ActionEvidenceIndependent},
		unified.ActionVerificationContradicted: {unified.ActionEvidenceAgentAttested, unified.ActionEvidenceIndependent},
		unified.ActionVerificationInconclusive: {unified.ActionEvidenceNone, unified.ActionEvidenceAgentAttested, unified.ActionEvidenceIndependent},
	}
	cases := 0
	for _, execution := range executions {
		for verification, classes := range verificationClasses {
			for _, class := range classes {
				cases++
				t.Run(fmt.Sprintf("%s/%s/%s", execution, verification, class), func(t *testing.T) {
					truth := apiActionResultTruth(t, execution, verification, class)
					state := unified.ActionStateFailed
					if execution == unified.ActionExecutionSucceeded {
						state = unified.ActionStateCompleted
					}
					record := unified.ActionAuditRecord{
						ID: "action-1", State: state, UpdatedAt: time.Now().UTC(),
						Request:   unified.ActionRequest{ResourceID: "vm:42", CapabilityName: "restart"},
						Result:    &unified.ExecutionResult{ActionResultV2: &truth},
						Approvals: []unified.ActionApprovalRecord{{Outcome: unified.OutcomeApproved}},
					}
					event, ok := projectAgentActionCompletedPayload(record)
					if !ok || event.ActionResultV2 == nil || event.ActionResultV2.Execution.Status != execution || event.ActionResultV2.Verification.Status != verification || event.Success != (execution == unified.ActionExecutionSucceeded) {
						t.Fatalf("event=%#v ok=%v", event, ok)
					}
					contexts := projectAgentResourceActions([]unified.ActionAuditRecord{record})
					if len(contexts) != 1 || contexts[0].ActionResultV2 == nil || contexts[0].ActionResultV2.Execution.Status != execution || contexts[0].ActionResultV2.Verification.Status != verification || contexts[0].Success != (execution == unified.ActionExecutionSucceeded) {
						t.Fatalf("contexts=%#v", contexts)
					}
					expectedOutcome := aicontracts.OutcomeFixVerificationUnknown
					if execution == unified.ActionExecutionFailed || execution == unified.ActionExecutionNotRun {
						expectedOutcome = aicontracts.OutcomeFixFailed
					} else if execution == unified.ActionExecutionSucceeded && verification == unified.ActionVerificationConfirmed {
						expectedOutcome = aicontracts.OutcomeFixVerified
					} else if execution == unified.ActionExecutionSucceeded && verification == unified.ActionVerificationContradicted {
						expectedOutcome = aicontracts.OutcomeFixVerificationFailed
					}
					if got := patrolOutcomeForActionAudit(record); got != expectedOutcome {
						t.Fatalf("finding outcome=%q want=%q", got, expectedOutcome)
					}
					if got := pulseIntelligenceActionVerifiedOutcome(record); got != (execution == unified.ActionExecutionSucceeded && verification == unified.ActionVerificationConfirmed) {
						t.Fatalf("telemetry verified=%v", got)
					}
					disposition := dispositionFromRecord(record)
					var dispositionTruth unified.ActionResultV2
					if err := json.Unmarshal(disposition.ActionResultV2, &dispositionTruth); err != nil || dispositionTruth.Execution.Status != execution || dispositionTruth.Verification.Status != verification {
						t.Fatalf("disposition=%#v truth=%#v err=%v", disposition, dispositionTruth, err)
					}
					notification := relay.NewCanonicalActionOutcomeNotification("finding-1", "Action", truth)
					if !strings.Contains(notification.Body, "Execution ") || !strings.Contains(notification.Body, "verification ") {
						t.Fatalf("notification collapsed axes: %#v", notification)
					}
				})
			}
		}
	}
	if cases != 32 {
		t.Fatalf("legal consumer projection cases=%d, want 32", cases)
	}
}

func TestActionResultV2JSONContractSnapshot(t *testing.T) {
	truth := apiActionResultTruth(t, unified.ActionExecutionSucceeded, unified.ActionVerificationConfirmed, unified.ActionEvidenceIndependent)
	payload, err := json.Marshal(AgentEventActionCompletedPayload{ActionID: "action-1", State: "completed", ActionResultV2: &truth})
	if err != nil {
		t.Fatal(err)
	}
	jsonText := string(payload)
	for _, required := range []string{`"actionResultV2":{"version":2`, `"execution":{"status":"succeeded"`, `"verification":{"status":"confirmed","evidenceClass":"independent"`, `"compensation":{"support":"unavailable","status":"not_available"`} {
		if !strings.Contains(jsonText, required) {
			t.Fatalf("JSON contract missing %s: %s", required, jsonText)
		}
	}
}

func TestMalformedStoredActionResultV2CannotProjectSuccessOrVerification(t *testing.T) {
	base := apiActionResultTruth(t, unified.ActionExecutionSucceeded, unified.ActionVerificationConfirmed, unified.ActionEvidenceIndependent)
	tests := []struct {
		name   string
		mutate func(*unified.ActionResultV2)
	}{
		{"false_independence", func(result *unified.ActionResultV2) {
			result.Verification.Evidence[0].ObserverTrustDomain = result.Verification.Evidence[0].ExecutorTrustDomain
		}},
		{"bad_digest", func(result *unified.ActionResultV2) {
			result.Verification.Evidence[0].Digest = "sha256:" + strings.Repeat("0", 64)
		}},
		{"invalid_status", func(result *unified.ActionResultV2) { result.Execution.Status = "successful" }},
		{"invalid_version", func(result *unified.ActionResultV2) { result.Version = 99 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			malicious := base
			malicious.Verification.Evidence = append([]unified.ActionEvidence(nil), base.Verification.Evidence...)
			test.mutate(&malicious)
			record := unified.ActionAuditRecord{
				ID: "malicious", State: unified.ActionStateCompleted, UpdatedAt: time.Now().UTC(),
				Result:              &unified.ExecutionResult{Success: true, ActionResultV2: &malicious, Verification: &unified.ActionVerificationResult{Ran: true, Success: true}},
				VerificationOutcome: unified.VerificationOutcome{Status: unified.VerificationVerified},
				Approvals:           []unified.ActionApprovalRecord{{Outcome: unified.OutcomeApproved}},
			}
			event, ok := projectAgentActionCompletedPayload(record)
			contexts := projectAgentResourceActions([]unified.ActionAuditRecord{record})
			disposition := dispositionFromRecord(record)
			var dispositionTruth unified.ActionResultV2
			unmarshalErr := json.Unmarshal(disposition.ActionResultV2, &dispositionTruth)
			if !ok || event.Success || event.ActionResultV2 == nil || event.ActionResultV2.Execution.Status != unified.ActionExecutionInconclusive || event.ActionResultV2.Verification.Status != unified.ActionVerificationInconclusive {
				t.Fatalf("event failed open: %#v", event)
			}
			if len(contexts) != 1 || contexts[0].Success || contexts[0].ActionResultV2.Execution.Status != unified.ActionExecutionInconclusive || contexts[0].ActionResultV2.Verification.Status != unified.ActionVerificationInconclusive {
				t.Fatalf("context failed open: %#v", contexts)
			}
			if got := patrolOutcomeForActionAudit(record); got != aicontracts.OutcomeFixVerificationUnknown {
				t.Fatalf("finding failed open: %q", got)
			}
			if pulseIntelligenceActionVerifiedOutcome(record) {
				t.Fatal("telemetry projected malformed V2 as verified")
			}
			if unmarshalErr != nil || dispositionTruth.Execution.Status != unified.ActionExecutionInconclusive || dispositionTruth.Verification.Status != unified.ActionVerificationInconclusive || disposition.VerificationStatus == string(unified.VerificationVerified) {
				t.Fatalf("disposition failed open: %#v truth=%#v err=%v", disposition, dispositionTruth, unmarshalErr)
			}
			notification := relay.NewCanonicalActionOutcomeNotification("finding-1", "Action", dispositionTruth)
			if strings.Contains(notification.Body, "succeeded") || strings.Contains(notification.Body, "confirmed") {
				t.Fatalf("push failed open: %#v", notification)
			}
		})
	}
}
