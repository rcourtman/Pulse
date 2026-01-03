package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDockerMetadataStore(t *testing.T) {
	tempDir := t.TempDir()
	store := NewDockerMetadataStore(tempDir)
	assert.NotNil(t, store)
	assert.Empty(t, store.GetAll())
	assert.Empty(t, store.GetAllHostMetadata())
}

func TestDockerMetadataStore_Container_CRUD(t *testing.T) {
	tempDir := t.TempDir()
	store := NewDockerMetadataStore(tempDir)

	id := "container1"
	meta := &DockerMetadata{
		ID:          id,
		Description: "Test Container",
	}

	err := store.Set(id, meta)
	require.NoError(t, err)

	readMeta := store.Get(id)
	assert.Equal(t, meta.Description, readMeta.Description)

	store2 := NewDockerMetadataStore(tempDir)
	assert.Equal(t, meta.Description, store2.Get(id).Description)

	err = store.Delete(id)
	require.NoError(t, err)
	assert.Nil(t, store.Get(id))
}

func TestDockerMetadataStore_Host_CRUD(t *testing.T) {
	tempDir := t.TempDir()
	store := NewDockerMetadataStore(tempDir)

	id := "host1"
	meta := &DockerHostMetadata{
		CustomDisplayName: "My Host",
	}

	err := store.SetHostMetadata(id, meta)
	require.NoError(t, err)

	readMeta := store.GetHostMetadata(id)
	assert.Equal(t, "My Host", readMeta.CustomDisplayName)

	store2 := NewDockerMetadataStore(tempDir)
	assert.Equal(t, "My Host", store2.GetHostMetadata(id).CustomDisplayName)

	// Delete by setting empty/nil
	err = store.SetHostMetadata(id, nil)
	require.NoError(t, err)
	assert.Nil(t, store.GetHostMetadata(id))
}

func TestDockerMetadataStore_Set_NilContainer(t *testing.T) {
	store := NewDockerMetadataStore(t.TempDir())
	err := store.Set("id", nil)
	assert.Error(t, err)
}

func TestDockerMetadataStore_ReplaceAll(t *testing.T) {
	tempDir := t.TempDir()
	store := NewDockerMetadataStore(tempDir)

	newMeta := map[string]*DockerMetadata{
		"c1": {Description: "C1", Tags: nil},
	}

	err := store.ReplaceAll(newMeta)
	require.NoError(t, err)

	assert.Equal(t, "C1", store.Get("c1").Description)
	assert.NotNil(t, store.Get("c1").Tags)
}

func TestDockerMetadataStore_Load_Legacy(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "docker_metadata.json")

	// Legacy format: Top level map is containers
	legacyContent := `{"c1": {"id": "c1", "description": "Legacy"}}`
	require.NoError(t, os.WriteFile(filePath, []byte(legacyContent), 0644))

	store := NewDockerMetadataStore(tempDir)
	assert.Equal(t, "Legacy", store.Get("c1").Description)

	// Save should upgrade format
	err := store.Set("c2", &DockerMetadata{ID: "c2", Description: "New"})
	require.NoError(t, err)

	// Check file content for "containers" key
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `"containers"`)
}

func TestDockerMetadataStore_Load_Versioned(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "docker_metadata.json")

	content := `{
		"containers": {"c1": {"id": "c1"}},
		"hosts": {"h1": {"customDisplayName": "H1"}}
	}`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	store := NewDockerMetadataStore(tempDir)
	assert.NotNil(t, store.Get("c1"))
	assert.NotNil(t, store.GetHostMetadata("h1"))
}

func TestDockerMetadataStore_Load_Error(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "docker_metadata.json")
	require.NoError(t, os.WriteFile(filePath, []byte("{invalid"), 0644))

	store := NewDockerMetadataStore(tempDir)
	assert.NotNil(t, store)

	err := store.Load()
	assert.Error(t, err)
}

func TestDockerMetadataStore_Save_Error(t *testing.T) {
	tempDir := t.TempDir()
	badPath := filepath.Join(tempDir, "file")
	require.NoError(t, os.WriteFile(badPath, []byte("content"), 0644))

	store := NewDockerMetadataStore(tempDir)
	store.dataPath = badPath // hack

	err := store.Set("c1", &DockerMetadata{ID: "c1"})
	assert.Error(t, err)
}
