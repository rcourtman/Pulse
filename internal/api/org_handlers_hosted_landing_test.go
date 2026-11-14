package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// A provider opening a client's address in a new browser session carries the
// handoff's session but neither the organization cookie nor the tab's stored
// selection. The client runtime must still open the client's workspace, not
// its empty default organization.
func TestHostedClientRuntimeRevisitLandsInItsWorkspace(t *testing.T) {
	installTestRBACManager(t)
	enableSharedMultiOrgForTest(t)
	t.Setenv("PULSE_TENANT_ID", "acme")
	t.Setenv(pkglicensing.ProviderHostedRuntimeEnvVar, "true")
	persistence, checker := newOrgAccessFixture(t, true)
	mw := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{Persistence: persistence, AuthChecker: checker, HostedMode: true})

	servedOrg := func(req *http.Request) (int, string) {
		t.Helper()
		org := ""
		rec := httptest.NewRecorder()
		mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			org = GetOrgID(r.Context())
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(rec, req)
		return rec.Code, org
	}

	for _, path := range []string{"/", "/api/state/summary", "/api/license/runtime-capabilities", "/api/orgs", "/ws"} {
		code, org := servedOrg(orgAccessTestRequest(t, http.MethodGet, path, "bob", nil))
		if code != http.StatusNoContent || org != "acme" {
			t.Fatalf("revisit %s: status %d org %q, want the client workspace acme", path, code, org)
		}
	}

	// An explicit selection still wins.
	code, org := servedOrg(orgAccessTestRequest(t, http.MethodGet, "/api/state/summary", "bob", func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: CookieNameOrgID, Value: "default"})
	}))
	if code != http.StatusNoContent || org != "default" {
		t.Fatalf("explicit default selection: status %d org %q", code, org)
	}
	// Someone outside the workspace and API tokens keep the default organization.
	if _, org := servedOrg(orgAccessTestRequest(t, http.MethodGet, "/api/state/summary", "carol", nil)); org != "default" {
		t.Fatalf("non-member landed in %q, want default", org)
	}
	tokenReq := orgAccessTestRequest(t, http.MethodGet, "/api/state/summary", "", func(r *http.Request) {
		*r = *r.WithContext(internalauth.WithAPIToken(r.Context(), &config.APITokenRecord{ID: "tok"}))
	})
	if _, org := servedOrg(tokenReq); org != "default" {
		t.Fatalf("unbound token landed in %q, want default", org)
	}

	// The organization list names the workspace first, and an unlicensed
	// provider client may read it: its workspace is open without a lease.
	// The app reads the list with the default organization named.
	h := NewOrgHandlers(persistence, nil)
	h.SetHostedMode(true)
	h.SetAccessPolicy(checker)
	listOrgs := func() (int, string) {
		t.Helper()
		rec := httptest.NewRecorder()
		req := orgAccessTestRequest(t, http.MethodGet, "/api/orgs", "bob", func(r *http.Request) {
			r.Header.Set("X-Pulse-Org-ID", "default")
		})
		mw.Middleware(http.HandlerFunc(h.HandleListOrgs)).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			return rec.Code, rec.Body.String()
		}
		var orgs []models.Organization
		if err := json.Unmarshal(rec.Body.Bytes(), &orgs); err != nil {
			t.Fatalf("decode orgs: %v", err)
		}
		ids := make([]string, 0, len(orgs))
		for _, org := range orgs {
			ids = append(ids, org.ID)
		}
		return rec.Code, strings.Join(ids, ",")
	}
	if code, ids := listOrgs(); code != http.StatusOK || ids != "acme,default" {
		t.Fatalf("org list: status %d ids %q, want acme first", code, ids)
	}

	// A Pulse Cloud runtime without a subscription stays closed.
	t.Setenv(pkglicensing.ProviderHostedRuntimeEnvVar, "")
	if code, body := listOrgs(); code != http.StatusPaymentRequired {
		t.Fatalf("unsubscribed cloud runtime org list: status %d %s, want 402", code, body)
	}
}
