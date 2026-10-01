package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Read what the guide actually sends, rather than maintaining a second client.
// Authentication, licensing and CSRF are tested by the existing route tests;
// these documentation tests exercise the real handlers and persistent values.
func adminDocBodies(t *testing.T, name, heading string) [][]byte {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "docs", name+".md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, found := strings.Cut(string(contents), "### "+heading+"\n")
	if !found {
		t.Fatalf("missing %s heading %q", name, heading)
	}
	if next := strings.Index(section, "\n##"); next >= 0 {
		section = section[:next]
	}
	// Both shell heredocs and API schema blocks contain standalone JSON.
	re := regexp.MustCompile("(?s)(?:<<'JSON'\\n|```json\\n)(.*?)(?:\\nJSON|\\n```)")
	var bodies [][]byte
	for _, match := range re.FindAllStringSubmatch(section, -1) {
		if !json.Valid([]byte(match[1])) {
			t.Fatalf("invalid JSON in %s/%s", name, heading)
		}
		bodies = append(bodies, []byte(match[1]))
	}
	if len(bodies) == 0 {
		t.Fatalf("missing request JSON in %s/%s", name, heading)
	}
	return bodies
}

func TestAdminDocsCustomRoleLifecycle(t *testing.T) {
	manager, err := auth.NewFileManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	previous := auth.GetManager()
	auth.SetManager(manager)
	t.Cleanup(func() { auth.SetManager(previous) })
	h := NewRBACHandlers(&config.Config{AuthUser: "alice"})

	// Exact-base guide control: creating/updating the built-in operator fails.
	legacy := httptest.NewRecorder()
	h.HandleRoles(legacy, withUser(httptest.NewRequest(http.MethodPost, "/api/admin/roles",
		strings.NewReader(`{"id":"operator","name":"Operator"}`)), "alice"))
	if legacy.Code != http.StatusInternalServerError {
		t.Fatalf("legacy built-in mutation was not rejected: %d", legacy.Code)
	}

	assignments := adminDocBodies(t, "RBAC", "Setting Roles for a User")
	if len(assignments) != 2 {
		t.Fatalf("want assign and clear examples, got %d", len(assignments))
	}
	steps := []struct {
		method, path string
		body         []byte
		handler      http.HandlerFunc
		status       int
	}{
		{http.MethodPost, "/api/admin/roles", adminDocBodies(t, "RBAC", "Creating a Role")[0], h.HandleRoles, http.StatusOK},
		{http.MethodPut, "/api/admin/roles/alert-manager", adminDocBodies(t, "RBAC", "Updating a Role")[0], h.HandleRoles, http.StatusOK},
		{http.MethodPut, "/api/admin/users/jane/roles", assignments[0], h.HandleUserRoleActions, http.StatusNoContent},
		{http.MethodPut, "/api/admin/users/jane/roles", assignments[1], h.HandleUserRoleActions, http.StatusNoContent},
		{http.MethodDelete, "/api/admin/users/jane", nil, h.HandleUserRoleActions, http.StatusNoContent},
		{http.MethodDelete, "/api/admin/roles/alert-manager", nil, h.HandleRoles, http.StatusNoContent},
	}
	for _, step := range steps {
		rec := httptest.NewRecorder()
		step.handler(rec, withUser(httptest.NewRequest(step.method, step.path, bytes.NewReader(step.body)), "alice"))
		if rec.Code != step.status {
			t.Fatalf("documented %s %s: got %d: %s", step.method, step.path, rec.Code, rec.Body.String())
		}
		if step.method == http.MethodPut && step.path == "/api/admin/roles/alert-manager" {
			role, exists := manager.GetRole("alert-manager")
			if !exists || role.IsBuiltIn || len(role.Permissions) != 4 {
				t.Fatalf("custom role update did not persist its permissions: %+v", role)
			}
		}
		if step.method == http.MethodPut && strings.HasSuffix(step.path, "/jane/roles") {
			assignment, exists := manager.GetUserAssignment("jane")
			var want struct {
				RoleIDs []string `json:"roleIds"`
			}
			if err := json.Unmarshal(step.body, &want); err != nil {
				t.Fatal(err)
			}
			if !exists || strings.Join(assignment.RoleIDs, ",") != strings.Join(want.RoleIDs, ",") {
				t.Fatalf("assignment did not persist the complete role list: %+v", assignment)
			}
		}
	}
	if _, exists := manager.GetRole("alert-manager"); exists {
		t.Fatal("custom role deletion did not persist")
	}
	if role, exists := manager.GetRole("operator"); !exists || !role.IsBuiltIn {
		t.Fatal("examples altered the built-in role")
	}
}

