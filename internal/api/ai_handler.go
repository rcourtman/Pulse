package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/opencode"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// AIHandler handles all AI endpoints using OpenCode
type AIHandler struct {
	config      *config.Config
	persistence *config.ConfigPersistence
	service     *opencode.Service
	agentServer *agentexec.Server
}

// NewAIHandler creates a new AI handler
func NewAIHandler(cfg *config.Config, persistence *config.ConfigPersistence, agentServer *agentexec.Server) *AIHandler {
	return &AIHandler{
		config:      cfg,
		persistence: persistence,
		agentServer: agentServer,
	}
}

// StateProvider interface for infrastructure state
type AIStateProvider interface {
	GetState() models.StateSnapshot
}

// Start initializes and starts the OpenCode service
func (h *AIHandler) Start(ctx context.Context, stateProvider AIStateProvider) error {
	log.Info().Msg("AIHandler.Start called")
	aiCfg := h.loadAIConfig()
	if aiCfg == nil {
		log.Info().Msg("AI config is nil, AI is disabled")
		return nil
	}
	if !aiCfg.Enabled {
		log.Info().Bool("enabled", aiCfg.Enabled).Msg("AI is disabled in config")
		return nil
	}

	log.Info().Bool("enabled", aiCfg.Enabled).Str("model", aiCfg.Model).Msg("Starting OpenCode service")
	h.service = opencode.NewService(opencode.Config{
		AIConfig:      aiCfg,
		StateProvider: stateProvider,
		AgentServer:   h.agentServer,
	})

	if err := h.service.Start(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to start OpenCode service")
		return err
	}

	// Initialize approval store for command approval workflow
	dataDir := aiCfg.OpenCodeDataDir
	if dataDir == "" {
		dataDir = "/tmp/pulse-opencode"
	}

	approvalStore, err := approval.NewStore(approval.StoreConfig{
		DataDir:        dataDir,
		DefaultTimeout: 5 * time.Minute,
		MaxApprovals:   100,
	})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create approval store, approvals will not be persisted")
	} else {
		approval.SetStore(approvalStore)
		approvalStore.StartCleanup(ctx)
		log.Info().Str("data_dir", dataDir).Msg("Approval store initialized")
	}

	log.Info().Msg("Pulse AI started (powered by OpenCode)")
	return nil
}

// Stop stops the OpenCode service
func (h *AIHandler) Stop(ctx context.Context) error {
	if h.service != nil {
		return h.service.Stop(ctx)
	}
	return nil
}

// Restart restarts the OpenCode service with updated configuration
// Call this when model or other settings change
func (h *AIHandler) Restart(ctx context.Context) error {
	if h.service == nil || !h.service.IsRunning() {
		return nil // Not running, nothing to restart
	}
	// Load fresh config from persistence to get latest settings
	newCfg := h.loadAIConfig()
	return h.service.Restart(ctx, newCfg)
}

// IsRunning returns whether AI is running
func (h *AIHandler) IsRunning() bool {
	return h.service != nil && h.service.IsRunning()
}

// GetService returns the underlying OpenCode service for direct access
func (h *AIHandler) GetService() *opencode.Service {
	return h.service
}

// GetAIConfig returns the current AI configuration
func (h *AIHandler) GetAIConfig() *config.AIConfig {
	return h.loadAIConfig()
}

func (h *AIHandler) loadAIConfig() *config.AIConfig {
	if h.persistence == nil {
		return nil
	}
	cfg, err := h.persistence.LoadAIConfig()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load AI config")
		return nil
	}
	return cfg
}

// ChatRequest represents a chat request
type ChatRequest struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id,omitempty"`
	Model     string `json:"model,omitempty"`
}

// HandleChat handles POST /api/ai/chat - streaming chat
func (h *AIHandler) HandleChat(w http.ResponseWriter, r *http.Request) {
	// CORS
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

	// Auth already handled by RequireAuth wrapper - no need to check again

	if !h.IsRunning() {
		http.Error(w, "AI is not running", http.StatusServiceUnavailable)
		return
	}

	// Parse request
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Set up SSE
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

	// Disable timeouts
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})
	_ = rc.SetReadDeadline(time.Time{})

	flusher.Flush()

	// Context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Heartbeat
	heartbeatDone := make(chan struct{})
	var clientDisconnected atomic.Bool

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
				_, err := w.Write([]byte(": heartbeat\n\n"))
				if err != nil {
					clientDisconnected.Store(true)
					return
				}
				flusher.Flush()
			case <-heartbeatDone:
				return
			}
		}
	}()
	defer close(heartbeatDone)

	// Write helper
	writeEvent := func(event opencode.StreamEvent) {
		if clientDisconnected.Load() {
			return
		}
		data, err := json.Marshal(event)
		if err != nil {
			return
		}
		_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err = w.Write([]byte("data: " + string(data) + "\n\n"))
		if err != nil {
			clientDisconnected.Store(true)
			return
		}
		flusher.Flush()
	}

	// Stream from OpenCode
	err := h.service.ExecuteStream(ctx, opencode.ExecuteRequest{
		Prompt:    req.Prompt,
		SessionID: req.SessionID,
		Model:     req.Model,
	}, func(event opencode.StreamEvent) {
		writeEvent(event)
	})

	if err != nil {
		log.Error().Err(err).Msg("Chat stream error")
		errData, _ := json.Marshal(err.Error())
		writeEvent(opencode.StreamEvent{Type: "error", Data: errData})
	}

	// Send done
	writeEvent(opencode.StreamEvent{Type: "done", Data: nil})
}

