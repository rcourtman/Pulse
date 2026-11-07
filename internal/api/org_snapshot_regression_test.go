package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type orgSnapshotGatedBody struct {
	io.Reader
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *orgSnapshotGatedBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered); <-b.release })
	return b.Reader.Read(p)
}
func (b *orgSnapshotGatedBody) Close() error { return nil }

// Each route has loaded and authorized an organization before decoding this
// request body. A second ordinary handler then withdraws membership. Releasing
// the body must not let the first handler save its obsolete complete snapshot.
func TestOrgMutationSnapshotCannotRestoreWithdrawnMember(t *testing.T) {
	for _, kind := range []string{"rename", "invitation", "share", "lifecycle", "withdrawn-manager"} {
		t.Run(kind, func(t *testing.T) {
			enableSharedMultiOrgForTest(t)
			t.Setenv("PULSE_DEV", "true")
			persistence, checker := newOrgAccessFixture(t, false)
			h := NewOrgHandlers(persistence, nil)
			h.SetAccessPolicy(checker)
			if err := persistence.SaveOrganization(&models.Organization{ID: "beta", DisplayName: "Beta", OwnerUserID: "blair"}); err != nil {
				t.Fatal(err)
			}
			initial, err := persistence.LoadOrganizationStrict("acme")
			if err != nil {
				t.Fatal(err)
			}
			if kind == "withdrawn-manager" {
				initial.Members = append(initial.Members, models.OrganizationMember{UserID: "manager", Role: models.OrgRoleAdmin})
				if err := persistence.SaveOrganization(initial); err != nil {
					t.Fatal(err)
				}
			}
			method, path, payload, user := http.MethodPut, "/api/orgs/acme", `{"displayName":"Acme renamed"}`, "alice"
			handler := h.HandleUpdateOrg
			switch kind {
			case "invitation", "withdrawn-manager":
				method, path, payload = http.MethodPost, "/api/orgs/acme/members", `{"userId":"dan","role":"viewer"}`
				handler = h.HandleInviteMember
			case "share":
				method, path, payload = http.MethodPost, "/api/orgs/acme/shares", `{"targetOrgId":"beta","resourceType":"vm","resourceId":"vm-1","accessRole":"viewer"}`
				handler = h.HandleCreateShare
			case "lifecycle":
				method, path, payload = http.MethodPost, "/api/admin/orgs/acme/suspend", `{"reason":"synthetic hold"}`
				handler = NewOrgLifecycleHandlers(persistence, true).HandleSuspendOrg
			}
			removed := "bob"
			if kind == "withdrawn-manager" {
				user, removed = "manager", "manager"
			}
			gate := &orgSnapshotGatedBody{Reader: strings.NewReader(payload), entered: make(chan struct{}), release: make(chan struct{})}
			var release sync.Once
			unblock := func() { release.Do(func() { close(gate.release) }) }
			defer unblock()
			req := withUser(httptest.NewRequest(method, path, gate), user)
			req.SetPathValue("id", "acme")
			rec := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { defer close(done); handler(rec, req) }()
			select {
			case <-gate.entered:
			case <-done:
				t.Fatalf("mutation completed before body gate: %d %s", rec.Code, rec.Body)
			case <-time.After(5 * time.Second):
				t.Fatal("mutation did not reach body gate")
			}
			remove := withUser(httptest.NewRequest(http.MethodDelete, "/api/orgs/acme/members/"+removed, nil), "alice")
			remove.SetPathValue("id", "acme")
			remove.SetPathValue("userId", removed)
			removeRec := httptest.NewRecorder()
			h.HandleRemoveMember(removeRec, remove)
			if removeRec.Code != http.StatusNoContent {
				t.Fatalf("withdrawal = %d %s", removeRec.Code, removeRec.Body)
			}
			withdrawn, err := persistence.LoadOrganizationStrict("acme")
			if err != nil || withdrawn.HasMemberUserID(removed) {
				t.Fatalf("withdrawal not persisted: %+v %v", withdrawn, err)
			}
			unblock()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("mutation did not finish")
			}
			final, err := persistence.LoadOrganizationStrict("acme")
			if err != nil {
				t.Fatal(err)
			}
			wire := map[string]any{"case": kind, "status": rec.Code, "body": rec.Body.String(), "initial": initial, "withdrawn": withdrawn, "final": final}
			raw, _ := json.Marshal(wire)
			fmt.Printf("ORG_SNAPSHOT_WIRE %s\n", raw)
			if rec.Code != http.StatusConflict {
				t.Errorf("obsolete mutation = %d %s, want 409 conflict", rec.Code, rec.Body)
			}
			if final.HasMemberUserID(removed) || checker.CheckAccess(nil, removed, "acme").Allowed {
				t.Errorf("obsolete %s restored withdrawn membership", kind)
			}
			if !final.HasMemberUserID("alice") || !checker.CheckAccess(nil, "alice", "acme").Allowed {
				t.Error("independent owner lost access")
			}
			if final.DisplayName != withdrawn.DisplayName || len(final.PendingInvitations) != len(withdrawn.PendingInvitations) || len(final.SharedResources) != len(withdrawn.SharedResources) || final.Status != withdrawn.Status {
				t.Errorf("conflicting %s changed saved intent", kind)
			}
		})
	}
}

