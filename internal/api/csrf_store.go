package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// CSRFToken represents a hashed token with expiration metadata.
type CSRFToken struct {
	Hash    string
	Expires time.Time
}

// CSRFTokenStore handles persistent CSRF token storage
type CSRFTokenStore struct {
	tokens     map[string][]*CSRFToken
	mu         sync.RWMutex
	saveMu     sync.Mutex // Serializes disk writes to prevent save corruption
	dataPath   string
	saveTicker *time.Ticker
	stopChan   chan bool
	workerDone chan struct{}
	stopOnce   sync.Once
}

const maxCSRFTokensPerSession = 8

func csrfSessionKey(sessionID string) string {
	return sessionHash(sessionID)
}

func csrfTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CSRFTokenData represents CSRF token data
type CSRFTokenData struct {
	TokenHash  string    `json:"token_hash"`
	SessionKey string    `json:"session_key"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type legacyCSRFTokenData struct {
	Token     string    `json:"token"`
	SessionID string    `json:"session_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (c *CSRFTokenStore) migrateLegacyTokens(data []byte, now time.Time) (bool, error) {
	var legacy map[string]*legacyCSRFTokenData
	if err := json.Unmarshal(data, &legacy); err != nil {
		return false, err
	}

	loaded := 0
	for rawSessionID, record := range legacy {
		if record == nil || now.After(record.ExpiresAt) || record.Token == "" {
			continue
		}

		sessionID := rawSessionID
		if sessionID == "" {
			sessionID = record.SessionID
		}
		if sessionID == "" {
			continue
		}

		c.addTokenUnsafe(csrfSessionKey(sessionID), &CSRFToken{
			Hash:    csrfTokenHash(record.Token),
			Expires: record.ExpiresAt,
		}, now)
		loaded++
	}

	log.Info().
		Int("loaded", loaded).
		Int("total", len(legacy)).
		Msg("CSRF tokens loaded from disk (legacy raw-token format)")
	c.saveUnsafe()
	return true, nil
}

var (
	csrfStore         *CSRFTokenStore
	csrfStoreDataPath string
	csrfStoreMu       sync.Mutex
)

// InitCSRFStore initializes the persistent CSRF token store
func InitCSRFStore(dataPath string) {
	_ = ensureCSRFStore(dataPath)
}

func ensureCSRFStore(dataPath string) *CSRFTokenStore {
	newDataPath := strings.TrimSpace(dataPath)
	if newDataPath == "" {
		return nil
	}

	csrfStoreMu.Lock()
	defer csrfStoreMu.Unlock()

	if csrfStore != nil && csrfStoreDataPath == newDataPath {
		return csrfStore
	}

	oldStore := csrfStore
	csrfStore = &CSRFTokenStore{
		tokens:     make(map[string][]*CSRFToken),
		dataPath:   newDataPath,
		stopChan:   make(chan bool),
		workerDone: make(chan struct{}),
	}
	csrfStoreDataPath = newDataPath

	// Load existing tokens from disk
	csrfStore.load()

	// Start periodic save and cleanup
	csrfStore.saveTicker = time.NewTicker(5 * time.Minute)
	go csrfStore.backgroundWorker()

	if oldStore != nil {
		oldStore.Shutdown()
	}
	return csrfStore
}

// GetCSRFStore returns the global CSRF token store
func GetCSRFStore() *CSRFTokenStore {
	csrfStoreMu.Lock()
	store := csrfStore
	csrfStoreMu.Unlock()
	if store == nil {
		panic("csrf store not initialized; call InitCSRFStore with the configured data path first")
	}
	return store
}

func (c *CSRFTokenStore) Shutdown() {
	if c == nil {
		return
	}
	c.stopOnce.Do(func() {
		if c.saveTicker != nil {
			c.saveTicker.Stop()
		}
		if c.stopChan != nil {
			close(c.stopChan)
		}
	})
	if c.workerDone != nil {
		<-c.workerDone
	}
}

func resetCSRFStoreForTests() {
	csrfStoreMu.Lock()
	oldStore := csrfStore
	csrfStore = nil
	csrfStoreDataPath = ""
	csrfStoreMu.Unlock()
	if oldStore != nil {
		oldStore.Shutdown()
	}
}

// backgroundWorker handles periodic saves and cleanup
func (c *CSRFTokenStore) backgroundWorker() {
	defer close(c.workerDone)
	for {
		select {
		case <-c.saveTicker.C:
			c.cleanup()
			c.save()
		case <-c.stopChan:
			c.save()
			return
		}
	}
}

// GenerateCSRFToken creates a new CSRF token for a session
func (c *CSRFTokenStore) GenerateCSRFToken(sessionID string) string {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Error().Err(err).Msg("Failed to generate CSRF token")
		return ""
	}

	token := base64.URLEncoding.EncodeToString(tokenBytes)

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	key := csrfSessionKey(sessionID)
	c.addTokenUnsafe(key, &CSRFToken{
		Hash:    csrfTokenHash(token),
		Expires: now.Add(4 * time.Hour),
	}, now)

	// Save immediately for important operations
	c.saveUnsafe()

	return token
}

// ValidateCSRFToken checks if a CSRF token is valid for a session
func (c *CSRFTokenStore) ValidateCSRFToken(sessionID, token string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	candidates, exists := c.tokens[csrfSessionKey(sessionID)]
	if !exists {
		return false
	}

	now := time.Now()
	tokenHash := csrfTokenHash(token)
	valid := false
	for _, csrfToken := range candidates {
		if csrfToken == nil || now.After(csrfToken.Expires) {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(csrfToken.Hash), []byte(tokenHash)) == 1 {
			valid = true
		}
	}

	return valid
}

