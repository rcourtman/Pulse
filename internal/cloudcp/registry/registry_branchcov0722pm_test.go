package registry

import (
	"strings"
	"testing"
	"time"
)

func newCovAccount(t *testing.T, reg *TenantRegistry) string {
	t.Helper()
	id, err := GenerateAccountID()
	if err != nil {
		t.Fatalf("GenerateAccountID: %v", err)
	}
	if err := reg.CreateAccount(&Account{ID: id, Kind: AccountKindIndividual, DisplayName: "cov account"}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	return id
}

func TestBranchcov0722PM_WorkspaceLimitExceededErrorError(t *testing.T) {
	t.Run("nil receiver falls back to generic message", func(t *testing.T) {
		var nilErr *WorkspaceLimitExceededError
		got := nilErr.Error()
		const want = "workspace limit exceeded"
		if got != want {
			t.Fatalf("nil receiver Error() = %q, want %q", got, want)
		}
	})

	t.Run("populated message embeds account current and limit", func(t *testing.T) {
		err := &WorkspaceLimitExceededError{AccountID: "a_cov123456", Current: 7, Limit: 3}
		got := err.Error()
		for _, want := range []string{`"a_cov123456"`, "7", "3"} {
			if !strings.Contains(got, want) {
				t.Errorf("Error() = %q, missing substring %q", got, want)
			}
		}
	})
}

func TestBranchcov0722PM_CountActiveByAccountID(t *testing.T) {
	reg := newTestRegistry(t)

	const targetAccount = "a_count_tgt1"
	const otherAccount = "a_count_other"

	got, err := reg.CountActiveByAccountID(targetAccount)
	if err != nil {
		t.Fatalf("CountActiveByAccountID on empty account: %v", err)
	}
	if got != 0 {
		t.Fatalf("empty account count = %d, want 0", got)
	}

	countedStates := []struct {
		id    string
		state TenantState
	}{
		{"t-CNTACTV01", TenantStateActive},
		{"t-CNTPRVN01", TenantStateProvisioning},
		{"t-CNTSPN001", TenantStateSuspended},
		{"t-CNTFLD001", TenantStateFailed},
	}
	excludedStates := []struct {
		id    string
		state TenantState
	}{
		{"t-CNTDLT001", TenantStateDeleted},
		{"t-CNTCNC001", TenantStateCanceled},
		{"t-CNTDLG001", TenantStateDeleting},
	}

	for _, s := range countedStates {
		if err := reg.Create(&Tenant{ID: s.id, AccountID: targetAccount, State: s.state}); err != nil {
			t.Fatalf("Create %s (%s): %v", s.id, s.state, err)
		}
	}
	for _, s := range excludedStates {
		if err := reg.Create(&Tenant{ID: s.id, AccountID: targetAccount, State: s.state}); err != nil {
			t.Fatalf("Create %s (%s): %v", s.id, s.state, err)
		}
	}
	if err := reg.Create(&Tenant{ID: "t-CNTOTH001", AccountID: otherAccount, State: TenantStateActive}); err != nil {
		t.Fatalf("Create other-account tenant: %v", err)
	}

	got, err = reg.CountActiveByAccountID(targetAccount)
	if err != nil {
		t.Fatalf("CountActiveByAccountID after inserts: %v", err)
	}
	if want := len(countedStates); got != want {
		t.Fatalf("CountActiveByAccountID = %d, want %d (excluded states and other accounts must not count)", got, want)
	}

	gotOther, err := reg.CountActiveByAccountID(otherAccount)
	if err != nil {
		t.Fatalf("CountActiveByAccountID otherAccount: %v", err)
	}
	if gotOther != 1 {
		t.Fatalf("CountActiveByAccountID otherAccount = %d, want 1", gotOther)
	}
}

func TestBranchcov0722PM_GetTenantForAccount(t *testing.T) {
	reg := newTestRegistry(t)

	const ownerAccount = "a_owner_get1"
	const otherAccount = "a_other_get1"
	const tenantID = "t-GETOWN001"
	const foreignTenantID = "t-GETOTH001"

	if err := reg.Create(&Tenant{ID: tenantID, AccountID: ownerAccount, State: TenantStateActive, DisplayName: "Owned"}); err != nil {
		t.Fatalf("Create owned tenant: %v", err)
	}
	if err := reg.Create(&Tenant{ID: foreignTenantID, AccountID: otherAccount, State: TenantStateActive, DisplayName: "Foreign"}); err != nil {
		t.Fatalf("Create foreign tenant: %v", err)
	}

	t.Run("found for owning account", func(t *testing.T) {
		got, err := reg.GetTenantForAccount(ownerAccount, tenantID)
		if err != nil {
			t.Fatalf("GetTenantForAccount: %v", err)
		}
		if got == nil {
			t.Fatal("expected tenant, got nil")
		}
		if got.ID != tenantID {
			t.Fatalf("got.ID = %q, want %q", got.ID, tenantID)
		}
		if got.AccountID != ownerAccount {
			t.Fatalf("got.AccountID = %q, want %q", got.AccountID, ownerAccount)
		}
	})

	t.Run("exists but belongs to a different account returns nil nil", func(t *testing.T) {
		got, err := reg.GetTenantForAccount(ownerAccount, foreignTenantID)
		if err != nil {
			t.Fatalf("GetTenantForAccount foreign: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil tenant for foreign-owned workspace, got %+v", got)
		}
	})

	t.Run("missing tenant returns nil nil", func(t *testing.T) {
		got, err := reg.GetTenantForAccount(ownerAccount, "t-GETMS001")
		if err != nil {
			t.Fatalf("GetTenantForAccount missing: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil tenant for missing id, got %+v", got)
		}
	})
}

func TestBranchcov0722PM_ListInvitationsByEmail(t *testing.T) {
	reg := newTestRegistry(t)

	account1 := newCovAccount(t, reg)
	account2 := newCovAccount(t, reg)
	account3 := newCovAccount(t, reg)
	mixedAccount := newCovAccount(t, reg)

	for _, acc := range []string{account1, account2} {
		if err := reg.UpsertInvitation(&AccountInvitation{
			AccountID: acc,
			Email:     "shared@example.com",
			Role:      MemberRoleTech,
		}); err != nil {
			t.Fatalf("UpsertInvitation shared for %s: %v", acc, err)
		}
	}
	if err := reg.UpsertInvitation(&AccountInvitation{
		AccountID: account3,
		Email:     "solo@example.com",
		Role:      MemberRoleAdmin,
	}); err != nil {
		t.Fatalf("UpsertInvitation solo: %v", err)
	}

	if _, err := reg.db.Exec(
		`INSERT INTO account_invitations (id, account_id, email, role, invited_by, invited_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"ainv_RAWLWR01", mixedAccount, "rawinsert@example.com", "tech", "", time.Now().UTC().Unix(),
	); err != nil {
		t.Fatalf("raw insert lowercase invitation: %v", err)
	}

	t.Run("no matches returns empty", func(t *testing.T) {
		got, err := reg.ListInvitationsByEmail("nobody@example.com")
		if err != nil {
			t.Fatalf("ListInvitationsByEmail no-match: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected 0 invitations, got %d (%+v)", len(got), got)
		}
	})

	t.Run("single match returns the one invitation", func(t *testing.T) {
		got, err := reg.ListInvitationsByEmail("solo@example.com")
		if err != nil {
			t.Fatalf("ListInvitationsByEmail solo: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 invitation, got %d", len(got))
		}
		if got[0].AccountID != account3 {
			t.Fatalf("got[0].AccountID = %q, want %q", got[0].AccountID, account3)
		}
		if got[0].Email != "solo@example.com" {
			t.Fatalf("got[0].Email = %q, want %q", got[0].Email, "solo@example.com")
		}
		if got[0].Role != MemberRoleAdmin {
			t.Fatalf("got[0].Role = %q, want %q", got[0].Role, MemberRoleAdmin)
		}
	})

	t.Run("several matches across accounts", func(t *testing.T) {
		got, err := reg.ListInvitationsByEmail("shared@example.com")
		if err != nil {
			t.Fatalf("ListInvitationsByEmail shared: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 invitations, got %d", len(got))
		}
		seen := map[string]bool{}
		for _, inv := range got {
			seen[inv.AccountID] = true
			if inv.Email != "shared@example.com" {
				t.Errorf("inv.Email = %q, want %q", inv.Email, "shared@example.com")
			}
		}
		if !seen[account1] || !seen[account2] {
			t.Fatalf("expected both accounts in results, seen=%v", seen)
		}
	})

	t.Run("query normalizes case and surrounding whitespace", func(t *testing.T) {
		got, err := reg.ListInvitationsByEmail("  RAWINSERT@EXAMPLE.COM  ")
		if err != nil {
			t.Fatalf("ListInvitationsByEmail normalized: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 invitation after normalization, got %d", len(got))
		}
		if got[0].ID != "ainv_RAWLWR01" {
			t.Fatalf("got[0].ID = %q, want %q", got[0].ID, "ainv_RAWLWR01")
		}
	})
}

func TestBranchcov0722PM_DeleteInvitationByAccountAndEmail(t *testing.T) {
	reg := newTestRegistry(t)

	account := newCovAccount(t, reg)
	const email = "deletable@example.com"

	if err := reg.UpsertInvitation(&AccountInvitation{
		AccountID: account,
		Email:     email,
		Role:      MemberRoleTech,
	}); err != nil {
		t.Fatalf("UpsertInvitation: %v", err)
	}

	before, err := reg.ListInvitationsByEmail(email)
	if err != nil {
		t.Fatalf("ListInvitationsByEmail before delete: %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("expected 1 invitation before delete, got %d", len(before))
	}

	t.Run("deletes existing invitation then it is gone via list", func(t *testing.T) {
		if err := reg.DeleteInvitationByAccountAndEmail(account, email); err != nil {
			t.Fatalf("DeleteInvitationByAccountAndEmail: %v", err)
		}
		after, err := reg.ListInvitationsByEmail(email)
		if err != nil {
			t.Fatalf("ListInvitationsByEmail after delete: %v", err)
		}
		if len(after) != 0 {
			t.Fatalf("expected 0 invitations after delete, got %d (%+v)", len(after), after)
		}
	})

	t.Run("deleting a non-existent pair returns nil", func(t *testing.T) {
		beforeNoOp, err := reg.ListInvitationsByEmail(email)
		if err != nil {
			t.Fatalf("ListInvitationsByEmail before no-op: %v", err)
		}
		if err := reg.DeleteInvitationByAccountAndEmail(account, "never@existed.com"); err != nil {
			t.Fatalf("DeleteInvitationByAccountAndEmail non-existent = %v, want nil", err)
		}
		got, err := reg.ListInvitationsByEmail(email)
		if err != nil {
			t.Fatalf("ListInvitationsByEmail after no-op: %v", err)
		}
		if len(got) != len(beforeNoOp) {
			t.Fatalf("no-op delete changed invitation count: before %d, after %d", len(beforeNoOp), len(got))
		}
	})
}
