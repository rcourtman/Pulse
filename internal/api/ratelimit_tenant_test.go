package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTenantRateLimiterPerOrgLimiting(t *testing.T) {
	trl := NewTenantRateLimiter(3, time.Minute)
	defer trl.Stop()

	for i := 0; i < 3; i++ {
		if !trl.Allow("org-a") {
			t.Fatalf("org-a request %d should be allowed", i+1)
		}
	}

	if trl.Allow("org-a") {
		t.Fatal("org-a request 4 should be denied")
	}

	if !trl.Allow("org-b") {
		t.Fatal("org-b should have an independent limit and be allowed")
	}
}

func TestTenantRateLimitMiddlewareBlocks(t *testing.T) {
	trl := NewTenantRateLimiter(2, time.Minute)
	defer trl.Stop()

	h := TenantRateLimitMiddleware(trl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 1; i <= 2; i++ {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, requestWithOrg("test-org"))
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want %d", i, rr.Code, http.StatusOK)
		}
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, requestWithOrg("test-org"))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked request status = %d, want %d", rr.Code, http.StatusTooManyRequests)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}
	if body["error"] != "tenant_rate_limit_exceeded" {
		t.Fatalf("error code = %q, want %q", body["error"], "tenant_rate_limit_exceeded")
	}
}

func TestTenantRateLimitMiddlewareDefaultOrgExempt(t *testing.T) {
	trl := NewTenantRateLimiter(1, time.Minute)
	defer trl.Stop()

	h := TenantRateLimitMiddleware(trl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 1; i <= 5; i++ {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, requestWithOrg("default"))
		if rr.Code != http.StatusOK {
			t.Fatalf("default org request %d status = %d, want %d", i, rr.Code, http.StatusOK)
		}
	}
}

func TestTenantRateLimitMiddlewareNilSafe(t *testing.T) {
	h := TenantRateLimitMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, requestWithOrg("test-org"))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestTenantRateLimitMiddlewareHeaders(t *testing.T) {
	trl := NewTenantRateLimiter(1, time.Minute)
	defer trl.Stop()

	h := TenantRateLimitMiddleware(trl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request consumes the limit.
	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, requestWithOrg("headers-org"))
	if rr1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want %d", rr1.Code, http.StatusOK)
	}

	// Second request should be blocked with headers.
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, requestWithOrg("headers-org"))
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked request status = %d, want %d", rr2.Code, http.StatusTooManyRequests)
	}

	if rr2.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header should be set")
	}
	if rr2.Header().Get("X-RateLimit-Limit") != "1" {
		t.Fatalf("X-RateLimit-Limit = %q, want %q", rr2.Header().Get("X-RateLimit-Limit"), "1")
	}
	if rr2.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("X-RateLimit-Remaining = %q, want %q", rr2.Header().Get("X-RateLimit-Remaining"), "0")
	}
	if rr2.Header().Get("X-Pulse-Org-ID") != "headers-org" {
		t.Fatalf("X-Pulse-Org-ID = %q, want %q", rr2.Header().Get("X-Pulse-Org-ID"), "headers-org")
	}
}

func requestWithOrg(orgID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	ctx := context.WithValue(req.Context(), OrgIDContextKey, orgID)
	return req.WithContext(ctx)
}
