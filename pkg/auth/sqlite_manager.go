package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

const maxLegacyRBACFileSize = 16 << 20

// SQLiteManagerConfig configures the SQLite RBAC manager.
type SQLiteManagerConfig struct {
	DataDir            string // Directory for rbac.db
	MigrateFromFiles   bool   // Attempt to migrate from file-based storage
	ChangeLogRetention int    // Days to keep change logs (default: 90, 0 = forever)
}

// SQLiteManager implements ExtendedManager with SQLite persistence.
type SQLiteManager struct {
	mu                 sync.RWMutex
	db                 *sql.DB
	dbPath             string
	changeLogRetention int
}

// NewSQLiteManager creates a new SQLite-backed RBAC manager.
func NewSQLiteManager(cfg SQLiteManagerConfig) (*SQLiteManager, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("data directory is required")
	}

	dataDir, err := securityutil.NormalizeStorageDir(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("normalize data directory: %w", err)
	}
	rbacDir, err := securityutil.JoinStorageLeaf(dataDir, "rbac")
	if err != nil {
		return nil, fmt.Errorf("resolve rbac directory: %w", err)
	}
	if err := securityutil.EnsureSecureStorageDir(rbacDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create rbac directory: %w", err)
	}

	dbPath, err := securityutil.JoinStorageLeaf(rbacDir, "rbac.db")
	if err != nil {
		return nil, fmt.Errorf("resolve rbac database path: %w", err)
	}

	// Open database with pragmas in DSN so every pool connection is configured
	dsn := dbPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
			"foreign_keys(ON)",
			"cache_size(-32000)",
		},
	}.Encode()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open rbac database: %w", err)
	}

	// SQLite works best with a single writer connection
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	retention := cfg.ChangeLogRetention
	if retention == 0 {
		retention = 90 // Default 90 days
	}

	m := &SQLiteManager{
		db:                 db,
		dbPath:             dbPath,
		changeLogRetention: retention,
	}

	// Initialize schema
	if err := m.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Initialize built-in roles if they don't exist
	if err := m.initBuiltInRoles(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize built-in roles: %w", err)
	}

	// Migrate from file-based storage if requested
	if cfg.MigrateFromFiles {
		if err := m.migrateFromFiles(dataDir); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate legacy RBAC data: %w", err)
		}
	}

	return m, nil
}

func (m *SQLiteManager) initSchema() error {
	schema := `
	-- Roles table
	CREATE TABLE IF NOT EXISTS rbac_roles (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		parent_id TEXT,
		is_built_in INTEGER NOT NULL DEFAULT 0,
		priority INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		FOREIGN KEY (parent_id) REFERENCES rbac_roles(id) ON DELETE SET NULL
	);

	-- Permissions table (one-to-many with roles)
	CREATE TABLE IF NOT EXISTS rbac_permissions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		role_id TEXT NOT NULL,
		action TEXT NOT NULL,
		resource TEXT NOT NULL,
		effect TEXT NOT NULL DEFAULT 'allow',
		conditions TEXT,
		FOREIGN KEY (role_id) REFERENCES rbac_roles(id) ON DELETE CASCADE
	);

	-- User role assignments
	CREATE TABLE IF NOT EXISTS rbac_users (
		username TEXT PRIMARY KEY,
		updated_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS rbac_user_assignments (
		username TEXT NOT NULL,
		role_id TEXT NOT NULL,
		updated_at INTEGER NOT NULL,
		PRIMARY KEY (username, role_id),
		FOREIGN KEY (username) REFERENCES rbac_users(username) ON DELETE CASCADE,
		FOREIGN KEY (role_id) REFERENCES rbac_roles(id) ON DELETE CASCADE
	);

	-- Backfill the identity table for databases created before rbac_users
	-- existed. This preserves users when their last role is removed.
	INSERT OR IGNORE INTO rbac_users (username, updated_at)
	SELECT username, MAX(updated_at)
	FROM rbac_user_assignments
	GROUP BY username;

	-- Change log
	CREATE TABLE IF NOT EXISTS rbac_changelog (
		id TEXT PRIMARY KEY,
		action TEXT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		old_value TEXT,
		new_value TEXT,
		user TEXT,
		timestamp INTEGER NOT NULL
	);

	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_rbac_perm_role ON rbac_permissions(role_id);
	CREATE INDEX IF NOT EXISTS idx_rbac_assign_user ON rbac_user_assignments(username);
	CREATE INDEX IF NOT EXISTS idx_rbac_changelog_time ON rbac_changelog(timestamp);
	CREATE INDEX IF NOT EXISTS idx_rbac_changelog_entity ON rbac_changelog(entity_type, entity_id);
	`

	_, err := m.db.Exec(schema)
	return err
}

