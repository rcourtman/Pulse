package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadPersistedSSOConfigDoesNotMigrateLegacy(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		t.Run(map[bool]string{false: "plaintext", true: "encrypted"}[encrypted], func(t *testing.T) {
			dir := t.TempDir()
			cp := NewConfigPersistence(dir)
			if !encrypted {
				cp.crypto = nil
			}
			legacy := &OIDCConfig{Enabled: true, IssuerURL: "https://legacy.example.test", ClientID: "legacy", ClientSecret: "synthetic-legacy-secret", AllowedGroups: []string{"old-ops"}, GroupRoleMappings: map[string]string{"old-ops": "viewer"}}
			raw, err := json.Marshal(legacy)
			require.NoError(t, err)
			if encrypted {
				raw, err = cp.crypto.Encrypt(raw)
				require.NoError(t, err)
			}
			legacyFile := filepath.Join(dir, "oidc.enc")
			require.NoError(t, os.WriteFile(legacyFile, raw, 0600))
			// Persisted-only reads do not turn absence into a new migrated file.
			cfg, err := cp.LoadPersistedSSOConfig()
			require.NoError(t, err)
			require.Nil(t, cfg)
			_, err = os.Stat(filepath.Join(dir, "sso.enc"))
			require.True(t, os.IsNotExist(err))
			// Ordinary initial startup still performs its legitimate migration.
			cfg, err = cp.LoadSSOConfig()
			require.NoError(t, err)
			provider := cfg.GetProvider(LegacyOIDCProviderID)
			require.NotNil(t, provider)
			require.True(t, provider.Enabled)
			require.Equal(t, legacy.ClientSecret, provider.OIDC.ClientSecret)
			require.Equal(t, legacy.AllowedGroups, provider.AllowedGroups)
			require.Equal(t, legacy.GroupRoleMappings, provider.GroupRoleMappings)
			saved, err := os.ReadFile(filepath.Join(dir, "sso.enc"))
			require.NoError(t, err)
			loaded, err := cp.LoadPersistedSSOConfig()
			require.NoError(t, err)
			require.Equal(t, cfg, loaded)
			after, err := os.ReadFile(filepath.Join(dir, "sso.enc"))
			require.NoError(t, err)
			require.Equal(t, saved, after)
			afterLegacy, err := os.ReadFile(legacyFile)
			require.NoError(t, err)
			require.Equal(t, raw, afterLegacy)
		})
	}
}
