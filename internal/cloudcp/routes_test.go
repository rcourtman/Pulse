package cloudcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestRegisterRoutes_AccountAndTenantMethodDispatch(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	trialStore, err := NewTrialSignupStore(dir)
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { trialStore.Close() })

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry:         reg,
		TrialSignupStore: trialStore,
		Version:          "test",
	})

	tests := []struct {
		name string
		path string
		want int
	}{
		// /members collection handler: GET/POST supported, others rejected.
		{name: "members-get-dispatches", path: "/api/accounts/acct_1/members", want: http.StatusNotFound},
		{name: "members-post-dispatches", path: "/api/accounts/acct_1/members", want: http.StatusNotFound},
		{name: "members-put-rejected", path: "/api/accounts/acct_1/members", want: http.StatusMethodNotAllowed},

		// /members/{user_id} handler: PATCH/DELETE supported, others rejected.
		{name: "member-patch-dispatches", path: "/api/accounts/acct_1/members/user_1", want: http.StatusNotFound},
		{name: "member-delete-dispatches", path: "/api/accounts/acct_1/members/user_1", want: http.StatusNotFound},
		{name: "member-get-rejected", path: "/api/accounts/acct_1/members/user_1", want: http.StatusMethodNotAllowed},

		// /tenants collection handler: GET/POST supported, others rejected.
		{name: "tenants-get-dispatches", path: "/api/accounts/acct_1/tenants", want: http.StatusNotFound},
		{name: "tenants-post-dispatches", path: "/api/accounts/acct_1/tenants", want: http.StatusNotFound},
		{name: "tenants-put-rejected", path: "/api/accounts/acct_1/tenants", want: http.StatusMethodNotAllowed},

		// /tenants/{tenant_id} handler: PATCH/DELETE supported, others rejected.
		{name: "tenant-patch-dispatches", path: "/api/accounts/acct_1/tenants/tenant_1", want: http.StatusNotFound},
		{name: "tenant-delete-dispatches", path: "/api/accounts/acct_1/tenants/tenant_1", want: http.StatusNotFound},
		{name: "tenant-get-rejected", path: "/api/accounts/acct_1/tenants/tenant_1", want: http.StatusMethodNotAllowed},
	}

	methodFor := map[string]string{
		"members-get-dispatches":   http.MethodGet,
		"members-post-dispatches":  http.MethodPost,
		"members-put-rejected":     http.MethodPut,
		"member-patch-dispatches":  http.MethodPatch,
		"member-delete-dispatches": http.MethodDelete,
		"member-get-rejected":      http.MethodGet,
		"tenants-get-dispatches":   http.MethodGet,
		"tenants-post-dispatches":  http.MethodPost,
		"tenants-put-rejected":     http.MethodPut,
		"tenant-patch-dispatches":  http.MethodPatch,
		"tenant-delete-dispatches": http.MethodDelete,
		"tenant-get-rejected":      http.MethodGet,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(methodFor[tt.name], tt.path, nil)
			req.Header.Set("X-Admin-Key", "test-admin-key")

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("%s %s status = %d, want %d (body=%q)",
					methodFor[tt.name], tt.path, rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}

func TestRegisterRoutes_TrialSignupRoutes(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	trialStore, err := NewTrialSignupStore(dir)
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { trialStore.Close() })

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry:         reg,
		TrialSignupStore: trialStore,
		Version:          "test",
	})

	pageReq := httptest.NewRequest(http.MethodGet, "/start-pro-trial?org_id=default&return_url=https://pulse.example.com/auth/trial-activate&instance_token=tsi_test", nil)
	pageRec := httptest.NewRecorder()
	mux.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("GET /start-pro-trial status=%d, want %d", pageRec.Code, http.StatusOK)
	}
	if !strings.Contains(pageRec.Body.String(), "Continue to secure checkout") {
		t.Fatalf("expected trial signup page body")
	}

	verifyReq := httptest.NewRequest(http.MethodGet, "/trial-signup/verify", nil)
	verifyRec := httptest.NewRecorder()
	mux.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusBadRequest {
		t.Fatalf("GET /trial-signup/verify status=%d, want %d", verifyRec.Code, http.StatusBadRequest)
	}

	requestVerificationReq := httptest.NewRequest(http.MethodGet, "/api/trial-signup/request-verification", nil)
	requestVerificationRec := httptest.NewRecorder()
	mux.ServeHTTP(requestVerificationRec, requestVerificationReq)
	if requestVerificationRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/trial-signup/request-verification status=%d, want %d", requestVerificationRec.Code, http.StatusMethodNotAllowed)
	}

	checkoutReq := httptest.NewRequest(http.MethodGet, "/api/trial-signup/checkout", nil)
	checkoutRec := httptest.NewRecorder()
	mux.ServeHTTP(checkoutRec, checkoutReq)
	if checkoutRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/trial-signup/checkout status=%d, want %d", checkoutRec.Code, http.StatusMethodNotAllowed)
	}

	redeemReq := httptest.NewRequest(http.MethodGet, "/api/trial-signup/redeem", nil)
	redeemRec := httptest.NewRecorder()
	mux.ServeHTTP(redeemRec, redeemReq)
	if redeemRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/trial-signup/redeem status=%d, want %d", redeemRec.Code, http.StatusMethodNotAllowed)
	}

	refreshReq := httptest.NewRequest(http.MethodGet, "/api/trial-signup/refresh", nil)
	refreshRec := httptest.NewRecorder()
	mux.ServeHTTP(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/trial-signup/refresh status=%d, want %d", refreshRec.Code, http.StatusMethodNotAllowed)
	}

	hostedRefreshReq := httptest.NewRequest(http.MethodGet, "/api/entitlements/refresh", nil)
	hostedRefreshRec := httptest.NewRecorder()
	mux.ServeHTTP(hostedRefreshRec, hostedRefreshReq)
	if hostedRefreshRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/entitlements/refresh status=%d, want %d", hostedRefreshRec.Code, http.StatusMethodNotAllowed)
	}
}

