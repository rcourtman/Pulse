package telemetry

import (
	"net"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Schema v17 closed vocabularies. Every value below is a fixed bucket; the
// receiver canonicalizes anything outside these lists to "unknown", so a
// future sender cannot smuggle free text through these fields.
const (
	// AIProviderClass buckets describe how the Patrol model is reached, never
	// which provider, model, endpoint, or account it is.
	AIProviderClassNone              = "none"
	AIProviderClassLocal             = "local"
	AIProviderClassCloudBYOK         = "cloud_byok"
	AIProviderClassCloudSubscription = "cloud_subscription"
	AIProviderClassHostedLegacy      = "hosted_legacy"
	AIProviderClassUnknown           = "unknown"

	// Patrol 30-day input token buckets. Boundaries are inclusive lower and
	// exclusive upper: under_1m is [1, 1M), 1m_5m is [1M, 5M), and so on.
	PatrolTokenBucketZero       = "zero"
	PatrolInputTokensUnder1M    = "under_1m"
	PatrolInputTokens1M5M       = "1m_5m"
	PatrolInputTokens5M20M      = "5m_20m"
	PatrolInputTokens20MPlus    = "20m_plus"
	PatrolOutputTokensUnder100K = "under_100k"
	PatrolOutputTokens100K500K  = "100k_500k"
	PatrolOutputTokens500K2M    = "500k_2m"
	PatrolOutputTokens2MPlus    = "2m_plus"

	// Investigation outcome buckets partition the findings counted by
	// pulse_intelligence_patrol_investigations_30d: each investigated finding
	// lands in exactly one bucket based on its current investigation state.
	PatrolInvestigationOutcomeFixVerified            = "fix_verified"
	PatrolInvestigationOutcomeFixQueued              = "fix_queued"
	PatrolInvestigationOutcomeFixExecuted            = "fix_executed"
	PatrolInvestigationOutcomeFixRejected            = "fix_rejected"
	PatrolInvestigationOutcomeFixFailed              = "fix_failed"
	PatrolInvestigationOutcomeFixVerificationUnknown = "fix_verification_unknown"
	PatrolInvestigationOutcomeResolved               = "resolved"
	PatrolInvestigationOutcomeNeedsAttention         = "needs_attention"
	PatrolInvestigationOutcomeCannotFix              = "cannot_fix"
	PatrolInvestigationOutcomeTimedOut               = "timed_out"
	PatrolInvestigationOutcomeInProgress             = "in_progress"
	PatrolInvestigationOutcomeFailed                 = "failed"
	PatrolInvestigationOutcomeOther                  = "other"
)

// AIProviderClassValues is the closed export vocabulary for ai_provider_class.
func AIProviderClassValues() []string {
	return []string{
		AIProviderClassNone,
		AIProviderClassLocal,
		AIProviderClassCloudBYOK,
		AIProviderClassCloudSubscription,
		AIProviderClassHostedLegacy,
		AIProviderClassUnknown,
	}
}

// PatrolInputTokenBucketValues is the closed export vocabulary for
// pulse_intelligence_patrol_input_tokens_bucket_30d.
func PatrolInputTokenBucketValues() []string {
	return []string{PatrolTokenBucketZero, PatrolInputTokensUnder1M, PatrolInputTokens1M5M, PatrolInputTokens5M20M, PatrolInputTokens20MPlus}
}

// PatrolOutputTokenBucketValues is the closed export vocabulary for
// pulse_intelligence_patrol_output_tokens_bucket_30d.
func PatrolOutputTokenBucketValues() []string {
	return []string{PatrolTokenBucketZero, PatrolOutputTokensUnder100K, PatrolOutputTokens100K500K, PatrolOutputTokens500K2M, PatrolOutputTokens2MPlus}
}

// PatrolInvestigationOutcomeBucketValues lists every outcome bucket in the
// order the Ping fields are declared.
func PatrolInvestigationOutcomeBucketValues() []string {
	return []string{
		PatrolInvestigationOutcomeFixVerified,
		PatrolInvestigationOutcomeFixQueued,
		PatrolInvestigationOutcomeFixExecuted,
		PatrolInvestigationOutcomeFixRejected,
		PatrolInvestigationOutcomeFixFailed,
		PatrolInvestigationOutcomeFixVerificationUnknown,
		PatrolInvestigationOutcomeResolved,
		PatrolInvestigationOutcomeNeedsAttention,
		PatrolInvestigationOutcomeCannotFix,
		PatrolInvestigationOutcomeTimedOut,
		PatrolInvestigationOutcomeInProgress,
		PatrolInvestigationOutcomeFailed,
		PatrolInvestigationOutcomeOther,
	}
}

// ClassifyAIProviderClass reduces the Patrol model route to one closed bucket.
// It classifies the route, not the credential state: an install that selected
// a cloud model and has not entered a key still reports cloud_byok, and
// pulse_intelligence_patrol_blocked_cause says whether Patrol can run. The
// provider ID, model name, endpoint, and account identity never leave the
// function.
//
//   - none: AI disabled or no default/Patrol model selected.
//   - local: Ollama, or an operator-supplied OpenAI-compatible endpoint whose
//     host is loopback, private-range, link-local, unqualified, or a
//     .local/.lan/.internal style name (llama.cpp, LM Studio, LocalAI, vLLM).
//   - cloud_byok: any hosted provider reached with the operator's own key,
//     including a custom OpenAI-compatible endpoint on a public host.
//   - cloud_subscription: the locally authenticated Codex or Claude CLI
//     subscription routes, which carry no per-token bill.
//   - hosted_legacy: the retired Pulse-hosted route still selected in a
//     legacy config that has not been normalized yet.
func ClassifyAIProviderClass(cfg *config.AIConfig) string {
	if cfg == nil || !cfg.Enabled {
		return AIProviderClassNone
	}
	rawModel := strings.TrimSpace(cfg.PatrolModel)
	if rawModel == "" {
		rawModel = strings.TrimSpace(cfg.Model)
	}
	if rawModel == "" {
		return AIProviderClassNone
	}
	model := config.NormalizeQuickstartModelString(rawModel)
	if model == "" {
		return AIProviderClassHostedLegacy
	}
	provider, _ := config.ParseModelString(model)
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case config.AIProviderOllama:
		return AIProviderClassLocal
	case config.AIProviderCodexSubscription, config.AIProviderClaudeSubscription:
		return AIProviderClassCloudSubscription
	case config.AIProviderOpenAI:
		if config.IsCustomOpenAICompatibleEndpoint(cfg.OpenAIBaseURL) && endpointHostIsLocal(cfg.OpenAIBaseURL) {
			return AIProviderClassLocal
		}
		return AIProviderClassCloudBYOK
	case config.AIProviderQuickstart:
		return AIProviderClassHostedLegacy
	case "":
		return AIProviderClassUnknown
	}
	if _, ok := config.LookupAIProviderDefinition(provider); ok {
		return AIProviderClassCloudBYOK
	}
	return AIProviderClassUnknown
}

