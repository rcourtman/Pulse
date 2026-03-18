package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestConfigHandlersSetMultiTenantMonitor(t *testing.T) {
	handler := &ConfigHandlers{}
	handler.SetMultiTenantMonitor(nil)
	if handler.mtMonitor != nil {
		t.Fatalf("mtMonitor should be nil after SetMultiTenantMonitor(nil)")
	}
}

func TestRouterSetMultiTenantMonitor(t *testing.T) {
	router := &Router{
		alertHandlers:           &AlertHandlers{},
		notificationHandlers:    &NotificationHandlers{},
		dockerAgentHandlers:     &DockerAgentHandlers{},
		unifiedAgentHandlers:    &UnifiedAgentHandlers{},
		kubernetesAgentHandlers: &KubernetesAgentHandlers{},
		systemSettingsHandler:   &SystemSettingsHandler{},
		resourceHandlers:        NewResourceHandlers(nil),
	}

	router.SetMultiTenantMonitor(nil)

	if router.mtMonitor != nil {
		t.Fatalf("mtMonitor should be nil after SetMultiTenantMonitor(nil)")
	}
	if router.resourceHandlers.tenantStateProvider == nil {
		t.Fatalf("tenantStateProvider should be set on resource handlers")
	}
}

func TestNewRouterWiresTenantResourceStateProvider(t *testing.T) {
	cfg := &config.Config{
		DataPath:   t.TempDir(),
		ConfigPath: t.TempDir(),
	}
	mtm := &monitoring.MultiTenantMonitor{}

	router := NewRouter(cfg, nil, mtm, nil, nil, "1.0.0")
	if router.resourceHandlers == nil {
		t.Fatal("expected resource handlers to be initialized")
	}
	if router.resourceHandlers.tenantStateProvider == nil {
		t.Fatal("expected NewRouter to wire the tenant resource state provider when a multi-tenant monitor is provided")
	}
}

func TestNewRouterServesTenantResourceListWithMultiTenantMonitor(t *testing.T) {
	cfg := &config.Config{
		DataPath:   t.TempDir(),
		ConfigPath: t.TempDir(),
	}
	mtm := &monitoring.MultiTenantMonitor{}
	router := NewRouter(cfg, nil, mtm, nil, nil, "1.0.0")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?page=1&limit=100", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "tenant-a"))

	router.resourceHandlers.HandleListResources(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}
