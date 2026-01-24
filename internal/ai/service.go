package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/types"
	"github.com/rs/zerolog/log"
)

// StateProvider provides access to the current infrastructure state
type StateProvider interface {
	GetState() models.StateSnapshot
}

// CommandPolicy defines the interface for command security policy
type CommandPolicy interface {
	Evaluate(command string) agentexec.PolicyDecision
}

// LicenseState represents the current state of the license
type LicenseState string

const (
	LicenseStateNone        LicenseState = "none"
	LicenseStateActive      LicenseState = "active"
	LicenseStateExpired     LicenseState = "expired"
	LicenseStateGracePeriod LicenseState = "grace_period"
)

// LicenseChecker provides license feature checking for Pro features
type LicenseChecker interface {
	HasFeature(feature string) bool
	// GetLicenseStateString returns the current license state (none, active, expired, grace_period)
	// and whether features are available (true for active/grace_period)
	GetLicenseStateString() (string, bool)
}

// AgentServer defines the interface for communicating with agents
type AgentServer interface {
	GetConnectedAgents() []agentexec.ConnectedAgent
	ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error)
}

// ChatServiceProvider defines the interface for accessing chat functionality
// This is used by the investigation orchestrator to run investigations
type ChatServiceProvider interface {
	CreateSession(ctx context.Context) (*ChatSession, error)
	ExecuteStream(ctx context.Context, req ChatExecuteRequest, callback ChatStreamCallback) error
	GetMessages(ctx context.Context, sessionID string) ([]ChatMessage, error)
	DeleteSession(ctx context.Context, sessionID string) error
}

// ChatSession represents a chat session (minimal interface for investigations)
type ChatSession struct {
	ID string `json:"id"`
}

// ChatExecuteRequest represents a chat execution request
type ChatExecuteRequest struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id,omitempty"`
}

// ChatStreamCallback is called for each streaming event
type ChatStreamCallback func(event ChatStreamEvent)

// ChatStreamEvent represents a streaming event
type ChatStreamEvent struct {
	Type string `json:"type"`
	Data []byte `json:"data,omitempty"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Service orchestrates AI interactions
type Service struct {
	mu               sync.RWMutex
	persistence      *config.ConfigPersistence
	provider         providers.Provider
	cfg              *config.AIConfig
	agentServer      AgentServer
	policy           CommandPolicy
	stateProvider    StateProvider
	alertProvider    AlertProvider
	knowledgeStore   *knowledge.Store
	costStore        *cost.Store
	resourceProvider ResourceProvider      // Unified resource model provider (Phase 2)
	patrolService    *PatrolService        // Background AI monitoring service
	metadataProvider MetadataProvider      // Enables AI to update resource URLs
	incidentStore    *memory.IncidentStore // Incident timelines for alert memory
	chatService      ChatServiceProvider   // Chat service for investigation orchestrator

	// Alert-triggered analysis - token-efficient real-time AI insights
	alertTriggeredAnalyzer *AlertTriggeredAnalyzer

	limits executionLimits

	modelsCache modelsCache

	// License checker for Pro feature gating
	licenseChecker LicenseChecker
}

type executionLimits struct {
	chatSlots   chan struct{}
	patrolSlots chan struct{}
}

type modelsCache struct {
	mu     sync.RWMutex
	at     time.Time
	key    string
	models []providers.ModelInfo
	ttl    time.Duration
}

// NewService creates a new AI service
func NewService(persistence *config.ConfigPersistence, agentServer AgentServer) *Service {
	// Initialize knowledge store
	var knowledgeStore *knowledge.Store
	costStore := cost.NewStore(cost.DefaultMaxDays)
	if persistence != nil {
		var err error
		knowledgeStore, err = knowledge.NewStore(persistence.DataDir())
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize knowledge store")
		}
		if err := costStore.SetPersistence(NewCostPersistenceAdapter(persistence)); err != nil {
			log.Warn().Err(err).Msg("Failed to initialize AI usage cost store")
		}
	}

	return &Service{
		persistence:    persistence,
		agentServer:    agentServer,
		policy:         agentexec.DefaultPolicy(),
		knowledgeStore: knowledgeStore,
		costStore:      costStore,
		limits: executionLimits{
			chatSlots:   make(chan struct{}, 4),
			patrolSlots: make(chan struct{}, 1),
		},
		modelsCache: modelsCache{
			ttl: 5 * time.Minute,
		},
	}
}

func (s *Service) acquireExecutionSlot(ctx context.Context, useCase string) (func(), error) {
	normalized := strings.TrimSpace(strings.ToLower(useCase))
	if normalized == "" {
		normalized = "chat"
	}

	var slots chan struct{}
	if normalized == "patrol" {
		slots = s.limits.patrolSlots
	} else {
		slots = s.limits.chatSlots
	}

	select {
	case slots <- struct{}{}:
		return func() { <-slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("Pulse Assistant is busy - too many concurrent requests")
	}
}

func (s *Service) enforceBudget(useCase string) error {
	s.mu.RLock()
	cfg := s.cfg
	store := s.costStore
	s.mu.RUnlock()

	if cfg == nil || cfg.CostBudgetUSD30d <= 0 || store == nil {
		return nil
	}

	summary := store.GetSummary(30)
	if !summary.Totals.PricingKnown {
		// We can't reliably enforce without pricing. Keep tracking and allow.
		return nil
	}

	if summary.Totals.EstimatedUSD >= cfg.CostBudgetUSD30d {
		normalized := strings.TrimSpace(strings.ToLower(useCase))
		if normalized == "" {
			normalized = "chat"
		}
		return fmt.Errorf("Pulse Assistant cost budget exceeded (%.2f/%.2f USD over %d days) - disable Pulse Assistant or raise budget to continue",
			summary.Totals.EstimatedUSD, cfg.CostBudgetUSD30d, summary.EffectiveDays)
	}

	return nil
}

// SetStateProvider sets the state provider for infrastructure context
func (s *Service) SetStateProvider(sp StateProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateProvider = sp

	// Initialize patrol service if not already done
	if s.patrolService == nil && sp != nil {
		s.patrolService = NewPatrolService(s, sp)
		// Connect knowledge store to patrol for per-resource notes in context
		if s.knowledgeStore != nil {
			s.patrolService.SetKnowledgeStore(s.knowledgeStore)
		}
		if s.incidentStore != nil {
			s.patrolService.SetIncidentStore(s.incidentStore)
		}
	}

	// Initialize alert-triggered analyzer if not already done
	if s.alertTriggeredAnalyzer == nil && sp != nil && s.patrolService != nil {
		s.alertTriggeredAnalyzer = NewAlertTriggeredAnalyzer(s.patrolService, sp)
	}
}

// GetStateProvider returns the state provider for infrastructure context
func (s *Service) GetStateProvider() StateProvider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stateProvider
}

// GetPatrolService returns the patrol service for background monitoring
func (s *Service) GetPatrolService() *PatrolService {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.patrolService
}

// SetChatService sets the chat service for investigation orchestrator
func (s *Service) SetChatService(cs ChatServiceProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chatService = cs
}

// GetChatService returns the chat service for investigation orchestrator
func (s *Service) GetChatService() ChatServiceProvider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.chatService
}

// GetRemediationLog returns the remediation log from the patrol service
func (s *Service) GetRemediationLog() *RemediationLog {
	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()
	if patrol == nil {
		return nil
	}
	return patrol.GetRemediationLog()
}

// GetAlertTriggeredAnalyzer returns the alert-triggered analyzer for token-efficient real-time analysis
func (s *Service) GetAlertTriggeredAnalyzer() *AlertTriggeredAnalyzer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.alertTriggeredAnalyzer
}

// GetAIConfig returns the current AI configuration
func (s *Service) GetAIConfig() *config.AIConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// GetCostSummary returns usage rollups for the last N days.
func (s *Service) GetCostSummary(days int) cost.Summary {
	s.mu.RLock()
	store := s.costStore
	s.mu.RUnlock()

	if store == nil {
		if days <= 0 {
			days = 30
		}
		effectiveDays := days
		truncated := false
		if cost.DefaultMaxDays > 0 && days > cost.DefaultMaxDays {
			effectiveDays = cost.DefaultMaxDays
			truncated = true
		}
		return cost.Summary{
			Days:           days,
			RetentionDays:  cost.DefaultMaxDays,
			EffectiveDays:  effectiveDays,
			Truncated:      truncated,
			PricingAsOf:    cost.PricingAsOf(),
			ProviderModels: []cost.ProviderModelSummary{},
			UseCases:       []cost.UseCaseSummary{},
			DailyTotals:    []cost.DailySummary{},
			Totals: cost.ProviderModelSummary{
				Provider: "all",
			},
		}
	}
	return store.GetSummary(days)
}

// ListCostEvents returns retained AI usage events within the requested time window.
func (s *Service) ListCostEvents(days int) []cost.UsageEvent {
	s.mu.RLock()
	store := s.costStore
	s.mu.RUnlock()
	if store == nil {
		return nil
	}
	return store.ListEvents(days)
}

// ClearCostHistory deletes retained AI usage events (admin operation).
func (s *Service) ClearCostHistory() error {
	s.mu.RLock()
	store := s.costStore
	s.mu.RUnlock()
	if store == nil {
		return nil
	}
	return store.Clear()
}

// SetPatrolThresholdProvider sets the threshold provider for patrol
// This should be called with an AlertThresholdAdapter to connect patrol to user-configured thresholds
func (s *Service) SetPatrolThresholdProvider(provider ThresholdProvider) {
	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol != nil {
		patrol.SetThresholdProvider(provider)
	}
}

// MetricsHistoryProvider provides access to historical metrics for trend analysis
// This interface matches the monitoring.MetricsHistory methods we need
type MetricsHistoryProvider interface {
	GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []MetricPoint
	GetGuestMetrics(guestID string, metricType string, duration time.Duration) []MetricPoint
	GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]MetricPoint
	GetAllStorageMetrics(storageID string, duration time.Duration) map[string][]MetricPoint
}

// MetricPoint is an alias for the shared metric point type
type MetricPoint = types.MetricPoint

// SetMetricsHistoryProvider sets the metrics history provider for enriched AI context
// This enables the AI to see trends, anomalies, and predictions based on historical data
func (s *Service) SetMetricsHistoryProvider(provider MetricsHistoryProvider) {
	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol != nil {
		patrol.SetMetricsHistoryProvider(provider)
	}
}

// SetBaselineStore sets the baseline store for anomaly detection
func (s *Service) SetBaselineStore(store *BaselineStore) {
	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol != nil {
		patrol.SetBaselineStore(store)
	}
}

// SetChangeDetector sets the change detector for operational memory
func (s *Service) SetChangeDetector(detector *ChangeDetector) {
	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol != nil {
		patrol.SetChangeDetector(detector)
	}
}

// SetRemediationLog sets the remediation log for tracking fix attempts
func (s *Service) SetRemediationLog(remLog *RemediationLog) {
	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol != nil {
		patrol.SetRemediationLog(remLog)
	}
}

// SetIncidentStore sets the incident store for alert timeline memory.
func (s *Service) SetIncidentStore(store *memory.IncidentStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.incidentStore = store

	if s.patrolService != nil {
		s.patrolService.SetIncidentStore(store)
	}
}

// SetPatternDetector sets the pattern detector for failure prediction
func (s *Service) SetPatternDetector(detector *PatternDetector) {
	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol != nil {
		patrol.SetPatternDetector(detector)
	}
}

// SetCorrelationDetector sets the correlation detector for multi-resource correlation
func (s *Service) SetCorrelationDetector(detector *CorrelationDetector) {
	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol != nil {
		patrol.SetCorrelationDetector(detector)
	}
}

// SetLicenseChecker sets the license checker for Pro feature gating
func (s *Service) SetLicenseChecker(checker LicenseChecker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.licenseChecker = checker
}

// HasLicenseFeature checks if a Pro feature is licensed (returns true if no license checker is set)
func (s *Service) HasLicenseFeature(feature string) bool {
	s.mu.RLock()
	checker := s.licenseChecker
	s.mu.RUnlock()

	if checker == nil {
		// No license checker means no enforcement (development mode or feature not gated)
		return true
	}
	return checker.HasFeature(feature)
}

// GetLicenseState returns the current license state and whether features are available
func (s *Service) GetLicenseState() (string, bool) {
	s.mu.RLock()
	checker := s.licenseChecker
	s.mu.RUnlock()

	if checker == nil {
		// No license checker means development mode - treat as active
		return string(LicenseStateActive), true
	}
	return checker.GetLicenseStateString()
}

// Feature constants are imported from the license package for compile-time consistency.
// This ensures the ai package always uses the same feature strings as the license system.
const (
	FeatureAIPatrol  = license.FeatureAIPatrol
	FeatureAIAlerts  = license.FeatureAIAlerts
	FeatureAIAutoFix = license.FeatureAIAutoFix
)

// StartPatrol starts the background patrol service
func (s *Service) StartPatrol(ctx context.Context) {
	s.mu.RLock()
	patrol := s.patrolService
	alertAnalyzer := s.alertTriggeredAnalyzer
	cfg := s.cfg
	licenseChecker := s.licenseChecker
	s.mu.RUnlock()

	if patrol == nil {
		log.Debug().Msg("Patrol service not initialized, cannot start")
		return
	}

	if cfg == nil || !cfg.IsPatrolEnabled() {
		log.Debug().Msg("AI Patrol not enabled")
		return
	}

	// Check license for AI Patrol feature (Pro only)
	if licenseChecker != nil && !licenseChecker.HasFeature(FeatureAIPatrol) {
		log.Info().Msg("AI Patrol requires Pulse Pro license - patrol will not run LLM analysis")
		// Note: We still configure patrol but it won't have LLM capability
		// The patrol service itself should check HasLicenseFeature before LLM calls
	}

	// Configure patrol from AI config (preserve defaults for resource types not in AI config)
	patrolCfg := DefaultPatrolConfig()
	patrolCfg.Enabled = true
	patrolCfg.Interval = cfg.GetPatrolInterval()
	patrolCfg.AnalyzeNodes = cfg.PatrolAnalyzeNodes
	patrolCfg.AnalyzeGuests = cfg.PatrolAnalyzeGuests
	patrolCfg.AnalyzeDocker = cfg.PatrolAnalyzeDocker
	patrolCfg.AnalyzeStorage = cfg.PatrolAnalyzeStorage
	patrol.SetConfig(patrolCfg)
	patrol.SetProactiveMode(cfg.UseProactiveThresholds)
	patrol.Start(ctx)

	// Configure alert-triggered analyzer (also Pro-only)
	if alertAnalyzer != nil {
		// Only enable if licensed for AI Alerts
		enabled := cfg.IsAlertTriggeredAnalysisEnabled()
		if enabled && licenseChecker != nil && !licenseChecker.HasFeature(FeatureAIAlerts) {
			log.Info().Msg("AI Alert Analysis requires Pulse Pro license - alert-triggered analysis disabled")
			enabled = false
		}
		alertAnalyzer.SetEnabled(enabled)
		alertAnalyzer.Start() // Start cleanup goroutine
		log.Info().
			Bool("enabled", enabled).
			Msg("Alert-triggered AI analysis configured")
	}

	// In demo/mock mode, inject realistic AI findings for showcasing
	if IsDemoMode() {
		patrol.InjectDemoFindings()
	}
}

// StopPatrol stops the background patrol service
func (s *Service) StopPatrol() {
	s.mu.RLock()
	patrol := s.patrolService
	alertAnalyzer := s.alertTriggeredAnalyzer
	s.mu.RUnlock()

	if patrol != nil {
		patrol.Stop()
	}
	if alertAnalyzer != nil {
		alertAnalyzer.Stop()
	}
}

// ReconfigurePatrol updates the patrol configuration without restarting
// Call this after changing patrol settings to apply them immediately
func (s *Service) ReconfigurePatrol() {
	s.mu.RLock()
	patrol := s.patrolService
	alertAnalyzer := s.alertTriggeredAnalyzer
	cfg := s.cfg
	licenseChecker := s.licenseChecker
	s.mu.RUnlock()

	if patrol == nil || cfg == nil {
		return
	}

	// Update patrol configuration (preserve defaults for resource types not in AI config)
	patrolCfg := DefaultPatrolConfig()
	patrolCfg.Enabled = cfg.IsPatrolEnabled()
	patrolCfg.Interval = cfg.GetPatrolInterval()
	patrolCfg.AnalyzeNodes = cfg.PatrolAnalyzeNodes
	patrolCfg.AnalyzeGuests = cfg.PatrolAnalyzeGuests
	patrolCfg.AnalyzeDocker = cfg.PatrolAnalyzeDocker
	patrolCfg.AnalyzeStorage = cfg.PatrolAnalyzeStorage
	patrol.SetConfig(patrolCfg)

	// Update proactive threshold mode
	patrol.SetProactiveMode(cfg.UseProactiveThresholds)

	log.Info().
		Bool("enabled", patrolCfg.Enabled).
		Dur("interval", patrolCfg.Interval).
		Bool("proactiveThresholds", cfg.UseProactiveThresholds).
		Msg("Patrol configuration updated")

	// Update alert-triggered analyzer (re-check license on each config change)
	if alertAnalyzer != nil {
		enabled := cfg.IsAlertTriggeredAnalysisEnabled()
		// Re-check license - don't allow re-enabling without valid license
		if enabled && licenseChecker != nil && !licenseChecker.HasFeature(FeatureAIAlerts) {
			log.Debug().Msg("Alert-triggered analysis requires Pulse Pro license - staying disabled")
			enabled = false
		}
		alertAnalyzer.SetEnabled(enabled)
	}
}

// enrichRequestFromFinding looks up a patrol finding by ID and enriches the request
// with the finding's context (node, resource ID, resource type, etc.)
// This ensures proper command routing when the AI helps fix a patrol finding.
// The function modifies the request in place.
func (s *Service) enrichRequestFromFinding(req *ExecuteRequest) {
	if req.FindingID == "" {
		return
	}

	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol == nil {
		log.Debug().Str("finding_id", req.FindingID).Msg("Cannot enrich request - patrol service not available")
		return
	}

	findings := patrol.GetFindings()
	if findings == nil {
		log.Debug().Str("finding_id", req.FindingID).Msg("Cannot enrich request - findings store not available")
		return
	}

	finding := findings.Get(req.FindingID)
	if finding == nil {
		log.Debug().Str("finding_id", req.FindingID).Msg("Cannot enrich request - finding not found")
		return
	}

	// Ensure context map exists
	if req.Context == nil {
		req.Context = make(map[string]interface{})
	}

	// Inject finding context (only if not already set)
	if _, ok := req.Context["node"]; !ok && finding.Node != "" {
		req.Context["node"] = finding.Node
	}
	if _, ok := req.Context["guestName"]; !ok && finding.ResourceName != "" {
		req.Context["guestName"] = finding.ResourceName
	}
	if req.TargetID == "" && finding.ResourceID != "" {
		req.TargetID = finding.ResourceID
	}
	if req.TargetType == "" && finding.ResourceType != "" {
		req.TargetType = finding.ResourceType
	}

	// Also store the finding details for reference in context
	req.Context["finding_resource_id"] = finding.ResourceID
	req.Context["finding_resource_name"] = finding.ResourceName
	req.Context["finding_resource_type"] = finding.ResourceType
	if finding.Node != "" {
		req.Context["finding_node"] = finding.Node
	}

	log.Debug().
		Str("finding_id", req.FindingID).
		Str("node", finding.Node).
		Str("resource_id", finding.ResourceID).
		Str("resource_name", finding.ResourceName).
		Str("resource_type", finding.ResourceType).
		Msg("Enriched request with finding context")
}

// GuestInfo contains information about a guest (VM or container) found by VMID lookup
type GuestInfo struct {
	Node     string
	Name     string
	Type     string // "lxc" or "qemu"
	Instance string // PVE instance ID (for multi-cluster disambiguation)
}

// lookupNodeForVMID looks up which node owns a given VMID using the state provider
// Returns the node name and guest name if found, empty strings otherwise
// If targetInstance is provided, only matches guests from that instance (for multi-cluster setups)
func (s *Service) lookupNodeForVMID(vmid int) (node string, guestName string, guestType string) {
	guests := s.lookupGuestsByVMID(vmid, "")
	if len(guests) == 1 {
		return guests[0].Node, guests[0].Name, guests[0].Type
	}
	if len(guests) > 1 {
		// Multiple matches - VMID collision across instances
		// Log warning and return first match (caller should use lookupGuestsByVMID with instance filter)
		log.Warn().
			Int("vmid", vmid).
			Int("matches", len(guests)).
			Msg("VMID collision detected - multiple guests with same VMID across instances")
		return guests[0].Node, guests[0].Name, guests[0].Type
	}
	return "", "", ""
}

// lookupGuestsByVMID finds all guests with the given VMID
// If targetInstance is non-empty, only returns guests from that instance
func (s *Service) lookupGuestsByVMID(vmid int, targetInstance string) []GuestInfo {
	s.mu.RLock()
	sp := s.stateProvider
	s.mu.RUnlock()

	if sp == nil {
		return nil
	}

	state := sp.GetState()
	var results []GuestInfo

	// Check containers
	for _, ct := range state.Containers {
		if ct.VMID == vmid {
			if targetInstance == "" || ct.Instance == targetInstance {
				results = append(results, GuestInfo{
					Node:     ct.Node,
					Name:     ct.Name,
					Type:     "lxc",
					Instance: ct.Instance,
				})
			}
		}
	}

	// Check VMs
	for _, vm := range state.VMs {
		if vm.VMID == vmid {
			if targetInstance == "" || vm.Instance == targetInstance {
				results = append(results, GuestInfo{
					Node:     vm.Node,
					Name:     vm.Name,
					Type:     "qemu",
					Instance: vm.Instance,
				})
			}
		}
	}

	return results
}

// extractVMIDFromCommand parses pct/qm/vzdump commands to extract the VMID being targeted
// Returns the VMID, whether it requires node-specific routing, and whether found
// Some commands (like vzdump) can run from any cluster node, others (like pct exec) must run on the owning node
func extractVMIDFromCommand(command string) (vmid int, requiresOwnerNode bool, found bool) {
	// Commands that MUST run on the node that owns the guest
	// These interact directly with the container/VM runtime
	nodeSpecificPatterns := []string{
		// pct commands - match any pct subcommand followed by VMID
		// Covers: exec, enter, start, stop, shutdown, reboot, status, push, pull, mount, unmount, etc.
		`(?:^|\s)pct\s+\w+\s+(\d+)`,
		// qm commands - match any qm subcommand followed by VMID
		// Covers: start, stop, shutdown, reset, status, guest exec, monitor, etc.
		`(?:^|\s)qm\s+(?:guest\s+)?\w+\s+(\d+)`,
	}

	// Commands that can run from any cluster node (cluster-aware)
	// vzdump uses the cluster to route to the right node automatically
	clusterAwarePatterns := []string{
		`(?:^|\s)vzdump\s+(\d+)`,
		// pvesh commands can specify node in path, so we don't force routing
	}

	// Check node-specific commands first (higher priority)
	for _, pattern := range nodeSpecificPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(command); len(matches) > 1 {
			if v, err := strconv.Atoi(matches[1]); err == nil {
				return v, true, true
			}
		}
	}

	// Check cluster-aware commands (don't force node routing)
	for _, pattern := range clusterAwarePatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(command); len(matches) > 1 {
			if v, err := strconv.Atoi(matches[1]); err == nil {
				return v, false, true
			}
		}
	}

	return 0, false, false
}

// formatApprovalNeededToolResult returns a structured tool result for commands that require approval.
// It is encoded as a marker + JSON so the LLM can reliably detect it.
func formatApprovalNeededToolResult(command, toolID, reason string) string {
	payload := map[string]interface{}{
		"type":           "approval_required",
		"command":        command,
		"tool_id":        toolID,
		"reason":         reason,
		"how_to_approve": "Ask the user to click the approval button shown in the UI.",
		"do_not_retry":   true,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		// Fallback to a safe plain-text marker.
		return fmt.Sprintf("APPROVAL_REQUIRED: %s", command)
	}
	return "APPROVAL_REQUIRED: " + string(b)
}

// formatPolicyBlockedToolResult returns a structured tool result for commands blocked by policy.
func formatPolicyBlockedToolResult(command, reason string) string {
	payload := map[string]interface{}{
		"type":         "policy_blocked",
		"command":      command,
		"reason":       reason,
		"do_not_retry": true,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("POLICY_BLOCKED: %s", reason)
	}
	return "POLICY_BLOCKED: " + string(b)
}

func parseApprovalNeededMarker(content string) (ApprovalNeededData, bool) {
	const prefix = "APPROVAL_REQUIRED:"
	if !strings.HasPrefix(content, prefix) {
		return ApprovalNeededData{}, false
	}
	trimmed := strings.TrimSpace(strings.TrimPrefix(content, prefix))
	if trimmed == "" {
		return ApprovalNeededData{}, false
	}

	var payload struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		ToolID  string `json:"tool_id"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return ApprovalNeededData{}, false
	}
	if payload.Type != "approval_required" || payload.Command == "" {
		return ApprovalNeededData{}, false
	}

	return ApprovalNeededData{
		Command: payload.Command,
		ToolID:  payload.ToolID,
	}, true
}

