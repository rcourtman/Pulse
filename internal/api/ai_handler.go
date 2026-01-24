package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rs/zerolog/log"
)

// AIPersistence interface for loading/saving AI config
type AIPersistence interface {
	LoadAIConfig() (*config.AIConfig, error)
	DataDir() string
}

// AIService interface for the AI chat service - enables mocking in tests
type AIService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context, newCfg *config.AIConfig) error
	IsRunning() bool
	Execute(ctx context.Context, req chat.ExecuteRequest) (map[string]interface{}, error)
	ExecuteStream(ctx context.Context, req chat.ExecuteRequest, callback chat.StreamCallback) error
	ListSessions(ctx context.Context) ([]chat.Session, error)
	CreateSession(ctx context.Context) (*chat.Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	GetMessages(ctx context.Context, sessionID string) ([]chat.Message, error)
	AbortSession(ctx context.Context, sessionID string) error
	SummarizeSession(ctx context.Context, sessionID string) (map[string]interface{}, error)
	GetSessionDiff(ctx context.Context, sessionID string) (map[string]interface{}, error)
	ForkSession(ctx context.Context, sessionID string) (*chat.Session, error)
	RevertSession(ctx context.Context, sessionID string) (map[string]interface{}, error)
	UnrevertSession(ctx context.Context, sessionID string) (map[string]interface{}, error)
	AnswerQuestion(ctx context.Context, questionID string, answers []chat.QuestionAnswer) error
	SetAlertProvider(provider chat.MCPAlertProvider)
	SetFindingsProvider(provider chat.MCPFindingsProvider)
	SetBaselineProvider(provider chat.MCPBaselineProvider)
	SetPatternProvider(provider chat.MCPPatternProvider)
	SetMetricsHistory(provider chat.MCPMetricsHistoryProvider)
	SetAgentProfileManager(manager chat.AgentProfileManager)
	SetStorageProvider(provider chat.MCPStorageProvider)
	SetBackupProvider(provider chat.MCPBackupProvider)
	SetDiskHealthProvider(provider chat.MCPDiskHealthProvider)
	SetUpdatesProvider(provider chat.MCPUpdatesProvider)
	SetFindingsManager(manager chat.FindingsManager)
	SetMetadataUpdater(updater chat.MetadataUpdater)
	SetKnowledgeStoreProvider(provider chat.KnowledgeStoreProvider)
	SetIncidentRecorderProvider(provider chat.IncidentRecorderProvider)
	SetEventCorrelatorProvider(provider chat.EventCorrelatorProvider)
	SetTopologyProvider(provider chat.TopologyProvider)
	UpdateControlSettings(cfg *config.AIConfig)
	GetBaseURL() string
}

// AIHandler handles all AI endpoints using direct AI integration
type AIHandler struct {
	mtPersistence     *config.MultiTenantPersistence
	mtMonitor         *monitoring.MultiTenantMonitor
	legacyConfig      *config.Config
	legacyPersistence AIPersistence
	legacyService     AIService
	agentServer       *agentexec.Server
	services          map[string]AIService
	servicesMu        sync.RWMutex
	stateProviders    map[string]AIStateProvider
	stateProvidersMu  sync.RWMutex
}

// newChatService is the factory function for creating the AI service.
// Can be swapped in tests for mocking.
var newChatService = func(cfg chat.Config) AIService {
	return chat.NewService(cfg)
}

// NewAIHandler creates a new AI handler
func NewAIHandler(mtp *config.MultiTenantPersistence, mtm *monitoring.MultiTenantMonitor, agentServer *agentexec.Server) *AIHandler {
	var defaultConfig *config.Config
	var defaultPersistence AIPersistence

	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil && m != nil {
			defaultConfig = m.GetConfig()
		}
	}
	if mtp != nil {
		if p, err := mtp.GetPersistence("default"); err == nil {
			defaultPersistence = p
		}
	}

	return &AIHandler{
		mtPersistence:     mtp,
		mtMonitor:         mtm,
		legacyConfig:      defaultConfig,
		legacyPersistence: defaultPersistence,
		agentServer:       agentServer,
		services:          make(map[string]AIService),
		stateProviders:    make(map[string]AIStateProvider),
	}
}

