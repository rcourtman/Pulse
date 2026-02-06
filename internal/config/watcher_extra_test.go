package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigWatcher_WatchForChanges_Live(t *testing.T) {
	origEnv := debounceEnvWrite
	origAPITokens := debounceAPITokensWrite
	origMock := debounceMockWrite
	debounceEnvWrite = 0
	debounceAPITokensWrite = 0
	debounceMockWrite = 0
	t.Cleanup(func() {
		debounceEnvWrite = origEnv
		debounceAPITokensWrite = origAPITokens
		debounceMockWrite = origMock
	})

	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	apiTokensPath := filepath.Join(tempDir, "api_tokens.json")
	mockEnvPath := filepath.Join(tempDir, "mock.env")

	require.NoError(t, os.WriteFile(envPath, []byte("PULSE_AUTH_USER=initial"), 0644))
	require.NoError(t, os.WriteFile(apiTokensPath, []byte("[]"), 0644))
	require.NoError(t, os.WriteFile(mockEnvPath, []byte("PULSE_MOCK_TEST=1"), 0644))

	t.Setenv("PULSE_AUTH_CONFIG_DIR", tempDir)

	cfg := &Config{}
	cw, err := NewConfigWatcher(cfg)
	require.NoError(t, err)
	cw.mockEnvPath = mockEnvPath // Force mock.env path for test

	// Setup callbacks
	mockReloaded := make(chan bool, 1)
	tokensReloaded := make(chan bool, 1)
	cw.SetMockReloadCallback(func() { mockReloaded <- true })
	cw.SetAPITokenReloadCallback(func() { tokensReloaded <- true })

	// Start watching
	err = cw.Start()
	require.NoError(t, err)
	defer cw.Stop()

	// 1. Test .env change
	require.NoError(t, os.WriteFile(envPath, []byte("PULSE_AUTH_USER=something-different"), 0644))

	require.Eventually(t, func() bool {
		Mu.RLock()
		defer Mu.RUnlock()
		return cfg.AuthUser == "something-different"
	}, 5*time.Second, 25*time.Millisecond)

	// 2. Test api_tokens.json change
	// Mock global persistence for API token reloads
	p := NewConfigPersistence(tempDir)
	originalPersistence := globalPersistence
	globalPersistence = p
	defer func() { globalPersistence = originalPersistence }()

	// Write empty tokens list but it MUST be valid JSON
	require.NoError(t, os.WriteFile(apiTokensPath, []byte("[]"), 0644))

	select {
	case <-tokensReloaded:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("Timed out waiting for API token reload")
	}

	// 3. Test mock.env change
	require.NoError(t, os.WriteFile(mockEnvPath, []byte("PULSE_MOCK_TEST=2"), 0644))

	select {
	case <-mockReloaded:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Timed out waiting for mock reload")
	}
}

func TestConfigWatcher_WatchForChanges_ErrorHandling(t *testing.T) {
	cw := &ConfigWatcher{stopChan: make(chan struct{})}

	events := make(chan fsnotify.Event)
	errors := make(chan error, 1)
	errors <- os.ErrPermission
	close(errors)
	close(events)

	// Should return cleanly even when error channel yields values.
	cw.handleEvents(events, errors)
}

func TestConfigWatcher_CalculateFileHash_NotFound(t *testing.T) {
	cw := &ConfigWatcher{}
	hash, err := cw.calculateFileHash("/path/to/nothing")
	assert.Error(t, err)
	assert.Empty(t, hash)
}
