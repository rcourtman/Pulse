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

func TestTenantMiddlewareAllowsOrgIDsToken(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	rawToken := "org-ids-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	record.OrgIDs = []string{"org-a", "org-b"}
	cfg := newTestConfigWithTokens(t, record)

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

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for org-bound token in allowed org, got %d", rec.Code)
	}
}

func TestTenantMiddlewareBlocksOrgIDsTokenForOtherOrg(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	rawToken := "org-ids-blocked-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	record.OrgIDs = []string{"org-a", "org-b"}
	cfg := newTestConfigWithTokens(t, record)

	baseDir := cfg.DataPath
	for _, orgID := range []string{"org-a", "org-b", "org-c"} {
		if err := os.MkdirAll(filepath.Join(baseDir, "orgs", orgID), 0o755); err != nil {
			t.Fatalf("create org dir: %v", err)
		}
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("X-Pulse-Org-ID", "org-c")
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for org-bound token access to other org, got %d", rec.Code)
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
