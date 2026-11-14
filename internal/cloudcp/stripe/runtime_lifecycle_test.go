package stripe

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/lifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func TestMSPBillingLifecycleCannotChangeAuthorityDuringRollout(t *testing.T) {
	for _, action := range []string{"update", "cancel"} {
		t.Run(action, func(t *testing.T) {
			reg := newStripeTestRegistry(t)
			dir := t.TempDir()
			p := newTestProvisioner(t, reg, dir, nil, true)
			if err := reg.CreateAccount(&registry.Account{ID: "a-runtime", Kind: registry.AccountKindMSP}); err != nil {
				t.Fatal(err)
			}
			if err := reg.CreateStripeAccount(&registry.StripeAccount{AccountID: "a-runtime", StripeCustomerID: "cus_runtime", StripeSubscriptionID: "sub_original", PlanVersion: "msp_starter", SubscriptionState: "active"}); err != nil {
				t.Fatal(err)
			}
			var tenants []*registry.Tenant
			var billing [][]byte
			for _, name := range []string{"One", "Two"} {
				tenant, err := p.ProvisionWorkspace(context.Background(), "a-runtime", name)
				if err != nil {
					t.Fatal(err)
				}
				tenant.ContainerID = "runtime-" + name
				if err := reg.Update(tenant); err != nil {
					t.Fatal(err)
				}
				tenants = append(tenants, tenant)
				data, err := os.ReadFile(filepath.Join(dir, tenant.ID, "billing.json"))
				if err != nil {
					t.Fatal(err)
				}
				billing = append(billing, data)
			}
			_, lease, err := reg.AcquireRuntimeLifecycle(context.Background(), tenants[1].ID)
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			sub := Subscription{ID: "sub_changed", Customer: "cus_runtime", Status: "active", Metadata: map[string]string{"plan_version": "msp_max"}}
			var changeErr error
			if action == "update" {
				changeErr = p.HandleMSPSubscriptionUpdated(context.Background(), sub)
			} else {
				changeErr = p.HandleMSPSubscriptionDeleted(context.Background(), sub)
			}
			if !errors.Is(changeErr, lifecycle.ErrBusy) {
				t.Fatalf("billing mutation did not stop at rollout: %v", changeErr)
			}
			account, err := reg.GetStripeAccount("a-runtime")
			if err != nil {
				t.Fatal(err)
			}
			if action == "update" && (account.SubscriptionState != "active" || account.StripeSubscriptionID != "sub_original" || account.PlanVersion != "msp_starter") {
				t.Fatalf("positive authority changed before tenant exclusion: %+v", account)
			}
			if action == "cancel" && (account.SubscriptionState != "canceled" || account.StripeSubscriptionID != "sub_changed") {
				t.Fatalf("recovery hold blocked cancellation: %+v", account)
			}
			for i, tenant := range tenants {
				current, err := reg.Get(tenant.ID)
				if err != nil {
					t.Fatal(err)
				}
				wantState := tenant.State
				if action == "cancel" {
					wantState = registry.TenantStateCanceled
				}
				if current.State != wantState || current.ContainerID != tenant.ContainerID || current.PlanVersion != tenant.PlanVersion {
					t.Fatalf("busy billing replaced runtime state: %+v", current)
				}
				data, err := os.ReadFile(filepath.Join(dir, tenant.ID, "billing.json"))
				if err != nil || (action == "update" || i == 1) && !bytes.Equal(data, billing[i]) {
					t.Fatalf("busy billing rewrote tenant data: %v", err)
				}
			}
			if action == "cancel" {
				rows, err := reg.ListHostedEntitlements()
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != 2 {
					t.Fatalf("fixture entitlement count %d", len(rows))
				}
				for _, row := range rows {
					if row.RevokedAt == nil {
						t.Fatal("held client prevented authority revocation")
					}
					if _, err := p.hostedEntitlements.RefreshEntitlement(row.RefreshToken, "fixture.invalid"); !errors.Is(err, entitlements.ErrHostedEntitlementInactive) {
						t.Fatalf("withdrawn authority could refresh: %v", err)
					}
				}
			}
			// Any earlier admitted tenant is released when a later one is busy.
			_, first, err := reg.AcquireRuntimeLifecycle(context.Background(), tenants[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			_ = first.Close()
		})
	}
}

func TestCloudBillingAndDeprovisionHonorUnfinishedRuntimeRecovery(t *testing.T) {
	reg := newStripeTestRegistry(t)
	dir := t.TempDir()
	p := newTestProvisioner(t, reg, dir, nil, true)
	if err := p.HandleCheckout(context.Background(), CheckoutSession{Customer: "cus_runtime_cloud", CustomerEmail: "fixture@example.invalid", Subscription: "sub_original", Metadata: map[string]string{"plan_version": "cloud_starter"}}); err != nil {
		t.Fatal(err)
	}
	tenant, err := reg.GetByStripeCustomerID("cus_runtime_cloud")
	if err != nil || tenant == nil {
		t.Fatalf("fixture: %v", err)
	}
	tenant.ContainerID = "saved-runtime"
	if err := reg.Update(tenant); err != nil {
		t.Fatal(err)
	}
	billingPath := filepath.Join(dir, tenant.ID, "billing.json")
	original, err := os.ReadFile(billingPath)
	if err != nil {
		t.Fatal(err)
	}
	_, lease, err := reg.AcquireRuntimeLifecycle(context.Background(), tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.MarkRecovery(lifecycle.Recovery{TenantID: tenant.ID, Phase: "restore-incomplete"}); err != nil {
		t.Fatal(err)
	}
	_ = lease.Close()
	sub := Subscription{Customer: "cus_runtime_cloud", Status: "canceled"}
	for name, action := range map[string]func() error{
		"update": func() error { return p.HandleSubscriptionUpdated(context.Background(), sub) },
		"cancel": func() error { return p.HandleSubscriptionDeleted(context.Background(), sub) },
		"remove": func() error { return p.DeprovisionWorkspaceContainer(context.Background(), tenant) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := action(); !errors.Is(err, lifecycle.ErrRecoveryRequired) {
				t.Fatalf("unfinished recovery did not fence lifecycle write: %v", err)
			}
		})
	}
	current, err := reg.Get(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContainerID != "saved-runtime" || current.State != registry.TenantStateCanceled {
		t.Fatalf("saved runtime identity lost or withdrawal blocked during unfinished recovery: %+v", current)
	}
	got, err := os.ReadFile(billingPath)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("saved billing changed during unfinished recovery: %v", err)
	}
}

func TestProvisioningRollbackRetainsDataWhenRuntimeRemovalFails(t *testing.T) {
	reg := newStripeTestRegistry(t)
	dir := t.TempDir()
	p := newTestProvisioner(t, reg, dir, newFailingDockerManager(t), false)
	tenant := &registry.Tenant{ID: "t-runtime-cleanup", State: registry.TenantStateProvisioning, ContainerID: "unknown-runtime"}
	if err := reg.Create(tenant); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(dir, tenant.ID)
	if err := os.Mkdir(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "state.txt"), []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	p.rollbackProvisioning(provisioningCleanupState{tenantID: tenant.ID, tenantDataDir: dataDir, containerID: tenant.ContainerID, tenantCreated: true})
	if current, err := reg.Get(tenant.ID); err != nil || current == nil {
		t.Fatalf("unconfirmed removal erased registry: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dataDir, "state.txt")); err != nil || string(data) != "preserve" {
		t.Fatalf("unconfirmed removal erased data: %q %v", data, err)
	}
}
