package config

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/require"
)

// These fixtures are unrelated local state, not real credentials or host files.
func staleConfigPath(t *testing.T, path, kind string) string {
	t.Helper()
	unrelated := filepath.Join(t.TempDir(), "unrelated")
	require.NoError(t, os.WriteFile(unrelated, []byte("unrelated state"), 0o600))
	var err error
	switch kind {
	case "regular":
		err = os.WriteFile(path, []byte("stale state"), 0o600)
		if err == nil {
			err = os.Chmod(path, 0o666)
		}
	case "symlink":
		err = os.Symlink(unrelated, path)
	case "hardlink":
		err = os.Link(unrelated, path)
	case "directory":
		err = os.Mkdir(path, 0o700)
	default:
		t.Fatalf("unknown fixture kind %q", kind)
	}
	if err != nil && runtime.GOOS == "windows" && (kind == "symlink" || kind == "hardlink") {
		t.Skipf("link fixture unavailable: %v", err)
	}
	require.NoError(t, err)
	return unrelated
}

func requirePrivateConfigFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular(), "configuration must be a regular file")
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func requireUntouchedConfigFixture(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "unrelated state", string(raw))
}

func TestConfigPersistenceIgnoresStaleTemporaryPaths(t *testing.T) {
	for _, operation := range []string{"email", "tokens", "environment"} {
		for _, kind := range []string{"regular", "symlink", "hardlink", "directory"} {
			t.Run(operation+"/"+kind, func(t *testing.T) {
				dir := t.TempDir()
				cp := NewConfigPersistence(dir)
				path := cp.emailFile
				switch operation {
				case "tokens":
					path = cp.apiTokensFile
				case "environment":
					path = filepath.Join(dir, ".env")
					require.NoError(t, os.WriteFile(path, []byte("PULSE_AUTH_PASS=synthetic-private-value\nUPDATE_CHANNEL=beta\n"), 0o600))
				}
				stale := path + ".tmp"
				unrelated := staleConfigPath(t, stale, kind)
				staleInfo, err := os.Lstat(stale)
				require.NoError(t, err)

				switch operation {
				case "email":
					want := notifications.EmailConfig{Password: "synthetic-private-value"}
					require.NoError(t, cp.SaveEmailConfig(want))
					got, err := cp.LoadEmailConfig()
					require.NoError(t, err)
					require.Equal(t, want.Password, got.Password)
					raw, err := os.ReadFile(path)
					require.NoError(t, err)
					require.NotContains(t, string(raw), want.Password)
				case "tokens":
					want := []APITokenRecord{{ID: "fixture-id", Hash: "fixture-hash", Name: "fixture-token", Scopes: []string{"monitoring:read"}}}
					require.NoError(t, cp.SaveAPITokens(want))
					got, err := cp.LoadAPITokens()
					require.NoError(t, err)
					require.Len(t, got, 1)
					require.Equal(t, want[0].Hash, got[0].Hash)
				case "environment":
					require.NoError(t, cp.SaveSystemSettings(SystemSettings{UpdateChannel: "stable"}))
					raw, err := os.ReadFile(path)
					require.NoError(t, err)
					require.Equal(t, "PULSE_AUTH_PASS=synthetic-private-value\nUPDATE_CHANNEL=stable\n", string(raw))
				}
				requireUntouchedConfigFixture(t, unrelated)
				requirePrivateConfigFile(t, path)
				// Do not delete or replace a path that this write never owned.
				after, err := os.Lstat(stale)
				require.NoError(t, err)
				require.True(t, os.SameFile(staleInfo, after))
				if kind == "regular" {
					raw, err := os.ReadFile(stale)
					require.NoError(t, err)
					require.Equal(t, "stale state", string(raw))
				}
			})
		}
	}
}

func TestConfigPersistenceBackupsReplaceLinksPrivately(t *testing.T) {
	for _, kind := range []string{"regular", "symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			cp := NewConfigPersistence(t.TempDir())
			records := []APITokenRecord{{ID: "fixture-id", Hash: "first-hash", Scopes: []string{"monitoring:read"}}}
			require.NoError(t, cp.SaveAPITokens(records))
			original, err := os.ReadFile(cp.apiTokensFile)
			require.NoError(t, err)
			backup := cp.apiTokensFile + ".backup"
			unrelated := staleConfigPath(t, backup, kind)
			records[0].Hash = "second-hash"
			require.NoError(t, cp.SaveAPITokens(records))
			requireUntouchedConfigFixture(t, unrelated)
			requirePrivateConfigFile(t, backup)
			raw, err := os.ReadFile(backup)
			require.NoError(t, err)
			require.True(t, bytes.Equal(original, raw), "backup must preserve exact previous ciphertext")
			got, err := cp.LoadAPITokens()
			require.NoError(t, err)
			require.Len(t, got, 1)
			require.Equal(t, "second-hash", got[0].Hash)
		})
	}
}

func TestConfigPersistenceMigrationIgnoresStaleTemporaryPaths(t *testing.T) {
	for _, kind := range []string{"regular", "symlink", "hardlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			cp := NewConfigPersistence(t.TempDir())
			legacy := []byte(`{"providers":[],"allowMultipleProviders":true}`)
			require.NoError(t, os.WriteFile(cp.ssoFile, legacy, 0o600))
			unrelated := staleConfigPath(t, cp.ssoFile+".tmp", kind)
			loaded, err := cp.LoadSSOConfig()
			require.NoError(t, err)
			require.NotNil(t, loaded)
			require.True(t, loaded.AllowMultipleProviders)
			requireUntouchedConfigFixture(t, unrelated)
			requirePrivateConfigFile(t, cp.ssoFile)
			raw, err := os.ReadFile(cp.ssoFile)
			require.NoError(t, err)
			require.False(t, bytes.Equal(legacy, raw))
			decrypted, err := cp.crypto.Decrypt(raw)
			require.NoError(t, err)
			require.JSONEq(t, string(legacy), string(decrypted))
		})
	}
}
