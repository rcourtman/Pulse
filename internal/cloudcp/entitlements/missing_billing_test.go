package entitlements

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestRefreshMissingBillingDoesNotRecreateAuthority(t *testing.T) {
	for _, hasAccount := range []bool{false, true} {
		name := "legacy_accountless"
		if hasAccount {
			name = "current_account"
		}
		t.Run(name, func(t *testing.T) {
			svc, _, reg := newTestService(t)
			accountID := ""
			if hasAccount {
				accountID = "a_MISSINGBILLING"
				if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindIndividual, DisplayName: "Fixture"}); err != nil {
					t.Fatal(err)
				}
			}
			tenant := &registry.Tenant{ID: "t-MISSINGBILLING", AccountID: accountID, State: registry.TenantStateActive, PlanVersion: "cloud_starter", Email: "owner@example.com"}
			if err := reg.Create(tenant); err != nil {
				t.Fatal(err)
			}
			now := time.Unix(1710000000, 0).UTC()
			svc.SetNow(func() time.Time { return now })
			if _, _, err := reg.StoreOrIssueHostedEntitlement(tenant.ID, "etr_missing_billing_fixture", now.Add(-time.Hour)); err != nil {
				t.Fatal(err)
			}
			result, err := svc.RefreshEntitlement("etr_missing_billing_fixture", "t-missingbilling.cloud.example.com")
			if err == nil || result != nil {
				t.Fatal("missing billing authority minted a new paid lease")
			}
			stored, err := reg.GetHostedEntitlementByRefreshToken("etr_missing_billing_fixture")
			if err != nil {
				t.Fatal(err)
			}
			if stored == nil || stored.RevokedAt != nil || stored.LastRefreshedAt != nil {
				t.Fatal("missing billing changed saved refresh identity")
			}
		})
	}
}

func TestRefreshMissingBillingRecoversWithTheSameIdentity(t *testing.T) {
	svc, pub, reg := newTestService(t)
	const accountID = "a_RECOVERBILLING"
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindIndividual}); err != nil {
		t.Fatal(err)
	}
	tenant := &registry.Tenant{ID: "t-RECOVERBILLING", AccountID: accountID, State: registry.TenantStateActive, PlanVersion: "cloud_starter"}
	if err := reg.Create(tenant); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1710000000, 0).UTC()
	svc.SetNow(func() time.Time { return now })
	if _, _, err := reg.StoreOrIssueHostedEntitlement(tenant.ID, "etr_recover_billing_fixture", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RefreshEntitlement("etr_recover_billing_fixture", "t-recoverbilling.cloud.example.com"); !errors.Is(err, ErrHostedEntitlementBillingUnavailable) {
		t.Fatalf("missing authority error = %v", err)
	}
	if err := reg.CreateStripeAccount(&registry.StripeAccount{AccountID: accountID, StripeCustomerID: "cus_recover_billing_fixture", SubscriptionState: "active", PlanVersion: "cloud_starter"}); err != nil {
		t.Fatal(err)
	}
	result, err := svc.RefreshEntitlement("etr_recover_billing_fixture", "t-recoverbilling.cloud.example.com")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := pkglicensing.VerifyEntitlementLeaseToken(result.EntitlementJWT, pub, "t-recoverbilling.cloud.example.com", now)
	if err != nil {
		t.Fatal(err)
	}
	if claims.SubscriptionState != pkglicensing.SubStateActive {
		t.Fatalf("recovered state = %s", claims.SubscriptionState)
	}
	stored, err := reg.GetHostedEntitlementByRefreshToken("etr_recover_billing_fixture")
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.RevokedAt != nil || stored.LastRefreshedAt == nil || !stored.LastRefreshedAt.Equal(now) {
		t.Fatal("successful refresh did not retain and update the original identity")
	}
}

