package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAIConfigPersistence(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	cfg := config.AIConfig{
		Enabled: true,
		Provider: "anthropic",
		APIKey: "test-key",
		Model: "claude-3-opus",
	}

	if err := cp.SaveAIConfig(cfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	loaded, err := cp.LoadAIConfig()
	if err != nil {
		t.Fatalf("LoadAIConfig: %v", err)
	}

	if loaded.Enabled != cfg.Enabled || loaded.Provider != cfg.Provider || loaded.APIKey != cfg.APIKey {
		t.Errorf("Loaded config mismatch: %+v", loaded)
	}
}

func TestAIFindingsPersistence(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	findings := map[string]*config.AIFindingRecord{
		"f1": {
			ID: "f1",
			Title: "Test Finding",
			Severity: "warning",
			ResourceID: "res-1",
		},
	}

	if err := cp.SaveAIFindings(findings); err != nil {
		t.Fatalf("SaveAIFindings: %v", err)
	}

	loaded, err := cp.LoadAIFindings()
	if err != nil {
		t.Fatalf("LoadAIFindings: %v", err)
	}

	if len(loaded.Findings) != 1 || loaded.Findings["f1"].Title != "Test Finding" {
		t.Errorf("Loaded findings mismatch: %+v", loaded)
	}
}

func TestIsEncryptionEnabled(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	
	// NewConfigPersistence always enables encryption by generating a key if missing
	if !cp.IsEncryptionEnabled() {
		t.Error("Encryption should be enabled by default")
	}

	// Verify the key file was created
	keyPath := filepath.Join(tempDir, ".encryption.key")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Encryption key file should be created automatically")
	}
}

func TestMetadataPersistence(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)

	// 1. Guest Metadata
	guestMeta := map[string]*config.GuestMetadata{
		"guest-1": {
			ID:    "guest-1",
			Notes: []string{"Important guest"},
		},
	}
	
	// Create the file manually since SaveGuestMetadata doesn't exist in ConfigPersistence (it's in GuestMetadataStore)
	// but LoadGuestMetadata is in ConfigPersistence.
	// This tests the LoadGuestMetadata method in persistence.go
	guestFile := filepath.Join(tempDir, "guest_metadata.json")
	data, _ := json.Marshal(guestMeta)
	os.WriteFile(guestFile, data, 0644)

	loadedGuest, err := cp.LoadGuestMetadata()
	if err != nil {
		t.Fatalf("LoadGuestMetadata failed: %v", err)
	}
	if len(loadedGuest) != 1 || loadedGuest["guest-1"].Notes[0] != "Important guest" {
		t.Errorf("Loaded guest metadata mismatch: %+v", loadedGuest)
	}

	// 2. Docker Metadata
	dockerMeta := map[string]*config.DockerMetadata{
		"docker-1": {
			ID: "docker-1",
			Notes: []string{"Worker node"},
		},
	}
	dockerFile := filepath.Join(tempDir, "docker_metadata.json")
	data, _ = json.Marshal(dockerMeta)
	os.WriteFile(dockerFile, data, 0644)

	loadedDocker, err := cp.LoadDockerMetadata()
	if err != nil {
		t.Fatalf("LoadDockerMetadata failed: %v", err)
	}
	if len(loadedDocker) != 1 || loadedDocker["docker-1"].Notes[0] != "Worker node" {
		t.Errorf("Loaded docker metadata mismatch: %+v", loadedDocker)
	}
}
