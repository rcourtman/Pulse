package unifiedresources

import (
	"strings"
	"testing"
	"time"
)

func TestRedactAuditText_StripsCommonSecretShapes(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		mustStrip   string
		mustContain string // a marker the redactor inserts
	}{
		{
			name:        "openai-style API key",
			input:       "rotate sk-abc123def456ghi key after leak",
			mustStrip:   "sk-abc123def456ghi",
			mustContain: "[redacted-secret]",
		},
		{
			name:        "Authorization Bearer header",
			input:       "curl -H 'Authorization: Bearer eyJabc.def.ghi' https://api.example.com/v1",
			mustStrip:   "eyJabc.def.ghi",
			mustContain: "[redacted]",
		},
		{
			name:        "x-api-key header",
			input:       "headers: x-api-key: prod-secret-12345 caused 401",
			mustStrip:   "prod-secret-12345",
			mustContain: "[redacted]",
		},
		{
			name:        "URL with embedded credentials",
			input:       "fetched from https://admin:hunter2@db.internal/api/snapshot to verify",
			mustStrip:   "hunter2",
			mustContain: "[redacted-credentials]",
		},
		{
			name:        "query string secret param",
			input:       "called https://api.example.com/v1/items?api_key=verysecret&limit=10",
			mustStrip:   "verysecret",
			mustContain: "[redacted]",
		},
		{
			name:        "JSON-style token field",
			input:       `payload {"name":"app","token":"sup3rs3cret","tier":"pro"}`,
			mustStrip:   "sup3rs3cret",
			mustContain: "[redacted]",
		},
		{
			name:        "env-style password assignment",
			input:       "ran: PASSWORD=hunter2 service start",
			mustStrip:   "hunter2",
			mustContain: "[redacted]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactAuditText(tc.input)
			if strings.Contains(got, tc.mustStrip) {
				t.Fatalf("expected secret %q to be stripped, got: %s", tc.mustStrip, got)
			}
			if !strings.Contains(got, tc.mustContain) {
				t.Fatalf("expected redaction marker %q, got: %s", tc.mustContain, got)
			}
		})
	}
}

func TestRedactAuditText_LeavesPublicURLsAlone(t *testing.T) {
	// Operators legitimately reference runbooks, ticket links, and
	// GitHub issues in audit reasons. The redactor must NOT strip
	// arbitrary URLs — only URLs with embedded credentials.
	input := "see https://runbook.example.com/restart-procedure or ticket https://github.com/org/repo/issues/123"
	got := RedactAuditText(input)
	if got != input {
		t.Fatalf("expected public URLs preserved, got: %s", got)
	}
}

func TestRedactAuditText_EmptyStringPassesThrough(t *testing.T) {
	if got := RedactAuditText(""); got != "" {
		t.Fatalf("expected empty string passthrough, got %q", got)
	}
}

