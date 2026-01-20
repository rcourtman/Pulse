package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// StateProvider provides access to infrastructure state
type StateProvider interface {
	GetState() models.StateSnapshot
}

// CommandPolicy evaluates command security
type CommandPolicy interface {
	Evaluate(command string) agentexec.PolicyDecision
}

// AgentServer executes commands on agents
type AgentServer interface {
	GetConnectedAgents() []agentexec.ConnectedAgent
	ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error)
}

// MCP provider type aliases for external use
type (
	MCPAlertProvider          = tools.AlertProvider
	MCPFindingsProvider       = tools.FindingsProvider
	MCPBaselineProvider       = tools.BaselineProvider
	MCPPatternProvider        = tools.PatternProvider
	MCPMetricsHistoryProvider = tools.MetricsHistoryProvider
	MCPBackupProvider         = tools.BackupProvider
	MCPStorageProvider        = tools.StorageProvider
	MCPDiskHealthProvider     = tools.DiskHealthProvider
	MCPUpdatesProvider        = tools.UpdatesProvider
	AgentProfileManager       = tools.AgentProfileManager
	FindingsManager           = tools.FindingsManager
	MetadataUpdater           = tools.MetadataUpdater
)

// Config holds service configuration
type Config struct {
	AIConfig      *config.AIConfig
	StateProvider StateProvider
	Policy        CommandPolicy
	AgentServer   AgentServer
	DataDir       string
}

// Service provides direct AI chat without external sidecar
type Service struct {
	mu sync.RWMutex

	cfg           *config.AIConfig
	stateProvider StateProvider
	agentServer   AgentServer
	executor      *tools.PulseToolExecutor
	sessions      *SessionStore
	agenticLoop   *AgenticLoop
	provider      providers.StreamingProvider
	started       bool
}

// NewService creates a new chat service
func NewService(cfg Config) *Service {
	// Create tool executor
	var stateProvider tools.StateProvider
	var policy tools.CommandPolicy
	var agentServer tools.AgentServer

	if cfg.StateProvider != nil {
		stateProvider = &stateProviderAdapter{cfg.StateProvider}
	}
	if cfg.Policy != nil {
		policy = &commandPolicyAdapter{cfg.Policy}
	}
	if cfg.AgentServer != nil {
		agentServer = &agentServerAdapter{cfg.AgentServer}
	}

	execCfg := tools.ExecutorConfig{
		StateProvider: stateProvider,
		Policy:        policy,
		AgentServer:   agentServer,
	}

	if cfg.AIConfig != nil {
		execCfg.ControlLevel = tools.ControlLevel(cfg.AIConfig.GetControlLevel())
		execCfg.ProtectedGuests = cfg.AIConfig.GetProtectedGuests()
	}

	executor := tools.NewPulseToolExecutor(execCfg)

	return &Service{
		cfg:           cfg.AIConfig,
		stateProvider: cfg.StateProvider,
		agentServer:   cfg.AgentServer,
		executor:      executor,
	}
}

// Adapter types
type stateProviderAdapter struct {
	sp StateProvider
}

func (a *stateProviderAdapter) GetState() models.StateSnapshot {
	return a.sp.GetState()
}

type commandPolicyAdapter struct {
	p CommandPolicy
}

func (a *commandPolicyAdapter) Evaluate(command string) agentexec.PolicyDecision {
	return a.p.Evaluate(command)
}

type agentServerAdapter struct {
	s AgentServer
}

func (a *agentServerAdapter) GetConnectedAgents() []agentexec.ConnectedAgent {
	return a.s.GetConnectedAgents()
}

func (a *agentServerAdapter) ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	return a.s.ExecuteCommand(ctx, agentID, cmd)
}

// Start initializes the service
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	if s.cfg == nil {
		return fmt.Errorf("AI config is nil")
	}

	s.applyChatContextSettings()

	// Create session store
	dataDir := s.cfg.OpenCodeDataDir
	if dataDir == "" {
		dataDir = "/tmp/pulse-ai"
	}

	store, err := NewSessionStore(dataDir)
	if err != nil {
		return fmt.Errorf("failed to create session store: %w", err)
	}
	s.sessions = store

	// Create provider
	provider, err := s.createProvider()
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create agentic loop
	systemPrompt := s.buildSystemPrompt()
	s.agenticLoop = NewAgenticLoop(provider, s.executor, systemPrompt)
	s.provider = provider

	s.started = true
	log.Info().
		Str("model", s.cfg.GetChatModel()).
		Str("data_dir", dataDir).
		Msg("Pulse AI (direct) started")

	return nil
}

