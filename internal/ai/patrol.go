package ai

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	aicontext "github.com/rcourtman/pulse-go-rewrite/internal/ai/context"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// ThresholdProvider provides user-configured alert thresholds for patrol to use
type ThresholdProvider interface {
	// GetNodeCPUThreshold returns the CPU alert trigger threshold for nodes (0-100%)
	GetNodeCPUThreshold() float64
	// GetNodeMemoryThreshold returns the memory alert trigger threshold for nodes (0-100%)
	GetNodeMemoryThreshold() float64
	// GetGuestMemoryThreshold returns the memory alert trigger threshold for guests (0-100%)
	GetGuestMemoryThreshold() float64
	// GetGuestDiskThreshold returns the disk alert trigger threshold for guests (0-100%)
	GetGuestDiskThreshold() float64
	// GetStorageThreshold returns the usage alert trigger threshold for storage (0-100%)
	GetStorageThreshold() float64
}

// PatrolThresholds holds calculated thresholds for patrol (derived from alert thresholds)
type PatrolThresholds struct {
	// Node thresholds
	NodeCPUWatch   float64 // CPU % to flag as "watch" (typically alertThreshold - 15)
	NodeCPUWarning float64 // CPU % to flag as "warning" (typically alertThreshold - 5)
	NodeMemWatch   float64
	NodeMemWarning float64
	// Guest thresholds (VMs/containers)
	GuestMemWatch   float64
	GuestMemWarning float64
	GuestDiskWatch  float64
	GuestDiskWarn   float64
	GuestDiskCrit   float64
	// Storage thresholds
	StorageWatch    float64
	StorageWarning  float64
	StorageCritical float64
}

// DefaultPatrolThresholds returns fallback thresholds when no provider is set
func DefaultPatrolThresholds() PatrolThresholds {
	return PatrolThresholds{
		NodeCPUWatch: 75, NodeCPUWarning: 85,
		NodeMemWatch: 75, NodeMemWarning: 85,
		GuestMemWatch: 80, GuestMemWarning: 88,
		GuestDiskWatch: 75, GuestDiskWarn: 85, GuestDiskCrit: 92,
		StorageWatch: 70, StorageWarning: 80, StorageCritical: 90,
	}
}

// CalculatePatrolThresholds derives patrol thresholds from alert thresholds
// Patrol warns ~10% BEFORE the alert fires so users can take action early
func CalculatePatrolThresholds(provider ThresholdProvider) PatrolThresholds {
	if provider == nil {
		return DefaultPatrolThresholds()
	}

	// Get user's alert thresholds
	nodeCPU := provider.GetNodeCPUThreshold()
	nodeMem := provider.GetNodeMemoryThreshold()
	guestMem := provider.GetGuestMemoryThreshold()
	guestDisk := provider.GetGuestDiskThreshold()
	storage := provider.GetStorageThreshold()

	// Calculate patrol thresholds (watch = alert-15%, warning = alert-5%)
	return PatrolThresholds{
		NodeCPUWatch:    clampThreshold(nodeCPU - 15),
		NodeCPUWarning:  clampThreshold(nodeCPU - 5),
		NodeMemWatch:    clampThreshold(nodeMem - 15),
		NodeMemWarning:  clampThreshold(nodeMem - 5),
		GuestMemWatch:   clampThreshold(guestMem - 12),
		GuestMemWarning: clampThreshold(guestMem - 5),
		GuestDiskWatch:  clampThreshold(guestDisk - 15),
		GuestDiskWarn:   clampThreshold(guestDisk - 8),
		GuestDiskCrit:   clampThreshold(guestDisk - 3),
		StorageWatch:    clampThreshold(storage - 15),
		StorageWarning:  clampThreshold(storage - 8),
		StorageCritical: clampThreshold(storage - 3),
	}
}

// clampThreshold ensures a threshold is within valid range
func clampThreshold(v float64) float64 {
	if v < 10 {
		return 10 // Never go below 10%
	}
	if v > 99 {
		return 99
	}
	return v
}

// PatrolConfig holds configuration for the AI patrol service
type PatrolConfig struct {
	// Enabled controls whether background patrol runs
	Enabled bool `json:"enabled"`
	// Interval is how often to run AI patrol analysis
	Interval time.Duration `json:"interval"`
	// QuickCheckInterval is deprecated, kept for backwards compat with old configs
	QuickCheckInterval time.Duration `json:"quick_check_interval,omitempty"`
	// DeepAnalysisInterval is deprecated, kept for backwards compat with old configs
	DeepAnalysisInterval time.Duration `json:"deep_analysis_interval,omitempty"`
	// AnalyzeNodes controls whether to analyze Proxmox nodes
	AnalyzeNodes bool `json:"analyze_nodes"`
	// AnalyzeGuests controls whether to analyze VMs/containers
	AnalyzeGuests bool `json:"analyze_guests"`
	// AnalyzeDocker controls whether to analyze Docker hosts
	AnalyzeDocker bool `json:"analyze_docker"`
	// AnalyzeStorage controls whether to analyze storage
	AnalyzeStorage bool `json:"analyze_storage"`
	// AnalyzePBS controls whether to analyze PBS backup servers
	AnalyzePBS bool `json:"analyze_pbs"`
	// AnalyzeHosts controls whether to analyze agent hosts (RAID, sensors)
	AnalyzeHosts bool `json:"analyze_hosts"`
}

// GetInterval returns the effective patrol interval, handling migration from old config
func (c PatrolConfig) GetInterval() time.Duration {
	if c.Interval > 0 {
		return c.Interval
	}
	// Migrate from old config: use QuickCheckInterval if set
	if c.QuickCheckInterval > 0 {
		return c.QuickCheckInterval
	}
	// Default to 15 minutes
	return 15 * time.Minute
}

// DefaultPatrolConfig returns sensible defaults
func DefaultPatrolConfig() PatrolConfig {
	return PatrolConfig{
		Enabled:        true,
		Interval:       15 * time.Minute,
		AnalyzeNodes:   true,
		AnalyzeGuests:  true,
		AnalyzeDocker:  true,
		AnalyzeStorage: true,
		AnalyzePBS:     true,
		AnalyzeHosts:   true,
	}
}

// PatrolStatus represents the current state of the patrol service
type PatrolStatus struct {
	Running          bool          `json:"running"`
	Enabled          bool          `json:"enabled"`
	LastPatrolAt     *time.Time    `json:"last_patrol_at,omitempty"`
	NextPatrolAt     *time.Time    `json:"next_patrol_at,omitempty"`
	LastDuration     time.Duration `json:"last_duration_ms"`
	ResourcesChecked int           `json:"resources_checked"`
	FindingsCount    int           `json:"findings_count"`
	ErrorCount       int           `json:"error_count"`
	Healthy          bool          `json:"healthy"`
	IntervalMs       int64         `json:"interval_ms"` // Patrol interval in milliseconds
}

// PatrolRunRecord represents a single patrol check run
type PatrolRunRecord struct {
	ID               string        `json:"id"`
	StartedAt        time.Time     `json:"started_at"`
	CompletedAt      time.Time     `json:"completed_at"`
	Duration         time.Duration `json:"duration_ms"`
	Type             string        `json:"type"` // Always "patrol" now (kept for backwards compat)
	ResourcesChecked int           `json:"resources_checked"`
	// Breakdown by resource type
	NodesChecked   int `json:"nodes_checked"`
	GuestsChecked  int `json:"guests_checked"`
	DockerChecked  int `json:"docker_checked"`
	StorageChecked int `json:"storage_checked"`
	HostsChecked   int `json:"hosts_checked"`
	PBSChecked     int `json:"pbs_checked"`
	// Findings from this run
	NewFindings      int      `json:"new_findings"`
	ExistingFindings int      `json:"existing_findings"`
	ResolvedFindings int      `json:"resolved_findings"`
	AutoFixCount     int      `json:"auto_fix_count,omitempty"`
	FindingsSummary  string   `json:"findings_summary"` // e.g., "All healthy" or "2 warnings, 1 critical"
	FindingIDs       []string `json:"finding_ids"`      // IDs of findings from this run
	ErrorCount       int      `json:"error_count"`
	Status           string   `json:"status"` // "healthy", "issues_found", "error"
	// AI Analysis details
	AIAnalysis   string `json:"ai_analysis,omitempty"`   // The AI's raw response/analysis
	InputTokens  int    `json:"input_tokens,omitempty"`  // Tokens sent to AI
	OutputTokens int    `json:"output_tokens,omitempty"` // Tokens received from AI
}

// MaxPatrolRunHistory is the maximum number of patrol runs to keep in history
const MaxPatrolRunHistory = 100

// PatrolService runs background AI analysis of infrastructure
type PatrolService struct {
	mu sync.RWMutex

	aiService           *Service
	stateProvider       StateProvider
	thresholdProvider   ThresholdProvider
	config              PatrolConfig
	findings            *FindingsStore
	knowledgeStore      *knowledge.Store       // For per-resource notes in patrol context
	metricsHistory      MetricsHistoryProvider // For trend analysis and predictions
	baselineStore       *baseline.Store        // For anomaly detection via learned baselines
	changeDetector      *ChangeDetector        // For tracking infrastructure changes
	remediationLog      *RemediationLog        // For tracking remediation actions
	patternDetector     *PatternDetector       // For failure prediction from historical patterns
	correlationDetector *CorrelationDetector   // For multi-resource correlation
	incidentStore       *memory.IncidentStore  // For incident timeline capture

	// Unified intelligence facade - aggregates all subsystems for unified view
	intelligence *Intelligence

	// Cached thresholds (recalculated when thresholdProvider changes)
	thresholds PatrolThresholds

	// Runtime state
	running          bool
	stopCh           chan struct{}
	configChanged    chan struct{} // Signal when config changes to reset ticker
	lastPatrol       time.Time
	lastDuration     time.Duration
	resourcesChecked int
	errorCount       int

	// Patrol run history with persistence support
	runHistoryStore *PatrolRunHistoryStore

	// Live streaming support
	streamMu          sync.RWMutex
	streamSubscribers map[chan PatrolStreamEvent]struct{}
	currentOutput     strings.Builder // Buffer for current streaming output
	streamPhase       string          // "idle", "analyzing", "complete"
}

// PatrolStreamEvent represents a streaming update from the patrol
type PatrolStreamEvent struct {
	Type    string `json:"type"` // "start", "content", "phase", "complete", "error"
	Content string `json:"content,omitempty"`
	Phase   string `json:"phase,omitempty"`  // Current phase description
	Tokens  int    `json:"tokens,omitempty"` // Token count so far
}

// NewPatrolService creates a new patrol service
func NewPatrolService(aiService *Service, stateProvider StateProvider) *PatrolService {
	return &PatrolService{
		aiService:         aiService,
		stateProvider:     stateProvider,
		config:            DefaultPatrolConfig(),
		findings:          NewFindingsStore(),
		thresholds:        DefaultPatrolThresholds(),
		stopCh:            make(chan struct{}),
		runHistoryStore:   NewPatrolRunHistoryStore(MaxPatrolRunHistory),
		streamSubscribers: make(map[chan PatrolStreamEvent]struct{}),
		streamPhase:       "idle",
	}
}

