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
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	_ "github.com/rcourtman/pulse-go-rewrite/internal/mock" // Import for init() to run
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

	// Create license service early for feature checks
	licenseService := license.NewService()

	// Initialize RBAC manager for role-based access control
	dataDir := os.Getenv("PULSE_DATA_DIR")
	if dataDir == "" {
		dataDir = "/etc/pulse"
	}

	// Use SQLite-backed manager for Pro features, file-based for Community
	if licenseService.HasFeature(license.FeatureRBAC) {
		sqliteManager, err := auth.NewSQLiteManager(auth.SQLiteManagerConfig{
			DataDir:          dataDir,
			MigrateFromFiles: true, // Automatically migrate from file-based storage
		})
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize SQLite RBAC manager, falling back to file-based")
			// Fall back to file-based manager
			fileManager, fileErr := auth.NewFileManager(dataDir)
			if fileErr != nil {
				log.Warn().Err(fileErr).Msg("Failed to initialize file-based RBAC manager")
			} else {
				auth.SetManager(fileManager)
				log.Info().Msg("RBAC manager initialized (file-based fallback)")
			}
		} else {
			auth.SetManager(sqliteManager)
			// Set up RBAC authorizer with policy evaluation
			rbacAuthorizer := auth.NewRBACAuthorizer(sqliteManager)
			auth.SetAuthorizer(rbacAuthorizer)
			log.Info().Msg("RBAC Pro manager initialized with SQLite backend (deny policies, inheritance, changelog)")
		}
	} else {
		rbacManager, err := auth.NewFileManager(dataDir)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize RBAC manager, role management will be unavailable")
		} else {
			auth.SetManager(rbacManager)
			log.Info().Msg("RBAC manager initialized (file-based)")
		}
	}

	// Initialize SQLite audit logger for Pro/Enterprise
	// Check if audit logging feature is available via mock mode or license
	if licenseService.HasFeature(license.FeatureAuditLogging) {
		// Initialize crypto manager for signing key encryption
		cryptoMgr, err := crypto.NewCryptoManagerAt(dataDir)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize crypto manager for audit logging")
		} else {
			auditLogger, err := audit.NewSQLiteLogger(audit.SQLiteLoggerConfig{
				DataDir:       dataDir,
				CryptoMgr:     cryptoMgr,
				RetentionDays: 90, // Default 90 day retention
			})
			if err != nil {
				log.Warn().Err(err).Msg("Failed to initialize SQLite audit logger, using console logger")
			} else {
				audit.SetLogger(auditLogger)
				log.Info().Msg("Enterprise audit logging enabled with SQLite backend")
				// Ensure cleanup on shutdown
				defer func() {
					if err := auditLogger.Close(); err != nil {
						log.Error().Err(err).Msg("Failed to close audit logger")
					}
				}()
			}
		}
	} else {
		log.Debug().Msg("Audit logging feature not licensed, using console logger")
	}

	// Initialize AI approval store for Auto-Fix workflows (Pro feature)
	if licenseService.HasFeature(license.FeatureAIAutoFix) {
		approvalStore, err := approval.NewStore(approval.StoreConfig{
			DataDir:        dataDir,
			DefaultTimeout: 5 * time.Minute,
			MaxApprovals:   1000,
		})
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize AI approval store")
		} else {
			approval.SetStore(approvalStore)
			approvalStore.StartCleanup(ctx)
			log.Info().Msg("AI approval store initialized for Auto-Fix workflows")
		}
	}

	log.Info().Msg("Starting Pulse monitoring server")

	// Validate agent binaries are available for download
	agentbinaries.EnsureHostAgentBinaries(version)

	// Create derived context that cancels on interrupt
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Metrics port is configurable via MetricsPort variable
	metricsAddr := fmt.Sprintf("%s:%d", cfg.BackendHost, MetricsPort)
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
	reloadableMonitor, err := monitoring.NewReloadableMonitor(cfg, wsHub)
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

	// Set state getter for WebSocket hub
	wsHub.SetStateGetter(func() interface{} {
		state := reloadableMonitor.GetMonitor().GetState()
		return state.ToFrontend()
	})

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

	// Initialize reporting engine for Pro/Enterprise
	if licenseService.HasFeature(license.FeatureAdvancedReporting) {
		if store := reloadableMonitor.GetMonitor().GetMetricsStore(); store != nil {
			reportEngine := reporting.NewReportEngine(reporting.EngineConfig{
				MetricsStore: store,
			})
			reporting.SetEngine(reportEngine)
			log.Info().Msg("Advanced reporting engine initialized")
		} else {
			log.Warn().Msg("Metrics store not available, reporting engine not initialized")
		}
	} else {
		log.Debug().Msg("Advanced reporting feature not licensed")
	}

	// Initialize API server with reload function
	var router *api.Router
	reloadFunc := func() error {
		if err := reloadableMonitor.Reload(); err != nil {
			return err
		}
		if router != nil {
			router.SetMonitor(reloadableMonitor.GetMonitor())
			if cfg := reloadableMonitor.GetConfig(); cfg != nil {
				router.SetConfig(cfg)
			}
		}
		return nil
	}
	router = api.NewRouter(cfg, reloadableMonitor.GetMonitor(), wsHub, reloadFunc, version)

	// Inject resource store into monitor for WebSocket broadcasts
	router.SetMonitor(reloadableMonitor.GetMonitor())

	// Start AI patrol service for background infrastructure monitoring
	router.StartPatrol(ctx)

	// Start OpenCode-powered AI chat service
	router.StartOpenCode(ctx)

	// Wire alert-triggered AI analysis
	router.WireAlertTriggeredAI()

	// Create HTTP server with unified configuration
	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.BackendHost, cfg.FrontendPort),
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
				Str("host", cfg.BackendHost).
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
				Str("host", cfg.BackendHost).
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

	cancel()
	reloadableMonitor.Stop()

	if configWatcher != nil {
		configWatcher.Stop()
	}

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
