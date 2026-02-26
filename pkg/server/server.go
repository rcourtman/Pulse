package server

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rcourtman/pulse-go-rewrite/internal/hosted"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
	"github.com/rs/zerolog/log"
)

// Version information
var (
	MetricsPort = 9091
)

// BusinessHooks allows enterprise features to hook into the server lifecycle.
type BusinessHooks struct {
	// OnMetricsStoreReady is called when the metrics store is initialized.
	// This allows enterprise features to access metrics for reporting.
	OnMetricsStoreReady func(store *metrics.Store)

	// BindRBACAdminEndpoints allows enterprise modules to replace or decorate
	// RBAC admin endpoints without importing internal API packages.
	BindRBACAdminEndpoints extensions.BindRBACAdminEndpointsFunc

	// BindAuditAdminEndpoints allows enterprise modules to replace or decorate
	// audit admin endpoints without importing internal API packages.
	BindAuditAdminEndpoints extensions.BindAuditAdminEndpointsFunc

	// BindSSOAdminEndpoints allows enterprise modules to replace or decorate
	// SSO admin endpoints without importing internal API packages.
	BindSSOAdminEndpoints extensions.BindSSOAdminEndpointsFunc

	// BindReportingAdminEndpoints allows enterprise modules to replace or decorate
	// reporting admin endpoints without importing internal API packages.
	BindReportingAdminEndpoints extensions.BindReportingAdminEndpointsFunc

	// BindAIAutoFixEndpoints allows enterprise modules to replace or decorate
	// AI auto-fix endpoints (investigation, remediation, autonomy, fix execution).
	BindAIAutoFixEndpoints extensions.BindAIAutoFixEndpointsFunc

	// BindAIAlertAnalysisEndpoints allows enterprise modules to replace or decorate
	// AI alert analysis endpoints (alert investigation, Kubernetes analysis).
	BindAIAlertAnalysisEndpoints extensions.BindAIAlertAnalysisEndpointsFunc

	// AIInvestigationEnabled controls whether premium AI investigation and
	// remediation components are created at runtime. When nil or returns false,
	// patrol runs in monitor-only mode (findings reported but never investigated,
	// no remediation plans generated). Enterprise sets this to return true.
	AIInvestigationEnabled func() bool
}

var (
	globalHooks   BusinessHooks
	globalHooksMu sync.Mutex
)

// SetBusinessHooks registers hooks for the server.
func SetBusinessHooks(h BusinessHooks) {
	globalHooksMu.Lock()
	defer globalHooksMu.Unlock()
	globalHooks = h
}

