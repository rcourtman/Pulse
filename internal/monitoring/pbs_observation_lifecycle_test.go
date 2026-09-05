package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

// Issue #1882: unrelated poll completions must not multiply PBS history, but
// deduplication must retain a later observation even when its values are equal.
func TestPBSObservationHistorySurvivesRebuildsAndStoreReopen(t *testing.T) {
	cfg := metrics.DefaultConfig(t.TempDir())
	persistent, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if persistent != nil {
			_ = persistent.Close()
		}
	})
	monitor := &Monitor{metricsHistory: NewMetricsHistory(1024, 24*time.Hour), metricsStore: persistent}
	adapter := unifiedresources.NewMonitorAdapter(nil)
	start := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	var targetID string
	values := []float64{40, 40, 60}
	for observation, usage := range values {
		snapshot := models.StateSnapshot{PBSInstances: []models.PBSInstance{{
			ID: "pbs-history", Name: "pbs-history", Status: "online",
			LastSeen:   start.Add(time.Duration(observation) * 10 * time.Second),
			Datastores: []models.PBSDatastore{{Name: "backups", Status: "available", Total: 1000, Used: int64(usage * 10), Free: int64(1000 - usage*10), Usage: usage}},
		}}}
		for rebuild := 0; rebuild < 6; rebuild++ {
			adapter.PopulateFromSnapshot(snapshot)
			for _, resource := range adapter.GetAll() {
				if resource.Type != unifiedresources.ResourceTypeStorage || resource.Storage == nil || resource.Storage.Platform != "pbs" {
					continue
				}
				target := adapter.MetricsTargetForResource(resource.ID)
				if target == nil || target.ResourceType != "storage" {
					t.Fatalf("missing storage target: %+v", target)
				}
				if targetID != "" && targetID != target.ResourceID {
					t.Fatalf("target changed: %s -> %s", targetID, target.ResourceID)
				}
				targetID = target.ResourceID
			}
			if targetID == "" {
				t.Fatal("missing PBS datastore")
			}
			monitor.syncUnifiedStorageMetrics(adapter)
		}
	}
	want := map[string][]float64{
		"usage": {40, 40, 60}, "used": {400, 400, 600},
		"total": {1000, 1000, 1000}, "avail": {600, 600, 400},
	}
	memory := monitor.GetStorageMetrics(targetID, time.Hour)
	for metric, expected := range want {
		points := memory[metric]
		if len(points) != len(expected) {
			t.Fatalf("memory %s: got %d points, want %d", metric, len(points), len(expected))
		}
		for i, point := range points {
			timestamp := start.Add(time.Duration(i) * 10 * time.Second)
			if point.Value != expected[i] || !point.Timestamp.Equal(timestamp) {
				t.Fatalf("memory %s[%d] = %+v, want %v at %s", metric, i, point, expected[i], timestamp)
			}
		}
	}
	// Query only after closing and reopening: buffered reads alone are not proof
	// that the deduplicated observations survived SQLite persistence.
	if err := persistent.Close(); err != nil {
		t.Fatal(err)
	}
	persistent, err = metrics.NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for metric, expected := range want {
		points, err := persistent.Query("storage", targetID, metric, start.Add(-time.Second), start.Add(time.Minute), 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(points) != len(expected) {
			t.Fatalf("reopened %s: got %d points, want %d", metric, len(points), len(expected))
		}
		for i, point := range points {
			timestamp := start.Add(time.Duration(i) * 10 * time.Second)
			if point.Value != expected[i] || !point.Timestamp.Equal(timestamp) {
				t.Fatalf("reopened %s[%d] = %+v, want %v at %s", metric, i, point, expected[i], timestamp)
			}
		}
	}
}
