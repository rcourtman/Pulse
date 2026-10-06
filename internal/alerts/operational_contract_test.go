package alerts

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestEnsureOperationalContractBackfillsLegacyAlertHonestly(t *testing.T) {
	firstObservedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	lastObservedAt := firstObservedAt.Add(time.Minute)
	alert := &Alert{
		ID:              "legacy-alert",
		Type:            "cpu",
		Level:           AlertLevelWarning,
		ResourceID:      "resource-1",
		CanonicalSpecID: "metric-threshold:cpu",
		CanonicalKind:   "metric-threshold",
		CanonicalState:  "resource-1::metric-threshold:cpu",
		Message:         "CPU usage exceeded threshold",
		StartTime:       firstObservedAt,
		LastSeen:        lastObservedAt,
	}

	ensureOperationalContract(alert, lastObservedAt.Add(time.Second))

	if len(alert.Evidence) != 1 {
		t.Fatalf("evidence count = %d, want 1", len(alert.Evidence))
	}
	if got := alert.Evidence[0].Reason; got == nil || got.Code != legacyAlertEvidenceReason {
		t.Fatalf("evidence reason = %+v, want %q", got, legacyAlertEvidenceReason)
	}
	if got := alert.Evidence[0].Completeness; got != operationaltrust.EvidencePartial {
		t.Fatalf("evidence completeness = %q, want partial", got)
	}
	if err := alert.Evidence[0].Validate(); err != nil {
		t.Fatalf("evidence Validate() error = %v", err)
	}
	if alert.OperationalRecord == nil {
		t.Fatal("operational record is nil")
	}
	if got := alert.OperationalRecord.State; got != operationaltrust.OperationalOpen {
		t.Fatalf("operational state = %q, want open", got)
	}
	if len(alert.OperationalRecord.EvidenceIDs) != 1 ||
		alert.OperationalRecord.EvidenceIDs[0] != alert.Evidence[0].ID {
		t.Fatalf(
			"operational evidence ids = %v, want %q",
			alert.OperationalRecord.EvidenceIDs,
			alert.Evidence[0].ID,
		)
	}
	if err := alert.OperationalRecord.Validate(); err != nil {
		t.Fatalf("operational record Validate() error = %v", err)
	}
	if alert.LatestTransition == nil {
		t.Fatal("latest transition is nil")
	}
	if got := alert.LatestTransition.Cause; got != operationaltrust.TransitionDetectorDecision {
		t.Fatalf("transition cause = %q, want detector decision", got)
	}
	if err := alert.LatestTransition.Validate(); err != nil {
		t.Fatalf("transition Validate() error = %v", err)
	}
	if len(alert.Transitions) != 1 ||
		alert.Transitions[0].ID != alert.LatestTransition.ID {
		t.Fatalf("transition timeline = %+v, want initial transition", alert.Transitions)
	}
	if got, want := alert.OperationalRecord.RecommendedNextStep, "Open the affected resource and verify its current state before making changes."; got != want {
		t.Fatalf("RecommendedNextStep = %q, want %q", got, want)
	}
}

func TestIndefiniteOperationalSuppressionRemainsDeliverySafe(t *testing.T) {
	m := newTestManager(t)
	now := time.Now().UTC()
	cfg := m.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = ActivationActive
	m.UpdateConfig(cfg)
	alert := &Alert{ID: "maintenance:vm/100", CanonicalState: "maintenance:vm/100", CanonicalSpecID: "maintenance", Type: "maintenance", Level: AlertLevelWarning, ResourceID: "vm/100", ResourceName: "vm-100", StartTime: now, LastSeen: now}
	m.mu.Lock()
	m.setActiveAlertNoLock(alert.ID, alert)
	m.mu.Unlock()
	if err := m.SuppressOperationalAlert(alert.ID, "operator@example.com", "maintenance", nil); err != nil {
		t.Fatal(err)
	}
	if m.dispatchAlert(activeAlert(t, m, alert.ID), false) {
		t.Fatal("indefinitely suppressed alert was dispatched")
	}
	diagnosis, ok := m.DiagnoseAlertDelivery(alert.ID)
	if !ok || diagnosis.Reason != AlertDeliveryReasonSnoozed || diagnosis.SuppressedUntil != nil {
		t.Fatalf("diagnosis = %+v, exists=%v", diagnosis, ok)
	}
}

func TestEnsureOperationalContractUsesCanonicalIncidentAction(t *testing.T) {
	now := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	alert := &Alert{
		ID:           "storage:pool-1",
		Type:         "storage-incident",
		Level:        AlertLevelCritical,
		ResourceID:   "pool-1",
		ResourceName: "Pool 1",
		Message:      "Pool health is degraded",
		StartTime:    now.Add(-time.Minute),
		LastSeen:     now,
		Metadata: map[string]interface{}{
			"incidentAction": "Inspect the degraded devices and restore redundancy",
		},
	}

	ensureOperationalContract(alert, now)

	if alert.OperationalRecord == nil {
		t.Fatal("expected operational record")
	}
	if got, want := alert.OperationalRecord.RecommendedNextStep, "Inspect the degraded devices and restore redundancy"; got != want {
		t.Fatalf("RecommendedNextStep = %q, want %q", got, want)
	}
}

