package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// /api/resources/ is one prefix route. Its POST sub-paths (link, unlink,
// report-merge) persist resource identity changes, so a read-only token must
// be refused while reads stay available to it.
func TestResourceRoutesRequireWriteScopeForMutations(t *testing.T) {
	const readToken = "resources-read-only-scope.12345678"
	const writeToken = "resources-write-scope.12345678"
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, readToken, []string{config.ScopeMonitoringRead}, nil),
		newTokenRecord(t, writeToken, []string{config.ScopeMonitoringWrite}, nil),
	)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	do := func(token, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("X-API-Token", token)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}

	mutations := []struct{ path, body string }{
		{"/api/resources/vm-1/link", `{"targetId":"agent-1"}`},
		{"/api/resources/vm-1/unlink", `{"targetId":"agent-1"}`},
		{"/api/resources/vm-1/report-merge", `{}`},
	}
	for _, m := range mutations {
		rec := do(readToken, http.MethodPost, m.path, m.body)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), config.ScopeMonitoringWrite) {
			t.Errorf("read-only token POST %s = %d %q, want 403 naming %s", m.path, rec.Code, rec.Body.String(), config.ScopeMonitoringWrite)
		}
		rec = do(writeToken, http.MethodPost, m.path, m.body)
		if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
			t.Errorf("write-scoped token POST %s refused with %d: %s", m.path, rec.Code, rec.Body.String())
		}
	}

	// Reads on the same prefix stay available to a read-only token.
	rec := do(readToken, http.MethodGet, "/api/resources/vm-1", "")
	if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
		t.Errorf("read-only token GET refused with %d: %s", rec.Code, rec.Body.String())
	}
	// The operator-state registration is more specific than the prefix and
	// has its own write requirement; this also refuses the read-only token.
	rec = do(readToken, http.MethodPut, "/api/resources/vm-1/operator-state", `{}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("read-only token PUT operator-state = %d, want 403", rec.Code)
	}
	// A write-only token has no read scope, so reads stay refused.
	rec = do(writeToken, http.MethodGet, "/api/resources/vm-1", "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("write-only token GET = %d, want 403", rec.Code)
	}
}
