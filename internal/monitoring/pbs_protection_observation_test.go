package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestBuildPBSProtectionProviderObservationMapsEvidenceQuality(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name             string
		datastores       int
		fetches          int
		errors           int
		terminalFailures int
		wantJob          recovery.Outcome
		wantHistory      recovery.ProtectionHistoryCompleteness
		wantPermissions  operationaltrust.EvidencePermissions
		wantReason       string
	}{
		{
			name:            "complete enumeration",
			datastores:      2,
			fetches:         2,
			wantJob:         recovery.OutcomeSuccess,
			wantHistory:     recovery.ProtectionHistoryComplete,
			wantPermissions: operationaltrust.EvidencePermissionsSufficient,
		},
		{
			name:            "partial transient enumeration",
			datastores:      2,
			fetches:         1,
			errors:          1,
			wantJob:         recovery.OutcomeWarning,
			wantHistory:     recovery.ProtectionHistoryPartial,
			wantPermissions: operationaltrust.EvidencePermissionsUnknown,
			wantReason:      "pbs_partial_enumeration",
		},
		{
			name:             "partial provider access",
			datastores:       2,
			fetches:          1,
			errors:           1,
			terminalFailures: 1,
			wantJob:          recovery.OutcomeWarning,
			wantHistory:      recovery.ProtectionHistoryPartial,
			wantPermissions:  operationaltrust.EvidencePermissionsPartial,
			wantReason:       "pbs_partial_provider_access",
		},
		{
			name:            "provider timeout",
			datastores:      2,
			errors:          2,
			wantJob:         recovery.OutcomeFailed,
			wantHistory:     recovery.ProtectionHistoryUnavailable,
			wantPermissions: operationaltrust.EvidencePermissionsUnknown,
			wantReason:      "pbs_collection_unavailable",
		},
		{
			name:             "provider denies every datastore",
			datastores:       2,
			errors:           2,
			terminalFailures: 2,
			wantJob:          recovery.OutcomeFailed,
			wantHistory:      recovery.ProtectionHistoryUnavailable,
			wantPermissions:  operationaltrust.EvidencePermissionsDenied,
			wantReason:       "pbs_provider_access_denied",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildPBSProtectionProviderObservation(
				"pbs-main",
				test.datastores,
				test.fetches,
				test.errors,
				test.terminalFailures,
				now,
			)
			if err != nil {
				t.Fatalf("buildPBSProtectionProviderObservation() error = %v", err)
			}
			if got.JobState != test.wantJob {
				t.Fatalf("job state = %q, want %q", got.JobState, test.wantJob)
			}
			if got.HistoryCompleteness != test.wantHistory {
				t.Fatalf(
					"history completeness = %q, want %q",
					got.HistoryCompleteness,
					test.wantHistory,
				)
			}
			if got.Permissions != test.wantPermissions {
				t.Fatalf("permissions = %q, want %q", got.Permissions, test.wantPermissions)
			}
			if test.wantReason == "" {
				if got.Evidence.Reason != nil {
					t.Fatalf("reason = %#v, want nil", got.Evidence.Reason)
				}
			} else if got.Evidence.Reason == nil ||
				got.Evidence.Reason.Code != test.wantReason {
				t.Fatalf("reason = %#v, want %q", got.Evidence.Reason, test.wantReason)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
