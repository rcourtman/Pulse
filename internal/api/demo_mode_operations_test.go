package api

import (
	"net/http"
	"net/http/httptest"
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
