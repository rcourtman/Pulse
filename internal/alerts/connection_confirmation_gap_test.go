package alerts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func interruptConnectionSnapshot(m *Manager, snap ConnectionSnapshot, gap string) {
	switch gap {
	case "pending":
		snap.State = ConnectionStatePending
	case "unknown":
		snap.State = ConnectionState("unknown")
	case "healthy":
		snap.State = ConnectionStateActive
	case "paused":
		snap.State = ConnectionStatePaused
	case "disabled":
		snap.Enabled = false
	case "offline-policy":
		m.mu.Lock()
		m.config.Overrides[snap.ID] = ThresholdConfig{DisableConnectivity: true}
		m.mu.Unlock()
	}
	m.CheckConnection(snap)
	if gap == "offline-policy" {
		m.mu.Lock()
		delete(m.config.Overrides, snap.ID)
		m.mu.Unlock()
	}
}

func TestConnectionConfirmationInterrupted(t *testing.T) {
	for _, platform := range []ConnectionType{ConnectionTypePVE, ConnectionTypePBS, ConnectionTypePMG, ConnectionTypeVMware, ConnectionTypeTrueNAS} {
		for _, gap := range []string{"pending", "unknown", "healthy", "paused", "disabled", "offline-policy"} {
			t.Run(string(platform)+"/"+gap, func(t *testing.T) {
				m := newShadowFeedManager(t)
				snap := platformConnectionSnapshot("connection-a", "Test connection", ConnectionStateStale)
				snap.Type = platform
				other := snap
				other.ID = "connection-ab"
				key := canonicalDiscreteStateSpecID(snap.ID, connectionDegradedStateKey)
				alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)
				for range 2 {
					m.CheckConnection(snap)
					m.CheckConnection(other)
				}
				interruptConnectionSnapshot(m, snap, gap)
				m.mu.RLock()
				pending := testCoreHasIncident(m, snap.ID, key)
				otherCount := testCoreConfirmations(m, other.ID, canonicalDiscreteStateSpecID(other.ID, connectionDegradedStateKey))
				m.mu.RUnlock()
				if pending {
					t.Error("interrupted observation retained activation confirmations")
				}
				if otherCount != 2 {
					t.Errorf("interruption changed another connection's count to %d", otherCount)
				}
				for range 2 {
					m.CheckConnection(snap)
					if testHasActiveAlert(t, m, alertID) {
						t.Fatal("non-consecutive degraded observations fired an alert")
					}
				}
				m.CheckConnection(snap)
				testRequireActiveAlert(t, m, alertID)
				m.CheckConnection(other)
				testRequireActiveAlert(t, m, canonicalDiscreteStateStateID(other.ID, connectionDegradedStateKey))
				if n := m.ShadowDivergences(); n != 0 {
					t.Fatalf("shadow diverged %d times", n)
				}
			})
		}
	}
}

func TestConnectionUnknownInterruptsRecoveryWithoutResolving(t *testing.T) {
	for _, gap := range []string{"pending", "unknown"} {
		t.Run(gap, func(t *testing.T) {
			m := newShadowFeedManager(t)
			var fires, resolutions atomic.Int32
			resolved := make(chan struct{}, 2)
			m.mu.Lock()
			m.config.ActivationState = ActivationActive
			m.mu.Unlock()
			m.SetAlertCallback(func(*Alert) { fires.Add(1) })
			m.SetResolvedCallback(func(string) {
				resolutions.Add(1)
				resolved <- struct{}{}
			})
			snap := platformConnectionSnapshot("connection-a", "Test connection", ConnectionStateStale)
			alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)
			for range 3 {
				m.CheckConnection(snap)
			}
			if err := m.AcknowledgeAlert(alertID, "test-operator"); err != nil {
				t.Fatal(err)
			}
			snap.State = ConnectionStateActive
			for range 2 {
				m.CheckConnection(snap)
			}
			before := testRequireActiveAlert(t, m, alertID).Clone()
			interruptConnectionSnapshot(m, snap, gap)
			after := testRequireActiveAlert(t, m, alertID).Clone()
			if !reflect.DeepEqual(before, after) {
				t.Fatal("unknown observation changed the acknowledged active occurrence")
			}
			for range 2 {
				m.CheckConnection(snap)
				if !testHasActiveAlert(t, m, alertID) {
					t.Fatal("non-consecutive healthy observations resolved the outage")
				}
			}
			if fires.Load() != 1 || resolutions.Load() != 0 {
				t.Fatalf("unexpected callbacks: fires=%d resolutions=%d", fires.Load(), resolutions.Load())
			}
			m.CheckConnection(snap)
			if testHasActiveAlert(t, m, alertID) {
				t.Fatal("three fresh healthy observations did not resolve")
			}
			select {
			case <-resolved:
			case <-time.After(2 * time.Second):
				t.Fatal("confirmed recovery did not dispatch its resolved callback")
			}
			if fires.Load() != 1 || resolutions.Load() != 1 {
				t.Fatalf("unexpected final callbacks: fires=%d resolutions=%d", fires.Load(), resolutions.Load())
			}
			if n := m.ShadowDivergences(); n != 0 {
				t.Fatalf("shadow diverged %d times", n)
			}
		})
	}
}

func TestConnectionInterruptionRestartsIntentGrace(t *testing.T) {
	for _, gap := range []string{"pending", "unknown", "healthy", "paused", "disabled", "offline-policy"} {
		t.Run(gap, func(t *testing.T) {
			m := newShadowFeedManager(t)
			var tick time.Duration
			m.mu.Lock()
			m.intentClock = func() time.Duration { return tick }
			m.mu.Unlock()
			grace := 60
			document := NewAlertIntentPolicyDocument()
			document.Defaults[string(AlertIntentSignalOffline)] = AlertIntentRule{GraceSeconds: &grace}
			if err := m.LoadIntentPolicies(document); err != nil {
				t.Fatal(err)
			}
			snap := platformConnectionSnapshot("connection-a", "Test connection", ConnectionStateStale)
			alertID := canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey)
			m.CheckConnection(snap)
			tick = 40 * time.Second
			m.CheckConnection(snap)
			if err := m.SaveActiveAlerts(); err != nil {
				t.Fatal(err)
			}
			interruptConnectionSnapshot(m, snap, gap)
			m.mu.RLock()
			_, pending := m.intentPending[alertID]
			_, ticking := m.intentRuntimeTicks[alertID]
			m.mu.RUnlock()
			if pending || ticking {
				t.Error("interrupted observation retained intent grace bookkeeping")
			}
			if err := m.SaveActiveAlerts(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(m.getAlertsDir(), intentPendingFileName))
			if err != nil {
				t.Fatal(err)
			}
			var saved []IntentPendingState
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if len(saved) != 0 {
				t.Fatal("checkpoint retained interrupted intent grace for restart")
			}
			for _, next := range []time.Duration{70, 90, 129} {
				tick = next * time.Second
				m.CheckConnection(snap)
				if testHasActiveAlert(t, m, alertID) {
					t.Fatal("intent grace accrued across an observation gap")
				}
			}
			tick = 130 * time.Second
			m.CheckConnection(snap)
			testRequireActiveAlert(t, m, alertID)
			if n := m.ShadowDivergences(); n != 0 {
				t.Fatalf("shadow diverged %d times", n)
			}
		})
	}
}
