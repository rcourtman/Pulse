package alerts

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Ack-lifecycle parity: acknowledge suppression state must survive alert
// rebuilds (preserveAlertState's existing branch), survive short
// resolve/re-fire cycles via the canonical ack records
// (preserveAlertState's restore branch), be removed by unacknowledge, and
// expire once cleanup prunes an inactive record (one-hour TTL). The
// manager's restore has no age check of its own — expiry is cleanup — so
// the expiry scenario backdates the record and runs Cleanup explicitly,
// mirroring how the hour boundary is enforced in production.

type ackParityStep struct {
	action  string // "observe", "ack", "unack", "cleanup"
	matched bool
	advance time.Duration

	wantFiring bool
	wantAcked  bool
}

type ackParityScenario struct {
	name  string
	steps []ackParityStep
}

const ackParityResourceID = "parity-ack-1"

type managerAckParityEngine struct {
	manager *Manager
	clock   time.Time
}

func (e *managerAckParityEngine) alertID() string {
	return canonicalDiscreteStateStateID(ackParityResourceID, "parity-state")
}

func (e *managerAckParityEngine) shift(advance time.Duration) {
	if advance <= 0 {
		return
	}
	e.manager.resolvedMutex.Lock()
	for _, resolved := range e.manager.recentlyResolved {
		if resolved != nil {
			resolved.ResolvedTime = resolved.ResolvedTime.Add(-advance)
		}
	}
	e.manager.resolvedMutex.Unlock()
	e.manager.mu.Lock()
	for key, record := range e.manager.ackState {
		record.time = record.time.Add(-advance)
		if !record.inactiveAt.IsZero() {
			record.inactiveAt = record.inactiveAt.Add(-advance)
		}
		e.manager.ackState[key] = record
	}
	for key, record := range e.manager.ackStateByCanonical {
		record.time = record.time.Add(-advance)
		if !record.inactiveAt.IsZero() {
			record.inactiveAt = record.inactiveAt.Add(-advance)
		}
		e.manager.ackStateByCanonical[key] = record
	}
	e.manager.mu.Unlock()
}

func (e *managerAckParityEngine) step(t *testing.T, s ackParityStep) {
	t.Helper()
	e.clock = e.clock.Add(s.advance)
	e.shift(s.advance)

	switch s.action {
	case "ack":
		if err := e.manager.AcknowledgeAlert(e.alertID(), "richard"); err != nil {
			t.Fatalf("AcknowledgeAlert: %v", err)
		}
		return
	case "unack":
		if err := e.manager.UnacknowledgeAlert(e.alertID()); err != nil {
			t.Fatalf("UnacknowledgeAlert: %v", err)
		}
		return
	case "cleanup":
		e.manager.Cleanup(24 * time.Hour)
		return
	}

	spec, err := buildCanonicalDiscreteStateSpec(
		ackParityResourceID, "parity-ack", unifiedresources.ResourceTypeAgent,
		AlertLevelWarning, 1, false, "parity-state", []string{"bad"},
	)
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}
	observed := "ok"
	if s.matched {
		observed = "bad"
	}
	if _, ok := e.manager.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt:    e.clock,
			DiscreteState: &alertspecs.DiscreteStateEvidence{StateKey: "parity-state", Observed: observed},
		},
		AlertID:      e.alertID(),
		AlertType:    "parity-state",
		ResourceID:   ackParityResourceID,
		ResourceName: "parity-ack",
		Message:      "parity ack condition",
	}); !ok {
		t.Fatal("evaluateCanonicalLifecycleAlert rejected the parity spec")
	}
}

