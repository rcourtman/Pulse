package ai

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	aicontext "github.com/rcourtman/pulse-go-rewrite/internal/ai/context"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/aidiscovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
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

// AlertResolver provides the ability to review and resolve alerts
type AlertResolver interface {
	// GetActiveAlerts returns all currently active alerts
	GetActiveAlerts() []AlertInfo
	// ResolveAlert clears an active alert, returns true if successful
	ResolveAlert(alertID string) bool
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
	// AnalyzeKubernetes controls whether to analyze Kubernetes clusters
	AnalyzeKubernetes bool `json:"analyze_kubernetes"`
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
	Duration         time.Duration `json:"-"`
	DurationMs       int64         `json:"duration_ms"`
	Type             string        `json:"type"` // Always "patrol" now (kept for backwards compat)
	ResourcesChecked int           `json:"resources_checked"`
	// Breakdown by resource type
	NodesChecked      int `json:"nodes_checked"`
	GuestsChecked     int `json:"guests_checked"`
	DockerChecked     int `json:"docker_checked"`
	StorageChecked    int `json:"storage_checked"`
	HostsChecked      int `json:"hosts_checked"`
	PBSChecked        int `json:"pbs_checked"`
	KubernetesChecked int `json:"kubernetes_checked"`
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

// LearningProvider provides learned preferences for patrol context
type LearningProvider interface {
	// FormatForContext returns learned preferences formatted for AI prompt injection
	FormatForContext() string
}

// ProxmoxEventProvider provides recent Proxmox events for patrol context
type ProxmoxEventProvider interface {
	// FormatForPatrol formats recent events for AI patrol context
	FormatForPatrol(duration time.Duration) string
}

// ForecastProvider provides trend forecasts for patrol context
type ForecastProvider interface {
	// FormatKeyForecasts returns formatted forecasts for resources with concerning trends
	FormatKeyForecasts() string
}

// UnifiedFindingCallback is called when patrol creates a new finding
// It allows the unified store to receive patrol findings in addition to alerts
type UnifiedFindingCallback func(f *Finding) bool

// InvestigationOrchestrator defines the interface for autonomous investigation of findings
type InvestigationOrchestrator interface {
	// InvestigateFinding starts an investigation for a finding
	InvestigateFinding(ctx context.Context, finding *InvestigationFinding, autonomyLevel string) error
	// GetInvestigationByFinding returns the latest investigation for a finding
	GetInvestigationByFinding(findingID string) *InvestigationSession
	// GetRunningCount returns the number of running investigations
	GetRunningCount() int
	// GetFixedCount returns the number of issues auto-fixed by Patrol
	GetFixedCount() int
	// CanStartInvestigation returns true if a new investigation can be started
	CanStartInvestigation() bool
	// ReinvestigateFinding triggers a re-investigation of a finding
	ReinvestigateFinding(ctx context.Context, findingID, autonomyLevel string) error
}

// InvestigationFinding is the finding type expected by the orchestrator
type InvestigationFinding struct {
	ID                     string
	Severity               string
	Category               string
	ResourceID             string
	ResourceName           string
	ResourceType           string
	Title                  string
	Description            string
	Recommendation         string
	Evidence               string
	InvestigationSessionID string
	InvestigationStatus    string
	InvestigationOutcome   string
	LastInvestigatedAt     *time.Time
	InvestigationAttempts  int
}

// InvestigationSession represents the result of an investigation (minimal interface)
type InvestigationSession struct {
	ID          string            `json:"id"`
	FindingID   string            `json:"finding_id"`
	SessionID   string            `json:"session_id"`
	Status      string            `json:"status"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	TurnCount   int               `json:"turn_count"`
	Outcome     string            `json:"outcome,omitempty"`
	Summary     string            `json:"summary,omitempty"`
	Error       string            `json:"error,omitempty"`
	ProposedFix *InvestigationFix `json:"proposed_fix,omitempty"`
	ApprovalID  string            `json:"approval_id,omitempty"`
}

// InvestigationFix represents a proposed remediation action
type InvestigationFix struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Commands    []string `json:"commands,omitempty"`
	RiskLevel   string   `json:"risk_level,omitempty"`
	Destructive bool     `json:"destructive"`
	TargetHost  string   `json:"target_host,omitempty"`
	Rationale   string   `json:"rationale,omitempty"`
}

// PatrolService runs background AI analysis of infrastructure
type PatrolService struct {
	mu sync.RWMutex

	aiService           *Service
	stateProvider       StateProvider
	thresholdProvider   ThresholdProvider
	config              PatrolConfig
	findings            *FindingsStore
	knowledgeStore      *knowledge.Store       // For per-resource notes in patrol context
	discoveryStore      *aidiscovery.Store     // For AI-discovered infrastructure context
	metricsHistory      MetricsHistoryProvider // For trend analysis and predictions
	baselineStore       *baseline.Store        // For anomaly detection via learned baselines
	changeDetector      *ChangeDetector        // For tracking infrastructure changes
	remediationLog      *RemediationLog        // For tracking remediation actions
	patternDetector     *PatternDetector       // For failure prediction from historical patterns
	correlationDetector *CorrelationDetector   // For multi-resource correlation
	incidentStore       *memory.IncidentStore  // For incident timeline capture
	alertResolver       AlertResolver          // For AI-based alert resolution

	// New AI intelligence providers (Phase 6)
	learningProvider     LearningProvider     // For learned preferences from user feedback
	proxmoxEventProvider ProxmoxEventProvider // For recent Proxmox operations
	forecastProvider     ForecastProvider     // For trend forecasts

	// Event-driven patrol triggers (Phase 7)
	triggerManager *TriggerManager // For event-driven patrol scheduling

	// Unified intelligence facade - aggregates all subsystems for unified view
	intelligence *Intelligence

	// Circuit breaker for resilient AI API calls
	circuitBreaker *circuit.Breaker

	// Remediation engine for generating fix plans from findings
	remediationEngine *remediation.Engine

	// Investigation orchestrator for autonomous investigation of findings
	investigationOrchestrator InvestigationOrchestrator

	// Unified findings callback - pushes findings to unified store
	unifiedFindingCallback UnifiedFindingCallback

	// Cached thresholds (recalculated when thresholdProvider changes)
	thresholds    PatrolThresholds
	proactiveMode bool // When true, warn before thresholds; when false, use exact thresholds

	// Runtime state
	running          bool
	runInProgress    bool
	stopCh           chan struct{}
	configChanged    chan struct{} // Signal when config changes to reset ticker
	lastPatrol       time.Time
	lastDuration     time.Duration
	resourcesChecked int
	errorCount       int

	// Patrol run history with persistence support
	runHistoryStore *PatrolRunHistoryStore

	// Ad-hoc trigger channel for event-driven patrols (alert driven)
	adHocTrigger chan *alerts.Alert

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
		adHocTrigger:      make(chan *alerts.Alert, 10), // Buffer triggers
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
	log.Info().Msg("Circuit breaker configured for patrol")
}

// SetRemediationEngine sets the remediation engine for generating fix plans from findings
func (p *PatrolService) SetRemediationEngine(engine *remediation.Engine) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.remediationEngine = engine
	log.Info().Msg("Remediation engine configured for patrol")
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
	log.Info().Msg("Investigation orchestrator configured for patrol")
}

// GetInvestigationOrchestrator returns the investigation orchestrator
func (p *PatrolService) GetInvestigationOrchestrator() InvestigationOrchestrator {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.investigationOrchestrator
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
		log.Info().Msg("Unified finding callback configured for patrol")
	}
}

// generateRemediationPlan creates a remediation plan for a finding if appropriate.
// Only generates plans for critical/warning findings when a remediation engine is configured.
func (p *PatrolService) generateRemediationPlan(finding *Finding) {
	p.mu.RLock()
	engine := p.remediationEngine
	p.mu.RUnlock()

	if engine == nil {
		return
	}

	// Only generate plans for actionable findings
	if finding.Severity != FindingSeverityCritical && finding.Severity != FindingSeverityWarning {
		return
	}

	// Generate remediation steps based on finding category and resource type
	steps := p.generateRemediationSteps(finding)
	if len(steps) == 0 {
		return
	}

	// Determine risk level based on finding severity and category
	riskLevel := remediation.RiskLow
	if finding.Severity == FindingSeverityWarning {
		riskLevel = remediation.RiskMedium
	}
	if finding.Severity == FindingSeverityCritical {
		riskLevel = remediation.RiskHigh
	}
	// Reliability issues involving restarts/reboots are higher risk
	if finding.Category == FindingCategoryReliability {
		title := strings.ToLower(finding.Title)
		if strings.Contains(title, "restart") || strings.Contains(title, "reboot") || strings.Contains(title, "offline") {
			if riskLevel < remediation.RiskHigh {
				riskLevel = remediation.RiskHigh
			}
		} else if riskLevel < remediation.RiskMedium {
			riskLevel = remediation.RiskMedium
		}
	}

	// Create the remediation plan
	plan := &remediation.RemediationPlan{
		FindingID:   finding.ID,
		ResourceID:  finding.ResourceID,
		Title:       fmt.Sprintf("Fix: %s", finding.Title),
		Description: finding.Description,
		Category:    remediation.CategoryGuided, // All auto-generated plans require user approval
		RiskLevel:   riskLevel,
		Steps:       steps,
		Rationale:   finding.Recommendation,
	}

	// Add warnings based on risk level
	if riskLevel == remediation.RiskHigh {
		plan.Warnings = append(plan.Warnings, "High risk: This action may cause service disruption. Review carefully and consider scheduling during maintenance window.")
	} else if riskLevel == remediation.RiskMedium {
		plan.Warnings = append(plan.Warnings, "Review steps carefully before execution")
	}

	if err := engine.CreatePlan(plan); err != nil {
		log.Debug().
			Err(err).
			Str("finding_id", finding.ID).
			Str("resource", finding.ResourceName).
			Msg("AI Patrol: Failed to create remediation plan")
		return
	}

	log.Info().
		Str("plan_id", plan.ID).
		Str("finding_id", finding.ID).
		Str("resource", finding.ResourceName).
		Int("steps", len(steps)).
		Msg("AI Patrol: Remediation plan generated")
}

// generateRemediationSteps creates appropriate steps based on finding type
func (p *PatrolService) generateRemediationSteps(finding *Finding) []remediation.RemediationStep {
	var steps []remediation.RemediationStep

	switch finding.Category {
	case FindingCategoryPerformance:
		steps = p.generatePerformanceSteps(finding)
	case FindingCategoryCapacity:
		steps = p.generateCapacitySteps(finding)
	case FindingCategoryReliability:
		steps = p.generateAvailabilitySteps(finding)
	case FindingCategoryBackup:
		steps = p.generateBackupSteps(finding)
	case FindingCategorySecurity:
		steps = p.generateSecuritySteps(finding)
	case FindingCategoryGeneral:
		steps = p.generateConfigurationSteps(finding)
	default:
		// Generic investigation steps for unknown categories
		steps = []remediation.RemediationStep{
			{Order: 1, Description: "Investigate the issue by reviewing current resource state"},
			{Order: 2, Description: "Review recent changes that may have caused this issue"},
			{Order: 3, Description: "Take appropriate corrective action based on findings"},
		}
	}

	return steps
}

// generatePerformanceSteps creates steps for performance issues
func (p *PatrolService) generatePerformanceSteps(finding *Finding) []remediation.RemediationStep {
	title := strings.ToLower(finding.Title)

	if strings.Contains(title, "cpu") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Identify processes consuming excessive CPU", Target: finding.ResourceID},
			{Order: 2, Description: "Check if resource needs more CPU cores allocated"},
			{Order: 3, Description: "Consider migrating to a less loaded host if VM/container"},
			{Order: 4, Description: "Optimize or restart resource-hungry applications"},
		}
	}

	if strings.Contains(title, "memory") || strings.Contains(title, "ram") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Identify processes consuming excessive memory", Target: finding.ResourceID},
			{Order: 2, Description: "Check for memory leaks in running applications"},
			{Order: 3, Description: "Consider increasing allocated memory"},
			{Order: 4, Description: "Restart affected services to reclaim memory"},
		}
	}

	if strings.Contains(title, "io") || strings.Contains(title, "disk") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Identify processes causing high disk I/O", Target: finding.ResourceID},
			{Order: 2, Description: "Check for runaway log files or heavy writes"},
			{Order: 3, Description: "Consider migrating to faster storage"},
		}
	}

	// Generic performance steps
	return []remediation.RemediationStep{
		{Order: 1, Description: "Review current resource utilization metrics", Target: finding.ResourceID},
		{Order: 2, Description: "Identify performance bottlenecks"},
		{Order: 3, Description: "Optimize resource allocation or application configuration"},
	}
}

// generateCapacitySteps creates steps for capacity issues
func (p *PatrolService) generateCapacitySteps(finding *Finding) []remediation.RemediationStep {
	title := strings.ToLower(finding.Title)

	if strings.Contains(title, "disk") || strings.Contains(title, "storage") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Identify largest files and directories consuming space", Target: finding.ResourceID},
			{Order: 2, Description: "Clean up temporary files, logs, and caches"},
			{Order: 3, Description: "Remove unused packages and old kernels"},
			{Order: 4, Description: "Consider expanding disk or adding additional storage"},
		}
	}

	if strings.Contains(title, "memory") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Review memory allocation across workloads", Target: finding.ResourceID},
			{Order: 2, Description: "Reduce memory allocation on over-provisioned VMs"},
			{Order: 3, Description: "Add more physical memory to the host"},
		}
	}

	// Generic capacity steps
	return []remediation.RemediationStep{
		{Order: 1, Description: "Review current capacity utilization", Target: finding.ResourceID},
		{Order: 2, Description: "Identify growth trends and plan for expansion"},
		{Order: 3, Description: "Clean up unused resources to free capacity"},
	}
}

// generateAvailabilitySteps creates steps for availability issues
func (p *PatrolService) generateAvailabilitySteps(finding *Finding) []remediation.RemediationStep {
	title := strings.ToLower(finding.Title)

	if strings.Contains(title, "offline") || strings.Contains(title, "down") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Verify network connectivity to the resource", Target: finding.ResourceID},
			{Order: 2, Description: "Check host status if this is a VM/container"},
			{Order: 3, Description: "Review system logs for crash or shutdown reasons"},
			{Order: 4, Description: "Attempt to start or restart the resource"},
		}
	}

	if strings.Contains(title, "restart") || strings.Contains(title, "reboot") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Review system logs for cause of restarts", Target: finding.ResourceID},
			{Order: 2, Description: "Check for OOM kills or kernel panics"},
			{Order: 3, Description: "Investigate application crashes"},
			{Order: 4, Description: "Consider enabling watchdog or health checks"},
		}
	}

	// Generic availability steps
	return []remediation.RemediationStep{
		{Order: 1, Description: "Verify resource health and connectivity", Target: finding.ResourceID},
		{Order: 2, Description: "Review recent events and logs"},
		{Order: 3, Description: "Take corrective action to restore availability"},
	}
}

// generateBackupSteps creates steps for backup-related issues
func (p *PatrolService) generateBackupSteps(finding *Finding) []remediation.RemediationStep {
	title := strings.ToLower(finding.Title)

	if strings.Contains(title, "missing") || strings.Contains(title, "no backup") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Verify backup job configuration exists", Target: finding.ResourceID},
			{Order: 2, Description: "Check backup storage availability and capacity"},
			{Order: 3, Description: "Create or enable backup schedule"},
			{Order: 4, Description: "Run initial backup job"},
		}
	}

	if strings.Contains(title, "failed") || strings.Contains(title, "error") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Review backup job logs for error details", Target: finding.ResourceID},
			{Order: 2, Description: "Check backup storage connectivity and space"},
			{Order: 3, Description: "Verify backup credentials and permissions"},
			{Order: 4, Description: "Retry backup job after fixing issues"},
		}
	}

	if strings.Contains(title, "old") || strings.Contains(title, "stale") || strings.Contains(title, "outdated") {
		return []remediation.RemediationStep{
			{Order: 1, Description: "Check why scheduled backups are not running", Target: finding.ResourceID},
			{Order: 2, Description: "Review backup retention policy"},
			{Order: 3, Description: "Trigger a new backup immediately"},
		}
	}

	// Generic backup steps
	return []remediation.RemediationStep{
		{Order: 1, Description: "Review backup configuration and schedule", Target: finding.ResourceID},
		{Order: 2, Description: "Verify backup storage health"},
		{Order: 3, Description: "Ensure backup jobs are running successfully"},
	}
}

// generateConfigurationSteps creates steps for configuration issues
func (p *PatrolService) generateConfigurationSteps(finding *Finding) []remediation.RemediationStep {
	return []remediation.RemediationStep{
		{Order: 1, Description: "Review current configuration settings", Target: finding.ResourceID},
		{Order: 2, Description: "Compare against recommended best practices"},
		{Order: 3, Description: "Apply configuration changes as needed"},
		{Order: 4, Description: "Verify changes don't impact dependent services"},
	}
}

// generateSecuritySteps creates steps for security issues
func (p *PatrolService) generateSecuritySteps(finding *Finding) []remediation.RemediationStep {
	return []remediation.RemediationStep{
		{Order: 1, Description: "Assess the security impact and urgency", Target: finding.ResourceID},
		{Order: 2, Description: "Review access logs for suspicious activity"},
		{Order: 3, Description: "Apply security patches or configuration fixes"},
		{Order: 4, Description: "Verify remediation and update security policies"},
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

// SetDiscoveryStore sets the AI discovery store for infrastructure context
// This enables the patrol service to include discovered service info in prompts
func (p *PatrolService) SetDiscoveryStore(store *aidiscovery.Store) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.discoveryStore = store
	log.Info().Msg("AI Patrol: Discovery store set for infrastructure context")
}

// GetDiscoveryStore returns the discovery store for external access
func (p *PatrolService) GetDiscoveryStore() *aidiscovery.Store {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.discoveryStore
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

	// Log the scoped patrol
	log.Info().
		Str("reason", string(scope.Reason)).
		Strs("resources", scope.ResourceIDs).
		Strs("types", scope.ResourceTypes).
		Str("depth", scope.Depth.String()).
		Str("context", scope.Context).
		Msg("AI Patrol: Running scoped patrol")

	// If no specific resources or types are specified, run full patrol
	if len(scope.ResourceIDs) == 0 && len(scope.ResourceTypes) == 0 {
		p.runPatrol(ctx)
		return
	}

	// Run scoped patrol with filtered resources
	p.runScopedPatrol(ctx, scope)
}

// runScopedPatrol runs a patrol on a filtered subset of resources.
// This provides token-efficient analysis for event-driven patrols.
func (p *PatrolService) runScopedPatrol(ctx context.Context, scope PatrolScope) {
	p.mu.RLock()
	cfg := p.config
	breaker := p.circuitBreaker
	p.mu.RUnlock()

	if !cfg.Enabled {
		return
	}

	if !p.tryStartRun("scoped") {
		return
	}
	defer p.endRun()

	// Check if circuit breaker allows LLM calls
	llmAllowed := breaker == nil || breaker.Allow()
	if !llmAllowed {
		log.Warn().Msg("AI Patrol: Circuit breaker is open for scoped patrol, running heuristics only")
	}

	start := time.Now()

	// Get current state
	if p.stateProvider == nil {
		log.Warn().Msg("AI Patrol: No state provider available for scoped patrol")
		return
	}

	fullState := p.stateProvider.GetState()

	// Filter state based on scope
	filteredState := p.filterStateByScope(fullState, scope)

	// Count filtered resources (respect analysis configuration)
	resourceCount := 0
	if cfg.AnalyzeNodes {
		resourceCount += len(filteredState.Nodes)
	}
	if cfg.AnalyzeGuests {
		resourceCount += len(filteredState.VMs) + len(filteredState.Containers)
	}
	if cfg.AnalyzeDocker {
		resourceCount += len(filteredState.DockerHosts)
	}
	if cfg.AnalyzeStorage {
		resourceCount += len(filteredState.Storage)
	}
	if cfg.AnalyzePBS {
		resourceCount += len(filteredState.PBSInstances)
	}
	if cfg.AnalyzeHosts {
		resourceCount += len(filteredState.Hosts)
	}
	if cfg.AnalyzeKubernetes {
		resourceCount += len(filteredState.KubernetesClusters)
	}

	if resourceCount == 0 {
		log.Debug().
			Strs("requested_ids", scope.ResourceIDs).
			Strs("requested_types", scope.ResourceTypes).
			Msg("AI Patrol: No resources matched scope filter")
		return
	}

	log.Debug().
		Int("resource_count", resourceCount).
		Str("reason", string(scope.Reason)).
		Msg("AI Patrol: Running scoped analysis")

	// Determine if we can run LLM analysis
	aiServiceEnabled := p.aiService != nil && p.aiService.IsEnabled()
	hasPatrolFeature := aiServiceEnabled && p.aiService.HasLicenseFeature(FeatureAIPatrol)
	canRunLLM := hasPatrolFeature && llmAllowed

	if !canRunLLM {
		reason := "requires Pulse Pro license for LLM analysis"
		if !aiServiceEnabled {
			reason = "Pulse Assistant service not configured - configure a provider in Settings > Pulse Assistant"
		} else if !llmAllowed {
			reason = "circuit breaker is open"
		}
		log.Debug().Str("reason", reason).Msg("AI Patrol: Running scoped heuristic analysis only")
		// Run heuristics on filtered state only
		for _, f := range p.runHeuristicAnalysis(filteredState) {
			isNew := p.findings.Add(f)
			if isNew && (f.Severity == FindingSeverityWarning || f.Severity == FindingSeverityCritical) {
				log.Info().
					Str("finding_id", f.ID).
					Str("severity", string(f.Severity)).
					Str("resource", f.ResourceName).
					Str("title", f.Title).
					Msg("AI Patrol (scoped): New finding")
				p.generateRemediationPlan(f)
			}
			// Push to unified store if active
			if p.unifiedFindingCallback != nil {
				if stored := p.findings.Get(f.ID); stored != nil && stored.IsActive() {
					p.unifiedFindingCallback(stored)
				}
			}
		}
	} else {
		// Run AI analysis on filtered state
		aiResult, aiErr := p.runAIAnalysis(ctx, filteredState)
		if aiErr != nil {
			log.Warn().Err(aiErr).Msg("AI Patrol (scoped): LLM analysis failed, running heuristics as safety net")
			for _, f := range p.runHeuristicAnalysis(filteredState) {
				isNew := p.findings.Add(f)
				if isNew && (f.Severity == FindingSeverityWarning || f.Severity == FindingSeverityCritical) {
					log.Info().
						Str("finding_id", f.ID).
						Str("severity", string(f.Severity)).
						Str("resource", f.ResourceName).
						Str("title", f.Title).
						Msg("AI Patrol (scoped): New finding")
					p.generateRemediationPlan(f)
				}
				// Push to unified store if active
				if p.unifiedFindingCallback != nil {
					if stored := p.findings.Get(f.ID); stored != nil && stored.IsActive() {
						p.unifiedFindingCallback(stored)
					}
				}
			}
		} else if aiResult != nil {
			if len(aiResult.Findings) == 0 {
				log.Warn().Msg("AI Patrol (scoped): LLM response contained no parseable findings, running heuristics as safety net")
				for _, f := range p.runHeuristicAnalysis(filteredState) {
					isNew := p.findings.Add(f)
					if isNew && (f.Severity == FindingSeverityWarning || f.Severity == FindingSeverityCritical) {
						log.Info().
							Str("finding_id", f.ID).
							Str("severity", string(f.Severity)).
							Str("resource", f.ResourceName).
							Str("title", f.Title).
							Msg("AI Patrol (scoped): New finding")
						p.generateRemediationPlan(f)
					}
					// Push to unified store if active
					if p.unifiedFindingCallback != nil {
						if stored := p.findings.Get(f.ID); stored != nil && stored.IsActive() {
							p.unifiedFindingCallback(stored)
						}
					}
				}
			} else {
				for _, f := range aiResult.Findings {
					isNew := p.findings.Add(f)
					if isNew && (f.Severity == FindingSeverityWarning || f.Severity == FindingSeverityCritical) {
						log.Info().
							Str("finding_id", f.ID).
							Str("severity", string(f.Severity)).
							Str("resource", f.ResourceName).
							Str("title", f.Title).
							Msg("AI Patrol (scoped): New finding")
						if !(f.Key == "ai-patrol-error" || f.ResourceID == "ai-service") {
							p.generateRemediationPlan(f)
						}
					}
					// Push to unified store if active
					if p.unifiedFindingCallback != nil {
						if stored := p.findings.Get(f.ID); stored != nil && stored.IsActive() {
							p.unifiedFindingCallback(stored)
						}
					}
				}
			}
		}
	}

	duration := time.Since(start)
	log.Info().
		Dur("duration", duration).
		Int("resources", resourceCount).
		Str("reason", string(scope.Reason)).
		Msg("AI Patrol: Scoped patrol complete")
}

// filterStateByScope filters a StateSnapshot to only include resources matching the scope.
func (p *PatrolService) filterStateByScope(state models.StateSnapshot, scope PatrolScope) models.StateSnapshot {
	// Build lookup sets for efficient matching
	resourceIDSet := make(map[string]bool)
	for _, id := range scope.ResourceIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		resourceIDSet[trimmed] = true
	}

	typeSet := make(map[string]bool)
	addScopeType := func(t string) {
		trimmed := strings.TrimSpace(strings.ToLower(t))
		if trimmed == "" {
			return
		}
		typeSet[trimmed] = true
		switch trimmed {
		case "docker", "docker_host", "docker_container":
			typeSet["docker"] = true
			typeSet["docker_host"] = true
			typeSet["docker_container"] = true
		case "k8s", "kubernetes", "kubernetes_cluster":
			typeSet["k8s"] = true
			typeSet["kubernetes"] = true
			typeSet["kubernetes_cluster"] = true
		case "lxc", "container":
			typeSet["lxc"] = true
			typeSet["container"] = true
		case "vm", "qemu":
			typeSet["vm"] = true
			typeSet["qemu"] = true
		case "host", "host_raid", "host_sensor":
			typeSet["host"] = true
			typeSet["host_raid"] = true
			typeSet["host_sensor"] = true
		case "pbs", "pbs_datastore", "pbs_job":
			typeSet["pbs"] = true
			typeSet["pbs_datastore"] = true
			typeSet["pbs_job"] = true
		}
	}
	for _, t := range scope.ResourceTypes {
		addScopeType(t)
	}

	hasIDs := len(resourceIDSet) > 0
	hasTypes := len(typeSet) > 0

	matchesType := func(candidates ...string) bool {
		if !hasTypes {
			return true
		}
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			if typeSet[strings.ToLower(candidate)] {
				return true
			}
		}
		return false
	}

	matchesID := func(candidates ...string) bool {
		if !hasIDs {
			return true
		}
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			if resourceIDSet[candidate] {
				return true
			}
		}
		return false
	}

	filtered := models.StateSnapshot{
		LastUpdate:       state.LastUpdate,
		ConnectionHealth: state.ConnectionHealth,
		Stats:            state.Stats,
		ActiveAlerts:     state.ActiveAlerts,
		RecentlyResolved: state.RecentlyResolved,
	}

	// Filter each resource type
	for _, n := range state.Nodes {
		if matchesType("node") && matchesID(n.ID, n.Name) {
			filtered.Nodes = append(filtered.Nodes, n)
		}
	}
	for _, vm := range state.VMs {
		if matchesType("vm", "qemu") && matchesID(vm.ID, vm.Name) {
			filtered.VMs = append(filtered.VMs, vm)
		}
	}
	for _, c := range state.Containers {
		if matchesType("container", "lxc") && matchesID(c.ID, c.Name) {
			filtered.Containers = append(filtered.Containers, c)
		}
	}
	for _, d := range state.DockerHosts {
		if !matchesType("docker", "docker_host", "docker_container") {
			continue
		}

		hostName := d.CustomDisplayName
		if hostName == "" {
			hostName = d.DisplayName
		}
		if hostName == "" {
			hostName = d.Hostname
		}

		hostMatches := matchesID(d.ID, hostName, d.Hostname, d.DisplayName, d.CustomDisplayName)
		if !hasIDs {
			filtered.DockerHosts = append(filtered.DockerHosts, d)
			continue
		}

		var matchedContainers []models.DockerContainer
		for _, c := range d.Containers {
			if matchesID(c.ID, c.Name) {
				matchedContainers = append(matchedContainers, c)
			}
		}

		if hostMatches {
			filtered.DockerHosts = append(filtered.DockerHosts, d)
			continue
		}
		if len(matchedContainers) > 0 {
			hostCopy := d
			hostCopy.Containers = matchedContainers
			filtered.DockerHosts = append(filtered.DockerHosts, hostCopy)
		}
	}
	for _, s := range state.Storage {
		if matchesType("storage") && matchesID(s.ID, s.Name) {
			filtered.Storage = append(filtered.Storage, s)
		}
	}
	for _, pbs := range state.PBSInstances {
		if !matchesType("pbs", "pbs_datastore", "pbs_job") {
			continue
		}

		pbsName := pbs.Name
		if pbsName == "" {
			pbsName = pbs.Host
		}
		pbsMatches := matchesID(pbs.ID, pbs.Name, pbsName, pbs.Host)
		if !hasIDs {
			filtered.PBSInstances = append(filtered.PBSInstances, pbs)
			continue
		}
		if !pbsMatches {
			for _, ds := range pbs.Datastores {
				if matchesID(pbs.ID+":"+ds.Name, ds.Name) {
					pbsMatches = true
					break
				}
			}
		}
		if !pbsMatches {
			for _, job := range pbs.BackupJobs {
				if matchesID(pbs.ID+":job:"+job.ID, job.ID) {
					pbsMatches = true
					break
				}
			}
		}
		if !pbsMatches {
			for _, job := range pbs.VerifyJobs {
				if matchesID(pbs.ID+":verify:"+job.ID, job.ID) {
					pbsMatches = true
					break
				}
			}
		}
		if pbsMatches {
			filtered.PBSInstances = append(filtered.PBSInstances, pbs)
		}
	}
	for _, h := range state.Hosts {
		if matchesType("host", "host_raid", "host_sensor") && matchesID(h.ID, h.DisplayName, h.Hostname) {
			filtered.Hosts = append(filtered.Hosts, h)
		}
	}
	for _, k := range state.KubernetesClusters {
		clusterName := k.CustomDisplayName
		if clusterName == "" {
			clusterName = k.DisplayName
		}
		if clusterName == "" {
			clusterName = k.Name
		}
		if matchesType("kubernetes", "k8s", "kubernetes_cluster") && matchesID(k.ID, clusterName) {
			filtered.KubernetesClusters = append(filtered.KubernetesClusters, k)
		}
	}

	return filtered
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
	analysisInProgress := p.runInProgress

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

	// Calculate next patrol time only when patrol is enabled
	if p.config.Enabled && interval > 0 && !p.lastPatrol.IsZero() {
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
// Subscribers with full channels are automatically removed to prevent memory leaks
func (p *PatrolService) broadcast(event PatrolStreamEvent) {
	p.streamMu.Lock()
	defer p.streamMu.Unlock()

	var staleChannels []chan PatrolStreamEvent
	for ch := range p.streamSubscribers {
		select {
		case ch <- event:
			// Successfully sent
		default:
			// Channel full - mark for removal (likely dead subscriber)
			staleChannels = append(staleChannels, ch)
		}
	}

	// Clean up stale subscribers
	for _, ch := range staleChannels {
		delete(p.streamSubscribers, ch)
		// Close in a goroutine to avoid blocking if receiver is stuck
		go func(c chan PatrolStreamEvent) {
			defer func() { recover() }() // Ignore panic if already closed
			close(c)
		}(ch)
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

		case alert := <-p.adHocTrigger:
			// Run immediate targeted patrol for this alert
			log.Info().Str("alert_id", alert.ID).Msg("Patrol triggered by alert")
			p.runTargetedPatrol(ctx, alert)

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
	breaker := p.circuitBreaker
	p.mu.RUnlock()

	if !cfg.Enabled {
		return
	}

	if !p.tryStartRun("full") {
		return
	}
	defer p.endRun()

	// Check if circuit breaker allows LLM calls
	// We still run heuristics and basic analysis even when breaker is open
	llmAllowed := breaker == nil || breaker.Allow()
	if !llmAllowed {
		log.Warn().Msg("AI Patrol: Circuit breaker is open, running heuristics only (no LLM calls)")
	}

	start := time.Now()
	patrolType := "patrol" // Simplified - no longer distinguishing quick/deep

	log.Debug().Msg("AI Patrol: Starting patrol run")

	// Track run statistics
	var runStats struct {
		resourceCount     int
		nodesChecked      int
		guestsChecked     int
		dockerChecked     int
		storageChecked    int
		hostsChecked      int
		pbsChecked        int
		kubernetesChecked int
		newFindings       int
		existingFindings  int
		findingIDs        []string
		errors            int
		aiAnalysis        *AIAnalysisResult // Stores the AI's analysis for the run record
	}
	var newFindings []*Finding

	// Get current state
	if p.stateProvider == nil {
		log.Warn().Msg("AI Patrol: No state provider available")
		return
	}

	state := p.stateProvider.GetState()

	// Helper to track findings
	// Note: Only warning+ severity findings count toward newFindings since watch/info are filtered from UI
	trackFinding := func(f *Finding) bool {
		isNew := p.findings.Add(f)
		if isNew {
			// Only count warning+ findings as "new" for user-facing stats
			if f.Severity == FindingSeverityWarning || f.Severity == FindingSeverityCritical {
				runStats.newFindings++
				newFindings = append(newFindings, f)
			}
			log.Info().
				Str("finding_id", f.ID).
				Str("severity", string(f.Severity)).
				Str("resource", f.ResourceName).
				Str("title", f.Title).
				Msg("AI Patrol: New finding")

			// Generate remediation plan for actionable findings
			// Skip internal error findings (not actionable by users)
			// Using negated OR: generate only when NEITHER condition matches
			if !(f.Key == "ai-patrol-error" || f.ResourceID == "ai-service") {
				p.generateRemediationPlan(f)
			}
		} else {
			runStats.existingFindings++
		}

		// Push to unified store only if finding is active (not suppressed/dismissed)
		// This ensures last_seen, severity, times_raised stay in sync for active findings
		// but doesn't resurface findings the user has dismissed
		if p.unifiedFindingCallback != nil {
			if stored := p.findings.Get(f.ID); stored != nil && stored.IsActive() {
				p.unifiedFindingCallback(stored)
			}
		}

		// Only track warning+ severity finding IDs in the run record
		if f.Severity == FindingSeverityWarning || f.Severity == FindingSeverityCritical {
			runStats.findingIDs = append(runStats.findingIDs, f.ID)
		}

		// Trigger autonomous investigation if enabled and finding warrants it
		p.MaybeInvestigateFinding(f)

		return isNew
	}

	// Count resources for statistics (respect analysis configuration)
	if cfg.AnalyzeNodes {
		runStats.nodesChecked = len(state.Nodes)
	}
	if cfg.AnalyzeGuests {
		runStats.guestsChecked = len(state.VMs) + len(state.Containers)
	}
	if cfg.AnalyzeDocker {
		runStats.dockerChecked = len(state.DockerHosts)
	}
	if cfg.AnalyzeStorage {
		runStats.storageChecked = len(state.Storage)
	}
	if cfg.AnalyzePBS {
		runStats.pbsChecked = len(state.PBSInstances)
	}
	if cfg.AnalyzeHosts {
		runStats.hostsChecked = len(state.Hosts)
	}
	if cfg.AnalyzeKubernetes {
		runStats.kubernetesChecked = len(state.KubernetesClusters)
	}
	runStats.resourceCount = runStats.nodesChecked + runStats.guestsChecked +
		runStats.dockerChecked + runStats.storageChecked + runStats.pbsChecked + runStats.hostsChecked +
		runStats.kubernetesChecked

	// Determine if we can run LLM analysis (requires AI service + license + circuit breaker not open)
	aiServiceEnabled := p.aiService != nil && p.aiService.IsEnabled()
	hasPatrolFeature := aiServiceEnabled && p.aiService.HasLicenseFeature(FeatureAIPatrol)
	canRunLLM := hasPatrolFeature && llmAllowed

	// Check if we can run LLM analysis (Pro feature + circuit breaker allows)
	if !canRunLLM {
		reason := "requires Pulse Pro license for LLM analysis"
		if !aiServiceEnabled {
			reason = "Pulse Assistant service not configured - configure a provider in Settings > Pulse Assistant"
		} else if !llmAllowed {
			reason = "circuit breaker is open"
		}
		log.Debug().Str("reason", reason).Msg("AI Patrol: Running heuristic analysis only")
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
				title = "Pulse Patrol: Insufficient API credits"
				description = "Pulse Patrol cannot analyze your infrastructure because your provider account has insufficient credits."
				recommendation = "Add credits to your provider account (DeepSeek, OpenAI, etc.) or switch to a different provider in Pulse Assistant settings."
			} else if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "Unauthorized") {
				title = "Pulse Patrol: Invalid API key"
				description = "Pulse Patrol cannot analyze your infrastructure because the API key is invalid or expired."
				recommendation = "Check your API key in Pulse Assistant settings and verify it is correct."
			} else if strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "429") {
				title = "Pulse Patrol: Rate limited"
				description = "Pulse Patrol is being rate limited by your provider. Analysis will be retried on the next patrol run."
				recommendation = "Wait for the rate limit to reset, or consider upgrading your API plan for higher limits."
			} else {
				title = "Pulse Patrol: Analysis failed"
				description = fmt.Sprintf("Pulse Patrol encountered an error while analyzing your infrastructure: %s", errMsg)
				recommendation = "Check your Pulse Assistant settings and API key. If the problem persists, check the logs for more details."
			}

			errorFinding := &Finding{
				ID:             generateFindingID("ai-service", "reliability", "ai-patrol-error"),
				Key:            "ai-patrol-error",
				Severity:       "warning",
				Category:       "reliability",
				ResourceID:     "ai-service",
				ResourceName:   "Pulse Patrol Service",
				ResourceType:   "service",
				Title:          title,
				Description:    description,
				Recommendation: recommendation,
				Evidence:       fmt.Sprintf("Error: %s", errMsg),
				DetectedAt:     time.Now(),
				LastSeenAt:     time.Now(),
			}
			trackFinding(errorFinding)

			// Fall back to heuristic analysis so we still surface local issues
			log.Debug().Msg("AI Patrol: Falling back to heuristic analysis after LLM error")
			for _, f := range p.runHeuristicAnalysis(state) {
				trackFinding(f)
			}
		} else if aiResult != nil {
			runStats.aiAnalysis = aiResult
			if len(aiResult.Findings) == 0 {
				log.Warn().Msg("AI Patrol: LLM response contained no parseable findings, running heuristics as safety net")
				for _, f := range p.runHeuristicAnalysis(state) {
					trackFinding(f)
				}
			} else {
				for _, f := range aiResult.Findings {
					trackFinding(f)
				}
			}
		}
	}

	// Auto-fix with runbooks when enabled (Pro only - requires license)
	var runbookResolved int
	autoFixEnabled := false
	if p.aiService != nil {
		if aiCfg := p.aiService.GetAIConfig(); aiCfg != nil {
			// Auto-fix requires both config flag AND Pro license with ai_autofix feature
			autoFixEnabled = aiCfg.PatrolAutoFix && p.aiService.HasLicenseFeature(FeatureAIAutoFix)
		}
	}
	_ = autoFixEnabled // Auto-fix via runbooks removed - dynamic AI remediation handles this

	// AI-resolve findings: the AI uses tools to query current state and decides what's resolved
	// This is TRUE AI-powered resolution - we provide context, AI provides intelligence
	// Only run AI resolution if AI service is enabled AND circuit breaker allows
	var resolvedCount int
	if aiServiceEnabled && llmAllowed && hasPatrolFeature {
		resolvedCount = p.aiResolveFindings(ctx)
	}

	// Cleanup old resolved findings (always runs, doesn't require LLM)
	cleaned := p.findings.Cleanup(24 * time.Hour)
	if cleaned > 0 {
		log.Debug().Int("cleaned", cleaned).Msg("AI Patrol: Cleaned up old findings")
	}
	resolvedCount += runbookResolved

	// AI-based alert review: check active alerts against current state and auto-resolve fixed issues
	// Pass llmAllowed so it knows whether AI calls are allowed (heuristics always run)
	alertsResolved := p.reviewAndResolveAlerts(ctx, state, llmAllowed)
	if alertsResolved > 0 {
		log.Info().Int("alerts_resolved", alertsResolved).Msg("AI Patrol: Auto-resolved alerts where issues are fixed")
	}

	duration := time.Since(start)
	completedAt := time.Now()

	// Build findings summary string
	summary := p.findings.GetSummary()
	var findingsSummaryStr string
	var status string
	// Only count critical and warning as active issues (watch/info are filtered from UI)
	totalActive := summary.Critical + summary.Warning
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
		ID:                fmt.Sprintf("%d", start.UnixNano()),
		StartedAt:         start,
		CompletedAt:       completedAt,
		Duration:          duration,
		DurationMs:        duration.Milliseconds(),
		Type:              patrolType,
		ResourcesChecked:  runStats.resourceCount,
		NodesChecked:      runStats.nodesChecked,
		GuestsChecked:     runStats.guestsChecked,
		DockerChecked:     runStats.dockerChecked,
		StorageChecked:    runStats.storageChecked,
		HostsChecked:      runStats.hostsChecked,
		PBSChecked:        runStats.pbsChecked,
		KubernetesChecked: runStats.kubernetesChecked,
		NewFindings:       runStats.newFindings,
		ExistingFindings:  runStats.existingFindings,
		ResolvedFindings:  resolvedCount,
		AutoFixCount:      runbookResolved,
		FindingsSummary:   findingsSummaryStr,
		FindingIDs:        runStats.findingIDs,
		ErrorCount:        runStats.errors,
		Status:            status,
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

	// Record circuit breaker result only if we actually attempted LLM calls
	// canRunLLM is true only when AI is enabled, licensed, AND breaker allowed
	if breaker != nil && canRunLLM {
		if runStats.errors > 0 {
			breaker.RecordFailure(fmt.Errorf("patrol completed with %d errors", runStats.errors))
		} else {
			breaker.RecordSuccess()
		}
	}

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
	return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
}

// generateFindingID creates a stable ID for a finding based on resource, category, and issue.
// All three components are included to ensure distinct issues on the same resource remain separate.
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

	if cfg.AnalyzeKubernetes {
		for _, cluster := range state.KubernetesClusters {
			findings = append(findings, p.analyzeKubernetesCluster(cluster)...)
		}
	}

	for _, finding := range findings {
		if finding != nil && finding.Source == "" {
			finding.Source = "heuristic"
		}
	}

	// Filter out findings from mock resources when not in demo mode
	// This ensures clean separation between mock and real data
	if !IsDemoMode() {
		filtered := make([]*Finding, 0, len(findings))
		for _, f := range findings {
			if f != nil && !IsMockResource(f.ResourceID, f.ResourceName, f.Node) {
				filtered = append(filtered, f)
			}
		}
		findings = filtered
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

		// Record event for correlation
		p.recordEvent(node.ID, node.Name, "node", CorrelationEventOffline, 0)
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

		// Record event for correlation
		p.recordEvent(node.ID, node.Name, "node", CorrelationEventHighMem, memUsagePct)
	}

	// Record CPU event if high
	if cpuPct > p.thresholds.NodeCPUWatch {
		p.recordEvent(node.ID, node.Name, "node", CorrelationEventHighCPU, cpuPct)
	}

	// Run anomaly detection
	findings = append(findings, p.checkAnomalies(node.ID, node.Name, "node", map[string]float64{
		"cpu":    cpuPct,
		"memory": memUsagePct,
	})...)

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

		// Record event for correlation
		p.recordEvent(id, name, guestType, CorrelationEventHighMem, memPct)
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

	// Run anomaly detection
	findings = append(findings, p.checkAnomalies(id, name, guestType, map[string]float64{
		"cpu":    cpu * 100,
		"memory": memPct,
		"disk":   diskPct,
	})...)

	return findings
}

// analyzeDockerHost checks a Docker/Podman host for issues
func (p *PatrolService) analyzeDockerHost(host models.DockerHost) []*Finding {
	var findings []*Finding

	hostName := host.Hostname
	if host.CustomDisplayName != "" {
		hostName = host.CustomDisplayName
	} else if host.DisplayName != "" {
		hostName = host.DisplayName
	}

	// Determine runtime type for better messages
	runtime := "Docker"
	if host.Runtime == "podman" || strings.Contains(strings.ToLower(host.RuntimeVersion), "podman") {
		runtime = "Podman"
	}

	// Host offline
	if host.Status != "online" && host.Status != "connected" && host.Status != "" {
		findings = append(findings, &Finding{
			ID:             generateFindingID(host.ID, "reliability", "offline"),
			Key:            "docker-host-offline",
			Severity:       FindingSeverityCritical,
			Category:       FindingCategoryReliability,
			ResourceID:     host.ID,
			ResourceName:   hostName,
			ResourceType:   "docker_host",
			Title:          runtime + " host offline",
			Description:    fmt.Sprintf("%s host '%s' is not responding (status: %s)", runtime, hostName, host.Status),
			Recommendation: "Check network connectivity and pulse-agent service on the host",
			Evidence:       fmt.Sprintf("Status: %s", host.Status),
		})

		// Record event for correlation
		p.recordEvent(host.ID, hostName, "docker_host", CorrelationEventOffline, 0)
	}

	// Host not seen recently (stale data)
	if !host.LastSeen.IsZero() && time.Since(host.LastSeen) > 10*time.Minute {
		findings = append(findings, &Finding{
			ID:             generateFindingID(host.ID, "reliability", "stale"),
			Key:            "docker-host-stale",
			Severity:       FindingSeverityWarning,
			Category:       FindingCategoryReliability,
			ResourceID:     host.ID,
			ResourceName:   hostName,
			ResourceType:   "docker_host",
			Title:          runtime + " host not reporting",
			Description:    fmt.Sprintf("%s host '%s' has not reported in %s", runtime, hostName, formatDurationPatrol(time.Since(host.LastSeen))),
			Recommendation: "Check pulse-agent service status and network connectivity",
			Evidence:       fmt.Sprintf("Last seen: %s", host.LastSeen.Format(time.RFC3339)),
		})
	}

	// Check individual containers
	for _, c := range host.Containers {
		containerName := c.Name

		// Restarting containers or containers in restart loop
		if c.State == "restarting" || c.RestartCount > 5 {
			severity := FindingSeverityWarning
			if c.RestartCount > 10 {
				severity = FindingSeverityCritical
			}
			findings = append(findings, &Finding{
				ID:             generateFindingID(c.ID, "reliability", "restart-loop"),
				Key:            "docker-restart-loop",
				Severity:       severity,
				Category:       FindingCategoryReliability,
				ResourceID:     c.ID,
				ResourceName:   containerName,
				ResourceType:   "docker_container",
				Node:           hostName,
				Title:          "Container restart loop",
				Description:    fmt.Sprintf("Container '%s' on '%s' has restarted %d times", containerName, hostName, c.RestartCount),
				Recommendation: fmt.Sprintf("Check container logs: docker logs %s", containerName),
				Evidence:       fmt.Sprintf("State: %s, Restarts: %d", c.State, c.RestartCount),
			})

			// Record restart event for correlation
			p.recordEvent(c.ID, containerName, "docker_container", CorrelationEventRestart, float64(c.RestartCount))
		}

		// Unhealthy containers (health check failing)
		if strings.ToLower(c.Health) == "unhealthy" {
			findings = append(findings, &Finding{
				ID:             generateFindingID(c.ID, "reliability", "unhealthy"),
				Key:            "docker-unhealthy",
				Severity:       FindingSeverityWarning,
				Category:       FindingCategoryReliability,
				ResourceID:     c.ID,
				ResourceName:   containerName,
				ResourceType:   "docker_container",
				Node:           hostName,
				Title:          "Container health check failing",
				Description:    fmt.Sprintf("Container '%s' on '%s' is reporting unhealthy", containerName, hostName),
				Recommendation: fmt.Sprintf("Check health check logs: docker inspect %s | jq '.[0].State.Health'", containerName),
				Evidence:       fmt.Sprintf("Health: %s, State: %s", c.Health, c.State),
			})
		}

		// Exited or dead containers with non-zero exit code
		if (c.State == "exited" || c.State == "dead") && c.ExitCode != 0 {
			findings = append(findings, &Finding{
				ID:             generateFindingID(c.ID, "reliability", "exited-error"),
				Key:            "docker-exited-error",
				Severity:       FindingSeverityWarning,
				Category:       FindingCategoryReliability,
				ResourceID:     c.ID,
				ResourceName:   containerName,
				ResourceType:   "docker_container",
				Node:           hostName,
				Title:          "Container exited with error",
				Description:    fmt.Sprintf("Container '%s' on '%s' exited with code %d", containerName, hostName, c.ExitCode),
				Recommendation: fmt.Sprintf("Check container logs: docker logs --tail 100 %s", containerName),
				Evidence:       fmt.Sprintf("State: %s, Exit code: %d", c.State, c.ExitCode),
			})
		}

		// High CPU usage
		if c.CPUPercent > 90 {
			severity := FindingSeverityWatch
			if c.CPUPercent > 95 {
				severity = FindingSeverityWarning
			}
			findings = append(findings, &Finding{
				ID:             generateFindingID(c.ID, "performance", "high-cpu"),
				Key:            "docker-high-cpu",
				Severity:       severity,
				Category:       FindingCategoryPerformance,
				ResourceID:     c.ID,
				ResourceName:   containerName,
				ResourceType:   "docker_container",
				Node:           hostName,
				Title:          "High CPU usage",
				Description:    fmt.Sprintf("Container '%s' on '%s' using %.0f%% CPU", containerName, hostName, c.CPUPercent),
				Recommendation: "Check for runaway processes or resource-intensive operations",
				Evidence:       fmt.Sprintf("CPU: %.1f%%", c.CPUPercent),
			})

			// Record event for correlation
			p.recordEvent(c.ID, containerName, "docker_container", CorrelationEventHighCPU, c.CPUPercent)
		}

		// High memory usage
		if c.MemoryPercent > 90 {
			severity := FindingSeverityWatch
			if c.MemoryPercent > 95 {
				severity = FindingSeverityWarning
			}
			findings = append(findings, &Finding{
				ID:             generateFindingID(c.ID, "performance", "high-memory"),
				Key:            "docker-high-memory",
				Severity:       severity,
				Category:       FindingCategoryPerformance,
				ResourceID:     c.ID,
				ResourceName:   containerName,
				ResourceType:   "docker_container",
				Node:           hostName,
				Title:          "High memory usage",
				Description:    fmt.Sprintf("Container '%s' on '%s' using %.0f%% of allocated memory", containerName, hostName, c.MemoryPercent),
				Recommendation: "Consider increasing container memory limit or optimizing memory usage",
				Evidence:       fmt.Sprintf("Memory: %.1f%%", c.MemoryPercent),
			})

			// Record event for correlation
			p.recordEvent(c.ID, containerName, "docker_container", CorrelationEventHighMem, c.MemoryPercent)
		}

		// Run anomaly detection for container
		findings = append(findings, p.checkAnomalies(c.ID, containerName, "docker_container", map[string]float64{
			"cpu":    c.CPUPercent,
			"memory": c.MemoryPercent,
		})...)
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

	// Run anomaly detection
	findings = append(findings, p.checkAnomalies(storage.ID, storage.Name, "storage", map[string]float64{
		"disk": usage,
	})...)

	return findings
}

// aiResolveFindings uses the LLM with MCP tools to intelligently evaluate active findings.
// This is TRUE AI-powered resolution - the AI uses tools to query current infrastructure state,
// analyzes each finding against live data, and decides which are resolved.
// We provide the scaffolding and context; the AI provides the intelligence.
func (p *PatrolService) aiResolveFindings(ctx context.Context) int {
	p.mu.RLock()
	aiService := p.aiService
	p.mu.RUnlock()

	if aiService == nil || !aiService.IsEnabled() || !aiService.HasLicenseFeature(FeatureAIPatrol) {
		return 0
	}

	activeFindings := p.findings.GetActive(FindingSeverityInfo)
	if len(activeFindings) == 0 {
		return 0
	}

	// Build context: list all active findings for the AI to evaluate
	var findingsContext strings.Builder
	findingsContext.WriteString("ACTIVE FINDINGS TO EVALUATE FOR RESOLUTION:\n\n")

	for i, f := range activeFindings {
		findingsContext.WriteString(fmt.Sprintf("FINDING %d:\n", i+1))
		findingsContext.WriteString(fmt.Sprintf("- ID: %s\n", f.ID))
		findingsContext.WriteString(fmt.Sprintf("- Resource: %s (type: %s)\n", f.ResourceName, f.ResourceType))
		findingsContext.WriteString(fmt.Sprintf("- Category: %s, Severity: %s\n", f.Category, f.Severity))
		findingsContext.WriteString(fmt.Sprintf("- Title: %s\n", f.Title))
		findingsContext.WriteString(fmt.Sprintf("- Description: %s\n", f.Description))
		findingsContext.WriteString(fmt.Sprintf("- Evidence when detected: %s\n", f.Evidence))
		findingsContext.WriteString(fmt.Sprintf("- First detected: %s ago\n", time.Since(f.DetectedAt).Round(time.Minute)))
		findingsContext.WriteString("\n")
	}

	// Build the prompt - give AI full toolkit for intelligent investigation
	prompt := fmt.Sprintf(`You are the AI Patrol resolution agent. Your job is to intelligently determine which findings are still valid issues and which have been resolved.

%s

## Your Available Tools
You have access to comprehensive infrastructure monitoring tools:

**Current State:**
- pulse_get_topology - Full infrastructure snapshot (nodes, VMs, containers, storage)
- pulse_get_resource - Detailed info on a specific resource
- pulse_search_resources - Search for resources by name/type

**Deep Analysis:**
- pulse_get_metrics - Time-series metrics to see trends (is it improving or worsening?)
- pulse_get_baselines - Learned NORMAL behavior patterns for each resource
- pulse_get_patterns - Detected anomaly patterns

**Infrastructure Health:**
- pulse_get_disk_health - SMART data, disk errors, predicted failures
- pulse_get_temperatures - CPU and disk temperatures
- pulse_get_ceph_status - Ceph cluster health (if applicable)
- pulse_get_host_raid_status - RAID array status
- pulse_get_connection_health - API connectivity status
- pulse_get_cluster_status - PVE cluster health

**Storage & Backups:**
- pulse_list_storage - Storage pool details
- pulse_list_backups - Backup status and history
- pulse_list_pbs_jobs - PBS backup jobs
- pulse_list_physical_disks - Physical disk information

**I/O & Network:**
- pulse_get_network_stats - Network throughput
- pulse_get_diskio_stats - Disk I/O statistics

## Investigation Strategy
For each finding, do a THOROUGH investigation:

1. **Check current state** - Is the metric still problematic?
2. **Check trends** - Is it getting better, worse, or stable?
3. **Check baselines** - Is current behavior normal for this resource?
4. **Look for root cause** - A finding might be a symptom of something else
5. **Correlate data** - High CPU might be caused by disk I/O issues, thermal throttling, etc.

## Decision Criteria
RESOLVE when:
- The metric has improved significantly AND is stable
- The issue was transient and baseline shows this is normal now
- The root cause has been addressed (e.g., disk was replaced)

KEEP when:
- Issue is still present, even if slightly improved
- Metrics are unstable or fluctuating
- You can't determine the current state with confidence
- There's a risk the issue could recur

## Output Format
After your investigation, respond with a JSON object:
{"resolve": [{"id": "finding-id", "reason": "detailed explanation based on your investigation"}]}

If nothing should be resolved:
{"resolve": []}

Begin your investigation now. Use multiple tools to build a complete picture before making decisions.`, findingsContext.String())

	log.Debug().Int("findings", len(activeFindings)).Msg("AI Patrol: Using AI with MCP tools to evaluate findings for resolution")

	// Execute with streaming - AI has access to MCP tools to query current infrastructure state
	var responseBuffer strings.Builder
	_, err := aiService.ExecuteStream(ctx, ExecuteRequest{
		Prompt:  prompt,
		UseCase: "patrol", // Use patrol model for this background task
	}, func(event StreamEvent) {
		if event.Type == "content" {
			if content, ok := event.Data.(string); ok {
				responseBuffer.WriteString(content)
			}
		}
	})

	if err != nil {
		log.Debug().Err(err).Msg("AI Patrol: Failed to get AI judgment on findings")
		return 0
	}

	response := responseBuffer.String()

	// Parse the AI's response and resolve findings
	resolvedCount := p.parseAndResolveFindings(response, activeFindings)
	if resolvedCount > 0 {
		log.Info().Int("resolved", resolvedCount).Msg("AI Patrol: AI resolved findings after tool-based investigation")
	}
	return resolvedCount
}

// parseAndResolveFindings parses the AI's JSON response and resolves the indicated findings
func (p *PatrolService) parseAndResolveFindings(response string, findings []*Finding) int {
	response = strings.TrimSpace(response)

	// Find JSON in the response (it might have surrounding text from tool use)
	startIdx := strings.Index(response, "{")
	endIdx := strings.LastIndex(response, "}")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		log.Debug().Str("response", response[:min(200, len(response))]).Msg("AI Patrol: Could not find JSON in AI response")
		return 0
	}
	jsonStr := response[startIdx : endIdx+1]

	// Parse the JSON
	type resolveItem struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}
	type aiResponse struct {
		Resolve []resolveItem `json:"resolve"`
	}

	var parsed aiResponse
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		log.Debug().Err(err).Str("json", jsonStr[:min(200, len(jsonStr))]).Msg("AI Patrol: Failed to parse AI response JSON")
		return 0
	}

	// Build a map of findings by ID for quick lookup
	findingMap := make(map[string]*Finding)
	for _, f := range findings {
		findingMap[f.ID] = f
	}

	// Resolve the findings the AI indicated
	resolvedCount := 0
	for _, item := range parsed.Resolve {
		if f, exists := findingMap[item.ID]; exists {
			if p.findings.Resolve(f.ID, true) {
				resolvedCount++
				log.Info().
					Str("finding_id", f.ID).
					Str("resource", f.ResourceName).
					Str("title", f.Title).
					Str("ai_reason", item.Reason).
					Msg("AI Patrol: AI resolved finding after investigation")
			}
		}
	}

	return resolvedCount
}

// reviewAndResolveAlerts uses AI to review active alerts and resolve those where the issue is fixed.
// This is the core of autonomous alert management - the AI looks at each alert, checks current state,
// and determines if the underlying issue has been resolved.
func (p *PatrolService) reviewAndResolveAlerts(ctx context.Context, state models.StateSnapshot, llmAllowed bool) int {
	p.mu.RLock()
	resolver := p.alertResolver
	aiService := p.aiService
	p.mu.RUnlock()

	if resolver == nil {
		return 0
	}

	activeAlerts := resolver.GetActiveAlerts()
	if len(activeAlerts) == 0 {
		return 0
	}

	// Only review alerts that have been active for at least 10 minutes
	// This avoids thrashing on transient alerts
	minAge := 10 * time.Minute
	var alertsToReview []AlertInfo
	for _, alert := range activeAlerts {
		if time.Since(alert.StartTime) >= minAge {
			alertsToReview = append(alertsToReview, alert)
		}
	}

	if len(alertsToReview) == 0 {
		return 0
	}

	log.Info().
		Int("total_active", len(activeAlerts)).
		Int("to_review", len(alertsToReview)).
		Msg("AI Patrol: Reviewing alerts for auto-resolution")

	resolvedCount := 0

	// Pass nil for aiService if LLM is not allowed (forces heuristics-only)
	aiSvc := aiService
	if !llmAllowed {
		aiSvc = nil
	}

	for _, alert := range alertsToReview {
		shouldResolve, reason := p.shouldResolveAlert(ctx, alert, state, aiSvc)
		if shouldResolve {
			if resolver.ResolveAlert(alert.ID) {
				resolvedCount++
				log.Info().
					Str("alertID", alert.ID).
					Str("resource", alert.ResourceName).
					Str("reason", reason).
					Dur("age", time.Since(alert.StartTime)).
					Msg("AI Patrol: Auto-resolved alert - issue no longer detected")
			}
		}
	}

	if resolvedCount > 0 {
		log.Info().
			Int("resolved", resolvedCount).
			Msg("AI Patrol: Completed alert review")
	}

	return resolvedCount
}

// shouldResolveAlert determines if an alert should be auto-resolved based on current state.
// Returns (shouldResolve, reason)
func (p *PatrolService) shouldResolveAlert(ctx context.Context, alert AlertInfo, state models.StateSnapshot, aiService *Service) (bool, string) {
	// First, try smart heuristic checks based on alert type
	switch alert.Type {
	case "usage": // Storage usage alert
		// Find the storage in current state
		for _, storage := range state.Storage {
			if storage.ID == alert.ResourceID {
				// If current usage is below the threshold (with some margin), resolve
				if storage.Usage < alert.Threshold*0.95 { // 5% margin below threshold
					return true, fmt.Sprintf("storage usage dropped from %.1f%% to %.1f%% (threshold: %.1f%%)",
						alert.Value, storage.Usage, alert.Threshold)
				}
				// Still high, don't resolve
				return false, ""
			}
		}
		// Storage not found in current state - might have been removed
		// Resolve after 24 hours if resource is gone
		if time.Since(alert.StartTime) > 24*time.Hour {
			return true, "resource no longer present in infrastructure"
		}

	case "cpu", "memory": // Resource utilization alerts
		// Check if this is a node, VM, container, or docker container
		currentValue := p.getCurrentMetricValue(alert, state)
		if currentValue >= 0 && currentValue < alert.Threshold*0.95 {
			return true, fmt.Sprintf("%s dropped from %.1f%% to %.1f%% (threshold: %.1f%%)",
				alert.Type, alert.Value, currentValue, alert.Threshold)
		}

	case "offline", "stopped", "docker-offline":
		// Check if the resource is now online
		if p.isResourceOnline(alert, state) {
			return true, "resource is now online/running"
		}
	}

	// For complex cases or when heuristics don't apply, use AI judgment if available
	if aiService != nil && aiService.IsEnabled() {
		return p.askAIAboutAlert(ctx, alert, state, aiService)
	}

	return false, ""
}

// getCurrentMetricValue gets the current value of the metric that triggered the alert
func (p *PatrolService) getCurrentMetricValue(alert AlertInfo, state models.StateSnapshot) float64 {
	switch alert.ResourceType {
	case "node":
		for _, node := range state.Nodes {
			if node.ID == alert.ResourceID || node.Name == alert.ResourceName {
				if alert.Type == "cpu" {
					return node.CPU
				} else if alert.Type == "memory" {
					return node.Memory.Usage
				}
			}
		}
	case "guest", "vm":
		for _, vm := range state.VMs {
			if vm.ID == alert.ResourceID || vm.Name == alert.ResourceName {
				if alert.Type == "cpu" {
					return vm.CPU
				} else if alert.Type == "memory" {
					return vm.Memory.Usage
				}
			}
		}
	case "container":
		for _, ct := range state.Containers {
			if ct.ID == alert.ResourceID || ct.Name == alert.ResourceName {
				if alert.Type == "cpu" {
					return ct.CPU
				} else if alert.Type == "memory" {
					return ct.Memory.Usage
				}
			}
		}
	case "docker":
		for _, host := range state.DockerHosts {
			for _, container := range host.Containers {
				if container.ID == alert.ResourceID || container.Name == alert.ResourceName {
					if alert.Type == "cpu" {
						return container.CPUPercent
					} else if alert.Type == "memory" {
						return container.MemoryPercent
					}
				}
			}
		}
	case "Storage":
		for _, storage := range state.Storage {
			if storage.ID == alert.ResourceID || storage.Name == alert.ResourceName {
				return storage.Usage
			}
		}
	}
	return -1 // Not found
}

// isResourceOnline checks if a resource that triggered an offline alert is now online
func (p *PatrolService) isResourceOnline(alert AlertInfo, state models.StateSnapshot) bool {
	switch alert.ResourceType {
	case "node":
		for _, node := range state.Nodes {
			if (node.ID == alert.ResourceID || node.Name == alert.ResourceName) && node.Status == "online" {
				return true
			}
		}
	case "guest", "vm":
		for _, vm := range state.VMs {
			if (vm.ID == alert.ResourceID || vm.Name == alert.ResourceName) && vm.Status == "running" {
				return true
			}
		}
	case "container":
		for _, ct := range state.Containers {
			if (ct.ID == alert.ResourceID || ct.Name == alert.ResourceName) && ct.Status == "running" {
				return true
			}
		}
	case "docker":
		for _, host := range state.DockerHosts {
			for _, container := range host.Containers {
				if (container.ID == alert.ResourceID || container.Name == alert.ResourceName) && container.State == "running" {
					return true
				}
			}
		}
	}
	return false
}

// askAIAboutAlert uses the AI to determine if an alert should be resolved
func (p *PatrolService) askAIAboutAlert(ctx context.Context, alert AlertInfo, state models.StateSnapshot, aiService *Service) (bool, string) {
	// Build a focused prompt for the AI
	prompt := fmt.Sprintf(`Review this alert and determine if it should be auto-resolved based on current state.

ALERT:
- ID: %s
- Type: %s
- Resource: %s (%s)
- Message: %s
- Value when triggered: %.1f
- Threshold: %.1f
- Active for: %s

CURRENT STATE OF THIS RESOURCE:
%s

Should this alert be RESOLVED because the underlying issue is fixed?
Respond with ONLY one of:
- RESOLVE: <brief reason>
- KEEP: <brief reason>`,
		alert.ID, alert.Type, alert.ResourceName, alert.ResourceType,
		alert.Message, alert.Value, alert.Threshold, alert.Duration,
		p.getResourceCurrentState(alert, state))

	// Use a quick, low-cost AI call
	response, err := aiService.QuickAnalysis(ctx, prompt)
	if err != nil {
		log.Debug().Err(err).Str("alertID", alert.ID).Msg("AI Patrol: Failed to get AI judgment on alert")
		return false, ""
	}

	response = strings.TrimSpace(response)
	if strings.HasPrefix(strings.ToUpper(response), "RESOLVE:") {
		reason := strings.TrimSpace(strings.TrimPrefix(response, "RESOLVE:"))
		if reason == "" {
			reason = strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(response), "RESOLVE:"))
		}
		return true, "Patrol: " + reason
	}

	return false, ""
}

// getResourceCurrentState returns a description of the resource's current state
func (p *PatrolService) getResourceCurrentState(alert AlertInfo, state models.StateSnapshot) string {
	switch alert.ResourceType {
	case "Storage":
		for _, storage := range state.Storage {
			if storage.ID == alert.ResourceID || storage.Name == alert.ResourceName {
				return fmt.Sprintf("Storage '%s': %.1f%% used, status: %s", storage.Name, storage.Usage, storage.Status)
			}
		}
		return "Storage not found in current state (may have been removed)"
	case "node":
		for _, node := range state.Nodes {
			if node.ID == alert.ResourceID || node.Name == alert.ResourceName {
				return fmt.Sprintf("Node '%s': CPU %.1f%%, Memory %.1f%%, Status: %s",
					node.Name, node.CPU, node.Memory.Usage, node.Status)
			}
		}
		return "Node not found in current state"
	case "guest", "vm":
		for _, vm := range state.VMs {
			if vm.ID == alert.ResourceID || vm.Name == alert.ResourceName {
				return fmt.Sprintf("VM '%s': CPU %.1f%%, Memory %.1f%%, Status: %s",
					vm.Name, vm.CPU, vm.Memory.Usage, vm.Status)
			}
		}
		return "VM not found in current state"
	case "container":
		for _, ct := range state.Containers {
			if ct.ID == alert.ResourceID || ct.Name == alert.ResourceName {
				return fmt.Sprintf("Container '%s': CPU %.1f%%, Memory %.1f%%, Status: %s",
					ct.Name, ct.CPU, ct.Memory.Usage, ct.Status)
			}
		}
		return "Container not found in current state"
	case "docker":
		for _, host := range state.DockerHosts {
			for _, container := range host.Containers {
				if container.ID == alert.ResourceID || container.Name == alert.ResourceName {
					return fmt.Sprintf("Docker container '%s': CPU %.1f%%, Memory %.1f%%, State: %s",
						container.Name, container.CPUPercent, container.MemoryPercent, container.State)
				}
			}
		}
		return "Docker container not found in current state"
	default:
		return "Resource state unknown"
	}
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

// DismissFinding dismisses a finding with a reason and note
// This is called when the AI determines the finding is not actually an issue
// For reasons "expected_behavior" or "not_an_issue", a suppression rule is automatically created
func (p *PatrolService) DismissFinding(findingID string, reason string, note string) error {
	if findingID == "" {
		return fmt.Errorf("finding ID is required")
	}

	// Validate reason
	validReasons := map[string]bool{"not_an_issue": true, "expected_behavior": true, "will_fix_later": true}
	if !validReasons[reason] {
		return fmt.Errorf("invalid reason: %s", reason)
	}

	// Check that the finding exists
	finding := p.findings.Get(findingID)
	if finding == nil {
		return fmt.Errorf("finding not found: %s", findingID)
	}

	// Dismiss the finding:
	// - "not_an_issue" creates permanent suppression (true false positive)
	// - "expected_behavior" and "will_fix_later" just acknowledge (stays visible but marked)
	if !p.findings.Dismiss(findingID, reason, note) {
		return fmt.Errorf("failed to dismiss finding: %s", findingID)
	}

	log.Info().
		Str("finding_id", findingID).
		Str("reason", reason).
		Str("note", note).
		Bool("permanently_suppressed", reason == "not_an_issue").
		Msg("AI dismissed finding")

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
	runCtx := context.Background()
	if ctx != nil {
		runCtx = context.WithoutCancel(ctx)
	}
	go p.runPatrol(runCtx)
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

		// Record event for correlation
		p.recordEvent(host.ID, hostName, "host", CorrelationEventOffline, 0)
	}

	// Check CPU usage
	if host.CPUUsage > 90 {
		severity := FindingSeverityWatch
		if host.CPUUsage > 95 {
			severity = FindingSeverityWarning
		}
		findings = append(findings, &Finding{
			ID:             generateFindingID(host.ID, "performance", "high-cpu"),
			Key:            "host-high-cpu",
			Severity:       severity,
			Category:       FindingCategoryPerformance,
			ResourceID:     host.ID,
			ResourceName:   hostName,
			ResourceType:   "host",
			Title:          "High CPU usage",
			Description:    fmt.Sprintf("Host '%s' CPU at %.0f%%", hostName, host.CPUUsage),
			Recommendation: "Check for runaway processes or resource-intensive operations",
			Evidence:       fmt.Sprintf("CPU: %.1f%%", host.CPUUsage),
		})

		// Record event for correlation
		p.recordEvent(host.ID, hostName, "host", CorrelationEventHighCPU, host.CPUUsage)
	}

	// Check Memory usage
	memPct := 0.0
	if host.Memory.Total > 0 {
		memPct = float64(host.Memory.Used) / float64(host.Memory.Total) * 100
	}
	if memPct > 90 {
		severity := FindingSeverityWatch
		if memPct > 95 {
			severity = FindingSeverityWarning
		}
		findings = append(findings, &Finding{
			ID:             generateFindingID(host.ID, "performance", "high-memory"),
			Key:            "host-high-memory",
			Severity:       severity,
			Category:       FindingCategoryPerformance,
			ResourceID:     host.ID,
			ResourceName:   hostName,
			ResourceType:   "host",
			Title:          "High memory usage",
			Description:    fmt.Sprintf("Host '%s' memory at %.0f%%", hostName, memPct),
			Recommendation: "Check for memory leaks or investigate memory-intensive operations",
			Evidence:       fmt.Sprintf("Memory: %.1f%%", memPct),
		})

		// Record event for correlation
		p.recordEvent(host.ID, hostName, "host", CorrelationEventHighMem, memPct)
	}

	// Run anomaly detection
	findings = append(findings, p.checkAnomalies(host.ID, hostName, "host", map[string]float64{
		"cpu":    host.CPUUsage,
		"memory": memPct,
	})...)

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

// analyzeKubernetesCluster checks a Kubernetes cluster for issues
func (p *PatrolService) analyzeKubernetesCluster(cluster models.KubernetesCluster) []*Finding {
	var findings []*Finding

	clusterName := cluster.CustomDisplayName
	if clusterName == "" {
		clusterName = cluster.DisplayName
	}
	if clusterName == "" {
		clusterName = cluster.Name
	}
	if clusterName == "" {
		clusterName = cluster.ID
	}

	// Check cluster connectivity (if last seen is too old)
	if !cluster.LastSeen.IsZero() && time.Since(cluster.LastSeen) > 10*time.Minute {
		findings = append(findings, &Finding{
			ID:             generateFindingID(cluster.ID, "reliability", "cluster-offline"),
			Key:            "kubernetes-cluster-offline",
			Severity:       FindingSeverityCritical,
			Category:       FindingCategoryReliability,
			ResourceID:     cluster.ID,
			ResourceName:   clusterName,
			ResourceType:   "kubernetes_cluster",
			Title:          "Kubernetes cluster offline",
			Description:    fmt.Sprintf("Kubernetes cluster '%s' has not reported in %s", clusterName, formatDurationPatrol(time.Since(cluster.LastSeen))),
			Recommendation: "Check the Pulse Kubernetes agent deployment and cluster connectivity",
			Evidence:       fmt.Sprintf("Last seen: %s", cluster.LastSeen.Format(time.RFC3339)),
		})
	}

	// Check for pending uninstall
	if cluster.PendingUninstall {
		findings = append(findings, &Finding{
			ID:             generateFindingID(cluster.ID, "configuration", "pending-uninstall"),
			Key:            "kubernetes-pending-uninstall",
			Severity:       FindingSeverityInfo,
			Category:       FindingCategoryGeneral,
			ResourceID:     cluster.ID,
			ResourceName:   clusterName,
			ResourceType:   "kubernetes_cluster",
			Title:          "Kubernetes cluster pending uninstall",
			Description:    fmt.Sprintf("Kubernetes cluster '%s' is marked for uninstall", clusterName),
			Recommendation: "Complete the uninstall process or cancel if unintended",
		})
	}

	// Check for unhealthy nodes
	unhealthyNodes := 0
	unschedulableNodes := 0
	for _, node := range cluster.Nodes {
		if !node.Ready {
			unhealthyNodes++
		}
		if node.Unschedulable {
			unschedulableNodes++
		}
	}

	if unhealthyNodes > 0 {
		severity := FindingSeverityWarning
		if unhealthyNodes == len(cluster.Nodes) {
			severity = FindingSeverityCritical // All nodes are unhealthy
		}
		findings = append(findings, &Finding{
			ID:             generateFindingID(cluster.ID, "reliability", "nodes-not-ready"),
			Key:            "kubernetes-nodes-not-ready",
			Severity:       severity,
			Category:       FindingCategoryReliability,
			ResourceID:     cluster.ID,
			ResourceName:   clusterName,
			ResourceType:   "kubernetes_cluster",
			Title:          "Kubernetes nodes not ready",
			Description:    fmt.Sprintf("%d of %d nodes in cluster '%s' are not ready", unhealthyNodes, len(cluster.Nodes), clusterName),
			Recommendation: "Check node conditions with 'kubectl get nodes' and 'kubectl describe node <name>'",
			Evidence:       fmt.Sprintf("Not ready: %d, Unschedulable: %d, Total: %d", unhealthyNodes, unschedulableNodes, len(cluster.Nodes)),
		})
	}

	// Check for pods in problematic states
	crashLoopPods := 0
	pendingPods := 0
	failedPods := 0
	highRestartPods := 0

	for _, pod := range cluster.Pods {
		phase := strings.ToLower(strings.TrimSpace(pod.Phase))

		switch phase {
		case "failed":
			failedPods++
		case "pending":
			pendingPods++
		}

		// Check for CrashLoopBackOff or high restarts
		if pod.Restarts > 10 {
			highRestartPods++
		}
		for _, container := range pod.Containers {
			if strings.Contains(strings.ToLower(container.Reason), "crashloop") {
				crashLoopPods++
				break
			}
		}
	}

	if crashLoopPods > 0 {
		findings = append(findings, &Finding{
			ID:             generateFindingID(cluster.ID, "reliability", "crashloop-pods"),
			Key:            "kubernetes-crashloop-pods",
			Severity:       FindingSeverityWarning,
			Category:       FindingCategoryReliability,
			ResourceID:     cluster.ID,
			ResourceName:   clusterName,
			ResourceType:   "kubernetes_cluster",
			Title:          "Pods in CrashLoopBackOff",
			Description:    fmt.Sprintf("%d pod(s) in cluster '%s' are in CrashLoopBackOff", crashLoopPods, clusterName),
			Recommendation: "Check pod logs with 'kubectl logs <pod>' and events with 'kubectl describe pod <pod>'",
			Evidence:       fmt.Sprintf("CrashLoopBackOff: %d", crashLoopPods),
		})
	}

	if failedPods > 0 {
		findings = append(findings, &Finding{
			ID:             generateFindingID(cluster.ID, "reliability", "failed-pods"),
			Key:            "kubernetes-failed-pods",
			Severity:       FindingSeverityWarning,
			Category:       FindingCategoryReliability,
			ResourceID:     cluster.ID,
			ResourceName:   clusterName,
			ResourceType:   "kubernetes_cluster",
			Title:          "Failed pods",
			Description:    fmt.Sprintf("%d pod(s) in cluster '%s' are in Failed state", failedPods, clusterName),
			Recommendation: "Check pod logs and events. Failed pods may need manual cleanup or intervention.",
			Evidence:       fmt.Sprintf("Failed: %d", failedPods),
		})
	}

	// Check for deployments not at desired replica count
	unhealthyDeployments := 0
	for _, deployment := range cluster.Deployments {
		if deployment.DesiredReplicas > 0 {
			if deployment.AvailableReplicas < deployment.DesiredReplicas ||
				deployment.ReadyReplicas < deployment.DesiredReplicas {
				unhealthyDeployments++
			}
		}
	}

	if unhealthyDeployments > 0 {
		findings = append(findings, &Finding{
			ID:             generateFindingID(cluster.ID, "reliability", "deployments-unavailable"),
			Key:            "kubernetes-deployments-unavailable",
			Severity:       FindingSeverityWarning,
			Category:       FindingCategoryReliability,
			ResourceID:     cluster.ID,
			ResourceName:   clusterName,
			ResourceType:   "kubernetes_cluster",
			Title:          "Deployments not fully available",
			Description:    fmt.Sprintf("%d deployment(s) in cluster '%s' are not at desired replica count", unhealthyDeployments, clusterName),
			Recommendation: "Check deployment status with 'kubectl rollout status' and pod events",
			Evidence:       fmt.Sprintf("Unhealthy deployments: %d of %d", unhealthyDeployments, len(cluster.Deployments)),
		})
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

// cleanThinkingTokens removes model-specific thinking markers from AI responses.
// Different AI models use different markers for their internal reasoning:
// - DeepSeek: <｜end▁of▁thinking｜> or similar unicode variants
// - Other models may use <think></think> or similar markers
func cleanThinkingTokens(content string) string {
	if content == "" {
		return content
	}

	// Remove DeepSeek thinking markers and everything before them on the same line
	// These appear as: <｜end▁of▁thinking｜> or <|end_of_thinking|>
	thinkingMarkers := []string{
		"<｜end▁of▁thinking｜>", // DeepSeek Unicode variant
		"<|end_of_thinking|>", // ASCII variant
		"<|end▁of▁thinking|>", // Mixed variant
		"</think>",            // Generic thinking block end
	}

	for _, marker := range thinkingMarkers {
		for strings.Contains(content, marker) {
			idx := strings.Index(content, marker)
			if idx >= 0 {
				// Find start of the line containing the marker
				lineStart := strings.LastIndex(content[:idx], "\n")
				if lineStart == -1 {
					lineStart = 0
				}
				// Find end of the line containing the marker
				markerEnd := idx + len(marker)
				lineEnd := strings.Index(content[markerEnd:], "\n")
				if lineEnd == -1 {
					lineEnd = len(content)
				} else {
					lineEnd = markerEnd + lineEnd
				}
				// Remove the entire line containing the marker
				content = content[:lineStart] + content[lineEnd:]
			}
		}
	}

	// Also remove any lines that look like internal reasoning
	// These typically start with patterns like "Now, " or "Let's " after a blank line
	lines := strings.Split(content, "\n")
	var cleanedLines []string
	skipUntilContent := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip lines that look like internal reasoning
		if skipUntilContent {
			// Resume when we hit actual content (markdown headers, findings, etc.)
			if strings.HasPrefix(trimmed, "#") ||
				strings.HasPrefix(trimmed, "[FINDING]") ||
				strings.HasPrefix(trimmed, "**") ||
				strings.HasPrefix(trimmed, "-") ||
				strings.HasPrefix(trimmed, "1.") {
				skipUntilContent = false
			} else {
				continue
			}
		}

		// Detect reasoning patterns (typically after empty lines)
		if trimmed == "" && i+1 < len(lines) {
			nextTrimmed := strings.TrimSpace(lines[i+1])
			if strings.HasPrefix(nextTrimmed, "Now, ") ||
				strings.HasPrefix(nextTrimmed, "Let's ") ||
				strings.HasPrefix(nextTrimmed, "Let me ") ||
				strings.HasPrefix(nextTrimmed, "I should ") ||
				strings.HasPrefix(nextTrimmed, "I'll ") ||
				strings.HasPrefix(nextTrimmed, "I need to ") ||
				strings.HasPrefix(nextTrimmed, "Checking ") ||
				strings.HasPrefix(nextTrimmed, "Looking at ") {
				skipUntilContent = true
				continue
			}
		}

		cleanedLines = append(cleanedLines, line)
	}

	// Clean up excessive blank lines
	content = strings.Join(cleanedLines, "\n")
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(content)
}

// runAIAnalysis uses the LLM to analyze infrastructure and identify issues
func (p *PatrolService) runAIAnalysis(ctx context.Context, state models.StateSnapshot) (*AIAnalysisResult, error) {
	if p.aiService == nil {
		return nil, fmt.Errorf("Pulse Patrol service not available")
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
			// Thinking chunks are for live streaming only - don't persist them
			// They allow users to see the AI's reasoning in real-time, but the
			// final stored analysis should only contain the actual findings
			if thinking, ok := event.Data.(string); ok && thinking != "" {
				// Broadcast for live viewing ONLY - don't add to contentBuffer
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

	// Clean any thinking tokens that might have leaked through from the provider
	finalContent = cleanThinkingTokens(finalContent)

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

	// Validate findings to filter out noise before storing
	findings = p.validateAIFindings(findings, state)

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

	basePrompt := `You are an infrastructure analyst for Pulse, a Proxmox monitoring system. Your job is to identify ONLY issues that require human attention.

IMPORTANT: You must respond in a specific structured format so findings can be parsed.

For each issue you identify, output a finding block like this:

[FINDING]
KEY: <stable issue key>
SEVERITY: critical|warning|watch|info
CATEGORY: performance|reliability|security|capacity|configuration
RESOURCE: <resource name or ID>
RESOURCE_TYPE: node|vm|container|docker_container|storage|host|kubernetes_cluster
TITLE: <brief issue title>
DESCRIPTION: <detailed description of the issue>
RECOMMENDATION: <specific actionable recommendation>
EVIDENCE: <specific data that supports this finding>
[/FINDING]

Guidelines:
- Use KEY as a stable identifier for the issue type (examples: high-cpu, high-memory, high-disk, backup-stale, backup-never, restart-loop, storage-high-usage, pbs-datastore-high-usage, pbs-job-failed, node-offline). Use "general" if nothing fits.

SEVERITY GUIDELINES (be VERY conservative):
- CRITICAL: Service completely down, data loss imminent, disk >95%, node offline
- WARNING: Disk >85%, memory >90% sustained, backup failed >48 hours
- WATCH: Only use for trends that WILL become critical within 7 days at current rate
- INFO: Almost never use - only for significant security or config issues

===== STRICT THRESHOLDS - DO NOT CREATE FINDINGS BELOW THESE =====
- CPU: Only report if >70% sustained (brief spikes are normal)
- Memory: Only report if >80% sustained (applications cache memory, this is fine)
- Disk/Storage: Only report if >75% OR growing >5%/week toward full
- Baseline deviations: IGNORE unless current value exceeds the absolute thresholds above

===== NOISE TO AVOID - DO NOT REPORT THESE =====
- "CPU at 15% vs baseline 8%" - This is NORMAL variance, NOT an issue
- "Memory at 45% which is elevated" - This is FINE, lots of headroom
- "Disk at 30% is above baseline" - This is FINE, not actionable
- Stopped containers/VMs (unless autostart is enabled AND they crashed)
- Minor metric fluctuations compared to baseline
- Resources that are simply "busier than usual" but not near limits

BEFORE CREATING A FINDING, ASK YOURSELF:
1. Would an operator need to DO something about this RIGHT NOW?
2. Is this an actual problem, or just "different from yesterday"?
3. If I woke someone up at 3am for this, would they thank me or curse me?

If everything looks healthy, respond with NO findings. An empty report is the BEST report.`

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

	// Kubernetes clusters
	if len(state.KubernetesClusters) > 0 {
		sb.WriteString("## Kubernetes Clusters\n")
		for _, cluster := range state.KubernetesClusters {
			clusterName := cluster.CustomDisplayName
			if clusterName == "" {
				clusterName = cluster.DisplayName
			}
			if clusterName == "" {
				clusterName = cluster.Name
			}
			if clusterName == "" {
				clusterName = cluster.ID
			}

			// Count node health
			readyNodes := 0
			for _, node := range cluster.Nodes {
				if node.Ready {
					readyNodes++
				}
			}

			// Count pod health
			runningPods := 0
			problemPods := 0
			for _, pod := range cluster.Pods {
				phase := strings.ToLower(strings.TrimSpace(pod.Phase))
				switch phase {
				case "running":
					runningPods++
				case "failed", "pending":
					problemPods++
				}
			}

			// Count deployment health
			healthyDeployments := 0
			for _, d := range cluster.Deployments {
				if d.DesiredReplicas <= 0 || (d.AvailableReplicas >= d.DesiredReplicas && d.ReadyReplicas >= d.DesiredReplicas) {
					healthyDeployments++
				}
			}

			lastSeen := "unknown"
			if !cluster.LastSeen.IsZero() {
				lastSeen = fmt.Sprintf("%s ago", formatDurationPatrol(time.Since(cluster.LastSeen)))
			}

			sb.WriteString(fmt.Sprintf("- **%s** (ID:%s): Version=%s, LastSeen=%s\n",
				clusterName, cluster.ID, cluster.Version, lastSeen))
			sb.WriteString(fmt.Sprintf("  - Nodes: %d/%d ready\n", readyNodes, len(cluster.Nodes)))
			sb.WriteString(fmt.Sprintf("  - Pods: %d running, %d problem, %d total\n", runningPods, problemPods, len(cluster.Pods)))
			sb.WriteString(fmt.Sprintf("  - Deployments: %d/%d healthy\n", healthyDeployments, len(cluster.Deployments)))
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
	correlationDetector := p.correlationDetector
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

			// Record events for correlation analysis
			if correlationDetector != nil {
				for _, change := range newChanges {
					var eventType CorrelationEventType
					switch change.ChangeType {
					case memory.ChangeMigrated:
						eventType = CorrelationEventMigration
					case memory.ChangeRestarted:
						eventType = CorrelationEventRestart
					default:
						continue
					}

					p.recordEvent(change.ResourceID, "", change.ResourceType, eventType, 0)
				}
			}
		}
	}

	// Append failure predictions if pattern detector is available
	p.mu.RLock()
	patternDetector := p.patternDetector
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
	var infraContext string
	var incidentContext string
	var discoveryContext string
	p.mu.RLock()
	knowledgeStore := p.knowledgeStore
	incidentStore := p.incidentStore
	discoveryStore := p.discoveryStore
	p.mu.RUnlock()
	if knowledgeStore != nil {
		knowledgeContext = knowledgeStore.FormatAllForContext()
		infraContext = knowledgeStore.GetInfrastructureContext()
	}
	if incidentStore != nil {
		incidentContext = incidentStore.FormatForPatrol(8)
	}
	// Get AI discovery context (deep-scanned service info)
	if discoveryStore != nil {
		if discoveries, err := discoveryStore.List(); err == nil && len(discoveries) > 0 {
			discoveryContext = aidiscovery.FormatForAIContext(discoveries)
		}
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

	// Append infrastructure discovery context (auto-discovered apps and services)
	if infraContext != "" {
		contextAdditions.WriteString("\n\n")
		contextAdditions.WriteString(infraContext)
		contextAdditions.WriteString(`
IMPORTANT: When proposing remediation commands, use the CLI access method shown above.
- If a service runs in Docker, use 'docker exec <container> <command>' instead of direct commands
- Example: For PBS in Docker, use 'docker exec pbs proxmox-backup-manager gc pbs-delly' not 'proxmox-backup-manager gc pbs-delly'
- This ensures commands execute in the correct environment where the service actually runs.
`)
	}

	// Append deep AI discovery context (service details, versions, config paths, ports)
	if discoveryContext != "" {
		contextAdditions.WriteString("\n\n")
		contextAdditions.WriteString(discoveryContext)
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

	// Get new AI intelligence context (Phase 6)
	p.mu.RLock()
	learningProvider := p.learningProvider
	proxmoxProvider := p.proxmoxEventProvider
	forecastProvider := p.forecastProvider
	p.mu.RUnlock()

	// Append learned preferences from user feedback
	if learningProvider != nil {
		learningContext := learningProvider.FormatForContext()
		if learningContext != "" {
			contextAdditions.WriteString("\n\n## LEARNED PREFERENCES\n")
			contextAdditions.WriteString(learningContext)
			contextAdditions.WriteString("\nIMPORTANT: These preferences are learned from user feedback. Respect them when prioritizing and reporting issues.")
		}
	}

	// Append recent Proxmox operations
	if proxmoxProvider != nil {
		proxmoxContext := proxmoxProvider.FormatForPatrol(30 * time.Minute)
		if proxmoxContext != "" {
			contextAdditions.WriteString("\n\n## RECENT PROXMOX OPERATIONS (last 30 min)\n")
			contextAdditions.WriteString(proxmoxContext)
			contextAdditions.WriteString("\nIMPORTANT: Consider these recent operations when analyzing resource behavior. Migrations, backups, and snapshots can explain transient performance changes.")
		}
	}

	// Append key forecasts for resources with concerning trends
	if forecastProvider != nil {
		forecastContext := forecastProvider.FormatKeyForecasts()
		if forecastContext != "" {
			contextAdditions.WriteString("\n\n## TREND FORECASTS\n")
			contextAdditions.WriteString(forecastContext)
			contextAdditions.WriteString("\nIMPORTANT: Use these forecasts to provide proactive warnings about resources approaching capacity.")
		}
	}

	if contextAdditions.Len() > 0 {
		return basePrompt + contextAdditions.String()
	}

	return basePrompt
}

// recordEvent records an event in the correlation detector if it's significant
func (p *PatrolService) recordEvent(resourceID, resourceName, resourceType string, eventType CorrelationEventType, value float64) {
	p.mu.RLock()
	cd := p.correlationDetector
	p.mu.RUnlock()

	if cd == nil {
		return
	}

	cd.RecordEvent(CorrelationEvent{
		ID:           fmt.Sprintf("%s-%s-%d", resourceID, eventType, time.Now().Unix()),
		ResourceID:   resourceID,
		ResourceName: resourceName,
		ResourceType: resourceType,
		EventType:    eventType,
		Timestamp:    time.Now(),
		Value:        value,
	})
}

// checkAnomalies uses learned baselines to detect abnormal metric values
func (p *PatrolService) checkAnomalies(resourceID, resourceName, resourceType string, metrics map[string]float64) []*Finding {
	p.mu.RLock()
	bs := p.baselineStore
	p.mu.RUnlock()

	if bs == nil {
		return nil
	}

	var findings []*Finding
	for metric, value := range metrics {
		severity, zScore, bl := bs.CheckAnomaly(resourceID, metric, value)
		// We only care about High or Critical anomalies (Z-score > 3.0)
		if severity == baseline.AnomalyHigh || severity == baseline.AnomalyCritical {
			findingSeverity := FindingSeverityWatch
			if severity == baseline.AnomalyCritical {
				findingSeverity = FindingSeverityWarning
			}

			findings = append(findings, &Finding{
				ID:             generateFindingID(resourceID, "performance", "anomaly-"+metric),
				Key:            "metric-anomaly",
				Severity:       findingSeverity,
				Category:       FindingCategoryPerformance,
				ResourceID:     resourceID,
				ResourceName:   resourceName,
				ResourceType:   resourceType,
				Title:          fmt.Sprintf("Anomalous %s usage", metric),
				Description:    fmt.Sprintf("'%s' is showing abnormal %s usage of %.1f%% (typical mean: %.1f%%, dev: %.1f)", resourceName, metric, value, bl.Mean, bl.StdDev),
				Recommendation: "Investigate if this change in behavior is expected for this resource.",
				Evidence:       fmt.Sprintf("Current: %.1f, Baseline Mean: %.1f, Z-Score: %.1f", value, bl.Mean, zScore),
			})

			// Record this as an event for correlation if it's a spike
			switch metric {
			case "cpu":
				p.recordEvent(resourceID, resourceName, resourceType, CorrelationEventHighCPU, value)
			case "memory":
				p.recordEvent(resourceID, resourceName, resourceType, CorrelationEventHighMem, value)
			case "disk":
				p.recordEvent(resourceID, resourceName, resourceType, CorrelationEventDiskFull, value)
			}
		}
	}
	return findings
}

// validateAIFindings filters out noisy/invalid findings from AI analysis
// This is a safety net to catch cases where the LLM generates low-quality findings
// that would annoy users (e.g., "CPU at 7% vs typical 4% is elevated")
func (p *PatrolService) validateAIFindings(findings []*Finding, state models.StateSnapshot) []*Finding {
	if len(findings) == 0 {
		return findings
	}

	// Build a lookup of current resource metrics for validation
	resourceMetrics := make(map[string]map[string]float64)

	// Index node metrics
	for _, n := range state.Nodes {
		metrics := make(map[string]float64)
		metrics["cpu"] = n.CPU * 100
		if n.Memory.Total > 0 {
			metrics["memory"] = float64(n.Memory.Used) / float64(n.Memory.Total) * 100
		}
		resourceMetrics[n.ID] = metrics
		resourceMetrics[n.Name] = metrics // Also index by name
	}

	// Index VM metrics
	for _, vm := range state.VMs {
		metrics := make(map[string]float64)
		metrics["cpu"] = vm.CPU * 100
		metrics["memory"] = vm.Memory.Usage
		metrics["disk"] = vm.Disk.Usage
		resourceMetrics[vm.ID] = metrics
		resourceMetrics[vm.Name] = metrics
	}

	// Index container metrics
	for _, ct := range state.Containers {
		metrics := make(map[string]float64)
		metrics["cpu"] = ct.CPU * 100
		metrics["memory"] = ct.Memory.Usage
		metrics["disk"] = ct.Disk.Usage
		resourceMetrics[ct.ID] = metrics
		resourceMetrics[ct.Name] = metrics
	}

	// Index storage metrics
	for _, s := range state.Storage {
		metrics := make(map[string]float64)
		if s.Total > 0 {
			metrics["usage"] = float64(s.Used) / float64(s.Total) * 100
		}
		resourceMetrics[s.ID] = metrics
		resourceMetrics[s.Name] = metrics
	}

	var validated []*Finding
	for _, f := range findings {
		if f == nil {
			continue
		}

		// Check if this finding is actionable based on actual metrics
		if !p.isActionableFinding(f, resourceMetrics) {
			log.Debug().
				Str("finding_id", f.ID).
				Str("title", f.Title).
				Str("resource", f.ResourceName).
				Msg("AI Patrol: Filtering out low-confidence finding")
			continue
		}

		validated = append(validated, f)
	}

	if filtered := len(findings) - len(validated); filtered > 0 {
		log.Info().
			Int("original", len(findings)).
			Int("validated", len(validated)).
			Int("filtered", filtered).
			Msg("AI Patrol: Filtered noisy findings from AI response")
	}

	return validated
}

// isActionableFinding determines if a finding is worth showing to users
// Returns false for findings that would just be noise
func (p *PatrolService) isActionableFinding(f *Finding, resourceMetrics map[string]map[string]float64) bool {
	// Always allow critical findings through
	if f.Severity == FindingSeverityCritical {
		return true
	}

	// Always allow backup-related findings (these are actionable)
	if f.Category == FindingCategoryBackup {
		return true
	}

	// Always allow reliability findings (offline, errors)
	if f.Category == FindingCategoryReliability {
		return true
	}

	// For performance/capacity findings, validate against actual metrics
	metrics, hasMetrics := resourceMetrics[f.ResourceID]
	if !hasMetrics {
		// Try by resource name
		metrics, hasMetrics = resourceMetrics[f.ResourceName]
	}

	// If we can't find metrics, allow the finding (benefit of doubt)
	if !hasMetrics {
		return true
	}

	// Check specific finding types against actual thresholds
	key := strings.ToLower(f.Key)

	// High CPU findings
	if strings.Contains(key, "cpu") || strings.Contains(strings.ToLower(f.Title), "cpu") {
		if cpu, ok := metrics["cpu"]; ok {
			// Reject CPU findings if actual CPU is below 50%
			// Even if it's "elevated" compared to baseline, <50% isn't actionable
			if cpu < 50.0 {
				return false
			}
		}
	}

	// High memory findings
	if strings.Contains(key, "memory") || strings.Contains(key, "mem") || strings.Contains(strings.ToLower(f.Title), "memory") {
		if mem, ok := metrics["memory"]; ok {
			// Reject memory findings if actual memory is below 60%
			if mem < 60.0 {
				return false
			}
		}
	}

	// High disk/storage findings
	if strings.Contains(key, "disk") || strings.Contains(key, "storage") || strings.Contains(strings.ToLower(f.Title), "disk") {
		disk, hasDisk := metrics["disk"]
		usage, hasUsage := metrics["usage"]

		if hasDisk && disk < 70.0 {
			return false // Disk at <70% isn't urgent
		}
		if hasUsage && usage < 70.0 {
			return false // Storage at <70% isn't urgent
		}
	}

	// Default: allow the finding
	return true
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
	case "backup":
		cat = FindingCategoryBackup
	case "configuration":
		cat = FindingCategoryGeneral // Configuration maps to General
	default:
		cat = FindingCategoryPerformance
	}

	// Generate stable ID from resource and category to collapse duplicate findings
	// across patrol runs, while preserving the normalized key for issue type context.
	normalizedKey := normalizeFindingKey(key)
	if normalizedKey == "" {
		// Fallback to title-based key if LLM didn't provide one
		normalizedKey = normalizeFindingKey(title)
		if normalizedKey == "" {
			normalizedKey = "llm-finding"
		}
	}
	id := generateFindingID(resource, string(cat), normalizedKey)

	return &Finding{
		ID:             id,
		Key:            normalizedKey,
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

// TriggerPatrolForAlert triggers an immediate patrol for a specific alert
func (p *PatrolService) TriggerPatrolForAlert(alert *alerts.Alert) {
	if alert == nil {
		return
	}

	p.mu.RLock()
	triggerManager := p.triggerManager
	p.mu.RUnlock()

	if triggerManager != nil {
		scope := PatrolScope{
			ResourceIDs: []string{alert.ResourceID},
			Depth:       PatrolDepthQuick,
			Reason:      TriggerReasonAlertFired,
			Context:     "Alert: " + alert.Type,
			Priority:    80,
			AlertID:     alert.ID,
		}
		if triggerManager.TriggerPatrol(scope) {
			log.Debug().Str("alert_id", alert.ID).Msg("Queued alert-triggered patrol via trigger manager")
		} else {
			log.Warn().Str("alert_id", alert.ID).Msg("Alert-triggered patrol rejected by trigger manager")
		}
		return
	}

	// Non-blocking send
	select {
	case p.adHocTrigger <- alert:
		log.Debug().Str("alert_id", alert.ID).Msg("Queued ad-hoc patrol trigger")
	default:
		log.Warn().Str("alert_id", alert.ID).Msg("Patrol trigger queue full, dropping trigger")
	}
}

func (p *PatrolService) tryStartRun(kind string) bool {
	p.mu.Lock()
	if p.runInProgress {
		p.mu.Unlock()
		log.Debug().Str("kind", kind).Msg("AI Patrol: Run already in progress, skipping")
		return false
	}
	p.runInProgress = true
	p.mu.Unlock()
	return true
}

func (p *PatrolService) endRun() {
	p.mu.Lock()
	p.runInProgress = false
	p.mu.Unlock()
}

// runTargetedPatrol executes a focused patrol for a specific alert
func (p *PatrolService) runTargetedPatrol(ctx context.Context, alert *alerts.Alert) {
	log.Info().
		Str("alert_id", alert.ID).
		Str("resource_id", alert.ResourceID).
		Msg("Running targeted AI patrol for alert")

	scope := PatrolScope{
		ResourceIDs: []string{alert.ResourceID},
		Depth:       PatrolDepthQuick,
		Reason:      TriggerReasonAlertFired,
		Context:     "Alert: " + alert.Type,
		Priority:    80,
		AlertID:     alert.ID,
	}
	p.TriggerScopedPatrol(ctx, scope)
}

// MaybeInvestigateFinding checks if a finding should be investigated and triggers investigation if so
// This is called both during scheduled patrol runs and when alert-triggered findings are created
func (p *PatrolService) MaybeInvestigateFinding(f *Finding) {
	p.mu.RLock()
	orchestrator := p.investigationOrchestrator
	aiService := p.aiService
	p.mu.RUnlock()

	// No orchestrator configured
	if orchestrator == nil {
		return
	}

	// Get autonomy level from AI config
	if aiService == nil {
		return
	}
	cfg := aiService.GetConfig()
	if cfg == nil {
		return
	}
	autonomyLevel := cfg.GetPatrolAutonomyLevel()

	// Check if finding should be investigated
	if !f.ShouldInvestigate(autonomyLevel) {
		return
	}

	// Check if we can start another investigation (concurrency limit)
	if !orchestrator.CanStartInvestigation() {
		log.Debug().
			Str("finding_id", f.ID).
			Msg("Cannot start investigation: max concurrent investigations reached")
		return
	}

	// Convert Finding to InvestigationFinding
	invFinding := &InvestigationFinding{
		ID:                     f.ID,
		Severity:               string(f.Severity),
		Category:               string(f.Category),
		ResourceID:             f.ResourceID,
		ResourceName:           f.ResourceName,
		ResourceType:           f.ResourceType,
		Title:                  f.Title,
		Description:            f.Description,
		Recommendation:         f.Recommendation,
		Evidence:               f.Evidence,
		InvestigationSessionID: f.InvestigationSessionID,
		InvestigationStatus:    f.InvestigationStatus,
		InvestigationOutcome:   f.InvestigationOutcome,
		LastInvestigatedAt:     f.LastInvestigatedAt,
		InvestigationAttempts:  f.InvestigationAttempts,
	}

	// Trigger investigation in background
	go func() {
		ctx := context.Background()
		if err := orchestrator.InvestigateFinding(ctx, invFinding, autonomyLevel); err != nil {
			log.Error().
				Err(err).
				Str("finding_id", f.ID).
				Msg("Failed to start investigation")
		}
	}()

	log.Info().
		Str("finding_id", f.ID).
		Str("severity", string(f.Severity)).
		Str("resource", f.ResourceName).
		Str("autonomy_level", autonomyLevel).
		Msg("Triggered autonomous investigation for finding")
}