// Run starts the Pulse monitoring server.
func Run(ctx context.Context, version string) error {
	// Initialize logger with baseline defaults for early startup logs
	logging.Init(logging.Config{
		Format:    "auto",
		Level:     "info",
		Component: "pulse",
	})
	defer logging.Shutdown()

	// Check for auto-import on first startup
	if ShouldAutoImport() {
		if err := PerformAutoImport(); err != nil {
			log.Error().Err(err).Msg("Auto-import failed, continuing with normal startup")
		}
	}

	// Load unified configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Re-initialize logging with configuration-driven settings
	logging.Init(logging.Config{
		Format:     cfg.LogFormat,
		Level:      cfg.LogLevel,
		Component:  "pulse",
		FilePath:   cfg.LogFile,
		MaxSizeMB:  cfg.LogMaxSize,
		MaxAgeDays: cfg.LogMaxAge,
		Compress:   cfg.LogCompress,
	})

	// Initialize license public key for Pro feature validation
	pkglicensing.InitEmbeddedPublicKey()

	// Multi-tenant persistence is the canonical way to resolve the base data directory.
	// It uses cfg.DataPath, which already includes PULSE_DATA_DIR overrides.
	mtPersistence := config.NewMultiTenantPersistence(cfg.DataPath)
	baseDataDir := mtPersistence.BaseDataDir()

	// Initialize RBAC manager for role-based access control
	rbacManager, err := auth.NewFileManager(baseDataDir)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize RBAC manager, role management will be unavailable")
	} else {
		auth.SetManager(rbacManager)
		log.Info().Msg("RBAC manager initialized")
	}

	// Run multi-tenant data migration only when the feature is explicitly enabled.
	// This prevents any on-disk layout changes for default (single-tenant) users.
	if api.IsMultiTenantEnabled() {
		if err := config.RunMigrationIfNeeded(baseDataDir); err != nil {
			log.Error().Err(err).Msg("Multi-tenant data migration failed")
			// Continue anyway - migration failure shouldn't block startup
		}
	}
	if err := ensureDefaultOrgOwnerMembership(mtPersistence, cfg.AuthUser); err != nil {
		log.Warn().Err(err).Msg("Failed to ensure default organization owner membership")
	}

	// Local upgrade metrics must be durable and tenant-aware (P0-6).
	// Renamed from conversion.db -> upgrade_metrics.db to reduce "marketing telemetry" optics.
	upgradeMetricsDB := filepath.Join(baseDataDir, "upgrade_metrics.db")
	legacyConversionDB := filepath.Join(baseDataDir, "conversion.db")
	if _, err := os.Stat(upgradeMetricsDB); os.IsNotExist(err) {
		if _, err := os.Stat(legacyConversionDB); err == nil {
			renameIfExists := func(from, to string) {
				if _, statErr := os.Stat(from); statErr != nil {
					return
				}
				if renameErr := os.Rename(from, to); renameErr != nil {
					log.Warn().Err(renameErr).Str("from", from).Str("to", to).Msg("Failed to migrate legacy upgrade metrics db artifact")
				}
			}
			renameIfExists(legacyConversionDB, upgradeMetricsDB)
			renameIfExists(legacyConversionDB+"-wal", upgradeMetricsDB+"-wal")
			renameIfExists(legacyConversionDB+"-shm", upgradeMetricsDB+"-shm")
			log.Info().Str("from", legacyConversionDB).Str("to", upgradeMetricsDB).Msg("Migrated legacy conversion db to upgrade metrics db")
		}
	}

	conversionStore, err := pkglicensing.NewConversionStore(upgradeMetricsDB)
	if err != nil {
		return fmt.Errorf("failed to initialize local upgrade metrics store: %w", err)
	}
	conversionStoreClosed := false
	closeConversionStore := func() {
		if conversionStoreClosed {
			return
		}
		conversionStoreClosed = true
		if err := conversionStore.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close conversion store")
		}
	}
	defer closeConversionStore()

	// Always capture audit events to SQLite (defense in depth). Read/export endpoints are license-gated.
	// For the default org, TenantLoggerManager routes to the global logger, so initialize it as SQLite too.
	var globalCrypto audit.CryptoEncryptor
	if cm, err := crypto.NewCryptoManagerAt(baseDataDir); err != nil {
		log.Warn().Err(err).Str("data_dir", baseDataDir).Msg("Failed to initialize crypto manager for audit signing; signatures will be disabled")
	} else {
		globalCrypto = cm
		if sqliteLogger, err := audit.NewSQLiteLogger(audit.SQLiteLoggerConfig{
			DataDir:   baseDataDir,
			CryptoMgr: cm,
		}); err != nil {
			log.Warn().Err(err).Str("data_dir", baseDataDir).Msg("Failed to initialize global SQLite audit logger; falling back to console logger")
		} else {
			audit.SetLogger(sqliteLogger)
		}
	}

	// Initialize tenant audit manager for per-tenant audit logging
	tenantAuditManager := audit.NewTenantLoggerManager(baseDataDir, &audit.SQLiteLoggerFactory{
		// Prefer per-tenant crypto managers so each org has its own .encryption.key.
		CryptoMgrForDataDir: func(dataDir string) (audit.CryptoEncryptor, error) {
			return crypto.NewCryptoManagerAt(dataDir)
		},
		// Fallback for environments where per-tenant crypto initialization fails.
		CryptoMgr: globalCrypto,
	})
	api.SetTenantAuditManager(tenantAuditManager)
	log.Info().Msg("Tenant audit manager initialized")

	// Enable async audit logging to avoid request latency on audit writes.
	if !strings.EqualFold(os.Getenv("PULSE_AUDIT_ASYNC"), "false") {
		audit.EnableAsyncLogging(audit.AsyncLoggerConfig{BufferSize: 4096})
		log.Info().Msg("Async audit logging enabled")
	}

	log.Info().Msg("Starting Pulse monitoring server")

	// Validate agent binaries are available for download
	updates.EnsureHostAgentBinaries(version)

	// Create derived context that cancels on interrupt
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Metrics port is configurable via MetricsPort variable
	metricsAddr := fmt.Sprintf("%s:%d", cfg.BindAddress, MetricsPort)
	startMetricsServer(ctx, metricsAddr, cfg.MetricsToken)

	// Initialize WebSocket hub first
	wsHub := websocket.NewHub(nil)
	// Gate X-Forwarded-* trust in checkOrigin on the same trusted-proxy list
	// used by the main API's auth.go / security.go.
	wsHub.SetTrustedProxyChecker(api.IsTrustedProxyIP)
	// Set allowed origins from configuration
	if cfg.AllowedOrigins != "" {
		if cfg.AllowedOrigins == "*" {
			// Explicit wildcard - allow all origins (less secure)
			wsHub.SetAllowedOrigins([]string{"*"})
		} else {
			// Use configured origins
			wsHub.SetAllowedOrigins(strings.Split(cfg.AllowedOrigins, ","))
		}
	} else {
		// Default: don't set any specific origins
		wsHub.SetAllowedOrigins([]string{})
	}
	go wsHub.Run()
	defer wsHub.Stop()

	// Initialize reloadable monitoring system
	reloadableMonitor, err := monitoring.NewReloadableMonitor(cfg, mtPersistence, wsHub)
	if err != nil {
		return fmt.Errorf("failed to initialize monitoring system: %w", err)
	}

	// Trigger enterprise hooks if registered
	globalHooksMu.Lock()
	onMetricsStoreReady := globalHooks.OnMetricsStoreReady
	bindRBACAdminEndpoints := globalHooks.BindRBACAdminEndpoints
	bindAuditAdminEndpoints := globalHooks.BindAuditAdminEndpoints
	bindSSOAdminEndpoints := globalHooks.BindSSOAdminEndpoints
	bindReportingAdminEndpoints := globalHooks.BindReportingAdminEndpoints
	bindAIAutoFixEndpoints := globalHooks.BindAIAutoFixEndpoints
	bindAIAlertAnalysisEndpoints := globalHooks.BindAIAlertAnalysisEndpoints
	aiInvestigationEnabled := globalHooks.AIInvestigationEnabled
	globalHooksMu.Unlock()

	api.SetAIInvestigationEnabled(aiInvestigationEnabled)
	api.SetRBACAdminEndpointsBinder(bindRBACAdminEndpoints)
	api.SetAuditAdminEndpointsBinder(bindAuditAdminEndpoints)
	api.SetSSOAdminEndpointsBinder(bindSSOAdminEndpoints)
	api.SetReportingAdminEndpointsBinder(bindReportingAdminEndpoints)
	api.SetAIAutoFixEndpointsBinder(bindAIAutoFixEndpoints)
	api.SetAIAlertAnalysisEndpointsBinder(bindAIAlertAnalysisEndpoints)

	if onMetricsStoreReady != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Msg("Enterprise OnMetricsStoreReady hook panicked")
				}
			}()
			if store := reloadableMonitor.GetMonitor().GetMetricsStore(); store != nil {
				onMetricsStoreReady(store)
			}
		}()
	}

	// Initialize reporting engine if not already set by enterprise hooks.
	// This ensures Pro license holders get reporting even with the standard binary.
	// Uses a dynamic store getter so the engine always queries the current monitor's
	// metrics store, even after monitor reloads (which close and recreate the store).
	if reporting.GetEngine() == nil {
		if store := reloadableMonitor.GetMonitor().GetMetricsStore(); store != nil {
			engine := reporting.NewReportEngine(reporting.EngineConfig{
				MetricsStoreGetter: func() *metrics.Store {
					return reloadableMonitor.GetMonitor().GetMetricsStore()
				},
			})
			reporting.SetEngine(engine)
			log.Info().Msg("Advanced Infrastructure Reporting (PDF/CSV) initialized")
		}
	}

	// Set state getter for WebSocket hub (legacy - for default org)
	wsHub.SetStateGetter(func() interface{} {
		state := reloadableMonitor.GetMonitor().GetState()
		return state.ToFrontend()
	})

	// Set tenant-aware state getter for multi-tenant support
	wsHub.SetStateGetterForTenant(func(orgID string) interface{} {
		if orgID == "" || orgID == "default" {
			state := reloadableMonitor.GetMonitor().GetState()
			return state.ToFrontend()
		}

		mtMonitor := reloadableMonitor.GetMultiTenantMonitor()
		if mtMonitor == nil {
			// Security: never expose default-org state to non-default org clients.
			log.Warn().Str("org_id", orgID).Msg("Tenant monitor unavailable for org state request")
			return models.StateFrontend{}
		}
		monitor, err := mtMonitor.GetMonitor(orgID)
		if err != nil || monitor == nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to get tenant monitor for org state request")
			return models.StateFrontend{}
		}
		state := monitor.GetState()
		return state.ToFrontend()
	})

	// Set org authorization checker for WebSocket connections
	// This ensures clients can only subscribe to orgs they have access to
	orgLoader := api.NewMultiTenantOrganizationLoader(mtPersistence)
	wsHub.SetOrgAuthChecker(api.NewAuthorizationChecker(orgLoader))

	// Set multi-tenant checker for WebSocket connections
	// This ensures the feature flag and license are checked before allowing non-default org connections
	hostedMode := os.Getenv("PULSE_HOSTED_MODE") == "true"
	wsHub.SetMultiTenantChecker(api.NewMultiTenantChecker(hostedMode))

	// Wire up Prometheus metrics for alert lifecycle
	alerts.SetMetricHooks(
		metrics.RecordAlertFired,
		metrics.RecordAlertResolved,
		metrics.RecordAlertSuppressed,
		metrics.RecordAlertAcknowledged,
	)
	log.Info().Msg("Alert metrics hooks registered")

	// Start monitoring
	reloadableMonitor.Start(ctx)

	// Initialize API server with reload function
	var router *api.Router
	reloadFunc := func() error {
		if err := reloadableMonitor.Reload(); err != nil {
			return err
		}
		if router != nil {
			router.SetMonitor(reloadableMonitor.GetMonitor())
			router.SetMultiTenantMonitor(reloadableMonitor.GetMultiTenantMonitor())
			if cfg := reloadableMonitor.GetConfig(); cfg != nil {
				router.SetConfig(cfg)
			}
		}
		return nil
	}
	router = api.NewRouter(cfg, reloadableMonitor.GetMonitor(), reloadableMonitor.GetMultiTenantMonitor(), wsHub, reloadFunc, version, conversionStore)

	// Inject resource store into monitor for WebSocket broadcasts
	router.SetMonitor(reloadableMonitor.GetMonitor())
	// Wire multi-tenant monitor to resource handlers for tenant-aware state
	router.SetMultiTenantMonitor(reloadableMonitor.GetMultiTenantMonitor())

	// Start AI patrol service for background infrastructure monitoring
	router.StartPatrol(ctx)

	// Start AI chat service
	router.StartAIChat(ctx)

	// Start hosted tenant reaper for automatic soft-delete cleanup
	if os.Getenv("PULSE_HOSTED_MODE") == "true" {
		reaper := hosted.NewReaper(mtPersistence, mtPersistence, 5*time.Minute, true)
		reaper.OnBeforeDelete = func(orgID string) error {
			return router.CleanupTenant(ctx, orgID)
		}
		go reaper.Run(ctx)
		log.Info().Msg("Hosted tenant reaper started")
	}

	// Start relay client for mobile remote access
	router.StartRelay(ctx)

	// Wire alert-triggered AI analysis
	router.WireAlertTriggeredAI()

	// Start anonymous telemetry (enabled by default; opt out via PULSE_TELEMETRY=false or Settings toggle).
	// Persistence is created once here (outside the closure) to avoid NewConfigPersistence's
	// fatal-on-error path running inside the telemetry goroutine.
	isDocker := os.Getenv("PULSE_DOCKER") == "true"
	telemetryPersistence := config.NewConfigPersistence(baseDataDir)
	telemetryCfg := telemetry.Config{
		Version:  version,
		DataDir:  baseDataDir,
		IsDocker: isDocker,
		Enabled:  cfg.TelemetryEnabled,
		GetSnapshot: func() telemetry.Snapshot {
			// Use the latest config (may have been swapped by a reload).
			currentCfg := cfg
			if reloaded := reloadableMonitor.GetConfig(); reloaded != nil {
				currentCfg = reloaded
			}

			snap := telemetry.Snapshot{
				MultiTenant: currentCfg.MultiTenantEnabled,
				APITokens:   len(currentCfg.APITokens),
				LicenseTier: "free",
			}

			// Resource counts from monitor state.
			if mon := reloadableMonitor.GetMonitor(); mon != nil {
				state := mon.GetState()
				snap.PVENodes = len(state.Nodes)
				snap.PBSInstances = len(state.PBSInstances)
				snap.PMGInstances = len(state.PMGInstances)
				snap.VMs = len(state.VMs)
				snap.Containers = len(state.Containers)
				snap.DockerHosts = len(state.DockerHosts)
				snap.KubernetesClusters = len(state.KubernetesClusters)
				snap.ActiveAlerts = len(state.ActiveAlerts)
			}

			// Feature flags from persisted config (using pre-created persistence).
			if aiCfg, err := telemetryPersistence.LoadAIConfig(); err == nil && aiCfg != nil {
				snap.AIEnabled = aiCfg.Enabled
			}
			if relayCfg, err := telemetryPersistence.LoadRelayConfig(); err == nil {
				snap.RelayEnabled = relayCfg.Enabled
			}

			// SSO/OIDC status.
			snap.SSOEnabled = currentCfg.OIDC != nil && currentCfg.OIDC.Enabled

			// License tier.
			if router != nil && router.GetLicenseHandlers() != nil {
				if svc := router.GetLicenseHandlers().Service(context.Background()); svc != nil {
					if lic := svc.Current(); lic != nil {
						snap.LicenseTier = string(lic.Claims.Tier)
					}
				}
			}

			return snap
		},
	}
	telemetry.Start(ctx, telemetryCfg)
	defer telemetry.Stop()

	// Wire live telemetry toggle so Settings changes take effect immediately.
	router.SetTelemetryToggleFunc(func(enabled bool) {
		if enabled {
			telemetryCfg.Enabled = true
			telemetry.Start(ctx, telemetryCfg)
			log.Info().Msg("Telemetry re-enabled via settings (live toggle)")
		} else {
			telemetry.Stop()
			log.Info().Msg("Telemetry disabled via settings (live toggle)")
		}
	})

	// Create HTTP server with unified configuration
	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.BindAddress, cfg.FrontendPort),
		Handler:           router.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      0, // Disabled to support SSE/streaming
		IdleTimeout:       120 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	// Start config watcher for .env file changes
	configWatcher, err := config.NewConfigWatcher(cfg)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create config watcher, .env changes will require restart")
	} else {
		configWatcher.SetMockReloadCallback(func() {
			log.Info().Msg("mock.env changed, reloading monitor")
			if err := reloadableMonitor.Reload(); err != nil {
				log.Error().Err(err).Msg("Failed to reload monitor after mock.env change")
			} else if router != nil {
				router.SetMonitor(reloadableMonitor.GetMonitor())
				router.SetMultiTenantMonitor(reloadableMonitor.GetMultiTenantMonitor())
				if cfg := reloadableMonitor.GetConfig(); cfg != nil {
					router.SetConfig(cfg)
				}
			}
		})

		configWatcher.SetAPITokenReloadCallback(func() {
			if monitor := reloadableMonitor.GetMonitor(); monitor != nil {
				monitor.RebuildTokenBindings()
			}
		})

		if err := configWatcher.Start(); err != nil {
			log.Warn().Err(err).Msg("Failed to start config watcher")
		}
		defer configWatcher.Stop()
	}

	// Start HTTP→HTTPS redirect server when HTTPS is active and redirect port is configured.
	var redirectSrv *http.Server
	if cfg.HTTPSEnabled && cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" && cfg.HTTPRedirectPort > 0 {
		if cfg.HTTPRedirectPort == cfg.FrontendPort {
			log.Error().
				Int("redirect_port", cfg.HTTPRedirectPort).
				Int("frontend_port", cfg.FrontendPort).
				Msg("HTTP_REDIRECT_PORT must differ from FRONTEND_PORT; skipping redirect server")
		} else {
			httpsPort := cfg.FrontendPort
			redirectSrv = &http.Server{
				Addr: fmt.Sprintf("%s:%d", cfg.BindAddress, cfg.HTTPRedirectPort),
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Host == "" {
						http.Error(w, "Bad Request", http.StatusBadRequest)
						return
					}
					// Extract the hostname, stripping any port. net.SplitHostPort
					// handles IPv6 bracket notation (e.g. [::1]:80 → "::1").
					// If SplitHostPort fails, the Host has no port — strip any
					// stray brackets that a bare IPv6 literal may carry.
					host := r.Host
					if h, _, err := net.SplitHostPort(host); err == nil {
						host = h
					} else {
						host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
					}
					// Rebuild the authority with the HTTPS port.
					// net.JoinHostPort handles IPv6 bracketing automatically.
					var authority string
					if httpsPort != 443 {
						authority = net.JoinHostPort(host, strconv.Itoa(httpsPort))
					} else if strings.Contains(host, ":") {
						authority = "[" + host + "]"
					} else {
						authority = host
					}
					target := "https://" + authority + r.URL.RequestURI()
					http.Redirect(w, r, target, http.StatusMovedPermanently)
				}),
				ReadHeaderTimeout: 5 * time.Second,
			}
			go func() {
				log.Info().
					Str("host", cfg.BindAddress).
					Int("port", cfg.HTTPRedirectPort).
					Int("https_port", httpsPort).
					Msg("HTTP→HTTPS redirect server listening")
				if err := redirectSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Error().Err(err).Msg("Failed to start HTTP redirect server")
				}
			}()
		}
	}

	// Start server
	go func() {
		if cfg.HTTPSEnabled && cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			log.Info().
				Str("host", cfg.BindAddress).
				Int("port", cfg.FrontendPort).
				Str("protocol", "HTTPS").
				Msg("Server listening")
			if err := srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("Failed to start HTTPS server")
			}
		} else {
			if cfg.HTTPSEnabled {
				log.Warn().Msg("HTTPS_ENABLED is true but TLS_CERT_FILE or TLS_KEY_FILE not configured, falling back to HTTP")
			}
			log.Info().
				Str("host", cfg.BindAddress).
				Int("port", cfg.FrontendPort).
				Str("protocol", "HTTP").
				Msg("Server listening")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("Failed to start HTTP server")
			}
		}
	}()

	// Setup signal handlers
	sigChan := make(chan os.Signal, 1)
	reloadChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	signal.Notify(reloadChan, syscall.SIGHUP)
	defer signal.Stop(sigChan)
	defer signal.Stop(reloadChan)

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Context cancelled, shutting down...")
			goto shutdown

		case <-reloadChan:
			log.Info().Msg("Received SIGHUP, reloading configuration...")
			if configWatcher != nil {
				configWatcher.ReloadConfig()
			}

			if err := reloadFunc(); err != nil {
				log.Error().Err(err).Msg("Failed to reload monitor after SIGHUP")
			} else {
				log.Info().Msg("Runtime configuration reloaded")
			}

		case <-sigChan:
			log.Info().Msg("Shutting down server...")
			goto shutdown
		}
	}

