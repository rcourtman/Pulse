package config

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

func TestEffectiveControlLevelForEntitlement(t *testing.T) {
	tests := []struct {
		name              string
		level             string
		autonomousAllowed bool
		want              string
	}{
		{
			name:              "autonomous allowed stays autonomous",
			level:             ControlLevelAutonomous,
			autonomousAllowed: true,
			want:              ControlLevelAutonomous,
		},
		{
			name:              "autonomous without entitlement becomes controlled",
			level:             ControlLevelAutonomous,
			autonomousAllowed: false,
			want:              ControlLevelControlled,
		},
		{
			name:              "controlled stays controlled without entitlement",
			level:             ControlLevelControlled,
			autonomousAllowed: false,
			want:              ControlLevelControlled,
		},
		{
			name:              "invalid stays fail closed",
			level:             "bad",
			autonomousAllowed: true,
			want:              ControlLevelReadOnly,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveControlLevelForEntitlement(tt.level, tt.autonomousAllowed); got != tt.want {
				t.Fatalf("EffectiveControlLevelForEntitlement(%q, %v) = %q, want %q", tt.level, tt.autonomousAllowed, got, tt.want)
			}
		})
	}
}

func TestAIConfigControlLevelsUseSharedAgentCapabilityVocabulary(t *testing.T) {
	if ControlLevelReadOnly != string(agentcapabilities.ControlLevelReadOnly) {
		t.Fatalf("ControlLevelReadOnly = %q, want shared %q", ControlLevelReadOnly, agentcapabilities.ControlLevelReadOnly)
	}
	if ControlLevelControlled != string(agentcapabilities.ControlLevelControlled) {
		t.Fatalf("ControlLevelControlled = %q, want shared %q", ControlLevelControlled, agentcapabilities.ControlLevelControlled)
	}
	if ControlLevelAutonomous != string(agentcapabilities.ControlLevelAutonomous) {
		t.Fatalf("ControlLevelAutonomous = %q, want shared %q", ControlLevelAutonomous, agentcapabilities.ControlLevelAutonomous)
	}
	if !(&AIConfig{ControlLevel: ControlLevelControlled}).IsControlEnabled() {
		t.Fatalf("controlled config should allow control tools through the shared predicate")
	}
	if (&AIConfig{ControlLevel: "bad"}).IsControlEnabled() {
		t.Fatalf("unknown config control level should fail closed through the shared predicate")
	}
}

