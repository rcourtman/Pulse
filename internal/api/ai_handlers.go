package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// AISettingsHandler handles AI settings endpoints
type AISettingsHandler struct {
	config      *config.Config
	persistence *config.ConfigPersistence
	aiService   *ai.Service
	agentServer *agentexec.Server
}

// NewAISettingsHandler creates a new AI settings handler
func NewAISettingsHandler(cfg *config.Config, persistence *config.ConfigPersistence, agentServer *agentexec.Server) *AISettingsHandler {
	aiService := ai.NewService(persistence, agentServer)
	if err := aiService.LoadConfig(); err != nil {
		log.Warn().Err(err).Msg("Failed to load AI config on startup")
	}

	return &AISettingsHandler{
		config:      cfg,
		persistence: persistence,
		aiService:   aiService,
		agentServer: agentServer,
	}
}

// SetConfig updates the configuration reference used by the handler.
func (h *AISettingsHandler) SetConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	h.config = cfg
}

// SetStateProvider sets the state provider for infrastructure context
func (h *AISettingsHandler) SetStateProvider(sp ai.StateProvider) {
	h.aiService.SetStateProvider(sp)
}

// SetResourceProvider sets the resource provider for unified infrastructure context (Phase 2)
func (h *AISettingsHandler) SetResourceProvider(rp ai.ResourceProvider) {
	h.aiService.SetResourceProvider(rp)
}

// SetMetadataProvider sets the metadata provider for AI URL discovery
func (h *AISettingsHandler) SetMetadataProvider(mp ai.MetadataProvider) {
	h.aiService.SetMetadataProvider(mp)
}

// StartPatrol starts the background AI patrol service
func (h *AISettingsHandler) StartPatrol(ctx context.Context) {
	h.aiService.StartPatrol(ctx)
}

// SetPatrolThresholdProvider sets the threshold provider for the patrol service
func (h *AISettingsHandler) SetPatrolThresholdProvider(provider ai.ThresholdProvider) {
	h.aiService.SetPatrolThresholdProvider(provider)
}

// SetPatrolFindingsPersistence enables findings persistence for the patrol service
func (h *AISettingsHandler) SetPatrolFindingsPersistence(persistence ai.FindingsPersistence) error {
	if patrol := h.aiService.GetPatrolService(); patrol != nil {
		return patrol.SetFindingsPersistence(persistence)
	}
	return nil
}

// SetPatrolRunHistoryPersistence enables patrol run history persistence for the patrol service
func (h *AISettingsHandler) SetPatrolRunHistoryPersistence(persistence ai.PatrolHistoryPersistence) error {
	if patrol := h.aiService.GetPatrolService(); patrol != nil {
		return patrol.SetRunHistoryPersistence(persistence)
	}
	return nil
}

// SetMetricsHistoryProvider sets the metrics history provider for enriched AI context
func (h *AISettingsHandler) SetMetricsHistoryProvider(provider ai.MetricsHistoryProvider) {
	h.aiService.SetMetricsHistoryProvider(provider)
}

// SetBaselineStore sets the baseline store for anomaly detection
func (h *AISettingsHandler) SetBaselineStore(store *ai.BaselineStore) {
	h.aiService.SetBaselineStore(store)
}

// StopPatrol stops the background AI patrol service
func (h *AISettingsHandler) StopPatrol() {
	h.aiService.StopPatrol()
}

// GetAlertTriggeredAnalyzer returns the alert-triggered analyzer for wiring into alert callbacks
func (h *AISettingsHandler) GetAlertTriggeredAnalyzer() *ai.AlertTriggeredAnalyzer {
	return h.aiService.GetAlertTriggeredAnalyzer()
}

