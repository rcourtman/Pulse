package alerts

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Discrete-family parity: the reducer's confirmation semantics
// (internal/alerts/reducer/discrete.go) are diffed against the live
// manager's canonical lifecycle path — buildCanonicalDiscreteStateSpec +
// evaluateCanonicalLifecycleAlert, the same shape external_probe.go uses —
// after every step. Time is fully simulated on both sides here: the
// lifecycle path takes ObservedAt from the evidence.
//
// Alert StartTime parity is asserted on first activations only. On a
// re-fire within the recently-resolved retention window the manager
// restores the ORIGINAL start time (consumeRecentlyResolvedForRefire);
// that behavior needs resolved-history state in the reducer and is left
// for a later slice.

type discreteParityStep struct {
	matched  bool
	severity AlertLevel
	disabled bool
	advance  time.Duration

	wantFiring   bool
	wantSeverity AlertLevel
	// wantStartOffset, when >= 0, asserts the firing alert's start time as
	// an offset from the scenario's first observation. Use -1 to skip.
	wantStartOffset time.Duration
}

type discreteParityScenario struct {
	name          string
	confirmations int
	steps         []discreteParityStep
}

const (
	discreteParityResourceID = "parity-node-1"
	discreteParityStateKey   = "parity-state"
)

type managerDiscreteParityEngine struct {
	manager *Manager
	clock   time.Time
	conf    int
}

func newManagerDiscreteParityEngine(t *testing.T, scenario discreteParityScenario, epoch time.Time) *managerDiscreteParityEngine {
	t.Helper()
	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)

	manager.mu.Lock()
	manager.config.Enabled = true
	manager.config.FlappingEnabled = false
	manager.mu.Unlock()

	return &managerDiscreteParityEngine{manager: manager, clock: epoch, conf: scenario.confirmations}
}

func (e *managerDiscreteParityEngine) observe(t *testing.T, step discreteParityStep) {
	t.Helper()
	e.clock = e.clock.Add(step.advance)

	severity := step.severity
	if severity == "" {
		severity = AlertLevelWarning
	}
	spec, err := buildCanonicalDiscreteStateSpec(
		discreteParityResourceID,
		"parity-node",
		unifiedresources.ResourceTypeAgent,
		severity,
		e.conf,
		step.disabled,
		discreteParityStateKey,
		[]string{"bad"},
	)
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}

	observed := "ok"
	if step.matched {
		observed = "bad"
	}
	_, ok := e.manager.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: e.clock,
			DiscreteState: &alertspecs.DiscreteStateEvidence{
				StateKey: discreteParityStateKey,
				Observed: observed,
			},
		},
		AlertID:       canonicalDiscreteStateStateID(discreteParityResourceID, discreteParityStateKey),
		AlertType:     "parity-state",
		ResourceID:    discreteParityResourceID,
		ResourceName:  "parity-node",
		Instance:      "parity",
		Message:       "parity condition observed",
		AddToRecent:   true,
		AddToHistory:  true,
		RateLimit:     false,
		DispatchAsync: false,
	})
	if !ok {
		t.Fatalf("evaluateCanonicalLifecycleAlert rejected the parity spec")
	}
}

func (e *managerDiscreteParityEngine) snapshot() (bool, AlertLevel, time.Time) {
	e.manager.mu.Lock()
	defer e.manager.mu.Unlock()
	alert, exists := e.manager.getActiveAlertNoLock(
		canonicalDiscreteStateStateID(discreteParityResourceID, discreteParityStateKey))
	if !exists || alert == nil {
		return false, "", time.Time{}
	}
	return true, alert.Level, alert.StartTime
}

type reducerDiscreteParityEngine struct {
	state *reducer.State
	clock time.Time
	conf  int
}

func (e *reducerDiscreteParityEngine) observe(step discreteParityStep) {
	e.clock = e.clock.Add(step.advance)
	severity := reducer.SeverityWarning
	if step.severity == AlertLevelCritical {
		severity = reducer.SeverityCritical
	}
	e.state.ApplyDiscrete(reducer.DiscreteSignal{
		ResourceID: discreteParityResourceID,
		Key:        discreteParityStateKey,
		Matched:    step.matched,
		Severity:   severity,
		ObservedAt: e.clock,
	}, reducer.DiscreteRule{Confirmations: e.conf, Disabled: step.disabled})
}

