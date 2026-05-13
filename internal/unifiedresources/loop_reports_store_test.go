package unifiedresources

import (
	"testing"
	"time"
)

func newLoopReport(id, scope string, windowEnd time.Time, status LoopReportStatus) LoopReport {
	return LoopReport{
		ID:            id,
		Type:          LoopReportTypeMaintenanceVerification,
		Scope:         scope,
		Trigger:       "maintenance_window_end",
		Goal:          "verify recovery",
		Status:        status,
		StartedAt:     windowEnd.Add(time.Minute),
		CompletedAt:   windowEnd.Add(time.Minute),
		WindowEndedAt: &windowEnd,
		Evidence:      LoopReportEvidence{OperatorStateSummary: "maintenance window ended"},
	}
}

func TestSQLiteRecordLoopReport_RoundTrip(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "default")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	report := newLoopReport("mv-vm_101-20260512T120000Z", "vm:101", windowEnd, LoopReportStatusHealthy)
	if err := store.RecordLoopReport(report); err != nil {
		t.Fatalf("record: %v", err)
	}
	got, found, err := store.GetLoopReport(report.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !found {
		t.Fatal("expected report to be found after record")
	}
	if got.Scope != "vm:101" || got.Status != LoopReportStatusHealthy {
		t.Fatalf("round trip mismatch: scope=%q status=%q", got.Scope, got.Status)
	}
}

func TestLoopReport_RecordMaintenanceVerificationResourceChange(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "default")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	windowStart := time.Date(2026, 5, 12, 11, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	report := newLoopReport("mv-vm_101-20260512T120000Z", "vm:101", windowEnd, LoopReportStatusNeedsReview)
	report.WindowStartedAt = &windowStart
	report.LinkedAlertIDs = []string{"alert-1"}
	report.Evidence.ActiveWarningAlerts = 1
	if err := store.RecordLoopReport(report); err != nil {
		t.Fatalf("record: %v", err)
	}

	changes, err := store.GetRecentChanges("vm:101", windowEnd.Add(-time.Minute), 0)
	if err != nil {
		t.Fatalf("recent changes: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected one resource change for the loop report, got %d", len(changes))
	}
	change := changes[0]
	if change.Kind != ChangeActivity {
		t.Fatalf("change kind = %q want activity", change.Kind)
	}
	if change.SourceType != SourceHeuristic {
		t.Fatalf("source type = %q want heuristic", change.SourceType)
	}
	if change.SourceAdapter != maintenanceVerificationSourceAdapter {
		t.Fatalf("source adapter = %q want maintenance_sentinel", change.SourceAdapter)
	}
	if change.To != string(LoopReportStatusNeedsReview) {
		t.Fatalf("change to = %q want needs_review", change.To)
	}
	if change.OccurredAt == nil || !change.OccurredAt.Equal(windowEnd) {
		t.Fatalf("occurredAt = %v want %v", change.OccurredAt, windowEnd)
	}
	if change.Metadata["activityType"] != MaintenanceVerificationActivityReported {
		t.Fatalf("activityType = %#v want maintenance verification reported", change.Metadata["activityType"])
	}
	if change.Metadata["reportId"] != report.ID {
		t.Fatalf("reportId = %#v want %q", change.Metadata["reportId"], report.ID)
	}
	if change.Metadata["linkedAlertCount"] != float64(1) {
		t.Fatalf("linkedAlertCount = %#v want 1", change.Metadata["linkedAlertCount"])
	}
}

