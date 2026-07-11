package unifiedresources

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func actionResultTestEvidence(class ActionEvidenceClass) []ActionEvidence {
	if class == ActionEvidenceNone {
		return nil
	}
	observerDomain := "agent:node-1"
	if class == ActionEvidenceIndependent {
		observerDomain = "provider:proxmox-api"
	}
	now := time.Date(2026, 7, 11, 15, 0, 0, 0, time.UTC)
	return []ActionEvidence{{
		Version: ActionEvidenceVersion, ID: "evidence-1", ObserverID: "observer-1",
		ObserverKind:        "agent",
		ObserverTrustDomain: observerDomain, ExecutorTrustDomain: "agent:node-1",
		Method: "resource_status", SubjectID: "vm:42", ObservedAt: now, ReceivedAt: now.Add(time.Second),
		Summary: "resource reached expected state",
	}}
}

func actionResultTestCompensation() ActionCompensationTruth {
	return ActionCompensationTruth{Support: ActionCompensationUnavailable, Status: ActionCompensationNotAvailable}
}

func actionResultTestTime(value time.Time) *time.Time {
	value = value.UTC()
	return &value
}

func TestNormalizeActionResultTruthMatrix48(t *testing.T) {
	executions := []ActionExecutionStatus{ActionExecutionNotRun, ActionExecutionSucceeded, ActionExecutionFailed, ActionExecutionInconclusive}
	verifications := []ActionVerificationStatus{ActionVerificationNotAttempted, ActionVerificationConfirmed, ActionVerificationContradicted, ActionVerificationInconclusive}
	classes := []ActionEvidenceClass{ActionEvidenceNone, ActionEvidenceAgentAttested, ActionEvidenceIndependent}
	cases := 0
	for _, execution := range executions {
		for _, verification := range verifications {
			for _, class := range classes {
				cases++
				name := fmt.Sprintf("%s/%s/%s", execution, verification, class)
				t.Run(name, func(t *testing.T) {
					result := ActionResultV2{
						Version:      ActionResultV2Version,
						Execution:    ActionExecutionTruth{Status: execution},
						Verification: ActionVerificationTruth{Status: verification, EvidenceClass: class, Evidence: actionResultTestEvidence(class)},
						Compensation: actionResultTestCompensation(),
					}
					if execution != ActionExecutionSucceeded {
						result.Execution.ReasonCode = "execution_reason"
					}
					if verification == ActionVerificationInconclusive {
						result.Verification.ReasonCode = "verification_reason"
					}
					valid := false
					switch verification {
					case ActionVerificationNotAttempted:
						valid = class == ActionEvidenceNone
					case ActionVerificationConfirmed, ActionVerificationContradicted:
						valid = class == ActionEvidenceAgentAttested || class == ActionEvidenceIndependent
					case ActionVerificationInconclusive:
						valid = true
					}
					normalized, err := NormalizeActionResultV2(result)
					if valid && err != nil {
						t.Fatalf("NormalizeActionResultV2: %v", err)
					}
					if !valid && err == nil {
						t.Fatalf("invalid combination normalized: %#v", normalized)
					}
					if valid && (normalized.Execution.Status != execution || normalized.Verification.Status != verification || normalized.Verification.EvidenceClass != class) {
						t.Fatalf("truth collapsed: %#v", normalized)
					}
				})
			}
		}
	}
	if cases != 48 {
		t.Fatalf("matrix cases=%d, want 48", cases)
	}
}

func TestValidateActionResultEvidenceClassMatrix(t *testing.T) {
	base := ActionResultV2{
		Version:      ActionResultV2Version,
		Execution:    ActionExecutionTruth{Status: ActionExecutionSucceeded},
		Verification: ActionVerificationTruth{Status: ActionVerificationConfirmed, EvidenceClass: ActionEvidenceIndependent, Evidence: actionResultTestEvidence(ActionEvidenceIndependent)},
		Compensation: actionResultTestCompensation(),
	}
	if _, err := NormalizeActionResultV2(base); err != nil {
		t.Fatalf("valid independent evidence: %v", err)
	}
	sameDomain := base
	sameDomain.Verification.Evidence = actionResultTestEvidence(ActionEvidenceIndependent)
	sameDomain.Verification.Evidence[0].ObserverTrustDomain = sameDomain.Verification.Evidence[0].ExecutorTrustDomain
	if _, err := NormalizeActionResultV2(sameDomain); !errors.Is(err, ErrFalseIndependentEvidence) {
		t.Fatalf("same trust domain error=%v", err)
	}
	missing := base
	missing.Verification.Evidence = nil
	if _, err := NormalizeActionResultV2(missing); err == nil {
		t.Fatal("independent verification without evidence accepted")
	}
}

