package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

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
		var role auth.Role
		if err := json.NewDecoder(r.Body).Decode(&role); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid role data", nil)
			return
		}

		if err := manager.SaveRole(role); err != nil {
			LogAuditEvent("role_create_failed", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false, "Failed to create role "+role.ID+": "+err.Error())
			writeErrorResponse(w, http.StatusInternalServerError, "save_failed", err.Error(), nil)
			return
		}

		LogAuditEvent("role_created", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true, "Created role "+role.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(role)

	case http.MethodPut:
		var role auth.Role
		if err := json.NewDecoder(r.Body).Decode(&role); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid role data", nil)
			return
		}

		if id != "" {
			role.ID = id
		}

		if err := manager.SaveRole(role); err != nil {
			LogAuditEvent("role_update_failed", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false, "Failed to update role "+role.ID+": "+err.Error())
			writeErrorResponse(w, http.StatusInternalServerError, "save_failed", err.Error(), nil)
			return
		}

		LogAuditEvent("role_updated", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true, "Updated role "+role.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(role)

	case http.MethodDelete:
		if id == "" {
			writeErrorResponse(w, http.StatusBadRequest, "missing_id", "Role ID is required", nil)
			return
		}

		if err := manager.DeleteRole(id); err != nil {
			LogAuditEvent("role_delete_failed", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false, "Failed to delete role "+id+": "+err.Error())
			writeErrorResponse(w, http.StatusInternalServerError, "delete_failed", err.Error(), nil)
			return
		}

		LogAuditEvent("role_deleted", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true, "Deleted role "+id)
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

	switch r.Method {
	case http.MethodPut, http.MethodPost:
		var req struct {
			RoleIDs []string `json:"roleIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
			return
		}

		if err := manager.UpdateUserRoles(username, req.RoleIDs); err != nil {
			LogAuditEvent("user_roles_update_failed", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false, "Failed to update roles for user "+username+": "+err.Error())
			writeErrorResponse(w, http.StatusInternalServerError, "update_failed", err.Error(), nil)
			return
		}

		LogAuditEvent("user_roles_updated", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true, "Updated roles for user "+username+": ["+strings.Join(req.RoleIDs, ", ")+"]")
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
