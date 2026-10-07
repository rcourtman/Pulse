package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicDemoAdminOperationsPolicyForRequest(t *testing.T) {
	hiddenRoutes := []struct {
		name   string
		method string
		path   string
	}{
		{name: "diagnostics", method: http.MethodGet, path: "/api/diagnostics"},
		{name: "diagnostics token prepare", method: http.MethodPost, path: "/api/diagnostics/docker/prepare-token"},
		{name: "logs stream", method: http.MethodGet, path: "/api/logs/stream"},
		{name: "logs download", method: http.MethodGet, path: "/api/logs/download"},
		{name: "logs level read", method: http.MethodGet, path: "/api/logs/level"},
		{name: "logs level write", method: http.MethodPost, path: "/api/logs/level"},
		{name: "admin users read", method: http.MethodGet, path: "/api/admin/users"},
		{name: "admin users trailing slash read", method: http.MethodGet, path: "/api/admin/users/"},
		{name: "admin users head", method: http.MethodHead, path: "/api/admin/users"},
		{name: "manual discovery read", method: http.MethodGet, path: "/api/discover"},
		{name: "manual discovery head", method: http.MethodHead, path: "/api/discover"},
		{name: "pprof root", method: http.MethodGet, path: "/debug/pprof"},
		{name: "pprof index", method: http.MethodGet, path: "/debug/pprof/"},
		{name: "pprof heap with forced gc", method: http.MethodGet, path: "/debug/pprof/heap?gc=1"},
		{name: "pprof goroutine dump", method: http.MethodGet, path: "/debug/pprof/goroutine?debug=2"},
		{name: "pprof cmdline", method: http.MethodGet, path: "/debug/pprof/cmdline"},
		{name: "pprof cpu profile", method: http.MethodGet, path: "/debug/pprof/profile?seconds=30"},
		{name: "pprof trace", method: http.MethodGet, path: "/debug/pprof/trace?seconds=5"},
		{name: "pprof symbol lookup", method: http.MethodPost, path: "/debug/pprof/symbol"},
		{name: "pprof head", method: http.MethodHead, path: "/debug/pprof/allocs"},
		{name: "pprof options", method: http.MethodOptions, path: "/debug/pprof/"},
	}

	for _, tc := range hiddenRoutes {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			exposure, ok := publicDemoAdminOperationsPolicyForRequest(req)
			if !ok {
				t.Fatalf("%s %s did not match public demo admin operations policy", tc.method, tc.path)
			}
			if exposure != publicDemoCommercialExposureHidden {
				t.Fatalf("%s %s exposure=%q, want %q", tc.method, tc.path, exposure, publicDemoCommercialExposureHidden)
			}
		})
	}

	allowedRoutes := []struct {
		name   string
		method string
		path   string
	}{
		{name: "health", method: http.MethodGet, path: "/api/health"},
		{name: "resources", method: http.MethodGet, path: "/api/resources"},
		{name: "runtime capabilities", method: http.MethodGet, path: "/api/license/runtime-capabilities"},
		{name: "admin users write", method: http.MethodPost, path: "/api/admin/users"},
		{name: "manual discovery write", method: http.MethodPost, path: "/api/discover"},
		{name: "pprof lookalike", method: http.MethodGet, path: "/debug/pprofile"},
	}

	for _, tc := range allowedRoutes {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if exposure, ok := publicDemoAdminOperationsPolicyForRequest(req); ok {
				t.Fatalf("%s %s unexpectedly matched public demo admin operations policy with exposure=%q", tc.method, tc.path, exposure)
			}
		})
	}
}

// Every literal route the router files register under /debug/, with or without
// a method prefix, must stay hidden on the public demo whatever the method, so
// a new debug handler cannot ship reachable to the demo admin session by
// default.
func TestPublicDemoAdminOperationsPolicyHidesEveryDebugRoute(t *testing.T) {
	literalRoutes, _, _ := parseRouterRoutes(t)

	methods := []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodOptions,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	}

	var debugRoutes int
	for _, route := range literalRoutes {
		// ServeMux separates a method from its path with spaces or tabs.
		pattern := route
		if i := strings.IndexAny(route, " \t"); i > 0 {
			if rest := strings.TrimLeft(route[i+1:], " \t"); strings.HasPrefix(rest, "/") {
				pattern = rest
			}
		}
		if !strings.HasPrefix(pattern, "/debug/") {
			continue
		}
		debugRoutes++
		paths := []string{pattern}
		if strings.HasSuffix(pattern, "/") {
			paths = append(paths, pattern+"goroutine")
		}
		for _, path := range paths {
			for _, method := range methods {
				req := httptest.NewRequest(method, path, nil)
				exposure, ok := publicDemoAdminOperationsPolicyForRequest(req)
				if !ok || exposure != publicDemoCommercialExposureHidden {
					t.Errorf("%s %s is not hidden on the public demo (matched=%v exposure=%q)", method, path, ok, exposure)
				}
			}
		}
	}
	if debugRoutes == 0 {
		t.Fatal("no /debug/ routes found in the router inventory")
	}
}
