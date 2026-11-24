package stripe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/account"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A workspace authorizes against its own org.json, so removing or demoting an
// account member has to be written there too, not only to the registry.
func TestAccountMemberRemovalAndDemotionReachWorkspaces(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	p := newTestProvisioner(t, reg, tenantsDir, nil, true)
	var chowned []string
	p.chownFile = func(path string, _, _ int) error {
		chowned = append(chowned, path)
		return nil
	}

	const accountID, otherAccountID = "a-access", "a-other"
	for _, id := range []string{accountID, otherAccountID} {
		if err := reg.CreateAccount(&registry.Account{ID: id, Kind: registry.AccountKindMSP}); err != nil {
			t.Fatal(err)
		}
		seedWorkspaceBilling(t, reg, id)
	}
	users := map[string]registry.MemberRole{
		"u_owner": registry.MemberRoleOwner,
		"u_admin": registry.MemberRoleAdmin,
		"u_tech":  registry.MemberRoleTech,
	}
	for userID, role := range users {
		email := strings.TrimPrefix(userID, "u_") + "@msp.example"
		if err := reg.CreateUser(&registry.User{ID: userID, Email: email}); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{accountID, otherAccountID} {
			if err := reg.CreateMembership(&registry.AccountMembership{AccountID: id, UserID: userID, Role: role}); err != nil {
				t.Fatal(err)
			}
		}
	}

	ctx := context.Background()
	// The admin created this workspace and so owns it.
	adminOwned, err := p.ProvisionWorkspaceForOwner(ctx, accountID, "Admin's client", "admin@msp.example")
	if err != nil {
		t.Fatalf("ProvisionWorkspaceForOwner: %v", err)
	}
	shared, err := p.ProvisionWorkspace(ctx, accountID, "Client")
	if err != nil {
		t.Fatalf("ProvisionWorkspace: %v", err)
	}
	other, err := p.ProvisionWorkspace(ctx, otherAccountID, "Other account's client")
	if err != nil {
		t.Fatalf("ProvisionWorkspace: %v", err)
	}

	loadOrg := func(tenantID string) *models.Organization {
		t.Helper()
		org, err := config.NewMultiTenantPersistence(filepath.Join(tenantsDir, tenantID)).LoadOrganizationStrict(tenantID)
		if err != nil {
			t.Fatalf("LoadOrganizationStrict(%s): %v", tenantID, err)
		}
		return org
	}
	sharedOrg := loadOrg(shared.ID)
	sharedOrg.PendingInvitations = append(sharedOrg.PendingInvitations, models.OrganizationInvitation{
		UserID: "u_tech", Role: models.OrgRoleAdmin, InvitedAt: time.Now().UTC(), InvitedBy: "u_owner",
	})
	if err := config.NewMultiTenantPersistence(filepath.Join(tenantsDir, shared.ID)).SaveOrganization(sharedOrg); err != nil {
		t.Fatalf("SaveOrganization: %v", err)
	}
	for _, tenantID := range []string{adminOwned.ID, shared.ID, other.ID} {
		org := loadOrg(tenantID)
		if got := org.GetMemberRoleByUserID("u_admin"); !models.OrganizationRoleAtLeast(got, models.OrgRoleAdmin) {
			t.Fatalf("seeded role of u_admin in %s = %q", tenantID, got)
		}
		if got := org.GetMemberRoleByUserID("u_tech"); got != models.OrgRoleEditor {
			t.Fatalf("seeded role of u_tech in %s = %q", tenantID, got)
		}
	}
	if got := loadOrg(adminOwned.ID).OwnerUserID; got != "u_admin" {
		t.Fatalf("seeded owner = %q, want u_admin", got)
	}
	chowned = nil

	mux := http.NewServeMux()
	mux.Handle("PATCH /api/accounts/{account_id}/members/{user_id}", account.HandleUpdateMemberRole(reg, p))
	mux.Handle("DELETE /api/accounts/{account_id}/members/{user_id}", account.HandleRemoveMember(reg, p))
	do := func(method, userID, body string, want int) {
		t.Helper()
		req := httptest.NewRequest(method, "/api/accounts/"+accountID+"/members/"+userID, strings.NewReader(body))
		req.Header.Set("X-User-Role", string(registry.MemberRoleOwner))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: status = %d, want %d (body=%q)", method, userID, rec.Code, want, rec.Body.String())
		}
	}

	do(http.MethodPatch, "u_admin", `{"role":"read_only"}`, http.StatusOK)
	for _, tenantID := range []string{adminOwned.ID, shared.ID} {
		if got := loadOrg(tenantID).GetMemberRoleByUserID("u_admin"); got != models.OrgRoleViewer {
			t.Fatalf("role of demoted u_admin in %s = %q, want %q", tenantID, got, models.OrgRoleViewer)
		}
	}
	org := loadOrg(adminOwned.ID)
	if org.OwnerUserID != "u_owner" || org.OwnerEmail != "owner@msp.example" || org.GetMemberRoleByUserID("u_owner") != models.OrgRoleOwner {
		t.Fatalf("workspace owned by the demoted member passed to %q (%q), owner role %q", org.OwnerUserID, org.OwnerEmail, org.GetMemberRoleByUserID("u_owner"))
	}
	if len(chowned) != 2 {
		t.Fatalf("rewritten org.json files given runtime ownership = %v, want both workspaces", chowned)
	}

	// Raising an account role restores the corresponding workspace role floor
	// across the account's existing workspaces.
	do(http.MethodPatch, "u_admin", `{"role":"tech"}`, http.StatusOK)
	for _, tenantID := range []string{adminOwned.ID, shared.ID} {
		if got := loadOrg(tenantID).GetMemberRoleByUserID("u_admin"); got != models.OrgRoleEditor {
			t.Fatalf("role of re-promoted u_admin in %s = %q, want %q", tenantID, got, models.OrgRoleEditor)
		}
	}

	do(http.MethodDelete, "u_tech", "", http.StatusNoContent)
	for _, tenantID := range []string{adminOwned.ID, shared.ID} {
		org := loadOrg(tenantID)
		if org.CanUserAccess("u_tech") || org.CanUserAccess("tech@msp.example") {
			t.Fatalf("removed member still has access to %s: %+v", tenantID, org.Members)
		}
		if !org.CanUserIDAccess("u_owner") || !org.CanUserIDAccess("u_admin") {
			t.Fatalf("removal of u_tech dropped other members of %s: %+v", tenantID, org.Members)
		}
	}
	if invitations := loadOrg(shared.ID).PendingInvitations; len(invitations) != 0 {
		t.Fatalf("removed member keeps a pending invitation: %+v", invitations)
	}

	// Membership of another account is a separate grant.
	org = loadOrg(other.ID)
	if org.GetMemberRoleByUserID("u_tech") != models.OrgRoleEditor || org.GetMemberRoleByUserID("u_admin") != models.OrgRoleAdmin {
		t.Fatalf("another account's workspace changed: %+v", org.Members)
	}
}