// GetService returns the AI service for the current context
func (h *AIHandler) GetService(ctx context.Context) AIService {
	orgID := GetOrgID(ctx)
	if orgID == "default" || orgID == "" {
		return h.legacyService
	}

	h.servicesMu.RLock()
	svc, exists := h.services[orgID]
	h.servicesMu.RUnlock()

	if exists {
		return svc
	}

	h.servicesMu.Lock()
	defer h.servicesMu.Unlock()

	// Double check
	if svc, exists = h.services[orgID]; exists {
		return svc
	}

	// Create and start service for this tenant
	svc = h.initTenantService(ctx, orgID)
	if svc != nil {
		h.services[orgID] = svc
	}
	return svc
}

// RemoveTenantService stops and removes the AI service for a specific tenant.
// This should be called when a tenant is offboarded to free resources.
func (h *AIHandler) RemoveTenantService(ctx context.Context, orgID string) error {
	if orgID == "default" || orgID == "" {
		return nil // Don't remove legacy service
	}

	h.servicesMu.Lock()
	defer h.servicesMu.Unlock()

	svc, exists := h.services[orgID]
	if !exists {
		return nil // Nothing to remove
	}

	if svc != nil {
		if err := svc.Stop(ctx); err != nil {
			log.Warn().Str("orgID", orgID).Err(err).Msg("Error stopping AI service for removed tenant")
		}
	}

	delete(h.services, orgID)
	log.Info().Str("orgID", orgID).Msg("Removed AI service for tenant")
	return nil
}

func (h *AIHandler) initTenantService(ctx context.Context, orgID string) AIService {
	if h.mtPersistence == nil {
		return nil
	}

	persistence, err := h.mtPersistence.GetPersistence(orgID)
	if err != nil {
		log.Warn().Str("orgID", orgID).Err(err).Msg("Failed to get persistence for AI service")
		return nil
	}

	// We need the config to get the data directory
	aiCfg, _ := persistence.LoadAIConfig()

	dataDir := h.getDataDir(aiCfg, persistence.DataDir())

	// Create chat config
	chatCfg := chat.Config{
		AIConfig:    aiCfg,
		DataDir:     dataDir,
		AgentServer: h.agentServer,
	}

	// Get monitor for state provider
	if h.mtMonitor != nil {
		if m, err := h.mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			chatCfg.StateProvider = m
		}
	}

	svc := newChatService(chatCfg)
	if err := svc.Start(ctx); err != nil {
		log.Error().Str("orgID", orgID).Err(err).Msg("Failed to start AI service for tenant")
	}

	return svc
}

func (h *AIHandler) getDataDir(aiCfg *config.AIConfig, baseDir string) string {
	dataDir := baseDir
	if dataDir == "" {
		dataDir = "data"
	}
	return dataDir
}

func (h *AIHandler) getConfig(ctx context.Context) *config.Config {
	orgID := GetOrgID(ctx)
	if h.mtMonitor != nil {
		if m, err := h.mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			return m.GetConfig()
		}
	}
	return h.legacyConfig
}

func (h *AIHandler) getPersistence(ctx context.Context) AIPersistence {
	orgID := GetOrgID(ctx)
	if h.mtPersistence != nil {
		if p, err := h.mtPersistence.GetPersistence(orgID); err == nil {
			return p
		}
	}
	return h.legacyPersistence
}

// loadAIConfig loads AI config for the current context
func (h *AIHandler) loadAIConfig(ctx context.Context) *config.AIConfig {
	p := h.getPersistence(ctx)
	if p == nil {
		return nil
	}
	cfg, err := p.LoadAIConfig()
	if err != nil {
		return nil
	}
	return cfg
}

// SetMultiTenantPersistence updates the persistence manager
func (h *AIHandler) SetMultiTenantPersistence(mtp *config.MultiTenantPersistence) {
	h.mtPersistence = mtp
}

// SetMultiTenantMonitor updates the monitor manager
func (h *AIHandler) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	h.mtMonitor = mtm
}

// StateProvider interface for infrastructure state
type AIStateProvider interface {
	GetState() models.StateSnapshot
}

// Start initializes and starts the AI chat service
func (h *AIHandler) Start(ctx context.Context, stateProvider AIStateProvider) error {
	log.Info().Msg("AIHandler.Start called")
	aiCfg := h.loadAIConfig(ctx)
	if aiCfg == nil {
		log.Info().Msg("AI config is nil, AI is disabled")
		return nil
	}
	if !aiCfg.Enabled {
		log.Info().Bool("enabled", aiCfg.Enabled).Msg("AI is disabled in config")
		return nil
	}

	// Determine data directory
	persistence := h.getPersistence(ctx)
	dataDir := h.getDataDir(aiCfg, persistence.DataDir())

	// Create chat config
	chatCfg := chat.Config{
		AIConfig:      aiCfg,
		DataDir:       dataDir,
		StateProvider: stateProvider,
		AgentServer:   h.agentServer,
	}

	h.legacyService = newChatService(chatCfg)
	if err := h.legacyService.Start(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to start AI chat service")
		return err
	}

	// Initialize approval store for command approval workflow
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

	log.Info().Msg("Pulse AI started (direct integration)")
	return nil
}

