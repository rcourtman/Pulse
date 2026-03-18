package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

var orgIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// MultiTenantPersistence manages a collection of TenantPersistence instances,
// one for each organization.
type MultiTenantPersistence struct {
	baseDataDir string
	mu          sync.RWMutex
	tenants     map[string]*ConfigPersistence
}

// NewMultiTenantPersistence creates a new multi-tenant persistence manager.
func NewMultiTenantPersistence(baseDataDir string) *MultiTenantPersistence {
	return &MultiTenantPersistence{
		baseDataDir: baseDataDir,
		tenants:     make(map[string]*ConfigPersistence),
	}
}

// BaseDataDir returns the base data directory used by multi-tenant persistence.
func (mtp *MultiTenantPersistence) BaseDataDir() string {
	return mtp.baseDataDir
}

func isValidOrgID(orgID string) bool {
	if orgID == "" || orgID == "." || orgID == ".." {
		return false
	}
	if filepath.Base(orgID) != orgID {
		return false
	}
	return orgIDPattern.MatchString(orgID)
}

// GetPersistence returns the persistence instance for a specific organization.
// It initializes the persistence if it hasn't been loaded yet.
func (mtp *MultiTenantPersistence) GetPersistence(orgID string) (*ConfigPersistence, error) {
	mtp.mu.RLock()
	persistence, exists := mtp.tenants[orgID]
	mtp.mu.RUnlock()

	if exists {
		return persistence, nil
	}

	mtp.mu.Lock()
	defer mtp.mu.Unlock()

	// Double-check locking pattern
	if persistence, exists = mtp.tenants[orgID]; exists {
		return persistence, nil
	}

	// Validate OrgID (prevent directory traversal)
	if !isValidOrgID(orgID) {
		return nil, fmt.Errorf("invalid organization ID: %s", orgID)
	}

	// Determine org data directory
	// Global/Default org uses the root data dir (legacy compatibility)
	// New orgs use /data/orgs/<org-id>
	var orgDir string
	if orgID == "default" {
		// IMPORTANT: Default org uses root data dir for backward compatibility
		// This ensures existing users' configs (nodes.enc, ai.enc, etc.) continue to work
		orgDir = mtp.baseDataDir
	} else {
		orgDir = filepath.Join(mtp.baseDataDir, "orgs", orgID)
	}

	log.Info().Str("org_id", orgID).Str("dir", orgDir).Msg("Initializing tenant persistence")

	cp, err := newConfigPersistence(orgDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize persistence for org %s: %w", orgID, err)
	}

	// Ensure the directory exists
	if err := cp.EnsureConfigDir(); err != nil {
		return nil, fmt.Errorf("ensure config directory for org %s: %w", orgID, err)
	}

	mtp.tenants[orgID] = cp
	return cp, nil
}

// OrgExists checks if an organization exists (directory exists) without creating it.
func (mtp *MultiTenantPersistence) OrgExists(orgID string) bool {
	if orgID == "default" {
		return true
	}

	// Validate to prevent traversal
	if !isValidOrgID(orgID) {
		return false
	}

	orgDir := filepath.Join(mtp.baseDataDir, "orgs", orgID)
	stat, err := os.Stat(orgDir)
	return err == nil && stat.IsDir()
}

// LoadOrganization loads the organization metadata including members.
// Org metadata is stored in <orgDir>/org.json.
func (mtp *MultiTenantPersistence) LoadOrganization(orgID string) (*models.Organization, error) {
	persistence, err := mtp.GetPersistence(orgID)
	if err != nil {
		return nil, fmt.Errorf("get persistence for org %s: %w", orgID, err)
	}

	org, err := persistence.LoadOrganization()
	if err != nil {
		// If org.json doesn't exist, return a default org.
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("load organization %s: %w", orgID, err)
		}
		return &models.Organization{
			ID:          orgID,
			DisplayName: orgID,
		}, nil
	}

	return org, nil
}

// LoadOrganizationStrict loads organization metadata from org.json and returns an error when it does not exist.
// This is useful for hosted control-plane paths that need to distinguish "missing org metadata" from defaults.
func (mtp *MultiTenantPersistence) LoadOrganizationStrict(orgID string) (*models.Organization, error) {
	if mtp == nil {
		return nil, fmt.Errorf("no persistence configured")
	}
	if orgID != "default" && !mtp.OrgExists(orgID) {
		return nil, os.ErrNotExist
	}
	persistence, err := mtp.GetPersistence(orgID)
	if err != nil {
		return nil, fmt.Errorf("get persistence for org %s: %w", orgID, err)
	}
	org, err := persistence.LoadOrganization()
	if err != nil {
		return nil, fmt.Errorf("load organization %s: %w", orgID, err)
	}
	return org, nil
}

// SaveOrganization saves the organization metadata.
func (mtp *MultiTenantPersistence) SaveOrganization(org *models.Organization) error {
	if org == nil {
		return fmt.Errorf("organization is required")
	}

	persistence, err := mtp.GetPersistence(org.ID)
	if err != nil {
		return fmt.Errorf("get persistence for org %s: %w", org.ID, err)
	}

	if err := persistence.SaveOrganization(org); err != nil {
		return fmt.Errorf("save organization %s: %w", org.ID, err)
	}

	return nil
}

// ListOrganizations returns all known organizations (including the default org).
func (mtp *MultiTenantPersistence) ListOrganizations() ([]*models.Organization, error) {
	orgIDs := map[string]struct{}{
		"default": {},
	}

	orgsDir := filepath.Join(mtp.baseDataDir, "orgs")
	entries, err := os.ReadDir(orgsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read organizations directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		orgID := entry.Name()
		if !isValidOrgID(orgID) {
			log.Warn().Str("org_id", orgID).Msg("Skipping invalid organization directory name")
			continue
		}
		orgIDs[orgID] = struct{}{}
	}

	sortedIDs := make([]string, 0, len(orgIDs))
	for orgID := range orgIDs {
		sortedIDs = append(sortedIDs, orgID)
	}
	sort.Strings(sortedIDs)

	orgs := make([]*models.Organization, 0, len(sortedIDs))
	for _, orgID := range sortedIDs {
		org, loadErr := mtp.LoadOrganization(orgID)
		if loadErr != nil {
			return nil, fmt.Errorf("failed to load organization %s: %w", orgID, loadErr)
		}
		orgs = append(orgs, org)
	}

	return orgs, nil
}

// DeleteOrganization removes a non-default organization and its persisted directory.
func (mtp *MultiTenantPersistence) DeleteOrganization(orgID string) error {
	if orgID == "default" {
		return fmt.Errorf("default organization cannot be deleted")
	}
	if !isValidOrgID(orgID) {
		return fmt.Errorf("invalid organization ID: %s", orgID)
	}
	if !mtp.OrgExists(orgID) {
		return os.ErrNotExist
	}

	mtp.mu.Lock()
	delete(mtp.tenants, orgID)
	mtp.mu.Unlock()

	orgDir := filepath.Join(mtp.baseDataDir, "orgs", orgID)
	if err := os.RemoveAll(orgDir); err != nil {
		return fmt.Errorf("failed to delete organization directory: %w", err)
	}

	return nil
}
