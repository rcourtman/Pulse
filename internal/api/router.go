package api

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/adapters"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/learning"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/chartapi"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/resourceapi"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/deploy"
	"github.com/rcourtman/pulse-go-rewrite/internal/maintenancesentinel"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
	metricstore "github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
	"github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

type relayRuntimeClient interface {
	Status() relay.ClientStatus
	Close()
	SendPushNotification(relay.PushNotificationPayload) error
}

// Router handles HTTP routing
type Router struct {
	mux                             *http.ServeMux
	config                          *config.Config
	eventLogger                     routerEventLogger
	monitor                         *monitoring.Monitor            // Legacy/Default support
	mtMonitor                       *monitoring.MultiTenantMonitor // Multi-tenant manager
	chartService                    *chartapi.Service
	alertHandlers                   *AlertHandlers
	configHandlers                  *ConfigHandlers
	trueNASHandlers                 *TrueNASHandlers
	vmwareHandlers                  *VMwareHandlers
	connectionsHandlers             *ConnectionsHandlers
	availabilityHandlers            *AvailabilityHandlers
	notificationHandlers            *NotificationHandlers
	notificationQueueHandlers       *NotificationQueueHandlers
	dockerAgentHandlers             *DockerAgentHandlers
	kubernetesAgentHandlers         *KubernetesAgentHandlers
	unifiedAgentHandlers            *UnifiedAgentHandlers
	systemSettingsHandler           *SystemSettingsHandler
	aiSettingsHandler               *AISettingsHandler
	attentionHandlers               *AttentionHandlers
	aiHandler                       *AIHandler // AI chat handler
	discoveryHandlers               *DiscoveryHandlers
	resourceHandlers                *ResourceHandlers
	maintenanceVerificationHandlers *MaintenanceVerificationHandlers
	maintenanceSentinel             *maintenancesentinel.Sentinel
	agentContextHandler             *AgentContextHandler
	agentEventBroadcaster           *AgentEventBroadcaster
	resourceRegistry                *unifiedresources.ResourceRegistry
	trueNASPoller                   *monitoring.TrueNASPoller
	vmwarePoller                    *monitoring.VMwarePoller
	monitorResourceAdapter          *unifiedresources.MonitorAdapter
	monitorResourceAdapters         map[string]*unifiedresources.MonitorAdapter
	monitorAdapterMu                sync.Mutex
	monitorSupplementalRecords      map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider
	reportingHandlers               *ReportingHandlers
	configProfileHandler            *ConfigProfileHandler
	licenseHandlers                 *LicenseHandlers
	recoveryHandlers                *RecoveryHandlers
	rbacProvider                    *TenantRBACProvider
	logHandlers                     *LogHandlers
	agentExecServer                 *agentexec.Server
	deployHandlers                  *DeployHandlers
	deployStore                     *deploy.Store
	wsHub                           *websocket.Hub
	reloadFunc                      func() error
	updateManager                   *updates.Manager
	updateHistory                   *updates.UpdateHistory
	exportLimiter                   *RateLimiter
	downloadLimiter                 *RateLimiter
	signupRateLimiter               *RateLimiter
	handoffExchangeRateLimiter      *RateLimiter
	bootstrapTokenValidationLimiter *RateLimiter
	tenantRateLimiter               *TenantRateLimiter
	persistence                     *config.ConfigPersistence
	multiTenant                     *config.MultiTenantPersistence
	oidcMu                          sync.Mutex
	oidcService                     *OIDCService
	oidcManager                     *OIDCServiceManager
	samlManager                     *SAMLServiceManager
	ssoConfig                       *config.SSOConfig
	ssoAuthLoadFailed               atomic.Bool
	sessionStore                    *SessionStore
	csrfStore                       *CSRFTokenStore
	recoveryTokenStore              *RecoveryTokenStore
	authorizer                      auth.Authorizer
	wrapped                         http.Handler
	serverVersion                   string
	projectRoot                     string
	// Cached system settings to avoid loading from disk on every request
	settingsMu                sync.RWMutex
	cachedAllowEmbedding      bool
	cachedAllowedOrigins      string
	publicURLMu               sync.Mutex
	publicURLDetected         bool
	bootstrapTokenHash        string
	bootstrapTokenPath        string
	checksumMu                sync.RWMutex
	checksumCache             map[string]checksumCacheEntry
	installScriptClient       *http.Client
	relayMu                   sync.RWMutex
	relayClient               relayRuntimeClient
	relayCancel               context.CancelFunc
	relayAlertMinimumSeverity string
	lifecycleCtx              context.Context
	lifecycleCancel           context.CancelFunc
	lifecycleWG               sync.WaitGroup
	backgroundWorkersOnce     sync.Once
	hostedMode                bool
	stripeWebhookHandlers     *StripeWebhookHandlers
	patrolLifecycleMu         sync.Mutex
	startedPatrolOrgs         map[string]bool
	actionRecoveryMu          sync.Mutex
	aiAutoFixEndpoints        extensions.AIAutoFixEndpoints
	aiAlertAnalysisEndpoints  extensions.AIAlertAnalysisEndpoints

	// Per-router perf state: keeps caches and singleflight groups isolated
	// between tests and prevents cross-tenant cache pollution.
	stateComputeGroup singleflight.Group
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
	// Initialize persistent auth stores and capture the exact workers this router owns.
	sessionStore := ensureSessionStore(cfg.DataPath)
	csrfStore := ensureCSRFStore(cfg.DataPath)

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
		mux:                             http.NewServeMux(),
		config:                          cfg,
		monitor:                         monitor,
		mtMonitor:                       mtMonitor,
		wsHub:                           wsHub,
		reloadFunc:                      reloadFunc,
		updateManager:                   updateManager,
		updateHistory:                   updateHistory,
		exportLimiter:                   NewRateLimiter(5, 1*time.Minute),  // 5 attempts per minute
		downloadLimiter:                 NewRateLimiter(60, 1*time.Minute), // downloads/installers per minute per IP
		signupRateLimiter:               NewRateLimiter(5, 1*time.Hour),    // signup attempts per hour per IP
		handoffExchangeRateLimiter:      NewRateLimiter(20, 1*time.Minute), // cloud handoff token exchange per minute per IP
		bootstrapTokenValidationLimiter: NewRateLimiter(10, 5*time.Minute), // bootstrap token validation attempts per 5 minutes per IP
		persistence:                     config.NewConfigPersistence(cfg.DataPath),
		multiTenant:                     config.NewMultiTenantPersistence(cfg.DataPath),
		sessionStore:                    sessionStore,
		csrfStore:                       csrfStore,
		authorizer:                      auth.GetAuthorizer(),
		serverVersion:                   strings.TrimSpace(serverVersion),
		projectRoot:                     projectRoot,
		checksumCache:                   make(map[string]checksumCacheEntry),
		lifecycleCtx:                    lifecycleCtx,
		lifecycleCancel:                 lifecycleCancel,
		hostedMode:                      os.Getenv("PULSE_HOSTED_MODE") == "true",
		monitorResourceAdapters:         make(map[string]*unifiedresources.MonitorAdapter),
		monitorSupplementalRecords:      make(map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider),
		startedPatrolOrgs:               make(map[string]bool),
	}
	r.chartService = chartapi.NewService(routerChartMonitorResolver{router: r})
	if r.wsHub != nil {
		r.wsHub.SetTrustedProxyChecker(isTrustedProxyIP)
	}
	if r.hostedMode {
		// Use defaults: 2000 req/min per org.
		r.tenantRateLimiter = NewTenantRateLimiter(0, 0)
	}
	r.resourceRegistry = unifiedresources.NewRegistry(nil)
	r.monitorResourceAdapter = unifiedresources.NewMonitorAdapterWithStaleThresholds(
		r.resourceRegistry,
		monitoring.ResourceStaleThresholdsForConfig(cfg),
	)

	// Sync the configured admin user to the authorizer (if supported)
	if cfg.AuthUser != "" {
		auth.SetAdminUser(cfg.AuthUser)
	}

	// The tenant provider is the sole owner of v6 RBAC persistence. Initialize
	// the default manager before SSO services and routes so settings, SSO role
	// mapping, and authorization all observe the same SQLite store.
	r.rbacProvider = NewTenantRBACProvider(r.config.DataPath)
	defaultRBACManager, err := r.rbacProvider.GetManager("default")
	if err != nil {
		auth.SetManager(nil)
		log.Error().Err(err).Msg("Failed to initialize the canonical RBAC store")
	} else {
		auth.SetManager(defaultRBACManager)
		log.Info().Msg("Canonical RBAC store initialized")
	}

	// Initialize SSO service managers
	r.oidcManager = NewOIDCServiceManager()
	r.samlManager = NewSAMLServiceManager("")
	// Load persisted and environment-backed SSO before routes begin serving.
	// Configuration transfer and no-auth recovery must never observe the old
	// lazy-loading window as an unauthenticated installation.
	r.ensureSSOConfig()
	if err := r.syncSAMLPublicURL(); err != nil {
		log.Error().Err(err).Msg("Failed to initialize SAML public URL")
	}

	r.initializeBootstrapToken()

	r.setupRoutes()
	log.Debug().Msg("Routes registered successfully")

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
	devMode := utils.GetenvTrim("FRONTEND_DEV_SERVER") != ""
	handler := SecurityHeadersWithConfig(r, allowEmbedding, allowedOrigins, devMode)
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
	// Security: fail closed for non-default org requests when tenant monitor resolution fails.
	// Wrapped before TenantMiddleware so TenantMiddleware executes first and sets org context.
	handler = r.tenantMonitorGuardMiddleware(handler)
	handler = tenantMiddleware.Middleware(handler)

	// Auth context middleware extracts user/token info BEFORE tenant middleware
	handler = AuthContextMiddleware(cfg, r.mtMonitor, handler)

	handler = UniversalRateLimitMiddlewareWithConfig(newEndpointRateLimitConfig(), handler)
	r.wrapped = handler
	return r
}

func (r *Router) tenantMonitorGuardMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		orgID := strings.TrimSpace(GetOrgID(req.Context()))
		if orgID == "" || orgID == "default" {
			next.ServeHTTP(w, req)
			return
		}

		if r.mtMonitor == nil {
			writeErrorResponse(w, http.StatusServiceUnavailable, "tenant_unavailable", "Tenant monitor is not configured", nil)
			return
		}
		monitor, err := r.mtMonitor.GetMonitor(orgID)
		if err != nil || monitor == nil {
			writeErrorResponse(w, http.StatusServiceUnavailable, "tenant_unavailable", "Tenant monitor is not available", nil)
			return
		}

		next.ServeHTTP(w, req)
	})
}