func approvalNeededFromToolCall(req ExecuteRequest, tc providers.ToolCall, result string) (ApprovalNeededData, bool) {
	if !strings.HasPrefix(result, "APPROVAL_REQUIRED:") {
		return ApprovalNeededData{}, false
	}
	if tc.Name != "run_command" {
		return ApprovalNeededData{}, false
	}

	cmd, _ := tc.Input["command"].(string)
	runOnHost, _ := tc.Input["run_on_host"].(bool)
	targetHost, _ := tc.Input["target_host"].(string)

	if targetHost == "" {
		if node, ok := req.Context["node"].(string); ok && node != "" {
			targetHost = node
		} else if node, ok := req.Context["hostname"].(string); ok && node != "" {
			targetHost = node
		} else if node, ok := req.Context["host_name"].(string); ok && node != "" {
			targetHost = node
		}
	}

	return ApprovalNeededData{
		Command:    cmd,
		ToolID:     tc.ID,
		ToolName:   tc.Name,
		RunOnHost:  runOnHost,
		TargetHost: targetHost,
	}, true
}

// LoadConfig loads the AI configuration and initializes the provider
func (s *Service) LoadConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := s.persistence.LoadAIConfig()
	if err != nil {
		return fmt.Errorf("failed to load Pulse Assistant config: %w", err)
	}

	s.cfg = cfg

	// Don't initialize provider if AI is not enabled or not configured
	if cfg == nil || !cfg.Enabled || !cfg.IsConfigured() {
		s.provider = nil
		return nil
	}

	selectedModel := cfg.GetModel()
	selectedProvider, _ := config.ParseModelString(selectedModel)

	providerClient, err := providers.NewForModel(cfg, selectedModel)
	if err != nil {
		// Only fall back to legacy config if no multi-provider credentials are set.
		if len(cfg.GetConfiguredProviders()) == 0 && (cfg.Provider != "" || cfg.APIKey != "") {
			if legacyClient, legacyErr := providers.NewFromConfig(cfg); legacyErr == nil {
				providerClient = legacyClient
				selectedProvider = providerClient.Name()
				log.Info().
					Str("provider", selectedProvider).
					Str("model", cfg.GetModel()).
					Msg("AI service initialized via legacy config (migration path)")
			} else {
				log.Warn().Err(legacyErr).Msg("Failed to initialize legacy AI provider")
				s.provider = nil
				return nil
			}
		} else {
			// Smart fallback: if selected provider isn't configured but OTHER providers are,
			// automatically switch to a model from a configured provider.
			// This prevents confusing errors when the user has e.g. DeepSeek configured
			// but the model is still set to an Anthropic model.
			configuredProviders := cfg.GetConfiguredProviders()
			if len(configuredProviders) > 0 {
				fallbackProvider := configuredProviders[0]
				var fallbackModel string
				switch fallbackProvider {
				case config.AIProviderAnthropic:
					fallbackModel = config.AIProviderAnthropic + ":" + config.DefaultAIModelAnthropic
				case config.AIProviderOpenAI:
					fallbackModel = config.AIProviderOpenAI + ":" + config.DefaultAIModelOpenAI
				case config.AIProviderDeepSeek:
					fallbackModel = config.AIProviderDeepSeek + ":" + config.DefaultAIModelDeepSeek
				case config.AIProviderGemini:
					fallbackModel = config.AIProviderGemini + ":" + config.DefaultAIModelGemini
				case config.AIProviderOllama:
					fallbackModel = config.AIProviderOllama + ":" + config.DefaultAIModelOllama
				}

				if fallbackModel != "" {
					log.Warn().
						Str("selected_model", selectedModel).
						Str("selected_provider", selectedProvider).
						Str("fallback_model", fallbackModel).
						Str("fallback_provider", fallbackProvider).
						Msg("Selected provider not configured - automatically falling back to configured provider")

					providerClient, err = providers.NewForModel(cfg, fallbackModel)
					if err == nil {
						selectedModel = fallbackModel
						selectedProvider = fallbackProvider
					} else {
						log.Error().Err(err).Str("fallback_model", fallbackModel).Msg("Failed to create fallback provider")
						s.provider = nil
						return nil
					}
				}
			}

			if providerClient == nil {
				log.Warn().
					Err(err).
					Str("selected_model", selectedModel).
					Str("selected_provider", selectedProvider).
					Strs("configured_providers", cfg.GetConfiguredProviders()).
					Msg("AI enabled but no providers configured")
				s.provider = nil
				return nil
			}
		}
	}

	s.provider = providerClient
	log.Info().
		Str("provider", selectedProvider).
		Str("model", selectedModel).
		Str("control_level", cfg.GetControlLevel()).
		Bool("autonomous", cfg.IsAutonomous()).
		Msg("AI service initialized")

	return nil
}

// IsEnabled returns true if AI is enabled and configured
func (s *Service) IsEnabled() bool {
	// In demo mode, AI is always considered enabled (using mock backend)
	if IsDemoMode() {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg != nil && s.cfg.Enabled && s.provider != nil
}

// QuickAnalysis performs a lightweight AI analysis for simple decisions.
// This is used for things like determining if an alert should be auto-resolved.
// It uses a single-turn, no-tools call for efficiency.
func (s *Service) QuickAnalysis(ctx context.Context, prompt string) (string, error) {
	s.mu.RLock()
	provider := s.provider
	cfg := s.cfg
	s.mu.RUnlock()

	if provider == nil {
		return "", fmt.Errorf("Pulse Assistant is not enabled or configured")
	}

	// Use a fast model for quick analysis if available
	model := ""
	if cfg != nil && cfg.PatrolModel != "" {
		model = cfg.PatrolModel
	}

	messages := []providers.Message{
		{
			Role:    "system",
			Content: "You are a brief, decisive infrastructure monitoring assistant. Give short, direct answers.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	resp, err := provider.Chat(ctx, providers.ChatRequest{
		Messages: messages,
		Model:    model,
	})
	if err != nil {
		return "", fmt.Errorf("Pulse Assistant analysis failed: %w", err)
	}

	if resp.Content == "" {
		return "", fmt.Errorf("Pulse Assistant returned empty response")
	}

	return resp.Content, nil
}

// GetConfig returns a copy of the current AI config
func (s *Service) GetConfig() *config.AIConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg == nil {
		return nil
	}
	cfg := *s.cfg
	return &cfg
}

