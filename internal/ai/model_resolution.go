package ai

import (
	"context"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/modelresolution"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// ResolveConfiguredModel returns the effective model for the current AI config.
// When the operator has not selected a concrete provider model yet, Pulse resolves
// one from the provider's live catalog instead of relying on hardcoded vendor IDs.
func ResolveConfiguredModel(ctx context.Context, cfg *config.AIConfig) (string, error) {
	return modelresolution.ResolveConfiguredModel(ctx, cfg)
}

// ResolveConfiguredChatModel returns the effective chat-suitable model for the
// current AI config.
func ResolveConfiguredChatModel(ctx context.Context, cfg *config.AIConfig) (string, error) {
	return modelresolution.ResolveConfiguredChatModel(ctx, cfg)
}

// ResolvePreferredModelForProvider returns the best model to use for the requested
// provider. Explicit provider-scoped selections win; otherwise Pulse resolves a
// recommended model from the provider's live catalog.
func ResolvePreferredModelForProvider(ctx context.Context, cfg *config.AIConfig, provider string) (string, error) {
	return modelresolution.ResolvePreferredModelForProvider(ctx, cfg, provider)
}

// ResolveConfiguredProviderModel resolves the best current model for a configured provider.
func ResolveConfiguredProviderModel(ctx context.Context, cfg *config.AIConfig, provider string) (string, error) {
	return modelresolution.ResolveConfiguredProviderModel(ctx, cfg, provider)
}

// ResolveConfiguredChatProviderModel resolves a chat-suitable model for a
// configured provider.
func ResolveConfiguredChatProviderModel(ctx context.Context, cfg *config.AIConfig, provider string) (string, error) {
	return modelresolution.ResolveConfiguredChatProviderModel(ctx, cfg, provider)
}

// SelectRecommendedProviderModel picks the current best candidate from a provider's
// live model catalog. The policy is intentionally vendor-neutral but Assistant first.
func SelectRecommendedProviderModel(models []providers.ModelInfo) (providers.ModelInfo, bool) {
	return modelresolution.SelectRecommendedProviderModel(models)
}

// isModelUsableWithConfig reports whether model names a non-retired provider
// with usable credentials in the supplied config.
func isModelUsableWithConfig(cfg *config.AIConfig, model string) bool {
	return modelresolution.IsModelUsableWithConfig(cfg, model)
}

func isModelUsableForChatWithConfig(cfg *config.AIConfig, model string) bool {
	return modelresolution.IsModelUsableForChatWithConfig(cfg, model)
}
