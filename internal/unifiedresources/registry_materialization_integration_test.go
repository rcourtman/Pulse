package unifiedresources_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Use the real multi-provider demo graph, including its metadata-heavy
// Kubernetes estate, rather than accepting a VM-only copying oracle.
func TestRegistryMaterializedMetadataConnectedDemo(t *testing.T) {
	previousEnabled := mock.IsMockEnabled()
	previousConfig := mock.GetConfig()
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
	cfg.UpdateInterval = 24 * time.Hour
	mock.SetMockConfig(cfg)
	if err := mock.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	graph := mock.CurrentFixtureGraph()
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(graph.State)
	for _, source := range []unifiedresources.DataSource{unifiedresources.SourceTrueNAS, unifiedresources.SourceVMware, unifiedresources.SourceAvailability} {
		registry.IngestRecords(source, graph.SupplementalRecords(source))
	}
	assertEqual := func(stage string) {
		t.Helper()
		actual := registry.List()
		expected := unifiedresources.FreshRegistryMetadataListForTest(registry)
		if len(actual) < 1000 {
			t.Fatalf("%s: fixture is not a large estate (%d resources)", stage, len(actual))
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("%s: complete demo resources differ from fresh clone", stage)
		}
		a, err := json.Marshal(actual)
		if err != nil {
			t.Fatal(err)
		}
		e, err := json.Marshal(expected)
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(e) {
			t.Fatalf("%s: demo wire content differs", stage)
		}
		sources := map[unifiedresources.DataSource]bool{}
		for _, r := range actual {
			for _, source := range r.Sources {
				sources[source] = true
			}
		}
		for _, source := range []unifiedresources.DataSource{unifiedresources.SourceProxmox, unifiedresources.SourceAgent, unifiedresources.SourceDocker, unifiedresources.SourcePBS, unifiedresources.SourcePMG, unifiedresources.SourceK8s, unifiedresources.SourceTrueNAS, unifiedresources.SourceVMware, unifiedresources.SourceAvailability} {
			if !sources[source] {
				t.Fatalf("%s: fixture omits provider %s", stage, source)
			}
		}
		t.Logf("%s: %d complete resources equal the independent fresh-clone oracle", stage, len(actual))
	}
	assertEqual("initial")
	registry.Workloads()
	registry.IngestSnapshot(graph.State)
	assertEqual("snapshot-refresh")
	registry.MarkStale(time.Now().UTC().Add(7*24*time.Hour), nil)
	assertEqual("stale-transition")
	// A normalized re-seed must keep identity, supersession, policy and every
	// native facet, rather than assuming old derived fields are trustworthy.
	resources := registry.List()
	resources[0].Name = "changed-name"
	resources[0].Tags = []string{"customer-data"}
	resources[0].PlatformScopes = []string{"old"}
	resources[0].AISafeSummary = "old"
	registry.IngestResources(resources)
	assertEqual("resource-reseed")
}
