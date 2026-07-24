package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestKeylessOpenAICompatibleModelCatalogAndProviderRemovalLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("keyless endpoint received Authorization %q", got)
		}
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "model-without-a-known-prefix", "name": "Opaque local model"},
				{"id": "vendor/model:quant", "name": "Vendor model"},
			},
		})
	}))
	defer server.Close()

	dir := t.TempDir()
	persistence := config.NewConfigPersistence(dir)
	cfg := config.NewDefaultAIConfig()
	cfg.Enabled = true
	cfg.Model = "openai:model-without-a-known-prefix"
	cfg.OpenAIBaseURL = server.URL
	if err := persistence.SaveAIConfig(*cfg); err != nil {
		t.Fatalf("SaveAIConfig() error = %v", err)
	}

	service := NewService(persistence, nil)
	models, cached, err := service.ListModelsWithCache(context.Background())
	if err != nil {
		t.Fatalf("ListModelsWithCache() error = %v", err)
	}
	if cached || len(models) != 2 {
		t.Fatalf("models=%+v cached=%t, want two fresh models", models, cached)
	}
	if models[0].ID != "openai:model-without-a-known-prefix" || models[0].Provider != config.AIProviderOpenAI {
		t.Fatalf("opaque model lost authoritative provider identity: %+v", models[0])
	}
	if models[1].ID != "openai:vendor/model:quant" || models[1].Provider != config.AIProviderOpenAI {
		t.Fatalf("arbitrary model id was misclassified: %+v", models[1])
	}
	initialCacheKey := service.modelsCache.key

	if err := cfg.RemoveProvider(config.AIProviderOpenAI); err != nil {
		t.Fatalf("RemoveProvider() error = %v", err)
	}
	if err := persistence.SaveAIConfig(*cfg); err != nil {
		t.Fatalf("SaveAIConfig(removed) error = %v", err)
	}
	models, cached, err = service.ListModelsWithCache(context.Background())
	if err != nil {
		t.Fatalf("ListModelsWithCache(after removal) error = %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("removed provider models survived cache invalidation: %+v cached=%t", models, cached)
	}
	if service.modelsCache.key == initialCacheKey {
		t.Fatal("provider removal did not invalidate the model catalog cache key")
	}
}
