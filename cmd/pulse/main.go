package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
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

var rootCmd = &cobra.Command{
	Use:   "pulse",
	Short: "Pulse - Proxmox VE and PBS monitoring system",
	Long:  `Pulse is a real-time monitoring system for Proxmox Virtual Environment (PVE) and Proxmox Backup Server (PBS)`,
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

func init() {
	// Add config command
	rootCmd.AddCommand(configCmd)
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
	if cfg.AllowedOrigins != "" && cfg.AllowedOrigins != "*" {
		wsHub.SetAllowedOrigins(strings.Split(cfg.AllowedOrigins, ","))
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
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.BackendHost, cfg.BackendPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	go func() {
		log.Info().
			Str("host", cfg.BackendHost).
			Int("port", cfg.BackendPort).
			Msg("Server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info().Msg("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	}

	// Stop monitoring
	cancel()
	reloadableMonitor.Stop()

	log.Info().Msg("Server stopped")
}