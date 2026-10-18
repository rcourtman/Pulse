package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// The token inventory is global. A token bound to one organization must not
// list, read, rename, delete or rotate (and so obtain the raw value of) a
// token that belongs to another organization. Instance-wide full-access
// tokens keep managing every token.
type orgTokenFixture struct {
	router *Router
	cfg    *config.Config
	// orgA is a full-access token bound to org-a; def is a full-access token
	// in the default organization; orgB belongs to org-b.
	orgA, def, orgB config.APITokenRecord
}

func newOrgTokenFixture(t *testing.T) *orgTokenFixture {
	t.Helper()
	mk := func(raw, name, org string, scopes []string) config.APITokenRecord {
		record, err := config.NewAPITokenRecord(raw, name, scopes)
		if err != nil {
			t.Fatalf("new token record %s: %v", name, err)
		}
		record.OrgID = org
		return *record
	}
	f := &orgTokenFixture{
		orgA: mk("org-a-wildcard-token.12345678", "org-a", "org-a", []string{config.ScopeWildcard}),
		def:  mk("default-wildcard-token.12345678", "default", "default", []string{config.ScopeWildcard}),
		orgB: mk("org-b-scoped-token.12345678", "org-b", "org-b", []string{config.ScopeMonitoringRead}),
	}
	dataDir := t.TempDir()
	f.cfg = &config.Config{DataPath: dataDir, ConfigPath: dataDir, APITokens: []config.APITokenRecord{f.orgA, f.def, f.orgB}}
	f.cfg.SortAPITokens()
	f.router = &Router{config: f.cfg, persistence: config.NewConfigPersistence(dataDir)}
	return f
}

// call runs a token-management handler as the given caller token.
func (f *orgTokenFixture) call(caller *config.APITokenRecord, method, path string, handler func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(`{"name":"renamed"}`))
	attachAPITokenRecord(req, caller)
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func (f *orgTokenFixture) ids() map[string]bool {
	config.Mu.RLock()
	defer config.Mu.RUnlock()
	out := map[string]bool{}
	for _, record := range f.cfg.APITokens {
		out[record.ID] = true
	}
	return out
}

func TestTokenManagementHidesOtherOrganizationsTokensFromBoundCallers(t *testing.T) {
	f := newOrgTokenFixture(t)

	rec := f.call(&f.orgA, http.MethodGet, "/api/security/tokens", f.router.handleListAPITokens)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d (%s)", rec.Code, rec.Body.String())
	}
	var list struct {
		Tokens []struct {
			ID string `json:"id"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Tokens) != 1 || list.Tokens[0].ID != f.orgA.ID {
		t.Fatalf("org-a caller should list only its own organization's tokens, got %+v", list.Tokens)
	}

	for _, target := range []config.APITokenRecord{f.def, f.orgB} {
		rec = f.call(&f.orgA, http.MethodGet, "/api/security/tokens/"+target.ID, f.router.handleGetAPIToken)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET other-org token %s = %d, want 404", target.Name, rec.Code)
		}
	}

	rec = f.call(&f.orgA, http.MethodGet, "/api/security/tokens/"+f.orgA.ID, f.router.handleGetAPIToken)
	if rec.Code != http.StatusOK {
		t.Errorf("GET own-organization token = %d, want 200", rec.Code)
	}
}

func TestTokenManagementRefusesChangesAcrossOrganizations(t *testing.T) {
	f := newOrgTokenFixture(t)
	before := f.ids()

	for _, tc := range []struct {
		name    string
		method  string
		path    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"rename default token", http.MethodPatch, "/api/security/tokens/" + f.def.ID, f.router.handleUpdateAPIToken},
		{"delete default token", http.MethodDelete, "/api/security/tokens/" + f.def.ID, f.router.handleDeleteAPIToken},
		{"rotate default token", http.MethodPost, "/api/security/tokens/" + f.def.ID + "/rotate", f.router.handleRotateAPIToken},
		{"rename org-b token", http.MethodPatch, "/api/security/tokens/" + f.orgB.ID, f.router.handleUpdateAPIToken},
		{"delete org-b token", http.MethodDelete, "/api/security/tokens/" + f.orgB.ID, f.router.handleDeleteAPIToken},
		{"rotate org-b token", http.MethodPost, "/api/security/tokens/" + f.orgB.ID + "/rotate", f.router.handleRotateAPIToken},
	} {
		rec := f.call(&f.orgA, tc.method, tc.path, tc.handler)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s of another organization's token = %d (%s), want 404", tc.name, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), `"token"`) {
			t.Errorf("%s response leaked a credential: %s", tc.name, rec.Body.String())
		}
	}

	after := f.ids()
	if len(after) != len(before) {
		t.Fatalf("token inventory changed: before %v after %v", before, after)
	}
	for id := range before {
		if !after[id] {
			t.Fatalf("token %s disappeared after refused changes", id)
		}
	}
	config.Mu.RLock()
	defer config.Mu.RUnlock()
	for _, record := range f.cfg.APITokens {
		if record.Name == "renamed" {
			t.Fatalf("token %s was renamed across organizations", record.ID)
		}
	}
}

func TestTokenManagementStillWorksWithinAnOrganization(t *testing.T) {
	f := newOrgTokenFixture(t)

	rec := f.call(&f.orgA, http.MethodPost, "/api/security/tokens/"+f.orgA.ID+"/rotate", f.router.handleRotateAPIToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("an org-a caller rotating an org-a token = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	var rotated struct {
		Token  string `json:"token"`
		Record struct {
			ID    string `json:"id"`
			OrgID string `json:"orgId"`
		} `json:"record"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("decode rotation response: %v", err)
	}
	if strings.TrimSpace(rotated.Token) == "" {
		t.Fatalf("rotation returned no credential: %s", rec.Body.String())
	}
	if ids := f.ids(); ids[f.orgA.ID] {
		t.Fatal("the rotated token's old ID is still in the inventory")
	}
	config.Mu.RLock()
	defer config.Mu.RUnlock()
	for _, record := range f.cfg.APITokens {
		if record.Name == f.orgA.Name && record.ID != f.orgA.ID && record.OrgID != "org-a" {
			t.Fatalf("rotated token lost its organization binding: %q", record.OrgID)
		}
	}
}

