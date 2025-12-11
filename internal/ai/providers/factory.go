package providers

import (
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// NewFromConfig creates a Provider based on the AIConfig settings
// DEPRECATED: Use NewForModel or NewForProvider for multi-provider support
func NewFromConfig(cfg *config.AIConfig) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("AI config is nil")
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("AI is not enabled")
	}

	// Try multi-provider format first (uses per-provider API keys)
	provider, model := config.ParseModelString(cfg.Model)
	if providerClient, err := NewForProvider(cfg, provider, model); err == nil {
		return providerClient, nil
	}

	// Fall back to legacy single-provider format
	switch cfg.Provider {
	case config.AIProviderAnthropic:
		// If we have an API key (from direct entry or OAuth-created), use regular client
		if cfg.APIKey != "" {
			return NewAnthropicClient(cfg.APIKey, cfg.GetModel()), nil
		}
		// Pro/Max users without org:create_api_key will use OAuth tokens directly
		if cfg.IsUsingOAuth() && cfg.OAuthAccessToken != "" {
			client := NewAnthropicOAuthClient(
				cfg.OAuthAccessToken,
				cfg.OAuthRefreshToken,
				cfg.OAuthExpiresAt,
				cfg.GetModel(),
			)
			return client, nil
		}
		return nil, fmt.Errorf("Anthropic API key is required (or use OAuth login for Pro/Max subscription)")

	case config.AIProviderOpenAI:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required")
		}
		return NewOpenAIClient(cfg.APIKey, cfg.GetModel(), cfg.GetBaseURL()), nil

	case config.AIProviderOllama:
		return NewOllamaClient(cfg.GetModel(), cfg.GetBaseURL()), nil

	case config.AIProviderDeepSeek:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("DeepSeek API key is required")
		}
		// DeepSeek uses OpenAI-compatible API
		return NewOpenAIClient(cfg.APIKey, cfg.GetModel(), cfg.GetBaseURL()), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

// NewForProvider creates a Provider for a specific provider using multi-provider credentials
func NewForProvider(cfg *config.AIConfig, provider, model string) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("AI config is nil")
	}

	switch provider {
	case config.AIProviderAnthropic:
		// Check for OAuth first
		if cfg.IsUsingOAuth() && cfg.OAuthAccessToken != "" {
			return NewAnthropicOAuthClient(
				cfg.OAuthAccessToken,
				cfg.OAuthRefreshToken,
				cfg.OAuthExpiresAt,
				model,
			), nil
		}
		// Then check for per-provider API key
		apiKey := cfg.GetAPIKeyForProvider(config.AIProviderAnthropic)
		if apiKey == "" {
			return nil, fmt.Errorf("Anthropic API key not configured")
		}
		return NewAnthropicClient(apiKey, model), nil

	case config.AIProviderOpenAI:
		apiKey := cfg.GetAPIKeyForProvider(config.AIProviderOpenAI)
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key not configured")
		}
		baseURL := cfg.GetBaseURLForProvider(config.AIProviderOpenAI)
		return NewOpenAIClient(apiKey, model, baseURL), nil

	case config.AIProviderDeepSeek:
		apiKey := cfg.GetAPIKeyForProvider(config.AIProviderDeepSeek)
		if apiKey == "" {
			return nil, fmt.Errorf("DeepSeek API key not configured")
		}
		baseURL := cfg.GetBaseURLForProvider(config.AIProviderDeepSeek)
		return NewOpenAIClient(apiKey, model, baseURL), nil

	case config.AIProviderOllama:
		baseURL := cfg.GetBaseURLForProvider(config.AIProviderOllama)
		return NewOllamaClient(model, baseURL), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

// NewForModel creates a Provider for a specific model, automatically detecting the provider
func NewForModel(cfg *config.AIConfig, modelString string) (Provider, error) {
	provider, model := config.ParseModelString(modelString)
	return NewForProvider(cfg, provider, model)
}
