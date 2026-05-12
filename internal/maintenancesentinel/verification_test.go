package maintenancesentinel

import (
	"strings"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestEvaluateVerification_HealthyWhenNoSignals(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-30 * time.Minute)
	windowEnd := now.Add(-10 * time.Minute)
	inputs := VerificationInputs{
		ResourceID:            "vm:101",
		OperatorState:         &unified.ResourceOperatorState{CanonicalID: "vm:101", MaintenanceStartAt: &windowStart, MaintenanceEndAt: &windowEnd},
		WindowStartedAt:       windowStart,
		WindowEndedAt:         windowEnd,
		Now:                   now,
		MetricSourceAvailable: true,
		PostWindowMetricSamples: []MetricSample{
			{Metric: "cpu", Value: 12, Timestamp: windowEnd.Add(time.Minute)},
			{Metric: "cpu", Value: 13, Timestamp: windowEnd.Add(2 * time.Minute)},
			{Metric: "memory", Value: 40, Timestamp: windowEnd.Add(time.Minute)},
			{Metric: "memory", Value: 41, Timestamp: windowEnd.Add(2 * time.Minute)},
		},
	}
	report := EvaluateVerification(inputs)
	if report.Status != unified.LoopReportStatusHealthy {
		t.Fatalf("status = %q want %q (recommendation=%q)", report.Status, unified.LoopReportStatusHealthy, report.Recommendation)
	}
	if report.Type != unified.LoopReportTypeMaintenanceVerification {
		t.Fatalf("type = %q want %q", report.Type, unified.LoopReportTypeMaintenanceVerification)
	}
	if report.Scope != "vm:101" {
		t.Fatalf("scope = %q want vm:101", report.Scope)
	}
	if report.WindowEndedAt == nil || !report.WindowEndedAt.Equal(windowEnd) {
		t.Fatalf("windowEndedAt = %v want %v", report.WindowEndedAt, windowEnd)
	}
	if !strings.HasPrefix(report.ID, "mv-vm_101-") {
		t.Fatalf("report id = %q want prefix mv-vm_101-", report.ID)
	}
}

func TestEvaluateVerification_FailedOnCriticalAlert(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-time.Hour)
	windowEnd := now.Add(-15 * time.Minute)
	inputs := VerificationInputs{
		ResourceID:      "vm:101",
		WindowStartedAt: windowStart,
		WindowEndedAt:   windowEnd,
		Now:             now,
		ActiveAlerts: []AlertSummary{
			{ID: "alert-1", Severity: SeverityCritical, Type: "cpu"},
			{ID: "alert-2", Severity: SeverityWarning, Type: "memory"},
		},
		MetricSourceAvailable: true,
	}
	report := EvaluateVerification(inputs)
	if report.Status != unified.LoopReportStatusFailedVerification {
		t.Fatalf("status = %q want failed_verification", report.Status)
	}
	if report.Evidence.ActiveCriticalAlerts != 1 || report.Evidence.ActiveWarningAlerts != 1 {
		t.Fatalf("alert counts = %d/%d want 1/1", report.Evidence.ActiveCriticalAlerts, report.Evidence.ActiveWarningAlerts)
	}
	if len(report.LinkedAlertIDs) != 2 {
		t.Fatalf("linked alert ids = %v want 2 entries", report.LinkedAlertIDs)
	}
}

func TestEvaluateVerification_NeedsReviewOnNoMetrics(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-time.Hour)
	windowEnd := now.Add(-15 * time.Minute)
	inputs := VerificationInputs{
		ResourceID:            "ct:200",
		WindowStartedAt:       windowStart,
		WindowEndedAt:         windowEnd,
		Now:                   now,
		MetricSourceAvailable: true,
	}
	report := EvaluateVerification(inputs)
	if report.Status != unified.LoopReportStatusNeedsReview {
		t.Fatalf("status = %q want needs_review", report.Status)
	}
	if report.Evidence.MetricRecovery == nil || report.Evidence.MetricRecovery.SamplesAfterEnd != 0 {
		t.Fatalf("metric recovery = %+v", report.Evidence.MetricRecovery)
	}
}

