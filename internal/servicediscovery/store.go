package servicediscovery

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rs/zerolog/log"
)

// CryptoManager interface for encryption/decryption.
type CryptoManager interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// Store provides encrypted per-resource storage for discovery data.
type Store struct {
	mu        sync.RWMutex
	dataDir   string
	crypto    CryptoManager
	cache     map[string]*ResourceDiscovery // In-memory cache
	cacheTime map[string]time.Time          // Cache timestamps
	cacheTTL  time.Duration

	// Fingerprint storage (in-memory with file persistence)
	fingerprintDir      string
	fingerprints        map[string]*ContainerFingerprint // resourceID -> fingerprint
	fingerprintMu       sync.RWMutex
	lastFingerprintScan time.Time
}

func cloneResourceDiscovery(src *ResourceDiscovery) *ResourceDiscovery {
	if src == nil {
		return nil
	}

	cloned := *src
	if src.Facts != nil {
		cloned.Facts = append([]DiscoveryFact(nil), src.Facts...)
	}
	if src.ConfigPaths != nil {
		cloned.ConfigPaths = append([]string(nil), src.ConfigPaths...)
	}
	if src.DataPaths != nil {
		cloned.DataPaths = append([]string(nil), src.DataPaths...)
	}
	if src.LogPaths != nil {
		cloned.LogPaths = append([]string(nil), src.LogPaths...)
	}
	if src.Ports != nil {
		cloned.Ports = append([]PortInfo(nil), src.Ports...)
	}
	if src.DockerMounts != nil {
		cloned.DockerMounts = append([]DockerBindMount(nil), src.DockerMounts...)
	}
	if src.UserSecrets != nil {
		cloned.UserSecrets = make(map[string]string, len(src.UserSecrets))
		for k, v := range src.UserSecrets {
			cloned.UserSecrets[k] = v
		}
	}
	if src.RawCommandOutput != nil {
		cloned.RawCommandOutput = make(map[string]string, len(src.RawCommandOutput))
		for k, v := range src.RawCommandOutput {
			cloned.RawCommandOutput[k] = v
		}
	}

	return &cloned
}

func cloneContainerFingerprint(src *ContainerFingerprint) *ContainerFingerprint {
	if src == nil {
		return nil
	}

	cloned := *src
	if src.Ports != nil {
		cloned.Ports = append([]string(nil), src.Ports...)
	}
	if src.MountPaths != nil {
		cloned.MountPaths = append([]string(nil), src.MountPaths...)
	}
	if src.EnvKeys != nil {
		cloned.EnvKeys = append([]string(nil), src.EnvKeys...)
	}

	return &cloned
}

// NormalizeResourceType maps legacy resource type strings to their current values.
func NormalizeResourceType(rt ResourceType) ResourceType {
	switch rt {
	case "lxc":
		return ResourceTypeSystemContainer
	case "docker_lxc":
		return ResourceTypeDockerSystemContainer
	default:
		return rt
	}
}

// normalizeResourceID replaces legacy type prefixes in resource IDs.
func normalizeResourceID(id string) string {
	if strings.HasPrefix(id, "lxc:") {
		return string(ResourceTypeSystemContainer) + id[3:]
	}
	if strings.HasPrefix(id, "docker_lxc:") {
		return string(ResourceTypeDockerSystemContainer) + id[10:]
	}
	return id
}

// normalizeDiscovery normalizes legacy type strings in a loaded discovery.
func normalizeDiscovery(d *ResourceDiscovery) {
	if d == nil {
		return
	}
	d.ResourceType = NormalizeResourceType(d.ResourceType)
	d.ID = normalizeResourceID(d.ID)
}

// toLegacyID converts a normalized resource ID back to its legacy form for file lookup.
func toLegacyID(id string) string {
	if strings.HasPrefix(id, "system-container:") {
		return "lxc" + id[len("system-container"):]
	}
	if strings.HasPrefix(id, "docker_system-container:") {
		return "docker_lxc" + id[len("docker_system-container"):]
	}
	return id
}

// For testing - allows injecting a mock crypto manager
var newCryptoManagerAt = crypto.NewCryptoManagerAt

// For testing - allows injecting a mock marshaler.
var marshalDiscovery = json.Marshal

// File read limits for persisted discovery data.
// These are variables (not constants) so tests can temporarily override them.
var maxDiscoveryFileReadBytes int64 = 16 * 1024 * 1024  // 16 MiB
var maxFingerprintFileReadBytes int64 = 1 * 1024 * 1024 // 1 MiB

