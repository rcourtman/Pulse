package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// Version information (set at build time with -ldflags)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "pulse",
	Short: "Pulse - Proxmox VE and PBS monitoring system",
	Long:  `Pulse is a real-time monitoring system for Proxmox Virtual Environment (PVE) and Proxmox Backup Server (PBS)`,
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
	// Initialize logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

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

	log.Info().Msg("Starting Pulse monitoring server")

	// Create context that cancels on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	wsHub.SetStateGetter(func() interface{} {
		return reloadableMonitor.GetState()
	})

	// Start monitoring
	reloadableMonitor.Start(ctx)

	// Initialize API server with reload function
	reloadFunc := func() error {
		return reloadableMonitor.Reload()
	}
	router := api.NewRouter(cfg, reloadableMonitor.GetMonitor(), wsHub, reloadFunc)

	// Create HTTP server with unified configuration
	// In production, serve everything (frontend + API) on the frontend port
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.BackendHost, cfg.FrontendPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start config watcher for .env file changes
	configWatcher, err := config.NewConfigWatcher(cfg)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create config watcher, .env changes will require restart")
	} else {
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
			
			// Reload system.json
			persistence := config.NewConfigPersistence(cfg.DataPath)
			if persistence != nil {
				if sysConfig, err := persistence.LoadSystemSettings(); err == nil {
					// Update polling interval if changed
					if sysConfig.PollingInterval > 0 {
						oldInterval := cfg.PollingInterval
						cfg.PollingInterval = time.Duration(sysConfig.PollingInterval) * time.Second
						if cfg.PollingInterval != oldInterval {
							log.Info().
								Dur("old", oldInterval).
								Dur("new", cfg.PollingInterval).
								Msg("Polling interval updated")
							// Update monitor's polling interval
							if reloadableMonitor != nil {
								reloadableMonitor.UpdatePollingInterval(cfg.PollingInterval)
							}
						}
					}
					// Could reload other system.json settings here
					log.Info().Msg("Reloaded system configuration")
				} else {
					log.Error().Err(err).Msg("Failed to reload system.json")
				}
			}
			
			// Could reload other configs here (alerts.json, webhooks.json, etc.)
			
			log.Info().Msg("Configuration reload complete")
			
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
		encryptedData = string(data)
	} else if configData != "" {
		// Try to decode base64 if it looks encoded
		if decoded, err := base64.StdEncoding.DecodeString(configData); err == nil {
			encryptedData = string(decoded)
		} else {
			encryptedData = configData
		}
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
