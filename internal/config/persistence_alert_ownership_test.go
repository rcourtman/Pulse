package config_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog"
)

// SaveAlertConfig used to normalize the config it was handed in place.
// Callers hand it alerts.Manager.GetConfig() snapshots, whose maps and
// threshold pointers were the live manager config's own, so every save
// wrote into the running config outside the manager lock. Evaluation reads
// those maps under the lock and the GET handler encodes a snapshot outside
// it, so a save racing either one could abort Pulse with a concurrent map
// write. Run with -race: the rounds below take the same steps as alert
// activation, the identity migration and GET /api/alerts/config, while
// evaluation runs.
func TestSaveAlertConfigLeavesTheLiveManagerConfigAlone(t *testing.T) {
	originalLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	t.Cleanup(func() { zerolog.SetGlobalLevel(originalLevel) })

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)

	seed := manager.GetConfig()
	seed.Enabled = true
	seed.ActivationState = alerts.ActivationActive
	// Agent CPU alerts off: persistence rewrites the clear value of a zero
	// trigger, so a shared threshold pointer is written on every save.
	seed.AgentDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 0, Clear: 0}
	// No delay for guests: persistence used to replace an explicit zero with
	// five seconds, writing the live delay map.
	seed.TimeThresholds = map[string]int{"guest": 0}
	seed.Overrides = make(map[string]alerts.ThresholdConfig)
	for i := 0; i < 64; i++ {
		seed.Overrides[fmt.Sprintf("agent:host-%d", i)] = alerts.ThresholdConfig{
			CPU: &alerts.HysteresisThreshold{Trigger: 95, Clear: 90},
		}
	}
	manager.UpdateConfig(seed)
	want := manager.GetConfig()
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal seeded config: %v", err)
	}

	persistence := config.NewConfigPersistence(t.TempDir())
	if err := persistence.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	const rounds = 200
	var wg sync.WaitGroup
	run := func(step func(round int)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < rounds; round++ {
				step(round)
			}
		}()
	}
	// POST /api/alerts/activate: apply a snapshot, then persist it.
	run(func(int) {
		cfg := manager.GetConfig()
		manager.UpdateConfig(cfg)
		if err := persistence.SaveAlertConfig(cfg); err != nil {
			t.Errorf("save after update: %v", err)
		}
	})
	// The alert identity migration persists a snapshot.
	run(func(int) {
		if err := persistence.SaveAlertConfig(manager.GetConfig()); err != nil {
			t.Errorf("save snapshot: %v", err)
		}
	})
	// GET /api/alerts/config encodes a snapshot outside the manager lock.
	run(func(int) {
		if _, err := json.Marshal(manager.GetConfig()); err != nil {
			t.Errorf("encode snapshot: %v", err)
		}
	})
	run(func(round int) {
		id := fmt.Sprintf("host-%d", round%64)
		manager.CheckHost(models.Host{ID: id, Hostname: id, Status: "online", CPUUsage: 50})
	})
	wg.Wait()

	got := manager.GetConfig()
	if got.TimeThresholds["guest"] != 0 {
		t.Fatalf("live guest delay = %d after saves, want the configured 0", got.TimeThresholds["guest"])
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal final config: %v", err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("saving changed the live config:\n got %s\nwant %s", gotJSON, wantJSON)
	}

	loaded, err := persistence.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}
	if delay, ok := loaded.TimeThresholds["guest"]; !ok || delay != 0 {
		t.Fatalf("persisted guest delay = %d (present %v), want the configured 0", delay, ok)
	}
}

func TestSaveAlertConfigDoesNotNormalizeTheCallersConfig(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	if err := persistence.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	cfg := alerts.AlertConfig{
		AgentDefaults: alerts.ThresholdConfig{
			CPU: &alerts.HysteresisThreshold{Trigger: 80, Clear: 0},
		},
		TimeThresholds: map[string]int{"guest": -1, "node": 0},
		Overrides: map[string]alerts.ThresholdConfig{
			"storage:local": {Usage: &alerts.HysteresisThreshold{Trigger: 90, Clear: 0}},
		},
		BackupDefaults: alerts.BackupAlertConfig{IgnoreVMIDs: []string{" 100 ", "100"}},
	}
	before, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cpu, override := cfg.AgentDefaults.CPU, cfg.Overrides["storage:local"].Usage

	if err := persistence.SaveAlertConfig(cfg); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	after, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("SaveAlertConfig changed its argument:\n got %s\nwant %s", after, before)
	}
	if cfg.AgentDefaults.CPU != cpu || cfg.Overrides["storage:local"].Usage != override {
		t.Fatal("SaveAlertConfig replaced the caller's threshold pointers")
	}

	loaded, err := persistence.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}
	if got := loaded.TimeThresholds; got["guest"] != 5 || got["node"] != 0 {
		t.Fatalf("persisted delays = %v, want guest 5 (unset) and node 0 (no delay)", got)
	}
	if got := *loaded.AgentDefaults.CPU; got != (alerts.HysteresisThreshold{Trigger: 80, Clear: 75}) {
		t.Fatalf("persisted agent CPU = %+v, want clear derived below the trigger", got)
	}
	if got := *loaded.Overrides["storage:local"].Usage; got != (alerts.HysteresisThreshold{Trigger: 90, Clear: 85}) {
		t.Fatalf("persisted override usage = %+v, want clear derived below the trigger", got)
	}
	if want := []string{"100"}; !reflect.DeepEqual(loaded.BackupDefaults.IgnoreVMIDs, want) {
		t.Fatalf("persisted ignore list = %v, want %v", loaded.BackupDefaults.IgnoreVMIDs, want)
	}
}