// SetIncidentStore attaches an incident store for alert timeline capture.
func (p *PatrolService) SetIncidentStore(store *memory.IncidentStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.incidentStore = store
}

// GetIncidentStore returns the incident store if configured.
func (p *PatrolService) GetIncidentStore() *memory.IncidentStore {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.incidentStore
}

// SetConfig updates the patrol configuration
func (p *PatrolService) SetConfig(cfg PatrolConfig) {
	p.mu.Lock()
	oldInterval := p.config.GetInterval()
	p.config = cfg
	newInterval := cfg.GetInterval()
	configCh := p.configChanged
	p.mu.Unlock()

	// Signal config change if patrol is running and interval changed
	if configCh != nil && newInterval != oldInterval {
		select {
		case configCh <- struct{}{}:
			log.Info().
				Dur("old_interval", oldInterval).
				Dur("new_interval", newInterval).
				Msg("Patrol interval updated, resetting ticker")
		default:
			// Channel full or not ready, config will be picked up on next cycle
		}
	}
}

// SetThresholdProvider sets the provider for user-configured alert thresholds
// This allows patrol to warn BEFORE alerts fire
func (p *PatrolService) SetThresholdProvider(provider ThresholdProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.thresholdProvider = provider
	p.thresholds = CalculatePatrolThresholds(provider)
	log.Debug().
		Float64("storageWatch", p.thresholds.StorageWatch).
		Float64("storageWarning", p.thresholds.StorageWarning).
		Float64("storageCritical", p.thresholds.StorageCritical).
		Msg("Patrol thresholds updated from alert config")
}

// SetFindingsPersistence enables findings persistence (load from and save to disk)
// This should be called before Start() to load any existing findings
func (p *PatrolService) SetFindingsPersistence(persistence FindingsPersistence) error {
	p.mu.Lock()
	findings := p.findings
	p.mu.Unlock()

	if findings != nil && persistence != nil {
		if err := findings.SetPersistence(persistence); err != nil {
			return err
		}
		log.Info().Msg("AI Patrol findings persistence enabled")
	}
	return nil
}

// SetRunHistoryPersistence enables patrol run history persistence (load from and save to disk)
// This should be called before Start() to load any existing history
func (p *PatrolService) SetRunHistoryPersistence(persistence PatrolHistoryPersistence) error {
	p.mu.Lock()
	store := p.runHistoryStore
	p.mu.Unlock()

	if store != nil && persistence != nil {
		if err := store.SetPersistence(persistence); err != nil {
			return err
		}
		log.Info().Msg("AI Patrol run history persistence enabled")
	}
	return nil
}

// SetKnowledgeStore sets the knowledge store for including per-resource notes in patrol context
func (p *PatrolService) SetKnowledgeStore(store *knowledge.Store) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.knowledgeStore = store
}

// SetMetricsHistoryProvider sets the metrics history provider for enriched context
// This enables the patrol service to compute trends and predictions based on historical data
func (p *PatrolService) SetMetricsHistoryProvider(provider MetricsHistoryProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metricsHistory = provider
	log.Info().Msg("AI Patrol: Metrics history provider set for enriched context")
}

// SetBaselineStore sets the baseline store for anomaly detection
// This enables the patrol service to detect anomalies based on learned normal behavior
func (p *PatrolService) SetBaselineStore(store *baseline.Store) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.baselineStore = store
	log.Info().Msg("AI Patrol: Baseline store set for anomaly detection")
}

// GetBaselineStore returns the baseline store (for external baseline learning)
func (p *PatrolService) GetBaselineStore() *baseline.Store {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.baselineStore
}

// GetMetricsHistoryProvider returns the metrics history provider for trend analysis
func (p *PatrolService) GetMetricsHistoryProvider() MetricsHistoryProvider {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metricsHistory
}

// SetChangeDetector sets the change detector for tracking infrastructure changes
func (p *PatrolService) SetChangeDetector(detector *ChangeDetector) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.changeDetector = detector
	log.Info().Msg("AI Patrol: Change detector set for operational memory")
}

// SetRemediationLog sets the remediation log for tracking fix attempts
func (p *PatrolService) SetRemediationLog(remLog *RemediationLog) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.remediationLog = remLog
	log.Info().Msg("AI Patrol: Remediation log set for operational memory")
}

// GetRemediationLog returns the remediation log (for logging actions)
func (p *PatrolService) GetRemediationLog() *RemediationLog {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.remediationLog
}

// SetPatternDetector sets the pattern detector for failure prediction
func (p *PatrolService) SetPatternDetector(detector *PatternDetector) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.patternDetector = detector
	log.Info().Msg("AI Patrol: Pattern detector set for failure prediction")
}

// GetPatternDetector returns the pattern detector
func (p *PatrolService) GetPatternDetector() *PatternDetector {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.patternDetector
}

// SetCorrelationDetector sets the correlation detector for multi-resource correlation
func (p *PatrolService) SetCorrelationDetector(detector *CorrelationDetector) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.correlationDetector = detector
	log.Info().Msg("AI Patrol: Correlation detector set for multi-resource analysis")
}

// GetCorrelationDetector returns the correlation detector
func (p *PatrolService) GetCorrelationDetector() *CorrelationDetector {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.correlationDetector
}

// GetChangeDetector returns the change detector
func (p *PatrolService) GetChangeDetector() *ChangeDetector {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.changeDetector
}

// GetConfig returns the current patrol configuration
func (p *PatrolService) GetConfig() PatrolConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// GetFindings returns the findings store
func (p *PatrolService) GetFindings() *FindingsStore {
	return p.findings
}

// GetIntelligence returns the unified intelligence facade that aggregates all AI subsystems.
// This provides a single entry point for getting system-wide and resource-specific AI insights.
// The facade is lazily initialized and wires together existing subsystems.
func (p *PatrolService) GetIntelligence() *Intelligence {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Lazy initialization - build facade from existing subsystems
	if p.intelligence == nil {
		p.intelligence = NewIntelligence(IntelligenceConfig{})
	}

	// Always refresh subsystem pointers (they may have been set after intelligence was created)
	p.intelligence.SetSubsystems(
		p.findings,
		p.patternDetector,
		p.correlationDetector,
		p.baselineStore,
		p.incidentStore,
		p.knowledgeStore,
		p.changeDetector,
		p.remediationLog,
	)

	if p.stateProvider != nil {
		p.intelligence.SetStateProvider(p.stateProvider)
	}

	return p.intelligence
}

// GetStatus returns the current patrol status
func (p *PatrolService) GetStatus() PatrolStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	interval := p.config.GetInterval()
	intervalMs := int64(interval / time.Millisecond)

	// "Running" means an analysis is currently in progress, not just the service loop
	// Check streamPhase to determine if we're actively analyzing
	p.streamMu.RLock()
	analysisInProgress := p.streamPhase == "analyzing"
	p.streamMu.RUnlock()

	status := PatrolStatus{
		Running:          analysisInProgress,
		Enabled:          p.config.Enabled,
		LastDuration:     p.lastDuration,
		ResourcesChecked: p.resourcesChecked,
		FindingsCount:    len(p.findings.GetActive(FindingSeverityInfo)),
		ErrorCount:       p.errorCount,
		IntervalMs:       intervalMs,
	}

	if !p.lastPatrol.IsZero() {
		status.LastPatrolAt = &p.lastPatrol
	}

	// Calculate next patrol time if we have interval and last patrol time
	if interval > 0 && !p.lastPatrol.IsZero() {
		next := p.lastPatrol.Add(interval)
		status.NextPatrolAt = &next
	}

	summary := p.findings.GetSummary()
	status.Healthy = summary.IsHealthy()

	return status
}

// SubscribeToStream returns a channel that will receive streaming patrol events
func (p *PatrolService) SubscribeToStream() chan PatrolStreamEvent {
	ch := make(chan PatrolStreamEvent, 100) // Buffered to prevent blocking

	p.streamMu.Lock()
	p.streamSubscribers[ch] = struct{}{}
	// Send current state to new subscriber
	if p.streamPhase != "idle" {
		ch <- PatrolStreamEvent{
			Type:    "content",
			Content: p.currentOutput.String(),
			Phase:   p.streamPhase,
		}
	}
	p.streamMu.Unlock()

	return ch
}

// UnsubscribeFromStream removes a subscriber
func (p *PatrolService) UnsubscribeFromStream(ch chan PatrolStreamEvent) {
	p.streamMu.Lock()
	delete(p.streamSubscribers, ch)
	p.streamMu.Unlock()
	close(ch)
}

// broadcast sends an event to all subscribers
func (p *PatrolService) broadcast(event PatrolStreamEvent) {
	p.streamMu.RLock()
	defer p.streamMu.RUnlock()

	for ch := range p.streamSubscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip (don't block on slow consumers)
		}
	}
}

// appendStreamContent adds content to the current output and broadcasts it
func (p *PatrolService) appendStreamContent(content string) {
	p.streamMu.Lock()
	p.currentOutput.WriteString(content)
	p.streamMu.Unlock()

	p.broadcast(PatrolStreamEvent{
		Type:    "content",
		Content: content,
	})
}

// setStreamPhase updates the current phase (internal state tracking only)
// Does not broadcast phase changes - those are explicit via broadcast()
func (p *PatrolService) setStreamPhase(phase string) {
	p.streamMu.Lock()
	p.streamPhase = phase
	if phase == "idle" {
		p.currentOutput.Reset()
	}
	p.streamMu.Unlock()
	// Note: We don't broadcast phase changes automatically
	// The patrol explicitly broadcasts "start" and "complete" events
}

// GetCurrentStreamOutput returns the current buffered output (for late joiners)
func (p *PatrolService) GetCurrentStreamOutput() (string, string) {
	p.streamMu.RLock()
	defer p.streamMu.RUnlock()
	return p.currentOutput.String(), p.streamPhase
}

// Start begins the background patrol loop
func (p *PatrolService) Start(ctx context.Context) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.stopCh = make(chan struct{})
	p.configChanged = make(chan struct{}, 1) // Buffered to allow non-blocking send
	p.mu.Unlock()

	log.Info().
		Dur("interval", p.config.GetInterval()).
		Msg("Starting AI Patrol Service")

	go p.patrolLoop(ctx)
}

// Stop stops the patrol service
func (p *PatrolService) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.stopCh)
	p.mu.Unlock()

	log.Info().Msg("Stopping AI Patrol Service")
}

