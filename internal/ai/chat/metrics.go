package chat

import (
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// maxLabelLen is the maximum length for a metric label value
const maxLabelLen = 64

// sanitizeLabel ensures a label value is safe for Prometheus:
// - Truncates to maxLabelLen
// - Replaces spaces with underscores
// - Returns "unknown" for empty values
func sanitizeLabel(s string) string {
	if s == "" {
		return "unknown"
	}
	s = strings.ReplaceAll(s, " ", "_")
	if len(s) > maxLabelLen {
		s = s[:maxLabelLen]
	}
	return s
}

// AIMetrics manages Prometheus instrumentation for AI chat safety/reliability.
// These metrics help prove the structural guarantees stay fixed over time.
type AIMetrics struct {
	// FSM blocks - tracks when workflow gates prevent unsafe actions
	fsmToolBlock  *prometheus.CounterVec
	fsmFinalBlock *prometheus.CounterVec

	// Strict resolution blocks - tracks when undiscovered resources are blocked
	strictResolutionBlock *prometheus.CounterVec

	// Routing mismatch blocks - tracks when operations target wrong layer
	routingMismatchBlock *prometheus.CounterVec

	// Phantom detection - tracks hallucinated tool execution claims
	phantomDetected *prometheus.CounterVec

	// Auto-recovery - tracks self-healing attempts and outcomes
	autoRecoveryAttempt *prometheus.CounterVec
	autoRecoverySuccess *prometheus.CounterVec

	// Loop health - tracks agentic loop iterations
	agenticIterations *prometheus.CounterVec
}

var (
	aiMetricsInstance *AIMetrics
	aiMetricsOnce     sync.Once
)

// GetAIMetrics returns the singleton AI metrics instance.
// Call this to record metrics from anywhere in the chat package.
func GetAIMetrics() *AIMetrics {
	aiMetricsOnce.Do(func() {
		aiMetricsInstance = newAIMetrics()
	})
	return aiMetricsInstance
}

func newAIMetrics() *AIMetrics {
	m := &AIMetrics{
		fsmToolBlock: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "ai",
				Name:      "fsm_tool_block_total",
				Help:      "Total FSM blocks of tool execution by state, tool, and kind",
			},
			[]string{"state", "tool", "kind"},
		),
		fsmFinalBlock: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "ai",
				Name:      "fsm_final_block_total",
				Help:      "Total FSM blocks of final answer by state",
			},
			[]string{"state"},
		),
		strictResolutionBlock: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "ai",
				Name:      "strict_resolution_block_total",
				Help:      "Total strict resolution blocks by tool and action",
			},
			[]string{"tool", "action"},
		),
		routingMismatchBlock: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "ai",
				Name:      "routing_mismatch_block_total",
				Help:      "Total routing mismatch blocks when targeting parent host instead of child resource",
			},
			[]string{"tool", "target_kind", "child_kind"},
		),
		phantomDetected: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "ai",
				Name:      "phantom_detected_total",
				Help:      "Total phantom execution detections by provider and model",
			},
			[]string{"provider", "model"},
		),
		autoRecoveryAttempt: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "ai",
				Name:      "auto_recovery_attempt_total",
				Help:      "Total auto-recovery attempts by error code and tool",
			},
			[]string{"error_code", "tool"},
		),
		autoRecoverySuccess: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "ai",
				Name:      "auto_recovery_success_total",
				Help:      "Total successful auto-recoveries by error code and tool",
			},
			[]string{"error_code", "tool"},
		),
		agenticIterations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "ai",
				Name:      "agentic_iterations_total",
				Help:      "Total agentic loop iterations by provider and model",
			},
			[]string{"provider", "model"},
		),
	}

	// Register all metrics
	prometheus.MustRegister(
		m.fsmToolBlock,
		m.fsmFinalBlock,
		m.strictResolutionBlock,
		m.routingMismatchBlock,
		m.phantomDetected,
		m.autoRecoveryAttempt,
		m.autoRecoverySuccess,
		m.agenticIterations,
	)

	return m
}

// RecordFSMToolBlock records when FSM blocks a tool execution
func (m *AIMetrics) RecordFSMToolBlock(state SessionState, tool string, kind ToolKind) {
	m.fsmToolBlock.WithLabelValues(string(state), sanitizeLabel(tool), kind.String()).Inc()
}

// RecordFSMFinalBlock records when FSM blocks a final answer
func (m *AIMetrics) RecordFSMFinalBlock(state SessionState) {
	m.fsmFinalBlock.WithLabelValues(string(state)).Inc()
}