// DeleteCSRFToken removes a CSRF token
func (c *CSRFTokenStore) DeleteCSRFToken(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.tokens, csrfSessionKey(sessionID))
	c.saveUnsafe()
}

func (c *CSRFTokenStore) addTokenUnsafe(sessionKey string, token *CSRFToken, now time.Time) {
	if token == nil || sessionKey == "" {
		return
	}

	tokens := c.pruneSessionTokensUnsafe(sessionKey, now)
	tokens = append(tokens, token)
	if len(tokens) > maxCSRFTokensPerSession {
		tokens = tokens[len(tokens)-maxCSRFTokensPerSession:]
	}
	c.tokens[sessionKey] = tokens
}

func (c *CSRFTokenStore) pruneSessionTokensUnsafe(sessionKey string, now time.Time) []*CSRFToken {
	tokens := c.tokens[sessionKey]
	if len(tokens) == 0 {
		delete(c.tokens, sessionKey)
		return nil
	}

	kept := tokens[:0]
	for _, token := range tokens {
		if token == nil || now.After(token.Expires) {
			continue
		}
		kept = append(kept, token)
	}
	if len(kept) == 0 {
		delete(c.tokens, sessionKey)
		return nil
	}
	if len(kept) > maxCSRFTokensPerSession {
		kept = kept[len(kept)-maxCSRFTokensPerSession:]
	}
	c.tokens[sessionKey] = kept
	return kept
}

func (c *CSRFTokenStore) tokenRecordCountUnsafe() int {
	count := 0
	for _, tokens := range c.tokens {
		count += len(tokens)
	}
	return count
}

// cleanup removes expired CSRF tokens
func (c *CSRFTokenStore) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for sessionKey, tokens := range c.tokens {
		before := len(tokens)
		kept := c.pruneSessionTokensUnsafe(sessionKey, now)
		removed := before - len(kept)
		if removed > 0 {
			log.Debug().
				Str("sessionKey", safePrefixForLog(sessionKey, 8)+"...").
				Int("removed", removed).
				Msg("Cleaned up expired CSRF tokens")
		}
	}
}

// save persists CSRF tokens to disk
func (c *CSRFTokenStore) save() {
	// Serialize all disk writes to prevent corruption
	c.saveMu.Lock()
	defer c.saveMu.Unlock()

	c.mu.RLock()
	defer c.mu.RUnlock()
	c.saveUnsafe()
}

// saveUnsafe saves without locking (caller must hold lock)
func (c *CSRFTokenStore) saveUnsafe() {
	csrfFile := filepath.Join(c.dataPath, "csrf_tokens.json")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(c.dataPath, 0700); err != nil {
		log.Error().Err(err).Msg("Failed to create CSRF tokens directory")
		return
	}

	// Convert to serializable format
	persisted := make([]*CSRFTokenData, 0, c.tokenRecordCountUnsafe())
	for sessionKey, tokens := range c.tokens {
		for _, token := range tokens {
			if token == nil {
				continue
			}
			persisted = append(persisted, &CSRFTokenData{
				TokenHash:  token.Hash,
				SessionKey: sessionKey,
				ExpiresAt:  token.Expires,
			})
		}
	}

	// Marshal tokens
	jsonData, err := json.Marshal(persisted)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal CSRF tokens")
		return
	}

	// Write to temporary file first
	tmpFile := csrfFile + ".tmp"
	if err := os.WriteFile(tmpFile, jsonData, 0600); err != nil {
		log.Error().Err(err).Msg("Failed to write CSRF tokens file")
		return
	}

	// Atomic rename
	if err := os.Rename(tmpFile, csrfFile); err != nil {
		log.Error().Err(err).Msg("Failed to rename CSRF tokens file")
		return
	}

	log.Debug().
		Int("sessions", len(c.tokens)).
		Int("tokens", len(persisted)).
		Msg("CSRF tokens saved to disk")
}

// load reads CSRF tokens from disk
func (c *CSRFTokenStore) load() {
	csrfFile := filepath.Join(c.dataPath, "csrf_tokens.json")

	data, err := os.ReadFile(csrfFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Error().Err(err).Msg("Failed to read CSRF tokens file")
		}
		return
	}

	c.tokens = make(map[string][]*CSRFToken)

	var current []*CSRFTokenData
	if err := json.Unmarshal(data, &current); err == nil {
		now := time.Now()
		loaded := 0
		for _, record := range current {
			if record == nil || record.SessionKey == "" || record.TokenHash == "" || now.After(record.ExpiresAt) {
				continue
			}
			c.addTokenUnsafe(record.SessionKey, &CSRFToken{
				Hash:    record.TokenHash,
				Expires: record.ExpiresAt,
			}, now)
			loaded++
		}
		log.Info().
			Int("loaded", loaded).
			Int("sessions", len(c.tokens)).
			Int("total", len(current)).
			Msg("CSRF tokens loaded from disk (hashed format)")
		return
	}

	if migrated, err := c.migrateLegacyTokens(data, time.Now()); err == nil && migrated {
		return
	}
	log.Error().Msg("Failed to decode CSRF tokens file; unsupported format")
}