func TestCanonicalAlertEvidenceBuildsConfirmedEnvelope(t *testing.T) {
	observedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	spec := alertspecs.ResourceAlertSpec{
		ID:           "provider-incident:disk-1",
		ResourceID:   "disk-1",
		ResourceType: unifiedresources.ResourceTypeStorage,
		Kind:         alertspecs.AlertSpecKindProviderIncident,
	}
	evidence := alertspecs.AlertEvidence{
		ObservedAt: observedAt,
		ProviderIncident: &alertspecs.ProviderIncidentEvidence{
			Provider: "truenas",
			NativeID: "provider-alert-7",
			Source:   "truenas-alert-feed",
		},
	}

	first, ok := canonicalAlertEvidenceEnvelope(spec, evidence, "nas-a", observedAt.Add(time.Second))
	if !ok {
		t.Fatal("canonicalAlertEvidenceEnvelope() rejected valid evidence")
	}
	second, ok := canonicalAlertEvidenceEnvelope(spec, evidence, "nas-a", observedAt.Add(2*time.Second))
	if !ok {
		t.Fatal("canonicalAlertEvidenceEnvelope() retry rejected valid evidence")
	}
	if first.ID != second.ID {
		t.Fatalf("canonical evidence retry id = %q, want %q", second.ID, first.ID)
	}
	if first.Source.Provider != "truenas" ||
		first.Source.Collector != "truenas-alert-feed" ||
		first.Completeness != operationaltrust.EvidenceComplete ||
		first.Confidence != operationaltrust.EvidenceConfirmed {
		t.Fatalf("canonical evidence = %+v", first)
	}
	if first.PayloadRef == nil || first.PayloadRef.Kind != "alert-evidence-digest" {
		t.Fatalf("payload reference = %+v, want bounded digest", first.PayloadRef)
	}
}

func TestCanonicalAlertEvidencePreservesTypedLimitations(t *testing.T) {
	observedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	spec := alertspecs.ResourceAlertSpec{
		ID:           "connectivity:resource-1",
		ResourceID:   "resource-1",
		ResourceType: unifiedresources.ResourceTypeAgent,
		Kind:         alertspecs.AlertSpecKindConnectivity,
	}
	source := operationaltrust.EvidenceSource{
		Provider:  "proxmox",
		Collector: "connectivity-check",
	}
	subject := operationaltrust.EvidenceSubject{ResourceID: spec.ResourceID}
	id, err := operationaltrust.NewEvidenceID(source, subject, observedAt, "check-7")
	if err != nil {
		t.Fatalf("NewEvidenceID() error = %v", err)
	}
	envelope := operationaltrust.EvidenceEnvelope{
		ID:           id,
		Source:       source,
		Subject:      subject,
		ObservedAt:   observedAt,
		IngestedAt:   observedAt,
		Completeness: operationaltrust.EvidencePartial,
		Confidence:   operationaltrust.EvidenceUnknown,
		Permissions:  operationaltrust.EvidencePermissionsDenied,
		Reason:       &operationaltrust.EvidenceReason{Code: "permission_denied"},
	}
	evidence := alertspecs.AlertEvidence{
		ObservedAt: observedAt,
		Envelope:   &envelope,
		Connectivity: &alertspecs.ConnectivityEvidence{
			Signal:    "heartbeat",
			Connected: false,
		},
	}

	got, ok := canonicalAlertEvidenceEnvelope(spec, evidence, "pve-a", observedAt)
	if !ok {
		t.Fatal("canonicalAlertEvidenceEnvelope() rejected typed limited evidence")
	}
	if got.Permissions != operationaltrust.EvidencePermissionsDenied ||
		got.Reason == nil ||
		got.Reason.Code != "permission_denied" {
		t.Fatalf("limited evidence = %+v", got)
	}

	mismatched := envelope.Clone()
	mismatched.Subject.ResourceID = "other-resource"
	evidence.Envelope = &mismatched
	if _, ok := canonicalAlertEvidenceEnvelope(spec, evidence, "pve-a", observedAt); ok {
		t.Fatal("canonicalAlertEvidenceEnvelope() accepted mismatched subject")
	}
}

