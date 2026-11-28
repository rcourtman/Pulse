package monitoring

import (
	stdErrors "errors"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	internalerrors "github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
)

// PollMetrics manages Prometheus instrumentation for polling activity.
type PollMetrics struct {
	pollDuration              *prometheus.HistogramVec
	pollResults               *prometheus.CounterVec
	pollErrors                *prometheus.CounterVec
	queueDepth                prometheus.Gauge
	inflight                  *prometheus.GaugeVec
	nodePollDuration          *prometheus.HistogramVec
	nodePollResults           *prometheus.CounterVec
	nodePollErrors            *prometheus.CounterVec
	schedulerQueueReady       prometheus.Gauge
	schedulerQueueDepthByType *prometheus.GaugeVec
	schedulerQueues           *pollQueueCollector
	schedulerQueueWait        *prometheus.HistogramVec
	workProgress              *pollWorkProgress
	schedulerDeadLetterDepth  *prometheus.GaugeVec
	schedulerDeadLetters      *pollDeadLetterCollector
	schedulerBreakers         *pollBreakerCollector

	mu sync.RWMutex
	// Missing keys have no result. A stored zero time is a first failure
	// without a success; it is exposed as -1 only in the age series.
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
		pollMetricsInstance = newPollMetrics(prometheus.DefaultRegisterer)
	})
	return pollMetricsInstance
}

func newPollMetrics(reg prometheus.Registerer) *PollMetrics {
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
				Help:      schedulerQueueDepthByTypeHelp,
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
		workProgress: newPollWorkProgress(),
		schedulerDeadLetterDepth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "dead_letter_depth",
				Help:      schedulerDeadLetterDepthHelp,
			},
			[]string{"instance_type", "instance"},
		),
		schedulerBreakers:    newPollBreakerCollector(),
		lastSuccessByKey:     make(map[metricKey]time.Time),
		nodeLastSuccessByKey: make(map[nodeMetricKey]time.Time),
		lastQueueTypeKeys:    make(map[string]struct{}),
		lastDLQKeys:          make(map[string]struct{}),
	}
	pm.schedulerQueues = newPollQueueCollector(pm)
	pm.schedulerDeadLetters = newPollDeadLetterCollector(pm.schedulerDeadLetterDepth)

	reg.MustRegister(
		pm.pollDuration,
		pm.pollResults,
		pm.pollErrors,
		pm.inflight,
		pm.nodePollDuration,
		pm.nodePollResults,
		pm.nodePollErrors,
		newPollFreshnessCollector(pm),
		pm.schedulerQueues,
		pm.schedulerQueueWait,
		pm.workProgress,
		pm.schedulerDeadLetters,
		pm.schedulerBreakers,
	)

	return pm
}

// pollFreshnessCollector exports timestamps and ages from the same snapshot,
// even when no new result arrives. It takes no provider or monitor lock.
type pollFreshnessCollector struct {
	metrics                       *PollMetrics
	connectionAge, connectionTime *prometheus.Desc
	nodeAge, nodeTime             *prometheus.Desc
}

func newPollFreshnessCollector(pm *PollMetrics) *pollFreshnessCollector {
	return &pollFreshnessCollector{
		metrics: pm,
		connectionAge: prometheus.NewDesc("pulse_monitor_poll_staleness_seconds",
			"Seconds since the last successful poll. -1 indicates no successes yet.",
			[]string{"instance_type", "instance"}, nil),
		connectionTime: prometheus.NewDesc(prometheus.BuildFQName("pulse", "monitor", "poll_last_success_timestamp"),
			"Unix timestamp of the last successful poll.", []string{"instance_type", "instance"}, nil),
		nodeAge: prometheus.NewDesc("pulse_monitor_node_poll_staleness_seconds",
			"Seconds since the last successful poll for a node. -1 indicates no successes yet.",
			[]string{"instance_type", "instance", "node"}, nil),
		nodeTime: prometheus.NewDesc(prometheus.BuildFQName("pulse", "monitor", "node_poll_last_success_timestamp"),
			"Unix timestamp of the last successful poll for a node.", []string{"instance_type", "instance", "node"}, nil),
	}
}

func (c *pollFreshnessCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.connectionAge
	ch <- c.connectionTime
	ch <- c.nodeAge
	ch <- c.nodeTime
}

