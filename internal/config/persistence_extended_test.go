package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

func TestConfigPersistence_DataDir(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if cp.DataDir() != tempDir {
		t.Errorf("Expected %s, got %s", tempDir, cp.DataDir())
	}
}

func TestConfigPersistence_MigrateWebhooksIfNeeded(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)

	// 1. Create legacy file
	legacyFile := filepath.Join(tempDir, "webhooks.json")
	legacyWebhooks := []notifications.WebhookConfig{
		{ID: "webhook-1", URL: "http://example.com/legacy"},
	}
	data, _ := json.Marshal(legacyWebhooks)
	os.WriteFile(legacyFile, data, 0644)

	// 2. Migrate
	if err := cp.MigrateWebhooksIfNeeded(); err != nil {
		t.Fatalf("MigrateWebhooksIfNeeded failed: %v", err)
	}

	// 3. Verify encrypted file exists (since SaveWebhooks uses encryption if key exists)
	// NewConfigPersistence generates a key if it doesn't exist, so encryption IS enabled by default.
	loaded, err := cp.LoadWebhooks()
	if err != nil {
		t.Fatalf("LoadWebhooks failed: %v", err)
	}
	if len(loaded) != 1 || loaded[0].URL != "http://example.com/legacy" {
		t.Errorf("Migration failed to preserve data: %+v", loaded)
	}
}

func TestConfigPersistence_PatrolRunHistory(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)

	runs := []config.PatrolRunRecord{
		{
			ID:               "run-1",
			StartedAt:        time.Now().Add(-1 * time.Hour),
			CompletedAt:      time.Now().Add(-59 * time.Minute),
			DurationMs:       60000,
			Type:             "quick",
			ResourcesChecked: 10,
			NewFindings:      2,
		},
	}

	if err := cp.SavePatrolRunHistory(runs); err != nil {
		t.Fatalf("SavePatrolRunHistory failed: %v", err)
	}

	history, err := cp.LoadPatrolRunHistory()
	if err != nil {
		t.Fatalf("LoadPatrolRunHistory failed: %v", err)
	}

	if len(history.Runs) != 1 || history.Runs[0].ID != "run-1" {
		t.Errorf("Patrol history mismatch: %+v", history)
	}

	// Test non-existent file
	cp2 := config.NewConfigPersistence(t.TempDir())
	history2, err := cp2.LoadPatrolRunHistory()
	if err != nil {
		t.Fatalf("LoadPatrolRunHistory on empty dir failed: %v", err)
	}
	if len(history2.Runs) != 0 {
		t.Errorf("Expected 0 runs, got %d", len(history2.Runs))
	}
}

func TestConfigPersistence_UpdateEnvFile(t *testing.T) {
	tempDir := t.TempDir()
	envFile := filepath.Join(tempDir, ".env")

	initialContent := `UPDATE_CHANNEL=stable
AUTO_UPDATE_ENABLED=false
POLLING_INTERVAL=10
CUSTOM_VAR=value`
	os.WriteFile(envFile, []byte(initialContent), 0644)

	cp := config.NewConfigPersistence(tempDir)

	settings := config.SystemSettings{
		UpdateChannel:           "beta",
		AutoUpdateEnabled:       true,
		AutoUpdateCheckInterval: 3600,
	}

	if err := cp.SaveSystemSettings(settings); err != nil {
		t.Fatalf("SaveSystemSettings failed: %v", err)
	}

	updatedData, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("Failed to read .env file: %v", err)
	}
	updatedContent := string(updatedData)

	if !strings.Contains(updatedContent, "UPDATE_CHANNEL=beta") {
		t.Errorf("UPDATE_CHANNEL not updated. Content: %s", updatedContent)
	}
	if !strings.Contains(updatedContent, "AUTO_UPDATE_ENABLED=true") {
		t.Errorf("AUTO_UPDATE_ENABLED not updated. Content: %s", updatedContent)
	}
	if strings.Contains(updatedContent, "POLLING_INTERVAL=") {
		t.Error("POLLING_INTERVAL should have been removed")
	}
	if !strings.Contains(updatedContent, "CUSTOM_VAR=value") {
		t.Error("CUSTOM_VAR should have been preserved")
	}
}
