package ai

import (
	"fmt"
	"strings"
	"time"
)

const patrolRuntimeFailureDetailLimit = 2000

type patrolRuntimeFailure struct {
	Title          string
	Summary        string
	Description    string
	Recommendation string
	Detail         string
	Evidence       string
}

func patrolRuntimeFailureFromError(err error) patrolRuntimeFailure {
	raw := ""
	if err != nil {
		raw = strings.TrimSpace(err.Error())
	}
	detail := truncateString(raw, patrolRuntimeFailureDetailLimit)
	lower := strings.ToLower(raw)

	failure := patrolRuntimeFailure{
		Title:          "Pulse Patrol: Provider analysis error",
		Summary:        "Provider analysis error",
		Description:    "Pulse Patrol reached the configured provider, but the provider did not complete the Patrol analysis request.",
		Recommendation: "Review the Patrol provider settings, selected model, and provider logs, then rerun Patrol after the provider path is healthy.",
		Detail:         detail,
	}

	switch {
	case strings.Contains(lower, "tool_choice") ||
		strings.Contains(lower, "tool calling") ||
		strings.Contains(lower, "tools are not supported") ||
		strings.Contains(lower, "no endpoints found") && strings.Contains(lower, "tool"):
		failure.Title = "Pulse Patrol: Selected model does not support Patrol tools"
		failure.Summary = "Selected model does not support Patrol tools"
		failure.Description = "Pulse Patrol reached the provider, but the selected model or routed endpoint rejected tool-calling. Patrol needs tool support to inspect resources and report governed findings."
		failure.Recommendation = "Choose a Patrol model or provider route that supports tool calling. For OpenRouter, select an endpoint that supports tools/tool_choice, or switch to a local or BYOK model with tool support."
	case strings.Contains(lower, "model") && (strings.Contains(lower, "not available") ||
		strings.Contains(lower, "not found") ||
		strings.Contains(lower, "does not exist") ||
		strings.Contains(lower, "no such model")):
		failure.Title = "Pulse Patrol: Selected model unavailable"
		failure.Summary = "Selected model unavailable"
		failure.Description = "Pulse Patrol reached the provider, but the configured Patrol model is not available from that provider path."
		failure.Recommendation = "Open Patrol provider settings and choose one of the models currently returned by the provider, then rerun Patrol."
	case isPatrolContextWindowError(err):
		failure.Title = "Pulse Patrol: Selected model context window too small"
		failure.Summary = "Selected model context window too small"
		failure.Description = "The provider rejected Patrol analysis because the selected model could not fit the Patrol context after retrying with smaller context budgets."
		failure.Recommendation = "Choose a model with a larger context window or run a narrower scoped Patrol check."
	case strings.Contains(lower, "insufficient balance") ||
		strings.Contains(lower, "402") ||
		strings.Contains(lower, "payment required") ||
		strings.Contains(lower, "quota") ||
		strings.Contains(lower, "credit"):
		failure.Title = "Pulse Patrol: Provider billing or quota issue"
		failure.Summary = "Provider billing or quota issue"
		failure.Description = "Pulse Patrol cannot analyze your infrastructure because the configured provider rejected the request for billing or quota reasons."
		failure.Recommendation = "Resolve the billing or quota issue with your provider, or switch Patrol to a different provider or local model."
	case strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "429") ||
		strings.Contains(lower, "too many requests"):
		failure.Title = "Pulse Patrol: Provider rate limited"
		failure.Summary = "Provider rate limited"
		failure.Description = "Pulse Patrol is being rate limited by the configured provider, so this analysis run could not complete."
		failure.Recommendation = "Wait for the provider rate limit to reset, increase provider limits, or switch Patrol to another capable model."
	case strings.Contains(lower, "401") ||
		strings.Contains(lower, "403") ||
		strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "forbidden") ||
		strings.Contains(lower, "api key"):
		failure.Title = "Pulse Patrol: Provider authentication issue"
		failure.Summary = "Provider authentication issue"
		failure.Description = "Pulse Patrol cannot analyze your infrastructure because the provider rejected the configured credentials or account access."
		failure.Recommendation = "Check the API key or provider authentication in Patrol provider settings, then rerun Patrol."
	case strings.Contains(lower, "not configured") ||
		strings.Contains(lower, "chat service not available") ||
		strings.Contains(lower, "provider not available") ||
		strings.Contains(lower, "failed to create provider"):
		failure.Title = "Pulse Patrol: Provider not ready"
		failure.Summary = "Provider not ready"
		failure.Description = "Pulse Patrol could not start analysis because the Patrol provider runtime is not ready."
		failure.Recommendation = "Open Patrol provider settings, complete provider configuration, verify the selected model, and rerun Patrol."
	case strings.Contains(lower, "failed to connect") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "timeout"):
		failure.Title = "Pulse Patrol: Provider connection issue"
		failure.Summary = "Provider connection issue"
		failure.Description = "Pulse Patrol could not maintain a healthy connection to the configured provider during analysis."
		failure.Recommendation = "Check provider reachability, base URL, firewall or proxy rules, and provider availability, then rerun Patrol."
	}

	if failure.Detail != "" {
		failure.Evidence = fmt.Sprintf("Provider error: %s", failure.Detail)
	}

	return failure
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
		Recommendation: failure.Recommendation,
		Evidence:       failure.Evidence,
		DetectedAt:     now,
		LastSeenAt:     now,
	}
}