shutdown:
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if redirectSrv != nil {
		if err := redirectSrv.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("HTTP redirect server shutdown error")
		}
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	}

	// Stop license grant refresh loops
	router.StopGrantRefresh()

	// Gracefully stop AI intelligence services (patrol, investigations, triggers)
	router.ShutdownAIIntelligence()

	// Stop relay client
	router.StopRelay()

	// Stop AI chat service (kills sidecar process group)
	router.StopAIChat(shutdownCtx)

	// Ensure mock-mode background update ticker is stopped before process exit.
	if mock.IsMockEnabled() {
		mock.SetEnabled(false)
	}

	cancel()
	reloadableMonitor.Stop()

	if configWatcher != nil {
		configWatcher.Stop()
	}

	// Close tenant audit loggers
	tenantAuditManager.Close()
	if err := audit.Close(); err != nil {
		log.Error().Err(err).Msg("Failed to close audit logger")
	}
	closeConversionStore()

	log.Info().Msg("Server stopped")
	return nil
}

// startMetricsServer starts the Prometheus /metrics endpoint. When metricsToken
// is non-empty, requests must include a matching Authorization: Bearer <token> header.
func startMetricsServer(ctx context.Context, addr string, metricsToken string) {
	handler := promhttp.Handler()
	if metricsToken != "" {
		inner := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) || auth[len(prefix):] != metricsToken {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			inner.ServeHTTP(w, r)
		})
		log.Info().Msg("Metrics endpoint requires bearer token authentication")
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", handler)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("Metrics server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Metrics server failed")
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
}

