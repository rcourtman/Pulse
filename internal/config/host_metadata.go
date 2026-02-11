package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"
)

// HostMetadata holds additional metadata for a host
type HostMetadata struct {
	ID              string   `json:"id"`              // Host ID
	CustomURL       string   `json:"customUrl"`       // Custom URL for the host
	Description     string   `json:"description"`     // Optional description
	Tags            []string `json:"tags"`            // Optional tags for categorization
	Notes           []string `json:"notes"`           // User annotations for AI context
	CommandsEnabled *bool    `json:"commandsEnabled"` // Remote override for AI command execution (nil = use agent default)
}

// HostMetadataStore manages host metadata
type HostMetadataStore struct {
	mu       sync.RWMutex
	metadata map[string]*HostMetadata // keyed by host ID
	dataPath string
	fs       FileSystem
}

// NewHostMetadataStore creates a new host metadata store
func NewHostMetadataStore(dataPath string, fs FileSystem) *HostMetadataStore {
	store := &HostMetadataStore{
		metadata: make(map[string]*HostMetadata),
		dataPath: dataPath,
		fs:       fs,
	}

	if store.fs == nil {
		store.fs = defaultFileSystem{}
	}

	// Load existing metadata
	if err := store.Load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load host metadata")
	}

	return store
}

// ... Get/Set/Delete/ReplaceAll ... (unchanged except struct definition)

// Load reads metadata from disk
func (s *HostMetadataStore) Load() error {
	filePath := filepath.Join(s.dataPath, "host_metadata.json")

	log.Debug().Str("path", filePath).Msg("Loading host metadata from disk")

	// Use configured FS
	data, err := s.fs.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, not an error
			log.Debug().Str("path", filePath).Msg("Host metadata file does not exist yet")
			return nil
		}
		return fmt.Errorf("failed to read metadata file: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := json.Unmarshal(data, &s.metadata); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	log.Info().
		Int("hostCount", len(s.metadata)).
		Msg("Loaded host metadata")

	return nil
}

// save writes metadata to disk (must be called with lock held)
func (s *HostMetadataStore) save() error {
	filePath := filepath.Join(s.dataPath, "host_metadata.json")

	log.Debug().Str("path", filePath).Msg("Saving host metadata to disk")

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

	log.Debug().Str("path", filePath).Int("hosts", len(s.metadata)).Msg("Host metadata saved successfully")

	return nil
}

// Get retrieves metadata for a host
func (s *HostMetadataStore) Get(hostID string) *HostMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if meta, exists := s.metadata[hostID]; exists {
		return meta
	}
	return nil
}

// GetAll retrieves all host metadata
func (s *HostMetadataStore) GetAll() map[string]*HostMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modifications
	result := make(map[string]*HostMetadata)
	for k, v := range s.metadata {
		result[k] = v
	}
	return result
}

// Set updates or creates metadata for a host
func (s *HostMetadataStore) Set(hostID string, meta *HostMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if meta == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	meta.ID = hostID
	s.metadata[hostID] = meta

	// Save to disk
	return s.save()
}

// Delete removes metadata for a host
func (s *HostMetadataStore) Delete(hostID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.metadata, hostID)

	// Save to disk
	return s.save()
}

// ReplaceAll replaces all metadata entries and persists them to disk.
func (s *HostMetadataStore) ReplaceAll(metadata map[string]*HostMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metadata = make(map[string]*HostMetadata)

	for hostID, meta := range metadata {
		if meta == nil {
			continue
		}

		clone := *meta
		clone.ID = hostID
		// Ensure slice copy is not nil to allow JSON marshalling of empty tags
		if clone.Tags == nil {
			clone.Tags = []string{}
		}
		s.metadata[hostID] = &clone
	}

	return s.save()
}
