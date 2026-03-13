package stripe

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/account"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/admin"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/portal"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// TestMSPLifecycle_AccountToPortal exercises the full MSP lifecycle:
//
//	MSP account creation → workspace provisioning → member management →
//	billing state propagation → portal dashboard verification → subscription
//	updates cascade to all workspaces → subscription deletion revokes access.
//
// This is an integration test that wires together the registry, provisioner,
// account/member handlers, and portal handlers to verify cross-component behavior.
func TestMSPLifecycle_AccountToPortal(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	provisioner := newTestProvisioner(t, reg, tenantsDir, nil, true)
	var chowned []string
	provisioner.chownFile = func(path string, uid, gid int) error {
		chowned = append(chowned, path)
		if uid != hostedTenantRuntimeUID || gid != hostedTenantRuntimeGID {
			t.Fatalf("unexpected hosted runtime ownership target uid=%d gid=%d", uid, gid)
		}
		return nil
	}

	// ── Phase 1: MSP account creation ──────────────────────────────────

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{
		ID:          accountID,
		Kind:        registry.AccountKindMSP,
		DisplayName: "Acme MSP",
	}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	got, err := reg.GetAccount(accountID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if got == nil {
		t.Fatal("expected account to exist")
	}
	if got.Kind != registry.AccountKindMSP {
		t.Fatalf("account.Kind = %q, want %q", got.Kind, registry.AccountKindMSP)
	}
	if got.DisplayName != "Acme MSP" {
		t.Fatalf("account.DisplayName = %q, want %q", got.DisplayName, "Acme MSP")
	}

	// Wire Stripe account mapping early so HandleCreateTenant workspace
	// limit enforcement can look up the billing record (Phase 2b needs it).
	if err := reg.CreateStripeAccount(&registry.StripeAccount{
		AccountID:        accountID,
		StripeCustomerID: "cus_msp_lifecycle_test",
		PlanVersion:      "msp_starter",
	}); err != nil {
		t.Fatalf("CreateStripeAccount: %v", err)
	}

	// ── Phase 2: Workspace provisioning ────────────────────────────────

	ws1, err := provisioner.ProvisionWorkspace(context.Background(), accountID, "Client One")
	if err != nil {
		t.Fatalf("ProvisionWorkspace(Client One): %v", err)
	}
	ws2, err := provisioner.ProvisionWorkspace(context.Background(), accountID, "Client Two")
	if err != nil {
		t.Fatalf("ProvisionWorkspace(Client Two): %v", err)
	}

	// Verify both workspaces created with correct state and billing.
	for _, ws := range []*registry.Tenant{ws1, ws2} {
		if ws.AccountID != accountID {
			t.Fatalf("workspace %s: AccountID = %q, want %q", ws.ID, ws.AccountID, accountID)
		}
		if ws.State != registry.TenantStateActive {
			t.Fatalf("workspace %s: State = %q, want %q", ws.ID, ws.State, registry.TenantStateActive)
		}
		if ws.PlanVersion != "msp_starter" {
			t.Fatalf("workspace %s: PlanVersion = %q, want %q", ws.ID, ws.PlanVersion, "msp_starter")
		}

		mtp := config.NewMultiTenantPersistence(provisioner.tenantDataDir(ws.ID))
		org, err := mtp.LoadOrganizationStrict(ws.ID)
		if err != nil {
			t.Fatalf("workspace %s: LoadOrganizationStrict: %v", ws.ID, err)
		}
		if org.ID != ws.ID {
			t.Fatalf("workspace %s: org.ID = %q, want %q", ws.ID, org.ID, ws.ID)
		}
		if org.DisplayName != ws.DisplayName {
			t.Fatalf("workspace %s: org.DisplayName = %q, want %q", ws.ID, org.DisplayName, ws.DisplayName)
		}
		if org.OwnerUserID != "" {
			t.Fatalf("workspace %s: org.OwnerUserID = %q, want empty before members exist", ws.ID, org.OwnerUserID)
		}
		if len(org.Members) != 0 {
			t.Fatalf("workspace %s: expected no seeded org members before account members exist, got %+v", ws.ID, org.Members)
		}

		// Verify billing state was written.
		store := config.NewFileBillingStore(provisioner.tenantDataDir(ws.ID))
		bs, err := store.GetBillingState("default")
		if err != nil {
			t.Fatalf("workspace %s: GetBillingState: %v", ws.ID, err)
		}
		if bs == nil {
			t.Fatalf("workspace %s: billing state is nil", ws.ID)
		}
		if bs.PlanVersion != "msp_starter" {
			t.Fatalf("workspace %s: billing PlanVersion = %q, want %q", ws.ID, bs.PlanVersion, "msp_starter")
		}
		if bs.SubscriptionState != pkglicensing.SubStateActive {
			t.Fatalf("workspace %s: SubscriptionState = %q, want %q", ws.ID, bs.SubscriptionState, pkglicensing.SubStateActive)
		}
		if len(bs.Capabilities) == 0 {
			t.Fatalf("workspace %s: expected capabilities, got empty", ws.ID)
		}
		raw := loadRawBillingState(t, provisioner.tenantDataDir(ws.ID))
		if strings.TrimSpace(raw.EntitlementJWT) == "" || strings.TrimSpace(raw.EntitlementRefreshToken) == "" {
			t.Fatalf("workspace %s: expected raw hosted entitlement lease state, got %+v", ws.ID, raw)
		}
		if raw.SubscriptionState != "" || len(raw.Capabilities) != 0 {
			t.Fatalf("workspace %s: expected raw lease-only state, got %+v", ws.ID, raw)
		}
	}
	wantOwnership := map[string]bool{
		filepath.Join(tenantsDir, ws1.ID, "orgs"):                     true,
		filepath.Join(tenantsDir, ws1.ID, "orgs", ws1.ID):             true,
		filepath.Join(tenantsDir, ws1.ID, "orgs", ws1.ID, "org.json"): true,
		filepath.Join(tenantsDir, ws1.ID, "billing.json"):             true,
		filepath.Join(tenantsDir, ws1.ID, ".cloud_handoff_key"):       true,
		filepath.Join(tenantsDir, ws1.ID, "secrets", "handoff.key"):   true,
		filepath.Join(tenantsDir, ws2.ID, "orgs"):                     true,
		filepath.Join(tenantsDir, ws2.ID, "orgs", ws2.ID):             true,
		filepath.Join(tenantsDir, ws2.ID, "orgs", ws2.ID, "org.json"): true,
		filepath.Join(tenantsDir, ws2.ID, "billing.json"):             true,
		filepath.Join(tenantsDir, ws2.ID, ".cloud_handoff_key"):       true,
		filepath.Join(tenantsDir, ws2.ID, "secrets", "handoff.key"):   true,
	}
	for _, path := range chowned {
		delete(wantOwnership, path)
	}
	if len(wantOwnership) != 0 {
		t.Fatalf("missing hosted runtime ownership paths: %v", wantOwnership)
	}

	// Verify listing returns both workspaces.
	tenants, err := reg.ListByAccountID(accountID)
	if err != nil {
		t.Fatalf("ListByAccountID: %v", err)
	}
	if len(tenants) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(tenants))
	}

	// ── Phase 2b: Workspace creation via API handler ──────────────────

	tenantMux := newTenantMux(reg, provisioner)
	createBody := `{"display_name":"Client Three (API)"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/tenants", bytes.NewBufferString(createBody))
	createReq.Header.Set("X-Admin-Key", "secret-key")
	createRec := httptest.NewRecorder()
	tenantMux.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create workspace via API: status = %d, want %d (body=%q)", createRec.Code, http.StatusCreated, createRec.Body.String())
	}
	var ws3 registry.Tenant
	if err := json.Unmarshal(createRec.Body.Bytes(), &ws3); err != nil {
		t.Fatalf("decode created workspace: %v", err)
	}
	if ws3.AccountID != accountID {
		t.Fatalf("ws3.AccountID = %q, want %q", ws3.AccountID, accountID)
	}
	if ws3.DisplayName != "Client Three (API)" {
		t.Fatalf("ws3.DisplayName = %q, want %q", ws3.DisplayName, "Client Three (API)")
	}

	// Verify billing state for API-created workspace matches direct provisioning.
	{
		store := config.NewFileBillingStore(provisioner.tenantDataDir(ws3.ID))
		bs, err := store.GetBillingState("default")
		if err != nil {
			t.Fatalf("ws3: GetBillingState: %v", err)
		}
		if bs == nil {
			t.Fatalf("ws3: billing state is nil after API creation")
		}
		if bs.PlanVersion != "msp_starter" {
			t.Fatalf("ws3: billing PlanVersion = %q, want %q", bs.PlanVersion, "msp_starter")
		}
		if bs.SubscriptionState != pkglicensing.SubStateActive {
			t.Fatalf("ws3: SubscriptionState = %q, want %q", bs.SubscriptionState, pkglicensing.SubStateActive)
		}
		if len(bs.Capabilities) == 0 {
			t.Fatalf("ws3: expected capabilities after API creation, got empty")
		}
	}

	// ── Phase 3: Member management ─────────────────────────────────────

	// Create owner user.
	ownerID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: ownerID, Email: "owner@acmemsp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{
		AccountID: accountID,
		UserID:    ownerID,
		Role:      registry.MemberRoleOwner,
	}); err != nil {
		t.Fatal(err)
	}

	// Invite a tech member via the API handler.
	memberMux := newMemberMux(reg)
	inviteBody := `{"email":"tech@acmemsp.com","role":"tech"}`
	inviteReq := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/members", bytes.NewBufferString(inviteBody))
	inviteReq.Header.Set("X-Admin-Key", "secret-key")
	inviteRec := httptest.NewRecorder()
	memberMux.ServeHTTP(inviteRec, inviteReq)

	if inviteRec.Code != http.StatusCreated {
		t.Fatalf("invite member: status = %d, want %d (body=%q)", inviteRec.Code, http.StatusCreated, inviteRec.Body.String())
	}

	// Verify member list includes both users.
	listReq := httptest.NewRequest(http.MethodGet, "/api/accounts/"+accountID+"/members", nil)
	listReq.Header.Set("X-Admin-Key", "secret-key")
	listRec := httptest.NewRecorder()
	memberMux.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("list members: status = %d, want %d", listRec.Code, http.StatusOK)
	}

	var members []struct {
		Email string              `json:"email"`
		Role  registry.MemberRole `json:"role"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &members); err != nil {
		t.Fatalf("decode members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d (%+v)", len(members), members)
	}

	sort.Slice(members, func(i, j int) bool { return members[i].Email < members[j].Email })
	if members[0].Email != "owner@acmemsp.com" || members[0].Role != registry.MemberRoleOwner {
		t.Fatalf("member[0] = %+v, want owner@acmemsp.com/owner", members[0])
	}
	if members[1].Email != "tech@acmemsp.com" || members[1].Role != registry.MemberRoleTech {
		t.Fatalf("member[1] = %+v, want tech@acmemsp.com/tech", members[1])
	}

	// ── Phase 4: Portal dashboard ──────────────────────────────────────

	portalMux := newPortalMux(reg)
	dashReq := httptest.NewRequest(http.MethodGet, "/api/portal/dashboard?account_id="+accountID, nil)
	dashReq.Header.Set("X-Admin-Key", "secret-key")
	dashRec := httptest.NewRecorder()
	portalMux.ServeHTTP(dashRec, dashReq)

	if dashRec.Code != http.StatusOK {
		t.Fatalf("portal dashboard: status = %d, want %d (body=%q)", dashRec.Code, http.StatusOK, dashRec.Body.String())
	}

	var dash struct {
		Account struct {
			ID          string               `json:"id"`
			DisplayName string               `json:"display_name"`
			Kind        registry.AccountKind `json:"kind"`
		} `json:"account"`
		Workspaces []struct {
			ID          string               `json:"id"`
			DisplayName string               `json:"display_name"`
			State       registry.TenantState `json:"state"`
		} `json:"workspaces"`
		Summary struct {
			Total   int `json:"total"`
			Active  int `json:"active"`
			Healthy int `json:"healthy"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(dashRec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	if dash.Account.ID != accountID {
		t.Fatalf("dashboard account.id = %q, want %q", dash.Account.ID, accountID)
	}
	if dash.Account.Kind != registry.AccountKindMSP {
		t.Fatalf("dashboard account.kind = %q, want %q", dash.Account.Kind, registry.AccountKindMSP)
	}
	// Dashboard should list the 3 workspaces (2 direct + 1 via API).
	if len(dash.Workspaces) != 3 {
		t.Fatalf("dashboard workspaces = %d, want 3", len(dash.Workspaces))
	}
	if dash.Summary.Total != 3 {
		t.Fatalf("dashboard summary.total = %d, want 3", dash.Summary.Total)
	}
	if dash.Summary.Active != 3 {
		t.Fatalf("dashboard summary.active = %d, want 3", dash.Summary.Active)
	}

	// Verify each workspace ID/state appears in the dashboard response.
	dashWSByID := map[string]registry.TenantState{}
	for _, w := range dash.Workspaces {
		dashWSByID[w.ID] = w.State
	}
	for _, ws := range []*registry.Tenant{ws1, ws2, &ws3} {
		state, ok := dashWSByID[ws.ID]
		if !ok {
			t.Fatalf("dashboard missing workspace %s (%s)", ws.ID, ws.DisplayName)
		}
		if state != registry.TenantStateActive {
			t.Fatalf("dashboard workspace %s: state = %q, want %q", ws.ID, state, registry.TenantStateActive)
		}
	}

	// ── Phase 5: Billing state propagation (subscription update) ───────
	// (StripeAccount already created before Phase 2.)

	// Simulate subscription update to past_due (grace period).
	sub := Subscription{
		ID:       "sub_msp_lifecycle_test",
		Customer: "cus_msp_lifecycle_test",
		Status:   "past_due",
		Metadata: map[string]string{"plan_version": "msp_hosted_v1"},
	}
	if err := provisioner.HandleMSPSubscriptionUpdated(context.Background(), sub); err != nil {
		t.Fatalf("HandleMSPSubscriptionUpdated: %v", err)
	}

	// Verify account-level Stripe state was updated.
	sa, err := reg.GetStripeAccount(accountID)
	if err != nil {
		t.Fatalf("GetStripeAccount: %v", err)
	}
	if sa == nil {
		t.Fatal("expected StripeAccount to exist after subscription update")
	}
	if sa.StripeSubscriptionID != "sub_msp_lifecycle_test" {
		t.Fatalf("StripeAccount.StripeSubscriptionID = %q, want %q", sa.StripeSubscriptionID, "sub_msp_lifecycle_test")
	}
	if sa.PlanVersion != "msp_starter" {
		t.Fatalf("StripeAccount.PlanVersion = %q, want %q", sa.PlanVersion, "msp_starter")
	}
	// past_due → grace window should have been started.
	if sa.GraceStartedAt == nil || *sa.GraceStartedAt <= 0 {
		t.Fatalf("StripeAccount.GraceStartedAt = %v, want non-nil positive timestamp", sa.GraceStartedAt)
	}

	// Verify all tenants received updated billing state.
	for _, ws := range []*registry.Tenant{ws1, ws2, &ws3} {
		refreshed, err := reg.Get(ws.ID)
		if err != nil {
			t.Fatalf("Get(%s): %v", ws.ID, err)
		}
		if refreshed == nil {
			t.Fatalf("workspace %s missing after subscription update", ws.ID)
		}
		// past_due maps to SubStateGrace → tenants stay active.
		if refreshed.State != registry.TenantStateActive {
			t.Fatalf("workspace %s: State after past_due = %q, want %q", ws.ID, refreshed.State, registry.TenantStateActive)
		}

		store := config.NewFileBillingStore(provisioner.tenantDataDir(ws.ID))
		bs, err := store.GetBillingState("default")
		if err != nil {
			t.Fatalf("workspace %s: GetBillingState after update: %v", ws.ID, err)
		}
		if bs == nil {
			t.Fatalf("workspace %s: billing state nil after update", ws.ID)
		}
		if bs.SubscriptionState != pkglicensing.SubStateGrace {
			t.Fatalf("workspace %s: SubscriptionState = %q, want %q", ws.ID, bs.SubscriptionState, pkglicensing.SubStateGrace)
		}
		// Grace period should still grant capabilities.
		if len(bs.Capabilities) == 0 {
			t.Fatalf("workspace %s: expected capabilities during grace, got empty", ws.ID)
		}
		if bs.StripeCustomerID != "cus_msp_lifecycle_test" {
			t.Fatalf("workspace %s: StripeCustomerID = %q, want %q", ws.ID, bs.StripeCustomerID, "cus_msp_lifecycle_test")
		}
		if bs.StripeSubscriptionID != "sub_msp_lifecycle_test" {
			t.Fatalf("workspace %s: StripeSubscriptionID = %q, want %q", ws.ID, bs.StripeSubscriptionID, "sub_msp_lifecycle_test")
		}
		raw := loadRawBillingState(t, provisioner.tenantDataDir(ws.ID))
		if strings.TrimSpace(raw.EntitlementJWT) == "" || strings.TrimSpace(raw.EntitlementRefreshToken) == "" {
			t.Fatalf("workspace %s: expected raw entitlement lease during grace, got %+v", ws.ID, raw)
		}
	}

	// ── Phase 6: Subscription deletion revokes access ──────────────────

	delSub := Subscription{
		ID:       "sub_msp_lifecycle_test",
		Customer: "cus_msp_lifecycle_test",
		Status:   "canceled",
	}
	if err := provisioner.HandleMSPSubscriptionDeleted(context.Background(), delSub); err != nil {
		t.Fatalf("HandleMSPSubscriptionDeleted: %v", err)
	}

	// Verify account-level Stripe state after deletion.
	saAfterDel, err := reg.GetStripeAccount(accountID)
	if err != nil {
		t.Fatalf("GetStripeAccount after deletion: %v", err)
	}
	if saAfterDel == nil {
		t.Fatal("expected StripeAccount to exist after deletion")
	}
	if saAfterDel.SubscriptionState != "canceled" {
		t.Fatalf("StripeAccount.SubscriptionState after deletion = %q, want %q", saAfterDel.SubscriptionState, "canceled")
	}
	if saAfterDel.GraceStartedAt != nil {
		t.Fatalf("StripeAccount.GraceStartedAt after deletion = %v, want nil", saAfterDel.GraceStartedAt)
	}

	for _, ws := range []*registry.Tenant{ws1, ws2, &ws3} {
		refreshed, err := reg.Get(ws.ID)
		if err != nil {
			t.Fatalf("Get(%s): %v", ws.ID, err)
		}
		if refreshed == nil {
			t.Fatalf("workspace %s missing after subscription deletion", ws.ID)
		}
		if refreshed.State != registry.TenantStateCanceled {
			t.Fatalf("workspace %s: State after deletion = %q, want %q", ws.ID, refreshed.State, registry.TenantStateCanceled)
		}

		store := config.NewFileBillingStore(provisioner.tenantDataDir(ws.ID))
		bs, err := store.GetBillingState("default")
		if err != nil {
			t.Fatalf("workspace %s: GetBillingState after deletion: %v", ws.ID, err)
		}
		if bs == nil {
			t.Fatalf("workspace %s: billing state nil after deletion", ws.ID)
		}
		if bs.SubscriptionState != pkglicensing.SubStateCanceled {
			t.Fatalf("workspace %s: SubscriptionState = %q, want %q", ws.ID, bs.SubscriptionState, pkglicensing.SubStateCanceled)
		}
		if len(bs.Capabilities) != 0 {
			t.Fatalf("workspace %s: expected empty capabilities after cancellation, got %v", ws.ID, bs.Capabilities)
		}
		raw := loadRawBillingState(t, provisioner.tenantDataDir(ws.ID))
		if raw.EntitlementJWT != "" || raw.EntitlementRefreshToken != "" {
			t.Fatalf("workspace %s: expected canceled raw state without lease tokens, got %+v", ws.ID, raw)
		}
	}

	// ── Phase 7: Portal reflects canceled state ────────────────────────

	dashReq2 := httptest.NewRequest(http.MethodGet, "/api/portal/dashboard?account_id="+accountID, nil)
	dashReq2.Header.Set("X-Admin-Key", "secret-key")
	dashRec2 := httptest.NewRecorder()
	portalMux.ServeHTTP(dashRec2, dashReq2)

	if dashRec2.Code != http.StatusOK {
		t.Fatalf("portal dashboard after cancel: status = %d, want %d", dashRec2.Code, http.StatusOK)
	}

	var dash2 struct {
		Workspaces []struct {
			ID    string               `json:"id"`
			State registry.TenantState `json:"state"`
		} `json:"workspaces"`
		Summary struct {
			Total     int `json:"total"`
			Active    int `json:"active"`
			Suspended int `json:"suspended"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(dashRec2.Body.Bytes(), &dash2); err != nil {
		t.Fatalf("decode dashboard2: %v", err)
	}
	if dash2.Summary.Total != 3 {
		t.Fatalf("dashboard after cancel: total = %d, want 3", dash2.Summary.Total)
	}
	if dash2.Summary.Active != 0 {
		t.Fatalf("dashboard after cancel: active = %d, want 0", dash2.Summary.Active)
	}

	// Verify each workspace shows canceled state in portal.
	for _, w := range dash2.Workspaces {
		if w.State != registry.TenantStateCanceled {
			t.Fatalf("dashboard after cancel: workspace %s state = %q, want %q", w.ID, w.State, registry.TenantStateCanceled)
		}
	}

	// ── Phase 8: Workspace deletion ────────────────────────────────────

	// Reuse tenantMux from Phase 2b.
	delReq := httptest.NewRequest(http.MethodDelete, "/api/accounts/"+accountID+"/tenants/"+ws1.ID, nil)
	delReq.Header.Set("X-Admin-Key", "secret-key")
	delRec := httptest.NewRecorder()
	tenantMux.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete workspace: status = %d, want %d (body=%q)", delRec.Code, http.StatusNoContent, delRec.Body.String())
	}

	deleted, err := reg.Get(ws1.ID)
	if err != nil {
		t.Fatalf("Get(%s) after delete: %v", ws1.ID, err)
	}
	if deleted == nil {
		t.Fatalf("expected soft-deleted tenant to exist")
	}
	if deleted.State != registry.TenantStateDeleted {
		t.Fatalf("workspace %s: State after delete = %q, want %q", ws1.ID, deleted.State, registry.TenantStateDeleted)
	}
}

