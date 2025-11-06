package metrics

import (
	"fmt"

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

	// Notification metrics
	NotificationsSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulse_notifications_sent_total",
			Help: "Total number of notifications sent by method and status",
		},
		[]string{"method", "status"}, // method: email, webhook, apprise; status: success, failure
	)

	NotificationQueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pulse_notification_queue_depth",
			Help: "Number of notifications in queue by status",
		},
		[]string{"status"}, // pending, sending, dlq
	)

	NotificationDeliveryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pulse_notification_delivery_duration_seconds",
			Help:    "Time to deliver notifications",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	NotificationRetriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulse_notification_retries_total",
			Help: "Total number of notification retry attempts",
		},
		[]string{"method"},
	)

	NotificationDLQTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "pulse_notification_dlq_total",
			Help: "Total number of notifications moved to dead letter queue",
		},
	)

	// Alert grouping metrics
	NotificationsGroupedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "pulse_notifications_grouped_total",
			Help: "Total number of grouped notifications sent",
		},
	)

	AlertsGroupedCount = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "pulse_alerts_grouped_count",
			Help:    "Number of alerts in grouped notifications",
			Buckets: []float64{1, 2, 5, 10, 20, 50, 100},
		},
	)

	// Escalation metrics
	AlertEscalationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulse_alert_escalations_total",
			Help: "Total number of alert escalations by level",
		},
		[]string{"level"},
	)

	// History metrics
	AlertHistorySize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "pulse_alert_history_size",
			Help: "Number of alerts in history",
		},
	)

	AlertHistorySaveErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "pulse_alert_history_save_errors_total",
			Help: "Total number of alert history save failures",
		},
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

// RecordNotificationSent records a notification delivery attempt
func RecordNotificationSent(method string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	NotificationsSentTotal.WithLabelValues(method, status).Inc()
}

// RecordNotificationRetry records a notification retry
func RecordNotificationRetry(method string) {
	NotificationRetriesTotal.WithLabelValues(method).Inc()
}

// RecordNotificationDLQ records a notification moved to DLQ
func RecordNotificationDLQ() {
	NotificationDLQTotal.Inc()
}

// RecordGroupedNotification records a grouped notification
func RecordGroupedNotification(alertCount int) {
	NotificationsGroupedTotal.Inc()
	AlertsGroupedCount.Observe(float64(alertCount))
}

// RecordAlertEscalation records an alert escalation
func RecordAlertEscalation(level int) {
	AlertEscalationsTotal.WithLabelValues(fmt.Sprintf("%d", level)).Inc()
}

// RecordAlertSuppressed records a suppressed alert
func RecordAlertSuppressed(reason string) {
	AlertsSuppressedTotal.WithLabelValues(reason).Inc()
}

// UpdateQueueDepth updates the notification queue depth metric
func UpdateQueueDepth(status string, count int) {
	NotificationQueueDepth.WithLabelValues(status).Set(float64(count))
}

// UpdateHistorySize updates the alert history size metric
func UpdateHistorySize(size int) {
	AlertHistorySize.Set(float64(size))
}

// RecordHistorySaveError records a history save failure
func RecordHistorySaveError() {
	AlertHistorySaveErrors.Inc()
}
