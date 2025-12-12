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
	tokens     map[string]*CSRFToken
	mu         sync.RWMutex
	saveMu     sync.Mutex // Serializes disk writes to prevent save corruption
	dataPath   string
	saveTicker *time.Ticker
	stopChan   chan bool
}

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

var (
	csrfStore     *CSRFTokenStore
	csrfStoreOnce sync.Once
)

// InitCSRFStore initializes the persistent CSRF token store
func InitCSRFStore(dataPath string) {
	csrfStoreOnce.Do(func() {
		csrfStore = &CSRFTokenStore{
			tokens:   make(map[string]*CSRFToken),
			dataPath: dataPath,
			stopChan: make(chan bool),
		}

		// Load existing tokens from disk
		csrfStore.load()

		// Start periodic save and cleanup
		csrfStore.saveTicker = time.NewTicker(5 * time.Minute)
		go csrfStore.backgroundWorker()
	})
}

// GetCSRFStore returns the global CSRF token store
func GetCSRFStore() *CSRFTokenStore {
	if csrfStore == nil {
		InitCSRFStore("/etc/pulse")
	}
	return csrfStore
}

// backgroundWorker handles periodic saves and cleanup
func (c *CSRFTokenStore) backgroundWorker() {
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

	key := csrfSessionKey(sessionID)
	c.tokens[key] = &CSRFToken{
		Hash:    csrfTokenHash(token),
		Expires: time.Now().Add(4 * time.Hour),
	}

	// Save immediately for important operations
	c.saveUnsafe()

	return token
}

// ValidateCSRFToken checks if a CSRF token is valid for a session
func (c *CSRFTokenStore) ValidateCSRFToken(sessionID, token string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	csrfToken, exists := c.tokens[csrfSessionKey(sessionID)]
	if !exists {
		return false
	}

	if time.Now().After(csrfToken.Expires) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(csrfToken.Hash), []byte(csrfTokenHash(token))) == 1
}

// DeleteCSRFToken removes a CSRF token
func (c *CSRFTokenStore) DeleteCSRFToken(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.tokens, csrfSessionKey(sessionID))
	c.saveUnsafe()
}

// cleanup removes expired CSRF tokens
func (c *CSRFTokenStore) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for sessionKey, token := range c.tokens {
		if now.After(token.Expires) {
			delete(c.tokens, sessionKey)
			log.Debug().Str("sessionKey", safePrefixForLog(sessionKey, 8)+"...").Msg("Cleaned up expired CSRF token")
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
	persisted := make([]*CSRFTokenData, 0, len(c.tokens))
	for sessionKey, token := range c.tokens {
		persisted = append(persisted, &CSRFTokenData{
			TokenHash:  token.Hash,
			SessionKey: sessionKey,
			ExpiresAt:  token.Expires,
		})
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

	log.Debug().Int("count", len(c.tokens)).Msg("CSRF tokens saved to disk")
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

	c.tokens = make(map[string]*CSRFToken)

	var current []*CSRFTokenData
	if err := json.Unmarshal(data, &current); err == nil {
		now := time.Now()
		for _, record := range current {
			if record == nil || now.After(record.ExpiresAt) {
				continue
			}
			c.tokens[record.SessionKey] = &CSRFToken{
				Hash:    record.TokenHash,
				Expires: record.ExpiresAt,
			}
		}
		log.Info().
			Int("loaded", len(c.tokens)).
			Int("total", len(current)).
			Msg("CSRF tokens loaded from disk (hashed format)")
		return
	}

	var legacy map[string]*legacyCSRFTokenData
	if err := json.Unmarshal(data, &legacy); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal CSRF tokens")
		return
	}

	now := time.Now()
	loaded := 0
	for sessionID, tokenData := range legacy {
		if now.Before(tokenData.ExpiresAt) {
			key := csrfSessionKey(sessionID)
			c.tokens[key] = &CSRFToken{
				Hash:    csrfTokenHash(tokenData.Token),
				Expires: tokenData.ExpiresAt,
			}
			loaded++
		}
	}

	log.Info().
		Int("loaded", loaded).
		Int("total", len(legacy)).
		Msg("CSRF tokens loaded from disk (legacy format migrated)")
}
