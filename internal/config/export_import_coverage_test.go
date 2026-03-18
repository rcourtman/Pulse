package config

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportConfig_ErrorPaths(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	// 1. Passphrase required
	_, err := cp.ExportConfig("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "passphrase is required")

	// 2. LoadNodesConfig error
	mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("load error")}
	cp.SetFileSystem(mfs)
	_, err = cp.ExportConfig("pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load nodes config")
}

func TestImportConfig_ErrorPaths(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	// 1. Passphrase required
	err := cp.ImportConfig("data", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "passphrase is required")

	// 2. Invalid base64
	err = cp.ImportConfig("invalid-base64-!!!", "pass")
	assert.Error(t, err)

	// 3. Decryption failure (wrong passphrase / corrupted data)
	invalidEncrypted := base64.StdEncoding.EncodeToString([]byte("not-encrypted-properly"))
	err = cp.ImportConfig(invalidEncrypted, "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt")
}
