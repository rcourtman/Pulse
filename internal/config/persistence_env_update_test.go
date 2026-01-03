package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveSystemSettings_UpdateEnvFile_Content(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	envFile := filepath.Join(tempDir, ".env")

	// 1. Setup initial .env with various fields
	initialContent := `
POLLING_INTERVAL=10
UPDATE_CHANNEL=beta
AUTO_UPDATE_ENABLED=true
AUTO_UPDATE_CHECK_INTERVAL=3600
OTHER_VAR=value
`
	err := os.WriteFile(envFile, []byte(strings.TrimSpace(initialContent)), 0600)
	require.NoError(t, err)

	// 2. Save settings that should update .env
	settings := SystemSettings{
		UpdateChannel:           "stable",
		AutoUpdateEnabled:       false,
		AutoUpdateCheckInterval: 7200,
	}

	err = cp.SaveSystemSettings(settings)
	assert.NoError(t, err)

	// 3. Verify .env content
	data, err := os.ReadFile(envFile)
	require.NoError(t, err)
	content := string(data)

	// Check updates
	assert.Contains(t, content, "UPDATE_CHANNEL=stable")
	assert.Contains(t, content, "AUTO_UPDATE_ENABLED=false")
	assert.Contains(t, content, "AUTO_UPDATE_CHECK_INTERVAL=7200")
	assert.Contains(t, content, "OTHER_VAR=value")

	// Check removal of deprecated
	assert.NotContains(t, content, "POLLING_INTERVAL=")

	// Check original values are gone
	assert.NotContains(t, content, "UPDATE_CHANNEL=beta")
	assert.NotContains(t, content, "AUTO_UPDATE_ENABLED=true")
	assert.NotContains(t, content, "AUTO_UPDATE_CHECK_INTERVAL=3600")
}

func TestSaveSystemSettings_UpdateEnvFile_NoUpdate(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	envFile := filepath.Join(tempDir, ".env")

	// Case where settings values are empty, shouldn't replace if not set?
	// Based on code:
	// UPDATE_CHANNEL replaced if settings.UpdateChannel != ""
	// AUTO_UPDATE_ENABLED always replaced
	// AUTO_UPDATE_CHECK_INTERVAL replaced if > 0

	initialContent := `
UPDATE_CHANNEL=beta
AUTO_UPDATE_CHECK_INTERVAL=3600
`
	err := os.WriteFile(envFile, []byte(strings.TrimSpace(initialContent)), 0600)
	require.NoError(t, err)

	settings := SystemSettings{
		UpdateChannel:           "", // Empty, should not replace
		AutoUpdateCheckInterval: 0,  // Zero, should not replace
	}

	err = cp.SaveSystemSettings(settings)
	assert.NoError(t, err)

	data, err := os.ReadFile(envFile)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "UPDATE_CHANNEL=beta")
	assert.Contains(t, content, "AUTO_UPDATE_CHECK_INTERVAL=3600")
}

func TestSaveSystemSettings_EnvFileMissing(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	// Do NOT create .env file

	settings := SystemSettings{UpdateChannel: "stable"}
	err := cp.SaveSystemSettings(settings)
	assert.NoError(t, err)
	// Should cover IsNotExist branch
}

func TestSaveSystemSettings_EnvWriteError(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	// Create .env file so it tries to update it
	envFile := filepath.Join(tempDir, ".env")
	os.WriteFile(envFile, []byte("UPDATE_CHANNEL=beta"), 0600)

	// Use mock FS to fail write to .env
	// We need mockFSWriteSpecific from persistence_coverage_test.go
	// (Available since same package)
	mfs := &mockFSWriteSpecific{FileSystem: defaultFileSystem{}, failPattern: ".env"}
	cp.SetFileSystem(mfs)

	settings := SystemSettings{UpdateChannel: "stable"}
	err := cp.SaveSystemSettings(settings)
	assert.NoError(t, err) // Should suppress error
	// But it should have tried to write and failed
}
