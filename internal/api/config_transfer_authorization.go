package api

import (
	"net/http"
	"os"
	"strings"

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

// authorizeConfigTransfer is the single route-local boundary for export and
// import. It runs before either handler receives the request, so denial cannot
// parse an archive, read export persistence, write import state, or reload the
// runtime.
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
			if !ensureAdminSession(r.config, w, req) {
				return false
			}
			return true
		}
	}

	// HTTP Basic represents the configured instance administrator. Validate it
	// directly so this decision cannot accidentally inherit no-auth fallback.
	if username, password, ok := req.BasicAuth(); ok {
		config.Mu.RLock()
		configuredUser := r.config.AuthUser
		configuredHash := r.config.AuthPass
		config.Mu.RUnlock()
		if configuredUser != "" && configuredHash != "" &&
			constantTimeStringEqual(username, configuredUser) &&
			internalauth.CheckPasswordHash(password, configuredHash) {
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
