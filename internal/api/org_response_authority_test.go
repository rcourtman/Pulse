package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func orgResponseRequest(t *testing.T, checker contextAuthorizationChecker, orgID, user string) *http.Request {
	t.Helper()
	req := orgAccessTestRequest(t, http.MethodGet, "/api/result", user, nil)
	ctx := context.WithValue(req.Context(), OrgIDContextKey, orgID)
	return req.WithContext(withOrgAccessChecker(ctx, checker))
}

func TestOrgResponseAuthorityWithdrawsWithoutStreamCache(t *testing.T) {
	for _, orgID := range []string{"default", "acme"} {
		for _, principal := range []string{"session", "proxy", "user"} {
			for _, media := range []struct {
				name   string
				policy responseMediaPolicy
			}{{"json", responseMediaJSON}, {"zip", responseMediaZIP}, {"report", responseMediaReport}, {"audit", responseMediaAudit}} {
				t.Run(orgID+"/"+principal+"/"+media.name, func(t *testing.T) {
					installTestRBACManager(t)
					enableSharedMultiOrgForTest(t)
					persistence, checker := newOrgAccessFixture(t, false)
					const user = "response-viewer"
					// newOrgAccessFixture already seeds acme. Replacing that safe
					// fixture is an intentional edit of its current snapshot, not
					// authority to overwrite unknown or superseded metadata.
					current, err := persistence.LoadOrganization(orgID)
					if err != nil {
						t.Fatal(err)
					}
					org := &models.Organization{PersistenceVersion: current.PersistenceVersion, ID: orgID, DisplayName: orgID, Members: []models.OrganizationMember{{UserID: user, Role: models.OrgRoleViewer}}}
					if err := persistence.SaveOrganization(org); err != nil {
						t.Fatal(err)
					}
					req := orgResponseRequest(t, checker, orgID, user)
					if principal == "session" {
						InitSessionStore(t.TempDir())
						const raw = "synthetic-org-response-session"
						GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", user)
						attachSessionReadAdmission(req, raw, GetSessionStore().GetSession(raw))
					} else if principal == "proxy" {
						req = attachProxyPrincipal(req, user, false)
					}
					// A grant made at admission must not be reused even within the normal
					// stream cache window. No sleep or shortened global interval is used.
					writer := httptest.NewRecorder()
					retainResponseAuthority(&config.Config{}, config.ScopeMonitoringRead, false, media.policy, func(w http.ResponseWriter, r *http.Request) {
						if r.Context().Done() != req.Context().Done() {
							t.Fatal("response guard changed work lifetime")
						}
						w.Header().Set("X-Private-Result", "private-result")
						org.Members = nil
						if err := persistence.SaveOrganization(org); err != nil {
							t.Fatal(err)
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						if n, err := io.WriteString(w, `{"private":"result"}`); n != 0 || !errors.Is(err, io.ErrClosedPipe) {
							t.Errorf("withdrawn write = %d %v", n, err)
						}
					})(writer, req)
					if writer.Code != http.StatusForbidden || !strings.Contains(writer.Body.String(), `"error":"access_denied"`) || writer.Header().Get("X-Private-Result") != "" {
						t.Fatalf("withdrawn response = %d %v %s", writer.Code, writer.Header(), writer.Body)
					}
					if principal == "session" && !sessionReadRevalidator(req)() {
						t.Fatal("membership withdrawal destroyed the session")
					}
				})
			}
		}
	}
}

func TestOrgResponseAuthorityWithholdsBodylessAndErrorResults(t *testing.T) {
	for _, mode := range []string{"bodyless", "error", "partial", "unreadable", "before_work"} {
		t.Run(mode, func(t *testing.T) {
			installTestRBACManager(t)
			enableSharedMultiOrgForTest(t)
			persistence, checker := newOrgAccessFixture(t, false)
			saveDefaultOrgMembers(t, persistence, "response-viewer")
			req := orgResponseRequest(t, checker, "default", "response-viewer")
			withdraw := func() { saveDefaultOrgMembers(t, persistence) }
			if mode == "before_work" {
				withdraw()
			}
			writer := httptest.NewRecorder()
			called := false
			retainTokenResponseAuthority(&config.Config{}, config.ScopeMonitoringRead, false, func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.Header().Set("X-Private-Result", "private")
				w.Header().Set("Content-Type", "application/json")
				if mode == "partial" {
					if _, err := io.WriteString(w, `{"admitted":`); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "unreadable" {
					if err := os.WriteFile(filepath.Join(persistence.BaseDataDir(), "org.json"), []byte("{"), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					withdraw()
				}
				switch mode {
				case "error":
					http.Error(w, "private provider detail", http.StatusInternalServerError)
				case "partial":
					if _, err := io.WriteString(w, "private tail"); !errors.Is(err, io.ErrClosedPipe) {
						t.Fatalf("late output = %v", err)
					}
					saveDefaultOrgMembers(t, persistence, "response-viewer")
					if _, err := io.WriteString(w, "regranted tail"); !errors.Is(err, io.ErrClosedPipe) {
						t.Fatal("refused response reopened")
					}
				}
			})(writer, req)
			if mode == "before_work" && called {
				t.Fatal("stale admission began work")
			}
			if mode == "partial" {
				if writer.Code != http.StatusOK || writer.Body.String() != `{"admitted":` {
					t.Fatalf("partial response changed: %d %s", writer.Code, writer.Body)
				}
				return
			}
			if writer.Code != http.StatusForbidden || !strings.Contains(writer.Body.String(), `"error":"access_denied"`) || writer.Header().Get("X-Private-Result") != "" {
				t.Fatalf("withdrawn %s result = %d %v %s", mode, writer.Code, writer.Header(), writer.Body)
			}
		})
	}
}

func TestOrgResponseAuthorityRetainsCredentialAndOrganizationIdentity(t *testing.T) {
	manager := installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	persistence, checker := newOrgAccessFixture(t, false)
	// A user's inherited administrator grant is live policy, not an admission
	// snapshot. The configured local administrator remains a separate control.
	if err := manager.SaveRole(auth.Role{ID: "response-admin", Name: "Response admin", ParentID: auth.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateUserRoles("rbac-viewer", []string{"response-admin"}); err != nil {
		t.Fatal(err)
	}
	req := orgResponseRequest(t, checker, "default", "rbac-viewer")
	writer := httptest.NewRecorder()
	retainTokenResponseAuthority(&config.Config{}, config.ScopeMonitoringRead, false, func(w http.ResponseWriter, _ *http.Request) {
		if err := manager.UpdateUserRoles("rbac-viewer", []string{auth.RoleViewer}); err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, "private")
	})(writer, req)
	if writer.Code != http.StatusForbidden {
		t.Fatalf("withdrawn inherited admin = %d %s", writer.Code, writer.Body)
	}

	// Later request-context/header changes cannot retarget the admitted user or
	// tenant to an authorized administrator/default organization.
	req = orgResponseRequest(t, checker, "acme", "bob")
	writer = httptest.NewRecorder()
	retainTokenResponseAuthority(&config.Config{}, config.ScopeMonitoringRead, false, func(w http.ResponseWriter, r *http.Request) {
		org, err := persistence.LoadOrganizationStrict("acme")
		if err != nil {
			t.Fatal(err)
		}
		org.Members = nil
		org.OwnerUserID = "alice"
		if err := persistence.SaveOrganization(org); err != nil {
			t.Fatal(err)
		}
		ctx := auth.WithUser(r.Context(), orgAccessTestAdmin)
		ctx = context.WithValue(ctx, OrgIDContextKey, "default")
		*r = *r.WithContext(ctx)
		io.WriteString(w, "private")
	})(writer, req)
	if writer.Code != http.StatusForbidden {
		t.Fatalf("retargeted read = %d %s", writer.Code, writer.Body)
	}
}

func TestOrgResponseAuthorityKeepsAuthorizedAndUnboundControls(t *testing.T) {
	installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	persistence, checker := newOrgAccessFixture(t, false)
	saveDefaultOrgMembers(t, persistence, "response-viewer")
	for _, mode := range []string{"member", "admin", "proxy-admin", "no-policy", "anonymous", "token"} {
		t.Run(mode, func(t *testing.T) {
			req := orgResponseRequest(t, checker, "default", "response-viewer")
			cfg := &config.Config{}
			switch mode {
			case "admin":
				req = orgResponseRequest(t, checker, "default", orgAccessTestAdmin)
			case "proxy-admin":
				req = attachProxyPrincipal(orgResponseRequest(t, checker, "default", "proxy-admin"), "proxy-admin", true)
			case "no-policy":
				req = httptest.NewRequest(http.MethodGet, "/api/result", nil)
			case "anonymous":
				req = orgResponseRequest(t, checker, "default", "")
			case "token":
				cfg = newTestConfigWithTokens(t, newTokenRecord(t, scopedReadToken, []string{config.ScopeMonitoringRead}, nil))
				req.Header.Set("X-API-Token", scopedReadToken)
				attachAPITokenRecord(req, &cfg.APITokens[0])
			}
			writer := httptest.NewRecorder()
			retainTokenResponseAuthority(cfg, config.ScopeMonitoringRead, false, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"ok":true}`)
			})(writer, req)
			if writer.Code != http.StatusOK || strings.TrimSpace(writer.Body.String()) != `{"ok":true}` {
				t.Fatalf("control %s changed: %d %s", mode, writer.Code, writer.Body)
			}
		})
	}
}

func TestOrgResponseAuthorityAuthenticationPrecedesMembershipDenial(t *testing.T) {
	for _, beforeWork := range []bool{false, true} {
		t.Run(fmt.Sprint(beforeWork), func(t *testing.T) {
			installTestRBACManager(t)
			enableSharedMultiOrgForTest(t)
			persistence, checker := newOrgAccessFixture(t, false)
			saveDefaultOrgMembers(t, persistence, "viewer")
			InitSessionStore(t.TempDir())
			const raw = "synthetic-dual-withdrawal-session"
			GetSessionStore().CreateSession(raw, time.Hour, "fixture", "127.0.0.1", "viewer")
			req := orgResponseRequest(t, checker, "default", "viewer")
			attachSessionReadAdmission(req, raw, GetSessionStore().GetSession(raw))
			withdraw := func() { GetSessionStore().DeleteSession(raw); saveDefaultOrgMembers(t, persistence) }
			if beforeWork {
				withdraw()
			}
			writer := httptest.NewRecorder()
			called := false
			retainTokenResponseAuthority(&config.Config{}, config.ScopeMonitoringRead, false, func(w http.ResponseWriter, _ *http.Request) {
				called = true
				withdraw()
				io.WriteString(w, "private")
			})(writer, req)
			if writer.Code != http.StatusUnauthorized || writer.Body.String() != "Authentication required\n" || (beforeWork && called) {
				t.Fatalf("real authentication withdrawal = %d %s called=%v", writer.Code, writer.Body, called)
			}
		})
	}
}

func TestOrgResponseAuthorityPreservesUnrelatedMembership(t *testing.T) {
	installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	persistence, checker := newOrgAccessFixture(t, false)
	saveDefaultOrgMembers(t, persistence, "bob")
	saveDefaultOrgMembers(t, persistence)
	// Bob's independent acme membership comes from newOrgAccessFixture. Losing
	// default membership must neither retarget that denied read nor revoke acme.
	for _, orgID := range []string{"default", "acme"} {
		rec := httptest.NewRecorder()
		retainResponseAuthority(&config.Config{}, config.ScopeMonitoringRead, false, responseMediaAudit, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/csv")
			io.WriteString(w, "synthetic authorized audit\n")
		})(rec, orgResponseRequest(t, checker, orgID, "bob"))
		want := http.StatusOK
		if orgID == "default" {
			want = http.StatusForbidden
		}
		if rec.Code != want {
			t.Fatalf("%s audit = %d %s", orgID, rec.Code, rec.Body)
		}
	}
}