// Stop stops the service
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.started = false
	s.provider = nil
	log.Info().Msg("Pulse AI (direct) stopped")
	return nil
}

// Restart restarts the service with new config
func (s *Service) Restart(ctx context.Context, newCfg *config.AIConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return fmt.Errorf("service not started")
	}

	if newCfg != nil {
		s.cfg = newCfg
	}

	s.applyChatContextSettings()

	// Update executor settings
	if s.executor != nil && s.cfg != nil {
		s.executor.SetControlLevel(tools.ControlLevel(s.cfg.GetControlLevel()))
		s.executor.SetProtectedGuests(s.cfg.GetProtectedGuests())
	}

	// Recreate provider with new settings
	provider, err := s.createProvider()
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Update agentic loop
	systemPrompt := s.buildSystemPrompt()
	s.agenticLoop = NewAgenticLoop(provider, s.executor, systemPrompt)
	s.provider = provider

	log.Info().
		Str("model", s.cfg.GetChatModel()).
		Msg("Pulse AI restarted with new config")

	return nil
}

// IsRunning returns whether the service is running
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}

// ExecuteStream sends a prompt and streams the response
func (s *Service) ExecuteStream(ctx context.Context, req ExecuteRequest, callback StreamCallback) error {
	s.mu.RLock()
	if !s.started {
		s.mu.RUnlock()
		return fmt.Errorf("service not started")
	}
	sessions := s.sessions
	agenticLoop := s.agenticLoop
	s.mu.RUnlock()

	// Ensure session exists
	session, err := sessions.EnsureSession(req.SessionID)
	if err != nil {
		return fmt.Errorf("failed to ensure session: %w", err)
	}

	// Add user message
	userMsg := Message{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   req.Prompt,
		Timestamp: time.Now(),
	}
	if err := sessions.AddMessage(session.ID, userMsg); err != nil {
		log.Warn().Err(err).Msg("Failed to save user message")
	}

	// Get existing messages for context
	messages, err := sessions.GetMessages(session.ID)
	if err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	// Run agentic loop
	filteredTools := s.filterToolsForPrompt(ctx, req.Prompt)
	resultMessages, err := agenticLoop.ExecuteWithTools(ctx, session.ID, messages, filteredTools, callback)
	if err != nil {
		// Still save any messages we got
		for _, msg := range resultMessages {
			if saveErr := sessions.AddMessage(session.ID, msg); saveErr != nil {
				log.Warn().Err(saveErr).Msg("Failed to save message after error")
			}
		}
		return err
	}

	// Save result messages
	for _, msg := range resultMessages {
		// Skip user messages (already saved)
		if msg.Role == "user" && msg.ToolResult == nil {
			continue
		}
		if err := sessions.AddMessage(session.ID, msg); err != nil {
			log.Warn().Err(err).Msg("Failed to save message")
		}
	}

	// Send done event
	doneData, _ := json.Marshal(DoneData{SessionID: session.ID})
	callback(StreamEvent{Type: "done", Data: doneData})

	return nil
}

// ListSessions returns all sessions
func (s *Service) ListSessions(ctx context.Context) ([]Session, error) {
	s.mu.RLock()
	sessions := s.sessions
	s.mu.RUnlock()

	if sessions == nil {
		return nil, fmt.Errorf("service not started")
	}

	return sessions.List()
}

// CreateSession creates a new session
func (s *Service) CreateSession(ctx context.Context) (*Session, error) {
	s.mu.RLock()
	sessions := s.sessions
	s.mu.RUnlock()

	if sessions == nil {
		return nil, fmt.Errorf("service not started")
	}

	return sessions.Create()
}

// DeleteSession deletes a session
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	s.mu.RLock()
	sessions := s.sessions
	s.mu.RUnlock()

	if sessions == nil {
		return fmt.Errorf("service not started")
	}

	return sessions.Delete(sessionID)
}

// GetMessages returns messages for a session
func (s *Service) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	s.mu.RLock()
	sessions := s.sessions
	s.mu.RUnlock()

	if sessions == nil {
		return nil, fmt.Errorf("service not started")
	}

	return sessions.GetMessages(sessionID)
}

// AbortSession aborts an ongoing session
func (s *Service) AbortSession(ctx context.Context, sessionID string) error {
	s.mu.RLock()
	agenticLoop := s.agenticLoop
	s.mu.RUnlock()

	if agenticLoop == nil {
		return fmt.Errorf("service not started")
	}

	agenticLoop.Abort(sessionID)
	return nil
}

