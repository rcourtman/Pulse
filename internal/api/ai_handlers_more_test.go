package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type stubMetadataProvider struct{}

func (stubMetadataProvider) SetGuestURL(string, string) error  { return nil }
func (stubMetadataProvider) SetDockerURL(string, string) error { return nil }
func (stubMetadataProvider) SetHostURL(string, string) error   { return nil }

type recordingMetadataProvider struct {
	guestURLs map[string]string
}

func TestTenantAgentServerForOrganizationFailsClosedAcrossTenants(t *testing.T) {
	admissions := map[string]agentexec.AgentAdmission{
		"token-a": {OrganizationID: "tenant-a", TokenID: "token-a", AgentID: "agent-a", Hostname: "host-a"},
		"token-b": {OrganizationID: "tenant-b", TokenID: "token-b", AgentID: "agent-b", Hostname: "host-b"},
	}
	server := agentexec.NewServerWithAdmissionValidator(
		func(token, _, _ string) (agentexec.AgentAdmission, bool) {
			admission, ok := admissions[token]
			return admission, ok
		},
		func(agentexec.AgentAdmission) bool { return true },
	)
	websocketServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer websocketServer.Close()

	register := func(admission agentexec.AgentAdmission) *websocket.Conn {
		t.Helper()
		url := "ws" + strings.TrimPrefix(websocketServer.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{
			"Origin": []string{websocketServer.URL},
		})
		if err != nil {
			t.Fatalf("dial agent websocket: %v", err)
		}
		message, err := agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{
			AgentID:  admission.AgentID,
			Hostname: admission.Hostname,
			Token:    admission.TokenID,
		})
		if err != nil {
			t.Fatalf("create registration message: %v", err)
		}
		if err := conn.WriteJSON(message); err != nil {
			t.Fatalf("write registration: %v", err)
		}
		var response agentexec.Message
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatalf("read registration response: %v", err)
		}
		var registered agentexec.RegisteredPayload
		if err := response.DecodePayload(&registered); err != nil || !registered.Success {
			t.Fatalf("registration failed: payload=%+v err=%v", registered, err)
		}
		return conn
	}

	connA := register(admissions["token-a"])
	defer connA.Close()
	connB := register(admissions["token-b"])
	defer connB.Close()

	tenantA := tenantAgentServerForOrganization(server, "tenant-a")
	agents := tenantA.GetConnectedAgents()
	if len(agents) != 1 || agents[0].AgentID != "agent-a" {
		t.Fatalf("tenant-a command view leaked another tenant: %#v", agents)
	}
	if _, err := tenantA.ExecuteCommand(context.Background(), "agent-b", agentexec.ExecuteCommandPayload{
		Command: "true",
		Trusted: true,
	}); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("cross-tenant command dispatch did not fail closed: %v", err)
	}
}

func (p *recordingMetadataProvider) SetGuestURL(id, url string) error {
	p.guestURLs[id] = url
	return nil
}

func (*recordingMetadataProvider) SetDockerURL(string, string) error { return nil }
func (*recordingMetadataProvider) SetHostURL(string, string) error   { return nil }

func TestAISettingsHandlerMetadataProviderFactoryScopesExistingServices(t *testing.T) {
	handler := NewAISettingsHandler(config.NewMultiTenantPersistence(t.TempDir()), nil, nil)
	tenantContext := context.WithValue(context.Background(), OrgIDContextKey, "tenant-a")

	defaultService := handler.GetAIService(context.Background())
	tenantService := handler.GetAIService(tenantContext)
	if defaultService == nil || tenantService == nil {
		t.Fatal("expected default and tenant AI services")
	}

	providers := map[string]*recordingMetadataProvider{}
	handler.SetMetadataProviderFactory(func(orgID string) ai.MetadataProvider {
		provider := &recordingMetadataProvider{guestURLs: make(map[string]string)}
		providers[orgID] = provider
		return provider
	})

	if err := defaultService.SetResourceURL("vm", "shared-id", "https://default.internal"); err != nil {
		t.Fatalf("set default URL: %v", err)
	}
	if err := tenantService.SetResourceURL("vm", "shared-id", "https://tenant.internal"); err != nil {
		t.Fatalf("set tenant URL: %v", err)
	}
	if got := providers["default"].guestURLs["shared-id"]; got != "https://default.internal" {
		t.Fatalf("default provider URL = %q", got)
	}
	if got := providers["tenant-a"].guestURLs["shared-id"]; got != "https://tenant.internal" {
		t.Fatalf("tenant provider URL = %q", got)
	}

	newTenantContext := context.WithValue(context.Background(), OrgIDContextKey, "tenant-b")
	newTenantService := handler.GetAIService(newTenantContext)
	if err := newTenantService.SetResourceURL("vm", "shared-id", "https://new-tenant.internal"); err != nil {
		t.Fatalf("set new tenant URL: %v", err)
	}
	if got := providers["tenant-b"].guestURLs["shared-id"]; got != "https://new-tenant.internal" {
		t.Fatalf("new tenant provider URL = %q", got)
	}
}

