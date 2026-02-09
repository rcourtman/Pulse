package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_EnvOverrides_Comprehensive(t *testing.T) {
	// Clear relevant env vars
	vars := []string{
		"PULSE_DATA_DIR",
		"BACKUP_POLLING_CYCLES",
		"BACKUP_POLLING_INTERVAL",
		"PVE_POLLING_INTERVAL",
		"ENABLE_TEMPERATURE_MONITORING",
		"PULSE_AUTH_HIDE_LOCAL_LOGIN",
		"PULSE_DISABLE_DOCKER_UPDATE_ACTIONS",
		"DISABLE_DOCKER_UPDATE_ACTIONS",
		"PULSE_HIDE_DOCKER_UPDATE_ACTIONS",
		"HIDE_DOCKER_UPDATE_ACTIONS",
		"ENABLE_BACKUP_POLLING",
		"ADAPTIVE_POLLING_ENABLED",
		"ADAPTIVE_POLLING_BASE_INTERVAL",
		"ADAPTIVE_POLLING_MIN_INTERVAL",
		"ADAPTIVE_POLLING_MAX_INTERVAL",
		"GUEST_METADATA_MIN_REFRESH_INTERVAL",
		"GUEST_METADATA_REFRESH_JITTER",
	}
	for _, v := range vars {
		t.Setenv(v, "")
	}

	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	// Set overrides
	t.Setenv("BACKUP_POLLING_CYCLES", "20")
	t.Setenv("BACKUP_POLLING_INTERVAL", "30s")
	t.Setenv("PVE_POLLING_INTERVAL", "15s")
	t.Setenv("ENABLE_TEMPERATURE_MONITORING", "false")
	t.Setenv("PULSE_AUTH_HIDE_LOCAL_LOGIN", "true")
	t.Setenv("PULSE_DISABLE_DOCKER_UPDATE_ACTIONS", "true")
	t.Setenv("ENABLE_BACKUP_POLLING", "false")
	t.Setenv("ADAPTIVE_POLLING_ENABLED", "true")
	t.Setenv("ADAPTIVE_POLLING_BASE_INTERVAL", "20s")
	t.Setenv("ADAPTIVE_POLLING_MIN_INTERVAL", "10s")
	t.Setenv("ADAPTIVE_POLLING_MAX_INTERVAL", "10m")
	t.Setenv("GUEST_METADATA_MIN_REFRESH_INTERVAL", "1m")
	t.Setenv("GUEST_METADATA_REFRESH_JITTER", "5s")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 20, cfg.BackupPollingCycles)
	assert.Equal(t, 30*time.Second, cfg.BackupPollingInterval)
	assert.Equal(t, 15*time.Second, cfg.PVEPollingInterval)
	assert.False(t, cfg.TemperatureMonitoringEnabled)
	assert.True(t, cfg.HideLocalLogin)
	assert.True(t, cfg.DisableDockerUpdateActions)
	assert.False(t, cfg.EnableBackupPolling)
	assert.True(t, cfg.AdaptivePollingEnabled)
	assert.Equal(t, 20*time.Second, cfg.AdaptivePollingBaseInterval)
	assert.Equal(t, 10*time.Second, cfg.AdaptivePollingMinInterval)
	assert.Equal(t, 10*time.Minute, cfg.AdaptivePollingMaxInterval)
	assert.Equal(t, 1*time.Minute, cfg.GuestMetadataMinRefreshInterval)
	assert.Equal(t, 5*time.Second, cfg.GuestMetadataRefreshJitter)
}

func TestLoad_EnvOverrides_InvalidValues(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	// Set invalid overrides
	t.Setenv("BACKUP_POLLING_CYCLES", "abc")
	t.Setenv("BACKUP_POLLING_INTERVAL", "invalid")
	t.Setenv("PVE_POLLING_INTERVAL", "5s") // Below min
	t.Setenv("ENABLE_TEMPERATURE_MONITORING", "maybe")
	t.Setenv("GUEST_METADATA_MIN_REFRESH_INTERVAL", "0s")

	cfg, err := Load()
	require.NoError(t, err)

	// Should fall back to defaults
	assert.Equal(t, 10, cfg.BackupPollingCycles)
	assert.Equal(t, 10*time.Second, cfg.PVEPollingInterval) // Default
	assert.True(t, cfg.TemperatureMonitoringEnabled)        // Default
}

func TestLoad_EnvOverrides_NegativeValues(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	t.Setenv("BACKUP_POLLING_CYCLES", "-5")
	t.Setenv("BACKUP_POLLING_INTERVAL", "-10s")
	t.Setenv("GUEST_METADATA_REFRESH_JITTER", "-1s")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 10, cfg.BackupPollingCycles)
}

func TestLoad_EnvOverrides_BackupPolling_Alternative(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	t.Setenv("ENABLE_BACKUP_POLLING", "0")
	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.EnableBackupPolling)

	t.Setenv("ENABLE_BACKUP_POLLING", "yes")
	cfg, err = Load()
	require.NoError(t, err)
	assert.True(t, cfg.EnableBackupPolling)
}

func TestLoad_EnvOverrides_AdaptivePolling_Alternative(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	t.Setenv("ADAPTIVE_POLLING_ENABLED", "off")
	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.AdaptivePollingEnabled)
}

func TestLoad_EnvOverrides_PBSAndPMG(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	t.Setenv("PBS_POLLING_INTERVAL", "45s")
	t.Setenv("PMG_POLLING_INTERVAL", "120")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 45*time.Second, cfg.PBSPollingInterval)
	assert.Equal(t, 120*time.Second, cfg.PMGPollingInterval)
}

func TestLoad_EnvOverrides_DockerUpdateActionsAliases(t *testing.T) {
	cases := []struct {
		name   string
		envVar string
		value  string
		want   bool
	}{
		{name: "canonical", envVar: "PULSE_DISABLE_DOCKER_UPDATE_ACTIONS", value: "true", want: true},
		{name: "legacy disable", envVar: "DISABLE_DOCKER_UPDATE_ACTIONS", value: "true", want: true},
		{name: "legacy pulse hide", envVar: "PULSE_HIDE_DOCKER_UPDATE_ACTIONS", value: "true", want: true},
		{name: "legacy hide", envVar: "HIDE_DOCKER_UPDATE_ACTIONS", value: "true", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			t.Setenv("PULSE_DATA_DIR", tempDir)
			t.Setenv("PULSE_DISABLE_DOCKER_UPDATE_ACTIONS", "")
			t.Setenv("DISABLE_DOCKER_UPDATE_ACTIONS", "")
			t.Setenv("PULSE_HIDE_DOCKER_UPDATE_ACTIONS", "")
			t.Setenv("HIDE_DOCKER_UPDATE_ACTIONS", "")
			t.Setenv(tc.envVar, tc.value)

			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.DisableDockerUpdateActions)
			assert.True(t, cfg.EnvOverrides["disableDockerUpdateActions"])
		})
	}
}
