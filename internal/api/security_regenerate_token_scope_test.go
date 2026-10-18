package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Regenerating tokens replaces the whole token inventory, wildcard tokens
// included, and issues a full-access credential. Token creation refuses to
// grant scopes the caller lacks, so regeneration must not be a way around it:
// a token caller needs instance-wide full-access authority.
const regenerateCallerRaw = "regen-scope-token-123.12345678"

func regenerateAs(t *testing.T, caller *config.APITokenRecord, others ...config.APITokenRecord) (*config.Config, *httptest.ResponseRecorder) {
	t.Helper()
	dataDir := t.TempDir()
	cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir}
	if caller != nil {
		cfg.APITokens = append([]config.APITokenRecord{*caller}, others...)
		cfg.SortAPITokens()
	}
	router := &Router{config: cfg, persistence: config.NewConfigPersistence(cfg.DataPath)}

	authLimiter.Reset("127.0.0.1")
	req := newLoopbackRequest(http.MethodPost, "/api/security/regenerate-token", nil)
	if caller != nil {
		req.Header.Set("X-API-Token", regenerateCallerRaw)
	}
	rec := httptest.NewRecorder()
	router.HandleRegenerateAPIToken(rec, req)
	return cfg, rec
}

func newRegenerateCaller(t *testing.T, scopes []string, orgID string) *config.APITokenRecord {
	t.Helper()
	record, err := config.NewAPITokenRecord(regenerateCallerRaw, "caller", scopes)
	if err != nil {
		t.Fatalf("new caller token record: %v", err)
	}
	if orgID != "" {
		record.OrgID = orgID
	}
	return record
}

func assertInventoryUntouched(t *testing.T, cfg *config.Config, wantIDs ...string) {
	t.Helper()
	got := make([]string, 0, len(cfg.APITokens))
	for _, record := range cfg.APITokens {
		got = append(got, record.ID)
	}
	sort.Strings(got)
	want := append([]string(nil), wantIDs...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("token inventory changed: got %v, want %v", got, wantIDs)
	}
}

func TestHandleRegenerateAPITokenRefusesScopedTokenCaller(t *testing.T) {
	caller := newRegenerateCaller(t, []string{config.ScopeSettingsWrite}, "")
	wildcard, err := config.NewAPITokenRecord("other-wildcard-token.12345678", "other", []string{config.ScopeWildcard})
	if err != nil {
		t.Fatalf("new wildcard record: %v", err)
	}

	cfg, rec := regenerateAs(t, caller, *wildcard)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	// Nothing was replaced: the scoped caller cannot wipe a wildcard token.
	assertInventoryUntouched(t, cfg, caller.ID, wildcard.ID)
}

func TestHandleRegenerateAPITokenRefusesOrganizationBoundWildcardCaller(t *testing.T) {
	caller := newRegenerateCaller(t, []string{config.ScopeWildcard}, "org-a")

	cfg, rec := regenerateAs(t, caller)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	assertInventoryUntouched(t, cfg, caller.ID)
}

func TestTokenHasInstanceWideAuthority(t *testing.T) {
	wildcard := []string{config.ScopeWildcard}
	cases := []struct {
		name   string
		scopes []string
		orgID  string
		orgIDs []string
		want   bool
	}{
		{"full access, default org", wildcard, "default", nil, true},
		{"full access, unbound", wildcard, "", nil, true},
		{"full access, default-only org list", wildcard, "", []string{"default"}, true},
		{"org list takes precedence over a stale org id", wildcard, "org-a", []string{"default"}, true},
		{"full access, other org", wildcard, "org-a", nil, false},
		{"full access, other org list", wildcard, "default", []string{"org-a"}, false},
		{"full access, default plus other org", wildcard, "", []string{"default", "org-a"}, false},
		{"scoped, default org", []string{config.ScopeSettingsWrite}, "default", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, err := config.NewAPITokenRecord("instance-wide-"+tc.name+".12345678", tc.name, tc.scopes)
			if err != nil {
				t.Fatalf("new token record: %v", err)
			}
			record.OrgID = tc.orgID
			record.OrgIDs = tc.orgIDs
			if got := tokenHasInstanceWideAuthority(record); got != tc.want {
				t.Fatalf("tokenHasInstanceWideAuthority = %v, want %v", got, tc.want)
			}
		})
	}
	if tokenHasInstanceWideAuthority(nil) {
		t.Fatal("a nil record must not have instance-wide authority")
	}
}

func TestHandleRegenerateAPITokenAllowsInstanceWideWildcardCaller(t *testing.T) {
	caller := newRegenerateCaller(t, []string{config.ScopeWildcard}, "")
	other, err := config.NewAPITokenRecord("other-scoped-token.12345678", "other", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("new scoped record: %v", err)
	}

	cfg, rec := regenerateAs(t, caller, *other)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("regeneration should leave exactly one token, got %d", len(cfg.APITokens))
	}
	if !reflect.DeepEqual(cfg.APITokens[0].Scopes, []string{config.ScopeWildcard}) {
		t.Fatalf("regenerated scopes = %v, want full access", cfg.APITokens[0].Scopes)
	}
}

func TestHandleRegenerateAPITokenWithoutCallerTokenStillIssuesFullAccess(t *testing.T) {
	cfg, rec := regenerateAs(t, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 || !reflect.DeepEqual(cfg.APITokens[0].Scopes, []string{config.ScopeWildcard}) {
		t.Fatalf("a caller without a token should still receive a full-access token: %+v", cfg.APITokens)
	}
}
