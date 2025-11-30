package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDockerMetadataStore_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Test get on empty store
	result := store.Get("nonexistent")
	if result != nil {
		t.Error("Get on empty store should return nil")
	}

	// Add metadata
	store.metadata["host1:container:abc123"] = &DockerMetadata{
		ID:          "host1:container:abc123",
		CustomURL:   "http://example.com",
		Description: "Test container",
		Tags:        []string{"tag1", "tag2"},
	}

	// Test get existing
	result = store.Get("host1:container:abc123")
	if result == nil {
		t.Fatal("Get should return metadata for existing entry")
	}
	if result.CustomURL != "http://example.com" {
		t.Errorf("CustomURL = %q, want %q", result.CustomURL, "http://example.com")
	}
	if result.Description != "Test container" {
		t.Errorf("Description = %q, want %q", result.Description, "Test container")
	}
	if len(result.Tags) != 2 {
		t.Errorf("Tags count = %d, want 2", len(result.Tags))
	}
}

func TestDockerMetadataStore_GetAll(t *testing.T) {
	tmpDir := t.TempDir()
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Test empty store
	all := store.GetAll()
	if len(all) != 0 {
		t.Errorf("GetAll on empty store returned %d entries, want 0", len(all))
	}

	// Add metadata
	store.metadata["id1"] = &DockerMetadata{ID: "id1", CustomURL: "url1"}
	store.metadata["id2"] = &DockerMetadata{ID: "id2", CustomURL: "url2"}

	// Test GetAll returns all entries
	all = store.GetAll()
	if len(all) != 2 {
		t.Errorf("GetAll returned %d entries, want 2", len(all))
	}

	// Verify it's a copy (modification shouldn't affect store)
	all["id3"] = &DockerMetadata{ID: "id3"}
	if len(store.metadata) != 2 {
		t.Error("GetAll should return a copy, not the original map")
	}
}

func TestDockerMetadataStore_GetHostMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Test get on empty store
	result := store.GetHostMetadata("nonexistent")
	if result != nil {
		t.Error("GetHostMetadata on empty store should return nil")
	}

	// Add host metadata
	store.hostMetadata["host1"] = &DockerHostMetadata{
		CustomDisplayName: "Production Server",
	}

	// Test get existing
	result = store.GetHostMetadata("host1")
	if result == nil {
		t.Fatal("GetHostMetadata should return metadata for existing entry")
	}
	if result.CustomDisplayName != "Production Server" {
		t.Errorf("CustomDisplayName = %q, want %q", result.CustomDisplayName, "Production Server")
	}
}

func TestDockerMetadataStore_GetAllHostMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Test empty store
	all := store.GetAllHostMetadata()
	if len(all) != 0 {
		t.Errorf("GetAllHostMetadata on empty store returned %d entries, want 0", len(all))
	}

	// Add host metadata
	store.hostMetadata["host1"] = &DockerHostMetadata{CustomDisplayName: "Host 1"}
	store.hostMetadata["host2"] = &DockerHostMetadata{CustomDisplayName: "Host 2"}

	// Test returns all entries
	all = store.GetAllHostMetadata()
	if len(all) != 2 {
		t.Errorf("GetAllHostMetadata returned %d entries, want 2", len(all))
	}

	// Verify it's a copy
	all["host3"] = &DockerHostMetadata{CustomDisplayName: "Host 3"}
	if len(store.hostMetadata) != 2 {
		t.Error("GetAllHostMetadata should return a copy, not the original map")
	}
}

func TestDockerMetadataStore_Set(t *testing.T) {
	tmpDir := t.TempDir()
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Test set nil
	err := store.Set("id1", nil)
	if err == nil {
		t.Error("Set with nil metadata should return error")
	}

	// Test successful set
	meta := &DockerMetadata{
		CustomURL:   "http://test.com",
		Description: "Test desc",
		Tags:        []string{"tag1"},
	}
	err = store.Set("id1", meta)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify ID is set
	if meta.ID != "id1" {
		t.Errorf("ID = %q, want %q", meta.ID, "id1")
	}

	// Verify stored
	stored := store.Get("id1")
	if stored == nil {
		t.Fatal("Set did not store metadata")
	}
	if stored.CustomURL != "http://test.com" {
		t.Errorf("Stored CustomURL = %q, want %q", stored.CustomURL, "http://test.com")
	}

	// Verify persisted to disk
	filePath := filepath.Join(tmpDir, "docker_metadata.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read persisted file: %v", err)
	}

	var fileData dockerMetadataFile
	if err := json.Unmarshal(data, &fileData); err != nil {
		t.Fatalf("Failed to unmarshal persisted data: %v", err)
	}
	if fileData.Containers["id1"] == nil {
		t.Error("Metadata not persisted in containers map")
	}
}

