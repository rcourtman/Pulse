package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
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

// AISettingsResponse is returned by GET /api/settings/ai
// API key is masked for security
type AISettingsResponse struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`
	APIKeySet      bool   `json:"api_key_set"` // true if API key is configured (never expose actual key)
	Model          string `json:"model"`
	BaseURL        string `json:"base_url,omitempty"`
	Configured     bool   `json:"configured"`      // true if AI is ready to use
	AutonomousMode bool   `json:"autonomous_mode"` // true if AI can execute without approval
	CustomContext  string `json:"custom_context"`  // user-provided infrastructure context
}

// AISettingsUpdateRequest is the request body for PUT /api/settings/ai
type AISettingsUpdateRequest struct {
	Enabled        *bool   `json:"enabled,omitempty"`
	Provider       *string `json:"provider,omitempty"`
	APIKey         *string `json:"api_key,omitempty"` // empty string clears, null preserves
	Model          *string `json:"model,omitempty"`
	BaseURL        *string `json:"base_url,omitempty"`
	AutonomousMode *bool   `json:"autonomous_mode,omitempty"`
	CustomContext  *string `json:"custom_context,omitempty"` // user-provided infrastructure context
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

	response := AISettingsResponse{
		Enabled:        settings.Enabled,
		Provider:       settings.Provider,
		APIKeySet:      settings.APIKey != "",
		Model:          settings.GetModel(),
		BaseURL:        settings.BaseURL,
		Configured:     settings.IsConfigured(),
		AutonomousMode: settings.AutonomousMode,
		CustomContext:  settings.CustomContext,
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
		case config.AIProviderAnthropic, config.AIProviderOpenAI, config.AIProviderOllama:
			settings.Provider = provider
		default:
			http.Error(w, "Invalid provider. Must be 'anthropic', 'openai', or 'ollama'", http.StatusBadRequest)
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
			case config.AIProviderAnthropic, config.AIProviderOpenAI:
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

	log.Info().
		Bool("enabled", settings.Enabled).
		Str("provider", settings.Provider).
		Str("model", settings.GetModel()).
		Msg("AI settings updated")

	// Return updated settings
	response := AISettingsResponse{
		Enabled:        settings.Enabled,
		Provider:       settings.Provider,
		APIKeySet:      settings.APIKey != "",
		Model:          settings.GetModel(),
		BaseURL:        settings.BaseURL,
		Configured:     settings.IsConfigured(),
		AutonomousMode: settings.AutonomousMode,
		CustomContext:  settings.CustomContext,
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

// AIExecuteRequest is the request body for POST /api/ai/execute
// AIConversationMessage represents a message in conversation history
type AIConversationMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"`
}

type AIExecuteRequest struct {
	Prompt     string                   `json:"prompt"`
	TargetType string                   `json:"target_type,omitempty"` // "host", "container", "vm", "node"
	TargetID   string                   `json:"target_id,omitempty"`
	Context    map[string]interface{}   `json:"context,omitempty"` // Current metrics, state, etc.
	History    []AIConversationMessage  `json:"history,omitempty"` // Previous conversation messages
}

// AIExecuteResponse is the response from POST /api/ai/execute
type AIExecuteResponse struct {
	Content      string                 `json:"content"`
	Model        string                 `json:"model"`
	InputTokens  int                    `json:"input_tokens"`
	OutputTokens int                    `json:"output_tokens"`
	ToolCalls    []ai.ToolExecution     `json:"tool_calls,omitempty"` // Commands that were executed
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
		// Continue anyway - heartbeats should help keep connection alive
	}

	// Flush headers immediately
	flusher.Flush()

	// Create context with timeout (5 minutes for complex analysis with multiple tool calls)
	// Use background context to avoid browser disconnect canceling the request
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// Set up heartbeat to keep connection alive during long tool executions
	// NOTE: We don't check r.Context().Done() because Vite proxy may close
	// the request context prematurely. We detect real disconnection via write failures.
	heartbeatDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Send SSE comment as heartbeat
				_, err := w.Write([]byte(": heartbeat\n\n"))
				if err != nil {
					log.Debug().Err(err).Msg("Heartbeat write failed, client disconnected")
					cancel() // Cancel the AI request
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

	// Stream callback - write SSE events
	callback := func(event ai.StreamEvent) {
		data, err := json.Marshal(event)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal stream event")
			return
		}

		log.Debug().
			Str("event_type", event.Type).
			Msg("Streaming AI event")

		// SSE format: data: <json>\n\n
		_, writeErr := w.Write([]byte("data: " + string(data) + "\n\n"))
		if writeErr != nil {
			log.Debug().Err(writeErr).Msg("Failed to write SSE event (client may have disconnected)")
			return
		}
		flusher.Flush()
	}

	// Convert history from API type to service type
	var history []ai.ConversationMessage
	for _, msg := range req.History {
		history = append(history, ai.ConversationMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

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
		_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
		flusher.Flush()
		return
	}

	log.Info().
		Str("model", resp.Model).
		Int("input_tokens", resp.InputTokens).
		Int("output_tokens", resp.OutputTokens).
		Int("tool_calls", len(resp.ToolCalls)).
		Msg("AI streaming request completed")

	// Send final response with metadata
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
	_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
	flusher.Flush()
}

// AIRunCommandRequest is the request body for POST /api/ai/run-command
type AIRunCommandRequest struct {
	Command    string `json:"command"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	RunOnHost  bool   `json:"run_on_host"`
	VMID       string `json:"vmid,omitempty"`
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
	var req AIRunCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
		Msg("Executing approved command")

	// Execute with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	resp, err := h.aiService.RunCommand(ctx, ai.RunCommandRequest{
		Command:    req.Command,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		RunOnHost:  req.RunOnHost,
		VMID:       req.VMID,
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
