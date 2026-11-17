package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Return complete valid input while ending the request. A buffered body does
// not have to report a read error when the client has already gone away.
type adminResetCancelReader struct {
	io.Reader
	cancel context.CancelFunc
	reads  int
}

func (r *adminResetCancelReader) Read(p []byte) (int, error) {
	r.reads++
	n, err := r.Reader.Read(p)
	r.cancel()
	return n, err
}

type adminResetCancelOrgContext struct {
	context.Context
	cancel context.CancelFunc
}

func (c adminResetCancelOrgContext) Value(key any) any {
	if key == OrgIDContextKey {
		c.cancel()
	}
	return c.Context.Value(key)
}

type adminResetLifetimeFixture struct {
	provider *TenantRBACProvider
	store    *RecoveryTokenStore
	org      string
	token    string
}

func newAdminResetLifetimeFixture(t *testing.T, org string) adminResetLifetimeFixture {
	t.Helper()
	dir := t.TempDir()
	provider := NewTenantRBACProvider(dir)
	t.Cleanup(func() { _ = provider.Close() })
	// Exercise the existing public role APIs, not a new provider or storage seam.
	for _, id := range []string{org, "peer-org"} {
		if id != "default" {
			if err := os.MkdirAll(filepath.Join(dir, "orgs", id), 0700); err != nil {
				t.Fatal(err)
			}
		}
		manager, err := provider.GetManager(id)
		if err != nil {
			t.Fatal(err)
		}
		if err := manager.UpdateUserRoles("intended", []string{auth.RoleViewer}); err != nil {
			t.Fatal(err)
		}
		if err := manager.UpdateUserRoles("neighbor", []string{auth.RoleOperator}); err != nil {
			t.Fatal(err)
		}
	}
	resetRecoveryStore()
	t.Cleanup(resetRecoveryStore)
	InitRecoveryTokenStore(dir)
	store := GetRecoveryTokenStore()
	token, err := store.GenerateRecoveryToken(time.Hour, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	return adminResetLifetimeFixture{provider: provider, store: store, org: org, token: token}
}

func (f adminResetLifetimeFixture) body(t *testing.T) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]string{"org_id": f.org, "username": "intended", "recovery_token": f.token})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func (f adminResetLifetimeFixture) request(body io.Reader, ctx context.Context) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/admin/rbac/reset-admin", body)
	r.RemoteAddr = "127.0.0.1:12345"
	return r.WithContext(context.WithValue(ctx, OrgIDContextKey, f.org))
}

func (f adminResetLifetimeFixture) roles(t *testing.T) map[string]string {
	t.Helper()
	saved := make(map[string]string)
	for _, org := range []string{f.org, "peer-org"} {
		manager, err := f.provider.GetManager(org)
		if err != nil {
			t.Fatal(err)
		}
		errorAware := manager.(auth.ErrorAwareManager)
		roles, err := errorAware.GetRolesWithError()
		if err != nil {
			t.Fatal(err)
		}
		assignments, err := errorAware.GetUserAssignmentsWithError()
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(struct {
			Roles       []auth.Role
			Assignments []auth.UserRoleAssignment
		}{roles, assignments})
		if err != nil {
			t.Fatal(err)
		}
		saved[org] = string(data)
	}
	return saved
}

func TestRBACAdminResetRequestLifetime(t *testing.T) {
	for _, org := range []string{"default", "selected-org"} {
		for _, phase := range []string{"cancelled_at_entry", "deadline_at_entry", "during_body", "during_org_selection"} {
			t.Run(org+"/"+phase, func(t *testing.T) {
				f := newAdminResetLifetimeFixture(t, org)
				h := NewRBACHandlers(nil, f.provider)
				beforeTokens := recoveryPersistenceBytes(t, f.store)
				beforeRoles := f.roles(t)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				var requestCtx context.Context = ctx
				if phase == "cancelled_at_entry" {
					cancel()
				} else if phase == "deadline_at_entry" {
					var stop context.CancelFunc
					requestCtx, stop = context.WithDeadline(ctx, time.Now().Add(-time.Hour))
					defer stop()
				}
				reader := &adminResetCancelReader{Reader: bytes.NewReader(f.body(t)), cancel: func() {}}
				if phase == "during_body" {
					reader.cancel = cancel
				}
				r := f.request(reader, requestCtx)
				if phase == "during_org_selection" {
					r = r.WithContext(adminResetCancelOrgContext{Context: r.Context(), cancel: cancel})
				}
				w := httptest.NewRecorder()
				h.HandleRBACAdminReset(w, r)
				var result struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if r.Context().Err() == nil || w.Code != http.StatusRequestTimeout || result.Code != "request_cancelled" {
					t.Fatalf("ended reset = %d, code %q, context %v", w.Code, result.Code, r.Context().Err())
				}
				for _, private := range []string{f.token, "intended", "peer-org"} {
					if bytes.Contains(w.Body.Bytes(), []byte(private)) {
						t.Fatal("cancellation response disclosed recovery input")
					}
				}
				if (phase == "cancelled_at_entry" || phase == "deadline_at_entry") && reader.reads != 0 {
					t.Fatal("already-ended request consumed its body")
				}
				if !bytes.Equal(beforeTokens, recoveryPersistenceBytes(t, f.store)) || !f.store.IsRecoveryTokenValidConstantTime(f.token, "127.0.0.1") || !reloadRecoveryPersistence(f.store).IsRecoveryTokenValidConstantTime(f.token, "127.0.0.1") {
					t.Fatal("ended reset consumed or changed saved recovery authority")
				}
				assertAdminResetRolesEqual(t, beforeRoles, f.roles(t))
				if err := f.provider.Close(); err != nil {
					t.Fatal(err)
				}
				assertAdminResetRolesEqual(t, beforeRoles, f.roles(t))

				// The same token still admits one deliberate live recovery. Only
				// the exact selected principal changes, also after reopening.
				w = httptest.NewRecorder()
				h.HandleRBACAdminReset(w, f.request(bytes.NewReader(f.body(t)), context.Background()))
				if w.Code != http.StatusOK {
					t.Fatalf("live recovery = %d", w.Code)
				}
				assertAdminResetSelectedRecovery(t, f, beforeRoles)
				if err := f.provider.Close(); err != nil {
					t.Fatal(err)
				}
				assertAdminResetSelectedRecovery(t, f, beforeRoles)
				after := f.roles(t)
				w = httptest.NewRecorder()
				h.HandleRBACAdminReset(w, f.request(bytes.NewReader(f.body(t)), context.Background()))
				if w.Code != http.StatusForbidden || reloadRecoveryPersistence(f.store).IsRecoveryTokenValidConstantTime(f.token, "127.0.0.1") {
					t.Fatal("admitted recovery made its consumed token reusable")
				}
				assertAdminResetRolesEqual(t, after, f.roles(t))
			})
		}
	}
}

