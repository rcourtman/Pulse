package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileManager implements the Manager interface with file-based persistence.
type FileManager struct {
	mu          sync.RWMutex
	dataDir     string
	roles       map[string]Role
	assignments map[string]UserRoleAssignment
}

// NewFileManager creates a new file-based RBAC manager.
func NewFileManager(dataDir string) (*FileManager, error) {
	m := &FileManager{
		dataDir:     dataDir,
		roles:       make(map[string]Role),
		assignments: make(map[string]UserRoleAssignment),
	}

	// Initialize built-in roles
	m.initBuiltInRoles()

	// Load persisted data
	if err := m.load(); err != nil {
		// Non-fatal - just start fresh if no data exists
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load RBAC data: %w", err)
		}
	}

	return m, nil
}

func (m *FileManager) initBuiltInRoles() {
	now := time.Now()

	builtInRoles := []Role{
		{
			ID:          RoleAdmin,
			Name:        "Administrator",
			Description: "Full administrative access to all features",
			Permissions: []Permission{
				{Action: "admin", Resource: "*"},
			},
			IsBuiltIn: true,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:          RoleOperator,
			Name:        "Operator",
			Description: "Can manage nodes and perform operational tasks",
			Permissions: []Permission{
				{Action: "read", Resource: "*"},
				{Action: "write", Resource: "nodes"},
				{Action: "write", Resource: "vms"},
				{Action: "write", Resource: "containers"},
				{Action: "write", Resource: "alerts"},
			},
			IsBuiltIn: true,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:          RoleViewer,
			Name:        "Viewer",
			Description: "Read-only access to monitoring data",
			Permissions: []Permission{
				{Action: "read", Resource: "*"},
			},
			IsBuiltIn: true,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:          RoleAuditor,
			Name:        "Auditor",
			Description: "Access to audit logs and compliance data",
			Permissions: []Permission{
				{Action: "read", Resource: "audit_logs"},
				{Action: "read", Resource: "nodes"},
				{Action: "read", Resource: "alerts"},
				{Action: "read", Resource: "compliance"},
			},
			IsBuiltIn: true,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	for _, role := range builtInRoles {
		m.roles[role.ID] = role
	}
}

func (m *FileManager) rolesFile() string {
	return filepath.Join(m.dataDir, "rbac_roles.json")
}

func (m *FileManager) assignmentsFile() string {
	return filepath.Join(m.dataDir, "rbac_assignments.json")
}

func (m *FileManager) load() error {
	// Load custom roles (built-in roles are always initialized)
	if data, err := os.ReadFile(m.rolesFile()); err == nil {
		var roles []Role
		if err := json.Unmarshal(data, &roles); err != nil {
			return err
		}
		for _, role := range roles {
			if !role.IsBuiltIn {
				m.roles[role.ID] = role
			}
		}
	}

	// Load assignments
	if data, err := os.ReadFile(m.assignmentsFile()); err == nil {
		var assignments []UserRoleAssignment
		if err := json.Unmarshal(data, &assignments); err != nil {
			return err
		}
		for _, a := range assignments {
			m.assignments[a.Username] = a
		}
	}

	return nil
}

func (m *FileManager) saveRoles() error {
	roles := make([]Role, 0, len(m.roles))
	for _, role := range m.roles {
		roles = append(roles, role)
	}

	data, err := json.MarshalIndent(roles, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.rolesFile(), data, 0600)
}

func (m *FileManager) saveAssignments() error {
	assignments := make([]UserRoleAssignment, 0, len(m.assignments))
	for _, a := range m.assignments {
		assignments = append(assignments, a)
	}

	data, err := json.MarshalIndent(assignments, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.assignmentsFile(), data, 0600)
}

// GetRoles returns all roles.
func (m *FileManager) GetRoles() []Role {
	m.mu.RLock()
	defer m.mu.RUnlock()

	roles := make([]Role, 0, len(m.roles))
	for _, role := range m.roles {
		roles = append(roles, role)
	}
	return roles
}

// GetRole returns a role by ID.
func (m *FileManager) GetRole(id string) (Role, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	role, ok := m.roles[id]
	return role, ok
}

// SaveRole creates or updates a role.
func (m *FileManager) SaveRole(role Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cannot modify built-in roles
	if existing, ok := m.roles[role.ID]; ok && existing.IsBuiltIn {
		return fmt.Errorf("cannot modify built-in role: %s", role.ID)
	}

	role.UpdatedAt = time.Now()
	if role.CreatedAt.IsZero() {
		role.CreatedAt = role.UpdatedAt
	}

	m.roles[role.ID] = role
	return m.saveRoles()
}

// DeleteRole removes a role by ID.
func (m *FileManager) DeleteRole(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	role, ok := m.roles[id]
	if !ok {
		return nil // Already deleted
	}

	if role.IsBuiltIn {
		return fmt.Errorf("cannot delete built-in role: %s", id)
	}

	delete(m.roles, id)
	return m.saveRoles()
}

// GetUserAssignments returns all user role assignments.
func (m *FileManager) GetUserAssignments() []UserRoleAssignment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assignments := make([]UserRoleAssignment, 0, len(m.assignments))
	for _, a := range m.assignments {
		assignments = append(assignments, a)
	}
	return assignments
}

// GetUserAssignment returns the role assignment for a user.
func (m *FileManager) GetUserAssignment(username string) (UserRoleAssignment, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	a, ok := m.assignments[username]
	return a, ok
}

// AssignRole adds a role to a user.
func (m *FileManager) AssignRole(username string, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify role exists
	if _, ok := m.roles[roleID]; !ok {
		return fmt.Errorf("role not found: %s", roleID)
	}

	a, ok := m.assignments[username]
	if !ok {
		a = UserRoleAssignment{
			Username: username,
			RoleIDs:  []string{},
		}
	}

	// Check if already assigned
	for _, id := range a.RoleIDs {
		if id == roleID {
			return nil // Already assigned
		}
	}

	a.RoleIDs = append(a.RoleIDs, roleID)
	a.UpdatedAt = time.Now()
	m.assignments[username] = a

	return m.saveAssignments()
}

// UpdateUserRoles replaces all roles for a user.
func (m *FileManager) UpdateUserRoles(username string, roleIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Verify all roles exist
	for _, roleID := range roleIDs {
		if _, ok := m.roles[roleID]; !ok {
			return fmt.Errorf("role not found: %s", roleID)
		}
	}

	m.assignments[username] = UserRoleAssignment{
		Username:  username,
		RoleIDs:   roleIDs,
		UpdatedAt: time.Now(),
	}

	return m.saveAssignments()
}

// RemoveRole removes a role from a user.
func (m *FileManager) RemoveRole(username string, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.assignments[username]
	if !ok {
		return nil // No assignment exists
	}

	newRoles := make([]string, 0, len(a.RoleIDs))
	for _, id := range a.RoleIDs {
		if id != roleID {
			newRoles = append(newRoles, id)
		}
	}

	a.RoleIDs = newRoles
	a.UpdatedAt = time.Now()
	m.assignments[username] = a

	return m.saveAssignments()
}

// GetUserPermissions returns the effective permissions for a user.
func (m *FileManager) GetUserPermissions(username string) []Permission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	a, ok := m.assignments[username]
	if !ok {
		return nil
	}

	// Collect unique permissions from all assigned roles
	permMap := make(map[string]Permission)
	for _, roleID := range a.RoleIDs {
		if role, ok := m.roles[roleID]; ok {
			for _, perm := range role.Permissions {
				key := perm.Action + ":" + perm.Resource
				permMap[key] = perm
			}
		}
	}

	perms := make([]Permission, 0, len(permMap))
	for _, perm := range permMap {
		perms = append(perms, perm)
	}
	return perms
}
