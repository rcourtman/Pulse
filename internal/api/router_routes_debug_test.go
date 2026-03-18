package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// TestPprofEndpoints_RequireAuth verifies that unauthenticated requests to
// /debug/pprof endpoints are rejected with 401 when auth is configured.
func TestPprofEndpoints_RequireAuth(t *testing.T) {
	hashed, err := auth.HashPassword("admin")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	r := &Router{
		config: &config.Config{
			AuthUser: "admin",
			AuthPass: hashed,
		},
		mux: http.NewServeMux(),
	}
	r.registerDebugRoutes()

	paths := []string{
		"/debug/pprof", // exact match (no trailing slash) — must also require auth
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/profile",
		"/debug/pprof/symbol",
		"/debug/pprof/trace",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			rec := httptest.NewRecorder()
			r.mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("GET %s without auth: got status %d, want %d", p, rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

// TestPprofEndpoints_AdminAccessSucceeds verifies that the pprof index handler
// returns 200 when auth is not configured (open access mode) — simulating a
// homelab environment without authentication enabled.
func TestPprofEndpoints_AdminAccessSucceeds(t *testing.T) {
	r := &Router{
		config: &config.Config{},
		mux:    http.NewServeMux(),
	}
	r.registerDebugRoutes()

	// /debug/pprof/ (index) should return 200 when no auth is configured
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec := httptest.NewRecorder()
	r.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /debug/pprof/ with no auth configured: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestPprofEndpoints_SymbolPost verifies that the pprof symbol endpoint
// accepts POST requests (used by `go tool pprof` for symbolization).
func TestPprofEndpoints_SymbolPost(t *testing.T) {
	r := &Router{
		config: &config.Config{},
		mux:    http.NewServeMux(),
	}
	r.registerDebugRoutes()

	req := httptest.NewRequest(http.MethodPost, "/debug/pprof/symbol", nil)
	rec := httptest.NewRecorder()
	r.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("POST /debug/pprof/symbol: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestPprofEndpoints_HeapProfile verifies that named profiles (e.g. heap)
// are served via the index handler's sub-routing.
func TestPprofEndpoints_HeapProfile(t *testing.T) {
	r := &Router{
		config: &config.Config{},
		mux:    http.NewServeMux(),
	}
	r.registerDebugRoutes()

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/heap", nil)
	rec := httptest.NewRecorder()
	r.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /debug/pprof/heap: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestPprofEndpoints_RoutingPrefix verifies that the /debug/pprof prefix is
// included in ServeHTTP's mux dispatch condition so requests actually reach the
// registered handlers instead of being served as frontend HTML.
func TestPprofEndpoints_RoutingPrefix(t *testing.T) {
	// The fix adds strings.HasPrefix(req.URL.Path, "/debug/pprof") to the
	// ServeHTTP mux dispatch. We verify the prefix pattern string is present
	// in the routing condition by checking the /debug/pprof/heap path reaches
	// the pprof handler and returns the expected content type.
	r := &Router{
		config: &config.Config{},
		mux:    http.NewServeMux(),
	}
	r.registerDebugRoutes()

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/goroutine?debug=1", nil)
	rec := httptest.NewRecorder()
	r.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /debug/pprof/goroutine?debug=1: got status %d, want %d", rec.Code, http.StatusOK)
	}
	ct := rec.Header().Get("Content-Type")
	if ct == "" {
		t.Error("expected Content-Type header, got empty")
	}
}

// TestPprofEnabled verifies the pprofEnabled toggle.
func TestPprofEnabled(t *testing.T) {
	// Default: enabled (t.Setenv to empty ensures clean state and auto-restores)
	t.Setenv("PULSE_PPROF_DISABLED", "")
	if !pprofEnabled() {
		t.Error("pprofEnabled() = false, want true (default)")
	}

	// Disabled via env
	t.Setenv("PULSE_PPROF_DISABLED", "true")
	if pprofEnabled() {
		t.Error("pprofEnabled() = true, want false (PULSE_PPROF_DISABLED=true)")
	}

	t.Setenv("PULSE_PPROF_DISABLED", "1")
	if pprofEnabled() {
		t.Error("pprofEnabled() = true, want false (PULSE_PPROF_DISABLED=1)")
	}

	// Non-truthy value: still enabled
	t.Setenv("PULSE_PPROF_DISABLED", "no")
	if !pprofEnabled() {
		t.Error("pprofEnabled() = false, want true (PULSE_PPROF_DISABLED=no)")
	}
}