// Stop stops the AI chat service
func (h *AIHandler) Stop(ctx context.Context) error {
	if h.legacyService != nil {
		return h.legacyService.Stop(ctx)
	}
	return nil
}

// Restart restarts the AI chat service with updated configuration
// Call this when model or other settings change
func (h *AIHandler) Restart(ctx context.Context) error {
	// Load fresh config from persistence to get latest settings
	newCfg := h.loadAIConfig(ctx)

	if h.legacyService == nil {
		return nil
	}

	if !h.legacyService.IsRunning() {
		// If not running but enabled, try to start
		if newCfg != nil && newCfg.Enabled {
			log.Info().Msg("Starting AI service via restart trigger")

			// We need a state provider to start
			// Try to get default state provider from existing map if available
			var sp AIStateProvider
			h.stateProvidersMu.RLock()
			for _, p := range h.stateProviders {
				sp = p
				break
			}
			h.stateProvidersMu.RUnlock()

			// Reuse start logic
			return h.Start(ctx, sp)
		}
		return nil // Not running and not enabled, nothing to do
	}

	return h.legacyService.Restart(ctx, newCfg)
}

// IsRunning returns whether AI is running
// GetAIConfig returns the current AI configuration
func (h *AIHandler) GetAIConfig(ctx context.Context) *config.AIConfig {
	return h.loadAIConfig(ctx)
}

