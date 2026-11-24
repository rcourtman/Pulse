package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

type configTransferOperation string

const (
	configTransferExport configTransferOperation = "export"
	configTransferImport configTransferOperation = "import"
)

func (op configTransferOperation) requiredScope() string {
	if op == configTransferImport {
		return config.ScopeSettingsWrite
	}
	return config.ScopeSettingsRead
}

// configTransferAuthenticationConfigured is the canonical fail-closed view of
// whether configuration transfer must authenticate. It deliberately includes
// hosted operation and an uncertain SSO load: neither state may fall back to
// unauthenticated recovery.
func (r *Router) configTransferAuthenticationConfigured() bool {
	if r == nil || r.config == nil {
		return true
	}

	config.Mu.RLock()
	localAuthConfigured := strings.TrimSpace(r.config.AuthUser) != "" || strings.TrimSpace(r.config.AuthPass) != ""
	tokenAuthConfigured := r.config.HasAPITokens()
	proxyAuthConfigured := strings.TrimSpace(r.config.ProxyAuthSecret) != ""
	config.Mu.RUnlock()

	return localAuthConfigured ||
		tokenAuthConfigured ||
		proxyAuthConfigured ||
		hasEnabledSSOProvidersForAuth(r.config) ||
		r.hostedMode ||
		r.ssoAuthenticationLoadFailed()
}

func (r *Router) allowUnauthenticatedConfigTransfer(req *http.Request, op configTransferOperation) bool {
	if r.configTransferAuthenticationConfigured() {
		return false
	}
	if isDirectLoopbackRequest(req) {
		return true
	}
	return op == configTransferExport && os.Getenv("ALLOW_UNPROTECTED_EXPORT") == "true"
}

// configTransferUserManagesOrganization is the management rule for a verified
// user transferring the request organization's secret-bearing configuration.
// In a tenant organization the user must manage that organization
// (CanUserIDManage on the organization TenantMiddleware admitted): instance
// administration combined with membership is not enough, because export
// discloses and import replaces that organization's credentials. In the
// default organization it is the canonical instance-administrator rule.
func configTransferUserManagesOrganization(cfg *config.Config, req *http.Request, userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false
	}
	if sessionIsOrgScoped(req) {
		org := GetOrganization(req.Context())
		return org != nil && org.CanUserIDManage(userID)
	}
	return sessionUserCarriesAdminPrivileges(cfg, userID)
}

// keepImportAccessCredentials marks an admitted import whose caller may not
// issue the archive's API tokens or SSO configuration. Restoring those sections
// issues every credential in the file, whatever its scopes, organization
// binding or owner, and anyone can build an archive. It therefore needs the
// authority that issuing them directly needs: instance administration for a
// user (ensureAdminSession), which a tenant's own owners and admins do not
// carry, and instance-wide full access for an API token, as for regenerating
// every token. Any other admitted caller restores the remaining sections.
func keepImportAccessCredentials(req *http.Request) {
	*req = *req.WithContext(apicontext.WithImportKeepingAccessCredentials(req.Context()))
}

