package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
	MCPAlertProvider           = tools.AlertProvider
	MCPFindingsProvider        = tools.FindingsProvider
	MCPBaselineProvider        = tools.BaselineProvider
	MCPPatternProvider         = tools.PatternProvider
	MCPMetricsHistoryProvider  = tools.MetricsHistoryProvider
	MCPBackupProvider          = tools.BackupProvider
	MCPGuestConfigProvider     = tools.GuestConfigProvider
	MCPDiskHealthProvider      = tools.DiskHealthProvider
	MCPUpdatesProvider         = tools.UpdatesProvider
	AgentProfileManager        = tools.AgentProfileManager
	FindingsManager            = tools.FindingsManager
	MetadataUpdater            = tools.MetadataUpdater
	IncidentRecorderProvider   = tools.IncidentRecorderProvider
	EventCorrelatorProvider    = tools.EventCorrelatorProvider
	TopologyProvider           = tools.TopologyProvider
	KnowledgeStoreProvider     = tools.KnowledgeStoreProvider
	MCPDiscoveryProvider       = tools.DiscoveryProvider
	MCPUnifiedResourceProvider = tools.UnifiedResourceProvider
)

// Config holds service configuration
type Config struct {
	AIConfig      *config.AIConfig
	StateProvider StateProvider
	ReadState     unifiedresources.ReadState
	Policy        CommandPolicy
	AgentServer   AgentServer
	DataDir       string
}