func TestEvaluateVerification_FailedOnFailedAction(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-time.Hour)
	windowEnd := now.Add(-15 * time.Minute)
	inputs := VerificationInputs{
		ResourceID:      "vm:101",
		WindowStartedAt: windowStart,
		WindowEndedAt:   windowEnd,
		Now:             now,
		RecentActions: []ActionSummary{
			{ID: "action-1", State: "failed", UpdatedAt: windowEnd.Add(-time.Minute)},
			{ID: "action-2", State: "succeeded", UpdatedAt: windowEnd.Add(time.Minute)},
		},
		MetricSourceAvailable: true,
	}
	report := EvaluateVerification(inputs)
	if report.Status != unified.LoopReportStatusFailedVerification {
		t.Fatalf("status = %q want failed_verification", report.Status)
	}
	if report.Evidence.FailedActionsSinceWindowStart != 1 {
		t.Fatalf("failed actions = %d want 1", report.Evidence.FailedActionsSinceWindowStart)
	}
	if len(report.LinkedActionIDs) != 1 || report.LinkedActionIDs[0] != "action-1" {
		t.Fatalf("linked action ids = %v want [action-1]", report.LinkedActionIDs)
	}
}

func TestEvaluateVerification_NeedsReviewWhenIntentionallyOffline(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-time.Hour)
	windowEnd := now.Add(-15 * time.Minute)
	inputs := VerificationInputs{
		ResourceID: "vm:101",
		OperatorState: &unified.ResourceOperatorState{
			CanonicalID:          "vm:101",
			IntentionallyOffline: true,
			MaintenanceStartAt:   &windowStart,
			MaintenanceEndAt:     &windowEnd,
		},
		WindowStartedAt:       windowStart,
		WindowEndedAt:         windowEnd,
		Now:                   now,
		MetricSourceAvailable: true,
		PostWindowMetricSamples: []MetricSample{
			{Metric: "cpu", Value: 5, Timestamp: windowEnd.Add(time.Minute)},
			{Metric: "cpu", Value: 6, Timestamp: windowEnd.Add(2 * time.Minute)},
		},
	}
	report := EvaluateVerification(inputs)
	if report.Status != unified.LoopReportStatusNeedsReview {
		t.Fatalf("status = %q want needs_review", report.Status)
	}
	if !strings.Contains(report.Evidence.OperatorStateSummary, "intentionally offline") {
		t.Fatalf("operator state summary = %q want to mention intentionally offline", report.Evidence.OperatorStateSummary)
	}
}

func TestEvaluateVerification_FilesPatrolTODOWhenWarningsOnly(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-time.Hour)
	windowEnd := now.Add(-15 * time.Minute)
	inputs := VerificationInputs{
		ResourceID:      "vm:101",
		WindowStartedAt: windowStart,
		WindowEndedAt:   windowEnd,
		Now:             now,
		ActiveFindings: []FindingSummary{
			{ID: "finding-1", Severity: SeverityWarning, Category: "performance"},
		},
		MetricSourceAvailable: true,
	}
	report := EvaluateVerification(inputs)
	if report.Status != unified.LoopReportStatusNeedsReview {
		t.Fatalf("status = %q want needs_review", report.Status)
	}
	if report.Evidence.PatrolRunTODO == "" {
		t.Fatalf("expected patrol run TODO breadcrumb on ambiguous evidence")
	}
	if len(report.LinkedFindingIDs) != 1 {
		t.Fatalf("linked finding ids = %v want 1", report.LinkedFindingIDs)
	}
}

func TestEvaluateVerification_NeedsReviewWhenNoMetricSource(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-time.Hour)
	windowEnd := now.Add(-15 * time.Minute)
	inputs := VerificationInputs{
		ResourceID:      "storage:tank",
		WindowStartedAt: windowStart,
		WindowEndedAt:   windowEnd,
		Now:             now,
	}
	report := EvaluateVerification(inputs)
	if report.Status != unified.LoopReportStatusNeedsReview {
		t.Fatalf("status = %q want needs_review when no metric source", report.Status)
	}
	if report.Evidence.MetricRecovery != nil {
		t.Fatalf("metric recovery = %+v want nil when source unavailable", report.Evidence.MetricRecovery)
	}
}
