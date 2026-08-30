package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/edition"
)

type securityStatusSettingsCapabilities struct {
	// InfrastructureRead mirrors the RequireAdmin + settings:read gate that
	// every data source behind Settings → Infrastructure enforces
	// (/api/connections, /api/config/nodes, /api/system/settings,
	// /api/truenas/connections, /api/vmware/connections, and /api/discover,
	// which needs settings:write on top). Serving it lets the nav hide a page
	// that would otherwise mount 15s and 30s pollers whose every request is
	// refused and logged at warn level.
	InfrastructureRead bool `json:"infrastructureRead"`
	// AvailabilityRead, DiagnosticsRead, and SystemLogsRead mirror the
	// RequireAdmin + settings:read guards on the read routes mounted by those
	// Settings panels.
	AvailabilityRead bool `json:"availabilityRead"`
	DiagnosticsRead  bool `json:"diagnosticsRead"`
	SystemLogsRead   bool `json:"systemLogsRead"`
	// PulseIntelligenceRead also includes read:settings authorization because
	// /api/settings/ai applies RequirePermission before its handler repeats the
	// settings:read admin/scope check.
	PulseIntelligenceRead bool `json:"pulseIntelligenceRead"`
	// ReportingRead includes read:nodes authorization as well as the explicit
	// admin session and settings:read checks on the reporting catalog route.
	// The advanced_reporting feature gate remains a separate UI concern.
	ReportingRead bool `json:"reportingRead"`
	// SystemSettingsRead mirrors the same RequireAdmin + settings:read gate for
	// the System → Network, Pulse server updates, and Recovery tabs, whose
	// panels read and write the public URL and CORS boundaries, the server
	// update channel, and backup polling plus config export/import. It is a
	// sibling of InfrastructureRead rather than a reuse of it: they happen to
	// share a derivation today, but each names the surface it describes, so
	// tightening one gate cannot silently hide the other's tabs. System →
	// General stays ungated - theme, language, and unit preferences there are
	// user-scoped, not instance administration.
	SystemSettingsRead  bool `json:"systemSettingsRead"`
	APIAccessRead       bool `json:"apiAccessRead"`
	APIAccessWrite      bool `json:"apiAccessWrite"`
	AuthenticationRead  bool `json:"authenticationRead"`
	AuthenticationWrite bool `json:"authenticationWrite"`
	SingleSignOnRead    bool `json:"singleSignOnRead"`
	SingleSignOnWrite   bool `json:"singleSignOnWrite"`
	Roles               bool `json:"roles"`
	Users               bool `json:"users"`
	AuditLog            bool `json:"auditLog"`
	AuditWebhooksRead   bool `json:"auditWebhooksRead"`
	AuditWebhooksWrite  bool `json:"auditWebhooksWrite"`
	RelayRead           bool `json:"relayRead"`
	RelayWrite          bool `json:"relayWrite"`
	BillingAdmin        bool `json:"billingAdmin"`
}

type securityStatusSessionCapabilities struct {
	DemoMode         bool `json:"demoMode"`
	AssistantEnabled bool `json:"assistantEnabled"`
}

type securityStatusPresentationPolicy struct {
	DemoMode       bool `json:"demoMode"`
	ReadOnly       bool `json:"readOnly"`
	HideCommercial bool `json:"hideCommercial"`
	HideUpgrade    bool `json:"hideUpgrade"`
}

type securityStatusAuthSnapshot struct {
	request        *http.Request
	authenticated  bool
	authMethod     string
	username       string
	proxyIsAdmin   bool
	sessionIsAdmin bool
	tokenRecord    *config.APITokenRecord
}

func (s securityStatusAuthSnapshot) tokenScopes() []string {
	if s.tokenRecord == nil {
		return nil
	}
	return append([]string{}, s.tokenRecord.Scopes...)
}

func (s securityStatusAuthSnapshot) hasScopes(scopes ...string) bool {
	if s.tokenRecord == nil {
		return true
	}
	for _, scope := range scopes {
		if scope == "" {
			continue
		}
		if !s.tokenRecord.HasScope(scope) {
			return false
		}
	}
	return true
}

func (s securityStatusAuthSnapshot) passesPrivilegedSessionGate() bool {
	if !s.authenticated {
		return false
	}
	if s.authMethod == "session" {
		return s.sessionIsAdmin
	}
	return true
}

func (s securityStatusAuthSnapshot) canAccessAdminSurface(scopes ...string) bool {
	if !s.authenticated {
		return false
	}

	switch s.authMethod {
	case "proxy":
		if !s.proxyIsAdmin {
			return false
		}
	case "session":
		if !s.sessionIsAdmin {
			return false
		}
	}

	return s.hasScopes(scopes...)
}

