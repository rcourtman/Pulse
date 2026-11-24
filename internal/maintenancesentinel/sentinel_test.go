package maintenancesentinel

import (
	"context"
	"strings"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestSentinelTickOnceWritesReportAndDedupes(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-time.Hour)
	windowEnd := now.Add(-15 * time.Minute)

	store := unified.NewMemoryStore()
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:101",
		MaintenanceStartAt: &windowStart,
		MaintenanceEndAt:   &windowEnd,
		SetAt:              windowStart,
		SetBy:              "operator",
	}); err != nil {
		t.Fatalf("seed operator state: %v", err)
	}

	providers := Providers{
		Stores: func(orgID string) (unified.ResourceStore, error) { return store, nil },
		Now:    func() time.Time { return now },
	}
	sentinel, err := New(Config{OrgID: "default", Tick: time.Minute}, providers)
	if err != nil {
		t.Fatalf("new sentinel: %v", err)
	}

	sentinel.tickOnce(context.Background())

	reports, err := store.ListLoopReportsForResource(unified.LoopReportTypeMaintenanceVerification, "vm:101", 0)
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report after first tick, got %d", len(reports))
	}

	sentinel.tickOnce(context.Background())

	reports, err = store.ListLoopReportsForResource(unified.LoopReportTypeMaintenanceVerification, "vm:101", 0)
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected dedupe; got %d reports after second tick", len(reports))
	}
}

func TestSentinelTickOnceVerifiesEveryEndedRecurringOccurrence(t *testing.T) {
	now := time.Date(2026, 8, 27, 5, 0, 0, 0, time.UTC) // Thursday
	store := unified.NewMemoryStore()
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID: "node:pve-a",
		MaintenanceRecurrence: &unified.RecurringMaintenanceWindow{
			Timezone: "UTC", Weekdays: []string{"wednesday", "thursday"}, StartMinute: 120, EndMinute: 180,
		},
		MaintenanceScope: unified.MaintenanceScopeResourceAndDescendants,
		SetAt:            now.Add(-48 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	sentinel, err := New(Config{LookbackLimit: 7 * 24 * time.Hour}, Providers{
		Stores: func(string) (unified.ResourceStore, error) { return store, nil },
		Now:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	sentinel.tickOnce(context.Background())
	reports, err := store.ListLoopReportsForResource(unified.LoopReportTypeMaintenanceVerification, "node:pve-a", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 {
		t.Fatalf("recurring verification reports = %d, want one per ended occurrence", len(reports))
	}
	sentinel.tickOnce(context.Background())
	reports, _ = store.ListLoopReportsForResource(unified.LoopReportTypeMaintenanceVerification, "node:pve-a", 0)
	if len(reports) != 2 {
		t.Fatalf("recurring verification dedupe failed: %d reports", len(reports))
	}
}

func TestMaintenanceSentinelTickOnceWritesTimelineEvidence(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-time.Hour)
	windowEnd := now.Add(-15 * time.Minute)

	store := unified.NewMemoryStore()
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:101",
		MaintenanceStartAt: &windowStart,
		MaintenanceEndAt:   &windowEnd,
		SetAt:              windowStart,
		SetBy:              "operator",
	}); err != nil {
		t.Fatalf("seed operator state: %v", err)
	}

	providers := Providers{
		Stores: func(orgID string) (unified.ResourceStore, error) { return store, nil },
		Now:    func() time.Time { return now },
	}
	sentinel, err := New(Config{OrgID: "default", Tick: time.Minute}, providers)
	if err != nil {
		t.Fatalf("new sentinel: %v", err)
	}

	sentinel.tickOnce(context.Background())

	changes, err := store.GetRecentChanges("vm:101", windowStart, 0)
	if err != nil {
		t.Fatalf("recent changes: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected one timeline change from the verification report, got %d", len(changes))
	}
	if changes[0].SourceAdapter != unified.ChangeSourceAdapter("maintenance_sentinel") {
		t.Fatalf("source adapter = %q want maintenance_sentinel", changes[0].SourceAdapter)
	}
	if changes[0].Metadata["activityType"] != unified.MaintenanceVerificationActivityReported {
		t.Fatalf("activityType = %#v want maintenance verification reported", changes[0].Metadata["activityType"])
	}
}

func TestSentinelSkipsAncientWindows(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-30 * 24 * time.Hour)
	windowEnd := now.Add(-29 * 24 * time.Hour)

	store := unified.NewMemoryStore()
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:202",
		MaintenanceStartAt: &windowStart,
		MaintenanceEndAt:   &windowEnd,
		SetAt:              windowStart,
		SetBy:              "operator",
	}); err != nil {
		t.Fatalf("seed operator state: %v", err)
	}

	providers := Providers{
		Stores: func(orgID string) (unified.ResourceStore, error) { return store, nil },
		Now:    func() time.Time { return now },
	}
	sentinel, err := New(Config{OrgID: "default", LookbackLimit: 7 * 24 * time.Hour}, providers)
	if err != nil {
		t.Fatalf("new sentinel: %v", err)
	}
	sentinel.tickOnce(context.Background())

	reports, err := store.ListLoopReportsForResource(unified.LoopReportTypeMaintenanceVerification, "vm:202", 0)
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(reports) != 0 {
		t.Fatalf("expected no report for ancient window, got %d", len(reports))
	}
}

