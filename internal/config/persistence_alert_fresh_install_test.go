package config

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

// A fresh install has no alerts.json. It must load the alert manager's
// factory configuration rather than a copy declared in persistence: the copy
// left out Docker container thresholds, so container CPU, memory and disk
// alerts started Off and the settings page called them Custom.
func TestLoadAlertConfigFreshInstallUsesFactoryConfig(t *testing.T) {
	persistence := NewConfigPersistence(t.TempDir())

	loaded, err := persistence.LoadAlertConfig()
	require.NoError(t, err)
	factory := alerts.DefaultAlertConfig()
	factory.ActivationState = ""
	require.Equal(t, factory, *loaded)

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	defer manager.Stop()
	manager.UpdateConfig(*loaded)

	applied := manager.GetConfig()
	require.Equal(t, alerts.ActivationPending, applied.ActivationState)
	docker := applied.DockerDefaults
	require.Equal(t, alerts.HysteresisThreshold{Trigger: 80, Clear: 75}, docker.CPU)
	require.Equal(t, alerts.HysteresisThreshold{Trigger: 85, Clear: 80}, docker.Memory)
	require.Equal(t, alerts.HysteresisThreshold{Trigger: 85, Clear: 80}, docker.Disk)
}

// Node updates and imports reapply LoadAlertConfig to the running manager. A
// missing alerts.json holds no activation decision, so it must not send an
// active install back to pending review.
func TestLoadAlertConfigMissingFileKeepsRunningActivation(t *testing.T) {
	persistence := NewConfigPersistence(t.TempDir())

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	defer manager.Stop()
	active := manager.GetConfig()
	activatedAt := time.Now().Add(-time.Hour)
	active.ActivationState = alerts.ActivationActive
	active.ActivationTime = &activatedAt
	manager.UpdateConfig(active)

	loaded, err := persistence.LoadAlertConfig()
	require.NoError(t, err)
	manager.UpdateConfig(*loaded)

	applied := manager.GetConfig()
	require.Equal(t, alerts.ActivationActive, applied.ActivationState)
	require.NotNil(t, applied.ActivationTime)
	require.True(t, applied.ActivationTime.Equal(activatedAt))
}

// Trigger 0 is how a saved Off is stored, so a stored all-zero Docker default
// stays Off; only the missing-file path takes factory values.
func TestLoadAlertConfigKeepsStoredDockerDefaultsOff(t *testing.T) {
	persistence := NewConfigPersistence(t.TempDir())

	stored := alerts.DefaultAlertConfig()
	stored.DockerDefaults.CPU = alerts.HysteresisThreshold{}
	stored.DockerDefaults.Memory = alerts.HysteresisThreshold{}
	stored.DockerDefaults.Disk = alerts.HysteresisThreshold{}
	require.NoError(t, persistence.SaveAlertConfig(stored))

	loaded, err := persistence.LoadAlertConfig()
	require.NoError(t, err)

	manager := alerts.NewManagerWithDataDir(t.TempDir())
	defer manager.Stop()
	manager.UpdateConfig(*loaded)

	docker := manager.GetConfig().DockerDefaults
	require.Equal(t, alerts.HysteresisThreshold{}, docker.CPU)
	require.Equal(t, alerts.HysteresisThreshold{}, docker.Memory)
	require.Equal(t, alerts.HysteresisThreshold{}, docker.Disk)
}