// Service provides direct AI chat without external sidecar
type Service struct {
	mu sync.RWMutex

	cfg               *config.AIConfig
	dataDir           string
	stateProvider     StateProvider
	readState         unifiedresources.ReadState
	agentServer       AgentServer
	executor          *tools.PulseToolExecutor
	sessions          *SessionStore
	agenticLoop       *AgenticLoop
	provider          providers.StreamingProvider
	providerFactory   func(modelStr string) (providers.StreamingProvider, error)
	started           bool
	autonomousMode    bool
	contextPrefetcher *ContextPrefetcher
	budgetChecker     func() error // Optional mid-run budget enforcement
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
		ReadState:     cfg.ReadState,
		Policy:        policy,
		AgentServer:   agentServer,
	}

	if cfg.AIConfig != nil {
		execCfg.ControlLevel = tools.ControlLevel(cfg.AIConfig.GetControlLevel())
		execCfg.ProtectedGuests = cfg.AIConfig.GetProtectedGuests()
	}

	executor := tools.NewPulseToolExecutor(execCfg)

	// Set telemetry callback for strict resolution metrics
	executor.SetTelemetryCallback(NewAIMetricsTelemetryCallback())

	return &Service{
		cfg:           cfg.AIConfig,
		dataDir:       cfg.DataDir,
		stateProvider: cfg.StateProvider,
		readState:     cfg.ReadState,
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
		return fmt.Errorf("Pulse Assistant config is nil")
	}

	s.applyChatContextSettings()

	// Create session store
	dataDir := s.dataDir
	if dataDir == "" {
		dataDir = "/tmp/pulse-ai"
		s.dataDir = dataDir
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
	log.Debug().
		Str("session_id", req.SessionID).
		Int("prompt_len", len(req.Prompt)).
		Msg("[ChatService] ExecuteStream called")

	s.mu.RLock()
	if !s.started {
		s.mu.RUnlock()
		log.Error().Msg("[ChatService] Service not started")
		return fmt.Errorf("service not started")
	}
	sessions := s.sessions
	agenticLoop := s.agenticLoop
	s.mu.RUnlock()

	log.Debug().
		Str("session_id", req.SessionID).
		Bool("has_sessions", sessions != nil).
		Bool("has_agentic_loop", agenticLoop != nil).
		Msg("[ChatService] Retrieved internal state")

	// Ensure session exists
	session, err := sessions.EnsureSession(req.SessionID)
	if err != nil {
		log.Error().Err(err).Str("session_id", req.SessionID).Msg("[ChatService] Failed to ensure session")
		return fmt.Errorf("failed to ensure session: %w", err)
	}

	log.Debug().Str("session_id", session.ID).Msg("[ChatService] Session ensured")

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
		log.Error().Err(err).Str("session_id", session.ID).Msg("[ChatService] Failed to get messages")
		return fmt.Errorf("failed to get messages: %w", err)
	}

	log.Debug().
		Str("session_id", session.ID).
		Int("message_count", len(messages)).
		Msg("[ChatService] Got messages, calling agentic loop")

	// Determine which model/loop to use for this request.
	selectedModel := ""
	configuredModel := ""
	overrideModel := strings.TrimSpace(req.Model)
	var executor *tools.PulseToolExecutor
	autonomousMode := false
	s.mu.RLock()
	executor = s.executor
	autonomousMode = s.autonomousMode
	if s.cfg != nil {
		configuredModel = strings.TrimSpace(s.cfg.GetChatModel())
	}
	s.mu.RUnlock()

	// Per-request autonomous mode override (used by investigation to avoid
	// mutating shared service state from concurrent goroutines).
	if req.AutonomousMode != nil {
		autonomousMode = *req.AutonomousMode
	}
	selectedModel = configuredModel
	if overrideModel != "" {
		selectedModel = overrideModel
	}
	// Create a per-request AgenticLoop to ensure complete isolation between
	// concurrent sessions. This prevents race conditions where concurrent
	// ExecuteStream calls would overwrite each other's FSM, knowledge accumulator,
	// autonomous mode, budget checker, and provider info on a shared loop.
	var loop *AgenticLoop
	if overrideModel != "" && overrideModel != configuredModel {
		provider, err := s.createProviderForModel(overrideModel)
		if err != nil {
			return fmt.Errorf("failed to create provider for model override %q: %w", overrideModel, err)
		}
		systemPrompt := s.buildSystemPrompt()
		loop = NewAgenticLoop(provider, executor, systemPrompt)
	} else {
		// Create a fresh loop with the configured provider for this request
		s.mu.RLock()
		provider := s.provider
		s.mu.RUnlock()
		if provider == nil {
			return fmt.Errorf("provider not initialized")
		}
		systemPrompt := s.buildSystemPrompt()
		loop = NewAgenticLoop(provider, executor, systemPrompt)
	}
	loop.SetAutonomousMode(autonomousMode)

	// Proactively gather context for mentioned resources
	s.mu.RLock()
	prefetcher := s.contextPrefetcher
	s.mu.RUnlock()

	log.Info().
		Bool("hasPrefetcher", prefetcher != nil).
		Str("prompt", req.Prompt[:min(50, len(req.Prompt))]).
		Msg("[ChatService] Checking prefetcher")

	mentionsFound := false
	if prefetcher != nil {
		prefetchCtx := prefetcher.Prefetch(ctx, req.Prompt, req.Mentions)
		if prefetchCtx != nil {
			mentionsFound = len(prefetchCtx.Mentions) > 0
		}
		if prefetchCtx != nil && prefetchCtx.Summary != "" {
			log.Info().
				Int("mentions", len(prefetchCtx.Mentions)).
				Int("discoveries", len(prefetchCtx.Discoveries)).
				Msg("[ChatService] Injecting prefetched context")

			// Mark mentioned resources as explicitly accessed for routing validation
			// This ensures that if user says "@homepage-docker" and model targets the host,
			// the routing validation will catch it.
			resolvedCtx := sessions.GetResolvedContext(session.ID)
			for _, mention := range prefetchCtx.Mentions {
				// Build canonical resource ID: kind:host:id
				var resourceID string
				switch mention.ResourceType {
				case "lxc":
					if mention.HostID != "" {
						resourceID = "lxc:" + mention.HostID + ":" + mention.ResourceID
					}
				case "vm":
					if mention.HostID != "" {
						resourceID = "vm:" + mention.HostID + ":" + mention.ResourceID
					}
				case "docker":
					if mention.TargetHost != "" {
						resourceID = "docker_container:" + mention.TargetHost + ":" + mention.ResourceID
					}
				}
				if resourceID != "" {
					resolvedCtx.MarkExplicitAccess(resourceID)
					log.Debug().
						Str("resource_id", resourceID).
						Str("mention", mention.Name).
						Msg("[ChatService] Marked @mention as explicit access")
				}
			}

			// Augment the user's message with prefetched context
			// This is more reliable than a separate system message - the AI treats it as authoritative user-provided info
			if len(messages) > 0 {
				lastIdx := len(messages) - 1
				if messages[lastIdx].Role == "user" {
					augmentedContent := prefetchCtx.Summary + "\n\n---\nUser question: " + messages[lastIdx].Content
					messages[lastIdx].Content = augmentedContent
				}
			}
		}

		if !mentionsFound {
			s.injectRecentContextIfNeeded(req.Prompt, session.ID, messages, sessions)
		}
	} else {
		s.injectRecentContextIfNeeded(req.Prompt, session.ID, messages, sessions)
	}

	// Set session-scoped resolved context on executor for resource validation.
	// This ensures tools can only operate on resources discovered in this session.
	if executor != nil {
		resolvedCtx := sessions.GetResolvedContext(session.ID)
		executor.SetResolvedContext(resolvedCtx)
		log.Debug().
			Str("session_id", session.ID).
			Int("resolved_resources", len(resolvedCtx.Resources)).
			Msg("[ChatService] Set resolved context on executor")
	}

	// Shared session state for pre-pass + main loop.
	sessionFSM := sessions.GetSessionFSM(session.ID)
	ka := sessions.GetKnowledgeAccumulator(session.ID)

	// If the prefetcher resolved mentions, advance FSM past RESOLVING.
	// The prefetched context already contains the resource details (type, VMID, node, host)
	// so forcing the AI to redundantly call a read tool would be wasteful.
	if mentionsFound && sessionFSM.State == StateResolving {
		sessionFSM.State = StateReading
		log.Info().
			Str("session_id", session.ID).
			Msg("[ChatService] Advanced FSM to READING — prefetched mentions count as resolution")
	}

	// Explore pre-pass (interactive chat only): run a short read-only scout step
	// and inject its findings into the main loop context.
	if s.shouldRunExplore(autonomousMode) {
		exploreResult := s.runExplorePrepass(
			ctx,
			session.ID,
			req.Prompt,
			overrideModel,
			selectedModel,
			messages,
			executor,
			loop.provider,
			callback,
		)
		if exploreResult.Summary != "" {
			injectExploreSummaryIntoLatestUserMessage(messages, exploreResult.Summary, exploreResult.Model)
			log.Info().
				Str("session_id", session.ID).
				Str("outcome", exploreResult.Outcome).
				Str("model", exploreResult.Model).
				Int("summary_len", len(exploreResult.Summary)).
				Int("input_tokens", exploreResult.InputTokens).
				Int("output_tokens", exploreResult.OutputTokens).
				Dur("duration", exploreResult.Duration).
				Msg("[ChatService] Explore pre-pass completed")
		} else {
			log.Debug().
				Str("session_id", session.ID).
				Str("outcome", exploreResult.Outcome).
				Str("model", exploreResult.Model).
				Dur("duration", exploreResult.Duration).
				Msg("[ChatService] Explore pre-pass skipped or produced no summary")
		}
	}

	// Run agentic loop
	filteredTools := s.filterToolsForPrompt(ctx, req.Prompt, autonomousMode)
	log.Debug().
		Str("session_id", session.ID).
		Int("tools_count", len(filteredTools)).
		Msg("[ChatService] Filtered tools, starting agentic loop")

	// Set session-scoped FSM on agentic loop for workflow enforcement
	// This ensures structural guarantees: discover before write, verify after write
	loop.SetSessionFSM(sessionFSM)

	// Set session-scoped knowledge accumulator for fact extraction across turns.
	// For user chat, this persists across messages so facts accumulate during a conversation.
	loop.SetKnowledgeAccumulator(ka)

	log.Debug().
		Str("session_id", session.ID).
		Str("fsm_state", string(sessionFSM.State)).
		Bool("wrote_this_episode", sessionFSM.WroteThisEpisode).
		Msg("[ChatService] Set session FSM on agentic loop")

	// Set mid-run budget checker if configured
	if s.budgetChecker != nil {
		loop.SetBudgetChecker(s.budgetChecker)
	}

	// Set provider info for telemetry
	if selectedModel != "" {
		parts := strings.SplitN(selectedModel, ":", 2)
		if len(parts) == 2 {
			loop.SetProviderInfo(parts[0], parts[1])
		}
	}

	// Override max turns if the caller specified one (e.g. investigations use 15).
	// Reset after the call to avoid affecting concurrent sessions on the shared loop.
	if req.MaxTurns > 0 {
		loop.SetMaxTurns(req.MaxTurns)
		defer loop.SetMaxTurns(MaxAgenticTurns)
	}

	// Apply per-request autonomous mode to the loop. For investigation requests
	// with AutonomousMode set, this uses the per-request value instead of
	// mutating shared service state from concurrent goroutines.
	loop.SetAutonomousMode(autonomousMode)

	resultMessages, err := loop.ExecuteWithTools(ctx, session.ID, messages, filteredTools, callback)

	log.Debug().
		Str("session_id", session.ID).
		Int("result_messages", len(resultMessages)).
		Err(err).
		Msg("[ChatService] Agentic loop returned")

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

	// Send done event with token usage for this request.
	doneData, _ := json.Marshal(DoneData{
		SessionID:    session.ID,
		InputTokens:  loop.GetTotalInputTokens(),
		OutputTokens: loop.GetTotalOutputTokens(),
	})
	callback(StreamEvent{Type: "done", Data: doneData})

	return nil
}

