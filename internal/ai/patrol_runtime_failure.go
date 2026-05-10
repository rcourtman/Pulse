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
const patrolRuntimeFailureImpact = "While Patrol cannot analyze, alerts continue to fire without evidence or recommended actions, and AI Intelligence summaries cannot refresh."

type PatrolRuntimeFailureDiagnostic struct {
	Title          string
	Summary        string
	Cause          PatrolFailureCause
	Description    string
	Recommendation string
}

// patrolToolChoiceValueRejected reports whether the upstream error indicates
// the provider rejected the specific tool_choice value Pulse sent (for
// example, "deepseek-reasoner does not support this tool_choice"). This is
// distinct from the model truly lacking tool support: the model accepts
// tools but not the requested coercion.
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
	case patrolToolChoiceValueRejected(lower):
		failure.Title = "Pulse Patrol: Provider rejected forced tool selection"
		failure.Summary = "Provider rejected forced tool selection"
		failure.Cause = PatrolFailureCauseToolChoiceRejected
		failure.Description = "Pulse Patrol reached the provider and the model accepts tools, but the provider rejected the specific tool-selection coercion Pulse sent. This usually means the routed model accepts tools yet does not honour a request to force a particular tool, only automatic selection."
		failure.Recommendation = "Pulse will retry with automatic tool selection on the next Patrol run. If the failure persists, switch Patrol to a different model or provider where forced tool selection is accepted, or report the model in question."
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
	case patrolToolChoiceValueRejected(lower):
		return "Provider rejected Pulse's forced tool selection. Pulse will retry with automatic tool selection on the next Patrol run."
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