// endpointHostIsLocal reports whether a configured base URL points at a host
// an operator would run themselves. It is a syntactic check only; telemetry
// must never resolve DNS or open a connection to classify a route.
func endpointHostIsLocal(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
	}
	if host == "localhost" || !strings.Contains(host, ".") {
		return true
	}
	for _, suffix := range []string{".local", ".localdomain", ".lan", ".internal", ".home", ".home.arpa", ".localhost"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// PatrolInputTokensBucket maps a 30-day Patrol input token total to its
// closed bucket. Negative totals are treated as zero.
func PatrolInputTokensBucket(total int64) string {
	switch {
	case total <= 0:
		return PatrolTokenBucketZero
	case total < 1_000_000:
		return PatrolInputTokensUnder1M
	case total < 5_000_000:
		return PatrolInputTokens1M5M
	case total < 20_000_000:
		return PatrolInputTokens5M20M
	default:
		return PatrolInputTokens20MPlus
	}
}

// PatrolOutputTokensBucket maps a 30-day Patrol output token total to its
// closed bucket. Output volume runs roughly an order of magnitude below input,
// so the ladder is finer than the input one.
func PatrolOutputTokensBucket(total int64) string {
	switch {
	case total <= 0:
		return PatrolTokenBucketZero
	case total < 100_000:
		return PatrolOutputTokensUnder100K
	case total < 500_000:
		return PatrolOutputTokens100K500K
	case total < 2_000_000:
		return PatrolOutputTokens500K2M
	default:
		return PatrolOutputTokens2MPlus
	}
}

// NormalizePatrolAutonomyLevelForTelemetry bounds the effective Patrol
// autonomy level to the four released modes. Anything else, including an empty
// value from an install without an AI service, reports monitor, which is also
// the runtime's own fallback.
func NormalizePatrolAutonomyLevelForTelemetry(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case config.PatrolAutonomyApproval:
		return config.PatrolAutonomyApproval
	case config.PatrolAutonomyAssisted:
		return config.PatrolAutonomyAssisted
	case config.PatrolAutonomyFull:
		return config.PatrolAutonomyFull
	default:
		return config.PatrolAutonomyMonitor
	}
}

