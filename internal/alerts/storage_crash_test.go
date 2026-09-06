package alerts

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Unlike an orderly restart, process exit must not get a shutdown checkpoint
// or store Close to rescue a lifecycle transition. Keep the recovery mirror
// deliberately stale to prove SQLite, not the mirror, preserves the incident.
// This tests process interruption, not power loss or installed delivery.
func TestStorageLifecycleAcrossProcessExit(t *testing.T) {
	const helperEnv = "PULSE_TEST_STORAGE_CRASH_PHASE"
	storage := models.Storage{ID: "pbs-crash-backups", Name: "backups", Type: "pbs", Status: "online", Total: 1000, Used: 850, Free: 150, Usage: 85}
	id := canonicalMetricStateID(storage.ID, "usage")
	start := func(t *testing.T, dir string) *Manager {
		t.Helper()
		m := NewManagerWithDataDir(dir)
		m.EnableEventLog()
		if !m.activeStateAuthoritative.Load() {
			t.Fatal("SQLite authority not enabled")
		}
		m.UpdateConfig(AlertConfig{Enabled: true, ActivationState: ActivationActive,
			TimeThresholds: map[string]int{"storage": 0}, StorageDefault: HysteresisThreshold{Trigger: 80, Clear: 70}})
		return m
	}
	observe := func(m *Manager, s models.Storage) {
		for range 5 {
			m.CheckStorage(s)
		}
	}
	if phase := os.Getenv(helperEnv); phase != "" {
		if phase != "fired" && phase != "resolved" {
			t.Fatal("invalid child phase")
		}
		m := start(t, os.Getenv("PULSE_TEST_STORAGE_CRASH_DIR"))
		if phase == "resolved" {
			observe(m, storage)
			testRequireActiveAlert(t, m, id)
		}
		if err := m.SaveActiveAlerts(); err != nil {
			t.Fatal(err)
		}
		// Freeze checkpoints, leaving either an empty or a firing JSON mirror.
		// Lifecycle commits must remain independent of periodic/async saves.
		m.saveMu.Lock()
		if phase == "resolved" {
			storage.Used, storage.Free, storage.Usage = 0, 1000, 0
		}
		observe(m, storage)
		if testHasActiveAlert(t, m, id) != (phase == "fired") {
			t.Fatal("child did not reach expected lifecycle state")
		}
		os.Exit(0) // Intentionally bypass all cleanups, Stop and store Close.
	}

	for _, phase := range []string{"fired", "resolved"} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStorageLifecycleAcrossProcessExit$")
			cmd.Env = append(os.Environ(), helperEnv+"="+phase, "PULSE_TEST_STORAGE_CRASH_DIR="+dir)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("child failed: %v\n%s", err, output)
			}
			mirror, err := os.ReadFile(filepath.Join(dir, "alerts", "active-alerts.json"))
			if err != nil {
				t.Fatal(err)
			}
			var stale []*Alert
			if err := json.Unmarshal(mirror, &stale); err != nil {
				t.Fatal(err)
			}
			wantStale := 0
			if phase == "resolved" {
				wantStale = 1
			}
			if len(stale) != wantStale {
				t.Fatalf("recovery mirror was not stale: %s", mirror)
			}
			m := start(t, dir)
			t.Cleanup(m.Stop)
			if testHasActiveAlert(t, m, id) != (phase == "fired") {
				t.Fatal("process exit lost firing incident or resurrected resolved incident")
			}
			events, err := m.AlertEvents(eventlog.Filter{Types: []string{eventlog.TypeFired, eventlog.TypeResolved}})
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if phase == "resolved" {
				want = 2
			}
			counts := make(map[string]int)
			for _, event := range events {
				counts[event.Type]++
			}
			if counts[eventlog.TypeFired] != 1 || counts[eventlog.TypeResolved] != want-1 {
				t.Fatalf("unexpected lifecycle history: %v", counts)
			}
			if len(events) != want {
				t.Fatalf("durable lifecycle events = %d, want %d", len(events), want)
			}
			missing := storage
			missing.Total, missing.Used, missing.Free, missing.Usage = 0, 0, 0, 0
			observe(m, missing)
			if testHasActiveAlert(t, m, id) != (phase == "fired") {
				t.Fatal("missing post-restart counters changed the incident state")
			}
		})
	}
}
