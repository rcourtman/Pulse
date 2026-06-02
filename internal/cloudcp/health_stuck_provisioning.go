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

// StuckProvisioningReport describes tenants that have exceeded the
// provider-control-plane provisioning grace period.
type StuckProvisioningReport struct {
	Timeout   time.Duration
	Count     int
	TenantIDs []string
}

// InspectStuckProvisioning returns provisioning tenants that are old enough to
// be treated as failed by the cleanup loop.
func InspectStuckProvisioning(reg *registry.TenantRegistry, now time.Time) (*StuckProvisioningReport, error) {
	if reg == nil {
		return nil, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tenants, err := reg.ListByState(registry.TenantStateProvisioning)
	if err != nil {
		return nil, err
	}
	cutoff := now.UTC().Add(-provisioningTimeout)
	report := &StuckProvisioningReport{Timeout: provisioningTimeout}
	for _, tenant := range tenants {
		if tenant == nil || tenant.CreatedAt.After(cutoff) {
			continue
		}
		report.Count++
		report.TenantIDs = append(report.TenantIDs, tenant.ID)
	}
	return report, nil
}

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
	report, err := InspectStuckProvisioning(s.registry, time.Now().UTC())
	if err != nil {
		log.Error().Err(err).Msg("Stuck provisioning cleanup: failed to list provisioning tenants")
		return
	}
	if report == nil || report.Count == 0 {
		return
	}
	for _, tenantID := range report.TenantIDs {
		if ctx.Err() != nil {
			return
		}
		tenant, err := s.registry.Get(tenantID)
		if err != nil {
			log.Error().Err(err).Str("tenant_id", tenantID).Msg("Stuck provisioning cleanup: failed to reload tenant")
			continue
		}
		if tenant == nil || tenant.State != registry.TenantStateProvisioning {
			continue
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
