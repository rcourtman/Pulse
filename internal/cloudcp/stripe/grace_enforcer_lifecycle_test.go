package stripe

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestGraceEnforcerRevokesBillingCapabilitiesAfterExpiry(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	provisioner := NewProvisioner(reg, tenantsDir, nil, nil, "https://cloud.example.com", nil, "", true)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatalf("GenerateAccountID: %v", err)
	}
	if err := reg.CreateAccount(&registry.Account{
		ID:          accountID,
		Kind:        registry.AccountKindIndividual,
		DisplayName: "Acme",
	}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	customerID := "cus_grace_123"
	subscriptionID := "sub_grace_123"
	tenantID := "t-grace1234"
	tenant := &registry.Tenant{
		ID:                   tenantID,
		AccountID:            accountID,
		Email:                "owner@example.com",
		State:                registry.TenantStateActive,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: subscriptionID,
		PlanVersion:          "cloud_v1",
	}
	if err := reg.Create(tenant); err != nil {
		t.Fatalf("Create tenant: %v", err)
	}

	if err := reg.CreateStripeAccount(&registry.StripeAccount{
		AccountID:            accountID,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: subscriptionID,
		PlanVersion:          "cloud_v1",
		SubscriptionState:    "past_due",
		UpdatedAt:            time.Now().UTC().Add(-15 * 24 * time.Hour).Unix(),
	}); err != nil {
		t.Fatalf("CreateStripeAccount: %v", err)
	}

	tenantDataDir := filepath.Join(tenantsDir, tenantID)
	if err := os.MkdirAll(tenantDataDir, 0o755); err != nil {
		t.Fatalf("ensure tenant data dir: %v", err)
	}
	if err := provisioner.writeBillingState(tenantDataDir, &pkglicensing.BillingState{
		Capabilities:         pkglicensing.DeriveCapabilitiesFromTier(pkglicensing.TierCloud, nil),
		Limits:               map[string]int64{},
		MetersEnabled:        []string{},
		PlanVersion:          "cloud_v1",
		SubscriptionState:    pkglicensing.SubStateGrace,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: subscriptionID,
	}); err != nil {
		t.Fatalf("writeBillingState: %v", err)
	}

	enforcer := NewGraceEnforcer(reg, provisioner)
	enforcer.enforce(context.Background())

	updatedTenant, err := reg.Get(tenantID)
	if err != nil {
		t.Fatalf("Get tenant: %v", err)
	}
	if updatedTenant == nil {
		t.Fatal("expected tenant to exist")
	}
	if updatedTenant.State != registry.TenantStateCanceled {
		t.Fatalf("tenant state = %q, want %q", updatedTenant.State, registry.TenantStateCanceled)
	}

	billingStore := config.NewFileBillingStore(tenantDataDir)
	updatedState, err := billingStore.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if updatedState == nil {
		t.Fatal("expected billing state to exist")
	}
	if updatedState.SubscriptionState != pkglicensing.SubStateCanceled {
		t.Fatalf("subscription state = %q, want %q", updatedState.SubscriptionState, pkglicensing.SubStateCanceled)
	}
	if len(updatedState.Capabilities) != 0 {
		t.Fatalf("capabilities = %v, want empty", updatedState.Capabilities)
	}

	sa, err := reg.GetStripeAccountByCustomerID(customerID)
	if err != nil {
		t.Fatalf("GetStripeAccountByCustomerID: %v", err)
	}
	if sa == nil {
		t.Fatal("expected stripe account mapping to exist")
	}
	if sa.SubscriptionState != "canceled" {
		t.Fatalf("stripe subscription state = %q, want %q", sa.SubscriptionState, "canceled")
	}
}
