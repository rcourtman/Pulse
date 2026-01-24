package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// validRoleID matches alphanumeric IDs with hyphens and underscores (1-64 chars)
var validRoleID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// validUsername matches reasonable username formats (1-128 chars, alphanumeric, plus common chars)
var validUsername = regexp.MustCompile(`^[a-zA-Z0-9._@+-]{1,128}$`)

// RBACHandlers provides HTTP handlers for RBAC management.
type RBACHandlers struct {
	config *config.Config
}

// NewRBACHandlers creates a new RBACHandlers instance.
func NewRBACHandlers(cfg *config.Config) *RBACHandlers {
	return &RBACHandlers{config: cfg}
}

// HandleRoles handles list, create, update, and delete actions for roles.
func (h *RBACHandlers) HandleRoles(w http.ResponseWriter, r *http.Request) {
	manager := auth.GetManager()
	if manager == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "rbac_unavailable", "RBAC management is not available", nil)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/admin/roles")
	id = strings.TrimPrefix(id, "/")
	id = strings.TrimSuffix(id, "/")

	// Validate role ID format if provided
	if id != "" && !validRoleID.MatchString(id) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_role_id", "Invalid role ID format", nil)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if id == "" {
			// List all roles
			roles := manager.GetRoles()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(roles)
		} else {
			// Get specific role
			role, ok := manager.GetRole(id)
			if !ok {
				writeErrorResponse(w, http.StatusNotFound, "not_found", "Role not found", nil)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(role)
		}

	case http.MethodPost:
		if id != "" {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "POST is for creating roles; do not include an ID in the path", nil)
			return
		}

		// Limit request body size
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64KB max

		var role auth.Role
		if err := json.NewDecoder(r.Body).Decode(&role); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid role data", nil)
			return
		}

		// Validate role ID in body
		if role.ID != "" && !validRoleID.MatchString(role.ID) {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_role_id", "Invalid role ID format", nil)
			return
		}

		if err := manager.SaveRole(role); err != nil {
			LogAuditEventForTenant(GetOrgID(r.Context()), "role_create_failed", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false, fmt.Sprintf("Failed to create role %s", role.ID))
			writeErrorResponse(w, http.StatusInternalServerError, "save_failed", "Failed to save role", nil)
			return
		}

		LogAuditEventForTenant(GetOrgID(r.Context()), "role_created", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true, "Created role "+role.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(role)

	case http.MethodPut:
		// Limit request body size
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64KB max

		var role auth.Role
		if err := json.NewDecoder(r.Body).Decode(&role); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid role data", nil)
			return
		}

		if id != "" {
			role.ID = id
		}

		// Validate role ID
		if role.ID != "" && !validRoleID.MatchString(role.ID) {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_role_id", "Invalid role ID format", nil)
			return
		}

		if err := manager.SaveRole(role); err != nil {
			LogAuditEventForTenant(GetOrgID(r.Context()), "role_update_failed", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false, fmt.Sprintf("Failed to update role %s", role.ID))
			writeErrorResponse(w, http.StatusInternalServerError, "save_failed", "Failed to save role", nil)
			return
		}

		LogAuditEventForTenant(GetOrgID(r.Context()), "role_updated", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true, "Updated role "+role.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(role)

	case http.MethodDelete:
		if id == "" {
			writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Role ID is required", nil)
			return
		}

		if err := manager.DeleteRole(id); err != nil {
			LogAuditEventForTenant(GetOrgID(r.Context()), "role_delete_failed", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false, fmt.Sprintf("Failed to delete role %s", id))
			writeErrorResponse(w, http.StatusInternalServerError, "delete_failed", "Failed to delete role", nil)
			return
		}

		LogAuditEventForTenant(GetOrgID(r.Context()), "role_deleted", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true, "Deleted role "+id)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleGetUsers lists users with their assigned roles.
func (h *RBACHandlers) HandleGetUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	manager := auth.GetManager()
	if manager == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "rbac_unavailable", "RBAC management is not available", nil)
		return
	}

	assignments := manager.GetUserAssignments()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assignments)
}

