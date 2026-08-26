package alerts

import (
	"testing"
	"time"
)

// platformConnectionSnapshot is a small helper that produces a baseline
// ConnectionSnapshot for a pve connection so test cases can override one
// field at a time without restating the boilerplate.
func platformConnectionSnapshot(id, name string, state ConnectionState) ConnectionSnapshot {
	return ConnectionSnapshot{
		ID:      id,
		Name:    name,
		Type:    ConnectionTypePVE,
		State:   state,
		Enabled: true,
	}
}

func TestCheckConnection(t *testing.T) {
	t.Run("active state never fires", func(t *testing.T) {
		m := newTestManager(t)
		snap := platformConnectionSnapshot("conn-1", "Delly PVE", ConnectionStateActive)

		for i := 0; i < 5; i++ {
			m.CheckConnection(snap)
		}

		alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)
		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected no alert for active connection")
		}
		m.mu.RLock()
		count := testCoreConfirmations(m, snap.ID, canonicalDiscreteStateSpecID(snap.ID, connectionDegradedStateKey))
		m.mu.RUnlock()
		if count != 0 {
			t.Errorf("expected degraded count 0, got %d", count)
		}
	})

	t.Run("three consecutive stale observations fire a warning", func(t *testing.T) {
		m := newTestManager(t)
		snap := platformConnectionSnapshot("conn-stale", "Delly PVE", ConnectionStateStale)
		snap.StateReason = "no successful poll in 5m"

		alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)

		m.CheckConnection(snap)
		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected no alert after 1 stale observation")
		}
		m.CheckConnection(snap)
		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected no alert after 2 stale observations")
		}
		m.CheckConnection(snap)

		alert := testRequireActiveAlert(t, m, alertID)
		if alert.Level != AlertLevelWarning {
			t.Errorf("expected warning level for stale, got %s", alert.Level)
		}
		if alert.Type != connectionDegradedAlertType {
			t.Errorf("expected alert type %s, got %s", connectionDegradedAlertType, alert.Type)
		}
		if alert.ResourceID != snap.ID {
			t.Errorf("expected resourceID %s, got %s", snap.ID, alert.ResourceID)
		}
		if got, _ := alert.Metadata["connectionType"].(string); got != string(ConnectionTypePVE) {
			t.Errorf("expected connectionType=pve in metadata, got %q", got)
		}
		if got, _ := alert.Metadata["state"].(string); got != string(ConnectionStateStale) {
			t.Errorf("expected state=stale in metadata, got %q", got)
		}
	})

	t.Run("unreachable escalates an already-firing warning to critical", func(t *testing.T) {
		m := newTestManager(t)
		snap := platformConnectionSnapshot("conn-esc", "Delly PVE", ConnectionStateStale)
		alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)

		// Build up to a firing warning first.
		m.CheckConnection(snap)
		m.CheckConnection(snap)
		m.CheckConnection(snap)
		warningAlert := testRequireActiveAlert(t, m, alertID)
		if warningAlert.Level != AlertLevelWarning {
			t.Fatalf("expected warning alert before escalation, got %s", warningAlert.Level)
		}

		// Now the connection goes fully unreachable.
		snap.State = ConnectionStateUnreachable
		snap.StateReason = "context deadline exceeded"
		m.CheckConnection(snap)

		escalated := testRequireActiveAlert(t, m, alertID)
		if escalated.Level != AlertLevelCritical {
			t.Errorf("expected critical level after escalation, got %s", escalated.Level)
		}
		if got, _ := escalated.Metadata["state"].(string); got != string(ConnectionStateUnreachable) {
			t.Errorf("expected state=unreachable in metadata, got %q", got)
		}
	})

	t.Run("unauthorized fires critical from cold start", func(t *testing.T) {
		m := newTestManager(t)
		snap := platformConnectionSnapshot("conn-auth", "Delly PVE", ConnectionStateUnauthorized)
		snap.StateReason = "invalid api key"

		alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)

		m.CheckConnection(snap)
		m.CheckConnection(snap)
		m.CheckConnection(snap)

		alert := testRequireActiveAlert(t, m, alertID)
		if alert.Level != AlertLevelCritical {
			t.Errorf("expected critical level for unauthorized, got %s", alert.Level)
		}
	})

	t.Run("paused connection never fires", func(t *testing.T) {
		m := newTestManager(t)
		snap := platformConnectionSnapshot("conn-paused", "Delly PVE", ConnectionStatePaused)

		for i := 0; i < 5; i++ {
			m.CheckConnection(snap)
		}

		alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)
		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected no alert for paused connection")
		}
	})

	t.Run("disabled connection never fires even when unreachable", func(t *testing.T) {
		m := newTestManager(t)
		snap := platformConnectionSnapshot("conn-disabled", "Delly PVE", ConnectionStateUnreachable)
		snap.Enabled = false

		for i := 0; i < 5; i++ {
			m.CheckConnection(snap)
		}

		alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)
		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected no alert for disabled connection")
		}
	})

	t.Run("agent connection is dropped before reaching the alert path", func(t *testing.T) {
		m := newTestManager(t)
		// Synthesize a non-platform snapshot directly — agents and
		// availability targets never reach CheckConnection in production
		// because the aggregator drops them, but a defense-in-depth test
		// here guards the type guard.
		snap := ConnectionSnapshot{
			ID:      "agent-1",
			Name:    "Host agent",
			Type:    ConnectionType("agent"),
			State:   ConnectionStateUnreachable,
			Enabled: true,
		}

		for i := 0; i < 5; i++ {
			m.CheckConnection(snap)
		}

		alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)
		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected no alert for non-platform connection type")
		}
		m.mu.RLock()
		count := testCoreConfirmations(m, snap.ID, canonicalDiscreteStateSpecID(snap.ID, connectionDegradedStateKey))
		m.mu.RUnlock()
		if count != 0 {
			t.Errorf("expected degraded count untouched for non-platform type, got %d", count)
		}
	})
}

