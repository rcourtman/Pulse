package pulsecli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCLIConfigExportReplacesBackupPrivatelyAndRecoversAuthority(t *testing.T) {
	source := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", source)
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "absent-key"))
	t.Setenv("PULSE_PASSPHRASE", "synthetic-private-export-passphrase")
	createTestEncryptionKey(t, source)
	persistence := config.NewConfigPersistence(source)
	const rawToken = "synthetic-export-recovery-token"
	token, err := config.NewAPITokenRecord(rawToken, "saved monitoring", []string{config.ScopeMonitoringRead})
	require.NoError(t, err)
	token.OrgIDs = []string{"tenant-a", "tenant-b"}
	token.Metadata = map[string]string{"owner_user_id": "synthetic-owner"}
	expires := time.Now().UTC().Add(time.Hour)
	token.ExpiresAt = &expires
	require.NoError(t, persistence.SaveAPITokens([]config.APITokenRecord{*token}))
	sso := &config.SSOConfig{
		DefaultProviderID: "saved-idp", AllowMultipleProviders: true,
		Providers: []config.SSOProvider{{
			ID: "saved-idp", Name: "Saved SSO", Type: config.SSOProviderTypeOIDC, Enabled: true,
			AllowedGroups: []string{"ops"}, AllowedDomains: []string{"example.test"},
			AllowedEmails: []string{"operator@example.test"}, GroupRoleMappings: map[string]string{"ops": "viewer"},
			OIDC: &config.OIDCProviderConfig{IssuerURL: "https://idp.example.test", ClientID: "saved-client", ClientSecret: "synthetic-saved-idp-secret", Scopes: []string{"openid", "profile", "email"}},
		}},
	}
	require.NoError(t, persistence.SaveSSOConfig(sso))
	wantSSO, err := persistence.LoadSSOConfig()
	require.NoError(t, err)
	wantTokens, err := persistence.LoadAPITokens()
	require.NoError(t, err)
	outputDir := t.TempDir()
	path := filepath.Join(outputDir, "backup.enc")
	require.NoError(t, os.WriteFile(path, []byte("previous encrypted backup"), 0o644))
	output, err := newTestCLI(t).execute("config", "export", "-o", path)
	require.NoError(t, err)
	require.Contains(t, output, "Configuration exported to "+path)
	requirePrivateExportFile(t, path)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	for _, secret := range []string{rawToken, "synthetic-saved-idp-secret", "synthetic-private-export-passphrase"} {
		require.NotContains(t, string(data), secret)
		require.NotContains(t, output, secret)
	}
	require.NotContains(t, output, string(data), "file export also printed the encrypted archive")
	// Actual existing CLI import into a different disposable installation, not
	// a new archive decoder or substitute API, must retain the saved authority.
	recoveredDir := t.TempDir()
	createTestEncryptionKey(t, recoveredDir)
	t.Setenv("PULSE_DATA_DIR", recoveredDir)
	_, err = newTestCLI(t).execute("config", "import", "-i", path, "--force")
	require.NoError(t, err)
	recovered := config.NewConfigPersistence(recoveredDir)
	gotSSO, err := recovered.LoadSSOConfig()
	require.NoError(t, err)
	require.Equal(t, wantSSO, gotSSO)
	gotTokens, err := recovered.LoadAPITokens()
	require.NoError(t, err)
	require.Equal(t, wantTokens, gotTokens)
	cfg := &config.Config{APITokens: gotTokens}
	require.True(t, cfg.IsValidAPIToken(rawToken))
	require.True(t, gotTokens[0].CanAccessOrg("tenant-a"))
	require.False(t, gotTokens[0].CanAccessOrg("tenant-c"))
	require.True(t, gotTokens[0].HasScope(config.ScopeMonitoringRead))
	require.False(t, gotTokens[0].HasScope(config.ScopeSettingsWrite))
	entries, err := os.ReadDir(outputDir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "export left temporary output behind")
}

func TestCLIConfigExportRejectsNonRegularDestination(t *testing.T) {
	source := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", source)
	t.Setenv("PULSE_PASSPHRASE", "synthetic-passphrase")
	createTestEncryptionKey(t, source)
	for _, kind := range []string{"directory", "symlink", "dangling-symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "backup.enc")
			target := filepath.Join(dir, "unrelated-file")
			if kind == "directory" {
				require.NoError(t, os.Mkdir(path, 0o700))
			} else {
				if kind == "symlink" {
					require.NoError(t, os.WriteFile(target, []byte("unrelated original bytes"), 0o600))
				}
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			before, err := os.Lstat(path)
			require.NoError(t, err)
			output, err := newTestCLI(t).execute("config", "export", "-o", path)
			require.ErrorContains(t, err, "failed to write export file")
			require.ErrorContains(t, err, "not a regular file")
			require.NotContains(t, output, "Configuration exported to")
			after, err := os.Lstat(path)
			require.NoError(t, err)
			require.True(t, os.SameFile(before, after), "rejected destination changed")
			data, err := os.ReadFile(target)
			if kind == "symlink" {
				require.NoError(t, err)
				require.Equal(t, "unrelated original bytes", string(data))
			} else {
				require.True(t, os.IsNotExist(err))
			}
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			count := 1
			if kind == "symlink" {
				count = 2
			}
			require.Len(t, entries, count, "rejection left a temporary export")
		})
	}
}

func TestCLIConfigExportDoesNotRewriteHardLinkedFile(t *testing.T) {
	source := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", source)
	t.Setenv("PULSE_PASSPHRASE", "synthetic-passphrase")
	createTestEncryptionKey(t, source)
	dir := t.TempDir()
	path, other := filepath.Join(dir, "backup.enc"), filepath.Join(dir, "another-file")
	require.NoError(t, os.WriteFile(other, []byte("original linked bytes"), 0o600))
	if err := os.Link(other, path); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	_, err := newTestCLI(t).execute("config", "export", "-o", path)
	require.NoError(t, err)
	data, err := os.ReadFile(other)
	require.NoError(t, err)
	require.Equal(t, "original linked bytes", string(data))
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	require.NotEqual(t, "original linked bytes", string(data))
	requirePrivateExportFile(t, path)
}
