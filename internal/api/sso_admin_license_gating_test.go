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

// TestSAMLAdminEndpointsAllowedWithoutPaidLicense verifies that the SSO admin
// endpoints do not return 402 Payment Required when a user attempts SAML
// operations without a paid license. OIDC, SAML, and multi-provider SSO are
// Community-tier SSO capabilities.
//
// The SSO admin routes are gated at the route level by featureSSOKey ("sso"),
// which is included on the default self-hosted Community tier.
//
//   - handleCreateSSOProvider (POST /api/security/sso/providers)
//   - handleUpdateSSOProvider (PUT /api/security/sso/providers/{id})
//   - handleTestSSOProvider (POST /api/security/sso/providers/test)
//   - handleMetadataPreview (POST /api/security/sso/providers/metadata/preview)
func TestSAMLAdminEndpointsAllowedWithoutPaidLicense(t *testing.T) {
	rawToken := "sso-admin-license-test-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	// Test 1: POST /api/security/sso/providers with type=saml should create.
	t.Run("CreateSAMLProvider", func(t *testing.T) {
		body := `{"id":"test-saml-create","name":"test-saml","type":"saml","saml":{"idpMetadataUrl":"https://idp.example.com/metadata"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers", strings.NewReader(body))
		req.RemoteAddr = ssoTestIP()
		req.Header.Set("X-API-Token", rawToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("POST /api/security/sso/providers (saml): expected 201, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	// Test 2: PUT /api/security/sso/providers/{id} with type=saml should update.
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

		// Now update it to SAML type.
		updateBody := `{"name":"test-saml-updated","type":"saml","saml":{"idpMetadataUrl":"https://idp.example.com/metadata"}}`
		req := httptest.NewRequest(http.MethodPut, "/api/security/sso/providers/test-oidc-for-update", strings.NewReader(updateBody))
		req.RemoteAddr = ssoTestIP()
		req.Header.Set("X-API-Token", rawToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("PUT /api/security/sso/providers/{id} (saml): expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	// Test 3: POST /api/security/sso/providers/test with type=saml should reach validation.
	t.Run("TestSAMLConnection", func(t *testing.T) {
		body := `{"type":"saml","saml":{"idpMetadataUrl":"https://idp.example.com/metadata"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/test", strings.NewReader(body))
		req.RemoteAddr = ssoTestIP()
		req.Header.Set("X-API-Token", rawToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusPaymentRequired {
			t.Errorf("POST /api/security/sso/providers/test (saml): should not get 402 for SAML, got body: %s", rec.Body.String())
		}
	})

	// Test 4: POST /api/security/sso/providers/metadata/preview with type=saml should reach validation.
	t.Run("PreviewSAMLMetadata", func(t *testing.T) {
		body := `{"type":"saml","metadataUrl":"https://idp.example.com/metadata"}`
		req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers/metadata/preview", strings.NewReader(body))
		req.RemoteAddr = ssoTestIP()
		req.Header.Set("X-API-Token", rawToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusPaymentRequired {
			t.Errorf("POST /api/security/sso/providers/metadata/preview (saml): should not get 402 for SAML, got body: %s", rec.Body.String())
		}
	})
}

// TestOIDCAdminEndpointsAllowedWithoutProLicense verifies that OIDC operations
// through the same SSO admin endpoints succeed without a Pro license. OIDC is
// part of the same Community SSO contract as SAML.
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

// TestSAMLAdminCreateDoesNotEmitUpgradeResponse verifies SAML creation does not
// produce the old upgrade response shape.
func TestSAMLAdminCreateDoesNotEmitUpgradeResponse(t *testing.T) {
	rawToken := "sso-admin-format-test-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	body := `{"id":"format-test","name":"format-test","type":"saml","saml":{"idpMetadataUrl":"https://idp.example.com/metadata"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers", strings.NewReader(body))
	req.RemoteAddr = ssoTestIP()
	req.Header.Set("X-API-Token", rawToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type to contain application/json, got %q", ct)
	}

	respBody := rec.Body.String()
	if strings.Contains(respBody, "upgrade_url") {
		t.Errorf("SAML create returned old upgrade response shape: %s", respBody)
	}
}
