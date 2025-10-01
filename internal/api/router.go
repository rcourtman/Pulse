package api

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

// Router handles HTTP routing
type Router struct {
	mux           *http.ServeMux
	config        *config.Config
	monitor       *monitoring.Monitor
	wsHub         *websocket.Hub
	reloadFunc    func() error
	updateManager *updates.Manager
	exportLimiter *RateLimiter
	persistence   *config.ConfigPersistence
	oidcMu        sync.Mutex
	oidcService   *OIDCService
}

// NewRouter creates a new router instance
func NewRouter(cfg *config.Config, monitor *monitoring.Monitor, wsHub *websocket.Hub, reloadFunc func() error) http.Handler {
	// Initialize persistent session and CSRF stores
	InitSessionStore(cfg.DataPath)
	InitCSRFStore(cfg.DataPath)

	r := &Router{
		mux:           http.NewServeMux(),
		config:        cfg,
		monitor:       monitor,
		wsHub:         wsHub,
		reloadFunc:    reloadFunc,
		updateManager: updates.NewManager(cfg),
		exportLimiter: NewRateLimiter(5, 1*time.Minute), // 5 attempts per minute
		persistence:   config.NewConfigPersistence(cfg.DataPath),
	}

	r.setupRoutes()

	// Start forwarding update progress to WebSocket
	go r.forwardUpdateProgress()

	// Load system settings to configure security headers
	allowEmbedding := false
	allowedOrigins := ""
	if systemSettings, err := r.persistence.LoadSystemSettings(); err == nil && systemSettings != nil {
		allowEmbedding = systemSettings.AllowEmbedding
		allowedOrigins = systemSettings.AllowedEmbedOrigins
	}

	// Apply middleware chain:
	// 1. Universal rate limiting (outermost to stop attacks early)
	// 2. Demo mode (read-only protection)
	// 3. Error handling
	// 4. Security headers with embedding configuration
	// Note: TimeoutHandler breaks WebSocket upgrades
	handler := SecurityHeadersWithConfig(r, allowEmbedding, allowedOrigins)
	handler = ErrorHandler(handler)
	handler = DemoModeMiddleware(cfg, handler)
	handler = UniversalRateLimitMiddleware(handler)
	return handler
}

