package monitoring

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	if err := persistent.WaitForMaintenance(0); err != nil {
		t.Fatal(err)
	}
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
	offsets := []time.Duration{0, 10 * time.Second, 20 * time.Second}
	// The live monitor deliberately uses bounded writes. A returned poll is not
	// a commit acknowledgement; establish visibility before testing reopen.
	requirePBSHistoryObservations(t, persistent, targetID, start, offsets, want)
	// Query again after closing and reopening: buffered reads alone are not
	// proof that the acknowledged observations survived persistence.
	if err := persistent.Close(); err != nil {
		t.Fatal(err)
	}
	persistent, err = metrics.NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	requirePBSHistoryObservations(t, persistent, targetID, start, offsets, want)
}

// A fresh process has no in-memory deduplication state. Replaying the cached
// observation while PBS is offline must not manufacture a new durable sample;
// an equal-valued observation after recovery must still be retained. Writers
// explicitly acknowledge their observations before exit: this tests observation
// identity across processes, not shutdown with unacknowledged live writes.
func TestPBSObservationHistoryAcrossMonitorReplacement(t *testing.T) {
	dir := os.Getenv("PULSE_PBS_HISTORY_TEST_DIR")
	child := dir != ""
	if !child {
		dir = t.TempDir()
	}
	cfg := metrics.DefaultConfig(dir)
	start := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	if child {
		var err error
		start, err = time.Parse(time.RFC3339, os.Getenv("PULSE_PBS_HISTORY_TEST_START"))
		if err != nil {
			t.Fatal(err)
		}
	}
	var targetID string
	for generation := 0; generation < 2; generation++ {
		if !child {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPBSObservationHistoryAcrossMonitorReplacement$", "-test.count=1", "-test.v")
			cmd.Env = append(os.Environ(),
				"PULSE_PBS_HISTORY_TEST_DIR="+dir,
				"PULSE_PBS_HISTORY_TEST_START="+start.Format(time.RFC3339),
				"PULSE_PBS_HISTORY_TEST_GENERATION="+strconv.Itoa(generation))
			output, err := cmd.CombinedOutput()
			cancel()
			// Keep successful generations too. A final reader failure must not
			// hide writer backlog, close diagnostics or the missing timestamp.
			t.Logf("generation %d writer output:\n%s", generation, output)
			if err != nil {
				t.Fatalf("generation %d: %v\n%s", generation, err, output)
			}
			id, err := os.ReadFile(filepath.Join(dir, "target-id"))
			if err != nil {
				t.Fatal(err)
			}
			if targetID != "" && targetID != string(id) {
				t.Fatal("target changed across process restart")
			}
			targetID = string(id)
			continue
		}
		if strconv.Itoa(generation) != os.Getenv("PULSE_PBS_HISTORY_TEST_GENERATION") {
			continue
		}
		persistent, err := metrics.NewStore(cfg)
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer func() {
				if err := persistent.Close(); err != nil {
					t.Error(err)
				} else {
					t.Logf("generation %d store closed", generation)
				}
			}()
			if err := persistent.WaitForMaintenance(0); err != nil {
				t.Fatal(err)
			}
			monitor := &Monitor{metricsHistory: NewMetricsHistory(1024, 24*time.Hour), metricsStore: persistent}
			adapter := unifiedresources.NewMonitorAdapter(nil)
			snapshot := models.StateSnapshot{PBSInstances: []models.PBSInstance{{
				ID: "pbs-restart", Name: "pbs-restart", Status: "online", LastSeen: start,
				Datastores: []models.PBSDatastore{{Name: "backups", Status: "available", Total: 1000, Used: 400, Free: 600, Usage: 40}},
			}}}
			if generation == 1 {
				snapshot.PBSInstances[0].Status = "offline"
			}
			for poll := 0; poll < 6; poll++ {
				adapter.PopulateFromSnapshot(snapshot)
				for _, resource := range adapter.GetAll() {
					if resource.Storage == nil || resource.Storage.Platform != "pbs" {
						continue
					}
					target := adapter.MetricsTargetForResource(resource.ID)
					if target == nil {
						t.Fatal("missing metrics target")
					}
					if targetID != "" && targetID != target.ResourceID {
						t.Fatal("target changed across replacement")
					}
					targetID = target.ResourceID
				}
				if targetID == "" {
					t.Fatal("missing PBS datastore")
				}
				monitor.syncUnifiedStorageMetrics(adapter)
			}
			want := map[string][]float64{"usage": {40}, "used": {400}, "total": {1000}, "avail": {600}}
			requirePBSHistoryObservations(t, persistent, targetID, start, []time.Duration{0}, want)
			t.Logf("generation %d: six cached rebuilds acknowledged at %s", generation, start.Format(time.RFC3339))
			if generation == 1 {
				snapshot.PBSInstances[0].Status = "online"
				snapshot.PBSInstances[0].LastSeen = start.Add(30 * time.Second)
				adapter.PopulateFromSnapshot(snapshot)
				monitor.syncUnifiedStorageMetrics(adapter)
				for metric, values := range want {
					want[metric] = append(values, values[0])
				}
				requirePBSHistoryObservations(t, persistent, targetID, start, []time.Duration{0, 30 * time.Second}, want)
				t.Logf("generation %d: equal-valued recovery acknowledged at %s", generation, snapshot.PBSInstances[0].LastSeen.Format(time.RFC3339))
			}
		}()
	}
	if child {
		if err := os.WriteFile(filepath.Join(dir, "target-id"), []byte(targetID), 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	// Both writer processes exited before these queries: no shared Go memory or
	// buffered writes can satisfy the assertions.
	persistent, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer persistent.Close()
	want := map[string][]float64{"usage": {40, 40}, "used": {400, 400}, "total": {1000, 1000}, "avail": {600, 600}}
	requirePBSHistoryObservations(t, persistent, targetID, start, []time.Duration{0, 30 * time.Second}, want)
}

// Flush is the public ingestion acknowledgement barrier, unlike a bounded live
// write or Close. Query every series before failing so a missing observation is
// localised by its actual timestamp/values, not by random map iteration order.
func requirePBSHistoryObservations(t *testing.T, persistent *metrics.Store, targetID string, start time.Time, offsets []time.Duration, want map[string][]float64) {
	t.Helper()
	persistent.Flush()
	for _, metric := range []string{"usage", "used", "total", "avail"} {
		points, err := persistent.Query("storage", targetID, metric, start.Add(-time.Second), start.Add(time.Minute), 0)
		if err != nil {
			t.Errorf("%s query: %v", metric, err)
			continue
		}
		expected := want[metric]
		if len(points) != len(expected) {
			t.Errorf("%s: got %d durable points %+v, want %v at offsets %v from %s", metric, len(points), points, expected, offsets, start.Format(time.RFC3339))
			continue
		}
		for i, point := range points {
			expectedTime := start.Add(offsets[i])
			if point.Value != expected[i] || !point.Timestamp.Equal(expectedTime) {
				t.Errorf("%s[%d] = %+v, want %v at %s", metric, i, point, expected[i], expectedTime)
			}
		}
	}
	if t.Failed() {
		t.FailNow()
	}
}
