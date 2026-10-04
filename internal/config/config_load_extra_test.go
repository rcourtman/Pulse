package config

import (
	"encoding/base64"
	"net"
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
	// An offline proof guest need not have an outbound route. Exercise the
	// existing dial seam, rather than making coverage depend on that route.
	original := netDial
	t.Cleanup(func() { netDial = original })
	calls := 0
	netDial = func(network, address string) (net.Conn, error) {
		calls++
		require.Equal(t, "udp", network)
		require.Equal(t, "8.8.8.8:80", address)
		return &mockConn{localAddr: &net.UDPAddr{IP: net.ParseIP("192.0.2.44")}}, nil
	}
	require.Equal(t, "192.0.2.44", getOutboundIP())
	require.Equal(t, 1, calls)
}

func TestLoad_Errors(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	// First create encryption key so crypto doesn't fail when it sees .enc files
	// Key must be base64-encoded 32-byte value
	keyPath := filepath.Join(tempDir, ".encryption.key")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i) // Non-zero key
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	require.NoError(t, os.WriteFile(keyPath, []byte(encoded), 0600))

	// 1. Corrupted Nodes
	nodesPath := filepath.Join(tempDir, "nodes.enc")
	require.NoError(t, os.WriteFile(nodesPath, []byte("corrupted"), 0644))

	// 2. Corrupted System
	systemPath := filepath.Join(tempDir, "system.json")
	require.NoError(t, os.WriteFile(systemPath, []byte("{invalid}"), 0644))

	// 3. Corrupted Tokens
	tokensPath := filepath.Join(tempDir, "api_tokens.json")
	require.NoError(t, os.WriteFile(tokensPath, []byte("{invalid}"), 0644))

	// Non-credential files retain their recovery behavior, but a corrupted API
	// token inventory must stop startup rather than silently disable token auth.
	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "load API tokens")
}

func TestLoad_MockEnvErrors(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".env"), []byte("invalid="), 0644))

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
