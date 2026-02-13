package monitoring

import (
	stdErrors "errors"
	"strings"

	internalerrors "github.com/rcourtman/pulse-go-rewrite/internal/errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PollMetrics manages Prometheus instrumentation for polling activity.
type PollMetrics struct {
	pollDuration                 *prometheus.HistogramVec
	pollResults                  *prometheus.CounterVec
	pollErrors                   *prometheus.CounterVec
	lastSuccess                  *prometheus.GaugeVec
	staleness                    *prometheus.GaugeVec
	queueDepth                   prometheus.Gauge
	inflight                     *prometheus.GaugeVec
	nodePollDuration             *prometheus.HistogramVec
	nodePollResults              *prometheus.CounterVec
	nodePollErrors               *prometheus.CounterVec
	nodeLastSuccess              *prometheus.GaugeVec
	nodeStaleness                *prometheus.GaugeVec
	schedulerQueueReady          prometheus.Gauge
	schedulerQueueDepthByType    *prometheus.GaugeVec
	schedulerQueueWait           *prometheus.HistogramVec
	schedulerDeadLetterDepth     *prometheus.GaugeVec
	schedulerBreakerState        *prometheus.GaugeVec
	schedulerBreakerFailureCount *prometheus.GaugeVec
	schedulerBreakerRetrySeconds *prometheus.GaugeVec

	mu                   sync.RWMutex
	lastSuccessByKey     map[metricKey]time.Time
	nodeLastSuccessByKey map[nodeMetricKey]time.Time
	lastQueueTypeKeys    map[string]struct{}
	lastDLQKeys          map[string]struct{}
	pending              int
}

type metricKey struct {
	instanceType string
	instance     string
}

type nodeMetricKey struct {
	instanceType string
	instance     string
	node         string
}

var (
	pollMetricsInstance *PollMetrics
	pollMetricsOnce     sync.Once
)

func getPollMetrics() *PollMetrics {
	pollMetricsOnce.Do(func() {
		pollMetricsInstance = newPollMetrics()
	})
	return pollMetricsInstance
}

func newPollMetrics() *PollMetrics {
	pm := &PollMetrics{
		pollDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_duration_seconds",
				Help:      "Duration of polling operations per instance.",
				Buckets:   []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 30},
			},
			[]string{"instance_type", "instance"},
		),
		pollResults: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_total",
				Help:      "Total polling attempts partitioned by result.",
			},
			[]string{"instance_type", "instance", "result"},
		),
		pollErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_errors_total",
				Help:      "Polling failures grouped by error type.",
			},
			[]string{"instance_type", "instance", "error_type"},
		),
		lastSuccess: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_last_success_timestamp",
				Help:      "Unix timestamp of the last successful poll.",
			},
			[]string{"instance_type", "instance"},
		),
		staleness: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_staleness_seconds",
				Help:      "Seconds since the last successful poll. -1 indicates no successes yet.",
			},
			[]string{"instance_type", "instance"},
		),
		queueDepth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_queue_depth",
				Help:      "Approximate number of poll tasks waiting to complete in the current cycle.",
			},
		),
		inflight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_inflight",
				Help:      "Current number of poll operations executing per instance type.",
			},
			[]string{"instance_type"},
		),
		nodePollDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "node_poll_duration_seconds",
				Help:      "Duration of polling operations per node.",
				Buckets:   []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 30},
			},
			[]string{"instance_type", "instance", "node"},
		),
		nodePollResults: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "node_poll_total",
				Help:      "Total polling attempts per node partitioned by result.",
			},
			[]string{"instance_type", "instance", "node", "result"},
		),
		nodePollErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "node_poll_errors_total",
				Help:      "Polling failures per node grouped by error type.",
			},
			[]string{"instance_type", "instance", "node", "error_type"},
		),
		nodeLastSuccess: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "node_poll_last_success_timestamp",
				Help:      "Unix timestamp of the last successful poll for a node.",
			},
			[]string{"instance_type", "instance", "node"},
		),
		nodeStaleness: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "node_poll_staleness_seconds",
				Help:      "Seconds since the last successful poll for a node. -1 indicates no successes yet.",
			},
			[]string{"instance_type", "instance", "node"},
		),
		schedulerQueueReady: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "queue_due_soon",
				Help:      "Number of tasks due to run within the immediate window (12s).",
			},
		),
		schedulerQueueDepthByType: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "queue_depth",
				Help:      "Current scheduler queue depth partitioned by instance type.",
			},
			[]string{"instance_type"},
		),
		schedulerQueueWait: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "queue_wait_seconds",
				Help:      "Observed wait time between task readiness and execution.",
				Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
			},
			[]string{"instance_type"},
		),
		schedulerDeadLetterDepth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "dead_letter_depth",
				Help:      "Number of tasks currently parked in the dead-letter queue per instance.",
			},
			[]string{"instance_type", "instance"},
		),
		schedulerBreakerState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "breaker_state",
				Help:      "Circuit breaker state encoded as 0=closed, 1=half-open, 2=open, -1=unknown.",
			},
			[]string{"instance_type", "instance"},
		),
		schedulerBreakerFailureCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "breaker_failure_count",
				Help:      "Current consecutive failure count tracked by the circuit breaker.",
			},
			[]string{"instance_type", "instance"},
		),
		schedulerBreakerRetrySeconds: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "breaker_retry_seconds",
				Help:      "Seconds until the circuit breaker will allow another attempt.",
			},
			[]string{"instance_type", "instance"},
		),
		lastSuccessByKey:     make(map[metricKey]time.Time),
		nodeLastSuccessByKey: make(map[nodeMetricKey]time.Time),
		lastQueueTypeKeys:    make(map[string]struct{}),
		lastDLQKeys:          make(map[string]struct{}),
	}

	prometheus.MustRegister(
		pm.pollDuration,
		pm.pollResults,
		pm.pollErrors,
		pm.lastSuccess,
		pm.staleness,
		pm.queueDepth,
		pm.inflight,
		pm.nodePollDuration,
		pm.nodePollResults,
		pm.nodePollErrors,
		pm.nodeLastSuccess,
		pm.nodeStaleness,
		pm.schedulerQueueReady,
		pm.schedulerQueueDepthByType,
		pm.schedulerQueueWait,
		pm.schedulerDeadLetterDepth,
		pm.schedulerBreakerState,
		pm.schedulerBreakerFailureCount,
		pm.schedulerBreakerRetrySeconds,
	)

	return pm
}

