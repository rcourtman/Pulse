package config

import "strings"

// AIProviderProtocol identifies the transport/runtime shape a provider uses.
type AIProviderProtocol string

const (
	AIProviderProtocolAnthropic        AIProviderProtocol = "anthropic"
	AIProviderProtocolOpenAICompatible AIProviderProtocol = "openai_compatible"
	AIProviderProtocolGemini           AIProviderProtocol = "gemini"
	AIProviderProtocolOllama           AIProviderProtocol = "ollama"
	AIProviderProtocolRetired          AIProviderProtocol = "retired"
)

// AIProviderModelDefinition is the config-layer representation of catalog
// models that Pulse should present even when a provider's /models endpoint is
// absent, incomplete, or temporarily unavailable.
type AIProviderModelDefinition struct {
	ID          string
	Name        string
	Description string
	Notable     bool
}

// AIProviderDefinition is the canonical internal registry record for provider
// metadata and runtime capability selection.
type AIProviderDefinition struct {
	ID                  string
	DisplayName         string
	Description         string
	Protocol            AIProviderProtocol
	DefaultModel        string
	DefaultBaseURL      string
	APIKeyField         string
	ConfiguredField     string
	ClearKeyField       string
	BaseURLField        string
	RequiresAPIKey      bool
	UserConfigurable    bool
	Gateway             bool
	ModelsDevProviderID string
	EnvVars             []string
	DocsURL             string
	FallbackModels      []AIProviderModelDefinition
}