// GetDebugContext returns debug information about what context would be sent to the AI
func (s *Service) GetDebugContext(req ExecuteRequest) map[string]interface{} {
	s.mu.RLock()
	stateProvider := s.stateProvider
	agentServer := s.agentServer
	cfg := s.cfg
	s.mu.RUnlock()

	result := make(map[string]interface{})

	// State provider info
	result["has_state_provider"] = stateProvider != nil
	if stateProvider != nil {
		state := stateProvider.GetState()
		result["state_summary"] = map[string]interface{}{
			"nodes":         len(state.Nodes),
			"vms":           len(state.VMs),
			"containers":    len(state.Containers),
			"docker_hosts":  len(state.DockerHosts),
			"hosts":         len(state.Hosts),
			"pbs_instances": len(state.PBSInstances),
		}

		// List some VMs/containers for verification
		var vmNames []string
		for _, vm := range state.VMs {
			vmNames = append(vmNames, fmt.Sprintf("%s (VMID:%d, node:%s)", vm.Name, vm.VMID, vm.Node))
		}
		if len(vmNames) > 10 {
			vmNames = vmNames[:10]
		}
		result["sample_vms"] = vmNames

		var ctNames []string
		for _, ct := range state.Containers {
			ctNames = append(ctNames, fmt.Sprintf("%s (VMID:%d, node:%s)", ct.Name, ct.VMID, ct.Node))
		}
		if len(ctNames) > 10 {
			ctNames = ctNames[:10]
		}
		result["sample_containers"] = ctNames

		var hostNames []string
		for _, h := range state.Hosts {
			hostNames = append(hostNames, h.Hostname)
		}
		result["host_names"] = hostNames

		var dockerHostNames []string
		for _, dh := range state.DockerHosts {
			dockerHostNames = append(dockerHostNames, fmt.Sprintf("%s (%d containers)", dh.DisplayName, len(dh.Containers)))
		}
		result["docker_host_names"] = dockerHostNames
	}

	// Agent info
	result["has_agent_server"] = agentServer != nil
	if agentServer != nil {
		agents := agentServer.GetConnectedAgents()
		var agentNames []string
		for _, a := range agents {
			agentNames = append(agentNames, a.Hostname)
		}
		result["connected_agents"] = agentNames
	}

	// Config info
	result["has_config"] = cfg != nil
	if cfg != nil {
		result["custom_context_length"] = len(cfg.CustomContext)
		if len(cfg.CustomContext) > 200 {
			result["custom_context_preview"] = cfg.CustomContext[:200] + "..."
		} else {
			result["custom_context_preview"] = cfg.CustomContext
		}
	}

	// Build and include the system prompt
	systemPrompt := s.buildSystemPrompt(req)
	result["system_prompt_length"] = len(systemPrompt)
	result["system_prompt"] = systemPrompt

	return result
}

// IsAutonomous returns true if autonomous mode is enabled AND licensed.
// Autonomous mode requires the ai_autofix license feature (Pro tier).
func (s *Service) IsAutonomous() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg == nil || !s.cfg.IsAutonomous() {
		return false
	}
	// Autonomous mode requires Pro license with ai_autofix feature
	if s.licenseChecker != nil && !s.licenseChecker.HasFeature(FeatureAIAutoFix) {
		return false
	}
	return true
}

// ConversationMessage represents a message in conversation history
type ConversationMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// ExecuteRequest represents a request to execute an AI prompt
type ExecuteRequest struct {
	Prompt       string                 `json:"prompt"`
	TargetType   string                 `json:"target_type,omitempty"` // "host", "container", "vm", "node"
	TargetID     string                 `json:"target_id,omitempty"`
	Context      map[string]interface{} `json:"context,omitempty"`       // Current metrics, state, etc.
	SystemPrompt string                 `json:"system_prompt,omitempty"` // Override system prompt
	History      []ConversationMessage  `json:"history,omitempty"`       // Previous conversation messages
	FindingID    string                 `json:"finding_id,omitempty"`    // If fixing a patrol finding, the ID to resolve
	Model        string                 `json:"model,omitempty"`         // Override model for this request (for user selection in chat)
	UseCase      string                 `json:"use_case,omitempty"`      // "chat" or "patrol" - determines which default model to use
}

// ExecuteResponse represents the AI's response
type ExecuteResponse struct {
	Content          string               `json:"content"`
	Model            string               `json:"model"`
	InputTokens      int                  `json:"input_tokens"`
	OutputTokens     int                  `json:"output_tokens"`
	ToolCalls        []ToolExecution      `json:"tool_calls,omitempty"`        // Commands that were executed
	PendingApprovals []ApprovalNeededData `json:"pending_approvals,omitempty"` // Commands that require approval (non-streaming)
}

// ToolExecution represents a tool that was executed during the AI conversation
type ToolExecution struct {
	Name    string `json:"name"`
	Input   string `json:"input"`  // Human-readable input (e.g., the command)
	Output  string `json:"output"` // Result of execution
	Success bool   `json:"success"`
}

// getModelForRequest determines which model to use for a request
// Priority: explicit override > use case default > config default
func (s *Service) getModelForRequest(req ExecuteRequest) string {
	// If request has explicit model override, use it
	if req.Model != "" {
		return req.Model
	}

	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	if cfg == nil {
		return ""
	}

	// Use case-specific model selection
	switch req.UseCase {
	case "patrol":
		return cfg.GetPatrolModel()
	case "chat":
		return cfg.GetChatModel()
	default:
		// Default to chat model for interactive requests
		return cfg.GetChatModel()
	}
}

// StreamEvent represents an event during AI execution for streaming
type StreamEvent struct {
	Type string      `json:"type"` // "tool_start", "tool_end", "content", "done", "error", "approval_needed"
	Data interface{} `json:"data,omitempty"`
}

// StreamCallback is called for each event during streaming execution
type StreamCallback func(event StreamEvent)

// ToolStartData is sent when a tool execution begins
type ToolStartData struct {
	Name  string `json:"name"`
	Input string `json:"input"`
}

// ToolEndData is sent when a tool execution completes
type ToolEndData struct {
	Name    string `json:"name"`
	Input   string `json:"input"`
	Output  string `json:"output"`
	Success bool   `json:"success"`
}

// ApprovalNeededData is sent when a command needs user approval
type ApprovalNeededData struct {
	Command    string `json:"command"`
	ToolID     string `json:"tool_id"`   // ID to reference when approving
	ToolName   string `json:"tool_name"` // "run_command"
	RunOnHost  bool   `json:"run_on_host"`
	TargetHost string `json:"target_host,omitempty"` // Explicit host to route to
	ApprovalID string `json:"approval_id,omitempty"` // ID of the approval record in the store
}

// Execute sends a prompt to the AI and returns the response
// If tools are available and the AI requests them, it executes them in a loop
func (s *Service) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	release, err := s.acquireExecutionSlot(ctx, req.UseCase)
	if err != nil {
		return nil, err
	}
	defer release()

	if err := s.enforceBudget(req.UseCase); err != nil {
		return nil, err
	}

	// Enrich request with finding context if this is a "help me fix" request
	s.enrichRequestFromFinding(&req)

	s.mu.RLock()
	defaultProvider := s.provider
	agentServer := s.agentServer
	cfg := s.cfg
	costStore := s.costStore
	s.mu.RUnlock()

	// Determine the model to use for this request
	modelString := s.getModelForRequest(req)

	// Create a provider for this specific model (supports multi-provider switching)
	provider, err := providers.NewForModel(cfg, modelString)
	if err != nil {
		// Fall back to default provider if model-specific provider can't be created
		log.Debug().Err(err).Str("model", modelString).Msg("Could not create provider for model, using default")
		provider = defaultProvider
	}

	if provider == nil {
		// In demo mode, return mock response if no provider is configured
		if IsDemoMode() {
			return GenerateDemoAIResponse(req.Prompt), nil
		}
		return nil, fmt.Errorf("Pulse Assistant is not enabled or configured")
	}

	// Build the system prompt
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = s.buildSystemPrompt(req)
	}

	// Check if agent is available for this target
	hasAgent := s.hasAgentForTarget(req)

	// Build tools list if agent is available
	var tools []providers.Tool
	if hasAgent && agentServer != nil {
		tools = s.getTools()
		systemPrompt += `

## Available Tools
You have access to tools to execute commands on the target system. You should:
1. Use run_command to investigate issues, gather information, and PERFORM actions
2. Actually execute the commands - don't just explain what commands to run
3. For Proxmox operations (resize disk, manage containers/VMs), run commands on the HOST (target_type=host)
4. For operations inside a container, run commands on the container (target_type=container)

Examples of actions you can perform:
- Resize LXC disk: pct resize <vmid> rootfs +10G (run on host)
- Check disk usage: df -h (run on container)
- View processes: ps aux --sort=-%mem | head -20
- Check logs: tail -100 /var/log/syslog

Always execute the commands rather than telling the user how to do it.`
	}

	// Inject previously learned knowledge about this guest
	if s.knowledgeStore != nil {
		guestID := s.getGuestID(req)
		if guestID != "" {
			if knowledgeContext := s.knowledgeStore.FormatForContext(guestID); knowledgeContext != "" {
				systemPrompt += knowledgeContext
			}
		}
	}

	// Build initial messages with conversation history
	var messages []providers.Message
	for _, histMsg := range req.History {
		messages = append(messages, providers.Message{
			Role:    histMsg.Role,
			Content: histMsg.Content,
		})
	}
	messages = append(messages, providers.Message{Role: "user", Content: req.Prompt})

	var toolExecutions []ToolExecution
	totalInputTokens := 0
	totalOutputTokens := 0
	var finalContent string
	var model string

	// Agentic loop - keep going while AI requests tools
	maxIterations := 10 // Safety limit
	for i := 0; i < maxIterations; i++ {
		if err := s.enforceBudget(req.UseCase); err != nil {
			return nil, err
		}

		resp, err := provider.Chat(ctx, providers.ChatRequest{
			Messages:  messages,
			Model:     s.getModelForRequest(req),
			System:    systemPrompt,
			MaxTokens: 4096,
			Tools:     tools,
		})
		if err != nil {
			return nil, fmt.Errorf("Pulse Assistant request failed: %w", err)
		}

		if costStore != nil {
			providerName, _ := config.ParseModelString(modelString)
			if providerName == "" {
				providerName = provider.Name()
			}
			costStore.Record(cost.UsageEvent{
				Timestamp:     time.Now(),
				Provider:      providerName,
				RequestModel:  modelString,
				ResponseModel: resp.Model,
				UseCase:       req.UseCase,
				InputTokens:   resp.InputTokens,
				OutputTokens:  resp.OutputTokens,
				TargetType:    req.TargetType,
				TargetID:      req.TargetID,
				FindingID:     req.FindingID,
			})
		}

		totalInputTokens += resp.InputTokens
		totalOutputTokens += resp.OutputTokens
		model = resp.Model
		finalContent = resp.Content

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 || resp.StopReason != "tool_use" {
			break
		}

		// Add assistant's response with tool calls to messages
		messages = append(messages, providers.Message{
			Role:             "assistant",
			Content:          resp.Content,
			ReasoningContent: resp.ReasoningContent, // DeepSeek thinking mode
			ToolCalls:        resp.ToolCalls,
		})

		// Execute each tool call and add results
		var pendingApprovals []ApprovalNeededData
		for _, tc := range resp.ToolCalls {
			result, execution := s.executeTool(ctx, req, tc)
			toolExecutions = append(toolExecutions, execution)

			if approval, ok := approvalNeededFromToolCall(req, tc, result); ok {
				pendingApprovals = append(pendingApprovals, approval)
				continue
			}

			// Add tool result to messages
			messages = append(messages, providers.Message{
				Role: "user",
				ToolResult: &providers.ToolResult{
					ToolUseID: tc.ID,
					Content:   result,
					IsError:   !execution.Success,
				},
			})

		}

		// Stop the agentic loop when approvals are required.
		// The caller can execute approvals via /api/ai/run-command and continue.
		if len(pendingApprovals) > 0 {
			return &ExecuteResponse{
				Content:          finalContent,
				Model:            model,
				InputTokens:      totalInputTokens,
				OutputTokens:     totalOutputTokens,
				ToolCalls:        toolExecutions,
				PendingApprovals: pendingApprovals,
			}, nil
		}
	}

	return &ExecuteResponse{
		Content:      finalContent,
		Model:        model,
		InputTokens:  totalInputTokens,
		OutputTokens: totalOutputTokens,
		ToolCalls:    toolExecutions,
	}, nil
}

