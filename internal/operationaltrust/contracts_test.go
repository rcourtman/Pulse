package operationaltrust

import (
	"strings"
	"testing"
	"time"
)

func TestEvidenceEnvelopeRequiresTypedLimitations(t *testing.T) {
	observedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	validUntil := observedAt.Add(15 * time.Minute)
	source := EvidenceSource{
		Provider:  "proxmox",
		Collector: "pulse-core",
		Instance:  "pve-a",
	}
	subject := EvidenceSubject{ResourceID: "resource-123"}
	id, err := NewEvidenceID(source, subject, observedAt, "provider-event-7")
	if err != nil {
		t.Fatalf("NewEvidenceID() error = %v", err)
	}

	envelope := EvidenceEnvelope{
		ID:           id,
		Source:       source,
		Subject:      subject,
		ObservedAt:   observedAt,
		IngestedAt:   observedAt.Add(time.Second),
		ValidUntil:   &validUntil,
		Completeness: EvidencePartial,
		Confidence:   EvidenceConfirmed,
		Permissions:  EvidencePermissionsPartial,
	}
	if err := envelope.Validate(); err == nil || !strings.Contains(err.Error(), "typed reason") {
		t.Fatalf("Validate() error = %v, want typed reason error", err)
	}

	envelope.Reason = &EvidenceReason{Code: "permission_scope_incomplete"}
	if err := envelope.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got := envelope.FreshnessAt(validUntil); got != EvidenceFresh {
		t.Fatalf("FreshnessAt(validUntil) = %q, want %q", got, EvidenceFresh)
	}
	if got := envelope.FreshnessAt(validUntil.Add(time.Nanosecond)); got != EvidenceStale {
		t.Fatalf("FreshnessAt(expired) = %q, want %q", got, EvidenceStale)
	}
}

func TestContractClonesDoNotShareMutableState(t *testing.T) {
	validUntil := time.Date(2026, 7, 18, 21, 0, 0, 0, time.UTC)
	envelope := EvidenceEnvelope{
		ValidUntil: &validUntil,
		Reason:     &EvidenceReason{Code: "inferred"},
		Correlation: &IdentityCorrelation{
			MatchedFields: map[string]string{"hostname": "db-01"},
		},
	}
	envelopeClone := envelope.Clone()
	envelopeClone.Correlation.MatchedFields["hostname"] = "db-02"
	*envelopeClone.ValidUntil = validUntil.Add(time.Hour)
	if envelope.Correlation.MatchedFields["hostname"] != "db-01" {
		t.Fatal("evidence clone mutated source correlation")
	}
	if !envelope.ValidUntil.Equal(validUntil) {
		t.Fatal("evidence clone mutated source validity")
	}

	record := OperationalRecord{
		EvidenceIDs:        []string{"evidence-1"},
		RelatedResourceIDs: []string{"resource-2"},
		Suppression: &Suppression{
			ExpiresAt: &validUntil,
		},
	}
	recordClone := record.Clone()
	recordClone.EvidenceIDs[0] = "evidence-2"
	recordClone.RelatedResourceIDs[0] = "resource-3"
	*recordClone.Suppression.ExpiresAt = validUntil.Add(time.Hour)
	if record.EvidenceIDs[0] != "evidence-1" ||
		record.RelatedResourceIDs[0] != "resource-2" ||
		!record.Suppression.ExpiresAt.Equal(validUntil) {
		t.Fatal("operational record clone shared mutable state")
	}
}

func TestEvidenceEnvelopeDistinguishesDeniedFromUnavailable(t *testing.T) {
	observedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	source := EvidenceSource{Provider: "pbs", Collector: "recovery-adapter"}
	subject := EvidenceSubject{
		ProviderRef:   "vm/100",
		ProviderScope: "pbs-a",
	}
	id, err := NewEvidenceID(source, subject, observedAt, "permission-denied")
	if err != nil {
		t.Fatalf("NewEvidenceID() error = %v", err)
	}

	envelope := EvidenceEnvelope{
		ID:           id,
		Source:       source,
		Subject:      subject,
		ObservedAt:   observedAt,
		IngestedAt:   observedAt.Add(time.Second),
		Completeness: EvidenceUnavailable,
		Confidence:   EvidenceUnknown,
		Permissions:  EvidencePermissionsDenied,
		Reason:       &EvidenceReason{Code: "provider_permission_denied"},
	}
	if err := envelope.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got := envelope.FreshnessAt(observedAt.Add(time.Hour)); got != EvidenceFreshnessUnknown {
		t.Fatalf("FreshnessAt() = %q, want %q", got, EvidenceFreshnessUnknown)
	}
}

