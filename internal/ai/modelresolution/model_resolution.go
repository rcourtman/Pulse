package modelresolution

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
	if explicit != "" {
		if IsModelUsableWithConfig(cfg, explicit) {
			return explicit, nil
		}
		return "", selectedModelProviderError(cfg, explicit)
	}

	configuredProviders := cfg.GetConfiguredProviders()
	if len(configuredProviders) == 0 {
		return "", fmt.Errorf("no provider configured")
	}

	return ResolveConfiguredProviderModel(ctx, cfg, configuredProviders[0])
}

// ResolveConfiguredChatModel returns the effective model for interactive chat.
// A chat-specific selection wins; otherwise this follows the same configured
// provider resolution as the shared AI runtime.
func ResolveConfiguredChatModel(ctx context.Context, cfg *config.AIConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("Pulse Assistant config is nil")
	}

	explicit := strings.TrimSpace(cfg.GetChatModel())
	if explicit != "" && IsModelUsableForChatWithConfig(cfg, explicit) {
		return explicit, nil
	}
	if explicit != "" {
		return "", selectedChatModelProviderError(cfg, explicit)
	}

	configuredProviders := cfg.GetConfiguredProviders()
	if len(configuredProviders) == 0 {
		return "", fmt.Errorf("no provider configured")
	}

	return ResolveConfiguredChatProviderModel(ctx, cfg, configuredProviders[0])
}

// ResolveConfiguredChatModelOffline returns the effective chat model without
// calling a provider catalog. Interactive chat uses this path so first-token
// latency is controlled by the selected provider stream, not a preflight
// /models request.
func ResolveConfiguredChatModelOffline(cfg *config.AIConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("Pulse Assistant config is nil")
	}

	explicit := strings.TrimSpace(cfg.GetChatModel())
	if explicit != "" && IsModelUsableForChatWithConfig(cfg, explicit) {
		return explicit, nil
	}
	if explicit != "" {
		return "", selectedChatModelProviderError(cfg, explicit)
	}

	configuredProviders := cfg.GetConfiguredProviders()
	if len(configuredProviders) == 0 {
		return "", fmt.Errorf("no provider configured")
	}

	return ResolveConfiguredChatProviderModelOffline(cfg, configuredProviders[0])
}

func selectedModelProvider(cfg *config.AIConfig, model string) (string, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", fmt.Errorf("selected model route is empty")
	}
	provider, _ := config.ParseModelString(model)
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "", fmt.Errorf("selected model route %q does not name a provider", model)
	}
	if provider == config.AIProviderQuickstart {
		return "", fmt.Errorf("quickstart provider is retired; configure a provider API key or Ollama")
	}
	if cfg == nil || !cfg.HasProvider(provider) {
		return "", fmt.Errorf("%s provider is not configured for selected model route %q", provider, model)
	}
	return provider, nil
}

func selectedModelProviderError(cfg *config.AIConfig, model string) error {
	if _, err := selectedModelProvider(cfg, model); err != nil {
		return err
	}
	return fmt.Errorf("selected model route %q is not usable with the current Pulse Assistant config", strings.TrimSpace(model))
}

func selectedChatModelProviderError(cfg *config.AIConfig, model string) error {
	if _, err := selectedModelProvider(cfg, model); err != nil {
		return err
	}
	return fmt.Errorf("selected chat model route %q is not usable for Assistant chat with the current Pulse Assistant config", strings.TrimSpace(model))
}

// ResolveConfiguredChatProviderModelOffline resolves a chat-suitable model for
// a configured provider using only explicit config and stable provider defaults.
func ResolveConfiguredChatProviderModelOffline(cfg *config.AIConfig, provider string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("Pulse Assistant config is nil")
	}

	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "", fmt.Errorf("provider is required")
	}
	if provider == config.AIProviderQuickstart {
		return "", fmt.Errorf("quickstart provider is retired; configure a provider API key or Ollama")
	}
	if !cfg.HasProvider(provider) {
		return "", fmt.Errorf("%s provider is not configured", provider)
	}
	if preferred := strings.TrimSpace(cfg.GetPreferredModelForProvider(provider)); preferred != "" && modelUsableWithConfig(cfg, preferred, true) {
		return preferred, nil
	}
	if fallback := strings.TrimSpace(config.DefaultModelForProvider(provider)); fallback != "" && modelUsableWithConfig(cfg, fallback, true) {
		return fallback, nil
	}
	return "", fmt.Errorf("no chat model configured for %s provider", provider)
}

