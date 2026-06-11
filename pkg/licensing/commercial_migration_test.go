package licensing

import (
	"fmt"
	"testing"
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
