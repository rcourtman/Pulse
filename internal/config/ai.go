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
	AuthMethod        AuthMethod `json:"auth_method,omitempty"`         // "api_key" or "oauth" (for anthropic only)
	OAuthAccessToken  string     `json:"oauth_access_token,omitempty"`  // OAuth access token (encrypted at rest)
	OAuthRefreshToken string     `json:"oauth_refresh_token,omitempty"` // OAuth refresh token (encrypted at rest)
	OAuthExpiresAt    time.Time  `json:"oauth_expires_at,omitempty"`    // Token expiration time

	// Patrol settings for background AI monitoring
	PatrolEnabled          bool   `json:"patrol_enabled"`                      // Enable background AI health patrol
	PatrolIntervalMinutes  int    `json:"patrol_interval_minutes,omitempty"`   // How often to run quick patrols (default: 360 = 6 hours)
	PatrolSchedulePreset   string `json:"patrol_schedule_preset,omitempty"`    // User-friendly preset: "15min", "1hr", "6hr", "12hr", "daily", "disabled"
	PatrolAnalyzeNodes     bool   `json:"patrol_analyze_nodes,omitempty"`      // Include Proxmox nodes in patrol
	PatrolAnalyzeGuests    bool   `json:"patrol_analyze_guests,omitempty"`     // Include VMs/containers in patrol
	PatrolAnalyzeDocker    bool   `json:"patrol_analyze_docker,omitempty"`     // Include Docker hosts in patrol
	PatrolAnalyzeStorage   bool   `json:"patrol_analyze_storage,omitempty"`    // Include storage in patrol
	PatrolAutoFix          bool   `json:"patrol_auto_fix,omitempty"`           // When true, patrol can attempt automatic remediation (default: false, observe only)

	// Alert-triggered AI analysis - analyze specific resources when alerts fire
	AlertTriggeredAnalysis bool `json:"alert_triggered_analysis,omitempty"` // Enable AI analysis when alerts fire (token-efficient)
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
	DefaultAIModelDeepSeek  = "deepseek-chat" // V3.2 with tool-use support
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
		// Patrol defaults - enabled when AI is enabled
		// Default to 6 hour intervals (much more token-efficient than 15 min)
		PatrolEnabled:          true,
		PatrolIntervalMinutes:  360, // 6 hours - balance between coverage and token efficiency
		PatrolSchedulePreset:   "6hr",
		PatrolAnalyzeNodes:     true,
		PatrolAnalyzeGuests:    true,
		PatrolAnalyzeDocker:    true,
		PatrolAnalyzeStorage:   true,
		// Alert-triggered analysis is highly token-efficient - enabled by default
		AlertTriggeredAnalysis: true,
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

// GetPatrolInterval returns the patrol interval as a duration
// Uses the preset if set, otherwise falls back to custom minutes
func (c *AIConfig) GetPatrolInterval() time.Duration {
	// If preset is set, use it
	if c.PatrolSchedulePreset != "" {
		switch c.PatrolSchedulePreset {
		case "15min":
			return 15 * time.Minute
		case "1hr":
			return 1 * time.Hour
		case "6hr":
			return 6 * time.Hour
		case "12hr":
			return 12 * time.Hour
		case "daily":
			return 24 * time.Hour
		case "disabled":
			return 0 // Signal that scheduled patrol is disabled
		}
	}

	// Fall back to custom minutes if set
	// BUT: If PatrolIntervalMinutes is the old default (15), migrate to new default (360 = 6hr)
	// This provides better token efficiency for existing installations
	if c.PatrolIntervalMinutes > 0 {
		// Migrate old 15-minute default to new 6-hour default
		if c.PatrolIntervalMinutes == 15 && c.PatrolSchedulePreset == "" {
			return 6 * time.Hour
		}
		return time.Duration(c.PatrolIntervalMinutes) * time.Minute
	}

	return 6 * time.Hour // default to 6 hours
}

// PresetToMinutes converts a patrol schedule preset to minutes
func PresetToMinutes(preset string) int {
	switch preset {
	case "15min":
		return 15
	case "1hr":
		return 60
	case "6hr":
		return 360
	case "12hr":
		return 720
	case "daily":
		return 1440
	case "disabled":
		return 0
	default:
		return 360 // default 6hr
	}
}

// IsPatrolEnabled returns true if patrol should run
// Note: Patrol uses local heuristics and doesn't require an AI API key
func (c *AIConfig) IsPatrolEnabled() bool {
	// If preset is "disabled", patrol is disabled
	if c.PatrolSchedulePreset == "disabled" {
		return false
	}
	return c.PatrolEnabled
}

// IsAlertTriggeredAnalysisEnabled returns true if AI should analyze resources when alerts fire
func (c *AIConfig) IsAlertTriggeredAnalysisEnabled() bool {
	return c.AlertTriggeredAnalysis
}

