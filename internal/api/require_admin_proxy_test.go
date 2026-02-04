package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestRequireAdminProxyAuthRejectsNonAdmin(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret:     "proxy-secret",
		ProxyAuthUserHeader: "X-Proxy-User",
		ProxyAuthRoleHeader: "X-Proxy-Roles",
		ProxyAuthAdminRole:  "admin",
	}

	handlerCalled := false
	handler := RequireAdmin(cfg, func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/test", nil)
	req.Header.Set("X-Proxy-Secret", "proxy-secret")
	req.Header.Set("X-Proxy-User", "alice")
	req.Header.Set("X-Proxy-Roles", "viewer|user")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if handlerCalled {
		t.Fatalf("handler should not be called for non-admin proxy user")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestRequireAdminProxyAuthAllowsAdmin(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret:     "proxy-secret",
		ProxyAuthUserHeader: "X-Proxy-User",
		ProxyAuthRoleHeader: "X-Proxy-Roles",
		ProxyAuthAdminRole:  "admin",
	}

	handlerCalled := false
	handler := RequireAdmin(cfg, func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/test", nil)
	req.Header.Set("X-Proxy-Secret", "proxy-secret")
	req.Header.Set("X-Proxy-User", "alice")
	req.Header.Set("X-Proxy-Roles", "viewer|admin")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if !handlerCalled {
		t.Fatalf("expected handler to be called for admin proxy user")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusNoContent, rec.Code, rec.Body.String())
	}
}
