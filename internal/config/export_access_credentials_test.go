package config_test

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

// An import that keeps access credentials restores every other section and
// leaves the destination's API token inventory and SSO configuration as saved;
// the default import still replaces both.
func TestImportConfigKeepAccessCredentials(t *testing.T) {
	const passphrase = "access-credential-passphrase"

	sso := func(id string) *config.SSOConfig {
		return &config.SSOConfig{Providers: []config.SSOProvider{{
			ID: id, Name: id, Type: config.SSOProviderTypeOIDC,
			OIDC: &config.OIDCProviderConfig{IssuerURL: "https://" + id + ".invalid", ClientID: id},
		}}}
	}
	token := func(raw string, scopes ...string) config.APITokenRecord {
		record, err := config.NewAPITokenRecord(raw, raw, scopes)
		require.NoError(t, err)
		return *record
	}

	source := config.NewConfigPersistence(t.TempDir())
	require.NoError(t, source.EnsureConfigDir())
	archived := token("archived-full-access-token-0123456789", config.ScopeWildcard)
	require.NoError(t, source.SaveAPITokens([]config.APITokenRecord{archived}))
	require.NoError(t, source.SaveSSOConfig(sso("archived")))
	settings := config.DefaultSystemSettings()
	settings.PublicURL = "https://archived.invalid"
	require.NoError(t, source.SaveSystemSettings(*settings))
	exported, err := source.ExportConfig(passphrase)
	require.NoError(t, err)

	target := config.NewConfigPersistence(t.TempDir())
	require.NoError(t, target.EnsureConfigDir())
	saved := token("saved-settings-write-token-0123456789", config.ScopeSettingsWrite)
	require.NoError(t, target.SaveAPITokens([]config.APITokenRecord{saved}))
	require.NoError(t, target.SaveSSOConfig(sso("saved")))

	state := func() (string, string, string) {
		t.Helper()
		tokens, err := target.LoadAPITokens()
		require.NoError(t, err)
		require.Len(t, tokens, 1)
		loadedSSO, err := target.LoadSSOConfig()
		require.NoError(t, err)
		require.Len(t, loadedSSO.Providers, 1)
		loadedSettings, err := target.LoadSystemSettings()
		require.NoError(t, err)
		return tokens[0].ID, loadedSSO.Providers[0].ID, loadedSettings.PublicURL
	}

	require.NoError(t, target.ImportConfigWithOptions(exported, passphrase, config.ImportOptions{KeepAccessCredentials: true}))
	tokenID, providerID, publicURL := state()
	require.Equal(t, saved.ID, tokenID)
	require.Equal(t, "saved", providerID)
	require.Equal(t, "https://archived.invalid", publicURL)

	require.NoError(t, target.ImportConfig(exported, passphrase))
	tokenID, providerID, _ = state()
	require.Equal(t, archived.ID, tokenID)
	require.Equal(t, "archived", providerID)
}
