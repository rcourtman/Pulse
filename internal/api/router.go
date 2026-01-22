package api

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentbinaries"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

// Router handles HTTP routing
type Router struct {
	mux                       *http.ServeMux
	config                    *config.Config
	monitor                   *monitoring.Monitor            // Legacy/Default support
	mtMonitor                 *monitoring.MultiTenantMonitor // Multi-tenant manager
	alertHandlers             *AlertHandlers
	configHandlers            *ConfigHandlers
	notificationHandlers      *NotificationHandlers
	notificationQueueHandlers *NotificationQueueHandlers
	dockerAgentHandlers       *DockerAgentHandlers
	kubernetesAgentHandlers   *KubernetesAgentHandlers
	hostAgentHandlers         *HostAgentHandlers
	systemSettingsHandler     *SystemSettingsHandler
	aiSettingsHandler         *AISettingsHandler
	aiHandler                 *AIHandler // AI chat handler
	resourceHandlers          *ResourceHandlers
	reportingHandlers         *ReportingHandlers
	configProfileHandler      *ConfigProfileHandler
	licenseHandlers           *LicenseHandlers
	agentExecServer           *agentexec.Server
	wsHub                     *websocket.Hub
	reloadFunc                func() error
	updateManager             *updates.Manager
	updateHistory             *updates.UpdateHistory
	exportLimiter             *RateLimiter
	downloadLimiter           *RateLimiter
	persistence               *config.ConfigPersistence
	multiTenant               *config.MultiTenantPersistence
	oidcMu                    sync.Mutex
	oidcService               *OIDCService
	samlManager               *SAMLServiceManager
	ssoConfig                 *config.SSOConfig
	authorizer                auth.Authorizer
	wrapped                   http.Handler
	serverVersion             string
	projectRoot               string
	// Cached system settings to avoid loading from disk on every request
	settingsMu           sync.RWMutex
	cachedAllowEmbedding bool
	cachedAllowedOrigins string
	publicURLMu          sync.Mutex
	publicURLDetected    bool
	bootstrapTokenHash   string
	bootstrapTokenPath   string
	checksumMu           sync.RWMutex
	checksumCache        map[string]checksumCacheEntry
}

func pulseBinDir() string {
	if dir := strings.TrimSpace(os.Getenv("PULSE_BIN_DIR")); dir != "" {
		return dir
	}
	return "/opt/pulse/bin"
}

func isDirectLoopbackRequest(req *http.Request) bool {
	if req == nil {
		return false
	}

	remote := extractRemoteIP(req.RemoteAddr)
	ip := net.ParseIP(remote)
	if ip == nil || !ip.IsLoopback() {
		return false
	}

	if req.Header.Get("X-Forwarded-For") != "" ||
		req.Header.Get("Forwarded") != "" ||
		req.Header.Get("X-Real-IP") != "" {
		return false
	}

	return true
}

// NewRouter creates a new router instance
func NewRouter(cfg *config.Config, monitor *monitoring.Monitor, mtMonitor *monitoring.MultiTenantMonitor, wsHub *websocket.Hub, reloadFunc func() error, serverVersion string) *Router {
	// Initialize persistent session and CSRF stores
	InitSessionStore(cfg.DataPath)
	InitCSRFStore(cfg.DataPath)

	updateHistory, err := updates.NewUpdateHistory(cfg.DataPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize update history")
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		projectRoot = "."
	}

	updateManager := updates.NewManager(cfg)
	updateManager.SetHistory(updateHistory)

	r := &Router{
		mux:             http.NewServeMux(),
		config:          cfg,
		monitor:         monitor,
		mtMonitor:       mtMonitor,
		wsHub:           wsHub,
		reloadFunc:      reloadFunc,
		updateManager:   updateManager,
		updateHistory:   updateHistory,
		exportLimiter:   NewRateLimiter(5, 1*time.Minute),  // 5 attempts per minute
		downloadLimiter: NewRateLimiter(60, 1*time.Minute), // downloads/installers per minute per IP
		persistence:     config.NewConfigPersistence(cfg.DataPath),
		multiTenant:     config.NewMultiTenantPersistence(cfg.DataPath),
		authorizer:      auth.GetAuthorizer(),
		serverVersion:   strings.TrimSpace(serverVersion),
		projectRoot:     projectRoot,
		checksumCache:   make(map[string]checksumCacheEntry),
	}

	// Sync the configured admin user to the authorizer (if supported)
	if cfg.AuthUser != "" {
		auth.SetAdminUser(cfg.AuthUser)
	}

	// Initialize SAML manager (baseURL will be set dynamically on first use)
	r.samlManager = NewSAMLServiceManager("")

	r.initializeBootstrapToken()

	r.setupRoutes()
	log.Debug().Msg("Routes registered successfully")

	// Start forwarding update progress to WebSocket
	go r.forwardUpdateProgress()

	// Start background update checker
	go r.backgroundUpdateChecker()

	// Load system settings once at startup and cache them
	r.reloadSystemSettings()

	// Get cached values for middleware configuration
	r.settingsMu.RLock()
	allowEmbedding := r.cachedAllowEmbedding
	allowedOrigins := r.cachedAllowedOrigins
	r.settingsMu.RUnlock()

	// Apply middleware chain:
	// 1. Universal rate limiting (outermost to stop attacks early)
	// 2. Demo mode (read-only protection)
	// 3. Error handling
	// 4. Security headers with embedding configuration
	// Note: TimeoutHandler breaks WebSocket upgrades
	handler := SecurityHeadersWithConfig(r, allowEmbedding, allowedOrigins)
	handler = ErrorHandler(handler)
	handler = DemoModeMiddleware(cfg, handler)
	handler = NewTenantMiddleware(r.multiTenant).Middleware(handler)
	handler = UniversalRateLimitMiddleware(handler)
	r.wrapped = handler
	return r
}