func (r *Router) buildSecurityStatusAuthSnapshot(req *http.Request) securityStatusAuthSnapshot {
	if r == nil || req == nil || r.config == nil {
		return securityStatusAuthSnapshot{}
	}

	if adminBypassEnabled() {
		snapshotReq := attachAdminBypassContext(attachUserContext(req, "admin"))
		return securityStatusAuthSnapshot{
			request:        snapshotReq,
			authenticated:  true,
			authMethod:     "bypass",
			username:       "admin",
			sessionIsAdmin: true,
		}
	}

	if r.config.ProxyAuthSecret != "" {
		if valid, username, isAdmin := CheckProxyAuth(r.config, req); valid {
			snapshotReq := req
			if username != "" {
				snapshotReq = attachUserContext(req, username)
			}
			return securityStatusAuthSnapshot{
				request:        snapshotReq,
				authenticated:  true,
				authMethod:     "proxy",
				username:       username,
				proxyIsAdmin:   isAdmin,
				sessionIsAdmin: false,
			}
		}
	}

	if token := strings.TrimSpace(req.Header.Get("X-API-Token")); token != "" {
		if record, ok := r.config.ValidateAPIToken(token); ok {
			snapshotReq := req
			attachAPITokenRecord(snapshotReq, record)
			tokenUsername := apiTokenAuthenticatedUser(record)
			snapshotReq = attachUserContext(snapshotReq, tokenUsername)
			recordClone := record.Clone()
			return securityStatusAuthSnapshot{
				request:       snapshotReq,
				authenticated: true,
				authMethod:    "api_token",
				username:      tokenUsername,
				tokenRecord:   &recordClone,
			}
		}
	}

	if cookie, err := readSessionCookie(req); err == nil && cookie.Value != "" && ValidateSession(cookie.Value) {
		username := strings.TrimSpace(GetSessionUsername(cookie.Value))
		snapshotReq := attachUserContext(req, username)
		// Same privilege rule as ensureAdminSession: the configured admin
		// identity or an explicit RBAC admin grant (including SSO group role
		// mappings). Org-scoped sessions keep their own management rules.
		sessionIsAdmin := false
		if !sessionIsOrgScoped(req) {
			sessionIsAdmin = sessionUserCarriesAdminPrivileges(r.config, username)
		}
		return securityStatusAuthSnapshot{
			request:        snapshotReq,
			authenticated:  true,
			authMethod:     "session",
			username:       username,
			sessionIsAdmin: sessionIsAdmin,
		}
	}

	return securityStatusAuthSnapshot{}
}

func (r *Router) canAccessPermissionSurface(snapshot securityStatusAuthSnapshot, action, resource string, scopes ...string) bool {
	if !snapshot.authenticated || snapshot.request == nil {
		return false
	}

	// Without a real RBAC authorizer, Authorize allows every action, so it
	// cannot be the sole input to a capability. The routes these capabilities
	// describe are still gated, by ensureSettingsScope and in turn
	// ensureAdminSession, so reporting the capability from the authorizer alone
	// advertises a surface the caller will be refused: the tab renders and its
	// first request comes back 403. Fall back to the same admin identity the
	// routes enforce, which snapshot.sessionIsAdmin already derives from
	// sessionUserCarriesAdminPrivileges. Only the proxy half of this rule was
	// ever written, so session and SSO callers were told they could reach API
	// token management and SSO provider configuration when they could not.
	if _, isDefaultAuthorizer := r.authorizer.(*internalauth.DefaultAuthorizer); isDefaultAuthorizer {
		switch snapshot.authMethod {
		case "proxy":
			if !snapshot.proxyIsAdmin {
				return false
			}
		case "session":
			if !snapshot.sessionIsAdmin {
				return false
			}
		}
	}

	allowed, err := r.authorizer.Authorize(snapshot.request.Context(), action, resource)
	if err != nil || !allowed {
		return false
	}

	return snapshot.hasScopes(scopes...)
}

func (r *Router) canAccessPlatformAdminSurface(snapshot securityStatusAuthSnapshot) bool {
	if !snapshot.authenticated {
		return false
	}

	switch snapshot.authMethod {
	case "bypass":
		return true
	case "session":
		return snapshot.sessionIsAdmin
	case "proxy":
		return snapshot.proxyIsAdmin
	case "api_token":
		return false
	default:
		return false
	}
}

