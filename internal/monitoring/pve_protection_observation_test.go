package monitoring

import (
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestBuildPVEProtectionProviderObservationMapsEvidenceQuality(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name            string
		stats           pveProtectionCollectionStats
		wantJob         recovery.Outcome
		wantHistory     recovery.ProtectionHistoryCompleteness
		wantPermissions operationaltrust.EvidencePermissions
		wantReason      string
	}{
		{
			name: "complete enumeration",
			stats: pveProtectionCollectionStats{
				NodeCount:             2,
				SuccessfulNodeQueries: 2,
				BackupStorageCount:    2,
				ContentSuccesses:      2,
			},
			wantJob:         recovery.OutcomeSuccess,
			wantHistory:     recovery.ProtectionHistoryComplete,
			wantPermissions: operationaltrust.EvidencePermissionsSufficient,
		},
		{
			name: "complete empty enumeration",
			stats: pveProtectionCollectionStats{
				NodeCount:             1,
				SuccessfulNodeQueries: 1,
			},
			wantJob:         recovery.OutcomeSuccess,
			wantHistory:     recovery.ProtectionHistoryComplete,
			wantPermissions: operationaltrust.EvidencePermissionsSufficient,
		},
		{
			name: "partial transient enumeration",
			stats: pveProtectionCollectionStats{
				NodeCount:             2,
				SuccessfulNodeQueries: 1,
				BackupStorageCount:    1,
				ContentSuccesses:      1,
			},
			wantJob:         recovery.OutcomeWarning,
			wantHistory:     recovery.ProtectionHistoryPartial,
			wantPermissions: operationaltrust.EvidencePermissionsUnknown,
			wantReason:      "pve_partial_enumeration",
		},
		{
			name: "partial provider access",
			stats: pveProtectionCollectionStats{
				NodeCount:              1,
				SuccessfulNodeQueries:  1,
				BackupStorageCount:     2,
				ContentSuccesses:       1,
				ContentFailures:        1,
				PermissionFailureCount: 1,
			},
			wantJob:         recovery.OutcomeWarning,
			wantHistory:     recovery.ProtectionHistoryPartial,
			wantPermissions: operationaltrust.EvidencePermissionsPartial,
			wantReason:      "pve_partial_provider_access",
		},
		{
			name: "provider timeout",
			stats: pveProtectionCollectionStats{
				NodeCount: 2,
			},
			wantJob:         recovery.OutcomeFailed,
			wantHistory:     recovery.ProtectionHistoryUnavailable,
			wantPermissions: operationaltrust.EvidencePermissionsUnknown,
			wantReason:      "pve_collection_unavailable",
		},
		{
			name: "provider denies every node",
			stats: pveProtectionCollectionStats{
				NodeCount:              2,
				PermissionFailureCount: 2,
			},
			wantJob:         recovery.OutcomeFailed,
			wantHistory:     recovery.ProtectionHistoryUnavailable,
			wantPermissions: operationaltrust.EvidencePermissionsDenied,
			wantReason:      "pve_provider_access_denied",
		},
		{
			name: "provider denies every backup storage",
			stats: pveProtectionCollectionStats{
				NodeCount:              1,
				SuccessfulNodeQueries:  1,
				BackupStorageCount:     2,
				ContentFailures:        2,
				PermissionFailureCount: 2,
			},
			wantJob:         recovery.OutcomeFailed,
			wantHistory:     recovery.ProtectionHistoryUnavailable,
			wantPermissions: operationaltrust.EvidencePermissionsDenied,
			wantReason:      "pve_provider_access_denied",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildPVEProtectionProviderObservation("pve-main", test.stats, now)
			if err != nil {
				t.Fatalf("buildPVEProtectionProviderObservation() error = %v", err)
			}
			if got.JobState != test.wantJob ||
				got.HistoryCompleteness != test.wantHistory ||
				got.Permissions != test.wantPermissions {
				t.Fatalf(
					"observation = job %q, history %q, permissions %q; want %q, %q, %q",
					got.JobState,
					got.HistoryCompleteness,
					got.Permissions,
					test.wantJob,
					test.wantHistory,
					test.wantPermissions,
				)
			}
			if test.wantReason == "" {
				if got.Evidence.Reason != nil {
					t.Fatalf("reason = %#v, want nil", got.Evidence.Reason)
				}
			} else if got.Evidence.Reason == nil || got.Evidence.Reason.Code != test.wantReason {
				t.Fatalf("reason = %#v, want %q", got.Evidence.Reason, test.wantReason)
			}
		})
	}
}

func TestIsPVEBackupPermissionError(t *testing.T) {
	t.Parallel()

	for _, err := range []error{
		errors.New("403 Forbidden"),
		errors.New("permission check failed"),
		errors.New("HTTP 401"),
	} {
		if !isPVEBackupPermissionError(err) {
			t.Fatalf("expected %q to be recognized as a permission failure", err)
		}
	}
	if isPVEBackupPermissionError(errors.New("connection timed out")) {
		t.Fatal("transient timeout must not be classified as a permission failure")
	}
}
