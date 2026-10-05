package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

	// writeMu serializes disk snapshots with synchronous mutations. Background
	// I/O never holds mu, so a burst can accumulate in one follow-up snapshot.
	// Always acquire writeMu before mu when both locks are needed.
	writeMu           sync.Mutex
	revision          uint64
	attemptedRevision uint64
	lastWriteErr      error
	writerDone        chan struct{}
	closed            atomic.Bool
	closeMu           sync.Mutex
	closeAttempt      *guestMetadataCloseAttempt
}

type guestMetadataCloseAttempt struct {
	done chan struct{}
	err  error
}

var ErrGuestMetadataStoreClosed = errors.New("guest metadata store is closed")

// SetAsync admits a detached replacement before returning. One store-owned
// writer persists the newest complete snapshot, coalescing changes that arrive
// during I/O. It does not start a goroutine or rewrite the file per guest.
func (s *GuestMetadataStore) SetAsync(guestID string, meta *GuestMetadata) error {
	if s == nil {
		return nil
	}
	if meta == nil {
		return fmt.Errorf("metadata cannot be nil")
	}
	clone := cloneGuestMetadata(meta)
	clone.ID = guestID
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrGuestMetadataStoreClosed
	}
	s.metadata[guestID] = clone
	s.queueSaveLocked()
	return nil
}

// RememberIdentity merges only monitor-owned identity fields at admission.
// A background name/type update must never restore an old URL, tag or note
// over a concurrent operator edit. OCI classification cannot be downgraded by
// a later poll lacking the classification evidence.
func (s *GuestMetadataStore) RememberIdentity(guestID, name, guestType string) error {
	if s == nil {
		return nil
	}
	guestType = strings.TrimSpace(guestType)
	if guestType == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrGuestMetadataStoreClosed
	}
	meta := cloneGuestMetadata(s.metadata[guestID])
	if meta == nil {
		meta = &GuestMetadata{ID: guestID, Tags: []string{}}
	}
	if meta.LastKnownType == "oci" {
		guestType = "oci"
	}
	if meta.LastKnownName == name && meta.LastKnownType == guestType && s.lastWriteErr == nil {
		return nil
	}
	meta.LastKnownName = name
	meta.LastKnownType = guestType
	s.metadata[guestID] = meta
	s.queueSaveLocked()
	return nil
}

func (s *GuestMetadataStore) queueSaveLocked() {
	s.revision++
	if s.writerDone != nil {
		return
	}
	s.writerDone = make(chan struct{})
	go s.writePendingSnapshots()
}

func (s *GuestMetadataStore) writePendingSnapshots() {
	for {
		s.writeMu.Lock()
		s.mu.Lock()
		if s.attemptedRevision == s.revision {
			close(s.writerDone)
			s.writerDone = nil
			s.mu.Unlock()
			s.writeMu.Unlock()
			return
		}
		revision := s.revision
		data, err := json.Marshal(s.metadata)
		s.mu.Unlock()
		if err == nil {
			err = persistMetadata(s.fs, s.dataPath, "guest_metadata.json", data)
		}
		s.mu.Lock()
		s.attemptedRevision = revision
		s.lastWriteErr = err
		s.mu.Unlock()
		s.writeMu.Unlock()
		if err != nil {
			// No unbounded automatic retry on a failing filesystem. A later
			// admitted mutation (or ordinary identity poll) can try again.
			log.Error().Err(err).Msg("failed to persist guest metadata snapshot")
		}
	}
}

// WaitForPendingWrites reports quiescence, not persistence success. Producers
// must already be stopped if the caller plans to remove the data directory;
// Close seals admission as well. A timeout never makes teardown safe.
func (s *GuestMetadataStore) WaitForPendingWrites(timeout time.Duration) bool {
	if s == nil {
		return true
	}
	s.mu.RLock()
	done := s.writerDone
	s.mu.RUnlock()
	if done == nil {
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		log.Warn().Dur("timeout", timeout).Msg("timed out waiting for guest metadata writes to drain")
		return false
	}
}

