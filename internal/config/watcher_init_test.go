package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigWatcher_Scenarios(t *testing.T) {
	config := &Config{ConfigPath: "/etc/pulse"}

	// Reset mocks after test
	origStat := watcherOsStat
	origGetenv := watcherOsGetenv
	defer func() {
		watcherOsStat = origStat
		watcherOsGetenv = origGetenv
	}()

	mockEnv := make(map[string]string)
	watcherOsGetenv = func(key string) string {
		return mockEnv[key]
	}
	watcherOsStat = func(name string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	// Option 1: PULSE_AUTH_CONFIG_DIR
	t.Run("PULSE_AUTH_CONFIG_DIR", func(t *testing.T) {
		mockEnv["PULSE_AUTH_CONFIG_DIR"] = "/custom/auth"
		cw, err := NewConfigWatcher(config)
		require.NoError(t, err)
		assert.Equal(t, "/custom/auth/.env", cw.envPath)
		cw.Stop()
	})

	// Option 2: PULSE_DATA_DIR matches production
	t.Run("PULSE_DATA_DIR_production", func(t *testing.T) {
		delete(mockEnv, "PULSE_AUTH_CONFIG_DIR")
		mockEnv["PULSE_DATA_DIR"] = "/etc/pulse"
		cw, err := NewConfigWatcher(config)
		require.NoError(t, err)
		assert.Equal(t, "/etc/pulse/.env", cw.envPath)
		cw.Stop()
	})

	// Option 5: Fallback to PULSE_DATA_DIR
	t.Run("PULSE_DATA_DIR_fallback", func(t *testing.T) {
		delete(mockEnv, "PULSE_AUTH_CONFIG_DIR")
		mockEnv["PULSE_DATA_DIR"] = "/tmp/mock-data"
		cw, err := NewConfigWatcher(config)
		require.NoError(t, err)
		assert.Equal(t, "/tmp/mock-data/.env", cw.envPath)
		cw.Stop()
	})

	// Docker mode
	t.Run("Docker_mode", func(t *testing.T) {
		mockEnv["PULSE_DOCKER"] = "true"
		mockEnv["PULSE_DATA_DIR"] = "/data"
		cw, err := NewConfigWatcher(config)
		require.NoError(t, err)
		assert.Equal(t, "", cw.mockEnvPath)
		cw.Stop()
	})
}

func TestConfigWatcher_Start_Options(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	os.WriteFile(envPath, []byte(""), 0644)

	cfg := &Config{ConfigPath: tempDir}
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cw, err := NewConfigWatcher(cfg)
	require.NoError(t, err)
	defer cw.Stop()

	cw.envPath = envPath
	cw.mockEnvPath = filepath.Join(tempDir, "mock.env")
	os.WriteFile(cw.mockEnvPath, []byte(""), 0644)

	err = cw.Start()
	assert.NoError(t, err)
}

func TestConfigWatcher_PollForChanges_Coverage(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	os.WriteFile(envPath, []byte("V1"), 0644)

	cw := &ConfigWatcher{
		config:       &Config{},
		envPath:      envPath,
		pollInterval: 10 * time.Millisecond,
		stopChan:     make(chan struct{}),
	}

	// Start polling in background
	go cw.pollForChanges()

	// Wait a bit
	time.Sleep(20 * time.Millisecond)

	// change file
	os.WriteFile(envPath, []byte("V2"), 0644)

	time.Sleep(50 * time.Millisecond)
	close(cw.stopChan)
}