func TestEnsureOperationalContractTracksAcknowledgementWithoutResolution(t *testing.T) {
	firstObservedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	acknowledgedAt := firstObservedAt.Add(time.Minute)
	alert := &Alert{
		ID:              "alert-1",
		Type:            "offline",
		Level:           AlertLevelCritical,
		ResourceID:      "resource-1",
		CanonicalSpecID: "connectivity:offline",
		CanonicalState:  "resource-1::connectivity:offline",
		StartTime:       firstObservedAt,
		LastSeen:        acknowledgedAt,
		Acknowledged:    true,
		AckTime:         &acknowledgedAt,
		AckUser:         "operator-1",
	}

	ensureOperationalContract(alert, acknowledgedAt)

	record := alert.OperationalRecord
	if record == nil {
		t.Fatal("operational record is nil")
	}
	if record.State != operationaltrust.OperationalAcknowledged {
		t.Fatalf("state = %q, want acknowledged", record.State)
	}
	if record.ResolvedAt != nil {
		t.Fatalf("acknowledged record resolvedAt = %v, want nil", record.ResolvedAt)
	}
	if record.Acknowledgement == nil || record.Acknowledgement.By != "operator-1" {
		t.Fatalf("acknowledgement = %+v, want operator-1", record.Acknowledgement)
	}
	if alert.LatestTransition == nil ||
		alert.LatestTransition.Cause != operationaltrust.TransitionAcknowledgement {
		t.Fatalf("latest transition = %+v, want acknowledgement", alert.LatestTransition)
	}
	if alert.LatestTransition.To != operationaltrust.OperationalAcknowledged {
		t.Fatalf("transition to = %q, want acknowledged", alert.LatestTransition.To)
	}
}

func TestEnsureOperationalContractTracksUnacknowledgement(t *testing.T) {
	firstObservedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	acknowledgedAt := firstObservedAt.Add(time.Minute)
	unacknowledgedAt := acknowledgedAt.Add(time.Minute)
	alert := &Alert{
		ID:              "alert-1",
		Type:            "offline",
		Level:           AlertLevelCritical,
		ResourceID:      "resource-1",
		CanonicalSpecID: "connectivity:offline",
		CanonicalState:  "resource-1::connectivity:offline",
		StartTime:       firstObservedAt,
		LastSeen:        firstObservedAt,
	}
	ensureOperationalContract(alert, firstObservedAt)

	alert.Acknowledged = true
	alert.AckTime = &acknowledgedAt
	alert.AckUser = "operator-1"
	alert.LastSeen = acknowledgedAt
	ensureOperationalContract(alert, acknowledgedAt)

	alert.Acknowledged = false
	alert.AckTime = nil
	alert.AckUser = ""
	alert.LastSeen = unacknowledgedAt
	ensureOperationalContract(alert, unacknowledgedAt)

	if alert.OperationalRecord == nil ||
		alert.OperationalRecord.State != operationaltrust.OperationalOpen {
		t.Fatalf("record = %+v, want open", alert.OperationalRecord)
	}
	if alert.LatestTransition == nil ||
		alert.LatestTransition.Cause != operationaltrust.TransitionUnacknowledgement {
		t.Fatalf("latest transition = %+v, want unacknowledgement", alert.LatestTransition)
	}
	if len(alert.Transitions) != 3 {
		t.Fatalf("transition count = %d, want open, acknowledged, open", len(alert.Transitions))
	}
	for _, transition := range alert.Transitions {
		if err := transition.Validate(); err != nil {
			t.Fatalf("transition %q Validate() error = %v", transition.ID, err)
		}
	}
}

func TestNewResolvedAlertAddsDistinctRecoveryEvidence(t *testing.T) {
	firstObservedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	resolvedAt := firstObservedAt.Add(5 * time.Minute)
	alert := &Alert{
		ID:              "alert-1",
		Type:            "cpu",
		Level:           AlertLevelWarning,
		ResourceID:      "resource-1",
		CanonicalSpecID: "metric-threshold:cpu",
		CanonicalState:  "resource-1::metric-threshold:cpu",
		StartTime:       firstObservedAt,
		LastSeen:        firstObservedAt,
	}
	ensureOperationalContract(alert, firstObservedAt)
	triggerEvidenceID := alert.Evidence[0].ID

	resolved := newResolvedAlert(alert, resolvedAt, nil)

	if resolved == nil || resolved.OperationalRecord == nil {
		t.Fatal("resolved operational record is nil")
	}
	if resolved.OperationalRecord.State != operationaltrust.OperationalResolved {
		t.Fatalf("resolved state = %q, want resolved", resolved.OperationalRecord.State)
	}
	if len(resolved.Evidence) != 2 {
		t.Fatalf("resolved evidence count = %d, want trigger and recovery", len(resolved.Evidence))
	}
	recoveryEvidence := resolved.Evidence[1]
	if recoveryEvidence.ID == triggerEvidenceID {
		t.Fatal("recovery evidence reused trigger evidence id")
	}
	if recoveryEvidence.Reason == nil ||
		recoveryEvidence.Reason.Code != legacyAlertRecoveryEvidenceReason {
		t.Fatalf("recovery reason = %+v, want legacy recovery projection", recoveryEvidence.Reason)
	}
	if resolved.LatestTransition == nil ||
		resolved.LatestTransition.Cause != operationaltrust.TransitionRecoveryEvidence {
		t.Fatalf("resolved transition = %+v, want recovery evidence", resolved.LatestTransition)
	}
	if len(resolved.LatestTransition.EvidenceIDs) != 1 ||
		resolved.LatestTransition.EvidenceIDs[0] != recoveryEvidence.ID {
		t.Fatalf(
			"resolution transition evidence = %v, want %q",
			resolved.LatestTransition.EvidenceIDs,
			recoveryEvidence.ID,
		)
	}
	if err := resolved.OperationalRecord.Validate(); err != nil {
		t.Fatalf("resolved operational record Validate() error = %v", err)
	}
	if err := resolved.LatestTransition.Validate(); err != nil {
		t.Fatalf("resolved transition Validate() error = %v", err)
	}
	if len(resolved.Transitions) != 2 {
		t.Fatalf("resolved transition count = %d, want trigger and resolution", len(resolved.Transitions))
	}
	if resolved.Transitions[0].ID == resolved.Transitions[1].ID ||
		resolved.Transitions[1].ID != resolved.LatestTransition.ID {
		t.Fatalf("resolved transitions = %+v, want distinct ordered timeline", resolved.Transitions)
	}
}