// AISettingsResponse is returned by GET /api/settings/ai
// API keys are masked for security
type AISettingsResponse struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`    // DEPRECATED: legacy single provider
	APIKeySet      bool   `json:"api_key_set"` // DEPRECATED: true if legacy API key is configured
	Model          string `json:"model"`
	ChatModel      string `json:"chat_model,omitempty"`   // Model for interactive chat (empty = use default)
	PatrolModel    string `json:"patrol_model,omitempty"` // Model for patrol (empty = use default)
	BaseURL        string `json:"base_url,omitempty"`     // DEPRECATED: legacy base URL
	Configured     bool   `json:"configured"`             // true if AI is ready to use
	AutonomousMode bool   `json:"autonomous_mode"`        // true if AI can execute without approval
	CustomContext  string `json:"custom_context"`         // user-provided infrastructure context
	// OAuth fields for Claude Pro/Max subscription authentication
	AuthMethod     string `json:"auth_method"`     // "api_key" or "oauth"
	OAuthConnected bool   `json:"oauth_connected"` // true if OAuth tokens are configured
	// Patrol settings for token efficiency
	PatrolSchedulePreset   string             `json:"patrol_schedule_preset"`   // DEPRECATED: legacy preset
	PatrolIntervalMinutes  int                `json:"patrol_interval_minutes"`  // Patrol interval in minutes (0 = disabled)
	AlertTriggeredAnalysis bool               `json:"alert_triggered_analysis"` // true if AI analyzes when alerts fire
	AvailableModels        []config.ModelInfo `json:"available_models"`         // List of models for current provider
	// Multi-provider credentials - shows which providers are configured
	AnthropicConfigured bool     `json:"anthropic_configured"`      // true if Anthropic API key or OAuth is set
	OpenAIConfigured    bool     `json:"openai_configured"`         // true if OpenAI API key is set
	DeepSeekConfigured  bool     `json:"deepseek_configured"`       // true if DeepSeek API key is set
	OllamaConfigured    bool     `json:"ollama_configured"`         // true (always available for attempt)
	OllamaBaseURL       string   `json:"ollama_base_url"`           // Ollama server URL
	OpenAIBaseURL       string   `json:"openai_base_url,omitempty"` // Custom OpenAI base URL
	ConfiguredProviders []string `json:"configured_providers"`      // List of provider names with credentials
	// Cost controls
	CostBudgetUSD30d float64 `json:"cost_budget_usd_30d,omitempty"`
}

// AISettingsUpdateRequest is the request body for PUT /api/settings/ai
type AISettingsUpdateRequest struct {
	Enabled        *bool   `json:"enabled,omitempty"`
	Provider       *string `json:"provider,omitempty"` // DEPRECATED: use model selection instead
	APIKey         *string `json:"api_key,omitempty"`  // DEPRECATED: use per-provider keys
	Model          *string `json:"model,omitempty"`
	ChatModel      *string `json:"chat_model,omitempty"`   // Model for interactive chat
	PatrolModel    *string `json:"patrol_model,omitempty"` // Model for background patrol
	BaseURL        *string `json:"base_url,omitempty"`     // DEPRECATED: use per-provider URLs
	AutonomousMode *bool   `json:"autonomous_mode,omitempty"`
	CustomContext  *string `json:"custom_context,omitempty"` // user-provided infrastructure context
	AuthMethod     *string `json:"auth_method,omitempty"`    // "api_key" or "oauth"
	// Patrol settings for token efficiency
	PatrolSchedulePreset   *string `json:"patrol_schedule_preset,omitempty"`   // DEPRECATED: use patrol_interval_minutes
	PatrolIntervalMinutes  *int    `json:"patrol_interval_minutes,omitempty"`  // Custom interval in minutes (0 = disabled, minimum 10)
	AlertTriggeredAnalysis *bool   `json:"alert_triggered_analysis,omitempty"` // true if AI analyzes when alerts fire
	// Multi-provider credentials
	AnthropicAPIKey *string `json:"anthropic_api_key,omitempty"` // Set Anthropic API key
	OpenAIAPIKey    *string `json:"openai_api_key,omitempty"`    // Set OpenAI API key
	DeepSeekAPIKey  *string `json:"deepseek_api_key,omitempty"`  // Set DeepSeek API key
	OllamaBaseURL   *string `json:"ollama_base_url,omitempty"`   // Set Ollama server URL
	OpenAIBaseURL   *string `json:"openai_base_url,omitempty"`   // Set custom OpenAI base URL
	// Clear flags for removing credentials
	ClearAnthropicKey *bool `json:"clear_anthropic_key,omitempty"` // Clear Anthropic API key
	ClearOpenAIKey    *bool `json:"clear_openai_key,omitempty"`    // Clear OpenAI API key
	ClearDeepSeekKey  *bool `json:"clear_deepseek_key,omitempty"`  // Clear DeepSeek API key
	ClearOllamaURL    *bool `json:"clear_ollama_url,omitempty"`    // Clear Ollama URL
	// Cost controls
	CostBudgetUSD30d *float64 `json:"cost_budget_usd_30d,omitempty"`
}

// HandleGetAISettings returns the current AI settings (GET /api/settings/ai)
func (h *AISettingsHandler) HandleGetAISettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settings, err := h.persistence.LoadAIConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load AI settings")
		http.Error(w, "Failed to load AI settings", http.StatusInternalServerError)
		return
	}

	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}

	// Determine auth method string
	authMethod := string(settings.AuthMethod)
	if authMethod == "" {
		authMethod = string(config.AuthMethodAPIKey)
	}

	response := AISettingsResponse{
		Enabled:        settings.Enabled,
		Provider:       settings.Provider,
		APIKeySet:      settings.APIKey != "",
		Model:          settings.GetModel(),
		ChatModel:      settings.ChatModel,
		PatrolModel:    settings.PatrolModel,
		BaseURL:        settings.BaseURL,
		Configured:     settings.IsConfigured(),
		AutonomousMode: settings.AutonomousMode,
		CustomContext:  settings.CustomContext,
		AuthMethod:     authMethod,
		OAuthConnected: settings.OAuthAccessToken != "",
		// Patrol settings
		PatrolSchedulePreset:   settings.PatrolSchedulePreset,
		PatrolIntervalMinutes:  settings.PatrolIntervalMinutes,
		AlertTriggeredAnalysis: settings.AlertTriggeredAnalysis,
		AvailableModels:        nil, // Now populated via /api/ai/models endpoint
		// Multi-provider configuration
		AnthropicConfigured: settings.HasProvider(config.AIProviderAnthropic),
		OpenAIConfigured:    settings.HasProvider(config.AIProviderOpenAI),
		DeepSeekConfigured:  settings.HasProvider(config.AIProviderDeepSeek),
		OllamaConfigured:    settings.HasProvider(config.AIProviderOllama),
		OllamaBaseURL:       settings.GetBaseURLForProvider(config.AIProviderOllama),
		OpenAIBaseURL:       settings.OpenAIBaseURL,
		ConfiguredProviders: settings.GetConfiguredProviders(),
		CostBudgetUSD30d:    settings.CostBudgetUSD30d,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write AI settings response")
	}
}

// HandleUpdateAISettings updates AI settings (PUT /api/settings/ai)
func (h *AISettingsHandler) HandleUpdateAISettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require admin authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	// Check proxy auth admin status if applicable
	if h.config.ProxyAuthSecret != "" {
		if valid, username, isAdmin := CheckProxyAuth(h.config, r); valid && !isAdmin {
			log.Warn().
				Str("ip", r.RemoteAddr).
				Str("path", r.URL.Path).
				Str("method", r.Method).
				Str("username", username).
				Msg("Non-admin user attempted to update AI settings")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Admin privileges required"})
			return
		}
	}

	// Load existing settings
	settings, err := h.persistence.LoadAIConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load existing AI settings")
		settings = config.NewDefaultAIConfig()
	}
	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var req AISettingsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate and apply updates
	if req.Provider != nil {
		provider := strings.ToLower(strings.TrimSpace(*req.Provider))
		switch provider {
		case config.AIProviderAnthropic, config.AIProviderOpenAI, config.AIProviderOllama, config.AIProviderDeepSeek:
			settings.Provider = provider
		default:
			http.Error(w, "Invalid provider. Must be 'anthropic', 'openai', 'ollama', or 'deepseek'", http.StatusBadRequest)
			return
		}
	}

	if req.APIKey != nil {
		// Empty string clears the API key
		settings.APIKey = strings.TrimSpace(*req.APIKey)
	}

	if req.Model != nil {
		settings.Model = strings.TrimSpace(*req.Model)
	}

	if req.ChatModel != nil {
		settings.ChatModel = strings.TrimSpace(*req.ChatModel)
	}

	if req.PatrolModel != nil {
		settings.PatrolModel = strings.TrimSpace(*req.PatrolModel)
	}

	if req.BaseURL != nil {
		settings.BaseURL = strings.TrimSpace(*req.BaseURL)
	}

	if req.AutonomousMode != nil {
		settings.AutonomousMode = *req.AutonomousMode
	}

	if req.CustomContext != nil {
		settings.CustomContext = strings.TrimSpace(*req.CustomContext)
	}

	if req.Enabled != nil {
		// Only allow enabling if properly configured
		if *req.Enabled {
			switch settings.Provider {
			case config.AIProviderAnthropic, config.AIProviderOpenAI, config.AIProviderDeepSeek:
				if settings.APIKey == "" {
					http.Error(w, "Cannot enable AI: API key is required for "+settings.Provider, http.StatusBadRequest)
					return
				}
			case config.AIProviderOllama:
				// Ollama doesn't need API key, but needs base URL (or will use default)
				if settings.BaseURL == "" {
					settings.BaseURL = config.DefaultOllamaBaseURL
				}
			}
		}
		settings.Enabled = *req.Enabled
	}

	// Handle patrol interval - prefer custom minutes over preset
	if req.PatrolIntervalMinutes != nil {
		minutes := *req.PatrolIntervalMinutes
		if minutes < 0 {
			http.Error(w, "patrol_interval_minutes cannot be negative", http.StatusBadRequest)
			return
		}
		if minutes > 0 && minutes < 10 {
			http.Error(w, "patrol_interval_minutes must be at least 10 minutes (or 0 to disable)", http.StatusBadRequest)
			return
		}
		if minutes > 10080 { // 7 days max
			http.Error(w, "patrol_interval_minutes cannot exceed 10080 (7 days)", http.StatusBadRequest)
			return
		}
		settings.PatrolIntervalMinutes = minutes
		settings.PatrolSchedulePreset = "" // Clear preset when using custom minutes
	} else if req.PatrolSchedulePreset != nil {
		// Legacy preset support
		preset := strings.ToLower(strings.TrimSpace(*req.PatrolSchedulePreset))
		switch preset {
		case "15min", "1hr", "6hr", "12hr", "daily", "disabled":
			settings.PatrolSchedulePreset = preset
			settings.PatrolIntervalMinutes = config.PresetToMinutes(preset)
		default:
			http.Error(w, "Invalid patrol_schedule_preset. Must be '15min', '1hr', '6hr', '12hr', 'daily', or 'disabled'", http.StatusBadRequest)
			return
		}
	}

	if req.CostBudgetUSD30d != nil {
		if *req.CostBudgetUSD30d < 0 {
			http.Error(w, "cost_budget_usd_30d cannot be negative", http.StatusBadRequest)
			return
		}
		settings.CostBudgetUSD30d = *req.CostBudgetUSD30d
	}

	// Handle alert-triggered analysis toggle
	if req.AlertTriggeredAnalysis != nil {
		settings.AlertTriggeredAnalysis = *req.AlertTriggeredAnalysis
	}

	// Handle multi-provider credentials
	// Clear flags take priority over setting new values
	if req.ClearAnthropicKey != nil && *req.ClearAnthropicKey {
		settings.AnthropicAPIKey = ""
	} else if req.AnthropicAPIKey != nil {
		settings.AnthropicAPIKey = strings.TrimSpace(*req.AnthropicAPIKey)
	}
	if req.ClearOpenAIKey != nil && *req.ClearOpenAIKey {
		settings.OpenAIAPIKey = ""
	} else if req.OpenAIAPIKey != nil {
		settings.OpenAIAPIKey = strings.TrimSpace(*req.OpenAIAPIKey)
	}
	if req.ClearDeepSeekKey != nil && *req.ClearDeepSeekKey {
		settings.DeepSeekAPIKey = ""
	} else if req.DeepSeekAPIKey != nil {
		settings.DeepSeekAPIKey = strings.TrimSpace(*req.DeepSeekAPIKey)
	}
	if req.ClearOllamaURL != nil && *req.ClearOllamaURL {
		settings.OllamaBaseURL = ""
	} else if req.OllamaBaseURL != nil {
		settings.OllamaBaseURL = strings.TrimSpace(*req.OllamaBaseURL)
	}
	if req.OpenAIBaseURL != nil {
		settings.OpenAIBaseURL = strings.TrimSpace(*req.OpenAIBaseURL)
	}

	// Save settings
	if err := h.persistence.SaveAIConfig(*settings); err != nil {
		log.Error().Err(err).Msg("Failed to save AI settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	// Reload the AI service with new settings
	if err := h.aiService.Reload(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload AI service after settings update")
	}

	// Reconfigure patrol service with new settings (applies interval changes immediately)
	h.aiService.ReconfigurePatrol()

	// Update alert-triggered analyzer if available
	if analyzer := h.aiService.GetAlertTriggeredAnalyzer(); analyzer != nil {
		analyzer.SetEnabled(settings.AlertTriggeredAnalysis)
	}

	log.Info().
		Bool("enabled", settings.Enabled).
		Str("provider", settings.Provider).
		Str("model", settings.GetModel()).
		Str("patrolPreset", settings.PatrolSchedulePreset).
		Bool("alertTriggeredAnalysis", settings.AlertTriggeredAnalysis).
		Msg("AI settings updated")

	// Determine auth method for response
	authMethod := string(settings.AuthMethod)
	if authMethod == "" {
		authMethod = string(config.AuthMethodAPIKey)
	}

	// Return updated settings
	response := AISettingsResponse{
		Enabled:                settings.Enabled,
		Provider:               settings.Provider,
		APIKeySet:              settings.APIKey != "",
		Model:                  settings.GetModel(),
		ChatModel:              settings.ChatModel,
		PatrolModel:            settings.PatrolModel,
		BaseURL:                settings.BaseURL,
		Configured:             settings.IsConfigured(),
		AutonomousMode:         settings.AutonomousMode,
		CustomContext:          settings.CustomContext,
		AuthMethod:             authMethod,
		OAuthConnected:         settings.OAuthAccessToken != "",
		PatrolSchedulePreset:   settings.PatrolSchedulePreset,
		PatrolIntervalMinutes:  settings.PatrolIntervalMinutes,
		AlertTriggeredAnalysis: settings.AlertTriggeredAnalysis,
		AvailableModels:        nil, // Now populated via /api/ai/models endpoint
		// Multi-provider configuration
		AnthropicConfigured: settings.HasProvider(config.AIProviderAnthropic),
		OpenAIConfigured:    settings.HasProvider(config.AIProviderOpenAI),
		DeepSeekConfigured:  settings.HasProvider(config.AIProviderDeepSeek),
		OllamaConfigured:    settings.HasProvider(config.AIProviderOllama),
		OllamaBaseURL:       settings.GetBaseURLForProvider(config.AIProviderOllama),
		OpenAIBaseURL:       settings.OpenAIBaseURL,
		ConfiguredProviders: settings.GetConfiguredProviders(),
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write AI settings update response")
	}
}

// HandleTestAIConnection tests the AI provider connection (POST /api/ai/test)
func (h *AISettingsHandler) HandleTestAIConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require admin authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var testResult struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Model   string `json:"model,omitempty"`
	}

	err := h.aiService.TestConnection(ctx)
	if err != nil {
		testResult.Success = false
		testResult.Message = err.Error()
	} else {
		cfg := h.aiService.GetConfig()
		testResult.Success = true
		testResult.Message = "Connection successful"
		if cfg != nil {
			testResult.Model = cfg.GetModel()
		}
	}

	if err := utils.WriteJSONResponse(w, testResult); err != nil {
		log.Error().Err(err).Msg("Failed to write AI test response")
	}
}

// HandleTestProvider tests a specific AI provider connection (POST /api/ai/test/:provider)
func (h *AISettingsHandler) HandleTestProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require admin authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	// Get provider from URL path (e.g., /api/ai/test/anthropic -> anthropic)
	provider := strings.TrimPrefix(r.URL.Path, "/api/ai/test/")
	if provider == "" || provider == r.URL.Path {
		http.Error(w, `{"error":"Provider is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var testResult struct {
		Success  bool   `json:"success"`
		Message  string `json:"message"`
		Provider string `json:"provider"`
	}
	testResult.Provider = provider

	// Load config and create provider for testing
	cfg := h.aiService.GetConfig()
	if cfg == nil {
		testResult.Success = false
		testResult.Message = "AI not configured"
		utils.WriteJSONResponse(w, testResult)
		return
	}

	// Check if provider is configured
	if !cfg.HasProvider(provider) {
		testResult.Success = false
		testResult.Message = "Provider not configured"
		utils.WriteJSONResponse(w, testResult)
		return
	}

	// Create provider and test connection
	testProvider, err := providers.NewForProvider(cfg, provider, cfg.GetModel())
	if err != nil {
		testResult.Success = false
		testResult.Message = fmt.Sprintf("Failed to create provider: %v", err)
		utils.WriteJSONResponse(w, testResult)
		return
	}

	err = testProvider.TestConnection(ctx)
	if err != nil {
		testResult.Success = false
		testResult.Message = err.Error()
	} else {
		testResult.Success = true
		testResult.Message = "Connection successful"
	}

	if err := utils.WriteJSONResponse(w, testResult); err != nil {
		log.Error().Err(err).Msg("Failed to write provider test response")
	}
}

