package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func (r *Router) registerHostedRoutes(hostedSignupHandlers *HostedSignupHandlers, magicLinkHandlers *MagicLinkHandlers, stripeWebhookHandlers *StripeWebhookHandlers) {
	if r.signupRateLimiter == nil {
		r.signupRateLimiter = NewRateLimiter(5, 1*time.Hour)
	}
	if r.handoffExchangeRateLimiter == nil {
		r.handoffExchangeRateLimiter = NewRateLimiter(20, 1*time.Minute)
	}

	routerConfig := r.config
	if routerConfig == nil {
		routerConfig = &config.Config{}
	}

	billingHandlers := NewBillingStateHandlers(config.NewFileBillingStore(routerConfig.DataPath), r.hostedMode)
	lifecycleHandlers := NewOrgLifecycleHandlers(r.multiTenant, r.hostedMode)
	hostedOrgAdminHandlers := NewHostedOrgAdminHandlers(r.multiTenant, r.hostedMode)
	r.mux.HandleFunc(
		"GET /api/admin/orgs/{id}/billing-state",
		RequireOrgOwnerOrPlatformAdmin(routerConfig, r.multiTenant, RequireScope(config.ScopeSettingsRead, billingHandlers.HandleGetBillingState)),
	)
	r.mux.HandleFunc(
		"PUT /api/admin/orgs/{id}/billing-state",
		RequireOrgOwnerOrPlatformAdmin(routerConfig, r.multiTenant, RequireScope(config.ScopeSettingsWrite, billingHandlers.HandlePutBillingState)),
	)
	r.mux.HandleFunc(
		"POST /api/admin/orgs/{id}/suspend",
		RequireOrgOwnerOrPlatformAdmin(routerConfig, r.multiTenant, RequireScope(config.ScopeSettingsWrite, lifecycleHandlers.HandleSuspendOrg)),
	)
	r.mux.HandleFunc(
		"POST /api/admin/orgs/{id}/unsuspend",
		RequireOrgOwnerOrPlatformAdmin(routerConfig, r.multiTenant, RequireScope(config.ScopeSettingsWrite, lifecycleHandlers.HandleUnsuspendOrg)),
	)
	r.mux.HandleFunc(
		"POST /api/admin/orgs/{id}/soft-delete",
		RequireOrgOwnerOrPlatformAdmin(routerConfig, r.multiTenant, RequireScope(config.ScopeSettingsWrite, lifecycleHandlers.HandleSoftDeleteOrg)),
	)
	r.mux.HandleFunc(
		"GET /api/hosted/organizations",
		RequirePlatformAdmin(routerConfig, RequireScope(config.ScopeSettingsRead, hostedOrgAdminHandlers.HandleListOrganizations)),
	)
	r.mux.HandleFunc(
		"POST /api/admin/orgs/{id}/agent-install-command",
		RequireOrgOwnerOrPlatformAdmin(routerConfig, r.multiTenant, RequireScope(config.ScopeSettingsWrite, r.handleHostedTenantAgentInstallCommand)),
	)

	if hostedSignupHandlers != nil {
		// Register bare paths and let handlers enforce the method checks.
		// This keeps inventory tests and auth bypass logic consistent across the codebase.
		r.mux.HandleFunc("/api/public/signup", r.signupRateLimiter.Middleware(hostedSignupHandlers.HandlePublicSignup))
	}
	if magicLinkHandlers != nil {
		r.mux.HandleFunc("/api/public/magic-link/request", magicLinkHandlers.HandlePublicMagicLinkRequest)
		r.mux.HandleFunc("/api/public/magic-link/verify", magicLinkHandlers.HandlePublicMagicLinkVerify)
	}
	if stripeWebhookHandlers != nil {
		// No auth: Stripe calls this endpoint. Signature verification is the auth layer.
		r.mux.HandleFunc("/api/webhooks/stripe", stripeWebhookHandlers.HandleStripeWebhook)
	}

	// Cloud handoff: control-plane redirects here after magic link verification.
	// Handler self-guards via handoff key file check — returns 404 if not a cloud tenant.
	r.mux.HandleFunc("/auth/cloud-handoff", HandleCloudHandoff(routerConfig.DataPath))
	if r.licenseHandlers != nil {
		// Hosted trial signup callback: signed token activation flow for self-hosted Pulse Pro trials.
		r.mux.HandleFunc("/auth/trial-activate", r.licenseHandlers.HandleTrialActivation)
	}
	// Workspace switch handoff: control-plane posts a short-lived JWT for session exchange.
	// Handler is token-authenticated, self-guards with the tenant handoff key, and
	// is rate-limited independently because the endpoint is public+CSRF-exempt.
	r.mux.HandleFunc("/api/cloud/handoff/exchange", r.handoffExchangeRateLimiter.Middleware(HandleHandoffExchange(routerConfig.DataPath)))
}
