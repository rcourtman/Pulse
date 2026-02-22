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
	"github.com/rcourtman/pulse-go-rewrite/internal/system"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
	"github.com/rs/zerolog/log"
)

func (r *Router) registerAuthSecurityInstallRoutes() {
	// API routes
	r.mux.HandleFunc("/api/health", r.handleHealth)
	r.mux.HandleFunc("/api/state", r.handleState)
	r.mux.HandleFunc("/api/version", r.handleVersion)
	r.mux.HandleFunc("/api/install/install-docker.sh", r.handleDownloadDockerInstallerScript)
	r.mux.HandleFunc("/api/install/install.sh", r.handleDownloadUnifiedInstallScript)
	r.mux.HandleFunc("/api/install/install.ps1", r.handleDownloadUnifiedInstallScriptPS)
	r.mux.HandleFunc("/api/security/validate-bootstrap-token", r.handleValidateBootstrapToken)
	// Security routes
	r.mux.HandleFunc("/api/security/change-password", r.handleChangePassword)
	r.mux.HandleFunc("/api/logout", r.handleLogout)
	r.mux.HandleFunc("/api/login", r.handleLogin)
	r.mux.HandleFunc("/api/security/reset-lockout", r.handleResetLockout)
	r.mux.HandleFunc("/api/security/oidc", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.handleOIDCConfig)))
	r.mux.HandleFunc("/api/oidc/login", r.handleOIDCLogin)
	r.mux.HandleFunc(config.DefaultOIDCCallbackPath, r.handleOIDCCallback)
	ssoAdminEndpoints := resolveSSOAdminEndpoints(ssoAdminEndpointAdapter{router: r}, newSSOAdminRuntime(r))
	// Per-provider SSO OIDC routes: /api/oidc/{providerID}/login and /api/oidc/{providerID}/callback
	// Use a prefix handler since Go 1.x ServeMux doesn't support path params.
	// Requests matching /api/oidc/{something}/ are dispatched here; the legacy
	// /api/oidc/login and /api/oidc/callback routes registered above take priority
	// because ServeMux prefers longer exact matches over prefix patterns.
	r.mux.HandleFunc("/api/oidc/", func(w http.ResponseWriter, req *http.Request) {
		// Determine which sub-endpoint was requested
		parts := strings.Split(strings.TrimPrefix(req.URL.Path, "/"), "/")
		// Expected: ["api", "oidc", "{providerID}", "{endpoint}"]
		if len(parts) < 4 {
			http.NotFound(w, req)
			return
		}
		endpoint := parts[3]
		switch endpoint {
		case "login":
			r.handleSSOOIDCLogin(w, req)
		case "callback":
			r.handleSSOOIDCCallback(w, req)
		default:
			http.NotFound(w, req)
		}
	})
	r.mux.HandleFunc("/api/security/sso/providers/test", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsWriteScope(r.config, w, req) {
			return
		}
		ssoAdminEndpoints.HandleProviderTest(w, req)
	}))
	r.mux.HandleFunc("/api/security/sso/providers/metadata/preview", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureSettingsReadScope(r.config, w, req) {
			return
		}
		ssoAdminEndpoints.HandleMetadataPreview(w, req)
	}))
	r.mux.HandleFunc("/api/security/sso/providers", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
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
	}))
	r.mux.HandleFunc("/api/security/sso/providers/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
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
	}))

	// SAML login flow routes (unauthenticated - these are login/callback endpoints)
	r.mux.HandleFunc("/api/saml/", func(w http.ResponseWriter, req *http.Request) {
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
	})

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
		if req.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")

			// Check for basic auth configuration
			// Check both environment variables and loaded config
			oidcCfg := r.ensureOIDCConfig()
			ssoCfg := r.ensureSSOConfig()
			hasAuthentication := os.Getenv("PULSE_AUTH_USER") != "" ||
				os.Getenv("REQUIRE_AUTH") == "true" ||
				r.config.AuthUser != "" ||
				r.config.AuthPass != "" ||
				(oidcCfg != nil && oidcCfg.Enabled) ||
				r.config.HasAPITokens() ||
				r.config.ProxyAuthSecret != "" ||
				r.hostedMode ||
				(ssoCfg != nil && ssoCfg.HasEnabledProviders())

			// Check if .env file exists but hasn't been loaded yet (pending restart)
			configuredButPendingRestart := false
			envPath := filepath.Join(r.config.ConfigPath, ".env")
			if envPath == "" || r.config.ConfigPath == "" {
				envPath = "/etc/pulse/.env"
			}

			authLastModified := ""
			if stat, err := os.Stat(envPath); err == nil {
				authLastModified = stat.ModTime().UTC().Format(time.RFC3339)
				if !hasAuthentication && r.config.AuthUser == "" && r.config.AuthPass == "" {
					configuredButPendingRestart = true
				}
			}

			// Check for audit logging
			hasAuditLogging := os.Getenv("PULSE_AUDIT_LOG") == "true" || os.Getenv("AUDIT_LOG_ENABLED") == "true"

			// Credentials are always encrypted in current implementation
			credentialsEncrypted := true

			// Check network context
			clientIP := GetClientIP(req)
			isPrivateNetwork := isPrivateIP(clientIP)

			// Get trusted networks from environment
			trustedNetworks := []string{}
			if nets := os.Getenv("PULSE_TRUSTED_NETWORKS"); nets != "" {
				trustedNetworks = strings.Split(nets, ",")
			}
			isTrustedNetwork := isTrustedNetwork(clientIP, trustedNetworks)

			// Determine whether the caller is authenticated before exposing sensitive fields
			// Also track token scopes for kiosk/limited-access scenarios
			//
			// SECURITY: Do NOT check ?token= query param here - this public endpoint would
			// act as a token validity oracle, allowing attackers to probe for valid tokens
			// without rate limiting. Only check session cookies and X-API-Token header.
			isAuthenticated := false
			var tokenScopes []string
			if cookie, err := req.Cookie("pulse_session"); err == nil && cookie.Value != "" && ValidateSession(cookie.Value) {
				isAuthenticated = true
			} else if token := strings.TrimSpace(req.Header.Get("X-API-Token")); token != "" {
				if record, ok := r.config.ValidateAPIToken(token); ok {
					isAuthenticated = true
					tokenScopes = record.Scopes
				}
			}

			// Create token hint if token exists (only revealed to authenticated callers)
			apiTokenHint := ""
			if isAuthenticated {
				apiTokenHint = r.config.PrimaryAPITokenHint()
			}

			// Check for proxy auth
			hasProxyAuth := r.config.ProxyAuthSecret != ""
			proxyAuthUsername := ""
			proxyAuthIsAdmin := false
			if hasProxyAuth {
				// Check if current request has valid proxy auth
				if valid, username, isAdmin := CheckProxyAuth(r.config, req); valid {
					proxyAuthUsername = username
					proxyAuthIsAdmin = isAdmin
				}
			}

			// Check for OIDC session
			oidcUsername := ""
			if oidcCfg != nil && oidcCfg.Enabled {
				if cookie, err := req.Cookie("pulse_session"); err == nil && cookie.Value != "" {
					if ValidateSession(cookie.Value) {
						oidcUsername = GetSessionUsername(cookie.Value)
					}
				}
			}

			requiresAuth := r.config.HasAPITokens() ||
				(r.config.AuthUser != "" && r.config.AuthPass != "") ||
				(r.config.OIDC != nil && r.config.OIDC.Enabled) ||
				r.config.ProxyAuthSecret != "" ||
				(ssoCfg != nil && ssoCfg.HasEnabledProviders())

			// Resolve the public URL for agent install commands
			// If PULSE_PUBLIC_URL is configured, use that; otherwise derive from request
			agentURL := r.resolvePublicURL(req)

			status := map[string]interface{}{
				"apiTokenConfigured":          r.config.HasAPITokens(),
				"apiTokenHint":                apiTokenHint,
				"requiresAuth":                requiresAuth,
				"exportProtected":             r.config.HasAPITokens() || os.Getenv("ALLOW_UNPROTECTED_EXPORT") != "true",
				"unprotectedExportAllowed":    os.Getenv("ALLOW_UNPROTECTED_EXPORT") == "true",
				"hasAuthentication":           hasAuthentication,
				"configuredButPendingRestart": configuredButPendingRestart,
				"hasAuditLogging":             hasAuditLogging,
				"credentialsEncrypted":        credentialsEncrypted,
				"hasHTTPS":                    req.TLS != nil || strings.EqualFold(req.Header.Get("X-Forwarded-Proto"), "https"),
				"clientIP":                    clientIP,
				"isPrivateNetwork":            isPrivateNetwork,
				"isTrustedNetwork":            isTrustedNetwork,
				"publicAccess":                !isPrivateNetwork,
				"hasProxyAuth":                hasProxyAuth,
				"proxyAuthLogoutURL":          r.config.ProxyAuthLogoutURL,
				"proxyAuthUsername":           proxyAuthUsername,
				"proxyAuthIsAdmin":            proxyAuthIsAdmin,
				"authUsername":                "",
				"authLastModified":            "",
				"oidcUsername":                oidcUsername,
				"hideLocalLogin":              r.config.HideLocalLogin,
				"agentUrl":                    agentURL,
			}

			if isAuthenticated {
				status["authUsername"] = r.config.AuthUser
				status["authLastModified"] = authLastModified
			}

			// Include token scopes when authenticated via API token (for kiosk mode UI)
			if len(tokenScopes) > 0 {
				status["tokenScopes"] = tokenScopes
			}

			if oidcCfg != nil {
				status["oidcEnabled"] = oidcCfg.Enabled
				status["oidcIssuer"] = oidcCfg.IssuerURL
				status["oidcClientId"] = oidcCfg.ClientID
				status["oidcLogoutURL"] = oidcCfg.LogoutURL
				if len(oidcCfg.EnvOverrides) > 0 {
					status["oidcEnvOverrides"] = oidcCfg.EnvOverrides
				}
			}

			// Include SSO providers for login page discovery
			if ssoConfig := r.ensureSSOConfig(); ssoConfig != nil {
				enabledProviders := ssoConfig.GetEnabledProviders()
				if len(enabledProviders) > 0 {
					baseURL := r.config.PublicURL
					if baseURL == "" {
						baseURL = ""
					}
					type ssoProviderInfo struct {
						ID          string `json:"id"`
						Name        string `json:"name"`
						Type        string `json:"type"`
						DisplayName string `json:"displayName,omitempty"`
						IconURL     string `json:"iconUrl,omitempty"`
						LoginURL    string `json:"loginUrl"`
					}
					var ssoProviders []ssoProviderInfo
					for _, p := range enabledProviders {
						info := ssoProviderInfo{
							ID:          p.ID,
							Name:        p.Name,
							Type:        string(p.Type),
							DisplayName: p.DisplayName,
							IconURL:     p.IconURL,
						}
						if info.DisplayName == "" {
							info.DisplayName = p.Name
						}
						switch p.Type {
						case config.SSOProviderTypeOIDC:
							info.LoginURL = "/api/oidc/" + p.ID + "/login"
						case config.SSOProviderTypeSAML:
							info.LoginURL = "/api/saml/" + p.ID + "/login"
						}
						ssoProviders = append(ssoProviders, info)
					}
					status["ssoProviders"] = ssoProviders
				}
			}

			// Add bootstrap token location for first-run setup UI
			if r.bootstrapTokenHash != "" {
				status["bootstrapTokenPath"] = r.bootstrapTokenPath
				status["isDocker"] = os.Getenv("PULSE_DOCKER") == "true"
				status["inContainer"] = system.InContainer()
				// Try auto-detection first, then fall back to env override
				if ctid := system.DetectLXCCTID(); ctid != "" {
					status["lxcCtid"] = ctid
				} else if envCtid := os.Getenv("PULSE_LXC_CTID"); envCtid != "" {
					status["lxcCtid"] = envCtid
				}
				if containerName := system.DetectDockerContainerName(); containerName != "" {
					status["dockerContainerName"] = containerName
				}
			}

			if r.config.DisableAuthEnvDetected {
				status["deprecatedDisableAuth"] = true
				status["message"] = "DISABLE_AUTH is deprecated and no longer disables authentication. Remove the environment variable and restart Pulse to manage authentication from the UI."
			}

			json.NewEncoder(w).Encode(status)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Quick security setup route - using fixed version
	r.mux.HandleFunc("/api/security/quick-setup", handleQuickSecuritySetupFixed(r))

	// API token regeneration endpoint
	r.mux.HandleFunc("/api/security/regenerate-token", r.HandleRegenerateAPIToken)

	// API token validation endpoint
	r.mux.HandleFunc("/api/security/validate-token", r.HandleValidateAPIToken)

	// Apply security restart endpoint
	// SECURITY: Require admin auth to prevent DoS via unauthenticated service restarts
	r.mux.HandleFunc("/api/security/apply-restart", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			// SECURITY: Require authentication - this endpoint can trigger service restart (DoS risk)
			// Allow if: (1) auth is not configured yet (initial setup), or (2) caller is admin-authenticated
			authConfigured := (r.config.AuthUser != "" && r.config.AuthPass != "") ||
				r.config.HasAPITokens() ||
				r.config.ProxyAuthSecret != "" ||
				(r.config.OIDC != nil && r.config.OIDC.Enabled)
			if authConfigured {
				if !CheckAuth(r.config, w, req) {
					log.Warn().
						Str("ip", GetClientIP(req)).
						Msg("Unauthenticated apply-restart attempt blocked")
					return // CheckAuth already wrote the error
				}
				// Check proxy auth for admin status (session users with basic auth are implicitly admin)
				if r.config.ProxyAuthSecret != "" {
					if valid, username, isAdmin := CheckProxyAuth(r.config, req); valid && !isAdmin {
						log.Warn().
							Str("ip", GetClientIP(req)).
							Str("username", username).
							Msg("Non-admin user attempted service restart")
						http.Error(w, "Admin privileges required", http.StatusForbidden)
						return
					}
				}
				// Require settings:write scope for API tokens
				if !ensureSettingsWriteScope(r.config, w, req) {
					return
				}
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

			// Write a recovery flag file before restarting
			recoveryFile := filepath.Join(r.config.DataPath, ".auth_recovery")
			recoveryContent := fmt.Sprintf("Auth setup at %s\nIf locked out, delete this file and restart to disable auth temporarily\n", time.Now().Format(time.RFC3339))
			if err := os.WriteFile(recoveryFile, []byte(recoveryContent), 0600); err != nil {
				log.Warn().Err(err).Str("path", recoveryFile).Msg("Failed to write recovery flag file")
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

				token, err := GetRecoveryTokenStore().GenerateRecoveryToken(time.Duration(duration) * time.Minute)
				if err != nil {
					response["success"] = false
					response["message"] = fmt.Sprintf("Failed to generate recovery token: %v", err)
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
				// Temporarily disable auth by creating recovery file
				recoveryFile := filepath.Join(r.config.DataPath, ".auth_recovery")
				content := fmt.Sprintf("Recovery mode enabled at %s\nAuth temporarily disabled for local access\nEnabled by: %s\n", time.Now().Format(time.RFC3339), clientIP)
				if err := os.WriteFile(recoveryFile, []byte(content), 0600); err != nil {
					response["success"] = false
					response["message"] = fmt.Sprintf("Failed to enable recovery mode: %v", err)
				} else {
					response["success"] = true
					response["message"] = "Recovery mode enabled. Auth disabled for localhost. Delete .auth_recovery file to re-enable."
					log.Warn().
						Str("ip", clientIP).
						Bool("direct_loopback", isLoopback).
						Bool("via_token", hasValidToken).
						Msg("AUTH RECOVERY: Authentication disabled via recovery endpoint")
				}

			case "enable_auth":
				// Re-enable auth by removing recovery file
				recoveryFile := filepath.Join(r.config.DataPath, ".auth_recovery")
				if err := os.Remove(recoveryFile); err != nil {
					response["success"] = false
					response["message"] = fmt.Sprintf("Failed to disable recovery mode: %v", err)
				} else {
					response["success"] = true
					response["message"] = "Recovery mode disabled. Authentication re-enabled."
					log.Info().Msg("AUTH RECOVERY: Authentication re-enabled via recovery endpoint")
				}

			default:
				response["success"] = false
				response["message"] = "Invalid action. Use 'disable_auth' or 'enable_auth'"
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if req.Method == http.MethodGet {
			// Check recovery status
			recoveryFile := filepath.Join(r.config.DataPath, ".auth_recovery")
			_, err := os.Stat(recoveryFile)
			response := map[string]interface{}{
				"recovery_mode": err == nil,
				"message":       "Recovery endpoint accessible from localhost only",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	// Agent WebSocket for AI command execution
	r.mux.HandleFunc("/api/agent/ws", r.handleAgentWebSocket)

	// Docker agent download endpoints (public but rate limited)
	r.mux.HandleFunc("/install-docker-agent.sh", r.downloadLimiter.Middleware(r.handleDownloadInstallScript)) // Serves the Docker agent install script
	r.mux.HandleFunc("/install-container-agent.sh", r.downloadLimiter.Middleware(r.handleDownloadContainerAgentInstallScript))
	r.mux.HandleFunc("/download/pulse-docker-agent", r.downloadLimiter.Middleware(r.handleDownloadAgent))

	// Host agent download endpoints (public but rate limited)
	r.mux.HandleFunc("/install-host-agent.sh", r.downloadLimiter.Middleware(r.handleDownloadHostAgentInstallScript))
	r.mux.HandleFunc("/install-host-agent.ps1", r.downloadLimiter.Middleware(r.handleDownloadHostAgentInstallScriptPS))
	r.mux.HandleFunc("/uninstall-host-agent.sh", r.downloadLimiter.Middleware(r.handleDownloadHostAgentUninstallScript))
	r.mux.HandleFunc("/uninstall-host-agent.ps1", r.downloadLimiter.Middleware(r.handleDownloadHostAgentUninstallScriptPS))
	r.mux.HandleFunc("/download/pulse-host-agent", r.downloadLimiter.Middleware(r.handleDownloadHostAgent))
	r.mux.HandleFunc("/download/pulse-host-agent.sha256", r.downloadLimiter.Middleware(r.handleDownloadHostAgent))

	// Unified Agent endpoints (public but rate limited)
	r.mux.HandleFunc("/install.sh", r.downloadLimiter.Middleware(r.handleDownloadUnifiedInstallScript))
	r.mux.HandleFunc("/install.ps1", r.downloadLimiter.Middleware(r.handleDownloadUnifiedInstallScriptPS))
	r.mux.HandleFunc("/download/pulse-agent", r.downloadLimiter.Middleware(r.handleDownloadUnifiedAgent))

	r.mux.HandleFunc("/api/agent/version", r.handleAgentVersion)
	r.mux.HandleFunc("/api/server/info", r.handleServerInfo)

	// WebSocket endpoint
	r.mux.HandleFunc("/ws", r.handleWebSocket)

	// Socket.io compatibility endpoints
	r.mux.HandleFunc("/socket.io/", r.handleSocketIO)

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
	runtime.GetPublicURL = func() string {
		if router == nil || router.config == nil {
			return ""
		}
		return router.config.PublicURL
	}
	runtime.HandleListProviders = router.handleListSSOProviders
	runtime.HandleCreateProvider = router.handleCreateSSOProvider
	runtime.HandleGetProvider = router.handleGetSSOProvider
	runtime.HandleUpdateProvider = router.handleUpdateSSOProvider
	runtime.HandleDeleteProvider = router.handleDeleteSSOProvider

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
			return extensions.MetadataPreviewResponse{}, &extensions.MetadataPreviewError{
				Code:    "fetch_error",
				Message: "Failed to fetch metadata: " + err.Error(),
			}
		}
	} else {
		rawXML = []byte(req.MetadataXML)
		metadata, err = parseSAMLMetadataXML(rawXML)
		if err != nil {
			return extensions.MetadataPreviewResponse{}, &extensions.MetadataPreviewError{
				Code:    "parse_error",
				Message: "Failed to parse metadata: " + err.Error(),
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
