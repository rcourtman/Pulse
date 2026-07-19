package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	recoverymodel "github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
)

func TestProjectAttentionItemsUsesCanonicalLifecycleAndStableOrdering(t *testing.T) {
	now := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	active := []alerts.Alert{
		attentionTestAlert("warning-old", operationaltrust.OperationalOpen, operationaltrust.SeverityWarning, now.Add(-2*time.Hour), now),
		attentionTestAlert("critical", operationaltrust.OperationalOpen, operationaltrust.SeverityCritical, now.Add(-time.Hour), now),
		attentionTestAlert("warning-protected", operationaltrust.OperationalOpen, operationaltrust.SeverityWarning, now.Add(-3*time.Hour), now),
	}
	postures := map[string]recoverymodel.ProtectionPosture{
		"resource-warning-old": attentionTestPosture("resource-warning-old", recoverymodel.ProtectionStateUnprotected, now),
		"resource-warning-protected": attentionTestPosture(
			"resource-warning-protected",
			recoverymodel.ProtectionStateProtected,
			now,
		),
	}

	projected := ProjectAttentionItems(active, nil, postures, now)
	if projected.Summary.ActiveCount != 3 {
		t.Fatalf("ActiveCount = %d, want 3", projected.Summary.ActiveCount)
	}
	if projected.Summary.Calm {
		t.Fatal("active projection reported a calm state")
	}
	if len(projected.Details) != 3 {
		t.Fatalf("details = %d, want 3", len(projected.Details))
	}
	got := []string{
		projected.Details[0].Item.ID,
		projected.Details[1].Item.ID,
		projected.Details[2].Item.ID,
	}
	want := []string{"critical", "warning-old", "warning-protected"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
	if projected.Details[1].Item.OperationalRecordID != "warning-old" {
		t.Fatal("attention item did not retain canonical operational record identity")
	}
	if projected.Details[1].Item.ProtectionPosture == nil ||
		projected.Details[1].Item.ProtectionPosture.State != recoverymodel.ProtectionStateUnprotected {
		t.Fatal("attention item did not receive canonical protection posture")
	}
}

func TestProjectAttentionItemsDeduplicatesHistoryAndActiveByOperationalRecord(t *testing.T) {
	now := time.Date(2026, 7, 19, 2, 0, 0, 0, time.UTC)
	historical := attentionTestAlert(
		"shared-record",
		operationaltrust.OperationalResolved,
		operationaltrust.SeverityWarning,
		now.Add(-3*time.Hour),
		now.Add(-time.Hour),
	)
	resolvedAt := now.Add(-time.Hour)
	historical.OperationalRecord.ResolvedAt = &resolvedAt
	active := attentionTestAlert(
		"shared-record",
		operationaltrust.OperationalOpen,
		operationaltrust.SeverityWarning,
		now.Add(-3*time.Hour),
		now,
	)

	projected := ProjectAttentionItems([]alerts.Alert{active}, []alerts.Alert{historical}, nil, now)
	if len(projected.Details) != 1 {
		t.Fatalf("details = %d, want one canonical item", len(projected.Details))
	}
	if projected.Details[0].Item.State != operationaltrust.OperationalOpen {
		t.Fatalf("state = %q, want active lifecycle state", projected.Details[0].Item.State)
	}
}

func TestProjectAttentionItemsDoesNotReviveNonTerminalHistorySnapshots(t *testing.T) {
	now := time.Date(2026, 7, 19, 2, 30, 0, 0, time.UTC)
	orphanedOpen := attentionTestAlert(
		"historical-open",
		operationaltrust.OperationalOpen,
		operationaltrust.SeverityCritical,
		now.Add(-3*time.Hour),
		now.Add(-2*time.Hour),
	)
	resolved := attentionTestAlert(
		"historical-resolved",
		operationaltrust.OperationalResolved,
		operationaltrust.SeverityWarning,
		now.Add(-4*time.Hour),
		now.Add(-time.Hour),
	)
	resolvedAt := now.Add(-time.Hour)
	resolved.OperationalRecord.ResolvedAt = &resolvedAt

	projected := ProjectAttentionItems(
		nil,
		[]alerts.Alert{orphanedOpen, resolved},
		nil,
		now,
	)
	if len(projected.Details) != 1 {
		t.Fatalf("details = %d, want only the resolved historical record", len(projected.Details))
	}
	if got := projected.Details[0].Item.ID; got != "historical-resolved" {
		t.Fatalf("historical item = %q, want resolved record only", got)
	}
	if projected.Summary.ActiveCount != 0 || !projected.Summary.Calm {
		t.Fatalf("summary = %+v, historical open snapshot revived active work", projected.Summary)
	}
}

func TestProjectAttentionItemsRepresentsUncertainEvidenceHonestly(t *testing.T) {
	now := time.Date(2026, 7, 19, 3, 0, 0, 0, time.UTC)
	stale := attentionTestAlert(
		"stale",
		operationaltrust.OperationalStale,
		operationaltrust.SeverityCritical,
		now.Add(-time.Hour),
		now,
	)
	stale.Evidence = nil
	unknown := attentionTestAlert(
		"unknown",
		operationaltrust.OperationalUnknown,
		operationaltrust.SeverityWarning,
		now.Add(-time.Hour),
		now,
	)
	unknown.Evidence = nil

	projected := ProjectAttentionItems([]alerts.Alert{stale, unknown}, nil, nil, now)
	if projected.Summary.UncertainCount != 2 || projected.Summary.ActiveCount != 2 {
		t.Fatalf("summary = %+v, want two uncertain active items", projected.Summary)
	}
	if projected.Details[0].Item.EvidenceFreshness != operationaltrust.EvidenceStale {
		t.Fatalf(
			"stale freshness = %q",
			projected.Details[0].Item.EvidenceFreshness,
		)
	}
	if projected.Details[1].Item.EvidenceFreshness != operationaltrust.EvidenceFreshnessUnknown {
		t.Fatalf(
			"unknown freshness = %q",
			projected.Details[1].Item.EvidenceFreshness,
		)
	}
	for _, detail := range projected.Details {
		if detail.Item.EvidenceCompleteness != operationaltrust.EvidenceUnavailable {
			t.Fatalf("completeness = %q, want unavailable", detail.Item.EvidenceCompleteness)
		}
	}
}

func TestAttentionFiltersAndPaginationCoverLifecycleStates(t *testing.T) {
	now := time.Date(2026, 7, 19, 4, 0, 0, 0, time.UTC)
	states := []operationaltrust.OperationalState{
		operationaltrust.OperationalOpen,
		operationaltrust.OperationalAcknowledged,
		operationaltrust.OperationalSuppressed,
		operationaltrust.OperationalStale,
		operationaltrust.OperationalUnknown,
		operationaltrust.OperationalResolved,
	}
	active := make([]alerts.Alert, 0, len(states)-1)
	history := make([]alerts.Alert, 0, 1)
	for index, state := range states {
		alert := attentionTestAlert(
			string(state),
			state,
			operationaltrust.SeverityWarning,
			now.Add(-time.Duration(index+1)*time.Minute),
			now,
		)
		switch state {
		case operationaltrust.OperationalAcknowledged:
			alert.OperationalRecord.Acknowledgement = &operationaltrust.Acknowledgement{
				At: now,
				By: "operator",
			}
		case operationaltrust.OperationalSuppressed:
			alert.OperationalRecord.Suppression = &operationaltrust.Suppression{
				At:     now,
				By:     "operator",
				Reason: "maintenance",
			}
		case operationaltrust.OperationalResolved:
			resolvedAt := now
			alert.OperationalRecord.ResolvedAt = &resolvedAt
			history = append(history, alert)
			continue
		}
		active = append(active, alert)
	}

	projected := ProjectAttentionItems(active, history, nil, now)
	checks := map[AttentionFilter]int{
		AttentionFilterActive:       3,
		AttentionFilterOpen:         1,
		AttentionFilterAcknowledged: 1,
		AttentionFilterSuppressed:   1,
		AttentionFilterUncertain:    2,
		AttentionFilterResolved:     1,
		AttentionFilterAll:          6,
	}
	for filter, want := range checks {
		got, err := FilterAttentionDetails(projected.Details, filter)
		if err != nil {
			t.Fatalf("FilterAttentionDetails(%q): %v", filter, err)
		}
		if len(got) != want {
			t.Fatalf("FilterAttentionDetails(%q) = %d, want %d", filter, len(got), want)
		}
	}
	page, err := PaginateAttentionDetails(projected.Details, 2, 2)
	if err != nil {
		t.Fatalf("PaginateAttentionDetails(): %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("page length = %d, want 2", len(page))
	}
}

func TestProjectAttentionItemsProducesCurrentCalmEvaluation(t *testing.T) {
	now := time.Date(2026, 7, 19, 5, 0, 0, 0, time.UTC)
	projected := ProjectAttentionItems(nil, nil, nil, now)
	if !projected.Summary.Calm || projected.Summary.ActiveCount != 0 {
		t.Fatalf("summary = %+v, want current calm evaluation", projected.Summary)
	}
	if projected.Summary.CoverageState != "current" ||
		!projected.Summary.EvaluatedAt.Equal(now) {
		t.Fatalf("summary = %+v, want bounded current evaluation", projected.Summary)
	}
}

func attentionTestAlert(
	id string,
	state operationaltrust.OperationalState,
	severity operationaltrust.OperationalSeverity,
	firstObservedAt time.Time,
	lastObservedAt time.Time,
) alerts.Alert {
	resourceID := "resource-" + id
	evidence := attentionTestEvidence(resourceID, lastObservedAt)
	record := operationaltrust.OperationalRecord{
		ID:                  id,
		CanonicalSpecID:     "spec-" + id,
		SubjectResourceID:   resourceID,
		State:               state,
		Severity:            severity,
		FirstObservedAt:     firstObservedAt,
		LastObservedAt:      lastObservedAt,
		StateChangedAt:      lastObservedAt,
		EvidenceIDs:         []string{evidence.ID},
		CauseKey:            "cause-" + id,
		RelatedResourceIDs:  []string{"related-" + id},
		ImpactSummary:       "Service may be interrupted.",
		RecommendedNextStep: "Review the source evidence.",
	}
	return alerts.Alert{
		ID:                "alert-" + id,
		Type:              "service-health",
		Level:             alerts.AlertLevelWarning,
		ResourceID:        resourceID,
		ResourceName:      "Resource " + id,
		Message:           "The service health check needs attention.",
		StartTime:         firstObservedAt,
		LastSeen:          lastObservedAt,
		OperationalRecord: &record,
		Evidence:          []operationaltrust.EvidenceEnvelope{evidence},
	}
}

func attentionTestEvidence(
	resourceID string,
	observedAt time.Time,
) operationaltrust.EvidenceEnvelope {
	validUntil := observedAt.Add(time.Hour)
	return operationaltrust.EvidenceEnvelope{
		ID: "evidence-" + resourceID,
		Source: operationaltrust.EvidenceSource{
			Provider:  "test",
			Collector: "test-collector",
		},
		Subject:      operationaltrust.EvidenceSubject{ResourceID: resourceID},
		ObservedAt:   observedAt,
		IngestedAt:   observedAt,
		ValidUntil:   &validUntil,
		Completeness: operationaltrust.EvidenceComplete,
		Confidence:   operationaltrust.EvidenceConfirmed,
		Permissions:  operationaltrust.EvidencePermissionsSufficient,
	}
}

func attentionTestPosture(
	resourceID string,
	state recoverymodel.ProtectionState,
	evaluatedAt time.Time,
) recoverymodel.ProtectionPosture {
	return recoverymodel.ProtectionPosture{
		SubjectResourceID: resourceID,
		State:             state,
		Freshness:         recoverymodel.ProtectionFreshnessCurrent,
		Verification:      recoverymodel.ProtectionVerificationVerified,
		Coverage:          recoverymodel.ProtectionCoverageComplete,
		ProviderStates:    []recoverymodel.ProtectionProviderState{},
		EvidenceIDs:       []string{},
		Explanation:       "Test posture.",
		EvaluatedAt:       evaluatedAt,
	}
}
