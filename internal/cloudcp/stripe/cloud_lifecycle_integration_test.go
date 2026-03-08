package stripe

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// TestCloudLifecycle_CheckoutToBillingToAgentLimits exercises the full Cloud
// individual tier lifecycle:
//
//	checkout.session.completed with plan_version metadata →
//	tenant provisioning with correct plan version →
//	billing state written with correct agent limits →
//	subscription update propagates new tier limits →
//	subscription cancellation revokes capabilities.
//
// This is an integration test that wires together the registry, provisioner,
// and entitlements service to verify that Cloud tier assignment is end-to-end
// correct for Starter (10 agents), Power (30), and Max (75).
func TestCloudLifecycle_CheckoutToBillingToAgentLimits(t *testing.T) {
	tests := []struct {
		name         string
		planVersion  string
		wantAgents   int64
		wantCaps     []string // subset of expected capabilities
		wantSubState pkglicensing.SubscriptionState
	}{
		{
			name:         "cloud_starter_via_metadata",
			planVersion:  "cloud_starter",
			wantAgents:   10,
			wantCaps:     []string{"ai_autofix", "relay", "mobile_app", "rbac"},
			wantSubState: pkglicensing.SubStateActive,
		},
		{
			name:         "cloud_power_via_metadata",
			planVersion:  "cloud_power",
			wantAgents:   30,
			wantCaps:     []string{"ai_autofix", "relay", "mobile_app", "rbac"},
			wantSubState: pkglicensing.SubStateActive,
		},
		{
			name:         "cloud_max_via_metadata",
			planVersion:  "cloud_max",
			wantAgents:   75,
			wantCaps:     []string{"ai_autofix", "relay", "mobile_app", "rbac"},
			wantSubState: pkglicensing.SubStateActive,
		},
		{
			name:         "cloud_founding_via_metadata",
			planVersion:  "cloud_founding",
			wantAgents:   10, // Founding rate = Starter limits
			wantCaps:     []string{"ai_autofix", "relay"},
			wantSubState: pkglicensing.SubStateActive,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reg := newStripeTestRegistry(t)
			tenantsDir := t.TempDir()
			provisioner := newTestProvisioner(t, reg, tenantsDir, nil, true)

			// Build checkout session with metadata or price-based resolution.
			session := CheckoutSession{
				Customer:      "cus_cloud_" + tc.name,
				Subscription:  "sub_cloud_" + tc.name,
				CustomerEmail: tc.name + "@example.com",
			}
			if tc.planVersion != "" {
				session.Metadata = map[string]string{"plan_version": tc.planVersion}
			}

			// Execute HandleCheckout — this is the real entry point for
			// checkout.session.completed webhook events.
			if err := provisioner.HandleCheckout(context.Background(), session); err != nil {
				t.Fatalf("HandleCheckout: %v", err)
			}

			// ── Verify tenant registry record ───────────────────────────
			tenant, err := reg.GetByStripeCustomerID(session.Customer)
			if err != nil {
				t.Fatalf("GetByStripeCustomerID: %v", err)
			}
			if tenant == nil {
				t.Fatal("expected tenant to exist after checkout")
			}
			if tenant.State != registry.TenantStateActive {
				t.Fatalf("tenant.State = %q, want %q", tenant.State, registry.TenantStateActive)
			}

			if tenant.PlanVersion != tc.planVersion {
				t.Fatalf("tenant.PlanVersion = %q, want %q", tenant.PlanVersion, tc.planVersion)
			}

			// ── Verify account creation and Stripe mapping ──────────────
			if strings.TrimSpace(tenant.AccountID) == "" {
				t.Fatal("tenant.AccountID is empty — account was not created")
			}
			acct, err := reg.GetAccount(tenant.AccountID)
			if err != nil {
				t.Fatalf("GetAccount: %v", err)
			}
			if acct == nil {
				t.Fatal("expected account to exist for Cloud tenant")
			}
			if acct.Kind != registry.AccountKindIndividual {
				t.Fatalf("account.Kind = %q, want %q", acct.Kind, registry.AccountKindIndividual)
			}

			sa, err := reg.GetStripeAccount(tenant.AccountID)
			if err != nil {
				t.Fatalf("GetStripeAccount: %v", err)
			}
			if sa == nil {
				t.Fatal("expected StripeAccount mapping to exist")
			}
			if sa.StripeCustomerID != session.Customer {
				t.Fatalf("StripeAccount.StripeCustomerID = %q, want %q", sa.StripeCustomerID, session.Customer)
			}
			if sa.StripeSubscriptionID != session.Subscription {
				t.Fatalf("StripeAccount.StripeSubscriptionID = %q, want %q", sa.StripeSubscriptionID, session.Subscription)
			}

			// ── Verify billing state and agent limits ───────────────────
			store := config.NewFileBillingStore(provisioner.tenantDataDir(tenant.ID))
			bs, err := store.GetBillingState("default")
			if err != nil {
				t.Fatalf("GetBillingState: %v", err)
			}
			if bs == nil {
				t.Fatal("billing state is nil")
			}
			if bs.SubscriptionState != tc.wantSubState {
				t.Fatalf("billing.SubscriptionState = %q, want %q", bs.SubscriptionState, tc.wantSubState)
			}

			// Agent limit is the critical assertion — this is what the Pulse
			// runtime reads to enforce agent caps for Cloud tenants.
			if bs.Limits["max_agents"] != tc.wantAgents {
				t.Fatalf("billing.Limits[max_agents] = %d, want %d", bs.Limits["max_agents"], tc.wantAgents)
			}

			// Verify capabilities include Pro-level features.
			capSet := make(map[string]struct{}, len(bs.Capabilities))
			for _, c := range bs.Capabilities {
				capSet[c] = struct{}{}
			}
			for _, want := range tc.wantCaps {
				if _, ok := capSet[want]; !ok {
					t.Fatalf("billing.Capabilities missing %q; got %v", want, bs.Capabilities)
				}
			}

			// Verify entitlement lease tokens were written and raw state is
			// lease-only (no redundant SubscriptionState/Capabilities in the
			// raw file — those are derived at read time from the lease).
			raw := loadRawBillingState(t, provisioner.tenantDataDir(tenant.ID))
			if strings.TrimSpace(raw.EntitlementJWT) == "" {
				t.Fatal("raw billing state missing EntitlementJWT")
			}
			if strings.TrimSpace(raw.EntitlementRefreshToken) == "" {
				t.Fatal("raw billing state missing EntitlementRefreshToken")
			}
			if raw.SubscriptionState != "" || len(raw.Capabilities) != 0 {
				t.Fatalf("expected raw lease-only state (no SubscriptionState/Capabilities), got sub_state=%q caps=%v", raw.SubscriptionState, raw.Capabilities)
			}
		})
	}
}

