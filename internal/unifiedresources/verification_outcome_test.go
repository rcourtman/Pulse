package unifiedresources

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNormalizeVerificationOutcomeDefaultsToUnknown(t *testing.T) {
	out := NormalizeVerificationOutcome(VerificationOutcome{})
	if out.Status != VerificationUnknown {
		t.Fatalf("empty status normalized to %q, want %q", out.Status, VerificationUnknown)
	}
	out = NormalizeVerificationOutcome(VerificationOutcome{Status: VerificationStatus("garbage"), EvidenceSummary: "  trimmed me  "})
	if out.Status != VerificationUnknown {
		t.Fatalf("invalid status not coerced to unknown: %q", out.Status)
	}
	if out.EvidenceSummary != "trimmed me" {
		t.Fatalf("evidence summary not trimmed: %q", out.EvidenceSummary)
	}
}

func TestNormalizeVerificationOutcomeAcceptsClosedEnum(t *testing.T) {
	for _, status := range []VerificationStatus{VerificationUnknown, VerificationVerified, VerificationUnverified, VerificationFailed} {
		out := NormalizeVerificationOutcome(VerificationOutcome{Status: status, EvidenceSummary: "ok"})
		if out.Status != status {
			t.Fatalf("status %q normalized to %q", status, out.Status)
		}
	}
}

func TestNormalizeActionAuditRecordDefaultsVerificationOutcomeToUnknown(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	record, err := NormalizeActionAuditRecord(ActionAuditRecord{
		ID:        "act_verify",
		CreatedAt: now,
		State:     ActionStateExecuting,
		Request: ActionRequest{
			ResourceID:     "vm:42",
			CapabilityName: "qm.start",
			RequestedBy:    "agent:test",
		},
	})
	if err != nil {
		t.Fatalf("NormalizeActionAuditRecord: %v", err)
	}
	if record.VerificationOutcome.Status != VerificationUnknown {
		t.Fatalf("verification status = %q, want %q", record.VerificationOutcome.Status, VerificationUnknown)
	}
}

func TestVerificationOutcomeFromExecutionResult(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result *ExecutionResult
		want   VerificationStatus
	}{
		{name: "no result", result: nil, want: VerificationUnknown},
		{name: "execution failed", result: &ExecutionResult{Success: false}, want: VerificationUnknown},
		{name: "no verifier", result: &ExecutionResult{Success: true}, want: VerificationUnknown},
		{name: "inconclusive", result: &ExecutionResult{Success: true, Verification: &ActionVerificationResult{Ran: false, Note: "agent unreachable"}}, want: VerificationUnverified},
		{name: "verified", result: &ExecutionResult{Success: true, Verification: &ActionVerificationResult{Ran: true, Success: true, Note: "healthy"}}, want: VerificationVerified},
		{name: "postcondition failed", result: &ExecutionResult{Success: true, Verification: &ActionVerificationResult{Ran: true, Success: false, Note: "still degraded"}}, want: VerificationFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := VerificationOutcomeFromExecutionResult(tc.result); got.Status != tc.want {
				t.Fatalf("status = %q, want %q", got.Status, tc.want)
			}
		})
	}
}

// TestActionAuditRecordJSONRoundtripWithMissingVerificationOutcome simulates an
// older record persisted before the verifier substrate was added: the JSON
// blob omits the verificationOutcome field. After unmarshal + normalize, the
// status must read as unknown so the closed enum holds for older audits.
func TestActionAuditRecordJSONRoundtripWithMissingVerificationOutcome(t *testing.T) {
	raw := []byte(`{
		"id": "act_legacy",
		"createdAt": "2026-04-01T00:00:00Z",
		"updatedAt": "2026-04-01T00:00:00Z",
		"state": "completed",
		"request": {
			"requestId": "req-legacy",
			"resourceId": "vm:42",
			"capabilityName": "restart",
			"requestedBy": "agent:legacy"
		},
		"plan": {
			"actionId": "act_legacy",
			"requestId": "req-legacy",
			"allowed": true,
			"plannedAt": "2026-04-01T00:00:00Z",
			"expiresAt": "2026-04-01T00:05:00Z"
		}
	}`)
	var record ActionAuditRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatalf("unmarshal legacy record: %v", err)
	}
	normalized, err := NormalizeActionAuditRecord(record)
	if err != nil {
		t.Fatalf("normalize legacy record: %v", err)
	}
	if normalized.VerificationOutcome.Status != VerificationUnknown {
		t.Fatalf("legacy record verification status = %q, want %q", normalized.VerificationOutcome.Status, VerificationUnknown)
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		t.Fatalf("marshal normalized record: %v", err)
	}
	if !strings.Contains(string(encoded), `"verificationOutcome":{"status":"unknown"}`) {
		t.Fatalf("verificationOutcome should serialize with the default status; got %s", encoded)
	}
}

func TestActionAuditRecordRedactionPreservesVerificationOutcome(t *testing.T) {
	rec := ActionAuditRecord{
		ID:        "act_redact",
		CreatedAt: time.Now().UTC(),
		State:     ActionStateCompleted,
		Request: ActionRequest{
			RequestID:      "req-redact",
			ResourceID:     "vm:42",
			CapabilityName: "qm.start",
			RequestedBy:    "agent:test",
			Reason:         "restart for upgrade",
		},
		Plan: ActionPlan{
			ActionID:  "act_redact",
			RequestID: "req-redact",
			Allowed:   true,
			PlannedAt: time.Now().UTC(),
		},
		VerificationOutcome: VerificationOutcome{
			Status:          VerificationVerified,
			EvidenceSummary: "vm:42 status=running observed in 8s",
		},
	}
	out := RedactAuditRecord(rec)
	if out.VerificationOutcome.Status != VerificationVerified {
		t.Fatalf("redaction dropped status: %#v", out.VerificationOutcome)
	}
	if out.VerificationOutcome.EvidenceSummary == "" {
		t.Fatalf("redaction nuked evidence summary even when no secret present")
	}
}
