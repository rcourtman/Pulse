package config

import (
	"strings"
	"time"
)

// AuthMethod represents how Anthropic authentication is performed
type AuthMethod string

const (
	// AuthMethodAPIKey uses a traditional API key (pay-per-use billing)
	AuthMethodAPIKey AuthMethod = "api_key"
	// AuthMethodOAuth uses OAuth tokens (subscription-based, Pro/Max plans)
	AuthMethodOAuth AuthMethod = "oauth"
)

// PatrolEventTriggerSettings describes which event sources may enqueue scoped patrol runs.
type PatrolEventTriggerSettings struct {
	AlertTriggersEnabled   bool
	AnomalyTriggersEnabled bool
}

// AIConfig holds AI feature configuration
// This is stored in ai.enc (encrypted) in the config directory
type AIConfig struct {
	Enabled        bool   `json:"enabled"`
	Model          string `json:"model"`                     // Currently selected default model (format: "provider:model-name")
	ChatModel      string `json:"chat_model,omitempty"`      // Model for interactive chat (defaults to Model)
	PatrolModel    string `json:"patrol_model,omitempty"`    // Model for background patrol (defaults to Model, can be cheaper)
	DiscoveryModel string `json:"discovery_model,omitempty"` // Model for infrastructure discovery (defaults to cheapest available, e.g., haiku)
	CustomContext  string `json:"custom_context"`            // user-provided context about their infrastructure

	// Multi-provider credentials - each provider can be configured independently
	AnthropicAPIKey  string `json:"anthropic_api_key,omitempty"`  // Anthropic API key
	OpenAIAPIKey     string `json:"openai_api_key,omitempty"`     // OpenAI API key
	OpenRouterAPIKey string `json:"openrouter_api_key,omitempty"` // OpenRouter API key
	DeepSeekAPIKey   string `json:"deepseek_api_key,omitempty"`   // DeepSeek API key
	GeminiAPIKey     string `json:"gemini_api_key,omitempty"`     // Google Gemini API key
	OllamaBaseURL    string `json:"ollama_base_url,omitempty"`    // Ollama server URL (default: http://localhost:11434)
	OllamaUsername   string `json:"ollama_username,omitempty"`    // Optional Basic Auth username for Ollama
	OllamaPassword   string `json:"ollama_password,omitempty"`    // Optional Basic Auth password for Ollama
	OpenAIBaseURL    string `json:"openai_base_url,omitempty"`    // Custom OpenAI-compatible base URL (optional)

	// OAuth fields for Claude Pro/Max subscription authentication
	AuthMethod        AuthMethod `json:"auth_method,omitempty"`         // "api_key" or "oauth" (for anthropic only)
	OAuthAccessToken  string     `json:"oauth_access_token,omitempty"`  // OAuth access token (encrypted at rest)
	OAuthRefreshToken string     `json:"oauth_refresh_token,omitempty"` // OAuth refresh token (encrypted at rest)
	OAuthExpiresAt    time.Time  `json:"oauth_expires_at,omitempty"`    // Token expiration time

	// Patrol settings for background AI monitoring
	PatrolEnabled          bool   `json:"patrol_enabled"`                     // Enable background AI health patrol
	PatrolIntervalMinutes  int    `json:"patrol_interval_minutes"`            // How often to run quick patrols (default: 360 = 6 hours)
	PatrolAnalyzeNodes     bool   `json:"patrol_analyze_nodes"`               // Include Proxmox nodes in patrol
	PatrolAnalyzeGuests    bool   `json:"patrol_analyze_guests"`              // Include VMs/containers in patrol
	PatrolAnalyzeDocker    bool   `json:"patrol_analyze_docker"`              // Include Docker hosts in patrol
	PatrolAnalyzeStorage   bool   `json:"patrol_analyze_storage"`             // Include storage in patrol
	PatrolAutoFix          bool   `json:"patrol_auto_fix,omitempty"`          // When true, patrol can attempt automatic remediation (default: false, observe only)
	UseProactiveThresholds bool   `json:"use_proactive_thresholds,omitempty"` // When true, patrol warns 5-15% BEFORE alert thresholds (default: false, use exact thresholds)
	AutoFixModel           string `json:"auto_fix_model,omitempty"`           // Model for automatic remediation (defaults to PatrolModel, may want more capable model)

	// Alert-triggered AI analysis - analyze specific resources when alerts fire
	AlertTriggeredAnalysis bool `json:"alert_triggered_analysis"` // Enable AI analysis when alerts fire (token-efficient)

	// Event-triggered patrols - run extra patrols when alerts fire or anomalies are detected.
	// The legacy aggregate flag is retained for compatibility; the canonical model is now split
	// between alert-triggered and anomaly-triggered scoped patrol preferences.
	PatrolEventTriggersEnabled   bool `json:"patrol_event_triggers_enabled"`
	PatrolAlertTriggersEnabled   bool `json:"patrol_alert_triggers_enabled"`
	PatrolAnomalyTriggersEnabled bool `json:"patrol_anomaly_triggers_enabled"`

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
	PatrolAutonomyLevel           string `json:"patrol_autonomy_level,omitempty"`            // "monitor", "approval", "assisted", "full"
	PatrolFullModeUnlocked        bool   `json:"patrol_full_mode_unlocked"`                  // User has acknowledged Full mode risks (required to use "full")
	PatrolInvestigationBudget     int    `json:"patrol_investigation_budget,omitempty"`      // Max turns per investigation (default: 15)
	PatrolInvestigationTimeoutSec int    `json:"patrol_investigation_timeout_sec,omitempty"` // Max seconds per investigation (default: 300)

	// Discovery settings - controls automatic infrastructure discovery
	DiscoveryEnabled       bool `json:"discovery_enabled"`                  // Enable infrastructure discovery
	DiscoveryIntervalHours int  `json:"discovery_interval_hours,omitempty"` // Hours between automatic re-scans (0 = manual only, default: 0)
}