// patrolLoop is the main background loop
func (p *PatrolService) patrolLoop(ctx context.Context) {
	// Run initial patrol shortly after startup, but only if one hasn't run recently
	initialDelay := 30 * time.Second
	select {
	case <-time.After(initialDelay):
		// Check if a patrol ran recently (within last hour) to avoid wasting tokens on restarts
		runHistory := p.GetRunHistory(1)

		skipInitial := false
		if len(runHistory) > 0 {
			lastRun := runHistory[0]
			timeSinceLastRun := time.Since(lastRun.CompletedAt)
			if timeSinceLastRun < 1*time.Hour {
				log.Info().
					Dur("time_since_last", timeSinceLastRun).
					Msg("AI Patrol: Skipping initial patrol - recent run exists")
				skipInitial = true
			}
		}

		if !skipInitial {
			p.runPatrol(ctx)
		}
	case <-p.stopCh:
		return
	case <-ctx.Done():
		return
	}

	p.mu.RLock()
	interval := p.config.GetInterval()
	configCh := p.configChanged
	p.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.runPatrol(ctx)

		case <-configCh:
			// Config changed - reset ticker with new interval
			p.mu.RLock()
			newInterval := p.config.GetInterval()
			p.mu.RUnlock()

			if newInterval != interval {
				interval = newInterval
				ticker.Reset(interval)
				log.Info().
					Dur("interval", interval).
					Msg("Patrol ticker reset to new interval")
			}

		case <-p.stopCh:
			return

		case <-ctx.Done():
			return
		}
	}
}

// runPatrol executes a patrol run
func (p *PatrolService) runPatrol(ctx context.Context) {
	p.mu.RLock()
	cfg := p.config
	p.mu.RUnlock()

	if !cfg.Enabled {
		return
	}

	// Check if AI service is enabled
	if p.aiService == nil || !p.aiService.IsEnabled() {
		log.Debug().Msg("AI Patrol: AI service not enabled, skipping patrol")
		return
	}

	start := time.Now()
	patrolType := "patrol" // Simplified - no longer distinguishing quick/deep

	log.Debug().Msg("AI Patrol: Starting patrol run")

	// Track run statistics
	var runStats struct {
		resourceCount    int
		nodesChecked     int
		guestsChecked    int
		dockerChecked    int
		storageChecked   int
		hostsChecked     int
		pbsChecked       int
		newFindings      int
		existingFindings int
		findingIDs       []string
		errors           int
		aiAnalysis       *AIAnalysisResult // Stores the AI's analysis for the run record
	}
	var newFindings []*Finding

	// Get current state
	if p.stateProvider == nil {
		log.Warn().Msg("AI Patrol: No state provider available")
		return
	}

	state := p.stateProvider.GetState()

	// Helper to track findings
	trackFinding := func(f *Finding) bool {
		isNew := p.findings.Add(f)
		if isNew {
			runStats.newFindings++
			newFindings = append(newFindings, f)
			log.Info().
				Str("finding_id", f.ID).
				Str("severity", string(f.Severity)).
				Str("resource", f.ResourceName).
				Str("title", f.Title).
				Msg("AI Patrol: New finding")
		} else {
			runStats.existingFindings++
		}
		runStats.findingIDs = append(runStats.findingIDs, f.ID)
		return isNew
	}

	// Count resources for statistics (but analysis is done by LLM only)
	runStats.nodesChecked = len(state.Nodes)
	runStats.guestsChecked = len(state.VMs) + len(state.Containers)
	runStats.dockerChecked = len(state.DockerHosts)
	runStats.storageChecked = len(state.Storage)
	runStats.pbsChecked = len(state.PBSInstances)
	runStats.hostsChecked = len(state.Hosts)
	runStats.resourceCount = runStats.nodesChecked + runStats.guestsChecked +
		runStats.dockerChecked + runStats.storageChecked + runStats.pbsChecked + runStats.hostsChecked

	hasPatrolFeature := p.aiService == nil || p.aiService.HasLicenseFeature(FeatureAIPatrol)
	// Check license before running LLM analysis (Pro feature)
	if !hasPatrolFeature {
		log.Debug().Msg("AI Patrol: Running heuristic analysis only - requires Pulse Pro license for LLM analysis")
		for _, f := range p.runHeuristicAnalysis(state) {
			trackFinding(f)
		}
	} else {
		// Run AI analysis using the LLM - this is the ONLY analysis method for Pro users
		// The LLM analyzes the infrastructure and identifies issues
		aiResult, aiErr := p.runAIAnalysis(ctx, state)
		if aiErr != nil {
			log.Warn().Err(aiErr).Msg("AI Patrol: LLM analysis failed")
			runStats.errors++

			// Create a finding to surface this error to the user
			errMsg := aiErr.Error()
			var title, description, recommendation string
			if strings.Contains(errMsg, "Insufficient Balance") || strings.Contains(errMsg, "402") {
				title = "AI Patrol: Insufficient API credits"
				description = "The AI patrol cannot analyze your infrastructure because your AI provider account has insufficient credits."
				recommendation = "Add credits to your AI provider account (DeepSeek, OpenAI, etc.) or switch to a different provider in AI Settings."
			} else if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "Unauthorized") {
				title = "AI Patrol: Invalid API key"
				description = "The AI patrol cannot analyze your infrastructure because the API key is invalid or expired."
				recommendation = "Check your API key in AI Settings and verify it is correct."
			} else if strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "429") {
				title = "AI Patrol: Rate limited"
				description = "The AI patrol is being rate limited by your AI provider. Analysis will be retried on the next patrol run."
				recommendation = "Wait for the rate limit to reset, or consider upgrading your API plan for higher limits."
			} else {
				title = "AI Patrol: Analysis failed"
				description = fmt.Sprintf("The AI patrol encountered an error while analyzing your infrastructure: %s", errMsg)
				recommendation = "Check your AI settings and API key. If the problem persists, check the logs for more details."
			}

			errorFinding := &Finding{
				ID:           generateFindingID("ai-service", "reliability", "ai-patrol-error"),
				Key:          "ai-patrol-error",
				Severity:     "warning",
				Category:     "reliability",
				ResourceID:   "ai-service",
				ResourceName: "AI Patrol Service",
				ResourceType: "service",
				Title:        title,
				Description:  description,
				Recommendation: recommendation,
				Evidence:     fmt.Sprintf("Error: %s", errMsg),
				DetectedAt:   time.Now(),
				LastSeenAt:   time.Now(),
			}
			trackFinding(errorFinding)
		} else if aiResult != nil {
			runStats.aiAnalysis = aiResult
			for _, f := range aiResult.Findings {
				trackFinding(f)
			}
		}
	}

	// Auto-fix with runbooks when enabled (Pro only)
	var runbookResolved int
	autoFixEnabled := false
	if p.aiService != nil {
		if aiCfg := p.aiService.GetAIConfig(); aiCfg != nil {
			autoFixEnabled = aiCfg.PatrolAutoFix
		}
	}
	_ = autoFixEnabled // Auto-fix via runbooks removed - dynamic AI remediation handles this

	// Auto-resolve findings that weren't seen in this patrol run
	var resolvedCount int
	if hasPatrolFeature {
		resolvedCount = p.autoResolveStaleFindings(start, nil)

		// Cleanup old resolved findings (only when licensed to modify AI findings)
		cleaned := p.findings.Cleanup(24 * time.Hour)
		if cleaned > 0 {
			log.Debug().Int("cleaned", cleaned).Msg("AI Patrol: Cleaned up old findings")
		}
	} else {
		resolvedCount = p.autoResolveStaleFindings(start, map[string]bool{"heuristic": true})
	}
	resolvedCount += runbookResolved

	duration := time.Since(start)
	completedAt := time.Now()

	// Build findings summary string
	summary := p.findings.GetSummary()
	var findingsSummaryStr string
	var status string
	totalActive := summary.Critical + summary.Warning + summary.Watch
	if totalActive == 0 {
		findingsSummaryStr = "All healthy"
		status = "healthy"
	} else {
		parts := []string{}
		if summary.Critical > 0 {
			parts = append(parts, fmt.Sprintf("%d critical", summary.Critical))
		}
		if summary.Warning > 0 {
			parts = append(parts, fmt.Sprintf("%d warning", summary.Warning))
		}
		if summary.Watch > 0 {
			parts = append(parts, fmt.Sprintf("%d watch", summary.Watch))
		}
		findingsSummaryStr = fmt.Sprintf("%s", joinParts(parts))
		if summary.Critical > 0 {
			status = "critical"
		} else {
			status = "issues_found"
		}
	}
	if runStats.errors > 0 {
		status = "error"
		// Don't claim "All healthy" if there were errors - the patrol didn't complete properly
		if findingsSummaryStr == "All healthy" {
			findingsSummaryStr = fmt.Sprintf("Analysis incomplete (%d errors)", runStats.errors)
		}
	}

	// Create run record
	runRecord := PatrolRunRecord{
		ID:               fmt.Sprintf("%d", start.UnixNano()),
		StartedAt:        start,
		CompletedAt:      completedAt,
		Duration:         duration,
		Type:             patrolType,
		ResourcesChecked: runStats.resourceCount,
		NodesChecked:     runStats.nodesChecked,
		GuestsChecked:    runStats.guestsChecked,
		DockerChecked:    runStats.dockerChecked,
		StorageChecked:   runStats.storageChecked,
		HostsChecked:     runStats.hostsChecked,
		PBSChecked:       runStats.pbsChecked,
		NewFindings:      runStats.newFindings,
		ExistingFindings: runStats.existingFindings,
		ResolvedFindings: resolvedCount,
		AutoFixCount:     runbookResolved,
		FindingsSummary:  findingsSummaryStr,
		FindingIDs:       runStats.findingIDs,
		ErrorCount:       runStats.errors,
		Status:           status,
	}

	// Add AI analysis details if available
	if runStats.aiAnalysis != nil {
		runRecord.AIAnalysis = runStats.aiAnalysis.Response
		runRecord.InputTokens = runStats.aiAnalysis.InputTokens
		runRecord.OutputTokens = runStats.aiAnalysis.OutputTokens
		log.Debug().
			Int("response_length", len(runStats.aiAnalysis.Response)).
			Int("input_tokens", runStats.aiAnalysis.InputTokens).
			Int("output_tokens", runStats.aiAnalysis.OutputTokens).
			Msg("AI Patrol: Storing AI analysis in run record")
	} else {
		log.Debug().Msg("AI Patrol: No AI analysis to store (aiAnalysis is nil)")
	}

	p.mu.Lock()
	p.lastPatrol = completedAt
	p.lastDuration = duration
	p.resourcesChecked = runStats.resourceCount
	p.errorCount = runStats.errors
	p.mu.Unlock()

	// Add to history store (handles persistence automatically)
	p.runHistoryStore.Add(runRecord)

	log.Info().
		Str("type", patrolType).
		Dur("duration", duration).
		Int("resources", runStats.resourceCount).
		Int("new_findings", runStats.newFindings).
		Int("resolved", resolvedCount).
		Int("critical", summary.Critical).
		Int("warning", summary.Warning).
		Int("watch", summary.Watch).
		Msg("AI Patrol: Completed patrol run")
}

// joinParts joins string parts with commas and "and" for the last element
func joinParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 2 {
		return parts[0] + " and " + parts[1]
	}
	return fmt.Sprintf("%s, and %s",
		fmt.Sprintf("%s", parts[0:len(parts)-1]),
		parts[len(parts)-1])
}