// TestCloudLifecycle_SubscriptionUpdateChangesLimits verifies that when a
// Cloud tenant upgrades (e.g., Starter → Power), the subscription.updated
// webhook correctly updates both the tenant record and billing state with
// the new plan's agent limits.
func TestCloudLifecycle_SubscriptionUpdateChangesLimits(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	provisioner := newTestProvisioner(t, reg, tenantsDir, nil, true)

	// Phase 1: Checkout as Cloud Starter (10 agents).
	session := CheckoutSession{
		Customer:      "cus_upgrade_test",
		Subscription:  "sub_upgrade_test",
		CustomerEmail: "upgrader@example.com",
		Metadata:      map[string]string{"plan_version": "cloud_starter"},
	}
	if err := provisioner.HandleCheckout(context.Background(), session); err != nil {
		t.Fatalf("HandleCheckout: %v", err)
	}

	tenant, err := reg.GetByStripeCustomerID("cus_upgrade_test")
	if err != nil || tenant == nil {
		t.Fatalf("lookup tenant: %v (tenant=%v)", err, tenant)
	}

	// Verify initial state: Starter = 10 agents.
	store := config.NewFileBillingStore(provisioner.tenantDataDir(tenant.ID))
	bs, err := store.GetBillingState("default")
	if err != nil || bs == nil {
		t.Fatalf("initial GetBillingState: %v", err)
	}
	if bs.Limits["max_agents"] != 10 {
		t.Fatalf("initial max_agents = %d, want 10", bs.Limits["max_agents"])
	}

	// Phase 2: Simulate subscription.updated → upgrade to Cloud Power (30 agents).
	sub := Subscription{
		ID:       "sub_upgrade_test",
		Customer: "cus_upgrade_test",
		Status:   "active",
		Metadata: map[string]string{"plan_version": "cloud_power"},
	}
	if err := provisioner.HandleSubscriptionUpdated(context.Background(), sub); err != nil {
		t.Fatalf("HandleSubscriptionUpdated (upgrade to power): %v", err)
	}

	// Verify tenant record updated.
	tenant, err = reg.GetByStripeCustomerID("cus_upgrade_test")
	if err != nil || tenant == nil {
		t.Fatalf("lookup tenant after upgrade: %v", err)
	}
	if tenant.PlanVersion != "cloud_power" {
		t.Fatalf("tenant.PlanVersion after upgrade = %q, want %q", tenant.PlanVersion, "cloud_power")
	}
	if tenant.State != registry.TenantStateActive {
		t.Fatalf("tenant.State after upgrade = %q, want %q", tenant.State, registry.TenantStateActive)
	}

	// Verify billing state has new limits.
	bs, err = store.GetBillingState("default")
	if err != nil || bs == nil {
		t.Fatalf("GetBillingState after upgrade: %v", err)
	}
	if bs.Limits["max_agents"] != 30 {
		t.Fatalf("max_agents after upgrade = %d, want 30", bs.Limits["max_agents"])
	}
	if bs.SubscriptionState != pkglicensing.SubStateActive {
		t.Fatalf("SubscriptionState after upgrade = %q, want %q", bs.SubscriptionState, pkglicensing.SubStateActive)
	}

	// Phase 3: Upgrade again to Cloud Max (75 agents).
	sub.Metadata = map[string]string{"plan_version": "cloud_max"}
	if err := provisioner.HandleSubscriptionUpdated(context.Background(), sub); err != nil {
		t.Fatalf("HandleSubscriptionUpdated (upgrade to max): %v", err)
	}

	bs, err = store.GetBillingState("default")
	if err != nil || bs == nil {
		t.Fatalf("GetBillingState after max upgrade: %v", err)
	}
	if bs.Limits["max_agents"] != 75 {
		t.Fatalf("max_agents after max upgrade = %d, want 75", bs.Limits["max_agents"])
	}
}

