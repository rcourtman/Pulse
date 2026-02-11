package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/system"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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
	r.mux.HandleFunc("/api/security/sso/providers/test", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureScope(w, req, config.ScopeSettingsWrite) {
			return
		}
		r.handleTestSSOProvider(w, req)
	}))
	r.mux.HandleFunc("/api/security/sso/providers/metadata/preview", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureScope(w, req, config.ScopeSettingsRead) {
			return
		}
		r.handleMetadataPreview(w, req)
	}))
	r.mux.HandleFunc("/api/security/sso/providers", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeSettingsRead) {
				return
			}
		case http.MethodPost:
			if !ensureScope(w, req, config.ScopeSettingsWrite) {
				return
			}
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.handleSSOProviders(w, req)
	}))
	r.mux.HandleFunc("/api/security/sso/providers/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeSettingsRead) {
				return
			}
		case http.MethodPut, http.MethodDelete:
			if !ensureScope(w, req, config.ScopeSettingsWrite) {
				return
			}
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.handleSSOProvider(w, req)
	}))
	r.mux.HandleFunc("/api/security/tokens", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeSettingsRead) {
				return
			}
			r.handleListAPITokens(w, req)
		case http.MethodPost:
			if !ensureScope(w, req, config.ScopeSettingsWrite) {
				return
			}
			r.handleCreateAPIToken(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	r.mux.HandleFunc("/api/security/tokens/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, func(w http.ResponseWriter, req *http.Request) {
		if !ensureScope(w, req, config.ScopeSettingsWrite) {
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
			hasAuthentication := os.Getenv("PULSE_AUTH_USER") != "" ||
				os.Getenv("REQUIRE_AUTH") == "true" ||
				r.config.AuthUser != "" ||
				r.config.AuthPass != "" ||
				(oidcCfg != nil && oidcCfg.Enabled) ||
				r.config.HasAPITokens() ||
				r.config.ProxyAuthSecret != ""

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
				r.config.ProxyAuthSecret != ""

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
				if !ensureSettingsWriteScope(w, req) {
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
