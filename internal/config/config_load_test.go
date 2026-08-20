package config

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	os.Unsetenv("FRONTEND_PORT")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 7655, cfg.FrontendPort)
	assert.Equal(t, tmpDefault, cfg.DataPath)
}

func TestLoad_DefaultHostedCommercialBaseURLAvoidsRetiredTrialPath(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PULSE_PRO_TRIAL_SIGNUP_URL", "")

	cfg, err := Load()
	require.NoError(t, err)

	if strings.Contains(cfg.ProTrialSignupURL, "start-pro-trial") {
		t.Fatalf("ProTrialSignupURL=%q must not default to retired trial signup route", cfg.ProTrialSignupURL)
	}
}

func TestConfigValidateRejectsUnsafePVEClusterDisplayName(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	cfg, err := LoadWithoutLoggingInit()
	require.NoError(t, err)
	cfg.PVEInstances = []PVEInstance{{
		Host:       "https://pve.example.test:8006",
		TokenName:  "root@pam!pulse",
		TokenValue: "secret",
		IsCluster:  true,
		ClusterNodeIdentities: []PVEClusterNodeIdentity{{
			ID:          "production-pve1",
			NativeName:  "pve1",
			DisplayName: "unsafe\nlabel",
		}},
	}}

	err = cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "control characters")
}

func TestResolveRuntimeDataDir(t *testing.T) {
	tmpDefault := t.TempDir()
	prevDefault := defaultDataDir
	defaultDataDir = tmpDefault
	t.Cleanup(func() { defaultDataDir = prevDefault })

	t.Run("explicit_path_wins", func(t *testing.T) {
		t.Setenv("PULSE_DATA_DIR", t.TempDir())
		explicit := t.TempDir()
		assert.Equal(t, explicit, ResolveRuntimeDataDir(explicit))
	})

	t.Run("env_path_fallback", func(t *testing.T) {
		envDir := t.TempDir()
		t.Setenv("PULSE_DATA_DIR", envDir)
		assert.Equal(t, envDir, ResolveRuntimeDataDir(""))
	})

	t.Run("default_fallback", func(t *testing.T) {
		os.Unsetenv("PULSE_DATA_DIR")
		assert.Equal(t, tmpDefault, ResolveRuntimeDataDir(""))
	})
}

func TestLoad_EnvOverrides(t *testing.T) {
	// Set some env vars
	t.Setenv("FRONTEND_PORT", "8080")
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

func TestLoad_TelemetryEnabledDefaultAndEnvOverride(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	os.Unsetenv("PULSE_TELEMETRY")

	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.TelemetryEnabled)
	assert.False(t, cfg.EnvOverrides["PULSE_TELEMETRY"])
	assert.False(t, cfg.EnvOverrides["telemetryEnabled"])

	t.Setenv("PULSE_TELEMETRY", "false")
	cfg, err = Load()
	require.NoError(t, err)
	assert.False(t, cfg.TelemetryEnabled)
	assert.True(t, cfg.EnvOverrides["PULSE_TELEMETRY"])
	assert.True(t, cfg.EnvOverrides["telemetryEnabled"])
}

func TestLoad_AgentIngestPortDefaultsDisabled(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	os.Unsetenv("PULSE_AGENT_INGEST_PORT")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 0, cfg.AgentIngestPort)
}

func TestLoad_AgentIngestPortEnvOverride(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PULSE_AGENT_INGEST_PORT", "7656")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 7656, cfg.AgentIngestPort)
}

func TestLoad_AgentIngestPortInvalidValueIgnored(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PULSE_AGENT_INGEST_PORT", "not-a-port")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 0, cfg.AgentIngestPort)
}

func TestLoad_AgentIngestPortRejectsFrontendPortCollision(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("FRONTEND_PORT", "7655")
	t.Setenv("PULSE_AGENT_INGEST_PORT", "7655")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent ingest port")
}

