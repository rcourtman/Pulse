package alerting

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
)

func TestExternalProbePushNotificationSelectsOnlyProbeLoss(t *testing.T) {
	alert := &alerts.Alert{
		ID: "alert-1",
		Metadata: map[string]interface{}{
			"incidentCode":   alerts.ExternalProbeUnavailableIncidentCode,
			"incidentSource": alerts.ExternalProbeIncidentSource,
		},
	}
	notification, ok := ExternalProbePushNotification(alert, nil)
	if !ok {
		t.Fatal("external probe loss did not produce a push notification")
	}
	if notification.Type != relay.PushTypeExternalProbeOffline ||
		notification.ActionType != relay.PushActionViewAlert ||
		notification.ActionID != alert.ID {
		t.Fatalf("notification = %#v, want dedicated type routed to canonical mobile attention", notification)
	}

	for _, other := range []*alerts.Alert{
		nil,
		{ID: "missing-metadata"},
		{ID: "local-availability", Metadata: map[string]interface{}{
			"incidentCode":   "availability_unreachable",
			"incidentSource": "availability",
		}},
		{ID: "external-target-failure", Metadata: map[string]interface{}{
			"incidentCode":   "availability_unreachable",
			"incidentSource": alerts.ExternalProbeIncidentSource,
		}},
	} {
		if got, ok := ExternalProbePushNotification(other, nil); ok {
			t.Fatalf("unrelated alert %#v produced push %#v", other, got)
		}
	}
}

func TestExternalProbePushNotificationSelectsAssignedHostOffline(t *testing.T) {
	alert := &alerts.Alert{
		ID:   "host-offline-agent-1",
		Type: alerts.HostOfflineAlertType,
		Metadata: map[string]interface{}{
			"hostId": "agent-1",
		},
	}
	ownsAssignments := func(agentID string) bool { return agentID == "agent-1" }
	notification, ok := ExternalProbePushNotification(alert, ownsAssignments)
	if !ok {
		t.Fatal("assigned probe host-offline alert did not produce a push")
	}
	if notification.ActionID != alert.ID || notification.Type != relay.PushTypeExternalProbeOffline {
		t.Fatalf("notification = %#v, want external probe mobile routing", notification)
	}

	if got, ok := ExternalProbePushNotification(alert, func(string) bool { return false }); ok {
		t.Fatalf("ordinary host-offline alert produced external probe push %#v", got)
	}
}

func TestCanonicalAlertPushNotificationCoversOrdinaryAlerts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		level    alerts.AlertLevel
		priority string
	}{
		{name: "warning", level: alerts.AlertLevelWarning, priority: relay.PushPriorityNormal},
		{name: "critical", level: alerts.AlertLevelCritical, priority: relay.PushPriorityHigh},
	} {
		t.Run(tc.name, func(t *testing.T) {
			notification := CanonicalAlertPushNotification(&alerts.Alert{
				ID:           " alert-ordinary ",
				Level:        tc.level,
				ResourceName: "private-cluster-node-1",
				Message:      "10.0.0.7 exceeded its threshold",
			}, nil)
			if notification.Type != relay.PushTypeAlertFired || notification.Priority != tc.priority {
				t.Fatalf("notification = %#v, want ordinary alert push with priority %q", notification, tc.priority)
			}
			if notification.ActionType != relay.PushActionViewAlert || notification.ActionID != "alert-ordinary" {
				t.Fatalf("notification does not route to canonical alert: %#v", notification)
			}
			if strings.Contains(notification.Title+notification.Body, "private-cluster") ||
				strings.Contains(notification.Title+notification.Body, "10.0.0.7") {
				t.Fatalf("notification leaked infrastructure detail: %#v", notification)
			}
		})
	}
}

func TestCanonicalAlertPushNotificationRetainsProbeSpecificSignal(t *testing.T) {
	notification := CanonicalAlertPushNotification(&alerts.Alert{
		ID:    "probe-alert",
		Level: alerts.AlertLevelWarning,
		Metadata: map[string]interface{}{
			"incidentCode":   alerts.ExternalProbeUnavailableIncidentCode,
			"incidentSource": alerts.ExternalProbeIncidentSource,
		},
	}, nil)
	if notification.Type != relay.PushTypeExternalProbeOffline {
		t.Fatalf("notification = %#v, want specialized probe-loss signal", notification)
	}
}