func TestAIConfig_IsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		config   AIConfig
		expected bool
	}{
		{
			name:     "disabled config",
			config:   AIConfig{Enabled: false},
			expected: false,
		},
		{
			name: "enabled with anthropic key",
			config: AIConfig{
				Enabled:         true,
				AnthropicAPIKey: "sk-ant-123",
			},
			expected: true,
		},
		{
			name: "enabled with openai key",
			config: AIConfig{
				Enabled:      true,
				OpenAIAPIKey: "sk-openai-123",
			},
			expected: true,
		},
		{
			name: "enabled with openrouter key",
			config: AIConfig{
				Enabled:          true,
				OpenRouterAPIKey: "sk-or-123",
			},
			expected: true,
		},
		{
			name: "enabled with gemini key",
			config: AIConfig{
				Enabled:      true,
				GeminiAPIKey: "gemini-123",
			},
			expected: true,
		},
		{
			name: "enabled with ollama url",
			config: AIConfig{
				Enabled:       true,
				OllamaBaseURL: "http://localhost:11434",
			},
			expected: true,
		},
		{
			name: "enabled with legacy oauth only",
			config: AIConfig{
				Enabled:          true,
				AuthMethod:       AuthMethodOAuth,
				OAuthAccessToken: "oauth-token",
			},
			expected: false,
		},
		{
			name: "enabled but no credentials",
			config: AIConfig{
				Enabled: true,
			},
			expected: false,
		},
		{
			name: "enabled with deepseek key",
			config: AIConfig{
				Enabled:        true,
				DeepSeekAPIKey: "sk-ds-123",
			},
			expected: true,
		},
		{
			name: "enabled with zai key",
			config: AIConfig{
				Enabled:   true,
				ZaiAPIKey: "sk-zai-123",
			},
			expected: true,
		},
		{
			name: "enabled with groq key",
			config: AIConfig{
				Enabled:    true,
				GroqAPIKey: "gsk-123",
			},
			expected: true,
		},
		{
			name: "enabled with mistral key",
			config: AIConfig{
				Enabled:       true,
				MistralAPIKey: "mistral-123",
			},
			expected: true,
		},
		{
			name: "enabled with cerebras key",
			config: AIConfig{
				Enabled:        true,
				CerebrasAPIKey: "cerebras-123",
			},
			expected: true,
		},
		{
			name: "enabled with together key",
			config: AIConfig{
				Enabled:        true,
				TogetherAPIKey: "together-123",
			},
			expected: true,
		},
		{
			name: "enabled with fireworks key",
			config: AIConfig{
				Enabled:         true,
				FireworksAPIKey: "fireworks-123",
			},
			expected: true,
		},
		{
			name: "enabled with ollama but no URL configured",
			config: AIConfig{
				Enabled: true,
			},
			expected: false,
		},
		{
			name: "anthropic oauth needs token",
			config: AIConfig{
				Enabled:    true,
				AuthMethod: AuthMethodOAuth,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsConfigured()
			if result != tt.expected {
				t.Errorf("IsConfigured() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAIConfig_HasProvider(t *testing.T) {
	tests := []struct {
		name     string
		config   AIConfig
		provider string
		expected bool
	}{
		{
			name:     "no anthropic configured",
			config:   AIConfig{},
			provider: AIProviderAnthropic,
			expected: false,
		},
		{
			name:     "anthropic with api key",
			config:   AIConfig{AnthropicAPIKey: "key"},
			provider: AIProviderAnthropic,
			expected: true,
		},
		{
			name: "anthropic with legacy oauth only",
			config: AIConfig{
				AuthMethod:       AuthMethodOAuth,
				OAuthAccessToken: "token",
			},
			provider: AIProviderAnthropic,
			expected: false,
		},
		{
			name:     "openai configured",
			config:   AIConfig{OpenAIAPIKey: "key"},
			provider: AIProviderOpenAI,
			expected: true,
		},
		{
			name:     "openrouter configured",
			config:   AIConfig{OpenRouterAPIKey: "key"},
			provider: AIProviderOpenRouter,
			expected: true,
		},
		{
			name:     "deepseek configured",
			config:   AIConfig{DeepSeekAPIKey: "key"},
			provider: AIProviderDeepSeek,
			expected: true,
		},
		{
			name:     "gemini configured",
			config:   AIConfig{GeminiAPIKey: "key"},
			provider: AIProviderGemini,
			expected: true,
		},
		{
			name:     "zai configured",
			config:   AIConfig{ZaiAPIKey: "key"},
			provider: AIProviderZai,
			expected: true,
		},
		{
			name:     "groq configured",
			config:   AIConfig{GroqAPIKey: "key"},
			provider: AIProviderGroq,
			expected: true,
		},
		{
			name:     "mistral configured",
			config:   AIConfig{MistralAPIKey: "key"},
			provider: AIProviderMistral,
			expected: true,
		},
		{
			name:     "cerebras configured",
			config:   AIConfig{CerebrasAPIKey: "key"},
			provider: AIProviderCerebras,
			expected: true,
		},
		{
			name:     "together configured",
			config:   AIConfig{TogetherAPIKey: "key"},
			provider: AIProviderTogether,
			expected: true,
		},
		{
			name:     "fireworks configured",
			config:   AIConfig{FireworksAPIKey: "key"},
			provider: AIProviderFireworks,
			expected: true,
		},
		{
			name:     "ollama configured",
			config:   AIConfig{OllamaBaseURL: "http://localhost:11434"},
			provider: AIProviderOllama,
			expected: true,
		},
		{
			name:     "unknown provider",
			config:   AIConfig{},
			provider: "unknown",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.HasProvider(tt.provider)
			if result != tt.expected {
				t.Errorf("HasProvider(%q) = %v, want %v", tt.provider, result, tt.expected)
			}
		})
	}
}

func TestDefaultModelForProvider_UsesCanonicalProviderFallbacks(t *testing.T) {
	tests := []struct {
		provider string
		expected string
	}{
		{provider: AIProviderAnthropic, expected: "anthropic:claude-3-5-sonnet-latest"},
		{provider: AIProviderOpenAI, expected: "openai:gpt-4o"},
		{provider: AIProviderOpenRouter, expected: "openrouter:openai/gpt-4o-mini"},
		{provider: AIProviderDeepSeek, expected: "deepseek:deepseek-v4-flash"},
		{provider: AIProviderGemini, expected: "gemini:gemini-1.5-pro"},
		{provider: AIProviderZai, expected: "zai:glm-5.2"},
		{provider: AIProviderGroq, expected: "groq:llama-3.3-70b-versatile"},
		{provider: AIProviderMistral, expected: "mistral:mistral-large-latest"},
		{provider: AIProviderCerebras, expected: "cerebras:llama-4-scout-17b-16e-instruct"},
		{provider: AIProviderTogether, expected: "together:meta-llama/Llama-3.3-70B-Instruct-Turbo"},
		{provider: AIProviderFireworks, expected: "fireworks:accounts/fireworks/models/llama-v3p1-70b-instruct"},
		{provider: AIProviderOllama, expected: "ollama:llama3.2"},
		{provider: AIProviderQuickstart, expected: ""},
		{provider: "unknown", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			if got := DefaultModelForProvider(tt.provider); got != tt.expected {
				t.Fatalf("DefaultModelForProvider(%q) = %q, want %q", tt.provider, got, tt.expected)
			}
		})
	}
}

func TestAIProviderDefinitions_CanonicalDirectProviderRegistry(t *testing.T) {
	defs := AIConfigurableProviderDefinitions()
	gotIDs := make([]string, 0, len(defs))
	for _, def := range defs {
		gotIDs = append(gotIDs, def.ID)
	}
	wantIDs := []string{
		AIProviderAnthropic,
		AIProviderOpenAI,
		AIProviderOpenRouter,
		AIProviderDeepSeek,
		AIProviderGemini,
		AIProviderZai,
		AIProviderGroq,
		AIProviderMistral,
		AIProviderCerebras,
		AIProviderTogether,
		AIProviderFireworks,
		AIProviderOllama,
	}
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("AIConfigurableProviderDefinitions returned %d providers %v, want %d %v", len(gotIDs), gotIDs, len(wantIDs), wantIDs)
	}
	for i, want := range wantIDs {
		if gotIDs[i] != want {
			t.Fatalf("provider order mismatch at %d: got %q in %v, want %q in %v", i, gotIDs[i], gotIDs, want, wantIDs)
		}
	}

	for _, provider := range []string{
		AIProviderOpenRouter,
		AIProviderDeepSeek,
		AIProviderZai,
		AIProviderGroq,
		AIProviderMistral,
		AIProviderCerebras,
		AIProviderTogether,
		AIProviderFireworks,
	} {
		def, ok := LookupAIProviderDefinition(provider)
		if !ok {
			t.Fatalf("missing provider definition for %q", provider)
		}
		if def.Protocol != AIProviderProtocolOpenAICompatible {
			t.Fatalf("%s protocol = %q, want %q", provider, def.Protocol, AIProviderProtocolOpenAICompatible)
		}
		if def.DefaultBaseURL == "" {
			t.Fatalf("%s must declare a default base URL", provider)
		}
		if def.APIKeyField == "" || def.ConfiguredField == "" || def.ClearKeyField == "" {
			t.Fatalf("%s must declare API settings fields: %#v", provider, def)
		}
	}

	if _, ok := LookupAIProviderDefinition(AIProviderQuickstart); !ok {
		t.Fatalf("retired quickstart marker should remain known for migration cleanup")
	}
	for _, def := range AIConfigurableProviderDefinitions() {
		if def.ID == AIProviderQuickstart {
			t.Fatalf("retired quickstart marker must not be user configurable")
		}
	}
}

func TestDeepSeekModelCatalogHelpers(t *testing.T) {
	if got := DeepSeekV4ModelIDs(); len(got) != 2 || got[0] != DeepSeekModelV4Flash || got[1] != DeepSeekModelV4Pro {
		t.Fatalf("DeepSeekV4ModelIDs() = %#v", got)
	}
	if got := DeepSeekLegacyAliasModelIDs(); len(got) != 2 || got[0] != DeepSeekModelLegacyChat || got[1] != DeepSeekModelLegacyReasoner {
		t.Fatalf("DeepSeekLegacyAliasModelIDs() = %#v", got)
	}
	if !IsDeepSeekV4Model(" DeepSeek-V4-Flash ") {
		t.Fatal("expected DeepSeek V4 Flash helper to normalize case and whitespace")
	}
	if IsDeepSeekV4Model(DeepSeekModelLegacyChat) {
		t.Fatal("legacy alias must not be classified as a current V4 model")
	}
	if !IsDeepSeekLegacyAliasModel(" DEEPSEEK-REASONER ") {
		t.Fatal("expected DeepSeek legacy alias helper to normalize case and whitespace")
	}
	if IsDeepSeekLegacyAliasModel(DeepSeekModelV4Pro) {
		t.Fatal("current V4 model must not be classified as a legacy alias")
	}
}

func TestAIConfig_GetConfiguredProviders(t *testing.T) {
	tests := []struct {
		name   string
		config AIConfig
		count  int
	}{
		{
			name:   "no providers",
			config: AIConfig{},
			count:  0,
		},
		{
			name: "one provider",
			config: AIConfig{
				AnthropicAPIKey: "key",
			},
			count: 1,
		},
		{
			name: "multiple providers",
			config: AIConfig{
				AnthropicAPIKey: "key1",
				OpenAIAPIKey:    "key2",
				GeminiAPIKey:    "key3",
			},
			count: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providers := tt.config.GetConfiguredProviders()
			if len(providers) != tt.count {
				t.Errorf("GetConfiguredProviders() returned %d providers, want %d", len(providers), tt.count)
			}
		})
	}
}