// ExecuteStream sends a prompt to the AI and streams events via callback
// This allows the UI to show real-time progress during tool execution
func (s *Service) ExecuteStream(ctx context.Context, req ExecuteRequest, callback StreamCallback) (*ExecuteResponse, error) {
	release, err := s.acquireExecutionSlot(ctx, req.UseCase)
	if err != nil {
		return nil, err
	}
	defer release()

	if err := s.enforceBudget(req.UseCase); err != nil {
		return nil, err
	}

	// Enrich request with finding context if this is a "help me fix" request
	s.enrichRequestFromFinding(&req)

	s.mu.RLock()
	defaultProvider := s.provider
	agentServer := s.agentServer
	cfg := s.cfg
	costStore := s.costStore
	s.mu.RUnlock()

	// Determine the model to use for this request
	modelString := s.getModelForRequest(req)

	// Create a provider for this specific model (supports multi-provider switching)
	provider, err := providers.NewForModel(cfg, modelString)
	if err != nil {
		// Fall back to default provider if model-specific provider can't be created
		log.Debug().Err(err).Str("model", modelString).Msg("Could not create provider for model, using default")
		provider = defaultProvider
	}

	if provider == nil {
		// In demo mode, simulate streaming response if no provider is configured
		if IsDemoMode() {
			return GenerateDemoAIStream(req.Prompt, callback)
		}
		return nil, fmt.Errorf("Pulse Assistant is not enabled or configured")
	}

	// Build the system prompt
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = s.buildSystemPrompt(req)
	}

	// Debug log the system prompt length and key sections
	log.Debug().
		Int("prompt_length", len(systemPrompt)).
		Bool("has_infrastructure_map", strings.Contains(systemPrompt, "## Infrastructure Map")).
		Bool("has_docker_hosts", strings.Contains(systemPrompt, "### Docker Hosts")).
		Bool("has_standalone_hosts", strings.Contains(systemPrompt, "### Standalone Hosts")).
		Bool("has_guests", strings.Contains(systemPrompt, "### All Guests")).
		Msg("AI system prompt built")

	// Check if agent is available for this target
	hasAgent := s.hasAgentForTarget(req)

	// Build tools list if agent is available
	var tools []providers.Tool
	if hasAgent && agentServer != nil {
		tools = s.getTools()
		systemPrompt += `

## Available Tools
You have access to tools to execute commands on the target system. You should:
1. Use run_command to investigate issues, gather information, and PERFORM actions
2. Actually execute the commands - don't just explain what commands to run
3. For Proxmox operations (resize disk, manage containers/VMs), run commands on the HOST (target_type=host)
4. For operations inside a container, run commands on the container (target_type=container)

Examples of actions you can perform:
- Resize LXC disk: pct resize <vmid> rootfs +10G (run on host)
- Check disk usage: df -h (run on container)
- View processes: ps aux --sort=-%mem | head -20
- Check logs: tail -100 /var/log/syslog

Always execute the commands rather than telling the user how to do it.`
	}

	// Inject previously learned knowledge about this guest
	if s.knowledgeStore != nil {
		guestID := s.getGuestID(req)
		if guestID != "" {
			if knowledgeContext := s.knowledgeStore.FormatForContext(guestID); knowledgeContext != "" {
				log.Debug().
					Str("guest_id", guestID).
					Int("context_length", len(knowledgeContext)).
					Msg("Injecting saved knowledge into AI context")
				systemPrompt += knowledgeContext
			} else {
				log.Debug().Str("guest_id", guestID).Msg("No saved knowledge for guest")
			}
		}
	}

	// Build initial messages with conversation history
	var messages []providers.Message
	for _, histMsg := range req.History {
		messages = append(messages, providers.Message{
			Role:    histMsg.Role,
			Content: histMsg.Content,
		})
	}
	messages = append(messages, providers.Message{Role: "user", Content: req.Prompt})

	var toolExecutions []ToolExecution
	totalInputTokens := 0
	totalOutputTokens := 0
	var finalContent string
	var model string

	// Agentic loop - keep going while AI requests tools
	// No artificial iteration limit - the context timeout (5 minutes) provides the safety net
	iteration := 0
	for {
		iteration++
		log.Debug().
			Int("iteration", iteration).
			Int("message_count", len(messages)).
			Int("system_prompt_length", len(systemPrompt)).
			Int("tools_count", len(tools)).
			Msg("Calling AI provider...")

		// Send a processing event so the frontend knows we're making an AI call
		// This is especially important after tool execution when the next AI call can take a while
		if iteration > 1 {
			callback(StreamEvent{Type: "processing", Data: fmt.Sprintf("Analyzing results (iteration %d)...", iteration)})
		}

		if err := s.enforceBudget(req.UseCase); err != nil {
			callback(StreamEvent{Type: "error", Data: err.Error()})
			return nil, err
		}

		resp, err := provider.Chat(ctx, providers.ChatRequest{
			Messages:  messages,
			Model:     s.getModelForRequest(req),
			System:    systemPrompt,
			MaxTokens: 4096,
			Tools:     tools,
		})
		if err != nil {
			log.Error().Err(err).Int("iteration", iteration).Msg("AI provider call failed")
			callback(StreamEvent{Type: "error", Data: err.Error()})
			return nil, fmt.Errorf("Pulse Assistant request failed: %w", err)
		}

		if costStore != nil {
			providerName, _ := config.ParseModelString(modelString)
			if providerName == "" {
				providerName = provider.Name()
			}
			costStore.Record(cost.UsageEvent{
				Timestamp:     time.Now(),
				Provider:      providerName,
				RequestModel:  modelString,
				ResponseModel: resp.Model,
				UseCase:       req.UseCase,
				InputTokens:   resp.InputTokens,
				OutputTokens:  resp.OutputTokens,
				TargetType:    req.TargetType,
				TargetID:      req.TargetID,
				FindingID:     req.FindingID,
			})
		}

		log.Debug().Int("iteration", iteration).Msg("AI provider returned successfully")

		totalInputTokens += resp.InputTokens
		totalOutputTokens += resp.OutputTokens
		model = resp.Model
		finalContent = resp.Content

		// Stream thinking/reasoning content if present (DeepSeek reasoner)
		if resp.ReasoningContent != "" {
			callback(StreamEvent{Type: "thinking", Data: resp.ReasoningContent})
		}

		// Stream intermediate content so users see the AI's explanations between tool calls
		// This gives users visibility into the AI's reasoning as it works, not just at the end
		if resp.Content != "" {
			callback(StreamEvent{Type: "content", Data: resp.Content})
		}

		log.Debug().
			Int("tool_calls", len(resp.ToolCalls)).
			Str("stop_reason", resp.StopReason).
			Int("iteration", iteration).
			Int("total_input_tokens", totalInputTokens).
			Int("total_output_tokens", totalOutputTokens).
			Int("content_length", len(resp.Content)).
			Bool("has_content", resp.Content != "").
			Msg("AI streaming iteration complete")

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 || resp.StopReason != "tool_use" {
			log.Info().
				Int("tool_calls", len(resp.ToolCalls)).
				Str("stop_reason", resp.StopReason).
				Int("iteration", iteration).
				Msg("AI streaming loop ending - no more tool calls or stop_reason != tool_use")
			break
		}

		// Add assistant's response with tool calls to messages
		messages = append(messages, providers.Message{
			Role:             "assistant",
			Content:          resp.Content,
			ReasoningContent: resp.ReasoningContent, // DeepSeek thinking mode
			ToolCalls:        resp.ToolCalls,
		})

		// Execute each tool call and add results
		// Track if any command needs approval - if so, we'll stop the loop after processing
		anyNeedsApproval := false
		for _, tc := range resp.ToolCalls {
			toolInput := s.getToolInputDisplay(tc)

			// Check if this command needs approval
			needsApproval := false
			if tc.Name == "run_command" {
				cmd, _ := tc.Input["command"].(string)
				runOnHost, _ := tc.Input["run_on_host"].(bool)
				targetHost, _ := tc.Input["target_host"].(string)

				// If AI didn't specify target_host, try to get it from request context
				// This is crucial for proper routing when the command is approved
				if targetHost == "" {
					if node, ok := req.Context["node"].(string); ok && node != "" {
						targetHost = node
					} else if node, ok := req.Context["hostname"].(string); ok && node != "" {
						targetHost = node
					} else if node, ok := req.Context["host_name"].(string); ok && node != "" {
						targetHost = node
					}
				}

				isAuto := s.IsAutonomous()
				policyDecision := s.policy.Evaluate(cmd)
				log.Debug().
					Bool("autonomous", isAuto).
					Str("policy_decision", string(policyDecision)).
					Str("command", cmd).
					Str("target_host", targetHost).
					Msg("Checking command policy/approval")

				// Always block commands blocked by policy (even in autonomous mode).
				if policyDecision == agentexec.PolicyBlock {
					result := formatPolicyBlockedToolResult(cmd, "This command is blocked by security policy")
					execution := ToolExecution{
						Name:    tc.Name,
						Input:   toolInput,
						Output:  result,
						Success: false,
					}
					toolExecutions = append(toolExecutions, execution)

					callback(StreamEvent{
						Type: "tool_start",
						Data: ToolStartData{Name: tc.Name, Input: toolInput},
					})
					callback(StreamEvent{
						Type: "tool_end",
						Data: ToolEndData{Name: tc.Name, Input: toolInput, Output: result, Success: false},
					})

					messages = append(messages, providers.Message{
						Role: "user",
						ToolResult: &providers.ToolResult{
							ToolUseID: tc.ID,
							Content:   result,
							IsError:   true,
						},
					})
					continue
				}

				// If policy requires approval and we're not in autonomous mode, request approval.
				if !isAuto && policyDecision == agentexec.PolicyRequireApproval {
					needsApproval = true
					anyNeedsApproval = true
					callback(StreamEvent{
						Type: "approval_needed",
						Data: ApprovalNeededData{
							Command:    cmd,
							ToolID:     tc.ID,
							ToolName:   tc.Name,
							RunOnHost:  runOnHost,
							TargetHost: targetHost,
						},
					})
				}
			}

			var result string
			var execution ToolExecution

			if needsApproval {
				// Don't execute - command needs user approval
				// We'll break out of the loop after processing all tool calls
				// Note: We don't add to toolExecutions here because the approval_needed event
				// already tells the frontend to show the approval UI
				cmd, _ := tc.Input["command"].(string)
				result = formatApprovalNeededToolResult(cmd, tc.ID, "Command requires user approval")
				execution = ToolExecution{
					Name:    tc.Name,
					Input:   toolInput,
					Output:  result,
					Success: true, // Not an error; awaiting approval
				}
			} else {
				// Stream tool start event
				callback(StreamEvent{
					Type: "tool_start",
					Data: ToolStartData{Name: tc.Name, Input: toolInput},
				})

				result, execution = s.executeTool(ctx, req, tc)
				toolExecutions = append(toolExecutions, execution)

				// Stream tool end event
				callback(StreamEvent{
					Type: "tool_end",
					Data: ToolEndData{Name: tc.Name, Input: toolInput, Output: result, Success: execution.Success},
				})

				// Log to remediation log for operational memory
				// Only log run_command since that's the main remediation action
				if tc.Name == "run_command" {
					s.logRemediation(req, toolInput, result, execution.Success)
				}
			}

			// Truncate large results to prevent context bloat
			// Keep first and last parts for context
			resultForContext := result
			const maxResultSize = 8000 // ~8KB per tool result
			if len(result) > maxResultSize {
				halfSize := maxResultSize / 2
				resultForContext = result[:halfSize] + "\n\n[... output truncated (" +
					fmt.Sprintf("%d", len(result)-maxResultSize) + " bytes omitted) ...]\n\n" +
					result[len(result)-halfSize:]
				log.Debug().
					Int("original_size", len(result)).
					Int("truncated_size", len(resultForContext)).
					Msg("Truncated large tool result")
			}

			// Add tool result to messages
			messages = append(messages, providers.Message{
				Role: "user",
				ToolResult: &providers.ToolResult{
					ToolUseID: tc.ID,
					Content:   resultForContext,
					IsError:   !execution.Success,
				},
			})
		}

		// If any command needed approval, stop the agentic loop here.
		// Don't call the AI again with "COMMAND_BLOCKED" results - this causes duplicate
		// approval requests and confusing "click the button" messages.
		// The frontend will show approval buttons, and user action will continue the conversation.
		if anyNeedsApproval {
			log.Info().
				Int("pending_approvals", len(resp.ToolCalls)).
				Int("iteration", iteration).
				Msg("Stopping AI loop - commands need user approval")
			// Use the AI's current response as final content (if any)
			// This preserves any explanation the AI provided before requesting the command
			break
		}
	}

	// Don't stream finalContent here - it was already streamed in the iteration above
	// Sending it again causes duplicate responses (issue #947)
	callback(StreamEvent{Type: "done"})

	return &ExecuteResponse{
		Content:      finalContent,
		Model:        model,
		InputTokens:  totalInputTokens,
		OutputTokens: totalOutputTokens,
		ToolCalls:    toolExecutions,
	}, nil
}

// getToolInputDisplay returns a human-readable display of tool input
func (s *Service) getToolInputDisplay(tc providers.ToolCall) string {
	switch tc.Name {
	case "run_command":
		cmd, _ := tc.Input["command"].(string)
		if runOnHost, ok := tc.Input["run_on_host"].(bool); ok && runOnHost {
			return fmt.Sprintf("[host] %s", cmd)
		}
		return cmd
	case "fetch_url":
		url, _ := tc.Input["url"].(string)
		return url
	case "set_resource_url":
		resourceType, _ := tc.Input["resource_type"].(string)
		url, _ := tc.Input["url"].(string)
		return fmt.Sprintf("Set %s URL: %s", resourceType, url)
	default:
		return fmt.Sprintf("%v", tc.Input)
	}
}

// logRemediation logs a tool execution to the remediation log for operational memory
// This enables learning from past fix attempts
// ONLY logs commands that perform actual ACTIONS (restarts, resizes, cleanups, fixes)
// Skips diagnostic commands (df, grep, cat, tail, ps) to avoid noise
func (s *Service) logRemediation(req ExecuteRequest, command, output string, success bool) {
	// First, check if this is an actionable command worth logging
	// Diagnostic/read-only commands don't provide value in the achievement log
	if !isActionableCommand(command) {
		log.Debug().
			Str("command", command).
			Msg("Skipping diagnostic command - not logging to remediation history")
		return
	}

	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol == nil {
		return
	}

	remLog := patrol.GetRemediationLog()
	if remLog == nil {
		return
	}

	// Determine outcome
	outcome := OutcomeUnknown
	if success {
		outcome = OutcomeResolved
	} else {
		outcome = OutcomeFailed
	}

	// Extract problem from the original prompt (first 200 chars as summary)
	problem := req.Prompt
	if len(problem) > 200 {
		problem = problem[:200] + "..."
	}

	// Generate a meaningful summary from the command that describes what was achieved
	// This is what gets displayed in the Pulse AI Impact section
	summary := generateRemediationSummary(command, req.TargetType, req.Context)

	// Get resource name from context if available
	resourceName := ""
	if req.Context != nil {
		if name, ok := req.Context["name"].(string); ok {
			resourceName = name
		}
	}

	// Truncate output for storage
	truncatedOutput := output
	const maxOutputLen = 1000
	if len(truncatedOutput) > maxOutputLen {
		truncatedOutput = truncatedOutput[:maxOutputLen] + "..."
	}

	remLog.Log(RemediationRecord{
		ResourceID:   req.TargetID,
		ResourceType: req.TargetType,
		ResourceName: resourceName,
		FindingID:    req.FindingID,
		Problem:      problem,
		Summary:      summary,
		Action:       command,
		Output:       truncatedOutput,
		Outcome:      outcome,
		Automatic:    req.UseCase == "patrol", // Patrol runs are automatic
	})

	log.Info().
		Str("resource_id", req.TargetID).
		Str("resource_name", resourceName).
		Str("command", command).
		Str("summary", summary).
		Bool("success", success).
		Msg("Logged ACTION to Pulse AI Impact")
}

// isActionableCommand returns true if the command performs an actual action
// that's worth logging as an achievement, false for read-only diagnostics
func isActionableCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)

	// Strip [host] or [hostname] prefix if present
	if strings.HasPrefix(cmd, "[") {
		if idx := strings.Index(cmd, "]"); idx != -1 {
			cmd = strings.TrimSpace(cmd[idx+1:])
		}
	}

	// Commands that PERFORM ACTIONS (should be logged)
	actionPatterns := []string{
		"docker restart", "docker start", "docker stop", "docker rm",
		"docker compose up", "docker compose down", "docker compose restart",
		"systemctl restart", "systemctl start", "systemctl stop", "systemctl enable", "systemctl disable",
		"service restart", "service start", "service stop",
		"pct resize", "pct start", "pct stop", "pct shutdown", "pct reboot",
		"qm resize", "qm start", "qm stop", "qm shutdown", "qm reboot",
		"rm -", "rm /", // File deletion/cleanup
		"chmod", "chown", // Permission fixes
		"mkdir",      // Creating directories
		"mv ", "cp ", // File operations
		"echo >", "tee ", // Writing to files
		"apt install", "apt upgrade", "apt remove",
		"yum install", "dnf install",
		"pip install", "npm install",
		"kill ", "pkill ", "killall ",
		"reboot", "shutdown",
	}

	for _, pattern := range actionPatterns {
		if strings.Contains(cmd, pattern) {
			return true
		}
	}

	// Everything else is diagnostic (df, grep, cat, tail, ps, ls, etc.)
	return false
}

