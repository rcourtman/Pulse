package operationaltrust

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestOperationalTrustMetricsUseBoundedLabelsAndRecordRequiredOutcomes(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := newMetrics(registry)
	now := time.Date(2026, 7, 19, 6, 0, 0, 0, time.UTC)
	validUntil := now.Add(-time.Minute)

	metrics.ObserveObservationToOpen(now.Add(-2*time.Second), now)
	metrics.ObserveOpenToNotification(now, now.Add(time.Second))
	metrics.ObserveEvidence(EvidenceEnvelope{
		Source:       EvidenceSource{Provider: "customer-controlled-provider-name"},
		ObservedAt:   now.Add(-time.Minute),
		ValidUntil:   &validUntil,
		Completeness: EvidenceComplete,
		Permissions:  EvidencePermissionsSufficient,
	}, now)
	metrics.ObserveIdentityCorrelation("ambiguous")
	metrics.ObserveProtectionEvaluation("protected")
	metrics.ObserveProtectionEvaluationFailure("invalid_policy")
	metrics.ObserveNotificationDelivery("dead-letter")
	metrics.RecordActiveCountMismatch()
	metrics.ObserveActionOffer("eligible")
	metrics.ObserveActionVerification("inconclusive")

	for name, collector := range map[string]prometheus.Collector{
		"observation latency":  metrics.observationToOpen,
		"notification latency": metrics.openToNotification,
		"evidence":             metrics.evidenceObservations,
		"identity":             metrics.identityCorrelations,
		"posture":              metrics.protectionEvaluations,
		"posture failure":      metrics.protectionFailures,
		"delivery":             metrics.notificationDeliveries,
		"mismatch":             metrics.activeCountMismatches,
		"offer":                metrics.actionOffers,
		"verification":         metrics.actionVerification,
	} {
		if count := testutil.CollectAndCount(collector); count != 1 {
			t.Fatalf("%s metric family count = %d, want 1", name, count)
		}
	}

	if got := testutil.ToFloat64(
		metrics.evidenceObservations.WithLabelValues("other", "stale"),
	); got != 1 {
		t.Fatalf("bounded evidence metric = %v, want 1", got)
	}
	if got := testutil.ToFloat64(
		metrics.notificationDeliveries.WithLabelValues("dead_letter"),
	); got != 1 {
		t.Fatalf("dead-letter metric = %v, want 1", got)
	}
}

func TestOperationalTrustMetricsIgnoreInvalidNegativeLatency(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := newMetrics(registry)
	now := time.Now().UTC()

	metrics.ObserveObservationToOpen(now, now.Add(-time.Second))
	metrics.ObserveOpenToNotification(time.Time{}, now)

	if got := testutil.CollectAndCount(metrics.observationToOpen); got != 1 {
		t.Fatalf("observation histogram family count = %d, want 1", got)
	}
	if got := testutil.CollectAndCount(metrics.openToNotification); got != 1 {
		t.Fatalf("notification histogram family count = %d, want 1", got)
	}
}