// AnswerQuestion provides answers to a pending question
func (s *Service) AnswerQuestion(ctx context.Context, questionID string, answers []QuestionAnswer) error {
	s.mu.RLock()
	agenticLoop := s.agenticLoop
	s.mu.RUnlock()

	if agenticLoop == nil {
		return fmt.Errorf("service not started")
	}

	return agenticLoop.AnswerQuestion(questionID, answers)
}

// Stub methods for features not yet implemented

// SummarizeSession compresses context
func (s *Service) SummarizeSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "not_implemented"}, nil
}

// GetSessionDiff returns file changes
func (s *Service) GetSessionDiff(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "not_implemented"}, nil
}

// ForkSession creates a branch
func (s *Service) ForkSession(ctx context.Context, sessionID string) (*Session, error) {
	return nil, fmt.Errorf("not implemented")
}

// RevertSession reverts changes
func (s *Service) RevertSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "not_implemented"}, nil
}

// UnrevertSession restores reverted changes
func (s *Service) UnrevertSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "not_implemented"}, nil
}

// GetBaseURL returns empty since there's no sidecar
func (s *Service) GetBaseURL() string {
	return ""
}

// Provider setter methods

func (s *Service) SetAlertProvider(provider MCPAlertProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetAlertProvider(provider)
	}
}

func (s *Service) SetFindingsProvider(provider MCPFindingsProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetFindingsProvider(provider)
	}
}

func (s *Service) SetBaselineProvider(provider MCPBaselineProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetBaselineProvider(provider)
	}
}

func (s *Service) SetPatternProvider(provider MCPPatternProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetPatternProvider(provider)
	}
}

func (s *Service) SetMetricsHistory(provider MCPMetricsHistoryProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetMetricsHistory(provider)
	}
}

func (s *Service) SetBackupProvider(provider MCPBackupProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetBackupProvider(provider)
	}
}

func (s *Service) SetStorageProvider(provider MCPStorageProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetStorageProvider(provider)
	}
}

func (s *Service) SetDiskHealthProvider(provider MCPDiskHealthProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetDiskHealthProvider(provider)
	}
}

func (s *Service) SetUpdatesProvider(provider MCPUpdatesProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetUpdatesProvider(provider)
	}
}

func (s *Service) SetAgentProfileManager(manager AgentProfileManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetAgentProfileManager(manager)
	}
}

func (s *Service) SetFindingsManager(manager FindingsManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetFindingsManager(manager)
	}
}

func (s *Service) SetMetadataUpdater(updater MetadataUpdater) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetMetadataUpdater(updater)
	}
}

func (s *Service) UpdateControlSettings(cfg *config.AIConfig) {
	if cfg == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetControlLevel(tools.ControlLevel(cfg.GetControlLevel()))
		s.executor.SetProtectedGuests(cfg.GetProtectedGuests())
	}
}

