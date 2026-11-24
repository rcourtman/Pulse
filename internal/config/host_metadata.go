package config

import (
	"encoding/json"
	"errors"
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

func cloneHostMetadata(meta *HostMetadata) *HostMetadata {
	if meta == nil {
		return nil
	}

	clone := *meta
	if meta.Tags != nil {
		clone.Tags = append([]string{}, meta.Tags...)
	}
	if meta.Notes != nil {
		clone.Notes = append([]string{}, meta.Notes...)
	}
	if meta.CommandsEnabled != nil {
		commandsEnabled := *meta.CommandsEnabled
		clone.CommandsEnabled = &commandsEnabled
	}
	return &clone
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
	data, err := readLimitedRegularFileFS(s.fs, filePath, maxHostMetadataFileSizeBytes)
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

// save writes metadata to disk (must be called with lock held). It refuses a
// file above maxBytes, which is never more than Load accepts.
func (s *HostMetadataStore) save(maxBytes int64) error {
	filePath := filepath.Join(s.dataPath, "host_metadata.json")

	log.Debug().Str("path", filePath).Msg("Saving host metadata to disk")

	data, err := json.Marshal(s.metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := checkMetadataFileSize("host_metadata.json", data, maxBytes); err != nil {
		return err
	}

	if err := persistMetadata(s.fs, s.dataPath, "host_metadata.json", data); err != nil {
		return err
	}

	log.Debug().Str("path", filePath).Int("hosts", len(s.metadata)).Msg("Host metadata saved successfully")

	return nil
}

// Get retrieves metadata for a host
func (s *HostMetadataStore) Get(hostID string) *HostMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if meta, exists := s.metadata[hostID]; exists {
		return cloneHostMetadata(meta)
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
		result[k] = cloneHostMetadata(v)
	}
	return result
}

// Set updates or creates metadata for a host. The metadata routes, open to
// every signed-in user, write through it, so it is refused once the file
// would pass maxHostAnnotationsFileSizeBytes. The room above that is kept for
// SetCommandsEnabled.
func (s *HostMetadataStore) Set(hostID string, meta *HostMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if meta == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	clone := cloneHostMetadata(meta)
	clone.ID = hostID

	return s.setAndSave(hostID, clone, maxHostAnnotationsFileSizeBytes)
}

// SetCommandsEnabled sets the command policy of a host (nil = use agent
// default) and keeps the rest of its record. It may use the whole file,
// including the room Set leaves free.
func (s *HostMetadataStore) SetCommandsEnabled(hostID string, commandsEnabled *bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	clone := cloneHostMetadata(s.metadata[hostID])
	if clone == nil {
		clone = &HostMetadata{}
	}
	clone.ID = hostID
	clone.CommandsEnabled = nil
	if commandsEnabled != nil {
		enabled := *commandsEnabled
		clone.CommandsEnabled = &enabled
	}

	return s.setAndSave(hostID, clone, maxHostMetadataFileSizeBytes)
}

// setAndSave stores one record and saves (must be called with lock held). A
// record the file has no room for is not kept in memory either, or every
// later save would be refused along with it.
func (s *HostMetadataStore) setAndSave(hostID string, meta *HostMetadata, maxBytes int64) error {
	previous, existed := s.metadata[hostID]
	s.metadata[hostID] = meta

	// Save to disk
	if err := s.save(maxBytes); err != nil {
		if errors.Is(err, ErrMetadataFileTooLarge) {
			if existed {
				s.metadata[hostID] = previous
			} else {
				delete(s.metadata, hostID)
			}
		}
		return err
	}
	return nil
}

// Delete removes metadata for a host
func (s *HostMetadataStore) Delete(hostID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.metadata, hostID)

	// Save to disk
	return s.save(maxHostMetadataFileSizeBytes)
}

// ReplaceAll replaces all metadata entries and persists them to disk.
func (s *HostMetadataStore) ReplaceAll(metadata map[string]*HostMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous := s.metadata
	s.metadata = make(map[string]*HostMetadata)

	for hostID, meta := range metadata {
		if meta == nil {
			continue
		}

		clone := cloneHostMetadata(meta)
		clone.ID = hostID
		// Ensure slice copy is not nil to allow JSON marshalling of empty tags
		if clone.Tags == nil {
			clone.Tags = []string{}
		}
		s.metadata[hostID] = clone
	}

	if err := s.save(maxHostMetadataFileSizeBytes); err != nil {
		if errors.Is(err, ErrMetadataFileTooLarge) {
			s.metadata = previous
		}
		return err
	}
	return nil
}
