package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateWebhooksIfNeeded(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)

	// Create legacy webhooks.json
	legacyContent := `[{"url":"http://example.com/legacy","headers":{"X-Legacy":"true"}}]`
	legacyFile := filepath.Join(tempDir, "webhooks.json")
	require.NoError(t, os.WriteFile(legacyFile, []byte(legacyContent), 0644))

	// Ensure initialized (encryption key etc)
	require.NoError(t, cp.EnsureConfigDir())

	// Run migration
	err := cp.MigrateWebhooksIfNeeded()
	require.NoError(t, err)

	// Verify encryption file exists
	webhooksEnc := filepath.Join(tempDir, "webhooks.enc")
	assert.FileExists(t, webhooksEnc)

	// Verify backup exists
	assert.FileExists(t, legacyFile+".backup")
	assert.NoFileExists(t, legacyFile) // Original should be renamed

	// Load to verify content
	loaded, err := cp.LoadWebhooks()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, "http://example.com/legacy", loaded[0].URL)
	assert.Equal(t, "true", loaded[0].Headers["X-Legacy"])
}

func TestMigrateWebhooksIfNeeded_AlreadyMigrated(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	require.NoError(t, cp.EnsureConfigDir())

	// Create encrypted file (simulated by saving empty config)
	require.NoError(t, cp.SaveWebhooks(nil))

	// Create legacy file which should be IGNORED if encrypted exists
	legacyContent := `[{"url":"http://example.com/ignored"}]`
	legacyFile := filepath.Join(tempDir, "webhooks.json")
	require.NoError(t, os.WriteFile(legacyFile, []byte(legacyContent), 0644))

	err := cp.MigrateWebhooksIfNeeded()
	require.NoError(t, err)

	// Legacy file should still exist and NOT be backed up
	assert.FileExists(t, legacyFile)
	assert.NoFileExists(t, legacyFile+".backup")
}

func TestMigrateWebhooksIfNeeded_NoLegacy(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	require.NoError(t, cp.EnsureConfigDir())

	err := cp.MigrateWebhooksIfNeeded()
	require.NoError(t, err)

	webhooksEnc := filepath.Join(tempDir, "webhooks.enc")
	assert.NoFileExists(t, webhooksEnc)
}
