package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestInfrastructureIncidentSynthesisGroupsSupportedDeliverySymptoms(t *testing.T) {
	manager := newTestManager(t)
	startedAt := time.Date(2026, 8, 30, 18, 0, 0, 0, time.UTC)
	observedAt := startedAt.Add(time.Minute)
	hostID := "agent:edge-1"
	endpointID := "availability:checkout"
	resources := map[string]unifiedresources.Resource{
		hostID: {
			ID: hostID, Type: unifiedresources.ResourceTypeAgent, Name: "Edge host",
			Status: unifiedresources.StatusOffline,
		},
		endpointID: {
			ID: endpointID, Type: unifiedresources.ResourceTypeNetworkEndpoint, Name: "Checkout",
			Status: unifiedresources.StatusOffline,
			Availability: &unifiedresources.AvailabilityData{
				AggregateState:   "unavailable",
				TransportOutcome: "unreachable",
			},
			Relationships: []unifiedresources.ResourceRelationship{{
				SourceID: endpointID, TargetID: hostID, Type: unifiedresources.RelChecks,
				Confidence: 1, Active: true,
			}},
		},
	}
	root := &Alert{
		ID: "host-offline", Type: "offline", Level: AlertLevelCritical,
		ResourceID: hostID, ResourceName: "Edge host", StartTime: startedAt, LastSeen: observedAt,
	}
	symptom := &Alert{
		ID: "checkout-unreachable", Type: "resource-incident", Level: AlertLevelCritical,
		ResourceID: endpointID, ResourceName: "Checkout", StartTime: startedAt.Add(30 * time.Second), LastSeen: observedAt,
		Metadata: map[string]interface{}{"incidentCode": "availability_unreachable"},
		Evidence: []operationaltrust.EvidenceEnvelope{{ID: "evidence_checkout"}},
	}
	desired := map[string]*Alert{"root": root, "symptom": symptom}

	manager.mu.Lock()
	manager.applyInfrastructureIncidentSynthesisNoLock(resources, desired)
	manager.mu.Unlock()

	if root.Correlation == nil || root.Correlation.Role != AlertCorrelationRolePrimary {
		t.Fatalf("root correlation = %+v, want primary", root.Correlation)
	}
	if root.Correlation.Inference != AlertCorrelationInferenceSupportedCause {
		t.Fatalf("inference = %q, want supported cause", root.Correlation.Inference)
	}
	if root.Correlation.FailureClass != AlertFailureClassRuntime {
		t.Fatalf("root failure class = %q, want runtime", root.Correlation.FailureClass)
	}
	if symptom.Correlation == nil || symptom.Correlation.Role != AlertCorrelationRoleSupporting {
		t.Fatalf("symptom correlation = %+v, want supporting", symptom.Correlation)
	}
	if symptom.Correlation.FailureClass != AlertFailureClassNetworkPath {
		t.Fatalf("symptom failure class = %q, want network path", symptom.Correlation.FailureClass)
	}
	if !isSupportedInfrastructureSymptom(symptom) {
		t.Fatal("supported symptom must use the primary alert's notification delivery")
	}
	if len(root.Correlation.Observations) != 2 || root.Correlation.Observations[1].EvidenceIDs[0] != "evidence_checkout" {
		t.Fatalf("observations = %+v, want both detector records and endpoint evidence", root.Correlation.Observations)
	}
}

