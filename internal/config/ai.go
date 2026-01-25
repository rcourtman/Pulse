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
	Provider       string `json:"provider"`                  // DEPRECATED: legacy single provider field, kept for migration
	APIKey         string `json:"api_key"`                   // DEPRECATED: legacy single API key, kept for migration
	Model          string `json:"model"`                     // Currently selected default model (format: "provider:model-name")
	ChatModel      string `json:"chat_model,omitempty"`      // Model for interactive chat (defaults to Model)
	PatrolModel    string `json:"patrol_model,omitempty"`    // Model for background patrol (defaults to Model, can be cheaper)
	DiscoveryModel string `json:"discovery_model,omitempty"` // Model for infrastructure discovery (defaults to cheapest available, e.g., haiku)
	BaseURL        string `json:"base_url"`                  // DEPRECATED: legacy base URL, kept for migration
	AutonomousMode bool   `json:"autonomous_mode"`           // when true, AI executes commands without approval
	CustomContext  string `json:"custom_context"`            // user-provided context about their infrastructure

	// Multi-provider credentials - each provider can be configured independently
	AnthropicAPIKey string `json:"anthropic_api_key,omitempty"` // Anthropic API key
	OpenAIAPIKey    string `json:"openai_api_key,omitempty"`    // OpenAI API key
	DeepSeekAPIKey  string `json:"deepseek_api_key,omitempty"`  // DeepSeek API key
	GeminiAPIKey    string `json:"gemini_api_key,omitempty"`    // Google Gemini API key
	OllamaBaseURL   string `json:"ollama_base_url,omitempty"`   // Ollama server URL (default: http://localhost:11434)
	OpenAIBaseURL   string `json:"openai_base_url,omitempty"`   // Custom OpenAI-compatible base URL (optional)

	// OAuth fields for Claude Pro/Max subscription authentication
	AuthMethod        AuthMethod `json:"auth_method,omitempty"`         // "api_key" or "oauth" (for anthropic only)
	OAuthAccessToken  string     `json:"oauth_access_token,omitempty"`  // OAuth access token (encrypted at rest)
	OAuthRefreshToken string     `json:"oauth_refresh_token,omitempty"` // OAuth refresh token (encrypted at rest)
	OAuthExpiresAt    time.Time  `json:"oauth_expires_at,omitempty"`    // Token expiration time

	// Patrol settings for background AI monitoring
	PatrolEnabled          bool   `json:"patrol_enabled"`                     // Enable background AI health patrol
	PatrolIntervalMinutes  int    `json:"patrol_interval_minutes,omitempty"`  // How often to run quick patrols (default: 360 = 6 hours)
	PatrolSchedulePreset   string `json:"patrol_schedule_preset,omitempty"`   // User-friendly preset: "15min", "1hr", "6hr", "12hr", "daily", "disabled"
	PatrolAnalyzeNodes     bool   `json:"patrol_analyze_nodes"`               // Include Proxmox nodes in patrol
	PatrolAnalyzeGuests    bool   `json:"patrol_analyze_guests"`              // Include VMs/containers in patrol
	PatrolAnalyzeDocker    bool   `json:"patrol_analyze_docker"`              // Include Docker hosts in patrol
	PatrolAnalyzeStorage   bool   `json:"patrol_analyze_storage"`             // Include storage in patrol
	PatrolAutoFix          bool   `json:"patrol_auto_fix,omitempty"`          // When true, patrol can attempt automatic remediation (default: false, observe only)
	UseProactiveThresholds bool   `json:"use_proactive_thresholds,omitempty"` // When true, patrol warns 5-15% BEFORE alert thresholds (default: false, use exact thresholds)
	AutoFixModel           string `json:"auto_fix_model,omitempty"`           // Model for automatic remediation (defaults to PatrolModel, may want more capable model)

	// Alert-triggered AI analysis - analyze specific resources when alerts fire
	AlertTriggeredAnalysis bool `json:"alert_triggered_analysis"` // Enable AI analysis when alerts fire (token-efficient)

	// Request timeout - how long to wait for AI responses (default: 300s / 5 min)
	// Increase this for slow hardware running local models (e.g., Ollama on low-power devices)
	RequestTimeoutSeconds int `json:"request_timeout_seconds,omitempty"`

	// AI cost controls
	// Budget is expressed as an estimated USD amount over a 30-day window (pro-rated in UI for other ranges).
	CostBudgetUSD30d float64 `json:"cost_budget_usd_30d,omitempty"`

	// AI Infrastructure Control settings
	// These control whether AI can take actions on infrastructure (start/stop VMs, containers, etc.)
	ControlLevel    string   `json:"control_level,omitempty"`    // "read_only", "controlled", "autonomous"
	ProtectedGuests []string `json:"protected_guests,omitempty"` // VMIDs or names that AI cannot control

	// Patrol Autonomy settings - controls automatic investigation and remediation of findings
	PatrolAutonomyLevel           string `json:"patrol_autonomy_level,omitempty"`            // "monitor", "approval", "full"
	PatrolInvestigationBudget     int    `json:"patrol_investigation_budget,omitempty"`      // Max turns per investigation (default: 15)
	PatrolInvestigationTimeoutSec int    `json:"patrol_investigation_timeout_sec,omitempty"` // Max seconds per investigation (default: 300)
	PatrolCriticalRequireApproval bool   `json:"patrol_critical_require_approval"`           // Critical findings always require approval (default: true)

	// AI Discovery settings - controls automatic infrastructure discovery
	DiscoveryEnabled       bool `json:"discovery_enabled"`                  // Enable AI-powered infrastructure discovery
	DiscoveryIntervalHours int  `json:"discovery_interval_hours,omitempty"` // Hours between automatic re-scans (0 = manual only, default: 0)
}

