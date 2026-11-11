package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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
	cleanupDone chan struct{}
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

func normalizeRecoveryBindingIP(ip string) string {
	trimmed := strings.TrimSpace(ip)
	if trimmed == "" {
		return ""
	}
	parsed := net.ParseIP(trimmed)
	if parsed == nil {
		return trimmed
	}
	if parsed.IsLoopback() {
		return "loopback"
	}
	return parsed.String()
}

func recoveryTokenBoundToIP(token *RecoveryToken, ip string) bool {
	if token == nil {
		return false
	}
	expectedIP := normalizeRecoveryBindingIP(token.IP)
	actualIP := normalizeRecoveryBindingIP(ip)
	if expectedIP == "" || actualIP == "" {
		return false
	}
	return len(expectedIP) == len(actualIP) &&
		subtle.ConstantTimeCompare([]byte(expectedIP), []byte(actualIP)) == 1
}

// InitRecoveryTokenStore initializes the recovery token store
func InitRecoveryTokenStore(dataPath string) {
	_ = ensureRecoveryTokenStore(dataPath)
}

func ensureRecoveryTokenStore(dataPath string) *RecoveryTokenStore {
	newDataPath := strings.TrimSpace(dataPath)
	if newDataPath == "" {
		return nil
	}

	recoveryStoreMu.Lock()
	defer recoveryStoreMu.Unlock()

	if recoveryStore != nil && recoveryStoreDataPath == newDataPath {
		return recoveryStore
	}

	oldStore := recoveryStore
	recoveryStore = &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    newDataPath,
		stopCleanup: make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}
	recoveryStoreDataPath = newDataPath
	recoveryStore.load()

	// Start cleanup routine
	go recoveryStore.cleanupRoutine()

	if oldStore != nil {
		oldStore.Shutdown()
	}
	return recoveryStore
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

// GenerateRecoveryToken creates a new recovery token bound to the generating client IP.
func (r *RecoveryTokenStore) GenerateRecoveryToken(duration time.Duration, ip string) (string, error) {
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
		IP:        normalizeRecoveryBindingIP(ip),
	}
	if token.IP == "" {
		return "", os.ErrInvalid
	}

	r.tokens[tokenHash] = token
	if err := r.saveUnsafe(); err != nil {
		delete(r.tokens, tokenHash)
		return "", fmt.Errorf("persist recovery token: %w", err)
	}

	log.Info().
		Str("token", safePrefixForLog(tokenStr, 8)+"...").
		Str("ip", token.IP).
		Time("expires", token.ExpiresAt).
		Msg("Recovery token generated")

	return tokenStr, nil
}

// IsRecoveryTokenValidConstantTime checks token validity without consuming it.
// This is intended for preflight decisions (e.g., CSRF skip routing).
func (r *RecoveryTokenStore) IsRecoveryTokenValidConstantTime(providedToken string, ip string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	token, exists := r.tokens[recoveryTokenHash(providedToken)]
	if !exists {
		return false
	}
	return !time.Now().After(token.ExpiresAt) && !token.Used && recoveryTokenBoundToIP(token, ip)
}

// ValidateRecoveryTokenConstantTime validates token with constant-time comparison
func (r *RecoveryTokenStore) ValidateRecoveryTokenConstantTime(providedToken string, ip string) bool {
	tokenHash := recoveryTokenHash(providedToken)

	// Keep admission, persistence and publication in one critical section.
	// No caller may authorize from an in-flight or failed consumption.
	r.mu.Lock()
	defer r.mu.Unlock()

	token, exists := r.tokens[tokenHash]
	if !exists || time.Now().After(token.ExpiresAt) || token.Used || !recoveryTokenBoundToIP(token, ip) {
		return false
	}

	previous := *token
	token.Used = true
	token.UsedAt = time.Now()
	if err := r.saveUnsafe(); err != nil {
		*token = previous
		log.Error().Err(err).Msg("Failed to persist recovery token consumption")
		return false
	}

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
	if r.cleanupDone != nil {
		defer close(r.cleanupDone)
	}

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
		if r.stopCleanup != nil {
			close(r.stopCleanup)
		}
	})
	if r.cleanupDone != nil {
		<-r.cleanupDone
	}
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
		if err := r.saveUnsafe(); err != nil {
			log.Error().Err(err).Msg("Failed to persist recovery token cleanup")
			return
		}
		log.Info().Int("count", cleaned).Msg("Cleaned up recovery tokens")
	}
}

// saveUnsafe publishes a complete private snapshot without locking (caller
// must hold the lock or own the not-yet-published store during initialization).
// Every returned error precedes rename; callers may then roll back live changes
// without undoing an acknowledged write. This does not promise directory-fsync
// durability across sudden power loss.
func (r *RecoveryTokenStore) saveUnsafe() (retErr error) {
	tokensFile := filepath.Join(r.dataPath, "recovery_tokens.json")
	if err := os.MkdirAll(r.dataPath, 0700); err != nil {
		return fmt.Errorf("create recovery tokens directory: %w", err)
	}

	persisted := make([]*RecoveryToken, 0, len(r.tokens))
	for _, token := range r.tokens {
		copy := *token
		persisted = append(persisted, &copy)
	}
	data, err := json.Marshal(persisted)
	if err != nil {
		return fmt.Errorf("marshal recovery tokens: %w", err)
	}

	// Own the exclusive descriptor, not a reusable temporary pathname that
	// could follow an old symlink or retain a previous file's wider mode.
	file, err := os.CreateTemp(r.dataPath, ".recovery_tokens.*.tmp")
	if err != nil {
		return fmt.Errorf("create recovery tokens file: %w", err)
	}
	closed, published := false, false
	defer func() {
		if !closed {
			retErr = errors.Join(retErr, file.Close())
		}
		if !published {
			if err := os.Remove(file.Name()); err != nil && !os.IsNotExist(err) {
				retErr = errors.Join(retErr, fmt.Errorf("cleanup recovery tokens file: %w", err))
			}
		}
	}()

	if n, err := file.Write(data); err != nil {
		return fmt.Errorf("write recovery tokens: %w", err)
	} else if n != len(data) {
		return fmt.Errorf("write recovery tokens: %w", io.ErrShortWrite)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync recovery tokens: %w", err)
	}
	closed = true
	if err := file.Close(); err != nil {
		return fmt.Errorf("close recovery tokens: %w", err)
	}
	if err := os.Rename(file.Name(), tokensFile); err != nil {
		return fmt.Errorf("publish recovery tokens: %w", err)
	}
	published = true
	return nil
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

	if err := r.saveUnsafe(); err != nil {
		// Retain the known legacy inventory. Any later consumption still has
		// to publish its complete hashed snapshot before granting authority.
		return true, err
	}
	log.Info().Int("loaded", loaded).Int("total", len(legacy)).Msg("Recovery tokens loaded from disk (legacy raw-token format)")
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

	if migrated, err := r.migrateLegacyTokens(data, now); migrated {
		if err != nil {
			log.Error().Err(err).Msg("Failed to persist recovery token migration")
		}
		return
	}
	log.Error().Msg("Failed to decode recovery tokens file; unsupported format")
}