func TestActionResultLegacyProjectionMatrix48(t *testing.T) {
	executions := []ActionExecutionStatus{ActionExecutionNotRun, ActionExecutionSucceeded, ActionExecutionFailed, ActionExecutionInconclusive}
	verifications := []ActionVerificationStatus{ActionVerificationNotAttempted, ActionVerificationConfirmed, ActionVerificationContradicted, ActionVerificationInconclusive}
	classes := []ActionEvidenceClass{ActionEvidenceNone, ActionEvidenceAgentAttested, ActionEvidenceIndependent}
	for _, execution := range executions {
		for _, verification := range verifications {
			for _, class := range classes {
				result := ActionResultV2{Version: ActionResultV2Version, Execution: ActionExecutionTruth{Status: execution}, Verification: ActionVerificationTruth{Status: verification, EvidenceClass: class, Evidence: actionResultTestEvidence(class)}, Compensation: actionResultTestCompensation()}
				if execution != ActionExecutionSucceeded {
					result.Execution.ReasonCode = "execution_reason"
				}
				if verification == ActionVerificationInconclusive {
					result.Verification.ReasonCode = "verification_reason"
				}
				if _, err := NormalizeActionResultV2(result); err != nil {
					continue
				}
				legacy, outcome, err := ApplyActionResultV2(nil, result)
				if err != nil {
					t.Fatal(err)
				}
				if legacy.Success != (execution == ActionExecutionSucceeded) {
					t.Fatalf("execution %q legacy success=%v", execution, legacy.Success)
				}
				wantOutcome := map[ActionVerificationStatus]VerificationStatus{
					ActionVerificationNotAttempted: VerificationUnknown,
					ActionVerificationConfirmed:    VerificationVerified,
					ActionVerificationContradicted: VerificationFailed,
					ActionVerificationInconclusive: VerificationUnverified,
				}[verification]
				if outcome.Status != wantOutcome {
					t.Fatalf("verification %q legacy outcome=%q want=%q", verification, outcome.Status, wantOutcome)
				}
			}
		}
	}
}

