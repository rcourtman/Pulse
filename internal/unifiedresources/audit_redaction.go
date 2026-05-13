package unifiedresources

import (
	"regexp"
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

func redactActionVerificationResult(result *ActionVerificationResult) *ActionVerificationResult {
	redacted := NormalizeActionVerificationResult(result)
	if redacted == nil {
		return nil
	}
	redacted.Command = RedactAuditText(redacted.Command)
	redacted.Output = RedactAuditText(redacted.Output)
	redacted.Note = RedactAuditText(redacted.Note)
	return redacted
}

// RedactAuditRecord returns a copy of the input ActionAuditRecord with
// known secret shapes scrubbed from the operator-authored reason, the
// params map's string values, execution output, and verification command
// fields. The canonical Plan, Approvals, and identity fields are left
// alone — they are produced by Pulse, not by operators or external
// command output.
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
	if record.Result != nil {
		redacted := *record.Result
		redacted.Output = RedactAuditText(redacted.Output)
		redacted.ErrorMessage = RedactAuditText(redacted.ErrorMessage)
		redacted.Verification = redactActionVerificationResult(redacted.Verification)
		record.Result = &redacted
	}
	record.Verification = redactActionVerificationResult(record.Verification)
	record.VerificationOutcome.EvidenceSummary = RedactAuditText(record.VerificationOutcome.EvidenceSummary)
	return record
}