func (e *reducerDiscreteParityEngine) snapshot() (bool, AlertLevel, time.Time) {
	incident, ok := e.state.Incident(discreteParityResourceID, discreteParityStateKey)
	if !ok || incident.State != reducer.StateFiring {
		return false, "", time.Time{}
	}
	return true, AlertLevel(incident.Severity), incident.StartedAt
}

func discreteParityScenarios() []discreteParityScenario {
	skip := time.Duration(-1)
	return []discreteParityScenario{
		{
			name:          "three-confirmation activation, single recovery",
			confirmations: 3,
			steps: []discreteParityStep{
				{matched: true, severity: AlertLevelCritical, wantFiring: false, wantStartOffset: skip},
				{matched: true, severity: AlertLevelCritical, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: true, severity: AlertLevelCritical, advance: time.Minute, wantFiring: true, wantSeverity: AlertLevelCritical, wantStartOffset: 0},
				{matched: false, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
			},
		},
		{
			name:          "interrupted run resets the confirmation count",
			confirmations: 3,
			steps: []discreteParityStep{
				{matched: true, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: false, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: time.Minute, wantFiring: true, wantSeverity: AlertLevelWarning, wantStartOffset: 3 * time.Minute},
			},
		},
		{
			name:          "single confirmation fires immediately and re-fires after recovery",
			confirmations: 1,
			steps: []discreteParityStep{
				{matched: true, wantFiring: true, wantSeverity: AlertLevelWarning, wantStartOffset: 0},
				{matched: false, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: time.Minute, wantFiring: true, wantSeverity: AlertLevelWarning, wantStartOffset: skip},
			},
		},
		{
			name:          "severity follows the spec while firing",
			confirmations: 1,
			steps: []discreteParityStep{
				{matched: true, severity: AlertLevelWarning, wantFiring: true, wantSeverity: AlertLevelWarning, wantStartOffset: 0},
				{matched: true, severity: AlertLevelCritical, advance: time.Minute, wantFiring: true, wantSeverity: AlertLevelCritical, wantStartOffset: 0},
				{matched: true, severity: AlertLevelWarning, advance: time.Minute, wantFiring: true, wantSeverity: AlertLevelWarning, wantStartOffset: 0},
			},
		},
		{
			name:          "disabling the spec resolves a firing incident",
			confirmations: 1,
			steps: []discreteParityStep{
				{matched: true, wantFiring: true, wantSeverity: AlertLevelWarning, wantStartOffset: 0},
				{matched: true, disabled: true, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
			},
		},
	}
}

func TestReducerParityWithManagerDiscretePath(t *testing.T) {
	epoch := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

	for _, scenario := range discreteParityScenarios() {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			managerEngine := newManagerDiscreteParityEngine(t, scenario, epoch)
			reducerEngine := &reducerDiscreteParityEngine{
				state: reducer.NewState(),
				clock: epoch,
				conf:  scenario.confirmations,
			}

			for i, step := range scenario.steps {
				managerEngine.observe(t, step)
				reducerEngine.observe(step)

				managerFiring, managerLevel, managerStart := managerEngine.snapshot()
				reducerFiring, reducerLevel, reducerStart := reducerEngine.snapshot()

				label := fmt.Sprintf("step %d (matched=%v severity=%s disabled=%v advance=%s)",
					i, step.matched, step.severity, step.disabled, step.advance)

				if managerFiring != step.wantFiring {
					t.Fatalf("%s: manager firing = %v, scenario expects %v (characterization drift)",
						label, managerFiring, step.wantFiring)
				}
				if managerFiring && managerLevel != step.wantSeverity {
					t.Fatalf("%s: manager severity = %q, scenario expects %q", label, managerLevel, step.wantSeverity)
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
				if managerFiring && reducerLevel != managerLevel {
					t.Fatalf("%s: PARITY DIVERGENCE — reducer severity = %q, manager = %q",
						label, reducerLevel, managerLevel)
				}
				if managerFiring && step.wantStartOffset >= 0 && !reducerStart.Equal(managerStart) {
					t.Fatalf("%s: PARITY DIVERGENCE — reducer start = %v, manager = %v",
						label, reducerStart, managerStart)
				}
			}
		})
	}
}