func ShouldAutoImport() bool {
	configPath := os.Getenv("PULSE_DATA_DIR")
	if configPath == "" {
		configPath = "/etc/pulse"
	}

	if _, err := os.Stat(filepath.Join(configPath, "nodes.enc")); err == nil {
		return false
	}

	return os.Getenv("PULSE_INIT_CONFIG_DATA") != "" ||
		os.Getenv("PULSE_INIT_CONFIG_FILE") != ""
}

func PerformAutoImport() error {
	configData := os.Getenv("PULSE_INIT_CONFIG_DATA")
	configFile := os.Getenv("PULSE_INIT_CONFIG_FILE")
	configPass := os.Getenv("PULSE_INIT_CONFIG_PASSPHRASE")
	source := "none"
	if configFile != "" {
		source = "file"
	} else if configData != "" {
		source = "env_data"
	}

	logAudit := func(success bool, reason string) {
		details := "source=" + source
		if reason != "" {
			details += " reason=" + reason
		}
		audit.Log("config_auto_import", "system", "", "/startup/auto-import", success, details)
	}

	if configPass == "" {
		logAudit(false, "missing_passphrase")
		return fmt.Errorf("PULSE_INIT_CONFIG_PASSPHRASE is required for auto-import")
	}

	var encryptedData string

	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			logAudit(false, "read_config_file_failed")
			return fmt.Errorf("failed to read config file: %w", err)
		}
		payload, err := NormalizeImportPayload(data)
		if err != nil {
			logAudit(false, "normalize_payload_failed")
			return err
		}
		encryptedData = payload
	} else if configData != "" {
		payload, err := NormalizeImportPayload([]byte(configData))
		if err != nil {
			logAudit(false, "normalize_payload_failed")
			return err
		}
		encryptedData = payload
	} else {
		logAudit(false, "missing_payload")
		return fmt.Errorf("no config data provided")
	}

	configPath := os.Getenv("PULSE_DATA_DIR")
	if configPath == "" {
		configPath = "/etc/pulse"
	}

	persistence := config.NewConfigPersistence(configPath)
	if err := persistence.ImportConfig(encryptedData, configPass); err != nil {
		logAudit(false, "import_failed")
		return fmt.Errorf("failed to import configuration: %w", err)
	}

	logAudit(true, "")
	log.Info().Msg("Configuration auto-imported successfully")
	return nil
}

