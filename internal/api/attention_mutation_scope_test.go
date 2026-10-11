package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// The attention prefix serves reads, lifecycle mutations and action planning.
// The pinned capability inventory says a lifecycle POST needs monitoring:write
// and a read needs monitoring:read, so a token must be admitted by the scope
// that matches what it is doing, not by the read scope for everything.
func TestAttentionRoutesGateEachOperationOnItsOwnScope(t *testing.T) {
	const readToken = "attention-scope-read.12345678"
	const writeToken = "attention-scope-write.12345678"
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, readToken, []string{config.ScopeMonitoringRead}, nil),
		newTokenRecord(t, writeToken, []string{config.ScopeMonitoringWrite}, nil),
	)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	do := func(token, method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set("X-API-Token", token)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}
	refused := func(rec *httptest.ResponseRecorder) bool {
		return rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized
	}

	const mutation = "/api/ai/patrol/attention/item-1/acknowledge"
	if rec := do(writeToken, http.MethodPost, mutation); refused(rec) {
		t.Errorf("monitoring:write token POST %s refused with %d: %s", mutation, rec.Code, rec.Body.String())
	}
	if rec := do(readToken, http.MethodPost, mutation); rec.Code != http.StatusForbidden {
		t.Errorf("monitoring:read token POST %s = %d, want 403", mutation, rec.Code)
	}

	const detail = "/api/ai/patrol/attention/item-1"
	if rec := do(readToken, http.MethodGet, detail); refused(rec) {
		t.Errorf("monitoring:read token GET %s refused with %d: %s", detail, rec.Code, rec.Body.String())
	}
	if rec := do(writeToken, http.MethodGet, detail); rec.Code != http.StatusForbidden {
		t.Errorf("monitoring:write token GET %s = %d, want 403 (write does not imply read)", detail, rec.Code)
	}

	// Action planning shares the prefix but is not a lifecycle mutation: it
	// keeps the read-side outer gate plus its own authorization, so write
	// scope alone must not admit it.
	const plan = "/api/ai/patrol/attention/item-1/actions/restart/plan"
	if rec := do(writeToken, http.MethodPost, plan); rec.Code != http.StatusForbidden {
		t.Errorf("monitoring:write token POST %s = %d, want 403", plan, rec.Code)
	}
}
