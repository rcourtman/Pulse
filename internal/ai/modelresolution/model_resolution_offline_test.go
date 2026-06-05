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