func TestResolvedOperationalContractUpdatesHistorySnapshot(t *testing.T) {
	firstObservedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	resolvedAt := firstObservedAt.Add(5 * time.Minute)
	alert := &Alert{
		ID:              "alert-1",
		Type:            "cpu",
		Level:           AlertLevelWarning,
		ResourceID:      "resource-1",
		CanonicalSpecID: "metric-threshold:cpu",
		CanonicalState:  "resource-1::metric-threshold:cpu",
		StartTime:       firstObservedAt,
		LastSeen:        firstObservedAt,
	}
	ensureOperationalContract(alert, firstObservedAt)

	history := newTestHistoryManager(t)
	history.AddAlert(*alert)
	manager := &Manager{historyManager: history}
	if resolved := manager.newResolvedAlert(alert, resolvedAt, nil); resolved == nil {
		t.Fatal("newResolvedAlert() returned nil")
	}

	entries := history.GetAllHistory(1)
	if len(entries) != 1 {
		t.Fatalf("history entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.OperationalRecord == nil ||
		entry.OperationalRecord.State != operationaltrust.OperationalResolved {
		t.Fatalf("history operational record = %+v, want resolved", entry.OperationalRecord)
	}
	if entry.LatestTransition == nil ||
		entry.LatestTransition.ID != alert.LatestTransition.ID {
		t.Fatalf("history latest transition = %+v, want %q", entry.LatestTransition, alert.LatestTransition.ID)
	}
	if len(entry.Transitions) != 2 || len(entry.Evidence) != 2 {
		t.Fatalf(
			"history contract transitions/evidence = %d/%d, want 2/2",
			len(entry.Transitions),
			len(entry.Evidence),
		)
	}
}

func TestReleasedMetricAlertCarriesResolutionInsteadOfRecovery(t *testing.T) {
	manager := newUnifiedEvalParityManager(t)
	store, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory event log: %v", err)
	}
	manager.SetEventLog(store)
	manager.UpdateConfig(AlertConfig{
		Enabled: true,
		NodeDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
		},
	})
	disableTestTimeThresholds(manager)
	node := models.Node{
		ID:       "pve-1",
		Name:     "pve-1",
		Instance: "test",
		Status:   "online",
		CPU:      0.95,
		Memory:   models.Memory{Total: 8 << 30, Used: 7 << 30, Usage: 95},
	}
	manager.CheckNode(node)
	if active := manager.GetActiveAlerts(); len(active) != 2 {
		t.Fatalf("active alerts = %#v, want cpu and memory", active)
	}

	resolvedByType := map[string]*ResolvedAlert{}
	manager.SetResolvedAlertCallback(func(resolved *ResolvedAlert) {
		resolvedByType[resolved.Alert.Type] = resolved
	})
	release := func(metric string, resolution *AlertResolution) {
		t.Helper()
		spec, err := buildCanonicalMetricSpec(node.ID, node.Name, unifiedresources.ResourceType("node"), metric, nil)
		if err != nil {
			t.Fatalf("build %s spec: %v", metric, err)
		}
		manager.releaseCanonicalMetricAlert(spec, node.Name, node.Name, node.Instance, "node", 0, resolution)
	}

	handover := &AlertResolution{
		Reason:              AlertResolutionMovedToAgent,
		SuccessorResourceID: "agent-pve-1",
		SuccessorName:       "pve-1 (Host Agent)",
	}
	release("memory", handover)
	// The close is recorded as a copy: the caller's value cannot rewrite it.
	handover.SuccessorName = "changed"
	release("cpu", nil)

	if active := manager.GetActiveAlerts(); len(active) != 0 {
		t.Fatalf("active alerts after release = %#v, want none: the close must still happen", active)
	}

	moved := resolvedByType["memory"]
	if moved == nil {
		t.Fatal("memory release did not reach the resolved consumer")
	}
	resolution := moved.Alert.Resolution
	if resolution == nil ||
		resolution.Reason != AlertResolutionMovedToAgent ||
		resolution.SuccessorResourceID != "agent-pve-1" ||
		resolution.SuccessorName != "pve-1 (Host Agent)" {
		t.Fatalf("resolution = %+v, want moved to pve-1 (Host Agent)", resolution)
	}
	wantSummary := "Alert moved to pve-1 (Host Agent). This is not a recovery: check the agent for the current reading."
	if got := resolution.Summary(); got != wantSummary {
		t.Fatalf("Summary() = %q, want %q", got, wantSummary)
	}
	if got := resolution.Outcome(); got != "moved to pve-1 (Host Agent)" {
		t.Fatalf("Outcome() = %q", got)
	}
	if got := resolution.Describe("Memory alert"); got != "Memory alert moved to pve-1 (Host Agent). This is not a recovery: check the agent for the current reading." {
		t.Fatalf("Describe() = %q", got)
	}

	record := moved.Alert.OperationalRecord
	if record == nil || record.State != operationaltrust.OperationalResolved {
		t.Fatalf("operational record = %+v, want resolved", record)
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("operational record Validate() error = %v", err)
	}
	transition := moved.Alert.LatestTransition
	if transition == nil || transition.Reason != wantSummary {
		t.Fatalf("resolve transition = %+v, want reason %q", transition, wantSummary)
	}
	if err := transition.Validate(); err != nil {
		t.Fatalf("resolve transition Validate() error = %v", err)
	}
	// Patrol's timeline prints the cause above the reason, so a handover
	// must not be labelled as recovery evidence.
	if transition.Cause != operationaltrust.TransitionOwnershipTransferred {
		t.Fatalf("resolve transition cause = %q, want %q", transition.Cause, operationaltrust.TransitionOwnershipTransferred)
	}
	if recorded := moved.Alert.Transitions[len(moved.Alert.Transitions)-1]; recorded.ID != transition.ID ||
		recorded.Cause != operationaltrust.TransitionOwnershipTransferred {
		t.Fatalf("recorded timeline transition = %+v, want the ownership transfer", recorded)
	}
	closing := moved.Alert.Evidence[len(moved.Alert.Evidence)-1]
	if closing.ID != transition.EvidenceIDs[0] ||
		closing.Reason == nil ||
		closing.Reason.Code != string(AlertResolutionMovedToAgent) ||
		closing.Reason.Message != wantSummary {
		t.Fatalf("closing evidence = %+v, want the handover, not a recovery projection", closing)
	}

	recovered := resolvedByType["cpu"]
	if recovered == nil || recovered.Alert.Resolution != nil {
		t.Fatalf("plain disabled release = %+v, want an ordinary resolve with no resolution", recovered)
	}
	if last := recovered.Alert.Evidence[len(recovered.Alert.Evidence)-1]; last.Reason == nil ||
		last.Reason.Code != legacyAlertRecoveryEvidenceReason {
		t.Fatalf("plain release evidence = %+v, want legacy recovery projection", last.Reason)
	}
	if recovered.Alert.LatestTransition == nil ||
		recovered.Alert.LatestTransition.Cause != operationaltrust.TransitionRecoveryEvidence {
		t.Fatalf("plain release transition = %+v, want recovery_evidence", recovered.Alert.LatestTransition)
	}

	var historyResolution *AlertResolution
	for _, entry := range manager.GetAlertHistory(10) {
		if entry.Type == "memory" {
			historyResolution = entry.Resolution
		}
	}
	if historyResolution == nil || historyResolution.Reason != AlertResolutionMovedToAgent {
		t.Fatalf("history resolution = %+v, want moved_to_agent", historyResolution)
	}

	events, err := manager.AlertEvents(eventlog.Filter{AlertID: moved.Alert.ID, Types: []string{eventlog.TypeResolved}})
	if err != nil {
		t.Fatalf("AlertEvents: %v", err)
	}
	if len(events) != 1 ||
		events[0].Reason != string(AlertResolutionMovedToAgent) ||
		events[0].Message != wantSummary {
		t.Fatalf("resolved events = %+v, want one handover event", events)
	}
}