func NormalizeImportPayload(raw []byte) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "", fmt.Errorf("configuration payload is empty")
	}

	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
		decodedTrimmed := strings.TrimSpace(string(decoded))
		if LooksLikeBase64(decodedTrimmed) {
			return decodedTrimmed, nil
		}
		return trimmed, nil
	}

	return base64.StdEncoding.EncodeToString(raw), nil
}

func LooksLikeBase64(s string) bool {
	if s == "" {
		return false
	}
	compact := strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		default:
			return r
		}
	}, s)

	if compact == "" || len(compact)%4 != 0 {
		return false
	}
	for i := 0; i < len(compact); i++ {
		c := compact[i]
		isAlphaNum := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		if isAlphaNum || c == '+' || c == '/' || c == '=' {
			continue
		}
		return false
	}
	return true
}

func ensureDefaultOrgOwnerMembership(mtp *config.MultiTenantPersistence, adminUser string) error {
	if mtp == nil {
		return nil
	}
	adminUser = strings.TrimSpace(adminUser)
	if adminUser == "" {
		return nil
	}

	org, err := mtp.LoadOrganization("default")
	if err != nil {
		return fmt.Errorf("load default organization: %w", err)
	}
	if org == nil {
		org = &models.Organization{}
	}

	changed := false
	now := time.Now().UTC()
	if strings.TrimSpace(org.ID) == "" {
		org.ID = "default"
		changed = true
	}
	if strings.TrimSpace(org.DisplayName) == "" {
		org.DisplayName = "default"
		changed = true
	}
	if org.CreatedAt.IsZero() {
		org.CreatedAt = now
		changed = true
	}
	if strings.TrimSpace(org.OwnerUserID) == "" {
		org.OwnerUserID = adminUser
		changed = true
	}

	if ensureOrgOwnerMembership(org, adminUser, now) {
		changed = true
	}

	if ownerUserID := strings.TrimSpace(org.OwnerUserID); ownerUserID != "" && ownerUserID != adminUser {
		if ensureOrgOwnerMembership(org, ownerUserID, now) {
			changed = true
		}
	}

	if !changed {
		return nil
	}
	return mtp.SaveOrganization(org)
}

func ensureOrgOwnerMembership(org *models.Organization, userID string, now time.Time) bool {
	userID = strings.TrimSpace(userID)
	if org == nil || userID == "" {
		return false
	}

	for i := range org.Members {
		if strings.TrimSpace(org.Members[i].UserID) != userID {
			continue
		}
		changed := false
		if org.Members[i].Role != models.OrgRoleOwner {
			org.Members[i].Role = models.OrgRoleOwner
			changed = true
		}
		if org.Members[i].AddedAt.IsZero() {
			org.Members[i].AddedAt = now
			changed = true
		}
		if strings.TrimSpace(org.Members[i].AddedBy) == "" {
			org.Members[i].AddedBy = userID
			changed = true
		}
		return changed
	}

	org.Members = append(org.Members, models.OrganizationMember{
		UserID:  userID,
		Role:    models.OrgRoleOwner,
		AddedAt: now,
		AddedBy: userID,
	})
	return true
}
