package ai

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const patrolRuntimeFailureDetailLimit = 2000
const patrolProviderNotConfiguredReason = "Patrol provider not configured - open Assistant & Patrol provider settings, configure a provider, and choose a Patrol model that supports tools"

var patrolRuntimeFailureDetailRedactors = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{
		pattern:     regexp.MustCompile(`(?i)([?&](?:key|api_key|apikey|access_token|token)=)[^\s&"']+`),
		replacement: `${1}[redacted]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)("(?:api[_-]?key|apikey|access[_-]?token|token|authorization|x-api-key)"\s*:\s*")[^"]+`),
		replacement: `${1}[redacted]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)((?:authorization:\s*bearer|x-api-key:)\s+)[^\s,;]+`),
		replacement: `${1}[redacted]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)(https?://)[^\s/@:]+:[^\s/@]+@`),
		replacement: `${1}[redacted]@`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)https?://[^\s"')]+`),
		replacement: `[redacted-url]`,
	},
	{
		pattern:     regexp.MustCompile(`\buser_[A-Za-z0-9_-]+\b`),
		replacement: `[redacted-user]`,
	},
	{
		pattern:     regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}\b`),
		replacement: `[redacted-secret]`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)("(?:user[_-]?id)"\s*:\s*")[^"]+`),
		replacement: `${1}[redacted]`,
	},
}

type patrolRuntimeFailure struct {
	Title          string
	Summary        string
	Cause          PatrolFailureCause
	Description    string
	Impact         string
	Recommendation string
	Detail         string
	Evidence       string
}

// patrolRuntimeFailureImpact is the shared consequence-if-ignored statement
// for any Patrol runtime failure. The cause varies but the operational
// consequence is constant: while Patrol is not analyzing, alerts continue
// to fire without enrichment.
const patrolRuntimeFailureImpact = "While Patrol cannot analyze, alerts continue to fire without fresh Patrol evidence, and AI Intelligence summaries cannot refresh."

type PatrolRuntimeFailureDiagnostic struct {
	Title          string
	Summary        string
	Cause          PatrolFailureCause
	Description    string
	Recommendation string
}

// patrolToolChoiceValueRejected reports whether the upstream error indicates
// the provider rejected a tool_choice transport field. This is distinct from
// the model truly lacking tool support: the provider accepted tools but not
// that request-shape detail.
func patrolToolChoiceValueRejected(lower string) bool {
	if !strings.Contains(lower, "tool_choice") {
		return false
	}
	return strings.Contains(lower, "does not support this tool_choice") ||
		strings.Contains(lower, "tool_choice is not supported") ||
		strings.Contains(lower, "tool_choice value is not supported") ||
		strings.Contains(lower, "invalid tool_choice") ||
		strings.Contains(lower, "unsupported tool_choice")
}

// patrolNoToolCapableEndpoint reports whether the upstream error indicates
// the provider has no available endpoint that supports tools for the
// selected model. OpenRouter surfaces this as "No endpoints found that
// support tool use" when account-level provider or data-policy filters
// exclude every tool-capable route.
func patrolNoToolCapableEndpoint(lower string) bool {
	return strings.Contains(lower, "no endpoints found") && strings.Contains(lower, "tool")
}

// patrolMalformedToolHistory reports whether the upstream error indicates
// Pulse sent a conversation where an assistant message had tool_calls
// without matching tool result messages for every tool_call_id. Distinct
// from tool_choice / capability errors: this is a structural mismatch in
// the message slice Pulse assembled. DeepSeek phrases it as
// "An assistant message with 'tool_calls' must be followed by tool messages
// responding to each 'tool_call_id'", OpenAI uses similar wording.
func patrolMalformedToolHistory(lower string) bool {
	if !strings.Contains(lower, "tool_call_id") && !strings.Contains(lower, "tool_calls") {
		return false
	}
	return strings.Contains(lower, "must be followed by tool messages") ||
		strings.Contains(lower, "insufficient tool messages") ||
		strings.Contains(lower, "responding to each")
}

