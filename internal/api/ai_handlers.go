package api

import (
	"context"
	"encoding/json"
	"io"
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
	}
	// Also disable read deadline
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Msg("Failed to disable read deadline for SSE")
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

	// Execute with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
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
