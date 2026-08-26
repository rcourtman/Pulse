package alerts

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Backup-offline deferral parity: while Pulse has fresh evidence a backup
// caused the offline state, activation is deferred, bounded by the
// max-deferral cap on total condition-active time; after the backup ends
// the grace extends to the backup's end plus the post-grace, still capped.
// The manager side runs the real composition (LoadIntentPolicies with a
// backupOffline rule, IntentBackup context per observation) with its time
// seams simulated; the reducer models the deferral independently.

type backupParityStep struct {
	matched      bool
	backupActive bool
	advance      time.Duration

	wantFiring      bool
	wantStartOffset time.Duration // -1 skips
}

type backupParityScenario struct {
	name         string
	graceSeconds int
	postGrace    int
	maxDeferral  int
	steps        []backupParityStep
}

const backupParityResourceID = "parity-backup-1"

func backupParityScenarios() []backupParityScenario {
	skip := time.Duration(-1)
	return []backupParityScenario{
		{
			name:         "backup defers activation, post-grace releases it",
			graceSeconds: 60, postGrace: 120, maxDeferral: 600,
			steps: []backupParityStep{
				{matched: true, backupActive: true, wantFiring: false, wantStartOffset: skip},
				{matched: true, backupActive: true, advance: 3 * time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: true, backupActive: false, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: true, backupActive: false, advance: time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: true, backupActive: false, advance: time.Minute, wantFiring: true, wantStartOffset: 0},
			},
		},
		{
			name:         "max deferral caps a backup that never ends",
			graceSeconds: 60, postGrace: 120, maxDeferral: 300,
			steps: []backupParityStep{
				{matched: true, backupActive: true, wantFiring: false, wantStartOffset: skip},
				{matched: true, backupActive: true, advance: 4 * time.Minute, wantFiring: false, wantStartOffset: skip},
				{matched: true, backupActive: true, advance: time.Minute, wantFiring: true, wantStartOffset: 0},
			},
		},
	}
}

func TestReducerParityWithManagerBackupDeferral(t *testing.T) {
	for _, scenario := range backupParityScenarios() {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			epoch := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
			simClock := epoch

			manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
			t.Cleanup(manager.Stop)
			manager.mu.Lock()
			manager.config.Enabled = true
			manager.config.FlappingEnabled = false
			manager.now = func() time.Time { return simClock }
			manager.intentClock = func() time.Duration { return simClock.Sub(epoch) }
			manager.mu.Unlock()

			grace := scenario.graceSeconds
			document := NewAlertIntentPolicyDocument()
			document.Defaults[string(AlertIntentSignalOffline)] = AlertIntentRule{
				GraceSeconds: &grace,
				BackupOffline: &BackupOfflineIntentPolicy{
					Enabled:            true,
					PostGraceSeconds:   scenario.postGrace,
					MaxDeferralSeconds: scenario.maxDeferral,
				},
			}
			if err := manager.LoadIntentPolicies(document); err != nil {
				t.Fatalf("LoadIntentPolicies: %v", err)
			}

			reducerState := reducer.NewState()
			alertID := canonicalConnectivityStateID(backupParityResourceID)

			for i, step := range scenario.steps {
				simClock = simClock.Add(step.advance)

				spec, err := buildCanonicalConnectivitySpec(
					backupParityResourceID, "parity-backup", unifiedresources.ResourceTypePBS,
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
					ResourceID:   backupParityResourceID,
					ResourceName: "parity-backup",
					Message:      "parity backup condition",
					IntentBackup: BackupIntentContext{Active: step.backupActive, ObservedAt: simClock},
				}); !ok {
					t.Fatal("evaluateCanonicalLifecycleAlert rejected the parity spec")
				}

				reducerState.ApplyDiscrete(reducer.DiscreteSignal{
					ResourceID: backupParityResourceID,
					Key:        "connectivity",
					Matched:    step.matched,
					Severity:   reducer.SeverityCritical,
					ObservedAt: simClock,
				}, reducer.DiscreteRule{
					Confirmations: 1,
					Intent: &reducer.DiscreteIntent{
						Explicit:                 true,
						GraceSeconds:             scenario.graceSeconds,
						BackupEnabled:            true,
						BackupActive:             step.backupActive,
						BackupPostGraceSeconds:   scenario.postGrace,
						BackupMaxDeferralSeconds: scenario.maxDeferral,
					},
				})

				manager.mu.Lock()
				alert, exists := manager.getActiveAlertNoLock(alertID)
				manager.mu.Unlock()
				managerFiring := exists && alert != nil
				incident, ok := reducerState.Incident(backupParityResourceID, "connectivity")
				reducerFiring := ok && incident.State == reducer.StateFiring

				label := fmt.Sprintf("step %d (matched=%v backup=%v advance=%s)", i, step.matched, step.backupActive, step.advance)

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
