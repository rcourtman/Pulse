package ai

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func findGuidance(t *testing.T, provider, model string) []PatrolModelGuidanceRule {
	t.Helper()
	var matched []PatrolModelGuidanceRule
	for _, rule := range StaticPatrolModelGuidance() {
		if MatchesPatrolModelGuidance(rule, provider, model) {
			matched = append(matched, rule)
		}
	}
	return matched
}

func TestStaticPatrolModelGuidance_OllamaBlessingIsRecommendedWithHardwareNote(t *testing.T) {
	rules := findGuidance(t, config.AIProviderOllama, config.OllamaSuggestedPatrolModel)
	if len(rules) != 1 || rules[0].Level != PatrolModelGuidanceRecommended {
		t.Fatalf("expected one recommended rule for the Ollama blessing, got %+v", rules)
	}
	if !strings.Contains(rules[0].Reason, "8 GB") || !strings.Contains(rules[0].Reason, "no per-token bill") {
		t.Fatalf("reason should carry the hardware note and the no-bill point: %q", rules[0].Reason)
	}
	if len(findGuidance(t, config.AIProviderOllama, "qwen3:4b")) != 0 {
		t.Fatal("smaller qwen3 tags failed preflight and must not inherit the blessing")
	}
}

func TestStaticPatrolModelGuidance_FlashLiteIsCautionAndFlashIsSuggested(t *testing.T) {
	lite := findGuidance(t, config.AIProviderGemini, "gemini-2.5-flash-lite")
	if len(lite) != 1 || lite[0].Level != PatrolModelGuidanceCaution {
		t.Fatalf("expected a single caution rule for flash-lite, got %+v", lite)
	}
	if !strings.Contains(lite[0].Reason, "verdicts") {
		t.Fatalf("caution should say what failed: %q", lite[0].Reason)
	}
	flash := findGuidance(t, config.AIProviderGemini, "gemini-2.5-flash")
	if len(flash) != 1 || flash[0].Level != PatrolModelGuidanceSuggested {
		t.Fatalf("expected flash to be suggested, got %+v", flash)
	}
	if !strings.Contains(flash[0].Reason, "$0.04") || !strings.Contains(flash[0].Reason, "Not yet qualified") {
		t.Fatalf("suggested reason should show the per-run price and the qualification caveat: %q", flash[0].Reason)
	}
	if len(findGuidance(t, config.AIProviderGemini, "gemini-2.5-flash-image")) != 0 {
		t.Fatal("image and live variants are not Patrol starting points")
	}
}

func TestStaticPatrolModelGuidance_CloudStartingPointsMatchDatedIDs(t *testing.T) {
	haiku := findGuidance(t, config.AIProviderAnthropic, "claude-haiku-4-5-20251001")
	if len(haiku) != 1 || haiku[0].Level != PatrolModelGuidanceSuggested {
		t.Fatalf("expected dated haiku id to match the prefix rule, got %+v", haiku)
	}
	if len(findGuidance(t, config.AIProviderAnthropic, "claude-opus-5")) != 0 {
		t.Fatal("opus carries no guidance: price alone is not a caution")
	}
	if len(findGuidance(t, config.AIProviderOpenAI, "gpt-4o-mini")) != 1 {
		t.Fatal("expected gpt-4o-mini starting point")
	}
	if len(findGuidance(t, "openai", "GPT-4O")) != 0 {
		t.Fatal("full gpt-4o is not the lowest-cost tier")
	}
}

func TestPatrolModelGuidance_VerifiedRequiresCachedPassForConfiguredModel(t *testing.T) {
	svc := NewService(nil, nil)
	response := svc.PatrolModelGuidance()
	if response.Verified != nil {
		t.Fatal("no cached readiness: verified marker must be absent")
	}
	if len(response.Rules) == 0 {
		t.Fatal("static rules must always be present")
	}
	var nilService *Service
	if got := nilService.PatrolModelGuidance(); len(got.Rules) == 0 || got.Verified != nil {
		t.Fatalf("nil service should still return static rules, got %+v", got)
	}
}
