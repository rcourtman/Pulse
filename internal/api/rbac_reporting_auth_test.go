package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRBACEndpointsRequireAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "rbac-auth-token-123.12345678", nil, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/admin/roles",
		"/api/admin/users",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 without auth on %s, got %d", path, rec.Code)
		}
	}
}

func TestReportingEndpointsRequireAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "reporting-auth-token-123.12345678", nil, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/admin/reports/generate", body: ""},
		{method: http.MethodPost, path: "/api/admin/reports/generate-multi", body: `{}`},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 without auth on %s %s, got %d", tc.method, tc.path, rec.Code)
		}
	}
}