func TestRegisterRoutes_TrialSignupVerificationRateLimit(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	trialStore, err := NewTrialSignupStore(dir)
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { trialStore.Close() })

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry:         reg,
		TrialSignupStore: trialStore,
		Version:          "test",
	})

	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/trial-signup/request-verification", nil)
		req.RemoteAddr = "198.51.100.25:7777"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("attempt %d status=%d, want %d", i+1, rec.Code, http.StatusMethodNotAllowed)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/trial-signup/request-verification", nil)
	req.RemoteAddr = "198.51.100.25:7777"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limited status=%d, want %d body=%q", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
}

func TestRegisterRoutes_PublicCloudSignupRoutes(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	trialStore, err := NewTrialSignupStore(dir)
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { trialStore.Close() })

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry:         reg,
		TrialSignupStore: trialStore,
		Version:          "test",
	})

	signupPageReq := httptest.NewRequest(http.MethodGet, "/signup", nil)
	signupPageRec := httptest.NewRecorder()
	mux.ServeHTTP(signupPageRec, signupPageReq)
	if signupPageRec.Code != http.StatusOK {
		t.Fatalf("GET /signup status=%d, want %d", signupPageRec.Code, http.StatusOK)
	}
	if !strings.Contains(signupPageRec.Body.String(), "Start Pulse Cloud") {
		t.Fatalf("expected public signup page body")
	}

	cloudSignupPageReq := httptest.NewRequest(http.MethodGet, "/cloud/signup", nil)
	cloudSignupPageRec := httptest.NewRecorder()
	mux.ServeHTTP(cloudSignupPageRec, cloudSignupPageReq)
	if cloudSignupPageRec.Code != http.StatusOK {
		t.Fatalf("GET /cloud/signup status=%d, want %d", cloudSignupPageRec.Code, http.StatusOK)
	}

	publicSignupGetReq := httptest.NewRequest(http.MethodGet, "/api/public/signup", nil)
	publicSignupGetRec := httptest.NewRecorder()
	mux.ServeHTTP(publicSignupGetRec, publicSignupGetReq)
	if publicSignupGetRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/public/signup status=%d, want %d", publicSignupGetRec.Code, http.StatusMethodNotAllowed)
	}

	publicMagicLinkReq := httptest.NewRequest(http.MethodPost, "/api/public/magic-link/request", strings.NewReader(`{"email":"owner@example.com"}`))
	publicMagicLinkReq.Header.Set("Content-Type", "application/json")
	publicMagicLinkRec := httptest.NewRecorder()
	mux.ServeHTTP(publicMagicLinkRec, publicMagicLinkReq)
	if publicMagicLinkRec.Code != http.StatusOK {
		t.Fatalf("POST /api/public/magic-link/request status=%d, want %d", publicMagicLinkRec.Code, http.StatusOK)
	}
}