// createProvider creates an AI provider based on config
func (s *Service) createProvider() (providers.StreamingProvider, error) {
	if s.cfg == nil {
		return nil, fmt.Errorf("no AI config")
	}

	chatModel := s.cfg.GetChatModel()
	if chatModel == "" {
		return nil, fmt.Errorf("no chat model configured")
	}

	// Parse provider:model format
	parts := strings.SplitN(chatModel, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid model format: %s (expected provider:model)", chatModel)
	}
	providerName := parts[0]
	modelName := parts[1]

	timeout := 5 * time.Minute

	switch providerName {
	case "anthropic":
		if s.cfg.AnthropicAPIKey == "" {
			return nil, fmt.Errorf("Anthropic API key not configured")
		}
		return providers.NewAnthropicClient(s.cfg.AnthropicAPIKey, modelName, timeout), nil

	case "openai":
		if s.cfg.OpenAIAPIKey == "" {
			return nil, fmt.Errorf("OpenAI API key not configured")
		}
		return providers.NewOpenAIClient(s.cfg.OpenAIAPIKey, modelName, "", timeout), nil

	case "deepseek":
		if s.cfg.DeepSeekAPIKey == "" {
			return nil, fmt.Errorf("DeepSeek API key not configured")
		}
		return providers.NewOpenAIClient(s.cfg.DeepSeekAPIKey, modelName, "https://api.deepseek.com", timeout), nil

	case "gemini":
		if s.cfg.GeminiAPIKey == "" {
			return nil, fmt.Errorf("Gemini API key not configured")
		}
		return providers.NewGeminiClient(s.cfg.GeminiAPIKey, modelName, "", timeout), nil

	case "ollama":
		baseURL := s.cfg.OllamaBaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return providers.NewOllamaClient(modelName, baseURL, timeout), nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}

func (s *Service) applyChatContextSettings() {
	StatelessContext = DefaultStatelessContext
}

// buildSystemPrompt builds the system prompt for the AI
func (s *Service) buildSystemPrompt() string {
	return `You are Pulse AI, an intelligent assistant for infrastructure monitoring and management.

You have access to tools that let you:
- Query infrastructure state (VMs, containers, nodes) from Pulse monitoring
- Get metrics and performance data
- Check alerts and findings
- Execute commands on hosts via connected agents (with approval)
- Manage Docker containers
- Update resource metadata

Prefer the most targeted tool and filters to keep context small (use pulse_search_resources or pulse_list_infrastructure before full topology).

When users ask about their infrastructure:
1. Use monitoring/query tools first (list/search/topology/alerts/metrics)
2. Ask a clarifying question if the target host/resource or time range is unclear
3. Only run commands when monitoring data is insufficient or the user explicitly asks; scope to a single host/agent
4. Suggest actions when appropriate and explain control actions before executing

Be concise but thorough. Focus on actionable information.
Trust the data from your tools - it's updated in real-time.`
}

type runCommandDecision struct {
	Allow  bool   `json:"allow_run_command"`
	Reason string `json:"reason,omitempty"`
}

const runCommandClassifierPrompt = `You are a routing classifier for Pulse AI chat.
Decide whether the user is explicitly asking to execute a command or log into a machine.
If the user is asking for status, explanations, or what a command does, return false.
Return only JSON: {"allow_run_command": true|false, "reason": "short"}.`

func (s *Service) filterToolsForPrompt(ctx context.Context, prompt string) []providers.Tool {
	mcpTools := s.executor.ListTools()
	providerTools := ConvertMCPToolsToProvider(mcpTools)

	allow, err := s.classifyRunCommand(ctx, prompt)
	if err != nil {
		log.Warn().Err(err).Msg("Run command routing failed")
		return filterOutRunCommand(providerTools)
	}
	if allow {
		return providerTools
	}
	return filterOutRunCommand(providerTools)
}

func (s *Service) classifyRunCommand(ctx context.Context, prompt string) (bool, error) {
	if s.provider == nil {
		return false, fmt.Errorf("provider not available")
	}

	req := providers.ChatRequest{
		Messages:    []providers.Message{{Role: "user", Content: prompt}},
		System:      runCommandClassifierPrompt,
		MaxTokens:   80,
		Temperature: 0,
	}
	resp, err := s.provider.Chat(ctx, req)
	if err != nil {
		return false, err
	}
	decision, err := parseRunCommandDecision(resp.Content)
	if err != nil {
		return false, err
	}
	return decision.Allow, nil
}

func parseRunCommandDecision(text string) (runCommandDecision, error) {
	var decision runCommandDecision
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return decision, fmt.Errorf("empty response")
	}

	if strings.EqualFold(trimmed, "true") {
		decision.Allow = true
		return decision, nil
	}
	if strings.EqualFold(trimmed, "false") {
		decision.Allow = false
		return decision, nil
	}

	if err := json.Unmarshal([]byte(trimmed), &decision); err == nil {
		if strings.Contains(trimmed, "allow_run_command") {
			return decision, nil
		}
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		candidate := trimmed[start : end+1]
		if err := json.Unmarshal([]byte(candidate), &decision); err == nil {
			if strings.Contains(candidate, "allow_run_command") {
				return decision, nil
			}
		}
	}

	return decision, fmt.Errorf("unable to parse decision")
}

func filterOutRunCommand(tools []providers.Tool) []providers.Tool {
	filtered := make([]providers.Tool, 0, len(tools))
	for _, tool := range tools {
		if tool.Name == "pulse_run_command" {
			continue
		}
		filtered = append(filtered, tool)
	}
	return filtered
}

// Execute provides non-streaming execution (for compatibility)
func (s *Service) Execute(ctx context.Context, req ExecuteRequest) (map[string]interface{}, error) {
	var lastContent string
	err := s.ExecuteStream(ctx, req, func(event StreamEvent) {
		if event.Type == "content" {
			var data ContentData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				lastContent += data.Text
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"content": lastContent,
	}, nil
}
