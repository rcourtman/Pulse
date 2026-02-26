// patrol_init.go contains PatrolService configuration types, default constructors,
// threshold calculation, and all setter/getter methods for dependency injection.
package ai

import (
	"context"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

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
// This is the default mode which now uses EXACT thresholds (matching user configuration).
// For proactive/early warning mode, use CalculatePatrolThresholdsWithMode(provider, true).
func CalculatePatrolThresholds(provider ThresholdProvider) PatrolThresholds {
	return CalculatePatrolThresholdsWithMode(provider, false)
}

// CalculatePatrolThresholdsWithMode derives patrol thresholds from alert thresholds
// When proactiveMode is false (default): uses exact alert thresholds
// When proactiveMode is true: warns 5-15% BEFORE alerts fire for early warning
func CalculatePatrolThresholdsWithMode(provider ThresholdProvider, proactiveMode bool) PatrolThresholds {
	if provider == nil {
		return DefaultPatrolThresholds()
	}

	// Get user's alert thresholds
	nodeCPU := provider.GetNodeCPUThreshold()
	nodeMem := provider.GetNodeMemoryThreshold()
	guestMem := provider.GetGuestMemoryThreshold()
	guestDisk := provider.GetGuestDiskThreshold()
	storage := provider.GetStorageThreshold()

	if proactiveMode {
		// Proactive mode: warn BEFORE thresholds are reached
		// watch = alert-15%, warning = alert-5%
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

	// Exact mode (default): use exact alert thresholds
	// Watch is slightly below warning, warning is at threshold
	return PatrolThresholds{
		NodeCPUWatch:    clampThreshold(nodeCPU - 5), // Watch slightly before threshold
		NodeCPUWarning:  nodeCPU,                     // Warning at exact threshold
		NodeMemWatch:    clampThreshold(nodeMem - 5),
		NodeMemWarning:  nodeMem,
		GuestMemWatch:   clampThreshold(guestMem - 5),
		GuestMemWarning: guestMem,
		GuestDiskWatch:  clampThreshold(guestDisk - 5),
		GuestDiskWarn:   guestDisk,
		GuestDiskCrit:   guestDisk + 5, // Critical slightly above threshold
		StorageWatch:    clampThreshold(storage - 5),
		StorageWarning:  storage,
		StorageCritical: storage + 5,
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
	// AnalyzeKubernetes controls whether to analyze Kubernetes clusters
	AnalyzeKubernetes bool `json:"analyze_kubernetes"`
	// AnalyzePMG controls whether to analyze Proxmox Mail Gateway instances
	AnalyzePMG bool `json:"analyze_pmg"`
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
		Enabled:           true,
		Interval:          15 * time.Minute,
		AnalyzeNodes:      true,
		AnalyzeGuests:     true,
		AnalyzeDocker:     true,
		AnalyzeStorage:    true,
		AnalyzePBS:        true,
		AnalyzeHosts:      true,
		AnalyzeKubernetes: true,
		AnalyzePMG:        true,
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

// SetAlertResolver sets the alert resolver for AI-based alert management.
// This allows patrol to review and auto-resolve alerts when issues are fixed.
func (p *PatrolService) SetAlertResolver(resolver AlertResolver) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.alertResolver = resolver
	log.Info().Msg("AI Patrol: Alert resolver configured for autonomous alert management")
}

// GetAlertResolver returns the alert resolver if configured.
func (p *PatrolService) GetAlertResolver() AlertResolver {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.alertResolver
}

// SetCircuitBreaker sets the circuit breaker for resilient AI API calls.
// When set, AI calls during patrol will be protected by the circuit breaker.
func (p *PatrolService) SetCircuitBreaker(breaker *circuit.Breaker) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.circuitBreaker = breaker
	log.Info().Msg("circuit breaker configured for patrol")
}

// SetRemediationEngine sets the remediation engine for generating fix plans from findings
func (p *PatrolService) SetRemediationEngine(engine *remediation.Engine) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.remediationEngine = engine
	log.Info().Msg("remediation engine configured for patrol")
}

// GetRemediationEngine returns the remediation engine
func (p *PatrolService) GetRemediationEngine() *remediation.Engine {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.remediationEngine
}

// SetInvestigationOrchestrator sets the investigation orchestrator for autonomous finding investigation
func (p *PatrolService) SetInvestigationOrchestrator(orchestrator InvestigationOrchestrator) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.investigationOrchestrator = orchestrator
	log.Info().Msg("investigation orchestrator configured for patrol")
}

// GetInvestigationOrchestrator returns the investigation orchestrator
func (p *PatrolService) GetInvestigationOrchestrator() InvestigationOrchestrator {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.investigationOrchestrator
}

// SetPushNotifyCallback sets the callback for sending push notifications via relay.
func (p *PatrolService) SetPushNotifyCallback(cb PushNotifyCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pushNotifyCallback = cb
}

