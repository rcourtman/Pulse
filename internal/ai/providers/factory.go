package providers

import (
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// NewFromConfig creates a Provider based on the AIConfig settings
func NewFromConfig(cfg *config.AIConfig) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("AI config is nil")
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("AI is not enabled")
	}

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