// NewStore creates a new discovery store with automatic encryption.
func NewStore(dataDir string) (*Store, error) {
	trimmedDataDir := strings.TrimSpace(dataDir)
	if trimmedDataDir == "" {
		return nil, fmt.Errorf("discovery data directory is required")
	}
	normalizedDataDir := filepath.Clean(trimmedDataDir)

	discoveryDir := filepath.Join(normalizedDataDir, "discovery")
	if err := os.MkdirAll(discoveryDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create discovery directory: %w", err)
	}

	// Create fingerprint subdirectory
	fingerprintDir := filepath.Join(discoveryDir, "fingerprints")
	if err := os.MkdirAll(fingerprintDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create fingerprint directory: %w", err)
	}

	// Initialize crypto manager for encryption (uses same key as other Pulse secrets)
	cryptoMgr, err := newCryptoManagerAt(normalizedDataDir)
	if err != nil {
		log.Warn().Err(err).Msg("failed to initialize crypto for discovery store, data will be unencrypted")
	}

	store := &Store{
		dataDir:        discoveryDir,
		fingerprintDir: fingerprintDir,
		crypto:         cryptoMgr,
		cache:          make(map[string]*ResourceDiscovery),
		cacheTime:      make(map[string]time.Time),
		cacheTTL:       5 * time.Minute,
		fingerprints:   make(map[string]*ContainerFingerprint),
	}

	// Load existing fingerprints from disk
	store.loadFingerprints()

	return store, nil
}

// getFilePath returns the file path for a resource ID.
func (s *Store) getFilePath(id string) string {
	// Sanitize ID for filename: replace : with _
	safeID := strings.ReplaceAll(id, ":", "_")
	safeID = strings.ReplaceAll(safeID, "/", "_")
	return filepath.Join(s.dataDir, safeID+".enc")
}

// readRegularFileWithLimit reads a file with a strict size cap and rejects non-regular files.
func readRegularFileWithLimit(path string, maxBytes int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return nil, fmt.Errorf("file exceeds max size (%d bytes)", maxBytes)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	openedInfo, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !openedInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	if maxBytes > 0 && openedInfo.Size() > maxBytes {
		return nil, fmt.Errorf("file exceeds max size (%d bytes)", maxBytes)
	}

	if maxBytes <= 0 {
		return io.ReadAll(f)
	}

	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("file exceeds max size (%d bytes)", maxBytes)
	}
	return data, nil
}

// Save persists a discovery to encrypted storage.
func (s *Store) Save(d *ResourceDiscovery) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d.ID == "" {
		return fmt.Errorf("discovery ID is required")
	}

	// Update timestamp
	d.UpdatedAt = time.Now()
	if d.DiscoveredAt.IsZero() {
		d.DiscoveredAt = d.UpdatedAt
	}

	// Persist/cache a defensive copy so callers cannot mutate shared state after Save.
	toSave := cloneResourceDiscovery(d)

	data, err := marshalDiscovery(toSave)
	if err != nil {
		return fmt.Errorf("failed to marshal discovery: %w", err)
	}

	// Encrypt if crypto is available
	if s.crypto != nil {
		encrypted, err := s.crypto.Encrypt(data)
		if err != nil {
			return fmt.Errorf("failed to encrypt discovery: %w", err)
		}
		data = encrypted
	}

	// Write atomically using tmp file + rename
	filePath := s.getFilePath(d.ID)
	tmpPath := filePath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write discovery file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		if cleanupErr := os.Remove(tmpPath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			log.Warn().Err(cleanupErr).Str("tmp_path", tmpPath).Msg("Failed to remove temp discovery file after rename failure")
		}
		return fmt.Errorf("failed to finalize discovery file: %w", err)
	}

	// Update cache
	s.cache[d.ID] = toSave
	s.cacheTime[d.ID] = time.Now()

	log.Debug().Str("id", d.ID).Str("service", d.ServiceType).Msg("discovery saved")
	return nil
}