func (r *Router) securityStatusSettingsCapabilitiesFromSnapshot(snapshot securityStatusAuthSnapshot) securityStatusSettingsCapabilities {
	if !snapshot.authenticated {
		return securityStatusSettingsCapabilities{}
	}

	canAdminSettings := snapshot.canAccessAdminSurface(config.ScopeSettingsRead, config.ScopeSettingsWrite)
	canReadSettings := snapshot.canAccessAdminSurface(config.ScopeSettingsRead)
	canManageUsers := r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceUsers)
	canReadAudit := snapshot.passesPrivilegedSessionGate() &&
		r.canAccessPermissionSurface(snapshot, internalauth.ActionRead, internalauth.ResourceAuditLogs, config.ScopeAuditRead)
	canManageRoles := snapshot.passesPrivilegedSessionGate() && canManageUsers
	canReadPulseIntelligence := canReadSettings &&
		r.canAccessPermissionSurface(snapshot, internalauth.ActionRead, internalauth.ResourceSettings, config.ScopeSettingsRead)
	canReadReporting := canReadSettings &&
		r.canAccessPermissionSurface(snapshot, internalauth.ActionRead, internalauth.ResourceNodes, config.ScopeSettingsRead)

	return securityStatusSettingsCapabilities{
		InfrastructureRead:    canReadSettings,
		AvailabilityRead:      canReadSettings,
		PulseIntelligenceRead: canReadPulseIntelligence,
		DiagnosticsRead:       canReadSettings,
		SystemLogsRead:        canReadSettings,
		ReportingRead:         canReadReporting,
		SystemSettingsRead:    canReadSettings,
		APIAccessRead:         r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceUsers, config.ScopeSettingsRead),
		APIAccessWrite:        r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceUsers, config.ScopeSettingsWrite),
		AuthenticationRead:    canReadSettings,
		AuthenticationWrite:   canAdminSettings,
		SingleSignOnRead:      r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceUsers, config.ScopeSettingsRead),
		SingleSignOnWrite:     r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceUsers, config.ScopeSettingsWrite),
		Roles:                 canManageRoles,
		Users:                 canManageRoles,
		AuditLog:              canReadAudit,
		AuditWebhooksRead:     snapshot.passesPrivilegedSessionGate() && r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceAuditLogs, config.ScopeSettingsRead),
		AuditWebhooksWrite:    snapshot.passesPrivilegedSessionGate() && r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceAuditLogs, config.ScopeSettingsWrite),
		RelayRead:             canReadSettings,
		RelayWrite:            canAdminSettings,
		BillingAdmin:          r.canAccessPlatformAdminSurface(snapshot),
	}
}

func (r *Router) securityStatusSessionCapabilities(ctx context.Context) securityStatusSessionCapabilities {
	demoMode := r != nil && r.config != nil && r.config.DemoMode
	assistantEnabled := false
	if r != nil && r.aiSettingsHandler != nil {
		assistantEnabled = r.aiSettingsHandler.AssistantEnabled(ctx)
	}
	return securityStatusSessionCapabilities{
		DemoMode:         demoMode,
		AssistantEnabled: assistantEnabled,
	}
}

// resolveSecurityStatusPresentationPolicy maps commercial suppression inputs
// to the served policy. Ordinary free self-hosted sessions stay opt-in and do
// not receive upgrade prompts. Hosted sessions and installs with existing paid
// or recovery context may expose the commercial navigation; a compiled
// Pro-edition binary counts as paid context because it is only distributed
// through the paid broker flow, and hiding Plans & Billing from it strands a
// customer with no way to find the activation form. Demo mode and white-label
// runtimes remain fully suppressed.
func resolveSecurityStatusPresentationPolicy(demoMode, whiteLabel, commercialContext bool) securityStatusPresentationPolicy {
	hideCommercial := demoMode || whiteLabel
	return securityStatusPresentationPolicy{
		DemoMode:       demoMode,
		ReadOnly:       demoMode,
		HideCommercial: hideCommercial,
		HideUpgrade:    hideCommercial || !commercialContext,
	}
}

func (r *Router) securityStatusPresentationPolicy(ctx context.Context) securityStatusPresentationPolicy {
	demoMode := r != nil && r.config != nil && r.config.DemoMode
	whiteLabel := false
	commercialContext := r != nil && r.hostedMode
	if edition.IsPro() {
		// The compiled Pro binary reaches an install only through the paid
		// broker flow, so the session was never an ordinary free self-hosted
		// one — even before its license activates. Without this, the only
		// navigation entry holding the activation form is hidden exactly
		// while the install is unlicensed. Community binaries with or
		// without a license key are unaffected.
		commercialContext = true
	}
	if r != nil && !demoMode {
		if svc := getLicenseServiceForContext(ctx); svc != nil {
			whiteLabel = svc.HasFeature(featureWhiteLabelValue)
			if lic := svc.Current(); lic != nil && lic.Claims.Tier != licenseTierFreeValue {
				commercialContext = true
			}
			switch strings.ToLower(strings.TrimSpace(svc.SubscriptionState())) {
			case "active", "grace", "canceled", "suspended", "trial":
				commercialContext = true
			}
		}
	}
	return resolveSecurityStatusPresentationPolicy(demoMode, whiteLabel, commercialContext)
}
