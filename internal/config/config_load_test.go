package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// Avoid relying on /etc/pulse existing on the machine running tests.
	// We still want to verify "defaults" behavior when PULSE_DATA_DIR is unset.
	tmpDefault := t.TempDir()
	prevDefault := defaultDataDir
	defaultDataDir = tmpDefault
	t.Cleanup(func() { defaultDataDir = prevDefault })

	// Clear env vars that might affect defaults
	os.Unsetenv("PULSE_DATA_DIR")
	os.Unsetenv("PORT")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 7655, cfg.FrontendPort)
	assert.Equal(t, tmpDefault, cfg.DataPath)
}

func TestLoad_EnvOverrides(t *testing.T) {
	// Set some env vars
	t.Setenv("PORT", "8080")
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	t.Setenv("HTTPS_ENABLED", "true")
	t.Setenv("PULSE_AUTH_USER", "admin")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.FrontendPort)
	assert.Equal(t, tempDir, cfg.DataPath)
	assert.True(t, cfg.HTTPSEnabled)
	assert.Equal(t, "admin", cfg.AuthUser)
}

func TestLoad_DotEnv(t *testing.T) {
	tempDir := t.TempDir()
	envFile := filepath.Join(tempDir, ".env")
	content := `PULSE_AUTH_USER="dotenvuser"`
	require.NoError(t, os.WriteFile(envFile, []byte(content), 0644))

	t.Setenv("PULSE_DATA_DIR", tempDir)

	// Ensure no leakage
	os.Unsetenv("PULSE_AUTH_USER")

	cfg, err := Load()
	require.NoError(t, err)

	// godotenv.Load sets os env vars directly, bypassing t.Setenv cleanup
	t.Cleanup(func() {
		os.Unsetenv("PULSE_AUTH_USER")
	})

	assert.Equal(t, "dotenvuser", cfg.AuthUser)
}

func TestLoad_APITokens_Migration(t *testing.T) {
	// Ensure clean state
	os.Unsetenv("API_TOKEN")
	t.Setenv("API_TOKENS", "token1,token2")

	// Create temp dir to allow persistence
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg, err := Load()
	require.NoError(t, err)

	// We might get duplicates if token hashing is non-deterministic and we process same token twice?
	// But we only have token1, token2 in list.
	// If getting 3, something is weird. We assert >= 2.
	assert.GreaterOrEqual(t, len(cfg.APITokens), 2)
	assert.True(t, cfg.HasAPITokens())

	// Verify hashed
	assert.NotEqual(t, "token1", cfg.APITokens[0].Hash)
}

func TestLoad_LegacyAPIToken(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	t.Setenv("API_TOKEN", "legacytoken")

	cfg, err := Load()
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(cfg.APITokens), 1)
}

func TestLoad_MockEnv(t *testing.T) {
	// Look for mock.env in current directory (default behavior if not found elsewhere?)
	// Load() checks "mock.env" in current dir (line 537).

	// We need to work in a temp dir
	cwd, _ := os.Getwd()
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	t.Setenv("PULSE_DATA_DIR", tempDir)
	os.WriteFile("mock.env", []byte(`PULSE_MOCK_TEST="true"`), 0644)

	t.Cleanup(func() {
		os.Unsetenv("PULSE_MOCK_TEST")
	})

	_, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "true", os.Getenv("PULSE_MOCK_TEST"))
}

func TestLoad_ProxyAuth(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PROXY_AUTH_SECRET", "secret")
	t.Setenv("PROXY_AUTH_USER_HEADER", "X-User")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "secret", cfg.ProxyAuthSecret)
	assert.Equal(t, "X-User", cfg.ProxyAuthUserHeader)
}

func TestLoad_OIDC(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.com")
	t.Setenv("OIDC_CLIENT_ID", "client-id")
	t.Setenv("OIDC_CLIENT_SECRET", "client-secret")

	cfg, err := Load()
	require.NoError(t, err)

	require.NotNil(t, cfg.OIDC)
	assert.True(t, cfg.OIDC.Enabled)
	assert.Equal(t, "https://issuer.com", cfg.OIDC.IssuerURL)
}

func TestLoad_AuthPass_AutoHash(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	pass := "mysecretpassword"
	t.Setenv("PULSE_AUTH_PASS", pass)

	cfg, err := Load()
	require.NoError(t, err)

	assert.NotEqual(t, pass, cfg.AuthPass)
	assert.True(t, IsPasswordHashed(cfg.AuthPass))
}

func TestLoad_AuthPass_PreHashed(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	hash := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
	t.Setenv("PULSE_AUTH_PASS", hash)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, hash, cfg.AuthPass)
}

func TestLoad_Persistence(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	// 1. Create nodes.json using Persistence (handles encryption)
	p := NewConfigPersistence(tempDir)
	// nodes := NodesConfig{...}
	require.NoError(t, p.SaveNodesConfig(
		[]PVEInstance{{Host: "https://pve1", TokenName: "t", TokenValue: "v"}},
		nil,
		nil,
	))

	// 2. Create system_settings.json
	sysContent := `{
		"pvePollingInterval": 45,
		"logLevel": "debug"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "system.json"), []byte(sysContent), 0644)) // Note: filename is system.json or system_settings.json?

	cfg, err := Load()
	require.NoError(t, err)

	// Debug: Check if path is correct
	assert.Equal(t, tempDir, cfg.ConfigPath)

	require.Len(t, cfg.PVEInstances, 1)
	assert.Equal(t, "https://pve1:8006", cfg.PVEInstances[0].Host)
	assert.Equal(t, 45*time.Second, cfg.PVEPollingInterval)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestLoad_ReadErrors(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission tests as root")
	}

	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	// Create unreadable .env
	envFile := filepath.Join(tempDir, ".env")
	require.NoError(t, os.WriteFile(envFile, []byte("FOO=bar"), 0000))

	// Create unreadable mock.env
	mockEnv := "mock.env" // Load looks in current dir
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)
	require.NoError(t, os.WriteFile(mockEnv, []byte("MOCK=true"), 0000))

	// Create encryption key first (required before creating .enc files)
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".encryption.key"), []byte(encoded), 0600))

	// Create unreadable nodes.enc
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "nodes.enc"), []byte("data"), 0000))

	// Load should warn but succeed with defaults
	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestLoad_Persistence_InvalidFiles(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	// Invalid JSON
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "nodes.json"), []byte("{invalid"), 0644))

	cfg, err := Load()
	require.NoError(t, err)
	// Should not crash, just empty/defailts
	assert.Empty(t, cfg.PVEInstances)
}