// HandleListModels fetches available models from the configured AI provider (GET /api/ai/models)
func (h *AISettingsHandler) HandleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	type ModelInfo struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}

	type Response struct {
		Models []ModelInfo `json:"models"`
		Error  string      `json:"error,omitempty"`
		Cached bool        `json:"cached"`
	}

	models, err := h.aiService.ListModels(ctx)
	if err != nil {
		// Return error but don't fail the request - frontend can show a fallback
		resp := Response{
			Models: []ModelInfo{},
			Error:  err.Error(),
		}
		if jsonErr := utils.WriteJSONResponse(w, resp); jsonErr != nil {
			log.Error().Err(jsonErr).Msg("Failed to write AI models response")
		}
		return
	}

	// Convert provider models to response format
	responseModels := make([]ModelInfo, 0, len(models))
	for _, m := range models {
		responseModels = append(responseModels, ModelInfo{
			ID:          m.ID,
			Name:        m.Name,
			Description: m.Description,
		})
	}

	resp := Response{
		Models: responseModels,
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to write AI models response")
	}
}

// AIExecuteRequest is the request body for POST /api/ai/execute
// AIConversationMessage represents a message in conversation history
type AIConversationMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

type AIExecuteRequest struct {
	Prompt     string                  `json:"prompt"`
	TargetType string                  `json:"target_type,omitempty"` // "host", "container", "vm", "node"
	TargetID   string                  `json:"target_id,omitempty"`
	Context    map[string]interface{}  `json:"context,omitempty"` // Current metrics, state, etc.
	History    []AIConversationMessage `json:"history,omitempty"` // Previous conversation messages
}

// AIExecuteResponse is the response from POST /api/ai/execute
type AIExecuteResponse struct {
	Content      string             `json:"content"`
	Model        string             `json:"model"`
	InputTokens  int                `json:"input_tokens"`
	OutputTokens int                `json:"output_tokens"`
	ToolCalls    []ai.ToolExecution `json:"tool_calls,omitempty"` // Commands that were executed
}

