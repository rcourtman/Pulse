package config

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigWatcher_DirectoryPriority(t *testing.T) {
	tempDir := t.TempDir()
	dir1 := filepath.Join(tempDir, "dir1")
	dir2 := filepath.Join(tempDir, "dir2")

	require.NoError(t, os.MkdirAll(dir1, 0755))
	require.NoError(t, os.MkdirAll(dir2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir1, ".env"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, ".env"), []byte(""), 0644))

	tests := []struct {
		name          string
		cfg           *Config
		authConfigDir string
		dataDir       string
		expectedPath  string
	}{
		{
			name:          "Prefer PULSE_AUTH_CONFIG_DIR",
			cfg:           &Config{ConfigPath: dir2},
			authConfigDir: dir1,
			dataDir:       dir2,
			expectedPath:  filepath.Join(dir1, ".env"),
		},
		{
			name:         "Prefer ConfigPath over PULSE_DATA_DIR",
			cfg:          &Config{ConfigPath: dir1},
			dataDir:      dir2,
			expectedPath: filepath.Join(dir1, ".env"),
		},
		{
			name:         "Prefer DataPath when ConfigPath empty",
			cfg:          &Config{DataPath: dir1},
			dataDir:      dir2,
			expectedPath: filepath.Join(dir1, ".env"),
		},
		{
			name:         "Fallback to PULSE_DATA_DIR",
			cfg:          &Config{},
			dataDir:      dir2,
			expectedPath: filepath.Join(dir2, ".env"),
		},
		{
			name:         "Default fallback uses defaultDataDir",
			cfg:          &Config{},
			expectedPath: filepath.Join(defaultDataDir, ".env"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.authConfigDir != "" {
				t.Setenv("PULSE_AUTH_CONFIG_DIR", tt.authConfigDir)
			} else {
				os.Unsetenv("PULSE_AUTH_CONFIG_DIR")
			}

			if tt.dataDir != "" {
				t.Setenv("PULSE_DATA_DIR", tt.dataDir)
			} else {
				os.Unsetenv("PULSE_DATA_DIR")
			}

			cw, err := NewConfigWatcher(tt.cfg)
			require.NoError(t, err)
			defer cw.Stop()

			assert.Equal(t, tt.expectedPath, cw.envPath)
		})
	}
}

func TestConfigWatcher_ReloadConfig(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	// Ensure temp dir is used
	t.Setenv("PULSE_AUTH_CONFIG_DIR", tempDir)

	cfg := &Config{}
	cw, err := NewConfigWatcher(cfg)
	require.NoError(t, err)

	// Create .env content
	envContent := `PULSE_AUTH_USER="admin"
PULSE_AUTH_PASS="secret"`
	require.NoError(t, os.WriteFile(envPath, []byte(envContent), 0644))

	// Reload
	cw.reloadConfig()

	// Assert
	assert.Equal(t, "admin", cfg.AuthUser)
	assert.Equal(t, "secret", cfg.AuthPass)

	// Test update
	envContentUpdated := `PULSE_AUTH_USER="newadmin"
PULSE_AUTH_PASS="newsecret"`
	require.NoError(t, os.WriteFile(envPath, []byte(envContentUpdated), 0644))

	cw.reloadConfig()

	assert.Equal(t, "newadmin", cfg.AuthUser)
	assert.Equal(t, "newsecret", cfg.AuthPass)
}

func TestConfigWatcher_ReloadConfig_MockSettings(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	t.Setenv("PULSE_AUTH_CONFIG_DIR", tempDir)

	cfg := &Config{}
	cw, err := NewConfigWatcher(cfg)
	require.NoError(t, err)

	var callbackCalled atomic.Bool
	cw.SetMockReloadCallback(func() {
		callbackCalled.Store(true)
	})

	require.NoError(t, os.WriteFile(envPath, []byte(`PULSE_MOCK_TEST="true"`), 0644))
	cw.reloadConfig()
	assert.Equal(t, "true", os.Getenv("PULSE_MOCK_TEST"))

	require.Eventually(t, func() bool { return callbackCalled.Load() }, 1*time.Second, 10*time.Millisecond)
	os.Unsetenv("PULSE_MOCK_TEST")
}

