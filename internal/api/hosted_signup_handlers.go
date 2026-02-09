package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/hosted"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const hostedSignupRequestBodyLimit = 64 * 1024

type HostedSignupHandlers struct {
	persistence  *config.MultiTenantPersistence
	rbacProvider *TenantRBACProvider
	hostedMode   bool
}

type hostedSignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	OrgName  string `json:"org_name"`
}

type hostedSignupResponse struct {
	OrgID   string `json:"org_id"`
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

func NewHostedSignupHandlers(
	persistence *config.MultiTenantPersistence,
	rbacProvider *TenantRBACProvider,
	hostedMode bool,
) *HostedSignupHandlers {
	return &HostedSignupHandlers{
		persistence:  persistence,
		rbacProvider: rbacProvider,
		hostedMode:   hostedMode,
	}
}

func (h *HostedSignupHandlers) HandlePublicSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.hostedMode {
		http.NotFound(w, r)
		return
	}
	if h.persistence == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "orgs_unavailable", "Organization persistence is not configured", nil)
		return
	}
	if h.rbacProvider == nil {
		writeErrorResponse(w, http.StatusNotImplemented, "provisioner_not_available", "provisioner not yet available", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, hostedSignupRequestBodyLimit)
	var req hostedSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.OrgName = strings.TrimSpace(req.OrgName)
	if !isValidSignupEmail(req.Email) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_email", "Invalid email format", nil)
		return
	}
	if len(req.Password) < 8 {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_password", "Password must be at least 8 characters", nil)
		return
	}
	if !isValidHostedOrgName(req.OrgName) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_org_name", "Invalid organization name", nil)
		return
	}

	orgID := uuid.NewString()
	userID := req.Email

	// Force tenant directory creation using the same EnsureConfigDir-backed path as multi-tenant persistence.
	if _, err := h.persistence.GetPersistence(orgID); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "tenant_init_failed", "Failed to initialize tenant data directory", nil)
		return
	}

	now := time.Now().UTC()
	org := &models.Organization{
		ID:          orgID,
		DisplayName: req.OrgName,
		CreatedAt:   now,
		OwnerUserID: userID,
		Members: []models.OrganizationMember{
			{
				UserID:  userID,
				Role:    models.OrgRoleOwner,
				AddedAt: now,
				AddedBy: userID,
			},
		},
	}
	if err := h.persistence.SaveOrganization(org); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "create_failed", "Failed to create organization", nil)
		return
	}

	authManager, err := h.rbacProvider.GetManager(orgID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "auth_unavailable", "Failed to initialize organization auth manager", nil)
		return
	}
	if err := authManager.UpdateUserRoles(userID, []string{auth.RoleAdmin}); err != nil {
		hosted.GetHostedMetrics().RecordProvision("failure")
		writeErrorResponse(w, http.StatusInternalServerError, "user_create_failed", "Failed to create admin user", nil)
		return
	}

	hosted.GetHostedMetrics().RecordSignup()
	hosted.GetHostedMetrics().RecordProvision("success")
	writeJSON(w, http.StatusCreated, hostedSignupResponse{
		OrgID:   orgID,
		UserID:  userID,
		Message: "Tenant provisioned successfully",
	})
}

func isValidSignupEmail(email string) bool {
	if email == "" || strings.Contains(email, " ") {
		return false
	}
	at := strings.Index(email, "@")
	if at <= 0 || at >= len(email)-1 {
		return false
	}
	domain := email[at+1:]
	if strings.Contains(domain, "@") {
		return false
	}
	dot := strings.Index(domain, ".")
	if dot <= 0 || dot >= len(domain)-1 {
		return false
	}
	return true
}

func isValidHostedOrgName(orgName string) bool {
	if len(orgName) < 3 || len(orgName) > 64 {
		return false
	}
	return isValidOrganizationID(orgName)
}
