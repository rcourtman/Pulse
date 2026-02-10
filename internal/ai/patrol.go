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
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/finding"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
	PMGChecked        int `json:"pmg_checked"`
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

// PushNotifyCallback is called to send a push notification through the relay.
type PushNotifyCallback func(notification relay.PushNotificationPayload)

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
	// Shutdown signals all running investigations to stop, persists state,
	// and waits for them to finish (up to the context deadline).
	Shutdown(ctx context.Context) error
}

// InvestigationStoreMaintainer is an optional interface for orchestrators that
// expose their investigation store for periodic maintenance.
type InvestigationStoreMaintainer interface {
	CleanupInvestigationStore(maxAge time.Duration, maxSessions int)
}

// InvestigationFinding is the shared finding type used by the investigation
// orchestrator. Type alias so *InvestigationFinding and *finding.Finding
// are interchangeable, eliminating field-by-field copies at the adapter layer.
type InvestigationFinding = finding.Finding

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
	guestProber         GuestProber             // For pre-patrol guest reachability checks
	metricsHistory      MetricsHistoryProvider  // For trend analysis and predictions
	baselineStore       *baseline.Store         // For anomaly detection via learned baselines
	changeDetector      *ChangeDetector         // For tracking infrastructure changes
	remediationLog      *RemediationLog         // For tracking remediation actions
	patternDetector     *PatternDetector        // For failure prediction from historical patterns
	correlationDetector *CorrelationDetector    // For multi-resource correlation
	incidentStore       *memory.IncidentStore   // For incident timeline capture
	alertResolver       AlertResolver           // For AI-based alert resolution

	// Unified resource provider — reads physical disks, Ceph, etc. from canonical model
	unifiedResourceProvider UnifiedResourceProvider
	// ReadState provides typed read-only views over resource state (VMs, nodes, hosts, etc.).
	// This is injected separately from stateProvider since stateProvider also contains
	// non-resource telemetry (alerts, backups, connection health) that isn't modeled as resources yet.
	readState unifiedresources.ReadState

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
	investigationWg           sync.WaitGroup // Tracks in-flight investigation goroutines

	// Unified findings callback - pushes findings to unified store
	unifiedFindingCallback UnifiedFindingCallback
	// Unified resolver callback - marks findings resolved in unified store
	unifiedFindingResolver func(findingID string)

	// Push notification callback - sends push via relay to mobile devices
	pushNotifyCallback PushNotifyCallback

	// Cached thresholds (recalculated when thresholdProvider changes)
	thresholds    PatrolThresholds
	proactiveMode bool // When true, warn before thresholds; when false, use exact thresholds

	// Runtime state
	running           bool
	runInProgress     bool
	runStartedAt      time.Time
	stopCh            chan struct{}
	configChanged     chan struct{} // Signal when config changes to reset ticker
	lastPatrol        time.Time
	lastDuration      time.Duration
	resourcesChecked  int
	errorCount        int
	lastBlockedReason string
	lastBlockedAt     time.Time
	nextScheduledAt   time.Time // Tracks actual next patrol time (accounts for ticker resets)

	// Patrol run history with persistence support
	runHistoryStore *PatrolRunHistoryStore

	// Ad-hoc trigger channel for event-driven patrols (alert driven)
	adHocTrigger chan *alerts.Alert

	// Live streaming support
	streamMu          sync.RWMutex
	streamSubscribers map[chan PatrolStreamEvent]*streamSubscriber
	currentOutput     streamOutputBuffer // Tail buffer for current streaming output
	streamPhase       string             // "idle", "analyzing", "complete"
	streamRunID       string             // Identifies the current streamed run (best-effort)
	streamSeq         int64              // Monotonic sequence for SSE events within streamRunID
	streamCurrentTool string             // Last observed tool name (best-effort)
	streamEvents      []PatrolStreamEvent
}

const patrolStreamMaxOutputBytes = 64 * 1024

// streamOutputBuffer retains only the most recent bytes written, to cap memory usage
// while still allowing late joiners to get a useful snapshot.
type streamOutputBuffer struct {
	buf       []byte
	truncated bool
}

func (b *streamOutputBuffer) Reset() {
	// Keep capacity bounded so long-running streams can't retain a large backing array.
	if cap(b.buf) > patrolStreamMaxOutputBytes {
		b.buf = make([]byte, 0, patrolStreamMaxOutputBytes)
	} else {
		b.buf = b.buf[:0]
	}
	b.truncated = false
}

