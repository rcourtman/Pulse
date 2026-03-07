package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHostAgentConfigRequiresHostConfigScope(t *testing.T) {
	rawToken := "host-config-read-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	for _, path := range []string{"/api/agents/agent/host-1/config", "/api/agents/host/host-1/config"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing agent:config:read scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeAgentConfigRead) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeAgentConfigRead, rec.Body.String())
		}
	}
}