// generateRemediationSummary creates a human-readable summary of what a command achieved
// This is used to display meaningful descriptions in the Pulse AI Impact section
func generateRemediationSummary(command string, _ string, context map[string]interface{}) string {
	cmd := strings.TrimSpace(command)

	// Extract target name from context
	targetName := ""
	if context != nil {
		if name, ok := context["name"].(string); ok && name != "" {
			targetName = name
		} else if name, ok := context["containerName"].(string); ok && name != "" {
			targetName = name
		} else if name, ok := context["guestName"].(string); ok && name != "" {
			targetName = name
		}
	}

	// Extract meaningful path or service from command
	extractPath := func() string {
		// Look for common path patterns
		pathPatterns := []string{
			`/[\w/._-]+`, // Unix paths
		}
		for _, pattern := range pathPatterns {
			re := regexp.MustCompile(pattern)
			if match := re.FindString(cmd); match != "" {
				// Extract the meaningful part (last 2 segments)
				parts := strings.Split(strings.Trim(match, "/"), "/")
				if len(parts) > 2 {
					parts = parts[len(parts)-2:]
				}
				return "/" + strings.Join(parts, "/")
			}
		}
		return ""
	}

	// Extract container/service name from docker commands
	extractDockerTarget := func() string {
		// docker ps --filter name=XXX
		if match := regexp.MustCompile(`name[=:]\s*(\w+)`).FindStringSubmatch(cmd); len(match) > 1 {
			return match[1]
		}
		// docker restart XXX, docker start XXX, etc.
		if match := regexp.MustCompile(`docker\s+(?:restart|start|stop|logs)\s+([\w.-]+)`).FindStringSubmatch(cmd); len(match) > 1 {
			return match[1]
		}
		return ""
	}

	path := extractPath()
	dockerTarget := extractDockerTarget()

	// Generate summary based on command type
	switch {
	case strings.Contains(cmd, "docker restart") || strings.Contains(cmd, "docker start"):
		if dockerTarget != "" {
			return fmt.Sprintf("Restarted %s container", dockerTarget)
		}
		return "Restarted container"

	case strings.Contains(cmd, "docker stop"):
		if dockerTarget != "" {
			return fmt.Sprintf("Stopped %s container", dockerTarget)
		}
		return "Stopped container"

	case strings.Contains(cmd, "docker ps"):
		if dockerTarget != "" {
			return fmt.Sprintf("Verified %s container is running", dockerTarget)
		}
		return "Checked container status"

	case strings.Contains(cmd, "docker logs"):
		if dockerTarget != "" {
			return fmt.Sprintf("Retrieved %s logs", dockerTarget)
		}
		return "Retrieved container logs"

	case strings.Contains(cmd, "systemctl restart"):
		if match := regexp.MustCompile(`systemctl\s+restart\s+(\S+)`).FindStringSubmatch(cmd); len(match) > 1 {
			return fmt.Sprintf("Restarted %s service", match[1])
		}
		return "Restarted system service"

	case strings.Contains(cmd, "systemctl status"):
		if match := regexp.MustCompile(`systemctl\s+status\s+(\S+)`).FindStringSubmatch(cmd); len(match) > 1 {
			return fmt.Sprintf("Checked %s service status", match[1])
		}
		return "Checked service status"

	case strings.Contains(cmd, "df ") || strings.Contains(cmd, "du "):
		if path != "" {
			// Check for known services in path
			pathLower := strings.ToLower(path)
			if strings.Contains(pathLower, "frigate") {
				return "Analyzed Frigate storage usage"
			}
			if strings.Contains(pathLower, "plex") {
				return "Analyzed Plex storage usage"
			}
			if strings.Contains(pathLower, "recordings") {
				return "Analyzed recordings storage"
			}
			return fmt.Sprintf("Analyzed %s storage", path)
		}
		return "Analyzed disk usage"

	case strings.Contains(cmd, "grep") && (strings.Contains(cmd, "config") || strings.Contains(cmd, "conf") || strings.Contains(cmd, ".yml") || strings.Contains(cmd, ".yaml")):
		if path != "" && strings.Contains(strings.ToLower(path), "frigate") {
			return "Inspected Frigate configuration"
		}
		if path != "" {
			return fmt.Sprintf("Inspected %s configuration", path)
		}
		return "Inspected configuration"

	case strings.Contains(cmd, "tail") || strings.Contains(cmd, "journalctl"):
		if targetName != "" {
			return fmt.Sprintf("Reviewed %s logs", targetName)
		}
		return "Reviewed system logs"

	case strings.Contains(cmd, "pct resize"):
		if match := regexp.MustCompile(`pct\s+resize\s+(\d+)`).FindStringSubmatch(cmd); len(match) > 1 {
			return fmt.Sprintf("Resized container %s disk", match[1])
		}
		return "Resized container disk"

	case strings.Contains(cmd, "qm resize"):
		if match := regexp.MustCompile(`qm\s+resize\s+(\d+)`).FindStringSubmatch(cmd); len(match) > 1 {
			return fmt.Sprintf("Resized VM %s disk", match[1])
		}
		return "Resized VM disk"

	case strings.Contains(cmd, "ping") || strings.Contains(cmd, "curl"):
		return "Tested network connectivity"

	case strings.Contains(cmd, "free") || strings.Contains(cmd, "meminfo"):
		return "Checked memory usage"

	case strings.Contains(cmd, "ps aux") || strings.Contains(cmd, "top"):
		return "Analyzed running processes"

	case strings.Contains(cmd, "rm "):
		return "Cleaned up files"

	case strings.Contains(cmd, "chmod") || strings.Contains(cmd, "chown"):
		return "Fixed file permissions"

	default:
		// Generic fallback - try to use target name if available
		if targetName != "" {
			return fmt.Sprintf("Ran diagnostics on %s", targetName)
		}
		return "Ran system diagnostics"
	}
}

// hasAgentForTarget checks if we have an agent connection for the given target.
// This uses the same routing logic as command execution to determine if the target
// can be reached, including cluster peer routing for Proxmox clusters.
func (s *Service) hasAgentForTarget(req ExecuteRequest) bool {
	// Check for nil interface or nil underlying value
	// Note: A typed nil pointer assigned to an interface is NOT nil according to Go's == operator
	// We need to use reflection to check if the underlying value is nil
	if s.agentServer == nil {
		return false
	}
	// Check if the interface contains a typed nil pointer
	v := reflect.ValueOf(s.agentServer)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return false
	}

	agents := s.agentServer.GetConnectedAgents()
	if len(agents) == 0 {
		return false
	}

	// For host targets with no specific context, any agent will do
	if req.TargetType == "host" && len(req.Context) == 0 {
		return true
	}

	// Try to determine the target node from the request context
	// This mirrors the logic in routeToAgent
	targetNode := ""

	// Check context fields for the target node
	hostFields := []string{"node", "host", "guest_node", "hostname", "host_name", "target_host"}
	for _, field := range hostFields {
		if value, ok := req.Context[field].(string); ok && value != "" {
			targetNode = strings.ToLower(value)
			break
		}
	}

	// If no target node found in context, try the ResourceProvider
	if targetNode == "" {
		s.mu.RLock()
		rp := s.resourceProvider
		s.mu.RUnlock()

		if rp != nil {
			resourceName := ""
			if name, ok := req.Context["containerName"].(string); ok && name != "" {
				resourceName = name
			} else if name, ok := req.Context["name"].(string); ok && name != "" {
				resourceName = name
			} else if name, ok := req.Context["guestName"].(string); ok && name != "" {
				resourceName = name
			}

			if resourceName != "" {
				if host := rp.FindContainerHost(resourceName); host != "" {
					targetNode = strings.ToLower(host)
				}
			}
		}
	}

	// If we still don't have a target node, check for single agent scenario
	if targetNode == "" {
		// For unknown targets, we need at least one agent
		// The actual routing will determine which one to use
		return len(agents) >= 1
	}

	// Check if we have a direct agent match or a cluster peer
	for _, agent := range agents {
		if strings.EqualFold(agent.Hostname, targetNode) {
			return true
		}
	}

	// Try cluster peer routing (for Proxmox clusters)
	if peerAgentID := s.findClusterPeerAgent(targetNode, agents); peerAgentID != "" {
		return true
	}

	return false
}

// getTools returns the available tools for AI
func (s *Service) getTools() []providers.Tool {
	tools := []providers.Tool{
		{
			Name:        "run_command",
			Description: "Execute a shell command. By default runs on the current target (container/VM), but set run_on_host=true for Proxmox host commands. IMPORTANT: For targets on different nodes, specify target_host to route to the correct PVE node.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The shell command to execute (e.g., 'ps aux --sort=-%mem | head -20')",
					},
					"run_on_host": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, run on the Proxmox/Docker host instead of inside the container/VM. Use for pct/qm commands like 'pct resize 101 rootfs +10G'. When true, you should also set target_host.",
					},
					"target_host": map[string]interface{}{
						"type":        "string",
						"description": "Optional hostname of the specific host/node to run the command on. Use this to explicitly route pct/qm/docker commands to the correct Proxmox node or Docker host. Check the 'node' or 'PVE Node' field in the target's context.",
					},
				},
				"required": []string{"command"},
			},
		},
		{
			Name:        "fetch_url",
			Description: "Fetch content from a URL. Use this to check if web services are responding, read API endpoints, or fetch documentation. Works with local network URLs and public sites.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL to fetch (e.g., 'http://192.168.1.50:8080/api/health' or 'https://example.com/docs')",
					},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "set_resource_url",
			Description: "Set the web URL for a resource in Pulse after discovering a web service. Use this when you've found a web server running on a guest/container/host and want to save it for quick access. The URL will appear as a clickable link in the Pulse dashboard.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"resource_type": map[string]interface{}{
						"type":        "string",
						"description": "Type of resource: 'guest' for VMs/LXC containers, 'docker' for Docker containers/services, or 'host' for standalone hosts",
						"enum":        []string{"guest", "docker", "host"},
					},
					"resource_id": map[string]interface{}{
						"type":        "string",
						"description": "The resource ID from the context. For Proxmox guests, use format 'instance-VMID' (e.g., 'delly-150' where 'delly' is the PVE instance name and '150' is the VMID). For Docker, use format 'hostid:container:containerid'. Use the ID shown in the current context.",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The discovered URL (e.g., 'http://192.168.1.50:8096' for Jellyfin). Use an empty string to remove the URL.",
					},
				},
				"required": []string{"resource_type", "resource_id"},
			},
		},
		{
			Name:        "resolve_finding",
			Description: "Mark an AI patrol finding as resolved after successfully fixing the issue. Use the finding ID shown in your Patrol Finding Context section. Call this after verifying the fix worked - do NOT ask the user for the finding ID.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"finding_id": map[string]interface{}{
						"type":        "string",
						"description": "The finding ID from your context (shown in ## Patrol Finding Context section).",
					},
					"resolution_note": map[string]interface{}{
						"type":        "string",
						"description": "Brief description of how the issue was resolved (e.g., 'Restarted nginx service', 'Cleaned up disk space').",
					},
				},
				"required": []string{"finding_id", "resolution_note"},
			},
		},
		{
			Name:        "dismiss_finding",
			Description: "Dismiss an AI patrol finding when it's not actually an issue or is expected behavior. Use this instead of resolve_finding when the finding is a false positive or the configuration is intentional. This creates a suppression rule to prevent similar findings from being raised again.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"finding_id": map[string]interface{}{
						"type":        "string",
						"description": "The finding ID from your context (shown in ## Patrol Finding Context section).",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Why the finding is being dismissed.",
						"enum":        []string{"not_an_issue", "expected_behavior", "will_fix_later"},
					},
					"note": map[string]interface{}{
						"type":        "string",
						"description": "Explanation of why this is not an issue or is expected behavior (e.g., 'PBS storage restricted to specific nodes is intentional').",
					},
				},
				"required": []string{"finding_id", "reason", "note"},
			},
		},
	}

	// Add web search tool for Anthropic provider
	if s.provider != nil && s.provider.Name() == "anthropic" {
		tools = append(tools, providers.Tool{
			Type:    "web_search_20250305",
			Name:    "web_search",
			MaxUses: 3, // Limit searches per request to control costs
		})
	}

	return tools
}

// executeTool executes a tool call and returns the result
func (s *Service) executeTool(ctx context.Context, req ExecuteRequest, tc providers.ToolCall) (string, ToolExecution) {
	execution := ToolExecution{
		Name:    tc.Name,
		Success: false,
	}

	switch tc.Name {
	case "run_command":
		command, _ := tc.Input["command"].(string)
		runOnHost, _ := tc.Input["run_on_host"].(bool)
		targetHost, _ := tc.Input["target_host"].(string)
		execution.Input = command
		if runOnHost && targetHost != "" {
			execution.Input = fmt.Sprintf("[%s] %s", targetHost, command)
		} else if runOnHost {
			execution.Input = fmt.Sprintf("[host] %s", command)
		}

		if command == "" {
			execution.Output = "Error: command is required"
			return execution.Output, execution
		}

		// Enforce security policy.
		// - Blocked commands are ALWAYS blocked (even in autonomous mode).
		// - Approval-required commands only require approval when not in autonomous mode.
		decision := s.policy.Evaluate(command)
		if decision == agentexec.PolicyBlock {
			execution.Output = formatPolicyBlockedToolResult(command, "This command is blocked by security policy")
			return execution.Output, execution
		}
		if decision == agentexec.PolicyRequireApproval && !s.IsAutonomous() {
			execution.Output = formatApprovalNeededToolResult(command, tc.ID, "Security policy requires approval")
			execution.Success = true // Not an error, just needs approval
			return execution.Output, execution
		}

		// Build execution request with proper targeting
		execReq := req

		// If target_host is explicitly specified by AI, use it for routing
		if targetHost != "" {
			// Ensure Context map exists
			if execReq.Context == nil {
				execReq.Context = make(map[string]interface{})
			} else {
				// Make a copy to avoid modifying the original
				newContext := make(map[string]interface{})
				for k, v := range req.Context {
					newContext[k] = v
				}
				execReq.Context = newContext
			}
			// Set the node explicitly - this takes priority in routing
			execReq.Context["node"] = targetHost
			log.Debug().
				Str("target_host", targetHost).
				Str("command", command).
				Msg("AI explicitly specified target_host for command routing")
		}

		// If run_on_host is true, override the target type to run on host
		if runOnHost {
			log.Debug().
				Str("command", command).
				Str("target_host", targetHost).
				Str("original_target_type", req.TargetType).
				Str("original_target_id", req.TargetID).
				Msg("run_on_host=true - overriding target type to 'host'")
			execReq.TargetType = "host"
			execReq.TargetID = ""
		} else {
			log.Debug().
				Str("command", command).
				Str("target_type", req.TargetType).
				Str("target_id", req.TargetID).
				Bool("run_on_host", runOnHost).
				Msg("Executing command with current target type")
		}

		// Execute via agent
		result, err := s.executeOnAgent(ctx, execReq, command)
		recordIncident := func(success bool, output string) {
			alertID := extractAlertID(req.Context)
			if alertID == "" {
				return
			}
			s.mu.RLock()
			incidentStore := s.incidentStore
			s.mu.RUnlock()
			if incidentStore == nil {
				return
			}
			details := map[string]interface{}{
				"resource_id":   req.TargetID,
				"resource_type": req.TargetType,
				"run_on_host":   runOnHost,
			}
			if targetHost != "" {
				details["target_host"] = targetHost
			}
			incidentStore.RecordCommand(alertID, command, success, output, details)
		}
		if err != nil {
			recordIncident(false, result)
			execution.Output = fmt.Sprintf("Error executing command: %s", err)
			return execution.Output, execution
		}

		recordIncident(true, result)
		execution.Output = result
		execution.Success = true
		return result, execution

	case "fetch_url":
		urlStr, _ := tc.Input["url"].(string)
		execution.Input = urlStr

		if urlStr == "" {
			execution.Output = "Error: url is required"
			return execution.Output, execution
		}

		// Fetch the URL
		result, err := s.fetchURL(ctx, urlStr)
		if err != nil {
			execution.Output = fmt.Sprintf("Error fetching URL: %s", err)
			return execution.Output, execution
		}

		execution.Output = result
		execution.Success = true
		return result, execution

	case "set_resource_url":
		resourceType, _ := tc.Input["resource_type"].(string)
		resourceID, _ := tc.Input["resource_id"].(string)
		url, _ := tc.Input["url"].(string)
		execution.Input = fmt.Sprintf("%s %s -> %s", resourceType, resourceID, url)

		if resourceType == "" {
			execution.Output = "Error: resource_type is required (use 'guest', 'docker', or 'host')"
			return execution.Output, execution
		}
		if resourceID == "" {
			// Try to get the resource ID from the request context
			if req.TargetID != "" {
				resourceID = req.TargetID
			} else {
				execution.Output = "Error: resource_id is required"
				return execution.Output, execution
			}
		}

		// Allow empty URL to clear the setting
		// if url == "" {
		// 	execution.Output = "Error: url is required"
		// 	return execution.Output, execution
		// }

		// Update the metadata
		if err := s.SetResourceURL(resourceType, resourceID, url); err != nil {
			execution.Output = fmt.Sprintf("Error setting URL: %s", err)
			return execution.Output, execution
		}

		execution.Output = fmt.Sprintf("Successfully set URL for %s '%s' to: %s\nThe URL is now visible in the Pulse dashboard as a clickable link.", resourceType, resourceID, url)
		execution.Success = true
		return execution.Output, execution

	case "resolve_finding":
		findingID, _ := tc.Input["finding_id"].(string)
		resolutionNote, _ := tc.Input["resolution_note"].(string)
		execution.Input = fmt.Sprintf("finding: %s, note: %s", findingID, resolutionNote)

		// If no finding ID provided by AI, check the request context
		if findingID == "" {
			findingID = req.FindingID
		}

		if findingID == "" {
			execution.Output = "Error: finding_id is required. The finding ID should be provided in the request context when helping fix a patrol finding."
			return execution.Output, execution
		}

		if resolutionNote == "" {
			execution.Output = "Error: resolution_note is required. Please describe how the issue was resolved."
			return execution.Output, execution
		}

		// Get the patrol service to resolve the finding
		s.mu.RLock()
		patrolService := s.patrolService
		s.mu.RUnlock()

		if patrolService == nil {
			execution.Output = "Error: Patrol service not available"
			return execution.Output, execution
		}

		// Resolve the finding
		err := patrolService.ResolveFinding(findingID, resolutionNote)
		if err != nil {
			execution.Output = fmt.Sprintf("Error resolving finding: %s", err)
			return execution.Output, execution
		}

		execution.Output = fmt.Sprintf("Finding resolved! The Patrol finding has been marked as fixed.\nID: %s\nResolution: %s", findingID, resolutionNote)
		execution.Success = true
		return execution.Output, execution

	case "dismiss_finding":
		findingID, _ := tc.Input["finding_id"].(string)
		reason, _ := tc.Input["reason"].(string)
		note, _ := tc.Input["note"].(string)
		execution.Input = fmt.Sprintf("finding: %s, reason: %s, note: %s", findingID, reason, note)

		// If no finding ID provided by AI, check the request context
		if findingID == "" {
			findingID = req.FindingID
		}

		if findingID == "" {
			execution.Output = "Error: finding_id is required. The finding ID should be provided in the request context when helping fix a patrol finding."
			return execution.Output, execution
		}

		// Validate reason
		validReasons := map[string]bool{"not_an_issue": true, "expected_behavior": true, "will_fix_later": true}
		if !validReasons[reason] {
			execution.Output = "Error: reason must be one of: not_an_issue, expected_behavior, will_fix_later"
			return execution.Output, execution
		}

		if note == "" {
			execution.Output = "Error: note is required. Please explain why this finding is being dismissed."
			return execution.Output, execution
		}

		// Get the patrol service to dismiss the finding
		s.mu.RLock()
		patrolService := s.patrolService
		s.mu.RUnlock()

		if patrolService == nil {
			execution.Output = "Error: Patrol service not available"
			return execution.Output, execution
		}

		// Dismiss the finding (this will also create a suppression rule for expected_behavior/not_an_issue)
		err := patrolService.DismissFinding(findingID, reason, note)
		if err != nil {
			execution.Output = fmt.Sprintf("Error dismissing finding: %s", err)
			return execution.Output, execution
		}

		// Format a helpful response based on reason
		var resultMsg string
		if reason == "will_fix_later" {
			resultMsg = fmt.Sprintf("Finding dismissed as '%s'. Pulse Patrol will continue to monitor this issue.\nID: %s\nNote: %s", reason, findingID, note)
		} else {
			resultMsg = fmt.Sprintf("Finding dismissed as '%s' and suppression rule created. Similar findings for this resource will not be raised again.\nID: %s\nNote: %s", reason, findingID, note)
		}
		execution.Output = resultMsg
		execution.Success = true
		return execution.Output, execution

	default:
		execution.Output = fmt.Sprintf("Unknown tool: %s", tc.Name)
		return execution.Output, execution
	}
}

