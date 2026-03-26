package api

import (
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

var aiRouteAuthStoreTestMu sync.Mutex

func lockAIRouteAuthStoreTests(t *testing.T) {
	t.Helper()
	aiRouteAuthStoreTestMu.Lock()
	t.Cleanup(aiRouteAuthStoreTestMu.Unlock)
}

type aiRouteTestOptions struct {
	scopes      []string
	aiEnabled   bool
	configureAI bool
	model       string
	ollamaURL   string
}

func newAIRouteTestOptions(scopes []string, ollamaURL string) aiRouteTestOptions {
	return aiRouteTestOptions{
		scopes:      scopes,
		aiEnabled:   true,
		configureAI: true,
		model:       "ollama:llama3",
		ollamaURL:   ollamaURL,
	}
}

func setupAIRouteRouter(t *testing.T, opts aiRouteTestOptions) (*Router, string) {
	t.Helper()

	lockAIRouteAuthStoreTests(t)
	resetPersistentAuthStoresForTests()
	t.Cleanup(resetPersistentAuthStoresForTests)

	rawToken := "ai-route-token-" + t.Name() + ".12345678"
	record := newTokenRecord(t, rawToken, opts.scopes, nil)
	cfg := newTestConfigWithTokens(t, record)

	persistence := config.NewConfigPersistence(cfg.DataPath)
	if opts.configureAI {
		aiCfg := config.NewDefaultAIConfig()
		aiCfg.Enabled = opts.aiEnabled
		aiCfg.Model = opts.model
		aiCfg.OllamaBaseURL = opts.ollamaURL
		if err := persistence.SaveAIConfig(*aiCfg); err != nil {
			t.Fatalf("SaveAIConfig: %v", err)
		}
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	t.Cleanup(router.shutdownBackgroundWorkers)
	router.aiSettingsHandler.defaultConfig = cfg
	router.aiSettingsHandler.defaultPersistence = persistence
	svc := ai.NewService(persistence, nil)
	if err := svc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	router.aiSettingsHandler.defaultAIService = svc

	return router, rawToken
}
