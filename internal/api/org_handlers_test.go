package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
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

// Organization management is a paid capability, so a provider client's fall back to
// Community behaviour must not open it.
func TestOrgHandlersRequireMultiTenantGate_ProviderClientWithoutLeaseStaysBlocked(t *testing.T) {
	setupHostedLicenseProvider(t, pkglicensing.SubStateExpired, nil)
	t.Setenv(pkglicensing.ProviderHostedRuntimeEnvVar, "true")
	t.Setenv("PULSE_TENANT_ID", "t-client")
	h := NewOrgHandlers(nil, nil)
	h.SetHostedMode(true)
	req := httptest.NewRequest(http.MethodPost, "/api/orgs", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "t-client"))
	rec := httptest.NewRecorder()
	if h.requireMultiTenantGate(rec, req) {
		t.Fatal("organization routes must stay gated without a valid lease")
	}
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", rec.Code)
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

func TestOrgHandlersOrganizationReadsLimitPendingInvitationsToManagers(t *testing.T) {
	enableSharedMultiOrgForTest(t)
	installTestRBACManager(t)
	t.Setenv("PULSE_DEV", "true")

	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(dir, "unused-legacy-key"))
	p := config.NewMultiTenantPersistence(dir)
	for _, org := range []models.Organization{
		{ID: "acme", DisplayName: "Acme", OwnerUserID: "alice", Members: []models.OrganizationMember{
			{UserID: "bob", Role: " ADMIN "},
			{UserID: "editor", Role: models.OrgRoleEditor},
			{UserID: "viewer", Role: models.OrgRoleViewer},
		}},
		{ID: "other", DisplayName: "Other", OwnerUserID: "other-owner", Members: []models.OrganizationMember{
			{UserID: "bob", Role: models.OrgRoleViewer},
		}},
		{ID: "default", DisplayName: "Default", OwnerUserID: "legacy-owner", Members: []models.OrganizationMember{
			{UserID: "legacy-owner", Role: models.OrgRoleOwner},
		}},
	} {
		if org.ID == "default" {
			saved, err := p.LoadOrganization(org.ID)
			if err != nil {
				t.Fatal(err)
			}
			org.PersistenceVersion = saved.PersistenceVersion
		}
		org.PendingInvitations = []models.OrganizationInvitation{{
			UserID: " invited-" + org.ID + " ", Role: " EDITOR ",
			InvitedAt: time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC), InvitedBy: org.OwnerUserID,
		}}
		if err := p.SaveOrganization(&org); err != nil {
			t.Fatal(err)
		}
	}
	h := NewOrgHandlers(p, nil)
	h.SetAccessPolicy(newOrgAccessChecker(&config.Config{AuthUser: orgAccessTestAdmin}, p, false))

	for _, tc := range []struct {
		name, user string
		token      *config.APITokenRecord
		visible    map[string]bool // org ID -> may also read pending invitations
	}{
		{name: "owner", user: "alice", visible: map[string]bool{"acme": true}},
		{name: "admin in only one workspace", user: "bob", visible: map[string]bool{"acme": true, "other": false}},
		{name: "editor", user: "editor", visible: map[string]bool{"acme": false}},
		{name: "viewer", user: "viewer", visible: map[string]bool{"acme": false}},
		{name: "default legacy owner is not instance admin", user: "legacy-owner", visible: map[string]bool{"default": false}},
		{name: "instance admin", user: orgAccessTestAdmin, visible: map[string]bool{"default": true}},
		{name: "bound token even in owner context", user: "alice", token: &config.APITokenRecord{OrgID: "acme"}, visible: map[string]bool{"acme": false}},
		{name: "multi-org token", user: "token:fixture", token: &config.APITokenRecord{OrgIDs: []string{"acme", "default"}}, visible: map[string]bool{"acme": false, "default": false}},
		{name: "invitee is not yet a member", user: "invited-acme"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			get := func(path, orgID string, handler http.HandlerFunc, status int) []byte {
				t.Helper()
				req := withUser(httptest.NewRequest(http.MethodGet, path, nil), tc.user)
				req.SetPathValue("id", orgID)
				attachAPITokenRecord(req, tc.token)
				rec := httptest.NewRecorder()
				handler(rec, req)
				if rec.Code != status {
					t.Fatalf("%s: status %d, want %d: %s", path, rec.Code, status, rec.Body.Bytes())
				}
				return rec.Body.Bytes()
			}
			check := func(orgID string, body map[string]json.RawMessage, detail bool) {
				t.Helper()
				wantPending := tc.visible[orgID]
				pending, hasPending := body["pendingInvitations"]
				if hasPending != wantPending {
					t.Errorf("%s detail=%t: pendingInvitations present = %t, want %t", orgID, detail, hasPending, wantPending)
				}
				if hasPending && wantPending {
					var invitations []models.OrganizationInvitation
					if err := json.Unmarshal(pending, &invitations); err != nil || len(invitations) != 1 ||
						invitations[0].UserID != "invited-"+orgID || invitations[0].Role != models.OrgRoleEditor {
						t.Fatalf("pending invitations for %s are not normalized: %s (%v)", orgID, pending, err)
					}
				}
				capability, hasCapability := body["canManageMembers"]
				if wantCapability := detail && orgID == "default" && wantPending; hasCapability != wantCapability ||
					hasCapability && string(capability) != "true" {
					t.Errorf("%s detail=%t: canManageMembers = %s, want present and true = %t", orgID, detail, capability, wantCapability)
				}
			}

			listJSON := get("/api/orgs", "", h.HandleListOrgs, http.StatusOK)
			var listed []map[string]json.RawMessage
			if err := json.Unmarshal(listJSON, &listed); err != nil || listed == nil || len(listed) != len(tc.visible) {
				t.Fatalf("unexpected list: %s (%v)", listJSON, err)
			}
			seen := make(map[string]bool)
			for _, row := range listed {
				var orgID string
				if err := json.Unmarshal(row["id"], &orgID); err != nil {
					t.Fatal(err)
				}
				if _, visible := tc.visible[orgID]; !visible || seen[orgID] {
					t.Fatalf("unexpected list entry for %q", orgID)
				}
				seen[orgID] = true
				check(orgID, row, false)
			}
			for _, orgID := range []string{"acme", "other", "default"} {
				path := "/api/orgs/" + orgID
				if wantPending, visible := tc.visible[orgID]; visible {
					detail := get(path, orgID, h.HandleGetOrg, http.StatusOK)
					var row map[string]json.RawMessage
					if err := json.Unmarshal(detail, &row); err != nil {
						t.Fatal(err)
					}
					check(orgID, row, true)
					if wantPending {
						managerList := get(path+"/invitations", orgID, h.HandleListInvitations, http.StatusOK)
						assertJSONSnapshot(t, managerList, string(row["pendingInvitations"]))
					} else {
						get(path+"/invitations", orgID, h.HandleListInvitations, http.StatusForbidden)
					}
				} else {
					get(path, orgID, h.HandleGetOrg, http.StatusForbidden)
				}
			}
		})
	}
}

