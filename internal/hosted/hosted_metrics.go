package hosted

import (
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	provisionStatusSuccess = "success"
	provisionStatusFailure = "failure"

	lifecycleStatusActive          = "active"
	lifecycleStatusSuspended       = "suspended"
	lifecycleStatusPendingDeletion = "pending_deletion"
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
	m.provisionsTotal.WithLabelValues(normalizeProvisionStatus(status)).Inc()
}

// RecordLifecycleTransition increments hosted lifecycle transitions.
func (m *HostedMetrics) RecordLifecycleTransition(from, to string) {
	fromStatus := normalizeLifecycleStatus(from)
	toStatus := normalizeLifecycleStatus(to)
	m.lifecycleTransitionsTotal.WithLabelValues(fromStatus, toStatus).Inc()
}

// SetActiveTenants sets the active hosted tenant count.
func (m *HostedMetrics) SetActiveTenants(count float64) {
	m.activeTenants.Set(count)
}

func normalizeProvisionStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case provisionStatusSuccess:
		return provisionStatusSuccess
	default:
		return provisionStatusFailure
	}
}

func normalizeLifecycleStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", lifecycleStatusActive:
		return lifecycleStatusActive
	case lifecycleStatusSuspended:
		return lifecycleStatusSuspended
	case lifecycleStatusPendingDeletion:
		return lifecycleStatusPendingDeletion
	default:
		return lifecycleStatusActive
	}
}