// HandleExecute executes an AI prompt (POST /api/ai/execute)
func (h *AISettingsHandler) HandleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	// Check if AI is enabled
	if !h.aiService.IsEnabled() {
		http.Error(w, "AI is not enabled or configured", http.StatusBadRequest)
		return
	}

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64KB max
	var req AIExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		http.Error(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	// Execute the prompt with a timeout
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	resp, err := h.aiService.Execute(ctx, ai.ExecuteRequest{
		Prompt:     req.Prompt,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Context:    req.Context,
	})
	if err != nil {
		log.Error().Err(err).Msg("AI execution failed")
		http.Error(w, "AI request failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := AIExecuteResponse{
		Content:      resp.Content,
		Model:        resp.Model,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		ToolCalls:    resp.ToolCalls,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write AI execute response")
	}
}

// HandleExecuteStream executes an AI prompt with SSE streaming (POST /api/ai/execute/stream)
func (h *AISettingsHandler) HandleExecuteStream(w http.ResponseWriter, r *http.Request) {
	// Handle CORS for dev mode (frontend on different port)
	origin := r.Header.Get("Origin")
	if origin != "" {
		// Must use specific origin (not *) when credentials are included
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Cookie")
		w.Header().Set("Vary", "Origin")
	}

	// Handle preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	// Check if AI is enabled
	if !h.aiService.IsEnabled() {
		http.Error(w, "AI is not enabled or configured", http.StatusBadRequest)
		return
	}

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64KB max
	var req AIExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		http.Error(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	log.Info().
		Str("prompt", req.Prompt).
		Str("target_type", req.TargetType).
		Str("target_id", req.TargetID).
		Msg("AI streaming request started")

	// Set up SSE headers
	// IMPORTANT: Set headers BEFORE any writes to prevent Go from auto-adding Transfer-Encoding: chunked
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
	// Prevent chunked encoding which causes "Invalid character in chunk size" errors in Vite proxy
	w.Header().Set("Transfer-Encoding", "identity")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Disable the server's write deadline for this SSE connection
	// This is critical for long-running AI requests that can take several minutes
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Msg("Failed to disable write deadline for SSE")
	}
	// Also disable read deadline
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Msg("Failed to disable read deadline for SSE")
	}

	// Flush headers immediately
	flusher.Flush()

	// Create context with timeout (15 minutes for complex analysis with multiple tool calls)
	// Use background context to avoid browser disconnect canceling the request
	// DeepSeek reasoning models + multiple tool executions can easily take 5+ minutes
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Second)
	defer cancel()

	// Set up heartbeat to keep connection alive during long tool executions
	// NOTE: We don't check r.Context().Done() because Vite proxy may close
	// the request context prematurely. We detect real disconnection via write failures.
	heartbeatDone := make(chan struct{})
	var clientDisconnected bool
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Extend write deadline before heartbeat
				_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
				// Send SSE comment as heartbeat
				_, err := w.Write([]byte(": heartbeat\n\n"))
				if err != nil {
					log.Debug().Err(err).Msg("Heartbeat write failed, stopping heartbeat (AI continues)")
					clientDisconnected = true
					// Don't cancel the AI request - let it complete with its own timeout
					// The SSE connection may have issues but the AI work can still finish
					return
				}
				flusher.Flush()
				log.Debug().Msg("Sent SSE heartbeat")
			case <-heartbeatDone:
				return
			}
		}
	}()
	defer close(heartbeatDone)

	// Helper to safely write SSE events, tracking if client disconnected
	safeWrite := func(data []byte) bool {
		if clientDisconnected {
			return false
		}
		_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err := w.Write(data)
		if err != nil {
			log.Debug().Err(err).Msg("Failed to write SSE event (client may have disconnected)")
			clientDisconnected = true
			return false
		}
		flusher.Flush()
		return true
	}

	// Stream callback - write SSE events
	callback := func(event ai.StreamEvent) {
		// Skip the 'done' event from service - we'll send our own at the end
		// This ensures 'complete' comes before 'done'
		if event.Type == "done" {
			log.Debug().Msg("Skipping service 'done' event - will send final 'done' after 'complete'")
			return
		}

		data, err := json.Marshal(event)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal stream event")
			return
		}

		log.Debug().
			Str("event_type", event.Type).
			Msg("Streaming AI event")

		// SSE format: data: <json>\n\n
		safeWrite([]byte("data: " + string(data) + "\n\n"))
	}

	// Convert history from API type to service type
	var history []ai.ConversationMessage
	for _, msg := range req.History {
		history = append(history, ai.ConversationMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Ensure we always send a final 'done' event
	defer func() {
		if !clientDisconnected {
			doneEvent := ai.StreamEvent{Type: "done"}
			data, _ := json.Marshal(doneEvent)
			safeWrite([]byte("data: " + string(data) + "\n\n"))
			log.Debug().Msg("Sent final 'done' event")
		}
	}()

	// Execute with streaming
	resp, err := h.aiService.ExecuteStream(ctx, ai.ExecuteRequest{
		Prompt:     req.Prompt,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Context:    req.Context,
		History:    history,
	}, callback)

	if err != nil {
		log.Error().Err(err).Msg("AI streaming execution failed")
		// Send error event
		errEvent := ai.StreamEvent{Type: "error", Data: err.Error()}
		data, _ := json.Marshal(errEvent)
		safeWrite([]byte("data: " + string(data) + "\n\n"))
		return
	}

	log.Info().
		Str("model", resp.Model).
		Int("input_tokens", resp.InputTokens).
		Int("output_tokens", resp.OutputTokens).
		Int("tool_calls", len(resp.ToolCalls)).
		Msg("AI streaming request completed")

	// Send final response with metadata (before 'done')
	finalEvent := struct {
		Type         string             `json:"type"`
		Model        string             `json:"model"`
		InputTokens  int                `json:"input_tokens"`
		OutputTokens int                `json:"output_tokens"`
		ToolCalls    []ai.ToolExecution `json:"tool_calls,omitempty"`
	}{
		Type:         "complete",
		Model:        resp.Model,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		ToolCalls:    resp.ToolCalls,
	}
	data, _ := json.Marshal(finalEvent)
	safeWrite([]byte("data: " + string(data) + "\n\n"))
	// 'done' event is sent by the defer above
}

// AIRunCommandRequest is the request body for POST /api/ai/run-command
type AIRunCommandRequest struct {
	Command    string `json:"command"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	RunOnHost  bool   `json:"run_on_host"`
	VMID       string `json:"vmid,omitempty"`
	TargetHost string `json:"target_host,omitempty"` // Explicit host for routing
}

// HandleRunCommand executes a single approved command (POST /api/ai/run-command)
func (h *AISettingsHandler) HandleRunCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	bodyBytes, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		log.Error().Err(readErr).Msg("Failed to read request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Debug().Str("body", string(bodyBytes)).Msg("run-command request body")

	var req AIRunCommandRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		log.Error().Err(err).Str("body", string(bodyBytes)).Msg("Failed to decode JSON body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Command) == "" {
		http.Error(w, "Command is required", http.StatusBadRequest)
		return
	}

	log.Info().
		Str("command", req.Command).
		Str("target_type", req.TargetType).
		Str("target_id", req.TargetID).
		Bool("run_on_host", req.RunOnHost).
		Str("target_host", req.TargetHost).
		Msg("Executing approved command")

	// Execute with timeout (5 minutes for long-running commands)
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
	defer cancel()

	resp, err := h.aiService.RunCommand(ctx, ai.RunCommandRequest{
		Command:    req.Command,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		RunOnHost:  req.RunOnHost,
		VMID:       req.VMID,
		TargetHost: req.TargetHost,
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to execute command")
		http.Error(w, "Failed to execute command: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to write run command response")
	}
}

// HandleGetGuestKnowledge returns all notes for a guest
func (h *AISettingsHandler) HandleGetGuestKnowledge(w http.ResponseWriter, r *http.Request) {
	guestID := r.URL.Query().Get("guest_id")
	if guestID == "" {
		http.Error(w, "guest_id is required", http.StatusBadRequest)
		return
	}

	knowledge, err := h.aiService.GetGuestKnowledge(guestID)
	if err != nil {
		http.Error(w, "Failed to get knowledge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := utils.WriteJSONResponse(w, knowledge); err != nil {
		log.Error().Err(err).Msg("Failed to write knowledge response")
	}
}

// HandleSaveGuestNote saves a note for a guest
func (h *AISettingsHandler) HandleSaveGuestNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GuestID   string `json:"guest_id"`
		GuestName string `json:"guest_name"`
		GuestType string `json:"guest_type"`
		Category  string `json:"category"`
		Title     string `json:"title"`
		Content   string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.GuestID == "" || req.Category == "" || req.Title == "" || req.Content == "" {
		http.Error(w, "guest_id, category, title, and content are required", http.StatusBadRequest)
		return
	}

	if err := h.aiService.SaveGuestNote(req.GuestID, req.GuestName, req.GuestType, req.Category, req.Title, req.Content); err != nil {
		http.Error(w, "Failed to save note: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true}`))
}

// HandleDeleteGuestNote deletes a note from a guest
func (h *AISettingsHandler) HandleDeleteGuestNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GuestID string `json:"guest_id"`
		NoteID  string `json:"note_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.GuestID == "" || req.NoteID == "" {
		http.Error(w, "guest_id and note_id are required", http.StatusBadRequest)
		return
	}

	if err := h.aiService.DeleteGuestNote(req.GuestID, req.NoteID); err != nil {
		http.Error(w, "Failed to delete note: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true}`))
}

// HandleExportGuestKnowledge exports all knowledge for a guest as JSON
func (h *AISettingsHandler) HandleExportGuestKnowledge(w http.ResponseWriter, r *http.Request) {
	guestID := r.URL.Query().Get("guest_id")
	if guestID == "" {
		http.Error(w, "guest_id is required", http.StatusBadRequest)
		return
	}

	knowledge, err := h.aiService.GetGuestKnowledge(guestID)
	if err != nil {
		http.Error(w, "Failed to get knowledge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set headers for file download
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"pulse-notes-"+guestID+".json\"")

	if err := json.NewEncoder(w).Encode(knowledge); err != nil {
		log.Error().Err(err).Msg("Failed to encode knowledge export")
	}
}

// HandleImportGuestKnowledge imports knowledge from a JSON export
func (h *AISettingsHandler) HandleImportGuestKnowledge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body size to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var importData struct {
		GuestID   string `json:"guest_id"`
		GuestName string `json:"guest_name"`
		GuestType string `json:"guest_type"`
		Notes     []struct {
			Category string `json:"category"`
			Title    string `json:"title"`
			Content  string `json:"content"`
		} `json:"notes"`
		Merge bool `json:"merge"` // If true, add to existing notes; if false, replace
	}

	if err := json.NewDecoder(r.Body).Decode(&importData); err != nil {
		http.Error(w, "Invalid import data: "+err.Error(), http.StatusBadRequest)
		return
	}

	if importData.GuestID == "" {
		http.Error(w, "guest_id is required in import data", http.StatusBadRequest)
		return
	}

	if len(importData.Notes) == 0 {
		http.Error(w, "No notes to import", http.StatusBadRequest)
		return
	}

	// If not merging, we need to delete existing notes first
	if !importData.Merge {
		existing, err := h.aiService.GetGuestKnowledge(importData.GuestID)
		if err == nil && existing != nil {
			for _, note := range existing.Notes {
				_ = h.aiService.DeleteGuestNote(importData.GuestID, note.ID)
			}
		}
	}

	// Import each note
	imported := 0
	for _, note := range importData.Notes {
		if note.Category == "" || note.Title == "" || note.Content == "" {
			continue
		}
		if err := h.aiService.SaveGuestNote(
			importData.GuestID,
			importData.GuestName,
			importData.GuestType,
			note.Category,
			note.Title,
			note.Content,
		); err != nil {
			log.Warn().Err(err).Str("title", note.Title).Msg("Failed to import note")
			continue
		}
		imported++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"imported": imported,
		"total":    len(importData.Notes),
	})
}

