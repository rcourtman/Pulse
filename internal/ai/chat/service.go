package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentcontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/modelboundary"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/modelresolution"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
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

	// Optional report-narration providers for the pulse_summarize tool.
	// When the per-tenant AI service is configured the API layer passes
	// it here for all three roles; when unconfigured the tool returns
	// heuristic narrative.
	ReportNarrator         reporting.Narrator
	ReportFleetNarrator    reporting.FleetNarrator
	ReportFindingsProvider reporting.FindingsProvider

	// Optional cost store. When set, ExecuteStream records a
	// cost.UsageEvent after each chat turn so user-chat token usage
	// appears in the operator dashboard alongside patrol, discovery,
	// and report-narrative spend. Absent value means "don't record"
	// — useful for tests and for callers that bring their own ledger.
	// ExecutePatrolStream intentionally does NOT record here; its
	// caller (patrol_ai.go) records via its own helper so cost is
	// never double-counted on the patrol-via-chat path.
	CostStore *cost.Store
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
	discoveryProvider       tools.DiscoveryProvider
	budgetChecker           func() error // Optional mid-run budget enforcement
	orgID                   string
	controlLevelResolver    func(*config.AIConfig) string

	// costStore receives a cost.UsageEvent after each ExecuteStream
	// turn so user-chat token usage is visible in the operator
	// dashboard. Nil means "don't record".
	costStore *cost.Store

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
		ReportNarrator:         cfg.ReportNarrator,
		ReportFleetNarrator:    cfg.ReportFleetNarrator,
		ReportFindingsProvider: cfg.ReportFindingsProvider,
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
		costStore:            cfg.CostStore,
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

