package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/resources"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
)

type stubMetadataProvider struct{}

func (stubMetadataProvider) SetGuestURL(string, string) error  { return nil }
func (stubMetadataProvider) SetDockerURL(string, string) error { return nil }
func (stubMetadataProvider) SetHostURL(string, string) error   { return nil }

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

type stubResourceProvider struct{}

func (stubResourceProvider) GetAll() []resources.Resource            { return nil }
func (stubResourceProvider) GetInfrastructure() []resources.Resource { return nil }
func (stubResourceProvider) GetWorkloads() []resources.Resource      { return nil }
func (stubResourceProvider) GetByType(resources.ResourceType) []resources.Resource {
	return nil
}
func (stubResourceProvider) GetStats() resources.StoreStats {
	return resources.StoreStats{}
}
func (stubResourceProvider) GetTopByCPU(int, []resources.ResourceType) []resources.Resource {
	return nil
}
func (stubResourceProvider) GetTopByMemory(int, []resources.ResourceType) []resources.Resource {
	return nil
}
func (stubResourceProvider) GetTopByDisk(int, []resources.ResourceType) []resources.Resource {
	return nil
}
func (stubResourceProvider) GetRelated(string) map[string][]resources.Resource {
	return map[string][]resources.Resource{}
}
func (stubResourceProvider) GetResourceSummary() resources.ResourceSummary {
	return resources.ResourceSummary{}
}
func (stubResourceProvider) FindContainerHost(string) string { return "" }

func TestAISettingsHandler_setSSECORSHeaders(t *testing.T) {
	handler := newTestAISettingsHandler(&config.Config{AllowedOrigins: "*"}, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/stream", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.setSSECORSHeaders(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("expected allow origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header, got %q", got)
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatalf("expected allow methods header")
	}
	if rec.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Fatalf("expected allow headers header")
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary=Origin, got %q", got)
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
	handler.SetResourceProvider(stubResourceProvider{})
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

func TestAutonomyLevelProviderAdapter(t *testing.T) {
	adapter := &autonomyLevelProviderAdapter{}
	if got := adapter.GetCurrentAutonomyLevel(); got != config.PatrolAutonomyMonitor {
		t.Fatalf("expected default autonomy %q, got %q", config.PatrolAutonomyMonitor, got)
	}
	if adapter.IsFullModeUnlocked() {
		t.Fatalf("expected full mode locked by default")
	}

	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.PatrolAutonomyLevel = config.PatrolAutonomyAssisted
	aiCfg.PatrolFullModeUnlocked = true
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	svc := ai.NewService(persistence, nil)
	if err := svc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	adapter = &autonomyLevelProviderAdapter{svc: svc}

	if got := adapter.GetCurrentAutonomyLevel(); got != config.PatrolAutonomyAssisted {
		t.Fatalf("expected autonomy %q, got %q", config.PatrolAutonomyAssisted, got)
	}
	if !adapter.IsFullModeUnlocked() {
		t.Fatalf("expected full mode unlocked")
	}
}

func TestLicenseCheckerForOrchestrator(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	svc := ai.NewService(persistence, nil)
	if err := svc.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	checker := &licenseCheckerForOrchestrator{svc: svc}
	if !checker.HasFeature("pro") {
		t.Fatalf("expected default license to allow feature")
	}

	svc.SetLicenseChecker(stubLicenseChecker{allow: false})
	if checker.HasFeature("pro") {
		t.Fatalf("expected license checker to deny feature")
	}
}