func TestCheckConnectionHonorsOwningResourceAvailabilityPolicy(t *testing.T) {
	pbs, adapter, canonicalID := newPBSOfflinePolicyFixture(t)
	connectionID := "pbs:" + pbs.Name
	resolvedPolicyID, ok := adapter.ResolveCanonicalResourceID(pbs.ID)
	if !ok || resolvedPolicyID != canonicalID {
		t.Fatalf("PBS policy ID %q resolved to %q, %t; want %q", pbs.ID, resolvedPolicyID, ok, canonicalID)
	}

	newPBSConnectionManager := func(t *testing.T) (*Manager, chan *Alert) {
		t.Helper()
		m := newTestManager(t)
		m.SetResourceIntentIdentityResolver(adapter.ResolveCanonicalResourceID)
		cfg := m.GetConfig()
		cfg.ActivationState = ActivationActive
		m.UpdateConfig(cfg)
		fired := make(chan *Alert, 4)
		m.SetAlertCallback(func(alert *Alert) { fired <- alert })
		return m, fired
	}
	snap := ConnectionSnapshot{
		ID:               connectionID,
		PolicyResourceID: pbs.ID,
		Name:             pbs.Name,
		Type:             ConnectionTypePBS,
		State:            ConnectionStateUnreachable,
		Enabled:          true,
	}
	alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)

	t.Run("canonical per-resource offline toggle blocks alert and notification", func(t *testing.T) {
		m, fired := newPBSConnectionManager(t)
		cfg := m.GetConfig()
		cfg.Overrides = map[string]ThresholdConfig{
			canonicalID: {DisableConnectivity: true},
		}
		m.UpdateConfig(cfg)

		for range 5 {
			m.CheckConnection(snap)
		}

		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("offline-disabled PBS created a connection-degraded alert")
		}
		m.mu.RLock()
		tracked := testCoreHasIncident(m, snap.ID, canonicalDiscreteStateSpecID(snap.ID, connectionDegradedStateKey))
		m.mu.RUnlock()
		if tracked {
			t.Fatal("offline-disabled PBS retained connection-degraded confirmation state")
		}
		select {
		case alert := <-fired:
			t.Fatalf("offline-disabled PBS dispatched %q", alert.ID)
		default:
		}
	})

	t.Run("global PBS offline toggle blocks the parallel detector", func(t *testing.T) {
		m, fired := newPBSConnectionManager(t)
		cfg := m.GetConfig()
		cfg.DisableAllPBSOffline = true
		m.UpdateConfig(cfg)

		for range 5 {
			m.CheckConnection(snap)
		}

		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("global PBS offline toggle left connection-degraded active")
		}
		select {
		case alert := <-fired:
			t.Fatalf("global PBS offline toggle dispatched %q", alert.ID)
		default:
		}
	})

	t.Run("enabling the toggle immediately resolves a standing alert", func(t *testing.T) {
		m, fired := newPBSConnectionManager(t)
		for range 3 {
			m.CheckConnection(snap)
		}
		testRequireActiveAlert(t, m, alertID)
		select {
		case <-fired:
		default:
			t.Fatal("enabled PBS connection did not dispatch its firing alert")
		}

		cfg := m.GetConfig()
		cfg.Overrides = map[string]ThresholdConfig{
			canonicalID: {DisableConnectivity: true},
		}
		m.UpdateConfig(cfg)

		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("policy update did not immediately resolve connection-degraded")
		}
		m.mu.RLock()
		tracked := testCoreHasIncident(m, snap.ID, canonicalDiscreteStateSpecID(snap.ID, connectionDegradedStateKey))
		m.mu.RUnlock()
		if tracked {
			t.Fatal("policy update left connection-degraded confirmation state")
		}
	})

	t.Run("expected-offline intent blocks both state and notification", func(t *testing.T) {
		m, fired := newPBSConnectionManager(t)
		m.SetOperatorIntentContextResolver(func(resourceID string, observedAt time.Time) (OperatorIntentContext, bool) {
			if resourceID != connectionID {
				return OperatorIntentContext{}, false
			}
			return OperatorIntentContext{MonitoringMode: "expected_offline", LifecycleState: "active"}, true
		})

		for range 5 {
			m.CheckConnection(snap)
		}

		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected-offline PBS created a connection-degraded alert")
		}
		select {
		case alert := <-fired:
			t.Fatalf("expected-offline PBS dispatched %q", alert.ID)
		default:
		}
	})
}