// setupRoutes configures all routes
func (r *Router) setupRoutes() {
	// Create handlers
	r.alertHandlers = NewAlertHandlers(r.mtMonitor, NewAlertMonitorWrapper(r.monitor), r.wsHub)
	r.notificationHandlers = NewNotificationHandlers(r.mtMonitor, NewNotificationMonitorWrapper(r.monitor))
	r.notificationHandlers.SetReadState(r.defaultReadState())
	r.notificationQueueHandlers = NewNotificationQueueHandlers(r.monitor)
	guestMetadataHandler := NewGuestMetadataHandler(r.multiTenant)
	dockerMetadataHandler := NewDockerMetadataHandler(r.multiTenant)
	hostMetadataHandler := NewHostMetadataHandler(r.multiTenant)
	r.configHandlers = NewConfigHandlers(r.multiTenant, r.mtMonitor, r.reloadFunc, r.wsHub, guestMetadataHandler, r.reloadSystemSettings, r.hostedMode)
	if r.monitor != nil {
		r.configHandlers.SetMonitor(r.monitor)
		r.bindDefaultMetadataStores(r.monitor)
	}
	guestMetadataHandler.SetStoreResolver(func(ctx context.Context) *config.GuestMetadataStore {
		if monitor := r.configHandlers.Monitor(ctx); monitor != nil {
			return monitor.GuestMetadataStore()
		}
		return nil
	})
	dockerMetadataHandler.SetStoreResolver(func(ctx context.Context) *config.DockerMetadataStore {
		if monitor := r.configHandlers.Monitor(ctx); monitor != nil {
			return monitor.DockerMetadataStore()
		}
		return nil
	})
	hostMetadataHandler.SetStoreResolver(func(ctx context.Context) *config.HostMetadataStore {
		if monitor := r.configHandlers.Monitor(ctx); monitor != nil {
			return monitor.HostMetadataStore()
		}
		return nil
	})
	r.configHandlers.SetConfig(r.config)
	r.configHandlers.SetMockModeChangeHook(r.syncPlatformSupplementalProviders)
	r.trueNASHandlers = &TrueNASHandlers{
		getPersistence: r.configHandlers.Persistence,
		getConfig:      r.configHandlers.Config,
		getMonitor:     r.configHandlers.Monitor,
		getPoller:      func(context.Context) *monitoring.TrueNASPoller { return r.trueNASPoller },
	}
	r.vmwareHandlers = &VMwareHandlers{
		getPersistence: r.configHandlers.Persistence,
		getMonitor:     r.configHandlers.Monitor,
		getPoller:      func(context.Context) *monitoring.VMwarePoller { return r.vmwarePoller },
	}
	r.connectionsHandlers = NewConnectionsHandlers(
		r.configHandlers.Config,
		r.configHandlers.Persistence,
		r.configHandlers.Monitor,
	)
	r.connectionsHandlers.SetPlatformPollers(
		func(context.Context) *monitoring.TrueNASPoller { return r.trueNASPoller },
		func(context.Context) *monitoring.VMwarePoller { return r.vmwarePoller },
	)
	if r.monitor != nil {
		// Drive the connection-degraded alert off the same aggregator the
		// HTTP handler uses, so the active-notification stream stays in
		// lockstep with the Settings → Infrastructure badges. Single-tenant
		// only for now; multi-tenant per-org wiring is a follow-up.
		getCfg := r.configHandlers.Config
		getPersist := r.configHandlers.Persistence
		monitor := r.monitor
		trueNASPoller := r.trueNASPoller
		vmwarePoller := r.vmwarePoller
		r.monitor.SetConnectionsSnapshotLister(func() []alerts.ConnectionSnapshot {
			ctx := context.Background()
			return buildAlertConnectionSnapshotsWithRuntimeSources(ctx, getCfg(ctx), getPersist(ctx), monitor, aggregatorRuntimeSources{
				orgID:         "default",
				truenasPoller: trueNASPoller,
				vmwarePoller:  vmwarePoller,
			})
		})
	}
	r.availabilityHandlers = NewAvailabilityHandlers(
		r.configHandlers.Persistence,
		r.configHandlers.Monitor,
		// Resolved lazily: license handlers are constructed after this point.
		availabilityFeatureResolverFunc(func(ctx context.Context) licenseFeatureChecker {
			if r.licenseHandlers == nil {
				return nil
			}
			// Service() rather than FeatureService() so an absent tenant
			// service stays a nil interface instead of a typed nil.
			service := r.licenseHandlers.Service(ctx)
			if service == nil {
				return nil
			}
			return service
		}),
	)
	recoveryManager := recoverymanager.New(r.multiTenant)
	r.recoveryHandlers = NewRecoveryHandlers(recoveryManager)
	r.attentionHandlers = NewAttentionHandlers(r.configHandlers.Monitor, recoveryManager)
	if r.mtMonitor != nil {
		r.mtMonitor.SetRecoveryManager(recoveryManager)
	}
	if r.monitor != nil {
		r.monitor.SetRecoveryManager(recoveryManager)
	}
	r.trueNASPoller = monitoring.NewTrueNASPoller(r.multiTenant, 0, recoveryManager)
	r.trueNASPoller.Start(r.lifecycleCtx)
	r.vmwarePoller = monitoring.NewVMwarePoller(r.multiTenant, 0)
	r.vmwarePoller.Start(r.lifecycleCtx)
	updateHandlers := NewUpdateHandlersWithContext(r.updateManager, r.updateHistory, r.lifecycleCtx)
	updateHandlers.SetUpdateReadinessSources(
		r.updateReadinessConfigSnapshot,
		func(context.Context) []models.Host {
			if r.monitor == nil {
				return nil
			}
			return r.monitor.HostsSnapshot()
		},
	)
	r.dockerAgentHandlers = NewDockerAgentHandlers(r.mtMonitor, r.monitor, r.wsHub, r.config)
	r.kubernetesAgentHandlers = NewKubernetesAgentHandlers(r.mtMonitor, r.monitor, r.wsHub)
	r.unifiedAgentHandlers = NewUnifiedAgentHandlers(r.mtMonitor, r.monitor, r.wsHub)
	r.unifiedAgentHandlers.SetServerVersion(r.serverVersion)
	r.kubernetesAgentHandlers.SetRecoveryIngestor(r.recoveryHandlers)
	r.resourceHandlers = NewResourceHandlers(r.config)
	r.resourceHandlers.SetOperatorStateChanged(func(_ string, resourceID string) {
		seen := make(map[*monitoring.Monitor]struct{})
		reconcile := func(monitor *monitoring.Monitor) {
			if monitor == nil {
				return
			}
			if _, duplicate := seen[monitor]; duplicate {
				return
			}
			seen[monitor] = struct{}{}
			if reconciler, ok := any(monitor.GetAlertManager()).(interface {
				ReconcileOperatorIntentState() int
			}); ok {
				reconciler.ReconcileOperatorIntentState()
				monitor.SyncAlertState()
			} else if reconciler, ok := any(monitor.GetAlertManager()).(interface {
				ReconcileResourceOperatorState(string) int
			}); ok {
				reconciler.ReconcileResourceOperatorState(resourceID)
				monitor.SyncAlertState()
			}
		}

		if r.mtMonitor != nil {
			// The store mutation is already tenant scoped. Each live monitor's
			// resolver reads its own tenant store, so visiting every live monitor
			// cannot apply another tenant's policy. It does avoid selecting a newly
			// initialized, empty tenant monitor while alerts still live on the
			// legacy/default runtime during startup and development transitions.
			r.mtMonitor.ForEachMonitor(reconcile)
		}
		reconcile(r.monitor)
	})
	actionOrgChecker := NewAuthorizationChecker(NewMultiTenantOrganizationLoader(r.multiTenant))
	actionAuth := actionAuthority{authorizer: r.authorizer, orgChecker: actionOrgChecker}
	r.resourceHandlers.SetActionAuthorizers(actionAuth, actionAuth)
	r.attentionHandlers.SetActionDependencies(r.resourceHandlers, actionAuth)
	r.maintenanceSentinel = r.buildMaintenanceVerificationSentinel()
	r.maintenanceVerificationHandlers = NewMaintenanceVerificationHandlers(r.resourceHandlers, r.maintenanceSentinel)
	if r.maintenanceSentinel != nil {
		r.maintenanceSentinel.Start(r.lifecycleCtx)
	}
	r.agentContextHandler = NewAgentContextHandler(r.resourceHandlers)
	r.agentContextHandler.SetWorkflowPromptActivityProvider(agentWorkflowPromptActivityProviderFunc(func(ctx context.Context) (*config.WorkflowPromptActivityHistoryData, error) {
		persistence := r.persistenceForOrg(ctx)
		if persistence == nil {
			return nil, nil
		}
		return persistence.LoadWorkflowPromptActivityHistory()
	}))
	r.agentContextHandler.SetAIUsageProvider(agentAIUsageProviderFunc(func(ctx context.Context) (*config.AIUsageHistoryData, error) {
		persistence := r.persistenceForOrg(ctx)
		if persistence == nil {
			return nil, nil
		}
		return persistence.LoadAIUsageHistory()
	}))
	r.agentContextHandler.SetExternalAgentActivityProvider(agentExternalAgentActivityProviderFunc(func(ctx context.Context) (*config.ExternalAgentActivityHistoryData, error) {
		persistence := r.persistenceForOrg(ctx)
		if persistence == nil {
			return nil, nil
		}
		return persistence.LoadExternalAgentActivityHistory()
	}))
	r.agentContextHandler.SetExternalAgentReadinessProvider(agentExternalAgentReadinessProviderFunc(func(_ context.Context, manifest agentcapabilities.Manifest, now time.Time) bool {
		if r.config == nil {
			return false
		}
		config.Mu.Lock()
		tokens := make([]config.APITokenRecord, len(r.config.APITokens))
		copy(tokens, r.config.APITokens)
		config.Mu.Unlock()
		return agentOperationsLoopExternalAgentTokenReady(manifest, tokens, now)
	}))
	// Wire pending-approvals into the bundle. The provider resolves
	// the approval store at request time so multi-tenant rebuilds
	// (which install a new global store via approval.SetStore) stay
	// honored without a re-wire. Org scoping uses BelongsToOrg so
	// cross-tenant pending requests don't leak into an agent context
	// read for the wrong org.
	r.agentContextHandler.SetApprovalsProvider(agentApprovalStoreProvider{})
	r.agentEventBroadcaster = NewAgentEventBroadcaster()
	if r.resourceHandlers != nil && r.agentEventBroadcaster != nil {
		r.resourceHandlers.SetActionCompletedPublisher(r.agentEventBroadcaster.PublishActionCompletedRecord)
	}
	if r.resourceHandlers != nil {
		if store, err := r.resourceHandlers.getStore("default"); err == nil && store != nil {
			r.monitorResourceAdapter = unifiedresources.NewMonitorAdapterWithStaleThresholds(
				unifiedresources.NewRegistry(store),
				monitoring.ResourceStaleThresholdsForConfig(r.config),
			)
		}
	}
	r.syncPlatformSupplementalProviders(mock.IsMockEnabled())
	if r.monitor != nil {
		r.configureMonitorDependencies(r.monitor)
		if r.resourceHandlers != nil {
			r.resourceHandlers.SetStateProvider(r.monitor)
		}
	}
	if r.mtMonitor != nil && r.resourceHandlers != nil {
		r.resourceHandlers.SetTenantStateProvider(NewMultiTenantStateProvider(r.mtMonitor, r.monitor))
	}
	if r.mtMonitor != nil {
		r.mtMonitor.SetMonitorInitializer(r.configureMonitorDependencies)
	}
	r.configProfileHandler = NewConfigProfileHandler(r.multiTenant)
	r.licenseHandlers = NewLicenseHandlers(r.multiTenant, r.hostedMode, r.config)
	r.licenseHandlers.SetRuntimeVersion(r.serverVersion)
	r.licenseHandlers.SetMonitors(r.monitor, r.mtMonitor)
	// The compiled Pro binary self-updates from the license server download
	// broker (never the public community releases), so give the updater lazy
	// access to this installation's activation credentials. The source is
	// consulted per check/apply, picking up activation done after startup; the
	// community binary carries the source too but never consults it (the
	// updater keys off pkg/edition).
	r.updateManager.SetProUpdateCredentialSource(func() (updates.ProUpdateCredentials, bool) {
		svc := r.licenseHandlers.Service(context.Background())
		if svc == nil {
			return updates.ProUpdateCredentials{}, false
		}
		state := svc.GetActivationState()
		if state == nil || strings.TrimSpace(state.InstallationToken) == "" {
			return updates.ProUpdateCredentials{}, false
		}
		serverURL := strings.TrimSpace(state.LicenseServerURL)
		if serverURL == "" {
			serverURL = defaultLicenseServerURLValue
		}
		return updates.ProUpdateCredentials{
			LicenseServerURL:    serverURL,
			InstallationToken:   state.InstallationToken,
			InstanceFingerprint: state.InstanceFingerprint,
		}, true
	})
	rbacProvider := r.rbacProvider
	orgHandlers := NewOrgHandlers(r.multiTenant, r.mtMonitor, rbacProvider)
	orgHandlers.SetHostedMode(r.hostedMode)
	orgHandlers.SetOnDelete(func(ctx context.Context, orgID string) error {
		return r.CleanupTenant(ctx, orgID)
	})
	// Wire license service provider so middleware can access per-tenant license services
	SetLicenseServiceProvider(r.licenseHandlers)
	r.reportingHandlers = NewReportingHandlers(r.mtMonitor, recoveryManager)
	r.reportingHandlers.SetSystemSettingsStore(r.persistence)
	r.reportingHandlers.SetCommercialLicenseResolver(func(ctx context.Context) *licenseService {
		return r.licenseHandlers.Service(ctx)
	})
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
	r.stripeWebhookHandlers = NewStripeWebhookHandlers(
		config.NewFileBillingStore(r.config.DataPath),
		r.multiTenant,
		rbacProvider,
		magicLinkService,
		r.resolvePublicURL,
		r.hostedMode,
		r.config.DataPath,
	)
	stripeWebhookHandlers := r.stripeWebhookHandlers
	infraUpdateHandlers := NewUpdateDetectionHandlers(r.monitor, r.defaultReadState())
	auditHandlers := NewAuditHandlers()
	auditHandlers.SetResourceStoreProvider(r.resourceHandlers.getStore)

	// System settings and API token management
	r.systemSettingsHandler = NewSystemSettingsHandler(r.config, r.persistence, r.wsHub, r.mtMonitor, r.monitor, r.reloadSystemSettings, r.reloadFunc)
	// Toggling the Proxmox guest Docker inventory opt-in reconfigures the
	// monitor's checker and collector immediately instead of waiting for a
	// restart.
	r.systemSettingsHandler.SetGuestDockerInventoryToggleFunc(func() {
		if r.monitor != nil {
			r.configureProxmoxGuestDockerDetection(r.monitor)
		}
	})

	// Agent execution server for AI tool use
	r.agentExecServer = agentexec.NewServerWithAdmissionValidator(r.admitAgentExecToken, r.validateAgentExecSession)
	r.agentExecServer.SetCommandAuthorizationVerifier(verifyAndConsumeCommandAuthorization)
	if r.connectionsHandlers != nil {
		r.connectionsHandlers.SetAgentCommandSessionProvider(r.agentCommandSessionConnected)
	}
	if r.resourceHandlers != nil {
		r.resourceHandlers.SetActionExecutor(newRoutedActionExecutor(
			r.resourceHandlers,
			newDockerContainerActionExecutor(r.resourceHandlers, r.agentExecServer),
			newProxmoxGuestActionExecutor(r.resourceHandlers, r.agentExecServer, newProxmoxGuestMonitoringObserver(r.resolveMonitorForOrg)),
			newHostStorageCleanupActionExecutor(r.resourceHandlers, r.agentExecServer),
			newHostUpdateActionExecutor(r.resourceHandlers, r.agentExecServer),
		))
		// Receipt-pending dispatch attempts can only reconcile while the
		// owning agent is connected, so each agent (re)registration re-drives
		// the durable-dispatch recovery pass that startup begins.
		r.agentExecServer.SetAgentRegisteredNotifier(func(string) {
			r.recoverExecutingActions("system:agent-reconnect-recovery")
		})
	}
	if r.monitor != nil {
		r.configureProxmoxGuestDockerDetection(r.monitor)
	}

	// Deploy store and handlers for cluster agent deployment
	deployStore, deployErr := deploy.Open(filepath.Join(r.config.DataPath, "deploy.db"))
	if deployErr != nil {
		log.Error().Err(deployErr).Msg("Failed to open deploy store (cluster deployment disabled)")
	}
	if deployStore != nil {
		r.deployStore = deployStore
		if r.monitor != nil {
			r.deployHandlers = NewDeployHandlers(deployStore, r.monitor, r.agentExecServer, r.resolvePublicURL, r.config, r.persistence)
		}
	}

	// AI settings endpoints
	r.aiSettingsHandler = NewAISettingsHandler(r.multiTenant, r.mtMonitor, r.agentExecServer)
	if r.attentionHandlers != nil {
		r.attentionHandlers.SetActionLicenseChecker(func(ctx context.Context) bool {
			service := r.aiSettingsHandler.GetAIService(ctx)
			return service != nil && service.HasLicenseFeature(ai.FeatureAIAutoFix)
		})
	}
	if r.resourceHandlers != nil {
		r.resourceHandlers.SetActionEmergencyStopChecker(func(orgID string) (bool, error) {
			orgCtx := context.WithValue(context.Background(), OrgIDContextKey, approval.NormalizeOrgID(orgID))
			svc := r.aiSettingsHandler.GetAIService(orgCtx)
			if svc == nil || svc.GetConfig() == nil {
				return true, nil
			}
			return svc.GetConfig().PatrolActionEmergencyStop, nil
		})
		r.aiSettingsHandler.SetResourceStoreProvider(r.resourceHandlers.getStore)
		r.aiSettingsHandler.SetPolicyMutationCoordinator(r.resourceHandlers.ActionLifecycle().WithPolicyMutation)
		r.resourceHandlers.SetActionTransitionPublisher(r.aiSettingsHandler.ReconcilePatrolActionTransition)
		resourceHandlers := r.resourceHandlers
		patrolPolicyProvider := func(ctx context.Context, scopedOrgID string) (PatrolActionPolicySnapshot, error) {
			orgCtx := context.WithValue(ctx, OrgIDContextKey, approval.NormalizeOrgID(scopedOrgID))
			svc := r.aiSettingsHandler.GetAIService(orgCtx)
			if svc == nil {
				return PatrolActionPolicySnapshot{}, nil
			}
			cfg := svc.GetConfig()
			if cfg == nil {
				return PatrolActionPolicySnapshot{}, nil
			}
			effectiveAutonomyLevel := svc.GetEffectivePatrolAutonomyLevel()
			return PatrolActionPolicySnapshot{
				EffectiveAutonomyLevel: effectiveAutonomyLevel,
				FullModeUnlocked:       effectiveAutonomyLevel == config.PatrolAutonomyFull,
				EmergencyStop:          cfg.PatrolActionEmergencyStop,
			}, nil
		}
		resourceHandlers.SetActionRefreshPlanner(NewActionRefreshPlanner(resourceHandlers, patrolPolicyProvider))
		r.aiSettingsHandler.SetActionBrokerFactory(func(orgID string) aicontracts.OrchestratorActionBroker {
			return NewPatrolActionBroker(orgID, resourceHandlers, patrolPolicyProvider)
		})
		r.aiSettingsHandler.SetProposalCatalogFactory(func(orgID string) tools.ProposalCatalog {
			return func(ctx context.Context, resourceID string) ([]unifiedresources.ResourceCapability, error) {
				return resourceHandlers.ActionLifecycle().Capabilities(ctx, orgID, resourceID)
			}
		})
	}
	r.aiSettingsHandler.SetConfig(r.config)
	// Inject state provider so AI has access to full infrastructure context (VMs, containers, IPs)
	if r.monitor != nil {
		r.aiSettingsHandler.SetStateProvider(r.monitor)
		r.aiSettingsHandler.SetReadState(r.defaultReadState())
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
	if provider := r.defaultUnifiedResourceProvider(); provider != nil {
		r.aiSettingsHandler.SetUnifiedResourceProvider(provider)
	} else {
		log.Warn().Msg("[Router] unified resource provider is nil, cannot inject unified resource provider")
	}
	// Inject tenant-scoped metadata providers for AI URL discovery. Existing
	// tenant services are refreshed when Router.SetMonitor replaces a monitor.
	r.configureMetadataProviderFactory()

	// Wire the per-tenant AI narrator, fleet narrator, and Patrol
	// findings provider into reporting. The AI service implements all
	// three interfaces; when not configured for the tenant the engine
	// falls back to the heuristic narrators with no findings section.
	if r.reportingHandlers != nil {
		settings := r.aiSettingsHandler
		r.reportingHandlers.SetPatrolDigestResolver(func(ctx context.Context, days int) (ai.PatrolDigest, bool) {
			if settings == nil {
				return ai.PatrolDigest{}, false
			}
			return settings.BuildPatrolDigest(ctx, days)
		})
		r.reportingHandlers.SetNarratorResolver(func(ctx context.Context) (reporting.Narrator, reporting.FleetNarrator, reporting.FindingsProvider) {
			if settings == nil {
				return nil, nil, nil
			}
			svc := settings.GetAIService(ctx)
			if svc == nil {
				return nil, nil, nil
			}
			cfg := svc.GetAIConfig()
			if cfg == nil || !cfg.Enabled {
				return nil, nil, nil
			}
			return svc, svc, svc
		})
	}

	// AI chat handler
	r.aiHandler = NewAIHandler(r.multiTenant, r.mtMonitor, r.agentExecServer)
	r.aiHandler.SetReadState(r.defaultReadState())
	r.aiHandler.SetRecoveryManager(recoveryManager)
	// Bridge approval creation into the agent SSE stream so an agent
	// holding /api/agent/events open hears about new pending approvals
	// in real time. The callback is fire-and-forget on the approval
	// hot path; PublishApprovalPending drops events for slow
	// subscribers rather than blocking.
	if r.agentEventBroadcaster != nil {
		broadcaster := r.agentEventBroadcaster
		r.aiHandler.SetApprovalCreatedCallback(func(req *approval.ApprovalRequest) {
			if req == nil {
				return
			}
			broadcaster.PublishApprovalPending(AgentEventApprovalPendingPayload{
				ApprovalID:  req.ID,
				ResourceID:  req.CanonicalResourceID(),
				TargetType:  req.TargetType,
				TargetID:    req.TargetID,
				TargetName:  req.TargetName,
				Command:     req.Command,
				RiskLevel:   string(req.RiskLevel),
				RequestedBy: req.RequestedBy,
				RequestedAt: req.RequestedAt,
				ExpiresAt:   req.ExpiresAt,
			})
		})
	}
	r.aiHandler.SetControlLevelResolver(func(ctx context.Context, cfg *config.AIConfig) string {
		return r.aiSettingsHandler.EffectiveControlLevel(ctx, cfg)
	})
	r.aiHandler.SetServiceInitializer(func(ctx context.Context, service AIService) {
		r.wireAIChatDependenciesForService(ctx, service)
	})
	// Wire the per-tenant report-narrator resolver into the chat
	// handler so the pulse_summarize tool can produce AI-narrated
	// synthesis using the same Service that powers the report PDF
	// endpoint. The AI service implements all three interfaces; an
	// unconfigured tenant returns nil and the tool falls back to
	// heuristic narrative.
	r.aiHandler.SetReportNarratorResolver(func(ctx context.Context) (reporting.Narrator, reporting.FleetNarrator, reporting.FindingsProvider) {
		if r.aiSettingsHandler == nil {
			return nil, nil, nil
		}
		svc := r.aiSettingsHandler.GetAIService(ctx)
		if svc == nil {
			return nil, nil, nil
		}
		cfg := svc.GetAIConfig()
		if cfg == nil || !cfg.Enabled {
			return nil, nil, nil
		}
		return svc, svc, svc
	})
	// Wire the per-tenant cost store into chat sessions so user-chat
	// token usage flows into the same operator-facing dashboard the
	// rest of the AI runtime uses (patrol, discovery,
	// report-narrative). No Enabled gate — even when chat ran briefly
	// while AI was being configured, those tokens were billed and
	// should be visible.
	r.aiHandler.SetCostStoreResolver(func(ctx context.Context) *cost.Store {
		if r.aiSettingsHandler == nil {
			return nil
		}
		svc := r.aiSettingsHandler.GetAIService(ctx)
		if svc == nil {
			return nil
		}
		return svc.CostStore()
	})
	// Mobile polls approvals as part of its normal paired runtime. Keep the
	// approval store available even when AI chat is disabled or not configured.
	r.aiHandler.ensureApprovalStore(r.config.DataPath)

	// AI-powered infrastructure discovery handlers
	// Note: The actual service is wired up later via SetDiscoveryService
	r.discoveryHandlers = NewDiscoveryHandlers(nil, r.config)
	if r.resourceHandlers != nil {
		r.resourceHandlers.SetDiscoveryReadinessProvider(r.discoveryHandlers)
	}

	// Wire license checker for Pro feature gating (AI Patrol, Alert Analysis, Auto-Fix)
	r.aiSettingsHandler.SetLicenseHandlers(r.licenseHandlers)
	// Wire model change callback to restart AI chat service when model is changed
	r.aiSettingsHandler.SetOnModelChange(func() {
		r.RestartAIChat(context.Background())
	})
	// Wire control settings change callback to update Assistant tool visibility.
	r.aiSettingsHandler.SetOnControlSettingsChange(func() {
		if r.aiHandler != nil {
			ctx := context.Background()
			if svc := r.aiHandler.GetService(ctx); svc != nil {
				cfg := r.aiHandler.GetAIConfig(ctx)
				if cfg != nil {
					svc.UpdateControlSettings(cfg)
					log.Info().
						Str("control_level", r.aiSettingsHandler.EffectiveControlLevel(ctx, cfg)).
						Msg("Updated AI control settings")
				}
			}
		}
	})
	// Wire AI handler to profile handler for AI-assisted suggestions
	r.configProfileHandler.SetAIHandler(r.aiHandler)
	r.aiHandler.SetPatrolRunHandoffProvider(func(ctx context.Context, runID string) (ai.PatrolRunRecord, bool) {
		if r.aiSettingsHandler == nil {
			return ai.PatrolRunRecord{}, false
		}
		aiService := r.aiSettingsHandler.GetAIService(ctx)
		if aiService == nil {
			return ai.PatrolRunRecord{}, false
		}
		patrol := aiService.GetPatrolService()
		if patrol == nil {
			return ai.PatrolRunRecord{}, false
		}
		run, ok := patrol.GetRunByID(runID)
		if !ok {
			return ai.PatrolRunRecord{}, false
		}
		run.ToolCalls = nil
		return run, true
	})
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
		// Wire license checker for monitoring Pro features (external probes).
		// The checker is consulted per read, so an entitlement lapse converges
		// on the next polling cycle without a restart.
		monitorLicSvc := r.licenseHandlers.Service(context.Background())
		r.monitor.SetLicenseChecker(func(feature string) bool {
			if monitorLicSvc == nil {
				return false
			}
			return monitorLicSvc.HasFeature(feature)
		})
		// A probe that stops reporting is a first-class availability incident.
		// Normal notification routes are handled by the monitor; Relay adds a
		// privacy-safe mobile push while the Pulse instance is still online.
		// If the whole instance is offline, Relay's independent disconnect
		// timer supplies the complementary dark-site notification.
		r.monitor.SetAlertPushCallback(func(alert *alerts.Alert) {
			if alert == nil {
				return
			}
			r.relayMu.RLock()
			client := r.relayClient
			minimumSeverity := r.relayAlertMinimumSeverity
			r.relayMu.RUnlock()
			if client == nil {
				return
			}
			if !relay.AlertMeetsMinimumSeverity(string(alert.Level), minimumSeverity) {
				log.Debug().Str("alertID", alert.ID).Str("level", string(alert.Level)).Msg("Alert excluded from mobile push by destination severity policy")
				return
			}
			notification := canonicalAlertPushNotification(
				alert,
				r.monitor.HasExternalProbeAssignments,
			)
			if err := client.SendPushNotification(notification); err != nil {
				log.Warn().
					Err(err).
					Str("alertID", notification.ActionID).
					Str("type", notification.Type).
					Msg("Alert push notification send failed")
			}
		})
	}

	// Initialize recovery token store and capture the exact worker this router owns.
	r.recoveryTokenStore = ensureRecoveryTokenStore(r.config.DataPath)

	r.registerPublicAndAuthRoutes()
	r.registerMonitoringRoutes(guestMetadataHandler, dockerMetadataHandler, hostMetadataHandler, infraUpdateHandlers)
	r.registerConfigSystemRoutes(updateHandlers)
	r.registerAIRelayRoutes()
	r.registerOrgLicenseRoutes(orgHandlers, rbacHandlers, auditHandlers)
	r.registerHostedRoutes(hostedSignupHandlers, magicLinkHandlers, stripeWebhookHandlers)

	// Debug profiling endpoints (admin-only, can be disabled via PULSE_PPROF_DISABLED=true)
	if pprofEnabled() {
		r.registerDebugRoutes()
	}

	// Note: Frontend handler is handled manually in ServeHTTP to prevent redirect issues
	// See issue #334 - ServeMux redirects empty path to "./" which breaks reverse proxies
}