func TestSentinelSkipsOpenWindow(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-30 * time.Minute)
	windowEnd := now.Add(30 * time.Minute)

	store := unified.NewMemoryStore()
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:303",
		MaintenanceStartAt: &windowStart,
		MaintenanceEndAt:   &windowEnd,
		SetAt:              windowStart,
		SetBy:              "operator",
	}); err != nil {
		t.Fatalf("seed operator state: %v", err)
	}

	providers := Providers{
		Stores: func(orgID string) (unified.ResourceStore, error) { return store, nil },
		Now:    func() time.Time { return now },
	}
	sentinel, err := New(Config{OrgID: "default"}, providers)
	if err != nil {
		t.Fatalf("new sentinel: %v", err)
	}
	sentinel.tickOnce(context.Background())

	reports, err := store.ListLoopReportsForResource(unified.LoopReportTypeMaintenanceVerification, "vm:303", 0)
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(reports) != 0 {
		t.Fatalf("expected no report while window still open, got %d", len(reports))
	}
}

func TestSentinelEvaluateOncePersistsRerunReport(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-time.Hour)
	windowEnd := now.Add(-15 * time.Minute)

	store := unified.NewMemoryStore()
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:101",
		MaintenanceStartAt: &windowStart,
		MaintenanceEndAt:   &windowEnd,
		SetAt:              windowStart,
		SetBy:              "operator",
	}); err != nil {
		t.Fatalf("seed operator state: %v", err)
	}

	providers := Providers{
		Stores: func(orgID string) (unified.ResourceStore, error) { return store, nil },
		Now:    func() time.Time { return now },
	}
	sentinel, err := New(Config{OrgID: "default"}, providers)
	if err != nil {
		t.Fatalf("new sentinel: %v", err)
	}
	sentinel.tickOnce(context.Background())

	rerun, err := sentinel.EvaluateOnce(context.Background(), "default", "vm:101")
	if err != nil {
		t.Fatalf("evaluate once: %v", err)
	}
	if !strings.Contains(rerun.ID, "rerun") {
		t.Fatalf("expected rerun id suffix, got %q", rerun.ID)
	}
	reports, err := store.ListLoopReportsForResource(unified.LoopReportTypeMaintenanceVerification, "vm:101", 0)
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports after rerun, got %d", len(reports))
	}
}

func TestSentinelEvaluateOnceErrorsWhenNoWindow(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	store := unified.NewMemoryStore()
	if err := store.SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID: "vm:404",
		SetAt:       now,
		SetBy:       "operator",
	}); err != nil {
		t.Fatalf("seed operator state: %v", err)
	}
	providers := Providers{
		Stores: func(orgID string) (unified.ResourceStore, error) { return store, nil },
		Now:    func() time.Time { return now },
	}
	sentinel, err := New(Config{OrgID: "default"}, providers)
	if err != nil {
		t.Fatalf("new sentinel: %v", err)
	}
	if _, err := sentinel.EvaluateOnce(context.Background(), "default", "vm:404"); err == nil {
		t.Fatalf("expected error when no maintenance window present")
	}
}