// generateFindingID creates a stable ID for a finding based on resource and issue
func generateFindingID(resourceID, category, issue string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", resourceID, category, issue)))
	return fmt.Sprintf("%x", hash[:8])
}

func (p *PatrolService) runHeuristicAnalysis(state models.StateSnapshot) []*Finding {
	p.mu.RLock()
	cfg := p.config
	p.mu.RUnlock()

	var findings []*Finding

	if cfg.AnalyzeNodes {
		for _, node := range state.Nodes {
			findings = append(findings, p.analyzeNode(node)...)
		}
	}

	if cfg.AnalyzeGuests {
		for _, vm := range state.VMs {
			var lastBackup *time.Time
			if !vm.LastBackup.IsZero() {
				lastBackup = &vm.LastBackup
			}
			findings = append(findings, p.analyzeGuest(
				vm.ID, vm.Name, "vm", vm.Node, vm.Status,
				vm.CPU, vm.Memory.Usage, vm.Disk.Usage,
				lastBackup, vm.Template,
			)...)
		}
		for _, ct := range state.Containers {
			var lastBackup *time.Time
			if !ct.LastBackup.IsZero() {
				lastBackup = &ct.LastBackup
			}
			findings = append(findings, p.analyzeGuest(
				ct.ID, ct.Name, "container", ct.Node, ct.Status,
				ct.CPU, ct.Memory.Usage, ct.Disk.Usage,
				lastBackup, ct.Template,
			)...)
		}
	}

	if cfg.AnalyzeDocker {
		for _, host := range state.DockerHosts {
			findings = append(findings, p.analyzeDockerHost(host)...)
		}
	}

	if cfg.AnalyzeStorage {
		for _, storage := range state.Storage {
			findings = append(findings, p.analyzeStorage(storage)...)
		}
	}

	if cfg.AnalyzePBS {
		for _, pbs := range state.PBSInstances {
			findings = append(findings, p.analyzePBSInstance(pbs, state.PBSBackups)...)
		}
	}

	if cfg.AnalyzeHosts {
		for _, host := range state.Hosts {
			findings = append(findings, p.analyzeHost(host)...)
		}
	}

	for _, finding := range findings {
		if finding != nil && finding.Source == "" {
			finding.Source = "heuristic"
		}
	}

	return findings
}

// analyzeNode checks a Proxmox node for issues
func (p *PatrolService) analyzeNode(node models.Node) []*Finding {
	var findings []*Finding

	// Calculate memory usage from Memory struct (as percentage 0-100)
	var memUsagePct float64
	if node.Memory.Total > 0 {
		memUsagePct = float64(node.Memory.Used) / float64(node.Memory.Total) * 100
	}

	// CPU as percentage (node.CPU is 0-1 ratio from Proxmox)
	cpuPct := node.CPU * 100

	// Check for offline nodes
	if node.Status == "offline" || node.Status == "unknown" {
		findings = append(findings, &Finding{
			ID:             generateFindingID(node.ID, "reliability", "offline"),
			Key:            "node-offline",
			Severity:       FindingSeverityCritical,
			Category:       FindingCategoryReliability,
			ResourceID:     node.ID,
			ResourceName:   node.Name,
			ResourceType:   "node",
			Title:          "Node offline",
			Description:    fmt.Sprintf("Node '%s' is not responding", node.Name),
			Recommendation: "Check network connectivity, SSH access, and Proxmox services on the node",
		})
	}

	// High CPU - use dynamic thresholds from user's alert config
	if cpuPct > p.thresholds.NodeCPUWatch {
		severity := FindingSeverityWatch
		if cpuPct > p.thresholds.NodeCPUWarning {
			severity = FindingSeverityWarning
		}
		findings = append(findings, &Finding{
			ID:             generateFindingID(node.ID, "performance", "high-cpu"),
			Key:            "high-cpu",
			Severity:       severity,
			Category:       FindingCategoryPerformance,
			ResourceID:     node.ID,
			ResourceName:   node.Name,
			ResourceType:   "node",
			Title:          "High CPU usage",
			Description:    fmt.Sprintf("Node '%s' CPU at %.0f%%", node.Name, cpuPct),
			Recommendation: "Check which VMs/containers are consuming CPU. Consider load balancing.",
			Evidence:       fmt.Sprintf("CPU: %.1f%%", cpuPct),
		})
	}

	// High memory - use dynamic thresholds
	if memUsagePct > p.thresholds.NodeMemWatch {
		severity := FindingSeverityWatch
		if memUsagePct > p.thresholds.NodeMemWarning {
			severity = FindingSeverityWarning
		}
		findings = append(findings, &Finding{
			ID:             generateFindingID(node.ID, "performance", "high-memory"),
			Key:            "high-memory",
			Severity:       severity,
			Category:       FindingCategoryPerformance,
			ResourceID:     node.ID,
			ResourceName:   node.Name,
			ResourceType:   "node",
			Title:          "High memory usage",
			Description:    fmt.Sprintf("Node '%s' memory at %.0f%%", node.Name, memUsagePct),
			Recommendation: "Consider migrating some VMs to other nodes or increasing node RAM",
			Evidence:       fmt.Sprintf("Memory: %.1f%%", memUsagePct),
		})
	}

	return findings
}

// analyzeGuest checks a VM or container for issues
// Note: cpu is 0-1 ratio, memUsage and diskUsage are already 0-100 percentages from Memory.Usage/Disk.Usage
func (p *PatrolService) analyzeGuest(id, name, guestType, node, status string,
	cpu, memUsage, diskUsage float64, lastBackup *time.Time, template bool) []*Finding {
	var findings []*Finding

	// Skip templates
	if template {
		return findings
	}

	// memUsage and diskUsage are already percentages (0-100)
	memPct := memUsage
	diskPct := diskUsage

	// High memory (sustained) - use dynamic thresholds
	if memPct > p.thresholds.GuestMemWatch {
		severity := FindingSeverityWatch
		if memPct > p.thresholds.GuestMemWarning {
			severity = FindingSeverityWarning
		}
		findings = append(findings, &Finding{
			ID:             generateFindingID(id, "performance", "high-memory"),
			Key:            "high-memory",
			Severity:       severity,
			Category:       FindingCategoryPerformance,
			ResourceID:     id,
			ResourceName:   name,
			ResourceType:   guestType,
			Node:           node,
			Title:          "High memory usage",
			Description:    fmt.Sprintf("'%s' memory at %.0f%% - risk of OOM", name, memPct),
			Recommendation: "Consider increasing allocated RAM or investigating memory-hungry processes",
			Evidence:       fmt.Sprintf("Memory: %.1f%%", memPct),
		})
	}

	// High disk usage - use dynamic thresholds
	if diskPct > p.thresholds.GuestDiskWatch {
		severity := FindingSeverityWatch
		if diskPct > p.thresholds.GuestDiskWarn {
			severity = FindingSeverityWarning
		}
		if diskPct > p.thresholds.GuestDiskCrit {
			severity = FindingSeverityCritical
		}
		findings = append(findings, &Finding{
			ID:             generateFindingID(id, "capacity", "high-disk"),
			Key:            "high-disk",
			Severity:       severity,
			Category:       FindingCategoryCapacity,
			ResourceID:     id,
			ResourceName:   name,
			ResourceType:   guestType,
			Node:           node,
			Title:          "High disk usage",
			Description:    fmt.Sprintf("'%s' disk at %.0f%%", name, diskPct),
			Recommendation: "Clean up old files, logs, or docker images. Consider expanding disk.",
			Evidence:       fmt.Sprintf("Disk: %.1f%%", diskPct),
		})
	}

	// Backup check (only for running guests)
	if status == "running" && lastBackup != nil {
		daysSinceBackup := time.Since(*lastBackup).Hours() / 24
		if daysSinceBackup > 14 {
			severity := FindingSeverityWatch
			if daysSinceBackup > 30 {
				severity = FindingSeverityWarning
			}
			findings = append(findings, &Finding{
				ID:             generateFindingID(id, "backup", "stale"),
				Key:            "backup-stale",
				Severity:       severity,
				Category:       FindingCategoryBackup,
				ResourceID:     id,
				ResourceName:   name,
				ResourceType:   guestType,
				Node:           node,
				Title:          "Backup overdue",
				Description:    fmt.Sprintf("'%s' hasn't been backed up in %.0f days", name, daysSinceBackup),
				Recommendation: "Check backup job configuration or run a manual backup",
				Evidence:       fmt.Sprintf("Last backup: %s", lastBackup.Format("2006-01-02")),
			})
		}
	} else if status == "running" && lastBackup == nil {
		findings = append(findings, &Finding{
			ID:             generateFindingID(id, "backup", "never"),
			Key:            "backup-never",
			Severity:       FindingSeverityWarning,
			Category:       FindingCategoryBackup,
			ResourceID:     id,
			ResourceName:   name,
			ResourceType:   guestType,
			Node:           node,
			Title:          "Never backed up",
			Description:    fmt.Sprintf("'%s' has no backup history", name),
			Recommendation: "Configure backup job for this guest",
		})
	}

	return findings
}

// analyzeDockerHost checks a Docker host for issues
func (p *PatrolService) analyzeDockerHost(host models.DockerHost) []*Finding {
	var findings []*Finding

	hostName := host.Hostname
	if host.DisplayName != "" {
		hostName = host.DisplayName
	}

	// Host offline
	if host.Status != "online" && host.Status != "connected" {
		findings = append(findings, &Finding{
			ID:             generateFindingID(host.ID, "reliability", "offline"),
			Key:            "docker-host-offline",
			Severity:       FindingSeverityCritical,
			Category:       FindingCategoryReliability,
			ResourceID:     host.ID,
			ResourceName:   hostName,
			ResourceType:   "docker_host",
			Title:          "Docker host offline",
			Description:    fmt.Sprintf("Docker host '%s' is not responding", hostName),
			Recommendation: "Check network connectivity and docker-agent service",
		})
	}

	// Check individual containers
	for _, c := range host.Containers {
		// Restarting containers
		if c.State == "restarting" || c.RestartCount > 3 {
			findings = append(findings, &Finding{
				ID:             generateFindingID(c.ID, "reliability", "restart-loop"),
				Key:            "restart-loop",
				Severity:       FindingSeverityWarning,
				Category:       FindingCategoryReliability,
				ResourceID:     c.ID,
				ResourceName:   c.Name,
				ResourceType:   "docker_container",
				Node:           hostName,
				Title:          "Container restart loop",
				Description:    fmt.Sprintf("Container '%s' has restarted %d times", c.Name, c.RestartCount),
				Recommendation: "Check container logs: docker logs " + c.Name,
				Evidence:       fmt.Sprintf("State: %s, Restarts: %d", c.State, c.RestartCount),
			})
		}

		// High memory containers
		if c.MemoryPercent > 90 {
			findings = append(findings, &Finding{
				ID:             generateFindingID(c.ID, "performance", "high-memory"),
				Key:            "high-memory",
				Severity:       FindingSeverityWatch,
				Category:       FindingCategoryPerformance,
				ResourceID:     c.ID,
				ResourceName:   c.Name,
				ResourceType:   "docker_container",
				Node:           hostName,
				Title:          "High memory usage",
				Description:    fmt.Sprintf("Container '%s' using %.0f%% of allocated memory", c.Name, c.MemoryPercent),
				Recommendation: "Consider increasing container memory limit",
				Evidence:       fmt.Sprintf("Memory: %.1f%%", c.MemoryPercent),
			})
		}
	}

	return findings
}

