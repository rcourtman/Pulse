package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	dockerReportOrderFileName         = "docker_report_order.json"
	maxDockerReportOrderFileSizeBytes = 4 * 1024 * 1024
)

// DockerReportOrderEntry is the compact durable source-order watermark for one
// Docker/Podman reporting host. It is deliberately separate from potentially
// large operator-managed Docker metadata because it is updated every report.
type DockerReportOrderEntry struct {
	HostID           string    `json:"hostId"`
	ObservedAt       time.Time `json:"observedAt,omitempty"`
	LastReceivedAt   time.Time `json:"lastReceivedAt,omitempty"`
	StreamID         string    `json:"streamId,omitempty"`
	Sequence         uint64    `json:"sequence,omitempty"`
	RetiredStreamIDs []string  `json:"retiredStreamIds,omitempty"`
}

// DockerReportOrderStore persists Docker report ordering independently from
// telemetry state so delayed buffered reports stay rejected after restart.
type DockerReportOrderStore struct {
	mu       sync.RWMutex
	entries  map[string]DockerReportOrderEntry
	dataPath string
	fs       FileSystem
	loadErr  error
}

func NewDockerReportOrderStore(dataPath string, fs FileSystem) *DockerReportOrderStore {
	store := &DockerReportOrderStore{
		entries:  make(map[string]DockerReportOrderEntry),
		dataPath: dataPath,
		fs:       fs,
	}
	if store.fs == nil {
		store.fs = defaultFileSystem{}
	}
	if err := store.Load(); err != nil {
		store.loadErr = err
		log.Warn().Err(err).Msg("Failed to load Docker report order journal")
	}
	return store
}

func normalizeDockerReportOrderEntry(entry DockerReportOrderEntry) (DockerReportOrderEntry, bool) {
	entry.HostID = strings.TrimSpace(entry.HostID)
	if entry.HostID == "" {
		return DockerReportOrderEntry{}, false
	}
	entry.StreamID = strings.TrimSpace(entry.StreamID)
	entry.RetiredStreamIDs = uniqueTrimmedStrings(entry.RetiredStreamIDs...)
	if len(entry.RetiredStreamIDs) > 8 {
		entry.RetiredStreamIDs = append([]string(nil), entry.RetiredStreamIDs[len(entry.RetiredStreamIDs)-8:]...)
	}
	if entry.StreamID == "" {
		entry.Sequence = 0
	}
	if !entry.ObservedAt.IsZero() {
		entry.ObservedAt = entry.ObservedAt.UTC()
	}
	if !entry.LastReceivedAt.IsZero() {
		entry.LastReceivedAt = entry.LastReceivedAt.UTC()
	}
	return entry, true
}

func (s *DockerReportOrderStore) LoadError() error {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadErr
}

func (s *DockerReportOrderStore) Load() error {
	filePath := filepath.Join(s.dataPath, dockerReportOrderFileName)
	data, err := readLimitedRegularFileFS(s.fs, filePath, maxDockerReportOrderFileSizeBytes)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read Docker report order journal: %w", err)
	}

	entries := make(map[string]DockerReportOrderEntry)
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to unmarshal Docker report order journal: %w", err)
	}
	normalized := make(map[string]DockerReportOrderEntry, len(entries))
	for hostID, entry := range entries {
		if strings.TrimSpace(entry.HostID) == "" {
			entry.HostID = hostID
		}
		if clean, ok := normalizeDockerReportOrderEntry(entry); ok {
			normalized[clean.HostID] = clean
		}
	}

	s.mu.Lock()
	s.entries = normalized
	s.loadErr = nil
	s.mu.Unlock()
	return nil
}

func (s *DockerReportOrderStore) save() error {
	data, err := json.Marshal(s.entries)
	if err != nil {
		return fmt.Errorf("failed to marshal Docker report order journal: %w", err)
	}
	return persistMetadata(s.fs, s.dataPath, dockerReportOrderFileName, data)
}

func (s *DockerReportOrderStore) Get(hostID string) (DockerReportOrderEntry, bool) {
	hostID = strings.TrimSpace(hostID)
	if s == nil || hostID == "" {
		return DockerReportOrderEntry{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[hostID]
	entry.RetiredStreamIDs = append([]string(nil), entry.RetiredStreamIDs...)
	return entry, ok
}

func (s *DockerReportOrderStore) Upsert(entry DockerReportOrderEntry) error {
	normalized, ok := normalizeDockerReportOrderEntry(entry)
	if !ok {
		return fmt.Errorf("Docker report order entry requires host ID")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[string]DockerReportOrderEntry)
	}
	previous, existed := s.entries[normalized.HostID]
	s.entries[normalized.HostID] = normalized
	if err := s.save(); err != nil {
		if existed {
			s.entries[normalized.HostID] = previous
		} else {
			delete(s.entries, normalized.HostID)
		}
		return err
	}
	return nil
}
