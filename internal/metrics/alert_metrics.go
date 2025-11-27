package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

var (
	// Alert lifecycle metrics
	AlertsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pulse_alerts_active",
			Help: "Number of currently active alerts by level and type",
		},
		[]string{"level", "type"},
	)

	AlertsFiredTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulse_alerts_fired_total",
			Help: "Total number of alerts fired by level and type",
		},
		[]string{"level", "type"},
	)

	AlertsResolvedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulse_alerts_resolved_total",
			Help: "Total number of alerts resolved by type",
		},
		[]string{"type"},
	)

	AlertsAcknowledgedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "pulse_alerts_acknowledged_total",
			Help: "Total number of alerts acknowledged",
		},
	)

	AlertDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pulse_alert_duration_seconds",
			Help:    "Duration of alerts from fire to resolve",
			Buckets: []float64{60, 300, 900, 1800, 3600, 7200, 14400, 28800, 86400}, // 1m to 1d
		},
		[]string{"type"},
	)

	// Suppression and quiet hours metrics
	AlertsSuppressedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulse_alerts_suppressed_total",
			Help: "Total number of alerts suppressed by reason",
		},
		[]string{"reason"}, // quiet_hours, rate_limit, duplicate, etc.
	)

	AlertsRateLimitedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "pulse_alerts_rate_limited_total",
			Help: "Total number of alerts suppressed due to rate limiting",
		},
	)
)

// RecordAlertFired records when an alert is fired
func RecordAlertFired(alert *alerts.Alert) {
	AlertsFiredTotal.WithLabelValues(string(alert.Level), alert.Type).Inc()
	AlertsActive.WithLabelValues(string(alert.Level), alert.Type).Inc()
}

// RecordAlertResolved records when an alert is resolved
func RecordAlertResolved(alert *alerts.Alert) {
	AlertsResolvedTotal.WithLabelValues(alert.Type).Inc()
	AlertsActive.WithLabelValues(string(alert.Level), alert.Type).Dec()

	// Record duration
	duration := alert.LastSeen.Sub(alert.StartTime).Seconds()
	AlertDurationSeconds.WithLabelValues(alert.Type).Observe(duration)
}

// RecordAlertAcknowledged records when an alert is acknowledged
func RecordAlertAcknowledged() {
	AlertsAcknowledgedTotal.Inc()
}

// RecordAlertSuppressed records a suppressed alert
func RecordAlertSuppressed(reason string) {
	AlertsSuppressedTotal.WithLabelValues(reason).Inc()
}