// TestCloudLifecycle_CancellationRevokesCapabilities proves that
// subscription.deleted correctly revokes all capabilities and clears
// entitlement lease tokens for Cloud tenants.
func TestCloudLifecycle_CancellationRevokesCapabilities(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	provisioner := newTestProvisioner(t, reg, tenantsDir, nil, true)

	// Provision a Cloud Starter tenant.
	session := CheckoutSession{
		Customer:      "cus_cancel_test",
		Subscription:  "sub_cancel_test",
		CustomerEmail: "canceller@example.com",
		Metadata:      map[string]string{"plan_version": "cloud_starter"},
	}
	if err := provisioner.HandleCheckout(context.Background(), session); err != nil {
		t.Fatalf("HandleCheckout: %v", err)
	}

	tenant, err := reg.GetByStripeCustomerID("cus_cancel_test")
	if err != nil || tenant == nil {
		t.Fatalf("lookup tenant: %v", err)
	}

	// Verify active state before cancellation.
	store := config.NewFileBillingStore(provisioner.tenantDataDir(tenant.ID))
	bs, err := store.GetBillingState("default")
	if err != nil || bs == nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if len(bs.Capabilities) == 0 {
		t.Fatal("expected capabilities before cancellation, got empty")
	}
	if bs.Limits["max_agents"] != 10 {
		t.Fatalf("max_agents before cancel = %d, want 10", bs.Limits["max_agents"])
	}

	// Simulate subscription.deleted.
	delSub := Subscription{
		ID:       "sub_cancel_test",
		Customer: "cus_cancel_test",
		Status:   "canceled",
	}
	if err := provisioner.HandleSubscriptionDeleted(context.Background(), delSub); err != nil {
		t.Fatalf("HandleSubscriptionDeleted: %v", err)
	}

	// Verify tenant state is canceled.
	tenant, err = reg.GetByStripeCustomerID("cus_cancel_test")
	if err != nil || tenant == nil {
		t.Fatalf("lookup tenant after cancel: %v", err)
	}
	if tenant.State != registry.TenantStateCanceled {
		t.Fatalf("tenant.State = %q, want %q", tenant.State, registry.TenantStateCanceled)
	}

	// Verify billing state: capabilities revoked, canceled state.
	bs, err = store.GetBillingState("default")
	if err != nil || bs == nil {
		t.Fatalf("GetBillingState after cancel: %v", err)
	}
	if bs.SubscriptionState != pkglicensing.SubStateCanceled {
		t.Fatalf("SubscriptionState = %q, want %q", bs.SubscriptionState, pkglicensing.SubStateCanceled)
	}
	if len(bs.Capabilities) != 0 {
		t.Fatalf("expected empty capabilities after cancellation, got %v", bs.Capabilities)
	}

	// Verify entitlement lease tokens were cleared.
	raw := loadRawBillingState(t, provisioner.tenantDataDir(tenant.ID))
	if raw.EntitlementJWT != "" {
		t.Fatalf("expected empty EntitlementJWT after cancel, got %q", raw.EntitlementJWT)
	}
	if raw.EntitlementRefreshToken != "" {
		t.Fatalf("expected empty EntitlementRefreshToken after cancel, got %q", raw.EntitlementRefreshToken)
	}

	// Verify account-level Stripe state after cancellation (matches MSP test parity).
	sa, err := reg.GetStripeAccountByCustomerID("cus_cancel_test")
	if err != nil {
		t.Fatalf("GetStripeAccountByCustomerID after cancel: %v", err)
	}
	if sa == nil {
		t.Fatal("expected StripeAccount to exist after cancellation")
	}
	if sa.SubscriptionState != "canceled" {
		t.Fatalf("StripeAccount.SubscriptionState = %q, want %q", sa.SubscriptionState, "canceled")
	}
	if sa.GraceStartedAt != nil {
		t.Fatalf("StripeAccount.GraceStartedAt after cancel = %v, want nil", sa.GraceStartedAt)
	}
}