// recordChatTurnCost records a cost.UsageEvent for a completed (or
// failed) user-chat agentic turn. Uses the totals accumulated by the
// loop's stream callbacks so the recorded numbers match what the
// frontend sees in the SSE done event. No-op when the cost store is
// not configured or when the loop consumed zero tokens (an early
// failure before the first turn completed). UseCase is "chat" — the
// canonical taxonomy noted on cost.UsageEvent.UseCase.
func (s *Service) recordChatTurnCost(loop *AgenticLoop, requestModel string) {
	if s == nil || loop == nil {
		return
	}
	s.mu.RLock()
	store := s.costStore
	s.mu.RUnlock()
	if store == nil {
		return
	}
	inputTokens := loop.GetTotalInputTokens()
	outputTokens := loop.GetTotalOutputTokens()
	if inputTokens == 0 && outputTokens == 0 {
		return
	}
	providerName := ""
	if requestModel != "" {
		if idx := strings.Index(requestModel, ":"); idx > 0 {
			providerName = strings.ToLower(requestModel[:idx])
		}
	}
	store.Record(cost.UsageEvent{
		Timestamp:    time.Now(),
		Provider:     providerName,
		RequestModel: requestModel,
		UseCase:      "chat",
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	})
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

	streamCallback := callback
	if streamCallback == nil {
		streamCallback = func(StreamEvent) {}
	}
	if !req.SuppressSessionEvent {
		sessionData, _ := json.Marshal(SessionData{ID: session.ID})
		streamCallback(StreamEvent{Type: "session", Data: sessionData})
		emitWorkflowState(streamCallback, "prepare", "Preparing Pulse context.", "", "")
	}

	handoffContext := strings.TrimSpace(req.HandoffContext)
	handoffResources := normalizeHandoffResources(req.HandoffResources)
	handoffActions := normalizeHandoffActions(req.HandoffActions)
	handoffMetadata := NormalizeHandoffMetadata(req.HandoffMetadata)
	handoffFindingID := strings.TrimSpace(req.FindingID)
	handoffMetadataCarriesEnvelope := !handoffMetadataEmpty(handoffMetadata)
	if handoffMetadata.Kind == sessionHandoffKindResourceContext &&
		handoffFindingID == "" &&
		handoffContext == "" &&
		len(handoffResources) == 0 &&
		len(handoffActions) == 0 {
		handoffMetadataCarriesEnvelope = false
	}
	hasRequestHandoffEnvelope := handoffFindingID != "" ||
		handoffContext != "" ||
		len(handoffResources) > 0 ||
		len(handoffActions) > 0 ||
		handoffMetadataCarriesEnvelope
	if hasRequestHandoffEnvelope {
		if err := sessions.SetModelHandoffEnvelope(session.ID, handoffFindingID, handoffContext, handoffResources, handoffActions, handoffMetadata); err != nil {
			log.Warn().Err(err).Str("session_id", session.ID).Msg("[ChatService] Failed to persist model handoff envelope")
		}
	} else {
		storedContext, storedResources, storedActions, storedMetadata, err := sessions.GetModelHandoffEnvelope(session.ID)
		if err != nil {
			log.Warn().Err(err).Str("session_id", session.ID).Msg("[ChatService] Failed to load model handoff envelope")
		} else {
			handoffContext = storedContext
			handoffResources = storedResources
			handoffActions = storedActions
			handoffMetadata = NormalizeHandoffMetadata(storedMetadata)
		}
	}

	// Determine which model/provider attempts to use for this request before
	// persisting the turn, so session history can restore the same model route.
	cfgSnapshot := (*config.AIConfig)(nil)
	configuredModel := ""
	overrideModel := strings.TrimSpace(req.Model)
	var baseExecutor *tools.PulseToolExecutor
	var configuredProvider providers.StreamingProvider
	autonomousMode := false
	effectiveControlLevel := tools.ControlLevelReadOnly
	s.mu.RLock()
	cfgSnapshot = s.cfg
	baseExecutor = s.executor
	configuredProvider = s.provider
	unifiedResourceProvider := s.unifiedResourceProvider
	autonomousMode = s.autonomousMode
	effectiveControlLevel = s.effectiveControlLevelLocked()
	s.mu.RUnlock()

	if cfgSnapshot != nil {
		if resolved, resolveErr := modelresolution.ResolveConfiguredChatModelOffline(cfgSnapshot); resolveErr == nil {
			configuredModel = strings.TrimSpace(resolved)
		} else {
			configuredModel = strings.TrimSpace(cfgSnapshot.GetChatModel())
			log.Warn().Err(resolveErr).Msg("[ChatService] Unable to resolve configured chat model")
		}
	}
	selectedModel := configuredModel
	if overrideModel != "" {
		selectedModel = overrideModel
	}

	// Add user message
	userMsg := Message{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   req.Prompt,
		Model:     selectedModel,
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

	if streamMockAssistantTurnIfEnabled(ctx, sessions, session.ID, req.Prompt, streamCallback) {
		return nil
	}

	handoffActions = refreshHandoffActionStatus(handoffActions, s.orgID, s.actionAuditStore)
	if len(handoffActions) > 0 {
		if err := sessions.SetModelHandoffActions(session.ID, handoffActions); err != nil {
			log.Warn().Err(err).Str("session_id", session.ID).Msg("[ChatService] Failed to persist refreshed handoff action status")
		}
	}
	s.mu.RLock()
	handoffResourceProvider := s.unifiedResourceProvider
	s.mu.RUnlock()
	handoffContext = mergeResourceContextHandoffDirective(handoffContext, handoffResources, handoffMetadata)
	handoffContext = mergeHandoffResourceContextPack(handoffContext, handoffResources, handoffResourceProvider, s.actionAuditStore, time.Now())
	handoffContext = mergeHandoffResourcePolicyContext(handoffContext, handoffResources, handoffResourceProvider)
	handoffContext = mergeHandoffResourceStateContext(handoffContext, handoffResources, handoffResourceProvider)
	handoffContext = mergeHandoffResourceRelationshipContext(handoffContext, handoffResources, handoffResourceProvider)
	handoffContext = mergeHandoffResourceTimelineContext(handoffContext, handoffResources, handoffResourceProvider, s.actionAuditStore, time.Now())
	handoffContext = mergeHandoffActionContext(handoffContext, handoffActions)
	handoffContext = sanitizeHandoffContextForResourcePolicy(handoffContext, handoffResources, handoffResourceProvider)
	injectHandoffContextIntoLatestUserMessage(messages, handoffContext)

	s.hydrateHandoffResources(session.ID, handoffResources, sessions, unifiedResourceProvider)

	// Per-request autonomous mode override (used by investigation to avoid
	// mutating shared service state from concurrent goroutines).
	if req.AutonomousMode != nil {
		autonomousMode = *req.AutonomousMode
	}
	effectiveControlLevel = controlLevelForRequestAutonomousMode(effectiveControlLevel, req.AutonomousMode)
	hasModelHandoffContext := handoffContext != "" ||
		len(handoffResources) > 0 ||
		len(handoffActions) > 0 ||
		handoffFindingID != "" ||
		!handoffMetadataEmpty(handoffMetadata)
	assistantToolScope := assistantToolScopeForPrompt(req.Prompt, hasModelHandoffContext)

	// Proactively gather context for mentioned resources
	s.mu.RLock()
	prefetcher := s.contextPrefetcher
	readState := s.readState
	s.mu.RUnlock()

	log.Info().
		Bool("hasPrefetcher", prefetcher != nil).
		Str("prompt", req.Prompt[:min(50, len(req.Prompt))]).
		Msg("[ChatService] Checking prefetcher")

	mentionsFound := false
	var modelBoundaryAllowedCloudContext []string
	if prefetcher != nil {
		cloudContextPolicy := CloudContextPolicy{
			CloudRouting: modelboundary.ModelUsesExternalProvider(selectedModel),
		}
		// Route product-originated handoff resources through the same prefetch path
		// as explicit @-mentions so a drawer "Ask Assistant" handoff delivers the
		// resource's cloud-safe operational context to the model, not just the
		// structural handoff context pack.
		prefetchMentions := mentionsIncludingHandoff(req.Mentions, handoffResources)
		prefetchCtx := prefetcher.PrefetchWithCloudPolicy(ctx, req.Prompt, prefetchMentions, cloudContextPolicy)
		if prefetchCtx != nil {
			mentionsFound = len(prefetchCtx.Mentions) > 0
			modelBoundaryAllowedCloudContext = prefetchCtx.CloudSafeContextSpans
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
	}
	// Plain-text resource resolution is a fallback for unstructured references.
	// Only run it when the prefetcher did not already resolve a structured
	// mention — otherwise it overwrites the injected cloud-safe operational
	// context with its provider-safe prompt rewrite
	// (injectPlainTextResourceContextIntoLatestUserMessage replaces the user
	// message content), and the model loses the prefetched context entirely.
	if assistantToolScope == assistantTurnToolScopeFull && !mentionsFound {
		if attachPlainTextAssistantResourceContext(session.ID, messages, sessions, unifiedResourceProvider, readState, req.Prompt) {
			mentionsFound = true
		}
	}
	if !mentionsFound {
		s.injectRecentSessionContext(session.ID, messages, sessions)
	}

	// Set session-scoped resolved context on executor for resource validation.
	// This ensures tools can only operate on resources discovered in this session.
	resolvedCtx := sessions.GetResolvedContext(session.ID)

	// Shared session state for the selected model's turn.
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

	// Run agentic loop
	modelBoundaryAllowedInventoryContext := ""
	if assistantToolScope == assistantTurnToolScopeQueryOnly {
		emitWorkflowState(streamCallback, "context", "Reading current Pulse inventory with pulse_query.", "", "pulse_query")
		if assistantPromptRequestsDeterministicInventoryCount(normalizeAssistantToolRoutingPrompt(req.Prompt)) {
			if answer, ok := s.directAssistantInventoryCountAnswer(ctx, baseExecutor); ok {
				emitWorkflowState(streamCallback, "context_ready", "Counted current Pulse inventory.", "", "pulse_query")
				s.streamDirectAssistantAnswer(sessions, session.ID, answer, streamCallback)
				log.Debug().
					Str("session_id", session.ID).
					Msg("[ChatService] Answered deterministic inventory count from canonical state")
				return nil
			}
		}
		if summaryContext := s.prefetchAssistantInventoryTopologyContext(ctx, baseExecutor); summaryContext != "" {
			modelBoundaryAllowedInventoryContext = summaryContext
			injectHandoffContextIntoLatestUserMessage(messages, summaryContext)
			assistantToolScope = assistantTurnToolScopeTextOnly
			emitWorkflowState(streamCallback, "context_ready", "Built compact inventory context for the model.", "", "pulse_query")
			log.Debug().
				Str("session_id", session.ID).
				Msg("[ChatService] Prefetched canonical inventory summary for query-only Assistant turn")
		}
	}
	filteredTools := s.toolsForAssistantTurnWithScope(assistantToolScope, autonomousMode, false)
	if resourceContextTurnIsContextOnly(req.Prompt, handoffResources, handoffMetadata) {
		filteredTools = []providers.Tool{}
		assistantToolScope = assistantTurnToolScopeTextOnly
		log.Debug().
			Str("session_id", session.ID).
			Msg("[ChatService] Withholding tools for context-only resource handoff turn")
	}
	log.Debug().
		Str("session_id", session.ID).
		Int("tools_count", len(filteredTools)).
		Msg("[ChatService] Prepared governed tool manifest, starting agentic loop")

	wrappedCallback := func(event StreamEvent) {
		event = sanitizeStreamEventForHandoffResourcePolicy(event, handoffResources, handoffResourceProvider)
		var ok bool
		event, ok = event.ClientSafe()
		if !ok {
			return
		}
		streamCallback(event)
	}

	attempt, providerErr := s.chatProviderAttempt(selectedModel, configuredModel, configuredProvider)
	if providerErr != nil {
		return providerErr
	}

	runAttempt := func(attempt chatProviderAttempt) ([]Message, *AgenticLoop, error) {
		attemptProvider := attempt.Provider
		if attemptProvider == nil {
			if strings.TrimSpace(attempt.Model) == "" {
				return nil, nil, fmt.Errorf("no chat model configured")
			}
			var providerErr error
			attemptProvider, providerErr = s.createProviderForModel(attempt.Model)
			if providerErr != nil {
				return nil, nil, fmt.Errorf("failed to create provider for model %q: %w", attempt.Model, providerErr)
			}
		}

		var executor *tools.PulseToolExecutor
		if baseExecutor != nil {
			executor = baseExecutor.Clone()
			executor.SetControlLevel(effectiveControlLevel)
			executor.SetAutonomousMode(autonomousMode)
			executor.SetResolvedContext(resolvedCtx)
			log.Debug().
				Str("session_id", session.ID).
				Int("resolved_resources", len(resolvedCtx.Resources)).
				Msg("[ChatService] Set resolved context on executor")
		}

		// Create a per-attempt AgenticLoop to ensure complete isolation between
		// concurrent sessions and chat attempts. This prevents race
		// conditions where ExecuteStream calls overwrite each other's FSM,
		// knowledge accumulator, autonomous mode, budget checker, and provider info.
		systemPrompt := s.buildSystemPromptForOfferedTools(filteredTools)
		loop := NewAgenticLoop(attemptProvider, executor, systemPrompt)
		loop.SetOrgID(s.orgID)
		loop.SetAutonomousMode(autonomousMode)
		loop.SetPreferSummaryOnlyQueries(assistantToolScope == assistantTurnToolScopeQueryOnly)
		sanitizerOptions := []modelboundary.RequestSanitizerOption{}
		if modelBoundaryAllowedInventoryContext != "" {
			sanitizerOptions = append(sanitizerOptions, modelboundary.AllowResourcePolicyText(modelBoundaryAllowedInventoryContext))
		}
		// Preserve the PII-free operational context injected for opted-in cloud
		// turns so the resource-policy sanitizer does not re-strip it as if it
		// were a raw identifier. FormatCloudSafeContext already excludes PII.
		if len(modelBoundaryAllowedCloudContext) > 0 {
			sanitizerOptions = append(sanitizerOptions, modelboundary.AllowResourcePolicyText(modelBoundaryAllowedCloudContext...))
		}
		loop.SetRequestSanitizer(modelboundary.RequestSanitizerForModel(attempt.Model, unifiedResourceProvider, sanitizerOptions...))
		loop.SetSuppressProviderErrorEvents(true)
		loop.SetSessionFSM(sessionFSM)
		loop.SetKnowledgeAccumulator(ka)
		if s.budgetChecker != nil {
			loop.SetBudgetChecker(s.budgetChecker)
		}
		if attempt.Model != "" {
			parts := strings.SplitN(attempt.Model, ":", 2)
			if len(parts) == 2 {
				loop.SetProviderInfo(parts[0], parts[1])
			}
		}
		if req.MaxTurns > 0 {
			loop.SetMaxTurns(req.MaxTurns)
		}
		emitWorkflowState(
			streamCallback,
			"provider_start",
			"Waiting for assistant.",
			sessionFSMState(sessionFSM),
			"",
			withWorkflowModelRoute(attempt.Model),
		)

		log.Debug().
			Str("session_id", session.ID).
			Str("fsm_state", string(sessionFSM.State)).
			Bool("wrote_this_episode", sessionFSM.WroteThisEpisode).
			Str("model", attempt.Model).
			Msg("[ChatService] Set session FSM on agentic loop")

		attemptCallback := func(event StreamEvent) {
			if event.Type == "question" {
				var data QuestionData
				if err := json.Unmarshal(event.Data, &data); err == nil && data.QuestionID != "" {
					s.registerQuestionLoop(data.QuestionID, loop)
				}
			}
			wrappedCallback(event)
		}

		s.registerActiveLoop(session.ID, loop)
		resultMessages, err := loop.ExecuteWithTools(ctx, session.ID, messages, filteredTools, attemptCallback)
		s.unregisterActiveLoop(session.ID, loop)
		resultMessages = sanitizeMessagesForHandoffResourcePolicy(resultMessages, handoffResources, handoffResourceProvider)
		s.recordChatTurnCost(loop, attempt.Model)

		log.Debug().
			Str("session_id", session.ID).
			Str("model", attempt.Model).
			Int("result_messages", len(resultMessages)).
			Err(err).
			Msg("[ChatService] Agentic loop returned")

		return resultMessages, loop, err
	}

	var resultMessages []Message
	var loop *AgenticLoop
	var streamErr error
	lastAttemptModel := attempt.Model
	resultMessages, loop, streamErr = runAttempt(attempt)

	if streamErr != nil {
		emitChatProviderError(streamCallback, streamErr)
		// Still save any messages we got
		for _, msg := range resultMessages {
			if msg.Role == "assistant" && strings.TrimSpace(msg.Model) == "" {
				msg.Model = lastAttemptModel
			}
			if saveErr := sessions.AddMessage(session.ID, msg); saveErr != nil {
				log.Warn().Err(saveErr).Msg("failed to save message after error")
			}
		}
		return streamErr
	}

	// Save result messages
	for _, msg := range resultMessages {
		// Skip user messages (already saved)
		if msg.Role == "user" && msg.ToolResult == nil {
			continue
		}
		if msg.Role == "assistant" && strings.TrimSpace(msg.Model) == "" {
			msg.Model = selectedModel
		}
		if err := sessions.AddMessage(session.ID, msg); err != nil {
			log.Warn().Err(err).Msg("failed to save message")
		}
	}

	// Send done event with token usage for this request.
	doneData, _ := json.Marshal(DoneData{
		SessionID:    session.ID,
		Model:        selectedModel,
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

func (s *Service) prefetchAssistantInventoryTopologyContext(ctx context.Context, executor *tools.PulseToolExecutor) string {
	if contextText := strings.TrimSpace(s.prefetchAssistantInventoryTopologyContextFromReadState()); contextText != "" {
		return assistantInventoryTopologyContextPreamble(contextText) + "\n" +
			truncateToolResultForModel(contextText)
	}
	if executor == nil {
		return ""
	}
	result, err := executor.ExecuteTool(ctx, "pulse_query", map[string]interface{}{
		"action":                          "topology",
		"summary_only":                    false,
		"max_proxmox_nodes":               0,
		"max_vms_per_node":                0,
		"max_containers_per_node":         0,
		"max_docker_hosts":                0,
		"max_docker_containers_per_host":  0,
		"max_k8s_clusters":                0,
		"max_k8s_nodes_per_cluster":       0,
		"max_k8s_deployments_per_cluster": 0,
		"max_k8s_pods_per_cluster":        0,
	})
	if err != nil || result.IsError {
		log.Debug().
			Err(err).
			Bool("tool_error", result.IsError).
			Msg("[ChatService] Unable to prefetch Assistant inventory summary")
		return ""
	}
	resultText := strings.TrimSpace(FormatToolResult(result))
	if resultText == "" {
		return ""
	}
	contextText := resultText
	var topology tools.TopologyResponse
	if err := json.Unmarshal([]byte(resultText), &topology); err == nil {
		if inventoryText, marshalErr := marshalAssistantInventoryTopologyContext(topology.NormalizeCollections()); marshalErr == nil && strings.TrimSpace(inventoryText) != "" {
			contextText = inventoryText
		}
	}
	return assistantInventoryTopologyContextPreamble(contextText) + "\n" +
		truncateToolResultForModel(contextText)
}

func (s *Service) prefetchAssistantInventoryTopologyContextFromReadState() string {
	if s == nil || s.readState == nil {
		return ""
	}
	contextText, err := marshalAssistantInventoryTopologyContextFromReadState(s.readState)
	if err != nil {
		log.Debug().Err(err).Msg("[ChatService] Unable to prefetch Assistant inventory context from ReadState")
		return ""
	}
	return strings.TrimSpace(contextText)
}

func (s *Service) directAssistantInventoryCountAnswer(ctx context.Context, executor *tools.PulseToolExecutor) (string, bool) {
	context, ok := s.assistantInventoryTopologyContextForDirectAnswer(ctx, executor)
	if !ok {
		return "", false
	}
	return formatAssistantInventoryCountAnswer(context)
}

func (s *Service) assistantInventoryTopologyContextForDirectAnswer(ctx context.Context, executor *tools.PulseToolExecutor) (assistantInventoryTopologyContext, bool) {
	if contextText := strings.TrimSpace(s.prefetchAssistantInventoryTopologyContextFromReadState()); contextText != "" {
		return parseAssistantInventoryTopologyContext(contextText)
	}
	if executor == nil {
		return assistantInventoryTopologyContext{}, false
	}
	result, err := executor.ExecuteTool(ctx, "pulse_query", map[string]interface{}{
		"action":                          "topology",
		"summary_only":                    false,
		"max_proxmox_nodes":               0,
		"max_vms_per_node":                0,
		"max_containers_per_node":         0,
		"max_docker_hosts":                0,
		"max_docker_containers_per_host":  0,
		"max_k8s_clusters":                0,
		"max_k8s_nodes_per_cluster":       0,
		"max_k8s_deployments_per_cluster": 0,
		"max_k8s_pods_per_cluster":        0,
	})
	if err != nil || result.IsError {
		return assistantInventoryTopologyContext{}, false
	}
	resultText := strings.TrimSpace(FormatToolResult(result))
	if resultText == "" {
		return assistantInventoryTopologyContext{}, false
	}
	var topology tools.TopologyResponse
	if err := json.Unmarshal([]byte(resultText), &topology); err != nil {
		return assistantInventoryTopologyContext{}, false
	}
	contextText, err := marshalAssistantInventoryTopologyContext(topology.NormalizeCollections())
	if err != nil {
		return assistantInventoryTopologyContext{}, false
	}
	return parseAssistantInventoryTopologyContext(contextText)
}

func parseAssistantInventoryTopologyContext(contextText string) (assistantInventoryTopologyContext, bool) {
	var context assistantInventoryTopologyContext
	if err := json.Unmarshal([]byte(strings.TrimSpace(contextText)), &context); err != nil {
		return assistantInventoryTopologyContext{}, false
	}
	if context.Proxmox.Nodes == nil {
		context.Proxmox.Nodes = []assistantInventoryProxmoxNode{}
	}
	if context.Docker.Hosts == nil {
		context.Docker.Hosts = []assistantInventoryDockerHost{}
	}
	if context.Kubernetes.Clusters == nil {
		context.Kubernetes.Clusters = []assistantInventoryKubernetesCluster{}
	}
	return context, assistantInventorySummaryTotal(context.Summary) > 0 || assistantInventoryIncludedContextTotal(context) > 0
}

func (s *Service) streamDirectAssistantAnswer(sessions *SessionStore, sessionID, answer string, streamCallback StreamCallback) {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return
	}
	if sessions != nil {
		if err := sessions.AddMessage(sessionID, Message{
			ID:        uuid.New().String(),
			Role:      "assistant",
			Content:   answer,
			Model:     assistantLocalInventoryModelRoute,
			Timestamp: time.Now(),
		}); err != nil {
			log.Warn().Err(err).Str("session_id", sessionID).Msg("[ChatService] Failed to save direct Assistant answer")
		}
	}
	contentData, _ := json.Marshal(ContentData{Text: answer})
	streamCallback(StreamEvent{Type: "content", Data: contentData})
	doneData, _ := json.Marshal(DoneData{SessionID: sessionID, Model: assistantLocalInventoryModelRoute})
	streamCallback(StreamEvent{Type: "done", Data: doneData})
}

func formatAssistantInventoryCountAnswer(context assistantInventoryTopologyContext) (string, bool) {
	summary := assistantInventoryContextSummary(context)
	total := assistantInventorySummaryTotal(summary)
	if total == 0 {
		return "", false
	}

	computeTotal := summary.TotalNodes + summary.TotalVMs + summary.TotalSystemContainers
	dockerTotal := summary.TotalDockerHosts + summary.TotalDockerContainers
	kubernetesTotal := summary.TotalK8sClusters + summary.TotalK8sNodes + summary.TotalK8sDeployments + summary.TotalK8sPods

	var categories []string
	if computeTotal > 0 {
		categories = append(categories, fmt.Sprintf("%s (%s)",
			assistantInventoryCountPhrase(computeTotal, "compute resource", "compute resources"),
			assistantInventoryJoinPhrases([]string{
				assistantInventoryPositiveCountPhrase(summary.TotalNodes, "Proxmox node", "Proxmox nodes"),
				assistantInventoryPositiveCountPhrase(summary.TotalVMs, "VM", "VMs"),
				assistantInventoryPositiveCountPhrase(summary.TotalSystemContainers, "system container", "system containers"),
			}),
		))
	}
	if dockerTotal > 0 {
		categories = append(categories, fmt.Sprintf("%s (%s)",
			assistantInventoryCountPhrase(dockerTotal, "Docker inventory item", "Docker inventory items"),
			assistantInventoryJoinPhrases([]string{
				assistantInventoryPositiveCountPhrase(summary.TotalDockerHosts, "Docker host", "Docker hosts"),
				assistantInventoryPositiveCountPhrase(summary.TotalDockerContainers, "Docker container", "Docker containers"),
			}),
		))
	}
	if kubernetesTotal > 0 {
		categories = append(categories, fmt.Sprintf("%s (%s)",
			assistantInventoryCountPhrase(kubernetesTotal, "Kubernetes inventory item", "Kubernetes inventory items"),
			assistantInventoryJoinPhrases([]string{
				assistantInventoryPositiveCountPhrase(summary.TotalK8sClusters, "cluster", "clusters"),
				assistantInventoryPositiveCountPhrase(summary.TotalK8sNodes, "node", "nodes"),
				assistantInventoryPositiveCountPhrase(summary.TotalK8sDeployments, "deployment", "deployments"),
				assistantInventoryPositiveCountPhrase(summary.TotalK8sPods, "pod", "pods"),
			}),
		))
	}

	if len(categories) == 1 {
		return "Pulse currently sees " + categories[0] + ".", true
	}
	return fmt.Sprintf("Pulse currently sees %s: %s.",
		assistantInventoryCountPhrase(total, "visible inventory item", "visible inventory items"),
		assistantInventoryJoinPhrases(categories),
	), true
}

func assistantInventoryContextSummary(context assistantInventoryTopologyContext) tools.TopologySummary {
	summary := context.Summary
	included := assistantInventoryIncludedContextCounts(context)
	if summary.TotalNodes == 0 {
		summary.TotalNodes = included.proxmoxNodes
	}
	if summary.TotalVMs == 0 {
		summary.TotalVMs = included.vms
	}
	if summary.TotalSystemContainers == 0 {
		summary.TotalSystemContainers = included.systemContainers
	}
	if summary.TotalDockerHosts == 0 {
		summary.TotalDockerHosts = included.dockerHosts
	}
	if summary.TotalDockerContainers == 0 {
		summary.TotalDockerContainers = included.dockerContainers
	}
	if summary.TotalK8sClusters == 0 {
		summary.TotalK8sClusters = included.k8sClusters
	}
	if summary.TotalK8sNodes == 0 {
		summary.TotalK8sNodes = included.k8sNodes
	}
	if summary.TotalK8sDeployments == 0 {
		summary.TotalK8sDeployments = included.k8sDeployments
	}
	if summary.TotalK8sPods == 0 {
		summary.TotalK8sPods = included.k8sPods
	}
	return summary
}

func assistantInventorySummaryTotal(summary tools.TopologySummary) int {
	return summary.TotalNodes +
		summary.TotalVMs +
		summary.TotalSystemContainers +
		summary.TotalDockerHosts +
		summary.TotalDockerContainers +
		summary.TotalK8sClusters +
		summary.TotalK8sNodes +
		summary.TotalK8sDeployments +
		summary.TotalK8sPods
}

func assistantInventoryIncludedContextTotal(context assistantInventoryTopologyContext) int {
	counts := assistantInventoryIncludedContextCounts(context)
	return counts.proxmoxNodes +
		counts.vms +
		counts.systemContainers +
		counts.dockerHosts +
		counts.dockerContainers +
		counts.k8sClusters +
		counts.k8sNodes +
		counts.k8sDeployments +
		counts.k8sPods
}

func assistantInventoryIncludedContextCounts(context assistantInventoryTopologyContext) assistantInventoryTopologyCounts {
	counts := assistantInventoryTopologyCounts{
		proxmoxNodes: len(context.Proxmox.Nodes),
		dockerHosts:  len(context.Docker.Hosts),
		k8sClusters:  len(context.Kubernetes.Clusters),
	}
	for _, node := range context.Proxmox.Nodes {
		counts.vms += len(node.VMs)
		counts.systemContainers += len(node.Containers)
	}
	for _, host := range context.Docker.Hosts {
		counts.dockerContainers += len(host.Containers)
	}
	for _, cluster := range context.Kubernetes.Clusters {
		counts.k8sNodes += len(cluster.Nodes)
		counts.k8sDeployments += len(cluster.Deployments)
		counts.k8sPods += len(cluster.Pods)
	}
	return counts
}

func assistantInventoryCountPhrase(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func assistantInventoryPositiveCountPhrase(count int, singular, plural string) string {
	if count <= 0 {
		return ""
	}
	return assistantInventoryCountPhrase(count, singular, plural)
}

func assistantInventoryJoinPhrases(phrases []string) string {
	filtered := make([]string, 0, len(phrases))
	for _, phrase := range phrases {
		if trimmed := strings.TrimSpace(phrase); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	switch len(filtered) {
	case 0:
		return ""
	case 1:
		return filtered[0]
	case 2:
		return filtered[0] + " and " + filtered[1]
	default:
		return strings.Join(filtered[:len(filtered)-1], ", ") + ", and " + filtered[len(filtered)-1]
	}
}

func marshalAssistantInventoryTopologyContextFromReadState(rs unifiedresources.ReadState) (string, error) {
	if rs == nil {
		return "", nil
	}

	topology, ok := assistantInventoryTopologyResponseFromReadState(rs)
	if !ok {
		return "", nil
	}
	return marshalAssistantInventoryTopologyContext(topology)
}

func assistantInventoryTopologyResponseFromReadState(rs unifiedresources.ReadState) (tools.TopologyResponse, bool) {
	if rs == nil {
		return tools.EmptyTopologyResponse(), false
	}
	response := tools.BuildTopologyResponseFromReadState(rs, tools.TopologyBuildOptions{Include: "all"})
	return response, assistantInventorySummaryTotal(response.Summary) > 0
}

func marshalAssistantInventoryTopologyContext(topology tools.TopologyResponse) (string, error) {
	context := assistantInventoryTopologyContext{
		Summary: topology.Summary,
		Proxmox: assistantInventoryProxmoxTopology{
			Nodes: make([]assistantInventoryProxmoxNode, 0, len(topology.Proxmox.Nodes)),
		},
		Docker: assistantInventoryDockerTopology{
			Hosts: make([]assistantInventoryDockerHost, 0, len(topology.Docker.Hosts)),
		},
		Kubernetes: assistantInventoryKubernetesTopology{
			Clusters: make([]assistantInventoryKubernetesCluster, 0, len(topology.Kubernetes.Clusters)),
		},
	}
	for _, node := range topology.Proxmox.Nodes {
		nodeContext := assistantInventoryProxmoxNode{
			AnswerLabel:    assistantInventoryNodeAnswerLabel(node.Name),
			Name:           node.Name,
			Status:         node.Status,
			AgentConnected: node.AgentConnected,
			CanExecute:     node.CanExecute,
			VMCount:        node.VMCount,
			ContainerCount: node.ContainerCount,
			VMs:            make([]assistantInventoryWorkload, 0, len(node.VMs)),
			Containers:     make([]assistantInventoryWorkload, 0, len(node.Containers)),
		}
		for _, vm := range node.VMs {
			nodeContext.VMs = append(nodeContext.VMs, assistantInventoryWorkload{
				AnswerLabel:   assistantInventoryGuestAnswerLabel("VM", vm.VMID, vm.Name),
				VMID:          vm.VMID,
				Name:          vm.Name,
				Status:        vm.Status,
				CPUPercent:    vm.CPU,
				MemoryPercent: vm.Memory,
			})
		}
		for _, container := range node.Containers {
			nodeContext.Containers = append(nodeContext.Containers, assistantInventoryWorkload{
				AnswerLabel:   assistantInventoryGuestAnswerLabel("CT", container.VMID, container.Name),
				VMID:          container.VMID,
				Name:          container.Name,
				Status:        container.Status,
				CPUPercent:    container.CPU,
				MemoryPercent: container.Memory,
			})
		}
		context.Proxmox.Nodes = append(context.Proxmox.Nodes, nodeContext)
	}
	for _, host := range topology.Docker.Hosts {
		hostContext := assistantInventoryDockerHost{
			AnswerLabel:    firstNonEmptyString(host.DisplayName, host.Hostname),
			Hostname:       host.Hostname,
			DisplayName:    host.DisplayName,
			AgentConnected: host.AgentConnected,
			CanExecute:     host.CanExecute,
			ContainerCount: host.ContainerCount,
			RunningCount:   host.RunningCount,
			Containers:     make([]assistantInventoryAppContainer, 0, len(host.Containers)),
		}
		for _, container := range host.Containers {
			hostContext.Containers = append(hostContext.Containers, assistantInventoryAppContainer{
				AnswerLabel: firstNonEmptyString(container.Name, container.ID),
				ID:          container.ID,
				Name:        container.Name,
				State:       container.State,
				Health:      container.Health,
			})
		}
		context.Docker.Hosts = append(context.Docker.Hosts, hostContext)
	}
	for _, cluster := range topology.Kubernetes.Clusters {
		clusterContext := assistantInventoryKubernetesCluster{
			AnswerLabel:     firstNonEmptyString(cluster.Name, cluster.ID),
			Name:            cluster.Name,
			ID:              cluster.ID,
			Status:          cluster.Status,
			NodeCount:       cluster.NodeCount,
			DeploymentCount: cluster.DeploymentCount,
			PodCount:        cluster.PodCount,
			Nodes:           make([]assistantInventoryKubernetesNode, 0, len(cluster.Nodes)),
			Deployments:     make([]assistantInventoryKubernetesDeployment, 0, len(cluster.Deployments)),
			Pods:            make([]assistantInventoryKubernetesPod, 0, len(cluster.Pods)),
		}
		for _, node := range cluster.Nodes {
			clusterContext.Nodes = append(clusterContext.Nodes, assistantInventoryKubernetesNode{
				AnswerLabel:   node.Name,
				Name:          node.Name,
				Status:        node.Status,
				Ready:         node.Ready,
				Roles:         node.Roles,
				CPUPercent:    node.CPU,
				MemoryPercent: node.Memory,
			})
		}
		for _, deployment := range cluster.Deployments {
			clusterContext.Deployments = append(clusterContext.Deployments, assistantInventoryKubernetesDeployment{
				AnswerLabel:     deployment.Name,
				Name:            deployment.Name,
				Namespace:       deployment.Namespace,
				Status:          deployment.Status,
				DesiredReplicas: deployment.DesiredReplicas,
				ReadyReplicas:   deployment.ReadyReplicas,
			})
		}
		for _, pod := range cluster.Pods {
			clusterContext.Pods = append(clusterContext.Pods, assistantInventoryKubernetesPod{
				AnswerLabel: pod.Name,
				Name:        pod.Name,
				Namespace:   pod.Namespace,
				Status:      pod.Status,
				Restarts:    pod.Restarts,
				OwnerKind:   pod.OwnerKind,
				OwnerName:   pod.OwnerName,
			})
		}
		context.Kubernetes.Clusters = append(context.Kubernetes.Clusters, clusterContext)
	}

	encoded, err := json.Marshal(context)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

type assistantInventoryTopologyContext struct {
	Proxmox    assistantInventoryProxmoxTopology    `json:"proxmox"`
	Docker     assistantInventoryDockerTopology     `json:"docker"`
	Kubernetes assistantInventoryKubernetesTopology `json:"kubernetes"`
	Summary    tools.TopologySummary                `json:"summary"`
}

type assistantInventoryProxmoxTopology struct {
	Nodes []assistantInventoryProxmoxNode `json:"nodes"`
}

type assistantInventoryDockerTopology struct {
	Hosts []assistantInventoryDockerHost `json:"hosts"`
}

type assistantInventoryKubernetesTopology struct {
	Clusters []assistantInventoryKubernetesCluster `json:"clusters"`
}

type assistantInventoryProxmoxNode struct {
	AnswerLabel    string                       `json:"answer_label"`
	Name           string                       `json:"name"`
	Status         string                       `json:"status"`
	AgentConnected bool                         `json:"agent_connected,omitempty"`
	CanExecute     bool                         `json:"can_execute,omitempty"`
	VMCount        int                          `json:"vm_count"`
	ContainerCount int                          `json:"container_count"`
	VMs            []assistantInventoryWorkload `json:"vms"`
	Containers     []assistantInventoryWorkload `json:"containers"`
}

type assistantInventoryWorkload struct {
	AnswerLabel   string  `json:"answer_label"`
	VMID          int     `json:"vmid"`
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	CPUPercent    float64 `json:"cpu_percent,omitempty"`
	MemoryPercent float64 `json:"memory_percent,omitempty"`
}

type assistantInventoryDockerHost struct {
	AnswerLabel    string                           `json:"answer_label"`
	Hostname       string                           `json:"hostname"`
	DisplayName    string                           `json:"display_name,omitempty"`
	AgentConnected bool                             `json:"agent_connected,omitempty"`
	CanExecute     bool                             `json:"can_execute,omitempty"`
	ContainerCount int                              `json:"container_count"`
	RunningCount   int                              `json:"running_count"`
	Containers     []assistantInventoryAppContainer `json:"containers"`
}

type assistantInventoryAppContainer struct {
	AnswerLabel string `json:"answer_label"`
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Health      string `json:"health,omitempty"`
}

type assistantInventoryKubernetesCluster struct {
	AnswerLabel     string                                   `json:"answer_label"`
	Name            string                                   `json:"name"`
	ID              string                                   `json:"id,omitempty"`
	Status          string                                   `json:"status"`
	NodeCount       int                                      `json:"node_count"`
	DeploymentCount int                                      `json:"deployment_count"`
	PodCount        int                                      `json:"pod_count"`
	Nodes           []assistantInventoryKubernetesNode       `json:"nodes"`
	Deployments     []assistantInventoryKubernetesDeployment `json:"deployments"`
	Pods            []assistantInventoryKubernetesPod        `json:"pods"`
}

type assistantInventoryKubernetesNode struct {
	AnswerLabel   string   `json:"answer_label"`
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	Ready         bool     `json:"ready"`
	Roles         []string `json:"roles"`
	CPUPercent    float64  `json:"cpu_percent,omitempty"`
	MemoryPercent float64  `json:"memory_percent,omitempty"`
}

type assistantInventoryKubernetesDeployment struct {
	AnswerLabel     string `json:"answer_label"`
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	Status          string `json:"status"`
	DesiredReplicas int32  `json:"desired_replicas,omitempty"`
	ReadyReplicas   int32  `json:"ready_replicas,omitempty"`
}

type assistantInventoryKubernetesPod struct {
	AnswerLabel string `json:"answer_label"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Status      string `json:"status"`
	Restarts    int    `json:"restarts,omitempty"`
	OwnerKind   string `json:"owner_kind,omitempty"`
	OwnerName   string `json:"owner_name,omitempty"`
}

func assistantInventoryTopologyContextPreamble(resultText string) string {
	var lines []string
	lines = append(lines,
		"Pulse inventory summary from current canonical resource state, including per-node and workload detail where available.",
		"Use this context to answer the user's count, overview, status, list, or inventory-breakdown question. Do not claim shell inspection or host command execution.",
		"The answer_label fields are UI-visible inventory labels. Copy answer_label exactly when listing nodes, VMs, system containers, Docker containers, Kubernetes nodes, deployments, or pods.",
		"Do not shorten answer_label values or substitute generic labels like Node 1, VM 100, or CT 101 when an answer_label is available.",
		"Do not include raw secrets, config values, IP addresses, routing metadata, provider internals, or shell command claims in the answer.",
	)

	var topology tools.TopologyResponse
	if err := json.Unmarshal([]byte(resultText), &topology); err != nil {
		lines = append(lines, "Completeness: The topology detail could not be counted before the model request; if the payload is marked TRUNCATED, say which details may be omitted instead of claiming every workload is listed.")
		return strings.Join(lines, "\n")
	}
	topology = topology.NormalizeCollections()
	included := assistantInventoryIncludedTopologyCounts(topology)
	complete := !toolResultWouldTruncateForModel(resultText) &&
		included.proxmoxNodes >= topology.Summary.TotalNodes &&
		included.vms >= topology.Summary.TotalVMs &&
		included.systemContainers >= topology.Summary.TotalSystemContainers &&
		included.dockerHosts >= topology.Summary.TotalDockerHosts &&
		included.dockerContainers >= topology.Summary.TotalDockerContainers &&
		included.k8sClusters >= topology.Summary.TotalK8sClusters &&
		included.k8sNodes >= topology.Summary.TotalK8sNodes &&
		included.k8sDeployments >= topology.Summary.TotalK8sDeployments &&
		included.k8sPods >= topology.Summary.TotalK8sPods
	if complete {
		lines = append(lines, "Completeness: The topology detail below includes every Proxmox node, VM, system container, Docker host/container, and Kubernetes resource currently visible in Pulse.")
	} else {
		lines = append(lines, fmt.Sprintf("Completeness: The topology detail below may be capped or context-truncated. Included detail counts are nodes=%d/%d, VMs=%d/%d, system_containers=%d/%d, docker_hosts=%d/%d, docker_containers=%d/%d, k8s_clusters=%d/%d, k8s_nodes=%d/%d, k8s_deployments=%d/%d, k8s_pods=%d/%d. If included counts do not match totals or a TRUNCATED marker appears, say what may be omitted instead of claiming every workload is listed.",
			included.proxmoxNodes,
			topology.Summary.TotalNodes,
			included.vms,
			topology.Summary.TotalVMs,
			included.systemContainers,
			topology.Summary.TotalSystemContainers,
			included.dockerHosts,
			topology.Summary.TotalDockerHosts,
			included.dockerContainers,
			topology.Summary.TotalDockerContainers,
			included.k8sClusters,
			topology.Summary.TotalK8sClusters,
			included.k8sNodes,
			topology.Summary.TotalK8sNodes,
			included.k8sDeployments,
			topology.Summary.TotalK8sDeployments,
			included.k8sPods,
			topology.Summary.TotalK8sPods,
		))
	}
	return strings.Join(lines, "\n")
}

func assistantInventoryNodeAnswerLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Node unknown"
	}
	return "Node " + name
}

func assistantInventoryGuestAnswerLabel(kind string, id int, name string) string {
	kind = strings.TrimSpace(kind)
	name = strings.TrimSpace(name)
	prefix := kind
	if id > 0 {
		prefix = fmt.Sprintf("%s %d", kind, id)
	}
	if name == "" {
		return prefix
	}
	return prefix + " " + name
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func statusIsRunningOrOnline(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "online", "ready", "healthy":
		return true
	default:
		return false
	}
}

type assistantInventoryTopologyCounts struct {
	proxmoxNodes     int
	vms              int
	systemContainers int
	dockerHosts      int
	dockerContainers int
	k8sClusters      int
	k8sNodes         int
	k8sDeployments   int
	k8sPods          int
}

func assistantInventoryIncludedTopologyCounts(topology tools.TopologyResponse) assistantInventoryTopologyCounts {
	counts := assistantInventoryTopologyCounts{
		proxmoxNodes: len(topology.Proxmox.Nodes),
		dockerHosts:  len(topology.Docker.Hosts),
		k8sClusters:  len(topology.Kubernetes.Clusters),
	}
	for _, node := range topology.Proxmox.Nodes {
		counts.vms += len(node.VMs)
		counts.systemContainers += len(node.Containers)
	}
	for _, host := range topology.Docker.Hosts {
		counts.dockerContainers += len(host.Containers)
	}
	for _, cluster := range topology.Kubernetes.Clusters {
		counts.k8sNodes += len(cluster.Nodes)
		counts.k8sDeployments += len(cluster.Deployments)
		counts.k8sPods += len(cluster.Pods)
	}
	return counts
}

func toolResultWouldTruncateForModel(text string) bool {
	return MaxToolResultCharsLimit > 0 && len(text) > MaxToolResultCharsLimit
}

func sanitizeHandoffContextForResourcePolicy(handoffContext string, handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) string {
	return sanitizeTextForHandoffResourcePolicy(handoffContext, handoffResources, provider)
}

func sanitizeTextForHandoffResourcePolicy(text string, handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) string {
	contextText := strings.TrimSpace(text)
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

func sanitizeStreamEventForHandoffResourcePolicy(event StreamEvent, handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) StreamEvent {
	if len(handoffResources) == 0 || provider == nil || len(event.Data) == 0 {
		return event
	}

	switch event.Type {
	case "content":
		var data ContentData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return event
		}
		data.Text = sanitizeTextForHandoffResourcePolicy(data.Text, handoffResources, provider)
		if encoded, err := json.Marshal(data); err == nil {
			event.Data = encoded
		}
	case "tool_end":
		var data ToolEndData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return event
		}
		data.Output = sanitizeTextForHandoffResourcePolicy(data.Output, handoffResources, provider)
		if encoded, err := json.Marshal(data); err == nil {
			event.Data = encoded
		}
	}

	return event
}

func sanitizeMessagesForHandoffResourcePolicy(messages []Message, handoffResources []HandoffResource, provider tools.UnifiedResourceProvider) []Message {
	if len(messages) == 0 || len(handoffResources) == 0 || provider == nil {
		return messages
	}

	for idx := range messages {
		if messages[idx].Role == "assistant" && strings.TrimSpace(messages[idx].Content) != "" {
			messages[idx].Content = sanitizeTextForHandoffResourcePolicy(messages[idx].Content, handoffResources, provider)
		}
		if messages[idx].ToolResult != nil && strings.TrimSpace(messages[idx].ToolResult.Content) != "" {
			messages[idx].ToolResult.Content = sanitizeTextForHandoffResourcePolicy(messages[idx].ToolResult.Content, handoffResources, provider)
		}
	}
	return messages
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

func mergeHandoffResourceContextPack(handoffContext string, handoffResources []HandoffResource, provider tools.UnifiedResourceProvider, store unifiedresources.ResourceStore, now time.Time) string {
	contextPack := buildHandoffResourceContextPack(handoffResources, provider, store, now)
	switch {
	case strings.TrimSpace(handoffContext) == "":
		return contextPack
	case contextPack == "":
		return strings.TrimSpace(handoffContext)
	default:
		return strings.TrimSpace(handoffContext) + "\n\n" + contextPack
	}
}

func buildHandoffResourceContextPack(handoffResources []HandoffResource, provider tools.UnifiedResourceProvider, store unifiedresources.ResourceStore, now time.Time) string {
	resources := canonicalHandoffResources(handoffResources, provider)
	if len(resources) == 0 {
		return ""
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	blocks := make([]string, 0, len(resources))
	for _, resource := range resources {
		sections := agentcontext.BuildResourceContextSections(resource, store, agentcontext.BuildOptions{
			GeneratedAt: now.UTC(),
		})
		if contextText := agentcontext.FormatSectionsForModelContext(resource, sections); contextText != "" {
			blocks = append(blocks, contextText)
		}
	}
	return strings.Join(blocks, "\n\n")
}

func mergeResourceContextHandoffDirective(handoffContext string, handoffResources []HandoffResource, metadata HandoffMetadata) string {
	directive := buildResourceContextHandoffDirective(handoffResources, metadata)
	switch {
	case directive == "":
		return strings.TrimSpace(handoffContext)
	case strings.TrimSpace(handoffContext) == "":
		return directive
	default:
		return directive + "\n\n" + strings.TrimSpace(handoffContext)
	}
}

func buildResourceContextHandoffDirective(handoffResources []HandoffResource, metadata HandoffMetadata) string {
	metadata = NormalizeHandoffMetadata(metadata)
	if metadata.Kind != sessionHandoffKindResourceContext || len(normalizeHandoffResources(handoffResources)) == 0 {
		return ""
	}

	return strings.Join([]string{
		"[Resource Context Handoff Instructions]",
		"Source: Pulse resource drawer handoff",
		"Selected Resource: The attached handoff resource is the user's current selected resource. Do not ask which server, service, container, VM, or resource the user means.",
		"Tool Target Handle: When you need a read-only tool against the attached resource, use target_host=\"current_resource\" or resource_id=\"current_resource\". Do not copy a withheld or placeholder label into any tool argument.",
		"Context-First Answering: When the user asks what Pulse already knows, asks for discovery readiness, or asks a question that should be answerable from discovered/service context, answer from the attached context without tools. If the attached context lacks the fact, say that Pulse does not currently have that discovery/context fact instead of filling the gap with tools.",
		"Discovery Boundary: Do not call discovery tools only to identify this resource or fill in missing context. Use attached discovery readiness first; call discovery only when the user explicitly asks you to run discovery.",
		"Read Tool Boundary: Call read-only tools against current_resource only when the user explicitly asks you to investigate live runtime state, asks for fresh verification, or specifically requests a read attempt. Do not use read or mixed tools just to improve a context summary.",
		"Data Boundary: You may use the attached operational context — the service's access pattern, config/data/log paths, and ports — to answer and guide the user. Do not reveal, reconstruct, or guess raw hostnames, IP addresses, aliases, environment variables, credentials, secret-bearing metadata, or any value the attached context marks as withheld or redacted.",
		"Withheld Details: If asked to print, expand, or reveal a value the attached context withholds or redacts (a hostname, IP address, credential, or secret), start with exactly this boundary: \"That detail is withheld by policy.\" Then give a safe summary. This boundary does not apply to the operational paths, ports, and access pattern already provided — use those freely to help.",
		"Action Boundary: Context is read-only and grants no approval or execution authority. Any action requires the governed approval/action flow.",
	}, "\n")
}

func resourceContextTurnIsContextOnly(prompt string, handoffResources []HandoffResource, metadata HandoffMetadata) bool {
	metadata = NormalizeHandoffMetadata(metadata)
	if metadata.Kind != sessionHandoffKindResourceContext || len(normalizeHandoffResources(handoffResources)) == 0 {
		return false
	}
	text := strings.ToLower(strings.Join(strings.Fields(prompt), " "))
	if text == "" {
		return true
	}
	if resourceContextPromptDisallowsTools(text) {
		return true
	}
	return !resourceContextPromptExplicitlyRequestsTools(text)
}

func resourceContextPromptDisallowsTools(text string) bool {
	for _, phrase := range []string{
		"before using any tools",
		"without using tools",
		"without running tools",
		"do not use tools",
		"do not run tools",
		"don't use tools",
		"don't run tools",
		"no tools",
	} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func resourceContextPromptExplicitlyRequestsTools(text string) bool {
	for _, phrase := range []string{
		"pulse_read",
		"read-only pulse_read",
		"read-only tool",
		"read tool",
		"tool attempt",
		"use tools",
		"use a tool",
		"run tools",
		"run a tool",
		"call tools",
		"call a tool",
		"live runtime",
		"fresh runtime verification",
		"fresh verification",
		"verify live",
		"live check",
		"check logs",
		"read logs",
		"tail logs",
		"inspect live",
		"run discovery",
		"call discovery",
		"use discovery",
		"start discovery",
		"scan this resource",
	} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
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
		appendHandoffContextLine(&b, label+" Existing Action Artifact", action.Description)
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

	// Get governed tools for the Patrol run.
	filteredTools := s.toolsForExecutionMode(true, true)

	// Run the agentic loop
	resultMessages, err := tempLoop.ExecuteWithTools(ctx, session.ID, messages, filteredTools, callback)
	if err != nil {
		// Still save any messages we got
		for _, msg := range resultMessages {
			if saveErr := sessions.AddMessage(session.ID, msg); saveErr != nil {
				log.Warn().Err(saveErr).Msg("failed to save patrol message after error")
			}
		}
		return &PatrolResponse{
			InputTokens:  tempLoop.GetTotalInputTokens(),
			OutputTokens: tempLoop.GetTotalOutputTokens(),
		}, err
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

	// Patrol-main is reused across every scheduled run (the
	// stateless-input contract means we never load it back into the
	// agentic loop, but we still write to it for the Pulse Assistant
	// sidebar's forensic view). Without a cap, the file grew to 16 MB /
	// 3,593 messages within a month and every AddMessage rewrote the
	// whole file to disk. Cap at the most recent ~two Patrol runs'
	// worth of messages — enough for the sidebar to show recent
	// activity, far short of unbounded growth. The canonical forensic
	// log is the PatrolRunRecord history, not this session.
	const maxPatrolSessionMessages = 200
	if err := sessions.TrimMessages(session.ID, maxPatrolSessionMessages); err != nil {
		log.Warn().Err(err).Msg("failed to trim patrol session messages")
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
		Model:        patrolModel,
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

	provider, err := providers.NewForProvider(s.cfg, providerName, modelName)
	if err != nil {
		return nil, err
	}
	streamingProvider, ok := provider.(providers.StreamingProvider)
	if !ok {
		return nil, fmt.Errorf("provider %s does not support streaming chat", providerName)
	}
	return streamingProvider, nil
}

type chatProviderAttempt struct {
	Model    string
	Provider providers.StreamingProvider
}

func (s *Service) chatProviderAttempt(primaryModel, configuredModel string, configuredProvider providers.StreamingProvider) (chatProviderAttempt, error) {
	primaryModel = strings.TrimSpace(primaryModel)
	configuredModel = strings.TrimSpace(configuredModel)
	if primaryModel == "" {
		if configuredProvider != nil {
			return chatProviderAttempt{Provider: configuredProvider}, nil
		}
		return chatProviderAttempt{}, fmt.Errorf("no chat model configured")
	}

	provider := configuredProvider
	if primaryModel != configuredModel {
		provider = nil
	}
	return chatProviderAttempt{Model: primaryModel, Provider: provider}, nil
}

func emitChatProviderError(callback StreamCallback, err error) {
	if callback == nil {
		return
	}
	data, _ := json.Marshal(ErrorData{Message: fallbackProviderStreamErrorMessage(err)})
	callback(StreamEvent{Type: "error", Data: data})
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

	tools := s.toolsForExecutionMode(s.isAutonomousModeEnabled(), false)
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

// RenameSession updates the user-visible title for a persisted session.
func (s *Service) RenameSession(ctx context.Context, sessionID, title string) (*Session, error) {
	s.mu.RLock()
	sessions := s.sessions
	s.mu.RUnlock()

	if sessions == nil {
		return nil, fmt.Errorf("service not started")
	}

	return sessions.Rename(sessionID, title)
}

// GetMessages returns messages for a session
func (s *Service) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	s.mu.RLock()
	sessions := s.sessions
	s.mu.RUnlock()

	if sessions == nil {
		return nil, fmt.Errorf("service not started")
	}

	messages, err := sessions.GetMessages(sessionID)
	if err != nil {
		return nil, err
	}
	for i := range messages {
		messages[i] = messages[i].ClientSafe()
	}
	return clientSafeTranscriptMessages(messages), nil
}

func clientSafeTranscriptMessages(messages []Message) []Message {
	if len(messages) == 0 {
		return []Message{}
	}

	resultsByToolID := make(map[string]ToolResult)
	for _, msg := range messages {
		if msg.ToolResult == nil {
			continue
		}
		toolID := strings.TrimSpace(msg.ToolResult.ToolUseID)
		if toolID == "" {
			continue
		}
		resultsByToolID[toolID] = *msg.ToolResult
	}

	transcript := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if msg.ToolResult != nil {
			continue
		}

		safe := msg.ClientSafe()
		for i := range safe.ToolCalls {
			toolID := strings.TrimSpace(safe.ToolCalls[i].ID)
			if toolID == "" {
				continue
			}
			if result, ok := resultsByToolID[toolID]; ok {
				success := !result.IsError
				safe.ToolCalls[i].Output = result.Content
				safe.ToolCalls[i].Success = &success
			}
		}
		transcript = append(transcript, safe)
	}

	return transcript
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

// ForkSession creates a branch
func (s *Service) ForkSession(ctx context.Context, sessionID string) (*Session, error) {
	s.mu.RLock()
	sessions := s.sessions
	s.mu.RUnlock()

	if sessions == nil {
		return nil, fmt.Errorf("service not started")
	}

	return sessions.Fork(sessionID)
}

// UndoLastTurn removes the latest user turn from the durable chat session.
func (s *Service) UndoLastTurn(ctx context.Context, sessionID string) (*SessionTurnUndoResult, error) {
	s.mu.RLock()
	sessions := s.sessions
	s.mu.RUnlock()

	if sessions == nil {
		return nil, fmt.Errorf("service not started")
	}

	return sessions.UndoLastTurn(sessionID)
}

// RedoLastTurn restores the latest turn removed by UndoLastTurn.
func (s *Service) RedoLastTurn(ctx context.Context, sessionID string) (*SessionTurnRedoResult, error) {
	s.mu.RLock()
	sessions := s.sessions
	s.mu.RUnlock()

	if sessions == nil {
		return nil, fmt.Errorf("service not started")
	}

	return sessions.RedoLastTurn(sessionID)
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

func (s *Service) SetReadState(readState unifiedresources.ReadState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readState = readState
	if readState != nil && s.discoveryProvider != nil {
		s.contextPrefetcher = NewContextPrefetcher(readState, s.discoveryProvider)
	} else {
		s.contextPrefetcher = nil
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
	s.discoveryProvider = provider
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

	if provider, ok := mockAssistantProviderIfEnabled(); ok {
		return provider, nil
	}

	if s.providerFactory != nil {
		chatModel := chatModelForInjectedProviderFactory(s.cfg)
		if chatModel == "" {
			return nil, fmt.Errorf("no chat model configured")
		}
		return s.createProviderForModel(chatModel)
	}

	chatModel, err := modelresolution.ResolveConfiguredChatModelOffline(s.cfg)
	if err != nil {
		return nil, err
	}
	if chatModel == "" {
		return nil, fmt.Errorf("no chat model configured")
	}
	if strings.TrimSpace(s.cfg.ChatModel) == "" {
		s.cfg.ChatModel = chatModel
	}

	return s.createProviderForModel(chatModel)
}

func chatModelForInjectedProviderFactory(cfg *config.AIConfig) string {
	if cfg == nil {
		return ""
	}
	for _, candidate := range []string{
		cfg.GetChatModel(),
		cfg.GetModel(),
		config.DefaultModelForProvider(config.AIProviderOpenAI),
	} {
		if model := strings.TrimSpace(candidate); model != "" {
			return model
		}
	}
	return ""
}

func (s *Service) applyChatContextSettings() {
	StatelessContext = DefaultStatelessContext
}

// buildSystemPrompt builds the base system prompt for the AI.
// Mode-specific context (autonomous vs controlled) is added dynamically by the AgenticLoop.
//
// Philosophy: This prompt provides identity, context, and tool policy. Tool
// selection remains model-owned; Pulse enforces safety after a model choice via
// tool policy, approvals, and FSM verification gates.
func (s *Service) buildSystemPrompt() string {
	return s.buildSystemPromptWithToolGovernance(s.buildToolGovernancePromptSection())
}

func (s *Service) buildSystemPromptForOfferedTools(offeredTools []providers.Tool) string {
	return s.buildSystemPromptWithToolGovernance(s.buildToolGovernancePromptSectionForOfferedTools(offeredTools))
}

func (s *Service) buildSystemPromptWithToolGovernance(toolGovernance string) string {
	return `You are Pulse AI, a knowledgeable infrastructure assistant. You pair-program with the user on their homelab and infrastructure tasks.

` + toolGovernance + `

## INFRASTRUCTURE TOPOLOGY
- Resources are organized hierarchically: nodes → VMs/containers → Docker containers
- target_host specifies where commands run (host name, container name, or VM name)
- Commands execute inside the target: target_host="homepage-docker" runs inside that container
- For Docker containers inside system containers: target the container, then use docker commands
- The placeholder current_resource is valid only when this turn includes Pulse resource context that explicitly says an attached resource is selected, either from a resource-context handoff or from Pulse backend resource-reference resolution. If no attached resource context is present, do not use target_host="current_resource" or resource_id="current_resource"; ask which host, VM, container, app, or storage resource the user means, or use read-only query/search tools to identify an explicit target first.

## DOCKER BIND MOUNTS
- Container files are often mapped to host paths via bind mounts
- To edit a container's config, find the bind mount and edit the host path
- Bind mount mappings may be available in Pulse discovery context or tools when needed

## TOOL SELECTION
- Tool action modes and approval policies are generated from Pulse's tool registry. Treat the available-tool manifest for this turn as the source of truth for which tools exist and whether each tool is read-only, mixed, or write-capable.
- Use only tools that are explicitly offered in this turn's available-tool manifest.
- Write tools change infrastructure state. Mixed tools may include read subactions and write or decision-recording subactions; follow the governed path described in the manifest.
- Not every VM or container supports control. Some API-backed platforms are read-only even when the resource type is "vm" or "system-container".
- Write tools are allowed only when the user explicitly asks you to perform an action.
- Status checks and monitoring are read-oriented; do not change state unless the user asked for a state change.
- If a structured clarification tool is offered and you are missing critical information (target, risky choice, preference), use it.
- If no structured clarification tool is offered, ask for missing information in normal assistant text.
- Missing target information is not a safe default. In autonomous mode, ask for the missing target in normal assistant text instead of attempting a tool call with current_resource or another placeholder.

## HOW TO RESPOND
You are like a colleague doing pair programming on infrastructure tasks. Tool calls are your internal investigation — the user sees your final synthesized response.

1. INVESTIGATE THOROUGHLY: Decide whether tool evidence is needed, then gather enough information to answer well. Don't stop after the first tool call if more context would help.

2. SYNTHESIZE YOUR FINDINGS: After using tools, explain what you learned and did. Don't just confirm "done" — provide context that helps the user understand the outcome.

3. SURFACE ISSUES PROACTIVELY: If you discover something during investigation that affects the user's goal (prerequisites missing, config issues, limitations), mention it. Don't hide problems.

4. SUGGEST NEXT STEPS: If there's something the user might need to do next, or if you noticed a potential improvement, mention it.

5. BE DIRECT: Acknowledge mistakes or complications honestly. If something won't work as the user expects, say so clearly.

6. KEEP FORMATTING CLEAN: Use concise headings, bullets, and tables where they help. Do not use emoji, warning icons, or decorative symbols in normal operational answers unless the user explicitly asks for that tone.

## GROUNDING & PROVENANCE
Pulse context carries provenance: discovered facts include the source that produced them and a confidence, discovery context states when it was last gathered ("Last discovered: 2 days ago"), and metrics and events carry timestamps.
- When you state a fact drawn from this context, attribute it briefly so the user can trust and verify it — name the source for a discovered fact ("Debian 12, per /etc/os-release") and note recency for time-sensitive claims ("as of the last poll", "the discovery is 2 days old").
- Do not present stale or cached context as current. If the discovery is old or the underlying state may have changed since, say so and offer to re-check live.
- Keep attribution concise and inline; do not append a citation to every sentence or clutter the answer.

## TASK COMPLETION
- After successful control actions, respond once you have enough evidence to explain the result.
- If a tool call is blocked, treat the result as a policy boundary, not a tool-routing instruction.
- Use the returned facts and policy boundary to decide whether more context, a different governed tool call, or user clarification is needed.`
}

func (s *Service) buildToolGovernancePromptSection() string {
	return s.buildToolGovernancePromptSectionForOfferedToolNames(nil)
}

func (s *Service) buildToolGovernancePromptSectionForOfferedTools(offeredTools []providers.Tool) string {
	offeredNames := make(map[string]bool, len(offeredTools))
	orderedOfferedNames := make([]string, 0, len(offeredTools))
	for _, tool := range offeredTools {
		name := strings.TrimSpace(tool.Name)
		if name == "" || offeredNames[name] {
			continue
		}
		offeredNames[name] = true
		orderedOfferedNames = append(orderedOfferedNames, name)
	}
	return s.buildToolGovernancePromptSectionForOfferedToolNames(orderedOfferedNames)
}

func (s *Service) buildToolGovernancePromptSectionForOfferedToolNames(orderedOfferedNames []string) string {
	offeredFilter := map[string]bool(nil)
	if orderedOfferedNames != nil {
		offeredFilter = make(map[string]bool, len(orderedOfferedNames))
		for _, name := range orderedOfferedNames {
			if trimmed := strings.TrimSpace(name); trimmed != "" {
				offeredFilter[trimmed] = true
			}
		}
	}

	if offeredFilter != nil && len(offeredFilter) == 0 {
		return strings.Join([]string{
			"## AVAILABLE TOOL GOVERNANCE",
			"No Pulse tools are offered for this turn. Answer directly from the user's message and safe conversation context.",
			"Do not claim to inspect infrastructure, run commands, or use Pulse data unless that evidence is already present in the conversation.",
		}, "\n")
	}

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
	emitted := make(map[string]bool, len(manifest))
	for _, tool := range manifest {
		if offeredFilter != nil && !offeredFilter[tool.Name] {
			continue
		}
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
		emitted[tool.Name] = true
	}

	if offeredFilter == nil || offeredFilter[pulseQuestionToolName] {
		b.WriteString("- pulse_question: mode=interactive; approval=user answer required; asks the user for missing information using a structured prompt in interactive chat only.\n")
		emitted[pulseQuestionToolName] = true
	}
	if offeredFilter != nil {
		for _, name := range orderedOfferedNames {
			if emitted[name] {
				continue
			}
			b.WriteString(fmt.Sprintf("- %s: mode=read; approval=no approval required; Offered by Pulse for this turn.\n", name))
			emitted[name] = true
		}
	}
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
			ActionMode:     tools.ToolActionMixed,
			ApprovalPolicy: "no approval required; run uses read-only evidence collection and updates the discovery cache",
			Summary:        "Reads or refreshes discovered service paths, ports, and bind mounts for known resources.",
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

func (s *Service) injectRecentSessionContext(sessionID string, messages []Message, sessions *SessionStore) {
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

	s.mu.RLock()
	resourceProvider := s.unifiedResourceProvider
	s.mu.RUnlock()

	var lines []string
	primaryName := ""
	primaryResourceID := ""
	primaryTarget := ""
	primaryKind := ""
	primaryAdapter := ""
	primaryAllowsRoutingHint := true
	for _, resourceID := range recentIDs {
		res, ok := resolvedCtx.GetResourceByID(resourceID)
		if !ok || res == nil {
			continue
		}
		policy, aiSafeSummary := recentSessionResourceGovernance(resourceProvider, resourceID, res)
		label := res.Name
		if label == "" {
			label = resourceID
		}
		if policy != nil || strings.TrimSpace(aiSafeSummary) != "" {
			if safeLabel := unifiedresources.ResourcePolicyLabel(label, aiSafeSummary, policy); safeLabel != "" {
				label = safeLabel
			} else {
				label = unifiedresources.ResourcePolicyRedactedValue(label, policy,
					unifiedresources.ResourceRedactionAlias,
					unifiedresources.ResourceRedactionHostname,
					unifiedresources.ResourceRedactionPlatformID,
				)
			}
		}
		kind := res.Kind
		if kind == "" {
			kind = res.ResourceType
		}
		location := res.Node
		if location == "" {
			location = res.Scope.HostName
		}
		if policy != nil && location != "" {
			location = unifiedresources.ResourcePolicyRedactedValue(location, policy,
				unifiedresources.ResourceRedactionHostname,
				unifiedresources.ResourceRedactionAlias,
				unifiedresources.ResourceRedactionPlatformID,
			)
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
			primaryAllowsRoutingHint = policy == nil
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
	if primaryAllowsRoutingHint {
		readHint := readRoutingHintForResolvedResource(primaryKind, primaryAdapter, primaryTarget, primaryName, primaryResourceID)
		targetHint := readHint.targetFact()
		if targetHint != "" {
			lines[0] = lines[0] + "; " + targetHint
		}
	}

	log.Debug().
		Str("session_id", sessionID).
		Strs("recent_resource_ids", recentIDs).
		Msg("[ChatService] Injecting neutral recent session context")

	if len(messages) == 0 {
		return
	}
	lastIdx := len(messages) - 1
	if messages[lastIdx].Role != "user" {
		return
	}

	context := "Session context from earlier Assistant turns. Use only if relevant to the user's message; otherwise ignore it or ask a clarifying question.\n" + strings.Join(lines, "\n")
	messages[lastIdx].Content = context + "\n\n---\nUser message:\n" + messages[lastIdx].Content
}

func recentSessionResourceGovernance(provider tools.UnifiedResourceProvider, resourceID string, res *ResolvedResource) (*unifiedresources.ResourcePolicy, string) {
	if provider == nil || res == nil {
		return nil, ""
	}
	resourceType := res.Kind
	if resourceType == "" {
		resourceType = res.ResourceType
	}
	node := res.Node
	if node == "" {
		node = res.Scope.HostName
	}
	resource, ok := tools.CanonicalHandoffUnifiedResource(provider, firstNonEmptyRecentString(resourceID, res.ResourceID, res.ProviderUID), res.Name, resourceType, node)
	if !ok && res.ResourceID != "" && res.ResourceID != resourceID {
		resource, ok = tools.CanonicalHandoffUnifiedResource(provider, res.ResourceID, res.Name, resourceType, node)
	}
	if !ok && res.ProviderUID != "" {
		resource, ok = tools.CanonicalHandoffUnifiedResource(provider, res.ProviderUID, res.Name, resourceType, node)
	}
	if !ok {
		return nil, ""
	}
	return unifiedresources.CanonicalGovernanceMetadata(&resource)
}

func firstNonEmptyRecentString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (s *Service) toolsForExecutionMode(autonomousMode bool, patrolMode bool) []providers.Tool {
	mcpTools := s.executor.ListTools()
	providerTools := ConvertMCPToolsToProvider(mcpTools)

	// Patrol may be scoped by explicit product configuration, but never by
	// prompt keyword inference. The selected Patrol model owns tool choice.
	if patrolMode {
		filtered := s.filterToolsForPatrol(providerTools)
		log.Debug().
			Int("total_tools", len(providerTools)).
			Int("tool_manifest_count", len(filtered)).
			Bool("patrol_scope_filter", true).
			Msg("[toolsForExecutionMode] Built Patrol tool manifest from configured subsystem scope")
		return filtered
	}

	filtered := append([]providers.Tool{}, providerTools...)
	// pulse_question is interactive; exclude it for autonomous runs (Pulse Patrol).
	if !autonomousMode {
		filtered = append(filtered, userQuestionTool())
	}
	log.Debug().
		Int("total_tools", len(providerTools)).
		Int("tool_manifest_count", len(filtered)).
		Bool("autonomous", autonomousMode).
		Msg("[toolsForExecutionMode] Exposing governed tool manifest")

	return filtered
}

type assistantTurnToolScope string

const (
	assistantTurnToolScopeFull      assistantTurnToolScope = "full"
	assistantTurnToolScopeQueryOnly assistantTurnToolScope = "query-only"
	assistantTurnToolScopeTextOnly  assistantTurnToolScope = "text-only"
)

const assistantLocalInventoryModelRoute = "pulse:local-inventory"

func (s *Service) toolsForAssistantTurn(prompt string, autonomousMode bool, patrolMode bool, hasModelContext bool) []providers.Tool {
	scope := assistantToolScopeForPrompt(prompt, hasModelContext)
	return s.toolsForAssistantTurnWithScope(scope, autonomousMode, patrolMode)
}

func (s *Service) toolsForAssistantTurnWithScope(scope assistantTurnToolScope, autonomousMode bool, patrolMode bool) []providers.Tool {
	fullTools := s.toolsForExecutionMode(autonomousMode, patrolMode)
	if patrolMode {
		return fullTools
	}

	switch scope {
	case assistantTurnToolScopeTextOnly:
		log.Debug().
			Str("tool_scope", string(scope)).
			Bool("autonomous", autonomousMode).
			Msg("[toolsForAssistantTurn] Withholding tools for direct text turn")
		return []providers.Tool{}
	case assistantTurnToolScopeQueryOnly:
		allowed := map[string]bool{"pulse_query": true}
		if !autonomousMode {
			allowed[pulseQuestionToolName] = true
		}
		filtered := filterProviderToolsByName(fullTools, allowed)
		log.Debug().
			Str("tool_scope", string(scope)).
			Int("tool_manifest_count", len(filtered)).
			Bool("autonomous", autonomousMode).
			Msg("[toolsForAssistantTurn] Scoped Assistant turn to canonical query tools")
		return filtered
	default:
		return fullTools
	}
}

func filterProviderToolsByName(providerTools []providers.Tool, allowed map[string]bool) []providers.Tool {
	if len(providerTools) == 0 || len(allowed) == 0 {
		return []providers.Tool{}
	}
	filtered := make([]providers.Tool, 0, len(providerTools))
	for _, tool := range providerTools {
		if allowed[tool.Name] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func assistantToolScopeForPrompt(prompt string, hasModelContext bool) assistantTurnToolScope {
	if hasModelContext {
		return assistantTurnToolScopeFull
	}
	normalized := normalizeAssistantToolRoutingPrompt(prompt)
	if normalized == "" {
		return assistantTurnToolScopeTextOnly
	}
	if assistantPromptRequestsDirectText(normalized) || assistantPromptDisallowsToolUse(normalized) {
		return assistantTurnToolScopeTextOnly
	}
	if assistantPromptNeedsFullToolManifest(normalized) {
		return assistantTurnToolScopeFull
	}
	if assistantPromptIsInventoryQuery(normalized) {
		return assistantTurnToolScopeQueryOnly
	}
	if assistantPromptLooksConversational(normalized) {
		return assistantTurnToolScopeTextOnly
	}
	return assistantTurnToolScopeFull
}

func normalizeAssistantToolRoutingPrompt(prompt string) string {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	if normalized == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"\n", " ",
		"\r", " ",
		"\t", " ",
		".", " ",
		",", " ",
		";", " ",
		":", " ",
		"!", " ",
		"?", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
		"`", " ",
		"\"", " ",
		"'", " ",
	)
	return " " + strings.Join(strings.Fields(replacer.Replace(normalized)), " ") + " "
}

func assistantPromptRequestsDirectText(normalized string) bool {
	return assistantPromptContainsAny(normalized, []string{
		" reply exactly ",
		" respond exactly ",
		" answer exactly ",
		" say exactly ",
		" output exactly ",
		" print exactly ",
		" echo exactly ",
		" repeat exactly ",
		" just say ",
		" say only ",
		" reply only ",
		" respond only ",
	})
}

func assistantPromptDisallowsToolUse(normalized string) bool {
	return assistantPromptContainsAny(normalized, []string{
		" no tools ",
		" without tools ",
		" without using tools ",
		" do not use tools ",
		" don't use tools ",
		" dont use tools ",
		" before using tools ",
		" before using any tools ",
		" before you use tools ",
		" before calling tools ",
		" before you call tools ",
	})
}

func assistantPromptNeedsFullToolManifest(normalized string) bool {
	if assistantPromptExplicitlyRequestsToolUse(normalized) || assistantPromptLooksLikeShellRead(normalized) {
		return true
	}
	return assistantPromptContainsAny(normalized, []string{
		" run ",
		" execute ",
		" command ",
		" shell ",
		" terminal ",
		" ssh ",
		" log ",
		" logs ",
		" journal ",
		" tail ",
		" grep ",
		" cat ",
		" file ",
		" path ",
		" inspect ",
		" troubleshoot ",
		" diagnose ",
		" investigate ",
		" fix ",
		" repair ",
		" restart ",
		" start ",
		" stop ",
		" shutdown ",
		" reboot ",
		" delete ",
		" remove ",
		" edit ",
		" change ",
		" configure ",
		" create ",
		" install ",
		" update ",
		" upgrade ",
		" deploy ",
		" scale ",
		" backup ",
		" restore ",
		" snapshot ",
		" approve ",
		" deny ",
		" alert ",
		" alerts ",
		" finding ",
		" findings ",
		" cpu ",
		" memory ",
		" disk ",
		" temperature ",
		" latency ",
		" error ",
		" errors ",
	})
}

func assistantPromptExplicitlyRequestsToolUse(normalized string) bool {
	return assistantPromptContainsAny(normalized, []string{
		" use tools ",
		" use a tool ",
		" use the tool ",
		" use read-only tools ",
		" use read only tools ",
		" using read-only tools ",
		" using read only tools ",
		" read-only tools only ",
		" read only tools only ",
		" one read-only tool ",
		" one read only tool ",
		" call a tool ",
		" call the tool ",
		" call pulse_read ",
		" pulse_read ",
		" pulse_control ",
		" pulse_query ",
		" read attempt ",
		" read-only attempt ",
		" read only attempt ",
		" fresh verification ",
		" live runtime state ",
	})
}

func assistantPromptLooksLikeShellRead(normalized string) bool {
	shellCommands := []string{
		" awk ",
		" cat ",
		" df ",
		" docker ",
		" du ",
		" find ",
		" grep ",
		" head ",
		" journalctl ",
		" kubectl ",
		" ls ",
		" pct ",
		" ps ",
		" qm ",
		" sed ",
		" smartctl ",
		" systemctl ",
		" tail ",
		" wc ",
		" zfs ",
		" zpool ",
	}
	if strings.Contains(normalized, " | ") && assistantPromptContainsAny(normalized, shellCommands) {
		return true
	}
	if assistantPromptContainsAny(normalized, []string{
		" /dev",
		" /etc/",
		" /proc",
		" /sys",
		" /var/log",
	}) && assistantPromptContainsAny(normalized, shellCommands) {
		return true
	}
	return assistantPromptContainsAny(normalized, []string{
		" docker logs ",
		" journalctl -",
		" kubectl logs ",
		" ls /",
		" pct exec ",
		" qm guest exec ",
		" systemctl status ",
	})
}

func assistantPromptIsInventoryQuery(normalized string) bool {
	hasInventoryNoun := assistantPromptContainsAny(normalized, []string{
		" device ",
		" devices ",
		" resource ",
		" resources ",
		" system ",
		" systems ",
		" node ",
		" nodes ",
		" workload ",
		" workloads ",
		" vm ",
		" vms ",
		" container ",
		" containers ",
		" lxc ",
		" lxcs ",
		" host ",
		" hosts ",
		" service ",
		" services ",
		" cluster ",
		" clusters ",
		" inventory ",
		" inventories ",
	})
	if !hasInventoryNoun {
		return false
	}
	return assistantPromptContainsAny(normalized, []string{
		" how many ",
		" count ",
		" counts ",
		" total ",
		" totals ",
		" list ",
		" show ",
		" inventory ",
		" breakdown ",
		" detail ",
		" details ",
		" detailed ",
		" overview ",
		" status ",
		" running ",
		" stopped ",
		" online ",
		" offline ",
		" what is in ",
		" what s in ",
		" what are in ",
		" what do i have ",
	})
}

func assistantPromptRequestsDeterministicInventoryCount(normalized string) bool {
	if !assistantPromptIsInventoryQuery(normalized) {
		return false
	}
	if assistantPromptContainsAny(normalized, []string{
		" list ",
		" show ",
		" breakdown ",
		" detail ",
		" details ",
		" detailed ",
		" overview ",
		" status ",
	}) {
		return false
	}
	return assistantPromptContainsAny(normalized, []string{
		" how many ",
		" count ",
		" counts ",
		" total ",
		" totals ",
		" number ",
		" number of ",
	})
}

func assistantPromptLooksConversational(normalized string) bool {
	trimmed := strings.TrimSpace(normalized)
	if trimmed == "" {
		return true
	}
	if assistantPromptContainsAny(normalized, []string{
		" hello ",
		" hi ",
		" hey ",
		" thanks ",
		" thank you ",
		" who are you ",
		" what can you do ",
	}) {
		return true
	}
	// Do NOT treat a prompt as conversational just because it is short. Short
	// prompts are usually resource lookups ("hows esphome", "check frigate",
	// "grafana cpu") that NEED tools — withholding tools by word count made the
	// Assistant tell the user it had no way to inspect their infrastructure.
	// Only the explicit greeting/meta patterns above are conversational; let the
	// model decide whether to actually use the tools it is offered.
	return false
}

func assistantPromptContainsAny(normalized string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(normalized, needle) {
			return true
		}
	}
	return false
}

// filterToolsForPatrol applies explicit Patrol subsystem scope settings. It
// must not inspect prompt text or infer which tool the model should use.
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
