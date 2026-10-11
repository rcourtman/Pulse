package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// A collector uninstalls itself with its own agent:report token. Callers that
// are not tokens (browser sessions, proxy users) reach the administrative
// removal path in the handler, so they must be administrators, while the
// collector's token path keeps working.
func TestAgentUninstallKeepsTokenSelfRemovalReachable(t *testing.T) {
	const reportToken = "agent-uninstall-report.12345678"
	const unrelatedToken = "agent-uninstall-unrelated.12345678"
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, reportToken, []string{config.ScopeAgentReport}, nil),
		newTokenRecord(t, unrelatedToken, []string{config.ScopeMonitoringRead}, nil),
	)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	for _, path := range []string{"/api/agents/agent/uninstall", "/api/agents/host/uninstall"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("X-API-Token", unrelatedToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("token without agent:report POST %s = %d, want 403", path, rec.Code)
		}

		// An empty agentId is rejected by the handler itself, which proves the
		// request got past authentication and the scope gate.
		req = httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("X-API-Token", reportToken)
		rec = httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "missing_agent_id") {
			t.Errorf("agent:report token POST %s = %d %q, want 400 missing_agent_id from the handler", path, rec.Code, rec.Body.String())
		}
	}
}