// getGuestID returns a unique identifier for the guest based on the request
func (s *Service) getGuestID(req ExecuteRequest) string {
	// Build a consistent guest ID from the target information
	if req.TargetType == "" || req.TargetID == "" {
		return ""
	}

	// For Proxmox targets, include the node info
	// Format: instance-node-type-vmid or instance-targetid
	return fmt.Sprintf("%s-%s", req.TargetType, req.TargetID)
}

// GetGuestKnowledge returns all knowledge for a guest
func (s *Service) GetGuestKnowledge(guestID string) (*knowledge.GuestKnowledge, error) {
	if s.knowledgeStore == nil {
		return nil, fmt.Errorf("knowledge store not available")
	}
	return s.knowledgeStore.GetKnowledge(guestID)
}

// SaveGuestNote saves a note for a guest
func (s *Service) SaveGuestNote(guestID, guestName, guestType, category, title, content string) error {
	if s.knowledgeStore == nil {
		return fmt.Errorf("knowledge store not available")
	}
	return s.knowledgeStore.SaveNote(guestID, guestName, guestType, category, title, content)
}

// DeleteGuestNote deletes a note from a guest
func (s *Service) DeleteGuestNote(guestID, noteID string) error {
	if s.knowledgeStore == nil {
		return fmt.Errorf("knowledge store not available")
	}
	return s.knowledgeStore.DeleteNote(guestID, noteID)
}

// fetchURL fetches content from a URL with size limits and timeout
func (s *Service) fetchURL(ctx context.Context, urlStr string) (string, error) {
	parsedURL, err := parseAndValidateFetchURL(ctx, urlStr)
	if err != nil {
		return "", err
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			if _, err := parseAndValidateFetchURL(ctx, req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", parsedURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Set a reasonable user agent
	req.Header.Set("User-Agent", "Pulse-AI/1.0")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response with size limit (64KB)
	const maxSize = 64 * 1024
	limitedReader := io.LimitReader(resp.Body, maxSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Build result with status info
	result := fmt.Sprintf("HTTP %d %s\n", resp.StatusCode, resp.Status)
	result += fmt.Sprintf("Content-Type: %s\n", resp.Header.Get("Content-Type"))
	result += fmt.Sprintf("Content-Length: %d bytes\n\n", len(body))
	result += string(body)

	if len(body) == maxSize {
		result += "\n\n[Response truncated at 64KB]"
	}

	return result, nil
}

func parseAndValidateFetchURL(ctx context.Context, urlStr string) (*url.URL, error) {
	clean := strings.TrimSpace(urlStr)
	if clean == "" {
		return nil, fmt.Errorf("url is required")
	}

	parsed, err := url.Parse(clean)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if !parsed.IsAbs() {
		return nil, fmt.Errorf("URL must be absolute")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("only http/https URLs are allowed")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("URLs with embedded credentials are not allowed")
	}
	if parsed.Fragment != "" {
		return nil, fmt.Errorf("URL fragments are not allowed")
	}

	host := parsed.Hostname()
	if host == "" {
		return nil, fmt.Errorf("URL must include a host")
	}

	if isBlockedFetchHost(host) {
		return nil, fmt.Errorf("URL host is blocked")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedFetchIP(ip) {
			return nil, fmt.Errorf("URL IP is blocked")
		}
		return parsed, nil
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		// DNS failures are surfaced directly to the caller.
		return nil, fmt.Errorf("failed to resolve host: %w", err)
	}
	for _, addr := range addrs {
		if isBlockedFetchIP(addr.IP) {
			return nil, fmt.Errorf("URL host resolves to a blocked address")
		}
	}

	return parsed, nil
}

func isBlockedFetchHost(host string) bool {
	h := strings.TrimSpace(strings.ToLower(host))
	if h == "localhost" || h == "localhost." {
		return true
	}
	return false
}

func isBlockedFetchIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		if ip.IsLoopback() && os.Getenv("PULSE_AI_ALLOW_LOOPBACK") == "true" {
			return false
		}
		return true
	}
	// Block multicast and other non-unicast targets.
	if !ip.IsGlobalUnicast() && !ip.IsPrivate() {
		return true
	}
	return false
}

// sanitizeError cleans up error messages to remove internal networking details
// that are not helpful to users or AI models (IP addresses, port numbers, etc.)
func sanitizeError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// Replace raw TCP connection details with generic message
	// e.g., "write tcp 192.168.0.123:7655->192.168.0.134:58004: i/o timeout"
	// becomes "connection to agent timed out"
	if strings.Contains(errMsg, "i/o timeout") {
		if strings.Contains(errMsg, "failed to send command") {
			return fmt.Errorf("connection to agent timed out - the agent may be disconnected or unreachable")
		}
		return fmt.Errorf("network timeout - the target may be unreachable")
	}

	// Replace "write tcp ... connection refused" style errors
	if strings.Contains(errMsg, "connection refused") {
		return fmt.Errorf("connection refused - the agent may not be running on the target host")
	}

	// Replace "no such host" errors
	if strings.Contains(errMsg, "no such host") {
		return fmt.Errorf("host not found - verify the hostname is correct and DNS is working")
	}

	// Replace "context deadline exceeded" with friendlier message
	if strings.Contains(errMsg, "context deadline exceeded") {
		return fmt.Errorf("operation timed out - the command may have taken too long")
	}

	return err
}

// executeOnAgent executes a command via the agent WebSocket
func (s *Service) executeOnAgent(ctx context.Context, req ExecuteRequest, command string) (string, error) {
	if s.agentServer == nil {
		return "", fmt.Errorf("agent server not available")
	}

	// Find the appropriate agent using robust routing
	agents := s.agentServer.GetConnectedAgents()

	// Use the new robust routing logic
	routeResult, err := s.routeToAgent(req, command, agents)
	if err != nil {
		// Check if this is a routing error that should ask for clarification
		if routingErr, ok := err.(*RoutingError); ok && routingErr.AskForClarification {
			// Return a message that encourages the AI to ask the user for clarification
			// instead of just failing with an error
			return routingErr.ForAI(), nil
		}
		// Return actionable error message for other errors
		return "", err
	}

	// Log any warnings from routing
	for _, warning := range routeResult.Warnings {
		log.Warn().Str("warning", warning).Msg("Routing warning")
	}

	agentID := routeResult.AgentID

	log.Debug().
		Str("agent_id", agentID).
		Str("agent_hostname", routeResult.AgentHostname).
		Str("target_node", routeResult.TargetNode).
		Str("routing_method", routeResult.RoutingMethod).
		Bool("cluster_peer", routeResult.ClusterPeer).
		Msg("Command routed to agent")

	// Extract numeric VMID from target ID (e.g., "delly-135" -> "135")
	targetID := req.TargetID
	if req.TargetType == "container" || req.TargetType == "vm" {
		// Look for vmid in context first
		if vmid, ok := req.Context["vmid"]; ok {
			switch v := vmid.(type) {
			case float64:
				targetID = fmt.Sprintf("%.0f", v)
			case int:
				targetID = fmt.Sprintf("%d", v)
			case string:
				targetID = v
			}
		} else if req.TargetID != "" {
			// Extract number from end of ID like "delly-135" or "instance-135"
			parts := strings.Split(req.TargetID, "-")
			if len(parts) > 0 {
				lastPart := parts[len(parts)-1]
				// Check if it's numeric
				if _, err := fmt.Sscanf(lastPart, "%d", new(int)); err == nil {
					targetID = lastPart
				}
			}
		}
	}

	requestID := uuid.New().String()

	// Automatically force non-interactive mode for package managers
	// This prevents hanging when apt/dpkg asks for confirmation or configuration
	if strings.Contains(command, "apt") || strings.Contains(command, "dpkg") {
		if !strings.Contains(command, "DEBIAN_FRONTEND=") {
			command = "export DEBIAN_FRONTEND=noninteractive; " + command
		}
	}

	cmd := agentexec.ExecuteCommandPayload{
		RequestID:  requestID,
		Command:    command,
		TargetType: req.TargetType,
		TargetID:   targetID,
		Timeout:    300, // 5 minutes - commands like du, backups, etc. can take a while
	}

	result, err := s.agentServer.ExecuteCommand(ctx, agentID, cmd)
	if err != nil {
		return "", sanitizeError(err)
	}

	if !result.Success {
		if result.Error != "" {
			return "", fmt.Errorf("%s", result.Error)
		}
		if result.Stderr != "" {
			return result.Stderr, nil // Return stderr as output, not error
		}
	}

	output := result.Stdout
	if result.Stderr != "" && result.Stdout != "" {
		output = fmt.Sprintf("%s\n\nSTDERR:\n%s", result.Stdout, result.Stderr)
	} else if result.Stderr != "" {
		output = result.Stderr
	}

	return output, nil
}

// RunCommandRequest represents a request to run a single command
type RunCommandRequest struct {
	Command    string `json:"command"`
	TargetType string `json:"target_type"` // "host", "container", "vm"
	TargetID   string `json:"target_id"`
	RunOnHost  bool   `json:"run_on_host"` // If true, run on host instead of target
	VMID       string `json:"vmid,omitempty"`
	TargetHost string `json:"target_host,omitempty"` // Explicit host for routing
}

