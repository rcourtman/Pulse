package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

var (
	watcherOsStat   = os.Stat
	watcherOsGetenv = os.Getenv
)

// Debounce durations for fsnotify events. Package vars so tests can override to avoid real sleeps.
var (
	debounceEnvWrite       = 500 * time.Millisecond
	debounceAPITokensWrite = 250 * time.Millisecond
)

// ConfigWatcher monitors the .env file for changes and updates runtime config
type ConfigWatcher struct {
	config               *Config
	envPath              string
	apiTokensPath        string
	watcher              *fsnotify.Watcher
	stopChan             chan struct{}
	stopOnce             sync.Once // Ensures Stop() can only close channel once
	lastModTime          time.Time
	lastEnvHash          string
	apiTokensLastModTime time.Time
	mu                   sync.RWMutex
	onMockReload         func() // Callback to trigger backend restart when PULSE_MOCK_* changes
	onAPITokenReload     func() // Callback when API tokens are reloaded from disk
	pollInterval         time.Duration
}

// NewConfigWatcher creates a new config watcher
func NewConfigWatcher(config *Config) (*ConfigWatcher, error) {
	persistentDataDir := resolveWatcherDataDir(config)
	dataDir := strings.TrimSpace(watcherOsGetenv("PULSE_DATA_DIR"))
	configPath := ""
	dataPath := ""
	if config != nil {
		configPath = config.ConfigPath
		dataPath = config.DataPath
	}

	envPath := filepath.Join(persistentDataDir, ".env")

	logEvent := log.Info().
		Str("watchingPath", envPath).
		Str("authConfigDir", persistentDataDir).
		Str("pulseDataDir", dataDir).
		Str("configPathFromConfig", configPath).
		Str("dataPathFromConfig", dataPath)
	if authDir := strings.TrimSpace(watcherOsGetenv("PULSE_AUTH_CONFIG_DIR")); authDir != "" {
		logEvent = logEvent.Str("pathAuthority", "PULSE_AUTH_CONFIG_DIR")
	} else if strings.TrimSpace(configPath) != "" {
		logEvent = logEvent.Str("pathAuthority", "config.ConfigPath")
	} else if strings.TrimSpace(dataPath) != "" {
		logEvent = logEvent.Str("pathAuthority", "config.DataPath")
	} else if dataDir != "" {
		logEvent = logEvent.Str("pathAuthority", "PULSE_DATA_DIR")
	} else {
		logEvent = logEvent.Str("pathAuthority", "defaultDataDir")
	}
	logEvent.Msg("Config watcher initialized")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	apiTokensPath := filepath.Join(filepath.Dir(envPath), "api_tokens.json")

	cw := &ConfigWatcher{
		config:        config,
		envPath:       envPath,
		apiTokensPath: apiTokensPath,
		watcher:       watcher,
		stopChan:      make(chan struct{}),
		pollInterval:  5 * time.Second,
	}

	// Get initial mod times and hash
	if stat, err := watcherOsStat(envPath); err == nil {
		cw.lastModTime = stat.ModTime()
		if content, err := os.ReadFile(envPath); err == nil {
			hash := sha256.Sum256(content)
			cw.lastEnvHash = hex.EncodeToString(hash[:])
		}
	}
	if stat, err := watcherOsStat(apiTokensPath); err == nil {
		cw.apiTokensLastModTime = stat.ModTime()
	}

	return cw, nil
}

func resolveWatcherDataDir(config *Config) string {
	if authDir := strings.TrimSpace(watcherOsGetenv("PULSE_AUTH_CONFIG_DIR")); authDir != "" {
		log.Info().Str("authConfigDir", authDir).Msg("Using PULSE_AUTH_CONFIG_DIR for auth config")
		return authDir
	}
	if config != nil {
		if configPath := strings.TrimSpace(config.ConfigPath); configPath != "" {
			return configPath
		}
		if dataPath := strings.TrimSpace(config.DataPath); dataPath != "" {
			return dataPath
		}
	}
	return ResolveRuntimeDataDir(watcherOsGetenv("PULSE_DATA_DIR"))
}