// HandleSessions handles GET /api/ai/sessions - list sessions
func (h *AIHandler) HandleSessions(w http.ResponseWriter, r *http.Request) {
	if !CheckAuth(h.config, w, r) {
		return
	}

	if !h.IsRunning() {
		http.Error(w, "AI is not running", http.StatusServiceUnavailable)
		return
	}

	sessions, err := h.service.ListSessions(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// HandleCreateSession handles POST /api/ai/sessions - create session
func (h *AIHandler) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	if !CheckAuth(h.config, w, r) {
		return
	}

	if !h.IsRunning() {
		http.Error(w, "AI is not running", http.StatusServiceUnavailable)
		return
	}

	session, err := h.service.CreateSession(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// HandleDeleteSession handles DELETE /api/ai/sessions/{id}
func (h *AIHandler) HandleDeleteSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	if !CheckAuth(h.config, w, r) {
		return
	}

	if !h.IsRunning() {
		http.Error(w, "AI is not running", http.StatusServiceUnavailable)
		return
	}

	if err := h.service.DeleteSession(r.Context(), sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleMessages handles GET /api/ai/sessions/{id}/messages
func (h *AIHandler) HandleMessages(w http.ResponseWriter, r *http.Request, sessionID string) {
	if !CheckAuth(h.config, w, r) {
		return
	}

	if !h.IsRunning() {
		http.Error(w, "AI is not running", http.StatusServiceUnavailable)
		return
	}

	messages, err := h.service.GetMessages(r.Context(), sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// HandleAbort handles POST /api/ai/sessions/{id}/abort
func (h *AIHandler) HandleAbort(w http.ResponseWriter, r *http.Request, sessionID string) {
	if !CheckAuth(h.config, w, r) {
		return
	}

	if !h.IsRunning() {
		http.Error(w, "AI is not running", http.StatusServiceUnavailable)
		return
	}

	if err := h.service.AbortSession(r.Context(), sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleStatus handles GET /api/ai/status
func (h *AIHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if !CheckAuth(h.config, w, r) {
		return
	}

	status := map[string]interface{}{
		"running": h.IsRunning(),
		"engine":  "opencode",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// AnswerQuestionRequest represents a request to answer a question
type AnswerQuestionRequest struct {
	Answers []struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	} `json:"answers"`
}

// HandleAnswerQuestion handles POST /api/ai/question/{questionID}/answer
func (h *AIHandler) HandleAnswerQuestion(w http.ResponseWriter, r *http.Request, questionID string) {
	if !CheckAuth(h.config, w, r) {
		return
	}

	if !h.IsRunning() {
		http.Error(w, "AI is not running", http.StatusServiceUnavailable)
		return
	}

	var req AnswerQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Convert to opencode.QuestionAnswer
	answers := make([]opencode.QuestionAnswer, len(req.Answers))
	for i, a := range req.Answers {
		answers[i] = opencode.QuestionAnswer{
			ID:    a.ID,
			Value: a.Value,
		}
	}

	if err := h.service.AnswerQuestion(r.Context(), questionID, answers); err != nil {
		log.Error().Err(err).Str("questionID", questionID).Msg("Failed to answer question")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// SetAlertProvider sets the alert provider for MCP tools
func (h *AIHandler) SetAlertProvider(provider opencode.MCPAlertProvider) {
	if h.service != nil {
		h.service.SetAlertProvider(provider)
	}
}

// SetFindingsProvider sets the findings provider for MCP tools
func (h *AIHandler) SetFindingsProvider(provider opencode.MCPFindingsProvider) {
	if h.service != nil {
		h.service.SetFindingsProvider(provider)
	}
}

// SetBaselineProvider sets the baseline provider for MCP tools
func (h *AIHandler) SetBaselineProvider(provider opencode.MCPBaselineProvider) {
	if h.service != nil {
		h.service.SetBaselineProvider(provider)
	}
}

// SetPatternProvider sets the pattern provider for MCP tools
func (h *AIHandler) SetPatternProvider(provider opencode.MCPPatternProvider) {
	if h.service != nil {
		h.service.SetPatternProvider(provider)
	}
}

// SetMetricsHistory sets the metrics history provider for MCP tools
func (h *AIHandler) SetMetricsHistory(provider opencode.MCPMetricsHistoryProvider) {
	if h.service != nil {
		h.service.SetMetricsHistory(provider)
	}
}
