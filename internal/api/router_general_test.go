package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/stretchr/testify/assert"
)

func TestRouter_HandlerWrapping(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tmpDir,
		ConfigPath: tmpDir,
	}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	handler := router.Handler()
	assert.NotNil(t, handler)

	// Verify it implements http.Handler
	_, ok := handler.(http.Handler)
	assert.True(t, ok)

	// Verify it handles a basic request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
}

func TestRouter_GetTenantMonitor_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		MultiTenantEnabled: false,
		DataPath:           tmpDir,
		ConfigPath:         tmpDir,
	}
	defaultMon := &monitoring.Monitor{}

	router := NewRouter(cfg, defaultMon, nil, nil, nil, "1.0.0")

	// With flag disabled, should always return default monitor
	ctx := context.Background()
	m := router.getTenantMonitor(ctx)
	assert.Equal(t, defaultMon, m)

	// Non-default tenant context should fail closed when tenant monitor is unavailable
	ctx = context.WithValue(ctx, OrgIDContextKey, "some-tenant")
	m = router.getTenantMonitor(ctx)
	assert.Nil(t, m)
}

func TestTenantMonitorGuardMiddleware_DefaultOrgAllowed(t *testing.T) {
	router := &Router{}
	guarded := router.tenantMonitorGuardMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "default"))
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTenantMonitorGuardMiddleware_NonDefaultOrgRequiresTenantMonitor(t *testing.T) {
	router := &Router{}
	guarded := router.tenantMonitorGuardMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestTenantMonitorGuardMiddleware_NonDefaultOrgAllowedWhenTenantMonitorAvailable(t *testing.T) {
	tenantMonitor, _, _ := newTestMonitor(t)
	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"tenant-a": tenantMonitor,
	})

	router := &Router{mtMonitor: mtm}
	guarded := router.tenantMonitorGuardMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
