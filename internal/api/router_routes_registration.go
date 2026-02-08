package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

func (r *Router) registerPublicAndAuthRoutes() {
	r.registerAuthSecurityInstallRoutes()
}

func (r *Router) registerMonitoringRoutes(
	guestMetadataHandler *GuestMetadataHandler,
	dockerMetadataHandler *DockerMetadataHandler,
	hostMetadataHandler *HostMetadataHandler,
	infraUpdateHandlers *UpdateDetectionHandlers,
) {
	r.registerMonitoringResourceRoutes(
		guestMetadataHandler,
		dockerMetadataHandler,
		hostMetadataHandler,
		infraUpdateHandlers,
	)
}

func (r *Router) registerConfigSystemRoutes(updateHandlers *UpdateHandlers) {
	// Log management routes
	r.mux.HandleFunc("/api/logs/stream", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.logHandlers.HandleStreamLogs)))
	r.mux.HandleFunc("/api/logs/download", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.logHandlers.HandleDownloadBundle)))
	r.mux.HandleFunc("/api/logs/level", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.logHandlers.HandleGetLevel))(w, req)
		case http.MethodPost:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.logHandlers.HandleSetLevel))(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	r.mux.HandleFunc("/api/agents/docker/report", RequireAuth(r.config, RequireScope(config.ScopeDockerReport, r.dockerAgentHandlers.HandleReport)))
	r.mux.HandleFunc("/api/agents/kubernetes/report", RequireAuth(r.config, RequireScope(config.ScopeKubernetesReport, r.kubernetesAgentHandlers.HandleReport)))
	r.mux.HandleFunc("/api/agents/host/report", RequireAuth(r.config, RequireScope(config.ScopeHostReport, r.hostAgentHandlers.HandleReport)))
	r.mux.HandleFunc("/api/agents/host/lookup", RequireAuth(r.config, RequireScope(config.ScopeHostReport, r.hostAgentHandlers.HandleLookup)))
	r.mux.HandleFunc("/api/agents/host/uninstall", RequireAuth(r.config, RequireScope(config.ScopeHostReport, r.hostAgentHandlers.HandleUninstall)))
	// SECURITY: Use settings:write (not just host_manage) to prevent compromised host tokens from manipulating other hosts
	r.mux.HandleFunc("/api/agents/host/unlink", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.hostAgentHandlers.HandleUnlink)))
	r.mux.HandleFunc("/api/agents/host/link", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.hostAgentHandlers.HandleLink)))
	// Host agent management routes - config endpoint is accessible by agents (GET) and admins (PATCH)
	r.mux.HandleFunc("/api/agents/host/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		// Route /api/agents/host/{id}/config to HandleConfig
		if strings.HasSuffix(req.URL.Path, "/config") {
			// GET is for agents to fetch config (host config scope)
			// PATCH is for UI to update config (host_manage scope, admin only)
			if req.Method == http.MethodPatch {
				RequireAdmin(r.config, func(w http.ResponseWriter, req *http.Request) {
					if !ensureScope(w, req, config.ScopeHostManage) {
						return
					}
					r.hostAgentHandlers.HandleConfig(w, req)
				})(w, req)
				return
			}
			r.hostAgentHandlers.HandleConfig(w, req)
			return
		}
		// Route DELETE /api/agents/host/{id} to HandleDeleteHost
		// SECURITY: Require settings:write (not just host_manage) to prevent compromised host tokens from deleting other hosts
		if req.Method == http.MethodDelete {
			RequireAdmin(r.config, func(w http.ResponseWriter, req *http.Request) {
				if !ensureScope(w, req, config.ScopeSettingsWrite) {
					return
				}
				r.hostAgentHandlers.HandleDeleteHost(w, req)
			})(w, req)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}))
	r.mux.HandleFunc("/api/agents/docker/commands/", RequireAuth(r.config, RequireScope(config.ScopeDockerReport, r.dockerAgentHandlers.HandleCommandAck)))
	r.mux.HandleFunc("/api/agents/docker/hosts/", RequireAdmin(r.config, RequireScope(config.ScopeDockerManage, r.dockerAgentHandlers.HandleDockerHostActions)))
	r.mux.HandleFunc("/api/agents/docker/containers/update", RequireAdmin(r.config, RequireScope(config.ScopeDockerManage, r.dockerAgentHandlers.HandleContainerUpdate)))
	r.mux.HandleFunc("/api/agents/kubernetes/clusters/", RequireAdmin(r.config, RequireScope(config.ScopeKubernetesManage, r.kubernetesAgentHandlers.HandleClusterActions)))
	r.mux.HandleFunc("/api/diagnostics", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.handleDiagnostics)))
	r.mux.HandleFunc("/api/diagnostics/docker/prepare-token", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.handleDiagnosticsDockerPrepareToken)))
	r.mux.HandleFunc("/api/config", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleConfig)))
	// Update routes
	r.mux.HandleFunc("/api/updates/check", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleCheckUpdates)))
	r.mux.HandleFunc("/api/updates/apply", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, updateHandlers.HandleApplyUpdate)))
	r.mux.HandleFunc("/api/updates/status", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleUpdateStatus)))
	r.mux.HandleFunc("/api/updates/stream", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleUpdateStream)))
	r.mux.HandleFunc("/api/updates/plan", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleGetUpdatePlan)))
	r.mux.HandleFunc("/api/updates/history", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleListUpdateHistory)))
	r.mux.HandleFunc("/api/updates/history/entry", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleGetUpdateHistoryEntry)))
	// Config management routes
	r.mux.HandleFunc("/api/config/nodes", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.configHandlers.HandleGetNodes))(w, req)
		case http.MethodPost:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleAddNode))(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	// Test node configuration endpoint (for new nodes)
	r.mux.HandleFunc("/api/config/nodes/test-config", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleTestNodeConfig))(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Test connection endpoint
	r.mux.HandleFunc("/api/config/nodes/test-connection", func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleTestConnection))(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	r.mux.HandleFunc("/api/config/nodes/", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPut:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleUpdateNode))(w, req)
		case http.MethodDelete:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleDeleteNode))(w, req)
		case http.MethodPost:
			// Handle test endpoint and refresh-cluster endpoint
			if strings.HasSuffix(req.URL.Path, "/test") {
				RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleTestNode))(w, req)
			} else if strings.HasSuffix(req.URL.Path, "/refresh-cluster") {
				RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleRefreshClusterNodes))(w, req)
			} else {
				http.Error(w, "Not found", http.StatusNotFound)
			}
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// TrueNAS connection management
	r.mux.HandleFunc("/api/truenas/connections", func(w http.ResponseWriter, req *http.Request) {
		if r.trueNASHandlers == nil {
			writeErrorResponse(w, http.StatusServiceUnavailable, "truenas_unavailable", "TrueNAS service unavailable", nil)
			return
		}

		switch req.Method {
		case http.MethodGet:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.trueNASHandlers.HandleList))(w, req)
		case http.MethodPost:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.trueNASHandlers.HandleAdd))(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	r.mux.HandleFunc("/api/truenas/connections/test", func(w http.ResponseWriter, req *http.Request) {
		if r.trueNASHandlers == nil {
			writeErrorResponse(w, http.StatusServiceUnavailable, "truenas_unavailable", "TrueNAS service unavailable", nil)
			return
		}

		if req.Method == http.MethodPost {
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.trueNASHandlers.HandleTestConnection))(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	r.mux.HandleFunc("/api/truenas/connections/", func(w http.ResponseWriter, req *http.Request) {
		if r.trueNASHandlers == nil {
			writeErrorResponse(w, http.StatusServiceUnavailable, "truenas_unavailable", "TrueNAS service unavailable", nil)
			return
		}

		if req.Method == http.MethodDelete {
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.trueNASHandlers.HandleDelete))(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Config Profile Routes - Protected by Admin Auth, Settings Scope, and Pro License
	// SECURITY: Require settings:write scope to prevent low-privilege tokens from modifying agent profiles
	// r.configProfileHandler.ServeHTTP implements http.Handler, so we wrap it
	r.mux.Handle("/api/admin/profiles/", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, RequireLicenseFeature(r.licenseHandlers, license.FeatureAgentProfiles, func(w http.ResponseWriter, req *http.Request) {
		http.StripPrefix("/api/admin/profiles", r.configProfileHandler).ServeHTTP(w, req)
	}))))

	// System settings routes
	r.mux.HandleFunc("/api/config/system", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			handler := r.configHandlers.HandleGetSystemSettings
			if r.systemSettingsHandler != nil {
				handler = r.systemSettingsHandler.HandleGetSystemSettings
			}
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, handler))(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Mock mode toggle routes
	r.mux.HandleFunc("/api/system/mock-mode", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.configHandlers.HandleGetMockMode))(w, req)
		case http.MethodPost, http.MethodPut:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleUpdateMockMode))(w, req)
		default:
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

			validateAPIToken := func(token string) bool {
				if token == "" || !r.config.HasAPITokens() {
					return false
				}
				_, ok := r.config.ValidateAPIToken(token)
				return ok
			}

			token := req.Header.Get("X-API-Token")
			if token == "" {
				if authHeader := req.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
					token = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}
			hasValidAPIToken := validateAPIToken(token)

			// Check if any valid auth method is present
			hasValidAuth := hasValidProxyAuth || hasValidSession || hasValidAPIToken

			// Determine if auth is required
			authRequired := r.config.AuthUser != "" && r.config.AuthPass != "" ||
				r.config.HasAPITokens() ||
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
				clientIP := GetClientIP(req)

				isPrivate := isPrivateIP(clientIP)
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

			// SECURITY: Check settings:read scope for API token auth
			if hasValidAPIToken && token != "" {
				record, _ := r.config.ValidateAPIToken(token)
				if record != nil && !record.HasScope(config.ScopeSettingsRead) {
					log.Warn().
						Str("ip", req.RemoteAddr).
						Str("path", req.URL.Path).
						Str("token_id", record.ID).
						Msg("API token missing settings:read scope for export")
					http.Error(w, "API token missing required scope: settings:read", http.StatusForbidden)
					return
				}
			}

			// Log successful export attempt
			log.Info().
				Str("ip", req.RemoteAddr).
				Bool("proxy_auth", hasValidProxyAuth).
				Bool("session_auth", hasValidSession).
				Bool("api_token_auth", hasValidAPIToken).
				Msg("Configuration export initiated")

			r.configHandlers.HandleExportConfig(w, req)
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

			validateAPIToken := func(token string) bool {
				if token == "" || !r.config.HasAPITokens() {
					return false
				}
				_, ok := r.config.ValidateAPIToken(token)
				return ok
			}

			token := req.Header.Get("X-API-Token")
			if token == "" {
				if authHeader := req.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
					token = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}
			hasValidAPIToken := validateAPIToken(token)

			// Check if any valid auth method is present
			hasValidAuth := hasValidProxyAuth || hasValidSession || hasValidAPIToken

			// Determine if auth is required
			authRequired := r.config.AuthUser != "" && r.config.AuthPass != "" ||
				r.config.HasAPITokens() ||
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
				clientIP := GetClientIP(req)

				isPrivate := isPrivateIP(clientIP)
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

			// SECURITY: Check settings:write scope for API token auth
			if hasValidAPIToken && token != "" {
				record, _ := r.config.ValidateAPIToken(token)
				if record != nil && !record.HasScope(config.ScopeSettingsWrite) {
					log.Warn().
						Str("ip", req.RemoteAddr).
						Str("path", req.URL.Path).
						Str("token_id", record.ID).
						Msg("API token missing settings:write scope for import")
					http.Error(w, "API token missing required scope: settings:write", http.StatusForbidden)
					return
				}
			}

			// Log successful import attempt
			log.Info().
				Str("ip", req.RemoteAddr).
				Bool("session_auth", hasValidSession).
				Bool("api_token_auth", hasValidAPIToken).
				Msg("Configuration import initiated")

			r.configHandlers.HandleImportConfig(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Discovery route

	// Setup script route
	r.mux.HandleFunc("/api/setup-script", r.configHandlers.HandleSetupScript)

	// Generate setup script URL with temporary token (for authenticated users)
	r.mux.HandleFunc("/api/setup-script-url", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleSetupScriptURL)))

	// Generate agent install command with API token (for authenticated users)
	r.mux.HandleFunc("/api/agent-install-command", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.configHandlers.HandleAgentInstallCommand)))

	// Auto-register route for setup scripts
	r.mux.HandleFunc("/api/auto-register", r.configHandlers.HandleAutoRegister)
	// Discovery endpoint
	// Test endpoint for WebSocket notifications
	// SECURITY: Require settings:write scope for test notifications to prevent unauthenticated broadcasting
	r.mux.HandleFunc("/api/test-notification", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, func(w http.ResponseWriter, req *http.Request) {
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
	})))
	r.mux.HandleFunc("/api/system/settings", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.systemSettingsHandler.HandleGetSystemSettings)))
	r.mux.HandleFunc("/api/system/settings/update", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.systemSettingsHandler.HandleUpdateSystemSettings)))
	r.mux.HandleFunc("/api/system/ssh-config", r.handleSSHConfig)
	r.mux.HandleFunc("/api/system/verify-temperature-ssh", r.handleVerifyTemperatureSSH)
}

func (r *Router) registerAIRelayRoutes() {
	r.registerAIRelayRoutesGroup()
}

func (r *Router) registerOrgLicenseRoutes(orgHandlers *OrgHandlers, rbacHandlers *RBACHandlers, auditHandlers *AuditHandlers) {
	r.registerOrgLicenseRoutesGroup(orgHandlers, rbacHandlers, auditHandlers)
}
