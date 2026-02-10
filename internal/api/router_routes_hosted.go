package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func (r *Router) registerHostedRoutes(hostedSignupHandlers *HostedSignupHandlers, magicLinkHandlers *MagicLinkHandlers) {
	if r.signupRateLimiter == nil {
		r.signupRateLimiter = NewRateLimiter(5, 1*time.Hour)
	}

	routerConfig := r.config
	if routerConfig == nil {
		routerConfig = &config.Config{}
	}

	billingHandlers := NewBillingStateHandlers(config.NewFileBillingStore(routerConfig.DataPath), r.hostedMode)
	lifecycleHandlers := NewOrgLifecycleHandlers(r.multiTenant, r.hostedMode)
	r.mux.HandleFunc(
		"GET /api/admin/orgs/{id}/billing-state",
		RequireAdmin(routerConfig, RequireScope(config.ScopeSettingsRead, billingHandlers.HandleGetBillingState)),
	)
	r.mux.HandleFunc(
		"PUT /api/admin/orgs/{id}/billing-state",
		RequireAdmin(routerConfig, RequireScope(config.ScopeSettingsWrite, billingHandlers.HandlePutBillingState)),
	)
	r.mux.HandleFunc(
		"POST /api/admin/orgs/{id}/suspend",
		RequireAdmin(routerConfig, RequireScope(config.ScopeSettingsWrite, lifecycleHandlers.HandleSuspendOrg)),
	)
	r.mux.HandleFunc(
		"POST /api/admin/orgs/{id}/unsuspend",
		RequireAdmin(routerConfig, RequireScope(config.ScopeSettingsWrite, lifecycleHandlers.HandleUnsuspendOrg)),
	)
	r.mux.HandleFunc(
		"POST /api/admin/orgs/{id}/soft-delete",
		RequireAdmin(routerConfig, RequireScope(config.ScopeSettingsWrite, lifecycleHandlers.HandleSoftDeleteOrg)),
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
}
