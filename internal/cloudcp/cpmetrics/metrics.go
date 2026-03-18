package cpmetrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// TenantsByState tracks the number of tenants in each lifecycle state.
	TenantsByState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "pulse",
		Subsystem: "cp",
		Name:      "tenants_by_state",
		Help:      "Number of tenants by lifecycle state.",
	}, []string{"state"})

	// WebhookRequestsTotal counts Stripe webhook requests by event type and status.
	WebhookRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pulse",
		Subsystem: "cp",
		Name:      "webhook_requests_total",
		Help:      "Total Stripe webhook requests by event type and HTTP status.",
	}, []string{"event_type", "status"})

	// WebhookDuration tracks Stripe webhook processing latency.
	WebhookDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pulse",
		Subsystem: "cp",
		Name:      "webhook_duration_seconds",
		Help:      "Stripe webhook processing duration in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"event_type"})

	// HealthCheckResults tracks health check results by tenant state.
	HealthCheckResults = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pulse",
		Subsystem: "cp",
		Name:      "health_check_results_total",
		Help:      "Health check results (healthy/unhealthy).",
	}, []string{"result"})

	// ProvisioningTotal counts provisioning attempts and outcomes.
	ProvisioningTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pulse",
		Subsystem: "cp",
		Name:      "provisioning_total",
		Help:      "Total provisioning attempts by outcome.",
	}, []string{"outcome"})
)
