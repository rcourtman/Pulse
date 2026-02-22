package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestTenantMiddlewareBlocksOrgBoundTokenFromOtherOrg(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	// Dev mode enables license checks for multi-tenant features.
	t.Setenv("PULSE_DEV", "true")

	rawToken := "org-bound-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	record.OrgID = "org-a"
	cfg := newTestConfigWithTokens(t, record)

	// Ensure orgs exist to avoid 400 invalid_org.
	baseDir := cfg.DataPath
	for _, orgID := range []string{"org-a", "org-b"} {
		if err := os.MkdirAll(filepath.Join(baseDir, "orgs", orgID), 0o755); err != nil {
			t.Fatalf("create org dir: %v", err)
		}
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("X-Pulse-Org-ID", "org-b")
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for org-bound token access to another org, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "access_denied" {
		t.Fatalf("expected error=access_denied, got %q", payload["error"])
	}
	if msg := payload["message"]; msg == "" || !strings.Contains(msg, "authorized") {
		t.Fatalf("expected access denied message, got %q", msg)
	}
}

func TestTenantMiddlewareBlocksOrgBoundTokenFromOtherOrg_WebSocket(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	rawToken := "org-bound-ws-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	record.OrgID = "org-a"
	cfg := newTestConfigWithTokens(t, record)

	baseDir := cfg.DataPath
	for _, orgID := range []string{"org-a", "org-b"} {
		if err := os.MkdirAll(filepath.Join(baseDir, "orgs", orgID), 0o755); err != nil {
			t.Fatalf("create org dir: %v", err)
		}
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("X-Pulse-Org-ID", "org-b")
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for org-bound websocket access to another org, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "access_denied" {
		t.Fatalf("expected error=access_denied, got %q", payload["error"])
	}
}

func TestTenantMiddlewareBlocksOrgBoundTokenFromOtherOrg_AlertsEndpoint(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	rawToken := "org-bound-alerts-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	record.OrgID = "org-a"
	cfg := newTestConfigWithTokens(t, record)

	baseDir := cfg.DataPath
	for _, orgID := range []string{"org-a", "org-b"} {
		if err := os.MkdirAll(filepath.Join(baseDir, "orgs", orgID), 0o755); err != nil {
			t.Fatalf("create org dir: %v", err)
		}
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/alerts/active", nil)
	req.Header.Set("X-Pulse-Org-ID", "org-b")
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for org-bound token access to another org on alerts endpoint, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "access_denied" {
		t.Fatalf("expected error=access_denied, got %q", payload["error"])
	}
	if msg := payload["message"]; msg == "" || !strings.Contains(msg, "authorized") {
		t.Fatalf("expected access denied message, got %q", msg)
	}
}

func TestTenantMiddlewareBlocksOrgBoundTokenFromOtherOrg_AlertHistoryEndpoint(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	rawToken := "org-bound-alert-history-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	record.OrgID = "org-a"
	cfg := newTestConfigWithTokens(t, record)

	baseDir := cfg.DataPath
	for _, orgID := range []string{"org-a", "org-b"} {
		if err := os.MkdirAll(filepath.Join(baseDir, "orgs", orgID), 0o755); err != nil {
			t.Fatalf("create org dir: %v", err)
		}
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/alerts/history?limit=10", nil)
	req.Header.Set("X-Pulse-Org-ID", "org-b")
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for org-bound token access to another org on alert history, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "access_denied" {
		t.Fatalf("expected error=access_denied, got %q", payload["error"])
	}
	if msg := payload["message"]; msg == "" || !strings.Contains(msg, "authorized") {
		t.Fatalf("expected access denied message, got %q", msg)
	}
}

func TestTenantMiddlewareBlocksOrgBoundTokenFromOtherOrg_ResourceDetailEndpoints(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	rawToken := "org-bound-resource-detail-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	record.OrgID = "org-a"
	cfg := newTestConfigWithTokens(t, record)

	baseDir := cfg.DataPath
	for _, orgID := range []string{"org-a", "org-b"} {
		if err := os.MkdirAll(filepath.Join(baseDir, "orgs", orgID), 0o755); err != nil {
			t.Fatalf("create org dir: %v", err)
		}
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/metrics-store/history?resourceType=vm&resourceId=vm-1&metric=cpu&range=1h",
		"/api/docker/metadata/container-1",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Pulse-Org-ID", "org-b")
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for org-bound token on cross-tenant path %s, got %d", path, rec.Code)
		}

		var payload map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response for %s: %v", path, err)
		}
		if payload["error"] != "access_denied" {
			t.Fatalf("expected error=access_denied for %s, got %q", path, payload["error"])
		}
	}
}

func TestTenantMiddlewareOrgBoundReadScopeCannotWriteAlertsConfig(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	rawToken := "org-bound-read-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	record.OrgID = "org-a"
	cfg := newTestConfigWithTokens(t, record)

	baseDir := cfg.DataPath
	for _, orgID := range []string{"org-a", "org-b"} {
		if err := os.MkdirAll(filepath.Join(baseDir, "orgs", orgID), 0o755); err != nil {
			t.Fatalf("create org dir: %v", err)
		}
	}

	router := newMultiTenantRouter(t, cfg)

	req := httptest.NewRequest(http.MethodPut, "/api/alerts/config", strings.NewReader(`{}`))
	req.Header.Set("X-Pulse-Org-ID", "org-a")
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing monitoring:write scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeMonitoringWrite) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeMonitoringWrite, rec.Body.String())
	}
}

func TestTenantMiddlewareRejectsOrgBoundTokenReuseAcrossTenants(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	rawToken := "org-bound-reuse-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	record.OrgID = "org-a"
	cfg := newTestConfigWithTokens(t, record)

	baseDir := cfg.DataPath
	for _, orgID := range []string{"org-a", "org-b"} {
		if err := os.MkdirAll(filepath.Join(baseDir, "orgs", orgID), 0o755); err != nil {
			t.Fatalf("create org dir: %v", err)
		}
	}

	router := newMultiTenantRouter(t, cfg)

	orgAReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	orgAReq.Header.Set("X-Pulse-Org-ID", "org-a")
	orgAReq.Header.Set("X-API-Token", rawToken)
	orgARec := httptest.NewRecorder()
	router.Handler().ServeHTTP(orgARec, orgAReq)
	if orgARec.Code != http.StatusOK {
		t.Fatalf("expected 200 for org-a token access in org-a, got %d", orgARec.Code)
	}

	orgBReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	orgBReq.Header.Set("X-Pulse-Org-ID", "org-b")
	orgBReq.Header.Set("X-API-Token", rawToken)
	orgBRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(orgBRec, orgBReq)
	if orgBRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when reusing org-a token in org-b, got %d", orgBRec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(orgBRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "access_denied" {
		t.Fatalf("expected error=access_denied, got %q", payload["error"])
	}
}
