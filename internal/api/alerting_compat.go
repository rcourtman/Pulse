package api

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	alertingapi "github.com/rcourtman/pulse-go-rewrite/internal/api/alerting"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
)

// Compatibility aliases keep the established internal/api extension surface
// stable while alert-delivery production code and tests live in their domain
// package and can be scheduled independently by the Go toolchain.
type AlertManager = alertingapi.AlertManager
type ConfigPersistence = alertingapi.ConfigPersistence
type AlertMonitor = alertingapi.AlertMonitor
type AlertHandlers = alertingapi.AlertHandlers
type NotificationManager = alertingapi.NotificationManager
type NotificationConfigPersistence = alertingapi.NotificationConfigPersistence
type NotificationMonitor = alertingapi.NotificationMonitor
type NotificationHandlers = alertingapi.NotificationHandlers
type NotificationQueueHandlers = alertingapi.NotificationQueueHandlers
type AlertMonitorWrapper = alertingapi.AlertMonitorWrapper
type NotificationMonitorWrapper = alertingapi.NotificationMonitorWrapper

var NewAlertHandlers = alertingapi.NewAlertHandlers
var NewNotificationHandlers = alertingapi.NewNotificationHandlers
var NewNotificationQueueHandlers = alertingapi.NewNotificationQueueHandlers
var NewAlertMonitorWrapper = alertingapi.NewAlertMonitorWrapper
var NewNotificationMonitorWrapper = alertingapi.NewNotificationMonitorWrapper

func externalProbePushNotification(
	alert *alerts.Alert,
	hasProbeAssignments func(string) bool,
) (relay.PushNotificationPayload, bool) {
	return alertingapi.ExternalProbePushNotification(alert, hasProbeAssignments)
}

func canonicalAlertPushNotification(
	alert *alerts.Alert,
	hasProbeAssignments func(string) bool,
) relay.PushNotificationPayload {
	return alertingapi.CanonicalAlertPushNotification(alert, hasProbeAssignments)
}
