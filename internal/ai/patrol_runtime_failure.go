package ai

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rs/zerolog/log"
)

const patrolRuntimeFailureDetailLimit = 2000
const patrolProviderNotConfiguredReason = "Patrol provider not configured - open Pulse Intelligence settings, configure a provider, and choose a Patrol model that supports tools"

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

// patrolRunCancelled reports whether the upstream error is a mid-run
// cancellation rather than a provider failure. context.Canceled reaches this
// classifier when the operator cancels the run or the HTTP client connection
// drops (a reverse proxy cutting a long request cancels r.Context()). It is
// checked before every provider-fault pattern because a cancelled run says
// nothing about the provider or model (#1640). context.DeadlineExceeded is
// deliberately excluded: a deadline is a real timeout on the provider path.
//
// The error text alone is never enough, so a raw "context canceled" substring
// is not a signal on its own. Providers embed that exact phrase in their own
// error bodies when they abort an upstream request while our run is perfectly
// healthy (Ollama does), and misreading it as our cancellation makes the
// readiness path persist a genuine provider failure as "not assessed". The
// wording only counts once the run's own context is cancelled, at which point
// the cancelled context is itself the signal: nothing that surfaces out of a
// torn-down request (a bare EOF, a reset, a truncated stream) is evidence
// about the provider either.
func patrolRunCancelled(ctx context.Context, err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	return ctx != nil && errors.Is(ctx.Err(), context.Canceled)
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
	if setup, ok := subscriptionAgentSetupFailure(err); ok {
		return PatrolRuntimeFailureDiagnostic{
			Title:          "Local " + setup.displayName + " CLI not ready",
			Summary:        "Local " + setup.displayName + " CLI not ready",
			Cause:          PatrolFailureCauseProviderNotConfigured,
			Description:    "Pulse cannot use the local " + setup.displayName + " subscription because its CLI executable or login is unavailable to the operating-system account running Pulse.",
			Recommendation: setup.recommendation,
		}
	}
	failure := patrolRuntimeFailureFromError(err)
	diagnostic := PatrolRuntimeFailureDiagnostic{
		Title:          "Provider connection issue",
		Summary:        "Provider connection issue",
		Cause:          failure.Cause,
		Description:    "Pulse could not maintain a healthy connection to this provider.",
		Recommendation: "Check provider reachability, base URL, firewall or proxy rules, and provider availability, then retry.",
	}

	switch failure.Cause {
	case PatrolFailureCauseInterrupted:
		diagnostic.Title = "Connection test interrupted"
		diagnostic.Summary = "Connection test interrupted"
		diagnostic.Description = "The connection test was cancelled before the provider finished responding."
		diagnostic.Recommendation = "Run the test again when you are ready."
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
		diagnostic.Recommendation = "Check the API key or provider authentication on the Provider & Models settings page, then retry."
	case PatrolFailureCauseProviderNotConfigured, PatrolFailureCauseModelNotSelected, PatrolFailureCauseModelProviderUnconfigured, PatrolFailureCauseAssistantDisabled, PatrolFailureCauseSettingsPersistence:
		diagnostic.Title = "Provider not ready"
		diagnostic.Summary = "Provider not ready"
		diagnostic.Description = "Pulse cannot test this provider because the provider runtime is not ready."
		diagnostic.Recommendation = "Open the Provider & Models settings page, complete provider configuration, verify the selected model, and retry."
	}

	return diagnostic
}

type subscriptionAgentSetupCopy struct {
	displayName    string
	recommendation string
}

func subscriptionAgentSetupFailure(err error) (subscriptionAgentSetupCopy, bool) {
	var setupErr *providers.SubscriptionAgentSetupError
	if !errors.As(err, &setupErr) {
		return subscriptionAgentSetupCopy{}, false
	}
	switch setupErr.Agent {
	case providers.SubscriptionAgentClaude:
		return subscriptionAgentSetupCopy{
			displayName:    "Claude",
			recommendation: "Install the Claude CLI and run `claude auth login` as the same account that runs Pulse. On standard systemd installs, use the `pulse` account with home `/opt/pulse`; restart Pulse, then retry.",
		}, true
	case providers.SubscriptionAgentCodex:
		return subscriptionAgentSetupCopy{
			displayName:    "Codex",
			recommendation: "Install the Codex CLI and run `codex login` as the same account that runs Pulse. On standard systemd installs, use the `pulse` account with home `/opt/pulse`; restart Pulse, then retry.",
		}, true
	default:
		return subscriptionAgentSetupCopy{
			displayName:    "subscription",
			recommendation: "Install and sign in to the local subscription CLI as the same operating-system account that runs Pulse, restart Pulse, then retry.",
		}, true
	}
}

// patrolRuntimeFailureFromError classifies an error with no knowledge of the
// run context. Cancellation is then recognised only when the error actually
// wraps context.Canceled. Callers that hold the run's context should use
// patrolRuntimeFailureFromErrorCtx so a torn-down run is not blamed on the
// provider (#1640).
func patrolRuntimeFailureFromError(err error) patrolRuntimeFailure {
	return patrolRuntimeFailureFromErrorCtx(context.Background(), err)
}

