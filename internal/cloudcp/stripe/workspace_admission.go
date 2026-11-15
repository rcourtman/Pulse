package stripe

import (
	"context"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

// withWorkspaceAuthority orders the short lease-file/activation decisions with
// billing webhooks, never the container startup or its health wait. Acquisition
// is nonblocking, just like the existing webhook authority lease.
func (p *Provisioner) withWorkspaceAuthority(ctx context.Context, tenant *registry.Tenant, policy registry.WorkspaceLimitPolicy, action func(*registry.WorkspaceAdmission) error) error {
	admission, err := p.registry.ResolveWorkspaceAdmission(ctx, tenant.AccountID, policy)
	if err != nil {
		return err
	}
	if admission.StripeCustomerID != "" {
		_, authority, err := p.registry.AcquireBillingAuthority(ctx, admission.StripeCustomerID)
		if err != nil {
			return err
		}
		defer authority.Close()
	}
	currentAdmission, err := p.registry.ResolveWorkspaceAdmission(ctx, tenant.AccountID, policy)
	if err != nil {
		return err
	}
	if currentAdmission.StripeCustomerID != admission.StripeCustomerID || currentAdmission.PlanVersion != tenant.PlanVersion {
		return &registry.WorkspaceAdmissionError{Reason: "billing_changed", Message: "subscription changed while adding the workspace; try again"}
	}
	current, err := p.registry.Get(tenant.ID)
	if err != nil {
		return err
	}
	if current == nil || current.AccountID != tenant.AccountID || current.State != registry.TenantStateProvisioning {
		return fmt.Errorf("workspace state changed during provisioning")
	}
	return action(currentAdmission)
}

func (p *Provisioner) completeWorkspaceProvisioning(ctx context.Context, tenant *registry.Tenant, policy registry.WorkspaceLimitPolicy) error {
	return p.withWorkspaceAuthority(ctx, tenant, policy, func(_ *registry.WorkspaceAdmission) error {
		// A runtime result may update its own fields, not overwrite withdrawal
		// or billing authority using the pre-start tenant snapshot.
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
