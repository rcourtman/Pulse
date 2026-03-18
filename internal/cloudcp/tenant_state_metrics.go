package cloudcp

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

const tenantStateMetricsInterval = 30 * time.Second

func runTenantStateMetrics(ctx context.Context, reg *registry.TenantRegistry) {
	ticker := time.NewTicker(tenantStateMetricsInterval)
	defer ticker.Stop()

	// Prime once at startup so /metrics isn't empty for this gauge.
	updateTenantStateGauges(reg)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updateTenantStateGauges(reg)
		}
	}
}

func updateTenantStateGauges(reg *registry.TenantRegistry) {
	counts, err := reg.CountByState()
	if err != nil {
		log.Error().Err(err).Msg("Failed to update tenant state metrics")
		return
	}

	known := []registry.TenantState{
		registry.TenantStateProvisioning,
		registry.TenantStateActive,
		registry.TenantStateSuspended,
		registry.TenantStateCanceled,
		registry.TenantStateDeleting,
		registry.TenantStateDeleted,
		registry.TenantStateFailed,
	}

	seen := make(map[registry.TenantState]struct{}, len(counts))

	// Ensure stable label set for known states.
	for _, state := range known {
		seen[state] = struct{}{}
		cpmetrics.TenantsByState.WithLabelValues(string(state)).Set(float64(counts[state]))
	}

	// Record any unexpected states too (defensive; bounded by DB content).
	for state, c := range counts {
		if _, ok := seen[state]; ok {
			continue
		}
		cpmetrics.TenantsByState.WithLabelValues(string(state)).Set(float64(c))
	}
}
