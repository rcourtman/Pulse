package api

import (
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

func TestSSOSnapshotSave_RollsBackOnPersistenceFailure(t *testing.T) {
	dataDir := t.TempDir()
	router := &Router{
		persistence: config.NewConfigPersistence(dataDir),
		ssoConfig: &config.SSOConfig{
			Providers: []config.SSOProvider{
				{
					ID:   "original-provider",
					Name: "Original",
					Type: config.SSOProviderTypeOIDC,
					OIDC: &config.OIDCProviderConfig{
						IssuerURL: "https://issuer.example.com",
						ClientID:  "original-client",
					},
				},
			},
			AllowMultipleProviders: true,
		},
		config: &config.Config{DataPath: dataDir},
	}

	if err := router.saveSSOConfig(); err != nil {
		t.Fatalf("failed to save initial sso config: %v", err)
	}

	runtimeHooks := newSSOAdminRuntime(router)
	newSnapshot := extensions.SSOConfigSnapshot{
		Providers: []extensions.SSOProvider{
			{
				ID:   "new-provider",
				Name: "New",
				Type: extensions.SSOProviderTypeOIDC,
				OIDC: &extensions.OIDCProviderConfig{
					IssuerURL: "https://new-issuer.example.com",
					ClientID:  "new-client",
				},
			},
		},
		AllowMultipleProviders: true,
	}

	if err := os.RemoveAll(dataDir); err != nil {
		t.Fatalf("failed to remove config dir: %v", err)
	}
	if err := os.WriteFile(dataDir, []byte("block-dir-creation"), 0o600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	if err := runtimeHooks.SaveSSOConfigSnapshot(newSnapshot); err == nil {
		t.Fatalf("expected snapshot save to fail")
	}

	if len(router.ssoConfig.Providers) != 1 {
		t.Fatalf("expected router config rollback to restore single provider, got %d", len(router.ssoConfig.Providers))
	}
	if router.ssoConfig.Providers[0].ID != "original-provider" {
		t.Fatalf("expected original provider to remain after rollback, got %q", router.ssoConfig.Providers[0].ID)
	}
}
