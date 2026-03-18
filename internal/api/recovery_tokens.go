package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// RecoveryToken represents a recovery token for secure authentication bypass
type RecoveryToken struct {
	TokenHash string    `json:"token_hash"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	UsedAt    time.Time `json:"used_at,omitempty"`
	IP        string    `json:"ip,omitempty"`
}

type legacyRecoveryToken struct {
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
	stopOnce    sync.Once
}

var (
	recoveryStore         *RecoveryTokenStore
	recoveryStoreDataPath string
	recoveryStoreMu       sync.Mutex
)

func recoveryTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// InitRecoveryTokenStore initializes the recovery token store
func InitRecoveryTokenStore(dataPath string) {
	newDataPath := strings.TrimSpace(dataPath)
	if newDataPath == "" {
		return
	}

	recoveryStoreMu.Lock()
	defer recoveryStoreMu.Unlock()

	if recoveryStore != nil && recoveryStoreDataPath == newDataPath {
		return
	}

	oldStore := recoveryStore
	recoveryStore = &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    newDataPath,
		stopCleanup: make(chan struct{}),
	}
	recoveryStoreDataPath = newDataPath
	recoveryStore.load()

	// Start cleanup routine
	go recoveryStore.cleanupRoutine()

	if oldStore != nil {
		oldStore.Shutdown()
	}
}

// GetRecoveryTokenStore returns the global recovery token store
func GetRecoveryTokenStore() *RecoveryTokenStore {
	recoveryStoreMu.Lock()
	store := recoveryStore
	recoveryStoreMu.Unlock()
	if store == nil {
		panic("recovery token store not initialized; call InitRecoveryTokenStore with the configured data path first")
	}
	return store
}

// GenerateRecoveryToken creates a new recovery token
func (r *RecoveryTokenStore) GenerateRecoveryToken(duration time.Duration) (string, error) {
	// Generate secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}

	tokenStr := hex.EncodeToString(tokenBytes)
	tokenHash := recoveryTokenHash(tokenStr)

	r.mu.Lock()
	defer r.mu.Unlock()

	token := &RecoveryToken{
		TokenHash: tokenHash,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(duration),
		Used:      false,
	}

	r.tokens[tokenHash] = token
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
	r.mu.RLock()
	defer r.mu.RUnlock()

	token, exists := r.tokens[recoveryTokenHash(providedToken)]
	if !exists {
		return false
	}
	return !time.Now().After(token.ExpiresAt) && !token.Used
}

// ValidateRecoveryTokenConstantTime validates token with constant-time comparison
func (r *RecoveryTokenStore) ValidateRecoveryTokenConstantTime(providedToken string, ip string) bool {
	tokenHash := recoveryTokenHash(providedToken)

	r.mu.RLock()
	defer r.mu.RUnlock()

	token, exists := r.tokens[tokenHash]
	if !exists || time.Now().After(token.ExpiresAt) || token.Used {
		return false
	}

	r.mu.RUnlock()
	r.mu.Lock()

	token, exists = r.tokens[tokenHash]
	if !exists || time.Now().After(token.ExpiresAt) || token.Used {
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
		Str("token", safePrefixForLog(tokenHash, 8)+"...").
		Str("ip", ip).
		Msg("Recovery token successfully validated")

	return true
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

func (r *RecoveryTokenStore) Shutdown() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() {
		close(r.stopCleanup)
	})
}

func resetRecoveryStoreForTests() {
	recoveryStoreMu.Lock()
	oldStore := recoveryStore
	recoveryStore = nil
	recoveryStoreDataPath = ""
	recoveryStoreMu.Unlock()
	if oldStore != nil {
		oldStore.Shutdown()
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
	persisted := make([]*RecoveryToken, 0, len(r.tokens))
	for _, token := range r.tokens {
		copy := *token
		persisted = append(persisted, &copy)
	}
	data, err := json.Marshal(persisted)
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

func (r *RecoveryTokenStore) loadCanonicalTokens(tokens []*RecoveryToken, now time.Time) int {
	loaded := 0
	for _, token := range tokens {
		if token == nil || token.TokenHash == "" {
			continue
		}
		if now.Before(token.ExpiresAt) || (token.Used && now.Sub(token.UsedAt) < 24*time.Hour) {
			copy := *token
			r.tokens[token.TokenHash] = &copy
			loaded++
		}
	}
	return loaded
}

func (r *RecoveryTokenStore) migrateLegacyTokens(data []byte, now time.Time) (bool, error) {
	var legacy map[string]*legacyRecoveryToken
	if err := json.Unmarshal(data, &legacy); err != nil {
		return false, err
	}

	loaded := 0
	for rawToken, token := range legacy {
		if token == nil {
			continue
		}
		if rawToken == "" {
			rawToken = token.Token
		}
		if rawToken == "" {
			continue
		}
		if now.Before(token.ExpiresAt) || (token.Used && now.Sub(token.UsedAt) < 24*time.Hour) {
			tokenHash := recoveryTokenHash(rawToken)
			r.tokens[tokenHash] = &RecoveryToken{
				TokenHash: tokenHash,
				CreatedAt: token.CreatedAt,
				ExpiresAt: token.ExpiresAt,
				Used:      token.Used,
				UsedAt:    token.UsedAt,
				IP:        token.IP,
			}
			loaded++
		}
	}

	log.Info().Int("loaded", loaded).Int("total", len(legacy)).Msg("Recovery tokens loaded from disk (legacy raw-token format)")
	r.saveUnsafe()
	return true, nil
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

	now := time.Now()
	var tokens []*RecoveryToken
	if err := json.Unmarshal(data, &tokens); err == nil {
		loaded := r.loadCanonicalTokens(tokens, now)
		log.Info().Int("loaded", loaded).Int("total", len(tokens)).Msg("Recovery tokens loaded from disk (hashed format)")
		return
	}

	if migrated, err := r.migrateLegacyTokens(data, now); err == nil && migrated {
		return
	}
	log.Error().Msg("Failed to decode recovery tokens file; unsupported format")
}
