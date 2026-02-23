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

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry: reg,
		Version:  "test",
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

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry: reg,
		Version:  "test",
	})

	pageReq := httptest.NewRequest(http.MethodGet, "/start-pro-trial?org_id=default&return_url=https://pulse.example.com/auth/trial-activate", nil)
	pageRec := httptest.NewRecorder()
	mux.ServeHTTP(pageRec, pageReq)
	if pageRec.Code != http.StatusOK {
		t.Fatalf("GET /start-pro-trial status=%d, want %d", pageRec.Code, http.StatusOK)
	}
	if !strings.Contains(pageRec.Body.String(), "Start Your 14-Day Pulse Pro Trial") {
		t.Fatalf("expected trial signup page body")
	}

	checkoutReq := httptest.NewRequest(http.MethodGet, "/api/trial-signup/checkout", nil)
	checkoutRec := httptest.NewRecorder()
	mux.ServeHTTP(checkoutRec, checkoutReq)
	if checkoutRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/trial-signup/checkout status=%d, want %d", checkoutRec.Code, http.StatusMethodNotAllowed)
	}
}

func TestRegisterRoutes_PublicCloudSignupRoutes(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:             dir,
			AdminKey:            "test-admin-key",
			BaseURL:             "https://cloud.example.com",
			StripeWebhookSecret: "whsec_test",
		},
		Registry: reg,
		Version:  "test",
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