type stubThresholdProvider struct{}

func (stubThresholdProvider) GetNodeCPUThreshold() float64    { return 80 }
func (stubThresholdProvider) GetNodeMemoryThreshold() float64 { return 85 }
func (stubThresholdProvider) GetGuestMemoryThreshold() float64 {
	return 90
}
func (stubThresholdProvider) GetGuestDiskThreshold() float64 { return 95 }
func (stubThresholdProvider) GetStorageThreshold() float64   { return 92 }
func (stubThresholdProvider) GetDiskTemperatureThreshold(alerts.DiskTemperatureHost, string) (float64, float64) {
	return 70, 65
}

type stubMetricsHistoryProvider struct{}

func (stubMetricsHistoryProvider) GetNodeMetrics(string, string, time.Duration) []ai.MetricPoint {
	return nil
}
func (stubMetricsHistoryProvider) GetGuestMetrics(string, string, time.Duration) []ai.MetricPoint {
	return nil
}
func (stubMetricsHistoryProvider) GetAllGuestMetrics(string, time.Duration) map[string][]ai.MetricPoint {
	return nil
}
func (stubMetricsHistoryProvider) GetAllStorageMetrics(string, time.Duration) map[string][]ai.MetricPoint {
	return nil
}

type stubUnifiedResourceProvider struct {
	resources []unifiedresources.Resource
}

func (s stubUnifiedResourceProvider) GetAll() []unifiedresources.Resource {
	return s.resources
}
func (s stubUnifiedResourceProvider) GetInfrastructure() []unifiedresources.Resource {
	return nil
}
func (s stubUnifiedResourceProvider) GetWorkloads() []unifiedresources.Resource {
	return nil
}
func (s stubUnifiedResourceProvider) GetByType(unifiedresources.ResourceType) []unifiedresources.Resource {
	return nil
}
func (stubUnifiedResourceProvider) GetStats() unifiedresources.ResourceStats {
	return unifiedresources.ResourceStats{}
}
func (s stubUnifiedResourceProvider) GetTopByCPU(int, []unifiedresources.ResourceType) []unifiedresources.Resource {
	return nil
}
func (s stubUnifiedResourceProvider) GetTopByMemory(int, []unifiedresources.ResourceType) []unifiedresources.Resource {
	return nil
}
func (s stubUnifiedResourceProvider) GetTopByDisk(int, []unifiedresources.ResourceType) []unifiedresources.Resource {
	return nil
}
func (s stubUnifiedResourceProvider) GetRelated(string) map[string][]unifiedresources.Resource {
	return map[string][]unifiedresources.Resource{}
}
func (stubUnifiedResourceProvider) FindContainerHost(string) string { return "" }

func TestAISettingsHandler_setSSECORSHeaders(t *testing.T) {
	handler := newTestAISettingsHandler(&config.Config{AllowedOrigins: "*"}, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/stream", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.setSSECORSHeaders(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected allow origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("expected no credentials header for wildcard origins, got %q", got)
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatalf("expected allow methods header")
	}
	if rec.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Fatalf("expected allow headers header")
	}
	if got := rec.Header().Get("Vary"); got != "" {
		t.Fatalf("expected no Vary header for wildcard policy, got %q", got)
	}

	handler = newTestAISettingsHandler(&config.Config{AllowedOrigins: "https://allowed.com"}, nil, nil)
	req = httptest.NewRequest(http.MethodGet, "/api/ai/stream", nil)
	req.Header.Set("Origin", "https://allowed.com")
	rec = httptest.NewRecorder()
	handler.setSSECORSHeaders(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://allowed.com" {
		t.Fatalf("expected allow origin for matched origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header for matched origin, got %q", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary=Origin for matched origin, got %q", got)
	}

	handler = newTestAISettingsHandler(&config.Config{AllowedOrigins: "https://allowed.com"}, nil, nil)
	req = httptest.NewRequest(http.MethodGet, "/api/ai/stream", nil)
	req.Header.Set("Origin", "https://nope.com")
	rec = httptest.NewRecorder()
	handler.setSSECORSHeaders(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow origin for mismatched origin, got %q", got)
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatalf("expected allow methods header for mismatched origin")
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary=Origin for mismatched origin, got %q", got)
	}

	handler = newTestAISettingsHandler(&config.Config{AllowedOrigins: "*"}, nil, nil)
	req = httptest.NewRequest(http.MethodGet, "/api/ai/stream", nil)
	rec = httptest.NewRecorder()
	handler.setSSECORSHeaders(rec, req)
	if len(rec.Header()) != 0 {
		t.Fatalf("expected no headers when origin is missing, got %v", rec.Header())
	}
}