// setupRoutes configures all routes
func (r *Router) setupRoutes() {
	// Create handlers
	alertHandlers := NewAlertHandlers(r.monitor, r.wsHub)
	notificationHandlers := NewNotificationHandlers(r.monitor)
	guestMetadataHandler := NewGuestMetadataHandler(r.config.DataPath)
	configHandlers := NewConfigHandlers(r.config, r.monitor, r.reloadFunc, r.wsHub, guestMetadataHandler)
	updateHandlers := NewUpdateHandlers(r.updateManager)

	// API routes
	r.mux.HandleFunc("/api/health", r.handleHealth)
	r.mux.HandleFunc("/api/state", r.handleState)
	r.mux.HandleFunc("/api/version", r.handleVersion)
	r.mux.HandleFunc("/api/storage/", r.handleStorage)
	r.mux.HandleFunc("/api/storage-charts", r.handleStorageCharts)
	r.mux.HandleFunc("/api/charts", r.handleCharts)
	r.mux.HandleFunc("/api/diagnostics", RequireAuth(r.config, r.handleDiagnostics))
	r.mux.HandleFunc("/api/config", r.handleConfig)
	r.mux.HandleFunc("/api/backups", r.handleBackups)
	r.mux.HandleFunc("/api/backups/", r.handleBackups)
	r.mux.HandleFunc("/api/backups/unified", r.handleBackups)
	r.mux.HandleFunc("/api/backups/pve", r.handleBackupsPVE)
	r.mux.HandleFunc("/api/backups/pbs", r.handleBackupsPBS)
	r.mux.HandleFunc("/api/snapshots", r.handleSnapshots)

	// Guest metadata routes
	r.mux.HandleFunc("/api/guests/metadata", guestMetadataHandler.HandleGetMetadata)
	r.mux.HandleFunc("/api/guests/metadata/", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			guestMetadataHandler.HandleGetMetadata(w, req)
		case http.MethodPut, http.MethodPost:
			guestMetadataHandler.HandleUpdateMetadata(w, req)
		case http.MethodDelete:
			guestMetadataHandler.HandleDeleteMetadata(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Update routes
	r.mux.HandleFunc("/api/updates/check", updateHandlers.HandleCheckUpdates)
	r.mux.HandleFunc("/api/updates/apply", updateHandlers.HandleApplyUpdate)
	r.mux.HandleFunc("/api/updates/status", updateHandlers.HandleUpdateStatus)

	// Config management routes
	r.mux.HandleFunc("/api/config/nodes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			configHandlers.HandleGetNodes(w, r)
		case http.MethodPost:
			RequireAdmin(configHandlers.config, configHandlers.HandleAddNode)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Test node configuration endpoint (for new nodes)
	r.mux.HandleFunc("/api/config/nodes/test-config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			configHandlers.HandleTestNodeConfig(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Test connection endpoint
	r.mux.HandleFunc("/api/config/nodes/test-connection", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			configHandlers.HandleTestConnection(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	r.mux.HandleFunc("/api/config/nodes/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			RequireAdmin(configHandlers.config, configHandlers.HandleUpdateNode)(w, r)
		case http.MethodDelete:
			RequireAdmin(configHandlers.config, configHandlers.HandleDeleteNode)(w, r)
		case http.MethodPost:
			// Handle test endpoint
			if strings.HasSuffix(r.URL.Path, "/test") {
				configHandlers.HandleTestNode(w, r)
			} else {
				http.Error(w, "Not found", http.StatusNotFound)
			}
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// System settings routes
	r.mux.HandleFunc("/api/config/system", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			configHandlers.HandleGetSystemSettings(w, r)
		case http.MethodPut:
			// DEPRECATED - use /api/system/settings/update instead
			RequireAdmin(configHandlers.config, configHandlers.HandleUpdateSystemSettingsOLD)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Mock mode toggle routes
	r.mux.HandleFunc("/api/system/mock-mode", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			configHandlers.HandleGetMockMode(w, req)
		case http.MethodPost, http.MethodPut:
			RequireAdmin(configHandlers.config, configHandlers.HandleUpdateMockMode)(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Registration token routes removed - feature deprecated

	// Security routes
	r.mux.HandleFunc("/api/security/change-password", r.handleChangePassword)
	r.mux.HandleFunc("/api/logout", r.handleLogout)
	r.mux.HandleFunc("/api/login", r.handleLogin)
	r.mux.HandleFunc("/api/security/reset-lockout", r.handleResetLockout)
	r.mux.HandleFunc("/api/security/oidc", RequireAdmin(r.config, r.handleOIDCConfig))
	r.mux.HandleFunc("/api/oidc/login", r.handleOIDCLogin)
	r.mux.HandleFunc(config.DefaultOIDCCallbackPath, r.handleOIDCCallback)
	r.mux.HandleFunc("/api/security/status", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")

			// Check if auth is globally disabled
			if r.config.DisableAuth {
				// Even with auth disabled, check for OIDC sessions
				oidcCfg := r.ensureOIDCConfig()
				oidcUsername := ""
				if oidcCfg != nil && oidcCfg.Enabled {
					if cookie, err := req.Cookie("pulse_session"); err == nil && cookie.Value != "" {
						if ValidateSession(cookie.Value) {
							oidcUsername = GetSessionUsername(cookie.Value)
						}
					}
				}

				// Even with auth disabled, report API token status for API access
				var apiTokenHint string
				if r.config.APIToken != "" && len(r.config.APIToken) >= 8 {
					apiTokenHint = r.config.APIToken[:4] + "..." + r.config.APIToken[len(r.config.APIToken)-4:]
				}

				response := map[string]interface{}{
					"configured":         false,
					"disabled":           true,
					"message":            "Authentication is disabled via DISABLE_AUTH environment variable",
					"apiTokenConfigured": r.config.APIToken != "",
					"apiTokenHint":       apiTokenHint,
					"hasAuthentication":  false,
				}

				// Add OIDC info if available
				if oidcCfg != nil {
					response["oidcEnabled"] = oidcCfg.Enabled
					response["oidcIssuer"] = oidcCfg.IssuerURL
					response["oidcClientId"] = oidcCfg.ClientID
					response["oidcUsername"] = oidcUsername
					response["oidcLogoutURL"] = oidcCfg.LogoutURL
					if len(oidcCfg.EnvOverrides) > 0 {
						response["oidcEnvOverrides"] = oidcCfg.EnvOverrides
					}
				}

				json.NewEncoder(w).Encode(response)
				return
			}

			// Check for basic auth configuration
			// Check both environment variables and loaded config
			oidcCfg := r.ensureOIDCConfig()
			hasAuthentication := os.Getenv("PULSE_AUTH_USER") != "" ||
				os.Getenv("REQUIRE_AUTH") == "true" ||
				r.config.AuthUser != "" ||
				r.config.AuthPass != "" ||
				(oidcCfg != nil && oidcCfg.Enabled) ||
				r.config.APIToken != "" ||
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
			clientIP := utils.GetClientIP(
				req.RemoteAddr,
				req.Header.Get("X-Forwarded-For"),
				req.Header.Get("X-Real-IP"),
			)
			isPrivateNetwork := utils.IsPrivateIP(clientIP)

			// Get trusted networks from environment
			trustedNetworks := []string{}
			if nets := os.Getenv("PULSE_TRUSTED_NETWORKS"); nets != "" {
				trustedNetworks = strings.Split(nets, ",")
			}
			isTrustedNetwork := utils.IsTrustedNetwork(clientIP, trustedNetworks)

			// Create token hint if token exists
			var apiTokenHint string
			if r.config.APIToken != "" && len(r.config.APIToken) >= 8 {
				apiTokenHint = r.config.APIToken[:4] + "..." + r.config.APIToken[len(r.config.APIToken)-4:]
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

			requiresAuth := r.config.APIToken != "" ||
				(r.config.AuthUser != "" && r.config.AuthPass != "") ||
				(r.config.OIDC != nil && r.config.OIDC.Enabled) ||
				r.config.ProxyAuthSecret != ""

			status := map[string]interface{}{
				"apiTokenConfigured":          r.config.APIToken != "",
				"apiTokenHint":                apiTokenHint,
				"requiresAuth":                requiresAuth,
				"exportProtected":             r.config.APIToken != "" || os.Getenv("ALLOW_UNPROTECTED_EXPORT") != "true",
				"unprotectedExportAllowed":    os.Getenv("ALLOW_UNPROTECTED_EXPORT") == "true",
				"hasAuthentication":           hasAuthentication,
				"configuredButPendingRestart": configuredButPendingRestart,
				"hasAuditLogging":             hasAuditLogging,
				"credentialsEncrypted":        credentialsEncrypted,
				"hasHTTPS":                    req.TLS != nil,
				"clientIP":                    clientIP,
				"isPrivateNetwork":            isPrivateNetwork,
				"isTrustedNetwork":            isTrustedNetwork,
				"publicAccess":                !isPrivateNetwork,
				"hasProxyAuth":                hasProxyAuth,
				"proxyAuthLogoutURL":          r.config.ProxyAuthLogoutURL,
				"proxyAuthUsername":           proxyAuthUsername,
				"proxyAuthIsAdmin":            proxyAuthIsAdmin,
				"authUsername":                r.config.AuthUser,
				"authLastModified":            authLastModified,
				"oidcUsername":                oidcUsername,
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

			json.NewEncoder(w).Encode(status)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Quick security setup route - using fixed version
	r.mux.HandleFunc("/api/security/quick-setup", handleQuickSecuritySetupFixed(r))

	// API token regeneration endpoint
	r.mux.HandleFunc("/api/security/regenerate-token", r.HandleRegenerateAPIToken)

	// Apply security restart endpoint
	r.mux.HandleFunc("/api/security/apply-restart", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
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

	// Initialize recovery token store
	InitRecoveryTokenStore(r.config.DataPath)

	// Recovery endpoint - requires localhost access OR valid recovery token
	r.mux.HandleFunc("/api/security/recovery", func(w http.ResponseWriter, req *http.Request) {
		// Get client IP
		ip := strings.Split(req.RemoteAddr, ":")[0]
		isLocalhost := ip == "127.0.0.1" || ip == "::1" || ip == "localhost"

		// Check for recovery token in header
		recoveryToken := req.Header.Get("X-Recovery-Token")
		hasValidToken := false
		if recoveryToken != "" {
			hasValidToken = GetRecoveryTokenStore().ValidateRecoveryTokenConstantTime(recoveryToken, ip)
		}

		// Only allow from localhost OR with valid recovery token
		if !isLocalhost && !hasValidToken {
			log.Warn().
				Str("ip", ip).
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
				if !isLocalhost {
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
						Str("ip", ip).
						Int("duration_minutes", duration).
						Msg("Recovery token generated")
				}

			case "disable_auth":
				// Temporarily disable auth by creating recovery file
				recoveryFile := filepath.Join(r.config.DataPath, ".auth_recovery")
				content := fmt.Sprintf("Recovery mode enabled at %s\nAuth temporarily disabled for local access\nEnabled by: %s\n", time.Now().Format(time.RFC3339), ip)
				if err := os.WriteFile(recoveryFile, []byte(content), 0600); err != nil {
					response["success"] = false
					response["message"] = fmt.Sprintf("Failed to enable recovery mode: %v", err)
				} else {
					response["success"] = true
					response["message"] = "Recovery mode enabled. Auth disabled for localhost. Delete .auth_recovery file to re-enable."
					log.Warn().
						Str("ip", ip).
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

	// Config export/import routes (requires authentication)
	r.mux.HandleFunc("/api/config/export", r.exportLimiter.Middleware(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			// Check proxy auth first
			hasValidProxyAuth := false
			proxyAuthIsAdmin := false
			if r.config.ProxyAuthSecret != "" {
				if valid, _, isAdmin := CheckProxyAuth(r.config, req); valid {
					hasValidProxyAuth = true
					proxyAuthIsAdmin = isAdmin
				}
			}

			// Check authentication - accept proxy auth, session auth or API token
			hasValidSession := false
			if cookie, err := req.Cookie("pulse_session"); err == nil && cookie.Value != "" {
				hasValidSession = ValidateSession(cookie.Value)
			}

			hasValidAPIToken := false
			if r.config.APIToken != "" {
				authHeader := req.Header.Get("X-API-Token")
				// Check if stored token is hashed or plain text
				if auth.IsAPITokenHashed(r.config.APIToken) {
					// Compare against hash
					hasValidAPIToken = auth.CompareAPIToken(authHeader, r.config.APIToken)
				} else {
					// Plain text comparison (legacy)
					hasValidAPIToken = (authHeader == r.config.APIToken)
				}
			}

			// Check if any valid auth method is present
			hasValidAuth := hasValidProxyAuth || hasValidSession || hasValidAPIToken

			// Determine if auth is required
			authRequired := r.config.AuthUser != "" && r.config.AuthPass != "" ||
				r.config.APIToken != "" ||
				r.config.ProxyAuthSecret != ""

			// Check admin privileges for proxy auth users
			if hasValidProxyAuth && !proxyAuthIsAdmin {
				log.Warn().
					Str("ip", req.RemoteAddr).
					Str("path", req.URL.Path).
					Msg("Non-admin proxy auth user attempted export/import")
				http.Error(w, "Admin privileges required for export/import", http.StatusForbidden)
				return
			}

			if authRequired && !hasValidAuth {
				log.Warn().
					Str("ip", req.RemoteAddr).
					Str("path", req.URL.Path).
					Bool("proxyAuth", hasValidProxyAuth).
					Bool("session", hasValidSession).
					Bool("apiToken", hasValidAPIToken).
					Msg("Unauthorized export attempt")
				http.Error(w, "Unauthorized - please log in or provide API token", http.StatusUnauthorized)
				return
			} else if !authRequired {
				// No auth configured - check if this is a homelab/private network
				clientIP := utils.GetClientIP(req.RemoteAddr,
					req.Header.Get("X-Forwarded-For"),
					req.Header.Get("X-Real-IP"))

				isPrivate := utils.IsPrivateIP(clientIP)
				allowUnprotected := os.Getenv("ALLOW_UNPROTECTED_EXPORT") == "true"

				if !isPrivate && !allowUnprotected {
					// Public network access without auth - definitely block
					log.Warn().
						Str("ip", req.RemoteAddr).
						Bool("private_network", isPrivate).
						Msg("Export blocked - public network requires authentication")
					http.Error(w, "Export requires authentication on public networks", http.StatusForbidden)
					return
				} else if isPrivate && !allowUnprotected {
					// Private network but ALLOW_UNPROTECTED_EXPORT not set - show helpful message
					log.Info().
						Str("ip", req.RemoteAddr).
						Msg("Export allowed - private network with no auth")
					// Continue - allow export on private networks for homelab users
				}
			}

			// Log successful export attempt
			log.Info().
				Str("ip", req.RemoteAddr).
				Bool("proxy_auth", hasValidProxyAuth).
				Bool("session_auth", hasValidSession).
				Bool("api_token_auth", hasValidAPIToken).
				Msg("Configuration export initiated")

			configHandlers.HandleExportConfig(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	r.mux.HandleFunc("/api/config/import", r.exportLimiter.Middleware(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			// Check proxy auth first
			hasValidProxyAuth := false
			proxyAuthIsAdmin := false
			if r.config.ProxyAuthSecret != "" {
				if valid, _, isAdmin := CheckProxyAuth(r.config, req); valid {
					hasValidProxyAuth = true
					proxyAuthIsAdmin = isAdmin
				}
			}

			// Check authentication - accept proxy auth, session auth or API token
			hasValidSession := false
			if cookie, err := req.Cookie("pulse_session"); err == nil && cookie.Value != "" {
				hasValidSession = ValidateSession(cookie.Value)
			}

			hasValidAPIToken := false
			if r.config.APIToken != "" {
				authHeader := req.Header.Get("X-API-Token")
				// Check if stored token is hashed or plain text
				if auth.IsAPITokenHashed(r.config.APIToken) {
					// Compare against hash
					hasValidAPIToken = auth.CompareAPIToken(authHeader, r.config.APIToken)
				} else {
					// Plain text comparison (legacy)
					hasValidAPIToken = (authHeader == r.config.APIToken)
				}
			}

			// Check if any valid auth method is present
			hasValidAuth := hasValidProxyAuth || hasValidSession || hasValidAPIToken

			// Determine if auth is required
			authRequired := r.config.AuthUser != "" && r.config.AuthPass != "" ||
				r.config.APIToken != "" ||
				r.config.ProxyAuthSecret != ""

			// Check admin privileges for proxy auth users
			if hasValidProxyAuth && !proxyAuthIsAdmin {
				log.Warn().
					Str("ip", req.RemoteAddr).
					Str("path", req.URL.Path).
					Msg("Non-admin proxy auth user attempted export/import")
				http.Error(w, "Admin privileges required for export/import", http.StatusForbidden)
				return
			}

			if authRequired && !hasValidAuth {
				log.Warn().
					Str("ip", req.RemoteAddr).
					Str("path", req.URL.Path).
					Bool("proxyAuth", hasValidProxyAuth).
					Bool("session", hasValidSession).
					Bool("apiToken", hasValidAPIToken).
					Msg("Unauthorized import attempt")
				http.Error(w, "Unauthorized - please log in or provide API token", http.StatusUnauthorized)
				return
			} else if !authRequired {
				// No auth configured - check if this is a homelab/private network
				clientIP := utils.GetClientIP(req.RemoteAddr,
					req.Header.Get("X-Forwarded-For"),
					req.Header.Get("X-Real-IP"))

				isPrivate := utils.IsPrivateIP(clientIP)
				allowUnprotected := os.Getenv("ALLOW_UNPROTECTED_EXPORT") == "true"

				if !isPrivate && !allowUnprotected {
					// Public network access without auth - definitely block
					log.Warn().
						Str("ip", req.RemoteAddr).
						Bool("private_network", isPrivate).
						Msg("Import blocked - public network requires authentication")
					http.Error(w, "Import requires authentication on public networks", http.StatusForbidden)
					return
				} else if isPrivate && !allowUnprotected {
					// Private network but ALLOW_UNPROTECTED_EXPORT not set - show helpful message
					log.Info().
						Str("ip", req.RemoteAddr).
						Msg("Import allowed - private network with no auth")
					// Continue - allow import on private networks for homelab users
				}
			}

			// Log successful import attempt
			log.Info().
				Str("ip", req.RemoteAddr).
				Bool("session_auth", hasValidSession).
				Bool("api_token_auth", hasValidAPIToken).
				Msg("Configuration import initiated")

			configHandlers.HandleImportConfig(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Discovery route

	// Setup script route
	r.mux.HandleFunc("/api/setup-script", configHandlers.HandleSetupScript)

	// Generate setup script URL with temporary token (for authenticated users)
	r.mux.HandleFunc("/api/setup-script-url", configHandlers.HandleSetupScriptURL)

	// Auto-register route for setup scripts
	r.mux.HandleFunc("/api/auto-register", configHandlers.HandleAutoRegister)
	// Discovery endpoint
	r.mux.HandleFunc("/api/discover", RequireAuth(r.config, configHandlers.HandleDiscoverServers))

	// Test endpoint for WebSocket notifications
	r.mux.HandleFunc("/api/test-notification", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Send a test auto-registration notification
		r.wsHub.BroadcastMessage(websocket.Message{
			Type: "node_auto_registered",
			Data: map[string]interface{}{
				"type":     "pve",
				"host":     "test-node.example.com",
				"name":     "Test Node",
				"tokenId":  "test-token",
				"hasToken": true,
			},
			Timestamp: time.Now().Format(time.RFC3339),
		})

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "notification sent"})
	})

	// Alert routes
	r.mux.HandleFunc("/api/alerts/", alertHandlers.HandleAlerts)

	// Notification routes
	r.mux.HandleFunc("/api/notifications/", notificationHandlers.HandleNotifications)

	// Settings routes
	r.mux.HandleFunc("/api/settings", getSettings)
	r.mux.HandleFunc("/api/settings/update", updateSettings)

	// System settings and API token management
	systemSettingsHandler := NewSystemSettingsHandler(r.config, r.persistence, r.wsHub, r.monitor)
	r.mux.HandleFunc("/api/system/settings", systemSettingsHandler.HandleGetSystemSettings)
	r.mux.HandleFunc("/api/system/settings/update", systemSettingsHandler.HandleUpdateSystemSettings)
	// Old API token endpoints removed - now using /api/security/regenerate-token

	// WebSocket endpoint
	r.mux.HandleFunc("/ws", r.handleWebSocket)

	// Socket.io compatibility endpoints
	r.mux.HandleFunc("/socket.io/", r.handleSocketIO)

	// Simple stats page
	r.mux.HandleFunc("/simple-stats", r.handleSimpleStats)

	// Note: Frontend handler is handled manually in ServeHTTP to prevent redirect issues
	// See issue #334 - ServeMux redirects empty path to "./" which breaks reverse proxies

}

// ServeHTTP implements http.Handler
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Prevent path traversal attacks by cleaning the path
	cleanPath := filepath.Clean(req.URL.Path)
	// Reject requests with path traversal attempts
	if strings.Contains(req.URL.Path, "..") || cleanPath != req.URL.Path {
		// Return 401 for API paths to match expected test behavior
		if strings.HasPrefix(req.URL.Path, "/api/") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		} else {
			http.Error(w, "Invalid path", http.StatusBadRequest)
		}
		log.Warn().
			Str("ip", req.RemoteAddr).
			Str("path", req.URL.Path).
			Str("clean_path", cleanPath).
			Msg("Path traversal attempt blocked")
		return
	}

	// Load system settings to get embedding configuration
	var allowEmbedding bool
	var allowedEmbedOrigins string
	if systemSettings, err := r.persistence.LoadSystemSettings(); err == nil && systemSettings != nil {
		allowEmbedding = systemSettings.AllowEmbedding
		allowedEmbedOrigins = systemSettings.AllowedEmbedOrigins
	}

	// Apply security headers with embedding configuration
	SecurityHeadersWithConfig(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Add CORS headers if configured
		if r.config.AllowedOrigins != "" {
			w.Header().Set("Access-Control-Allow-Origin", r.config.AllowedOrigins)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Token, X-CSRF-Token")
		}

		// Handle preflight requests
		if req.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Check if we need authentication
		needsAuth := true

		// Check if auth is globally disabled
		// BUT still check for API tokens if provided (for API access when auth is disabled)
		if r.config.DisableAuth {
			// Check if an API token was provided
			providedToken := req.Header.Get("X-API-Token")
			if providedToken == "" {
				providedToken = req.URL.Query().Get("token")
			}

			// If a valid API token is provided, allow access even with DisableAuth
			if providedToken != "" && r.config.APIToken != "" {
				if auth.CompareAPIToken(providedToken, r.config.APIToken) {
					// Valid API token provided, allow access
					needsAuth = false
					w.Header().Set("X-Auth-Method", "api-token")
				} else {
					// Invalid API token - reject even with DisableAuth
					http.Error(w, "Invalid API token", http.StatusUnauthorized)
					return
				}
			} else {
				// No API token provided with DisableAuth - allow open access
				needsAuth = false
				w.Header().Set("X-Auth-Disabled", "true")
			}
		}

		// Recovery mechanism: Check if recovery mode is enabled
		recoveryFile := filepath.Join(r.config.DataPath, ".auth_recovery")
		if _, err := os.Stat(recoveryFile); err == nil {
			// Recovery mode is enabled - allow local access only
			ip := strings.Split(req.RemoteAddr, ":")[0]
			log.Debug().
				Str("recovery_file", recoveryFile).
				Str("remote_ip", ip).
				Str("path", req.URL.Path).
				Bool("file_exists", err == nil).
				Msg("Checking auth recovery mode")
			if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
				log.Warn().
					Str("recovery_file", recoveryFile).
					Msg("AUTH RECOVERY MODE: Allowing local access without authentication")
				// Allow access but add a warning header
				w.Header().Set("X-Auth-Recovery", "true")
				// Recovery mode bypasses auth for localhost
				needsAuth = false
			}
		}

		if needsAuth {
			// Normal authentication check
			// Skip auth for certain public endpoints and static assets
			publicPaths := []string{
				"/api/health",
				"/api/security/status",
				"/api/version",
				"/api/login", // Add login endpoint as public
				"/api/oidc/login",
				config.DefaultOIDCCallbackPath,
			}

			// Also allow static assets without auth (JS, CSS, etc)
			// These MUST be accessible for the login page to work
			isStaticAsset := strings.HasPrefix(req.URL.Path, "/assets/") ||
				strings.HasPrefix(req.URL.Path, "/@vite/") ||
				strings.HasPrefix(req.URL.Path, "/@solid-refresh") ||
				strings.HasPrefix(req.URL.Path, "/src/") ||
				strings.HasPrefix(req.URL.Path, "/node_modules/") ||
				req.URL.Path == "/" ||
				req.URL.Path == "/index.html" ||
				req.URL.Path == "/favicon.ico" ||
				req.URL.Path == "/logo.svg" ||
				strings.HasSuffix(req.URL.Path, ".js") ||
				strings.HasSuffix(req.URL.Path, ".css") ||
				strings.HasSuffix(req.URL.Path, ".map") ||
				strings.HasSuffix(req.URL.Path, ".ts") ||
				strings.HasSuffix(req.URL.Path, ".tsx") ||
				strings.HasSuffix(req.URL.Path, ".mjs") ||
				strings.HasSuffix(req.URL.Path, ".jsx")

			isPublic := isStaticAsset
			for _, path := range publicPaths {
				if req.URL.Path == path {
					isPublic = true
					break
				}
			}

			// Special case: setup-script should be public (uses setup codes for auth)
			if req.URL.Path == "/api/setup-script" {
				// The script itself prompts for a setup code
				isPublic = true
			}

			// Auto-register endpoint needs to be public (validates tokens internally)
			// BUT the tokens must be generated by authenticated users via setup-script-url
			if req.URL.Path == "/api/auto-register" {
				isPublic = true
			}

			// Special case: quick-setup should be accessible to check if already configured
			// The handler itself will verify if setup should be skipped
			if req.URL.Path == "/api/security/quick-setup" && req.Method == http.MethodPost {
				isPublic = true
			}
			// Check auth for protected routes (only if auth is needed)
			if needsAuth && !isPublic && !CheckAuth(r.config, w, req) {
				// Never send WWW-Authenticate - use custom login page
				// For API requests, return JSON
				if strings.HasPrefix(req.URL.Path, "/api/") || strings.Contains(req.Header.Get("Accept"), "application/json") {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error":"Authentication required"}`))
				} else {
					http.Error(w, "Authentication required", http.StatusUnauthorized)
				}
				log.Warn().
					Str("ip", req.RemoteAddr).
					Str("path", req.URL.Path).
					Msg("Unauthorized access attempt")
				return
			}
		}
		// Check CSRF for state-changing requests
		// CSRF is only needed when using session-based auth
		// Only skip CSRF for initial setup when no auth is configured
		skipCSRF := false
		if (req.URL.Path == "/api/security/quick-setup" || req.URL.Path == "/api/security/apply-restart") &&
			r.config.AuthUser == "" && r.config.AuthPass == "" {
			// Only skip CSRF for initial setup and restart when no auth exists
			skipCSRF = true
		}
		// Skip CSRF for setup-script-url endpoint (generates temporary tokens, not a state change)
		if req.URL.Path == "/api/setup-script-url" {
			skipCSRF = true
		}
		if strings.HasPrefix(req.URL.Path, "/api/") && !skipCSRF && !CheckCSRF(w, req) {
			http.Error(w, "CSRF token validation failed", http.StatusForbidden)
			LogAuditEvent("csrf_failure", "", GetClientIP(req), req.URL.Path, false, "Invalid CSRF token")
			return
		}

		// Rate limiting is now handled by UniversalRateLimitMiddleware
		// No need for duplicate rate limiting logic here

		// Log request
		start := time.Now()

		// Fix for issue #334: Custom routing to prevent ServeMux's "./" redirect
		// When accessing without trailing slash, ServeMux redirects to "./" which is wrong
		// We handle routing manually to avoid this issue

		// Check if this is an API or WebSocket route
		if strings.HasPrefix(req.URL.Path, "/api/") ||
			strings.HasPrefix(req.URL.Path, "/ws") ||
			strings.HasPrefix(req.URL.Path, "/socket.io/") ||
			req.URL.Path == "/simple-stats" {
			// Use the mux for API and special routes
			r.mux.ServeHTTP(w, req)
		} else {
			// Serve frontend for all other paths (including root)
			handler := serveFrontendHandler()
			handler(w, req)
		}

		log.Debug().
			Str("method", req.Method).
			Str("path", req.URL.Path).
			Dur("duration", time.Since(start)).
			Msg("Request handled")
	}), allowEmbedding, allowedEmbedOrigins).ServeHTTP(w, req)
}

// handleHealth handles health check requests
func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Unix(),
		Uptime:    time.Since(r.monitor.GetStartTime()).Seconds(),
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write health response")
	}
}

// handleChangePassword handles password change requests
func (r *Router) handleChangePassword(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only POST method is allowed", nil)
		return
	}

	// Check if using proxy auth and if so, verify admin status
	if r.config.ProxyAuthSecret != "" {
		if valid, username, isAdmin := CheckProxyAuth(r.config, req); valid {
			if !isAdmin {
				// User is authenticated but not an admin
				log.Warn().
					Str("ip", req.RemoteAddr).
					Str("path", req.URL.Path).
					Str("method", req.Method).
					Str("username", username).
					Msg("Non-admin user attempted to change password")

				// Return forbidden error
				writeErrorResponse(w, http.StatusForbidden, "forbidden",
					"Admin privileges required", nil)
				return
			}
		}
	}

	// Parse request
	var changeReq struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}

	if err := json.NewDecoder(req.Body).Decode(&changeReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request",
			"Invalid request body", nil)
		return
	}

	// Validate new password complexity
	if err := auth.ValidatePasswordComplexity(changeReq.NewPassword); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_password",
			err.Error(), nil)
		return
	}

	// Verify current password matches
	// When behind a proxy with Basic Auth, the proxy may overwrite the Authorization header
	// So we verify the current password from the JSON body instead

	// First, validate that currentPassword was provided
	if changeReq.CurrentPassword == "" {
		writeErrorResponse(w, http.StatusUnauthorized, "unauthorized",
			"Current password required", nil)
		return
	}

	// Check if we should use Basic Auth header or JSON body for verification
	// If there's an Authorization header AND it's not from a proxy, use it
	authHeader := req.Header.Get("Authorization")
	useAuthHeader := false
	username := r.config.AuthUser // Default to configured username

	if authHeader != "" {
		const basicPrefix = "Basic "
		if strings.HasPrefix(authHeader, basicPrefix) {
			decoded, err := base64.StdEncoding.DecodeString(authHeader[len(basicPrefix):])
			if err == nil {
				parts := strings.SplitN(string(decoded), ":", 2)
				if len(parts) == 2 {
					// Check if this looks like Pulse credentials (matching username)
					if parts[0] == r.config.AuthUser {
						// This is likely from Pulse's own auth, not a proxy
						username = parts[0]
						useAuthHeader = true
						// Verify the password from the header matches
						if !auth.CheckPasswordHash(parts[1], r.config.AuthPass) {
							log.Warn().
								Str("ip", req.RemoteAddr).
								Str("username", username).
								Msg("Failed password change attempt - incorrect current password in auth header")
							writeErrorResponse(w, http.StatusUnauthorized, "unauthorized",
								"Current password is incorrect", nil)
							return
						}
					}
					// If username doesn't match, this is likely proxy auth - ignore it
				}
			}
		}
	}

	// If we didn't use the auth header, or need to double-check, verify from JSON body
	if !useAuthHeader || changeReq.CurrentPassword != "" {
		// Verify current password from JSON body
		if !auth.CheckPasswordHash(changeReq.CurrentPassword, r.config.AuthPass) {
			log.Warn().
				Str("ip", req.RemoteAddr).
				Str("username", username).
				Msg("Failed password change attempt - incorrect current password")
			writeErrorResponse(w, http.StatusUnauthorized, "unauthorized",
				"Current password is incorrect", nil)
			return
		}
	}

	// Hash the new password before storing
	hashedPassword, err := auth.HashPassword(changeReq.NewPassword)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash new password")
		writeErrorResponse(w, http.StatusInternalServerError, "hash_error",
			"Failed to process new password", nil)
		return
	}

	// Check if we're running in Docker
	isDocker := os.Getenv("PULSE_DOCKER") == "true"

	if isDocker {
		// For Docker, update the .env file in the data directory
		envPath := filepath.Join(r.config.ConfigPath, ".env")

		// Read existing .env file to preserve other settings
		envContent := ""
		existingContent, err := os.ReadFile(envPath)
		if err == nil {
			// Parse existing content and update password
			scanner := bufio.NewScanner(strings.NewReader(string(existingContent)))
			for scanner.Scan() {
				line := scanner.Text()
				// Skip empty lines and comments
				if line == "" || strings.HasPrefix(line, "#") {
					envContent += line + "\n"
					continue
				}
				// Update password line, keep others
				if strings.HasPrefix(line, "PULSE_AUTH_PASS=") {
					envContent += fmt.Sprintf("PULSE_AUTH_PASS='%s'\n", hashedPassword)
				} else {
					envContent += line + "\n"
				}
			}
		} else {
			// Create new .env file if it doesn't exist
			envContent = fmt.Sprintf(`# Auto-generated by Pulse password change
# Generated on %s
PULSE_AUTH_USER='%s'
PULSE_AUTH_PASS='%s'
`, time.Now().Format(time.RFC3339), r.config.AuthUser, hashedPassword)

			// Include API token if configured
			if r.config.APIToken != "" {
				envContent += fmt.Sprintf("API_TOKEN='%s'\n", r.config.APIToken)
			}
		}

		// Write the updated .env file
		if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
			log.Error().Err(err).Str("path", envPath).Msg("Failed to write .env file")
			writeErrorResponse(w, http.StatusInternalServerError, "config_error",
				"Failed to save new password", nil)
			return
		}

		// Update the running config
		r.config.AuthPass = hashedPassword

		log.Info().Msg("Password changed successfully in Docker environment")

		// Invalidate all sessions
		InvalidateUserSessions(r.config.AuthUser)

		// Audit log
		LogAuditEvent("password_change", r.config.AuthUser, GetClientIP(req), req.URL.Path, true, "Password changed (Docker)")

		// Return success with Docker-specific message
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Password changed successfully. Please restart your Docker container to apply changes.",
		})

	} else {
		// For non-Docker (systemd/manual), save to .env file
		envPath := filepath.Join(r.config.ConfigPath, ".env")
		if r.config.ConfigPath == "" {
			envPath = "/etc/pulse/.env"
		}

		// Read existing .env file to preserve other settings
		envContent := ""
		existingContent, err := os.ReadFile(envPath)
		if err == nil {
			// Parse and update existing content
			scanner := bufio.NewScanner(strings.NewReader(string(existingContent)))
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" || strings.HasPrefix(line, "#") {
					envContent += line + "\n"
					continue
				}
				// Update password line, keep others
				if strings.HasPrefix(line, "PULSE_AUTH_PASS=") {
					envContent += fmt.Sprintf("PULSE_AUTH_PASS='%s'\n", hashedPassword)
				} else {
					envContent += line + "\n"
				}
			}
		} else {
			// Create new .env if doesn't exist
			envContent = fmt.Sprintf(`# Auto-generated by Pulse password change
# Generated on %s
PULSE_AUTH_USER='%s'
PULSE_AUTH_PASS='%s'
`, time.Now().Format(time.RFC3339), r.config.AuthUser, hashedPassword)

			if r.config.APIToken != "" {
				envContent += fmt.Sprintf("API_TOKEN='%s'\n", r.config.APIToken)
			}
		}

		// Try to write the .env file
		if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
			log.Error().Err(err).Str("path", envPath).Msg("Failed to write .env file")
			writeErrorResponse(w, http.StatusInternalServerError, "config_error",
				"Failed to save new password. You may need to update the password manually.", nil)
			return
		}

		// Update the running config
		r.config.AuthPass = hashedPassword

		log.Info().Msg("Password changed successfully")

		// Invalidate all sessions
		InvalidateUserSessions(r.config.AuthUser)

		// Audit log
		LogAuditEvent("password_change", r.config.AuthUser, GetClientIP(req), req.URL.Path, true, "Password changed")

		// Detect service name for restart instructions
		serviceName := detectServiceName()

		// Return success with manual restart instructions
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":         true,
			"message":         fmt.Sprintf("Password changed. Restart the service to apply: sudo systemctl restart %s", serviceName),
			"requiresRestart": true,
			"serviceName":     serviceName,
		})
	}
}

