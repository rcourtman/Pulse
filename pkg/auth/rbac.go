package auth

import (
	"context"
	"sync"
	"time"
)

// Permission defines a single allowed action on a resource.
type Permission struct {
	Action   string `json:"action"`   // read, write, delete, admin
	Resource string `json:"resource"` // nodes, settings, users, audit_logs, etc.
}

// Role represents a collection of permissions.
type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	IsBuiltIn   bool         `json:"isBuiltIn"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// UserRoleAssignment maps a user to one or more roles.
type UserRoleAssignment struct {
	Username  string    `json:"username"`
	RoleIDs   []string  `json:"roleIds"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Built-in Role IDs
const (
	RoleAdmin      = "admin"
	RoleOperator   = "operator"
	RoleViewer     = "viewer"
	RoleAuditor    = "auditor"
	RoleCompliance = "compliance" // Alias for Auditor
)

// Manager defines the interface for managing RBAC data.
// This is typically implemented by the enterprise RBAC store.
type Manager interface {
	// Role management
	GetRoles() []Role
	GetRole(id string) (Role, bool)
	SaveRole(role Role) error
	DeleteRole(id string) error

	// Assignment management
	GetUserAssignments() []UserRoleAssignment
	GetUserAssignment(username string) (UserRoleAssignment, bool)
	AssignRole(username string, roleID string) error
	UpdateUserRoles(username string, roleIDs []string) error
	RemoveRole(username string, roleID string) error

	// Effective permissions
	GetUserPermissions(username string) []Permission
}

var (
	globalManager Manager
	managerMu     sync.RWMutex
)

// SetManager sets the global RBAC manager instance.
// This should be called during application initialization.
func SetManager(m Manager) {
	managerMu.Lock()
	defer managerMu.Unlock()
	globalManager = m
}

// GetManager returns the global RBAC manager instance.
func GetManager() Manager {
	managerMu.RLock()
	defer managerMu.RUnlock()
	return globalManager
}

// Requirement helpers
func HasPermission(ctx context.Context, action, resource string) bool {
	authorizer := GetAuthorizer()
	allowed, _ := authorizer.Authorize(ctx, action, resource)
	return allowed
}