func TestAISettingsHandler_GetAIService_MultiTenantProviders(t *testing.T) {
	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)
	handler := NewAISettingsHandler(mtp, nil, nil)

	handler.SetStateProvider(&stubStateProvider{})
	handler.SetUnifiedResourceProvider(stubUnifiedResourceProvider{})
	handler.SetMetadataProvider(stubMetadataProvider{})
	handler.SetPatrolThresholdProvider(stubThresholdProvider{})
	handler.SetMetricsHistoryProvider(stubMetricsHistoryProvider{})
	handler.SetBaselineStore(ai.NewBaselineStore(ai.DefaultBaselineConfig()))
	handler.SetChangeDetector(ai.NewChangeDetector(ai.ChangeDetectorConfig{}))
	handler.SetRemediationLog(ai.NewRemediationLog(ai.RemediationLogConfig{MaxRecords: 1}))
	handler.SetIncidentStore(memory.NewIncidentStore(memory.IncidentStoreConfig{DataDir: t.TempDir()}))
	handler.SetPatternDetector(ai.NewPatternDetector(ai.DefaultPatternConfig()))
	handler.SetCorrelationDetector(ai.NewCorrelationDetector(ai.DefaultCorrelationConfig()))

	discoveryStore, err := servicediscovery.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	handler.SetDiscoveryStore(discoveryStore)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	svc := handler.GetAIService(ctx)
	if svc == nil {
		t.Fatalf("expected tenant service to be created")
	}
	if svc2 := handler.GetAIService(ctx); svc2 != svc {
		t.Fatalf("expected cached tenant service")
	}
}

// Report GETs read Patrol findings through ExistingAIService, so it must never
// construct a tenant service: construction can list provider models and start
// background discovery.
func TestAISettingsHandler_ExistingAIServiceNeverConstructs(t *testing.T) {
	handler := NewAISettingsHandler(config.NewMultiTenantPersistence(t.TempDir()), nil, nil)
	t.Cleanup(handler.StopServices)
	tenant := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")

	if svc := handler.ExistingAIService(tenant); svc != nil {
		t.Fatalf("expected no service before the tenant's first GetAIService, got %p", svc)
	}
	handler.aiServicesMu.RLock()
	built := len(handler.aiServices)
	handler.aiServicesMu.RUnlock()
	if built != 0 {
		t.Fatalf("ExistingAIService constructed %d tenant services, want 0", built)
	}

	created := handler.GetAIService(tenant)
	if created == nil {
		t.Fatal("expected GetAIService to construct the tenant service")
	}
	if got := handler.ExistingAIService(tenant); got != created {
		t.Fatalf("expected the running tenant service %p, got %p", created, got)
	}
	if got, want := handler.ExistingAIService(context.Background()), handler.GetAIService(context.Background()); got != want {
		t.Fatalf("expected the default service %p, got %p", want, got)
	}
}

func TestAISettingsHandler_GetAIService_UsesTenantReadState(t *testing.T) {
	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	t.Cleanup(mtm.Stop)

	tenantAdapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	tenantMonitor := &monitoring.Monitor{}
	tenantMonitor.SetResourceStore(tenantAdapter)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"tenant-1": tenantMonitor})

	handler := NewAISettingsHandler(mtp, mtm, nil)
	handler.SetReadState(unifiedresources.NewRegistry(nil))

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	svc := handler.GetAIService(ctx)
	if svc == nil {
		t.Fatalf("expected tenant AI service")
	}

	field := reflect.ValueOf(svc).Elem().FieldByName("readState")
	ptr := unsafe.Pointer(field.UnsafeAddr())
	current := reflect.NewAt(field.Type(), ptr).Elem().Interface().(unifiedresources.ReadState)
	if current != tenantAdapter {
		t.Fatalf("expected tenant read state adapter, got %#v", current)
	}
}

func TestAISettingsHandler_GetAIService_UsesTenantUnifiedResourceProvider(t *testing.T) {
	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	t.Cleanup(mtm.Stop)

	tenantAdapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	tenantMonitor := &monitoring.Monitor{}
	tenantMonitor.SetResourceStore(tenantAdapter)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"tenant-1": tenantMonitor})

	handler := NewAISettingsHandler(mtp, mtm, nil)
	handler.SetUnifiedResourceProvider(stubUnifiedResourceProvider{})

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	svc := handler.GetAIService(ctx)
	if svc == nil {
		t.Fatalf("expected tenant AI service")
	}

	field := reflect.ValueOf(svc).Elem().FieldByName("unifiedResourceProvider")
	ptr := unsafe.Pointer(field.UnsafeAddr())
	current := reflect.NewAt(field.Type(), ptr).Elem().Interface().(ai.UnifiedResourceProvider)
	if current != tenantAdapter {
		t.Fatalf("expected tenant unified provider adapter, got %#v", current)
	}
}

