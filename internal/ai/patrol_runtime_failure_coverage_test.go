package ai

import (
	"errors"
	"testing"
)

// TestClassifyProviderConnectionFailure pins every reachable switch arm in
// ClassifyProviderConnectionFailure (plus the implicit default) to its exact
// diagnostic output. Each input error is constructed so that
// patrolRuntimeFailureFromError assigns the PatrolFailureCause that drives
// the corresponding case label, ensuring no arm is left unexercised.
func TestClassifyProviderConnectionFailure(t *testing.T) {
	cases := []struct {
		name               string
		err                error
		wantCause          PatrolFailureCause
		wantTitle          string
		wantDescription    string
		wantRecommendation string
	}{
		{
			name:               "malformed_tool_history",
			err:                errors.New("An assistant message with 'tool_calls' must be followed by tool messages responding to each 'tool_call_id'"),
			wantCause:          PatrolFailureCauseMalformedToolHistory,
			wantTitle:          "Provider conversation state issue",
			wantDescription:    "The provider rejected the conversation structure used by Pulse.",
			wantRecommendation: "Start a new assistant session and retry. If the issue persists, restart Pulse and report the selected provider and model.",
		},
		{
			name:               "tool_choice_rejected",
			err:                errors.New("deepseek-reasoner does not support this tool_choice"),
			wantCause:          PatrolFailureCauseToolChoiceRejected,
			wantTitle:          "Provider rejected tool-choice request",
			wantDescription:    "Pulse reached the provider, but the provider rejected a tool-choice transport setting.",
			wantRecommendation: "Retry with automatic tool selection, or switch to a provider route with reliable tool-call support.",
		},
		{
			name:               "no_tool_capable_endpoint",
			err:                errors.New("No endpoints found that support tool use"),
			wantCause:          PatrolFailureCauseNoToolCapableEndpoint,
			wantTitle:          "No tool-capable provider endpoint available",
			wantDescription:    "Pulse reached the provider, but the provider reports no available endpoint with tool support for the selected model.",
			wantRecommendation: "Review provider routing and privacy filters, broaden allowed providers, or switch to a model with broader tool support.",
		},
		{
			name:               "model_unsupported_tools",
			err:                errors.New("tools are not supported by this model family"),
			wantCause:          PatrolFailureCauseModelUnsupportedTools,
			wantTitle:          "Selected model does not support tools",
			wantDescription:    "Pulse reached the provider, but the selected model or routed endpoint rejected tool calling.",
			wantRecommendation: "Choose a model or provider route that supports tool calling for governed Assistant and Patrol workflows.",
		},
		{
			name:               "model_unavailable",
			err:                errors.New(`model "qwen3.5:2b" is not available`),
			wantCause:          PatrolFailureCauseModelUnavailable,
			wantTitle:          "Selected model unavailable",
			wantDescription:    "The selected model is not available from this provider path.",
			wantRecommendation: "Choose one of the models currently returned by the provider, then retry.",
		},
		{
			name:               "context_window_too_small",
			err:                errors.New("maximum context length exceeded"),
			wantCause:          PatrolFailureCauseContextWindowTooSmall,
			wantTitle:          "Selected model context window too small",
			wantDescription:    "The provider rejected the request because the selected model could not fit the current context.",
			wantRecommendation: "Choose a model with a larger context window or retry with a narrower request.",
		},
		{
			name:               "provider_billing",
			err:                errors.New("insufficient balance"),
			wantCause:          PatrolFailureCauseProviderBilling,
			wantTitle:          "Provider billing or quota issue",
			wantDescription:    "The provider rejected the request for billing or quota reasons.",
			wantRecommendation: "Resolve the billing or quota issue with your provider, or switch to a different provider or model.",
		},
		{
			name:               "provider_rate_limited",
			err:                errors.New("rate limit exceeded"),
			wantCause:          PatrolFailureCauseProviderRateLimited,
			wantTitle:          "Provider rate limited",
			wantDescription:    "The provider is rate limiting requests for this account or model.",
			wantRecommendation: "Wait for the provider rate limit to reset, increase provider limits, or switch to another model.",
		},
		{
			name:               "provider_auth",
			err:                errors.New("401 unauthorized"),
			wantCause:          PatrolFailureCauseProviderAuth,
			wantTitle:          "Provider authentication issue",
			wantDescription:    "The provider rejected the configured credentials or account access.",
			wantRecommendation: "Check the API key or provider authentication on the Provider & Models settings page, then retry.",
		},
		{
			name:               "provider_not_ready_grouped_arm",
			err:                errors.New("provider not configured"),
			wantCause:          PatrolFailureCauseProviderNotConfigured,
			wantTitle:          "Provider not ready",
			wantDescription:    "Pulse cannot test this provider because the provider runtime is not ready.",
			wantRecommendation: "Open the Provider & Models settings page, complete provider configuration, verify the selected model, and retry.",
		},
		{
			name:               "default_provider_connection",
			err:                errors.New("failed to connect: i/o timeout"),
			wantCause:          PatrolFailureCauseProviderConnection,
			wantTitle:          "Provider connection issue",
			wantDescription:    "Pulse could not maintain a healthy connection to this provider.",
			wantRecommendation: "Check provider reachability, base URL, firewall or proxy rules, and provider availability, then retry.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diagnostic := ClassifyProviderConnectionFailure(tc.err)

			if diagnostic.Cause != tc.wantCause {
				t.Errorf("Cause = %q, want %q", diagnostic.Cause, tc.wantCause)
			}
			if diagnostic.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", diagnostic.Title, tc.wantTitle)
			}
			if diagnostic.Summary != tc.wantTitle {
				t.Errorf("Summary = %q, want %q (Summary must equal Title)", diagnostic.Summary, tc.wantTitle)
			}
			if diagnostic.Description != tc.wantDescription {
				t.Errorf("Description = %q, want %q", diagnostic.Description, tc.wantDescription)
			}
			if diagnostic.Recommendation != tc.wantRecommendation {
				t.Errorf("Recommendation = %q, want %q", diagnostic.Recommendation, tc.wantRecommendation)
			}
		})
	}
}

