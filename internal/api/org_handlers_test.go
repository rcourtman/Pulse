package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

type testLogCapture struct {
	id    string
	lines chan string
	buf   strings.Builder
}

func newTestLogCapture(t *testing.T) *testLogCapture {
	t.Helper()
	id, lines, _ := logging.GetBroadcaster().Subscribe()
	capture := &testLogCapture{id: id, lines: lines}
	t.Cleanup(func() {
		logging.GetBroadcaster().Unsubscribe(id)
	})
	return capture
}

func (c *testLogCapture) String() string {
	for {
		select {
		case line, ok := <-c.lines:
			if !ok {
				return c.buf.String()
			}
			c.buf.WriteString(line)
			if !strings.HasSuffix(line, "\n") {
				c.buf.WriteByte('\n')
			}
		default:
			return c.buf.String()
		}
	}
}

func TestOrgHandlersRequireMultiTenantGate_HostedModeBypassesFeatureLicense(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(false)
	setupHostedLicenseProvider(t, pkglicensing.SubStateActive, nil)

	h := NewOrgHandlers(config.NewMultiTenantPersistence(t.TempDir()), nil)
	h.SetHostedMode(true)

	req := httptest.NewRequest(http.MethodGet, "/api/orgs/t-tenant/members", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "t-tenant"))
	rec := httptest.NewRecorder()

	if !h.requireMultiTenantGate(rec, req) {
		t.Fatalf("expected hosted org gate to allow active hosted subscription, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOrgHandlersRequireMultiTenantGate_HostedModeBlocksInactiveSubscription(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	setupHostedLicenseProvider(t, pkglicensing.SubStateExpired, nil)

	h := NewOrgHandlers(config.NewMultiTenantPersistence(t.TempDir()), nil)
	h.SetHostedMode(true)

	req := httptest.NewRequest(http.MethodGet, "/api/orgs/t-tenant/members", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "t-tenant"))
	rec := httptest.NewRecorder()

	if h.requireMultiTenantGate(rec, req) {
		t.Fatalf("expected hosted org gate to block expired subscription")
	}
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOrgHandlersDeleteInvokesOnDeleteCallback(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	var callbackOrgID string
	h.SetOnDelete(func(_ context.Context, orgID string) error {
		callbackOrgID = orgID
		return nil
	})

	createBody := bytes.NewBufferString(`{"id":"acme","displayName":"Acme Corp"}`)
	createReq := withUser(httptest.NewRequest(http.MethodPost, "/api/orgs", createBody), "alice")
	createRec := httptest.NewRecorder()
	h.HandleCreateOrg(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRec.Code, createRec.Body.String())
	}

	deleteReq := withUser(httptest.NewRequest(http.MethodDelete, "/api/orgs/acme", nil), "alice")
	deleteReq.SetPathValue("id", "acme")
	deleteRec := httptest.NewRecorder()
	h.HandleDeleteOrg(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from delete, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
	if callbackOrgID != "acme" {
		t.Fatalf("expected delete callback org ID acme, got %q", callbackOrgID)
	}
}

func TestOrgHandlersDeleteStopsTenantMonitorBeforeDeletingOrgDir(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	logOutput := newTestLogCapture(t)

	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{DataPath: baseDir}, persistence, nil)
	t.Cleanup(mtm.Stop)

	h := NewOrgHandlers(persistence, mtm)

	createBody := bytes.NewBufferString(`{"id":"acme","displayName":"Acme Corp"}`)
	createReq := withUser(httptest.NewRequest(http.MethodPost, "/api/orgs", createBody), "alice")
	createRec := httptest.NewRecorder()
	h.HandleCreateOrg(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRec.Code, createRec.Body.String())
	}

	if _, err := mtm.GetMonitor("acme"); err != nil {
		t.Fatalf("GetMonitor(acme) failed: %v", err)
	}

	deleteReq := withUser(httptest.NewRequest(http.MethodDelete, "/api/orgs/acme", nil), "alice")
	deleteReq.SetPathValue("id", "acme")
	deleteRec := httptest.NewRecorder()
	h.HandleDeleteOrg(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from delete, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}

	if strings.Contains(logOutput.String(), "Failed to save alert history on shutdown") {
		t.Fatalf("expected org deletion to stop tenant monitor before removing history dir, got logs: %s", logOutput.String())
	}

	orgDir := filepath.Join(baseDir, "orgs", "acme")
	if _, err := os.Stat(orgDir); !os.IsNotExist(err) {
		t.Fatalf("expected deleted org dir %s to be removed, stat err = %v", orgDir, err)
	}
}

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
	if inviteRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 from invite, got %d: %s", inviteRec.Code, inviteRec.Body.String())
	}

	acceptInvitationForTest(t, h, "acme", "bob")

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