// Legacy organization records are keyed by email; the handoff still admits
// them, so a removal has to find them as well.
func TestRevokeWorkspaceAccessRemovesLegacyEmailKeyedMember(t *testing.T) {
	reg := newStripeTestRegistry(t)
	tenantsDir := t.TempDir()
	p := newTestProvisioner(t, reg, tenantsDir, nil, true)

	const accountID = "a-legacy"
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP}); err != nil {
		t.Fatal(err)
	}
	seedWorkspaceBilling(t, reg, accountID)
	for userID, role := range map[string]registry.MemberRole{"u_owner": registry.MemberRoleOwner, "u_tech": registry.MemberRoleTech} {
		if err := reg.CreateUser(&registry.User{ID: userID, Email: strings.TrimPrefix(userID, "u_") + "@msp.example"}); err != nil {
			t.Fatal(err)
		}
		if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: userID, Role: role}); err != nil {
			t.Fatal(err)
		}
	}
	tenant, err := p.ProvisionWorkspace(context.Background(), accountID, "Client")
	if err != nil {
		t.Fatalf("ProvisionWorkspace: %v", err)
	}
	mtp := config.NewMultiTenantPersistence(filepath.Join(tenantsDir, tenant.ID))
	org, err := mtp.LoadOrganizationStrict(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range org.Members {
		if org.Members[i].UserID == "u_tech" {
			org.Members[i].UserID, org.Members[i].Email = "Tech@MSP.example", ""
		}
	}
	if err := mtp.SaveOrganization(org); err != nil {
		t.Fatal(err)
	}

	if err := p.RevokeWorkspaceAccess(context.Background(), accountID, "u_tech"); err != nil {
		t.Fatalf("RevokeWorkspaceAccess: %v", err)
	}
	org, err = mtp.LoadOrganizationStrict(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := org.GetMemberRoleForPrincipal("u_tech", "tech@msp.example"); got != "" {
		t.Fatalf("removed member is still admitted with role %q: %+v", got, org.Members)
	}
	if len(org.Members) != 1 || org.Members[0].UserID != "u_owner" {
		t.Fatalf("members after removal = %+v, want only u_owner", org.Members)
	}

	// Re-adding the removed member restores their membership in the workspace.
	if err := p.SyncWorkspaceAccess(context.Background(), accountID, "u_tech", registry.MemberRoleAdmin); err != nil {
		t.Fatalf("SyncWorkspaceAccess: %v", err)
	}
	org, err = mtp.LoadOrganizationStrict(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := org.GetMemberRoleByUserID("u_tech"); got != models.OrgRoleAdmin {
		t.Fatalf("re-synced member role = %q, want %q", got, models.OrgRoleAdmin)
	}
}
