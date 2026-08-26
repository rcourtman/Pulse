package alerts

import (
	"testing"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestUnifiedProviderIncidentConfirmationRecoveryAndStableIdentity(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())

	observedAt := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	resource := confirmedProviderIncidentResource(observedAt)

	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	if alerts := m.GetActiveAlerts(); len(alerts) != 0 {
		t.Fatalf("first transient observation must stay pending: %+v", alerts)
	}
	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	alerts := m.GetActiveAlerts()
	if len(alerts) != 1 {
		t.Fatalf("second observation must activate one alert: %+v", alerts)
	}
	alertID := alerts[0].ID
	canonicalState := alerts[0].CanonicalState
	if !alerts[0].StartTime.Equal(observedAt) {
		t.Fatalf("start time = %s, want first observation %s", alerts[0].StartTime, observedAt)
	}

	escalated := resource
	escalated.Incidents = append([]unifiedresources.ResourceIncident(nil), resource.Incidents...)
	escalated.Incidents[0].Severity = storagehealth.RiskCritical
	escalated.Incidents[0].Summary = "TrueNAS app media is crashed"
	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{escalated})
	alerts = m.GetActiveAlerts()
	if len(alerts) != 1 || alerts[0].ID != alertID || alerts[0].CanonicalState != canonicalState || alerts[0].Level != AlertLevelCritical {
		t.Fatalf("severity escalation must update one occurrence: %+v", alerts)
	}
	if history := m.GetAlertHistory(100); len(history) != 1 {
		t.Fatalf("escalation must not create duplicate history: %+v", history)
	}

	m.SyncUnifiedResourceIncidents(nil)
	if alerts := m.GetActiveAlerts(); len(alerts) != 1 {
		t.Fatalf("missing provider telemetry is unknown, not recovery: %+v", alerts)
	}

	recovered := escalated
	recovered.Incidents = nil
	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{recovered})
	if alerts := m.GetActiveAlerts(); len(alerts) != 1 {
		t.Fatalf("first healthy observation must not resolve: %+v", alerts)
	}
	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{recovered})
	if alerts := m.GetActiveAlerts(); len(alerts) != 0 {
		t.Fatalf("second healthy observation must resolve: %+v", alerts)
	}
	if resolved := m.GetRecentlyResolved(); len(resolved) != 1 {
		t.Fatalf("resolved lifecycle entry = %+v", resolved)
	}
}

func TestUnifiedProviderIncidentRecoveryConfirmationSurvivesRestart(t *testing.T) {
	dataDir := t.TempDir()
	cfg := unifiedEvalBaseConfig()
	// Restore drops alerts older than 24h, so the incident must start recently.
	observedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	resource := confirmedProviderIncidentResource(observedAt)

	first := NewManagerWithDataDir(dataDir)
	configureUnifiedEvalManager(t, first, cfg)
	first.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	first.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	if len(first.GetActiveAlerts()) != 1 {
		first.Stop()
		t.Fatal("expected active incident before restart")
	}
	first.Stop()

	second := NewManagerWithDataDir(dataDir)
	t.Cleanup(second.Stop)
	if len(second.GetActiveAlerts()) != 1 {
		t.Fatal("persisted incident was not restored")
	}
	recovered := resource
	recovered.Incidents = nil
	second.SyncUnifiedResourceIncidents([]unifiedresources.Resource{recovered})
	if len(second.GetActiveAlerts()) != 1 {
		t.Fatal("restored incident resolved on first healthy poll")
	}
	second.SyncUnifiedResourceIncidents([]unifiedresources.Resource{recovered})
	if len(second.GetActiveAlerts()) != 0 {
		t.Fatal("restored incident did not resolve after confirmed recovery")
	}
}

func TestUnifiedProviderIncidentPendingActivationRestartsSafely(t *testing.T) {
	dataDir := t.TempDir()
	cfg := unifiedEvalBaseConfig()
	resource := confirmedProviderIncidentResource(time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC))

	first := NewManagerWithDataDir(dataDir)
	configureUnifiedEvalManager(t, first, cfg)
	first.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	if len(first.GetActiveAlerts()) != 0 {
		first.Stop()
		t.Fatal("one observation activated an incident before restart")
	}
	first.Stop()

	second := NewManagerWithDataDir(dataDir)
	t.Cleanup(second.Stop)
	second.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	if len(second.GetActiveAlerts()) != 0 {
		t.Fatal("pending state from before restart caused a false activation")
	}
	second.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	if len(second.GetActiveAlerts()) != 1 {
		t.Fatal("incident did not activate after two post-restart observations")
	}
}

func TestUnifiedProviderIncidentGlobalDisableClearsImmediately(t *testing.T) {
	m := newTestManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())
	resource := confirmedProviderIncidentResource(time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC))

	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	if len(m.GetActiveAlerts()) != 1 {
		t.Fatal("expected active incident before global disable")
	}

	m.mu.Lock()
	m.config.Enabled = false
	m.mu.Unlock()
	m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	if len(m.GetActiveAlerts()) != 0 {
		t.Fatal("global alert disable did not clear the provider incident")
	}
}

