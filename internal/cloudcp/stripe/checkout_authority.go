package stripe

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// withCheckoutAuthority orders issuance/activation with subscription withdrawal,
// never container startup or health. A signed old checkout is not current billing.
func (p *Provisioner) withCheckoutAuthority(ctx context.Context, tenant *registry.Tenant, action func() error) error {
	_, authority, err := p.registry.AcquireBillingAuthority(ctx, tenant.StripeCustomerID)
	if err != nil {
		return err
	}
	defer authority.Close()
	current, err := p.registry.GetStripeAccountByCustomerID(tenant.StripeCustomerID)
	if err != nil {
		return err
	}
	if current == nil || current.AccountID != tenant.AccountID ||
		canonicalizeProvisionedPlanVersion(current.PlanVersion) != tenant.PlanVersion ||
		strings.TrimSpace(current.StripeSubscriptionID) != tenant.StripeSubscriptionID {
		return fmt.Errorf("checkout billing authority changed during provisioning")
	}
	if !ShouldGrantCapabilities(pkglicensing.ParseStoredSubscriptionState(current.SubscriptionState)) {
		return entitlements.ErrHostedEntitlementInactive
	}
	return action()
}

func (p *Provisioner) completeCheckoutProvisioning(ctx context.Context, tenant *registry.Tenant) error {
	return p.withCheckoutAuthority(ctx, tenant, func() error {
		current, err := p.registry.Get(tenant.ID)
		if err != nil {
			return err
		}
		if current == nil || current.AccountID != tenant.AccountID || current.State != registry.TenantStateProvisioning {
			return fmt.Errorf("checkout tenant state changed during provisioning")
		}
		// Only the runtime result belongs to this completion. A current account
		// label, manual hold, billing state or terminal deletion cannot be replaced.
		if err := p.registry.UpdateRuntimeState(tenant); err != nil {
			return err
		}
		if err := p.registry.TransitionTenantState(tenant.ID, registry.TenantStateProvisioning, registry.TenantStateActive); err != nil {
			return err
		}
		tenant.State = registry.TenantStateActive
		return nil
	})
}