// analyzeStorage checks storage for issues
func (p *PatrolService) analyzeStorage(storage models.Storage) []*Finding {
	var findings []*Finding

	// Note: storage.Usage is already a percentage (0-100, e.g. 85.5 means 85.5%)
	// If Usage is 0 but we have bytes data, calculate it as percentage
	usage := storage.Usage
	if usage == 0 && storage.Total > 0 {
		usage = float64(storage.Used) / float64(storage.Total) * 100
	}

	// High storage usage - use dynamic thresholds from user's alert config
	if usage > p.thresholds.StorageWatch {
		severity := FindingSeverityWatch
		if usage > p.thresholds.StorageWarning {
			severity = FindingSeverityWarning
		}
		if usage > p.thresholds.StorageCritical {
			severity = FindingSeverityCritical
		}

		findings = append(findings, &Finding{
			ID:             generateFindingID(storage.ID, "capacity", "high-usage"),
			Key:            "storage-high-usage",
			Severity:       severity,
			Category:       FindingCategoryCapacity,
			ResourceID:     storage.ID,
			ResourceName:   storage.Name,
			ResourceType:   "storage",
			Title:          "Storage filling up",
			Description:    fmt.Sprintf("Storage '%s' at %.0f%% capacity", storage.Name, usage),
			Recommendation: "Clean up old backups, snapshots, or unused disk images",
			Evidence:       fmt.Sprintf("Usage: %.1f%%", usage),
		})
	}

	return findings
}

// autoResolveHealthyResources marks findings as resolved when they weren't seen in the current patrol
// patrolStartTime is used to determine which findings are stale (LastSeenAt < patrolStartTime)
// Returns the count of findings that were resolved
func (p *PatrolService) autoResolveStaleFindings(patrolStartTime time.Time, sourceAllowlist map[string]bool) int {
	// Get all active findings and check if they're stale
	activeFindings := p.findings.GetActive(FindingSeverityInfo)
	resolvedCount := 0

	for _, f := range activeFindings {
		if sourceAllowlist != nil {
			if !sourceAllowlist[f.Source] {
				continue
			}
		}
		// If the finding wasn't updated during this patrol (LastSeenAt is before patrol started),
		// it means the condition that caused it has been resolved
		if f.LastSeenAt.Before(patrolStartTime) {
			if p.findings.Resolve(f.ID, true) {
				resolvedCount++
				log.Info().
					Str("finding_id", f.ID).
					Str("resource", f.ResourceName).
					Str("title", f.Title).
					Msg("AI Patrol: Auto-resolved finding")
			}
		}
	}
	return resolvedCount
}

// GetFindingsForResource returns active findings for a specific resource
func (p *PatrolService) GetFindingsForResource(resourceID string) []*Finding {
	return p.findings.GetByResource(resourceID)
}

// GetFindingsSummary returns a summary of all findings
func (p *PatrolService) GetFindingsSummary() FindingsSummary {
	return p.findings.GetSummary()
}

// ResolveFinding marks a finding as resolved with a resolution note
// This is called when the AI successfully fixes an issue
func (p *PatrolService) ResolveFinding(findingID string, resolutionNote string) error {
	if findingID == "" {
		return fmt.Errorf("finding ID is required")
	}

	// Get the finding first to update its resolution note
	finding := p.findings.Get(findingID)
	if finding == nil {
		return fmt.Errorf("finding not found: %s", findingID)
	}

	// Update the user note with the resolution
	finding.UserNote = resolutionNote

	// Mark as resolved (not auto-resolved since user/AI initiated it)
	if !p.findings.Resolve(findingID, false) {
		return fmt.Errorf("failed to resolve finding: %s", findingID)
	}

	log.Info().
		Str("finding_id", findingID).
		Str("resolution_note", resolutionNote).
		Msg("AI resolved finding")

	return nil
}

// GetRunHistory returns the history of patrol runs
// If limit is > 0, returns at most that many records
func (p *PatrolService) GetRunHistory(limit int) []PatrolRunRecord {
	if limit <= 0 {
		return p.runHistoryStore.GetAll()
	}
	return p.runHistoryStore.GetRecent(limit)
}

// GetAllFindings returns all active findings sorted by severity
// Only returns critical and warning findings - watch/info are filtered out as noise
func (p *PatrolService) GetAllFindings() []*Finding {
	findings := p.findings.GetActive(FindingSeverityWarning)


	// Sort by severity (critical first) then by time
	severityOrder := map[FindingSeverity]int{
		FindingSeverityCritical: 0,
		FindingSeverityWarning:  1,
		FindingSeverityWatch:    2,
		FindingSeverityInfo:     3,
	}

	sort.Slice(findings, func(i, j int) bool {
		if severityOrder[findings[i].Severity] != severityOrder[findings[j].Severity] {
			return severityOrder[findings[i].Severity] < severityOrder[findings[j].Severity]
		}
		return findings[i].DetectedAt.After(findings[j].DetectedAt)
	})

	return findings
}

// GetFindingsHistory returns all findings including resolved ones for history display
// Optionally filter by startTime
func (p *PatrolService) GetFindingsHistory(startTime *time.Time) []*Finding {
	findings := p.findings.GetAll(startTime)

	// Sort by detected time (newest first)
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].DetectedAt.After(findings[j].DetectedAt)
	})

	return findings
}

// ForcePatrol triggers an immediate patrol run
// The deep parameter is kept for API backwards compatibility but is ignored
// Uses context.Background() since this runs async after the HTTP response
func (p *PatrolService) ForcePatrol(ctx context.Context, deep bool) {
	go p.runPatrol(context.Background())
}

// analyzePBSInstance checks a PBS backup server for issues
func (p *PatrolService) analyzePBSInstance(pbs models.PBSInstance, allBackups []models.PBSBackup) []*Finding {
	var findings []*Finding

	pbsName := pbs.Name
	if pbsName == "" {
		pbsName = pbs.Host
	}

	// Check PBS connectivity
	if pbs.Status != "online" && pbs.Status != "connected" && pbs.Status != "" {
		findings = append(findings, &Finding{
			ID:             generateFindingID(pbs.ID, "reliability", "offline"),
			Key:            "pbs-offline",
			Severity:       FindingSeverityCritical,
			Category:       FindingCategoryReliability,
			ResourceID:     pbs.ID,
			ResourceName:   pbsName,
			ResourceType:   "pbs",
			Title:          "PBS server offline",
			Description:    fmt.Sprintf("Proxmox Backup Server '%s' is not responding", pbsName),
			Recommendation: "Check network connectivity and PBS service status",
		})
	}

	// Check each datastore capacity
	for _, ds := range pbs.Datastores {
		usage := ds.Usage
		if usage == 0 && ds.Total > 0 {
			usage = float64(ds.Used) / float64(ds.Total) * 100
		}

		// PBS datastores should trigger earlier than regular storage
		// since running out of backup space is critical
		if usage > p.thresholds.StorageWatch {
			severity := FindingSeverityWatch
			if usage > p.thresholds.StorageWarning {
				severity = FindingSeverityWarning
			}
			if usage > p.thresholds.StorageCritical {
				severity = FindingSeverityCritical
			}

			findings = append(findings, &Finding{
				ID:             generateFindingID(pbs.ID+":"+ds.Name, "capacity", "high-usage"),
				Key:            "pbs-datastore-high-usage",
				Severity:       severity,
				Category:       FindingCategoryCapacity,
				ResourceID:     pbs.ID + ":" + ds.Name,
				ResourceName:   fmt.Sprintf("%s/%s", pbsName, ds.Name),
				ResourceType:   "pbs_datastore",
				Title:          "PBS datastore filling up",
				Description:    fmt.Sprintf("Datastore '%s' on PBS '%s' at %.0f%% capacity", ds.Name, pbsName, usage),
				Recommendation: "Run garbage collection, prune old backups, or expand storage",
				Evidence:       fmt.Sprintf("Usage: %.1f%%", usage),
			})
		}

		// Check for datastore errors
		if ds.Error != "" {
			findings = append(findings, &Finding{
				ID:             generateFindingID(pbs.ID+":"+ds.Name, "reliability", "error"),
				Key:            "pbs-datastore-error",
				Severity:       FindingSeverityCritical,
				Category:       FindingCategoryReliability,
				ResourceID:     pbs.ID + ":" + ds.Name,
				ResourceName:   fmt.Sprintf("%s/%s", pbsName, ds.Name),
				ResourceType:   "pbs_datastore",
				Title:          "PBS datastore error",
				Description:    fmt.Sprintf("Datastore '%s' has an error: %s", ds.Name, ds.Error),
				Recommendation: "Check PBS server logs and datastore configuration",
				Evidence:       ds.Error,
			})
		}
	}

	// Check for backup staleness per datastore
	// Build a map of latest backup time per datastore
	datastoreLastBackup := make(map[string]time.Time)
	for _, backup := range allBackups {
		if backup.Instance != pbs.ID && backup.Instance != pbs.Name {
			continue
		}
		dsKey := backup.Datastore
		if backup.BackupTime.After(datastoreLastBackup[dsKey]) {
			datastoreLastBackup[dsKey] = backup.BackupTime
		}
	}

	for _, ds := range pbs.Datastores {
		lastBackup, hasBackups := datastoreLastBackup[ds.Name]

		if !hasBackups {
			// No backups found for this datastore - might be intentional (empty datastore)
			// Only warn if datastore has actual content
			if ds.Used > 0 {
				findings = append(findings, &Finding{
					ID:             generateFindingID(pbs.ID+":"+ds.Name, "backup", "no-recent"),
					Key:            "pbs-backup-no-recent",
					Severity:       FindingSeverityWatch,
					Category:       FindingCategoryBackup,
					ResourceID:     pbs.ID + ":" + ds.Name,
					ResourceName:   fmt.Sprintf("%s/%s", pbsName, ds.Name),
					ResourceType:   "pbs_datastore",
					Title:          "No recent backup metadata",
					Description:    fmt.Sprintf("Datastore '%s' has content but no recent backup entries visible", ds.Name),
					Recommendation: "Verify backup jobs are configured and running",
				})
			}
			continue
		}

		hoursSinceBackup := time.Since(lastBackup).Hours()
		if hoursSinceBackup > 48 {
			severity := FindingSeverityWatch
			if hoursSinceBackup > 72 {
				severity = FindingSeverityWarning
			}
			if hoursSinceBackup > 168 { // 7 days
				severity = FindingSeverityCritical
			}

			findings = append(findings, &Finding{
				ID:             generateFindingID(pbs.ID+":"+ds.Name, "backup", "stale"),
				Key:            "pbs-backup-stale",
				Severity:       severity,
				Category:       FindingCategoryBackup,
				ResourceID:     pbs.ID + ":" + ds.Name,
				ResourceName:   fmt.Sprintf("%s/%s", pbsName, ds.Name),
				ResourceType:   "pbs_datastore",
				Title:          "Stale backups",
				Description:    fmt.Sprintf("No backups to '%s/%s' in %.0f hours", pbsName, ds.Name, hoursSinceBackup),
				Recommendation: "Check backup job schedule and logs for failures",
				Evidence:       fmt.Sprintf("Last backup: %s", lastBackup.Format("2006-01-02 15:04")),
			})
		}
	}

	// Check backup jobs for failures
	for _, job := range pbs.BackupJobs {
		if job.Status == "error" || job.Error != "" {
			findings = append(findings, &Finding{
				ID:             generateFindingID(pbs.ID+":job:"+job.ID, "backup", "job-failed"),
				Key:            "pbs-job-failed",
				Severity:       FindingSeverityWarning,
				Category:       FindingCategoryBackup,
				ResourceID:     pbs.ID + ":job:" + job.ID,
				ResourceName:   fmt.Sprintf("%s/job/%s", pbsName, job.ID),
				ResourceType:   "pbs_job",
				Title:          "Backup job failed",
				Description:    fmt.Sprintf("Backup job '%s' on PBS '%s' is failing", job.ID, pbsName),
				Recommendation: "Check PBS task logs for error details",
				Evidence:       job.Error,
			})
		}
	}

	for _, job := range pbs.VerifyJobs {
		if job.Status == "error" || job.Error != "" {
			findings = append(findings, &Finding{
				ID:             generateFindingID(pbs.ID+":verify:"+job.ID, "backup", "verify-failed"),
				Key:            "pbs-verify-failed",
				Severity:       FindingSeverityWarning,
				Category:       FindingCategoryBackup,
				ResourceID:     pbs.ID + ":verify:" + job.ID,
				ResourceName:   fmt.Sprintf("%s/verify/%s", pbsName, job.ID),
				ResourceType:   "pbs_job",
				Title:          "Verify job failed",
				Description:    fmt.Sprintf("Verify job '%s' on PBS '%s' is failing", job.ID, pbsName),
				Recommendation: "Check PBS task logs - verify failures may indicate backup corruption",
				Evidence:       job.Error,
			})
		}
	}

	return findings
}

