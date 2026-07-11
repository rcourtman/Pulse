package unifiedresources

import (
	"regexp"
	"strings"
)

const (
	auditVerificationCommandRedacted = "[redacted-verification-command]"
	auditVerificationOutputRedacted  = "[redacted-verification-output]"
	auditVerificationNoteRedacted    = "[redacted-verification-note]"
)

// auditSecretRedactors scrubs well-known credential shapes from audit-log
// text fields before persistence. The audit log is plaintext SQL; an
// operator who pastes a secret into a natural-language `reason` field, or a
// command output that echoes a token, must not have that secret persisted
// unredacted.
//
// The set is intentionally narrower than the patrol-failure redactor in
// internal/ai/patrol_runtime_failure.go: this redactor must not strip
// arbitrary URLs, because operators legitimately reference public URLs
// (runbooks, ticket links, GitHub issues) in audit reasons. It targets
// only patterns that are very likely real secrets.
var auditSecretRedactors = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	// URL with embedded basic-auth credentials: https://user:pass@host
	{
		pattern:     regexp.MustCompile(`(?i)(https?://)[^\s/@:]+:[^\s/@]+@`),
		replacement: `${1}[redacted-credentials]@`,
	},
	// Authorization: Bearer <token> headers
	{
		pattern:     regexp.MustCompile(`(?i)((?:authorization:\s*bearer|x-api-key:)\s+)[^\s,;]+`),
		replacement: `${1}[redacted]`,
	},
	// Query-string secret params: ?key=..., &api_key=..., &access_token=...
	{
		pattern:     regexp.MustCompile(`(?i)([?&](?:key|api_key|apikey|access_token|token)=)[^\s&"']+`),
		replacement: `${1}[redacted]`,
	},
	// JSON-style secret fields: "api_key": "...", "token": "...", "password": "..."
	{
		pattern:     regexp.MustCompile(`(?i)("(?:api[_-]?key|apikey|access[_-]?token|token|authorization|x-api-key|password|secret)"\s*:\s*")[^"]+`),
		replacement: `${1}[redacted]`,
	},
	// Unquoted env-style or CLI-style secret assignments: PASSWORD=..., api_key=...
	{
		pattern:     regexp.MustCompile(`(?i)\b((?:password|passwd|api[_-]?key|apikey|access[_-]?token|token|secret)\s*=\s*)\S+`),
		replacement: `${1}[redacted]`,
	},
	// OpenAI/Anthropic-style API keys
	{
		pattern:     regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}\b`),
		replacement: `[redacted-secret]`,
	},
}

// RedactAuditText returns the input with well-known credential shapes
// replaced by [redacted] markers. Safe to call on operator-authored reasons,
// command strings, and command output before persistence to the audit log.
// Returns the original string when no redactor matches.
func RedactAuditText(s string) string {
	if s == "" {
		return s
	}
	for _, redactor := range auditSecretRedactors {
		s = redactor.pattern.ReplaceAllString(s, redactor.replacement)
	}
	return s
}

func redactActionAuditDetailText(s string, replacement string) string {
	if strings.TrimSpace(RedactAuditText(s)) == "" {
		return ""
	}
	return replacement
}

func redactActionVerificationResult(result *ActionVerificationResult) *ActionVerificationResult {
	redacted := NormalizeActionVerificationResult(result)
	if redacted == nil {
		return nil
	}
	redacted.Command = redactActionAuditDetailText(redacted.Command, auditVerificationCommandRedacted)
	redacted.Output = redactActionAuditDetailText(redacted.Output, auditVerificationOutputRedacted)
	redacted.Note = redactActionAuditDetailText(redacted.Note, auditVerificationNoteRedacted)
	return redacted
}

func redactActionExecutionResult(result *ExecutionResult) *ExecutionResult {
	if result == nil {
		return nil
	}
	redacted := *result
	redacted.Output = RedactAuditText(redacted.Output)
	redacted.ErrorMessage = RedactAuditText(redacted.ErrorMessage)
	redacted.Verification = redactActionVerificationResult(redacted.Verification)
	if redacted.ActionResultV2 != nil {
		canonical := cloneActionResultV2(*redacted.ActionResultV2)
		clearActionEvidenceDigests(canonical.Verification.Evidence)
		clearActionEvidenceDigests(canonical.Compensation.Evidence)
		if canonical.Compensation.Verification != nil {
			clearActionEvidenceDigests(canonical.Compensation.Verification.Evidence)
		}
		if normalized, err := NormalizeActionResultV2(canonical); err == nil {
			redacted.ActionResultV2 = &normalized
		} else {
			fallback := redactionContractViolationResult()
			redacted.ActionResultV2 = &fallback
		}
	}
	return &redacted
}

func cloneActionResultV2(result ActionResultV2) ActionResultV2 {
	result.Verification.Evidence = cloneActionEvidence(result.Verification.Evidence)
	result.Compensation.Evidence = cloneActionEvidence(result.Compensation.Evidence)
	if result.Compensation.StartedAt != nil {
		startedAt := *result.Compensation.StartedAt
		result.Compensation.StartedAt = &startedAt
	}
	if result.Compensation.CompletedAt != nil {
		completedAt := *result.Compensation.CompletedAt
		result.Compensation.CompletedAt = &completedAt
	}
	if result.Compensation.Execution != nil {
		execution := *result.Compensation.Execution
		result.Compensation.Execution = &execution
	}
	if result.Compensation.Verification != nil {
		verification := *result.Compensation.Verification
		verification.Evidence = cloneActionEvidence(verification.Evidence)
		result.Compensation.Verification = &verification
	}
	if result.Compensation.RestoredState != nil {
		restored := *result.Compensation.RestoredState
		result.Compensation.RestoredState = &restored
	}
	return result
}

func cloneActionEvidence(evidence []ActionEvidence) []ActionEvidence {
	if evidence == nil {
		return nil
	}
	cloned := make([]ActionEvidence, len(evidence))
	copy(cloned, evidence)
	for i := range cloned {
		cloned[i].Refs = append([]ActionEvidenceRef(nil), evidence[i].Refs...)
	}
	return cloned
}

func clearActionEvidenceDigests(evidence []ActionEvidence) {
	for i := range evidence {
		evidence[i].Digest = ""
	}
}

func redactionContractViolationResult() ActionResultV2 {
	fallback := ActionResultV2{
		Version: ActionResultV2Version,
		Execution: ActionExecutionTruth{
			Status: ActionExecutionInconclusive, ReasonCode: "redaction_contract_violation",
			Summary: "Action result was rejected by the audit redaction contract.",
		},
		Verification: ActionVerificationTruth{
			Status: ActionVerificationInconclusive, EvidenceClass: ActionEvidenceNone,
			ReasonCode: "redaction_contract_violation",
		},
		Compensation: ActionCompensationTruth{Support: ActionCompensationUnavailable, Status: ActionCompensationNotAvailable},
	}
	normalized, err := NormalizeActionResultV2(fallback)
	if err != nil {
		panic("canonical redaction fallback is invalid: " + err.Error())
	}
	return normalized
}

// RedactAuditRecord returns a copy of the input ActionAuditRecord with
// known secret shapes scrubbed from the operator-authored reason, the
// params map's string values, execution output, and error text. Verification
// command/output/note details are policy-hidden as stable markers because
// they can expose sensitive execution context even when no recognizable
// secret pattern is present. The canonical Plan, Approvals, and identity
// fields are left alone — they are produced by Pulse, not by operators or
// external command output.
func RedactAuditRecord(record ActionAuditRecord) ActionAuditRecord {
	record.Request.Reason = RedactAuditText(record.Request.Reason)
	if len(record.Request.Params) > 0 {
		redactedParams := make(map[string]any, len(record.Request.Params))
		for k, v := range record.Request.Params {
			if str, ok := v.(string); ok {
				redactedParams[k] = RedactAuditText(str)
			} else {
				redactedParams[k] = v
			}
		}
		record.Request.Params = redactedParams
	}
	record.Result = redactActionExecutionResult(record.Result)
	record.Verification = redactActionVerificationResult(record.Verification)
	record.VerificationOutcome.EvidenceSummary = RedactAuditText(record.VerificationOutcome.EvidenceSummary)
	return record
}