// Get retrieves a discovery from storage.
func (s *Store) Get(id string) (*ResourceDiscovery, error) {
	s.mu.RLock()
	// Check cache first
	if cached, ok := s.cache[id]; ok {
		if cacheTime, hasTime := s.cacheTime[id]; hasTime {
			if time.Since(cacheTime) < s.cacheTTL {
				s.mu.RUnlock()
				return cloneResourceDiscovery(cached), nil
			}
		}
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.getFilePath(id)
	data, err := readRegularFileWithLimit(filePath, maxDiscoveryFileReadBytes)
	if err != nil {
		if os.IsNotExist(err) {
			// Try legacy filename for migrated IDs (e.g., "lxc_node1_101.enc" for "system-container:node1:101")
			legacyID := toLegacyID(id)
			if legacyID != id {
				legacyPath := s.getFilePath(legacyID)
				data, err = readRegularFileWithLimit(legacyPath, maxDiscoveryFileReadBytes)
				if err != nil {
					if os.IsNotExist(err) {
						return nil, nil
					}
					return nil, fmt.Errorf("failed to read discovery file: %w", err)
				}
				// Fall through to decrypt/unmarshal below
			} else {
				return nil, nil // Not found is not an error
			}
		} else {
			return nil, fmt.Errorf("failed to read discovery file: %w", err)
		}
	}

	// Decrypt if crypto is available
	if s.crypto != nil {
		decrypted, err := s.crypto.Decrypt(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt discovery: %w", err)
		}
		data = decrypted
	}

	var discovery ResourceDiscovery
	if err := json.Unmarshal(data, &discovery); err != nil {
		return nil, fmt.Errorf("failed to unmarshal discovery: %w", err)
	}

	// Normalize legacy type strings loaded from disk
	normalizeDiscovery(&discovery)

	// Update cache
	s.cache[id] = cloneResourceDiscovery(&discovery)
	s.cacheTime[id] = time.Now()

	return cloneResourceDiscovery(&discovery), nil
}

// GetByResource retrieves a discovery by resource type and ID.
func (s *Store) GetByResource(resourceType ResourceType, hostID, resourceID string) (*ResourceDiscovery, error) {
	id := MakeResourceID(resourceType, hostID, resourceID)
	return s.Get(id)
}

// Delete removes a discovery from storage.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.getFilePath(id)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete discovery file: %w", err)
	}

	// Remove from cache
	delete(s.cache, id)
	delete(s.cacheTime, id)

	log.Debug().Str("id", id).Msg("discovery deleted")
	return nil
}

// List returns all discoveries.
func (s *Store) List() ([]*ResourceDiscovery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*ResourceDiscovery{}, nil
		}
		return nil, fmt.Errorf("failed to list discovery directory: %w", err)
	}

	var discoveries []*ResourceDiscovery
	for _, entry := range entries {
		// Skip tmp files first to avoid reading partial writes.
		if strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".enc") {
			continue
		}

		data, err := readRegularFileWithLimit(filepath.Join(s.dataDir, entry.Name()), maxDiscoveryFileReadBytes)
		if err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to read discovery file")
			continue
		}

		// Decrypt if crypto is available
		if s.crypto != nil {
			decrypted, err := s.crypto.Decrypt(data)
			if err != nil {
				log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to decrypt discovery")
				continue
			}
			data = decrypted
		}

		var discovery ResourceDiscovery
		if err := json.Unmarshal(data, &discovery); err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to unmarshal discovery")
			continue
		}

		// Normalize legacy type strings loaded from disk
		normalizeDiscovery(&discovery)

		discoveries = append(discoveries, &discovery)
	}

	// Deduplicate by normalized ID — both legacy and new-format files may
	// exist during lazy migration, producing duplicate entries after normalization.
	seen := make(map[string]int, len(discoveries))
	deduped := make([]*ResourceDiscovery, 0, len(discoveries))
	for _, d := range discoveries {
		if idx, exists := seen[d.ID]; exists {
			// Keep the more recently updated entry
			if d.UpdatedAt.After(deduped[idx].UpdatedAt) {
				deduped[idx] = d
			}
		} else {
			seen[d.ID] = len(deduped)
			deduped = append(deduped, d)
		}
	}

	return deduped, nil
}

// ListByType returns discoveries for a specific resource type.
func (s *Store) ListByType(resourceType ResourceType) ([]*ResourceDiscovery, error) {
	all, err := s.List()
	if err != nil {
		return nil, fmt.Errorf("list discoveries for type %s: %w", resourceType, err)
	}

	var filtered []*ResourceDiscovery
	for _, d := range all {
		if d.ResourceType == resourceType {
			filtered = append(filtered, d)
		}
	}
	return filtered, nil
}

