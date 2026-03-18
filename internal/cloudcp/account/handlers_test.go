package account

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
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

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	u, err := reg.GetUserByEmail("tech@msp.com")
	if err != nil {
		t.Fatal(err)
	}
	if u == nil {
		t.Fatal("expected user to be created")
	}

	m, err := reg.GetMembership(accountID, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected membership to be created")
	}
	if m.Role != registry.MemberRoleTech {
		t.Fatalf("role = %q, want %q", m.Role, registry.MemberRoleTech)
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
	req.Header.Set("X-User-Role", string(registry.MemberRoleAdmin))
	rec := doRequest(t, mux, req)

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
	req.Header.Set("X-User-Role", string(registry.MemberRoleAdmin))
	rec := doRequest(t, mux, req)

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
