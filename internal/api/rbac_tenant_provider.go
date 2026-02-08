package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// TenantRBACProvider manages per-tenant RBAC Manager instances.
// Follows the same file-based isolation pattern as MultiTenantPersistence.
type TenantRBACProvider struct {
	baseDataDir string
	mu          sync.RWMutex
	managers    map[string]*auth.SQLiteManager
}

// NewTenantRBACProvider creates a new provider.
func NewTenantRBACProvider(baseDataDir string) *TenantRBACProvider {
	return &TenantRBACProvider{
		baseDataDir: baseDataDir,
		managers:    make(map[string]*auth.SQLiteManager),
	}
}

// GetManager returns the RBAC Manager for the given org, creating it lazily if needed.
// For "default" org: DataDir = baseDataDir (db at {baseDataDir}/rbac/rbac.db — existing location).
// For other orgs: DataDir = {baseDataDir}/orgs/{orgID} (db at {baseDataDir}/orgs/{orgID}/rbac/rbac.db).
func (p *TenantRBACProvider) GetManager(orgID string) (auth.ExtendedManager, error) {
	orgID = normalizeOrgID(orgID)

	p.mu.RLock()
	manager, exists := p.managers[orgID]
	p.mu.RUnlock()
	if exists {
		return manager, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check locking pattern.
	if manager, exists = p.managers[orgID]; exists {
		return manager, nil
	}

	dataDir, err := p.resolveDataDir(orgID)
	if err != nil {
		return nil, err
	}

	manager, err = auth.NewSQLiteManager(auth.SQLiteManagerConfig{
		DataDir: dataDir,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize RBAC manager for org %s: %w", orgID, err)
	}

	p.managers[orgID] = manager
	return manager, nil
}

// RemoveTenant closes and removes the cached manager for the given org.
// Called when an org is deleted.
func (p *TenantRBACProvider) RemoveTenant(orgID string) error {
	orgID = normalizeOrgID(orgID)

	p.mu.Lock()
	manager, exists := p.managers[orgID]
	if exists {
		delete(p.managers, orgID)
	}
	p.mu.Unlock()

	if !exists {
		return nil
	}

	if err := manager.Close(); err != nil {
		return fmt.Errorf("failed to close RBAC manager for org %s: %w", orgID, err)
	}

	return nil
}

// Close closes all cached managers.
func (p *TenantRBACProvider) Close() error {
	p.mu.Lock()
	managers := p.managers
	p.managers = make(map[string]*auth.SQLiteManager)
	p.mu.Unlock()

	var closeErr error
	for orgID, manager := range managers {
		if err := manager.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("failed to close RBAC manager for org %s: %w", orgID, err))
		}
	}

	return closeErr
}

func (p *TenantRBACProvider) resolveDataDir(orgID string) (string, error) {
	if orgID == "default" {
		return p.baseDataDir, nil
	}

	if !isValidTenantOrgID(orgID) {
		return "", fmt.Errorf("invalid organization ID: %s", orgID)
	}

	orgDir := filepath.Join(p.baseDataDir, "orgs", orgID)
	stat, err := os.Stat(orgDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("organization directory does not exist: %s", orgID)
		}
		return "", fmt.Errorf("failed to read organization directory for %s: %w", orgID, err)
	}
	if !stat.IsDir() {
		return "", fmt.Errorf("organization path is not a directory: %s", orgID)
	}

	return orgDir, nil
}

func normalizeOrgID(orgID string) string {
	if orgID == "" {
		return "default"
	}
	return orgID
}

func isValidTenantOrgID(orgID string) bool {
	return filepath.Base(orgID) == orgID && orgID != "" && orgID != "." && orgID != ".."
}
