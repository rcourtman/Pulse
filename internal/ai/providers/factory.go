package providers

import (
	"fmt"

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

	// Get the configured timeout
	timeout := cfg.GetRequestTimeout()

	switch provider {
	case config.AIProviderAnthropic:
		// Check for OAuth first
		if cfg.IsUsingOAuth() && cfg.OAuthAccessToken != "" {
			return NewAnthropicOAuthClient(
				cfg.OAuthAccessToken,
				cfg.OAuthRefreshToken,
				cfg.OAuthExpiresAt,
				model,
				timeout,
			), nil
		}
		// Then check for per-provider API key
		apiKey := cfg.GetAPIKeyForProvider(config.AIProviderAnthropic)
		if apiKey == "" {
			return nil, fmt.Errorf("Anthropic API key not configured")
		}
		return NewAnthropicClient(apiKey, model, timeout), nil

	case config.AIProviderOpenAI:
		apiKey := cfg.GetAPIKeyForProvider(config.AIProviderOpenAI)
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key not configured")
		}
		baseURL := cfg.GetBaseURLForProvider(config.AIProviderOpenAI)
		return NewOpenAIClient(apiKey, model, baseURL, timeout), nil

	case config.AIProviderOpenRouter:
		apiKey := cfg.GetAPIKeyForProvider(config.AIProviderOpenRouter)
		if apiKey == "" {
			return nil, fmt.Errorf("OpenRouter API key not configured")
		}
		baseURL := cfg.GetBaseURLForProvider(config.AIProviderOpenRouter)
		return NewOpenAIClient(apiKey, model, baseURL, timeout), nil

	case config.AIProviderDeepSeek:
		apiKey := cfg.GetAPIKeyForProvider(config.AIProviderDeepSeek)
		if apiKey == "" {
			return nil, fmt.Errorf("DeepSeek API key not configured")
		}
		baseURL := cfg.GetBaseURLForProvider(config.AIProviderDeepSeek)
		return NewOpenAIClient(apiKey, model, baseURL, timeout), nil

	case config.AIProviderOllama:
		baseURL := cfg.GetBaseURLForProvider(config.AIProviderOllama)
		return NewOllamaClient(model, baseURL, timeout), nil

	case config.AIProviderGemini:
		apiKey := cfg.GetAPIKeyForProvider(config.AIProviderGemini)
		if apiKey == "" {
			return nil, fmt.Errorf("Gemini API key not configured")
		}
		baseURL := cfg.GetBaseURLForProvider(config.AIProviderGemini)
		return NewGeminiClient(apiKey, model, baseURL, timeout), nil

	case config.AIProviderQuickstart:
		// Quickstart uses the hosted proxy — no API key needed.
		// Note: license_id is empty here; the primary quickstart paths
		// (chat/service.go and quickstart.go) inject the real org ID.
		// This factory fallback is used by ListModels/TestConnection where
		// workspace attribution is not critical.
		return NewQuickstartClient(""), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

// NewForModel creates a Provider for a specific model, automatically detecting the provider
func NewForModel(cfg *config.AIConfig, modelString string) (Provider, error) {
	provider, model := config.ParseModelString(modelString)
	return NewForProvider(cfg, provider, model)
}