type orgDirectoryLicenseProbe struct {
	lookups int
}

func (p *orgDirectoryLicenseProbe) Service(context.Context) *licenseService {
	p.lookups++
	return newLicenseService()
}

func TestOrgHandlersProviderDirectoryKeepsInvitationGate(t *testing.T) {
	t.Setenv(pkglicensing.ProviderHostedRuntimeEnvVar, "true")
	t.Setenv("PULSE_TENANT_ID", "t-client")
	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(dir, "unused-legacy-key"))
	p := config.NewMultiTenantPersistence(dir)
	if err := p.SaveOrganization(&models.Organization{
		ID: "t-client", DisplayName: "Client", OwnerUserID: "alice",
		PendingInvitations: []models.OrganizationInvitation{{UserID: "bob", Role: models.OrgRoleViewer}},
	}); err != nil {
		t.Fatal(err)
	}
	h := NewOrgHandlers(p, nil)
	h.SetHostedMode(true)
	call := func(path string, handler http.HandlerFunc) *httptest.ResponseRecorder {
		t.Helper()
		req := withUser(httptest.NewRequest(http.MethodGet, path, nil), "alice")
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "t-client"))
		req.SetPathValue("id", "t-client")
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}
	for _, tc := range []struct {
		state  pkglicensing.SubscriptionState
		status int
	}{
		{state: pkglicensing.SubStateExpired, status: http.StatusPaymentRequired},
		{state: pkglicensing.SubStateActive, status: http.StatusOK},
	} {
		t.Run(string(tc.state), func(t *testing.T) {
			probe := &orgDirectoryLicenseProbe{}
			SetLicenseServiceProvider(probe)
			t.Cleanup(func() { SetLicenseServiceProvider(nil) })
			rec := call("/api/orgs", h.HandleListOrgs)
			if probe.lookups != 0 {
				t.Fatalf("workspace directory performed %d paid capability lookups", probe.lookups)
			}
			var listed []struct {
				ID                 string                      `json:"id"`
				DisplayName        string                      `json:"displayName"`
				Members            []models.OrganizationMember `json:"members"`
				PendingInvitations json.RawMessage             `json:"pendingInvitations"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &listed); rec.Code != http.StatusOK || err != nil {
				t.Fatalf("workspace directory: %d %s (%v)", rec.Code, rec.Body.Bytes(), err)
			}
			found := false
			for _, row := range listed {
				if row.ID != "t-client" {
					continue
				}
				found = true
				if row.DisplayName != "Client" || len(row.Members) != 1 || row.Members[0].UserID != "alice" ||
					row.Members[0].Role != models.OrgRoleOwner {
					t.Fatalf("workspace identity or current member changed: %#v", row)
				}
				if row.PendingInvitations != nil {
					t.Fatalf("workspace directory returned management data without using the gate: %s", row.PendingInvitations)
				}
			}
			if !found {
				t.Fatal("provider client could not see its workspace")
			}
			setupHostedLicenseProvider(t, tc.state, nil)
			if rec := call("/api/orgs/t-client/invitations", h.HandleListInvitations); rec.Code != tc.status {
				t.Fatalf("invitation list: %d, want %d", rec.Code, tc.status)
			}
			rec = call("/api/orgs/t-client", h.HandleGetOrg)
			if rec.Code != tc.status {
				t.Fatalf("workspace detail: %d, want %d", rec.Code, tc.status)
			}
			if tc.status == http.StatusOK {
				var detail map[string]json.RawMessage
				if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
					t.Fatal(err)
				}
				if detail["pendingInvitations"] == nil {
					t.Fatal("gated manager detail lost pending invitations")
				}
			}
		})
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
		{name: "docker swarm node supported", in: "docker-swarm-node", want: false},
		{name: "docker secret supported", in: "docker-secret", want: false},
		{name: "docker config supported", in: "docker-config", want: false},
		{name: "kubernetes namespace supported", in: "k8s-namespace", want: false},
		{name: "kubernetes service supported", in: "k8s-service", want: false},
		{name: "kubernetes replicaset supported", in: "k8s-replicaset", want: false},
		{name: "kubernetes statefulset supported", in: "k8s-statefulset", want: false},
		{name: "kubernetes daemonset supported", in: "k8s-daemonset", want: false},
		{name: "kubernetes job supported", in: "k8s-job", want: false},
		{name: "kubernetes cronjob supported", in: "k8s-cronjob", want: false},
		{name: "kubernetes ingress supported", in: "k8s-ingress", want: false},
		{name: "kubernetes endpoint slice supported", in: "k8s-endpoint-slice", want: false},
		{name: "kubernetes network policy supported", in: "k8s-network-policy", want: false},
		{name: "kubernetes persistent volume supported", in: "k8s-persistent-volume", want: false},
		{name: "kubernetes persistent volume claim supported", in: "k8s-persistent-volume-claim", want: false},
		{name: "kubernetes storage class supported", in: "k8s-storage-class", want: false},
		{name: "kubernetes configmap supported", in: "k8s-configmap", want: false},
		{name: "kubernetes secret supported", in: "k8s-secret", want: false},
		{name: "kubernetes serviceaccount supported", in: "k8s-serviceaccount", want: false},
		{name: "kubernetes role supported", in: "k8s-role", want: false},
		{name: "kubernetes cluster role supported", in: "k8s-cluster-role", want: false},
		{name: "kubernetes role binding supported", in: "k8s-role-binding", want: false},
		{name: "kubernetes cluster role binding supported", in: "k8s-cluster-role-binding", want: false},
		{name: "kubernetes resource quota supported", in: "k8s-resource-quota", want: false},
		{name: "kubernetes limit range supported", in: "k8s-limit-range", want: false},
		{name: "kubernetes pod disruption budget supported", in: "k8s-pod-disruption-budget", want: false},
		{name: "kubernetes horizontal pod autoscaler supported", in: "k8s-horizontal-pod-autoscaler", want: false},
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

// An organization with no pending invitations lists none as an empty array;
// the access panel refuses a list that is not an array.
func TestHandleListInvitationsListsNoneAsEmptyArray(t *testing.T) {
	previous := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(previous) })
	t.Setenv("PULSE_DEV", "true")

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	if err := persistence.SaveOrganization(&models.Organization{
		ID: "quiet", DisplayName: "Quiet", OwnerUserID: "alice",
		Members: []models.OrganizationMember{{UserID: "alice", Role: models.OrgRoleOwner}},
	}); err != nil {
		t.Fatalf("save organization: %v", err)
	}
	h := NewOrgHandlers(persistence, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/orgs/quiet/invitations", nil)
	req = req.WithContext(internalauth.WithUser(req.Context(), "alice"))
	req.SetPathValue("id", "quiet")
	rec := httptest.NewRecorder()
	h.HandleListInvitations(rec, req)

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("listing invitations: %d %q, want an empty array", rec.Code, rec.Body.String())
	}
}

// An organization write is one complete decision, not the first JSON value
// a reader happens to deliver. Keep this table at the actual handlers so each
// route (including the separate default-member branch) uses the same boundary.
type orgWriteBodyRoute struct {
	name, method, path, orgID, user, body, field string
	status                                       int
	handler                                      http.HandlerFunc
}

func orgWriteBodyFixture(t *testing.T) (*config.MultiTenantPersistence, *OrgHandlers, []orgWriteBodyRoute) {
	t.Helper()
	enableSharedMultiOrgForTest(t)
	t.Setenv("PULSE_DEV", "true")
	persistence, checker := newOrgAccessFixture(t, false)
	saveDefaultOrgMembers(t, persistence, "default-reader")
	if err := persistence.SaveOrganization(&models.Organization{ID: "beta", DisplayName: "Beta", OwnerUserID: "blair"}); err != nil {
		t.Fatal(err)
	}
	org, err := persistence.LoadOrganizationStrict("acme")
	if err != nil {
		t.Fatal(err)
	}
	org.SharedResources = []models.OrganizationShare{{ID: "retained-share", TargetOrgID: "beta", ResourceType: "vm", ResourceID: "vm-1", ResourceName: "Backup VM", AccessRole: models.OrgRoleViewer, Status: models.OrganizationShareStatusAccepted, AcceptedBy: "blair"}}
	if err := persistence.SaveOrganization(org); err != nil {
		t.Fatal(err)
	}
	h := NewOrgHandlers(persistence, nil)
	h.SetAccessPolicy(checker)
	return persistence, h, []orgWriteBodyRoute{
		{"create", http.MethodPost, "/api/orgs", "", "alice", `{"id":"new-org","displayName":"New organization"}`, "id", http.StatusCreated, h.HandleCreateOrg},
		{"rename", http.MethodPut, "/api/orgs/acme", "acme", "alice", `{"displayName":"Renamed organization"}`, "displayName", http.StatusOK, h.HandleUpdateOrg},
		{"invitation", http.MethodPost, "/api/orgs/acme/members", "acme", "alice", `{"userId":"carol","role":"editor"}`, "role", http.StatusAccepted, h.HandleInviteMember},
		{"member-role", http.MethodPost, "/api/orgs/acme/members", "acme", "alice", `{"userId":"bob","role":"editor"}`, "userId", http.StatusOK, h.HandleInviteMember},
		{"share-create", http.MethodPost, "/api/orgs/acme/shares", "acme", "alice", `{"targetOrgId":"beta","resourceType":"vm","resourceId":"vm-2","accessRole":"editor"}`, "resourceId", http.StatusCreated, h.HandleCreateShare},
		{"share-update", http.MethodPost, "/api/orgs/acme/shares", "acme", "alice", `{"targetOrgId":"beta","resourceType":"vm","resourceId":"vm-1","accessRole":"editor"}`, "accessRole", http.StatusOK, h.HandleCreateShare},
		{"default-member", http.MethodPost, "/api/orgs/default/members", "default", orgAccessTestAdmin, `{"userId":"new-reader","role":"viewer"}`, "userId", http.StatusOK, h.HandleInviteMember},
	}
}

func orgWriteBodyRequest(route orgWriteBodyRoute, body io.Reader) *http.Request {
	req := withUser(httptest.NewRequest(route.method, route.path, body), route.user)
	if route.orgID != "" {
		req.SetPathValue("id", route.orgID)
	}
	return req
}

func orgWriteSavedFiles(t *testing.T, base string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	if err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			files[relative+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(path)
		if err == nil {
			files[relative] = string(data)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

type orgWriteBodyReadFailure struct{ io.Reader }

func (b orgWriteBodyReadFailure) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if err == io.EOF {
		return n, errors.New("private reader detail must not reach the response")
	}
	return n, err
}

func TestOrgHandlersWriteBodyRejectsIncompleteOrCompetingDecisions(t *testing.T) {
	for routeIndex := 0; routeIndex < 7; routeIndex++ {
		_, _, routes := orgWriteBodyFixture(t)
		route := routes[routeIndex]
		t.Run(route.name, func(t *testing.T) {
			for _, name := range []string{"second-object", "junk-tail", "overflow-whitespace", "late-read-failure", "empty", "null", "array", "truncated", "wrong-type", "duplicate", "escaped-duplicate", "folded-duplicate"} {
				t.Run(name, func(t *testing.T) {
					persistence, _, routes := orgWriteBodyFixture(t)
					route := routes[routeIndex]
					before := orgWriteSavedFiles(t, persistence.BaseDataDir())
					body := route.body
					switch name {
					case "second-object":
						body += `{}`
					case "junk-tail":
						body += `private-unread-tail`
					case "overflow-whitespace":
						body += strings.Repeat(" ", orgRequestBodyLimit-len(body)+1)
					case "empty":
						body = " \n\t"
					case "null":
						body = "null"
					case "array":
						body = "[]"
					case "truncated":
						body = body[:len(body)-1]
					case "wrong-type":
						var fields map[string]any
						if err := json.Unmarshal([]byte(body), &fields); err != nil {
							t.Fatal(err)
						}
						fields[route.field] = true
						raw, err := json.Marshal(fields)
						if err != nil {
							t.Fatal(err)
						}
						body = string(raw)
					case "duplicate", "escaped-duplicate", "folded-duplicate":
						field := route.field
						if name == "escaped-duplicate" {
							field = fmt.Sprintf(`\u%04x%s`, field[0], field[1:])
						} else if name == "folded-duplicate" {
							field = strings.ToUpper(field)
						}
						body = body[:len(body)-1] + `,"` + field + `":"competing-value"}`
					}
					var reader io.Reader = strings.NewReader(body)
					if name == "late-read-failure" {
						reader = orgWriteBodyReadFailure{reader}
					}
					rec := httptest.NewRecorder()
					route.handler(rec, orgWriteBodyRequest(route, reader))
					if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"invalid_request"`) {
						t.Errorf("invalid body = %d %s, want generic 400 invalid_request", rec.Code, rec.Body)
					}
					if strings.Contains(rec.Body.String(), "private") || strings.Contains(rec.Body.String(), "competing-value") {
						t.Error("request or reader detail escaped in error response")
					}
					if !reflect.DeepEqual(before, orgWriteSavedFiles(t, persistence.BaseDataDir())) {
						t.Error("rejected decision changed saved organization files or directories")
					}
				})
			}
		})
	}
}