// AIProvider constants
const (
	AIProviderAnthropic = "anthropic"
	AIProviderOpenAI    = "openai"
	AIProviderOllama    = "ollama"
	AIProviderDeepSeek  = "deepseek"
	AIProviderGemini    = "gemini"
)

// AI Control Level constants
const (
	// ControlLevelReadOnly - AI can only query infrastructure, no control tools available
	ControlLevelReadOnly = "read_only"
	// ControlLevelControlled - AI can execute with per-command approval
	ControlLevelControlled = "controlled"
	// ControlLevelAutonomous - AI executes without approval (requires Pro license)
	ControlLevelAutonomous = "autonomous"
)

// Patrol Autonomy Level constants
const (
	// PatrolAutonomyMonitor - Detect issues and create findings, no automatic investigation
	PatrolAutonomyMonitor = "monitor"
	// PatrolAutonomyApproval - Spawn Chat sessions to investigate, queue fixes for user approval
	PatrolAutonomyApproval = "approval"
	// PatrolAutonomyFull - Spawn Chat sessions to investigate, execute non-critical fixes automatically
	PatrolAutonomyFull = "full"
	// PatrolAutonomyAutonomous - Full autonomy, execute ALL fixes including destructive ones without approval
	// User accepts full risk - similar to "auto-accept" mode in Claude Code
	PatrolAutonomyAutonomous = "autonomous"
)

// Default patrol investigation settings
const (
	DefaultPatrolInvestigationBudget     = 15  // Max turns (tool calls) per investigation
	DefaultPatrolInvestigationTimeoutSec = 600 // 10 minutes
	MaxConcurrentInvestigations          = 3   // Max parallel investigations
	MaxInvestigationAttempts             = 3   // Max retry attempts per finding
	InvestigationCooldownHours           = 1   // Hours before re-investigating same finding
)