type gatewayEquivalentResolver struct {
	gatewayProvider  string
	ownerForProvider func(provider string) string
}

var gatewayEquivalentResolvers = []gatewayEquivalentResolver{
	{
		gatewayProvider:  config.AIProviderOpenRouter,
		ownerForProvider: openRouterOwnerForProvider,
	},
}

// GatewayEquivalentChatModels maps a configured direct-provider chat model to
// equivalent routes through configured gateway providers. Gateway route syntax
// belongs in this model-resolution registry; chat execution consumes candidates
// without encoding provider-specific fallback policy.
func GatewayEquivalentChatModels(cfg *config.AIConfig, model string) []string {
	if cfg == nil {
		return nil
	}
	routes := make([]string, 0, len(gatewayEquivalentResolvers))
	seen := make(map[string]struct{}, len(gatewayEquivalentResolvers))
	for _, resolver := range gatewayEquivalentResolvers {
		candidate, ok := resolver.resolve(cfg, model)
		if !ok {
			continue
		}
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		routes = append(routes, candidate)
	}
	return routes
}

func (r gatewayEquivalentResolver) resolve(cfg *config.AIConfig, model string) (string, bool) {
	gatewayProvider := strings.TrimSpace(r.gatewayProvider)
	if cfg == nil || gatewayProvider == "" || r.ownerForProvider == nil || !cfg.HasProvider(gatewayProvider) {
		return "", false
	}
	provider, modelName := config.ParseModelString(strings.TrimSpace(model))
	modelName = strings.TrimSpace(modelName)
	if provider == "" || modelName == "" {
		return "", false
	}
	if provider == gatewayProvider ||
		provider == config.AIProviderOllama ||
		provider == config.AIProviderQuickstart {
		return "", false
	}
	gatewayOwner := strings.TrimSpace(r.ownerForProvider(provider))
	if gatewayOwner == "" {
		return "", false
	}
	candidate := config.FormatModelString(gatewayProvider, gatewayOwner+"/"+strings.TrimPrefix(modelName, gatewayOwner+"/"))
	if !IsModelUsableForChatWithConfig(cfg, candidate) {
		return "", false
	}
	return candidate, true
}

func openRouterOwnerForProvider(provider string) string {
	switch strings.TrimSpace(provider) {
	case config.AIProviderAnthropic:
		return "anthropic"
	case config.AIProviderOpenAI:
		return "openai"
	case config.AIProviderDeepSeek:
		return "deepseek"
	case config.AIProviderGemini:
		return "google"
	default:
		return ""
	}
}

// ResolvePreferredModelForProvider returns the best model to use for the requested
// provider. Explicit provider-scoped selections win; otherwise Pulse resolves a
// recommended model from the provider's live catalog.
func ResolvePreferredModelForProvider(ctx context.Context, cfg *config.AIConfig, provider string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("Pulse Assistant config is nil")
	}

	candidate := strings.TrimSpace(cfg.GetPreferredModelForProvider(provider))
	if candidate != "" && IsModelUsableWithConfig(cfg, candidate) {
		return candidate, nil
	}

	return ResolveConfiguredProviderModel(ctx, cfg, provider)
}

// ResolveConfiguredProviderModel resolves the best current model for a configured provider.
func ResolveConfiguredProviderModel(ctx context.Context, cfg *config.AIConfig, provider string) (string, error) {
	return resolveConfiguredProviderModel(ctx, cfg, provider, false)
}

// ResolveConfiguredChatProviderModel resolves a chat-suitable model for a
// configured provider. Explicit provider preferences win only when they are
// suitable for Assistant chat; specialized endpoints such as realtime, audio,
// moderation, and embedding models are skipped.
func ResolveConfiguredChatProviderModel(ctx context.Context, cfg *config.AIConfig, provider string) (string, error) {
	return resolveConfiguredProviderModel(ctx, cfg, provider, true)
}

