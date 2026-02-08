package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestOrgHandlersCRUDLifecycle(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	createBody := bytes.NewBufferString(`{"id":"acme","displayName":"Acme Corp"}`)
	createReq := withUser(httptest.NewRequest(http.MethodPost, "/api/orgs", createBody), "alice")
	createRec := httptest.NewRecorder()
	h.HandleCreateOrg(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRec.Code, createRec.Body.String())
	}

	listReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs", nil), "alice")
	listRec := httptest.NewRecorder()
	h.HandleListOrgs(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from list, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var listed []models.Organization
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 orgs (default + acme), got %d", len(listed))
	}

	getReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme", nil), "alice")
	getReq.SetPathValue("id", "acme")
	getRec := httptest.NewRecorder()
	h.HandleGetOrg(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from get, got %d: %s", getRec.Code, getRec.Body.String())
	}

	updateReq := withUser(
		httptest.NewRequest(http.MethodPut, "/api/orgs/acme", bytes.NewBufferString(`{"displayName":"Acme Updated"}`)),
		"alice",
	)
	updateReq.SetPathValue("id", "acme")
	updateRec := httptest.NewRecorder()
	h.HandleUpdateOrg(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from update, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	inviteReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/members", bytes.NewBufferString(`{"userId":"bob","role":"admin"}`)),
		"alice",
	)
	inviteReq.SetPathValue("id", "acme")
	inviteRec := httptest.NewRecorder()
	h.HandleInviteMember(inviteRec, inviteReq)
	if inviteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from invite, got %d: %s", inviteRec.Code, inviteRec.Body.String())
	}

	membersReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme/members", nil), "bob")
	membersReq.SetPathValue("id", "acme")
	membersRec := httptest.NewRecorder()
	h.HandleListMembers(membersRec, membersReq)
	if membersRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from member list, got %d: %s", membersRec.Code, membersRec.Body.String())
	}
	var members []models.OrganizationMember
	if err := json.Unmarshal(membersRec.Body.Bytes(), &members); err != nil {
		t.Fatalf("decode members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members (alice + bob), got %d", len(members))
	}

	deleteReq := withUser(httptest.NewRequest(http.MethodDelete, "/api/orgs/acme", nil), "bob")
	deleteReq.SetPathValue("id", "acme")
	deleteRec := httptest.NewRecorder()
	h.HandleDeleteOrg(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from delete, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}

	getAfterDeleteReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme", nil), "alice")
	getAfterDeleteReq.SetPathValue("id", "acme")
	getAfterDeleteRec := httptest.NewRecorder()
	h.HandleGetOrg(getAfterDeleteRec, getAfterDeleteReq)
	if getAfterDeleteRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d: %s", getAfterDeleteRec.Code, getAfterDeleteRec.Body.String())
	}
}