// Close rejects further mutations, drains the owned writer within the supplied
// budget, and reports any final persistence error. Failure keeps the writer
// owned and must stop tenant-directory removal; it is not a successful stop.
func (s *GuestMetadataStore) Close(timeout time.Duration) error {
	if s == nil {
		return nil
	}
	// Seal before waiting for a synchronous mutation's I/O-held memory lock.
	// One owned close observer, not one goroutine per waiter, covers both that
	// mutation and the background writer within the caller's unchanged budget.
	s.closed.Store(true)
	s.closeMu.Lock()
	attempt := s.closeAttempt
	retry := false
	if attempt != nil {
		select {
		case <-attempt.done:
			// A later explicit shutdown/offboarding attempt may flush retained
			// state after the filesystem is repaired. Never reopen admission,
			// retry an in-flight write or spin-retry a failed close on its own.
			if attempt.err != nil {
				attempt = nil
				retry = true
			}
		default:
		}
	}
	if attempt == nil {
		attempt = &guestMetadataCloseAttempt{done: make(chan struct{})}
		s.closeAttempt = attempt
		go func() {
			s.mu.Lock()
			if retry && s.lastWriteErr != nil && s.writerDone == nil {
				s.queueSaveLocked()
			}
			done := s.writerDone
			s.mu.Unlock()
			if done != nil {
				<-done
			}
			s.mu.RLock()
			attempt.err = s.lastWriteErr
			s.mu.RUnlock()
			close(attempt.done)
		}()
	}
	s.closeMu.Unlock()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-attempt.done:
		return attempt.err
	case <-timer.C:
		return fmt.Errorf("guest metadata writes did not drain within %s", timeout)
	}
}

func cloneGuestMetadata(meta *GuestMetadata) *GuestMetadata {
	if meta == nil {
		return nil
	}

	clone := *meta
	if meta.Tags != nil {
		clone.Tags = make([]string, len(meta.Tags))
		copy(clone.Tags, meta.Tags)
	}
	if meta.Notes != nil {
		clone.Notes = make([]string, len(meta.Notes))
		copy(clone.Notes, meta.Notes)
	}
	return &clone
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

// Load reads metadata from disk
func (s *GuestMetadataStore) Load() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrGuestMetadataStoreClosed
	}
	if s.revision != s.attemptedRevision || s.lastWriteErr != nil {
		return fmt.Errorf("cannot reload guest metadata with unpersisted changes")
	}
	filePath := filepath.Join(s.dataPath, "guest_metadata.json")

	log.Debug().Str("path", filePath).Msg("Loading guest metadata from disk")

	data, err := readLimitedRegularFileFS(s.fs, filePath, maxGuestMetadataFileSizeBytes)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, not an error
			log.Debug().Str("path", filePath).Msg("Guest metadata file does not exist yet")
			return nil
		}
		return fmt.Errorf("failed to read metadata file: %w", err)
	}

	if err := json.Unmarshal(data, &s.metadata); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	log.Info().Int("count", len(s.metadata)).Msg("Loaded guest metadata")
	return nil
}

// save writes a synchronous mutation (writeMu and mu must both be held).
func (s *GuestMetadataStore) save() error {
	s.revision++
	data, err := json.Marshal(s.metadata)
	if err == nil {
		err = persistMetadata(s.fs, s.dataPath, "guest_metadata.json", data)
	}
	s.attemptedRevision = s.revision
	s.lastWriteErr = err
	return err
}

