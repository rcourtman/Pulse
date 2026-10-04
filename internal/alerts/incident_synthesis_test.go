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

func TestSupportedSymptomNotifiesWhenPrimaryRecoversButSymptomPersists(t *testing.T) {
	manager := newTestManager(t)
	configureUnifiedEvalManager(t, manager, unifiedEvalBaseConfig())
	startedAt := time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC)
	hostID := "agent:edge-1"
	endpointID := "availability:checkout"
	host := unifiedresources.Resource{
		ID: hostID, Type: unifiedresources.ResourceTypeAgent, Name: "Edge host", Status: unifiedresources.StatusOffline,
		Incidents: []unifiedresources.ResourceIncident{{
			Provider: "agent", NativeID: "edge-1", Code: "agent_offline", Severity: storagehealth.RiskCritical,
			Source: "agent.heartbeat", Summary: "Edge host is offline", StartedAt: startedAt,
			ConfirmationsRequired: 1, RecoveryConfirmationsRequired: 1,
		}},
	}
	endpoint := unifiedresources.Resource{
		ID: endpointID, Type: unifiedresources.ResourceTypeNetworkEndpoint, Name: "Checkout", Status: unifiedresources.StatusOffline,
		Availability: &unifiedresources.AvailabilityData{AggregateState: "unavailable", TransportOutcome: "unreachable"},
		Relationships: []unifiedresources.ResourceRelationship{{
			SourceID: endpointID, TargetID: hostID, Type: unifiedresources.RelChecks, Confidence: 1, Active: true,
		}},
		Incidents: []unifiedresources.ResourceIncident{{
			Provider: "availability", NativeID: "checkout", Code: "availability_unreachable", Severity: storagehealth.RiskCritical,
			Source: "availability.probe", Summary: "Checkout is unreachable", StartedAt: startedAt.Add(time.Second),
			ConfirmationsRequired: 1, RecoveryConfirmationsRequired: 1,
		}},
	}
	var delivered []string
	manager.SetAlertCallback(func(alert *Alert) { delivered = append(delivered, alert.ResourceID) })

	manager.SyncUnifiedResourceIncidents([]unifiedresources.Resource{host, endpoint})
	if len(delivered) != 1 || delivered[0] != hostID {
		t.Fatalf("grouped delivery = %v, want only the primary host", delivered)
	}
	for _, alert := range manager.GetActiveAlerts() {
		if alert.ResourceID == endpointID && (!isSupportedInfrastructureSymptom(&alert) || alert.LastNotified != nil) {
			t.Fatalf("grouped symptom = %+v, want supported and not separately dispatched", alert)
		}
	}

	// The host is healthy again, but the endpoint still fails. It must regain
	// its own notification path without requiring a new alert occurrence.
	host.Status = unifiedresources.StatusOnline
	host.Incidents = nil
	manager.SyncUnifiedResourceIncidents([]unifiedresources.Resource{host, endpoint})
	if len(delivered) != 2 || delivered[1] != endpointID {
		t.Fatalf("after primary recovery delivery = %v, want continuing symptom", delivered)
	}
	manager.SyncUnifiedResourceIncidents([]unifiedresources.Resource{host, endpoint})
	if len(delivered) != 2 {
		t.Fatalf("unchanged independent symptom sent again: %v", delivered)
	}
}

