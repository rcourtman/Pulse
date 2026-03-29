package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigPersistence_Scenarios(t *testing.T) {
	// 1. configDir empty, PULSE_DATA_DIR set
	t.Run("PULSE_DATA_DIR", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("PULSE_DATA_DIR", tempDir)
		cp, err := newConfigPersistence("")
		require.NoError(t, err)
		assert.Equal(t, tempDir, cp.configDir)
	})

	// 2. Crypto initialization error
	t.Run("CryptoError", func(t *testing.T) {
		// Ensure the crypto migration path can't pick up a real on-disk legacy key.
		t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), ".encryption.key"))

		tempDir := t.TempDir()
		invalidPath := filepath.Join(tempDir, "file")
		err := os.WriteFile(invalidPath, []byte("not a dir"), 0644)
		require.NoError(t, err)

		info, err := os.Stat(invalidPath)
		require.NoError(t, err)
		assert.False(t, info.IsDir(), "Path should be a file, not a directory")

		_, err = newConfigPersistence(invalidPath)
		assert.Error(t, err, "Expected error when configDir is a file")
	})
}

func TestNewConfigPersistence_CanonicalizesOwnedConfigPaths(t *testing.T) {
	root := t.TempDir()
	explicit := filepath.Join(root, "nested", "..", "nested")

	cp, err := newConfigPersistence("  " + explicit + "  ")
	require.NoError(t, err)

	expectedDir := filepath.Clean(explicit)
	assert.Equal(t, expectedDir, cp.configDir)
	assert.Equal(t, filepath.Join(expectedDir, "alerts.json"), cp.alertFile)
	assert.Equal(t, filepath.Join(expectedDir, "system.json"), cp.systemFile)
	assert.Equal(t, filepath.Join(expectedDir, "ai_chat_sessions.json"), cp.aiChatSessionsFile)
	assert.Equal(t, filepath.Join(expectedDir, "relay.enc"), cp.relayFile)
}

func TestNewConfigPersistence_RejectsBlankResolvedConfigDir(t *testing.T) {
	originalDefaultDataDir := defaultDataDir
	defaultDataDir = " \t "
	t.Cleanup(func() { defaultDataDir = originalDefaultDataDir })
	t.Setenv("PULSE_DATA_DIR", " \t ")

	_, err := newConfigPersistence(" \t ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve config directory")
}