func TestOrgHandlersWriteBodyRetainsUnambiguousDecisions(t *testing.T) {
	for routeIndex := 0; routeIndex < 7; routeIndex++ {
		for _, name := range []string{"ordinary", "singleton-alias", "unknown-fields", "exact-limit", "default-role"} {
			persistence, _, routes := orgWriteBodyFixture(t)
			route := routes[routeIndex]
			t.Run(route.name+"/"+name, func(t *testing.T) {
				body := route.body
				switch name {
				case "singleton-alias":
					body = strings.Replace(body, `"`+route.field+`":`, `"`+strings.ToUpper(route.field)+`":`, 1)
				case "unknown-fields":
					body = body[:len(body)-1] + `,"future":{"role":"ignored","ROLE":false}}`
				case "exact-limit":
					body += strings.Repeat(" ", orgRequestBodyLimit-len(body))
				case "default-role":
					body = strings.ReplaceAll(body, `,"role":"editor"`, "")
					body = strings.ReplaceAll(body, `,"role":"viewer"`, "")
					body = strings.ReplaceAll(body, `,"accessRole":"editor"`, "")
				}
				peerBefore := orgWriteSavedFiles(t, filepath.Join(persistence.BaseDataDir(), "orgs", "beta"))
				rec := httptest.NewRecorder()
				route.handler(rec, orgWriteBodyRequest(route, strings.NewReader(body)))
				if rec.Code != route.status {
					t.Fatalf("valid body = %d %s, want %d", rec.Code, rec.Body, route.status)
				}
				if !reflect.DeepEqual(peerBefore, orgWriteSavedFiles(t, filepath.Join(persistence.BaseDataDir(), "orgs", "beta"))) {
					t.Fatal("valid write changed the peer organization")
				}
				orgID := route.orgID
				if orgID == "" {
					orgID = "new-org"
				}
				org, err := persistence.LoadOrganizationStrict(orgID)
				if err != nil {
					t.Fatal(err)
				}
				wantRole := models.OrgRoleEditor
				if name == "default-role" {
					wantRole = models.OrgRoleViewer
				}
				switch route.name {
				case "create", "rename":
					want := "New organization"
					if route.name == "rename" {
						want = "Renamed organization"
					}
					if org.DisplayName != want || org.OwnerUserID != "alice" || !org.HasMemberUserID("alice") {
						t.Fatalf("saved organization = %+v", org)
					}
				case "invitation":
					if org.HasMemberUserID("carol") || len(org.PendingInvitations) != 1 || org.PendingInvitations[0].Role != wantRole {
						t.Fatalf("invitation did not remain pending editor: %+v", org)
					}
				case "member-role":
					if organizationRoleForUser(org, "bob") != wantRole || org.OwnerUserID != "alice" {
						t.Fatalf("saved role or owner = %+v", org)
					}
				case "share-create", "share-update":
					share := org.SharedResources[len(org.SharedResources)-1]
					if share.TargetOrgID != "beta" || share.AccessRole != wantRole || (share.Status != models.OrganizationShareStatusPending && !(route.name == "share-update" && name == "default-role" && share.Status == models.OrganizationShareStatusAccepted)) {
						t.Fatalf("saved share = %+v", share)
					}
				case "default-member":
					if organizationRoleForUser(org, "new-reader") != models.OrgRoleViewer || !org.HasMemberUserID("default-reader") {
						t.Fatalf("saved default membership = %+v", org)
					}
				}
			})
		}
	}
}

