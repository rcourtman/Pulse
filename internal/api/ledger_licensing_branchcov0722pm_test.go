package api

import (
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

func assertPanicsBranchcov0722PM(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic, but call returned normally")
		}
	}()
	fn()
}

func bcovPtrTime(v time.Time) *time.Time {
	return &v
}

func TestBranchcov0722PM_MonitoredSystemLedgerStatusSummary(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{"online", "online", "All included top-level collection paths currently report online status."},
		{"warning", "warning", "At least one included top-level collection path is degraded, so Pulse marks this monitored system as warning."},
		{"offline", "offline", "At least one included source is offline or disconnected, so Pulse marks this monitored system as offline."},
		{"unknown_falls_through_default", "unknown", "Pulse cannot determine a canonical runtime status for this monitored system yet."},
		{"empty_status_default", "", "Pulse cannot determine a canonical runtime status for this monitored system yet."},
		{"unrecognized_status_default", "degraded", "Pulse cannot determine a canonical runtime status for this monitored system yet."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultMonitoredSystemLedgerStatusSummary(tt.status)
			if got != tt.want {
				t.Errorf("defaultMonitoredSystemLedgerStatusSummary(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestBranchcov0722PM_ErrMonitoredSystemLedgerValidation(t *testing.T) {
	const message = "candidate.source is required"
	err := errMonitoredSystemLedgerValidation(message)

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if got := err.Error(); got != message {
		t.Errorf("Error() = %q, want %q", got, message)
	}

	var typed monitoredSystemLedgerValidationError
	if !errors.As(err, &typed) {
		t.Fatalf("error is not classifiable as monitoredSystemLedgerValidationError (errors.As): %T", err)
	}
	if string(typed) != message {
		t.Errorf("typed error payload = %q, want %q", string(typed), message)
	}

	if errors.Is(err, errMalformedLicenseSentinel) {
		t.Errorf("validation error must not be confused with a license sentinel error")
	}
}

func TestBranchcov0722PM_CommunityRuntimeIdentityFromLicensing(t *testing.T) {
	got := communityRuntimeIdentityFromLicensing()

	if got.Build != "community" {
		t.Errorf("Build = %q, want %q", got.Build, "community")
	}
	if got.Label != "Pulse Community runtime" {
		t.Errorf("Label = %q, want %q", got.Label, "Pulse Community runtime")
	}
	if got.DownloadURL != "" {
		t.Errorf("DownloadURL = %q, want empty", got.DownloadURL)
	}
}

func TestBranchcov0722PM_NormalizeDowngradeRetentionStateFromLicensing(t *testing.T) {
	t.Run("already_normal_returned_unchanged", func(t *testing.T) {
		in := downgradeRetentionStateModel{
			PreviousTier:         "pro",
			CurrentTier:          "free",
			PreviousHistoryDays:  90,
			CurrentHistoryDays:   7,
			DetectedAt:           111,
			RecoveryGuaranteedTo: 222,
			PurgeEligibleAt:      333,
		}
		got := normalizeDowngradeRetentionStateFromLicensing(in)
		if got != in {
			t.Errorf("already-normal state was mutated: got %+v, want %+v", got, in)
		}
	})

	t.Run("previous_tier_clamped", func(t *testing.T) {
		in := downgradeRetentionStateModel{
			PreviousTier:        "  PRO  ",
			CurrentTier:         "free",
			PreviousHistoryDays: 90,
			DetectedAt:          111,
		}
		got := normalizeDowngradeRetentionStateFromLicensing(in)
		if got.PreviousTier != "pro" {
			t.Errorf("PreviousTier = %q, want %q", got.PreviousTier, "pro")
		}
		if got.CurrentTier != "free" {
			t.Errorf("CurrentTier = %q, want %q", got.CurrentTier, "free")
		}
		if got.PreviousHistoryDays != 90 || got.DetectedAt != 111 {
			t.Errorf("non-tier field changed: got %+v", got)
		}
	})

	t.Run("current_tier_clamped", func(t *testing.T) {
		in := downgradeRetentionStateModel{
			PreviousTier:       "pro",
			CurrentTier:        "\tFREE\n",
			CurrentHistoryDays: 7,
		}
		got := normalizeDowngradeRetentionStateFromLicensing(in)
		if got.CurrentTier != "free" {
			t.Errorf("CurrentTier = %q, want %q", got.CurrentTier, "free")
		}
		if got.PreviousTier != "pro" {
			t.Errorf("PreviousTier = %q, want %q", got.PreviousTier, "pro")
		}
	})

	t.Run("both_tiers_uppercase_trimmed", func(t *testing.T) {
		in := downgradeRetentionStateModel{
			PreviousTier: "RELAY",
			CurrentTier:  "FREE",
		}
		got := normalizeDowngradeRetentionStateFromLicensing(in)
		if got.PreviousTier != "relay" || got.CurrentTier != "free" {
			t.Errorf("tiers = (%q,%q), want (relay,free)", got.PreviousTier, got.CurrentTier)
		}
	})
}

func TestBranchcov0722PM_WantsMockFixturesFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"set_true", "true", true},
		{"set_true_case_insensitive_and_padded", " True ", true},
		{"set_false", "false", false},
		{"junk_value", "banana", false},
		{"empty_equivalent_to_unset", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULSE_MOCK_MODE", tt.value)
			if got := wantsMockFixturesFromEnv(); got != tt.want {
				t.Errorf("wantsMockFixturesFromEnv() with PULSE_MOCK_MODE=%q = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestBranchcov0722PM_UnifiedLifecycleFromAI(t *testing.T) {
	t.Run("nil_or_empty_returns_nil", func(t *testing.T) {
		if got := unifiedLifecycleFromAI(nil); got != nil {
			t.Errorf("unifiedLifecycleFromAI(nil) = %+v, want nil", got)
		}
		if got := unifiedLifecycleFromAI([]ai.FindingLifecycleEvent{}); got != nil {
			t.Errorf("unifiedLifecycleFromAI(empty) = %+v, want nil", got)
		}
	})

	at1 := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	at2 := time.Date(2026, 7, 22, 9, 30, 0, 0, time.UTC)

	t.Run("populated_events_converted_field_for_field", func(t *testing.T) {
		in := []ai.FindingLifecycleEvent{
			{
				At:      at1,
				Type:    "detected",
				Message: "first detection",
				To:      "active",
				Metadata: map[string]string{
					"run_id": "r-1",
					"host":   "tower",
				},
			},
			{
				At:   at2,
				Type: "remediated",
				From: "active",
				To:   "resolved",
			},
		}
		got := unifiedLifecycleFromAI(in)
		if len(got) != len(in) {
			t.Fatalf("len = %d, want %d", len(got), len(in))
		}

		if got[0].At != at1 || got[0].Type != "detected" || got[0].Message != "first detection" || got[0].To != "active" || got[0].From != "" {
			t.Errorf("event[0] mismatched: %+v", got[0])
		}
		if got[0].Metadata == nil || got[0].Metadata["run_id"] != "r-1" || got[0].Metadata["host"] != "tower" {
			t.Errorf("event[0].Metadata not preserved: %+v", got[0].Metadata)
		}

		if got[1].At != at2 || got[1].Type != "remediated" || got[1].From != "active" || got[1].To != "resolved" || got[1].Message != "" {
			t.Errorf("event[1] mismatched: %+v", got[1])
		}
		if got[1].Metadata != nil {
			t.Errorf("event[1].Metadata = %+v, want nil", got[1].Metadata)
		}
	})
}

func TestBranchcov0722PM_UnifiedFindingFromAI(t *testing.T) {
	t.Run("nil_input_panics", func(t *testing.T) {
		assertPanicsBranchcov0722PM(t, func() { _ = unifiedFindingFromAI(nil) })
	})

	t.Run("minimal_finding", func(t *testing.T) {
		in := &ai.Finding{
			ID:       "f-min",
			Title:    "Minimal",
			Severity: ai.FindingSeverityInfo,
			Category: ai.FindingCategoryGeneral,
		}
		got := unifiedFindingFromAI(in)
		if got == nil {
			t.Fatal("expected non-nil unified finding")
		}
		if got.ID != "f-min" {
			t.Errorf("ID = %q, want %q", got.ID, "f-min")
		}
		if got.Title != "Minimal" {
			t.Errorf("Title = %q, want %q", got.Title, "Minimal")
		}
		if got.Source != unified.SourceAIPatrol {
			t.Errorf("Source = %q, want %q", got.Source, unified.SourceAIPatrol)
		}
		if got.Severity != unified.SeverityInfo {
			t.Errorf("Severity = %q, want %q", got.Severity, unified.SeverityInfo)
		}
		if got.Category != unified.CategoryGeneral {
			t.Errorf("Category = %q, want %q", got.Category, unified.CategoryGeneral)
		}
		if got.Lifecycle != nil {
			t.Errorf("Lifecycle = %+v, want nil", got.Lifecycle)
		}
	})

	t.Run("fully_populated_finding", func(t *testing.T) {
		detected := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
		lastSeen := time.Date(2026, 7, 22, 8, 15, 0, 0, time.UTC)
		resolved := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
		lastInvestigated := time.Date(2026, 7, 22, 8, 45, 0, 0, time.UTC)
		lastRegression := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
		acknowledged := time.Date(2026, 7, 22, 8, 5, 0, 0, time.UTC)
		snoozed := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
		remind := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
		investigationRecord := &aicontracts.InvestigationRecord{}

		in := &ai.Finding{
			ID:                         "f-full",
			Severity:                   ai.FindingSeverityWarning,
			Category:                   ai.FindingCategoryCapacity,
			ResourceID:                 "res-1",
			ResourceName:               "tower",
			ResourceType:               "host",
			Node:                       "node-1",
			Title:                      "Disk filling up",
			Description:                "Root partition at 92%.",
			Impact:                     "System may become unresponsive.",
			PreviousResolvedFixSummary: "Cleared old logs on 2026-07-10.",
			Recommendation:             "Prune /var/log archives.",
			Evidence:                   "df -h / => 92%",
			DetectedAt:                 detected,
			LastSeenAt:                 lastSeen,
			ResolvedAt:                 &resolved,
			AutoResolved:               true,
			InvestigationSessionID:     "sess-1",
			InvestigationStatus:        "completed",
			InvestigationOutcome:       "fix_verified",
			LastInvestigatedAt:         &lastInvestigated,
			InvestigationAttempts:      3,
			InvestigationRecord:        investigationRecord,
			LoopState:                  "resolved",
			Lifecycle: []ai.FindingLifecycleEvent{
				{
					At:      detected,
					Type:    "detected",
					Message: "first seen",
					To:      "active",
					Metadata: map[string]string{
						"run_id": "r-1",
					},
				},
				{
					At:   resolved,
					Type: "remediated",
					From: "active",
					To:   "resolved",
				},
			},
			RegressionCount:  2,
			LastRegressionAt: &lastRegression,
			AcknowledgedAt:   &acknowledged,
			SnoozedUntil:     &snoozed,
			DismissedReason:  "will_fix_later",
			UserNote:         "watch over the weekend",
			Suppressed:       false,
			TimesRaised:      5,
			RemindAt:         &remind,
		}

		got := unifiedFindingFromAI(in)
		if got == nil {
			t.Fatal("expected non-nil unified finding")
		}

		if got.ID != "f-full" {
			t.Errorf("ID = %q", got.ID)
		}
		if got.Source != unified.SourceAIPatrol {
			t.Errorf("Source = %q, want %q", got.Source, unified.SourceAIPatrol)
		}
		if got.Severity != unified.SeverityWarning {
			t.Errorf("Severity = %q, want %q", got.Severity, unified.SeverityWarning)
		}
		if got.Category != unified.CategoryCapacity {
			t.Errorf("Category = %q, want %q", got.Category, unified.CategoryCapacity)
		}
		if got.ResourceID != "res-1" || got.ResourceName != "tower" || got.ResourceType != "host" || got.Node != "node-1" {
			t.Errorf("resource fields mismatched: %+v", got)
		}
		if got.Title != "Disk filling up" || got.Description != "Root partition at 92%." {
			t.Errorf("title/description mismatched: %+v", got)
		}
		if got.Impact != "System may become unresponsive." {
			t.Errorf("Impact = %q", got.Impact)
		}
		if got.PreviousResolvedFixSummary != "Cleared old logs on 2026-07-10." {
			t.Errorf("PreviousResolvedFixSummary = %q", got.PreviousResolvedFixSummary)
		}
		if got.Recommendation != "Prune /var/log archives." || got.Evidence != "df -h / => 92%" {
			t.Errorf("recommendation/evidence mismatched: %+v", got)
		}
		if got.DetectedAt != detected || got.LastSeenAt != lastSeen {
			t.Errorf("timestamps mismatched: detected=%v lastSeen=%v", got.DetectedAt, got.LastSeenAt)
		}
		if got.ResolvedAt == nil || *got.ResolvedAt != resolved {
			t.Errorf("ResolvedAt = %+v, want %v", got.ResolvedAt, resolved)
		}
		if !got.AutoResolved {
			t.Errorf("AutoResolved = false, want true")
		}
		if got.InvestigationSessionID != "sess-1" || got.InvestigationStatus != "completed" || got.InvestigationOutcome != "fix_verified" {
			t.Errorf("investigation identity/status mismatched: %+v", got)
		}
		if got.LastInvestigatedAt == nil || *got.LastInvestigatedAt != lastInvestigated {
			t.Errorf("LastInvestigatedAt = %+v, want %v", got.LastInvestigatedAt, lastInvestigated)
		}
		if got.InvestigationAttempts != 3 {
			t.Errorf("InvestigationAttempts = %d, want 3", got.InvestigationAttempts)
		}
		if got.InvestigationRecord != investigationRecord {
			t.Errorf("InvestigationRecord pointer not preserved: %p vs %p", got.InvestigationRecord, investigationRecord)
		}
		if got.LoopState != "resolved" {
			t.Errorf("LoopState = %q", got.LoopState)
		}
		if got.RegressionCount != 2 {
			t.Errorf("RegressionCount = %d, want 2", got.RegressionCount)
		}
		if got.LastRegressionAt == nil || *got.LastRegressionAt != lastRegression {
			t.Errorf("LastRegressionAt = %+v, want %v", got.LastRegressionAt, lastRegression)
		}
		if got.AcknowledgedAt == nil || *got.AcknowledgedAt != acknowledged {
			t.Errorf("AcknowledgedAt = %+v, want %v", got.AcknowledgedAt, acknowledged)
		}
		if got.SnoozedUntil == nil || *got.SnoozedUntil != snoozed {
			t.Errorf("SnoozedUntil = %+v, want %v", got.SnoozedUntil, snoozed)
		}
		if got.DismissedReason != "will_fix_later" || got.UserNote != "watch over the weekend" {
			t.Errorf("dismissed/usernote mismatched: %+v", got)
		}
		if got.Suppressed {
			t.Errorf("Suppressed = true, want false")
		}
		if got.TimesRaised != 5 {
			t.Errorf("TimesRaised = %d, want 5", got.TimesRaised)
		}
		if got.RemindAt == nil || *got.RemindAt != remind {
			t.Errorf("RemindAt = %+v, want %v", got.RemindAt, remind)
		}

		if len(got.Lifecycle) != 2 {
			t.Fatalf("Lifecycle len = %d, want 2", len(got.Lifecycle))
		}
		if got.Lifecycle[0].At != detected || got.Lifecycle[0].Type != "detected" || got.Lifecycle[0].Message != "first seen" || got.Lifecycle[0].To != "active" {
			t.Errorf("lifecycle[0] mismatched: %+v", got.Lifecycle[0])
		}
		if got.Lifecycle[0].Metadata == nil || got.Lifecycle[0].Metadata["run_id"] != "r-1" {
			t.Errorf("lifecycle[0].Metadata not preserved: %+v", got.Lifecycle[0].Metadata)
		}
		if got.Lifecycle[1].Type != "remediated" || got.Lifecycle[1].From != "active" || got.Lifecycle[1].To != "resolved" {
			t.Errorf("lifecycle[1] mismatched: %+v", got.Lifecycle[1])
		}
	})
}