// analyzeHost checks an agent host for issues (RAID, sensors, connectivity)
func (p *PatrolService) analyzeHost(host models.Host) []*Finding {
	var findings []*Finding

	hostName := host.DisplayName
	if hostName == "" {
		hostName = host.Hostname
	}

	// Check host connectivity
	if host.Status != "online" && host.Status != "connected" && host.Status != "" {
		findings = append(findings, &Finding{
			ID:             generateFindingID(host.ID, "reliability", "offline"),
			Key:            "host-offline",
			Severity:       FindingSeverityCritical,
			Category:       FindingCategoryReliability,
			ResourceID:     host.ID,
			ResourceName:   hostName,
			ResourceType:   "host",
			Title:          "Host agent offline",
			Description:    fmt.Sprintf("Host '%s' agent is not reporting", hostName),
			Recommendation: "Check network connectivity and pulse-agent service status",
		})
	}

	// Check RAID arrays
	for _, raid := range host.RAID {
		raidName := raid.Device
		if raid.Name != "" {
			raidName = raid.Name
		}

		// Check for degraded/failed state
		switch raid.State {
		case "degraded", "DEGRADED":
			findings = append(findings, &Finding{
				ID:             generateFindingID(host.ID+":"+raid.Device, "reliability", "raid-degraded"),
				Key:            "raid-degraded",
				Severity:       FindingSeverityCritical,
				Category:       FindingCategoryReliability,
				ResourceID:     host.ID + ":" + raid.Device,
				ResourceName:   fmt.Sprintf("%s/%s", hostName, raidName),
				ResourceType:   "host_raid",
				Title:          "RAID array degraded",
				Description:    fmt.Sprintf("RAID array '%s' on '%s' is degraded (%d/%d devices active)", raidName, hostName, raid.ActiveDevices, raid.TotalDevices),
				Recommendation: "Replace failed drive and initiate rebuild. Check dmesg for drive errors.",
				Evidence:       fmt.Sprintf("State: %s, Active: %d/%d, Failed: %d", raid.State, raid.ActiveDevices, raid.TotalDevices, raid.FailedDevices),
			})

		case "recovering", "rebuilding", "resyncing", "RECOVERING":
			severity := FindingSeverityWarning
			if raid.RebuildPercent < 50 {
				severity = FindingSeverityWatch // Early in rebuild, less urgent
			}
			findings = append(findings, &Finding{
				ID:             generateFindingID(host.ID+":"+raid.Device, "reliability", "raid-rebuilding"),
				Key:            "raid-rebuilding",
				Severity:       severity,
				Category:       FindingCategoryReliability,
				ResourceID:     host.ID + ":" + raid.Device,
				ResourceName:   fmt.Sprintf("%s/%s", hostName, raidName),
				ResourceType:   "host_raid",
				Title:          "RAID array rebuilding",
				Description:    fmt.Sprintf("RAID array '%s' on '%s' is rebuilding (%.1f%% complete)", raidName, hostName, raid.RebuildPercent),
				Recommendation: "Monitor rebuild progress. Avoid heavy I/O if possible. Array is vulnerable until rebuild completes.",
				Evidence:       fmt.Sprintf("State: %s, Progress: %.1f%%, Speed: %s", raid.State, raid.RebuildPercent, raid.RebuildSpeed),
			})

		case "inactive", "INACTIVE":
			findings = append(findings, &Finding{
				ID:             generateFindingID(host.ID+":"+raid.Device, "reliability", "raid-inactive"),
				Key:            "raid-inactive",
				Severity:       FindingSeverityCritical,
				Category:       FindingCategoryReliability,
				ResourceID:     host.ID + ":" + raid.Device,
				ResourceName:   fmt.Sprintf("%s/%s", hostName, raidName),
				ResourceType:   "host_raid",
				Title:          "RAID array inactive",
				Description:    fmt.Sprintf("RAID array '%s' on '%s' is inactive", raidName, hostName),
				Recommendation: "RAID array is not running. Check mdadm status and system logs.",
				Evidence:       fmt.Sprintf("State: %s", raid.State),
			})
		}

		// Check for failed devices even if array state is "clean"
		if raid.FailedDevices > 0 && raid.State != "degraded" {
			findings = append(findings, &Finding{
				ID:             generateFindingID(host.ID+":"+raid.Device, "reliability", "raid-failed-devices"),
				Key:            "raid-failed-devices",
				Severity:       FindingSeverityWarning,
				Category:       FindingCategoryReliability,
				ResourceID:     host.ID + ":" + raid.Device,
				ResourceName:   fmt.Sprintf("%s/%s", hostName, raidName),
				ResourceType:   "host_raid",
				Title:          "RAID has failed devices",
				Description:    fmt.Sprintf("RAID array '%s' on '%s' has %d failed device(s)", raidName, hostName, raid.FailedDevices),
				Recommendation: "Replace failed drives. Array may still be operational due to spares.",
				Evidence:       fmt.Sprintf("Failed: %d, Spare: %d", raid.FailedDevices, raid.SpareDevices),
			})
		}
	}

	// Check high temperature
	if len(host.Sensors.TemperatureCelsius) > 0 {
		for sensorName, temp := range host.Sensors.TemperatureCelsius {
			if temp > 85 {
				severity := FindingSeverityWarning
				if temp > 95 {
					severity = FindingSeverityCritical
				}
				findings = append(findings, &Finding{
					ID:             generateFindingID(host.ID+":temp:"+sensorName, "reliability", "high-temp"),
					Key:            "high-temp",
					Severity:       severity,
					Category:       FindingCategoryReliability,
					ResourceID:     host.ID + ":temp:" + sensorName,
					ResourceName:   fmt.Sprintf("%s/%s", hostName, sensorName),
					ResourceType:   "host_sensor",
					Title:          "High temperature",
					Description:    fmt.Sprintf("Sensor '%s' on '%s' reading %.0f°C", sensorName, hostName, temp),
					Recommendation: "Check cooling, fans, and airflow. High temps can cause hardware damage.",
					Evidence:       fmt.Sprintf("Temperature: %.1f°C", temp),
				})
			}
		}
	}

	return findings
}

// AIAnalysisResult contains the results of an AI analysis
type AIAnalysisResult struct {
	Response     string     // The AI's raw response text
	Findings     []*Finding // Parsed findings from the response
	InputTokens  int
	OutputTokens int
}

// runAIAnalysis uses the LLM to analyze infrastructure and identify issues
func (p *PatrolService) runAIAnalysis(ctx context.Context, state models.StateSnapshot) (*AIAnalysisResult, error) {
	if p.aiService == nil {
		return nil, fmt.Errorf("AI service not available")
	}

	// Build enriched infrastructure context with trends and predictions
	// Falls back to basic summary if metrics history is not available
	summary := p.buildEnrichedContext(state)
	if summary == "" {
		return nil, nil // Nothing to analyze
	}

	prompt := p.buildPatrolPrompt(summary)

	log.Debug().Msg("AI Patrol: Sending infrastructure to LLM for analysis")

	// Start streaming phase
	p.setStreamPhase("analyzing")
	p.broadcast(PatrolStreamEvent{Type: "start"})

	// Use streaming to broadcast updates in real-time
	var contentBuffer strings.Builder
	var inputTokens, outputTokens int

	resp, err := p.aiService.ExecuteStream(ctx, ExecuteRequest{
		Prompt:       prompt,
		SystemPrompt: p.getPatrolSystemPrompt(),
		UseCase:      "patrol", // Use patrol model for background analysis
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			if content, ok := event.Data.(string); ok {
				contentBuffer.WriteString(content)
				p.appendStreamContent(content)
			}
		case "thinking":
			// Thinking chunks become separate blocks (like AI chat)
			if thinking, ok := event.Data.(string); ok && thinking != "" {
				contentBuffer.WriteString(thinking)
				// Send as a "thinking" event type so frontend can style it differently
				p.broadcast(PatrolStreamEvent{
					Type:    "thinking",
					Content: thinking,
				})
			}
		}
	})

	if err != nil {
		p.setStreamPhase("idle")
		p.broadcast(PatrolStreamEvent{Type: "error", Content: err.Error()})
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	// Use response content (streaming may have captured it already)
	finalContent := resp.Content
	if finalContent == "" {
		finalContent = contentBuffer.String()
	}
	inputTokens = resp.InputTokens
	outputTokens = resp.OutputTokens

	log.Debug().
		Int("input_tokens", inputTokens).
		Int("output_tokens", outputTokens).
		Int("content_length", len(finalContent)).
		Msg("AI Patrol: LLM analysis complete")

	// Broadcast completion
	p.broadcast(PatrolStreamEvent{
		Type:   "complete",
		Tokens: outputTokens,
	})
	p.setStreamPhase("idle")

	// Parse findings from AI response
	findings := p.parseAIFindings(finalContent)

	return &AIAnalysisResult{
		Response:     finalContent,
		Findings:     findings,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}, nil
}