// HandleClearGuestKnowledge deletes all notes for a guest
func (h *AISettingsHandler) HandleClearGuestKnowledge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GuestID string `json:"guest_id"`
		Confirm bool   `json:"confirm"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.GuestID == "" {
		http.Error(w, "guest_id is required", http.StatusBadRequest)
		return
	}

	if !req.Confirm {
		http.Error(w, "confirm must be true to clear all notes", http.StatusBadRequest)
		return
	}

	// Get existing knowledge and delete all notes
	existing, err := h.aiService.GetGuestKnowledge(req.GuestID)
	if err != nil {
		http.Error(w, "Failed to get knowledge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	deleted := 0
	for _, note := range existing.Notes {
		if err := h.aiService.DeleteGuestNote(req.GuestID, note.ID); err != nil {
			log.Warn().Err(err).Str("note_id", note.ID).Msg("Failed to delete note")
			continue
		}
		deleted++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"deleted": deleted,
	})
}

// HandleDebugContext returns the system prompt and context that would be sent to the AI
// This is useful for debugging when the AI gives incorrect information
func (h *AISettingsHandler) HandleDebugContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Build a sample request to see what context would be sent
	req := ai.ExecuteRequest{
		Prompt:     "Debug context request",
		TargetType: r.URL.Query().Get("target_type"),
		TargetID:   r.URL.Query().Get("target_id"),
	}

	// Get the debug context from the service
	debugInfo := h.aiService.GetDebugContext(req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(debugInfo)
}

// HandleGetConnectedAgents returns the list of agents currently connected via WebSocket
// This is useful for debugging when AI can't reach certain hosts
func (h *AISettingsHandler) HandleGetConnectedAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type agentInfo struct {
		AgentID     string `json:"agent_id"`
		Hostname    string `json:"hostname"`
		Version     string `json:"version"`
		Platform    string `json:"platform"`
		ConnectedAt string `json:"connected_at"`
	}

	var agents []agentInfo
	if h.agentServer != nil {
		for _, a := range h.agentServer.GetConnectedAgents() {
			agents = append(agents, agentInfo{
				AgentID:     a.AgentID,
				Hostname:    a.Hostname,
				Version:     a.Version,
				Platform:    a.Platform,
				ConnectedAt: a.ConnectedAt.Format(time.RFC3339),
			})
		}
	}

	response := map[string]interface{}{
		"count":  len(agents),
		"agents": agents,
		"note":   "Agents connect via WebSocket to /api/agent/ws. If a host is missing, check that pulse-agent is installed and can reach the Pulse server.",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AIInvestigateAlertRequest is the request body for POST /api/ai/investigate-alert
type AIInvestigateAlertRequest struct {
	AlertID      string  `json:"alert_id"`
	ResourceID   string  `json:"resource_id"`
	ResourceName string  `json:"resource_name"`
	ResourceType string  `json:"resource_type"` // guest, node, storage, docker
	AlertType    string  `json:"alert_type"`    // cpu, memory, disk, offline, etc.
	Level        string  `json:"level"`         // warning, critical
	Value        float64 `json:"value"`
	Threshold    float64 `json:"threshold"`
	Message      string  `json:"message"`
	Duration     string  `json:"duration"` // How long the alert has been active
	Node         string  `json:"node,omitempty"`
	VMID         int     `json:"vmid,omitempty"`
}

// HandleInvestigateAlert investigates an alert using AI (POST /api/ai/investigate-alert)
// This is a dedicated endpoint for one-click alert investigation from the UI
func (h *AISettingsHandler) HandleInvestigateAlert(w http.ResponseWriter, r *http.Request) {
	// Handle CORS
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Cookie")
		w.Header().Set("Vary", "Origin")
	}

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	// Check if AI is enabled
	if !h.aiService.IsEnabled() {
		http.Error(w, "AI is not enabled or configured", http.StatusBadRequest)
		return
	}

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var req AIInvestigateAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Build investigation prompt
	investigationPrompt := ai.GenerateAlertInvestigationPrompt(ai.AlertInvestigationRequest{
		AlertID:      req.AlertID,
		ResourceID:   req.ResourceID,
		ResourceName: req.ResourceName,
		ResourceType: req.ResourceType,
		AlertType:    req.AlertType,
		Level:        req.Level,
		Value:        req.Value,
		Threshold:    req.Threshold,
		Message:      req.Message,
		Duration:     req.Duration,
		Node:         req.Node,
		VMID:         req.VMID,
	})

	log.Info().
		Str("alert_id", req.AlertID).
		Str("resource", req.ResourceName).
		Str("type", req.AlertType).
		Msg("AI alert investigation started")

	// Set up SSE streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Transfer-Encoding", "identity")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Disable write/read deadlines for SSE
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})
	_ = rc.SetReadDeadline(time.Time{})

	flusher.Flush()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// Heartbeat routine
	heartbeatDone := make(chan struct{})
	var clientDisconnected bool
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
				_, err := w.Write([]byte(": heartbeat\n\n"))
				if err != nil {
					clientDisconnected = true
					return
				}
				flusher.Flush()
			case <-heartbeatDone:
				return
			}
		}
	}()
	defer close(heartbeatDone)

	safeWrite := func(data []byte) bool {
		if clientDisconnected {
			return false
		}
		_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err := w.Write(data)
		if err != nil {
			clientDisconnected = true
			return false
		}
		flusher.Flush()
		return true
	}

	// Determine target type and ID from alert info
	targetType := req.ResourceType
	targetID := req.ResourceID

	// Map resource type to expected target type format
	switch req.ResourceType {
	case "guest":
		// Could be VM or container - try to determine from VMID
		if req.VMID > 0 {
			targetType = "container" // Default to container, AI will figure it out
		}
	case "docker":
		targetType = "docker_container"
	}

	// Stream callback
	callback := func(event ai.StreamEvent) {
		if event.Type == "done" {
			return
		}
		data, err := json.Marshal(event)
		if err != nil {
			return
		}
		safeWrite([]byte("data: " + string(data) + "\n\n"))
	}

	// Execute with streaming
	defer func() {
		if !clientDisconnected {
			doneEvent := ai.StreamEvent{Type: "done"}
			data, _ := json.Marshal(doneEvent)
			safeWrite([]byte("data: " + string(data) + "\n\n"))
		}
	}()

	resp, err := h.aiService.ExecuteStream(ctx, ai.ExecuteRequest{
		Prompt:     investigationPrompt,
		TargetType: targetType,
		TargetID:   targetID,
		Context: map[string]interface{}{
			"alertId":      req.AlertID,
			"alertType":    req.AlertType,
			"alertLevel":   req.Level,
			"alertMessage": req.Message,
			"guestName":    req.ResourceName,
			"node":         req.Node,
		},
	}, callback)

	if err != nil {
		log.Error().Err(err).Msg("AI alert investigation failed")
		errEvent := ai.StreamEvent{Type: "error", Data: err.Error()}
		data, _ := json.Marshal(errEvent)
		safeWrite([]byte("data: " + string(data) + "\n\n"))
		return
	}

	// Send completion event
	finalEvent := struct {
		Type         string             `json:"type"`
		Model        string             `json:"model"`
		InputTokens  int                `json:"input_tokens"`
		OutputTokens int                `json:"output_tokens"`
		ToolCalls    []ai.ToolExecution `json:"tool_calls,omitempty"`
	}{
		Type:         "complete",
		Model:        resp.Model,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
		ToolCalls:    resp.ToolCalls,
	}
	data, _ := json.Marshal(finalEvent)
	safeWrite([]byte("data: " + string(data) + "\n\n"))

	log.Info().
		Str("alert_id", req.AlertID).
		Str("model", resp.Model).
		Int("tool_calls", len(resp.ToolCalls)).
		Msg("AI alert investigation completed")
}

// SetAlertProvider sets the alert provider for AI context
func (h *AISettingsHandler) SetAlertProvider(ap ai.AlertProvider) {
	h.aiService.SetAlertProvider(ap)
}

// oauthSessions stores active OAuth sessions (state -> session)
// In production, consider using a more robust session store with expiry
var oauthSessions = make(map[string]*providers.OAuthSession)
var oauthSessionsMu sync.Mutex

// HandleOAuthStart initiates the OAuth flow for Claude Pro/Max subscription (POST /api/ai/oauth/start)
// Returns an authorization URL for the user to visit manually
func (h *AISettingsHandler) HandleOAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate OAuth session (redirect URI is not used since we use Anthropic's callback)
	session, err := providers.GenerateOAuthSession("")
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate OAuth session")
		http.Error(w, "Failed to start OAuth flow", http.StatusInternalServerError)
		return
	}

	// Store session (with cleanup of old sessions)
	oauthSessionsMu.Lock()
	// Clean up sessions older than 15 minutes
	for state, s := range oauthSessions {
		if time.Since(s.CreatedAt) > 15*time.Minute {
			delete(oauthSessions, state)
		}
	}
	oauthSessions[session.State] = session
	oauthSessionsMu.Unlock()

	// Get authorization URL
	authURL := providers.GetAuthorizationURL(session)

	log.Info().
		Str("state", safePrefixForLog(session.State, 8)+"...").
		Str("verifier_len", fmt.Sprintf("%d", len(session.CodeVerifier))).
		Str("auth_url", authURL).
		Msg("Starting Claude OAuth flow - user must visit URL and paste code back")

	// Return the URL for the user to visit
	response := map[string]string{
		"auth_url": authURL,
		"state":    session.State,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write OAuth start response")
	}
}

// HandleOAuthExchange exchanges a manually-pasted authorization code for tokens (POST /api/ai/oauth/exchange)
func (h *AISettingsHandler) HandleOAuthExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Code == "" || req.State == "" {
		http.Error(w, "Missing code or state", http.StatusBadRequest)
		return
	}

	// Trim any whitespace from the code (user might have copied extra spaces)
	code := strings.TrimSpace(req.Code)

	// Anthropic's callback page displays the code as "code#state"
	// We need to extract just the code part before the #
	if idx := strings.Index(code, "#"); idx > 0 {
		code = code[:idx]
	}

	log.Debug().
		Str("code_len", fmt.Sprintf("%d", len(code))).
		Str("code_prefix", code[:min(20, len(code))]).
		Str("state_prefix", req.State[:min(8, len(req.State))]).
		Msg("Processing OAuth code exchange")

	// Look up session
	oauthSessionsMu.Lock()
	session, ok := oauthSessions[req.State]
	if ok {
		delete(oauthSessions, req.State) // One-time use
	}
	oauthSessionsMu.Unlock()

	if !ok {
		log.Error().Str("state", req.State[:min(8, len(req.State))]+"...").Msg("OAuth exchange with unknown state")
		http.Error(w, "Invalid or expired session. Please start the OAuth flow again.", http.StatusBadRequest)
		return
	}

	// Exchange code for tokens
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	tokens, err := providers.ExchangeCodeForTokens(ctx, code, session)
	if err != nil {
		log.Error().Err(err).Msg("Failed to exchange OAuth code for tokens")
		http.Error(w, "Failed to exchange authorization code: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Try to create an API key from the OAuth access token
	// Team/Enterprise users get org:create_api_key scope and can create API keys
	// Pro/Max users don't have this scope and will use OAuth tokens directly
	apiKey, err := providers.CreateAPIKeyFromOAuth(ctx, tokens.AccessToken)
	if err != nil {
		// Check if it's a permission error (Pro/Max users)
		if strings.Contains(err.Error(), "org:create_api_key") || strings.Contains(err.Error(), "403") {
			log.Info().Msg("User doesn't have org:create_api_key permission - will use OAuth tokens directly")
			// This is fine for Pro/Max users - they'll use OAuth tokens
		} else {
			log.Error().Err(err).Msg("Failed to create API key from OAuth token")
			http.Error(w, "Failed to create API key: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	if apiKey != "" {
		log.Info().Msg("Successfully created API key from OAuth - using subscription-based billing")
	}

	// Load existing settings
	settings, err := h.persistence.LoadAIConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load AI settings for OAuth")
		settings = config.NewDefaultAIConfig()
	}
	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}

	// Update settings
	settings.Provider = config.AIProviderAnthropic
	settings.AuthMethod = config.AuthMethodOAuth
	settings.OAuthAccessToken = tokens.AccessToken
	settings.OAuthRefreshToken = tokens.RefreshToken
	settings.OAuthExpiresAt = tokens.ExpiresAt
	settings.Enabled = true

	// If we got an API key, use it; otherwise use OAuth tokens directly
	if apiKey != "" {
		settings.APIKey = apiKey
	} else {
		// Pro/Max users: clear any old API key, will use OAuth client
		settings.ClearAPIKey()
	}

	// Save settings
	if err := h.persistence.SaveAIConfig(*settings); err != nil {
		log.Error().Err(err).Msg("Failed to save OAuth tokens")
		http.Error(w, "Failed to save OAuth credentials", http.StatusInternalServerError)
		return
	}

	// Reload the AI service with new settings
	if err := h.aiService.Reload(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload AI service after OAuth setup")
	}

	log.Info().Msg("Claude OAuth authentication successful")

	response := map[string]interface{}{
		"success": true,
		"message": "Successfully connected to Claude with your subscription",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write OAuth exchange response")
	}
}

// HandleOAuthCallback handles the OAuth callback (GET /api/ai/oauth/callback)
// This is kept for backwards compatibility but mainly serves as a fallback
func (h *AISettingsHandler) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get code and state from query params
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")
	errDesc := r.URL.Query().Get("error_description")

	// Check for OAuth error
	if errParam != "" {
		log.Error().
			Str("error", errParam).
			Str("description", errDesc).
			Msg("OAuth authorization failed")
		// Redirect to settings page with error
		http.Redirect(w, r, "/settings?ai_oauth_error="+errParam, http.StatusTemporaryRedirect)
		return
	}

	if code == "" || state == "" {
		log.Error().Msg("OAuth callback missing code or state")
		http.Redirect(w, r, "/settings?ai_oauth_error=missing_params", http.StatusTemporaryRedirect)
		return
	}

	// Look up session
	oauthSessionsMu.Lock()
	session, ok := oauthSessions[state]
	if ok {
		delete(oauthSessions, state) // One-time use
	}
	oauthSessionsMu.Unlock()

	if !ok {
		log.Error().Str("state", state).Msg("OAuth callback with unknown state")
		http.Redirect(w, r, "/settings?ai_oauth_error=invalid_state", http.StatusTemporaryRedirect)
		return
	}

	// Exchange code for tokens
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	tokens, err := providers.ExchangeCodeForTokens(ctx, code, session)
	if err != nil {
		log.Error().Err(err).Msg("Failed to exchange OAuth code for tokens")
		http.Redirect(w, r, "/settings?ai_oauth_error=token_exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	// Load existing settings
	settings, err := h.persistence.LoadAIConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load AI settings for OAuth")
		settings = config.NewDefaultAIConfig()
	}
	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}

	// Update settings with OAuth tokens
	settings.Provider = config.AIProviderAnthropic
	settings.AuthMethod = config.AuthMethodOAuth
	settings.OAuthAccessToken = tokens.AccessToken
	settings.OAuthRefreshToken = tokens.RefreshToken
	settings.OAuthExpiresAt = tokens.ExpiresAt
	settings.Enabled = true
	// Clear API key since we're using OAuth
	settings.ClearAPIKey()

	// Save settings
	if err := h.persistence.SaveAIConfig(*settings); err != nil {
		log.Error().Err(err).Msg("Failed to save OAuth tokens")
		http.Redirect(w, r, "/settings?ai_oauth_error=save_failed", http.StatusTemporaryRedirect)
		return
	}

	// Reload the AI service with new settings
	if err := h.aiService.Reload(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload AI service after OAuth setup")
	}

	log.Info().Msg("Claude OAuth authentication successful")

	// Redirect to settings page with success
	http.Redirect(w, r, "/settings?ai_oauth_success=true", http.StatusTemporaryRedirect)
}

// HandleOAuthDisconnect disconnects OAuth and clears tokens (POST /api/ai/oauth/disconnect)
func (h *AISettingsHandler) HandleOAuthDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require admin authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	// Load existing settings
	settings, err := h.persistence.LoadAIConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load AI settings for OAuth disconnect")
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}
	if settings == nil {
		settings = config.NewDefaultAIConfig()
	}

	// Clear OAuth tokens
	settings.ClearOAuthTokens()
	settings.AuthMethod = config.AuthMethodAPIKey

	// Save settings
	if err := h.persistence.SaveAIConfig(*settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings after OAuth disconnect")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	// Reload the AI service
	if err := h.aiService.Reload(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload AI service after OAuth disconnect")
	}

	log.Info().Msg("Claude OAuth disconnected")

	response := map[string]interface{}{
		"success": true,
		"message": "OAuth disconnected successfully",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write OAuth disconnect response")
	}
}

// PatrolStatusResponse is the response for GET /api/ai/patrol/status
type PatrolStatusResponse struct {
	Running          bool       `json:"running"`
	Enabled          bool       `json:"enabled"`
	LastPatrolAt     *time.Time `json:"last_patrol_at,omitempty"`
	LastDeepAnalysis *time.Time `json:"last_deep_analysis_at,omitempty"`
	NextPatrolAt     *time.Time `json:"next_patrol_at,omitempty"`
	LastDurationMs   int64      `json:"last_duration_ms"`
	ResourcesChecked int        `json:"resources_checked"`
	FindingsCount    int        `json:"findings_count"`
	Healthy          bool       `json:"healthy"`
	Summary          struct {
		Critical int `json:"critical"`
		Warning  int `json:"warning"`
		Watch    int `json:"watch"`
		Info     int `json:"info"`
	} `json:"summary"`
}

// HandleGetPatrolStatus returns the current patrol status (GET /api/ai/patrol/status)
func (h *AISettingsHandler) HandleGetPatrolStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		// Patrol not initialized
		response := PatrolStatusResponse{
			Running: false,
			Enabled: false,
			Healthy: true,
		}
		if err := utils.WriteJSONResponse(w, response); err != nil {
			log.Error().Err(err).Msg("Failed to write patrol status response")
		}
		return
	}

	status := patrol.GetStatus()
	summary := patrol.GetFindingsSummary()

	response := PatrolStatusResponse{
		Running:          status.Running,
		Enabled:          h.aiService.IsEnabled(),
		LastPatrolAt:     status.LastPatrolAt,
		NextPatrolAt:     status.NextPatrolAt,
		LastDurationMs:   status.LastDuration.Milliseconds(),
		ResourcesChecked: status.ResourcesChecked,
		FindingsCount:    status.FindingsCount,
		Healthy:          status.Healthy,
	}
	response.Summary.Critical = summary.Critical
	response.Summary.Warning = summary.Warning
	response.Summary.Watch = summary.Watch
	response.Summary.Info = summary.Info

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write patrol status response")
	}
}

// HandlePatrolStream streams real-time patrol analysis via SSE (GET /api/ai/patrol/stream)
func (h *AISettingsHandler) HandlePatrolStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Subscribe to patrol stream
	// Note: SubscribeToStream already sends the current buffered output to the channel
	ch := patrol.SubscribeToStream()
	defer patrol.UnsubscribeFromStream(ch)

	// Stream events until client disconnects
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// HandleGetPatrolFindings returns all active findings (GET /api/ai/patrol/findings)
func (h *AISettingsHandler) HandleGetPatrolFindings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		// Return empty findings
		if err := utils.WriteJSONResponse(w, []interface{}{}); err != nil {
			log.Error().Err(err).Msg("Failed to write patrol findings response")
		}
		return
	}

	// Check for resource_id query parameter
	resourceID := r.URL.Query().Get("resource_id")
	var findings []*ai.Finding
	if resourceID != "" {
		findings = patrol.GetFindingsForResource(resourceID)
	} else {
		findings = patrol.GetAllFindings()
	}

	if err := utils.WriteJSONResponse(w, findings); err != nil {
		log.Error().Err(err).Msg("Failed to write patrol findings response")
	}
}

// HandleForcePatrol triggers an immediate patrol run (POST /api/ai/patrol/run)
func (h *AISettingsHandler) HandleForcePatrol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require admin authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	// Check for deep=true query parameter
	deep := r.URL.Query().Get("deep") == "true"

	// Trigger patrol asynchronously
	patrol.ForcePatrol(r.Context(), deep)

	patrolType := "quick"
	if deep {
		patrolType = "deep"
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Triggered %s patrol run", patrolType),
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write force patrol response")
	}
}

// HandleAcknowledgeFinding acknowledges a finding (POST /api/ai/patrol/acknowledge)
// This marks the finding as seen but keeps it visible (dimmed). Auto-resolve removes it when condition clears.
// This matches alert acknowledgement behavior for UI consistency.
func (h *AISettingsHandler) HandleAcknowledgeFinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FindingID string `json:"finding_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FindingID == "" {
		http.Error(w, "finding_id is required", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()

	// Just acknowledge - don't resolve. Finding stays visible but marked as seen.
	// Auto-resolve will remove it when the underlying condition clears.
	if !findings.Acknowledge(req.FindingID) {
		http.Error(w, "Finding not found", http.StatusNotFound)
		return
	}

	log.Info().
		Str("finding_id", req.FindingID).
		Msg("AI Patrol: Finding acknowledged by user")

	response := map[string]interface{}{
		"success": true,
		"message": "Finding acknowledged",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write acknowledge response")
	}
}

