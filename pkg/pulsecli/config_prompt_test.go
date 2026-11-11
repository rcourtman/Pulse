package pulsecli

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

// Capture the process defaults as well as Cobra's configurable writers: a
// successful encrypted export must not depend on a caller stripping prompts.
func captureConfigPromptStreams(t *testing.T, fn func()) (string, string) {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "stdout"))
	require.NoError(t, err)
	defer out.Close()
	diagnostic, err := os.Create(filepath.Join(dir, "stderr"))
	require.NoError(t, err)
	defer diagnostic.Close()
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, diagnostic
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	fn()
	stdout, err := os.ReadFile(out.Name())
	require.NoError(t, err)
	stderr, err := os.ReadFile(diagnostic.Name())
	require.NoError(t, err)
	return string(stdout), string(stderr)
}

func TestGetPassphraseKeepsPromptsOffStdout(t *testing.T) {
	for _, tc := range []struct {
		name       string
		env, flag  string
		confirm    bool
		answers    []string
		failRead   int
		want       string
		diagnostic string
	}{
		{name: "environment-precedes-flag", env: "synthetic-env-secret", flag: "synthetic-flag-secret", want: "synthetic-env-secret"},
		{name: "flag", flag: "synthetic-flag-secret", want: "synthetic-flag-secret"},
		{name: "interactive", answers: []string{"synthetic-secret"}, want: "synthetic-secret", diagnostic: "Passphrase: \n"},
		{name: "confirmed", confirm: true, answers: []string{"synthetic-secret", "synthetic-secret"}, want: "synthetic-secret", diagnostic: "Passphrase: \nConfirm passphrase: \n"},
		{name: "mismatch", confirm: true, answers: []string{"synthetic-secret", "synthetic-other"}, diagnostic: "Passphrase: \nConfirm passphrase: \nPassphrases do not match\n"},
		{name: "read-error", failRead: 1, answers: []string{""}, diagnostic: "Passphrase: \n"},
		{name: "confirmation-error", confirm: true, failRead: 2, answers: []string{"synthetic-secret", ""}, diagnostic: "Passphrase: \nConfirm passphrase: \n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PULSE_PASSPHRASE", tc.env)
			reads := 0
			deps := &ConfigDeps{Passphrase: &tc.flag, ReadPassword: func(fd int) ([]byte, error) {
				require.Equal(t, int(syscall.Stdin), fd)
				reads++
				if reads == tc.failRead {
					return nil, errors.New("synthetic terminal read failure")
				}
				require.LessOrEqual(t, reads, len(tc.answers))
				return []byte(tc.answers[reads-1]), nil
			}}
			stdout, stderr := captureConfigPromptStreams(t, func() {
				require.Equal(t, tc.want, GetPassphrase(deps, "Passphrase: ", tc.confirm))
			})
			require.Empty(t, stdout, "human prompts contaminate a redirected backup")
			require.Equal(t, tc.diagnostic, stderr)
			require.Equal(t, len(tc.answers), reads)
		})
	}
}

func TestCLIConfigInteractiveExportUsesDiagnosticStreamAndRecovers(t *testing.T) {
	source := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", source)
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "absent-key"))
	t.Setenv("PULSE_PASSPHRASE", "")
	createTestEncryptionKey(t, source)
	persistence := config.NewConfigPersistence(source)
	token, err := config.NewAPITokenRecord("synthetic-interactive-token", "saved monitoring", []string{config.ScopeMonitoringRead})
	require.NoError(t, err)
	token.OrgIDs = []string{"tenant-a"}
	expires := time.Date(2030, 5, 6, 7, 8, 9, 0, time.UTC)
	token.ExpiresAt = &expires
	require.NoError(t, persistence.SaveAPITokens([]config.APITokenRecord{*token}))
	wantTokens, err := persistence.LoadAPITokens()
	require.NoError(t, err)

	cli := newTestCLI(t)
	cli.readPassword = func(fd int) ([]byte, error) {
		require.Equal(t, int(syscall.Stdin), fd)
		return []byte("synthetic-interactive-passphrase"), nil
	}
	cmd := cli.commandForConfigExport()
	var archive, diagnostic bytes.Buffer
	cmd.SetOut(&archive)
	cmd.SetErr(&diagnostic)
	require.NoError(t, cmd.Execute())
	require.Equal(t, "Enter passphrase for encryption: \n", diagnostic.String())
	require.Equal(t, 1, bytes.Count(archive.Bytes(), []byte("\n")))
	_, err = base64.StdEncoding.DecodeString(archive.String())
	require.NoError(t, err)
	for _, secret := range []string{"synthetic-interactive-token", "synthetic-interactive-passphrase"} {
		require.NotContains(t, archive.String(), secret)
		require.NotContains(t, diagnostic.String(), secret)
	}
	path := filepath.Join(t.TempDir(), "backup.enc")
	require.NoError(t, os.WriteFile(path, archive.Bytes(), 0o600))
	recoveredDir := t.TempDir()
	createTestEncryptionKey(t, recoveredDir)
	t.Setenv("PULSE_DATA_DIR", recoveredDir)
	cmd = NewRootCommand(CommandSpec{Use: "pulse"}, RuntimeSpec{}, CommandDeps{Config: cli.configDeps()})
	cmd.SetArgs([]string{"config", "import", "-i", path, "--force"})
	var status bytes.Buffer
	diagnostic.Reset()
	cmd.SetOut(&status)
	cmd.SetErr(&diagnostic)
	require.NoError(t, cmd.Execute())
	require.Equal(t, "Enter passphrase for decryption: \n", diagnostic.String())
	require.Contains(t, status.String(), "Configuration imported successfully")
	require.NotContains(t, status.String(), "passphrase")
	recovered := config.NewConfigPersistence(recoveredDir)
	gotTokens, err := recovered.LoadAPITokens()
	require.NoError(t, err)
	require.Equal(t, wantTokens, gotTokens)
	require.True(t, gotTokens[0].HasScope(config.ScopeMonitoringRead))
	require.False(t, gotTokens[0].HasScope(config.ScopeSettingsWrite))
	require.True(t, gotTokens[0].CanAccessOrg("tenant-a"))
	require.False(t, gotTokens[0].CanAccessOrg("tenant-b"))
}
