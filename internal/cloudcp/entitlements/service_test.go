package entitlements

import (
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func newTestService(t *testing.T) (*Service, ed25519.PublicKey, *registry.TenantRegistry) {
	return newTestServiceWithBaseURL(t, "https://cloud.example.com")
}

func newTestServiceWithBaseURL(t *testing.T, baseURL string) (*Service, ed25519.PublicKey, *registry.TenantRegistry) {
	svc, pub, reg, _ := newTestServiceWithBaseURLAndDir(t, baseURL)
	return svc, pub, reg
}

func newTestServiceWithBaseURLAndDir(t *testing.T, baseURL string) (*Service, ed25519.PublicKey, *registry.TenantRegistry, string) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	svc := NewService(reg, baseURL, base64.StdEncoding.EncodeToString(priv))
	return svc, pub, reg, dir
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
		PlanVersion:       "cloud_starter",
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
		PlanVersion:      "cloud_starter",
	}
	if err := reg.Create(tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	svc.SetNow(func() time.Time { return now })

	state, err := svc.IssueTenantBillingState(tenant, pkglicensing.SubStateActive, "cloud_starter", "cus_paid_service", "sub_paid_service", "price_paid_service")
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
	if claims.PlanVersion != "cloud_starter" {
		t.Fatalf("claims.PlanVersion=%q, want %q", claims.PlanVersion, "cloud_starter")
	}
	if claims.SubscriptionState != pkglicensing.SubStateActive {
		t.Fatalf("claims.SubscriptionState=%q, want %q", claims.SubscriptionState, pkglicensing.SubStateActive)
	}
}

func TestIssueTenantBillingStateDefaultsIndividualCloudPlanToStarter(t *testing.T) {
	svc, pub, reg := newTestService(t)

	tenant := &registry.Tenant{
		ID:    "t-SERVICE04",
		Email: "owner@example.com",
		State: registry.TenantStateActive,
	}
	if err := reg.Create(tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}

	now := time.Unix(1710000100, 0).UTC()
	svc.SetNow(func() time.Time { return now })

	state, err := svc.IssueTenantBillingState(tenant, pkglicensing.SubStateActive, "", "", "", "")
	if err != nil {
		t.Fatalf("IssueTenantBillingState: %v", err)
	}
	if state == nil {
		t.Fatal("expected billing state")
	}

	claims, err := pkglicensing.VerifyEntitlementLeaseToken(state.EntitlementJWT, pub, "t-SERVICE04.cloud.example.com", now)
	if err != nil {
		t.Fatalf("VerifyEntitlementLeaseToken: %v", err)
	}
	if claims.PlanVersion != "cloud_starter" {
		t.Fatalf("claims.PlanVersion=%q, want %q", claims.PlanVersion, "cloud_starter")
	}
	if _, ok := claims.Limits["max_monitored_systems"]; ok {
		t.Fatalf("claims retained retired max_monitored_systems: %v", claims.Limits)
	}
}

