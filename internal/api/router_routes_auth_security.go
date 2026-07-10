package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/crewjam/saml"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
	"github.com/rs/zerolog/log"
)

const featureSSOKey = "sso"

const (
	securityStatusDetailPublic        = "public"
	securityStatusDetailAuthenticated = "authenticated"
	securityStatusDetailPrivileged    = "privileged"
)

type securityStatusSSOProvider struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	DisplayName string `json:"displayName,omitempty"`
	IconURL     string `json:"iconUrl,omitempty"`
	LoginURL    string `json:"loginUrl"`
}

// publicSecurityStatusResponse is the intentional unauthenticated contract for
// login and first-run routing. Deployment, network, credential, and operator
// configuration details must never be added to this type.
type publicSecurityStatusResponse struct {
	DetailLevel        string                           `json:"detailLevel"`
	HasAuthentication  bool                             `json:"hasAuthentication"`
	RequiresAuth       bool                             `json:"requiresAuth"`
	SSOEnabled         bool                             `json:"ssoEnabled"`
	HideLocalLogin     bool                             `json:"hideLocalLogin"`
	SSOProviders       []securityStatusSSOProvider      `json:"ssoProviders,omitempty"`
	PresentationPolicy securityStatusPresentationPolicy `json:"presentationPolicy"`
}

type authenticatedSecurityStatusResponse struct {
	publicSecurityStatusResponse
	HasProxyAuth          bool                               `json:"hasProxyAuth"`
	ProxyAuthLogoutURL    string                             `json:"proxyAuthLogoutURL"`
	ProxyAuthUsername     string                             `json:"proxyAuthUsername"`
	ProxyAuthIsAdmin      bool                               `json:"proxyAuthIsAdmin"`
	SSOSessionUsername    string                             `json:"ssoSessionUsername"`
	SSOSessionDisplayName string                             `json:"ssoSessionDisplayName"`
	SSOLogoutURL          string                             `json:"ssoLogoutURL,omitempty"`
	TokenScopes           []string                           `json:"tokenScopes,omitempty"`
	SessionCapabilities   securityStatusSessionCapabilities  `json:"sessionCapabilities"`
	SettingsCapabilities  securityStatusSettingsCapabilities `json:"settingsCapabilities"`
}

type privilegedSecurityStatusResponse struct {
	authenticatedSecurityStatusResponse
	APITokenConfigured          bool   `json:"apiTokenConfigured"`
	APITokenHint                string `json:"apiTokenHint"`
	ExportProtected             bool   `json:"exportProtected"`
	UnprotectedExportAllowed    bool   `json:"unprotectedExportAllowed"`
	ConfiguredButPendingRestart bool   `json:"configuredButPendingRestart"`
	HasAuditLogging             bool   `json:"hasAuditLogging"`
	CredentialsEncrypted        bool   `json:"credentialsEncrypted"`
	HasHTTPS                    bool   `json:"hasHTTPS"`
	ClientIP                    string `json:"clientIP"`
	IsPrivateNetwork            bool   `json:"isPrivateNetwork"`
	IsTrustedNetwork            bool   `json:"isTrustedNetwork"`
	PublicAccess                bool   `json:"publicAccess"`
	AuthUsername                string `json:"authUsername"`
	AuthLastModified            string `json:"authLastModified"`
	AgentURL                    string `json:"agentUrl"`
}

func (r *Router) securityAuthenticationConfigured() bool {
	if r == nil || r.config == nil {
		return false
	}
	if (r.config.AuthUser != "" && r.config.AuthPass != "") ||
		r.config.HasAPITokens() ||
		r.config.ProxyAuthSecret != "" ||
		r.hostedMode {
		return true
	}
	ssoCfg := r.ensureSSOConfig()
	return ssoCfg != nil && ssoCfg.HasEnabledProviders()
}

func (r *Router) authorizeSecurityRestart(w http.ResponseWriter, req *http.Request) bool {
	clientIP := GetClientIP(req)
	if r.securityAuthenticationConfigured() {
		if !CheckAuth(r.config, w, req) {
			log.Warn().Str("ip", clientIP).Msg("Unauthenticated apply-restart attempt blocked")
			return false
		}
		if r.config.ProxyAuthSecret != "" {
			if valid, username, isAdmin := CheckProxyAuth(r.config, req); valid && !isAdmin {
				log.Warn().
					Str("ip", clientIP).
					Str("username", username).
					Msg("Non-admin user attempted service restart")
				http.Error(w, "Admin privileges required", http.StatusForbidden)
				return false
			}
		}
		return ensureSettingsWriteScope(r.config, w, req)
	}

	if r.bootstrapTokenHash == "" {
		log.Error().Msg("Bootstrap setup token unavailable; refusing unauthenticated service restart")
		http.Error(w, "Bootstrap token unavailable", http.StatusServiceUnavailable)
		return false
	}

	if limiter := r.bootstrapTokenLimiter(); limiter != nil {
		if allowed, retryAfter := limiter.allowAt(clientIP, time.Now()); !allowed {
			retrySeconds := int(retryAfter.Round(time.Second) / time.Second)
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retrySeconds))
			http.Error(w, "Too many bootstrap token attempts", http.StatusTooManyRequests)
			return false
		}
	}

	providedToken := strings.TrimSpace(req.Header.Get(bootstrapTokenHeader))
	if providedToken == "" || !r.bootstrapTokenValid(providedToken) {
		log.Warn().Str("ip", clientIP).Msg("Rejected apply-restart attempt without a valid bootstrap token")
		http.Error(w, "Valid bootstrap token required", http.StatusUnauthorized)
		return false
	}

	return true
}

