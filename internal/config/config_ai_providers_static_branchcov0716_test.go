package config

import (
	"reflect"
	"testing"
)

// These tests pin the static provider-registry accessors in ai_providers.go
// (SuggestedModelForProvider, SuggestedModelIDSet, IsOpenAICompatibleProvider,
// AIProviderFallbackModels, AIProviderDisplayName) which were previously
// uncovered. They exercise the not-found / empty / normalization branches each
// helper inherits from LookupAIProviderDefinition, plus the per-protocol and
// per-catalog arms of the registry data.

func TestBranchCovSuggestedModelForProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     string
	}{
		// Found-provider arm with a real blessing.
		{name: "ollama returns blessed patrol model", provider: AIProviderOllama, want: OllamaSuggestedPatrolModel},
		// LookupAIProviderDefinition normalizes via ToLower(TrimSpace); the
		// suggestion must still resolve through that normalization.
		{name: "ollama id case-insensitive", provider: "OLLAMA", want: OllamaSuggestedPatrolModel},
		{name: "ollama id trims surrounding whitespace", provider: "  ollama ", want: OllamaSuggestedPatrolModel},
		// Found-provider arm where the registry deliberately sets no blessing.
		{name: "anthropic has no suggestion", provider: AIProviderAnthropic, want: ""},
		{name: "openai has no suggestion", provider: AIProviderOpenAI, want: ""},
		{name: "retired quickstart has no suggestion", provider: AIProviderQuickstart, want: ""},
		// Not-found arm: empty, whitespace-only, unknown, and a lookalike id
		// that must NOT be treated as a fuzzy match.
		{name: "empty provider returns empty", provider: "", want: ""},
		{name: "whitespace-only provider returns empty", provider: "   ", want: ""},
		{name: "unknown provider returns empty", provider: "no-such-provider", want: ""},
		{name: "lookalike id with trailing colon rejected", provider: "ollama:", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SuggestedModelForProvider(tt.provider); got != tt.want {
				t.Fatalf("SuggestedModelForProvider(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestBranchCovSuggestedModelIDSet(t *testing.T) {
	set := SuggestedModelIDSet()
	if set == nil {
		t.Fatal("SuggestedModelIDSet returned a nil map")
	}

	// Only the Ollama provider contributes a SuggestedModel plus equivalents;
	// every other provider's empty suggestion is skipped by the `if id != ""`
	// guard, which is the branch under test here.
	want := map[string]struct{}{
		OllamaSuggestedPatrolModel: {},
		"qwen3:latest":             {},
	}
	if !reflect.DeepEqual(set, want) {
		t.Fatalf("SuggestedModelIDSet() = %v, want exactly %v (ollama blessing + equivalents only)", set, want)
	}

	// Guard against the suggested set being confused with the fallback catalog:
	// Z.ai exposes "glm-5.2" as a fallback model but has no Patrol blessing, so
	// it must be absent from the suggestion set.
	if _, ok := set["glm-5.2"]; ok {
		t.Fatal(`fallback-catalog model "glm-5.2" leaked into the suggested-model set`)
	}
}

func TestBranchCovIsOpenAICompatibleProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     bool
	}{
		// OpenAI-compatible protocol arm (ok && protocol == openai_compatible).
		{name: "openai", provider: AIProviderOpenAI, want: true},
		{name: "openrouter gateway", provider: AIProviderOpenRouter, want: true},
		{name: "deepseek", provider: AIProviderDeepSeek, want: true},
		{name: "zai", provider: AIProviderZai, want: true},
		{name: "groq", provider: AIProviderGroq, want: true},
		{name: "mistral", provider: AIProviderMistral, want: true},
		{name: "cerebras", provider: AIProviderCerebras, want: true},
		{name: "together", provider: AIProviderTogether, want: true},
		{name: "fireworks", provider: AIProviderFireworks, want: true},
		{name: "openai id case-insensitive", provider: "OpenAI", want: true},

		// Known provider with a non-compatible protocol arm
		// (ok && protocol != openai_compatible).
		{name: "anthropic native protocol", provider: AIProviderAnthropic, want: false},
		{name: "gemini native protocol", provider: AIProviderGemini, want: false},
		{name: "ollama native protocol", provider: AIProviderOllama, want: false},
		{name: "codex subscription agent", provider: AIProviderCodexSubscription, want: false},
		{name: "claude subscription agent", provider: AIProviderClaudeSubscription, want: false},
		{name: "retired quickstart marker", provider: AIProviderQuickstart, want: false},

		// Not-found arm (ok == false short-circuits the protocol check).
		{name: "unknown provider", provider: "no-such", want: false},
		{name: "empty provider", provider: "", want: false},
		{name: "whitespace-only provider", provider: "  ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOpenAICompatibleProvider(tt.provider); got != tt.want {
				t.Fatalf("IsOpenAICompatibleProvider(%q) = %v, want %v", tt.provider, got, tt.want)
			}
		})
	}
}

