package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// ssoTestIPCounter generates unique client IPs to avoid rate-limiter
// interference between test requests (authLimiter in identity_sso_handlers.go).
var ssoTestIPCounter atomic.Int64

func ssoTestIP() string {
	n := ssoTestIPCounter.Add(1)
	return fmt.Sprintf("10.99.%d.%d:12345", (n>>8)&0xFF, n&0xFF)
}

// TestSAMLAdminEndpointsRequireAdvancedSSOLicense verifies that the SSO admin
// endpoints return 402 Payment Required when a user attempts SAML operations
// without a Pro license. OIDC SSO is free-tier, but SAML is an advanced_sso
// feature requiring Pro.
//
// The SSO admin routes are gated at the route level by featureSSOKey ("sso"),
// which is free-tier and passes without a license. The inner handlers
// additionally check featureAdvancedSSOKey ("advanced_sso") when the request
// involves SAML. This test verifies those defense-in-depth checks:
//
//   - handleCreateSSOProvider (POST /api/security/sso/providers) — line 247
//   - handleUpdateSSOProvider (PUT /api/security/sso/providers/{id}) — line 373
//   - handleTestSSOProvider (POST /api/security/sso/providers/test) — line 694
//   - handleMetadataPreview (POST /api/security/sso/providers/metadata/preview) — line 1002
func TestSAMLAdminEndpointsRequireAdvancedSSOLicense(t *testing.T) {
	rawToken := "sso-admin-license-test-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	// Test 1: POST /api/security/sso/providers with type=saml → 402
	t.Run("CreateSAMLProvider", func(t *testing.T) {
		body := `{"name":"test-saml","type":"saml","saml":{"metadataUrl":"https://idp.example.com/metadata"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers", strings.NewReader(body))
		req.RemoteAddr = ssoTestIP()
		req.Header.Set("X-API-Token", rawToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusPaymentRequired {
			t.Errorf("POST /api/security/sso/providers (saml): expected 402, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		assertAdvancedSSOResponse(t, rec)
	})

	// Test 2: PUT /api/security/sso/providers/{id} with type=saml → 402
	// First create an OIDC provider (OIDC is free-tier), then try to update it to SAML.
	t.Run("UpdateToSAMLProvider", func(t *testing.T) {
		// Create an OIDC provider first
		createBody := `{"id":"test-oidc-for-update","name":"test-oidc","type":"oidc","oidc":{"issuerUrl":"https://auth.example.com","clientId":"test","clientSecret":"secret","redirectUrl":"https://pulse.example.com/callback"}}`
		createReq := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers", strings.NewReader(createBody))
		createReq.RemoteAddr = ssoTestIP()
		createReq.Header.Set("X-API-Token", rawToken)
		createReq.Header.Set("Content-Type", "application/json")
		createRec := httptest.NewRecorder()
		handler.ServeHTTP(createRec, createReq)

		if createRec.Code != http.StatusCreated {
			t.Fatalf("setup: failed to create OIDC provider, got %d (body: %s)", createRec.Code, createRec.Body.String())
		}

		// Now try to update it to SAML type
		updateBody := `{"name":"test-saml-updated","type":"saml","saml":{"metadataUrl":"https://idp.example.com/metadata"}}`
		req := httptest.NewRequest(http.MethodPut, "/api/security/sso/providers/test-oidc-for-update", strings.NewReader(updateBody))
		req.RemoteAddr = ssoTestIP()
		req.Header.Set("X-API-Token", rawToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusPaymentRequired {
			t.Errorf("PUT /api/security/sso/providers/{id} (saml): expected 402, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		assertAdvancedSSOResponse(t, rec)
	})

	// Test 3: POST /api/security/sso/providers/test with type=saml → 402
	t.Run("TestSAMLConnection", func(t *testing.T) {
		body := `{"type":"saml","saml":{"metadataUrl":"https://idp.example.com/metadata"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", strings.NewReader(body))
		req.RemoteAddr = ssoTestIP()
		req.Header.Set("X-API-Token", rawToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusPaymentRequired {
			t.Errorf("POST /api/security/sso/providers/test (saml): expected 402, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		assertAdvancedSSOResponse(t, rec)
	})

	// Test 4: POST /api/security/sso/providers/metadata/preview with type=saml → 402
	t.Run("PreviewSAMLMetadata", func(t *testing.T) {
		body := `{"type":"saml","metadataUrl":"https://idp.example.com/metadata"}`
		req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/metadata/preview", strings.NewReader(body))
		req.RemoteAddr = ssoTestIP()
		req.Header.Set("X-API-Token", rawToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusPaymentRequired {
			t.Errorf("POST /api/security/sso/providers/metadata/preview (saml): expected 402, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		assertAdvancedSSOResponse(t, rec)
	})
}

// TestOIDCAdminEndpointsAllowedWithoutProLicense verifies that OIDC operations
// through the same SSO admin endpoints succeed without a Pro license. OIDC is
// free-tier and should NOT be blocked by the advanced_sso gate.
func TestOIDCAdminEndpointsAllowedWithoutProLicense(t *testing.T) {
	rawToken := "sso-admin-oidc-test-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	// OIDC create should succeed with 201 Created (not 402)
	body := `{"name":"test-oidc","type":"oidc","oidc":{"issuerUrl":"https://auth.example.com","clientId":"test","clientSecret":"secret","redirectUrl":"https://pulse.example.com/callback"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers", strings.NewReader(body))
	req.RemoteAddr = ssoTestIP()
	req.Header.Set("X-API-Token", rawToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("POST /api/security/sso/providers (oidc): expected 201 Created, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	// OIDC test-connection should not be license-gated. The handler makes a
	// real network call (which will fail), so we accept any status except 402.
	testBody := `{"type":"oidc","oidc":{"issuerUrl":"https://auth.example.com","clientId":"test","clientSecret":"secret"}}`
	testReq := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", strings.NewReader(testBody))
	testReq.RemoteAddr = ssoTestIP()
	testReq.Header.Set("X-API-Token", rawToken)
	testReq.Header.Set("Content-Type", "application/json")
	testRec := httptest.NewRecorder()
	handler.ServeHTTP(testRec, testReq)

	if testRec.Code == http.StatusPaymentRequired {
		t.Errorf("POST /api/security/sso/providers/test (oidc): should NOT get 402 for OIDC, got 402 (body: %s)", testRec.Body.String())
	}
}

// TestSAMLAdminLicenseGatingResponseFormat verifies that the 402 response from
// SAML admin endpoints includes a JSON body with the advanced_sso feature key
// and an upgrade URL, matching the canonical WriteLicenseRequired format.
func TestSAMLAdminLicenseGatingResponseFormat(t *testing.T) {
	rawToken := "sso-admin-format-test-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	body := `{"name":"format-test","type":"saml","saml":{"metadataUrl":"https://idp.example.com/metadata"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers", strings.NewReader(body))
	req.RemoteAddr = ssoTestIP()
	req.Header.Set("X-API-Token", rawToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type to contain application/json, got %q", ct)
	}

	respBody := rec.Body.String()
	if !strings.Contains(respBody, "advanced_sso") {
		t.Errorf("expected body to reference advanced_sso feature, got: %s", respBody)
	}
	if !strings.Contains(respBody, "upgrade_url") {
		t.Errorf("expected body to contain upgrade_url, got: %s", respBody)
	}
	if !strings.Contains(respBody, "SAML SSO requires a Pro license") {
		t.Errorf("expected body to contain SAML-specific message, got: %s", respBody)
	}
}

// assertAdvancedSSOResponse checks that a 402 response references the
// advanced_sso feature in its body.
func assertAdvancedSSOResponse(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusPaymentRequired {
		return // Primary status check already logged by caller
	}
	body := rec.Body.String()
	if !strings.Contains(body, "advanced_sso") {
		t.Errorf("402 body should reference advanced_sso feature, got: %s", body)
	}
}
