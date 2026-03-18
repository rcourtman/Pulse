package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/stretchr/testify/assert"
)

func TestSaveNodesConfig_Scenarios(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	nodesFile := filepath.Join(tempDir, "nodes.enc")

	// 1. Mock mode enabled
	t.Run("MockModeEnabled", func(t *testing.T) {
		mock.SetEnabled(true)
		defer mock.SetEnabled(false)

		err := cp.SaveNodesConfig([]PVEInstance{{Host: "test"}}, nil, nil)
		assert.NoError(t, err)

		// Verify file NOT created
		_, err = os.Stat(nodesFile)
		assert.True(t, os.IsNotExist(err))
	})

	// 2. Blocked Wipe branch
	t.Run("BlockedWipe", func(t *testing.T) {
		// Create a non-empty config first
		initialNodes := []PVEInstance{{Host: "existing"}}
		validData, _ := json.Marshal(NodesConfig{PVEInstances: initialNodes})
		_ = os.WriteFile(nodesFile, validData, 0600)

		// Attempt to save empty config with allowEmpty=false (default for SaveNodesConfig wrapper)
		err := cp.SaveNodesConfig([]PVEInstance{}, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "refusing to save empty nodes config")

		// Verify file still has original content
		data, _ := os.ReadFile(nodesFile)
		var cfg NodesConfig
		_ = json.Unmarshal(data, &cfg)
		assert.Equal(t, "existing", cfg.PVEInstances[0].Host)
	})

	// 3. Blocked Wipe with Crypto Decrypt Failure
	t.Run("BlockedWipe_DecryptFailure", func(t *testing.T) {
		cm, _ := crypto.NewCryptoManagerAt(tempDir)
		cp.crypto = cm

		// Write invalid encrypted data
		_ = os.WriteFile(nodesFile, []byte("invalid-encrypted-data-too-short"), 0600)

		err := cp.SaveNodesConfig([]PVEInstance{}, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "existing nodes config is not decryptable")
	})

	// 4. Blocked Wipe with JSON Parse Failure
	t.Run("BlockedWipe_ParseFailure", func(t *testing.T) {
		cp.crypto = nil
		_ = os.WriteFile(nodesFile, []byte("not json"), 0600)

		err := cp.SaveNodesConfig([]PVEInstance{}, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "existing nodes config is not parseable")
	})

	// 5. Success with Backups
	t.Run("SuccessWithBackups", func(t *testing.T) {
		_ = os.WriteFile(nodesFile, []byte("{}"), 0600) // Initial file

		err := cp.SaveNodesConfig([]PVEInstance{{Host: "new"}}, nil, nil)
		assert.NoError(t, err)

		// Check for backup file
		_, err = os.Stat(nodesFile + ".backup")
		assert.NoError(t, err)

		// Check for timestamped backup
		matches, _ := filepath.Glob(nodesFile + ".backup-*")
		assert.NotEmpty(t, matches)
	})

	// 6. Backup Rename Error
	t.Run("BackupRenameError", func(t *testing.T) {
		_ = os.WriteFile(nodesFile, []byte("{}"), 0600)

		mfs := &mockFSRenameSpecific{FileSystem: defaultFileSystem{}, failPattern: ".backup"}
		cp.SetFileSystem(mfs)

		err := cp.SaveNodesConfig([]PVEInstance{{Host: "new"}}, nil, nil)
		assert.NoError(t, err) // Should succeed despite backup error
	})

	// 7. Backup Write Error
	t.Run("BackupWriteError", func(t *testing.T) {
		_ = os.WriteFile(nodesFile, []byte("{}"), 0600)

		mfs := &mockFSWriteSpecific{FileSystem: defaultFileSystem{}, failPattern: ".backup"}
		cp.SetFileSystem(mfs)

		err := cp.SaveNodesConfig([]PVEInstance{{Host: "new"}}, nil, nil)
		assert.NoError(t, err) // Should succeed despite backup write error (logged warning)
	})

	// 8. Mkdir Error
	t.Run("MkdirError", func(t *testing.T) {
		mfs := &mockFSError{FileSystem: defaultFileSystem{}, mkdirError: os.ErrPermission}
		cp.SetFileSystem(mfs)

		err := cp.SaveNodesConfig([]PVEInstance{{Host: "new"}}, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
	})
}
