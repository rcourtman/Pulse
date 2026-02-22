package ai

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// PatrolMetrics manages Prometheus instrumentation for patrol decisions.
type PatrolMetrics struct {
	findingRejected      *prometheus.CounterVec
	circuitBlock         prometheus.Counter
	investigationOutcome *prometheus.CounterVec
	fixVerification      *prometheus.CounterVec
	runTotal             *prometheus.CounterVec
	triageFlags          prometheus.Histogram
	triageQuietTotal     prometheus.Counter
	scopedDropped        prometheus.Counter
	scopedDroppedFinal   prometheus.Counter
	streamResumeOutcome  *prometheus.CounterVec
	streamResyncReason   *prometheus.CounterVec
	streamReplayEvents   prometheus.Counter
	streamReplayBatch    prometheus.Histogram
	streamSubscriberDrop *prometheus.CounterVec
}

var (
	patrolMetricsInstance *PatrolMetrics
	patrolMetricsOnce     sync.Once
)

// GetPatrolMetrics returns the singleton patrol metrics instance.
func GetPatrolMetrics() *PatrolMetrics {
	patrolMetricsOnce.Do(func() {
		patrolMetricsInstance = newPatrolMetrics()
	})
	return patrolMetricsInstance
}

func newPatrolMetrics() *PatrolMetrics {
	m := &PatrolMetrics{
		findingRejected: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "finding_rejected_total",
				Help:      "Total findings rejected by threshold validation",
			},
			[]string{"resource_type", "metric"},
		),
		circuitBlock: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "circuit_block_total",
				Help:      "Total patrol runs blocked by circuit breaker",
			},
		),
		investigationOutcome: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "investigation_outcome_total",
				Help:      "Total investigation outcomes by result",
			},
			[]string{"outcome"},
		),
		fixVerification: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "fix_verification_total",
				Help:      "Total fix verification results",
			},
			[]string{"result"},
		),
		runTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "run_total",
				Help:      "Total patrol runs by trigger and type",
			},
			[]string{"trigger", "type"},
		),
		triageFlags: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "triage_flags",
				Help:      "Number of deterministic triage flags per patrol run",
				Buckets:   []float64{0, 1, 2, 3, 5, 10, 20, 50},
			},
		),
		triageQuietTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "triage_quiet_total",
				Help:      "Total patrol runs skipped due to quiet infrastructure (no triage flags)",
			},
		),
		scopedDropped: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "scoped_dropped_total",
				Help:      "Total scoped patrols dropped because a run was already in progress",
			},
		),
		scopedDroppedFinal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "scoped_dropped_final_total",
				Help:      "Total scoped patrols permanently dropped after exhausting retries",
			},
		),
		streamResumeOutcome: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "stream_resume_outcome_total",
				Help:      "Total patrol stream resume outcomes",
			},
			[]string{"outcome"},
		),
		streamResyncReason: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "stream_resync_reason_total",
				Help:      "Total synthetic patrol stream snapshots by resync reason",
			},
			[]string{"reason"},
		),
		streamReplayEvents: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "stream_replay_events_total",
				Help:      "Total patrol stream events replayed to resuming subscribers",
			},
		),
		streamReplayBatch: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "stream_replay_batch_size",
				Help:      "Patrol stream replay batch size per subscriber sync",
				Buckets:   []float64{1, 2, 5, 10, 25, 50, 100},
			},
		),
		streamSubscriberDrop: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "patrol",
				Name:      "stream_subscriber_drop_total",
				Help:      "Total patrol stream subscribers dropped by reason",
			},
			[]string{"reason"},
		),
	}

	prometheus.MustRegister(
		m.findingRejected,
		m.circuitBlock,
		m.investigationOutcome,
		m.fixVerification,
		m.runTotal,
		m.triageFlags,
		m.triageQuietTotal,
		m.scopedDropped,
		m.scopedDroppedFinal,
		m.streamResumeOutcome,
		m.streamResyncReason,
		m.streamReplayEvents,
		m.streamReplayBatch,
		m.streamSubscriberDrop,
	)

	return m
}

// RecordFindingRejected records a finding rejected by threshold validation.
func (m *PatrolMetrics) RecordFindingRejected(resourceType, metric string) {
	m.findingRejected.WithLabelValues(resourceType, metric).Inc()
}

// RecordCircuitBlock records a patrol run blocked by circuit breaker.
func (m *PatrolMetrics) RecordCircuitBlock() {
	m.circuitBlock.Inc()
}

// RecordInvestigationOutcome records an investigation completion outcome.
func (m *PatrolMetrics) RecordInvestigationOutcome(outcome string) {
	m.investigationOutcome.WithLabelValues(outcome).Inc()
}

// RecordFixVerification records a fix verification result.
func (m *PatrolMetrics) RecordFixVerification(result string) {
	m.fixVerification.WithLabelValues(result).Inc()
}

// RecordScopedDropped records a scoped patrol dropped because a run was already in progress.
func (m *PatrolMetrics) RecordScopedDropped() {
	m.scopedDropped.Inc()
}

// RecordScopedDroppedFinal records a scoped patrol permanently dropped after exhausting retries.
func (m *PatrolMetrics) RecordScopedDroppedFinal() {
	m.scopedDroppedFinal.Inc()
}

// RecordRun records a patrol run.
func (m *PatrolMetrics) RecordRun(trigger, runType string) {
	m.runTotal.WithLabelValues(trigger, runType).Inc()
}

// RecordTriageFlags records the number of triage flags for a patrol run.
func (m *PatrolMetrics) RecordTriageFlags(count int) {
	m.triageFlags.Observe(float64(count))
}

// RecordTriageQuiet records a patrol run that skipped LLM due to quiet infrastructure.
func (m *PatrolMetrics) RecordTriageQuiet() {
	m.triageQuietTotal.Inc()
}

// RecordStreamReplay records a stream subscriber sync that replayed buffered events.
func (m *PatrolMetrics) RecordStreamReplay(replayEvents int) {
	m.streamResumeOutcome.WithLabelValues("replay").Inc()
	if replayEvents > 0 {
		m.streamReplayEvents.Add(float64(replayEvents))
		m.streamReplayBatch.Observe(float64(replayEvents))
	}
}

// RecordStreamSnapshot records a synthetic snapshot sent for resync or late join.
func (m *PatrolMetrics) RecordStreamSnapshot(reason string) {
	if reason == "" {
		reason = "unknown"
	}
	m.streamResumeOutcome.WithLabelValues("snapshot").Inc()
	m.streamResyncReason.WithLabelValues(reason).Inc()
}

// RecordStreamMiss records a resume attempt where no replay or snapshot was emitted.
func (m *PatrolMetrics) RecordStreamMiss() {
	m.streamResumeOutcome.WithLabelValues("miss").Inc()
}

// RecordStreamSubscriberDrop records subscriber removals from the live stream.
func (m *PatrolMetrics) RecordStreamSubscriberDrop(reason string) {
	if reason == "" {
		reason = "unknown"
	}
	m.streamSubscriberDrop.WithLabelValues(reason).Inc()
}