// TestMSPLifecycle_TenantIsolation verifies that workspaces from one MSP
// account cannot be accessed or modified through another MSP's account context.
func TestMSPLifecycle_TenantIsolation(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	provisioner := newTestProvisioner(t, reg, tenantsDir, nil, true)

	// Create two separate MSP accounts.
	msp1ID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	msp2ID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: msp1ID, Kind: registry.AccountKindMSP, DisplayName: "MSP One"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: msp2ID, Kind: registry.AccountKindMSP, DisplayName: "MSP Two"}); err != nil {
		t.Fatal(err)
	}

	// Provision a workspace under MSP One.
	ws, err := provisioner.ProvisionWorkspace(context.Background(), msp1ID, "MSP One Client")
	if err != nil {
		t.Fatalf("ProvisionWorkspace: %v", err)
	}

	// Attempt to update MSP One's workspace via MSP Two's account — should fail.
	tenantMux := newTenantMux(reg, provisioner)
	updateBody := `{"display_name":"Hijacked"}`
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/accounts/"+msp2ID+"/tenants/"+ws.ID, bytes.NewBufferString(updateBody))
	updateReq.Header.Set("X-Admin-Key", "secret-key")
	updateRec := httptest.NewRecorder()
	tenantMux.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusNotFound && updateRec.Code != http.StatusForbidden {
		t.Fatalf("cross-account update: status = %d, want 404 or 403", updateRec.Code)
	}

	// Attempt to delete MSP One's workspace via MSP Two's account — should fail.
	delReq := httptest.NewRequest(http.MethodDelete, "/api/accounts/"+msp2ID+"/tenants/"+ws.ID, nil)
	delReq.Header.Set("X-Admin-Key", "secret-key")
	delRec := httptest.NewRecorder()
	tenantMux.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusNotFound && delRec.Code != http.StatusForbidden {
		t.Fatalf("cross-account delete: status = %d, want 404 or 403", delRec.Code)
	}

	// Verify the workspace is untouched.
	original, err := reg.Get(ws.ID)
	if err != nil {
		t.Fatalf("Get(%s): %v", ws.ID, err)
	}
	if original.DisplayName != "MSP One Client" {
		t.Fatalf("workspace display name changed to %q", original.DisplayName)
	}
	if original.State != registry.TenantStateActive {
		t.Fatalf("workspace state = %q, want %q", original.State, registry.TenantStateActive)
	}

	// Attempt to read MSP One's workspace detail via MSP Two's account — should fail.
	portalMux := newPortalMux(reg)
	detailReq := httptest.NewRequest(http.MethodGet, "/api/portal/workspaces/"+ws.ID+"?account_id="+msp2ID, nil)
	detailReq.Header.Set("X-Admin-Key", "secret-key")
	detailRec := httptest.NewRecorder()
	portalMux.ServeHTTP(detailRec, detailReq)

	if detailRec.Code != http.StatusNotFound && detailRec.Code != http.StatusForbidden {
		t.Fatalf("cross-account workspace detail: status = %d, want 404 or 403 (body=%q)", detailRec.Code, detailRec.Body.String())
	}

	// MSP Two's portal should show 0 workspaces.
	dashReq := httptest.NewRequest(http.MethodGet, "/api/portal/dashboard?account_id="+msp2ID, nil)
	dashReq.Header.Set("X-Admin-Key", "secret-key")
	dashRec := httptest.NewRecorder()
	portalMux.ServeHTTP(dashRec, dashReq)

	if dashRec.Code != http.StatusOK {
		t.Fatalf("MSP Two dashboard: status = %d, want %d", dashRec.Code, http.StatusOK)
	}

	var dash struct {
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(dashRec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	if dash.Summary.Total != 0 {
		t.Fatalf("MSP Two: summary.total = %d, want 0", dash.Summary.Total)
	}
}

// TestMSPLifecycle_PlanVersionFromAccount verifies that ProvisionWorkspace
// uses the account's actual Stripe plan version instead of a hardcoded default.
// MSP Growth accounts should get msp_growth limits (150 agents), not msp_hosted_v1 (50).
func TestMSPLifecycle_PlanVersionFromAccount(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	provisioner := newTestProvisioner(t, reg, tenantsDir, nil, true)

	tests := []struct {
		name        string
		planVersion string
		wantAgents  int64
	}{
		{"msp_starter", "msp_starter", 50},
		{"msp_growth", "msp_growth", 150},
		{"msp_scale", "msp_scale", 400},
		{"legacy_msp_hosted_v1", "msp_hosted_v1", 50},
		{"canonicalized_cloud_alias", "cloud_v1", 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			accountID, err := registry.GenerateAccountID()
			if err != nil {
				t.Fatal(err)
			}
			if err := reg.CreateAccount(&registry.Account{
				ID:          accountID,
				Kind:        registry.AccountKindMSP,
				DisplayName: "MSP " + tc.name,
			}); err != nil {
				t.Fatalf("CreateAccount: %v", err)
			}
			if err := reg.CreateStripeAccount(&registry.StripeAccount{
				AccountID:        accountID,
				StripeCustomerID: "cus_plan_test_" + tc.name,
				PlanVersion:      tc.planVersion,
			}); err != nil {
				t.Fatalf("CreateStripeAccount: %v", err)
			}

			ws, err := provisioner.ProvisionWorkspace(context.Background(), accountID, "Client")
			if err != nil {
				t.Fatalf("ProvisionWorkspace: %v", err)
			}
			wantPlanVersion := pkglicensing.CanonicalizePlanVersion(tc.planVersion)
			if ws.PlanVersion != wantPlanVersion {
				t.Fatalf("workspace.PlanVersion = %q, want %q", ws.PlanVersion, wantPlanVersion)
			}

			store := config.NewFileBillingStore(provisioner.tenantDataDir(ws.ID))
			bs, err := store.GetBillingState("default")
			if err != nil {
				t.Fatalf("GetBillingState: %v", err)
			}
			if bs == nil {
				t.Fatal("billing state is nil")
			}
			if bs.PlanVersion != wantPlanVersion {
				t.Fatalf("billing.PlanVersion = %q, want %q", bs.PlanVersion, wantPlanVersion)
			}
			if bs.Limits["max_agents"] != tc.wantAgents {
				t.Fatalf("billing.Limits[max_agents] = %d, want %d", bs.Limits["max_agents"], tc.wantAgents)
			}
		})
	}
}

