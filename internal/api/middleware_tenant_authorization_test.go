package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type recordingAuthorizationChecker struct {
	result   AuthorizationResult
	calls    int
	lastOrg  string
	lastUser string
	lastTok  *config.APITokenRecord
}

func (s *recordingAuthorizationChecker) TokenCanAccessOrg(_ *config.APITokenRecord, _ string) bool {
	return s.result.Allowed
}

func (s *recordingAuthorizationChecker) UserCanAccessOrg(_ string, _ string) bool {
	return s.result.Allowed
}

func (s *recordingAuthorizationChecker) CheckAccess(token *config.APITokenRecord, userID, orgID string) AuthorizationResult {
	s.calls++
	s.lastTok = token
	s.lastUser = userID
	s.lastOrg = orgID
	return s.result
}

func TestTenantMiddleware_RejectsUnknownOrgBeforeLicense(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(false)
	t.Setenv("PULSE_DEV", "true")

	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mw := NewTenantMiddleware(mtp)
	handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "acme")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestTenantMiddleware_AuthorizationDenied(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	baseDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "orgs", "acme"), 0o755); err != nil {
		t.Fatalf("failed to create org dir: %v", err)
	}

	checker := &recordingAuthorizationChecker{result: AuthorizationResult{Allowed: false, Reason: "denied"}}
	mw := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{
		Persistence: config.NewMultiTenantPersistence(baseDir),
		AuthChecker: checker,
	})

	handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "acme")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if checker.calls != 1 {
		t.Fatalf("expected auth checker to be called once, got %d", checker.calls)
	}
	if checker.lastOrg != "acme" || checker.lastUser != "alice" {
		t.Fatalf("unexpected auth checker args: org=%q user=%q", checker.lastOrg, checker.lastUser)
	}
}

func TestTenantMiddleware_AuthorizationAllowed(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	baseDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "orgs", "acme"), 0o755); err != nil {
		t.Fatalf("failed to create org dir: %v", err)
	}

	checker := &recordingAuthorizationChecker{result: AuthorizationResult{Allowed: true}}
	mw := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{
		Persistence: config.NewMultiTenantPersistence(baseDir),
		AuthChecker: checker,
	})

	handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := GetOrgID(r.Context()); got != "acme" {
			t.Fatalf("expected org in context, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	token := &config.APITokenRecord{ID: "token-1", OrgID: "acme"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "acme")
	req = req.WithContext(auth.WithAPIToken(req.Context(), token))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if checker.calls != 1 {
		t.Fatalf("expected auth checker to be called once, got %d", checker.calls)
	}
	if checker.lastTok == nil || checker.lastTok.ID != "token-1" {
		t.Fatalf("expected token to be passed to auth checker")
	}
}