// handleLogout handles logout requests
func (r *Router) handleLogout(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only POST method is allowed", nil)
		return
	}

	// Get session token from cookie
	var sessionToken string
	if cookie, err := req.Cookie("pulse_session"); err == nil {
		sessionToken = cookie.Value
	}

	// Delete the session if it exists
	if sessionToken != "" {
		GetSessionStore().DeleteSession(sessionToken)

		// Also delete CSRF token if exists
		GetCSRFStore().DeleteCSRFToken(sessionToken)
	}

	// Get appropriate cookie settings based on proxy detection (consistent with login)
	isSecure, sameSitePolicy := getCookieSettings(req)

	// Clear the session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "pulse_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: sameSitePolicy,
	})

	// Audit log logout (use admin as username since we have single user for now)
	LogAuditEvent("logout", "admin", GetClientIP(req), req.URL.Path, true, "User logged out")

	log.Info().
		Str("user", "admin").
		Str("ip", GetClientIP(req)).
		Msg("User logged out")

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Successfully logged out",
	})
}

func (r *Router) establishSession(w http.ResponseWriter, req *http.Request, username string) error {
	token := generateSessionToken()
	if token == "" {
		return fmt.Errorf("failed to generate session token")
	}

	userAgent := req.Header.Get("User-Agent")
	clientIP := GetClientIP(req)
	GetSessionStore().CreateSession(token, 24*time.Hour, userAgent, clientIP)

	if username != "" {
		TrackUserSession(username, token)
	}

	csrfToken := generateCSRFToken(token)
	isSecure, sameSitePolicy := getCookieSettings(req)

	http.SetCookie(w, &http.Cookie{
		Name:     "pulse_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: sameSitePolicy,
		MaxAge:   86400,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "pulse_csrf",
		Value:    csrfToken,
		Path:     "/",
		Secure:   isSecure,
		SameSite: sameSitePolicy,
		MaxAge:   86400,
	})

	return nil
}