func TestInfrastructureIncidentSynthesisPreservesContradictionAsObservationSet(t *testing.T) {
	manager := newTestManager(t)
	startedAt := time.Date(2026, 8, 30, 18, 0, 0, 0, time.UTC)
	hostID := "agent:edge-1"
	endpointID := "availability:checkout"
	resources := map[string]unifiedresources.Resource{
		hostID: {ID: hostID, Type: unifiedresources.ResourceTypeAgent, Name: "Edge host", Status: unifiedresources.StatusOnline},
		endpointID: {
			ID: endpointID, Type: unifiedresources.ResourceTypeNetworkEndpoint, Name: "Checkout",
			Status: unifiedresources.StatusOffline,
			Relationships: []unifiedresources.ResourceRelationship{{
				SourceID: endpointID, TargetID: hostID, Type: unifiedresources.RelChecks,
				Confidence: 1, Active: true,
			}},
		},
	}
	root := &Alert{ID: "host-offline", Type: "offline", Level: AlertLevelCritical, ResourceID: hostID, ResourceName: "Edge host", StartTime: startedAt}
	symptom := &Alert{ID: "checkout-unreachable", Type: "resource-incident", Level: AlertLevelCritical, ResourceID: endpointID, ResourceName: "Checkout", StartTime: startedAt, Metadata: map[string]interface{}{"incidentCode": "availability_unreachable"}}

	manager.mu.Lock()
	manager.applyInfrastructureIncidentSynthesisNoLock(resources, map[string]*Alert{"root": root, "symptom": symptom})
	manager.mu.Unlock()

	if root.Correlation == nil || root.Correlation.Inference != AlertCorrelationInferenceObservationSet {
		t.Fatalf("correlation = %+v, want observation set for contradictory healthy root", root.Correlation)
	}
	if isSupportedInfrastructureSymptom(symptom) {
		t.Fatal("an observation set must not suppress an independently notifying symptom")
	}
}

func TestInfrastructureIncidentSynthesisRequiresOrderedBoundedTiming(t *testing.T) {
	manager := newTestManager(t)
	startedAt := time.Date(2026, 8, 30, 18, 0, 0, 0, time.UTC)
	hostID := "agent:edge-1"
	endpointID := "availability:checkout"
	resources := map[string]unifiedresources.Resource{
		hostID: {ID: hostID, Type: unifiedresources.ResourceTypeAgent, Status: unifiedresources.StatusOffline},
		endpointID: {
			ID: endpointID, Type: unifiedresources.ResourceTypeNetworkEndpoint, Status: unifiedresources.StatusOffline,
			Relationships: []unifiedresources.ResourceRelationship{{SourceID: endpointID, TargetID: hostID, Type: unifiedresources.RelChecks, Confidence: 1, Active: true}},
		},
	}
	root := &Alert{ID: "host-offline", Type: "offline", Level: AlertLevelCritical, ResourceID: hostID, StartTime: startedAt}
	symptom := &Alert{ID: "checkout-unreachable", Type: "resource-incident", Level: AlertLevelCritical, ResourceID: endpointID, StartTime: startedAt.Add(maxSupportedCauseLead + time.Second), Metadata: map[string]interface{}{"incidentCode": "availability_unreachable"}}

	manager.mu.Lock()
	manager.applyInfrastructureIncidentSynthesisNoLock(resources, map[string]*Alert{"root": root, "symptom": symptom})
	manager.mu.Unlock()

	if root.Correlation == nil || root.Correlation.Inference != AlertCorrelationInferenceObservationSet {
		t.Fatalf("correlation = %+v, want observation set when the primary is too early", root.Correlation)
	}
	if isSupportedInfrastructureSymptom(symptom) {
		t.Fatal("late timing must not suppress the symptom's notification")
	}
}

func TestInfrastructureIncidentSynthesisPreservesExistingSharedSystemCorrelation(t *testing.T) {
	manager := newTestManager(t)
	startedAt := time.Date(2026, 8, 30, 18, 0, 0, 0, time.UTC)
	hostID := "agent:edge-1"
	endpointID := "availability:checkout"
	shared := &Alert{
		ID: "host-offline", Type: "offline", Level: AlertLevelCritical, ResourceID: hostID, StartTime: startedAt,
		Correlation: &AlertCorrelation{Key: "pve:edge", Kind: AlertCorrelationKindSharedSystem, Role: AlertCorrelationRolePrimary, Reason: "Provider-owned membership."},
	}
	manager.activeAlerts["root"] = shared
	desiredRoot := &Alert{ID: shared.ID, Type: shared.Type, Level: shared.Level, ResourceID: hostID, StartTime: startedAt}
	symptom := &Alert{ID: "checkout-unreachable", Type: "resource-incident", Level: AlertLevelCritical, ResourceID: endpointID, StartTime: startedAt, Metadata: map[string]interface{}{"incidentCode": "availability_unreachable"}}
	resources := map[string]unifiedresources.Resource{
		hostID: {ID: hostID, Type: unifiedresources.ResourceTypeAgent, Status: unifiedresources.StatusOffline},
		endpointID: {
			ID: endpointID, Type: unifiedresources.ResourceTypeNetworkEndpoint, Status: unifiedresources.StatusOffline,
			Relationships: []unifiedresources.ResourceRelationship{{SourceID: endpointID, TargetID: hostID, Type: unifiedresources.RelChecks, Confidence: 1, Active: true}},
		},
	}

	manager.mu.Lock()
	manager.applyInfrastructureIncidentSynthesisNoLock(resources, map[string]*Alert{"root": desiredRoot, "symptom": symptom})
	manager.mu.Unlock()

	if shared.Correlation == nil || shared.Correlation.Kind != AlertCorrelationKindSharedSystem {
		t.Fatalf("shared-system correlation = %+v, want preserved", shared.Correlation)
	}
	if desiredRoot.Correlation != nil || symptom.Correlation != nil {
		t.Fatalf("synthesis must not route through an authoritative shared-system member: root=%+v symptom=%+v", desiredRoot.Correlation, symptom.Correlation)
	}
}

