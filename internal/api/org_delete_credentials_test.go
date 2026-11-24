package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func newOrgBoundTokenForTest(t *testing.T, id string, age time.Duration, scope, orgID string, orgIDs ...string) config.APITokenRecord {
	t.Helper()
	record, err := config.NewAPITokenRecord(id+"-raw-token-0123456789abcdef0123456789", id, []string{scope})
	if err != nil {
		t.Fatalf("NewAPITokenRecord(%s): %v", id, err)
	}
	record.ID = id
	record.CreatedAt = time.Now().UTC().Add(-age)
	record.OrgID = orgID
	record.OrgIDs = orgIDs
	return *record
}

// An organization is identified by an ID that is free to be claimed again once
// the organization is deleted. Its instance API tokens must not survive into
// whichever organization takes the ID next.
func TestCleanupTenantRevokesOrganizationAPITokens(t *testing.T) {
	dataDir := t.TempDir()
	persistence := config.NewConfigPersistence(dataDir)

	cfg := &config.Config{DataPath: dataDir}
	cfg.APITokens = []config.APITokenRecord{
		newOrgBoundTokenForTest(t, "acme-only", 1*time.Minute, config.ScopeMonitoringRead, "acme"),
		newOrgBoundTokenForTest(t, "acme-only-list", 2*time.Minute, config.ScopeMonitoringRead, "", "acme"),
		newOrgBoundTokenForTest(t, "acme-list-over-globex", 3*time.Minute, config.ScopeMonitoringRead, "globex", "acme"),
		newOrgBoundTokenForTest(t, "acme-and-globex", 4*time.Minute, config.ScopeMonitoringRead, "acme", "acme", "globex"),
		newOrgBoundTokenForTest(t, "acme-and-default", 5*time.Minute, config.ScopeWildcard, "", "acme", "default"),
		newOrgBoundTokenForTest(t, "globex-only", 6*time.Minute, config.ScopeMonitoringRead, "globex"),
		newOrgBoundTokenForTest(t, "default-only", 7*time.Minute, config.ScopeWildcard, "default"),
	}
	cfg.SortAPITokens()
	if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
		t.Fatalf("SaveAPITokens: %v", err)
	}

	router := &Router{config: cfg, persistence: persistence}
	if err := router.CleanupTenant(context.Background(), "acme"); err != nil {
		t.Fatalf("CleanupTenant: %v", err)
	}

	assertInventory := func(source string, tokens []config.APITokenRecord) {
		t.Helper()
		var ids []string
		for i := range tokens {
			ids = append(ids, tokens[i].ID)
			if tokens[i].CanAccessOrg("acme") || tokens[i].OrgID == "acme" {
				t.Errorf("%s: token %q is still bound to the deleted organization", source, tokens[i].ID)
			}
			if tokens[i].IsLegacyToken() {
				t.Errorf("%s: token %q lost its organization binding instead of being revoked", source, tokens[i].ID)
			}
			// Only the token that already was instance-wide may remain so.
			if tokenActsForInstance(&tokens[i]) != (tokens[i].ID == "default-only") {
				t.Errorf("%s: token %q changed whether it acts for the instance", source, tokens[i].ID)
			}
		}
		if want := []string{"acme-and-globex", "globex-only", "default-only"}; !slices.Equal(ids, want) {
			t.Fatalf("%s: tokens = %v, want %v", source, ids, want)
		}
		if !tokens[0].CanAccessOrg("globex") {
			t.Errorf("%s: multi-organization token lost its remaining organization", source)
		}
	}

	assertInventory("live", cfg.APITokens)
	persisted, err := persistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens: %v", err)
	}
	assertInventory("persisted", persisted)
}

func TestRevokeTenantCredentialsKeepsInventoryWhenSaveFails(t *testing.T) {
	dataDir := t.TempDir()
	persistence := config.NewConfigPersistence(dataDir)

	cfg := &config.Config{DataPath: dataDir}
	cfg.APITokens = []config.APITokenRecord{
		newOrgBoundTokenForTest(t, "acme-only", 1*time.Minute, config.ScopeMonitoringRead, "acme"),
		newOrgBoundTokenForTest(t, "globex-only", 2*time.Minute, config.ScopeMonitoringRead, "globex"),
	}
	cfg.SortAPITokens()
	if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
		t.Fatalf("SaveAPITokens: %v", err)
	}

	// A real write refusal, confined to this disposable test directory.
	tokensPath := filepath.Join(dataDir, "api_tokens.json")
	if err := os.Remove(tokensPath); err != nil {
		t.Fatalf("remove token file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tokensPath, "blocked"), 0o700); err != nil {
		t.Fatalf("block token file: %v", err)
	}

	router := &Router{config: cfg, persistence: persistence}
	if err := router.revokeTenantCredentials("acme"); err == nil {
		t.Fatal("expected an error when the token inventory cannot be saved")
	}
	if len(cfg.APITokens) != 2 || !cfg.APITokens[0].CanAccessOrg("acme") {
		t.Fatalf("live inventory changed although the revocation was not persisted: %+v", cfg.APITokens)
	}
}

