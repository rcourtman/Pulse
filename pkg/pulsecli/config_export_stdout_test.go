package pulsecli

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

type configExportWriterFunc func([]byte) (int, error)

func (f configExportWriterFunc) Write(p []byte) (int, error) { return f(p) }

func (tc *testCLI) commandForConfigExport() *cobra.Command {
	cmd := NewRootCommand(CommandSpec{Use: "pulse"}, RuntimeSpec{}, CommandDeps{Config: tc.configDeps()})
	cmd.SetArgs([]string{"config", "export"})
	return cmd
}

func TestCLIConfigExportStdoutFailures(t *testing.T) {
	source := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", source)
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "absent-key"))
	const passphrase = "synthetic-stdout-export-passphrase"
	t.Setenv("PULSE_PASSPHRASE", passphrase)
	createTestEncryptionKey(t, source)
	persistence := config.NewConfigPersistence(source)
	token, err := config.NewAPITokenRecord("synthetic-stdout-saved-token", "saved monitoring", []string{config.ScopeMonitoringRead})
	require.NoError(t, err)
	token.OrgIDs = []string{"tenant-a"}
	require.NoError(t, persistence.SaveAPITokens([]config.APITokenRecord{*token}))
	// Settle any existing read-time migrations before comparing failed output.
	_, err = persistence.ExportConfig(passphrase)
	require.NoError(t, err)
	before := configExportSourceFiles(t, source)
	sinkErr := errors.New("synthetic destination write failure")
	for _, tc := range []struct {
		name     string
		accepted func(int) int
		writeErr error
		wantErr  error
	}{
		{"rejected", func(int) int { return 0 }, sinkErr, sinkErr},
		{"partial-error", func(n int) int { return n / 2 }, sinkErr, sinkErr},
		{"full-count-with-error", func(n int) int { return n }, sinkErr, sinkErr},
		{"short-without-error", func(n int) int { return n / 2 }, nil, io.ErrShortWrite},
		{"zero-without-error", func(int) int { return 0 }, nil, io.ErrShortWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newTestCLI(t).commandForConfigExport()
			calls := 0
			var attempted []byte
			cmd.SetOut(configExportWriterFunc(func(p []byte) (int, error) {
				calls++
				attempted = append(attempted, p...)
				return tc.accepted(len(p)), tc.writeErr
			}))
			var diagnostic bytes.Buffer
			cmd.SetErr(&diagnostic)
			err := cmd.Execute()
			require.ErrorIs(t, err, tc.wantErr)
			require.ErrorContains(t, err, "failed to write configuration export to stdout")
			require.Equal(t, 1, calls, "an uncertain archive write must not be retried")
			require.NotEmpty(t, attempted)
			require.Equal(t, byte('\n'), attempted[len(attempted)-1])
			_, decodeErr := base64.StdEncoding.DecodeString(string(attempted[:len(attempted)-1]))
			require.NoError(t, decodeErr, "stdout format changed")
			require.NotContains(t, string(attempted), passphrase)
			require.NotContains(t, string(attempted), "synthetic-stdout-saved-token")
			require.NotContains(t, diagnostic.String(), passphrase)
			require.NotContains(t, diagnostic.String(), "synthetic-stdout-saved-token")
			require.NotContains(t, diagnostic.String(), string(attempted[:len(attempted)-1]))
			require.Equal(t, before, configExportSourceFiles(t, source), "failed output changed saved source state")
		})
	}
}

func TestCLIConfigExportStdoutRecoversSavedAuthority(t *testing.T) {
	source := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", source)
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "absent-key"))
	t.Setenv("PULSE_PASSPHRASE", "synthetic-stdout-roundtrip-passphrase")
	createTestEncryptionKey(t, source)
	persistence := config.NewConfigPersistence(source)
	token, err := config.NewAPITokenRecord("synthetic-stdout-roundtrip-token", "saved monitoring", []string{config.ScopeMonitoringRead})
	require.NoError(t, err)
	token.OrgIDs = []string{"tenant-a"}
	expires := time.Date(2030, 5, 6, 7, 8, 9, 0, time.UTC)
	token.ExpiresAt = &expires
	require.NoError(t, persistence.SaveAPITokens([]config.APITokenRecord{*token}))
	sso := config.NewSSOConfig()
	sso.Providers = []config.SSOProvider{{
		ID: "saved-idp", Type: config.SSOProviderTypeOIDC, Enabled: true,
		AllowedGroups: []string{"ops"}, AllowedDomains: []string{"example.test"},
		OIDC: &config.OIDCProviderConfig{ClientID: "saved-client", ClientSecret: "synthetic-stdout-idp-secret"},
	}}
	require.NoError(t, persistence.SaveSSOConfig(sso))
	wantTokens, err := persistence.LoadAPITokens()
	require.NoError(t, err)
	wantSSO, err := persistence.LoadSSOConfig()
	require.NoError(t, err)

	cmd := newTestCLI(t).commandForConfigExport()
	var stdout bytes.Buffer
	calls := 0
	cmd.SetOut(configExportWriterFunc(func(p []byte) (int, error) {
		calls++
		return stdout.Write(p)
	}))
	cmd.SetErr(io.Discard)
	require.NoError(t, cmd.Execute())
	require.Equal(t, 1, calls)
	require.Equal(t, 1, bytes.Count(stdout.Bytes(), []byte("\n")))
	require.Equal(t, byte('\n'), stdout.Bytes()[stdout.Len()-1])
	_, err = base64.StdEncoding.DecodeString(stdout.String())
	require.NoError(t, err)
	for _, secret := range []string{"synthetic-stdout-roundtrip-passphrase", "synthetic-stdout-roundtrip-token", "synthetic-stdout-idp-secret"} {
		require.NotContains(t, stdout.String(), secret)
	}
	archive := filepath.Join(t.TempDir(), "stdout.enc")
	require.NoError(t, os.WriteFile(archive, stdout.Bytes(), 0o600))
	recoveredDir := t.TempDir()
	createTestEncryptionKey(t, recoveredDir)
	t.Setenv("PULSE_DATA_DIR", recoveredDir)
	_, err = newTestCLI(t).execute("config", "import", "-i", archive, "--force")
	require.NoError(t, err)
	recovered := config.NewConfigPersistence(recoveredDir)
	gotTokens, err := recovered.LoadAPITokens()
	require.NoError(t, err)
	require.Equal(t, wantTokens, gotTokens)
	gotSSO, err := recovered.LoadSSOConfig()
	require.NoError(t, err)
	require.Equal(t, wantSSO, gotSSO)
	// Recovery must not turn an unchanged monitoring token into a wider grant.
	require.True(t, gotTokens[0].HasScope(config.ScopeMonitoringRead))
	require.False(t, gotTokens[0].HasScope(config.ScopeSettingsWrite))
	require.True(t, gotTokens[0].CanAccessOrg("tenant-a"))
	require.False(t, gotTokens[0].CanAccessOrg("tenant-b"))
}

func configExportSourceFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	files := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		require.False(t, entry.IsDir())
		files[entry.Name()], err = os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, err)
	}
	return files
}