func TestResolvedHistoryRowKeepsResolutionWithoutEventLog(t *testing.T) {
	firstObservedAt := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	alert := &Alert{
		ID:              "pve-1-memory",
		Type:            "memory",
		Level:           AlertLevelWarning,
		ResourceID:      "pve-1",
		CanonicalSpecID: "metric-threshold:memory",
		CanonicalState:  "pve-1::metric-threshold:memory",
		StartTime:       firstObservedAt,
		LastSeen:        firstObservedAt,
	}
	ensureOperationalContract(alert, firstObservedAt)
	history := newTestHistoryManager(t)
	history.AddAlert(*alert)
	manager := &Manager{historyManager: history}

	alert.Resolution = &AlertResolution{Reason: AlertResolutionMovedToAgent, SuccessorName: "pve-1 (Host Agent)"}
	manager.newResolvedAlert(alert, firstObservedAt.Add(time.Minute), nil)

	entries := history.GetAllHistory(1)
	if len(entries) != 1 || entries[0].Resolution == nil ||
		entries[0].Resolution.Reason != AlertResolutionMovedToAgent {
		t.Fatalf("history entries = %+v, want the row to keep the moved_to_agent resolution", entries)
	}
}

func TestMergeOperationalRecurrenceReopensStableCause(t *testing.T) {
	firstObservedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	resolvedAt := firstObservedAt.Add(5 * time.Minute)
	recurredAt := resolvedAt.Add(time.Minute)
	previous := &Alert{
		ID:              "alert-1",
		Type:            "cpu",
		Level:           AlertLevelWarning,
		ResourceID:      "resource-1",
		CanonicalSpecID: "metric-threshold:cpu",
		CanonicalState:  "resource-1::metric-threshold:cpu",
		StartTime:       firstObservedAt,
		LastSeen:        firstObservedAt,
	}
	ensureOperationalContract(previous, firstObservedAt)
	newResolvedAlert(previous, resolvedAt, nil)

	recurrence := &Alert{
		ID:              previous.ID,
		Type:            previous.Type,
		Level:           previous.Level,
		ResourceID:      previous.ResourceID,
		CanonicalSpecID: previous.CanonicalSpecID,
		CanonicalState:  previous.CanonicalState,
		StartTime:       previous.StartTime,
		LastSeen:        recurredAt,
	}
	ensureOperationalContract(recurrence, recurredAt)
	recurrenceEvidenceID := recurrence.Evidence[0].ID

	if !mergeOperationalRecurrence(recurrence, previous, recurredAt) {
		t.Fatal("mergeOperationalRecurrence() did not reopen resolved cause")
	}
	if recurrence.OperationalRecord == nil ||
		recurrence.OperationalRecord.ID != previous.OperationalRecord.ID ||
		recurrence.OperationalRecord.State != operationaltrust.OperationalOpen ||
		recurrence.OperationalRecord.ResolvedAt != nil {
		t.Fatalf("recurrence record = %+v", recurrence.OperationalRecord)
	}
	if recurrence.LatestTransition == nil ||
		recurrence.LatestTransition.From != operationaltrust.OperationalResolved ||
		recurrence.LatestTransition.To != operationaltrust.OperationalOpen ||
		recurrence.LatestTransition.Cause != operationaltrust.TransitionDetectorDecision {
		t.Fatalf("recurrence transition = %+v", recurrence.LatestTransition)
	}
	if len(recurrence.Transitions) != 3 {
		t.Fatalf("recurrence transition count = %d, want open, resolved, reopened", len(recurrence.Transitions))
	}
	if len(recurrence.Evidence) != 3 {
		t.Fatalf("recurrence evidence count = %d, want trigger, recovery, recurrence", len(recurrence.Evidence))
	}
	if recurrence.Evidence[2].ID != recurrenceEvidenceID {
		t.Fatalf("recurrence evidence id = %q, want %q", recurrence.Evidence[2].ID, recurrenceEvidenceID)
	}
	for _, transition := range recurrence.Transitions {
		if err := transition.Validate(); err != nil {
			t.Fatalf("transition %q Validate() error = %v", transition.ID, err)
		}
	}
}

