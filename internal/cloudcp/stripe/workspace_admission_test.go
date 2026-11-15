package stripe

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/lifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func seedWorkspaceBilling(t *testing.T, reg *registry.TenantRegistry, accountID string) {
	t.Helper()
	if err := reg.CreateStripeAccount(&registry.StripeAccount{AccountID: accountID, StripeCustomerID: "cus_" + accountID, StripeSubscriptionID: "sub_" + accountID, SubscriptionState: "active", PlanVersion: "msp_growth"}); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceProvisioningRefusesBeforeWritingOwnerOrFiles(t *testing.T) {
	for _, state := range []string{"missing", "suspended", "expired", "canceled", "empty-plan"} {
		t.Run(state, func(t *testing.T) {
			reg := newStripeTestRegistry(t)
			dir := t.TempDir()
			if err := reg.CreateAccount(&registry.Account{ID: "a-admission", Kind: registry.AccountKindMSP}); err != nil {
				t.Fatal(err)
			}
			if state != "missing" {
				seedWorkspaceBilling(t, reg, "a-admission")
				sa, err := reg.GetStripeAccount("a-admission")
				if err != nil {
					t.Fatal(err)
				}
				if state == "empty-plan" {
					sa.PlanVersion = ""
				} else {
					sa.SubscriptionState = state
				}
				if err := reg.UpdateStripeAccount(sa); err != nil {
					t.Fatal(err)
				}
			}
			p := newTestProvisioner(t, reg, dir, nil, true, WithDefaultMSPPlanVersion("msp_growth"))
			tenant, err := p.ProvisionWorkspaceForOwner(context.Background(), "a-admission", "New Client", "new-owner@example.invalid")
			var denied *registry.WorkspaceAdmissionError
			if tenant != nil || !errors.As(err, &denied) {
				t.Fatalf("want policy refusal, got %+v/%v", tenant, err)
			}
			users, err := reg.GetUserByEmail("new-owner@example.invalid")
			if err != nil || users != nil {
				t.Fatalf("refusal created identity: %+v/%v", users, err)
			}
			tenants, err := reg.ListByAccountID("a-admission")
			if err != nil || len(tenants) != 0 {
				t.Fatalf("refusal created tenant: %+v/%v", tenants, err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("refusal created files: %v/%v", entries, err)
			}
		})
	}
}

func TestWorkspaceProvisioningFinalInsertRechecksAfterStorageAdmission(t *testing.T) {
	reg := newStripeTestRegistry(t)
	dir := t.TempDir()
	if err := reg.CreateAccount(&registry.Account{ID: "a-admission", Kind: registry.AccountKindMSP}); err != nil {
		t.Fatal(err)
	}
	seedWorkspaceBilling(t, reg, "a-admission")
	p := newTestProvisioner(t, reg, dir, nil, true, WithAdmissionCheck(func(context.Context) error {
		sa, err := reg.GetStripeAccount("a-admission")
		if err != nil {
			return err
		}
		sa.SubscriptionState = "suspended"
		return reg.UpdateStripeAccount(sa)
	}))
	_, err := p.ProvisionWorkspaceForOwner(context.Background(), "a-admission", "New Client", "new-owner@example.invalid")
	var denied *registry.WorkspaceAdmissionError
	if !errors.As(err, &denied) {
		t.Fatalf("stale authority survived storage precheck: %v", err)
	}
	user, err := reg.GetUserByEmail("new-owner@example.invalid")
	if err != nil || user != nil {
		t.Fatalf("failed final admission created owner: %+v/%v", user, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed final admission created files: %v/%v", entries, err)
	}
}

func TestWorkspaceProvisioningCannotFinishOverWithdrawnAuthority(t *testing.T) {
	for _, status := range []string{"paused", "incomplete", "incomplete_expired", "canceled", "future-status"} {
		t.Run(status, func(t *testing.T) {
			reg := newStripeTestRegistry(t)
			if err := reg.CreateAccount(&registry.Account{ID: "a-admission", Kind: registry.AccountKindMSP}); err != nil {
				t.Fatal(err)
			}
			seedWorkspaceBilling(t, reg, "a-admission")
			p := newTestProvisioner(t, reg, t.TempDir(), nil, true)
			tenant := &registry.Tenant{ID: "t-provisioning", AccountID: "a-admission", State: registry.TenantStateProvisioning, PlanVersion: "msp_growth"}
			if err := reg.CreateWithWorkspaceAdmission(tenant, registry.WorkspaceLimitPolicy{}); err != nil {
				t.Fatal(err)
			}
			_, lease, err := reg.AcquireRuntimeLifecycle(context.Background(), tenant.ID)
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			err = p.HandleMSPSubscriptionUpdated(context.Background(), Subscription{ID: "sub_changed", Customer: "cus_a-admission", Status: status})
			if !errors.Is(err, lifecycle.ErrBusy) {
				t.Fatalf("withdrawal did not retain runtime-busy result: %v", err)
			}
			if err := p.completeWorkspaceProvisioning(context.Background(), tenant, registry.WorkspaceLimitPolicy{}); err == nil {
				t.Fatal("stale provisioning completion reactivated withdrawal")
			}
			current, err := reg.Get(tenant.ID)
			if err != nil || current.State == registry.TenantStateActive {
				t.Fatalf("withdrawal replaced: %+v/%v", current, err)
			}
			if _, err := reg.ResolveWorkspaceAdmission(context.Background(), "a-admission", registry.WorkspaceLimitPolicy{}); err == nil {
				t.Fatal("normalized withdrawn billing admitted another workspace")
			}
		})
	}
}

func TestWorkspaceProvisioningRollsBackWhenAuthorityChangesBeforeLeaseWrite(t *testing.T) {
	reg := newStripeTestRegistry(t)
	dir := t.TempDir()
	if err := reg.CreateAccount(&registry.Account{ID: "a-admission", Kind: registry.AccountKindMSP}); err != nil {
		t.Fatal(err)
	}
	seedWorkspaceBilling(t, reg, "a-admission")
	p := newTestProvisioner(t, reg, dir, nil, true)
	changed := false
	p.chownFile = func(path string, _, _ int) error {
		if !changed && strings.HasSuffix(path, filepath.Join("secrets", "handoff.key")) {
			changed = true
			err := p.HandleMSPSubscriptionUpdated(context.Background(), Subscription{ID: "sub_changed", Customer: "cus_a-admission", Status: "paused"})
			if !errors.Is(err, lifecycle.ErrBusy) {
				t.Fatalf("withdrawal result: %v", err)
			}
		}
		return nil
	}
	if _, err := p.ProvisionWorkspace(context.Background(), "a-admission", "Client"); err == nil {
		t.Fatal("withdrawn provisioning succeeded")
	}
	if !changed {
		t.Fatal("fixture did not reach file initialization")
	}
	tenants, err := reg.ListByAccountID("a-admission")
	if err != nil || len(tenants) != 0 {
		t.Fatalf("partial tenant retained: %+v/%v", tenants, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial files retained: %v/%v", entries, err)
	}
	rows, err := reg.ListHostedEntitlements()
	if err != nil || len(rows) != 0 {
		t.Fatalf("withdrawn provisioning issued lease identity: %+v/%v", rows, err)
	}
}

func TestWorkspaceProvisioningKeepsUsableLeaseAndRefreshForAdmittedStates(t *testing.T) {
	for _, tc := range []struct {
		name, state string
		trialEnd    bool
		want        pkglicensing.SubscriptionState
	}{
		{"active", "active", false, pkglicensing.SubStateActive},
		{"checkout-no-trial-end", "trial", false, pkglicensing.SubStateActive},
		{"bounded-trial", "trial", true, pkglicensing.SubStateTrial},
		{"payment-grace", "past_due", false, pkglicensing.SubStateGrace},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := newStripeTestRegistry(t)
			if err := reg.CreateAccount(&registry.Account{ID: "a-admission", Kind: registry.AccountKindMSP}); err != nil {
				t.Fatal(err)
			}
			seedWorkspaceBilling(t, reg, "a-admission")
			sa, err := reg.GetStripeAccount("a-admission")
			if err != nil {
				t.Fatal(err)
			}
			sa.SubscriptionState = tc.state
			deadline := time.Now().Add(12 * time.Hour).Unix()
			if tc.trialEnd {
				sa.TrialEndsAt = &deadline
			}
			if err := reg.UpdateStripeAccount(sa); err != nil {
				t.Fatal(err)
			}
			p := newTestProvisioner(t, reg, t.TempDir(), nil, true)
			tenant, err := p.ProvisionWorkspace(context.Background(), "a-admission", "Client")
			if err != nil {
				t.Fatal(err)
			}
			raw := loadRawBillingState(t, p.tenantDataDir(tenant.ID))
			if raw.StripeCustomerID != "" || raw.StripeSubscriptionID != "" {
				t.Fatal("initial workspace file copied the consolidated account's billing identity")
			}
			key, err := base64.StdEncoding.DecodeString(os.Getenv("PULSE_TRIAL_ACTIVATION_PUBLIC_KEY"))
			if err != nil {
				t.Fatal(err)
			}
			host := tenant.ID + ".cloud.example.com"
			for _, refresh := range []bool{false, true} {
				token := raw.EntitlementJWT
				if refresh {
					renewed, err := p.hostedEntitlements.RefreshEntitlement(raw.EntitlementRefreshToken, host)
					if err != nil {
						t.Fatal(err)
					}
					token = renewed.EntitlementJWT
				}
				claims, err := pkglicensing.VerifyEntitlementLeaseToken(token, ed25519.PublicKey(key), host, time.Now())
				wantState := pkglicensing.SubStateActive // Preserve the existing initial writer; refresh resolves billing state.
				if refresh {
					wantState = tc.want
				}
				if err != nil || claims.SubscriptionState != wantState {
					t.Fatalf("unusable admitted lease (refresh=%t): %+v/%v", refresh, claims, err)
				}
				if tc.trialEnd && refresh && (claims.TrialEndsAt == nil || *claims.TrialEndsAt != deadline) {
					t.Fatalf("trial deadline lost (refresh=%t): %+v", refresh, claims)
				}
			}
		})
	}
}
