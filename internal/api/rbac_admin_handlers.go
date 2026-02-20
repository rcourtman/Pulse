package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// HandleRBACIntegrityCheck returns RBAC data integrity status for an org.
// GET /api/admin/rbac/integrity?org_id=default
func (h *RBACHandlers) HandleRBACIntegrityCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.rbacProvider == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "rbac_unavailable", "RBAC management is not available", nil)
		return
	}

	contextOrgID := strings.TrimSpace(GetOrgID(r.Context()))
	if contextOrgID == "" {
		contextOrgID = "default"
	}

	orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
	if orgID == "" {
		orgID = contextOrgID
	} else {
		if !isValidOrganizationID(orgID) {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_org_id", "Invalid organization ID", nil)
			return
		}
		if orgID != contextOrgID {
			writeErrorResponse(w, http.StatusForbidden, "access_denied", "Token is not authorized for this organization", nil)
			return
		}
	}

	result := VerifyRBACIntegrity(h.rbacProvider, orgID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// HandleRBACAdminReset performs break-glass admin role reset.
// POST /api/admin/rbac/reset-admin
// Requires a valid recovery token for security.
func (h *RBACHandlers) HandleRBACAdminReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.rbacProvider == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "rbac_unavailable", "RBAC management is not available", nil)
		return
	}

	// Limit request body size.
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024) // 4KB max

	var req struct {
		OrgID         string `json:"org_id"`
		Username      string `json:"username"`
		RecoveryToken string `json:"recovery_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}

	if req.Username == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_username", "Username is required", nil)
		return
	}
	if req.RecoveryToken == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_token", "Recovery token is required", nil)
		return
	}

	contextOrgID := strings.TrimSpace(GetOrgID(r.Context()))
	if contextOrgID == "" {
		contextOrgID = "default"
	}

	req.OrgID = strings.TrimSpace(req.OrgID)
	if req.OrgID == "" {
		req.OrgID = contextOrgID
	} else {
		if !isValidOrganizationID(req.OrgID) {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_org_id", "Invalid organization ID", nil)
			return
		}
		if req.OrgID != contextOrgID {
			writeErrorResponse(w, http.StatusForbidden, "access_denied", "Token is not authorized for this organization", nil)
			return
		}
	}

	store := GetRecoveryTokenStore()
	if !store.ValidateRecoveryTokenConstantTime(req.RecoveryToken, GetClientIP(r)) {
		writeErrorResponse(w, http.StatusForbidden, "invalid_token", "Invalid or expired recovery token", nil)
		return
	}

	if err := ResetAdminRole(h.rbacProvider, req.OrgID, req.Username); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "reset_failed", "Failed to reset admin role", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"org_id":   req.OrgID,
		"username": req.Username,
		"message":  "Admin role reset successfully",
	})
}
