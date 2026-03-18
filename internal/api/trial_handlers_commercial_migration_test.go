package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestTrialStart_RejectsUnresolvedCommercialMigration(t *testing.T) {
	tests := []struct {
		name        string
		migration   *pkglicensing.CommercialMigrationStatus
		wantMessage string
	}{
		{
			name: "pending",
			migration: &pkglicensing.CommercialMigrationStatus{
				Source:            pkglicensing.CommercialMigrationSourceV5License,
				State:             pkglicensing.CommercialMigrationStatePending,
				Reason:            pkglicensing.CommercialMigrationReasonExchangeUnavailable,
				RecommendedAction: pkglicensing.CommercialMigrationActionRetryActivation,
			},
			wantMessage: "Trial cannot be started while a paid v5 license migration is pending",
		},
		{
			name: "failed",
			migration: &pkglicensing.CommercialMigrationStatus{
				Source:            pkglicensing.CommercialMigrationSourceV5License,
				State:             pkglicensing.CommercialMigrationStateFailed,
				Reason:            pkglicensing.CommercialMigrationReasonExchangeInvalid,
				RecommendedAction: pkglicensing.CommercialMigrationActionUseV6Activation,
			},
			wantMessage: "Trial cannot be started until the paid v5 license migration is resolved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := t.TempDir()
			mtp := config.NewMultiTenantPersistence(baseDir)
			h := NewLicenseHandlers(mtp, false, &config.Config{PublicURL: "https://pulse.example.com"})

			orgID := "default"
			store := config.NewFileBillingStore(baseDir)
			if err := store.SaveBillingState(orgID, &entitlements.BillingState{
				Capabilities:        []string{},
				Limits:              map[string]int64{},
				MetersEnabled:       []string{},
				PlanVersion:         "free",
				SubscriptionState:   entitlements.SubStateExpired,
				CommercialMigration: tt.migration,
			}); err != nil {
				t.Fatalf("SaveBillingState: %v", err)
			}

			ctx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
			req := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			h.HandleStartTrial(rec, req)

			if rec.Code != http.StatusConflict {
				t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusConflict, rec.Body.String())
			}

			var resp APIError
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Code != "trial_not_available" {
				t.Fatalf("code=%q, want %q", resp.Code, "trial_not_available")
			}
			if resp.ErrorMessage != tt.wantMessage {
				t.Fatalf("message=%q, want %q", resp.ErrorMessage, tt.wantMessage)
			}
			if len(resp.Details) != 0 {
				t.Fatalf("details=%v, want empty", resp.Details)
			}
		})
	}
}