func TestDockerMetadataStore_SetHostMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Test set with valid metadata
	meta := &DockerHostMetadata{CustomDisplayName: "My Host"}
	err := store.SetHostMetadata("host1", meta)
	if err != nil {
		t.Fatalf("SetHostMetadata failed: %v", err)
	}

	// Verify stored
	stored := store.GetHostMetadata("host1")
	if stored == nil {
		t.Fatal("SetHostMetadata did not store metadata")
	}
	if stored.CustomDisplayName != "My Host" {
		t.Errorf("CustomDisplayName = %q, want %q", stored.CustomDisplayName, "My Host")
	}

	// Test set nil removes entry
	err = store.SetHostMetadata("host1", nil)
	if err != nil {
		t.Fatalf("SetHostMetadata with nil failed: %v", err)
	}
	if store.GetHostMetadata("host1") != nil {
		t.Error("SetHostMetadata with nil should delete entry")
	}

	// Test set with empty display name removes entry
	store.hostMetadata["host2"] = &DockerHostMetadata{CustomDisplayName: "Host 2"}
	err = store.SetHostMetadata("host2", &DockerHostMetadata{CustomDisplayName: ""})
	if err != nil {
		t.Fatalf("SetHostMetadata with empty display name failed: %v", err)
	}
	if store.GetHostMetadata("host2") != nil {
		t.Error("SetHostMetadata with empty display name should delete entry")
	}
}

func TestDockerMetadataStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Add metadata
	store.metadata["id1"] = &DockerMetadata{ID: "id1", CustomURL: "url1"}

	// Delete
	err := store.Delete("id1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	if store.Get("id1") != nil {
		t.Error("Delete did not remove metadata")
	}

	// Delete nonexistent (should not error)
	err = store.Delete("nonexistent")
	if err != nil {
		t.Errorf("Delete nonexistent should not error: %v", err)
	}
}

func TestDockerMetadataStore_ReplaceAll(t *testing.T) {
	tmpDir := t.TempDir()
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Add initial data
	store.metadata["old1"] = &DockerMetadata{ID: "old1"}

	// Replace all
	newData := map[string]*DockerMetadata{
		"new1": {CustomURL: "url1", Tags: []string{"t1"}},
		"new2": {CustomURL: "url2"},
	}

	err := store.ReplaceAll(newData)
	if err != nil {
		t.Fatalf("ReplaceAll failed: %v", err)
	}

	// Verify old data gone
	if store.Get("old1") != nil {
		t.Error("ReplaceAll should remove old entries")
	}

	// Verify new data present
	if store.Get("new1") == nil || store.Get("new2") == nil {
		t.Error("ReplaceAll should add new entries")
	}

	// Verify IDs set correctly
	if store.Get("new1").ID != "new1" {
		t.Errorf("ID = %q, want %q", store.Get("new1").ID, "new1")
	}

	// Verify nil tags converted to empty slice
	if store.Get("new2").Tags == nil {
		t.Error("ReplaceAll should convert nil tags to empty slice")
	}
}

func TestDockerMetadataStore_ReplaceAll_NilEntry(t *testing.T) {
	tmpDir := t.TempDir()
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Replace with map containing nil entry
	newData := map[string]*DockerMetadata{
		"valid": {CustomURL: "url1"},
		"nil":   nil,
	}

	err := store.ReplaceAll(newData)
	if err != nil {
		t.Fatalf("ReplaceAll failed: %v", err)
	}

	// Verify valid entry present
	if store.Get("valid") == nil {
		t.Error("ReplaceAll should add valid entries")
	}

	// Verify nil entry skipped
	if store.Get("nil") != nil {
		t.Error("ReplaceAll should skip nil entries")
	}
}