func TestInfrastructureIncidentSynthesisPersistsAcrossReconciliation(t *testing.T) {
	manager := newTestManager(t)
	configureUnifiedEvalManager(t, manager, unifiedEvalBaseConfig())
	startedAt := time.Date(2026, 8, 30, 18, 0, 0, 0, time.UTC)
	hostID := "agent:edge-1"
	endpointID := "availability:checkout"
	resources := []unifiedresources.Resource{
		{
			ID: hostID, Type: unifiedresources.ResourceTypeAgent, Name: "Edge host", Status: unifiedresources.StatusOffline,
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "agent", NativeID: "edge-1", Code: "agent_offline", Severity: storagehealth.RiskCritical,
				Source: "agent.heartbeat", Summary: "Edge host is offline", StartedAt: startedAt, ConfirmationsRequired: 1, RecoveryConfirmationsRequired: 1,
			}},
		},
		{
			ID: endpointID, Type: unifiedresources.ResourceTypeNetworkEndpoint, Name: "Checkout", Status: unifiedresources.StatusOffline,
			Availability:  &unifiedresources.AvailabilityData{AggregateState: "unavailable", TransportOutcome: "unreachable"},
			Relationships: []unifiedresources.ResourceRelationship{{SourceID: endpointID, TargetID: hostID, Type: unifiedresources.RelChecks, Confidence: 1, Active: true}},
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "availability", NativeID: "checkout", Code: "availability_unreachable", Severity: storagehealth.RiskCritical,
				Source: "availability.probe", Summary: "Checkout is unreachable", StartedAt: startedAt.Add(time.Second), ConfirmationsRequired: 1, RecoveryConfirmationsRequired: 1,
			}},
		},
	}

	for pass := 1; pass <= 2; pass++ {
		manager.SyncUnifiedResourceIncidents(resources)
		active := manager.GetActiveAlerts()
		if len(active) != 2 {
			t.Fatalf("pass %d active alerts = %+v, want two", pass, active)
		}
		roles := map[AlertCorrelationRole]int{}
		for _, alert := range active {
			if alert.Correlation == nil || alert.Correlation.Kind != AlertCorrelationKindInfrastructureIncident {
				t.Fatalf("pass %d alert %q correlation = %+v, want infrastructure synthesis", pass, alert.ID, alert.Correlation)
			}
			roles[alert.Correlation.Role]++
		}
		if roles[AlertCorrelationRolePrimary] != 1 || roles[AlertCorrelationRoleSupporting] != 1 {
			t.Fatalf("pass %d roles = %+v, want one primary and one supporting", pass, roles)
		}
	}
}

