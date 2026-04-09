package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSelectRecommendedProviderModel_PrefersNotable(t *testing.T) {
	t.Parallel()

	selected, ok := SelectRecommendedProviderModel([]providers.ModelInfo{
		{ID: "provider:older-fast", Name: "Older Fast", CreatedAt: 100},
		{ID: "provider:flagship", Name: "Flagship", CreatedAt: 50, Notable: true},
	})
	if !ok {
		t.Fatal("expected a model selection")
	}
	if selected.ID != "provider:flagship" {
		t.Fatalf("selected model = %q, want %q", selected.ID, "provider:flagship")
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
