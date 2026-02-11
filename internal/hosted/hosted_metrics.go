package hosted

import (
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type ProvisionMetricStatus string
type LifecycleMetricStatus string

const (
	ProvisionMetricStatusSuccess ProvisionMetricStatus = "success"
	ProvisionMetricStatusFailure ProvisionMetricStatus = "failure"

	LifecycleMetricStatusActive          LifecycleMetricStatus = "active"
	LifecycleMetricStatusSuspended       LifecycleMetricStatus = "suspended"
	LifecycleMetricStatusPendingDeletion LifecycleMetricStatus = "pending_deletion"
)

// HostedMetrics manages Prometheus instrumentation for hosted-mode operations.
type HostedMetrics struct {
	signupsTotal              prometheus.Counter
	provisionsTotal           *prometheus.CounterVec
	lifecycleTransitionsTotal *prometheus.CounterVec
	activeTenants             prometheus.Gauge
}

var (
	hostedMetricsInstance *HostedMetrics
	hostedMetricsOnce     sync.Once
)

// GetHostedMetrics returns the singleton hosted metrics instance.
func GetHostedMetrics() *HostedMetrics {
	hostedMetricsOnce.Do(func() {
		hostedMetricsInstance = newHostedMetrics()
	})
	return hostedMetricsInstance
}

func newHostedMetrics() *HostedMetrics {
	m := &HostedMetrics{
		signupsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "hosted",
				Name:      "signups_total",
				Help:      "Total successful hosted signups.",
			},
		),
		provisionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "hosted",
				Name:      "provisions_total",
				Help:      "Total hosted tenant provisioning attempts by status.",
			},
			[]string{"status"},
		),
		lifecycleTransitionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "hosted",
				Name:      "lifecycle_transitions_total",
				Help:      "Total hosted lifecycle transitions by source and destination status.",
			},
			[]string{"from_status", "to_status"},
		),
		activeTenants: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "hosted",
				Name:      "active_tenants",
				Help:      "Current number of active hosted tenants.",
			},
		),
	}

	prometheus.MustRegister(
		m.signupsTotal,
		m.provisionsTotal,
		m.lifecycleTransitionsTotal,
		m.activeTenants,
	)

	return m
}

// RecordSignup increments successful hosted signups.
func (m *HostedMetrics) RecordSignup() {
	m.signupsTotal.Inc()
}

// RecordProvision increments hosted provisioning attempts by status.
func (m *HostedMetrics) RecordProvision(status string) {
	m.RecordProvisionStatus(ParseProvisionMetricStatus(status))
}

// RecordProvisionStatus increments hosted provisioning attempts by typed status.
func (m *HostedMetrics) RecordProvisionStatus(status ProvisionMetricStatus) {
	normalizedStatus := normalizeProvisionMetricStatus(status)
	m.provisionsTotal.WithLabelValues(string(normalizedStatus)).Inc()
}

// RecordLifecycleTransition increments hosted lifecycle transitions.
func (m *HostedMetrics) RecordLifecycleTransition(from, to string) {
	m.RecordLifecycleTransitionStatus(
		ParseLifecycleMetricStatus(from),
		ParseLifecycleMetricStatus(to),
	)
}

// RecordLifecycleTransitionStatus increments hosted lifecycle transitions by typed statuses.
func (m *HostedMetrics) RecordLifecycleTransitionStatus(from, to LifecycleMetricStatus) {
	fromStatus := normalizeLifecycleMetricStatus(from)
	toStatus := normalizeLifecycleMetricStatus(to)
	m.lifecycleTransitionsTotal.WithLabelValues(string(fromStatus), string(toStatus)).Inc()
}

// SetActiveTenants sets the active hosted tenant count.
func (m *HostedMetrics) SetActiveTenants(count float64) {
	m.activeTenants.Set(count)
}

func ParseProvisionMetricStatus(status string) ProvisionMetricStatus {
	return normalizeProvisionMetricStatus(ProvisionMetricStatus(strings.ToLower(strings.TrimSpace(status))))
}

func ParseLifecycleMetricStatus(status string) LifecycleMetricStatus {
	return normalizeLifecycleMetricStatus(LifecycleMetricStatus(strings.ToLower(strings.TrimSpace(status))))
}

func normalizeProvisionMetricStatus(status ProvisionMetricStatus) ProvisionMetricStatus {
	switch status {
	case ProvisionMetricStatusSuccess:
		return ProvisionMetricStatusSuccess
	default:
		return ProvisionMetricStatusFailure
	}
}

func normalizeLifecycleMetricStatus(status LifecycleMetricStatus) LifecycleMetricStatus {
	switch status {
	case "", LifecycleMetricStatusActive:
		return LifecycleMetricStatusActive
	case LifecycleMetricStatusSuspended:
		return LifecycleMetricStatusSuspended
	case LifecycleMetricStatusPendingDeletion:
		return LifecycleMetricStatusPendingDeletion
	default:
		return LifecycleMetricStatusActive
	}
}
