package alerts

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
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
		Tracking:     e.manager.offlineConfirmations,
		TrackingKey:  "parity:" + ackParityResourceID,
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