func (r *Router) registerAuthSecurityInstallRoutes() {
	// API routes
	r.mux.HandleFunc("/api/health", r.handleHealth)
	r.mux.HandleFunc("/api/state", r.handleState)
	r.mux.HandleFunc("/api/state/summary", r.handleStateSummary)
	r.mux.HandleFunc("/api/version", r.handleVersion)
	r.mux.HandleFunc("/api/security/validate-bootstrap-token", r.handleValidateBootstrapToken)
	// Security routes
	r.mux.HandleFunc("/api/security/change-password", r.handleChangePassword)
	r.mux.HandleFunc("/api/logout", r.handleLogout)
	r.mux.HandleFunc("/api/login", r.handleLogin)
	r.mux.HandleFunc("/api/security/reset-lockout", r.handleResetLockout)
	ssoAdminEndpoints := resolveSSOAdminEndpoints(ssoAdminEndpointAdapter{router: r}, newSSOAdminRuntime(r))
	// Per-provider SSO OIDC routes: /api/oidc/{providerID}/login and /api/oidc/{providerID}/callback
	// Use a prefix handler since Go 1.x ServeMux doesn't support path params.
	// Requests matching /api/oidc/{something}/ are dispatched here.
	r.mux.HandleFunc("/api/oidc/", func(w http.ResponseWriter, req *http.Request) {
		// Determine which sub-endpoint was requested.
		parts := strings.Split(strings.TrimPrefix(req.URL.Path, "/"), "/")
		// Per-provider: ["api", "oidc", "{providerID}", "{endpoint}"]
		// Legacy v5:    ["api", "oidc", "{endpoint}"] -> mapped to the migrated
		//               legacy provider by handleSSOOIDC*. An upgraded provider
		//               keeps its v5 redirect URL (/api/oidc/callback), so the IdP
		//               still redirects the browser to the 3-part path.
		endpoint := ""
		switch {
		case len(parts) >= 4:
			endpoint = parts[3]
		case len(parts) == 3:
			endpoint = parts[2]
		default:
			http.NotFound(w, req)
			return
		}
		switch endpoint {
		case "login":
			r.handleSSOOIDCLogin(w, req)
		case "callback":
			r.handleSSOOIDCCallback(w, req)
		default:
			http.NotFound(w, req)
		}
	})
	r.mux.HandleFunc("/api/security/sso/providers/test", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, featureSSOKey, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsWriteScope(r.config, w, req) {
			return
		}
		ssoAdminEndpoints.HandleProviderTest(w, req)
	})))
	r.mux.HandleFunc("/api/security/sso/providers/metadata/preview", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, featureSSOKey, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsReadScope(r.config, w, req) {
			return
		}
		ssoAdminEndpoints.HandleMetadataPreview(w, req)
	})))
	r.mux.HandleFunc("/api/security/sso/providers", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, featureSSOKey, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureSettingsReadScope(r.config, w, req) {
				return
			}
		case http.MethodPost:
			if !ensureSettingsWriteScope(r.config, w, req) {
				return
			}
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ssoAdminEndpoints.HandleProvidersCollection(w, req)
	})))
	r.mux.HandleFunc("/api/security/sso/providers/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, featureSSOKey, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureSettingsReadScope(r.config, w, req) {
				return
			}
		case http.MethodPut, http.MethodDelete:
			if !ensureSettingsWriteScope(r.config, w, req) {
				return
			}
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ssoAdminEndpoints.HandleProviderItem(w, req)
	})))

	// SAML login flow routes (unauthenticated - these are login/callback endpoints).
	// SAML is part of the core SSO contract and is included with Community.
	r.mux.HandleFunc("/api/saml/", RequireLicenseFeature(r.licenseHandlers, featureSSOKey, func(w http.ResponseWriter, req *http.Request) {
		parts := strings.Split(strings.TrimPrefix(req.URL.Path, "/"), "/")
		if len(parts) < 4 {
			http.NotFound(w, req)
			return
		}
		switch parts[3] {
		case "login":
			r.handleSAMLLogin(w, req)
		case "acs":
			r.handleSAMLACS(w, req)
		case "metadata":
			r.handleSAMLMetadata(w, req)
		case "logout":
			r.handleSAMLLogout(w, req)
		case "slo":
			r.handleSAMLSLO(w, req)
		default:
			http.NotFound(w, req)
		}
	}))

	r.mux.HandleFunc("/api/security/tokens/relay-mobile", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsWriteScope(r.config, w, req) {
			return
		}
		if req.Method != http.MethodPost {
			r.handleCreateRelayMobileAccessToken(w, req)
			return
		}
		RequireLicenseFeature(r.licenseHandlers, featureRelayKey, r.handleCreateRelayMobileAccessToken)(w, req)
	}))
	r.mux.HandleFunc("/api/security/tokens", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureSettingsReadScope(r.config, w, req) {
				return
			}
			r.handleListAPITokens(w, req)
		case http.MethodPost:
			if !ensureSettingsWriteScope(r.config, w, req) {
				return
			}
			r.handleCreateAPIToken(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	r.mux.HandleFunc("/api/security/tokens/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			if !ensureSettingsReadScope(r.config, w, req) {
				return
			}
			r.handleGetAPIToken(w, req)
			return
		}
		if !ensureSettingsWriteScope(r.config, w, req) {
			return
		}
		if strings.HasSuffix(req.URL.Path, "/rotate") && req.Method == http.MethodPost {
			r.handleRotateAPIToken(w, req)
			return
		}
		r.handleDeleteAPIToken(w, req)
	}))
	r.mux.HandleFunc("/api/security/status", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		ssoCfg := r.ensureSSOConfig()
		enabledProviders := []config.SSOProvider{}
		if ssoCfg != nil {
			enabledProviders = ssoCfg.GetEnabledProviders()
		}
		hasEnabledSSO := len(enabledProviders) > 0
		var primaryOIDCConfig *config.OIDCProviderConfig
		ssoProviders := make([]securityStatusSSOProvider, 0, len(enabledProviders))
		for i := range enabledProviders {
			provider := enabledProviders[i]
			if primaryOIDCConfig == nil && provider.Type == config.SSOProviderTypeOIDC && provider.OIDC != nil {
				primaryOIDCConfig = provider.OIDC
			}
			info := securityStatusSSOProvider{
				ID:          provider.ID,
				Name:        provider.Name,
				Type:        string(provider.Type),
				DisplayName: provider.DisplayName,
				IconURL:     provider.IconURL,
			}
			if info.DisplayName == "" {
				info.DisplayName = provider.Name
			}
			switch provider.Type {
			case config.SSOProviderTypeOIDC:
				info.LoginURL = "/api/oidc/" + provider.ID + "/login"
			case config.SSOProviderTypeSAML:
				info.LoginURL = "/api/saml/" + provider.ID + "/login"
			}
			ssoProviders = append(ssoProviders, info)
		}

		hasAuthentication := os.Getenv("PULSE_AUTH_USER") != "" ||
			os.Getenv("REQUIRE_AUTH") == "true" ||
			r.config.AuthUser != "" ||
			r.config.AuthPass != "" ||
			r.config.HasAPITokens() ||
			r.config.ProxyAuthSecret != "" ||
			r.hostedMode ||
			hasEnabledSSO
		requiresAuth := r.config.HasAPITokens() ||
			(r.config.AuthUser != "" && r.config.AuthPass != "") ||
			r.config.ProxyAuthSecret != "" ||
			hasEnabledSSO

		publicStatus := publicSecurityStatusResponse{
			DetailLevel:        securityStatusDetailPublic,
			HasAuthentication:  hasAuthentication,
			RequiresAuth:       requiresAuth,
			SSOEnabled:         hasEnabledSSO,
			HideLocalLogin:     r.config.HideLocalLogin,
			SSOProviders:       ssoProviders,
			PresentationPolicy: r.securityStatusPresentationPolicy(),
		}

		// Bearer and query-string tokens are intentionally not accepted here. The
		// public endpoint exposes only login discovery until a session, proxy
		// identity, or X-API-Token proves the caller's authority.
		authSnapshot := r.buildSecurityStatusAuthSnapshot(req)
		if !authSnapshot.authenticated {
			_ = json.NewEncoder(w).Encode(publicStatus)
			return
		}

		hasProxyAuth := r.config.ProxyAuthSecret != ""
		proxyAuthUsername := ""
		proxyAuthIsAdmin := false
		if authSnapshot.authMethod == "proxy" {
			if valid, username, isAdmin := CheckProxyAuth(r.config, req); valid {
				proxyAuthUsername = username
				proxyAuthIsAdmin = isAdmin
			}
		}

		ssoSessionUsername := ""
		ssoSessionDisplayName := ""
		if hasEnabledSSO {
			if cookie, err := readSessionCookie(req); err == nil && cookie.Value != "" && ValidateSession(cookie.Value) {
				session := GetSessionStore().GetSession(cookie.Value)
				if session != nil && (strings.TrimSpace(session.OIDCIssuer) != "" || strings.TrimSpace(session.SAMLProviderID) != "") {
					ssoSessionUsername = GetSessionUsername(cookie.Value)
					ssoSessionDisplayName = GetSessionDisplayUsername(cookie.Value)
				}
			}
		}

		publicStatus.DetailLevel = securityStatusDetailAuthenticated
		authenticatedStatus := authenticatedSecurityStatusResponse{
			publicSecurityStatusResponse: publicStatus,
			HasProxyAuth:                 authSnapshot.authMethod == "proxy",
			ProxyAuthLogoutURL:           r.config.ProxyAuthLogoutURL,
			ProxyAuthUsername:            proxyAuthUsername,
			ProxyAuthIsAdmin:             proxyAuthIsAdmin,
			SSOSessionUsername:           ssoSessionUsername,
			SSOSessionDisplayName:        ssoSessionDisplayName,
			TokenScopes:                  authSnapshot.tokenScopes(),
			SessionCapabilities:          r.securityStatusSessionCapabilities(req.Context()),
			SettingsCapabilities:         r.securityStatusSettingsCapabilitiesFromSnapshot(authSnapshot),
		}
		if primaryOIDCConfig != nil && ssoSessionUsername != "" {
			authenticatedStatus.SSOLogoutURL = primaryOIDCConfig.LogoutURL
		}
		if !authSnapshot.canAccessAdminSurface(config.ScopeSettingsRead) {
			_ = json.NewEncoder(w).Encode(authenticatedStatus)
			return
		}

		configuredButPendingRestart := false
		authLastModified := ""
		if stat, err := os.Stat(resolveAuthEnvPath(r.config.ConfigPath)); err == nil {
			authLastModified = stat.ModTime().UTC().Format(time.RFC3339)
			if !hasAuthentication && r.config.AuthUser == "" && r.config.AuthPass == "" {
				configuredButPendingRestart = true
			}
		}

		clientIP := GetClientIP(req)
		isPrivateNetwork := isPrivateIP(clientIP)
		trustedNetworks := []string{}
		if nets := os.Getenv("PULSE_TRUSTED_NETWORKS"); nets != "" {
			trustedNetworks = strings.Split(nets, ",")
		}

		authenticatedStatus.DetailLevel = securityStatusDetailPrivileged
		authenticatedStatus.HasProxyAuth = hasProxyAuth
		status := privilegedSecurityStatusResponse{
			authenticatedSecurityStatusResponse: authenticatedStatus,
			APITokenConfigured:                  r.config.HasAPITokens(),
			APITokenHint:                        r.config.PrimaryAPITokenHint(),
			ExportProtected:                     r.config.HasAPITokens() || os.Getenv("ALLOW_UNPROTECTED_EXPORT") != "true",
			UnprotectedExportAllowed:            os.Getenv("ALLOW_UNPROTECTED_EXPORT") == "true",
			ConfiguredButPendingRestart:         configuredButPendingRestart,
			HasAuditLogging:                     os.Getenv("PULSE_AUDIT_LOG") == "true" || os.Getenv("AUDIT_LOG_ENABLED") == "true",
			CredentialsEncrypted:                true,
			HasHTTPS:                            req.TLS != nil || strings.EqualFold(req.Header.Get("X-Forwarded-Proto"), "https"),
			ClientIP:                            clientIP,
			IsPrivateNetwork:                    isPrivateNetwork,
			IsTrustedNetwork:                    isTrustedNetwork(clientIP, trustedNetworks),
			PublicAccess:                        !isPrivateNetwork,
			AuthUsername:                        r.config.AuthUser,
			AuthLastModified:                    authLastModified,
			AgentURL:                            r.resolvePublicURL(req),
		}

		_ = json.NewEncoder(w).Encode(status)
	})

	// Quick security setup route - using fixed version
	r.mux.HandleFunc("/api/security/quick-setup", handleQuickSecuritySetupFixed(r))
	r.mux.HandleFunc("/api/security/dev/reset-first-run", r.handleResetFirstRunSecurity)

	// API token regeneration endpoint
	r.mux.HandleFunc("/api/security/regenerate-token", r.HandleRegenerateAPIToken)

	// API token validation endpoint
	r.mux.HandleFunc("/api/security/validate-token", r.HandleValidateAPIToken)

	// Apply security restart endpoint
	r.mux.HandleFunc("/api/security/apply-restart", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			if !r.authorizeSecurityRestart(w, req) {
				return
			}

			// Only allow restart if we're running under systemd (safer)
			isSystemd := os.Getenv("INVOCATION_ID") != ""

			if !isSystemd {
				response := map[string]interface{}{
					"success": false,
					"message": "Automatic restart is only available when running under systemd. Please restart Pulse manually.",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}

			// Retire the legacy filesystem recovery toggle before restart. Recovery
			// remains available through the localhost-bound recovery session flow.
			recoveryFile := filepath.Join(r.config.DataPath, ".auth_recovery")
			if err := os.Remove(recoveryFile); err != nil && !os.IsNotExist(err) {
				log.Warn().Err(err).Str("path", recoveryFile).Msg("Failed to remove legacy recovery flag file")
			}

			// Schedule restart with full service restart to pick up new config
			go func() {
				time.Sleep(2 * time.Second)
				log.Info().Msg("Triggering restart to apply security settings")

				// We need to do a full systemctl restart to pick up new environment variables
				// First try daemon-reload
				cmd := exec.Command("sudo", "-n", "systemctl", "daemon-reload")
				if err := cmd.Run(); err != nil {
					log.Error().Err(err).Msg("Failed to reload systemd daemon")
				}

				// Then restart the service - this will kill us and restart with new env
				time.Sleep(500 * time.Millisecond)
				// Try to restart with the detected service name
				serviceName := detectServiceName()
				cmd = exec.Command("sudo", "-n", "systemctl", "restart", serviceName)
				if err := cmd.Run(); err != nil {
					log.Error().Err(err).Str("service", serviceName).Msg("Failed to restart service, falling back to exit")
					// Fallback to exit if restart fails
					os.Exit(0)
				}
				// If restart succeeds, we'll be killed by systemctl
			}()

			response := map[string]interface{}{
				"success": true,
				"message": "Restarting Pulse to apply security settings...",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Recovery endpoint - requires localhost access OR valid recovery token
	r.mux.HandleFunc("/api/security/recovery", func(w http.ResponseWriter, req *http.Request) {
		// Get client IP
		isLoopback := isDirectLoopbackRequest(req)
		clientIP := GetClientIP(req)

		// Check for recovery token in header
		recoveryToken := req.Header.Get("X-Recovery-Token")
		hasValidToken := false
		if recoveryToken != "" {
			hasValidToken = GetRecoveryTokenStore().ValidateRecoveryTokenConstantTime(recoveryToken, clientIP)
		}

		// Only allow from localhost OR with valid recovery token
		if !isLoopback && !hasValidToken {
			log.Warn().
				Str("ip", clientIP).
				Bool("direct_loopback", isLoopback).
				Bool("has_token", recoveryToken != "").
				Msg("Unauthorized recovery endpoint access attempt")
			http.Error(w, "Recovery endpoint requires localhost access or valid recovery token", http.StatusForbidden)
			return
		}

		if req.Method == http.MethodPost {
			// Parse action
			var recoveryRequest struct {
				Action   string `json:"action"`
				Duration int    `json:"duration,omitempty"` // Duration in minutes for token generation
			}

			if err := json.NewDecoder(req.Body).Decode(&recoveryRequest); err != nil {
				http.Error(w, "Invalid request", http.StatusBadRequest)
				return
			}

			response := map[string]interface{}{}

			switch recoveryRequest.Action {
			case "generate_token":
				// Only allow token generation from localhost
				if !isLoopback {
					http.Error(w, "Token generation only allowed from localhost", http.StatusForbidden)
					return
				}

				// Default to 15 minutes if not specified
				duration := 15
				if recoveryRequest.Duration > 0 && recoveryRequest.Duration <= 60 {
					duration = recoveryRequest.Duration
				}

				token, err := GetRecoveryTokenStore().GenerateRecoveryToken(time.Duration(duration)*time.Minute, clientIP)
				if err != nil {
					log.Error().Err(err).Msg("Failed to generate recovery token")
					response["success"] = false
					response["message"] = "Failed to generate recovery token"
				} else {
					response["success"] = true
					response["token"] = token
					response["expires_in_minutes"] = duration
					response["message"] = fmt.Sprintf("Recovery token generated. Valid for %d minutes.", duration)
					log.Warn().
						Str("ip", clientIP).
						Bool("direct_loopback", isLoopback).
						Int("duration_minutes", duration).
						Msg("Recovery token generated")
				}

			case "disable_auth":
				recoveryUser := strings.TrimSpace(r.config.AuthUser)
				if recoveryUser == "" {
					recoveryUser = "recovery"
				}
				if err := r.establishRecoverySession(w, req, recoveryUser); err != nil {
					log.Error().Err(err).Msg("Failed to establish recovery session")
					response["success"] = false
					response["message"] = "Failed to enable recovery mode"
				} else {
					recoveryFile := filepath.Join(r.config.DataPath, ".auth_recovery")
					if err := os.Remove(recoveryFile); err != nil && !os.IsNotExist(err) {
						log.Warn().Err(err).Str("path", recoveryFile).Msg("Failed to remove legacy recovery flag file")
					}
					response["success"] = true
					response["message"] = "Recovery mode enabled for this local browser session only."
					log.Warn().
						Str("ip", clientIP).
						Bool("direct_loopback", isLoopback).
						Bool("via_token", hasValidToken).
						Msg("AUTH RECOVERY: Recovery session established")
				}

			case "enable_auth":
				r.clearSession(w, req)
				recoveryFile := filepath.Join(r.config.DataPath, ".auth_recovery")
				if err := os.Remove(recoveryFile); err != nil && !os.IsNotExist(err) {
					log.Error().Err(err).Msg("Failed to disable recovery mode")
					response["success"] = false
					response["message"] = "Failed to disable recovery mode"
				} else {
					response["success"] = true
					response["message"] = "Recovery mode disabled for this browser session."
					log.Info().Msg("AUTH RECOVERY: Recovery session cleared via recovery endpoint")
				}

			default:
				response["success"] = false
				response["message"] = "Invalid action. Use 'disable_auth' or 'enable_auth'"
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if req.Method == http.MethodGet {
			recoveryMode := false
			if cookie, err := readSessionCookie(req); err == nil && cookie.Value != "" && ValidateSession(cookie.Value) {
				if session := GetSessionStore().GetSession(cookie.Value); requestMatchesRecoverySession(req, session) {
					recoveryMode = true
				}
			}
			response := map[string]interface{}{
				"recovery_mode": recoveryMode,
				"message":       "Recovery endpoint accessible from localhost only; recovery sessions are browser-bound",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	// Agent WebSocket for AI command execution
	r.mux.HandleFunc("/api/agent/ws", r.handleAgentWebSocket)

	// Unified Agent endpoints (public but rate limited)
	r.mux.HandleFunc("/install.sh", r.downloadLimiter.Middleware(r.handleDownloadUnifiedInstallScript))
	r.mux.HandleFunc("/install.ps1", r.downloadLimiter.Middleware(r.handleDownloadUnifiedInstallScriptPS))
	r.mux.HandleFunc("/download/pulse-agent", r.downloadLimiter.Middleware(r.handleDownloadUnifiedAgent))

	r.mux.HandleFunc("/api/agent/version", r.handleAgentVersion)
	r.mux.HandleFunc("/api/server/info", r.handleServerInfo)

	// WebSocket endpoint
	r.mux.HandleFunc("/ws", r.handleWebSocket)

	// Simple stats page - requires authentication
	r.mux.HandleFunc("/simple-stats", RequireAuth(r.config, r.handleSimpleStats))
}

type ssoAdminEndpointAdapter struct {
	router *Router
}

func newSSOAdminRuntime(router *Router) extensions.SSOAdminRuntime {
	runtime := extensions.SSOAdminRuntime{
		GetClientIP: GetClientIP,
		AllowAuthRequest: func(clientIP string) bool {
			return authLimiter.Allow(clientIP)
		},
		LogAuditEvent: func(ctx context.Context, event, path string, success bool, message, clientIP string) {
			LogAuditEventForTenant(GetOrgID(ctx), event, "", clientIP, path, success, message)
		},
		WriteError: writeErrorResponse,
		RequireFeature: func(ctx context.Context, feature string) error {
			if router == nil || router.licenseHandlers == nil {
				return fmt.Errorf("license service unavailable")
			}
			svc := router.licenseHandlers.Service(ctx)
			if svc == nil {
				return fmt.Errorf("license service unavailable")
			}
			return svc.RequireFeature(feature)
		},
		WriteLicenseRequired: WriteLicenseRequired,
	}

	if router == nil {
		return runtime
	}

	runtime.TestSAMLConnection = func(ctx context.Context, cfg *extensions.SAMLTestConfig) extensions.SSOTestResponse {
		return toExtensionSSOTestResponse(router.testSAMLConnection(ctx, toAPISAMLTestConfig(cfg)))
	}
	runtime.TestOIDCConnection = func(ctx context.Context, cfg *extensions.OIDCTestConfig) extensions.SSOTestResponse {
		return toExtensionSSOTestResponse(router.testOIDCConnection(ctx, toAPIOIDCTestConfig(cfg)))
	}
	runtime.PreviewSAMLMetadata = previewSAMLMetadataFromRuntime
	runtime.IsValidProviderID = validateProviderID
	runtime.GetSSOConfigSnapshot = func() extensions.SSOConfigSnapshot {
		return toExtensionSSOConfigSnapshot(router.ensureSSOConfig())
	}
	runtime.SaveSSOConfigSnapshot = func(snapshot extensions.SSOConfigSnapshot) error {
		if router == nil {
			return nil
		}
		previous := router.ssoConfig
		router.ssoConfig = toCoreSSOConfig(snapshot)
		if err := router.saveSSOConfig(); err != nil {
			router.ssoConfig = previous
			return err
		}
		return nil
	}
	runtime.GetPublicURL = func() string {
		if router == nil || router.config == nil {
			return ""
		}
		return router.config.PublicURL
	}
	runtime.InitializeSAMLProvider = func(ctx context.Context, id string, samlCfg *extensions.SAMLProviderConfig) error {
		if router == nil || router.samlManager == nil || samlCfg == nil {
			return nil
		}
		return router.samlManager.InitializeProvider(ctx, id, toCoreSAMLProviderConfig(samlCfg))
	}
	runtime.RemoveSAMLProvider = func(id string) {
		if router == nil || router.samlManager == nil {
			return
		}
		router.samlManager.RemoveProvider(id)
	}
	runtime.InitializeOIDCProvider = func(ctx context.Context, id string, provider *extensions.SSOProvider) error {
		if router == nil || router.oidcManager == nil || provider == nil {
			return nil
		}
		coreProvider := toCoreSSOProvider(*provider)
		return router.oidcManager.InitializeProvider(ctx, id, &coreProvider, "")
	}
	runtime.RemoveOIDCProvider = func(id string) {
		if router == nil || router.oidcManager == nil {
			return
		}
		router.oidcManager.RemoveService(id)
	}
	return runtime
}

func toAPISAMLTestConfig(cfg *extensions.SAMLTestConfig) *SAMLTestConfig {
	if cfg == nil {
		return nil
	}
	return &SAMLTestConfig{
		IDPMetadataURL: cfg.IDPMetadataURL,
		IDPMetadataXML: cfg.IDPMetadataXML,
		IDPSSOURL:      cfg.IDPSSOURL,
		IDPCertificate: cfg.IDPCertificate,
	}
}

func toAPIOIDCTestConfig(cfg *extensions.OIDCTestConfig) *OIDCTestConfig {
	if cfg == nil {
		return nil
	}
	return &OIDCTestConfig{
		IssuerURL: cfg.IssuerURL,
		ClientID:  cfg.ClientID,
	}
}

func toExtensionSSOTestResponse(resp SSOTestResponse) extensions.SSOTestResponse {
	return extensions.SSOTestResponse{
		Success: resp.Success,
		Message: resp.Message,
		Error:   resp.Error,
		Details: toExtensionSSOTestDetails(resp.Details),
	}
}

func toExtensionSSOTestDetails(details *SSOTestDetails) *extensions.SSOTestDetails {
	if details == nil {
		return nil
	}

	converted := &extensions.SSOTestDetails{
		Type:             details.Type,
		EntityID:         details.EntityID,
		SSOURL:           details.SSOURL,
		SLOURL:           details.SLOURL,
		TokenEndpoint:    details.TokenEndpoint,
		UserinfoEndpoint: details.UserinfoEndpoint,
		JWKSURI:          details.JWKSURI,
		SupportedScopes:  details.SupportedScopes,
	}

	if len(details.Certificates) > 0 {
		converted.Certificates = make([]extensions.CertificateInfo, 0, len(details.Certificates))
		for _, cert := range details.Certificates {
			converted.Certificates = append(converted.Certificates, extensions.CertificateInfo{
				Subject:   cert.Subject,
				Issuer:    cert.Issuer,
				NotBefore: cert.NotBefore,
				NotAfter:  cert.NotAfter,
				IsExpired: cert.IsExpired,
			})
		}
	}

	return converted
}

func toExtensionSSOConfigSnapshot(cfg *config.SSOConfig) extensions.SSOConfigSnapshot {
	if cfg == nil {
		return extensions.SSOConfigSnapshot{
			Providers:              []extensions.SSOProvider{},
			AllowMultipleProviders: true,
		}
	}

	out := extensions.SSOConfigSnapshot{
		Providers:              make([]extensions.SSOProvider, 0, len(cfg.Providers)),
		DefaultProviderID:      cfg.DefaultProviderID,
		AllowMultipleProviders: cfg.AllowMultipleProviders,
	}

	for _, p := range cfg.Providers {
		out.Providers = append(out.Providers, toExtensionSSOProvider(p))
	}

	return out
}

func toExtensionSSOProvider(p config.SSOProvider) extensions.SSOProvider {
	out := extensions.SSOProvider{
		ID:                p.ID,
		Name:              p.Name,
		Type:              extensions.SSOProviderType(p.Type),
		Enabled:           p.Enabled,
		DisplayName:       p.DisplayName,
		IconURL:           p.IconURL,
		Priority:          p.Priority,
		AllowedGroups:     p.AllowedGroups,
		AllowedDomains:    p.AllowedDomains,
		AllowedEmails:     p.AllowedEmails,
		GroupsClaim:       p.GroupsClaim,
		GroupRoleMappings: p.GroupRoleMappings,
	}

	if p.OIDC != nil {
		out.OIDC = &extensions.OIDCProviderConfig{
			IssuerURL:       p.OIDC.IssuerURL,
			ClientID:        p.OIDC.ClientID,
			ClientSecret:    p.OIDC.ClientSecret,
			RedirectURL:     p.OIDC.RedirectURL,
			LogoutURL:       p.OIDC.LogoutURL,
			Scopes:          p.OIDC.Scopes,
			UsernameClaim:   p.OIDC.UsernameClaim,
			EmailClaim:      p.OIDC.EmailClaim,
			CABundle:        p.OIDC.CABundle,
			ClientSecretSet: p.OIDC.ClientSecretSet,
		}
	}

	if p.SAML != nil {
		out.SAML = &extensions.SAMLProviderConfig{
			IDPMetadataURL:       p.SAML.IDPMetadataURL,
			IDPMetadataXML:       p.SAML.IDPMetadataXML,
			IDPSSOURL:            p.SAML.IDPSSOURL,
			IDPSLOURL:            p.SAML.IDPSLOURL,
			IDPCertificate:       p.SAML.IDPCertificate,
			IDPCertFile:          p.SAML.IDPCertFile,
			IDPEntityID:          p.SAML.IDPEntityID,
			IDPIssuer:            p.SAML.IDPIssuer,
			SPEntityID:           p.SAML.SPEntityID,
			SPACSPath:            p.SAML.SPACSPath,
			SPMetadataPath:       p.SAML.SPMetadataPath,
			SPCertificate:        p.SAML.SPCertificate,
			SPPrivateKey:         p.SAML.SPPrivateKey,
			SPCertFile:           p.SAML.SPCertFile,
			SPKeyFile:            p.SAML.SPKeyFile,
			SignRequests:         p.SAML.SignRequests,
			WantAssertionsSigned: p.SAML.WantAssertionsSigned,
			AllowUnencrypted:     p.SAML.AllowUnencrypted,
			UsernameAttr:         p.SAML.UsernameAttr,
			EmailAttr:            p.SAML.EmailAttr,
			GroupsAttr:           p.SAML.GroupsAttr,
			FirstNameAttr:        p.SAML.FirstNameAttr,
			LastNameAttr:         p.SAML.LastNameAttr,
			NameIDFormat:         p.SAML.NameIDFormat,
			ForceAuthn:           p.SAML.ForceAuthn,
			AllowIDPInitiated:    p.SAML.AllowIDPInitiated,
			RelayStateTemplate:   p.SAML.RelayStateTemplate,
		}
	}

	return out
}

func toCoreSSOConfig(snapshot extensions.SSOConfigSnapshot) *config.SSOConfig {
	cfg := &config.SSOConfig{
		Providers:              make([]config.SSOProvider, 0, len(snapshot.Providers)),
		DefaultProviderID:      snapshot.DefaultProviderID,
		AllowMultipleProviders: snapshot.AllowMultipleProviders,
	}
	for _, p := range snapshot.Providers {
		cfg.Providers = append(cfg.Providers, toCoreSSOProvider(p))
	}
	return cfg
}

func toCoreSSOProvider(p extensions.SSOProvider) config.SSOProvider {
	out := config.SSOProvider{
		ID:                p.ID,
		Name:              p.Name,
		Type:              config.SSOProviderType(p.Type),
		Enabled:           p.Enabled,
		DisplayName:       p.DisplayName,
		IconURL:           p.IconURL,
		Priority:          p.Priority,
		AllowedGroups:     p.AllowedGroups,
		AllowedDomains:    p.AllowedDomains,
		AllowedEmails:     p.AllowedEmails,
		GroupsClaim:       p.GroupsClaim,
		GroupRoleMappings: p.GroupRoleMappings,
	}

	if p.OIDC != nil {
		out.OIDC = &config.OIDCProviderConfig{
			IssuerURL:       p.OIDC.IssuerURL,
			ClientID:        p.OIDC.ClientID,
			ClientSecret:    p.OIDC.ClientSecret,
			RedirectURL:     p.OIDC.RedirectURL,
			LogoutURL:       p.OIDC.LogoutURL,
			Scopes:          p.OIDC.Scopes,
			UsernameClaim:   p.OIDC.UsernameClaim,
			EmailClaim:      p.OIDC.EmailClaim,
			CABundle:        p.OIDC.CABundle,
			ClientSecretSet: p.OIDC.ClientSecretSet,
		}
	}

	if p.SAML != nil {
		out.SAML = toCoreSAMLProviderConfig(p.SAML)
	}

	return out
}

func toCoreSAMLProviderConfig(cfg *extensions.SAMLProviderConfig) *config.SAMLProviderConfig {
	if cfg == nil {
		return nil
	}
	return &config.SAMLProviderConfig{
		IDPMetadataURL:       cfg.IDPMetadataURL,
		IDPMetadataXML:       cfg.IDPMetadataXML,
		IDPSSOURL:            cfg.IDPSSOURL,
		IDPSLOURL:            cfg.IDPSLOURL,
		IDPCertificate:       cfg.IDPCertificate,
		IDPCertFile:          cfg.IDPCertFile,
		IDPEntityID:          cfg.IDPEntityID,
		IDPIssuer:            cfg.IDPIssuer,
		SPEntityID:           cfg.SPEntityID,
		SPACSPath:            cfg.SPACSPath,
		SPMetadataPath:       cfg.SPMetadataPath,
		SPCertificate:        cfg.SPCertificate,
		SPPrivateKey:         cfg.SPPrivateKey,
		SPCertFile:           cfg.SPCertFile,
		SPKeyFile:            cfg.SPKeyFile,
		SignRequests:         cfg.SignRequests,
		WantAssertionsSigned: cfg.WantAssertionsSigned,
		AllowUnencrypted:     cfg.AllowUnencrypted,
		UsernameAttr:         cfg.UsernameAttr,
		EmailAttr:            cfg.EmailAttr,
		GroupsAttr:           cfg.GroupsAttr,
		FirstNameAttr:        cfg.FirstNameAttr,
		LastNameAttr:         cfg.LastNameAttr,
		NameIDFormat:         cfg.NameIDFormat,
		ForceAuthn:           cfg.ForceAuthn,
		AllowIDPInitiated:    cfg.AllowIDPInitiated,
		RelayStateTemplate:   cfg.RelayStateTemplate,
	}
}

func previewSAMLMetadataFromRuntime(ctx context.Context, req extensions.MetadataPreviewRequest) (extensions.MetadataPreviewResponse, error) {
	var (
		rawXML   []byte
		metadata *saml.EntityDescriptor
		err      error
	)

	httpClient := newTestHTTPClient()

	if req.MetadataURL != "" {
		if !validateURL(req.MetadataURL, []string{"https", "http"}) {
			return extensions.MetadataPreviewResponse{}, &extensions.MetadataPreviewError{
				Code:    "validation_error",
				Message: "Invalid metadata URL",
			}
		}
		rawXML, metadata, err = fetchSAMLMetadataFromURL(ctx, httpClient, req.MetadataURL)
		if err != nil {
			log.Error().Err(err).Msg("Failed to fetch SAML metadata for preview")
			return extensions.MetadataPreviewResponse{}, &extensions.MetadataPreviewError{
				Code:    "fetch_error",
				Message: "Failed to fetch metadata from the provided URL",
			}
		}
	} else {
		rawXML = []byte(req.MetadataXML)
		metadata, err = parseSAMLMetadataXML(rawXML)
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse SAML metadata XML for preview")
			return extensions.MetadataPreviewResponse{}, &extensions.MetadataPreviewError{
				Code:    "parse_error",
				Message: "Failed to parse metadata XML",
			}
		}
	}

	parsed := &extensions.ParsedMetadataInfo{
		EntityID: metadata.EntityID,
	}

	if len(metadata.IDPSSODescriptors) > 0 {
		idpDesc := metadata.IDPSSODescriptors[0]
		for _, sso := range idpDesc.SingleSignOnServices {
			if sso.Binding == saml.HTTPPostBinding || sso.Binding == saml.HTTPRedirectBinding {
				parsed.SSOURL = sso.Location
				break
			}
		}
		for _, slo := range idpDesc.SingleLogoutServices {
			parsed.SLOURL = slo.Location
			break
		}
		for _, nid := range idpDesc.NameIDFormats {
			parsed.NameIDFormats = append(parsed.NameIDFormats, string(nid))
		}
		for _, kd := range idpDesc.KeyDescriptors {
			if kd.Use == "signing" || kd.Use == "" {
				for _, x509Cert := range kd.KeyInfo.X509Data.X509Certificates {
					certInfo := extractCertificateInfo(x509Cert.Data)
					if certInfo != nil {
						parsed.Certificates = append(parsed.Certificates, extensions.CertificateInfo{
							Subject:   certInfo.Subject,
							Issuer:    certInfo.Issuer,
							NotBefore: certInfo.NotBefore,
							NotAfter:  certInfo.NotAfter,
							IsExpired: certInfo.IsExpired,
						})
					}
				}
			}
		}
	}

	return extensions.MetadataPreviewResponse{
		XML:    formatXML(rawXML),
		Parsed: parsed,
	}, nil
}

var _ extensions.SSOAdminEndpoints = ssoAdminEndpointAdapter{}

func (a ssoAdminEndpointAdapter) HandleProvidersCollection(w http.ResponseWriter, req *http.Request) {
	if a.router == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "sso_unavailable", "SSO management is unavailable", nil)
		return
	}
	a.router.handleSSOProviders(w, req)
}

func (a ssoAdminEndpointAdapter) HandleProviderItem(w http.ResponseWriter, req *http.Request) {
	if a.router == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "sso_unavailable", "SSO management is unavailable", nil)
		return
	}
	a.router.handleSSOProvider(w, req)
}

func (a ssoAdminEndpointAdapter) HandleProviderTest(w http.ResponseWriter, req *http.Request) {
	if a.router == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "sso_unavailable", "SSO management is unavailable", nil)
		return
	}
	a.router.handleTestSSOProvider(w, req)
}

func (a ssoAdminEndpointAdapter) HandleMetadataPreview(w http.ResponseWriter, req *http.Request) {
	if a.router == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "sso_unavailable", "SSO management is unavailable", nil)
		return
	}
	a.router.handleMetadataPreview(w, req)
}
