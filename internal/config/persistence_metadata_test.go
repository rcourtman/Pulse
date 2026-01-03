package config_test

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigPersistence_LoadGuestMetadata(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)

	store, err := cp.LoadGuestMetadata()
	require.NoError(t, err)
	assert.NotNil(t, store)

	// Ensure we can use the store
	assert.Empty(t, store.GetAll())
}

func TestConfigPersistence_LoadDockerMetadata(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)

	store, err := cp.LoadDockerMetadata()
	require.NoError(t, err)
	assert.NotNil(t, store)

	// Ensure we can use the store
	assert.Empty(t, store.GetAll())
}