func TestOrgHandlersViewerCannotManageOrg(t *testing.T) {
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
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/members", bytes.NewBufferString(`{"userId":"bob","role":"viewer"}`)),
		"alice",
	)
	inviteReq.SetPathValue("id", "acme")
	inviteRec := httptest.NewRecorder()
	h.HandleInviteMember(inviteRec, inviteReq)
	if inviteRec.Code != http.StatusAccepted {
		t.Fatalf("invite failed: %d %s", inviteRec.Code, inviteRec.Body.String())
	}

	acceptInvitationForTest(t, h, "acme", "bob")

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

func TestOrgHandlersRejectContactEmailWithoutStoredPrincipalMatch(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	if err := persistence.SaveOrganization(&models.Organization{
		ID:          "acme",
		DisplayName: "Acme",
		OwnerUserID: "u_owner",
		OwnerEmail:  "owner@example.com",
		Members: []models.OrganizationMember{
			{UserID: "u_owner", Email: "owner@example.com", Role: models.OrgRoleOwner, AddedAt: time.Now()},
			{UserID: "u_admin", Email: "admin@example.com", Role: models.OrgRoleAdmin, AddedAt: time.Now()},
		},
	}); err != nil {
		t.Fatalf("save organization: %v", err)
	}

	getReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme", nil), "owner@example.com")
	getReq.SetPathValue("id", "acme")
	getRec := httptest.NewRecorder()
	h.HandleGetOrg(getRec, getReq)
	if getRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for owner contact-email read, got %d: %s", getRec.Code, getRec.Body.String())
	}

	updateReq := withUser(
		httptest.NewRequest(http.MethodPut, "/api/orgs/acme", bytes.NewBufferString(`{"displayName":"Nope"}`)),
		"admin@example.com",
	)
	updateReq.SetPathValue("id", "acme")
	updateRec := httptest.NewRecorder()
	h.HandleUpdateOrg(updateRec, updateReq)
	if updateRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for admin contact-email update, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	principalReq := withUser(
		httptest.NewRequest(http.MethodPut, "/api/orgs/acme", bytes.NewBufferString(`{"displayName":"Renamed"}`)),
		"u_admin",
	)
	principalReq.SetPathValue("id", "acme")
	principalRec := httptest.NewRecorder()
	h.HandleUpdateOrg(principalRec, principalReq)
	if principalRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for stored admin principal update, got %d: %s", principalRec.Code, principalRec.Body.String())
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
	if len(listed) != 1 {
		t.Fatalf("expected 1 visible org for token scoped to acme, got %d", len(listed))
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

func TestOrgHandlersRejectsLegacyMemberRole(t *testing.T) {
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
	if inviteRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for legacy member role, got %d: %s", inviteRec.Code, inviteRec.Body.String())
	}
}