// HandleSnoozeFinding snoozes a finding for a specified duration (POST /api/ai/patrol/snooze)
// Snoozed findings are hidden from the active list but will reappear if condition persists after snooze expires
func (h *AISettingsHandler) HandleSnoozeFinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FindingID     string `json:"finding_id"`
		DurationHours int    `json:"duration_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FindingID == "" {
		http.Error(w, "finding_id is required", http.StatusBadRequest)
		return
	}

	if req.DurationHours <= 0 {
		http.Error(w, "duration_hours must be positive", http.StatusBadRequest)
		return
	}

	// Cap snooze duration at 7 days
	if req.DurationHours > 168 {
		req.DurationHours = 168
	}

	findings := patrol.GetFindings()
	duration := time.Duration(req.DurationHours) * time.Hour

	if !findings.Snooze(req.FindingID, duration) {
		http.Error(w, "Finding not found or already resolved", http.StatusNotFound)
		return
	}

	log.Info().
		Str("finding_id", req.FindingID).
		Int("hours", req.DurationHours).
		Msg("AI Patrol: Finding snoozed by user")

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Finding snoozed for %d hours", req.DurationHours),
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write snooze response")
	}
}

// HandleResolveFinding manually marks a finding as resolved (POST /api/ai/patrol/resolve)
// Use this when the user has fixed the issue and wants to mark it as resolved
func (h *AISettingsHandler) HandleResolveFinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FindingID string `json:"finding_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FindingID == "" {
		http.Error(w, "finding_id is required", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()

	// Mark as manually resolved (auto=false since user did it)
	if !findings.Resolve(req.FindingID, false) {
		http.Error(w, "Finding not found or already resolved", http.StatusNotFound)
		return
	}

	log.Info().
		Str("finding_id", req.FindingID).
		Msg("AI Patrol: Finding manually resolved by user")

	response := map[string]interface{}{
		"success": true,
		"message": "Finding marked as resolved",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write resolve response")
	}
}