// PatrolRequest represents a patrol execution request within the chat service
type PatrolRequest struct {
	Prompt       string `json:"prompt"`
	SystemPrompt string `json:"system_prompt"`
	SessionID    string `json:"session_id,omitempty"`
	UseCase      string `json:"use_case"`
	MaxTurns     int    `json:"max_turns,omitempty"`
}

// PatrolResponse contains the results of a patrol execution
type PatrolResponse struct {
	Content      string `json:"content"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

// ExecutePatrolStream creates a temporary agentic loop for patrol execution.
// Unlike ExecuteStream (which uses the shared agentic loop with the chat system prompt),
// this creates an isolated loop with the patrol's own system prompt and model.
func (s *Service) ExecutePatrolStream(ctx context.Context, req PatrolRequest, callback StreamCallback) (*PatrolResponse, error) {
	log.Debug().
		Str("session_id", req.SessionID).
		Int("prompt_len", len(req.Prompt)).
		Msg("[ChatService] ExecutePatrolStream called")

	s.mu.RLock()
	if !s.started {
		s.mu.RUnlock()
		return nil, fmt.Errorf("service not started")
	}
	sessions := s.sessions
	executor := s.executor
	cfg := s.cfg
	s.mu.RUnlock()

	// Determine model: use patrol model or fall back to chat model
	patrolModel := ""
	if cfg != nil {
		patrolModel = cfg.GetPatrolModel()
		if patrolModel == "" {
			patrolModel = cfg.GetChatModel()
		}
	}
	if patrolModel == "" {
		return nil, fmt.Errorf("no patrol model configured")
	}

	// Create a temporary provider for the patrol model
	provider, err := s.createProviderForModel(patrolModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create patrol provider: %w", err)
	}

	// Create a temporary agentic loop with the patrol system prompt
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = s.buildSystemPrompt()
	}
	tempLoop := NewAgenticLoop(provider, executor, systemPrompt)
	tempLoop.SetAutonomousMode(true) // Patrol runs without approval prompts
	if req.MaxTurns > 0 {
		tempLoop.SetMaxTurns(req.MaxTurns)
	}

	// Set provider info for telemetry
	parts := strings.SplitN(patrolModel, ":", 2)
	if len(parts) == 2 {
		tempLoop.SetProviderInfo(parts[0], parts[1])
	}

	// Ensure patrol session exists
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = "patrol-main"
	}
	session, err := sessions.EnsureSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure patrol session: %w", err)
	}

	// Set resolved context on executor (same as ExecuteStream does)
	if executor != nil {
		resolvedCtx := sessions.GetResolvedContext(session.ID)
		executor.SetResolvedContext(resolvedCtx)
	}

	// Set session FSM
	sessionFSM := sessions.GetSessionFSM(session.ID)
	tempLoop.SetSessionFSM(sessionFSM)

	// Create a fresh knowledge accumulator for this patrol run.
	// Unlike user chat (which reuses session-scoped KA across messages),
	// patrol runs need a clean slate to avoid stale facts from prior runs
	// (patrol-main reuses the same session ID across scheduled runs).
	ka := sessions.NewKnowledgeAccumulatorForRun(session.ID)
	tempLoop.SetKnowledgeAccumulator(ka)

	// Set mid-run budget checker if configured
	if s.budgetChecker != nil {
		tempLoop.SetBudgetChecker(s.budgetChecker)
	}

	// Add user message
	userMsg := Message{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   req.Prompt,
		Timestamp: time.Now(),
	}
	if err := sessions.AddMessage(session.ID, userMsg); err != nil {
		log.Warn().Err(err).Msg("Failed to save patrol user message")
	}

	// Get messages for context
	messages, err := sessions.GetMessages(session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get patrol messages: %w", err)
	}

	// Get all tools (patrol runs in autonomous mode)
	filteredTools := s.filterToolsForPrompt(ctx, req.Prompt, true)

	// Run the agentic loop
	resultMessages, err := tempLoop.ExecuteWithTools(ctx, session.ID, messages, filteredTools, callback)
	if err != nil {
		// Still save any messages we got
		for _, msg := range resultMessages {
			if saveErr := sessions.AddMessage(session.ID, msg); saveErr != nil {
				log.Warn().Err(saveErr).Msg("Failed to save patrol message after error")
			}
		}
		return nil, err
	}

	// Save result messages
	for _, msg := range resultMessages {
		if msg.Role == "user" && msg.ToolResult == nil {
			continue
		}
		if err := sessions.AddMessage(session.ID, msg); err != nil {
			log.Warn().Err(err).Msg("Failed to save patrol message")
		}
	}

	// Collect content from result messages
	var contentBuilder strings.Builder
	for _, msg := range resultMessages {
		if msg.Role == "assistant" && msg.Content != "" {
			contentBuilder.WriteString(msg.Content)
		}
	}

	// Send done event
	doneData, _ := json.Marshal(DoneData{
		SessionID:    session.ID,
		InputTokens:  tempLoop.GetTotalInputTokens(),
		OutputTokens: tempLoop.GetTotalOutputTokens(),
	})
	callback(StreamEvent{Type: "done", Data: doneData})

	return &PatrolResponse{
		Content:      contentBuilder.String(),
		InputTokens:  tempLoop.GetTotalInputTokens(),
		OutputTokens: tempLoop.GetTotalOutputTokens(),
	}, nil
}

// createProviderForModel creates a streaming provider for a specific model string (provider:model format).
func (s *Service) createProviderForModel(modelStr string) (providers.StreamingProvider, error) {
	if s.providerFactory != nil {
		return s.providerFactory(modelStr)
	}
	if s.cfg == nil {
		return nil, fmt.Errorf("no Pulse Assistant config")
	}

	parts := strings.SplitN(modelStr, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid model format: %s (expected provider:model)", modelStr)
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
		return providers.NewOpenAIClient(s.cfg.OpenAIAPIKey, modelName, s.cfg.OpenAIBaseURL, timeout), nil
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

// ListAvailableTools returns tool names available for the given prompt.
func (s *Service) ListAvailableTools(ctx context.Context, prompt string) []string {
	s.mu.RLock()
	executor := s.executor
	s.mu.RUnlock()

	if executor == nil {
		return nil
	}

	tools := s.filterToolsForPrompt(ctx, prompt, s.isAutonomousModeEnabled())
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool.Name == "" {
			continue
		}
		names = append(names, tool.Name)
	}

	sort.Strings(names)
	return names
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

// GetExecutor returns the underlying tool executor so callers (e.g. patrol)
// can set session-scoped state like the PatrolFindingCreator before executing.
func (s *Service) GetExecutor() *tools.PulseToolExecutor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.executor
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

func (s *Service) SetGuestConfigProvider(provider MCPGuestConfigProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetGuestConfigProvider(provider)
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

func (s *Service) SetIncidentRecorderProvider(provider IncidentRecorderProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetIncidentRecorderProvider(provider)
	}
}

func (s *Service) SetEventCorrelatorProvider(provider EventCorrelatorProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetEventCorrelatorProvider(provider)
	}
}

func (s *Service) SetTopologyProvider(provider TopologyProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetTopologyProvider(provider)
	}
}

func (s *Service) SetKnowledgeStoreProvider(provider KnowledgeStoreProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetKnowledgeStoreProvider(provider)
	}
}

func (s *Service) SetUnifiedResourceProvider(provider tools.UnifiedResourceProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetUnifiedResourceProvider(provider)
	}
}

func (s *Service) SetDiscoveryProvider(provider MCPDiscoveryProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetDiscoveryProvider(provider)
	}
	// Create/update context prefetcher with the discovery provider
	if s.stateProvider != nil && provider != nil {
		s.contextPrefetcher = NewContextPrefetcher(s.stateProvider, s.readState, provider)
		log.Info().Msg("[ChatService] Context prefetcher created with discovery provider")
	} else {
		log.Warn().
			Bool("hasStateProvider", s.stateProvider != nil).
			Bool("hasDiscoveryProvider", provider != nil).
			Msg("[ChatService] Cannot create context prefetcher - missing provider")
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

// SetAutonomousMode enables or disables autonomous mode for investigations
// When enabled, read-only commands can be auto-approved without user confirmation
// SetBudgetChecker sets a function called after each agentic turn to enforce
// token spending limits. The checker is propagated to the agentic loop.
func (s *Service) SetBudgetChecker(fn func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.budgetChecker = fn
}

func (s *Service) SetAutonomousMode(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autonomousMode = enabled
	if s.executor != nil {
		s.executor.SetContext("", "", enabled)
	}
	if s.agenticLoop != nil {
		s.agenticLoop.SetAutonomousMode(enabled)
	}
}

// ExecuteCommand executes a command directly via the tool executor (bypasses LLM)
// This is used for auto-executing fixes in full autonomy mode
func (s *Service) ExecuteCommand(ctx context.Context, command, targetHost string) (output string, exitCode int, err error) {
	s.mu.RLock()
	executor := s.executor
	s.mu.RUnlock()

	if executor == nil {
		return "", -1, fmt.Errorf("executor not available")
	}

	// Build args for pulse_run_command
	args := map[string]interface{}{
		"command":     command,
		"run_on_host": true,
	}
	if targetHost != "" && targetHost != "local" {
		args["target_host"] = targetHost
	}

	// Execute the command
	result, toolErr := executor.ExecuteTool(ctx, "pulse_run_command", args)
	if toolErr != nil {
		return "", -1, toolErr
	}

	// Extract text from result content
	var resultText string
	for _, content := range result.Content {
		if content.Type == "text" {
			resultText += content.Text
		}
	}

	// Parse the result
	if result.IsError {
		return resultText, 1, fmt.Errorf("command execution failed: %s", resultText)
	}

	// Check if approval was required (this shouldn't happen in autonomous mode, but just in case)
	if strings.HasPrefix(resultText, "APPROVAL_REQUIRED:") {
		return "", -1, fmt.Errorf("command requires approval (unexpected in autonomous mode)")
	}
	if strings.HasPrefix(resultText, "POLICY_BLOCKED:") {
		return "", -1, fmt.Errorf("command blocked by security policy")
	}

	// Check for exit code in result
	if strings.Contains(resultText, "Command failed (exit code") {
		// Parse exit code from message like "Command failed (exit code 1):"
		var code int
		fmt.Sscanf(resultText, "Command failed (exit code %d)", &code)
		return resultText, code, nil
	}

	return resultText, 0, nil
}

// createProvider creates an AI provider based on config
func (s *Service) createProvider() (providers.StreamingProvider, error) {
	if s.cfg == nil {
		return nil, fmt.Errorf("no Pulse Assistant config")
	}

	chatModel := s.cfg.GetChatModel()
	if chatModel == "" {
		return nil, fmt.Errorf("no chat model configured")
	}

	return s.createProviderForModel(chatModel)
}

func (s *Service) applyChatContextSettings() {
	StatelessContext = DefaultStatelessContext
}

// buildSystemPrompt builds the base system prompt for the AI.
// Mode-specific context (autonomous vs controlled) is added dynamically by the AgenticLoop.
//
// Philosophy: This prompt provides IDENTITY and CONTEXT only, not behavioral steering.
// Behavioral guarantees (tool use, no hallucination) are enforced structurally via:
// - tool_choice API parameter (forces tool calls when needed)
// - Phantom execution detection (catches false claims at runtime)
func (s *Service) buildSystemPrompt() string {
	return `You are Pulse AI, a knowledgeable infrastructure assistant. You pair-program with the user on their homelab and infrastructure tasks.

## CAPABILITIES
- pulse_query: Find resources (VMs, containers, hosts) and their locations
- pulse_discovery: Get service details, config paths, ports, bind mounts
- pulse_control: Run commands on hosts/LXCs/VMs
- pulse_docker: Manage Docker containers
- pulse_file_edit: Read and edit configuration files
- pulse_question: Ask the user for missing information using a structured prompt (interactive only)

## INFRASTRUCTURE TOPOLOGY
- Resources are organized hierarchically: Proxmox nodes → VMs/LXCs → Docker containers
- target_host specifies where commands run (host name, LXC name, or VM name)
- Commands execute inside the target: target_host="homepage-docker" runs inside that LXC
- For Docker containers inside LXCs: target the LXC, then use docker commands

## DOCKER BIND MOUNTS
- Container files are often mapped to host paths via bind mounts
- To edit a container's config, find the bind mount and edit the host path
- Use pulse_discovery to find bind mount mappings

## TOOL SELECTION
- pulse_control and pulse_docker are WRITE tools — they change infrastructure state.
- ONLY use write tools when the user explicitly asks you to perform an action.
- For status checks or monitoring, use pulse_query or pulse_read instead.
- If you are missing critical information (target, risky choice, preference), use pulse_question to ask structured questions.
- Do not use pulse_question in autonomous mode; proceed with safe defaults and clearly state assumptions instead.

## HOW TO RESPOND
You are like a colleague doing pair programming on infrastructure tasks. Tool calls are your internal investigation — the user sees your final synthesized response.

1. INVESTIGATE THOROUGHLY: Use tools to gather the information you need. Don't stop after the first tool call if more context would help.

2. SYNTHESIZE YOUR FINDINGS: After using tools, explain what you learned and did. Don't just confirm "done" — provide context that helps the user understand the outcome.

3. SURFACE ISSUES PROACTIVELY: If you discover something during investigation that affects the user's goal (prerequisites missing, config issues, limitations), mention it. Don't hide problems.

4. SUGGEST NEXT STEPS: If there's something the user might need to do next, or if you noticed a potential improvement, mention it.

5. BE DIRECT: Acknowledge mistakes or complications honestly. If something won't work as the user expects, say so clearly.

## TASK COMPLETION
- After control actions succeed, the system auto-verifies — no need to run additional verification commands.
- After receiving "The action is complete" or "Verification complete", stop making tool calls and respond to the user.`
}

var recentContextPronounPattern = regexp.MustCompile(`(?i)\b(it|its|that|those|this|them|previous|earlier|last|same|former|latter)\b`)
var recentContextNounPattern = regexp.MustCompile(`(?i)\b(the (service|container|vm|lxc|node|host|docker|instance|one))\b`)

func shouldInjectRecentContext(prompt string) bool {
	return recentContextPronounPattern.MatchString(prompt) || recentContextNounPattern.MatchString(prompt)
}

func (s *Service) injectRecentContextIfNeeded(prompt, sessionID string, messages []Message, sessions *SessionStore) {
	if !shouldInjectRecentContext(prompt) {
		return
	}

	if sessions == nil {
		return
	}
	resolvedCtx := sessions.GetResolvedContext(sessionID)
	if resolvedCtx == nil {
		return
	}

	recentIDs := resolvedCtx.GetRecentlyAccessedResourcesSorted(tools.RecentAccessWindow, 3)
	if len(recentIDs) == 0 {
		return
	}

	var lines []string
	primaryName := ""
	primaryTarget := ""
	for _, resourceID := range recentIDs {
		res, ok := resolvedCtx.GetResourceByID(resourceID)
		if !ok || res == nil {
			continue
		}
		label := res.Name
		if label == "" {
			label = resourceID
		}
		kind := res.Kind
		if kind == "" {
			kind = res.ResourceType
		}
		location := res.Node
		if location == "" {
			location = res.Scope.HostName
		}
		if kind != "" && location != "" {
			label = fmt.Sprintf("%s (%s on %s)", label, kind, location)
		} else if kind != "" {
			label = fmt.Sprintf("%s (%s)", label, kind)
		}
		lines = append(lines, "- "+label)
		if primaryName == "" {
			primaryName = res.Name
			primaryTarget = res.TargetHost
			if primaryName == "" {
				primaryName = label
			}
		}
	}

	if len(lines) == 0 {
		return
	}

	primary := strings.TrimPrefix(lines[0], "- ")
	if primaryName == "" {
		primaryName = primary
	}
	targetHint := ""
	if primaryTarget != "" {
		targetHint = fmt.Sprintf(" Use target_host=\"%s\".", primaryTarget)
	}
	summary := fmt.Sprintf("Context: The most recently referenced resource is %s. If the user says \"it/its/that\", assume they mean this resource unless they specify otherwise. Do not ask for clarification unless the user names a different resource.%s", primary, targetHint)
	if len(lines) > 1 {
		others := strings.Join(lines[1:], "\n")
		summary += "\nOther recent resources:\n" + others
	}

	lowerPrompt := strings.ToLower(prompt)
	if strings.Contains(lowerPrompt, "log") || strings.Contains(lowerPrompt, "journal") {
		rewrite := fmt.Sprintf("Show logs for %s (last 50 lines).", primaryName)
		if primaryTarget != "" {
			summary += fmt.Sprintf("\nInstruction: %s Use pulse_read action=logs target_host=\"%s\" lines=50.", rewrite, primaryTarget)
		} else {
			summary += fmt.Sprintf("\nInstruction: %s Use pulse_read action=logs target_host=\"%s\" lines=50.", rewrite, primaryName)
		}
	}

	log.Debug().
		Str("session_id", sessionID).
		Strs("recent_resource_ids", recentIDs).
		Msg("[ChatService] Injecting recent context")

	if len(messages) == 0 {
		return
	}
	lastIdx := len(messages) - 1
	if messages[lastIdx].Role != "user" {
		return
	}

	messages[lastIdx].Content = summary + "\n\n---\nExplicit target: " + primaryName + "\nUser question (targeted): " + messages[lastIdx].Content
}

func (s *Service) filterToolsForPrompt(ctx context.Context, prompt string, autonomousMode bool) []providers.Tool {
	mcpTools := s.executor.ListTools()
	providerTools := ConvertMCPToolsToProvider(mcpTools)

	// Filter out write/control tools when the user's request is read-only.
	// This prevents models from calling pulse_control (restart, stop, etc.) when
	// the user only asked for status, logs, or monitoring information.
	//
	// The tool set is determined once per user message and stays consistent for
	// the entire agentic loop. This avoids the old problem of tools appearing/
	// disappearing mid-conversation (which caused hallucinated tool names).
	readOnly := !hasWriteIntent(convertPromptToMessages(prompt))

	// Determine which specialty tools are relevant based on prompt keywords.
	// Core tools are always included; specialty tools only when topic-relevant.
	// This reduces token consumption on every request.
	lowerPrompt := strings.ToLower(prompt)
	includeK8s := promptMentionsAny(lowerPrompt, k8sKeywords)
	includePMG := promptMentionsAny(lowerPrompt, pmgKeywords)
	includeStorage := promptMentionsAny(lowerPrompt, storageKeywords) || promptMentionsBroadInfra(lowerPrompt)
	includeDocker := promptMentionsAny(lowerPrompt, dockerKeywords)

	// If no specialty keywords detected, include everything (safe default).
	noSpecialtyDetected := !includeK8s && !includePMG && !includeStorage && !includeDocker

	filtered := make([]providers.Tool, 0, len(providerTools))
	for _, tool := range providerTools {
		// Remove write tools for read-only prompts
		if readOnly && isWriteTool(tool.Name) {
			continue
		}
		// Conditionally include specialty tools
		if !noSpecialtyDetected && isSpecialtyTool(tool.Name) {
			switch tool.Name {
			case "pulse_kubernetes":
				if !includeK8s {
					continue
				}
			case "pulse_pmg":
				if !includePMG {
					continue
				}
			case "pulse_storage":
				if !includeStorage {
					continue
				}
			case "pulse_docker":
				if !includeDocker && readOnly {
					// Only filter docker for read-only; write-intent already implies docker may be needed
					continue
				}
			}
		}
		filtered = append(filtered, tool)
	}

	log.Debug().
		Int("total_tools", len(providerTools)).
		Int("filtered_tools", len(filtered)).
		Bool("read_only", readOnly).
		Bool("specialty_filter_active", !noSpecialtyDetected).
		Str("prompt_prefix", truncateForLog(prompt, 80)).
		Msg("[filterToolsForPrompt] Filtered tools for prompt")

	// pulse_question is interactive; exclude it for autonomous runs (Pulse Patrol).
	if !autonomousMode {
		filtered = append(filtered, userQuestionTool())
	}

	return filtered
}

// isSpecialtyTool returns true for tools that are only relevant to specific topics.
func isSpecialtyTool(name string) bool {
	switch name {
	case "pulse_kubernetes", "pulse_pmg", "pulse_storage", "pulse_docker":
		return true
	default:
		return false
	}
}

// Keyword lists for specialty tool detection
var (
	k8sKeywords = []string{
		"k8s", "kubernetes", "kubectl", "pod", "pods", "deployment", "deployments",
		"namespace", "namespaces", "replica", "replicas", "cluster",
		"node pool", "daemonset", "statefulset", "ingress", "helm",
	}
	pmgKeywords = []string{
		"mail", "email", "spam", "pmg", "mail gateway", "postfix",
		"smtp", "queue", "bounce", "quarantine",
	}
	storageKeywords = []string{
		"backup", "backups", "snapshot", "snapshots", "storage", "disk",
		"ceph", "zfs", "raid", "replication", "pbs", "lvm",
		"smart", "pool", "pools", "s3", "nfs",
	}
	dockerKeywords = []string{
		"docker", "container", "containers", "swarm", "compose",
		"image", "images", "registry",
	}
)

// promptMentionsAny checks if the lowercased prompt contains any of the keywords.
func promptMentionsAny(lowerPrompt string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(lowerPrompt, kw) {
			return true
		}
	}
	return false
}

// promptMentionsBroadInfra returns true for broad infrastructure questions
// where storage tools should remain available.
func promptMentionsBroadInfra(lowerPrompt string) bool {
	broadPatterns := []string{
		"infrastructure", "overview", "health check", "full status",
		"everything", "all systems", "entire",
	}
	for _, p := range broadPatterns {
		if strings.Contains(lowerPrompt, p) {
			return true
		}
	}
	return false
}

// convertPromptToMessages wraps a prompt string into a providers.Message slice
// for use with hasWriteIntent.
func convertPromptToMessages(prompt string) []providers.Message {
	return []providers.Message{{Role: "user", Content: prompt}}
}

func (s *Service) isAutonomousModeEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.autonomousMode {
		return true
	}
	return s.cfg != nil && s.cfg.IsAutonomous()
}

// ExecuteMCPTool executes an MCP tool directly by name with arguments
// This is used for executing investigation fixes that are MCP tool calls
func (s *Service) ExecuteMCPTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	s.mu.RLock()
	executor := s.executor
	s.mu.RUnlock()

	if executor == nil {
		return "", fmt.Errorf("executor not available")
	}

	log.Debug().
		Str("tool", toolName).
		Interface("args", args).
		Msg("Executing MCP tool directly")

	// Execute the tool
	result, toolErr := executor.ExecuteTool(ctx, toolName, args)
	if toolErr != nil {
		return "", toolErr
	}

	// Extract text from result content
	var resultText string
	for _, content := range result.Content {
		if content.Type == "text" {
			resultText += content.Text
		}
	}

	// Check for error
	if result.IsError {
		return resultText, fmt.Errorf("tool execution failed: %s", resultText)
	}

	// Check if approval was required
	if strings.HasPrefix(resultText, "APPROVAL_REQUIRED:") {
		return "", fmt.Errorf("tool requires approval (unexpected in fix execution)")
	}
	if strings.HasPrefix(resultText, "POLICY_BLOCKED:") {
		return "", fmt.Errorf("tool blocked by security policy")
	}

	return resultText, nil
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
