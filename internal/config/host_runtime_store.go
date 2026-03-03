package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

const hostRuntimeFileName = "host_runtime_hosts.json"

// HostRuntimeStore persists the last known host-agent state so hosts remain
// visible across server restarts, even while agents are offline.
type HostRuntimeStore struct {
	mu       sync.RWMutex
	hosts    map[string]models.Host // keyed by host ID
	dataPath string
	fs       FileSystem
}

// NewHostRuntimeStore creates a host runtime store and loads existing data.
func NewHostRuntimeStore(dataPath string, fs FileSystem) *HostRuntimeStore {
	store := &HostRuntimeStore{
		hosts:    make(map[string]models.Host),
		dataPath: dataPath,
		fs:       fs,
	}

	if store.fs == nil {
		store.fs = defaultFileSystem{}
	}

	if err := store.Load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load host runtime store")
	}

	return store
}

// Load reads host runtime data from disk.
func (s *HostRuntimeStore) Load() error {
	filePath := filepath.Join(s.dataPath, hostRuntimeFileName)

	data, err := s.fs.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read host runtime file: %w", err)
	}

	var decoded map[string]models.Host
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("failed to decode host runtime file: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.hosts = make(map[string]models.Host, len(decoded))
	for hostID, host := range decoded {
		if hostID == "" {
			continue
		}
		host.ID = hostID
		s.hosts[hostID] = host
	}

	return nil
}

// GetAll returns a copy of all persisted hosts keyed by ID.
func (s *HostRuntimeStore) GetAll() map[string]models.Host {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]models.Host, len(s.hosts))
	for hostID, host := range s.hosts {
		result[hostID] = host
	}
	return result
}

// Upsert inserts or updates a persisted host entry.
func (s *HostRuntimeStore) Upsert(host models.Host) error {
	hostID := host.ID
	if hostID == "" {
		return fmt.Errorf("host id is required")
	}

	host.ID = hostID

	s.mu.Lock()
	defer s.mu.Unlock()

	s.hosts[hostID] = host
	return s.saveLocked()
}

// Delete removes a persisted host entry.
func (s *HostRuntimeStore) Delete(hostID string) error {
	if hostID == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.hosts, hostID)
	return s.saveLocked()
}

// Clear removes all persisted host runtime entries.
func (s *HostRuntimeStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hosts = make(map[string]models.Host)
	return s.saveLocked()
}

func (s *HostRuntimeStore) saveLocked() error {
	filePath := filepath.Join(s.dataPath, hostRuntimeFileName)

	data, err := json.Marshal(s.hosts)
	if err != nil {
		return fmt.Errorf("failed to encode host runtime file: %w", err)
	}

	if err := s.fs.MkdirAll(s.dataPath, 0755); err != nil {
		return fmt.Errorf("failed to ensure host runtime directory: %w", err)
	}

	tempPath := filePath + ".tmp"
	if err := s.fs.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write host runtime temp file: %w", err)
	}

	if err := s.fs.Rename(tempPath, filePath); err != nil {
		return fmt.Errorf("failed to replace host runtime file: %w", err)
	}

	return nil
}
