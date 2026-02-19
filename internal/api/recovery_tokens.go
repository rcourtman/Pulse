package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// RecoveryToken represents a recovery token for secure authentication bypass
type RecoveryToken struct {
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	UsedAt    time.Time `json:"used_at,omitempty"`
	IP        string    `json:"ip,omitempty"`
}

// RecoveryTokenStore manages recovery tokens
type RecoveryTokenStore struct {
	tokens      map[string]*RecoveryToken
	mu          sync.RWMutex
	dataPath    string
	stopCleanup chan struct{}
}

var (
	recoveryStore     *RecoveryTokenStore
	recoveryStoreOnce sync.Once
)

// InitRecoveryTokenStore initializes the recovery token store
func InitRecoveryTokenStore(dataPath string) {
	recoveryStoreOnce.Do(func() {
		recoveryStore = &RecoveryTokenStore{
			tokens:      make(map[string]*RecoveryToken),
			dataPath:    dataPath,
			stopCleanup: make(chan struct{}),
		}
		recoveryStore.load()

		// Start cleanup routine
		go recoveryStore.cleanupRoutine()
	})
}

// GetRecoveryTokenStore returns the global recovery token store
func GetRecoveryTokenStore() *RecoveryTokenStore {
	// Always route through sync.Once to avoid unsynchronized reads on recoveryStore.
	InitRecoveryTokenStore("/etc/pulse")
	return recoveryStore
}

// GenerateRecoveryToken creates a new recovery token
func (r *RecoveryTokenStore) GenerateRecoveryToken(duration time.Duration) (string, error) {
	// Generate secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}

	tokenStr := hex.EncodeToString(tokenBytes)

	r.mu.Lock()
	defer r.mu.Unlock()

	token := &RecoveryToken{
		Token:     tokenStr,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(duration),
		Used:      false,
	}

	r.tokens[tokenStr] = token
	r.saveUnsafe()

	log.Info().
		Str("token", safePrefixForLog(tokenStr, 8)+"...").
		Time("expires", token.ExpiresAt).
		Msg("Recovery token generated")

	return tokenStr, nil
}

// IsRecoveryTokenValidConstantTime checks token validity without consuming it.
// This is intended for preflight decisions (e.g., CSRF skip routing).
func (r *RecoveryTokenStore) IsRecoveryTokenValidConstantTime(providedToken string) bool {
	providedBytes := []byte(providedToken)

	r.mu.RLock()
	defer r.mu.RUnlock()

	for tokenStr, token := range r.tokens {
		if subtle.ConstantTimeCompare(providedBytes, []byte(tokenStr)) == 1 {
			return !time.Now().After(token.ExpiresAt) && !token.Used
		}
	}

	return false
}

// ValidateRecoveryTokenConstantTime validates token with constant-time comparison
func (r *RecoveryTokenStore) ValidateRecoveryTokenConstantTime(providedToken string, ip string) bool {
	// Use constant-time comparison to prevent timing attacks
	providedBytes := []byte(providedToken)

	r.mu.RLock()
	defer r.mu.RUnlock()

	for tokenStr, token := range r.tokens {
		tokenBytes := []byte(tokenStr)

		// Constant-time comparison
		if subtle.ConstantTimeCompare(providedBytes, tokenBytes) == 1 {
			// Token matches
			if time.Now().After(token.ExpiresAt) || token.Used {
				return false
			}

			// Need to upgrade to write lock to mark as used
			r.mu.RUnlock()
			r.mu.Lock()

			// CRITICAL: Re-check token.Used after acquiring write lock
			// Prevents replay attack where two concurrent requests both pass initial check
			if token.Used {
				r.mu.Unlock()
				r.mu.RLock()
				return false
			}

			token.Used = true
			token.UsedAt = time.Now()
			token.IP = ip
			r.saveUnsafe()
			r.mu.Unlock()
			r.mu.RLock()

			log.Info().
				Str("token", safePrefixForLog(tokenStr, 8)+"...").
				Str("ip", ip).
				Msg("Recovery token successfully validated")

			return true
		}
	}

	return false
}

// cleanupRoutine periodically removes expired tokens
func (r *RecoveryTokenStore) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case <-r.stopCleanup:
			log.Debug().Msg("Recovery token cleanup routine stopped")
			return
		}
	}
}

// cleanup removes expired and used tokens
func (r *RecoveryTokenStore) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for tokenStr, token := range r.tokens {
		// Remove if expired or used more than 24 hours ago
		if now.After(token.ExpiresAt) || (token.Used && now.Sub(token.UsedAt) > 24*time.Hour) {
			delete(r.tokens, tokenStr)
			cleaned++
		}
	}

	if cleaned > 0 {
		r.saveUnsafe()
		log.Info().Int("count", cleaned).Msg("Cleaned up recovery tokens")
	}
}

// saveUnsafe saves without locking (caller must hold lock)
func (r *RecoveryTokenStore) saveUnsafe() {
	tokensFile := filepath.Join(r.dataPath, "recovery_tokens.json")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(r.dataPath, 0700); err != nil {
		log.Error().Err(err).Msg("Failed to create recovery tokens directory")
		return
	}

	// Marshal tokens
	data, err := json.Marshal(r.tokens)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal recovery tokens")
		return
	}

	// Write to temporary file first
	tmpFile := tokensFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		log.Error().Err(err).Msg("Failed to write recovery tokens file")
		return
	}

	// Atomic rename
	if err := os.Rename(tmpFile, tokensFile); err != nil {
		log.Error().Err(err).Msg("Failed to rename recovery tokens file")
		return
	}
}

// load reads tokens from disk
func (r *RecoveryTokenStore) load() {
	tokensFile := filepath.Join(r.dataPath, "recovery_tokens.json")

	data, err := os.ReadFile(tokensFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Error().Err(err).Msg("Failed to read recovery tokens file")
		}
		return
	}

	var tokens map[string]*RecoveryToken
	if err := json.Unmarshal(data, &tokens); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal recovery tokens")
		return
	}

	// Filter out expired tokens
	now := time.Now()
	loaded := 0
	for tokenStr, token := range tokens {
		// Keep unexpired tokens and recently used tokens
		if now.Before(token.ExpiresAt) || (token.Used && now.Sub(token.UsedAt) < 24*time.Hour) {
			r.tokens[tokenStr] = token
			loaded++
		}
	}

	log.Info().Int("loaded", loaded).Int("total", len(tokens)).Msg("Recovery tokens loaded from disk")
}