// NodePollResult captures timing and outcome for a specific node within a poll cycle.
type NodePollResult struct {
	InstanceName string
	InstanceType string
	NodeName     string
	Success      bool
	Error        error
	StartTime    time.Time
	EndTime      time.Time
}

// RecordNodeResult records metrics for an individual node poll.
func (pm *PollMetrics) RecordNodeResult(result NodePollResult) {
	if pm == nil {
		return
	}

	instType, inst := sanitizeInstanceLabels(result.InstanceType, result.InstanceName)
	nodeLabel := normalizeNodeLabel(result.NodeName)

	duration := result.EndTime.Sub(result.StartTime).Seconds()
	if duration < 0 {
		duration = 0
	}
	pm.nodePollDuration.WithLabelValues(instType, inst, nodeLabel).Observe(duration)

	resultValue := "success"
	if !result.Success {
		resultValue = "error"
	}
	pm.nodePollResults.WithLabelValues(instType, inst, nodeLabel, resultValue).Inc()

	if result.Success {
		pm.nodeLastSuccess.WithLabelValues(instType, inst, nodeLabel).Set(float64(result.EndTime.Unix()))
		pm.storeNodeLastSuccess(instType, inst, nodeLabel, result.EndTime)
		pm.updateNodeStaleness(instType, inst, nodeLabel, 0)
		return
	}

	errType := pm.classifyError(result.Error)
	pm.nodePollErrors.WithLabelValues(instType, inst, nodeLabel, errType).Inc()

	if last, ok := pm.lastNodeSuccessFor(instType, inst, nodeLabel); ok && !last.IsZero() {
		staleness := result.EndTime.Sub(last).Seconds()
		if staleness < 0 {
			staleness = 0
		}
		pm.updateNodeStaleness(instType, inst, nodeLabel, staleness)
	} else {
		pm.updateNodeStaleness(instType, inst, nodeLabel, -1)
	}
}

// RecordQueueWait observes the time a task spent waiting in the scheduler queue.
func (pm *PollMetrics) RecordQueueWait(instanceType string, wait time.Duration) {
	if pm == nil {
		return
	}
	if wait < 0 {
		wait = 0
	}
	label := normalizeLabel(instanceType)
	pm.schedulerQueueWait.WithLabelValues(label).Observe(wait.Seconds())
}

