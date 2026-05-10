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
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/modelboundary"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// StateProvider is a type alias for models.SnapshotProvider.
// Kept for local convenience; all new code should use models.SnapshotProvider directly.
type StateProvider = models.SnapshotProvider

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
	MCPAlertProvider              = tools.AlertProvider
	MCPFindingsProvider           = tools.FindingsProvider
	MCPBaselineProvider           = tools.BaselineProvider
	MCPPatternProvider            = tools.PatternProvider
	MCPMetricsHistoryProvider     = tools.MetricsHistoryProvider
	MCPBackupProvider             = tools.BackupProvider
	MCPGuestConfigProvider        = tools.GuestConfigProvider
	MCPAppContainerConfigProvider = tools.AppContainerConfigProvider
	MCPDiskHealthProvider         = tools.DiskHealthProvider
	MCPUpdatesProvider            = tools.UpdatesProvider
	MCPAppContainerActionProvider = tools.AppContainerActionProvider
	MCPAppContainerReadProvider   = tools.AppContainerReadProvider
	AgentProfileManager           = tools.AgentProfileManager
	FindingsManager               = tools.FindingsManager
	MetadataUpdater               = tools.MetadataUpdater
	IncidentRecorderProvider      = tools.IncidentRecorderProvider
	EventCorrelatorProvider       = tools.EventCorrelatorProvider
	KnowledgeStoreProvider        = tools.KnowledgeStoreProvider
	MCPDiscoveryProvider          = tools.DiscoveryProvider
	MCPUnifiedResourceProvider    = tools.UnifiedResourceProvider
)

// Config holds service configuration
type Config struct {
	AIConfig      *config.AIConfig
	StateProvider StateProvider
	ReadState     unifiedresources.ReadState
	Policy        CommandPolicy
	AgentServer   AgentServer
	DataDir       string
	OrgID         string

	// Optional: provides access to persisted recovery points (backups/snapshots).
	RecoveryPointsProvider tools.RecoveryPointsProvider

	// Optional: resolves the effective control level for current entitlements.
	// Stored config may say autonomous, but runtime execution must use the
	// entitlement-clamped level.
	ControlLevelResolver func(*config.AIConfig) string
}

// Service provides direct AI chat without external sidecar
type Service struct {
	mu sync.RWMutex

	cfg                     *config.AIConfig
	dataDir                 string
	stateProvider           StateProvider
	readState               unifiedresources.ReadState
	agentServer             AgentServer
	executor                *tools.PulseToolExecutor
	unifiedResourceProvider tools.UnifiedResourceProvider
	actionAuditStore        unifiedresources.ResourceStore
	sessions                *SessionStore
	agenticLoop             *AgenticLoop
	provider                providers.StreamingProvider
	providerFactory         func(modelStr string) (providers.StreamingProvider, error)
	patrolProviderFactory   func(modelStr string) (providers.StreamingProvider, error)
	started                 bool
	autonomousMode          bool
	contextPrefetcher       *ContextPrefetcher
	budgetChecker           func() error // Optional mid-run budget enforcement
	orgID                   string
	controlLevelResolver    func(*config.AIConfig) string

	activeMu           sync.RWMutex
	activeExecutions   map[string]map[*AgenticLoop]struct{}
	questionExecutions map[string]*AgenticLoop
}

