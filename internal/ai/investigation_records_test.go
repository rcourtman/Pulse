package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

func TestBuildFindingInvestigationRecord_FromSession(t *testing.T) {
	detectedAt := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	completedAt := detectedAt.Add(3 * time.Minute)
	finding := &Finding{
		ID:                     "finding-1",
		Key:                    "cpu-high",
		Severity:               FindingSeverityCritical,
		Category:               FindingCategoryPerformance,
		ResourceID:             "vm-100",
		ResourceName:           "web-server",
		ResourceType:           "vm",
		Node:                   "pve-1",
		Title:                  "High CPU",
		Description:            "CPU above threshold",
		Recommendation:         "Reduce CPU pressure",
		Evidence:               "cpu=96%",
		Source:                 "ai-analysis",
		FailureCause:           string(PatrolFailureCauseModelUnsupportedTools),
		DetectedAt:             detectedAt,
		InvestigationSessionID: "chat-1",
		InvestigationStatus:    string(InvestigationStatusCompleted),
		InvestigationOutcome:   string(InvestigationOutcomeFixQueued),
		LastInvestigatedAt:     &completedAt,
	}
	session := &InvestigationSession{
		ID:          "investigation-1",
		FindingID:   "finding-1",
		SessionID:   "chat-1",
		Status:      aicontracts.InvestigationStatusCompleted,
		StartedAt:   detectedAt.Add(time.Minute),
		CompletedAt: &completedAt,
		Outcome:     aicontracts.OutcomeFixQueued,
		ToolsUsed:   []string{"ssh.exec", "ssh.exec", "  "},
		EvidenceIDs: []string{"evidence-1"},
		Summary:     "Postgres was consuming CPU.",
		ApprovalID:  "approval-1",
		ProposedFix: &aicontracts.Fix{
			ID:          "fix-1",
			Description: "Restart postgres",
			Commands:    []string{"systemctl restart postgres"},
			RiskLevel:   "medium",
			TargetHost:  "pve-1",
			Rationale:   "Service is stuck in a busy loop",
		},
	}

	record := BuildFindingInvestigationRecord(finding, session)
	if record == nil {
		t.Fatal("expected investigation record")
	}
	if record.ID != "investigation-1" {
		t.Fatalf("record ID = %q, want investigation-1", record.ID)
	}
	if record.Subject.ResourceID != "vm-100" || record.Subject.Node != "pve-1" {
		t.Fatalf("unexpected subject: %#v", record.Subject)
	}
	if record.Trigger.FindingKey != "cpu-high" || record.Trigger.Title != "High CPU" {
		t.Fatalf("unexpected trigger: %#v", record.Trigger)
	}
	if record.Trigger.Cause != string(PatrolFailureCauseModelUnsupportedTools) {
		t.Fatalf("trigger cause = %q", record.Trigger.Cause)
	}
	if record.Conclusion != "Postgres was consuming CPU." {
		t.Fatalf("conclusion = %q", record.Conclusion)
	}
	if record.RecommendedAction != "Reduce CPU pressure" {
		t.Fatalf("recommended action = %q", record.RecommendedAction)
	}
	if record.Confidence != aicontracts.InvestigationRecordConfidenceMedium {
		t.Fatalf("confidence = %q", record.Confidence)
	}
	if len(record.Evidence) != 2 {
		t.Fatalf("expected finding and investigation evidence, got %#v", record.Evidence)
	}
	if len(record.ToolsUsed) != 1 || record.ToolsUsed[0] != "ssh.exec" {
		t.Fatalf("tools used not normalized: %#v", record.ToolsUsed)
	}
	if record.ProposedFix == nil || record.ProposedFix.RiskLevel != "medium" {
		t.Fatalf("expected proposed fix, got %#v", record.ProposedFix)
	}
	if record.ApprovalID != "approval-1" {
		t.Fatalf("approval ID = %q", record.ApprovalID)
	}
	if record.Impact != "" {
		t.Fatalf("expected empty default impact, got %q", record.Impact)
	}
	if record.Rollback == nil || len(record.Rollback) != 0 {
		t.Fatalf("expected normalized empty rollback slice, got %#v", record.Rollback)
	}
}

func TestEmptyInvestigationRecord_NormalizesRollback(t *testing.T) {
	record := aicontracts.EmptyInvestigationRecord()
	if record.Rollback == nil || len(record.Rollback) != 0 {
		t.Fatalf("expected empty rollback slice on empty record, got %#v", record.Rollback)
	}
}

func TestFindingsStore_UpdateInvestigationRecord(t *testing.T) {
	store := NewFindingsStore()
	store.Add(&Finding{
		ID:         "finding-1",
		Severity:   FindingSeverityWarning,
		Category:   FindingCategoryPerformance,
		ResourceID: "vm-100",
		Title:      "High CPU",
		DetectedAt: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
	})

	record := &aicontracts.InvestigationRecord{
		ID:        "investigation-1",
		FindingID: "finding-1",
		Status:    aicontracts.InvestigationStatusCompleted,
		Outcome:   aicontracts.OutcomeFixVerified,
		Evidence:  []aicontracts.InvestigationRecordEvidence{},
	}
	if !store.UpdateInvestigationRecord("finding-1", record) {
		t.Fatal("UpdateInvestigationRecord failed")
	}
	updated := store.Get("finding-1")
	if updated == nil || updated.InvestigationRecord == nil {
		t.Fatalf("expected persisted investigation record, got %#v", updated)
	}
	if updated.InvestigationRecord.ID != "investigation-1" {
		t.Fatalf("record ID = %q", updated.InvestigationRecord.ID)
	}
	if store.UpdateInvestigationRecord("missing", record) {
		t.Fatal("expected false for missing finding")
	}
}
