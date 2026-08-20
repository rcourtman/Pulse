package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesPersistedGuestDockerInventoryOptIn(t *testing.T) {
	dir := t.TempDir()
	settings := map[string]interface{}{
		"enableProxmoxGuestDockerInventory": true,
	}
	raw, _ := json.Marshal(settings)
	if err := os.WriteFile(filepath.Join(dir, "system.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PULSE_DATA_DIR", dir)
	t.Setenv("PULSE_MOCK_MODE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableProxmoxGuestDockerInventory {
		t.Fatal("persisted guest Docker inventory opt-in was not applied at load")
	}
}
