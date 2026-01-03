package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveOIDCConfig(t *testing.T) {
	// Setup persistence
	tempDir := t.TempDir()
	p := NewConfigPersistence(tempDir)

	// Mock global persistence
	originalPersistence := globalPersistence
	globalPersistence = p
	defer func() { globalPersistence = originalPersistence }()

	// Test nil settings
	err := SaveOIDCConfig(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")

	// Test persistence not initialized (mock nil)
	globalPersistence = nil
	err = SaveOIDCConfig(&OIDCConfig{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "persistence not initialized")
	globalPersistence = p

	// Test Valid Save
	settings := &OIDCConfig{
		Enabled:   true,
		IssuerURL: "https://issuer.com",
		ClientID:  "client-id",
	}

	err = SaveOIDCConfig(settings)
	require.NoError(t, err)

	// Verify persistence
	loaded, err := p.LoadOIDCConfig()
	require.NoError(t, err)
	assert.Equal(t, settings.IssuerURL, loaded.IssuerURL)
}

func TestLoadHostMetadata_Wait(t *testing.T) {
	// Just to make sure we covered HostMetadataStore if I missed anything
	// (Already covered in host_metadata_test.go)
}
