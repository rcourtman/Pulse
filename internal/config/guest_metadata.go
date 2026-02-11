package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"
)

// GuestMetadata holds additional metadata for a guest (VM/container)
type GuestMetadata struct {
	ID          string   `json:"id"`          // Guest ID (e.g., "node:vmid" format)
	CustomURL   string   `json:"customUrl"`   // Custom URL for the guest
	Description string   `json:"description"` // Optional description
	Tags        []string `json:"tags"`        // Optional tags for categorization
	Notes       []string `json:"notes"`       // User annotations for AI context (e.g., "Runs PBS in Docker")
	// Last-known identity (persisted even after guest deletion)
	LastKnownName string `json:"lastKnownName,omitempty"` // Last known guest name
	LastKnownType string `json:"lastKnownType,omitempty"` // Last known guest type (qemu, lxc)
}

// GuestMetadataStore manages guest metadata
type GuestMetadataStore struct {
	mu       sync.RWMutex
	metadata map[string]*GuestMetadata // keyed by guest ID
	dataPath string
	fs       FileSystem
}

// NewGuestMetadataStore creates a new metadata store
func NewGuestMetadataStore(dataPath string, fs FileSystem) *GuestMetadataStore {
	store := &GuestMetadataStore{
		metadata: make(map[string]*GuestMetadata),
		dataPath: dataPath,
		fs:       fs,
	}

	if store.fs == nil {
		store.fs = defaultFileSystem{}
	}

	// Load existing metadata
	if err := store.Load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load guest metadata")
	}

	return store
}

// ... Get/GetWithLegacyMigration/GetAll/Set/Delete/ReplaceAll ... (unchanged)

// Load reads metadata from disk
func (s *GuestMetadataStore) Load() error {
	filePath := filepath.Join(s.dataPath, "guest_metadata.json")

	log.Debug().Str("path", filePath).Msg("Loading guest metadata from disk")

	data, err := s.fs.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, not an error
			log.Debug().Str("path", filePath).Msg("Guest metadata file does not exist yet")
			return nil
		}
		return fmt.Errorf("failed to read metadata file: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := json.Unmarshal(data, &s.metadata); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	log.Info().Int("count", len(s.metadata)).Msg("Loaded guest metadata")
	return nil
}

// save writes metadata to disk (must be called with lock held)
func (s *GuestMetadataStore) save() error {
	filePath := filepath.Join(s.dataPath, "guest_metadata.json")

	log.Debug().Str("path", filePath).Msg("Saving guest metadata to disk")

	data, err := json.Marshal(s.metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Restrict metadata persistence to owner-only access.
	if err := s.fs.MkdirAll(s.dataPath, 0o700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Write to temp file first for atomic operation
	tempFile := filePath + ".tmp"
	if err := s.fs.WriteFile(tempFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	// Rename temp file to actual file (atomic on most systems)
	if err := s.fs.Rename(tempFile, filePath); err != nil {
		return fmt.Errorf("failed to rename metadata file: %w", err)
	}

	log.Debug().Str("path", filePath).Int("entries", len(s.metadata)).Msg("Guest metadata saved successfully")

	return nil
}

// Get retrieves metadata for a guest
func (s *GuestMetadataStore) Get(guestID string) *GuestMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if meta, exists := s.metadata[guestID]; exists {
		return meta
	}
	return nil
}

// GetWithLegacyMigration retrieves metadata for a guest, attempting legacy ID formats if needed
// and migrating them to the new stable format (instance:node:vmid).
// This should be called when full guest context (instance, node, vmid) is available.
//
// Legacy formats attempted (in order):
// 1. instance-node-vmid (e.g., "delly-minipc-201") - most specific legacy format
// 2. instance-vmid (e.g., "delly-201") - old cluster format without node
// 3. node-vmid (e.g., "minipc-201") - standalone format
func (s *GuestMetadataStore) GetWithLegacyMigration(guestID, instance, node string, vmid int) *GuestMetadata {
	s.mu.RLock()
	meta, exists := s.metadata[guestID]
	s.mu.RUnlock()

	if exists {
		return meta
	}

	// Helper to migrate a legacy ID to the new format
	migrate := func(legacyID string) *GuestMetadata {
		s.mu.RLock()
		legacyMeta := s.metadata[legacyID]
		s.mu.RUnlock()

		if legacyMeta != nil {
			log.Info().
				Str("legacyID", legacyID).
				Str("newID", guestID).
				Msg("Migrating guest metadata from legacy ID format")

			s.mu.Lock()
			// Move to new ID
			s.metadata[guestID] = legacyMeta
			legacyMeta.ID = guestID
			delete(s.metadata, legacyID)
			// Save asynchronously
			go func() {
				if err := s.save(); err != nil {
					log.Error().Err(err).Msg("Failed to save guest metadata after migration")
				}
			}()
			s.mu.Unlock()

			return legacyMeta
		}
		return nil
	}

	// Try legacy format 1: instance-node-vmid (most specific)
	if instance != node {
		if result := migrate(fmt.Sprintf("%s-%s-%d", instance, node, vmid)); result != nil {
			return result
		}
	}

	// Try legacy format 2: instance-vmid (old cluster format)
	// This was used when cluster name was used without node differentiation
	if result := migrate(fmt.Sprintf("%s-%d", instance, vmid)); result != nil {
		return result
	}

	// Try legacy format 3: node-vmid (standalone format or node-only reference)
	// Only try if instance != node to avoid duplicate check
	if instance != node {
		if result := migrate(fmt.Sprintf("%s-%d", node, vmid)); result != nil {
			return result
		}
	}

	return nil
}

// GetAll retrieves all guest metadata
func (s *GuestMetadataStore) GetAll() map[string]*GuestMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modifications
	result := make(map[string]*GuestMetadata)
	for k, v := range s.metadata {
		result[k] = v
	}
	return result
}

// Set updates or creates metadata for a guest
func (s *GuestMetadataStore) Set(guestID string, meta *GuestMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if meta == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	meta.ID = guestID
	s.metadata[guestID] = meta

	// Save to disk
	return s.save()
}

// Delete removes metadata for a guest
func (s *GuestMetadataStore) Delete(guestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.metadata, guestID)

	// Save to disk
	return s.save()
}

// ReplaceAll replaces all metadata entries and persists them to disk.
func (s *GuestMetadataStore) ReplaceAll(metadata map[string]*GuestMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metadata = make(map[string]*GuestMetadata)

	for guestID, meta := range metadata {
		if meta == nil {
			continue
		}

		clone := *meta
		clone.ID = guestID
		// Ensure slice copy is not nil to allow JSON marshalling of empty tags
		if clone.Tags == nil {
			clone.Tags = []string{}
		}
		s.metadata[guestID] = &clone
	}

	return s.save()
}