func TestDefaultModelForProvider_RetiredQuickstart(t *testing.T) {
	if DefaultAIModelQuickstart != "pulse-hosted" {
		t.Fatalf("DefaultAIModelQuickstart = %q, want pulse-hosted", DefaultAIModelQuickstart)
	}
	got := DefaultModelForProvider(AIProviderQuickstart)
	if got != "" {
		t.Fatalf("DefaultModelForProvider(%q) = %q, want empty for retired quickstart", AIProviderQuickstart, got)
	}
}

func TestAIConfig_GetAPIKeyForProvider(t *testing.T) {
	config := AIConfig{
		AnthropicAPIKey:  "anthropic-key",
		OpenAIAPIKey:     "openai-key",
		OpenRouterAPIKey: "openrouter-key",
		DeepSeekAPIKey:   "deepseek-key",
		GeminiAPIKey:     "gemini-key",
		ZaiAPIKey:        "zai-key",
		GroqAPIKey:       "groq-key",
		MistralAPIKey:    "mistral-key",
		CerebrasAPIKey:   "cerebras-key",
		TogetherAPIKey:   "together-key",
		FireworksAPIKey:  "fireworks-key",
	}

	tests := []struct {
		provider string
		expected string
	}{
		{AIProviderAnthropic, "anthropic-key"},
		{AIProviderOpenAI, "openai-key"},
		{AIProviderOpenRouter, "openrouter-key"},
		{AIProviderDeepSeek, "deepseek-key"},
		{AIProviderGemini, "gemini-key"},
		{AIProviderZai, "zai-key"},
		{AIProviderGroq, "groq-key"},
		{AIProviderMistral, "mistral-key"},
		{AIProviderCerebras, "cerebras-key"},
		{AIProviderTogether, "together-key"},
		{AIProviderFireworks, "fireworks-key"},
		{AIProviderOllama, ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			result := config.GetAPIKeyForProvider(tt.provider)
			if result != tt.expected {
				t.Errorf("GetAPIKeyForProvider(%q) = %q, want %q", tt.provider, result, tt.expected)
			}
		})
	}
}

