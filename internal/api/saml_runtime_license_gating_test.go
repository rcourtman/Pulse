package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// TestSAMLRuntimeEndpointsUseCommunitySSOFeature verifies that the SAML
// login-flow endpoints (/api/saml/{id}/login, /acs, /metadata, /logout, /slo)
// do not return the old paid-feature 402 when no paid license is active. SAML
// is part of the Community SSO capability.
func TestSAMLRuntimeEndpointsUseCommunitySSOFeature(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{DataPath: tempDir, ConfigPath: tempDir}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/saml/test-provider/login"},
		{http.MethodPost, "/api/saml/test-provider/acs"},
		{http.MethodGet, "/api/saml/test-provider/metadata"},
		{http.MethodGet, "/api/saml/test-provider/logout"},
		{http.MethodPost, "/api/saml/test-provider/slo"},
	}

	for _, tc := range cases {
		var req *http.Request
		if tc.method == http.MethodPost {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(`<samlp:Response/>`))
		} else {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusPaymentRequired {
			t.Errorf("%s %s: should not get 402 for Community SSO, got body: %s", tc.method, tc.path, rec.Body.String())
		}
	}
}

// TestSAMLRuntimeNoUpgradeResponse verifies SAML runtime endpoints do not emit
// upgrade metadata when the provider is missing.
func TestSAMLRuntimeNoUpgradeResponse(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{DataPath: tempDir, ConfigPath: tempDir}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/saml/test-provider/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusPaymentRequired {
		t.Fatalf("expected non-402 for Community SSO, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if strings.Contains(body, "upgrade_url") || strings.Contains(body, "advanced_sso") {
		t.Errorf("SAML runtime returned old upgrade response body: %s", body)
	}
}
