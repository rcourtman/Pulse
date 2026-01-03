package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MoreOverrides(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	// Test discrete values for coverage
	t.Setenv("BACKUP_POLLING_INTERVAL", "60") // seconds
	t.Setenv("PVE_POLLING_INTERVAL", "20")    // seconds
	t.Setenv("ENABLE_BACKUP_POLLING", "off")
	t.Setenv("ADAPTIVE_POLLING_ENABLED", "no")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 60*time.Second, cfg.BackupPollingInterval)
	assert.Equal(t, 20*time.Second, cfg.PVEPollingInterval)
	assert.False(t, cfg.EnableBackupPolling)
	assert.False(t, cfg.AdaptivePollingEnabled)

	// Test durations
	t.Setenv("BACKUP_POLLING_INTERVAL", "2m")
	t.Setenv("PVE_POLLING_INTERVAL", "30s")
	cfg, _ = Load()
	assert.Equal(t, 2*time.Minute, cfg.BackupPollingInterval)
	assert.Equal(t, 30*time.Second, cfg.PVEPollingInterval)
}

func TestLoad_GuestMetadataOverrides(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	t.Setenv("GUEST_METADATA_REFRESH_JITTER", "10s")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, cfg.GuestMetadataRefreshJitter)
}

func TestLoad_OutboundIP(t *testing.T) {
	// Calling getOutboundIP for coverage
	ip := getOutboundIP()
	assert.NotEmpty(t, ip)
}

func TestLoad_Errors(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	// 1. Corrupted Nodes
	nodesPath := filepath.Join(tempDir, "nodes.enc")
	require.NoError(t, os.WriteFile(nodesPath, []byte("corrupted"), 0644))

	// 2. Corrupted System
	systemPath := filepath.Join(tempDir, "system.json")
	require.NoError(t, os.WriteFile(systemPath, []byte("{invalid}"), 0644))

	// 3. Corrupted OIDC
	oidcPath := filepath.Join(tempDir, "oidc.enc")
	require.NoError(t, os.WriteFile(oidcPath, []byte("corrupted"), 0644))

	// 4. Corrupted Tokens
	tokensPath := filepath.Join(tempDir, "api_tokens.json")
	require.NoError(t, os.WriteFile(tokensPath, []byte("{invalid}"), 0644))

	// 5. Corrupted Suppressions
	suppressionsPath := filepath.Join(tempDir, "env_token_suppressions.json")
	require.NoError(t, os.WriteFile(suppressionsPath, []byte("{invalid}"), 0644))

	// Load should still proceed with defaults and log warnings
	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestLoad_MockEnvErrors(t *testing.T) {
	cwd, _ := os.Getwd()
	tempCWD := t.TempDir()
	os.Chdir(tempCWD)
	defer os.Chdir(cwd)

	require.NoError(t, os.WriteFile("mock.env", []byte("invalid="), 0644))
	require.NoError(t, os.WriteFile("mock.env.local", []byte("invalid="), 0644))

	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestLoad_SystemJsonDirError(t *testing.T) {
	tempDir := t.TempDir()
	// Make system.json a directory to trigger SaveSystemSettings error during creation
	systemPath := filepath.Join(tempDir, "system.json")
	require.NoError(t, os.Mkdir(systemPath, 0755))

	t.Setenv("PULSE_DATA_DIR", tempDir)
	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}
