package api

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	rbacMetricsOnce sync.Once

	rbacManagersActive  prometheus.Gauge
	rbacRoleMutations   *prometheus.CounterVec
	rbacIntegrityChecks *prometheus.CounterVec
)

func initRBACMetrics() {
	rbacManagersActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "pulse",
			Subsystem: "rbac",
			Name:      "managers_active",
			Help:      "Number of active tenant RBAC manager instances in cache.",
		},
	)

	rbacRoleMutations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "rbac",
			Name:      "role_mutations_total",
			Help:      "Total number of RBAC role mutation operations.",
		},
		[]string{"action"},
	)

	rbacIntegrityChecks = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "pulse",
			Subsystem: "rbac",
			Name:      "integrity_checks_total",
			Help:      "Total number of RBAC integrity check operations.",
		},
		[]string{"result"},
	)

	prometheus.MustRegister(rbacManagersActive, rbacRoleMutations, rbacIntegrityChecks)
}

func ensureRBACMetrics() {
	rbacMetricsOnce.Do(initRBACMetrics)
}

// RecordRBACManagerCreated increments the active managers gauge.
func RecordRBACManagerCreated() {
	ensureRBACMetrics()
	rbacManagersActive.Inc()
}

// RecordRBACManagerRemoved decrements the active managers gauge.
func RecordRBACManagerRemoved() {
	ensureRBACMetrics()
	rbacManagersActive.Dec()
}

// RecordRBACRoleMutation records a role mutation event.
// action should be one of: "create", "update", "delete", "assign"
func RecordRBACRoleMutation(action string) {
	ensureRBACMetrics()
	rbacRoleMutations.WithLabelValues(action).Inc()
}

// RecordRBACIntegrityCheck records the result of an integrity check.
// result should be "healthy" or "unhealthy"
func RecordRBACIntegrityCheck(result string) {
	ensureRBACMetrics()
	rbacIntegrityChecks.WithLabelValues(result).Inc()
}
