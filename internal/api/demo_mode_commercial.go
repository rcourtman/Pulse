package api

import (
	"net/http"
	"strings"
)

type publicDemoCommercialExposure string

const (
	publicDemoCommercialExposureHidden publicDemoCommercialExposure = "hidden"
)

type publicDemoCommercialRoutePolicy struct {
	route    string
	exposure publicDemoCommercialExposure
	matches  func(*http.Request) bool
}

var publicDemoCommercialPolicies = []publicDemoCommercialRoutePolicy{
	{
		route:    "/api/license/status",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialPath("/api/license/status"),
	},
	{
		route:    "/api/license/features",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialPath("/api/license/features"),
	},
	{
		route:    "GET /api/license/commercial-posture",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodGet, "/api/license/commercial-posture"),
	},
	{
		route:    "GET /auth/license-purchase-start",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodGet, "/auth/license-purchase-start"),
	},
	{
		route:    licensePurchaseHandoffPath,
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodGet, licensePurchaseHandoffPath),
	},
	{
		route:    "/api/license/activate",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialPath("/api/license/activate"),
	},
	{
		route:    "/api/license/clear",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialPath("/api/license/clear"),
	},
	{
		route:    "GET /api/license/entitlements",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodGet, "/api/license/entitlements"),
	},
	{
		route:    "POST /api/license/trial/start",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodPost, "/api/license/trial/start"),
	},
	{
		route:    "GET /api/license/monitored-system-ledger",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodGet, "/api/license/monitored-system-ledger"),
	},
	{
		route:    "POST /api/upgrade-metrics/events",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodPost, "/api/upgrade-metrics/events"),
	},
	{
		route:    "GET /api/upgrade-metrics/stats",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodGet, "/api/upgrade-metrics/stats"),
	},
	{
		route:    "GET /api/upgrade-metrics/health",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodGet, "/api/upgrade-metrics/health"),
	},
	{
		route:    "GET /api/upgrade-metrics/config",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodGet, "/api/upgrade-metrics/config"),
	},
	{
		route:    "PUT /api/upgrade-metrics/config",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodPut, "/api/upgrade-metrics/config"),
	},
	{
		route:    "GET /api/admin/upgrade-metrics-funnel",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialMethodPath(http.MethodGet, "/api/admin/upgrade-metrics-funnel"),
	},
	{
		route:    "GET /api/admin/orgs/{id}/billing-state",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialOrgBillingStatePath(http.MethodGet),
	},
	{
		route:    "PUT /api/admin/orgs/{id}/billing-state",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialOrgBillingStatePath(http.MethodPut),
	},
	{
		route:    "/auth/trial-activate",
		exposure: publicDemoCommercialExposureHidden,
		matches:  exactDemoCommercialPath("/auth/trial-activate"),
	},
}

func publicDemoCommercialPolicyForRequest(
	r *http.Request,
) (publicDemoCommercialExposure, bool) {
	for _, policy := range publicDemoCommercialPolicies {
		if policy.matches != nil && policy.matches(r) {
			return policy.exposure, true
		}
	}
	return "", false
}

func publicDemoCommercialRouteInventory() []string {
	routes := make([]string, 0, len(publicDemoCommercialPolicies))
	for _, policy := range publicDemoCommercialPolicies {
		routes = append(routes, policy.route)
	}
	return routes
}

func sanitizeRuntimeCapabilitiesPayloadForPublicDemo(
	payload RuntimeCapabilitiesPayload,
) RuntimeCapabilitiesPayload {
	sanitized := payload
	sanitized.Limits = sanitizeLimitStatusesForPublicDemo(payload.Limits)
	if sanitized.Capabilities == nil {
		sanitized.Capabilities = []string{}
	}
	return sanitized
}

func sanitizeLimitStatusesForPublicDemo(limits []LimitStatus) []LimitStatus {
	if len(limits) == 0 {
		return []LimitStatus{}
	}

	sanitized := make([]LimitStatus, 0, len(limits))
	for _, limit := range limits {
		sanitized = append(sanitized, LimitStatus{
			Key:     limit.Key,
			Limit:   0,
			Current: 0,
			State:   "ok",
		})
	}
	return sanitized
}

func exactDemoCommercialPath(path string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		if r == nil || r.URL == nil {
			return false
		}
		return normalizeDemoCommercialPath(r.URL.Path) == path
	}
}

func exactDemoCommercialMethodPath(method, path string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		if r == nil || r.Method != method {
			return false
		}
		return exactDemoCommercialPath(path)(r)
	}
}

func exactDemoCommercialOrgBillingStatePath(method string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		if r == nil || r.Method != method || r.URL == nil {
			return false
		}

		parts := strings.Split(strings.Trim(normalizeDemoCommercialPath(r.URL.Path), "/"), "/")
		if len(parts) != 5 {
			return false
		}

		return parts[0] == "api" &&
			parts[1] == "admin" &&
			parts[2] == "orgs" &&
			strings.TrimSpace(parts[3]) != "" &&
			parts[4] == "billing-state"
	}
}

func normalizeDemoCommercialPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path == "/" {
		return path
	}
	return strings.TrimRight(path, "/")
}
