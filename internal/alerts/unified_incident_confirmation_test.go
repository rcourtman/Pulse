package alerts

import (
	"testing"
	"time"

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
	observedAt := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
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
