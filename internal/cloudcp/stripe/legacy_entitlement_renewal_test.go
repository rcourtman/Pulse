package stripe

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/lifecycle"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestLegacyStripeWebhookKeepsRenewalAndWithdrawalAuthoritative(t *testing.T) {
	for _, withdrawal := range []string{"canceled", "paused"} {
		t.Run(withdrawal, func(t *testing.T) {
			// Unpaid/past_due deliberately retain grace. Use actual terminal
			// or suspended statuses to exercise withdrawal, not positive sync.
			if ShouldGrantCapabilities(MapSubscriptionStatus(withdrawal)) {
				t.Fatal("withdrawal fixture selected a granting Stripe status")
			}
			reg := newStripeTestRegistry(t)
			dir := t.TempDir()
			p := newTestProvisioner(t, reg, dir, nil, true)
			tenant := &registry.Tenant{
				ID: "t-LEGACYHOOK", State: registry.TenantStateActive, Email: "fixture@example.invalid",
				StripeCustomerID: "cus_legacy_hook", StripeSubscriptionID: "sub_legacy_old",
				StripePriceID: "price_legacy_old", PlanVersion: "cloud_starter", ContainerID: "original-runtime",
			}
			if err := reg.Create(tenant); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(dir, tenant.ID), 0700); err != nil {
				t.Fatal(err)
			}
			sub := Subscription{ID: "sub_legacy_current", Customer: tenant.StripeCustomerID, Status: "active", Metadata: map[string]string{"plan_version": "cloud_power"}}
			if err := p.HandleSubscriptionUpdated(context.Background(), sub); err != nil {
				t.Fatalf("real legacy subscription update: %v", err)
			}
			current, err := reg.Get(tenant.ID)
			if err != nil || current == nil || current.AccountID != "" || current.StripeSubscriptionID != sub.ID || current.PlanVersion != "cloud_power" || current.ContainerID != tenant.ContainerID || current.State != registry.TenantStateActive {
				t.Fatalf("webhook lost legacy identity: %+v, %v", current, err)
			}
			mappings, err := reg.ListStripeAccounts()
			if err != nil || len(mappings) != 0 {
				t.Fatalf("webhook created consolidated billing: %v", err)
			}
			billing, err := config.NewFileBillingStore(filepath.Join(dir, tenant.ID)).GetBillingState("default")
			if err != nil || billing == nil || billing.EntitlementJWT == "" || billing.EntitlementRefreshToken == "" {
				t.Fatalf("webhook billing state: %+v, %v", billing, err)
			}
			pub, err := base64.StdEncoding.DecodeString(os.Getenv("PULSE_TRIAL_ACTIVATION_PUBLIC_KEY"))
			if err != nil {
				t.Fatal(err)
			}
			const host = "t-legacyhook.cloud.example.com"
			now := time.Now().UTC().Add(72 * time.Hour)
			p.hostedEntitlements.SetNow(func() time.Time { return now })
			result, err := p.hostedEntitlements.RefreshEntitlement(billing.EntitlementRefreshToken, host)
			if err != nil {
				t.Fatalf("legacy renewal beyond webhook-issued lease: %v", err)
			}
			claims, err := pkglicensing.VerifyEntitlementLeaseToken(result.EntitlementJWT, pub, host, now)
			if err != nil || claims.SubscriptionState != pkglicensing.SubStateActive || claims.PlanVersion != "cloud_power" || claims.ExpiresAt.Unix() != now.Add(24*time.Hour).Unix() {
				t.Fatalf("legacy renewed claims: %+v, %v", claims, err)
			}
			file := filepath.Join(dir, tenant.ID, "billing.json")
			before, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			_, held, err := reg.AcquireRuntimeLifecycle(context.Background(), tenant.ID)
			if err != nil {
				t.Fatal(err)
			}
			defer held.Close()
			sub.Status = withdrawal
			wantState := registry.TenantStateSuspended
			if withdrawal == "canceled" {
				err = p.HandleSubscriptionDeleted(context.Background(), sub)
				wantState = registry.TenantStateCanceled
			} else {
				err = p.HandleSubscriptionUpdated(context.Background(), sub)
			}
			if !errors.Is(err, lifecycle.ErrBusy) {
				t.Fatalf("expected billing-file deferral behind held runtime, got %v", err)
			}
			current, err = reg.Get(tenant.ID)
			if err != nil || current == nil || current.State != wantState || current.ContainerID != tenant.ContainerID || current.AccountID != "" {
				t.Fatalf("withdrawal did not preserve legacy runtime identity: %+v, %v", current, err)
			}
			authority, err := reg.GetHostedEntitlementByRefreshToken(billing.EntitlementRefreshToken)
			if err != nil || authority == nil || authority.RevokedAt == nil {
				t.Fatalf("legacy authority not withdrawn: %+v, %v", authority, err)
			}
			if result, err := p.hostedEntitlements.RefreshEntitlement(billing.EntitlementRefreshToken, host); result != nil || !errors.Is(err, entitlements.ErrHostedEntitlementInactive) {
				t.Fatalf("withdrawn legacy token renewed: result present=%v err=%v", result != nil, err)
			}
			after, err := os.ReadFile(file)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("held runtime billing file changed: %v", err)
			}
		})
	}
}
