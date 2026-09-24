package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Exercise the detector entrypoints, not just the active map: rejecting an
// alert must also reject its history, firing event and notification intent.
func TestOperatorSuppressionPreventsFiringSideEffects(t *testing.T) {
	const resourceID = "storage:tank"
	evaluators := map[string]func(*testing.T, *Manager){
		"provider-incident": func(t *testing.T, m *Manager) {
			m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{admissionTestResource()})
		},
		"metric": func(t *testing.T, m *Manager) {
			m.checkMetric(resourceID, "tank", "nas", "nas", "storage", "cpu", 95,
				&HysteresisThreshold{Trigger: 80, Clear: 70}, nil)
		},
		"canonical-metric": func(t *testing.T, m *Manager) {
			spec := alertspecs.ResourceAlertSpec{
				ID: "metric-threshold:cpu", ResourceID: resourceID,
				ResourceType: unifiedresources.ResourceTypeStorage,
				Kind:         alertspecs.AlertSpecKindMetricThreshold, Severity: alertspecs.AlertSeverityWarning,
				MetricThreshold: &alertspecs.MetricThresholdSpec{Metric: "cpu", Trigger: 80, Direction: alertspecs.ThresholdDirectionAbove},
			}
			m.evaluateCanonicalMetricAlert(spec, "tank", "nas", "nas", "storage", 95,
				&HysteresisThreshold{Trigger: 80, Clear: 70}, nil)
		},
		"lifecycle": func(t *testing.T, m *Manager) {
			spec, err := buildCanonicalDiscreteStateSpec(resourceID, "Pool state", unifiedresources.ResourceTypeStorage,
				AlertLevelWarning, 1, false, "health", []string{"degraded"})
			if err != nil {
				t.Fatal(err)
			}
			m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
				Spec: spec, Evidence: alertspecs.AlertEvidence{ObservedAt: time.Now(),
					DiscreteState: &alertspecs.DiscreteStateEvidence{StateKey: "health", Observed: "degraded"}},
				AlertID: spec.ID, AlertType: "zfs-pool-state", ResourceID: resourceID,
				ResourceName: "tank", AddToRecent: true, AddToHistory: true,
			})
		},
		"stateful": func(t *testing.T, m *Manager) {
			m.syncCanonicalHealthAssessmentAlert(canonicalHealthAssessmentAlertParams{
				SpecID: "pool-health", Signal: "zfs_pool", Codes: zfsPoolAssessmentCodes,
				Reasons: []storagehealth.Reason{{Code: "zfs_pool_state", Severity: storagehealth.RiskCritical, Summary: "Pool is degraded"}},
				AlertID: "pool-health", AlertType: "zfs-pool-state", SpecResourceID: resourceID,
				ResourceID: resourceID, ResourceName: "tank", ResourceType: unifiedresources.ResourceTypeStorage,
			})
		},
	}
	start, end := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	for name, policy := range map[string]OperatorIntentContext{
		"muted":       {MonitoringMode: "muted"},
		"retired":     {LifecycleState: "retired"},
		"maintenance": {MaintenanceStartAt: &start, MaintenanceEndAt: &end},
	} {
		for family, evaluate := range evaluators {
			t.Run(name+"/"+family, func(t *testing.T) {
				m := newEventLogManager(t)
				configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())
				m.SetAlertCallback(func(*Alert) {})
				setPolicy := func(current OperatorIntentContext) {
					m.SetOperatorIntentContextResolver(func(string, time.Time) (OperatorIntentContext, bool) {
						return current, true
					})
				}
				setPolicy(policy)
				for range 20 {
					evaluate(t, m)
				}
				if got := len(m.GetActiveAlerts()); got != 0 {
					t.Fatalf("suppressed active alerts = %d", got)
				}
				if got := len(m.historyManager.GetAllHistory(100)); got != 0 {
					t.Fatalf("suppressed history entries = %d", got)
				}
				if events := queryAlertEvents(t, m, eventlog.Filter{}); len(events) != 0 {
					t.Fatalf("suppressed detector produced lifecycle/delivery events: %+v", events)
				}
				m.mu.RLock()
				recent := len(m.recentAlerts)
				m.mu.RUnlock()
				if recent != 0 {
					t.Fatalf("suppressed recent alerts = %d", recent)
				}

				// Expiry or removal must admit the still-present condition once.
				if name == "maintenance" {
					expired := time.Now().Add(-time.Minute)
					setPolicy(OperatorIntentContext{MaintenanceStartAt: &start, MaintenanceEndAt: &expired})
				} else {
					setPolicy(OperatorIntentContext{})
				}
				for range 20 {
					evaluate(t, m)
				}
				if got := len(m.GetActiveAlerts()); got != 1 {
					t.Fatalf("active alerts after unsuppression = %d, want 1", got)
				}
				events := queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeFired, eventlog.TypeRefired}})
				if len(events) != 1 {
					t.Fatalf("firing events after unsuppression = %d, want 1", len(events))
				}

				setPolicy(OperatorIntentContext{MonitoringMode: "muted"})
				if got := m.ReconcileResourceOperatorState(resourceID); got != 1 {
					t.Fatalf("newly muted active alerts cleared = %d, want 1", got)
				}
				before := len(queryAlertEvents(t, m, eventlog.Filter{}))
				for range 20 {
					evaluate(t, m)
				}
				if got := len(m.GetActiveAlerts()); got != 0 {
					t.Fatalf("muted alert reappeared after reconciliation: %d", got)
				}
				if got := len(queryAlertEvents(t, m, eventlog.Filter{})); got != before {
					t.Fatalf("suppressed re-evaluation added events after reconciliation: %d -> %d", before, got)
				}
			})
		}
	}
}