// UpdateQueueSnapshot updates scheduler queue depth metrics.
func (pm *PollMetrics) UpdateQueueSnapshot(snapshot QueueSnapshot) {
	if pm == nil {
		return
	}

	pm.schedulerQueueReady.Set(float64(snapshot.DueWithinSeconds))

	current := make(map[string]struct{}, len(snapshot.PerType))
	for instanceType, depth := range snapshot.PerType {
		key := normalizeLabel(instanceType)
		pm.schedulerQueueDepthByType.WithLabelValues(key).Set(float64(depth))
		current[key] = struct{}{}
	}

	pm.mu.Lock()
	for key := range pm.lastQueueTypeKeys {
		if _, ok := current[key]; !ok {
			pm.schedulerQueueDepthByType.WithLabelValues(key).Set(0)
		}
	}
	pm.lastQueueTypeKeys = current
	pm.mu.Unlock()
}

// UpdateDeadLetterCounts refreshes dead-letter queue gauges based on the provided tasks.
func (pm *PollMetrics) UpdateDeadLetterCounts(tasks []DeadLetterTask) {
	if pm == nil {
		return
	}

	current := make(map[string]float64)
	for _, task := range tasks {
		instType := normalizeLabel(task.Type)
		inst := normalizeLabel(task.Instance)
		key := instType + "::" + inst
		current[key] = current[key] + 1
	}

	pm.mu.Lock()
	prev := pm.lastDLQKeys
	pm.lastDLQKeys = make(map[string]struct{}, len(current))
	pm.mu.Unlock()

	for key, count := range current {
		instType, inst := splitInstanceKey(key)
		pm.schedulerDeadLetterDepth.WithLabelValues(instType, inst).Set(count)
	}

	pm.mu.Lock()
	for key := range current {
		pm.lastDLQKeys[key] = struct{}{}
	}
	for key := range prev {
		if _, ok := current[key]; !ok {
			instType, inst := splitInstanceKey(key)
			pm.schedulerDeadLetterDepth.WithLabelValues(instType, inst).Set(0)
		}
	}
	pm.mu.Unlock()
}

// SetBreakerState updates circuit breaker metrics for a specific instance.
func (pm *PollMetrics) SetBreakerState(instanceType, instance, state string, failures int, retryAt time.Time) {
	if pm == nil {
		return
	}

	instType, inst := sanitizeInstanceLabels(instanceType, instance)

	value := breakerStateToValue(state)
	pm.schedulerBreakerState.WithLabelValues(instType, inst).Set(value)
	pm.schedulerBreakerFailureCount.WithLabelValues(instType, inst).Set(float64(failures))

	retrySeconds := 0.0
	if !retryAt.IsZero() {
		retrySeconds = time.Until(retryAt).Seconds()
		if retrySeconds < 0 {
			retrySeconds = 0
		}
	}
	pm.schedulerBreakerRetrySeconds.WithLabelValues(instType, inst).Set(retrySeconds)
}

// RecordResult records metrics for a polling result.
func (pm *PollMetrics) RecordResult(result PollResult) {
	if pm == nil {
		return
	}

	instType, inst := sanitizeInstanceLabels(result.InstanceType, result.InstanceName)

	duration := result.EndTime.Sub(result.StartTime).Seconds()
	if duration < 0 {
		duration = 0
	}
	pm.pollDuration.WithLabelValues(instType, inst).Observe(duration)

	resultValue := "success"
	if !result.Success {
		resultValue = "error"
	}
	pm.pollResults.WithLabelValues(instType, inst, resultValue).Inc()

	if result.Success {
		pm.lastSuccess.WithLabelValues(instType, inst).Set(float64(result.EndTime.Unix()))
		pm.storeLastSuccess(instType, inst, result.EndTime)
		pm.updateStaleness(instType, inst, 0)
	} else {
		errType := pm.classifyError(result.Error)
		pm.pollErrors.WithLabelValues(instType, inst, errType).Inc()

		if last, ok := pm.lastSuccessFor(instType, inst); ok && !last.IsZero() {
			staleness := result.EndTime.Sub(last).Seconds()
			if staleness < 0 {
				staleness = 0
			}
			pm.updateStaleness(instType, inst, staleness)
		} else {
			pm.updateStaleness(instType, inst, -1)
		}
	}

	pm.decrementPending()
}

// ResetQueueDepth sets the pending queue depth for the next polling cycle.
func (pm *PollMetrics) ResetQueueDepth(total int) {
	if pm == nil {
		return
	}
	if total < 0 {
		total = 0
	}

	pm.mu.Lock()
	pm.pending = total
	pm.mu.Unlock()
	pm.queueDepth.Set(float64(total))
}

