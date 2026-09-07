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
	// Strict resolution blocks - tracks when undiscovered resources are blocked
	strictResolutionBlock *prometheus.CounterVec

	// Routing mismatch blocks - tracks when operations target wrong layer
	routingMismatchBlock *prometheus.CounterVec

	// Phantom detection - tracks hallucinated tool execution claims
	phantomDetected *prometheus.CounterVec

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
		m.strictResolutionBlock,
		m.routingMismatchBlock,
		m.phantomDetected,
		m.agenticIterations,
	)

	return m
}

// RecordStrictResolutionBlock records when strict resolution blocks an action
// Note: tool should be the function name (e.g., "validateResolvedResource"), not user input
// Note: action should be a small enum (e.g., "restart", "exec"), not resource IDs
func (m *AIMetrics) RecordStrictResolutionBlock(tool, action string) {
	m.strictResolutionBlock.WithLabelValues(sanitizeLabel(tool), sanitizeLabel(action)).Inc()
}

// RecordRoutingMismatchBlock records when routing validation blocks an operation
// that targeted a parent host when the user recently referenced a child resource.
// Note: use small enums for kinds (node, system-container, vm, app-container), not resource IDs
func (m *AIMetrics) RecordRoutingMismatchBlock(tool, targetKind, childKind string) {
	m.routingMismatchBlock.WithLabelValues(sanitizeLabel(tool), sanitizeLabel(targetKind), sanitizeLabel(childKind)).Inc()
}

// RecordPhantomDetected records when phantom execution is detected
func (m *AIMetrics) RecordPhantomDetected(provider, model string) {
	m.phantomDetected.WithLabelValues(sanitizeLabel(provider), sanitizeLabel(model)).Inc()
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
	}
}

// RecordRoutingMismatchBlock implements tools.TelemetryCallback
func (c *AIMetricsTelemetryCallback) RecordRoutingMismatchBlock(tool, targetKind, childKind string) {
	if c.metrics != nil {
		c.metrics.RecordRoutingMismatchBlock(tool, targetKind, childKind)
	}
}
