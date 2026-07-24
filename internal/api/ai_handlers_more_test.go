package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