func TestAISettingsHandler_GetAIService_NonDefaultWithoutTenantReadStateFailsClosed(t *testing.T) {
	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	t.Cleanup(mtm.Stop)

	handler := NewAISettingsHandler(mtp, mtm, nil)
	handler.SetReadState(unifiedresources.NewRegistry(nil))

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	svc := handler.GetAIService(ctx)
	if svc == nil {
		t.Fatalf("expected tenant AI service")
	}

	field := reflect.ValueOf(svc).Elem().FieldByName("readState")
	ptr := unsafe.Pointer(field.UnsafeAddr())
	current := reflect.NewAt(field.Type(), ptr).Elem().Interface()
	if current != nil {
		t.Fatalf("expected nil tenant read state when tenant monitor read-state is unavailable, got %#v", current)
	}
}

func TestAISettingsHandler_GetAIService_NonDefaultWithoutTenantUnifiedProviderFailsClosed(t *testing.T) {
	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	t.Cleanup(mtm.Stop)

	handler := NewAISettingsHandler(mtp, mtm, nil)
	handler.SetUnifiedResourceProvider(stubUnifiedResourceProvider{})

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	svc := handler.GetAIService(ctx)
	if svc == nil {
		t.Fatalf("expected tenant AI service")
	}

	field := reflect.ValueOf(svc).Elem().FieldByName("unifiedResourceProvider")
	ptr := unsafe.Pointer(field.UnsafeAddr())
	current := reflect.NewAt(field.Type(), ptr).Elem().Interface()
	if current != nil {
		t.Fatalf("expected nil tenant unified provider when tenant monitor provider is unavailable, got %#v", current)
	}
}

func TestAISettingsHandler_SetUnifiedResourceProvider_ReappliesTenantScopedProvider(t *testing.T) {
	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	t.Cleanup(mtm.Stop)

	tenantAdapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	tenantMonitor := &monitoring.Monitor{}
	tenantMonitor.SetResourceStore(tenantAdapter)
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{"tenant-1": tenantMonitor})

	handler := NewAISettingsHandler(mtp, mtm, nil)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	svc := handler.GetAIService(ctx)
	if svc == nil {
		t.Fatalf("expected tenant AI service")
	}

	handler.SetUnifiedResourceProvider(stubUnifiedResourceProvider{})

	field := reflect.ValueOf(svc).Elem().FieldByName("unifiedResourceProvider")
	ptr := unsafe.Pointer(field.UnsafeAddr())
	current := reflect.NewAt(field.Type(), ptr).Elem().Interface().(ai.UnifiedResourceProvider)
	if current != tenantAdapter {
		t.Fatalf("expected tenant scoped provider to be preserved, got %#v", current)
	}
}

func TestAISettingsHandler_GetAIService_TenantUsesCanonicalReadStateWithoutSnapshotProvider(t *testing.T) {
	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)

	defaultMonitor, defaultState, _ := newTestMonitor(t)
	defaultState.VMs = []models.VM{{ID: "vm-default"}}

	tenantAdapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	tenantMonitor := &monitoring.Monitor{}
	tenantMonitor.SetResourceStore(tenantAdapter)

	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"default":  defaultMonitor,
		"tenant-1": tenantMonitor,
	})

	handler := NewAISettingsHandler(mtp, mtm, nil)
	handler.SetStateProvider(defaultMonitor)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	svc := handler.GetAIService(ctx)
	if svc == nil {
		t.Fatalf("expected tenant AI service")
	}
	if svc.GetStateProvider() != nil {
		t.Fatalf("expected tenant AI service to avoid snapshot provider bridge")
	}
	if svc.GetPatrolService() == nil {
		t.Fatalf("expected patrol service to initialize from tenant read state")
	}

	field := reflect.ValueOf(svc).Elem().FieldByName("readState")
	ptr := unsafe.Pointer(field.UnsafeAddr())
	current := reflect.NewAt(field.Type(), ptr).Elem().Interface().(unifiedresources.ReadState)
	if current != tenantAdapter {
		t.Fatalf("expected tenant read state adapter, got %#v", current)
	}
}