func TestOrgHandlersMemberCannotManageOrg(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	createReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"acme","displayName":"Acme"}`)),
		"alice",
	)
	createRec := httptest.NewRecorder()
	h.HandleCreateOrg(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", createRec.Code, createRec.Body.String())
	}

	inviteReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/members", bytes.NewBufferString(`{"userId":"bob","role":"member"}`)),
		"alice",
	)
	inviteReq.SetPathValue("id", "acme")
	inviteRec := httptest.NewRecorder()
	h.HandleInviteMember(inviteRec, inviteReq)
	if inviteRec.Code != http.StatusOK {
		t.Fatalf("invite failed: %d %s", inviteRec.Code, inviteRec.Body.String())
	}

	updateReq := withUser(
		httptest.NewRequest(http.MethodPut, "/api/orgs/acme", bytes.NewBufferString(`{"displayName":"Nope"}`)),
		"bob",
	)
	updateReq.SetPathValue("id", "acme")
	updateRec := httptest.NewRecorder()
	h.HandleUpdateOrg(updateRec, updateReq)
	if updateRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for member update, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	deleteReq := withUser(httptest.NewRequest(http.MethodDelete, "/api/orgs/acme", nil), "bob")
	deleteReq.SetPathValue("id", "acme")
	deleteRec := httptest.NewRecorder()
	h.HandleDeleteOrg(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for member delete, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestOrgHandlersTokenListAllowedButWriteForbidden(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	createReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"acme","displayName":"Acme"}`)),
		"alice",
	)
	createRec := httptest.NewRecorder()
	h.HandleCreateOrg(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", createRec.Code, createRec.Body.String())
	}

	token := &config.APITokenRecord{OrgID: "acme"}
	listReq := httptest.NewRequest(http.MethodGet, "/api/orgs", nil)
	attachAPITokenRecord(listReq, token)
	listRec := httptest.NewRecorder()
	h.HandleListOrgs(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for token list, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var listed []models.Organization
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode token list: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 visible orgs for token (default + acme), got %d", len(listed))
	}

	writeReq := httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"x","displayName":"X"}`))
	attachAPITokenRecord(writeReq, token)
	writeRec := httptest.NewRecorder()
	h.HandleCreateOrg(writeRec, writeReq)
	if writeRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for token write, got %d: %s", writeRec.Code, writeRec.Body.String())
	}
}

func TestOrgHandlersMultiTenantGate(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(false)

	req := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs", nil), "alice")
	rec := httptest.NewRecorder()
	h.HandleListOrgs(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 when feature disabled, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOrgHandlersNormalizesLegacyMemberRoleToViewer(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	createReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"acme","displayName":"Acme"}`)),
		"alice",
	)
	createRec := httptest.NewRecorder()
	h.HandleCreateOrg(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", createRec.Code, createRec.Body.String())
	}

	inviteReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/members", bytes.NewBufferString(`{"userId":"bob","role":"member"}`)),
		"alice",
	)
	inviteReq.SetPathValue("id", "acme")
	inviteRec := httptest.NewRecorder()
	h.HandleInviteMember(inviteRec, inviteReq)
	if inviteRec.Code != http.StatusOK {
		t.Fatalf("invite failed: %d %s", inviteRec.Code, inviteRec.Body.String())
	}

	var member models.OrganizationMember
	if err := json.Unmarshal(inviteRec.Body.Bytes(), &member); err != nil {
		t.Fatalf("decode member: %v", err)
	}
	if member.Role != models.OrgRoleViewer {
		t.Fatalf("expected normalized viewer role, got %q", member.Role)
	}
}

func TestOrgHandlersOwnershipTransfer(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	createReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"acme","displayName":"Acme"}`)),
		"alice",
	)
	createRec := httptest.NewRecorder()
	h.HandleCreateOrg(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", createRec.Code, createRec.Body.String())
	}

	transferReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/members", bytes.NewBufferString(`{"userId":"bob","role":"owner"}`)),
		"alice",
	)
	transferReq.SetPathValue("id", "acme")
	transferRec := httptest.NewRecorder()
	h.HandleInviteMember(transferRec, transferReq)
	if transferRec.Code != http.StatusOK {
		t.Fatalf("transfer failed: %d %s", transferRec.Code, transferRec.Body.String())
	}

	getReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme", nil), "bob")
	getReq.SetPathValue("id", "acme")
	getRec := httptest.NewRecorder()
	h.HandleGetOrg(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get failed: %d %s", getRec.Code, getRec.Body.String())
	}

	var org models.Organization
	if err := json.Unmarshal(getRec.Body.Bytes(), &org); err != nil {
		t.Fatalf("decode org: %v", err)
	}
	if org.OwnerUserID != "bob" {
		t.Fatalf("expected owner bob, got %q", org.OwnerUserID)
	}
	if org.GetMemberRole("alice") != models.OrgRoleAdmin {
		t.Fatalf("expected previous owner to become admin, got %q", org.GetMemberRole("alice"))
	}
}

func TestOrgHandlersRemoveMember(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	createReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"acme","displayName":"Acme"}`)),
		"alice",
	)
	createRec := httptest.NewRecorder()
	h.HandleCreateOrg(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", createRec.Code, createRec.Body.String())
	}

	inviteReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/members", bytes.NewBufferString(`{"userId":"bob","role":"editor"}`)),
		"alice",
	)
	inviteReq.SetPathValue("id", "acme")
	inviteRec := httptest.NewRecorder()
	h.HandleInviteMember(inviteRec, inviteReq)
	if inviteRec.Code != http.StatusOK {
		t.Fatalf("invite failed: %d %s", inviteRec.Code, inviteRec.Body.String())
	}

	removeReq := withUser(httptest.NewRequest(http.MethodDelete, "/api/orgs/acme/members/bob", nil), "alice")
	removeReq.SetPathValue("id", "acme")
	removeReq.SetPathValue("userId", "bob")
	removeRec := httptest.NewRecorder()
	h.HandleRemoveMember(removeRec, removeReq)
	if removeRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 remove, got %d: %s", removeRec.Code, removeRec.Body.String())
	}

	membersReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme/members", nil), "alice")
	membersReq.SetPathValue("id", "acme")
	membersRec := httptest.NewRecorder()
	h.HandleListMembers(membersRec, membersReq)
	if membersRec.Code != http.StatusOK {
		t.Fatalf("member list failed: %d %s", membersRec.Code, membersRec.Body.String())
	}
	var members []models.OrganizationMember
	if err := json.Unmarshal(membersRec.Body.Bytes(), &members); err != nil {
		t.Fatalf("decode members: %v", err)
	}
	if len(members) != 1 || members[0].UserID != "alice" {
		t.Fatalf("expected only alice after removal, got %+v", members)
	}
}