// setupRoutes configures all routes
func (r *Router) setupRoutes() {
	// Create handlers
	r.alertHandlers = NewAlertHandlers(r.mtMonitor, NewAlertMonitorWrapper(r.monitor), r.wsHub)
	r.notificationHandlers = NewNotificationHandlers(r.mtMonitor, NewNotificationMonitorWrapper(r.monitor))
	r.notificationQueueHandlers = NewNotificationQueueHandlers(r.monitor)
	guestMetadataHandler := NewGuestMetadataHandler(r.multiTenant)
	dockerMetadataHandler := NewDockerMetadataHandler(r.multiTenant)
	hostMetadataHandler := NewHostMetadataHandler(r.multiTenant)
	r.configHandlers = NewConfigHandlers(r.multiTenant, r.mtMonitor, r.reloadFunc, r.wsHub, guestMetadataHandler, r.reloadSystemSettings)
	if r.monitor != nil {
		r.configHandlers.SetMonitor(r.monitor)
	}
	updateHandlers := NewUpdateHandlers(r.updateManager, r.updateHistory)
	r.dockerAgentHandlers = NewDockerAgentHandlers(r.mtMonitor, r.monitor, r.wsHub, r.config)
	r.kubernetesAgentHandlers = NewKubernetesAgentHandlers(r.mtMonitor, r.monitor, r.wsHub)
	r.hostAgentHandlers = NewHostAgentHandlers(r.mtMonitor, r.monitor, r.wsHub)
	r.resourceHandlers = NewResourceHandlers()
	r.configProfileHandler = NewConfigProfileHandler(r.multiTenant)
	r.licenseHandlers = NewLicenseHandlers(r.multiTenant)
	r.reportingHandlers = NewReportingHandlers()
	rbacHandlers := NewRBACHandlers(r.config)

	// API routes
	r.mux.HandleFunc("/api/health", r.handleHealth)
	r.mux.HandleFunc("/api/monitoring/scheduler/health", RequireAuth(r.config, r.handleSchedulerHealth))
	r.mux.HandleFunc("/api/state", r.handleState)
	r.mux.HandleFunc("/api/agents/docker/report", RequireAuth(r.config, RequireScope(config.ScopeDockerReport, r.dockerAgentHandlers.HandleReport)))
	r.mux.HandleFunc("/api/agents/kubernetes/report", RequireAuth(r.config, RequireScope(config.ScopeKubernetesReport, r.kubernetesAgentHandlers.HandleReport)))
	r.mux.HandleFunc("/api/agents/host/report", RequireAuth(r.config, RequireScope(config.ScopeHostReport, r.hostAgentHandlers.HandleReport)))
	r.mux.HandleFunc("/api/agents/host/lookup", RequireAuth(r.config, RequireScope(config.ScopeHostReport, r.hostAgentHandlers.HandleLookup)))
	r.mux.HandleFunc("/api/agents/host/uninstall", RequireAuth(r.config, RequireScope(config.ScopeHostReport, r.hostAgentHandlers.HandleUninstall)))
	r.mux.HandleFunc("/api/agents/host/unlink", RequireAdmin(r.config, RequireScope(config.ScopeHostManage, r.hostAgentHandlers.HandleUnlink)))
	r.mux.HandleFunc("/api/agents/host/link", RequireAdmin(r.config, RequireScope(config.ScopeHostManage, r.hostAgentHandlers.HandleLink)))
	// Host agent management routes - config endpoint is accessible by agents (GET) and admins (PATCH)
	r.mux.HandleFunc("/api/agents/host/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		// Route /api/agents/host/{id}/config to HandleConfig
		if strings.HasSuffix(req.URL.Path, "/config") {
			// GET is for agents to fetch config (host config scope)
			// PATCH is for UI to update config (host_manage scope, admin only)
			if req.Method == http.MethodPatch {
				if !ensureScope(w, req, config.ScopeHostManage) {
					return
				}
			}
			r.hostAgentHandlers.HandleConfig(w, req)
			return
		}
		// Route DELETE /api/agents/host/{id} to HandleDeleteHost (host_manage scope)
		if req.Method == http.MethodDelete {
			if !ensureScope(w, req, config.ScopeHostManage) {
				return
			}
			r.hostAgentHandlers.HandleDeleteHost(w, req)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}))
	r.mux.HandleFunc("/api/agents/docker/commands/", RequireAuth(r.config, RequireScope(config.ScopeDockerReport, r.dockerAgentHandlers.HandleCommandAck)))
	r.mux.HandleFunc("/api/agents/docker/hosts/", RequireAdmin(r.config, RequireScope(config.ScopeDockerManage, r.dockerAgentHandlers.HandleDockerHostActions)))
	r.mux.HandleFunc("/api/agents/docker/containers/update", RequireAdmin(r.config, RequireScope(config.ScopeDockerManage, r.dockerAgentHandlers.HandleContainerUpdate)))
	r.mux.HandleFunc("/api/agents/kubernetes/clusters/", RequireAdmin(r.config, RequireScope(config.ScopeKubernetesManage, r.kubernetesAgentHandlers.HandleClusterActions)))
	r.mux.HandleFunc("/api/version", r.handleVersion)
	r.mux.HandleFunc("/api/storage/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleStorage)))
	r.mux.HandleFunc("/api/storage-charts", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleStorageCharts)))
	r.mux.HandleFunc("/api/charts", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleCharts)))
	r.mux.HandleFunc("/api/metrics-store/stats", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleMetricsStoreStats)))
	r.mux.HandleFunc("/api/metrics-store/history", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleMetricsHistory)))
	r.mux.HandleFunc("/api/diagnostics", RequireAuth(r.config, r.handleDiagnostics))
	r.mux.HandleFunc("/api/diagnostics/docker/prepare-token", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.handleDiagnosticsDockerPrepareToken)))
	r.mux.HandleFunc("/api/install/install-docker.sh", r.handleDownloadDockerInstallerScript)
	r.mux.HandleFunc("/api/install/install.sh", r.handleDownloadUnifiedInstallScript)
	r.mux.HandleFunc("/api/install/install.ps1", r.handleDownloadUnifiedInstallScriptPS)
	r.mux.HandleFunc("/api/config", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleConfig)))
	r.mux.HandleFunc("/api/backups", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleBackups)))
	r.mux.HandleFunc("/api/backups/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleBackups)))
	r.mux.HandleFunc("/api/backups/unified", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleBackups)))
	r.mux.HandleFunc("/api/backups/pve", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleBackupsPVE)))
	r.mux.HandleFunc("/api/backups/pbs", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleBackupsPBS)))
	r.mux.HandleFunc("/api/snapshots", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.handleSnapshots)))

	// Unified resources API (Phase 1 of unified resource architecture)
	r.mux.HandleFunc("/api/resources", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceHandlers.HandleGetResources)))
	r.mux.HandleFunc("/api/resources/stats", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceHandlers.HandleGetResourceStats)))
	r.mux.HandleFunc("/api/resources/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, r.resourceHandlers.HandleGetResource)))

	// Guest metadata routes
	r.mux.HandleFunc("/api/guests/metadata", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, guestMetadataHandler.HandleGetMetadata)))
	r.mux.HandleFunc("/api/guests/metadata/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeMonitoringRead) {
				return
			}
			guestMetadataHandler.HandleGetMetadata(w, req)
		case http.MethodPut, http.MethodPost:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			guestMetadataHandler.HandleUpdateMetadata(w, req)
		case http.MethodDelete:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			guestMetadataHandler.HandleDeleteMetadata(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Docker metadata routes
	r.mux.HandleFunc("/api/docker/metadata", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, dockerMetadataHandler.HandleGetMetadata)))
	r.mux.HandleFunc("/api/docker/metadata/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeMonitoringRead) {
				return
			}
			dockerMetadataHandler.HandleGetMetadata(w, req)
		case http.MethodPut, http.MethodPost:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			dockerMetadataHandler.HandleUpdateMetadata(w, req)
		case http.MethodDelete:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			dockerMetadataHandler.HandleDeleteMetadata(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Docker host metadata routes (for managing Docker host custom URLs, e.g., Portainer links)
	r.mux.HandleFunc("/api/docker/hosts/metadata", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, dockerMetadataHandler.HandleGetHostMetadata)))
	r.mux.HandleFunc("/api/docker/hosts/metadata/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeMonitoringRead) {
				return
			}
			dockerMetadataHandler.HandleGetHostMetadata(w, req)
		case http.MethodPut, http.MethodPost:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			dockerMetadataHandler.HandleUpdateHostMetadata(w, req)
		case http.MethodDelete:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			dockerMetadataHandler.HandleDeleteHostMetadata(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Host metadata routes
	r.mux.HandleFunc("/api/hosts/metadata", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, hostMetadataHandler.HandleGetMetadata)))
	r.mux.HandleFunc("/api/hosts/metadata/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			if !ensureScope(w, req, config.ScopeMonitoringRead) {
				return
			}
			hostMetadataHandler.HandleGetMetadata(w, req)
		case http.MethodPut, http.MethodPost:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			hostMetadataHandler.HandleUpdateMetadata(w, req)
		case http.MethodDelete:
			if !ensureScope(w, req, config.ScopeMonitoringWrite) {
				return
			}
			hostMetadataHandler.HandleDeleteMetadata(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Update routes
	r.mux.HandleFunc("/api/updates/check", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleCheckUpdates)))
	r.mux.HandleFunc("/api/updates/apply", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, updateHandlers.HandleApplyUpdate)))
	r.mux.HandleFunc("/api/updates/status", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleUpdateStatus)))
	r.mux.HandleFunc("/api/updates/stream", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleUpdateStream)))
	r.mux.HandleFunc("/api/updates/plan", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleGetUpdatePlan)))
	r.mux.HandleFunc("/api/updates/history", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleListUpdateHistory)))
	r.mux.HandleFunc("/api/updates/history/entry", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, updateHandlers.HandleGetUpdateHistoryEntry)))

	// Infrastructure update detection routes (Docker containers, packages, etc.)
	infraUpdateHandlers := NewUpdateDetectionHandlers(r.monitor)
	r.mux.HandleFunc("/api/infra-updates", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, infraUpdateHandlers.HandleGetInfraUpdates)))
	r.mux.HandleFunc("/api/infra-updates/summary", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, infraUpdateHandlers.HandleGetInfraUpdatesSummary)))
	r.mux.HandleFunc("/api/infra-updates/check", RequireAuth(r.config, RequireScope(config.ScopeMonitoringWrite, infraUpdateHandlers.HandleTriggerInfraUpdateCheck)))
	r.mux.HandleFunc("/api/infra-updates/host/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, func(w http.ResponseWriter, req *http.Request) {
		// Extract host ID from path: /api/infra-updates/host/{hostId}
		hostID := strings.TrimPrefix(req.URL.Path, "/api/infra-updates/host/")
		hostID = strings.TrimSuffix(hostID, "/")
		if hostID == "" {
			writeErrorResponse(w, http.StatusBadRequest, "missing_host_id", "Host ID is required", nil)
			return
		}
		infraUpdateHandlers.HandleGetInfraUpdatesForHost(w, req, hostID)
	})))
	r.mux.HandleFunc("/api/infra-updates/", RequireAuth(r.config, RequireScope(config.ScopeMonitoringRead, func(w http.ResponseWriter, req *http.Request) {
		// Extract resource ID from path: /api/infra-updates/{resourceId}
		resourceID := strings.TrimPrefix(req.URL.Path, "/api/infra-updates/")
		resourceID = strings.TrimSuffix(resourceID, "/")
		if resourceID == "" || resourceID == "summary" || resourceID == "check" || strings.HasPrefix(resourceID, "host/") {
			// Let specific handlers deal with these
			http.NotFound(w, req)
			return
		}
		infraUpdateHandlers.HandleGetInfraUpdateForResource(w, req, resourceID)
	})))

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
	r.mux.HandleFunc("/api/security/validate-bootstrap-token", r.handleValidateBootstrapToken)

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

	// Config Profile Routes - Protected by Admin Auth and Pro License
	// r.configProfileHandler.ServeHTTP implements http.Handler, so we wrap it
	r.mux.Handle("/api/admin/profiles/", RequireAdmin(r.config, RequireLicenseFeature(r.licenseHandlers, license.FeatureAgentProfiles, func(w http.ResponseWriter, req *http.Request) {
		http.StripPrefix("/api/admin/profiles", r.configProfileHandler).ServeHTTP(w, req)
	})))

	// System settings routes
	r.mux.HandleFunc("/api/config/system", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.configHandlers.HandleGetSystemSettings))(w, req)
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

	// Registration token routes removed - feature deprecated

	// License routes (Pulse Pro)
	r.mux.HandleFunc("/api/license/status", RequireAdmin(r.config, r.licenseHandlers.HandleLicenseStatus))
	r.mux.HandleFunc("/api/license/features", RequireAuth(r.config, r.licenseHandlers.HandleLicenseFeatures))
	r.mux.HandleFunc("/api/license/activate", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.licenseHandlers.HandleActivateLicense)))
	r.mux.HandleFunc("/api/license/clear", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.licenseHandlers.HandleClearLicense)))

	// Audit log routes (Enterprise feature)
	auditHandlers := NewAuditHandlers()
	r.mux.HandleFunc("GET /api/audit", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, RequireLicenseFeature(r.licenseHandlers, license.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleListAuditEvents))))
	r.mux.HandleFunc("GET /api/audit/", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, RequireLicenseFeature(r.licenseHandlers, license.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleListAuditEvents))))
	r.mux.HandleFunc("GET /api/audit/{id}/verify", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceAuditLogs, RequireLicenseFeature(r.licenseHandlers, license.FeatureAuditLogging, RequireScope(config.ScopeSettingsRead, auditHandlers.HandleVerifyAuditEvent))))

	// RBAC routes (Phase 2 - Enterprise feature)
	r.mux.HandleFunc("/api/admin/roles", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, license.FeatureRBAC, rbacHandlers.HandleRoles)))
	r.mux.HandleFunc("/api/admin/roles/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, license.FeatureRBAC, rbacHandlers.HandleRoles)))
	r.mux.HandleFunc("/api/admin/users", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, license.FeatureRBAC, rbacHandlers.HandleGetUsers)))
	r.mux.HandleFunc("/api/admin/users/", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, RequireLicenseFeature(r.licenseHandlers, license.FeatureRBAC, rbacHandlers.HandleUserRoleActions)))

	// Advanced Reporting routes
	r.mux.HandleFunc("/api/admin/reports/generate", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceNodes, RequireLicenseFeature(r.licenseHandlers, license.FeatureAdvancedReporting, RequireScope(config.ScopeSettingsRead, r.reportingHandlers.HandleGenerateReport))))

	// Audit Webhook routes
	r.mux.HandleFunc("/api/admin/webhooks/audit", RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceAuditLogs, RequireLicenseFeature(r.licenseHandlers, license.FeatureAuditLogging, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			RequireScope(config.ScopeSettingsRead, auditHandlers.HandleGetWebhooks)(w, req)
		} else {
			RequireScope(config.ScopeSettingsWrite, auditHandlers.HandleUpdateWebhooks)(w, req)
		}
	})))

	// Security routes
	r.mux.HandleFunc("/api/security/change-password", r.handleChangePassword)
	r.mux.HandleFunc("/api/logout", r.handleLogout)
	r.mux.HandleFunc("/api/login", r.handleLogin)
	r.mux.HandleFunc("/api/security/reset-lockout", r.handleResetLockout)
	r.mux.HandleFunc("/api/security/oidc", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.handleOIDCConfig)))
	r.mux.HandleFunc("/api/oidc/login", r.handleOIDCLogin)
	r.mux.HandleFunc(config.DefaultOIDCCallbackPath, r.handleOIDCCallback)
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
			isAuthenticated := false
			var tokenScopes []string
			if cookie, err := req.Cookie("pulse_session"); err == nil && cookie.Value != "" && ValidateSession(cookie.Value) {
				isAuthenticated = true
			} else if token := strings.TrimSpace(req.Header.Get("X-API-Token")); token != "" {
				if record, ok := r.config.ValidateAPIToken(token); ok {
					isAuthenticated = true
					tokenScopes = record.Scopes
				}
			} else if token := req.URL.Query().Get("token"); token != "" {
				// Also check URL query param (used for kiosk mode)
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

			hasValidAPIToken := validateAPIToken(req.Header.Get("X-API-Token"))

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

			hasValidAPIToken := validateAPIToken(req.Header.Get("X-API-Token"))

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
	r.mux.HandleFunc("/api/agent-install-command", RequireAuth(r.config, r.configHandlers.HandleAgentInstallCommand))

	// Auto-register route for setup scripts
	r.mux.HandleFunc("/api/auto-register", r.configHandlers.HandleAutoRegister)
	// Discovery endpoint
	r.mux.HandleFunc("/api/discover", RequireAuth(r.config, r.configHandlers.HandleDiscoverServers))

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
	r.mux.HandleFunc("/api/alerts/", RequireAuth(r.config, r.alertHandlers.HandleAlerts))

	// Notification routes
	r.mux.HandleFunc("/api/notifications/", RequireAdmin(r.config, r.notificationHandlers.HandleNotifications))

	// Notification queue/DLQ routes
	// Security tokens are handled later in the setup with RBAC
	r.mux.HandleFunc("/api/notifications/dlq", RequireAdmin(r.config, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			r.notificationQueueHandlers.GetDLQ(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	r.mux.HandleFunc("/api/notifications/queue/stats", RequireAdmin(r.config, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			r.notificationQueueHandlers.GetQueueStats(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	r.mux.HandleFunc("/api/notifications/dlq/retry", RequireAdmin(r.config, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost {
			r.notificationQueueHandlers.RetryDLQItem(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	r.mux.HandleFunc("/api/notifications/dlq/delete", RequireAdmin(r.config, func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodPost || req.Method == http.MethodDelete {
			r.notificationQueueHandlers.DeleteDLQItem(w, req)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// System settings and API token management
	r.systemSettingsHandler = NewSystemSettingsHandler(r.config, r.persistence, r.wsHub, r.mtMonitor, r.monitor, r.reloadSystemSettings, r.reloadFunc)
	r.mux.HandleFunc("/api/system/settings", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.systemSettingsHandler.HandleGetSystemSettings)))
	r.mux.HandleFunc("/api/system/settings/update", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.systemSettingsHandler.HandleUpdateSystemSettings)))
	r.mux.HandleFunc("/api/system/ssh-config", r.handleSSHConfig)
	r.mux.HandleFunc("/api/system/verify-temperature-ssh", r.handleVerifyTemperatureSSH)
	// Old API token endpoints removed - now using /api/security/regenerate-token

	// Agent execution server for AI tool use
	r.agentExecServer = agentexec.NewServer(func(token string) bool {
		// Validate agent tokens using the API tokens system
		if r.config == nil {
			return false
		}
		// First check the new API tokens system
		if _, ok := r.config.ValidateAPIToken(token); ok {
			return true
		}
		// Fall back to legacy single token if set
		if r.config.APIToken != "" {
			return auth.CompareAPIToken(token, r.config.APIToken)
		}
		return false
	})

	// AI settings endpoints
	r.aiSettingsHandler = NewAISettingsHandler(r.multiTenant, r.mtMonitor, r.agentExecServer)
	// Inject state provider so AI has access to full infrastructure context (VMs, containers, IPs)
	if r.monitor != nil {
		r.aiSettingsHandler.SetStateProvider(r.monitor)
		// Inject alert provider so AI has awareness of current alerts
		// Also inject alert resolver so AI Patrol can autonomously resolve alerts when issues are fixed
		if alertManager := r.monitor.GetAlertManager(); alertManager != nil {
			alertAdapter := ai.NewAlertManagerAdapter(alertManager)
			r.aiSettingsHandler.SetAlertProvider(alertAdapter)
			r.aiSettingsHandler.SetAlertResolver(alertAdapter)
		}
		if incidentStore := r.monitor.GetIncidentStore(); incidentStore != nil {
			r.aiSettingsHandler.SetIncidentStore(incidentStore)
		}
	}
	// Inject unified resource provider for Phase 2 AI context (cleaner, deduplicated view)
	if r.resourceHandlers != nil {
		r.aiSettingsHandler.SetResourceProvider(r.resourceHandlers.Store())
	}
	// Inject metadata provider for AI URL discovery feature
	// This allows AI to set resource URLs when it discovers web services
	metadataProvider := NewMetadataProvider(
		guestMetadataHandler.Store(),
		dockerMetadataHandler.Store(),
		hostMetadataHandler.Store(),
	)
	r.aiSettingsHandler.SetMetadataProvider(metadataProvider)

	// AI chat handler
	r.aiHandler = NewAIHandler(r.multiTenant, r.mtMonitor, r.agentExecServer)
	// Wire license checker for Pro feature gating (AI Patrol, Alert Analysis, Auto-Fix)
	r.aiSettingsHandler.SetLicenseHandlers(r.licenseHandlers)
	// Wire model change callback to restart AI chat service when model is changed
	r.aiSettingsHandler.SetOnModelChange(func() {
		r.RestartAIChat(context.Background())
	})
	// Wire control settings change callback to update MCP tool visibility
	r.aiSettingsHandler.SetOnControlSettingsChange(func() {
		if r.aiHandler != nil {
			ctx := context.Background()
			if svc := r.aiHandler.GetService(ctx); svc != nil {
				cfg := r.aiHandler.GetAIConfig(ctx)
				if cfg != nil {
					svc.UpdateControlSettings(cfg)
					log.Info().Str("control_level", cfg.GetControlLevel()).Msg("Updated AI control settings")
				}
			}
		}
	})
	// Wire AI handler to profile handler for AI-assisted suggestions
	r.configProfileHandler.SetAIHandler(r.aiHandler)
	// Wire license checker for alert manager Pro features (Update Alerts)
	if r.monitor != nil {
		alertMgr := r.monitor.GetAlertManager()
		if alertMgr != nil {
			licSvc := r.licenseHandlers.Service(context.Background())
			alertMgr.SetLicenseChecker(func(feature string) bool {
				return licSvc.HasFeature(feature)
			})
		}
	}
	r.mux.HandleFunc("/api/settings/ai", RequirePermission(r.config, r.authorizer, auth.ActionRead, auth.ResourceSettings, RequireScope(config.ScopeSettingsRead, r.aiSettingsHandler.HandleGetAISettings)))
	r.mux.HandleFunc("/api/settings/ai/update", RequirePermission(r.config, r.authorizer, auth.ActionWrite, auth.ResourceSettings, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleUpdateAISettings)))
	r.mux.HandleFunc("/api/ai/test", RequirePermission(r.config, r.authorizer, auth.ActionWrite, auth.ResourceSettings, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleTestAIConnection)))
	r.mux.HandleFunc("/api/ai/test/{provider}", RequirePermission(r.config, r.authorizer, auth.ActionWrite, auth.ResourceSettings, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleTestProvider)))
	r.mux.HandleFunc("/api/ai/models", RequireAuth(r.config, r.aiSettingsHandler.HandleListModels))
	r.mux.HandleFunc("/api/ai/execute", RequireAuth(r.config, r.aiSettingsHandler.HandleExecute))
	r.mux.HandleFunc("/api/ai/execute/stream", RequireAuth(r.config, r.aiSettingsHandler.HandleExecuteStream))
	r.mux.HandleFunc("/api/ai/kubernetes/analyze", RequireAuth(r.config, RequireLicenseFeature(r.licenseHandlers, license.FeatureKubernetesAI, r.aiSettingsHandler.HandleAnalyzeKubernetesCluster)))
	r.mux.HandleFunc("/api/ai/investigate-alert", RequireAuth(r.config, RequireLicenseFeature(r.licenseHandlers, license.FeatureAIAlerts, r.aiSettingsHandler.HandleInvestigateAlert)))

	r.mux.HandleFunc("/api/ai/run-command", RequireAuth(r.config, r.aiSettingsHandler.HandleRunCommand))
	r.mux.HandleFunc("/api/ai/knowledge", RequireAuth(r.config, r.aiSettingsHandler.HandleGetGuestKnowledge))
	r.mux.HandleFunc("/api/ai/knowledge/save", RequireAuth(r.config, r.aiSettingsHandler.HandleSaveGuestNote))
	r.mux.HandleFunc("/api/ai/knowledge/delete", RequireAuth(r.config, r.aiSettingsHandler.HandleDeleteGuestNote))
	r.mux.HandleFunc("/api/ai/knowledge/export", RequireAuth(r.config, r.aiSettingsHandler.HandleExportGuestKnowledge))
	r.mux.HandleFunc("/api/ai/knowledge/import", RequireAuth(r.config, r.aiSettingsHandler.HandleImportGuestKnowledge))
	r.mux.HandleFunc("/api/ai/knowledge/clear", RequireAuth(r.config, r.aiSettingsHandler.HandleClearGuestKnowledge))
	r.mux.HandleFunc("/api/ai/debug/context", RequireAdmin(r.config, r.aiSettingsHandler.HandleDebugContext))
	r.mux.HandleFunc("/api/ai/agents", RequireAuth(r.config, r.aiSettingsHandler.HandleGetConnectedAgents))
	r.mux.HandleFunc("/api/ai/cost/summary", RequireAuth(r.config, r.aiSettingsHandler.HandleGetAICostSummary))
	r.mux.HandleFunc("/api/ai/cost/reset", RequireAdmin(r.config, RequireScope(config.ScopeSettingsWrite, r.aiSettingsHandler.HandleResetAICostHistory)))
	r.mux.HandleFunc("/api/ai/cost/export", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, r.aiSettingsHandler.HandleExportAICostHistory)))
	// OAuth endpoints for Claude Pro/Max subscription authentication
	r.mux.HandleFunc("/api/ai/oauth/start", RequireAdmin(r.config, r.aiSettingsHandler.HandleOAuthStart))
	r.mux.HandleFunc("/api/ai/oauth/exchange", RequireAdmin(r.config, r.aiSettingsHandler.HandleOAuthExchange)) // Manual code input
	r.mux.HandleFunc("/api/ai/oauth/callback", r.aiSettingsHandler.HandleOAuthCallback)                         // Public - receives redirect from Anthropic
	r.mux.HandleFunc("/api/ai/oauth/disconnect", RequireAdmin(r.config, r.aiSettingsHandler.HandleOAuthDisconnect))

	// AI Patrol routes for background monitoring
	// Note: Status remains accessible so UI can show license/upgrade state
	// Read endpoints (findings, history, runs) return redacted preview data when unlicensed
	// Mutation endpoints (run, acknowledge, dismiss, etc.) return 402 to prevent unauthorized actions
	r.mux.HandleFunc("/api/ai/patrol/status", RequireAuth(r.config, r.aiSettingsHandler.HandleGetPatrolStatus))
	r.mux.HandleFunc("/api/ai/patrol/stream", RequireAuth(r.config, RequireLicenseFeature(r.licenseHandlers, license.FeatureAIPatrol, r.aiSettingsHandler.HandlePatrolStream)))
	r.mux.HandleFunc("/api/ai/patrol/findings", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetPatrolFindings(w, req)
		case http.MethodDelete:
			// Clear all findings - doesn't require Pro license so users can clean up accumulated findings
			r.aiSettingsHandler.HandleClearAllFindings(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	r.mux.HandleFunc("/api/ai/patrol/history", RequireAuth(r.config, r.aiSettingsHandler.HandleGetFindingsHistory))
	r.mux.HandleFunc("/api/ai/patrol/run", RequireAdmin(r.config, RequireLicenseFeature(r.licenseHandlers, license.FeatureAIPatrol, r.aiSettingsHandler.HandleForcePatrol)))
	r.mux.HandleFunc("/api/ai/patrol/acknowledge", RequireAuth(r.config, RequireLicenseFeature(r.licenseHandlers, license.FeatureAIPatrol, r.aiSettingsHandler.HandleAcknowledgeFinding)))
	// Dismiss and resolve don't require Pro license - users should be able to clear findings they can see
	// This is especially important for users who accumulated findings before fixing the patrol-without-AI bug
	r.mux.HandleFunc("/api/ai/patrol/dismiss", RequireAuth(r.config, r.aiSettingsHandler.HandleDismissFinding))
	r.mux.HandleFunc("/api/ai/patrol/suppress", RequireAuth(r.config, RequireLicenseFeature(r.licenseHandlers, license.FeatureAIPatrol, r.aiSettingsHandler.HandleSuppressFinding)))
	r.mux.HandleFunc("/api/ai/patrol/snooze", RequireAuth(r.config, RequireLicenseFeature(r.licenseHandlers, license.FeatureAIPatrol, r.aiSettingsHandler.HandleSnoozeFinding)))
	r.mux.HandleFunc("/api/ai/patrol/resolve", RequireAuth(r.config, r.aiSettingsHandler.HandleResolveFinding))
	r.mux.HandleFunc("/api/ai/patrol/runs", RequireAuth(r.config, r.aiSettingsHandler.HandleGetPatrolRunHistory))
	// Suppression rules management (also Pro-only since they control LLM behavior)
	// GET returns empty array for unlicensed, POST returns 402
	r.mux.HandleFunc("/api/ai/patrol/suppressions", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			// GET: return empty array if unlicensed
			if err := r.licenseHandlers.Service(req.Context()).RequireFeature(license.FeatureAIPatrol); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-License-Required", "true")
				w.Header().Set("X-License-Feature", license.FeatureAIPatrol)
				w.Write([]byte("[]"))
				return
			}
			r.aiSettingsHandler.HandleGetSuppressionRules(w, req)
		case http.MethodPost:
			// POST: return 402 if unlicensed
			if err := r.licenseHandlers.Service(req.Context()).RequireFeature(license.FeatureAIPatrol); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusPaymentRequired)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":       "license_required",
					"message":     err.Error(),
					"feature":     license.FeatureAIPatrol,
					"upgrade_url": "https://pulserelay.pro/",
				})
				return
			}
			r.aiSettingsHandler.HandleAddSuppressionRule(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	r.mux.HandleFunc("/api/ai/patrol/suppressions/", RequireAuth(r.config, RequireLicenseFeature(r.licenseHandlers, license.FeatureAIPatrol, r.aiSettingsHandler.HandleDeleteSuppressionRule)))
	r.mux.HandleFunc("/api/ai/patrol/dismissed", RequireAuth(r.config, LicenseGatedEmptyResponse(r.licenseHandlers, license.FeatureAIPatrol, r.aiSettingsHandler.HandleGetDismissedFindings)))

	// AI Intelligence endpoints - expose learned patterns, correlations, and predictions
	// Unified intelligence endpoint - aggregates all AI subsystems into a single view
	r.mux.HandleFunc("/api/ai/intelligence", RequireAuth(r.config, r.aiSettingsHandler.HandleGetIntelligence))
	// Individual sub-endpoints for specific intelligence layers
	r.mux.HandleFunc("/api/ai/intelligence/patterns", RequireAuth(r.config, r.aiSettingsHandler.HandleGetPatterns))
	r.mux.HandleFunc("/api/ai/intelligence/predictions", RequireAuth(r.config, r.aiSettingsHandler.HandleGetPredictions))
	r.mux.HandleFunc("/api/ai/intelligence/correlations", RequireAuth(r.config, r.aiSettingsHandler.HandleGetCorrelations))
	r.mux.HandleFunc("/api/ai/intelligence/changes", RequireAuth(r.config, r.aiSettingsHandler.HandleGetRecentChanges))
	r.mux.HandleFunc("/api/ai/intelligence/baselines", RequireAuth(r.config, r.aiSettingsHandler.HandleGetBaselines))
	r.mux.HandleFunc("/api/ai/intelligence/remediations", RequireAuth(r.config, r.aiSettingsHandler.HandleGetRemediations))
	r.mux.HandleFunc("/api/ai/intelligence/anomalies", RequireAuth(r.config, r.aiSettingsHandler.HandleGetAnomalies))
	r.mux.HandleFunc("/api/ai/intelligence/learning", RequireAuth(r.config, r.aiSettingsHandler.HandleGetLearningStatus))

	// AI Chat Sessions - sync across devices (legacy endpoints)
	r.mux.HandleFunc("/api/ai/chat/sessions", RequireAuth(r.config, r.aiSettingsHandler.HandleListAIChatSessions))
	r.mux.HandleFunc("/api/ai/chat/sessions/", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiSettingsHandler.HandleGetAIChatSession(w, req)
		case http.MethodPut:
			r.aiSettingsHandler.HandleSaveAIChatSession(w, req)
		case http.MethodDelete:
			r.aiSettingsHandler.HandleDeleteAIChatSession(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// AI chat endpoints
	r.mux.HandleFunc("/api/ai/status", RequireAuth(r.config, r.aiHandler.HandleStatus))
	r.mux.HandleFunc("/api/ai/chat", RequireAuth(r.config, r.aiHandler.HandleChat))
	r.mux.HandleFunc("/api/ai/sessions", RequireAuth(r.config, func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			r.aiHandler.HandleSessions(w, req)
		case http.MethodPost:
			r.aiHandler.HandleCreateSession(w, req)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	r.mux.HandleFunc("/api/ai/sessions/", RequireAuth(r.config, r.routeAISessions))

	// AI approval endpoints - for command approval workflow
	r.mux.HandleFunc("/api/ai/approvals", RequireAuth(r.config, r.aiSettingsHandler.HandleListApprovals))
	r.mux.HandleFunc("/api/ai/approvals/", RequireAuth(r.config, r.routeApprovals))

	// AI question endpoints
	r.mux.HandleFunc("/api/ai/question/", RequireAuth(r.config, r.routeQuestions))

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

	// Simple stats page
	r.mux.HandleFunc("/simple-stats", r.handleSimpleStats)

	// Note: Frontend handler is handled manually in ServeHTTP to prevent redirect issues
	// See issue #334 - ServeMux redirects empty path to "./" which breaks reverse proxies

}

// routeAISessions routes session-specific AI chat requests
func (r *Router) routeAISessions(w http.ResponseWriter, req *http.Request) {
	// Extract session ID from path: /api/ai/sessions/{id}[/messages|/abort|/summarize|/diff|/fork|/revert|/unrevert]
	path := strings.TrimPrefix(req.URL.Path, "/api/ai/sessions/")
	parts := strings.SplitN(path, "/", 2)
	sessionID := parts[0]

	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Check if there's a sub-resource
	if len(parts) > 1 {
		switch parts[1] {
		case "messages":
			r.aiHandler.HandleMessages(w, req, sessionID)
		case "abort":
			r.aiHandler.HandleAbort(w, req, sessionID)
		case "summarize":
			r.aiHandler.HandleSummarize(w, req, sessionID)
		case "diff":
			r.aiHandler.HandleDiff(w, req, sessionID)
		case "fork":
			r.aiHandler.HandleFork(w, req, sessionID)
		case "revert":
			r.aiHandler.HandleRevert(w, req, sessionID)
		case "unrevert":
			r.aiHandler.HandleUnrevert(w, req, sessionID)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
		return
	}

	// Handle session-level operations
	switch req.Method {
	case http.MethodDelete:
		r.aiHandler.HandleDeleteSession(w, req, sessionID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// routeApprovals routes approval-specific requests
func (r *Router) routeApprovals(w http.ResponseWriter, req *http.Request) {
	// Extract approval ID and action from path: /api/ai/approvals/{id}[/approve|/deny]
	path := strings.TrimPrefix(req.URL.Path, "/api/ai/approvals/")
	parts := strings.SplitN(path, "/", 2)

	if parts[0] == "" {
		http.Error(w, "Approval ID required", http.StatusBadRequest)
		return
	}

	// Check if there's an action
	if len(parts) > 1 {
		switch parts[1] {
		case "approve":
			r.aiSettingsHandler.HandleApproveCommand(w, req)
		case "deny":
			r.aiSettingsHandler.HandleDenyCommand(w, req)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
		return
	}

	// Handle approval-level operations (GET specific approval)
	switch req.Method {
	case http.MethodGet:
		r.aiSettingsHandler.HandleGetApproval(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// routeQuestions routes question-specific requests
func (r *Router) routeQuestions(w http.ResponseWriter, req *http.Request) {
	// Extract question ID and action from path: /api/ai/question/{id}/answer
	path := strings.TrimPrefix(req.URL.Path, "/api/ai/question/")
	parts := strings.SplitN(path, "/", 2)

	if parts[0] == "" {
		http.Error(w, "Question ID required", http.StatusBadRequest)
		return
	}

	questionID := parts[0]

	// Check if there's an action
	if len(parts) > 1 && parts[1] == "answer" {
		if req.Method == http.MethodPost {
			r.aiHandler.HandleAnswerQuestion(w, req, questionID)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

// handleAgentWebSocket handles WebSocket connections from agents for AI command execution
func (r *Router) handleAgentWebSocket(w http.ResponseWriter, req *http.Request) {
	if r.agentExecServer == nil {
		http.Error(w, "Agent execution not available", http.StatusServiceUnavailable)
		return
	}
	r.agentExecServer.HandleWebSocket(w, req)
}

func (r *Router) handleVerifyTemperatureSSH(w http.ResponseWriter, req *http.Request) {
	if r.configHandlers == nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	if token := extractSetupToken(req); token != "" {
		if r.configHandlers.ValidateSetupToken(token) {
			r.configHandlers.HandleVerifyTemperatureSSH(w, req)
			return
		}
	}

	if CheckAuth(r.config, w, req) {
		r.configHandlers.HandleVerifyTemperatureSSH(w, req)
		return
	}

	log.Warn().
		Str("ip", req.RemoteAddr).
		Str("path", req.URL.Path).
		Str("method", req.Method).
		Msg("Unauthorized access attempt (verify-temperature-ssh)")

	if strings.HasPrefix(req.URL.Path, "/api/") || strings.Contains(req.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"Authentication required"}`))
	} else {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

// handleSSHConfig handles SSH config writes with setup token or API auth
func (r *Router) handleSSHConfig(w http.ResponseWriter, req *http.Request) {
	if r.systemSettingsHandler == nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Check setup token first (for setup scripts)
	if token := extractSetupToken(req); token != "" {
		if r.configHandlers != nil && r.configHandlers.ValidateSetupToken(token) {
			r.systemSettingsHandler.HandleSSHConfig(w, req)
			return
		}
	}

	// Fall back to standard API authentication
	if CheckAuth(r.config, w, req) {
		r.systemSettingsHandler.HandleSSHConfig(w, req)
		return
	}

	log.Warn().
		Str("ip", req.RemoteAddr).
		Str("path", req.URL.Path).
		Str("method", req.Method).
		Msg("Unauthorized access attempt (ssh-config)")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"Authentication required"}`))
}

func extractSetupToken(req *http.Request) string {
	if token := strings.TrimSpace(req.Header.Get("X-Setup-Token")); token != "" {
		return token
	}
	if token := extractBearerToken(req.Header.Get("Authorization")); token != "" {
		return token
	}
	if token := strings.TrimSpace(req.URL.Query().Get("auth_token")); token != "" {
		return token
	}
	return ""
}

func extractBearerToken(header string) string {
	if header == "" {
		return ""
	}

	trimmed := strings.TrimSpace(header)
	if len(trimmed) < 7 {
		return ""
	}

	if strings.HasPrefix(strings.ToLower(trimmed), "bearer ") {
		return strings.TrimSpace(trimmed[7:])
	}

	return ""
}

// Handler returns the router wrapped with middleware.
func (r *Router) Handler() http.Handler {
	if r.wrapped != nil {
		return r.wrapped
	}
	return r
}

// SetMonitor updates the router and associated handlers with a new monitor instance.
func (r *Router) SetMonitor(m *monitoring.Monitor) {
	r.monitor = m
	if r.alertHandlers != nil {
		r.alertHandlers.SetMonitor(NewAlertMonitorWrapper(m))
	}
	if r.configHandlers != nil {
		r.configHandlers.SetMonitor(m)
	}
	if r.notificationHandlers != nil {
		r.notificationHandlers.SetMonitor(NewNotificationMonitorWrapper(m))
	}
	if r.dockerAgentHandlers != nil {
		r.dockerAgentHandlers.SetMonitor(m)
	}
	if r.hostAgentHandlers != nil {
		r.hostAgentHandlers.SetMonitor(m)
	}
	if r.systemSettingsHandler != nil {
		r.systemSettingsHandler.SetMonitor(m)
	}
	if m != nil {
		if url := strings.TrimSpace(r.config.PublicURL); url != "" {
			if mgr := m.GetNotificationManager(); mgr != nil {
				mgr.SetPublicURL(url)
			}
		}
		// Inject resource store for polling optimization
		if r.resourceHandlers != nil {
			log.Debug().Msg("[Router] Injecting resource store into monitor")
			m.SetResourceStore(r.resourceHandlers.Store())
			// Also set state provider for on-demand resource population
			r.resourceHandlers.SetStateProvider(m)
		} else {
			log.Warn().Msg("[Router] resourceHandlers is nil, cannot inject resource store")
		}

		// Set up Docker detector for automatic Docker detection in LXC containers
		if r.agentExecServer != nil {
			// Create a command executor function that wraps the agent exec server
			execFunc := func(ctx context.Context, hostname string, command string, timeout int) (string, int, error) {
				agentID, found := r.agentExecServer.GetAgentForHost(hostname)
				if !found {
					return "", -1, fmt.Errorf("no agent connected for host %s", hostname)
				}
				result, err := r.agentExecServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
					RequestID: fmt.Sprintf("docker-check-%d", time.Now().UnixNano()),
					Command:   command,
					Timeout:   timeout,
				})
				if err != nil {
					return "", -1, err
				}
				return result.Stdout + result.Stderr, result.ExitCode, nil
			}

			checker := monitoring.NewAgentDockerChecker(execFunc)
			m.SetDockerChecker(checker)
			log.Info().Msg("[Router] Docker detector configured for automatic LXC Docker detection")
		}
	}
}

// SetConfig refreshes the configuration reference used by the router and dependent handlers.
func (r *Router) SetConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}

	config.Mu.Lock()
	defer config.Mu.Unlock()

	if r.config == nil {
		r.config = cfg
	} else {
		*r.config = *cfg
	}

	if r.configHandlers != nil {
		r.configHandlers.SetConfig(r.config)
	}
	if r.systemSettingsHandler != nil {
		r.systemSettingsHandler.SetConfig(r.config)
	}
}

// StartPatrol starts the AI patrol service for background infrastructure monitoring
func (r *Router) StartPatrol(ctx context.Context) {
	if r.aiSettingsHandler != nil {
		// Connect patrol to user-configured alert thresholds so it warns before alerts fire
		if r.monitor != nil {
			if alertManager := r.monitor.GetAlertManager(); alertManager != nil {
				thresholdAdapter := ai.NewAlertThresholdAdapter(alertManager)
				r.aiSettingsHandler.SetPatrolThresholdProvider(thresholdAdapter)
			}
		}

		// Enable findings persistence (load from disk, auto-save on changes)
		if r.persistence != nil {
			findingsPersistence := ai.NewFindingsPersistenceAdapter(r.persistence)
			if err := r.aiSettingsHandler.SetPatrolFindingsPersistence(findingsPersistence); err != nil {
				log.Error().Err(err).Msg("Failed to initialize AI findings persistence")
			}

			// Enable patrol run history persistence
			historyPersistence := ai.NewPatrolHistoryPersistenceAdapter(r.persistence)
			if err := r.aiSettingsHandler.SetPatrolRunHistoryPersistence(historyPersistence); err != nil {
				log.Error().Err(err).Msg("Failed to initialize AI patrol run history persistence")
			}
		}

		// Connect patrol to metrics history for enriched context (trends, predictions)
		if r.monitor != nil {
			if metricsHistory := r.monitor.GetMetricsHistory(); metricsHistory != nil {
				adapter := ai.NewMetricsHistoryAdapter(metricsHistory)
				if adapter != nil {
					r.aiSettingsHandler.SetMetricsHistoryProvider(adapter)
				}

				// Only initialize baseline learning if AI is enabled
				// This prevents anomaly data from being collected and displayed when AI is disabled
				if r.aiSettingsHandler.IsAIEnabled(context.Background()) {
					// Initialize baseline store for anomaly detection
					// Uses config dir for persistence
					baselineCfg := ai.DefaultBaselineConfig()
					if r.persistence != nil {
						baselineCfg.DataDir = r.persistence.DataDir()
					}
					baselineStore := ai.NewBaselineStore(baselineCfg)
					if baselineStore != nil {
						r.aiSettingsHandler.SetBaselineStore(baselineStore)

						// Start background baseline learning loop
						go r.startBaselineLearning(ctx, baselineStore, metricsHistory)
					}
				}
			}
		}

		// Initialize operational memory (change detection and remediation logging)
		dataDir := ""
		if r.persistence != nil {
			dataDir = r.persistence.DataDir()
		}

		changeDetector := ai.NewChangeDetector(ai.ChangeDetectorConfig{
			MaxChanges: 1000,
			DataDir:    dataDir,
		})
		if changeDetector != nil {
			r.aiSettingsHandler.SetChangeDetector(changeDetector)
		}

		remediationLog := ai.NewRemediationLog(ai.RemediationLogConfig{
			MaxRecords: 500,
			DataDir:    dataDir,
		})
		if remediationLog != nil {
			r.aiSettingsHandler.SetRemediationLog(remediationLog)
		}

		// Only initialize pattern and correlation detectors if AI is enabled
		// This prevents these subsystems from collecting data and displaying findings when AI is disabled
		if r.aiSettingsHandler.IsAIEnabled(context.Background()) {
			// Initialize pattern detector for failure prediction
			patternDetector := ai.NewPatternDetector(ai.PatternDetectorConfig{
				MaxEvents:       5000,
				MinOccurrences:  3,
				PatternWindow:   90 * 24 * time.Hour,
				PredictionLimit: 30 * 24 * time.Hour,
				DataDir:         dataDir,
			})
			if patternDetector != nil {
				r.aiSettingsHandler.SetPatternDetector(patternDetector)

				// Wire alert history to pattern detector for event tracking
				if alertManager := r.monitor.GetAlertManager(); alertManager != nil {
					alertManager.OnAlertHistory(func(alert alerts.Alert) {
						// Convert alert type to trackable event
						patternDetector.RecordFromAlert(alert.ResourceID, alert.Type+"_"+string(alert.Level), alert.StartTime)
					})
					log.Info().Msg("AI Pattern Detector: Wired to alert history for failure prediction")
				}
			}

			// Initialize correlation detector for multi-resource relationships
			correlationDetector := ai.NewCorrelationDetector(ai.CorrelationConfig{
				MaxEvents:         10000,
				CorrelationWindow: 10 * time.Minute,
				MinOccurrences:    3,
				RetentionWindow:   30 * 24 * time.Hour,
				DataDir:           dataDir,
			})
			if correlationDetector != nil {
				r.aiSettingsHandler.SetCorrelationDetector(correlationDetector)

				// Wire alert history to correlation detector
				if alertManager := r.monitor.GetAlertManager(); alertManager != nil {
					alertManager.OnAlertHistory(func(alert alerts.Alert) {
						// Record as correlation event
						eventType := ai.CorrelationEventType(ai.CorrelationEventAlert)
						switch alert.Type {
						case "cpu":
							eventType = ai.CorrelationEventHighCPU
						case "memory":
							eventType = ai.CorrelationEventHighMem
						case "disk":
							eventType = ai.CorrelationEventDiskFull
						case "offline", "connectivity":
							eventType = ai.CorrelationEventOffline
						}
						correlationDetector.RecordEvent(ai.CorrelationEvent{
							ResourceID:   alert.ResourceID,
							ResourceName: alert.ResourceName,
							ResourceType: alert.Type,
							EventType:    eventType,
							Timestamp:    alert.StartTime,
							Value:        alert.Value,
						})
					})
					log.Info().Msg("AI Correlation Detector: Wired to alert history for multi-resource analysis")
				}
			}
		}

		r.aiSettingsHandler.StartPatrol(ctx)
	}
}

// StopPatrol stops the AI patrol service
func (r *Router) StopPatrol() {
	if r.aiSettingsHandler != nil {
		r.aiSettingsHandler.StopPatrol()
	}
}

// StartAIChat starts the AI chat service
// This is the new AI backend that supports tool calling and multi-model support
func (r *Router) StartAIChat(ctx context.Context) {
	if r.aiHandler == nil {
		return
	}
	if r.monitor == nil {
		log.Warn().Msg("Cannot start AI chat: monitor not available")
		return
	}

	if err := r.aiHandler.Start(ctx, r.monitor); err != nil {
		log.Error().Err(err).Msg("Failed to start AI chat service")
		return
	}

	// Wire up MCP tool providers so AI can access real data
	r.wireAIChatProviders()

	// Wire up AI patrol if AI is running
	aiCfg := r.aiHandler.GetAIConfig(context.Background())
	if aiCfg != nil && r.aiHandler.IsRunning(context.Background()) {
		service := r.aiHandler.GetService(context.Background())
		if service != nil {
			// Create patrol service - need concrete type for patrol
			chatService, ok := service.(*chat.Service)
			if !ok {
				log.Warn().Msg("AI service is not a *chat.Service, patrol disabled")
			}
			aiPatrol := chat.NewPatrolService(chatService)

			// Wire to existing patrol service
			if r.aiSettingsHandler != nil {
				if patrolSvc := r.aiSettingsHandler.GetAIService(context.Background()).GetPatrolService(); patrolSvc != nil {
					patrolSvc.SetChatPatrol(aiPatrol, true)
					log.Info().Msg("AI patrol integration enabled")
				}
			}
		}
	}
}

// wireAIChatProviders wires up all MCP tool providers for AI chat
func (r *Router) wireAIChatProviders() {
	if r.aiHandler == nil || !r.aiHandler.IsRunning(context.Background()) {
		return
	}

	service := r.aiHandler.GetService(context.Background())
	if service == nil {
		return
	}

	// Wire alert provider
	if r.monitor != nil {
		if alertManager := r.monitor.GetAlertManager(); alertManager != nil {
			alertAdapter := tools.NewAlertManagerMCPAdapter(alertManager)
			if alertAdapter != nil {
				service.SetAlertProvider(alertAdapter)
				log.Debug().Msg("AI chat: Alert provider wired")
			}
		}
	}

	// Wire findings provider from patrol service
	if r.aiSettingsHandler != nil {
		if patrolSvc := r.aiSettingsHandler.GetAIService(context.Background()).GetPatrolService(); patrolSvc != nil {
			if findingsStore := patrolSvc.GetFindings(); findingsStore != nil {
				findingsAdapter := ai.NewFindingsMCPAdapter(findingsStore)
				if findingsAdapter != nil {
					service.SetFindingsProvider(findingsAdapter)
					log.Debug().Msg("AI chat: Findings provider wired")
				}
			}
		}
	}

	if r.persistence != nil {
		// For MCP, we normally use a scoped context or default.
		// Assuming MCP server is tenant-aware or global.
		// If global, we might use background context, but if it receives requests, it should have request context.
		// The MCPAgentProfileManager likely needs refactoring for multi-tenancy too or accepts a helper.
		// For now, let's use Background context as a temporary fix, assuming default tenant.
		manager := NewMCPAgentProfileManager(r.persistence, r.licenseHandlers.Service(context.Background()))
		service.SetAgentProfileManager(manager)
		log.Debug().Msg("AI chat: Agent profile manager wired")
	}

	// Wire storage provider
	if r.monitor != nil {
		storageAdapter := tools.NewStorageMCPAdapter(r.monitor)
		if storageAdapter != nil {
			service.SetStorageProvider(storageAdapter)
			log.Debug().Msg("AI chat: Storage provider wired")
		}
	}

	// Wire backup provider
	if r.monitor != nil {
		backupAdapter := tools.NewBackupMCPAdapter(r.monitor)
		if backupAdapter != nil {
			service.SetBackupProvider(backupAdapter)
			log.Debug().Msg("AI chat: Backup provider wired")
		}
	}

	// Wire disk health provider
	if r.monitor != nil {
		diskHealthAdapter := tools.NewDiskHealthMCPAdapter(r.monitor)
		if diskHealthAdapter != nil {
			service.SetDiskHealthProvider(diskHealthAdapter)
			log.Debug().Msg("AI chat: Disk health provider wired")
		}
	}

	// Wire updates provider for Docker container updates
	if r.monitor != nil {
		updatesAdapter := tools.NewUpdatesMCPAdapter(r.monitor, &updatesConfigWrapper{cfg: r.config})
		if updatesAdapter != nil {
			service.SetUpdatesProvider(updatesAdapter)
			log.Debug().Msg("AI chat: Updates provider wired")
		}
	}

	// Wire metrics history provider
	if r.monitor != nil {
		if metricsHistory := r.monitor.GetMetricsHistory(); metricsHistory != nil {
			metricsAdapter := tools.NewMetricsHistoryMCPAdapter(
				r.monitor,
				&metricsSourceWrapper{history: metricsHistory},
			)
			if metricsAdapter != nil {
				service.SetMetricsHistory(metricsAdapter)
				log.Debug().Msg("AI chat: Metrics history provider wired")
			}
		}
	}

	// Wire baseline provider
	if r.aiSettingsHandler != nil {
		if patrolSvc := r.aiSettingsHandler.GetAIService(context.Background()).GetPatrolService(); patrolSvc != nil {
			if baselineStore := patrolSvc.GetBaselineStore(); baselineStore != nil {
				baselineAdapter := tools.NewBaselineMCPAdapter(&baselineSourceWrapper{store: baselineStore})
				if baselineAdapter != nil {
					service.SetBaselineProvider(baselineAdapter)
					log.Debug().Msg("AI chat: Baseline provider wired")
				}
			}
		}
	}

	// Wire pattern provider
	if r.aiSettingsHandler != nil {
		if patrolSvc := r.aiSettingsHandler.GetAIService(context.Background()).GetPatrolService(); patrolSvc != nil {
			if patternDetector := patrolSvc.GetPatternDetector(); patternDetector != nil {
				patternAdapter := tools.NewPatternMCPAdapter(
					&patternSourceWrapper{detector: patternDetector},
					r.monitor,
				)
				if patternAdapter != nil {
					service.SetPatternProvider(patternAdapter)
					log.Debug().Msg("AI chat: Pattern provider wired")
				}
			}
		}
	}

	// Wire findings manager
	if r.aiSettingsHandler != nil {
		if patrolSvc := r.aiSettingsHandler.GetAIService(context.Background()).GetPatrolService(); patrolSvc != nil {
			findingsManagerAdapter := tools.NewFindingsManagerMCPAdapter(patrolSvc)
			if findingsManagerAdapter != nil {
				service.SetFindingsManager(findingsManagerAdapter)
				log.Debug().Msg("AI chat: Findings manager wired")
			}
		}
	}

	// Wire metadata updater
	if r.aiSettingsHandler != nil {
		metadataAdapter := tools.NewMetadataUpdaterMCPAdapter(r.aiSettingsHandler.GetAIService(context.Background()))
		if metadataAdapter != nil {
			service.SetMetadataUpdater(metadataAdapter)
			log.Debug().Msg("AI chat: Metadata updater wired")
		}
	}

	log.Info().Msg("AI chat MCP tool providers wired")
}

// metricsSourceWrapper wraps monitoring.MetricsHistory to implement tools.MetricsSource
type metricsSourceWrapper struct {
	history *monitoring.MetricsHistory
}

func (w *metricsSourceWrapper) GetGuestMetrics(guestID string, metricType string, duration time.Duration) []tools.RawMetricPoint {
	points := w.history.GetGuestMetrics(guestID, metricType, duration)
	return convertMetricPoints(points)
}

func (w *metricsSourceWrapper) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []tools.RawMetricPoint {
	points := w.history.GetNodeMetrics(nodeID, metricType, duration)
	return convertMetricPoints(points)
}

func (w *metricsSourceWrapper) GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]tools.RawMetricPoint {
	metricsMap := w.history.GetAllGuestMetrics(guestID, duration)
	result := make(map[string][]tools.RawMetricPoint, len(metricsMap))
	for key, points := range metricsMap {
		result[key] = convertMetricPoints(points)
	}
	return result
}

func convertMetricPoints(points []monitoring.MetricPoint) []tools.RawMetricPoint {
	result := make([]tools.RawMetricPoint, len(points))
	for i, p := range points {
		result[i] = tools.RawMetricPoint{
			Value:     p.Value,
			Timestamp: p.Timestamp,
		}
	}
	return result
}

// baselineSourceWrapper wraps baseline.Store to implement tools.BaselineSource
type baselineSourceWrapper struct {
	store *ai.BaselineStore
}

func (w *baselineSourceWrapper) GetBaseline(resourceID, metric string) (mean, stddev float64, sampleCount int, ok bool) {
	if w.store == nil {
		return 0, 0, 0, false
	}
	baseline, found := w.store.GetBaseline(resourceID, metric)
	if !found || baseline == nil {
		return 0, 0, 0, false
	}
	return baseline.Mean, baseline.StdDev, baseline.SampleCount, true
}

func (w *baselineSourceWrapper) GetAllBaselines() map[string]map[string]tools.BaselineData {
	if w.store == nil {
		return nil
	}
	allFlat := w.store.GetAllBaselines()
	if allFlat == nil {
		return nil
	}

	result := make(map[string]map[string]tools.BaselineData)
	for key, flat := range allFlat {
		// key format is "resourceID:metric"
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		resourceID, metric := parts[0], parts[1]

		if result[resourceID] == nil {
			result[resourceID] = make(map[string]tools.BaselineData)
		}
		result[resourceID][metric] = tools.BaselineData{
			Mean:        flat.Mean,
			StdDev:      flat.StdDev,
			SampleCount: flat.Samples,
		}
	}
	return result
}

// patternSourceWrapper wraps patterns.Detector to implement tools.PatternSource
type patternSourceWrapper struct {
	detector *ai.PatternDetector
}

func (w *patternSourceWrapper) GetPatterns() []tools.PatternData {
	if w.detector == nil {
		return nil
	}

	patterns := w.detector.GetPatterns()
	if patterns == nil {
		return nil
	}

	result := make([]tools.PatternData, 0, len(patterns))
	for _, p := range patterns {
		if p == nil {
			continue
		}
		result = append(result, tools.PatternData{
			ResourceID:  p.ResourceID,
			PatternType: string(p.EventType),
			Description: fmt.Sprintf("%s pattern with %d occurrences", p.EventType, p.Occurrences),
			Confidence:  p.Confidence,
			LastSeen:    p.LastOccurrence,
		})
	}
	return result
}

func (w *patternSourceWrapper) GetPredictions() []tools.PredictionData {
	if w.detector == nil {
		return nil
	}

	predictions := w.detector.GetPredictions()
	if predictions == nil {
		return nil
	}

	result := make([]tools.PredictionData, 0, len(predictions))
	for _, p := range predictions {
		result = append(result, tools.PredictionData{
			ResourceID:     p.ResourceID,
			IssueType:      string(p.EventType),
			PredictedTime:  p.PredictedAt,
			Confidence:     p.Confidence,
			Recommendation: p.Basis,
		})
	}
	return result
}

// updatesConfigWrapper wraps config.Config to implement tools.UpdatesConfig
type updatesConfigWrapper struct {
	cfg *config.Config
}

func (w *updatesConfigWrapper) IsDockerUpdateActionsEnabled() bool {
	if w.cfg == nil {
		return true // Default to enabled
	}
	return !w.cfg.DisableDockerUpdateActions
}

// StopAIChat stops the AI chat service
func (r *Router) StopAIChat(ctx context.Context) {
	if r.aiHandler != nil {
		if err := r.aiHandler.Stop(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to stop AI chat service")
		}
	}
}

// RestartAIChat restarts the AI chat service with updated configuration
// Call this when AI settings change that affect the service (e.g., model selection)
func (r *Router) RestartAIChat(ctx context.Context) {
	if r.aiHandler != nil {
		if err := r.aiHandler.Restart(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to restart AI chat service")
		} else {
			log.Info().Msg("AI chat service restarted with new configuration")
		}
	}
}

// startBaselineLearning runs a background loop that learns baselines from metrics history
// This enables anomaly detection by understanding what "normal" looks like for each resource
func (r *Router) startBaselineLearning(ctx context.Context, store *ai.BaselineStore, metricsHistory *monitoring.MetricsHistory) {
	if store == nil || metricsHistory == nil {
		return
	}

	// Learn every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run initial learning after a short delay (allow metrics to accumulate)
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Minute):
		r.learnBaselines(store, metricsHistory)
	}

	log.Info().Msg("Baseline learning loop started")

	for {
		select {
		case <-ctx.Done():
			// Save baselines before exit
			if err := store.Save(); err != nil {
				log.Warn().Err(err).Msg("Failed to save baselines on shutdown")
			}
			log.Info().Msg("Baseline learning loop stopped")
			return
		case <-ticker.C:
			r.learnBaselines(store, metricsHistory)
		}
	}
}

// learnBaselines updates baselines for all resources from metrics history
func (r *Router) learnBaselines(store *ai.BaselineStore, metricsHistory *monitoring.MetricsHistory) {
	if r.monitor == nil {
		return
	}

	state := r.monitor.GetState()
	learningWindow := 7 * 24 * time.Hour // Learn from 7 days of data
	var learned int

	// Learn baselines for nodes
	for _, node := range state.Nodes {
		for _, metric := range []string{"cpu", "memory"} {
			points := metricsHistory.GetNodeMetrics(node.ID, metric, learningWindow)
			if len(points) > 0 {
				baselinePoints := make([]ai.BaselineMetricPoint, len(points))
				for i, p := range points {
					baselinePoints[i] = ai.BaselineMetricPoint{Value: p.Value, Timestamp: p.Timestamp}
				}
				if err := store.Learn(node.ID, "node", metric, baselinePoints); err == nil {
					learned++
				}
			}
		}
	}

	// Learn baselines for VMs
	for _, vm := range state.VMs {
		if vm.Template {
			continue
		}
		for _, metric := range []string{"cpu", "memory", "disk"} {
			points := metricsHistory.GetGuestMetrics(vm.ID, metric, learningWindow)
			if len(points) > 0 {
				baselinePoints := make([]ai.BaselineMetricPoint, len(points))
				for i, p := range points {
					baselinePoints[i] = ai.BaselineMetricPoint{Value: p.Value, Timestamp: p.Timestamp}
				}
				if err := store.Learn(vm.ID, "vm", metric, baselinePoints); err == nil {
					learned++
				}
			}
		}
	}

	// Learn baselines for containers
	for _, ct := range state.Containers {
		if ct.Template {
			continue
		}
		for _, metric := range []string{"cpu", "memory", "disk"} {
			points := metricsHistory.GetGuestMetrics(ct.ID, metric, learningWindow)
			if len(points) > 0 {
				baselinePoints := make([]ai.BaselineMetricPoint, len(points))
				for i, p := range points {
					baselinePoints[i] = ai.BaselineMetricPoint{Value: p.Value, Timestamp: p.Timestamp}
				}
				if err := store.Learn(ct.ID, "container", metric, baselinePoints); err == nil {
					learned++
				}
			}
		}
	}

	// Save after learning
	if err := store.Save(); err != nil {
		log.Warn().Err(err).Msg("Failed to save baselines")
	}

	log.Debug().
		Int("baselines_updated", learned).
		Int("resources", store.ResourceCount()).
		Msg("Baseline learning complete")
}

// GetAlertTriggeredAnalyzer returns the alert-triggered analyzer for wiring into the monitor's alert callback
// This enables AI to analyze specific resources when alerts fire, providing token-efficient real-time insights
func (r *Router) GetAlertTriggeredAnalyzer() *ai.AlertTriggeredAnalyzer {
	if r.aiSettingsHandler != nil {
		return r.aiSettingsHandler.GetAlertTriggeredAnalyzer(context.Background())
	}
	return nil
}

// WireAlertTriggeredAI connects the alert-triggered AI analyzer to the monitor's alert callback
// This should be called after StartPatrol() to ensure the analyzer is initialized
func (r *Router) WireAlertTriggeredAI() {
	analyzer := r.GetAlertTriggeredAnalyzer()
	if analyzer == nil {
		log.Debug().Msg("Alert-triggered AI analyzer not available")
		return
	}

	if r.monitor == nil {
		log.Debug().Msg("Monitor not available for AI alert callback")
		return
	}

	// Wire the analyzer's OnAlertFired method to the monitor's alert callback
	r.monitor.SetAlertTriggeredAICallback(analyzer.OnAlertFired)
	log.Info().Msg("Alert-triggered AI analysis wired to monitor")
}

// reloadSystemSettings loads system settings from disk and caches them
func (r *Router) reloadSystemSettings() {
	r.settingsMu.Lock()
	defer r.settingsMu.Unlock()

	// Load from disk
	if systemSettings, err := r.persistence.LoadSystemSettings(); err == nil && systemSettings != nil {
		r.cachedAllowEmbedding = systemSettings.AllowEmbedding
		r.cachedAllowedOrigins = systemSettings.AllowedEmbedOrigins

		// Update HideLocalLogin so it takes effect immediately without restart
		// BUT respect environment variable override if present
		if !r.config.EnvOverrides["PULSE_AUTH_HIDE_LOCAL_LOGIN"] {
			r.config.HideLocalLogin = systemSettings.HideLocalLogin
		}

		// Update webhook allowed private CIDRs in notification manager
		if r.monitor != nil {
			if nm := r.monitor.GetNotificationManager(); nm != nil {
				if err := nm.UpdateAllowedPrivateCIDRs(systemSettings.WebhookAllowedPrivateCIDRs); err != nil {
					log.Error().Err(err).Msg("Failed to update webhook allowed private CIDRs during settings reload")
				}
			}
		}
	} else {
		// On error, use safe defaults
		r.cachedAllowEmbedding = false
		r.cachedAllowedOrigins = ""
	}
}

// ServeHTTP implements http.Handler
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Prevent path traversal attacks
	// We strictly block ".." to prevent directory traversal
	if strings.Contains(req.URL.Path, "..") {
		// Return 401 for API paths to match expected test behavior
		if strings.HasPrefix(req.URL.Path, "/api/") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		} else {
			http.Error(w, "Invalid path", http.StatusBadRequest)
		}
		log.Warn().
			Str("ip", req.RemoteAddr).
			Str("path", req.URL.Path).
			Msg("Path traversal attempt blocked")
		return
	}

	// Get cached system settings (loaded once at startup, not from disk every request)
	r.capturePublicURLFromRequest(req)
	r.settingsMu.RLock()
	allowEmbedding := r.cachedAllowEmbedding
	allowedEmbedOrigins := r.cachedAllowedOrigins
	r.settingsMu.RUnlock()

	// Apply security headers with embedding configuration
	SecurityHeadersWithConfig(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Add CORS headers if configured
		if r.config.AllowedOrigins != "" {
			reqOrigin := req.Header.Get("Origin")
			allowedOrigin := ""

			if r.config.AllowedOrigins == "*" {
				allowedOrigin = "*"
			} else if reqOrigin != "" {
				// Parse comma-separated origins and check for match
				origins := strings.Split(r.config.AllowedOrigins, ",")
				for _, o := range origins {
					o = strings.TrimSpace(o)
					if o == "" {
						continue
					}
					if o == reqOrigin {
						allowedOrigin = o
						break
					}
				}
			} else {
				// No Origin header (same-origin or direct request)
				// Set to first allowed origin for simple responses, though not strictly required for same-origin
				origins := strings.Split(r.config.AllowedOrigins, ",")
				if len(origins) > 0 {
					allowedOrigin = strings.TrimSpace(origins[0])
				}
			}

			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Token, X-CSRF-Token, X-Setup-Token")
				w.Header().Set("Access-Control-Expose-Headers", "X-CSRF-Token, X-Authenticated-User, X-Auth-Method")
				// Allow credentials when origin is specific (not *)
				if allowedOrigin != "*" {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					// Must add Vary: Origin when Origin is used to decide the response
					w.Header().Add("Vary", "Origin")
				}
			}
		}

		// Handle preflight requests
		if req.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Check if we need authentication
		needsAuth := true
		clientIP := GetClientIP(req)

		// Recovery mechanism: Check if recovery mode is enabled
		recoveryFile := filepath.Join(r.config.DataPath, ".auth_recovery")
		if _, err := os.Stat(recoveryFile); err == nil {
			// Recovery mode is enabled - allow local access only
			log.Debug().
				Str("recovery_file", recoveryFile).
				Str("client_ip", clientIP).
				Str("remote_addr", req.RemoteAddr).
				Str("path", req.URL.Path).
				Bool("file_exists", err == nil).
				Msg("Checking auth recovery mode")
			if isDirectLoopbackRequest(req) {
				log.Warn().
					Str("recovery_file", recoveryFile).
					Str("client_ip", clientIP).
					Msg("AUTH RECOVERY MODE: Allowing local access without authentication")
				// Allow access but add a warning header
				w.Header().Set("X-Auth-Recovery", "true")
				// Recovery mode bypasses auth for localhost
				needsAuth = false
			}
		}

		if needsAuth {
			// Normal authentication check
			// Normalize path to handle double slashes (e.g., //download -> /download)
			// This prevents auth bypass failures when URLs have trailing slashes
			normalizedPath := path.Clean(req.URL.Path)

			// Skip auth for certain public endpoints and static assets
			publicPaths := []string{
				"/api/health",
				"/api/security/status",
				"/api/security/validate-bootstrap-token",
				"/api/security/quick-setup", // Handler does its own auth (bootstrap token or session)
				"/api/version",
				"/api/login", // Add login endpoint as public
				"/api/oidc/login",
				config.DefaultOIDCCallbackPath,
				"/install-docker-agent.sh",       // Docker agent bootstrap script must be public
				"/install-container-agent.sh",    // Container agent bootstrap script must be public
				"/download/pulse-docker-agent",   // Agent binary download should not require auth
				"/install-host-agent.sh",         // Host agent bootstrap script must be public
				"/install-host-agent.ps1",        // Host agent PowerShell script must be public
				"/uninstall-host-agent.sh",       // Host agent uninstall script must be public
				"/uninstall-host-agent.ps1",      // Host agent uninstall script must be public
				"/download/pulse-host-agent",     // Host agent binary download should not require auth
				"/install.sh",                    // Unified agent installer
				"/install.ps1",                   // Unified agent Windows installer
				"/download/pulse-agent",          // Unified agent binary
				"/api/agent/version",             // Agent update checks need to work before auth
				"/api/agent/ws",                  // Agent WebSocket has its own auth via registration
				"/api/server/info",               // Server info for installer script
				"/api/install/install-docker.sh", // Docker turnkey installer
				"/api/ai/oauth/callback",         // OAuth callback from Anthropic for Claude subscription auth
			}

			// Also allow static assets without auth (JS, CSS, etc)
			// These MUST be accessible for the login page to work
			// Frontend routes (non-API, non-download) should also be public
			// because authentication is handled by the frontend after page load
			isFrontendRoute := !strings.HasPrefix(req.URL.Path, "/api/") &&
				!strings.HasPrefix(req.URL.Path, "/ws") &&
				!strings.HasPrefix(req.URL.Path, "/socket.io/") &&
				!strings.HasPrefix(req.URL.Path, "/download/") &&
				req.URL.Path != "/simple-stats" &&
				req.URL.Path != "/install-docker-agent.sh" &&
				req.URL.Path != "/install-container-agent.sh" &&
				req.URL.Path != "/install-host-agent.sh" &&
				req.URL.Path != "/install-host-agent.ps1" &&
				req.URL.Path != "/uninstall-host-agent.sh" &&
				req.URL.Path != "/uninstall-host-agent.ps1" &&
				req.URL.Path != "/install.sh" &&
				req.URL.Path != "/install.ps1"

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

			isPublic := isStaticAsset || isFrontendRoute
			for _, path := range publicPaths {
				if normalizedPath == path {
					isPublic = true
					break
				}
			}

			// Special case: setup-script should be public (uses setup codes for auth)
			if normalizedPath == "/api/setup-script" {
				// The script itself prompts for a setup code
				isPublic = true
			}

			// Allow temperature verification endpoint when a setup token is provided
			if normalizedPath == "/api/system/verify-temperature-ssh" && r.configHandlers != nil {
				if token := extractSetupToken(req); token != "" && r.configHandlers.ValidateSetupToken(token) {
					isPublic = true
				}
			}

			// Allow SSH config endpoint when a setup token is provided
			if normalizedPath == "/api/system/ssh-config" && r.configHandlers != nil {
				if token := extractSetupToken(req); token != "" && r.configHandlers.ValidateSetupToken(token) {
					isPublic = true
				}
			}

			// Auto-register endpoint needs to be public (validates tokens internally)
			// BUT the tokens must be generated by authenticated users via setup-script-url
			if normalizedPath == "/api/auto-register" {
				isPublic = true
			}

			// Dev mode bypass for admin endpoints (disabled by default)
			if adminBypassEnabled() {
				log.Debug().
					Str("path", req.URL.Path).
					Msg("Admin bypass enabled - skipping global auth")
				needsAuth = false
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
		if req.URL.Path == "/api/security/quick-setup" || req.URL.Path == "/api/security/apply-restart" {
			// Quick-setup has its own auth logic (bootstrap token or session validation)
			// so we can skip CSRF here - the handler will reject unauthorized requests
			skipCSRF = true
		}
		// Skip CSRF for setup-script-url endpoint (generates temporary tokens, not a state change)
		if req.URL.Path == "/api/setup-script-url" {
			skipCSRF = true
		}
		// Skip CSRF for bootstrap token validation (used during initial setup before session exists)
		if req.URL.Path == "/api/security/validate-bootstrap-token" {
			skipCSRF = true
		}
		// Skip CSRF for login to avoid blocking re-auth when a stale session cookie exists.
		if req.URL.Path == "/api/login" {
			skipCSRF = true
		}
		if strings.HasPrefix(req.URL.Path, "/api/") && !skipCSRF && !CheckCSRF(w, req) {
			http.Error(w, "CSRF token validation failed", http.StatusForbidden)
			LogAuditEvent("csrf_failure", "", GetClientIP(req), req.URL.Path, false, "Invalid CSRF token")
			return
		}

		// Issue CSRF token for GET requests if session exists but CSRF cookie is missing
		// This ensures the frontend has a token before making POST requests
		if req.Method == "GET" && strings.HasPrefix(req.URL.Path, "/api/") {
			sessionCookie, err := req.Cookie("pulse_session")
			if err == nil && sessionCookie.Value != "" {
				// Check if CSRF cookie exists
				_, csrfErr := req.Cookie("pulse_csrf")
				if csrfErr != nil {
					// Session exists but no CSRF cookie - issue one
					csrfToken := generateCSRFToken(sessionCookie.Value)
					isSecure, sameSitePolicy := getCookieSettings(req)
					http.SetCookie(w, &http.Cookie{
						Name:     "pulse_csrf",
						Value:    csrfToken,
						Path:     "/",
						Secure:   isSecure,
						SameSite: sameSitePolicy,
						MaxAge:   86400,
					})
				}
			}
		}

		// Rate limiting is now handled by UniversalRateLimitMiddleware
		// No need for duplicate rate limiting logic here

		// Log request
		start := time.Now()

		// Fix for issue #334: Custom routing to prevent ServeMux's "./" redirect
		// When accessing without trailing slash, ServeMux redirects to "./" which is wrong
		// We handle routing manually to avoid this issue

		// Check if this is an API or WebSocket route
		log.Debug().Str("path", req.URL.Path).Msg("Routing request")

		if strings.HasPrefix(req.URL.Path, "/api/") ||
			strings.HasPrefix(req.URL.Path, "/ws") ||
			strings.HasPrefix(req.URL.Path, "/socket.io/") ||
			strings.HasPrefix(req.URL.Path, "/download/") ||
			req.URL.Path == "/simple-stats" ||
			req.URL.Path == "/install-docker-agent.sh" ||
			req.URL.Path == "/install-container-agent.sh" ||
			path.Clean(req.URL.Path) == "/install-host-agent.sh" ||
			path.Clean(req.URL.Path) == "/install-host-agent.ps1" ||
			path.Clean(req.URL.Path) == "/uninstall-host-agent.sh" ||
			path.Clean(req.URL.Path) == "/uninstall-host-agent.ps1" ||
			path.Clean(req.URL.Path) == "/install.sh" ||
			path.Clean(req.URL.Path) == "/install.ps1" {
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

func (r *Router) capturePublicURLFromRequest(req *http.Request) {
	if req == nil || r == nil || r.config == nil {
		return
	}

	if !canCapturePublicURL(r.config, req) {
		return
	}

	if r.config.EnvOverrides != nil && r.config.EnvOverrides["publicURL"] {
		return
	}

	peerIP := extractRemoteIP(req.RemoteAddr)
	trustedProxy := isTrustedProxyIP(peerIP)

	rawHost := ""
	if trustedProxy {
		rawHost = firstForwardedValue(req.Header.Get("X-Forwarded-Host"))
	}
	if rawHost == "" {
		rawHost = req.Host
	}
	hostWithPort, hostOnly := sanitizeForwardedHost(rawHost)
	if hostWithPort == "" {
		return
	}
	if isLoopbackHost(hostOnly) {
		return
	}

	rawProto := ""
	if trustedProxy {
		rawProto = firstForwardedValue(req.Header.Get("X-Forwarded-Proto"))
		if rawProto == "" {
			rawProto = firstForwardedValue(req.Header.Get("X-Forwarded-Scheme"))
		}
	}
	scheme := strings.ToLower(strings.TrimSpace(rawProto))
	switch scheme {
	case "https", "http":
		// supported values
	default:
		if req.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	if scheme == "" {
		scheme = "http"
	}

	if _, _, err := net.SplitHostPort(hostWithPort); err != nil {
		if forwardedPort := firstForwardedValue(req.Header.Get("X-Forwarded-Port")); forwardedPort != "" {
			if shouldAppendForwardedPort(forwardedPort, scheme) {
				if strings.Contains(hostWithPort, ":") && !strings.HasPrefix(hostWithPort, "[") {
					hostWithPort = fmt.Sprintf("[%s]", hostWithPort)
				} else if strings.HasPrefix(hostWithPort, "[") && !strings.Contains(hostWithPort, "]") {
					hostWithPort = fmt.Sprintf("[%s]", strings.TrimPrefix(hostWithPort, "["))
				}
				hostWithPort = fmt.Sprintf("%s:%s", hostWithPort, forwardedPort)
			}
		}
	}

	candidate := fmt.Sprintf("%s://%s", scheme, hostWithPort)
	normalizedCandidate := strings.TrimRight(strings.TrimSpace(candidate), "/")

	r.publicURLMu.Lock()
	if r.publicURLDetected {
		r.publicURLMu.Unlock()
		return
	}

	current := strings.TrimRight(strings.TrimSpace(r.config.PublicURL), "/")
	if current != "" && current == normalizedCandidate {
		r.publicURLDetected = true
		r.publicURLMu.Unlock()
		return
	}

	r.config.PublicURL = normalizedCandidate
	r.publicURLDetected = true
	r.publicURLMu.Unlock()

	log.Info().
		Str("publicURL", normalizedCandidate).
		Msg("Detected public URL from inbound request; using for notifications")

	if r.monitor != nil {
		if mgr := r.monitor.GetNotificationManager(); mgr != nil {
			mgr.SetPublicURL(normalizedCandidate)
		}
	}
}

func firstForwardedValue(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Split(header, ",")
	return strings.TrimSpace(parts[0])
}

func sanitizeForwardedHost(raw string) (string, string) {
	host := strings.TrimSpace(raw)
	if host == "" {
		return "", ""
	}

	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimSpace(strings.TrimSuffix(host, "/"))
	if host == "" {
		return "", ""
	}

	hostOnly := host
	if h, _, err := net.SplitHostPort(hostOnly); err == nil {
		hostOnly = h
	}
	hostOnly = strings.Trim(hostOnly, "[]")

	return host, hostOnly
}

func isLoopbackHost(host string) bool {
	if host == "" {
		return true
	}
	lower := strings.ToLower(host)
	if lower == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() {
			return true
		}
	}
	return false
}

func shouldAppendForwardedPort(port, scheme string) bool {
	if port == "" {
		return false
	}
	if _, err := strconv.Atoi(port); err != nil {
		return false
	}
	if scheme == "https" && port == "443" {
		return false
	}
	if scheme == "http" && port == "80" {
		return false
	}
	return true
}

func canCapturePublicURL(cfg *config.Config, req *http.Request) bool {
	if cfg == nil || req == nil {
		return false
	}

	if isDirectLoopbackRequest(req) {
		return true
	}

	return isRequestAuthenticated(cfg, req)
}

func isRequestAuthenticated(cfg *config.Config, req *http.Request) bool {
	if cfg == nil || req == nil {
		return false
	}

	if cfg.ProxyAuthSecret != "" {
		if ok, _, _ := CheckProxyAuth(cfg, req); ok {
			return true
		}
	}

	if cfg.HasAPITokens() {
		if token := strings.TrimSpace(req.Header.Get("X-API-Token")); token != "" {
			if _, ok := cfg.ValidateAPIToken(token); ok {
				return true
			}
		}
		if authHeader := strings.TrimSpace(req.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			if _, ok := cfg.ValidateAPIToken(strings.TrimSpace(authHeader[7:])); ok {
				return true
			}
		}
	}

	if cookie, err := req.Cookie("pulse_session"); err == nil && cookie.Value != "" {
		if ValidateSession(cookie.Value) {
			return true
		}
	}

	if cfg.AuthUser != "" && cfg.AuthPass != "" {
		const prefix = "Basic "
		if authHeader := req.Header.Get("Authorization"); strings.HasPrefix(authHeader, prefix) {
			if decoded, err := base64.StdEncoding.DecodeString(authHeader[len(prefix):]); err == nil {
				if parts := strings.SplitN(string(decoded), ":", 2); len(parts) == 2 {
					if parts[0] == cfg.AuthUser && auth.CheckPasswordHash(parts[1], cfg.AuthPass) {
						return true
					}
				}
			}
		}
	}

	return false
}

// handleHealth handles health check requests
func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := HealthResponse{
		Status:                      "healthy",
		Timestamp:                   time.Now().Unix(),
		Uptime:                      time.Since(r.monitor.GetStartTime()).Seconds(),
		ProxyInstallScriptAvailable: true,
		DevModeSSH:                  os.Getenv("PULSE_DEV_ALLOW_CONTAINER_SSH") == "true",
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write health response")
	}
}

// handleSchedulerHealth returns scheduler health status for adaptive polling
func (r *Router) handleSchedulerHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.monitor == nil {
		http.Error(w, "Monitor not available", http.StatusServiceUnavailable)
		return
	}

	health := r.monitor.SchedulerHealth()
	if err := utils.WriteJSONResponse(w, health); err != nil {
		log.Error().Err(err).Msg("Failed to write scheduler health response")
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
			if r.config.HasAPITokens() {
				hashes := make([]string, len(r.config.APITokens))
				for i, t := range r.config.APITokens {
					hashes[i] = t.Hash
				}
				envContent += fmt.Sprintf("API_TOKEN='%s'\n", r.config.PrimaryAPITokenHash())
				envContent += fmt.Sprintf("API_TOKENS='%s'\n", strings.Join(hashes, ","))
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

			if r.config.HasAPITokens() {
				hashes := make([]string, len(r.config.APITokens))
				for i, t := range r.config.APITokens {
					hashes[i] = t.Hash
				}
				envContent += fmt.Sprintf("API_TOKEN='%s'\n", r.config.PrimaryAPITokenHash())
				envContent += fmt.Sprintf("API_TOKENS='%s'\n", strings.Join(hashes, ","))
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
	GetSessionStore().CreateSession(token, 24*time.Hour, userAgent, clientIP, username)

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

// establishOIDCSession creates a session with OIDC token information for refresh token support
func (r *Router) establishOIDCSession(w http.ResponseWriter, req *http.Request, username string, oidcTokens *OIDCTokenInfo) error {
	token := generateSessionToken()
	if token == "" {
		return fmt.Errorf("failed to generate session token")
	}

	userAgent := req.Header.Get("User-Agent")
	clientIP := GetClientIP(req)

	// Create session with OIDC tokens (including username for restart survival)
	GetSessionStore().CreateOIDCSession(token, 24*time.Hour, userAgent, clientIP, username, oidcTokens)

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
		Username   string `json:"username"`
		Password   string `json:"password"`
		RememberMe bool   `json:"rememberMe"`
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

		// Store session persistently with appropriate duration (including username for restart survival)
		userAgent := req.Header.Get("User-Agent")
		sessionDuration := 24 * time.Hour
		if loginReq.RememberMe {
			sessionDuration = 30 * 24 * time.Hour // 30 days
		}
		GetSessionStore().CreateSession(token, sessionDuration, userAgent, clientIP, loginReq.Username)

		// Track session for user (in-memory for fast lookups)
		TrackUserSession(loginReq.Username, token)

		// Generate CSRF token
		csrfToken := generateCSRFToken(token)

		// Get appropriate cookie settings based on proxy detection
		isSecure, sameSitePolicy := getCookieSettings(req)

		// Set cookie MaxAge to match session duration
		cookieMaxAge := int(sessionDuration.Seconds())

		// Set session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "pulse_session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   isSecure,
			SameSite: sameSitePolicy,
			MaxAge:   cookieMaxAge,
		})

		// Set CSRF cookie (not HttpOnly so JS can read it)
		http.SetCookie(w, &http.Cookie{
			Name:     "pulse_csrf",
			Value:    csrfToken,
			Path:     "/",
			Secure:   isSecure,
			SameSite: sameSitePolicy,
			MaxAge:   cookieMaxAge,
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

	if !CheckAuth(r.config, w, req) {
		return
	}
	if !ensureSettingsWriteScope(w, req) {
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
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only GET method is allowed", nil)
		return
	}

	// Use standard auth check (supports both basic auth and API tokens) unless auth is disabled
	if !CheckAuth(r.config, w, req) {
		writeErrorResponse(w, http.StatusUnauthorized, "unauthorized",
			"Authentication required", nil)
		return
	}

	if record := getAPITokenRecordFromRequest(req); record != nil && !record.HasScope(config.ScopeMonitoringRead) {
		respondMissingScope(w, config.ScopeMonitoringRead)
		return
	}

	state := r.monitor.GetState()

	// Also populate the unified resource store (Phase 1 of unified architecture)
	// This runs on every state request to keep resources up-to-date
	if r.resourceHandlers != nil {
		r.resourceHandlers.PopulateFromSnapshot(state)
	}

	frontendState := state.ToFrontend()

	if err := utils.WriteJSONResponse(w, frontendState); err != nil {
		log.Error().Err(err).Msg("Failed to encode state response")
		writeErrorResponse(w, http.StatusInternalServerError, "encoding_error",
			"Failed to encode state data", nil)
	}
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
			Version:       strings.TrimSpace(string(versionBytes)),
			BuildTime:     "development",
			Build:         "development",
			GoVersion:     runtime.Version(),
			Runtime:       runtime.Version(),
			Channel:       "stable",
			IsDocker:      false,
			IsSourceBuild: false,
			IsDevelopment: true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Convert to typed response
	response := VersionResponse{
		Version:        versionInfo.Version,
		BuildTime:      versionInfo.Build,
		Build:          versionInfo.Build,
		GoVersion:      runtime.Version(),
		Runtime:        versionInfo.Runtime,
		Channel:        versionInfo.Channel,
		IsDocker:       versionInfo.IsDocker,
		IsSourceBuild:  versionInfo.IsSourceBuild,
		IsDevelopment:  versionInfo.IsDevelopment,
		DeploymentType: versionInfo.DeploymentType,
	}

	// Detect containerization (LXC/Docker)
	if containerType, err := os.ReadFile("/run/systemd/container"); err == nil {
		response.Containerized = true

		// Try to get container ID from hostname (LXC containers often use CTID as hostname)
		if hostname, err := os.Hostname(); err == nil {
			// For LXC, try to extract numeric ID from hostname or use full hostname
			response.ContainerId = hostname
		}

		// Add container type to deployment type if not already set
		if response.DeploymentType == "" {
			response.DeploymentType = string(containerType)
		}
	}

	// Add cached update info if available
	if cachedUpdate := r.updateManager.GetCachedUpdateInfo(); cachedUpdate != nil {
		response.UpdateAvailable = cachedUpdate.Available
		response.LatestVersion = cachedUpdate.LatestVersion
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleAgentVersion returns the current server version for agent update checks.
// Agents compare this to their own version to determine if an update is available.
func (r *Router) handleAgentVersion(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return the server version - all agents should match the server version
	version := "dev"
	if versionInfo, err := updates.GetCurrentVersion(); err == nil {
		version = versionInfo.Version
	}

	response := AgentVersionResponse{
		Version: version,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
func (r *Router) handleServerInfo(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	versionInfo, err := updates.GetCurrentVersion()
	isDev := true
	version := "dev"
	if err == nil {
		isDev = versionInfo.IsDevelopment
		version = versionInfo.Version
	}

	response := map[string]interface{}{
		"isDevelopment": isDev,
		"version":       version,
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

	if req.Method != http.MethodGet && req.Method != http.MethodHead {
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
	case "8h":
		duration = 8 * time.Hour
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

	// Build guest types map for frontend to correctly identify VM vs Container
	guestTypes := make(map[string]string)
	for _, vm := range state.VMs {
		guestTypes[vm.ID] = "vm"
	}
	for _, ct := range state.Containers {
		guestTypes[ct.ID] = "container"
	}

	// Process Docker containers - get historical data
	dockerData := make(map[string]VMChartData)
	for _, host := range state.DockerHosts {
		for _, container := range host.Containers {
			if container.ID == "" {
				continue
			}

			if dockerData[container.ID] == nil {
				dockerData[container.ID] = make(VMChartData)
			}

			// Get historical metrics using the docker: prefix key
			metricKey := fmt.Sprintf("docker:%s", container.ID)
			metrics := r.monitor.GetGuestMetrics(metricKey, duration)

			// Convert metric points to API format
			for metricType, points := range metrics {
				dockerData[container.ID][metricType] = make([]MetricPoint, len(points))
				for i, point := range points {
					ts := point.Timestamp.Unix() * 1000
					if ts < oldestTimestamp {
						oldestTimestamp = ts
					}
					dockerData[container.ID][metricType][i] = MetricPoint{
						Timestamp: ts,
						Value:     point.Value,
					}
				}
			}

			// If no historical data, add current value
			if len(dockerData[container.ID]["cpu"]) == 0 {
				dockerData[container.ID]["cpu"] = []MetricPoint{
					{Timestamp: currentTime, Value: container.CPUPercent},
				}
				dockerData[container.ID]["memory"] = []MetricPoint{
					{Timestamp: currentTime, Value: container.MemoryPercent},
				}
				// Calculate disk percentage for Docker containers
				var diskPercent float64
				if container.RootFilesystemBytes > 0 && container.WritableLayerBytes > 0 {
					diskPercent = float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
					if diskPercent > 100 {
						diskPercent = 100
					}
				}
				dockerData[container.ID]["disk"] = []MetricPoint{
					{Timestamp: currentTime, Value: diskPercent},
				}
			}
		}
	}

	// Process Docker hosts - get historical data
	dockerHostData := make(map[string]VMChartData)
	for _, host := range state.DockerHosts {
		if host.ID == "" {
			continue
		}

		if dockerHostData[host.ID] == nil {
			dockerHostData[host.ID] = make(VMChartData)
		}

		// Get historical metrics using the dockerHost: prefix key
		metricKey := fmt.Sprintf("dockerHost:%s", host.ID)
		metrics := r.monitor.GetGuestMetrics(metricKey, duration)

		// Convert metric points to API format
		for metricType, points := range metrics {
			dockerHostData[host.ID][metricType] = make([]MetricPoint, len(points))
			for i, point := range points {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				dockerHostData[host.ID][metricType][i] = MetricPoint{
					Timestamp: ts,
					Value:     point.Value,
				}
			}
		}

		// If no historical data, add current value
		if len(dockerHostData[host.ID]["cpu"]) == 0 {
			dockerHostData[host.ID]["cpu"] = []MetricPoint{
				{Timestamp: currentTime, Value: host.CPUUsage},
			}
			dockerHostData[host.ID]["memory"] = []MetricPoint{
				{Timestamp: currentTime, Value: host.Memory.Usage},
			}
			// Use first disk for host disk percentage
			var diskPercent float64
			if len(host.Disks) > 0 {
				diskPercent = host.Disks[0].Usage
			}
			dockerHostData[host.ID]["disk"] = []MetricPoint{
				{Timestamp: currentTime, Value: diskPercent},
			}
		}
	}

	response := ChartResponse{
		ChartData:      chartData,
		NodeData:       nodeData,
		StorageData:    storageData,
		DockerData:     dockerData,
		DockerHostData: dockerHostData,
		GuestTypes:     guestTypes,
		Timestamp:      currentTime,
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
		Int("dockerContainers", len(dockerData)).
		Str("range", timeRange).
		Msg("Chart data response sent")
}

// handleStorageCharts handles storage chart data requests
func (r *Router) handleStorageCharts(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
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

// handleMetricsStoreStats returns statistics about the persistent metrics store
func (r *Router) handleMetricsStoreStats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	store := r.monitor.GetMetricsStore()
	if store == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": false,
			"error":   "Persistent metrics store not initialized",
		})
		return
	}

	stats := store.GetStats()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":       true,
		"dbSize":        stats.DBSize,
		"rawCount":      stats.RawCount,
		"minuteCount":   stats.MinuteCount,
		"hourlyCount":   stats.HourlyCount,
		"dailyCount":    stats.DailyCount,
		"totalWrites":   stats.TotalWrites,
		"bufferSize":    stats.BufferSize,
		"lastFlush":     stats.LastFlush,
		"lastRollup":    stats.LastRollup,
		"lastRetention": stats.LastRetention,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to encode metrics store stats")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleMetricsHistory returns historical metrics from the persistent SQLite store
// Query params:
//   - resourceType: "node", "guest", "storage", "docker", "dockerHost" (required)
//   - resourceId: the resource identifier (required)
//   - metric: "cpu", "memory", "disk", etc. (optional, omit for all metrics)
//   - range: time range like "1h", "24h", "7d", "30d", "90d" (optional, default "24h")
func (r *Router) handleMetricsHistory(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := req.URL.Query()
	resourceType := query.Get("resourceType")
	resourceID := query.Get("resourceId")
	metricType := query.Get("metric")
	timeRange := query.Get("range")

	if resourceType == "" || resourceID == "" {
		http.Error(w, "resourceType and resourceId are required", http.StatusBadRequest)
		return
	}

	// Parse time range
	var duration time.Duration
	var stepSecs int64 = 0 // Default to no downsampling (use tier resolution)

	switch timeRange {
	case "1h":
		duration = time.Hour
	case "6h":
		duration = 6 * time.Hour
	case "12h":
		duration = 12 * time.Hour
	case "24h", "1d", "":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	case "90d":
		duration = 90 * 24 * time.Hour
	default:
		// Try parsing as duration
		var err error
		duration, err = time.ParseDuration(timeRange)
		if err != nil {
			duration = 24 * time.Hour // Default to 24 hours
		}
	}

	// Optional downsampling based on requested max points.
	// When omitted, we return the native tier resolution.
	if maxPointsStr := query.Get("maxPoints"); maxPointsStr != "" {
		if maxPoints, err := strconv.Atoi(maxPointsStr); err == nil && maxPoints > 0 {
			durationSecs := int64(duration.Seconds())
			if durationSecs > 0 {
				stepSecs = (durationSecs + int64(maxPoints) - 1) / int64(maxPoints)
				if stepSecs <= 1 {
					stepSecs = 0
				} else {
					minStep := func(d time.Duration) int64 {
						switch {
						case d <= 2*time.Hour:
							return 5
						case d <= 24*time.Hour:
							return 60
						case d <= 7*24*time.Hour:
							return 3600
						default:
							return 86400
						}
					}
					if stepSecs < minStep(duration) {
						stepSecs = 0
					}
				}
			}
		}
	}

	// Enforce license limits: 7d free, 30d/90d require Pro
	// Returns 402 Payment Required for unlicensed long-term requests
	maxFreeDuration := 7 * 24 * time.Hour
	// Check license for long-term metrics
	if duration > maxFreeDuration && !r.licenseHandlers.Service(req.Context()).HasFeature(license.FeatureLongTermMetrics) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":       "license_required",
			"message":     "Long-term metrics history (30d/90d) requires a Pulse Pro license",
			"feature":     license.FeatureLongTermMetrics,
			"upgrade_url": "https://pulserelay.pro/",
			"max_free":    "7d",
		})
		return
	}

	end := time.Now()
	start := end.Add(-duration)

	const (
		historySourceStore  = "store"
		historySourceMemory = "memory"
		historySourceLive   = "live"
	)

	fallbackAllowed := duration <= 24*time.Hour
	buildHistoryPoints := func(points []monitoring.MetricPoint, bucketSecs int64) []map[string]interface{} {
		if len(points) == 0 {
			return []map[string]interface{}{}
		}
		if bucketSecs <= 1 {
			apiPoints := make([]map[string]interface{}, 0, len(points))
			for _, p := range points {
				apiPoints = append(apiPoints, map[string]interface{}{
					"timestamp": p.Timestamp.UnixMilli(),
					"value":     p.Value,
					"min":       p.Value,
					"max":       p.Value,
				})
			}
			return apiPoints
		}

		type bucket struct {
			sum   float64
			count int
			min   float64
			max   float64
		}

		buckets := make(map[int64]*bucket)
		for _, p := range points {
			ts := p.Timestamp.Unix()
			if ts <= 0 {
				continue
			}
			start := (ts / bucketSecs) * bucketSecs
			b, ok := buckets[start]
			if !ok {
				b = &bucket{
					sum:   p.Value,
					count: 1,
					min:   p.Value,
					max:   p.Value,
				}
				buckets[start] = b
				continue
			}
			b.sum += p.Value
			b.count++
			if p.Value < b.min {
				b.min = p.Value
			}
			if p.Value > b.max {
				b.max = p.Value
			}
		}

		keys := make([]int64, 0, len(buckets))
		for k := range buckets {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

		apiPoints := make([]map[string]interface{}, 0, len(keys))
		for _, k := range keys {
			b := buckets[k]
			if b.count == 0 {
				continue
			}
			ts := time.Unix(k+(bucketSecs/2), 0)
			apiPoints = append(apiPoints, map[string]interface{}{
				"timestamp": ts.UnixMilli(),
				"value":     b.sum / float64(b.count),
				"min":       b.min,
				"max":       b.max,
			})
		}
		return apiPoints
	}
	state := r.monitor.GetState()

	parseGuestID := func(id string) (string, string, int, bool) {
		parts := strings.Split(id, ":")
		if len(parts) != 3 {
			return "", "", 0, false
		}
		vmid, err := strconv.Atoi(parts[2])
		if err != nil {
			return "", "", 0, false
		}
		return parts[0], parts[1], vmid, true
	}

	findVM := func(id string) *models.VM {
		for i := range state.VMs {
			if state.VMs[i].ID == id {
				return &state.VMs[i]
			}
		}
		if instance, node, vmid, ok := parseGuestID(id); ok {
			for i := range state.VMs {
				vm := &state.VMs[i]
				if vm.VMID == vmid && vm.Node == node && vm.Instance == instance {
					return vm
				}
			}
		}
		return nil
	}

	findContainer := func(id string) *models.Container {
		for i := range state.Containers {
			if state.Containers[i].ID == id {
				return &state.Containers[i]
			}
		}
		if instance, node, vmid, ok := parseGuestID(id); ok {
			for i := range state.Containers {
				ct := &state.Containers[i]
				if ct.VMID == vmid && ct.Node == node && ct.Instance == instance {
					return ct
				}
			}
		}
		return nil
	}

	findNode := func(id string) *models.Node {
		for i := range state.Nodes {
			if state.Nodes[i].ID == id {
				return &state.Nodes[i]
			}
		}
		return nil
	}

	findStorage := func(id string) *models.Storage {
		for i := range state.Storage {
			if state.Storage[i].ID == id {
				return &state.Storage[i]
			}
		}
		return nil
	}

	findDockerHost := func(id string) *models.DockerHost {
		for i := range state.DockerHosts {
			if state.DockerHosts[i].ID == id {
				return &state.DockerHosts[i]
			}
		}
		return nil
	}

	findDockerContainer := func(id string) *models.DockerContainer {
		for i := range state.DockerHosts {
			host := &state.DockerHosts[i]
			for j := range host.Containers {
				if host.Containers[j].ID == id {
					return &host.Containers[j]
				}
			}
		}
		return nil
	}

	liveMetricPoints := func(resourceType, resourceID string) map[string]monitoring.MetricPoint {
		now := time.Now()
		points := make(map[string]monitoring.MetricPoint)

		switch resourceType {
		case "vm", "guest":
			vm := findVM(resourceID)
			if vm == nil {
				return points
			}
			points["cpu"] = monitoring.MetricPoint{Timestamp: now, Value: vm.CPU * 100}
			points["memory"] = monitoring.MetricPoint{Timestamp: now, Value: vm.Memory.Usage}
			if vm.Disk.Usage >= 0 {
				points["disk"] = monitoring.MetricPoint{Timestamp: now, Value: vm.Disk.Usage}
			}
			points["diskread"] = monitoring.MetricPoint{Timestamp: now, Value: float64(vm.DiskRead)}
			points["diskwrite"] = monitoring.MetricPoint{Timestamp: now, Value: float64(vm.DiskWrite)}
			points["netin"] = monitoring.MetricPoint{Timestamp: now, Value: float64(vm.NetworkIn)}
			points["netout"] = monitoring.MetricPoint{Timestamp: now, Value: float64(vm.NetworkOut)}
		case "container":
			ct := findContainer(resourceID)
			if ct == nil {
				return points
			}
			points["cpu"] = monitoring.MetricPoint{Timestamp: now, Value: ct.CPU * 100}
			points["memory"] = monitoring.MetricPoint{Timestamp: now, Value: ct.Memory.Usage}
			if ct.Disk.Usage >= 0 {
				points["disk"] = monitoring.MetricPoint{Timestamp: now, Value: ct.Disk.Usage}
			}
			points["diskread"] = monitoring.MetricPoint{Timestamp: now, Value: float64(ct.DiskRead)}
			points["diskwrite"] = monitoring.MetricPoint{Timestamp: now, Value: float64(ct.DiskWrite)}
			points["netin"] = monitoring.MetricPoint{Timestamp: now, Value: float64(ct.NetworkIn)}
			points["netout"] = monitoring.MetricPoint{Timestamp: now, Value: float64(ct.NetworkOut)}
		case "node":
			node := findNode(resourceID)
			if node == nil {
				return points
			}
			points["cpu"] = monitoring.MetricPoint{Timestamp: now, Value: node.CPU * 100}
			points["memory"] = monitoring.MetricPoint{Timestamp: now, Value: node.Memory.Usage}
			points["disk"] = monitoring.MetricPoint{Timestamp: now, Value: node.Disk.Usage}
		case "storage":
			storage := findStorage(resourceID)
			if storage == nil {
				return points
			}
			usagePercent := float64(0)
			if storage.Total > 0 {
				usagePercent = (float64(storage.Used) / float64(storage.Total)) * 100
			}
			points["disk"] = monitoring.MetricPoint{Timestamp: now, Value: usagePercent}
			points["usage"] = monitoring.MetricPoint{Timestamp: now, Value: usagePercent}
			points["used"] = monitoring.MetricPoint{Timestamp: now, Value: float64(storage.Used)}
			points["total"] = monitoring.MetricPoint{Timestamp: now, Value: float64(storage.Total)}
			points["avail"] = monitoring.MetricPoint{Timestamp: now, Value: float64(storage.Free)}
		case "dockerHost":
			host := findDockerHost(resourceID)
			if host == nil {
				return points
			}
			points["cpu"] = monitoring.MetricPoint{Timestamp: now, Value: host.CPUUsage}
			points["memory"] = monitoring.MetricPoint{Timestamp: now, Value: host.Memory.Usage}
			diskPercent := float64(0)
			if len(host.Disks) > 0 {
				diskPercent = host.Disks[0].Usage
			}
			points["disk"] = monitoring.MetricPoint{Timestamp: now, Value: diskPercent}
		case "docker", "dockerContainer":
			container := findDockerContainer(resourceID)
			if container == nil {
				return points
			}
			points["cpu"] = monitoring.MetricPoint{Timestamp: now, Value: container.CPUPercent}
			points["memory"] = monitoring.MetricPoint{Timestamp: now, Value: container.MemoryPercent}
			if container.RootFilesystemBytes > 0 && container.WritableLayerBytes > 0 {
				diskPercent := float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
				if diskPercent > 100 {
					diskPercent = 100
				}
				points["disk"] = monitoring.MetricPoint{Timestamp: now, Value: diskPercent}
			}
		}

		return points
	}

	fallbackSingle := func() ([]map[string]interface{}, string, bool) {
		if !fallbackAllowed || metricType == "" {
			return nil, "", false
		}

		switch resourceType {
		case "vm", "container", "guest":
			metrics := r.monitor.GetGuestMetrics(resourceID, duration)
			points := metrics[metricType]
			if len(points) == 0 {
				livePoints := liveMetricPoints(resourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), historySourceMemory, true
		case "dockerHost":
			metrics := r.monitor.GetGuestMetrics(fmt.Sprintf("dockerHost:%s", resourceID), duration)
			points := metrics[metricType]
			if len(points) == 0 {
				livePoints := liveMetricPoints(resourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), historySourceMemory, true
		case "docker", "dockerContainer":
			metrics := r.monitor.GetGuestMetrics(fmt.Sprintf("docker:%s", resourceID), duration)
			points := metrics[metricType]
			if len(points) == 0 {
				livePoints := liveMetricPoints(resourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), historySourceMemory, true
		case "node":
			points := r.monitor.GetNodeMetrics(resourceID, metricType, duration)
			if len(points) == 0 {
				livePoints := liveMetricPoints(resourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), historySourceMemory, true
		case "storage":
			metrics := r.monitor.GetStorageMetrics(resourceID, duration)
			points := metrics[metricType]
			if len(points) == 0 {
				livePoints := liveMetricPoints(resourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), historySourceMemory, true
		default:
			livePoints := liveMetricPoints(resourceType, resourceID)
			if live, ok := livePoints[metricType]; ok {
				return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
			}
			return nil, "", false
		}
	}

	fallbackAll := func() (map[string][]map[string]interface{}, string, bool) {
		if !fallbackAllowed || metricType != "" {
			return nil, "", false
		}

		var metrics map[string][]monitoring.MetricPoint
		switch resourceType {
		case "vm", "container", "guest":
			metrics = r.monitor.GetGuestMetrics(resourceID, duration)
		case "dockerHost":
			metrics = r.monitor.GetGuestMetrics(fmt.Sprintf("dockerHost:%s", resourceID), duration)
		case "docker", "dockerContainer":
			metrics = r.monitor.GetGuestMetrics(fmt.Sprintf("docker:%s", resourceID), duration)
		case "storage":
			metrics = r.monitor.GetStorageMetrics(resourceID, duration)
		default:
			if resourceType == "node" {
				metrics = map[string][]monitoring.MetricPoint{
					"cpu":    r.monitor.GetNodeMetrics(resourceID, "cpu", duration),
					"memory": r.monitor.GetNodeMetrics(resourceID, "memory", duration),
					"disk":   r.monitor.GetNodeMetrics(resourceID, "disk", duration),
				}
			} else {
				return nil, "", false
			}
		}

		apiData := make(map[string][]map[string]interface{})
		source := historySourceMemory
		for metric, points := range metrics {
			if len(points) == 0 {
				continue
			}
			apiData[metric] = buildHistoryPoints(points, stepSecs)
		}
		if len(apiData) == 0 {
			livePoints := liveMetricPoints(resourceType, resourceID)
			for metric, point := range livePoints {
				apiData[metric] = buildHistoryPoints([]monitoring.MetricPoint{point}, 0)
			}
			source = historySourceLive
		}
		if len(apiData) == 0 {
			return nil, "", false
		}
		return apiData, source, true
	}

	store := r.monitor.GetMetricsStore()
	if store == nil {
		if metricType != "" {
			if apiPoints, source, ok := fallbackSingle(); ok {
				log.Warn().
					Str("resourceType", resourceType).
					Str("resourceId", resourceID).
					Str("metric", metricType).
					Str("source", source).
					Msg("Metrics store unavailable; serving history from fallback source")
				response := map[string]interface{}{
					"resourceType": resourceType,
					"resourceId":   resourceID,
					"metric":       metricType,
					"range":        timeRange,
					"start":        start.UnixMilli(),
					"end":          end.UnixMilli(),
					"points":       apiPoints,
					"source":       source,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
		} else {
			if apiData, source, ok := fallbackAll(); ok {
				log.Warn().
					Str("resourceType", resourceType).
					Str("resourceId", resourceID).
					Str("source", source).
					Msg("Metrics store unavailable; serving history from fallback source")
				response := map[string]interface{}{
					"resourceType": resourceType,
					"resourceId":   resourceID,
					"range":        timeRange,
					"start":        start.UnixMilli(),
					"end":          end.UnixMilli(),
					"metrics":      apiData,
					"source":       source,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Persistent metrics store not available",
		})
		return
	}

	var response interface{}

	if metricType != "" {
		source := historySourceStore
		// Query single metric type
		points, err := store.Query(resourceType, resourceID, metricType, start, end, stepSecs)
		if err != nil {
			log.Error().Err(err).
				Str("resourceType", resourceType).
				Str("resourceId", resourceID).
				Str("metric", metricType).
				Msg("Failed to query metrics history")
			http.Error(w, "Failed to query metrics", http.StatusInternalServerError)
			return
		}

		if len(points) == 0 {
			if apiPoints, fallbackSource, ok := fallbackSingle(); ok {
				source = fallbackSource
				log.Info().
					Str("resourceType", resourceType).
					Str("resourceId", resourceID).
					Str("metric", metricType).
					Str("source", source).
					Msg("Metrics store empty; serving history from fallback source")
				response = map[string]interface{}{
					"resourceType": resourceType,
					"resourceId":   resourceID,
					"metric":       metricType,
					"range":        timeRange,
					"start":        start.UnixMilli(),
					"end":          end.UnixMilli(),
					"points":       apiPoints,
					"source":       source,
				}
			}
		}

		// Convert to frontend format (timestamps in milliseconds)
		if response == nil {
			apiPoints := make([]map[string]interface{}, len(points))
			for i, p := range points {
				apiPoints[i] = map[string]interface{}{
					"timestamp": p.Timestamp.UnixMilli(),
					"value":     p.Value,
					"min":       p.Min,
					"max":       p.Max,
				}
			}

			response = map[string]interface{}{
				"resourceType": resourceType,
				"resourceId":   resourceID,
				"metric":       metricType,
				"range":        timeRange,
				"start":        start.UnixMilli(),
				"end":          end.UnixMilli(),
				"points":       apiPoints,
				"source":       source,
			}
		}
	} else {
		source := historySourceStore
		// Query all metrics for this resource
		metricsMap, err := store.QueryAll(resourceType, resourceID, start, end, stepSecs)
		if err != nil {
			log.Error().Err(err).
				Str("resourceType", resourceType).
				Str("resourceId", resourceID).
				Msg("Failed to query all metrics history")
			http.Error(w, "Failed to query metrics", http.StatusInternalServerError)
			return
		}

		if len(metricsMap) == 0 {
			if apiData, fallbackSource, ok := fallbackAll(); ok {
				source = fallbackSource
				log.Info().
					Str("resourceType", resourceType).
					Str("resourceId", resourceID).
					Str("source", source).
					Msg("Metrics store empty; serving history from fallback source")
				response = map[string]interface{}{
					"resourceType": resourceType,
					"resourceId":   resourceID,
					"range":        timeRange,
					"start":        start.UnixMilli(),
					"end":          end.UnixMilli(),
					"metrics":      apiData,
					"source":       source,
				}
			}
		}

		// Convert to frontend format
		if response == nil {
			apiData := make(map[string][]map[string]interface{})
			for metric, points := range metricsMap {
				apiPoints := make([]map[string]interface{}, len(points))
				for i, p := range points {
					apiPoints[i] = map[string]interface{}{
						"timestamp": p.Timestamp.UnixMilli(),
						"value":     p.Value,
						"min":       p.Min,
						"max":       p.Max,
					}
				}
				apiData[metric] = apiPoints
			}

			response = map[string]interface{}{
				"resourceType": resourceType,
				"resourceId":   resourceID,
				"range":        timeRange,
				"start":        start.UnixMilli(),
				"end":          end.UnixMilli(),
				"metrics":      apiData,
				"source":       source,
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode metrics history response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleConfig handles configuration requests
func (r *Router) handleConfig(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	config.Mu.RLock()
	defer config.Mu.RUnlock()

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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Backups     models.Backups         `json:"backups"`
		PVEBackups  models.PVEBackups      `json:"pveBackups"`
		PBSBackups  []models.PBSBackup     `json:"pbsBackups"`
		PMGBackups  []models.PMGBackup     `json:"pmgBackups"`
		BackupTasks []models.BackupTask    `json:"backupTasks"`
		Storage     []models.StorageBackup `json:"storageBackups"`
		GuestSnaps  []models.GuestSnapshot `json:"guestSnapshots"`
	}{
		Backups:     state.Backups,
		PVEBackups:  state.PVEBackups,
		PBSBackups:  state.PBSBackups,
		PMGBackups:  state.PMGBackups,
		BackupTasks: state.PVEBackups.BackupTasks,
		Storage:     state.PVEBackups.StorageBackups,
		GuestSnaps:  state.PVEBackups.GuestSnapshots,
	})
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
	// Check authentication before allowing WebSocket upgrade
	if !CheckAuth(r.config, w, req) {
		return
	}
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

// backgroundUpdateChecker periodically checks for updates and caches the result
func (r *Router) backgroundUpdateChecker() {
	// Delay initial check to allow WebSocket clients to receive welcome messages first
	time.Sleep(1 * time.Second)

	ctx := context.Background()
	if _, err := r.updateManager.CheckForUpdates(ctx); err != nil {
		log.Debug().Err(err).Msg("Initial update check failed")
	} else {
		log.Info().Msg("Initial update check completed")
	}

	// Then check every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		if _, err := r.updateManager.CheckForUpdates(ctx); err != nil {
			log.Debug().Err(err).Msg("Periodic update check failed")
		} else {
			log.Debug().Msg("Periodic update check completed")
		}
	}
}

// handleDownloadInstallScript serves the Docker agent installation script
func (r *Router) handleDownloadInstallScript(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Prevent caching - always serve the latest version
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	scriptPath := "/opt/pulse/scripts/install-docker-agent.sh"
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		// Fallback to project root (dev environment)
		scriptPath = filepath.Join(r.projectRoot, "scripts", "install-docker-agent.sh")
		content, err = os.ReadFile(scriptPath)
		if err != nil {
			log.Error().Err(err).Str("path", scriptPath).Msg("Failed to read Docker agent installer script")
			http.Error(w, "Failed to read installer script", http.StatusInternalServerError)
			return
		}
	}

	http.ServeContent(w, req, "install-docker-agent.sh", time.Now(), bytes.NewReader(content))
}

// handleDownloadContainerAgentInstallScript serves the container agent install script
func (r *Router) handleDownloadContainerAgentInstallScript(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Prevent caching - always serve the latest version
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	scriptPath := "/opt/pulse/scripts/install-container-agent.sh"
	http.ServeFile(w, req, scriptPath)
}

// handleDownloadAgent serves the Docker agent binary
func (r *Router) handleDownloadAgent(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Prevent caching - always serve the latest version
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	archParam := strings.TrimSpace(req.URL.Query().Get("arch"))
	searchPaths := make([]string, 0, 6)

	if normalized := normalizeDockerAgentArch(archParam); normalized != "" {
		searchPaths = append(searchPaths,
			filepath.Join(pulseBinDir(), "pulse-docker-agent-"+normalized),
			filepath.Join("/opt/pulse", "pulse-docker-agent-"+normalized),
			filepath.Join("/app", "pulse-docker-agent-"+normalized),               // legacy Docker image layout
			filepath.Join(r.projectRoot, "bin", "pulse-docker-agent-"+normalized), // dev environment
		)
	}

	// Default locations (host architecture)
	searchPaths = append(searchPaths,
		filepath.Join(pulseBinDir(), "pulse-docker-agent"),
		"/opt/pulse/pulse-docker-agent",
		filepath.Join("/app", "pulse-docker-agent"),               // legacy Docker image layout
		filepath.Join(r.projectRoot, "bin", "pulse-docker-agent"), // dev environment
	)

	for _, candidate := range searchPaths {
		if candidate == "" {
			continue
		}

		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}

		checksum, err := r.cachedSHA256(candidate, info)
		if err != nil {
			log.Error().Err(err).Str("path", candidate).Msg("Failed to compute docker agent checksum")
			continue
		}

		file, err := os.Open(candidate)
		if err != nil {
			log.Error().Err(err).Str("path", candidate).Msg("Failed to open docker agent binary for download")
			continue
		}

		w.Header().Set("X-Checksum-Sha256", checksum)
		http.ServeContent(w, req, filepath.Base(candidate), info.ModTime(), file)
		file.Close()
		return
	}

	http.Error(w, "Agent binary not found", http.StatusNotFound) // Agent binary not found
}

// handleDownloadHostAgentInstallScript serves the Host agent installation script
func (r *Router) handleDownloadHostAgentInstallScript(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Prevent caching - always serve the latest version
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Serve the unified install.sh script (backwards compatible with install-host-agent.sh URL)
	scriptPath := "/opt/pulse/scripts/install.sh"
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		// Fallback to project root (dev environment)
		scriptPath = filepath.Join(r.projectRoot, "scripts", "install.sh")
		content, err = os.ReadFile(scriptPath)
		if err != nil {
			log.Error().Err(err).Str("path", scriptPath).Msg("Failed to read unified agent installer script")
			http.Error(w, "Failed to read installer script", http.StatusInternalServerError)
			return
		}
	}

	http.ServeContent(w, req, "install.sh", time.Now(), bytes.NewReader(content))
}

// handleDownloadHostAgentInstallScriptPS serves the PowerShell installation script for Windows
func (r *Router) handleDownloadHostAgentInstallScriptPS(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Prevent caching - always serve the latest version
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	scriptPath := "/opt/pulse/scripts/install-host-agent.ps1"
	http.ServeFile(w, req, scriptPath)
}

// handleDownloadHostAgentUninstallScript serves the bash uninstallation script for Linux/macOS
func (r *Router) handleDownloadHostAgentUninstallScript(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Prevent caching - always serve the latest version
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	scriptPath := "/opt/pulse/scripts/uninstall-host-agent.sh"
	http.ServeFile(w, req, scriptPath)
}

// handleDownloadHostAgentUninstallScriptPS serves the PowerShell uninstallation script for Windows
func (r *Router) handleDownloadHostAgentUninstallScriptPS(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Prevent caching - always serve the latest version
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	scriptPath := "/opt/pulse/scripts/uninstall-host-agent.ps1"
	http.ServeFile(w, req, scriptPath)
}

// handleDownloadHostAgent serves the Host agent binary
func (r *Router) handleDownloadHostAgent(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Prevent caching - always serve the latest version
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	platformParam := strings.TrimSpace(req.URL.Query().Get("platform"))
	archParam := strings.TrimSpace(req.URL.Query().Get("arch"))

	// Validate platform and arch to prevent path traversal attacks
	// Only allow alphanumeric characters and hyphens
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9\-]+$`)
	if platformParam != "" && !validPattern.MatchString(platformParam) {
		http.Error(w, "Invalid platform parameter", http.StatusBadRequest)
		return
	}
	if archParam != "" && !validPattern.MatchString(archParam) {
		http.Error(w, "Invalid arch parameter", http.StatusBadRequest)
		return
	}

	checkedPaths, served := r.tryServeHostAgentBinary(w, req, platformParam, archParam)
	if served {
		return
	}

	remainingMissing := agentbinaries.EnsureHostAgentBinaries(r.serverVersion)

	afterRestorePaths, served := r.tryServeHostAgentBinary(w, req, platformParam, archParam)
	checkedPaths = append(checkedPaths, afterRestorePaths...)
	if served {
		return
	}

	// Build detailed error message with troubleshooting guidance
	var errorMsg strings.Builder
	errorMsg.WriteString(fmt.Sprintf("Host agent binary not found for %s/%s\n\n", platformParam, archParam))
	errorMsg.WriteString("Troubleshooting:\n")
	errorMsg.WriteString("1. If running in Docker: Rebuild the Docker image to include all platform binaries\n")
	errorMsg.WriteString("2. If running from source: Run 'scripts/build-release.sh' to build all platform binaries\n")
	errorMsg.WriteString("3. Build from source:\n")
	errorMsg.WriteString(fmt.Sprintf("   GOOS=%s GOARCH=%s go build -o pulse-host-agent-%s-%s ./cmd/pulse-host-agent\n", platformParam, archParam, platformParam, archParam))
	errorMsg.WriteString(fmt.Sprintf("   sudo mv pulse-host-agent-%s-%s /opt/pulse/bin/\n\n", platformParam, archParam))

	if len(remainingMissing) > 0 {
		errorMsg.WriteString("Automatic repair attempted but the following binaries are still missing:\n")
		for _, key := range sortedHostAgentKeys(remainingMissing) {
			errorMsg.WriteString(fmt.Sprintf("  - %s\n", key))
		}
		if r.serverVersion != "" {
			errorMsg.WriteString(fmt.Sprintf("Release bundle used: %s\n\n", strings.TrimSpace(r.serverVersion)))
		} else {
			errorMsg.WriteString("\n")
		}
	}

	errorMsg.WriteString("Searched locations:\n")
	for _, path := range dedupeStrings(checkedPaths) {
		errorMsg.WriteString(fmt.Sprintf("  - %s\n", path))
	}

	http.Error(w, errorMsg.String(), http.StatusNotFound)
}

func (r *Router) tryServeHostAgentBinary(w http.ResponseWriter, req *http.Request, platformParam, archParam string) ([]string, bool) {
	searchPaths := hostAgentSearchCandidates(platformParam, archParam)
	checkedPaths := make([]string, 0, len(searchPaths)*2)

	shouldCheckWindowsExe := func(path string) bool {
		base := strings.ToLower(filepath.Base(path))
		return strings.Contains(base, "windows") && !strings.HasSuffix(base, ".exe")
	}

	for _, candidate := range searchPaths {
		if candidate == "" {
			continue
		}
		pathsToCheck := []string{candidate}
		if shouldCheckWindowsExe(candidate) {
			pathsToCheck = append(pathsToCheck, candidate+".exe")
		}

		for _, path := range pathsToCheck {
			checkedPaths = append(checkedPaths, path)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				if strings.HasSuffix(req.URL.Path, ".sha256") {
					r.serveChecksum(w, path)
					return checkedPaths, true
				}
				http.ServeFile(w, req, path)
				return checkedPaths, true
			}
		}
	}

	return checkedPaths, false
}

func hostAgentSearchCandidates(platformParam, archParam string) []string {
	searchPaths := make([]string, 0, 12)
	strictMode := platformParam != "" && archParam != ""

	if strictMode {
		searchPaths = append(searchPaths,
			filepath.Join(pulseBinDir(), fmt.Sprintf("pulse-host-agent-%s-%s", platformParam, archParam)),
			filepath.Join("/opt/pulse", fmt.Sprintf("pulse-host-agent-%s-%s", platformParam, archParam)),
			filepath.Join("/app", fmt.Sprintf("pulse-host-agent-%s-%s", platformParam, archParam)),
		)
	}

	if platformParam != "" && !strictMode {
		searchPaths = append(searchPaths,
			filepath.Join(pulseBinDir(), "pulse-host-agent-"+platformParam),
			filepath.Join("/opt/pulse", "pulse-host-agent-"+platformParam),
			filepath.Join("/app", "pulse-host-agent-"+platformParam),
		)
	}

	if !strictMode && platformParam == "" {
		searchPaths = append(searchPaths,
			filepath.Join(pulseBinDir(), "pulse-host-agent"),
			"/opt/pulse/pulse-host-agent",
			filepath.Join("/app", "pulse-host-agent"),
		)
	}

	return searchPaths
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func sortedHostAgentKeys(missing map[string]agentbinaries.HostAgentBinary) []string {
	if len(missing) == 0 {
		return nil
	}
	keys := make([]string, 0, len(missing))
	for key := range missing {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type checksumCacheEntry struct {
	checksum string
	modTime  time.Time
	size     int64
}

func (r *Router) cachedSHA256(filePath string, info os.FileInfo) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("empty file path")
	}

	if info == nil {
		var err error
		info, err = os.Stat(filePath)
		if err != nil {
			return "", err
		}
	}

	r.checksumMu.RLock()
	entry, ok := r.checksumCache[filePath]
	r.checksumMu.RUnlock()
	if ok && entry.size == info.Size() && entry.modTime.Equal(info.ModTime()) {
		return entry.checksum, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	r.checksumMu.Lock()
	if r.checksumCache == nil {
		r.checksumCache = make(map[string]checksumCacheEntry)
	}
	r.checksumCache[filePath] = checksumCacheEntry{
		checksum: checksum,
		modTime:  info.ModTime(),
		size:     info.Size(),
	}
	r.checksumMu.Unlock()

	return checksum, nil
}

// serveChecksum computes and serves the SHA256 checksum of a file
func (r *Router) serveChecksum(w http.ResponseWriter, filePath string) {
	info, err := os.Stat(filePath)
	if err != nil {
		http.Error(w, "Failed to stat file", http.StatusInternalServerError)
		return
	}

	checksum, err := r.cachedSHA256(filePath, info)
	if err != nil {
		http.Error(w, "Failed to compute checksum", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "%s\n", checksum)
}

func (r *Router) handleDiagnosticsDockerPrepareToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	var payload struct {
		HostID    string `json:"hostId"`
		TokenName string `json:"tokenName"`
	}

	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", nil)
		return
	}

	hostID := strings.TrimSpace(payload.HostID)
	if hostID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_host_id", "hostId is required", nil)
		return
	}

	host, ok := r.monitor.GetDockerHost(hostID)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, "host_not_found", "Docker host not found", nil)
		return
	}

	name := strings.TrimSpace(payload.TokenName)
	if name == "" {
		displayName := preferredDockerHostName(host)
		name = fmt.Sprintf("Docker host: %s", displayName)
	}

	rawToken, err := auth.GenerateAPIToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate docker migration token")
		writeErrorResponse(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate API token", nil)
		return
	}

	record, err := config.NewAPITokenRecord(rawToken, name, []string{config.ScopeDockerReport})
	if err != nil {
		log.Error().Err(err).Msg("Failed to construct token record for docker migration")
		writeErrorResponse(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate API token", nil)
		return
	}

	r.config.APITokens = append(r.config.APITokens, *record)
	r.config.SortAPITokens()

	if r.persistence != nil {
		if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
			r.config.RemoveAPIToken(record.ID)
			log.Error().Err(err).Msg("Failed to persist API tokens after docker migration generation")
			writeErrorResponse(w, http.StatusInternalServerError, "token_persist_failed", "Failed to persist API token", nil)
			return
		}
	}

	baseURL := strings.TrimRight(r.resolvePublicURL(req), "/")
	installCommand := fmt.Sprintf("curl -fSL '%s/install-docker-agent.sh' -o /tmp/pulse-install-docker-agent.sh && sudo bash /tmp/pulse-install-docker-agent.sh --url '%s' --token '%s' && rm -f /tmp/pulse-install-docker-agent.sh", baseURL, baseURL, rawToken)
	systemdSnippet := fmt.Sprintf("[Service]\nType=simple\nEnvironment=\"PULSE_URL=%s\"\nEnvironment=\"PULSE_TOKEN=%s\"\nExecStart=/usr/local/bin/pulse-docker-agent --url %s --interval 30s\nRestart=always\nRestartSec=5s\nUser=root", baseURL, rawToken, baseURL)

	response := map[string]any{
		"success": true,
		"token":   rawToken,
		"record":  toAPITokenDTO(*record),
		"host": map[string]any{
			"id":   host.ID,
			"name": preferredDockerHostName(host),
		},
		"installCommand":        installCommand,
		"systemdServiceSnippet": systemdSnippet,
		"pulseURL":              baseURL,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to serialize docker token migration response")
	}
}

func (r *Router) handleDownloadDockerInstallerScript(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	// Try pre-built location first (in container)
	scriptPath := "/opt/pulse/scripts/install-docker.sh"
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		// Fallback to project root (dev environment)
		scriptPath = filepath.Join(r.projectRoot, "scripts", "install-docker.sh")
		content, err = os.ReadFile(scriptPath)
		if err != nil {
			log.Error().Err(err).Str("path", scriptPath).Msg("Failed to read Docker installer script")
			writeErrorResponse(w, http.StatusInternalServerError, "read_error", "Failed to read Docker installer script", nil)
			return
		}
	}

	w.Header().Set("Content-Type", "text/x-shellscript")
	w.Header().Set("Content-Disposition", "attachment; filename=install-docker.sh")
	if _, err := w.Write(content); err != nil {
		log.Error().Err(err).Msg("Failed to write Docker installer script to client")
	}
}

func (r *Router) resolvePublicURL(req *http.Request) string {
	if agentConnectURL := strings.TrimSpace(r.config.AgentConnectURL); agentConnectURL != "" {
		return strings.TrimRight(agentConnectURL, "/")
	}

	if publicURL := strings.TrimSpace(r.config.PublicURL); publicURL != "" {
		return strings.TrimRight(publicURL, "/")
	}

	scheme := "http"
	if req != nil {
		if req.TLS != nil {
			scheme = "https"
		} else if proto := req.Header.Get("X-Forwarded-Proto"); strings.EqualFold(proto, "https") {
			scheme = "https"
		}
	}

	host := ""
	if req != nil {
		host = strings.TrimSpace(req.Host)
	}
	if host == "" {
		if r.config.FrontendPort > 0 {
			host = fmt.Sprintf("localhost:%d", r.config.FrontendPort)
		} else {
			host = "localhost:7655"
		}
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func normalizeDockerAgentArch(arch string) string {
	if arch == "" {
		return ""
	}

	arch = strings.ToLower(strings.TrimSpace(arch))
	switch arch {
	case "linux-amd64", "amd64", "x86_64", "x86-64":
		return "linux-amd64"
	case "linux-arm64", "arm64", "aarch64":
		return "linux-arm64"
	case "linux-armv7", "armv7", "armv7l", "armhf":
		return "linux-armv7"
	case "linux-armv6", "armv6", "armv6l":
		return "linux-armv6"
	case "linux-386", "386", "i386", "i686":
		return "linux-386"
	default:
		return ""
	}
}

// trigger rebuild Fri Jan 16 10:52:41 UTC 2026
