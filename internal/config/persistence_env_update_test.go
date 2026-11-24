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
		UpdateChannel:     "stable",
		AutoUpdateEnabled: false,
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
	assert.Contains(t, content, "OTHER_VAR=value")

	// A legacy AUTO_UPDATE_CHECK_INTERVAL line is preserved verbatim; the
	// setting was removed as dead (nothing ever consumed it) so saves no
	// longer rewrite it.
	assert.Contains(t, content, "AUTO_UPDATE_CHECK_INTERVAL=3600")

	// Check removal of deprecated
	assert.NotContains(t, content, "POLLING_INTERVAL=")

	// Check original values are gone
	assert.NotContains(t, content, "UPDATE_CHANNEL=beta")
	assert.NotContains(t, content, "AUTO_UPDATE_ENABLED=true")
}

func TestSaveSystemSettings_UpdateEnvFile_NoUpdate(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	envFile := filepath.Join(tempDir, ".env")

	// Case where settings values are empty, shouldn't replace if not set?
	// Based on code:
	// UPDATE_CHANNEL replaced if settings.UpdateChannel is a supported channel
	// AUTO_UPDATE_ENABLED always replaced
	// AUTO_UPDATE_CHECK_INTERVAL is legacy and always preserved verbatim

	initialContent := `
UPDATE_CHANNEL=beta
AUTO_UPDATE_CHECK_INTERVAL=3600
`
	err := os.WriteFile(envFile, []byte(strings.TrimSpace(initialContent)), 0600)
	require.NoError(t, err)

	settings := SystemSettings{
		UpdateChannel: "", // Empty, should not replace
	}

	err = cp.SaveSystemSettings(settings)
	assert.NoError(t, err)

	data, err := os.ReadFile(envFile)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "UPDATE_CHANNEL=beta")
	assert.Contains(t, content, "AUTO_UPDATE_CHECK_INTERVAL=3600")
}

func TestSaveSystemSettings_UpdateEnvFile_WritesOnlySupportedChannels(t *testing.T) {
	const initialContent = "PULSE_AUTH_USER=admin\nUPDATE_CHANNEL=rc\nOTHER_VAR=value\n"

	// A channel is one line of .env. Values the settings API would refuse can
	// still arrive from an imported backup, and must not add lines of their own.
	for _, channel := range []string{
		"stable\nPROXY_AUTH_SECRET=planted\nPROXY_AUTH_USER_HEADER=X-Remote-User",
		"stable\r\nPROXY_AUTH_SECRET=planted",
		"stable\rPROXY_AUTH_SECRET=planted",
		"stable # comment",
		"'stable'",
		"beta",
	} {
		tempDir := t.TempDir()
		cp := NewConfigPersistence(tempDir)
		envFile := filepath.Join(tempDir, ".env")
		require.NoError(t, os.WriteFile(envFile, []byte(initialContent), 0600))

		require.NoError(t, cp.SaveSystemSettings(SystemSettings{UpdateChannel: channel}))

		data, err := os.ReadFile(envFile)
		require.NoError(t, err)
		assert.Equal(t, initialContent, string(data), "channel %q must leave .env unchanged", channel)
	}

	// A supported channel is still kept in sync, in its canonical spelling.
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	envFile := filepath.Join(tempDir, ".env")
	require.NoError(t, os.WriteFile(envFile, []byte(initialContent), 0600))

	require.NoError(t, cp.SaveSystemSettings(SystemSettings{UpdateChannel: " Stable "}))

	data, err := os.ReadFile(envFile)
	require.NoError(t, err)
	assert.Equal(t, "PULSE_AUTH_USER=admin\nUPDATE_CHANNEL=stable\nOTHER_VAR=value\n", string(data))
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
	_ = os.WriteFile(envFile, []byte("UPDATE_CHANNEL=beta"), 0600)

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