func TestOrgHandlersShareLifecycle(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	createSourceReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"acme","displayName":"Acme"}`)),
		"alice",
	)
	createSourceRec := httptest.NewRecorder()
	h.HandleCreateOrg(createSourceRec, createSourceReq)
	if createSourceRec.Code != http.StatusCreated {
		t.Fatalf("create source failed: %d %s", createSourceRec.Code, createSourceRec.Body.String())
	}

	createTargetReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"beta","displayName":"Beta"}`)),
		"alice",
	)
	createTargetRec := httptest.NewRecorder()
	h.HandleCreateOrg(createTargetRec, createTargetReq)
	if createTargetRec.Code != http.StatusCreated {
		t.Fatalf("create target failed: %d %s", createTargetRec.Code, createTargetRec.Body.String())
	}

	createShareReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/shares", bytes.NewBufferString(`{"targetOrgId":"beta","resourceType":"view","resourceId":"alerts-high-cpu","resourceName":"High CPU","accessRole":"viewer"}`)),
		"alice",
	)
	createShareReq.SetPathValue("id", "acme")
	createShareRec := httptest.NewRecorder()
	h.HandleCreateShare(createShareRec, createShareReq)
	if createShareRec.Code != http.StatusCreated {
		t.Fatalf("create share failed: %d %s", createShareRec.Code, createShareRec.Body.String())
	}
	var share models.OrganizationShare
	if err := json.Unmarshal(createShareRec.Body.Bytes(), &share); err != nil {
		t.Fatalf("decode share: %v", err)
	}
	if share.ID == "" {
		t.Fatalf("expected share id")
	}

	incomingReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/beta/shares/incoming", nil), "alice")
	incomingReq.SetPathValue("id", "beta")
	incomingRec := httptest.NewRecorder()
	h.HandleListIncomingShares(incomingRec, incomingReq)
	if incomingRec.Code != http.StatusOK {
		t.Fatalf("list incoming failed: %d %s", incomingRec.Code, incomingRec.Body.String())
	}
	var incoming []incomingOrganizationShare
	if err := json.Unmarshal(incomingRec.Body.Bytes(), &incoming); err != nil {
		t.Fatalf("decode incoming: %v", err)
	}
	if len(incoming) != 1 || incoming[0].SourceOrgID != "acme" {
		t.Fatalf("unexpected incoming shares: %+v", incoming)
	}

	deleteShareReq := withUser(httptest.NewRequest(http.MethodDelete, "/api/orgs/acme/shares/"+share.ID, nil), "alice")
	deleteShareReq.SetPathValue("id", "acme")
	deleteShareReq.SetPathValue("shareId", share.ID)
	deleteShareRec := httptest.NewRecorder()
	h.HandleDeleteShare(deleteShareRec, deleteShareReq)
	if deleteShareRec.Code != http.StatusNoContent {
		t.Fatalf("delete share failed: %d %s", deleteShareRec.Code, deleteShareRec.Body.String())
	}
}