func assertAdminResetRolesEqual(t *testing.T, want, got map[string]string) {
	t.Helper()
	for org, snapshot := range want {
		if got[org] != snapshot {
			t.Fatal("rejected recovery changed selected or peer roles")
		}
	}
}

func assertAdminResetSelectedRecovery(t *testing.T, f adminResetLifetimeFixture, before map[string]string) {
	t.Helper()
	if f.roles(t)["peer-org"] != before["peer-org"] {
		t.Fatal("selected recovery changed peer roles")
	}
	manager, err := f.provider.GetManager(f.org)
	if err != nil {
		t.Fatal(err)
	}
	for user, role := range map[string]string{"intended": auth.RoleAdmin, "neighbor": auth.RoleOperator} {
		assignment, ok := manager.GetUserAssignment(user)
		if !ok || len(assignment.RoleIDs) != 1 || assignment.RoleIDs[0] != role {
			t.Fatal("live recovery did not preserve exact selected/neighbor assignments")
		}
	}
}

func TestRBACAdminResetCancellationAfterAdmission(t *testing.T) {
	f := newAdminResetLifetimeFixture(t, "default")
	before := f.roles(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := f.request(bytes.NewReader(f.body(t)), ctx)
	w := httptest.NewRecorder()
	h := NewRBACHandlers(nil, f.provider)
	// Hold role-provider admission, not the token store: cancellation is
	// delivered only after consumption has completed and before roles change.
	f.provider.mu.Lock()
	locked := true
	defer func() {
		if locked {
			f.provider.mu.Unlock()
		}
	}()
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.HandleRBACAdminReset(w, r)
	}()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	consumed := false
	for !consumed {
		f.store.mu.RLock()
		consumed = f.store.tokens[recoveryTokenHash(f.token)].Used
		f.store.mu.RUnlock()
		if consumed {
			break
		}
		select {
		case <-deadline.C:
			// Unblock and drain the owned request before failing the test.
			f.provider.mu.Unlock()
			locked = false
			<-done
			t.Fatal("recovery did not reach token consumption")
		case <-done:
			t.Fatal("recovery returned before role-provider admission")
		case <-tick.C:
		}
	}
	cancel()
	f.provider.mu.Unlock()
	locked = false
	<-done
	if w.Code != http.StatusOK || r.Context().Err() == nil || reloadRecoveryPersistence(f.store).IsRecoveryTokenValidConstantTime(f.token, "127.0.0.1") {
		t.Fatal("cancellation interrupted admitted recovery or restored its token")
	}
	assertAdminResetSelectedRecovery(t, f, before)
	if err := f.provider.Close(); err != nil {
		t.Fatal(err)
	}
	assertAdminResetSelectedRecovery(t, f, before)
}

func TestRBACAdminResetEndedRequestAdmissionOrder(t *testing.T) {
	provider := NewTenantRBACProvider(t.TempDir())
	t.Cleanup(func() { _ = provider.Close() })
	for _, tc := range []struct {
		name     string
		method   string
		provider *TenantRBACProvider
		status   int
	}{
		{"method", http.MethodGet, provider, http.StatusMethodNotAllowed},
		{"unavailable", http.MethodPost, nil, http.StatusNotImplemented},
		{"ended", http.MethodPost, provider, http.StatusRequestTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			reader := &adminResetCancelReader{Reader: bytes.NewReader([]byte(`{}`)), cancel: func() {}}
			r := httptest.NewRequest(tc.method, "/api/admin/rbac/reset-admin", reader).WithContext(ctx)
			w := httptest.NewRecorder()
			NewRBACHandlers(nil, tc.provider).HandleRBACAdminReset(w, r)
			if w.Code != tc.status || reader.reads != 0 || provider.ManagerCount() != 0 {
				t.Fatal("ended request changed method/provider admission, read input or provisioned roles")
			}
		})
	}
}
