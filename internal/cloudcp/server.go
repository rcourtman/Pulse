package cloudcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/email"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/health"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	cpstripe "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/stripe"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rs/zerolog/log"
)

// Run starts the control plane HTTP server with graceful shutdown.
func Run(ctx context.Context, version string) error {
	logging.Init(logging.Config{
		Format:    "auto",
		Level:     "info",
		Component: "control-plane",
	})

	log.Info().Str("version", version).Msg("Starting Pulse Cloud Control Plane")

	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Ensure data directories exist
	if err := os.MkdirAll(cfg.TenantsDir(), 0o755); err != nil {
		return fmt.Errorf("create tenants dir: %w", err)
	}
	if err := os.MkdirAll(cfg.ControlPlaneDir(), 0o755); err != nil {
		return fmt.Errorf("create control-plane dir: %w", err)
	}

	// Open tenant registry
	reg, err := registry.NewTenantRegistry(cfg.ControlPlaneDir())
	if err != nil {
		return fmt.Errorf("open tenant registry: %w", err)
	}
	defer reg.Close()

	// Initialize Docker manager (best-effort — control plane can run without Docker for dev/testing)
	var dockerMgr *cpDocker.Manager
	dockerMgr, err = cpDocker.NewManager(cpDocker.ManagerConfig{
		Image:       cfg.PulseImage,
		Network:     cfg.DockerNetwork,
		BaseDomain:  baseDomainFromURL(cfg.BaseURL),
		MemoryLimit: cfg.TenantMemoryLimit,
		CPUShares:   cfg.TenantCPUShares,
	})
	if err != nil {
		log.Warn().Err(err).Msg("Docker unavailable — container management disabled")
		dockerMgr = nil
	} else {
		defer dockerMgr.Close()
	}

	// Initialize magic link service
	magicLinkSvc, err := cpauth.NewService(cfg.ControlPlaneDir())
	if err != nil {
		return fmt.Errorf("init magic link service: %w", err)
	}
	defer magicLinkSvc.Close()

	// Initialize email sender
	var emailSender email.Sender
	if cfg.ResendAPIKey != "" {
		emailSender = email.NewResendSender(cfg.ResendAPIKey)
		log.Info().Msg("Email sender configured (Resend)")
	} else {
		emailSender = email.NewLogSender(func(to, subject, body string) {
			const maxBody = 4096
			bodyForLog := body
			if len(bodyForLog) > maxBody {
				bodyForLog = bodyForLog[:maxBody] + "...(truncated)"
			}
			log.Info().
				Str("to", to).
				Str("subject", subject).
				Str("body", bodyForLog).
				Msg("Email (log-only, no email provider configured)")
		})
		log.Info().Msg("Email sender: log-only (set RESEND_API_KEY to enable)")
	}

	// Build HTTP routes
	mux := http.NewServeMux()
	deps := &Deps{
		Config:      cfg,
		Registry:    reg,
		Docker:      dockerMgr,
		MagicLinks:  magicLinkSvc,
		Version:     version,
		EmailSender: emailSender,
	}
	RegisterRoutes(mux, deps)

	addr := fmt.Sprintf("%s:%d", cfg.BindAddress, cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Create derived context for background goroutines
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start health monitor if Docker is available
	if dockerMgr != nil {
		monitor := health.NewMonitor(reg, dockerMgr, health.MonitorConfig{
			Interval:      60 * time.Second,
			RestartOnFail: true,
			FailThreshold: 3,
		})
		go monitor.Run(ctx)
	}

	// Start grace period enforcer
	graceEnforcer := cpstripe.NewGraceEnforcer(reg)
	go graceEnforcer.Run(ctx)

	// Start stuck provisioning cleanup
	stuckCleanup := health.NewStuckProvisioningCleanup(reg)
	go stuckCleanup.Run(ctx)

	// Start metrics updater
	go runTenantStateMetrics(ctx, reg)

	// Start server in background
	go func() {
		log.Info().Str("addr", addr).Msg("Control plane listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Server failed")
		}
	}()

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	select {
	case <-ctx.Done():
		log.Info().Msg("Context cancelled, shutting down...")
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received signal, shutting down...")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	}

	cancel()
	log.Info().Msg("Control plane stopped")
	return nil
}

// baseDomainFromURL extracts a base domain from a URL like "https://cloud.pulserelay.pro".
func baseDomainFromURL(baseURL string) string {
	// Strip scheme
	domain := baseURL
	for _, prefix := range []string{"https://", "http://"} {
		if len(domain) > len(prefix) && domain[:len(prefix)] == prefix {
			domain = domain[len(prefix):]
			break
		}
	}
	// Strip port and path
	for i := 0; i < len(domain); i++ {
		if domain[i] == ':' || domain[i] == '/' {
			domain = domain[:i]
			break
		}
	}
	return domain
}
