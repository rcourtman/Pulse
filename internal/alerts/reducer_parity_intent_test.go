package alerts

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Intent-gate parity: operator suppression (expected offline, maintenance
// windows) and explicit grace policies hold ACTIVATION while the condition
// keeps confirming, then release with the run's first active observation
// as the alert start. The manager side runs the real composition —
// evaluateIntentNoLock inside evaluateCanonicalLifecycleAlert — with its
// wall-clock and runtime-tick seams (m.now, m.intentClock) driven by the
// simulated clock, a scenario-controlled operator resolver, and policies
// loaded through LoadIntentPolicies. StartTime parity on the operator
// scenarios exercises the lifecycleFirstMatched preservation landed in
// earlier slices; on the grace scenario it exercises intentPending's
// FirstMatchedAt restoration. The backup-offline deferral sub-policy is
// deferred.

type intentParityStep struct {
	matched bool
	advance time.Duration

	operatorSuppressed bool
	operatorReason     string
	maintenance        bool // active maintenance window at this observation

	wantFiring      bool
	wantStartOffset time.Duration // -1 skips
}

type intentParityScenario struct {
	name         string
	graceSeconds int // loaded as an explicit state.offline default when > 0
	steps        []intentParityStep
}

const intentParityResourceID = "parity-intent-1"

func intentParityScenarios() []intentParityScenario {
	skip := time.Duration(-1)
	return []intentParityScenario{
		{
			name: "expected-offline suppression releases with original start",
			steps: []intentParityStep{
				{matched: true, operatorSuppressed: true, operatorReason: "operator_expected_offline", wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: time.Minute, operatorSuppressed: true, operatorReason: "operator_expected_offline", wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: time.Minute, wantFiring: true, wantStartOffset: 0},
			},
		},
		{
			name: "maintenance window suppresses until it ends",
			steps: []intentParityStep{
				{matched: true, maintenance: true, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: time.Minute, maintenance: true, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: time.Minute, wantFiring: true, wantStartOffset: 0},
			},
		},
		{
			name:         "explicit grace holds activation until elapsed",
			graceSeconds: 120,
			steps: []intentParityStep{
				{matched: true, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: true, advance: time.Minute, wantFiring: true, wantStartOffset: 0},
			},
		},
		{
			name: "maintenance starting mid-firing does not clear the alert",
			steps: []intentParityStep{
				{matched: true, wantFiring: true, wantStartOffset: 0},
				{matched: true, advance: time.Minute, maintenance: true, wantFiring: true, wantStartOffset: 0},
			},
		},
	}
}

func TestReducerParityWithManagerIntentGate(t *testing.T) {
	for _, scenario := range intentParityScenarios() {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			epoch := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
			simClock := epoch

			manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
			t.Cleanup(manager.Stop)

			var currentStep intentParityStep
			manager.mu.Lock()
			manager.config.Enabled = true
			manager.config.FlappingEnabled = false
			manager.now = func() time.Time { return simClock }
			manager.intentClock = func() time.Duration { return simClock.Sub(epoch) }
			manager.mu.Unlock()
			manager.SetOperatorIntentContextResolver(func(resourceID string, now time.Time) (OperatorIntentContext, bool) {
				if currentStep.maintenance {
					start := epoch.Add(-time.Hour)
					end := simClock.Add(30 * time.Second)
					return OperatorIntentContext{MaintenanceStartAt: &start, MaintenanceEndAt: &end}, true
				}
				if currentStep.operatorSuppressed {
					return OperatorIntentContext{MonitoringMode: "expected_offline"}, true
				}
				return OperatorIntentContext{}, true
			})
			if scenario.graceSeconds > 0 {
				grace := scenario.graceSeconds
				document := NewAlertIntentPolicyDocument()
				document.Defaults[string(AlertIntentSignalOffline)] = AlertIntentRule{GraceSeconds: &grace}
				if err := manager.LoadIntentPolicies(document); err != nil {
					t.Fatalf("LoadIntentPolicies: %v", err)
				}
			}

			reducerState := reducer.NewState()

			alertID := canonicalConnectivityStateID(intentParityResourceID)
			for i, step := range scenario.steps {
				simClock = simClock.Add(step.advance)
				currentStep = step

				spec, err := buildCanonicalConnectivitySpec(
					intentParityResourceID, "parity-intent", unifiedresources.ResourceTypeAgent,
					AlertLevelCritical, 1, false,
				)
				if err != nil {
					t.Fatalf("build spec: %v", err)
				}
				if _, ok := manager.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
					Spec: spec,
					Evidence: alertspecs.AlertEvidence{
						ObservedAt:   simClock,
						Connectivity: &alertspecs.ConnectivityEvidence{Signal: "status", Connected: !step.matched},
					},
					AlertID:      alertID,
					AlertType:    "offline",
					ResourceID:   intentParityResourceID,
					ResourceName: "parity-intent",
					Message:      "parity intent condition",
				}); !ok {
					t.Fatal("evaluateCanonicalLifecycleAlert rejected the parity spec")
				}

				intent := &reducer.DiscreteIntent{
					Explicit:           scenario.graceSeconds > 0,
					GraceSeconds:       scenario.graceSeconds,
					OperatorSuppressed: step.operatorSuppressed || step.maintenance,
					OperatorReason:     step.operatorReason,
				}
				reducerState.ApplyDiscrete(reducer.DiscreteSignal{
					ResourceID: intentParityResourceID,
					Key:        "connectivity",
					Matched:    step.matched,
					Severity:   reducer.SeverityCritical,
					ObservedAt: simClock,
				}, reducer.DiscreteRule{Confirmations: 1, Intent: intent})

				manager.mu.Lock()
				alert, exists := manager.getActiveAlertNoLock(alertID)
				manager.mu.Unlock()
				managerFiring := exists && alert != nil
				incident, ok := reducerState.Incident(intentParityResourceID, "connectivity")
				reducerFiring := ok && incident.State == reducer.StateFiring

				label := fmt.Sprintf("step %d (matched=%v suppressed=%v maint=%v advance=%s)",
					i, step.matched, step.operatorSuppressed, step.maintenance, step.advance)

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