// SetUnifiedFindingCallback sets the callback for pushing findings to the unified store
// When set, it also syncs existing active findings to the unified store
func (p *PatrolService) SetUnifiedFindingCallback(cb UnifiedFindingCallback) {
	p.mu.Lock()
	p.unifiedFindingCallback = cb
	findings := p.findings
	p.mu.Unlock()

	// Sync existing active findings to unified store
	if cb != nil && findings != nil {
		activeFindings := findings.GetActive(FindingSeverityInfo)
		synced := 0
		for _, f := range activeFindings {
			if cb(f) {
				synced++
			}
		}
		log.Info().
			Int("synced", synced).
			Int("total", len(activeFindings)).
			Msg("Unified finding callback configured and existing findings synced")
	} else {
		log.Info().Msg("unified finding callback configured for patrol")
	}
}

// SetUnifiedFindingResolver sets the callback for marking findings resolved in the unified store.
func (p *PatrolService) SetUnifiedFindingResolver(cb func(findingID string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.unifiedFindingResolver = cb
	if cb != nil {
		log.Info().Msg("unified finding resolver configured for patrol")
	}
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
// This allows patrol to use user-configured thresholds for alerting
func (p *PatrolService) SetThresholdProvider(provider ThresholdProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.thresholdProvider = provider
	p.thresholds = CalculatePatrolThresholdsWithMode(provider, p.proactiveMode)
	log.Debug().
		Float64("storageWatch", p.thresholds.StorageWatch).
		Float64("storageWarning", p.thresholds.StorageWarning).
		Float64("storageCritical", p.thresholds.StorageCritical).
		Bool("proactiveMode", p.proactiveMode).
		Msg("Patrol thresholds updated from alert config")
}

// SetProactiveMode configures whether patrol warns before thresholds (true) or at exact thresholds (false)
func (p *PatrolService) SetProactiveMode(proactive bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.proactiveMode == proactive {
		return // No change
	}
	p.proactiveMode = proactive
	// Recalculate thresholds with new mode
	p.thresholds = CalculatePatrolThresholdsWithMode(p.thresholdProvider, proactive)
	log.Info().
		Bool("proactiveMode", proactive).
		Float64("storageWarning", p.thresholds.StorageWarning).
		Msg("Patrol mode updated")
}

// GetProactiveMode returns whether proactive threshold mode is enabled
func (p *PatrolService) GetProactiveMode() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.proactiveMode
}

// GetThresholds returns the current patrol thresholds (for display in UI)
func (p *PatrolService) GetThresholds() PatrolThresholds {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.thresholds
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

// GetKnowledgeStore returns the knowledge store for external wiring
func (p *PatrolService) GetKnowledgeStore() *knowledge.Store {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.knowledgeStore
}

// SetUnifiedResourceProvider sets the unified resource provider for reading
// physical disks, Ceph clusters, etc. from the canonical resource registry.
func (p *PatrolService) SetUnifiedResourceProvider(urp UnifiedResourceProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.unifiedResourceProvider = urp
	log.Info().Msg("AI Patrol: Unified resource provider set")
}

// SetReadState sets the typed ReadState view provider for resource iteration.
// When nil, patrol will fall back to legacy models.StateSnapshot field iteration.
func (p *PatrolService) SetReadState(rs unifiedresources.ReadState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.readState = rs
	if rs != nil {
		log.Info().Msg("AI Patrol: ReadState configured")
	} else {
		log.Info().Msg("AI Patrol: ReadState cleared")
	}
}

// SetDiscoveryStore sets the discovery store for infrastructure context
// This enables the patrol service to include discovered service info in prompts
func (p *PatrolService) SetDiscoveryStore(store *servicediscovery.Store) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.discoveryStore = store
	log.Info().Msg("AI Patrol: Discovery store set for infrastructure context")
}

// GetDiscoveryStore returns the discovery store for external access
func (p *PatrolService) GetDiscoveryStore() *servicediscovery.Store {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.discoveryStore
}

// SetGuestProber sets the guest prober for pre-patrol reachability checks.
// This enables the patrol service to ping guests via connected host agents.
func (p *PatrolService) SetGuestProber(prober GuestProber) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.guestProber = prober
	log.Info().Msg("AI Patrol: Guest prober set for reachability checks")
}

// GetGuestProber returns the guest prober for external access.
func (p *PatrolService) GetGuestProber() GuestProber {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.guestProber
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

// SetLearningProvider sets the learning provider for user feedback context
func (p *PatrolService) SetLearningProvider(provider LearningProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.learningProvider = provider
	log.Info().Msg("AI Patrol: Learning provider set for user preference context")
}

// SetProxmoxEventProvider sets the Proxmox event provider for operations context
func (p *PatrolService) SetProxmoxEventProvider(provider ProxmoxEventProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.proxmoxEventProvider = provider
	log.Info().Msg("AI Patrol: Proxmox event provider set for operations context")
}

// SetForecastProvider sets the forecast provider for trend predictions
func (p *PatrolService) SetForecastProvider(provider ForecastProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.forecastProvider = provider
	log.Info().Msg("AI Patrol: Forecast provider set for trend predictions")
}

// SetTriggerManager sets the event-driven trigger manager for patrol scheduling.
// When set, the trigger manager handles event-driven patrol execution alongside
// the scheduled patrol loop.
func (p *PatrolService) SetTriggerManager(tm *TriggerManager) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.triggerManager = tm
	if tm != nil {
		log.Info().Msg("AI Patrol: Trigger manager set for event-driven patrol")
	}
}

