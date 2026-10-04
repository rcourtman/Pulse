package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/require"
)

// Exercise the documented existing-login recovery at the real configuration
// loader, not by re-running setup. This is not a native deployment/session test.
func TestPasswordRecoveryDocsLoadPreservesSettingsAndAgentToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	clearFileEnv := func() {
		for _, key := range []string{"PULSE_AUTH_USER", "PULSE_AUTH_PASS", "FRONTEND_PORT"} {
			require.NoError(t, os.Unsetenv(key))
		}
	}
	for _, key := range []string{"PULSE_AUTH_USER", "PULSE_AUTH_PASS", "FRONTEND_PORT"} {
		// Register restoration even though dotenv sets these outside t.Setenv.
		t.Setenv(key, "")
	}
	clearFileEnv()

	oldPassword, newPassword := "synthetic-old-password", "synthetic-new-password"
	oldHash, err := auth.HashPassword(oldPassword)
	require.NoError(t, err)
	newHash, err := auth.HashPassword(newPassword)
	require.NoError(t, err)
	// Apache's bcrypt records use the compatible $2y$ prefix.
	newHash = strings.Replace(newHash, "$2a$", "$2y$", 1)
	require.Len(t, newHash, 60)
	require.True(t, auth.CheckPasswordHash(newPassword, newHash))

	envPath := filepath.Join(dir, ".env")
	content := "PULSE_AUTH_USER='existing-admin'\nPULSE_AUTH_PASS='" + oldHash + "'\nFRONTEND_PORT=8765\n# preserve unrelated settings\n"
	require.NoError(t, os.WriteFile(envPath, []byte(content), 0o600))
	token, err := NewAPITokenRecord("synthetic-existing-agent-token", "Existing agent", []string{ScopeAgentReport})
	require.NoError(t, err)
	persistence := NewConfigPersistence(dir)
	require.NoError(t, persistence.SaveAPITokens([]APITokenRecord{*token}))
	tokenPath := filepath.Join(dir, "api_tokens.json")
	originalTokens, err := os.ReadFile(tokenPath)
	require.NoError(t, err)

	cfg, err := LoadWithoutLoggingInit()
	require.NoError(t, err)
	require.True(t, auth.CheckPasswordHash(oldPassword, cfg.AuthPass))

	// Model a private editor changing only the complete quoted hash.
	updated := strings.Replace(content, oldHash, newHash, 1)
	require.NoError(t, os.WriteFile(envPath, []byte(updated), 0o600))
	clearFileEnv() // A fresh process does not inherit the last dotenv load.
	cfg, err = LoadWithoutLoggingInit()
	require.NoError(t, err)
	require.Equal(t, "existing-admin", cfg.AuthUser)
	require.Equal(t, 8765, cfg.FrontendPort)
	require.Equal(t, newHash, cfg.AuthPass)
	require.True(t, auth.CheckPasswordHash(newPassword, cfg.AuthPass))
	require.False(t, auth.CheckPasswordHash(oldPassword, cfg.AuthPass))
	_, valid := cfg.ValidateAPIToken("synthetic-existing-agent-token")
	require.True(t, valid)
	gotTokens, err := os.ReadFile(tokenPath)
	require.NoError(t, err)
	require.Equal(t, originalTokens, gotTokens)
	gotEnv, err := os.ReadFile(envPath)
	require.NoError(t, err)
	require.Equal(t, updated, string(gotEnv))

	// The old deployment-supplied value still wins until its owning source is
	// updated too: editing the mounted file alone cannot repair that install.
	clearFileEnv()
	t.Setenv("PULSE_AUTH_PASS", oldHash)
	cfg, err = LoadWithoutLoggingInit()
	require.NoError(t, err)
	require.Equal(t, oldHash, cfg.AuthPass)
	require.False(t, auth.CheckPasswordHash(newPassword, cfg.AuthPass))

	// Restoring the saved file returns the former credential and port, without
	// changing the persisted agent token. This models source rollback only.
	clearFileEnv()
	require.NoError(t, os.WriteFile(envPath, []byte(content), 0o600))
	cfg, err = LoadWithoutLoggingInit()
	require.NoError(t, err)
	require.True(t, auth.CheckPasswordHash(oldPassword, cfg.AuthPass))
	require.False(t, auth.CheckPasswordHash(newPassword, cfg.AuthPass))
	require.Equal(t, 8765, cfg.FrontendPort)
	_, valid = cfg.ValidateAPIToken("synthetic-existing-agent-token")
	require.True(t, valid)
}