// Default models per provider
const (
	DefaultAIModelAnthropic = "claude-3-5-haiku-latest"
	DefaultAIModelOpenAI    = "gpt-4o"
	DefaultAIModelOllama    = "llama3"
	DefaultAIModelDeepSeek  = "deepseek-chat"    // V3.2 with tool-use support
	DefaultAIModelGemini    = "gemini-2.5-flash" // Latest stable Gemini model
	DefaultOllamaBaseURL    = "http://localhost:11434"
	DefaultDeepSeekBaseURL  = "https://api.deepseek.com/chat/completions"
	DefaultGeminiBaseURL    = "https://generativelanguage.googleapis.com/v1beta"
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
		PatrolEnabled:         true,
		PatrolIntervalMinutes: 360, // 6 hours - balance between coverage and token efficiency
		PatrolSchedulePreset:  "6hr",
		PatrolAnalyzeNodes:    true,
		PatrolAnalyzeGuests:   true,
		PatrolAnalyzeDocker:   true,
		PatrolAnalyzeStorage:  true,
		// Alert-triggered analysis is highly token-efficient - enabled by default
		AlertTriggeredAnalysis: true,
	}
}

// IsConfigured returns true if the AI config has enough info to make API calls
// For multi-provider setup, returns true if ANY provider is configured
func (c *AIConfig) IsConfigured() bool {
	if !c.Enabled {
		return false
	}

	// Check multi-provider credentials first (new format)
	if c.HasProvider(AIProviderAnthropic) || c.HasProvider(AIProviderOpenAI) ||
		c.HasProvider(AIProviderDeepSeek) || c.HasProvider(AIProviderOllama) ||
		c.HasProvider(AIProviderGemini) {
		return true
	}

	// Fall back to legacy single-provider check for backward compatibility
	switch c.Provider {
	case AIProviderAnthropic:
		if c.AuthMethod == AuthMethodOAuth {
			return c.OAuthAccessToken != ""
		}
		return c.APIKey != ""
	case AIProviderOpenAI, AIProviderDeepSeek:
		return c.APIKey != ""
	case AIProviderOllama:
		return true
	default:
		return false
	}
}

// HasProvider returns true if the specified provider has credentials configured
func (c *AIConfig) HasProvider(provider string) bool {
	switch provider {
	case AIProviderAnthropic:
		// Anthropic can use API key OR OAuth
		if c.AuthMethod == AuthMethodOAuth && c.OAuthAccessToken != "" {
			return true
		}
		return c.AnthropicAPIKey != ""
	case AIProviderOpenAI:
		return c.OpenAIAPIKey != ""
	case AIProviderDeepSeek:
		return c.DeepSeekAPIKey != ""
	case AIProviderGemini:
		return c.GeminiAPIKey != ""
	case AIProviderOllama:
		// Ollama is only "configured" if user has explicitly set a base URL
		return c.OllamaBaseURL != ""
	default:
		return false
	}
}

// GetConfiguredProviders returns a list of all providers with credentials configured
func (c *AIConfig) GetConfiguredProviders() []string {
	var providers []string
	if c.HasProvider(AIProviderAnthropic) {
		providers = append(providers, AIProviderAnthropic)
	}
	if c.HasProvider(AIProviderOpenAI) {
		providers = append(providers, AIProviderOpenAI)
	}
	if c.HasProvider(AIProviderDeepSeek) {
		providers = append(providers, AIProviderDeepSeek)
	}
	if c.HasProvider(AIProviderGemini) {
		providers = append(providers, AIProviderGemini)
	}
	if c.HasProvider(AIProviderOllama) {
		providers = append(providers, AIProviderOllama)
	}
	return providers
}

// GetAPIKeyForProvider returns the API key for the specified provider
func (c *AIConfig) GetAPIKeyForProvider(provider string) string {
	switch provider {
	case AIProviderAnthropic:
		if c.AnthropicAPIKey != "" {
			return c.AnthropicAPIKey
		}
		// Fall back to legacy API key if provider matches
		if c.Provider == AIProviderAnthropic {
			return c.APIKey
		}
	case AIProviderOpenAI:
		if c.OpenAIAPIKey != "" {
			return c.OpenAIAPIKey
		}
		if c.Provider == AIProviderOpenAI {
			return c.APIKey
		}
	case AIProviderDeepSeek:
		if c.DeepSeekAPIKey != "" {
			return c.DeepSeekAPIKey
		}
		if c.Provider == AIProviderDeepSeek {
			return c.APIKey
		}
	case AIProviderGemini:
		if c.GeminiAPIKey != "" {
			return c.GeminiAPIKey
		}
		if c.Provider == AIProviderGemini {
			return c.APIKey
		}
	}
	return ""
}