func TestAIConfig_GetBaseURLForProvider(t *testing.T) {
	config := AIConfig{
		OllamaBaseURL: "http://custom:11434",
		OpenAIBaseURL: "https://custom-openai.com",
	}

	tests := []struct {
		provider string
		expected string
	}{
		{AIProviderOllama, "http://custom:11434"},
		{AIProviderOpenAI, "https://custom-openai.com"},
		{AIProviderOpenRouter, DefaultOpenRouterBaseURL},
		{AIProviderDeepSeek, DefaultDeepSeekBaseURL},
		{AIProviderGemini, DefaultGeminiBaseURL},
		{AIProviderZai, DefaultZaiBaseURL},
		{AIProviderGroq, DefaultGroqBaseURL},
		{AIProviderMistral, DefaultMistralBaseURL},
		{AIProviderCerebras, DefaultCerebrasBaseURL},
		{AIProviderTogether, DefaultTogetherBaseURL},
		{AIProviderFireworks, DefaultFireworksBaseURL},
		{AIProviderAnthropic, ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			result := config.GetBaseURLForProvider(tt.provider)
			if result != tt.expected {
				t.Errorf("GetBaseURLForProvider(%q) = %q, want %q", tt.provider, result, tt.expected)
			}
		})
	}

	t.Run("default urls", func(t *testing.T) {
		cfg := AIConfig{}
		if url := cfg.GetBaseURLForProvider(AIProviderOllama); url != DefaultOllamaBaseURL {
			t.Errorf("ollama default = %q, want %q", url, DefaultOllamaBaseURL)
		}
		if url := cfg.GetBaseURLForProvider(AIProviderOpenAI); url != "" {
			t.Errorf("openai default = %q, want empty", url)
		}
	})

}

func TestAIConfig_OllamaKeepAliveDefaultsAndValidation(t *testing.T) {
	t.Run("default config keeps pulse keep alive", func(t *testing.T) {
		cfg := NewDefaultAIConfig()
		if got := cfg.GetOllamaKeepAlive(); got != DefaultOllamaKeepAlive {
			t.Fatalf("GetOllamaKeepAlive() = %q, want %q", got, DefaultOllamaKeepAlive)
		}
	})

	t.Run("empty preserves server default intent", func(t *testing.T) {
		cfg := &AIConfig{OllamaKeepAlive: ""}
		if got := cfg.GetOllamaKeepAlive(); got != "" {
			t.Fatalf("GetOllamaKeepAlive() = %q, want empty", got)
		}
	})

	valid := []string{"30s", "5m", "24h", "0", "-1", "3600"}
	for _, value := range valid {
		t.Run("valid "+value, func(t *testing.T) {
			got, err := NormalizeOllamaKeepAlive(" " + value + " ")
			if err != nil {
				t.Fatalf("NormalizeOllamaKeepAlive(%q) returned error: %v", value, err)
			}
			if got != value {
				t.Fatalf("NormalizeOllamaKeepAlive(%q) = %q", value, got)
			}
		})
	}

	invalid := []string{"later", "forever", "NaN", "+Inf"}
	for _, value := range invalid {
		t.Run("invalid "+value, func(t *testing.T) {
			if _, err := NormalizeOllamaKeepAlive(value); err == nil {
				t.Fatalf("NormalizeOllamaKeepAlive(%q) returned nil error", value)
			}
		})
	}
}

