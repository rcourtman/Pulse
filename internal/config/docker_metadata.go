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
	Notes       []string `json:"notes"`       // User annotations for AI context
}

// DockerHostMetadata holds additional metadata for a Docker host
type DockerHostMetadata struct {
	CustomDisplayName string   `json:"customDisplayName,omitempty"` // User-defined custom display name
	CustomURL         string   `json:"customUrl,omitempty"`         // Custom URL for administration (e.g., Portainer)
	Notes             []string `json:"notes,omitempty"`             // User annotations for AI context
}

// dockerMetadataFile represents the on-disk format for Docker metadata
type dockerMetadataFile struct {
	Containers map[string]*DockerMetadata     `json:"containers,omitempty"` // Container/service metadata (legacy: may be top-level)
	Hosts      map[string]*DockerHostMetadata `json:"hosts,omitempty"`      // Host-level metadata
}

// DockerMetadataStore manages Docker resource metadata
type DockerMetadataStore struct {
	mu           sync.RWMutex
	metadata     map[string]*DockerMetadata     // keyed by resource ID (containers/services)
	hostMetadata map[string]*DockerHostMetadata // keyed by host ID
	dataPath     string
	fs           FileSystem
}

// NewDockerMetadataStore creates a new metadata store
func NewDockerMetadataStore(dataPath string, fs FileSystem) *DockerMetadataStore {
	store := &DockerMetadataStore{
		metadata:     make(map[string]*DockerMetadata),
		hostMetadata: make(map[string]*DockerHostMetadata),
		dataPath:     dataPath,
		fs:           fs,
	}

	if store.fs == nil {
		store.fs = defaultFileSystem{}
	}

	// Load existing metadata
	if err := store.Load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load Docker metadata")
	}

	return store
}

// ... Get/GetAll/GetHostMetadata/GetAllHostMetadata/SetHostMetadata/Set/Delete/ReplaceAll ... (unchanged)

// Load reads metadata from disk
func (s *DockerMetadataStore) Load() error {
	filePath := filepath.Join(s.dataPath, "docker_metadata.json")

	log.Debug().Str("path", filePath).Msg("Loading Docker metadata from disk")

	data, err := readLimitedRegularFileFS(s.fs, filePath, maxDockerMetadataFileSizeBytes)
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

	// Try to load as versioned format first
	var fileData dockerMetadataFile
	if err := json.Unmarshal(data, &fileData); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// Check if this is the new format (has "hosts" or "containers" keys)
	if fileData.Hosts != nil || fileData.Containers != nil {
		// New versioned format
		if fileData.Containers != nil {
			s.metadata = fileData.Containers
		} else {
			s.metadata = make(map[string]*DockerMetadata)
		}
		if fileData.Hosts != nil {
			s.hostMetadata = fileData.Hosts
		} else {
			s.hostMetadata = make(map[string]*DockerHostMetadata)
		}
		log.Info().
			Int("containerCount", len(s.metadata)).
			Int("hostCount", len(s.hostMetadata)).
			Msg("Loaded Docker metadata (versioned format)")
	} else {
		// Legacy format: top-level map is container metadata
		if err := json.Unmarshal(data, &s.metadata); err != nil {
			return fmt.Errorf("failed to unmarshal legacy metadata: %w", err)
		}
		s.hostMetadata = make(map[string]*DockerHostMetadata)
		log.Info().
			Int("containerCount", len(s.metadata)).
			Msg("Loaded Docker metadata (legacy format)")
	}

	return nil
}

// save writes metadata to disk (must be called with lock held)
func (s *DockerMetadataStore) save() error {
	filePath := filepath.Join(s.dataPath, "docker_metadata.json")

	log.Debug().Str("path", filePath).Msg("Saving Docker metadata to disk")

	// Use versioned format
	fileData := dockerMetadataFile{
		Containers: s.metadata,
		Hosts:      s.hostMetadata,
	}

	data, err := json.Marshal(fileData)
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

	log.Debug().Str("path", filePath).Int("containers", len(s.metadata)).Int("hosts", len(s.hostMetadata)).Msg("Docker metadata saved successfully")

	return nil
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

// GetHostMetadata retrieves metadata for a Docker host
func (s *DockerMetadataStore) GetHostMetadata(hostID string) *DockerHostMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if meta, exists := s.hostMetadata[hostID]; exists {
		return meta
	}
	return nil
}

// GetAllHostMetadata retrieves all Docker host metadata
func (s *DockerMetadataStore) GetAllHostMetadata() map[string]*DockerHostMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modifications
	result := make(map[string]*DockerHostMetadata)
	for k, v := range s.hostMetadata {
		result[k] = v
	}
	return result
}

// SetHostMetadata updates or creates metadata for a Docker host
func (s *DockerMetadataStore) SetHostMetadata(hostID string, meta *DockerHostMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If metadata is nil or all fields are empty, delete the entry
	if meta == nil || (meta.CustomDisplayName == "" && meta.CustomURL == "" && len(meta.Notes) == 0) {
		delete(s.hostMetadata, hostID)
	} else {
		s.hostMetadata[hostID] = meta
	}

	// Save to disk
	return s.save()

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

	return s.save()
}