func TestSuppressedAlertRestoreCannotReviveAcknowledgementOrEscalation(t *testing.T) {
	m := newEventLogManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())
	m.SetAlertCallback(func(*Alert) {})
	resource := admissionTestResource()
	for range 3 {
		m.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	}
	active := m.GetActiveAlerts()
	if len(active) != 1 {
		t.Fatalf("unsuppressed control alerts = %d, want 1", len(active))
	}
	if err := m.AcknowledgeAlert(active[0].ID, "operator"); err != nil {
		t.Fatal(err)
	}
	snapshot := m.GetActiveAlerts()

	restored := newEventLogManager(t)
	configureUnifiedEvalManager(t, restored, unifiedEvalBaseConfig())
	restored.SetAlertCallback(func(*Alert) {})
	restored.SetOperatorIntentContextResolver(func(string, time.Time) (OperatorIntentContext, bool) {
		return OperatorIntentContext{LifecycleState: "retired"}, true
	})
	if err := restored.restoreActiveAlertSnapshots([]*Alert{&snapshot[0]}, "test restart", true); err != nil {
		t.Fatal(err)
	}
	resource.Incidents[0].Severity = storagehealth.RiskCritical
	for range 20 {
		restored.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	}
	if len(restored.GetActiveAlerts()) != 0 || len(queryAlertEvents(t, restored, eventlog.Filter{})) != 0 {
		t.Fatal("suppressed restore or escalation produced an active alert or event")
	}
	restored.SetOperatorIntentContextResolver(nil)
	for range 20 {
		restored.SyncUnifiedResourceIncidents([]unifiedresources.Resource{resource})
	}
	active = restored.GetActiveAlerts()
	if len(active) != 1 || active[0].Acknowledged || active[0].Level != AlertLevelCritical {
		t.Fatalf("unsuppressed condition did not create a fresh critical alert: %+v", active)
	}
	if events := queryAlertEvents(t, restored, eventlog.Filter{Types: []string{eventlog.TypeFired}}); len(events) != 1 {
		t.Fatalf("firing events after restore and unsuppression = %d, want 1", len(events))
	}
}

func admissionTestResource() unifiedresources.Resource {
	return unifiedresources.Resource{
		ID: "storage:tank", Type: unifiedresources.ResourceTypeStorage, Name: "tank",
		Sources: []unifiedresources.DataSource{unifiedresources.SourceTrueNAS},
		Storage: &unifiedresources.StorageMeta{Platform: "truenas", Topology: "pool", Protection: "zfs", IsZFS: true},
		Incidents: []unifiedresources.ResourceIncident{{Provider: "truenas", NativeID: "native-1",
			Code: "truenas_volume_status", Severity: storagehealth.RiskWarning, Summary: "Pool is degraded"}},
	}
}