// authorizeConfigTransfer is the single route-local boundary for export and
// import. It runs before either handler receives the request, so denial cannot
// parse an archive, read export persistence, write import state, or reload the
// runtime. It also decides, from the credential it admits, whether an import
// may replace access credentials (keepImportAccessCredentials), so that
// decision cannot change while the body is outstanding.
func (r *Router) authorizeConfigTransfer(w http.ResponseWriter, req *http.Request, op configTransferOperation) bool {
	if adminBypassEnabled() {
		return true
	}
	if r == nil || r.config == nil {
		http.Error(w, "Configuration authorization unavailable", http.StatusServiceUnavailable)
		return false
	}

	scope := op.requiredScope()

	// Explicit API-token credentials take precedence over every browser
	// credential. AuthContextMiddleware has already validated the token against
	// the resolved tenant config, and TenantMiddleware has enforced its org
	// binding before this route can run.
	if _, provided := explicitAPITokenFromRequest(req); provided {
		record := getAPITokenRecordFromRequest(req)
		if record == nil {
			http.Error(w, "Invalid API token", http.StatusUnauthorized)
			return false
		}
		if !record.HasScope(scope) {
			respondMissingScope(w, scope)
			return false
		}
		if op == configTransferImport && !tokenHasInstanceWideAuthority(record) {
			keepImportAccessCredentials(req)
		}
		// Neither credential here passes through CheckAuth; an import archive
		// must not arrive after the token or session that admitted it is gone.
		retainRequestBodyAuthority(r.config, req)
		return true
	}

	// A valid proxy identity is authoritative. Membership-like proxy roles are
	// insufficient for a secret-bearing transfer.
	if strings.TrimSpace(r.config.ProxyAuthSecret) != "" {
		if valid, username, isAdmin := CheckProxyAuth(r.config, req); valid {
			if !isAdmin {
				logAuthDenial(req, username, "Non-admin proxy user attempted configuration transfer", nil)
				http.Error(w, "Admin privileges required for configuration transfer", http.StatusForbidden)
				return false
			}
			return true
		}
	}

	// A presented valid session must itself carry management authority. For a
	// tenant this is CanUserIDManage on the resolved organization; for the
	// default organization it is the canonical instance-admin rule.
	if cookie, err := readSessionCookie(req); err == nil && cookie.Value != "" {
		session := GetSessionStore().GetSession(cookie.Value)
		validSession := session != nil && ValidateSession(cookie.Value)
		if validSession && session.RecoveryBypass {
			validSession = requestMatchesRecoverySession(req, session)
		}
		if validSession {
			sessionUser := strings.TrimSpace(GetSessionUsername(cookie.Value))
			if !configTransferUserManagesOrganization(r.config, req, sessionUser) {
				logAuthDenial(req, sessionUser, "Session user without organization management attempted configuration transfer", nil)
				http.Error(w, "Admin privileges required", http.StatusForbidden)
				return false
			}
			// Transfer is a public recovery route, so it does not pass through
			// CheckAuth's session admission. Retain the session actually admitted
			// here, not an ambient cookie discovered after the export has run.
			attachSessionReadAdmission(req, cookie.Value, session)
			if op == configTransferImport && !sessionUserCarriesAdminPrivileges(r.config, sessionUser) {
				keepImportAccessCredentials(req)
			}
			retainRequestBodyAuthority(r.config, req)
			return true
		}
	}

	// HTTP Basic represents the configured instance administrator. Validate it
	// directly so this decision cannot accidentally inherit no-auth fallback.
	// TenantMiddleware admitted the request's organization before this
	// credential was verified, so the verified user must then pass the same
	// organization access policy and the transfer management rule that a
	// session user passes.
	if username, password, ok := req.BasicAuth(); ok {
		config.Mu.RLock()
		configuredUser := r.config.AuthUser
		configuredHash := r.config.AuthPass
		config.Mu.RUnlock()
		// This route does not pass through CheckAuth, so the password test
		// below keeps sign-in's lockout itself: it refuses while the account
		// or the address is locked and atomically counts outcomes against both.
		passwordConfigured := configuredUser != "" && configuredHash != ""
		clientIP := GetClientIP(req)
		if passwordConfigured {
			_, _, userLocked := GetLockoutInfo(username)
			_, _, ipLocked := GetLockoutInfo(clientIP)
			if userLocked || ipLocked {
				logAuthDenial(req, username, "Basic credentials refused for configuration transfer during lockout", nil)
				http.Error(w, "Too many failed attempts. Please try again later.", http.StatusForbidden)
				return false
			}
		}
		credentialsValid := passwordConfigured &&
			constantTimeStringEqual(username, configuredUser) &&
			internalauth.CheckPasswordHash(password, configuredHash)
		if passwordConfigured {
			if _, locked := RecordLoginAttempt(credentialsValid, username, clientIP); locked {
				logAuthDenial(req, username, "Basic credentials refused for configuration transfer during lockout", nil)
				http.Error(w, "Too many failed attempts. Please try again later.", http.StatusForbidden)
				return false
			}
		}
		if credentialsValid {
			if access := authorizeVerifiedUserForRequestOrganization(req, configuredUser); !access.Allowed {
				logAuthDenial(req, configuredUser, "Basic credentials refused for the request organization during configuration transfer", nil)
				writeJSONError(w, http.StatusForbidden, "access_denied", access.Reason)
				return false
			}
			if !configTransferUserManagesOrganization(r.config, req, configuredUser) {
				logAuthDenial(req, configuredUser, "Basic credentials without organization management attempted configuration transfer", nil)
				http.Error(w, "Admin privileges required", http.StatusForbidden)
				return false
			}
			return true
		}
	}

	if r.allowUnauthenticatedConfigTransfer(req, op) {
		return true
	}

	if r.configTransferAuthenticationConfigured() {
		logAuthDenial(req, "", "Unauthenticated configuration transfer attempt", nil)
		http.Error(w, "Unauthorized - please log in or provide an API token", http.StatusUnauthorized)
		return false
	}

	log.Warn().
		Str("ip", req.RemoteAddr).
		Str("operation", string(op)).
		Msg("Configuration transfer blocked outside direct loopback recovery policy")
	http.Error(w, "Configuration transfer requires authentication outside direct loopback", http.StatusForbidden)
	return false
}
