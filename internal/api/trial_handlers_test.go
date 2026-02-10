package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

func TestTrialStart_DefaultOrgWritesBillingJSONAndEnablesTrialEntitlements(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")

	// Warm the cached service to cover the "service already exists" path.
	if _, _, err := h.getTenantComponents(ctx); err != nil {
		t.Fatalf("getTenantComponents: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleStartTrial(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var state entitlements.BillingState
	if err := json.NewDecoder(rec.Body).Decode(&state); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if state.SubscriptionState != entitlements.SubStateTrial {
		t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, entitlements.SubStateTrial)
	}
	if state.TrialStartedAt == nil || state.TrialEndsAt == nil {
		t.Fatalf("expected trial_started_at and trial_ends_at to be populated, got started=%v ends=%v", state.TrialStartedAt, state.TrialEndsAt)
	}
	if *state.TrialEndsAt <= *state.TrialStartedAt {
		t.Fatalf("trial_ends_at (%d) must be after trial_started_at (%d)", *state.TrialEndsAt, *state.TrialStartedAt)
	}

	contains := func(values []string, key string) bool {
		for _, v := range values {
			if v == key {
				return true
			}
		}
		return false
	}
	if !contains(state.Capabilities, license.FeatureRelay) {
		t.Fatalf("expected billing capabilities to include %q, got %v", license.FeatureRelay, state.Capabilities)
	}
	if !contains(state.Capabilities, license.FeatureAIAutoFix) {
		t.Fatalf("expected billing capabilities to include %q, got %v", license.FeatureAIAutoFix, state.Capabilities)
	}

	billingPath := filepath.Join(baseDir, "billing.json")
	if _, err := os.Stat(billingPath); err != nil {
		t.Fatalf("expected billing.json at %s: %v", billingPath, err)
	}

	entReq := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	entRec := httptest.NewRecorder()
	h.HandleEntitlements(entRec, entReq)
	if entRec.Code != http.StatusOK {
		t.Fatalf("entitlements status=%d, want %d: %s", entRec.Code, http.StatusOK, entRec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.NewDecoder(entRec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode entitlements payload: %v", err)
	}
	if payload.SubscriptionState != string(license.SubStateTrial) {
		t.Fatalf("payload.subscription_state=%q, want %q", payload.SubscriptionState, license.SubStateTrial)
	}
	if payload.TrialExpiresAt == nil || payload.TrialDaysRemaining == nil {
		t.Fatalf("expected trial fields populated, got expires_at=%v days=%v", payload.TrialExpiresAt, payload.TrialDaysRemaining)
	}
	if *payload.TrialExpiresAt != *state.TrialEndsAt {
		t.Fatalf("payload.trial_expires_at=%d, want %d", *payload.TrialExpiresAt, *state.TrialEndsAt)
	}
	if *payload.TrialDaysRemaining < 13 || *payload.TrialDaysRemaining > 14 {
		t.Fatalf("payload.trial_days_remaining=%d, want 13-14", *payload.TrialDaysRemaining)
	}
	if !contains(payload.Capabilities, license.FeatureRelay) {
		t.Fatalf("expected payload capabilities to include %q, got %v", license.FeatureRelay, payload.Capabilities)
	}
}

func TestTrialStart_RejectsSecondAttemptForSameOrg(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")

	req1 := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(ctx)
	rec1 := httptest.NewRecorder()
	h.HandleStartTrial(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first status=%d, want %d: %s", rec1.Code, http.StatusOK, rec1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(ctx)
	rec2 := httptest.NewRecorder()
	h.HandleStartTrial(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("second status=%d, want %d: %s", rec2.Code, http.StatusConflict, rec2.Body.String())
	}
}

func TestTrialEntitlements_TrialDaysRemainingFromBillingState(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	orgID := "default"
	store := config.NewFileBillingStore(baseDir)
	now := time.Now()
	startedAt := now.Add(-1 * time.Hour).Unix()
	endsAt := now.Add(36 * time.Hour).Unix()
	if err := store.SaveBillingState(orgID, &entitlements.BillingState{
		Capabilities:      append([]string(nil), license.TierFeatures[license.TierPro]...),
		Limits:            map[string]int64{},
		MetersEnabled:     []string{},
		PlanVersion:       "trial",
		SubscriptionState: entitlements.SubStateTrial,
		TrialStartedAt:    &startedAt,
		TrialEndsAt:       &endsAt,
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	ctx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleEntitlements(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.SubscriptionState != string(license.SubStateTrial) {
		t.Fatalf("subscription_state=%q, want %q", payload.SubscriptionState, license.SubStateTrial)
	}
	if payload.TrialExpiresAt == nil || payload.TrialDaysRemaining == nil {
		t.Fatalf("expected trial fields, got expires_at=%v days=%v", payload.TrialExpiresAt, payload.TrialDaysRemaining)
	}
	if *payload.TrialExpiresAt != endsAt {
		t.Fatalf("trial_expires_at=%d, want %d", *payload.TrialExpiresAt, endsAt)
	}
	// 36 hours => 2 days (ceil).
	if *payload.TrialDaysRemaining != 2 {
		t.Fatalf("trial_days_remaining=%d, want %d", *payload.TrialDaysRemaining, 2)
	}
}
