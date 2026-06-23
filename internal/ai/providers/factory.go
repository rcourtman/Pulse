package providers

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// NewFromConfig creates a Provider based on the configured default model.
func NewFromConfig(cfg *config.AIConfig) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("Pulse Assistant config is nil")
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("Pulse Assistant is not enabled")
	}

	model := cfg.GetModel()
	if model == "" {
		return nil, fmt.Errorf("Pulse Assistant model is not configured")
	}
	return NewForModel(cfg, model)
}

// NewForProvider creates a Provider for a specific provider using multi-provider credentials
func NewForProvider(cfg *config.AIConfig, provider, model string) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("Pulse Assistant config is nil")
	}

	if normalizedProvider, normalizedModel := config.ParseModelString(model); normalizedProvider == provider && normalizedModel != "" {
		model = normalizedModel
	}

	// Get the configured timeout
	timeout := cfg.GetRequestTimeout()
	provider = strings.TrimSpace(provider)

	if config.IsOpenAICompatibleProvider(provider) {
		apiKey := cfg.GetAPIKeyForProvider(provider)
		if apiKey == "" {
			return nil, fmt.Errorf("%s API key not configured", config.AIProviderDisplayName(provider))
		}
		baseURL := cfg.GetBaseURLForProvider(provider)
		return NewOpenAICompatibleClient(provider, apiKey, model, baseURL, timeout), nil
	}

	switch provider {
	case config.AIProviderAnthropic:
		apiKey := cfg.GetAPIKeyForProvider(config.AIProviderAnthropic)
		if apiKey == "" {
			if cfg.AuthMethod == config.AuthMethodOAuth && cfg.OAuthAccessToken != "" {
				return nil, fmt.Errorf("Anthropic OAuth subscription authentication is unsupported; configure an Anthropic API key")
			}
			return nil, fmt.Errorf("Anthropic API key not configured")
		}
		return NewAnthropicClient(apiKey, model, timeout), nil

	case config.AIProviderOllama:
		baseURL := cfg.GetBaseURLForProvider(config.AIProviderOllama)
		return NewOllamaClientWithKeepAlive(model, baseURL, cfg.OllamaUsername, cfg.OllamaPassword, cfg.GetOllamaKeepAlive(), timeout)

	case config.AIProviderGemini:
		apiKey := cfg.GetAPIKeyForProvider(config.AIProviderGemini)
		if apiKey == "" {
			return nil, fmt.Errorf("Gemini API key not configured")
		}
		baseURL := cfg.GetBaseURLForProvider(config.AIProviderGemini)
		return NewGeminiClient(apiKey, model, baseURL, timeout), nil

	case config.AIProviderQuickstart:
		return nil, fmt.Errorf("quickstart provider is retired; configure a provider API key or Ollama")

	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

// NewForModel creates a Provider for a specific model, automatically detecting the provider
func NewForModel(cfg *config.AIConfig, modelString string) (Provider, error) {
	provider, model := config.ParseModelString(modelString)
	return NewForProvider(cfg, provider, model)
}