// AIProvider constants
const (
	AIProviderAnthropic  = "anthropic"
	AIProviderOpenAI     = "openai"
	AIProviderOpenRouter = "openrouter"
	AIProviderOllama     = "ollama"
	AIProviderDeepSeek   = "deepseek"
	AIProviderGemini     = "gemini"
	AIProviderQuickstart = "quickstart" // Pulse-hosted proxy for free quickstart credits
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
	// PatrolAutonomyApproval - Spawn Chat sessions to investigate, queue ALL fixes for user approval
	PatrolAutonomyApproval = "approval"
	// PatrolAutonomyAssisted - Auto-fix warnings, critical findings still need approval
	PatrolAutonomyAssisted = "assisted"
	// PatrolAutonomyFull - Full autonomy, auto-fix everything including critical (user accepts risk)
	PatrolAutonomyFull = "full"
)

// Default patrol investigation settings
const (
	DefaultPatrolInvestigationBudget     = 15  // Max turns (tool calls) per investigation
	DefaultPatrolInvestigationTimeoutSec = 600 // 10 minutes
	MaxConcurrentInvestigations          = 3   // Max parallel investigations
	MaxInvestigationAttempts             = 3   // Max retry attempts per finding
	InvestigationCooldownHours           = 1   // Hours before re-investigating same finding
)

const (
	// DefaultAIModelQuickstart is a Pulse-owned stable alias. The hosted
	// quickstart backend chooses the real upstream vendor model server-side.
	DefaultAIModelQuickstart = "pulse-hosted"
	DefaultOllamaBaseURL     = "http://localhost:11434"
	DefaultOpenRouterBaseURL = "https://openrouter.ai/api/v1/chat/completions"
	DefaultDeepSeekBaseURL   = "https://api.deepseek.com/chat/completions"
	DefaultGeminiBaseURL     = "https://generativelanguage.googleapis.com/v1beta"
)

// NewDefaultAIConfig returns an AIConfig with sensible defaults
func NewDefaultAIConfig() *AIConfig {
	return &AIConfig{
		Enabled:    false,
		Model:      "",
		AuthMethod: AuthMethodAPIKey,
		// Patrol defaults - enabled when AI is enabled
		// Default to 6 hour intervals (much more token-efficient than 15 min)
		PatrolEnabled:         true,
		PatrolIntervalMinutes: 360, // 6 hours - balance between coverage and token efficiency
		PatrolAnalyzeNodes:    true,
		PatrolAnalyzeGuests:   true,
		PatrolAnalyzeDocker:   true,
		PatrolAnalyzeStorage:  true,
		// Alert-triggered analysis is highly token-efficient - enabled by default
		AlertTriggeredAnalysis: true,
		// Event-triggered patrols enabled by default (alerts, anomalies trigger extra patrols)
		PatrolEventTriggersEnabled:   true,
		PatrolAlertTriggersEnabled:   true,
		PatrolAnomalyTriggersEnabled: true,
	}
}