func (b *streamOutputBuffer) Len() int             { return len(b.buf) }
func (b *streamOutputBuffer) String() string       { return string(b.buf) }
func (b *streamOutputBuffer) Truncated() bool      { return b.truncated }
func (b *streamOutputBuffer) WriteString(s string) { b.appendString(s) }

func (b *streamOutputBuffer) appendString(s string) {
	if len(s) == 0 {
		return
	}

	max := patrolStreamMaxOutputBytes
	if len(s) >= max {
		// Keep only the tail of the incoming chunk.
		b.buf = append(b.buf[:0], s[len(s)-max:]...)
		b.truncated = true
		b.normalizeUTF8Start()
		b.shrinkCapIfNeeded()
		return
	}

	// Keep as much of existing tail as possible.
	needKeep := max - len(s)
	if len(b.buf) > needKeep {
		b.buf = append(b.buf[:0], b.buf[len(b.buf)-needKeep:]...)
		b.truncated = true
	}
	b.buf = append(b.buf, s...)
	if len(b.buf) > max {
		b.buf = b.buf[len(b.buf)-max:]
		b.truncated = true
	}
	b.normalizeUTF8Start()
	b.shrinkCapIfNeeded()
}

func (b *streamOutputBuffer) normalizeUTF8Start() {
	// If we truncated by bytes, we may have cut in the middle of a UTF-8 rune.
	// Drop leading continuation bytes so the string starts on a rune boundary.
	for len(b.buf) > 0 && (b.buf[0]&0xC0) == 0x80 {
		b.buf = b.buf[1:]
		b.truncated = true
	}
}

func (b *streamOutputBuffer) shrinkCapIfNeeded() {
	if cap(b.buf) <= patrolStreamMaxOutputBytes {
		return
	}
	tmp := make([]byte, len(b.buf), patrolStreamMaxOutputBytes)
	copy(tmp, b.buf)
	b.buf = tmp
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

// streamSubscriber wraps a stream channel with an atomic close flag
// to prevent double-close panics when both broadcast and unsubscribe race.
type streamSubscriber struct {
	ch     chan PatrolStreamEvent
	closed atomic.Bool
	// Consecutive times we couldn't deliver to this subscriber because its channel
	// was full. Used to tolerate short bursts without immediately disconnecting.
	fullCount int
}

// PatrolStreamEvent represents a streaming update from the patrol
type PatrolStreamEvent struct {
	// Meta
	// Seq is suitable to be used as an SSE "id:" for Last-Event-ID, but replay is best-effort.
	RunID string `json:"run_id,omitempty"`
	Seq   int64  `json:"seq,omitempty"`
	TsMs  int64  `json:"ts_ms,omitempty"`
	// If this is a synthetic snapshot/resync event, why it was emitted.
	// Examples: "late_joiner", "stale_last_event_id".
	ResyncReason string `json:"resync_reason,omitempty"`
	BufferStart  int64  `json:"buffer_start_seq,omitempty"`
	BufferEnd    int64  `json:"buffer_end_seq,omitempty"`
	// True when the snapshot content has been truncated due to the tail buffer.
	ContentTruncated *bool `json:"content_truncated,omitempty"`

	// Payload
	// Known types include: "snapshot", "start", "content", "phase", "thinking",
	// "complete", "error", "tool_start", "tool_end".
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Phase   string `json:"phase,omitempty"`  // Current phase description
	Tokens  int    `json:"tokens,omitempty"` // Token count so far
	// Tool event fields (present only for tool_start/tool_end)
	ToolID       string `json:"tool_id,omitempty"`
	ToolName     string `json:"tool_name,omitempty"`
	ToolInput    string `json:"tool_input,omitempty"`
	ToolRawInput string `json:"tool_raw_input,omitempty"`
	ToolOutput   string `json:"tool_output,omitempty"`
	ToolSuccess  *bool  `json:"tool_success,omitempty"` // pointer so omitempty works with false
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
		streamSubscribers: make(map[chan PatrolStreamEvent]*streamSubscriber),
		streamPhase:       "idle",
		adHocTrigger:      make(chan *alerts.Alert, 10), // Buffer triggers
	}
}
