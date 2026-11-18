package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	tokens        map[string][]*CSRFToken
	mu            sync.RWMutex
	saveMu        sync.Mutex // serializes snapshot selection and writes; never acquire while holding mu
	dataPath      string
	snapshotWrite authSnapshotWriteFunc // nil in runtime stores; per-instance real-I/O fixture seam
	saveTicker    *time.Ticker
	stopChan      chan bool
	workerDone    chan struct{}
	stopOnce      sync.Once
	// Set only by startup load, then owned by saveMu. An unread inventory
	// must be retained outside active storage before any fresh snapshot.
	preserveBeforeSave    bool
	preserveFailureLogged bool
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
	// Do not partially load or immediately migrate an incomplete inventory.
	for rawSessionID, record := range legacy {
		if record == nil || strings.TrimSpace(record.Token) == "" {
			return false, errIncompleteAuthInventory
		}
		sessionID := rawSessionID
		if sessionID == "" {
			sessionID = record.SessionID
		}
		if strings.TrimSpace(sessionID) == "" {
			return false, errIncompleteAuthInventory
		}
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
	c.save()
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

	now := time.Now()
	key := csrfSessionKey(sessionID)
	c.addTokenUnsafe(key, &CSRFToken{
		Hash:    csrfTokenHash(token),
		Expires: now.Add(4 * time.Hour),
	}, now)
	c.mu.Unlock()

	// Save immediately for important operations
	c.save()

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
	c.removeSessionKeys([]string{csrfSessionKey(sessionID)})
	c.save()
}

// DeleteCSRFSessionKeys removes records by their hashed session-storage keys.
// This is used for account-wide revocation, where persisted sessions can be
// found by principal without retaining their raw bearer tokens.
func (c *CSRFTokenStore) DeleteCSRFSessionKeys(sessionKeys []string) {
	if len(sessionKeys) == 0 {
		return
	}
	c.removeSessionKeys(sessionKeys)
	// Even when another pending revocation already removed these live keys,
	// this caller must wait for a fresh save attempt before returning.
	c.save()
}

// removeSessionKeys withdraws all live tokens for the selected sessions without
// disk I/O. Coordinated callers must save after releasing the tracking lock.
func (c *CSRFTokenStore) removeSessionKeys(sessionKeys []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, key := range sessionKeys {
		delete(c.tokens, key)
	}
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

// save persists a detached snapshot without holding the validation/withdrawal
// mutex during marshaling or disk I/O. Mutators still wait for their save.
// Select the snapshot only AFTER acquiring saveMu: selecting it earlier could
// queue stale token authority behind a newer revocation. No caller holds mu.
func (c *CSRFTokenStore) save() {
	c.saveMu.Lock()
	defer c.saveMu.Unlock()

	if c.preserveBeforeSave {
		preserved, err := c.preserveUnreadTokens()
		if err != nil {
			if !c.preserveFailureLogged {
				log.Error().Err(err).Msg("Cannot preserve unread CSRF token file; disk saves are disabled until it can be preserved. New CSRF tokens are memory-only and token revocations are not durable. Preserve or move csrf_tokens.json before restarting; do not restore stale tokens")
				c.preserveFailureLogged = true
			}
			return
		}
		c.preserveBeforeSave = false
		if preserved != "" {
			log.Warn().Str("preserved_file", preserved).Msg("Unread CSRF token file preserved outside active storage; it will not be loaded automatically. Refresh CSRF tokens rather than restoring stale tokens")
		} else {
			log.Warn().Msg("Unread CSRF token file is no longer in active storage; saving only current CSRF tokens")
		}
	}

	persisted, sessions := c.persistenceSnapshot()
	c.persistSnapshot(persisted, sessions)
}

// preserveUnreadTokens retains the original inode without reading it. Only the
// canonical file is loaded; private retained files are never active authority.
// Called under saveMu, never the token mutex. Nonregular paths stay untouched,
// and failure holds fresh publication until storage recovers or an operator
// moves the unread entry. No protection against concurrent directory tampering
// or promise of crash/power-loss durability is added.
func (c *CSRFTokenStore) preserveUnreadTokens() (string, error) {
	path := filepath.Join(c.dataPath, "csrf_tokens.json")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect unread CSRF token file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("unread CSRF token path is not a regular file; leaving it untouched")
	}
	dir, err := os.MkdirTemp(c.dataPath, ".csrf-tokens-unread-*")
	if err != nil {
		return "", fmt.Errorf("prepare private CSRF preservation directory: %w", err)
	}
	preserved := filepath.Join(dir, "csrf_tokens.json")
	if err := os.Rename(path, preserved); err != nil {
		return "", errors.Join(fmt.Errorf("preserve unread CSRF token file: %w", err), os.Remove(dir))
	}
	if err := os.Chmod(preserved, 0600); err != nil {
		// The private parent still protects the original. Do not put unknown
		// tokens back into live storage if ownership prevents normalization.
		log.Warn().Err(err).Str("preserved_file", preserved).Msg("CSRF token file retained in a private directory but its permissions could not be normalized; backup readers may need filesystem repair")
	}
	return preserved, nil
}