// RunCommandResponse represents the result of running a command
type RunCommandResponse struct {
	Output  string `json:"output"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// RunCommand executes a single command via the agent (used for approved commands)
func (s *Service) RunCommand(ctx context.Context, req RunCommandRequest) (*RunCommandResponse, error) {
	if s.agentServer == nil {
		return &RunCommandResponse{Success: false, Error: "Agent server not available"}, nil
	}

	// Build an ExecuteRequest from the RunCommandRequest
	execReq := ExecuteRequest{
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Context:    make(map[string]interface{}),
	}

	// If running on host, override target type
	if req.RunOnHost {
		execReq.TargetType = "host"
		// Keep the original target info for routing
	}

	// Add VMID to context if provided
	if req.VMID != "" {
		execReq.Context["vmid"] = req.VMID
	}

	// If target_host is specified, set it in context for routing
	if req.TargetHost != "" {
		execReq.Context["node"] = req.TargetHost
		log.Debug().
			Str("target_host", req.TargetHost).
			Str("command", req.Command).
			Msg("RunCommand using explicit target_host for routing")
	}

	output, err := s.executeOnAgent(ctx, execReq, req.Command)
	if err != nil {
		return &RunCommandResponse{
			Success: false,
			Error:   err.Error(),
			Output:  output,
		}, nil
	}

	return &RunCommandResponse{
		Success: true,
		Output:  output,
	}, nil
}

// buildSystemPrompt creates the system prompt based on the request context
func (s *Service) buildSystemPrompt(req ExecuteRequest) string {
	prompt := `You are Pulse's diagnostic assistant - a built-in tool for investigating Proxmox, Docker, and Kubernetes homelab issues.

## Response Style
- Be DIRECT and CONCISE. No greetings, no "I'll help you", no "Let me check"
- Report findings, not process. Don't narrate what you're doing
- NO emojis, NO markdown tables, NO excessive formatting
- Use simple lists only when showing multiple items
- Final response should be 2-4 sentences summarizing findings and any actions taken
- When you perform cleanup/fixes, report: what was done + the result (e.g., "Freed 6GB. Disk now at 81%.")

## When Investigating
- Execute commands silently in the background
- Only show the conclusion, not every command run
- If something fails, briefly state what and why

## Command Approval System
Pulse has a command approval policy that protects against accidental damage:
- **Safe commands** (read-only: ls, df, cat, ps, etc.) execute immediately
- **Potentially destructive commands** (rm, service restart, apt install, etc.) require approval unless autonomous mode is enabled
- **Blocked commands** are never executed
- When approval is required, the user will see the command and can approve or reject it

When you need to run a potentially destructive command:
1. Execute it normally - Pulse will handle the approval flow
2. If a command is blocked or needs approval, you'll get feedback
3. Continue with other work while waiting for approval, or explain what the command will do

Do NOT say things like:
- "I need your permission to..."
- "Would you like me to run...?"
- "This is destructive, are you sure?"

Instead, just execute the command. Pulse's approval system will handle user confirmation for risky operations.

## Response Format
BAD: "I'll check that for you. Let me run some commands..."
GOOD: State findings directly.

BAD: "Would you like me to run rm -rf...?"
GOOD: Execute the command. User will approve/reject via Pulse.

BAD: Tables, headers, bullet-heavy summaries
GOOD: Plain prose, 2-4 sentences.

## ACTION BIAS - AVOID INVESTIGATION LOOPS
When the user asks you to DO something (install, fix, update, configure), ACT IMMEDIATELY:
- Don't extensively investigate before acting. Run 1-2 diagnostic commands max, then DO the thing.
- Don't explain what you're about to do - just do it.
- If the command needs approval, Pulse will queue it. You don't need to ask separately.
- If the first approach fails, try the next most obvious approach. Don't stop to report.
- Complete the task END TO END. If asked to "install and run X", you're not done until X is running.

INVESTIGATION ANTI-PATTERNS TO AVOID:
- Running 10+ diagnostic commands before taking action
- Explaining each step before doing it
- Stopping to report partial progress
- Asking "would you like me to proceed?" (Pulse handles approval automatically)
- Checking version, checking config, checking service, checking ports, checking this, checking that... JUST ACT.

GOOD PATTERN for "install X and make sure it's running":
1. Download/install X (1 command)
2. Start X (1 command)  
3. Verify X is running (1 command)
4. Report: "Installed X vN.N. Service is running on port NNNN."

BAD PATTERN:
1. Check current version... 2. Check if installed... 3. Check service status... 4. Check config file... 
5. Check another config... 6. Try to enable something... 7. Check if it worked... 8. Read a script...
9. Check yet another file... [user: "do it"] 10. Still investigating... [user: "DO IT"]


## Using Context Data
Pulse provides real metrics in "Current Metrics and State". Use this data directly - don't ask users to check things you already know.

## Command Execution
- run_on_host=true: Run on PVE/Docker host (pct, qm, vzdump, docker commands)
- run_on_host=false: Run inside the container/VM
- target_host: ALWAYS set this when using run_on_host=true! Use the node/hostname from target context
- Execute commands to investigate, don't just explain what commands to run

## CRITICAL: Command Routing with target_host
When running commands that require a specific host (pct, qm, docker, vzdump), you MUST specify target_host to route correctly.

Example for LXC 106 on node 'minipc':
- To run 'df -h' inside the container: run_command(command="df -h", run_on_host=false)
- To run 'pct exec 106 -- df -h' on the host: run_command(command="pct exec 106 -- df -h", run_on_host=true, target_host="minipc")

Always check the target's context for the 'node' or 'PVE Node' field and pass it as target_host.
If you don't specify target_host when run_on_host=true, the command may route to the wrong host!

Rules:
1. Look at the target context for 'node', 'guest_node', or 'PVE Node' field
2. When running pct/qm commands: set run_on_host=true AND target_host=<node>
3. When running commands inside the guest: just set run_on_host=false (no target_host needed)
4. Error "Configuration file does not exist" means wrong host - check target_host

## Infrastructure Architecture - LXC Management
Pulse manages LXC containers agentlessly from the PVE host.
- DO NOT check for a Pulse agent process or service inside an LXC. It does not exist.
- Use run_command with run_on_host=false to execute commands inside the LXC. Pulse handles the routing.
- For pct commands, always use run_on_host=true and set target_host to the container's node.

## URL Discovery Feature
When asked to find the web URL for a guest/container/host, or when you discover a web service:

1. **Inspect for web servers**: Check for listening ports (ss -tlnp), running services (nginx, apache, node, etc.)
2. **Get the IP address**: Use 'hostname -I' or 'ip addr' to find the IP
3. **Test the URL**: Use fetch_url to verify the service is responding
4. **Save the URL**: Use set_resource_url tool to save it to Pulse

Common discovery commands:
- Check listening ports: ss -tlnp | grep LISTEN
- Check nginx: systemctl status nginx && grep -r 'listen' /etc/nginx/
- Check running processes: ps aux | grep -E 'node|python|java|nginx|apache|httpd'
- Get IP: hostname -I | awk '{print $1}'

**CRITICAL: resource_id format for set_resource_url**
The resource_id MUST be in the canonical format: {instance}:{node}:{vmid} (uses COLONS, not dashes)
- Example for VMID 201 on node "minipc" in instance "delly": resource_id = "delly:minipc:201"
- Example for VMID 101 on node "delly" in instance "delly": resource_id = "delly:delly:101"
- The instance name is typically the cluster name for PVE clusters
- Always check which NODE the guest is on (visible in the target_host or context)

When working across multiple nodes, use the correct instance:node:vmid format for each guest.

## Installing/Updating Pulse Itself
If asked to install or update Pulse itself, use the official install script. DO NOT investigate configs/services first.
Quick install/update command (x86_64 Linux):
` + "`" + `curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash` + "`" + `

To install a specific version: 
` + "`" + `curl -sSL https://raw.githubusercontent.com/rcourtman/Pulse/main/install.sh | bash -s -- --version vX.Y.Z` + "`" + `

After install, enable and start the service:
` + "`" + `systemctl enable pulse && systemctl start pulse` + "`" + `

The latest version can be found at: https://api.github.com/repos/rcourtman/Pulse/releases/latest
This is a 3-command job. Don't over-investigate.`

	// Add custom context from AI settings (user's infrastructure description)
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	if cfg != nil && cfg.CustomContext != "" {
		prompt += "\n\n## User's Infrastructure Description\n"
		prompt += "The user has provided this context about their infrastructure:\n\n"
		prompt += cfg.CustomContext
	}

	// Add connected infrastructure info via unified resource model
	s.mu.RLock()
	hasResourceProvider := s.resourceProvider != nil
	s.mu.RUnlock()

	if hasResourceProvider {
		prompt += s.buildUnifiedResourceContext()
	} else {
		log.Warn().Msg("AI context: resource provider not available, infrastructure context will be limited")
	}

	// Add user annotations from all resources (global context)
	prompt += s.buildUserAnnotationsContext()

	// Add current alert status - this gives AI awareness of active issues
	prompt += s.buildAlertContext()

	// Add incident memory for alert or resource context
	alertID := ""
	resourceID := req.TargetID
	if req.Context != nil {
		if val, ok := req.Context["alertId"].(string); ok && val != "" {
			alertID = val
		} else if val, ok := req.Context["alert_id"].(string); ok && val != "" {
			alertID = val
		}

		if resourceID == "" {
			if val, ok := req.Context["resourceId"].(string); ok && val != "" {
				resourceID = val
			} else if val, ok := req.Context["resource_id"].(string); ok && val != "" {
				resourceID = val
			} else if val, ok := req.Context["guest_id"].(string); ok && val != "" {
				resourceID = val
			}
		}
	}

	if incidentContext := s.buildIncidentContext(resourceID, alertID); incidentContext != "" {
		prompt += incidentContext
	}

	// Add all saved knowledge when no specific target is selected
	// This gives the AI context about everything learned from previous sessions
	if req.TargetType == "" && s.knowledgeStore != nil {
		prompt += s.knowledgeStore.FormatAllForContext()
	}

	// Add target context if provided
	if req.TargetType != "" {
		guestName := ""
		if name, ok := req.Context["guestName"].(string); ok {
			guestName = name
		} else if name, ok := req.Context["name"].(string); ok {
			guestName = name
		}

		if guestName != "" {
			// Include the node in the focus header so AI can't miss it for routing
			nodeName := ""
			if node, ok := req.Context["node"].(string); ok && node != "" {
				nodeName = node
			} else if node, ok := req.Context["guest_node"].(string); ok && node != "" {
				nodeName = node
			}
			if nodeName != "" {
				prompt += fmt.Sprintf("\n\n## Current Focus\nYou are analyzing **%s** (%s on node **%s**)\n**ROUTING: When using run_on_host=true, set target_host=\"%s\"**",
					guestName, req.TargetType, nodeName, nodeName)
			} else {
				prompt += fmt.Sprintf("\n\n## Current Focus\nYou are analyzing **%s** (%s)", guestName, req.TargetType)
			}
		} else if req.TargetID != "" {
			prompt += fmt.Sprintf("\n\n## Current Focus\nYou are analyzing %s '%s'", req.TargetType, req.TargetID)
		}

		// Add past remediation history for this resource
		prompt += s.buildRemediationContext(req.TargetID, req.Prompt)

	}

	// If we're helping fix a patrol finding, tell the AI the finding ID so it can resolve or dismiss it
	if req.FindingID != "" {
		prompt += fmt.Sprintf("\n\n## Patrol Finding Context\n"+
			"You are helping with patrol finding **%s**.\n\n"+
			"**After investigating, use ONE of these tools:**\n"+
			"- `resolve_finding` - Use when you've actually FIXED the underlying issue\n"+
			"- `dismiss_finding` - Use when the finding is a FALSE POSITIVE or EXPECTED BEHAVIOR\n\n"+
			"**Examples:**\n"+
			"- Issue fixed: `resolve_finding(finding_id=\"%s\", resolution_note=\"Restarted service\")`\n"+
			"- False positive: `dismiss_finding(finding_id=\"%s\", reason=\"expected_behavior\", note=\"Storage restricted to specific node is intentional\")`\n",
			req.FindingID, req.FindingID, req.FindingID)
	}

	// Add any provided context in a structured way
	if len(req.Context) > 0 {
		prompt += "\n\n## Current Metrics and State"

		// Group metrics by category for better readability
		categories := map[string][]string{
			"Identity":     {"name", "guestName", "type", "vmid", "node", "guest_node", "status", "uptime"},
			"CPU":          {"cpu_usage", "cpu_cores"},
			"Memory":       {"memory_used", "memory_total", "memory_usage", "memory_balloon", "swap_used", "swap_total"},
			"Disk":         {"disk_used", "disk_total", "disk_usage"},
			"I/O Rates":    {"disk_read_rate", "disk_write_rate", "network_in_rate", "network_out_rate"},
			"Backup":       {"backup_status", "last_backup", "days_since_backup"},
			"System Info":  {"os_name", "os_version", "guest_agent", "ip_addresses", "tags"},
			"User Context": {"user_notes", "user_annotations"},
		}

		categoryOrder := []string{"Identity", "User Context", "Backup", "CPU", "Memory", "Disk", "I/O Rates", "System Info"}

		for _, category := range categoryOrder {
			keys := categories[category]
			hasValues := false
			categoryContent := ""

			for _, k := range keys {
				if v, ok := req.Context[k]; ok && v != nil && v != "" {
					if !hasValues {
						categoryContent = fmt.Sprintf("\n### %s", category)
						hasValues = true
					}
					categoryContent += fmt.Sprintf("\n- %s: %v", formatContextKey(k), v)
				}
			}

			if hasValues {
				prompt += categoryContent
			}
		}

		// Add any remaining context that wasn't categorized
		for k, v := range req.Context {
			found := false
			for _, keys := range categories {
				for _, key := range keys {
					if k == key {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found && v != nil && v != "" {
				prompt += fmt.Sprintf("\n- %s: %v", formatContextKey(k), v)
			}
		}

		// Add enriched historical context (baselines, trends, predictions)
		// This is the Pulse Pro value-add that Claude Code can't replicate
		prompt += s.buildEnrichedResourceContext(req.TargetID, req.TargetType, req.Context)
	}

	return prompt
}

// formatContextKey converts snake_case keys to readable labels
func formatContextKey(key string) string {
	replacements := map[string]string{
		"guestName":         "Guest Name",
		"name":              "Name",
		"type":              "Type",
		"vmid":              "VMID",
		"node":              "PVE Node (host)",
		"guest_node":        "PVE Node (host)",
		"status":            "Status",
		"uptime":            "Uptime",
		"cpu_usage":         "CPU Usage",
		"cpu_cores":         "CPU Cores",
		"memory_used":       "Memory Used",
		"memory_total":      "Memory Total",
		"memory_usage":      "Memory Usage",
		"memory_balloon":    "Memory Balloon",
		"swap_used":         "Swap Used",
		"swap_total":        "Swap Total",
		"disk_used":         "Disk Used",
		"disk_total":        "Disk Total",
		"disk_usage":        "Disk Usage",
		"disk_read_rate":    "Disk Read Rate",
		"disk_write_rate":   "Disk Write Rate",
		"network_in_rate":   "Network In Rate",
		"network_out_rate":  "Network Out Rate",
		"backup_status":     "Backup Status",
		"last_backup":       "Last Backup",
		"days_since_backup": "Days Since Backup",
		"os_name":           "OS Name",
		"os_version":        "OS Version",
		"guest_agent":       "Guest Agent",
		"ip_addresses":      "IP Addresses",
		"tags":              "Tags",
		"user_notes":        "User Notes",
		"user_annotations":  "User Annotations",
	}

	if label, ok := replacements[key]; ok {
		return label
	}
	return key
}

// buildUserAnnotationsContext gathers all user annotations from guests and docker containers
// These provide infrastructure context that the AI should know about for any query
func (s *Service) buildUserAnnotationsContext() string {
	// Return empty if persistence is not available
	if s.persistence == nil {
		return ""
	}

	var annotations []string

	// Load guest metadata
	guestStore, err := s.persistence.LoadGuestMetadata()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load guest metadata for AI context")
	} else {
		guestMeta := guestStore.GetAll()
		log.Debug().Int("count", len(guestMeta)).Msg("Loaded guest metadata for AI context")
		for id, meta := range guestMeta {
			if meta != nil && len(meta.Notes) > 0 {
				// Use LastKnownName if available, otherwise use ID
				name := meta.LastKnownName
				if name == "" {
					name = id
				}
				for _, note := range meta.Notes {
					annotations = append(annotations, fmt.Sprintf("- Guest '%s': %s", name, note))
				}
			}
		}
	}

	// Load docker metadata - include host info for context
	dockerStore, err := s.persistence.LoadDockerMetadata()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load docker metadata for AI context")
	} else {
		dockerMeta := dockerStore.GetAll()
		log.Debug().Int("count", len(dockerMeta)).Msg("Loaded docker metadata for AI context")
		for id, meta := range dockerMeta {
			if meta != nil && len(meta.Notes) > 0 {
				// Extract host and container info from ID (format: hostid:container:containerid)
				name := id
				hostInfo := ""
				parts := strings.Split(id, ":")
				if len(parts) >= 3 {
					hostInfo = parts[0] // First part is the host identifier
					containerID := parts[2]
					if len(containerID) > 12 {
						containerID = containerID[:12]
					}
					name = fmt.Sprintf("Docker container %s", containerID)
				}
				log.Debug().Str("name", name).Str("host", hostInfo).Int("notes", len(meta.Notes)).Msg("Found docker container with annotations")
				for _, note := range meta.Notes {
					if hostInfo != "" {
						annotations = append(annotations, fmt.Sprintf("- %s (on host '%s'): %s", name, hostInfo, note))
					} else {
						annotations = append(annotations, fmt.Sprintf("- %s: %s", name, note))
					}
				}
			}
		}
	}

	log.Debug().Int("total_annotations", len(annotations)).Msg("Built user annotations context")

	if len(annotations) == 0 {
		return ""
	}

	return "\n\n## User Infrastructure Notes\nThe user has added these annotations to describe their infrastructure. USE THESE to understand relationships between systems:\n" + strings.Join(annotations, "\n")
}

// TestConnection tests the AI provider connection
// Tests the provider for the currently configured default model
func (s *Service) TestConnection(ctx context.Context) error {
	s.mu.RLock()
	cfg := s.cfg
	defaultProvider := s.provider
	s.mu.RUnlock()

	// Load config if not available
	if cfg == nil {
		var err error
		cfg, err = s.persistence.LoadAIConfig()
		if err != nil {
			return fmt.Errorf("failed to load Pulse Assistant config: %w", err)
		}
	}

	if cfg == nil || !cfg.IsConfigured() {
		return fmt.Errorf("no provider configured")
	}

	// Try to create a provider for the current default model
	provider, err := providers.NewForModel(cfg, cfg.GetModel())
	if err != nil {
		// Fall back to default provider or NewFromConfig
		log.Debug().Err(err).Str("model", cfg.GetModel()).Msg("Could not create provider for model, using fallback")
		if defaultProvider != nil {
			provider = defaultProvider
		} else {
			provider, err = providers.NewFromConfig(cfg)
			if err != nil {
				return err
			}
		}
	}

	return provider.TestConnection(ctx)
}

// ListModels fetches available models from ALL configured AI providers
// Returns a unified list with models prefixed by provider name
func (s *Service) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	models, _, err := s.ListModelsWithCache(ctx)
	return models, err
}