// TestPatrolRuntimeFailureFromError_NilErrorCoverage closes the remaining
// branch in patrolRuntimeFailureFromError: a nil error leaves raw empty, so
// no switch arm matches and Detail/Evidence stay empty (the
// `if failure.Detail != ""` guard on Evidence is never entered).
func TestPatrolRuntimeFailureFromError_NilErrorCoverage(t *testing.T) {
	failure := patrolRuntimeFailureFromError(nil)

	if failure.Title != "Pulse Patrol: Provider analysis error" {
		t.Fatalf("Title = %q, want %q", failure.Title, "Pulse Patrol: Provider analysis error")
	}
	if failure.Summary != "Provider analysis error" {
		t.Fatalf("Summary = %q, want %q", failure.Summary, "Provider analysis error")
	}
	if failure.Cause != PatrolFailureCauseProviderConnection {
		t.Fatalf("Cause = %q, want %q", failure.Cause, PatrolFailureCauseProviderConnection)
	}
	if failure.Detail != "" {
		t.Fatalf("Detail = %q, want empty for nil error", failure.Detail)
	}
	if failure.Evidence != "" {
		t.Fatalf("Evidence = %q, want empty when Detail is empty", failure.Evidence)
	}
}

// TestSummarizePatrolRuntimeFailureDetail closes the remaining ~6% gap by
// exercising the empty/whitespace early-return branch, and pins exact output
// for the rate-limit switch arm and the default redaction path.
func TestSummarizePatrolRuntimeFailureDetail(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "empty_string_returns_empty",
			raw:  "",
			want: "",
		},
		{
			name: "whitespace_only_returns_empty",
			raw:  "   \n\t  ",
			want: "",
		},
		{
			name: "rate_limit_arm_exact_output",
			raw:  "API error (429): rate limit exceeded",
			want: "Provider rate limit reached. Wait for capacity or adjust provider limits before retrying.",
		},
		{
			name: "default_path_redacts_secret_token",
			raw:  "upstream errored: sk-abcd1234efgh5678",
			want: "upstream errored: [redacted-secret]",
		},
		{
			name: "default_path_passthrough_trimmed",
			raw:  "  upstream returned unexpected eof  ",
			want: "upstream returned unexpected eof",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizePatrolRuntimeFailureDetail(tc.raw)
			if got != tc.want {
				t.Fatalf("summarizePatrolRuntimeFailureDetail(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