func ClassifyPatrolRuntimeFailure(err error) PatrolRuntimeFailureDiagnostic {
	failure := patrolRuntimeFailureFromError(err)
	return PatrolRuntimeFailureDiagnostic{
		Title:          failure.Title,
		Summary:        failure.Summary,
		Cause:          failure.Cause,
		Description:    failure.Description,
		Recommendation: failure.Recommendation,
	}
}

func ClassifyProviderConnectionFailure(err error) PatrolRuntimeFailureDiagnostic {
	failure := patrolRuntimeFailureFromError(err)
	diagnostic := PatrolRuntimeFailureDiagnostic{
		Title:          "Provider connection issue",
		Summary:        "Provider connection issue",
		Cause:          failure.Cause,
		Description:    "Pulse could not maintain a healthy connection to this provider.",
		Recommendation: "Check provider reachability, base URL, firewall or proxy rules, and provider availability, then retry.",
	}

	switch failure.Cause {
	case PatrolFailureCauseMalformedToolHistory:
		diagnostic.Title = "Provider conversation state issue"
		diagnostic.Summary = "Provider conversation state issue"
		diagnostic.Description = "The provider rejected the conversation structure used by Pulse."
		diagnostic.Recommendation = "Start a new assistant session and retry. If the issue persists, restart Pulse and report the selected provider and model."
	case PatrolFailureCauseToolChoiceRejected:
		diagnostic.Title = "Provider rejected tool-choice request"
		diagnostic.Summary = "Provider rejected tool-choice request"
		diagnostic.Description = "Pulse reached the provider, but the provider rejected a tool-choice transport setting."
		diagnostic.Recommendation = "Retry with automatic tool selection, or switch to a provider route with reliable tool-call support."
	case PatrolFailureCauseNoToolCapableEndpoint:
		diagnostic.Title = "No tool-capable provider endpoint available"
		diagnostic.Summary = "No tool-capable provider endpoint available"
		diagnostic.Description = "Pulse reached the provider, but the provider reports no available endpoint with tool support for the selected model."
		diagnostic.Recommendation = "Review provider routing and privacy filters, broaden allowed providers, or switch to a model with broader tool support."
	case PatrolFailureCauseModelUnsupportedTools:
		diagnostic.Title = "Selected model does not support tools"
		diagnostic.Summary = "Selected model does not support tools"
		diagnostic.Description = "Pulse reached the provider, but the selected model or routed endpoint rejected tool calling."
		diagnostic.Recommendation = "Choose a model or provider route that supports tool calling for governed Assistant and Patrol workflows."
	case PatrolFailureCauseModelUnavailable:
		diagnostic.Title = "Selected model unavailable"
		diagnostic.Summary = "Selected model unavailable"
		diagnostic.Description = "The selected model is not available from this provider path."
		diagnostic.Recommendation = "Choose one of the models currently returned by the provider, then retry."
	case PatrolFailureCauseContextWindowTooSmall:
		diagnostic.Title = "Selected model context window too small"
		diagnostic.Summary = "Selected model context window too small"
		diagnostic.Description = "The provider rejected the request because the selected model could not fit the current context."
		diagnostic.Recommendation = "Choose a model with a larger context window or retry with a narrower request."
	case PatrolFailureCauseProviderBilling:
		diagnostic.Title = "Provider billing or quota issue"
		diagnostic.Summary = "Provider billing or quota issue"
		diagnostic.Description = "The provider rejected the request for billing or quota reasons."
		diagnostic.Recommendation = "Resolve the billing or quota issue with your provider, or switch to a different provider or model."
	case PatrolFailureCauseProviderRateLimited:
		diagnostic.Title = "Provider rate limited"
		diagnostic.Summary = "Provider rate limited"
		diagnostic.Description = "The provider is rate limiting requests for this account or model."
		diagnostic.Recommendation = "Wait for the provider rate limit to reset, increase provider limits, or switch to another model."
	case PatrolFailureCauseProviderAuth:
		diagnostic.Title = "Provider authentication issue"
		diagnostic.Summary = "Provider authentication issue"
		diagnostic.Description = "The provider rejected the configured credentials or account access."
		diagnostic.Recommendation = "Check the API key or provider authentication in Assistant and Patrol settings, then retry."
	case PatrolFailureCauseProviderNotConfigured, PatrolFailureCauseModelNotSelected, PatrolFailureCauseModelProviderUnconfigured, PatrolFailureCauseAssistantDisabled, PatrolFailureCauseSettingsPersistence:
		diagnostic.Title = "Provider not ready"
		diagnostic.Summary = "Provider not ready"
		diagnostic.Description = "Pulse cannot test this provider because the provider runtime is not ready."
		diagnostic.Recommendation = "Open Assistant and Patrol provider settings, complete provider configuration, verify the selected model, and retry."
	}

	return diagnostic
}

