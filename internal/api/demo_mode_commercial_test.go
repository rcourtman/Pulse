package api

import (
	"strings"
	"testing"
)

func TestPublicDemoCommercialRouteInventoryCoverage(t *testing.T) {
	literalRoutes, _, _ := parseRouterRoutes(t)

	var actualCommercialRoutes []string
	for _, route := range literalRoutes {
		if routeBelongsToPublicDemoCommercialBoundary(route) {
			actualCommercialRoutes = append(actualCommercialRoutes, route)
		}
	}

	actualSet := sliceToSet(t, actualCommercialRoutes, "public demo commercial boundary routes")
	expectedSet := sliceToSet(t, publicDemoCommercialRouteInventory(), "public demo commercial route inventory")

	if missing := setDifference(actualSet, expectedSet); len(missing) > 0 {
		t.Fatalf(
			"commercial routes missing demo policy coverage: %s",
			strings.Join(sortedKeys(missing), ", "),
		)
	}
	if stale := setDifference(expectedSet, actualSet); len(stale) > 0 {
		t.Fatalf(
			"demo commercial route inventory contains routes outside the boundary family: %s",
			strings.Join(sortedKeys(stale), ", "),
		)
	}
}

func TestSanitizeRuntimeCapabilitiesPayloadForPublicDemo(t *testing.T) {
	sanitized := sanitizeRuntimeCapabilitiesPayloadForPublicDemo(RuntimeCapabilitiesPayload{
		Capabilities: []string{"relay", "ai_patrol"},
		Limits: []LimitStatus{
			{
				Key:     maxMonitoredSystemsLicenseGateKey,
				Limit:   5,
				Current: 16,
				State:   "enforced",
			},
		},
		HostedMode:     true,
		MaxHistoryDays: 90,
	})

	if len(sanitized.Capabilities) != 2 {
		t.Fatalf("capabilities=%v, want original capabilities preserved", sanitized.Capabilities)
	}
	if len(sanitized.Limits) != 1 {
		t.Fatalf("limits=%v, want one sanitized limit", sanitized.Limits)
	}
	if sanitized.Limits[0].Limit != 0 || sanitized.Limits[0].Current != 0 || sanitized.Limits[0].State != "ok" {
		t.Fatalf("sanitized limit=%+v, want limit=0 current=0 state=ok", sanitized.Limits[0])
	}
	if sanitized.MaxHistoryDays != 90 || !sanitized.HostedMode {
		t.Fatalf("non-commercial runtime capability fields should be preserved, got max_history_days=%d hosted_mode=%v", sanitized.MaxHistoryDays, sanitized.HostedMode)
	}
}

func routeBelongsToPublicDemoCommercialBoundary(route string) bool {
	switch {
	case route == "/auth/trial-activate":
		return true
	case route == licensePurchaseStartPath:
		return true
	case route == "GET "+licensePurchaseStartPath:
		return true
	case route == licensePurchaseHandoffPath:
		return true
	case route == "GET "+licensePurchaseHandoffPath:
		return true
	case route == "GET /api/license/runtime-capabilities":
		return false
	case strings.HasPrefix(route, "/api/license/"):
		return true
	case strings.HasPrefix(route, "GET /api/license/"):
		return true
	case strings.HasPrefix(route, "POST /api/license/"):
		return true
	case strings.HasPrefix(route, "GET /api/upgrade-metrics/"):
		return true
	case strings.HasPrefix(route, "POST /api/upgrade-metrics/"):
		return true
	case strings.HasPrefix(route, "PUT /api/upgrade-metrics/"):
		return true
	case route == "GET /api/admin/upgrade-metrics-funnel":
		return true
	case route == "GET /api/admin/orgs/{id}/billing-state":
		return true
	case route == "PUT /api/admin/orgs/{id}/billing-state":
		return true
	default:
		return false
	}
}
