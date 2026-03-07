package entitlements

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func newTestService(t *testing.T) (*Service, ed25519.PublicKey, *registry.TenantRegistry) {
	return newTestServiceWithBaseURL(t, "https://cloud.example.com")
}

func newTestServiceWithBaseURL(t *testing.T, baseURL string) (*Service, ed25519.PublicKey, *registry.TenantRegistry) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	reg, err := registry.NewTenantRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	svc := NewService(reg, baseURL, base64.StdEncoding.EncodeToString(priv))
	return svc, pub, reg
}

func TestIssueTenantBillingStateReturnsLeaseOnlyState(t *testing.T) {
	svc, pub, reg := newTestService(t)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatalf("GenerateAccountID: %v", err)
	}
	if err := reg.CreateAccount(&registry.Account{
		ID:          accountID,
		Kind:        registry.AccountKindIndividual,
		DisplayName: "Pulse Labs",
	}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if err := reg.CreateStripeAccount(&registry.StripeAccount{
		AccountID:         accountID,
		StripeCustomerID:  "cus_paid_service",
		PlanVersion:       "cloud_v1",
		SubscriptionState: "active",
	}); err != nil {
		t.Fatalf("CreateStripeAccount: %v", err)
	}

	tenant := &registry.Tenant{
		ID:               "t-SERVICE01",
		AccountID:        accountID,
		Email:            "owner@example.com",
		State:            registry.TenantStateActive,
		StripeCustomerID: "cus_paid_service",
		PlanVersion:      "cloud_v1",
	}
	if err := reg.Create(tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	svc.SetNow(func() time.Time { return now })

	state, err := svc.IssueTenantBillingState(tenant, pkglicensing.SubStateActive, "cloud_v1", "cus_paid_service", "sub_paid_service", "price_paid_service")
	if err != nil {
		t.Fatalf("IssueTenantBillingState: %v", err)
	}
	if state == nil {
		t.Fatal("expected billing state")
	}
	if state.EntitlementJWT == "" || state.EntitlementRefreshToken == "" {
		t.Fatalf("expected signed lease and refresh token, got %#v", state)
	}
	if state.PlanVersion != "" || state.SubscriptionState != "" {
		t.Fatalf("expected lease-only state, got plan=%q subscription=%q", state.PlanVersion, state.SubscriptionState)
	}

	claims, err := pkglicensing.VerifyEntitlementLeaseToken(state.EntitlementJWT, pub, "t-SERVICE01.cloud.example.com", now)
	if err != nil {
		t.Fatalf("VerifyEntitlementLeaseToken: %v", err)
	}
	if claims.PlanVersion != "cloud_v1" {
		t.Fatalf("claims.PlanVersion=%q, want %q", claims.PlanVersion, "cloud_v1")
	}
	if claims.SubscriptionState != pkglicensing.SubStateActive {
		t.Fatalf("claims.SubscriptionState=%q, want %q", claims.SubscriptionState, pkglicensing.SubStateActive)
	}
}

func TestRefreshPaidEntitlementRejectsTargetMismatch(t *testing.T) {
	svc, _, reg := newTestService(t)

	tenant := &registry.Tenant{
		ID:    "t-SERVICE02",
		Email: "owner@example.com",
		State: registry.TenantStateActive,
	}
	if err := reg.Create(tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, _, err := reg.StoreOrIssueHostedEntitlement(tenant.ID, "etr_paid_service", time.Unix(1710000000, 0).UTC()); err != nil {
		t.Fatalf("StoreOrIssueHostedEntitlement: %v", err)
	}

	_, err := svc.RefreshEntitlement("etr_paid_service", "wrong.cloud.example.com")
	if err == nil {
		t.Fatal("expected target mismatch")
	}
	if err != ErrHostedEntitlementTargetMismatch {
		t.Fatalf("err=%v, want %v", err, ErrHostedEntitlementTargetMismatch)
	}
}

func TestRefreshPaidEntitlementCanonicalizesBaseURLForExpectedHost(t *testing.T) {
	svc, pub, reg := newTestServiceWithBaseURL(t, "https://Cloud.Example.com:8443/admin")

	tenant := &registry.Tenant{
		ID:    "T-SERVICE03",
		Email: "owner@example.com",
		State: registry.TenantStateActive,
	}
	if err := reg.Create(tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	svc.SetNow(func() time.Time { return now })
	if _, _, err := reg.StoreOrIssueHostedEntitlement(tenant.ID, "etr_paid_canonical", now); err != nil {
		t.Fatalf("StoreOrIssueHostedEntitlement: %v", err)
	}

	result, err := svc.RefreshEntitlement("etr_paid_canonical", "t-service03.cloud.example.com")
	if err != nil {
		t.Fatalf("RefreshEntitlement: %v", err)
	}

	claims, err := pkglicensing.VerifyEntitlementLeaseToken(result.EntitlementJWT, pub, "t-service03.cloud.example.com", now)
	if err != nil {
		t.Fatalf("VerifyEntitlementLeaseToken: %v", err)
	}
	if claims.InstanceHost != "t-service03.cloud.example.com" {
		t.Fatalf("claims.InstanceHost=%q, want %q", claims.InstanceHost, "t-service03.cloud.example.com")
	}
}

func TestRedeemTrialEntitlementAndRefresh(t *testing.T) {
	svc, pub, reg := newTestServiceWithBaseURL(t, "https://cloud.example.com")

	now := time.Unix(1710000000, 0).UTC()
	svc.SetNow(func() time.Time { return now })

	redemption, err := svc.RedeemTrialEntitlement(TrialEntitlementInput{
		RequestID:      "trial_request_1",
		OrgID:          "default",
		Email:          "owner@example.com",
		ReturnURL:      "https://pulse.example.com/auth/trial-activate",
		InstanceToken:  "tsi_test",
		InstanceHost:   "pulse.example.com",
		TrialStartedAt: now,
		IssuedAt:       now,
		RedeemedAt:     now,
	})
	if err != nil {
		t.Fatalf("RedeemTrialEntitlement: %v", err)
	}
	if redemption.EntitlementJWT == "" || redemption.EntitlementRefreshToken == "" {
		t.Fatalf("expected lease and refresh token, got %#v", redemption)
	}

	loaded, err := reg.GetHostedEntitlementByRefreshToken(redemption.EntitlementRefreshToken)
	if err != nil {
		t.Fatalf("GetHostedEntitlementByRefreshToken: %v", err)
	}
	if loaded == nil || loaded.Kind != registry.HostedEntitlementKindTrial {
		t.Fatalf("expected trial entitlement record, got %#v", loaded)
	}

	refreshResult, err := svc.RefreshEntitlement(redemption.EntitlementRefreshToken, "pulse.example.com")
	if err != nil {
		t.Fatalf("RefreshEntitlement: %v", err)
	}
	claims, err := pkglicensing.VerifyEntitlementLeaseToken(refreshResult.EntitlementJWT, pub, "pulse.example.com", now)
	if err != nil {
		t.Fatalf("VerifyEntitlementLeaseToken: %v", err)
	}
	if claims.SubscriptionState != pkglicensing.SubStateTrial {
		t.Fatalf("claims.SubscriptionState=%q, want %q", claims.SubscriptionState, pkglicensing.SubStateTrial)
	}
}