func TestSentinelEvaluateOnceIsScopedToCallerOrg(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-time.Hour)
	windowEnd := now.Add(-15 * time.Minute)

	stores := map[string]*unified.MemoryStore{
		"default": unified.NewMemoryStore(),
		"acme":    unified.NewMemoryStore(),
	}
	if err := stores["default"].SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:101",
		MaintenanceStartAt: &windowStart,
		MaintenanceEndAt:   &windowEnd,
		SetAt:              windowStart,
		SetBy:              "operator",
	}); err != nil {
		t.Fatalf("seed default operator state: %v", err)
	}

	var evidenceOrgs []string
	providers := Providers{
		Stores: func(orgID string) (unified.ResourceStore, error) { return stores[orgID], nil },
		ActiveAlerts: func(orgID, _ string) []AlertSummary {
			evidenceOrgs = append(evidenceOrgs, orgID)
			return nil
		},
		ActiveFindings: func(orgID, _ string) []FindingSummary {
			evidenceOrgs = append(evidenceOrgs, orgID)
			return nil
		},
		RecentActions: func(orgID, _ string, _ time.Time) []ActionSummary {
			evidenceOrgs = append(evidenceOrgs, orgID)
			return nil
		},
		PostWindowMetricSamples: func(orgID, _ string, _, _ time.Time) ([]MetricSample, bool) {
			evidenceOrgs = append(evidenceOrgs, orgID)
			return nil, false
		},
		Now: func() time.Time { return now },
	}
	sentinel, err := New(Config{OrgID: "default"}, providers)
	if err != nil {
		t.Fatalf("new sentinel: %v", err)
	}
	countReports := func(orgID string) int {
		t.Helper()
		reports, err := stores[orgID].ListLoopReportsForResource(unified.LoopReportTypeMaintenanceVerification, "vm:101", 0)
		if err != nil {
			t.Fatalf("list %s reports: %v", orgID, err)
		}
		return len(reports)
	}

	// The window exists only in the sweep's own org: a rerun requested
	// from another org must not see it, nor leave a report behind.
	if _, err := sentinel.EvaluateOnce(context.Background(), "acme", "vm:101"); err == nil || !strings.Contains(err.Error(), "no operator state for") {
		t.Fatalf("cross-org rerun error = %v, want missing operator state", err)
	}
	if len(evidenceOrgs) != 0 {
		t.Fatalf("cross-org rerun gathered evidence for %v", evidenceOrgs)
	}
	if got := countReports("default"); got != 0 {
		t.Fatalf("cross-org rerun wrote %d reports into the default org", got)
	}

	acmeStart := now.Add(-3 * time.Hour)
	acmeEnd := now.Add(-2 * time.Hour)
	if err := stores["acme"].SetResourceOperatorState(unified.ResourceOperatorState{
		CanonicalID:        "vm:101",
		MaintenanceStartAt: &acmeStart,
		MaintenanceEndAt:   &acmeEnd,
		SetAt:              acmeStart,
		SetBy:              "operator",
	}); err != nil {
		t.Fatalf("seed acme operator state: %v", err)
	}
	report, err := sentinel.EvaluateOnce(context.Background(), "acme", "vm:101")
	if err != nil {
		t.Fatalf("evaluate once in acme: %v", err)
	}
	if report.WindowEndedAt == nil || !report.WindowEndedAt.Equal(acmeEnd) {
		t.Fatalf("window end = %v, want acme's %v", report.WindowEndedAt, acmeEnd)
	}
	if len(evidenceOrgs) != 4 {
		t.Fatalf("evidence providers called %d times, want 4", len(evidenceOrgs))
	}
	for _, orgID := range evidenceOrgs {
		if orgID != "acme" {
			t.Fatalf("evidence gathered for org %q, want acme only (%v)", orgID, evidenceOrgs)
		}
	}
	if got := countReports("acme"); got != 1 {
		t.Fatalf("acme reports = %d, want 1", got)
	}
	if got := countReports("default"); got != 0 {
		t.Fatalf("acme rerun wrote %d reports into the default org", got)
	}
}