func TestOrgHandlersOwnershipTransfer(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	InitSessionStore(t.TempDir())
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

	inviteMemberForTest(t, h, "acme", "alice", "bob", "viewer", http.StatusAccepted)
	acceptInvitationForTest(t, h, "acme", "bob")

	transferReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/members", bytes.NewBufferString(`{"userId":"bob","role":"owner"}`)),
		"alice",
	)
	transferSession := generateSessionToken()
	GetSessionStore().CreateSession(transferSession, time.Hour, "browser", "127.0.0.1", "alice")
	transferReq.AddCookie(&http.Cookie{Name: cookieNameSession, Value: transferSession})
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

func TestOrgHandlersOwnershipTransferRequiresFreshSession(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	InitSessionStore(t.TempDir())
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

	inviteMemberForTest(t, h, "acme", "alice", "bob", "viewer", http.StatusAccepted)
	acceptInvitationForTest(t, h, "acme", "bob")

	transferSession := generateSessionToken()
	store := GetSessionStore()
	store.CreateSession(transferSession, time.Hour, "browser", "127.0.0.1", "alice")
	store.mu.Lock()
	store.sessions[sessionHash(transferSession)].CreatedAt = time.Now().Add(-privilegedBrowserSessionMaxAge - time.Minute)
	store.mu.Unlock()

	transferReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/members", bytes.NewBufferString(`{"userId":"bob","role":"owner"}`)),
		"alice",
	)
	transferReq.AddCookie(&http.Cookie{Name: cookieNameSession, Value: transferSession})
	transferReq.SetPathValue("id", "acme")
	transferRec := httptest.NewRecorder()
	h.HandleInviteMember(transferRec, transferReq)
	if transferRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for stale owner-transfer session, got %d: %s", transferRec.Code, transferRec.Body.String())
	}
	if !strings.Contains(transferRec.Body.String(), "Sign in again to transfer ownership") {
		t.Fatalf("expected fresh-session message, got %s", transferRec.Body.String())
	}

	getReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme", nil), "alice")
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
	if org.OwnerUserID != "alice" {
		t.Fatalf("expected owner to remain alice, got %q", org.OwnerUserID)
	}
	if org.GetMemberRole("bob") != models.OrgRoleViewer {
		t.Fatalf("expected bob to remain viewer, got %q", org.GetMemberRole("bob"))
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

	inviteMemberForTest(t, h, "acme", "alice", "bob", "editor", http.StatusAccepted)
	acceptInvitationForTest(t, h, "acme", "bob")

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

func TestOrgHandlersInvitationsRequireAcceptance(t *testing.T) {
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

	inviteRec := inviteMemberForTest(t, h, "acme", "alice", "bob", "admin", http.StatusAccepted)
	var invitePayload organizationAccessMutationResponse
	if err := json.Unmarshal(inviteRec.Body.Bytes(), &invitePayload); err != nil {
		t.Fatalf("decode invite payload: %v", err)
	}
	if invitePayload.Kind != "invitation" || invitePayload.Invitation == nil {
		t.Fatalf("expected invitation payload, got %+v", invitePayload)
	}

	bobGetReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme", nil), "bob")
	bobGetReq.SetPathValue("id", "acme")
	bobGetRec := httptest.NewRecorder()
	h.HandleGetOrg(bobGetRec, bobGetReq)
	if bobGetRec.Code != http.StatusForbidden {
		t.Fatalf("expected bob to be blocked before accepting invite, got %d: %s", bobGetRec.Code, bobGetRec.Body.String())
	}

	myInvitesReq := withUser(httptest.NewRequest(http.MethodGet, "/api/org-invitations", nil), "bob")
	myInvitesRec := httptest.NewRecorder()
	h.HandleListMyInvitations(myInvitesRec, myInvitesReq)
	if myInvitesRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from my invitations, got %d: %s", myInvitesRec.Code, myInvitesRec.Body.String())
	}
	var myInvites []organizationUserInvitationResponse
	if err := json.Unmarshal(myInvitesRec.Body.Bytes(), &myInvites); err != nil {
		t.Fatalf("decode my invitations: %v", err)
	}
	if len(myInvites) != 1 || myInvites[0].OrgID != "acme" {
		t.Fatalf("unexpected invitation inbox: %+v", myInvites)
	}

	managerInvitesReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme/invitations", nil), "alice")
	managerInvitesReq.SetPathValue("id", "acme")
	managerInvitesRec := httptest.NewRecorder()
	h.HandleListInvitations(managerInvitesRec, managerInvitesReq)
	if managerInvitesRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from invitation list, got %d: %s", managerInvitesRec.Code, managerInvitesRec.Body.String())
	}
	var pending []models.OrganizationInvitation
	if err := json.Unmarshal(managerInvitesRec.Body.Bytes(), &pending); err != nil {
		t.Fatalf("decode pending invitations: %v", err)
	}
	if len(pending) != 1 || pending[0].UserID != "bob" {
		t.Fatalf("unexpected pending invitations: %+v", pending)
	}

	acceptInvitationForTest(t, h, "acme", "bob")

	orgAfterAccept, err := persistence.LoadOrganization("acme")
	if err != nil {
		t.Fatalf("load org after accept: %v", err)
	}
	if orgAfterAccept.GetMemberRole("bob") != models.OrgRoleAdmin {
		t.Fatalf("expected bob to become admin after acceptance, got %q", orgAfterAccept.GetMemberRole("bob"))
	}
	if len(orgAfterAccept.PendingInvitations) != 0 {
		t.Fatalf("expected invitations to be cleared after acceptance, got %+v", orgAfterAccept.PendingInvitations)
	}
}

