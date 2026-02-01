package ai

import (
	"testing"
	"time"
)

func TestShouldInvestigate_TimeoutUsesShortCooldown(t *testing.T) {
	// Finding with timed_out outcome and LastInvestigatedAt 15 minutes ago
	// should be eligible for re-investigation (shorter 10min cooldown)
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

	// Same finding but only 5 minutes ago — still within the 10min timeout cooldown
	fiveMinsAgo := time.Now().Add(-5 * time.Minute)
	f.LastInvestigatedAt = &fiveMinsAgo

	if f.ShouldInvestigate("approval") {
		t.Fatalf("expected timed-out finding (5min old) to still be in cooldown")
	}
}

func TestShouldInvestigate_NonTimeoutKeepsLongCooldown(t *testing.T) {
	// Finding with no special outcome (regular failure) at 15 minutes ago
	// should NOT be eligible — still within the 1-hour cooldown
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

	if f.ShouldInvestigate("approval") {
		t.Fatalf("expected non-timeout failure (15min old) to still be in 1hr cooldown")
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