// getPatrolSystemPrompt returns the system prompt for AI patrol analysis
// The prompt varies based on whether auto-fix mode is enabled
func (p *PatrolService) getPatrolSystemPrompt() string {
	autoFix := false
	if cfg := p.aiService.GetAIConfig(); cfg != nil {
		autoFix = cfg.PatrolAutoFix
	}

	basePrompt := `You are an infrastructure analyst for Pulse, a Proxmox monitoring system. Your job is to analyze infrastructure state and identify issues, potential problems, and optimization opportunities.

IMPORTANT: You must respond in a specific structured format so findings can be parsed.

For each issue you identify, output a finding block like this:

[FINDING]
KEY: <stable issue key>
SEVERITY: critical|warning|watch|info
CATEGORY: performance|reliability|security|capacity|configuration
RESOURCE: <resource name or ID>
RESOURCE_TYPE: node|vm|container|docker_container|storage|host
TITLE: <brief issue title>
DESCRIPTION: <detailed description of the issue>
RECOMMENDATION: <specific actionable recommendation>
EVIDENCE: <specific data that supports this finding>
[/FINDING]

Guidelines:
- Use KEY as a stable identifier for the issue type (examples: high-cpu, high-memory, high-disk, backup-stale, backup-never, restart-loop, storage-high-usage, pbs-datastore-high-usage, pbs-job-failed, node-offline). Use "general" if nothing fits.

SEVERITY GUIDELINES (be conservative - fewer findings are better than noisy alerts):
- CRITICAL: Immediate action required (data loss risk, service down, disk >95%)
- WARNING: Should be addressed soon (disk >85%, memory >90%, consistent failures)  
- WATCH: Only use for SIGNIFICANT trends that project to hit critical in <7 days
- INFO: Informational observations for context (stopped services, config notes)

IMPORTANT - DO NOT REPORT:
- Small baseline deviations (CPU at 7% vs typical 4% is NORMAL variance)
- Low absolute utilization (anything under 50% CPU or 60% memory is fine)
- Stopped containers UNLESS they should be running (check autostart)
- "Elevated" metrics that are still well under limits

Only flag something if an operator would actually need to take action.

Focus on:
1. Capacity issues that will become critical soon (projected disk full, memory exhaustion)
2. Actual failures or errors (service crashes, backup failures)
3. Configuration problems (missing backups, insecure settings)
4. Correlation between resources when it indicates a root cause

If everything looks healthy, respond with NO findings. Users prefer silence to noise.`


	if autoFix {
		return basePrompt + `

AUTO-FIX MODE ENABLED: You may use the run_command tool to attempt automatic remediation of issues you find.

Safe operations you can perform autonomously:
- Restart services (systemctl restart)
- Clear caches and temp files
- Rotate/compress logs
- Trigger garbage collection

Operations requiring extra caution:
- Deleting files (prefer moving to /tmp first)
- Installing packages
- Modifying configurations

Always:
1. Run a verification command after any fix to confirm success
2. Log what action was taken and the outcome
3. Stop and report if the fix doesn't resolve the issue`
	}

	return basePrompt + `

OBSERVE ONLY MODE: You are in observation mode. You may use read-only commands to gather diagnostic information (checking status, memory usage, disk space, logs, etc.) but DO NOT modify anything. Present your findings with clear recommendations for the user to review and action manually.`
}