// SetMockReloadCallback sets the callback function to trigger when PULSE_MOCK_* settings change.
func (cw *ConfigWatcher) SetMockReloadCallback(callback func()) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.onMockReload = callback
}

// SetAPITokenReloadCallback sets the callback function to trigger when API tokens are reloaded.
// This allows the Monitor to rebuild token-to-agent bindings after tokens change on disk.
func (cw *ConfigWatcher) SetAPITokenReloadCallback(callback func()) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.onAPITokenReload = callback
}

// Start begins watching the config file
func (cw *ConfigWatcher) Start() error {
	// Watch the directory for .env
	dir := filepath.Dir(cw.envPath)
	err := cw.watcher.Add(dir)
	if err != nil {
		log.Warn().Err(err).Str("path", dir).Msg("Failed to watch config directory")
	}

	if err != nil {
		log.Warn().
			Err(err).
			Str("env_path", cw.envPath).
			Dur("poll_interval", cw.pollInterval).
			Msg("Falling back to polling for config changes")
		go cw.pollForChanges()
		return nil
	}

	go cw.watchForChanges()
	log.Info().Str("env_path", cw.envPath).Str("api_tokens_path", cw.apiTokensPath).Msg("Started watching config files for changes")
	return nil
}

// Stop stops the config watcher
func (cw *ConfigWatcher) Stop() {
	// Use sync.Once to prevent double-close panic
	cw.stopOnce.Do(func() {
		close(cw.stopChan)
		cw.watcher.Close()
	})
}

// ReloadConfig manually triggers a config reload (e.g., from SIGHUP)
func (cw *ConfigWatcher) ReloadConfig() {
	cw.reloadConfig()
}

// watchForChanges handles fsnotify events
func (cw *ConfigWatcher) watchForChanges() {
	cw.handleEvents(cw.watcher.Events, cw.watcher.Errors)
}

// handleEvents processes events from the watcher channels
func (cw *ConfigWatcher) handleEvents(events <-chan fsnotify.Event, errors <-chan error) {
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}

			// Check if the event is for our .env file
			if filepath.Base(event.Name) == ".env" || event.Name == cw.envPath {
				// Debounce - wait a bit for write to complete
				if !cw.waitForDuration(debounceEnvWrite) {
					return
				}

				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					// Check if content actually changed to prevent restart loops on touch
					newHash, err := cw.calculateFileHash(cw.envPath)
					if err == nil {
						if newHash == cw.lastEnvHash {
							log.Debug().Msg("Detected .env file touch but content unchanged, skipping reload")
							continue
						}
						cw.lastEnvHash = newHash
					}

					log.Info().Str("event", event.Op.String()).Msg("Detected .env file change")
					cw.reloadConfig()
				}
			}

			if cw.apiTokensPath != "" && (filepath.Base(event.Name) == filepath.Base(cw.apiTokensPath) || event.Name == cw.apiTokensPath) {
				// Debounce - wait longer for atomic file operations to complete
				// (write to .tmp, rename to final file)
				if !cw.waitForDuration(debounceAPITokensWrite) {
					return
				}

				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					log.Info().Str("event", event.Op.String()).Msg("Detected API token file change")
					cw.reloadAPITokens()
				}
			}

		case err, ok := <-errors:
			if !ok {
				return
			}
			log.Error().
				Err(err).
				Str("env_path", cw.envPath).
				Str("api_tokens_path", cw.apiTokensPath).
				Msg("Config watcher error")

		case <-cw.stopChan:
			return
		}
	}
}

// waitForDuration blocks for d unless shutdown is requested first.
func (cw *ConfigWatcher) waitForDuration(d time.Duration) bool {
	if d <= 0 {
		select {
		case <-cw.stopChan:
			return false
		default:
			return true
		}
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-cw.stopChan:
		return false
	}
}

