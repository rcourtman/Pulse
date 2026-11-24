package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// TestConfigImportPublishesRestoredSSOConfig restores a backup taken before one
// provider was added and another was changed. Sign-in is served from the
// published SSO configuration, so the import has to publish what it saved: the
// removed provider must stop resolving, the changed one must carry the
// archive's client, and a later SSO save must not write either back. Every
// provider is synthetic and none is contacted.
func TestConfigImportPublishesRestoredSSOConfig(t *testing.T) {
	const passphrase = "synthetic-passphrase"
	fullRaw := "synthetic-full-token-123.12345678"
	t.Setenv("OIDC_ENABLED", "false")

	provider := func(id, clientID string) config.SSOProvider {
		return config.SSOProvider{ID: id, Name: id, Type: config.SSOProviderTypeOIDC, Enabled: true,
			OIDC: &config.OIDCProviderConfig{IssuerURL: "https://idp.invalid", ClientID: clientID}}
	}

	cfg := newTestConfigWithTokens(t, newTokenRecord(t, fullRaw, []string{config.ScopeWildcard}, nil))
	t.Setenv("PULSE_DATA_DIR", cfg.DataPath)
	persistence := config.NewConfigPersistence(cfg.DataPath)
	if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
		t.Fatalf("save destination tokens: %v", err)
	}

	source := config.NewConfigPersistence(t.TempDir())
	if err := source.SaveAPITokens(cfg.APITokens); err != nil {
		t.Fatalf("save archived tokens: %v", err)
	}
	if err := source.SaveSSOConfig(&config.SSOConfig{Providers: []config.SSOProvider{
		provider("kept", "kept-client"), provider("changed", "archived-client"),
	}}); err != nil {
		t.Fatalf("save archived SSO config: %v", err)
	}
	archive, err := source.ExportConfig(passphrase)
	if err != nil {
		t.Fatalf("export archive: %v", err)
	}
	body, err := json.Marshal(map[string]string{"data": archive, "passphrase": passphrase})
	if err != nil {
		t.Fatalf("marshal import request: %v", err)
	}

	router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	applyErr, saveErr := router.commitSSOConfig(func(sso *config.SSOConfig) error {
		for _, p := range []config.SSOProvider{provider("kept", "kept-client"), provider("changed", "live-client"), provider("partner", "partner-client")} {
			if err := sso.AddProvider(p); err != nil {
				return err
			}
		}
		return nil
	})
	if applyErr != nil || saveErr != nil {
		t.Fatalf("seed live SSO config: apply %v, save %v", applyErr, saveErr)
	}
	// Stand in for the services a sign-in through each provider left behind.
	for _, id := range []string{"kept", "changed", "partner"} {
		service := &OIDCService{stateStore: newOIDCStateStore()}
		t.Cleanup(service.Stop)
		router.oidcManager.services[id] = service
	}

	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(string(body)))
	req.RemoteAddr = "127.0.2.1:1234"
	req.Header.Set("X-API-Token", fullRaw)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("import status = %d: %s", rec.Code, rec.Body.String())
	}

	if router.getSSOProvider("partner") != nil {
		t.Fatal("provider removed by the import still resolves for sign-in")
	}
	if changed := router.getSSOProvider("changed"); changed == nil || changed.OIDC.ClientID != "archived-client" {
		t.Fatalf("provider changed by the import = %+v, want the archive's client", changed)
	}
	if router.getSSOProvider("kept") == nil || !hasEnabledSSOProvidersForAuth(cfg) {
		t.Fatal("provider kept by the import no longer resolves")
	}
	if router.oidcManager.GetService("partner") != nil || router.oidcManager.GetService("changed") != nil {
		t.Fatal("import left the live service of a removed or changed provider")
	}
	if router.oidcManager.GetService("kept") == nil {
		t.Fatal("import discarded the live service of an unchanged provider")
	}

	applyErr, saveErr = router.commitSSOConfig(func(sso *config.SSOConfig) error {
		sso.GetProvider("kept").DisplayName = "Kept"
		return nil
	})
	if applyErr != nil || saveErr != nil {
		t.Fatalf("edit SSO config after import: apply %v, save %v", applyErr, saveErr)
	}
	saved, err := persistence.LoadSSOConfig()
	if err != nil || saved == nil {
		t.Fatalf("load saved SSO config: %v", err)
	}
	if saved.GetProvider("partner") != nil {
		t.Fatal("SSO save after the import wrote the removed provider back")
	}
	if changed := saved.GetProvider("changed"); changed == nil || changed.OIDC.ClientID != "archived-client" {
		t.Fatalf("SSO save after the import wrote %+v over the archive's provider", changed)
	}
}