// TestMSPLifecycle_PlanVersionFallback verifies that ProvisionWorkspace falls
// back to msp_starter when no StripeAccount exists for the account.
func TestMSPLifecycle_PlanVersionFallback(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	provisioner := newTestProvisioner(t, reg, tenantsDir, nil, true)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{
		ID:          accountID,
		Kind:        registry.AccountKindMSP,
		DisplayName: "No Billing MSP",
	}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	// No StripeAccount created — should fall back to msp_starter.
	ws, err := provisioner.ProvisionWorkspace(context.Background(), accountID, "Client")
	if err != nil {
		t.Fatalf("ProvisionWorkspace: %v", err)
	}
	if ws.PlanVersion != "msp_starter" {
		t.Fatalf("workspace.PlanVersion = %q, want %q (fallback)", ws.PlanVersion, "msp_starter")
	}

	store := config.NewFileBillingStore(provisioner.tenantDataDir(ws.ID))
	bs, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if bs == nil {
		t.Fatal("billing state is nil")
	}
	if bs.PlanVersion != "msp_starter" {
		t.Fatalf("billing.PlanVersion = %q, want %q", bs.PlanVersion, "msp_starter")
	}
	if bs.Limits["max_agents"] != 50 {
		t.Fatalf("billing.Limits[max_agents] = %d, want 50 (msp_starter default)", bs.Limits["max_agents"])
	}
}

