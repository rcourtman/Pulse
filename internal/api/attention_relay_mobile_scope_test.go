package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Regression: v6.1.0-rc.4 shipped the attention workbench gated on
// monitoring:read only, so every mobile relay token (which carries just
// relay:mobile-access) got 403 on alert sync the moment a server upgraded
// (Strasser report, 2026-07-22). The attention routes supersede the legacy
// patrol findings routes and must accept the same mobile relay capability.
func TestAttentionRoutes_AcceptRelayMobileAccessScope(t *testing.T) {
	rawToken := "attention-relay-mobile-scope.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeRelayMobileAccess}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/ai/patrol/attention?filter=all&page=1&limit=20"},
		{http.MethodGet, "/api/ai/patrol/attention/item-1"},
		{http.MethodPost, "/api/ai/patrol/attention/item-1/acknowledge"},
	}

	for _, tc := range paths {
		var req *http.Request
		if tc.method == http.MethodPost {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		} else {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		}
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		// With no monitor wired the handler reports the read model as
		// unavailable; the point is that the scope gate no longer rejects
		// the mobile relay token.
		if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
			t.Errorf("%s %s: mobile relay token rejected with %d: %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestAttentionRoutes_AcceptLegacyAIExecuteScope(t *testing.T) {
	rawToken := "attention-legacy-ai-execute.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIExecute}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/attention", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
		t.Fatalf("ai:execute token rejected with %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAttentionRoutes_RejectUnrelatedScope(t *testing.T) {
	rawToken := "attention-unrelated-scope.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIChat}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/ai/patrol/attention"},
		{http.MethodPost, "/api/ai/patrol/attention/item-1/acknowledge"},
	}

	for _, tc := range paths {
		var req *http.Request
		if tc.method == http.MethodPost {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		} else {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		}
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: expected 403 for unrelated scope, got %d: %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}
