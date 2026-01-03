package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHostMetadataStore(t *testing.T) {
	tempDir := t.TempDir()
	store := NewHostMetadataStore(tempDir, nil)
	assert.NotNil(t, store)
	assert.Empty(t, store.GetAll())
}

func TestHostMetadataStore_CRUD(t *testing.T) {
	tempDir := t.TempDir()
	store := NewHostMetadataStore(tempDir, nil)

	hostID := "host1"
	meta := &HostMetadata{
		ID:          hostID,
		Description: "Test Host",
		Tags:        []string{"tag1", "tag2"},
	}

	// Create
	err := store.Set(hostID, meta)
	require.NoError(t, err)

	// Read
	readMeta := store.Get(hostID)
	require.NotNil(t, readMeta)
	assert.Equal(t, meta.Description, readMeta.Description)
	assert.Equal(t, meta.Tags, readMeta.Tags)

	// Persistence Check
	store2 := NewHostMetadataStore(tempDir, nil)
	readMeta2 := store2.Get(hostID)
	require.NotNil(t, readMeta2)
	assert.Equal(t, meta.Description, readMeta2.Description)

	// Update
	meta.Description = "Updated Host"
	err = store.Set(hostID, meta)
	require.NoError(t, err)
	assert.Equal(t, "Updated Host", store.Get(hostID).Description)

	// GetAll
	all := store.GetAll()
	assert.Len(t, all, 1)
	assert.Contains(t, all, hostID)

	// Delete
	err = store.Delete(hostID)
	require.NoError(t, err)
	assert.Nil(t, store.Get(hostID))

	// Persistence Check after delete
	store3 := NewHostMetadataStore(tempDir, nil)
	assert.Nil(t, store3.Get(hostID))
}

func TestHostMetadataStore_ReplaceAll(t *testing.T) {
	tempDir := t.TempDir()
	store := NewHostMetadataStore(tempDir, nil)

	newMetadata := map[string]*HostMetadata{
		"host1": {Description: "Host 1"},
		"host2": {Description: "Host 2", Tags: nil}, // Test nil tags handling
	}

	err := store.ReplaceAll(newMetadata)
	require.NoError(t, err)

	assert.Equal(t, "Host 1", store.Get("host1").Description)
	assert.Equal(t, "Host 2", store.Get("host2").Description)
	assert.NotNil(t, store.Get("host2").Tags) // Should contain empty slice, not nil

	// Verify nil items strictly skipped or handled
	err = store.ReplaceAll(map[string]*HostMetadata{
		"host3": nil,
	})
	require.NoError(t, err)
	assert.Nil(t, store.Get("host3"))
}

func TestHostMetadataStore_Set_Nil(t *testing.T) {
	store := NewHostMetadataStore(t.TempDir(), nil)
	err := store.Set("host1", nil)
	assert.Error(t, err)
}

func TestHostMetadataStore_Load_Error(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "host_metadata.json")
	require.NoError(t, os.WriteFile(filePath, []byte("{invalid-json"), 0644))

	store := NewHostMetadataStore(tempDir, nil)
	// It logs warning but returns store.
	assert.NotNil(t, store)
	assert.Empty(t, store.GetAll())

	// Explicit load call
	err := store.Load()
	assert.Error(t, err)
}

func TestHostMetadataStore_Save_Error(t *testing.T) {
	// Use a read-only directory to simulate save error
	tempDir := t.TempDir()
	readOnlyDir := filepath.Join(tempDir, "readonly")
	require.NoError(t, os.Mkdir(readOnlyDir, 0555))

	store := NewHostMetadataStore(readOnlyDir, nil)

	// Try to fail save
	// Making the directory strictly read-only might work,
	// but t.TempDir cleanup might fail if we don't fix permissions.
	// Alternative: Point to a file as directory.

	badPath := filepath.Join(tempDir, "file")
	require.NoError(t, os.WriteFile(badPath, []byte("content"), 0644))

	store.dataPath = badPath // Hack internal field for testing failure

	err := store.Set("host1", &HostMetadata{})
	assert.Error(t, err)
}