// SetPatrolProviderFactory installs a Patrol-only provider factory override.
// This lets Patrol resolve provider-scoped model strings without changing
// normal interactive chat provider selection in the same slice.
func (s *Service) SetPatrolProviderFactory(factory func(modelStr string) (providers.StreamingProvider, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.patrolProviderFactory = factory
}

// NewService creates a new chat service
func NewService(cfg Config) *Service {
	// Create tool executor
	var policy tools.CommandPolicy
	var agentServer tools.AgentServer

	if cfg.Policy != nil {
		policy = &commandPolicyAdapter{cfg.Policy}
	}
	if cfg.AgentServer != nil {
		agentServer = &agentServerAdapter{cfg.AgentServer}
	}

	execCfg := tools.ExecutorConfig{
		StateProvider:          cfg.StateProvider,
		ReadState:              cfg.ReadState,
		Policy:                 policy,
		AgentServer:            agentServer,
		RecoveryPointsProvider: cfg.RecoveryPointsProvider,
		OrgID:                  cfg.OrgID,
	}

	if cfg.AIConfig != nil {
		execCfg.ControlLevel = resolveEffectiveControlLevel(cfg.ControlLevelResolver, cfg.AIConfig)
		execCfg.ProtectedGuests = cfg.AIConfig.GetProtectedGuests()
	}

	executor := tools.NewPulseToolExecutor(execCfg)

	// Set telemetry callback for strict resolution metrics
	executor.SetTelemetryCallback(NewAIMetricsTelemetryCallback())

	return &Service{
		cfg:                  cfg.AIConfig,
		dataDir:              cfg.DataDir,
		stateProvider:        cfg.StateProvider,
		readState:            cfg.ReadState,
		agentServer:          cfg.AgentServer,
		executor:             executor,
		orgID:                strings.TrimSpace(cfg.OrgID),
		controlLevelResolver: cfg.ControlLevelResolver,
		activeExecutions:     make(map[string]map[*AgenticLoop]struct{}),
		questionExecutions:   make(map[string]*AgenticLoop),
	}
}

func resolveEffectiveControlLevel(
	resolver func(*config.AIConfig) string,
	cfg *config.AIConfig,
) tools.ControlLevel {
	if resolver != nil {
		if resolved := strings.TrimSpace(resolver(cfg)); config.IsValidControlLevel(resolved) {
			return tools.ControlLevel(resolved)
		}
	}
	if cfg == nil {
		return tools.ControlLevelReadOnly
	}
	return tools.ControlLevel(cfg.GetControlLevel())
}

func controlLevelForRequestAutonomousMode(level tools.ControlLevel, requested *bool) tools.ControlLevel {
	if requested == nil || *requested {
		return level
	}
	if level == tools.ControlLevelAutonomous {
		return tools.ControlLevelControlled
	}
	return level
}

func (s *Service) effectiveControlLevelLocked() tools.ControlLevel {
	return resolveEffectiveControlLevel(s.controlLevelResolver, s.cfg)
}

func (s *Service) registerActiveLoop(sessionID string, loop *AgenticLoop) {
	if s == nil || sessionID == "" || loop == nil {
		return
	}

	s.activeMu.Lock()
	defer s.activeMu.Unlock()

	if s.activeExecutions == nil {
		s.activeExecutions = make(map[string]map[*AgenticLoop]struct{})
	}
	loops := s.activeExecutions[sessionID]
	if loops == nil {
		loops = make(map[*AgenticLoop]struct{})
		s.activeExecutions[sessionID] = loops
	}
	loops[loop] = struct{}{}
}

func (s *Service) unregisterActiveLoop(sessionID string, loop *AgenticLoop) {
	if s == nil || sessionID == "" || loop == nil {
		return
	}

	s.activeMu.Lock()
	defer s.activeMu.Unlock()

	if loops := s.activeExecutions[sessionID]; loops != nil {
		delete(loops, loop)
		if len(loops) == 0 {
			delete(s.activeExecutions, sessionID)
		}
	}
	for questionID, registeredLoop := range s.questionExecutions {
		if registeredLoop == loop {
			delete(s.questionExecutions, questionID)
		}
	}
}

func (s *Service) registerQuestionLoop(questionID string, loop *AgenticLoop) {
	if s == nil || questionID == "" || loop == nil {
		return
	}

	s.activeMu.Lock()
	defer s.activeMu.Unlock()

	if s.questionExecutions == nil {
		s.questionExecutions = make(map[string]*AgenticLoop)
	}
	s.questionExecutions[questionID] = loop
}

func (s *Service) findQuestionLoop(questionID string) *AgenticLoop {
	if s == nil || questionID == "" {
		return nil
	}

	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	return s.questionExecutions[questionID]
}

func (s *Service) getActiveLoops(sessionID string) []*AgenticLoop {
	if s == nil || sessionID == "" {
		return nil
	}

	s.activeMu.RLock()
	defer s.activeMu.RUnlock()

	loops := s.activeExecutions[sessionID]
	if len(loops) == 0 {
		return nil
	}

	active := make([]*AgenticLoop, 0, len(loops))
	for loop := range loops {
		active = append(active, loop)
	}
	return active
}

// Adapter types
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

	if s.actionAuditStore == nil {
		actionAuditStore, err := unifiedresources.NewSQLiteResourceStore(dataDir, s.orgID)
		if err != nil {
			log.Warn().
				Err(err).
				Str("orgID", s.orgID).
				Str("data_dir", dataDir).
				Msg("failed to initialize action audit store")
		} else {
			s.actionAuditStore = actionAuditStore
			if s.executor != nil {
				s.executor.SetActionAuditStore(actionAuditStore)
			}
		}
	}

	// Create provider
	provider, err := s.createProvider()
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create agentic loop
	systemPrompt := s.buildSystemPrompt()
	s.agenticLoop = NewAgenticLoop(provider, s.executor, systemPrompt)
	s.agenticLoop.SetOrgID(s.orgID)
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
	if s.actionAuditStore != nil {
		if err := s.actionAuditStore.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close action audit store")
		}
		s.actionAuditStore = nil
	}
	log.Info().Msg("pulse AI (direct) stopped")
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
		s.executor.SetControlLevel(s.effectiveControlLevelLocked())
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
	s.agenticLoop.SetOrgID(s.orgID)
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

	handoffContext := strings.TrimSpace(req.HandoffContext)
	handoffResources := normalizeHandoffResources(req.HandoffResources)
	handoffActions := normalizeHandoffActions(req.HandoffActions)
	handoffMetadata := NormalizeHandoffMetadata(req.HandoffMetadata)
	handoffFindingID := strings.TrimSpace(req.FindingID)
	hasRequestHandoffEnvelope := handoffFindingID != "" ||
		handoffContext != "" ||
		len(handoffResources) > 0 ||
		len(handoffActions) > 0 ||
		!handoffMetadataEmpty(handoffMetadata)
	if hasRequestHandoffEnvelope {
		if err := sessions.SetModelHandoffEnvelope(session.ID, handoffFindingID, handoffContext, handoffResources, handoffActions, handoffMetadata); err != nil {
			log.Warn().Err(err).Str("session_id", session.ID).Msg("[ChatService] Failed to persist model handoff envelope")
		}
	} else {
		storedHandoffResources, err := sessions.GetModelHandoffResources(session.ID)
		if err != nil {
			log.Warn().Err(err).Str("session_id", session.ID).Msg("[ChatService] Failed to load model handoff resources")
		} else {
			handoffResources = storedHandoffResources
		}
		storedHandoffActions, err := sessions.GetModelHandoffActions(session.ID)
		if err != nil {
			log.Warn().Err(err).Str("session_id", session.ID).Msg("[ChatService] Failed to load model handoff actions")
		} else {
			handoffActions = storedHandoffActions
		}
		storedHandoffContext, err := sessions.GetModelHandoffContext(session.ID)
		if err != nil {
			log.Warn().Err(err).Str("session_id", session.ID).Msg("[ChatService] Failed to load model handoff context")
		} else {
			handoffContext = storedHandoffContext
		}
	}

	// Add user message
	userMsg := Message{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   req.Prompt,
		Timestamp: time.Now(),
	}
	if err := sessions.AddMessage(session.ID, userMsg); err != nil {
		log.Warn().Err(err).Msg("failed to save user message")
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

	handoffActions = refreshHandoffActionStatus(handoffActions, s.orgID, s.actionAuditStore)
	if len(handoffActions) > 0 {
		if err := sessions.SetModelHandoffActions(session.ID, handoffActions); err != nil {
			log.Warn().Err(err).Str("session_id", session.ID).Msg("[ChatService] Failed to persist refreshed handoff action status")
		}
	}
	s.mu.RLock()
	handoffResourceProvider := s.unifiedResourceProvider
	s.mu.RUnlock()
	handoffContext = mergeHandoffResourcePolicyContext(handoffContext, handoffResources, handoffResourceProvider)
	handoffContext = mergeHandoffResourceStateContext(handoffContext, handoffResources, handoffResourceProvider)
	handoffContext = mergeHandoffResourceRelationshipContext(handoffContext, handoffResources, handoffResourceProvider)
	handoffContext = mergeHandoffResourceTimelineContext(handoffContext, handoffResources, handoffResourceProvider, s.actionAuditStore, time.Now())
	handoffContext = mergeHandoffActionContext(handoffContext, handoffActions)
	handoffContext = sanitizeHandoffContextForResourcePolicy(handoffContext, handoffResources, handoffResourceProvider)
	injectHandoffContextIntoLatestUserMessage(messages, handoffContext)

	// Determine which model/loop to use for this request.
	selectedModel := ""
	configuredModel := ""
	overrideModel := strings.TrimSpace(req.Model)
	var executor *tools.PulseToolExecutor
	autonomousMode := false
	effectiveControlLevel := tools.ControlLevelReadOnly
	s.mu.RLock()
	baseExecutor := s.executor
	unifiedResourceProvider := s.unifiedResourceProvider
	autonomousMode = s.autonomousMode
	effectiveControlLevel = s.effectiveControlLevelLocked()
	if s.cfg != nil {
		configuredModel = strings.TrimSpace(s.cfg.GetChatModel())
	}
	s.mu.RUnlock()
	if baseExecutor != nil {
		executor = baseExecutor.Clone()
		executor.SetControlLevel(effectiveControlLevel)
	}
	s.hydrateHandoffResources(session.ID, handoffResources, sessions, unifiedResourceProvider)

	// Per-request autonomous mode override (used by investigation to avoid
	// mutating shared service state from concurrent goroutines).
	if req.AutonomousMode != nil {
		autonomousMode = *req.AutonomousMode
	}
	effectiveControlLevel = controlLevelForRequestAutonomousMode(effectiveControlLevel, req.AutonomousMode)
	if executor != nil {
		executor.SetControlLevel(effectiveControlLevel)
		executor.SetAutonomousMode(autonomousMode)
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
		loop.SetOrgID(s.orgID)
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
		loop.SetOrgID(s.orgID)
	}
	loop.SetAutonomousMode(autonomousMode)
	loop.SetRequestSanitizer(modelboundary.RequestSanitizerForModel(selectedModel, unifiedResourceProvider))
	s.registerActiveLoop(session.ID, loop)
	defer s.unregisterActiveLoop(session.ID, loop)

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
				// Build canonical resource ID: kind:scope:id
				resourceID := strings.TrimSpace(mention.UnifiedResourceID)
				if resourceID == "" {
					switch mention.ResourceType {
					case "system-container":
						if mention.TargetID != "" {
							resourceID = "system-container:" + mention.TargetID + ":" + mention.ResourceID
						}
					case "vm":
						if mention.TargetID != "" {
							resourceID = "vm:" + mention.TargetID + ":" + mention.ResourceID
						}
					case "app-container":
						if mention.TargetHost != "" {
							resourceID = "app-container:" + mention.TargetHost + ":" + mention.ResourceID
						}
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
	if s.shouldRunExplore(autonomousMode, req.Prompt) {
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
	filteredTools := s.filterToolsForPrompt(ctx, req.Prompt, autonomousMode, false)
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

	streamCallback := callback
	if streamCallback == nil {
		streamCallback = func(StreamEvent) {}
	}
	wrappedCallback := func(event StreamEvent) {
		if event.Type == "question" {
			var data QuestionData
			if err := json.Unmarshal(event.Data, &data); err == nil && data.QuestionID != "" {
				s.registerQuestionLoop(data.QuestionID, loop)
			}
		}
		streamCallback(event)
	}

	resultMessages, err := loop.ExecuteWithTools(ctx, session.ID, messages, filteredTools, wrappedCallback)

	log.Debug().
		Str("session_id", session.ID).
		Int("result_messages", len(resultMessages)).
		Err(err).
		Msg("[ChatService] Agentic loop returned")

	if err != nil {
		// Still save any messages we got
		for _, msg := range resultMessages {
			if saveErr := sessions.AddMessage(session.ID, msg); saveErr != nil {
				log.Warn().Err(saveErr).Msg("failed to save message after error")
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
			log.Warn().Err(err).Msg("failed to save message")
		}
	}

	// Send done event with token usage for this request.
	doneData, _ := json.Marshal(DoneData{
		SessionID:    session.ID,
		InputTokens:  loop.GetTotalInputTokens(),
		OutputTokens: loop.GetTotalOutputTokens(),
	})
	streamCallback(StreamEvent{Type: "done", Data: doneData})

	return nil
}

func injectHandoffContextIntoLatestUserMessage(messages []Message, handoffContext string) {
	contextText := strings.TrimSpace(handoffContext)
	if contextText == "" || len(messages) == 0 {
		return
	}

	for idx := len(messages) - 1; idx >= 0; idx-- {
		if messages[idx].Role != "user" {
			continue
		}
		userText := strings.TrimSpace(messages[idx].Content)
		if userText == "" {
			messages[idx].Content = contextText
			return
		}
		messages[idx].Content = contextText + "\n\n---\nUser message: " + userText
		return
	}
}

func sanitizeHandoffContextForResourcePolicy(handoffContext string, handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) string {
	contextText := strings.TrimSpace(handoffContext)
	resources := normalizeHandoffResources(handoffResources)
	if contextText == "" || len(resources) == 0 || provider == nil {
		return contextText
	}

	redacted := contextText
	for _, handoffResource := range resources {
		resource, ok := tools.CanonicalHandoffUnifiedResource(provider, handoffResource.ID, handoffResource.Name, handoffResource.Type, handoffResource.Node)
		if !ok {
			continue
		}

		policy, aiSafeSummary := unifiedresources.CanonicalGovernanceMetadata(&resource)
		if policy == nil {
			continue
		}
		resource.Policy = policy
		resource.AISafeSummary = aiSafeSummary
		redacted = unifiedresources.ResourcePolicyRedactedTextWithReferences(redacted, resource, handoffResourcePolicyReferences(handoffResource, resource)...)
	}
	return strings.TrimSpace(redacted)
}

func handoffResourcePolicyReferences(handoffResource HandoffResource, resource unifiedresources.Resource) []unifiedresources.ResourcePolicyRedactionReference {
	references := make([]unifiedresources.ResourcePolicyRedactionReference, 0, 16)
	add := func(value string, hints ...unifiedresources.ResourceRedactionHint) {
		references = append(references, unifiedresources.ResourcePolicyReference(value, hints...))
	}

	add(resource.Name,
		unifiedresources.ResourceRedactionHostname,
		unifiedresources.ResourceRedactionAlias,
	)
	add(resource.ID,
		unifiedresources.ResourceRedactionPlatformID,
		unifiedresources.ResourceRedactionAlias,
	)
	if resource.Canonical != nil {
		add(resource.Canonical.DisplayName,
			unifiedresources.ResourceRedactionHostname,
			unifiedresources.ResourceRedactionAlias,
		)
		add(resource.Canonical.Hostname,
			unifiedresources.ResourceRedactionHostname,
		)
		add(resource.Canonical.PlatformID,
			unifiedresources.ResourceRedactionPlatformID,
		)
		add(resource.Canonical.PrimaryID,
			unifiedresources.ResourceRedactionPlatformID,
		)
		for _, alias := range resource.Canonical.Aliases {
			add(alias,
				unifiedresources.ResourceRedactionAlias,
			)
		}
	}
	if resource.Proxmox != nil {
		add(resource.Proxmox.NodeName,
			unifiedresources.ResourceRedactionHostname,
			unifiedresources.ResourceRedactionAlias,
		)
		add(resource.Proxmox.ClusterName,
			unifiedresources.ResourceRedactionAlias,
		)
	}
	if resource.Kubernetes != nil {
		add(resource.Kubernetes.NodeName,
			unifiedresources.ResourceRedactionHostname,
			unifiedresources.ResourceRedactionAlias,
		)
		add(resource.Kubernetes.ClusterID,
			unifiedresources.ResourceRedactionPlatformID,
		)
		add(resource.Kubernetes.ClusterName,
			unifiedresources.ResourceRedactionAlias,
		)
		add(resource.Kubernetes.SourceName,
			unifiedresources.ResourceRedactionAlias,
		)
	}
	if resource.Storage != nil {
		add(resource.Storage.Path,
			unifiedresources.ResourceRedactionPath,
		)
	}

	add(handoffResource.Name,
		unifiedresources.ResourceRedactionHostname,
		unifiedresources.ResourceRedactionAlias,
	)
	add(handoffResource.ID,
		unifiedresources.ResourceRedactionPlatformID,
		unifiedresources.ResourceRedactionAlias,
	)
	add(handoffResource.Node,
		unifiedresources.ResourceRedactionHostname,
		unifiedresources.ResourceRedactionPlatformID,
		unifiedresources.ResourceRedactionAlias,
	)
	return references
}

func mergeHandoffResourcePolicyContext(handoffContext string, handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) string {
	policyContext := buildHandoffResourcePolicyContext(handoffResources, provider)
	switch {
	case strings.TrimSpace(handoffContext) == "":
		return policyContext
	case policyContext == "":
		return strings.TrimSpace(handoffContext)
	default:
		return strings.TrimSpace(handoffContext) + "\n\n" + policyContext
	}
}

func buildHandoffResourcePolicyContext(handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) string {
	resources := canonicalHandoffResources(handoffResources, provider)
	if len(resources) == 0 {
		return ""
	}

	var b strings.Builder
	count := 0
	for _, resource := range resources {
		policy, aiSafeSummary := unifiedresources.CanonicalGovernanceMetadata(&resource)
		if policy == nil && strings.TrimSpace(aiSafeSummary) == "" {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("[Resource Policy Context]")
		}
		count++

		label := unifiedresources.ResourcePolicyLabel(resource.Name, aiSafeSummary, policy)
		if label == "" {
			label = "resource"
		}
		resourceType := strings.TrimSpace(string(unifiedresources.ContractResourceType(resource)))
		if resourceType == "" {
			resourceType = strings.TrimSpace(string(resource.Type))
		}

		contextLabel := "Resource Policy"
		if count > 1 {
			contextLabel = fmt.Sprintf("Resource Policy %d", count)
		}
		appendHandoffContextLine(&b, contextLabel, label)
		appendHandoffContextLine(&b, contextLabel+" Type", resourceType)
		for _, line := range unifiedresources.ResourcePolicySummaryLines(policy) {
			if line = strings.TrimSpace(line); line != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(line)
			}
		}
		if summary := strings.TrimSpace(aiSafeSummary); summary != "" && summary != label {
			appendHandoffContextLine(&b, contextLabel+" AI-safe Summary", summary)
		}
	}
	if count == 0 {
		return ""
	}
	appendHandoffContextLine(&b, "Policy Boundary", "Resource policy is read-only data-handling context; local-only and redaction guidance must not be bypassed and does not grant approval or execution authority.")
	return strings.TrimSpace(b.String())
}

func mergeHandoffResourceStateContext(handoffContext string, handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) string {
	stateContext := buildHandoffResourceStateContext(handoffResources, provider)
	switch {
	case strings.TrimSpace(handoffContext) == "":
		return stateContext
	case stateContext == "":
		return strings.TrimSpace(handoffContext)
	default:
		return strings.TrimSpace(handoffContext) + "\n\n" + stateContext
	}
}

func buildHandoffResourceStateContext(handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) string {
	resources := canonicalHandoffResources(handoffResources, provider)
	if len(resources) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("[Resource State Context]")
	for idx, resource := range resources {
		labelPrefix := "Resource State"
		if len(resources) > 1 {
			labelPrefix = fmt.Sprintf("Resource State %d", idx+1)
		}
		policy, aiSafeSummary := unifiedresources.CanonicalGovernanceMetadata(&resource)
		label := unifiedresources.ResourcePolicyLabel(resource.Name, aiSafeSummary, policy)
		if label == "" {
			label = "resource"
		}
		resourceType := strings.TrimSpace(string(unifiedresources.ContractResourceType(resource)))
		if resourceType == "" {
			resourceType = strings.TrimSpace(string(resource.Type))
		}

		appendHandoffContextLine(&b, labelPrefix, label)
		appendHandoffContextLine(&b, labelPrefix+" ID", formatHandoffResourceID(resource, policy))
		appendHandoffContextLine(&b, labelPrefix+" Type", resourceType)
		appendHandoffContextLine(&b, labelPrefix+" Status", string(resource.Status))
		appendHandoffContextLine(&b, labelPrefix+" Last Seen At", formatHandoffContextTime(resource.LastSeen))
		appendHandoffContextLine(&b, labelPrefix+" Updated At", formatHandoffContextTime(resource.UpdatedAt))
		appendHandoffContextLine(&b, labelPrefix+" Sources", formatHandoffResourceSources(resource))
		appendHandoffContextLine(&b, labelPrefix+" Source Health", formatHandoffResourceSourceStatus(resource))
		appendHandoffContextLine(&b, labelPrefix+" Metrics", formatHandoffResourceMetrics(resource.Metrics))
		appendHandoffContextLine(&b, labelPrefix+" Parent", formatHandoffResourceParent(resource, policy))
		if resource.ChildCount > 0 {
			appendHandoffContextLine(&b, labelPrefix+" Child Count", fmt.Sprintf("%d", resource.ChildCount))
		}
		appendHandoffContextLine(&b, labelPrefix+" Incident Summary", formatHandoffResourceIncidentSummary(resource, policy))
		appendHandoffContextLine(&b, labelPrefix+" Capabilities", formatHandoffResourceCapabilities(resource, policy, 4))
	}
	appendHandoffContextLine(&b, "Resource State Boundary", "Current resource state, source health, incidents, metrics, and capabilities are read-only canonical infrastructure context; they are not approval or execution authority.")
	return strings.TrimSpace(b.String())
}

func mergeHandoffResourceRelationshipContext(handoffContext string, handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) string {
	relationshipContext := buildHandoffResourceRelationshipContext(handoffResources, provider)
	switch {
	case strings.TrimSpace(handoffContext) == "":
		return relationshipContext
	case relationshipContext == "":
		return strings.TrimSpace(handoffContext)
	default:
		return strings.TrimSpace(handoffContext) + "\n\n" + relationshipContext
	}
}

func buildHandoffResourceRelationshipContext(handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) string {
	resources := canonicalHandoffResources(handoffResources, provider)
	if len(resources) == 0 {
		return ""
	}

	var b strings.Builder
	count := 0
	for _, resource := range resources {
		resource.Relationships = unifiedresources.ResourceRelationshipsWithCanonicalParent(resource)
		relationshipContext := strings.TrimSpace(unifiedresources.FormatResourceRelationshipContext(&resource, 3))
		if relationshipContext == "" {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("[Resource Relationship Context]")
		}
		count++

		policy, aiSafeSummary := unifiedresources.CanonicalGovernanceMetadata(&resource)
		label := unifiedresources.ResourcePolicyLabel(resource.Name, aiSafeSummary, policy)
		if label == "" {
			label = "resource"
		}
		contextLabel := "Resource Relationships For"
		if count > 1 {
			contextLabel = fmt.Sprintf("Resource Relationships %d For", count)
		}
		appendHandoffContextLine(&b, contextLabel, label)
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(relationshipContext)
	}
	if count == 0 {
		return ""
	}
	appendHandoffContextLine(&b, "Relationship Boundary", "Relationships are read-only canonical topology context; they are not approval or execution authority.")
	return strings.TrimSpace(b.String())
}

func mergeHandoffActionContext(handoffContext string, handoffActions []HandoffAction) string {
	actionContext := buildHandoffActionContext(handoffActions)
	switch {
	case strings.TrimSpace(handoffContext) == "":
		return actionContext
	case actionContext == "":
		return strings.TrimSpace(handoffContext)
	default:
		return strings.TrimSpace(handoffContext) + "\n\n" + actionContext
	}
}

func mergeHandoffResourceTimelineContext(handoffContext string, handoffResources []HandoffResource, provider tools.UnifiedResourceProvider, store unifiedresources.ResourceStore, now time.Time) string {
	timelineContext := buildHandoffResourceTimelineContext(handoffResources, provider, store, now)
	switch {
	case strings.TrimSpace(handoffContext) == "":
		return timelineContext
	case timelineContext == "":
		return strings.TrimSpace(handoffContext)
	default:
		return strings.TrimSpace(handoffContext) + "\n\n" + timelineContext
	}
}

func buildHandoffResourceTimelineContext(handoffResources []HandoffResource, provider tools.UnifiedResourceProvider, store unifiedresources.ResourceStore, now time.Time) string {
	resourceIDs := canonicalHandoffTimelineResourceIDs(handoffResources, provider)
	if len(resourceIDs) == 0 || store == nil {
		return ""
	}
	if now.IsZero() {
		now = time.Now()
	}

	const (
		perResourceLimit = 5
		totalLimit       = 8
	)
	since := now.Add(-24 * time.Hour)
	changes := make([]unifiedresources.ResourceChange, 0, len(resourceIDs)*perResourceLimit)
	seen := make(map[string]struct{})
	for _, resourceID := range resourceIDs {
		recent, err := store.GetRecentChangesFiltered(resourceID, since, perResourceLimit, unifiedresources.ResourceChangeFilters{IncludeRelated: true})
		if err != nil {
			log.Debug().
				Err(err).
				Str("resource_id", resourceID).
				Msg("[ChatService] Failed to load handoff resource timeline")
			continue
		}
		for _, change := range recent {
			key := handoffResourceTimelineChangeKey(change)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			changes = append(changes, change)
		}
	}
	if len(changes) == 0 {
		return ""
	}
	sort.SliceStable(changes, func(i, j int) bool {
		return changes[i].ObservedAt.After(changes[j].ObservedAt)
	})
	if len(changes) > totalLimit {
		changes = changes[:totalLimit]
	}

	contextText := strings.TrimSpace(unifiedresources.FormatResourceRecentChangesContext(changes, true, "###"))
	if contextText == "" {
		return ""
	}
	return contextText + "\nTimeline Boundary: Recent changes are read-only canonical resource timeline context; they are not approval or execution authority."
}

func canonicalHandoffTimelineResourceIDs(handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) []string {
	resources := normalizeHandoffResources(handoffResources)
	if len(resources) == 0 {
		return nil
	}

	ids := make([]string, 0, len(resources))
	seen := make(map[string]struct{}, len(resources))
	for _, handoffResource := range resources {
		resourceID := strings.TrimSpace(handoffResource.ID)
		if provider != nil {
			if resource, ok := tools.CanonicalHandoffUnifiedResource(provider, handoffResource.ID, handoffResource.Name, handoffResource.Type, handoffResource.Node); ok {
				if canonicalID := strings.TrimSpace(resource.ID); canonicalID != "" {
					resourceID = canonicalID
				}
			}
		}
		if resourceID == "" {
			continue
		}
		key := strings.ToLower(resourceID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ids = append(ids, resourceID)
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func canonicalHandoffResources(handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) []unifiedresources.Resource {
	resources := normalizeHandoffResources(handoffResources)
	if len(resources) == 0 || provider == nil {
		return nil
	}

	out := make([]unifiedresources.Resource, 0, len(resources))
	seen := make(map[string]struct{}, len(resources))
	for _, handoffResource := range resources {
		resource, ok := tools.CanonicalHandoffUnifiedResource(provider, handoffResource.ID, handoffResource.Name, handoffResource.Type, handoffResource.Node)
		if !ok {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(string(resource.Type)) + "\x00" + strings.TrimSpace(resource.ID))
		if key == "\x00" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, resource)
	}
	return out
}

func handoffResourceTimelineChangeKey(change unifiedresources.ResourceChange) string {
	if id := strings.TrimSpace(change.ID); id != "" {
		return id
	}
	return strings.ToLower(strings.TrimSpace(change.ResourceID) + "\x00" + string(change.Kind) + "\x00" + change.ObservedAt.UTC().Format(time.RFC3339Nano))
}

func buildHandoffActionContext(handoffActions []HandoffAction) string {
	actions := normalizeHandoffActions(handoffActions)
	if len(actions) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("[Action Context]")
	for idx, action := range actions {
		label := "Action Reference"
		if len(actions) > 1 {
			label = fmt.Sprintf("Action Reference %d", idx+1)
		}
		appendHandoffContextLine(&b, label+" Finding ID", action.FindingID)
		appendHandoffContextLine(&b, label+" Record ID", action.RecordID)
		appendHandoffContextLine(&b, label+" Approval ID", action.ApprovalID)
		appendHandoffContextLine(&b, label+" Approval Status", action.ApprovalStatus)
		appendHandoffContextLine(&b, label+" Approval Requested At", action.ApprovalRequestedAt)
		appendHandoffContextLine(&b, label+" Approval Expires At", action.ApprovalExpiresAt)
		appendHandoffContextLine(&b, label+" Approval Decided At", action.ApprovalDecidedAt)
		if action.ApprovalConsumed {
			appendHandoffContextLine(&b, label+" Approval Consumed", "true")
		}
		appendHandoffContextLine(&b, label+" Action ID", action.ActionID)
		appendHandoffContextLine(&b, label+" Action State", action.ActionState)
		appendHandoffContextLine(&b, label+" Action Updated At", action.ActionUpdatedAt)
		appendHandoffContextLine(&b, label+" Action Requested By", action.ActionRequestedBy)
		appendHandoffContextLine(&b, label+" Action Capability", action.ActionCapability)
		appendHandoffContextLine(&b, label+" Action Approval Policy", action.ActionApprovalPolicy)
		if action.ActionRequiresApproval {
			appendHandoffContextLine(&b, label+" Action Requires Approval", "true")
		}
		appendHandoffContextLine(&b, label+" Action Plan Expires At", action.ActionPlanExpiresAt)
		appendHandoffContextLine(&b, label+" Action Plan Message", action.ActionPlanMessage)
		appendHandoffContextLine(&b, label+" Action Preflight", action.ActionPreflight)
		appendHandoffContextLine(&b, label+" Action Dry Run Summary", action.ActionDryRunSummary)
		appendHandoffContextLine(&b, label+" Action Result", action.ActionResult)
		appendHandoffContextLine(&b, label+" Fix ID", action.FixID)
		appendHandoffContextLine(&b, label+" Proposed Fix", action.Description)
		appendHandoffContextLine(&b, label+" Risk", action.RiskLevel)
		if action.Destructive {
			appendHandoffContextLine(&b, label+" Destructive", "true")
		}
		appendHandoffContextLine(&b, label+" Target Host", action.TargetHost)
		appendHandoffContextLine(&b, label+" Target Resource", formatHandoffActionResource(action))
	}
	appendHandoffContextLine(&b, "Action Boundary", "Review and approval must stay in governed approval/remediation flows; this reference is not command authority and must not be used to infer raw command text.")
	return strings.TrimSpace(b.String())
}

func refreshHandoffActionApprovalStatus(handoffActions []HandoffAction, orgID string) []HandoffAction {
	return refreshHandoffActionStatus(handoffActions, orgID, nil)
}

func refreshHandoffActionStatus(handoffActions []HandoffAction, orgID string, actionStore unifiedresources.ResourceStore) []HandoffAction {
	actions := normalizeHandoffActions(handoffActions)
	if len(actions) == 0 {
		return nil
	}

	store := approval.GetStore()
	normalizedOrgID := approval.NormalizeOrgID(orgID)
	for idx := range actions {
		clearHandoffActionStatus(&actions[idx])
		approvalID := strings.TrimSpace(actions[idx].ApprovalID)
		if approvalID != "" && store != nil {
			req, ok := store.GetApproval(approvalID)
			if ok && approval.BelongsToOrg(req, normalizedOrgID) {
				HydrateHandoffActionFromApproval(&actions[idx], req)
			}
		}
		hydrateHandoffActionAudit(&actions[idx], actionStore)
	}
	return normalizeHandoffActions(actions)
}

func clearHandoffActionStatus(action *HandoffAction) {
	if action == nil {
		return
	}
	action.ApprovalStatus = ""
	action.ApprovalRequestedAt = ""
	action.ApprovalExpiresAt = ""
	action.ApprovalDecidedAt = ""
	action.ApprovalConsumed = false
	action.ActionState = ""
	action.ActionUpdatedAt = ""
	action.ActionRequestedBy = ""
	action.ActionCapability = ""
	action.ActionApprovalPolicy = ""
	action.ActionRequiresApproval = false
	action.ActionPlanExpiresAt = ""
	action.ActionPlanMessage = ""
	action.ActionPreflight = ""
	action.ActionDryRunSummary = ""
	action.ActionResult = ""
}

// HydrateHandoffActionFromApproval copies non-command approval metadata into an
// Assistant handoff action. It deliberately excludes ApprovalRequest.Command so
// Assistant context can explain governed state without becoming command replay.
func HydrateHandoffActionFromApproval(action *HandoffAction, req *approval.ApprovalRequest) {
	if action == nil || req == nil {
		return
	}
	if approvalID := strings.TrimSpace(req.ID); approvalID != "" {
		action.ApprovalID = approvalID
	}
	action.ApprovalStatus = strings.TrimSpace(string(req.Status))
	action.ApprovalRequestedAt = formatHandoffActionTime(req.RequestedAt)
	action.ApprovalExpiresAt = formatHandoffActionTime(req.ExpiresAt)
	if req.DecidedAt != nil {
		action.ApprovalDecidedAt = formatHandoffActionTime(*req.DecidedAt)
	}
	action.ApprovalConsumed = req.Consumed
	if requestedBy := strings.TrimSpace(approval.RequesterForRequest(req)); requestedBy != "" {
		action.ActionRequestedBy = requestedBy
	}
	if strings.TrimSpace(action.RiskLevel) == "" {
		action.RiskLevel = strings.TrimSpace(string(req.RiskLevel))
	}
	if strings.TrimSpace(action.TargetResourceID) == "" {
		action.TargetResourceID = strings.TrimSpace(req.TargetID)
	}
	if strings.TrimSpace(action.TargetResourceName) == "" {
		action.TargetResourceName = strings.TrimSpace(req.TargetName)
	}
	if strings.TrimSpace(action.TargetResourceType) == "" {
		action.TargetResourceType = strings.TrimSpace(req.TargetType)
	}
	hydrateHandoffActionPlan(action, req.Plan)
}

func hydrateHandoffActionPlan(action *HandoffAction, plan *unifiedresources.ActionPlan) {
	if action == nil || plan == nil {
		return
	}
	if strings.TrimSpace(action.ActionID) == "" {
		action.ActionID = strings.TrimSpace(plan.ActionID)
	}
	action.ActionRequiresApproval = plan.RequiresApproval
	action.ActionApprovalPolicy = strings.TrimSpace(string(plan.ApprovalPolicy))
	action.ActionPlanExpiresAt = formatHandoffActionTime(plan.ExpiresAt)
	action.ActionPlanMessage = strings.TrimSpace(plan.Message)
	if plan.Preflight != nil {
		action.ActionPreflight = strings.TrimSpace(plan.Preflight.IntendedChange)
		action.ActionDryRunSummary = strings.TrimSpace(plan.Preflight.DryRunSummary)
	}
}

func hydrateHandoffActionAudit(action *HandoffAction, store unifiedresources.ResourceStore) {
	if action == nil || store == nil {
		return
	}
	actionID := strings.TrimSpace(action.ActionID)
	if actionID == "" {
		return
	}
	record, ok, err := store.GetActionAudit(actionID)
	if err != nil {
		log.Debug().
			Err(err).
			Str("action_id", actionID).
			Msg("[ChatService] Failed to load handoff action audit")
		return
	}
	if !ok {
		return
	}

	action.ActionID = strings.TrimSpace(record.ID)
	action.ActionState = strings.TrimSpace(string(record.State))
	action.ActionUpdatedAt = formatHandoffActionTime(record.UpdatedAt)
	action.ActionRequestedBy = strings.TrimSpace(record.Request.RequestedBy)
	action.ActionCapability = strings.TrimSpace(record.Request.CapabilityName)
	if strings.TrimSpace(action.TargetResourceID) == "" {
		action.TargetResourceID = strings.TrimSpace(record.Request.ResourceID)
	}
	hydrateHandoffActionPlan(action, &record.Plan)
	if record.Result != nil {
		if record.Result.Success {
			action.ActionResult = "success"
		} else {
			action.ActionResult = "failed"
		}
	}
}

func formatHandoffActionTime(value time.Time) string {
	return formatHandoffContextTime(value)
}

func formatHandoffContextTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatHandoffResourceSources(resource unifiedresources.Resource) string {
	seen := make(map[string]struct{}, len(resource.Sources)+len(resource.SourceStatus))
	sources := make([]string, 0, len(resource.Sources)+len(resource.SourceStatus))
	for _, source := range resource.Sources {
		name := strings.TrimSpace(string(source))
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sources = append(sources, name)
	}
	for source := range resource.SourceStatus {
		name := strings.TrimSpace(string(source))
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sources = append(sources, name)
	}
	sort.Strings(sources)
	return strings.Join(sources, ", ")
}

func formatHandoffResourceSourceStatus(resource unifiedresources.Resource) string {
	if len(resource.SourceStatus) == 0 {
		return ""
	}

	sources := make([]string, 0, len(resource.SourceStatus))
	for source := range resource.SourceStatus {
		if name := strings.TrimSpace(string(source)); name != "" {
			sources = append(sources, name)
		}
	}
	sort.Strings(sources)

	parts := make([]string, 0, len(sources))
	for _, sourceName := range sources {
		status := resource.SourceStatus[unifiedresources.DataSource(sourceName)]
		state := strings.TrimSpace(status.Status)
		if state == "" && status.LastSeen.IsZero() {
			continue
		}
		if state == "" {
			state = "reported"
		}
		part := sourceName + ": " + state
		if lastSeen := formatHandoffContextTime(status.LastSeen); lastSeen != "" {
			part += " (last seen " + lastSeen + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

func formatHandoffResourceMetrics(metrics *unifiedresources.ResourceMetrics) string {
	if metrics == nil {
		return ""
	}
	values := []struct {
		label string
		value *unifiedresources.MetricValue
	}{
		{label: "cpu", value: metrics.CPU},
		{label: "memory", value: metrics.Memory},
		{label: "disk", value: metrics.Disk},
		{label: "network in", value: metrics.NetIn},
		{label: "network out", value: metrics.NetOut},
	}

	parts := make([]string, 0, len(values))
	for _, item := range values {
		if formatted := formatHandoffMetricValue(item.value); formatted != "" {
			parts = append(parts, item.label+" "+formatted)
		}
	}
	return strings.Join(parts, ", ")
}

func formatHandoffMetricValue(value *unifiedresources.MetricValue) string {
	if value == nil {
		return ""
	}
	if value.Percent > 0 {
		return trimHandoffFloat(value.Percent) + "%"
	}
	unit := strings.TrimSpace(value.Unit)
	if value.Value != 0 {
		if unit != "" {
			return trimHandoffFloat(value.Value) + " " + unit
		}
		return trimHandoffFloat(value.Value)
	}
	if value.Used != nil && value.Total != nil && *value.Total > 0 {
		return fmt.Sprintf("%d/%d", *value.Used, *value.Total)
	}
	return ""
}

func trimHandoffFloat(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", value), "0"), ".")
}

func formatHandoffResourceParent(resource unifiedresources.Resource, policy *unifiedresources.ResourcePolicy) string {
	parent := strings.TrimSpace(resource.ParentName)
	if parent != "" {
		return unifiedresources.ResourcePolicyRedactedValue(parent, policy,
			unifiedresources.ResourceRedactionHostname,
			unifiedresources.ResourceRedactionAlias,
			unifiedresources.ResourceRedactionPlatformID,
		)
	}
	parentID := ""
	if resource.ParentID != nil {
		parentID = strings.TrimSpace(*resource.ParentID)
	}
	if parentID == "" {
		return ""
	}
	return unifiedresources.ResourcePolicyRedactedValue(parentID, policy,
		unifiedresources.ResourceRedactionAlias,
		unifiedresources.ResourceRedactionPlatformID,
		unifiedresources.ResourceRedactionPath,
	)
}

func formatHandoffResourceID(resource unifiedresources.Resource, policy *unifiedresources.ResourcePolicy) string {
	return unifiedresources.ResourcePolicyRedactedValue(resource.ID, policy,
		unifiedresources.ResourceRedactionAlias,
		unifiedresources.ResourceRedactionHostname,
		unifiedresources.ResourceRedactionPlatformID,
		unifiedresources.ResourceRedactionPath,
	)
}

func formatHandoffResourceIncidentSummary(resource unifiedresources.Resource, policy *unifiedresources.ResourcePolicy) string {
	parts := make([]string, 0, 4)
	if resource.IncidentCount > 0 {
		parts = append(parts, fmt.Sprintf("count %d", resource.IncidentCount))
	}
	if severity := strings.TrimSpace(string(resource.IncidentSeverity)); severity != "" {
		parts = append(parts, "severity "+severity)
	}
	if summary := strings.TrimSpace(resource.IncidentSummary); summary != "" {
		if safeSummary := formatHandoffResourceFreeText(summary, resource, policy); safeSummary != "" {
			parts = append(parts, safeSummary)
		}
	}
	if action := strings.TrimSpace(resource.IncidentAction); action != "" {
		if safeAction := formatHandoffResourceFreeText(action, resource, policy); safeAction != "" {
			parts = append(parts, "action "+safeAction)
		}
	}
	for idx, incident := range resource.Incidents {
		if idx >= 2 {
			break
		}
		summary := strings.TrimSpace(incident.Summary)
		if summary == "" && strings.TrimSpace(incident.Code) == "" {
			continue
		}
		item := formatHandoffResourceFreeText(summary, resource, policy)
		if item == "" {
			item = strings.TrimSpace(incident.Code)
		} else if code := strings.TrimSpace(incident.Code); code != "" {
			item += " (" + code + ")"
		}
		if severity := strings.TrimSpace(string(incident.Severity)); severity != "" {
			item += " severity " + severity
		}
		parts = append(parts, item)
	}
	return strings.Join(parts, "; ")
}

func formatHandoffResourceCapabilities(resource unifiedresources.Resource, policy *unifiedresources.ResourcePolicy, limit int) string {
	if len(resource.Capabilities) == 0 {
		return ""
	}
	if limit <= 0 {
		limit = len(resource.Capabilities)
	}

	seen := make(map[string]struct{}, len(resource.Capabilities))
	parts := make([]string, 0, min(limit, len(resource.Capabilities)))
	for _, capability := range resource.Capabilities {
		name := strings.TrimSpace(capability.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name + "\x00" + strings.TrimSpace(string(capability.Type)))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		qualifiers := make([]string, 0, 2)
		if capabilityType := strings.TrimSpace(string(capability.Type)); capabilityType != "" {
			qualifiers = append(qualifiers, capabilityType)
		}
		if approvalLevel := strings.TrimSpace(string(capability.MinimumApprovalLevel)); approvalLevel != "" {
			qualifiers = append(qualifiers, "approval "+approvalLevel)
		}

		part := name
		if len(qualifiers) > 0 {
			part += " (" + strings.Join(qualifiers, "; ") + ")"
		}
		if description := strings.TrimSpace(capability.Description); description != "" {
			if safeDescription := formatHandoffResourceFreeText(description, resource, policy); safeDescription != "" {
				part += ": " + safeDescription
			}
		}
		parts = append(parts, part)
		if len(parts) >= limit {
			break
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if remaining := len(resource.Capabilities) - len(parts); remaining > 0 {
		parts = append(parts, fmt.Sprintf("%d more", remaining))
	}
	return strings.Join(parts, "; ")
}

func formatHandoffResourceFreeText(value string, resource unifiedresources.Resource, policy *unifiedresources.ResourcePolicy) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	redacted := strings.TrimSpace(unifiedresources.ResourcePolicyRedactedText(value, resource))
	if redacted == "" {
		return ""
	}
	if unifiedresources.ResourcePolicyRequiresGovernedSummary(policy) && redacted == value {
		return ""
	}
	return redacted
}

func appendHandoffContextLine(b *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteByte('\n')
	}
	b.WriteString(label)
	b.WriteString(": ")
	b.WriteString(value)
}

func formatHandoffActionResource(action HandoffAction) string {
	resourceName := strings.TrimSpace(action.TargetResourceName)
	resourceType := strings.TrimSpace(action.TargetResourceType)
	resourceID := strings.TrimSpace(action.TargetResourceID)
	node := strings.TrimSpace(action.TargetNode)

	parts := make([]string, 0, 4)
	if resourceName != "" {
		parts = append(parts, resourceName)
	}
	if resourceType != "" {
		parts = append(parts, resourceType)
	}
	if resourceID != "" {
		parts = append(parts, resourceID)
	}
	if node != "" {
		parts = append(parts, "node "+node)
	}
	return strings.Join(parts, " / ")
}

func (s *Service) hydrateHandoffResources(sessionID string, handoffResources []HandoffResource, sessions *SessionStore, provider tools.UnifiedResourceProvider) {
	if len(handoffResources) == 0 || sessions == nil || provider == nil {
		return
	}

	resolvedCtx := sessions.GetResolvedContext(sessionID)
	seen := make(map[string]struct{}, len(handoffResources))
	for _, resource := range handoffResources {
		key := strings.ToLower(strings.TrimSpace(resource.Type) + "\x00" + strings.TrimSpace(resource.ID) + "\x00" + strings.TrimSpace(resource.Name))
		if key == "\x00\x00" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		reg, ok := tools.CanonicalHandoffResourceRegistration(provider, resource.ID, resource.Name, resource.Type, resource.Node)
		if !ok {
			log.Debug().
				Str("session_id", sessionID).
				Str("resource_id", resource.ID).
				Str("resource_name", resource.Name).
				Str("resource_type", resource.Type).
				Msg("[ChatService] Skipped unresolved handoff resource")
			continue
		}
		resolvedCtx.AddResolvedResource(reg)
		if resolved, ok := resolvedCtx.GetResolvedResourceByAlias(reg.Name); ok {
			resolvedCtx.MarkExplicitAccess(resolved.GetResourceID())
			log.Debug().
				Str("session_id", sessionID).
				Str("resource_id", resolved.GetResourceID()).
				Str("resource_name", reg.Name).
				Msg("[ChatService] Hydrated handoff resource into resolved context")
		}
	}
}

// PatrolRequest represents a patrol execution request within the chat service
type PatrolRequest struct {
	Prompt       string `json:"prompt"`
	SystemPrompt string `json:"system_prompt"`
	SessionID    string `json:"session_id,omitempty"`
	ExecutionID  string `json:"execution_id,omitempty"`
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
	baseExecutor := s.executor
	unifiedResourceProvider := s.unifiedResourceProvider
	cfg := s.cfg
	effectiveControlLevel := s.effectiveControlLevelLocked()
	s.mu.RUnlock()
	executor := baseExecutor
	if baseExecutor != nil {
		executor = baseExecutor.Clone()
		executor.SetControlLevel(effectiveControlLevel)
	}

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
	provider, err := s.createPatrolProviderForModel(patrolModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create patrol provider: %w", err)
	}

	// Create a temporary agentic loop with the patrol system prompt
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = s.buildSystemPrompt()
	}
	tempLoop := NewAgenticLoop(provider, executor, systemPrompt)
	tempLoop.SetOrgID(s.orgID)
	tempLoop.SetAutonomousMode(true) // Patrol runs without approval prompts
	tempLoop.SetExecutionID(req.ExecutionID)
	tempLoop.SetRequestSanitizer(modelboundary.RequestSanitizerForModel(patrolModel, unifiedResourceProvider))
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

	// Add user message to the session for forensics / audit trail.
	userMsg := Message{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   req.Prompt,
		Timestamp: time.Now(),
	}
	if err := sessions.AddMessage(session.ID, userMsg); err != nil {
		log.Warn().Err(err).Msg("failed to save patrol user message")
	}

	// Patrol runs are stateless investigations. The "patrol-main" session
	// reuses the same id across scheduled runs, so loading prior session
	// history into the agentic loop accumulates broken state: when any
	// run ends after the model emitted tool_calls but before all tool
	// results landed (provider error, timeout, context cancellation),
	// the orphan tool_calls persist and every subsequent run hits
	// "An assistant message with 'tool_calls' must be followed by tool
	// messages responding to each 'tool_call_id'." Patrol must see only
	// this run's user prompt; the session is just a forensic log.
	messages := []Message{userMsg}

	// Get all tools (patrol runs in autonomous mode)
	filteredTools := s.filterToolsForPrompt(ctx, req.Prompt, true, true)

	// Run the agentic loop
	resultMessages, err := tempLoop.ExecuteWithTools(ctx, session.ID, messages, filteredTools, callback)
	if err != nil {
		// Still save any messages we got
		for _, msg := range resultMessages {
			if saveErr := sessions.AddMessage(session.ID, msg); saveErr != nil {
				log.Warn().Err(saveErr).Msg("failed to save patrol message after error")
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
			log.Warn().Err(err).Msg("failed to save patrol message")
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
	case "openrouter":
		if s.cfg.OpenRouterAPIKey == "" {
			return nil, fmt.Errorf("OpenRouter API key not configured")
		}
		return providers.NewOpenAIClient(s.cfg.OpenRouterAPIKey, modelName, s.cfg.GetBaseURLForProvider(config.AIProviderOpenRouter), timeout), nil
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
		baseURL := s.cfg.GetBaseURLForProvider(config.AIProviderOllama)
		return providers.NewOllamaClient(modelName, baseURL, s.cfg.OllamaUsername, s.cfg.OllamaPassword, timeout)
	case config.AIProviderQuickstart:
		return nil, fmt.Errorf("quickstart provider is retired; configure a provider API key or Ollama")
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}

func (s *Service) createPatrolProviderForModel(modelStr string) (providers.StreamingProvider, error) {
	if s.patrolProviderFactory != nil {
		return s.patrolProviderFactory(modelStr)
	}
	return s.createProviderForModel(modelStr)
}

// ListAvailableTools returns tool names available for the given prompt.
func (s *Service) ListAvailableTools(ctx context.Context, prompt string) []string {
	s.mu.RLock()
	executor := s.executor
	s.mu.RUnlock()

	if executor == nil {
		return nil
	}

	tools := s.filterToolsForPrompt(ctx, prompt, s.isAutonomousModeEnabled(), false)
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
	actionAuditStore := s.actionAuditStore
	orgID := s.orgID
	s.mu.RUnlock()

	if sessions == nil {
		return nil, fmt.Errorf("service not started")
	}

	refreshSessionHandoffActionSummaries(sessions, orgID, actionAuditStore)
	return sessions.List()
}

func refreshSessionHandoffActionSummaries(sessions *SessionStore, orgID string, actionStore unifiedresources.ResourceStore) {
	if sessions == nil {
		return
	}

	list, err := sessions.List()
	if err != nil {
		log.Debug().Err(err).Msg("[ChatService] Failed to list sessions for handoff action refresh")
		return
	}

	for _, session := range list {
		if session.HandoffSummary == nil || session.HandoffSummary.ActionCount == 0 {
			continue
		}
		actions, err := sessions.GetModelHandoffActions(session.ID)
		if err != nil {
			log.Debug().Err(err).Str("session_id", session.ID).Msg("[ChatService] Failed to load session handoff actions for summary refresh")
			continue
		}
		refreshed := refreshHandoffActionStatus(actions, orgID, actionStore)
		if handoffActionsEqual(actions, refreshed) {
			continue
		}
		if err := sessions.SetModelHandoffActions(session.ID, refreshed); err != nil {
			log.Debug().Err(err).Str("session_id", session.ID).Msg("[ChatService] Failed to persist refreshed session handoff action summary")
		}
	}
}

func handoffActionsEqual(a, b []HandoffAction) bool {
	a = normalizeHandoffActions(a)
	b = normalizeHandoffActions(b)
	if len(a) != len(b) {
		return false
	}
	for idx := range a {
		if a[idx] != b[idx] {
			return false
		}
	}
	return true
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

// GetModelHandoffFindingID returns the session-scoped finding reference used to
// refresh model-only Patrol context on follow-up turns.
func (s *Service) GetModelHandoffFindingID(ctx context.Context, sessionID string) (string, error) {
	s.mu.RLock()
	sessions := s.sessions
	s.mu.RUnlock()

	if sessions == nil {
		return "", fmt.Errorf("service not started")
	}

	return sessions.GetModelHandoffFindingID(sessionID)
}

// GetModelHandoffMetadata returns the session-scoped product handoff identity
// used to refresh model-only Patrol context on follow-up turns.
func (s *Service) GetModelHandoffMetadata(ctx context.Context, sessionID string) (HandoffMetadata, error) {
	s.mu.RLock()
	sessions := s.sessions
	s.mu.RUnlock()

	if sessions == nil {
		return HandoffMetadata{}, fmt.Errorf("service not started")
	}

	return sessions.GetModelHandoffMetadata(sessionID)
}

// GetActionAuditStore returns the durable action-audit store used by the chat runtime.
func (s *Service) GetActionAuditStore() unifiedresources.ResourceStore {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.actionAuditStore
}

// ClearModelHandoffContext invalidates product-originated model-only handoff
// state after its source record can no longer be resolved. Unpinned resolved
// resources are cleared with it so stale Patrol handoffs cannot remain action
// scope on follow-up turns.
func (s *Service) ClearModelHandoffContext(ctx context.Context, sessionID string) error {
	s.mu.RLock()
	sessions := s.sessions
	s.mu.RUnlock()

	if sessions == nil {
		return fmt.Errorf("service not started")
	}

	err := sessions.ClearModelHandoffContext(sessionID)
	sessions.ClearSessionState(sessionID, true)
	return err
}

// AbortSession aborts an ongoing session
func (s *Service) AbortSession(ctx context.Context, sessionID string) error {
	s.mu.RLock()
	started := s.started
	agenticLoop := s.agenticLoop
	s.mu.RUnlock()

	if !started {
		return fmt.Errorf("service not started")
	}

	activeLoops := s.getActiveLoops(sessionID)
	if len(activeLoops) > 0 {
		for _, loop := range activeLoops {
			loop.Abort(sessionID)
		}
		return nil
	}

	if agenticLoop == nil {
		return fmt.Errorf("service not started")
	}

	agenticLoop.Abort(sessionID)
	return nil
}

// AnswerQuestion provides answers to a pending question
func (s *Service) AnswerQuestion(ctx context.Context, questionID string, answers []QuestionAnswer) error {
	s.mu.RLock()
	started := s.started
	agenticLoop := s.agenticLoop
	s.mu.RUnlock()

	if !started {
		return fmt.Errorf("service not started")
	}

	if loop := s.findQuestionLoop(questionID); loop != nil {
		return loop.AnswerQuestion(questionID, answers)
	}

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

func (s *Service) SetAppContainerConfigProvider(provider MCPAppContainerConfigProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetAppContainerConfigProvider(provider)
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
	s.unifiedResourceProvider = provider
	if s.executor != nil {
		s.executor.SetUnifiedResourceProvider(provider)
	}
}

func (s *Service) SetAppContainerActionProvider(provider MCPAppContainerActionProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetAppContainerActionProvider(provider)
	}
}

func (s *Service) SetAppContainerReadProvider(provider MCPAppContainerReadProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetAppContainerReadProvider(provider)
	}
}

func (s *Service) SetDiscoveryProvider(provider MCPDiscoveryProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executor != nil {
		s.executor.SetDiscoveryProvider(provider)
	}
	// Create/update context prefetcher with the discovery provider
	if s.readState != nil && provider != nil {
		s.contextPrefetcher = NewContextPrefetcher(s.readState, provider)
		log.Info().Msg("[ChatService] Context prefetcher created with discovery provider")
	} else {
		log.Warn().
			Bool("hasReadState", s.readState != nil).
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
	s.cfg = cfg
	if s.executor != nil {
		s.executor.SetControlLevel(s.effectiveControlLevelLocked())
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
		if n, err := fmt.Sscanf(resultText, "Command failed (exit code %d):", &code); n != 1 || err != nil {
			// Some providers omit the trailing colon in this message shape.
			if n, err := fmt.Sscanf(resultText, "Command failed (exit code %d)", &code); n != 1 || err != nil {
				log.Debug().Str("result_text", resultText).Msg("failed to parse command exit code, defaulting to 1")
				code = 1
			}
		}
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

` + s.buildToolGovernancePromptSection() + `

## INFRASTRUCTURE TOPOLOGY
- Resources are organized hierarchically: nodes → VMs/containers → Docker containers
- target_host specifies where commands run (host name, container name, or VM name)
- Commands execute inside the target: target_host="homepage-docker" runs inside that container
- For Docker containers inside system containers: target the container, then use docker commands

## DOCKER BIND MOUNTS
- Container files are often mapped to host paths via bind mounts
- To edit a container's config, find the bind mount and edit the host path
- Use pulse_discovery to find bind mount mappings

## TOOL SELECTION
- Tool action modes and approval policies are generated from Pulse's tool registry. Treat that manifest as the source of truth for whether a tool is read-only, mixed, or write-capable.
- pulse_control and pulse_file_edit are WRITE tools — they change infrastructure state.
- pulse_docker, pulse_kubernetes, pulse_alerts, and pulse_knowledge are MIXED tools — their read subactions are safe, but their write or decision-recording subactions require the governed path described in the manifest.
- Not every VM or container supports control. Some API-backed platforms are read-only even when the resource type is "vm" or "system-container".
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
- After successful control actions, the system auto-verifies. Once verified, stop making tool calls and respond.
- If a tool call is BLOCKED, read the error message carefully and follow its instructions exactly.
- If told to call pulse_query or pulse_read first, you MUST do that before retrying the blocked action.`
}

func (s *Service) buildToolGovernancePromptSection() string {
	manifest := []tools.ToolGovernanceDescriptor(nil)
	if s != nil && s.executor != nil {
		manifest = s.executor.ListToolGovernance()
	}
	if len(manifest) == 0 {
		manifest = fallbackAssistantToolGovernance()
	}

	var b strings.Builder
	b.WriteString("## AVAILABLE TOOL GOVERNANCE\n")
	b.WriteString("This manifest is generated from Pulse's governed tool registry. Use only tools that are actually offered by the provider for the current turn.\n")
	for _, tool := range manifest {
		mode := tool.ActionMode
		if mode == "" {
			mode = tools.ToolActionRead
		}
		policy := strings.TrimSpace(tool.ApprovalPolicy)
		if policy == "" {
			policy = "no approval required"
		}
		summary := strings.TrimSpace(tool.Summary)
		if summary == "" {
			summary = firstPromptLine(tool.Description)
		}
		if summary != "" {
			b.WriteString(fmt.Sprintf("- %s: mode=%s; approval=%s; %s\n", tool.Name, mode, policy, summary))
		} else {
			b.WriteString(fmt.Sprintf("- %s: mode=%s; approval=%s\n", tool.Name, mode, policy))
		}
	}
	b.WriteString("- pulse_question: mode=interactive; approval=user answer required; asks the user for missing information using a structured prompt in interactive chat only.\n")
	return strings.TrimRight(b.String(), "\n")
}

func fallbackAssistantToolGovernance() []tools.ToolGovernanceDescriptor {
	return []tools.ToolGovernanceDescriptor{
		{
			Name:           "pulse_query",
			ActionMode:     tools.ToolActionRead,
			ApprovalPolicy: "no approval required",
			Summary:        "Resolves canonical infrastructure identity, topology, config, and health without changing state.",
		},
		{
			Name:           "pulse_read",
			ActionMode:     tools.ToolActionRead,
			ApprovalPolicy: "no approval required; write-like commands are rejected",
			Summary:        "Runs read-only infrastructure inspection such as logs, file reads, tails, and safe exec.",
		},
		{
			Name:           "pulse_discovery",
			ActionMode:     tools.ToolActionRead,
			ApprovalPolicy: "no approval required",
			Summary:        "Reads discovered service details, config paths, ports, and bind mounts.",
		},
		{
			Name:           "pulse_control",
			ActionMode:     tools.ToolActionWrite,
			ApprovalPolicy: "hidden in read-only mode; approval required in controlled mode",
			Summary:        "Runs shared Pulse control actions and can control only resources that explicitly support shared Pulse actions.",
		},
		{
			Name:           "pulse_file_edit",
			ActionMode:     tools.ToolActionWrite,
			ApprovalPolicy: "hidden in read-only mode; approval required in controlled mode",
			Summary:        "Changes files through the governed file-edit path.",
		},
		{
			Name:           "pulse_docker",
			ActionMode:     tools.ToolActionMixed,
			ApprovalPolicy: "read/list actions are safe; control and update subactions require approval in controlled mode",
			Summary:        "Lists Docker state and performs governed Docker control/update subactions.",
		},
		{
			Name:           "pulse_kubernetes",
			ActionMode:     tools.ToolActionMixed,
			ApprovalPolicy: "read/list/log actions are safe; scale, restart, delete, and exec subactions require approval in controlled mode",
			Summary:        "Reads Kubernetes topology and runs governed workload-control subactions.",
		},
	}
}

func firstPromptLine(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return ""
	}
	if idx := strings.Index(description, "\n"); idx >= 0 {
		description = strings.TrimSpace(description[:idx])
	}
	return strings.Join(strings.Fields(description), " ")
}

var recentContextPronounPattern = regexp.MustCompile(`(?i)\b(it|its|that|those|this|them|previous|earlier|last|same|former|latter)\b`)
var recentContextNounPattern = regexp.MustCompile(`(?i)\b(the (service|container|vm|node|host|docker|instance|one))\b`)

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
	primaryResourceID := ""
	primaryTarget := ""
	primaryKind := ""
	primaryAdapter := ""
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
			primaryResourceID = res.ResourceID
			primaryTarget = res.TargetHost
			primaryKind = kind
			primaryAdapter = res.Adapter
			if primaryName == "" {
				primaryName = label
			}
			if primaryResourceID == "" {
				primaryResourceID = resourceID
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
	readHint := readRoutingHintForResolvedResource(primaryKind, primaryAdapter, primaryTarget, primaryName, primaryResourceID)
	targetHint := readHint.targetHintSuffix()
	summary := fmt.Sprintf("Context: The most recently referenced resource is %s. If the user says \"it/its/that\", assume they mean this resource unless they specify otherwise. Do not ask for clarification unless the user names a different resource.%s", primary, targetHint)
	if len(lines) > 1 {
		others := strings.Join(lines[1:], "\n")
		summary += "\nOther recent resources:\n" + others
	}

	lowerPrompt := strings.ToLower(prompt)
	if strings.Contains(lowerPrompt, "log") || strings.Contains(lowerPrompt, "journal") {
		instructionTarget := primaryName
		if instructionTarget == "" {
			instructionTarget = primary
		}
		if instruction := readHint.recentLogsInstruction(instructionTarget); instruction != "" {
			summary += "\n" + instruction
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

func (s *Service) filterToolsForPrompt(ctx context.Context, prompt string, autonomousMode bool, patrolMode bool) []providers.Tool {
	mcpTools := s.executor.ListTools()
	providerTools := ConvertMCPToolsToProvider(mcpTools)

	// For patrol (autonomous mode), use config flags instead of keyword detection.
	// Patrol seed context can mention all resource types, which defeats keyword filtering.
	if patrolMode {
		filtered := s.filterToolsForPatrol(providerTools)

		// Keep write-intent gating for autonomous runs.
		if !hasWriteIntent(convertPromptToMessages(prompt)) {
			nonWrite := make([]providers.Tool, 0, len(filtered))
			for _, tool := range filtered {
				if !isWriteTool(tool.Name) {
					nonWrite = append(nonWrite, tool)
				}
			}
			filtered = nonWrite
		}

		log.Debug().
			Int("total_tools", len(providerTools)).
			Int("filtered_tools", len(filtered)).
			Bool("autonomous_patrol_filter", true).
			Msg("[filterToolsForPrompt] Filtered tools for patrol using config flags")
		return filtered
	}

	// Filter out write/control tools when the user's request is read-only.
	// This prevents models from calling pulse_control (restart, stop, etc.) when
	// the user only asked for status, logs, or monitoring information.
	//
	// The tool set is determined once per user message and stays consistent for
	// the entire agentic loop. This avoids the old problem of tools appearing/
	// disappearing mid-conversation (which caused hallucinated tool names).
	//
	// Keep write-intent gating only for autonomous runs where commands execute
	// without approval. In interactive chat, always include write tools; safety
	// is enforced by approval flow, FSM gates, and tool-level policy checks.
	readOnly := autonomousMode && !hasWriteIntent(convertPromptToMessages(prompt))

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

// filterToolsForPatrol filters tools for patrol runs using AI config flags
// instead of keyword-based detection. This prevents the seed context
// (which mentions all resource types) from causing all tools to be included.
func (s *Service) filterToolsForPatrol(providerTools []providers.Tool) []providers.Tool {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	// Determine which subsystems are enabled for patrol.
	includeDocker := cfg != nil && cfg.PatrolAnalyzeDocker
	includeStorage := cfg != nil && cfg.PatrolAnalyzeStorage
	// K8s and PMG don't have dedicated top-level patrol flags.
	includeK8s := true
	includePMG := true

	filtered := make([]providers.Tool, 0, len(providerTools))
	for _, tool := range providerTools {
		switch tool.Name {
		case "pulse_docker":
			if !includeDocker {
				continue
			}
		case "pulse_storage":
			if !includeStorage {
				continue
			}
		case "pulse_kubernetes":
			if !includeK8s {
				continue
			}
		case "pulse_pmg":
			if !includePMG {
				continue
			}
		}
		filtered = append(filtered, tool)
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