func TestDockerMetadataStore_Load_Versioned(t *testing.T) {
	tmpDir := t.TempDir()

	// Write versioned format file
	fileData := dockerMetadataFile{
		Containers: map[string]*DockerMetadata{
			"c1": {ID: "c1", CustomURL: "url1"},
			"c2": {ID: "c2", CustomURL: "url2"},
		},
		Hosts: map[string]*DockerHostMetadata{
			"h1": {CustomDisplayName: "Host 1"},
		},
	}
	data, _ := json.Marshal(fileData)
	filePath := filepath.Join(tmpDir, "docker_metadata.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}
	err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify containers loaded
	if len(store.metadata) != 2 {
		t.Errorf("Container count = %d, want 2", len(store.metadata))
	}
	if store.Get("c1") == nil || store.Get("c2") == nil {
		t.Error("Load did not load containers")
	}

	// Verify hosts loaded
	if len(store.hostMetadata) != 1 {
		t.Errorf("Host count = %d, want 1", len(store.hostMetadata))
	}
	if store.GetHostMetadata("h1") == nil {
		t.Error("Load did not load host metadata")
	}
}

func TestDockerMetadataStore_Load_Legacy(t *testing.T) {
	tmpDir := t.TempDir()

	// Write legacy format file (flat map of container metadata)
	legacyData := map[string]*DockerMetadata{
		"c1": {ID: "c1", CustomURL: "url1"},
		"c2": {ID: "c2", CustomURL: "url2"},
	}
	data, _ := json.Marshal(legacyData)
	filePath := filepath.Join(tmpDir, "docker_metadata.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}
	err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify containers loaded from legacy format
	if len(store.metadata) != 2 {
		t.Errorf("Container count = %d, want 2", len(store.metadata))
	}

	// Verify host metadata initialized empty
	if store.hostMetadata == nil {
		t.Error("Host metadata should be initialized")
	}
}

func TestDockerMetadataStore_Load_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Load from nonexistent file should not error
	err := store.Load()
	if err != nil {
		t.Errorf("Load from nonexistent file should not error: %v", err)
	}
}

func TestDockerMetadataStore_Load_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid JSON
	filePath := filepath.Join(tmpDir, "docker_metadata.json")
	if err := os.WriteFile(filePath, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	err := store.Load()
	if err == nil {
		t.Error("Load with invalid JSON should return error")
	}
}

func TestDockerMetadataStore_Save_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "nested", "dir")

	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     subDir,
	}

	// Set should create directory and save
	err := store.Set("id1", &DockerMetadata{CustomURL: "url1"})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(subDir, "docker_metadata.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Save should create directory and file")
	}
}

func TestDockerMetadataStore_Save_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Set some data
	err := store.Set("id1", &DockerMetadata{CustomURL: "url1"})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify no temp file left behind
	tempFile := filepath.Join(tmpDir, "docker_metadata.json.tmp")
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Error("Temp file should be removed after successful save")
	}
}

func TestDockerMetadataStore_VersionedFormat_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store and add data
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Add container metadata
	err := store.Set("host1:container:abc", &DockerMetadata{
		CustomURL:   "http://container.local",
		Description: "Test container",
		Tags:        []string{"prod", "web"},
	})
	if err != nil {
		t.Fatalf("Set container metadata failed: %v", err)
	}

	// Add host metadata
	err = store.SetHostMetadata("host1", &DockerHostMetadata{
		CustomDisplayName: "Production Host",
	})
	if err != nil {
		t.Fatalf("Set host metadata failed: %v", err)
	}

	// Create new store and load
	store2 := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}
	err = store2.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify container metadata
	container := store2.Get("host1:container:abc")
	if container == nil {
		t.Fatal("Container metadata not loaded")
	}
	if container.CustomURL != "http://container.local" {
		t.Errorf("CustomURL = %q, want %q", container.CustomURL, "http://container.local")
	}
	if container.Description != "Test container" {
		t.Errorf("Description = %q, want %q", container.Description, "Test container")
	}
	if len(container.Tags) != 2 || container.Tags[0] != "prod" || container.Tags[1] != "web" {
		t.Errorf("Tags = %v, want [prod web]", container.Tags)
	}

	// Verify host metadata
	host := store2.GetHostMetadata("host1")
	if host == nil {
		t.Fatal("Host metadata not loaded")
	}
	if host.CustomDisplayName != "Production Host" {
		t.Errorf("CustomDisplayName = %q, want %q", host.CustomDisplayName, "Production Host")
	}
}

