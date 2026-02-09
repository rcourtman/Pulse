package truenas

import (
	"math"
	"strings"
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

func TestTrueNASResourcesFlowThroughUnifiedTypesWithoutSpecialCasing(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	registry := unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())
	registry.IngestRecords(unifiedresources.SourceTrueNAS, NewDefaultProvider().Records())

	adapter := unifiedresources.NewMonitorAdapter(registry)
	resources := adapter.GetAll()
	if len(resources) == 0 {
		t.Fatal("expected resources from registry")
	}

	for _, resource := range resources {
		if string(resource.Type) == "truenas" {
			t.Fatalf("expected canonical render type, got truenas-specific type for %s", resource.ID)
		}
		switch resource.Type {
		case unifiedresources.ResourceTypeHost, unifiedresources.ResourceTypeStorage:
		default:
			t.Fatalf("unexpected unified type for truenas fixture resource: %s (%s)", resource.Type, resource.ID)
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

func toFrontendInput(resource unifiedresources.Resource) models.ResourceConvertInput {
	input := models.ResourceConvertInput{
		ID:           resource.ID,
		Type:         string(resource.Type),
		Name:         resource.Name,
		DisplayName:  resource.Name,
		SourceType:   firstSourceType(resource.Sources),
		ParentID:     stringValue(resource.ParentID),
		ClusterID:    resource.Identity.ClusterName,
		Status:       string(resource.Status),
		Tags:         resource.Tags,
		LastSeenUnix: resource.LastSeen.UnixMilli(),
	}
	input.CPU = metricToInput(metricValue(resource.Metrics, func(metrics *unifiedresources.ResourceMetrics) *unifiedresources.MetricValue { return metrics.CPU }))
	input.Memory = metricToInput(metricValue(resource.Metrics, func(metrics *unifiedresources.ResourceMetrics) *unifiedresources.MetricValue { return metrics.Memory }))
	input.Disk = metricToInput(metricValue(resource.Metrics, func(metrics *unifiedresources.ResourceMetrics) *unifiedresources.MetricValue { return metrics.Disk }))

	if resource.Metrics != nil && (resource.Metrics.NetIn != nil || resource.Metrics.NetOut != nil) {
		input.HasNetwork = true
		if resource.Metrics.NetIn != nil {
			input.NetworkRX = int64(math.Round(resource.Metrics.NetIn.Value))
		}
		if resource.Metrics.NetOut != nil {
			input.NetworkTX = int64(math.Round(resource.Metrics.NetOut.Value))
		}
	}
	input.Identity = identityToInput(resource.Identity)
	return input
}

func firstSourceType(sources []unifiedresources.DataSource) string {
	if len(sources) == 0 {
		return ""
	}
	return string(sources[0])
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func metricValue(
	metrics *unifiedresources.ResourceMetrics,
	pick func(*unifiedresources.ResourceMetrics) *unifiedresources.MetricValue,
) *unifiedresources.MetricValue {
	if metrics == nil {
		return nil
	}
	return pick(metrics)
}

func metricToInput(metric *unifiedresources.MetricValue) *models.ResourceMetricInput {
	if metric == nil {
		return nil
	}

	current := metric.Percent
	if current == 0 {
		current = metric.Value
	}
	if metric.Percent != 0 && metric.Value != 0 {
		current = math.Max(metric.Percent, metric.Value)
	}

	result := &models.ResourceMetricInput{Current: current}
	if metric.Total != nil {
		total := *metric.Total
		result.Total = &total
	}
	if metric.Used != nil {
		used := *metric.Used
		result.Used = &used
	}
	if result.Total != nil && result.Used != nil {
		free := *result.Total - *result.Used
		result.Free = &free
	}
	return result
}

func identityToInput(identity unifiedresources.ResourceIdentity) *models.ResourceIdentityInput {
	hostname := ""
	for _, candidate := range identity.Hostnames {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			hostname = trimmed
			break
		}
	}

	ips := make([]string, 0, len(identity.IPAddresses))
	for _, ip := range identity.IPAddresses {
		if trimmed := strings.TrimSpace(ip); trimmed != "" {
			ips = append(ips, trimmed)
		}
	}

	machineID := strings.TrimSpace(identity.MachineID)
	if hostname == "" && machineID == "" && len(ips) == 0 {
		return nil
	}

	return &models.ResourceIdentityInput{
		Hostname:  hostname,
		MachineID: machineID,
		IPs:       ips,
	}
}
