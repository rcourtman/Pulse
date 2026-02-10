package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/hosted"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

const hostedSignupRequestBodyLimit = 64 * 1024

type HostedRBACProvider interface {
	GetManager(orgID string) (auth.ExtendedManager, error)
	RemoveTenant(orgID string) error
}

type HostedSignupHandlers struct {
	persistence  *config.MultiTenantPersistence
	rbacProvider HostedRBACProvider
	magicLinks   *MagicLinkService
	publicURL    func(*http.Request) string
	hostedMode   bool
}

type hostedSignupRequest struct {
	Email   string `json:"email"`
	OrgName string `json:"org_name"`
}

type hostedSignupResponse struct {
	OrgID   string `json:"org_id"`
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

func NewHostedSignupHandlers(
	persistence *config.MultiTenantPersistence,
	rbacProvider HostedRBACProvider,
	magicLinks *MagicLinkService,
	publicURL func(*http.Request) string,
	hostedMode bool,
) *HostedSignupHandlers {
	return &HostedSignupHandlers{
		persistence:  persistence,
		rbacProvider: rbacProvider,
		magicLinks:   magicLinks,
		publicURL:    publicURL,
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
	if h.magicLinks == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "magic_links_unavailable", "Magic link service is not configured", nil)
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
	if !isValidHostedOrgName(req.OrgName) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_org_name", "Invalid organization name", nil)
		return
	}

	orgID := uuid.NewString()
	userID := req.Email

	var cleanupOnce sync.Once
	cleanupProvisioning := func() {
		cleanupOnce.Do(func() {
			hosted.GetHostedMetrics().RecordProvision("failure")

			// Best-effort cleanup: close/remove RBAC manager first to avoid lingering DB handles.
			if err := h.rbacProvider.RemoveTenant(orgID); err != nil {
				log.Warn().Err(err).Str("org_id", orgID).Msg("Hosted signup cleanup: failed to remove RBAC tenant")
			}
			if err := h.persistence.DeleteOrganization(orgID); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Warn().Err(err).Str("org_id", orgID).Msg("Hosted signup cleanup: failed to delete org directory")
			}
		})
	}

	// Force tenant directory creation using the same EnsureConfigDir-backed path as multi-tenant persistence.
	if _, err := h.persistence.GetPersistence(orgID); err != nil {
		cleanupProvisioning()
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
		cleanupProvisioning()
		writeErrorResponse(w, http.StatusInternalServerError, "create_failed", "Failed to create organization", nil)
		return
	}

	authManager, err := h.rbacProvider.GetManager(orgID)
	if err != nil {
		cleanupProvisioning()
		writeErrorResponse(w, http.StatusInternalServerError, "auth_unavailable", "Failed to initialize organization auth manager", nil)
		return
	}
	if err := authManager.UpdateUserRoles(userID, []string{auth.RoleAdmin}); err != nil {
		cleanupProvisioning()
		writeErrorResponse(w, http.StatusInternalServerError, "user_create_failed", "Failed to create admin user", nil)
		return
	}

	// Issue a magic link for passwordless sign-in.
	// Rate limiting is enforced per-email to prevent abuse.
	if !h.magicLinks.AllowRequest(userID) {
		writeErrorResponse(w, http.StatusTooManyRequests, "rate_limited", "Too many magic link requests. Please wait and try again.", nil)
		return
	}
	token, err := h.magicLinks.GenerateToken(userID, orgID)
	if err != nil {
		cleanupProvisioning()
		writeErrorResponse(w, http.StatusInternalServerError, "magic_link_failed", "Failed to generate magic link", nil)
		return
	}
	baseURL := ""
	if h.publicURL != nil {
		baseURL = h.publicURL(r)
	}
	if baseURL == "" {
		// Best-effort fallback; hosted deployments should set PublicURL/AgentConnectURL.
		baseURL = "https://" + r.Host
	}
	if err := h.magicLinks.SendMagicLink(userID, orgID, token, baseURL); err != nil {
		cleanupProvisioning()
		writeErrorResponse(w, http.StatusInternalServerError, "magic_link_failed", "Failed to send magic link", nil)
		return
	}

	hosted.GetHostedMetrics().RecordSignup()
	hosted.GetHostedMetrics().RecordProvision("success")
	writeJSON(w, http.StatusCreated, hostedSignupResponse{
		OrgID:   orgID,
		UserID:  userID,
		Message: "Check your email for a magic link to finish signing in.",
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