func TestConfigWatcher_ReloadAPITokens(t *testing.T) {
	// Setup persistence
	tempDir := t.TempDir()
	p := NewConfigPersistence(tempDir)

	// Save globalPersistence to restore later
	originalPersistence := globalPersistence
	globalPersistence = p
	defer func() { globalPersistence = originalPersistence }()

	// Setup Watcher
	apiTokensPath := filepath.Join(tempDir, "api_tokens.json")
	cfg := &Config{}
	cw := &ConfigWatcher{
		config:        cfg,
		apiTokensPath: apiTokensPath,
	}

	var callbackCalled atomic.Bool
	cw.SetAPITokenReloadCallback(func() {
		callbackCalled.Store(true)
	})

	// Create API tokens file via persistence to ensure format matches
	tokens := []APITokenRecord{
		{
			ID:        "123",
			Name:      "Test Token",
			Hash:      "hash123",
			Prefix:    "pulse_",
			Suffix:    "123",
			Scopes:    []string{"read"},
			CreatedAt: time.Now(),
		},
	}
	require.NoError(t, p.SaveAPITokens(tokens))

	// Reload
	cw.reloadAPITokens()

	// Assert
	Mu.Lock() // config fields might be accessed under lock in real usage, but here we just read
	assert.Len(t, cfg.APITokens, 1)
	if len(cfg.APITokens) > 0 {
		assert.Equal(t, "Test Token", cfg.APITokens[0].Name)
		assert.Equal(t, "default", cfg.APITokens[0].OrgID)
	}
	Mu.Unlock()

	persisted, err := p.LoadAPITokens()
	require.NoError(t, err)
	require.Len(t, persisted, 1)
	assert.Equal(t, "default", persisted[0].OrgID)

	// Wait for callback
	require.Eventually(t, func() bool { return callbackCalled.Load() }, 1*time.Second, 10*time.Millisecond)
}

func TestConfigWatcher_CalculateFileHash_Error(t *testing.T) {
	cw := &ConfigWatcher{}
	_, err := cw.calculateFileHash("/non/existent/file")
	assert.Error(t, err)
}

func TestConfigWatcher_StartStop(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte(""), 0644))

	t.Setenv("PULSE_AUTH_CONFIG_DIR", tempDir)

	cfg := &Config{}
	cw, err := NewConfigWatcher(cfg)
	require.NoError(t, err)

	// Start
	err = cw.Start()
	require.NoError(t, err)

	// Stop
	cw.Stop()

	// Verify stop channel closed
	select {
	case <-cw.stopChan:
		// Closed
	default:
		t.Error("Stop channel should be closed")
	}
}

func TestConfigWatcher_PollForChanges(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	apiTokensPath := filepath.Join(tempDir, "api_tokens.json")

	// Create initial files
	require.NoError(t, os.WriteFile(envPath, []byte("PULSE_AUTH_USER=\"initial\"\nPULSE_MOCK_TEST=\"1\""), 0644))
	require.NoError(t, os.WriteFile(apiTokensPath, []byte("[]"), 0644))

	// Ensure temp dir is used
	t.Setenv("PULSE_AUTH_CONFIG_DIR", tempDir)

	cfg := &Config{}
	cw, err := NewConfigWatcher(cfg)
	require.NoError(t, err)
	if stat, err := os.Stat(apiTokensPath); err == nil {
		cw.apiTokensLastModTime = stat.ModTime()
	}

	// Set short poll interval
	cw.pollInterval = 10 * time.Millisecond

	// Hook up callbacks
	var mockCalled atomic.Bool
	var tokenCalled atomic.Bool

	cw.SetMockReloadCallback(func() { mockCalled.Store(true) })
	cw.SetAPITokenReloadCallback(func() { tokenCalled.Store(true) })

	// Mock global persistence for API token reloads
	p := NewConfigPersistence(tempDir)
	originalPersistence := globalPersistence
	globalPersistence = p
	defer func() { globalPersistence = originalPersistence }()

	// Run pollForChanges in background
	go cw.pollForChanges()
	defer cw.Stop()

	// 1. Update .env
	require.NoError(t, os.WriteFile(envPath, []byte(`PULSE_AUTH_USER="updated"`), 0644))
	// Avoid relying on filesystem timestamp granularity.
	future := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(envPath, future, future))

	require.Eventually(t, func() bool {
		Mu.RLock()
		defer Mu.RUnlock()
		return cfg.AuthUser == "updated"
	}, 1*time.Second, 10*time.Millisecond)

	// 2. Update .env mock settings
	require.NoError(t, os.WriteFile(envPath, []byte("PULSE_AUTH_USER=\"updated\"\nPULSE_MOCK_TEST=\"2\""), 0644))
	future = future.Add(2 * time.Second)
	require.NoError(t, os.Chtimes(envPath, future, future))

	require.Eventually(t, func() bool { return mockCalled.Load() }, 1*time.Second, 10*time.Millisecond)
	assert.Equal(t, "2", os.Getenv("PULSE_MOCK_TEST"))

	// 3. Update api_tokens.json
	// Write valid JSON
	// We need to write to file that Persistence reads.
	// ReloadAPITokens uses globalPersistence to load.
	tokens := []APITokenRecord{{ID: "new", Hash: "hash", Name: "New"}}
	require.NoError(t, p.SaveAPITokens(tokens))
	future = future.Add(2 * time.Second)
	require.NoError(t, os.Chtimes(apiTokensPath, future, future))

	// Waiting for polling to pick up change in file modification
	// persistence.SaveAPITokens writes to the file.

	require.Eventually(t, func() bool { return tokenCalled.Load() }, 1*time.Second, 10*time.Millisecond)

	Mu.RLock()
	defer Mu.RUnlock()
	assert.Len(t, cfg.APITokens, 1)
}

