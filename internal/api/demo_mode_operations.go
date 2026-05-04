package api

import "net/http"

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