func TestOrgHandlersRejectOwnershipTransferToNonMember(t *testing.T) {
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

	transferRec := inviteMemberForTest(t, h, "acme", "alice", "bob", "owner", http.StatusBadRequest)
	if !strings.Contains(transferRec.Body.String(), "existing member") {
		t.Fatalf("expected owner transfer rejection to explain member prerequisite, got %s", transferRec.Body.String())
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
		"bob",
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
	if share.Status != models.OrganizationShareStatusPending {
		t.Fatalf("share status=%q, want %q", share.Status, models.OrganizationShareStatusPending)
	}
	if !share.AcceptedAt.IsZero() {
		t.Fatalf("expected pending share accepted_at to be empty, got %s", share.AcceptedAt)
	}
	if share.AcceptedBy != "" {
		t.Fatalf("expected pending share accepted_by to be empty, got %q", share.AcceptedBy)
	}

	incomingReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/beta/shares/incoming", nil), "bob")
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
	if incoming[0].Status != models.OrganizationShareStatusPending {
		t.Fatalf("incoming status=%q, want %q", incoming[0].Status, models.OrganizationShareStatusPending)
	}

	accepted := acceptIncomingShareForTest(t, h, "beta", "bob", share.ID)
	if accepted.Status != models.OrganizationShareStatusAccepted {
		t.Fatalf("accepted share status=%q, want %q", accepted.Status, models.OrganizationShareStatusAccepted)
	}
	if accepted.AcceptedBy != "bob" {
		t.Fatalf("accepted share accepted_by=%q, want %q", accepted.AcceptedBy, "bob")
	}
	if accepted.AcceptedAt.IsZero() {
		t.Fatalf("accepted share missing accepted_at")
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

func TestOrgHandlersIncomingSharesPreserveAccessRole(t *testing.T) {
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
		"bob",
	)
	createTargetRec := httptest.NewRecorder()
	h.HandleCreateOrg(createTargetRec, createTargetReq)
	if createTargetRec.Code != http.StatusCreated {
		t.Fatalf("create target failed: %d %s", createTargetRec.Code, createTargetRec.Body.String())
	}
	inviteMemberForTest(t, h, "beta", "bob", "charlie", "editor", http.StatusAccepted)
	acceptInvitationForTest(t, h, "beta", "charlie")

	createShareReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/shares", bytes.NewBufferString(`{"targetOrgId":"beta","resourceType":"view","resourceId":"shared-view","resourceName":"Shared View","accessRole":"editor"}`)),
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
	if share.AccessRole != models.OrgRoleEditor {
		t.Fatalf("share access_role=%q, want %q", share.AccessRole, models.OrgRoleEditor)
	}
	if share.Status != models.OrganizationShareStatusPending {
		t.Fatalf("share status=%q, want %q", share.Status, models.OrganizationShareStatusPending)
	}

	pendingIncomingReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/beta/shares/incoming", nil), "charlie")
	pendingIncomingReq.SetPathValue("id", "beta")
	pendingIncomingRec := httptest.NewRecorder()
	h.HandleListIncomingShares(pendingIncomingRec, pendingIncomingReq)
	if pendingIncomingRec.Code != http.StatusOK {
		t.Fatalf("list pending incoming failed: %d %s", pendingIncomingRec.Code, pendingIncomingRec.Body.String())
	}
	var pendingIncoming []incomingOrganizationShare
	if err := json.Unmarshal(pendingIncomingRec.Body.Bytes(), &pendingIncoming); err != nil {
		t.Fatalf("decode pending incoming: %v", err)
	}
	if len(pendingIncoming) != 0 {
		t.Fatalf("expected pending share to stay hidden from non-managers, got %+v", pendingIncoming)
	}

	acceptIncomingShareForTest(t, h, "beta", "bob", share.ID)

	incomingReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/beta/shares/incoming", nil), "charlie")
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
	if len(incoming) != 1 {
		t.Fatalf("expected one incoming share, got %+v", incoming)
	}
	if incoming[0].SourceOrgID != "acme" {
		t.Fatalf("incoming source_org_id=%q, want %q", incoming[0].SourceOrgID, "acme")
	}
	if incoming[0].AccessRole != models.OrgRoleEditor {
		t.Fatalf("incoming access_role=%q, want %q", incoming[0].AccessRole, models.OrgRoleEditor)
	}
	if incoming[0].Status != models.OrganizationShareStatusAccepted {
		t.Fatalf("incoming status=%q, want %q", incoming[0].Status, models.OrganizationShareStatusAccepted)
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
	inviteMemberForTest(t, h, "beta", "bob", "charlie", "viewer", http.StatusAccepted)
	acceptInvitationForTest(t, h, "beta", "charlie")

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
	var share models.OrganizationShare
	if err := json.Unmarshal(createShareRec.Body.Bytes(), &share); err != nil {
		t.Fatalf("decode share: %v", err)
	}

	charlieIncomingReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/beta/shares/incoming", nil), "charlie")
	charlieIncomingReq.SetPathValue("id", "beta")
	charlieIncomingRec := httptest.NewRecorder()
	h.HandleListIncomingShares(charlieIncomingRec, charlieIncomingReq)
	if charlieIncomingRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for incoming shares in target org, got %d: %s", charlieIncomingRec.Code, charlieIncomingRec.Body.String())
	}
	var incoming []incomingOrganizationShare
	if err := json.Unmarshal(charlieIncomingRec.Body.Bytes(), &incoming); err != nil {
		t.Fatalf("decode incoming: %v", err)
	}
	if len(incoming) != 0 {
		t.Fatalf("expected pending incoming shares to stay hidden from non-managers, got %+v", incoming)
	}

	bobSourceSharesReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme/shares", nil), "bob")
	bobSourceSharesReq.SetPathValue("id", "acme")
	bobSourceSharesRec := httptest.NewRecorder()
	h.HandleListShares(bobSourceSharesRec, bobSourceSharesReq)
	if bobSourceSharesRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when target org user lists source org shares, got %d: %s", bobSourceSharesRec.Code, bobSourceSharesRec.Body.String())
	}

	danaIncomingReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/beta/shares/incoming", nil), "dana")
	danaIncomingReq.SetPathValue("id", "beta")
	danaIncomingRec := httptest.NewRecorder()
	h.HandleListIncomingShares(danaIncomingRec, danaIncomingReq)
	if danaIncomingRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when non-member lists incoming shares, got %d: %s", danaIncomingRec.Code, danaIncomingRec.Body.String())
	}

	acceptIncomingShareForTest(t, h, "beta", "bob", share.ID)

	acceptedIncomingReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/beta/shares/incoming", nil), "charlie")
	acceptedIncomingReq.SetPathValue("id", "beta")
	acceptedIncomingRec := httptest.NewRecorder()
	h.HandleListIncomingShares(acceptedIncomingRec, acceptedIncomingReq)
	if acceptedIncomingRec.Code != http.StatusOK {
		t.Fatalf("expected 200 after share acceptance, got %d: %s", acceptedIncomingRec.Code, acceptedIncomingRec.Body.String())
	}
	if err := json.Unmarshal(acceptedIncomingRec.Body.Bytes(), &incoming); err != nil {
		t.Fatalf("decode accepted incoming: %v", err)
	}
	if len(incoming) != 1 || incoming[0].SourceOrgID != "acme" {
		t.Fatalf("unexpected accepted incoming shares payload: %+v", incoming)
	}
}

