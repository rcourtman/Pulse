package auth

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

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

	// Ensure directory exists
	rbacDir := filepath.Join(cfg.DataDir, "rbac")
	if err := os.MkdirAll(rbacDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create rbac directory: %w", err)
	}

	dbPath := filepath.Join(rbacDir, "rbac.db")

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
		if err := m.migrateFromFiles(cfg.DataDir); err != nil {
			log.Warn().Err(err).Msg("Failed to migrate RBAC from files (may not exist)")
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
	CREATE TABLE IF NOT EXISTS rbac_user_assignments (
		username TEXT NOT NULL,
		role_id TEXT NOT NULL,
		updated_at INTEGER NOT NULL,
		PRIMARY KEY (username, role_id),
		FOREIGN KEY (role_id) REFERENCES rbac_roles(id) ON DELETE CASCADE
	);

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
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(`
		SELECT id, name, description, parent_id, is_built_in, priority, created_at, updated_at
		FROM rbac_roles
		ORDER BY name
	`)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query roles")
		return nil
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
			log.Error().Err(err).Msg("Failed to scan role")
			continue
		}

		role.ParentID = parentID.String
		role.IsBuiltIn = isBuiltIn == 1
		role.CreatedAt = time.Unix(createdAt, 0)
		role.UpdatedAt = time.Unix(updatedAt, 0)

		roles = append(roles, role)
	}
	rows.Close()

	// Load permissions after releasing the connection
	for i := range roles {
		roles[i].Permissions = m.loadRolePermissions(roles[i].ID)
	}

	return roles
}

func (m *SQLiteManager) loadRolePermissions(roleID string) []Permission {
	rows, err := m.db.Query(`
		SELECT action, resource, effect, conditions
		FROM rbac_permissions
		WHERE role_id = ?
	`, roleID)
	if err != nil {
		log.Error().Err(err).Str("roleId", roleID).Msg("Failed to query permissions")
		return nil
	}
	defer rows.Close()

	var perms []Permission
	for rows.Next() {
		var perm Permission
		var conditions sql.NullString

		if err := rows.Scan(&perm.Action, &perm.Resource, &perm.Effect, &conditions); err != nil {
			log.Error().Err(err).Msg("Failed to scan permission")
			continue
		}

		if conditions.Valid && conditions.String != "" {
			json.Unmarshal([]byte(conditions.String), &perm.Conditions)
		}

		perms = append(perms, perm)
	}

	return perms
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
	defer tx.Rollback()

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
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect usernames first, then close rows before nested queries
	// (avoids holding the connection during nested queries with MaxOpenConns=1)
	rows, err := m.db.Query("SELECT DISTINCT username FROM rbac_user_assignments")
	if err != nil {
		log.Error().Err(err).Msg("Failed to query user assignments")
		return nil
	}

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			continue
		}
		usernames = append(usernames, username)
	}
	rows.Close()

	var assignments []UserRoleAssignment
	for _, username := range usernames {
		assignment := m.getUserAssignmentUnsafe(username)
		if len(assignment.RoleIDs) > 0 {
			assignments = append(assignments, assignment)
		}
	}

	return assignments
}

func (m *SQLiteManager) getUserAssignmentUnsafe(username string) UserRoleAssignment {
	rows, err := m.db.Query(`
		SELECT role_id, updated_at
		FROM rbac_user_assignments
		WHERE username = ?
	`, username)
	if err != nil {
		return UserRoleAssignment{Username: username}
	}
	defer rows.Close()

	var roleIDs []string
	var latestUpdate int64
	for rows.Next() {
		var roleID string
		var updatedAt int64
		if err := rows.Scan(&roleID, &updatedAt); err != nil {
			continue
		}
		roleIDs = append(roleIDs, roleID)
		if updatedAt > latestUpdate {
			latestUpdate = updatedAt
		}
	}

	return UserRoleAssignment{
		Username:  username,
		RoleIDs:   roleIDs,
		UpdatedAt: time.Unix(latestUpdate, 0),
	}
}

// GetUserAssignment returns the role assignment for a user.
func (m *SQLiteManager) GetUserAssignment(username string) (UserRoleAssignment, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assignment := m.getUserAssignmentUnsafe(username)
	return assignment, len(assignment.RoleIDs) > 0
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
	_, err := m.db.Exec(`
		INSERT OR IGNORE INTO rbac_user_assignments (username, role_id, updated_at)
		VALUES (?, ?, ?)
	`, username, roleID, now)

	return err
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
	defer tx.Rollback()

	// Delete existing assignments
	_, err = tx.Exec("DELETE FROM rbac_user_assignments WHERE username = ?", username)
	if err != nil {
		return err
	}

	// Insert new assignments
	now := time.Now().Unix()
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
	m.mu.RLock()
	defer m.mu.RUnlock()

	assignment := m.getUserAssignmentUnsafe(username)
	if len(assignment.RoleIDs) == 0 {
		return nil
	}

	// Collect unique permissions from all assigned roles
	permMap := make(map[string]Permission)
	for _, roleID := range assignment.RoleIDs {
		perms := m.loadRolePermissions(roleID)
		for _, perm := range perms {
			key := perm.Action + ":" + perm.Resource + ":" + perm.GetEffect()
			permMap[key] = perm
		}
	}

	var perms []Permission
	for _, perm := range permMap {
		perms = append(perms, perm)
	}

	return perms
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
	rolesFile := filepath.Join(dataDir, "rbac_roles.json")
	assignmentsFile := filepath.Join(dataDir, "rbac_assignments.json")

	// Check if migration is needed
	var roleCount int
	m.db.QueryRow("SELECT COUNT(*) FROM rbac_roles WHERE is_built_in = 0").Scan(&roleCount)
	if roleCount > 0 {
		return nil // Already have custom roles, skip migration
	}

	// Migrate roles
	if data, err := os.ReadFile(rolesFile); err == nil {
		var roles []Role
		if err := json.Unmarshal(data, &roles); err == nil {
			for _, role := range roles {
				if !role.IsBuiltIn {
					if err := m.SaveRoleWithContext(role, "migration"); err != nil {
						log.Warn().Err(err).Str("roleId", role.ID).Msg("Failed to migrate role")
					}
				}
			}
			log.Info().Int("count", len(roles)).Msg("Migrated roles from file")

			// Rename old file
			os.Rename(rolesFile, rolesFile+".bak")
		}
	}

	// Migrate assignments
	if data, err := os.ReadFile(assignmentsFile); err == nil {
		var assignments []UserRoleAssignment
		if err := json.Unmarshal(data, &assignments); err == nil {
			for _, a := range assignments {
				if err := m.UpdateUserRolesWithContext(a.Username, a.RoleIDs, "migration"); err != nil {
					log.Warn().Err(err).Str("username", a.Username).Msg("Failed to migrate assignment")
				}
			}
			log.Info().Int("count", len(assignments)).Msg("Migrated assignments from file")

			// Rename old file
			os.Rename(assignmentsFile, assignmentsFile+".bak")
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