func TestCompleteActionExecutionRequiresExplicitResult(t *testing.T) {
	record := atomicLifecycleTestRecord("act-nil-result", ActionStateExecuting)
	completed, _, err := CompleteActionExecution(record, nil, "operator", record.CreatedAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	truth := CanonicalActionResultV2(completed)
	if completed.State != ActionStateFailed || completed.Result == nil || completed.Result.Success || truth.Execution.Status != ActionExecutionInconclusive || truth.Execution.ReasonCode != "executor_nil_result" {
		t.Fatalf("nil result truth=%#v record=%#v", truth, completed)
	}
}

func TestKnownPreDispatchRefusalIsNotRunAndNotVerified(t *testing.T) {
	record := atomicLifecycleTestRecord("action-refused", ActionStateExecuting)
	refused, _, err := RefuseActionExecution(record, ErrActionPlanDrift, "operator", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	truth := CanonicalActionResultV2(refused)
	if truth.Execution.Status != ActionExecutionNotRun || truth.Verification.Status != ActionVerificationNotAttempted {
		t.Fatalf("refusal truth=%#v", truth)
	}
}

func TestLegacyDockerAndProxmoxReadbackFailureMigratesSucceededContradicted(t *testing.T) {
	legacy := &ExecutionResult{Success: false, ErrorMessage: "container lifecycle action completed but verification did not confirm the expected state", Verification: &ActionVerificationResult{Ran: true, Success: false, RanAt: time.Now().UTC(), Note: "expected running=true"}}
	truth := ActionResultV2FromLegacy(legacy, VerificationOutcome{Status: VerificationUnknown}, false, "app-container:42", time.Now().UTC())
	if truth.Execution.Status != ActionExecutionSucceeded || truth.Verification.Status != ActionVerificationContradicted || truth.Verification.EvidenceClass != ActionEvidenceAgentAttested {
		t.Fatalf("legacy contradiction=%#v", truth)
	}
}

func TestLegacyVerifiedDefaultsToAgentAttested(t *testing.T) {
	truth := ActionResultV2FromLegacy(&ExecutionResult{Success: true}, VerificationOutcome{Status: VerificationVerified, EvidenceSummary: "healthy"}, false, "vm:42", time.Now().UTC())
	if truth.Verification.Status != ActionVerificationConfirmed || truth.Verification.EvidenceClass != ActionEvidenceAgentAttested {
		t.Fatalf("legacy verified=%#v", truth.Verification)
	}
}

func TestLegacyTimeoutAndDisconnectMigrateInconclusive(t *testing.T) {
	for _, message := range []string{"context deadline exceeded", "transport timed out after dispatch", "agent disconnected before receipt", "connection reset by peer"} {
		t.Run(message, func(t *testing.T) {
			truth := ActionResultV2FromLegacy(&ExecutionResult{Success: false, ErrorMessage: message}, VerificationOutcome{Status: VerificationFailed}, false, "vm:42", time.Now().UTC())
			if truth.Execution.Status != ActionExecutionInconclusive || truth.Execution.ReasonCode != "legacy_unknown_effect" {
				t.Fatalf("legacy unknown effect=%#v", truth)
			}
		})
	}
}

func TestMalformedStoredActionResultV2FailsClosed(t *testing.T) {
	valid := ActionResultV2{
		Version: ActionResultV2Version, Execution: ActionExecutionTruth{Status: ActionExecutionSucceeded},
		Verification: ActionVerificationTruth{Status: ActionVerificationConfirmed, EvidenceClass: ActionEvidenceIndependent, Evidence: actionResultTestEvidence(ActionEvidenceIndependent)},
		Compensation: actionResultTestCompensation(),
	}
	tests := []struct {
		name   string
		mutate func(*ActionResultV2)
	}{
		{"false_independence", func(result *ActionResultV2) {
			result.Verification.Evidence[0].ObserverTrustDomain = result.Verification.Evidence[0].ExecutorTrustDomain
		}},
		{"bad_digest", func(result *ActionResultV2) {
			result.Verification.Evidence[0].Digest = "sha256:" + strings.Repeat("0", 64)
		}},
		{"invalid_execution", func(result *ActionResultV2) { result.Execution.Status = "successful" }},
		{"invalid_version", func(result *ActionResultV2) { result.Version = 99 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			malicious := cloneActionResultV2(valid)
			test.mutate(&malicious)
			record := ActionAuditRecord{
				ID: "malicious", State: ActionStateCompleted,
				Result:              &ExecutionResult{Success: true, ActionResultV2: &malicious, Verification: &ActionVerificationResult{Ran: true, Success: true}},
				VerificationOutcome: VerificationOutcome{Status: VerificationVerified},
			}
			truth := CanonicalActionResultV2(record)
			legacy := LegacyActionResultProjection(record)
			if truth.Execution.Status != ActionExecutionInconclusive || truth.Execution.ReasonCode != "result_v2_invalid" || truth.Verification.Status != ActionVerificationInconclusive || legacy.Success || legacy.Verification == nil || legacy.Verification.Ran || legacy.Verification.Success {
				t.Fatalf("malformed stored V2 failed open: truth=%#v legacy=%#v", truth, legacy)
			}
		})
	}
}

func TestInconclusiveEvidenceReasonSurvivesNormalization(t *testing.T) {
	verification := &ActionVerificationResult{Ran: false, Note: "agent disconnected after dispatch"}
	legacy := NormalizeActionVerificationResult(verification)
	if legacy == nil || legacy.Note != "" {
		t.Fatalf("legacy unrun verification was not scrubbed: %#v", legacy)
	}
	truth := ActionResultV2FromLegacy(&ExecutionResult{Success: true, Verification: verification}, VerificationOutcome{}, false, "vm:42", time.Now().UTC())
	if truth.Verification.Status != ActionVerificationInconclusive || truth.Verification.Summary != "agent disconnected after dispatch" {
		t.Fatalf("inconclusive truth=%#v", truth.Verification)
	}
}

func TestActionEvidenceCanonicalDigestStable(t *testing.T) {
	evidence := actionResultTestEvidence(ActionEvidenceIndependent)[0]
	first, err := NormalizeActionEvidence(evidence)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NormalizeActionEvidence(first)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest || !strings.HasPrefix(first.Digest, "sha256:") {
		t.Fatalf("unstable digest first=%q second=%q", first.Digest, second.Digest)
	}
}

func TestActionEvidenceCanonicalDigestChangesWithRetainedSemanticFields(t *testing.T) {
	evidence := actionResultTestEvidence(ActionEvidenceIndependent)[0]
	first, err := NormalizeActionEvidence(evidence)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Method = "provider_generation"
	second, err := NormalizeActionEvidence(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest == second.Digest {
		t.Fatalf("digest did not bind retained semantic method: %q", first.Digest)
	}
}

func TestActionEvidenceRedactionPreservesTruthAndCanonicalDigest(t *testing.T) {
	evidence := actionResultTestEvidence(ActionEvidenceAgentAttested)[0]
	evidence.Summary = "Authorization: Bearer secret-token"
	normalized, err := NormalizeActionEvidence(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(normalized.Summary, "secret-token") || normalized.Digest == "" {
		t.Fatalf("evidence not safely canonicalized: %#v", normalized)
	}
}

func TestActionEvidenceRejectsUnboundedPayload(t *testing.T) {
	evidence := actionResultTestEvidence(ActionEvidenceAgentAttested)[0]
	evidence.Summary = strings.Repeat("x", ActionEvidenceMaxTextBytes+1)
	if _, err := NormalizeActionEvidence(evidence); !errors.Is(err, ErrActionEvidenceBounds) {
		t.Fatalf("unbounded evidence error=%v", err)
	}
}

func TestActionEvidenceBoundsApplyBeforeSecretRedaction(t *testing.T) {
	oversizedSecret := "Authorization: Bearer " + strings.Repeat("x", ActionEvidenceMaxTextBytes+1)
	tests := []struct {
		name   string
		mutate func(*ActionEvidence)
	}{
		{name: "summary", mutate: func(e *ActionEvidence) { e.Summary = oversizedSecret }},
		{name: "reason", mutate: func(e *ActionEvidence) { e.ReasonCode = "token=" + strings.Repeat("x", ActionEvidenceMaxReasonBytes+1) }},
		{name: "ref_id", mutate: func(e *ActionEvidence) {
			e.Refs = []ActionEvidenceRef{{ID: oversizedSecret, Kind: "audit", Digest: "sha256:" + strings.Repeat("a", 64)}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence := actionResultTestEvidence(ActionEvidenceAgentAttested)[0]
			test.mutate(&evidence)
			if _, err := NormalizeActionEvidence(evidence); !errors.Is(err, ErrActionEvidenceBounds) {
				t.Fatalf("pre-redaction bound error=%v", err)
			}
		})
	}
}

func TestActionResultRedactionDeepCopiesAndRecomputesDigest(t *testing.T) {
	evidence := actionResultTestEvidence(ActionEvidenceAgentAttested)[0]
	evidence.Summary = "Authorization: Bearer secret-token"
	evidence.Digest = "not-a-valid-digest"
	result := &ExecutionResult{ActionResultV2: &ActionResultV2{
		Version:      ActionResultV2Version,
		Execution:    ActionExecutionTruth{Status: ActionExecutionSucceeded, Summary: "token=execution-secret"},
		Verification: ActionVerificationTruth{Status: ActionVerificationConfirmed, EvidenceClass: ActionEvidenceAgentAttested, Evidence: []ActionEvidence{evidence}},
		Compensation: actionResultTestCompensation(),
	}}
	redacted := redactActionExecutionResult(result)
	got := redacted.ActionResultV2.Verification.Evidence[0]
	if strings.Contains(got.Summary, "secret-token") || !validSHA256Digest(got.Digest) {
		t.Fatalf("redacted evidence=%#v", got)
	}
	if result.ActionResultV2.Verification.Evidence[0].Summary != "Authorization: Bearer secret-token" || result.ActionResultV2.Verification.Evidence[0].Digest != "not-a-valid-digest" {
		t.Fatalf("redaction mutated input: %#v", result.ActionResultV2.Verification.Evidence[0])
	}
}

func TestActionResultRedactionFailsClosedOnMalformedEvidence(t *testing.T) {
	evidence := actionResultTestEvidence(ActionEvidenceAgentAttested)[0]
	evidence.Summary = "Authorization: Bearer must-not-leak"
	evidence.Refs = []ActionEvidenceRef{{ID: "token=ref-secret", Kind: "audit", Digest: "malformed"}}
	result := &ExecutionResult{ActionResultV2: &ActionResultV2{
		Version:      ActionResultV2Version,
		Execution:    ActionExecutionTruth{Status: ActionExecutionSucceeded, Summary: "token=execution-secret"},
		Verification: ActionVerificationTruth{Status: ActionVerificationConfirmed, EvidenceClass: ActionEvidenceAgentAttested, Evidence: []ActionEvidence{evidence}},
		Compensation: actionResultTestCompensation(),
	}}
	redacted := redactActionExecutionResult(result)
	truth := redacted.ActionResultV2
	if truth.Execution.Status != ActionExecutionInconclusive || truth.Execution.ReasonCode != "redaction_contract_violation" || len(truth.Verification.Evidence) != 0 {
		t.Fatalf("malformed evidence did not fail closed: %#v", truth)
	}
	payload := fmt.Sprintf("%#v", redacted)
	if strings.Contains(payload, "must-not-leak") || strings.Contains(payload, "execution-secret") || strings.Contains(payload, "ref-secret") {
		t.Fatalf("redaction fallback leaked original content: %s", payload)
	}
	if result.ActionResultV2.Verification.Evidence[0].Refs[0].Digest != "malformed" {
		t.Fatalf("redaction mutated original ref graph: %#v", result.ActionResultV2.Verification.Evidence[0].Refs)
	}
}

func TestCompensationNeverRewritesPrimaryResult(t *testing.T) {
	failedCompensation := ActionCompensationTruth{
		Support: ActionCompensationDeclared, Strategy: "restore_snapshot", Trigger: "verification_contradicted", Status: ActionCompensationFailed, ReasonCode: "restore_failed",
		AttemptID: "comp-1", StepID: "restore", StartedAt: actionResultTestTime(time.Now().UTC().Add(-time.Minute)), CompletedAt: actionResultTestTime(time.Now().UTC()),
		Execution: &ActionExecutionTruth{Status: ActionExecutionFailed, ReasonCode: "provider_failed"},
	}
	result := ActionResultV2{Version: ActionResultV2Version, Execution: ActionExecutionTruth{Status: ActionExecutionSucceeded}, Verification: ActionVerificationTruth{Status: ActionVerificationContradicted, EvidenceClass: ActionEvidenceAgentAttested, Evidence: actionResultTestEvidence(ActionEvidenceAgentAttested)}, Compensation: failedCompensation}
	normalized, err := NormalizeActionResultV2(result)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Execution.Status != ActionExecutionSucceeded || normalized.Verification.Status != ActionVerificationContradicted || normalized.Compensation.Status != ActionCompensationFailed {
		t.Fatalf("truth axes collapsed: %#v", normalized)
	}
}

func TestCompensationSucceededRequiresRestorationVerification(t *testing.T) {
	now := time.Now().UTC()
	result := ActionResultV2{Version: ActionResultV2Version, Execution: ActionExecutionTruth{Status: ActionExecutionFailed, ReasonCode: "update_failed"}, Verification: ActionVerificationTruth{Status: ActionVerificationContradicted, EvidenceClass: ActionEvidenceAgentAttested, Evidence: actionResultTestEvidence(ActionEvidenceAgentAttested)}, Compensation: ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "restore_backup", Trigger: "verification_contradicted", Status: ActionCompensationSucceeded, AttemptID: "comp-1", StepID: "restore", StartedAt: actionResultTestTime(now.Add(-time.Minute)), CompletedAt: actionResultTestTime(now), Execution: &ActionExecutionTruth{Status: ActionExecutionSucceeded}}}
	if _, err := NormalizeActionResultV2(result); err == nil {
		t.Fatal("compensation success without restoration verification accepted")
	}
	verification := ActionVerificationTruth{Status: ActionVerificationConfirmed, EvidenceClass: ActionEvidenceIndependent, Evidence: actionResultTestEvidence(ActionEvidenceIndependent)}
	result.Compensation.Verification = &verification
	digest := "sha256:" + strings.Repeat("a", 64)
	result.Compensation.RestoredState = &ActionRestoredStateTruth{SubjectID: "vm:42", ExpectedDigest: digest, ObservedDigest: digest, ObservedAt: now}
	if _, err := NormalizeActionResultV2(result); err != nil {
		t.Fatalf("verified compensation rejected: %v", err)
	}
}

func TestCompensationStateShapeValidationMatrix(t *testing.T) {
	now := time.Now().UTC()
	digest := "sha256:" + strings.Repeat("b", 64)
	confirmed := ActionVerificationTruth{Status: ActionVerificationConfirmed, EvidenceClass: ActionEvidenceIndependent, Evidence: actionResultTestEvidence(ActionEvidenceIndependent)}
	tests := []struct {
		name  string
		truth ActionCompensationTruth
		valid bool
	}{
		{"unavailable_empty", ActionCompensationTruth{Support: ActionCompensationUnavailable, Status: ActionCompensationNotAvailable}, true},
		{"unavailable_with_strategy", ActionCompensationTruth{Support: ActionCompensationUnavailable, Status: ActionCompensationNotAvailable, Strategy: "restore"}, false},
		{"not_needed_empty", ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "restore", Trigger: "no_change", Status: ActionCompensationNotNeeded}, true},
		{"not_needed_with_attempt", ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "restore", Trigger: "no_change", Status: ActionCompensationNotNeeded, AttemptID: "comp-1"}, false},
		{"not_attempted_with_evidence", ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "restore", Trigger: "manual", Status: ActionCompensationNotAttempted, Evidence: actionResultTestEvidence(ActionEvidenceAgentAttested)}, false},
		{"not_attempted_with_execution", ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "restore", Trigger: "manual", Status: ActionCompensationNotAttempted, Execution: &ActionExecutionTruth{Status: ActionExecutionSucceeded}}, false},
		{"running", ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "restore", Trigger: "contradicted", Status: ActionCompensationRunning, AttemptID: "comp-1", StepID: "restore", StartedAt: actionResultTestTime(now)}, true},
		{"running_with_terminal_axis", ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "restore", Trigger: "contradicted", Status: ActionCompensationRunning, AttemptID: "comp-1", StepID: "restore", StartedAt: actionResultTestTime(now), Execution: &ActionExecutionTruth{Status: ActionExecutionSucceeded}}, false},
		{"failed_without_axis", ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "restore", Trigger: "contradicted", Status: ActionCompensationFailed, ReasonCode: "failed", AttemptID: "comp-1", StepID: "restore", StartedAt: actionResultTestTime(now), CompletedAt: actionResultTestTime(now.Add(time.Second))}, false},
		{"inconclusive_with_success_axis", ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "restore", Trigger: "timeout", Status: ActionCompensationInconclusive, ReasonCode: "unknown", AttemptID: "comp-1", StepID: "restore", StartedAt: actionResultTestTime(now), CompletedAt: actionResultTestTime(now.Add(time.Second)), Execution: &ActionExecutionTruth{Status: ActionExecutionSucceeded}}, false},
		{"succeeded_without_restored_state", ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "restore", Trigger: "contradicted", Status: ActionCompensationSucceeded, AttemptID: "comp-1", StepID: "restore", StartedAt: actionResultTestTime(now), CompletedAt: actionResultTestTime(now.Add(time.Second)), Execution: &ActionExecutionTruth{Status: ActionExecutionSucceeded}, Verification: &confirmed}, false},
		{"succeeded", ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "restore", Trigger: "contradicted", Status: ActionCompensationSucceeded, AttemptID: "comp-1", StepID: "restore", StartedAt: actionResultTestTime(now), CompletedAt: actionResultTestTime(now.Add(time.Second)), Execution: &ActionExecutionTruth{Status: ActionExecutionSucceeded}, Verification: &confirmed, RestoredState: &ActionRestoredStateTruth{SubjectID: "vm:42", ExpectedDigest: digest, ObservedDigest: digest, ObservedAt: now.Add(time.Second)}}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ActionResultV2{Version: ActionResultV2Version, Execution: ActionExecutionTruth{Status: ActionExecutionSucceeded}, Verification: ActionVerificationTruth{Status: ActionVerificationNotAttempted, EvidenceClass: ActionEvidenceNone}, Compensation: test.truth}
			_, err := NormalizeActionResultV2(result)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v err=%v truth=%#v", test.valid, err, test.truth)
			}
		})
	}
}
