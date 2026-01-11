package auth

import (
	"context"
	"sync"
	"time"
)

// Permission defines an access rule for an action on a resource.
type Permission struct {
	Action     string            `json:"action"`               // read, write, delete, admin
	Resource   string            `json:"resource"`             // nodes, nodes:pve1, settings, *
	Effect     string            `json:"effect,omitempty"`     // "allow" (default) or "deny"
	Conditions map[string]string `json:"conditions,omitempty"` // ABAC conditions, e.g., {"tag": "production"}
}

// EffectAllow and EffectDeny are the valid values for Permission.Effect
const (
	EffectAllow = "allow"
	EffectDeny  = "deny"
)

// GetEffect returns the effect, defaulting to "allow" if empty
func (p Permission) GetEffect() string {
	if p.Effect == "" {
		return EffectAllow
	}
	return p.Effect
}

// Role represents a collection of permissions.
type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	ParentID    string       `json:"parentId,omitempty"` // For role inheritance
	Permissions []Permission `json:"permissions"`
	IsBuiltIn   bool         `json:"isBuiltIn"`
	Priority    int          `json:"priority,omitempty"` // For conflict resolution (higher = more priority)
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// UserRoleAssignment maps a user to one or more roles.
type UserRoleAssignment struct {
	Username  string    `json:"username"`
	RoleIDs   []string  `json:"roleIds"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RBACChangeLog represents an audit entry for RBAC changes.
type RBACChangeLog struct {
	ID         string    `json:"id"`
	Action     string    `json:"action"`             // role_created, role_updated, role_deleted, user_assigned, etc.
	EntityType string    `json:"entityType"`         // role, assignment
	EntityID   string    `json:"entityId"`           // Role ID or username
	OldValue   string    `json:"oldValue,omitempty"` // JSON of previous state
	NewValue   string    `json:"newValue,omitempty"` // JSON of new state
	User       string    `json:"user,omitempty"`     // Who made the change
	Timestamp  time.Time `json:"timestamp"`
}

// RBAC change action constants
const (
	ActionRoleCreated     = "role_created"
	ActionRoleUpdated     = "role_updated"
	ActionRoleDeleted     = "role_deleted"
	ActionUserAssigned    = "user_assigned"
	ActionUserUnassigned  = "user_unassigned"
	ActionUserRolesUpdate = "user_roles_updated"
)

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

// ExtendedManager extends Manager with advanced RBAC features.
// This is implemented by the SQLite-backed manager for Pro features.
type ExtendedManager interface {
	Manager

	// Role inheritance
	GetRoleWithInheritance(id string) (Role, []Permission, bool) // Returns role and all inherited permissions
	GetRolesWithInheritance(username string) []Role              // Returns user's roles with inheritance chain

	// Change log
	GetChangeLogs(limit int, offset int) []RBACChangeLog
	GetChangeLogsForEntity(entityType, entityID string) []RBACChangeLog

	// Context-aware operations (for audit trail)
	SaveRoleWithContext(role Role, username string) error
	DeleteRoleWithContext(id string, username string) error
	UpdateUserRolesWithContext(username string, roleIDs []string, byUser string) error
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

// GetExtendedManager returns the global manager as ExtendedManager if it implements the interface.
func GetExtendedManager() ExtendedManager {
	managerMu.RLock()
	defer managerMu.RUnlock()
	if em, ok := globalManager.(ExtendedManager); ok {
		return em
	}
	return nil
}

// MatchesResource checks if a permission's resource pattern matches a requested resource.
// Supports:
// - Exact match: "nodes" matches "nodes"
// - Specific ID: "nodes:pve1" matches "nodes:pve1"
// - Wildcard: "nodes:*" matches "nodes:pve1"
// - Global wildcard: "*" matches any resource
func MatchesResource(pattern, resource string) bool {
	// Global wildcard matches everything
	if pattern == "*" {
		return true
	}

	// Exact match
	if pattern == resource {
		return true
	}

	// Check for wildcard pattern (e.g., "nodes:*")
	if len(pattern) > 2 && pattern[len(pattern)-2:] == ":*" {
		prefix := pattern[:len(pattern)-2]
		// Match if resource starts with the prefix
		if resource == prefix {
			return true
		}
		// Match if resource has the prefix followed by a colon
		if len(resource) > len(prefix) && resource[:len(prefix)+1] == prefix+":" {
			return true
		}
	}

	return false
}

// MatchesAction checks if a permission's action matches a requested action.
// "admin" action matches any action.
func MatchesAction(permAction, requestedAction string) bool {
	if permAction == "admin" {
		return true
	}
	return permAction == requestedAction
}