// Regression: confirmation maps hold only counts, so the lifecycle owner must
// preserve the first matched observation separately and use it as StartTime
// when the final confirmation activates the alert.
func TestLifecycleAlertStartTimeIsFirstMatchedObservation(t *testing.T) {
	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)

	manager.mu.Lock()
	manager.config.Enabled = true
	manager.mu.Unlock()

	epoch := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	observe := func(observed string, at time.Time) {
		spec, err := buildCanonicalDiscreteStateSpec(
			"node-9", "node-9", unifiedresources.ResourceTypeAgent,
			AlertLevelCritical, 3, false, "connectivity", []string{"offline"},
		)
		if err != nil {
			t.Fatalf("build spec: %v", err)
		}
		if _, ok := manager.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
			Spec: spec,
			Evidence: alertspecs.AlertEvidence{
				ObservedAt:    at,
				DiscreteState: &alertspecs.DiscreteStateEvidence{StateKey: "connectivity", Observed: observed},
			},
			AlertID:      canonicalDiscreteStateStateID("node-9", "connectivity"),
			AlertType:    "connectivity",
			ResourceID:   "node-9",
			ResourceName: "node-9",
			Message:      "node offline",
		}); !ok {
			t.Fatal("evaluation rejected")
		}
	}

	observe("offline", epoch)
	observe("offline", epoch.Add(30*time.Second))
	observe("offline", epoch.Add(60*time.Second))

	manager.mu.Lock()
	alert, exists := manager.getActiveAlertNoLock(canonicalDiscreteStateStateID("node-9", "connectivity"))
	manager.mu.Unlock()
	if !exists || alert == nil {
		t.Fatal("expected alert to fire after three confirmations")
	}
	if !alert.StartTime.Equal(epoch) {
		t.Fatalf("StartTime = %v, want first offline observation %v", alert.StartTime, epoch)
	}

	observe("online", epoch.Add(90*time.Second))
	manager.mu.Lock()
	_, hasIncident := manager.core.Incident("node-9", "node-9-connectivity")
	manager.mu.Unlock()
	if hasIncident {
		t.Fatal("core incident should clear with the confirmation run")
	}
}

func confirmedProviderIncidentResource(observedAt time.Time) unifiedresources.Resource {
	return unifiedresources.Resource{
		ID:         "app:media",
		Type:       unifiedresources.ResourceTypeAppContainer,
		Name:       "media",
		ParentName: "nas-a",
		Sources:    []unifiedresources.DataSource{unifiedresources.SourceTrueNAS},
		TrueNAS:    &unifiedresources.TrueNASData{Hostname: "nas-a"},
		Incidents: []unifiedresources.ResourceIncident{{
			Provider:                      "truenas",
			NativeID:                      "app:media",
			Code:                          "truenas_app_stopped",
			Severity:                      storagehealth.RiskWarning,
			Source:                        "app.query",
			Summary:                       "TrueNAS app media is stopped",
			StartedAt:                     observedAt,
			ConfirmationsRequired:         2,
			RecoveryConfirmationsRequired: 2,
		}},
	}
}

// The reducer core owns confirmation runs: resetting the legacy count
// maps directly (the pre-cutover healthy-poll behavior) no longer
// disturbs a run in progress — only a genuine non-matching observation
// resets it. This replaces the pre-cutover restamp regression test, whose
// premise (reconstructing runs from the count maps) no longer exists.
func TestLifecycleRunImmuneToLegacyCountResets(t *testing.T) {
	manager := NewManagerWithDataDir(t.TempDir(), WithoutPersistedAlertRestore())
	t.Cleanup(manager.Stop)

	manager.mu.Lock()
	manager.config.Enabled = true
	manager.mu.Unlock()

	epoch := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	observe := func(observed string, at time.Time) {
		spec, err := buildCanonicalDiscreteStateSpec(
			"node-9", "node-9", unifiedresources.ResourceTypeAgent,
			AlertLevelCritical, 3, false, "connectivity", []string{"offline"},
		)
		if err != nil {
			t.Fatalf("build spec: %v", err)
		}
		if _, ok := manager.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
			Spec: spec,
			Evidence: alertspecs.AlertEvidence{
				ObservedAt:    at,
				DiscreteState: &alertspecs.DiscreteStateEvidence{StateKey: "connectivity", Observed: observed},
			},
			AlertID:      canonicalDiscreteStateStateID("node-9", "connectivity"),
			AlertType:    "connectivity",
			ResourceID:   "node-9",
			ResourceName: "node-9",
			Message:      "node offline",
		}); !ok {
			t.Fatal("evaluation rejected")
		}
	}

	// Two matches begin a run; a direct legacy count reset is a no-op for
	// the core-owned run, so the third consecutive match fires with the
	// run's true first observation as the start.
	observe("offline", epoch)
	observe("offline", epoch.Add(30*time.Second))
	observe("offline", epoch.Add(time.Minute))

	manager.mu.Lock()
	alert, exists := manager.getActiveAlertNoLock(canonicalDiscreteStateStateID("node-9", "connectivity"))
	manager.mu.Unlock()
	if !exists || alert == nil {
		t.Fatal("expected alert to fire on the third consecutive match")
	}
	if !alert.StartTime.Equal(epoch) {
		t.Fatalf("StartTime = %v, want the run's first observation %v", alert.StartTime, epoch)
	}

	// A genuine non-matching observation does reset the run.
	observe("online", epoch.Add(90*time.Second))
	restart := epoch.Add(2 * time.Minute)
	observe("offline", restart)
	observe("offline", restart.Add(30*time.Second))
	manager.mu.Lock()
	_, midRun := manager.getActiveAlertNoLock(canonicalDiscreteStateStateID("node-9", "connectivity"))
	manager.mu.Unlock()
	if midRun {
		t.Fatal("run must restart after a genuine recovery")
	}
}