func patrolRuntimeFailureFromError(err error) patrolRuntimeFailure {
	raw := ""
	if err != nil {
		raw = strings.TrimSpace(err.Error())
	}
	detail := truncateString(summarizePatrolRuntimeFailureDetail(raw), patrolRuntimeFailureDetailLimit)
	lower := strings.ToLower(raw)

	failure := patrolRuntimeFailure{
		Title:          "Pulse Patrol: Provider analysis error",
		Summary:        "Provider analysis error",
		Cause:          PatrolFailureCauseProviderConnection,
		Description:    "Pulse Patrol reached the configured provider, but the provider did not complete the Patrol analysis request.",
		Impact:         patrolRuntimeFailureImpact,
		Recommendation: "Review the Patrol provider settings, selected model, and provider logs, then rerun Patrol after the provider path is healthy.",
		Detail:         detail,
	}

	switch {
	case patrolMalformedToolHistory(lower):
		failure.Title = "Pulse Patrol: Malformed tool-call conversation history"
		failure.Summary = "Malformed tool-call conversation history"
		failure.Cause = PatrolFailureCauseMalformedToolHistory
		failure.Description = "Pulse Patrol reached the provider, but the conversation it sent had an assistant message containing tool_calls without matching tool result messages for every tool_call_id. The provider rejects this structure. This usually means a previous Patrol run ended after the model emitted tool calls but before all results were captured, leaving orphan tool_calls in persisted state that the next run reused."
		failure.Recommendation = "Pulse should treat each Patrol run as stateless. If the failure persists across runs, restart Pulse to clear any in-memory session state and report the issue."
	case patrolToolChoiceValueRejected(lower):
		failure.Title = "Pulse Patrol: Provider rejected tool-choice request"
		failure.Summary = "Provider rejected tool-choice request"
		failure.Cause = PatrolFailureCauseToolChoiceRejected
		failure.Description = "Pulse Patrol reached the provider and the model accepts tools, but the provider rejected a tool_choice transport field. Patrol should keep model tool selection automatic and avoid provider-specific coercion."
		failure.Recommendation = "Retry with automatic tool selection. If the failure persists, switch Patrol to a model or provider route with reliable tool-call support, or report the model in question."
	case patrolNoToolCapableEndpoint(lower):
		failure.Title = "Pulse Patrol: No tool-capable provider endpoint available"
		failure.Summary = "No tool-capable provider endpoint available"
		failure.Cause = PatrolFailureCauseNoToolCapableEndpoint
		failure.Description = "Pulse Patrol reached the provider, but the provider reports no available endpoint that supports tool calling for the selected model. For OpenRouter this typically reflects account-level provider or data-policy filters that exclude every tool-capable route, leaving only routes that do not support tools."
		failure.Recommendation = "Review provider routing and privacy filters (for OpenRouter, the Privacy / Data Policy settings and per-model allowed providers), broaden the allowed providers, or switch Patrol to a model with broader tool support."
	case strings.Contains(lower, "tool_choice") ||
		strings.Contains(lower, "tool calling") ||
		strings.Contains(lower, "tools are not supported"):
		failure.Title = "Pulse Patrol: Selected model does not support Patrol tools"
		failure.Summary = "Selected model does not support Patrol tools"
		failure.Cause = PatrolFailureCauseModelUnsupportedTools
		failure.Description = "Pulse Patrol reached the provider, but the selected model or routed endpoint rejected tool-calling. Patrol needs tool support to inspect resources and report governed findings."
		failure.Recommendation = "Choose a Patrol model or provider route that supports tool calling. For OpenRouter, select an endpoint that supports tools/tool_choice, or switch to a local or BYOK model with tool support."
	case strings.Contains(lower, "model") && (strings.Contains(lower, "not available") ||
		strings.Contains(lower, "not found") ||
		strings.Contains(lower, "does not exist") ||
		strings.Contains(lower, "no such model") ||
		strings.Contains(lower, "invalid model") ||
		strings.Contains(lower, "unsupported model")):
		failure.Title = "Pulse Patrol: Selected model unavailable"
		failure.Summary = "Selected model unavailable"
		failure.Cause = PatrolFailureCauseModelUnavailable
		failure.Description = "Pulse Patrol reached the provider, but the configured Patrol model is not available from that provider path."
		failure.Recommendation = "Open Patrol provider settings and choose one of the models currently returned by the provider, then rerun Patrol."
	case isPatrolContextWindowError(err):
		failure.Title = "Pulse Patrol: Selected model context window too small"
		failure.Summary = "Selected model context window too small"
		failure.Cause = PatrolFailureCauseContextWindowTooSmall
		failure.Description = "The provider rejected Patrol analysis because the selected model could not fit the Patrol context after retrying with smaller context budgets."
		failure.Recommendation = "Choose a model with a larger context window or run a narrower scoped Patrol check."
	case strings.Contains(lower, "insufficient balance") ||
		strings.Contains(lower, "402") ||
		strings.Contains(lower, "payment required") ||
		strings.Contains(lower, "quota") ||
		strings.Contains(lower, "credit"):
		failure.Title = "Pulse Patrol: Provider billing or quota issue"
		failure.Summary = "Provider billing or quota issue"
		failure.Cause = PatrolFailureCauseProviderBilling
		failure.Description = "Pulse Patrol cannot analyze your infrastructure because the configured provider rejected the request for billing or quota reasons."
		failure.Recommendation = "Resolve the billing or quota issue with your provider, or switch Patrol to a different provider or local model."
	case strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "429") ||
		strings.Contains(lower, "too many requests"):
		failure.Title = "Pulse Patrol: Provider rate limited"
		failure.Summary = "Provider rate limited"
		failure.Cause = PatrolFailureCauseProviderRateLimited
		failure.Description = "Pulse Patrol is being rate limited by the configured provider, so this analysis run could not complete."
		failure.Recommendation = "Wait for the provider rate limit to reset, increase provider limits, or switch Patrol to another capable model."
	case strings.Contains(lower, "401") ||
		strings.Contains(lower, "403") ||
		strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "forbidden") ||
		strings.Contains(lower, "api key"):
		failure.Title = "Pulse Patrol: Provider authentication issue"
		failure.Summary = "Provider authentication issue"
		failure.Cause = PatrolFailureCauseProviderAuth
		failure.Description = "Pulse Patrol cannot analyze your infrastructure because the provider rejected the configured credentials or account access."
		failure.Recommendation = "Check the API key or provider authentication in Patrol provider settings, then rerun Patrol."
	case strings.Contains(lower, "not configured") ||
		strings.Contains(lower, "no provider configured") ||
		strings.Contains(lower, "chat service not available") ||
		strings.Contains(lower, "provider not available") ||
		strings.Contains(lower, "failed to create provider"):
		failure.Title = "Pulse Patrol: Provider not ready"
		failure.Summary = "Provider not ready"
		failure.Cause = PatrolFailureCauseProviderNotConfigured
		failure.Description = "Pulse Patrol could not start analysis because the Patrol provider runtime is not ready."
		failure.Recommendation = "Open Patrol provider settings, complete provider configuration, verify the selected model, and rerun Patrol."
	case strings.Contains(lower, "failed to connect") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "returned status 5") ||
		strings.Contains(lower, "api error (5"):
		failure.Title = "Pulse Patrol: Provider connection issue"
		failure.Summary = "Provider connection issue"
		failure.Cause = PatrolFailureCauseProviderConnection
		failure.Description = "Pulse Patrol could not maintain a healthy connection to the configured provider during analysis."
		failure.Recommendation = "Check provider reachability, base URL, firewall or proxy rules, and provider availability, then rerun Patrol."
	}

	if failure.Detail != "" {
		failure.Evidence = fmt.Sprintf("Provider error: %s", failure.Detail)
	}

	return failure
}

