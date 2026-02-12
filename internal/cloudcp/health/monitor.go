package health

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

// MonitorConfig holds health monitor settings.
type MonitorConfig struct {
	Interval      time.Duration // how often to check (default 60s)
	RestartOnFail bool          // restart unhealthy containers
	FailThreshold int           // consecutive failures before restart (default 3)
}

// Monitor periodically health-checks active tenant containers and optionally
// restarts unhealthy ones.
type Monitor struct {
	registry *registry.TenantRegistry
	docker   *docker.Manager
	cfg      MonitorConfig
}

// NewMonitor creates a health monitor.
func NewMonitor(reg *registry.TenantRegistry, mgr *docker.Manager, cfg MonitorConfig) *Monitor {
	if cfg.Interval == 0 {
		cfg.Interval = 60 * time.Second
	}
	if cfg.FailThreshold == 0 {
		cfg.FailThreshold = 3
	}
	return &Monitor{
		registry: reg,
		docker:   mgr,
		cfg:      cfg,
	}
}

// Run starts the health check loop. It blocks until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context) {
	log.Info().
		Dur("interval", m.cfg.Interval).
		Bool("restart_on_fail", m.cfg.RestartOnFail).
		Msg("Health monitor started")

	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Health monitor stopped")
			return
		case <-ticker.C:
			m.checkAll(ctx)
		}
	}
}

func (m *Monitor) checkAll(ctx context.Context) {
	tenants, err := m.registry.ListByState(registry.TenantStateActive)
	if err != nil {
		log.Error().Err(err).Msg("Health monitor: failed to list active tenants")
		return
	}

	for _, tenant := range tenants {
		if ctx.Err() != nil {
			return
		}
		if tenant.ContainerID == "" {
			continue
		}

		healthy, err := m.docker.HealthCheck(ctx, tenant.ContainerID)
		if err != nil {
			log.Warn().Err(err).
				Str("tenant_id", tenant.ID).
				Str("container_id", tenant.ContainerID).
				Msg("Health check error")
		}

		now := time.Now().UTC()
		tenant.LastHealthCheck = &now
		tenant.HealthCheckOK = healthy

		if healthy {
			cpmetrics.HealthCheckResults.WithLabelValues("healthy").Inc()
		} else {
			cpmetrics.HealthCheckResults.WithLabelValues("unhealthy").Inc()
		}

		if err := m.registry.Update(tenant); err != nil {
			log.Error().
				Err(err).
				Str("tenant_id", tenant.ID).
				Str("container_id", tenant.ContainerID).
				Msg("Failed to update health status")
			continue
		}

		if !healthy && m.cfg.RestartOnFail {
			log.Warn().
				Str("tenant_id", tenant.ID).
				Str("container_id", tenant.ContainerID).
				Msg("Container unhealthy, attempting restart")

			if err := m.docker.Stop(ctx, tenant.ContainerID); err != nil {
				log.Error().
					Err(err).
					Str("tenant_id", tenant.ID).
					Str("container_id", tenant.ContainerID).
					Msg("Failed to stop unhealthy container")
			}
			// Docker restart policy (unless-stopped) will restart the container
		}
	}
}
