package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestNewTenantMiddlewareWithConfig(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	checker := stubAuthorizationChecker{}

	mw := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{
		Persistence: persistence,
		AuthChecker: checker,
	})

	if mw.persistence != persistence {
		t.Fatalf("expected persistence to be set")
	}
	if mw.authChecker == nil {
		t.Fatalf("expected auth checker to be set")
	}
}

type stubAuthorizationChecker struct{}

func (stubAuthorizationChecker) TokenCanAccessOrg(*config.APITokenRecord, string) bool {
	return true
}

func (stubAuthorizationChecker) UserCanAccessOrg(string, string) bool {
	return true
}

func (stubAuthorizationChecker) CheckAccess(*config.APITokenRecord, string, string) AuthorizationResult {
	return AuthorizationResult{Allowed: true}
}

func TestWriteJSONError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSONError(rec, http.StatusBadRequest, "bad", "message")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "bad" || payload["message"] != "message" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestTenantMiddleware_OrgExtraction(t *testing.T) {
	mw := NewTenantMiddleware(nil)

	handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := GetOrgID(r.Context())
		org := GetOrganization(r.Context())
		if orgID == "" || org == nil || org.ID != orgID {
			t.Fatalf("unexpected org context: %q %+v", orgID, org)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "header-org")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_org_id", Value: "cookie-org"})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

func TestTenantMiddleware_InvalidOrg(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	mw := NewTenantMiddleware(persistence)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "../bad")
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTenantMiddleware_MultiTenantDisabled(t *testing.T) {
	orig := IsMultiTenantEnabled()
	SetMultiTenantEnabled(false)
	t.Cleanup(func() { SetMultiTenantEnabled(orig) })

	mw := NewTenantMiddleware(nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "tenant-1")
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", rec.Code)
	}
}

func TestTenantMiddleware_MultiTenantLicenseRequired(t *testing.T) {
	orig := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(orig) })

	mw := NewTenantMiddleware(nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "tenant-2")
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rec.Code)
	}
}
