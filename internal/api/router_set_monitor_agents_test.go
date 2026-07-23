package api

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// TestRouterSetMonitor_UpdatesAgentHandlers verifies the fix for issue #1558:
// Router.SetMonitor must repoint the kubernetes agent handlers at the new
// monitor, exactly as it does for the docker agent handlers. Without this,
// kubernetes agent reports resolved through the single-tenant fallback keep
// landing in the orphaned pre-reload monitor after a config reload, so the
// cluster flaps in the UI.
func TestRouterSetMonitor_UpdatesAgentHandlers(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	monitor1, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New (initial): %v", err)
	}
	t.Cleanup(func() { monitor1.Stop() })

	monitor2, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New (reload): %v", err)
	}
	t.Cleanup(func() { monitor2.Stop() })

	// No multi-tenant monitor: getMonitor resolves via the defaultMonitor
	// fallback, which is exactly the field SetMonitor must refresh.
	kubernetesHandlers := NewKubernetesAgentHandlers(nil, monitor1, nil)
	dockerHandlers := NewDockerAgentHandlers(nil, monitor1, nil, cfg)
	multiTenantPersistence := config.NewMultiTenantPersistence(tempDir)

	// Minimal router, as in TestReloadSystemSettings_AppliesWebhookCIDRsToNewMonitor:
	// SetMonitor only touches the handlers that are non-nil.
	router := &Router{
		config:                  cfg,
		persistence:             config.NewConfigPersistence(tempDir),
		multiTenant:             multiTenantPersistence,
		kubernetesAgentHandlers: kubernetesHandlers,
		dockerAgentHandlers:     dockerHandlers,
		aiSettingsHandler:       NewAISettingsHandler(multiTenantPersistence, nil, nil),
	}

	router.SetMonitor(monitor2)

	ctx := context.Background()
	if got := kubernetesHandlers.getMonitor(ctx); got != monitor2 {
		t.Fatal("kubernetes agent handlers still resolve the pre-reload monitor after Router.SetMonitor")
	}
	if got := dockerHandlers.getMonitor(ctx); got != monitor2 {
		t.Fatal("docker agent handlers still resolve the pre-reload monitor after Router.SetMonitor")
	}
	if got := router.persistence.GetGuestMetadataStore(); got != monitor2.GuestMetadataStore() {
		t.Fatal("config persistence still resolves the pre-reload guest metadata store")
	}
	if got := router.persistence.GetDockerMetadataStore(); got != monitor2.DockerMetadataStore() {
		t.Fatal("config persistence still resolves the pre-reload Docker metadata store")
	}
	if got := router.persistence.GetHostMetadataStore(); got != monitor2.HostMetadataStore() {
		t.Fatal("config persistence still resolves the pre-reload host metadata store")
	}
	if err := router.aiSettingsHandler.GetAIService(ctx).SetResourceURL(
		"vm",
		"instance:node:100",
		"https://guest.internal",
	); err != nil {
		t.Fatalf("AI metadata update after monitor reload: %v", err)
	}
	if meta := monitor2.GuestMetadataStore().Get("instance:node:100"); meta == nil || meta.CustomURL != "https://guest.internal" {
		t.Fatalf("replacement monitor guest metadata = %#v", meta)
	}
	if meta := monitor1.GuestMetadataStore().Get("instance:node:100"); meta != nil {
		t.Fatalf("pre-reload monitor received URL update: %#v", meta)
	}
}
