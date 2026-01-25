package aidiscovery

import (
	"encoding/json"
	"fmt"
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
}

// For testing - allows injecting a mock crypto manager
var newCryptoManagerAt = crypto.NewCryptoManagerAt

// For testing - allows injecting a mock marshaler.
var marshalDiscovery = json.Marshal

// NewStore creates a new discovery store with automatic encryption.
func NewStore(dataDir string) (*Store, error) {
	discoveryDir := filepath.Join(dataDir, "discovery")
	if err := os.MkdirAll(discoveryDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create discovery directory: %w", err)
	}

	// Initialize crypto manager for encryption (uses same key as other Pulse secrets)
	cryptoMgr, err := newCryptoManagerAt(dataDir)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize crypto for discovery store, data will be unencrypted")
	}

	return &Store{
		dataDir:   discoveryDir,
		crypto:    cryptoMgr,
		cache:     make(map[string]*ResourceDiscovery),
		cacheTime: make(map[string]time.Time),
		cacheTTL:  5 * time.Minute,
	}, nil
}

// getFilePath returns the file path for a resource ID.
func (s *Store) getFilePath(id string) string {
	// Sanitize ID for filename: replace : with _
	safeID := strings.ReplaceAll(id, ":", "_")
	safeID = strings.ReplaceAll(safeID, "/", "_")
	return filepath.Join(s.dataDir, safeID+".enc")
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

	data, err := marshalDiscovery(d)
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
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to finalize discovery file: %w", err)
	}

	// Update cache
	s.cache[d.ID] = d
	s.cacheTime[d.ID] = time.Now()

	log.Debug().Str("id", d.ID).Str("service", d.ServiceType).Msg("Discovery saved")
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
				return cached, nil
			}
		}
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.getFilePath(id)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("failed to read discovery file: %w", err)
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

	// Update cache
	s.cache[id] = &discovery
	s.cacheTime[id] = time.Now()

	return &discovery, nil
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

	log.Debug().Str("id", id).Msg("Discovery deleted")
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

		data, err := os.ReadFile(filepath.Join(s.dataDir, entry.Name()))
		if err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to read discovery file")
			continue
		}

		// Decrypt if crypto is available
		if s.crypto != nil {
			decrypted, err := s.crypto.Decrypt(data)
			if err != nil {
				log.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to decrypt discovery")
				continue
			}
			data = decrypted
		}

		var discovery ResourceDiscovery
		if err := json.Unmarshal(data, &discovery); err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to unmarshal discovery")
			continue
		}

		discoveries = append(discoveries, &discovery)
	}

	return discoveries, nil
}

// ListByType returns discoveries for a specific resource type.
func (s *Store) ListByType(resourceType ResourceType) ([]*ResourceDiscovery, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
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
		return nil, err
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
		return err
	}
	if discovery == nil {
		return fmt.Errorf("discovery not found: %s", id)
	}

	discovery.UserNotes = notes
	if secrets != nil {
		discovery.UserSecrets = secrets
	}

	return s.Save(discovery)
}

// GetMultiple retrieves multiple discoveries by ID.
func (s *Store) GetMultiple(ids []string) ([]*ResourceDiscovery, error) {
	var discoveries []*ResourceDiscovery
	for _, id := range ids {
		d, err := s.Get(id)
		if err != nil {
			log.Warn().Err(err).Str("id", id).Msg("Failed to get discovery")
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
