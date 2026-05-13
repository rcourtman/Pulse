package migration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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
		name         string
		statusCode   int
		responseBody map[string]any
		wantState    pkglicensing.CommercialMigrationState
		wantReason   pkglicensing.CommercialMigrationReason
		wantAction   pkglicensing.CommercialMigrationAction
	}{
		{
			name:       "exchange_unavailable_retryable",
			statusCode: http.StatusServiceUnavailable,
			responseBody: map[string]any{
				"code":      "service_unavailable",
				"message":   "exchange unavailable",
				"retryable": true,
			},
			wantState:  pkglicensing.CommercialMigrationStatePending,
			wantReason: pkglicensing.CommercialMigrationReasonExchangeUnavailable,
			wantAction: pkglicensing.CommercialMigrationActionRetryActivation,
		},
		{
			name:       "exchange_rate_limited",
			statusCode: http.StatusTooManyRequests,
			responseBody: map[string]any{
				"code":      "rate_limited",
				"message":   "try again later",
				"retryable": true,
			},
			wantState:  pkglicensing.CommercialMigrationStatePending,
			wantReason: pkglicensing.CommercialMigrationReasonExchangeRateLimited,
			wantAction: pkglicensing.CommercialMigrationActionRetryActivation,
		},
		{
			name:       "exchange_conflict",
			statusCode: http.StatusConflict,
			responseBody: map[string]any{
				"code":      "activation_conflict",
				"message":   "activation already exists",
				"retryable": false,
			},
			wantState:  pkglicensing.CommercialMigrationStatePending,
			wantReason: pkglicensing.CommercialMigrationReasonExchangeConflict,
			wantAction: pkglicensing.CommercialMigrationActionRetryActivation,
		},
		{
			name:       "exchange_malformed",
			statusCode: http.StatusBadRequest,
			responseBody: map[string]any{
				"code":      "invalid_request",
				"message":   "malformed legacy key",
				"retryable": false,
			},
			wantState:  pkglicensing.CommercialMigrationStateFailed,
			wantReason: pkglicensing.CommercialMigrationReasonExchangeMalformed,
			wantAction: pkglicensing.CommercialMigrationActionEnterSupportedV5,
		},
		{
			name:       "exchange_invalid",
			statusCode: http.StatusUnauthorized,
			responseBody: map[string]any{
				"code":      "invalid_license",
				"message":   "legacy key invalid",
				"retryable": false,
			},
			wantState:  pkglicensing.CommercialMigrationStateFailed,
			wantReason: pkglicensing.CommercialMigrationReasonExchangeInvalid,
			wantAction: pkglicensing.CommercialMigrationActionEnterSupportedV5,
		},
		{
			name:       "exchange_revoked",
			statusCode: http.StatusForbidden,
			responseBody: map[string]any{
				"code":      "revoked",
				"message":   "license revoked",
				"retryable": false,
			},
			wantState:  pkglicensing.CommercialMigrationStateFailed,
			wantReason: pkglicensing.CommercialMigrationReasonExchangeRevoked,
			wantAction: pkglicensing.CommercialMigrationActionUseV6Activation,
		},
		{
			name:       "exchange_non_migratable",
			statusCode: http.StatusGone,
			responseBody: map[string]any{
				"code":      "non_migratable",
				"message":   "license already retired",
				"retryable": false,
			},
			wantState:  pkglicensing.CommercialMigrationStateFailed,
			wantReason: pkglicensing.CommercialMigrationReasonExchangeNonMigratable,
			wantAction: pkglicensing.CommercialMigrationActionUseV6Activation,
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
			assert.Empty(t, entitlements.TrialEligibilityReason, "retired self-hosted trial acquisition must not leak a trial start reason")
			assert.Equal(t, "expired", entitlements.SubscriptionState)
			assert.Equal(t, "free", entitlements.Tier)
		})
	}
}