// GetBaseURLForProvider returns the base URL for the specified provider
func (c *AIConfig) GetBaseURLForProvider(provider string) string {
	switch provider {
	case AIProviderOllama:
		if c.OllamaBaseURL != "" {
			return c.OllamaBaseURL
		}
		// Fall back to legacy BaseURL if provider matches
		if c.Provider == AIProviderOllama && c.BaseURL != "" {
			return c.BaseURL
		}
		return DefaultOllamaBaseURL
	case AIProviderOpenAI:
		if c.OpenAIBaseURL != "" {
			return c.OpenAIBaseURL
		}
		return "" // Uses default OpenAI URL
	case AIProviderDeepSeek:
		return DefaultDeepSeekBaseURL
	case AIProviderGemini:
		return DefaultGeminiBaseURL
	}
	return ""
}

// IsUsingOAuth returns true if OAuth authentication is configured for Anthropic
func (c *AIConfig) IsUsingOAuth() bool {
	return c.AuthMethod == AuthMethodOAuth && c.OAuthAccessToken != ""
}

// ParseModelString parses a model string in "provider:model-name" format
// Returns the provider and model name. If no provider prefix, attempts to detect.
func ParseModelString(model string) (provider, modelName string) {
	// Check for explicit provider prefix
	for _, p := range []string{AIProviderAnthropic, AIProviderOpenAI, AIProviderDeepSeek, AIProviderGemini, AIProviderOllama} {
		prefix := p + ":"
		if len(model) > len(prefix) && model[:len(prefix)] == prefix {
			return p, model[len(prefix):]
		}
	}

	// No prefix - try to detect from model name patterns
	switch {
	case len(model) >= 6 && model[:6] == "claude":
		return AIProviderAnthropic, model
	case len(model) >= 3 && (model[:3] == "gpt" || model[:2] == "o1" || model[:2] == "o3" || model[:2] == "o4"):
		return AIProviderOpenAI, model
	case len(model) >= 8 && model[:8] == "deepseek":
		return AIProviderDeepSeek, model
	case len(model) >= 6 && model[:6] == "gemini":
		return AIProviderGemini, model
	default:
		// Assume Ollama for unrecognized models (local models have varied names)
		return AIProviderOllama, model
	}
}

// FormatModelString creates a "provider:model-name" format string
func FormatModelString(provider, modelName string) string {
	return provider + ":" + modelName
}

// GetBaseURL returns the base URL, using defaults where appropriate
// DEPRECATED: Use GetBaseURLForProvider instead
func (c *AIConfig) GetBaseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	switch c.Provider {
	case AIProviderOllama:
		return DefaultOllamaBaseURL
	case AIProviderDeepSeek:
		return DefaultDeepSeekBaseURL
	case AIProviderGemini:
		return DefaultGeminiBaseURL
	}
	return ""
}

// GetModel returns the model, using defaults where appropriate
func (c *AIConfig) GetModel() string {
	if c.Model != "" {
		return c.Model
	}

	// If only one provider is configured, use its default model
	// This handles the case where user configures Ollama but doesn't explicitly select a model
	configured := c.GetConfiguredProviders()
	if len(configured) == 1 {
		switch configured[0] {
		case AIProviderAnthropic:
			return DefaultAIModelAnthropic
		case AIProviderOpenAI:
			return DefaultAIModelOpenAI
		case AIProviderOllama:
			return DefaultAIModelOllama
		case AIProviderDeepSeek:
			return DefaultAIModelDeepSeek
		case AIProviderGemini:
			return DefaultAIModelGemini
		}
	}

	// Fall back to legacy Provider field for backwards compatibility
	switch c.Provider {
	case AIProviderAnthropic:
		return DefaultAIModelAnthropic
	case AIProviderOpenAI:
		return DefaultAIModelOpenAI
	case AIProviderOllama:
		return DefaultAIModelOllama
	case AIProviderDeepSeek:
		return DefaultAIModelDeepSeek
	case AIProviderGemini:
		return DefaultAIModelGemini
	default:
		return ""
	}
}

