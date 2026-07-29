package api

import (
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
	notification, ok := externalProbePushNotification(alert, nil)
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
		if got, ok := externalProbePushNotification(other, nil); ok {
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
	notification, ok := externalProbePushNotification(alert, ownsAssignments)
	if !ok {
		t.Fatal("assigned probe host-offline alert did not produce a push")
	}
	if notification.ActionID != alert.ID || notification.Type != relay.PushTypeExternalProbeOffline {
		t.Fatalf("notification = %#v, want external probe mobile routing", notification)
	}

	if got, ok := externalProbePushNotification(alert, func(string) bool { return false }); ok {
		t.Fatalf("ordinary host-offline alert produced external probe push %#v", got)
	}
}