func TestDockerMetadata_Fields(t *testing.T) {
	meta := DockerMetadata{
		ID:          "host:container:id123",
		CustomURL:   "http://app.local:8080",
		Description: "My application container",
		Tags:        []string{"production", "frontend", "web"},
	}

	if meta.ID != "host:container:id123" {
		t.Errorf("ID = %q, want %q", meta.ID, "host:container:id123")
	}
	if meta.CustomURL != "http://app.local:8080" {
		t.Errorf("CustomURL = %q, want %q", meta.CustomURL, "http://app.local:8080")
	}
	if meta.Description != "My application container" {
		t.Errorf("Description = %q, want %q", meta.Description, "My application container")
	}
	if len(meta.Tags) != 3 {
		t.Errorf("Tags count = %d, want 3", len(meta.Tags))
	}
}

func TestDockerHostMetadata_Fields(t *testing.T) {
	meta := DockerHostMetadata{
		CustomDisplayName: "My Docker Server",
	}

	if meta.CustomDisplayName != "My Docker Server" {
		t.Errorf("CustomDisplayName = %q, want %q", meta.CustomDisplayName, "My Docker Server")
	}
}

func TestDockerMetadataStore_Load_VersionedWithOnlyContainers(t *testing.T) {
	tmpDir := t.TempDir()

	// Write versioned format with only containers (no hosts key)
	fileData := dockerMetadataFile{
		Containers: map[string]*DockerMetadata{
			"c1": {ID: "c1", CustomURL: "url1"},
		},
	}
	data, _ := json.Marshal(fileData)
	filePath := filepath.Join(tmpDir, "docker_metadata.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}
	err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify containers loaded
	if len(store.metadata) != 1 {
		t.Errorf("Container count = %d, want 1", len(store.metadata))
	}

	// Verify host metadata initialized (not nil)
	if store.hostMetadata == nil {
		t.Error("Host metadata should be initialized even when not in file")
	}
}

func TestDockerMetadataStore_Load_VersionedWithOnlyHosts(t *testing.T) {
	tmpDir := t.TempDir()

	// Write versioned format with only hosts (no containers key)
	fileData := dockerMetadataFile{
		Hosts: map[string]*DockerHostMetadata{
			"h1": {CustomDisplayName: "Host 1"},
		},
	}
	data, _ := json.Marshal(fileData)
	filePath := filepath.Join(tmpDir, "docker_metadata.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}
	err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify hosts loaded
	if len(store.hostMetadata) != 1 {
		t.Errorf("Host count = %d, want 1", len(store.hostMetadata))
	}

	// Verify container metadata initialized (not nil)
	if store.metadata == nil {
		t.Error("Container metadata should be initialized even when not in file")
	}
}

func TestDockerMetadataStore_Set_UpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Set initial
	err := store.Set("id1", &DockerMetadata{CustomURL: "url1", Description: "desc1"})
	if err != nil {
		t.Fatalf("Initial Set failed: %v", err)
	}

	// Update
	err = store.Set("id1", &DockerMetadata{CustomURL: "url2", Description: "desc2"})
	if err != nil {
		t.Fatalf("Update Set failed: %v", err)
	}

	// Verify updated
	meta := store.Get("id1")
	if meta.CustomURL != "url2" {
		t.Errorf("CustomURL = %q, want %q", meta.CustomURL, "url2")
	}
	if meta.Description != "desc2" {
		t.Errorf("Description = %q, want %q", meta.Description, "desc2")
	}
}

func TestDockerMetadataStore_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     tmpDir,
	}

	// Pre-populate
	store.metadata["id1"] = &DockerMetadata{ID: "id1", CustomURL: "url1"}
	store.hostMetadata["host1"] = &DockerHostMetadata{CustomDisplayName: "Host 1"}

	// Concurrent reads should not panic
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			store.Get("id1")
			store.GetAll()
			store.GetHostMetadata("host1")
			store.GetAllHostMetadata()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
