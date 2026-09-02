package ai

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Patrol model guidance answers the question every prospect asks before
// trying Patrol: "which model should I pick?" (support, 2026-07-30). Pulse
// has one qualified answer today (the Ollama blessing verified by
// RunPatrolToolPreflight) and one verified failure (Gemini Flash-Lite could
// not file per-finding verdicts, support 2026-07-20). Everything else here
// is a labelled starting point chosen by price, not a qualification claim:
// the per-install "Check Patrol model" run is what turns a suggestion into
// evidence, and its cached pass is surfaced as the strongest marker.
const (
	// PatrolModelGuidanceVerified: this install's own readiness check passed.
	PatrolModelGuidanceVerified = "verified"
	// PatrolModelGuidanceRecommended: passed Pulse's Patrol preflight.
	PatrolModelGuidanceRecommended = "recommended"
	// PatrolModelGuidanceSuggested: cheapest standard tier on Pulse's price
	// list for the provider; not yet qualified by Pulse.
	PatrolModelGuidanceSuggested = "suggested"
	// PatrolModelGuidanceCaution: known to fail or drop Patrol tool calls.
	PatrolModelGuidanceCaution = "caution"
)

// PatrolModelGuidanceRule matches models by provider and id prefix so it
// survives dated model ids (claude-haiku-4-5-20251001) and previews.
// Exclude entries are forbidden substrings; an entry starting with "!" is a
// required substring instead.
type PatrolModelGuidanceRule struct {
	Provider    string   `json:"provider"`
	ModelPrefix string   `json:"model_prefix"`
	ModelExact  bool     `json:"model_exact,omitempty"`
	Exclude     []string `json:"exclude,omitempty"`
	Level       string   `json:"level"`
	Reason      string   `json:"reason"`
	Evidence    string   `json:"evidence,omitempty"`
}

// PatrolModelVerifiedGuidance is the cached readiness pass for the model
// this install currently runs Patrol on.
type PatrolModelVerifiedGuidance struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	MaxVerifiedMode string `json:"max_verified_mode,omitempty"`
	RecordedAtUnix  int64  `json:"recorded_at_unix"`
}

// PatrolModelGuidanceResponse is the payload behind the model picker markers.
type PatrolModelGuidanceResponse struct {
	Rules    []PatrolModelGuidanceRule    `json:"rules"`
	Verified *PatrolModelVerifiedGuidance `json:"verified,omitempty"`
}

func patrolFullRunCostSentence(provider, model string) string {
	usd, ok, _ := cost.EstimateUSD(provider, model, DefaultPatrolRunInputTokens, DefaultPatrolRunOutputTokens)
	if !ok {
		return ""
	}
	if usd == 0 {
		return "No per-token charge on Pulse's price list."
	}
	if usd < 0.01 {
		return "Under a cent per full Patrol run on Pulse's price list."
	}
	return fmt.Sprintf("About $%.2f per full Patrol run on Pulse's price list.", usd)
}

func patrolSuggestedGuidance(provider, prefix string, exclude ...string) PatrolModelGuidanceRule {
	return PatrolModelGuidanceRule{
		Provider:    provider,
		ModelPrefix: prefix,
		Exclude:     exclude,
		Level:       PatrolModelGuidanceSuggested,
		Reason: strings.TrimSpace(fmt.Sprintf(
			"Lowest-cost standard tier for this provider. %s Not yet qualified by Pulse: run Check Patrol model before relying on it.",
			patrolFullRunCostSentence(provider, prefix))),
		Evidence: "price-table",
	}
}

// StaticPatrolModelGuidance returns the maintained guidance table. Cloud
// entries are price-driven starting points and say so; only the Ollama
// blessing and the Flash-Lite caution carry evidence.
func StaticPatrolModelGuidance() []PatrolModelGuidanceRule {
	ollamaReason := "Passed Pulse's Patrol tool-call check and runs on your own hardware, so there is no per-token bill."
	var ollamaEquivalents []string
	if def, ok := config.LookupAIProviderDefinition(config.AIProviderOllama); ok {
		if note := strings.TrimSpace(def.SuggestedModelNote); note != "" {
			ollamaReason += " " + note
		}
		ollamaEquivalents = def.SuggestedModelEquivalents
	}
	rules := []PatrolModelGuidanceRule{
		{
			Provider:    config.AIProviderOllama,
			ModelPrefix: config.OllamaSuggestedPatrolModel,
			ModelExact:  true,
			Level:       PatrolModelGuidanceRecommended,
			Reason:      ollamaReason,
			Evidence:    "patrol-preflight",
		},
	}
	for _, equivalent := range ollamaEquivalents {
		rules = append(rules, PatrolModelGuidanceRule{
			Provider:    config.AIProviderOllama,
			ModelPrefix: equivalent,
			ModelExact:  true,
			Level:       PatrolModelGuidanceRecommended,
			Reason:      ollamaReason,
			Evidence:    "patrol-preflight",
		})
	}
	rules = append(rules,
		PatrolModelGuidanceRule{
			Provider:    config.AIProviderGemini,
			ModelPrefix: "gemini-",
			Exclude:     []string{"!flash-lite"},
			Level:       PatrolModelGuidanceCaution,
			Reason:      "Flash-Lite could not file Patrol's per-finding verdicts, so every run on a Pro install ended in an assessment error. Pick a standard Flash or Pro tier instead.",
			Evidence:    "support-2026-07-20",
		},
		patrolSuggestedGuidance(config.AIProviderGemini, "gemini-2.5-flash", "lite", "image", "live"),
		patrolSuggestedGuidance(config.AIProviderAnthropic, "claude-haiku-4-5"),
		patrolSuggestedGuidance(config.AIProviderOpenAI, "gpt-4o-mini"),
		patrolSuggestedGuidance(config.AIProviderDeepSeek, config.DeepSeekModelV4Flash),
		patrolSuggestedGuidance(config.AIProviderOpenRouter, "deepseek/deepseek-v4-flash"),
	)
	return rules
}

// MatchesPatrolModelGuidance reports whether a rule applies to a model id.
func MatchesPatrolModelGuidance(rule PatrolModelGuidanceRule, provider, model string) bool {
	if !strings.EqualFold(strings.TrimSpace(provider), rule.Provider) {
		return false
	}
	candidate := strings.ToLower(strings.TrimSpace(model))
	prefix := strings.ToLower(rule.ModelPrefix)
	if rule.ModelExact {
		if candidate != prefix {
			return false
		}
	} else if !strings.HasPrefix(candidate, prefix) {
		return false
	}
	for _, needle := range rule.Exclude {
		lower := strings.ToLower(needle)
		if strings.HasPrefix(lower, "!") {
			if !strings.Contains(candidate, strings.TrimPrefix(lower, "!")) {
				return false
			}
			continue
		}
		if strings.Contains(candidate, lower) {
			return false
		}
	}
	return true
}

// PatrolModelGuidance combines the static table with this install's cached
// readiness pass for the configured Patrol model.
func (s *Service) PatrolModelGuidance() PatrolModelGuidanceResponse {
	response := PatrolModelGuidanceResponse{Rules: StaticPatrolModelGuidance()}
	if s == nil {
		return response
	}
	result, recordedAt := s.CachedPatrolModelReadiness()
	if result != nil && result.Status == PatrolModelReadinessPass && result.PatrolCapable {
		response.Verified = &PatrolModelVerifiedGuidance{
			Provider:        result.Provider,
			Model:           result.Model,
			MaxVerifiedMode: result.MaxVerifiedMode,
			RecordedAtUnix:  recordedAt.Unix(),
		}
	}
	return response
}
