package operationaltrust

import (
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics exposes low-cardinality health measurements for the canonical
// operational-trust loop. Raw resource, record, evidence, actor, destination,
// and provider-instance identifiers are deliberately never used as labels.
type Metrics struct {
	observationToOpen      prometheus.Histogram
	openToNotification     prometheus.Histogram
	evidenceObservations   *prometheus.CounterVec
	identityCorrelations   *prometheus.CounterVec
	protectionEvaluations  *prometheus.CounterVec
	protectionFailures     *prometheus.CounterVec
	notificationDeliveries *prometheus.CounterVec
	activeCountMismatches  prometheus.Counter
	actionOffers           *prometheus.CounterVec
	actionVerification     *prometheus.CounterVec
}

var (
	metricsInstance *Metrics
	metricsOnce     sync.Once
)

// GetMetrics returns the process-wide operational-trust metrics owner.
func GetMetrics() *Metrics {
	metricsOnce.Do(func() {
		metricsInstance = newMetrics(prometheus.DefaultRegisterer)
	})
	return metricsInstance
}

func newMetrics(registerer prometheus.Registerer) *Metrics {
	metrics := &Metrics{
		observationToOpen: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "pulse",
			Subsystem: "operational_trust",
			Name:      "observation_to_open_seconds",
			Help:      "Elapsed seconds from the first canonical observation to an open operational record",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 15, 30, 60},
		}),
		openToNotification: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "pulse",
			Subsystem: "operational_trust",
			Name:      "open_to_notification_enqueue_seconds",
			Help:      "Elapsed seconds from an open lifecycle transition to notification enqueue",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 15, 30, 60},
		}),
		evidenceObservations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "operational_trust",
			Name:      "evidence_observations_total",
			Help:      "Canonical evidence observations by bounded source and evidence state",
		}, []string{"source", "state"}),
		identityCorrelations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "operational_trust",
			Name:      "identity_correlations_total",
			Help:      "Canonical resource identity correlation decisions by outcome",
		}, []string{"outcome"}),
		protectionEvaluations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "operational_trust",
			Name:      "protection_posture_evaluations_total",
			Help:      "Protection posture evaluations by canonical posture state",
		}, []string{"state"}),
		protectionFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "operational_trust",
			Name:      "protection_posture_evaluation_failures_total",
			Help:      "Protection posture evaluation failures by bounded reason",
		}, []string{"reason"}),
		notificationDeliveries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "operational_trust",
			Name:      "notification_delivery_total",
			Help:      "Transition-linked notification delivery outcomes",
		}, []string{"outcome"}),
		activeCountMismatches: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "operational_trust",
			Name:      "active_count_mismatch_total",
			Help:      "Detected mismatches between the canonical active count and active attention projection",
		}),
		actionOffers: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "operational_trust",
			Name:      "action_offers_total",
			Help:      "Operational Trust action offer projections by eligibility",
		}, []string{"eligibility"}),
		actionVerification: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "operational_trust",
			Name:      "action_verification_total",
			Help:      "Operational Trust action verification outcomes",
		}, []string{"outcome"}),
	}
	registerer.MustRegister(
		metrics.observationToOpen,
		metrics.openToNotification,
		metrics.evidenceObservations,
		metrics.identityCorrelations,
		metrics.protectionEvaluations,
		metrics.protectionFailures,
		metrics.notificationDeliveries,
		metrics.activeCountMismatches,
		metrics.actionOffers,
		metrics.actionVerification,
	)
	return metrics
}

func (metrics *Metrics) ObserveObservationToOpen(firstObservedAt, openedAt time.Time) {
	if metrics == nil {
		return
	}
	if elapsed, ok := nonNegativeElapsed(firstObservedAt, openedAt); ok {
		metrics.observationToOpen.Observe(elapsed.Seconds())
	}
}

func (metrics *Metrics) ObserveOpenToNotification(openedAt, enqueuedAt time.Time) {
	if metrics == nil {
		return
	}
	if elapsed, ok := nonNegativeElapsed(openedAt, enqueuedAt); ok {
		metrics.openToNotification.Observe(elapsed.Seconds())
	}
}

