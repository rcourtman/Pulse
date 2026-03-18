package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/hosted"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const hostedSignupRequestBodyLimit = 64 * 1024

type HostedRBACProvider interface {
	GetManager(orgID string) (auth.ExtendedManager, error)
	RemoveTenant(orgID string) error
}

type HostedSignupHandlers struct {
	persistence  *config.MultiTenantPersistence
	billingStore *config.FileBillingStore
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
	var billingStore *config.FileBillingStore
	if persistence != nil {
		billingStore = config.NewFileBillingStore(persistence.BaseDataDir())
	}
	return &HostedSignupHandlers{
		persistence:  persistence,
		billingStore: billingStore,
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

	// Enforce per-email rate limit before any provisioning side effects.
	if !h.magicLinks.AllowRequest(req.Email) {
		writeErrorResponse(w, http.StatusTooManyRequests, "rate_limited", "Too many magic link requests. Please wait and try again.", nil)
		return
	}

	baseURL := ""
	if h.publicURL != nil {
		baseURL = strings.TrimSpace(h.publicURL(r))
	}
	if baseURL == "" {
		writeErrorResponse(w, http.StatusServiceUnavailable, "public_url_missing", "Public URL is not configured", nil)
		return
	}

	provisioner := hosted.NewProvisioner(h.persistence, hosted.NewTenantRBACAuthProvider(h.rbacProvider))
	result, err := provisioner.ProvisionHostedSignup(r.Context(), hosted.HostedSignupRequest{
		Email:   req.Email,
		OrgName: req.OrgName,
	})
	if err != nil {
		status := http.StatusInternalServerError
		code := "create_failed"
		message := "Failed to create organization"
		if hosted.IsValidationError(err) {
			status = http.StatusBadRequest
			code = "invalid_request"
			message = "Invalid signup request"
		}
		writeErrorResponse(w, status, code, message, nil)
		return
	}

	orgID := result.OrgID
	userID := result.UserID

	var cleanupOnce sync.Once
	cleanupProvisioning := func() {
		cleanupOnce.Do(func() {
			hosted.GetHostedMetrics().RecordProvisionStatus(hosted.ProvisionMetricStatusFailure)
			provisioner.RollbackProvisioning(orgID)
		})
	}

	// Seed trial billing state so the tenant is usable immediately (before Stripe checkout completes).
	// Stripe webhook will overwrite this with active subscription state on successful checkout.
	if h.billingStore != nil {
		now := time.Now().UTC()
		trialState := buildTrialBillingStateWithPlanFromLicensing(
			now,
			cloudCapabilitiesFromLicensing(),
			"cloud_trial",
			defaultTrialDurationFromLicensing(),
		)
		if err := h.billingStore.SaveBillingState(orgID, trialState); err != nil {
			cleanupProvisioning()
			writeErrorResponse(w, http.StatusInternalServerError, "billing_init_failed", "Failed to initialize billing state", nil)
			return
		}
	}

	// Issue a magic link for passwordless sign-in.
	token, err := h.magicLinks.GenerateToken(userID, orgID)
	if err != nil {
		cleanupProvisioning()
		writeErrorResponse(w, http.StatusInternalServerError, "magic_link_failed", "Failed to generate magic link", nil)
		return
	}
	if err := h.magicLinks.SendMagicLink(userID, orgID, token, baseURL); err != nil {
		cleanupProvisioning()
		writeErrorResponse(w, http.StatusInternalServerError, "magic_link_failed", "Failed to send magic link", nil)
		return
	}

	hosted.GetHostedMetrics().RecordSignup()
	hosted.GetHostedMetrics().RecordProvisionStatus(hosted.ProvisionMetricStatusSuccess)
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
	for _, r := range orgName {
		if unicode.IsControl(r) {
			return false
		}
		if r == '/' || r == '\\' {
			return false
		}
	}
	return true
}