func TestOrgHandlersDeclineIncomingShareRemovesPendingRequest(t *testing.T) {
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
		"bob",
	)
	createTargetRec := httptest.NewRecorder()
	h.HandleCreateOrg(createTargetRec, createTargetReq)
	if createTargetRec.Code != http.StatusCreated {
		t.Fatalf("create target failed: %d %s", createTargetRec.Code, createTargetRec.Body.String())
	}

	createShareReq := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/shares", bytes.NewBufferString(`{"targetOrgId":"beta","resourceType":"view","resourceId":"pending-view","resourceName":"Pending View","accessRole":"viewer"}`)),
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

	req := withUser(httptest.NewRequest(http.MethodDelete, "/api/orgs/beta/shares/incoming/"+share.ID, nil), "bob")
	req.SetPathValue("id", "beta")
	req.SetPathValue("shareId", share.ID)
	rec := httptest.NewRecorder()
	h.HandleDeclineIncomingShare(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("decline incoming share failed: %d %s", rec.Code, rec.Body.String())
	}

	sourceOrg, err := h.loadOrganization("acme")
	if err != nil {
		t.Fatalf("load source org: %v", err)
	}
	if len(sourceOrg.SharedResources) != 0 {
		t.Fatalf("expected declined share to be removed from source org, got %+v", sourceOrg.SharedResources)
	}
}