// Exercise the installed route composition as well as direct persistence: real
// saved browser sessions, CSRF, tenant selection and scope/auth middleware.
func TestOrgMutationSnapshotRegisteredRouteKeepsWithdrawal(t *testing.T) {
	enableSharedMultiOrgForTest(t)
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("PULSE_AUTH_BYPASS", "false")
	setMockModeForTest(t, false)
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser, cfg.AuthPass = "alice", "fixture-nonempty-hash"
	InitSessionStore(cfg.DataPath)
	router := newMultiTenantRouter(t, cfg)
	initial := &models.Organization{ID: "acme", DisplayName: "Acme", OwnerUserID: "alice", Members: []models.OrganizationMember{{UserID: "alice", Role: models.OrgRoleOwner}, {UserID: "bob", Role: models.OrgRoleViewer}}, PendingInvitations: []models.OrganizationInvitation{{UserID: "carol", Role: models.OrgRoleViewer}}}
	if err := router.multiTenant.SaveOrganization(initial); err != nil {
		t.Fatal(err)
	}
	const ownerSession = "synthetic-org-mutation-owner"
	const viewerSession = "synthetic-org-mutation-viewer"
	GetSessionStore().CreateSession(ownerSession, time.Hour, "fixture", "127.0.0.1", "alice")
	GetSessionStore().CreateSession(viewerSession, time.Hour, "fixture", "127.0.0.1", "bob")
	request := func(method, path, session string, body io.Reader) *http.Request {
		r := httptest.NewRequest(method, path, body)
		r.AddCookie(&http.Cookie{Name: cookieNameSession, Value: session})
		r.Header.Set("X-CSRF-Token", generateCSRFToken(session))
		r.Header.Set("X-Pulse-Org-ID", "acme")
		r.Header.Set("X-Requested-With", "XMLHttpRequest")
		return r
	}
	handler := router.Handler()
	gate := &orgSnapshotGatedBody{Reader: strings.NewReader(`{"userId":"dan","role":"viewer"}`), entered: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	unblock := func() { release.Do(func() { close(gate.release) }) }
	defer unblock()
	pending := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(pending, request(http.MethodPost, "/api/orgs/acme/members", ownerSession, gate))
	}()
	select {
	case <-gate.entered:
	case <-done:
		t.Fatalf("registered mutation before gate = %d %s", pending.Code, pending.Body)
	case <-time.After(5 * time.Second):
		t.Fatal("registered mutation did not reach body gate")
	}
	removal := httptest.NewRecorder()
	handler.ServeHTTP(removal, request(http.MethodDelete, "/api/orgs/acme/members/bob", ownerSession, nil))
	if removal.Code != http.StatusNoContent {
		t.Fatalf("registered removal = %d %s", removal.Code, removal.Body)
	}
	withdrawn, err := router.multiTenant.LoadOrganizationStrict("acme")
	if err != nil {
		t.Fatal(err)
	}
	unblock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("registered mutation did not finish")
	}
	final, err := router.multiTenant.LoadOrganizationStrict("acme")
	if err != nil {
		t.Fatal(err)
	}
	wire := map[string]any{"case": "registered-invitation", "status": pending.Code, "body": pending.Body.String(), "initial": initial, "withdrawn": withdrawn, "final": final}
	raw, _ := json.Marshal(wire)
	fmt.Printf("ORG_SNAPSHOT_WIRE %s\n", raw)
	if pending.Code != http.StatusConflict {
		t.Errorf("registered obsolete mutation = %d %s, want conflict", pending.Code, pending.Body)
	}
	viewer := httptest.NewRecorder()
	handler.ServeHTTP(viewer, request(http.MethodGet, "/api/orgs/acme/members", viewerSession, nil))
	if viewer.Code != http.StatusForbidden || GetSessionStore().GetSession(viewerSession) == nil {
		t.Errorf("withdrawn viewer = %d %s, want 403 with valid session", viewer.Code, viewer.Body)
	}
	control := httptest.NewRecorder()
	handler.ServeHTTP(control, request(http.MethodGet, "/api/orgs/acme/members", ownerSession, nil))
	if control.Code != http.StatusOK {
		t.Fatalf("independent owner = %d %s", control.Code, control.Body)
	}
	// A new deliberate invitation and the original viewer's acceptance are the
	// recovery, not a retry/merge of the already-refused operation.
	invite := httptest.NewRecorder()
	handler.ServeHTTP(invite, request(http.MethodPost, "/api/orgs/acme/members", ownerSession, strings.NewReader(`{"userId":"bob","role":"viewer"}`)))
	if invite.Code != http.StatusAccepted {
		t.Errorf("new explicit invitation = %d %s", invite.Code, invite.Body)
	}
	accept := httptest.NewRecorder()
	acceptRequest := request(http.MethodPost, "/api/org-invitations/acme/accept", viewerSession, nil)
	// Invitation acceptance is an existing organization-neutral bootstrap route,
	// not a read of the workspace whose membership was withdrawn.
	acceptRequest.Header.Set("X-Pulse-Org-ID", "default")
	handler.ServeHTTP(accept, acceptRequest)
	if accept.Code != http.StatusOK {
		t.Fatalf("original session acceptance = %d %s", accept.Code, accept.Body)
	}
	restored, err := router.multiTenant.LoadOrganizationStrict("acme")
	if err != nil || !restored.HasMemberUserID("bob") {
		t.Fatalf("explicit recovery = %+v %v", restored, err)
	}
}