// GetTriggerManager returns the trigger manager
func (p *PatrolService) GetTriggerManager() *TriggerManager {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.triggerManager
}

// SetEventTriggersEnabled controls whether event-driven patrol triggers (alert_fired, alert_cleared, anomaly)
// are accepted. Propagates the setting to both PatrolService and the TriggerManager.
func (p *PatrolService) SetEventTriggersEnabled(enabled bool) {
	p.mu.Lock()
	p.eventTriggersEnabled = enabled
	tm := p.triggerManager
	p.mu.Unlock()
	if tm != nil {
		tm.SetEventTriggersEnabled(enabled)
	}
}

// SetQuickstartCredits sets the quickstart credit manager for free hosted patrol runs.
func (p *PatrolService) SetQuickstartCredits(mgr QuickstartCreditManager) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quickstartCredits = mgr
	if mgr != nil {
		log.Info().Int("remaining", mgr.CreditsRemaining()).Msg("AI Patrol: Quickstart credit manager configured")
	}
}

// GetQuickstartCredits returns the quickstart credit manager.
func (p *PatrolService) GetQuickstartCredits() QuickstartCreditManager {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.quickstartCredits
}

// TriggerScopedPatrol runs a targeted patrol for specific resources.
// This is called by the TriggerManager for event-driven patrols.
// When ResourceIDs or ResourceTypes are specified in the scope, only those resources
// are analyzed, reducing token usage and providing faster, more focused analysis.
func (p *PatrolService) TriggerScopedPatrol(ctx context.Context, scope PatrolScope) {
	p.mu.RLock()
	cfg := p.config
	p.mu.RUnlock()

	if !cfg.Enabled {
		log.Debug().Msg("AI Patrol: Scoped patrol skipped - patrol disabled")
		return
	}

	// Filter out empty resource IDs to prevent accidentally matching all resources
	filteredIDs := make([]string, 0, len(scope.ResourceIDs))
	for _, id := range scope.ResourceIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			filteredIDs = append(filteredIDs, trimmed)
		}
	}
	scope.ResourceIDs = filteredIDs

	// Filter out empty resource types
	filteredTypes := make([]string, 0, len(scope.ResourceTypes))
	for _, t := range scope.ResourceTypes {
		if trimmed := strings.TrimSpace(t); trimmed != "" {
			filteredTypes = append(filteredTypes, trimmed)
		}
	}
	scope.ResourceTypes = filteredTypes

	// If no valid scope after filtering, skip - scheduled patrols provide full coverage
	if len(scope.ResourceIDs) == 0 && len(scope.ResourceTypes) == 0 {
		log.Debug().
			Str("reason", string(scope.Reason)).
			Msg("AI Patrol: Scoped patrol skipped - no valid resource IDs or types after filtering")
		return
	}

	scope = p.addDiscoveryScopeHint(scope)

	// Log the scoped patrol
	log.Info().
		Str("reason", string(scope.Reason)).
		Strs("resources", scope.ResourceIDs).
		Strs("types", scope.ResourceTypes).
		Str("depth", scope.Depth.String()).
		Str("context", scope.Context).
		Msg("AI Patrol: Running scoped patrol")

	// Run scoped patrol with filtered resources
	p.runScopedPatrol(ctx, scope)
}

func (p *PatrolService) addDiscoveryScopeHint(scope PatrolScope) PatrolScope {
	if len(scope.ResourceIDs) == 0 {
		return scope
	}

	if strings.Contains(strings.ToLower(scope.Context), "discovery:") {
		return scope
	}

	p.mu.RLock()
	discoveryStore := p.discoveryStore
	p.mu.RUnlock()

	if discoveryStore == nil {
		return scope
	}

	discoveries, err := discoveryStore.List()
	if err != nil || len(discoveries) == 0 {
		if err != nil {
			log.Debug().Err(err).Msg("AI Patrol: Failed to load discovery data for scope hints")
		}
		return scope
	}

	filtered := servicediscovery.FilterDiscoveriesByResourceIDs(discoveries, scope.ResourceIDs)
	hint := servicediscovery.FormatScopeHint(filtered)
	if hint == "" {
		return scope
	}
	if strings.Contains(scope.Context, hint) {
		return scope
	}

	const maxScopeContextLen = 240
	if scope.Context == "" {
		scope.Context = truncateScopeContext(hint, maxScopeContextLen)
		return scope
	}
	if len(scope.Context) >= maxScopeContextLen {
		return scope
	}

	scope.Context = truncateScopeContext(scope.Context+" | "+hint, maxScopeContextLen)

	return scope
}

func truncateScopeContext(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
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