func aiProviderDefinitions() []AIProviderDefinition {
	return []AIProviderDefinition{
		{
			ID:               AIProviderAnthropic,
			DisplayName:      "Anthropic",
			Description:      "Claude models from Anthropic",
			Protocol:         AIProviderProtocolAnthropic,
			DefaultModel:     "claude-3-5-sonnet-latest",
			APIKeyField:      "anthropic_api_key",
			ConfiguredField:  "anthropic_configured",
			ClearKeyField:    "clear_anthropic_key",
			RequiresAPIKey:   true,
			UserConfigurable: true,
			EnvVars:          []string{"ANTHROPIC_API_KEY"},
			DocsURL:          "https://docs.anthropic.com/en/api/getting-started",
		},
		{
			ID:               AIProviderOpenAI,
			DisplayName:      "OpenAI",
			Description:      "GPT and reasoning models from OpenAI, or a custom OpenAI-compatible endpoint",
			Protocol:         AIProviderProtocolOpenAICompatible,
			DefaultModel:     "gpt-4o",
			APIKeyField:      "openai_api_key",
			ConfiguredField:  "openai_configured",
			ClearKeyField:    "clear_openai_key",
			BaseURLField:     "openai_base_url",
			RequiresAPIKey:   true,
			UserConfigurable: true,
			EnvVars:          []string{"OPENAI_API_KEY"},
			DocsURL:          "https://platform.openai.com/docs/api-reference/chat",
		},
		{
			ID:                  AIProviderOpenRouter,
			DisplayName:         "OpenRouter",
			Description:         "Gateway for OpenAI-compatible hosted models",
			Protocol:            AIProviderProtocolOpenAICompatible,
			DefaultModel:        "openai/gpt-4o-mini",
			DefaultBaseURL:      DefaultOpenRouterBaseURL,
			APIKeyField:         "openrouter_api_key",
			ConfiguredField:     "openrouter_configured",
			ClearKeyField:       "clear_openrouter_key",
			RequiresAPIKey:      true,
			UserConfigurable:    true,
			Gateway:             true,
			ModelsDevProviderID: "openrouter",
			EnvVars:             []string{"OPENROUTER_API_KEY"},
			DocsURL:             "https://openrouter.ai/docs",
		},
		{
			ID:                  AIProviderDeepSeek,
			DisplayName:         "DeepSeek",
			Description:         "DeepSeek direct API models",
			Protocol:            AIProviderProtocolOpenAICompatible,
			DefaultModel:        DeepSeekModelV4Flash,
			DefaultBaseURL:      DefaultDeepSeekBaseURL,
			APIKeyField:         "deepseek_api_key",
			ConfiguredField:     "deepseek_configured",
			ClearKeyField:       "clear_deepseek_key",
			RequiresAPIKey:      true,
			UserConfigurable:    true,
			ModelsDevProviderID: "deepseek",
			EnvVars:             []string{"DEEPSEEK_API_KEY"},
			DocsURL:             "https://api-docs.deepseek.com/",
			FallbackModels: []AIProviderModelDefinition{
				{
					ID:          DeepSeekModelV4Flash,
					Name:        "DeepSeek V4 Flash",
					Description: "DeepSeek: current V4 Flash direct API model",
					Notable:     true,
				},
				{
					ID:          DeepSeekModelV4Pro,
					Name:        "DeepSeek V4 Pro",
					Description: "DeepSeek: current V4 Pro direct API model",
					Notable:     true,
				},
				{
					ID:          DeepSeekModelLegacyChat,
					Name:        "DeepSeek Chat (legacy alias)",
					Description: "DeepSeek: legacy alias currently routing to V4 Flash",
				},
				{
					ID:          DeepSeekModelLegacyReasoner,
					Name:        "DeepSeek Reasoner (legacy alias)",
					Description: "DeepSeek: legacy alias currently routing to V4 Flash",
				},
			},
		},
		{
			ID:                  AIProviderGemini,
			DisplayName:         "Google Gemini",
			Description:         "Gemini models from Google",
			Protocol:            AIProviderProtocolGemini,
			DefaultModel:        "gemini-1.5-pro",
			DefaultBaseURL:      DefaultGeminiBaseURL,
			APIKeyField:         "gemini_api_key",
			ConfiguredField:     "gemini_configured",
			ClearKeyField:       "clear_gemini_key",
			RequiresAPIKey:      true,
			UserConfigurable:    true,
			ModelsDevProviderID: "google",
			EnvVars:             []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"},
			DocsURL:             "https://ai.google.dev/gemini-api/docs",
		},
		{
			ID:                  AIProviderZai,
			DisplayName:         "Z.ai",
			Description:         "GLM models from Z.ai using the OpenAI-compatible API",
			Protocol:            AIProviderProtocolOpenAICompatible,
			DefaultModel:        "glm-5.2",
			DefaultBaseURL:      DefaultZaiBaseURL,
			APIKeyField:         "zai_api_key",
			ConfiguredField:     "zai_configured",
			ClearKeyField:       "clear_zai_key",
			RequiresAPIKey:      true,
			UserConfigurable:    true,
			ModelsDevProviderID: "zai",
			EnvVars:             []string{"ZHIPU_API_KEY", "ZAI_API_KEY"},
			DocsURL:             "https://docs.z.ai/guides/develop/openai/python",
			FallbackModels: []AIProviderModelDefinition{
				{
					ID:          "glm-5.2",
					Name:        "GLM 5.2",
					Description: "Z.ai: GLM 5.2 direct API model",
					Notable:     true,
				},
			},
		},
		{
			ID:                  AIProviderGroq,
			DisplayName:         "Groq",
			Description:         "Groq hosted models using the OpenAI-compatible API",
			Protocol:            AIProviderProtocolOpenAICompatible,
			DefaultModel:        "llama-3.3-70b-versatile",
			DefaultBaseURL:      DefaultGroqBaseURL,
			APIKeyField:         "groq_api_key",
			ConfiguredField:     "groq_configured",
			ClearKeyField:       "clear_groq_key",
			RequiresAPIKey:      true,
			UserConfigurable:    true,
			ModelsDevProviderID: "groq",
			EnvVars:             []string{"GROQ_API_KEY"},
			DocsURL:             "https://console.groq.com/docs/openai",
		},
		{
			ID:                  AIProviderMistral,
			DisplayName:         "Mistral",
			Description:         "Mistral models using the OpenAI-compatible chat API",
			Protocol:            AIProviderProtocolOpenAICompatible,
			DefaultModel:        "mistral-large-latest",
			DefaultBaseURL:      DefaultMistralBaseURL,
			APIKeyField:         "mistral_api_key",
			ConfiguredField:     "mistral_configured",
			ClearKeyField:       "clear_mistral_key",
			RequiresAPIKey:      true,
			UserConfigurable:    true,
			ModelsDevProviderID: "mistral",
			EnvVars:             []string{"MISTRAL_API_KEY"},
			DocsURL:             "https://docs.mistral.ai/resources/migration-guides",
		},
		{
			ID:                  AIProviderCerebras,
			DisplayName:         "Cerebras",
			Description:         "Cerebras Inference models using the OpenAI-compatible API",
			Protocol:            AIProviderProtocolOpenAICompatible,
			DefaultModel:        "llama-4-scout-17b-16e-instruct",
			DefaultBaseURL:      DefaultCerebrasBaseURL,
			APIKeyField:         "cerebras_api_key",
			ConfiguredField:     "cerebras_configured",
			ClearKeyField:       "clear_cerebras_key",
			RequiresAPIKey:      true,
			UserConfigurable:    true,
			ModelsDevProviderID: "cerebras",
			EnvVars:             []string{"CEREBRAS_API_KEY"},
			DocsURL:             "https://inference-docs.cerebras.ai/resources/openai",
		},
		{
			ID:                  AIProviderTogether,
			DisplayName:         "Together AI",
			Description:         "Together AI models using the OpenAI-compatible API",
			Protocol:            AIProviderProtocolOpenAICompatible,
			DefaultModel:        "meta-llama/Llama-3.3-70B-Instruct-Turbo",
			DefaultBaseURL:      DefaultTogetherBaseURL,
			APIKeyField:         "together_api_key",
			ConfiguredField:     "together_configured",
			ClearKeyField:       "clear_together_key",
			RequiresAPIKey:      true,
			UserConfigurable:    true,
			ModelsDevProviderID: "togetherai",
			EnvVars:             []string{"TOGETHER_API_KEY"},
			DocsURL:             "https://docs.together.ai/docs/inference/openai-compatibility",
		},
		{
			ID:                  AIProviderFireworks,
			DisplayName:         "Fireworks AI",
			Description:         "Fireworks models using the OpenAI-compatible API",
			Protocol:            AIProviderProtocolOpenAICompatible,
			DefaultModel:        "accounts/fireworks/models/llama-v3p1-70b-instruct",
			DefaultBaseURL:      DefaultFireworksBaseURL,
			APIKeyField:         "fireworks_api_key",
			ConfiguredField:     "fireworks_configured",
			ClearKeyField:       "clear_fireworks_key",
			RequiresAPIKey:      true,
			UserConfigurable:    true,
			ModelsDevProviderID: "fireworks-ai",
			EnvVars:             []string{"FIREWORKS_API_KEY"},
			DocsURL:             "https://docs.fireworks.ai/tools-sdks/openai-compatibility",
		},
		{
			ID:               AIProviderOllama,
			DisplayName:      "Ollama",
			Description:      "Local models served by Ollama",
			Protocol:         AIProviderProtocolOllama,
			DefaultModel:     "llama3.2",
			DefaultBaseURL:   DefaultOllamaBaseURL,
			ConfiguredField:  "ollama_configured",
			ClearKeyField:    "clear_ollama_url",
			BaseURLField:     "ollama_base_url",
			UserConfigurable: true,
			DocsURL:          "https://ollama.com",
		},
		{
			ID:              AIProviderQuickstart,
			DisplayName:     "Pulse hosted quickstart",
			Description:     "Retired pre-GA hosted proxy marker retained for migration cleanup",
			Protocol:        AIProviderProtocolRetired,
			DefaultModel:    "",
			ConfiguredField: "quickstart_configured",
		},
	}
}

