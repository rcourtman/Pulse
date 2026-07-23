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

func TestConfigPersistenceSharesActiveMetadataStores(t *testing.T) {
	cp := config.NewConfigPersistence(t.TempDir())
	guest := config.NewGuestMetadataStore(t.TempDir(), nil)
	docker := config.NewDockerMetadataStore(t.TempDir(), nil)
	host := config.NewHostMetadataStore(t.TempDir(), nil)

	cp.SetMetadataStores(guest, docker, host)

	assert.Same(t, guest, cp.GetGuestMetadataStore())
	assert.Same(t, docker, cp.GetDockerMetadataStore())
	assert.Same(t, host, cp.GetHostMetadataStore())
}
