package cloudcp

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/account"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/admin"
	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/email"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/handoff"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/portal"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	cpstripe "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/stripe"
)

// Deps holds shared dependencies injected into HTTP handlers.
type Deps struct {
	Config             *CPConfig
	Registry           *registry.TenantRegistry
	Docker             *docker.Manager // nil if Docker is unavailable
	MagicLinks         *cpauth.Service // control plane magic link service
	Provisioner        *cpstripe.Provisioner
	HostedEntitlements *entitlements.Service
	Version            string
	EmailSender        email.Sender
}

func publicCloudSignupPath(cfg *CPConfig) string {
	if cfg != nil && cfg.IsProviderHostedMSP() {
		return ""
	}
	if cfg != nil && cfg.PublicCloudSignupEnabled {
		return portal.PortalSignupPath
	}
	return ""
}

func providerMSPPlanVersion(cfg *CPConfig) string {
	if cfg == nil || strings.TrimSpace(cfg.ProviderMSPPlanVersion) == "" {
		return defaultProviderHostedMSPPlanVersion
	}
	return cfg.ProviderMSPPlanVersion
}

// RegisterRoutes wires all HTTP handlers onto the given ServeMux.
func RegisterRoutes(mux *http.ServeMux, deps *Deps) {
	webhookLimiter := NewCPRateLimiter(deps.Config.WebhookRateLimitPerMinute, time.Minute)
	magicLinkVerifyLimiter := NewCPRateLimiter(deps.Config.MagicLinkVerifyRateLimitPerMinute, time.Minute)
	sessionAuthLimiter := NewCPRateLimiter(deps.Config.SessionAuthRateLimitPerMinute, time.Minute)
	adminLimiter := NewCPRateLimiter(deps.Config.AdminRateLimitPerMinute, time.Minute)
	accountAPILimiter := NewCPRateLimiter(deps.Config.AccountAPIRateLimitPerMinute, time.Minute)
	portalAPILimiter := NewCPRateLimiter(deps.Config.PortalAPIRateLimitPerMinute, time.Minute)
	hostedEntitlementRefreshLimiter := NewCPRateLimiter(60, time.Hour)
	publicSignupLimiter := NewCPRateLimiter(30, time.Minute)
	publicMagicLinkLimiter := NewCPRateLimiter(20, time.Minute)
	publicCloudSignupPath := publicCloudSignupPath(deps.Config)

	adminAuth := func(next http.Handler) http.Handler {
		return admin.AdminKeyMiddleware(deps.Config.AdminKey, next)
	}
	sessionAuth := func(next http.Handler) http.Handler {
		if deps.MagicLinks == nil {
			return adminAuth(next)
		}
		return requireSessionAuth(deps.MagicLinks, deps.Registry, next)
	}
	accountSessionAuth := func(extract accountIDExtractor, next http.Handler) http.Handler {
		if deps.MagicLinks == nil {
			return adminAuth(next)
		}
		return sessionAuth(requireAccountMembership(deps.Registry, extract, next))
	}
	accountMutationAuth := requireAnyAccountRole(registry.MemberRoleOwner, registry.MemberRoleAdmin)
	rawCommercialLookup := newCommercialIdentityLookup(deps.Config)
	portalSetupFacts := portal.NewTenantDirWorkspaceSetupFactReader(deps.Config.TenantsDir())
	portalCommercialLookup := func(ctx context.Context, email string) (*portal.CommercialIdentity, error) {
		if rawCommercialLookup == nil {
			return nil, nil
		}
		identity, err := rawCommercialLookup(ctx, email)
		if err != nil {
			return nil, err
		}
		if identity == nil {
			return nil, nil
		}
		return &portal.CommercialIdentity{
			HasCommercialIdentity: identity.HasCommercialIdentity,
		}, nil
	}

	// Health / readiness are unauthenticated liveness/readiness probes.
	mux.HandleFunc("/healthz", admin.HandleHealthz)
	mux.HandleFunc("/readyz", admin.HandleReadyz(deps.Registry))
	mux.HandleFunc("/favicon.svg", handleControlPlaneFaviconSVG)
	mux.HandleFunc("/favicon.ico", handleControlPlaneFaviconICO)

	// Status and metrics are private by default.
	statusHandler := http.HandlerFunc(admin.HandleStatus(deps.Registry, deps.Version))
	if deps.Config.PublicStatus {
		mux.Handle("/status", statusHandler)
	} else {
		mux.Handle("/status", adminAuth(statusHandler))
	}

	metricsHandler := promhttp.Handler()
	if deps.Config.PublicMetrics {
		mux.Handle("/metrics", metricsHandler)
	} else {
		mux.Handle("/metrics", adminAuth(metricsHandler))
	}

	// Stripe webhook (signature-authenticated)
	hostedEntitlements := deps.HostedEntitlements
	if hostedEntitlements == nil {
		hostedEntitlements = entitlements.NewService(deps.Registry, deps.Config.BaseURL, deps.Config.TrialActivationPrivateKey)
	}
	provisioner := deps.Provisioner
	if provisioner == nil {
		provisioner = cpstripe.NewProvisioner(
			deps.Registry,
			deps.Config.TenantsDir(),
			deps.Docker,
			deps.MagicLinks,
			deps.Config.BaseURL,
			deps.EmailSender,
			deps.Config.EmailFrom,
			deps.Config.AllowDockerlessProvisioning,
			cpstripe.WithHostedEntitlementService(hostedEntitlements),
			cpstripe.WithTrialActivationPrivateKey(deps.Config.TrialActivationPrivateKey),
			cpstripe.WithDefaultMSPPlanVersion(providerMSPPlanVersion(deps.Config)),
		)
	}
	if deps.Config.UsesStripeBilling() {
		webhookHandler := cpstripe.NewWebhookHandler(deps.Config.StripeWebhookSecret, provisioner)
		mux.Handle("/api/stripe/webhook", webhookLimiter.Middleware(webhookHandler))
	}

	// Magic link verification (public, token-authenticated)
	baseDomain := baseDomainFromURL(deps.Config.BaseURL)
	mux.Handle("/auth/magic-link/verify", magicLinkVerifyLimiter.Middleware(http.HandlerFunc(cpauth.HandleMagicLinkVerify(deps.MagicLinks, deps.Registry, deps.Config.TenantsDir(), baseDomain, portal.PortalPagePath))))
	if deps.MagicLinks != nil {
		mux.Handle(portal.PortalLogoutPath, sessionAuthLimiter.Middleware(sessionAuth(cpauth.HandleLogout(deps.Registry))))
	}

	// Hosted entitlement refresh stays available for already-issued hosted leases.
	hostedEntitlementHandlers := NewHostedEntitlementHandlers(hostedEntitlements)
	mux.Handle("/api/entitlements/refresh", hostedEntitlementRefreshLimiter.Middleware(http.HandlerFunc(hostedEntitlementHandlers.HandleRefresh)))

	// Public commercial magic-link requests stay available for existing hosted
	// accounts even while public v6 Cloud signup remains prelaunch-disabled.
	publicCloudSignupHandlers := NewPublicCloudSignupHandlers(deps.Config, deps.Registry, deps.MagicLinks, deps.EmailSender)
	mux.Handle("/api/public/magic-link/request", publicMagicLinkLimiter.Middleware(http.HandlerFunc(publicCloudSignupHandlers.HandlePublicMagicLinkRequest)))

	// Pulse Cloud self-serve signup: public page + API checkout.
	if publicCloudSignupPath != "" {
		mux.Handle("/signup", publicSignupLimiter.Middleware(http.HandlerFunc(publicCloudSignupHandlers.HandleSignupPage)))
		mux.Handle("/cloud/signup", publicSignupLimiter.Middleware(http.HandlerFunc(publicCloudSignupHandlers.HandleSignupPage)))
		mux.Handle("/signup/complete", publicSignupLimiter.Middleware(http.HandlerFunc(publicCloudSignupHandlers.HandleSignupComplete)))
		mux.Handle("/cloud/signup/complete", publicSignupLimiter.Middleware(http.HandlerFunc(publicCloudSignupHandlers.HandleSignupComplete)))
		mux.Handle("/api/public/signup", publicSignupLimiter.Middleware(http.HandlerFunc(publicCloudSignupHandlers.HandlePublicSignup)))

		// Pulse Cloud for MSPs gated signup. Registered under the same
		// public-signup gate; stays inert (renders an unavailable state) until
		// an MSP tier price ID is configured in CP env.
		mux.Handle("/cloud/msp/signup", publicSignupLimiter.Middleware(http.HandlerFunc(publicCloudSignupHandlers.HandleMSPSignupPage)))
		mux.Handle("/cloud/msp/signup/complete", publicSignupLimiter.Middleware(http.HandlerFunc(publicCloudSignupHandlers.HandleMSPSignupComplete)))
		mux.Handle("/api/public/msp/signup", publicSignupLimiter.Middleware(http.HandlerFunc(publicCloudSignupHandlers.HandleMSPPublicSignup)))
	}

	// Admin API (key-authenticated)
	tenantsHandler := admin.HandleListTenants(deps.Registry)
	mux.Handle("/admin/tenants", adminLimiter.Middleware(adminAuth(tenantsHandler)))
	mux.Handle("/admin/magic-link", adminLimiter.Middleware(adminAuth(cpauth.HandleAdminGenerateMagicLink(deps.MagicLinks, deps.Config.BaseURL, deps.EmailSender, deps.Config.EmailFrom))))

	// Account membership (session + account-membership authenticated)
	listMembers := account.HandleListMembers(deps.Registry)
	inviteMember := account.HandleInviteMember(deps.Registry)
	updateRole := account.HandleUpdateMemberRole(deps.Registry)
	removeMember := account.HandleRemoveMember(deps.Registry)

	membersCollection := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listMembers(w, r)
		case http.MethodPost:
			accountMutationAuth(http.HandlerFunc(inviteMember)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	member := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			accountMutationAuth(http.HandlerFunc(updateRole)).ServeHTTP(w, r)
		case http.MethodDelete:
			accountMutationAuth(http.HandlerFunc(removeMember)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	accountIDFromPath := func(r *http.Request) string {
		return r.PathValue("account_id")
	}
	accountIDFromPortalRequest := func(r *http.Request) string {
		if v := strings.TrimSpace(r.URL.Query().Get("account_id")); v != "" {
			return v
		}
		if v := strings.TrimSpace(r.Header.Get("X-Account-ID")); v != "" {
			return v
		}
		if v := strings.TrimSpace(r.Header.Get("X-Account-Id")); v != "" {
			return v
		}
		return ""
	}

	mux.Handle("/api/accounts/{account_id}/members", accountAPILimiter.Middleware(accountSessionAuth(accountIDFromPath, membersCollection)))
	mux.Handle("/api/accounts/{account_id}/members/{user_id}", accountAPILimiter.Middleware(accountSessionAuth(accountIDFromPath, member)))

	// Workspace management (session + account-membership authenticated)
	listTenants := account.HandleListTenants(deps.Registry)
	workspaceLimitPolicy := account.WorkspaceLimitPolicy{}
	if deps.Config.IsProviderHostedMSP() {
		workspaceLimitPolicy.ProviderHostedMSP = true
		workspaceLimitPolicy.ProviderMSPPlanVersion = providerMSPPlanVersion(deps.Config)
	}
	createTenant := account.HandleCreateTenantWithWorkspaceLimitPolicy(deps.Registry, provisioner, workspaceLimitPolicy)
	updateTenant := account.HandleUpdateTenant(deps.Registry)
	deleteTenant := account.HandleDeleteTenant(deps.Registry, provisioner)

	tenantsCollection := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listTenants(w, r)
		case http.MethodPost:
			accountMutationAuth(http.HandlerFunc(createTenant)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	tenant := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			accountMutationAuth(http.HandlerFunc(updateTenant)).ServeHTTP(w, r)
		case http.MethodDelete:
			accountMutationAuth(http.HandlerFunc(deleteTenant)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.Handle("/api/accounts/{account_id}/tenants", accountAPILimiter.Middleware(accountSessionAuth(accountIDFromPath, tenantsCollection)))
	mux.Handle("/api/accounts/{account_id}/tenants/{tenant_id}", accountAPILimiter.Middleware(accountSessionAuth(accountIDFromPath, tenant)))

	// Tenant switching handoff (session + account-membership authenticated)
	handoffHandler := handoff.HandleHandoff(deps.Registry, deps.Config.TenantsDir())
	mux.Handle("/api/accounts/{account_id}/tenants/{tenant_id}/handoff", accountAPILimiter.Middleware(accountSessionAuth(accountIDFromPath, handoffHandler)))

	// MSP portal API (session + account-membership authenticated)
	mux.Handle(portal.PortalBootstrapPath, portalAPILimiter.Middleware(sessionAuth(portal.HandlePortalBootstrapWithSignupPathAndSetupFacts(deps.MagicLinks, deps.Registry, portalCommercialLookup, publicCloudSignupPath, portalSetupFacts))))
	mux.Handle(portal.PortalDashboardPath, portalAPILimiter.Middleware(accountSessionAuth(accountIDFromPortalRequest, portal.HandlePortalDashboardWithSetupFacts(deps.Registry, portalSetupFacts))))
	mux.Handle(portal.PortalWorkspacePath, portalAPILimiter.Middleware(accountSessionAuth(accountIDFromPortalRequest, portal.HandlePortalWorkspaceDetail(deps.Registry))))

	if deps.Config.UsesStripeBilling() {
		// Stripe Customer Portal redirect (session + account-membership authenticated)
		billingCfg := portal.BillingPortalConfig{
			StripeAPIKey: deps.Config.StripeAPIKey,
			ReturnURL:    buildCPURL(deps.Config.BaseURL, portal.PortalPagePath, nil),
		}
		mux.Handle(portal.PortalBillingPath, portalAPILimiter.Middleware(accountSessionAuth(accountIDFromPortalRequest, portal.HandleBillingPortalRedirect(deps.Registry, billingCfg))))
	}
	mux.Handle(portal.PortalCommercialProxyPath, portalAPILimiter.Middleware(sessionAuth(portal.HandleCommercialProxy(portal.CommercialProxyConfig{
		BaseURL: deps.Config.LicenseServerURL,
	}))))

	// MSP/Cloud portal HTML page — self-authenticating (shows login form if no session)
	portalPageLimiter := NewCPRateLimiter(60, time.Minute)
	mux.Handle(portal.PortalPagePath, portalPageLimiter.Middleware(http.HandlerFunc(portal.HandlePortalPageWithSignupPathAndSetupFacts(deps.MagicLinks, deps.Registry, portalCommercialLookup, controlPlaneFaviconHref(), publicCloudSignupPath, portalSetupFacts))))
}
