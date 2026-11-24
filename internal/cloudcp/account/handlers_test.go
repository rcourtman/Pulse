package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/admin"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

func newTestRegistry(t *testing.T) *registry.TenantRegistry {
	t.Helper()
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	return reg
}

func newTestMux(reg *registry.TenantRegistry) *http.ServeMux {
	mux := http.NewServeMux()

	listMembers := HandleListMembers(reg)
	inviteMember := HandleInviteMember(reg)
	updateRole := HandleUpdateMemberRole(reg)
	removeMember := HandleRemoveMember(reg)

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

func doRequest(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	req.Header.Set("X-Admin-Key", "secret-key")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func doRawRequest(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestInviteMember(t *testing.T) {
	reg := newTestRegistry(t)
	mux := newTestMux(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}

	body := `{"email":"tech@msp.com","role":"tech"}`
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/members", bytes.NewBufferString(body))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	u, err := reg.GetUserByEmail("tech@msp.com")
	if err != nil {
		t.Fatal(err)
	}
	if u != nil {
		t.Fatalf("expected invitation-only flow without user creation, got %+v", u)
	}

	invitation, err := reg.GetInvitationByAccountAndEmail(accountID, "tech@msp.com")
	if err != nil {
		t.Fatal(err)
	}
	if invitation == nil {
		t.Fatal("expected pending invitation to be stored")
	}
	if invitation.Role != registry.MemberRoleTech {
		t.Fatalf("role = %q, want %q", invitation.Role, registry.MemberRoleTech)
	}
}

func TestInviteMember_ListMembersIncludesPendingInvitationForManagers(t *testing.T) {
	reg := newTestRegistry(t)
	handler := HandleListMembers(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.UpsertInvitation(&registry.AccountInvitation{
		AccountID: accountID,
		Email:     "pending@example.com",
		Role:      registry.MemberRoleAdmin,
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/accounts/"+accountID+"/members", nil)
	req.SetPathValue("account_id", accountID)
	req.Header.Set("X-User-Role", string(registry.MemberRoleOwner))
	rec := doRawRequest(t, http.HandlerFunc(handler), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var subjects []struct {
		SubjectID string `json:"subject_id"`
		State     string `json:"state"`
		Email     string `json:"email"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &subjects); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(subjects) != 1 {
		t.Fatalf("subjects = %+v, want single pending invitation", subjects)
	}
	if subjects[0].State != string(registry.AccountAccessStatePending) {
		t.Fatalf("state = %q, want %q", subjects[0].State, registry.AccountAccessStatePending)
	}
	if subjects[0].Email != "pending@example.com" {
		t.Fatalf("email = %q, want %q", subjects[0].Email, "pending@example.com")
	}
	if subjects[0].SubjectID == "" {
		t.Fatal("expected subject_id for pending invitation")
	}
}

func TestInviteExistingUser(t *testing.T) {
	reg := newTestRegistry(t)
	mux := newTestMux(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}

	userID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: userID, Email: "existing@msp.com"}); err != nil {
		t.Fatal(err)
	}

	body := `{"email":"existing@msp.com","role":"tech"}`
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/members", bytes.NewBufferString(body))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	u, err := reg.GetUserByEmail("existing@msp.com")
	if err != nil {
		t.Fatal(err)
	}
	if u == nil || u.ID != userID {
		t.Fatalf("user = %+v, want id=%q", u, userID)
	}

	m, err := reg.GetMembership(accountID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected membership to be created")
	}
}

func TestListMembers(t *testing.T) {
	reg := newTestRegistry(t)
	mux := newTestMux(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}

	u1ID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	u2ID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: u1ID, Email: "owner@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: u2ID, Email: "tech@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: u1ID, Role: registry.MemberRoleOwner}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: u2ID, Role: registry.MemberRoleTech}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/accounts/"+accountID+"/members", nil)
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got []struct {
		UserID string              `json:"user_id"`
		Email  string              `json:"email"`
		Role   registry.MemberRole `json:"role"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 members, got %d (%+v)", len(got), got)
	}

	sort.Slice(got, func(i, j int) bool { return got[i].Email < got[j].Email })
	if got[0].Email != "owner@msp.com" || got[0].Role != registry.MemberRoleOwner {
		t.Fatalf("member[0]=%+v, want owner@msp.com owner", got[0])
	}
	if got[1].Email != "tech@msp.com" || got[1].Role != registry.MemberRoleTech {
		t.Fatalf("member[1]=%+v, want tech@msp.com tech", got[1])
	}
}

func TestUpdateMemberRole(t *testing.T) {
	reg := newTestRegistry(t)
	mux := newTestMux(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}

	userID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: userID, Email: "tech@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: userID, Role: registry.MemberRoleTech}); err != nil {
		t.Fatal(err)
	}

	body := `{"role":"admin"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/accounts/"+accountID+"/members/"+userID, bytes.NewBufferString(body))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	m, err := reg.GetMembership(accountID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected membership to exist")
	}
	if m.Role != registry.MemberRoleAdmin {
		t.Fatalf("role = %q, want %q", m.Role, registry.MemberRoleAdmin)
	}
}

func TestRemoveMember(t *testing.T) {
	reg := newTestRegistry(t)
	mux := newTestMux(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}

	ownerID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	techID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: ownerID, Email: "owner@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: techID, Email: "tech@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: ownerID, Role: registry.MemberRoleOwner}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: techID, Role: registry.MemberRoleTech}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/accounts/"+accountID+"/members/"+techID, nil)
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	m, err := reg.GetMembership(accountID, techID)
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatalf("expected membership to be deleted, got %+v", m)
	}
}

func TestCannotRemoveLastOwner(t *testing.T) {
	reg := newTestRegistry(t)
	mux := newTestMux(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}

	ownerID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: ownerID, Email: "owner@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: ownerID, Role: registry.MemberRoleOwner}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/accounts/"+accountID+"/members/"+ownerID, nil)
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusConflict && rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d or %d (body=%q)", rec.Code, http.StatusConflict, http.StatusBadRequest, rec.Body.String())
	}

	m, err := reg.GetMembership(accountID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected owner membership to remain")
	}
}

func TestUpdateMemberRole_AdminCannotDemoteOwner(t *testing.T) {
	reg := newTestRegistry(t)
	handler := HandleUpdateMemberRole(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}

	ownerID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	adminID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: ownerID, Email: "owner@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: adminID, Email: "admin@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: ownerID, Role: registry.MemberRoleOwner}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: adminID, Role: registry.MemberRoleAdmin}); err != nil {
		t.Fatal(err)
	}

	body := `{"role":"tech"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/accounts/"+accountID+"/members/"+ownerID, bytes.NewBufferString(body))
	req.SetPathValue("account_id", accountID)
	req.SetPathValue("user_id", ownerID)
	req.Header.Set("X-User-Role", string(registry.MemberRoleAdmin))
	rec := doRawRequest(t, http.HandlerFunc(handler), req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	m, err := reg.GetMembership(accountID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil || m.Role != registry.MemberRoleOwner {
		t.Fatalf("owner role changed unexpectedly: %+v", m)
	}
}

func TestUpdateMemberRole_CannotDemoteLastOwner(t *testing.T) {
	reg := newTestRegistry(t)
	mux := newTestMux(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}

	ownerID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: ownerID, Email: "owner@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: ownerID, Role: registry.MemberRoleOwner}); err != nil {
		t.Fatal(err)
	}

	body := `{"role":"admin"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/accounts/"+accountID+"/members/"+ownerID, bytes.NewBufferString(body))
	req.Header.Set("X-User-Role", string(registry.MemberRoleOwner))
	rec := doRequest(t, mux, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusConflict, rec.Body.String())
	}

	m, err := reg.GetMembership(accountID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil || m.Role != registry.MemberRoleOwner {
		t.Fatalf("owner role changed unexpectedly: %+v", m)
	}
}

func TestRemoveMember_AdminCannotRemoveOwner(t *testing.T) {
	reg := newTestRegistry(t)
	handler := HandleRemoveMember(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}

	ownerID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	adminID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: ownerID, Email: "owner@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: adminID, Email: "admin@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: ownerID, Role: registry.MemberRoleOwner}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: adminID, Role: registry.MemberRoleAdmin}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/accounts/"+accountID+"/members/"+ownerID, nil)
	req.SetPathValue("account_id", accountID)
	req.SetPathValue("user_id", ownerID)
	req.Header.Set("X-User-Role", string(registry.MemberRoleAdmin))
	rec := doRawRequest(t, http.HandlerFunc(handler), req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	m, err := reg.GetMembership(accountID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil || m.Role != registry.MemberRoleOwner {
		t.Fatalf("owner membership changed unexpectedly: %+v", m)
	}
}

func TestInviteMember_ForbiddenWhenActorRoleMissing(t *testing.T) {
	reg := newTestRegistry(t)
	handler := HandleInviteMember(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}

	body := `{"email":"tech@msp.com","role":"tech"}`
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+accountID+"/members", bytes.NewBufferString(body))
	req.SetPathValue("account_id", accountID)
	rec := doRawRequest(t, http.HandlerFunc(handler), req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestUpdateMemberRole_ForbiddenWhenActorRoleMissing(t *testing.T) {
	reg := newTestRegistry(t)
	handler := HandleUpdateMemberRole(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}

	userID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: userID, Email: "tech@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: userID, Role: registry.MemberRoleTech}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/api/accounts/"+accountID+"/members/"+userID, bytes.NewBufferString(`{"role":"admin"}`))
	req.SetPathValue("account_id", accountID)
	req.SetPathValue("user_id", userID)
	rec := doRawRequest(t, http.HandlerFunc(handler), req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestRemoveMember_ForbiddenWhenActorRoleMissing(t *testing.T) {
	reg := newTestRegistry(t)
	handler := HandleRemoveMember(reg)

	accountID, err := registry.GenerateAccountID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}

	userID, err := registry.GenerateUserID()
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateUser(&registry.User{ID: userID, Email: "tech@msp.com"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: userID, Role: registry.MemberRoleTech}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/accounts/"+accountID+"/members/"+userID, nil)
	req.SetPathValue("account_id", accountID)
	req.SetPathValue("user_id", userID)
	rec := doRawRequest(t, http.HandlerFunc(handler), req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

type fakeWorkspaceAccessLimiter struct {
	reg     *registry.TenantRegistry
	err     error
	revoked []string
	lowered []string
	synced  []string
}

func (f *fakeWorkspaceAccessLimiter) RevokeWorkspaceAccess(_ context.Context, accountID, userID string) error {
	// The membership is still there: workspaces are updated first.
	if m, err := f.reg.GetMembership(accountID, userID); err != nil || m == nil {
		return fmt.Errorf("membership of %s already removed (err=%v)", userID, err)
	}
	f.revoked = append(f.revoked, userID)
	return f.err
}

func (f *fakeWorkspaceAccessLimiter) LowerWorkspaceAccess(_ context.Context, _, userID string, role registry.MemberRole) error {
	f.lowered = append(f.lowered, userID+"="+string(role))
	return f.err
}

func (f *fakeWorkspaceAccessLimiter) SyncWorkspaceAccess(_ context.Context, _, userID string, role registry.MemberRole) error {
	f.synced = append(f.synced, userID+"="+string(role))
	return f.err
}

func TestMemberRemovalAndDemotionAreCarriedIntoWorkspaces(t *testing.T) {
	reg := newTestRegistry(t)
	const accountID = "a-workspaces"
	if err := reg.CreateAccount(&registry.Account{ID: accountID, Kind: registry.AccountKindMSP, DisplayName: "Test"}); err != nil {
		t.Fatal(err)
	}
	for userID, role := range map[string]registry.MemberRole{"u_owner": registry.MemberRoleOwner, "u_admin": registry.MemberRoleAdmin} {
		if err := reg.CreateUser(&registry.User{ID: userID, Email: userID + "@msp.com"}); err != nil {
			t.Fatal(err)
		}
		if err := reg.CreateMembership(&registry.AccountMembership{AccountID: accountID, UserID: userID, Role: role}); err != nil {
			t.Fatal(err)
		}
	}
	if err := reg.UpsertInvitation(&registry.AccountInvitation{ID: "inv_pending", AccountID: accountID, Email: "pending@msp.com", Role: registry.MemberRoleAdmin}); err != nil {
		t.Fatal(err)
	}
	pending, err := reg.ListInvitationsByAccount(accountID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("ListInvitationsByAccount = %v, %v", pending, err)
	}

	workspaces := &fakeWorkspaceAccessLimiter{reg: reg}
	mux := http.NewServeMux()
	mux.Handle("POST /api/accounts/{account_id}/members", HandleInviteMember(reg, workspaces))
	mux.Handle("PATCH /api/accounts/{account_id}/members/{user_id}", HandleUpdateMemberRole(reg, workspaces))
	mux.Handle("DELETE /api/accounts/{account_id}/members/{user_id}", HandleRemoveMember(reg, workspaces))
	do := func(method, subjectID, body string) int {
		t.Helper()
		path := "/api/accounts/" + accountID + "/members"
		if subjectID != "" {
			path += "/" + subjectID
		}
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("X-User-Role", string(registry.MemberRoleOwner))
		return doRawRequest(t, mux, req).Code
	}
	role := func() registry.MemberRole {
		t.Helper()
		m, err := reg.GetMembership(accountID, "u_admin")
		if err != nil || m == nil {
			t.Fatalf("GetMembership = %v, %v", m, err)
		}
		return m.Role
	}

	// A pending invitation has opened no workspace.
	if code := do(http.MethodDelete, pending[0].ID, ""); code != http.StatusNoContent {
		t.Fatalf("remove invitation: status = %d", code)
	}
	if len(workspaces.revoked) != 0 || len(workspaces.lowered) != 0 || len(workspaces.synced) != 0 {
		t.Fatalf("workspaces touched for pending invitation: revoked=%v lowered=%v synced=%v", workspaces.revoked, workspaces.lowered, workspaces.synced)
	}
	// A promotion syncs the higher role into existing workspaces.
	if code := do(http.MethodPatch, "u_admin", `{"role":"owner"}`); code != http.StatusOK {
		t.Fatalf("promote: status = %d", code)
	}
	if got, want := strings.Join(workspaces.synced, ","), "u_admin=owner"; got != want {
		t.Fatalf("synced after promote = %q, want %q", got, want)
	}
	if len(workspaces.revoked) != 0 || len(workspaces.lowered) != 0 {
		t.Fatalf("workspaces revoked/lowered on promotion: revoked=%v lowered=%v", workspaces.revoked, workspaces.lowered)
	}

	// A workspace that cannot be updated keeps the account as it was.
	workspaces.err = errors.New("workspace unavailable")
	if code := do(http.MethodPatch, "u_admin", `{"role":"tech"}`); code != http.StatusInternalServerError {
		t.Fatalf("demote with failing workspaces: status = %d", code)
	}
	if got := role(); got != registry.MemberRoleOwner {
		t.Fatalf("role after failed demotion = %q", got)
	}
	if code := do(http.MethodDelete, "u_admin", ""); code != http.StatusInternalServerError {
		t.Fatalf("remove with failing workspaces: status = %d", code)
	}
	role()

	workspaces.err = nil
	if code := do(http.MethodPatch, "u_admin", `{"role":"tech"}`); code != http.StatusOK {
		t.Fatalf("demote: status = %d", code)
	}
	if got := role(); got != registry.MemberRoleTech {
		t.Fatalf("role after demotion = %q", got)
	}
	if code := do(http.MethodDelete, "u_admin", ""); code != http.StatusNoContent {
		t.Fatalf("remove: status = %d", code)
	}
	if m, err := reg.GetMembership(accountID, "u_admin"); err != nil || m != nil {
		t.Fatalf("membership after removal = %v, %v", m, err)
	}
	// Re-inviting a removed user fails atomically if workspace sync fails, and
	// syncs into existing workspaces when it succeeds.
	workspaces.err = errors.New("workspace unavailable")
	if code := do(http.MethodPost, "", `{"email":"u_admin@msp.com","role":"admin"}`); code != http.StatusInternalServerError {
		t.Fatalf("re-invite with failing workspaces: status = %d", code)
	}
	if m, err := reg.GetMembership(accountID, "u_admin"); err != nil || m != nil {
		t.Fatalf("membership should be rolled back on failed workspace sync = %v, %v", m, err)
	}
	workspaces.err = nil
	if code := do(http.MethodPost, "", `{"email":"u_admin@msp.com","role":"admin"}`); code != http.StatusCreated {
		t.Fatalf("re-invite: status = %d", code)
	}
	if got := role(); got != registry.MemberRoleAdmin {
		t.Fatalf("role after re-invite = %q", got)
	}
	if got, want := strings.Join(workspaces.lowered, ","), "u_admin=tech,u_admin=tech"; got != want {
		t.Fatalf("lowered = %q, want %q", got, want)
	}
	if got, want := strings.Join(workspaces.revoked, ","), "u_admin,u_admin"; got != want {
		t.Fatalf("revoked = %q, want %q", got, want)
	}
	if got, want := strings.Join(workspaces.synced, ","), "u_admin=owner,u_admin=admin,u_admin=admin"; got != want {
		t.Fatalf("synced = %q, want %q", got, want)
	}
}