func TestInferredEvidenceRequiresAuditableSingleMatch(t *testing.T) {
	observedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	source := EvidenceSource{Provider: "uptime-kuma", Collector: "availability-adapter"}
	subject := EvidenceSubject{ResourceID: "resource-123"}
	id, err := NewEvidenceID(source, subject, observedAt, "check-42")
	if err != nil {
		t.Fatalf("NewEvidenceID() error = %v", err)
	}
	envelope := EvidenceEnvelope{
		ID:           id,
		Source:       source,
		Subject:      subject,
		ObservedAt:   observedAt,
		IngestedAt:   observedAt,
		Completeness: EvidenceComplete,
		Confidence:   EvidenceInferred,
		Permissions:  EvidencePermissionsSufficient,
		Reason:       &EvidenceReason{Code: "secondary_identity_match"},
	}
	if err := envelope.Validate(); err == nil || !strings.Contains(err.Error(), "correlation") {
		t.Fatalf("Validate() error = %v, want correlation error", err)
	}

	envelope.Correlation = &IdentityCorrelation{
		Rule:           "normalized_hostname",
		MatchedFields:  map[string]string{"hostname": "db-01.example.test"},
		CandidateCount: 2,
	}
	if err := envelope.Validate(); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("Validate() error = %v, want ambiguous correlation error", err)
	}

	envelope.Correlation.CandidateCount = 1
	if err := envelope.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestEvidenceAndTransitionIDsAreStableUnderRetry(t *testing.T) {
	observedAt := time.Date(2026, 7, 18, 20, 0, 0, 123, time.UTC)
	source := EvidenceSource{Provider: "proxmox", Collector: "pulse-core", Instance: "pve-a"}
	subject := EvidenceSubject{ResourceID: "resource-123"}
	firstEvidenceID, err := NewEvidenceID(source, subject, observedAt, "event-7")
	if err != nil {
		t.Fatalf("NewEvidenceID() error = %v", err)
	}
	secondEvidenceID, err := NewEvidenceID(source, subject, observedAt, "event-7")
	if err != nil {
		t.Fatalf("NewEvidenceID() retry error = %v", err)
	}
	if firstEvidenceID != secondEvidenceID {
		t.Fatalf("evidence retry id = %q, want %q", secondEvidenceID, firstEvidenceID)
	}

	firstTransitionID, err := NewTransitionID(
		"record-1",
		OperationalObserving,
		OperationalOpen,
		observedAt,
		TransitionDetectorDecision,
		"cpu-high",
		[]string{"evidence-b", firstEvidenceID, "evidence-b"},
	)
	if err != nil {
		t.Fatalf("NewTransitionID() error = %v", err)
	}
	secondTransitionID, err := NewTransitionID(
		"record-1",
		OperationalObserving,
		OperationalOpen,
		observedAt,
		TransitionDetectorDecision,
		"cpu-high",
		[]string{firstEvidenceID, "evidence-b"},
	)
	if err != nil {
		t.Fatalf("NewTransitionID() retry error = %v", err)
	}
	if firstTransitionID != secondTransitionID {
		t.Fatalf("transition retry id = %q, want %q", secondTransitionID, firstTransitionID)
	}
}

func TestLifecycleTransitionEnforcesOperationalSemantics(t *testing.T) {
	at := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	if _, err := NewTransitionID(
		"record-1",
		OperationalOpen,
		OperationalResolved,
		at,
		TransitionAcknowledgement,
		"cpu-high",
		nil,
	); err == nil || !strings.Contains(err.Error(), "acknowledged") {
		t.Fatalf("acknowledgement-to-resolution error = %v", err)
	}
	if _, err := NewTransitionID(
		"record-1",
		OperationalResolving,
		OperationalResolved,
		at,
		TransitionRecoveryEvidence,
		"cpu-high",
		nil,
	); err == nil || !strings.Contains(err.Error(), "requires evidence") {
		t.Fatalf("evidenceless resolution error = %v", err)
	}
	if _, err := NewTransitionID(
		"record-1",
		OperationalUnknown,
		OperationalResolved,
		at,
		TransitionCollectionUnknown,
		"cpu-high",
		[]string{"absence-only"},
	); err == nil || !strings.Contains(err.Error(), "unknown state") {
		t.Fatalf("unknown-to-resolution error = %v", err)
	}
}

func TestOperationalRecordRequiresResolutionEvidence(t *testing.T) {
	firstObserved := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	resolvedAt := firstObserved.Add(5 * time.Minute)
	record := OperationalRecord{
		ID:                "record-1",
		CanonicalSpecID:   "metric-threshold:cpu",
		SubjectResourceID: "resource-123",
		State:             OperationalResolved,
		Severity:          SeverityWarning,
		FirstObservedAt:   firstObserved,
		LastObservedAt:    resolvedAt,
		StateChangedAt:    resolvedAt,
		ResolvedAt:        &resolvedAt,
		CauseKey:          "cpu-high",
	}
	if err := record.Validate(); err == nil || !strings.Contains(err.Error(), "decisive evidence") {
		t.Fatalf("Validate() error = %v, want decisive evidence error", err)
	}
	record.EvidenceIDs = []string{"evidence-recovery"}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestNotificationLinkRequiresTransitionAndConsistentTiming(t *testing.T) {
	attemptedAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	completedAt := attemptedAt.Add(-time.Second)
	link := NotificationLink{
		NotificationID:      "notification-1",
		OperationalRecordID: "record-1",
		TransitionID:        "transition-1",
		LifecycleState:      OperationalOpen,
		CauseKey:            "cpu-high",
		DestinationID:       "email-primary",
		DeliveryState:       NotificationFailed,
		AttemptedAt:         &attemptedAt,
		CompletedAt:         &completedAt,
	}
	if err := link.Validate(); err == nil || !strings.Contains(err.Error(), "precedes") {
		t.Fatalf("Validate() error = %v, want timing error", err)
	}
	completedAt = attemptedAt.Add(time.Second)
	if err := link.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestNewNotificationIDIsStableAcrossGroupedTransitionOrder(t *testing.T) {
	createdAt := time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC)
	first, err := NewNotificationID(
		"destination-1",
		"email",
		createdAt,
		[]string{"transition-b", "transition-a"},
	)
	if err != nil {
		t.Fatalf("NewNotificationID() error = %v", err)
	}
	second, err := NewNotificationID(
		"destination-1",
		"email",
		createdAt,
		[]string{"transition-a", "transition-b", "transition-a"},
	)
	if err != nil {
		t.Fatalf("NewNotificationID() retry error = %v", err)
	}
	if first != second {
		t.Fatalf("notification ids = %q and %q, want stable id", first, second)
	}
}