// GetChatModel returns the model for interactive chat conversations
// Falls back to the main Model if ChatModel is not set
func (c *AIConfig) GetChatModel() string {
	if c.ChatModel != "" {
		return c.ChatModel
	}
	return c.GetModel()
}

// GetPatrolModel returns the model for background patrol analysis
// Falls back to the main Model if PatrolModel is not set
func (c *AIConfig) GetPatrolModel() string {
	if c.PatrolModel != "" {
		return c.PatrolModel
	}
	return c.GetModel()
}

// GetDiscoveryModel returns the model for infrastructure discovery
// Falls back to the main model since discovery needs to use the same provider
func (c *AIConfig) GetDiscoveryModel() string {
	if c.DiscoveryModel != "" {
		return c.DiscoveryModel
	}
	// Fall back to the main model to ensure we use the same provider
	return c.GetModel()
}

// GetAutoFixModel returns the model for automatic remediation actions
// Falls back to PatrolModel, then to the main Model if AutoFixModel is not set
// Auto-fix may warrant a more capable model since it takes actions
func (c *AIConfig) GetAutoFixModel() string {
	if c.AutoFixModel != "" {
		return c.AutoFixModel
	}
	return c.GetPatrolModel()
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
// Note: Patrol uses local heuristics and doesn't require an AI API key,
// but still requires AI to be enabled as a master switch
func (c *AIConfig) IsPatrolEnabled() bool {
	// If AI is disabled globally, patrol is disabled
	if !c.Enabled {
		return false
	}
	// If preset is "disabled", patrol is disabled
	if c.PatrolSchedulePreset == "disabled" {
		return false
	}
	return c.PatrolEnabled
}

// IsAlertTriggeredAnalysisEnabled returns true if AI should analyze resources when alerts fire
func (c *AIConfig) IsAlertTriggeredAnalysisEnabled() bool {
	// Requires AI to be enabled as a master switch
	if !c.Enabled {
		return false
	}
	return c.AlertTriggeredAnalysis
}

// GetRequestTimeout returns the timeout duration for AI requests
// Default is 5 minutes (300 seconds) if not configured
func (c *AIConfig) GetRequestTimeout() time.Duration {
	if c.RequestTimeoutSeconds > 0 {
		return time.Duration(c.RequestTimeoutSeconds) * time.Second
	}
	return 300 * time.Second // 5 minutes default
}

// GetControlLevel returns the AI control level, defaulting to read_only if not set.
// For backwards compatibility, if ControlLevel is empty but legacy AutonomousMode is true,
// returns autonomous to preserve existing behavior.
func (c *AIConfig) GetControlLevel() string {
	if c.ControlLevel == "" {
		// Legacy migration: honor old autonomous_mode field if set
		if c.AutonomousMode {
			return ControlLevelAutonomous
		}
		return ControlLevelReadOnly
	}
	switch c.ControlLevel {
	case ControlLevelReadOnly, ControlLevelControlled, ControlLevelAutonomous:
		return c.ControlLevel
	case "suggest":
		return ControlLevelControlled
	default:
		return ControlLevelReadOnly
	}
}

// IsControlEnabled returns true if AI has any control capability beyond read-only
func (c *AIConfig) IsControlEnabled() bool {
	level := c.GetControlLevel()
	return level != ControlLevelReadOnly
}

// IsAutonomous returns true if AI is configured for autonomous operation (no approval needed)
func (c *AIConfig) IsAutonomous() bool {
	return c.GetControlLevel() == ControlLevelAutonomous
}

// IsValidControlLevel checks if a control level string is valid
func IsValidControlLevel(level string) bool {
	switch level {
	case ControlLevelReadOnly, ControlLevelControlled, ControlLevelAutonomous:
		return true
	default:
		return false
	}
}

// GetProtectedGuests returns the list of protected guests (VMIDs or names)
func (c *AIConfig) GetProtectedGuests() []string {
	if c.ProtectedGuests == nil {
		return []string{}
	}
	return c.ProtectedGuests
}

// GetPatrolAutonomyLevel returns the patrol autonomy level, defaulting to "monitor" if not set
func (c *AIConfig) GetPatrolAutonomyLevel() string {
	if c.PatrolAutonomyLevel == "" {
		return PatrolAutonomyMonitor
	}
	switch c.PatrolAutonomyLevel {
	case PatrolAutonomyMonitor, PatrolAutonomyApproval, PatrolAutonomyFull, PatrolAutonomyAutonomous:
		return c.PatrolAutonomyLevel
	default:
		return PatrolAutonomyMonitor
	}
}

// GetPatrolInvestigationBudget returns the max turns per investigation
func (c *AIConfig) GetPatrolInvestigationBudget() int {
	if c.PatrolInvestigationBudget <= 0 {
		return DefaultPatrolInvestigationBudget
	}
	// Clamp to reasonable range (5-30)
	if c.PatrolInvestigationBudget < 5 {
		return 5
	}
	if c.PatrolInvestigationBudget > 30 {
		return 30
	}
	return c.PatrolInvestigationBudget
}

// GetPatrolInvestigationTimeout returns the investigation timeout as a duration
func (c *AIConfig) GetPatrolInvestigationTimeout() time.Duration {
	if c.PatrolInvestigationTimeoutSec <= 0 {
		return time.Duration(DefaultPatrolInvestigationTimeoutSec) * time.Second
	}
	// Clamp to reasonable range (60-1800 seconds / 30 minutes)
	if c.PatrolInvestigationTimeoutSec < 60 {
		return 60 * time.Second
	}
	if c.PatrolInvestigationTimeoutSec > 1800 {
		return 1800 * time.Second
	}
	return time.Duration(c.PatrolInvestigationTimeoutSec) * time.Second
}

// ShouldCriticalRequireApproval returns whether critical findings should always require approval
// Defaults to true for safety
func (c *AIConfig) ShouldCriticalRequireApproval() bool {
	// This is a safety feature, default to true
	// The JSON field uses the default Go behavior (false when not set),
	// so we explicitly check if it was intended to be false
	// For backwards compatibility, treat unset as true
	return c.PatrolCriticalRequireApproval || c.PatrolAutonomyLevel == ""
}

// IsValidPatrolAutonomyLevel checks if a patrol autonomy level string is valid
func IsValidPatrolAutonomyLevel(level string) bool {
	switch level {
	case PatrolAutonomyMonitor, PatrolAutonomyApproval, PatrolAutonomyFull, PatrolAutonomyAutonomous:
		return true
	default:
		return false
	}
}

// IsPatrolAutonomyEnabled returns true if patrol has any autonomy beyond monitor mode
func (c *AIConfig) IsPatrolAutonomyEnabled() bool {
	level := c.GetPatrolAutonomyLevel()
	return level != PatrolAutonomyMonitor
}

// IsDiscoveryEnabled returns whether AI-powered infrastructure discovery is enabled
func (c *AIConfig) IsDiscoveryEnabled() bool {
	return c.DiscoveryEnabled
}

// GetDiscoveryInterval returns the interval between automatic discovery scans
// Returns 0 if discovery is manual-only
func (c *AIConfig) GetDiscoveryInterval() time.Duration {
	if c.DiscoveryIntervalHours <= 0 {
		return 0 // Manual only
	}
	return time.Duration(c.DiscoveryIntervalHours) * time.Hour
}
