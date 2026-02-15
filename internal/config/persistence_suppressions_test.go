package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadEnvTokenSuppressions_Branches(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	suppFile := filepath.Join(tempDir, "env_token_suppressions.json")

	// 1. ReadFile error
	mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("read error")}
	cp.SetFileSystem(mfs)
	_, err := cp.LoadEnvTokenSuppressions()
	assert.Error(t, err)
	mfs.readError = nil

	// 2. Empty data
	_ = os.WriteFile(suppFile, []byte(""), 0600)
	hashes, err := cp.LoadEnvTokenSuppressions()
	assert.NoError(t, err)
	assert.Empty(t, hashes)

	// 3. Unmarshal error
	_ = os.WriteFile(suppFile, []byte("not-json"), 0600)
	_, err = cp.LoadEnvTokenSuppressions()
	assert.Error(t, err)
}
