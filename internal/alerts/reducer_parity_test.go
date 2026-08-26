package alerts

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
)

// The reducer parity harness drives the live manager's metric path and the
// Phase 1 reducer (internal/alerts/reducer) through identical observation
// sequences and diffs the outcome after every step. The reducer must
// CHARACTERIZE Manager.checkMetric, never improve on it: any divergence is
// a reducer bug by definition (docs/ALERT_ENGINE_EVOLUTION.md, Phase 1).
//
// Time is simulated. The reducer takes it via ObservedAt; the manager's
// delay tracking uses the wall clock, so the harness backdates
// m.pendingAlerts between steps — the same trick the existing
// time-threshold tests use.

type parityStep struct {
	value float64
	// advance is the simulated time between the previous observation and
	// this one.
	advance time.Duration
	// disabled observes with a disabled rule (nil threshold / zero trigger).
	disabled bool

	wantFiring   bool
	wantSeverity AlertLevel // checked only when wantFiring
}

type parityScenario struct {
	name         string
	trigger      float64
	clear        float64
	delaySeconds int
	steps        []parityStep
}

const (
	parityResourceID   = "parity-vm-100"
	parityMetric       = "cpu"
	parityResourceType = "VM"
)

func parityCanonicalStateID() string {
	return buildCanonicalStateID(parityResourceID, "metric-threshold:"+parityMetric)
}

type managerParityEngine struct {
	manager   *Manager
	threshold *HysteresisThreshold
}

func newManagerParityEngine(t *testing.T, scenario parityScenario) *managerParityEngine {
	t.Helper()
	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)

	manager.mu.Lock()
	manager.config.Enabled = true
	manager.config.MinimumDelta = 0
	manager.config.SuppressionWindow = 0
	manager.config.FlappingEnabled = false
	manager.config.TimeThresholds = map[string]int{}
	if scenario.delaySeconds > 0 {
		manager.config.TimeThresholds["guest"] = scenario.delaySeconds
	}
	manager.mu.Unlock()

	return &managerParityEngine{
		manager:   manager,
		threshold: &HysteresisThreshold{Trigger: scenario.trigger, Clear: scenario.clear},
	}
}

func (e *managerParityEngine) observe(step parityStep) {
	if step.advance > 0 {
		// Simulate elapsed time for the core's sustained-for delay.
		e.manager.mu.Lock()
		e.manager.core.ShiftPending(-step.advance)
		e.manager.mu.Unlock()
	}

	threshold := e.threshold
	if step.disabled {
		threshold = nil
	}
	e.manager.checkMetric(
		parityResourceID, "parity-vm", "node1", "qemu/100",
		parityResourceType, parityMetric, step.value, threshold, nil,
	)
}

func (e *managerParityEngine) snapshot() (bool, AlertLevel) {
	e.manager.mu.Lock()
	defer e.manager.mu.Unlock()
	alert, exists := e.manager.getActiveAlertNoLock(parityCanonicalStateID())
	if !exists || alert == nil {
		return false, ""
	}
	return true, alert.Level
}

type reducerParityEngine struct {
	state *reducer.State
	rule  reducer.MetricRule
	clock time.Time
}

func newReducerParityEngine(scenario parityScenario) *reducerParityEngine {
	return &reducerParityEngine{
		state: reducer.NewState(),
		rule: reducer.MetricRule{
			Trigger:      scenario.trigger,
			Clear:        scenario.clear,
			DelaySeconds: scenario.delaySeconds,
		},
		clock: time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC),
	}
}

func (e *reducerParityEngine) observe(step parityStep) {
	e.clock = e.clock.Add(step.advance)
	rule := e.rule
	if step.disabled {
		rule = reducer.MetricRule{}
	}
	e.state.ApplyMetric(reducer.MetricSignal{
		ResourceID: parityResourceID,
		Metric:     parityMetric,
		Value:      step.value,
		ObservedAt: e.clock,
	}, rule)
}

func (e *reducerParityEngine) snapshot() (bool, AlertLevel) {
	incident, ok := e.state.Incident(parityResourceID, parityMetric)
	if !ok || incident.State != reducer.StateFiring {
		return false, ""
	}
	return true, AlertLevel(incident.Severity)
}