// RecordStrictResolutionBlock records when strict resolution blocks an action
// Note: tool should be the function name (e.g., "validateResolvedResource"), not user input
// Note: action should be a small enum (e.g., "restart", "exec"), not resource IDs
func (m *AIMetrics) RecordStrictResolutionBlock(tool, action string) {
	m.strictResolutionBlock.WithLabelValues(sanitizeLabel(tool), sanitizeLabel(action)).Inc()
}

// RecordRoutingMismatchBlock records when routing validation blocks an operation
// that targeted a parent host when the user recently referenced a child resource.
// Note: use small enums for kinds (node, lxc, vm, docker_container), not resource IDs
func (m *AIMetrics) RecordRoutingMismatchBlock(tool, targetKind, childKind string) {
	m.routingMismatchBlock.WithLabelValues(sanitizeLabel(tool), sanitizeLabel(targetKind), sanitizeLabel(childKind)).Inc()
}

// RecordPhantomDetected records when phantom execution is detected
func (m *AIMetrics) RecordPhantomDetected(provider, model string) {
	m.phantomDetected.WithLabelValues(sanitizeLabel(provider), sanitizeLabel(model)).Inc()
}

// RecordAutoRecoveryAttempt records an auto-recovery attempt.
// Definition: "we returned a recoverable error that the model can self-correct"
func (m *AIMetrics) RecordAutoRecoveryAttempt(errorCode, tool string) {
	m.autoRecoveryAttempt.WithLabelValues(sanitizeLabel(errorCode), sanitizeLabel(tool)).Inc()
}

// RecordAutoRecoverySuccess records a successful auto-recovery.
// Definition: "a previously blocked operation succeeded on retry after discovery"
func (m *AIMetrics) RecordAutoRecoverySuccess(errorCode, tool string) {
	m.autoRecoverySuccess.WithLabelValues(sanitizeLabel(errorCode), sanitizeLabel(tool)).Inc()
}

// RecordAgenticIteration records an agentic loop iteration (one LLM call).
// This counts each turn in the agentic loop, not each tool call.
func (m *AIMetrics) RecordAgenticIteration(provider, model string) {
	m.agenticIterations.WithLabelValues(sanitizeLabel(provider), sanitizeLabel(model)).Inc()
}

// AIMetricsTelemetryCallback adapts AIMetrics to the tools.TelemetryCallback interface.
// This allows the tools package to record telemetry without importing the chat package.
type AIMetricsTelemetryCallback struct {
	metrics *AIMetrics
}

// NewAIMetricsTelemetryCallback creates a new telemetry callback adapter.
func NewAIMetricsTelemetryCallback() *AIMetricsTelemetryCallback {
	return &AIMetricsTelemetryCallback{
		metrics: GetAIMetrics(),
	}
}

// RecordStrictResolutionBlock implements tools.TelemetryCallback
func (c *AIMetricsTelemetryCallback) RecordStrictResolutionBlock(tool, action string) {
	if c.metrics != nil {
		c.metrics.RecordStrictResolutionBlock(tool, action)
		// Strict resolution blocks are recoverable (model can discover then retry)
		c.metrics.RecordAutoRecoveryAttempt("STRICT_RESOLUTION", tool)
	}
}

// RecordAutoRecoveryAttempt implements tools.TelemetryCallback
func (c *AIMetricsTelemetryCallback) RecordAutoRecoveryAttempt(errorCode, tool string) {
	if c.metrics != nil {
		c.metrics.RecordAutoRecoveryAttempt(errorCode, tool)
	}
}

// RecordAutoRecoverySuccess implements tools.TelemetryCallback
func (c *AIMetricsTelemetryCallback) RecordAutoRecoverySuccess(errorCode, tool string) {
	if c.metrics != nil {
		c.metrics.RecordAutoRecoverySuccess(errorCode, tool)
	}
}

// RecordRoutingMismatchBlock implements tools.TelemetryCallback
func (c *AIMetricsTelemetryCallback) RecordRoutingMismatchBlock(tool, targetKind, childKind string) {
	if c.metrics != nil {
		c.metrics.RecordRoutingMismatchBlock(tool, targetKind, childKind)
		// Routing mismatch blocks are recoverable (model can retry with correct target)
		c.metrics.RecordAutoRecoveryAttempt("ROUTING_MISMATCH", tool)
	}
}
