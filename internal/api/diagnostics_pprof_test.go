package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandlePprofDisabledReturnsNotFound(t *testing.T) {
	router := &Router{}

	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics/pprof/", nil)
	rec := httptest.NewRecorder()

	router.handlePprof(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlePprofEnabledServesIndex(t *testing.T) {
	t.Setenv("PULSE_ENABLE_PPROF", "true")
	router := &Router{}

	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics/pprof/", nil)
	rec := httptest.NewRecorder()

	router.handlePprof(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "heap") {
		t.Fatalf("expected pprof index to include heap profile")
	}
}

func TestPprofRouteRequiresAdmin(t *testing.T) {
	t.Setenv("PULSE_ENABLE_PPROF", "true")
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	t.Setenv("PULSE_DEV", "")
	t.Setenv("NODE_ENV", "")
	resetAdminBypassState()

	dataDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		AuthUser:   "admin",
		AuthPass:   "secret",
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics/pprof/", nil)
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