// HandleDismissFinding dismisses a finding with a reason and optional note (POST /api/ai/patrol/dismiss)
// This is part of the LLM memory system - dismissed findings are included in context to prevent re-raising
// Valid reasons: "not_an_issue", "expected_behavior", "will_fix_later"
func (h *AISettingsHandler) HandleDismissFinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FindingID string `json:"finding_id"`
		Reason    string `json:"reason"` // "not_an_issue", "expected_behavior", "will_fix_later"
		Note      string `json:"note"`   // Optional freeform note
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FindingID == "" {
		http.Error(w, "finding_id is required", http.StatusBadRequest)
		return
	}

	// Validate reason
	validReasons := map[string]bool{
		"not_an_issue":      true,
		"expected_behavior": true,
		"will_fix_later":    true,
	}
	if req.Reason != "" && !validReasons[req.Reason] {
		http.Error(w, "Invalid reason. Valid values: not_an_issue, expected_behavior, will_fix_later", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()

	if !findings.Dismiss(req.FindingID, req.Reason, req.Note) {
		http.Error(w, "Finding not found", http.StatusNotFound)
		return
	}

	log.Info().
		Str("finding_id", req.FindingID).
		Str("reason", req.Reason).
		Bool("has_note", req.Note != "").
		Msg("AI Patrol: Finding dismissed by user with reason")

	response := map[string]interface{}{
		"success": true,
		"message": "Finding dismissed",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write dismiss response")
	}
}

// HandleSuppressFinding permanently suppresses similar findings for a resource (POST /api/ai/patrol/suppress)
// The LLM will be told never to re-raise findings of this type for this resource
func (h *AISettingsHandler) HandleSuppressFinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FindingID string `json:"finding_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FindingID == "" {
		http.Error(w, "finding_id is required", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()

	if !findings.Suppress(req.FindingID) {
		http.Error(w, "Finding not found", http.StatusNotFound)
		return
	}

	log.Info().
		Str("finding_id", req.FindingID).
		Msg("AI Patrol: Finding type permanently suppressed by user")

	response := map[string]interface{}{
		"success": true,
		"message": "Finding type suppressed - similar issues will not be raised again",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write suppress response")
	}
}

// HandleGetFindingsHistory returns all findings including resolved for history (GET /api/ai/patrol/history)
func (h *AISettingsHandler) HandleGetFindingsHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		// Return empty history
		if err := utils.WriteJSONResponse(w, []interface{}{}); err != nil {
			log.Error().Err(err).Msg("Failed to write findings history response")
		}
		return
	}

	// Parse optional startTime query parameter
	var startTime *time.Time
	if startTimeStr := r.URL.Query().Get("start_time"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = &t
		}
	}

	findings := patrol.GetFindingsHistory(startTime)

	if err := utils.WriteJSONResponse(w, findings); err != nil {
		log.Error().Err(err).Msg("Failed to write findings history response")
	}
}

// HandleGetPatrolRunHistory returns the history of patrol check runs (GET /api/ai/patrol/runs)
func (h *AISettingsHandler) HandleGetPatrolRunHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		// Return empty history
		if err := utils.WriteJSONResponse(w, []interface{}{}); err != nil {
			log.Error().Err(err).Msg("Failed to write patrol run history response")
		}
		return
	}

	// Parse optional limit query parameter (default: 50)
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && l > 0 {
			if limit > 100 {
				limit = 100 // Cap at MaxPatrolRunHistory
			}
		}
	}

	runs := patrol.GetRunHistory(limit)

	if err := utils.WriteJSONResponse(w, runs); err != nil {
		log.Error().Err(err).Msg("Failed to write patrol run history response")
	}
}

// HandleGetAICostSummary returns AI usage rollups (GET /api/ai/cost/summary?days=N).
func (h *AISettingsHandler) HandleGetAICostSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse optional days query parameter (default: 30, max: 365)
	days := 30
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if _, err := fmt.Sscanf(daysStr, "%d", &days); err == nil && days > 0 {
			if days > 365 {
				days = 365
			}
		}
	}

	var summary cost.Summary
	if h.aiService != nil {
		summary = h.aiService.GetCostSummary(days)
	} else {
		summary = cost.Summary{
			Days:           days,
			ProviderModels: []cost.ProviderModelSummary{},
			DailyTotals:    []cost.DailySummary{},
			Totals:         cost.ProviderModelSummary{Provider: "all"},
		}
	}

	if err := utils.WriteJSONResponse(w, summary); err != nil {
		log.Error().Err(err).Msg("Failed to write AI cost summary response")
	}
}

// HandleResetAICostHistory deletes retained AI usage events (POST /api/ai/cost/reset).
func (h *AISettingsHandler) HandleResetAICostHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.aiService == nil {
		http.Error(w, "AI service unavailable", http.StatusServiceUnavailable)
		return
	}

	backupFile := ""
	if h.persistence != nil {
		configDir := h.persistence.DataDir()
		if strings.TrimSpace(configDir) != "" {
			usagePath := filepath.Join(configDir, "ai_usage_history.json")
			if _, err := os.Stat(usagePath); err == nil {
				ts := time.Now().UTC().Format("20060102-150405")
				backupFile = fmt.Sprintf("ai_usage_history.json.bak-%s", ts)
				backupPath := filepath.Join(configDir, backupFile)
				if err := os.Rename(usagePath, backupPath); err != nil {
					log.Error().Err(err).Str("path", usagePath).Msg("Failed to backup AI usage history before reset")
					http.Error(w, "Failed to backup AI usage history", http.StatusInternalServerError)
					return
				}
			}
		}
	}

	if err := h.aiService.ClearCostHistory(); err != nil {
		log.Error().Err(err).Msg("Failed to clear AI cost history")
		http.Error(w, "Failed to clear AI cost history", http.StatusInternalServerError)
		return
	}

	resp := map[string]any{"ok": true}
	if backupFile != "" {
		resp["backup_file"] = backupFile
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to write clear cost history response")
	}
}

// HandleExportAICostHistory exports recent AI usage history as JSON or CSV (GET /api/ai/cost/export?days=N&format=csv|json).
func (h *AISettingsHandler) HandleExportAICostHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.aiService == nil {
		http.Error(w, "AI service unavailable", http.StatusServiceUnavailable)
		return
	}

	days := 30
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if v, err := strconv.Atoi(daysStr); err == nil && v > 0 {
			if v > 365 {
				v = 365
			}
			days = v
		}
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		http.Error(w, "format must be 'json' or 'csv'", http.StatusBadRequest)
		return
	}

	events := h.aiService.ListCostEvents(days)

	filename := fmt.Sprintf("pulse-ai-usage-%s-%dd.%s", time.Now().UTC().Format("20060102"), days, format)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		type exportEvent struct {
			cost.UsageEvent
			EstimatedUSD float64 `json:"estimated_usd,omitempty"`
			PricingKnown bool    `json:"pricing_known"`
		}
		exported := make([]exportEvent, 0, len(events))
		for _, e := range events {
			provider, model := cost.ResolveProviderAndModel(e.Provider, e.RequestModel, e.ResponseModel)
			usd, ok, _ := cost.EstimateUSD(provider, model, int64(e.InputTokens), int64(e.OutputTokens))
			exported = append(exported, exportEvent{
				UsageEvent:   e,
				EstimatedUSD: usd,
				PricingKnown: ok,
			})
		}
		resp := map[string]any{
			"days":   days,
			"events": exported,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error().Err(err).Msg("Failed to write AI cost export JSON")
		}
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"timestamp",
		"provider",
		"request_model",
		"response_model",
		"use_case",
		"input_tokens",
		"output_tokens",
		"estimated_usd",
		"pricing_known",
		"target_type",
		"target_id",
		"finding_id",
	})
	for _, e := range events {
		provider, model := cost.ResolveProviderAndModel(e.Provider, e.RequestModel, e.ResponseModel)
		usd, ok, _ := cost.EstimateUSD(provider, model, int64(e.InputTokens), int64(e.OutputTokens))

		_ = cw.Write([]string{
			e.Timestamp.UTC().Format(time.RFC3339Nano),
			e.Provider,
			e.RequestModel,
			e.ResponseModel,
			e.UseCase,
			strconv.Itoa(e.InputTokens),
			strconv.Itoa(e.OutputTokens),
			strconv.FormatFloat(usd, 'f', 6, 64),
			strconv.FormatBool(ok),
			e.TargetType,
			e.TargetID,
			e.FindingID,
		})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		log.Error().Err(err).Msg("Failed to write AI cost export CSV")
	}
}

// HandleGetSuppressionRules returns all suppression rules (GET /api/ai/patrol/suppressions)
func (h *AISettingsHandler) HandleGetSuppressionRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, []interface{}{}); err != nil {
			log.Error().Err(err).Msg("Failed to write suppression rules response")
		}
		return
	}

	findings := patrol.GetFindings()
	rules := findings.GetSuppressionRules()

	if err := utils.WriteJSONResponse(w, rules); err != nil {
		log.Error().Err(err).Msg("Failed to write suppression rules response")
	}
}

// HandleAddSuppressionRule creates a new suppression rule (POST /api/ai/patrol/suppressions)
func (h *AISettingsHandler) HandleAddSuppressionRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ResourceID   string `json:"resource_id"`   // Can be empty for "any resource"
		ResourceName string `json:"resource_name"` // Human-readable name
		Category     string `json:"category"`      // Can be empty for "any category"
		Description  string `json:"description"`   // Required - user's reason
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Description == "" {
		http.Error(w, "description is required", http.StatusBadRequest)
		return
	}

	// Convert category string to FindingCategory
	var category ai.FindingCategory
	switch req.Category {
	case "performance":
		category = ai.FindingCategoryPerformance
	case "capacity":
		category = ai.FindingCategoryCapacity
	case "reliability":
		category = ai.FindingCategoryReliability
	case "backup":
		category = ai.FindingCategoryBackup
	case "security":
		category = ai.FindingCategorySecurity
	case "general":
		category = ai.FindingCategoryGeneral
	case "":
		category = "" // Any category
	default:
		http.Error(w, "Invalid category", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()
	rule := findings.AddSuppressionRule(req.ResourceID, req.ResourceName, category, req.Description)

	log.Info().
		Str("rule_id", rule.ID).
		Str("resource_id", req.ResourceID).
		Str("category", req.Category).
		Str("description", req.Description).
		Msg("AI Patrol: Manual suppression rule created")

	response := map[string]interface{}{
		"success": true,
		"message": "Suppression rule created",
		"rule":    rule,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write add suppression rule response")
	}
}

// HandleDeleteSuppressionRule removes a suppression rule (DELETE /api/ai/patrol/suppressions/:id)
func (h *AISettingsHandler) HandleDeleteSuppressionRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	// Get rule ID from URL path
	ruleID := strings.TrimPrefix(r.URL.Path, "/api/ai/patrol/suppressions/")
	if ruleID == "" {
		http.Error(w, "rule_id is required", http.StatusBadRequest)
		return
	}

	findings := patrol.GetFindings()

	if !findings.DeleteSuppressionRule(ruleID) {
		http.Error(w, "Rule not found", http.StatusNotFound)
		return
	}

	log.Info().
		Str("rule_id", ruleID).
		Msg("AI Patrol: Suppression rule deleted")

	response := map[string]interface{}{
		"success": true,
		"message": "Suppression rule deleted",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write delete suppression rule response")
	}
}

// HandleGetDismissedFindings returns all dismissed/suppressed findings (GET /api/ai/patrol/dismissed)
func (h *AISettingsHandler) HandleGetDismissedFindings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, []interface{}{}); err != nil {
			log.Error().Err(err).Msg("Failed to write dismissed findings response")
		}
		return
	}

	findings := patrol.GetFindings()
	dismissed := findings.GetDismissedFindings()

	if err := utils.WriteJSONResponse(w, dismissed); err != nil {
		log.Error().Err(err).Msg("Failed to write dismissed findings response")
	}
}