func TestSQLiteRecordLoopReport_AllowsRerunForSameWindow(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "default")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	original := newLoopReport("mv-vm_101-20260512T120000Z", "vm:101", windowEnd, LoopReportStatusHealthy)
	if err := store.RecordLoopReport(original); err != nil {
		t.Fatalf("record original: %v", err)
	}
	rerun := newLoopReport(original.ID+"-rerun-1", "vm:101", windowEnd, LoopReportStatusNeedsReview)
	if err := store.RecordLoopReport(rerun); err != nil {
		t.Fatalf("record rerun: %v", err)
	}

	reports, err := store.ListLoopReportsForResource(LoopReportTypeMaintenanceVerification, "vm:101", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports (original + rerun), got %d", len(reports))
	}

	found, ok, err := store.FindLoopReportByWindow(LoopReportTypeMaintenanceVerification, "vm:101", windowEnd)
	if err != nil || !ok {
		t.Fatalf("find by window: ok=%v err=%v", ok, err)
	}
	if found.ID == "" {
		t.Fatal("expected matching report id from FindLoopReportByWindow")
	}
}

func TestSQLiteRecordLoopReport_RejectsDuplicateID(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "default")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	report := newLoopReport("mv-dup-id", "vm:101", windowEnd, LoopReportStatusHealthy)
	if err := store.RecordLoopReport(report); err != nil {
		t.Fatalf("first record: %v", err)
	}
	if err := store.RecordLoopReport(report); err == nil {
		t.Fatal("expected error on duplicate id")
	}
}

func TestSQLiteUpdateLoopReportUserOutcome_RoundTrip(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "default")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	report := newLoopReport("mv-review", "vm:101", windowEnd, LoopReportStatusNeedsReview)
	if err := store.RecordLoopReport(report); err != nil {
		t.Fatalf("record: %v", err)
	}
	reviewedAt := time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC)
	if err := store.UpdateLoopReportUserOutcome(report.ID, LoopReportUserOutcomeReviewed, "rcourtman", "ack", reviewedAt); err != nil {
		t.Fatalf("update outcome: %v", err)
	}
	got, _, err := store.GetLoopReport(report.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.UserOutcome != LoopReportUserOutcomeReviewed {
		t.Fatalf("outcome = %q want reviewed", got.UserOutcome)
	}
	if got.ReviewedBy != "rcourtman" || got.ReviewNote != "ack" {
		t.Fatalf("review fields = %q/%q want rcourtman/ack", got.ReviewedBy, got.ReviewNote)
	}
	if got.ReviewedAt == nil || !got.ReviewedAt.Equal(reviewedAt) {
		t.Fatalf("reviewedAt = %v want %v", got.ReviewedAt, reviewedAt)
	}
	if got.Status != LoopReportStatusNeedsReview {
		t.Fatalf("status mutated to %q; should remain needs_review", got.Status)
	}
}

func TestMemoryStoreRecordLoopReport_AllowsRerun(t *testing.T) {
	store := NewMemoryStore()
	windowEnd := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	original := newLoopReport("mv-mem-1", "vm:101", windowEnd, LoopReportStatusHealthy)
	if err := store.RecordLoopReport(original); err != nil {
		t.Fatalf("record original: %v", err)
	}
	rerun := newLoopReport("mv-mem-1-rerun-1", "vm:101", windowEnd, LoopReportStatusNeedsReview)
	if err := store.RecordLoopReport(rerun); err != nil {
		t.Fatalf("record rerun: %v", err)
	}
	reports, err := store.ListLoopReportsForResource(LoopReportTypeMaintenanceVerification, "vm:101", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}
}

func TestSQLiteListResourceOperatorStates_ReturnsAllRows(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewSQLiteResourceStore(dataDir, "default")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	startA := now.Add(-time.Hour)
	endA := now.Add(-time.Minute)
	startB := now.Add(-30 * time.Minute)
	endB := now.Add(30 * time.Minute)

	for _, s := range []ResourceOperatorState{
		{CanonicalID: "vm:101", MaintenanceStartAt: &startA, MaintenanceEndAt: &endA, SetAt: now, SetBy: "op"},
		{CanonicalID: "vm:102", MaintenanceStartAt: &startB, MaintenanceEndAt: &endB, SetAt: now, SetBy: "op"},
	} {
		if err := store.SetResourceOperatorState(s); err != nil {
			t.Fatalf("seed %s: %v", s.CanonicalID, err)
		}
	}
	got, err := store.ListResourceOperatorStates()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 operator states, got %d", len(got))
	}
}
