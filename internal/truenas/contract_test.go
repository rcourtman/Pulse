package truenas

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestProviderFeatureFlagGatesFixtureRecords(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(false)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	provider := NewDefaultProvider()
	if got := len(provider.Records()); got != 0 {
		t.Fatalf("expected no records with feature disabled, got %d", got)
	}
}

func TestRegistryIngestRecordsTreatsTrueNASAsGenericDataSource(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	fixtures := DefaultFixtures()
	provider := NewProvider(fixtures)
	records := provider.Records()
	if len(records) == 0 {
		t.Fatal("expected fixture records from provider")
	}

	registry := unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())
	registry.IngestRecords(unifiedresources.SourceTrueNAS, records)

	resources := registry.List()
	wantCount := 1 + len(fixtures.Pools) + len(fixtures.Datasets)
	if len(resources) != wantCount {
		t.Fatalf("expected %d resources, got %d", wantCount, len(resources))
	}

	system := requireResource(t, resources, unifiedresources.ResourceTypeHost, fixtures.System.Hostname)
	assertSourceTracking(t, *system, unifiedresources.SourceTrueNAS)
	if system.ChildCount != len(fixtures.Pools) {
		t.Fatalf("expected system child count %d, got %d", len(fixtures.Pools), system.ChildCount)
	}

	pool := requireResource(t, resources, unifiedresources.ResourceTypeStorage, "tank")
	assertSourceTracking(t, *pool, unifiedresources.SourceTrueNAS)
	if pool.ParentID == nil || *pool.ParentID != system.ID {
		t.Fatalf("expected pool parent %q, got %+v", system.ID, pool.ParentID)
	}
	assertDiskMetric(t, pool.Metrics, 30*1024*1024*1024*1024, 12*1024*1024*1024*1024)

	dataset := requireResource(t, resources, unifiedresources.ResourceTypeStorage, "tank/apps")
	assertSourceTracking(t, *dataset, unifiedresources.SourceTrueNAS)
	if dataset.ParentID == nil || *dataset.ParentID != pool.ID {
		t.Fatalf("expected dataset parent %q, got %+v", pool.ID, dataset.ParentID)
	}
	assertDiskMetric(t, dataset.Metrics, 18*1024*1024*1024*1024, 5*1024*1024*1024*1024)

	targets := registry.SourceTargets(dataset.ID)
	if len(targets) == 0 {
		t.Fatal("expected source targets for dataset")
	}
	found := false
	for _, target := range targets {
		if target.Source == unifiedresources.SourceTrueNAS && target.SourceID == "dataset:tank/apps" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected truenas source target for dataset, got %+v", targets)
	}
}

func TestTrueNASResourcesFlowThroughLegacyRenderTypesWithoutSpecialCasing(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	registry := unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())
	registry.IngestRecords(unifiedresources.SourceTrueNAS, NewDefaultProvider().Records())

	adapter := unifiedresources.NewMonitorAdapter(registry)
	legacy := adapter.GetAll()
	if len(legacy) == 0 {
		t.Fatal("expected legacy resources from registry")
	}

	for _, resource := range legacy {
		if resource.Type == unifiedresources.LegacyResourceTypeTrueNAS {
			t.Fatalf("expected canonical render type, got truenas-specific type for %s", resource.ID)
		}
		switch resource.Type {
		case unifiedresources.LegacyResourceTypeHost, unifiedresources.LegacyResourceTypeStorage:
		default:
			t.Fatalf("unexpected legacy type for truenas fixture resource: %s (%s)", resource.Type, resource.ID)
		}

		frontend := models.ConvertResourceToFrontend(toFrontendInput(resource))
		if frontend.ID == "" {
			t.Fatalf("expected frontend conversion to preserve ID for %s", resource.ID)
		}
	}
}

func requireResource(t *testing.T, resources []unifiedresources.Resource, resourceType unifiedresources.ResourceType, name string) *unifiedresources.Resource {
	t.Helper()
	for i := range resources {
		if resources[i].Type == resourceType && resources[i].Name == name {
			return &resources[i]
		}
	}
	t.Fatalf("missing resource type=%s name=%s", resourceType, name)
	return nil
}

func assertSourceTracking(t *testing.T, resource unifiedresources.Resource, source unifiedresources.DataSource) {
	t.Helper()
	if !containsSource(resource.Sources, source) {
		t.Fatalf("expected source %s in %+v", source, resource.Sources)
	}
	if resource.SourceStatus == nil {
		t.Fatalf("expected source status for %s", resource.ID)
	}
	if _, ok := resource.SourceStatus[source]; !ok {
		t.Fatalf("expected source status entry for %s on %s", source, resource.ID)
	}
}

func assertDiskMetric(t *testing.T, metrics *unifiedresources.ResourceMetrics, total int64, used int64) {
	t.Helper()
	if metrics == nil || metrics.Disk == nil {
		t.Fatal("expected disk metric")
	}
	if metrics.Disk.Total == nil || *metrics.Disk.Total != total {
		t.Fatalf("expected total=%d, got %+v", total, metrics.Disk.Total)
	}
	if metrics.Disk.Used == nil || *metrics.Disk.Used != used {
		t.Fatalf("expected used=%d, got %+v", used, metrics.Disk.Used)
	}
	free := *metrics.Disk.Total - *metrics.Disk.Used
	if free <= 0 {
		t.Fatalf("expected positive free capacity, got %d", free)
	}
}

func containsSource(sources []unifiedresources.DataSource, source unifiedresources.DataSource) bool {
	for _, existing := range sources {
		if existing == source {
			return true
		}
	}
	return false
}

func toFrontendInput(resource unifiedresources.LegacyResource) models.ResourceConvertInput {
	input := models.ResourceConvertInput{
		ID:           resource.ID,
		Type:         string(resource.Type),
		Name:         resource.Name,
		DisplayName:  resource.DisplayName,
		PlatformID:   resource.PlatformID,
		PlatformType: string(resource.PlatformType),
		SourceType:   string(resource.SourceType),
		ParentID:     resource.ParentID,
		ClusterID:    resource.ClusterID,
		Status:       string(resource.Status),
		Temperature:  resource.Temperature,
		Uptime:       resource.Uptime,
		Tags:         resource.Tags,
		Labels:       resource.Labels,
		LastSeenUnix: resource.LastSeen.UnixMilli(),
		PlatformData: resource.PlatformData,
	}
	if resource.CPU != nil {
		input.CPU = &models.ResourceMetricInput{
			Current: resource.CPU.Current,
			Total:   resource.CPU.Total,
			Used:    resource.CPU.Used,
			Free:    resource.CPU.Free,
		}
	}
	if resource.Memory != nil {
		input.Memory = &models.ResourceMetricInput{
			Current: resource.Memory.Current,
			Total:   resource.Memory.Total,
			Used:    resource.Memory.Used,
			Free:    resource.Memory.Free,
		}
	}
	if resource.Disk != nil {
		input.Disk = &models.ResourceMetricInput{
			Current: resource.Disk.Current,
			Total:   resource.Disk.Total,
			Used:    resource.Disk.Used,
			Free:    resource.Disk.Free,
		}
	}
	if resource.Network != nil {
		input.HasNetwork = true
		input.NetworkRX = resource.Network.RXBytes
		input.NetworkTX = resource.Network.TXBytes
	}
	if resource.Identity != nil {
		input.Identity = &models.ResourceIdentityInput{
			Hostname:  resource.Identity.Hostname,
			MachineID: resource.Identity.MachineID,
			IPs:       resource.Identity.IPs,
		}
	}
	return input
}
