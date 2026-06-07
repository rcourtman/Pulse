package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSelectRecommendedProviderModel_PrefersNotable(t *testing.T) {
	t.Parallel()

	selected, ok := SelectRecommendedProviderModel([]providers.ModelInfo{
		{ID: "provider/older-fast", Name: "Older Fast", CreatedAt: 100},
		{ID: "provider/flagship", Name: "Flagship", CreatedAt: 50, Notable: true},
	})
	if !ok {
		t.Fatal("expected a model selection")
	}
	if selected.ID != "provider/flagship" {
		t.Fatalf("selected model = %q, want %q", selected.ID, "provider/flagship")
	}
}

func TestSelectRecommendedProviderModel_PrefersNewerCreatedAt(t *testing.T) {
	t.Parallel()

	selected, ok := SelectRecommendedProviderModel([]providers.ModelInfo{
		{ID: "provider:old", Name: "Old", CreatedAt: 100},
		{ID: "provider:new", Name: "New", CreatedAt: 200},
	})
	if !ok {
		t.Fatal("expected a model selection")
	}
	if selected.ID != "provider:new" {
		t.Fatalf("selected model = %q, want %q", selected.ID, "provider:new")
	}
}

func TestSelectRecommendedProviderModel_UsesLexicalTiebreak(t *testing.T) {
	t.Parallel()

	selected, ok := SelectRecommendedProviderModel([]providers.ModelInfo{
		{ID: "provider:zeta", Name: "Zeta"},
		{ID: "provider:alpha", Name: "Alpha"},
	})
	if !ok {
		t.Fatal("expected a model selection")
	}
	if selected.ID != "provider:alpha" {
		t.Fatalf("selected model = %q, want %q", selected.ID, "provider:alpha")
	}
}

func TestSelectRecommendedProviderModel_PrefersChatRouteOverSpecializedCatalogEntry(t *testing.T) {
	t.Parallel()

	selected, ok := SelectRecommendedProviderModel([]providers.ModelInfo{
		{
			ID:        "nvidia/nemotron-3.5-content-safety:free",
			Name:      "NVIDIA: Nemotron 3.5 Content Safety",
			CreatedAt: 300,
			Notable:   true,
		},
		{
			ID:        "openai/gpt-4o-mini",
			Name:      "GPT-4o mini",
			CreatedAt: 100,
		},
		{
			ID:        "anthropic/claude-sonnet-4.5",
			Name:      "Claude Sonnet 4.5",
			CreatedAt: 200,
		},
	})
	if !ok {
		t.Fatal("expected a model selection")
	}
	if selected.ID != "anthropic/claude-sonnet-4.5" {
		t.Fatalf("selected model = %q, want %q", selected.ID, "anthropic/claude-sonnet-4.5")
	}
}

func TestSelectRecommendedProviderModel_IgnoresRealtimeCatalogEntry(t *testing.T) {
	t.Parallel()

	selected, ok := SelectRecommendedProviderModel([]providers.ModelInfo{
		{
			ID:        "gpt-realtime-2",
			Name:      "GPT Realtime 2",
			CreatedAt: 300,
			Notable:   true,
		},
		{
			ID:        "gpt-4o",
			Name:      "GPT-4o",
			CreatedAt: 100,
		},
	})
	if !ok {
		t.Fatal("expected a model selection")
	}
	if selected.ID != "gpt-4o" {
		t.Fatalf("selected model = %q, want %q", selected.ID, "gpt-4o")
	}
}

func TestSelectRecommendedProviderModel_IgnoresSpecializedOnlyCatalog(t *testing.T) {
	t.Parallel()

	_, ok := SelectRecommendedProviderModel([]providers.ModelInfo{
		{ID: "openai/text-embedding-3-large", Name: "Text Embedding 3 Large", CreatedAt: 200, Notable: true},
		{ID: "openai/omni-moderation-latest", Name: "Omni Moderation", CreatedAt: 300, Notable: true},
	})
	if ok {
		t.Fatal("expected no recommendation from a specialized-only catalog")
	}
}

func TestResolveConfiguredChatProviderModel_SkipsSpecializedPreferredModel(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-realtime-2","name":"GPT Realtime 2","created":300},{"id":"gpt-4o","name":"GPT-4o","created":100}]}`))
	}))
	t.Cleanup(server.Close)

	cfg := &config.AIConfig{
		Enabled:       true,
		OpenAIAPIKey:  "sk-test",
		OpenAIBaseURL: server.URL + "/v1",
		ChatModel:     "openai:gpt-realtime-2",
	}

	got, err := ResolveConfiguredChatProviderModel(context.Background(), cfg, config.AIProviderOpenAI)
	if err != nil {
		t.Fatalf("ResolveConfiguredChatProviderModel() error = %v", err)
	}
	if want := "openai:gpt-4o"; got != want {
		t.Fatalf("ResolveConfiguredChatProviderModel() = %q, want %q", got, want)
	}
}

func TestResolveConfiguredModel_RejectsExplicitUnconfiguredRoute(t *testing.T) {
	t.Parallel()

	cfg := &config.AIConfig{
		Enabled:          true,
		OpenRouterAPIKey: "sk-or-test",
		Model:            "deepseek:deepseek-v4-pro",
	}

	got, err := ResolveConfiguredModel(context.Background(), cfg)
	if err == nil {
		t.Fatalf("ResolveConfiguredModel() error = nil, got model %q", got)
	}
	if !strings.Contains(err.Error(), "deepseek provider is not configured") {
		t.Fatalf("ResolveConfiguredModel() error = %q, want DeepSeek provider configuration error", err)
	}
}

func TestResolveConfiguredProviderModel_FallsBackToProviderDefaultWhenCatalogUnavailable(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"invalid key"}}`, http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	cfg := &config.AIConfig{
		Enabled:       true,
		OpenAIAPIKey:  "sk-test",
		OpenAIBaseURL: server.URL + "/v1",
	}

	got, err := ResolveConfiguredProviderModel(context.Background(), cfg, config.AIProviderOpenAI)
	if err != nil {
		t.Fatalf("ResolveConfiguredProviderModel() error = %v", err)
	}
	if want := config.DefaultModelForProvider(config.AIProviderOpenAI); got != want {
		t.Fatalf("ResolveConfiguredProviderModel() = %q, want %q", got, want)
	}
}