func TestEnsureOperationalContractDoesNotInventCanonicalSubject(t *testing.T) {
	alert := &Alert{
		ID:        "provider-only-alert",
		Type:      "provider-incident",
		Instance:  "provider-a",
		StartTime: time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC),
	}

	ensureOperationalContract(alert, alert.StartTime)

	if alert.OperationalRecord != nil {
		t.Fatalf("operational record = %+v, want nil without canonical subject", alert.OperationalRecord)
	}
	if len(alert.Evidence) != 1 || alert.Evidence[0].Subject.ProviderRef == "" {
		t.Fatalf("unresolved evidence = %+v, want scoped provider reference", alert.Evidence)
	}
}

func TestAlertCloneDoesNotShareOperationalContractState(t *testing.T) {
	now := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	alert := &Alert{
		ID:              "alert-1",
		Type:            "cpu",
		Level:           AlertLevelWarning,
		ResourceID:      "resource-1",
		CanonicalSpecID: "metric-threshold:cpu",
		CanonicalState:  "resource-1::metric-threshold:cpu",
		StartTime:       now,
		LastSeen:        now,
	}
	ensureOperationalContract(alert, now)

	clone := alert.Clone()
	clone.OperationalRecord.EvidenceIDs[0] = "mutated"
	clone.LatestTransition.EvidenceIDs[0] = "mutated"
	clone.Transitions[0].EvidenceIDs[0] = "mutated"
	clone.Evidence[0].Reason.Code = "mutated"

	if alert.OperationalRecord.EvidenceIDs[0] == "mutated" ||
		alert.LatestTransition.EvidenceIDs[0] == "mutated" ||
		alert.Transitions[0].EvidenceIDs[0] == "mutated" ||
		alert.Evidence[0].Reason.Code == "mutated" {
		t.Fatal("Alert.Clone() shared operational contract state")
	}
}