func TestOrgHandlersShareListsStripLegacyHostResourceTypes(t *testing.T) {
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

	sourceOrg, err := h.loadOrganization("acme")
	if err != nil {
		t.Fatalf("load acme: %v", err)
	}
	sourceOrg.SharedResources = []models.OrganizationShare{
		{
			ID:           "legacy-host",
			TargetOrgID:  "beta",
			ResourceType: "host",
			ResourceID:   "agent-1",
			AccessRole:   models.OrgRoleViewer,
		},
		{
			ID:           "canonical-agent",
			TargetOrgID:  "beta",
			ResourceType: "agent",
			ResourceID:   "agent-2",
			AccessRole:   models.OrgRoleViewer,
		},
	}
	if err := persistence.SaveOrganization(sourceOrg); err != nil {
		t.Fatalf("save acme with mixed shares: %v", err)
	}

	sourceSharesReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/acme/shares", nil), "alice")
	sourceSharesReq.SetPathValue("id", "acme")
	sourceSharesRec := httptest.NewRecorder()
	h.HandleListShares(sourceSharesRec, sourceSharesReq)
	if sourceSharesRec.Code != http.StatusOK {
		t.Fatalf("list source shares failed: %d %s", sourceSharesRec.Code, sourceSharesRec.Body.String())
	}

	var sourceShares []models.OrganizationShare
	if err := json.Unmarshal(sourceSharesRec.Body.Bytes(), &sourceShares); err != nil {
		t.Fatalf("decode source shares: %v", err)
	}
	if len(sourceShares) != 1 || sourceShares[0].ResourceType != "agent" {
		t.Fatalf("expected only canonical agent share in source payload, got %+v", sourceShares)
	}

	incomingReq := withUser(httptest.NewRequest(http.MethodGet, "/api/orgs/beta/shares/incoming", nil), "bob")
	incomingReq.SetPathValue("id", "beta")
	incomingRec := httptest.NewRecorder()
	h.HandleListIncomingShares(incomingRec, incomingReq)
	if incomingRec.Code != http.StatusOK {
		t.Fatalf("list incoming shares failed: %d %s", incomingRec.Code, incomingRec.Body.String())
	}

	var incoming []incomingOrganizationShare
	if err := json.Unmarshal(incomingRec.Body.Bytes(), &incoming); err != nil {
		t.Fatalf("decode incoming shares: %v", err)
	}
	if len(incoming) != 1 || incoming[0].ResourceType != "agent" {
		t.Fatalf("expected only canonical agent share in incoming payload, got %+v", incoming)
	}
}

