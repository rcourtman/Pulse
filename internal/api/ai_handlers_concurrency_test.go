package api

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type staticAIPersistence struct {
	cfg     *config.AIConfig
	dataDir string
}

func (s staticAIPersistence) LoadAIConfig() (*config.AIConfig, error) { return s.cfg, nil }
func (s staticAIPersistence) DataDir() string                         { return s.dataDir }

type concurrencyUnifiedProvider struct{}

func (concurrencyUnifiedProvider) GetAll() []unifiedresources.Resource { return nil }
func (concurrencyUnifiedProvider) GetInfrastructure() []unifiedresources.Resource {
	return nil
}
func (concurrencyUnifiedProvider) GetWorkloads() []unifiedresources.Resource { return nil }
func (concurrencyUnifiedProvider) GetByType(unifiedresources.ResourceType) []unifiedresources.Resource {
	return nil
}
func (concurrencyUnifiedProvider) GetStats() unifiedresources.ResourceStats {
	return unifiedresources.ResourceStats{}
}
func (concurrencyUnifiedProvider) GetTopByCPU(int, []unifiedresources.ResourceType) []unifiedresources.Resource {
	return nil
}
func (concurrencyUnifiedProvider) GetTopByMemory(int, []unifiedresources.ResourceType) []unifiedresources.Resource {
	return nil
}
func (concurrencyUnifiedProvider) GetTopByDisk(int, []unifiedresources.ResourceType) []unifiedresources.Resource {
	return nil
}
func (concurrencyUnifiedProvider) GetRelated(string) map[string][]unifiedresources.Resource {
	return map[string][]unifiedresources.Resource{}
}
func (concurrencyUnifiedProvider) FindContainerHost(string) string { return "" }

type concurrencyStateProvider struct{}

func (concurrencyStateProvider) GetState() models.StateSnapshot { return models.StateSnapshot{} }

type concurrencyMetadataProvider struct{}

func (concurrencyMetadataProvider) SetGuestURL(string, string) error  { return nil }
func (concurrencyMetadataProvider) SetDockerURL(string, string) error { return nil }
func (concurrencyMetadataProvider) SetHostURL(string, string) error   { return nil }

type concurrencyThresholdProvider struct{}

func (concurrencyThresholdProvider) GetNodeCPUThreshold() float64    { return 80 }
func (concurrencyThresholdProvider) GetNodeMemoryThreshold() float64 { return 85 }
func (concurrencyThresholdProvider) GetGuestMemoryThreshold() float64 {
	return 90
}
func (concurrencyThresholdProvider) GetGuestDiskThreshold() float64 { return 95 }
func (concurrencyThresholdProvider) GetStorageThreshold() float64   { return 92 }

type concurrencyMetricsHistoryProvider struct{}

func (concurrencyMetricsHistoryProvider) GetNodeMetrics(string, string, time.Duration) []ai.MetricPoint {
	return nil
}
func (concurrencyMetricsHistoryProvider) GetGuestMetrics(string, string, time.Duration) []ai.MetricPoint {
	return nil
}
func (concurrencyMetricsHistoryProvider) GetAllGuestMetrics(string, time.Duration) map[string][]ai.MetricPoint {
	return nil
}
func (concurrencyMetricsHistoryProvider) GetAllStorageMetrics(string, time.Duration) map[string][]ai.MetricPoint {
	return nil
}

func newTestMultiTenantRuntime(t *testing.T) (*config.MultiTenantPersistence, *monitoring.MultiTenantMonitor) {
	t.Helper()

	cfgDir := t.TempDir()
	cfg := &config.Config{DataPath: cfgDir, ConfigPath: cfgDir}

	mtp := config.NewMultiTenantPersistence(t.TempDir())
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("create default persistence: %v", err)
	}

	mtm := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(mtm.Stop)

	return mtp, mtm
}

func TestAIHandler_SetTenantPointersConcurrentAccess(t *testing.T) {
	mtp, mtm := newTestMultiTenantRuntime(t)
	h := NewAIHandler(nil, nil, nil)
	legacyDir := t.TempDir()
	h.legacyConfig = &config.Config{DataPath: legacyDir, ConfigPath: legacyDir}
	h.legacyPersistence = staticAIPersistence{
		cfg:     &config.AIConfig{},
		dataDir: t.TempDir(),
	}

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	readState := unifiedresources.NewRegistry(nil)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				if (worker+j)%2 == 0 {
					h.SetMultiTenantPersistence(mtp)
					h.SetMultiTenantMonitor(mtm)
					h.SetReadState(readState)
				} else {
					h.SetMultiTenantPersistence(nil)
					h.SetMultiTenantMonitor(nil)
					h.SetReadState(nil)
				}

				_ = h.getConfig(ctx)
				_ = h.getPersistence(ctx)
				_ = h.readStateForOrg("default")
			}
		}(i)
	}

	wg.Wait()
}

func TestAISettingsHandler_StateSettersConcurrentAccess(t *testing.T) {
	mtp, mtm := newTestMultiTenantRuntime(t)
	h := NewAISettingsHandler(mtp, mtm, nil)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	readState := unifiedresources.NewRegistry(nil)
	provider := ai.UnifiedResourceProvider(concurrencyUnifiedProvider{})
	cfgDirA := t.TempDir()
	cfgA := &config.Config{DataPath: cfgDirA, ConfigPath: cfgDirA}
	cfgDirB := t.TempDir()
	cfgB := &config.Config{DataPath: cfgDirB, ConfigPath: cfgDirB}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				if (worker+j)%2 == 0 {
					h.SetMultiTenantPersistence(mtp)
					h.SetMultiTenantMonitor(mtm)
					h.SetReadState(readState)
					h.SetUnifiedResourceProvider(provider)
					h.SetConfig(cfgA)
				} else {
					h.SetMultiTenantPersistence(nil)
					h.SetMultiTenantMonitor(nil)
					h.SetReadState(nil)
					h.SetUnifiedResourceProvider(nil)
					h.SetConfig(cfgB)
				}

				_ = h.getConfig(ctx)
				_ = h.getPersistence(ctx)
				_ = h.readStateForOrg("default")
				_ = h.unifiedResourceProviderForOrg("default")
				_ = h.GetAIService(ctx)
			}
		}(i)
	}

	wg.Wait()
}

func TestAISettingsHandler_ProviderSettersConcurrentServiceCreation(t *testing.T) {
	mtp, mtm := newTestMultiTenantRuntime(t)
	h := NewAISettingsHandler(mtp, mtm, nil)

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "acme")
	stateProvider := &concurrencyStateProvider{}
	metadataProvider := concurrencyMetadataProvider{}
	thresholdProvider := concurrencyThresholdProvider{}
	metricsProvider := concurrencyMetricsHistoryProvider{}

	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 80; j++ {
				switch (worker + j) % 4 {
				case 0:
					h.SetStateProvider(stateProvider)
				case 1:
					h.SetMetadataProvider(metadataProvider)
				case 2:
					h.SetPatrolThresholdProvider(thresholdProvider)
				default:
					h.SetMetricsHistoryProvider(metricsProvider)
				}
			}
		}(i)
	}

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 80; j++ {
				h.RemoveTenantService("acme")
				_ = h.GetAIService(ctx)
			}
		}()
	}

	wg.Wait()

	if svc := h.GetAIService(ctx); svc == nil {
		t.Fatalf("expected tenant AI service after concurrent provider updates")
	}
}