// Get retrieves metadata for a guest
func (s *GuestMetadataStore) Get(guestID string) *GuestMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if meta, exists := s.metadata[guestID]; exists {
		return cloneGuestMetadata(meta)
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
func (s *GuestMetadataStore) GetWithLegacyMigration(guestID, instance, node string, vmID int) *GuestMetadata {
	s.mu.RLock()
	meta, exists := s.metadata[guestID]
	s.mu.RUnlock()

	if exists {
		return cloneGuestMetadata(meta)
	}

	// Helper to migrate a legacy ID to the new format
	migrate := func(legacyID string) *GuestMetadata {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed.Load() {
			return nil
		}

		if current := s.metadata[guestID]; current != nil {
			return cloneGuestMetadata(current)
		}
		legacyMeta := s.metadata[legacyID]
		if legacyMeta == nil {
			return cloneGuestMetadata(s.metadata[guestID])
		}

		log.Info().
			Str("legacyID", legacyID).
			Str("newID", guestID).
			Msg("Migrating guest metadata from legacy ID format")

		migrated := cloneGuestMetadata(legacyMeta)
		migrated.ID = guestID
		s.metadata[guestID] = migrated
		delete(s.metadata, legacyID)
		// Persist while holding the store lock. save() assumes locked access.
		if err := s.save(); err != nil {
			log.Error().Err(err).Msg("Failed to save guest metadata after migration")
			s.metadata[legacyID] = legacyMeta
			delete(s.metadata, guestID)
		}

		return cloneGuestMetadata(migrated)
	}

	// Try legacy format 1: instance-node-vmid (most specific)
	if instance != node {
		if result := migrate(fmt.Sprintf("%s-%s-%d", instance, node, vmID)); result != nil {
			return result
		}
	}

	// Try legacy format 2: instance-vmid (old cluster format)
	// This was used when cluster name was used without node differentiation
	if result := migrate(fmt.Sprintf("%s-%d", instance, vmID)); result != nil {
		return result
	}

	// Try legacy format 3: node-vmid (standalone format or node-only reference)
	// Only try if instance != node to avoid duplicate check
	if instance != node {
		if result := migrate(fmt.Sprintf("%s-%d", node, vmID)); result != nil {
			return result
		}
	}

	// A guest that migrated to another cluster node keeps its VMID but shows
	// up under a new node-scoped ID, orphaning its metadata (#1669). VMIDs
	// are unique within a Proxmox cluster, so an entry for the same instance
	// and VMID on a different node belongs to this guest.
	if movedID := s.findGuestIDOnOtherNode(guestID, instance, vmID); movedID != "" {
		if result := migrate(movedID); result != nil {
			return result
		}
	}

	return nil
}

// findGuestIDOnOtherNode returns a stored metadata ID for the same instance
// and VMID under a different node, or "" when none exists.
func (s *GuestMetadataStore) findGuestIDOnOtherNode(guestID, instance string, vmID int) string {
	prefix := instance + ":"
	suffix := fmt.Sprintf(":%d", vmID)

	s.mu.RLock()
	defer s.mu.RUnlock()
	for id := range s.metadata {
		if id == guestID || len(id) <= len(prefix)+len(suffix) {
			continue
		}
		if !strings.HasPrefix(id, prefix) || !strings.HasSuffix(id, suffix) {
			continue
		}
		// The middle segment must be exactly one node name: an ID with more
		// separators is a different key shape, not a node-scoped guest ID.
		middle := id[len(prefix) : len(id)-len(suffix)]
		if middle == "" || strings.Contains(middle, ":") {
			continue
		}
		return id
	}
	return ""
}

// GetAll retrieves all guest metadata
func (s *GuestMetadataStore) GetAll() map[string]*GuestMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modifications
	result := make(map[string]*GuestMetadata)
	for k, v := range s.metadata {
		result[k] = cloneGuestMetadata(v)
	}
	return result
}

// Set updates or creates metadata for a guest
func (s *GuestMetadataStore) Set(guestID string, meta *GuestMetadata) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrGuestMetadataStoreClosed
	}

	if meta == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	clone := cloneGuestMetadata(meta)
	clone.ID = guestID
	s.metadata[guestID] = clone

	// Save to disk
	return s.save()
}

// Delete removes metadata for a guest
func (s *GuestMetadataStore) Delete(guestID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrGuestMetadataStoreClosed
	}

	delete(s.metadata, guestID)

	// Save to disk
	return s.save()
}

// ReplaceAll replaces all metadata entries and persists them to disk.
func (s *GuestMetadataStore) ReplaceAll(metadata map[string]*GuestMetadata) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrGuestMetadataStoreClosed
	}

	s.metadata = make(map[string]*GuestMetadata)

	for guestID, meta := range metadata {
		if meta == nil {
			continue
		}

		clone := cloneGuestMetadata(meta)
		clone.ID = guestID
		// Ensure slice copy is not nil to allow JSON marshalling of empty tags
		if clone.Tags == nil {
			clone.Tags = []string{}
		}
		s.metadata[guestID] = clone
	}

	return s.save()
}

// UpdateAll applies one atomic in-memory mutation and persists it once.
// The callback receives a deep-cloned working set and returns whether it
// changed. Concurrent Set/Delete calls cannot be lost between snapshot and
// replacement.
func (s *GuestMetadataStore) UpdateAll(
	update func(map[string]*GuestMetadata) bool,
) error {
	if update == nil {
		return nil
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return ErrGuestMetadataStoreClosed
	}

	working := make(map[string]*GuestMetadata, len(s.metadata))
	for id, meta := range s.metadata {
		working[id] = cloneGuestMetadata(meta)
	}
	if !update(working) {
		return nil
	}
	for id, meta := range working {
		if meta == nil {
			delete(working, id)
			continue
		}
		meta.ID = id
		if meta.Tags == nil {
			meta.Tags = []string{}
		}
	}

	previous := s.metadata
	s.metadata = working
	if err := s.save(); err != nil {
		s.metadata = previous
		return err
	}
	return nil
}