func (c *pollFreshnessCollector) Collect(ch chan<- prometheus.Metric) {
	c.metrics.mu.RLock()
	connections := copyPollMetricSamples(c.metrics.lastSuccessByKey)
	nodes := copyPollMetricSamples(c.metrics.nodeLastSuccessByKey)
	c.metrics.mu.RUnlock()

	now := time.Now()
	for _, sample := range connections {
		emitPollFreshness(ch, c.connectionAge, c.connectionTime, now, sample.value, sample.key.instanceType, sample.key.instance)
	}
	for _, sample := range nodes {
		emitPollFreshness(ch, c.nodeAge, c.nodeTime, now, sample.value, sample.key.instanceType, sample.key.instance, sample.key.node)
	}
}

type pollMetricSample[K comparable, V any] struct {
	key   K
	value V
}

// Scrapes iterate the snapshot once and never look up a key. Copy just the
// samples, without the hash tables. Callers synchronize access to the map.
func copyPollMetricSamples[K comparable, V any](values map[K]V) []pollMetricSample[K, V] {
	samples := make([]pollMetricSample[K, V], 0, len(values))
	for key, value := range values {
		samples = append(samples, pollMetricSample[K, V]{key: key, value: value})
	}
	return samples
}

func emitPollFreshness(ch chan<- prometheus.Metric, age, timestamp *prometheus.Desc, now, last time.Time, labels ...string) {
	ch <- prometheus.MustNewConstMetric(age, prometheus.GaugeValue, pollAgeSeconds(now, last), labels...)
	if !last.IsZero() {
		ch <- prometheus.MustNewConstMetric(timestamp, prometheus.GaugeValue, float64(last.Unix()), labels...)
	}
}

func pollAgeSeconds(now, last time.Time) float64 {
	if last.IsZero() {
		return -1
	}
	return max(now.Sub(last).Seconds(), 0)
}

// Recorders may arrive out of order after capturing their completion times.
func rememberPollSuccess[K comparable](lastSuccess map[K]time.Time, key K, completedAt time.Time) {
	if completedAt.After(lastSuccess[key]) {
		lastSuccess[key] = completedAt
	}
}

func rememberFirstPollFailure[K comparable](lastSuccess map[K]time.Time, key K) {
	if _, ok := lastSuccess[key]; !ok {
		lastSuccess[key] = time.Time{}
	}
}

type breakerMetricObservation struct {
	state    float64
	failures int
	retryAt  time.Time
}

// pollBreakerCollector snapshots the recorded state, failures and deadline
// together. Scraping neither waits for monitor or breaker locks nor advances
// the breaker. Only the scheduler can admit a retry.
type pollBreakerCollector struct {
	state, failures, retrySeconds *prometheus.Desc
	mu                            sync.RWMutex
	observations                  map[metricKey]breakerMetricObservation
}

func newPollBreakerCollector() *pollBreakerCollector {
	labels := []string{"instance_type", "instance"}
	return &pollBreakerCollector{
		state: prometheus.NewDesc("pulse_scheduler_breaker_state",
			"Circuit breaker state encoded as 0=closed, 1=half-open, 2=open, -1=unknown.", labels, nil),
		failures: prometheus.NewDesc("pulse_scheduler_breaker_failure_count",
			"Current consecutive failure count tracked by the circuit breaker.", labels, nil),
		retrySeconds: prometheus.NewDesc("pulse_scheduler_breaker_retry_seconds",
			"Seconds remaining at scrape time until the last recorded breaker retry deadline; zero if reached or unset.", labels, nil),
		observations: make(map[metricKey]breakerMetricObservation),
	}
}

func (c *pollBreakerCollector) record(key metricKey, observation breakerMetricObservation) {
	c.mu.Lock()
	c.observations[key] = observation
	c.mu.Unlock()
}

func (c *pollBreakerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.state
	ch <- c.failures
	ch <- c.retrySeconds
}

func (c *pollBreakerCollector) snapshot() []pollMetricSample[metricKey, breakerMetricObservation] {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return copyPollMetricSamples(c.observations)
}

