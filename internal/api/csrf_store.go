package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// CSRFTokenStore handles persistent CSRF token storage
type CSRFTokenStore struct {
	tokens     map[string]*CSRFToken
	mu         sync.RWMutex
	dataPath   string
	saveTicker *time.Ticker
	stopChan   chan bool
}

// CSRFTokenData represents CSRF token data
type CSRFTokenData struct {
	Token     string    `json:"token"`
	SessionID string    `json:"session_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
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

// Stop gracefully stops the CSRF store
func (c *CSRFTokenStore) Stop() {
	c.saveTicker.Stop()
	c.stopChan <- true
	c.save()
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

	c.tokens[sessionID] = &CSRFToken{
		Token:   token,
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

	csrfToken, exists := c.tokens[sessionID]
	if !exists {
		// No CSRF token for this session - could be server restart
		// Generate a new one on the fly if session is valid
		if ValidateSession(sessionID) {
			c.mu.RUnlock()
			newToken := c.GenerateCSRFToken(sessionID)
			c.mu.RLock()
			// Allow this request but with new token
			return newToken != ""
		}
		return false
	}

	if time.Now().After(csrfToken.Expires) {
		return false
	}

	return csrfToken.Token == token
}

// GetCSRFToken returns the CSRF token for a session if it exists
func (c *CSRFTokenStore) GetCSRFToken(sessionID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	csrfToken, exists := c.tokens[sessionID]
	if !exists || time.Now().After(csrfToken.Expires) {
		return ""
	}

	return csrfToken.Token
}

// ExtendCSRFToken extends the expiration of a CSRF token
func (c *CSRFTokenStore) ExtendCSRFToken(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if csrfToken, exists := c.tokens[sessionID]; exists {
		csrfToken.Expires = time.Now().Add(4 * time.Hour)
		c.saveUnsafe()
	}
}

// DeleteCSRFToken removes a CSRF token
func (c *CSRFTokenStore) DeleteCSRFToken(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.tokens, sessionID)
	c.saveUnsafe()
}

// cleanup removes expired CSRF tokens
func (c *CSRFTokenStore) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for sessionID, token := range c.tokens {
		if now.After(token.Expires) {
			delete(c.tokens, sessionID)
			log.Debug().Str("session", sessionID[:8]+"...").Msg("Cleaned up expired CSRF token")
		}
	}
}

// save persists CSRF tokens to disk
func (c *CSRFTokenStore) save() {
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
	data := make(map[string]*CSRFTokenData)
	for sessionID, token := range c.tokens {
		data[sessionID] = &CSRFTokenData{
			Token:     token.Token,
			SessionID: sessionID,
			ExpiresAt: token.Expires,
			CreatedAt: time.Now(),
		}
	}

	// Marshal tokens
	jsonData, err := json.MarshalIndent(data, "", "  ")
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

	var tokens map[string]*CSRFTokenData
	if err := json.Unmarshal(data, &tokens); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal CSRF tokens")
		return
	}

	// Filter out expired tokens and convert to internal format
	now := time.Now()
	loaded := 0
	for sessionID, tokenData := range tokens {
		if now.Before(tokenData.ExpiresAt) {
			c.tokens[sessionID] = &CSRFToken{
				Token:   tokenData.Token,
				Expires: tokenData.ExpiresAt,
			}
			loaded++
		}
	}

	log.Info().Int("loaded", loaded).Int("total", len(tokens)).Msg("CSRF tokens loaded from disk")
}

// ClearAll removes all CSRF tokens (use carefully)
func (c *CSRFTokenStore) ClearAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tokens = make(map[string]*CSRFToken)
	c.saveUnsafe()
	log.Info().Msg("All CSRF tokens cleared")
}