// buildInfrastructureSummary creates a text summary of infrastructure state for the AI
func (p *PatrolService) buildInfrastructureSummary(state models.StateSnapshot) string {
	var sb strings.Builder

	sb.WriteString("# Infrastructure State Summary\n\n")

	// Nodes
	if len(state.Nodes) > 0 {
		sb.WriteString("## Proxmox Nodes\n")
		for _, n := range state.Nodes {
			memPct := 0.0
			if n.Memory.Total > 0 {
				memPct = float64(n.Memory.Used) / float64(n.Memory.Total) * 100
			}
			sb.WriteString(fmt.Sprintf("- **%s**: Status=%s, CPU=%.1f%%, Memory=%.1f%%, Uptime=%s\n",
				n.Name, n.Status, n.CPU*100, memPct, formatDurationPatrol(time.Duration(n.Uptime)*time.Second)))
		}
		sb.WriteString("\n")
	}

	// VMs
	if len(state.VMs) > 0 {
		sb.WriteString("## Virtual Machines\n")
		for _, vm := range state.VMs {
			if vm.Template {
				continue // Skip templates
			}
			memPct := 0.0
			if vm.Memory.Total > 0 {
				memPct = float64(vm.Memory.Used) / float64(vm.Memory.Total) * 100
			}
			backupStatus := "never"
			if !vm.LastBackup.IsZero() {
				backupStatus = fmt.Sprintf("%s ago", time.Since(vm.LastBackup).Round(time.Hour))
			}
			sb.WriteString(fmt.Sprintf("- **%s** (ID:%s, Node:%s): Status=%s, CPU=%.1f%%, Memory=%.1f%%, LastBackup=%s\n",
				vm.Name, vm.ID, vm.Node, vm.Status, vm.CPU*100, memPct, backupStatus))
		}
		sb.WriteString("\n")
	}

	// Containers
	if len(state.Containers) > 0 {
		sb.WriteString("## LXC Containers\n")
		for _, ct := range state.Containers {
			if ct.Template {
				continue
			}
			memPct := 0.0
			if ct.Memory.Total > 0 {
				memPct = float64(ct.Memory.Used) / float64(ct.Memory.Total) * 100
			}
			backupStatus := "never"
			if !ct.LastBackup.IsZero() {
				backupStatus = fmt.Sprintf("%s ago", time.Since(ct.LastBackup).Round(time.Hour))
			}
			sb.WriteString(fmt.Sprintf("- **%s** (ID:%s, Node:%s): Status=%s, CPU=%.1f%%, Memory=%.1f%%, LastBackup=%s\n",
				ct.Name, ct.ID, ct.Node, ct.Status, ct.CPU*100, memPct, backupStatus))
		}
		sb.WriteString("\n")
	}

	// Storage
	if len(state.Storage) > 0 {
		sb.WriteString("## Storage\n")
		for _, st := range state.Storage {
			usedPct := 0.0
			if st.Total > 0 {
				usedPct = float64(st.Used) / float64(st.Total) * 100
			}
			sb.WriteString(fmt.Sprintf("- **%s** (Node:%s, Type:%s): %.1f%% used (%s / %s)\n",
				st.Name, st.Node, st.Type, usedPct, formatBytesInt64(st.Used), formatBytesInt64(st.Total)))
		}
		sb.WriteString("\n")
	}

	// Docker hosts
	if len(state.DockerHosts) > 0 {
		sb.WriteString("## Docker Hosts\n")
		for _, dh := range state.DockerHosts {
			sb.WriteString(fmt.Sprintf("- **%s**: Status=%s, Containers=%d\n",
				dh.Hostname, dh.Status, len(dh.Containers)))
			for _, c := range dh.Containers {
				sb.WriteString(fmt.Sprintf("  - %s: State=%s, CPU=%.1f%%, Memory=%.1f%%, Restarts=%d\n",
					c.Name, c.State, c.CPUPercent, c.MemoryPercent, c.RestartCount))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// buildEnrichedContext creates context with historical trends and predictions
// Falls back to basic summary if metrics history is not available
func (p *PatrolService) buildEnrichedContext(state models.StateSnapshot) string {
	p.mu.RLock()
	metricsHistory := p.metricsHistory
	knowledgeStore := p.knowledgeStore
	baselineStore := p.baselineStore
	changeDetector := p.changeDetector
	p.mu.RUnlock()

	// If no metrics history, fall back to basic summary
	if metricsHistory == nil {
		log.Debug().Msg("AI Patrol: No metrics history available, using basic summary")
		return p.buildInfrastructureSummary(state)
	}

	// Build enriched context using the context package
	builder := aicontext.NewBuilder().
		WithMetricsHistory(&metricsHistoryShim{provider: metricsHistory})

	// Add knowledge store if available
	if knowledgeStore != nil {
		builder = builder.WithKnowledge(&knowledgeShim{store: knowledgeStore})
	}

	// Add baseline provider for anomaly detection if available
	if baselineStore != nil {
		adapter := NewBaselineStoreAdapter(baselineStore)
		if adapter != nil {
			builder = builder.WithBaseline(&baselineShim{adapter: adapter})
		}
	}

	// Build full infrastructure context with trends
	infraCtx := builder.BuildForInfrastructure(state)
	if infraCtx == nil {
		log.Warn().Msg("AI Patrol: Failed to build enriched context, falling back")
		return p.buildInfrastructureSummary(state)
	}

	// Format for AI consumption
	formatted := aicontext.FormatInfrastructureContext(infraCtx)

	// Append recent changes if change detector is available
	if changeDetector != nil {
		// Detect any new changes from current state
		snapshots := stateToSnapshots(state)
		newChanges := changeDetector.DetectChanges(snapshots)

		// Get summary of recent changes (last 24 hours)
		since := time.Now().Add(-24 * time.Hour)
		changesSummary := changeDetector.GetChangesSummary(since, 20)

		if changesSummary != "" {
			formatted += "\n## Recent Infrastructure Changes (24h)\n\n" + changesSummary
		}

		if len(newChanges) > 0 {
			log.Debug().Int("new_changes", len(newChanges)).Msg("AI Patrol: Detected infrastructure changes")
		}
	}

	// Append failure predictions if pattern detector is available
	p.mu.RLock()
	patternDetector := p.patternDetector
	correlationDetector := p.correlationDetector
	p.mu.RUnlock()

	if patternDetector != nil {
		predictionsContext := patternDetector.FormatForContext("")
		if predictionsContext != "" {
			formatted += predictionsContext
		}
	}

	// Append resource correlations if correlation detector is available
	if correlationDetector != nil {
		correlationsContext := correlationDetector.FormatForContext("")
		if correlationsContext != "" {
			formatted += correlationsContext
		}
	}

	log.Debug().
		Int("resources", infraCtx.TotalResources).
		Int("predictions", len(infraCtx.Predictions)).
		Int("anomalies", len(infraCtx.Anomalies)).
		Msg("AI Patrol: Built enriched context with trends")

	return formatted
}

// stateToSnapshots converts state to resource snapshots for change detection
func stateToSnapshots(state models.StateSnapshot) []ResourceSnapshot {
	var snapshots []ResourceSnapshot

	for _, node := range state.Nodes {
		snapshots = append(snapshots, ResourceSnapshot{
			ID:          node.ID,
			Name:        node.Name,
			Type:        "node",
			Status:      node.Status,
			CPUCores:    node.CPUInfo.Cores,
			MemoryBytes: node.Memory.Total,
		})
	}

	for _, vm := range state.VMs {
		if vm.Template {
			continue
		}
		snapshots = append(snapshots, ResourceSnapshot{
			ID:          vm.ID,
			Name:        vm.Name,
			Type:        "vm",
			Status:      vm.Status,
			Node:        vm.Node,
			CPUCores:    vm.CPUs,
			MemoryBytes: vm.Memory.Total,
			DiskBytes:   vm.Disk.Total,
			LastBackup:  vm.LastBackup,
		})
	}

	for _, ct := range state.Containers {
		if ct.Template {
			continue
		}
		snapshots = append(snapshots, ResourceSnapshot{
			ID:          ct.ID,
			Name:        ct.Name,
			Type:        "container",
			Status:      ct.Status,
			Node:        ct.Node,
			CPUCores:    ct.CPUs,
			MemoryBytes: ct.Memory.Total,
			DiskBytes:   ct.Disk.Total,
			LastBackup:  ct.LastBackup,
		})
	}

	return snapshots
}

// metricsHistoryShim adapts ai.MetricsHistoryProvider to aicontext.MetricsHistoryProvider
type metricsHistoryShim struct {
	provider MetricsHistoryProvider
}

func (s *metricsHistoryShim) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []aicontext.MetricPoint {
	if s.provider == nil {
		return nil
	}
	points := s.provider.GetNodeMetrics(nodeID, metricType, duration)
	return convertToContextPoints(points)
}

func (s *metricsHistoryShim) GetGuestMetrics(guestID string, metricType string, duration time.Duration) []aicontext.MetricPoint {
	if s.provider == nil {
		return nil
	}
	points := s.provider.GetGuestMetrics(guestID, metricType, duration)
	return convertToContextPoints(points)
}

func (s *metricsHistoryShim) GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]aicontext.MetricPoint {
	if s.provider == nil {
		return nil
	}
	metricsMap := s.provider.GetAllGuestMetrics(guestID, duration)
	return convertToContextMetricsMap(metricsMap)
}

func (s *metricsHistoryShim) GetAllStorageMetrics(storageID string, duration time.Duration) map[string][]aicontext.MetricPoint {
	if s.provider == nil {
		return nil
	}
	metricsMap := s.provider.GetAllStorageMetrics(storageID, duration)
	return convertToContextMetricsMap(metricsMap)
}

// knowledgeShim adapts knowledge.Store to aicontext.KnowledgeProvider
type knowledgeShim struct {
	store *knowledge.Store
}

func (k *knowledgeShim) GetNotes(guestID string) []string {
	if k.store == nil {
		return nil
	}
	knowledge, err := k.store.GetKnowledge(guestID)
	if err != nil || knowledge == nil {
		return nil
	}
	// Extract note contents
	var notes []string
	for _, note := range knowledge.Notes {
		notes = append(notes, note.Content)
	}
	return notes
}

func (k *knowledgeShim) FormatAllForContext() string {
	if k.store == nil {
		return ""
	}
	return k.store.FormatAllForContext()
}

// baselineShim adapts BaselineStoreAdapter to aicontext.BaselineProvider
type baselineShim struct {
	adapter *BaselineStoreAdapter
}

func (b *baselineShim) CheckAnomaly(resourceID, metric string, value float64) (severity string, zScore float64, mean float64, stddev float64, ok bool) {
	if b.adapter == nil {
		return "", 0, 0, 0, false
	}
	return b.adapter.CheckAnomaly(resourceID, metric, value)
}

func (b *baselineShim) GetBaseline(resourceID, metric string) (mean float64, stddev float64, sampleCount int, ok bool) {
	if b.adapter == nil {
		return 0, 0, 0, false
	}
	return b.adapter.GetBaseline(resourceID, metric)
}

// convertToContextPoints converts ai.MetricPoint to aicontext.MetricPoint
// Since both are aliases for types.MetricPoint, this is just a type assertion
func convertToContextPoints(points []MetricPoint) []aicontext.MetricPoint {
	if points == nil {
		return nil
	}
	// Both types are aliases for types.MetricPoint, so they're compatible
	result := make([]aicontext.MetricPoint, len(points))
	for i, p := range points {
		result[i] = aicontext.MetricPoint{
			Value:     p.Value,
			Timestamp: p.Timestamp,
		}
	}
	return result
}

// convertToContextMetricsMap converts a map of metric points
func convertToContextMetricsMap(metricsMap map[string][]MetricPoint) map[string][]aicontext.MetricPoint {
	if metricsMap == nil {
		return nil
	}
	result := make(map[string][]aicontext.MetricPoint, len(metricsMap))
	for key, points := range metricsMap {
		result[key] = convertToContextPoints(points)
	}
	return result
}

// buildPatrolPrompt creates the prompt for AI analysis
// Includes user feedback context to prevent re-raising dismissed findings
func (p *PatrolService) buildPatrolPrompt(summary string) string {
	// Get user feedback context (dismissed/snoozed findings)
	feedbackContext := p.findings.GetDismissedForContext()

	// Get resource notes from knowledge store (per-resource user notes)
	var knowledgeContext string
	var incidentContext string
	p.mu.RLock()
	knowledgeStore := p.knowledgeStore
	incidentStore := p.incidentStore
	p.mu.RUnlock()
	if knowledgeStore != nil {
		knowledgeContext = knowledgeStore.FormatAllForContext()
	}
	if incidentStore != nil {
		incidentContext = incidentStore.FormatForPatrol(8)
	}

	basePrompt := fmt.Sprintf(`Please perform a comprehensive analysis of the following infrastructure and identify any issues, potential problems, or optimization opportunities.

%s

Analyze the above and report any findings using the structured format. Focus on:
- Resources showing high utilization or concerning trends (look for "rising" indicators)
- Predictions showing resources approaching capacity
- Anomalies flagged as unusual in the "ANOMALIES" section
- Patterns that might indicate problems over time (compare 24h vs 7d trends)
- Missing backups or stale backup schedules  
- Unbalanced resource distribution

IMPORTANT: The context includes historical trends (24h and 7d) where available. Use this to provide actionable insights:
- A resource that's "growing 5%%/day" needs proactive attention
- A resource that's "stable" with high usage may just need monitoring
- A "volatile" resource may indicate workload issues

If predictions show a resource will be full within 7 days, flag it as high priority.
If everything looks healthy with stable trends, say so briefly.`, summary)

	var contextAdditions strings.Builder

	// Append knowledge context (user notes about resources)
	if knowledgeContext != "" {
		contextAdditions.WriteString("\n\n")
		contextAdditions.WriteString(knowledgeContext)
		contextAdditions.WriteString("\nIMPORTANT: Consider the user's saved notes above when analyzing. If a user has noted that a resource behaves a certain way (e.g., 'runs hot for transcoding'), do not flag it as an issue.\n")
	}

	// Append user feedback context (dismissed/snoozed findings)
	if feedbackContext != "" {
		contextAdditions.WriteString("\n\n")
		contextAdditions.WriteString(feedbackContext)
		contextAdditions.WriteString(`

IMPORTANT: Respect the user's feedback above. Do NOT re-raise findings that are:
- Permanently suppressed - the user has explicitly said to never mention these again
- Dismissed as "not_an_issue" or "expected_behavior" - the user knows about these
- Currently snoozed - only re-raise if the severity has significantly worsened

Only report NEW issues or issues where the severity has clearly escalated.`)
	}

	if incidentContext != "" {
		contextAdditions.WriteString("\n\n")
		contextAdditions.WriteString(incidentContext)
		contextAdditions.WriteString("\nIMPORTANT: Use incident memory to avoid repeating known issues and to build on successful past investigations.")
	}

	if contextAdditions.Len() > 0 {
		return basePrompt + contextAdditions.String()
	}

	return basePrompt
}

// parseAIFindings extracts structured findings from AI response
func (p *PatrolService) parseAIFindings(response string) []*Finding {
	var findings []*Finding

	// Find all [FINDING]...[/FINDING] blocks
	findingPattern := regexp.MustCompile(`(?s)\[FINDING\](.*?)\[/FINDING\]`)
	matches := findingPattern.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		block := match[1]
		finding := p.parseFindingBlock(block)
		if finding != nil {
			findings = append(findings, finding)
		}
	}

	return findings
}

// parseFindingBlock extracts a single finding from a block
func (p *PatrolService) parseFindingBlock(block string) *Finding {
	extract := func(key string) string {
		pattern := regexp.MustCompile(`(?i)` + key + `:\s*(.+?)(?:\n|$)`)
		match := pattern.FindStringSubmatch(block)
		if len(match) >= 2 {
			return strings.TrimSpace(match[1])
		}
		return ""
	}

	severity := extract("SEVERITY")
	category := extract("CATEGORY")
	key := extract("KEY")
	if key == "" {
		key = extract("FINDING_KEY")
	}
	resource := extract("RESOURCE")
	resourceType := extract("RESOURCE_TYPE")
	title := extract("TITLE")
	description := extract("DESCRIPTION")
	recommendation := extract("RECOMMENDATION")
	evidence := extract("EVIDENCE")

	// Validate required fields
	if title == "" || description == "" {
		return nil
	}

	// Map severity
	var sev FindingSeverity
	switch strings.ToLower(severity) {
	case "critical":
		sev = FindingSeverityCritical
	case "warning":
		sev = FindingSeverityWarning
	case "watch":
		sev = FindingSeverityWatch
	default:
		sev = FindingSeverityInfo
	}

	// Map category
	var cat FindingCategory
	switch strings.ToLower(category) {
	case "performance":
		cat = FindingCategoryPerformance
	case "reliability":
		cat = FindingCategoryReliability
	case "security":
		cat = FindingCategorySecurity
	case "capacity":
		cat = FindingCategoryCapacity
	case "configuration":
		cat = FindingCategoryGeneral // Configuration maps to General
	default:
		cat = FindingCategoryPerformance
	}

	// Generate stable ID from resource and category ONLY (not title)
	// This ensures the same issue on the same resource gets the same ID even if
	// the LLM phrases it differently each time
	id := generateFindingID(resource, string(cat), "llm-finding")

	return &Finding{
		ID:             id,
		Key:            normalizeFindingKey(key),
		Severity:       sev,
		Category:       cat,
		ResourceID:     resource,
		ResourceName:   resource,
		ResourceType:   resourceType,
		Title:          title,
		Description:    description,
		Recommendation: recommendation,
		Evidence:       evidence,
		Source:         "ai-analysis", // Mark as coming from AI
	}
}

func normalizeFindingKey(key string) string {
	if key == "" {
		return ""
	}
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return ""
	}
	key = strings.ReplaceAll(key, "_", "-")
	key = strings.ReplaceAll(key, " ", "-")
	var b strings.Builder
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}

// formatDurationPatrol formats a duration as a human-readable string for patrol
func formatDurationPatrol(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// formatBytes formats bytes as a human-readable string
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// formatBytesInt64 formats int64 bytes as a human-readable string
func formatBytesInt64(b int64) string {
	if b < 0 {
		return "0 B"
	}
	return formatBytes(uint64(b))
}
