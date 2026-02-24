package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
	if len(snap.VMs) != 0 {
		t.Fatalf("expected empty snapshot for tenant error, got %#v", snap.VMs)
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

func TestRouterConfigureMonitorDependencies_UsesTenantSpecificResourceAdapters(t *testing.T) {
	defaultAdapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	router := &Router{
		monitorResourceAdapter:  defaultAdapter,
		monitorResourceAdapters: make(map[string]*unifiedresources.MonitorAdapter),
	}

	defaultMonitor := &monitoring.Monitor{}
	router.configureMonitorDependencies(defaultMonitor)
	defaultReadState := defaultMonitor.GetUnifiedReadState()
	if defaultReadState == nil {
		t.Fatal("expected default monitor read state to be configured")
	}
	if defaultReadState != defaultAdapter {
		t.Fatalf("expected default monitor to use default adapter, got %#v", defaultReadState)
	}

	tenantMonitor := &monitoring.Monitor{}
	tenantMonitor.SetOrgID("tenant-1")
	router.configureMonitorDependencies(tenantMonitor)
	tenantReadState := tenantMonitor.GetUnifiedReadState()
	if tenantReadState == nil {
		t.Fatal("expected tenant monitor read state to be configured")
	}
	if tenantReadState == defaultAdapter {
		t.Fatal("expected tenant monitor adapter to differ from default adapter")
	}

	router.configureMonitorDependencies(tenantMonitor)
	if second := tenantMonitor.GetUnifiedReadState(); second != tenantReadState {
		t.Fatal("expected tenant monitor to reuse stable adapter for same org")
	}
}

func TestRouterDefaultUnifiedResourceProvider_PrefersMonitorScopedAdapter(t *testing.T) {
	defaultAdapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	defaultMonitor := &monitoring.Monitor{}
	defaultMonitor.SetResourceStore(defaultAdapter)

	router := &Router{
		monitor:                 defaultMonitor,
		monitorResourceAdapter:  unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
		monitorResourceAdapters: map[string]*unifiedresources.MonitorAdapter{},
	}

	provider := router.defaultUnifiedResourceProvider()
	if provider == nil {
		t.Fatal("expected default unified provider")
	}
	if provider != defaultAdapter {
		t.Fatalf("expected monitor-scoped adapter, got %#v", provider)
	}
}

func TestRouterPersistenceForOrg_UsesTenantPersistence(t *testing.T) {
	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	tenantPersistence, err := mtp.GetPersistence("tenant-1")
	if err != nil {
		t.Fatalf("GetPersistence tenant-1: %v", err)
	}

	router := &Router{
		persistence: config.NewConfigPersistence(tempDir),
		multiTenant: mtp,
	}
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")

	got := router.persistenceForOrg(ctx)
	if got != tenantPersistence {
		t.Fatalf("expected tenant persistence, got %#v", got)
	}
}

func TestRouterPersistenceForOrg_NonDefaultDoesNotFallbackToDefault(t *testing.T) {
	router := &Router{persistence: config.NewConfigPersistence(t.TempDir())}
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")

	if got := router.persistenceForOrg(ctx); got != nil {
		t.Fatalf("expected nil persistence for non-default org without mt persistence, got %#v", got)
	}
}

func TestRouterStopPatrolForOrg_ClearsLifecycleMarker(t *testing.T) {
	router := &Router{
		startedPatrolOrgs: map[string]bool{
			"default":  true,
			"tenant-1": true,
		},
	}

	router.StopPatrolForOrg("tenant-1")

	if router.startedPatrolOrgs["tenant-1"] {
		t.Fatal("expected tenant patrol marker to be cleared")
	}
	if !router.startedPatrolOrgs["default"] {
		t.Fatal("expected default marker to remain set")
	}
}

func TestRouterStopPatrol_ClearsAllLifecycleMarkers(t *testing.T) {
	router := &Router{
		aiSettingsHandler: &AISettingsHandler{},
		startedPatrolOrgs: map[string]bool{
			"default":  true,
			"tenant-1": true,
		},
	}

	router.StopPatrol()

	if len(router.startedPatrolOrgs) != 0 {
		t.Fatalf("expected all patrol markers cleared, got %#v", router.startedPatrolOrgs)
	}
}

func TestStartPatrolForContext_DoesNotOverwriteOtherTenantPatrolComponents(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "true")

	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)

	defaultMonitor, _, _ := newTestMonitor(t)
	tenantOneMonitor, _, _ := newTestMonitor(t)
	tenantTwoMonitor, _, _ := newTestMonitor(t)

	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"default":  defaultMonitor,
		"tenant-1": tenantOneMonitor,
		"tenant-2": tenantTwoMonitor,
	})

	router := &Router{
		monitor:                 defaultMonitor,
		mtMonitor:               mtm,
		multiTenant:             mtp,
		aiSettingsHandler:       NewAISettingsHandler(mtp, mtm, nil),
		startedPatrolOrgs:       make(map[string]bool),
		monitorResourceAdapters: make(map[string]*unifiedresources.MonitorAdapter),
	}
	router.aiSettingsHandler.SetStateProvider(&stubStateProvider{})
	defer router.ShutdownAIIntelligence()

	ctxTenantOne := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	if ok := router.startPatrolForContext(ctxTenantOne, "tenant-1"); !ok {
		t.Fatal("expected tenant-1 patrol start to succeed")
	}
	svcTenantOne := router.aiSettingsHandler.GetAIService(ctxTenantOne)
	if svcTenantOne == nil {
		t.Fatal("expected tenant-1 AI service")
	}
	patrolTenantOne := svcTenantOne.GetPatrolService()
	if patrolTenantOne == nil {
		t.Fatal("expected tenant-1 patrol service")
	}
	baselineOne := patrolTenantOne.GetBaselineStore()
	changeDetectorOne := patrolTenantOne.GetChangeDetector()
	remediationLogOne := patrolTenantOne.GetRemediationLog()
	if baselineOne == nil || changeDetectorOne == nil || remediationLogOne == nil {
		t.Fatal("expected tenant-1 patrol components to be initialized")
	}

	ctxTenantTwo := context.WithValue(context.Background(), OrgIDContextKey, "tenant-2")
	if ok := router.startPatrolForContext(ctxTenantTwo, "tenant-2"); !ok {
		t.Fatal("expected tenant-2 patrol start to succeed")
	}

	if got := patrolTenantOne.GetBaselineStore(); got != baselineOne {
		t.Fatal("expected tenant-1 baseline store to remain unchanged after tenant-2 startup")
	}
	if got := patrolTenantOne.GetChangeDetector(); got != changeDetectorOne {
		t.Fatal("expected tenant-1 change detector to remain unchanged after tenant-2 startup")
	}
	if got := patrolTenantOne.GetRemediationLog(); got != remediationLogOne {
		t.Fatal("expected tenant-1 remediation log to remain unchanged after tenant-2 startup")
	}
}

func TestStartPatrolForContext_RejectsMismatchedAIServiceOrg(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "true")

	mtp := config.NewMultiTenantPersistence(t.TempDir())

	defaultMonitor, _, _ := newTestMonitor(t)
	tenantMonitor, _, _ := newTestMonitor(t)

	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"default":  defaultMonitor,
		"tenant-1": tenantMonitor,
	})

	handler := NewAISettingsHandler(mtp, mtm, nil)
	legacySvc := ai.NewService(config.NewConfigPersistence(t.TempDir()), nil)
	legacySvc.SetOrgID("default")
	handler.aiServices["tenant-1"] = legacySvc

	router := &Router{
		monitor:                 defaultMonitor,
		mtMonitor:               mtm,
		aiSettingsHandler:       handler,
		startedPatrolOrgs:       make(map[string]bool),
		monitorResourceAdapters: make(map[string]*unifiedresources.MonitorAdapter),
	}

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	if ok := router.startPatrolForContext(ctx, "tenant-1"); ok {
		t.Fatal("expected patrol start to fail when AI service org scope mismatches tenant org")
	}
}
