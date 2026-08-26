package alerts

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Re-fire parity: a resolve followed by a re-fire inside the
// recently-resolved retention window restores the ORIGINAL occurrence's
// StartTime (consumeRecentlyResolvedForRefireWithPrimaryLock); outside the
// window the re-fire is a fresh occurrence. The manager's retention check
// compares wall-clock now against ResolvedTime, so the harness anchors the
// simulated epoch at wall time and backdates recentlyResolved entries by
// each step's advance — the same trick the metric harness uses for
// pendingAlerts — making retention behave per simulated time while
// StartTime assertions stay exact.

type refireParityStep struct {
	matched bool
	advance time.Duration

	wantFiring      bool
	wantStartOffset time.Duration // -1 skips
}

type refireParityScenario struct {
	name          string
	confirmations int
	steps         []refireParityStep
}

const refireParityResourceID = "parity-refire-1"

type managerRefireParityEngine struct {
	manager *Manager
	clock   time.Time
	conf    int
}

func (e *managerRefireParityEngine) observe(t *testing.T, step refireParityStep) {
	t.Helper()
	e.clock = e.clock.Add(step.advance)

	if step.advance > 0 {
		// Shift resolved timestamps back so the wall-clock retention check
		// sees the simulated gap.
		e.manager.resolvedMutex.Lock()
		for _, resolved := range e.manager.recentlyResolved {
			if resolved != nil {
				resolved.ResolvedTime = resolved.ResolvedTime.Add(-step.advance)
			}
		}
		e.manager.resolvedMutex.Unlock()
	}

	spec, err := buildCanonicalDiscreteStateSpec(
		refireParityResourceID, "parity-refire", unifiedresources.ResourceTypeAgent,
		AlertLevelWarning, e.conf, false, "parity-state", []string{"bad"},
	)
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}
	observed := "ok"
	if step.matched {
		observed = "bad"
	}
	if _, ok := e.manager.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt:    e.clock,
			DiscreteState: &alertspecs.DiscreteStateEvidence{StateKey: "parity-state", Observed: observed},
		},
		Tracking:     e.manager.offlineConfirmations,
		TrackingKey:  "parity:" + refireParityResourceID,
		AlertID:      canonicalDiscreteStateStateID(refireParityResourceID, "parity-state"),
		AlertType:    "parity-state",
		ResourceID:   refireParityResourceID,
		ResourceName: "parity-refire",
		Message:      "parity refire condition",
	}); !ok {
		t.Fatal("evaluateCanonicalLifecycleAlert rejected the parity spec")
	}
}

func refireParityScenarios() []refireParityScenario {
	skip := time.Duration(-1)
	return []refireParityScenario{
		{
			name:          "re-fire within retention restores the original start",
			confirmations: 1,
			steps: []refireParityStep{
				{matched: true, wantFiring: true, wantStartOffset: 0},
				{matched: false, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: time.Minute, wantFiring: true, wantStartOffset: 0},
			},
		},
		{
			name:          "confirmation runs also restore across a short resolve gap",
			confirmations: 2,
			steps: []refireParityStep{
				{matched: true, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 0},
				{matched: false, advance: 30 * time.Second, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: 30 * time.Second, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: 30 * time.Second, wantFiring: true, wantStartOffset: 0},
			},
		},
		{
			name:          "re-fire outside retention is a fresh occurrence",
			confirmations: 1,
			steps: []refireParityStep{
				{matched: true, wantFiring: true, wantStartOffset: 0},
				{matched: false, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: 6 * time.Minute, wantFiring: true, wantStartOffset: 7 * time.Minute},
			},
		},
	}
}

func TestReducerParityWithManagerRefire(t *testing.T) {
	for _, scenario := range refireParityScenarios() {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			// Anchor at wall time so the manager's wall-clock retention
			// check and the simulated evidence clock agree.
			epoch := time.Now().UTC().Truncate(time.Second)

			manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
			t.Cleanup(manager.Stop)
			manager.mu.Lock()
			manager.config.Enabled = true
			manager.config.FlappingEnabled = false
			manager.mu.Unlock()

			managerEngine := &managerRefireParityEngine{manager: manager, clock: epoch, conf: scenario.confirmations}
			reducerState := reducer.NewState()
			reducerClock := epoch
			rule := reducer.DiscreteRule{Confirmations: scenario.confirmations}

			for i, step := range scenario.steps {
				managerEngine.observe(t, step)
				reducerClock = reducerClock.Add(step.advance)
				reducerState.ApplyDiscrete(reducer.DiscreteSignal{
					ResourceID: refireParityResourceID,
					Key:        "parity-state",
					Matched:    step.matched,
					Severity:   reducer.SeverityWarning,
					ObservedAt: reducerClock,
				}, rule)

				manager.mu.Lock()
				alert, exists := manager.getActiveAlertNoLock(
					canonicalDiscreteStateStateID(refireParityResourceID, "parity-state"))
				manager.mu.Unlock()
				managerFiring := exists && alert != nil
				incident, ok := reducerState.Incident(refireParityResourceID, "parity-state")
				reducerFiring := ok && incident.State == reducer.StateFiring

				label := fmt.Sprintf("step %d (matched=%v advance=%s)", i, step.matched, step.advance)

				if managerFiring != step.wantFiring {
					t.Fatalf("%s: manager firing = %v, scenario expects %v (characterization drift)",
						label, managerFiring, step.wantFiring)
				}
				if managerFiring && step.wantStartOffset >= 0 {
					wantStart := epoch.Add(step.wantStartOffset)
					if !alert.StartTime.Equal(wantStart) {
						t.Fatalf("%s: manager start = %v, scenario expects %v", label, alert.StartTime, wantStart)
					}
				}

				if reducerFiring != managerFiring {
					t.Fatalf("%s: PARITY DIVERGENCE — reducer firing = %v, manager = %v",
						label, reducerFiring, managerFiring)
				}
				if managerFiring && step.wantStartOffset >= 0 && !incident.StartedAt.Equal(alert.StartTime) {
					t.Fatalf("%s: PARITY DIVERGENCE — reducer start = %v, manager = %v",
						label, incident.StartedAt, alert.StartTime)
				}
			}
		})
	}
}
