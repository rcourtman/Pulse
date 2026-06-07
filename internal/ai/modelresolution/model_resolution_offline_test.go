package modelresolution

import (
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

func TestOpenRouterEquivalentChatModel_MapsDirectProviderRoutes(t *testing.T) {
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
			got, ok := OpenRouterEquivalentChatModel(cfg, tt.model)
			if !ok {
				t.Fatalf("OpenRouterEquivalentChatModel(%q) ok = false, want true", tt.model)
			}
			if got != tt.want {
				t.Fatalf("OpenRouterEquivalentChatModel(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}

func TestOpenRouterEquivalentChatModel_RejectsUnavailableOrNonGatewayRoutes(t *testing.T) {
	t.Parallel()

	cfg := &config.AIConfig{DeepSeekAPIKey: "deepseek-test"}
	if got, ok := OpenRouterEquivalentChatModel(cfg, "deepseek:deepseek-v4-pro"); ok || got != "" {
		t.Fatalf("OpenRouterEquivalentChatModel without OpenRouter = %q, %v; want empty false", got, ok)
	}

	cfg.OpenRouterAPIKey = "sk-or-test"
	for _, model := range []string{
		"openrouter:deepseek/deepseek-v4-pro",
		"ollama:llama3.2",
		"quickstart:minimax-2.5m",
		"deepseek:deepseek-embed",
	} {
		if got, ok := OpenRouterEquivalentChatModel(cfg, model); ok || got != "" {
			t.Fatalf("OpenRouterEquivalentChatModel(%q) = %q, %v; want empty false", model, got, ok)
		}
	}
}

func TestGatewayEquivalentChatModels_ReturnsConfiguredGatewayRoutes(t *testing.T) {
	t.Parallel()

	cfg := &config.AIConfig{
		OpenRouterAPIKey: "sk-or-test",
		DeepSeekAPIKey:   "deepseek-test",
	}

	got := GatewayEquivalentChatModels(cfg, "deepseek:deepseek-v4-pro")
	want := []string{"openrouter:deepseek/deepseek-v4-pro"}
	if len(got) != len(want) {
		t.Fatalf("GatewayEquivalentChatModels length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("GatewayEquivalentChatModels[%d] = %q, want %q", i, got[i], want[i])
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