func TestAIConfig_IsUsingOAuthUnsupported(t *testing.T) {
	tests := []struct {
		name     string
		config   AIConfig
		expected bool
	}{
		{
			name:     "not using oauth",
			config:   AIConfig{AuthMethod: AuthMethodAPIKey},
			expected: false,
		},
		{
			name: "oauth method but no token",
			config: AIConfig{
				AuthMethod: AuthMethodOAuth,
			},
			expected: false,
		},
		{
			name: "legacy oauth with token",
			config: AIConfig{
				AuthMethod:       AuthMethodOAuth,
				OAuthAccessToken: "token",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsUsingOAuth()
			if result != tt.expected {
				t.Errorf("IsUsingOAuth() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseModelString(t *testing.T) {
	tests := []struct {
		model        string
		wantProvider string
		wantModel    string
	}{
		// Explicit prefixes
		{"anthropic:claude-3-opus", AIProviderAnthropic, "claude-3-opus"},
		{"openai:gpt-4o", AIProviderOpenAI, "gpt-4o"},
		{"openrouter:openai/gpt-4o-mini", AIProviderOpenRouter, "openai/gpt-4o-mini"},
		{"ollama:llama3", AIProviderOllama, "llama3"},
		{"deepseek:deepseek-chat", AIProviderDeepSeek, "deepseek-chat"},
		{"gemini:gemini-1.5-pro", AIProviderGemini, "gemini-1.5-pro"},
		{"zai:glm-5.2", AIProviderZai, "glm-5.2"},
		{"groq:llama-3.3-70b-versatile", AIProviderGroq, "llama-3.3-70b-versatile"},
		{"mistral:mistral-large-latest", AIProviderMistral, "mistral-large-latest"},
		{"cerebras:llama-4-scout-17b-16e-instruct", AIProviderCerebras, "llama-4-scout-17b-16e-instruct"},
		{"together:meta-llama/Llama-3.3-70B-Instruct-Turbo", AIProviderTogether, "meta-llama/Llama-3.3-70B-Instruct-Turbo"},
		{"fireworks:accounts/fireworks/models/llama-v3p1-70b-instruct", AIProviderFireworks, "accounts/fireworks/models/llama-v3p1-70b-instruct"},
		// Detection by name
		{"claude-3-opus", AIProviderAnthropic, "claude-3-opus"},
		{"gpt-4o", AIProviderOpenAI, "gpt-4o"},
		{"o1-preview", AIProviderOpenAI, "o1-preview"},
		{"deepseek-chat", AIProviderDeepSeek, "deepseek-chat"},
		{"gemini-1.5-pro", AIProviderGemini, "gemini-1.5-pro"},
		{"anthropic/claude-sonnet-4.5", AIProviderOpenRouter, "anthropic/claude-sonnet-4.5"},
		// Unknown models default to Ollama
		{"llama3", AIProviderOllama, "llama3"},
		{"mistral", AIProviderOllama, "mistral"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			provider, model := ParseModelString(tt.model)
			if provider != tt.wantProvider {
				t.Errorf("ParseModelString(%q) provider = %q, want %q", tt.model, provider, tt.wantProvider)
			}
			if model != tt.wantModel {
				t.Errorf("ParseModelString(%q) model = %q, want %q", tt.model, model, tt.wantModel)
			}
		})
	}
}

func TestFormatModelString(t *testing.T) {
	result := FormatModelString(AIProviderAnthropic, "claude-3-opus")
	if result != "anthropic:claude-3-opus" {
		t.Errorf("FormatModelString() = %q, want %q", result, "anthropic:claude-3-opus")
	}
}

func TestAIConfig_GetModel(t *testing.T) {
	tests := []struct {
		name     string
		config   AIConfig
		expected string
	}{
		{
			name:     "explicit model set",
			config:   AIConfig{Model: "custom-model"},
			expected: "custom-model",
		},
		{
			name: "single provider configured does not invent anthropic model",
			config: AIConfig{
				AnthropicAPIKey: "key",
			},
			expected: "",
		},
		{
			name: "single provider configured does not invent openai model",
			config: AIConfig{
				OpenAIAPIKey: "key",
			},
			expected: "",
		},
		{
			name: "single provider configured does not invent openrouter model",
			config: AIConfig{
				OpenRouterAPIKey: "key",
			},
			expected: "",
		},
		{
			name: "single provider configured does not invent deepseek model",
			config: AIConfig{
				DeepSeekAPIKey: "key",
			},
			expected: "",
		},
		{
			name: "single provider configured does not invent gemini model",
			config: AIConfig{
				GeminiAPIKey: "key",
			},
			expected: "",
		},
		{
			name: "single provider configured does not invent ollama model",
			config: AIConfig{
				OllamaBaseURL: "http://localhost:11434",
			},
			expected: "",
		},
		{
			name: "multiple providers configured (no default)",
			config: AIConfig{
				AnthropicAPIKey: "key",
				OpenAIAPIKey:    "key",
			},
			expected: "",
		},
		{
			name:     "no model/provider",
			config:   AIConfig{},
			expected: "",
		},
		{
			name:     "legacy quickstart model is retired",
			config:   AIConfig{Model: "quickstart:minimax-2.5m"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetModel()
			if result != tt.expected {
				t.Errorf("GetModel() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestAIConfig_GetChatModel(t *testing.T) {
	t.Run("explicit chat model", func(t *testing.T) {
		config := AIConfig{
			Model:     "default-model",
			ChatModel: "chat-model",
		}
		if result := config.GetChatModel(); result != "chat-model" {
			t.Errorf("GetChatModel() = %q, want 'chat-model'", result)
		}
	})

	t.Run("fallback to main model", func(t *testing.T) {
		config := AIConfig{
			Model: "main-model",
		}
		if result := config.GetChatModel(); result != "main-model" {
			t.Errorf("GetChatModel() = %q, want 'main-model'", result)
		}
	})

	t.Run("retires legacy quickstart chat model", func(t *testing.T) {
		config := AIConfig{
			Model:     "openai:gpt-4o-mini",
			ChatModel: "quickstart:minimax-2.5m",
		}
		if result := config.GetChatModel(); result != "" {
			t.Errorf("GetChatModel() = %q, want empty for retired quickstart", result)
		}
	})
}

func TestAIConfig_NormalizeQuickstartModelAliases(t *testing.T) {
	config := AIConfig{
		Model:          "quickstart:minimax-2.5m",
		ChatModel:      "pulse-hosted",
		PatrolModel:    "quickstart:anything",
		DiscoveryModel: "",
		AutoFixModel:   "quickstart:legacy-provider-model",
	}

	changed := config.NormalizeQuickstartModelAliases()
	if !changed {
		t.Fatal("expected quickstart alias normalization to report a change")
	}

	if config.Model != "" || config.ChatModel != "" || config.PatrolModel != "" || config.AutoFixModel != "" {
		t.Fatalf("NormalizeQuickstartModelAliases() = %#v, want all quickstart fields cleared", config)
	}
	if config.DiscoveryModel != "" {
		t.Fatalf("expected empty discovery model to remain empty, got %q", config.DiscoveryModel)
	}
}

func TestAIConfig_GetPreferredModelForProvider(t *testing.T) {
	tests := []struct {
		name     string
		config   AIConfig
		provider string
		expected string
	}{
		{
			name: "uses main model when provider matches",
			config: AIConfig{
				Model: "ollama:llama3.2",
			},
			provider: AIProviderOllama,
			expected: "ollama:llama3.2",
		},
		{
			name: "falls back to patrol override for provider",
			config: AIConfig{
				Model:       "openai:gpt-4o",
				PatrolModel: "ollama:qwen2.5",
			},
			provider: AIProviderOllama,
			expected: "ollama:qwen2.5",
		},
		{
			name: "detects unprefixed ollama model",
			config: AIConfig{
				PatrolModel: "llama3.1",
			},
			provider: AIProviderOllama,
			expected: "llama3.1",
		},
		{
			name: "returns empty when no model matches",
			config: AIConfig{
				Model: "openai:gpt-4o",
			},
			provider: AIProviderGemini,
			expected: "",
		},
		{
			name:     "unknown provider returns empty",
			config:   AIConfig{Model: "openai:gpt-4o"},
			provider: "unknown",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.GetPreferredModelForProvider(tt.provider); got != tt.expected {
				t.Fatalf("GetPreferredModelForProvider(%q) = %q, want %q", tt.provider, got, tt.expected)
			}
		})
	}
}

func TestAIConfig_GetPatrolModel(t *testing.T) {
	t.Run("explicit patrol model", func(t *testing.T) {
		config := AIConfig{
			Model:       "default-model",
			PatrolModel: "patrol-model",
		}
		if result := config.GetPatrolModel(); result != "patrol-model" {
			t.Errorf("GetPatrolModel() = %q, want 'patrol-model'", result)
		}
	})

	t.Run("fallback to main model", func(t *testing.T) {
		config := AIConfig{
			Model: "main-model",
		}
		if result := config.GetPatrolModel(); result != "main-model" {
			t.Errorf("GetPatrolModel() = %q, want 'main-model'", result)
		}
	})
}

func TestAIConfig_GetAutoFixModel(t *testing.T) {
	t.Run("explicit autofix model", func(t *testing.T) {
		config := AIConfig{
			AutoFixModel: "autofix-model",
		}
		if result := config.GetAutoFixModel(); result != "autofix-model" {
			t.Errorf("GetAutoFixModel() = %q, want 'autofix-model'", result)
		}
	})

	t.Run("fallback to patrol model", func(t *testing.T) {
		config := AIConfig{
			PatrolModel: "patrol-model",
		}
		if result := config.GetAutoFixModel(); result != "patrol-model" {
			t.Errorf("GetAutoFixModel() = %q, want 'patrol-model'", result)
		}
	})
}

func TestAIConfig_ClearOAuthTokens(t *testing.T) {
	config := AIConfig{
		OAuthAccessToken:  "access",
		OAuthRefreshToken: "refresh",
		OAuthExpiresAt:    time.Now(),
	}

	config.ClearOAuthTokens()

	if config.OAuthAccessToken != "" {
		t.Error("OAuthAccessToken should be cleared")
	}
	if config.OAuthRefreshToken != "" {
		t.Error("OAuthRefreshToken should be cleared")
	}
	if !config.OAuthExpiresAt.IsZero() {
		t.Error("OAuthExpiresAt should be zero")
	}
}

func TestAIConfig_ClearAPIKey(t *testing.T) {
	config := AIConfig{AnthropicAPIKey: "key"}
	config.ClearAPIKey()
	if config.AnthropicAPIKey != "" {
		t.Error("AnthropicAPIKey should be cleared")
	}
}

func TestAIConfig_GetPatrolInterval(t *testing.T) {
	tests := []struct {
		name     string
		config   AIConfig
		expected time.Duration
	}{
		{
			name:     "custom minutes",
			config:   AIConfig{PatrolIntervalMinutes: 30},
			expected: 30 * time.Minute,
		},
		{
			name:     "explicit 15min should stay 15min",
			config:   AIConfig{PatrolIntervalMinutes: 15},
			expected: 15 * time.Minute,
		},
		{
			name:     "default 6hr",
			config:   AIConfig{},
			expected: 6 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetPatrolInterval()
			if result != tt.expected {
				t.Errorf("GetPatrolInterval() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAIConfig_IntervalSurvivesRoundTrip(t *testing.T) {
	// Custom patrol intervals must survive JSON round-trips.
	cfg := NewDefaultAIConfig()
	cfg.PatrolIntervalMinutes = 15

	// Simulate save → load round-trip via JSON
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	loaded := NewDefaultAIConfig()
	if err := json.Unmarshal(data, loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The interval must be the user's 15 minutes, not the default 6 hours
	interval := loaded.GetPatrolInterval()
	if interval != 15*time.Minute {
		t.Errorf("GetPatrolInterval() = %v after round-trip, want 15m", interval)
	}
}

func TestAIConfig_IsPatrolEnabled(t *testing.T) {
	tests := []struct {
		name     string
		config   AIConfig
		expected bool
	}{
		{
			name:     "patrol disabled when AI disabled",
			config:   AIConfig{Enabled: false, PatrolEnabled: true},
			expected: false,
		},
		{
			name:     "patrol enabled when AI enabled",
			config:   AIConfig{Enabled: true, PatrolEnabled: true},
			expected: true,
		},
		{
			name:     "patrol disabled by flag",
			config:   AIConfig{Enabled: true, PatrolEnabled: false},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsPatrolEnabled()
			if result != tt.expected {
				t.Errorf("IsPatrolEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAIConfig_IsAlertTriggeredAnalysisEnabled(t *testing.T) {
	t.Run("enabled when AI enabled", func(t *testing.T) {
		config := AIConfig{Enabled: true, AlertTriggeredAnalysis: true}
		if !config.IsAlertTriggeredAnalysisEnabled() {
			t.Error("expected true")
		}
	})

	t.Run("disabled when AI disabled", func(t *testing.T) {
		config := AIConfig{Enabled: false, AlertTriggeredAnalysis: true}
		if config.IsAlertTriggeredAnalysisEnabled() {
			t.Error("expected false when AI is disabled")
		}
	})

	t.Run("disabled by flag", func(t *testing.T) {
		config := AIConfig{Enabled: true, AlertTriggeredAnalysis: false}
		if config.IsAlertTriggeredAnalysisEnabled() {
			t.Error("expected false")
		}
	})
}

func TestNewDefaultAIConfig(t *testing.T) {
	config := NewDefaultAIConfig()

	if config.Enabled {
		t.Error("Default should not be enabled")
	}
	if config.PatrolIntervalMinutes != 360 {
		t.Errorf("Default patrol interval should be 360, got %d", config.PatrolIntervalMinutes)
	}
	if config.Model != "" {
		t.Errorf("Default model should be empty until a provider model is resolved, got %q", config.Model)
	}
	if !config.PatrolEnabled {
		t.Error("Default patrol should be enabled")
	}
	if !config.AlertTriggeredAnalysis {
		t.Error("Default alert triggered analysis should be enabled")
	}
	if !config.PatrolAlertTriggersEnabled {
		t.Error("Default alert-triggered patrols should be enabled")
	}
	if !config.PatrolAnomalyTriggersEnabled {
		t.Error("Default anomaly-triggered patrols should be enabled")
	}
}

func TestAIConfig_PatrolEventTriggerSettings(t *testing.T) {
	t.Run("legacy aggregate enables both scoped trigger sources", func(t *testing.T) {
		cfg := &AIConfig{
			Enabled:                    true,
			PatrolEventTriggersEnabled: true,
		}

		settings := cfg.GetPatrolEventTriggerSettings()
		if !settings.AlertTriggersEnabled || !settings.AnomalyTriggersEnabled {
			t.Fatalf("expected both scoped trigger sources to inherit from legacy aggregate, got %+v", settings)
		}
		if !cfg.IsPatrolAlertTriggersEnabled() || !cfg.IsPatrolAnomalyTriggersEnabled() {
			t.Fatal("expected runtime helpers to treat both scoped trigger sources as enabled")
		}
	})

	t.Run("normalize syncs legacy aggregate with split trigger settings", func(t *testing.T) {
		cfg := &AIConfig{
			Enabled:                      true,
			PatrolEventTriggersEnabled:   false,
			PatrolAlertTriggersEnabled:   true,
			PatrolAnomalyTriggersEnabled: false,
		}

		if !cfg.NormalizePatrolEventTriggerSettings() {
			t.Fatal("expected normalization to rewrite the legacy aggregate field")
		}
		if !cfg.PatrolEventTriggersEnabled {
			t.Fatal("expected legacy aggregate field to reflect enabled split settings")
		}
		if !cfg.IsPatrolEventTriggersEnabled() {
			t.Fatal("expected aggregate runtime helper to stay enabled when one source remains enabled")
		}
	})
}

func TestAIConfig_GetPatrolAlertTriggerMinSeverity(t *testing.T) {
	tests := []struct {
		name string
		cfg  *AIConfig
		want string
	}{
		{name: "nil receiver defaults to critical", cfg: nil, want: AlertTriggerSeverityCritical},
		{name: "empty defaults to critical", cfg: &AIConfig{}, want: AlertTriggerSeverityCritical},
		{name: "warning preserved", cfg: &AIConfig{PatrolAlertTriggerMinSeverity: AlertTriggerSeverityWarning}, want: AlertTriggerSeverityWarning},
		{name: "critical preserved", cfg: &AIConfig{PatrolAlertTriggerMinSeverity: AlertTriggerSeverityCritical}, want: AlertTriggerSeverityCritical},
		{name: "mixed case normalized", cfg: &AIConfig{PatrolAlertTriggerMinSeverity: " Warning "}, want: AlertTriggerSeverityWarning},
		{name: "unknown defaults to critical", cfg: &AIConfig{PatrolAlertTriggerMinSeverity: "bogus"}, want: AlertTriggerSeverityCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetPatrolAlertTriggerMinSeverity(); got != tt.want {
				t.Fatalf("GetPatrolAlertTriggerMinSeverity() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAIConfig_AlertTriggersInvestigation(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *AIConfig
		alertType   string
		level       string
		wantTrigger bool
	}{
		{
			name:        "nil receiver never triggers",
			cfg:         nil,
			level:       AlertTriggerSeverityCritical,
			wantTrigger: false,
		},
		{
			name:        "master toggle off never triggers",
			cfg:         &AIConfig{PatrolAlertTriggersEnabled: false, PatrolAlertTriggerMinSeverity: AlertTriggerSeverityWarning},
			level:       AlertTriggerSeverityCritical,
			wantTrigger: false,
		},
		{
			name:        "critical floor rejects warning",
			cfg:         &AIConfig{PatrolAlertTriggersEnabled: true, PatrolAlertTriggerMinSeverity: AlertTriggerSeverityCritical},
			level:       AlertTriggerSeverityWarning,
			wantTrigger: false,
		},
		{
			name:        "critical floor accepts critical",
			cfg:         &AIConfig{PatrolAlertTriggersEnabled: true, PatrolAlertTriggerMinSeverity: AlertTriggerSeverityCritical},
			level:       AlertTriggerSeverityCritical,
			wantTrigger: true,
		},
		{
			name:        "empty floor defaults to critical-only",
			cfg:         &AIConfig{PatrolAlertTriggersEnabled: true},
			level:       AlertTriggerSeverityWarning,
			wantTrigger: false,
		},
		{
			name:        "warning floor accepts warning",
			cfg:         &AIConfig{PatrolAlertTriggersEnabled: true, PatrolAlertTriggerMinSeverity: AlertTriggerSeverityWarning},
			level:       AlertTriggerSeverityWarning,
			wantTrigger: true,
		},
		{
			name:        "warning floor accepts critical",
			cfg:         &AIConfig{PatrolAlertTriggersEnabled: true, PatrolAlertTriggerMinSeverity: AlertTriggerSeverityWarning},
			level:       AlertTriggerSeverityCritical,
			wantTrigger: true,
		},
		{
			name:        "unknown level treated as critical",
			cfg:         &AIConfig{PatrolAlertTriggersEnabled: true, PatrolAlertTriggerMinSeverity: AlertTriggerSeverityCritical},
			level:       "",
			wantTrigger: true,
		},
		{
			name:        "allowlist admits matching type",
			cfg:         &AIConfig{PatrolAlertTriggersEnabled: true, PatrolAlertTriggerMinSeverity: AlertTriggerSeverityCritical, PatrolAlertTriggerTypes: []string{"cpu"}},
			alertType:   "CPU",
			level:       AlertTriggerSeverityCritical,
			wantTrigger: true,
		},
		{
			name:        "allowlist rejects unlisted type",
			cfg:         &AIConfig{PatrolAlertTriggersEnabled: true, PatrolAlertTriggerMinSeverity: AlertTriggerSeverityCritical, PatrolAlertTriggerTypes: []string{"cpu"}},
			alertType:   "memory",
			level:       AlertTriggerSeverityCritical,
			wantTrigger: false,
		},
		{
			name:        "empty allowlist admits any type",
			cfg:         &AIConfig{PatrolAlertTriggersEnabled: true, PatrolAlertTriggerMinSeverity: AlertTriggerSeverityCritical, PatrolAlertTriggerTypes: []string{}},
			alertType:   "disk",
			level:       AlertTriggerSeverityCritical,
			wantTrigger: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.AlertTriggersInvestigation(tt.alertType, tt.level); got != tt.wantTrigger {
				t.Fatalf("AlertTriggersInvestigation(%q, %q) = %v, want %v", tt.alertType, tt.level, got, tt.wantTrigger)
			}
		})
	}
}

func TestAIConfig_GetRequestTimeout(t *testing.T) {
	tests := []struct {
		name     string
		config   *AIConfig
		expected time.Duration
	}{
		{
			name:     "Default",
			config:   &AIConfig{},
			expected: 300 * time.Second,
		},
		{
			name: "Custom",
			config: &AIConfig{
				RequestTimeoutSeconds: 60,
			},
			expected: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if duration := tt.config.GetRequestTimeout(); duration != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, duration)
			}
		})
	}
}
