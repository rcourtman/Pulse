package licensing

import (
	"fmt"
	"testing"
	"time"
)

func TestClassifyLegacyExchangeError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantState  CommercialMigrationState
		wantReason CommercialMigrationReason
		wantAction CommercialMigrationAction
	}{
		{
			name:       "retryable server error stays pending",
			err:        fmt.Errorf("activation failed: %w", &LicenseServerError{StatusCode: 503, Code: "unavailable", Retryable: true}),
			wantState:  CommercialMigrationStatePending,
			wantReason: CommercialMigrationReasonExchangeUnavailable,
		},
		{
			name:       "rate limit stays pending",
			err:        fmt.Errorf("activation failed: %w", &LicenseServerError{StatusCode: 429, Code: "rate_limited"}),
			wantState:  CommercialMigrationStatePending,
			wantReason: CommercialMigrationReasonExchangeRateLimited,
		},
		{
			name:       "invalid token is terminal",
			err:        fmt.Errorf("activation failed: %w", &LicenseServerError{StatusCode: 401, Code: "invalid_legacy_token"}),
			wantState:  CommercialMigrationStateFailed,
			wantReason: CommercialMigrationReasonExchangeInvalid,
		},
		{
			name:       "renewed key is terminal with retrieve guidance",
			err:        fmt.Errorf("activation failed: %w", &LicenseServerError{StatusCode: 401, Code: "RENEWED_KEY_AVAILABLE"}),
			wantState:  CommercialMigrationStateFailed,
			wantReason: CommercialMigrationReasonExchangeStaleKey,
			wantAction: CommercialMigrationActionRetrieveCurrentKey,
		},
		{
			name:       "unsupported key format is terminal",
			err:        fmt.Errorf("license key is not a supported v6 activation key or migratable v5 license"),
			wantState:  CommercialMigrationStateFailed,
			wantReason: CommercialMigrationReasonExchangeUnsupportedKey,
		},
		{
			name:       "installation limit conflict is terminal with slot guidance",
			err:        fmt.Errorf("activation failed: %w", &LicenseServerError{StatusCode: 409, Code: "MAX_INSTALLATIONS"}),
			wantState:  CommercialMigrationStateFailed,
			wantReason: CommercialMigrationReasonExchangeInstallationLimit,
			wantAction: CommercialMigrationActionFreeInstallationSlot,
		},
		{
			name:       "other conflict stays pending",
			err:        fmt.Errorf("activation failed: %w", &LicenseServerError{StatusCode: 409, Code: "EXCHANGE_IN_PROGRESS"}),
			wantState:  CommercialMigrationStatePending,
			wantReason: CommercialMigrationReasonExchangeConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyLegacyExchangeError(tt.err)
			if got == nil {
				t.Fatal("expected non-nil migration status")
			}
			if got.State != tt.wantState {
				t.Fatalf("state=%q, want %q", got.State, tt.wantState)
			}
			if got.Reason != tt.wantReason {
				t.Fatalf("reason=%q, want %q", got.Reason, tt.wantReason)
			}
			if tt.wantAction != "" && got.RecommendedAction != tt.wantAction {
				t.Fatalf("action=%q, want %q", got.RecommendedAction, tt.wantAction)
			}
		})
	}
}

func TestApplyCommercialMigrationFailureTimingEscalatesSustainedTransportFailure(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	initial := ApplyCommercialMigrationFailureTiming(&CommercialMigrationStatus{
		Source:            CommercialMigrationSourceV5License,
		State:             CommercialMigrationStatePending,
		Reason:            CommercialMigrationReasonExchangeUnavailable,
		RecommendedAction: CommercialMigrationActionRetryActivation,
	}, nil, start)
	if initial == nil {
		t.Fatal("expected initial migration status")
	}
	if initial.FirstFailedAt != start.Unix() {
		t.Fatalf("first_failed_at=%d want %d", initial.FirstFailedAt, start.Unix())
	}
	if initial.Reason != CommercialMigrationReasonExchangeUnavailable {
		t.Fatalf("initial reason=%q want %q", initial.Reason, CommercialMigrationReasonExchangeUnavailable)
	}

	escalated := ApplyCommercialMigrationFailureTiming(&CommercialMigrationStatus{
		Source:            CommercialMigrationSourceV5License,
		State:             CommercialMigrationStatePending,
		Reason:            CommercialMigrationReasonExchangeUnavailable,
		RecommendedAction: CommercialMigrationActionRetryActivation,
	}, initial, start.Add(time.Duration(CommercialMigrationSustainedExchangeUnavailableSeconds+1)*time.Second))
	if escalated == nil {
		t.Fatal("expected escalated migration status")
	}
	if escalated.FirstFailedAt != start.Unix() {
		t.Fatalf("escalated first_failed_at=%d want %d", escalated.FirstFailedAt, start.Unix())
	}
	if escalated.Reason != CommercialMigrationReasonExchangeConnectivity {
		t.Fatalf("escalated reason=%q want %q", escalated.Reason, CommercialMigrationReasonExchangeConnectivity)
	}
	if escalated.RecommendedAction != CommercialMigrationActionAllowLicenseEgress {
		t.Fatalf("escalated action=%q want %q", escalated.RecommendedAction, CommercialMigrationActionAllowLicenseEgress)
	}

	terminal := ApplyCommercialMigrationFailureTiming(&CommercialMigrationStatus{
		Source:            CommercialMigrationSourceV5License,
		State:             CommercialMigrationStateFailed,
		Reason:            CommercialMigrationReasonExchangeInvalid,
		RecommendedAction: CommercialMigrationActionEnterSupportedV5,
	}, escalated, start.Add(48*time.Hour))
	if terminal == nil {
		t.Fatal("expected terminal migration status")
	}
	if terminal.FirstFailedAt != 0 {
		t.Fatalf("terminal first_failed_at=%d want 0", terminal.FirstFailedAt)
	}
}
