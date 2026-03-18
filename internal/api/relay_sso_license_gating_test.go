package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// TestRelayEndpointsRequireLicenseFeature verifies that relay settings endpoints
// return 402 Payment Required when no Pro license is active. Relay is a paid
// feature (FeatureRelay) and must not be accessible on the free tier.
//
// Covers acceptance test checklist items:
//   - 6.1: "Relay/mobile NOT available" without license
//   - 10.1: relay setup requires Pro
func TestRelayEndpointsRequireLicenseFeature(t *testing.T) {
	rawToken := "relay-license-test-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/settings/relay"},
		{http.MethodGet, "/api/settings/relay/status"},
		{http.MethodPut, "/api/settings/relay"},
	}

	for _, tc := range cases {
		var req *http.Request
		if tc.method == http.MethodPut {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"enabled":true}`))
		} else {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		}
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusPaymentRequired {
			t.Errorf("%s %s: expected 402 for missing relay license, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

// TestRelayOnboardingEndpointsRequireLicenseFeature verifies that relay
// onboarding endpoints (QR code, connection validation, deep-link) return
// 402 without a Pro license. These endpoints are wrapped with
// RequireLicenseFeature(featureRelayKey) in router_routes_ai_relay.go.
//
// Covers acceptance test checklist items:
//   - 10.2: QR code and mobile pairing require Pro
func TestRelayOnboardingEndpointsRequireLicenseFeature(t *testing.T) {
	rawToken := "relay-onboarding-license-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/onboarding/qr"},
		{http.MethodPost, "/api/onboarding/validate"},
		{http.MethodGet, "/api/onboarding/deep-link"},
	}

	for _, tc := range cases {
		var req *http.Request
		if tc.method == http.MethodPost {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		} else {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		}
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusPaymentRequired {
			t.Errorf("%s %s: expected 402 for missing relay license, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

// TestRelayLicenseGatingResponseFormat verifies that the 402 response from
// relay endpoints includes a JSON body with upgrade information. This ensures
// the frontend can show an appropriate upgrade prompt.
func TestRelayLicenseGatingResponseFormat(t *testing.T) {
	rawToken := "relay-format-test-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/settings/relay", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rec.Code)
	}

	// Verify the response is JSON.
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type to contain application/json, got %q", ct)
	}

	// Verify the body contains upgrade information.
	body := rec.Body.String()
	if !strings.Contains(body, "license") && !strings.Contains(body, "upgrade") {
		t.Errorf("expected 402 body to contain license/upgrade info, got: %s", body)
	}
}
