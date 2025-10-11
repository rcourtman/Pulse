package config_test

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
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
