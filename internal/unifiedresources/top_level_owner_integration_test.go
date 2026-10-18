package unifiedresources_test

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func connectedOwnerIndexResources(t testing.TB) []unified.Resource {
	t.Helper()
	previousEnabled, previousConfig := mock.IsMockEnabled(), mock.GetConfig()
	t.Cleanup(func() {
		_ = mock.SetEnabled(false)
		mock.SetMockConfig(previousConfig)
		if previousEnabled {
			if err := mock.SetEnabled(true); err != nil {
				t.Error(err)
			}
		}
	})
	if err := mock.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	cfg := mock.DefaultConfig
	cfg.UpdateInterval = 5 * time.Minute
	mock.SetMockConfig(cfg)
	if err := mock.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	graph := mock.CurrentFixtureGraph()
	registry := unified.NewRegistry(nil)
	registry.IngestSnapshot(graph.State)
	for _, source := range []unified.DataSource{unified.SourceTrueNAS, unified.SourceVMware, unified.SourceAvailability} {
		registry.IngestRecords(source, graph.SupplementalRecords(source))
	}
	resources := registry.List()
	if len(resources) < 1000 {
		t.Fatalf("connected fixture lost estate: %d resources", len(resources))
	}
	providers := map[unified.DataSource]bool{}
	for _, resource := range resources {
		for _, source := range resource.Sources {
			providers[source] = true
		}
	}
	if len(providers) < 9 {
		t.Fatalf("connected fixture lost providers: %v", providers)
	}
	return resources
}

func TestTopLevelOwnerIndexConnectedDemo(t *testing.T) {
	resources := connectedOwnerIndexResources(t)
	unified.CheckTopLevelOwnerIndexResourcesForTest(t, resources)
}

func BenchmarkTopLevelOwnerIndexConnectedDemo(b *testing.B) {
	resources := connectedOwnerIndexResources(b)
	unified.RunTopLevelOwnerIndexBenchmarkForTest(b, resources)
}
