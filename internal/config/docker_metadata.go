package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"
)

// DockerMetadata holds additional metadata for a Docker resource (container/service)
type DockerMetadata struct {
	ID          string   `json:"id"`          // Resource ID (e.g., "hostid:container:containerid" or "hostid:service:serviceid")
	CustomURL   string   `json:"customUrl"`   // Custom URL for the resource
	Description string   `json:"description"` // Optional description
	Tags        []string `json:"tags"`        // Optional tags for categorization
}

// DockerMetadataStore manages Docker resource metadata
type DockerMetadataStore struct {
	mu       sync.RWMutex
	metadata map[string]*DockerMetadata // keyed by resource ID
	dataPath string
}

// NewDockerMetadataStore creates a new metadata store
func NewDockerMetadataStore(dataPath string) *DockerMetadataStore {
	store := &DockerMetadataStore{
		metadata: make(map[string]*DockerMetadata),
		dataPath: dataPath,
	}

	// Load existing metadata
	if err := store.Load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load Docker metadata")
	}

	return store
}

// Get retrieves metadata for a Docker resource
func (s *DockerMetadataStore) Get(resourceID string) *DockerMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if meta, exists := s.metadata[resourceID]; exists {
		return meta
	}
	return nil
}

// GetAll retrieves all Docker resource metadata
func (s *DockerMetadataStore) GetAll() map[string]*DockerMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modifications
	result := make(map[string]*DockerMetadata)
	for k, v := range s.metadata {
		result[k] = v
	}
	return result
}

// Set updates or creates metadata for a Docker resource
func (s *DockerMetadataStore) Set(resourceID string, meta *DockerMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if meta == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	meta.ID = resourceID
	s.metadata[resourceID] = meta

	// Save to disk
	return s.save()
}

// Delete removes metadata for a Docker resource
func (s *DockerMetadataStore) Delete(resourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.metadata, resourceID)

	// Save to disk
	return s.save()
}

// ReplaceAll replaces all metadata entries and persists them to disk.
func (s *DockerMetadataStore) ReplaceAll(metadata map[string]*DockerMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metadata = make(map[string]*DockerMetadata)

	if metadata != nil {
		for resourceID, meta := range metadata {
			if meta == nil {
				continue
			}

			clone := *meta
			clone.ID = resourceID
			// Ensure slice copy is not nil to allow JSON marshalling of empty tags
			if clone.Tags == nil {
				clone.Tags = []string{}
			}
			s.metadata[resourceID] = &clone
		}
	}

	return s.save()
}

// Load reads metadata from disk
func (s *DockerMetadataStore) Load() error {
	filePath := filepath.Join(s.dataPath, "docker_metadata.json")

	log.Debug().Str("path", filePath).Msg("Loading Docker metadata from disk")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, not an error
			log.Debug().Str("path", filePath).Msg("Docker metadata file does not exist yet")
			return nil
		}
		return fmt.Errorf("failed to read metadata file: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := json.Unmarshal(data, &s.metadata); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	log.Info().Int("count", len(s.metadata)).Msg("Loaded Docker metadata")
	return nil
}

// save writes metadata to disk (must be called with lock held)
func (s *DockerMetadataStore) save() error {
	filePath := filepath.Join(s.dataPath, "docker_metadata.json")

	log.Debug().Str("path", filePath).Msg("Saving Docker metadata to disk")

	data, err := json.Marshal(s.metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(s.dataPath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Write to temp file first for atomic operation
	tempFile := filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	// Rename temp file to actual file (atomic on most systems)
	if err := os.Rename(tempFile, filePath); err != nil {
		return fmt.Errorf("failed to rename metadata file: %w", err)
	}

	log.Debug().Str("path", filePath).Int("entries", len(s.metadata)).Msg("Docker metadata saved successfully")

	return nil
}