func ackParityScenarios() []ackParityScenario {
	return []ackParityScenario{
		{
			name: "ack survives rebuilds and unack clears it",
			steps: []ackParityStep{
				{action: "observe", matched: true, wantFiring: true, wantAcked: false},
				{action: "ack", wantFiring: true, wantAcked: true},
				{action: "observe", matched: true, advance: 30 * time.Second, wantFiring: true, wantAcked: true},
				{action: "unack", wantFiring: true, wantAcked: false},
				{action: "observe", matched: true, advance: 30 * time.Second, wantFiring: true, wantAcked: false},
			},
		},
		{
			name: "ack survives a short resolve and re-fire",
			steps: []ackParityStep{
				{action: "observe", matched: true, wantFiring: true, wantAcked: false},
				{action: "ack", wantFiring: true, wantAcked: true},
				{action: "observe", matched: false, advance: 30 * time.Second, wantFiring: false},
				{action: "observe", matched: true, advance: 30 * time.Second, wantFiring: true, wantAcked: true},
			},
		},
		{
			name: "ack expires after the inactive TTL",
			steps: []ackParityStep{
				{action: "observe", matched: true, wantFiring: true, wantAcked: false},
				{action: "ack", wantFiring: true, wantAcked: true},
				{action: "observe", matched: false, advance: 30 * time.Second, wantFiring: false},
				{action: "cleanup", advance: 70 * time.Minute, wantFiring: false},
				{action: "observe", matched: true, wantFiring: true, wantAcked: false},
			},
		},
	}
}

func TestReducerParityWithManagerAckLifecycle(t *testing.T) {
	for _, scenario := range ackParityScenarios() {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			epoch := time.Now().UTC().Truncate(time.Second)

			manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
			t.Cleanup(manager.Stop)
			manager.mu.Lock()
			manager.config.Enabled = true
			manager.config.FlappingEnabled = false
			manager.mu.Unlock()

			managerEngine := &managerAckParityEngine{manager: manager, clock: epoch}
			reducerState := reducer.NewState()
			reducerClock := epoch
			rule := reducer.DiscreteRule{Confirmations: 1}

			for i, step := range scenario.steps {
				managerEngine.step(t, step)
				reducerClock = reducerClock.Add(step.advance)
				switch step.action {
				case "ack":
					if !reducerState.Acknowledge(ackParityResourceID, "parity-state", "richard", reducerClock) {
						t.Fatalf("step %d: reducer Acknowledge failed", i)
					}
				case "unack":
					if !reducerState.Unacknowledge(ackParityResourceID, "parity-state") {
						t.Fatalf("step %d: reducer Unacknowledge failed", i)
					}
				case "cleanup":
					// The reducer needs no cleanup pass: AckRetention is
					// enforced deterministically at restore time.
				case "observe":
					reducerState.ApplyDiscrete(reducer.DiscreteSignal{
						ResourceID: ackParityResourceID,
						Key:        "parity-state",
						Matched:    step.matched,
						Severity:   reducer.SeverityWarning,
						ObservedAt: reducerClock,
					}, rule)
				}

				manager.mu.Lock()
				alert, exists := manager.getActiveAlertNoLock(managerEngine.alertID())
				manager.mu.Unlock()
				managerFiring := exists && alert != nil
				managerAcked := managerFiring && alert.Acknowledged
				incident, ok := reducerState.Incident(ackParityResourceID, "parity-state")
				reducerFiring := ok && incident.State == reducer.StateFiring
				reducerAcked := reducerFiring && incident.Acknowledged

				label := fmt.Sprintf("step %d (%s matched=%v advance=%s)", i, step.action, step.matched, step.advance)

				if managerFiring != step.wantFiring {
					t.Fatalf("%s: manager firing = %v, scenario expects %v (characterization drift)",
						label, managerFiring, step.wantFiring)
				}
				if managerFiring && managerAcked != step.wantAcked {
					t.Fatalf("%s: manager acked = %v, scenario expects %v", label, managerAcked, step.wantAcked)
				}

				if reducerFiring != managerFiring {
					t.Fatalf("%s: PARITY DIVERGENCE — reducer firing = %v, manager = %v",
						label, reducerFiring, managerFiring)
				}
				if managerFiring && reducerAcked != managerAcked {
					t.Fatalf("%s: PARITY DIVERGENCE — reducer acked = %v, manager = %v",
						label, reducerAcked, managerAcked)
				}
			}
		})
	}
}