func TestAISettingsHandler_GetAIService_NonDefaultDoesNotInheritDefaultDiscoveryStore(t *testing.T) {
	tmp := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tmp)
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	t.Cleanup(mtm.Stop)

	handler := NewAISettingsHandler(mtp, mtm, nil)
	defaultDiscoveryStore, err := servicediscovery.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("default NewStore: %v", err)
	}
	handler.SetDiscoveryStore(defaultDiscoveryStore)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	svc := handler.GetAIService(ctx)
	if svc == nil {
		t.Fatalf("expected tenant AI service")
	}
	if got := svc.GetDiscoveryStore(); got == nil {
		t.Fatalf("expected tenant service to initialize its own discovery store")
	} else if got == defaultDiscoveryStore {
		t.Fatalf("expected tenant service discovery store to differ from default store")
	}

	tenantDiscoveryStore, err := servicediscovery.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("tenant NewStore: %v", err)
	}
	handler.SetDiscoveryStoreForOrg("tenant-1", tenantDiscoveryStore)
	if got := svc.GetDiscoveryStore(); got != tenantDiscoveryStore {
		t.Fatalf("expected tenant-specific discovery store, got %#v", got)
	}
}

func TestAISettingsHandler_DiscoveryStoreAccessors(t *testing.T) {
	handler := newTestAISettingsHandler(&config.Config{DataPath: t.TempDir()}, config.NewConfigPersistence(t.TempDir()), nil)

	store, err := servicediscovery.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	handler.SetDiscoveryStore(store)

	if got := handler.GetDiscoveryStore(); got != store {
		t.Fatalf("expected discovery store to match")
	}
}

func TestAISettingsHandler_GetConfig_NonDefaultFallsBackWhenMultiTenantUnavailable(t *testing.T) {
	handler := newTestAISettingsHandler(&config.Config{APIToken: "token"}, config.NewConfigPersistence(t.TempDir()), nil)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "acme")

	if got := handler.getConfig(ctx); got == nil {
		t.Fatalf("expected legacy config fallback for non-default org without tenant monitor")
	}
}

func TestAISettingsHandler_GetPersistence_NonDefaultFallsBackWhenMultiTenantUnavailable(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	handler := newTestAISettingsHandler(&config.Config{DataPath: t.TempDir()}, persistence, nil)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "acme")

	if got := handler.getPersistence(ctx); got != persistence {
		t.Fatalf("expected legacy persistence fallback for non-default org without tenant persistence, got %#v", got)
	}
}

func TestAISettingsHandler_GetConfig_NonDefaultInvalidOrgFailsClosedWhenMultiTenantAvailable(t *testing.T) {
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	t.Cleanup(mtm.Stop)

	handler := NewAISettingsHandler(mtp, mtm, nil)
	handler.SetConfig(&config.Config{APIToken: "token"})
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "../bad")

	if got := handler.getConfig(ctx); got != nil {
		t.Fatalf("expected nil config for invalid non-default org in multi-tenant mode, got %#v", got)
	}
}

func TestAISettingsHandler_GetPersistence_NonDefaultInvalidOrgFailsClosedWhenMultiTenantAvailable(t *testing.T) {
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	t.Cleanup(mtm.Stop)

	handler := NewAISettingsHandler(mtp, mtm, nil)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "../bad")

	if got := handler.getPersistence(ctx); got != nil {
		t.Fatalf("expected nil persistence for invalid non-default org in multi-tenant mode, got %#v", got)
	}
}

func TestAISettingsHandler_GetAIService_NonDefaultInvalidOrgReturnsFailClosedTenantService(t *testing.T) {
	setMockModeForTest(t, false)

	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	t.Cleanup(mtm.Stop)

	handler := NewAISettingsHandler(mtp, mtm, nil)
	defaultSvc := handler.GetAIService(context.Background())
	if defaultSvc == nil {
		t.Fatal("expected default AI service to be available")
	}

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "../bad")
	tenantSvc := handler.GetAIService(ctx)
	if tenantSvc == nil {
		t.Fatal("expected fail-closed tenant service")
	}
	if tenantSvc == defaultSvc {
		t.Fatal("expected non-default invalid org to not fall back to default legacy service")
	}
	if got := tenantSvc.GetOrgID(); got != "../bad" {
		t.Fatalf("expected tenant org id to be preserved, got %q", got)
	}
	if tenantSvc.IsEnabled() {
		t.Fatal("expected fail-closed tenant service to be disabled")
	}
	if tenantSvc.HasLicenseFeature(ai.FeatureAIAutoFix) {
		t.Fatal("expected fail-closed tenant service license checker to deny features")
	}
}

func TestNewAISettingsHandler_DefaultServiceAlwaysInitialized(t *testing.T) {
	setMockModeForTest(t, false)

	handler := NewAISettingsHandler(nil, nil, nil)
	svc := handler.GetAIService(context.Background())
	if svc == nil {
		t.Fatal("expected default AI service to be initialized even without persistence")
	}
	if got := svc.GetOrgID(); got != "default" {
		t.Fatalf("expected default org id, got %q", got)
	}
}