func redactPatrolRuntimeFailureDetail(raw string) string {
	redacted := raw
	for _, redactor := range patrolRuntimeFailureDetailRedactors {
		redacted = redactor.pattern.ReplaceAllString(redacted, redactor.replacement)
	}
	return redacted
}

func summarizePatrolRuntimeFailureDetail(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	switch {
	case patrolMalformedToolHistory(lower):
		return "Pulse sent a malformed tool-call conversation. Each Patrol run should be stateless; restart Pulse if the failure persists."
	case patrolToolChoiceValueRejected(lower):
		return "Provider rejected a tool-choice transport setting. Patrol should use automatic model-owned tool selection."
	case patrolNoToolCapableEndpoint(lower):
		return "Provider has no tool-capable endpoint for the selected model. Review provider routing or privacy filters."
	case strings.Contains(lower, "tool_choice") ||
		strings.Contains(lower, "tool calling") ||
		strings.Contains(lower, "tools are not supported"):
		return "Provider rejected Patrol tool calls. Choose a Patrol model and endpoint with tool-call support."
	case strings.Contains(lower, "reasoning_content"):
		return "Provider rejected Patrol reasoning state. Retry with a provider route that supports the selected model's reasoning and tool protocol."
	case strings.Contains(lower, "model") && (strings.Contains(lower, "not available") ||
		strings.Contains(lower, "not found") ||
		strings.Contains(lower, "does not exist") ||
		strings.Contains(lower, "no such model") ||
		strings.Contains(lower, "invalid model") ||
		strings.Contains(lower, "unsupported model")):
		return "Selected provider model is not available from this provider path."
	case strings.Contains(lower, "insufficient balance") ||
		strings.Contains(lower, "402") ||
		strings.Contains(lower, "payment required") ||
		strings.Contains(lower, "quota") ||
		strings.Contains(lower, "credit") ||
		strings.Contains(lower, "max_tokens"):
		return "Provider reported insufficient credits or token budget for the requested Patrol analysis."
	case strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "429") ||
		strings.Contains(lower, "too many requests"):
		return "Provider rate limit reached. Wait for capacity or adjust provider limits before retrying."
	case strings.Contains(lower, "401") ||
		strings.Contains(lower, "403") ||
		strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "forbidden") ||
		strings.Contains(lower, "api key"):
		return "Provider authentication failed. Check the configured provider key and account access."
	case strings.Contains(lower, "failed to connect") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "returned status 5"):
		return "Provider connection failed. Check provider reachability before retrying Patrol."
	default:
		return strings.TrimSpace(redactPatrolRuntimeFailureDetail(raw))
	}
}