// TestCloudLifecycle_GracePeriodPreservesAccess proves that a past_due
// subscription transitions the Cloud tenant to grace state (still active)
// with capabilities preserved, matching the behavior of MSP tenants.
func TestCloudLifecycle_GracePeriodPreservesAccess(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	provisioner := newTestProvisioner(t, reg, tenantsDir, nil, true)

	session := CheckoutSession{
		Customer:      "cus_grace_test",
		Subscription:  "sub_grace_test",
		CustomerEmail: "grace@example.com",
		Metadata:      map[string]string{"plan_version": "cloud_power"},
	}
	if err := provisioner.HandleCheckout(context.Background(), session); err != nil {
		t.Fatalf("HandleCheckout: %v", err)
	}

	tenant, err := reg.GetByStripeCustomerID("cus_grace_test")
	if err != nil || tenant == nil {
		t.Fatalf("lookup tenant: %v", err)
	}

	// Simulate past_due → grace period.
	sub := Subscription{
		ID:       "sub_grace_test",
		Customer: "cus_grace_test",
		Status:   "past_due",
		Metadata: map[string]string{"plan_version": "cloud_power"},
	}
	if err := provisioner.HandleSubscriptionUpdated(context.Background(), sub); err != nil {
		t.Fatalf("HandleSubscriptionUpdated (past_due): %v", err)
	}

	// Tenant should remain active during grace.
	tenant, err = reg.GetByStripeCustomerID("cus_grace_test")
	if err != nil || tenant == nil {
		t.Fatalf("lookup tenant after past_due: %v", err)
	}
	if tenant.State != registry.TenantStateActive {
		t.Fatalf("tenant.State during grace = %q, want %q", tenant.State, registry.TenantStateActive)
	}

	// Billing state should be grace with capabilities preserved.
	store := config.NewFileBillingStore(provisioner.tenantDataDir(tenant.ID))
	bs, err := store.GetBillingState("default")
	if err != nil || bs == nil {
		t.Fatalf("GetBillingState during grace: %v", err)
	}
	if bs.SubscriptionState != pkglicensing.SubStateGrace {
		t.Fatalf("SubscriptionState = %q, want %q", bs.SubscriptionState, pkglicensing.SubStateGrace)
	}
	if len(bs.Capabilities) == 0 {
		t.Fatal("expected capabilities during grace, got empty")
	}
	if bs.Limits["max_agents"] != 30 {
		t.Fatalf("max_agents during grace = %d, want 30 (Cloud Power)", bs.Limits["max_agents"])
	}

	// Stripe account should have grace window started.
	sa, err := reg.GetStripeAccountByCustomerID("cus_grace_test")
	if err != nil {
		t.Fatalf("GetStripeAccountByCustomerID: %v", err)
	}
	if sa == nil {
		t.Fatal("expected StripeAccount to exist")
	}
	if sa.GraceStartedAt == nil || *sa.GraceStartedAt <= 0 {
		t.Fatalf("StripeAccount.GraceStartedAt = %v, want non-nil positive timestamp", sa.GraceStartedAt)
	}
}