// IsConfigured returns true if the AI config has enough info to make API calls
// For multi-provider setup, returns true if ANY provider is configured
func (c *AIConfig) IsConfigured() bool {
	if !c.Enabled {
		return false
	}

	if c.HasProvider(AIProviderAnthropic) || c.HasProvider(AIProviderOpenAI) ||
		c.HasProvider(AIProviderOpenRouter) || c.HasProvider(AIProviderDeepSeek) || c.HasProvider(AIProviderOllama) ||
		c.HasProvider(AIProviderGemini) {
		return true
	}

	return false
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
	case AIProviderOpenRouter:
		return c.OpenRouterAPIKey != ""
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
	if c.HasProvider(AIProviderOpenRouter) {
		providers = append(providers, AIProviderOpenRouter)
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
	case AIProviderOpenAI:
		if c.OpenAIAPIKey != "" {
			return c.OpenAIAPIKey
		}
	case AIProviderOpenRouter:
		if c.OpenRouterAPIKey != "" {
			return c.OpenRouterAPIKey
		}
	case AIProviderDeepSeek:
		if c.DeepSeekAPIKey != "" {
			return c.DeepSeekAPIKey
		}
	case AIProviderGemini:
		if c.GeminiAPIKey != "" {
			return c.GeminiAPIKey
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
		return DefaultOllamaBaseURL
	case AIProviderOpenAI:
		if c.OpenAIBaseURL != "" {
			return c.OpenAIBaseURL
		}
		return "" // Uses default OpenAI URL
	case AIProviderOpenRouter:
		return DefaultOpenRouterBaseURL
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
	for _, p := range []string{AIProviderAnthropic, AIProviderOpenAI, AIProviderOpenRouter, AIProviderDeepSeek, AIProviderGemini, AIProviderOllama, AIProviderQuickstart} {
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
	case strings.HasPrefix(model, "openai/"), strings.HasPrefix(model, "anthropic/"), strings.HasPrefix(model, "google/"),
		strings.HasPrefix(model, "deepseek/"), strings.HasPrefix(model, "meta-llama/"), strings.HasPrefix(model, "mistralai/"),
		strings.HasPrefix(model, "x-ai/"), strings.HasPrefix(model, "xai/"), strings.HasPrefix(model, "cohere/"),
		strings.HasPrefix(model, "qwen/"):
		return AIProviderOpenRouter, model
	default:
		// Assume Ollama for unrecognized models (local models have varied names)
		return AIProviderOllama, model
	}
}

// FormatModelString creates a "provider:model-name" format string
func FormatModelString(provider, modelName string) string {
	return provider + ":" + modelName
}

// NormalizeQuickstartModelString canonicalizes legacy quickstart model strings to
// Pulse's owned hosted alias. The server chooses the real upstream vendor model.
func NormalizeQuickstartModelString(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	if strings.EqualFold(model, DefaultAIModelQuickstart) || strings.EqualFold(model, AIProviderQuickstart) {
		return DefaultModelForProvider(AIProviderQuickstart)
	}
	provider, _ := ParseModelString(model)
	if provider == AIProviderQuickstart {
		return DefaultModelForProvider(AIProviderQuickstart)
	}
	return model
}

// DefaultModelForProvider returns the default "provider:model" string for a given provider name.
// Returns empty string if the provider is unknown.
func DefaultModelForProvider(provider string) string {
	switch provider {
	case AIProviderQuickstart:
		return FormatModelString(AIProviderQuickstart, DefaultAIModelQuickstart)
	default:
		return ""
	}
}

// GetModel returns the explicitly configured model, if any.
func (c *AIConfig) GetModel() string {
	if c == nil {
		return ""
	}
	return NormalizeQuickstartModelString(c.Model)
}

// NormalizeQuickstartModelAliases rewrites any legacy quickstart model strings in-place
// to the owned Pulse alias. Returns true when a field changed.
func (c *AIConfig) NormalizeQuickstartModelAliases() bool {
	if c == nil {
		return false
	}

	changed := false
	normalizeField := func(field *string) {
		normalized := NormalizeQuickstartModelString(*field)
		if normalized == strings.TrimSpace(*field) {
			return
		}
		*field = normalized
		changed = true
	}

	normalizeField(&c.Model)
	normalizeField(&c.ChatModel)
	normalizeField(&c.PatrolModel)
	normalizeField(&c.DiscoveryModel)
	normalizeField(&c.AutoFixModel)

	return changed
}

// GetPreferredModelForProvider returns the most relevant configured model for a provider.
// It prefers explicitly selected models for that provider before falling back to the
// provider-owned quickstart alias when applicable.
func (c *AIConfig) GetPreferredModelForProvider(provider string) string {
	for _, candidate := range []string{c.Model, c.ChatModel, c.PatrolModel, c.AutoFixModel, c.DiscoveryModel} {
		candidate = NormalizeQuickstartModelString(candidate)
		if candidate == "" {
			continue
		}
		candidateProvider, _ := ParseModelString(candidate)
		if candidateProvider == provider {
			return candidate
		}
	}

	if provider == AIProviderQuickstart {
		return DefaultModelForProvider(provider)
	}
	return ""
}

// GetChatModel returns the model for interactive chat conversations
// Falls back to the main Model if ChatModel is not set
func (c *AIConfig) GetChatModel() string {
	if c.ChatModel != "" {
		return NormalizeQuickstartModelString(c.ChatModel)
	}
	return c.GetModel()
}

// GetPatrolModel returns the model for background patrol analysis
// Falls back to the main Model if PatrolModel is not set
func (c *AIConfig) GetPatrolModel() string {
	if c.PatrolModel != "" {
		return NormalizeQuickstartModelString(c.PatrolModel)
	}
	return c.GetModel()
}

// GetDiscoveryModel returns the model for infrastructure discovery
// Falls back to the main model since discovery needs to use the same provider
func (c *AIConfig) GetDiscoveryModel() string {
	if c.DiscoveryModel != "" {
		return NormalizeQuickstartModelString(c.DiscoveryModel)
	}
	// Fall back to the main model to ensure we use the same provider
	return c.GetModel()
}

// GetAutoFixModel returns the model for automatic remediation actions
// Falls back to PatrolModel, then to the main Model if AutoFixModel is not set
// Auto-fix may warrant a more capable model since it takes actions
func (c *AIConfig) GetAutoFixModel() string {
	if c.AutoFixModel != "" {
		return NormalizeQuickstartModelString(c.AutoFixModel)
	}
	return c.GetPatrolModel()
}

// ClearOAuthTokens clears OAuth tokens (used when switching back to API key auth)
func (c *AIConfig) ClearOAuthTokens() {
	c.OAuthAccessToken = ""
	c.OAuthRefreshToken = ""
	c.OAuthExpiresAt = time.Time{}
}

// ClearAPIKey clears the Anthropic API key (used when switching to OAuth auth)
func (c *AIConfig) ClearAPIKey() {
	c.AnthropicAPIKey = ""
}

// GetPatrolInterval returns the patrol interval as a duration.
func (c *AIConfig) GetPatrolInterval() time.Duration {
	// Use configured custom minutes when set.
	if c.PatrolIntervalMinutes > 0 {
		return time.Duration(c.PatrolIntervalMinutes) * time.Minute
	}

	return 6 * time.Hour // default to 6 hours
}

// IsPatrolEnabled returns true if patrol should run
// Note: Patrol uses local heuristics and doesn't require an AI API key,
// but still requires AI to be enabled as a master switch
func (c *AIConfig) IsPatrolEnabled() bool {
	// If AI is disabled globally, patrol is disabled
	if !c.Enabled {
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

func (c *AIConfig) patrolEventTriggerPreferences() (bool, bool) {
	if c == nil {
		return false, false
	}

	alertEnabled := c.PatrolAlertTriggersEnabled
	anomalyEnabled := c.PatrolAnomalyTriggersEnabled

	// Compatibility: older callers and persisted configs may still use only the
	// legacy aggregate flag. When neither granular preference is enabled but the
	// legacy flag is on, treat both trigger sources as enabled.
	if !alertEnabled && !anomalyEnabled && c.PatrolEventTriggersEnabled {
		return true, true
	}

	return alertEnabled, anomalyEnabled
}

// GetPatrolEventTriggerSettings returns the persisted scoped patrol trigger preferences
// without applying the AI master-switch gating used by runtime checks.
func (c *AIConfig) GetPatrolEventTriggerSettings() PatrolEventTriggerSettings {
	alertEnabled, anomalyEnabled := c.patrolEventTriggerPreferences()
	return PatrolEventTriggerSettings{
		AlertTriggersEnabled:   alertEnabled,
		AnomalyTriggersEnabled: anomalyEnabled,
	}
}

// NormalizePatrolEventTriggerSettings synchronizes the legacy aggregate field with the
// canonical split trigger preferences.
func (c *AIConfig) NormalizePatrolEventTriggerSettings() bool {
	if c == nil {
		return false
	}

	settings := c.GetPatrolEventTriggerSettings()
	changed := false
	if c.PatrolAlertTriggersEnabled != settings.AlertTriggersEnabled {
		c.PatrolAlertTriggersEnabled = settings.AlertTriggersEnabled
		changed = true
	}
	if c.PatrolAnomalyTriggersEnabled != settings.AnomalyTriggersEnabled {
		c.PatrolAnomalyTriggersEnabled = settings.AnomalyTriggersEnabled
		changed = true
	}
	aggregateEnabled := settings.AlertTriggersEnabled || settings.AnomalyTriggersEnabled
	if c.PatrolEventTriggersEnabled != aggregateEnabled {
		c.PatrolEventTriggersEnabled = aggregateEnabled
		changed = true
	}
	return changed
}

// SetPatrolEventTriggersEnabled updates both scoped patrol trigger sources together.
func (c *AIConfig) SetPatrolEventTriggersEnabled(enabled bool) {
	if c == nil {
		return
	}
	c.PatrolAlertTriggersEnabled = enabled
	c.PatrolAnomalyTriggersEnabled = enabled
	c.PatrolEventTriggersEnabled = enabled
}

// SetPatrolEventTriggerSettings updates the canonical scoped patrol trigger preferences.
func (c *AIConfig) SetPatrolEventTriggerSettings(alertEnabled, anomalyEnabled bool) {
	if c == nil {
		return
	}
	c.PatrolAlertTriggersEnabled = alertEnabled
	c.PatrolAnomalyTriggersEnabled = anomalyEnabled
	c.PatrolEventTriggersEnabled = alertEnabled || anomalyEnabled
}

// IsPatrolEventTriggersEnabled returns true if event-driven patrol triggers (alerts, anomalies) are enabled
func (c *AIConfig) IsPatrolEventTriggersEnabled() bool {
	if !c.Enabled {
		return false
	}
	settings := c.GetPatrolEventTriggerSettings()
	return settings.AlertTriggersEnabled || settings.AnomalyTriggersEnabled
}

// IsPatrolAlertTriggersEnabled returns true if alert-triggered scoped patrols are enabled.
func (c *AIConfig) IsPatrolAlertTriggersEnabled() bool {
	if !c.Enabled {
		return false
	}
	return c.GetPatrolEventTriggerSettings().AlertTriggersEnabled
}

// IsPatrolAnomalyTriggersEnabled returns true if anomaly-triggered scoped patrols are enabled.
func (c *AIConfig) IsPatrolAnomalyTriggersEnabled() bool {
	if !c.Enabled {
		return false
	}
	return c.GetPatrolEventTriggerSettings().AnomalyTriggersEnabled
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
func (c *AIConfig) GetControlLevel() string {
	if c.ControlLevel == "" {
		return ControlLevelReadOnly
	}
	switch c.ControlLevel {
	case ControlLevelReadOnly, ControlLevelControlled, ControlLevelAutonomous:
		return c.ControlLevel
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
	case PatrolAutonomyMonitor, PatrolAutonomyApproval, PatrolAutonomyAssisted, PatrolAutonomyFull:
		return c.PatrolAutonomyLevel
	// Migration: treat old "autonomous" as new "full"
	case "autonomous":
		return PatrolAutonomyFull
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

// IsValidPatrolAutonomyLevel checks if a patrol autonomy level string is valid
func IsValidPatrolAutonomyLevel(level string) bool {
	switch level {
	case PatrolAutonomyMonitor, PatrolAutonomyApproval, PatrolAutonomyAssisted, PatrolAutonomyFull:
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