func (metrics *Metrics) ObserveEvidence(envelope EvidenceEnvelope, at time.Time) {
	if metrics == nil {
		return
	}
	metrics.evidenceObservations.WithLabelValues(
		operationalMetricSource(envelope.Source.Provider),
		operationalEvidenceMetricState(envelope, at),
	).Inc()
}

func (metrics *Metrics) ObserveIdentityCorrelation(outcome string) {
	if metrics == nil {
		return
	}
	metrics.identityCorrelations.WithLabelValues(
		boundedMetricLabel(outcome, map[string]struct{}{
			"attached": {}, "standalone": {}, "ambiguous": {}, "unresolved": {},
		}),
	).Inc()
}

func (metrics *Metrics) ObserveProtectionEvaluation(state string) {
	if metrics == nil {
		return
	}
	metrics.protectionEvaluations.WithLabelValues(
		boundedMetricLabel(state, map[string]struct{}{
			"protected": {}, "attention": {}, "unprotected": {}, "unknown": {},
		}),
	).Inc()
}

func (metrics *Metrics) ObserveProtectionEvaluationFailure(reason string) {
	if metrics == nil {
		return
	}
	metrics.protectionFailures.WithLabelValues(
		boundedMetricLabel(reason, map[string]struct{}{
			"invalid_policy": {}, "invalid_posture": {}, "store": {}, "provider": {},
		}),
	).Inc()
}

func (metrics *Metrics) ObserveNotificationDelivery(outcome string) {
	if metrics == nil {
		return
	}
	metrics.notificationDeliveries.WithLabelValues(
		boundedMetricLabel(outcome, map[string]struct{}{
			"queued": {}, "retry": {}, "sent": {}, "failed": {}, "dead_letter": {}, "cancelled": {},
		}),
	).Inc()
}

func (metrics *Metrics) RecordActiveCountMismatch() {
	if metrics != nil {
		metrics.activeCountMismatches.Inc()
	}
}

func (metrics *Metrics) ObserveActionOffer(eligibility string) {
	if metrics == nil {
		return
	}
	metrics.actionOffers.WithLabelValues(
		boundedMetricLabel(eligibility, map[string]struct{}{
			"eligible": {}, "ineligible": {}, "unknown": {},
		}),
	).Inc()
}

func (metrics *Metrics) ObserveActionVerification(outcome string) {
	if metrics == nil {
		return
	}
	metrics.actionVerification.WithLabelValues(
		boundedMetricLabel(outcome, map[string]struct{}{
			"pending": {}, "confirmed": {}, "contradicted": {}, "inconclusive": {}, "timed_out": {}, "not_attempted": {},
		}),
	).Inc()
}

func nonNegativeElapsed(start, end time.Time) (time.Duration, bool) {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0, false
	}
	return end.Sub(start), true
}

func operationalMetricSource(source string) string {
	return boundedMetricLabel(source, map[string]struct{}{
		"agent": {}, "availability": {}, "docker": {}, "host": {}, "k8s": {},
		"kubernetes": {}, "pbs": {}, "podman": {}, "proxmox": {}, "pulse": {},
		"pve": {}, "qnap": {}, "synology": {}, "truenas": {}, "unraid": {},
		"vmware": {},
	})
}

func operationalEvidenceMetricState(envelope EvidenceEnvelope, at time.Time) string {
	switch envelope.Permissions {
	case EvidencePermissionsDenied:
		return "denied"
	case EvidencePermissionsPartial:
		return "partial_permission"
	}
	switch envelope.Completeness {
	case EvidenceUnavailable:
		return "unavailable"
	case EvidencePartial:
		return "partial"
	}
	switch envelope.FreshnessAt(at) {
	case EvidenceStale:
		return "stale"
	case EvidenceFreshnessUnknown:
		return "unknown"
	default:
		return "current"
	}
}

func boundedMetricLabel(value string, allowed map[string]struct{}) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	if _, ok := allowed[value]; ok {
		return value
	}
	return "other"
}
