package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// TestSAMLRuntimeEndpointsRequireLicenseFeature verifies that the SAML
// login-flow endpoints (/api/saml/{id}/login, /acs, /metadata, /logout, /slo)
// return 402 Payment Required when no Pro license is active. SAML is an
// advanced_sso feature and must not be usable on the free tier.
//
// Covers assessment gap #1: "SAML runtime endpoints have NO advanced_sso
// license enforcement."
func TestSAMLRuntimeEndpointsRequireLicenseFeature(t *testing.T) {
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

		if rec.Code != http.StatusPaymentRequired {
			t.Errorf("%s %s: expected 402 for missing advanced_sso license, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

// TestSAMLRuntimeLicenseGatingResponseFormat verifies that the 402 response
// from SAML runtime endpoints includes a JSON body with upgrade information
// referencing the advanced_sso feature.
func TestSAMLRuntimeLicenseGatingResponseFormat(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{DataPath: tempDir, ConfigPath: tempDir}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/saml/test-provider/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type to contain application/json, got %q", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "advanced_sso") {
		t.Errorf("expected 402 body to reference advanced_sso feature, got: %s", body)
	}
	if !strings.Contains(body, "upgrade_url") {
		t.Errorf("expected 402 body to contain upgrade_url, got: %s", body)
	}
}