func TestAutoAcknowledgementSurvivesMetricEvaluation(t *testing.T) {
	m := newTestManager(t)
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.SuppressionWindow = 0
	m.config.MinimumDelta = 0
	m.config.AutoAcknowledgeAfterHours = 2
	m.mu.Unlock()
	threshold := &HysteresisThreshold{Trigger: 80, Clear: 70}
	evaluate := func(value float64) {
		m.checkMetric("auto-ack-resource", "Resource", "node", "instance", "guest", "usage", value, threshold, nil)
	}
	evaluate(90)
	key := buildCanonicalStateID("auto-ack-resource", "metric-threshold:usage")
	m.mu.Lock()
	m.activeAlerts[key].StartTime = time.Now().Add(-3 * time.Hour)
	m.mu.Unlock()
	m.Cleanup(time.Hour)
	assertAcknowledged := func(stage string) {
		t.Helper()
		alerts := m.GetActiveAlerts()
		if len(alerts) != 1 || !alerts[0].Acknowledged || alerts[0].AckUser != "system-auto" || alerts[0].AckTime == nil {
			t.Fatalf("%s: automatic acknowledgement lost: %+v", stage, alerts)
		}
	}
	assertAcknowledged("cleanup")
	evaluate(85)
	assertAcknowledged("next evaluation")
	evaluate(60)
	evaluate(90)
	assertAcknowledged("short recovery and refire")
}

// Age only the acknowledgement timestamps, keeping detector observations fresh.
// The hourly sweep must not discard a long-running operator decision before a
// real recovery can start the existing inactive-retention window.
func TestTrackingCleanupPreservesAcknowledgedProviderRecurrence(t *testing.T) {
	for _, durable := range []bool{false, true} {
		for _, automatic := range []bool{false, true} {
			name := "JSON"
			if durable {
				name = "durable"
			}
			if automatic {
				name += "/automatic"
			} else {
				name += "/manual"
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				newManager := func() *Manager {
					return newCleanupRetentionManager(t, dir, durable)
				}
				m := newManager()
				resource := unifiedresources.Resource{
					ID: "storage:ack-tank", Type: unifiedresources.ResourceTypeStorage,
					Name: "ack-tank", ParentName: "truenas-main",
					Sources: []unifiedresources.DataSource{unifiedresources.SourceTrueNAS},
					Storage: &unifiedresources.StorageMeta{Platform: "truenas", Topology: "pool", Protection: "zfs", IsZFS: true},
					Incidents: []unifiedresources.ResourceIncident{{
						Provider: "truenas", NativeID: "ack-pool-alert", Code: "truenas_volume_status",
						Severity: storagehealth.RiskWarning, Summary: "Pool ack-tank is DEGRADED",
						StartedAt: time.Now().Add(-26 * time.Hour),
					}},
				}
				healthy := resource
				healthy.Incidents = nil
				evaluate := func(m *Manager, observed unifiedresources.Resource) {
					m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{observed})
				}
				var deliveries atomic.Int32
				m.SetAlertCallback(func(*Alert) { deliveries.Add(1) })
				evaluate(m, resource)
				initial := m.GetActiveAlerts()
				if len(initial) != 1 || deliveries.Load() != 1 {
					t.Fatalf("initial provider condition: %d alerts / %d dispatches, want 1 / 1", len(initial), deliveries.Load())
				}
				id := initial[0].ID
				user := "operator"
				if automatic {
					user = "system-auto"
					m.mu.Lock()
					m.config.AutoAcknowledgeAfterHours = 1
					m.mu.Unlock()
					m.Cleanup(time.Hour)
				} else if err := m.AcknowledgeAlert(id, user); err != nil {
					t.Fatal(err)
				}
				ackAt := time.Now().Add(-25 * time.Hour)
				m.mu.Lock()
				active, ok := m.getActiveAlertNoLock(id)
				if !ok || !active.Acknowledged {
					m.mu.Unlock()
					t.Fatal("detector acknowledgement was not established")
				}
				active.AckTime = &ackAt
				record := m.ackStateByCanonical[id]
				record.time = ackAt
				m.ackStateByCanonical[id] = record
				m.mirrorAcknowledgeNoLock(active, user, ackAt)
				m.setActiveAlertNoLock(id, active)
				m.mu.Unlock()

				checkCycle := func(m *Manager, count *atomic.Int32, stage string) {
					t.Helper()
					before := count.Load()
					m.cleanupStaleMaps()
					m.mu.RLock()
					record, retained := m.ackStateByCanonical[id]
					m.mu.RUnlock()
					if !retained || record.user != user || !record.time.Equal(ackAt) {
						t.Errorf("%s: hourly sweep lost the active acknowledgement: %+v, retained=%v", stage, record, retained)
					}
					evaluate(m, healthy) // Observed pool with no incident: affirmative recovery.
					if got := len(m.GetActiveAlerts()); got != 0 {
						t.Fatalf("%s: real recovery left %d active alerts", stage, got)
					}
					m.Cleanup(time.Hour) // Old acknowledgement, newly inactive: retain it.
					evaluate(m, resource)
					alerts := m.GetActiveAlerts()
					if len(alerts) != 1 || alerts[0].ID != id || !alerts[0].Acknowledged || alerts[0].AckUser != user || alerts[0].AckTime == nil || !alerts[0].AckTime.Equal(ackAt) {
						t.Errorf("%s: short recurrence lost acknowledgement identity: %+v", stage, alerts)
					}
					if got := count.Load() - before; got != 0 {
						t.Errorf("%s: acknowledged recovery/recurrence dispatched %d duplicate notifications", stage, got)
					}
					t.Logf("%s: recovery/recurrence dispatch delta %d", stage, count.Load()-before)
				}
				checkCycle(m, &deliveries, "before restart")
				m.Stop() // The actual final checkpoint, not a hand-built JSON fixture.
				restarted := newManager()
				var afterRestart atomic.Int32
				restarted.SetAlertCallback(func(*Alert) { afterRestart.Add(1) })
				checkCycle(restarted, &afterRestart, "after restart")
				if err := restarted.UnacknowledgeAlert(id); err != nil {
					t.Fatal(err)
				}
				before := afterRestart.Load()
				evaluate(restarted, healthy)
				evaluate(restarted, resource)
				if got := afterRestart.Load() - before; got != 1 {
					t.Errorf("explicit unacknowledge/recovery/recurrence dispatched %d notifications, want 1", got)
				}
			})
		}
	}
}