// SetQueueDepth allows direct gauge control when needed.
func (pm *PollMetrics) SetQueueDepth(depth int) {
	if pm == nil {
		return
	}
	if depth < 0 {
		depth = 0
	}
	pm.queueDepth.Set(float64(depth))
}

// IncInFlight increments the in-flight gauge for the given instance type.
func (pm *PollMetrics) IncInFlight(instanceType string) {
	if pm == nil {
		return
	}
	pm.inflight.WithLabelValues(instanceType).Inc()
}

// DecInFlight decrements the in-flight gauge for the given instance type.
func (pm *PollMetrics) DecInFlight(instanceType string) {
	if pm == nil {
		return
	}
	pm.inflight.WithLabelValues(instanceType).Dec()
}

func (pm *PollMetrics) decrementPending() {
	if pm == nil {
		return
	}

	pm.mu.Lock()
	if pm.pending > 0 {
		pm.pending--
	}
	current := pm.pending
	pm.mu.Unlock()

	pm.queueDepth.Set(float64(current))
}

func (pm *PollMetrics) storeLastSuccess(instanceType, instance string, ts time.Time) {
	pm.mu.Lock()
	pm.lastSuccessByKey[makeMetricKey(instanceType, instance)] = ts
	pm.mu.Unlock()
}

func (pm *PollMetrics) lastSuccessFor(instanceType, instance string) (time.Time, bool) {
	pm.mu.RLock()
	ts, ok := pm.lastSuccessByKey[makeMetricKey(instanceType, instance)]
	pm.mu.RUnlock()
	return ts, ok
}

func (pm *PollMetrics) updateStaleness(instanceType, instance string, value float64) {
	instType, inst := sanitizeInstanceLabels(instanceType, instance)
	pm.staleness.WithLabelValues(instType, inst).Set(value)
}

func (pm *PollMetrics) storeNodeLastSuccess(instanceType, instance, node string, ts time.Time) {
	pm.mu.Lock()
	pm.nodeLastSuccessByKey[makeNodeMetricKey(instanceType, instance, node)] = ts
	pm.mu.Unlock()
}

func (pm *PollMetrics) lastNodeSuccessFor(instanceType, instance, node string) (time.Time, bool) {
	pm.mu.RLock()
	ts, ok := pm.nodeLastSuccessByKey[makeNodeMetricKey(instanceType, instance, node)]
	pm.mu.RUnlock()
	return ts, ok
}

func (pm *PollMetrics) updateNodeStaleness(instanceType, instance, node string, value float64) {
	instType, inst := sanitizeInstanceLabels(instanceType, instance)
	nodeLabel := normalizeLabel(node)
	pm.nodeStaleness.WithLabelValues(instType, inst, nodeLabel).Set(value)
}

func splitInstanceKey(key string) (string, string) {
	parts := strings.SplitN(key, "::", 2)
	if len(parts) == 2 {
		return normalizeLabel(parts[0]), normalizeLabel(parts[1])
	}
	if key == "" {
		return "unknown", "unknown"
	}
	return "unknown", normalizeLabel(key)
}

func breakerStateToValue(state string) float64 {
	switch strings.ToLower(state) {
	case "closed":
		return 0
	case "half_open", "half-open":
		return 1
	case "open":
		return 2
	default:
		return -1
	}
}

func sanitizeInstanceLabels(instanceType, instance string) (string, string) {
	return normalizeLabel(instanceType), normalizeLabel(instance)
}

func makeMetricKey(instanceType, instance string) metricKey {
	instType, inst := sanitizeInstanceLabels(instanceType, instance)
	return metricKey{
		instanceType: instType,
		instance:     inst,
	}
}

func makeNodeMetricKey(instanceType, instance, node string) nodeMetricKey {
	instType, inst := sanitizeInstanceLabels(instanceType, instance)
	return nodeMetricKey{
		instanceType: instType,
		instance:     inst,
		node:         normalizeLabel(node),
	}
}

func normalizeLabel(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return "unknown"
	}
	return v
}

func normalizeNodeLabel(value string) string {
	label := normalizeLabel(value)
	if label == "unknown" {
		return "unknown-node"
	}
	return label
}

func (pm *PollMetrics) classifyError(err error) string {
	if err == nil {
		return "none"
	}

	var monitorErr *internalerrors.MonitorError
	if stdErrors.As(err, &monitorErr) {
		return string(monitorErr.Type)
	}

	return "unknown"
}