func (m *SQLiteManager) initBuiltInRoles() error {
	now := time.Now().Unix()

	builtInRoles := []struct {
		id          string
		name        string
		description string
		permissions []Permission
	}{
		{
			id:          RoleAdmin,
			name:        "Administrator",
			description: "Full administrative access to all features",
			permissions: []Permission{{Action: "admin", Resource: "*"}},
		},
		{
			id:          RoleOperator,
			name:        "Operator",
			description: "Can manage nodes and perform operational tasks",
			permissions: []Permission{
				{Action: "read", Resource: "*"},
				{Action: "write", Resource: "nodes"},
				{Action: "write", Resource: "vms"},
				{Action: "write", Resource: "containers"},
				{Action: "write", Resource: "alerts"},
			},
		},
		{
			id:          RoleViewer,
			name:        "Viewer",
			description: "Read-only access to monitoring data",
			permissions: []Permission{{Action: "read", Resource: "*"}},
		},
		{
			id:          RoleAuditor,
			name:        "Auditor",
			description: "Access to audit logs and compliance data",
			permissions: []Permission{
				{Action: "read", Resource: "audit_logs"},
				{Action: "read", Resource: "nodes"},
				{Action: "read", Resource: "alerts"},
				{Action: "read", Resource: "compliance"},
			},
		},
	}

	for _, r := range builtInRoles {
		// Check if role already exists
		var count int
		err := m.db.QueryRow("SELECT COUNT(*) FROM rbac_roles WHERE id = ?", r.id).Scan(&count)
		if err != nil {
			return err
		}

		if count > 0 {
			continue // Role already exists
		}

		// Insert role
		_, err = m.db.Exec(`
			INSERT INTO rbac_roles (id, name, description, is_built_in, priority, created_at, updated_at)
			VALUES (?, ?, ?, 1, 0, ?, ?)
		`, r.id, r.name, r.description, now, now)
		if err != nil {
			return err
		}

		// Insert permissions
		for _, perm := range r.permissions {
			_, err = m.db.Exec(`
				INSERT INTO rbac_permissions (role_id, action, resource, effect)
				VALUES (?, ?, ?, 'allow')
			`, r.id, perm.Action, perm.Resource)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Close closes the database connection.
func (m *SQLiteManager) Close() error {
	return m.db.Close()
}

// GetRoles returns all roles.
func (m *SQLiteManager) GetRoles() []Role {
	roles, err := m.GetRolesWithError()
	if err != nil {
		log.Error().Err(err).Msg("Failed to query roles")
		return nil
	}
	return roles
}

// GetRolesWithError returns all roles and preserves storage errors.
func (m *SQLiteManager) GetRolesWithError() ([]Role, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getRolesUnsafe()
}

func (m *SQLiteManager) getRolesUnsafe() ([]Role, error) {
	rows, err := m.db.Query(`
		SELECT id, name, description, parent_id, is_built_in, priority, created_at, updated_at
		FROM rbac_roles
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}

	// Collect roles first, then close rows before loading permissions
	// (avoids holding the connection during nested queries with MaxOpenConns=1)
	var roles []Role
	for rows.Next() {
		var role Role
		var parentID sql.NullString
		var createdAt, updatedAt int64
		var isBuiltIn int

		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &parentID, &isBuiltIn, &role.Priority, &createdAt, &updatedAt); err != nil {
			rows.Close()
			return nil, err
		}

		role.ParentID = parentID.String
		role.IsBuiltIn = isBuiltIn == 1
		role.CreatedAt = time.Unix(createdAt, 0)
		role.UpdatedAt = time.Unix(updatedAt, 0)

		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	// Load permissions after releasing the connection
	for i := range roles {
		permissions, err := m.loadRolePermissionsWithError(roles[i].ID)
		if err != nil {
			return nil, err
		}
		roles[i].Permissions = permissions
	}

	if roles == nil {
		roles = []Role{}
	}
	return roles, nil
}

func (m *SQLiteManager) loadRolePermissions(roleID string) []Permission {
	permissions, err := m.loadRolePermissionsWithError(roleID)
	if err != nil {
		log.Error().Err(err).Str("roleId", roleID).Msg("Failed to query permissions")
		return nil
	}
	return permissions
}

func (m *SQLiteManager) loadRolePermissionsWithError(roleID string) ([]Permission, error) {
	rows, err := m.db.Query(`
		SELECT action, resource, effect, conditions
		FROM rbac_permissions
		WHERE role_id = ?
	`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []Permission
	for rows.Next() {
		var perm Permission
		var conditions sql.NullString

		if err := rows.Scan(&perm.Action, &perm.Resource, &perm.Effect, &conditions); err != nil {
			return nil, err
		}

		if conditions.Valid && conditions.String != "" {
			if err := json.Unmarshal([]byte(conditions.String), &perm.Conditions); err != nil {
				return nil, fmt.Errorf("parse conditions for role %s: %w", roleID, err)
			}
		}

		perms = append(perms, perm)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	if perms == nil {
		perms = []Permission{}
	}
	return perms, nil
}

// GetRole returns a role by ID.
func (m *SQLiteManager) GetRole(id string) (Role, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var role Role
	var parentID sql.NullString
	var createdAt, updatedAt int64
	var isBuiltIn int

	err := m.db.QueryRow(`
		SELECT id, name, description, parent_id, is_built_in, priority, created_at, updated_at
		FROM rbac_roles
		WHERE id = ?
	`, id).Scan(&role.ID, &role.Name, &role.Description, &parentID, &isBuiltIn, &role.Priority, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return Role{}, false
	}
	if err != nil {
		log.Error().Err(err).Str("roleId", id).Msg("Failed to query role")
		return Role{}, false
	}

	role.ParentID = parentID.String
	role.IsBuiltIn = isBuiltIn == 1
	role.CreatedAt = time.Unix(createdAt, 0)
	role.UpdatedAt = time.Unix(updatedAt, 0)
	role.Permissions = m.loadRolePermissions(role.ID)

	return role, true
}

// SaveRole creates or updates a role.
func (m *SQLiteManager) SaveRole(role Role) error {
	return m.SaveRoleWithContext(role, "")
}

// SaveRoleWithContext creates or updates a role with audit context.
func (m *SQLiteManager) SaveRoleWithContext(role Role, username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if it's a built-in role
	var isBuiltIn int
	err := m.db.QueryRow("SELECT is_built_in FROM rbac_roles WHERE id = ?", role.ID).Scan(&isBuiltIn)
	if err == nil && isBuiltIn == 1 {
		return fmt.Errorf("cannot modify built-in role: %s", role.ID)
	}

	// Check for circular inheritance
	if role.ParentID != "" {
		if err := m.checkCircularInheritanceUnsafe(role.ID, role.ParentID); err != nil {
			return err
		}
	}

	// Get old value for changelog
	oldRole, exists := m.getRoleUnsafe(role.ID)
	var oldValueJSON string
	if exists {
		if data, err := json.Marshal(oldRole); err == nil {
			oldValueJSON = string(data)
		}
	}

	now := time.Now()
	role.UpdatedAt = now
	if role.CreatedAt.IsZero() {
		role.CreatedAt = now
	}

	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Upsert role
	_, err = tx.Exec(`
		INSERT INTO rbac_roles (id, name, description, parent_id, is_built_in, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			parent_id = excluded.parent_id,
			priority = excluded.priority,
			updated_at = excluded.updated_at
	`, role.ID, role.Name, role.Description, nullString(role.ParentID), role.Priority, role.CreatedAt.Unix(), role.UpdatedAt.Unix())
	if err != nil {
		return err
	}

	// Delete existing permissions and insert new ones
	_, err = tx.Exec("DELETE FROM rbac_permissions WHERE role_id = ?", role.ID)
	if err != nil {
		return err
	}

	for _, perm := range role.Permissions {
		var conditionsJSON *string
		if len(perm.Conditions) > 0 {
			if data, err := json.Marshal(perm.Conditions); err == nil {
				s := string(data)
				conditionsJSON = &s
			}
		}

		effect := perm.GetEffect()
		_, err = tx.Exec(`
			INSERT INTO rbac_permissions (role_id, action, resource, effect, conditions)
			VALUES (?, ?, ?, ?, ?)
		`, role.ID, perm.Action, perm.Resource, effect, conditionsJSON)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Log change
	action := ActionRoleCreated
	if exists {
		action = ActionRoleUpdated
	}
	newValueJSON, _ := json.Marshal(role)
	m.logChangeUnsafe(action, "role", role.ID, oldValueJSON, string(newValueJSON), username)

	return nil
}

func (m *SQLiteManager) getRoleUnsafe(id string) (Role, bool) {
	var role Role
	var parentID sql.NullString
	var createdAt, updatedAt int64
	var isBuiltIn int

	err := m.db.QueryRow(`
		SELECT id, name, description, parent_id, is_built_in, priority, created_at, updated_at
		FROM rbac_roles
		WHERE id = ?
	`, id).Scan(&role.ID, &role.Name, &role.Description, &parentID, &isBuiltIn, &role.Priority, &createdAt, &updatedAt)

	if err != nil {
		return Role{}, false
	}

	role.ParentID = parentID.String
	role.IsBuiltIn = isBuiltIn == 1
	role.CreatedAt = time.Unix(createdAt, 0)
	role.UpdatedAt = time.Unix(updatedAt, 0)
	role.Permissions = m.loadRolePermissions(role.ID)

	return role, true
}

// DeleteRole removes a role by ID.
func (m *SQLiteManager) DeleteRole(id string) error {
	return m.DeleteRoleWithContext(id, "")
}

// DeleteRoleWithContext removes a role with audit context.
func (m *SQLiteManager) DeleteRoleWithContext(id string, username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if it's a built-in role
	var isBuiltIn int
	err := m.db.QueryRow("SELECT is_built_in FROM rbac_roles WHERE id = ?", id).Scan(&isBuiltIn)
	if err == sql.ErrNoRows {
		return nil // Already deleted
	}
	if err != nil {
		return err
	}
	if isBuiltIn == 1 {
		return fmt.Errorf("cannot delete built-in role: %s", id)
	}

	// Get old value for changelog
	oldRole, _ := m.getRoleUnsafe(id)
	oldValueJSON, _ := json.Marshal(oldRole)

	// Delete role (cascades to permissions)
	_, err = m.db.Exec("DELETE FROM rbac_roles WHERE id = ?", id)
	if err != nil {
		return err
	}

	// Log change
	m.logChangeUnsafe(ActionRoleDeleted, "role", id, string(oldValueJSON), "", username)

	return nil
}

// GetUserAssignments returns all user role assignments.
func (m *SQLiteManager) GetUserAssignments() []UserRoleAssignment {
	assignments, err := m.GetUserAssignmentsWithError()
	if err != nil {
		log.Error().Err(err).Msg("Failed to query user assignments")
		return nil
	}
	return assignments
}

// GetUserAssignmentsWithError returns every known RBAC identity, including
// identities that currently have no roles, and preserves storage errors.
func (m *SQLiteManager) GetUserAssignmentsWithError() ([]UserRoleAssignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getUserAssignmentsUnsafe()
}

func (m *SQLiteManager) getUserAssignmentsUnsafe() ([]UserRoleAssignment, error) {
	// Collect identities first, then close rows before nested queries
	// (avoids holding the connection during nested queries with MaxOpenConns=1)
	rows, err := m.db.Query("SELECT username FROM rbac_users ORDER BY username")
	if err != nil {
		return nil, err
	}

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			rows.Close()
			return nil, err
		}
		usernames = append(usernames, username)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	var assignments []UserRoleAssignment
	for _, username := range usernames {
		assignment, _, err := m.getUserAssignmentWithErrorUnsafe(username)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}

	if assignments == nil {
		assignments = []UserRoleAssignment{}
	}
	return assignments, nil
}

func (m *SQLiteManager) getUserAssignmentUnsafe(username string) UserRoleAssignment {
	assignment, _, err := m.getUserAssignmentWithErrorUnsafe(username)
	if err != nil {
		log.Error().Err(err).Str("username", username).Msg("Failed to query user assignment")
		return UserRoleAssignment{Username: username}
	}
	return assignment
}

func (m *SQLiteManager) getUserAssignmentWithErrorUnsafe(username string) (UserRoleAssignment, bool, error) {
	var userUpdatedAt int64
	err := m.db.QueryRow(`
		SELECT updated_at
		FROM rbac_users
		WHERE username = ?
	`, username).Scan(&userUpdatedAt)
	if err == sql.ErrNoRows {
		return UserRoleAssignment{Username: username, RoleIDs: []string{}}, false, nil
	}
	if err != nil {
		return UserRoleAssignment{}, false, err
	}

	rows, err := m.db.Query(`
		SELECT role_id, updated_at
		FROM rbac_user_assignments
		WHERE username = ?
		ORDER BY role_id
	`, username)
	if err != nil {
		return UserRoleAssignment{}, false, err
	}
	defer rows.Close()

	roleIDs := []string{}
	latestUpdate := userUpdatedAt
	for rows.Next() {
		var roleID string
		var updatedAt int64
		if err := rows.Scan(&roleID, &updatedAt); err != nil {
			return UserRoleAssignment{}, false, err
		}
		roleIDs = append(roleIDs, roleID)
		if updatedAt > latestUpdate {
			latestUpdate = updatedAt
		}
	}
	if err := rows.Err(); err != nil {
		return UserRoleAssignment{}, false, err
	}

	return UserRoleAssignment{
		Username:  username,
		RoleIDs:   roleIDs,
		UpdatedAt: time.Unix(latestUpdate, 0),
	}, true, nil
}

// GetUserAssignment returns the role assignment for a user.
func (m *SQLiteManager) GetUserAssignment(username string) (UserRoleAssignment, bool) {
	assignment, ok, err := m.GetUserAssignmentWithError(username)
	if err != nil {
		log.Error().Err(err).Str("username", username).Msg("Failed to query user assignment")
		return UserRoleAssignment{}, false
	}
	return assignment, ok
}

// GetUserAssignmentWithError returns an assignment and preserves storage
// errors. A known identity with zero roles still exists.
func (m *SQLiteManager) GetUserAssignmentWithError(username string) (UserRoleAssignment, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getUserAssignmentWithErrorUnsafe(username)
}

// AssignRole adds a role to a user.
func (m *SQLiteManager) AssignRole(username string, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify role exists
	var count int
	if err := m.db.QueryRow("SELECT COUNT(*) FROM rbac_roles WHERE id = ?", roleID).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("role not found: %s", roleID)
	}

	now := time.Now().Unix()
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
		INSERT INTO rbac_users (username, updated_at)
		VALUES (?, ?)
		ON CONFLICT(username) DO UPDATE SET updated_at = excluded.updated_at
	`, username, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO rbac_user_assignments (username, role_id, updated_at)
		VALUES (?, ?, ?)
	`, username, roleID, now); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateUserRoles replaces all roles for a user.
func (m *SQLiteManager) UpdateUserRoles(username string, roleIDs []string) error {
	return m.UpdateUserRolesWithContext(username, roleIDs, "")
}

// UpdateUserRolesWithContext replaces all roles for a user with audit context.
func (m *SQLiteManager) UpdateUserRolesWithContext(username string, roleIDs []string, byUser string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify all roles exist
	for _, roleID := range roleIDs {
		var count int
		if err := m.db.QueryRow("SELECT COUNT(*) FROM rbac_roles WHERE id = ?", roleID).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("role not found: %s", roleID)
		}
	}

	// Get old assignment for changelog
	oldAssignment := m.getUserAssignmentUnsafe(username)
	oldValueJSON, _ := json.Marshal(oldAssignment)

	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	if _, err = tx.Exec(`
		INSERT INTO rbac_users (username, updated_at)
		VALUES (?, ?)
		ON CONFLICT(username) DO UPDATE SET updated_at = excluded.updated_at
	`, username, now); err != nil {
		return err
	}

	// Delete existing assignments
	_, err = tx.Exec("DELETE FROM rbac_user_assignments WHERE username = ?", username)
	if err != nil {
		return err
	}

	// Insert new assignments
	for _, roleID := range roleIDs {
		_, err = tx.Exec(`
			INSERT INTO rbac_user_assignments (username, role_id, updated_at)
			VALUES (?, ?, ?)
		`, username, roleID, now)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Log change
	newAssignment := UserRoleAssignment{Username: username, RoleIDs: roleIDs, UpdatedAt: time.Unix(now, 0)}
	newValueJSON, _ := json.Marshal(newAssignment)
	m.logChangeUnsafe(ActionUserRolesUpdate, "assignment", username, string(oldValueJSON), string(newValueJSON), byUser)

	return nil
}

// MigrateUserAssignment atomically moves a legacy identity alias to a
// canonical principal. Conflicting canonical roles fail closed rather than
// unioning grants and accidentally escalating access.
func (m *SQLiteManager) MigrateUserAssignment(fromUsername, toUsername string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fromUsername = strings.TrimSpace(fromUsername)
	toUsername = strings.TrimSpace(toUsername)
	if fromUsername == "" || toUsername == "" {
		return fmt.Errorf("source and destination usernames are required")
	}
	if fromUsername == toUsername {
		return nil
	}

	source, sourceExists, err := m.getUserAssignmentWithErrorUnsafe(fromUsername)
	if err != nil {
		return err
	}
	if !sourceExists {
		return nil
	}
	target, targetExists, err := m.getUserAssignmentWithErrorUnsafe(toUsername)
	if err != nil {
		return err
	}
	sourceRoleIDs := append([]string{}, source.RoleIDs...)
	targetRoleIDs := append([]string{}, target.RoleIDs...)
	sort.Strings(sourceRoleIDs)
	sort.Strings(targetRoleIDs)
	if targetExists && len(targetRoleIDs) > 0 &&
		strings.Join(sourceRoleIDs, "\x00") != strings.Join(targetRoleIDs, "\x00") {
		return fmt.Errorf("canonical assignment for %q conflicts with legacy identity %q", toUsername, fromUsername)
	}

	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	if _, err := tx.Exec(`
		INSERT INTO rbac_users (username, updated_at)
		VALUES (?, ?)
		ON CONFLICT(username) DO UPDATE SET updated_at = excluded.updated_at
	`, toUsername, now); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM rbac_user_assignments WHERE username = ?", toUsername); err != nil {
		return err
	}
	for _, roleID := range sourceRoleIDs {
		if _, err := tx.Exec(`
			INSERT INTO rbac_user_assignments (username, role_id, updated_at)
			VALUES (?, ?, ?)
		`, toUsername, roleID, now); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("DELETE FROM rbac_user_assignments WHERE username = ?", fromUsername); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM rbac_users WHERE username = ?", fromUsername); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	oldValueJSON, _ := json.Marshal(source)
	newValueJSON, _ := json.Marshal(UserRoleAssignment{
		Username:  toUsername,
		RoleIDs:   sourceRoleIDs,
		UpdatedAt: time.Unix(now, 0),
	})
	m.logChangeUnsafe(
		ActionUserRolesUpdate,
		"assignment",
		toUsername,
		string(oldValueJSON),
		string(newValueJSON),
		"identity-migration",
	)
	return nil
}

// RemoveRole removes a role from a user.
func (m *SQLiteManager) RemoveRole(username string, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		DELETE FROM rbac_user_assignments
		WHERE username = ? AND role_id = ?
	`, username, roleID)

	return err
}

// GetUserPermissions returns the effective permissions for a user.
func (m *SQLiteManager) GetUserPermissions(username string) []Permission {
	permissions, err := m.GetUserPermissionsWithError(username)
	if err != nil {
		log.Error().Err(err).Str("username", username).Msg("Failed to query user permissions")
		return nil
	}
	return permissions
}

// GetUserPermissionsWithError returns effective permissions and preserves
// storage failures.
func (m *SQLiteManager) GetUserPermissionsWithError(username string) ([]Permission, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assignment, _, err := m.getUserAssignmentWithErrorUnsafe(username)
	if err != nil {
		return nil, err
	}
	if len(assignment.RoleIDs) == 0 {
		return []Permission{}, nil
	}

	// Collect unique permissions from all assigned roles
	permMap := make(map[string]Permission)
	for _, roleID := range assignment.RoleIDs {
		perms, err := m.loadRolePermissionsWithError(roleID)
		if err != nil {
			return nil, err
		}
		for _, perm := range perms {
			key := perm.Action + ":" + perm.Resource + ":" + perm.GetEffect()
			permMap[key] = perm
		}
	}

	perms := make([]Permission, 0, len(permMap))
	for _, perm := range permMap {
		perms = append(perms, perm)
	}

	return perms, nil
}

// GetRoleWithInheritance returns a role and all inherited permissions.
func (m *SQLiteManager) GetRoleWithInheritance(id string) (Role, []Permission, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	role, exists := m.getRoleUnsafe(id)
	if !exists {
		return Role{}, nil, false
	}

	// Collect all permissions including inherited
	allPerms := m.collectInheritedPermissions(id, make(map[string]bool))

	return role, allPerms, true
}

func (m *SQLiteManager) collectInheritedPermissions(roleID string, visited map[string]bool) []Permission {
	if visited[roleID] {
		return nil // Circular reference protection
	}
	visited[roleID] = true

	role, exists := m.getRoleUnsafe(roleID)
	if !exists {
		return nil
	}

	var allPerms []Permission

	// First, get parent permissions (inherited)
	if role.ParentID != "" {
		allPerms = m.collectInheritedPermissions(role.ParentID, visited)
	}

	// Then add role's own permissions (can override parent)
	allPerms = append(allPerms, role.Permissions...)

	return allPerms
}

// checkCircularInheritanceUnsafe checks if setting parentID on roleID would create a cycle.
// Must be called with lock held.
func (m *SQLiteManager) checkCircularInheritanceUnsafe(roleID, parentID string) error {
	const maxDepth = 10
	visited := make(map[string]bool)
	visited[roleID] = true // The role we're updating

	// Walk up the parent chain from parentID
	current := parentID
	for depth := 0; depth < maxDepth && current != ""; depth++ {
		if visited[current] {
			return fmt.Errorf("circular inheritance detected: %s -> %s creates a cycle", roleID, parentID)
		}
		visited[current] = true

		// Get parent of current
		var parent *string
		err := m.db.QueryRow("SELECT parent_id FROM rbac_roles WHERE id = ?", current).Scan(&parent)
		if err != nil {
			break // Role doesn't exist or has no parent
		}
		if parent == nil {
			break
		}
		current = *parent
	}

	return nil
}

// GetRolesWithInheritance returns user's roles with full inheritance chain.
func (m *SQLiteManager) GetRolesWithInheritance(username string) []Role {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assignment := m.getUserAssignmentUnsafe(username)
	if len(assignment.RoleIDs) == 0 {
		return nil
	}

	var roles []Role
	visited := make(map[string]bool)

	for _, roleID := range assignment.RoleIDs {
		m.collectRoleChain(roleID, &roles, visited)
	}

	return roles
}

func (m *SQLiteManager) collectRoleChain(roleID string, roles *[]Role, visited map[string]bool) {
	if visited[roleID] {
		return // Circular reference protection
	}
	visited[roleID] = true

	role, exists := m.getRoleUnsafe(roleID)
	if !exists {
		return
	}

	// First collect parent
	if role.ParentID != "" {
		m.collectRoleChain(role.ParentID, roles, visited)
	}

	*roles = append(*roles, role)
}

// GetChangeLogs returns recent change logs.
func (m *SQLiteManager) GetChangeLogs(limit int, offset int) []RBACChangeLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := m.db.Query(`
		SELECT id, action, entity_type, entity_id, old_value, new_value, user, timestamp
		FROM rbac_changelog
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query change logs")
		return nil
	}
	defer rows.Close()

	return m.scanChangeLogs(rows)
}

// GetChangeLogsForEntity returns change logs for a specific entity.
func (m *SQLiteManager) GetChangeLogsForEntity(entityType, entityID string) []RBACChangeLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(`
		SELECT id, action, entity_type, entity_id, old_value, new_value, user, timestamp
		FROM rbac_changelog
		WHERE entity_type = ? AND entity_id = ?
		ORDER BY timestamp DESC
	`, entityType, entityID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query change logs")
		return nil
	}
	defer rows.Close()

	return m.scanChangeLogs(rows)
}

func (m *SQLiteManager) scanChangeLogs(rows *sql.Rows) []RBACChangeLog {
	var logs []RBACChangeLog
	for rows.Next() {
		var entry RBACChangeLog
		var oldValue, newValue, user sql.NullString
		var timestamp int64

		if err := rows.Scan(&entry.ID, &entry.Action, &entry.EntityType, &entry.EntityID,
			&oldValue, &newValue, &user, &timestamp); err != nil {
			log.Error().Err(err).Msg("Failed to scan change log")
			continue
		}

		entry.OldValue = oldValue.String
		entry.NewValue = newValue.String
		entry.User = user.String
		entry.Timestamp = time.Unix(timestamp, 0)

		logs = append(logs, entry)
	}

	return logs
}

func (m *SQLiteManager) logChangeUnsafe(action, entityType, entityID, oldValue, newValue, user string) {
	_, err := m.db.Exec(`
		INSERT INTO rbac_changelog (id, action, entity_type, entity_id, old_value, new_value, user, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.New().String(), action, entityType, entityID, nullString(oldValue), nullString(newValue), nullString(user), time.Now().Unix())

	if err != nil {
		log.Error().Err(err).Str("action", action).Msg("Failed to log RBAC change")
	}
}

// migrateFromFiles migrates data from file-based storage.
func (m *SQLiteManager) migrateFromFiles(dataDir string) error {
	rolesFile, err := securityutil.JoinStorageLeaf(dataDir, "rbac_roles.json")
	if err != nil {
		return fmt.Errorf("resolve legacy roles path: %w", err)
	}
	rolesBackup, err := securityutil.JoinStorageLeaf(dataDir, "rbac_roles.json.bak")
	if err != nil {
		return fmt.Errorf("resolve legacy roles backup path: %w", err)
	}
	assignmentsFile, err := securityutil.JoinStorageLeaf(dataDir, "rbac_assignments.json")
	if err != nil {
		return fmt.Errorf("resolve legacy assignments path: %w", err)
	}
	assignmentsBackup, err := securityutil.JoinStorageLeaf(dataDir, "rbac_assignments.json.bak")
	if err != nil {
		return fmt.Errorf("resolve legacy assignments backup path: %w", err)
	}

	roles, rolesExist, err := readLegacyRBACFile[Role](rolesFile, "roles")
	if err != nil {
		return err
	}
	assignments, assignmentsExist, err := readLegacyRBACFile[UserRoleAssignment](assignmentsFile, "assignments")
	if err != nil {
		return err
	}
	if !rolesExist && !assignmentsExist {
		return nil
	}
	if rolesExist {
		rolesBackup, err = availableLegacyBackupPath(rolesBackup)
		if err != nil {
			return err
		}
	}
	if assignmentsExist {
		assignmentsBackup, err = availableLegacyBackupPath(assignmentsBackup)
		if err != nil {
			return err
		}
	}

	if err := m.importLegacyRBAC(roles, assignments); err != nil {
		return err
	}

	// Source files remain untouched until the complete import transaction has
	// committed. A rename failure is returned so an operator can resolve it;
	// the next start safely verifies the imported records before retrying.
	if rolesExist {
		if err := securityutil.RenameSecureStorageFile(rolesFile, rolesBackup); err != nil {
			return fmt.Errorf("archive migrated legacy roles: %w", err)
		}
	}
	if assignmentsExist {
		if err := securityutil.RenameSecureStorageFile(assignmentsFile, assignmentsBackup); err != nil {
			return fmt.Errorf("archive migrated legacy assignments: %w", err)
		}
	}

	log.Info().
		Int("roles", len(roles)).
		Int("assignments", len(assignments)).
		Msg("Migrated legacy RBAC data to SQLite")
	return nil
}

func readLegacyRBACFile[T any](path, kind string) ([]T, bool, error) {
	data, err := securityutil.ReadSecureStorageFile(path, maxLegacyRBACFileSize)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read legacy RBAC %s: %w", kind, err)
	}

	var records []T
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, true, fmt.Errorf("decode legacy RBAC %s: %w", kind, err)
	}
	if records == nil {
		records = []T{}
	}
	return records, true, nil
}

func availableLegacyBackupPath(path string) (string, error) {
	for index := 0; index < 1000; index++ {
		candidate := path
		if index > 0 {
			candidate = fmt.Sprintf("%s.%d", path, index)
		}
		_, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("inspect legacy RBAC backup %s: %w", filepath.Base(candidate), err)
		}
	}
	return "", fmt.Errorf("too many legacy RBAC backups for %s", filepath.Base(path))
}

func (m *SQLiteManager) importLegacyRBAC(roles []Role, assignments []UserRoleAssignment) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	existingRoles, err := loadMigrationRoles(tx)
	if err != nil {
		return fmt.Errorf("load current roles: %w", err)
	}

	legacyRoles := make(map[string]Role, len(roles))
	for _, role := range roles {
		if strings.TrimSpace(role.ID) == "" {
			return fmt.Errorf("legacy role has an empty ID")
		}
		if previous, duplicate := legacyRoles[role.ID]; duplicate {
			if canonicalRole(previous) != canonicalRole(role) {
				return fmt.Errorf("conflicting duplicate legacy role %q", role.ID)
			}
			continue
		}
		legacyRoles[role.ID] = role
	}

	parentByRole := make(map[string]string, len(existingRoles)+len(legacyRoles))
	for id, role := range existingRoles {
		parentByRole[id] = role.ParentID
	}
	for id, role := range legacyRoles {
		if role.IsBuiltIn {
			existing, ok := existingRoles[id]
			if !ok || !existing.IsBuiltIn {
				return fmt.Errorf("legacy built-in role %q is not recognized", id)
			}
			if canonicalRole(existing) != canonicalRole(role) {
				return fmt.Errorf("legacy built-in role %q differs from the canonical definition", id)
			}
			continue
		}
		if existing, ok := existingRoles[id]; ok && canonicalRole(existing) != canonicalRole(role) {
			return fmt.Errorf("legacy role %q conflicts with current v6 data", id)
		}
		parentByRole[id] = role.ParentID
	}
	for id, parentID := range parentByRole {
		if parentID != "" {
			if _, ok := parentByRole[parentID]; !ok {
				return fmt.Errorf("role %q references missing parent %q", id, parentID)
			}
		}
	}
	if err := validateRoleParentGraph(parentByRole); err != nil {
		return err
	}

	newRoleIDs := make([]string, 0, len(legacyRoles))
	for id, role := range legacyRoles {
		if role.IsBuiltIn {
			continue
		}
		if _, exists := existingRoles[id]; exists {
			continue
		}
		if strings.TrimSpace(role.Name) == "" {
			return fmt.Errorf("legacy role %q has an empty name", id)
		}
		createdAt := role.CreatedAt.Unix()
		updatedAt := role.UpdatedAt.Unix()
		now := time.Now().Unix()
		if role.CreatedAt.IsZero() {
			createdAt = now
		}
		if role.UpdatedAt.IsZero() {
			updatedAt = createdAt
		}
		if _, err := tx.Exec(`
			INSERT INTO rbac_roles
				(id, name, description, parent_id, is_built_in, priority, created_at, updated_at)
			VALUES (?, ?, ?, NULL, 0, ?, ?, ?)
		`, role.ID, role.Name, role.Description, role.Priority, createdAt, updatedAt); err != nil {
			return fmt.Errorf("insert legacy role %q: %w", role.ID, err)
		}
		for _, permission := range role.Permissions {
			if strings.TrimSpace(permission.Action) == "" || strings.TrimSpace(permission.Resource) == "" {
				return fmt.Errorf("legacy role %q has an invalid permission", role.ID)
			}
			effect := permission.GetEffect()
			if effect != EffectAllow && effect != EffectDeny {
				return fmt.Errorf("legacy role %q has invalid permission effect %q", role.ID, effect)
			}
			var conditions interface{}
			if len(permission.Conditions) > 0 {
				encoded, err := json.Marshal(permission.Conditions)
				if err != nil {
					return fmt.Errorf("encode legacy role %q conditions: %w", role.ID, err)
				}
				conditions = string(encoded)
			}
			if _, err := tx.Exec(`
				INSERT INTO rbac_permissions (role_id, action, resource, effect, conditions)
				VALUES (?, ?, ?, ?, ?)
			`, role.ID, permission.Action, permission.Resource, effect, conditions); err != nil {
				return fmt.Errorf("insert permission for legacy role %q: %w", role.ID, err)
			}
		}
		newRoleIDs = append(newRoleIDs, id)
	}
	for _, id := range newRoleIDs {
		if parentID := legacyRoles[id].ParentID; parentID != "" {
			if _, err := tx.Exec("UPDATE rbac_roles SET parent_id = ? WHERE id = ?", parentID, id); err != nil {
				return fmt.Errorf("set parent for legacy role %q: %w", id, err)
			}
		}
	}

	knownRoleIDs := make(map[string]struct{}, len(parentByRole))
	for id := range parentByRole {
		knownRoleIDs[id] = struct{}{}
	}
	legacyAssignments := make(map[string][]string, len(assignments))
	for _, assignment := range assignments {
		username := strings.TrimSpace(assignment.Username)
		if username == "" {
			return fmt.Errorf("legacy assignment has an empty username")
		}
		roleIDs, err := normalizedRoleIDs(assignment.RoleIDs, knownRoleIDs)
		if err != nil {
			return fmt.Errorf("legacy assignment for %q: %w", username, err)
		}
		if previous, duplicate := legacyAssignments[username]; duplicate {
			if strings.Join(previous, "\x00") != strings.Join(roleIDs, "\x00") {
				return fmt.Errorf("conflicting duplicate legacy assignment for %q", username)
			}
			continue
		}
		legacyAssignments[username] = roleIDs
	}

	usernames := make([]string, 0, len(legacyAssignments))
	for username := range legacyAssignments {
		usernames = append(usernames, username)
	}
	sort.Strings(usernames)
	for _, username := range usernames {
		roleIDs := legacyAssignments[username]
		currentRoleIDs, exists, err := loadMigrationAssignment(tx, username)
		if err != nil {
			return fmt.Errorf("load current assignment for %q: %w", username, err)
		}
		if exists {
			if strings.Join(currentRoleIDs, "\x00") != strings.Join(roleIDs, "\x00") {
				return fmt.Errorf("legacy assignment for %q conflicts with current v6 data", username)
			}
			continue
		}
		now := time.Now().Unix()
		if _, err := tx.Exec(
			"INSERT INTO rbac_users (username, updated_at) VALUES (?, ?)",
			username,
			now,
		); err != nil {
			return fmt.Errorf("insert legacy identity %q: %w", username, err)
		}
		for _, roleID := range roleIDs {
			if _, err := tx.Exec(`
				INSERT INTO rbac_user_assignments (username, role_id, updated_at)
				VALUES (?, ?, ?)
			`, username, roleID, now); err != nil {
				return fmt.Errorf("insert legacy assignment for %q: %w", username, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit legacy RBAC import: %w", err)
	}
	return nil
}

func loadMigrationRoles(tx *sql.Tx) (map[string]Role, error) {
	rows, err := tx.Query(`
		SELECT id, name, description, parent_id, is_built_in, priority
		FROM rbac_roles
	`)
	if err != nil {
		return nil, err
	}
	roles := make(map[string]Role)
	for rows.Next() {
		var role Role
		var parentID sql.NullString
		var isBuiltIn int
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &parentID, &isBuiltIn, &role.Priority); err != nil {
			rows.Close()
			return nil, err
		}
		role.ParentID = parentID.String
		role.IsBuiltIn = isBuiltIn == 1
		roles[role.ID] = role
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	permissionRows, err := tx.Query(`
		SELECT role_id, action, resource, effect, conditions
		FROM rbac_permissions
		ORDER BY role_id, id
	`)
	if err != nil {
		return nil, err
	}
	defer permissionRows.Close()
	for permissionRows.Next() {
		var roleID string
		var permission Permission
		var conditions sql.NullString
		if err := permissionRows.Scan(&roleID, &permission.Action, &permission.Resource, &permission.Effect, &conditions); err != nil {
			return nil, err
		}
		if conditions.Valid && conditions.String != "" {
			if err := json.Unmarshal([]byte(conditions.String), &permission.Conditions); err != nil {
				return nil, fmt.Errorf("decode conditions for role %q: %w", roleID, err)
			}
		}
		role, ok := roles[roleID]
		if !ok {
			return nil, fmt.Errorf("permission references missing role %q", roleID)
		}
		role.Permissions = append(role.Permissions, permission)
		roles[roleID] = role
	}
	if err := permissionRows.Err(); err != nil {
		return nil, err
	}
	return roles, nil
}

func loadMigrationAssignment(tx *sql.Tx, username string) ([]string, bool, error) {
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM rbac_users WHERE username = ?", username).Scan(&count); err != nil {
		return nil, false, err
	}
	if count == 0 {
		return nil, false, nil
	}
	rows, err := tx.Query(`
		SELECT role_id
		FROM rbac_user_assignments
		WHERE username = ?
		ORDER BY role_id
	`, username)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	roleIDs := []string{}
	for rows.Next() {
		var roleID string
		if err := rows.Scan(&roleID); err != nil {
			return nil, false, err
		}
		roleIDs = append(roleIDs, roleID)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return roleIDs, true, nil
}

func normalizedRoleIDs(roleIDs []string, known map[string]struct{}) ([]string, error) {
	unique := make(map[string]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		if _, exists := known[roleID]; !exists {
			return nil, fmt.Errorf("references missing role %q", roleID)
		}
		unique[roleID] = struct{}{}
	}
	normalized := make([]string, 0, len(unique))
	for roleID := range unique {
		normalized = append(normalized, roleID)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func canonicalRole(role Role) string {
	permissions := make([]string, 0, len(role.Permissions))
	for _, permission := range role.Permissions {
		var conditions []byte
		if len(permission.Conditions) > 0 {
			conditions, _ = json.Marshal(permission.Conditions)
		}
		permissions = append(permissions, strings.Join([]string{
			permission.Action,
			permission.Resource,
			permission.GetEffect(),
			string(conditions),
		}, "\x00"))
	}
	sort.Strings(permissions)
	return strings.Join([]string{
		role.ID,
		role.Name,
		role.Description,
		role.ParentID,
		fmt.Sprintf("%t", role.IsBuiltIn),
		fmt.Sprintf("%d", role.Priority),
		strings.Join(permissions, "\x01"),
	}, "\x02")
}

func validateRoleParentGraph(parentByRole map[string]string) error {
	const (
		unvisited = iota
		visiting
		visited
	)
	state := make(map[string]int, len(parentByRole))
	var visit func(string) error
	visit = func(roleID string) error {
		switch state[roleID] {
		case visiting:
			return fmt.Errorf("role inheritance cycle contains %q", roleID)
		case visited:
			return nil
		}
		state[roleID] = visiting
		if parentID := parentByRole[roleID]; parentID != "" {
			if err := visit(parentID); err != nil {
				return err
			}
		}
		state[roleID] = visited
		return nil
	}
	for roleID := range parentByRole {
		if err := visit(roleID); err != nil {
			return err
		}
	}
	return nil
}

// Helper to convert empty strings to nil for nullable columns
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