// handleLogin handles login requests and provides detailed feedback about lockouts
func (r *Router) handleLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only POST method is allowed", nil)
		return
	}

	// Parse request
	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(req.Body).Decode(&loginReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request",
			"Invalid request body", nil)
		return
	}

	clientIP := GetClientIP(req)

	// Check if account is locked out before attempting login
	_, userLockedUntil, userLocked := GetLockoutInfo(loginReq.Username)
	_, ipLockedUntil, ipLocked := GetLockoutInfo(clientIP)

	if userLocked || ipLocked {
		lockedUntil := userLockedUntil
		if ipLocked && ipLockedUntil.After(lockedUntil) {
			lockedUntil = ipLockedUntil
		}

		remainingMinutes := int(time.Until(lockedUntil).Minutes())
		if remainingMinutes < 1 {
			remainingMinutes = 1
		}

		LogAuditEvent("login", loginReq.Username, clientIP, req.URL.Path, false, "Account locked")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":            "account_locked",
			"message":          fmt.Sprintf("Too many failed attempts. Account is locked for %d more minutes.", remainingMinutes),
			"lockedUntil":      lockedUntil.Format(time.RFC3339),
			"remainingMinutes": remainingMinutes,
		})
		return
	}

	// Check rate limiting
	if !authLimiter.Allow(clientIP) {
		LogAuditEvent("login", loginReq.Username, clientIP, req.URL.Path, false, "Rate limited")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "rate_limit",
			"message": "Too many requests. Please wait before trying again.",
		})
		return
	}

	// Verify credentials
	if loginReq.Username == r.config.AuthUser && auth.CheckPasswordHash(loginReq.Password, r.config.AuthPass) {
		// Clear failed login attempts
		ClearFailedLogins(loginReq.Username)
		ClearFailedLogins(clientIP)

		// Create session
		token := generateSessionToken()
		if token == "" {
			writeErrorResponse(w, http.StatusInternalServerError, "session_error",
				"Failed to create session", nil)
			return
		}

		// Store session persistently
		userAgent := req.Header.Get("User-Agent")
		GetSessionStore().CreateSession(token, 24*time.Hour, userAgent, clientIP)

		// Track session for user
		TrackUserSession(loginReq.Username, token)

		// Generate CSRF token
		csrfToken := generateCSRFToken(token)

		// Get appropriate cookie settings based on proxy detection
		isSecure, sameSitePolicy := getCookieSettings(req)

		// Set session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "pulse_session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   isSecure,
			SameSite: sameSitePolicy,
			MaxAge:   86400, // 24 hours
		})

		// Set CSRF cookie (not HttpOnly so JS can read it)
		http.SetCookie(w, &http.Cookie{
			Name:     "pulse_csrf",
			Value:    csrfToken,
			Path:     "/",
			Secure:   isSecure,
			SameSite: sameSitePolicy,
			MaxAge:   86400, // 24 hours
		})

		// Audit log successful login
		LogAuditEvent("login", loginReq.Username, clientIP, req.URL.Path, true, "Successful login")

		// Return success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Successfully logged in",
		})
	} else {
		// Failed login
		RecordFailedLogin(loginReq.Username)
		RecordFailedLogin(clientIP)
		LogAuditEvent("login", loginReq.Username, clientIP, req.URL.Path, false, "Invalid credentials")

		// Get updated attempt counts
		newUserAttempts, _, _ := GetLockoutInfo(loginReq.Username)
		newIPAttempts, _, _ := GetLockoutInfo(clientIP)

		// Use the higher count for warning
		attempts := newUserAttempts
		if newIPAttempts > attempts {
			attempts = newIPAttempts
		}

		// Prepare response with attempt information
		remaining := maxFailedAttempts - attempts

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)

		if remaining > 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":       "invalid_credentials",
				"message":     fmt.Sprintf("Invalid username or password. You have %d attempts remaining.", remaining),
				"attempts":    attempts,
				"remaining":   remaining,
				"maxAttempts": maxFailedAttempts,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":           "invalid_credentials",
				"message":         "Invalid username or password. Account is now locked for 15 minutes.",
				"locked":          true,
				"lockoutDuration": "15 minutes",
			})
		}
	}
}