type orgWriteUnreadBody struct{ reads int }

func (b *orgWriteUnreadBody) Read([]byte) (int, error) {
	b.reads++
	return 0, errors.New("body should not be read before authority checks")
}

func TestOrgHandlersWriteBodyDoesNotReplaceAuthorityChecks(t *testing.T) {
	for routeIndex := 0; routeIndex < 7; routeIndex++ {
		persistence, _, routes := orgWriteBodyFixture(t)
		route := routes[routeIndex]
		t.Run(route.name, func(t *testing.T) {
			before := orgWriteSavedFiles(t, persistence.BaseDataDir())
			route.user = "token:synthetic-reader"
			body := &orgWriteUnreadBody{}
			rec := httptest.NewRecorder()
			route.handler(rec, orgWriteBodyRequest(route, body))
			if rec.Code != http.StatusForbidden || body.reads != 0 {
				t.Fatalf("token principal reached body: status=%d reads=%d", rec.Code, body.reads)
			}
			if !reflect.DeepEqual(before, orgWriteSavedFiles(t, persistence.BaseDataDir())) {
				t.Fatal("refused principal changed saved organization state")
			}
		})
	}
}

func TestOrgHandlersWriteBodyNativeHTTP(t *testing.T) {
	for routeIndex := 0; routeIndex < 7; routeIndex++ {
		for _, invalid := range []bool{true, false} {
			persistence, _, routes := orgWriteBodyFixture(t)
			route := routes[routeIndex]
			t.Run(fmt.Sprintf("%s/invalid=%t", route.name, invalid), func(t *testing.T) {
				before := orgWriteSavedFiles(t, persistence.BaseDataDir())
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					r = withUser(r, route.user)
					if route.orgID != "" {
						r.SetPathValue("id", route.orgID)
					}
					route.handler(w, r)
				}))
				defer server.Close()
				body := route.body
				if invalid {
					body += `{}`
				}
				req, err := http.NewRequest(route.method, server.URL+route.path, strings.NewReader(body))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Content-Type", "application/json")
				response, err := server.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				raw, err := io.ReadAll(response.Body)
				if err != nil {
					t.Fatal(err)
				}
				want := route.status
				if invalid {
					want = http.StatusBadRequest
				}
				if response.StatusCode != want {
					t.Errorf("HTTP write = %d %s, want %d", response.StatusCode, raw, want)
				}
				unchanged := reflect.DeepEqual(before, orgWriteSavedFiles(t, persistence.BaseDataDir()))
				if unchanged != invalid {
					t.Errorf("HTTP write saved-state unchanged = %t, invalid = %t", unchanged, invalid)
				}
			})
		}
	}
}
