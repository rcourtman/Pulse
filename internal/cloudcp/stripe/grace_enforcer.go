package stripe

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
)

const (
	graceCheckInterval = 1 * time.Hour
	maxGraceDays       = 14
)

// GraceEnforcer periodically transitions tenants stuck in SubStateGrace
// for longer than maxGraceDays to canceled.
type GraceEnforcer struct {
	registry    *registry.TenantRegistry
	provisioner *Provisioner
}

// NewGraceEnforcer creates a GraceEnforcer.
func NewGraceEnforcer(reg *registry.TenantRegistry, provisioners ...*Provisioner) *GraceEnforcer {
	var provisioner *Provisioner
	if len(provisioners) > 0 {
		provisioner = provisioners[0]
	}
	return &GraceEnforcer{
		registry:    reg,
		provisioner: provisioner,
	}
}

// Run starts the enforcement loop. It blocks until ctx is cancelled.
func (g *GraceEnforcer) Run(ctx context.Context) {
	log.Info().Msg("Grace period enforcer started")

	ticker := time.NewTicker(graceCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Grace period enforcer stopped")
			return
		case <-ticker.C:
			g.enforce(ctx)
		}
	}
}

func (g *GraceEnforcer) enforce(ctx context.Context) {
	tenants, err := g.registry.ListByState(registry.TenantStateSuspended)
	if err != nil {
		log.Error().Err(err).Msg("Grace enforcer: failed to list suspended tenants")
		return
	}

	// Also check active tenants that might have grace subscription state.
	activeTenants, err := g.registry.ListByState(registry.TenantStateActive)
	if err != nil {
		log.Error().Err(err).Msg("Grace enforcer: failed to list active tenants")
		return
	}
	tenants = append(tenants, activeTenants...)

	cutoff := time.Now().UTC().Add(-time.Duration(maxGraceDays) * 24 * time.Hour)

	for _, tenant := range tenants {
		if ctx.Err() != nil {
			return
		}
		if tenant == nil {
			continue
		}

		// Check if the tenant's billing state indicates grace period.
		// We use the StripeAccount's subscription_state field to determine this.
		if tenant.StripeCustomerID == "" {
			continue
		}

		sa, err := g.registry.GetStripeAccountByCustomerID(tenant.StripeCustomerID)
		if err != nil || sa == nil {
			continue
		}

		// Only enforce on tenants in past_due/grace state.
		if sa.SubscriptionState != "past_due" && sa.SubscriptionState != string(pkglicensing.SubStateGrace) {
			continue
		}

		// Check if updated_at is older than the grace cutoff.
		if sa.UpdatedAt == 0 || time.Unix(sa.UpdatedAt, 0).UTC().After(cutoff) {
			continue
		}

		log.Warn().
			Str("tenant_id", tenant.ID).
			Str("account_id", tenant.AccountID).
			Str("stripe_customer_id", tenant.StripeCustomerID).
			Str("subscription_state", string(pkglicensing.SubStateGrace)).
			Int("grace_days_exceeded", maxGraceDays).
			Msg("Grace period expired, transitioning tenant to canceled")

		// Primary path: reuse the subscription-delete lifecycle so billing state
		// and capabilities are revoked alongside tenant state.
		if g.provisioner != nil {
			subID := sa.StripeSubscriptionID
			if subID == "" {
				subID = tenant.StripeSubscriptionID
			}
			if err := g.provisioner.HandleSubscriptionDeleted(ctx, Subscription{
				ID:       subID,
				Customer: tenant.StripeCustomerID,
			}); err != nil {
				log.Error().
					Err(err).
					Str("tenant_id", tenant.ID).
					Str("stripe_customer_id", tenant.StripeCustomerID).
					Msg("Grace enforcer: failed to revoke subscription entitlements")
			}
			continue
		}

		// Fallback for legacy callers without a provisioner dependency.
		tenant.State = registry.TenantStateCanceled
		if err := g.registry.Update(tenant); err != nil {
			log.Error().Err(err).Str("tenant_id", tenant.ID).Msg("Grace enforcer: failed to cancel tenant")
		}
	}
}
