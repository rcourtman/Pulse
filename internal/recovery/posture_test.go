package recovery

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

func protectionTestSummary(
	now time.Time,
	lastSuccess *time.Time,
) ProtectionProviderSummary {
	return ProtectionProviderSummary{
		Provider:             ProviderProxmoxPBS,
		Source:               "pbs-backup-enumeration",
		Scope:                "pbs-main",
		JobState:             OutcomeSuccess,
		HistoryCompleteness:  ProtectionHistoryComplete,
		Permissions:          operationaltrust.EvidencePermissionsSufficient,
		VerificationExpected: true,
		LastAttemptAt:        cloneProtectionTime(lastSuccess),
		LastSuccessAt:        cloneProtectionTime(lastSuccess),
		LastVerifiedAt:       cloneProtectionTime(lastSuccess),
		BackupPointCount:     1,
		RepositoryResourceIDs: []string{
			"repository:pbs-main/store-a",
		},
		EvidenceIDs: []string{"evidence:point-1", "evidence:provider-1"},
	}
}

func TestDeriveProtectionPostureTruthTable(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	recent := now.Add(-2 * time.Hour)
	stale := now.Add(-8 * 24 * time.Hour)
	policy := DefaultProtectionPosturePolicy

	tests := []struct {
		name         string
		summaries    []ProtectionProviderSummary
		wantState    ProtectionState
		wantFresh    ProtectionFreshness
		wantVerify   ProtectionVerification
		wantCoverage ProtectionCoverage
		wantText     string
	}{
		{
			name:         "current verified PBS backup is protected",
			summaries:    []ProtectionProviderSummary{protectionTestSummary(now, &recent)},
			wantState:    ProtectionStateProtected,
			wantFresh:    ProtectionFreshnessCurrent,
			wantVerify:   ProtectionVerificationVerified,
			wantCoverage: ProtectionCoverageComplete,
			wantText:     "current subject-linked backup",
		},
		{
			name: "stale success needs attention",
			summaries: []ProtectionProviderSummary{
				protectionTestSummary(now, &stale),
			},
			wantState:    ProtectionStateAttention,
			wantFresh:    ProtectionFreshnessStale,
			wantVerify:   ProtectionVerificationStale,
			wantCoverage: ProtectionCoverageComplete,
			wantText:     "older than",
		},
		{
			name: "missing expected verification needs attention",
			summaries: func() []ProtectionProviderSummary {
				summary := protectionTestSummary(now, &recent)
				summary.LastVerifiedAt = nil
				return []ProtectionProviderSummary{summary}
			}(),
			wantState:    ProtectionStateAttention,
			wantFresh:    ProtectionFreshnessCurrent,
			wantVerify:   ProtectionVerificationUnverified,
			wantCoverage: ProtectionCoverageComplete,
			wantText:     "verification evidence",
		},
		{
			name: "newer provider failure invalidates protected claim",
			summaries: func() []ProtectionProviderSummary {
				summary := protectionTestSummary(now, &recent)
				failedAt := now.Add(-time.Hour)
				summary.JobState = OutcomeFailed
				summary.LastAttemptAt = &failedAt
				return []ProtectionProviderSummary{summary}
			}(),
			wantState:    ProtectionStateAttention,
			wantFresh:    ProtectionFreshnessCurrent,
			wantVerify:   ProtectionVerificationVerified,
			wantCoverage: ProtectionCoverageComplete,
			wantText:     "newer provider failure",
		},
		{
			name: "complete snapshot-only history is unprotected",
			summaries: []ProtectionProviderSummary{
				{
					Provider:            ProviderProxmoxPVE,
					Source:              "pve-snapshot-enumeration",
					Scope:               "pve-main",
					JobState:            OutcomeSuccess,
					HistoryCompleteness: ProtectionHistoryComplete,
					Permissions:         operationaltrust.EvidencePermissionsSufficient,
					SnapshotPointCount:  3,
					EvidenceIDs:         []string{"evidence:snapshot"},
				},
			},
			wantState:    ProtectionStateUnprotected,
			wantFresh:    ProtectionFreshnessUnknown,
			wantVerify:   ProtectionVerificationUnknown,
			wantCoverage: ProtectionCoverageNone,
			wantText:     "snapshots alone",
		},
		{
			name: "partial provider history is attention when a backup exists",
			summaries: func() []ProtectionProviderSummary {
				summary := protectionTestSummary(now, &recent)
				summary.HistoryCompleteness = ProtectionHistoryPartial
				summary.Permissions = operationaltrust.EvidencePermissionsPartial
				return []ProtectionProviderSummary{summary}
			}(),
			wantState:    ProtectionStateAttention,
			wantFresh:    ProtectionFreshnessCurrent,
			wantVerify:   ProtectionVerificationVerified,
			wantCoverage: ProtectionCoveragePartial,
			wantText:     "incomplete",
		},
		{
			name: "permission denied is unknown",
			summaries: func() []ProtectionProviderSummary {
				summary := protectionTestSummary(now, &recent)
				summary.HistoryCompleteness = ProtectionHistoryUnavailable
				summary.Permissions = operationaltrust.EvidencePermissionsDenied
				return []ProtectionProviderSummary{summary}
			}(),
			wantState:    ProtectionStateUnknown,
			wantFresh:    ProtectionFreshnessCurrent,
			wantVerify:   ProtectionVerificationVerified,
			wantCoverage: ProtectionCoverageUnknown,
			wantText:     "permissions are unavailable",
		},
		{
			name: "confirmed PBS recovery is not invalidated by an unknown legacy provider",
			summaries: func() []ProtectionProviderSummary {
				confirmed := protectionTestSummary(now, &recent)
				legacy := ProtectionProviderSummary{
					Provider:            ProviderProxmoxPVE,
					Source:              "legacy-recovery-point",
					Scope:               "pve-main",
					JobState:            OutcomeSuccess,
					HistoryCompleteness: ProtectionHistoryUnknown,
					Permissions:         operationaltrust.EvidencePermissionsUnknown,
					LastAttemptAt:       &recent,
					LastSuccessAt:       &recent,
					BackupPointCount:    1,
					EvidenceIDs:         []string{"evidence:legacy-pve"},
				}
				return []ProtectionProviderSummary{confirmed, legacy}
			}(),
			wantState:    ProtectionStateProtected,
			wantFresh:    ProtectionFreshnessCurrent,
			wantVerify:   ProtectionVerificationVerified,
			wantCoverage: ProtectionCoverageUnknown,
			wantText:     "does not invalidate",
		},
		{
			name:         "no provider evidence is unknown",
			summaries:    nil,
			wantState:    ProtectionStateUnknown,
			wantFresh:    ProtectionFreshnessUnknown,
			wantVerify:   ProtectionVerificationUnknown,
			wantCoverage: ProtectionCoverageUnknown,
			wantText:     "no complete provider history",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := DeriveProtectionPostureAt(
				"resource:vm-100",
				test.summaries,
				policy,
				now,
			)
			if got.State != test.wantState {
				t.Fatalf("state = %q, want %q; posture=%#v", got.State, test.wantState, got)
			}
			if got.Freshness != test.wantFresh {
				t.Fatalf("freshness = %q, want %q", got.Freshness, test.wantFresh)
			}
			if got.Verification != test.wantVerify {
				t.Fatalf("verification = %q, want %q", got.Verification, test.wantVerify)
			}
			if got.Coverage != test.wantCoverage {
				t.Fatalf("coverage = %q, want %q", got.Coverage, test.wantCoverage)
			}
			if !strings.Contains(got.Explanation, test.wantText) {
				t.Fatalf("explanation = %q, want substring %q", got.Explanation, test.wantText)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestBuildProtectionPostureFromPointsUsesLatestProviderObservation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	completedAt := now.Add(-time.Hour)
	verified := true
	point := RecoveryPoint{
		ID:                "pbs-backup:vm-100-2026-07-19",
		Provider:          ProviderProxmoxPBS,
		Kind:              KindBackup,
		Mode:              ModeRemote,
		Outcome:           OutcomeSuccess,
		CompletedAt:       &completedAt,
		Verified:          &verified,
		SubjectResourceID: "resource:vm-100",
		ProviderScope:     "pbs-main",
	}
	envelope, err := NewRecoveryPointEvidence(point, "pbs-backup-inventory", now)
	if err != nil {
		t.Fatalf("NewRecoveryPointEvidence() error = %v", err)
	}
	point.Evidence = envelope

	older, err := NewProtectionProviderObservation(
		ProviderProxmoxPBS,
		"pbs-backup-enumeration",
		"pbs-main",
		OutcomeFailed,
		ProtectionHistoryUnavailable,
		operationaltrust.EvidencePermissionsUnknown,
		true,
		now.Add(-2*time.Hour),
		now.Add(-2*time.Hour),
		&operationaltrust.EvidenceReason{Code: "pbs_timeout"},
	)
	if err != nil {
		t.Fatalf("older observation error = %v", err)
	}
	current, err := NewProtectionProviderObservation(
		ProviderProxmoxPBS,
		"pbs-backup-enumeration",
		"pbs-main",
		OutcomeSuccess,
		ProtectionHistoryComplete,
		operationaltrust.EvidencePermissionsSufficient,
		true,
		now,
		now,
		nil,
	)
	if err != nil {
		t.Fatalf("current observation error = %v", err)
	}

	got := BuildProtectionPostureFromPointsAt(
		"resource:vm-100",
		[]RecoveryPoint{point},
		[]ProtectionProviderObservation{current, older},
		DefaultProtectionPosturePolicy,
		now,
	)
	if got.State != ProtectionStateProtected {
		t.Fatalf("state = %q, want protected; posture=%#v", got.State, got)
	}
	if len(got.ProviderStates) != 1 {
		t.Fatalf("provider states = %d, want 1", len(got.ProviderStates))
	}
	if got.ProviderStates[0].HistoryCompleteness != ProtectionHistoryComplete {
		t.Fatalf(
			"history completeness = %q, want complete",
			got.ProviderStates[0].HistoryCompleteness,
		)
	}
	if len(got.EvidenceIDs) != 2 {
		t.Fatalf("evidence ids = %#v, want point and latest provider evidence", got.EvidenceIDs)
	}
}

func TestNewProtectionProviderObservationRequiresTypedLimitation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	observation, err := NewProtectionProviderObservation(
		ProviderProxmoxPBS,
		"pbs-backup-enumeration",
		"pbs-main",
		OutcomeFailed,
		ProtectionHistoryUnavailable,
		operationaltrust.EvidencePermissionsDenied,
		true,
		now,
		now,
		&operationaltrust.EvidenceReason{
			Code:    "pbs_access_denied",
			Message: "PBS did not authorize backup history enumeration.",
		},
	)
	if err != nil {
		t.Fatalf("NewProtectionProviderObservation() error = %v", err)
	}
	if observation.Evidence.Reason == nil ||
		observation.Evidence.Reason.Code != "pbs_access_denied" {
		t.Fatalf("reason = %#v, want pbs_access_denied", observation.Evidence.Reason)
	}
	if observation.Evidence.Permissions != operationaltrust.EvidencePermissionsDenied {
		t.Fatalf("permissions = %q, want denied", observation.Evidence.Permissions)
	}
}