func (c *pollBreakerCollector) Collect(ch chan<- prometheus.Metric) {
	observations := c.snapshot()
	now := time.Now()
	for _, observation := range observations {
		key, sample := observation.key, observation.value
		seconds := 0.0
		if !sample.retryAt.IsZero() {
			seconds = max(sample.retryAt.Sub(now).Seconds(), 0)
		}
		ch <- prometheus.MustNewConstMetric(c.state, prometheus.GaugeValue,
			sample.state, key.instanceType, key.instance)
		ch <- prometheus.MustNewConstMetric(c.failures, prometheus.GaugeValue,
			float64(sample.failures), key.instanceType, key.instance)
		ch <- prometheus.MustNewConstMetric(c.retrySeconds, prometheus.GaugeValue,
			seconds, key.instanceType, key.instance)
	}
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
		pm.storeNodeLastSuccess(instType, inst, nodeLabel, result.EndTime)
		return
	}

	errType := pm.classifyError(result.Error)
	pm.nodePollErrors.WithLabelValues(instType, inst, nodeLabel, errType).Inc()

	pm.mu.Lock()
	rememberFirstPollFailure(pm.nodeLastSuccessByKey, makeNodeMetricKey(instType, inst, nodeLabel))
	pm.mu.Unlock()
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
	if pm == nil || pm.usesLiveSchedulerQueues() {
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

// UpdateDeadLetterCounts reports a sample for monitors driven without a
// runtime (for example, direct scheduler fixtures). Once a runtime has
// registered its queue, a Health or worker snapshot is not a metric source.
func (pm *PollMetrics) UpdateDeadLetterCounts(tasks []DeadLetterTask) {
	if pm == nil || pm.usesLiveDeadLetters() {
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

// clearDeadLetterCount publishes the retirement of one terminal failure.
// This is not a queue refresh: recovery must not clear other connections.
func (pm *PollMetrics) clearDeadLetterCount(instanceType, instance string) {
	if pm == nil || pm.usesLiveDeadLetters() {
		return
	}
	instType, inst := sanitizeInstanceLabels(instanceType, instance)
	pm.schedulerDeadLetterDepth.WithLabelValues(instType, inst).Set(0)
}

// SetBreakerState updates circuit breaker metrics for a specific instance.
func (pm *PollMetrics) SetBreakerState(instanceType, instance, state string, failures int, retryAt time.Time) {
	if pm == nil || pm.schedulerBreakers == nil {
		return
	}

	pm.schedulerBreakers.record(makeMetricKey(instanceType, instance), breakerMetricObservation{
		state: breakerStateToValue(state), failures: failures, retryAt: retryAt,
	})
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
		pm.storeLastSuccess(instType, inst, result.EndTime)
	} else {
		errType := pm.classifyError(result.Error)
		pm.pollErrors.WithLabelValues(instType, inst, errType).Inc()
		pm.mu.Lock()
		rememberFirstPollFailure(pm.lastSuccessByKey, makeMetricKey(instType, inst))
		pm.mu.Unlock()
	}

	pm.decrementPending()
}

// ResetQueueDepth sets the pending queue depth for the next polling cycle.
func (pm *PollMetrics) ResetQueueDepth(total int) {
	if pm == nil || pm.usesLiveSchedulerQueues() {
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
	if pm == nil || pm.usesLiveSchedulerQueues() {
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
	if pm == nil || pm.usesLiveSchedulerQueues() {
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
	rememberPollSuccess(pm.lastSuccessByKey, makeMetricKey(instanceType, instance), ts)
	pm.mu.Unlock()
}

func (pm *PollMetrics) lastSuccessFor(instanceType, instance string) (time.Time, bool) {
	pm.mu.RLock()
	ts, ok := pm.lastSuccessByKey[makeMetricKey(instanceType, instance)]
	pm.mu.RUnlock()
	return ts, ok && !ts.IsZero()
}

func (pm *PollMetrics) storeNodeLastSuccess(instanceType, instance, node string, ts time.Time) {
	pm.mu.Lock()
	rememberPollSuccess(pm.nodeLastSuccessByKey, makeNodeMetricKey(instanceType, instance, node), ts)
	pm.mu.Unlock()
}

func (pm *PollMetrics) lastNodeSuccessFor(instanceType, instance, node string) (time.Time, bool) {
	pm.mu.RLock()
	ts, ok := pm.nodeLastSuccessByKey[makeNodeMetricKey(instanceType, instance, node)]
	pm.mu.RUnlock()
	return ts, ok && !ts.IsZero()
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
