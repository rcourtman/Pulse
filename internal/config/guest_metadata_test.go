package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestGuestMetadataStore_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Test get on empty store
	result := store.Get("nonexistent")
	if result != nil {
		t.Error("Get on empty store should return nil")
	}

	// Add metadata
	store.metadata["pve1:node1:100"] = &GuestMetadata{
		ID:            "pve1:node1:100",
		CustomURL:     "http://example.com",
		Description:   "Test VM",
		Tags:          []string{"tag1", "tag2"},
		LastKnownName: "test-vm",
		LastKnownType: "qemu",
	}

	// Test get existing
	result = store.Get("pve1:node1:100")
	if result == nil {
		t.Fatal("Get should return metadata for existing entry")
	}
	if result.CustomURL != "http://example.com" {
		t.Errorf("CustomURL = %q, want %q", result.CustomURL, "http://example.com")
	}
	if result.Description != "Test VM" {
		t.Errorf("Description = %q, want %q", result.Description, "Test VM")
	}
	if len(result.Tags) != 2 {
		t.Errorf("Tags count = %d, want 2", len(result.Tags))
	}
	if result.LastKnownName != "test-vm" {
		t.Errorf("LastKnownName = %q, want %q", result.LastKnownName, "test-vm")
	}
	if result.LastKnownType != "qemu" {
		t.Errorf("LastKnownType = %q, want %q", result.LastKnownType, "qemu")
	}
}

func TestGuestMetadataStore_GetAll(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Test empty store
	all := store.GetAll()
	if len(all) != 0 {
		t.Errorf("GetAll on empty store returned %d entries, want 0", len(all))
	}

	// Add metadata
	store.metadata["id1"] = &GuestMetadata{ID: "id1", CustomURL: "url1"}
	store.metadata["id2"] = &GuestMetadata{ID: "id2", CustomURL: "url2"}

	// Test GetAll returns all entries
	all = store.GetAll()
	if len(all) != 2 {
		t.Errorf("GetAll returned %d entries, want 2", len(all))
	}

	// Verify it's a copy (modification shouldn't affect store)
	all["id3"] = &GuestMetadata{ID: "id3"}
	if len(store.metadata) != 2 {
		t.Error("GetAll should return a copy, not the original map")
	}
}

func TestGuestMetadataStore_Set(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Test set nil
	err := store.Set("id1", nil)
	if err == nil {
		t.Error("Set with nil metadata should return error")
	}

	// Test successful set
	meta := &GuestMetadata{
		CustomURL:     "http://test.com",
		Description:   "Test desc",
		Tags:          []string{"tag1"},
		LastKnownName: "vm1",
		LastKnownType: "lxc",
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
	if stored.LastKnownName != "vm1" {
		t.Errorf("Stored LastKnownName = %q, want %q", stored.LastKnownName, "vm1")
	}

	// Verify persisted to disk
	filePath := filepath.Join(tmpDir, "guest_metadata.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read persisted file: %v", err)
	}

	var fileData map[string]*GuestMetadata
	if err := json.Unmarshal(data, &fileData); err != nil {
		t.Fatalf("Failed to unmarshal persisted data: %v", err)
	}
	if fileData["id1"] == nil {
		t.Error("Metadata not persisted")
	}
}

func TestGuestMetadataStore_Set_UpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Set initial
	err := store.Set("id1", &GuestMetadata{CustomURL: "url1", Description: "desc1"})
	if err != nil {
		t.Fatalf("Initial Set failed: %v", err)
	}

	// Update
	err = store.Set("id1", &GuestMetadata{CustomURL: "url2", Description: "desc2"})
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

func TestGuestMetadataStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Add metadata
	store.metadata["id1"] = &GuestMetadata{ID: "id1", CustomURL: "url1"}

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

