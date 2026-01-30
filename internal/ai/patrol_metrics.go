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
	scopedDropped        prometheus.Counter
	scopedDroppedFinal   prometheus.Counter
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
	}

	prometheus.MustRegister(
		m.findingRejected,
		m.circuitBlock,
		m.investigationOutcome,
		m.fixVerification,
		m.runTotal,
		m.scopedDropped,
		m.scopedDroppedFinal,
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
