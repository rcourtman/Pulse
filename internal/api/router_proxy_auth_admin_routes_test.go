package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func newProxyAuthRouter(t *testing.T) *Router {
	t.Helper()
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	cfg := &config.Config{
		ProxyAuthSecret:     "proxy-secret",
		ProxyAuthUserHeader: "X-Proxy-User",
		ProxyAuthRoleHeader: "X-Proxy-Roles",
		ProxyAuthAdminRole:  "admin",
		DataPath:            t.TempDir(),
		ConfigPath:          t.TempDir(),
		EnvOverrides:        make(map[string]bool),
	}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	if router.configHandlers != nil {
		router.configHandlers.SetConfig(cfg)
	}
	return router
}

func TestProxyAuthAdminRouteConfigSystem(t *testing.T) {
	router := newProxyAuthRouter(t)

	t.Run("non-admin forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/config/system", nil)
		req.Header.Set("X-Proxy-Secret", "proxy-secret")
		req.Header.Set("X-Proxy-User", "viewer")
		req.Header.Set("X-Proxy-Roles", "viewer|user")
		rec := httptest.NewRecorder()

		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
		}
	})

	t.Run("admin allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/config/system", nil)
		req.Header.Set("X-Proxy-Secret", "proxy-secret")
		req.Header.Set("X-Proxy-User", "adminuser")
		req.Header.Set("X-Proxy-Roles", "viewer|admin")
		rec := httptest.NewRecorder()

		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
		}
	})
}

func TestProxyAuthAdminRouteLicenseStatus(t *testing.T) {
	router := newProxyAuthRouter(t)

	t.Run("non-admin forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/license/status", nil)
		req.Header.Set("X-Proxy-Secret", "proxy-secret")
		req.Header.Set("X-Proxy-User", "viewer")
		req.Header.Set("X-Proxy-Roles", "viewer|user")
		rec := httptest.NewRecorder()

		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
		}
	})

	t.Run("admin allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/license/status", nil)
		req.Header.Set("X-Proxy-Secret", "proxy-secret")
		req.Header.Set("X-Proxy-User", "adminuser")
		req.Header.Set("X-Proxy-Roles", "viewer|admin")
		rec := httptest.NewRecorder()

		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
		}
	})
}