// CleanupTenant removes all per-tenant resources (RBAC, AI, License) for a deleted org.
func (r *Router) CleanupTenant(ctx context.Context, orgID string) error {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" || orgID == "default" {
		return nil
	}

	var errs []error

	r.StopPatrolForOrg(orgID)

	if r.aiSettingsHandler != nil {
		r.aiSettingsHandler.RemoveTenantService(orgID)
	}

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
	if r.resourceHandlers != nil {
		if err := r.resourceHandlers.CloseTenantStore(orgID); err != nil {
			errs = append(errs, fmt.Errorf("resource store cleanup: %w", err))
		}
	}

	r.monitorAdapterMu.Lock()
	delete(r.monitorResourceAdapters, orgID)
	r.monitorAdapterMu.Unlock()

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
	// Extract session ID from path: /api/ai/sessions/{id}[/messages|/abort|/summarize|/diff|/fork|/undo|/redo|/revert|/unrevert]
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
			if !ensureRelayMobileRuntimeRoute(w, req, relayMobileRouteSessionMessages) {
				return
			}
			r.aiHandler.HandleMessages(w, req, sessionID)
		case "abort":
			if !ensureRelayMobileRuntimeRoute(w, req, relayMobileRouteSessionAbort) {
				return
			}
			r.aiHandler.HandleAbort(w, req, sessionID)
		case "summarize":
			if !ensureScope(w, req, config.ScopeAIChat) {
				return
			}
			r.aiHandler.HandleSummarize(w, req, sessionID)
		case "diff":
			if !ensureScope(w, req, config.ScopeAIChat) {
				return
			}
			r.aiHandler.HandleDiff(w, req, sessionID)
		case "fork":
			if !ensureScope(w, req, config.ScopeAIChat) {
				return
			}
			r.aiHandler.HandleFork(w, req, sessionID)
		case "undo":
			if !ensureScope(w, req, config.ScopeAIChat) {
				return
			}
			r.aiHandler.HandleUndoLastTurn(w, req, sessionID)
		case "redo":
			if !ensureScope(w, req, config.ScopeAIChat) {
				return
			}
			r.aiHandler.HandleRedoLastTurn(w, req, sessionID)
		case "steer":
			if !ensureScope(w, req, config.ScopeAIChat) {
				return
			}
			r.aiHandler.HandleSteerSession(w, req, sessionID)
		case "revert":
			if !ensureScope(w, req, config.ScopeAIChat) {
				return
			}
			r.aiHandler.HandleRevert(w, req, sessionID)
		case "unrevert":
			if !ensureScope(w, req, config.ScopeAIChat) {
				return
			}
			r.aiHandler.HandleUnrevert(w, req, sessionID)
		default:
			if !ensureScope(w, req, config.ScopeAIChat) {
				return
			}
			http.Error(w, "Not found", http.StatusNotFound)
		}
		return
	}

	// Handle session-level operations
	switch req.Method {
	case http.MethodPatch:
		if !ensureRelayMobileRuntimeRoute(w, req, relayMobileRouteSessionRename) {
			return
		}
		r.aiHandler.HandleRenameSession(w, req, sessionID)
	case http.MethodDelete:
		if !ensureRelayMobileRuntimeRoute(w, req, relayMobileRouteSessionDelete) {
			return
		}
		r.aiHandler.HandleDeleteSession(w, req, sessionID)
	default:
		if !ensureScope(w, req, config.ScopeAIChat) {
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) routeAISessionsCollection(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		if !ensureRelayMobileRuntimeRoute(w, req, relayMobileRouteSessionsList) {
			return
		}
		r.aiHandler.HandleSessions(w, req)
	case http.MethodPost:
		if !ensureRelayMobileRuntimeRoute(w, req, relayMobileRouteSessionCreate) {
			return
		}
		r.aiHandler.HandleCreateSession(w, req)
	default:
		if !ensureScope(w, req, config.ScopeAIChat) {
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) routeAIPatrolFindings(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		if !ensureRelayMobileRuntimeRoute(w, req, relayMobileRoutePatrolFindingsList) {
			return
		}
		r.recordExternalAgentCapabilityActivity(req, agentcapabilities.ListFindingsCapabilityName)
		r.aiSettingsHandler.HandleGetPatrolFindings(w, req)
	case http.MethodDelete:
		if !ensureScope(w, req, config.ScopeAIExecute) {
			return
		}
		// Clear all findings - doesn't require Pro license so users can clean up accumulated findings
		r.aiSettingsHandler.HandleClearAllFindings(w, req)
	default:
		if !ensureScope(w, req, config.ScopeAIExecute) {
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) routeAIFindings(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	switch {
	case strings.HasSuffix(path, "/investigation/messages"):
		if !ensureRelayMobileRuntimeRoute(w, req, relayMobileRouteFindingInvestigationMessages) {
			return
		}
		r.aiSettingsHandler.HandleGetInvestigationMessages(w, req)
	case strings.HasSuffix(path, "/investigation"):
		if !ensureRelayMobileRuntimeRoute(w, req, relayMobileRouteFindingInvestigation) {
			return
		}
		r.aiSettingsHandler.HandleGetInvestigation(w, req)
	case strings.HasSuffix(path, "/reinvestigate"):
		if !ensureScope(w, req, config.ScopeAIExecute) {
			return
		}
		r.aiAutoFixEndpoints.HandleReinvestigateFinding(w, req)
	case strings.HasSuffix(path, "/reapprove"):
		if !ensureScope(w, req, config.ScopeAIExecute) {
			return
		}
		r.aiAutoFixEndpoints.HandleReapproveInvestigationFix(w, req)
	default:
		if !ensureScope(w, req, config.ScopeAIExecute) {
			return
		}
		http.Error(w, "Not found", http.StatusNotFound)
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
			if !ensureRelayMobileRuntimeRoute(w, req, relayMobileRouteApprovalApprove) {
				return
			}
			r.aiSettingsHandler.HandleApproveCommand(w, req)
		case "deny":
			if !ensureRelayMobileRuntimeRoute(w, req, relayMobileRouteApprovalDeny) {
				return
			}
			r.aiSettingsHandler.HandleDenyCommand(w, req)
		default:
			if !ensureScope(w, req, config.ScopeAIExecute) {
				return
			}
			http.Error(w, "Not found", http.StatusNotFound)
		}
		return
	}

	// Handle approval-level operations (GET specific approval)
	switch req.Method {
	case http.MethodGet:
		if !ensureScope(w, req, config.ScopeAIExecute) {
			return
		}
		r.aiSettingsHandler.HandleGetApproval(w, req)
	default:
		if !ensureScope(w, req, config.ScopeAIExecute) {
			return
		}
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

// serveSetupTokenOrSettingsWrite gates a privileged settings endpoint behind
// either a valid setup token (setup scripts) or full authentication plus
// proxy-auth admin and settings:write scope. logLabel tags the unauthorized
// log line; nonAdminMsg is the proxy-auth denial log message.
func (r *Router) serveSetupTokenOrSettingsWrite(w http.ResponseWriter, req *http.Request, logLabel, nonAdminMsg string, delegate http.HandlerFunc) {
	// Check setup token first (for setup scripts)
	if r.isValidSetupTokenForRequest(req) {
		delegate(w, req)
		return
	}

	// Require authentication
	authWriter := &responseCapture{ResponseWriter: w}
	if !checkAuth(r.config, authWriter, req, false) {
		log.Warn().
			Str("ip", req.RemoteAddr).
			Str("path", req.URL.Path).
			Str("method", req.Method).
			Msg("Unauthorized access attempt (" + logLabel + ")")

		if !authWriter.wrote {
			writeAuthenticationRequired(w, req)
		}
		return
	}

	// Check admin privileges for proxy auth users
	if r.config.ProxyAuthSecret != "" {
		if valid, username, isAdmin := CheckProxyAuth(r.config, req); valid && !isAdmin {
			log.Warn().
				Str("ip", GetClientIP(req)).
				Str("username", username).
				Msg(nonAdminMsg)
			http.Error(w, "Admin privileges required", http.StatusForbidden)
			return
		}
	}

	// Require admin session identity or settings:write token for privileged writes.
	if !ensureSettingsWriteScope(r.config, w, req) {
		return
	}

	delegate(w, req)
}

func (r *Router) handleVerifyTemperatureSSH(w http.ResponseWriter, req *http.Request) {
	if r.configHandlers == nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	r.serveSetupTokenOrSettingsWrite(w, req, "verify-temperature-ssh", "Non-admin user attempted verify-temperature-ssh", r.configHandlers.HandleVerifyTemperatureSSH)
}

// handleSSHConfig handles SSH config writes with setup token or API auth
func (r *Router) handleSSHConfig(w http.ResponseWriter, req *http.Request) {
	if r.systemSettingsHandler == nil {
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	r.serveSetupTokenOrSettingsWrite(w, req, "ssh-config", "Non-admin user attempted ssh-config update", r.systemSettingsHandler.HandleSSHConfig)
}

func extractSetupToken(req *http.Request) string {
	if token := strings.TrimSpace(req.Header.Get("X-Setup-Token")); token != "" {
		return token
	}
	if token := extractBearerToken(req.Header.Get("Authorization")); token != "" {
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

// StartBackgroundWorkers starts router-owned background workers for the API
// server lifecycle. Unit tests that only need the HTTP handler can construct a
// router without starting update checks or WebSocket forwarding loops.
func (r *Router) StartBackgroundWorkers() {
	if r == nil {
		return
	}
	r.backgroundWorkersOnce.Do(func() {
		r.startLifecycleWorker(r.forwardUpdateProgress)
		r.startLifecycleWorker(func() {
			r.backgroundUpdateChecker(r.lifecycleCtx)
		})
		r.startLifecycleWorker(func() {
			r.recoverExecutingActions("system:restart-recovery")
		})
		r.startLifecycleWorker(func() {
			r.runPeriodicExecutingActionRecovery(r.lifecycleCtx)
		})
		if r.reportingHandlers != nil {
			r.startLifecycleWorker(func() {
				r.reportingHandlers.RunReportScheduleScheduler(r.lifecycleCtx)
			})
		}
	})
}

// maxExecutingActionRecoveryBatch bounds one durable-dispatch recovery pass
// per org. Recovery re-runs on every agent (re)registration, so a bounded
// pass still converges when more actions are stuck than one batch covers.
const maxExecutingActionRecoveryBatch = 100

// executingActionRecoveryInterval paces the standing recovery loop. Restart
// and agent-reconnect passes cover their own triggers, but an abandoned
// dispatch (client disconnect, transport timeout, send failure) otherwise
// waits for the owning agent to reconnect — which may never happen while the
// agent stays healthily connected. The periodic pass reconciles those from
// the durable agent receipt within one interval. A pass over zero executing
// actions is one indexed query per org.
const executingActionRecoveryInterval = 2 * time.Minute

// runPeriodicExecutingActionRecovery re-drives the durable-dispatch recovery
// pass on a fixed interval until the router lifecycle context ends.
func (r *Router) runPeriodicExecutingActionRecovery(ctx context.Context) {
	if r == nil || r.resourceHandlers == nil || mock.IsMockEnabled() {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ticker := time.NewTicker(executingActionRecoveryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.recoverExecutingActions("system:periodic-recovery")
		}
	}
}

// recoverExecutingActions reconciles actions left in the executing state by a
// server restart or a lost agent callback. Every org routes through
// Service.RecoverExecutingActions: receipt-pending attempts reconcile
// query-only against the durable agent operation receipt, a committed-but-
// never-sent dispatch may be re-driven once, and nothing is blindly
// re-executed. It runs at startup and again whenever an agent (re)registers,
// because receipt reconciliation can only answer while the owning agent is
// connected.
func (r *Router) recoverExecutingActions(actor string) {
	if r == nil || r.resourceHandlers == nil || mock.IsMockEnabled() {
		return
	}
	// Serialize passes: an agent reconnect storm after a restart would
	// otherwise run redundant overlapping passes over the same records.
	r.actionRecoveryMu.Lock()
	defer r.actionRecoveryMu.Unlock()
	ctx := r.lifecycleCtx
	if ctx == nil {
		ctx = context.Background()
	}
	orgIDs := []string{"default"}
	if r.multiTenant != nil {
		if orgs, err := r.multiTenant.ListOrganizations(); err != nil {
			log.Warn().Err(err).Msg("Executing-action recovery could not list organizations; recovering default org only")
		} else {
			orgIDs = orgIDs[:0]
			for _, org := range orgs {
				if org != nil {
					orgIDs = append(orgIDs, org.ID)
				}
			}
		}
	}
	lifecycle := r.resourceHandlers.ActionLifecycle()
	for _, orgID := range orgIDs {
		if ctx.Err() != nil {
			return
		}
		recovered, err := lifecycle.RecoverExecutingActions(ctx, orgID, actor, maxExecutingActionRecoveryBatch)
		if err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Str("actor", actor).Msg("Executing-action recovery pass failed")
			continue
		}
		terminal := 0
		for _, record := range recovered {
			if record.State == unifiedresources.ActionStateCompleted || record.State == unifiedresources.ActionStateFailed {
				terminal++
			}
		}
		if terminal > 0 {
			log.Info().Int("recovered", terminal).Int("examined", len(recovered)).Str("org_id", orgID).Str("actor", actor).Msg("Recovered executing actions from durable dispatch state")
		}
	}
}

func (r *Router) startLifecycleWorker(worker func()) {
	if r == nil || worker == nil {
		return
	}
	r.lifecycleWG.Add(1)
	go func() {
		defer r.lifecycleWG.Done()
		worker()
	}()
}

// SetMonitor updates the router and associated handlers with a new monitor instance.
func (r *Router) SetMonitor(m *monitoring.Monitor) {
	r.monitor = m
	r.bindDefaultMetadataStores(m)
	r.configureMetadataProviderFactory()
	if r.alertHandlers != nil {
		r.alertHandlers.SetMonitor(NewAlertMonitorWrapper(m))
	}
	if r.configHandlers != nil {
		r.configHandlers.SetMonitor(m)
		r.configHandlers.SetConfig(r.config)
	}
	if r.notificationHandlers != nil {
		r.notificationHandlers.SetMonitor(NewNotificationMonitorWrapper(m))
		r.notificationHandlers.SetReadState(r.defaultReadState())
	}
	if r.dockerAgentHandlers != nil {
		r.dockerAgentHandlers.SetMonitor(m)
	}
	if r.unifiedAgentHandlers != nil {
		r.unifiedAgentHandlers.SetMonitor(m)
	}
	if r.kubernetesAgentHandlers != nil {
		r.kubernetesAgentHandlers.SetMonitor(m)
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
		r.configureMonitorDependencies(m)
		if r.resourceHandlers != nil {
			r.resourceHandlers.SetStateProvider(m)
		}

		// Set state provider on AI handler so patrol service gets created
		// (Critical: patrol service is created lazily in SetStateProvider)
		if r.aiSettingsHandler != nil {
			r.aiSettingsHandler.SetStateProvider(m)
			r.aiSettingsHandler.SetReadState(r.defaultReadState())
			r.aiSettingsHandler.SetUnifiedResourceProvider(r.defaultUnifiedResourceProvider())
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
		if r.aiHandler != nil {
			r.aiHandler.SetReadState(r.defaultReadState())
		}

		r.configureProxmoxGuestDockerDetection(m)
	}
}

func (r *Router) bindDefaultMetadataStores(monitor *monitoring.Monitor) {
	if r == nil || monitor == nil {
		return
	}
	if r.persistence != nil {
		r.persistence.SetMetadataStores(
			monitor.GuestMetadataStore(),
			monitor.DockerMetadataStore(),
			monitor.HostMetadataStore(),
		)
	}
	if r.multiTenant != nil {
		if persistence, err := r.multiTenant.GetPersistence("default"); err == nil {
			persistence.SetMetadataStores(
				monitor.GuestMetadataStore(),
				monitor.DockerMetadataStore(),
				monitor.HostMetadataStore(),
			)
		}
	}
}

func (r *Router) configureMetadataProviderFactory() {
	if r == nil || r.aiSettingsHandler == nil {
		return
	}
	r.aiSettingsHandler.SetMetadataProviderFactory(func(orgID string) ai.MetadataProvider {
		orgID = strings.TrimSpace(orgID)
		if orgID == "" {
			orgID = "default"
		}

		var monitor *monitoring.Monitor
		if orgID == "default" {
			monitor = r.monitor
		} else if r.mtMonitor != nil {
			monitor, _ = r.mtMonitor.GetMonitor(orgID)
		}
		if monitor != nil {
			return NewMetadataProvider(
				monitor.GuestMetadataStore(),
				monitor.DockerMetadataStore(),
				monitor.HostMetadataStore(),
			)
		}

		if r.multiTenant != nil {
			if persistence, err := r.multiTenant.GetPersistence(orgID); err == nil && persistence != nil {
				return NewMetadataProvider(
					persistence.GetGuestMetadataStore(),
					persistence.GetDockerMetadataStore(),
					persistence.GetHostMetadataStore(),
				)
			}
		}
		if orgID == "default" && r.persistence != nil {
			return NewMetadataProvider(
				r.persistence.GetGuestMetadataStore(),
				r.persistence.GetDockerMetadataStore(),
				r.persistence.GetHostMetadataStore(),
			)
		}
		return nil
	})
}

func (r *Router) configureProxmoxGuestDockerDetection(m *monitoring.Monitor) {
	if r == nil || m == nil {
		return
	}
	inventoryEnabled := r.config != nil && r.config.EnableProxmoxGuestDockerInventory
	detectionEnabled := r.config != nil && (r.config.EnableProxmoxGuestDockerDetection || inventoryEnabled)
	if !detectionEnabled {
		m.SetDockerChecker(nil)
		m.SetDockerCheckerAllowedVMIDs(nil)
		m.SetDockerInventoryCollector(nil)
		return
	}
	if r.agentExecServer == nil {
		m.SetDockerChecker(nil)
		m.SetDockerCheckerAllowedVMIDs(nil)
		m.SetDockerInventoryCollector(nil)
		return
	}

	execFunc := func(ctx context.Context, hostname string, command string, timeout int) (string, int, error) {
		agentID, found := r.agentExecServer.GetAgentForHost(hostname)
		if !found {
			return "", -1, fmt.Errorf("no agent connected for host %s", hostname)
		}
		result, err := r.agentExecServer.ExecuteCommand(ctx, agentID, agentexec.ExecuteCommandPayload{
			RequestID: fmt.Sprintf("docker-check-%d", time.Now().UnixNano()),
			Command:   command,
			Timeout:   timeout,
			Trusted:   true,
		})
		if err != nil {
			return "", -1, err
		}
		return result.Stdout + result.Stderr, result.ExitCode, nil
	}

	allowedVMIDs, invalidVMIDs := monitoring.ParseProxmoxGuestDockerInventoryVMIDs(r.config.ProxmoxGuestDockerInventoryVMIDs)
	if len(invalidVMIDs) > 0 {
		log.Warn().
			Strs("invalidVmids", invalidVMIDs).
			Msg("[Router] Ignoring invalid Proxmox guest Docker inventory VMID allowlist entries")
	}

	checker := monitoring.NewAgentDockerChecker(execFunc)
	m.SetDockerChecker(checker)
	// The allowlist gates the socket probe as well as inventory collection:
	// opting into specific guests must not leave every other guest probed via
	// pct exec on each poll cycle.
	m.SetDockerCheckerAllowedVMIDs(allowedVMIDs)
	if inventoryEnabled {
		m.SetDockerInventoryCollector(monitoring.NewAgentDockerInventoryCollector(execFunc, monitoring.AgentDockerInventoryCollectorOptions{
			AllowedVMIDs: allowedVMIDs,
		}))
		log.Info().
			Int("allowedVmids", len(allowedVMIDs)).
			Msg("[Router] Proxmox guest Docker collector configured for explicit LXC inventory")
	} else {
		m.SetDockerInventoryCollector(nil)
		log.Info().Msg("[Router] Proxmox guest Docker detector configured for explicit LXC socket hinting")
	}
}

func (r *Router) defaultReadState() unifiedresources.ReadState {
	if r == nil {
		return nil
	}

	if r.monitor != nil {
		if readState := r.monitor.GetUnifiedReadState(); readState != nil {
			return readState
		}
	}
	if r.monitorResourceAdapter != nil {
		return r.monitorResourceAdapter
	}
	return r.resourceRegistry
}

func (r *Router) unifiedResourceProviderForMonitor(m *monitoring.Monitor) ai.UnifiedResourceProvider {
	if r == nil {
		return nil
	}

	if m != nil {
		if readState := m.GetUnifiedReadState(); readState != nil {
			if provider, ok := readState.(ai.UnifiedResourceProvider); ok && provider != nil {
				return provider
			}
		}
	}

	if adapter := r.monitorAdapterForMonitor(m); adapter != nil {
		return adapter
	}
	if r.monitorResourceAdapter != nil {
		return r.monitorResourceAdapter
	}
	return nil
}

func (r *Router) defaultUnifiedResourceProvider() ai.UnifiedResourceProvider {
	if r == nil {
		return nil
	}

	if r.monitor != nil {
		if provider := r.unifiedResourceProviderForMonitor(r.monitor); provider != nil {
			return provider
		}
	}
	return r.unifiedResourceProviderForMonitor(nil)
}

func (r *Router) monitorAdapterForMonitor(m *monitoring.Monitor) *unifiedresources.MonitorAdapter {
	if r == nil || m == nil {
		return nil
	}

	orgID := strings.TrimSpace(m.GetOrgID())
	if orgID == "" || orgID == "default" {
		return r.monitorResourceAdapter
	}

	r.monitorAdapterMu.Lock()
	defer r.monitorAdapterMu.Unlock()

	if r.monitorResourceAdapters == nil {
		r.monitorResourceAdapters = make(map[string]*unifiedresources.MonitorAdapter)
	}
	if existing := r.monitorResourceAdapters[orgID]; existing != nil {
		return existing
	}

	store := unifiedresources.ResourceStore(nil)
	if r.resourceHandlers != nil {
		if resolved, err := r.resourceHandlers.getStore(orgID); err == nil {
			store = resolved
		}
	}
	adapter := unifiedresources.NewMonitorAdapterWithStaleThresholds(
		unifiedresources.NewRegistry(store),
		monitoring.ResourceStaleThresholdsForConfig(m.GetConfig()),
	)
	r.monitorResourceAdapters[orgID] = adapter
	return adapter
}

func (r *Router) configureMonitorDependencies(m *monitoring.Monitor) {
	if r == nil || m == nil {
		return
	}

	if adapter := r.monitorAdapterForMonitor(m); adapter != nil {
		log.Debug().Msg("[Router] Injecting unified resource adapter into monitor")
		m.SetResourceStore(adapter)
	}

	// Tenant monitors must inherit the persisted instance-wide notification
	// settings. System settings are stored globally, so without this a tenant
	// monitor created after the allowlist was saved (or after a restart)
	// silently falls back to deny-all-private and per-client private webhook
	// targets (e.g. MSP Gotify endpoints reached over VPN) fail SSRF
	// validation with no org-side way to allow them.
	if r.persistence != nil {
		if settings, err := r.persistence.LoadSystemSettings(); err == nil && settings != nil {
			if nm := m.GetNotificationManager(); nm != nil {
				if settings.WebhookAllowedPrivateCIDRs != "" {
					if err := nm.UpdateAllowedPrivateCIDRs(settings.WebhookAllowedPrivateCIDRs); err != nil {
						log.Error().Err(err).Msg("Failed to apply webhook allowed private CIDRs to tenant monitor")
					}
				}
				if settings.PublicURL != "" {
					nm.SetPublicURL(settings.PublicURL)
				}
			}
		} else if err != nil {
			log.Warn().Err(err).Str("org", m.GetOrgID()).Msg("Failed to load system settings for tenant monitor; webhook private CIDR allowlist not applied")
		}
	}

	if len(r.monitorSupplementalRecords) == 0 {
		return
	}

	keys := make([]string, 0, len(r.monitorSupplementalRecords))
	for source := range r.monitorSupplementalRecords {
		keys = append(keys, string(source))
	}
	sort.Strings(keys)

	for _, key := range keys {
		source := unifiedresources.DataSource(key)
		provider := r.monitorSupplementalRecords[source]
		m.SetSupplementalRecordsProvider(source, provider)
	}
}

func (r *Router) setMonitorSupplementalRecordsProvider(source unifiedresources.DataSource, provider monitoring.MonitorSupplementalRecordsProvider) {
	if r == nil {
		return
	}

	normalized := unifiedresources.DataSource(strings.ToLower(strings.TrimSpace(string(source))))
	if normalized == "" {
		return
	}

	if r.monitorSupplementalRecords == nil {
		r.monitorSupplementalRecords = make(map[unifiedresources.DataSource]monitoring.MonitorSupplementalRecordsProvider)
	}
	if provider == nil {
		delete(r.monitorSupplementalRecords, normalized)
	} else {
		r.monitorSupplementalRecords[normalized] = provider
	}

	if r.monitor != nil {
		r.monitor.SetSupplementalRecordsProvider(normalized, provider)
	}
	if r.mtMonitor != nil {
		r.mtMonitor.SetMonitorInitializer(r.configureMonitorDependencies)
	}
}

func (r *Router) syncPlatformSupplementalProviders(mockEnabled bool) {
	if r == nil {
		return
	}

	if mockEnabled {
		truenas.SetFeatureEnabled(true)
		vmware.SetFeatureEnabled(true)

		trueNASAdapter := mockSupplementalRecordsAdapter{source: unifiedresources.SourceTrueNAS}
		vmwareAdapter := mockSupplementalRecordsAdapter{source: unifiedresources.SourceVMware}

		if r.resourceHandlers != nil {
			r.resourceHandlers.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, trueNASAdapter)
			r.resourceHandlers.SetSupplementalRecordsProvider(unifiedresources.SourceVMware, vmwareAdapter)
		}
		r.setMonitorSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, trueNASAdapter)
		r.setMonitorSupplementalRecordsProvider(unifiedresources.SourceVMware, vmwareAdapter)
		return
	}

	truenas.ResetFeatureEnabledFromEnv()
	vmware.ResetFeatureEnabledFromEnv()

	if r.resourceHandlers != nil {
		r.resourceHandlers.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, r.trueNASPoller)
		r.resourceHandlers.SetSupplementalRecordsProvider(unifiedresources.SourceVMware, r.vmwarePoller)
	}
	r.setMonitorSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, r.trueNASPoller)
	r.setMonitorSupplementalRecordsProvider(unifiedresources.SourceVMware, r.vmwarePoller)
}

// getTenantMonitor returns the appropriate monitor for the current request's tenant.
// For non-default orgs, it fails closed when tenant monitor resolution fails.
func (r *Router) getTenantMonitor(ctx context.Context) *monitoring.Monitor {
	orgID := GetOrgID(ctx)

	// Default/legacy path remains backward compatible.
	if orgID == "" || orgID == "default" {
		return r.monitor
	}

	// Security: non-default orgs must never use the default monitor.
	if r.mtMonitor == nil {
		log.Warn().
			Str("org_id", orgID).
			Msg("Tenant monitor unavailable for non-default org request")
		return nil
	}

	monitor, err := r.mtMonitor.GetMonitor(orgID)
	if err != nil || monitor == nil {
		log.Warn().
			Err(err).
			Str("org_id", orgID).
			Msg("Failed to resolve tenant monitor for non-default org request")
		return nil
	}

	r.StartPatrolForOrg(ctx, orgID)

	return monitor
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
	if r.aiSettingsHandler != nil {
		r.aiSettingsHandler.SetConfig(r.config)
	}
	if r.licenseHandlers != nil {
		r.licenseHandlers.SetConfig(r.config)
	}
}

// SetDiscoveryService sets the discovery service for the router.
func (r *Router) SetDiscoveryService(svc *servicediscovery.Service) {
	if r.discoveryHandlers != nil {
		r.discoveryHandlers.SetService(svc)
	}
	if r.resourceHandlers != nil && r.discoveryHandlers != nil {
		r.resourceHandlers.SetDiscoveryReadinessProvider(r.discoveryHandlers)
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

func normalizePatrolOrgID(orgID string) string {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return "default"
	}
	return orgID
}

func (r *Router) markPatrolStarted(orgID string) bool {
	if r == nil {
		return false
	}
	orgID = normalizePatrolOrgID(orgID)
	r.patrolLifecycleMu.Lock()
	defer r.patrolLifecycleMu.Unlock()
	if r.startedPatrolOrgs == nil {
		r.startedPatrolOrgs = make(map[string]bool)
	}
	if r.startedPatrolOrgs[orgID] {
		return false
	}
	r.startedPatrolOrgs[orgID] = true
	return true
}

func (r *Router) clearPatrolStarted(orgID string) {
	if r == nil {
		return
	}
	orgID = normalizePatrolOrgID(orgID)
	r.patrolLifecycleMu.Lock()
	delete(r.startedPatrolOrgs, orgID)
	r.patrolLifecycleMu.Unlock()
}

func (r *Router) patrolCtx(ctx context.Context, orgID string) context.Context {
	orgID = normalizePatrolOrgID(orgID)
	if ctx == nil {
		return context.WithValue(context.Background(), OrgIDContextKey, orgID)
	}
	if existing := strings.TrimSpace(GetOrgID(ctx)); existing != "" {
		return ctx
	}
	return context.WithValue(ctx, OrgIDContextKey, orgID)
}

// StartPatrol starts the AI patrol service for background infrastructure monitoring
func (r *Router) StartPatrol(ctx context.Context) {
	orgID := normalizePatrolOrgID(GetOrgID(ctx))
	r.StartPatrolForOrg(ctx, orgID)
}

// StartPatrolForOrg starts patrol/intelligence lifecycle for a specific org exactly once.
func (r *Router) StartPatrolForOrg(ctx context.Context, orgID string) {
	orgID = normalizePatrolOrgID(orgID)
	if !r.markPatrolStarted(orgID) {
		return
	}
	orgCtx := r.patrolCtx(ctx, orgID)
	if !r.startPatrolForContext(orgCtx, orgID) {
		r.clearPatrolStarted(orgID)
	}
}

// unifiedLifecycleFromAI converts patrol finding lifecycle events into their
// unified-store representation.
func unifiedLifecycleFromAI(events []ai.FindingLifecycleEvent) []unified.UnifiedFindingLifecycleEvent {
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

// unifiedFindingFromAI converts a patrol ai.Finding into its unified-store
// representation. Shared by the live patrol callback and the startup sync of
// persisted findings.
func unifiedFindingFromAI(f *ai.Finding) *unified.UnifiedFinding {
	return &unified.UnifiedFinding{
		ID:                         f.ID,
		Source:                     unified.SourceAIPatrol,
		Severity:                   unified.UnifiedSeverity(f.Severity),
		Category:                   unified.UnifiedCategory(f.Category),
		ResourceID:                 f.ResourceID,
		ResourceName:               f.ResourceName,
		ResourceType:               f.ResourceType,
		Node:                       f.Node,
		Title:                      f.Title,
		Description:                f.Description,
		Impact:                     f.Impact,
		PreviousResolvedFixSummary: f.PreviousResolvedFixSummary,
		Recommendation:             f.Recommendation,
		Evidence:                   f.Evidence,
		DetectedAt:                 f.DetectedAt,
		LastSeenAt:                 f.LastSeenAt,
		ResolvedAt:                 f.ResolvedAt,
		AutoResolved:               f.AutoResolved,
		InvestigationSessionID:     f.InvestigationSessionID,
		InvestigationStatus:        f.InvestigationStatus,
		InvestigationOutcome:       f.InvestigationOutcome,
		LastInvestigatedAt:         f.LastInvestigatedAt,
		InvestigationAttempts:      f.InvestigationAttempts,
		InvestigationRecord:        f.InvestigationRecord,
		LoopState:                  f.LoopState,
		Lifecycle:                  unifiedLifecycleFromAI(f.Lifecycle),
		RegressionCount:            f.RegressionCount,
		LastRegressionAt:           f.LastRegressionAt,
		AcknowledgedAt:             f.AcknowledgedAt,
		SnoozedUntil:               f.SnoozedUntil,
		DismissedReason:            f.DismissedReason,
		UserNote:                   f.UserNote,
		Suppressed:                 f.Suppressed,
		TimesRaised:                f.TimesRaised,
		RemindAt:                   f.RemindAt,
	}
}

func (r *Router) startPatrolForContext(ctx context.Context, orgID string) bool {
	if r == nil || r.aiSettingsHandler == nil {
		return false
	}
	ctx = r.patrolCtx(ctx, orgID)
	aiService := r.aiSettingsHandler.GetAIService(ctx)
	if aiService == nil || !aiService.IsEnabled() {
		return false
	}
	if orgID != "default" && aiService.GetOrgID() != orgID {
		log.Warn().
			Str("org_id", orgID).
			Str("service_org_id", aiService.GetOrgID()).
			Msg("Patrol start aborted: AI service org scope mismatch")
		return false
	}
	monitor := r.getTenantMonitor(ctx)
	if orgID != "default" && monitor == nil {
		log.Warn().
			Str("org_id", orgID).
			Msg("Patrol start aborted: tenant monitor unavailable")
		return false
	}
	persistence := r.persistenceForOrg(ctx)

	// Connect patrol to user-configured alert thresholds so it warns before alerts fire
	if monitor != nil {
		if alertManager := monitor.GetAlertManager(); alertManager != nil {
			thresholdAdapter := ai.NewAlertThresholdAdapter(alertManager)
			aiService.SetPatrolThresholdProvider(thresholdAdapter)
		}
		if incidentStore := monitor.GetIncidentStore(); incidentStore != nil {
			aiService.SetIncidentStore(incidentStore)
			r.aiSettingsHandler.SetIncidentStoreForOrg(orgID, incidentStore)
		}
	}

	// Enable findings persistence (load from disk, auto-save on changes)
	if persistence != nil {
		findingsPersistence := ai.NewFindingsPersistenceAdapter(persistence)
		historyPersistence := ai.NewPatrolHistoryPersistenceAdapter(persistence)
		if patrol := aiService.GetPatrolService(); patrol != nil {
			if err := patrol.SetFindingsPersistence(findingsPersistence); err != nil {
				log.Error().Err(err).Msg("Failed to initialize AI findings persistence")
			}
			// Enable patrol run history persistence
			if err := patrol.SetRunHistoryPersistence(historyPersistence); err != nil {
				log.Error().Err(err).Msg("Failed to initialize AI patrol run history persistence")
			}
		}
	}

	// Connect patrol to metrics history for enriched context (trends, predictions)
	if monitor != nil {
		if metricsHistory := monitor.GetMetricsHistory(); metricsHistory != nil {
			adapter := ai.NewMetricsHistoryAdapter(metricsHistory)
			if adapter != nil {
				aiService.SetMetricsHistoryProvider(adapter)
			}

			// Initialize baseline store for anomaly detection.
			baselineCfg := ai.DefaultBaselineConfig()
			if persistence != nil {
				baselineCfg.DataDir = persistence.DataDir()
			}
			baselineStore := ai.NewBaselineStore(baselineCfg)
			if baselineStore != nil {
				aiService.SetBaselineStore(baselineStore)

				// Start background baseline learning loop
				go r.startBaselineLearning(ctx, baselineStore, metricsHistory)
			}
		}
	}

	// Initialize operational memory (change detection and remediation logging)
	dataDir := ""
	if persistence != nil {
		dataDir = persistence.DataDir()
	}

	changeDetector := ai.NewChangeDetector(ai.ChangeDetectorConfig{
		MaxChanges: 1000,
		DataDir:    dataDir,
	})
	if changeDetector != nil {
		aiService.SetChangeDetector(changeDetector)
	}

	remediationLog := ai.NewRemediationLog(ai.RemediationLogConfig{
		MaxRecords: 500,
		DataDir:    dataDir,
	})
	if remediationLog != nil {
		aiService.SetRemediationLog(remediationLog)
	}

	// Initialize pattern and correlation detectors for AI-enabled orgs.
	if aiService.IsEnabled() {
		// Initialize pattern detector for failure prediction
		patternDetector := ai.NewPatternDetector(ai.PatternDetectorConfig{
			MaxEvents:       5000,
			MinOccurrences:  3,
			PatternWindow:   90 * 24 * time.Hour,
			PredictionLimit: 30 * 24 * time.Hour,
			DataDir:         dataDir,
		})
		if patternDetector != nil {
			aiService.SetPatternDetector(patternDetector)

			// Wire alert history to pattern detector for event tracking
			if monitor != nil {
				if alertManager := monitor.GetAlertManager(); alertManager != nil {
					alertManager.OnAlertHistory(func(alert alerts.Alert) {
						// Convert alert type to trackable event
						patternDetector.RecordFromAlert(alert.ResourceID, alert.Type+"_"+string(alert.Level), alert.StartTime)
					})
					log.Info().Msg("AI Pattern Detector: Wired to alert history for failure prediction")
				}
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
			aiService.SetCorrelationDetector(correlationDetector)

			// Wire alert history to correlation detector
			if monitor != nil {
				if alertManager := monitor.GetAlertManager(); alertManager != nil {
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
	}

	// Initialize new AI intelligence services (Phase 6)
	r.initializeAIIntelligenceServices(ctx, orgID, dataDir, monitor)

	// Wire unified finding callback AFTER initializeAIIntelligenceServices
	// (unified store is created there) and AFTER findings persistence is loaded
	patrol := aiService.GetPatrolService()
	if patrol != nil {
		if unifiedStore := r.aiSettingsHandler.GetUnifiedStoreForOrg(orgID); unifiedStore != nil {
			patrol.SetUnifiedFindingCallback(func(f *ai.Finding) bool {
				uf := unifiedFindingFromAI(f)
				_, isNew := unifiedStore.AddFromAI(uf)
				// Publish a finding.created event to the agent SSE
				// stream when the finding is new and not auto-dismissed
				// by operator-state suppression. Skip on update
				// (re-detection of an existing finding) so agents
				// aren't notified about every patrol cycle's
				// re-confirmation; skip when DismissedReason is
				// populated because the operator already said to stay
				// quiet about this resource. Fire-and-forget — slow
				// subscribers don't block the patrol loop.
				if isNew && r.agentEventBroadcaster != nil && f.DismissedReason == "" {
					r.agentEventBroadcaster.PublishFindingCreated(AgentEventFindingCreatedPayload{
						FindingID:    f.ID,
						ResourceID:   f.ResourceID,
						ResourceName: f.ResourceName,
						Severity:     string(f.Severity),
						Title:        f.Title,
						Category:     string(f.Category),
					})
				}
				return isNew
			})
			patrol.SetUnifiedFindingResolver(func(findingID string) {
				unifiedStore.Resolve(findingID)
			})
			// Wire the agent-consumable findings adapter so the bundled
			// context endpoint can return active findings without the
			// api package importing internal/ai. The closure captures
			// the patrol service and projects each Finding into the
			// agent-stable snapshot shape; agents see the situated
			// picture without any internal type leakage.
			if r.agentContextHandler != nil {
				r.agentContextHandler.SetFindingsProvider(
					agentFindingsProvider{
						activeForResource: func(resourceID string) []AgentResourceFindingSnapshot {
							if patrol == nil {
								return nil
							}
							findings := patrol.GetFindings()
							if findings == nil {
								return nil
							}
							active := findings.GetByResource(resourceID)
							if len(active) == 0 {
								return []AgentResourceFindingSnapshot{}
							}
							out := make([]AgentResourceFindingSnapshot, 0, len(active))
							for _, f := range active {
								snapshot := AgentResourceFindingSnapshot{
									ID:                         f.ID,
									Title:                      f.Title,
									Severity:                   string(f.Severity),
									Category:                   string(f.Category),
									Description:                f.Description,
									Impact:                     f.Impact,
									Recommendation:             f.Recommendation,
									RegressionCount:            f.RegressionCount,
									PreviousResolvedFixSummary: f.PreviousResolvedFixSummary,
								}
								if !f.DetectedAt.IsZero() {
									snapshot.DetectedAt = f.DetectedAt.Format(time.RFC3339)
								}
								if !f.LastSeenAt.IsZero() {
									snapshot.LastSeenAt = f.LastSeenAt.Format(time.RFC3339)
								}
								if f.InvestigationRecord != nil && f.InvestigationRecord.Confidence != "" {
									snapshot.Confidence = string(f.InvestigationRecord.Confidence)
								}
								out = append(out, snapshot)
							}
							return out
						},
						activeCount: func() int {
							if patrol == nil {
								return 0
							}
							findings := patrol.GetFindings()
							if findings == nil {
								return 0
							}
							return len(findings.GetActive(ai.FindingSeverityWarning))
						},
					},
				)
			}
			// Wire per-resource operator-state into the findings runtime so
			// new findings raised against a resource the operator has
			// flagged (maintenance window or intentionally offline) get
			// auto-acknowledged with reason "expected_behavior". One
			// adapter call returns the full projection so adding new
			// signals later does not multiply round-trips per finding.
			// Keeps `internal/ai` clear of an
			// `internal/unifiedresources` import.
			patrol.GetFindings().SetResourceOperatorStateProvider(
				ai.ResourceOperatorStateProviderFunc(
					func(resourceRef string, now time.Time) (ai.ResourceOperatorStateProjection, bool) {
						orgStore, lookupErr := r.resourceHandlers.getStore(orgID)
						if lookupErr != nil {
							return ai.ResourceOperatorStateProjection{}, false
						}
						// Findings carry whatever resource ID their
						// producer used: unified-derived findings hold
						// the canonical hash, but Patrol guest inventory
						// rows hold the node-scoped Proxmox source ID.
						// Operator state is keyed by canonical ID only,
						// so a reference that misses directly resolves
						// through the registry before giving up —
						// otherwise maintenance windows and
						// intentionally-offline intent silently never
						// reach guest findings.
						canonicalID := resourceRef
						state, found, fetchErr := orgStore.GetResourceOperatorState(canonicalID)
						if fetchErr != nil {
							return ai.ResourceOperatorStateProjection{}, false
						}
						if !found {
							registry, registryErr := r.resourceHandlers.buildRegistry(orgID)
							if registryErr != nil || registry == nil {
								return ai.ResourceOperatorStateProjection{}, false
							}
							_, resolvedID, resolved := registry.GetByReference(resourceRef)
							if !resolved || resolvedID == canonicalID {
								return ai.ResourceOperatorStateProjection{}, false
							}
							canonicalID = resolvedID
							state, found, fetchErr = orgStore.GetResourceOperatorState(canonicalID)
						}
						if fetchErr != nil || !found {
							return ai.ResourceOperatorStateProjection{}, false
						}
						projection := ai.ResourceOperatorStateProjection{
							IntentionallyOffline: state.IntentionallyOffline,
							MonitoringMode:       string(state.MonitoringMode),
							LifecycleState:       string(state.LifecycleState),
							NeverAutoRemediate:   state.NeverAutoRemediate,
							Criticality:          string(state.Criticality),
						}
						if state.LifecycleState == unifiedresources.LifecycleStateRetired {
							projection.NeverAutoRemediate = true
						}
						if state.IsInMaintenanceAt(now) {
							projection.MaintenanceWindow = &ai.ResourceOperatorStateMaintenanceWindow{
								StartAt: *state.MaintenanceStartAt,
								EndAt:   *state.MaintenanceEndAt,
								Reason:  state.MaintenanceReason,
							}
						}
						return projection, true
					},
				),
			)

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

			// Wire finding notifications: new warning+ patrol findings go to
			// the operator's alert notification channels (email, webhooks,
			// Apprise), so Patrol can reach operators who do not use the
			// mobile push path. Config gating is read live per finding.
			patrol.SetFindingNotifyCallback(func(f *ai.Finding) {
				if f == nil || ai.IsDemoMode() {
					return
				}
				cfg := aiService.GetAIConfig()
				if cfg == nil || !cfg.PatrolFindingTriggersNotification(string(f.Severity)) {
					return
				}
				if r.monitor == nil {
					return
				}
				if nm := r.monitor.GetNotificationManager(); nm != nil {
					nm.SendAlert(patrolFindingNotificationAlert(f))
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
					uf := unifiedFindingFromAI(f)
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
			r.aiHandler.SetUnifiedStoreForOrg(orgID, unifiedStore)
		}
	}

	// Finally start the actual patrol loop
	r.aiSettingsHandler.StartPatrol(ctx)

	// Wire up discovery service to the handlers
	// This enables the /api/discovery endpoints to trigger discovery scans
	aiService = r.aiSettingsHandler.GetAIService(ctx)
	if aiService != nil {
		if discoveryService := aiService.GetDiscoveryService(); discoveryService != nil {
			r.SetDiscoveryService(discoveryService)
			log.Info().Msg("Discovery: Service wired to API handlers")
		}
		// Wire up AI config provider for showing AI provider info in discovery UI
		r.SetDiscoveryAIConfigProvider(aiService)
	}

	return true
}

// initializeAIIntelligenceServices sets up the new AI intelligence subsystems
func (r *Router) initializeAIIntelligenceServices(ctx context.Context, orgID, dataDir string, monitor *monitoring.Monitor) {
	aiService := r.aiSettingsHandler.GetAIService(ctx)
	if aiService == nil || !aiService.IsEnabled() {
		return
	}
	if orgID != "default" && aiService.GetOrgID() != orgID {
		log.Warn().
			Str("org_id", orgID).
			Str("service_org_id", aiService.GetOrgID()).
			Msg("AI intelligence initialization skipped: AI service org scope mismatch")
		return
	}

	// 1. Initialize circuit breaker for resilient patrol
	circuitBreaker := circuit.NewBreaker("patrol", circuit.DefaultConfig())
	r.aiSettingsHandler.SetCircuitBreakerForOrg(orgID, circuitBreaker)
	log.Info().Msg("AI Intelligence: Circuit breaker initialized")

	// 2. Initialize learning store for feedback learning
	learningCfg := learning.LearningStoreConfig{
		DataDir: dataDir,
	}
	learningStore := learning.NewLearningStore(learningCfg)
	r.aiSettingsHandler.SetLearningStoreForOrg(orgID, learningStore)
	log.Info().Msg("AI Intelligence: Learning store initialized")

	// 4. Initialize forecast service for trend forecasting
	forecastCfg := forecast.DefaultForecastConfig()
	forecastService := forecast.NewService(forecastCfg)
	// Wire up data provider adapter
	if monitor != nil {
		if metricsHistory := monitor.GetMetricsHistory(); metricsHistory != nil {
			dataAdapter := adapters.NewForecastDataAdapter(metricsHistory)
			if dataAdapter != nil {
				forecastService.SetDataProvider(dataAdapter)
			}
		}
	}
	// Wire up resource iterator for forecast context (via ReadState)
	if monitor != nil {
		if rs := monitor.GetUnifiedReadState(); rs != nil {
			forecastService.SetResourceIterator(&forecastResourceIterator{readState: rs})
		} else {
			log.Warn().Msg("AI Intelligence: Forecast resource iterator not wired — ReadState unavailable")
		}
	}
	r.aiSettingsHandler.SetForecastServiceForOrg(orgID, forecastService)
	log.Info().Msg("AI Intelligence: Forecast service initialized")

	// 5. Initialize Proxmox event correlator
	proxmoxCfg := proxmox.DefaultEventCorrelatorConfig()
	proxmoxCfg.DataDir = dataDir
	proxmoxCorrelator := proxmox.NewEventCorrelator(proxmoxCfg)
	r.aiSettingsHandler.SetProxmoxCorrelatorForOrg(orgID, proxmoxCorrelator)
	log.Info().Msg("AI Intelligence: Proxmox event correlator initialized")

	// 7. Initialize remediation engine for AI-guided fixes (requires Pulse Pro)
	var remediationEngine aicontracts.RemediationEngine
	if isAIInvestigationEnabled() {
		remediationCfg := aicontracts.DefaultEngineConfig()
		remediationCfg.DataDir = dataDir
		if factory := getCreateRemediationEngine(); factory != nil {
			remediationEngine = factory(remediationCfg)
		}
		if remediationEngine != nil {
			// Wire up command executor (disabled by default for safety)
			cmdExecutor := adapters.NewCommandExecutorAdapter()
			remediationEngine.SetCommandExecutor(cmdExecutor)
			r.aiSettingsHandler.SetRemediationEngineForOrg(orgID, remediationEngine)
			log.Info().Msg("AI Intelligence: Remediation engine initialized (command execution disabled)")
		} else {
			// Clear any stale engine from a prior init cycle.
			r.aiSettingsHandler.SetRemediationEngineForOrg(orgID, nil)
			log.Info().Msg("AI Intelligence: Remediation engine factory not registered")
		}
	} else {
		log.Info().Msg("AI Intelligence: Remediation engine skipped (requires Pulse Pro)")
	}

	// 8. Initialize unified alert/finding system and bridge
	if monitor != nil {
		if alertManager := monitor.GetAlertManager(); alertManager != nil {
			// Create unified store
			unifiedStore := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
			r.aiSettingsHandler.SetUnifiedStoreForOrg(orgID, unifiedStore)

			// Create alert bridge
			alertBridge := unified.NewAlertBridge(unifiedStore, unified.DefaultBridgeConfig())

			// Create and set alert provider adapter
			alertAdapter := unified.NewAlertManagerAdapter(alertManager)
			alertBridge.SetAlertProvider(alertAdapter)

			// Set patrol trigger function (triggers mini-patrol on alert events)
			patrol := aiService.GetPatrolService()
			if patrol != nil && ai.BackgroundAutomationDisabledForDev() {
				log.Info().
					Str("env", ai.DevDisableBackgroundAIEnv).
					Str("org_id", orgID).
					Msg("Pulse dev background AI disabled; alert bridge patrol trigger not wired")
			} else if patrol != nil {
				alertBridge.SetPatrolTrigger(func(event unified.PatrolTriggerEvent) {
					scope := ai.PatrolScope{
						ResourceIDs:     []string{event.ResourceID},
						ResourceTypes:   []string{event.ResourceType},
						Depth:           ai.PatrolDepthQuick,
						Context:         "Alert bridge: " + event.Reason,
						Priority:        50,
						AlertIdentifier: event.AlertIdentifier,
					}
					if event.AlertType != "" {
						scope.AlertContext = &ai.PatrolAlertContext{
							AlertType: event.AlertType,
							Level:     event.AlertLevel,
							Value:     event.Value,
							Threshold: event.Threshold,
							Message:   event.Message,
						}
					}
					switch event.Reason {
					case "alert_fired":
						// Per-rule policy: only investigate alerts the operator opted
						// in (minimum severity + optional alert-type allowlist).
						if aiCfg := aiService.GetAIConfig(); aiCfg != nil &&
							!aiCfg.AlertTriggersInvestigation(event.AlertType, event.AlertLevel) {
							log.Debug().
								Str("resource_id", event.ResourceID).
								Str("alert_type", event.AlertType).
								Str("level", event.AlertLevel).
								Msg("Alert bridge: alert-triggered patrol skipped by trigger policy")
							return
						}
						scope.Reason = ai.TriggerReasonAlertFired
						scope.Priority = 80
						if event.AlertType != "" {
							scope.Context = fmt.Sprintf("Alert: %s = %.1f (threshold %.1f)", event.AlertType, event.Value, event.Threshold)
						}
					case "alert_cleared":
						scope.Reason = ai.TriggerReasonAlertCleared
						scope.Priority = 40
						if event.AlertType != "" {
							scope.Context = "Alert cleared: " + event.AlertType
						}
					default:
						scope.Reason = ai.TriggerReasonManual
					}

					log.Debug().
						Str("resource_id", event.ResourceID).
						Str("reason", event.Reason).
						Msg("Alert bridge: Triggering mini-patrol")
					if triggerManager := r.aiSettingsHandler.GetTriggerManagerForOrg(orgID); triggerManager != nil {
						if triggerManager.TriggerPatrol(scope) {
							log.Debug().
								Str("resource_id", event.ResourceID).
								Str("reason", event.Reason).
								Msg("Alert bridge: Queued patrol via trigger manager")
						} else {
							log.Warn().
								Str("resource_id", event.ResourceID).
								Str("reason", event.Reason).
								Msg("Alert bridge: Patrol trigger rejected by trigger manager")
						}
						return
					}
					orgCtx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
					patrol.TriggerScopedPatrol(orgCtx, scope)
				})
			}

			// Start the bridge
			alertBridge.Start()
			r.aiSettingsHandler.SetAlertBridgeForOrg(orgID, alertBridge)
			log.Info().Msg("AI Intelligence: Unified alert/finding bridge initialized and started")
		}
	}

	// 9. Wire up AI intelligence providers to patrol service for context injection
	patrol := aiService.GetPatrolService()
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

		// Wire guest prober for pre-patrol reachability checks via Unified Agents
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
		r.aiSettingsHandler.SetTriggerManagerForOrg(orgID, triggerManager)

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

		// 11b. Wire alert-flapping callback to the trigger manager and surface
		// a postmortem finding so the operator sees the diagnosis instead of
		// silence. The alerts manager fires this callback exactly once per
		// flapping transition (one-shot for the cooldown window); we both
		// emit a reliability finding via Path B (direct FindingsStore.Add
		// with a stable dedup ID derived from trackingKey) AND enqueue a
		// scoped patrol so the postmortem context can be enriched later.
		if monitor != nil {
			if alertManager := monitor.GetAlertManager(); alertManager != nil {
				alertManager.SetFlappingDetectedCallback(func(alert *alerts.Alert, trackingKey string) {
					if alert == nil || trackingKey == "" {
						return
					}
					emitFlappingPostmortemFinding(patrol, alertManager, alert, trackingKey)
					scope := ai.FlappingPostmortemPatrolScope(
						alert.ID,
						alert.ResourceID,
						alert.CanonicalKind,
						alert.Type,
					)
					if triggerManager.TriggerPatrol(scope) {
						log.Debug().
							Str("alertID", alert.ID).
							Str("trackingKey", trackingKey).
							Str("alertType", alert.Type).
							Msg("Flapping-detected triggered postmortem patrol via TriggerManager")
					}
				})
				log.Info().Msg("AI Intelligence: Alert-flapping callback wired to trigger manager")
			}
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

		// Create metrics adapter for incident recorder (ReadState is sole source since SRC-03m)
		var metricsAdapter *adapters.MetricsAdapter
		if monitor != nil {
			metricsAdapter = adapters.NewMetricsAdapter(monitor.GetUnifiedReadState())
		}

		// Initialize and wire the incident recorder (high-frequency metrics)
		if metricsAdapter != nil {
			recorderCfg := metrics.DefaultIncidentRecorderConfig()
			recorderCfg.DataDir = dataDir
			recorder := metrics.NewIncidentRecorder(recorderCfg)
			recorder.SetMetricsProvider(metricsAdapter)
			recorder.Start()
			incidentCoordinator.SetRecorder(recorder)
			r.aiSettingsHandler.SetIncidentRecorderForOrg(orgID, recorder)
			log.Info().Msg("AI Intelligence: Incident recorder initialized and started")
		}

		// Start the coordinator
		incidentCoordinator.Start()

		// Store reference
		r.aiSettingsHandler.SetIncidentCoordinatorForOrg(orgID, incidentCoordinator)

		log.Info().Msg("AI Intelligence: Incident coordinator initialized and started")
	}

	log.Info().Msg("AI Intelligence: All Phase 6 & 7 services initialized successfully")
}

// StopPatrol stops the AI patrol service
func (r *Router) StopPatrol() {
	if r.aiSettingsHandler != nil {
		r.aiSettingsHandler.StopPatrol()
	}
	r.patrolLifecycleMu.Lock()
	r.startedPatrolOrgs = make(map[string]bool)
	r.patrolLifecycleMu.Unlock()
}

// StopPatrolForOrg stops patrol for a single org and clears its lifecycle marker.
func (r *Router) StopPatrolForOrg(orgID string) {
	orgID = normalizePatrolOrgID(orgID)
	if r.aiSettingsHandler != nil {
		r.aiSettingsHandler.StopPatrolForOrg(orgID)
	}
	r.clearPatrolStarted(orgID)
}

// ShutdownAIIntelligence gracefully shuts down all AI intelligence services (Phase 6)
// This should be called during application shutdown to ensure proper cleanup
func (r *Router) ShutdownAIIntelligence() {
	r.shutdownBackgroundWorkers()

	if r.aiSettingsHandler == nil {
		return
	}

	log.Info().Msg("AI Intelligence: Starting graceful shutdown")

	// 1. Stop alert bridges (stop listening for alert events)
	for orgID, alertBridge := range r.aiSettingsHandler.ListAlertBridges() {
		if alertBridge == nil {
			continue
		}
		alertBridge.Stop()
		log.Debug().Str("org_id", orgID).Msg("AI Intelligence: Alert bridge stopped")
	}

	// 2. Stop patrol service for all tenants (waits for in-flight investigations, force-saves state)
	// Use StopPatrol() which stops patrol for both legacy and all tenant services
	r.aiSettingsHandler.StopPatrol()
	log.Debug().Msg("AI Intelligence: All patrol services stopped")

	// 3. Stop trigger managers (stop event-driven patrol scheduling)
	for orgID, triggerManager := range r.aiSettingsHandler.ListTriggerManagers() {
		if triggerManager == nil {
			continue
		}
		triggerManager.Stop()
		log.Debug().Str("org_id", orgID).Msg("AI Intelligence: Trigger manager stopped")
	}

	// 4. Stop incident coordinators (stop high-frequency recording)
	for orgID, incidentCoordinator := range r.aiSettingsHandler.ListIncidentCoordinators() {
		if incidentCoordinator == nil {
			continue
		}
		incidentCoordinator.Stop()
		log.Debug().Str("org_id", orgID).Msg("AI Intelligence: Incident coordinator stopped")
	}

	// 4b. Stop incident recorders (stop background sampling)
	for orgID, incidentRecorder := range r.aiSettingsHandler.ListIncidentRecorders() {
		if incidentRecorder == nil {
			continue
		}
		incidentRecorder.Stop()
		log.Debug().Str("org_id", orgID).Msg("AI Intelligence: Incident recorder stopped")
	}

	// 5. Cleanup learning stores (removes old records, persists if data dir configured)
	for orgID, learningStore := range r.aiSettingsHandler.ListLearningStores() {
		if learningStore == nil {
			continue
		}
		learningStore.Cleanup()
		log.Debug().Str("org_id", orgID).Msg("AI Intelligence: Learning store cleaned up")
	}

	log.Info().Msg("AI Intelligence: Graceful shutdown complete")
}

// ShutdownRBAC closes every organization RBAC store owned by this router and
// clears the global manager only when it points at the same provider.
// ShutdownResourceStores closes every cached per-org unified resource store.
//
// getStore opens these lazily and caches them for the process lifetime, so
// without an explicit shutdown the SQLite handles and their -wal/-shm files
// outlive the router. Tests that build a Router against a temporary data
// directory must call this, or the open handles race directory cleanup.
func (r *Router) ShutdownResourceStores() {
	if r == nil || r.resourceHandlers == nil {
		return
	}
	if err := r.resourceHandlers.CloseStores(); err != nil {
		log.Warn().Err(err).Msg("failed to close unified resource stores")
	}
}

func (r *Router) ShutdownRBAC() {
	if r.rbacProvider == nil {
		return
	}
	ownsGlobal := r.rbacProvider.ownsManager(auth.GetManager())
	if err := r.rbacProvider.Close(); err != nil {
		log.Error().Err(err).Msg("Failed to close RBAC stores")
	}
	if ownsGlobal {
		auth.SetManager(nil)
	}
}

func (r *Router) shutdownBackgroundWorkers() {
	if r.lifecycleCancel != nil {
		r.lifecycleCancel()
	}
	if r.updateManager != nil {
		r.updateManager.Close()
	}
	if r.sessionStore != nil {
		r.sessionStore.Shutdown()
	}
	if r.csrfStore != nil {
		r.csrfStore.Shutdown()
	}
	if r.recoveryTokenStore != nil {
		r.recoveryTokenStore.Shutdown()
	}
	if r.aiSettingsHandler != nil {
		r.aiSettingsHandler.StopServices()
	}
	if r.aiHandler != nil {
		r.aiHandler.clearApprovalStore()
	}
	if r.trueNASPoller != nil {
		r.trueNASPoller.Stop()
	}
	if r.vmwarePoller != nil {
		r.vmwarePoller.Stop()
	}
	if r.deployStore != nil {
		if err := r.deployStore.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close deploy store")
		}
	}
	r.lifecycleWG.Wait()
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

	r.wireAIChatRuntimeAfterStart()
}

// wireAIChatRuntimeAfterStart applies the dependencies that require a live
// chat service. Settings-driven hot enablement and provider restarts must use
// the same wiring path as cold startup; otherwise Patrol detection can run
// while investigation remains unavailable until the whole server restarts.
func (r *Router) wireAIChatRuntimeAfterStart() {
	if r == nil || r.aiHandler == nil {
		return
	}

	defaultOrgCtx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	if !r.aiHandler.IsRunning(defaultOrgCtx) {
		return
	}
	if service := r.aiHandler.GetService(defaultOrgCtx); service != nil {
		r.wireAIChatDependenciesForService(defaultOrgCtx, service)
	}

	// Investigation orchestration depends on the live chat service and must be
	// rebuilt after both first enablement and an in-process provider restart.
	if r.aiSettingsHandler != nil {
		r.aiSettingsHandler.WireOrchestratorAfterChatStart()
		aiService := r.aiSettingsHandler.GetAIService(defaultOrgCtx)
		if aiService == nil {
			return
		}
		if patrolSvc := aiService.GetPatrolService(); patrolSvc != nil {
			if breaker := r.aiSettingsHandler.GetCircuitBreakerForOrg("default"); breaker != nil {
				patrolSvc.SetCircuitBreaker(breaker)
				log.Info().Msg("AI patrol circuit breaker wired")
			}
		}
	}
}

func (r *Router) persistenceForOrg(ctx context.Context) *config.ConfigPersistence {
	if r == nil {
		return nil
	}

	orgID := strings.TrimSpace(GetOrgID(ctx))
	if orgID == "" || orgID == "default" {
		return r.persistence
	}

	if r.multiTenant != nil {
		if p, err := r.multiTenant.GetPersistence(orgID); err == nil {
			return p
		}
	}

	return nil
}

// wireAIChatDependenciesForService wires org-scoped Assistant tool providers and chat-service
// integration for a specific chat service instance.
func (r *Router) wireAIChatDependenciesForService(ctx context.Context, service AIService) {
	if r == nil || service == nil {
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if r.resourceHandlers != nil {
		if plannerConsumer, ok := service.(interface {
			SetTypedActionPlanner(chat.AssistantTypedActionPlanner)
		}); ok {
			plannerConsumer.SetTypedActionPlanner(assistantTypedActionPlanner{resources: r.resourceHandlers})
		}
	}
	orgID := strings.TrimSpace(GetOrgID(ctx))
	if orgID == "" {
		orgID = "default"
		ctx = context.WithValue(context.Background(), OrgIDContextKey, orgID)
	}

	monitor := r.getTenantMonitor(ctx)
	aiService := (*ai.Service)(nil)
	if r.aiSettingsHandler != nil {
		aiService = r.aiSettingsHandler.GetAIService(ctx)
		if aiService != nil && orgID != "default" && aiService.GetOrgID() != orgID {
			log.Warn().
				Str("org_id", orgID).
				Str("service_org_id", aiService.GetOrgID()).
				Msg("AI chat dependency wiring skipped: AI service org scope mismatch")
			aiService = nil
		}
	}

	chatService, ok := service.(*chat.Service)
	if !ok {
		log.Warn().Msg("Chat service is not *chat.Service, cannot create patrol adapter")
	} else if aiService != nil {
		aiService.SetChatService(&chatServiceAdapter{svc: chatService})

		// Wire mid-run budget enforcement from AI service to chat service.
		chatService.SetBudgetChecker(func() error {
			return aiService.CheckBudget("patrol")
		})

		log.Info().Str("org_id", orgID).Msg("Chat service wired to AI service for patrol and investigation")
	}

	// Bridge action-audit completion into the agent SSE stream so an
	// agent holding /api/agent/events open hears about every dispatch
	// outcome — Completed, Failed, or refused-before-dispatch with the
	// stable `plan_drift:` / `resource_remediation_locked:` token —
	// without polling the audit endpoint. Independent of aiService
	// availability: the executor exists per-org regardless of whether
	// the patrol/investigation surface is wired up. The callback is
	// fire-and-forget on the dispatch hot path; PublishActionCompleted
	// drops events for slow subscribers rather than blocking.
	if chatService != nil && r.agentEventBroadcaster != nil {
		if executor := chatService.GetExecutor(); executor != nil {
			broadcaster := r.agentEventBroadcaster
			executor.SetOnActionCompleted(func(record unifiedresources.ActionAuditRecord) {
				r.publishActionCompletedAgentEvent(broadcaster, record)
			})
		}
	}

	// Wire alert provider
	if monitor != nil {
		if alertManager := monitor.GetAlertManager(); alertManager != nil {
			alertAdapter := tools.NewAlertManagerToolAdapter(alertManager)
			if alertAdapter != nil {
				service.SetAlertProvider(alertAdapter)
				log.Debug().Msg("AI chat: Alert provider wired")
			}
		}
	}

	// Wire findings provider from patrol service.
	if aiService != nil {
		if patrolSvc := aiService.GetPatrolService(); patrolSvc != nil {
			if r.aiSettingsHandler != nil {
				if breaker := r.aiSettingsHandler.GetCircuitBreakerForOrg(orgID); breaker != nil {
					patrolSvc.SetCircuitBreaker(breaker)
				}
			}
			if findingsStore := patrolSvc.GetFindings(); findingsStore != nil {
				findingsAdapter := ai.NewFindingsToolAdapter(findingsStore)
				if findingsAdapter != nil {
					service.SetFindingsProvider(findingsAdapter)
					log.Debug().Msg("AI chat: Findings provider wired")
				}
			}
		}
	}

	if persistence := r.persistenceForOrg(ctx); persistence != nil {
		var licenseSvc licenseFeatureChecker
		if r.licenseHandlers != nil {
			licenseSvc = r.licenseHandlers.Service(ctx)
		}
		manager := NewAssistantAgentProfileManager(persistence, licenseSvc)
		service.SetAgentProfileManager(manager)
		log.Debug().Msg("AI chat: Agent profile manager wired")
	}

	// Wire guest config provider (storage provider wiring removed)
	if monitor != nil {
		guestConfigAdapter := tools.NewGuestConfigToolAdapter(monitor)
		if guestConfigAdapter != nil {
			service.SetGuestConfigProvider(guestConfigAdapter)
			log.Debug().Msg("AI chat: Guest config provider wired")
		}
	}

	// Wire backup provider
	if monitor != nil {
		m := monitor
		backupAdapter := tools.NewBackupToolAdapter(
			func() models.Backups { return m.BackupsSnapshot() },
			func() []models.PBSInstance { return m.PBSInstancesSnapshot() },
		)
		if backupAdapter != nil {
			service.SetBackupProvider(backupAdapter)
			log.Debug().Msg("AI chat: Backup provider wired")
		}
	}

	// Wire disk health provider
	if monitor != nil {
		diskHealthAdapter := tools.NewDiskHealthToolAdapter(monitor.GetUnifiedReadState())
		if diskHealthAdapter != nil {
			service.SetDiskHealthProvider(diskHealthAdapter)
			log.Debug().Msg("AI chat: Disk health provider wired")
		}
	}

	// Wire updates provider for Docker container updates
	if monitor != nil {
		cfg := r.config
		if monitorCfg := monitor.GetConfig(); monitorCfg != nil {
			cfg = monitorCfg
		}
		updatesAdapter := tools.NewUpdatesToolAdapter(
			monitor.GetUnifiedReadState(),
			monitor,
			&updatesConfigWrapper{cfg: cfg},
		)
		if updatesAdapter != nil {
			service.SetUpdatesProvider(updatesAdapter)
			log.Debug().Msg("AI chat: Updates provider wired")
		}
	}

	// Wire metrics history provider
	if monitor != nil {
		if metricsHistory := monitor.GetMetricsHistory(); metricsHistory != nil {
			metricsAdapter := tools.NewMetricsHistoryToolAdapter(
				&metricsSourceWrapper{history: metricsHistory},
				monitor.GetUnifiedReadState(),
			)
			if metricsAdapter != nil {
				service.SetMetricsHistory(metricsAdapter)
				log.Debug().Msg("AI chat: Metrics history provider wired")
			}
		}
	}

	// Wire baseline provider.
	if aiService != nil {
		if patrolSvc := aiService.GetPatrolService(); patrolSvc != nil {
			if baselineStore := patrolSvc.GetBaselineStore(); baselineStore != nil {
				baselineAdapter := tools.NewBaselineToolAdapter(&baselineSourceWrapper{store: baselineStore})
				if baselineAdapter != nil {
					service.SetBaselineProvider(baselineAdapter)
					log.Debug().Msg("AI chat: Baseline provider wired")
				}
			}
		}
	}

	// Wire pattern provider.
	if aiService != nil {
		if patrolSvc := aiService.GetPatrolService(); patrolSvc != nil {
			if patternDetector := patrolSvc.GetPatternDetector(); patternDetector != nil {
				patternAdapter := tools.NewPatternToolAdapter(
					&patternSourceWrapper{detector: patternDetector},
					monitor.GetUnifiedReadState(),
				)
				if patternAdapter != nil {
					service.SetPatternProvider(patternAdapter)
					log.Debug().Msg("AI chat: Pattern provider wired")
				}
			}
		}
	}

	// Wire findings manager.
	if aiService != nil {
		if patrolSvc := aiService.GetPatrolService(); patrolSvc != nil {
			findingsManagerAdapter := tools.NewFindingsManagerToolAdapter(patrolSvc)
			if findingsManagerAdapter != nil {
				service.SetFindingsManager(findingsManagerAdapter)
				log.Debug().Msg("AI chat: Findings manager wired")
			}
		}
	}

	// Wire metadata updater.
	if aiService != nil {
		metadataAdapter := tools.NewMetadataUpdaterToolAdapter(aiService)
		if metadataAdapter != nil {
			service.SetMetadataUpdater(metadataAdapter)
			log.Debug().Msg("AI chat: Metadata updater wired")
		}
	}

	// Wire intelligence providers for Assistant tools.
	// - IncidentRecorderProvider: high-frequency incident data (pulse_get_incident_window)
	// - EventCorrelatorProvider: Proxmox events (pulse_correlate_events)
	// - KnowledgeStoreProvider: notes (pulse_remember, pulse_recall)

	// Wire incident recorder provider (high-frequency incident data)
	if r.aiSettingsHandler != nil {
		if recorder := r.aiSettingsHandler.GetIncidentRecorderForOrg(orgID); recorder != nil {
			service.SetIncidentRecorderProvider(&incidentRecorderProviderWrapper{recorder: recorder})
			log.Debug().Msg("AI chat: Incident recorder provider wired")
		}
	}

	// Wire event correlator provider (Proxmox events)
	if r.aiSettingsHandler != nil {
		if correlator := r.aiSettingsHandler.GetProxmoxCorrelatorForOrg(orgID); correlator != nil {
			service.SetEventCorrelatorProvider(&eventCorrelatorProviderWrapper{correlator: correlator})
			log.Debug().Msg("AI chat: Event correlator provider wired")
		}
	}

	// Wire knowledge store provider for notes (pulse_remember, pulse_recall).
	if aiService != nil {
		if patrolSvc := aiService.GetPatrolService(); patrolSvc != nil {
			if knowledgeStore := patrolSvc.GetKnowledgeStore(); knowledgeStore != nil {
				service.SetKnowledgeStoreProvider(&knowledgeStoreProviderWrapper{store: knowledgeStore})
				log.Debug().Msg("AI chat: Knowledge store provider wired")
			}
		}
	}

	// Wire discovery provider for AI-powered infrastructure discovery (pulse_get_discovery, pulse_list_discoveries).
	if aiService != nil {
		if discoverySvc := aiService.GetDiscoveryService(); discoverySvc != nil {
			adapter := servicediscovery.NewToolsAdapter(discoverySvc)
			if adapter != nil {
				service.SetDiscoveryProvider(tools.NewDiscoveryToolAdapter(adapter))
				log.Debug().Msg("AI chat: Discovery provider wired")
			}
		}
	}

	// Wire unified resource provider for physical disks, Ceph, etc.
	if monitor != nil {
		if provider := r.unifiedResourceProviderForMonitor(monitor); provider != nil {
			service.SetUnifiedResourceProvider(provider)
			log.Debug().Msg("AI chat: Unified resource provider wired")
		}
	} else if orgID == "default" {
		if provider := r.defaultUnifiedResourceProvider(); provider != nil {
			service.SetUnifiedResourceProvider(provider)
			log.Debug().Msg("AI chat: Unified resource provider wired")
		}
	}
	if provider := newTrueNASAppActionProvider(r.trueNASPoller); provider != nil {
		service.SetAppContainerActionProvider(provider)
		log.Debug().Msg("AI chat: App-container action provider wired")
	}
	if provider := newTrueNASAppReadProvider(r.trueNASPoller); provider != nil {
		service.SetAppContainerReadProvider(provider)
		log.Debug().Msg("AI chat: App-container read provider wired")
	}
	if provider := newTrueNASAppConfigProvider(r.trueNASPoller); provider != nil {
		service.SetAppContainerConfigProvider(provider)
		log.Debug().Msg("AI chat: App-container config provider wired")
	}

	log.Info().Str("org_id", orgID).Msg("AI chat Assistant tool providers wired")
}

// forecastResourceIterator wraps ReadState to implement forecast.ResourceIterator.
// Converts typed view accessors (ReadState) to forecast.ResourceInfo slices.
type forecastResourceIterator struct {
	readState unifiedresources.ReadState
}

func (w *forecastResourceIterator) ForecastVMs() []forecast.ResourceInfo {
	if w.readState == nil {
		return nil
	}
	vms := w.readState.VMs()
	result := make([]forecast.ResourceInfo, 0, len(vms))
	for _, vm := range vms {
		result = append(result, forecast.ResourceInfo{ID: vm.SourceID(), Name: vm.Name()})
	}
	return result
}

func (w *forecastResourceIterator) ForecastContainers() []forecast.ResourceInfo {
	if w.readState == nil {
		return nil
	}
	cts := w.readState.Containers()
	result := make([]forecast.ResourceInfo, 0, len(cts))
	for _, ct := range cts {
		result = append(result, forecast.ResourceInfo{ID: ct.SourceID(), Name: ct.Name()})
	}
	return result
}

func (w *forecastResourceIterator) ForecastNodes() []forecast.ResourceInfo {
	if w.readState == nil {
		return nil
	}
	nodes := w.readState.Nodes()
	result := make([]forecast.ResourceInfo, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, forecast.ResourceInfo{ID: node.SourceID(), Name: node.Name()})
	}
	return result
}

func (w *forecastResourceIterator) ForecastStoragePools() []forecast.ResourceInfo {
	if w.readState == nil {
		return nil
	}
	pools := w.readState.StoragePools()
	result := make([]forecast.ResourceInfo, 0, len(pools))
	for _, sp := range pools {
		result = append(result, forecast.ResourceInfo{ID: sp.SourceID(), Name: sp.Name()})
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

func (r *Router) publishActionCompletedAgentEvent(broadcaster *AgentEventBroadcaster, record unifiedresources.ActionAuditRecord) {
	if broadcaster == nil {
		return
	}
	payload, ok := projectAgentActionCompletedPayload(record)
	if !ok {
		broadcaster.PublishActionCompletedRecord(record)
		return
	}
	broadcaster.PublishActionCompleted(payload)
}

func projectAgentActionCompletedPayload(record unifiedresources.ActionAuditRecord) (AgentEventActionCompletedPayload, bool) {
	if record.State != unifiedresources.ActionStateCompleted && record.State != unifiedresources.ActionStateFailed {
		return AgentEventActionCompletedPayload{}, false
	}

	payload := AgentEventActionCompletedPayload{
		ActionID:       record.ID,
		ResourceID:     record.Request.ResourceID,
		CapabilityName: record.Request.CapabilityName,
		State:          string(record.State),
		RequestedBy:    record.Request.RequestedBy,
		CompletedAt:    record.UpdatedAt,
	}
	if cmd, ok := record.Request.Params["command"].(string); ok {
		payload.Command = cmd
	}
	canonical := unifiedresources.CanonicalActionResultV2(record)
	payload.ActionResultV2 = &canonical
	legacy := unifiedresources.LegacyActionResultProjection(record)
	payload.Success = legacy.Success
	payload.ErrorMessage = legacy.ErrorMessage
	if v := projectAgentResourceVerification(unifiedresources.CanonicalActionVerification(record)); v != nil {
		payload.Verification = v
	}
	return payload, true
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
			r.wireAIChatRuntimeAfterStart()
			log.Info().Msg("AI chat service restarted with new configuration")
		}
	}
}

// StartRelay starts the relay client if configured and licensed.
func (r *Router) StartRelay(ctx context.Context) {
	cfg, err := r.loadRelayConfigForRuntime(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load relay config")
		return
	}
	r.relayMu.Lock()
	r.relayAlertMinimumSeverity = relay.NormalizeAlertMinimumSeverity(cfg.AlertMinimumSeverity)
	r.relayMu.Unlock()
	if !cfg.Enabled {
		log.Debug().Msg("Relay not enabled, skipping")
		return
	}

	// Check license
	if r.licenseHandlers != nil {
		svc := r.licenseHandlers.Service(ctx)
		if svc != nil {
			if err := svc.RequireFeature(featureRelayKey); err != nil {
				log.Warn().Msg("Relay feature not licensed, skipping")
				return
			}
		}
	}

	localAddr := fmt.Sprintf("127.0.0.1:%d", r.config.FrontendPort)

	deps := relay.ClientDeps{
		LicenseTokenFunc: func() string {
			return r.relayRegistrationToken(context.Background())
		},
		TokenValidator: func(token string) bool {
			config.Mu.Lock()
			_, ok := r.config.ValidateAPIToken(token)
			config.Mu.Unlock()
			return ok
		},
		LocalAddr: localAddr,
		// Dispatch proxied requests to our own handler chain in-process.
		// The main listener may serve TLS (HTTPS_ENABLED) or bind a
		// non-loopback address, so dialing localAddr is not reliable.
		LocalHandler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			r.Handler().ServeHTTP(w, req)
		}),
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
	cfg, err := r.loadRelayConfigForRuntime(req.Context())
	if err != nil {
		http.Error(w, "failed to load relay config", http.StatusInternalServerError)
		return
	}

	// Omit the instance secret and private key from the response
	resp := struct {
		Enabled              bool   `json:"enabled"`
		ServerURL            string `json:"server_url"`
		IdentityPublicKey    string `json:"identity_public_key,omitempty"`
		IdentityFingerprint  string `json:"identity_fingerprint,omitempty"`
		AlertMinimumSeverity string `json:"alert_minimum_severity"`
	}{
		Enabled:              cfg.Enabled,
		ServerURL:            cfg.ServerURL,
		IdentityPublicKey:    cfg.IdentityPublicKey,
		IdentityFingerprint:  cfg.IdentityFingerprint,
		AlertMinimumSeverity: relay.NormalizeAlertMinimumSeverity(cfg.AlertMinimumSeverity),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (r *Router) handleUpdateRelayConfig(w http.ResponseWriter, req *http.Request) {
	var update struct {
		Enabled              *bool   `json:"enabled"`
		ServerURL            *string `json:"server_url"`
		InstanceSecret       *string `json:"instance_secret"`
		AlertMinimumSeverity *string `json:"alert_minimum_severity"`
	}
	if err := decodeSecurityRequestBody(w, req, &update); err != nil {
		http.Error(w, "invalid request body", securityRequestErrorStatus(err))
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
	if update.AlertMinimumSeverity != nil {
		cfg.AlertMinimumSeverity = relay.NormalizeAlertMinimumSeverity(*update.AlertMinimumSeverity)
	}

	// Generate a full identity keypair on first enable, or repair a partial
	// identity so the running client can sign mobile key exchanges.
	identityGenerated := false
	if cfg.Enabled {
		generated, err := ensureRelayIdentityKeyPair(&cfg)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate relay identity keypair")
			http.Error(w, "failed to generate identity keypair", http.StatusInternalServerError)
			return
		}
		identityGenerated = generated
		if identityGenerated {
			log.Info().Str("fingerprint", cfg.IdentityFingerprint).Msg("Generated relay instance identity keypair")
		}
	}

	if err := r.persistence.SaveRelayConfig(cfg); err != nil {
		http.Error(w, "failed to save relay config", http.StatusInternalServerError)
		return
	}
	r.relayMu.Lock()
	r.relayAlertMinimumSeverity = relay.NormalizeAlertMinimumSeverity(cfg.AlertMinimumSeverity)
	r.relayMu.Unlock()

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

// learnBaselines updates baselines for all resources from metrics history.
// Uses ReadState typed views — the legacy GetState fallback was removed
// after ReadState became the sole state access mechanism (SRC-03e+).
func (r *Router) learnBaselines(store *ai.BaselineStore, metricsHistory *monitoring.MetricsHistory) {
	if r.monitor == nil {
		return
	}

	readState := r.monitor.GetUnifiedReadState()
	if readState == nil {
		return
	}

	learningWindow := 7 * 24 * time.Hour // Learn from 7 days of data
	var learned int

	// Use SourceID() for all ID lookups — metrics history is keyed by legacy
	// source IDs (e.g. "node/pve", "qemu/100"), not unified registry IDs.

	for _, node := range readState.Nodes() {
		id := node.SourceID()
		if id == "" {
			continue
		}
		for _, metric := range []string{"cpu", "memory"} {
			points := metricsHistory.GetNodeMetrics(id, metric, learningWindow)
			if len(points) > 0 {
				baselinePoints := make([]ai.BaselineMetricPoint, len(points))
				for i, p := range points {
					baselinePoints[i] = ai.BaselineMetricPoint{Value: p.Value, Timestamp: p.Timestamp}
				}
				if err := store.Learn(id, "node", metric, baselinePoints); err == nil {
					learned++
				}
			}
		}
	}

	for _, vm := range readState.VMs() {
		if vm.Template() {
			continue
		}
		id := vm.SourceID()
		if id == "" {
			continue
		}
		for _, metric := range []string{"cpu", "memory", "disk"} {
			points := metricsHistory.GetGuestMetrics(id, metric, learningWindow)
			if len(points) > 0 {
				baselinePoints := make([]ai.BaselineMetricPoint, len(points))
				for i, p := range points {
					baselinePoints[i] = ai.BaselineMetricPoint{Value: p.Value, Timestamp: p.Timestamp}
				}
				if err := store.Learn(id, "vm", metric, baselinePoints); err == nil {
					learned++
				}
			}
		}
	}

	for _, ct := range readState.Containers() {
		if ct.Template() {
			continue
		}
		id := ct.SourceID()
		if id == "" {
			continue
		}
		for _, metric := range []string{"cpu", "memory", "disk"} {
			points := metricsHistory.GetGuestMetrics(id, metric, learningWindow)
			if len(points) > 0 {
				baselinePoints := make([]ai.BaselineMetricPoint, len(points))
				for i, p := range points {
					baselinePoints[i] = ai.BaselineMetricPoint{Value: p.Value, Timestamp: p.Timestamp}
				}
				if err := store.Learn(id, "container", metric, baselinePoints); err == nil {
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
// GetLicenseHandlers returns the license handlers for external callers (e.g. telemetry).
func (r *Router) GetLicenseHandlers() *LicenseHandlers {
	return r.licenseHandlers
}

func (r *Router) SetLicenseRuntimeIdentity(identity runtimeIdentityModel) {
	if r == nil || r.licenseHandlers == nil {
		return
	}
	r.licenseHandlers.SetRuntimeIdentity(identity)
}

// StopGrantRefresh stops all grant refresh and revocation poll loops across all tenants.
func (r *Router) StopGrantRefresh() {
	if r.licenseHandlers != nil {
		r.licenseHandlers.StopAllBackgroundLoops()
	}
}

// SetTelemetryToggleFunc wires a callback that is invoked when the user
// toggles telemetry on or off at runtime via system settings.
func (r *Router) SetTelemetryToggleFunc(fn func(enabled bool)) {
	if r.systemSettingsHandler != nil {
		r.systemSettingsHandler.SetTelemetryToggleFunc(fn)
	}
}

// SetTelemetryPreviewFunc wires the exact runtime telemetry preview callback
// into the system settings handler.
func (r *Router) SetTelemetryPreviewFunc(fn func() (telemetry.Ping, error)) {
	if r.systemSettingsHandler != nil {
		r.systemSettingsHandler.SetTelemetryPreviewFunc(fn)
	}
}

// SetTelemetryResetFunc wires the telemetry install-ID reset callback into the
// system settings handler.
func (r *Router) SetTelemetryResetFunc(fn func() (telemetry.Ping, error)) {
	if r.systemSettingsHandler != nil {
		r.systemSettingsHandler.SetTelemetryResetFunc(fn)
	}
}

func (r *Router) GetAlertTriggeredAnalyzer() aicontracts.AlertAnalyzer {
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

	// 2. Get the Monitor (The Trigger)
	if r.monitor == nil {
		log.Debug().Msg("Monitor not available for AI alert callback")
		return
	}

	if ai.BackgroundAutomationDisabledForDev() {
		log.Info().
			Str("env", ai.DevDisableBackgroundAIEnv).
			Msg("Pulse dev background AI disabled; alert-triggered AI analyzer not wired")
		return
	}

	// 3. Connect alert-fired events to the dedicated alert-triggered analyzer.
	// Patrol's event-triggered runs are owned by the canonical alert bridge /
	// trigger-manager path, so this callback should not enqueue Patrol directly.
	r.monitor.SetAlertTriggeredAICallback(func(alert *alerts.Alert) {
		if analyzer := r.GetAlertTriggeredAnalyzer(); analyzer != nil {
			log.Info().Str("alert_identifier", alert.ID).Msg("Alert fired leading to alert-triggered analysis")
			analyzer.OnAlertFired(alert)
		}
	})

	log.Info().Msg("Alert-triggered AI analyzer wired to monitor")
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

	// Prefer explicit canonical resource type from alert metadata.
	if alert.Metadata != nil {
		if raw, ok := alert.Metadata["resourceType"].(string); ok {
			switch canonicalAlertResourceTypeToken(raw) {
			case "vm":
				return "vm"
			case "system-container", "oci-container":
				return "system-container"
			case "app-container", "docker-host":
				return "app-container"
			case "node":
				return "node"
			case "storage", "disk":
				return "storage"
			case "pbs":
				return "pbs"
			case "k8s", "k8s-node", "k8s-cluster":
				return "k8s"
			}
		}
	}

	// Infer from resource ID patterns.
	resourceID := strings.ToLower(strings.TrimSpace(alert.ResourceID))
	switch {
	case strings.Contains(resourceID, "/node/"),
		strings.HasPrefix(resourceID, "node/"),
		strings.HasPrefix(resourceID, "node:"):
		return "node"
	case strings.Contains(resourceID, "/qemu/"),
		strings.HasPrefix(resourceID, "vm:"),
		strings.HasPrefix(resourceID, "vm/"):
		return "vm"
	case strings.Contains(resourceID, "/lxc/"),
		strings.HasPrefix(resourceID, "system-container:"),
		strings.HasPrefix(resourceID, "system-container/"),
		strings.HasPrefix(resourceID, "oci-container:"),
		strings.HasPrefix(resourceID, "oci-container/"):
		return "system-container"
	case strings.Contains(resourceID, "docker:"),
		strings.HasPrefix(resourceID, "app-container:"),
		strings.HasPrefix(resourceID, "app-container/"),
		strings.HasPrefix(resourceID, "docker-host:"),
		strings.HasPrefix(resourceID, "docker-host/"),
		strings.Contains(resourceID, "docker"):
		return "app-container"
	case strings.HasPrefix(resourceID, "storage/"), strings.Contains(resourceID, "storage"):
		return "storage"
	case strings.HasPrefix(resourceID, "pbs"), strings.Contains(resourceID, "/pbs/"):
		return "pbs"
	case strings.Contains(resourceID, "k8s"), strings.Contains(resourceID, "kubernetes"):
		return "k8s"
	}

	// Final fallback by alert type for broad non-workload classes.
	alertType := strings.ToLower(strings.TrimSpace(alert.Type))
	switch {
	case strings.HasPrefix(alertType, "node"):
		return "node"
	case strings.Contains(alertType, "storage"):
		return "storage"
	case strings.Contains(alertType, "pbs"):
		return "pbs"
	case strings.Contains(alertType, "kubernetes"), strings.Contains(alertType, "k8s"):
		return "k8s"
	default:
		return "vm"
	}
}

func canonicalAlertResourceTypeToken(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" || unifiedresources.IsUnsupportedLegacyResourceTypeAlias(normalized) {
		return ""
	}
	switch normalized {
	case "vm", "system-container", "oci-container", "app-container", "node", "storage", "disk", "agent", "docker-host", "pbs", "pmg", "k8s", "k8s-node", "k8s-cluster":
		return normalized
	default:
		return ""
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

		// Update webhook allowed private CIDRs in notification managers.
		// The setting is instance-wide, so every tenant monitor's manager
		// must observe it, not just the default org's.
		if r.monitor != nil {
			if nm := r.monitor.GetNotificationManager(); nm != nil {
				if err := nm.UpdateAllowedPrivateCIDRs(systemSettings.WebhookAllowedPrivateCIDRs); err != nil {
					log.Error().Err(err).Msg("Failed to update webhook allowed private CIDRs during settings reload")
				}
			}
		}
		if r.mtMonitor != nil {
			r.mtMonitor.ForEachMonitor(func(m *monitoring.Monitor) {
				if nm := m.GetNotificationManager(); nm != nil {
					if err := nm.UpdateAllowedPrivateCIDRs(systemSettings.WebhookAllowedPrivateCIDRs); err != nil {
						log.Error().Err(err).Msg("Failed to update webhook allowed private CIDRs on tenant monitor during settings reload")
					}
				}
			})
		}
	} else {
		if err != nil {
			// Failing closed (embedding off) is deliberate, but the error
			// must still be observable — a persistent read failure otherwise
			// looks identical to embedding being switched off on purpose.
			log.Warn().Err(err).Msg("Failed to load system settings during reload; using safe defaults")
		}
		// On error, use safe defaults
		r.cachedAllowEmbedding = false
		r.cachedAllowedOrigins = ""
	}
}

// ReloadSystemSettings re-applies persisted system settings to the router
// and all notification managers. This must be called after a monitor reload
// (which recreates the notification manager) to restore instance-wide
// settings like the webhook private CIDR allowlist that live inside the
// notification manager's runtime state.
func (r *Router) ReloadSystemSettings() {
	r.reloadSystemSettings()
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
		// Prevent caching of API responses that may contain sensitive data
		// (auth state, infrastructure topology, tokens, settings).
		// Static assets and HTML have their own cache headers set elsewhere.
		if strings.HasPrefix(req.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}

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
				// No Origin header — same-origin or non-browser (e.g. curl).
				// CORS headers are only meaningful when a browser sends an Origin,
				// so skip setting any CORS headers for these requests.
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
				// Cache preflight results for 1 hour (only meaningful on OPTIONS).
				if req.Method == "OPTIONS" {
					w.Header().Set("Access-Control-Max-Age", "3600")
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

		if needsAuth {
			// Normal authentication check
			// Normalize path to handle double slashes (e.g., //download -> /download)
			// This prevents auth bypass failures when URLs have trailing slashes
			normalizedPath := path.Clean(req.URL.Path)

			// Skip auth for certain public endpoints and static assets
			publicPaths := []string{
				"/api/health",
				"/api/security/status",
				"/api/security/recovery",
				"/api/security/validate-bootstrap-token",
				"/api/security/quick-setup", // Handler does its own auth (bootstrap token or session)
				"/api/version",
				"/api/login",                      // Add login endpoint as public
				"/api/public/signup",              // Hosted mode: public signup
				"/api/public/magic-link/request",  // Hosted mode: request magic link
				"/api/public/magic-link/verify",   // Hosted mode: verify magic link
				"/api/cloud/handoff/exchange",     // Hosted mode: control-plane workspace handoff (token-authenticated)
				"/api/webhooks/stripe",            // Hosted mode: Stripe webhook (signature verification is auth)
				"/install.sh",                     // Unified agent installer
				"/install.ps1",                    // Unified agent Windows installer
				"/download/pulse-agent",           // Unified agent binary
				"/download/pulse-agent-helper",    // Typed privileged helper binary
				"/download/pulse-agent-runner",    // Typed action runner binary
				"/api/agent/capabilities",         // Agent-paradigm discovery manifest; underlying capabilities keep their own auth scopes
				"/api/agent/version",              // Agent update checks need to work before auth
				"/api/agent/ws",                   // Agent WebSocket has its own auth via registration
				"/api/server/info",                // Server info for installer script
				"/api/ai/oauth/callback",          // OAuth callback from Anthropic for Claude subscription auth
				"/auth/cloud-handoff",             // Cloud control plane handoff (token-authenticated)
				"/auth/license-purchase-activate", // Self-hosted checkout return (session-authenticated via commercial backend)
			}

			// Also allow static assets without auth (JS, CSS, etc)
			// These MUST be accessible for the login page to work
			// Frontend routes (non-API, non-download) should also be public
			// because authentication is handled by the frontend after page load
			isFrontendRoute := !strings.HasPrefix(req.URL.Path, "/api/") &&
				!strings.HasPrefix(req.URL.Path, "/ws") &&
				!strings.HasPrefix(req.URL.Path, "/download/") &&
				req.URL.Path != "/simple-stats" &&
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

			// Per-provider SSO OIDC routes are public (login initiation + callback).
			// The legacy v5 single-provider paths /api/oidc/login and
			// /api/oidc/callback stay public too: an upgraded provider keeps its v5
			// redirect URL, so the IdP redirects the browser back to the 3-part path
			// carrying only the code/state, with no session cookie or API token yet.
			if strings.HasPrefix(normalizedPath, "/api/oidc/") {
				oidcParts := strings.Split(strings.TrimPrefix(normalizedPath, "/"), "/")
				if len(oidcParts) >= 4 && (oidcParts[3] == "login" || oidcParts[3] == "callback") {
					isPublic = true
				} else if len(oidcParts) == 3 && (oidcParts[2] == "login" || oidcParts[2] == "callback") {
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

			// Special case: setup-script should be public because it authenticates with setup tokens.
			if normalizedPath == "/api/setup-script" {
				// The script itself prompts for a setup token.
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
			if normalizedPath == "/api/config/export" || normalizedPath == "/api/config/import" {
				// These handlers apply stricter route-local auth and public-network checks.
				isPublic = true
			}

			// Auto-register endpoint needs to be public (validates tokens internally)
			// BUT the tokens must be generated by authenticated users via setup-script-url
			if normalizedPath == "/api/auto-register" {
				isPublic = true
			}
			if normalizedPath == "/api/auto-unregister" {
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
			if needsAuth && !isPublic {
				authWriter := &responseCapture{ResponseWriter: w}
				if !checkAuth(r.config, authWriter, req, false) {
					if !authWriter.wrote {
						// Never send WWW-Authenticate - use custom login page.
						writeAuthenticationRequired(w, req)
					}
					log.Warn().
						Str("ip", req.RemoteAddr).
						Str("path", req.URL.Path).
						Msg("Unauthorized access attempt")
					return
				}
			}
		}
		// Check CSRF for state-changing requests.
		// CSRF is only needed when using session-based auth.
		skipCSRF := false
		// Quick setup can run before auth exists. Keep bootstrap/recovery flows usable
		// without a prior session+CSRF pair, but enforce CSRF once auth is configured.
		config.Mu.RLock()
		authConfigured := (r.config.AuthUser != "" && r.config.AuthPass != "") ||
			r.config.HasAPITokens() ||
			r.config.ProxyAuthSecret != ""
		config.Mu.RUnlock()
		if !authConfigured {
			ssoCfg := r.ensureSSOConfig()
			if ssoCfg != nil && ssoCfg.HasEnabledProviders() {
				authConfigured = true
			}
		}
		validRecoveryToken := false
		if recoveryToken := strings.TrimSpace(req.Header.Get("X-Recovery-Token")); recoveryToken != "" {
			validRecoveryToken = GetRecoveryTokenStore().IsRecoveryTokenValidConstantTime(recoveryToken, clientIP)
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
		// Skip CSRF for self-hosted checkout activation return (POST from Pulse Account, no prior session required).
		if req.URL.Path == "/auth/license-purchase-activate" {
			skipCSRF = true
		}
		// Skip CSRF for control-plane workspace handoff exchange (POST with signed handoff token).
		if req.URL.Path == "/api/cloud/handoff/exchange" {
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
			sessionCookie, err := readSessionCookie(req)
			if err == nil && sessionCookie.Value != "" {
				// Check if CSRF cookie exists
				_, csrfErr := req.Cookie(CookieNameCSRF)
				if csrfErr != nil {
					// Session exists but no CSRF cookie - issue one
					csrfToken := generateCSRFToken(sessionCookie.Value)
					getBrowserCookiePolicy(req).setClientReadable(w, &http.Cookie{
						Name:   CookieNameCSRF,
						Value:  csrfToken,
						Path:   "/",
						MaxAge: 86400,
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
			strings.HasPrefix(req.URL.Path, "/download/") ||
			strings.HasPrefix(req.URL.Path, "/auth/") ||
			strings.HasPrefix(req.URL.Path, "/debug/pprof") ||
			req.URL.Path == "/simple-stats" ||
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
	}), allowEmbedding, allowedEmbedOrigins, utils.GetenvTrim("FRONTEND_DEV_SERVER") != "").ServeHTTP(w, req)
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

	origin, ok := resolveRequestOrigin(req)
	if !ok {
		return
	}
	if isLoopbackHost(origin.hostname) {
		return
	}

	normalizedCandidate := origin.baseURL()

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
	r.config.PublicURLAutoDetected = true
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
	if err := r.syncSAMLPublicURL(); err != nil {
		log.Error().Err(err).Str("publicURL", normalizedCandidate).Msg("Failed to synchronize SAML public URL from request")
	}
}

func firstForwardedValue(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Split(header, ",")
	return strings.TrimSpace(parts[0])
}

type resolvedRequestOrigin struct {
	scheme    string
	authority string
	hostname  string
}

func (o resolvedRequestOrigin) baseURL() string {
	if o.scheme == "" || o.authority == "" {
		return ""
	}
	return o.scheme + "://" + o.authority
}

type validatedRequestHost struct {
	authority string
	hostname  string
	hasPort   bool
}

// resolveRequestOrigin is the single trust boundary for request-derived
// absolute URLs. Direct Host values are always parsed as a strict HTTP
// authority. Forwarded host, scheme, and port values are considered only when
// the immediate peer is configured in PULSE_TRUSTED_PROXY_CIDRS; malformed
// forwarded values are ignored independently and cannot poison the direct
// request fallback.
func resolveRequestOrigin(req *http.Request) (resolvedRequestOrigin, bool) {
	if req == nil {
		return resolvedRequestOrigin{}, false
	}

	trustedProxy := isTrustedProxyIP(extractRemoteIP(req.RemoteAddr))
	var host validatedRequestHost
	var ok bool
	if trustedProxy {
		host, ok = validateRequestHost(firstForwardedValue(req.Header.Get("X-Forwarded-Host")))
	}
	if !ok {
		host, ok = validateRequestHost(req.Host)
	}
	if !ok {
		return resolvedRequestOrigin{}, false
	}

	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	if trustedProxy {
		if forwardedScheme, valid := validateForwardedScheme(firstForwardedValue(req.Header.Get("X-Forwarded-Proto"))); valid {
			scheme = forwardedScheme
		} else if forwardedScheme, valid := validateForwardedScheme(firstForwardedValue(req.Header.Get("X-Forwarded-Scheme"))); valid {
			scheme = forwardedScheme
		}
	}

	if trustedProxy && !host.hasPort {
		if forwardedPort := firstForwardedValue(req.Header.Get("X-Forwarded-Port")); shouldAppendForwardedPort(forwardedPort, scheme) {
			host.authority = net.JoinHostPort(host.hostname, forwardedPort)
			host.hasPort = true
		}
	}

	return resolvedRequestOrigin{
		scheme:    scheme,
		authority: host.authority,
		hostname:  host.hostname,
	}, true
}

func validateForwardedScheme(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "http":
		return "http", true
	case "https":
		return "https", true
	default:
		return "", false
	}
}

// requestOriginBaseURL derives the scheme://host base URL the admin's browser
// used to reach Pulse. It is the fallback for building absolute URLs (OIDC
// callbacks, SAML metadata/ACS) when no public URL has been configured — the
// request necessarily arrived over a reachable address, unlike a hardcoded
// localhost guess. Returns "" when the host cannot be resolved.
func requestOriginBaseURL(req *http.Request) string {
	origin, ok := resolveRequestOrigin(req)
	if !ok {
		return ""
	}
	return origin.baseURL()
}

func sanitizeForwardedHost(raw string) (string, string) {
	host, ok := validateRequestHost(raw)
	if !ok {
		return "", ""
	}
	return host.authority, host.hostname
}

func validateRequestHost(raw string) (validatedRequestHost, bool) {
	if raw == "" || strings.TrimSpace(raw) != raw {
		return validatedRequestHost{}, false
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] <= 0x20 || raw[i] >= 0x7f {
			return validatedRequestHost{}, false
		}
	}
	if strings.ContainsAny(raw, "/\\?#@%") {
		return validatedRequestHost{}, false
	}

	authority := raw
	hostname := raw
	hasPort := false

	if strings.HasPrefix(raw, "[") {
		closing := strings.IndexByte(raw, ']')
		if closing <= 1 {
			return validatedRequestHost{}, false
		}
		hostname = raw[1:closing]
		if ip := net.ParseIP(hostname); ip == nil || ip.To4() != nil {
			return validatedRequestHost{}, false
		}
		suffix := raw[closing+1:]
		switch {
		case suffix == "":
			authority = "[" + hostname + "]"
		case strings.HasPrefix(suffix, ":") && validRequestPort(suffix[1:]):
			hasPort = true
			authority = net.JoinHostPort(hostname, suffix[1:])
		default:
			return validatedRequestHost{}, false
		}
	} else if strings.Count(raw, ":") > 1 {
		if ip := net.ParseIP(raw); ip == nil || ip.To4() != nil {
			return validatedRequestHost{}, false
		}
		hostname = raw
		authority = "[" + raw + "]"
	} else if strings.Contains(raw, ":") {
		hostPart, port, err := net.SplitHostPort(raw)
		if err != nil || !validRequestPort(port) {
			return validatedRequestHost{}, false
		}
		hostname = hostPart
		hasPort = true
	}

	if ip := net.ParseIP(hostname); ip == nil && !validRequestHostname(hostname) {
		return validatedRequestHost{}, false
	}

	return validatedRequestHost{
		authority: authority,
		hostname:  hostname,
		hasPort:   hasPort,
	}, true
}

func validRequestHostname(hostname string) bool {
	name := strings.TrimSuffix(hostname, ".")
	if name == "" || len(name) > 253 {
		return false
	}
	if allNumericAndDots := strings.Trim(name, "0123456789.") == ""; allNumericAndDots {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}
	return true
}

func validRequestPort(port string) bool {
	if port == "" {
		return false
	}
	for i := 0; i < len(port); i++ {
		if port[i] < '0' || port[i] > '9' {
			return false
		}
	}
	value, err := strconv.Atoi(port)
	return err == nil && value >= 1 && value <= 65535
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
	if !validRequestPort(port) {
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

	// Session (Browser): allow capture only for an admin session. This prevents
	// low-privilege session users from poisoning public URL auto-detection.
	// The admin test is sessionUserCarriesAdminPrivileges, the same one the
	// settings routes apply, rather than a local comparison against
	// cfg.AuthUser: that comparison cannot match on an instance whose only
	// administrators are SSO principals, so it locked those operators out.
	if cookie, err := readSessionCookie(req); err == nil && cookie.Value != "" {
		if ValidateSession(cookie.Value) {
			username := strings.TrimSpace(GetSessionUsername(cookie.Value))
			if sessionUserCarriesAdminPrivileges(cfg, username) {
				return true
			}
		}
	}

	// Basic Auth: Trusted (Admin)
	if cfg.AuthUser != "" && cfg.AuthPass != "" {
		const prefix = "Basic "
		if authHeader := req.Header.Get("Authorization"); strings.HasPrefix(authHeader, prefix) {
			if decoded, err := base64.StdEncoding.DecodeString(authHeader[len(prefix):]); err == nil {
				if parts := strings.SplitN(string(decoded), ":", 2); len(parts) == 2 {
					if constantTimeStringEqual(parts[0], cfg.AuthUser) && internalauth.CheckPasswordHash(parts[1], cfg.AuthPass) {
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
	// The scheduler is healthy when the monitor is running. Individual
	// tasks that are dead-lettered (e.g. an unreachable node) are normal
	// operational events tracked via the scheduler health endpoint, not a
	// sign that the scheduler itself is broken. Gating the health check
	// on DeadLetterCount()==0 causes false 503s whenever any single node
	// goes down.
	schedulerHealthy := monitorHealthy

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

	response := EmptyHealthResponse()
	response.Status = status
	response.Timestamp = time.Now().Unix()
	response.Uptime = uptimeSeconds
	response.ProxyInstallScriptAvailable = true
	response.DevModeSSH = os.Getenv("PULSE_DEV_ALLOW_CONTAINER_SSH") == "true"
	response.Dependencies = map[string]bool{
		"monitor":   monitorHealthy,
		"scheduler": schedulerHealthy,
		"websocket": r.wsHub != nil,
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

	// Hold session callers to the same rule the proxy branch above applies.
	// Knowing the current password is the real gate on the change itself, so
	// this is not an escalation, but without it any authenticated non-admin
	// session could probe the admin password one guess at a time and read the
	// answer from the 401. ensureAdminSession is a no-op when the request
	// carries no session cookie, so the Basic Auth flow is unaffected.
	if !ensureAdminSession(r.config, w, req) {
		log.Warn().
			Str("ip", req.RemoteAddr).
			Str("path", req.URL.Path).
			Msg("Non-admin session attempted to change password")
		return
	}

	// Parse request
	var changeReq struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}

	if err := decodeSecurityRequestBody(w, req, &changeReq); err != nil {
		writeErrorResponse(w, securityRequestErrorStatus(err), "invalid_request",
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
					if constantTimeStringEqual(parts[0], r.config.AuthUser) {
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
		envPath := resolveAuthEnvPath(r.config.ConfigPath)

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
		}

		// Write the updated .env file
		envPath, err = writeAuthEnvFile(r.config.ConfigPath, r.config.DataPath, []byte(envContent))
		if err != nil {
			log.Error().Err(err).Msg("Failed to write .env file")
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
		envPath := resolveAuthEnvPath(r.config.ConfigPath)

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
		}

		// Try to write the .env file
		envPath, err = writeAuthEnvFile(r.config.ConfigPath, r.config.DataPath, []byte(envContent))
		if err != nil {
			log.Error().Err(err).Msg("Failed to write .env file")
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

	r.clearSession(w, req)

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
	// Invalidate any pre-existing session to prevent session fixation attacks.
	InvalidateOldSessionFromRequest(req)

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
	cookiePolicy := getBrowserCookiePolicy(req)

	cookiePolicy.setHTTPOnly(w, &http.Cookie{
		Name:     sessionCookieName(cookiePolicy.secure),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})

	cookiePolicy.setClientReadable(w, &http.Cookie{
		Name:   CookieNameCSRF,
		Value:  csrfToken,
		Path:   "/",
		MaxAge: 86400,
	})

	return nil
}

func (r *Router) establishRecoverySession(w http.ResponseWriter, req *http.Request, username string) error {
	InvalidateOldSessionFromRequest(req)

	token := generateSessionToken()
	if token == "" {
		return fmt.Errorf("failed to generate recovery session token")
	}

	userAgent := req.Header.Get("User-Agent")
	clientIP := normalizeRecoveryBindingIP(GetClientIP(req))
	if clientIP == "" {
		return fmt.Errorf("recovery session requires a client IP")
	}

	GetSessionStore().CreateRecoverySession(token, 24*time.Hour, userAgent, clientIP, username)

	csrfToken := generateCSRFToken(token)
	cookiePolicy := getBrowserCookiePolicy(req)

	cookiePolicy.setHTTPOnly(w, &http.Cookie{
		Name:     sessionCookieName(cookiePolicy.secure),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})

	cookiePolicy.setClientReadable(w, &http.Cookie{
		Name:   CookieNameCSRF,
		Value:  csrfToken,
		Path:   "/",
		MaxAge: 86400,
	})

	return nil
}

// establishOIDCSession creates a session with OIDC token information for refresh token support.
func (r *Router) establishOIDCSession(w http.ResponseWriter, req *http.Request, username, displayUsername string, oidcTokens *OIDCTokenInfo) error {
	// Invalidate any pre-existing session to prevent session fixation attacks.
	InvalidateOldSessionFromRequest(req)

	token := generateSessionToken()
	if token == "" {
		return fmt.Errorf("failed to generate session token")
	}

	userAgent := req.Header.Get("User-Agent")
	clientIP := GetClientIP(req)

	// Create session with OIDC tokens (including principal and display label for restart survival)
	GetSessionStore().CreateOIDCSessionWithDisplayName(token, 24*time.Hour, userAgent, clientIP, username, displayUsername, oidcTokens)

	if username != "" {
		TrackUserSession(username, token)
	}

	csrfToken := generateCSRFToken(token)
	cookiePolicy := getBrowserCookiePolicy(req)

	cookiePolicy.setHTTPOnly(w, &http.Cookie{
		Name:     sessionCookieName(cookiePolicy.secure),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})

	cookiePolicy.setClientReadable(w, &http.Cookie{
		Name:   CookieNameCSRF,
		Value:  csrfToken,
		Path:   "/",
		MaxAge: 86400,
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

	if err := decodeSecurityRequestBody(w, req, &loginReq); err != nil {
		writeErrorResponse(w, securityRequestErrorStatus(err), "invalid_request",
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

	// Verify credentials (or accept any credentials when admin bypass is enabled)
	config.Mu.RLock()
	cfgAuthUser := r.config.AuthUser
	cfgAuthPass := r.config.AuthPass
	config.Mu.RUnlock()
	credentialsValid := constantTimeStringEqual(loginReq.Username, cfgAuthUser) && auth.CheckPasswordHash(loginReq.Password, cfgAuthPass)
	effectiveUsername := loginReq.Username
	if adminBypassEnabled() {
		credentialsValid = true
		effectiveUsername = "admin"
	}
	if credentialsValid {
		// Clear failed login attempts
		ClearFailedLogins(loginReq.Username)
		ClearFailedLogins(clientIP)

		// Invalidate any pre-existing session to prevent session fixation attacks.
		InvalidateOldSessionFromRequest(req)

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
		GetSessionStore().CreateSession(token, sessionDuration, userAgent, clientIP, effectiveUsername)

		// Track session for user (in-memory for fast lookups)
		TrackUserSession(effectiveUsername, token)

		// Generate CSRF token
		csrfToken := generateCSRFToken(token)

		// Get appropriate cookie settings based on proxy detection
		cookiePolicy := getBrowserCookiePolicy(req)

		// Set cookie MaxAge to match session duration
		cookieMaxAge := int(sessionDuration.Seconds())

		// Set session cookie
		cookiePolicy.setHTTPOnly(w, &http.Cookie{
			Name:     sessionCookieName(cookiePolicy.secure),
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   cookieMaxAge,
		})

		// Set CSRF cookie (not HttpOnly so JS can read it)
		cookiePolicy.setClientReadable(w, &http.Cookie{
			Name:   CookieNameCSRF,
			Value:  csrfToken,
			Path:   "/",
			MaxAge: cookieMaxAge,
		})

		// Audit log successful login
		LogAuditEventForTenant(GetOrgID(req.Context()), "login", effectiveUsername, clientIP, req.URL.Path, true, "Successful login")

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
		if !ensureSettingsWriteScope(r.config, w, req) {
			return
		}

		// Parse request
		var resetReq struct {
			Identifier string `json:"identifier"` // Can be username or IP
		}

		if err := decodeSecurityRequestBody(w, req, &resetReq); err != nil {
			writeErrorResponse(w, securityRequestErrorStatus(err), "invalid_request",
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
	authWriter := &responseCapture{ResponseWriter: w}
	if !checkAuth(r.config, authWriter, req, false) {
		if !authWriter.wrote {
			writeErrorResponse(w, http.StatusUnauthorized, "unauthorized",
				"Authentication required", nil)
		}
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

	// Dedupe concurrent callers per tenant: only one goroutine builds +
	// marshals the snapshot; the rest receive the shared byte slice.
	orgID := GetOrgID(req.Context())
	if orgID == "" {
		orgID = "default"
	}
	v, err, _ := r.stateComputeGroup.Do(orgID, func() (any, error) {
		frontendState := monitor.BuildFrontendState()
		return json.Marshal(frontendState)
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to encode state response")
		writeErrorResponse(w, http.StatusInternalServerError, "encoding_error",
			"Failed to encode state data", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(v.([]byte)); err != nil {
		log.Error().Err(err).Msg("Failed to write state response")
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
		Version:                  versionInfo.Version,
		BuildTime:                versionInfo.Build,
		Build:                    versionInfo.Build,
		GoVersion:                runtime.Version(),
		Runtime:                  versionInfo.Runtime,
		Channel:                  versionInfo.Channel,
		IsDocker:                 versionInfo.IsDocker,
		IsSourceBuild:            versionInfo.IsSourceBuild,
		IsDevelopment:            versionInfo.IsDevelopment,
		DeploymentType:           versionInfo.DeploymentType,
		AgentUpdateTargetVersion: currentAgentTargetVersion(),
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

	version := currentAgentTargetVersion()
	if version == "" {
		version = "dev"
	}

	response := AgentVersionResponse{
		Version: version,
	}

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
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
	if monitor == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "tenant_unavailable", "Tenant monitor is not available", nil)
		return
	}
	// Find the storage by ID
	var storageDetail *models.Storage
	for _, storage := range monitor.StorageSnapshot() {
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

func parseMetricsHistoryDuration(timeRange string) time.Duration {
	normalizedRange := strings.ToLower(strings.TrimSpace(timeRange))
	switch normalizedRange {
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "24h", "1d", "":
		return 24 * time.Hour
	}

	if strings.HasSuffix(normalizedRange, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(normalizedRange, "d"))
		if err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour
		}
	}

	duration, err := time.ParseDuration(normalizedRange)
	if err != nil || duration <= 0 {
		return 24 * time.Hour
	}
	return duration
}

// handleMetricsHistory returns historical metrics from the persistent SQLite store
// Query params:
//   - resourceType: "node", "agent", "vm", "system-container", "oci-container", "app-container",
//     "docker-host", "k8s", "storage", or "disk" (required)
//   - resourceId: the resource identifier (required)
//   - metric: "cpu", "memory", "disk", etc. (optional, omit for all metrics)
//   - range: time range like "1h", "24h", "7d", "14d", "30d", "90d" (optional, default "24h")
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
	resourceTypeInput := strings.ToLower(strings.TrimSpace(query.Get("resourceType")))
	resourceID := query.Get("resourceId")
	metricType := query.Get("metric")
	timeRange := query.Get("range")

	if resourceTypeInput == "" || resourceID == "" {
		http.Error(w, "resourceType and resourceId are required", http.StatusBadRequest)
		return
	}
	// Normalize and validate query aliases to runtime/store resource types.
	responseResourceType, runtimeResourceType, storeResourceTypes, err := normalizeMetricsHistoryResourceType(resourceTypeInput)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resourceID = canonicalizeMetricsHistoryResourceID(runtimeResourceType, resourceID)

	duration := parseMetricsHistoryDuration(timeRange)
	var stepSecs int64 = 0 // Default to no downsampling (use tier resolution)

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

	// Enforce tier-aware history limits (e.g. Free=7d, Relay=14d, Pro=90d).
	{
		maxHistDays := freeHistoryDaysDefault
		if r.licenseHandlers != nil {
			if licSvc := r.licenseHandlers.Service(req.Context()); licSvc != nil {
				status := licSvc.Status()
				if status.Valid {
					maxHistDays = tierHistoryDaysFromLicensing(status.Tier)
				}
				// When !status.Valid, maxHistDays stays at free-tier default.
			}
		}
		maxDuration := time.Duration(maxHistDays) * 24 * time.Hour
		if duration > maxDuration {
			WriteLicenseRequired(w, featureLongTermMetricsValue, "Extended metrics history requires a higher-tier Pulse license")
			return
		}
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
	if runtimeResourceType == "storage" && metricType == "disk" {
		queryMetric = "usage"
	}

	// Allow in-memory fallback for any requested range when the persistent store is empty.
	// The in-memory history enforces its own retention limits, so it will naturally return
	// whatever data is available (better than showing "Collecting data..." indefinitely).
	fallbackAllowed := true
	historyMaxPoints := chartapi.ParseWorkloadMaxPoints(query.Get("maxPoints"))
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
	// Most requests are served from the metrics store. Keep the fallback/read-state
	// snapshots lazy so store-backed history does not pay an O(fleet-size) copy cost.
	type storageMetricFallback struct {
		ID         string
		ResourceID string
		Used       int64
		Total      int64
		Percent    float64
	}
	var (
		fallbackStateOnce sync.Once
		vms               []models.VM
		containers        []models.Container
		nodes             []models.Node
		dockerHosts       []models.DockerHost
		hosts             []models.Host

		legacyStorageFallbackOnce sync.Once
		storagePools              []models.Storage

		diskFallbackOnce sync.Once
		physicalDisks    []unifiedresources.Resource

		storageFallbackOnce sync.Once
		unifiedStorage      []storageMetricFallback
	)

	loadFallbackState := func() {
		fallbackStateOnce.Do(func() {
			vms = monitor.VMsSnapshot()
			containers = monitor.ContainersSnapshot()
			nodes = monitor.NodesSnapshot()
			dockerHosts = monitor.DockerHostsSnapshot()
			hosts = monitor.HostsSnapshot()
		})
	}

	loadLegacyStorage := func() {
		legacyStorageFallbackOnce.Do(func() {
			storagePools = monitor.StorageSnapshot()
		})
	}

	loadPhysicalDisks := func() {
		diskFallbackOnce.Do(func() {
			for _, resource := range monitor.GetUnifiedResources() {
				if resource.Type == unifiedresources.ResourceTypePhysicalDisk && resource.PhysicalDisk != nil {
					physicalDisks = append(physicalDisks, resource)
				}
			}
		})
	}

	loadUnifiedStorage := func() {
		storageFallbackOnce.Do(func() {
			readState := monitor.GetUnifiedReadStateOrSnapshot()
			seen := make(map[string]struct{})
			if readState != nil {
				for _, pool := range readState.StoragePools() {
					if pool == nil {
						continue
					}
					unifiedStorage = append(unifiedStorage, storageMetricFallback{
						ID:         pool.ID(),
						ResourceID: pool.SourceID(),
						Used:       pool.DiskUsed(),
						Total:      pool.DiskTotal(),
						Percent:    pool.DiskPercent(),
					})
					seen[pool.ID()] = struct{}{}
				}
			}
			resolver, _ := readState.(monitoring.MetricsTargetResourceStore)
			for _, resource := range monitor.GetUnifiedResources() {
				if (resource.Type == unifiedresources.ResourceTypeStorage || resource.Type == unifiedresources.ResourceTypeCeph) &&
					resource.Metrics != nil && resource.Metrics.Disk != nil {
					if _, ok := seen[resource.ID]; ok {
						continue
					}
					if resource.MetricsTarget == nil && resolver != nil {
						resource.MetricsTarget = resolver.MetricsTargetForResource(resource.ID)
					}
					targetID := ""
					if resource.MetricsTarget != nil {
						targetID = strings.TrimSpace(resource.MetricsTarget.ResourceID)
					}
					disk := resource.Metrics.Disk
					fallback := storageMetricFallback{
						ID:         resource.ID,
						ResourceID: targetID,
						Percent:    disk.Percent,
					}
					if disk.Used != nil {
						fallback.Used = *disk.Used
					}
					if disk.Total != nil {
						fallback.Total = *disk.Total
					}
					unifiedStorage = append(unifiedStorage, fallback)
				}
			}
		})
	}

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
		loadFallbackState()
		for i := range vms {
			if vms[i].ID == id {
				return &vms[i]
			}
		}
		if instance, node, vmID, ok := parseGuestID(id); ok {
			for i := range vms {
				vm := &vms[i]
				if vm.VMID == vmID && vm.Node == node && vm.Instance == instance {
					return vm
				}
			}
		}
		return nil
	}

	findContainer := func(id string) *models.Container {
		loadFallbackState()
		for i := range containers {
			if containers[i].ID == id {
				return &containers[i]
			}
		}
		if instance, node, vmID, ok := parseGuestID(id); ok {
			for i := range containers {
				ct := &containers[i]
				if ct.VMID == vmID && ct.Node == node && ct.Instance == instance {
					return ct
				}
			}
		}
		return nil
	}

	findNode := func(id string) *models.Node {
		loadFallbackState()
		for i := range nodes {
			if nodes[i].ID == id {
				return &nodes[i]
			}
		}
		return nil
	}

	findStorage := func(id string) *models.Storage {
		loadLegacyStorage()
		for i := range storagePools {
			if storagePools[i].ID == id {
				return &storagePools[i]
			}
		}
		return nil
	}

	findUnifiedStorage := func(id string) *storageMetricFallback {
		loadUnifiedStorage()
		target := strings.TrimSpace(id)
		if target == "" {
			return nil
		}
		for i := range unifiedStorage {
			if strings.EqualFold(unifiedStorage[i].ID, target) {
				return &unifiedStorage[i]
			}
		}
		matchIndex := -1
		for i := range unifiedStorage {
			if !strings.EqualFold(strings.TrimSpace(unifiedStorage[i].ResourceID), target) {
				continue
			}
			if matchIndex >= 0 {
				return nil
			}
			matchIndex = i
		}
		if matchIndex < 0 {
			return nil
		}
		return &unifiedStorage[matchIndex]
	}

	findDockerHost := func(id string) *models.DockerHost {
		loadFallbackState()
		for i := range dockerHosts {
			if dockerHosts[i].ID == id {
				return &dockerHosts[i]
			}
		}
		return nil
	}

	findHost := func(id string) *models.Host {
		loadFallbackState()
		for i := range hosts {
			if hosts[i].ID == id {
				return &hosts[i]
			}
		}
		return nil
	}

	findDockerContainer := func(id string) (*models.DockerContainer, int) {
		loadFallbackState()
		for i := range dockerHosts {
			host := &dockerHosts[i]
			for j := range host.Containers {
				if host.Containers[j].ID == id {
					return &host.Containers[j], host.CPUs
				}
			}
		}
		return nil, 0
	}

	findDisk := func(id string) *unifiedresources.Resource {
		loadPhysicalDisks()
		target := strings.TrimSpace(id)
		if target == "" {
			return nil
		}

		for i := range physicalDisks {
			if strings.EqualFold(physicalDisks[i].ID, target) {
				return &physicalDisks[i]
			}
		}
		uniqueMatch := func(predicate func(*unifiedresources.Resource) bool) *unifiedresources.Resource {
			matchIndex := -1
			for index := range physicalDisks {
				if !predicate(&physicalDisks[index]) {
					continue
				}
				if matchIndex >= 0 {
					return nil
				}
				matchIndex = index
			}
			if matchIndex < 0 {
				return nil
			}
			return &physicalDisks[matchIndex]
		}
		if disk := uniqueMatch(func(candidate *unifiedresources.Resource) bool {
			return candidate.MetricsTarget != nil &&
				strings.EqualFold(strings.TrimSpace(candidate.MetricsTarget.ResourceID), target)
		}); disk != nil {
			return disk
		}
		if !diskinventory.IsUsableHardwareID(target) {
			return nil
		}
		return uniqueMatch(func(candidate *unifiedresources.Resource) bool {
			if candidate.PhysicalDisk == nil {
				return false
			}
			serial := strings.TrimSpace(candidate.PhysicalDisk.Serial)
			wwn := strings.TrimSpace(candidate.PhysicalDisk.WWN)
			return (diskinventory.IsUsableHardwareID(serial) && strings.EqualFold(serial, target)) ||
				(diskinventory.IsUsableHardwareID(wwn) && strings.EqualFold(wwn, target))
		})
	}

	liveMetricPoints := func(resourceType, resourceID string) map[string]monitoring.MetricPoint {
		now := time.Now()
		points := make(map[string]monitoring.MetricPoint)

		switch resourceType {
		case "vm":
			vm := findVM(resourceID)
			if vm == nil {
				return points
			}
			points["cpu"] = monitoring.MetricPoint{Timestamp: now, Value: chartapi.ProxmoxModelCPURatioPercent(vm.CPU)}
			points["memory"] = monitoring.MetricPoint{Timestamp: now, Value: vm.Memory.Usage}
			if vm.Disk.Usage >= 0 {
				points["disk"] = monitoring.MetricPoint{Timestamp: now, Value: vm.Disk.Usage}
			}
			points["diskread"] = monitoring.MetricPoint{Timestamp: now, Value: float64(vm.DiskRead)}
			points["diskwrite"] = monitoring.MetricPoint{Timestamp: now, Value: float64(vm.DiskWrite)}
			points["netin"] = monitoring.MetricPoint{Timestamp: now, Value: float64(vm.NetworkIn)}
			points["netout"] = monitoring.MetricPoint{Timestamp: now, Value: float64(vm.NetworkOut)}
		case "system-container", "oci-container":
			ct := findContainer(resourceID)
			if ct == nil {
				return points
			}
			points["cpu"] = monitoring.MetricPoint{Timestamp: now, Value: chartapi.ProxmoxModelCPURatioPercent(ct.CPU)}
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
			points["cpu"] = monitoring.MetricPoint{Timestamp: now, Value: chartapi.ProxmoxModelCPURatioPercent(node.CPU)}
			points["memory"] = monitoring.MetricPoint{Timestamp: now, Value: node.Memory.Usage}
			points["disk"] = monitoring.MetricPoint{Timestamp: now, Value: node.Disk.Usage}
			if temperature := primaryNodeTemperatureCelsius(node.Temperature); temperature != nil {
				points["temperature"] = monitoring.MetricPoint{Timestamp: now, Value: *temperature}
			}
		case "storage":
			storage := findStorage(resourceID)
			if storage != nil {
				usagePercent := float64(0)
				if storage.Total > 0 {
					usagePercent = (float64(storage.Used) / float64(storage.Total)) * 100
				}
				points["disk"] = monitoring.MetricPoint{Timestamp: now, Value: usagePercent}
				points["usage"] = monitoring.MetricPoint{Timestamp: now, Value: usagePercent}
				points["used"] = monitoring.MetricPoint{Timestamp: now, Value: float64(storage.Used)}
				points["total"] = monitoring.MetricPoint{Timestamp: now, Value: float64(storage.Total)}
				points["avail"] = monitoring.MetricPoint{Timestamp: now, Value: float64(storage.Free)}
				return points
			}
			resource := findUnifiedStorage(resourceID)
			if resource == nil {
				return points
			}
			usagePercent := resource.Percent
			if resource.Total > 0 {
				usagePercent = (float64(resource.Used) / float64(resource.Total)) * 100
			}
			points["disk"] = monitoring.MetricPoint{Timestamp: now, Value: usagePercent}
			points["usage"] = monitoring.MetricPoint{Timestamp: now, Value: usagePercent}
			if resource.Total > 0 {
				points["used"] = monitoring.MetricPoint{Timestamp: now, Value: float64(resource.Used)}
				points["total"] = monitoring.MetricPoint{Timestamp: now, Value: float64(resource.Total)}
				points["avail"] = monitoring.MetricPoint{Timestamp: now, Value: float64(resource.Total - resource.Used)}
			}
		case "docker-host":
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
		case "agent":
			host := findHost(resourceID)
			if host != nil {
				points["cpu"] = monitoring.MetricPoint{Timestamp: now, Value: host.CPUUsage}
				points["memory"] = monitoring.MetricPoint{Timestamp: now, Value: host.Memory.Usage}
				diskPercent := float64(0)
				if len(host.Disks) > 0 {
					diskPercent = host.Disks[0].Usage
				}
				points["disk"] = monitoring.MetricPoint{Timestamp: now, Value: diskPercent}
				if temperature := primaryHostSensorTemperatureCelsius(host.Sensors); temperature != nil {
					points["temperature"] = monitoring.MetricPoint{Timestamp: now, Value: *temperature}
				}
				// Note: We intentionally don't include netin/netout here because the host model
				// only has cumulative RXBytes/TXBytes (total since boot), not rates.
				// The RateTracker in ApplyHostReport calculates rates and stores them in metrics history.
				// Showing cumulative bytes as if they were rates would be misleading (showing GB instead of KB/s).
				return points
			}
			node := findNode(resourceID)
			if node == nil {
				return points
			}
			points["cpu"] = monitoring.MetricPoint{Timestamp: now, Value: chartapi.ProxmoxModelCPURatioPercent(node.CPU)}
			points["memory"] = monitoring.MetricPoint{Timestamp: now, Value: node.Memory.Usage}
			points["disk"] = monitoring.MetricPoint{Timestamp: now, Value: node.Disk.Usage}
			if temperature := primaryNodeTemperatureCelsius(node.Temperature); temperature != nil {
				points["temperature"] = monitoring.MetricPoint{Timestamp: now, Value: *temperature}
			}
		case "app-container":
			container, hostCPUs := findDockerContainer(resourceID)
			if container == nil {
				return points
			}
			points["cpu"] = monitoring.MetricPoint{Timestamp: now, Value: models.DockerContainerCPUCapacityPercent(*container, hostCPUs)}
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
				if s.PowerOnHours != nil {
					points["smart_power_on_hours"] = monitoring.MetricPoint{Timestamp: now, Value: float64(*s.PowerOnHours)}
				}
				if s.PowerCycles != nil {
					points["smart_power_cycles"] = monitoring.MetricPoint{Timestamp: now, Value: float64(*s.PowerCycles)}
				}
				if s.ReallocatedSectors != nil {
					points["smart_reallocated_sectors"] = monitoring.MetricPoint{Timestamp: now, Value: float64(*s.ReallocatedSectors)}
				}
				if s.PendingSectors != nil {
					points["smart_pending_sectors"] = monitoring.MetricPoint{Timestamp: now, Value: float64(*s.PendingSectors)}
				}
				if s.OfflineUncorrectable != nil {
					points["smart_offline_uncorrectable"] = monitoring.MetricPoint{Timestamp: now, Value: float64(*s.OfflineUncorrectable)}
				}
				if s.UDMACRCErrors != nil {
					points["smart_crc_errors"] = monitoring.MetricPoint{Timestamp: now, Value: float64(*s.UDMACRCErrors)}
				}
				if s.PercentageUsed != nil {
					points["smart_percentage_used"] = monitoring.MetricPoint{Timestamp: now, Value: float64(*s.PercentageUsed)}
				}
				if s.AvailableSpare != nil {
					points["smart_available_spare"] = monitoring.MetricPoint{Timestamp: now, Value: float64(*s.AvailableSpare)}
				}
				if s.MediaErrors != nil {
					points["smart_media_errors"] = monitoring.MetricPoint{Timestamp: now, Value: float64(*s.MediaErrors)}
				}
				if s.UnsafeShutdowns != nil {
					points["smart_unsafe_shutdowns"] = monitoring.MetricPoint{Timestamp: now, Value: float64(*s.UnsafeShutdowns)}
				}
			}
		}

		return points
	}

	guestChartMetrics := func() (map[string][]monitoring.MetricPoint, bool) {
		inMemoryKey := resourceID
		sqlResourceType := ""
		switch runtimeResourceType {
		case "vm":
			sqlResourceType = "vm"
		case "system-container", "oci-container":
			sqlResourceType = "container"
		case "k8s":
			sqlResourceType = "k8s"
		case "docker-host":
			inMemoryKey = fmt.Sprintf("dockerHost:%s", resourceID)
			sqlResourceType = "dockerHost"
		case "agent":
			inMemoryKey = fmt.Sprintf("agent:%s", resourceID)
			sqlResourceType = "agent"
		case "app-container":
			inMemoryKey = fmt.Sprintf("docker:%s", resourceID)
			sqlResourceType = "dockerContainer"
		default:
			return nil, false
		}
		return monitor.GetGuestMetricsForChart(inMemoryKey, sqlResourceType, resourceID, duration), true
	}
	guestChartSource := func() string {
		if mock.IsMockEnabled() {
			return historySourceMock
		}
		return historySourceMemory
	}

	fallbackSingle := func() ([]map[string]interface{}, string, bool) {
		if !fallbackAllowed || metricType == "" {
			return nil, "", false
		}

		if mock.IsMockEnabled() && runtimeResourceType == "disk" {
			current := 0.0
			if disk := findDisk(resourceID); disk != nil && disk.PhysicalDisk != nil && metricType == "smart_temp" {
				current = float64(disk.PhysicalDisk.Temperature)
			}
			if current > 0 || metricType == "disk" || metricType == "diskread" || metricType == "diskwrite" {
				series := chartapi.BuildSyntheticMetricHistorySeries(
					end,
					duration,
					historyMaxPoints,
					"disk",
					resourceID,
					metricType,
					current,
				)
				if len(series) > 0 {
					return buildHistoryPoints(series, stepSecs), historySourceMock, true
				}
			}
		}

		if mock.IsMockEnabled() && runtimeResourceType == "storage" && queryMetric == "usage" {
			if current, ok := liveMetricPoints(runtimeResourceType, resourceID)["usage"]; ok {
				series := chartapi.BuildSyntheticMetricHistorySeries(
					end,
					duration,
					historyMaxPoints,
					"storage",
					resourceID,
					"usage",
					current.Value,
				)
				if len(series) > 0 {
					return buildHistoryPoints(series, stepSecs), historySourceMock, true
				}
			}
		}

		switch runtimeResourceType {
		case "vm", "system-container", "oci-container":
			metrics, _ := guestChartMetrics()
			points := metrics[metricType]
			if len(points) == 0 {
				livePoints := liveMetricPoints(runtimeResourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), guestChartSource(), true
		case "docker-host":
			metrics, _ := guestChartMetrics()
			points := metrics[metricType]
			if len(points) == 0 {
				livePoints := liveMetricPoints(runtimeResourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), guestChartSource(), true
		case "agent":
			metrics, _ := guestChartMetrics()
			points := metrics[metricType]
			if len(points) == 0 {
				points = monitor.GetNodeMetrics(resourceID, metricType, duration)
			}
			if len(points) == 0 {
				livePoints := liveMetricPoints(runtimeResourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), guestChartSource(), true
		case "app-container":
			metrics, _ := guestChartMetrics()
			points := metrics[metricType]
			if len(points) == 0 {
				livePoints := liveMetricPoints(runtimeResourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), guestChartSource(), true
		case "k8s":
			metrics, _ := guestChartMetrics()
			points := metrics[metricType]
			if len(points) == 0 {
				livePoints := liveMetricPoints(runtimeResourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), guestChartSource(), true
		case "node":
			points := monitor.GetNodeMetricsForChart(resourceID, metricType, duration)
			if len(points) == 0 {
				livePoints := liveMetricPoints(runtimeResourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			source := historySourceMemory
			if mock.IsMockEnabled() {
				source = historySourceMock
			}
			return buildHistoryPoints(points, stepSecs), source, true
		case "storage":
			metrics := monitor.GetStorageMetrics(resourceID, duration)
			points := metrics[queryMetric]
			if len(points) == 0 {
				livePoints := liveMetricPoints(runtimeResourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), historySourceMemory, true
		case "disk":
			points := monitor.GetDiskMetricsForChart(resourceID, queryMetric, duration)
			if len(points) == 0 {
				livePoints := liveMetricPoints(runtimeResourceType, resourceID)
				if live, ok := livePoints[metricType]; ok {
					return buildHistoryPoints([]monitoring.MetricPoint{live}, 0), historySourceLive, true
				}
				return nil, "", false
			}
			return buildHistoryPoints(points, stepSecs), historySourceMemory, true
		default:
			livePoints := liveMetricPoints(runtimeResourceType, resourceID)
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
		guestHistory := false
		source := historySourceMemory
		switch runtimeResourceType {
		case "vm", "system-container", "oci-container", "k8s", "docker-host":
			metrics, guestHistory = guestChartMetrics()
		case "agent":
			metrics, guestHistory = guestChartMetrics()
			if len(metrics) == 0 {
				metrics = map[string][]monitoring.MetricPoint{
					"cpu":    monitor.GetNodeMetrics(resourceID, "cpu", duration),
					"memory": monitor.GetNodeMetrics(resourceID, "memory", duration),
					"disk":   monitor.GetNodeMetrics(resourceID, "disk", duration),
				}
			}
		case "app-container":
			metrics, guestHistory = guestChartMetrics()
		case "storage":
			metrics = monitor.GetStorageMetrics(resourceID, duration)
			if mock.IsMockEnabled() && len(metrics["usage"]) == 0 {
				if current, ok := liveMetricPoints(runtimeResourceType, resourceID)["usage"]; ok {
					metrics = map[string][]monitoring.MetricPoint{
						"usage": chartapi.BuildSyntheticMetricHistorySeries(
							end,
							duration,
							historyMaxPoints,
							"storage",
							resourceID,
							"usage",
							current.Value,
						),
					}
					source = historySourceMock
				}
			}
		case "disk":
			metrics = map[string][]monitoring.MetricPoint{
				"disk":       monitor.GetDiskMetricsForChart(resourceID, "disk", duration),
				"diskread":   monitor.GetDiskMetricsForChart(resourceID, "diskread", duration),
				"diskwrite":  monitor.GetDiskMetricsForChart(resourceID, "diskwrite", duration),
				"smart_temp": monitor.GetDiskMetricsForChart(resourceID, "smart_temp", duration),
			}
		default:
			if runtimeResourceType == "node" {
				metrics = map[string][]monitoring.MetricPoint{
					"cpu":         monitor.GetNodeMetricsForChart(resourceID, "cpu", duration),
					"memory":      monitor.GetNodeMetricsForChart(resourceID, "memory", duration),
					"disk":        monitor.GetNodeMetricsForChart(resourceID, "disk", duration),
					"netin":       monitor.GetNodeMetricsForChart(resourceID, "netin", duration),
					"netout":      monitor.GetNodeMetricsForChart(resourceID, "netout", duration),
					"temperature": monitor.GetNodeMetricsForChart(resourceID, "temperature", duration),
				}
				if mock.IsMockEnabled() {
					source = historySourceMock
				}
			} else {
				return nil, "", false
			}
		}

		apiData := make(map[string][]map[string]interface{})
		if guestHistory {
			source = guestChartSource()
		}
		for metric, points := range metrics {
			if len(points) == 0 {
				continue
			}
			apiData[metric] = buildHistoryPoints(points, stepSecs)
		}
		if len(apiData) == 0 {
			livePoints := liveMetricPoints(runtimeResourceType, resourceID)
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

	// Mock storage targets with current capacity are fully answerable from the
	// in-memory history/live fallback. Serve that path before entering the
	// persistent store: provider-derived targets may have no stored series, and
	// a legacy row can still expose only a sparse point while demo backfill keeps
	// the store busy.
	if mock.IsMockEnabled() && runtimeResourceType == "storage" {
		response := map[string]interface{}{
			"resourceType": responseResourceType,
			"resourceId":   resourceID,
			"range":        timeRange,
			"start":        start.UnixMilli(),
			"end":          end.UnixMilli(),
		}
		if metricType != "" {
			if apiPoints, source, ok := fallbackSingle(); ok {
				response["metric"] = metricType
				response["points"] = apiPoints
				response["source"] = source
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
		} else if apiData, source, ok := fallbackAll(); ok {
			response["metrics"] = apiData
			response["source"] = source
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	store := monitor.GetMetricsStore()
	queryStoreMetric := func(metric string) ([]metricstore.MetricPoint, string, error) {
		if len(storeResourceTypes) == 0 {
			return nil, runtimeResourceType, nil
		}
		for _, storeType := range storeResourceTypes {
			points, err := store.Query(storeType, resourceID, metric, start, end, stepSecs)
			if err != nil {
				return nil, storeType, err
			}
			if len(points) > 0 {
				return points, storeType, nil
			}
		}
		return nil, storeResourceTypes[0], nil
	}
	queryStoreAllMetrics := func() (map[string][]metricstore.MetricPoint, string, error) {
		if len(storeResourceTypes) == 0 {
			return nil, runtimeResourceType, nil
		}
		for _, storeType := range storeResourceTypes {
			metricsMap, err := store.QueryAll(storeType, resourceID, start, end, stepSecs)
			if err != nil {
				return nil, storeType, err
			}
			if len(metricsMap) > 0 {
				return metricsMap, storeType, nil
			}
		}
		return nil, storeResourceTypes[0], nil
	}
	if store == nil {
		if metricType != "" {
			if apiPoints, source, ok := fallbackSingle(); ok {
				log.Warn().
					Str("resourceType", runtimeResourceType).
					Str("resourceId", resourceID).
					Str("metric", metricType).
					Str("source", source).
					Msg("Metrics store unavailable; serving history from fallback source")
				response := map[string]interface{}{
					"resourceType": responseResourceType,
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
					Str("resourceType", runtimeResourceType).
					Str("resourceId", resourceID).
					Str("source", source).
					Msg("Metrics store unavailable; serving history from fallback source")
				response := map[string]interface{}{
					"resourceType": responseResourceType,
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
		points, storeTypeUsed, err := queryStoreMetric(queryMetric)
		if err != nil {
			log.Error().Err(err).
				Str("resourceType", runtimeResourceType).
				Str("storeType", storeTypeUsed).
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
					Str("resourceType", runtimeResourceType).
					Str("resourceId", resourceID).
					Str("metric", metricType).
					Str("source", source).
					Msg("Metrics store empty; serving history from fallback source")
				response = map[string]interface{}{
					"resourceType": responseResourceType,
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

		if response == nil && mock.IsMockEnabled() && runtimeResourceType == "disk" &&
			(metricType == "smart_temp" || metricType == "disk" || metricType == "diskread" || metricType == "diskwrite") {
			targetPoints := chartapi.TargetMockSeriesPoints(duration, historyMaxPoints)
			if len(points) > 0 && len(points) < targetPoints {
				current := points[len(points)-1].Value
				if metricType == "smart_temp" {
					if disk := findDisk(resourceID); disk != nil && disk.PhysicalDisk != nil && disk.PhysicalDisk.Temperature > 0 {
						current = float64(disk.PhysicalDisk.Temperature)
					}
				}
				if metricType != "smart_temp" || current > 0 {
					series := chartapi.BuildSyntheticMetricHistorySeries(
						end,
						duration,
						historyMaxPoints,
						"disk",
						resourceID,
						metricType,
						current,
					)
					if len(series) > len(points) {
						source = historySourceMock
						response = map[string]interface{}{
							"resourceType": responseResourceType,
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
				"resourceType": responseResourceType,
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
		metricsMap, storeTypeUsed, err := queryStoreAllMetrics()
		if err != nil {
			log.Error().Err(err).
				Str("resourceType", runtimeResourceType).
				Str("storeType", storeTypeUsed).
				Str("resourceId", resourceID).
				Msg("Failed to query all metrics history")
			http.Error(w, "Failed to query metrics", http.StatusInternalServerError)
			return
		}

		if len(metricsMap) == 0 {
			if apiData, fallbackSource, ok := fallbackAll(); ok {
				source = fallbackSource
				log.Info().
					Str("resourceType", runtimeResourceType).
					Str("resourceId", resourceID).
					Str("source", source).
					Msg("Metrics store empty; serving history from fallback source")
				response = map[string]interface{}{
					"resourceType": responseResourceType,
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

			// QueryAll can return a non-empty but incomplete mock metric map. Fill
			// only the absent series from the deterministic chart fallback so one
			// populated metric cannot suppress entire drawer groups.
			if mock.IsMockEnabled() {
				if fallbackData, fallbackSource, ok := fallbackAll(); ok {
					supplemented := false
					for metric, points := range fallbackData {
						if len(apiData[metric]) > 0 || len(points) == 0 {
							continue
						}
						apiData[metric] = points
						supplemented = true
					}
					if supplemented {
						source = fallbackSource
						log.Info().
							Str("resourceType", runtimeResourceType).
							Str("resourceId", resourceID).
							Str("source", source).
							Msg("Metrics store incomplete; supplementing missing mock history series")
					}
				}
			}

			response = map[string]interface{}{
				"resourceType": responseResourceType,
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

func canonicalizeMetricsHistoryResourceID(runtimeResourceType, resourceID string) string {
	trimmed := strings.TrimSpace(resourceID)
	if runtimeResourceType != "k8s" {
		return trimmed
	}
	if strings.Contains(trimmed, ":pod:") {
		return unifiedresources.CanonicalKubernetesPodMetricID(trimmed)
	}
	return trimmed
}

func primaryHostSensorTemperatureCelsius(sensors models.HostSensorSummary) *float64 {
	if len(sensors.TemperatureCelsius) == 0 {
		return nil
	}
	if value, ok := sensors.TemperatureCelsius["cpu_package"]; ok && value > 0 {
		return &value
	}

	var best float64
	found := false
	for key, value := range sensors.TemperatureCelsius {
		if value <= 0 || !strings.HasPrefix(strings.ToLower(key), "cpu") {
			continue
		}
		if !found || value > best {
			best = value
			found = true
		}
	}
	if !found {
		return nil
	}
	return &best
}

func primaryNodeTemperatureCelsius(temperature *models.Temperature) *float64 {
	if temperature == nil || !temperature.Available {
		return nil
	}
	if temperature.CPUMax > 0 {
		value := temperature.CPUMax
		return &value
	}
	if temperature.CPUPackage > 0 {
		value := temperature.CPUPackage
		return &value
	}

	var best float64
	found := false
	for _, core := range temperature.Cores {
		if core.Temp <= 0 {
			continue
		}
		if !found || core.Temp > best {
			best = core.Temp
			found = true
		}
	}
	if !found {
		return nil
	}
	return &best
}

func normalizeMetricsHistoryResourceType(input string) (responseType string, runtimeType string, storeTypes []string, err error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "node":
		return "node", "node", []string{"node"}, nil
	case "storage":
		return "storage", "storage", []string{"storage"}, nil
	case "agent":
		return "agent", "agent", []string{"agent", "node"}, nil
	case "disk":
		return "disk", "disk", []string{"disk"}, nil
	case "k8s":
		return "k8s", "k8s", []string{"k8s"}, nil
	case "vm":
		return "vm", "vm", []string{"vm"}, nil
	case "system-container":
		return "system-container", "system-container", []string{"container"}, nil
	case "oci-container":
		return "oci-container", "oci-container", []string{"container"}, nil
	case "app-container":
		return "app-container", "app-container", []string{"dockerContainer", "docker"}, nil
	case "docker-host":
		return "docker-host", "docker-host", []string{"dockerHost"}, nil
	default:
		return "", "", nil, fmt.Errorf("unsupported resourceType %q", input)
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
	payload := map[string]interface{}{
		"csrfProtection":    false, // Not implemented yet
		"autoUpdateEnabled": config.EffectiveAutoUpdateEnabled(r.config.UpdateChannel, r.config.AutoUpdateEnabled),
		"updateChannel":     config.EffectiveUpdateChannel(r.config.UpdateChannel, ""),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
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
	nonce := CSPNonceFromContext(req.Context())
	nonceAttr := ""
	if nonce != "" {
		nonceAttr = ` nonce="` + nonce + `"`
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>Simple Pulse Stats</title>
    <style` + nonceAttr + `>
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
        #update-indicator { display: none; }
    </style>
</head>
<body>
    <h1>Simple Pulse Stats</h1>
    <div id="status">
        <div>
            <span id="status-text">Connecting...</span>
            <span class="update-indicator" id="update-indicator"></span>
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

    <script` + nonceAttr + `>
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

        function appendCell(row, value, className) {
            const cell = document.createElement('td');
            if (className) {
                cell.className = className;
            }
            cell.textContent = value;
            row.appendChild(cell);
            return cell;
        }

        function appendStatusCell(row, status) {
            const cell = document.createElement('td');
            const badge = document.createElement('span');
            badge.classList.add('status');
            if (status === 'running' || status === 'stopped') {
                badge.classList.add(status);
            }
            badge.textContent = status || 'unknown';
            cell.appendChild(badge);
            row.appendChild(cell);
        }
        
        function updateTable(containers) {
            const tbody = document.querySelector('#containers tbody');
            tbody.innerHTML = '';
            
            containers.sort((a, b) => a.name.localeCompare(b.name));
            
            containers.forEach(ct => {
                const row = document.createElement('tr');
                const nameCell = document.createElement('td');
                const nameStrong = document.createElement('strong');
                nameStrong.textContent = ct.name || '';
                nameCell.appendChild(nameStrong);
                row.appendChild(nameCell);

                appendStatusCell(row, ct.status);
                appendCell(row, (ct.cpu ? ct.cpu.toFixed(1) : '0.0') + '%', 'metric');
                appendCell(row, formatMemory(ct.mem || 0, ct.maxmem || 1), 'metric');
                appendCell(row, formatBytes(ct.diskread), 'metric');
                appendCell(row, formatBytes(ct.diskwrite), 'metric');
                appendCell(row, formatBytes(ct.netin), 'metric');
                appendCell(row, formatBytes(ct.netout), 'metric');
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

func resolveExplicitWebSocketOrgID(req *http.Request) (string, bool) {
	if req == nil {
		return "", false
	}

	if headerOrg := strings.TrimSpace(req.Header.Get("X-Pulse-Org-ID")); headerOrg != "" {
		return headerOrg, true
	}

	if cookie, err := req.Cookie(CookieNameOrgID); err == nil {
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
		AgentID    string `json:"agentId"`
		TokenName  string `json:"tokenName"`
		EnableHost *bool  `json:"enableHost,omitempty"`
	}

	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", nil)
		return
	}

	agentID := strings.TrimSpace(payload.AgentID)
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "Agent ID (agentId) is required", nil)
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

	readState := monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "read_state_unavailable", "Container runtime state is not available", nil)
		return
	}

	var host *unifiedresources.DockerHostView
	for _, candidate := range readState.DockerHosts() {
		if candidate == nil {
			continue
		}
		if candidate.HostSourceID() == agentID || candidate.ID() == agentID {
			host = candidate
			break
		}
	}
	if host == nil {
		writeErrorResponse(w, http.StatusNotFound, "agent_not_found", "Container runtime not found", nil)
		return
	}
	hostID := host.HostSourceID()
	if hostID == "" {
		hostID = host.ID()
	}

	name := strings.TrimSpace(payload.TokenName)
	if name == "" {
		displayName := preferredDockerHostName(host)
		name = fmt.Sprintf("Container runtime: %s", displayName)
	}
	enableHost := true
	if payload.EnableHost != nil {
		enableHost = *payload.EnableHost
	}

	// Resolve the credential target before minting or persisting the token.
	// Hosted mode deliberately has no request-origin or localhost fallback, so
	// an absent or invalid authoritative URL must leave token state untouched.
	baseURL := normalizeAgentInstallBaseURL(r.resolvePublicURL(req))
	if r.hostedMode && baseURL == "" {
		writeConfigAgentInstallBaseURLUnavailable(w)
		return
	}

	rawToken, err := auth.GenerateAPIToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate container runtime migration token")
		writeErrorResponse(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate API token", nil)
		return
	}

	record, err := config.NewAPITokenRecord(rawToken, name, containerRuntimeAgentScopes(enableHost))
	if err != nil {
		log.Error().Err(err).Msg("Failed to construct token record for container runtime migration")
		writeErrorResponse(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate API token", nil)
		return
	}
	record.OrgID = orgID
	if record.OrgID == "" {
		record.OrgID = "default"
	}
	setAPITokenOwnerUserID(record, apiTokenOwnerUserIDForRequest(r.config, req))

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
	previousTokens := append([]config.APITokenRecord(nil), activeConfig.APITokens...)
	activeConfig.APITokens = append(activeConfig.APITokens, *record)
	activeConfig.SortAPITokens()

	if activePersistence != nil {
		if err := activePersistence.SaveAPITokens(activeConfig.APITokens); err != nil {
			activeConfig.APITokens = previousTokens
			activeConfig.SortAPITokens()
			config.Mu.Unlock()
			log.Error().Err(err).Msg("Failed to persist API tokens after container runtime migration generation")
			writeErrorResponse(w, http.StatusInternalServerError, "token_persist_failed", "Failed to persist API token", nil)
			return
		}
	}
	config.Mu.Unlock()

	installCommand := buildContainerRuntimeAgentInstallCommand(baseURL, rawToken, enableHost)
	systemdSnippet := fmt.Sprintf("[Service]\nType=simple\nEnvironment=\"PULSE_URL=%s\"\nEnvironment=\"PULSE_TOKEN=%s\"\nExecStart=/usr/local/bin/pulse-agent --url %s --token %s --enable-docker %s --interval 30s\nRestart=always\nRestartSec=5s\nUser=root", baseURL, rawToken, baseURL, rawToken, containerRuntimeAgentHostFlag(enableHost))

	response := map[string]any{
		"success": true,
		"token":   rawToken,
		"record":  toAPITokenDTO(*record),
		"agent": map[string]any{
			"id":   hostID,
			"name": preferredDockerHostName(host),
		},
		"installCommand":        installCommand,
		"systemdServiceSnippet": systemdSnippet,
		"pulseURL":              baseURL,
		"enableHost":            enableHost,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to serialize container runtime token migration response")
	}
}

func resolveConfiguredPublicBaseURL(req *http.Request, cfg *config.Config, hostedMode bool) string {
	if cfg == nil {
		return ""
	}

	// Hosted mode must never fall back to request host or localhost.
	// A canonical externally-reachable URL must be configured via PublicURL / AgentConnectURL.
	if hostedMode {
		if agentConnectURL := strings.TrimSpace(cfg.AgentConnectURL); agentConnectURL != "" {
			normalized, err := securityutil.NormalizePulseHTTPBaseURL(agentConnectURL)
			if err != nil {
				return ""
			}
			return strings.TrimRight(normalized.String(), "/")
		}
		if publicURL := strings.TrimSpace(cfg.PublicURL); publicURL != "" && !cfg.PublicURLAutoDetected {
			normalized, err := securityutil.NormalizePulseHTTPBaseURL(publicURL)
			if err != nil {
				return ""
			}
			return strings.TrimRight(normalized.String(), "/")
		}
		return ""
	}

	if agentConnectURL := strings.TrimSpace(cfg.AgentConnectURL); agentConnectURL != "" {
		return strings.TrimRight(agentConnectURL, "/")
	}

	// An operator-configured public URL is authoritative. An auto-detected one
	// is only a guess (boot-time LAN IP probe or first-request capture), so the
	// live request's own origin outranks it: the URL the admin is browsing
	// right now is fresher evidence of how this instance is reached (#1692).
	publicURL := strings.TrimSpace(cfg.PublicURL)
	if publicURL != "" && !cfg.PublicURLAutoDetected {
		return strings.TrimRight(publicURL, "/")
	}

	if requestURL := requestOriginBaseURL(req); requestURL != "" {
		return requestURL
	}

	if publicURL != "" {
		return strings.TrimRight(publicURL, "/")
	}

	if cfg.FrontendPort > 0 {
		return fmt.Sprintf("http://localhost:%d", cfg.FrontendPort)
	}
	return "http://localhost:7655"
}

func (r *Router) resolvePublicURL(req *http.Request) string {
	if r == nil {
		return ""
	}
	return resolveConfiguredPublicBaseURL(req, r.config, r.hostedMode)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

type mockSupplementalRecordsAdapter struct {
	source unifiedresources.DataSource
}

func (a mockSupplementalRecordsAdapter) GetCurrentRecords() []unifiedresources.IngestRecord {
	return a.GetCurrentRecordsForOrg("default")
}

func (a mockSupplementalRecordsAdapter) GetCurrentRecordsForOrg(orgID string) []unifiedresources.IngestRecord {
	if strings.TrimSpace(orgID) != "" && strings.TrimSpace(orgID) != "default" {
		return nil
	}
	return mock.SupplementalRecords(a.source)
}

func (a mockSupplementalRecordsAdapter) SupplementalRecords(_ *monitoring.Monitor, orgID string) []unifiedresources.IngestRecord {
	return a.GetCurrentRecordsForOrg(orgID)
}

func (a mockSupplementalRecordsAdapter) GetCurrentChangesForOrg(orgID string) []unifiedresources.ResourceChange {
	if strings.TrimSpace(orgID) != "" && strings.TrimSpace(orgID) != "default" {
		return nil
	}
	return mock.SupplementalChanges(a.source)
}

func (a mockSupplementalRecordsAdapter) SupplementalChanges(_ *monitoring.Monitor, orgID string) []unifiedresources.ResourceChange {
	return a.GetCurrentChangesForOrg(orgID)
}

func (a mockSupplementalRecordsAdapter) SnapshotOwnedSources() []unifiedresources.DataSource {
	normalized := resourceapi.NormalizeDataSourceAlias(a.source)
	if normalized == "" {
		return nil
	}
	return []unifiedresources.DataSource{normalized}
}

func (a mockSupplementalRecordsAdapter) SnapshotOwnedSourcesForOrg(string) []unifiedresources.DataSource {
	return a.SnapshotOwnedSources()
}

func (a mockSupplementalRecordsAdapter) SupplementalInventoryReadyAt(*monitoring.Monitor, string) (time.Time, bool) {
	return time.Time{}, true
}

// trigger rebuild Fri Jan 16 10:52:41 UTC 2026