func TestIssueTenantBillingStateCanonicalizesLegacyStoredPlanVersion(t *testing.T) {
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
		StripeCustomerID:  "cus_legacy_plan",
		PlanVersion:       "cloud-v1",
		SubscriptionState: "active",
	}); err != nil {
		t.Fatalf("CreateStripeAccount: %v", err)
	}

	tenant := &registry.Tenant{
		ID:               "t-SERVICE05",
		AccountID:        accountID,
		Email:            "owner@example.com",
		State:            registry.TenantStateActive,
		StripeCustomerID: "cus_legacy_plan",
		PlanVersion:      "cloud-v1",
	}
	if err := reg.Create(tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}

	now := time.Unix(1710000200, 0).UTC()
	svc.SetNow(func() time.Time { return now })

	state, err := svc.IssueTenantBillingState(tenant, pkglicensing.SubStateActive, "", "", "", "")
	if err != nil {
		t.Fatalf("IssueTenantBillingState: %v", err)
	}

	claims, err := pkglicensing.VerifyEntitlementLeaseToken(state.EntitlementJWT, pub, "t-SERVICE05.cloud.example.com", now)
	if err != nil {
		t.Fatalf("VerifyEntitlementLeaseToken: %v", err)
	}
	if claims.PlanVersion != "cloud_starter" {
		t.Fatalf("claims.PlanVersion=%q, want %q", claims.PlanVersion, "cloud_starter")
	}
	if _, ok := claims.Limits["max_monitored_systems"]; ok {
		t.Fatalf("claims retained retired max_monitored_systems: %v", claims.Limits)
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

func TestRefreshPaidEntitlementRejectsDeletedTenantWithActiveBilling(t *testing.T) {
	svc, _, reg := newTestService(t)

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
		StripeCustomerID:  "cus_active_deleted_tenant",
		PlanVersion:       "cloud_starter",
		SubscriptionState: "active",
	}); err != nil {
		t.Fatalf("CreateStripeAccount: %v", err)
	}

	tenant := &registry.Tenant{
		ID:        "t-SERVICE06",
		AccountID: accountID,
		Email:     "owner@example.com",
		State:     registry.TenantStateDeleted,
	}
	if err := reg.Create(tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, _, err := reg.StoreOrIssueHostedEntitlement(tenant.ID, "etr_deleted_tenant", time.Unix(1710000000, 0).UTC()); err != nil {
		t.Fatalf("StoreOrIssueHostedEntitlement: %v", err)
	}

	_, err = svc.RefreshEntitlement("etr_deleted_tenant", "t-service06.cloud.example.com")
	if err == nil {
		t.Fatal("expected inactive entitlement")
	}
	if err != ErrHostedEntitlementInactive {
		t.Fatalf("err=%v, want %v", err, ErrHostedEntitlementInactive)
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

func TestRefreshLegacyTrialEntitlementReturnsLease(t *testing.T) {
	svc, pub, reg, dir := newTestServiceWithBaseURLAndDir(t, "https://cloud.example.com")

	now := time.Unix(1710000000, 0).UTC()
	svc.SetNow(func() time.Time { return now })

	seedLegacyTrialHostedEntitlement(t, dir, now, "etr_legacy_trial")

	loaded, err := reg.GetHostedEntitlementByRefreshToken("etr_legacy_trial")
	if err != nil {
		t.Fatalf("GetHostedEntitlementByRefreshToken: %v", err)
	}
	if loaded == nil || loaded.Kind != registry.HostedEntitlementKindTrial {
		t.Fatalf("expected trial entitlement record, got %#v", loaded)
	}

	refreshResult, err := svc.RefreshEntitlement("etr_legacy_trial", "pulse.example.com")
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
	if _, ok := claims.Limits[pkglicensing.MaxMonitoredSystemsLicenseGateKey]; ok {
		t.Fatalf("claims retained retired %s: %v", pkglicensing.MaxMonitoredSystemsLicenseGateKey, claims.Limits)
	}
}

func seedLegacyTrialHostedEntitlement(t *testing.T, registryDir string, now time.Time, refreshToken string) {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(registryDir, "tenants.db"))
	if err != nil {
		t.Fatalf("open registry db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		INSERT INTO hosted_entitlements (
			id, kind, tenant_id, trial_request_id, org_id, email, return_url, instance_token, instance_host,
			trial_started_at, refresh_token, activation_token, issued_at, activation_issued_at, last_refreshed_at, redeemed_at, revoked_at
		) VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, NULL, NULL, ?, NULL)`,
		"trial:trial_request_1",
		string(registry.HostedEntitlementKindTrial),
		"trial_request_1",
		"default",
		"owner@example.com",
		"https://pulse.example.com/auth/trial-activate",
		"tsi_test",
		"pulse.example.com",
		now.Unix(),
		refreshToken,
		now.Unix(),
		now.Unix(),
	)
	if err != nil {
		t.Fatalf("seed legacy trial hosted entitlement: %v", err)
	}
}