func TestRefreshNoBillingRequiresConfiguredMSPAccountAndPlan(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		localMSP, providerHosted bool
		kind                     registry.AccountKind
		plan                     string
		allow                    bool
	}{
		{name: "cloud individual", kind: registry.AccountKindIndividual, plan: "cloud_starter"},
		{name: "cloud MSP-looking plan", kind: registry.AccountKindMSP, plan: "msp_starter"},
		{name: "provider ceiling alone is not billing policy", providerHosted: true, kind: registry.AccountKindMSP, plan: "msp_starter"},
		{name: "configured MSP individual", localMSP: true, kind: registry.AccountKindIndividual, plan: "msp_starter"},
		{name: "configured MSP cloud plan", localMSP: true, kind: registry.AccountKindMSP, plan: "cloud_starter"},
		{name: "configured MSP eval", localMSP: true, providerHosted: true, kind: registry.AccountKindMSP, plan: pkglicensing.PlanVersionMSPEval, allow: true},
		{name: "configured MSP inferred account plan", localMSP: true, kind: registry.AccountKindMSP, allow: true},
		{name: "configured MSP stored plan", localMSP: true, kind: registry.AccountKindMSP, plan: "msp_starter", allow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, pub, reg := newTestService(t)
			svc.SetLocalMSPBilling(tc.localMSP)
			svc.SetProviderHosted(tc.providerHosted)
			const accountID = "a_NOBILLING"
			if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: tc.kind}); err != nil {
				t.Fatal(err)
			}
			tenant := &registry.Tenant{ID: "t-NOBILLING", AccountID: accountID, State: registry.TenantStateActive, PlanVersion: tc.plan}
			if err := reg.Create(tenant); err != nil {
				t.Fatal(err)
			}
			now := time.Unix(1710000000, 0).UTC()
			svc.SetNow(func() time.Time { return now })
			if _, _, err := reg.StoreOrIssueHostedEntitlement(tenant.ID, "etr_nobilling_fixture", now); err != nil {
				t.Fatal(err)
			}
			result, err := svc.RefreshEntitlement("etr_nobilling_fixture", "t-nobilling.cloud.example.com")
			if !tc.allow {
				if result != nil || !errors.Is(err, ErrHostedEntitlementBillingUnavailable) {
					t.Fatalf("wanted retryable absent authority; result present=%v err=%v", result != nil, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			claims, err := pkglicensing.VerifyEntitlementLeaseToken(result.EntitlementJWT, pub, "t-nobilling.cloud.example.com", now)
			if err != nil {
				t.Fatal(err)
			}
			if claims.SubscriptionState != pkglicensing.SubStateActive {
				t.Fatalf("state = %s", claims.SubscriptionState)
			}
			if tc.providerHosted {
				for _, c := range claims.Capabilities {
					if c == pkglicensing.FeatureRelay || c == pkglicensing.FeatureMobileApp || c == pkglicensing.FeaturePushNotifications {
						t.Fatalf("provider ceiling widened: %s", c)
					}
				}
			}
		})
	}
}

func TestLocalMSPBillingCannotOverrideStoredWithdrawal(t *testing.T) {
	for _, state := range []string{"canceled", "expired", "suspended", "unknown", ""} {
		t.Run("stored_"+state, func(t *testing.T) {
			svc, _, reg, dir := newTestServiceWithBaseURLAndDir(t, "https://cloud.example.com")
			svc.SetLocalMSPBilling(true)
			const accountID = "a_WITHDRAWN"
			if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP}); err != nil {
				t.Fatal(err)
			}
			if err := reg.CreateStripeAccount(&registry.StripeAccount{AccountID: accountID, StripeCustomerID: "cus_withdrawn_billing_fixture", SubscriptionState: state}); err != nil {
				t.Fatal(err)
			}
			if state == "" {
				// CreateStripeAccount intentionally defaults an omitted state to
				// trial. Seed the actual blank legacy row we mean to test rather
				// than mislabel that valid default as a stored denial.
				db, err := sql.Open("sqlite", filepath.Join(dir, "tenants.db"))
				if err != nil {
					t.Fatal(err)
				}
				_, err = db.Exec(`UPDATE stripe_accounts SET subscription_state = '' WHERE account_id = ?`, accountID)
				closeErr := db.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("seed blank subscription state: write=%v close=%v", err, closeErr)
				}
			}
			tenant := &registry.Tenant{ID: "t-WITHDRAWN", AccountID: accountID, State: registry.TenantStateActive, PlanVersion: "msp_starter"}
			if err := reg.Create(tenant); err != nil {
				t.Fatal(err)
			}
			if _, _, err := reg.StoreOrIssueHostedEntitlement(tenant.ID, "etr_withdrawn_fixture", time.Unix(1710000000, 0)); err != nil {
				t.Fatal(err)
			}
			result, err := svc.RefreshEntitlement("etr_withdrawn_fixture", "t-withdrawn.cloud.example.com")
			if result != nil || !errors.Is(err, ErrHostedEntitlementInactive) {
				t.Fatalf("stored denial bypassed; result present=%v err=%v", result != nil, err)
			}
		})
	}
}
