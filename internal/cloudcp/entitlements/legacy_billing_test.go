package entitlements

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// This is the representation left by the account_id migration: the tenant has
// no consolidated account, but retains the Stripe identity maintained by the
// individual subscription webhook. It is not an active row with no billing.
func legacyStripeTenant(t *testing.T, reg *registry.TenantRegistry) *registry.Tenant {
	t.Helper()
	tenant := &registry.Tenant{
		ID: "t-LEGACYPAID", Email: "fixture@example.invalid", State: registry.TenantStateActive,
		StripeCustomerID: "cus_legacy_paid", StripeSubscriptionID: "sub_legacy_paid",
		StripePriceID: "price_legacy_paid", PlanVersion: "cloud-v1", ContainerID: "saved-runtime",
	}
	if err := reg.Create(tenant); err != nil {
		t.Fatal(err)
	}
	return tenant
}

func TestLegacyStripeTenantRenewsContinuouslyAndRecoversSameToken(t *testing.T) {
	svc, pub, reg := newTestService(t)
	tenant := legacyStripeTenant(t, reg)
	now := time.Unix(1710000000, 0).UTC()
	svc.SetNow(func() time.Time { return now })
	initial, err := svc.IssueTenantBillingState(tenant, pkglicensing.SubStateActive, tenant.PlanVersion, tenant.StripeCustomerID, tenant.StripeSubscriptionID, tenant.StripePriceID)
	if err != nil {
		t.Fatal(err)
	}
	const host = "t-legacypaid.cloud.example.com"
	originalClaims, err := pkglicensing.VerifyEntitlementLeaseToken(initial.EntitlementJWT, pub, host, now)
	if err != nil {
		t.Fatal(err)
	}
	original, err := reg.GetHostedEntitlementByRefreshToken(initial.EntitlementRefreshToken)
	if err != nil || original == nil {
		t.Fatalf("original authority: %v", err)
	}
	started := now
	// No new webhook or explicit issuance accompanies these reads. Renewal
	// must continue across multiple original 24-hour lease boundaries.
	for _, hours := range []int{23, 47, 71, 95, 119} {
		now = started.Add(time.Duration(hours) * time.Hour)
		result, err := svc.RefreshEntitlement(initial.EntitlementRefreshToken, host)
		if err != nil {
			t.Fatalf("refresh after %dh: %v", hours, err)
		}
		claims, err := pkglicensing.VerifyEntitlementLeaseToken(result.EntitlementJWT, pub, host, now)
		if err != nil {
			t.Fatal(err)
		}
		if claims.SubscriptionState != pkglicensing.SubStateActive || claims.PlanVersion != "cloud_starter" || !reflect.DeepEqual(claims.Capabilities, originalClaims.Capabilities) || claims.IssuedAt.Unix() != now.Unix() || claims.ExpiresAt.Unix() != now.Add(24*time.Hour).Unix() {
			t.Fatalf("invalid continuous legacy renewal after %dh: %+v", hours, claims)
		}
		stored, err := reg.GetHostedEntitlementByRefreshToken(initial.EntitlementRefreshToken)
		if err != nil || stored == nil || stored.ID != original.ID || stored.RefreshToken != original.RefreshToken || !stored.IssuedAt.Equal(original.IssuedAt) || stored.RevokedAt != nil || stored.LastRefreshedAt == nil || !stored.LastRefreshedAt.Equal(now) {
			t.Fatalf("renewal replaced saved identity after %dh: %+v, %v", hours, stored, err)
		}
	}

	// Missing legacy metadata is retryable, not permission to renew or revoke.
	before, err := reg.GetHostedEntitlementByRefreshToken(initial.EntitlementRefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	tenant.StripeSubscriptionID = ""
	if err := reg.Update(tenant); err != nil {
		t.Fatal(err)
	}
	now = started.Add(145 * time.Hour) // the last successfully renewed lease expired, too
	result, err := svc.RefreshEntitlement(initial.EntitlementRefreshToken, host)
	if result != nil || !errors.Is(err, ErrHostedEntitlementBillingUnavailable) {
		t.Fatalf("missing legacy authority: result present=%v err=%v", result != nil, err)
	}
	after, err := reg.GetHostedEntitlementByRefreshToken(initial.EntitlementRefreshToken)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("missing metadata changed saved authority: %v", err)
	}
	tenant.StripeSubscriptionID = "sub_legacy_paid"
	if err := reg.Update(tenant); err != nil {
		t.Fatal(err)
	}
	expectedTenant, err := reg.Get(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err = svc.RefreshEntitlement(initial.EntitlementRefreshToken, host)
	if err != nil {
		t.Fatalf("same-token legacy recovery: %v", err)
	}
	if _, err := pkglicensing.VerifyEntitlementLeaseToken(result.EntitlementJWT, pub, host, now); err != nil {
		t.Fatal(err)
	}
	current, err := reg.Get(tenant.ID)
	if err != nil || !reflect.DeepEqual(current, expectedTenant) {
		t.Fatalf("renewal changed saved tenant state: %+v, %v", current, err)
	}
	mappings, err := reg.ListStripeAccounts()
	if err != nil || len(mappings) != 0 {
		t.Fatalf("legacy renewal created billing authority: %v", err)
	}
}

func TestLegacyStripeTenantHonorsCurrentBilling(t *testing.T) {
	for _, state := range []string{"active", "canceled", "expired", "suspended", "unknown"} {
		t.Run(state, func(t *testing.T) {
			svc, _, reg := newTestService(t)
			tenant := legacyStripeTenant(t, reg)
			if err := reg.CreateAccount(&registry.Account{ID: "a_LEGACYMAPPED", Kind: registry.AccountKindIndividual}); err != nil {
				t.Fatal(err)
			}
			if err := reg.CreateStripeAccount(&registry.StripeAccount{AccountID: "a_LEGACYMAPPED", StripeCustomerID: tenant.StripeCustomerID, StripeSubscriptionID: tenant.StripeSubscriptionID, SubscriptionState: state, PlanVersion: "cloud_starter"}); err != nil {
				t.Fatal(err)
			}
			if _, _, err := reg.StoreOrIssueHostedEntitlement(tenant.ID, "etr_legacy_mapped", time.Now()); err != nil {
				t.Fatal(err)
			}
			result, err := svc.RefreshEntitlement("etr_legacy_mapped", "t-legacypaid.cloud.example.com")
			if state == "active" {
				if err != nil || result == nil {
					t.Fatalf("current active mapping did not renew: %v", err)
				}
			} else if result != nil || !errors.Is(err, ErrHostedEntitlementInactive) {
				t.Fatalf("legacy identity bypassed current denial: result present=%v err=%v", result != nil, err)
			}
		})
	}
}

func TestLegacyStripeTenantPreservesRefreshGuards(t *testing.T) {
	for _, tc := range []struct {
		name               string
		state              registry.TenantState
		revoked, wrongHost bool
		want               error
	}{
		{name: "provisioning", state: registry.TenantStateProvisioning, want: ErrHostedEntitlementNotReady},
		{name: "suspended", state: registry.TenantStateSuspended, want: ErrHostedEntitlementInactive},
		{name: "canceled", state: registry.TenantStateCanceled, want: ErrHostedEntitlementInactive},
		{name: "deleting", state: registry.TenantStateDeleting, want: ErrHostedEntitlementInactive},
		{name: "deleted", state: registry.TenantStateDeleted, want: ErrHostedEntitlementInactive},
		{name: "failed", state: registry.TenantStateFailed, want: ErrHostedEntitlementInactive},
		{name: "unknown", state: registry.TenantState("unknown"), want: ErrHostedEntitlementInactive},
		{name: "revoked", state: registry.TenantStateActive, revoked: true, want: ErrHostedEntitlementInactive},
		{name: "wrong_host", state: registry.TenantStateActive, wrongHost: true, want: ErrHostedEntitlementTargetMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, reg := newTestService(t)
			tenant := legacyStripeTenant(t, reg)
			tenant.State = tc.state
			if err := reg.Update(tenant); err != nil {
				t.Fatal(err)
			}
			if _, _, err := reg.StoreOrIssueHostedEntitlement(tenant.ID, "etr_legacy_guard", time.Now()); err != nil {
				t.Fatal(err)
			}
			if tc.revoked {
				if err := svc.RevokeTenantEntitlement(tenant.ID, time.Now()); err != nil {
					t.Fatal(err)
				}
			}
			host := "t-legacypaid.cloud.example.com"
			if tc.wrongHost {
				host = "other.cloud.example.com"
			}
			result, err := svc.RefreshEntitlement("etr_legacy_guard", host)
			if result != nil || !errors.Is(err, tc.want) {
				t.Fatalf("legacy bypassed guard: result present=%v err=%v want=%v", result != nil, err, tc.want)
			}
			stored, err := reg.GetHostedEntitlementByRefreshToken("etr_legacy_guard")
			if err != nil || stored == nil || stored.LastRefreshedAt != nil || (stored.RevokedAt != nil) != tc.revoked {
				t.Fatalf("denied refresh changed identity: %+v, %v", stored, err)
			}
		})
	}
}
