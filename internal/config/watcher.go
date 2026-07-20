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

// ConfigWatcher monitors runtime config files for changes and updates runtime config.
type ConfigWatcher struct {
	config               *Config
	envPath              string
	apiTokensPath        string
	persistence          *ConfigPersistence
	watcher              *fsnotify.Watcher
	stopChan             chan struct{}
	stopOnce             sync.Once // Ensures Stop() can only close channel once
	wg                   sync.WaitGroup
	lastModTime          time.Time
	lastEnvHash          string
	apiTokensLastModTime time.Time
	mu                   sync.RWMutex
	onMockReload         func() // Callback to trigger backend restart when PULSE_MOCK_* changes
	onAPITokenReload     func() // Callback when API tokens are reloaded from disk
	pollInterval         time.Duration
	mockEnvOwned         map[string]struct{}
	mockEnvFallbacks     map[string]mockEnvFallback
}

type mockEnvFallback struct {
	value   string
	present bool
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
	persistence, persistenceErr := newConfigPersistence(persistentDataDir)
	if persistenceErr != nil {
		log.Warn().
			Err(persistenceErr).
			Str("configDir", persistentDataDir).
			Msg("Config watcher persistence unavailable; API token reloads will be disabled")
	}

	cw := &ConfigWatcher{
		config:           config,
		envPath:          envPath,
		apiTokensPath:    apiTokensPath,
		persistence:      persistence,
		watcher:          watcher,
		stopChan:         make(chan struct{}),
		pollInterval:     5 * time.Second,
		mockEnvOwned:     make(map[string]struct{}),
		mockEnvFallbacks: make(map[string]mockEnvFallback),
	}

	// Get initial mod times and hash
	if stat, err := watcherOsStat(envPath); err == nil {
		cw.lastModTime = stat.ModTime()
		if content, err := os.ReadFile(envPath); err == nil {
			hash := sha256.Sum256(content)
			cw.lastEnvHash = hex.EncodeToString(hash[:])
		}
		if envMap, err := godotenv.Read(envPath); err == nil {
			for key := range extractMockEnv(envMap) {
				cw.mockEnvOwned[key] = struct{}{}
			}
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

// Start begins watching the config files.
func (cw *ConfigWatcher) Start() error {
	watched := 0
	for _, path := range []string{cw.envPath, cw.apiTokensPath} {
		ok, err := cw.addFileWatchIfPresent(path)
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("Failed to watch config file")
			continue
		}
		if ok {
			watched++
		}
	}

	if watched > 0 {
		cw.wg.Add(1)
		go func() {
			defer cw.wg.Done()
			cw.watchForChanges()
		}()
		log.Info().
			Str("env_path", cw.envPath).
			Str("api_tokens_path", cw.apiTokensPath).
			Dur("poll_interval", cw.pollInterval).
			Msg("Started watching config files for changes")
	} else {
		log.Info().
			Str("env_path", cw.envPath).
			Str("api_tokens_path", cw.apiTokensPath).
			Dur("poll_interval", cw.pollInterval).
			Msg("Config files not present yet; polling for config changes")
	}

	cw.wg.Add(1)
	go func() {
		defer cw.wg.Done()
		cw.pollForChanges()
	}()
	return nil
}

func (cw *ConfigWatcher) addFileWatchIfPresent(path string) (bool, error) {
	if cw == nil || cw.watcher == nil || strings.TrimSpace(path) == "" {
		return false, nil
	}
	stat, err := watcherOsStat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if stat.IsDir() {
		return false, nil
	}
	if err := cw.watcher.Add(path); err != nil {
		return false, err
	}
	return true, nil
}

func (cw *ConfigWatcher) rewatchFileIfPresent(path string) {
	if cw == nil || cw.watcher == nil || strings.TrimSpace(path) == "" {
		return
	}
	if _, err := watcherOsStat(path); err != nil {
		return
	}
	_ = cw.watcher.Remove(path)
	if err := cw.watcher.Add(path); err != nil {
		log.Debug().Err(err).Str("path", path).Msg("Failed to re-arm config file watch")
	}
}

// Stop stops the config watcher
func (cw *ConfigWatcher) Stop() {
	// Use sync.Once to prevent double-close panic
	cw.stopOnce.Do(func() {
		close(cw.stopChan)
		if cw.watcher != nil {
			_ = cw.watcher.Close()
		}
		cw.wg.Wait()
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
			if configWatcherEventMatchesPath(event, cw.envPath) {
				// Debounce - wait a bit for write to complete
				if !cw.waitForDuration(debounceEnvWrite) {
					return
				}

				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
					cw.rewatchFileIfPresent(cw.envPath)
					// Check if content actually changed to prevent restart loops on touch
					newHash, err := cw.calculateFileHash(cw.envPath)
					if err == nil {
						if newHash == cw.lastEnvHash {
							log.Debug().Msg("Detected .env file touch but content unchanged, skipping reload")
							continue
						}
						cw.lastEnvHash = newHash
						if stat, statErr := watcherOsStat(cw.envPath); statErr == nil {
							cw.lastModTime = stat.ModTime()
						}
					}

					log.Info().Str("event", event.Op.String()).Msg("Detected .env file change")
					cw.reloadConfig()
				}
			}

			if configWatcherEventMatchesPath(event, cw.apiTokensPath) {
				// Debounce - wait longer for atomic file operations to complete
				// (write to .tmp, rename to final file)
				if !cw.waitForDuration(debounceAPITokensWrite) {
					return
				}

				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
					cw.rewatchFileIfPresent(cw.apiTokensPath)
					if stat, statErr := watcherOsStat(cw.apiTokensPath); statErr == nil {
						cw.apiTokensLastModTime = stat.ModTime()
					}
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

func configWatcherEventMatchesPath(event fsnotify.Event, path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	return event.Name == path || filepath.Base(event.Name) == filepath.Base(path)
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
	newPass, err := normalizeEnvAuthPassword(envMap["PULSE_AUTH_PASS"])
	if err != nil {
		log.Error().Err(err).Msg("Failed to normalize PULSE_AUTH_PASS from watched env file; keeping existing runtime auth password")
	} else if newPass != oldAuthPass {
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

	mockChanged := cw.applyMockEnv(newMockEnv)
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

	if cw.persistence == nil {
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
		tokens, err = cw.persistence.LoadAPITokens()
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
		if err := cw.persistence.SaveAPITokens(tokens); err != nil {
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

func (cw *ConfigWatcher) applyMockEnv(next map[string]string) bool {
	if cw.mockEnvOwned == nil {
		cw.mockEnvOwned = make(map[string]struct{})
	}
	if cw.mockEnvFallbacks == nil {
		cw.mockEnvFallbacks = make(map[string]mockEnvFallback)
	}

	current := currentMockEnv()
	changed := false
	seen := make(map[string]struct{}, len(next))
	for key, value := range next {
		seen[key] = struct{}{}
		if _, owned := cw.mockEnvOwned[key]; !owned {
			fallbackValue, fallbackPresent := current[key]
			cw.mockEnvFallbacks[key] = mockEnvFallback{
				value:   fallbackValue,
				present: fallbackPresent,
			}
			cw.mockEnvOwned[key] = struct{}{}
		}
		if currentValue, ok := current[key]; !ok || currentValue != value {
			_ = os.Setenv(key, value)
			changed = true
		}
	}
	for key := range cw.mockEnvOwned {
		if _, ok := seen[key]; ok {
			continue
		}
		fallback, hasFallback := cw.mockEnvFallbacks[key]
		currentValue, currentPresent := current[key]
		if hasFallback && fallback.present {
			if !currentPresent || currentValue != fallback.value {
				_ = os.Setenv(key, fallback.value)
				changed = true
			}
		} else if currentPresent {
			_ = os.Unsetenv(key)
			changed = true
		}
		delete(cw.mockEnvOwned, key)
		delete(cw.mockEnvFallbacks, key)
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