// AIProviderDefinitions returns the ordered provider registry. The slice and
// nested slices are copies so callers cannot mutate the canonical records.
func AIProviderDefinitions() []AIProviderDefinition {
	defs := aiProviderDefinitions()
	for i := range defs {
		defs[i].EnvVars = append([]string(nil), defs[i].EnvVars...)
		defs[i].FallbackModels = append([]AIProviderModelDefinition(nil), defs[i].FallbackModels...)
	}
	return defs
}

// AIConfigurableProviderDefinitions returns user-configurable providers in UI
// and model-list order.
func AIConfigurableProviderDefinitions() []AIProviderDefinition {
	defs := AIProviderDefinitions()
	out := make([]AIProviderDefinition, 0, len(defs))
	for _, def := range defs {
		if def.UserConfigurable {
			out = append(out, def)
		}
	}
	return out
}

func LookupAIProviderDefinition(provider string) (AIProviderDefinition, bool) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return AIProviderDefinition{}, false
	}
	for _, def := range aiProviderDefinitions() {
		if def.ID == provider {
			def.EnvVars = append([]string(nil), def.EnvVars...)
			def.FallbackModels = append([]AIProviderModelDefinition(nil), def.FallbackModels...)
			return def, true
		}
	}
	return AIProviderDefinition{}, false
}

func IsKnownAIProvider(provider string) bool {
	_, ok := LookupAIProviderDefinition(provider)
	return ok
}

func IsOpenAICompatibleProvider(provider string) bool {
	def, ok := LookupAIProviderDefinition(provider)
	return ok && def.Protocol == AIProviderProtocolOpenAICompatible
}

func AIProviderFallbackModels(provider string) []AIProviderModelDefinition {
	def, ok := LookupAIProviderDefinition(provider)
	if !ok || len(def.FallbackModels) == 0 {
		return nil
	}
	return append([]AIProviderModelDefinition(nil), def.FallbackModels...)
}

func AIProviderDisplayName(provider string) string {
	if def, ok := LookupAIProviderDefinition(provider); ok && def.DisplayName != "" {
		return def.DisplayName
	}
	return provider
}