// ListByHost returns discoveries for a specific host.
func (s *Store) ListByHost(hostID string) ([]*ResourceDiscovery, error) {
	all, err := s.List()
	if err != nil {
		return nil, fmt.Errorf("list discoveries for host %s: %w", hostID, err)
	}

	var filtered []*ResourceDiscovery
	for _, d := range all {
		if d.HostID == hostID {
			filtered = append(filtered, d)
		}
	}
	return filtered, nil
}

// UpdateNotes updates just the user notes and secrets for a discovery.
func (s *Store) UpdateNotes(id string, notes string, secrets map[string]string) error {
	discovery, err := s.Get(id)
	if err != nil {
		return fmt.Errorf("get discovery %s for note update: %w", id, err)
	}
	if discovery == nil {
		return fmt.Errorf("discovery not found: %s", id)
	}

	discovery.UserNotes = notes
	if secrets != nil {
		discovery.UserSecrets = secrets
	}

	if err := s.Save(discovery); err != nil {
		return fmt.Errorf("save updated discovery %s notes: %w", id, err)
	}
	return nil
}

// GetMultiple retrieves multiple discoveries by ID.
func (s *Store) GetMultiple(ids []string) ([]*ResourceDiscovery, error) {
	var discoveries []*ResourceDiscovery
	for _, id := range ids {
		d, err := s.Get(id)
		if err != nil {
			log.Warn().Err(err).Str("id", id).Msg("failed to get discovery")
			continue
		}
		if d != nil {
			discoveries = append(discoveries, d)
		}
	}
	return discoveries, nil
}

// ClearCache clears the in-memory cache.
func (s *Store) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]*ResourceDiscovery)
	s.cacheTime = make(map[string]time.Time)
}

// Exists checks if a discovery exists for the given ID.
func (s *Store) Exists(id string) bool {
	s.mu.RLock()
	if _, ok := s.cache[id]; ok {
		s.mu.RUnlock()
		return true
	}
	s.mu.RUnlock()

	filePath := s.getFilePath(id)
	_, err := os.Stat(filePath)
	if err != nil && !os.IsNotExist(err) {
		log.Warn().Err(err).Str("id", id).Str("file", filePath).Msg("Failed to stat discovery file")
	}
	return err == nil
}

// GetAge returns how old the discovery is, or -1 if not found.
func (s *Store) GetAge(id string) time.Duration {
	d, err := s.Get(id)
	if err != nil || d == nil {
		return -1
	}
	return time.Since(d.UpdatedAt)
}

// NeedsRefresh checks if a discovery needs to be refreshed.
func (s *Store) NeedsRefresh(id string, maxAge time.Duration) bool {
	age := s.GetAge(id)
	if age < 0 {
		return true // Not found, needs discovery
	}
	return age > maxAge
}

// --- Fingerprint Storage Methods ---

// getFingerprintFilePath returns the file path for a fingerprint.
func (s *Store) getFingerprintFilePath(resourceID string) string {
	// Sanitize ID for filename
	safeID := strings.ReplaceAll(resourceID, ":", "_")
	safeID = strings.ReplaceAll(safeID, "/", "_")
	return filepath.Join(s.fingerprintDir, safeID+".json")
}

// loadFingerprints loads all fingerprints from disk into memory.
func (s *Store) loadFingerprints() {
	s.fingerprintMu.Lock()
	defer s.fingerprintMu.Unlock()

	entries, err := os.ReadDir(s.fingerprintDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn().Err(err).Msg("failed to read fingerprint directory")
		}
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := readRegularFileWithLimit(filepath.Join(s.fingerprintDir, entry.Name()), maxFingerprintFileReadBytes)
		if err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to read fingerprint file")
			continue
		}

		var fp ContainerFingerprint
		if err := json.Unmarshal(data, &fp); err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to unmarshal fingerprint")
			continue
		}

		s.fingerprints[fp.ResourceID] = &fp
	}

	log.Debug().Int("count", len(s.fingerprints)).Msg("loaded fingerprints from disk")
}