func TestConnectionDegradedUsesOfflineNotificationPolicy(t *testing.T) {
	alert := &Alert{Type: connectionDegradedAlertType}
	if got := quietHoursCategoryForAlert(alert); got != "offline" {
		t.Fatalf("connection-degraded quiet-hours category = %q, want offline", got)
	}
}

func TestConnectionDegradedPolicyUsesEveryPlatformResourceOverride(t *testing.T) {
	tests := []struct {
		name           string
		connectionType ConnectionType
		resourceID     string
	}{
		{name: "PVE", connectionType: ConnectionTypePVE, resourceID: "pve:lab"},
		{name: "PBS", connectionType: ConnectionTypePBS, resourceID: "pbs:backup"},
		{name: "PMG", connectionType: ConnectionTypePMG, resourceID: "pmg:mail"},
		{name: "VMware", connectionType: ConnectionTypeVMware, resourceID: "vmware:vcenter"},
		{name: "TrueNAS", connectionType: ConnectionTypeTrueNAS, resourceID: "truenas:nas"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := newTestManager(t)
			cfg := m.GetConfig()
			cfg.Overrides = map[string]ThresholdConfig{
				test.resourceID: {DisableConnectivity: true},
			}
			m.UpdateConfig(cfg)

			snap := ConnectionSnapshot{
				ID:      test.resourceID,
				Name:    test.name,
				Type:    test.connectionType,
				State:   ConnectionStateUnreachable,
				Enabled: true,
			}
			for range 5 {
				m.CheckConnection(snap)
			}

			alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)
			if testHasActiveAlert(t, m, alertID) {
				t.Fatalf("%s connectivity-disabled override created connection-degraded", test.name)
			}
		})
	}
}