// HandleUserRoleActions handles assigning/updating roles for a user.
func (h *RBACHandlers) HandleUserRoleActions(w http.ResponseWriter, r *http.Request) {
	manager := auth.GetManager()
	if manager == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "rbac_unavailable", "RBAC management is not available", nil)
		return
	}

	// Extract username from path: /api/admin/users/{username}/roles
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_username", "Username is required", nil)
		return
	}
	username := parts[0]

	// Validate username format to prevent injection
	if !validUsername.MatchString(username) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_username", "Invalid username format", nil)
		return
	}

	switch r.Method {
	case http.MethodPut, http.MethodPost:
		// Limit request body size
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64KB max

		var req struct {
			RoleIDs []string `json:"roleIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
			return
		}

		// Validate all role IDs
		for _, roleID := range req.RoleIDs {
			if !validRoleID.MatchString(roleID) {
				writeErrorResponse(w, http.StatusBadRequest, "invalid_role_id", fmt.Sprintf("Invalid role ID format: %s", roleID), nil)
				return
			}
		}

		if err := manager.UpdateUserRoles(username, req.RoleIDs); err != nil {
			LogAuditEventForTenant(GetOrgID(r.Context()), "user_roles_update_failed", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false, fmt.Sprintf("Failed to update roles for user %s", username))
			writeErrorResponse(w, http.StatusInternalServerError, "update_failed", "Failed to update user roles", nil)
			return
		}

		LogAuditEventForTenant(GetOrgID(r.Context()), "user_roles_updated", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true, "Updated roles for user "+username+": ["+strings.Join(req.RoleIDs, ", ")+"]")
		w.WriteHeader(http.StatusNoContent)

	case http.MethodGet:
		// Get effective permissions
		if len(parts) > 1 && parts[1] == "permissions" {
			perms := manager.GetUserPermissions(username)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(perms)
			return
		}

		// Get specific assignment
		assignment, ok := manager.GetUserAssignment(username)
		if !ok {
			writeErrorResponse(w, http.StatusNotFound, "not_found", "User assignment not found", nil)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(assignment)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleRBACChangelog returns the RBAC change history.
func (h *RBACHandlers) HandleRBACChangelog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	em := auth.GetExtendedManager()
	if em == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "rbac_unavailable", "RBAC changelog is not available (requires Pro)", nil)
		return
	}

	// Parse query parameters
	limit := 100
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
		if limit <= 0 || limit > 1000 {
			limit = 100
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
		if offset < 0 {
			offset = 0
		}
	}

	// Filter by entity if provided
	entityType := r.URL.Query().Get("entity_type")
	entityID := r.URL.Query().Get("entity_id")

	var logs []auth.RBACChangeLog
	if entityType != "" && entityID != "" {
		logs = em.GetChangeLogsForEntity(entityType, entityID)
	} else {
		logs = em.GetChangeLogs(limit, offset)
	}

	// Return empty array instead of null
	if logs == nil {
		logs = []auth.RBACChangeLog{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// HandleRoleEffective returns a role with all inherited permissions.
func (h *RBACHandlers) HandleRoleEffective(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	em := auth.GetExtendedManager()
	if em == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "rbac_unavailable", "Role inheritance is not available (requires Pro)", nil)
		return
	}

	// Extract role ID from path: /api/admin/roles/{id}/effective
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/roles/")
	path = strings.TrimSuffix(path, "/effective")
	roleID := strings.TrimSuffix(path, "/")

	if roleID == "" || !validRoleID.MatchString(roleID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_role_id", "Invalid role ID format", nil)
		return
	}

	role, effectivePerms, ok := em.GetRoleWithInheritance(roleID)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Role not found", nil)
		return
	}

	// Return role with effective permissions
	response := struct {
		auth.Role
		EffectivePermissions []auth.Permission `json:"effectivePermissions"`
	}{
		Role:                 role,
		EffectivePermissions: effectivePerms,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleUserEffectivePermissions returns a user's effective permissions with inheritance.
func (h *RBACHandlers) HandleUserEffectivePermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	manager := auth.GetManager()
	if manager == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "rbac_unavailable", "RBAC management is not available", nil)
		return
	}

	// Extract username from path: /api/admin/users/{username}/effective-permissions
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	path = strings.TrimSuffix(path, "/effective-permissions")
	username := strings.TrimSuffix(path, "/")

	if username == "" || !validUsername.MatchString(username) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_username", "Invalid username format", nil)
		return
	}

	// Check if we have extended manager for inheritance
	em := auth.GetExtendedManager()
	if em != nil {
		roles := em.GetRolesWithInheritance(username)

		// Collect all effective permissions
		permMap := make(map[string]auth.Permission)
		for _, role := range roles {
			for _, perm := range role.Permissions {
				key := perm.Action + ":" + perm.Resource + ":" + perm.GetEffect()
				permMap[key] = perm
			}
		}

		var perms []auth.Permission
		for _, perm := range permMap {
			perms = append(perms, perm)
		}

		response := struct {
			Username    string            `json:"username"`
			Roles       []auth.Role       `json:"roles"`
			Permissions []auth.Permission `json:"permissions"`
		}{
			Username:    username,
			Roles:       roles,
			Permissions: perms,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Fall back to basic permissions
	perms := manager.GetUserPermissions(username)
	if perms == nil {
		perms = []auth.Permission{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(perms)
}
