// patrol.go defines the core types, interfaces, and struct for the PatrolService.
//
// Architecture:
//
//	Scheduled/Event Trigger
//	        │
//	        ▼
//	buildSeedContext()  ── infrastructure snapshot
//	        │
//	        ▼
//	runAIAnalysis()     ── agentic LLM loop with tools
//	        │
//	        ▼
//	recordFinding()     ── dedup, threshold validation
//	        │
//	        ├──▶ MaybeInvestigateFinding()  ── autonomous investigation
//	        │         │
//	        │         ▼
//	        │    parseInvestigationSummary() ── extract PROPOSED_FIX
//	        │         │
//	        │         ▼
//	        │    approval / execution / verification
//	        │
//	        └──▶ generateRemediationPlan()  ── template-based fix plan
//
// Safety: All command execution goes through internal/ai/safety for
// blocked command detection. Investigation guardrails and remediation
// engine both delegate to the shared safety package.
//
// The patrol system is split across these files:
//   - patrol.go: types, interfaces, PatrolService struct, constructor
//   - patrol_init.go: configuration, setters, getters, dependency injection
//   - patrol_run.go: lifecycle, scheduling, streaming, alert resolution
//   - patrol_ai.go: LLM interaction, seed context, prompt construction
//   - patrol_findings.go: finding lifecycle, remediation planning, investigation
//   - patrol_signals.go: deterministic signal detection from tool call outputs
//   - patrol_metrics.go: metrics collection for patrol operations
package ai

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
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
	BlockedReason    string        `json:"blocked_reason,omitempty"`
	BlockedAt        *time.Time    `json:"blocked_at,omitempty"`
}

// PatrolRunRecord represents a single patrol check run
type PatrolRunRecord struct {
	ID                 string        `json:"id"`
	StartedAt          time.Time     `json:"started_at"`
	CompletedAt        time.Time     `json:"completed_at"`
	Duration           time.Duration `json:"-"`
	DurationMs         int64         `json:"duration_ms"`
	Type               string        `json:"type"` // Always "patrol" now (kept for backwards compat)
	TriggerReason      string        `json:"trigger_reason,omitempty"`
	ScopeResourceIDs   []string      `json:"scope_resource_ids,omitempty"`
	ScopeResourceTypes []string      `json:"scope_resource_types,omitempty"`
	ScopeContext       string        `json:"scope_context,omitempty"`
	AlertID            string        `json:"alert_id,omitempty"`
	FindingID          string        `json:"finding_id,omitempty"`
	ResourcesChecked   int           `json:"resources_checked"`
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
	RejectedFindings int      `json:"rejected_findings"`
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
	// Tool call traces
	ToolCalls     []ToolCallRecord `json:"tool_calls,omitempty"`
	ToolCallCount int              `json:"tool_call_count"`
}

// MaxPatrolRunHistory is the maximum number of patrol runs to keep in history
const MaxPatrolRunHistory = 100

const (
	MaxToolInputSize   = 1024 // max chars for tool input in persisted record
	MaxToolOutputSize  = 2048 // max chars for tool output in persisted record
	MaxToolCallsPerRun = 100  // max tool calls stored per run (oldest dropped)
)

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
	Key                    string
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
	ID             string            `json:"id"`
	FindingID      string            `json:"finding_id"`
	SessionID      string            `json:"session_id"`
	Status         string            `json:"status"`
	StartedAt      time.Time         `json:"started_at"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
	TurnCount      int               `json:"turn_count"`
	Outcome        string            `json:"outcome,omitempty"`
	ToolsAvailable []string          `json:"tools_available,omitempty"`
	ToolsUsed      []string          `json:"tools_used,omitempty"`
	EvidenceIDs    []string          `json:"evidence_ids,omitempty"`
	Summary        string            `json:"summary,omitempty"`
	Error          string            `json:"error,omitempty"`
	ProposedFix    *InvestigationFix `json:"proposed_fix,omitempty"`
	ApprovalID     string            `json:"approval_id,omitempty"`
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
	knowledgeStore      *knowledge.Store        // For per-resource notes in patrol context
	discoveryStore      *servicediscovery.Store // For AI-discovered infrastructure context
	metricsHistory      MetricsHistoryProvider  // For trend analysis and predictions
	baselineStore       *baseline.Store         // For anomaly detection via learned baselines
	changeDetector      *ChangeDetector         // For tracking infrastructure changes
	remediationLog      *RemediationLog         // For tracking remediation actions
	patternDetector     *PatternDetector        // For failure prediction from historical patterns
	correlationDetector *CorrelationDetector    // For multi-resource correlation
	incidentStore       *memory.IncidentStore   // For incident timeline capture
	alertResolver       AlertResolver           // For AI-based alert resolution

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
	// Unified resolver callback - marks findings resolved in unified store
	unifiedFindingResolver func(findingID string)

	// Cached thresholds (recalculated when thresholdProvider changes)
	thresholds    PatrolThresholds
	proactiveMode bool // When true, warn before thresholds; when false, use exact thresholds

	// Runtime state
	running           bool
	runInProgress     bool
	stopCh            chan struct{}
	configChanged     chan struct{} // Signal when config changes to reset ticker
	lastPatrol        time.Time
	lastDuration      time.Duration
	resourcesChecked  int
	errorCount        int
	lastBlockedReason string
	lastBlockedAt     time.Time

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

// ToolCallRecord captures a single tool invocation during a patrol run.
type ToolCallRecord struct {
	ID        string `json:"id"`
	ToolName  string `json:"tool_name"`
	Input     string `json:"input"`
	Output    string `json:"output"`
	Success   bool   `json:"success"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
	Duration  int64  `json:"duration_ms"`
}

// PatrolStreamEvent represents a streaming update from the patrol
type PatrolStreamEvent struct {
	Type    string `json:"type"` // "start", "content", "phase", "complete", "error", "tool_start", "tool_end"
	Content string `json:"content,omitempty"`
	Phase   string `json:"phase,omitempty"`  // Current phase description
	Tokens  int    `json:"tokens,omitempty"` // Token count so far
	// Tool event fields (present only for tool_start/tool_end)
	ToolID      string `json:"tool_id,omitempty"`
	ToolName    string `json:"tool_name,omitempty"`
	ToolInput   string `json:"tool_input,omitempty"`
	ToolOutput  string `json:"tool_output,omitempty"`
	ToolSuccess *bool  `json:"tool_success,omitempty"` // pointer so omitempty works with false
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