func TestAISettingsHandler_GetAIService_NonDefaultWithTenantMonitorWithoutPersistenceFailsClosed(t *testing.T) {
	setMockModeForTest(t, false)

	mtp := config.NewMultiTenantPersistence(t.TempDir())
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{}, mtp, nil)
	t.Cleanup(mtm.Stop)

	handler := NewAISettingsHandler(nil, mtm, nil)
	defaultSvc := handler.GetAIService(context.Background())
	if defaultSvc == nil {
		t.Fatal("expected default AI service to be available")
	}

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "tenant-1")
	tenantSvc := handler.GetAIService(ctx)
	if tenantSvc == nil {
		t.Fatal("expected fail-closed tenant service")
	}
	if tenantSvc == defaultSvc {
		t.Fatal("expected non-default org to not fall back to default service when tenant monitor is present")
	}
	if got := tenantSvc.GetOrgID(); got != "tenant-1" {
		t.Fatalf("expected tenant org id, got %q", got)
	}
	if tenantSvc.IsEnabled() {
		t.Fatal("expected fail-closed tenant service to be disabled")
	}
}

// TestHandleGetPatrolDigest_PayloadContract pins the wire shape the Patrol
// "This week" card and docs/API.md describe: snake_case rollup groups, an
// explicit window with coverage flags, and by_outcome serialised as an object.
func TestHandleGetPatrolDigest_PayloadContract(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/digest?days=14", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetPatrolDigest(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"generated_at", "window", "mode", "runs", "findings", "investigations", "actions", "alerts", "spend"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("digest payload missing %q: %s", key, rec.Body.String())
		}
	}

	var window struct {
		Days            int  `json:"days"`
		HistoryComplete bool `json:"history_complete"`
	}
	if err := json.Unmarshal(payload["window"], &window); err != nil {
		t.Fatal(err)
	}
	if window.Days != 14 || !window.HistoryComplete {
		t.Fatalf("window = %+v", window)
	}

	var investigations map[string]json.RawMessage
	if err := json.Unmarshal(payload["investigations"], &investigations); err != nil {
		t.Fatal(err)
	}
	if string(investigations["by_outcome"]) != "{}" {
		t.Fatalf("by_outcome must serialise as an empty object, got %s", investigations["by_outcome"])
	}

	var findings map[string]json.RawMessage
	if err := json.Unmarshal(payload["findings"], &findings); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"new", "open_by_severity", "resolved", "auto_resolved", "dismissed", "suppressed"} {
		if _, ok := findings[key]; !ok {
			t.Fatalf("findings rollup missing %q: %s", key, payload["findings"])
		}
	}
	if strings.TrimSpace(string(payload["mode"])) != `"monitor"` {
		t.Fatalf("mode = %s, want monitor when no autonomy is configured", payload["mode"])
	}
}

// TestBuildPatrolDigestReportsAvailability pins the contract the scheduled
// Patrol summary relies on: without a Patrol service the digest is the zero
// shape and reported as unavailable, so a schedule fails clearly instead of
// emailing an empty week.
func TestBuildPatrolDigestReportsAvailability(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	digest, available := handler.BuildPatrolDigest(context.Background(), 0)
	if available {
		t.Fatal("digest must report unavailable without a Patrol service")
	}
	if digest.Window.Days != ai.PatrolDigestDefaultDays || digest.Runs.Total != 0 || digest.Mode != config.PatrolAutonomyMonitor {
		t.Fatalf("zero digest = %+v", digest)
	}
	if digest.Investigations.ByOutcome == nil {
		t.Fatal("by_outcome must stay an object for clients and email rendering")
	}

	patrol := &ai.PatrolService{}
	setUnexportedField(t, patrol, "runHistoryStore", ai.NewPatrolRunHistoryStore(10))
	setUnexportedField(t, handler.defaultAIService, "patrolService", patrol)
	if _, available := handler.BuildPatrolDigest(context.Background(), 45); !available {
		t.Fatal("digest must report available once a Patrol service exists")
	}
}

