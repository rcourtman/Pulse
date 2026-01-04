package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_EnvLoadErrors(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	// 1. Create a directory named .env to cause Read error
	err := os.Mkdir(filepath.Join(tempDir, ".env"), 0755)
	require.NoError(t, err)

	// 2. Mock env load errors
	// We need to be in a temp dir for mock.env
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	err = os.Mkdir("mock.env", 0755)
	require.NoError(t, err)
	err = os.Mkdir("mock.env.local", 0755)
	require.NoError(t, err)

	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestLoad_EnvOverrides_Invalid_Extra(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("BACKUP_POLLING_CYCLES", "-1")
	t.Setenv("BACKUP_POLLING_INTERVAL", "-5s")
	t.Setenv("PVE_POLLING_INTERVAL", "5s") // too low
	t.Setenv("ENABLE_TEMPERATURE_MONITORING", "not-a-bool")

	cfg, err := Load()
	assert.NoError(t, err)

	// Should use defaults
	assert.Equal(t, 10, cfg.BackupPollingCycles)
	assert.Equal(t, float64(0), cfg.BackupPollingInterval.Seconds())
	assert.Equal(t, 10.0, cfg.PVEPollingInterval.Seconds())
	assert.True(t, cfg.TemperatureMonitoringEnabled)
}

func TestLoad_EnvOverrides_Seconds(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("BACKUP_POLLING_INTERVAL", "30")
	t.Setenv("PVE_POLLING_INTERVAL", "45")

	cfg, err := Load()
	assert.NoError(t, err)

	assert.Equal(t, 30.0, cfg.BackupPollingInterval.Seconds())
	assert.Equal(t, 45.0, cfg.PVEPollingInterval.Seconds())
}

func TestLoad_EnvOverrides_More(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PULSE_ENABLE_SENSOR_PROXY", "true")
	t.Setenv("PULSE_AUTH_HIDE_LOCAL_LOGIN", "true")
	t.Setenv("PULSE_DISABLE_DOCKER_UPDATE_ACTIONS", "true")
	t.Setenv("ENABLE_BACKUP_POLLING", "0")
	t.Setenv("ADAPTIVE_POLLING_ENABLED", "on")
	t.Setenv("GUEST_METADATA_MIN_REFRESH_INTERVAL", "1s")
	t.Setenv("GUEST_METADATA_REFRESH_JITTER", "500ms")
	t.Setenv("GUEST_METADATA_RETRY_BACKOFF", "2s")
	t.Setenv("GUEST_METADATA_MAX_CONCURRENT", "5")
	t.Setenv("DNS_CACHE_TIMEOUT", "1m")

	cfg, err := Load()
	assert.NoError(t, err)

	assert.True(t, cfg.EnableSensorProxy)
	assert.True(t, cfg.HideLocalLogin)
	assert.True(t, cfg.DisableDockerUpdateActions)
	assert.False(t, cfg.EnableBackupPolling)
	assert.True(t, cfg.AdaptivePollingEnabled)
	assert.Equal(t, 1*time.Second, cfg.GuestMetadataMinRefreshInterval)
	assert.Equal(t, 500*time.Millisecond, cfg.GuestMetadataRefreshJitter)
	assert.Equal(t, 2*time.Second, cfg.GuestMetadataRetryBackoff)
	assert.Equal(t, 5, cfg.GuestMetadataMaxConcurrent)
	assert.Equal(t, 1*time.Minute, cfg.DNSCacheTimeout)
}

func TestLoad_EnvOverrides_AdaptivePolling_Intervals(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("ADAPTIVE_POLLING_BASE_INTERVAL", "30s")
	t.Setenv("ADAPTIVE_POLLING_MIN_INTERVAL", "10s")
	t.Setenv("ADAPTIVE_POLLING_MAX_INTERVAL", "10m")

	cfg, err := Load()
	assert.NoError(t, err)

	assert.Equal(t, 30*time.Second, cfg.AdaptivePollingBaseInterval)
	assert.Equal(t, 10*time.Second, cfg.AdaptivePollingMinInterval)
	assert.Equal(t, 10*time.Minute, cfg.AdaptivePollingMaxInterval)
}

func TestLoad_EnvOverrides_Invalid_Negative(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("GUEST_METADATA_MIN_REFRESH_INTERVAL", "-1s")
	t.Setenv("GUEST_METADATA_REFRESH_JITTER", "-500ms")
	t.Setenv("GUEST_METADATA_RETRY_BACKOFF", "-1s")
	t.Setenv("GUEST_METADATA_MAX_CONCURRENT", "-5")

	cfg, err := Load()
	assert.NoError(t, err)

	// Should use defaults
	assert.Equal(t, DefaultGuestMetadataMinRefresh, cfg.GuestMetadataMinRefreshInterval)
	assert.Equal(t, DefaultGuestMetadataRefreshJitter, cfg.GuestMetadataRefreshJitter)
	assert.Equal(t, DefaultGuestMetadataRetryBackoff, cfg.GuestMetadataRetryBackoff)
	assert.Equal(t, DefaultGuestMetadataMaxConcurrent, cfg.GuestMetadataMaxConcurrent)
}