// TestCloudLifecycle_PriceIDResolution verifies that when checkout metadata
// does NOT contain plan_version, the system correctly resolves the plan
// from the Stripe price ID using the canonical PriceIDToPlanVersion map.
// This is the fallback path when metadata is missing or checkout was created
// outside the control plane (e.g., directly in Stripe Dashboard).
func TestCloudLifecycle_PriceIDResolution(t *testing.T) {
	tests := []struct {
		name       string
		priceID    string
		wantPlan   string
		wantAgents int64
	}{
		{"starter_monthly", "price_1T5kflBrHBocJIGHUqPv1dzV", "cloud_starter", 10},
		{"starter_annual", "price_1T5kfmBrHBocJIGHTS3ymKxM", "cloud_starter", 10},
		{"founding_monthly", "price_1T5kfnBrHBocJIGHATQJr79D", "cloud_founding", 10},
		{"power_monthly", "price_1T5kg2BrHBocJIGHmkoF0zXY", "cloud_power", 30},
		{"power_annual", "price_1T5kg3BrHBocJIGH2EtzKofV", "cloud_power", 30},
		{"max_monthly", "price_1T5kg4BrHBocJIGHHa8Ecqho", "cloud_max", 75},
		{"max_annual", "price_1T5kg5BrHBocJIGH5AIJ4nVc", "cloud_max", 75},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reg := newStripeTestRegistry(t)
			tenantsDir := t.TempDir()
			provisioner := newTestProvisioner(t, reg, tenantsDir, nil, true)

			// Checkout with NO metadata — only a subscription with a price item.
			// HandleCheckout uses DerivePlanVersion which falls through to
			// PriceIDToPlanVersion when metadata is empty.
			session := CheckoutSession{
				Customer:      "cus_price_" + tc.name,
				Subscription:  "sub_price_" + tc.name,
				CustomerEmail: tc.name + "@example.com",
				// No Metadata — plan resolution must happen via subscription update.
			}
			if err := provisioner.HandleCheckout(context.Background(), session); err != nil {
				t.Fatalf("HandleCheckout: %v", err)
			}

			// The initial checkout without metadata will have a generic plan version.
			// Simulate the subscription.updated event that carries the actual price ID.
			sub := Subscription{
				ID:       "sub_price_" + tc.name,
				Customer: "cus_price_" + tc.name,
				Status:   "active",
			}
			sub.Items.Data = []struct {
				Price struct {
					ID       string            `json:"id"`
					Metadata map[string]string `json:"metadata"`
				} `json:"price"`
			}{
				{Price: struct {
					ID       string            `json:"id"`
					Metadata map[string]string `json:"metadata"`
				}{ID: tc.priceID}},
			}
			if err := provisioner.HandleSubscriptionUpdated(context.Background(), sub); err != nil {
				t.Fatalf("HandleSubscriptionUpdated: %v", err)
			}

			tenant, err := reg.GetByStripeCustomerID(session.Customer)
			if err != nil || tenant == nil {
				t.Fatalf("lookup tenant: %v", err)
			}
			if tenant.PlanVersion != tc.wantPlan {
				t.Fatalf("tenant.PlanVersion = %q, want %q", tenant.PlanVersion, tc.wantPlan)
			}
			// Verify price ID was persisted on the tenant record (used by
			// stale-plan preservation logic in HandleSubscriptionUpdated).
			if tenant.StripePriceID != tc.priceID {
				t.Fatalf("tenant.StripePriceID = %q, want %q", tenant.StripePriceID, tc.priceID)
			}

			store := config.NewFileBillingStore(provisioner.tenantDataDir(tenant.ID))
			bs, err := store.GetBillingState("default")
			if err != nil || bs == nil {
				t.Fatalf("GetBillingState: %v", err)
			}
			if bs.Limits["max_agents"] != tc.wantAgents {
				t.Fatalf("max_agents = %d, want %d", bs.Limits["max_agents"], tc.wantAgents)
			}
		})
	}
}

