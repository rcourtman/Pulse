package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/hosted"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestHostedLifecycle(t *testing.T) {
	t.Run("Signup_SeedsTrialBillingState_ForBillingAdmin", func(t *testing.T) {
		router, persistence, _, _, baseDir := newHostedSignupTestRouter(t, true)

		rec := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("signup status=%d, want %d: %s", rec.Code, http.StatusAccepted, rec.Body.String())
		}

		var signupResp hostedSignupResponse
		if err := json.NewDecoder(rec.Body).Decode(&signupResp); err != nil {
			t.Fatalf("decode signup response: %v", err)
		}
		if signupResp.OrgID != "" || signupResp.UserID != "" {
			t.Fatalf("expected signup response to omit identifiers, got %+v", signupResp)
		}
		orgID := requireHostedSignupProvisionedOrgID(t, persistence, "owner@example.com")

		billingStore := config.NewFileBillingStore(baseDir)
		stored, err := billingStore.GetBillingState(orgID)
		if err != nil {
			t.Fatalf("GetBillingState(%s) failed: %v", orgID, err)
		}
		if stored == nil {
			t.Fatal("expected seeded billing state after hosted signup")
		}
		if stored.SubscriptionState != entitlements.SubStateTrial {
			t.Fatalf("subscription_state=%q, want %q", stored.SubscriptionState, entitlements.SubStateTrial)
		}
		if stored.PlanVersion != "cloud_trial" {
			t.Fatalf("plan_version=%q, want %q", stored.PlanVersion, "cloud_trial")
		}
		if stored.TrialStartedAt == nil {
			t.Fatal("expected trial_started_at to be populated")
		}
		if stored.TrialEndsAt == nil {
			t.Fatal("expected trial_ends_at to be populated")
		}
		if stored.QuickstartCreditsGranted {
			t.Fatal("expected hosted signup billing state not to grant quickstart credits")
		}
		if stored.QuickstartCreditsGrantedAt != nil {
			t.Fatal("expected hosted signup quickstart grant timestamp to stay empty")
		}

		expectedCaps := append([]string(nil), cloudCapabilitiesFromLicensing()...)
		sort.Strings(expectedCaps)
		gotCaps := append([]string(nil), stored.Capabilities...)
		sort.Strings(gotCaps)
		if !reflect.DeepEqual(gotCaps, expectedCaps) {
			t.Fatalf("seeded capabilities mismatch\n got: %v\nwant: %v", gotCaps, expectedCaps)
		}

		billingHandlers := NewBillingStateHandlers(billingStore, true)
		req := httptest.NewRequest(http.MethodGet, "/api/admin/orgs/"+orgID+"/billing-state", nil)
		req.SetPathValue("id", orgID)
		rec2 := httptest.NewRecorder()
		billingHandlers.HandleGetBillingState(rec2, req)
		if rec2.Code != http.StatusOK {
			t.Fatalf("billing-admin status=%d, want %d: %s", rec2.Code, http.StatusOK, rec2.Body.String())
		}

		var adminPayload entitlements.BillingState
		if err := json.NewDecoder(rec2.Body).Decode(&adminPayload); err != nil {
			t.Fatalf("decode billing-admin response: %v", err)
		}
		if adminPayload.SubscriptionState != entitlements.SubStateTrial {
			t.Fatalf("billing-admin subscription_state=%q, want %q", adminPayload.SubscriptionState, entitlements.SubStateTrial)
		}
		if adminPayload.PlanVersion != "cloud_trial" {
			t.Fatalf("billing-admin plan_version=%q, want %q", adminPayload.PlanVersion, "cloud_trial")
		}
		adminCaps := append([]string(nil), adminPayload.Capabilities...)
		sort.Strings(adminCaps)
		if !reflect.DeepEqual(adminCaps, expectedCaps) {
			t.Fatalf("billing-admin capabilities mismatch\n got: %v\nwant: %v", adminCaps, expectedCaps)
		}
	})

	t.Run("Signup_Billing_Entitlements_Flow", func(t *testing.T) {
		router, mtp, _, _, baseDir := newHostedSignupTestRouter(t, true)

		rec := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("signup status=%d, want %d: %s", rec.Code, http.StatusAccepted, rec.Body.String())
		}

		var signupResp hostedSignupResponse
		if err := json.NewDecoder(rec.Body).Decode(&signupResp); err != nil {
			t.Fatalf("decode signup response: %v", err)
		}
		if signupResp.OrgID != "" || signupResp.UserID != "" {
			t.Fatalf("expected signup response to omit identifiers, got %+v", signupResp)
		}
		orgID := requireHostedSignupProvisionedOrgID(t, mtp, "owner@example.com")

		// Write Pro billing state via the billing store.
		billingStore := config.NewFileBillingStore(baseDir)
		if err := billingStore.SaveBillingState(orgID, &entitlements.BillingState{
			Capabilities:      append([]string(nil), license.TierFeatures[license.TierPro]...),
			Limits:            map[string]int64{},
			MetersEnabled:     []string{},
			PlanVersion:       string(license.TierPro),
			SubscriptionState: entitlements.SubStateActive,
		}); err != nil {
			t.Fatalf("SaveBillingState(%s) failed: %v", orgID, err)
		}

		handlers := NewLicenseHandlers(mtp, true)
		ctx := context.WithValue(context.Background(), OrgIDContextKey, orgID)

		// GET entitlements and verify capabilities match billing state.
		entReq := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
		entRec := httptest.NewRecorder()
		handlers.HandleEntitlements(entRec, entReq)
		if entRec.Code != http.StatusOK {
			t.Fatalf("entitlements status=%d, want %d: %s", entRec.Code, http.StatusOK, entRec.Body.String())
		}

		var payload EntitlementPayload
		if err := json.NewDecoder(entRec.Body).Decode(&payload); err != nil {
			t.Fatalf("decode entitlements payload: %v", err)
		}
		if payload.SubscriptionState != string(license.SubStateActive) {
			t.Fatalf("subscription_state=%q, want %q", payload.SubscriptionState, license.SubStateActive)
		}

		expectedCaps := append([]string(nil), license.TierFeatures[license.TierPro]...)
		sort.Strings(expectedCaps)
		gotCaps := append([]string(nil), payload.Capabilities...)
		sort.Strings(gotCaps)
		if !reflect.DeepEqual(gotCaps, expectedCaps) {
			t.Fatalf("capabilities mismatch\n got: %v\nwant: %v", gotCaps, expectedCaps)
		}

		// Verify HasFeature() returns true for billed (Pro) capabilities.
		svc, _, err := handlers.getTenantComponents(ctx)
		if err != nil {
			t.Fatalf("getTenantComponents: %v", err)
		}
		if !svc.HasFeature(license.FeatureAIAutoFix) {
			t.Fatalf("expected HasFeature(%q)=true for billed capability", license.FeatureAIAutoFix)
		}
	})

	t.Run("Reaper_Cleanup_Cascade", func(t *testing.T) {
		baseDir := t.TempDir()
		mtp := config.NewMultiTenantPersistence(baseDir)

		orgID := "reap-org"
		if _, err := mtp.GetPersistence(orgID); err != nil {
			t.Fatalf("GetPersistence(%s) failed: %v", orgID, err)
		}

		requestedAt := time.Now().Add(-1 * time.Hour)
		if err := mtp.SaveOrganization(&models.Organization{
			ID:                  orgID,
			Status:              models.OrgStatusPendingDeletion,
			DisplayName:         "Reap Org",
			OwnerUserID:         "owner@example.com",
			CreatedAt:           time.Now().UTC(),
			DeletionRequestedAt: &requestedAt,
			RetentionDays:       0, // immediate expiry
		}); err != nil {
			t.Fatalf("SaveOrganization(%s) failed: %v", orgID, err)
		}

		rbacProvider := NewTenantRBACProvider(baseDir)
		t.Cleanup(func() { _ = rbacProvider.Close() })

		if _, err := rbacProvider.GetManager(orgID); err != nil {
			t.Fatalf("GetManager(%s) failed: %v", orgID, err)
		}
		if got := rbacProvider.ManagerCount(); got != 1 {
			t.Fatalf("ManagerCount()=%d, want 1", got)
		}

		router := &Router{rbacProvider: rbacProvider}

		reaper := hosted.NewReaper(mtp, mtp, time.Minute, true)
		reaper.OnBeforeDelete = func(tenant string) error {
			return router.CleanupTenant(context.Background(), tenant)
		}

		results := reaper.ScanOnce()
		if len(results) != 1 {
			t.Fatalf("ScanOnce results=%d, want 1 (%v)", len(results), results)
		}
		if results[0].OrgID != orgID {
			t.Fatalf("ScanOnce org_id=%q, want %q", results[0].OrgID, orgID)
		}
		if results[0].Action != "deleted" {
			t.Fatalf("ScanOnce action=%q, want %q", results[0].Action, "deleted")
		}
		if results[0].Error != nil {
			t.Fatalf("ScanOnce error=%v, want nil", results[0].Error)
		}

		// Verify org directory removed.
		orgDir := filepath.Join(baseDir, "orgs", orgID)
		if _, err := os.Stat(orgDir); err == nil || !os.IsNotExist(err) {
			t.Fatalf("expected org directory %s to be removed; stat err=%v", orgDir, err)
		}

		// Verify RBAC cleaned up (cached manager removed).
		if got := rbacProvider.ManagerCount(); got != 0 {
			t.Fatalf("ManagerCount() after cleanup=%d, want 0", got)
		}
	})

	t.Run("Tenant_Rate_Limiting", func(t *testing.T) {
		trl := NewTenantRateLimiter(2, time.Minute)
		defer trl.Stop()

		h := TenantRateLimitMiddleware(trl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		orgID := "test-rate-limit"

		for i := 1; i <= 2; i++ {
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, hostedLifecycleRequestWithOrg(orgID))
			if rr.Code != http.StatusOK {
				t.Fatalf("request %d status=%d, want %d", i, rr.Code, http.StatusOK)
			}
		}

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, hostedLifecycleRequestWithOrg(orgID))
		if rr.Code != http.StatusTooManyRequests {
			t.Fatalf("blocked request status=%d, want %d", rr.Code, http.StatusTooManyRequests)
		}
		if rr.Header().Get("X-RateLimit-Limit") != "2" {
			t.Fatalf("X-RateLimit-Limit=%q, want %q", rr.Header().Get("X-RateLimit-Limit"), "2")
		}
		if rr.Header().Get("X-RateLimit-Remaining") != "0" {
			t.Fatalf("X-RateLimit-Remaining=%q, want %q", rr.Header().Get("X-RateLimit-Remaining"), "0")
		}
		if rr.Header().Get("Retry-After") == "" {
			t.Fatal("Retry-After header should be set")
		}
	})
}

func sliceContainsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func hostedLifecycleRequestWithOrg(orgID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	ctx := context.WithValue(req.Context(), OrgIDContextKey, orgID)
	return req.WithContext(ctx)
}
