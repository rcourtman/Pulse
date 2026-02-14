package config

import (
	"encoding/json"
	"testing"
	"time"
)

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
			name: "enabled with oauth",
			config: AIConfig{
				Enabled:          true,
				AuthMethod:       AuthMethodOAuth,
				OAuthAccessToken: "oauth-token",
			},
			expected: true,
		},
		{
			name: "enabled but no credentials",
			config: AIConfig{
				Enabled: true,
			},
			expected: false,
		},
		{
			name: "legacy provider with api key",
			config: AIConfig{
				Enabled:  true,
				Provider: AIProviderAnthropic,
				APIKey:   "legacy-key",
			},
			expected: true,
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
			name: "enabled with ollama (always configured if enabled)",
			config: AIConfig{
				Enabled:  true,
				Provider: AIProviderOllama,
			},
			expected: true,
		},
		{
			name: "enabled with unknown provider",
			config: AIConfig{
				Enabled:  true,
				Provider: "unknown",
			},
			expected: false,
		},
		{
			name: "anthropic legacy needs key",
			config: AIConfig{
				Enabled:  true,
				Provider: AIProviderAnthropic,
			},
			expected: false,
		},
		{
			name: "openai legacy needs key",
			config: AIConfig{
				Enabled:  true,
				Provider: AIProviderOpenAI,
			},
			expected: false,
		},
		{
			name: "anthropic oauth needs token",
			config: AIConfig{
				Enabled:    true,
				Provider:   AIProviderAnthropic,
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
			name: "anthropic with oauth",
			config: AIConfig{
				AuthMethod:       AuthMethodOAuth,
				OAuthAccessToken: "token",
			},
			provider: AIProviderAnthropic,
			expected: true,
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

func TestAIConfig_GetAPIKeyForProvider(t *testing.T) {
	config := AIConfig{
		AnthropicAPIKey:  "anthropic-key",
		OpenAIAPIKey:     "openai-key",
		OpenRouterAPIKey: "openrouter-key",
		DeepSeekAPIKey:   "deepseek-key",
		GeminiAPIKey:     "gemini-key",
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

	t.Run("legacy fallback anthropic", func(t *testing.T) {
		cfg := AIConfig{APIKey: "legacy", Provider: AIProviderAnthropic}
		if key := cfg.GetAPIKeyForProvider(AIProviderAnthropic); key != "legacy" {
			t.Errorf("want legacy, got %q", key)
		}
	})

	t.Run("legacy fallback openai", func(t *testing.T) {
		cfg := AIConfig{APIKey: "legacy", Provider: AIProviderOpenAI}
		if key := cfg.GetAPIKeyForProvider(AIProviderOpenAI); key != "legacy" {
			t.Errorf("want legacy, got %q", key)
		}
	})

	t.Run("legacy fallback deepseek", func(t *testing.T) {
		cfg := AIConfig{APIKey: "legacy", Provider: AIProviderDeepSeek}
		if key := cfg.GetAPIKeyForProvider(AIProviderDeepSeek); key != "legacy" {
			t.Errorf("want legacy, got %q", key)
		}
	})

	t.Run("legacy fallback openrouter", func(t *testing.T) {
		cfg := AIConfig{APIKey: "legacy", Provider: AIProviderOpenRouter}
		if key := cfg.GetAPIKeyForProvider(AIProviderOpenRouter); key != "legacy" {
			t.Errorf("want legacy, got %q", key)
		}
	})

	t.Run("legacy fallback gemini", func(t *testing.T) {
		cfg := AIConfig{APIKey: "legacy", Provider: AIProviderGemini}
		if key := cfg.GetAPIKeyForProvider(AIProviderGemini); key != "legacy" {
			t.Errorf("want legacy, got %q", key)
		}
	})
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

	t.Run("legacy base url fallback", func(t *testing.T) {
		cfg := AIConfig{
			Provider: AIProviderOllama,
			BaseURL:  "http://legacy:11434",
		}
		if url := cfg.GetBaseURLForProvider(AIProviderOllama); url != "http://legacy:11434" {
			t.Errorf("got %q, want legacy url", url)
		}
	})
}

func TestAIConfig_IsUsingOAuth(t *testing.T) {
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
			name: "using oauth with token",
			config: AIConfig{
				AuthMethod:       AuthMethodOAuth,
				OAuthAccessToken: "token",
			},
			expected: true,
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

func TestAIConfig_GetBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		config   AIConfig
		expected string
	}{
		{
			name: "custom base url",
			config: AIConfig{
				BaseURL: "https://custom.url",
			},
			expected: "https://custom.url",
		},
		{
			name: "ollama default",
			config: AIConfig{
				Provider: AIProviderOllama,
			},
			expected: DefaultOllamaBaseURL,
		},
		{
			name: "openrouter default",
			config: AIConfig{
				Provider: AIProviderOpenRouter,
			},
			expected: DefaultOpenRouterBaseURL,
		},
		{
			name: "deepseek default",
			config: AIConfig{
				Provider: AIProviderDeepSeek,
			},
			expected: DefaultDeepSeekBaseURL,
		},
		{
			name: "gemini default",
			config: AIConfig{
				Provider: AIProviderGemini,
			},
			expected: DefaultGeminiBaseURL,
		},
		{
			name: "anthropic no URL",
			config: AIConfig{
				Provider: AIProviderAnthropic,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetBaseURL()
			if result != tt.expected {
				t.Errorf("GetBaseURL() = %q, want %q", result, tt.expected)
			}
		})
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
			name: "single provider configured - anthropic",
			config: AIConfig{
				AnthropicAPIKey: "key",
			},
			expected: DefaultAIModelAnthropic,
		},
		{
			name: "single provider configured - openai",
			config: AIConfig{
				OpenAIAPIKey: "key",
			},
			expected: DefaultAIModelOpenAI,
		},
		{
			name: "single provider configured - openrouter",
			config: AIConfig{
				OpenRouterAPIKey: "key",
			},
			expected: DefaultAIModelOpenRouter,
		},
		{
			name: "single provider configured - deepseek",
			config: AIConfig{
				DeepSeekAPIKey: "key",
			},
			expected: DefaultAIModelDeepSeek,
		},
		{
			name: "single provider configured - gemini",
			config: AIConfig{
				GeminiAPIKey: "key",
			},
			expected: DefaultAIModelGemini,
		},
		{
			name: "single provider configured - ollama",
			config: AIConfig{
				OllamaBaseURL: "http://localhost:11434",
			},
			expected: DefaultAIModelOllama,
		},
		{
			name: "multiple providers configured (no default)",
			config: AIConfig{
				AnthropicAPIKey: "key",
				OpenAIAPIKey:    "key",
			},
			// Fallback to legacy Provider logic
			expected: "",
		},
		{
			name: "multiple providers configured with legacy provider set",
			config: AIConfig{
				AnthropicAPIKey: "key",
				OpenAIAPIKey:    "key",
				Provider:        AIProviderOpenAI,
			},
			expected: DefaultAIModelOpenAI,
		},
		{
			name: "legacy provider fallback - anthropic",
			config: AIConfig{
				Provider: AIProviderAnthropic,
			},
			expected: DefaultAIModelAnthropic,
		},
		{
			name: "legacy provider fallback - openrouter",
			config: AIConfig{
				Provider: AIProviderOpenRouter,
			},
			expected: DefaultAIModelOpenRouter,
		},
		{
			name: "legacy provider fallback - deepseek",
			config: AIConfig{
				Provider: AIProviderDeepSeek,
			},
			expected: DefaultAIModelDeepSeek,
		},
		{
			name: "legacy provider fallback - gemini",
			config: AIConfig{
				Provider: AIProviderGemini,
			},
			expected: DefaultAIModelGemini,
		},
		{
			name: "legacy provider fallback - ollama",
			config: AIConfig{
				Provider: AIProviderOllama,
			},
			expected: DefaultAIModelOllama,
		},
		{
			name: "ollama fallback (configured provider)",
			config: AIConfig{
				OllamaBaseURL: "http://localhost:11434",
			},
			expected: DefaultAIModelOllama,
		},
		{
			name:     "no model/provider",
			config:   AIConfig{},
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
	config := AIConfig{APIKey: "key"}
	config.ClearAPIKey()
	if config.APIKey != "" {
		t.Error("APIKey should be cleared")
	}
}

func TestAIConfig_GetPatrolInterval(t *testing.T) {
	tests := []struct {
		name     string
		config   AIConfig
		expected time.Duration
	}{
		{
			name:     "15min preset",
			config:   AIConfig{PatrolSchedulePreset: "15min"},
			expected: 15 * time.Minute,
		},
		{
			name:     "1hr preset",
			config:   AIConfig{PatrolSchedulePreset: "1hr"},
			expected: 1 * time.Hour,
		},
		{
			name:     "6hr preset",
			config:   AIConfig{PatrolSchedulePreset: "6hr"},
			expected: 6 * time.Hour,
		},
		{
			name:     "12hr preset",
			config:   AIConfig{PatrolSchedulePreset: "12hr"},
			expected: 12 * time.Hour,
		},
		{
			name:     "daily preset",
			config:   AIConfig{PatrolSchedulePreset: "daily"},
			expected: 24 * time.Hour,
		},
		{
			name:     "disabled preset",
			config:   AIConfig{PatrolSchedulePreset: "disabled"},
			expected: 0,
		},
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
	// This test catches the bug where setting patrol_interval_minutes via the API
	// clears PatrolSchedulePreset to "", but omitempty caused "" to be dropped
	// from the JSON. On reload, NewDefaultAIConfig() re-introduced "6hr" as the
	// preset, which took priority over the custom minutes.
	cfg := NewDefaultAIConfig()
	cfg.PatrolIntervalMinutes = 15
	cfg.PatrolSchedulePreset = "" // Cleared by API handler when user sets custom minutes

	// Simulate save → load round-trip via JSON
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	loaded := NewDefaultAIConfig()
	if err := json.Unmarshal(data, loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The preset must be empty after round-trip, not the default "6hr"
	if loaded.PatrolSchedulePreset != "" {
		t.Errorf("PatrolSchedulePreset should be empty after round-trip, got %q", loaded.PatrolSchedulePreset)
	}

	// The interval must be the user's 15 minutes, not the default 6 hours
	interval := loaded.GetPatrolInterval()
	if interval != 15*time.Minute {
		t.Errorf("GetPatrolInterval() = %v after round-trip, want 15m", interval)
	}
}

func TestPresetToMinutes(t *testing.T) {
	tests := []struct {
		preset   string
		expected int
	}{
		{"15min", 15},
		{"1hr", 60},
		{"6hr", 360},
		{"12hr", 720},
		{"daily", 1440},
		{"disabled", 0},
		{"unknown", 360}, // default
	}

	for _, tt := range tests {
		t.Run(tt.preset, func(t *testing.T) {
			result := PresetToMinutes(tt.preset)
			if result != tt.expected {
				t.Errorf("PresetToMinutes(%q) = %d, want %d", tt.preset, result, tt.expected)
			}
		})
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
			name:     "patrol disabled by preset",
			config:   AIConfig{Enabled: true, PatrolEnabled: true, PatrolSchedulePreset: "disabled"},
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
	if config.Provider != AIProviderAnthropic {
		t.Errorf("Default provider should be anthropic, got %q", config.Provider)
	}
	if config.PatrolIntervalMinutes != 360 {
		t.Errorf("Default patrol interval should be 360, got %d", config.PatrolIntervalMinutes)
	}
	if !config.PatrolEnabled {
		t.Error("Default patrol should be enabled")
	}
	if !config.AlertTriggeredAnalysis {
		t.Error("Default alert triggered analysis should be enabled")
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
