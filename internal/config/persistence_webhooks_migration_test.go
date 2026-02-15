package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockFSRenameSpecific struct {
	FileSystem
	failPattern string
}

func (m *mockFSRenameSpecific) Rename(oldpath, newpath string) error {
	if m.failPattern != "" && strings.Contains(newpath, m.failPattern) {
		return errors.New("specific rename error")
	}
	return m.FileSystem.Rename(oldpath, newpath)
}

func TestMigrateWebhooksIfNeeded_Scenarios(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	// Use mock file system
	mfs := &mockFSError{FileSystem: defaultFileSystem{}}
	cp.SetFileSystem(mfs)

	// 1. Encrypted file exists
	t.Run("EncryptedExists", func(t *testing.T) {
		cp.webhookFile = filepath.Join(tempDir, "webhooks.enc")
		_ = os.WriteFile(cp.webhookFile, []byte("exists"), 0600)
		err := cp.MigrateWebhooksIfNeeded()
		assert.NoError(t, err)
	})

	// 2. Legacy file doesn't exist
	t.Run("LegacyNotExists", func(t *testing.T) {
		os.Remove(cp.webhookFile)
		err := cp.MigrateWebhooksIfNeeded()
		assert.NoError(t, err)
	})

	// 3. Legacy file Read error
	t.Run("LegacyReadError", func(t *testing.T) {
		legacyFile := filepath.Join(tempDir, "webhooks.json")
		_ = os.WriteFile(legacyFile, []byte("[]"), 0600)

		mfs.readError = errors.New("read error")
		err := cp.MigrateWebhooksIfNeeded()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read legacy webhooks")
		mfs.readError = nil
	})

	// 4. Legacy file Unmarshal error
	t.Run("LegacyUnmarshalError", func(t *testing.T) {
		legacyFile := filepath.Join(tempDir, "webhooks.json")
		_ = os.WriteFile(legacyFile, []byte("not-json"), 0600)

		err := cp.MigrateWebhooksIfNeeded()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse legacy webhooks")
	})

	// 5. Success with Rename error for backup
	t.Run("RenameBackupError", func(t *testing.T) {
		legacyFile := filepath.Join(tempDir, "webhooks.json")
		_ = os.WriteFile(legacyFile, []byte("[]"), 0600)

		// Use specific rename mock to let SaveWebhooks succeed but fail the backup rename
		mfsSpec := &mockFSRenameSpecific{FileSystem: defaultFileSystem{}, failPattern: ".json.backup"}
		cp.SetFileSystem(mfsSpec)

		err := cp.MigrateWebhooksIfNeeded()
		assert.NoError(t, err) // Should only warn on rename error
	})
}