func TestGuestMetadataStore_ReplaceAll(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Add initial data
	store.metadata["old1"] = &GuestMetadata{ID: "old1"}

	// Replace all
	newData := map[string]*GuestMetadata{
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

func TestGuestMetadataStore_ReplaceAll_NilEntry(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Replace with map containing nil entry
	newData := map[string]*GuestMetadata{
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

func TestGuestMetadataStore_Load(t *testing.T) {
	tmpDir := t.TempDir()

	// Write test file
	fileData := map[string]*GuestMetadata{
		"pve1:node1:100": {
			ID:            "pve1:node1:100",
			CustomURL:     "http://vm1.local",
			Description:   "Production VM",
			Tags:          []string{"prod", "web"},
			LastKnownName: "web-server",
			LastKnownType: "qemu",
		},
		"pve1:node1:101": {
			ID:          "pve1:node1:101",
			CustomURL:   "http://ct1.local",
			Description: "Dev container",
		},
	}
	data, _ := json.Marshal(fileData)
	filePath := filepath.Join(tmpDir, "guest_metadata.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}
	err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify loaded
	if len(store.metadata) != 2 {
		t.Errorf("Metadata count = %d, want 2", len(store.metadata))
	}

	meta := store.Get("pve1:node1:100")
	if meta == nil {
		t.Fatal("Load did not load metadata")
	}
	if meta.CustomURL != "http://vm1.local" {
		t.Errorf("CustomURL = %q, want %q", meta.CustomURL, "http://vm1.local")
	}
	if meta.LastKnownName != "web-server" {
		t.Errorf("LastKnownName = %q, want %q", meta.LastKnownName, "web-server")
	}
}

func TestGuestMetadataStore_Load_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Load from nonexistent file should not error
	err := store.Load()
	if err != nil {
		t.Errorf("Load from nonexistent file should not error: %v", err)
	}
}

func TestGuestMetadataStore_Load_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid JSON
	filePath := filepath.Join(tmpDir, "guest_metadata.json")
	if err := os.WriteFile(filePath, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	err := store.Load()
	if err == nil {
		t.Error("Load with invalid JSON should return error")
	}
}

func TestGuestMetadataStore_Save_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "nested", "dir")

	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: subDir,
	}

	// Set should create directory and save
	err := store.Set("id1", &GuestMetadata{CustomURL: "url1"})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(subDir, "guest_metadata.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Save should create directory and file")
	}
}

func TestGuestMetadataStore_Save_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Set some data
	err := store.Set("id1", &GuestMetadata{CustomURL: "url1"})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify no temp file left behind
	tempFile := filepath.Join(tmpDir, "guest_metadata.json.tmp")
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Error("Temp file should be removed after successful save")
	}
}

func TestGuestMetadataStore_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store and add data
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	err := store.Set("pve1:node1:100", &GuestMetadata{
		CustomURL:     "http://vm.local",
		Description:   "Test VM",
		Tags:          []string{"prod", "web"},
		LastKnownName: "webserver",
		LastKnownType: "qemu",
	})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Create new store and load
	store2 := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}
	err = store2.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify metadata
	meta := store2.Get("pve1:node1:100")
	if meta == nil {
		t.Fatal("Metadata not loaded")
	}
	if meta.CustomURL != "http://vm.local" {
		t.Errorf("CustomURL = %q, want %q", meta.CustomURL, "http://vm.local")
	}
	if meta.Description != "Test VM" {
		t.Errorf("Description = %q, want %q", meta.Description, "Test VM")
	}
	if len(meta.Tags) != 2 || meta.Tags[0] != "prod" || meta.Tags[1] != "web" {
		t.Errorf("Tags = %v, want [prod web]", meta.Tags)
	}
	if meta.LastKnownName != "webserver" {
		t.Errorf("LastKnownName = %q, want %q", meta.LastKnownName, "webserver")
	}
	if meta.LastKnownType != "qemu" {
		t.Errorf("LastKnownType = %q, want %q", meta.LastKnownType, "qemu")
	}
}

func TestGuestMetadata_Fields(t *testing.T) {
	meta := GuestMetadata{
		ID:            "pve1:node1:100",
		CustomURL:     "http://app.local:8080",
		Description:   "My virtual machine",
		Tags:          []string{"production", "database"},
		LastKnownName: "db-server",
		LastKnownType: "qemu",
	}

	if meta.ID != "pve1:node1:100" {
		t.Errorf("ID = %q, want %q", meta.ID, "pve1:node1:100")
	}
	if meta.CustomURL != "http://app.local:8080" {
		t.Errorf("CustomURL = %q, want %q", meta.CustomURL, "http://app.local:8080")
	}
	if meta.Description != "My virtual machine" {
		t.Errorf("Description = %q, want %q", meta.Description, "My virtual machine")
	}
	if len(meta.Tags) != 2 {
		t.Errorf("Tags count = %d, want 2", len(meta.Tags))
	}
	if meta.LastKnownName != "db-server" {
		t.Errorf("LastKnownName = %q, want %q", meta.LastKnownName, "db-server")
	}
	if meta.LastKnownType != "qemu" {
		t.Errorf("LastKnownType = %q, want %q", meta.LastKnownType, "qemu")
	}
}

