package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"
)

func TestConfigWatcher_WatchForChanges_Live(t *testing.T) {
	origEnv := debounceEnvWrite
	origAPITokens := debounceAPITokensWrite
	debounceEnvWrite = 0
	debounceAPITokensWrite = 0
	t.Cleanup(func() {
		debounceEnvWrite = origEnv
		debounceAPITokensWrite = origAPITokens
	})

	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	apiTokensPath := filepath.Join(tempDir, "api_tokens.json")

	require.NoError(t, os.WriteFile(envPath, []byte("PULSE_AUTH_USER=initial\nPULSE_MOCK_TEST=1"), 0o644))
	require.NoError(t, os.WriteFile(apiTokensPath, []byte("[]"), 0o644))

	t.Setenv("PULSE_AUTH_CONFIG_DIR", tempDir)

	cfg := &Config{}
	cw, err := NewConfigWatcher(cfg)
	require.NoError(t, err)

	mockReloaded := make(chan bool, 1)
	tokensReloaded := make(chan bool, 1)
	cw.SetMockReloadCallback(func() { mockReloaded <- true })
	cw.SetAPITokenReloadCallback(func() { tokensReloaded <- true })

	p := NewConfigPersistence(tempDir)
	originalPersistence := globalPersistence
	globalPersistence = p

	err = cw.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		cw.Stop()
		globalPersistence = originalPersistence
	})

	require.NoError(t, os.WriteFile(envPath, []byte("PULSE_AUTH_USER=something-different\nPULSE_MOCK_TEST=2"), 0o644))

	require.Eventually(t, func() bool {
		Mu.RLock()
		defer Mu.RUnlock()
		return cfg.AuthUser == "something-different"
	}, 5*time.Second, 25*time.Millisecond)

	select {
	case <-mockReloaded:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for mock reload")
	}

	require.Equal(t, "2", os.Getenv("PULSE_MOCK_TEST"))

	require.NoError(t, os.WriteFile(apiTokensPath, []byte("[]"), 0o644))

	select {
	case <-tokensReloaded:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for API token reload")
	}
}

func TestConfigWatcher_WatchForChanges_ErrorHandling(t *testing.T) {
	cw := &ConfigWatcher{stopChan: make(chan struct{})}

	events := make(chan fsnotify.Event)
	errors := make(chan error, 1)
	errors <- os.ErrPermission
	close(errors)
	close(events)

	cw.handleEvents(events, errors)
}

func TestConfigWatcher_HandleEvents_StopDuringDebounce(t *testing.T) {
	origEnv := debounceEnvWrite
	debounceEnvWrite = 5 * time.Second
	t.Cleanup(func() {
		debounceEnvWrite = origEnv
	})

	cw := &ConfigWatcher{
		config:   &Config{},
		envPath:  "/tmp/.env",
		stopChan: make(chan struct{}),
	}

	events := make(chan fsnotify.Event, 1)
	errors := make(chan error)
	done := make(chan struct{})

	go func() {
		cw.handleEvents(events, errors)
		close(done)
	}()

	events <- fsnotify.Event{Name: "/tmp/.env", Op: fsnotify.Write}
	time.Sleep(25 * time.Millisecond)
	close(cw.stopChan)

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("handleEvents did not exit quickly after stop during debounce")
	}
}

func TestConfigWatcher_CalculateFileHash_NotFound(t *testing.T) {
	cw := &ConfigWatcher{}
	hash, err := cw.calculateFileHash("/path/to/nothing")
	require.Error(t, err)
	require.Empty(t, hash)
}
