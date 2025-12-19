package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentbinaries"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
	_ "github.com/rcourtman/pulse-go-rewrite/internal/mock" // Import for init() to run
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// Version information (set at build time with -ldflags)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

const metricsPort = 9091

var rootCmd = &cobra.Command{
	Use:     "pulse",
	Short:   "Pulse - Proxmox VE and PBS monitoring system",
	Long:    `Pulse is a real-time monitoring system for Proxmox Virtual Environment (PVE) and Proxmox Backup Server (PBS)`,
	Version: Version,
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

func init() {
	// Add config command
	rootCmd.AddCommand(configCmd)
	// Add version command
	rootCmd.AddCommand(versionCmd)
	// Add bootstrap-token command
	rootCmd.AddCommand(bootstrapTokenCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Pulse %s\n", Version)
		if BuildTime != "unknown" {
			fmt.Printf("Built: %s\n", BuildTime)
		}
		if GitCommit != "unknown" {
			fmt.Printf("Commit: %s\n", GitCommit)
		}
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runServer() {
	// Initialize logger with baseline defaults for early startup logs
	logging.Init(logging.Config{
		Format:    "auto",
		Level:     "info",
		Component: "pulse",
	})

	// Check for auto-import on first startup
	if shouldAutoImport() {
		if err := performAutoImport(); err != nil {
			log.Error().Err(err).Msg("Auto-import failed, continuing with normal startup")
		}
	}

	// Load unified configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Re-initialize logging with configuration-driven settings
	logging.Init(logging.Config{
		Format:    cfg.LogFormat,
		Level:     cfg.LogLevel,
		Component: "pulse",
	})

	// Initialize license public key for Pro feature validation
	license.InitPublicKey()

	log.Info().Msg("Starting Pulse monitoring server")

	// Validate agent binaries are available for download
	agentbinaries.EnsureHostAgentBinaries(Version)

	// Create context that cancels on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	metricsAddr := fmt.Sprintf("%s:%d", cfg.BackendHost, metricsPort)
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
		// This allows the WebSocket hub to use its lenient check for local/private networks
		// The hub will automatically allow connections from common local/Docker scenarios
		// while still being secure for public deployments
		wsHub.SetAllowedOrigins([]string{})
	}
	go wsHub.Run()

	// Initialize reloadable monitoring system
	reloadableMonitor, err := monitoring.NewReloadableMonitor(cfg, wsHub)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize monitoring system")
	}

	// Set state getter for WebSocket hub
	// IMPORTANT: Return StateFrontend (not StateSnapshot) to match broadcast format.
	// StateSnapshot uses time.Time fields while StateFrontend uses Unix timestamps,
	// and includes frontend-specific field transformations. Without this conversion,
	// nodes/hosts would be missing on initial page load but appear after broadcasts.
	wsHub.SetStateGetter(func() interface{} {
		// GetMonitor().GetState() returns models.StateSnapshot
		state := reloadableMonitor.GetMonitor().GetState()
		// Convert to frontend format, matching what BroadcastState does
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
	router = api.NewRouter(cfg, reloadableMonitor.GetMonitor(), wsHub, reloadFunc, Version)

	// Inject resource store into monitor for WebSocket broadcasts
	// This must be done after router creation since resourceHandlers is created in NewRouter
	router.SetMonitor(reloadableMonitor.GetMonitor())

	// Start AI patrol service for background infrastructure monitoring
	router.StartPatrol(ctx)

	// Wire alert-triggered AI analysis (token-efficient real-time insights when alerts fire)
	router.WireAlertTriggeredAI()

	// Create HTTP server with unified configuration
	// In production, serve everything (frontend + API) on the frontend port
	// NOTE: We use ReadHeaderTimeout instead of ReadTimeout to avoid affecting
	// WebSocket connections. ReadTimeout sets a deadline on the underlying connection
	// that persists even after WebSocket upgrade, causing premature disconnections.
	// ReadHeaderTimeout only applies during header reading, not the full request body.
	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.BackendHost, cfg.FrontendPort),
		Handler:           router.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      0, // Disabled to support SSE/streaming - each handler manages its own deadline
		IdleTimeout:       120 * time.Second,
	}

	// Start config watcher for .env file changes
	configWatcher, err := config.NewConfigWatcher(cfg)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create config watcher, .env changes will require restart")
	} else {
		// Set callback to reload monitor when mock.env changes
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

		// Set callback to rebuild token bindings when API tokens are reloaded from disk.
		// This fixes issue #773 where agent token bindings become orphaned after config reload.
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
				log.Fatal().Err(err).Msg("Failed to start HTTPS server")
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
				log.Fatal().Err(err).Msg("Failed to start HTTP server")
			}
		}
	}()

	// Setup signal handlers
	sigChan := make(chan os.Signal, 1)
	reloadChan := make(chan os.Signal, 1)

	// SIGTERM and SIGINT for shutdown
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	// SIGHUP for config reload
	signal.Notify(reloadChan, syscall.SIGHUP)

	// Handle signals
	for {
		select {
		case <-reloadChan:
			log.Info().Msg("Received SIGHUP, reloading configuration...")

			// Reload .env manually (watcher will also pick it up)
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

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	}

	// Stop monitoring
	cancel()
	reloadableMonitor.Stop()

	// Stop config watcher
	if configWatcher != nil {
		configWatcher.Stop()
	}

	log.Info().Msg("Server stopped")
}

// shouldAutoImport checks if auto-import environment variables are set
func shouldAutoImport() bool {
	// Check if config already exists
	configPath := os.Getenv("PULSE_DATA_DIR")
	if configPath == "" {
		configPath = "/etc/pulse"
	}

	// If nodes.enc already exists, skip auto-import
	if _, err := os.Stat(filepath.Join(configPath, "nodes.enc")); err == nil {
		return false
	}

	// Check for auto-import environment variables
	return os.Getenv("PULSE_INIT_CONFIG_DATA") != "" ||
		os.Getenv("PULSE_INIT_CONFIG_FILE") != ""
}

// performAutoImport imports configuration from environment variables
func performAutoImport() error {
	configData := os.Getenv("PULSE_INIT_CONFIG_DATA")
	configFile := os.Getenv("PULSE_INIT_CONFIG_FILE")
	configPass := os.Getenv("PULSE_INIT_CONFIG_PASSPHRASE")

	if configPass == "" {
		return fmt.Errorf("PULSE_INIT_CONFIG_PASSPHRASE is required for auto-import")
	}

	var encryptedData string

	// Get data from file or direct data
	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		payload, err := normalizeImportPayload(data)
		if err != nil {
			return err
		}
		encryptedData = payload
	} else if configData != "" {
		payload, err := normalizeImportPayload([]byte(configData))
		if err != nil {
			return err
		}
		encryptedData = payload
	} else {
		return fmt.Errorf("no config data provided")
	}

	// Load configuration path
	configPath := os.Getenv("PULSE_DATA_DIR")
	if configPath == "" {
		configPath = "/etc/pulse"
	}

	// Create persistence manager
	persistence := config.NewConfigPersistence(configPath)

	// Import configuration
	if err := persistence.ImportConfig(encryptedData, configPass); err != nil {
		return fmt.Errorf("failed to import configuration: %w", err)
	}

	log.Info().Msg("Configuration auto-imported successfully")
	return nil
}