func newPatrolRuntimeFailureFinding(failure patrolRuntimeFailure, now time.Time) *Finding {
	return &Finding{
		ID:             generateFindingID(patrolRuntimeResourceID, "reliability", patrolRuntimeFindingKey),
		Key:            patrolRuntimeFindingKey,
		Severity:       FindingSeverityWarning,
		Category:       FindingCategoryReliability,
		ResourceID:     patrolRuntimeResourceID,
		ResourceName:   "Pulse Patrol Service",
		ResourceType:   "service",
		Title:          failure.Title,
		Description:    failure.Description,
		Impact:         failure.Impact,
		Recommendation: failure.Recommendation,
		Evidence:       failure.Evidence,
		FailureCause:   string(failure.Cause),
		DetectedAt:     now,
		LastSeenAt:     now,
	}
}

func (p *PatrolService) resolvePatrolRuntimeFailureFinding(reason string) bool {
	if p == nil || p.findings == nil {
		return false
	}
	errorFindingID := generateFindingID(patrolRuntimeResourceID, "reliability", patrolRuntimeFindingKey)
	if existing := p.findings.Get(errorFindingID); existing == nil || existing.IsResolved() {
		return false
	}

	p.findings.Resolve(errorFindingID, true)
	if resolver := p.unifiedFindingResolver; resolver != nil {
		resolver(errorFindingID)
	}
	log.Info().Str("reason", reason).Msg("AI Patrol: Auto-resolved previous patrol runtime finding after successful provider-backed run")
	return true
}
