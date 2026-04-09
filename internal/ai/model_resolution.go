package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// ResolveConfiguredModel returns the effective model for the current AI config.
// When the operator has not selected a concrete provider model yet, Pulse resolves
// one from the provider's live catalog instead of relying on hardcoded vendor IDs.
func ResolveConfiguredModel(ctx context.Context, cfg *config.AIConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("Pulse Assistant config is nil")
	}

	explicit := strings.TrimSpace(cfg.GetModel())
	if explicit != "" && isModelUsableWithConfig(cfg, explicit) {
		return explicit, nil
	}

	configuredProviders := cfg.GetConfiguredProviders()
	if len(configuredProviders) == 0 {
		return "", fmt.Errorf("no provider configured")
	}

	return ResolveConfiguredProviderModel(ctx, cfg, configuredProviders[0])
}

// ResolvePreferredModelForProvider returns the best model to use for the requested
// provider. Explicit provider-scoped selections win; otherwise Pulse resolves a
// recommended model from the provider's live catalog.
func ResolvePreferredModelForProvider(ctx context.Context, cfg *config.AIConfig, provider string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("Pulse Assistant config is nil")
	}

	candidate := strings.TrimSpace(cfg.GetPreferredModelForProvider(provider))
	if candidate != "" && isModelUsableWithConfig(cfg, candidate) {
		return candidate, nil
	}

	return ResolveConfiguredProviderModel(ctx, cfg, provider)
}

// ResolveConfiguredProviderModel resolves the best current model for a configured provider.
func ResolveConfiguredProviderModel(ctx context.Context, cfg *config.AIConfig, provider string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("Pulse Assistant config is nil")
	}

	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "", fmt.Errorf("provider is required")
	}
	if provider == config.AIProviderQuickstart {
		return config.DefaultModelForProvider(provider), nil
	}
	if !cfg.HasProvider(provider) {
		return "", fmt.Errorf("%s provider is not configured", provider)
	}
	if preferred := strings.TrimSpace(cfg.GetPreferredModelForProvider(provider)); preferred != "" && isModelUsableWithConfig(cfg, preferred) {
		return preferred, nil
	}

	client, err := providers.NewForProvider(cfg, provider, "")
	if err != nil {
		if fallback := strings.TrimSpace(config.DefaultModelForProvider(provider)); fallback != "" {
			return fallback, nil
		}
		return "", fmt.Errorf("create %s provider for model resolution: %w", provider, err)
	}

	models, err := client.ListModels(ctx)
	if err != nil {
		if fallback := strings.TrimSpace(config.DefaultModelForProvider(provider)); fallback != "" {
			return fallback, nil
		}
		return "", fmt.Errorf("list %s models: %w", provider, err)
	}

	selected, ok := SelectRecommendedProviderModel(models)
	if !ok {
		if fallback := strings.TrimSpace(config.DefaultModelForProvider(provider)); fallback != "" {
			return fallback, nil
		}
		return "", fmt.Errorf("provider %s returned no usable models", provider)
	}

	return config.FormatModelString(provider, selected.ID), nil
}

// SelectRecommendedProviderModel picks the current best candidate from a provider's
// live model catalog. The policy is intentionally vendor-neutral:
// 1. prefer notable models,
// 2. then prefer models with a newer created timestamp,
// 3. then fall back to a stable lexical tie-break.
func SelectRecommendedProviderModel(models []providers.ModelInfo) (providers.ModelInfo, bool) {
	bestIndex := -1
	var best providers.ModelInfo
	for i, candidate := range models {
		if strings.TrimSpace(candidate.ID) == "" {
			continue
		}
		if bestIndex == -1 || recommendedModelBetter(candidate, i, best, bestIndex) {
			best = candidate
			bestIndex = i
		}
	}
	return best, bestIndex >= 0
}

func recommendedModelBetter(candidate providers.ModelInfo, candidateIndex int, current providers.ModelInfo, currentIndex int) bool {
	if candidate.Notable != current.Notable {
		return candidate.Notable
	}

	candidateHasCreatedAt := candidate.CreatedAt > 0
	currentHasCreatedAt := current.CreatedAt > 0
	if candidateHasCreatedAt != currentHasCreatedAt {
		return candidateHasCreatedAt
	}
	if candidate.CreatedAt != current.CreatedAt {
		return candidate.CreatedAt > current.CreatedAt
	}

	candidateLabel := recommendedModelSortKey(candidate)
	currentLabel := recommendedModelSortKey(current)
	if candidateLabel != currentLabel {
		return candidateLabel < currentLabel
	}

	return candidateIndex < currentIndex
}

func recommendedModelSortKey(model providers.ModelInfo) string {
	label := strings.TrimSpace(model.Name)
	if label == "" {
		label = strings.TrimSpace(model.ID)
	}
	return strings.ToLower(label)
}

func isModelUsableWithConfig(cfg *config.AIConfig, model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	provider, _ := config.ParseModelString(model)
	if provider == config.AIProviderQuickstart {
		return true
	}
	return cfg != nil && cfg.HasProvider(provider)
}
