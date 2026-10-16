package ai

import (
	"testing"
	"time"
)

func TestShouldInvestigate_FailedTimeoutUsesScheduledCadence(t *testing.T) {
	// Failed timeouts use the scheduled assessment cadence.
	fifteenMinsAgo := time.Now().Add(-15 * time.Minute)
	f := &Finding{
		ID:                    "f1",
		Severity:              FindingSeverityWarning,
		Category:              FindingCategoryPerformance,
		ResourceID:            "r1",
		InvestigationStatus:   string(InvestigationStatusFailed),
		InvestigationOutcome:  string(InvestigationOutcomeTimedOut),
		LastInvestigatedAt:    &fifteenMinsAgo,
		InvestigationAttempts: 1,
	}

	if !f.ShouldInvestigate("approval") {
		t.Fatalf("expected timed-out finding (15min old) to be eligible for re-investigation")
	}

	// A failed run follows the Watch cadence without an additional cooldown.
	fiveMinsAgo := time.Now().Add(-5 * time.Minute)
	f.LastInvestigatedAt = &fiveMinsAgo

	if !f.ShouldInvestigate("approval") {
		t.Fatalf("failed timeout should be eligible at the next scheduled assessment")
	}
}

func TestShouldInvestigate_FailedProviderUsesScheduledCadence(t *testing.T) {
	// Finding with no special outcome (regular failure) at 15 minutes ago
	// remains eligible for the next scheduled assessment.
	fifteenMinsAgo := time.Now().Add(-15 * time.Minute)
	f := &Finding{
		ID:                    "f1",
		Severity:              FindingSeverityWarning,
		Category:              FindingCategoryPerformance,
		ResourceID:            "r1",
		InvestigationStatus:   string(InvestigationStatusFailed),
		InvestigationOutcome:  "", // no special outcome
		LastInvestigatedAt:    &fifteenMinsAgo,
		InvestigationAttempts: 1,
	}

	if !f.ShouldInvestigate("approval") {
		t.Fatalf("failed provider run should not wait another hour")
	}

	// After 2 hours, should be eligible
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	f.LastInvestigatedAt = &twoHoursAgo
	if !f.ShouldInvestigate("approval") {
		t.Fatalf("expected non-timeout failure (2hr old) to be eligible for re-investigation")
	}
}

func TestCanRetryInvestigation_TimeoutUsesShortCooldown(t *testing.T) {
	fifteenMinsAgo := time.Now().Add(-15 * time.Minute)
	f := &Finding{
		ID:                    "f1",
		Severity:              FindingSeverityWarning,
		InvestigationOutcome:  string(InvestigationOutcomeTimedOut),
		LastInvestigatedAt:    &fifteenMinsAgo,
		InvestigationAttempts: 1,
	}

	if !f.CanRetryInvestigation() {
		t.Fatalf("expected timed-out finding (15min old) to allow retry")
	}

	fiveMinsAgo := time.Now().Add(-5 * time.Minute)
	f.LastInvestigatedAt = &fiveMinsAgo

	if f.CanRetryInvestigation() {
		t.Fatalf("expected timed-out finding (5min old) to block retry")
	}
}

func TestCanRetryInvestigation_NonTimeoutKeepsLongCooldown(t *testing.T) {
	fifteenMinsAgo := time.Now().Add(-15 * time.Minute)
	f := &Finding{
		ID:                    "f1",
		Severity:              FindingSeverityWarning,
		InvestigationOutcome:  "",
		LastInvestigatedAt:    &fifteenMinsAgo,
		InvestigationAttempts: 1,
	}

	if f.CanRetryInvestigation() {
		t.Fatalf("expected non-timeout failure (15min old) to block retry")
	}
}