func TestClearConnectionDegradedAlert(t *testing.T) {
	t.Run("clears an active alert after the recovery confirmation gate", func(t *testing.T) {
		m := newTestManager(t)
		resolvedCh := make(chan struct{}, 1)
		m.SetResolvedCallback(func(alertID string) {
			resolvedCh <- struct{}{}
		})

		snap := platformConnectionSnapshot("conn-recover", "Delly PVE", ConnectionStateStale)
		alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)

		// Drive the alert to firing.
		m.CheckConnection(snap)
		m.CheckConnection(snap)
		m.CheckConnection(snap)
		testRequireActiveAlert(t, m, alertID)

		// State returns to active — first observation does NOT clear yet
		// because the recovery confirmation gate is 3.
		snap.State = ConnectionStateActive
		m.CheckConnection(snap)
		if !testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected alert to remain active until recovery confirmations are met")
		}
		m.mu.RLock()
		count1 := testCoreRecoveryCount(m, snap.ID, canonicalDiscreteStateSpecID(snap.ID, connectionDegradedStateKey))
		m.mu.RUnlock()
		if count1 != 1 {
			t.Errorf("expected recovery confirmation 1, got %d", count1)
		}

		m.CheckConnection(snap)
		if !testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected alert to remain active until recovery confirmations are met")
		}

		m.CheckConnection(snap)
		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected alert to clear after 3 recovery confirmations")
		}

		// Confirm the resolved callback fires (handled via the safeCall path).
		select {
		case <-resolvedCh:
		case <-time.After(2 * time.Second):
			t.Fatal("expected resolved callback to fire")
		}
	})

	t.Run("no-op when nothing was ever firing", func(t *testing.T) {
		m := newTestManager(t)
		snap := platformConnectionSnapshot("conn-quiet", "Delly PVE", ConnectionStateActive)

		m.CheckConnection(snap)
		m.CheckConnection(snap)

		m.mu.RLock()
		alertCount := len(m.activeAlerts)
		recovery := testCoreRecoveryCount(m, snap.ID, canonicalDiscreteStateSpecID(snap.ID, connectionDegradedStateKey))
		m.mu.RUnlock()

		if alertCount != 0 {
			t.Errorf("expected no active alerts, got %d", alertCount)
		}
		if recovery != 0 {
			t.Errorf("expected no recovery confirmations tracked, got %d", recovery)
		}
	})

	t.Run("degraded observation resets in-flight recovery confirmations", func(t *testing.T) {
		m := newTestManager(t)
		snap := platformConnectionSnapshot("conn-flap", "Delly PVE", ConnectionStateStale)
		alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)

		// Drive to firing.
		m.CheckConnection(snap)
		m.CheckConnection(snap)
		m.CheckConnection(snap)
		testRequireActiveAlert(t, m, alertID)

		// One healthy observation builds up a recovery confirmation.
		snap.State = ConnectionStateActive
		m.CheckConnection(snap)
		m.mu.RLock()
		recoveryBefore := testCoreRecoveryCount(m, snap.ID, canonicalDiscreteStateSpecID(snap.ID, connectionDegradedStateKey))
		m.mu.RUnlock()
		if recoveryBefore != 1 {
			t.Fatalf("expected 1 recovery confirmation, got %d", recoveryBefore)
		}

		// A degraded blip should wipe the recovery progress so a single
		// flap doesn't pretend the outage is healed.
		snap.State = ConnectionStateStale
		m.CheckConnection(snap)
		m.mu.RLock()
		recoveryAfter := testCoreRecoveryCount(m, snap.ID, canonicalDiscreteStateSpecID(snap.ID, connectionDegradedStateKey))
		m.mu.RUnlock()
		if recoveryAfter != 0 {
			t.Errorf("expected recovery confirmations reset to 0, got %d", recoveryAfter)
		}
		if !testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected alert to stay active after flap")
		}
	})
}

func TestConnectionAlertReFireCooldown(t *testing.T) {
	t.Run("re-fire within cooldown does not create duplicate history entry", func(t *testing.T) {
		m := newTestManager(t)
		snap := platformConnectionSnapshot("conn-cooldown", "Delly PVE", ConnectionStateUnreachable)
		snap.StateReason = "context deadline exceeded"
		alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)

		m.CheckConnection(snap)
		m.CheckConnection(snap)
		m.CheckConnection(snap)
		originalAlert := testRequireActiveAlert(t, m, alertID)
		originalStart := originalAlert.StartTime

		historyAfterFire := len(m.historyManager.GetAllHistory(1000))

		snap.State = ConnectionStateActive
		m.CheckConnection(snap)
		m.CheckConnection(snap)
		m.CheckConnection(snap)
		if testHasActiveAlert(t, m, alertID) {
			t.Fatal("expected alert to clear after recovery")
		}

		snap.State = ConnectionStateUnreachable
		m.CheckConnection(snap)
		m.CheckConnection(snap)
		m.CheckConnection(snap)
		reactivated := testRequireActiveAlert(t, m, alertID)

		historyAfterReFire := len(m.historyManager.GetAllHistory(1000))
		if historyAfterReFire != historyAfterFire {
			t.Errorf("expected %d history entries after re-fire (same as after initial fire), got %d", historyAfterFire, historyAfterReFire)
		}

		if !reactivated.StartTime.Equal(originalStart) {
			t.Errorf("expected reactivated alert to preserve original StartTime %v, got %v", originalStart, reactivated.StartTime)
		}
	})
}