func TestLoad_ProxmoxGuestDockerDetectionEnvOptIn(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.EnableProxmoxGuestDockerDetection)
	assert.False(t, cfg.EnableProxmoxGuestDockerInventory)
	assert.Empty(t, cfg.ProxmoxGuestDockerInventoryVMIDs)
	assert.False(t, cfg.EnvOverrides["PULSE_ENABLE_PROXMOX_GUEST_DOCKER_DETECTION"])
	assert.False(t, cfg.EnvOverrides["PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY"])
	assert.False(t, cfg.EnvOverrides["PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS"])

	t.Setenv("PULSE_ENABLE_PROXMOX_GUEST_DOCKER_DETECTION", "true")
	t.Setenv("PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY", "true")
	t.Setenv("PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS", "101,102")

	cfg, err = Load()
	require.NoError(t, err)
	assert.True(t, cfg.EnableProxmoxGuestDockerDetection)
	assert.True(t, cfg.EnableProxmoxGuestDockerInventory)
	assert.Equal(t, "101,102", cfg.ProxmoxGuestDockerInventoryVMIDs)
	assert.True(t, cfg.EnvOverrides["PULSE_ENABLE_PROXMOX_GUEST_DOCKER_DETECTION"])
	assert.True(t, cfg.EnvOverrides["PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY"])
	assert.True(t, cfg.EnvOverrides["PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS"])
}

func TestLoad_MetricsStorageEnvOverrides(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PULSE_METRICS_DB_PATH", "/dev/shm/pulse/metrics.db")
	t.Setenv("PULSE_METRICS_ROLLUP_INTERVAL", "30m")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "/dev/shm/pulse/metrics.db", cfg.MetricsDBPath)
	assert.Equal(t, 30*time.Minute, cfg.MetricsRollupInterval)
	assert.True(t, cfg.EnvOverrides["PULSE_METRICS_DB_PATH"])
	assert.True(t, cfg.EnvOverrides["PULSE_METRICS_ROLLUP_INTERVAL"])
}

func TestLoad_MetricsBindSecurityDefaultsAndOverrides(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", cfg.MetricsBindAddress)
	assert.False(t, cfg.MetricsAllowInsecureRemote)

	t.Setenv("PULSE_METRICS_BIND_ADDRESS", "0.0.0.0")
	t.Setenv("PULSE_METRICS_ALLOW_INSECURE_REMOTE", "true")

	cfg, err = Load()
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", cfg.MetricsBindAddress)
	assert.True(t, cfg.MetricsAllowInsecureRemote)
	assert.True(t, cfg.EnvOverrides["PULSE_METRICS_BIND_ADDRESS"])
	assert.True(t, cfg.EnvOverrides["PULSE_METRICS_ALLOW_INSECURE_REMOTE"])
}

func TestLoad_InvalidMetricsRollupIntervalIgnored(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PULSE_METRICS_ROLLUP_INTERVAL", "2m")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Zero(t, cfg.MetricsRollupInterval)
	assert.False(t, cfg.EnvOverrides["PULSE_METRICS_ROLLUP_INTERVAL"])
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

func TestLoad_APITokensEnvIgnored(t *testing.T) {
	os.Unsetenv("API_TOKEN")
	t.Setenv("API_TOKENS", "token1,token2")
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	cfg, err := Load()
	require.NoError(t, err)

	assert.Len(t, cfg.APITokens, 0)
	assert.False(t, cfg.HasAPITokens())
}

func TestLoad_LegacyAPITokenEnvIgnored(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("API_TOKEN", "legacytoken")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Len(t, cfg.APITokens, 0)
	assert.False(t, cfg.HasAPITokens())
}

func TestLoad_APITokens_LegacyOrgBindingMigrated(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("API_TOKENS")

	p := NewConfigPersistence(tempDir)
	legacy := []APITokenRecord{
		{
			ID:        "legacy-token",
			Name:      "Legacy",
			Hash:      "legacy-hash",
			Prefix:    "legacy",
			Suffix:    "hash",
			CreatedAt: time.Now().UTC(),
			Scopes:    []string{ScopeWildcard},
		},
	}
	require.NoError(t, p.SaveAPITokens(legacy))

	cfg, err := Load()
	require.NoError(t, err)
	require.Len(t, cfg.APITokens, 1)
	assert.Equal(t, "default", cfg.APITokens[0].OrgID)

	persisted, err := p.LoadAPITokens()
	require.NoError(t, err)
	require.Len(t, persisted, 1)
	assert.Equal(t, "default", persisted[0].OrgID)
}

func TestLoad_APITokens_MissingIDMigrated(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	os.Unsetenv("API_TOKEN")
	os.Unsetenv("API_TOKENS")

	p := NewConfigPersistence(tempDir)
	legacy := []APITokenRecord{
		{
			ID:        "",
			Name:      "Legacy Missing ID",
			Hash:      "legacy-hash",
			Prefix:    "legacy",
			Suffix:    "hash",
			CreatedAt: time.Now().UTC(),
			Scopes:    []string{ScopeWildcard},
			OrgID:     "default",
		},
	}
	require.NoError(t, p.SaveAPITokens(legacy))

	cfg, err := Load()
	require.NoError(t, err)
	require.Len(t, cfg.APITokens, 1)
	assert.NotEmpty(t, cfg.APITokens[0].ID)

	persisted, err := p.LoadAPITokens()
	require.NoError(t, err)
	require.Len(t, persisted, 1)
	assert.NotEmpty(t, persisted[0].ID)
}

func TestLoad_MockEnv(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".env"), []byte(`PULSE_MOCK_TEST="true"`), 0644))

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

