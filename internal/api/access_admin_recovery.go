package api

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

// RBACIntegrityResult contains the result of an RBAC data integrity check.
type RBACIntegrityResult = extensions.RBACIntegrityResult

// VerifyRBACIntegrity checks the RBAC data integrity for a given org.
// Returns a structured result indicating db health, schema presence, and role counts.
func VerifyRBACIntegrity(provider *TenantRBACProvider, orgID string) RBACIntegrityResult {
	normalizedOrgID := normalizeOrgID(orgID)
	result := RBACIntegrityResult{OrgID: normalizedOrgID}

	if provider == nil {
		result.Error = "provider is nil"
		return result
	}

	manager, err := provider.GetManager(normalizedOrgID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get manager: %v", err)
		return result
	}
	result.DBAccessible = true

	// If manager access succeeds, schema queries are available through manager methods.
	roles := manager.GetRoles()
	result.TablesPresent = true
	result.TotalRoles = len(roles)

	for _, role := range roles {
		if role.IsBuiltIn {
			result.BuiltInRoleCount++
		}
	}

	assignments := manager.GetUserAssignments()
	result.TotalAssignments = len(assignments)

	// Healthy if db accessible, tables present, and at least 4 built-in roles exist.
	result.Healthy = result.DBAccessible && result.TablesPresent && result.BuiltInRoleCount >= 4
	if result.Healthy {
		RecordRBACIntegrityCheck("healthy")
	} else {
		RecordRBACIntegrityCheck("unhealthy")
	}

	return result
}

// ResetAdminRole ensures the admin role exists with full permissions and assigns
// the specified user to it. This is a break-glass recovery function for RBAC lockout.
// It uses context-aware manager operations to capture an audit trail.
func ResetAdminRole(provider *TenantRBACProvider, orgID string, username string) error {
	if provider == nil {
		return fmt.Errorf("provider is nil")
	}
	if username == "" {
		return fmt.Errorf("username is required")
	}

	normalizedOrgID := normalizeOrgID(orgID)

	manager, err := provider.GetManager(normalizedOrgID)
	if err != nil {
		return fmt.Errorf("failed to get manager for org %s: %w", normalizedOrgID, err)
	}

	if _, exists := manager.GetRole(auth.RoleAdmin); !exists {
		// Force manager reinitialization to trigger built-in role bootstrap.
		if err := provider.RemoveTenant(normalizedOrgID); err != nil {
			return fmt.Errorf("failed to refresh manager for org %s: %w", normalizedOrgID, err)
		}

		manager, err = provider.GetManager(normalizedOrgID)
		if err != nil {
			return fmt.Errorf("failed to reinitialize manager for org %s: %w", normalizedOrgID, err)
		}

		if _, exists := manager.GetRole(auth.RoleAdmin); !exists {
			// Fallback recreation if bootstrap could not restore admin.
			adminRole := auth.Role{
				ID:          auth.RoleAdmin,
				Name:        "Administrator",
				Description: "Full administrative access to all features",
				IsBuiltIn:   true,
				Priority:    100,
				Permissions: []auth.Permission{
					{Action: "admin", Resource: "*", Effect: auth.EffectAllow},
				},
			}
			if err := manager.SaveRoleWithContext(adminRole, "break-glass-recovery"); err != nil {
				return fmt.Errorf("failed to recreate admin role: %w", err)
			}
		}
	}

	if err := manager.UpdateUserRolesWithContext(username, []string{auth.RoleAdmin}, "break-glass-recovery"); err != nil {
		return fmt.Errorf("failed to assign admin role to user %s: %w", username, err)
	}

	return nil
}

// BackupRBACData copies the RBAC database for the given org to a timestamped backup.
// Returns the path to the backup file.
func BackupRBACData(provider *TenantRBACProvider, orgID string, destDir string) (string, error) {
	if provider == nil {
		return "", fmt.Errorf("provider is nil")
	}
	if destDir == "" {
		return "", fmt.Errorf("destination directory is required")
	}

	normalizedOrgID := normalizeOrgID(orgID)

	var dbPath string
	if normalizedOrgID == "default" {
		dbPath = filepath.Join(provider.baseDataDir, "rbac", "rbac.db")
	} else {
		dbPath = filepath.Join(provider.baseDataDir, "orgs", normalizedOrgID, "rbac", "rbac.db")
	}

	stat, err := os.Stat(dbPath)
	if err != nil {
		return "", fmt.Errorf("RBAC database not found for org %s: %w", normalizedOrgID, err)
	}
	if stat.IsDir() {
		return "", fmt.Errorf("RBAC database path is a directory: %s", dbPath)
	}

	if err := os.MkdirAll(destDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	backupName := fmt.Sprintf("rbac-%s-%s.db", normalizedOrgID, timestamp)
	backupPath := filepath.Join(destDir, backupName)

	src, err := os.Open(dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source db: %w", err)
	}
	defer func() {
		if closeErr := src.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close source db: %w", closeErr))
		}
	}()

	dst, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}

	cleanupFailedBackup := func(baseErr error) error {
		if closeErr := dst.Close(); closeErr != nil {
			baseErr = errors.Join(baseErr, fmt.Errorf("failed to close backup file after failure: %w", closeErr))
		}
		if removeErr := os.Remove(backupPath); removeErr != nil && !os.IsNotExist(removeErr) {
			baseErr = errors.Join(baseErr, fmt.Errorf("failed to remove incomplete backup %s: %w", backupPath, removeErr))
		}
		return baseErr
	}

	if _, err := io.Copy(dst, src); err != nil {
		return "", cleanupFailedBackup(fmt.Errorf("failed to copy database: %w", err))
	}

	if err := dst.Sync(); err != nil {
		return "", cleanupFailedBackup(fmt.Errorf("failed to flush backup file: %w", err))
	}

	if err := dst.Close(); err != nil {
		closeErr := fmt.Errorf("failed to close backup file: %w", err)
		if removeErr := os.Remove(backupPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", errors.Join(closeErr, fmt.Errorf("failed to remove incomplete backup %s: %w", backupPath, removeErr))
		}
		return "", closeErr
	}

	return backupPath, nil
}
