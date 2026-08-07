package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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
	// BusinessEstate marks a free self-hosted install whose monitored estate
	// crosses the business-scale thresholds. Authenticated-session only: it
	// must never ride the pre-auth presentation policy, where it would leak
	// estate size to anonymous visitors.
	BusinessEstate bool `json:"businessEstate"`
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
		// identity, an RBAC admin grant (SSO group role mappings), or an SSO
		// session on an instance with no local admin. Org-scoped sessions keep
		// their own management rules.
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

	return securityStatusSettingsCapabilities{
		InfrastructureRead:  canReadSettings,
		SystemSettingsRead:  canReadSettings,
		APIAccessRead:       r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceUsers, config.ScopeSettingsRead),
		APIAccessWrite:      r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceUsers, config.ScopeSettingsWrite),
		AuthenticationRead:  canReadSettings,
		AuthenticationWrite: canAdminSettings,
		SingleSignOnRead:    r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceUsers, config.ScopeSettingsRead),
		SingleSignOnWrite:   r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceUsers, config.ScopeSettingsWrite),
		Roles:               canManageRoles,
		Users:               canManageRoles,
		AuditLog:            canReadAudit,
		AuditWebhooksRead:   snapshot.passesPrivilegedSessionGate() && r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceAuditLogs, config.ScopeSettingsRead),
		AuditWebhooksWrite:  snapshot.passesPrivilegedSessionGate() && r.canAccessPermissionSurface(snapshot, internalauth.ActionAdmin, internalauth.ResourceAuditLogs, config.ScopeSettingsWrite),
		RelayRead:           canReadSettings,
		RelayWrite:          canAdminSettings,
		BillingAdmin:        r.canAccessPlatformAdminSurface(snapshot),
	}
}

func (r *Router) securityStatusSessionCapabilities(ctx context.Context) securityStatusSessionCapabilities {
	demoMode := r != nil && r.config != nil && r.config.DemoMode
	assistantEnabled := false
	if r != nil && r.aiSettingsHandler != nil {
		assistantEnabled = r.aiSettingsHandler.AssistantEnabled(ctx)
	}
	businessEstate := false
	if r != nil && !demoMode && !r.hostedMode {
		paidOrBranded := false
		if svc := getLicenseServiceForContext(ctx); svc != nil {
			if lic := svc.Current(); lic != nil && lic.Claims.Tier != licenseTierFreeValue {
				paidOrBranded = true
			}
			if svc.HasFeature(featureWhiteLabelValue) {
				paidOrBranded = true
			}
		}
		if !paidOrBranded {
			businessEstate = r.businessScaleEstate(ctx)
		}
	}
	return securityStatusSessionCapabilities{
		DemoMode:         demoMode,
		AssistantEnabled: assistantEnabled,
		BusinessEstate:   businessEstate,
	}
}

// businessScaleEstateCounts delegates to the canonical business-scale estate
// definition in internal/monitoring, which the outbound telemetry snapshot
// also classifies against.
func businessScaleEstateCounts(pveNodes, dockerHosts, vmwareHosts int) bool {
	return monitoring.BusinessScaleEstateCounts(pveNodes, dockerHosts, vmwareHosts)
}

func (r *Router) businessScaleEstate(ctx context.Context) bool {
	if r == nil || r.configHandlers == nil {
		return false
	}
	monitor := r.configHandlers.getMonitor(ctx)
	if monitor == nil {
		return false
	}
	pveNodes, dockerHosts := 0, 0
	if readState := monitor.GetUnifiedReadStateOrSnapshot(); readState != nil {
		pveNodes = len(readState.Nodes())
		dockerHosts = len(readState.DockerHosts())
	}
	vmwareHosts := 0
	resources, _ := monitor.UnifiedResourceSnapshot()
	for _, resource := range resources {
		if resource.VMware == nil {
			continue
		}
		if unifiedresources.CanonicalResourceType(resource.Type) == unifiedresources.ResourceTypeAgent {
			vmwareHosts++
		}
	}
	return businessScaleEstateCounts(pveNodes, dockerHosts, vmwareHosts)
}

// resolveSecurityStatusPresentationPolicy maps commercial suppression inputs
// to the served policy. Per the 2026-08-07 self-hosted commercial-surfaces
// revision (supersedes the 2026-04-25 opt-in record, RA5), upgrade CTAs are
// visible by default; demo mode and white-label runtimes stay suppressed so
// demos, kiosks, and MSP tenant containers never show commercial content.
func resolveSecurityStatusPresentationPolicy(demoMode, whiteLabel bool) securityStatusPresentationPolicy {
	hideCommercial := demoMode || whiteLabel
	return securityStatusPresentationPolicy{
		DemoMode:       demoMode,
		ReadOnly:       demoMode,
		HideCommercial: hideCommercial,
		HideUpgrade:    hideCommercial,
	}
}

func (r *Router) securityStatusPresentationPolicy(ctx context.Context) securityStatusPresentationPolicy {
	demoMode := r != nil && r.config != nil && r.config.DemoMode
	whiteLabel := false
	if r != nil && !demoMode {
		if svc := getLicenseServiceForContext(ctx); svc != nil {
			whiteLabel = svc.HasFeature(featureWhiteLabelValue)
		}
	}
	return resolveSecurityStatusPresentationPolicy(demoMode, whiteLabel)
}
