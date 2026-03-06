package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type securityStatusSettingsCapabilities struct {
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
			tokenUsername := fmt.Sprintf("token:%s", record.ID)
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
		configuredAdmin := ""
		if r.config != nil {
			configuredAdmin = strings.TrimSpace(r.config.AuthUser)
		}
		return securityStatusAuthSnapshot{
			request:        snapshotReq,
			authenticated:  true,
			authMethod:     "session",
			username:       username,
			sessionIsAdmin: configuredAdmin != "" && strings.EqualFold(username, configuredAdmin),
		}
	}

	return securityStatusAuthSnapshot{}
}

func (r *Router) canAccessPermissionSurface(snapshot securityStatusAuthSnapshot, action, resource string, scopes ...string) bool {
	if !snapshot.authenticated || snapshot.request == nil {
		return false
	}

	if snapshot.authMethod == "proxy" && !snapshot.proxyIsAdmin {
		if _, isDefaultAuthorizer := r.authorizer.(*internalauth.DefaultAuthorizer); isDefaultAuthorizer {
			return false
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
		r.canAccessPermissionSurface(snapshot, internalauth.ActionRead, internalauth.ResourceAuditLogs, config.ScopeSettingsRead)
	canManageRoles := snapshot.passesPrivilegedSessionGate() && canManageUsers

	return securityStatusSettingsCapabilities{
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

func (r *Router) securityStatusSettingsCapabilities(req *http.Request) securityStatusSettingsCapabilities {
	return r.securityStatusSettingsCapabilitiesFromSnapshot(r.buildSecurityStatusAuthSnapshot(req))
}
