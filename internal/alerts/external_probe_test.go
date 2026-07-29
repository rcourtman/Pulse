package alerts

import "testing"

func externalProbeSnapshot(agentID string, state ExternalProbeState, targets ...string) ExternalProbeSnapshot {
	return ExternalProbeSnapshot{
		AgentID:        agentID,
		Name:           "Probe " + agentID,
		State:          state,
		TargetIDs:      targets,
		StaleTargetIDs: targets,
	}
}

func activeExternalProbeAlerts(manager *Manager) []Alert {
	var out []Alert
	for _, alert := range manager.GetActiveAlerts() {
		if alert.Type == ExternalProbeUnavailableAlertType {
			out = append(out, alert)
		}
	}
	return out
}

func TestSyncExternalProbesUsesStablePerAgentIdentity(t *testing.T) {
	manager := newTestManager(t)
	manager.SyncExternalProbes([]ExternalProbeSnapshot{
		externalProbeSnapshot("agent-1", ExternalProbeStateUnavailable, "target-b", "target-a"),
		externalProbeSnapshot("agent-2", ExternalProbeStateUnavailable, "target-c"),
	})

	active := activeExternalProbeAlerts(manager)
	if len(active) != 2 {
		t.Fatalf("active alerts = %+v, want one per unavailable probe", active)
	}
	var agentOne Alert
	for _, alert := range active {
		if alert.Metadata["hostId"] == "agent-1" {
			agentOne = alert
			break
		}
	}
	if agentOne.ID == "" {
		t.Fatalf("agent-1 alert missing from %+v", active)
	}
	if agentOne.ResourceID != "agent:agent-1" ||
		agentOne.Metadata["incidentCode"] != ExternalProbeUnavailableIncidentCode ||
		agentOne.Metadata["incidentSource"] != ExternalProbeIncidentSource {
		t.Fatalf("agent-1 alert = %+v, want stable external-probe metadata", agentOne)
	}
	firstID := agentOne.ID

	// Deleting the target that happened to sort first must update the same
	// per-agent alert instead of resolving and reopening a new incident.
	manager.SyncExternalProbes([]ExternalProbeSnapshot{
		externalProbeSnapshot("agent-1", ExternalProbeStateUnavailable, "target-z"),
		externalProbeSnapshot("agent-2", ExternalProbeStateUnavailable, "target-c"),
	})
	active = activeExternalProbeAlerts(manager)
	if len(active) != 2 {
		t.Fatalf("active alerts after target replacement = %+v, want two", active)
	}
	for _, alert := range active {
		if alert.Metadata["hostId"] == "agent-1" && alert.ID != firstID {
			t.Fatalf("agent-1 alert ID changed from %q to %q", firstID, alert.ID)
		}
	}
}

func TestSyncExternalProbesAvoidsHostOfflineDuplicationAndRecovers(t *testing.T) {
	manager := newTestManager(t)
	unavailable := externalProbeSnapshot(
		"agent-1",
		ExternalProbeStateUnavailable,
		"target-a",
	)
	manager.SyncExternalProbes([]ExternalProbeSnapshot{unavailable})
	if got := activeExternalProbeAlerts(manager); len(got) != 1 {
		t.Fatalf("active alerts = %+v, want one unavailable alert", got)
	}

	hostOffline := unavailable
	hostOffline.State = ExternalProbeStateHostOffline
	manager.SyncExternalProbes([]ExternalProbeSnapshot{hostOffline})
	if got := activeExternalProbeAlerts(manager); len(got) != 0 {
		t.Fatalf("probe alerts while host-offline owns lifecycle = %+v, want none", got)
	}

	manager.SyncExternalProbes([]ExternalProbeSnapshot{unavailable})
	if got := activeExternalProbeAlerts(manager); len(got) != 1 {
		t.Fatalf("active alerts after recurrence = %+v, want one", got)
	}
	healthy := unavailable
	healthy.State = ExternalProbeStateHealthy
	healthy.StaleTargetIDs = nil
	manager.SyncExternalProbes([]ExternalProbeSnapshot{healthy})
	if got := activeExternalProbeAlerts(manager); len(got) != 0 {
		t.Fatalf("active alerts after fresh results = %+v, want none", got)
	}
}

func TestSyncExternalProbesGracePreservesActiveAlertAndRemovalClearsIt(t *testing.T) {
	manager := newTestManager(t)
	unavailable := externalProbeSnapshot(
		"agent-1",
		ExternalProbeStateUnavailable,
		"target-a",
	)
	manager.SyncExternalProbes([]ExternalProbeSnapshot{unavailable})

	grace := unavailable
	grace.State = ExternalProbeStateGrace
	grace.StaleTargetIDs = nil
	manager.SyncExternalProbes([]ExternalProbeSnapshot{grace})
	if got := activeExternalProbeAlerts(manager); len(got) != 1 {
		t.Fatalf("restart grace cleared an existing outage: %+v", got)
	}

	manager.SyncExternalProbes(nil)
	if got := activeExternalProbeAlerts(manager); len(got) != 0 {
		t.Fatalf("unassigned probe alerts = %+v, want none", got)
	}
}
