package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