func TestNormalizeOrganizationShareResourceType(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "host remains host in strict normalization", in: "host", want: "host"},
		{name: "host mixed case remains host in strict normalization", in: " HoSt ", want: "host"},
		{name: "agent", in: "agent", want: "agent"},
		{name: "node", in: "node", want: "node"},
		{name: "container", in: "container", want: "container"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeOrganizationShareResourceType(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeOrganizationShareResourceType(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsUnsupportedOrganizationShareResourceType(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "host unsupported", in: "host", want: true},
		{name: "mixed case host unsupported", in: " HoSt ", want: true},
		{name: "agent supported", in: "agent", want: false},
		{name: "docker host supported", in: "docker-host", want: false},
		{name: "docker image supported", in: "docker-image", want: false},
		{name: "docker volume supported", in: "docker-volume", want: false},
		{name: "docker network supported", in: "docker-network", want: false},
		{name: "docker task supported", in: "docker-task", want: false},
		{name: "kubernetes namespace supported", in: "k8s-namespace", want: false},
		{name: "kubernetes service supported", in: "k8s-service", want: false},
		{name: "kubernetes statefulset supported", in: "k8s-statefulset", want: false},
		{name: "kubernetes daemonset supported", in: "k8s-daemonset", want: false},
		{name: "kubernetes job supported", in: "k8s-job", want: false},
		{name: "kubernetes cronjob supported", in: "k8s-cronjob", want: false},
		{name: "kubernetes ingress supported", in: "k8s-ingress", want: false},
		{name: "kubernetes persistent volume supported", in: "k8s-persistent-volume", want: false},
		{name: "kubernetes persistent volume claim supported", in: "k8s-persistent-volume-claim", want: false},
		{name: "kubernetes event supported", in: "k8s-event", want: false},
		{name: "network share supported", in: "network-share", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUnsupportedOrganizationShareResourceType(tt.in)
			if got != tt.want {
				t.Fatalf("isUnsupportedOrganizationShareResourceType(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeOrganizationShares_DropsLegacyAndUnsupportedResourceTypes(t *testing.T) {
	shares := []models.OrganizationShare{
		{
			ID:           "legacy-host",
			TargetOrgID:  "beta",
			ResourceType: "host",
			ResourceID:   "agent-1",
			AccessRole:   models.OrgRoleViewer,
		},
		{
			ID:           "unsupported-custom",
			TargetOrgID:  "beta",
			ResourceType: "my-custom-type",
			ResourceID:   "res-1",
			AccessRole:   models.OrgRoleViewer,
		},
		{
			ID:           "legacy-container",
			TargetOrgID:  "beta",
			ResourceType: "container",
			ResourceID:   "ct-101",
			AccessRole:   models.OrgRoleViewer,
		},
		{
			ID:           "legacy-docker-container",
			TargetOrgID:  "beta",
			ResourceType: "docker-container",
			ResourceID:   "docker-101",
			AccessRole:   models.OrgRoleViewer,
		},
		{
			ID:           "v6-agent",
			TargetOrgID:  "beta",
			ResourceType: "agent",
			ResourceID:   "agent-2",
			AccessRole:   models.OrgRoleViewer,
		},
	}

	normalized := normalizeOrganizationShares(shares)
	if len(normalized) != 1 {
		t.Fatalf("expected only canonical v6 share to remain, got %d: %+v", len(normalized), normalized)
	}
	if normalized[0].ResourceType != "agent" {
		t.Fatalf("expected canonical agent share to remain, got %+v", normalized[0])
	}
}

func TestHandleCreateShareRejectsUnsupportedHostResourceType(t *testing.T) {
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
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/shares", bytes.NewBufferString(`{"targetOrgId":"beta","resourceType":"host","resourceId":"agent-1","accessRole":"viewer"}`)),
		"alice",
	)
	createShareReq.SetPathValue("id", "acme")
	createShareRec := httptest.NewRecorder()
	h.HandleCreateShare(createShareRec, createShareReq)

	if createShareRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported host resource type, got %d: %s", createShareRec.Code, createShareRec.Body.String())
	}
}

func TestHandleCreateShareRejectsUnsupportedCustomResourceType(t *testing.T) {
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
		httptest.NewRequest(http.MethodPost, "/api/orgs/acme/shares", bytes.NewBufferString(`{"targetOrgId":"beta","resourceType":"my-custom-type","resourceId":"res-1","accessRole":"viewer"}`)),
		"alice",
	)
	createShareReq.SetPathValue("id", "acme")
	createShareRec := httptest.NewRecorder()
	h.HandleCreateShare(createShareRec, createShareReq)

	if createShareRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported custom resource type, got %d: %s", createShareRec.Code, createShareRec.Body.String())
	}
}

func withUser(req *http.Request, username string) *http.Request {
	return req.WithContext(internalauth.WithUser(req.Context(), username))
}

func inviteMemberForTest(t *testing.T, h *OrgHandlers, orgID, actor, userID, role string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	req := withUser(
		httptest.NewRequest(http.MethodPost, "/api/orgs/"+orgID+"/members", bytes.NewBufferString(`{"userId":"`+userID+`","role":"`+role+`"}`)),
		actor,
	)
	req.SetPathValue("id", orgID)
	rec := httptest.NewRecorder()
	h.HandleInviteMember(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("invite %s as %s returned %d, want %d: %s", userID, role, rec.Code, wantStatus, rec.Body.String())
	}
	return rec
}

func acceptInvitationForTest(t *testing.T, h *OrgHandlers, orgID, username string) *httptest.ResponseRecorder {
	t.Helper()
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/org-invitations/"+orgID+"/accept", nil), username)
	req.SetPathValue("id", orgID)
	rec := httptest.NewRecorder()
	h.HandleAcceptMyInvitation(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("accept invitation for %s returned %d: %s", username, rec.Code, rec.Body.String())
	}
	return rec
}

func acceptIncomingShareForTest(t *testing.T, h *OrgHandlers, orgID, username, shareID string) models.OrganizationShare {
	t.Helper()
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/orgs/"+orgID+"/shares/incoming/"+shareID+"/accept", nil), username)
	req.SetPathValue("id", orgID)
	req.SetPathValue("shareId", shareID)
	rec := httptest.NewRecorder()
	h.HandleAcceptIncomingShare(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("accept incoming share %s for %s returned %d: %s", shareID, username, rec.Code, rec.Body.String())
	}

	var share models.OrganizationShare
	if err := json.Unmarshal(rec.Body.Bytes(), &share); err != nil {
		t.Fatalf("decode accepted share: %v", err)
	}
	return share
}