// persistenceSnapshot copies token values, not pointers or live slice headers.
// Pruning and bounded replacement may mutate those slices as soon as mu is free.
func (c *CSRFTokenStore) persistenceSnapshot() ([]CSRFTokenData, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	persisted := make([]CSRFTokenData, 0, c.tokenRecordCountUnsafe())
	for sessionKey, tokens := range c.tokens {
		for _, token := range tokens {
			if token == nil {
				continue
			}
			persisted = append(persisted, CSRFTokenData{
				TokenHash:  token.Hash,
				SessionKey: sessionKey,
				ExpiresAt:  token.Expires,
			})
		}
	}
	return persisted, len(c.tokens)
}

// persistSnapshot runs under saveMu, with no token mutex held.
func (c *CSRFTokenStore) persistSnapshot(persisted []CSRFTokenData, sessions int) {
	csrfFile := filepath.Join(c.dataPath, "csrf_tokens.json")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(c.dataPath, 0700); err != nil {
		log.Error().Err(err).Msg("Failed to create CSRF tokens directory")
		return
	}

	// Marshal tokens
	jsonData, err := json.Marshal(persisted)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal CSRF tokens")
		return
	}

	if err := writePrivateAuthSnapshot(csrfFile, jsonData, c.snapshotWrite); err != nil {
		log.Error().Err(err).Msg("Failed to publish private CSRF tokens file")
		return
	}

	log.Debug().
		Int("sessions", sessions).
		Int("tokens", len(persisted)).
		Msg("CSRF tokens saved to disk")
}

// load reads CSRF tokens from disk
func (c *CSRFTokenStore) load() {
	csrfFile := filepath.Join(c.dataPath, "csrf_tokens.json")

	data, err := readAuthSnapshot(csrfFile)
	if err != nil {
		_, entryErr := os.Lstat(csrfFile)
		if !os.IsNotExist(err) || !os.IsNotExist(entryErr) {
			c.preserveBeforeSave = true
			log.Error().Err(err).Msg("Failed to read CSRF tokens file; existing tokens were not loaded. The next save or shutdown will preserve it outside active storage, not restore it; refresh CSRF tokens")
		}
		return
	}

	c.tokens = make(map[string][]*CSRFToken)

	var current []*CSRFTokenData
	if err := json.Unmarshal(data, &current); err == nil {
		for _, record := range current {
			if record == nil || strings.TrimSpace(record.SessionKey) == "" || strings.TrimSpace(record.TokenHash) == "" {
				c.preserveBeforeSave = true
				log.Error().Msg("Incomplete CSRF credential record; existing tokens were not loaded. The next save or shutdown will preserve the file outside active storage, not restore it; refresh CSRF tokens")
				return
			}
		}
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
	c.preserveBeforeSave = true
	log.Error().Msg("Failed to decode CSRF tokens file; existing tokens were not loaded. The next save or shutdown will preserve it outside active storage, not restore it; refresh CSRF tokens")
}