func parityScenarios() []parityScenario {
	return []parityScenario{
		{
			name:    "fire, hold in hysteresis band, resolve at clear",
			trigger: 80, clear: 75,
			steps: []parityStep{
				{value: 85, wantFiring: true, wantSeverity: AlertLevelWarning},
				{value: 78, advance: time.Minute, wantFiring: true, wantSeverity: AlertLevelWarning},
				{value: 74, advance: time.Minute, wantFiring: false},
			},
		},
		{
			name:    "critical on fire, demote while firing, resolve",
			trigger: 80, clear: 75,
			steps: []parityStep{
				{value: 95, wantFiring: true, wantSeverity: AlertLevelCritical},
				{value: 85, advance: time.Minute, wantFiring: true, wantSeverity: AlertLevelWarning},
				{value: 60, advance: time.Minute, wantFiring: false},
			},
		},
		{
			name:    "percentage critical cap keeps escalation reachable at high trigger",
			trigger: 95, clear: 90,
			steps: []parityStep{
				{value: 99, wantFiring: true, wantSeverity: AlertLevelCritical},
			},
		},
		{
			name:    "sustained-for delay with dip reset",
			trigger: 80, clear: 75, delaySeconds: 120,
			steps: []parityStep{
				{value: 90, wantFiring: false},
				{value: 91, advance: 60 * time.Second, wantFiring: false},
				{value: 70, advance: 30 * time.Second, wantFiring: false},
				{value: 92, advance: 10 * time.Second, wantFiring: false},
				{value: 92, advance: 119 * time.Second, wantFiring: false},
				{value: 92, advance: 2 * time.Second, wantFiring: true, wantSeverity: AlertLevelCritical},
			},
		},
		{
			name:    "clear falls back to trigger when unset",
			trigger: 80, clear: 0,
			steps: []parityStep{
				{value: 85, wantFiring: true, wantSeverity: AlertLevelWarning},
				{value: 80, advance: time.Minute, wantFiring: true, wantSeverity: AlertLevelWarning},
				{value: 79, advance: time.Minute, wantFiring: false},
			},
		},
		{
			name:    "disabling the rule resolves a firing incident",
			trigger: 80, clear: 75,
			steps: []parityStep{
				{value: 85, wantFiring: true, wantSeverity: AlertLevelWarning},
				{value: 85, advance: time.Minute, disabled: true, wantFiring: false},
			},
		},
		{
			name:    "re-fires after resolution",
			trigger: 80, clear: 75,
			steps: []parityStep{
				{value: 85, wantFiring: true, wantSeverity: AlertLevelWarning},
				{value: 70, advance: time.Minute, wantFiring: false},
				{value: 86, advance: time.Minute, wantFiring: true, wantSeverity: AlertLevelWarning},
			},
		},
	}
}

func TestReducerParityWithManagerMetricPath(t *testing.T) {
	for _, scenario := range parityScenarios() {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			managerEngine := newManagerParityEngine(t, scenario)
			reducerEngine := newReducerParityEngine(scenario)

			for i, step := range scenario.steps {
				managerEngine.observe(step)
				reducerEngine.observe(step)

				managerFiring, managerLevel := managerEngine.snapshot()
				reducerFiring, reducerLevel := reducerEngine.snapshot()

				label := fmt.Sprintf("step %d (value=%.0f advance=%s disabled=%v)",
					i, step.value, step.advance, step.disabled)

				// The manager is the reference: first pin it to the
				// scenario's expectation, then require the reducer to match
				// the manager exactly.
				if managerFiring != step.wantFiring {
					t.Fatalf("%s: manager firing = %v, scenario expects %v (characterization drift — update the scenario, then the reducer)",
						label, managerFiring, step.wantFiring)
				}
				if managerFiring && managerLevel != step.wantSeverity {
					t.Fatalf("%s: manager severity = %q, scenario expects %q",
						label, managerLevel, step.wantSeverity)
				}

				if reducerFiring != managerFiring {
					t.Fatalf("%s: PARITY DIVERGENCE — reducer firing = %v, manager = %v",
						label, reducerFiring, managerFiring)
				}
				if managerFiring && reducerLevel != managerLevel {
					t.Fatalf("%s: PARITY DIVERGENCE — reducer severity = %q, manager = %q",
						label, reducerLevel, managerLevel)
				}
			}
		})
	}
}