// SaveFingerprint stores a container fingerprint.
func (s *Store) SaveFingerprint(fp *ContainerFingerprint) error {
	if fp == nil || fp.ResourceID == "" {
		return fmt.Errorf("fingerprint or resource ID is required")
	}

	s.fingerprintMu.Lock()
	defer s.fingerprintMu.Unlock()

	// Update in-memory cache
	fpCopy := cloneContainerFingerprint(fp)
	s.fingerprints[fpCopy.ResourceID] = fpCopy

	// Persist to disk
	data, err := json.Marshal(fpCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal fingerprint: %w", err)
	}

	filePath := s.getFingerprintFilePath(fpCopy.ResourceID)
	tmpPath := filePath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write fingerprint file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		if cleanupErr := os.Remove(tmpPath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			log.Warn().Err(cleanupErr).Str("tmp_path", tmpPath).Msg("Failed to remove temp fingerprint file after rename failure")
		}
		return fmt.Errorf("failed to finalize fingerprint file: %w", err)
	}

	return nil
}

// GetFingerprint retrieves the last known fingerprint for a resource.
func (s *Store) GetFingerprint(resourceID string) (*ContainerFingerprint, error) {
	s.fingerprintMu.RLock()
	defer s.fingerprintMu.RUnlock()

	fp, ok := s.fingerprints[resourceID]
	if !ok {
		return nil, nil // Not found is not an error
	}
	return cloneContainerFingerprint(fp), nil
}

// GetAllFingerprints returns all stored fingerprints.
func (s *Store) GetAllFingerprints() map[string]*ContainerFingerprint {
	s.fingerprintMu.RLock()
	defer s.fingerprintMu.RUnlock()

	result := make(map[string]*ContainerFingerprint, len(s.fingerprints))
	for k, v := range s.fingerprints {
		result[k] = cloneContainerFingerprint(v)
	}
	return result
}

// GetChangedResources returns resource IDs where the fingerprint changed since last discovery.
// It compares the stored fingerprint hash against the discovery's fingerprint field.
func (s *Store) GetChangedResources() ([]string, error) {
	s.fingerprintMu.RLock()
	fingerprints := make(map[string]*ContainerFingerprint, len(s.fingerprints))
	for k, v := range s.fingerprints {
		fingerprints[k] = cloneContainerFingerprint(v)
	}
	s.fingerprintMu.RUnlock()

	var changed []string
	for resourceID, fp := range fingerprints {
		// The fingerprint key is already in resource ID format (type:host:id)
		// so use it directly as the discovery ID
		discovery, err := s.Get(resourceID)
		if err != nil {
			log.Warn().Err(err).Str("resource_id", resourceID).Msg("Failed to load discovery while checking fingerprint changes")
			continue
		}

		// If no discovery exists, it needs discovery
		if discovery == nil {
			changed = append(changed, resourceID)
			continue
		}

		// If fingerprint hash differs from discovery's stored fingerprint, it changed
		if discovery.Fingerprint != fp.Hash {
			changed = append(changed, resourceID)
		}
	}

	return changed, nil
}

// GetStaleResources returns resources not discovered in maxAge duration.
func (s *Store) GetStaleResources(maxAge time.Duration) ([]string, error) {
	discoveries, err := s.List()
	if err != nil {
		return nil, fmt.Errorf("list discoveries for stale scan: %w", err)
	}

	var stale []string
	now := time.Now()
	for _, d := range discoveries {
		if d == nil {
			continue
		}

		// Staleness should be based on the last successful discovery update.
		// DiscoveredAt is intentionally preserved as first-seen time.
		lastSeenAt := d.UpdatedAt
		if lastSeenAt.IsZero() {
			lastSeenAt = d.DiscoveredAt
		}
		if lastSeenAt.IsZero() || now.Sub(lastSeenAt) > maxAge {
			stale = append(stale, d.ID)
		}
	}

	return stale, nil
}

// SetLastFingerprintScan updates the timestamp of the last fingerprint scan.
func (s *Store) SetLastFingerprintScan(t time.Time) {
	s.fingerprintMu.Lock()
	defer s.fingerprintMu.Unlock()
	s.lastFingerprintScan = t
}

// GetLastFingerprintScan returns the timestamp of the last fingerprint scan.
func (s *Store) GetLastFingerprintScan() time.Time {
	s.fingerprintMu.RLock()
	defer s.fingerprintMu.RUnlock()
	return s.lastFingerprintScan
}

// GetFingerprintCount returns the number of stored fingerprints.
func (s *Store) GetFingerprintCount() int {
	s.fingerprintMu.RLock()
	defer s.fingerprintMu.RUnlock()
	return len(s.fingerprints)
}

