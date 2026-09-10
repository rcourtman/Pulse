package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// Browser contexts on the E2E Docker bridge share an IP, not a rate budget
// per cookie. Exercise both bootstrap endpoints observed returning 429 in CI.
func TestJourneyGeneralAPIBudgetIsolation(t *testing.T) {
	for _, tc := range []struct {
		name, dev string
		limit     int
	}{
		{"production retains enforcement", "", 500},
		{"development harness remains bounded", "true", 5000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PULSE_DEV", tc.dev)
			t.Setenv("PULSE_DEV_GENERAL_API_RATE_LIMIT", "5000")
			cfg := newEndpointRateLimitConfig()
			for _, limiter := range []*RateLimiter{cfg.AuthEndpoints, cfg.ConfigEndpoints,
				cfg.ExportEndpoints, cfg.RecoveryEndpoints, cfg.UpdateEndpoints,
				cfg.WebSocketEndpoints, cfg.GeneralAPI, cfg.PublicEndpoints} {
				t.Cleanup(limiter.Stop)
			}
			handler := UniversalRateLimitMiddlewareWithConfig(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			request := func(endpoint string, n int) *httptest.ResponseRecorder {
				r := httptest.NewRequest(http.MethodGet, endpoint, nil)
				r.RemoteAddr = "192.0.2.17:12345" // not the direct-loopback exemption
				r.AddCookie(&http.Cookie{Name: "pulse_session", Value: "synthetic-" + strconv.Itoa(n)})
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				return w
			}
			endpoints := []string{"/api/orgs", "/api/license/runtime-capabilities"}
			for i := 0; i < tc.limit; i++ {
				if w := request(endpoints[i%2], i); w.Code != http.StatusNoContent {
					t.Fatalf("request %d: status %d, want 204", i+1, w.Code)
				}
			}
			for _, endpoint := range endpoints {
				w := request(endpoint, tc.limit)
				if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") != "60" ||
					w.Header().Get("X-RateLimit-Limit") != strconv.Itoa(tc.limit) {
					t.Fatalf("exhausted %s: status %d, headers %v", endpoint, w.Code, w.Header())
				}
			}
			// Raising the development general budget must not raise the login budget.
			for i := 0; i < 11; i++ {
				want := http.StatusNoContent
				if i == 10 {
					want = http.StatusTooManyRequests
				}
				if w := request("/api/login", i); w.Code != want {
					t.Fatalf("login request %d: status %d, want %d", i+1, w.Code, want)
				}
			}
		})
	}
}