func TestAlertOperationalContractJSONRemainsAdditive(t *testing.T) {
	legacyPayload := []byte(`{
		"id":"alert-1",
		"type":"cpu",
		"level":"warning",
		"resourceId":"resource-1",
		"resourceName":"vm-1",
		"node":"node-1",
		"instance":"pve-1",
		"message":"CPU high",
		"value":95,
		"threshold":90,
		"startTime":"2026-07-18T20:00:00Z",
		"lastSeen":"2026-07-18T20:00:00Z",
		"acknowledged":false
	}`)
	var alert Alert
	if err := json.Unmarshal(legacyPayload, &alert); err != nil {
		t.Fatalf("legacy alert unmarshal error = %v", err)
	}
	ensureOperationalContract(&alert, alert.LastSeen)
	payload, err := json.Marshal(alert)
	if err != nil {
		t.Fatalf("operational alert marshal error = %v", err)
	}
	var roundTrip Alert
	if err := json.Unmarshal(payload, &roundTrip); err != nil {
		t.Fatalf("operational alert unmarshal error = %v", err)
	}
	if roundTrip.ID != "alert-1" ||
		roundTrip.OperationalRecord == nil ||
		len(roundTrip.Transitions) != 1 ||
		len(roundTrip.Evidence) != 1 {
		t.Fatalf("round-trip alert = %+v", roundTrip)
	}
}

func TestEnsureOperationalContractKeepsExplicitUncertainStateAheadOfLegacyAcknowledgement(t *testing.T) {
	observedAt := time.Date(2026, 7, 19, 6, 0, 0, 0, time.UTC)
	alert := &Alert{
		ID:              "alert-uncertain",
		Type:            "availability",
		ResourceID:      "app-container:web",
		CanonicalSpecID: "availability",
		CanonicalState:  "app-container:web::availability",
		StartTime:       observedAt.Add(-time.Hour),
		LastSeen:        observedAt.Add(-time.Minute),
		Acknowledged:    true,
		OperationalRecord: &operationaltrust.OperationalRecord{
			ID:                "alert-uncertain",
			CanonicalSpecID:   "availability",
			SubjectResourceID: "app-container:web",
			State:             operationaltrust.OperationalStale,
			Severity:          operationaltrust.SeverityWarning,
			FirstObservedAt:   observedAt.Add(-time.Hour),
			LastObservedAt:    observedAt,
			StateChangedAt:    observedAt.Add(-time.Minute),
			CauseKey:          "availability",
			EvidenceIDs:       []string{},
		},
	}

	ensureOperationalContract(alert, observedAt.Add(time.Minute))

	if got := alert.OperationalRecord.State; got != operationaltrust.OperationalStale {
		t.Fatalf("operational state = %q, want stale", got)
	}
	if got := alert.OperationalRecord.LastObservedAt; !got.Equal(observedAt) {
		t.Fatalf("LastObservedAt = %s, want %s", got, observedAt)
	}
}

