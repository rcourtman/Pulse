package alerts

import (
	"slices"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

func TestOperationalStateWritersPreserveFindingTruthAcrossSuppressionAndCollectionFailure(t *testing.T) {
	manager := newTestManager(t)
	alertID := "docker-container-health:container/web"
	now := time.Now().UTC()
	alert := &Alert{
		ID:              alertID,
		CanonicalState:  alertID,
		CanonicalSpecID: "docker-container-health",
		Type:            "docker-container-health",
		Level:           AlertLevelCritical,
		ResourceID:      "app-container:web",
		ResourceName:    "web",
		StartTime:       now.Add(-time.Minute),
		LastSeen:        now,
		Message:         "Container health check is failing.",
	}
	manager.mu.Lock()
	manager.setActiveAlertNoLock(alertID, alert)
	manager.mu.Unlock()

	expiresAt := now.Add(time.Hour)
	if err := manager.SuppressOperationalAlert(
		alertID,
		"operator@example.com",
		"Maintenance is in progress.",
		&expiresAt,
	); err != nil {
		t.Fatalf("SuppressOperationalAlert() error = %v", err)
	}
	suppressed := activeAlert(t, manager, alertID)
	if suppressed.OperationalRecord.State != operationaltrust.OperationalSuppressed {
		t.Fatalf("suppressed state = %q", suppressed.OperationalRecord.State)
	}
	if suppressed.LatestTransition == nil ||
		suppressed.LatestTransition.Cause != operationaltrust.TransitionSuppression {
		t.Fatalf("suppression transition = %+v", suppressed.LatestTransition)
	}

	if err := manager.UnsuppressOperationalAlert(alertID); err != nil {
		t.Fatalf("UnsuppressOperationalAlert() error = %v", err)
	}
	reopened := activeAlert(t, manager, alertID)
	if reopened.OperationalRecord.State != operationaltrust.OperationalOpen {
		t.Fatalf("reopened state = %q", reopened.OperationalRecord.State)
	}
	if reopened.LatestTransition == nil ||
		reopened.LatestTransition.Cause != operationaltrust.TransitionSuppressionExpired {
		t.Fatalf("unsuppression transition = %+v", reopened.LatestTransition)
	}

	staleEvidence := operationalStateWriterEvidence(
		t,
		"availability",
		"app-container:web",
		now.Add(-2*time.Minute),
		now.Add(-time.Minute),
		operationaltrust.EvidenceUnavailable,
		operationaltrust.EvidencePermissionsSufficient,
		"collector_disconnected",
	)
	if err := manager.MarkOperationalCollectionStale(
		alertID,
		staleEvidence,
		"Collector has not completed a current observation.",
	); err != nil {
		t.Fatalf("MarkOperationalCollectionStale() error = %v", err)
	}
	stale := activeAlert(t, manager, alertID)
	if stale.OperationalRecord.State != operationaltrust.OperationalStale {
		t.Fatalf("stale state = %q", stale.OperationalRecord.State)
	}
	if stale.OperationalRecord.ResolvedAt != nil {
		t.Fatal("stale collection must not resolve the operational record")
	}

	unknownEvidence := operationalStateWriterEvidence(
		t,
		"docker",
		"app-container:web",
		now,
		now.Add(time.Minute),
		operationaltrust.EvidencePartial,
		operationaltrust.EvidencePermissionsDenied,
		"provider_permission_denied",
	)
	if err := manager.MarkOperationalCollectionUnknown(
		alertID,
		unknownEvidence,
		"Provider permissions no longer support a health decision.",
	); err != nil {
		t.Fatalf("MarkOperationalCollectionUnknown() error = %v", err)
	}
	unknown := activeAlert(t, manager, alertID)
	if unknown.OperationalRecord.State != operationaltrust.OperationalUnknown {
		t.Fatalf("unknown state = %q", unknown.OperationalRecord.State)
	}
	if unknown.LatestTransition == nil ||
		unknown.LatestTransition.Cause != operationaltrust.TransitionCollectionUnknown {
		t.Fatalf("unknown transition = %+v", unknown.LatestTransition)
	}

	freshEvidence := operationalStateWriterEvidence(
		t,
		"docker",
		"app-container:web",
		now.Add(time.Second),
		now.Add(2*time.Minute),
		operationaltrust.EvidenceComplete,
		operationaltrust.EvidencePermissionsSufficient,
		"",
	)
	if err := manager.RestoreOperationalCollectionState(alertID, freshEvidence); err != nil {
		t.Fatalf("RestoreOperationalCollectionState() error = %v", err)
	}
	restored := activeAlert(t, manager, alertID)
	if restored.OperationalRecord.State != operationaltrust.OperationalOpen {
		t.Fatalf("restored state = %q", restored.OperationalRecord.State)
	}
	if restored.OperationalRecord.ResolvedAt != nil {
		t.Fatal("restored collection must reopen, not resolve, the finding")
	}
}

func TestMarkOperationalResolvingRequiresFreshRecoveryEvidence(t *testing.T) {
	manager := newTestManager(t)
	now := time.Now().UTC()
	alert := &Alert{
		ID:              "container-health",
		CanonicalState:  "container-health",
		CanonicalSpecID: "docker-container-health",
		Type:            "docker-container-health",
		Level:           AlertLevelWarning,
		ResourceID:      "app-container:web",
		StartTime:       now.Add(-time.Minute),
		LastSeen:        now,
	}
	manager.mu.Lock()
	manager.setActiveAlertNoLock(alert.ID, alert)
	manager.mu.Unlock()

	invalid := operationaltrust.EvidenceEnvelope{}
	if err := manager.MarkOperationalResolving(
		alert.ID,
		invalid,
		"Waiting for a post-action health observation.",
	); err == nil {
		t.Fatal("invalid recovery evidence must fail closed")
	}

	recovery := operationalStateWriterEvidence(
		t,
		"docker",
		"app-container:web",
		now.Add(time.Second),
		now.Add(time.Minute),
		operationaltrust.EvidenceComplete,
		operationaltrust.EvidencePermissionsSufficient,
		"",
	)
	if err := manager.MarkOperationalResolving(
		alert.ID,
		recovery,
		"Waiting for a post-action health observation.",
	); err != nil {
		t.Fatalf("MarkOperationalResolving() error = %v", err)
	}
	current := activeAlert(t, manager, alert.ID)
	if current.OperationalRecord.State != operationaltrust.OperationalResolving {
		t.Fatalf("resolving state = %q", current.OperationalRecord.State)
	}
	if current.LatestTransition == nil ||
		current.LatestTransition.Cause != operationaltrust.TransitionRecoveryEvidence {
		t.Fatalf("resolving transition = %+v", current.LatestTransition)
	}
}

func TestOperationalSuppressionSurvivesManagerRestart(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewManagerWithDataDir(dataDir)
	now := time.Now().UTC()
	alert := &Alert{
		ID:              "container-health",
		CanonicalState:  "container-health",
		CanonicalSpecID: "docker-container-health",
		Type:            "docker-container-health",
		Level:           AlertLevelWarning,
		ResourceID:      "app-container:web",
		ResourceName:    "web",
		StartTime:       now.Add(-time.Minute),
		LastSeen:        now,
	}
	manager.mu.Lock()
	manager.setActiveAlertNoLock(alert.ID, alert)
	manager.mu.Unlock()

	expiresAt := now.Add(2 * time.Hour)
	if err := manager.SuppressOperationalAlert(
		alert.ID,
		"operator",
		"Planned maintenance",
		&expiresAt,
	); err != nil {
		t.Fatalf("SuppressOperationalAlert() error = %v", err)
	}
	if err := manager.SaveActiveAlerts(); err != nil {
		t.Fatalf("SaveActiveAlerts() error = %v", err)
	}
	manager.Stop()

	reloaded := NewManagerWithDataDir(dataDir)
	t.Cleanup(reloaded.Stop)
	if err := reloaded.LoadActiveAlerts(); err != nil {
		t.Fatalf("LoadActiveAlerts() error = %v", err)
	}
	current := activeAlert(t, reloaded, alert.ID)
	if current.OperationalRecord == nil ||
		current.OperationalRecord.State != operationaltrust.OperationalSuppressed {
		t.Fatalf("reloaded record = %+v", current.OperationalRecord)
	}
	if current.OperationalRecord.Suppression == nil ||
		current.OperationalRecord.Suppression.Reason != "Planned maintenance" {
		t.Fatalf("reloaded suppression = %+v", current.OperationalRecord.Suppression)
	}
	if current.LatestTransition == nil ||
		current.LatestTransition.Cause != operationaltrust.TransitionSuppression {
		t.Fatalf("reloaded transition = %+v", current.LatestTransition)
	}
}

func TestOperationalLifecycleRetriesDoNotDuplicateTransitionsAndRetainNewEvidence(t *testing.T) {
	manager := newTestManager(t)
	now := time.Now().UTC()
	alert := &Alert{
		ID:              "container-health",
		CanonicalState:  "container-health",
		CanonicalSpecID: "docker-container-health",
		Type:            "docker-container-health",
		Level:           AlertLevelWarning,
		ResourceID:      "app-container:web",
		StartTime:       now.Add(-time.Minute),
		LastSeen:        now,
	}
	manager.mu.Lock()
	manager.setActiveAlertNoLock(alert.ID, alert)
	manager.mu.Unlock()

	acknowledgementCallbacks := make(chan struct{}, 2)
	manager.SetAcknowledgedCallback(func(*Alert, string) {
		acknowledgementCallbacks <- struct{}{}
	})
	if err := manager.AcknowledgeAlert(alert.ID, "operator"); err != nil {
		t.Fatalf("first AcknowledgeAlert() error = %v", err)
	}
	select {
	case <-acknowledgementCallbacks:
	case <-time.After(time.Second):
		t.Fatal("first acknowledgement callback did not run")
	}
	firstAcknowledged := activeAlert(t, manager, alert.ID)
	firstAckTime := *firstAcknowledged.AckTime
	if err := manager.AcknowledgeAlert(alert.ID, "operator"); err != nil {
		t.Fatalf("retry AcknowledgeAlert() error = %v", err)
	}
	retriedAcknowledged := activeAlert(t, manager, alert.ID)
	select {
	case <-acknowledgementCallbacks:
		t.Fatal("idempotent acknowledgement retry emitted another callback")
	case <-time.After(50 * time.Millisecond):
	}
	if retriedAcknowledged.AckTime == nil ||
		!retriedAcknowledged.AckTime.Equal(firstAckTime) {
		t.Fatalf(
			"retry changed acknowledgement time from %s to %v",
			firstAckTime,
			retriedAcknowledged.AckTime,
		)
	}

	firstEvidence := operationalStateWriterEvidence(
		t,
		"docker",
		"app-container:web",
		now.Add(time.Second),
		now.Add(2*time.Second),
		operationaltrust.EvidenceUnavailable,
		operationaltrust.EvidencePermissionsSufficient,
		"collector_timeout",
	)
	if err := manager.MarkOperationalCollectionStale(
		alert.ID,
		firstEvidence,
		"Collector timed out.",
	); err != nil {
		t.Fatalf("first stale transition error = %v", err)
	}
	firstStale := activeAlert(t, manager, alert.ID)
	if firstStale.LatestTransition == nil {
		t.Fatal("first stale transition is missing")
	}
	transitionID := firstStale.LatestTransition.ID

	secondEvidence := operationalStateWriterEvidence(
		t,
		"docker",
		"app-container:web",
		now.Add(3*time.Second),
		now.Add(4*time.Second),
		operationaltrust.EvidenceUnavailable,
		operationaltrust.EvidencePermissionsSufficient,
		"collector_timeout",
	)
	if err := manager.MarkOperationalCollectionStale(
		alert.ID,
		secondEvidence,
		"Collector timed out.",
	); err != nil {
		t.Fatalf("retry stale observation error = %v", err)
	}
	refreshed := activeAlert(t, manager, alert.ID)
	if refreshed.LatestTransition == nil ||
		refreshed.LatestTransition.ID != transitionID {
		t.Fatalf(
			"retry created a new transition: first=%q current=%+v",
			transitionID,
			refreshed.LatestTransition,
		)
	}
	retainedIDs := operationalEvidenceIDs(refreshed.Evidence)
	if !slices.Contains(retainedIDs, firstEvidence.ID) ||
		!slices.Contains(retainedIDs, secondEvidence.ID) {
		t.Fatalf(
			"retained evidence IDs = %v, want %q and %q",
			retainedIDs,
			firstEvidence.ID,
			secondEvidence.ID,
		)
	}
	if !refreshed.OperationalRecord.LastObservedAt.Equal(secondEvidence.ObservedAt) {
		t.Fatalf(
			"LastObservedAt = %s, want %s",
			refreshed.OperationalRecord.LastObservedAt,
			secondEvidence.ObservedAt,
		)
	}
}

func operationalStateWriterEvidence(
	t *testing.T,
	provider string,
	resourceID string,
	observedAt time.Time,
	validUntil time.Time,
	completeness operationaltrust.EvidenceCompleteness,
	permissions operationaltrust.EvidencePermissions,
	reasonCode string,
) operationaltrust.EvidenceEnvelope {
	t.Helper()
	source := operationaltrust.EvidenceSource{
		Provider:  provider,
		Collector: provider + "-collector",
	}
	subject := operationaltrust.EvidenceSubject{ResourceID: resourceID}
	id, err := operationaltrust.NewEvidenceID(
		source,
		subject,
		observedAt,
		provider+"-observation",
	)
	if err != nil {
		t.Fatalf("NewEvidenceID() error = %v", err)
	}
	envelope := operationaltrust.EvidenceEnvelope{
		ID:           id,
		Source:       source,
		Subject:      subject,
		ObservedAt:   observedAt,
		IngestedAt:   observedAt,
		ValidUntil:   &validUntil,
		Completeness: completeness,
		Confidence:   operationaltrust.EvidenceConfirmed,
		Permissions:  permissions,
	}
	if reasonCode != "" {
		envelope.Reason = &operationaltrust.EvidenceReason{Code: reasonCode}
	}
	if err := envelope.Validate(); err != nil {
		t.Fatalf("evidence Validate() error = %v", err)
	}
	return envelope
}
