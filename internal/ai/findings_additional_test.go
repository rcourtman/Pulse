package ai

import (
	"testing"
	"time"
)

func TestFinding_ShouldInvestigate(t *testing.T) {
	now := time.Now()
	old := now.Add(-2 * time.Hour)

	base := &Finding{
		ID:         "f1",
		Severity:   FindingSeverityWarning,
		Category:   FindingCategoryPerformance,
		ResourceID: "r1",
	}

	if !base.ShouldInvestigate("approval") {
		t.Fatalf("expected base finding to be investigated")
	}

	if base.ShouldInvestigate("") {
		t.Fatalf("expected autonomy disabled to skip investigation")
	}

	base.ResolvedAt = &now
	if base.ShouldInvestigate("approval") {
		t.Fatalf("expected resolved finding to skip investigation")
	}
	base.ResolvedAt = nil

	base.Suppressed = true
	if base.ShouldInvestigate("approval") {
		t.Fatalf("expected suppressed finding to skip investigation")
	}
	base.Suppressed = false

	base.DismissedReason = "not_an_issue"
	if base.ShouldInvestigate("approval") {
		t.Fatalf("expected dismissed finding to skip investigation")
	}
	base.DismissedReason = ""

	future := now.Add(time.Minute)
	base.SnoozedUntil = &future
	if base.ShouldInvestigate("approval") {
		t.Fatalf("expected snoozed finding to skip investigation")
	}
	base.SnoozedUntil = nil

	base.Severity = FindingSeverityInfo
	if base.ShouldInvestigate("approval") {
		t.Fatalf("expected info severity to skip investigation")
	}
	base.Severity = FindingSeverityWarning

	base.InvestigationStatus = string(InvestigationStatusRunning)
	if base.ShouldInvestigate("approval") {
		t.Fatalf("expected running investigation to skip")
	}
	base.InvestigationStatus = ""

	base.InvestigationAttempts = 3
	if base.ShouldInvestigate("approval") {
		t.Fatalf("expected max attempts to skip")
	}
	base.InvestigationAttempts = 0

	base.LastInvestigatedAt = &now
	if base.ShouldInvestigate("approval") {
		t.Fatalf("expected cooldown to skip")
	}
	base.LastInvestigatedAt = &old
	if !base.ShouldInvestigate("approval") {
		t.Fatalf("expected investigation after cooldown")
	}
}

func TestFindingInvestigationHelpers(t *testing.T) {
	f := &Finding{}
	f.InvestigationStatus = string(InvestigationStatusRunning)
	if !f.IsBeingInvestigated() {
		t.Fatalf("expected IsBeingInvestigated true")
	}

	f.InvestigationAttempts = 2
	old := time.Now().Add(-2 * time.Hour)
	f.LastInvestigatedAt = &old
	f.InvestigationStatus = ""
	if !f.CanRetryInvestigation() {
		t.Fatalf("expected retry to be allowed")
	}

	f.InvestigationAttempts = 3
	if f.CanRetryInvestigation() {
		t.Fatalf("expected retry blocked by max attempts")
	}
}

func TestFinding_Getters(t *testing.T) {
	ts := time.Now()
	f := &Finding{
		ID:                     "f1",
		Severity:               FindingSeverityCritical,
		Category:               FindingCategoryBackup,
		ResourceID:             "r1",
		ResourceName:           "db-1",
		ResourceType:           "vm",
		Title:                  "Backup missing",
		Description:            "no backups",
		Recommendation:         "configure backups",
		Evidence:               "pbs: none",
		InvestigationSessionID: "sess-1",
		InvestigationStatus:    string(InvestigationStatusFailed),
		InvestigationOutcome:   string(InvestigationOutcomeCannotFix),
		LastInvestigatedAt:     &ts,
		InvestigationAttempts:  2,
	}

	if f.GetID() != "f1" || f.GetSeverity() != string(FindingSeverityCritical) || f.GetCategory() != string(FindingCategoryBackup) {
		t.Fatalf("unexpected basic getters")
	}
	if f.GetResourceID() != "r1" || f.GetResourceName() != "db-1" || f.GetResourceType() != "vm" {
		t.Fatalf("unexpected resource getters")
	}
	if f.GetTitle() != "Backup missing" || f.GetDescription() != "no backups" {
		t.Fatalf("unexpected title/description getters")
	}
	if f.GetRecommendation() != "configure backups" || f.GetEvidence() != "pbs: none" {
		t.Fatalf("unexpected recommendation/evidence getters")
	}
	if f.GetInvestigationSessionID() != "sess-1" || f.GetInvestigationStatus() != string(InvestigationStatusFailed) {
		t.Fatalf("unexpected investigation getters")
	}
	if f.GetInvestigationOutcome() != string(InvestigationOutcomeCannotFix) || f.GetInvestigationAttempts() != 2 {
		t.Fatalf("unexpected investigation outcome/attempts")
	}
	if f.GetLastInvestigatedAt() == nil {
		t.Fatalf("expected last investigated timestamp")
	}
}
