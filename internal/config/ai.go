package config

// AIConfig holds AI feature configuration
// This is stored in ai.enc (encrypted) in the config directory
type AIConfig struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`        // "anthropic", "openai", "ollama"
	APIKey         string `json:"api_key"`         // encrypted at rest (not needed for ollama)
	Model          string `json:"model"`           // e.g., "claude-opus-4-5-20250514", "gpt-4o", "llama3"
	BaseURL        string `json:"base_url"`        // custom endpoint (required for ollama, optional for openai)
	AutonomousMode bool   `json:"autonomous_mode"` // when true, AI executes commands without approval
	CustomContext  string `json:"custom_context"`  // user-provided context about their infrastructure
}

// AIProvider constants
const (
	AIProviderAnthropic = "anthropic"
	AIProviderOpenAI    = "openai"
	AIProviderOllama    = "ollama"
)

// Default models per provider
const (
	DefaultAIModelAnthropic = "claude-opus-4-5-20251101"
	DefaultAIModelOpenAI    = "gpt-4o"
	DefaultAIModelOllama    = "llama3"
	DefaultOllamaBaseURL    = "http://localhost:11434"
)

// NewDefaultAIConfig returns an AIConfig with sensible defaults
func NewDefaultAIConfig() *AIConfig {
	return &AIConfig{
		Enabled:  false,
		Provider: AIProviderAnthropic,
		Model:    DefaultAIModelAnthropic,
	}
}

// IsConfigured returns true if the AI config has enough info to make API calls
func (c *AIConfig) IsConfigured() bool {
	if !c.Enabled {
		return false
	}

	switch c.Provider {
	case AIProviderAnthropic, AIProviderOpenAI:
		return c.APIKey != ""
	case AIProviderOllama:
		// Ollama doesn't need an API key
		return true
	default:
		return false
	}
}

// GetBaseURL returns the base URL, using defaults where appropriate
func (c *AIConfig) GetBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	if c.Provider == AIProviderOllama {
		return DefaultOllamaBaseURL
	}
	return ""
}

// GetModel returns the model, using defaults where appropriate
func (c *AIConfig) GetModel() string {
	if c.Model != "" {
		return c.Model
	}
	switch c.Provider {
	case AIProviderAnthropic:
		return DefaultAIModelAnthropic
	case AIProviderOpenAI:
		return DefaultAIModelOpenAI
	case AIProviderOllama:
		return DefaultAIModelOllama
	default:
		return ""
	}
}
