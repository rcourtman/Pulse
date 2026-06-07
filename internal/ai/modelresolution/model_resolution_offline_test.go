package modelresolution

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestResolveConfiguredChatModelOffline_UsesExplicitChatModel(t *testing.T) {
	t.Parallel()

	cfg := &config.AIConfig{
		OpenRouterAPIKey: "sk-or-test",
		ChatModel:        "openrouter:qwen/qwen3.7-plus",
	}

	got, err := ResolveConfiguredChatModelOffline(cfg)
	if err != nil {
		t.Fatalf("ResolveConfiguredChatModelOffline() error = %v", err)
	}
	if want := "openrouter:qwen/qwen3.7-plus"; got != want {
		t.Fatalf("ResolveConfiguredChatModelOffline() = %q, want %q", got, want)
	}
}

func TestResolveConfiguredChatModelOffline_UsesStableProviderDefault(t *testing.T) {
	t.Parallel()

	cfg := &config.AIConfig{
		OpenRouterAPIKey: "sk-or-test",
	}

	got, err := ResolveConfiguredChatModelOffline(cfg)
	if err != nil {
		t.Fatalf("ResolveConfiguredChatModelOffline() error = %v", err)
	}
	if want := config.DefaultModelForProvider(config.AIProviderOpenRouter); got != want {
		t.Fatalf("ResolveConfiguredChatModelOffline() = %q, want %q", got, want)
	}
}

func TestResolveConfiguredChatModelOffline_ReplacesSpecializedChatModelWithStableDefault(t *testing.T) {
	t.Parallel()

	cfg := &config.AIConfig{
		OpenAIAPIKey: "sk-test",
		ChatModel:    "openai:gpt-realtime-2",
	}

	got, err := ResolveConfiguredChatModelOffline(cfg)
	if err != nil {
		t.Fatalf("ResolveConfiguredChatModelOffline() error = %v", err)
	}
	if want := config.DefaultModelForProvider(config.AIProviderOpenAI); got != want {
		t.Fatalf("ResolveConfiguredChatModelOffline() = %q, want %q", got, want)
	}
}

func TestResolveConfiguredChatModelOffline_RejectsExplicitUnconfiguredRoute(t *testing.T) {
	t.Parallel()

	cfg := &config.AIConfig{
		OpenRouterAPIKey: "sk-or-test",
		ChatModel:        "deepseek:deepseek-v4-pro",
	}

	got, err := ResolveConfiguredChatModelOffline(cfg)
	if err == nil {
		t.Fatalf("ResolveConfiguredChatModelOffline() error = nil, got model %q", got)
	}
	if !strings.Contains(err.Error(), "deepseek provider is not configured") {
		t.Fatalf("ResolveConfiguredChatModelOffline() error = %q, want DeepSeek provider configuration error", err)
	}
}

func TestResolveConfiguredChatModelOffline_RejectsDetectedUnconfiguredRoute(t *testing.T) {
	t.Parallel()

	cfg := &config.AIConfig{
		OllamaBaseURL: "http://localhost:11434",
		ChatModel:     "gpt-4",
	}

	got, err := ResolveConfiguredChatModelOffline(cfg)
	if err == nil {
		t.Fatalf("ResolveConfiguredChatModelOffline() error = nil, got model %q", got)
	}
	if !strings.Contains(err.Error(), "openai provider is not configured") {
		t.Fatalf("ResolveConfiguredChatModelOffline() error = %q, want OpenAI provider configuration error", err)
	}
}

func TestGatewayEquivalentChatModels_MapsConfiguredGatewayEquivalentRoutes(t *testing.T) {
	t.Parallel()

	cfg := &config.AIConfig{
		OpenRouterAPIKey: "sk-or-test",
		DeepSeekAPIKey:   "deepseek-test",
		OpenAIAPIKey:     "sk-test",
		AnthropicAPIKey:  "anthropic-test",
		GeminiAPIKey:     "gemini-test",
	}

	tests := []struct {
		name  string
		model string
		want  string
	}{
		{
			name:  "deepseek",
			model: "deepseek:deepseek-v4-pro",
			want:  "openrouter:deepseek/deepseek-v4-pro",
		},
		{
			name:  "openai",
			model: "openai:gpt-4o",
			want:  "openrouter:openai/gpt-4o",
		},
		{
			name:  "anthropic",
			model: "anthropic:claude-sonnet-4.5",
			want:  "openrouter:anthropic/claude-sonnet-4.5",
		},
		{
			name:  "gemini",
			model: "gemini:gemini-3.1-flash-lite",
			want:  "openrouter:google/gemini-3.1-flash-lite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GatewayEquivalentChatModels(cfg, tt.model)
			if len(got) != 1 {
				t.Fatalf("GatewayEquivalentChatModels(%q) length = %d, want 1: %#v", tt.model, len(got), got)
			}
			if got[0] != tt.want {
				t.Fatalf("GatewayEquivalentChatModels(%q)[0] = %q, want %q", tt.model, got[0], tt.want)
			}
		})
	}
}

func TestGatewayEquivalentChatModels_RejectsUnavailableOrNonEquivalentRoutes(t *testing.T) {
	t.Parallel()

	cfg := &config.AIConfig{DeepSeekAPIKey: "deepseek-test"}
	if got := GatewayEquivalentChatModels(cfg, "deepseek:deepseek-v4-pro"); len(got) != 0 {
		t.Fatalf("GatewayEquivalentChatModels without gateway = %#v, want empty", got)
	}

	cfg.OpenRouterAPIKey = "sk-or-test"
	for _, model := range []string{
		"openrouter:deepseek/deepseek-v4-pro",
		"ollama:llama3.2",
		"quickstart:minimax-2.5m",
		"deepseek:deepseek-embed",
	} {
		if got := GatewayEquivalentChatModels(cfg, model); len(got) != 0 {
			t.Fatalf("GatewayEquivalentChatModels(%q) = %#v, want empty", model, got)
		}
	}
}

func TestGatewayEquivalentChatModels_ReturnsEmptyWithoutConfiguredGateway(t *testing.T) {
	t.Parallel()

	cfg := &config.AIConfig{DeepSeekAPIKey: "deepseek-test"}
	if got := GatewayEquivalentChatModels(cfg, "deepseek:deepseek-v4-pro"); len(got) != 0 {
		t.Fatalf("GatewayEquivalentChatModels without gateway = %#v, want empty", got)
	}
}