// TestCloudLifecycle_WorkspaceLimitEnforcement verifies that Cloud individual
// accounts are limited to 1 workspace (as per CloudPlanWorkspaceLimits).
// After checkout creates the first workspace, attempting a second via the
// HandleCreateTenant HTTP handler must return 403 Forbidden.
func TestCloudLifecycle_WorkspaceLimitEnforcement(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	provisioner := newTestProvisioner(t, reg, tenantsDir, nil, true)

	// Provision a Cloud Starter tenant via checkout.
	session := CheckoutSession{
		Customer:      "cus_wslimit_test",
		Subscription:  "sub_wslimit_test",
		CustomerEmail: "wslimit@example.com",
		Metadata:      map[string]string{"plan_version": "cloud_starter"},
	}
	if err := provisioner.HandleCheckout(context.Background(), session); err != nil {
		t.Fatalf("HandleCheckout: %v", err)
	}

	tenant, err := reg.GetByStripeCustomerID("cus_wslimit_test")
	if err != nil || tenant == nil {
		t.Fatalf("lookup tenant: %v", err)
	}

	// Verify workspace limit from plan version.
	limit, known := pkglicensing.WorkspaceLimitForPlan("cloud_starter")
	if !known {
		t.Fatal("cloud_starter should be a known plan")
	}
	if limit != 1 {
		t.Fatalf("WorkspaceLimitForPlan(cloud_starter) = %d, want 1", limit)
	}

	// Count active workspaces for this account — should be 1 after checkout.
	count, err := reg.CountActiveByAccountID(tenant.AccountID)
	if err != nil {
		t.Fatalf("CountActiveByAccountID: %v", err)
	}
	if count != 1 {
		t.Fatalf("active workspace count = %d, want 1", count)
	}

	// Exercise the actual HTTP handler to prove workspace creation is blocked.
	tenantMux := newTenantMux(reg, provisioner)
	createBody := `{"display_name":"Second Workspace (should fail)"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/accounts/"+tenant.AccountID+"/tenants", bytes.NewBufferString(createBody))
	createReq.Header.Set("X-Admin-Key", "secret-key")
	createRec := httptest.NewRecorder()
	tenantMux.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusForbidden {
		t.Fatalf("second workspace creation: status = %d, want %d (body=%q)", createRec.Code, http.StatusForbidden, createRec.Body.String())
	}

	// Verify the response body mentions the workspace limit.
	body := createRec.Body.String()
	if !strings.Contains(body, "workspace limit") {
		t.Fatalf("expected workspace limit error message, got %q", body)
	}

	// Confirm only 1 workspace still exists.
	countAfter, err := reg.CountActiveByAccountID(tenant.AccountID)
	if err != nil {
		t.Fatalf("CountActiveByAccountID after blocked creation: %v", err)
	}
	if countAfter != 1 {
		t.Fatalf("workspace count after blocked creation = %d, want 1", countAfter)
	}
}
