package config

import "time"

// AuthMethod represents how Anthropic authentication is performed
type AuthMethod string

const (
	// AuthMethodAPIKey uses a traditional API key (pay-per-use billing)
	AuthMethodAPIKey AuthMethod = "api_key"
	// AuthMethodOAuth uses OAuth tokens (subscription-based, Pro/Max plans)
	AuthMethodOAuth AuthMethod = "oauth"
)

// AIConfig holds AI feature configuration
// This is stored in ai.enc (encrypted) in the config directory
type AIConfig struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`        // "anthropic", "openai", "ollama", "deepseek"
	APIKey         string `json:"api_key"`         // encrypted at rest (not needed for ollama or oauth)
	Model          string `json:"model"`           // e.g., "claude-opus-4-5-20250514", "gpt-4o", "llama3"
	BaseURL        string `json:"base_url"`        // custom endpoint (required for ollama, optional for openai)
	AutonomousMode bool   `json:"autonomous_mode"` // when true, AI executes commands without approval
	CustomContext  string `json:"custom_context"`  // user-provided context about their infrastructure

	// OAuth fields for Claude Pro/Max subscription authentication
	AuthMethod       AuthMethod `json:"auth_method,omitempty"`        // "api_key" or "oauth" (for anthropic only)
	OAuthAccessToken string     `json:"oauth_access_token,omitempty"` // OAuth access token (encrypted at rest)
	OAuthRefreshToken string    `json:"oauth_refresh_token,omitempty"` // OAuth refresh token (encrypted at rest)
	OAuthExpiresAt   time.Time  `json:"oauth_expires_at,omitempty"`   // Token expiration time
}

// AIProvider constants
const (
	AIProviderAnthropic = "anthropic"
	AIProviderOpenAI    = "openai"
	AIProviderOllama    = "ollama"
	AIProviderDeepSeek  = "deepseek"
)

// Default models per provider
const (
	DefaultAIModelAnthropic = "claude-opus-4-5-20251101"
	DefaultAIModelOpenAI    = "gpt-4o"
	DefaultAIModelOllama    = "llama3"
	DefaultAIModelDeepSeek  = "deepseek-reasoner"
	DefaultOllamaBaseURL    = "http://localhost:11434"
	DefaultDeepSeekBaseURL  = "https://api.deepseek.com/chat/completions"
)

// NewDefaultAIConfig returns an AIConfig with sensible defaults
func NewDefaultAIConfig() *AIConfig {
	return &AIConfig{
		Enabled:    false,
		Provider:   AIProviderAnthropic,
		Model:      DefaultAIModelAnthropic,
		AuthMethod: AuthMethodAPIKey,
	}
}

// IsConfigured returns true if the AI config has enough info to make API calls
func (c *AIConfig) IsConfigured() bool {
	if !c.Enabled {
		return false
	}

	switch c.Provider {
	case AIProviderAnthropic:
		// Anthropic can use API key OR OAuth
		if c.AuthMethod == AuthMethodOAuth {
			return c.OAuthAccessToken != ""
		}
		return c.APIKey != ""
	case AIProviderOpenAI, AIProviderDeepSeek:
		return c.APIKey != ""
	case AIProviderOllama:
		// Ollama doesn't need an API key
		return true
	default:
		return false
	}
}

// IsUsingOAuth returns true if OAuth authentication is configured for Anthropic
func (c *AIConfig) IsUsingOAuth() bool {
	return c.Provider == AIProviderAnthropic && c.AuthMethod == AuthMethodOAuth && c.OAuthAccessToken != ""
}

// GetBaseURL returns the base URL, using defaults where appropriate
func (c *AIConfig) GetBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	switch c.Provider {
	case AIProviderOllama:
		return DefaultOllamaBaseURL
	case AIProviderDeepSeek:
		return DefaultDeepSeekBaseURL
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
	case AIProviderDeepSeek:
		return DefaultAIModelDeepSeek
	default:
		return ""
	}
}

// ClearOAuthTokens clears OAuth tokens (used when switching back to API key auth)
func (c *AIConfig) ClearOAuthTokens() {
	c.OAuthAccessToken = ""
	c.OAuthRefreshToken = ""
	c.OAuthExpiresAt = time.Time{}
}

// ClearAPIKey clears the API key (used when switching to OAuth auth)
func (c *AIConfig) ClearAPIKey() {
	c.APIKey = ""
}
