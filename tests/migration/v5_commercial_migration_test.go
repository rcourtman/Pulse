package migration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestV5PaidLicenseUpgrade_CommercialMigrationFailureMatrix(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		responseBody    map[string]any
		wantState       pkglicensing.CommercialMigrationState
		wantReason      pkglicensing.CommercialMigrationReason
		wantAction      pkglicensing.CommercialMigrationAction
		wantTrialReason string
	}{
		{
			name:       "exchange_unavailable_retryable",
			statusCode: http.StatusServiceUnavailable,
			responseBody: map[string]any{
				"code":      "service_unavailable",
				"message":   "exchange unavailable",
				"retryable": true,
			},
			wantState:       pkglicensing.CommercialMigrationStatePending,
			wantReason:      pkglicensing.CommercialMigrationReasonExchangeUnavailable,
			wantAction:      pkglicensing.CommercialMigrationActionRetryActivation,
			wantTrialReason: "commercial_migration_pending",
		},
		{
			name:       "exchange_rate_limited",
			statusCode: http.StatusTooManyRequests,
			responseBody: map[string]any{
				"code":      "rate_limited",
				"message":   "try again later",
				"retryable": true,
			},
			wantState:       pkglicensing.CommercialMigrationStatePending,
			wantReason:      pkglicensing.CommercialMigrationReasonExchangeRateLimited,
			wantAction:      pkglicensing.CommercialMigrationActionRetryActivation,
			wantTrialReason: "commercial_migration_pending",
		},
		{
			name:       "exchange_conflict",
			statusCode: http.StatusConflict,
			responseBody: map[string]any{
				"code":      "activation_conflict",
				"message":   "activation already exists",
				"retryable": false,
			},
			wantState:       pkglicensing.CommercialMigrationStatePending,
			wantReason:      pkglicensing.CommercialMigrationReasonExchangeConflict,
			wantAction:      pkglicensing.CommercialMigrationActionRetryActivation,
			wantTrialReason: "commercial_migration_pending",
		},
		{
			name:       "exchange_malformed",
			statusCode: http.StatusBadRequest,
			responseBody: map[string]any{
				"code":      "invalid_request",
				"message":   "malformed legacy key",
				"retryable": false,
			},
			wantState:       pkglicensing.CommercialMigrationStateFailed,
			wantReason:      pkglicensing.CommercialMigrationReasonExchangeMalformed,
			wantAction:      pkglicensing.CommercialMigrationActionEnterSupportedV5,
			wantTrialReason: "commercial_migration_failed",
		},
		{
			name:       "exchange_invalid",
			statusCode: http.StatusUnauthorized,
			responseBody: map[string]any{
				"code":      "invalid_license",
				"message":   "legacy key invalid",
				"retryable": false,
			},
			wantState:       pkglicensing.CommercialMigrationStateFailed,
			wantReason:      pkglicensing.CommercialMigrationReasonExchangeInvalid,
			wantAction:      pkglicensing.CommercialMigrationActionEnterSupportedV5,
			wantTrialReason: "commercial_migration_failed",
		},
		{
			name:       "exchange_revoked",
			statusCode: http.StatusForbidden,
			responseBody: map[string]any{
				"code":      "revoked",
				"message":   "license revoked",
				"retryable": false,
			},
			wantState:       pkglicensing.CommercialMigrationStateFailed,
			wantReason:      pkglicensing.CommercialMigrationReasonExchangeRevoked,
			wantAction:      pkglicensing.CommercialMigrationActionUseV6Activation,
			wantTrialReason: "commercial_migration_failed",
		},
		{
			name:       "exchange_non_migratable",
			statusCode: http.StatusGone,
			responseBody: map[string]any{
				"code":      "non_migratable",
				"message":   "license already retired",
				"retryable": false,
			},
			wantState:       pkglicensing.CommercialMigrationStateFailed,
			wantReason:      pkglicensing.CommercialMigrationReasonExchangeNonMigratable,
			wantAction:      pkglicensing.CommercialMigrationActionUseV6Activation,
			wantTrialReason: "commercial_migration_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

			dataDir, _, _, _, _ := buildV5DataDir(t)
			legacyLicense, err := pkglicensing.GenerateLicenseForTesting(
				"legacy-paid@example.com",
				pkglicensing.TierLifetime,
				365*24*time.Hour,
			)
			require.NoError(t, err)

			persistence, err := pkglicensing.NewPersistence(dataDir)
			require.NoError(t, err)
			require.NoError(t, persistence.Save(legacyLicense))

			exchangeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/v1/licenses/exchange", r.URL.Path)

				var req pkglicensing.ExchangeLegacyLicenseRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				assert.Equal(t, legacyLicense, req.LegacyLicenseKey)

				w.WriteHeader(tt.statusCode)
				require.NoError(t, json.NewEncoder(w).Encode(tt.responseBody))
			}))
			defer exchangeServer.Close()
			t.Setenv("PULSE_LICENSE_SERVER_URL", exchangeServer.URL)

			mtp := config.NewMultiTenantPersistence(dataDir)
			handlers := api.NewLicenseHandlers(mtp, false)
			t.Cleanup(handlers.StopAllBackgroundLoops)

			ctx := context.WithValue(context.Background(), api.OrgIDContextKey, "default")
			svc := handlers.Service(ctx)
			require.NotNil(t, svc)
			assert.False(t, svc.IsActivated(), "failed exchange must not produce an activated v6 state")
			assert.Nil(t, svc.Current(), "failed exchange must not leave an active grant loaded")

			activationState, err := persistence.LoadActivationState()
			require.NoError(t, err)
			assert.Nil(t, activationState, "failed exchange must not persist activation state")

			legacyLeft, err := persistence.Load()
			require.NoError(t, err)
			assert.Equal(t, legacyLicense, legacyLeft, "legacy v5 license must remain persisted for downgrade")

			store := config.NewFileBillingStore(dataDir)
			state, err := store.GetBillingState("default")
			require.NoError(t, err)
			require.NotNil(t, state, "commercial migration state must be persisted in billing state")
			require.NotNil(t, state.CommercialMigration, "commercial migration state must be recorded")
			assert.Equal(t, tt.wantState, state.CommercialMigration.State)
			assert.Equal(t, tt.wantReason, state.CommercialMigration.Reason)
			assert.Equal(t, tt.wantAction, state.CommercialMigration.RecommendedAction)

			statusReq := httptest.NewRequest(http.MethodGet, "/api/license/status", nil).WithContext(ctx)
			statusRec := httptest.NewRecorder()
			handlers.HandleLicenseStatus(statusRec, statusReq)
			require.Equal(t, http.StatusOK, statusRec.Code)

			var status pkglicensing.LicenseStatus
			require.NoError(t, json.Unmarshal(statusRec.Body.Bytes(), &status))
			assert.False(t, status.Valid, "failed exchange must not report an active license")
			assert.Equal(t, pkglicensing.TierFree, status.Tier, "failed exchange must remain in free tier state")

			entReq := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
			entRec := httptest.NewRecorder()
			handlers.HandleEntitlements(entRec, entReq)
			require.Equal(t, http.StatusOK, entRec.Code)

			var entitlements api.EntitlementPayload
			require.NoError(t, json.Unmarshal(entRec.Body.Bytes(), &entitlements))
			require.NotNil(t, entitlements.CommercialMigration, "entitlements must expose commercial migration state")
			assert.Equal(t, tt.wantState, entitlements.CommercialMigration.State)
			assert.Equal(t, tt.wantReason, entitlements.CommercialMigration.Reason)
			assert.Equal(t, tt.wantAction, entitlements.CommercialMigration.RecommendedAction)
			assert.False(t, entitlements.TrialEligible, "commercial migration work must block trial start")
			assert.Equal(t, tt.wantTrialReason, entitlements.TrialEligibilityReason)
			assert.Equal(t, "expired", entitlements.SubscriptionState)
			assert.Equal(t, "free", entitlements.Tier)
		})
	}
}