func TestTrackingCleanupCanonicalAckRetentionBounds(t *testing.T) {
	for _, tc := range []struct {
		name       string
		active     bool
		legacy     bool
		inactive   time.Duration
		wantHourly bool
		wantNormal bool
	}{
		{name: "active canonical", active: true, wantHourly: true, wantNormal: true},
		{name: "active legacy identity", active: true, legacy: true, wantHourly: true, wantNormal: true},
		{name: "active after earlier recurrence", active: true, inactive: 25 * time.Hour, wantHourly: true, wantNormal: true},
		{name: "recently inactive old acknowledgement", inactive: 30 * time.Minute, wantHourly: true, wantNormal: true},
		{name: "existing one-hour expiry", inactive: 2 * time.Hour, wantHourly: true},
		{name: "stale inactive", inactive: 25 * time.Hour},
		{name: "legacy missing inactive timestamp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)
			now := time.Now()
			old := now.Add(-25 * time.Hour)
			id, alert := testNewCanonicalAlert("ack-bounds", "metric-threshold:cpu", "metric-threshold", "cpu")
			alert.StartTime, alert.LastSeen = old, now
			alert.Acknowledged, alert.AckUser, alert.AckTime = true, "operator", &old
			record := ackRecord{acknowledged: true, user: "operator", time: old}
			if tc.inactive > 0 {
				record.inactiveAt = now.Add(-tc.inactive)
			}
			m.mu.Lock()
			m.ackStateByCanonical[id] = record
			if tc.active {
				storageKey := id
				if tc.legacy {
					storageKey = "legacy-cpu-ack-bounds"
				}
				m.activeAlerts[storageKey] = alert
			}
			m.mu.Unlock()
			assertRetained := func(stage string, want bool) {
				t.Helper()
				m.mu.RLock()
				got, retained := m.ackStateByCanonical[id]
				m.mu.RUnlock()
				if retained != want || (retained && got != record) {
					t.Errorf("%s: acknowledgement retained=%v (%+v), want %v with unchanged record", stage, retained, got, want)
				}
			}
			m.cleanupStaleMaps()
			assertRetained("hourly sweep", tc.wantHourly)
			m.Cleanup(time.Hour)
			assertRetained("normal one-hour inactive expiry", tc.wantNormal)
		})
	}
}