func TestBranchCovAIProviderFallbackModels(t *testing.T) {
	t.Run("unknown provider returns nil", func(t *testing.T) {
		if got := AIProviderFallbackModels("no-such"); got != nil {
			t.Fatalf("AIProviderFallbackModels(unknown) = %#v, want nil", got)
		}
	})

	t.Run("empty provider returns nil", func(t *testing.T) {
		if got := AIProviderFallbackModels(""); got != nil {
			t.Fatalf(`AIProviderFallbackModels("") = %#v, want nil`, got)
		}
	})

	t.Run("whitespace-only provider returns nil", func(t *testing.T) {
		if got := AIProviderFallbackModels("   "); got != nil {
			t.Fatalf(`AIProviderFallbackModels("   ") = %#v, want nil`, got)
		}
	})

	// Known-provider-but-no-catalog arm: the early return fires because
	// len(def.FallbackModels) == 0 even though the lookup succeeded.
	t.Run("anthropic has no static fallback catalog", func(t *testing.T) {
		if got := AIProviderFallbackModels(AIProviderAnthropic); got != nil {
			t.Fatalf("AIProviderFallbackModels(%q) = %#v, want nil", AIProviderAnthropic, got)
		}
	})
	t.Run("ollama has no static fallback catalog", func(t *testing.T) {
		if got := AIProviderFallbackModels(AIProviderOllama); got != nil {
			t.Fatalf("AIProviderFallbackModels(%q) = %#v, want nil", AIProviderOllama, got)
		}
	})
	t.Run("openrouter relies on gateway catalog not static list", func(t *testing.T) {
		if got := AIProviderFallbackModels(AIProviderOpenRouter); got != nil {
			t.Fatalf("AIProviderFallbackModels(%q) = %#v, want nil", AIProviderOpenRouter, got)
		}
	})

	// Catalog arm: returns a non-nil copy whose contents match the registry.
	t.Run("deepseek returns four-entry fallback catalog copy", func(t *testing.T) {
		got := AIProviderFallbackModels(AIProviderDeepSeek)
		if got == nil {
			t.Fatal("expected a non-nil fallback catalog for deepseek")
		}
		if len(got) != 4 {
			t.Fatalf("deepseek fallback count = %d, want 4 (%#v)", len(got), got)
		}
		if got[0].ID != DeepSeekModelV4Flash {
			t.Fatalf("first deepseek fallback = %q, want %q", got[0].ID, DeepSeekModelV4Flash)
		}
		// The two current V4 entries are flagged Notable; the two legacy
		// aliases are not. Pinning this guards the catalog's shape.
		notable := 0
		for _, m := range got {
			if m.Notable {
				notable++
			}
		}
		if notable != 2 {
			t.Fatalf("deepseek notable fallback count = %d, want 2", notable)
		}
	})

	t.Run("zai returns single fallback entry", func(t *testing.T) {
		got := AIProviderFallbackModels(AIProviderZai)
		if got == nil || len(got) != 1 {
			t.Fatalf("zai fallback = %#v, want exactly one entry", got)
		}
		if got[0].ID != "glm-5.2" || !got[0].Notable {
			t.Fatalf("zai fallback[0] = %#v, want {ID:glm-5.2 Notable:true}", got[0])
		}
	})

	t.Run("claude subscription returns two alias entries", func(t *testing.T) {
		got := AIProviderFallbackModels(AIProviderClaudeSubscription)
		if got == nil || len(got) != 2 {
			t.Fatalf("claude subscription fallback = %#v, want exactly two entries", got)
		}
	})

	t.Run("codex subscription returns single fallback entry", func(t *testing.T) {
		got := AIProviderFallbackModels(AIProviderCodexSubscription)
		if got == nil || len(got) != 1 {
			t.Fatalf("codex subscription fallback = %#v, want exactly one entry", got)
		}
	})

	// The accessor must hand back an independent copy on every call so that
	// callers cannot mutate the canonical registry records.
	t.Run("returned slice is an independent copy", func(t *testing.T) {
		a := AIProviderFallbackModels(AIProviderDeepSeek)
		b := AIProviderFallbackModels(AIProviderDeepSeek)
		if len(a) != 4 || len(b) != 4 {
			t.Fatalf("precondition: expected 4 deepseek fallbacks, got a=%d b=%d", len(a), len(b))
		}

		originalFirst := a[0].ID
		// Grow one returned slice and rewrite an element field on the other.
		b = append(b, AIProviderModelDefinition{ID: "injected"})
		a[0] = AIProviderModelDefinition{ID: "mutated"}

		if len(a) != 4 {
			t.Fatalf("appending to b leaked into a: len(a) = %d, want 4", len(a))
		}
		if a[0].ID != "mutated" {
			t.Fatalf("a[0].ID = %q, want \"mutated\" (independent element expected)", a[0].ID)
		}

		// A fresh lookup must reflect neither mutation.
		fresh := AIProviderFallbackModels(AIProviderDeepSeek)
		if len(fresh) != 4 || fresh[0].ID != originalFirst {
			t.Fatalf("mutation poisoned the canonical record: fresh = %#v, want first id %q", fresh, originalFirst)
		}
	})
}

func TestBranchCovAIProviderDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     string
	}{
		// Found-provider arm: returns the registry DisplayName, never the raw
		// input id.
		{name: "anthropic display name", provider: AIProviderAnthropic, want: "Anthropic"},
		{name: "openrouter display name", provider: AIProviderOpenRouter, want: "OpenRouter"},
		{name: "zai display name", provider: AIProviderZai, want: "Z.ai"},
		{name: "codex subscription display name", provider: AIProviderCodexSubscription, want: "Codex subscription (local)"},
		{name: "retired quickstart keeps display name", provider: AIProviderQuickstart, want: "Pulse hosted quickstart"},
		// Lookup normalizes case/whitespace before resolving the id, so the
		// canonical DisplayName still comes back.
		{name: "lookup is case-insensitive", provider: "ANTHROPIC", want: "Anthropic"},
		{name: "lookup trims surrounding whitespace", provider: "  anthropic ", want: "Anthropic"},

		// Not-found arm: the raw input is returned VERBATIM (it is not
		// normalized). This pins the current asymmetric behaviour.
		{name: "unknown provider returns input verbatim", provider: "WeirdThing", want: "WeirdThing"},
		{name: "unknown mixed-case input preserved", provider: "SomeCloud", want: "SomeCloud"},
		{name: "empty input returns empty", provider: "", want: ""},
		{name: "whitespace-only input returned with whitespace", provider: "   ", want: "   "},
		{name: "lookalike id returned verbatim", provider: "ollama:", want: "ollama:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AIProviderDisplayName(tt.provider); got != tt.want {
				t.Fatalf("AIProviderDisplayName(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}
