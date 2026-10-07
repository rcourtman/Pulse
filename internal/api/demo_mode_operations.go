package api

import (
	"net/http"
	"strings"
)

var publicDemoAdminOperationsPolicies = []publicDemoCommercialRoutePolicy{
	{
		route:    "GET /api/diagnostics",
		exposure: publicDemoCommercialExposureHidden,
		matches:  readDemoCommercialPath("/api/diagnostics"),
	},
	{
		route:    "POST /api/diagnostics/docker/prepare-token",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodPost, "/api/diagnostics/docker/prepare-token"),
	},
	{
		route:    "GET /api/logs/stream",
		exposure: publicDemoCommercialExposureHidden,
		matches:  readDemoCommercialPath("/api/logs/stream"),
	},
	{
		route:    "GET /api/logs/download",
		exposure: publicDemoCommercialExposureHidden,
		matches:  readDemoCommercialPath("/api/logs/download"),
	},
	{
		route:    "GET /api/logs/level",
		exposure: publicDemoCommercialExposureHidden,
		matches:  readDemoCommercialPath("/api/logs/level"),
	},
	{
		route:    "POST /api/logs/level",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodPost, "/api/logs/level"),
	},
	{
		route:    "GET /api/admin/users",
		exposure: publicDemoCommercialExposureHidden,
		matches:  readDemoCommercialPath("/api/admin/users"),
	},
	{
		route:    "GET /api/discover",
		exposure: publicDemoCommercialExposureHidden,
		matches:  readDemoCommercialPath("/api/discover"),
	},
	// Go runtime profiling. These routes are gated only on admin auth, which
	// a demo login that is the configured admin passes, so every visitor could
	// otherwise pull goroutine and heap dumps or the command line, or start
	// CPU profiles and traces that cost the whole process. Hidden for every
	// method, so a write probe such as the POST symbol lookup gets the same
	// 404 instead of revealing the route.
	{
		route:    "/debug/pprof",
		exposure: publicDemoCommercialExposureHidden,
		matches:  familyDemoCommercialPath("/debug/pprof"),
	},
}

func publicDemoAdminOperationsPolicyForRequest(
	r *http.Request,
) (publicDemoCommercialExposure, bool) {
	for _, policy := range publicDemoAdminOperationsPolicies {
		if policy.matches != nil && policy.matches(r) {
			return policy.exposure, true
		}
	}
	return "", false
}

// familyDemoCommercialPath matches root and every path below it, for any
// method.
func familyDemoCommercialPath(root string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		if r == nil || r.URL == nil {
			return false
		}
		path := normalizeDemoCommercialPath(r.URL.Path)
		return path == root || strings.HasPrefix(path, root+"/")
	}
}