func TestInfrastructureIncidentSynthesisShowsPartialRecovery(t *testing.T) {
	manager := newTestManager(t)
	startedAt := time.Date(2026, 8, 30, 18, 0, 0, 0, time.UTC)
	hostID := "agent:edge-1"
	endpointID := "availability:checkout"
	resources := map[string]unifiedresources.Resource{
		hostID: {ID: hostID, Type: unifiedresources.ResourceTypeAgent, Status: unifiedresources.StatusOffline},
		endpointID: {
			ID: endpointID, Type: unifiedresources.ResourceTypeNetworkEndpoint, Status: unifiedresources.StatusOffline,
			Relationships: []unifiedresources.ResourceRelationship{{SourceID: endpointID, TargetID: hostID, Type: unifiedresources.RelChecks, Confidence: 1, Active: true}},
		},
	}
	root := &Alert{ID: "host-offline", Type: "offline", Level: AlertLevelCritical, ResourceID: hostID, StartTime: startedAt}
	symptom := &Alert{ID: "checkout-unreachable", Type: "resource-incident", Level: AlertLevelCritical, ResourceID: endpointID, StartTime: startedAt, Metadata: map[string]interface{}{"incidentCode": "availability_unreachable"}}

	manager.mu.Lock()
	manager.applyInfrastructureIncidentSynthesisNoLock(resources, map[string]*Alert{"root": root, "symptom": symptom})
	if symptom.Correlation == nil {
		manager.mu.Unlock()
		t.Fatal("expected initial grouped symptom")
	}
	manager.applyInfrastructureIncidentSynthesisNoLock(resources, map[string]*Alert{"symptom": symptom})
	manager.mu.Unlock()

	if symptom.Correlation != nil {
		t.Fatalf("symptom correlation = %+v, want standalone active symptom after primary recovery", symptom.Correlation)
	}
}

func TestClassifyIncidentFailureSeparatesApplicationCertificateAndCoverage(t *testing.T) {
	applicationResource := unifiedresources.Resource{Availability: &unifiedresources.AvailabilityData{
		AggregateState: "unavailable", TransportOutcome: "reachable", ApplicationOutcome: "failed",
	}}
	application := &Alert{Type: "resource-incident", Metadata: map[string]interface{}{"incidentCode": "availability_unreachable"}}
	if got := classifyIncidentFailure(application, applicationResource); got != AlertFailureClassApplicationResponse {
		t.Fatalf("application failure class = %q", got)
	}
	certificate := &Alert{Type: "resource-incident", Metadata: map[string]interface{}{"incidentCode": "certificate_expired"}}
	if got := classifyIncidentFailure(certificate, unifiedresources.Resource{}); got != AlertFailureClassCertificate {
		t.Fatalf("certificate failure class = %q", got)
	}
	coverage := &Alert{Type: ExternalProbeUnavailableAlertType}
	if got := classifyIncidentFailure(coverage, unifiedresources.Resource{}); got != AlertFailureClassEvidenceCoverage {
		t.Fatalf("coverage failure class = %q", got)
	}
}

func TestSupportedInfrastructureSymptomUsesPrimaryNotification(t *testing.T) {
	manager := newTestManager(t)
	deliveries := 0
	manager.SetAlertCallback(func(*Alert) { deliveries++ })
	symptom := &Alert{
		ID: "checkout-unreachable", Type: "resource-incident", Level: AlertLevelCritical,
		ResourceID: "availability:checkout", ResourceName: "Checkout",
		Correlation: &AlertCorrelation{
			Key: "infrastructure:host-offline", Kind: AlertCorrelationKindInfrastructureIncident,
			Role: AlertCorrelationRoleSupporting, Reason: "Grouped beneath Edge host.",
			FailureClass: AlertFailureClassNetworkPath, Inference: AlertCorrelationInferenceSupportedCause,
			PrimaryAlertID: "host-offline", PrimaryResourceID: "agent:edge-1",
		},
	}

	if manager.dispatchAlert(symptom, false) {
		t.Fatal("supporting symptom must not dispatch a duplicate notification")
	}
	if deliveries != 0 {
		t.Fatalf("deliveries = %d, want none", deliveries)
	}

	manager.mu.Lock()
	diagnosis := manager.diagnoseActiveAlertLocked(symptom)
	manager.mu.Unlock()
	if diagnosis.Status != AlertDeliveryStatusSuppressed || diagnosis.Reason != AlertDeliveryReasonCorrelatedPrimary {
		t.Fatalf("diagnosis = %+v, want correlated-primary suppression", diagnosis)
	}
}
