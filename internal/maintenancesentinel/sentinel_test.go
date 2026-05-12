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

	rerun, err := sentinel.EvaluateOnce(context.Background(), "vm:101")
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
	if _, err := sentinel.EvaluateOnce(context.Background(), "vm:404"); err == nil {
		t.Fatalf("expected error when no maintenance window present")
	}
}
