package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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
	debounceMockWrite      = 100 * time.Millisecond
)

// ConfigWatcher monitors the .env file for changes and updates runtime config
type ConfigWatcher struct {
	config               *Config
	envPath              string
	mockEnvPath          string
	apiTokensPath        string
	watcher              *fsnotify.Watcher
	stopChan             chan struct{}
	stopOnce             sync.Once // Ensures Stop() can only close channel once
	lastModTime          time.Time
	lastEnvHash          string
	mockLastModTime      time.Time
	apiTokensLastModTime time.Time
	mu                   sync.RWMutex
	onMockReload         func() // Callback to trigger backend restart
	onAPITokenReload     func() // Callback when API tokens are reloaded from disk
	pollInterval         time.Duration
}

// NewConfigWatcher creates a new config watcher
func NewConfigWatcher(config *Config) (*ConfigWatcher, error) {
	// CRITICAL FIX: Config watcher must ALWAYS watch the persistent production config,
	// NOT the mock data directory. Mock mode should only affect Proxmox data, not auth.
	//
	// Strategy:
	// 1. Check PULSE_AUTH_CONFIG_DIR (dedicated env var for auth config)
	// 2. If PULSE_DATA_DIR looks like production (/etc/pulse or /data), use it
	// 3. Otherwise, always prefer /etc/pulse if it exists (production auth)
	// 4. Fall back to /data for Docker environments
	// 5. Last resort: use PULSE_DATA_DIR (may be mock/dev)

	persistentDataDir := ""
	dataDir := watcherOsGetenv("PULSE_DATA_DIR")

	// Option 1: Explicit auth config directory override
	if authDir := watcherOsGetenv("PULSE_AUTH_CONFIG_DIR"); authDir != "" {
		persistentDataDir = authDir
		log.Info().Str("authConfigDir", authDir).Msg("Using PULSE_AUTH_CONFIG_DIR for auth config")
	} else if dataDir == "/etc/pulse" || dataDir == "/data" {
		// Option 2: PULSE_DATA_DIR is already production, use it
		persistentDataDir = dataDir
	} else if _, err := watcherOsStat("/etc/pulse/.env"); err == nil {
		// Option 3: /etc/pulse exists, use it (production)
		persistentDataDir = "/etc/pulse"
		if dataDir != "" && dataDir != persistentDataDir {
			log.Warn().
				Str("dataDir", dataDir).
				Str("authConfigDir", persistentDataDir).
				Msg("PULSE_DATA_DIR points to non-production directory - using /etc/pulse for auth config instead")
		}
	} else if _, err := watcherOsStat("/data/.env"); err == nil {
		// Option 4: Docker environment
		persistentDataDir = "/data"
	} else if dataDir != "" {
		// Option 5: Use PULSE_DATA_DIR as fallback
		persistentDataDir = dataDir
		if strings.Contains(persistentDataDir, "/mock-data") || strings.Contains(persistentDataDir, "/tmp/") {
			log.Warn().
				Str("authConfigDir", persistentDataDir).
				Msg("WARNING: Auth config watcher is using temporary/mock directory - auth may be unstable")
		}
	} else {
		// Option 6: Last resort default
		persistentDataDir = "/etc/pulse"
	}

	envPath := filepath.Join(persistentDataDir, ".env")

	// Log what we're watching for debugging
	log.Info().
		Str("watchingPath", envPath).
		Str("authConfigDir", persistentDataDir).
		Str("pulseDataDir", dataDir).
		Str("configPathFromConfig", config.ConfigPath).
		Msg("Config watcher initialized - watching production auth config")

	// Determine mock.env path - skip in Docker or if directory doesn't exist
	mockEnvPath := ""
	isDocker := watcherOsGetenv("PULSE_DOCKER") == "true"
	mockDir := "/opt/pulse"
	if !isDocker {
		if stat, err := watcherOsStat(mockDir); err == nil && stat.IsDir() {
			mockEnvPath = filepath.Join(mockDir, "mock.env")
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	apiTokensPath := filepath.Join(filepath.Dir(envPath), "api_tokens.json")

	cw := &ConfigWatcher{
		config:        config,
		envPath:       envPath,
		mockEnvPath:   mockEnvPath,
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
	if mockEnvPath != "" {
		if stat, err := watcherOsStat(mockEnvPath); err == nil {
			cw.mockLastModTime = stat.ModTime()
		}
	}
	if stat, err := watcherOsStat(apiTokensPath); err == nil {
		cw.apiTokensLastModTime = stat.ModTime()
	}

	return cw, nil
}

// SetMockReloadCallback sets the callback function to trigger when mock.env changes
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

	// Also watch the mock.env directory if it's configured (not in Docker)
	if cw.mockEnvPath != "" {
		mockDir := filepath.Dir(cw.mockEnvPath)
		if err := cw.watcher.Add(mockDir); err != nil {
			log.Warn().Err(err).Str("path", mockDir).Msg("Failed to watch mock.env directory")
		}
	}

	if err != nil {
		log.Warn().Msg("Falling back to polling for config changes")
		go cw.pollForChanges()
		return nil
	}

	go cw.watchForChanges()
	logEvent := log.Info().Str("env_path", cw.envPath).Str("api_tokens_path", cw.apiTokensPath)
	if cw.mockEnvPath != "" {
		logEvent = logEvent.Str("mock_env_path", cw.mockEnvPath)
	}
	logEvent.Msg("Started watching config files for changes")
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
				time.Sleep(debounceEnvWrite)

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
				time.Sleep(debounceAPITokensWrite)

				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					log.Info().Str("event", event.Op.String()).Msg("Detected API token file change")
					cw.reloadAPITokens()
				}
			}

			// Check if the event is for mock.env (only if mock.env watching is enabled)
			if cw.mockEnvPath != "" && (filepath.Base(event.Name) == "mock.env" || event.Name == cw.mockEnvPath) {
				// Debounce - wait a bit for write to complete
				time.Sleep(debounceMockWrite)

				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					log.Info().Str("event", event.Op.String()).Msg("Detected mock.env file change")
					cw.reloadMockConfig()
				}
			}

		case err, ok := <-errors:
			if !ok {
				return
			}
			log.Error().Err(err).Msg("Config watcher error")

		case <-cw.stopChan:
			return
		}
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

			// Check mock.env (only if mock.env watching is enabled)
			if cw.mockEnvPath != "" {
				if stat, err := os.Stat(cw.mockEnvPath); err == nil {
					if stat.ModTime().After(cw.mockLastModTime) {
						log.Info().Msg("Detected mock.env file change via polling")
						cw.mockLastModTime = stat.ModTime()
						cw.reloadMockConfig()
					}
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
	cw.mu.Lock()
	defer cw.mu.Unlock()

	// Load the .env file
	envMap, err := godotenv.Read(cw.envPath)
	if err != nil {
		// File might not exist, which is fine (no auth)
		if !os.IsNotExist(err) {
			log.Error().Err(err).Msg("Failed to read .env file")
			return
		}
		envMap = make(map[string]string)
	}

	// Track what changed
	var changes []string

	// Update auth settings
	oldAuthUser := cw.config.AuthUser
	oldAuthPass := cw.config.AuthPass
	oldTokenHashes := cw.config.ActiveAPITokenHashes()
	existingByHash := make(map[string]APITokenRecord, len(cw.config.APITokens))
	for _, record := range cw.config.APITokens {
		existingByHash[record.Hash] = record.Clone()
	}

	// Apply auth user
	Mu.Lock()
	defer Mu.Unlock()

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

	// Legacy env token support: only process if api_tokens.json is empty
	// This prevents .env changes from overwriting UI-managed tokens (fixes #685)
	rawTokens := make([]string, 0, 4)
	if raw, ok := envMap["API_TOKENS"]; ok {
		raw = strings.Trim(raw, "'\"")
		if raw != "" {
			parts := strings.Split(raw, ",")
			for _, part := range parts {
				token := strings.TrimSpace(part)
				if token != "" {
					rawTokens = append(rawTokens, token)
				}
			}
		}
	}
	if raw, ok := envMap["API_TOKEN"]; ok {
		raw = strings.Trim(raw, "'\"")
		if raw != "" {
			rawTokens = append(rawTokens, raw)
		}
	}

	// Only reload tokens from .env if NO tokens exist in api_tokens.json
	// This makes api_tokens.json the authoritative source once it has records
	if len(rawTokens) > 0 && len(cw.config.APITokens) == 0 {
		log.Debug().Msg("No existing API tokens found - loading from .env (legacy)")
		seen := make(map[string]struct{}, len(rawTokens))
		newRecords := make([]APITokenRecord, 0, len(rawTokens))
		for _, tokenValue := range rawTokens {
			tokenValue = strings.TrimSpace(tokenValue)
			if tokenValue == "" {
				continue
			}

			hashed := tokenValue
			prefix := tokenPrefix(tokenValue)
			suffix := tokenSuffix(tokenValue)
			if !auth.IsAPITokenHashed(tokenValue) {
				hashed = auth.HashAPIToken(tokenValue)
				prefix = tokenPrefix(tokenValue)
				suffix = tokenSuffix(tokenValue)
			}

			if _, exists := seen[hashed]; exists {
				continue
			}
			seen[hashed] = struct{}{}

			newRecords = append(newRecords, APITokenRecord{
				ID:        uuid.NewString(),
				Name:      "Environment token",
				Hash:      hashed,
				Prefix:    prefix,
				Suffix:    suffix,
				CreatedAt: time.Now().UTC(),
				Scopes:    []string{ScopeWildcard},
			})
		}

		cw.config.APITokens = newRecords
		cw.config.SortAPITokens()

		newHashes := cw.config.ActiveAPITokenHashes()
		if !reflect.DeepEqual(oldTokenHashes, newHashes) {
			changes = append(changes, "API tokens added")

			if globalPersistence != nil {
				if err := globalPersistence.SaveAPITokens(cw.config.APITokens); err != nil {
					log.Error().Err(err).Msg("Failed to persist API tokens from .env reload")
				}
			}
		}
	} else if len(rawTokens) > 0 && len(cw.config.APITokens) > 0 {
		log.Debug().Msg("Ignoring API_TOKEN/API_TOKENS from .env - api_tokens.json is authoritative")
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
}

func (cw *ConfigWatcher) reloadAPITokens() {
	cw.mu.Lock()
	callback := cw.onAPITokenReload
	cw.mu.Unlock()

	cw.mu.Lock()
	defer cw.mu.Unlock()

	if globalPersistence == nil {
		log.Warn().Msg("Config persistence unavailable; cannot reload API tokens")
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
				Msg("Failed to reload API tokens, retrying...")
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		}
	}

	if err != nil {
		log.Error().
			Err(err).
			Int("existingTokens", existingCount).
			Msg("Failed to reload API tokens after retries - preserving existing tokens")
		// CRITICAL: Keep existing tokens rather than clearing them
		return
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

// reloadMockConfig handles mock.env file changes
func (cw *ConfigWatcher) reloadMockConfig() {
	// Skip if mock.env watching is disabled (Docker environment)
	if cw.mockEnvPath == "" {
		return
	}

	cw.mu.Lock()
	callback := cw.onMockReload
	cw.mu.Unlock()

	// Load the mock.env file to update environment variables
	envMap, err := godotenv.Read(cw.mockEnvPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Error().Err(err).Msg("Failed to read mock.env file")
			return
		}
		log.Warn().Msg("mock.env file not found")
		return
	}

	// Load local overrides if they exist
	mockEnvLocalPath := cw.mockEnvPath + ".local"
	if localEnv, err := godotenv.Read(mockEnvLocalPath); err == nil {
		// Merge local overrides into envMap
		for key, value := range localEnv {
			envMap[key] = value
		}
		log.Debug().Str("path", mockEnvLocalPath).Msg("Loaded mock.env.local overrides")
	}

	// Update environment variables for the mock package to read
	for key, value := range envMap {
		if strings.HasPrefix(key, "PULSE_MOCK_") {
			os.Setenv(key, value)
		}
	}

	log.Info().
		Str("path", cw.mockEnvPath).
		Interface("config", envMap).
		Msg("Reloaded mock.env configuration")

	// Trigger callback to restart backend if set
	if callback != nil {
		log.Info().Msg("Triggering backend restart due to mock.env change")
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().
						Interface("panic", r).
						Stack().
						Msg("Recovered from panic in mock.env callback")
				}
			}()
			callback()
		}()
	}
}