// ─── Test helpers ──────────────────────────────────────────────────────────

func newMemberMux(reg *registry.TenantRegistry) *http.ServeMux {
	mux := http.NewServeMux()
	listMembers := account.HandleListMembers(reg)
	inviteMember := account.HandleInviteMember(reg)
	updateRole := account.HandleUpdateMemberRole(reg)
	removeMember := account.HandleRemoveMember(reg)

	collection := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listMembers(w, r)
		case http.MethodPost:
			inviteMember(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	member := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			updateRole(w, r)
		case http.MethodDelete:
			removeMember(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.Handle("/api/accounts/{account_id}/members", admin.AdminKeyMiddleware("secret-key", collection))
	mux.Handle("/api/accounts/{account_id}/members/{user_id}", admin.AdminKeyMiddleware("secret-key", member))
	return mux
}

func newPortalMux(reg *registry.TenantRegistry) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/api/portal/dashboard", admin.AdminKeyMiddleware("secret-key", portal.HandlePortalDashboard(reg)))
	mux.Handle("/api/portal/workspaces/{tenant_id}", admin.AdminKeyMiddleware("secret-key", portal.HandlePortalWorkspaceDetail(reg)))
	return mux
}

func newTenantMux(reg *registry.TenantRegistry, provisioner *Provisioner) *http.ServeMux {
	mux := http.NewServeMux()
	listTenants := account.HandleListTenants(reg)
	createTenant := account.HandleCreateTenant(reg, provisioner)
	updateTenant := account.HandleUpdateTenant(reg)
	deleteTenant := account.HandleDeleteTenant(reg, provisioner)

	collection := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listTenants(w, r)
		case http.MethodPost:
			createTenant(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	tenantItem := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			updateTenant(w, r)
		case http.MethodDelete:
			deleteTenant(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.Handle("/api/accounts/{account_id}/tenants", admin.AdminKeyMiddleware("secret-key", collection))
	mux.Handle("/api/accounts/{account_id}/tenants/{tenant_id}", admin.AdminKeyMiddleware("secret-key", tenantItem))
	return mux
}

func loadRawBillingState(t *testing.T, tenantDataDir string) pkglicensing.BillingState {
	t.Helper()
	rawPath := filepath.Join(tenantDataDir, "billing.json")
	data, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", rawPath, err)
	}
	var state pkglicensing.BillingState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal(%s): %v", rawPath, err)
	}
	return state
}