func TestMemoryUnavailablePreservesActiveAlertsUntilTrustedRecovery(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Manager)
		check func(*Manager, models.Memory)
	}{
		{
			name: "guest",
			setup: func(manager *Manager) {
				manager.UpdateConfig(AlertConfig{
					Enabled: true,
					GuestDefaults: ThresholdConfig{
						Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
					},
				})
			},
			check: func(manager *Manager, memory models.Memory) {
				manager.CheckGuest(models.VM{
					ID:       "qemu/1501",
					Name:     "linux-guest",
					Node:     "pve-1",
					Instance: "test",
					Status:   "running",
					Memory:   memory,
				}, "test")
			},
		},
		{
			name: "node",
			setup: func(manager *Manager) {
				manager.UpdateConfig(AlertConfig{
					Enabled: true,
					NodeDefaults: ThresholdConfig{
						Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
					},
				})
			},
			check: func(manager *Manager, memory models.Memory) {
				manager.CheckNode(models.Node{
					ID:       "pve-1",
					Name:     "pve-1",
					Instance: "test",
					Status:   "online",
					Memory:   memory,
				})
			},
		},
		{
			name: "agent",
			setup: func(manager *Manager) {
				manager.UpdateConfig(AlertConfig{
					Enabled: true,
					AgentDefaults: ThresholdConfig{
						Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
					},
				})
			},
			check: func(manager *Manager, memory models.Memory) {
				manager.CheckHost(models.Host{
					ID:       "agent-1501",
					Hostname: "linux-host",
					Status:   "online",
					Memory:   memory,
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := newUnifiedEvalParityManager(t)
			tt.setup(manager)
			disableTestTimeThresholds(manager)

			tt.check(manager, models.Memory{
				Total: 8 << 30,
				Used:  7 << 30,
				Usage: 90,
			})
			active := manager.GetActiveAlerts()
			if len(active) != 1 || active[0].Type != "memory" {
				t.Fatalf("active alerts after trusted high sample = %#v, want one memory alert", active)
			}

			tt.check(manager, models.UnavailableMemory(8<<30))
			active = manager.GetActiveAlerts()
			if len(active) != 1 || active[0].Type != "memory" || active[0].Value != 90 {
				t.Fatalf("active alerts after unavailable sample = %#v, want prior memory alert", active)
			}

			tt.check(manager, models.Memory{
				Total: 8 << 30,
				Used:  5 << 30,
				Usage: 70,
			})
			if active = manager.GetActiveAlerts(); len(active) != 0 {
				t.Fatalf("active alerts after trusted recovery = %#v, want cleared", active)
			}
		})
	}
}

func TestResolvedOccurrenceCallbackCapturesBeforeDispatch(t *testing.T) {
	m := newTestManager(t)
	start := time.Now().Add(-time.Hour)
	end := start.Add(time.Minute)
	resolved := &ResolvedAlert{Alert: &Alert{ID: "reusable", StartTime: start,
		Metadata: map[string]interface{}{"generation": "old"}}, ResolvedTime: end}
	entered, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	m.SetResolvedCallback(func(id string) {
		if id != "reusable" {
			t.Errorf("legacy ID = %s", id)
		}
		close(entered)
		<-release
	})
	got := make(chan *ResolvedAlert, 1)
	m.SetResolvedAlertCallback(func(r *ResolvedAlert) { got <- r })
	m.safeCallResolvedAlertCallback(resolved, resolved.Alert.ID, true)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("legacy callback did not start")
	}
	// Both the source object and the ID-indexed history can change while the
	// consumer waits. Neither is a source for the already-admitted snapshot.
	resolved.Alert.StartTime = start.Add(time.Hour)
	resolved.Alert.Metadata["generation"] = "new"
	resolved.ResolvedTime = end.Add(time.Hour)
	m.addRecentlyResolvedUnlocked(resolved)
	// Unblock without closing twice in cleanup.
	release <- struct{}{}
	select {
	case r := <-got:
		if !r.Alert.StartTime.Equal(start) || !r.ResolvedTime.Equal(end) || r.Alert.Metadata["generation"] != "old" {
			t.Fatalf("callback reconstructed the wrong occurrence: %+v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("no occurrence callback")
	}
}

func TestResolvedOccurrenceCallbackSurvivesLegacyPanic(t *testing.T) {
	m := newTestManager(t)
	m.SetResolvedCallback(func(string) { panic("legacy consumer failure") })
	called := false
	m.SetResolvedAlertCallback(func(r *ResolvedAlert) { called = r.Alert.ID == "reusable" })
	m.safeCallResolvedAlertCallback(&ResolvedAlert{Alert: &Alert{ID: "reusable"}}, "reusable", false)
	if !called {
		t.Fatal("legacy panic prevented occurrence-qualified recovery")
	}
}

func TestConnectionDegradedAlertNamesItsPlatform(t *testing.T) {
	tests := []struct {
		connectionType ConnectionType
		wantPlatform   string
	}{
		{connectionType: ConnectionTypePVE, wantPlatform: "proxmox-pve"},
		{connectionType: ConnectionTypePBS, wantPlatform: "proxmox-pbs"},
		{connectionType: ConnectionTypePMG, wantPlatform: "proxmox-pmg"},
		{connectionType: ConnectionTypeVMware, wantPlatform: "vmware-vsphere"},
		{connectionType: ConnectionTypeTrueNAS, wantPlatform: "truenas"},
	}

	for _, test := range tests {
		t.Run(string(test.connectionType), func(t *testing.T) {
			m := newTestManager(t)
			snap := ConnectionSnapshot{
				ID:      string(test.connectionType) + ":lab",
				Name:    "Lab",
				Type:    test.connectionType,
				State:   ConnectionStateUnreachable,
				Enabled: true,
			}
			for range 5 {
				m.CheckConnection(snap)
			}

			alert := testRequireActiveAlert(t, m, canonicalDiscreteStateStateID(snap.ID, connectionDegradedStateKey))
			if got, _ := alert.Metadata["platformType"].(string); got != test.wantPlatform {
				t.Fatalf("platformType metadata = %q, want %q", got, test.wantPlatform)
			}
		})
	}
}

func TestCapacityForecastAlertNamesItsPlatform(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()
	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.VMwareDefaults.Usage = &HysteresisThreshold{Trigger: 85, Clear: 75}
	m.mu.Unlock()

	input := &UnifiedResourceInput{
		ID:       "vmware:vc-1/datastore:ds-ssd",
		Type:     "vmware-datastore",
		Name:     "ds-ssd",
		Node:     "Lab DC",
		Instance: "Lab vCenter",
		Disk:     &UnifiedResourceMetric{Percent: 70},
	}
	trend := CapacityTrendObservation{
		Ready: true, Reason: "increasing", ObservedAt: time.Now(), DailyChange: 7,
		Confidence: 0.99, SampleCount: 300, BucketCount: 48, CoverageSpan: 48 * time.Hour,
	}
	m.CheckUnifiedResourceWithCapacityTrend(input, trend)
	m.CheckUnifiedResourceWithCapacityTrend(input, trend)

	alert := testRequireActiveAlert(t, m, canonicalMetricStateID(input.ID, "usage"))
	if got := alert.Metadata[capacityAlertOriginKey]; got != capacityAlertOriginForecast {
		t.Fatalf("capacity origin = %v, want forecast", got)
	}
	if got := alert.Metadata[alertPlatformTypeKey]; got != "vmware-vsphere" {
		t.Fatalf("platformType metadata = %v, want vmware-vsphere", got)
	}
}
