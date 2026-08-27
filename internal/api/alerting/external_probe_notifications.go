package alerting

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
)

// externalProbePushNotification keeps mobile payloads deliberately generic:
// alert metadata selects the event, while infrastructure names and addresses
// remain inside the self-hosted Pulse instance.
func ExternalProbePushNotification(
	alert *alerts.Alert,
	hasProbeAssignments func(string) bool,
) (relay.PushNotificationPayload, bool) {
	if alert == nil || alert.Metadata == nil {
		return relay.PushNotificationPayload{}, false
	}
	code, _ := alert.Metadata["incidentCode"].(string)
	source, _ := alert.Metadata["incidentSource"].(string)
	probeSpecific := strings.TrimSpace(code) == alerts.ExternalProbeUnavailableIncidentCode &&
		strings.TrimSpace(source) == alerts.ExternalProbeIncidentSource

	hostOffline := false
	if alert.Type == alerts.HostOfflineAlertType && hasProbeAssignments != nil {
		hostID, _ := alert.Metadata["hostId"].(string)
		hostOffline = hasProbeAssignments(strings.TrimSpace(hostID))
	}
	if !probeSpecific && !hostOffline {
		return relay.PushNotificationPayload{}, false
	}
	return relay.NewExternalProbeUnavailableNotification(alert.ID), true
}

// CanonicalAlertPushNotification projects every dispatched alert into a
// privacy-safe mobile notification. External-probe outages retain their
// purpose-built attention signal; every other alert uses the canonical alert
// identity and severity without exposing infrastructure details.
func CanonicalAlertPushNotification(
	alert *alerts.Alert,
	hasProbeAssignments func(string) bool,
) relay.PushNotificationPayload {
	if notification, specialized := ExternalProbePushNotification(alert, hasProbeAssignments); specialized {
		return notification
	}
	if alert == nil {
		return relay.PushNotificationPayload{}
	}
	return relay.NewAlertFiredNotification(alert.ID, string(alert.Level))
}