func (s *Service) ListModelsWithCache(ctx context.Context) ([]providers.ModelInfo, bool, error) {
	cfg, err := s.persistence.LoadAIConfig()
	if err != nil {
		return nil, false, fmt.Errorf("failed to load Pulse Assistant config: %w", err)
	}
	if cfg == nil {
		return nil, false, fmt.Errorf("Pulse Assistant not configured")
	}

	cacheKey := buildModelsCacheKey(cfg)
	s.modelsCache.mu.RLock()
	if s.modelsCache.key == cacheKey && s.modelsCache.ttl > 0 && time.Since(s.modelsCache.at) < s.modelsCache.ttl && len(s.modelsCache.models) > 0 {
		out := make([]providers.ModelInfo, len(s.modelsCache.models))
		copy(out, s.modelsCache.models)
		s.modelsCache.mu.RUnlock()
		return out, true, nil
	}
	s.modelsCache.mu.RUnlock()

	var allModels []providers.ModelInfo

	// Query each configured provider
	providersList := []string{config.AIProviderAnthropic, config.AIProviderOpenAI, config.AIProviderDeepSeek, config.AIProviderGemini, config.AIProviderOllama}

	for _, providerName := range providersList {
		if !cfg.HasProvider(providerName) {
			continue
		}

		// Create provider for this specific provider
		provider, err := providers.NewForProvider(cfg, providerName, "")
		if err != nil {
			log.Debug().Err(err).Str("provider", providerName).Msg("Skipping provider - not configured")
			continue
		}

		// Fetch models from this provider
		models, err := provider.ListModels(ctx)
		if err != nil {
			log.Warn().Err(err).Str("provider", providerName).Msg("Failed to fetch models from provider")
			continue
		}

		// Add provider prefix to each model
		for _, m := range models {
			allModels = append(allModels, providers.ModelInfo{
				ID:          config.FormatModelString(providerName, m.ID),
				Name:        m.Name,
				Description: providerDisplayName(providerName) + ": " + m.ID,
				CreatedAt:   m.CreatedAt,
				Notable:     m.Notable,
			})
		}
	}

	s.modelsCache.mu.Lock()
	s.modelsCache.key = cacheKey
	s.modelsCache.at = time.Now()
	s.modelsCache.models = make([]providers.ModelInfo, len(allModels))
	copy(s.modelsCache.models, allModels)
	s.modelsCache.mu.Unlock()

	return allModels, false, nil
}

func buildModelsCacheKey(cfg *config.AIConfig) string {
	if cfg == nil {
		return "nil"
	}

	var b strings.Builder
	b.WriteString("providers=")
	b.WriteString(strings.Join(cfg.GetConfiguredProviders(), ","))
	b.WriteString("|auth=")
	b.WriteString(string(cfg.AuthMethod))

	b.WriteString("|openai_base=")
	b.WriteString(cfg.OpenAIBaseURL)
	b.WriteString("|ollama_base=")
	b.WriteString(cfg.OllamaBaseURL)

	return b.String()
}

// providerDisplayName returns a user-friendly name for a provider
func providerDisplayName(provider string) string {
	switch provider {
	case config.AIProviderAnthropic:
		return "Anthropic"
	case config.AIProviderOpenAI:
		return "OpenAI"
	case config.AIProviderDeepSeek:
		return "DeepSeek"
	case config.AIProviderGemini:
		return "Google Gemini"
	case config.AIProviderOllama:
		return "Ollama"
	default:
		return provider
	}
}

// Reload reloads the AI configuration (call after settings change)
func (s *Service) Reload() error {
	return s.LoadConfig()
}

// buildRemediationContext adds past remediation history to help AI learn from previous fixes
func (s *Service) buildRemediationContext(resourceID, currentProblem string) string {
	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol == nil {
		return ""
	}

	remLog := patrol.GetRemediationLog()
	if remLog == nil {
		return ""
	}

	var context string

	// Get similar past remediations based on the current problem
	if currentProblem != "" {
		successful := remLog.GetSuccessfulRemediations(currentProblem, 3)
		if len(successful) > 0 {
			context += "\n\n## Past Successful Fixes for Similar Issues\n"
			context += "These actions worked for similar problems before:\n"
			for _, rec := range successful {
				context += fmt.Sprintf("- **%s**: `%s` (%s)\n",
					truncateString(rec.Problem, 60),
					truncateString(rec.Action, 80),
					rec.Outcome)
			}
		}
	}

	// Get history for this specific resource
	if resourceID != "" {
		history := remLog.GetForResource(resourceID, 5)
		if len(history) > 0 {
			context += "\n\n## Remediation History for This Resource\n"
			for _, rec := range history {
				ago := time.Since(rec.Timestamp)
				agoStr := formatDuration(ago)
				context += fmt.Sprintf("- %s ago: %s → `%s` (%s)\n",
					agoStr,
					truncateString(rec.Problem, 50),
					truncateString(rec.Action, 60),
					rec.Outcome)
			}
		}
	}

	return context
}

// buildIncidentContext adds incident timeline context for alerts/resources.
func (s *Service) buildIncidentContext(resourceID, alertID string) string {
	s.mu.RLock()
	store := s.incidentStore
	s.mu.RUnlock()

	if store == nil {
		return ""
	}

	if alertID != "" {
		return store.FormatForAlert(alertID, 8)
	}
	if resourceID != "" {
		return store.FormatForResource(resourceID, 4)
	}
	return ""
}

// RecordIncidentAnalysis stores an AI analysis event for an alert.
func (s *Service) RecordIncidentAnalysis(alertID, summary string, details map[string]interface{}) {
	if alertID == "" {
		return
	}
	s.mu.RLock()
	store := s.incidentStore
	s.mu.RUnlock()
	if store == nil {
		return
	}
	store.RecordAnalysis(alertID, summary, details)
}

// RecordIncidentRunbook stores a runbook execution event for an alert.
func (s *Service) RecordIncidentRunbook(alertID, runbookID, title string, outcome memory.Outcome, automatic bool, message string) {
	if alertID == "" || runbookID == "" {
		return
	}
	s.mu.RLock()
	store := s.incidentStore
	s.mu.RUnlock()
	if store == nil {
		return
	}
	store.RecordRunbook(alertID, runbookID, title, string(outcome), automatic, message)
}

func extractAlertID(ctx map[string]interface{}) string {
	if ctx == nil {
		return ""
	}
	if alertID, ok := ctx["alertId"].(string); ok && alertID != "" {
		return alertID
	}
	if alertID, ok := ctx["alert_id"].(string); ok && alertID != "" {
		return alertID
	}
	return ""
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// buildEnrichedResourceContext adds historical intelligence to regular AI chat
// This is what makes Pulse Pro valuable - context that Claude Code can't have:
// - Baseline comparisons ("this is 2x normal")
// - Trend analysis ("memory growing 5%/day")
// - Predictions ("disk full in 12 days")
// - Recent changes ("config changed 2 hours ago")
func (s *Service) buildEnrichedResourceContext(resourceID, _ string, currentMetrics map[string]interface{}) string {
	s.mu.RLock()
	patrol := s.patrolService
	s.mu.RUnlock()

	if patrol == nil {
		return ""
	}

	var sections []string

	// Get baseline store for comparisons
	baselineStore := patrol.GetBaselineStore()
	if baselineStore != nil {
		var baselineInfo []string

		// Helper to get raw numeric value from context
		// Prefers _raw fields, falls back to regular fields
		getRawValue := func(rawKey, formattedKey string) (float64, bool) {
			// Try raw value first
			if val, ok := currentMetrics[rawKey].(float64); ok && val > 0 {
				return val, true
			}
			// Fallback to formatted value if it's already a float
			if val, ok := currentMetrics[formattedKey].(float64); ok && val > 0 {
				return val, true
			}
			return 0, false
		}

		// Helper to format baseline comparison with status
		// ALWAYS shows baseline info when available - this is Pulse Pro's value-add
		formatBaselineComparison := func(metric string, currentVal float64, bl *baseline.MetricBaseline) string {
			if bl.SampleCount < 10 {
				return fmt.Sprintf("- %s: %.1f%% (baseline learning: %d/10 samples)", metric, currentVal, bl.SampleCount)
			}

			ratio := currentVal / bl.Mean
			status := "✓ normal"
			if ratio > 2.0 {
				status = "🔴 **ANOMALY**"
			} else if ratio > 1.5 {
				status = "🟡 elevated"
			} else if ratio < 0.3 {
				status = "🔵 unusually low"
			} else if ratio < 0.5 {
				status = "low"
			}

			return fmt.Sprintf("- %s: %.1f%% vs baseline %.1f%% (σ=%.1f) → %s", metric, currentVal, bl.Mean, bl.StdDev, status)
		}

		// Check CPU baseline - always show if we have data
		if cpuVal, ok := getRawValue("cpu_usage_raw", "cpu_usage"); ok {
			if bl, exists := baselineStore.GetBaseline(resourceID, "cpu"); exists {
				baselineInfo = append(baselineInfo, formatBaselineComparison("CPU", cpuVal, bl))
			}
		}

		// Check memory baseline - always show if we have data
		if memVal, ok := getRawValue("memory_usage_raw", "memory_usage"); ok {
			if bl, exists := baselineStore.GetBaseline(resourceID, "memory"); exists {
				baselineInfo = append(baselineInfo, formatBaselineComparison("Memory", memVal, bl))
			}
		}

		// Check disk baseline - always show if we have data
		if diskVal, ok := getRawValue("disk_usage_raw", "disk_usage"); ok {
			if bl, exists := baselineStore.GetBaseline(resourceID, "disk"); exists {
				baselineInfo = append(baselineInfo, formatBaselineComparison("Disk", diskVal, bl))
			}
		}

		if len(baselineInfo) > 0 {
			sections = append(sections, "### Learned Baselines (7-day patterns)\n"+strings.Join(baselineInfo, "\n"))
		}
	}

	// Compute trends from metrics history (this works immediately, no learning period)
	metricsHistory := patrol.GetMetricsHistoryProvider()
	if metricsHistory != nil {
		var trendInfo []string
		duration := 24 * time.Hour // Last 24 hours

		// Get metrics for this resource
		// Try guest metrics first, then node metrics
		allMetrics := metricsHistory.GetAllGuestMetrics(resourceID, duration)
		if len(allMetrics) == 0 {
			// Try as node
			cpuPoints := metricsHistory.GetNodeMetrics(resourceID, "cpu", duration)
			memPoints := metricsHistory.GetNodeMetrics(resourceID, "memory", duration)
			if len(cpuPoints) > 0 {
				allMetrics["cpu"] = cpuPoints
			}
			if len(memPoints) > 0 {
				allMetrics["memory"] = memPoints
			}
		}

		// Analyze trends for each metric
		for metric, points := range allMetrics {
			if len(points) < 3 { // Need at least 3 points for trends
				continue
			}

			// Get first and last values
			first := points[0].Value
			last := points[len(points)-1].Value
			duration := points[len(points)-1].Timestamp.Sub(points[0].Timestamp)

			if duration < 30*time.Minute { // Need at least 30min of data
				continue
			}

			// Calculate rate of change per day
			change := last - first
			hoursSpan := duration.Hours()
			ratePerDay := change * (24 / hoursSpan)

			// Only report significant trends
			if metric == "disk" && ratePerDay > 0.5 { // Growing by >0.5GB/day
				trendInfo = append(trendInfo, fmt.Sprintf("**Disk** growing %.1f GB/day over last %.0fh",
					ratePerDay, hoursSpan))
			} else if metric == "memory" && absFloat(ratePerDay) > 2 { // >2% change per day
				direction := "growing"
				if ratePerDay < 0 {
					direction = "declining"
				}
				trendInfo = append(trendInfo, fmt.Sprintf("**Memory** %s %.1f%%/day over last %.0fh",
					direction, absFloat(ratePerDay), hoursSpan))
			} else if metric == "cpu" && absFloat(ratePerDay) > 5 { // >5% change per day
				direction := "increasing"
				if ratePerDay < 0 {
					direction = "decreasing"
				}
				trendInfo = append(trendInfo, fmt.Sprintf("**CPU** %s %.1f%%/day over last %.0fh",
					direction, absFloat(ratePerDay), hoursSpan))
			}
		}

		if len(trendInfo) > 0 {
			sections = append(sections, "### Trends (24h)\n"+strings.Join(trendInfo, "\n"))
		}
	}

	// Get pattern detector for predictions
	patternDetector := patrol.GetPatternDetector()
	if patternDetector != nil {
		predictions := patternDetector.GetPredictionsForResource(resourceID)
		if len(predictions) > 0 {
			var predInfo []string
			for _, pred := range predictions {
				if pred.DaysUntil < 30 { // Only show predictions within 30 days
					predInfo = append(predInfo, fmt.Sprintf("**%s** predicted in %.0f days (%.0f%% confidence)",
						pred.EventType, pred.DaysUntil, pred.Confidence*100))
				}
			}
			if len(predInfo) > 0 {
				sections = append(sections, "### Predictions\n"+strings.Join(predInfo, "\n"))
			}
		}
	}

	// Get change detector for recent changes
	changeDetector := patrol.GetChangeDetector()
	if changeDetector != nil {
		changes := changeDetector.GetChangesForResource(resourceID, 5)
		if len(changes) > 0 {
			var changeInfo []string
			for _, c := range changes {
				if len(changeInfo) >= 3 { // Limit to 3 recent changes
					break
				}
				ago := time.Since(c.DetectedAt).Truncate(time.Minute)
				changeInfo = append(changeInfo, fmt.Sprintf("**%s** %s (%s ago)", c.ChangeType, c.Description, ago))
			}
			if len(changeInfo) > 0 {
				sections = append(sections, "### Recent Changes\n"+strings.Join(changeInfo, "\n"))
			}
		}
	}

	// Get correlation detector for related resources
	correlationDetector := patrol.GetCorrelationDetector()
	if correlationDetector != nil {
		correlations := correlationDetector.GetCorrelationsForResource(resourceID)
		if len(correlations) > 0 {
			var corrInfo []string
			limit := 2
			if len(correlations) < limit {
				limit = len(correlations)
			}
			for _, corr := range correlations[:limit] {
				corrInfo = append(corrInfo, fmt.Sprintf("Correlated with **%s**: %s", corr.TargetName, corr.Description))
			}
			if len(corrInfo) > 0 {
				sections = append(sections, "### Related Resources\n"+strings.Join(corrInfo, "\n"))
			}
		}
	}

	// Get alert history for this resource (this works immediately, uses existing alert data)
	s.mu.RLock()
	alertProvider := s.alertProvider
	s.mu.RUnlock()

	if alertProvider != nil {
		// Get active alerts for this resource
		activeAlerts := alertProvider.GetAlertsByResource(resourceID)

		// Get historical alerts (last 20)
		historicalAlerts := alertProvider.GetAlertHistory(resourceID, 20)

		if len(activeAlerts) > 0 || len(historicalAlerts) > 0 {
			var alertInfo []string

			if len(activeAlerts) > 0 {
				alertInfo = append(alertInfo, fmt.Sprintf("**%d active alert(s)** right now", len(activeAlerts)))
				for _, a := range activeAlerts {
					alertInfo = append(alertInfo, fmt.Sprintf("- %s %s: %s (active %s)",
						strings.ToUpper(a.Level), a.Type, a.Message, a.Duration))
				}
			}

			if len(historicalAlerts) > 0 {
				// Count alerts by type in the last 30 days
				alertsByType := make(map[string]int)
				cutoff := time.Now().Add(-30 * 24 * time.Hour)
				for _, a := range historicalAlerts {
					if a.ResolvedTime.After(cutoff) {
						alertsByType[a.Type]++
					}
				}

				if len(alertsByType) > 0 {
					var typeCounts []string
					for alertType, count := range alertsByType {
						typeCounts = append(typeCounts, fmt.Sprintf("%d %s", count, alertType))
					}
					alertInfo = append(alertInfo, fmt.Sprintf("**Past 30 days:** %s alerts", strings.Join(typeCounts, ", ")))
				}
			}

			if len(alertInfo) > 0 {
				sections = append(sections, "### Alert History\n"+strings.Join(alertInfo, "\n"))
			}
		}
	}

	// If no sections but baseline store exists, show learning status
	if len(sections) == 0 {
		baselineStore := patrol.GetBaselineStore()
		if baselineStore != nil {
			// Check if we're still learning baselines for this resource
			if rb, exists := baselineStore.GetResourceBaseline(resourceID); exists {
				// Count metrics with enough samples
				ready := 0
				learning := 0
				for _, mb := range rb.Metrics {
					if mb.SampleCount >= 10 {
						ready++
					} else if mb.SampleCount > 0 {
						learning++
					}
				}

				if learning > 0 && ready == 0 {
					return "\n\n## Historical Intelligence (Pulse Pro)\n*Learning baselines for this resource... Trends and anomaly detection will be available after more data is collected.*"
				}
			}
		}
		return ""
	}

	return "\n\n## Historical Intelligence (Pulse Pro)\n" + strings.Join(sections, "\n\n")
}

// absFloat returns the absolute value of a float64
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