func TestPreviouslyNotifiedSymptomDoesNotDuplicateWhenPrimaryRecovers(t *testing.T) {
	manager := newTestManager(t)
	configureUnifiedEvalManager(t, manager, unifiedEvalBaseConfig())
	startedAt := time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC)
	hostID := "agent:edge-1"
	endpointID := "availability:checkout"
	host := unifiedresources.Resource{
		ID: hostID, Type: unifiedresources.ResourceTypeAgent, Status: unifiedresources.StatusOnline,
	}
	endpoint := unifiedresources.Resource{
		ID: endpointID, Type: unifiedresources.ResourceTypeNetworkEndpoint, Status: unifiedresources.StatusOffline,
		Availability: &unifiedresources.AvailabilityData{AggregateState: "unavailable", TransportOutcome: "unreachable"},
		Relationships: []unifiedresources.ResourceRelationship{{
			SourceID: endpointID, TargetID: hostID, Type: unifiedresources.RelChecks, Confidence: 1, Active: true,
		}},
		Incidents: []unifiedresources.ResourceIncident{{
			Provider: "availability", NativeID: "checkout", Code: "availability_unreachable", Severity: storagehealth.RiskCritical,
			Source: "availability.probe", StartedAt: startedAt.Add(time.Second), ConfirmationsRequired: 1, RecoveryConfirmationsRequired: 1,
		}},
	}
	var delivered []string
	manager.SetAlertCallback(func(alert *Alert) { delivered = append(delivered, alert.ResourceID) })

	manager.SyncUnifiedResourceIncidents([]unifiedresources.Resource{host, endpoint})
	if len(delivered) != 1 || delivered[0] != endpointID {
		t.Fatalf("standalone symptom delivery = %v, want endpoint", delivered)
	}
	host.Status = unifiedresources.StatusOffline
	host.Incidents = []unifiedresources.ResourceIncident{{
		Provider: "agent", NativeID: "edge-1", Code: "agent_offline", Severity: storagehealth.RiskCritical,
		Source: "agent.heartbeat", StartedAt: startedAt, ConfirmationsRequired: 1, RecoveryConfirmationsRequired: 1,
	}}
	manager.SyncUnifiedResourceIncidents([]unifiedresources.Resource{host, endpoint})
	if len(delivered) != 2 || delivered[1] != hostID {
		t.Fatalf("new primary delivery = %v, want endpoint then host", delivered)
	}
	var grouped bool
	for _, alert := range manager.GetActiveAlerts() {
		if alert.ResourceID == endpointID {
			grouped = isSupportedInfrastructureSymptom(&alert)
		}
	}
	if !grouped {
		t.Fatal("endpoint did not gain a supported primary")
	}

	host.Status = unifiedresources.StatusOnline
	host.Incidents = nil
	manager.SyncUnifiedResourceIncidents([]unifiedresources.Resource{host, endpoint})
	if len(delivered) != 2 {
		t.Fatalf("already-notified endpoint duplicated after primary recovery: %v", delivered)
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

func TestSupportedInfrastructureSymptomDoesNotEscalateOrRepeat(t *testing.T) {
	m := newTestManager(t)
	now := time.Date(2026, 9, 29, 19, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	m.mu.Lock()
	m.config.Enabled = true
	m.config.ActivationState = ActivationActive
	m.config.Schedule.Escalation = EscalationConfig{
		Enabled: true, RepeatCritical: true, RepeatEvery: 5,
		Levels: []EscalationLevel{{After: 5, Notify: "email"}},
	}
	correlation := &AlertCorrelation{
		Key: "infrastructure:host-offline", Kind: AlertCorrelationKindInfrastructureIncident,
		Role: AlertCorrelationRoleSupporting, Reason: "Verified host dependency.",
		Inference: AlertCorrelationInferenceSupportedCause, PrimaryAlertID: "host-offline",
	}
	newSymptom := &Alert{
		ID: "new-symptom", Level: AlertLevelCritical, StartTime: now.Add(-10 * time.Minute),
		Correlation: cloneAlertCorrelation(correlation),
	}
	repeatingSymptom := &Alert{
		ID: "repeating-symptom", Level: AlertLevelCritical, StartTime: now.Add(-30 * time.Minute),
		LastEscalation: 1, EscalationTimes: []time.Time{now.Add(-10 * time.Minute)},
		Correlation: cloneAlertCorrelation(correlation),
	}
	observationOnly := &Alert{
		ID: "observation-only", Level: AlertLevelCritical, StartTime: now.Add(-10 * time.Minute),
		Correlation: &AlertCorrelation{
			Key: "infrastructure:unconfirmed", Kind: AlertCorrelationKindInfrastructureIncident,
			Role: AlertCorrelationRoleSupporting, Reason: "Coincident observations only.",
			Inference: AlertCorrelationInferenceObservationSet, PrimaryAlertID: "host-offline",
		},
	}
	m.setActiveAlertNoLock(newSymptom.ID, newSymptom)
	m.setActiveAlertNoLock(repeatingSymptom.ID, repeatingSymptom)
	m.setActiveAlertNoLock(observationOnly.ID, observationOnly)
	m.mu.Unlock()

	m.checkEscalations()
	if newSymptom.LastEscalation != 0 || len(newSymptom.EscalationTimes) != 0 {
		t.Fatalf("supported symptom scheduled an escalation: %+v", newSymptom)
	}
	if repeatingSymptom.LastEscalation != 1 || len(repeatingSymptom.EscalationTimes) != 1 {
		t.Fatalf("supported symptom repeated an escalation: %+v", repeatingSymptom)
	}
	if observationOnly.LastEscalation != 1 || len(observationOnly.EscalationTimes) != 1 {
		t.Fatalf("uncorroborated observation lost independent escalation: %+v", observationOnly)
	}
}

func TestQueuedEscalationRejectsNewlySupportedInfrastructureSymptom(t *testing.T) {
	m := newTestManager(t)
	m.mu.Lock()
	m.config.Enabled = true
	m.config.ActivationState = ActivationActive
	m.config.Schedule.Escalation = EscalationConfig{
		Enabled: true, Levels: []EscalationLevel{{After: 5, Notify: "email"}},
	}
	alert := &Alert{ID: "newly-correlated", Level: AlertLevelCritical, StartTime: time.Now().Add(-10 * time.Minute)}
	m.setActiveAlertNoLock(alert.ID, alert)
	snapshot := cloneAlertForOutput(alert)
	alert.Correlation = &AlertCorrelation{
		Key: "infrastructure:host-offline", Kind: AlertCorrelationKindInfrastructureIncident,
		Role: AlertCorrelationRoleSupporting, Reason: "Verified host dependency.",
		Inference: AlertCorrelationInferenceSupportedCause, PrimaryAlertID: "host-offline",
	}
	alert.Metadata = map[string]interface{}{
		MetadataQuietHoursSuppressed: true,
		MetadataQuietHoursReplayAt:   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}
	m.mu.Unlock()

	if _, _, eligible := m.PrepareEscalationNotification(snapshot, 1); eligible {
		t.Fatal("queued escalation remained eligible after the symptom gained a supported primary")
	}
	if !m.ShouldSuppressNotification(alert) {
		t.Fatal("supported symptom remained eligible through the public delivery helper")
	}
	if hasQuietHoursNotificationReplay(alert) {
		t.Fatal("supported symptom retained a queued quiet-hours replay")
	}
}
