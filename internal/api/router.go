package api

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/adapters"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/learning"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/conversion"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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
	trueNASHandlers           *TrueNASHandlers
	notificationHandlers      *NotificationHandlers
	notificationQueueHandlers *NotificationQueueHandlers
	dockerAgentHandlers       *DockerAgentHandlers
	kubernetesAgentHandlers   *KubernetesAgentHandlers
	hostAgentHandlers         *HostAgentHandlers
	systemSettingsHandler     *SystemSettingsHandler
	aiSettingsHandler         *AISettingsHandler
	aiHandler                 *AIHandler // AI chat handler
	discoveryHandlers         *DiscoveryHandlers
	resourceHandlers          *ResourceHandlers
	resourceRegistry          *unifiedresources.ResourceRegistry
	trueNASPoller             *monitoring.TrueNASPoller
	monitorResourceAdapter    *unifiedresources.MonitorAdapter
	aiUnifiedAdapter          *unifiedresources.UnifiedAIAdapter
	reportingHandlers         *ReportingHandlers
	configProfileHandler      *ConfigProfileHandler
	licenseHandlers           *LicenseHandlers
	recoveryHandlers          *RecoveryHandlers
	rbacProvider              *TenantRBACProvider
	logHandlers               *LogHandlers
	agentExecServer           *agentexec.Server
	wsHub                     *websocket.Hub
	reloadFunc                func() error
	updateManager             *updates.Manager
	updateHistory             *updates.UpdateHistory
	exportLimiter             *RateLimiter
	downloadLimiter           *RateLimiter
	signupRateLimiter         *RateLimiter
	tenantRateLimiter         *TenantRateLimiter
	persistence               *config.ConfigPersistence
	multiTenant               *config.MultiTenantPersistence
	oidcMu                    sync.Mutex
	oidcService               *OIDCService
	oidcManager               *OIDCServiceManager
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
	installScriptClient  *http.Client
	relayMu              sync.RWMutex
	relayClient          *relay.Client
	relayCancel          context.CancelFunc
	lifecycleCtx         context.Context
	lifecycleCancel      context.CancelFunc
	hostedMode           bool
	conversionStore      *conversion.ConversionStore
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
func NewRouter(cfg *config.Config, monitor *monitoring.Monitor, mtMonitor *monitoring.MultiTenantMonitor, wsHub *websocket.Hub, reloadFunc func() error, serverVersion string, conversionStore ...*conversion.ConversionStore) *Router {
	var store *conversion.ConversionStore
	if len(conversionStore) > 0 {
		store = conversionStore[0]
	}

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
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())

	r := &Router{
		mux:               http.NewServeMux(),
		config:            cfg,
		monitor:           monitor,
		mtMonitor:         mtMonitor,
		wsHub:             wsHub,
		reloadFunc:        reloadFunc,
		updateManager:     updateManager,
		updateHistory:     updateHistory,
		exportLimiter:     NewRateLimiter(5, 1*time.Minute),  // 5 attempts per minute
		downloadLimiter:   NewRateLimiter(60, 1*time.Minute), // downloads/installers per minute per IP
		signupRateLimiter: NewRateLimiter(5, 1*time.Hour),    // signup attempts per hour per IP
		persistence:       config.NewConfigPersistence(cfg.DataPath),
		multiTenant:       config.NewMultiTenantPersistence(cfg.DataPath),
		authorizer:        auth.GetAuthorizer(),
		serverVersion:     strings.TrimSpace(serverVersion),
		projectRoot:       projectRoot,
		checksumCache:     make(map[string]checksumCacheEntry),
		lifecycleCtx:      lifecycleCtx,
		lifecycleCancel:   lifecycleCancel,
		hostedMode:        os.Getenv("PULSE_HOSTED_MODE") == "true",
		conversionStore:   store,
	}
	if r.hostedMode {
		// Use defaults: 2000 req/min per org.
		r.tenantRateLimiter = NewTenantRateLimiter(0, 0)
	}
	r.resourceRegistry = unifiedresources.NewRegistry(nil)
	r.monitorResourceAdapter = unifiedresources.NewMonitorAdapter(r.resourceRegistry)
	r.aiUnifiedAdapter = unifiedresources.NewUnifiedAIAdapter(r.resourceRegistry)

	// Sync the configured admin user to the authorizer (if supported)
	if cfg.AuthUser != "" {
		auth.SetAdminUser(cfg.AuthUser)
	}

	// Initialize SSO service managers
	r.oidcManager = NewOIDCServiceManager()
	r.samlManager = NewSAMLServiceManager("")

	r.initializeBootstrapToken()

	r.setupRoutes()
	log.Debug().Msg("Routes registered successfully")

	// Start forwarding update progress to WebSocket
	go r.forwardUpdateProgress()

	// Start background update checker
	go r.backgroundUpdateChecker(r.lifecycleCtx)

	// Load system settings once at startup and cache them
	r.reloadSystemSettings()

	// Get cached values for middleware configuration
	r.settingsMu.RLock()
	allowEmbedding := r.cachedAllowEmbedding
	allowedOrigins := r.cachedAllowedOrigins
	r.settingsMu.RUnlock()

	// Apply middleware chain:
	// 1. Universal rate limiting (outermost to stop attacks early)
	// 2. Auth context extraction (populates user/token in context)
	// 3. Tenant selection and authorization (uses auth context)
	// 4. Demo mode (read-only protection)
	// 5. Error handling
	// 6. Security headers with embedding configuration
	// Note: TimeoutHandler breaks WebSocket upgrades
	handler := SecurityHeadersWithConfig(r, allowEmbedding, allowedOrigins)
	handler = ErrorHandler(handler)
	handler = DemoModeMiddleware(cfg, handler)

	// Create tenant middleware with authorization checker.
	// In hosted mode, tenant routing uses subscription lifecycle checks instead of FeatureMultiTenant.
	var orgLoader OrganizationLoader
	if r.multiTenant != nil {
		orgLoader = NewMultiTenantOrganizationLoader(r.multiTenant)
	}
	authChecker := NewAuthorizationChecker(orgLoader)
	tenantMiddleware := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{
		Persistence: r.multiTenant,
		AuthChecker: authChecker,
		HostedMode:  r.hostedMode,
	})

	// Per-tenant rate limiting (hosted mode only).
	// This relies on org ID stored in context by TenantMiddleware; because the chain is built inside-out,
	// it must be wrapped before TenantMiddleware so TenantMiddleware runs first.
	if r.tenantRateLimiter != nil {
		handler = TenantRateLimitMiddleware(r.tenantRateLimiter)(handler)
	}
	handler = tenantMiddleware.Middleware(handler)

	// Auth context middleware extracts user/token info BEFORE tenant middleware
	handler = AuthContextMiddleware(cfg, r.mtMonitor, handler)

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
	r.trueNASHandlers = &TrueNASHandlers{
		getPersistence: r.configHandlers.getPersistence,
		getConfig:      r.configHandlers.getConfig,
		getMonitor:     r.configHandlers.getMonitor,
	}
	recoveryManager := recoverymanager.New(r.multiTenant)
	r.recoveryHandlers = NewRecoveryHandlers(recoveryManager)
	if r.mtMonitor != nil {
		r.mtMonitor.SetRecoveryManager(recoveryManager)
	}
	if r.monitor != nil {
		r.monitor.SetRecoveryManager(recoveryManager)
	}
	r.trueNASPoller = monitoring.NewTrueNASPoller(r.resourceRegistry, r.multiTenant, 0, recoveryManager)
	r.trueNASPoller.Start(r.lifecycleCtx)
	updateHandlers := NewUpdateHandlersWithContext(r.updateManager, r.updateHistory, r.lifecycleCtx)
	r.dockerAgentHandlers = NewDockerAgentHandlers(r.mtMonitor, r.monitor, r.wsHub, r.config)
	r.kubernetesAgentHandlers = NewKubernetesAgentHandlers(r.mtMonitor, r.monitor, r.wsHub)
	r.hostAgentHandlers = NewHostAgentHandlers(r.mtMonitor, r.monitor, r.wsHub)
	r.kubernetesAgentHandlers.SetRecoveryIngestor(r.recoveryHandlers)
	r.resourceHandlers = NewResourceHandlers(r.config)
	if mock.IsMockEnabled() {
		truenas.SetFeatureEnabled(true)
		mockTrueNASProvider := truenas.NewDefaultProvider()
		r.resourceHandlers.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, trueNASRecordsAdapter{provider: mockTrueNASProvider})
	} else if r.trueNASPoller != nil {
		r.resourceHandlers.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, r.trueNASPoller)
	}
	r.configProfileHandler = NewConfigProfileHandler(r.multiTenant)
	r.licenseHandlers = NewLicenseHandlers(r.multiTenant, r.hostedMode)
	rbacProvider := NewTenantRBACProvider(r.config.DataPath)
	r.rbacProvider = rbacProvider
	orgHandlers := NewOrgHandlers(r.multiTenant, r.mtMonitor, rbacProvider)
	// Wire license service provider so middleware can access per-tenant license services
	SetLicenseServiceProvider(r.licenseHandlers)
	r.reportingHandlers = NewReportingHandlers(r.mtMonitor, r.resourceRegistry, recoveryManager)
	r.logHandlers = NewLogHandlers(r.config, r.persistence)
	rbacHandlers := NewRBACHandlers(r.config, rbacProvider)
	var magicLinkService *MagicLinkService
	var magicLinkHandlers *MagicLinkHandlers
	if r.hostedMode {
		svc, err := NewMagicLinkServiceForDataPath(r.config.DataPath, nil)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize magic link service")
		} else {
			magicLinkService = svc
		}
		magicLinkHandlers = NewMagicLinkHandlers(r.multiTenant, magicLinkService, r.hostedMode, r.resolvePublicURL)
	}

	hostedSignupHandlers := NewHostedSignupHandlers(r.multiTenant, rbacProvider, magicLinkService, r.resolvePublicURL, r.hostedMode)
	stripeWebhookHandlers := NewStripeWebhookHandlers(
		config.NewFileBillingStore(r.config.DataPath),
		r.multiTenant,
		rbacProvider,
		magicLinkService,
		r.resolvePublicURL,
		r.hostedMode,
		r.config.DataPath,
	)
	infraUpdateHandlers := NewUpdateDetectionHandlers(r.monitor)
	auditHandlers := NewAuditHandlers()

	// System settings and API token management
	r.systemSettingsHandler = NewSystemSettingsHandler(r.config, r.persistence, r.wsHub, r.mtMonitor, r.monitor, r.reloadSystemSettings, r.reloadFunc)

	// Agent execution server for AI tool use
	r.agentExecServer = agentexec.NewServer(func(token string, agentID string) bool {
		// Validate agent tokens using the API tokens system with scope check
		if r.config == nil {
			return false
		}
		// Check the new API tokens system with scope validation
		if record, ok := r.config.ValidateAPIToken(token); ok {
			// SECURITY: Require agent:exec scope for WebSocket connections
			if !record.HasScope(config.ScopeAgentExec) {
				log.Warn().
					Str("token_id", record.ID).
					Msg("Agent exec token missing required scope: agent:exec")
				return false
			}

			// SECURITY: Check if token is bound to a specific agent
			if boundID, ok := record.Metadata["bound_agent_id"]; ok && boundID != "" {
				if boundID != agentID {
					log.Warn().
						Str("token_id", record.ID).
						Str("bound_id", boundID).
						Str("requested_id", agentID).
						Msg("Agent token mismatch: token is bound to a different agent ID")
					return false
				}
			}

			return true
		}
		// Fall back to legacy single token if set (legacy tokens have wildcard access)
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
		r.aiSettingsHandler.SetReadState(r.resourceRegistry)
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
	// Inject unified resource provider for AI context and routing.
	if r.aiUnifiedAdapter != nil {
		r.aiSettingsHandler.SetUnifiedResourceProvider(r.aiUnifiedAdapter)
	} else {
		log.Warn().Msg("[Router] aiUnifiedAdapter is nil, cannot inject unified resource provider")
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
	r.aiHandler.SetReadState(r.resourceRegistry)
	r.aiHandler.SetRecoveryManager(recoveryManager)

	// AI-powered infrastructure discovery handlers
	// Note: The actual service is wired up later via SetDiscoveryService
	r.discoveryHandlers = NewDiscoveryHandlers(nil, r.config)

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
	// Wire chat handler to AI settings handler for investigation orchestration
	r.aiSettingsHandler.SetChatHandler(r.aiHandler)
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

	// Initialize recovery token store
	InitRecoveryTokenStore(r.config.DataPath)

	r.registerPublicAndAuthRoutes()
	r.registerMonitoringRoutes(guestMetadataHandler, dockerMetadataHandler, hostMetadataHandler, infraUpdateHandlers)
	r.registerConfigSystemRoutes(updateHandlers)
	r.registerAIRelayRoutes()
	r.registerOrgLicenseRoutes(orgHandlers, rbacHandlers, auditHandlers)
	r.registerHostedRoutes(hostedSignupHandlers, magicLinkHandlers, stripeWebhookHandlers)

	// Note: Frontend handler is handled manually in ServeHTTP to prevent redirect issues
	// See issue #334 - ServeMux redirects empty path to "./" which breaks reverse proxies
}

// CleanupTenant removes all per-tenant resources (RBAC, AI, License) for a deleted org.
func (r *Router) CleanupTenant(ctx context.Context, orgID string) error {
	var errs []error

	if r.rbacProvider != nil {
		if err := r.rbacProvider.RemoveTenant(orgID); err != nil {
			errs = append(errs, fmt.Errorf("rbac cleanup: %w", err))
		}
	}

	if r.aiHandler != nil {
		if err := r.aiHandler.RemoveTenantService(ctx, orgID); err != nil {
			errs = append(errs, fmt.Errorf("ai cleanup: %w", err))
		}
	}

	if r.licenseHandlers != nil {
		r.licenseHandlers.RemoveTenantService(orgID)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// RemoveTenantService removes the cached license service for a deleted org.
func (h *LicenseHandlers) RemoveTenantService(orgID string) {
	h.services.Delete(orgID)
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

	// Check setup token first (for setup scripts)
	if r.isValidSetupTokenForRequest(req) {
		r.configHandlers.HandleVerifyTemperatureSSH(w, req)
		return
	}

	// Require authentication
	if !CheckAuth(r.config, w, req) {
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
		return
	}

	// Check admin privileges for proxy auth users
	if r.config.ProxyAuthSecret != "" {
		if valid, username, isAdmin := CheckProxyAuth(r.config, req); valid && !isAdmin {
			log.Warn().
				Str("ip", GetClientIP(req)).
				Str("username", username).
				Msg("Non-admin user attempted verify-temperature-ssh")
			http.Error(w, "Admin privileges required", http.StatusForbidden)
			return
		}
	}

	// Require settings:write scope for API tokens (SSH probes are a privileged operation)
	if !ensureScope(w, req, config.ScopeSettingsWrite) {
		return
	}

	r.configHandlers.HandleVerifyTemperatureSSH(w, req)
}

// handleSSHConfig handles SSH config writes with setup token or API auth
func (r *Router) handleSSHConfig(w http.ResponseWriter, req *http.Request) {
	if r.systemSettingsHandler == nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Check setup token first (for setup scripts)
	if r.isValidSetupTokenForRequest(req) {
		r.systemSettingsHandler.HandleSSHConfig(w, req)
		return
	}

	// Require authentication
	if !CheckAuth(r.config, w, req) {
		log.Warn().
			Str("ip", req.RemoteAddr).
			Str("path", req.URL.Path).
			Str("method", req.Method).
			Msg("Unauthorized access attempt (ssh-config)")

		if strings.HasPrefix(req.URL.Path, "/api/") || strings.Contains(req.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Authentication required"}`))
		} else {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Check admin privileges for proxy auth users
	if r.config.ProxyAuthSecret != "" {
		if valid, username, isAdmin := CheckProxyAuth(r.config, req); valid && !isAdmin {
			log.Warn().
				Str("ip", GetClientIP(req)).
				Str("username", username).
				Msg("Non-admin user attempted ssh-config update")
			http.Error(w, "Admin privileges required", http.StatusForbidden)
			return
		}
	}

	// Require settings:write scope for API tokens (SSH config writes are a privileged operation)
	if !ensureScope(w, req, config.ScopeSettingsWrite) {
		return
	}

	r.systemSettingsHandler.HandleSSHConfig(w, req)
}

// handleSSHConfigUnauthorized logs an unauthorized access attempt (legacy helper, no longer used)
func (r *Router) handleSSHConfigUnauthorized(w http.ResponseWriter, req *http.Request) {
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

func (r *Router) isValidSetupTokenForRequest(req *http.Request) bool {
	if r == nil || r.configHandlers == nil || req == nil {
		return false
	}

	token := extractSetupToken(req)
	if token == "" {
		return false
	}

	requestOrgID := resolveTenantOrgID(req)
	if !isValidOrganizationID(requestOrgID) {
		return false
	}

	return r.configHandlers.ValidateSetupTokenForOrg(token, requestOrgID)
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
		// Inject unified resource adapter for polling optimization
		if r.monitorResourceAdapter != nil {
			log.Debug().Msg("[Router] Injecting unified resource adapter into monitor")
			m.SetResourceStore(r.monitorResourceAdapter)
		} else {
			log.Warn().Msg("[Router] monitorResourceAdapter is nil, cannot inject resource store")
		}
		if r.resourceHandlers != nil {
			r.resourceHandlers.SetStateProvider(m)
		}

		// Set state provider on AI handler so patrol service gets created
		// (Critical: patrol service is created lazily in SetStateProvider)
		if r.aiSettingsHandler != nil {
			r.aiSettingsHandler.SetStateProvider(m)
			r.aiSettingsHandler.SetReadState(r.resourceRegistry)
			// Also inject alert provider and resolver now that monitor is available
			if alertManager := m.GetAlertManager(); alertManager != nil {
				alertAdapter := ai.NewAlertManagerAdapter(alertManager)
				r.aiSettingsHandler.SetAlertProvider(alertAdapter)
				r.aiSettingsHandler.SetAlertResolver(alertAdapter)
			}
			if incidentStore := m.GetIncidentStore(); incidentStore != nil {
				r.aiSettingsHandler.SetIncidentStore(incidentStore)
			}
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

// SetUnifiedResourceProvider forwards the unified-resource-native provider to active AI services.
func (h *AISettingsHandler) SetUnifiedResourceProvider(urp ai.UnifiedResourceProvider) {
	if h == nil {
		return
	}
	h.unifiedResourceProvider = urp

	if h.legacyAIService != nil {
		h.legacyAIService.SetUnifiedResourceProvider(urp)
	}

	h.aiServicesMu.Lock()
	defer h.aiServicesMu.Unlock()
	for _, svc := range h.aiServices {
		svc.SetUnifiedResourceProvider(urp)
	}
}

// getTenantMonitor returns the appropriate monitor for the current request's tenant.
// It extracts the org ID from the request context and returns the corresponding monitor.
// Falls back to the default monitor if multi-tenant is not configured or on error.
func (r *Router) getTenantMonitor(ctx context.Context) *monitoring.Monitor {
	// Get org ID from context
	orgID := GetOrgID(ctx)

	// If multi-tenant monitor is configured, get the tenant-specific monitor
	if r.mtMonitor != nil && orgID != "" {
		monitor, err := r.mtMonitor.GetMonitor(orgID)
		if err != nil {
			log.Warn().
				Err(err).
				Str("org_id", orgID).
				Msg("Failed to get tenant monitor, falling back to default")
			return r.monitor
		}
		if monitor != nil {
			return monitor
		}
	}

	// Fall back to the default monitor
	return r.monitor
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

// SetDiscoveryService sets the discovery service for the router.
func (r *Router) SetDiscoveryService(svc *servicediscovery.Service) {
	if r.discoveryHandlers != nil {
		r.discoveryHandlers.SetService(svc)
	}

	// Wire up WebSocket hub for progress broadcasting
	if svc != nil && r.wsHub != nil {
		svc.SetWSHub(&wsHubAdapter{hub: r.wsHub})
		log.Info().Msg("Discovery: WebSocket hub wired for progress broadcasting")
	}
}

// SetDiscoveryAIConfigProvider sets the AI config provider for showing AI provider info in discovery.
func (r *Router) SetDiscoveryAIConfigProvider(provider AIConfigProvider) {
	if r.discoveryHandlers != nil {
		r.discoveryHandlers.SetAIConfigProvider(provider)
	}
}

// wsHubAdapter adapts websocket.Hub to the servicediscovery.WSBroadcaster interface.
type wsHubAdapter struct {
	hub *websocket.Hub
}

// BroadcastDiscoveryProgress broadcasts discovery progress to all WebSocket clients.
func (a *wsHubAdapter) BroadcastDiscoveryProgress(progress *servicediscovery.DiscoveryProgress) {
	if a.hub == nil || progress == nil {
		return
	}
	a.hub.BroadcastMessage(websocket.Message{
		Type: "ai_discovery_progress",
		Data: progress,
	})
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

		// Initialize new AI intelligence services (Phase 6)
		r.initializeAIIntelligenceServices(ctx, dataDir)

		// Wire unified finding callback AFTER initializeAIIntelligenceServices
		// (unified store is created there) and AFTER findings persistence is loaded
		patrol := r.aiSettingsHandler.GetAIService(ctx).GetPatrolService()
		if patrol != nil {
			if unifiedStore := r.aiSettingsHandler.GetUnifiedStore(); unifiedStore != nil {
				toUnifiedLifecycle := func(events []ai.FindingLifecycleEvent) []unified.UnifiedFindingLifecycleEvent {
					if len(events) == 0 {
						return nil
					}
					out := make([]unified.UnifiedFindingLifecycleEvent, 0, len(events))
					for _, e := range events {
						out = append(out, unified.UnifiedFindingLifecycleEvent{
							At:       e.At,
							Type:     e.Type,
							Message:  e.Message,
							From:     e.From,
							To:       e.To,
							Metadata: e.Metadata,
						})
					}
					return out
				}
				patrol.SetUnifiedFindingCallback(func(f *ai.Finding) bool {
					// Convert ai.Finding to unified.UnifiedFinding
					uf := &unified.UnifiedFinding{
						ID:                     f.ID,
						Source:                 unified.SourceAIPatrol,
						Severity:               unified.UnifiedSeverity(f.Severity),
						Category:               unified.UnifiedCategory(f.Category),
						ResourceID:             f.ResourceID,
						ResourceName:           f.ResourceName,
						ResourceType:           f.ResourceType,
						Node:                   f.Node,
						Title:                  f.Title,
						Description:            f.Description,
						Recommendation:         f.Recommendation,
						Evidence:               f.Evidence,
						DetectedAt:             f.DetectedAt,
						LastSeenAt:             f.LastSeenAt,
						ResolvedAt:             f.ResolvedAt,
						InvestigationSessionID: f.InvestigationSessionID,
						InvestigationStatus:    f.InvestigationStatus,
						InvestigationOutcome:   f.InvestigationOutcome,
						LastInvestigatedAt:     f.LastInvestigatedAt,
						InvestigationAttempts:  f.InvestigationAttempts,
						LoopState:              f.LoopState,
						Lifecycle:              toUnifiedLifecycle(f.Lifecycle),
						RegressionCount:        f.RegressionCount,
						LastRegressionAt:       f.LastRegressionAt,
						AcknowledgedAt:         f.AcknowledgedAt,
						SnoozedUntil:           f.SnoozedUntil,
						DismissedReason:        f.DismissedReason,
						UserNote:               f.UserNote,
						Suppressed:             f.Suppressed,
						TimesRaised:            f.TimesRaised,
					}
					_, isNew := unifiedStore.AddFromAI(uf)
					return isNew
				})
				patrol.SetUnifiedFindingResolver(func(findingID string) {
					unifiedStore.Resolve(findingID)
				})

				// Wire push notifications: patrol findings → relay client (best-effort)
				patrol.SetPushNotifyCallback(func(n relay.PushNotificationPayload) {
					r.relayMu.RLock()
					client := r.relayClient
					r.relayMu.RUnlock()
					if client != nil {
						if err := client.SendPushNotification(n); err != nil {
							log.Debug().Err(err).Str("type", n.Type).Msg("Push notification send failed")
						}
					}
				})

				log.Info().Msg("AI Intelligence: Patrol findings wired to unified store")

				// Sync existing findings from persistence to the unified store
				// (findings loaded from disk before the callback was set)
				existingFindings := patrol.GetFindingsHistory(nil)
				if len(existingFindings) > 0 {
					for _, f := range existingFindings {
						if f == nil {
							continue
						}
						uf := &unified.UnifiedFinding{
							ID:                     f.ID,
							Source:                 unified.SourceAIPatrol,
							Severity:               unified.UnifiedSeverity(f.Severity),
							Category:               unified.UnifiedCategory(f.Category),
							ResourceID:             f.ResourceID,
							ResourceName:           f.ResourceName,
							ResourceType:           f.ResourceType,
							Node:                   f.Node,
							Title:                  f.Title,
							Description:            f.Description,
							Recommendation:         f.Recommendation,
							Evidence:               f.Evidence,
							DetectedAt:             f.DetectedAt,
							LastSeenAt:             f.LastSeenAt,
							ResolvedAt:             f.ResolvedAt,
							InvestigationSessionID: f.InvestigationSessionID,
							InvestigationStatus:    f.InvestigationStatus,
							InvestigationOutcome:   f.InvestigationOutcome,
							LastInvestigatedAt:     f.LastInvestigatedAt,
							InvestigationAttempts:  f.InvestigationAttempts,
							LoopState:              f.LoopState,
							Lifecycle:              toUnifiedLifecycle(f.Lifecycle),
							RegressionCount:        f.RegressionCount,
							LastRegressionAt:       f.LastRegressionAt,
							AcknowledgedAt:         f.AcknowledgedAt,
							SnoozedUntil:           f.SnoozedUntil,
							DismissedReason:        f.DismissedReason,
							UserNote:               f.UserNote,
							Suppressed:             f.Suppressed,
							TimesRaised:            f.TimesRaised,
						}
						// Copy resolution timestamp if resolved
						if f.ResolvedAt != nil || f.AutoResolved {
							now := time.Now()
							if f.ResolvedAt != nil {
								uf.ResolvedAt = f.ResolvedAt
							} else {
								uf.ResolvedAt = &now
							}
						}
						unifiedStore.AddFromAI(uf)
					}
					log.Info().Int("count", len(existingFindings)).Msg("AI Intelligence: Synced existing patrol findings to unified store")
				}

				// Wire unified store for "Discuss with Assistant" finding context lookup
				r.aiHandler.SetUnifiedStore(unifiedStore)
			}
		}

		// Finally start the actual patrol loop
		r.aiSettingsHandler.StartPatrol(ctx)

		// Wire up discovery service to the handlers
		// This enables the /api/discovery endpoints to trigger discovery scans
		aiService := r.aiSettingsHandler.GetAIService(ctx)
		if aiService != nil {
			if discoveryService := aiService.GetDiscoveryService(); discoveryService != nil {
				r.SetDiscoveryService(discoveryService)
				log.Info().Msg("Discovery: Service wired to API handlers")
			}
			// Wire up AI config provider for showing AI provider info in discovery UI
			r.SetDiscoveryAIConfigProvider(aiService)
		}
	}
}

// initializeAIIntelligenceServices sets up the new AI intelligence subsystems
func (r *Router) initializeAIIntelligenceServices(ctx context.Context, dataDir string) {
	// Only initialize if AI is enabled
	if !r.aiSettingsHandler.IsAIEnabled(ctx) {
		return
	}

	// 1. Initialize circuit breaker for resilient patrol
	circuitBreaker := circuit.NewBreaker("patrol", circuit.DefaultConfig())
	r.aiSettingsHandler.SetCircuitBreaker(circuitBreaker)
	log.Info().Msg("AI Intelligence: Circuit breaker initialized")

	// 2. Initialize learning store for feedback learning
	learningCfg := learning.LearningStoreConfig{
		DataDir: dataDir,
	}
	learningStore := learning.NewLearningStore(learningCfg)
	r.aiSettingsHandler.SetLearningStore(learningStore)
	log.Info().Msg("AI Intelligence: Learning store initialized")

	// 4. Initialize forecast service for trend forecasting
	forecastCfg := forecast.DefaultForecastConfig()
	forecastService := forecast.NewService(forecastCfg)
	// Wire up data provider adapter
	if r.monitor != nil {
		if metricsHistory := r.monitor.GetMetricsHistory(); metricsHistory != nil {
			dataAdapter := adapters.NewForecastDataAdapter(metricsHistory)
			if dataAdapter != nil {
				forecastService.SetDataProvider(dataAdapter)
			}
		}
	}
	// Wire up state provider for forecast context
	if r.monitor != nil {
		forecastStateAdapter := &forecastStateProviderWrapper{monitor: r.monitor}
		forecastService.SetStateProvider(forecastStateAdapter)
	}
	r.aiSettingsHandler.SetForecastService(forecastService)
	log.Info().Msg("AI Intelligence: Forecast service initialized")

	// 5. Initialize Proxmox event correlator
	proxmoxCfg := proxmox.DefaultEventCorrelatorConfig()
	proxmoxCfg.DataDir = dataDir
	proxmoxCorrelator := proxmox.NewEventCorrelator(proxmoxCfg)
	r.aiSettingsHandler.SetProxmoxCorrelator(proxmoxCorrelator)
	log.Info().Msg("AI Intelligence: Proxmox event correlator initialized")

	// 7. Initialize remediation engine for AI-guided fixes
	remediationCfg := remediation.DefaultEngineConfig()
	remediationCfg.DataDir = dataDir
	remediationEngine := remediation.NewEngine(remediationCfg)
	// Wire up command executor (disabled by default for safety)
	cmdExecutor := adapters.NewCommandExecutorAdapter()
	remediationEngine.SetCommandExecutor(cmdExecutor)
	r.aiSettingsHandler.SetRemediationEngine(remediationEngine)
	log.Info().Msg("AI Intelligence: Remediation engine initialized (command execution disabled)")

	// 8. Initialize unified alert/finding system and bridge
	if r.monitor != nil {
		if alertManager := r.monitor.GetAlertManager(); alertManager != nil {
			// Create unified store
			unifiedStore := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
			r.aiSettingsHandler.SetUnifiedStore(unifiedStore)

			// Create alert bridge
			alertBridge := unified.NewAlertBridge(unifiedStore, unified.DefaultBridgeConfig())

			// Create and set alert provider adapter
			alertAdapter := unified.NewAlertManagerAdapter(alertManager)
			alertBridge.SetAlertProvider(alertAdapter)

			// Set patrol trigger function (triggers mini-patrol on alert events)
			patrol := r.aiSettingsHandler.GetAIService(ctx).GetPatrolService()
			if patrol != nil {
				alertBridge.SetPatrolTrigger(func(resourceID, resourceType, reason, alertType string) {
					scope := ai.PatrolScope{
						ResourceIDs:   []string{resourceID},
						ResourceTypes: []string{resourceType},
						Depth:         ai.PatrolDepthQuick,
						Context:       "Alert bridge: " + reason,
						Priority:      50,
					}
					switch reason {
					case "alert_fired":
						scope.Reason = ai.TriggerReasonAlertFired
						scope.Priority = 80
						if alertType != "" {
							scope.Context = "Alert: " + alertType
						}
					case "alert_cleared":
						scope.Reason = ai.TriggerReasonAlertCleared
						scope.Priority = 40
						if alertType != "" {
							scope.Context = "Alert cleared: " + alertType
						}
					default:
						scope.Reason = ai.TriggerReasonManual
					}

					log.Debug().
						Str("resource_id", resourceID).
						Str("reason", reason).
						Msg("Alert bridge: Triggering mini-patrol")
					if triggerManager := r.aiSettingsHandler.GetTriggerManager(); triggerManager != nil {
						if triggerManager.TriggerPatrol(scope) {
							log.Debug().
								Str("resource_id", resourceID).
								Str("reason", reason).
								Msg("Alert bridge: Queued patrol via trigger manager")
						} else {
							log.Warn().
								Str("resource_id", resourceID).
								Str("reason", reason).
								Msg("Alert bridge: Patrol trigger rejected by trigger manager")
						}
						return
					}

					patrol.TriggerScopedPatrol(context.Background(), scope)
				})
			}

			// Start the bridge
			alertBridge.Start()
			r.aiSettingsHandler.SetAlertBridge(alertBridge)
			log.Info().Msg("AI Intelligence: Unified alert/finding bridge initialized and started")
		}
	}

	// 9. Wire up AI intelligence providers to patrol service for context injection
	patrol := r.aiSettingsHandler.GetAIService(ctx).GetPatrolService()
	if patrol != nil {
		// Wire learning store for user preference context
		if learningStore != nil {
			patrol.SetLearningProvider(learningStore)
		}

		// Wire proxmox correlator for operations context
		if proxmoxCorrelator != nil {
			patrol.SetProxmoxEventProvider(proxmoxCorrelator)
		}

		// Wire forecast service for trend predictions
		if forecastService != nil {
			patrol.SetForecastProvider(forecastService)
		}

		// Wire remediation engine for auto-generating fix plans from findings
		if remediationEngine != nil {
			patrol.SetRemediationEngine(remediationEngine)
		}

		// Wire guest prober for pre-patrol reachability checks via host agents
		if r.agentExecServer != nil {
			patrol.SetGuestProber(ai.NewAgentExecProber(r.agentExecServer))
		}

		// NOTE: Unified finding callback is wired in StartPatrol after findings persistence is loaded

		log.Info().Msg("AI Intelligence: Patrol context providers wired up")
	}

	// 10. Initialize event-driven patrol trigger manager (Phase 7)
	if patrol != nil {
		triggerManager := ai.NewTriggerManager(ai.DefaultTriggerManagerConfig())

		// Set the patrol executor callback
		triggerManager.SetOnTrigger(func(ctx context.Context, scope ai.PatrolScope) {
			patrol.TriggerScopedPatrol(ctx, scope)
		})

		// Start the trigger manager
		triggerManager.Start(ctx)

		// Wire to patrol service
		patrol.SetTriggerManager(triggerManager)

		// Store reference for shutdown and alert callbacks
		r.aiSettingsHandler.SetTriggerManager(triggerManager)

		// 11. Wire baseline anomaly callback to TriggerManager
		if baselineStore := patrol.GetBaselineStore(); baselineStore != nil {
			baselineStore.SetAnomalyCallback(func(resourceID, resourceType, metric string, severity baseline.AnomalySeverity, value, baselineValue float64) {
				// Only trigger for significant anomalies (high or critical)
				if severity == baseline.AnomalyHigh || severity == baseline.AnomalyCritical {
					scope := ai.AnomalyTriggeredPatrolScope(
						resourceID,
						resourceType,
						metric,
						string(severity),
					)
					if triggerManager.TriggerPatrol(scope) {
						log.Debug().
							Str("resourceID", resourceID).
							Str("metric", metric).
							Str("severity", string(severity)).
							Msg("Anomaly triggered mini-patrol via TriggerManager")
					}
				}
			})
			log.Info().Msg("AI Intelligence: Baseline anomaly callback wired to trigger manager")
		}

		log.Info().Msg("AI Intelligence: Event-driven trigger manager initialized and started")
	}

	// 12. Initialize incident coordinator for high-frequency recording
	if patrol != nil {
		incidentCoordinator := ai.NewIncidentCoordinator(ai.DefaultIncidentCoordinatorConfig())

		// Wire the incident store if available
		if incidentStore := patrol.GetIncidentStore(); incidentStore != nil {
			incidentCoordinator.SetIncidentStore(incidentStore)
		}

		// Create metrics adapter for incident recorder
		var metricsAdapter *adapters.MetricsAdapter
		if stateProvider := r.aiSettingsHandler.GetStateProvider(); stateProvider != nil {
			metricsAdapter = adapters.NewMetricsAdapter(stateProvider)
		}

		// Initialize and wire the incident recorder (high-frequency metrics)
		if metricsAdapter != nil {
			recorderCfg := metrics.DefaultIncidentRecorderConfig()
			recorderCfg.DataDir = dataDir
			recorder := metrics.NewIncidentRecorder(recorderCfg)
			recorder.SetMetricsProvider(metricsAdapter)
			recorder.Start()
			incidentCoordinator.SetRecorder(recorder)
			r.aiSettingsHandler.SetIncidentRecorder(recorder)
			log.Info().Msg("AI Intelligence: Incident recorder initialized and started")
		}

		// Start the coordinator
		incidentCoordinator.Start()

		// Store reference
		r.aiSettingsHandler.SetIncidentCoordinator(incidentCoordinator)

		log.Info().Msg("AI Intelligence: Incident coordinator initialized and started")
	}

	log.Info().Msg("AI Intelligence: All Phase 6 & 7 services initialized successfully")
}

// StopPatrol stops the AI patrol service
func (r *Router) StopPatrol() {
	if r.aiSettingsHandler != nil {
		r.aiSettingsHandler.StopPatrol()
	}
}

// ShutdownAIIntelligence gracefully shuts down all AI intelligence services (Phase 6)
// This should be called during application shutdown to ensure proper cleanup
func (r *Router) ShutdownAIIntelligence() {
	r.shutdownBackgroundWorkers()

	if r.aiSettingsHandler == nil {
		return
	}

	log.Info().Msg("AI Intelligence: Starting graceful shutdown")

	// 1. Stop alert bridge (stop listening for alert events)
	if alertBridge := r.aiSettingsHandler.GetAlertBridge(); alertBridge != nil {
		alertBridge.Stop()
		log.Debug().Msg("AI Intelligence: Alert bridge stopped")
	}

	// 2. Stop patrol service for all tenants (waits for in-flight investigations, force-saves state)
	// Use StopPatrol() which stops patrol for both legacy and all tenant services
	r.aiSettingsHandler.StopPatrol()
	log.Debug().Msg("AI Intelligence: All patrol services stopped")

	// 3. Stop trigger manager (stop event-driven patrol scheduling)
	if triggerManager := r.aiSettingsHandler.GetTriggerManager(); triggerManager != nil {
		triggerManager.Stop()
		log.Debug().Msg("AI Intelligence: Trigger manager stopped")
	}

	// 4. Stop incident coordinator (stop high-frequency recording)
	if incidentCoordinator := r.aiSettingsHandler.GetIncidentCoordinator(); incidentCoordinator != nil {
		incidentCoordinator.Stop()
		log.Debug().Msg("AI Intelligence: Incident coordinator stopped")
	}

	// 4b. Stop incident recorder (stops background sampling)
	if incidentRecorder := r.aiSettingsHandler.GetIncidentRecorder(); incidentRecorder != nil {
		incidentRecorder.Stop()
		log.Debug().Msg("AI Intelligence: Incident recorder stopped")
	}

	// 5. Cleanup learning store (removes old records, persists if data dir configured)
	if learningStore := r.aiSettingsHandler.GetLearningStore(); learningStore != nil {
		learningStore.Cleanup()
		log.Debug().Msg("AI Intelligence: Learning store cleaned up")
	}

	log.Info().Msg("AI Intelligence: Graceful shutdown complete")
}

func (r *Router) shutdownBackgroundWorkers() {
	if r.lifecycleCancel != nil {
		r.lifecycleCancel()
	}
	if r.trueNASPoller != nil {
		r.trueNASPoller.Stop()
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

	// Wire chat service to AI service for patrol and investigation
	r.wireChatServiceToAI()

	// Wire up investigation orchestrator now that chat service is ready
	// This must happen after Start() because the orchestrator needs the chat service
	if r.aiSettingsHandler != nil {
		r.aiSettingsHandler.WireOrchestratorAfterChatStart()
	}

	// Wire circuit breaker for patrol if AI is running
	if r.aiHandler != nil && r.aiHandler.IsRunning(context.Background()) {
		if r.aiSettingsHandler != nil {
			if patrolSvc := r.aiSettingsHandler.GetAIService(context.Background()).GetPatrolService(); patrolSvc != nil {
				// Wire circuit breaker for resilient AI API calls
				if breaker := r.aiSettingsHandler.GetCircuitBreaker(); breaker != nil {
					patrolSvc.SetCircuitBreaker(breaker)
					log.Info().Msg("AI patrol circuit breaker wired")
				}
			}
		}
	}
}

// wireChatServiceToAI wires the chat service adapter to the AI service,
// enabling patrol and investigation to use the chat service's execution path
// (50+ MCP tools, FSM safety, sessions) instead of the legacy 3-tool path.
func (r *Router) wireChatServiceToAI() {
	if r.aiHandler == nil || r.aiSettingsHandler == nil {
		return
	}

	// Use default org context for legacy service wiring
	// Multi-tenant orgs get their services wired via setupInvestigationOrchestrator
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	chatSvc := r.aiHandler.GetService(ctx)
	if chatSvc == nil {
		return
	}

	chatService, ok := chatSvc.(*chat.Service)
	if !ok {
		log.Warn().Msg("Chat service is not *chat.Service, cannot create patrol adapter")
		return
	}

	aiService := r.aiSettingsHandler.GetAIService(ctx)
	if aiService == nil {
		return
	}

	aiService.SetChatService(&chatServiceAdapter{svc: chatService})

	// Wire mid-run budget enforcement from AI service to chat service
	chatService.SetBudgetChecker(func() error {
		return aiService.CheckBudget("patrol")
	})

	log.Info().Msg("Chat service wired to AI service for patrol and investigation")
}

// wireAIChatProviders wires up all MCP tool providers for AI chat
func (r *Router) wireAIChatProviders() {
	if r.aiHandler == nil || !r.aiHandler.IsRunning(context.Background()) {
		return
	}

	// Use default org context for legacy service wiring
	service := r.aiHandler.GetService(context.WithValue(context.Background(), OrgIDContextKey, "default"))
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

	// Wire findings provider from patrol service (default org for legacy wiring)
	defaultOrgCtx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	if r.aiSettingsHandler != nil {
		if patrolSvc := r.aiSettingsHandler.GetAIService(defaultOrgCtx).GetPatrolService(); patrolSvc != nil {
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

	// Wire guest config provider (storage provider wiring removed)
	if r.monitor != nil {
		guestConfigAdapter := tools.NewGuestConfigMCPAdapter(r.monitor)
		if guestConfigAdapter != nil {
			service.SetGuestConfigProvider(guestConfigAdapter)
			log.Debug().Msg("AI chat: Guest config provider wired")
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

	// Wire baseline provider (default org for legacy wiring)
	if r.aiSettingsHandler != nil {
		if patrolSvc := r.aiSettingsHandler.GetAIService(defaultOrgCtx).GetPatrolService(); patrolSvc != nil {
			if baselineStore := patrolSvc.GetBaselineStore(); baselineStore != nil {
				baselineAdapter := tools.NewBaselineMCPAdapter(&baselineSourceWrapper{store: baselineStore})
				if baselineAdapter != nil {
					service.SetBaselineProvider(baselineAdapter)
					log.Debug().Msg("AI chat: Baseline provider wired")
				}
			}
		}
	}

	// Wire pattern provider (default org for legacy wiring)
	if r.aiSettingsHandler != nil {
		if patrolSvc := r.aiSettingsHandler.GetAIService(defaultOrgCtx).GetPatrolService(); patrolSvc != nil {
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

	// Wire findings manager (default org for legacy wiring)
	if r.aiSettingsHandler != nil {
		if patrolSvc := r.aiSettingsHandler.GetAIService(defaultOrgCtx).GetPatrolService(); patrolSvc != nil {
			findingsManagerAdapter := tools.NewFindingsManagerMCPAdapter(patrolSvc)
			if findingsManagerAdapter != nil {
				service.SetFindingsManager(findingsManagerAdapter)
				log.Debug().Msg("AI chat: Findings manager wired")
			}
		}
	}

	// Wire metadata updater (default org for legacy wiring)
	if r.aiSettingsHandler != nil {
		metadataAdapter := tools.NewMetadataUpdaterMCPAdapter(r.aiSettingsHandler.GetAIService(defaultOrgCtx))
		if metadataAdapter != nil {
			service.SetMetadataUpdater(metadataAdapter)
			log.Debug().Msg("AI chat: Metadata updater wired")
		}
	}

	// Wire intelligence providers for MCP tools
	// - IncidentRecorderProvider: high-frequency incident data (pulse_get_incident_window)
	// - EventCorrelatorProvider: Proxmox events (pulse_correlate_events)
	// - TopologyProvider: relationship graph (pulse_get_relationship_graph)
	// - KnowledgeStoreProvider: notes (pulse_remember, pulse_recall)

	// Wire incident recorder provider (high-frequency incident data)
	if r.aiSettingsHandler != nil {
		if recorder := r.aiSettingsHandler.GetIncidentRecorder(); recorder != nil {
			service.SetIncidentRecorderProvider(&incidentRecorderProviderWrapper{recorder: recorder})
			log.Debug().Msg("AI chat: Incident recorder provider wired")
		}
	}

	// Wire event correlator provider (Proxmox events)
	if r.aiSettingsHandler != nil {
		if correlator := r.aiSettingsHandler.GetProxmoxCorrelator(); correlator != nil {
			service.SetEventCorrelatorProvider(&eventCorrelatorProviderWrapper{correlator: correlator})
			log.Debug().Msg("AI chat: Event correlator provider wired")
		}
	}

	// Wire knowledge store provider for notes (pulse_remember, pulse_recall) (default org for legacy wiring)
	if r.aiSettingsHandler != nil {
		if aiSvc := r.aiSettingsHandler.GetAIService(defaultOrgCtx); aiSvc != nil {
			if patrolSvc := aiSvc.GetPatrolService(); patrolSvc != nil {
				if knowledgeStore := patrolSvc.GetKnowledgeStore(); knowledgeStore != nil {
					service.SetKnowledgeStoreProvider(&knowledgeStoreProviderWrapper{store: knowledgeStore})
					log.Debug().Msg("AI chat: Knowledge store provider wired")
				}
			}
		}
	}

	// Wire discovery provider for AI-powered infrastructure discovery (pulse_get_discovery, pulse_list_discoveries) (default org for legacy wiring)
	if r.aiSettingsHandler != nil {
		if aiSvc := r.aiSettingsHandler.GetAIService(defaultOrgCtx); aiSvc != nil {
			if discoverySvc := aiSvc.GetDiscoveryService(); discoverySvc != nil {
				adapter := servicediscovery.NewToolsAdapter(discoverySvc)
				if adapter != nil {
					service.SetDiscoveryProvider(tools.NewDiscoveryMCPAdapter(adapter))
					log.Debug().Msg("AI chat: Discovery provider wired")
				}
			}
		}
	}

	// Wire unified resource provider for physical disks, Ceph, etc.
	if r.aiUnifiedAdapter != nil {
		service.SetUnifiedResourceProvider(r.aiUnifiedAdapter)
		log.Debug().Msg("AI chat: Unified resource provider wired")
	}

	log.Info().Msg("AI chat MCP tool providers wired")
}

// forecastStateProviderWrapper wraps monitor to implement forecast.StateProvider
type forecastStateProviderWrapper struct {
	monitor *monitoring.Monitor
}

func (w *forecastStateProviderWrapper) GetState() forecast.StateSnapshot {
	if w.monitor == nil {
		return forecast.StateSnapshot{}
	}

	state := w.monitor.GetState()
	result := forecast.StateSnapshot{
		VMs:        make([]forecast.VMInfo, 0, len(state.VMs)),
		Containers: make([]forecast.ContainerInfo, 0, len(state.Containers)),
		Nodes:      make([]forecast.NodeInfo, 0, len(state.Nodes)),
		Storage:    make([]forecast.StorageInfo, 0, len(state.Storage)),
	}

	for _, vm := range state.VMs {
		result.VMs = append(result.VMs, forecast.VMInfo{
			ID:   vm.ID,
			Name: vm.Name,
		})
	}

	for _, ct := range state.Containers {
		result.Containers = append(result.Containers, forecast.ContainerInfo{
			ID:   ct.ID,
			Name: ct.Name,
		})
	}

	for _, node := range state.Nodes {
		result.Nodes = append(result.Nodes, forecast.NodeInfo{
			ID:   node.ID,
			Name: node.Name,
		})
	}

	for _, storage := range state.Storage {
		result.Storage = append(result.Storage, forecast.StorageInfo{
			ID:   storage.ID,
			Name: storage.Name,
		})
	}

	return result
}

// incidentRecorderProviderWrapper adapts metrics.IncidentRecorder to tools.IncidentRecorderProvider.
type incidentRecorderProviderWrapper struct {
	recorder *metrics.IncidentRecorder
}

func (w *incidentRecorderProviderWrapper) GetWindowsForResource(resourceID string, limit int) []*tools.IncidentWindow {
	if w.recorder == nil {
		return nil
	}

	windows := w.recorder.GetWindowsForResource(resourceID, limit)
	if len(windows) == 0 {
		return nil
	}

	result := make([]*tools.IncidentWindow, 0, len(windows))
	for _, window := range windows {
		if window == nil {
			continue
		}
		result = append(result, convertIncidentWindow(window))
	}
	return result
}

func (w *incidentRecorderProviderWrapper) GetWindow(windowID string) *tools.IncidentWindow {
	if w.recorder == nil {
		return nil
	}
	window := w.recorder.GetWindow(windowID)
	if window == nil {
		return nil
	}
	return convertIncidentWindow(window)
}

func convertIncidentWindow(window *metrics.IncidentWindow) *tools.IncidentWindow {
	if window == nil {
		return nil
	}

	points := make([]tools.IncidentDataPoint, 0, len(window.DataPoints))
	for _, point := range window.DataPoints {
		points = append(points, tools.IncidentDataPoint{
			Timestamp: point.Timestamp,
			Metrics:   point.Metrics,
		})
	}

	var summary *tools.IncidentSummary
	if window.Summary != nil {
		summary = &tools.IncidentSummary{
			Duration:   window.Summary.Duration,
			DataPoints: window.Summary.DataPoints,
			Peaks:      window.Summary.Peaks,
			Lows:       window.Summary.Lows,
			Averages:   window.Summary.Averages,
			Changes:    window.Summary.Changes,
		}
	}

	return &tools.IncidentWindow{
		ID:           window.ID,
		ResourceID:   window.ResourceID,
		ResourceName: window.ResourceName,
		ResourceType: window.ResourceType,
		TriggerType:  window.TriggerType,
		TriggerID:    window.TriggerID,
		StartTime:    window.StartTime,
		EndTime:      window.EndTime,
		Status:       string(window.Status),
		DataPoints:   points,
		Summary:      summary,
	}
}

// eventCorrelatorProviderWrapper adapts proxmox.EventCorrelator to tools.EventCorrelatorProvider.
type eventCorrelatorProviderWrapper struct {
	correlator *proxmox.EventCorrelator
}

func (w *eventCorrelatorProviderWrapper) GetCorrelationsForResource(resourceID string, window time.Duration) []tools.EventCorrelation {
	if w.correlator == nil {
		return nil
	}

	correlations := w.correlator.GetCorrelationsForResource(resourceID)
	if len(correlations) == 0 {
		return nil
	}

	result := make([]tools.EventCorrelation, 0, len(correlations))
	for _, corr := range correlations {
		result = append(result, tools.EventCorrelation{
			EventType:    string(corr.Event.Type),
			Timestamp:    corr.Event.Timestamp,
			ResourceID:   corr.Event.ResourceID,
			ResourceName: corr.Event.ResourceName,
			Description:  corr.Explanation,
			Metadata: map[string]interface{}{
				"confidence": corr.Confidence,
				"anomalies":  len(corr.Anomalies),
				"event_id":   corr.Event.ID,
			},
		})
	}
	return result
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

// StartRelay starts the relay client if configured and licensed.
func (r *Router) StartRelay(ctx context.Context) {
	cfg, err := r.persistence.LoadRelayConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load relay config")
		return
	}
	if !cfg.Enabled {
		log.Debug().Msg("Relay not enabled, skipping")
		return
	}

	// Check license
	if r.licenseHandlers != nil {
		svc := r.licenseHandlers.Service(ctx)
		if svc != nil {
			if err := svc.RequireFeature(license.FeatureRelay); err != nil {
				log.Warn().Msg("Relay feature not licensed, skipping")
				return
			}
		}
	}

	localAddr := fmt.Sprintf("127.0.0.1:%d", r.config.FrontendPort)

	deps := relay.ClientDeps{
		LicenseTokenFunc: func() string {
			if r.licenseHandlers == nil {
				return ""
			}
			svc := r.licenseHandlers.Service(context.Background())
			if svc == nil {
				return ""
			}
			lic := svc.Current()
			if lic == nil {
				return ""
			}
			return lic.Raw
		},
		TokenValidator: func(token string) bool {
			config.Mu.Lock()
			_, ok := r.config.ValidateAPIToken(token)
			config.Mu.Unlock()
			return ok
		},
		LocalAddr:          localAddr,
		ServerVersion:      r.serverVersion,
		IdentityPubKey:     cfg.IdentityPublicKey,
		IdentityPrivateKey: cfg.IdentityPrivateKey,
	}

	relayCtx, relayCancel := context.WithCancel(ctx)
	client := relay.NewClient(*cfg, deps, log.Logger)

	r.relayMu.Lock()
	r.relayClient = client
	r.relayCancel = relayCancel
	r.relayMu.Unlock()

	go func() {
		if err := client.Run(relayCtx); err != nil && relayCtx.Err() == nil {
			log.Error().Err(err).Msg("Relay client stopped unexpectedly")
		}
	}()

	log.Info().Str("server_url", cfg.ServerURL).Msg("Relay client started")
}

// StopRelay stops the relay client.
func (r *Router) StopRelay() {
	r.relayMu.Lock()
	cancel := r.relayCancel
	client := r.relayClient
	r.relayClient = nil
	r.relayCancel = nil
	r.relayMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if client != nil {
		client.Close()
		log.Info().Msg("Relay client stopped")
	}
}

func (r *Router) handleGetRelayConfig(w http.ResponseWriter, req *http.Request) {
	cfg, err := r.persistence.LoadRelayConfig()
	if err != nil {
		http.Error(w, "failed to load relay config", http.StatusInternalServerError)
		return
	}

	// Omit the instance secret and private key from the response
	resp := struct {
		Enabled             bool   `json:"enabled"`
		ServerURL           string `json:"server_url"`
		IdentityPublicKey   string `json:"identity_public_key,omitempty"`
		IdentityFingerprint string `json:"identity_fingerprint,omitempty"`
	}{
		Enabled:             cfg.Enabled,
		ServerURL:           cfg.ServerURL,
		IdentityPublicKey:   cfg.IdentityPublicKey,
		IdentityFingerprint: cfg.IdentityFingerprint,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (r *Router) handleUpdateRelayConfig(w http.ResponseWriter, req *http.Request) {
	var update struct {
		Enabled        *bool   `json:"enabled"`
		ServerURL      *string `json:"server_url"`
		InstanceSecret *string `json:"instance_secret"`
	}
	if err := json.NewDecoder(req.Body).Decode(&update); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	prev, err := r.persistence.LoadRelayConfig()
	if err != nil {
		http.Error(w, "failed to load relay config", http.StatusInternalServerError)
		return
	}

	// Apply updates to a copy
	cfg := *prev
	if update.Enabled != nil {
		cfg.Enabled = *update.Enabled
	}
	if update.ServerURL != nil && *update.ServerURL != "" {
		cfg.ServerURL = *update.ServerURL
	}
	if update.InstanceSecret != nil {
		cfg.InstanceSecret = *update.InstanceSecret
	}

	// Generate identity keypair on first enable
	identityGenerated := false
	if cfg.Enabled && cfg.IdentityPrivateKey == "" {
		privKey, pubKey, fp, err := relay.GenerateIdentityKeyPair()
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate relay identity keypair")
			http.Error(w, "failed to generate identity keypair", http.StatusInternalServerError)
			return
		}
		cfg.IdentityPrivateKey = privKey
		cfg.IdentityPublicKey = pubKey
		cfg.IdentityFingerprint = fp
		identityGenerated = true
		log.Info().Str("fingerprint", fp).Msg("Generated relay instance identity keypair")
	}

	if err := r.persistence.SaveRelayConfig(cfg); err != nil {
		http.Error(w, "failed to save relay config", http.StatusInternalServerError)
		return
	}

	// Restart relay client if any connection-relevant field changed.
	// Also restart when identity keypair was just generated so the running
	// client picks up the new IdentityPubKey.
	configChanged := cfg.Enabled != prev.Enabled ||
		cfg.ServerURL != prev.ServerURL ||
		cfg.InstanceSecret != prev.InstanceSecret ||
		identityGenerated
	if configChanged {
		r.StopRelay()
		if cfg.Enabled {
			// Use Background context — the relay client must outlive this HTTP request.
			r.StartRelay(context.Background())
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (r *Router) handleGetRelayStatus(w http.ResponseWriter, req *http.Request) {
	r.relayMu.RLock()
	client := r.relayClient
	r.relayMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if client == nil {
		json.NewEncoder(w).Encode(relay.ClientStatus{})
		return
	}
	json.NewEncoder(w).Encode(client.Status())
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
	initialDelay := time.NewTimer(5 * time.Minute)
	defer initialDelay.Stop()

	select {
	case <-ctx.Done():
		return
	case <-initialDelay.C:
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
// WireAlertTriggeredAI connects the alert-triggered AI analyzer to the monitor's alert callback
// This should be called after StartPatrol() to ensure the analyzer is initialized
func (r *Router) WireAlertTriggeredAI() {
	// 1. Get the AI service (default tenant for now)
	if r.aiSettingsHandler == nil {
		log.Debug().Msg("AI settings handler not available for wiring")
		return
	}
	aiService := r.aiSettingsHandler.GetAIService(context.Background())
	if aiService == nil {
		log.Debug().Msg("AI service not available for wiring")
		return
	}

	// 2. Get the Patrol Service (The Watchdog)
	patrol := aiService.GetPatrolService()
	if patrol == nil {
		log.Debug().Msg("Patrol service not available for wiring")
		return
	}

	// 3. Get the Monitor (The Trigger)
	if r.monitor == nil {
		log.Debug().Msg("Monitor not available for AI alert callback")
		return
	}

	// 4. Connect Trigger -> Watchdog
	// When an alert fires, we immediately trigger the Patrol Agent to investigate
	r.monitor.SetAlertTriggeredAICallback(func(alert *alerts.Alert) {
		log.Info().Str("alert_id", alert.ID).Msg("Alert fired leading to Patrol Trigger")
		patrol.TriggerPatrolForAlert(alert)

		// We also trigger the specific analyzer if enabled, as it tracks specific stats
		if analyzer := r.GetAlertTriggeredAnalyzer(); analyzer != nil {
			analyzer.OnAlertFired(alert)
		}
	})

	log.Info().Msg("Alert-triggered AI Watchdog wired to monitor")
}

// Deprecated: deriveResourceTypeFromAlert uses heuristic string matching.
// Use alert.Metadata["resourceType"] as the canonical source instead.
// This function is retained for test backward compatibility only.
// See: Appendix C of alerts-unified-resource-hardening-plan-2026-02.md.
//
// deriveResourceTypeFromAlert derives the resource type from an alert.
func deriveResourceTypeFromAlert(alert *alerts.Alert) string {
	if alert == nil {
		return ""
	}

	// Try to derive from alert type
	alertType := strings.ToLower(alert.Type)
	switch {
	case strings.HasPrefix(alertType, "node") || strings.Contains(alert.ResourceID, "/node/"):
		return "node"
	case strings.Contains(alertType, "qemu") || strings.Contains(alert.ResourceID, "/qemu/"):
		return "vm"
	case strings.Contains(alertType, "lxc") || strings.Contains(alert.ResourceID, "/lxc/"):
		return "container"
	case strings.Contains(alertType, "docker"):
		return "docker"
	case strings.Contains(alertType, "storage"):
		return "storage"
	case strings.Contains(alertType, "pbs"):
		return "pbs"
	case strings.Contains(alertType, "kubernetes") || strings.Contains(alertType, "k8s"):
		return "kubernetes"
	default:
		// Try to infer from resource ID patterns
		if strings.Contains(alert.ResourceID, "/qemu/") {
			return "vm"
		}
		if strings.Contains(alert.ResourceID, "/lxc/") {
			return "container"
		}
		if strings.Contains(alert.ResourceID, "docker") {
			return "docker"
		}
		return "guest" // Default fallback
	}
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
		// Update DisableLegacyRouteRedirects so frontend sunset behavior applies immediately
		// BUT respect environment variable override if present
		if !r.config.EnvOverrides["PULSE_DISABLE_LEGACY_ROUTE_REDIRECTS"] {
			r.config.DisableLegacyRouteRedirects = systemSettings.DisableLegacyRouteRedirects
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
				"/api/public/signup",             // Hosted mode: public signup
				"/api/public/magic-link/request", // Hosted mode: request magic link
				"/api/public/magic-link/verify",  // Hosted mode: verify magic link
				"/api/webhooks/stripe",           // Hosted mode: Stripe webhook (signature verification is auth)
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
				"/auth/cloud-handoff",            // Cloud control plane handoff (token-authenticated)
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

			// Per-provider SSO OIDC routes are public (login initiation + callback)
			if strings.HasPrefix(normalizedPath, "/api/oidc/") {
				oidcParts := strings.Split(strings.TrimPrefix(normalizedPath, "/"), "/")
				if len(oidcParts) >= 4 && (oidcParts[3] == "login" || oidcParts[3] == "callback") {
					isPublic = true
				}
			}

			// Per-provider SSO SAML routes are public (login, ACS, metadata, SLO)
			if strings.HasPrefix(normalizedPath, "/api/saml/") {
				samlParts := strings.Split(strings.TrimPrefix(normalizedPath, "/"), "/")
				if len(samlParts) >= 4 {
					switch samlParts[3] {
					case "login", "acs", "metadata", "slo", "logout":
						isPublic = true
					}
				}
			}

			// Special case: setup-script should be public (uses setup codes for auth)
			if normalizedPath == "/api/setup-script" {
				// The script itself prompts for a setup code
				isPublic = true
			}

			// Allow temperature verification endpoint when a setup token is provided
			if normalizedPath == "/api/system/verify-temperature-ssh" && r.configHandlers != nil {
				if r.isValidSetupTokenForRequest(req) {
					isPublic = true
				}
			}

			// Allow SSH config endpoint when a setup token is provided
			if normalizedPath == "/api/system/ssh-config" && r.configHandlers != nil {
				if r.isValidSetupTokenForRequest(req) {
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
		// Check CSRF for state-changing requests.
		// CSRF is only needed when using session-based auth.
		skipCSRF := false
		// Quick setup can run before auth exists. Keep bootstrap/recovery flows usable
		// without a prior session+CSRF pair, but enforce CSRF once auth is configured.
		authConfigured := (r.config.AuthUser != "" && r.config.AuthPass != "") ||
			r.config.HasAPITokens() ||
			r.config.ProxyAuthSecret != "" ||
			(r.config.OIDC != nil && r.config.OIDC.Enabled)
		validRecoveryToken := false
		if recoveryToken := strings.TrimSpace(req.Header.Get("X-Recovery-Token")); recoveryToken != "" {
			validRecoveryToken = GetRecoveryTokenStore().IsRecoveryTokenValidConstantTime(recoveryToken)
		}
		if req.URL.Path == "/api/security/quick-setup" &&
			(!authConfigured || validRecoveryToken) {
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
		// Skip CSRF for SSO login/callback endpoints (OIDC and SAML)
		if strings.HasPrefix(req.URL.Path, "/api/oidc/") || strings.HasPrefix(req.URL.Path, "/api/saml/") {
			skipCSRF = true
		}
		// Skip CSRF for hosted public endpoints (may be called without a session or with a stale cookie).
		if req.URL.Path == "/api/public/signup" || req.URL.Path == "/api/public/magic-link/request" {
			skipCSRF = true
		}
		// Skip CSRF for cloud handoff (GET with token param, no prior session).
		if req.URL.Path == "/auth/cloud-handoff" {
			skipCSRF = true
		}
		if strings.HasPrefix(req.URL.Path, "/api/") && !skipCSRF && isValidProxyAuthRequest(r.config, req) && isCrossSiteBrowserRequest(req) {
			http.Error(w, "CSRF origin validation failed", http.StatusForbidden)
			LogAuditEventForTenant(GetOrgID(req.Context()), "csrf_failure", "", GetClientIP(req), req.URL.Path, false, "Cross-site browser mutation blocked for proxy auth")
			return
		}
		if strings.HasPrefix(req.URL.Path, "/api/") && !skipCSRF && !CheckCSRF(w, req) {
			http.Error(w, "CSRF token validation failed", http.StatusForbidden)
			LogAuditEventForTenant(GetOrgID(req.Context()), "csrf_failure", "", GetClientIP(req), req.URL.Path, false, "Invalid CSRF token")
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
			strings.HasPrefix(req.URL.Path, "/auth/") ||
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

	// Hosted mode must never derive a "public" URL from inbound requests.
	// It is too easy to abuse Host / forwarded headers and poison config.
	if r.hostedMode {
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
	if current != "" {
		// If explicitly configured, never overwrite from request
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

func isValidProxyAuthRequest(cfg *config.Config, req *http.Request) bool {
	if cfg == nil || req == nil || cfg.ProxyAuthSecret == "" {
		return false
	}
	if strings.TrimSpace(req.Header.Get("X-Proxy-Secret")) == "" {
		return false
	}
	valid, _, _ := CheckProxyAuth(cfg, req)
	return valid
}

func requestOrigin(req *http.Request) string {
	if req == nil {
		return ""
	}
	host := strings.TrimSpace(req.Host)
	if host == "" {
		return ""
	}

	scheme := "http"
	if isConnectionSecure(req) {
		scheme = "https"
	}
	return scheme + "://" + host
}

func canonicalOrigin(raw string) (scheme, host, port string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return "", "", "", false
	}

	scheme = strings.ToLower(strings.TrimSpace(u.Scheme))
	host = strings.ToLower(strings.TrimSpace(u.Hostname()))
	port = strings.TrimSpace(u.Port())
	if scheme == "" || host == "" {
		return "", "", "", false
	}
	if port == "" {
		switch scheme {
		case "https":
			port = "443"
		case "http":
			port = "80"
		}
	}
	return scheme, host, port, true
}

func sameOrigin(left, right string) bool {
	schemeL, hostL, portL, okL := canonicalOrigin(left)
	schemeR, hostR, portR, okR := canonicalOrigin(right)
	if !okL || !okR {
		return false
	}
	return schemeL == schemeR && hostL == hostR && portL == portR
}

// isCrossSiteBrowserRequest detects browser-originated cross-site requests.
// It is used as an additional safeguard for sessionless proxy-auth flows.
func isCrossSiteBrowserRequest(req *http.Request) bool {
	if req == nil {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(req.Header.Get("Sec-Fetch-Site"))) {
	case "cross-site":
		return true
	case "same-origin", "same-site", "none":
		return false
	}

	expected := requestOrigin(req)
	if expected == "" {
		return false
	}

	if origin := strings.TrimSpace(req.Header.Get("Origin")); origin != "" {
		if strings.EqualFold(origin, "null") {
			return true
		}
		return !sameOrigin(origin, expected)
	}

	if referer := strings.TrimSpace(req.Header.Get("Referer")); referer != "" {
		return !sameOrigin(referer, expected)
	}

	// Allow non-browser or legacy clients with neither Origin nor Referer.
	return false
}

func canCapturePublicURL(cfg *config.Config, req *http.Request) bool {
	if cfg == nil || req == nil {
		return false
	}

	// Proxy Auth: Require Admin
	if cfg.ProxyAuthSecret != "" {
		if valid, _, isAdmin := CheckProxyAuth(cfg, req); valid && isAdmin {
			return true
		}
	}

	// API Tokens: Require settings:write scope
	if cfg.HasAPITokens() {
		if token := strings.TrimSpace(req.Header.Get("X-API-Token")); token != "" {
			if record, ok := cfg.ValidateAPIToken(token); ok && record.HasScope(config.ScopeSettingsWrite) {
				return true
			}
		}
		if authHeader := strings.TrimSpace(req.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			if record, ok := cfg.ValidateAPIToken(strings.TrimSpace(authHeader[7:])); ok && record.HasScope(config.ScopeSettingsWrite) {
				return true
			}
		}
	}

	// Session (Browser): allow capture only for the configured local admin session.
	// This prevents low-privilege session users from poisoning public URL auto-detection.
	if cookie, err := req.Cookie("pulse_session"); err == nil && cookie.Value != "" {
		if ValidateSession(cookie.Value) {
			adminUser := strings.TrimSpace(cfg.AuthUser)
			if adminUser != "" {
				username := strings.TrimSpace(GetSessionUsername(cookie.Value))
				if strings.EqualFold(username, adminUser) {
					return true
				}
			}
		}
	}

	// Basic Auth: Trusted (Admin)
	if cfg.AuthUser != "" && cfg.AuthPass != "" {
		const prefix = "Basic "
		if authHeader := req.Header.Get("Authorization"); strings.HasPrefix(authHeader, prefix) {
			if decoded, err := base64.StdEncoding.DecodeString(authHeader[len(prefix):]); err == nil {
				if parts := strings.SplitN(string(decoded), ":", 2); len(parts) == 2 {
					if parts[0] == cfg.AuthUser && internalauth.CheckPasswordHash(parts[1], cfg.AuthPass) {
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

	monitorHealthy := r.monitor != nil
	schedulerHealthy := false
	if monitorHealthy {
		schedulerHealthy = r.monitor.SchedulerHealth().DeadLetter.Count == 0
	}

	statusCode := http.StatusOK
	status := "healthy"
	if !monitorHealthy || !schedulerHealthy {
		statusCode = http.StatusServiceUnavailable
		status = "unhealthy"
	}

	uptimeSeconds := 0.0
	if monitorHealthy {
		uptimeSeconds = time.Since(r.monitor.GetStartTime()).Seconds()
	}

	response := HealthResponse{
		Status:                      status,
		Timestamp:                   time.Now().Unix(),
		Uptime:                      uptimeSeconds,
		ProxyInstallScriptAvailable: true,
		DevModeSSH:                  os.Getenv("PULSE_DEV_ALLOW_CONTAINER_SSH") == "true",
		Dependencies: map[string]bool{
			"monitor":   monitorHealthy,
			"scheduler": schedulerHealthy,
			"websocket": r.wsHub != nil,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
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

	// SECURITY: Require authentication before allowing password change attempts
	// This prevents brute-force attacks on the current password
	if !CheckAuth(r.config, w, req) {
		log.Warn().
			Str("ip", req.RemoteAddr).
			Str("path", req.URL.Path).
			Msg("Unauthenticated password change attempt blocked")
		// CheckAuth already wrote the error response
		return
	}

	// Apply rate limiting to password change attempts to prevent brute-force
	clientIP := GetClientIP(req)
	if !authLimiter.Allow(clientIP) {
		log.Warn().
			Str("ip", clientIP).
			Msg("Rate limit exceeded for password change")
		writeErrorResponse(w, http.StatusTooManyRequests, "rate_limited",
			"Too many password change attempts. Please try again later.", nil)
		return
	}

	// Check lockout status for the client IP
	_, lockedUntil, isLocked := GetLockoutInfo(clientIP)
	if isLocked {
		remainingMinutes := int(time.Until(lockedUntil).Minutes())
		if remainingMinutes < 1 {
			remainingMinutes = 1
		}
		log.Warn().
			Str("ip", clientIP).
			Time("locked_until", lockedUntil).
			Msg("Password change blocked - IP locked out")
		writeErrorResponse(w, http.StatusForbidden, "locked_out",
			fmt.Sprintf("Too many failed attempts. Try again in %d minutes.", remainingMinutes), nil)
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
							RecordFailedLogin(clientIP)
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
			RecordFailedLogin(clientIP)
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
		LogAuditEventForTenant(GetOrgID(req.Context()), "password_change", r.config.AuthUser, GetClientIP(req), req.URL.Path, true, "Password changed (Docker)")

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
		LogAuditEventForTenant(GetOrgID(req.Context()), "password_change", r.config.AuthUser, GetClientIP(req), req.URL.Path, true, "Password changed")

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
	LogAuditEventForTenant(GetOrgID(req.Context()), "logout", "admin", GetClientIP(req), req.URL.Path, true, "User logged out")

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

		LogAuditEventForTenant(GetOrgID(req.Context()), "login", loginReq.Username, clientIP, req.URL.Path, false, "Account locked")

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
		LogAuditEventForTenant(GetOrgID(req.Context()), "login", loginReq.Username, clientIP, req.URL.Path, false, "Rate limited")
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
		LogAuditEventForTenant(GetOrgID(req.Context()), "login", loginReq.Username, clientIP, req.URL.Path, true, "Successful login")

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
		LogAuditEventForTenant(GetOrgID(req.Context()), "login", loginReq.Username, clientIP, req.URL.Path, false, "Invalid credentials")

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

	// Use RequireAdmin to ensure proper admin checks (including proxy auth) for session users
	RequireAdmin(r.config, func(w http.ResponseWriter, req *http.Request) {
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
		LogAuditEventForTenant(GetOrgID(req.Context()), "lockout_reset", "admin", GetClientIP(req), req.URL.Path, true,
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
	})(w, req)
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

	// Use tenant-aware monitor to get state for the current organization
	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "no_monitor",
			"Monitor not available", nil)
		return
	}
	state := monitor.GetState()

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
			response.ContainerID = hostname
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

	// Get tenant-specific monitor and current state
	monitor := r.getTenantMonitor(req.Context())
	state := monitor.GetState()

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
	const inMemoryChartThreshold = 2 * time.Hour

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

	// Convert time range to duration.
	duration := parseChartsRangeDuration(timeRange)

	// Get tenant-specific monitor and current state
	monitor := r.getTenantMonitor(req.Context())
	state := monitor.GetState()
	metricsStoreEnabled := monitor.GetMetricsStore() != nil
	primarySourceHint := "memory"
	if metricsStoreEnabled && duration > inMemoryChartThreshold {
		primarySourceHint = "store_or_memory_fallback"
	}

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

		// Get historical metrics (falls back to SQLite + LTTB for long ranges)
		metrics := monitor.GetGuestMetricsForChart(vm.ID, "vm", vm.ID, duration)

		// Convert metric points to API format (sparkline metrics only)
		for metricType, points := range metrics {
			if !sparklineMetrics[metricType] {
				continue
			}
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

		// Get historical metrics (falls back to SQLite + LTTB for long ranges)
		metrics := monitor.GetGuestMetricsForChart(ct.ID, "container", ct.ID, duration)

		// Convert metric points to API format (sparkline metrics only)
		for metricType, points := range metrics {
			if !sparklineMetrics[metricType] {
				continue
			}
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

		// Get historical metrics (falls back to SQLite + LTTB for long ranges)
		metrics := monitor.GetStorageMetricsForChart(storage.ID, duration)

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

		// Get historical metrics for each type (falls back to SQLite + LTTB for long ranges)
		for _, metricType := range []string{"cpu", "memory", "disk", "netin", "netout"} {
			points := monitor.GetNodeMetricsForChart(node.ID, metricType, duration)
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
				hasFallbackValue := true
				switch metricType {
				case "cpu":
					value = node.CPU * 100
				case "memory":
					value = node.Memory.Usage
				case "disk":
					value = node.Disk.Usage
				default:
					// No synthetic fallback for node netin/netout.
					// We only emit these when actual history exists.
					hasFallbackValue = false
				}
				if hasFallbackValue {
					nodeData[node.ID][metricType] = []MetricPoint{
						{Timestamp: currentTime, Value: value},
					}
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

			// Get historical metrics using the docker: prefix key (falls back to SQLite + LTTB for long ranges)
			metricKey := fmt.Sprintf("docker:%s", container.ID)
			metrics := monitor.GetGuestMetricsForChart(metricKey, "dockerContainer", container.ID, duration)

			// Convert metric points to API format (sparkline metrics only)
			for metricType, points := range metrics {
				if !sparklineMetrics[metricType] {
					continue
				}
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

		// Get historical metrics using the dockerHost: prefix key (falls back to SQLite + LTTB for long ranges)
		metricKey := fmt.Sprintf("dockerHost:%s", host.ID)
		metrics := monitor.GetGuestMetricsForChart(metricKey, "dockerHost", host.ID, duration)

		// Convert metric points to API format (sparkline metrics only)
		for metricType, points := range metrics {
			if !sparklineMetrics[metricType] {
				continue
			}
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
			dockerHostData[host.ID]["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: host.CPUUsage}}
			dockerHostData[host.ID]["memory"] = []MetricPoint{{Timestamp: currentTime, Value: host.Memory.Usage}}
			var diskPercent float64
			if len(host.Disks) > 0 {
				diskPercent = host.Disks[0].Usage
			}
			dockerHostData[host.ID]["disk"] = []MetricPoint{{Timestamp: currentTime, Value: diskPercent}}
		}
	}

	// Process unified host agents - get historical data
	hostData := make(map[string]VMChartData)
	for _, host := range state.Hosts {
		if host.ID == "" {
			continue
		}

		if hostData[host.ID] == nil {
			hostData[host.ID] = make(VMChartData)
		}

		// Get historical metrics using the host: prefix key (falls back to SQLite + LTTB for long ranges)
		metricKey := fmt.Sprintf("host:%s", host.ID)
		metrics := monitor.GetGuestMetricsForChart(metricKey, "host", host.ID, duration)

		// Convert metric points to API format (sparkline metrics only)
		for metricType, points := range metrics {
			if !sparklineMetrics[metricType] {
				continue
			}
			hostData[host.ID][metricType] = make([]MetricPoint, len(points))
			for i, point := range points {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				hostData[host.ID][metricType][i] = MetricPoint{
					Timestamp: ts,
					Value:     point.Value,
				}
			}
		}

		// If no historical data, add current value
		if len(hostData[host.ID]["cpu"]) == 0 {
			hostData[host.ID]["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: host.CPUUsage}}
			hostData[host.ID]["memory"] = []MetricPoint{{Timestamp: currentTime, Value: host.Memory.Usage}}
			var diskPercent float64
			if len(host.Disks) > 0 {
				diskPercent = host.Disks[0].Usage
			}
			hostData[host.ID]["disk"] = []MetricPoint{{Timestamp: currentTime, Value: diskPercent}}
		}
	}

	countChartPoints := func(metricsMap map[string]VMChartData) int {
		total := 0
		for _, metricSeries := range metricsMap {
			for _, points := range metricSeries {
				total += len(points)
			}
		}
		return total
	}

	countNodePoints := func(metricsMap map[string]NodeChartData) int {
		total := 0
		for _, metricSeries := range metricsMap {
			for _, points := range metricSeries {
				total += len(points)
			}
		}
		return total
	}

	countStoragePoints := func(metricsMap map[string]StorageChartData) int {
		total := 0
		for _, metricSeries := range metricsMap {
			for _, points := range metricSeries {
				total += len(points)
			}
		}
		return total
	}

	guestPoints := countChartPoints(chartData)
	nodePoints := countNodePoints(nodeData)
	storagePoints := countStoragePoints(storageData)
	dockerContainerPoints := countChartPoints(dockerData)
	dockerHostPoints := countChartPoints(dockerHostData)
	hostPoints := countChartPoints(hostData)

	response := ChartResponse{
		ChartData:      chartData,
		NodeData:       nodeData,
		StorageData:    storageData,
		DockerData:     dockerData,
		DockerHostData: dockerHostData,
		HostData:       hostData,
		GuestTypes:     guestTypes,
		Timestamp:      currentTime,
		Stats: ChartStats{
			OldestDataTimestamp:   oldestTimestamp,
			Range:                 timeRange,
			RangeSeconds:          int64(duration / time.Second),
			MetricsStoreEnabled:   metricsStoreEnabled,
			PrimarySourceHint:     primarySourceHint,
			InMemoryThresholdSecs: int64(inMemoryChartThreshold / time.Second),
			PointCounts: ChartPointCounts{
				Total:            guestPoints + nodePoints + storagePoints + dockerContainerPoints + dockerHostPoints + hostPoints,
				Guests:           guestPoints,
				Nodes:            nodePoints,
				Storage:          storagePoints,
				DockerContainers: dockerContainerPoints,
				DockerHosts:      dockerHostPoints,
				Hosts:            hostPoints,
			},
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
		Int("hosts", len(hostData)).
		Str("range", timeRange).
		Msg("Chart data response sent")
}

func parseWorkloadMaxPoints(raw string) int {
	const (
		defaultMaxPoints = 180
		minMaxPoints     = 30
		maxMaxPoints     = 500
	)

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultMaxPoints
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return defaultMaxPoints
	}
	if value < minMaxPoints {
		return minMaxPoints
	}
	if value > maxMaxPoints {
		return maxMaxPoints
	}
	return value
}

func capMetricPointSeries(points []MetricPoint, maxPoints int) []MetricPoint {
	if len(points) <= maxPoints || maxPoints <= 0 {
		return points
	}
	if maxPoints == 1 {
		return []MetricPoint{points[len(points)-1]}
	}

	result := make([]MetricPoint, 0, maxPoints)
	step := float64(len(points)-1) / float64(maxPoints-1)
	prevIndex := -1

	for i := 0; i < maxPoints; i++ {
		index := int(float64(i)*step + 0.5)
		if index <= prevIndex {
			index = prevIndex + 1
		}
		if index >= len(points) {
			index = len(points) - 1
		}
		result = append(result, points[index])
		prevIndex = index
	}

	if result[len(result)-1].Timestamp != points[len(points)-1].Timestamp {
		result[len(result)-1] = points[len(points)-1]
	}
	return result
}

// sparklineMetrics lists the metric types consumed by summary sparklines.
// Metrics not in this set (e.g. diskread, diskwrite) are omitted to keep
// payloads small.
var sparklineMetrics = map[string]bool{
	"cpu":    true,
	"memory": true,
	"disk":   true,
	"netin":  true,
	"netout": true,
}

func convertMetricsForChart(
	metrics map[string][]monitoring.MetricPoint,
	oldestTimestamp *int64,
	maxPoints int,
) VMChartData {
	converted := make(VMChartData, len(metrics))
	for metricType, metricPoints := range metrics {
		if !sparklineMetrics[metricType] {
			continue
		}
		points := make([]MetricPoint, len(metricPoints))
		for i, point := range metricPoints {
			ts := point.Timestamp.Unix() * 1000
			if ts < *oldestTimestamp {
				*oldestTimestamp = ts
			}
			points[i] = MetricPoint{
				Timestamp: ts,
				Value:     point.Value,
			}
		}
		converted[metricType] = capMetricPointSeries(points, maxPoints)
	}
	return converted
}

const (
	mockWorkloadMinSeriesPoints = 24
	mockWorkloadMaxSeriesPoints = 180
)

func clampChartValue(value, min, max float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func hashChartSeed(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

func targetMockSeriesPoints(duration time.Duration, maxPoints int) int {
	target := int(duration / (2 * time.Minute))
	if target < mockWorkloadMinSeriesPoints {
		target = mockWorkloadMinSeriesPoints
	}
	if maxPoints > 0 && target > maxPoints {
		target = maxPoints
	}
	if target > mockWorkloadMaxSeriesPoints {
		target = mockWorkloadMaxSeriesPoints
	}
	if target < 2 {
		target = 2
	}
	return target
}

// mockMetricStyle returns the series style for a given metric type.
func mockMetricStyle(metricType string) monitoring.SeriesStyle {
	switch metricType {
	case "cpu", "diskread", "diskwrite", "netin", "netout":
		return monitoring.StyleSpiky
	case "memory":
		return monitoring.StylePlateau
	default:
		return monitoring.StyleFlat
	}
}

// generateStyledMockSeries produces a MetricPoint slice using the style-based
// generator from the monitoring package.
func generateStyledMockSeries(
	nowMillis int64,
	duration time.Duration,
	numPoints int,
	current float64,
	min float64,
	max float64,
	seedPrefix string,
	resourceID string,
	metricType string,
) []MetricPoint {
	seed := monitoring.HashSeed(seedPrefix, resourceID, metricType)
	style := mockMetricStyle(metricType)
	values := monitoring.GenerateSeededSeries(current, numPoints, seed, min, max, style)

	durationMillis := int64(duration / time.Millisecond)
	if durationMillis <= 0 {
		durationMillis = int64(time.Minute / time.Millisecond)
	}
	step := durationMillis / int64(numPoints-1)
	if step <= 0 {
		step = 1
	}
	startMillis := nowMillis - durationMillis
	points := make([]MetricPoint, numPoints)
	for i := 0; i < numPoints; i++ {
		points[i] = MetricPoint{
			Timestamp: startMillis + int64(i)*step,
			Value:     values[i],
		}
	}
	return points
}

func updateOldestTimestampFromSeries(metrics VMChartData, oldestTimestamp *int64) {
	if oldestTimestamp == nil {
		return
	}
	for _, points := range metrics {
		for _, point := range points {
			if point.Timestamp < *oldestTimestamp {
				*oldestTimestamp = point.Timestamp
			}
		}
	}
}

func buildSyntheticMetricHistorySeries(
	now time.Time,
	duration time.Duration,
	maxPoints int,
	resourceID string,
	metricType string,
	current float64,
) []monitoring.MetricPoint {
	var min float64
	var max float64

	switch metricType {
	case "smart_temp":
		if current <= 0 {
			return nil
		}
		min = 25
		max = 95
	default:
		return nil
	}

	numPoints := targetMockSeriesPoints(duration, maxPoints)
	series := generateStyledMockSeries(
		now.UnixMilli(), duration, numPoints,
		current, min, max,
		"history-mock", resourceID, metricType,
	)

	converted := make([]monitoring.MetricPoint, len(series))
	for i, point := range series {
		converted[i] = monitoring.MetricPoint{
			Timestamp: time.UnixMilli(point.Timestamp),
			Value:     point.Value,
		}
	}

	return converted
}

func buildMockWorkloadMetricHistorySeries(
	now time.Time,
	duration time.Duration,
	maxPoints int,
	resourceID string,
	metricType string,
	current float64,
) []monitoring.MetricPoint {
	var min float64
	var max float64

	switch metricType {
	case "cpu", "memory", "disk":
		min = 0
		max = 100
	case "diskread", "diskwrite", "netin", "netout":
		min = 0
		max = math.Max(current*1.8, 1)
	default:
		return nil
	}

	numPoints := targetMockSeriesPoints(duration, maxPoints)
	series := generateStyledMockSeries(
		now.UnixMilli(), duration, numPoints,
		current, min, max,
		"history-mock", resourceID, metricType,
	)

	converted := make([]monitoring.MetricPoint, len(series))
	for i, point := range series {
		converted[i] = monitoring.MetricPoint{
			Timestamp: time.UnixMilli(point.Timestamp),
			Value:     point.Value,
		}
	}

	return converted
}

// handleWorkloadCharts serves workload-only chart data used by workloads
// sparklines. It intentionally excludes infrastructure/storage chart payloads
// to keep requests small and stable for large fleets.
func (r *Router) handleWorkloadCharts(w http.ResponseWriter, req *http.Request) {
	log.Debug().Str("method", req.Method).Str("url", req.URL.String()).Msg("Workload charts endpoint hit")
	const inMemoryChartThreshold = 2 * time.Hour

	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := req.URL.Query()
	timeRange := query.Get("range")
	if timeRange == "" {
		timeRange = "1h"
	}
	selectedNodeID := strings.TrimSpace(query.Get("node"))
	maxPoints := parseWorkloadMaxPoints(query.Get("maxPoints"))
	duration := parseChartsRangeDuration(timeRange)

	monitor := r.getTenantMonitor(req.Context())
	state := monitor.GetState()
	metricsStoreEnabled := monitor.GetMetricsStore() != nil
	primarySourceHint := "memory"
	if metricsStoreEnabled && duration > inMemoryChartThreshold {
		primarySourceHint = "store_or_memory_fallback"
	}

	currentTime := time.Now().Unix() * 1000
	oldestTimestamp := currentTime

	var selectedNode *models.Node
	if selectedNodeID != "" {
		for idx := range state.Nodes {
			if state.Nodes[idx].ID == selectedNodeID {
				selectedNode = &state.Nodes[idx]
				break
			}
		}
		if selectedNode == nil {
			log.Debug().
				Str("selectedNodeID", selectedNodeID).
				Msg("Workload charts node filter not found in current state; falling back to global scope")
		}
	}

	matchesSelectedNode := func(instance, nodeName string) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		return strings.EqualFold(strings.TrimSpace(instance), strings.TrimSpace(selectedNode.Instance)) &&
			strings.EqualFold(strings.TrimSpace(nodeName), strings.TrimSpace(selectedNode.Name))
	}

	matchesSelectedDockerHost := func(host models.DockerHost) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		nodeName := strings.TrimSpace(selectedNode.Name)
		if nodeName == "" {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(host.Hostname), nodeName) ||
			strings.EqualFold(strings.TrimSpace(host.DisplayName), nodeName)
	}

	matchesSelectedKubernetesPod := func(pod models.KubernetesPod) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		nodeName := strings.TrimSpace(selectedNode.Name)
		if nodeName == "" {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(pod.NodeName), nodeName)
	}

	chartData := make(map[string]VMChartData)
	dockerData := make(map[string]VMChartData)

	guestTypes := make(map[string]string)

	for _, vm := range state.VMs {
		if !matchesSelectedNode(vm.Instance, vm.Node) {
			continue
		}

		metrics := monitor.GetGuestMetricsForChart(vm.ID, "vm", vm.ID, duration)
		series := convertMetricsForChart(metrics, &oldestTimestamp, maxPoints)
		guestTypes[vm.ID] = "vm"

		if len(series["cpu"]) == 0 {
			series["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: vm.CPU * 100}}
			series["memory"] = []MetricPoint{{Timestamp: currentTime, Value: vm.Memory.Usage}}
			series["disk"] = []MetricPoint{{Timestamp: currentTime, Value: vm.Disk.Usage}}
			series["netin"] = []MetricPoint{{Timestamp: currentTime, Value: float64(vm.NetworkIn)}}
			series["netout"] = []MetricPoint{{Timestamp: currentTime, Value: float64(vm.NetworkOut)}}
		}
		updateOldestTimestampFromSeries(series, &oldestTimestamp)
		chartData[vm.ID] = series
	}

	for _, ct := range state.Containers {
		if !matchesSelectedNode(ct.Instance, ct.Node) {
			continue
		}

		metrics := monitor.GetGuestMetricsForChart(ct.ID, "container", ct.ID, duration)
		series := convertMetricsForChart(metrics, &oldestTimestamp, maxPoints)
		guestTypes[ct.ID] = "container"

		if len(series["cpu"]) == 0 {
			series["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: ct.CPU * 100}}
			series["memory"] = []MetricPoint{{Timestamp: currentTime, Value: ct.Memory.Usage}}
			series["disk"] = []MetricPoint{{Timestamp: currentTime, Value: ct.Disk.Usage}}
			series["netin"] = []MetricPoint{{Timestamp: currentTime, Value: float64(ct.NetworkIn)}}
			series["netout"] = []MetricPoint{{Timestamp: currentTime, Value: float64(ct.NetworkOut)}}
		}
		updateOldestTimestampFromSeries(series, &oldestTimestamp)
		chartData[ct.ID] = series
	}

	for _, cluster := range state.KubernetesClusters {
		if cluster.Hidden {
			continue
		}
		for _, pod := range cluster.Pods {
			if !matchesSelectedKubernetesPod(pod) {
				continue
			}

			metricKey := kubernetesPodMetricID(cluster, pod)
			if metricKey == "" {
				continue
			}
			currentMetrics := kubernetesPodCurrentMetrics(cluster, pod)

			metrics := monitor.GetGuestMetricsForChart(metricKey, "k8s", metricKey, duration)
			series := convertMetricsForChart(metrics, &oldestTimestamp, maxPoints)
			guestTypes[metricKey] = "k8s"

			if len(series["cpu"]) == 0 {
				series["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: currentMetrics["cpu"]}}
				series["memory"] = []MetricPoint{{Timestamp: currentTime, Value: currentMetrics["memory"]}}
				series["disk"] = []MetricPoint{{Timestamp: currentTime, Value: currentMetrics["disk"]}}
				series["netin"] = []MetricPoint{{Timestamp: currentTime, Value: currentMetrics["netin"]}}
				series["netout"] = []MetricPoint{{Timestamp: currentTime, Value: currentMetrics["netout"]}}
			}
			updateOldestTimestampFromSeries(series, &oldestTimestamp)
			chartData[metricKey] = series
		}
	}

	for _, host := range state.DockerHosts {
		if !matchesSelectedDockerHost(host) {
			continue
		}

		for _, container := range host.Containers {
			if container.ID == "" {
				continue
			}

			metricKey := fmt.Sprintf("docker:%s", container.ID)
			metrics := monitor.GetGuestMetricsForChart(metricKey, "dockerContainer", container.ID, duration)
			series := convertMetricsForChart(metrics, &oldestTimestamp, maxPoints)

			if len(series["cpu"]) == 0 {
				series["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: container.CPUPercent}}
				series["memory"] = []MetricPoint{{Timestamp: currentTime, Value: container.MemoryPercent}}

				var diskPercent float64
				if container.RootFilesystemBytes > 0 && container.WritableLayerBytes > 0 {
					diskPercent = float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
					if diskPercent > 100 {
						diskPercent = 100
					}
				}
				series["disk"] = []MetricPoint{{Timestamp: currentTime, Value: diskPercent}}
				series["netin"] = []MetricPoint{{Timestamp: currentTime, Value: container.NetInRate}}
				series["netout"] = []MetricPoint{{Timestamp: currentTime, Value: container.NetOutRate}}
			}
			updateOldestTimestampFromSeries(series, &oldestTimestamp)
			dockerData[container.ID] = series
		}
	}

	countChartPoints := func(metricsMap map[string]VMChartData) int {
		total := 0
		for _, metricSeries := range metricsMap {
			for _, points := range metricSeries {
				total += len(points)
			}
		}
		return total
	}

	guestPoints := countChartPoints(chartData)
	dockerContainerPoints := countChartPoints(dockerData)

	response := WorkloadChartsResponse{
		ChartData:  chartData,
		DockerData: dockerData,
		GuestTypes: guestTypes,
		Timestamp:  currentTime,
		Stats: ChartStats{
			OldestDataTimestamp:   oldestTimestamp,
			Range:                 timeRange,
			RangeSeconds:          int64(duration / time.Second),
			MetricsStoreEnabled:   metricsStoreEnabled,
			PrimarySourceHint:     primarySourceHint,
			InMemoryThresholdSecs: int64(inMemoryChartThreshold / time.Second),
			PointCounts: ChartPointCounts{
				Total:            guestPoints + dockerContainerPoints,
				Guests:           guestPoints,
				DockerContainers: dockerContainerPoints,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode workload chart data response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// parseChartsRangeDuration converts the UI chart range query (e.g. "5m", "1h")
// into a duration. This is shared by /api/charts and /api/charts/infrastructure
// to prevent drift.
func parseChartsRangeDuration(rangeStr string) time.Duration {
	switch rangeStr {
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "8h":
		return 8 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return time.Hour
	}
}

// handleInfrastructureCharts serves infrastructure-only chart data.
// This is intentionally narrower than /api/charts to reduce payload size and server-side compute
// for the Infrastructure page summary cards.
func (r *Router) handleInfrastructureCharts(w http.ResponseWriter, req *http.Request) {
	log.Debug().Str("method", req.Method).Str("url", req.URL.String()).Msg("Infrastructure charts endpoint hit")
	const inMemoryChartThreshold = 2 * time.Hour

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
	// Convert time range to duration.
	duration := parseChartsRangeDuration(timeRange)

	monitor := r.getTenantMonitor(req.Context())
	state := monitor.GetState()
	metricsStoreEnabled := monitor.GetMetricsStore() != nil
	primarySourceHint := "memory"
	if metricsStoreEnabled && duration > inMemoryChartThreshold {
		primarySourceHint = "store_or_memory_fallback"
	}

	currentTime := time.Now().Unix() * 1000
	oldestTimestamp := currentTime

	// Nodes - cpu/memory/disk/netin/netout
	nodeData := make(map[string]NodeChartData)
	for _, node := range state.Nodes {
		if nodeData[node.ID] == nil {
			nodeData[node.ID] = make(NodeChartData)
		}
		for _, metricType := range []string{"cpu", "memory", "disk", "netin", "netout"} {
			points := monitor.GetNodeMetricsForChart(node.ID, metricType, duration)
			nodeData[node.ID][metricType] = make([]MetricPoint, len(points))
			for i, point := range points {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				nodeData[node.ID][metricType][i] = MetricPoint{Timestamp: ts, Value: point.Value}
			}
			if len(nodeData[node.ID][metricType]) == 0 {
				var value float64
				hasFallbackValue := true
				switch metricType {
				case "cpu":
					value = node.CPU * 100
				case "memory":
					value = node.Memory.Usage
				case "disk":
					value = node.Disk.Usage
				default:
					hasFallbackValue = false
				}
				if hasFallbackValue {
					nodeData[node.ID][metricType] = []MetricPoint{{Timestamp: currentTime, Value: value}}
				}
			}
		}
	}

	// Docker hosts - cpu/memory/disk
	dockerHostData := make(map[string]VMChartData)
	for _, host := range state.DockerHosts {
		if host.ID == "" {
			continue
		}
		if dockerHostData[host.ID] == nil {
			dockerHostData[host.ID] = make(VMChartData)
		}
		metricKey := fmt.Sprintf("dockerHost:%s", host.ID)
		metrics := monitor.GetGuestMetricsForChart(metricKey, "dockerHost", host.ID, duration)
		for metricType, points := range metrics {
			if !sparklineMetrics[metricType] {
				continue
			}
			dockerHostData[host.ID][metricType] = make([]MetricPoint, len(points))
			for i, point := range points {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				dockerHostData[host.ID][metricType][i] = MetricPoint{Timestamp: ts, Value: point.Value}
			}
		}
		if len(dockerHostData[host.ID]["cpu"]) == 0 {
			dockerHostData[host.ID]["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: host.CPUUsage}}
			dockerHostData[host.ID]["memory"] = []MetricPoint{{Timestamp: currentTime, Value: host.Memory.Usage}}
			var diskPercent float64
			if len(host.Disks) > 0 {
				diskPercent = host.Disks[0].Usage
			}
			dockerHostData[host.ID]["disk"] = []MetricPoint{{Timestamp: currentTime, Value: diskPercent}}
		}
	}

	// Unified host agents - cpu/memory/disk
	hostData := make(map[string]VMChartData)
	for _, host := range state.Hosts {
		if host.ID == "" {
			continue
		}
		if hostData[host.ID] == nil {
			hostData[host.ID] = make(VMChartData)
		}
		metricKey := fmt.Sprintf("host:%s", host.ID)
		metrics := monitor.GetGuestMetricsForChart(metricKey, "host", host.ID, duration)
		for metricType, points := range metrics {
			if !sparklineMetrics[metricType] {
				continue
			}
			hostData[host.ID][metricType] = make([]MetricPoint, len(points))
			for i, point := range points {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				hostData[host.ID][metricType][i] = MetricPoint{Timestamp: ts, Value: point.Value}
			}
		}
		if len(hostData[host.ID]["cpu"]) == 0 {
			hostData[host.ID]["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: host.CPUUsage}}
			hostData[host.ID]["memory"] = []MetricPoint{{Timestamp: currentTime, Value: host.Memory.Usage}}
			var diskPercent float64
			if len(host.Disks) > 0 {
				diskPercent = host.Disks[0].Usage
			}
			hostData[host.ID]["disk"] = []MetricPoint{{Timestamp: currentTime, Value: diskPercent}}
		}
	}

	countNodePoints := func(metricsMap map[string]NodeChartData) int {
		total := 0
		for _, metricSeries := range metricsMap {
			for _, points := range metricSeries {
				total += len(points)
			}
		}
		return total
	}
	countChartPoints := func(metricsMap map[string]VMChartData) int {
		total := 0
		for _, metricSeries := range metricsMap {
			for _, points := range metricSeries {
				total += len(points)
			}
		}
		return total
	}

	nodePoints := countNodePoints(nodeData)
	dockerHostPoints := countChartPoints(dockerHostData)
	hostPoints := countChartPoints(hostData)

	response := InfrastructureChartsResponse{
		NodeData:       nodeData,
		DockerHostData: dockerHostData,
		HostData:       hostData,
		Timestamp:      currentTime,
		Stats: ChartStats{
			OldestDataTimestamp:   oldestTimestamp,
			Range:                 timeRange,
			RangeSeconds:          int64(duration / time.Second),
			MetricsStoreEnabled:   metricsStoreEnabled,
			PrimarySourceHint:     primarySourceHint,
			InMemoryThresholdSecs: int64(inMemoryChartThreshold / time.Second),
			PointCounts: ChartPointCounts{
				Total:       nodePoints + dockerHostPoints + hostPoints,
				Nodes:       nodePoints,
				DockerHosts: dockerHostPoints,
				Hosts:       hostPoints,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode infrastructure chart data response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleInfrastructureSummaryCharts is a compatibility endpoint for older UIs.
// It currently serves the same payload as handleInfrastructureCharts.
func (r *Router) handleInfrastructureSummaryCharts(w http.ResponseWriter, req *http.Request) {
	r.handleInfrastructureCharts(w, req)
}

type workloadSummaryBuckets struct {
	cpu     []float64
	memory  []float64
	disk    []float64
	network []float64
}

type workloadsSummarySnapshot struct {
	id      string
	name    string
	cpu     float64
	memory  float64
	disk    float64
	network float64
}

func workloadSummaryBucketTimestamp(timestampMs int64) int64 {
	const bucketSizeMs = int64(30_000)
	return (timestampMs / bucketSizeMs) * bucketSizeMs
}

func clampWorkloadPercent(value float64) float64 {
	if value != value {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func clampNonNegativeWorkloadValue(value float64) float64 {
	if value != value {
		return 0
	}
	if value < 0 {
		return 0
	}
	return value
}

func kubernetesClusterKey(cluster models.KubernetesCluster) string {
	if value := strings.TrimSpace(cluster.ID); value != "" {
		return value
	}
	if value := strings.TrimSpace(cluster.Name); value != "" {
		return value
	}
	if value := strings.TrimSpace(cluster.DisplayName); value != "" {
		return value
	}
	return "k8s-cluster"
}

func kubernetesPodIdentifier(pod models.KubernetesPod) string {
	if value := strings.TrimSpace(pod.UID); value != "" {
		return value
	}
	namespace := strings.TrimSpace(pod.Namespace)
	name := strings.TrimSpace(pod.Name)
	if namespace != "" || name != "" {
		return fmt.Sprintf("%s/%s", namespace, name)
	}
	return "pod"
}

func kubernetesPodMetricID(cluster models.KubernetesCluster, pod models.KubernetesPod) string {
	clusterKey := kubernetesClusterKey(cluster)
	podKey := kubernetesPodIdentifier(pod)
	if clusterKey == "" || podKey == "" {
		return ""
	}
	return fmt.Sprintf("k8s:%s:pod:%s", clusterKey, podKey)
}

func kubernetesPodDisplayName(pod models.KubernetesPod) string {
	name := strings.TrimSpace(pod.Name)
	namespace := strings.TrimSpace(pod.Namespace)
	if namespace == "" {
		if name == "" {
			return kubernetesPodIdentifier(pod)
		}
		return name
	}
	if name == "" {
		return namespace
	}
	return fmt.Sprintf("%s/%s", namespace, name)
}

func kubernetesPodIsRunning(pod models.KubernetesPod) bool {
	return strings.EqualFold(strings.TrimSpace(pod.Phase), "running")
}

func kubernetesPodCurrentMetrics(cluster models.KubernetesCluster, pod models.KubernetesPod) map[string]float64 {
	cpuPercent := clampWorkloadPercent(pod.UsageCPUPercent)
	memoryPercent := clampWorkloadPercent(pod.UsageMemoryPercent)

	if memoryPercent <= 0 && pod.UsageMemoryBytes > 0 {
		totalBytes := kubernetesPodMemoryTotalBytes(cluster, pod)
		if totalBytes > 0 {
			memoryPercent = clampWorkloadPercent((float64(pod.UsageMemoryBytes) / float64(totalBytes)) * 100)
		}
	}

	diskPercent := clampWorkloadPercent(pod.DiskUsagePercent)
	netIn := clampNonNegativeWorkloadValue(pod.NetInRate)
	netOut := clampNonNegativeWorkloadValue(pod.NetOutRate)

	return map[string]float64{
		"cpu":       cpuPercent,
		"memory":    memoryPercent,
		"disk":      diskPercent,
		"diskread":  0,
		"diskwrite": 0,
		"netin":     netIn,
		"netout":    netOut,
	}
}

func kubernetesPodMemoryTotalBytes(cluster models.KubernetesCluster, pod models.KubernetesPod) int64 {
	nodeName := strings.TrimSpace(pod.NodeName)
	if nodeName == "" {
		return 0
	}
	for _, node := range cluster.Nodes {
		if !strings.EqualFold(strings.TrimSpace(node.Name), nodeName) {
			continue
		}
		if node.AllocMemoryBytes > 0 {
			return node.AllocMemoryBytes
		}
		if node.CapacityMemoryBytes > 0 {
			return node.CapacityMemoryBytes
		}
		return 0
	}
	return 0
}

func getOrCreateWorkloadBucket(buckets map[int64]*workloadSummaryBuckets, bucketTs int64) *workloadSummaryBuckets {
	if bucket, ok := buckets[bucketTs]; ok {
		return bucket
	}
	bucket := &workloadSummaryBuckets{}
	buckets[bucketTs] = bucket
	return bucket
}

func appendWorkloadMetricPoints(
	buckets map[int64]*workloadSummaryBuckets,
	points []monitoring.MetricPoint,
	target string,
	oldestTimestamp *int64,
) int {
	added := 0
	for _, point := range points {
		ts := point.Timestamp.Unix() * 1000
		if ts <= 0 {
			continue
		}
		if ts < *oldestTimestamp {
			*oldestTimestamp = ts
		}
		bucketTs := workloadSummaryBucketTimestamp(ts)
		bucket := getOrCreateWorkloadBucket(buckets, bucketTs)
		value := clampNonNegativeWorkloadValue(point.Value)
		switch target {
		case "cpu", "memory", "disk":
			value = clampWorkloadPercent(value)
		}
		switch target {
		case "cpu":
			bucket.cpu = append(bucket.cpu, value)
		case "memory":
			bucket.memory = append(bucket.memory, value)
		case "disk":
			bucket.disk = append(bucket.disk, value)
		case "network":
			bucket.network = append(bucket.network, value)
		}
		added++
	}
	return added
}

func mergeWorkloadNetworkPoints(
	netIn []monitoring.MetricPoint,
	netOut []monitoring.MetricPoint,
) []monitoring.MetricPoint {
	totals := make(map[int64]float64)
	for _, point := range netIn {
		ts := point.Timestamp.Unix() * 1000
		if ts <= 0 {
			continue
		}
		totals[ts] += clampNonNegativeWorkloadValue(point.Value)
	}
	for _, point := range netOut {
		ts := point.Timestamp.Unix() * 1000
		if ts <= 0 {
			continue
		}
		totals[ts] += clampNonNegativeWorkloadValue(point.Value)
	}
	if len(totals) == 0 {
		return nil
	}
	keys := make([]int64, 0, len(totals))
	for ts := range totals {
		keys = append(keys, ts)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	points := make([]monitoring.MetricPoint, 0, len(keys))
	for _, ts := range keys {
		points = append(points, monitoring.MetricPoint{
			Timestamp: time.UnixMilli(ts),
			Value:     totals[ts],
		})
	}
	return points
}

func averageValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func maxValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for i := 1; i < len(values); i++ {
		if values[i] > max {
			max = values[i]
		}
	}
	return max
}

func buildWorkloadsSummaryMetric(
	buckets map[int64]*workloadSummaryBuckets,
	selector func(*workloadSummaryBuckets) []float64,
) WorkloadsSummaryMetricData {
	keys := make([]int64, 0, len(buckets))
	for ts := range buckets {
		keys = append(keys, ts)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	data := WorkloadsSummaryMetricData{
		P50: make([]MetricPoint, 0, len(keys)),
		P95: make([]MetricPoint, 0, len(keys)),
	}
	for _, ts := range keys {
		values := selector(buckets[ts])
		if len(values) == 0 {
			continue
		}
		data.P50 = append(data.P50, MetricPoint{
			Timestamp: ts,
			Value:     averageValue(values),
		})
		data.P95 = append(data.P95, MetricPoint{
			Timestamp: ts,
			Value:     maxValue(values),
		})
	}
	return data
}

func summaryMetricPointCount(metric WorkloadsSummaryMetricData) int {
	return len(metric.P50) + len(metric.P95)
}

func latestSummaryMetricValue(points []monitoring.MetricPoint, fallback float64, clamp func(float64) float64) float64 {
	if len(points) == 0 {
		return clamp(fallback)
	}

	latest := points[0]
	for i := 1; i < len(points); i++ {
		if points[i].Timestamp.After(latest.Timestamp) {
			latest = points[i]
		}
	}
	return clamp(latest.Value)
}

func buildWorkloadsTopContributors(
	snapshots []workloadsSummarySnapshot,
	selector func(workloadsSummarySnapshot) float64,
) []WorkloadsSummaryContributor {
	contributors := make([]WorkloadsSummaryContributor, 0, len(snapshots))
	for _, snapshot := range snapshots {
		value := selector(snapshot)
		if value <= 0 {
			continue
		}
		contributors = append(contributors, WorkloadsSummaryContributor{
			ID:    snapshot.id,
			Name:  snapshot.name,
			Value: value,
		})
	}

	sort.Slice(contributors, func(i, j int) bool {
		if contributors[i].Value == contributors[j].Value {
			if contributors[i].Name == contributors[j].Name {
				return contributors[i].ID < contributors[j].ID
			}
			return contributors[i].Name < contributors[j].Name
		}
		return contributors[i].Value > contributors[j].Value
	})

	if len(contributors) > 3 {
		contributors = contributors[:3]
	}
	return contributors
}

func buildWorkloadsBlastRadius(
	snapshots []workloadsSummarySnapshot,
	selector func(workloadsSummarySnapshot) float64,
) WorkloadsSummaryBlastRadius {
	values := make([]float64, 0, len(snapshots))
	for _, snapshot := range snapshots {
		value := selector(snapshot)
		if value <= 0 {
			continue
		}
		values = append(values, value)
	}

	if len(values) == 0 {
		return WorkloadsSummaryBlastRadius{
			Scope:           "idle",
			Top3Share:       0,
			ActiveWorkloads: 0,
		}
	}

	sort.Slice(values, func(i, j int) bool { return values[i] > values[j] })
	total := 0.0
	for _, value := range values {
		total += value
	}

	topCount := 3
	if len(values) < topCount {
		topCount = len(values)
	}
	top3 := 0.0
	for i := 0; i < topCount; i++ {
		top3 += values[i]
	}

	share := 0.0
	if total > 0 {
		share = (top3 / total) * 100
	}

	scope := "distributed"
	switch {
	case share >= 80:
		scope = "concentrated"
	case share >= 55:
		scope = "mixed"
	}

	return WorkloadsSummaryBlastRadius{
		Scope:           scope,
		Top3Share:       share,
		ActiveWorkloads: len(values),
	}
}

// handleWorkloadsSummaryCharts serves compact, aggregate workload sparklines
// for the Workloads top cards. It intentionally avoids returning per-workload
// time series to keep payloads bounded for large fleets.
func (r *Router) handleWorkloadsSummaryCharts(w http.ResponseWriter, req *http.Request) {
	log.Debug().Str("method", req.Method).Str("url", req.URL.String()).Msg("Workloads summary charts endpoint hit")
	const inMemoryChartThreshold = 2 * time.Hour

	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := req.URL.Query()
	timeRange := query.Get("range")
	if timeRange == "" {
		timeRange = "1h"
	}
	selectedNodeID := strings.TrimSpace(query.Get("node"))
	duration := parseChartsRangeDuration(timeRange)

	monitor := r.getTenantMonitor(req.Context())
	state := monitor.GetState()
	mockModeEnabled := mock.IsMockEnabled()
	metricsStoreEnabled := monitor.GetMetricsStore() != nil
	primarySourceHint := "memory"
	if metricsStoreEnabled && duration > inMemoryChartThreshold {
		primarySourceHint = "store_or_memory_fallback"
	}

	currentTime := time.Now().Unix() * 1000
	currentTimeTime := time.UnixMilli(currentTime)
	oldestTimestamp := currentTime
	buckets := make(map[int64]*workloadSummaryBuckets)
	guestPointCount := 0
	guestCounts := WorkloadsGuestCounts{}
	snapshots := make([]workloadsSummarySnapshot, 0, len(state.VMs)+len(state.Containers))

	var selectedNode *models.Node
	if selectedNodeID != "" {
		for idx := range state.Nodes {
			if state.Nodes[idx].ID == selectedNodeID {
				selectedNode = &state.Nodes[idx]
				break
			}
		}
		if selectedNode == nil {
			log.Debug().
				Str("selectedNodeID", selectedNodeID).
				Msg("Workloads summary node filter not found in current state; falling back to global scope")
		}
	}

	matchesSelectedNode := func(instance, nodeName string) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		return strings.EqualFold(strings.TrimSpace(instance), strings.TrimSpace(selectedNode.Instance)) &&
			strings.EqualFold(strings.TrimSpace(nodeName), strings.TrimSpace(selectedNode.Name))
	}

	matchesSelectedDockerHost := func(host models.DockerHost) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		nodeName := strings.TrimSpace(selectedNode.Name)
		if nodeName == "" {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(host.Hostname), nodeName) ||
			strings.EqualFold(strings.TrimSpace(host.DisplayName), nodeName)
	}

	matchesSelectedKubernetesPod := func(pod models.KubernetesPod) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		nodeName := strings.TrimSpace(selectedNode.Name)
		if nodeName == "" {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(pod.NodeName), nodeName)
	}

	for _, vm := range state.VMs {
		if !matchesSelectedNode(vm.Instance, vm.Node) {
			continue
		}
		guestCounts.Total++
		if strings.EqualFold(vm.Status, "running") {
			guestCounts.Running++
		} else {
			guestCounts.Stopped++
		}

		snapshot := workloadsSummarySnapshot{
			id:      vm.ID,
			name:    strings.TrimSpace(vm.Name),
			cpu:     clampWorkloadPercent(vm.CPU * 100),
			memory:  clampWorkloadPercent(vm.Memory.Usage),
			disk:    clampWorkloadPercent(vm.Disk.Usage),
			network: clampNonNegativeWorkloadValue(float64(vm.NetworkIn) + float64(vm.NetworkOut)),
		}
		if snapshot.name == "" {
			snapshot.name = vm.ID
		}

		metrics := monitor.GetGuestMetricsForChart(vm.ID, "vm", vm.ID, duration)
		cpuPoints := metrics["cpu"]
		if len(cpuPoints) == 0 {
			cpuPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: vm.CPU * 100}}
		}
		memoryPoints := metrics["memory"]
		if len(memoryPoints) == 0 {
			memoryPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: vm.Memory.Usage}}
		}
		diskPoints := metrics["disk"]
		if len(diskPoints) == 0 {
			diskPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: vm.Disk.Usage}}
		}
		netInPoints := metrics["netin"]
		netOutPoints := metrics["netout"]
		if len(netInPoints) == 0 && len(netOutPoints) == 0 {
			netInPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: float64(vm.NetworkIn)}}
			netOutPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: float64(vm.NetworkOut)}}
		}

		networkPoints := mergeWorkloadNetworkPoints(netInPoints, netOutPoints)

		snapshot.cpu = latestSummaryMetricValue(cpuPoints, snapshot.cpu, clampWorkloadPercent)
		snapshot.memory = latestSummaryMetricValue(memoryPoints, snapshot.memory, clampWorkloadPercent)
		snapshot.disk = latestSummaryMetricValue(diskPoints, snapshot.disk, clampWorkloadPercent)
		snapshot.network = latestSummaryMetricValue(networkPoints, snapshot.network, clampNonNegativeWorkloadValue)

		guestPointCount += appendWorkloadMetricPoints(buckets, cpuPoints, "cpu", &oldestTimestamp)
		guestPointCount += appendWorkloadMetricPoints(buckets, memoryPoints, "memory", &oldestTimestamp)
		guestPointCount += appendWorkloadMetricPoints(buckets, diskPoints, "disk", &oldestTimestamp)
		guestPointCount += appendWorkloadMetricPoints(buckets, networkPoints, "network", &oldestTimestamp)
		snapshots = append(snapshots, snapshot)
	}

	for _, ct := range state.Containers {
		if !matchesSelectedNode(ct.Instance, ct.Node) {
			continue
		}
		guestCounts.Total++
		if strings.EqualFold(ct.Status, "running") {
			guestCounts.Running++
		} else {
			guestCounts.Stopped++
		}

		snapshot := workloadsSummarySnapshot{
			id:      ct.ID,
			name:    strings.TrimSpace(ct.Name),
			cpu:     clampWorkloadPercent(ct.CPU * 100),
			memory:  clampWorkloadPercent(ct.Memory.Usage),
			disk:    clampWorkloadPercent(ct.Disk.Usage),
			network: clampNonNegativeWorkloadValue(float64(ct.NetworkIn) + float64(ct.NetworkOut)),
		}
		if snapshot.name == "" {
			snapshot.name = ct.ID
		}

		metrics := monitor.GetGuestMetricsForChart(ct.ID, "container", ct.ID, duration)
		cpuPoints := metrics["cpu"]
		if len(cpuPoints) == 0 {
			cpuPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: ct.CPU * 100}}
		}
		memoryPoints := metrics["memory"]
		if len(memoryPoints) == 0 {
			memoryPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: ct.Memory.Usage}}
		}
		diskPoints := metrics["disk"]
		if len(diskPoints) == 0 {
			diskPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: ct.Disk.Usage}}
		}
		netInPoints := metrics["netin"]
		netOutPoints := metrics["netout"]
		if len(netInPoints) == 0 && len(netOutPoints) == 0 {
			netInPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: float64(ct.NetworkIn)}}
			netOutPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: float64(ct.NetworkOut)}}
		}

		networkPoints := mergeWorkloadNetworkPoints(netInPoints, netOutPoints)

		snapshot.cpu = latestSummaryMetricValue(cpuPoints, snapshot.cpu, clampWorkloadPercent)
		snapshot.memory = latestSummaryMetricValue(memoryPoints, snapshot.memory, clampWorkloadPercent)
		snapshot.disk = latestSummaryMetricValue(diskPoints, snapshot.disk, clampWorkloadPercent)
		snapshot.network = latestSummaryMetricValue(networkPoints, snapshot.network, clampNonNegativeWorkloadValue)

		guestPointCount += appendWorkloadMetricPoints(buckets, cpuPoints, "cpu", &oldestTimestamp)
		guestPointCount += appendWorkloadMetricPoints(buckets, memoryPoints, "memory", &oldestTimestamp)
		guestPointCount += appendWorkloadMetricPoints(buckets, diskPoints, "disk", &oldestTimestamp)
		guestPointCount += appendWorkloadMetricPoints(buckets, networkPoints, "network", &oldestTimestamp)
		snapshots = append(snapshots, snapshot)
	}

	for _, cluster := range state.KubernetesClusters {
		if cluster.Hidden {
			continue
		}
		for _, pod := range cluster.Pods {
			if !matchesSelectedKubernetesPod(pod) {
				continue
			}

			metricKey := kubernetesPodMetricID(cluster, pod)
			if metricKey == "" {
				continue
			}
			currentMetrics := kubernetesPodCurrentMetrics(cluster, pod)

			guestCounts.Total++
			if kubernetesPodIsRunning(pod) {
				guestCounts.Running++
			} else {
				guestCounts.Stopped++
			}

			snapshot := workloadsSummarySnapshot{
				id:      metricKey,
				name:    kubernetesPodDisplayName(pod),
				cpu:     clampWorkloadPercent(currentMetrics["cpu"]),
				memory:  clampWorkloadPercent(currentMetrics["memory"]),
				disk:    clampWorkloadPercent(currentMetrics["disk"]),
				network: clampNonNegativeWorkloadValue(currentMetrics["netin"] + currentMetrics["netout"]),
			}
			if snapshot.name == "" {
				snapshot.name = metricKey
			}

			metrics := monitor.GetGuestMetricsForChart(metricKey, "k8s", metricKey, duration)
			cpuPoints := metrics["cpu"]
			if len(cpuPoints) == 0 {
				cpuPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: currentMetrics["cpu"]}}
			}
			memoryPoints := metrics["memory"]
			if len(memoryPoints) == 0 {
				memoryPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: currentMetrics["memory"]}}
			}
			diskPoints := metrics["disk"]
			if len(diskPoints) == 0 {
				diskPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: currentMetrics["disk"]}}
			}
			netInPoints := metrics["netin"]
			if len(netInPoints) == 0 {
				netInPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: currentMetrics["netin"]}}
			}
			netOutPoints := metrics["netout"]
			if len(netOutPoints) == 0 {
				netOutPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: currentMetrics["netout"]}}
			}

			if mockModeEnabled {
				if len(cpuPoints) < mockWorkloadMinSeriesPoints {
					cpuPoints = buildMockWorkloadMetricHistorySeries(currentTimeTime, duration, 0, metricKey, "cpu", currentMetrics["cpu"])
				}
				if len(memoryPoints) < mockWorkloadMinSeriesPoints {
					memoryPoints = buildMockWorkloadMetricHistorySeries(currentTimeTime, duration, 0, metricKey, "memory", currentMetrics["memory"])
				}
				if len(diskPoints) < mockWorkloadMinSeriesPoints {
					diskPoints = buildMockWorkloadMetricHistorySeries(currentTimeTime, duration, 0, metricKey, "disk", currentMetrics["disk"])
				}
				if len(netInPoints) < mockWorkloadMinSeriesPoints {
					netInPoints = buildMockWorkloadMetricHistorySeries(currentTimeTime, duration, 0, metricKey, "netin", currentMetrics["netin"])
				}
				if len(netOutPoints) < mockWorkloadMinSeriesPoints {
					netOutPoints = buildMockWorkloadMetricHistorySeries(currentTimeTime, duration, 0, metricKey, "netout", currentMetrics["netout"])
				}
			}

			networkPoints := mergeWorkloadNetworkPoints(netInPoints, netOutPoints)

			snapshot.cpu = latestSummaryMetricValue(cpuPoints, snapshot.cpu, clampWorkloadPercent)
			snapshot.memory = latestSummaryMetricValue(memoryPoints, snapshot.memory, clampWorkloadPercent)
			snapshot.disk = latestSummaryMetricValue(diskPoints, snapshot.disk, clampWorkloadPercent)
			snapshot.network = latestSummaryMetricValue(networkPoints, snapshot.network, clampNonNegativeWorkloadValue)

			guestPointCount += appendWorkloadMetricPoints(buckets, cpuPoints, "cpu", &oldestTimestamp)
			guestPointCount += appendWorkloadMetricPoints(buckets, memoryPoints, "memory", &oldestTimestamp)
			guestPointCount += appendWorkloadMetricPoints(buckets, diskPoints, "disk", &oldestTimestamp)
			guestPointCount += appendWorkloadMetricPoints(buckets, networkPoints, "network", &oldestTimestamp)
			snapshots = append(snapshots, snapshot)
		}
	}

	for _, host := range state.DockerHosts {
		if !matchesSelectedDockerHost(host) {
			continue
		}
		for _, container := range host.Containers {
			if container.ID == "" {
				continue
			}
			guestCounts.Total++
			if strings.EqualFold(container.State, "running") {
				guestCounts.Running++
			} else {
				guestCounts.Stopped++
			}

			diskFallback := 0.0
			if container.RootFilesystemBytes > 0 && container.WritableLayerBytes > 0 {
				diskFallback = float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
			}
			snapshot := workloadsSummarySnapshot{
				id:      container.ID,
				name:    strings.TrimSpace(container.Name),
				cpu:     clampWorkloadPercent(container.CPUPercent),
				memory:  clampWorkloadPercent(container.MemoryPercent),
				disk:    clampWorkloadPercent(diskFallback),
				network: 0,
			}
			if snapshot.name == "" {
				snapshot.name = container.ID
			}

			metricKey := fmt.Sprintf("docker:%s", container.ID)
			metrics := monitor.GetGuestMetricsForChart(metricKey, "dockerContainer", container.ID, duration)
			cpuPoints := metrics["cpu"]
			if len(cpuPoints) == 0 {
				cpuPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: container.CPUPercent}}
			}
			memoryPoints := metrics["memory"]
			if len(memoryPoints) == 0 {
				memoryPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: container.MemoryPercent}}
			}
			diskPoints := metrics["disk"]
			if len(diskPoints) == 0 {
				diskPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: diskFallback}}
			}
			netInPoints := metrics["netin"]
			netOutPoints := metrics["netout"]

			networkPoints := mergeWorkloadNetworkPoints(netInPoints, netOutPoints)

			snapshot.cpu = latestSummaryMetricValue(cpuPoints, snapshot.cpu, clampWorkloadPercent)
			snapshot.memory = latestSummaryMetricValue(memoryPoints, snapshot.memory, clampWorkloadPercent)
			snapshot.disk = latestSummaryMetricValue(diskPoints, snapshot.disk, clampWorkloadPercent)
			snapshot.network = latestSummaryMetricValue(networkPoints, snapshot.network, clampNonNegativeWorkloadValue)

			guestPointCount += appendWorkloadMetricPoints(buckets, cpuPoints, "cpu", &oldestTimestamp)
			guestPointCount += appendWorkloadMetricPoints(buckets, memoryPoints, "memory", &oldestTimestamp)
			guestPointCount += appendWorkloadMetricPoints(buckets, diskPoints, "disk", &oldestTimestamp)
			guestPointCount += appendWorkloadMetricPoints(buckets, networkPoints, "network", &oldestTimestamp)
			snapshots = append(snapshots, snapshot)
		}
	}

	cpuMetric := buildWorkloadsSummaryMetric(buckets, func(bucket *workloadSummaryBuckets) []float64 {
		return bucket.cpu
	})
	memoryMetric := buildWorkloadsSummaryMetric(buckets, func(bucket *workloadSummaryBuckets) []float64 {
		return bucket.memory
	})
	diskMetric := buildWorkloadsSummaryMetric(buckets, func(bucket *workloadSummaryBuckets) []float64 {
		return bucket.disk
	})
	networkMetric := buildWorkloadsSummaryMetric(buckets, func(bucket *workloadSummaryBuckets) []float64 {
		return bucket.network
	})

	summaryPointCount := summaryMetricPointCount(cpuMetric) +
		summaryMetricPointCount(memoryMetric) +
		summaryMetricPointCount(diskMetric) +
		summaryMetricPointCount(networkMetric)

	topContributors := WorkloadsSummaryContributors{
		CPU: buildWorkloadsTopContributors(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.cpu
		}),
		Memory: buildWorkloadsTopContributors(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.memory
		}),
		Disk: buildWorkloadsTopContributors(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.disk
		}),
		Network: buildWorkloadsTopContributors(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.network
		}),
	}

	blastRadius := WorkloadsSummaryBlastRadiusGroup{
		CPU: buildWorkloadsBlastRadius(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.cpu
		}),
		Memory: buildWorkloadsBlastRadius(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.memory
		}),
		Disk: buildWorkloadsBlastRadius(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.disk
		}),
		Network: buildWorkloadsBlastRadius(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.network
		}),
	}

	response := WorkloadsSummaryChartsResponse{
		CPU:             cpuMetric,
		Memory:          memoryMetric,
		Disk:            diskMetric,
		Network:         networkMetric,
		GuestCounts:     guestCounts,
		TopContributors: topContributors,
		BlastRadius:     blastRadius,
		Timestamp:       currentTime,
		Stats: ChartStats{
			OldestDataTimestamp:   oldestTimestamp,
			Range:                 timeRange,
			RangeSeconds:          int64(duration / time.Second),
			MetricsStoreEnabled:   metricsStoreEnabled,
			PrimarySourceHint:     primarySourceHint,
			InMemoryThresholdSecs: int64(inMemoryChartThreshold / time.Second),
			PointCounts: ChartPointCounts{
				Total:  summaryPointCount,
				Guests: guestPointCount,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode workloads summary chart data response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
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

	// Use tenant-aware monitor
	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		http.Error(w, "Monitor not available", http.StatusInternalServerError)
		return
	}
	state := monitor.GetState()

	// Build storage chart data
	storageData := make(StorageChartsResponse)

	for _, storage := range state.Storage {
		metrics := monitor.GetStorageMetricsForChart(storage.ID, duration)

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

	// Use tenant-aware monitor
	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		http.Error(w, "Monitor not available", http.StatusInternalServerError)
		return
	}

	store := monitor.GetMetricsStore()
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

	// Use tenant-aware monitor
	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		http.Error(w, "Monitor not available", http.StatusInternalServerError)
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

	// Normalize resource types so frontend aliases match the SQLite store keys.
	switch resourceType {
	case "docker":
		resourceType = "dockerContainer"
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

	// Enforce license limits: 7d free, longer ranges require Pro
	maxFreeDuration := 7 * 24 * time.Hour
	if duration > maxFreeDuration && !r.licenseHandlers.Service(req.Context()).HasFeature(license.FeatureLongTermMetrics) {
		WriteLicenseRequired(w, license.FeatureLongTermMetrics, "Long-term metrics history requires a Pulse Pro license")
		return
	}

	end := time.Now()
	start := end.Add(-duration)

	const (
		historySourceStore  = "store"
		historySourceMemory = "memory"
		historySourceLive   = "live"
		historySourceMock   = "mock_synthetic"
	)

	// Metric aliasing: storage metrics are stored under "usage", but some clients request "disk".
	// Keep metricType unchanged for the response JSON; only alias the lookup/query key.
	queryMetric := metricType
	if resourceType == "storage" && metricType == "disk" {
		queryMetric = "usage"
	}

	// Allow in-memory fallback for any requested range when the persistent store is empty.
	// The in-memory history enforces its own retention limits, so it will naturally return
	// whatever data is available (better than showing "Collecting data..." indefinitely).
	fallbackAllowed := true
	historyMaxPoints := parseWorkloadMaxPoints(query.Get("maxPoints"))
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
	state := monitor.GetState()
	mockModeEnabled := mock.IsMockEnabled()

	parseGuestID := func(id string) (string, string, int, bool) {
		parts := strings.Split(id, ":")
		if len(parts) != 3 {
			return "", "", 0, false
		}
		vmID, err := strconv.Atoi(parts[2])
		if err != nil {
			return "", "", 0, false
		}
		return parts[0], parts[1], vmID, true
	}

	findVM := func(id string) *models.VM {
		for i := range state.VMs {
			if state.VMs[i].ID == id {
				return &state.VMs[i]
			}
		}
		if instance, node, vmID, ok := parseGuestID(id); ok {
			for i := range state.VMs {
				vm := &state.VMs[i]
				if vm.VMID == vmID && vm.Node == node && vm.Instance == instance {
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
		if instance, node, vmID, ok := parseGuestID(id); ok {
			for i := range state.Containers {
				ct := &state.Containers[i]
				if ct.VMID == vmID && ct.Node == node && ct.Instance == instance {
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

	findHost := func(id string) *models.Host {
		for i := range state.Hosts {
			if state.Hosts[i].ID == id {
				return &state.Hosts[i]
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

	findDisk := func(id string) *unifiedresources.Resource {
		if r.aiUnifiedAdapter == nil {
			return nil
		}
		for _, res := range r.aiUnifiedAdapter.GetByType(unifiedresources.ResourceTypePhysicalDisk) {
			if res.PhysicalDisk == nil {
				continue
			}
			if res.PhysicalDisk.Serial == id || res.PhysicalDisk.WWN == id || res.ID == id {
				return &res
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
		case "host":
			host := findHost(resourceID)
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
			// Note: We intentionally don't include netin/netout here because the host model
			// only has cumulative RXBytes/TXBytes (total since boot), not rates.
			// The RateTracker in ApplyHostReport calculates rates and stores them in metrics history.
			// Showing cumulative bytes as if they were rates would be misleading (showing GB instead of KB/s).
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
		case "disk":
			disk := findDisk(resourceID)
			if disk == nil || disk.PhysicalDisk == nil {
				return points
			}
			pd := disk.PhysicalDisk
			if pd.Temperature > 0 {
				points["smart_temp"] = monitoring.MetricPoint{Timestamp: now, Value: float64(pd.Temperature)}
			}
			if pd.SMART != nil {
				s := pd.SMART
				if s.PowerOnHours > 0 {
					points["smart_power_on_hours"] = monitoring.MetricPoint{Timestamp: now, Value: float64(s.PowerOnHours)}
				}
				if s.PowerCycles > 0 {
					points["smart_power_cycles"] = monitoring.MetricPoint{Timestamp: now, Value: float64(s.PowerCycles)}
				}
				if s.ReallocatedSectors > 0 {
					points["smart_reallocated_sectors"] = monitoring.MetricPoint{Timestamp: now, Value: float64(s.ReallocatedSectors)}
				}
				if s.PendingSectors > 0 {
					points["smart_pending_sectors"] = monitoring.MetricPoint{Timestamp: now, Value: float64(s.PendingSectors)}
				}
				if s.OfflineUncorrectable > 0 {
					points["smart_offline_uncorrectable"] = monitoring.MetricPoint{Timestamp: now, Value: float64(s.OfflineUncorrectable)}
				}
				if s.UDMACRCErrors > 0 {
					points["smart_crc_errors"] = monitoring.MetricPoint{Timestamp: now, Value: float64(s.UDMACRCErrors)}
				}
				if s.PercentageUsed > 0 {
					points["smart_percentage_used"] = monitoring.MetricPoint{Timestamp: now, Value: float64(s.PercentageUsed)}
				}
				if s.AvailableSpare > 0 {
					points["smart_available_spare"] = monitoring.MetricPoint{Timestamp: now, Value: float64(s.AvailableSpare)}
				}
				if s.MediaErrors > 0 {
					points["smart_media_errors"] = monitoring.MetricPoint{Timestamp: now, Value: float64(s.MediaErrors)}
				}
				if s.UnsafeShutdowns > 0 {
					points["smart_unsafe_shutdowns"] = monitoring.MetricPoint{Timestamp: now, Value: float64(s.UnsafeShutdowns)}
				}
			}
		}

		return points
	}

	fallbackSingle := func() ([]map[string]interface{}, string, bool) {
		if !fallbackAllowed || metricType == "" {
			return nil, "", false
		}

		if mockModeEnabled && resourceType == "disk" && metricType == "smart_temp" {
			if disk := findDisk(resourceID); disk != nil && disk.PhysicalDisk != nil && disk.PhysicalDisk.Temperature > 0 {
				series := buildSyntheticMetricHistorySeries(
					end,
					duration,
					historyMaxPoints,
					resourceID,
					metricType,
					float64(disk.PhysicalDisk.Temperature),
				)
				if len(series) > 0 {
					return buildHistoryPoints(series, stepSecs), historySourceMock, true
				}
			}
		}

		switch resourceType {
		case "vm", "container", "guest":
			metrics := monitor.GetGuestMetrics(resourceID, duration)
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
			metrics := monitor.GetGuestMetrics(fmt.Sprintf("dockerHost:%s", resourceID), duration)
			points := metrics[metricType]
			if len(points) == 0 {
				livePoints := liveMetricPoints(resourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), historySourceMemory, true
		case "host":
			metrics := monitor.GetGuestMetrics(fmt.Sprintf("host:%s", resourceID), duration)
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
			metrics := monitor.GetGuestMetrics(fmt.Sprintf("docker:%s", resourceID), duration)
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
			points := monitor.GetNodeMetrics(resourceID, metricType, duration)
			if len(points) == 0 {
				livePoints := liveMetricPoints(resourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), historySourceMemory, true
		case "storage":
			metrics := monitor.GetStorageMetrics(resourceID, duration)
			points := metrics[queryMetric]
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
			metrics = monitor.GetGuestMetrics(resourceID, duration)
		case "dockerHost":
			metrics = monitor.GetGuestMetrics(fmt.Sprintf("dockerHost:%s", resourceID), duration)
		case "host":
			metrics = monitor.GetGuestMetrics(fmt.Sprintf("host:%s", resourceID), duration)
		case "docker", "dockerContainer":
			metrics = monitor.GetGuestMetrics(fmt.Sprintf("docker:%s", resourceID), duration)
		case "storage":
			metrics = monitor.GetStorageMetrics(resourceID, duration)
		default:
			if resourceType == "node" {
				metrics = map[string][]monitoring.MetricPoint{
					"cpu":    monitor.GetNodeMetrics(resourceID, "cpu", duration),
					"memory": monitor.GetNodeMetrics(resourceID, "memory", duration),
					"disk":   monitor.GetNodeMetrics(resourceID, "disk", duration),
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

	store := monitor.GetMetricsStore()
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
		points, err := store.Query(resourceType, resourceID, queryMetric, start, end, stepSecs)
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

		if response == nil && mockModeEnabled && resourceType == "disk" && metricType == "smart_temp" {
			targetPoints := targetMockSeriesPoints(duration, historyMaxPoints)
			if len(points) > 0 && len(points) < targetPoints {
				current := points[len(points)-1].Value
				if disk := findDisk(resourceID); disk != nil && disk.PhysicalDisk != nil && disk.PhysicalDisk.Temperature > 0 {
					current = float64(disk.PhysicalDisk.Temperature)
				}
				series := buildSyntheticMetricHistorySeries(
					end,
					duration,
					historyMaxPoints,
					resourceID,
					metricType,
					current,
				)
				if len(series) > len(points) {
					source = historySourceMock
					response = map[string]interface{}{
						"resourceType": resourceType,
						"resourceId":   resourceID,
						"metric":       metricType,
						"range":        timeRange,
						"start":        start.UnixMilli(),
						"end":          end.UnixMilli(),
						"points":       buildHistoryPoints(series, stepSecs),
						"source":       source,
					}
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

// handleWebSocket handles WebSocket connections
func (r *Router) handleWebSocket(w http.ResponseWriter, req *http.Request) {
	// Check authentication before allowing WebSocket upgrade
	if !CheckAuth(r.config, w, req) {
		return
	}
	// SECURITY: Ensure monitoring:read scope for WebSocket connections
	// This prevents tokens with only agent scopes from accessing full infra state via requestData
	if !ensureScope(w, req, config.ScopeMonitoringRead) {
		return
	}

	boundReq, ok := bindWebSocketOrgToTenantContext(w, req)
	if !ok {
		return
	}

	r.wsHub.HandleWebSocket(w, boundReq)
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
	// SECURITY: Ensure authentication is checked for socket.io transport upgrades
	if !CheckAuth(r.config, w, req) {
		return
	}
	// SECURITY: Ensure monitoring:read scope for socket.io connections
	if !ensureScope(w, req, config.ScopeMonitoringRead) {
		return
	}
	// For socket.io.js, redirect to CDN
	if strings.Contains(req.URL.Path, "socket.io.js") {
		http.Redirect(w, req, "https://cdn.socket.io/4.8.1/socket.io.min.js", http.StatusFound)
		return
	}

	// For other socket.io endpoints, use our WebSocket
	// This provides basic compatibility
	if strings.Contains(req.URL.RawQuery, "transport=websocket") {
		boundReq, ok := bindWebSocketOrgToTenantContext(w, req)
		if !ok {
			return
		}
		r.wsHub.HandleWebSocket(w, boundReq)
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

func resolveExplicitWebSocketOrgID(req *http.Request) (string, bool) {
	if req == nil {
		return "", false
	}

	if headerOrg := strings.TrimSpace(req.Header.Get("X-Pulse-Org-ID")); headerOrg != "" {
		return headerOrg, true
	}

	if cookie, err := req.Cookie("pulse_org_id"); err == nil {
		if cookieOrg := strings.TrimSpace(cookie.Value); cookieOrg != "" {
			return cookieOrg, true
		}
	}

	if queryOrg := strings.TrimSpace(req.URL.Query().Get("org_id")); queryOrg != "" {
		return queryOrg, true
	}

	return "", false
}

func bindWebSocketOrgToTenantContext(w http.ResponseWriter, req *http.Request) (*http.Request, bool) {
	if req == nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return nil, false
	}

	contextOrgID := strings.TrimSpace(GetOrgID(req.Context()))
	if contextOrgID == "" {
		contextOrgID = "default"
	}

	if requestedOrgID, explicit := resolveExplicitWebSocketOrgID(req); explicit {
		if !isValidOrganizationID(requestedOrgID) {
			http.Error(w, "Invalid organization ID", http.StatusBadRequest)
			return nil, false
		}
		if requestedOrgID != contextOrgID {
			http.Error(w, "Unauthorized organization context", http.StatusForbidden)
			return nil, false
		}
	}

	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	cloned.Header.Set("X-Pulse-Org-ID", contextOrgID)
	return cloned, true
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
		if r.wsHub != nil {
			r.wsHub.BroadcastMessage(message)
		}

		// Log progress
		log.Debug().
			Str("status", status.Status).
			Int("progress", status.Progress).
			Str("message", status.Message).
			Msg("Update progress")
	}
}

// backgroundUpdateChecker periodically checks for updates and caches the result
func (r *Router) backgroundUpdateChecker(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Delay initial check to allow WebSocket clients to receive welcome messages first
	startupDelay := time.NewTimer(1 * time.Second)
	defer startupDelay.Stop()

	select {
	case <-ctx.Done():
		return
	case <-startupDelay.C:
	}

	if _, err := r.updateManager.CheckForUpdates(ctx); err != nil {
		log.Debug().Err(err).Msg("Initial update check failed")
	} else {
		log.Info().Msg("Initial update check completed")
	}

	// Then check every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := r.updateManager.CheckForUpdates(ctx); err != nil {
				log.Debug().Err(err).Msg("Periodic update check failed")
			} else {
				log.Debug().Msg("Periodic update check completed")
			}
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
		defer file.Close()

		w.Header().Set("X-Checksum-Sha256", checksum)
		http.ServeContent(w, req, filepath.Base(candidate), info.ModTime(), file)
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

	remainingMissing := updates.EnsureHostAgentBinaries(r.serverVersion)

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

func sortedHostAgentKeys(missing map[string]updates.HostAgentBinary) []string {
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

	orgID := strings.TrimSpace(GetOrgID(req.Context()))

	monitor := r.getTenantMonitor(req.Context())
	if orgID != "" && orgID != "default" {
		// Security-sensitive endpoint: do not fall back to the default monitor for tenant-scoped requests.
		if r.mtMonitor == nil {
			writeErrorResponse(w, http.StatusInternalServerError, "tenant_unavailable", "Tenant monitor is not configured", nil)
			return
		}
		tenantMonitor, err := r.mtMonitor.GetMonitor(orgID)
		if err != nil || tenantMonitor == nil {
			writeErrorResponse(w, http.StatusInternalServerError, "tenant_unavailable", "Failed to resolve tenant monitor", nil)
			return
		}
		monitor = tenantMonitor
	}
	if monitor == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "monitor_unavailable", "Monitor is not configured", nil)
		return
	}

	host, ok := monitor.GetDockerHost(hostID)
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

	if orgID != "" && orgID != "default" {
		record.OrgID = orgID
	}

	activeConfig := r.config
	activePersistence := r.persistence
	if orgID != "" && orgID != "default" {
		activeConfig = monitor.GetConfig()
		if activeConfig == nil {
			writeErrorResponse(w, http.StatusInternalServerError, "tenant_config_unavailable", "Tenant config is not available", nil)
			return
		}
		if r.multiTenant == nil {
			writeErrorResponse(w, http.StatusInternalServerError, "tenant_persistence_unavailable", "Tenant persistence is not configured", nil)
			return
		}
		tenantPersistence, err := r.multiTenant.GetPersistence(orgID)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "tenant_persistence_unavailable", "Failed to resolve tenant persistence", nil)
			return
		}
		activePersistence = tenantPersistence
	}
	if activeConfig == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "config_unavailable", "Configuration is not loaded", nil)
		return
	}

	config.Mu.Lock()
	activeConfig.APITokens = append(activeConfig.APITokens, *record)
	activeConfig.SortAPITokens()

	if activePersistence != nil {
		if err := activePersistence.SaveAPITokens(activeConfig.APITokens); err != nil {
			activeConfig.RemoveAPIToken(record.ID)
			config.Mu.Unlock()
			log.Error().Err(err).Msg("Failed to persist API tokens after docker migration generation")
			writeErrorResponse(w, http.StatusInternalServerError, "token_persist_failed", "Failed to persist API token", nil)
			return
		}
	}
	config.Mu.Unlock()

	baseURL := strings.TrimRight(r.resolvePublicURL(req), "/")
	installScriptURL := baseURL + "/install-docker-agent.sh"
	installCommand := fmt.Sprintf(
		"curl -fSL %s -o /tmp/pulse-install-docker-agent.sh && sudo bash /tmp/pulse-install-docker-agent.sh --url %s --token %s && rm -f /tmp/pulse-install-docker-agent.sh",
		posixShellQuote(installScriptURL),
		posixShellQuote(baseURL),
		posixShellQuote(rawToken),
	)
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
	// Hosted mode must never fall back to request host or localhost.
	// A canonical externally-reachable URL must be configured via PublicURL / AgentConnectURL.
	if r != nil && r.hostedMode {
		if agentConnectURL := strings.TrimSpace(r.config.AgentConnectURL); agentConnectURL != "" {
			return strings.TrimRight(agentConnectURL, "/")
		}
		if publicURL := strings.TrimSpace(r.config.PublicURL); publicURL != "" {
			return strings.TrimRight(publicURL, "/")
		}
		return ""
	}

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

// knowledgeStoreProviderWrapper adapts knowledge.Store to tools.KnowledgeStoreProvider.
type knowledgeStoreProviderWrapper struct {
	store *knowledge.Store
}

func (w *knowledgeStoreProviderWrapper) SaveNote(resourceID, note, category string) error {
	if w.store == nil {
		return fmt.Errorf("knowledge store not available")
	}
	// Use resourceID as both guestID and guestName, with a generic type and category
	return w.store.SaveNote(resourceID, resourceID, "resource", category, "Note", note)
}

func (w *knowledgeStoreProviderWrapper) GetKnowledge(resourceID string, category string) []tools.KnowledgeEntry {
	if w.store == nil {
		return nil
	}

	guestKnowledge, err := w.store.GetKnowledge(resourceID)
	if err != nil || guestKnowledge == nil {
		return nil
	}

	var result []tools.KnowledgeEntry

	// If category is specified, only get notes from that category
	if category != "" {
		notes, err := w.store.GetNotesByCategory(resourceID, category)
		if err != nil {
			return nil
		}
		for _, note := range notes {
			result = append(result, tools.KnowledgeEntry{
				ID:         note.ID,
				ResourceID: resourceID,
				Note:       note.Content,
				Category:   note.Category,
				CreatedAt:  note.CreatedAt,
				UpdatedAt:  note.UpdatedAt,
			})
		}
		return result
	}

	// Otherwise return all notes
	for _, note := range guestKnowledge.Notes {
		result = append(result, tools.KnowledgeEntry{
			ID:         note.ID,
			ResourceID: resourceID,
			Note:       note.Content,
			Category:   note.Category,
			CreatedAt:  note.CreatedAt,
			UpdatedAt:  note.UpdatedAt,
		})
	}
	return result
}

// trueNASRecordsAdapter wraps a truenas.Provider to satisfy SupplementalRecordsProvider.
type trueNASRecordsAdapter struct {
	provider *truenas.Provider
}

func (a trueNASRecordsAdapter) GetCurrentRecords() []unifiedresources.IngestRecord {
	// Legacy behavior: treat this mock adapter as default-org scoped.
	return a.GetCurrentRecordsForOrg("default")
}

func (a trueNASRecordsAdapter) GetCurrentRecordsForOrg(orgID string) []unifiedresources.IngestRecord {
	if strings.TrimSpace(orgID) != "" && strings.TrimSpace(orgID) != "default" {
		return nil
	}
	if a.provider == nil {
		return nil
	}
	return a.provider.Records()
}

// trigger rebuild Fri Jan 16 10:52:41 UTC 2026