// pollForChanges is a fallback that polls for changes
func (cw *ConfigWatcher) pollForChanges() {
	ticker := time.NewTicker(cw.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check .env
			if stat, err := os.Stat(cw.envPath); err == nil {
				if stat.ModTime().After(cw.lastModTime) {
					cw.lastModTime = stat.ModTime()
					// Check if content actually changed to prevent restart loops on touch
					newHash, err := cw.calculateFileHash(cw.envPath)
					if err == nil {
						if newHash == cw.lastEnvHash {
							log.Debug().Msg("Detected .env file touch but content unchanged (polling), skipping reload")
							continue
						}
						cw.lastEnvHash = newHash
					}
					log.Info().Msg("Detected .env file change via polling")
					cw.reloadConfig()
				}
			}

			if cw.apiTokensPath != "" {
				if stat, err := os.Stat(cw.apiTokensPath); err == nil {
					if stat.ModTime().After(cw.apiTokensLastModTime) {
						log.Info().Msg("Detected API token file change via polling")
						cw.apiTokensLastModTime = stat.ModTime()
						cw.reloadAPITokens()
					}
				}
			}

		case <-cw.stopChan:
			return
		}
	}
}

// calculateFileHash returns the SHA256 hash of a file
func (cw *ConfigWatcher) calculateFileHash(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:]), nil
}

// reloadConfig reloads the config from the .env file
func (cw *ConfigWatcher) reloadConfig() {
	// Load the .env file
	envMap, err := godotenv.Read(cw.envPath)
	if err != nil {
		// File might not exist, which is fine (no auth)
		if !os.IsNotExist(err) {
			log.Error().Err(err).Str("env_path", cw.envPath).Msg("Failed to read .env file")
			return
		}
		envMap = make(map[string]string)
	}

	// Track what changed
	var changes []string

	cw.mu.Lock()
	callback := cw.onMockReload
	defer cw.mu.Unlock()

	// Update auth settings
	oldAuthUser := cw.config.AuthUser
	oldAuthPass := cw.config.AuthPass
	oldMockEnv := currentMockEnv()
	newMockEnv := extractMockEnv(envMap)

	// Apply auth user
	Mu.Lock()
	newUser := strings.Trim(envMap["PULSE_AUTH_USER"], "'\"")
	if newUser != oldAuthUser {
		cw.config.AuthUser = newUser
		if newUser == "" {
			changes = append(changes, "auth user removed")
		} else if oldAuthUser == "" {
			changes = append(changes, "auth user added")
		} else {
			changes = append(changes, "auth user updated")
		}
	}

	// Apply auth password
	newPass := strings.Trim(envMap["PULSE_AUTH_PASS"], "'\"")
	if newPass != oldAuthPass {
		cw.config.AuthPass = newPass
		if newPass == "" {
			changes = append(changes, "auth password removed")
		} else if oldAuthPass == "" {
			changes = append(changes, "auth password added")
		} else {
			changes = append(changes, "auth password updated")
		}
	}
	Mu.Unlock()

	mockChanged := applyMockEnv(newMockEnv, oldMockEnv)
	if mockChanged {
		changes = append(changes, "mock runtime updated")
	}

	// REMOVED: POLLING_INTERVAL from .env - now ONLY in system.json
	// This prevents confusion and ensures single source of truth

	// Log changes
	if len(changes) > 0 {
		log.Info().
			Strs("changes", changes).
			Bool("has_auth", cw.config.AuthUser != "" && cw.config.AuthPass != "").
			Bool("has_token", cw.config.HasAPITokens()).
			Msg("Applied .env file changes to runtime config")
	} else {
		log.Debug().Msg("No relevant changes detected in .env file")
	}

	if mockChanged && callback != nil {
		log.Info().Str("env_path", cw.envPath).Msg("Triggering backend reload due to .env mock setting change")
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().
						Interface("panic", r).
						Stack().
						Msg("Recovered from panic in .env mock reload callback")
				}
			}()
			callback()
		}()
	}
}