// handleResetLockout allows administrators to manually reset account lockouts
func (r *Router) handleResetLockout(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only POST method is allowed", nil)
		return
	}

	// Parse request
	var resetReq struct {
		Identifier string `json:"identifier"` // Can be username or IP
	}

	if err := json.NewDecoder(req.Body).Decode(&resetReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request",
			"Invalid request body", nil)
		return
	}

	if resetReq.Identifier == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_identifier",
			"Identifier (username or IP) is required", nil)
		return
	}

	// Reset the lockout
	ResetLockout(resetReq.Identifier)

	// Also clear failed login attempts
	ClearFailedLogins(resetReq.Identifier)

	// Audit log the reset
	LogAuditEvent("lockout_reset", "admin", GetClientIP(req), req.URL.Path, true,
		fmt.Sprintf("Lockout reset for: %s", resetReq.Identifier))

	log.Info().
		Str("identifier", resetReq.Identifier).
		Str("reset_by", "admin").
		Str("ip", GetClientIP(req)).
		Msg("Account lockout manually reset")

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Lockout reset for %s", resetReq.Identifier),
	})
}

// handleState handles state requests
func (r *Router) handleState(w http.ResponseWriter, req *http.Request) {
	log.Debug().Msg("[DEBUG] handleState: START")
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only GET method is allowed", nil)
		return
	}

	log.Debug().Msg("[DEBUG] handleState: Before auth check")
	// Use standard auth check (supports both basic auth and API tokens) unless auth is disabled
	if !r.config.DisableAuth && !CheckAuth(r.config, w, req) {
		writeErrorResponse(w, http.StatusUnauthorized, "unauthorized",
			"Authentication required", nil)
		return
	}

	log.Debug().Msg("[DEBUG] handleState: Before GetState")
	state := r.monitor.GetState()
	log.Debug().Msg("[DEBUG] handleState: After GetState, before ToFrontend")
	frontendState := state.ToFrontend()

	log.Debug().Msg("[DEBUG] handleState: Before WriteJSONResponse")
	if err := utils.WriteJSONResponse(w, frontendState); err != nil {
		log.Error().Err(err).Msg("Failed to encode state response")
		writeErrorResponse(w, http.StatusInternalServerError, "encoding_error",
			"Failed to encode state data", nil)
	}
	log.Debug().Msg("[DEBUG] handleState: END")
}

