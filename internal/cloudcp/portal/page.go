package portal

import (
	"errors"
	"html/template"
	"net/http"
	"time"

	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpsec"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

// portalPageWorkspace holds per-workspace display data for the portal template.
type portalPageWorkspace struct {
	ID                          string
	DisplayName                 string
	State                       string
	Healthy                     bool
	HealthStatus                string
	SetupStatus                 string
	AgentCount                  *int
	AgentTokenCount             *int
	UnusedAgentTokenCount       *int
	LastAgentSeenAt             *time.Time
	AlertRouteCount             *int
	DisabledAlertRouteCount     *int
	ReportScheduleCount         *int
	DisabledReportScheduleCount *int
	LastHealthCheck             *time.Time
	CreatedAt                   time.Time
}

type portalPageMember struct {
	SubjectID string
	UserID    string
	Email     string
	Role      registry.MemberRole
	State     registry.AccountAccessState
	CreatedAt time.Time
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
	Members    []portalPageMember
}

// portalPageData is passed to the portal HTML template.
type portalPageData struct {
	Nonce         string
	FaviconHref   string
	Styles        template.CSS
	ShellScript   template.JS
	BootstrapJSON template.JS
}

const (
	defaultPublicSiteURL        = "https://pulserelay.pro"
	defaultSupportEmail         = "support@pulserelay.pro"
	defaultCommercialAPIBaseURL = PortalCommercialAPIBasePath
	defaultPortalPath           = PortalPagePath
	defaultLogoutPath           = PortalLogoutPath
	defaultAccountAPIBasePath   = PortalAccountAPIBasePath
	defaultPortalAPIBasePath    = PortalAPIBasePath
)

var errPortalAuthRequired = errors.New("portal auth required")

func HandlePortalPageWithSignupPathAndSetupFacts(sessionSvc *cpauth.Service, reg *registry.TenantRegistry, commercialLookup CommercialIdentityLookup, faviconHref string, signupPath string, setupFacts WorkspaceSetupFactReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		nonce := cpsec.NonceFromContext(r.Context())

		claims, err := validatePortalSessionClaims(r, sessionSvc, reg)
		switch {
		case err == nil:
			accounts, loadErr := loadPortalAccountsForUserWithSetupFacts(reg, claims.UserID, setupFacts)
			if loadErr != nil {
				log.Error().Err(loadErr).Str("user_id", claims.UserID).Msg("cloudcp.portal.page: load accounts")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			renderPortalPage(w, nonce, faviconHref, BuildBootstrapDataWithSignupPath(true, claims.Email, accounts, resolveSelfHostedCommercial(r.Context(), commercialLookup, claims.Email, accounts), signupPath))
		case errors.Is(err, errPortalAuthRequired):
			renderPortalPage(w, nonce, faviconHref, BuildAnonymousBootstrapDataWithSignupPath(signupPath))
		default:
			log.Error().Err(err).Msg("cloudcp.portal.page: validate session")
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}
}

func validatePortalSessionClaims(r *http.Request, sessionSvc *cpauth.Service, reg *registry.TenantRegistry) (*cpauth.SessionClaims, error) {
	if r == nil || sessionSvc == nil || reg == nil {
		return nil, errPortalAuthRequired
	}

	token := cpauth.SessionTokenFromRequest(r)
	if token == "" {
		return nil, errPortalAuthRequired
	}

	claims, err := sessionSvc.ValidateSessionToken(token)
	if err != nil {
		return nil, errPortalAuthRequired
	}

	sessionVersion, err := reg.GetUserSessionVersion(claims.UserID)
	if err != nil {
		return nil, err
	}
	if claims.SessionVersion != sessionVersion {
		return nil, errPortalAuthRequired
	}
	return claims, nil
}

func loadPortalAccountsForUser(reg *registry.TenantRegistry, userID string) ([]portalPageAccount, error) {
	return loadPortalAccountsForUserWithSetupFacts(reg, userID, nil)
}

func loadPortalAccountsForUserWithSetupFacts(reg *registry.TenantRegistry, userID string, setupFacts WorkspaceSetupFactReader) ([]portalPageAccount, error) {
	accountIDs, err := reg.ListAccountsByUser(userID)
	if err != nil {
		return nil, err
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

		m, err := reg.GetMembership(accountID, userID)
		if err != nil {
			log.Error().Err(err).Str("account_id", accountID).Str("user_id", userID).Msg("cloudcp.portal.page: get membership")
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
		members, err := loadPortalAccountMembers(reg, accountID, m.Role)
		if err != nil {
			log.Error().Err(err).Str("account_id", accountID).Msg("cloudcp.portal.page: list members")
			continue
		}

		workspaces := make([]portalPageWorkspace, 0, len(tenants))
		for _, t := range tenants {
			if !isPortalVisibleTenant(t) {
				continue
			}
			workspaces = append(workspaces, portalPageWorkspaceFromTenant(t, workspaceSetupFactsForTenant(setupFacts, t.ID)))
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
			Members:    members,
		})
	}

	return accounts, nil
}

func workspaceSetupFactsForTenant(setupFacts WorkspaceSetupFactReader, tenantID string) WorkspaceSetupFacts {
	if setupFacts == nil {
		return WorkspaceSetupFacts{}
	}
	return setupFacts.FactsForWorkspace(tenantID)
}

func portalPageWorkspaceFromTenant(t *registry.Tenant, facts WorkspaceSetupFacts) portalPageWorkspace {
	if t == nil {
		return portalPageWorkspace{}
	}
	return portalPageWorkspace{
		ID:                          t.ID,
		DisplayName:                 t.DisplayName,
		State:                       string(t.State),
		Healthy:                     t.HealthCheckOK,
		HealthStatus:                workspaceHealthStatus(t.HealthCheckOK, t.LastHealthCheck),
		SetupStatus:                 workspaceSetupStatus(t.State, t.HealthCheckOK, t.LastHealthCheck, facts),
		AgentCount:                  facts.AgentCount,
		AgentTokenCount:             facts.AgentTokenCount,
		UnusedAgentTokenCount:       facts.UnusedAgentTokenCount,
		LastAgentSeenAt:             facts.LastAgentSeenAt,
		AlertRouteCount:             facts.AlertRouteCount,
		DisabledAlertRouteCount:     facts.DisabledAlertRouteCount,
		ReportScheduleCount:         facts.ReportScheduleCount,
		DisabledReportScheduleCount: facts.DisabledReportScheduleCount,
		LastHealthCheck:             t.LastHealthCheck,
		CreatedAt:                   t.CreatedAt,
	}
}

func loadPortalAccountMembers(reg *registry.TenantRegistry, accountID string, actorRole registry.MemberRole) ([]portalPageMember, error) {
	subjects, err := reg.ListAccessSubjectsByAccount(accountID)
	if err != nil {
		return nil, err
	}
	if subjects == nil {
		return []portalPageMember{}, nil
	}

	canManage := actorRole == registry.MemberRoleOwner || actorRole == registry.MemberRoleAdmin
	members := make([]portalPageMember, 0, len(subjects))
	for _, subject := range subjects {
		if subject == nil {
			continue
		}
		if subject.State == registry.AccountAccessStatePending && !canManage {
			continue
		}
		members = append(members, portalPageMember{
			SubjectID: subject.SubjectID,
			UserID:    subject.UserID,
			Email:     subject.Email,
			Role:      subject.Role,
			State:     subject.State,
			CreatedAt: subject.CreatedAt,
		})
	}

	return members, nil
}

func workspaceHealthStatus(healthy bool, lastHealthCheck *time.Time) string {
	if healthy {
		return "healthy"
	}
	if lastHealthCheck == nil {
		return "checking"
	}
	return "unhealthy"
}

func workspaceSetupStatus(state registry.TenantState, healthy bool, lastHealthCheck *time.Time, facts WorkspaceSetupFacts) string {
	switch state {
	case registry.TenantStateActive:
		if !healthy && lastHealthCheck != nil {
			return "review"
		}
		if facts.AgentCount != nil {
			if *facts.AgentCount <= 0 {
				return "install_agents"
			}
			if factCountIsZero(facts.AlertRouteCount) || factCountIsZero(facts.ReportScheduleCount) {
				return "configure_outputs"
			}
			if factCountIsPositive(facts.AlertRouteCount) && factCountIsPositive(facts.ReportScheduleCount) {
				return "ready"
			}
		}
		return "setup_path"
	case registry.TenantStateProvisioning:
		return "setup_path"
	default:
		return "review"
	}
}

func factCountIsZero(value *int) bool {
	return value != nil && *value <= 0
}

func factCountIsPositive(value *int) bool {
	return value != nil && *value > 0
}

func renderPortalPage(w http.ResponseWriter, nonce string, faviconHref string, bootstrapData BootstrapData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	bootstrapJSON, err := MarshalBootstrapJSON(bootstrapData)
	if err != nil {
		log.Error().Err(err).Msg("cloudcp.portal.page: marshal bootstrap data")
		bootstrapJSON = template.JS(`{}`)
	}
	if err := portalPageTmpl.Execute(w, portalPageData{
		Nonce:         nonce,
		FaviconHref:   faviconHref,
		Styles:        portalStyles,
		ShellScript:   portalShellScript,
		BootstrapJSON: bootstrapJSON,
	}); err != nil {
		log.Error().Err(err).Msg("cloudcp.portal.page: render portal page")
	}
}