func patrolRuntimeFailureFromErrorCtx(ctx context.Context, err error) patrolRuntimeFailure {
	raw := ""
	if err != nil {
		raw = strings.TrimSpace(err.Error())
	}
	cancelled := patrolRunCancelled(ctx, err)
	detail := truncateString(summarizePatrolRuntimeFailureDetail(raw, cancelled), patrolRuntimeFailureDetailLimit)
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

	setup, setupFailure := subscriptionAgentSetupFailure(err)
	switch {
	case cancelled:
		failure.Title = "Pulse Patrol: Analysis interrupted"
		failure.Summary = "Analysis interrupted before completion"
		failure.Cause = PatrolFailureCauseInterrupted
		failure.Description = "The Patrol run was cancelled before the provider finished, either by an operator cancel or because the client connection closed mid-analysis. An interrupted run is not evidence about the provider or model."
		failure.Recommendation = "Run the analysis again when you are ready. If you did not cancel it, check for reverse proxies or load balancers that close long-running requests."
	case errors.Is(err, ErrCostBudgetExceeded):
		// The provider was never called: Pulse itself declined to spend.
		failure.Title = "Pulse Patrol: 30-day cost budget reached"
		failure.Summary = "30-day cost budget reached"
		failure.Cause = PatrolFailureCauseBudgetExhausted
		failure.Description = "Pulse Patrol skipped its analysis because estimated spend on AI providers reached the 30-day budget you set. Nothing was sent to the provider, so this says nothing about the model."
		failure.Recommendation = "Raise the 30-day budget under Pulse Intelligence › Provider & Models, choose a cheaper Patrol model or a slower schedule, or wait for the 30-day window to roll on. Patrol resumes on its next scheduled run once spend is back under budget."
	case setupFailure:
		failure.Title = "Pulse Patrol: Local " + setup.displayName + " CLI not ready"
		failure.Summary = "Local " + setup.displayName + " CLI not ready"
		failure.Cause = PatrolFailureCauseProviderNotConfigured
		failure.Description = "Pulse Patrol cannot use the local " + setup.displayName + " subscription because its CLI executable or login is unavailable to the operating-system account running Pulse."
		failure.Recommendation = setup.recommendation
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

// summarizePatrolRuntimeFailureDetail rewrites a raw provider error into
// operator-facing detail. cancelled must be the caller's decision about whether
// the run was actually cancelled: the "context canceled" wording on its own is
// not proof, because providers put it in their own error bodies (#1640).
func summarizePatrolRuntimeFailureDetail(raw string, cancelled bool) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	switch {
	case cancelled:
		return "The run was cancelled before the provider finished. This is not evidence of a provider or model fault."
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

// patrolBlockedRunFailure describes a scheduled run Patrol skipped before any
// provider attempt — a persistent readiness blocker (no provider configured,
// no usable Patrol model) or a provider that failed to initialize. Raising it
// as the deduped runtime finding gives the operator one nudge on the surfaces
// they actually watch (findings list, attention badge, alert notification
// channels); the Patrol page banner alone left field installs blocked for
// weeks without anyone noticing. reason must already be operator-honest: it
// becomes the finding description.
func patrolBlockedRunFailure(reason string, cause PatrolFailureCause) patrolRuntimeFailure {
	description := strings.TrimSpace(redactPatrolRuntimeFailureDetail(reason))
	if description == "" {
		description = "Patrol is enabled and scheduled, but its AI runtime is not ready, so analysis runs are being skipped."
	}
	if cause == "" {
		cause = PatrolFailureCauseProviderNotConfigured
	}
	return patrolRuntimeFailure{
		Title:          "Pulse Patrol: Runs are being skipped",
		Summary:        "Patrol is enabled but runs are being skipped",
		Cause:          cause,
		Description:    description,
		Impact:         patrolRuntimeFailureImpact,
		Recommendation: "Open the Provider & Models settings page and complete the provider setup, or turn Patrol off if you do not plan to use AI analysis.",
	}
}

// retryProviderInitIfNeeded reports whether the AI service can run LLM
// analysis, first retrying provider initialization when the config claims a
// configured provider but the boot-time build failed. Provider construction
// can depend on a live model-catalog call, so Pulse starting before a local
// Ollama server must strand Patrol for at most one interval, not until the
// next settings save.
func (p *PatrolService) retryProviderInitIfNeeded(ctx context.Context) bool {
	if p == nil || p.aiService == nil {
		return false
	}
	if p.aiService.IsEnabled() {
		return true
	}
	if p.aiService.RetryProviderInit(ctx) {
		log.Info().Msg("AI Patrol: Provider initialization recovered on scheduled retry")
		return true
	}
	return false
}

// providerUnavailableReason returns the operator-facing reason Patrol cannot
// use a provider right now. When the config claims a configured provider but
// initialization failed, the reason names that failure instead of telling an
// operator who has configured a provider to configure one.
func (p *PatrolService) providerUnavailableReason() (string, PatrolFailureCause) {
	if p != nil && p.aiService != nil {
		if initErr := strings.TrimSpace(p.aiService.ProviderInitError()); initErr != "" {
			return fmt.Sprintf("The configured AI provider failed to initialize, so Patrol runs are being skipped. Pulse retries on every scheduled run. Provider error: %s", initErr), PatrolFailureCauseProviderConnection
		}
	}
	return patrolProviderNotConfiguredReason, PatrolFailureCauseProviderNotConfigured
}

// raiseBlockedRunFinding records the deduped runtime finding for a scheduled
// run skipped by a persistent readiness blocker. The stable runtime finding ID
// means repeated blocked ticks notify at most once, and the finding
// auto-resolves on the next successful provider-backed run.
func (p *PatrolService) raiseBlockedRunFinding(reason string, cause PatrolFailureCause) {
	if p == nil || p.findings == nil {
		return
	}
	p.recordFinding(newPatrolRuntimeFailureFinding(patrolBlockedRunFailure(reason, cause), time.Now()))
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
	log.Info().Str("reason", reason).Msg("AI Patrol: Resolved patrol runtime finding")
	return true
}