func TestGuestMetadataStore_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Pre-populate
	store.metadata["id1"] = &GuestMetadata{ID: "id1", CustomURL: "url1"}

	// Concurrent reads should not panic
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			store.Get("id1")
			store.GetAll()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestGuestMetadataStore_GetWithLegacyMigration_ExistingNewFormat(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Add metadata with new format ID
	store.metadata["pve1:node1:100"] = &GuestMetadata{
		ID:        "pve1:node1:100",
		CustomURL: "http://example.com",
	}

	// Get with new format should return directly
	result := store.GetWithLegacyMigration("pve1:node1:100", "pve1", "node1", 100)
	if result == nil {
		t.Fatal("Should return metadata for existing new format ID")
	}
	if result.CustomURL != "http://example.com" {
		t.Errorf("CustomURL = %q, want %q", result.CustomURL, "http://example.com")
	}
}

func TestGuestMetadataStore_GetWithLegacyMigration_ClusteredLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Add metadata with legacy clustered format: instance-node-VMID
	store.metadata["pve1-node1-100"] = &GuestMetadata{
		ID:        "pve1-node1-100",
		CustomURL: "http://legacy.com",
	}

	// Get with new format should migrate
	result := store.GetWithLegacyMigration("pve1:node1:100", "pve1", "node1", 100)
	if result == nil {
		t.Fatal("Should migrate and return metadata")
	}
	if result.CustomURL != "http://legacy.com" {
		t.Errorf("CustomURL = %q, want %q", result.CustomURL, "http://legacy.com")
	}

	// ID should be updated to new format
	if result.ID != "pve1:node1:100" {
		t.Errorf("ID = %q, want %q", result.ID, "pve1:node1:100")
	}

	// Wait for async save
	time.Sleep(100 * time.Millisecond)

	// Old ID should be removed
	if store.metadata["pve1-node1-100"] != nil {
		t.Error("Legacy ID should be removed after migration")
	}

	// New ID should exist
	if store.metadata["pve1:node1:100"] == nil {
		t.Error("New ID should exist after migration")
	}
}

func TestGuestMetadataStore_GetWithLegacyMigration_StandaloneLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Add metadata with legacy standalone format: node-VMID
	store.metadata["node1-100"] = &GuestMetadata{
		ID:        "node1-100",
		CustomURL: "http://standalone.com",
	}

	// Get with instance == node (standalone) should migrate
	result := store.GetWithLegacyMigration("node1:node1:100", "node1", "node1", 100)
	if result == nil {
		t.Fatal("Should migrate and return metadata")
	}
	if result.CustomURL != "http://standalone.com" {
		t.Errorf("CustomURL = %q, want %q", result.CustomURL, "http://standalone.com")
	}

	// ID should be updated
	if result.ID != "node1:node1:100" {
		t.Errorf("ID = %q, want %q", result.ID, "node1:node1:100")
	}

	// Wait for async save
	time.Sleep(100 * time.Millisecond)

	// Old ID should be removed
	if store.metadata["node1-100"] != nil {
		t.Error("Legacy ID should be removed after migration")
	}
}

func TestGuestMetadataStore_GetWithLegacyMigration_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Get non-existent should return nil
	result := store.GetWithLegacyMigration("pve1:node1:100", "pve1", "node1", 100)
	if result != nil {
		t.Error("Should return nil for non-existent metadata")
	}
}

func TestGuestMetadataStore_GetWithLegacyMigration_ClusteredMatchesNodeFormat(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Add node-vmid format (legacy standalone format)
	store.metadata["node1-100"] = &GuestMetadata{
		ID:        "node1-100",
		CustomURL: "http://standalone.com",
	}

	// Clustered request (instance != node) CAN match node-vmid as fallback
	// This handles cases where metadata was created with old format
	result := store.GetWithLegacyMigration("pve1:node1:100", "pve1", "node1", 100)
	if result == nil {
		t.Fatal("Should migrate from node-vmid format for clustered request")
	}
	if result.CustomURL != "http://standalone.com" {
		t.Errorf("CustomURL = %q, want %q", result.CustomURL, "http://standalone.com")
	}
	if result.ID != "pve1:node1:100" {
		t.Errorf("ID = %q, want %q", result.ID, "pve1:node1:100")
	}

	// Wait for async save to complete before test cleanup
	time.Sleep(100 * time.Millisecond)
}

func TestGuestMetadataStore_GetWithLegacyMigration_ConcurrentMigration(t *testing.T) {
	tmpDir := t.TempDir()
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: tmpDir,
	}

	// Add legacy metadata
	store.metadata["pve1-node1-100"] = &GuestMetadata{
		ID:        "pve1-node1-100",
		CustomURL: "http://legacy.com",
	}

	// Multiple concurrent migrations should not panic
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.GetWithLegacyMigration("pve1:node1:100", "pve1", "node1", 100)
		}()
	}
	wg.Wait()

	// Wait for any async saves
	time.Sleep(200 * time.Millisecond)

	// New ID should exist
	if store.metadata["pve1:node1:100"] == nil {
		t.Error("New ID should exist after migration")
	}
}
