package config

import (
	"os"
	"testing"
)

func TestOpenAICustomEndpointMayBeConfiguredWithoutKey(t *testing.T) {
	cfg := &AIConfig{OpenAIBaseURL: "http://127.0.0.1:8080/v1"}
	if !cfg.HasProvider(AIProviderOpenAI) {
		t.Fatal("custom OpenAI-compatible endpoint should configure the provider without a key")
	}
	if cfg.ProviderRequiresAPIKey(AIProviderOpenAI) {
		t.Fatal("custom OpenAI-compatible endpoint should not require a key")
	}

	cfg.OpenAIBaseURL = ""
	if cfg.HasProvider(AIProviderOpenAI) {
		t.Fatal("official OpenAI route without a key must not be configured")
	}
	if !cfg.ProviderRequiresAPIKey(AIProviderOpenAI) {
		t.Fatal("official OpenAI route must require a key")
	}
	cfg.OpenAIBaseURL = "https://api.openai.com/v1"
	if cfg.HasProvider(AIProviderOpenAI) || !cfg.ProviderRequiresAPIKey(AIProviderOpenAI) {
		t.Fatal("explicit official OpenAI URL must remain key-required")
	}
	cfg.OpenAIBaseURL = "https://api.openai.com./v1"
	if cfg.HasProvider(AIProviderOpenAI) || !cfg.ProviderRequiresAPIKey(AIProviderOpenAI) {
		t.Fatal("DNS-qualified official OpenAI URL must remain key-required")
	}
}

func TestRemoveProviderClearsOwnedSecretsEndpointOptionsAndModels(t *testing.T) {
	cfg := &AIConfig{
		Enabled:         true,
		Model:           "openai:opaque-model",
		ChatModel:       "openai:chat-model",
		PatrolModel:     "ollama:qwen3:8b",
		DiscoveryModel:  "openai:discovery-model",
		AutoFixModel:    "openai:fix-model",
		OpenAIAPIKey:    "secret",
		OpenAIBaseURL:   "http://127.0.0.1:8080/v1",
		OllamaBaseURL:   "http://127.0.0.1:11434",
		OllamaUsername:  "operator",
		OllamaPassword:  "password",
		OllamaKeepAlive: "24h",
	}

	if err := cfg.RemoveProvider(AIProviderOpenAI); err != nil {
		t.Fatalf("RemoveProvider(openai) error = %v", err)
	}
	if cfg.OpenAIAPIKey != "" || cfg.OpenAIBaseURL != "" {
		t.Fatal("OpenAI removal left provider-owned secrets or endpoint behind")
	}
	if cfg.Model != "" || cfg.ChatModel != "" || cfg.DiscoveryModel != "" || cfg.AutoFixModel != "" {
		t.Fatalf("OpenAI model selections were not cleared: %+v", cfg)
	}
	if cfg.PatrolModel != "ollama:qwen3:8b" || !cfg.Enabled {
		t.Fatal("unrelated Ollama selection/provider should remain enabled")
	}

	if err := cfg.RemoveProvider(AIProviderOllama); err != nil {
		t.Fatalf("RemoveProvider(ollama) error = %v", err)
	}
	if cfg.OllamaBaseURL != "" || cfg.OllamaUsername != "" || cfg.OllamaPassword != "" || cfg.OllamaKeepAlive != "" {
		t.Fatal("Ollama removal left provider-owned state behind")
	}
	if cfg.PatrolModel != "" || cfg.Enabled {
		t.Fatal("removing the final provider must clear its model and disable AI")
	}
}

func TestOllamaKeepAliveInheritsByDefaultAndPersistsExplicitValues(t *testing.T) {
	dir := t.TempDir()
	persistence := NewConfigPersistence(dir)

	defaults := NewDefaultAIConfig()
	if defaults.GetOllamaKeepAlive() != "" {
		t.Fatalf("default keep_alive = %q, want server inheritance", defaults.GetOllamaKeepAlive())
	}
	if err := persistence.SaveAIConfig(AIConfig{OllamaBaseURL: "http://127.0.0.1:11434", OllamaKeepAlive: "24h"}); err != nil {
		t.Fatalf("SaveAIConfig() error = %v", err)
	}
	reloaded, err := NewConfigPersistence(dir).LoadAIConfig()
	if err != nil {
		t.Fatalf("LoadAIConfig() error = %v", err)
	}
	if reloaded.GetOllamaKeepAlive() != "24h" {
		t.Fatalf("reloaded keep_alive = %q, want 24h", reloaded.GetOllamaKeepAlive())
	}

	legacyDir := t.TempDir()
	legacy := NewConfigPersistence(legacyDir)
	if err := os.WriteFile(legacy.aiFile, []byte(`{"enabled":false,"ollama_base_url":"http://127.0.0.1:11434"}`), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}
	legacyReloaded, err := legacy.LoadAIConfig()
	if err != nil {
		t.Fatalf("LoadAIConfig(legacy) error = %v", err)
	}
	if legacyReloaded.GetOllamaKeepAlive() != "" {
		t.Fatalf("legacy missing keep_alive = %q, want server inheritance", legacyReloaded.GetOllamaKeepAlive())
	}
}