func TestOrgHandlersCrossOrgIsolation(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	createAcmeReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"acme","displayName":"Acme"}`)),
		"alice",
	)
	createAcmeRec := httptest.NewRecorder()
	h.HandleCreateOrg(createAcmeRec, createAcmeReq)
	if createAcmeRec.Code != http.StatusCreated {
		t.Fatalf("create acme failed: %d %s", createAcmeRec.Code, createAcmeRec.Body.String())
	}

	createBetaReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"beta","displayName":"Beta"}`)),
		"bob",
	)
	createBetaRec := httptest.NewRecorder()
	h.HandleCreateOrg(createBetaRec, createBetaReq)
	if createBetaRec.Code != http.StatusCreated {
		t.Fatalf("create beta failed: %d %s", createBetaRec.Code, createBetaRec.Body.String())
	}

	aliceGetBetaReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/beta", nil), "alice")
	aliceGetBetaReq.SetPathValue("id", "beta")
	aliceGetBetaRec := httptest.NewRecorder()
	h.HandleGetOrg(aliceGetBetaRec, aliceGetBetaReq)
	if aliceGetBetaRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-org get, got %d: %s", aliceGetBetaRec.Code, aliceGetBetaRec.Body.String())
	}

	aliceListBetaMembersReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/beta/members", nil), "alice")
	aliceListBetaMembersReq.SetPathValue("id", "beta")
	aliceListBetaMembersRec := httptest.NewRecorder()
	h.HandleListMembers(aliceListBetaMembersRec, aliceListBetaMembersReq)
	if aliceListBetaMembersRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-org member list, got %d: %s", aliceListBetaMembersRec.Code, aliceListBetaMembersRec.Body.String())
	}

	bobGetAcmeReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme", nil), "bob")
	bobGetAcmeReq.SetPathValue("id", "acme")
	bobGetAcmeRec := httptest.NewRecorder()
	h.HandleGetOrg(bobGetAcmeRec, bobGetAcmeReq)
	if bobGetAcmeRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for reverse cross-org get, got %d: %s", bobGetAcmeRec.Code, bobGetAcmeRec.Body.String())
	}
}

func TestOrgHandlersShareIsolationAcrossOrganizations(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	createAcmeReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"acme","displayName":"Acme"}`)),
		"alice",
	)
	createAcmeRec := httptest.NewRecorder()
	h.HandleCreateOrg(createAcmeRec, createAcmeReq)
	if createAcmeRec.Code != http.StatusCreated {
		t.Fatalf("create acme failed: %d %s", createAcmeRec.Code, createAcmeRec.Body.String())
	}

	createBetaReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs", bytes.NewBufferString(`{"id":"beta","displayName":"Beta"}`)),
		"bob",
	)
	createBetaRec := httptest.NewRecorder()
	h.HandleCreateOrg(createBetaRec, createBetaReq)
	if createBetaRec.Code != http.StatusCreated {
		t.Fatalf("create beta failed: %d %s", createBetaRec.Code, createBetaRec.Body.String())
	}

	createShareReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/shares", bytes.NewBufferString(`{"targetOrgId":"beta","resourceType":"view","resourceId":"r-1","resourceName":"Shared View","accessRole":"viewer"}`)),
		"alice",
	)
	createShareReq.SetPathValue("id", "acme")
	createShareRec := httptest.NewRecorder()
	h.HandleCreateShare(createShareRec, createShareReq)
	if createShareRec.Code != http.StatusCreated {
		t.Fatalf("create share failed: %d %s", createShareRec.Code, createShareRec.Body.String())
	}

	bobIncomingReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/beta/shares/incoming", nil), "bob")
	bobIncomingReq.SetPathValue("id", "beta")
	bobIncomingRec := httptest.NewRecorder()
	h.HandleListIncomingShares(bobIncomingRec, bobIncomingReq)
	if bobIncomingRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for incoming shares in target org, got %d: %s", bobIncomingRec.Code, bobIncomingRec.Body.String())
	}
	var incoming []incomingOrganizationShare
	if err := json.Unmarshal(bobIncomingRec.Body.Bytes(), &incoming); err != nil {
		t.Fatalf("decode incoming: %v", err)
	}
	if len(incoming) != 1 || incoming[0].SourceOrgID != "acme" {
		t.Fatalf("unexpected incoming shares payload: %+v", incoming)
	}

	bobSourceSharesReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme/shares", nil), "bob")
	bobSourceSharesReq.SetPathValue("id", "acme")
	bobSourceSharesRec := httptest.NewRecorder()
	h.HandleListShares(bobSourceSharesRec, bobSourceSharesReq)
	if bobSourceSharesRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when target org user lists source org shares, got %d: %s", bobSourceSharesRec.Code, bobSourceSharesRec.Body.String())
	}

	charlieIncomingReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/beta/shares/incoming", nil), "charlie")
	charlieIncomingReq.SetPathValue("id", "beta")
	charlieIncomingRec := httptest.NewRecorder()
	h.HandleListIncomingShares(charlieIncomingRec, charlieIncomingReq)
	if charlieIncomingRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when non-member lists incoming shares, got %d: %s", charlieIncomingRec.Code, charlieIncomingRec.Body.String())
	}
}

func withUser(req *http.Request, username string) *http.Request {
	return req.WithContext(internalauth.WithUser(req.Context(), username))
}