// A deployment that configures a role header but never names an admin role must
// still get the documented default. Leaving ProxyAuthAdminRole empty made
// CheckProxyAuth skip role gating entirely, so every proxied user read and wrote
// admin-only settings.
func TestLoad_ProxyAuthAdminRoleDefaultsWhenUnset(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PROXY_AUTH_SECRET", "secret")
	t.Setenv("PROXY_AUTH_USER_HEADER", "X-User")
	t.Setenv("PROXY_AUTH_ROLE_HEADER", "X-Roles")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, DefaultProxyAuthAdminRole, cfg.ProxyAuthAdminRole)
}

func TestLoad_ProxyAuthAdminRoleEnvOverridesDefault(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PROXY_AUTH_SECRET", "secret")
	t.Setenv("PROXY_AUTH_USER_HEADER", "X-User")
	t.Setenv("PROXY_AUTH_ROLE_HEADER", "X-Roles")
	t.Setenv("PROXY_AUTH_ADMIN_ROLE", "pulse-admins")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "pulse-admins", cfg.ProxyAuthAdminRole)
}

func TestLegacyOIDCEnvProvider(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.com")
	t.Setenv("OIDC_CLIENT_ID", "client-id")
	t.Setenv("OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("OIDC_ALLOWED_GROUPS", "admins, operators")
	t.Setenv("OIDC_GROUP_ROLE_MAPPINGS", "admins=admin,operators=viewer")

	provider, ok := LegacyOIDCEnvProvider("https://pulse.example.com")
	require.True(t, ok)
	require.NotNil(t, provider)
	require.NotNil(t, provider.OIDC)
	assert.Equal(t, LegacyOIDCProviderID, provider.ID)
	assert.True(t, provider.RuntimeManaged)
	assert.Equal(t, "https://issuer.com", provider.OIDC.IssuerURL)
	assert.Equal(t, "client-id", provider.OIDC.ClientID)
	assert.Equal(t, "client-secret", provider.OIDC.ClientSecret)
	assert.Equal(t, "https://pulse.example.com/api/oidc/callback", provider.OIDC.RedirectURL)
	assert.Equal(t, []string{"admins", "operators"}, provider.AllowedGroups)
	assert.Equal(t, map[string]string{"admins": "admin", "operators": "viewer"}, provider.GroupRoleMappings)
	assert.True(t, provider.OIDC.EnvOverrides["clientSecret"])
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

func TestLoad_AuthPass_HashFailureFailsClosed(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PULSE_AUTH_PASS", "plainpass")

	originalHashPasswordFn := hashPasswordFn
	hashPasswordFn = func(string) (string, error) {
		return "", errors.New("boom")
	}
	t.Cleanup(func() {
		hashPasswordFn = originalHashPasswordFn
	})

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.ErrorContains(t, err, "hash PULSE_AUTH_PASS")
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

func TestLoad_DisablesAutoUpdatesForRCChannel(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	p := NewConfigPersistence(tempDir)
	require.NoError(t, p.SaveSystemSettings(SystemSettings{
		UpdateChannel:     "rc",
		AutoUpdateEnabled: true,
	}))

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "rc", cfg.UpdateChannel)
	assert.False(t, cfg.AutoUpdateEnabled)
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

// The dead autoUpdateCheckInterval / autoUpdateTime settings were removed from
// SystemSettings (#1643/#1637 triage): nothing ever consumed them — the
// update schedule is owned by the install.sh-rendered systemd timer. Boxes
// that persisted the fields before the removal must keep loading cleanly,
// with the legacy keys ignored and the real update settings still honored.
func TestLoad_IgnoresLegacyAutoUpdateScheduleFields(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)

	systemJSON := `{"autoUpdateEnabled":true,"updateChannel":"stable","autoUpdateCheckInterval":24,"autoUpdateTime":"03:00"}`
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "system.json"), []byte(systemJSON), 0600))

	cfg, err := Load()
	require.NoError(t, err)

	assert.True(t, cfg.AutoUpdateEnabled, "autoUpdateEnabled from a legacy system.json must still apply")
	assert.Equal(t, "stable", cfg.UpdateChannel)
}

func captureConfigLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	origLogger := log.Logger
	origLevel := zerolog.GlobalLevel()
	log.Logger = zerolog.New(&buf).Level(zerolog.DebugLevel)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	t.Cleanup(func() {
		log.Logger = origLogger
		zerolog.SetGlobalLevel(origLevel)
	})

	return &buf
}

// A system.json that exists but cannot be read must not be confused with a
// missing one: the failure has to be logged, the run continues on defaults,
// and the file on disk must not be replaced with a default system.json.
func TestLoad_SystemSettingsReadFailureWarnsAndKeepsFileIntact(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)

	// A directory named system.json makes the read fail with an error that
	// is not os.IsNotExist — the same shape as a transient read failure.
	require.NoError(t, os.Mkdir(filepath.Join(dataDir, "system.json"), 0o700))

	prevDelay := systemSettingsRetryDelay
	systemSettingsRetryDelay = 0
	t.Cleanup(func() { systemSettingsRetryDelay = prevDelay })

	logOutput := captureConfigLogs(t)

	cfg, err := LoadWithoutLoggingInit()
	require.NoError(t, err)
	assert.True(t, cfg.TemperatureMonitoringEnabled, "run should continue on defaults")

	assert.Contains(t, logOutput.String(), "Failed to load system settings",
		"a failing system settings read must be logged, not skipped silently")
	assert.Contains(t, logOutput.String(), filepath.Join(dataDir, "system.json"),
		"the warning should name the settings file")
	assert.NotContains(t, logOutput.String(), "No system.json found",
		"a read failure must not take the missing-file create-default path")

	info, err := os.Stat(filepath.Join(dataDir, "system.json"))
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "system.json on disk must be left untouched")
}

func TestLoadSystemSettingsWithRetry(t *testing.T) {
	prevDelay := systemSettingsRetryDelay
	systemSettingsRetryDelay = 0
	t.Cleanup(func() { systemSettingsRetryDelay = prevDelay })

	t.Run("transient_failure_recovers_on_retry", func(t *testing.T) {
		want := &SystemSettings{PVEPollingInterval: 42}
		calls := 0
		settings, err := loadSystemSettingsWithRetry(func() (*SystemSettings, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("transient read failure")
			}
			return want, nil
		})
		require.NoError(t, err)
		assert.Equal(t, 2, calls)
		assert.Same(t, want, settings)
	})

	t.Run("persistent_failure_surfaces_error", func(t *testing.T) {
		calls := 0
		settings, err := loadSystemSettingsWithRetry(func() (*SystemSettings, error) {
			calls++
			return nil, errors.New("still failing")
		})
		require.Error(t, err)
		assert.Nil(t, settings)
		assert.Equal(t, 2, calls, "exactly one retry")
	})

	t.Run("success_does_not_retry", func(t *testing.T) {
		calls := 0
		_, err := loadSystemSettingsWithRetry(func() (*SystemSettings, error) {
			calls++
			return nil, nil
		})
		require.NoError(t, err)
		assert.Equal(t, 1, calls)
	})
}
