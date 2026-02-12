package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentbinaries"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rcourtman/pulse-go-rewrite/internal/hosted"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/conversion"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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
	license.InitPublicKey()

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

	// Conversion telemetry must be durable and tenant-aware (P0-6).
	conversionStore, err := conversion.NewConversionStore(filepath.Join(baseDataDir, "conversion.db"))
	if err != nil {
		return fmt.Errorf("failed to initialize conversion telemetry store: %w", err)
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
	agentbinaries.EnsureHostAgentBinaries(version)

	// Create derived context that cancels on interrupt
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Metrics port is configurable via MetricsPort variable
	metricsAddr := fmt.Sprintf("%s:%d", cfg.BindAddress, MetricsPort)
	startMetricsServer(ctx, metricsAddr)

	// Initialize WebSocket hub first
	wsHub := websocket.NewHub(nil)
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

	// Initialize reloadable monitoring system
	reloadableMonitor, err := monitoring.NewReloadableMonitor(cfg, mtPersistence, wsHub)
	if err != nil {
		return fmt.Errorf("failed to initialize monitoring system: %w", err)
	}

	// Trigger enterprise hooks if registered
	globalHooksMu.Lock()
	onMetricsStoreReady := globalHooks.OnMetricsStoreReady
	globalHooksMu.Unlock()

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
		mtMonitor := reloadableMonitor.GetMultiTenantMonitor()
		if mtMonitor == nil {
			// Fall back to default monitor
			state := reloadableMonitor.GetMonitor().GetState()
			return state.ToFrontend()
		}
		monitor, err := mtMonitor.GetMonitor(orgID)
		if err != nil || monitor == nil {
			// Fall back to default monitor on error
			log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to get tenant monitor, using default")
			state := reloadableMonitor.GetMonitor().GetState()
			return state.ToFrontend()
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

	// Create HTTP server with unified configuration
	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.BindAddress, cfg.FrontendPort),
		Handler:           router.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      0, // Disabled to support SSE/streaming
		IdleTimeout:       120 * time.Second,
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

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	}

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

// startMetricsServer is moved from main.go
func startMetricsServer(ctx context.Context, addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

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

	if configPass == "" {
		return fmt.Errorf("PULSE_INIT_CONFIG_PASSPHRASE is required for auto-import")
	}

	var encryptedData string

	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		payload, err := NormalizeImportPayload(data)
		if err != nil {
			return err
		}
		encryptedData = payload
	} else if configData != "" {
		payload, err := NormalizeImportPayload([]byte(configData))
		if err != nil {
			return err
		}
		encryptedData = payload
	} else {
		return fmt.Errorf("no config data provided")
	}

	configPath := os.Getenv("PULSE_DATA_DIR")
	if configPath == "" {
		configPath = "/etc/pulse"
	}

	persistence := config.NewConfigPersistence(configPath)
	if err := persistence.ImportConfig(encryptedData, configPass); err != nil {
		return fmt.Errorf("failed to import configuration: %w", err)
	}

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