func TestConfigWatcher_ReloadConfig_APITokensIgnored(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	// Ensure temp dir is used
	t.Setenv("PULSE_AUTH_CONFIG_DIR", tempDir)

	cfg := &Config{
		APITokens: []APITokenRecord{{ID: "id", Hash: "hash"}},
	}
	cw, err := NewConfigWatcher(cfg)
	require.NoError(t, err)

	// API_TOKEN/API_TOKENS are ignored by watcher reloads in strict v6 mode.
	envContent := `API_TOKEN="token1"
API_TOKENS="token2,token3"`
	require.NoError(t, os.WriteFile(envPath, []byte(envContent), 0644))

	cw.reloadConfig()

	assert.Len(t, cfg.APITokens, 1)
	assert.Equal(t, "hash", cfg.APITokens[0].Hash)
}

func TestConfigWatcher_ReloadConfig_Auth(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	t.Setenv("PULSE_AUTH_CONFIG_DIR", tempDir)

	cfg := &Config{
		AuthUser: "oldUser",
		AuthPass: "oldPass",
	}
	cw, err := NewConfigWatcher(cfg)
	require.NoError(t, err)

	// Update auth
	envContent := `PULSE_AUTH_USER="newUser"
PULSE_AUTH_PASS="newPass"`
	require.NoError(t, os.WriteFile(envPath, []byte(envContent), 0644))

	cw.reloadConfig()

	assert.Equal(t, "newUser", cfg.AuthUser)
	assert.Equal(t, "newPass", cfg.AuthPass)

	// Remove auth
	require.NoError(t, os.WriteFile(envPath, []byte(""), 0644))
	cw.reloadConfig()

	assert.Equal(t, "", cfg.AuthUser)
	assert.Equal(t, "", cfg.AuthPass)
}

func TestConfigWatcher_ReloadConfig_Manual(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	t.Setenv("PULSE_AUTH_CONFIG_DIR", tempDir)
	require.NoError(t, os.WriteFile(envPath, []byte(`PULSE_AUTH_USER="initial"`), 0644))

	cfg := &Config{}
	cw, err := NewConfigWatcher(cfg)
	require.NoError(t, err)

	// Update file
	require.NoError(t, os.WriteFile(envPath, []byte(`PULSE_AUTH_USER="manual"`), 0644))

	// Manual Trigger
	cw.ReloadConfig()

	assert.Equal(t, "manual", cfg.AuthUser)
}

func TestConfigWatcher_ReloadConfig_RemovesMockSetting(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	t.Setenv("PULSE_AUTH_CONFIG_DIR", tempDir)

	cfg := &Config{}
	cw, err := NewConfigWatcher(cfg)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(envPath, []byte(`PULSE_MOCK_TEST="base"`), 0644))
	cw.reloadConfig()
	require.Equal(t, "base", os.Getenv("PULSE_MOCK_TEST"))

	require.NoError(t, os.WriteFile(envPath, []byte(""), 0644))
	cw.reloadConfig()
	assert.Equal(t, "", os.Getenv("PULSE_MOCK_TEST"))
	os.Unsetenv("PULSE_MOCK_TEST")
}

func TestConfigWatcher_ReloadConfig_MissingEnvRemovesMockSetting(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_AUTH_CONFIG_DIR", tempDir)
	os.Setenv("PULSE_MOCK_TEST", "stale")
	t.Cleanup(func() { os.Unsetenv("PULSE_MOCK_TEST") })

	cw, err := NewConfigWatcher(&Config{})
	require.NoError(t, err)

	cw.reloadConfig()
	assert.Equal(t, "", os.Getenv("PULSE_MOCK_TEST"))
}

func TestConfigWatcher_ReloadAPITokens_Retries(t *testing.T) {
	tempDir := t.TempDir()
	p := NewConfigPersistence(tempDir)

	originalPersistence := globalPersistence
	globalPersistence = p
	defer func() { globalPersistence = originalPersistence }()

	apiTokensPath := filepath.Join(tempDir, "api_tokens.json")
	require.NoError(t, os.WriteFile(apiTokensPath, []byte("{invalid-json"), 0644))

	cfg := &Config{}
	cw := &ConfigWatcher{
		config:        cfg,
		apiTokensPath: apiTokensPath,
	}

	// Should attempt retries and log errors but continue
	cw.reloadAPITokens()
}
