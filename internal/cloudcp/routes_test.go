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
			DataDir:                  dir,
			AdminKey:                 "test-admin-key",
			BaseURL:                  "https://cloud.example.com",
			StripeWebhookSecret:      "whsec_test",
			PublicCloudSignupEnabled: true,
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

func TestRegisterRoutes_FaviconRouteParity(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:                  dir,
			AdminKey:                 "test-admin-key",
			BaseURL:                  "https://cloud.example.com",
			StripeWebhookSecret:      "whsec_test",
			PublicCloudSignupEnabled: true,
		},
		Registry: reg,
		Version:  "test",
	})

	svgReq := httptest.NewRequest(http.MethodGet, "/favicon.svg", nil)
	svgRec := httptest.NewRecorder()
	mux.ServeHTTP(svgRec, svgReq)
	if svgRec.Code != http.StatusOK {
		t.Fatalf("GET /favicon.svg status=%d, want %d", svgRec.Code, http.StatusOK)
	}
	if got := svgRec.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Fatalf("GET /favicon.svg content-type=%q, want %q", got, "image/svg+xml")
	}
	body := svgRec.Body.String()
	if !strings.Contains(body, "<svg") || !strings.Contains(body, "rx=\"16\"") {
		t.Fatalf("expected current favicon svg payload")
	}

	icoReq := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	icoRec := httptest.NewRecorder()
	mux.ServeHTTP(icoRec, icoReq)
	if icoRec.Code != http.StatusMovedPermanently {
		t.Fatalf("GET /favicon.ico status=%d, want %d", icoRec.Code, http.StatusMovedPermanently)
	}
	if got := icoRec.Header().Get("Location"); got != "/favicon.svg" {
		t.Fatalf("GET /favicon.ico location=%q, want %q", got, "/favicon.svg")
	}
	if got := controlPlaneFaviconHref(); !strings.HasPrefix(got, "/favicon.svg?v=") {
		t.Fatalf("controlPlaneFaviconHref=%q, want versioned favicon href", got)
	}
}

func TestRegisterRoutes_RetiredTrialSignupRoutes(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:                  dir,
			AdminKey:                 "test-admin-key",
			BaseURL:                  "https://cloud.example.com",
			StripeWebhookSecret:      "whsec_test",
			PublicCloudSignupEnabled: true,
		},
		Registry: reg,
		Version:  "test",
	})

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/start-pro-trial?org_id=default&return_url=https://pulse.example.com/auth/trial-activate&instance_token=tsi_test"},
		{method: http.MethodPost, path: "/api/trial-signup/request-verification"},
		{method: http.MethodGet, path: "/trial-signup/verify"},
		{method: http.MethodPost, path: "/api/trial-signup/checkout"},
		{method: http.MethodGet, path: "/trial-signup/complete"},
		{method: http.MethodPost, path: "/api/trial-signup/redeem"},
		{method: http.MethodPost, path: "/api/trial-signup/refresh"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s status=%d, want %d", tc.method, tc.path, rec.Code, http.StatusNotFound)
		}
	}

	hostedRefreshReq := httptest.NewRequest(http.MethodGet, "/api/entitlements/refresh", nil)
	hostedRefreshRec := httptest.NewRecorder()
	mux.ServeHTTP(hostedRefreshRec, hostedRefreshReq)
	if hostedRefreshRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/entitlements/refresh status=%d, want %d", hostedRefreshRec.Code, http.StatusMethodNotAllowed)
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
			DataDir:                  dir,
			AdminKey:                 "test-admin-key",
			BaseURL:                  "https://cloud.example.com",
			StripeWebhookSecret:      "whsec_test",
			PublicCloudSignupEnabled: true,
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

func TestRegisterRoutes_PublicMSPSignupRoutes(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	mux := http.NewServeMux()
	RegisterRoutes(mux, &Deps{
		Config: &CPConfig{
			DataDir:                  dir,
			AdminKey:                 "test-admin-key",
			BaseURL:                  "https://cloud.example.com",
			StripeWebhookSecret:      "whsec_test",
			PublicCloudSignupEnabled: true,
		},
		Registry: reg,
		Version:  "test",
	})

	mspPageReq := httptest.NewRequest(http.MethodGet, "/cloud/msp/signup", nil)
	mspPageRec := httptest.NewRecorder()
	mux.ServeHTTP(mspPageRec, mspPageReq)
	if mspPageRec.Code != http.StatusOK {
		t.Fatalf("GET /cloud/msp/signup status=%d, want %d", mspPageRec.Code, http.StatusOK)
	}
	if !strings.Contains(mspPageRec.Body.String(), "Pulse Cloud for MSPs") {
		t.Fatalf("expected public MSP signup page body")
	}

	mspCompleteReq := httptest.NewRequest(http.MethodGet, "/cloud/msp/signup/complete", nil)
	mspCompleteRec := httptest.NewRecorder()
	mux.ServeHTTP(mspCompleteRec, mspCompleteReq)
	if mspCompleteRec.Code != http.StatusOK {
		t.Fatalf("GET /cloud/msp/signup/complete status=%d, want %d", mspCompleteRec.Code, http.StatusOK)
	}

	mspAPIGetReq := httptest.NewRequest(http.MethodGet, "/api/public/msp/signup", nil)
	mspAPIGetRec := httptest.NewRecorder()
	mux.ServeHTTP(mspAPIGetRec, mspAPIGetReq)
	if mspAPIGetRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/public/msp/signup status=%d, want %d", mspAPIGetRec.Code, http.StatusMethodNotAllowed)
	}
}

func TestRegisterRoutes_PublicCloudSignupRoutesDisabledByDefault(t *testing.T) {
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

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/signup"},
		{method: http.MethodGet, path: "/cloud/signup"},
		{method: http.MethodGet, path: "/signup/complete"},
		{method: http.MethodPost, path: "/api/public/signup"},
		{method: http.MethodGet, path: "/cloud/msp/signup"},
		{method: http.MethodGet, path: "/cloud/msp/signup/complete"},
		{method: http.MethodPost, path: "/api/public/msp/signup"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s status=%d, want %d", tc.method, tc.path, rec.Code, http.StatusNotFound)
		}
	}

	publicMagicLinkReq := httptest.NewRequest(http.MethodPost, "/api/public/magic-link/request", strings.NewReader(`{"email":"owner@example.com"}`))
	publicMagicLinkReq.Header.Set("Content-Type", "application/json")
	publicMagicLinkRec := httptest.NewRecorder()
	mux.ServeHTTP(publicMagicLinkRec, publicMagicLinkReq)
	if publicMagicLinkRec.Code != http.StatusOK {
		t.Fatalf("POST /api/public/magic-link/request status=%d, want %d", publicMagicLinkRec.Code, http.StatusOK)
	}
}