// handleVersion handles version requests
func (r *Router) handleVersion(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	versionInfo, err := updates.GetCurrentVersion()
	if err != nil {
		// Fallback to VERSION file
		versionBytes, _ := os.ReadFile("VERSION")
		response := VersionResponse{
			Version:   strings.TrimSpace(string(versionBytes)),
			BuildTime: "development",
			GoVersion: runtime.Version(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Convert to typed response
	response := VersionResponse{
		Version:   versionInfo.Version,
		BuildTime: versionInfo.Build,
		GoVersion: runtime.Version(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStorage handles storage detail requests
func (r *Router) handleStorage(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only GET method is allowed", nil)
		return
	}

	// Extract storage ID from path
	path := strings.TrimPrefix(req.URL.Path, "/api/storage/")
	if path == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_storage_id",
			"Storage ID is required", nil)
		return
	}

	// Get current state
	state := r.monitor.GetState()

	// Find the storage by ID
	var storageDetail *models.Storage
	for _, storage := range state.Storage {
		if storage.ID == path {
			storageDetail = &storage
			break
		}
	}

	if storageDetail == nil {
		writeErrorResponse(w, http.StatusNotFound, "storage_not_found",
			fmt.Sprintf("Storage with ID '%s' not found", path), nil)
		return
	}

	// Return storage details
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"data":      storageDetail,
		"timestamp": time.Now().Unix(),
	}); err != nil {
		log.Error().Err(err).Str("storage_id", path).Msg("Failed to encode storage details")
		writeErrorResponse(w, http.StatusInternalServerError, "encoding_error",
			"Failed to encode response", nil)
	}
}

// handleCharts handles chart data requests
func (r *Router) handleCharts(w http.ResponseWriter, req *http.Request) {
	log.Debug().Str("method", req.Method).Str("url", req.URL.String()).Msg("Charts endpoint hit")

	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get time range from query parameters
	query := req.URL.Query()
	timeRange := query.Get("range")
	if timeRange == "" {
		timeRange = "1h"
	}

	// Convert time range to duration
	var duration time.Duration
	switch timeRange {
	case "5m":
		duration = 5 * time.Minute
	case "15m":
		duration = 15 * time.Minute
	case "30m":
		duration = 30 * time.Minute
	case "1h":
		duration = time.Hour
	case "4h":
		duration = 4 * time.Hour
	case "12h":
		duration = 12 * time.Hour
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	default:
		duration = time.Hour
	}

	// Get current state from monitor
	state := r.monitor.GetState()

	// Create chart data structure that matches frontend expectations
	chartData := make(map[string]VMChartData)
	nodeData := make(map[string]NodeChartData)

	currentTime := time.Now().Unix() * 1000 // JavaScript timestamp format
	oldestTimestamp := currentTime

	// Process VMs - get historical data
	for _, vm := range state.VMs {
		if chartData[vm.ID] == nil {
			chartData[vm.ID] = make(VMChartData)
		}

		// Get historical metrics
		metrics := r.monitor.GetGuestMetrics(vm.ID, duration)

		// Convert metric points to API format
		for metricType, points := range metrics {
			chartData[vm.ID][metricType] = make([]MetricPoint, len(points))
			for i, point := range points {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				chartData[vm.ID][metricType][i] = MetricPoint{
					Timestamp: ts,
					Value:     point.Value,
				}
			}
		}

		// If no historical data, add current value
		if len(chartData[vm.ID]["cpu"]) == 0 {
			chartData[vm.ID]["cpu"] = []MetricPoint{
				{Timestamp: currentTime, Value: vm.CPU * 100},
			}
			chartData[vm.ID]["memory"] = []MetricPoint{
				{Timestamp: currentTime, Value: vm.Memory.Usage},
			}
			chartData[vm.ID]["disk"] = []MetricPoint{
				{Timestamp: currentTime, Value: vm.Disk.Usage},
			}
			chartData[vm.ID]["diskread"] = []MetricPoint{
				{Timestamp: currentTime, Value: float64(vm.DiskRead)},
			}
			chartData[vm.ID]["diskwrite"] = []MetricPoint{
				{Timestamp: currentTime, Value: float64(vm.DiskWrite)},
			}
			chartData[vm.ID]["netin"] = []MetricPoint{
				{Timestamp: currentTime, Value: float64(vm.NetworkIn)},
			}
			chartData[vm.ID]["netout"] = []MetricPoint{
				{Timestamp: currentTime, Value: float64(vm.NetworkOut)},
			}
		}
	}

	// Process Containers - get historical data
	for _, ct := range state.Containers {
		if chartData[ct.ID] == nil {
			chartData[ct.ID] = make(VMChartData)
		}

		// Get historical metrics
		metrics := r.monitor.GetGuestMetrics(ct.ID, duration)

		// Convert metric points to API format
		for metricType, points := range metrics {
			chartData[ct.ID][metricType] = make([]MetricPoint, len(points))
			for i, point := range points {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				chartData[ct.ID][metricType][i] = MetricPoint{
					Timestamp: ts,
					Value:     point.Value,
				}
			}
		}

		// If no historical data, add current value
		if len(chartData[ct.ID]["cpu"]) == 0 {
			chartData[ct.ID]["cpu"] = []MetricPoint{
				{Timestamp: currentTime, Value: ct.CPU * 100},
			}
			chartData[ct.ID]["memory"] = []MetricPoint{
				{Timestamp: currentTime, Value: ct.Memory.Usage},
			}
			chartData[ct.ID]["disk"] = []MetricPoint{
				{Timestamp: currentTime, Value: ct.Disk.Usage},
			}
			chartData[ct.ID]["diskread"] = []MetricPoint{
				{Timestamp: currentTime, Value: float64(ct.DiskRead)},
			}
			chartData[ct.ID]["diskwrite"] = []MetricPoint{
				{Timestamp: currentTime, Value: float64(ct.DiskWrite)},
			}
			chartData[ct.ID]["netin"] = []MetricPoint{
				{Timestamp: currentTime, Value: float64(ct.NetworkIn)},
			}
			chartData[ct.ID]["netout"] = []MetricPoint{
				{Timestamp: currentTime, Value: float64(ct.NetworkOut)},
			}
		}
	}

	// Process Storage - get historical data
	storageData := make(map[string]StorageChartData)
	for _, storage := range state.Storage {
		if storageData[storage.ID] == nil {
			storageData[storage.ID] = make(StorageChartData)
		}

		// Get historical metrics
		metrics := r.monitor.GetStorageMetrics(storage.ID, duration)

		// Convert usage metrics to chart format
		if usagePoints, ok := metrics["usage"]; ok && len(usagePoints) > 0 {
			// Convert MetricPoint slice to chart format
			storageData[storage.ID]["disk"] = make([]MetricPoint, len(usagePoints))
			for i, point := range usagePoints {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				storageData[storage.ID]["disk"][i] = MetricPoint{
					Timestamp: ts,
					Value:     point.Value,
				}
			}
		} else {
			// Add current value if no historical data
			usagePercent := float64(0)
			if storage.Total > 0 {
				usagePercent = (float64(storage.Used) / float64(storage.Total)) * 100
			}
			storageData[storage.ID]["disk"] = []MetricPoint{
				{Timestamp: currentTime, Value: usagePercent},
			}
		}
	}

	// Process Nodes - get historical data
	for _, node := range state.Nodes {
		if nodeData[node.ID] == nil {
			nodeData[node.ID] = make(NodeChartData)
		}

		// Get historical metrics for each type
		for _, metricType := range []string{"cpu", "memory", "disk"} {
			points := r.monitor.GetNodeMetrics(node.ID, metricType, duration)
			nodeData[node.ID][metricType] = make([]MetricPoint, len(points))
			for i, point := range points {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				nodeData[node.ID][metricType][i] = MetricPoint{
					Timestamp: ts,
					Value:     point.Value,
				}
			}

			// If no historical data, add current value
			if len(nodeData[node.ID][metricType]) == 0 {
				var value float64
				switch metricType {
				case "cpu":
					value = node.CPU * 100
				case "memory":
					value = node.Memory.Usage
				case "disk":
					value = node.Disk.Usage
				}
				nodeData[node.ID][metricType] = []MetricPoint{
					{Timestamp: currentTime, Value: value},
				}
			}
		}
	}

	response := ChartResponse{
		ChartData:   chartData,
		NodeData:    nodeData,
		StorageData: storageData,
		Timestamp:   currentTime,
		Stats: ChartStats{
			OldestDataTimestamp: oldestTimestamp,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode chart data response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Debug().
		Int("guests", len(chartData)).
		Int("nodes", len(nodeData)).
		Int("storage", len(storageData)).
		Str("range", timeRange).
		Msg("Chart data response sent")
}

// handleStorageCharts handles storage chart data requests
func (r *Router) handleStorageCharts(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	query := req.URL.Query()
	rangeMinutes := 60 // default 1 hour
	if rangeStr := query.Get("range"); rangeStr != "" {
		if _, err := fmt.Sscanf(rangeStr, "%d", &rangeMinutes); err != nil {
			log.Warn().Err(err).Str("range", rangeStr).Msg("Invalid range parameter; using default")
		}
	}

	duration := time.Duration(rangeMinutes) * time.Minute
	state := r.monitor.GetState()

	// Build storage chart data
	storageData := make(StorageChartsResponse)

	for _, storage := range state.Storage {
		metrics := r.monitor.GetStorageMetrics(storage.ID, duration)

		storageData[storage.ID] = StorageMetrics{
			Usage: metrics["usage"],
			Used:  metrics["used"],
			Total: metrics["total"],
			Avail: metrics["avail"],
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(storageData); err != nil {
		log.Error().Err(err).Msg("Failed to encode storage chart data")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleConfig handles configuration requests
func (r *Router) handleConfig(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return public configuration
	config := map[string]interface{}{
		"csrfProtection":    false, // Not implemented yet
		"autoUpdateEnabled": r.config.AutoUpdateEnabled,
		"updateChannel":     r.config.UpdateChannel,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// handleBackups handles backup requests
func (r *Router) handleBackups(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current state
	state := r.monitor.GetState()

	// Return backup data structure
	backups := map[string]interface{}{
		"backupTasks":    state.PVEBackups.BackupTasks,
		"storageBackups": state.PVEBackups.StorageBackups,
		"guestSnapshots": state.PVEBackups.GuestSnapshots,
		"pbsBackups":     state.PBSBackups,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(backups)
}

// handleBackupsPVE handles PVE backup requests
func (r *Router) handleBackupsPVE(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get state and extract PVE backups
	state := r.monitor.GetState()

	// Return PVE backup data in expected format
	backups := state.PVEBackups.StorageBackups
	if backups == nil {
		backups = []models.StorageBackup{}
	}

	pveBackups := map[string]interface{}{
		"backups": backups,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pveBackups); err != nil {
		log.Error().Err(err).Msg("Failed to encode PVE backups response")
		// Return empty array as fallback
		w.Write([]byte(`{"backups":[]}`))
	}
}

// handleBackupsPBS handles PBS backup requests
func (r *Router) handleBackupsPBS(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get state and extract PBS backups
	state := r.monitor.GetState()

	// Return PBS backup data in expected format
	instances := state.PBSInstances
	if instances == nil {
		instances = []models.PBSInstance{}
	}

	pbsData := map[string]interface{}{
		"instances": instances,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pbsData); err != nil {
		log.Error().Err(err).Msg("Failed to encode PBS response")
		// Return empty array as fallback
		w.Write([]byte(`{"instances":[]}`))
	}
}

// handleSnapshots handles snapshot requests
func (r *Router) handleSnapshots(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get state and extract guest snapshots
	state := r.monitor.GetState()

	// Return snapshot data
	snaps := state.PVEBackups.GuestSnapshots
	if snaps == nil {
		snaps = []models.GuestSnapshot{}
	}

	snapshots := map[string]interface{}{
		"snapshots": snaps,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(snapshots); err != nil {
		log.Error().Err(err).Msg("Failed to encode snapshots response")
		// Return empty array as fallback
		w.Write([]byte(`{"snapshots":[]}`))
	}
}

// handleWebSocket handles WebSocket connections
func (r *Router) handleWebSocket(w http.ResponseWriter, req *http.Request) {
	r.wsHub.HandleWebSocket(w, req)
}

// handleSimpleStats serves a simple stats page
func (r *Router) handleSimpleStats(w http.ResponseWriter, req *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Simple Pulse Stats</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
            background: #f5f5f5;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            background: white;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        th {
            background: #333;
            color: white;
            font-weight: bold;
            position: sticky;
            top: 0;
        }
        tr:hover {
            background: #f5f5f5;
        }
        .status {
            padding: 4px 8px;
            border-radius: 4px;
            color: white;
            font-size: 12px;
        }
        .running { background: #28a745; }
        .stopped { background: #dc3545; }
        #status {
            margin-bottom: 20px;
            padding: 10px;
            background: #e9ecef;
            border-radius: 4px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .update-indicator {
            display: inline-block;
            width: 10px;
            height: 10px;
            background: #28a745;
            border-radius: 50%;
            animation: pulse 0.5s ease-out;
        }
        @keyframes pulse {
            0% { transform: scale(1); opacity: 1; }
            50% { transform: scale(1.5); opacity: 0.7; }
            100% { transform: scale(1); opacity: 1; }
        }
        .update-timer {
            font-family: monospace;
            font-size: 14px;
            color: #666;
        }
        .metric {
            font-family: monospace;
            text-align: right;
        }
    </style>
</head>
<body>
    <h1>Simple Pulse Stats</h1>
    <div id="status">
        <div>
            <span id="status-text">Connecting...</span>
            <span class="update-indicator" id="update-indicator" style="display:none"></span>
        </div>
        <div class="update-timer" id="update-timer"></div>
    </div>
    
    <h2>Containers</h2>
    <table id="containers">
        <thead>
            <tr>
                <th>Name</th>
                <th>Status</th>
                <th>CPU %</th>
                <th>Memory</th>
                <th>Disk Read</th>
                <th>Disk Write</th>
                <th>Net In</th>
                <th>Net Out</th>
            </tr>
        </thead>
        <tbody></tbody>
    </table>

    <script>
        let ws;
        let lastUpdateTime = null;
        let updateCount = 0;
        let updateInterval = null;
        
        function formatBytes(bytes) {
            if (!bytes || bytes < 0) return '0 B/s';
            const units = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
            let i = 0;
            let value = bytes;
            while (value >= 1024 && i < units.length - 1) {
                value /= 1024;
                i++;
            }
            return value.toFixed(1) + ' ' + units[i];
        }
        
        function formatMemory(used, total) {
            const usedGB = (used / 1024 / 1024 / 1024).toFixed(1);
            const totalGB = (total / 1024 / 1024 / 1024).toFixed(1);
            const percent = ((used / total) * 100).toFixed(0);
            return usedGB + '/' + totalGB + ' GB (' + percent + '%)';
        }
        
        function updateTable(containers) {
            const tbody = document.querySelector('#containers tbody');
            tbody.innerHTML = '';
            
            containers.sort((a, b) => a.name.localeCompare(b.name));
            
            containers.forEach(ct => {
                const row = document.createElement('tr');
                row.innerHTML = 
                    '<td><strong>' + ct.name + '</strong></td>' +
                    '<td><span class="status ' + ct.status + '">' + ct.status + '</span></td>' +
                    '<td class="metric">' + (ct.cpu ? ct.cpu.toFixed(1) : '0.0') + '%</td>' +
                    '<td class="metric">' + formatMemory(ct.mem || 0, ct.maxmem || 1) + '</td>' +
                    '<td class="metric">' + formatBytes(ct.diskread) + '</td>' +
                    '<td class="metric">' + formatBytes(ct.diskwrite) + '</td>' +
                    '<td class="metric">' + formatBytes(ct.netin) + '</td>' +
                    '<td class="metric">' + formatBytes(ct.netout) + '</td>';
                tbody.appendChild(row);
            });
        }
        
        function updateTimer() {
            if (lastUpdateTime) {
                const secondsSince = Math.floor((Date.now() - lastUpdateTime) / 1000);
                document.getElementById('update-timer').textContent = 'Next update in: ' + (2 - (secondsSince % 2)) + 's';
            }
        }
        
        function connect() {
            const statusText = document.getElementById('status-text');
            const indicator = document.getElementById('update-indicator');
            statusText.textContent = 'Connecting to WebSocket...';
            
            ws = new WebSocket('ws://' + window.location.host + '/ws');
            
            ws.onopen = function() {
                statusText.textContent = 'Connected! Updates every 2 seconds';
                console.log('WebSocket connected');
                // Start the countdown timer
                if (updateInterval) clearInterval(updateInterval);
                updateInterval = setInterval(updateTimer, 100);
            };
            
            ws.onmessage = function(event) {
                try {
                    const msg = JSON.parse(event.data);
                    
                    if (msg.type === 'initialState' || msg.type === 'rawData') {
                        if (msg.data && msg.data.containers) {
                            updateCount++;
                            lastUpdateTime = Date.now();
                            
                            // Show update indicator with animation
                            indicator.style.display = 'inline-block';
                            indicator.style.animation = 'none';
                            setTimeout(() => {
                                indicator.style.animation = 'pulse 0.5s ease-out';
                            }, 10);
                            
                            statusText.textContent = 'Update #' + updateCount + ' at ' + new Date().toLocaleTimeString();
                            updateTable(msg.data.containers);
                        }
                    }
                } catch (err) {
                    console.error('Parse error:', err);
                }
            };
            
            ws.onclose = function(event) {
                statusText.textContent = 'Disconnected: ' + event.code + ' ' + event.reason + '. Reconnecting in 3s...';
                indicator.style.display = 'none';
                if (updateInterval) clearInterval(updateInterval);
                setTimeout(connect, 3000);
            };
            
            ws.onerror = function(error) {
                statusText.textContent = 'Connection error. Retrying...';
                console.error('WebSocket error:', error);
            };
        }
        
        // Start connection
        connect();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// handleSocketIO handles socket.io requests
func (r *Router) handleSocketIO(w http.ResponseWriter, req *http.Request) {
	// For socket.io.js, redirect to CDN
	if strings.Contains(req.URL.Path, "socket.io.js") {
		http.Redirect(w, req, "https://cdn.socket.io/4.8.1/socket.io.min.js", http.StatusFound)
		return
	}

	// For other socket.io endpoints, use our WebSocket
	// This provides basic compatibility
	if strings.Contains(req.URL.RawQuery, "transport=websocket") {
		r.wsHub.HandleWebSocket(w, req)
		return
	}

	// For polling transport, return proper socket.io response
	// Socket.io v4 expects specific format
	if strings.Contains(req.URL.RawQuery, "transport=polling") {
		if strings.Contains(req.URL.RawQuery, "sid=") {
			// Already connected, return empty poll
			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("6"))
		} else {
			// Initial handshake
			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			w.WriteHeader(http.StatusOK)
			// Send open packet with session ID and config
			sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
			response := fmt.Sprintf(`0{"sid":"%s","upgrades":["websocket"],"pingInterval":25000,"pingTimeout":60000}`, sessionID)
			w.Write([]byte(response))
		}
		return
	}

	// Default: redirect to WebSocket
	http.Redirect(w, req, "/ws", http.StatusFound)
}

// forwardUpdateProgress forwards update progress to WebSocket clients
func (r *Router) forwardUpdateProgress() {
	progressChan := r.updateManager.GetProgressChannel()

	for status := range progressChan {
		// Create update event for WebSocket
		message := websocket.Message{
			Type:      "update:progress",
			Data:      status,
			Timestamp: time.Now().Format(time.RFC3339),
		}

		// Broadcast to all connected clients
		r.wsHub.BroadcastMessage(message)

		// Log progress
		log.Debug().
			Str("status", status.Status).
			Int("progress", status.Progress).
			Str("message", status.Message).
			Msg("Update progress")
	}
}
