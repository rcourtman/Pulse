package api

import (
	"encoding/json"
	"net/http"
	"net/url"
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

	// Query() discards malformed pairs and Get() selects the first repeated
	// value. Neither may turn a damaged selection into a successful check.
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_query", "Invalid query encoding", nil)
		return
	}
	if len(query["org_id"]) > 1 {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_query", "org_id must be supplied at most once", nil)
		return
	}

	contextOrgID := strings.TrimSpace(GetOrgID(r.Context()))
	if contextOrgID == "" {
		contextOrgID = "default"
	}

	orgID := contextOrgID
	if query.Has("org_id") {
		orgID = strings.TrimSpace(query.Get("org_id"))
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

	var req struct {
		OrgID         json.RawMessage `json:"org_id"`
		Username      string          `json:"username"`
		RecoveryToken string          `json:"recovery_token"`
	}
	if err := decodeAdminJSONObject(w, r, &req, 4*1024); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid request body", nil)
		return
	}
	// Only omission selects the request organisation. Keep explicit null and
	// blank selections distinct, before validating (and spending) the token.
	var selectedOrgID string
	if req.OrgID != nil && json.Unmarshal(req.OrgID, &selectedOrgID) != nil {
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

	orgID := contextOrgID
	if req.OrgID != nil {
		orgID = strings.TrimSpace(selectedOrgID)
		if !isValidOrganizationID(orgID) {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_org_id", "Invalid organization ID", nil)
			return
		}
		if orgID != contextOrgID {
			writeErrorResponse(w, http.StatusForbidden, "access_denied", "Token is not authorized for this organization", nil)
			return
		}
	}

	store := GetRecoveryTokenStore()
	if !store.ValidateRecoveryTokenConstantTime(req.RecoveryToken, GetClientIP(r)) {
		writeErrorResponse(w, http.StatusForbidden, "invalid_token", "Invalid or expired recovery token", nil)
		return
	}

	if err := ResetAdminRole(h.rbacProvider, orgID, req.Username); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "reset_failed", "Failed to reset admin role", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"org_id":   orgID,
		"username": req.Username,
		"message":  "Admin role reset successfully",
	})
}