func resolveConfiguredProviderModel(ctx context.Context, cfg *config.AIConfig, provider string, chatOnly bool) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("Pulse Assistant config is nil")
	}

	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "", fmt.Errorf("provider is required")
	}
	if provider == config.AIProviderQuickstart {
		return "", fmt.Errorf("quickstart provider is retired; configure a provider API key or Ollama")
	}
	if !cfg.HasProvider(provider) {
		return "", fmt.Errorf("%s provider is not configured", provider)
	}
	if preferred := strings.TrimSpace(cfg.GetPreferredModelForProvider(provider)); preferred != "" && modelUsableWithConfig(cfg, preferred, chatOnly) {
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
// live model catalog. The policy is intentionally vendor-neutral but Assistant
// first:
//  1. ignore obvious non-chat catalog entries such as moderation, embeddings, and
//     content-safety endpoints,
//  2. prefer likely chat/instruction models over unknown model families,
//  3. prefer notable models,
//  4. then prefer models with a newer created timestamp,
//  5. then fall back to a stable lexical tie-break.
func SelectRecommendedProviderModel(models []providers.ModelInfo) (providers.ModelInfo, bool) {
	bestIndex := -1
	var best providers.ModelInfo
	for i, candidate := range models {
		if strings.TrimSpace(candidate.ID) == "" {
			continue
		}
		if recommendedModelSuitabilityRank(candidate) >= recommendedModelRankSpecialized {
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
	candidateRank := recommendedModelSuitabilityRank(candidate)
	currentRank := recommendedModelSuitabilityRank(current)
	if candidateRank != currentRank {
		return candidateRank < currentRank
	}

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

const (
	recommendedModelRankChat = iota
	recommendedModelRankUnknown
	recommendedModelRankSpecialized
)

func recommendedModelSuitabilityRank(model providers.ModelInfo) int {
	return recommendedModelLabelSuitabilityRank(model.ID + " " + model.Name)
}

func recommendedModelLabelSuitabilityRank(label string) int {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" {
		return recommendedModelRankSpecialized
	}

	for _, marker := range []string{
		"audio",
		"classifier",
		"content-safety",
		"embed",
		"guard",
		"image",
		"moderation",
		"realtime",
		"rerank",
		"reward",
		"speech",
		"transcribe",
		"transcription",
		"tts",
		"whisper",
	} {
		if strings.Contains(label, marker) {
			return recommendedModelRankSpecialized
		}
	}

	for _, marker := range []string{
		"chat",
		"claude",
		"command",
		"deepseek",
		"flash",
		"gemini",
		"gpt",
		"grok",
		"haiku",
		"instruct",
		"kimi",
		"llama",
		"mistral",
		"mixtral",
		"o1",
		"o3",
		"o4",
		"opus",
		"qwen",
		"sonnet",
	} {
		if strings.Contains(label, marker) {
			return recommendedModelRankChat
		}
	}

	return recommendedModelRankUnknown
}

func recommendedModelSortKey(model providers.ModelInfo) string {
	label := strings.TrimSpace(model.Name)
	if label == "" {
		label = strings.TrimSpace(model.ID)
	}
	return strings.ToLower(label)
}

// IsModelUsableWithConfig reports whether model names a non-retired provider
// with usable credentials in the supplied config.
func IsModelUsableWithConfig(cfg *config.AIConfig, model string) bool {
	return modelUsableWithConfig(cfg, model, false)
}

// IsModelUsableForChatWithConfig reports whether model names a configured
// provider and does not look like a specialized non-chat endpoint.
func IsModelUsableForChatWithConfig(cfg *config.AIConfig, model string) bool {
	return modelUsableWithConfig(cfg, model, true)
}

func modelUsableWithConfig(cfg *config.AIConfig, model string, chatOnly bool) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	provider, _ := config.ParseModelString(model)
	if provider == config.AIProviderQuickstart {
		return false
	}
	if cfg == nil || !cfg.HasProvider(provider) {
		return false
	}
	return !chatOnly || IsModelLikelyChatSuitable(model)
}

// IsModelLikelyChatSuitable reports whether a model string looks usable for
// text chat. Unknown model families are allowed; obvious specialized endpoints
// are rejected.
func IsModelLikelyChatSuitable(model string) bool {
	return recommendedModelLabelSuitabilityRank(model) < recommendedModelRankSpecialized
}