func TestV5PaidLicenseUpgrade_CommercialMigrationClearsAfterLocalRetry(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "false")

	dataDir, _, _, _, _ := buildV5DataDir(t)
	legacyLicense, err := pkglicensing.GenerateLicenseForTesting(
		"legacy-retry@example.com",
		pkglicensing.TierPro,
		365*24*time.Hour,
	)
	require.NoError(t, err)

	persistence, err := pkglicensing.NewPersistence(dataDir)
	require.NoError(t, err)
	require.NoError(t, persistence.Save(legacyLicense))

	grantJWT, grantPublicKey, err := pkglicensing.GenerateGrantJWTForTesting(pkglicensing.GrantClaims{
		LicenseID: "lic_v5_retry_migrated",
		Tier:      string(pkglicensing.TierPro),
		PlanKey:   "v5_pro_monthly_grandfathered",
		State:     "active",
		Features:  append([]string(nil), pkglicensing.TierFeatures[pkglicensing.TierPro]...),
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
		Email:     "legacy-retry@example.com",
	})
	require.NoError(t, err)
	pkglicensing.SetPublicKey(grantPublicKey)
	t.Cleanup(func() { pkglicensing.SetPublicKey(nil) })

	var exchangeAvailable atomic.Bool
	exchangeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/licenses/exchange", r.URL.Path)

		var req pkglicensing.ExchangeLegacyLicenseRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, legacyLicense, req.LegacyLicenseKey)

		if !exchangeAvailable.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"code":      "service_unavailable",
				"message":   "exchange unavailable",
				"retryable": true,
			}))
			return
		}

		w.WriteHeader(http.StatusCreated)
		require.NoError(t, json.NewEncoder(w).Encode(pkglicensing.ActivateInstallationResponse{
			License: pkglicensing.ActivateResponseLicense{
				LicenseID: "lic_v5_retry_migrated",
				State:     "active",
				Tier:      string(pkglicensing.TierPro),
				Features:  append([]string(nil), pkglicensing.TierFeatures[pkglicensing.TierPro]...),
			},
			Installation: pkglicensing.ActivateResponseInstallation{
				InstallationID:    "inst_v5_retry_migrated",
				InstallationToken: "pit_live_v5_retry_migrated",
				Status:            "active",
			},
			Grant: pkglicensing.GrantEnvelope{
				JWT:       grantJWT,
				JTI:       "grant_v5_retry_migrated",
				ExpiresAt: time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
			},
		}))
	}))
	defer exchangeServer.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", exchangeServer.URL)

	ctx := context.WithValue(context.Background(), api.OrgIDContextKey, "default")
	mtp := config.NewMultiTenantPersistence(dataDir)
	handlers := api.NewLicenseHandlers(mtp, false)

	svc := handlers.Service(ctx)
	require.NotNil(t, svc)
	assert.False(t, svc.IsActivated(), "unavailable exchange must leave v6 activation unset")

	store := config.NewFileBillingStore(dataDir)
	state, err := store.GetBillingState("default")
	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, state.CommercialMigration, "failed exchange must persist retryable migration state")
	assert.Equal(t, pkglicensing.CommercialMigrationStatePending, state.CommercialMigration.State)
	assert.Equal(t, pkglicensing.CommercialMigrationReasonExchangeUnavailable, state.CommercialMigration.Reason)
	assert.Equal(t, pkglicensing.CommercialMigrationActionRetryActivation, state.CommercialMigration.RecommendedAction)

	entReq := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	entRec := httptest.NewRecorder()
	handlers.HandleEntitlements(entRec, entReq)
	require.Equal(t, http.StatusOK, entRec.Code)

	var pendingEntitlements api.EntitlementPayload
	require.NoError(t, json.Unmarshal(entRec.Body.Bytes(), &pendingEntitlements))
	require.NotNil(t, pendingEntitlements.CommercialMigration)
	assert.Equal(t, pkglicensing.CommercialMigrationStatePending, pendingEntitlements.CommercialMigration.State)
	assert.False(t, pendingEntitlements.TrialEligible)
	assert.Empty(t, pendingEntitlements.TrialEligibilityReason)

	handlers.StopAllBackgroundLoops()
	exchangeAvailable.Store(true)

	handlers = api.NewLicenseHandlers(config.NewMultiTenantPersistence(dataDir), false)
	t.Cleanup(handlers.StopAllBackgroundLoops)

	svc = handlers.Service(ctx)
	require.NotNil(t, svc)
	require.True(t, svc.IsActivated(), "local retry after exchange recovery must activate v6 entitlements")

	current := svc.Current()
	require.NotNil(t, current)
	assert.Equal(t, "lic_v5_retry_migrated", current.Claims.LicenseID)
	assert.Equal(t, pkglicensing.TierPro, current.Claims.Tier)
	assert.Equal(t, "v5_pro_monthly_grandfathered", current.Claims.PlanVersion)

	activationState, err := persistence.LoadActivationState()
	require.NoError(t, err)
	require.NotNil(t, activationState, "successful retry must persist v6 activation state")
	assert.True(t, activationState.Continuity.LegacyMigration, "activation state must retain v5 migration continuity")

	legacyLeft, err := persistence.Load()
	require.NoError(t, err)
	assert.Equal(t, legacyLicense, legacyLeft, "legacy v5 license must remain persisted for downgrade")

	state, err = store.GetBillingState("default")
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Nil(t, state.CommercialMigration, "successful local retry must clear unresolved migration state")

	entReq = httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	entRec = httptest.NewRecorder()
	handlers.HandleEntitlements(entRec, entReq)
	require.Equal(t, http.StatusOK, entRec.Code)

	var activeEntitlements api.EntitlementPayload
	require.NoError(t, json.Unmarshal(entRec.Body.Bytes(), &activeEntitlements))
	assert.Nil(t, activeEntitlements.CommercialMigration)
	assert.Equal(t, "active", activeEntitlements.SubscriptionState)
	assert.Equal(t, string(pkglicensing.TierPro), activeEntitlements.Tier)
	assert.Equal(t, "v5_pro_monthly_grandfathered", activeEntitlements.PlanVersion)
	assert.False(t, activeEntitlements.TrialEligible)
	assert.Empty(t, activeEntitlements.TrialEligibilityReason)
	assertEntitlementLimitAbsent(t, activeEntitlements.Limits, pkglicensing.MaxMonitoredSystemsLicenseGateKey)
}
