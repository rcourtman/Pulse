package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func assertMissingScope(t *testing.T, rec *httptest.ResponseRecorder, expectedScope string, path string) {
	t.Helper()
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for %s, got %d", path, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response for %s: %v", path, err)
	}
	if payload["error"] != "missing_scope" {
		t.Fatalf("expected missing_scope error for %s, got %v", path, payload["error"])
	}
	if payload["requiredScope"] != expectedScope {
		t.Fatalf("expected requiredScope %q for %s, got %v", expectedScope, path, payload["requiredScope"])
	}
}

func TestNotificationReadEndpointsRequireSettingsReadScope(t *testing.T) {
	rawToken := "notifications-read-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	endpoints := []string{
		"/api/notifications/email",
		"/api/notifications/apprise",
		"/api/notifications/webhooks",
		"/api/notifications/webhook-templates",
		"/api/notifications/webhook-history",
		"/api/notifications/email-providers",
		"/api/notifications/health",
	}

	for _, path := range endpoints {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		assertMissingScope(t, rec, config.ScopeSettingsRead, path)
	}
}

func TestNotificationWriteEndpointsRequireSettingsWriteScope(t *testing.T) {
	rawToken := "notifications-write-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	testCases := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/api/notifications/email"},
		{http.MethodPut, "/api/notifications/apprise"},
		{http.MethodPost, "/api/notifications/webhooks"},
		{http.MethodPost, "/api/notifications/webhooks/test"},
		{http.MethodPut, "/api/notifications/webhooks/wh1"},
		{http.MethodDelete, "/api/notifications/webhooks/wh1"},
		{http.MethodPost, "/api/notifications/test"},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		assertMissingScope(t, rec, config.ScopeSettingsWrite, tc.method+" "+tc.path)
	}
}
