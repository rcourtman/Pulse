package api

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/hosted"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestHostedLifecycle(t *testing.T) {
	t.Run("Signup_Billing_Entitlements_Flow", func(t *testing.T) {
		router, mtp, _, _, baseDir := newHostedSignupTestRouter(t, true)

		rec := doHostedSignupRequest(router, `{"email":"owner@example.com","org_name":"My Organization"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("signup status=%d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
		}

		var signupResp struct {
			OrgID string `json:"org_id"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&signupResp); err != nil {
			t.Fatalf("decode signup response: %v", err)
		}
		if signupResp.OrgID == "" {
			t.Fatal("expected signup response org_id to be populated")
		}

		// Write Pro billing state via the billing store.
		billingStore := config.NewFileBillingStore(baseDir)
		if err := billingStore.SaveBillingState(signupResp.OrgID, &entitlements.BillingState{
			Capabilities:      append([]string(nil), license.TierFeatures[license.TierPro]...),
			Limits:            map[string]int64{},
			MetersEnabled:     []string{},
			PlanVersion:       string(license.TierPro),
			SubscriptionState: entitlements.SubStateActive,
		}); err != nil {
			t.Fatalf("SaveBillingState(%s) failed: %v", signupResp.OrgID, err)
		}

		handlers := NewLicenseHandlers(mtp, true)
		ctx := context.WithValue(context.Background(), OrgIDContextKey, signupResp.OrgID)

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

	t.Run("Trial_Start_Countdown_Expiry", func(t *testing.T) {
		baseDir := t.TempDir()
		mtp := config.NewMultiTenantPersistence(baseDir)
		handlers := NewLicenseHandlers(mtp, false, &config.Config{PublicURL: "https://pulse.example.com"})

		ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")

		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatalf("GenerateKey: %v", err)
		}
		t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

		token, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
			OrgID:        "default",
			Email:        "owner@example.com",
			InstanceHost: "pulse.example.com",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			},
		})
		if err != nil {
			t.Fatalf("SignTrialActivationToken: %v", err)
		}

		startReq := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(token), nil).WithContext(ctx)
		startReq.Host = "pulse.example.com"
		startRec := httptest.NewRecorder()
		handlers.HandleTrialActivation(startRec, startReq)
		if startRec.Code != http.StatusTemporaryRedirect {
			t.Fatalf("trial activation status=%d, want %d: %s", startRec.Code, http.StatusTemporaryRedirect, startRec.Body.String())
		}
		if got := startRec.Header().Get("Location"); got != "/settings?trial=activated" {
			t.Fatalf("trial activation redirect=%q, want %q", got, "/settings?trial=activated")
		}

		billingStore := config.NewFileBillingStore(baseDir)
		state, err := billingStore.GetBillingState("default")
		if err != nil || state == nil {
			t.Fatalf("GetBillingState(default) failed: %v state=%v", err, state)
		}
		if state.SubscriptionState != entitlements.SubStateTrial {
			t.Fatalf("trial activation subscription_state=%q, want %q", state.SubscriptionState, entitlements.SubStateTrial)
		}

		entReq1 := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
		entRec1 := httptest.NewRecorder()
		handlers.HandleEntitlements(entRec1, entReq1)
		if entRec1.Code != http.StatusOK {
			t.Fatalf("entitlements status=%d, want %d: %s", entRec1.Code, http.StatusOK, entRec1.Body.String())
		}

		var payload1 EntitlementPayload
		if err := json.NewDecoder(entRec1.Body).Decode(&payload1); err != nil {
			t.Fatalf("decode entitlements payload: %v", err)
		}
		if payload1.SubscriptionState != string(license.SubStateTrial) {
			t.Fatalf("subscription_state=%q, want %q", payload1.SubscriptionState, license.SubStateTrial)
		}
		if payload1.TrialDaysRemaining == nil || *payload1.TrialDaysRemaining <= 0 {
			t.Fatalf("expected trial_days_remaining to be populated and > 0, got %v", payload1.TrialDaysRemaining)
		}

		// Manually expire the trial by moving TrialEndsAt into the past.
		loaded, err := billingStore.GetBillingState("default")
		if err != nil || loaded == nil {
			t.Fatalf("GetBillingState(default) failed: %v state=%v", err, loaded)
		}
		endsAtPast := time.Now().Add(-2 * time.Hour).Unix()
		loaded.SubscriptionState = entitlements.SubStateTrial // keep trial; evaluator normalizes to expired when endsAt is in the past
		loaded.TrialEndsAt = &endsAtPast
		if err := billingStore.SaveBillingState("default", loaded); err != nil {
			t.Fatalf("SaveBillingState(default) failed: %v", err)
		}

		entReq2 := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
		entRec2 := httptest.NewRecorder()
		handlers.HandleEntitlements(entRec2, entReq2)
		if entRec2.Code != http.StatusOK {
			t.Fatalf("entitlements (post-expiry) status=%d, want %d: %s", entRec2.Code, http.StatusOK, entRec2.Body.String())
		}

		var payload2 EntitlementPayload
		if err := json.NewDecoder(entRec2.Body).Decode(&payload2); err != nil {
			t.Fatalf("decode entitlements payload (post-expiry): %v", err)
		}
		if payload2.SubscriptionState != string(license.SubStateExpired) {
			t.Fatalf("subscription_state=%q, want %q", payload2.SubscriptionState, license.SubStateExpired)
		}
		if sliceContainsString(payload2.Capabilities, license.FeatureAIAutoFix) {
			t.Fatalf("expected pro-only capability %q to be removed on expiry; got %v", license.FeatureAIAutoFix, payload2.Capabilities)
		}
		for _, freeFeature := range license.TierFeatures[license.TierFree] {
			if !sliceContainsString(payload2.Capabilities, freeFeature) {
				t.Fatalf("expected free-tier feature %q to remain after expiry; got %v", freeFeature, payload2.Capabilities)
			}
		}
	})

	t.Run("One_Trial_Per_Org_Enforcement", func(t *testing.T) {
		baseDir := t.TempDir()
		mtp := config.NewMultiTenantPersistence(baseDir)
		handlers := NewLicenseHandlers(mtp, false, &config.Config{PublicURL: "https://pulse.example.com"})

		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatalf("GenerateKey: %v", err)
		}
		t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

		org1 := "trial-org-1"
		ctx1 := context.WithValue(context.Background(), OrgIDContextKey, org1)

		token1, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
			OrgID:        org1,
			Email:        "owner1@example.com",
			InstanceHost: "pulse.example.com",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			},
		})
		if err != nil {
			t.Fatalf("SignTrialActivationToken(org1): %v", err)
		}

		req1 := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(token1), nil).WithContext(ctx1)
		req1.Host = "pulse.example.com"
		rec1 := httptest.NewRecorder()
		handlers.HandleTrialActivation(rec1, req1)
		if rec1.Code != http.StatusTemporaryRedirect {
			t.Fatalf("org1 first activation status=%d, want %d: %s", rec1.Code, http.StatusTemporaryRedirect, rec1.Body.String())
		}

		token2, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
			OrgID:        org1,
			Email:        "owner1@example.com",
			InstanceHost: "pulse.example.com",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			},
		})
		if err != nil {
			t.Fatalf("SignTrialActivationToken(org1 second): %v", err)
		}

		req2 := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(token2), nil).WithContext(ctx1)
		req2.Host = "pulse.example.com"
		rec2 := httptest.NewRecorder()
		handlers.HandleTrialActivation(rec2, req2)
		if rec2.Code != http.StatusTemporaryRedirect {
			t.Fatalf("org1 second activation status=%d, want %d: %s", rec2.Code, http.StatusTemporaryRedirect, rec2.Body.String())
		}
		if got := rec2.Header().Get("Location"); got != "/settings?trial=ineligible" {
			t.Fatalf("org1 second activation redirect=%q, want %q", got, "/settings?trial=ineligible")
		}

		org2 := "trial-org-2"
		ctx2 := context.WithValue(context.Background(), OrgIDContextKey, org2)
		token3, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
			OrgID:        org2,
			Email:        "owner2@example.com",
			InstanceHost: "pulse.example.com",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
			},
		})
		if err != nil {
			t.Fatalf("SignTrialActivationToken(org2): %v", err)
		}
		req3 := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(token3), nil).WithContext(ctx2)
		req3.Host = "pulse.example.com"
		rec3 := httptest.NewRecorder()
		handlers.HandleTrialActivation(rec3, req3)
		if rec3.Code != http.StatusTemporaryRedirect {
			t.Fatalf("org2 activation status=%d, want %d: %s", rec3.Code, http.StatusTemporaryRedirect, rec3.Body.String())
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
