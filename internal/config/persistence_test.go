package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

func TestSaveAlertConfig_PreservesStorageOverrideHysteresis(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	cfg := alerts.AlertConfig{
		Enabled:          true,
		StorageDefault:   alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		HysteresisMargin: 5.0,
		Overrides: map[string]alerts.ThresholdConfig{
			"storage-123": {
				Usage: &alerts.HysteresisThreshold{Trigger: 90, Clear: 0},
			},
		},
	}

	if err := cp.SaveAlertConfig(cfg); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	loaded, err := cp.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}

	override, ok := loaded.Overrides["storage-123"]
	if !ok {
		t.Fatalf("storage override missing after load: %+v", loaded.Overrides)
	}
	if override.Usage == nil {
		t.Fatalf("usage threshold nil after load")
	}
	if got, want := override.Usage.Trigger, 90.0; got != want {
		t.Fatalf("trigger mismatch: got %v want %v", got, want)
	}
	if got, want := override.Usage.Clear, 85.0; got != want {
		t.Fatalf("clear threshold mismatch: got %v want %v", got, want)
	}
}

func TestSaveAlertConfig_DoesNotOverwriteExistingClear(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	cfg := alerts.AlertConfig{
		Enabled:          true,
		HysteresisMargin: 5.0,
		StorageDefault:   alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides: map[string]alerts.ThresholdConfig{
			"storage-456": {
				Usage: &alerts.HysteresisThreshold{Trigger: 92, Clear: 88},
			},
		},
	}

	if err := cp.SaveAlertConfig(cfg); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	loaded, err := cp.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}

	override := loaded.Overrides["storage-456"]
	if override.Usage == nil {
		t.Fatalf("usage threshold nil")
	}
	if got, want := override.Usage.Clear, 88.0; got != want {
		t.Fatalf("clear threshold changed unexpectedly: got %v want %v", got, want)
	}
}

func TestAlertConfigPersistenceNormalizesDockerIgnoredPrefixes(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	cfg := alerts.AlertConfig{
		Enabled:        true,
		StorageDefault: alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		DockerIgnoredContainerPrefixes: []string{
			"  Foo ",
			"foo",
			"Bar",
			" bar ",
		},
	}

	if err := cp.SaveAlertConfig(cfg); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	loaded, err := cp.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}

	expected := []string{"Foo", "Bar"}
	if !reflect.DeepEqual(loaded.DockerIgnoredContainerPrefixes, expected) {
		t.Fatalf("unexpected prefixes: got %v want %v", loaded.DockerIgnoredContainerPrefixes, expected)
	}
}

func TestLoadAlertConfigAppliesDefaults(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	raw := alerts.AlertConfig{
		Enabled:                        false,
		TimeThreshold:                  0,
		TimeThresholds:                 map[string]int{"guest": 0, "node": 0},
		DockerIgnoredContainerPrefixes: []string{" Runner "},
		SnapshotDefaults: alerts.SnapshotAlertConfig{
			Enabled:      true,
			WarningDays:  20,
			CriticalDays: 10,
		},
		NodeDefaults: alerts.ThresholdConfig{
			Temperature: &alerts.HysteresisThreshold{Trigger: 0, Clear: 0},
		},
	}

	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, "alerts.json"), data, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := cp.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}

	if loaded.TimeThreshold != 5 {
		t.Fatalf("expected time threshold default 5, got %d", loaded.TimeThreshold)
	}
	if got := loaded.TimeThresholds["guest"]; got != 5 {
		t.Fatalf("expected guest threshold default 5, got %d", got)
	}
	if got := loaded.TimeThresholds["node"]; got != 5 {
		t.Fatalf("expected node threshold default 5, got %d", got)
	}
	if loaded.NodeDefaults.Temperature == nil {
		t.Fatalf("expected node temperature defaults to be set")
	}
	if loaded.NodeDefaults.Temperature.Trigger != 80 || loaded.NodeDefaults.Temperature.Clear != 75 {
		t.Fatalf("expected temperature defaults 80/75, got %+v", loaded.NodeDefaults.Temperature)
	}
	expectedPrefixes := []string{"Runner"}
	if !reflect.DeepEqual(loaded.DockerIgnoredContainerPrefixes, expectedPrefixes) {
		t.Fatalf("expected normalized prefixes %v, got %v", expectedPrefixes, loaded.DockerIgnoredContainerPrefixes)
	}
	if loaded.SnapshotDefaults.Enabled != true {
		t.Fatalf("expected snapshot defaults to preserve enabled state")
	}
	if loaded.SnapshotDefaults.WarningDays != 10 {
		t.Fatalf("expected snapshot warning days normalized to critical, got %d", loaded.SnapshotDefaults.WarningDays)
	}
	if loaded.SnapshotDefaults.CriticalDays != 10 {
		t.Fatalf("expected snapshot critical days preserved at 10, got %d", loaded.SnapshotDefaults.CriticalDays)
	}
}

func TestAppriseConfigPersistence(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	cfg := notifications.AppriseConfig{
		Enabled:        true,
		Targets:        []string{"  discord://token  ", "", "mailto://alerts@example.com"},
		CLIPath:        " /usr/local/bin/apprise ",
		TimeoutSeconds: 3,
	}

	if err := cp.SaveAppriseConfig(cfg); err != nil {
		t.Fatalf("SaveAppriseConfig: %v", err)
	}

	loaded, err := cp.LoadAppriseConfig()
	if err != nil {
		t.Fatalf("LoadAppriseConfig: %v", err)
	}

	if !loaded.Enabled {
		t.Fatalf("expected config to remain enabled")
	}

	expectedTargets := []string{"discord://token", "mailto://alerts@example.com"}
	if !reflect.DeepEqual(loaded.Targets, expectedTargets) {
		t.Fatalf("unexpected targets: got %v want %v", loaded.Targets, expectedTargets)
	}

	if loaded.CLIPath != "/usr/local/bin/apprise" {
		t.Fatalf("expected CLI path to be trimmed, got %q", loaded.CLIPath)
	}

	if loaded.TimeoutSeconds != 5 {
		t.Fatalf("expected timeout normalized to minimum 5 seconds, got %d", loaded.TimeoutSeconds)
	}

	// Clearing targets should disable the config on next load
	if err := cp.SaveAppriseConfig(notifications.AppriseConfig{Enabled: true}); err != nil {
		t.Fatalf("SaveAppriseConfig empty: %v", err)
	}

	empty, err := cp.LoadAppriseConfig()
	if err != nil {
		t.Fatalf("LoadAppriseConfig empty: %v", err)
	}
	if empty.Enabled {
		t.Fatalf("expected disabled configuration when no targets stored")
	}
}