// PatrolInvestigationOutcomeBucket reduces a finding's investigation outcome
// and status to one closed bucket. The outcome wins when present; a finding
// with no recorded outcome is bucketed by whether its investigation is still
// running, errored out, or finished without a typed outcome.
func PatrolInvestigationOutcomeBucket(outcome, status string) string {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "fix_verified":
		return PatrolInvestigationOutcomeFixVerified
	case "fix_queued":
		return PatrolInvestigationOutcomeFixQueued
	case "fix_executed":
		return PatrolInvestigationOutcomeFixExecuted
	case "fix_rejected":
		return PatrolInvestigationOutcomeFixRejected
	case "fix_failed", "fix_verification_failed":
		return PatrolInvestigationOutcomeFixFailed
	case "fix_verification_unknown":
		return PatrolInvestigationOutcomeFixVerificationUnknown
	case "resolved":
		return PatrolInvestigationOutcomeResolved
	case "needs_attention":
		return PatrolInvestigationOutcomeNeedsAttention
	case "cannot_fix":
		return PatrolInvestigationOutcomeCannotFix
	case "timed_out":
		return PatrolInvestigationOutcomeTimedOut
	case "":
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "pending", "running":
			return PatrolInvestigationOutcomeInProgress
		case "failed":
			return PatrolInvestigationOutcomeFailed
		case "needs_attention":
			return PatrolInvestigationOutcomeNeedsAttention
		default:
			return PatrolInvestigationOutcomeOther
		}
	default:
		return PatrolInvestigationOutcomeOther
	}
}

// PatrolInvestigationOutcomeCounts is the count-only partition of investigated
// findings by outcome bucket.
type PatrolInvestigationOutcomeCounts struct {
	FixVerified            int
	FixQueued              int
	FixExecuted            int
	FixRejected            int
	FixFailed              int
	FixVerificationUnknown int
	Resolved               int
	NeedsAttention         int
	CannotFix              int
	TimedOut               int
	InProgress             int
	Failed                 int
	Other                  int
}

// Add increments the bucket for one investigated finding.
func (c *PatrolInvestigationOutcomeCounts) Add(outcome, status string) {
	if c == nil {
		return
	}
	switch PatrolInvestigationOutcomeBucket(outcome, status) {
	case PatrolInvestigationOutcomeFixVerified:
		c.FixVerified++
	case PatrolInvestigationOutcomeFixQueued:
		c.FixQueued++
	case PatrolInvestigationOutcomeFixExecuted:
		c.FixExecuted++
	case PatrolInvestigationOutcomeFixRejected:
		c.FixRejected++
	case PatrolInvestigationOutcomeFixFailed:
		c.FixFailed++
	case PatrolInvestigationOutcomeFixVerificationUnknown:
		c.FixVerificationUnknown++
	case PatrolInvestigationOutcomeResolved:
		c.Resolved++
	case PatrolInvestigationOutcomeNeedsAttention:
		c.NeedsAttention++
	case PatrolInvestigationOutcomeCannotFix:
		c.CannotFix++
	case PatrolInvestigationOutcomeTimedOut:
		c.TimedOut++
	case PatrolInvestigationOutcomeInProgress:
		c.InProgress++
	case PatrolInvestigationOutcomeFailed:
		c.Failed++
	default:
		c.Other++
	}
}

// Total returns the number of investigated findings partitioned so far.
func (c PatrolInvestigationOutcomeCounts) Total() int {
	return c.FixVerified + c.FixQueued + c.FixExecuted + c.FixRejected + c.FixFailed +
		c.FixVerificationUnknown + c.Resolved + c.NeedsAttention + c.CannotFix +
		c.TimedOut + c.InProgress + c.Failed + c.Other
}
