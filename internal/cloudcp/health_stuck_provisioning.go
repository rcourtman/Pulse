package cloudcp

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

const (
	stuckCheckInterval  = 5 * time.Minute
	provisioningTimeout = 15 * time.Minute
)

// StuckProvisioningCleanup transitions tenants stuck in provisioning state
// for longer than provisioningTimeout to failed state.
type StuckProvisioningCleanup struct {
	registry *registry.TenantRegistry
}

// NewStuckProvisioningCleanup creates a cleanup job.
func NewStuckProvisioningCleanup(reg *registry.TenantRegistry) *StuckProvisioningCleanup {
	return &StuckProvisioningCleanup{registry: reg}
}

// Run starts the cleanup loop. It blocks until ctx is cancelled.
func (s *StuckProvisioningCleanup) Run(ctx context.Context) {
	log.Info().Msg("Stuck provisioning cleanup started")

	ticker := time.NewTicker(stuckCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Stuck provisioning cleanup stopped")
			return
		case <-ticker.C:
			s.cleanup(ctx)
		}
	}
}

func (s *StuckProvisioningCleanup) cleanup(ctx context.Context) {
	tenants, err := s.registry.ListByState(registry.TenantStateProvisioning)
	if err != nil {
		log.Error().Err(err).Msg("Stuck provisioning cleanup: failed to list provisioning tenants")
		return
	}

	cutoff := time.Now().UTC().Add(-provisioningTimeout)

	for _, tenant := range tenants {
		if ctx.Err() != nil {
			return
		}
		if tenant == nil {
			continue
		}

		if tenant.CreatedAt.After(cutoff) {
			continue // Still within provisioning window
		}

		log.Warn().
			Str("tenant_id", tenant.ID).
			Str("account_id", tenant.AccountID).
			Dur("stuck_duration", time.Since(tenant.CreatedAt)).
			Msg("Tenant stuck in provisioning state, transitioning to failed")

		tenant.State = registry.TenantStateFailed
		if err := s.registry.Update(tenant); err != nil {
			log.Error().Err(err).Str("tenant_id", tenant.ID).Msg("Stuck provisioning cleanup: failed to update tenant")
		}
	}
}
