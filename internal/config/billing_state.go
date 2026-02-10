package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

// Ensure FileBillingStore satisfies the hosted entitlement BillingStore interface.
var _ entitlements.BillingStore = (*FileBillingStore)(nil)

// FileBillingStore persists billing state in per-org files under the data directory.
type FileBillingStore struct {
	baseDataDir string
	mu          sync.RWMutex
}

// NewFileBillingStore creates a file-backed billing store rooted at baseDataDir.
func NewFileBillingStore(baseDataDir string) *FileBillingStore {
	return &FileBillingStore{baseDataDir: baseDataDir}
}

// GetBillingState returns the current billing state for an org.
// Missing billing files are treated as "no state yet" and return (nil, nil).
func (s *FileBillingStore) GetBillingState(orgID string) (*entitlements.BillingState, error) {
	billingPath, err := s.billingStatePath(orgID)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(billingPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read billing state for org %q: %w", orgID, err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var state entitlements.BillingState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode billing state for org %q: %w", orgID, err)
	}

	return &state, nil
}

// SaveBillingState persists billing state for an org to billing.json.
func (s *FileBillingStore) SaveBillingState(orgID string, state *entitlements.BillingState) error {
	if state == nil {
		return errors.New("billing state is required")
	}

	billingPath, err := s.billingStatePath(orgID)
	if err != nil {
		return err
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode billing state for org %q: %w", orgID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(billingPath), 0o700); err != nil {
		return fmt.Errorf("create billing directory for org %q: %w", orgID, err)
	}

	tmpPath := billingPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write temp billing state for org %q: %w", orgID, err)
	}
	if err := os.Rename(tmpPath, billingPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("commit billing state for org %q: %w", orgID, err)
	}

	return nil
}

func (s *FileBillingStore) billingStatePath(orgID string) (string, error) {
	orgID = strings.TrimSpace(orgID)
	if !isValidOrgID(orgID) {
		return "", fmt.Errorf("invalid organization ID: %s", orgID)
	}
	// Default org stores config at the root data dir for backward compatibility,
	// so billing state for the default org must live alongside other root configs.
	if orgID == "default" {
		return filepath.Join(s.resolveDataDir(), "billing.json"), nil
	}
	return filepath.Join(s.resolveDataDir(), "orgs", orgID, "billing.json"), nil
}

func (s *FileBillingStore) resolveDataDir() string {
	if dir := strings.TrimSpace(s.baseDataDir); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("PULSE_DATA_DIR")); dir != "" {
		return dir
	}
	return "/etc/pulse"
}
