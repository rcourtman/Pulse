package portal

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
	stripe "github.com/stripe/stripe-go/v82"
	billingportalsession "github.com/stripe/stripe-go/v82/billingportal/session"
)

type accountInfo struct {
	ID          string               `json:"id"`
	DisplayName string               `json:"display_name"`
	Kind        registry.AccountKind `json:"kind"`
}

type workspaceSummaryItem struct {
	ID              string               `json:"id"`
	DisplayName     string               `json:"display_name"`
	State           registry.TenantState `json:"state"`
	HealthCheckOK   bool                 `json:"health_check_ok"`
	LastHealthCheck *time.Time           `json:"last_health_check"`
	CreatedAt       time.Time            `json:"created_at"`
}

type dashboardSummary struct {
	Total     int `json:"total"`
	Active    int `json:"active"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Suspended int `json:"suspended"`
}

type dashboardResponse struct {
	Account    accountInfo            `json:"account"`
	Workspaces []workspaceSummaryItem `json:"workspaces"`
	Summary    dashboardSummary       `json:"summary"`
}

type workspaceDetailResponse struct {
	Account   accountInfo      `json:"account"`
	Workspace *registry.Tenant `json:"workspace"`
}

func accountIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if v := strings.TrimSpace(r.URL.Query().Get("account_id")); v != "" {
		return v
	}
	// Convenience for future callers; spec says query param is fine for now.
	if v := strings.TrimSpace(r.Header.Get("X-Account-ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.Header.Get("X-Account-Id")); v != "" {
		return v
	}
	return ""
}

// HandlePortalDashboard returns a portal-oriented dashboard response for an account.
// Route: GET /api/portal/dashboard?account_id=...
//
// Auth: control-plane session + account membership middleware.
func HandlePortalDashboard(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if reg == nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		accountID := accountIDFromRequest(r)
		if accountID == "" {
			http.Error(w, "missing account_id", http.StatusBadRequest)
			return
		}

		a, err := reg.GetAccount(accountID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if a == nil {
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}

		tenants, err := reg.ListByAccountID(accountID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if tenants == nil {
			tenants = []*registry.Tenant{}
		}

		resp := dashboardResponse{
			Account: accountInfo{
				ID:          a.ID,
				DisplayName: a.DisplayName,
				Kind:        a.Kind,
			},
			Workspaces: make([]workspaceSummaryItem, 0, len(tenants)),
		}

		for _, t := range tenants {
			if t == nil {
				continue
			}

			resp.Workspaces = append(resp.Workspaces, workspaceSummaryItem{
				ID:              t.ID,
				DisplayName:     t.DisplayName,
				State:           t.State,
				HealthCheckOK:   t.HealthCheckOK,
				LastHealthCheck: t.LastHealthCheck,
				CreatedAt:       t.CreatedAt,
			})

			resp.Summary.Total++

			switch t.State {
			case registry.TenantStateActive:
				resp.Summary.Active++
				if t.HealthCheckOK {
					resp.Summary.Healthy++
				} else {
					resp.Summary.Unhealthy++
				}
			case registry.TenantStateSuspended:
				resp.Summary.Suspended++
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		encodeJSON(w, resp)
	}
}

// HandlePortalWorkspaceDetail returns a portal-oriented detail response for a single tenant.
// Route: GET /api/portal/workspaces/{tenant_id}?account_id=...
//
// Auth: control-plane session + account membership middleware.
func HandlePortalWorkspaceDetail(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if reg == nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		accountID := accountIDFromRequest(r)
		tenantID := strings.TrimSpace(r.PathValue("tenant_id"))
		if accountID == "" || tenantID == "" {
			http.Error(w, "missing account_id or tenant_id", http.StatusBadRequest)
			return
		}

		a, err := reg.GetAccount(accountID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if a == nil {
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}

		t, err := reg.GetTenantForAccount(accountID, tenantID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if t == nil {
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		resp := workspaceDetailResponse{
			Account: accountInfo{
				ID:          a.ID,
				DisplayName: a.DisplayName,
				Kind:        a.Kind,
			},
			Workspace: t,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		encodeJSON(w, resp)
	}
}

// BillingPortalConfig holds the configuration for the billing portal redirect handler.
type BillingPortalConfig struct {
	StripeAPIKey string
	ReturnURL    string // URL the user returns to after leaving the Stripe portal
}

// billingPortalHandler is the internal handler for Stripe billing portal session creation.
type billingPortalHandler struct {
	reg           *registry.TenantRegistry
	cfg           BillingPortalConfig
	createSession func(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error)
}

// HandleBillingPortalRedirect creates a Stripe Customer Portal session and returns
// the redirect URL. The authenticated user must be an owner or admin of the account.
// Route: POST /api/portal/billing?account_id=...
//
// Auth: control-plane session + account membership middleware + owner/admin role check.
func HandleBillingPortalRedirect(reg *registry.TenantRegistry, cfg BillingPortalConfig) http.HandlerFunc {
	h := &billingPortalHandler{
		reg:           reg,
		cfg:           cfg,
		createSession: defaultCreateBillingPortalSession,
	}
	return h.serveHTTP
}

func (h *billingPortalHandler) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.reg == nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Only account owners and admins may access billing.
	role := registry.MemberRole(strings.TrimSpace(r.Header.Get("X-User-Role")))
	if role != registry.MemberRoleOwner && role != registry.MemberRoleAdmin {
		http.Error(w, "forbidden: billing access requires owner or admin role", http.StatusForbidden)
		return
	}

	if strings.TrimSpace(h.cfg.StripeAPIKey) == "" {
		log.Warn().Msg("cloudcp.portal: billing portal redirect called but Stripe API key not configured")
		http.Error(w, "billing portal not configured", http.StatusServiceUnavailable)
		return
	}

	accountID := accountIDFromRequest(r)
	if accountID == "" {
		http.Error(w, "missing account_id", http.StatusBadRequest)
		return
	}

	sa, err := h.reg.GetStripeAccount(accountID)
	if err != nil {
		log.Error().Err(err).Str("account_id", accountID).Msg("cloudcp.portal: lookup stripe account")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if sa == nil || strings.TrimSpace(sa.StripeCustomerID) == "" {
		http.Error(w, "no billing account found", http.StatusNotFound)
		return
	}

	stripe.Key = strings.TrimSpace(h.cfg.StripeAPIKey)
	params := &stripe.BillingPortalSessionParams{
		Customer: stripe.String(strings.TrimSpace(sa.StripeCustomerID)),
	}
	if returnURL := strings.TrimSpace(h.cfg.ReturnURL); returnURL != "" {
		params.ReturnURL = stripe.String(returnURL)
	}

	session, err := h.createSession(params)
	if err != nil {
		log.Error().Err(err).Str("account_id", accountID).Msg("cloudcp.portal: create billing portal session")
		http.Error(w, "failed to create billing portal session", http.StatusBadGateway)
		return
	}
	if session == nil || strings.TrimSpace(session.URL) == "" {
		log.Error().Str("account_id", accountID).Msg("cloudcp.portal: Stripe returned empty billing portal URL")
		http.Error(w, "failed to create billing portal session", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encodeJSON(w, map[string]string{"url": strings.TrimSpace(session.URL)})
}

func defaultCreateBillingPortalSession(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
	return billingportalsession.New(params)
}

func encodeJSON(w http.ResponseWriter, payload any) {
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error().Err(err).Msg("cloudcp.portal: encode JSON response")
	}
}
