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
		configHandlers:          &ConfigHandlers{},
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

func TestRouterSetMultiTenantMonitorRefreshesConfigHandlerMonitorSource(t *testing.T) {
	oldMonitor, _, _ := newTestMonitor(t)
	newMonitor, _, _ := newTestMonitor(t)

	oldConfig := &config.Config{DataPath: t.TempDir(), ConfigPath: t.TempDir()}
	newConfig := &config.Config{DataPath: t.TempDir(), ConfigPath: t.TempDir()}
	setUnexportedField(t, oldMonitor, "config", oldConfig)
	setUnexportedField(t, newMonitor, "config", newConfig)

	oldMTM := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, oldMTM, "monitors", map[string]*monitoring.Monitor{
		"default": oldMonitor,
	})
	newMTM := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, newMTM, "monitors", map[string]*monitoring.Monitor{
		"default": newMonitor,
	})

	router := &Router{
		configHandlers: &ConfigHandlers{},
	}
	router.configHandlers.SetMultiTenantMonitor(oldMTM)

	if got := router.configHandlers.getMonitor(context.Background()); got != oldMonitor {
		t.Fatalf("precondition monitor = %#v, want old monitor %#v", got, oldMonitor)
	}

	router.SetMultiTenantMonitor(newMTM)

	if got := router.configHandlers.getMonitor(context.Background()); got != newMonitor {
		t.Fatalf("config handler monitor = %#v, want reloaded monitor %#v", got, newMonitor)
	}
	if got := router.configHandlers.getConfig(context.Background()); got != newConfig {
		t.Fatalf("config handler config = %#v, want reloaded config %#v", got, newConfig)
	}
}

func TestConfigHandlersNonDefaultMissingTenantMonitorFailsClosed(t *testing.T) {
	defaultConfig := &config.Config{DataPath: t.TempDir(), ConfigPath: t.TempDir()}
	defaultMonitor, _, _ := newTestMonitor(t)
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mtm := monitoring.NewMultiTenantMonitor(defaultConfig, mtp, nil)
	defer mtm.Stop()

	handler := &ConfigHandlers{
		defaultConfig:      defaultConfig,
		defaultPersistence: config.NewConfigPersistence(defaultConfig.ConfigPath),
		defaultMonitor:     defaultMonitor,
	}
	handler.SetMultiTenantMonitor(mtm)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-missing")
	cfg, persistence, monitor := handler.getContextState(ctx)
	if cfg != nil || persistence != nil || monitor != nil {
		t.Fatalf("expected missing non-default tenant state to fail closed, got cfg=%#v persistence=%#v monitor=%#v", cfg, persistence, monitor)
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
