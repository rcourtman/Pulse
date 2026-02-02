package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestRouterGetMonitor_Defaults(t *testing.T) {
	defaultMonitor, _, _ := newTestMonitor(t)
	router := &Router{monitor: defaultMonitor}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	monitor, err := router.getMonitor(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if monitor != defaultMonitor {
		t.Fatalf("expected default monitor to be returned")
	}
}

func TestRouterGetMonitor_WithTenant(t *testing.T) {
	defaultMonitor, _, _ := newTestMonitor(t)
	tenantMonitor, _, _ := newTestMonitor(t)

	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"tenant-1": tenantMonitor,
	})

	router := &Router{monitor: defaultMonitor, mtMonitor: mtm}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), OrgIDContextKey, "tenant-1")
	req = req.WithContext(ctx)

	monitor, err := router.getMonitor(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if monitor != tenantMonitor {
		t.Fatalf("expected tenant monitor to be returned")
	}
}

func TestMultiTenantStateProvider_DefaultAndTenant(t *testing.T) {
	defaultMonitor, defaultState, _ := newTestMonitor(t)
	defaultState.VMs = []models.VM{{ID: "vm-default"}}

	tenantMonitor, tenantState, _ := newTestMonitor(t)
	tenantState.VMs = []models.VM{{ID: "vm-tenant"}}

	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"tenant-1": tenantMonitor,
	})

	provider := NewMultiTenantStateProvider(mtm, defaultMonitor)

	snapDefault := provider.GetStateForTenant("")
	if len(snapDefault.VMs) != 1 || snapDefault.VMs[0].ID != "vm-default" {
		t.Fatalf("unexpected default snapshot: %#v", snapDefault.VMs)
	}
	snapDefault = provider.GetStateForTenant("default")
	if len(snapDefault.VMs) != 1 || snapDefault.VMs[0].ID != "vm-default" {
		t.Fatalf("unexpected default snapshot: %#v", snapDefault.VMs)
	}

	snapTenant := provider.GetStateForTenant("tenant-1")
	if len(snapTenant.VMs) != 1 || snapTenant.VMs[0].ID != "vm-tenant" {
		t.Fatalf("unexpected tenant snapshot: %#v", snapTenant.VMs)
	}
}

func TestMultiTenantStateProvider_FallbackOnError(t *testing.T) {
	defaultMonitor, defaultState, _ := newTestMonitor(t)
	defaultState.VMs = []models.VM{{ID: "vm-default"}}

	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	defer mtm.Stop()

	provider := NewMultiTenantStateProvider(mtm, defaultMonitor)

	snap := provider.GetStateForTenant("../bad")
	if len(snap.VMs) != 1 || snap.VMs[0].ID != "vm-default" {
		t.Fatalf("expected fallback snapshot, got %#v", snap.VMs)
	}
}

func TestSetMultiTenantMonitor_WiresHandlers(t *testing.T) {
	defaultMonitor, _, _ := newTestMonitor(t)
	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"default": defaultMonitor,
	})

	router := &Router{
		alertHandlers:           &AlertHandlers{},
		notificationHandlers:    &NotificationHandlers{},
		dockerAgentHandlers:     &DockerAgentHandlers{},
		hostAgentHandlers:       &HostAgentHandlers{},
		kubernetesAgentHandlers: &KubernetesAgentHandlers{},
		systemSettingsHandler:   &SystemSettingsHandler{},
		resourceHandlers:        &ResourceHandlers{},
	}

	router.SetMultiTenantMonitor(mtm)

	if router.mtMonitor != mtm {
		t.Fatalf("expected router mtMonitor to be updated")
	}
	if router.monitor != defaultMonitor {
		t.Fatalf("expected router monitor to be set to default monitor")
	}
	if router.alertHandlers.mtMonitor != mtm {
		t.Fatalf("expected alertHandlers mtMonitor to be set")
	}
	if router.notificationHandlers.mtMonitor != mtm {
		t.Fatalf("expected notificationHandlers mtMonitor to be set")
	}
	if router.dockerAgentHandlers.mtMonitor != mtm {
		t.Fatalf("expected dockerAgentHandlers mtMonitor to be set")
	}
	if router.hostAgentHandlers.mtMonitor != mtm {
		t.Fatalf("expected hostAgentHandlers mtMonitor to be set")
	}
	if router.kubernetesAgentHandlers.mtMonitor != mtm {
		t.Fatalf("expected kubernetesAgentHandlers mtMonitor to be set")
	}
	if router.systemSettingsHandler.mtMonitor != mtm {
		t.Fatalf("expected systemSettingsHandler mtMonitor to be set")
	}
	if router.resourceHandlers.tenantStateProvider == nil {
		t.Fatalf("expected tenant state provider to be set")
	}
}
