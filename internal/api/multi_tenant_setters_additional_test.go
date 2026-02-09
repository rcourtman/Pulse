package api

import "testing"

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
		hostAgentHandlers:       &HostAgentHandlers{},
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
