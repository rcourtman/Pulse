package alerts

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Recovery-gate parity: the manager's poll-driven offline composition —
// an offline poll deletes the recovery counter and runs
// evaluateCanonicalLifecycleAlert with a connectivity spec; a healthy poll
// skips the evaluator and runs clearResourceOfflineAlert instead (the
// pbs.go / node.go / storage.go shape) — diffed against the reducer's
// DiscreteRule.RecoveryConfirmations after every step. Resolve timestamps
// are not asserted (clearResourceOfflineAlert stamps wall-clock time);
// firing state, severity, and first-activation StartTime are.

type recoveryParityStep struct {
	offline bool
	advance time.Duration

	wantFiring bool
	// wantStartOffset asserts the firing alert's StartTime as an offset
	// from the scenario's first observation; -1 skips.
	wantStartOffset time.Duration
}

type recoveryParityScenario struct {
	name          string
	confirmations int
	recovery      int
	steps         []recoveryParityStep
}

const recoveryParityResourceID = "parity-pbs-1"

type managerRecoveryParityEngine struct {
	manager  *Manager
	clock    time.Time
	conf     int
	recovery int
}

func (e *managerRecoveryParityEngine) observe(t *testing.T, step recoveryParityStep) {
	t.Helper()
	e.clock = e.clock.Add(step.advance)

	if !step.offline {
		e.manager.clearResourceOfflineAlert(recoveryParityResourceID, "parity-pbs", "host-1", "PBS", e.recovery)
		return
	}

	e.manager.mu.Lock()
	e.manager.mu.Unlock()

	spec, err := buildCanonicalConnectivitySpec(
		recoveryParityResourceID, "parity-pbs", unifiedresources.ResourceTypePBS,
		AlertLevelCritical, e.conf, false,
	)
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}
	if _, ok := e.manager.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt:   e.clock,
			Connectivity: &alertspecs.ConnectivityEvidence{Signal: "status", Connected: false},
		},
		AlertID:      fmt.Sprintf("pbs-offline-%s", recoveryParityResourceID),
		AlertType:    "offline",
		ResourceID:   recoveryParityResourceID,
		ResourceName: "parity-pbs",
		Node:         "host-1",
		Instance:     "parity-pbs",
		Message:      "parity PBS offline",
	}); !ok {
		t.Fatal("evaluateCanonicalLifecycleAlert rejected the connectivity spec")
	}
}

func (e *managerRecoveryParityEngine) snapshot() (bool, AlertLevel, time.Time) {
	e.manager.mu.Lock()
	defer e.manager.mu.Unlock()
	alert, exists := e.manager.getActiveAlertNoLock(canonicalConnectivityStateID(recoveryParityResourceID))
	if !exists || alert == nil {
		return false, "", time.Time{}
	}
	return true, alert.Level, alert.StartTime
}

func recoveryParityScenarios() []recoveryParityScenario {
	skip := time.Duration(-1)
	return []recoveryParityScenario{
		{
			name:          "recovery gate holds until three consecutive healthy polls",
			confirmations: 3, recovery: 3,
			steps: []recoveryParityStep{
				{offline: true, wantFiring: false, wantStartOffset: skip},
				{offline: true, advance: 30 * time.Second, wantFiring: false, wantStartOffset: skip},
				{offline: true, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 0},
				{offline: false, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 0},
				{offline: false, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 0},
				{offline: false, advance: 30 * time.Second, wantFiring: false, wantStartOffset: skip},
			},
		},
		{
			name:          "an offline poll mid-recovery resets the healthy run",
			confirmations: 1, recovery: 3,
			steps: []recoveryParityStep{
				{offline: true, wantFiring: true, wantStartOffset: 0},
				{offline: false, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 0},
				{offline: false, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 0},
				{offline: true, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 0},
				{offline: false, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 0},
				{offline: false, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 0},
				{offline: false, advance: 30 * time.Second, wantFiring: false, wantStartOffset: skip},
			},
		},
		{
			name:          "storage-style two-poll recovery",
			confirmations: 2, recovery: 2,
			steps: []recoveryParityStep{
				{offline: true, wantFiring: false, wantStartOffset: skip},
				{offline: true, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 0},
				{offline: false, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 0},
				{offline: false, advance: 30 * time.Second, wantFiring: false, wantStartOffset: skip},
			},
		},
		{
			name:          "healthy poll during pending clears immediately and re-fire re-dates",
			confirmations: 3, recovery: 3,
			steps: []recoveryParityStep{
				{offline: true, wantFiring: false, wantStartOffset: skip},
				{offline: true, advance: 30 * time.Second, wantFiring: false, wantStartOffset: skip},
				{offline: false, advance: 30 * time.Second, wantFiring: false, wantStartOffset: skip},
				{offline: true, advance: 30 * time.Second, wantFiring: false, wantStartOffset: skip},
				{offline: true, advance: 30 * time.Second, wantFiring: false, wantStartOffset: skip},
				{offline: true, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 90 * time.Second},
			},
		},
	}
}

func TestReducerParityWithManagerRecoveryGate(t *testing.T) {
	epoch := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

	for _, scenario := range recoveryParityScenarios() {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
			t.Cleanup(manager.Stop)
			manager.mu.Lock()
			manager.config.Enabled = true
			manager.config.FlappingEnabled = false
			manager.mu.Unlock()

			managerEngine := &managerRecoveryParityEngine{
				manager: manager, clock: epoch,
				conf: scenario.confirmations, recovery: scenario.recovery,
			}
			reducerState := reducer.NewState()
			reducerClock := epoch
			rule := reducer.DiscreteRule{
				Confirmations:         scenario.confirmations,
				RecoveryConfirmations: scenario.recovery,
			}

			for i, step := range scenario.steps {
				managerEngine.observe(t, step)
				reducerClock = reducerClock.Add(step.advance)
				reducerState.ApplyDiscrete(reducer.DiscreteSignal{
					ResourceID: recoveryParityResourceID,
					Key:        "connectivity",
					Matched:    step.offline,
					Severity:   reducer.SeverityCritical,
					ObservedAt: reducerClock,
				}, rule)

				managerFiring, managerLevel, managerStart := managerEngine.snapshot()
				incident, ok := reducerState.Incident(recoveryParityResourceID, "connectivity")
				reducerFiring := ok && incident.State == reducer.StateFiring

				label := fmt.Sprintf("step %d (offline=%v advance=%s)", i, step.offline, step.advance)

				if managerFiring != step.wantFiring {
					t.Fatalf("%s: manager firing = %v, scenario expects %v (characterization drift)",
						label, managerFiring, step.wantFiring)
				}
				if managerFiring && step.wantStartOffset >= 0 {
					wantStart := epoch.Add(step.wantStartOffset)
					if !managerStart.Equal(wantStart) {
						t.Fatalf("%s: manager start = %v, scenario expects %v", label, managerStart, wantStart)
					}
				}

				if reducerFiring != managerFiring {
					t.Fatalf("%s: PARITY DIVERGENCE — reducer firing = %v, manager = %v",
						label, reducerFiring, managerFiring)
				}
				if managerFiring {
					if AlertLevel(incident.Severity) != managerLevel {
						t.Fatalf("%s: PARITY DIVERGENCE — reducer severity = %q, manager = %q",
							label, incident.Severity, managerLevel)
					}
					if step.wantStartOffset >= 0 && !incident.StartedAt.Equal(managerStart) {
						t.Fatalf("%s: PARITY DIVERGENCE — reducer start = %v, manager = %v",
							label, incident.StartedAt, managerStart)
					}
				}
			}
		})
	}
}