func TestOrgHandlersDeleteRetainsOrgWhenCredentialRevocationFails(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	persistence := config.NewMultiTenantPersistence(t.TempDir())
	h := NewOrgHandlers(persistence, nil)

	revocationErr := errors.New("token inventory unavailable")
	var order []string
	h.SetCredentialRevoker(func(orgID string) error {
		if !persistence.OrgExists(orgID) {
			t.Errorf("credentials must be revoked before organization %q is removed", orgID)
		}
		order = append(order, "before")
		return revocationErr
	})
	h.SetOnDelete(func(_ context.Context, _ string) error {
		order = append(order, "after")
		return nil
	})

	if err := persistence.SaveOrganization(&models.Organization{ID: "acme", DisplayName: "Acme Corp", OwnerUserID: "alice"}); err != nil {
		t.Fatalf("SaveOrganization: %v", err)
	}

	deleteOrg := func() int {
		req := withUser(httptest.NewRequest(http.MethodDelete, "/api/orgs/acme", nil), "alice")
		req.SetPathValue("id", "acme")
		rec := httptest.NewRecorder()
		h.HandleDeleteOrg(rec, req)
		return rec.Code
	}

	if code := deleteOrg(); code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when credentials cannot be revoked, got %d", code)
	}
	if !persistence.OrgExists("acme") {
		t.Fatal("organization was deleted although its credentials are still valid")
	}
	if !slices.Equal(order, []string{"before"}) {
		t.Fatalf("callbacks = %v, want only the pre-delete callback", order)
	}

	revocationErr = nil
	order = nil
	if code := deleteOrg(); code != http.StatusNoContent {
		t.Fatalf("expected 204 once credentials are revoked, got %d", code)
	}
	if persistence.OrgExists("acme") {
		t.Fatal("organization still exists after a successful delete")
	}
	if !slices.Equal(order, []string{"before", "after"}) {
		t.Fatalf("callbacks = %v, want before then after", order)
	}
}

// Tokens orphaned by an organization that was deleted before deletion revoked
// them, or whose directory was removed by hand, are still bound to its ID. A
// new organization must not start out with them.
func TestOrgHandlersCreateRevokesCredentialsLeftOnTheID(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)

	dataDir := t.TempDir()
	cfg := &config.Config{DataPath: dataDir}
	cfg.APITokens = []config.APITokenRecord{
		newOrgBoundTokenForTest(t, "old-acme", 1*time.Minute, config.ScopeMonitoringRead, "acme"),
		newOrgBoundTokenForTest(t, "globex-only", 2*time.Minute, config.ScopeMonitoringRead, "globex"),
	}
	cfg.SortAPITokens()
	router := &Router{config: cfg, persistence: config.NewConfigPersistence(dataDir)}

	persistence := config.NewMultiTenantPersistence(dataDir)
	h := NewOrgHandlers(persistence, nil)
	var revocationErr error
	h.SetCredentialRevoker(func(orgID string) error {
		if persistence.OrgExists(orgID) {
			t.Errorf("credentials must be revoked before organization %q exists", orgID)
		}
		if revocationErr != nil {
			return revocationErr
		}
		return router.revokeTenantCredentials(orgID)
	})

	createOrg := func() int {
		body := bytes.NewBufferString(`{"id":"acme","displayName":"New Acme"}`)
		req := withUser(httptest.NewRequest(http.MethodPost, "/api/orgs", body), "bob")
		rec := httptest.NewRecorder()
		h.HandleCreateOrg(rec, req)
		return rec.Code
	}

	revocationErr = errors.New("token inventory unavailable")
	if code := createOrg(); code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when stale credentials cannot be revoked, got %d", code)
	}
	if persistence.OrgExists("acme") {
		t.Fatal("organization was created although stale credentials are still bound to its ID")
	}

	revocationErr = nil
	if code := createOrg(); code != http.StatusCreated {
		t.Fatalf("expected 201 once stale credentials are revoked, got %d", code)
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != "globex-only" {
		t.Fatalf("tokens after create = %+v, want only globex-only", cfg.APITokens)
	}
}