// Pulse #2350: the usage export must carry the prompt-cache buckets and price
// them at the provider's cache rates, in both formats.
func TestHandleExportAICostHistoryCarriesPromptCacheBuckets(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)
	store := handler.defaultAIService.CostStore()
	if store == nil {
		t.Fatal("expected the AI service to own a cost store")
	}
	store.Record(cost.UsageEvent{
		Timestamp:                time.Now(),
		Provider:                 "anthropic",
		RequestModel:             "anthropic:claude-sonnet-5",
		UseCase:                  "patrol",
		InputTokens:              100_000,
		OutputTokens:             10_000,
		CacheCreationInputTokens: 200_000,
		CacheReadInputTokens:     1_000_000,
	})
	// 0.1M*2.00 + 0.01M*10.00 + 0.2M*2.50 + 1.0M*0.20 at Anthropic cache rates.
	const wantUSD = 1.0

	req := newLoopbackRequest(http.MethodGet, "/api/ai/cost/export?days=7&format=json", nil)
	rec := httptest.NewRecorder()
	handler.HandleExportAICostHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("json export status = %d: %s", rec.Code, rec.Body.String())
	}
	var exported struct {
		Events []struct {
			cost.UsageEvent
			EstimatedUSD float64 `json:"estimated_usd"`
			PricingKnown bool    `json:"pricing_known"`
		} `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &exported); err != nil {
		t.Fatalf("decode json export: %v", err)
	}
	if len(exported.Events) != 1 {
		t.Fatalf("expected one exported event, got %d", len(exported.Events))
	}
	got := exported.Events[0]
	if got.CacheCreationInputTokens != 200_000 || got.CacheReadInputTokens != 1_000_000 || got.InputTokens != 100_000 {
		t.Fatalf("json export lost cache buckets: %+v", got.UsageEvent)
	}
	if !got.PricingKnown || got.EstimatedUSD < wantUSD-1e-6 || got.EstimatedUSD > wantUSD+1e-6 {
		t.Fatalf("json export estimated_usd = %f (known=%v), want %f", got.EstimatedUSD, got.PricingKnown, wantUSD)
	}

	req = newLoopbackRequest(http.MethodGet, "/api/ai/cost/export?days=7&format=csv", nil)
	rec = httptest.NewRecorder()
	handler.HandleExportAICostHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("csv export status = %d: %s", rec.Code, rec.Body.String())
	}
	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header and one row, got %d lines: %q", len(lines), rec.Body.String())
	}
	if !strings.Contains(lines[0], "input_tokens,output_tokens,cache_creation_input_tokens,cache_read_input_tokens,estimated_usd,") {
		t.Fatalf("csv header missing cache columns after output_tokens: %s", lines[0])
	}
	if !strings.Contains(lines[1], ",100000,10000,200000,1000000,1.000000,true,") {
		t.Fatalf("csv row missing cache buckets or cache-rate pricing: %s", lines[1])
	}
}

// sessionMutationRecordingAIService records every Assistant session mutation
// and every running check that reaches the service, so a rejected request can
// be proven to stop before the handler touches the service at all.
type sessionMutationRecordingAIService struct {
	capturingAIService
	mu           sync.Mutex
	mutations    []string
	runningCalls int
}

func newSessionMutationRecordingAIService() *sessionMutationRecordingAIService {
	return &sessionMutationRecordingAIService{capturingAIService: capturingAIService{running: true}}
}

func (s *sessionMutationRecordingAIService) record(op, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mutations = append(s.mutations, op+":"+sessionID)
}

func (s *sessionMutationRecordingAIService) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.mutations...)
}

func (s *sessionMutationRecordingAIService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runningCalls++
	return s.capturingAIService.running
}

func (s *sessionMutationRecordingAIService) runningChecks() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runningCalls
}

func (s *sessionMutationRecordingAIService) AbortSession(ctx context.Context, sessionID string) error {
	s.record("abort", sessionID)
	return nil
}

func (s *sessionMutationRecordingAIService) SummarizeSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	s.record("summarize", sessionID)
	return map[string]interface{}{"success": true}, nil
}

func (s *sessionMutationRecordingAIService) ForkSession(ctx context.Context, sessionID string) (*chat.Session, error) {
	s.record("fork", sessionID)
	return &chat.Session{ID: sessionID + "-fork"}, nil
}

func (s *sessionMutationRecordingAIService) UndoLastTurn(ctx context.Context, sessionID string, opts chat.SessionTurnUndoOptions) (*chat.SessionTurnUndoResult, error) {
	s.record("undo", sessionID)
	return &chat.SessionTurnUndoResult{Success: true, SessionID: sessionID}, nil
}

func (s *sessionMutationRecordingAIService) RedoLastTurn(ctx context.Context, sessionID string) (*chat.SessionTurnRedoResult, error) {
	s.record("redo", sessionID)
	return &chat.SessionTurnRedoResult{Success: true, SessionID: sessionID}, nil
}

func (s *sessionMutationRecordingAIService) SteerSession(ctx context.Context, sessionID string, req chat.SessionSteerRequest) (*chat.SessionSteerResult, error) {
	s.record("steer", sessionID)
	return &chat.SessionSteerResult{Accepted: true, SessionID: sessionID}, nil
}

var assistantSessionMutationSubresources = []string{"abort", "summarize", "fork", "undo", "redo", "steer"}

func TestAssistantSessionMutationsRequirePost(t *testing.T) {
	for _, sub := range assistantSessionMutationSubresources {
		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete} {
			t.Run(sub+"/"+method, func(t *testing.T) {
				svc := newSessionMutationRecordingAIService()
				handler := &AIHandler{}
				setUnexportedField(t, handler, "defaultService", svc)
				router := &Router{aiHandler: handler}

				req := httptest.NewRequest(method, "/api/ai/sessions/session-1/"+sub, nil)
				rec := httptest.NewRecorder()
				router.routeAISessions(rec, req)

				if rec.Code != http.StatusMethodNotAllowed {
					t.Fatalf("%s %s status = %d, want %d", method, sub, rec.Code, http.StatusMethodNotAllowed)
				}
				if allow := rec.Header().Get("Allow"); allow != http.MethodPost {
					t.Fatalf("%s %s Allow = %q, want %q", method, sub, allow, http.MethodPost)
				}
				if got := svc.recorded(); len(got) != 0 {
					t.Fatalf("%s %s reached the service: %v", method, sub, got)
				}
				if checks := svc.runningChecks(); checks != 0 {
					t.Fatalf("%s %s resolved the service (%d running checks) before rejecting the method", method, sub, checks)
				}
			})
		}

		t.Run(sub+"/POST", func(t *testing.T) {
			svc := newSessionMutationRecordingAIService()
			handler := &AIHandler{}
			setUnexportedField(t, handler, "defaultService", svc)
			router := &Router{aiHandler: handler}

			body := ""
			if sub == "steer" {
				body = `{"prompt":"check the backup job too"}`
			}
			req := httptest.NewRequest(http.MethodPost, "/api/ai/sessions/session-1/"+sub, strings.NewReader(body))
			rec := httptest.NewRecorder()
			router.routeAISessions(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("POST %s status = %d, want %d: %s", sub, rec.Code, http.StatusOK, rec.Body.String())
			}
			if got := svc.recorded(); !reflect.DeepEqual(got, []string{sub + ":session-1"}) {
				t.Fatalf("POST %s mutations = %v, want [%s:session-1]", sub, got, sub)
			}
		})
	}
}

func TestAssistantSessionReadSubresourcesKeepTheirMethods(t *testing.T) {
	svc := newSessionMutationRecordingAIService()
	handler := &AIHandler{}
	setUnexportedField(t, handler, "defaultService", svc)
	router := &Router{aiHandler: handler}

	messagesReq := httptest.NewRequest(http.MethodGet, "/api/ai/sessions/session-1/messages", nil)
	messagesRec := httptest.NewRecorder()
	router.routeAISessions(messagesRec, messagesReq)
	if messagesRec.Code != http.StatusOK {
		t.Fatalf("GET messages status = %d, want %d: %s", messagesRec.Code, http.StatusOK, messagesRec.Body.String())
	}

	diffReq := httptest.NewRequest(http.MethodGet, "/api/ai/sessions/session-1/diff", nil)
	diffRec := httptest.NewRecorder()
	router.routeAISessions(diffRec, diffReq)
	if diffRec.Code != http.StatusNotImplemented {
		t.Fatalf("GET diff status = %d, want %d: %s", diffRec.Code, http.StatusNotImplemented, diffRec.Body.String())
	}

	if got := svc.recorded(); len(got) != 0 {
		t.Fatalf("read sub-resources mutated the session: %v", got)
	}
}

// TestDemoModeRouterRejectsAssistantSessionMutationsOverSafeMethods drives the
// production handler chain: the demo-mode guard admits GET and HEAD as reads,
// so the session mutation itself must refuse them.
func TestDemoModeRouterRejectsAssistantSessionMutationsOverSafeMethods(t *testing.T) {
	tempDir := t.TempDir()
	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		AuthUser:   "admin",
		AuthPass:   hashed,
		DemoMode:   true,
	}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	t.Cleanup(router.shutdownBackgroundWorkers)
	svc := newSessionMutationRecordingAIService()
	setUnexportedField(t, router.aiHandler, "defaultService", svc)

	serve := func(method, path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		req.SetBasicAuth("admin", "Password!1")
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}

	for _, sub := range assistantSessionMutationSubresources {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			rec := serve(method, "/api/ai/sessions/session-1/"+sub)
			if rec.Header().Get("X-Demo-Mode") != "true" {
				t.Fatalf("%s %s did not pass through the demo-mode guard", method, sub)
			}
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("demo %s %s status = %d, want %d: %s", method, sub, rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
			}
		}
		if rec := serve(http.MethodPost, "/api/ai/sessions/session-1/"+sub); rec.Code != http.StatusForbidden {
			t.Fatalf("demo POST %s status = %d, want %d: %s", sub, rec.Code, http.StatusForbidden, rec.Body.String())
		}
	}
	if got := svc.recorded(); len(got) != 0 {
		t.Fatalf("demo-mode requests mutated Assistant sessions: %v", got)
	}

	if rec := serve(http.MethodGet, "/api/ai/sessions/session-1/messages"); rec.Code != http.StatusOK {
		t.Fatalf("demo GET messages status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}