// CleanupOrphanedFingerprints removes fingerprints for resources that no longer exist.
// Pass in a set of current resource IDs (e.g., "docker:host1:nginx", "system-container:node1:101").
// Returns the number of fingerprints removed.
func (s *Store) CleanupOrphanedFingerprints(currentResourceIDs map[string]bool) int {
	s.fingerprintMu.Lock()
	defer s.fingerprintMu.Unlock()

	removed := 0
	for fpID := range s.fingerprints {
		normalizedFpID := normalizeResourceID(fpID)
		if !currentResourceIDs[fpID] && !currentResourceIDs[normalizedFpID] {
			// Remove from memory
			delete(s.fingerprints, fpID)

			// Remove from disk
			filePath := s.getFingerprintFilePath(fpID)
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				log.Warn().Err(err).Str("id", fpID).Msg("failed to remove orphaned fingerprint file")
			} else {
				log.Debug().Str("id", fpID).Msg("removed orphaned fingerprint")
			}
			removed++
		}
	}

	return removed
}

// CleanupOrphanedDiscoveries removes discoveries for resources that no longer exist.
// Pass in a set of current resource IDs.
// Returns the number of discoveries removed.
func (s *Store) CleanupOrphanedDiscoveries(currentResourceIDs map[string]bool) int {
	// List all discovery files
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		log.Warn().Err(err).Msg("failed to read discovery directory for cleanup")
		return 0
	}

	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".enc") {
			continue
		}

		resourceID, err := s.readDiscoveryIDFromFile(entry.Name())
		if err != nil {
			log.Warn().
				Err(err).
				Str("file", entry.Name()).
				Msg("Skipping orphan cleanup for discovery file with unreadable ID")
			continue
		}

		// Normalize the stored ID before checking membership (handles legacy "lxc:..." IDs)
		normalizedID := normalizeResourceID(resourceID)
		if !currentResourceIDs[resourceID] && !currentResourceIDs[normalizedID] {
			filePath := filepath.Join(s.dataDir, entry.Name())
			if err := os.Remove(filePath); err != nil {
				log.Warn().Err(err).Str("file", entry.Name()).Msg("failed to remove orphaned discovery file")
			} else {
				log.Debug().Str("id", resourceID).Msg("removed orphaned discovery")
				removed++
			}
		}
	}

	return removed
}

// filenameToResourceID converts a discovery filename back to a resource ID.
// Reverses the transformation done in getFilePath.
func filenameToResourceID(filename string) string {
	// The filename uses underscores for colons and slashes
	// We need to be smart about this - the format is type_host_resourceid
	// First underscore separates type, rest could have underscores in host/resource names

	parts := strings.SplitN(filename, "_", 3)
	if len(parts) < 3 {
		return filename // Can't parse, return as-is
	}

	resourceType := parts[0]
	host := parts[1]
	resourceID := parts[2]

	// For k8s, the resource ID might have been namespace/name which became namespace_name
	// We convert back: k8s:cluster:namespace/name
	if resourceType == "k8s" && strings.Contains(resourceID, "_") {
		// Could be namespace_name, convert back to namespace/name
		resourceID = strings.Replace(resourceID, "_", "/", 1)
	}

	return resourceType + ":" + host + ":" + resourceID
}

func (s *Store) readDiscoveryIDFromFile(filename string) (string, error) {
	filePath := filepath.Join(s.dataDir, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read discovery file: %w", err)
	}

	// Discovery files may be encrypted when crypto is configured.
	if s.crypto != nil {
		decrypted, err := s.crypto.Decrypt(data)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt discovery file: %w", err)
		}
		data = decrypted
	}

	var discovery struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &discovery); err != nil {
		return "", fmt.Errorf("failed to unmarshal discovery ID: %w", err)
	}
	if strings.TrimSpace(discovery.ID) == "" {
		return "", fmt.Errorf("discovery ID is empty")
	}

	return discovery.ID, nil
}

// ListDiscoveryIDs returns all discovery IDs currently stored.
func (s *Store) ListDiscoveryIDs() []string {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil
	}

	var ids []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".enc") {
			continue
		}
		id, err := s.readDiscoveryIDFromFile(entry.Name())
		if err != nil {
			log.Warn().
				Err(err).
				Str("file", entry.Name()).
				Msg("Skipping discovery ID listing for unreadable discovery file")
			continue
		}
		ids = append(ids, id)
	}
	return ids
}
