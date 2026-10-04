package mock

import (
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestSampleMetricSeriesMatchesCanonicalPointSampler(t *testing.T) {
	start := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	timestamps := make([]time.Time, 0, 96)
	for i := 0; i < 96; i++ {
		timestamps = append(timestamps, start.Add(time.Duration(i)*15*time.Minute))
	}

	for _, tc := range []struct {
		resourceClass string
		resourceID    string
		metric        string
	}{
		{resourceClass: "node", resourceID: "pve1", metric: "cpu"},
		{resourceClass: "vm", resourceID: "database-primary", metric: "memory"},
		{resourceClass: "storage", resourceID: "backup-pool", metric: "usage"},
		{resourceClass: "disk", resourceID: "NVME-SERIAL-1", metric: "smart_temp"},
		{resourceClass: "dockerContainer", resourceID: "api-service", metric: "netin"},
	} {
		t.Run(tc.resourceClass+"/"+tc.metric, func(t *testing.T) {
			got := SampleMetricSeries(tc.resourceClass, tc.resourceID, tc.metric, timestamps)
			if len(got) != len(timestamps) {
				t.Fatalf("series length = %d, want %d", len(got), len(timestamps))
			}
			for i, at := range timestamps {
				want := SampleMetric(tc.resourceClass, tc.resourceID, tc.metric, at)
				if math.Abs(got[i]-want) > 1e-12 {
					t.Fatalf("point %d at %v = %v, want %v", i, at, got[i], want)
				}
			}
		})
	}
}

func TestSampleMetricSeriesEmptyInput(t *testing.T) {
	if got := SampleMetricSeries("vm", "vm-1", "cpu", nil); got != nil {
		t.Fatalf("empty series = %#v, want nil", got)
	}
}

func TestMetricSamplerRemainsBoundToFixtureGraph(t *testing.T) {
	previousRegistry := currentMetricRoleRegistry()
	t.Cleanup(func() {
		setMetricRoleRegistry(previousRegistry)
	})

	graph := FixtureGraph{
		State: models.StateSnapshot{
			Containers: []models.Container{{
				ID:     "neutral-155",
				Name:   "backup-orchestrator",
				Status: "running",
			}},
		},
	}
	sampler := NewMetricSampler(graph)
	if got := sampler.role("container", "neutral-155"); got != metricRoleBackup {
		t.Fatalf("sampler role = %q, want %q", got, metricRoleBackup)
	}

	at := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	want := sampler.SampleMetric("container", "neutral-155", "memory", at)

	setMetricRoleRegistry(map[string]string{
		metricRoleRegistryKey("container", "neutral-155"): metricRoleDatabase,
	})
	if got := sampler.SampleMetric("container", "neutral-155", "memory", at); math.Abs(got-want) > 1e-12 {
		t.Fatalf("sampler changed after global registry update: got %v, want %v", got, want)
	}
	if global := SampleMetric("container", "neutral-155", "memory", at); math.Abs(global-want) < 1e-6 {
		t.Fatalf("global database sample unexpectedly matched graph-bound backup sample: %v", global)
	}

	timestamps := []time.Time{at.Add(-time.Minute), at, at.Add(time.Minute)}
	series := sampler.SampleMetricSeries("container", "neutral-155", "memory", timestamps)
	for i, timestamp := range timestamps {
		point := sampler.SampleMetric("container", "neutral-155", "memory", timestamp)
		if math.Abs(series[i]-point) > 1e-12 {
			t.Fatalf("series point %d = %v, want %v", i, series[i], point)
		}
	}
}

// Keep the graph and its version coherent, just like runtime publication. Do
// not restore an old version: existing readers may still hold its sampler.
func installMetricSamplerGraphForTest(tb testing.TB, graph FixtureGraph) {
	tb.Helper()
	dataMu.Lock()
	previousGraph, previousEnabled := mockGraph, enabled.Load()
	mockGraph = graph
	enabled.Store(true)
	fixtureDataVersion.Add(1)
	dataMu.Unlock()
	tb.Cleanup(func() {
		dataMu.Lock()
		mockGraph = previousGraph
		enabled.Store(previousEnabled)
		fixtureDataVersion.Add(1)
		dataMu.Unlock()
	})
}

func TestCurrentMetricSamplerTracksGraphVersionWithoutGlobalPersonaLeak(t *testing.T) {
	graph := FixtureGraph{State: models.StateSnapshot{Containers: []models.Container{{ID: "neutral-155", Name: "backup-orchestrator"}}}}
	installMetricSamplerGraphForTest(t, graph)
	at := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	timestamps := []time.Time{at.Add(-time.Minute), at, at.Add(time.Minute)}
	initial := CurrentMetricSampler()
	want := NewMetricSampler(CurrentFixtureGraph()).SampleMetricSeries("container", "neutral-155", "memory", timestamps)
	if !reflect.DeepEqual(initial.SampleMetricSeries("container", "neutral-155", "memory", timestamps), want) {
		t.Fatal("cached sampler differs from graph-bound reference")
	}
	previousRegistry := currentMetricRoleRegistry()
	t.Cleanup(func() { setMetricRoleRegistry(previousRegistry) })
	setMetricRoleRegistry(map[string]string{metricRoleRegistryKey("container", "neutral-155"): metricRoleDatabase})
	if !reflect.DeepEqual(CurrentMetricSampler().SampleMetricSeries("container", "neutral-155", "memory", timestamps), want) {
		t.Fatal("private/global persona update replaced canonical graph sampling")
	}
	if allocs := testing.AllocsPerRun(100, func() { _ = CurrentMetricSampler() }); allocs != 0 {
		t.Fatalf("warm sampler acquisition allocates %v times; graph must not be cloned per series", allocs)
	}
	dataMu.Lock()
	mockGraph.State.Containers[0].Name = "database-primary"
	fixtureDataVersion.Add(1)
	dataMu.Unlock()
	updated := CurrentMetricSampler().SampleMetricSeries("container", "neutral-155", "memory", timestamps)
	reference := NewMetricSampler(CurrentFixtureGraph()).SampleMetricSeries("container", "neutral-155", "memory", timestamps)
	if !reflect.DeepEqual(updated, reference) || reflect.DeepEqual(updated, want) {
		t.Fatal("new graph version did not refresh its immutable persona snapshot")
	}
	if !reflect.DeepEqual(initial.SampleMetricSeries("container", "neutral-155", "memory", timestamps), want) {
		t.Fatal("publishing a new sampler mutated an already-returned sampler")
	}
	enabled.Store(false)
	if got := CurrentMetricSampler(); got.roles != nil {
		t.Fatal("disabled mock mode retained canonical roles")
	}
}

func TestCurrentMetricSamplerConcurrentSeriesRemainSnapshotBound(t *testing.T) {
	graph := FixtureGraph{State: models.StateSnapshot{Containers: []models.Container{{ID: "neutral-155", Name: "backup-orchestrator"}}}}
	backup := NewMetricSampler(graph)
	graph.State.Containers[0].Name = "database-primary"
	database := NewMetricSampler(graph)
	installMetricSamplerGraphForTest(t, graph)
	at := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	times := []time.Time{at.Add(-time.Minute), at, at.Add(time.Minute)}
	first, second := backup.SampleMetricSeries("container", "neutral-155", "memory", times), database.SampleMetricSeries("container", "neutral-155", "memory", times)
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 50; j++ {
				got := CurrentMetricSampler().SampleMetricSeries("container", "neutral-155", "memory", times)
				if !reflect.DeepEqual(got, first) && !reflect.DeepEqual(got, second) {
					t.Error("one series mixed two graph revisions")
					return
				}
			}
		}()
	}
	for i := 0; i < 50; i++ {
		name := "backup-orchestrator"
		if i%2 == 0 {
			name = "database-primary"
		}
		dataMu.Lock()
		mockGraph.State.Containers[0].Name = name
		fixtureDataVersion.Add(1)
		dataMu.Unlock()
	}
	workers.Wait()
}

func BenchmarkCanonicalMetricSeriesSamplerReuse(b *testing.B) {
	graph := FixtureGraph{State: models.StateSnapshot{VMs: make([]models.VM, 1000)}}
	for i := range graph.State.VMs {
		graph.State.VMs[i] = models.VM{ID: fmt.Sprintf("neutral-%d", i), Name: "database-primary"}
	}
	installMetricSamplerGraphForTest(b, graph)
	at := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	times := []time.Time{at.Add(-time.Minute), at, at.Add(time.Minute)}
	_ = CurrentMetricSampler()
	b.Run("legacy-cloned-graph-per-series", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = NewMetricSampler(CurrentFixtureGraph()).SampleMetricSeries("vm", "neutral-1", "memory", times)
		}
	})
	b.Run("current-immutable-sampler", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = CurrentMetricSampler().SampleMetricSeries("vm", "neutral-1", "memory", times)
		}
	})
}
