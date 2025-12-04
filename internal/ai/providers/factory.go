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
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("Anthropic API key is required")
		}
		return NewAnthropicClient(cfg.APIKey, cfg.GetModel()), nil

	case config.AIProviderOpenAI:
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required")
		}
		return NewOpenAIClient(cfg.APIKey, cfg.GetModel(), cfg.GetBaseURL()), nil

	case config.AIProviderOllama:
		return NewOllamaClient(cfg.GetModel(), cfg.GetBaseURL()), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}