// IsRunning returns true if the AI chat service is running
func (h *AIHandler) IsRunning(ctx context.Context) bool {
	svc := h.GetService(ctx)
	return svc != nil && svc.IsRunning()
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

	ctx := r.Context()
	if !h.IsRunning(ctx) {
		http.Error(w, "Pulse Assistant is not running", http.StatusServiceUnavailable)
		return
	}
	svc := h.GetService(ctx)
	if svc == nil {
		http.Error(w, "Pulse Assistant service not available", http.StatusServiceUnavailable)
		return
	}

	// Parse request
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	preview := req.Prompt
	if len(preview) > 100 {
		preview = preview[:100] + "..."
	}
	log.Info().
		Str("sessionId", req.SessionID).
		Str("prompt_preview", preview).
		Msg("AIHandler: Received chat request")

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
	writeEvent := func(event chat.StreamEvent) {
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

	// Stream from AI chat service
	err := svc.ExecuteStream(ctx, chat.ExecuteRequest{
		Prompt:    req.Prompt,
		SessionID: req.SessionID,
		Model:     req.Model,
	}, func(event chat.StreamEvent) {
		writeEvent(event)
	})

	if err != nil {
		log.Error().Err(err).Msg("Chat stream error")
		errData, _ := json.Marshal(err.Error())
		writeEvent(chat.StreamEvent{Type: "error", Data: errData})
	}

	// Send done
	writeEvent(chat.StreamEvent{Type: "done", Data: nil})
}

// HandleSessions handles GET /api/ai/sessions - list sessions
func (h *AIHandler) HandleSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.IsRunning(ctx) {
		http.Error(w, "Pulse Assistant is not running", http.StatusServiceUnavailable)
		return
	}

	svc := h.GetService(ctx)
	if svc == nil {
		http.Error(w, "Pulse Assistant service not available", http.StatusServiceUnavailable)
		return
	}

	sessions, err := svc.ListSessions(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// HandleCreateSession handles POST /api/ai/sessions - create session
func (h *AIHandler) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !h.IsRunning(ctx) {
		http.Error(w, "Pulse Assistant is not running", http.StatusServiceUnavailable)
		return
	}

	svc := h.GetService(ctx)
	if svc == nil {
		http.Error(w, "Pulse Assistant service not available", http.StatusServiceUnavailable)
		return
	}

	session, err := svc.CreateSession(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// HandleDeleteSession handles DELETE /api/ai/sessions/{id}
func (h *AIHandler) HandleDeleteSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	ctx := r.Context()
	if !h.IsRunning(ctx) {
		http.Error(w, "Pulse Assistant is not running", http.StatusServiceUnavailable)
		return
	}

	svc := h.GetService(ctx)
	if svc == nil {
		http.Error(w, "Pulse Assistant service not available", http.StatusServiceUnavailable)
		return
	}

	if err := svc.DeleteSession(ctx, sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleMessages handles GET /api/ai/sessions/{id}/messages
func (h *AIHandler) HandleMessages(w http.ResponseWriter, r *http.Request, sessionID string) {
	ctx := r.Context()
	if !h.IsRunning(ctx) {
		http.Error(w, "Pulse Assistant is not running", http.StatusServiceUnavailable)
		return
	}

	svc := h.GetService(ctx)
	if svc == nil {
		http.Error(w, "Pulse Assistant service not available", http.StatusServiceUnavailable)
		return
	}

	messages, err := svc.GetMessages(ctx, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// HandleAbort handles POST /api/ai/sessions/{id}/abort
func (h *AIHandler) HandleAbort(w http.ResponseWriter, r *http.Request, sessionID string) {
	ctx := r.Context()
	if !h.IsRunning(ctx) {
		http.Error(w, "Pulse Assistant is not running", http.StatusServiceUnavailable)
		return
	}

	svc := h.GetService(ctx)
	if svc == nil {
		http.Error(w, "Pulse Assistant service not available", http.StatusServiceUnavailable)
		return
	}

	if err := svc.AbortSession(ctx, sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleStatus handles GET /api/ai/status
func (h *AIHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"running": h.IsRunning(r.Context()),
		"engine":  "direct",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HandleSummarize handles POST /api/ai/sessions/{id}/summarize
// Compresses context when nearing model limits
func (h *AIHandler) HandleSummarize(w http.ResponseWriter, r *http.Request, sessionID string) {
	ctx := r.Context()
	if !h.IsRunning(ctx) {
		http.Error(w, "Pulse Assistant is not running", http.StatusServiceUnavailable)
		return
	}

	svc := h.GetService(ctx)
	if svc == nil {
		http.Error(w, "Pulse Assistant service not available", http.StatusServiceUnavailable)
		return
	}

	result, err := svc.SummarizeSession(ctx, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleDiff handles GET /api/ai/sessions/{id}/diff
// Returns file changes made during the session
func (h *AIHandler) HandleDiff(w http.ResponseWriter, r *http.Request, sessionID string) {
	ctx := r.Context()
	if !h.IsRunning(ctx) {
		http.Error(w, "Pulse Assistant is not running", http.StatusServiceUnavailable)
		return
	}

	svc := h.GetService(ctx)
	if svc == nil {
		http.Error(w, "Pulse Assistant service not available", http.StatusServiceUnavailable)
		return
	}

	diff, err := svc.GetSessionDiff(ctx, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diff)
}

// HandleFork handles POST /api/ai/sessions/{id}/fork
// Creates a branch point in the conversation
func (h *AIHandler) HandleFork(w http.ResponseWriter, r *http.Request, sessionID string) {
	ctx := r.Context()
	if !h.IsRunning(ctx) {
		http.Error(w, "Pulse Assistant is not running", http.StatusServiceUnavailable)
		return
	}

	svc := h.GetService(ctx)
	if svc == nil {
		http.Error(w, "Pulse Assistant service not available", http.StatusServiceUnavailable)
		return
	}

	session, err := svc.ForkSession(ctx, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// HandleRevert handles POST /api/ai/sessions/{id}/revert
// Reverts file changes from the session
func (h *AIHandler) HandleRevert(w http.ResponseWriter, r *http.Request, sessionID string) {
	ctx := r.Context()
	if !h.IsRunning(ctx) {
		http.Error(w, "Pulse Assistant is not running", http.StatusServiceUnavailable)
		return
	}

	svc := h.GetService(ctx)
	if svc == nil {
		http.Error(w, "Pulse Assistant service not available", http.StatusServiceUnavailable)
		return
	}

	result, err := svc.RevertSession(ctx, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleUnrevert handles POST /api/ai/sessions/{id}/unrevert
// Restores previously reverted changes
func (h *AIHandler) HandleUnrevert(w http.ResponseWriter, r *http.Request, sessionID string) {
	ctx := r.Context()
	if !h.IsRunning(ctx) {
		http.Error(w, "Pulse Assistant is not running", http.StatusServiceUnavailable)
		return
	}

	svc := h.GetService(ctx)
	if svc == nil {
		http.Error(w, "Pulse Assistant service not available", http.StatusServiceUnavailable)
		return
	}

	result, err := svc.UnrevertSession(ctx, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
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
	ctx := r.Context()
	if !h.IsRunning(ctx) {
		http.Error(w, "Pulse Assistant is not running", http.StatusServiceUnavailable)
		return
	}

	var req AnswerQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Convert to chat.QuestionAnswer
	answers := make([]chat.QuestionAnswer, len(req.Answers))
	for i, a := range req.Answers {
		answers[i] = chat.QuestionAnswer{
			ID:    a.ID,
			Value: a.Value,
		}
	}

	log.Info().
		Str("questionID", questionID).
		Int("answers_count", len(answers)).
		Msg("AIHandler: Received answer to question")

	svc := h.GetService(ctx)
	if svc == nil {
		http.Error(w, "Pulse Assistant service not available", http.StatusServiceUnavailable)
		return
	}

	if err := svc.AnswerQuestion(ctx, questionID, answers); err != nil {
		log.Error().Err(err).Str("questionID", questionID).Msg("Failed to answer question")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// SetAlertProvider sets the alert provider for MCP tools
func (h *AIHandler) SetAlertProvider(provider chat.MCPAlertProvider) {
	if h.legacyService != nil {
		h.legacyService.SetAlertProvider(provider)
	}
}

// SetFindingsProvider sets the findings provider for MCP tools
func (h *AIHandler) SetFindingsProvider(provider chat.MCPFindingsProvider) {
	if h.legacyService != nil {
		h.legacyService.SetFindingsProvider(provider)
	}
}

// SetBaselineProvider sets the baseline provider for MCP tools
func (h *AIHandler) SetBaselineProvider(provider chat.MCPBaselineProvider) {
	if h.legacyService != nil {
		h.legacyService.SetBaselineProvider(provider)
	}
}

// SetPatternProvider sets the pattern provider for MCP tools
func (h *AIHandler) SetPatternProvider(provider chat.MCPPatternProvider) {
	if h.legacyService != nil {
		h.legacyService.SetPatternProvider(provider)
	}
}

// SetMetricsHistory sets the metrics history provider for MCP tools
func (h *AIHandler) SetMetricsHistory(provider chat.MCPMetricsHistoryProvider) {
	if h.legacyService != nil {
		h.legacyService.SetMetricsHistory(provider)
	}
}

// SetAgentProfileManager sets the agent profile manager for MCP tools
func (h *AIHandler) SetAgentProfileManager(manager chat.AgentProfileManager) {
	if h.legacyService != nil {
		h.legacyService.SetAgentProfileManager(manager)
	}
}

// SetStorageProvider sets the storage provider for MCP tools
func (h *AIHandler) SetStorageProvider(provider chat.MCPStorageProvider) {
	if h.legacyService != nil {
		h.legacyService.SetStorageProvider(provider)
	}
}

// SetBackupProvider sets the backup provider for MCP tools
func (h *AIHandler) SetBackupProvider(provider chat.MCPBackupProvider) {
	if h.legacyService != nil {
		h.legacyService.SetBackupProvider(provider)
	}
}

// SetDiskHealthProvider sets the disk health provider for MCP tools
func (h *AIHandler) SetDiskHealthProvider(provider chat.MCPDiskHealthProvider) {
	if h.legacyService != nil {
		h.legacyService.SetDiskHealthProvider(provider)
	}
}

// SetUpdatesProvider sets the updates provider for MCP tools
func (h *AIHandler) SetUpdatesProvider(provider chat.MCPUpdatesProvider) {
	if h.legacyService != nil {
		h.legacyService.SetUpdatesProvider(provider)
	}
}

// SetFindingsManager sets the findings manager for MCP tools
func (h *AIHandler) SetFindingsManager(manager chat.FindingsManager) {
	if h.legacyService != nil {
		h.legacyService.SetFindingsManager(manager)
	}
}

// SetMetadataUpdater sets the metadata updater for MCP tools
func (h *AIHandler) SetMetadataUpdater(updater chat.MetadataUpdater) {
	if h.legacyService != nil {
		h.legacyService.SetMetadataUpdater(updater)
	}
}

// UpdateControlSettings updates control settings in the service
func (h *AIHandler) UpdateControlSettings(cfg *config.AIConfig) {
	if h.legacyService != nil {
		h.legacyService.UpdateControlSettings(cfg)
	}
}