func TestRedactAuditRecord_ScrubsRequestAndResultStringFields(t *testing.T) {
	now := time.Now().UTC()
	record := ActionAuditRecord{
		ID:        "act-1",
		CreatedAt: now,
		UpdatedAt: now,
		State:     ActionStateCompleted,
		Request: ActionRequest{
			RequestID:      "req-1",
			ResourceID:     "vm-100",
			CapabilityName: "pulse_control",
			// Operator-authored reason includes a secret they pasted.
			Reason:      "rotate the key sk-abc123def456 because it leaked into logs",
			RequestedBy: "operator@example.com",
			Params: map[string]any{
				"command":     "curl -H 'Authorization: Bearer eyJsecret' https://api.example.com",
				"targetType":  "agent",
				"approvalId":  "approval-1",
				"requestedBy": "pulse_control",
				"retries":     3, // non-string param value left untouched
			},
		},
		Plan: ActionPlan{
			ActionID:  "act-1",
			RequestID: "req-1",
			PlanHash:  "deadbeef",
		},
		Verification: &ActionVerificationResult{
			Ran:     true,
			Command: "systemctl status nginx token=top-command-secret",
			Output:  "ActiveState=active token=top-output-secret",
			Success: true,
			RanAt:   now.Add(time.Minute),
			Note:    "verified active with password=top-note-secret",
		},
		VerificationOutcome: VerificationOutcome{
			Status:          VerificationVerified,
			EvidenceSummary: "observed active after api_key=evidence-secret",
		},
		Result: actionResultStub(),
	}
	redacted := RedactAuditRecord(record)

	if strings.Contains(redacted.Request.Reason, "sk-abc123def456") {
		t.Fatalf("Reason still contains secret: %s", redacted.Request.Reason)
	}
	if !strings.Contains(redacted.Request.Reason, "[redacted-secret]") {
		t.Fatalf("Reason missing redaction marker: %s", redacted.Request.Reason)
	}
	cmd, _ := redacted.Request.Params["command"].(string)
	if strings.Contains(cmd, "eyJsecret") {
		t.Fatalf("Params[command] still contains Bearer token: %s", cmd)
	}
	if got := redacted.Request.Params["retries"]; got != 3 {
		t.Fatalf("non-string Params value mutated: %#v", got)
	}
	// Plan must be untouched — it is produced by Pulse, not operators.
	if redacted.Plan.PlanHash != "deadbeef" {
		t.Fatalf("Plan hash mutated: %s", redacted.Plan.PlanHash)
	}
	// Result.Output redacted
	if strings.Contains(redacted.Result.Output, "hunter2") {
		t.Fatalf("Result.Output still contains password: %s", redacted.Result.Output)
	}
	if strings.Contains(redacted.Result.ErrorMessage, "leakedkey") {
		t.Fatalf("Result.ErrorMessage still contains API key: %s", redacted.Result.ErrorMessage)
	}
	if redacted.Verification == nil {
		t.Fatal("expected top-level verification to remain present")
	}
	if strings.Contains(redacted.Verification.Command, "top-command-secret") || !strings.Contains(redacted.Verification.Command, "systemctl status nginx") {
		t.Fatalf("top-level verification command redaction failed: %s", redacted.Verification.Command)
	}
	if strings.Contains(redacted.Verification.Output, "top-output-secret") || !strings.Contains(redacted.Verification.Output, "ActiveState=active") {
		t.Fatalf("top-level verification output redaction failed: %s", redacted.Verification.Output)
	}
	if strings.Contains(redacted.Verification.Note, "top-note-secret") || !strings.Contains(redacted.Verification.Note, "verified active") {
		t.Fatalf("top-level verification note redaction failed: %s", redacted.Verification.Note)
	}
	if redacted.Result.Verification == nil {
		t.Fatal("expected result verification to remain present")
	}
	if strings.Contains(redacted.Result.Verification.Command, "nested-command-secret") || !strings.Contains(redacted.Result.Verification.Command, "curl -H") {
		t.Fatalf("nested verification command redaction failed: %s", redacted.Result.Verification.Command)
	}
	if strings.Contains(redacted.Result.Verification.Output, "nested-output-secret") || !strings.Contains(redacted.Result.Verification.Output, "state") {
		t.Fatalf("nested verification output redaction failed: %s", redacted.Result.Verification.Output)
	}
	if strings.Contains(redacted.Result.Verification.Note, "nested-note-secret") || !strings.Contains(redacted.Result.Verification.Note, "nested check") {
		t.Fatalf("nested verification note redaction failed: %s", redacted.Result.Verification.Note)
	}
	if strings.Contains(redacted.VerificationOutcome.EvidenceSummary, "evidence-secret") {
		t.Fatalf("verification evidence summary still contains API key: %s", redacted.VerificationOutcome.EvidenceSummary)
	}
}

func TestRedactAuditRecord_PreservesUnrunVerificationNormalization(t *testing.T) {
	now := time.Now().UTC()
	record := ActionAuditRecord{
		ID:        "act-unrun",
		CreatedAt: now,
		UpdatedAt: now,
		State:     ActionStateFailed,
		Verification: &ActionVerificationResult{
			Ran:     false,
			Command: "should not persist token=top-command-secret",
			Output:  "should not persist token=top-output-secret",
			Success: true,
			RanAt:   now,
			Note:    "should not persist password=top-note-secret",
		},
		Result: &ExecutionResult{
			Success: false,
			Verification: &ActionVerificationResult{
				Ran:     false,
				Command: "should not persist token=nested-command-secret",
				Output:  "should not persist token=nested-output-secret",
				Success: true,
				RanAt:   now,
				Note:    "should not persist password=nested-note-secret",
			},
		},
	}

	redacted := RedactAuditRecord(record)
	if redacted.Verification == nil || redacted.Verification.Ran {
		t.Fatalf("expected normalized top-level ran=false verification, got %#v", redacted.Verification)
	}
	if redacted.Verification.Command != "" || redacted.Verification.Output != "" || redacted.Verification.Note != "" || redacted.Verification.Success || !redacted.Verification.RanAt.IsZero() {
		t.Fatalf("top-level ran=false verification retained details: %#v", redacted.Verification)
	}
	if redacted.Result == nil || redacted.Result.Verification == nil || redacted.Result.Verification.Ran {
		t.Fatalf("expected normalized nested ran=false verification, got %#v", redacted.Result)
	}
	if redacted.Result.Verification.Command != "" || redacted.Result.Verification.Output != "" || redacted.Result.Verification.Note != "" || redacted.Result.Verification.Success || !redacted.Result.Verification.RanAt.IsZero() {
		t.Fatalf("nested ran=false verification retained details: %#v", redacted.Result.Verification)
	}
}

// actionResultStub builds a result fixture with secret-shaped output and
// verification details for the audit-record redaction test.
func actionResultStub() *ExecutionResult {
	now := time.Now().UTC()
	return &ExecutionResult{
		Success:      false,
		Output:       "ran: PASSWORD=hunter2 service start && exit 1",
		ErrorMessage: "auth failed: api_key=leakedkey not accepted",
		Verification: &ActionVerificationResult{
			Ran:     true,
			Command: "curl -H 'Authorization: Bearer nested-command-secret' https://api.example.com/status",
			Output:  `payload {"token":"nested-output-secret","state":"active"}`,
			Success: true,
			RanAt:   now,
			Note:    "nested check completed with api_key=nested-note-secret",
		},
	}
}
