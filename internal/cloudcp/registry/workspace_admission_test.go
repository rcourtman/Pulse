package registry

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func seedWorkspaceAdmission(t *testing.T, reg *TenantRegistry, state, plan string) *StripeAccount {
	t.Helper()
	if err := reg.CreateAccount(&Account{ID: "a-admission", Kind: AccountKindMSP}); err != nil {
		t.Fatal(err)
	}
	if state == "missing" {
		return nil
	}
	sa := &StripeAccount{AccountID: "a-admission", StripeCustomerID: "cus_admission", PlanVersion: plan, SubscriptionState: state}
	if err := reg.CreateStripeAccount(sa); err != nil {
		t.Fatal(err)
	}
	return sa
}

func TestWorkspaceAdmissionUsesCurrentBillingBeforeLocalPolicy(t *testing.T) {
	for _, tc := range []struct {
		state, plan string
		allowed     bool
	}{
		{"active", "msp_growth", true}, {"trial", "msp_growth", true}, {"past_due", "msp_growth", true}, {"grace", "msp_growth", true},
		{"ACTIVE", "msp-growth", true}, {"suspended", "msp_growth", false}, {"expired", "msp_growth", false}, {"canceled", "msp_growth", false},
		{"paused", "msp_growth", false}, {"unexpected", "msp_growth", false}, {"active", "", false}, {"active", "unknown-plan", true},
	} {
		t.Run(tc.state+"/"+tc.plan, func(t *testing.T) {
			reg := newTestRegistry(t)
			seedWorkspaceAdmission(t, reg, tc.state, tc.plan)
			policy := WorkspaceLimitPolicy{ProviderHostedMSP: true, ProviderMSPPlanVersion: "msp_max"}
			a, err := reg.ResolveWorkspaceAdmission(context.Background(), "a-admission", policy)
			if !tc.allowed {
				var denied *WorkspaceAdmissionError
				if !errors.As(err, &denied) {
					t.Fatalf("want definite refusal, got %+v / %v", a, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if a.PlanVersion == "msp_max" {
				t.Fatal("local fallback replaced billing authority")
			}
			if tc.plan == "unknown-plan" && a.Limit != 1 {
				t.Fatalf("unknown paid plan enlarged the safe ceiling: %+v", a)
			}
			tenant := &Tenant{ID: "t-new", AccountID: "a-admission", PlanVersion: "msp_max", State: TenantStateProvisioning}
			if err := reg.CreateWithWorkspaceAdmission(tenant, policy); err != nil {
				t.Fatal(err)
			}
			if tenant.PlanVersion != a.PlanVersion {
				t.Fatalf("caller plan survived authoritative insertion: %+v", tenant)
			}
		})
	}
}

func TestWorkspaceAdmissionRequiresExplicitLocalMSPAuthority(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   AccountKind
		policy WorkspaceLimitPolicy
		want   string
	}{
		{"missing", AccountKindMSP, WorkspaceLimitPolicy{}, "no_billing_record"},
		{"plan-is-not-mode", AccountKindMSP, WorkspaceLimitPolicy{ProviderMSPPlanVersion: "msp_growth"}, "no_billing_record"},
		{"individual", AccountKindIndividual, WorkspaceLimitPolicy{ProviderHostedMSP: true, ProviderMSPPlanVersion: "msp_growth"}, "no_billing_record"},
		{"unconfigured", AccountKindMSP, WorkspaceLimitPolicy{ProviderHostedMSP: true}, "provider_msp_plan_missing"},
		{"cloud-not-msp", AccountKindMSP, WorkspaceLimitPolicy{ProviderHostedMSP: true, ProviderMSPPlanVersion: "cloud_starter"}, "provider_msp_plan_unknown"},
		{"unknown", AccountKindMSP, WorkspaceLimitPolicy{ProviderHostedMSP: true, ProviderMSPPlanVersion: "msp_unknown"}, "provider_msp_plan_unknown"},
		{"lapsed", AccountKindMSP, WorkspaceLimitPolicy{ProviderHostedMSP: true, ProviderMSPPlanVersion: "msp_eval", ProviderMSPLicenseLapsed: func() bool { return true }}, "provider_msp_license_lapsed"},
		{"evaluation", AccountKindMSP, WorkspaceLimitPolicy{ProviderHostedMSP: true, ProviderMSPPlanVersion: "msp_eval"}, ""},
		{"paid-provider", AccountKindMSP, WorkspaceLimitPolicy{ProviderHostedMSP: true, ProviderMSPPlanVersion: "msp_growth"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := newTestRegistry(t)
			if err := reg.CreateAccount(&Account{ID: "a-admission", Kind: tc.kind}); err != nil {
				t.Fatal(err)
			}
			a, err := reg.ResolveWorkspaceAdmission(context.Background(), "a-admission", tc.policy)
			if tc.want != "" {
				var denied *WorkspaceAdmissionError
				if !errors.As(err, &denied) || denied.Reason != tc.want {
					t.Fatalf("got %+v/%v, want %s", a, err, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if a.PlanVersion != tc.policy.ProviderMSPPlanVersion {
				t.Fatalf("wrong local plan: %+v", a)
			}
			if err := reg.CreateWithWorkspaceAdmission(&Tenant{ID: "t-new", AccountID: "a-admission", State: TenantStateProvisioning}, tc.policy); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkspaceInsertRechecksAuthorityAfterPreflight(t *testing.T) {
	for _, change := range []string{"withdraw", "missing", "empty-plan", "downgrade", "local-lapse"} {
		t.Run(change, func(t *testing.T) {
			reg := newTestRegistry(t)
			sa := seedWorkspaceAdmission(t, reg, "active", "msp_growth")
			policy := WorkspaceLimitPolicy{}
			lapsed := false
			if change == "local-lapse" {
				if _, err := reg.db.Exec("DELETE FROM stripe_accounts WHERE account_id = ?", "a-admission"); err != nil {
					t.Fatal(err)
				}
				policy = WorkspaceLimitPolicy{ProviderHostedMSP: true, ProviderMSPPlanVersion: "msp_eval", ProviderMSPLicenseLapsed: func() bool { return lapsed }}
			}
			if _, err := reg.ResolveWorkspaceAdmission(context.Background(), "a-admission", policy); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "withdraw":
				sa.SubscriptionState = "suspended"
				if err := reg.UpdateStripeAccount(sa); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if _, err := reg.db.Exec("DELETE FROM stripe_accounts WHERE account_id = ?", "a-admission"); err != nil {
					t.Fatal(err)
				}
			case "empty-plan":
				sa.PlanVersion = ""
				if err := reg.UpdateStripeAccount(sa); err != nil {
					t.Fatal(err)
				}
			case "downgrade":
				if err := reg.Create(&Tenant{ID: "t-existing", AccountID: "a-admission", State: TenantStateActive}); err != nil {
					t.Fatal(err)
				}
				sa.PlanVersion = "cloud_starter"
				if err := reg.UpdateStripeAccount(sa); err != nil {
					t.Fatal(err)
				}
			case "local-lapse":
				lapsed = true
			}
			err := reg.CreateWithWorkspaceAdmission(&Tenant{ID: "t-new", AccountID: "a-admission", PlanVersion: "msp_max", State: TenantStateProvisioning}, policy)
			if err == nil {
				t.Fatal("stale preflight admitted workspace")
			}
			current, getErr := reg.Get("t-new")
			if getErr != nil || current != nil {
				t.Fatalf("refusal left tenant %+v/%v", current, getErr)
			}
		})
	}
}

func TestWorkspaceInsertCannotEnlargeCurrentLimitAcrossRegistries(t *testing.T) {
	dir := t.TempDir()
	first, err := NewTenantRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	seedWorkspaceAdmission(t, first, "active", "cloud_starter")
	second, err := NewTenantRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	const attempts = 12
	var wg sync.WaitGroup
	results := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			reg := []*TenantRegistry{first, second}[i%2]
			results <- reg.CreateWithAccountWorkspaceLimit(&Tenant{ID: fmt.Sprintf("t-admission-%d", i), AccountID: "a-admission", PlanVersion: "msp_max", State: TenantStateProvisioning}, 1000)
		}(i)
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if _, ok := WorkspaceLimitExceeded(err); !ok {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful creates=%d, want current-plan limit 1", successes)
	}
}
