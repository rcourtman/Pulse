package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAlertConfig_ReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping as root")
	}
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	// Create unreadable alerts.json
	path := filepath.Join(tempDir, "alerts.json")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0000))

	cfg, err := cp.LoadAlertConfig()
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadAlertConfig_UnmarshalError(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	path := filepath.Join(tempDir, "alerts.json")
	require.NoError(t, os.WriteFile(path, []byte("{invalid"), 0644))

	cfg, err := cp.LoadAlertConfig()
	assert.Error(t, err)
	assert.Nil(t, cfg)
}
