package stripe

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	"github.com/rs/zerolog/log"
)

// Provisioner orchestrates tenant creation, billing state updates, and (later)
// container lifecycle in response to Stripe events.
type Provisioner struct {
	registry   *registry.TenantRegistry
	tenantsDir string
}

// NewProvisioner creates a Provisioner.
func NewProvisioner(reg *registry.TenantRegistry, tenantsDir string) *Provisioner {
	return &Provisioner{
		registry:   reg,
		tenantsDir: tenantsDir,
	}
}

// HandleCheckout provisions a new tenant from a checkout.session.completed event.
func (p *Provisioner) HandleCheckout(ctx context.Context, session CheckoutSession) error {
	customerID := strings.TrimSpace(session.Customer)
	if customerID == "" {
		return fmt.Errorf("checkout session missing customer")
	}
	if !IsSafeStripeID(customerID) {
		return fmt.Errorf("invalid stripe customer id: %s", customerID)
	}

	email := strings.ToLower(strings.TrimSpace(session.CustomerEmail))
	if email == "" {
		email = strings.ToLower(strings.TrimSpace(session.CustomerDetails.Email))
	}

	// Check if a tenant already exists for this Stripe customer
	existing, err := p.registry.GetByStripeCustomerID(customerID)
	if err != nil {
		return fmt.Errorf("lookup existing tenant: %w", err)
	}
	if existing != nil {
		log.Info().
			Str("tenant_id", existing.ID).
			Str("customer_id", customerID).
			Msg("Tenant already exists for Stripe customer, skipping provisioning")
		return nil
	}

	// Generate tenant ID
	tenantID, err := registry.GenerateTenantID()
	if err != nil {
		return fmt.Errorf("generate tenant id: %w", err)
	}

	planVersion := DerivePlanVersion(session.Metadata, "")

	// Write billing.json to tenant data dir.
	// FileBillingStore with baseDataDir=<tenantDir> + SaveBillingState("default", state)
	// writes billing.json at the root of the tenant data dir.
	tenantDataDir := p.tenantsDir + "/" + tenantID
	billingStore := config.NewFileBillingStore(tenantDataDir)
	state := &entitlements.BillingState{
		Capabilities:         license.DeriveCapabilitiesFromTier(license.TierCloud, nil),
		Limits:               map[string]int64{},
		MetersEnabled:        []string{},
		PlanVersion:          planVersion,
		SubscriptionState:    entitlements.SubStateActive,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: strings.TrimSpace(session.Subscription),
	}
	if err := billingStore.SaveBillingState("default", state); err != nil {
		return fmt.Errorf("write billing state: %w", err)
	}

	// Insert registry record
	tenant := &registry.Tenant{
		ID:                   tenantID,
		Email:                email,
		State:                registry.TenantStateProvisioning,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: strings.TrimSpace(session.Subscription),
		PlanVersion:          planVersion,
	}
	if err := p.registry.Create(tenant); err != nil {
		return fmt.Errorf("create tenant record: %w", err)
	}

	log.Info().
		Str("tenant_id", tenantID).
		Str("customer_id", customerID).
		Str("email", email).
		Str("plan_version", planVersion).
		Msg("Tenant provisioned from checkout")

	return nil
}

// HandleSubscriptionUpdated syncs billing state when a subscription changes.
func (p *Provisioner) HandleSubscriptionUpdated(ctx context.Context, sub Subscription) error {
	customerID := strings.TrimSpace(sub.Customer)
	if customerID == "" {
		return fmt.Errorf("subscription missing customer")
	}

	tenant, err := p.registry.GetByStripeCustomerID(customerID)
	if err != nil {
		return fmt.Errorf("lookup tenant by customer: %w", err)
	}
	if tenant == nil {
		log.Warn().Str("customer_id", customerID).Msg("subscription.updated: tenant not found")
		return nil
	}

	subState := MapSubscriptionStatus(sub.Status)
	priceID := sub.FirstPriceID()
	planVersion := DerivePlanVersion(sub.Metadata, priceID)

	// Update billing.json
	var caps []string
	if ShouldGrantCapabilities(subState) {
		caps = license.DeriveCapabilitiesFromTier(license.TierCloud, nil)
	}

	tenantDataDir := p.tenantsDir + "/" + tenant.ID
	billingStore := config.NewFileBillingStore(tenantDataDir)
	state := &entitlements.BillingState{
		Capabilities:         caps,
		Limits:               map[string]int64{},
		MetersEnabled:        []string{},
		PlanVersion:          planVersion,
		SubscriptionState:    subState,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: strings.TrimSpace(sub.ID),
		StripePriceID:        priceID,
	}
	if err := billingStore.SaveBillingState("default", state); err != nil {
		return fmt.Errorf("save billing state: %w", err)
	}

	// Update registry
	tenant.StripeSubscriptionID = strings.TrimSpace(sub.ID)
	tenant.StripePriceID = priceID
	tenant.PlanVersion = planVersion
	if subState == entitlements.SubStateSuspended {
		tenant.State = registry.TenantStateSuspended
	} else if subState == entitlements.SubStateActive || subState == entitlements.SubStateTrial || subState == entitlements.SubStateGrace {
		tenant.State = registry.TenantStateActive
	}
	if err := p.registry.Update(tenant); err != nil {
		return fmt.Errorf("update tenant record: %w", err)
	}

	log.Info().
		Str("tenant_id", tenant.ID).
		Str("customer_id", customerID).
		Str("subscription_state", string(subState)).
		Msg("Subscription updated")

	return nil
}

// HandleSubscriptionDeleted revokes capabilities on cancellation.
func (p *Provisioner) HandleSubscriptionDeleted(ctx context.Context, sub Subscription) error {
	customerID := strings.TrimSpace(sub.Customer)
	if customerID == "" {
		return fmt.Errorf("subscription missing customer")
	}

	tenant, err := p.registry.GetByStripeCustomerID(customerID)
	if err != nil {
		return fmt.Errorf("lookup tenant by customer: %w", err)
	}
	if tenant == nil {
		log.Warn().Str("customer_id", customerID).Msg("subscription.deleted: tenant not found")
		return nil
	}

	// Revoke capabilities immediately
	tenantDataDir := p.tenantsDir + "/" + tenant.ID
	billingStore := config.NewFileBillingStore(tenantDataDir)
	state := &entitlements.BillingState{
		Capabilities:         []string{},
		Limits:               map[string]int64{},
		MetersEnabled:        []string{},
		PlanVersion:          tenant.PlanVersion,
		SubscriptionState:    entitlements.SubStateCanceled,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: strings.TrimSpace(sub.ID),
	}
	if err := billingStore.SaveBillingState("default", state); err != nil {
		return fmt.Errorf("save billing state: %w", err)
	}

	// Update registry
	tenant.State = registry.TenantStateCanceled
	if err := p.registry.Update(tenant); err != nil {
		return fmt.Errorf("update tenant record: %w", err)
	}

	log.Info().
		Str("tenant_id", tenant.ID).
		Str("customer_id", customerID).
		Msg("Subscription deleted, capabilities revoked")

	return nil
}
