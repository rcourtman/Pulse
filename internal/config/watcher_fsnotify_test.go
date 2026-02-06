package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"
)

// TestHandleEvents tests handleEvents with mock channels
func TestHandleEvents(t *testing.T) {
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
	t.Setenv("PULSE_AUTH_CONFIG_DIR", tempDir)
	require.NoError(t, os.WriteFile(envPath, []byte(`PULSE_AUTH_USER="initial"`), 0644))

	cfg := &Config{}
	cw, err := NewConfigWatcher(cfg)
	require.NoError(t, err)

	// Override hash check
	cw.lastEnvHash = "dummy"

	events := make(chan fsnotify.Event, 1)
	errors := make(chan error, 1)

	go cw.handleEvents(events, errors)
	defer cw.Stop()

	// 1. Inject Write event
	require.NoError(t, os.WriteFile(envPath, []byte(`PULSE_AUTH_USER="handled"`), 0644))

	events <- fsnotify.Event{
		Name: envPath,
		Op:   fsnotify.Write,
	}

	require.Eventually(t, func() bool {
		Mu.RLock()
		defer Mu.RUnlock()
		return cfg.AuthUser == "handled"
	}, 2*time.Second, 100*time.Millisecond)

	// 2. Inject Error
	// Just ensure it doesn't panic and logs it (can't easily check log here without hook)
	errors <- parseError("test err")
}

func parseError(s string) error {
	return &testError{s}
}

type testError struct{ s string }

func (e *testError) Error() string { return e.s }