func TestInstanceWideTokenStillManagesEveryOrganization(t *testing.T) {
	f := newOrgTokenFixture(t)

	rec := f.call(&f.def, http.MethodPost, "/api/security/tokens/"+f.orgB.ID+"/rotate", f.router.handleRotateAPIToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("an instance-wide token rotating an org-b token = %d (%s), want 200", rec.Code, rec.Body.String())
	}
}

func TestCallerTokenMayManageRules(t *testing.T) {
	mk := func(scopes []string, org string, orgs ...string) *config.APITokenRecord {
		record, err := config.NewAPITokenRecord("rule-token-"+org+".12345678", org, scopes)
		if err != nil {
			t.Fatalf("new token record: %v", err)
		}
		record.OrgID = org
		record.OrgIDs = orgs
		return record
	}
	wild := []string{config.ScopeWildcard}
	scoped := []string{config.ScopeSettingsWrite}
	cases := []struct {
		name           string
		caller, target *config.APITokenRecord
		want           bool
	}{
		{"a session caller manages anything", nil, mk(wild, "org-a"), true},
		{"instance-wide manages other orgs", mk(wild, "default"), mk(wild, "org-a"), true},
		{"org-bound wildcard cannot manage the default org", mk(wild, "org-a"), mk(wild, "default"), false},
		{"org-bound wildcard cannot manage another org", mk(wild, "org-a"), mk(scoped, "org-b"), false},
		{"org-bound wildcard manages its own org", mk(wild, "org-a"), mk(scoped, "org-a"), true},
		{"scoped default caller manages default tokens", mk(scoped, "default"), mk(scoped, "default"), true},
		{"scoped default caller cannot reach org-a", mk(scoped, "default"), mk(scoped, "org-a"), false},
		{"multi-org caller covers both of its orgs", mk(scoped, "", "org-a", "org-b"), mk(scoped, "", "org-a", "org-b"), true},
		{"multi-org caller cannot cover a third org", mk(scoped, "", "org-a", "org-b"), mk(scoped, "", "org-a", "org-c"), false},
		{"org list takes precedence over a stale org id", mk(wild, "org-a", "default"), mk(wild, "default"), true},
		{"multi-org wildcard caller cannot touch an instance-wide target", mk(wild, "", "default", "org-a"), mk(wild, "default"), false},
		{"multi-org wildcard caller manages a scoped default-org target", mk(wild, "", "default", "org-a"), mk(scoped, "default"), true},
		{"instance-wide caller manages an instance-wide target", mk(wild, "default"), mk(wild, "default"), true},
		{"a scoped caller is left to the scope checks for an instance-wide target", mk(scoped, "default"), mk(wild, "default"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := callerTokenMayManage(tc.caller, tc.target); got != tc.want {
				t.Fatalf("callerTokenMayManage = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMultiOrganizationCallerCannotReachOrMintInstanceWideTokens(t *testing.T) {
	f := newOrgTokenFixture(t)
	multi, err := config.NewAPITokenRecord("multi-org-wildcard.12345678", "multi", []string{config.ScopeWildcard})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}
	multi.OrgID = ""
	multi.OrgIDs = []string{"default", "org-a"}
	config.Mu.Lock()
	f.cfg.APITokens = append(f.cfg.APITokens, *multi)
	f.cfg.SortAPITokens()
	config.Mu.Unlock()

	// Its organizations are a superset of the default-only wildcard target, but
	// rotating that target would hand it an instance-wide credential.
	rec := f.call(multi, http.MethodPost, "/api/security/tokens/"+f.def.ID+"/rotate", f.router.handleRotateAPIToken)
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), `"token"`) {
		t.Fatalf("multi-org caller rotating an instance-wide token = %d (%s), want 404 without a credential", rec.Code, rec.Body.String())
	}

	inventorySize := func() int {
		config.Mu.RLock()
		defer config.Mu.RUnlock()
		return len(f.cfg.APITokens)
	}
	create := func(caller *config.APITokenRecord, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/security/tokens", strings.NewReader(body))
		attachAPITokenRecord(req, caller)
		rec := httptest.NewRecorder()
		f.router.handleCreateAPIToken(rec, req)
		return rec
	}
	// Omitting scopes means full access; in the default organization that is an
	// instance-wide token.
	before := inventorySize()
	if rec := create(multi, `{"name":"escape"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("multi-org caller minting an instance-wide token = %d (%s), want 403", rec.Code, rec.Body.String())
	}
	if rec := create(&f.orgA, `{"name":"escape"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("org-a caller minting a default-organization full-access token = %d (%s), want 403", rec.Code, rec.Body.String())
	}
	if got := inventorySize(); got != before {
		t.Fatalf("refused creations still changed the inventory: %d tokens, want %d", got, before)
	}
	// A scoped token is not instance-wide, and an instance-wide caller may mint anything.
	if rec := create(multi, `{"name":"scoped","scopes":["monitoring:read"]}`); rec.Code >= 300 {
		t.Fatalf("multi-org caller minting a scoped token = %d (%s), want success", rec.Code, rec.Body.String())
	}
	if rec := create(&f.def, `{"name":"another"}`); rec.Code >= 300 {
		t.Fatalf("instance-wide caller minting a full-access token = %d (%s), want success", rec.Code, rec.Body.String())
	}
}

// A caller that cannot mint an instance-wide token must not be able to
// promote one it can create: edit a scoped default-organization token to full
// access.
func TestMultiOrganizationCallerCannotPromoteATokenToInstanceWide(t *testing.T) {
	f := newOrgTokenFixture(t)
	multi, err := config.NewAPITokenRecord("multi-org-promoter.12345678", "multi", []string{config.ScopeWildcard})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}
	multi.OrgID = ""
	multi.OrgIDs = []string{"default", "org-a"}
	scoped, err := config.NewAPITokenRecord("scoped-default-target.12345678", "scoped", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}
	config.Mu.Lock()
	f.cfg.APITokens = append(f.cfg.APITokens, *multi, *scoped)
	f.cfg.SortAPITokens()
	config.Mu.Unlock()

	patch := func(caller *config.APITokenRecord, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPatch, "/api/security/tokens/"+scoped.ID, strings.NewReader(body))
		attachAPITokenRecord(req, caller)
		rec := httptest.NewRecorder()
		f.router.handleUpdateAPIToken(rec, req)
		return rec
	}
	scopesNow := func() []string {
		config.Mu.RLock()
		defer config.Mu.RUnlock()
		for _, record := range f.cfg.APITokens {
			if record.ID == scoped.ID {
				return record.Scopes
			}
		}
		return nil
	}

	if rec := patch(multi, `{"scopes":["*"]}`); rec.Code != http.StatusForbidden {
		t.Fatalf("multi-org caller promoting a token to full access = %d (%s), want 403", rec.Code, rec.Body.String())
	}
	if got := scopesNow(); len(got) != 1 || got[0] != config.ScopeMonitoringRead {
		t.Fatalf("a refused promotion changed the token's scopes to %v", got)
	}
	// Narrowing or sidegrading stays allowed, as does an instance-wide caller promoting.
	if rec := patch(multi, `{"scopes":["monitoring:write"]}`); rec.Code >= 300 {
		t.Fatalf("multi-org caller changing a token to another scoped set = %d (%s), want success", rec.Code, rec.Body.String())
	}
	if rec := patch(&f.def, `{"scopes":["*"]}`); rec.Code >= 300 {
		t.Fatalf("instance-wide caller promoting a token = %d (%s), want success", rec.Code, rec.Body.String())
	}
	if got := scopesNow(); len(got) != 1 || got[0] != config.ScopeWildcard {
		t.Fatalf("an instance-wide caller's promotion left scopes %v, want full access", got)
	}
}
