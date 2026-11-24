package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestRegisterHostedRoutes_HandoffExchangeRateLimited(t *testing.T) {
	router := &Router{
		mux:                        http.NewServeMux(),
		config:                     &config.Config{DataPath: t.TempDir()},
		hostedMode:                 true,
		handoffExchangeRateLimiter: NewRateLimiter(1, time.Hour),
	}
	t.Cleanup(func() {
		if router.handoffExchangeRateLimiter != nil {
			router.handoffExchangeRateLimiter.Stop()
		}
	})

	router.registerHostedRoutes(nil, nil, nil)

	req1 := httptest.NewRequest(http.MethodPost, "/api/cloud/handoff/exchange?format=json", strings.NewReader(""))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req1.RemoteAddr = "198.51.100.30:443"
	rec1 := httptest.NewRecorder()
	router.mux.ServeHTTP(rec1, req1)
	if rec1.Code == http.StatusTooManyRequests {
		t.Fatalf("first request unexpectedly rate-limited: status=%d body=%q", rec1.Code, rec1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/cloud/handoff/exchange?format=json", strings.NewReader(""))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.RemoteAddr = "198.51.100.30:443"
	rec2 := httptest.NewRecorder()
	router.mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d (body=%q)", rec2.Code, http.StatusTooManyRequests, rec2.Body.String())
	}
}

// The checkout start mints the return token that the public
// /auth/license-purchase-activate route redeems into a license activation, so
// it is held to the admission of /api/license/activate: an administrator, and
// settings:write for an API token.
func TestLicensePurchaseStartRequiresLicenseActivationAuthority(t *testing.T) {
	var handoffs atomic.Int32
	licenseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handoffs.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"portal_handoff_id":"cph_route_authority"}`))
	}))
	defer licenseServer.Close()
	t.Setenv("PULSE_LICENSE_SERVER_URL", licenseServer.URL)

	for _, tc := range []struct {
		name         string
		scope        string
		anonymous    bool
		wantStatus   int
		wantHandoffs int32
	}{
		{name: "anonymous", scope: config.ScopeSettingsWrite, anonymous: true, wantStatus: http.StatusUnauthorized},
		{name: "agent report token", scope: config.ScopeAgentReport, wantStatus: http.StatusForbidden},
		{name: "settings read token", scope: config.ScopeSettingsRead, wantStatus: http.StatusForbidden},
		{name: "settings write token", scope: config.ScopeSettingsWrite, wantStatus: http.StatusSeeOther, wantHandoffs: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handoffs.Store(0)
			rawToken := "license-purchase-start-token-123.12345678"
			cfg := newTestConfigWithTokens(t, newTokenRecord(t, rawToken, []string{tc.scope}, nil))
			router := newTestRouter(t, cfg, nil, nil, nil, nil, "1.0.0")

			req := httptest.NewRequest(http.MethodGet, licensePurchaseStartPath+"?feature=relay", nil)
			if !tc.anonymous {
				req.Header.Set("X-API-Token", rawToken)
			}
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusForbidden {
				assertMissingScope(t, rec, config.ScopeSettingsWrite, licensePurchaseStartPath)
			}
			if got := handoffs.Load(); got != tc.wantHandoffs {
				t.Fatalf("checkout handoffs created = %d, want %d", got, tc.wantHandoffs)
			}
		})
	}
}
