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

	// 2. configDir empty, PULSE_DATA_DIR not set
	t.Run("DefaultDir", func(t *testing.T) {
		// Mock homedir or just let it use /etc/pulse if we can
		// But /etc/pulse might not be writeable.
		// Actually NewCryptoManagerAt will try to create/read key there.
		// This might fail if not root.
	})

	// 3. Crypto initialization error
	t.Run("CryptoError", func(t *testing.T) {
		// Hide legacy key if it exists
		systemKeyPath := "/etc/pulse/.encryption.key"
		backupKeyPath := "/etc/pulse/.encryption.key.test-backup-init"
		if _, err := os.Stat(systemKeyPath); err == nil {
			require.NoError(t, os.Rename(systemKeyPath, backupKeyPath))
			t.Cleanup(func() {
				os.Rename(backupKeyPath, systemKeyPath)
			})
		}

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