func (cw *ConfigWatcher) reloadAPITokens() {
	cw.mu.Lock()
	callback := cw.onAPITokenReload
	cw.mu.Unlock()

	cw.mu.Lock()
	defer cw.mu.Unlock()

	if globalPersistence == nil {
		log.Warn().Str("api_tokens_path", cw.apiTokensPath).Msg("Config persistence unavailable; cannot reload API tokens")
		return
	}

	// Preserve existing tokens in case reload fails
	existingTokens := cw.config.APITokens
	existingCount := len(existingTokens)

	// Retry logic to handle temporary file system issues
	var tokens []APITokenRecord
	var err error
	maxRetries := 3
	retryDelay := 50 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		tokens, err = globalPersistence.LoadAPITokens()
		if err == nil {
			break
		}

		if attempt < maxRetries {
			log.Warn().
				Err(err).
				Int("attempt", attempt).
				Int("maxRetries", maxRetries).
				Dur("retryDelay", retryDelay).
				Str("api_tokens_path", cw.apiTokensPath).
				Msg("Failed to reload API tokens, retrying...")
			if !cw.waitForDuration(retryDelay) {
				return
			}
			retryDelay *= 2 // Exponential backoff
		}
	}

	if err != nil {
		log.Error().
			Err(err).
			Int("existingTokens", existingCount).
			Str("api_tokens_path", cw.apiTokensPath).
			Msg("Failed to reload API tokens after retries - preserving existing tokens")
		// CRITICAL: Keep existing tokens rather than clearing them
		return
	}

	persistMutations := false
	if migrated := bindMissingAPITokenIDs(tokens); migrated > 0 {
		persistMutations = true
		log.Warn().
			Int("count", migrated).
			Str("api_tokens_path", cw.apiTokensPath).
			Msg("Migrated API tokens missing IDs during reload")
	}

	if migrated := bindLegacyAPITokensToDefault(tokens); migrated > 0 {
		persistMutations = true
		log.Warn().
			Int("count", migrated).
			Str("api_tokens_path", cw.apiTokensPath).
			Msg("Migrated legacy API tokens to default organization binding during reload")
	}

	if persistMutations {
		if err := globalPersistence.SaveAPITokens(tokens); err != nil {
			log.Error().
				Err(err).
				Str("api_tokens_path", cw.apiTokensPath).
				Msg("Failed to persist API token migrations during reload")
		}
	}

	// Only update if we successfully loaded tokens
	Mu.Lock()
	cw.config.APITokens = tokens
	cw.config.SortAPITokens()
	Mu.Unlock()

	if cw.apiTokensPath != "" {
		if stat, err := os.Stat(cw.apiTokensPath); err == nil {
			cw.apiTokensLastModTime = stat.ModTime()
		}
	}

	log.Info().
		Int("count", len(tokens)).
		Int("previousCount", existingCount).
		Msg("Reloaded API tokens from disk")

	// Notify Monitor to rebuild token bindings from current state.
	// This ensures agent bindings remain consistent after token list changes.
	if callback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().
						Interface("panic", r).
						Stack().
						Msg("Recovered from panic in API token reload callback")
				}
			}()
			callback()
		}()
	}
}

func currentMockEnv() map[string]string {
	mockEnv := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !strings.HasPrefix(key, "PULSE_MOCK_") {
			continue
		}
		mockEnv[key] = value
	}
	return mockEnv
}

func extractMockEnv(envMap map[string]string) map[string]string {
	mockEnv := make(map[string]string)
	for key, value := range envMap {
		if strings.HasPrefix(key, "PULSE_MOCK_") {
			mockEnv[key] = value
		}
	}
	return mockEnv
}

func applyMockEnv(next map[string]string, current map[string]string) bool {
	changed := false
	seen := make(map[string]struct{}, len(next))
	for key, value := range next {
		seen[key] = struct{}{}
		if currentValue, ok := current[key]; !ok || currentValue != value {
			_ = os.Setenv(key, value)
			changed = true
		}
	}
	for key := range current {
		if _, ok := seen[key]; ok {
			continue
		}
		if err := os.Unsetenv(key); err == nil {
			changed = true
		}
	}
	if changed {
		keys := make([]string, 0, len(next))
		for key := range next {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		log.Debug().Strs("variables", keys).Msg("Applied .env mock variables")
	}
	return changed
}
