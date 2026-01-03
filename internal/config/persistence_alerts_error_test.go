package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadAlertConfig_ReadFileError(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("read error")}
	cp.SetFileSystem(mfs)

	_, err := cp.LoadAlertConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read error")
}
