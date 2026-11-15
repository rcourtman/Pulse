package api

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestHostedEntitlementLegacyBillingPreservesClientRecovery(t *testing.T) {
	f := newHostedAuthorityFixture(t)
	tenant, err := f.reg.Get("t-AUTHORITY")
	if err != nil || tenant == nil {
		t.Fatalf("tenant fixture: %v", err)
	}
	tenant.AccountID = ""
	tenant.StripeCustomerID = "cus_legacy_client"
	tenant.StripeSubscriptionID = "sub_legacy_client"
	if err := f.reg.Update(tenant); err != nil {
		t.Fatal(err)
	}
	// The original explicit lease expired 24 hours ago. This exercises the
	// real HTTP client, issuer, encrypted store and runtime evaluator, not
	// another explicit webhook issuance that only lasts for one day.
	refreshed, permanent, err := f.handlers.refreshHostedEntitlementLeaseOnce("default", nil)
	if err != nil || !refreshed || permanent {
		t.Fatalf("legacy expired-lease refresh: refreshed=%v permanent=%v err=%v", refreshed, permanent, err)
	}
	saved, err := f.store.GetBillingState("default")
	if err != nil || saved == nil || saved.EntitlementRefreshToken != f.original.EntitlementRefreshToken || saved.EntitlementJWT == f.original.EntitlementJWT {
		t.Fatalf("legacy renewal lost client identity: %v", err)
	}
	evaluator := newLicenseEvaluatorForBillingStoreFromLicensing(f.store, "default", 0, "t-authority.cloud.example.com")
	if evaluator.SubscriptionState() != pkglicensing.SubStateActive || !evaluator.HasCapability(pkglicensing.FeatureAIAutoFix) {
		t.Fatal("legacy renewal did not restore paid runtime capabilities")
	}
	file := filepath.Join(f.baseDir, "billing.json")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	tenant.StripeSubscriptionID = ""
	if err := f.reg.Update(tenant); err != nil {
		t.Fatal(err)
	}
	refreshed, permanent, err = f.handlers.refreshHostedEntitlementLeaseOnce("default", nil)
	var refreshErr *hostedEntitlementRefreshError
	if refreshed || permanent || !errors.As(err, &refreshErr) || refreshErr.statusCode != http.StatusServiceUnavailable {
		t.Fatalf("missing legacy identity: refreshed=%v permanent=%v err=%v", refreshed, permanent, err)
	}
	after, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("missing legacy metadata rewrote encrypted client state: %v", err)
	}
	tenant.StripeSubscriptionID = "sub_legacy_client"
	if err := f.reg.Update(tenant); err != nil {
		t.Fatal(err)
	}
	refreshed, permanent, err = f.handlers.refreshHostedEntitlementLeaseOnce("default", nil)
	if err != nil || !refreshed || permanent {
		t.Fatalf("same-token legacy recovery: refreshed=%v permanent=%v err=%v", refreshed, permanent, err)
	}
	saved, err = f.store.GetBillingState("default")
	if err != nil || saved == nil || saved.EntitlementRefreshToken != f.original.EntitlementRefreshToken {
		t.Fatalf("legacy recovery replaced original identity: %v", err)
	}
}