func TestAdminDocsOrganizationSchemasAndAcceptance(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	wasEnabled := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(wasEnabled) })
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)
	call := func(method, path, user, orgID string, body []byte, handler http.HandlerFunc) *httptest.ResponseRecorder {
		req := withUser(httptest.NewRequest(method, path, bytes.NewReader(body)), user)
		req.SetPathValue("id", orgID)
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}
	check := func(rec *httptest.ResponseRecorder, status int) {
		t.Helper()
		if rec.Code != status {
			t.Fatalf("documented request: got %d, want %d: %s", rec.Code, status, rec.Body.String())
		}
	}

	// Exact-base guide controls: wrong creation fields and member PATCH fail.
	check(call(http.MethodPost, "/api/orgs", "alice", "", []byte(`{"name":"Production Datacenter","description":"EU production infrastructure"}`), h.HandleCreateOrg), http.StatusBadRequest)
	check(call(http.MethodPatch, "/api/orgs/production-datacenter/members/user-id", "alice", "production-datacenter", []byte(`{"role":"admin"}`), h.HandleInviteMember), http.StatusMethodNotAllowed)

	creation := adminDocBodies(t, "MULTI_TENANT", "Creating an Organization")[0]
	check(call(http.MethodPost, "/api/orgs", "alice", "", creation, h.HandleCreateOrg), http.StatusCreated)
	check(call(http.MethodPost, "/api/orgs", "bob", "", []byte(`{"id":"other-org-id","displayName":"Other estate"}`), h.HandleCreateOrg), http.StatusCreated)
	members := adminDocBodies(t, "MULTI_TENANT", "Managing Members")
	if len(members) != 2 {
		t.Fatalf("want invitation and existing-member update, got %d", len(members))
	}
	invitation := call(http.MethodPost, "/api/orgs/production-datacenter/members", "alice", "production-datacenter", members[0], h.HandleInviteMember)
	check(invitation, http.StatusAccepted)
	var invited organizationAccessMutationResponse
	if err := json.Unmarshal(invitation.Body.Bytes(), &invited); err != nil || invited.Kind != "invitation" || invited.Member != nil {
		t.Fatalf("new member was not pending: %s", invitation.Body.String())
	}
	acceptInvitationForTest(t, h, "production-datacenter", "user-id")
	check(call(http.MethodPost, "/api/orgs/production-datacenter/members", "alice", "production-datacenter", members[1], h.HandleInviteMember), http.StatusOK)
	org, err := persistence.LoadOrganization("production-datacenter")
	if err != nil || organizationRoleForUser(org, "user-id") != models.OrgRoleAdmin {
		t.Fatalf("documented role update did not persist: %v", err)
	}

	check(call(http.MethodPost, "/api/orgs/production-datacenter/shares", "alice", "production-datacenter", []byte(`{"targetOrgId":"other-org-id","resourceType":"host","resourceId":"resource-id","role":"viewer"}`), h.HandleCreateShare), http.StatusBadRequest)
	shareBody := adminDocBodies(t, "MULTI_TENANT", "Sharing Resources")[0]
	created := call(http.MethodPost, "/api/orgs/production-datacenter/shares", "alice", "production-datacenter", shareBody, h.HandleCreateShare)
	check(created, http.StatusCreated)
	var share models.OrganizationShare
	if err := json.Unmarshal(created.Body.Bytes(), &share); err != nil || share.Status != models.OrganizationShareStatusPending || share.AccessRole != models.OrgRoleViewer {
		t.Fatalf("documented share grant was not pending/viewer: %s", created.Body.String())
	}
	accepted := acceptIncomingShareForTest(t, h, "other-org-id", "bob", share.ID)
	if accepted.Status != models.OrganizationShareStatusAccepted {
		t.Fatal("share acceptance did not persist")
	}
}
