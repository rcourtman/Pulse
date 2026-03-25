package portal

import (
	"net/http"
	"time"

	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpsec"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

// portalPageWorkspace holds per-workspace display data for the portal template.
type portalPageWorkspace struct {
	ID          string
	DisplayName string
	State       string
	Healthy     bool
	CreatedAt   time.Time
}

// portalPageAccount holds per-account display data including the user's role.
type portalPageAccount struct {
	ID         string
	Kind       string
	KindLabel  string
	Name       string
	Role       string
	CanManage  bool // owner or admin
	HasBilling bool // true when a Stripe customer exists for the account
	Workspaces []portalPageWorkspace
}

// portalPageData is passed to the portal HTML template.
type portalPageData struct {
	Nonce         string
	Email         string
	PublicSiteURL string
	SupportEmail  string
	Accounts      []portalPageAccount
}

const (
	defaultPublicSiteURL = "https://pulserelay.pro"
	defaultSupportEmail  = "support@pulserelay.pro"
)

// HandlePortalPage serves the MSP/Cloud portal dashboard (browser-facing HTML).
// Route: GET /portal
//   - No session or invalid session -> shows a magic-link login form
//   - Valid session -> shows workspace list with management actions
func HandlePortalPage(sessionSvc *cpauth.Service, reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		nonce := cpsec.NonceFromContext(r.Context())

		// Validate session from cookie or Bearer token.
		token := cpauth.SessionTokenFromRequest(r)
		if token == "" || sessionSvc == nil || reg == nil {
			renderLoginPage(w, nonce)
			return
		}

		claims, err := sessionSvc.ValidateSessionToken(token)
		if err != nil {
			renderLoginPage(w, nonce)
			return
		}
		sessionVersion, err := reg.GetUserSessionVersion(claims.UserID)
		if err != nil {
			log.Error().Err(err).Str("user_id", claims.UserID).Msg("cloudcp.portal.page: get session version")
			renderLoginPage(w, nonce)
			return
		}
		if claims.SessionVersion != sessionVersion {
			renderLoginPage(w, nonce)
			return
		}

		accountIDs, err := reg.ListAccountsByUser(claims.UserID)
		if err != nil {
			log.Error().Err(err).Str("user_id", claims.UserID).Msg("cloudcp.portal.page: list accounts by user")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		accounts := make([]portalPageAccount, 0, len(accountIDs))
		for _, accountID := range accountIDs {
			a, err := reg.GetAccount(accountID)
			if err != nil {
				log.Error().Err(err).Str("account_id", accountID).Msg("cloudcp.portal.page: get account")
				continue
			}
			if a == nil {
				continue
			}

			m, err := reg.GetMembership(accountID, claims.UserID)
			if err != nil {
				log.Error().Err(err).Str("account_id", accountID).Str("user_id", claims.UserID).Msg("cloudcp.portal.page: get membership")
				continue
			}
			if m == nil {
				continue
			}

			tenants, err := reg.ListByAccountID(accountID)
			if err != nil {
				log.Error().Err(err).Str("account_id", accountID).Msg("cloudcp.portal.page: list tenants")
				continue
			}

			workspaces := make([]portalPageWorkspace, 0, len(tenants))
			for _, t := range tenants {
				if t == nil {
					continue
				}
				if t.State == registry.TenantStateDeleted || t.State == registry.TenantStateDeleting {
					continue
				}
				workspaces = append(workspaces, portalPageWorkspace{
					ID:          t.ID,
					DisplayName: t.DisplayName,
					State:       string(t.State),
					Healthy:     t.HealthCheckOK,
					CreatedAt:   t.CreatedAt,
				})
			}

			kindLabel := "Cloud"
			if a.Kind == registry.AccountKindMSP {
				kindLabel = "MSP"
			}

			hasBilling := false
			if sa, saErr := reg.GetStripeAccount(accountID); saErr != nil {
				log.Warn().Err(saErr).Str("account_id", accountID).Msg("cloudcp.portal.page: lookup stripe account for billing button")
			} else if sa != nil && sa.StripeCustomerID != "" {
				hasBilling = true
			}

			accounts = append(accounts, portalPageAccount{
				ID:         a.ID,
				Kind:       string(a.Kind),
				KindLabel:  kindLabel,
				Name:       a.DisplayName,
				Role:       string(m.Role),
				CanManage:  m.Role == registry.MemberRoleOwner || m.Role == registry.MemberRoleAdmin,
				HasBilling: hasBilling,
				Workspaces: workspaces,
			})
		}

		renderPortalPage(w, nonce, claims.Email, accounts)
	}
}

func renderPortalPage(w http.ResponseWriter, nonce, email string, accounts []portalPageAccount) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := portalPageTmpl.Execute(w, portalPageData{
		Nonce:         nonce,
		Email:         email,
		PublicSiteURL: defaultPublicSiteURL,
		SupportEmail:  defaultSupportEmail,
		Accounts:      accounts,
	}); err != nil {
		log.Error().Err(err).Msg("cloudcp.portal.page: render portal page")
	}
}

func renderLoginPage(w http.ResponseWriter, nonce string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := loginPageTmpl.Execute(w, struct{ Nonce string }{Nonce: nonce}); err != nil {
		log.Error().Err(err).Msg("cloudcp.portal.page: render login page")
	}
}
