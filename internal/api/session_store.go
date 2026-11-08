package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rs/zerolog/log"
)

// SessionStore handles persistent session storage
type SessionStore struct {
	sessions   map[string]*SessionData
	mu         sync.RWMutex
	persistMu  sync.Mutex // serializes snapshot selection and disk writes; never acquire while holding mu
	dataPath   string
	saveTicker *time.Ticker
	stopChan   chan bool
	workerDone chan struct{}
	stopOnce   sync.Once
	crypto     *crypto.CryptoManager
	// A failed startup read cannot authorize replacing the unknown inventory.
	// load sets this before workers start; later access belongs to persistMu.
	preserveBeforeSave    bool
	preserveFailureLogged bool
	// Refresh ownership is transient and belongs to this store, not the global
	// store selector or a raw-token-only process-wide guard.
	oidcRefreshes map[string]*oidcSessionRefresh
}

type oidcSessionRefresh struct {
	session  *SessionData
	snapshot SessionData
}

func sessionHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type sessionPersisted struct {
	Key              string        `json:"key"`
	Username         string        `json:"username,omitempty"`
	DisplayUsername  string        `json:"display_username,omitempty"`
	RecoveryBypass   bool          `json:"recovery_bypass,omitempty"`
	ExpiresAt        time.Time     `json:"expires_at"`
	CreatedAt        time.Time     `json:"created_at"`
	UserAgent        string        `json:"user_agent,omitempty"`
	IP               string        `json:"ip,omitempty"`
	OriginalDuration time.Duration `json:"original_duration,omitempty"`
	// OIDC token fields for refresh token support
	OIDCRefreshToken        string    `json:"oidc_refresh_token,omitempty"`
	OIDCAccessTokenExp      time.Time `json:"oidc_access_token_exp,omitempty"`
	OIDCAccessTokenIssuedAt time.Time `json:"oidc_access_token_issued_at,omitempty"`
	OIDCIssuer              string    `json:"oidc_issuer,omitempty"`
	OIDCClientID            string    `json:"oidc_client_id,omitempty"`
	OIDCTokenRefreshing     bool      `json:"-"` // transient, not persisted
	// SAML session fields for Single Logout (SLO) support
	SAMLProviderID   string `json:"saml_provider_id,omitempty"`
	SAMLNameID       string `json:"saml_name_id,omitempty"`
	SAMLSessionIndex string `json:"saml_session_index,omitempty"`
}

// SessionData represents a user session
type SessionData struct {
	Username         string        `json:"username,omitempty"`         // Stable authenticated principal
	DisplayUsername  string        `json:"display_username,omitempty"` // Human-readable session label
	RecoveryBypass   bool          `json:"recovery_bypass,omitempty"`
	ExpiresAt        time.Time     `json:"expires_at"`
	CreatedAt        time.Time     `json:"created_at"`
	UserAgent        string        `json:"user_agent,omitempty"`
	IP               string        `json:"ip,omitempty"`
	OriginalDuration time.Duration `json:"original_duration,omitempty"` // Track original duration for sliding expiration
	// OIDC token fields for refresh token support
	OIDCRefreshToken        string    `json:"oidc_refresh_token,omitempty"`          // Encrypted at rest
	OIDCAccessTokenExp      time.Time `json:"oidc_access_token_exp,omitempty"`       // When the access token expires
	OIDCAccessTokenIssuedAt time.Time `json:"oidc_access_token_issued_at,omitempty"` // When this access token was received
	OIDCIssuer              string    `json:"oidc_issuer,omitempty"`                 // IdP issuer URL
	OIDCClientID            string    `json:"oidc_client_id,omitempty"`              // OIDC client ID
	OIDCTokenRefreshing     bool      `json:"-"`                                     // Prevents concurrent refresh attempts
	// SAML session fields for Single Logout (SLO) support
	SAMLProviderID   string `json:"saml_provider_id,omitempty"`   // SAML IdP provider ID
	SAMLNameID       string `json:"saml_name_id,omitempty"`       // SAML NameID from assertion
	SAMLSessionIndex string `json:"saml_session_index,omitempty"` // SAML SessionIndex for SLO
}

func (s *SessionStore) loadHashedSessions(persisted []sessionPersisted, now time.Time) int {
	loaded := 0
	for _, entry := range persisted {
		if now.After(entry.ExpiresAt) {
			continue
		}
		refreshToken := entry.OIDCRefreshToken
		// Refresh tokens must be encrypted at rest in v6.
		if refreshToken != "" {
			if s.crypto != nil {
				if decrypted, err := s.crypto.DecryptString(refreshToken); err == nil {
					refreshToken = decrypted
				} else {
					log.Warn().
						Err(err).
						Str("session_key", entry.Key).
						Msg("Skipping invalid OIDC refresh token; encrypted format required")
					refreshToken = ""
				}
			} else {
				log.Warn().
					Str("session_key", entry.Key).
					Msg("Skipping persisted OIDC refresh token because session-store crypto is unavailable")
				refreshToken = ""
			}
		}

		s.sessions[entry.Key] = &SessionData{
			Username:                entry.Username,
			DisplayUsername:         entry.DisplayUsername,
			RecoveryBypass:          entry.RecoveryBypass,
			ExpiresAt:               entry.ExpiresAt,
			CreatedAt:               entry.CreatedAt,
			UserAgent:               entry.UserAgent,
			IP:                      entry.IP,
			OriginalDuration:        entry.OriginalDuration,
			OIDCRefreshToken:        refreshToken,
			OIDCAccessTokenExp:      entry.OIDCAccessTokenExp,
			OIDCAccessTokenIssuedAt: entry.OIDCAccessTokenIssuedAt,
			OIDCIssuer:              entry.OIDCIssuer,
			OIDCClientID:            entry.OIDCClientID,
			SAMLProviderID:          entry.SAMLProviderID,
			SAMLNameID:              entry.SAMLNameID,
			SAMLSessionIndex:        entry.SAMLSessionIndex,
		}
		loaded++
	}
	return loaded
}

func (s *SessionStore) loadLegacySessions(legacy map[string]SessionData, now time.Time) int {
	loaded := 0
	for rawToken, entry := range legacy {
		if rawToken == "" || now.After(entry.ExpiresAt) {
			continue
		}

		session := entry
		s.sessions[sessionHash(rawToken)] = &session
		loaded++
	}
	return loaded
}

func (s *SessionStore) migrateLegacySessions(data []byte, now time.Time) (bool, error) {
	var legacy map[string]SessionData
	if err := json.Unmarshal(data, &legacy); err != nil {
		return false, err
	}

	loaded := s.loadLegacySessions(legacy, now)
	log.Info().Int("loaded", loaded).Int("total", len(legacy)).Msg("Sessions loaded from disk (legacy raw-token format)")
	s.save()
	return true, nil
}

// NewSessionStore creates a new persistent session store
func NewSessionStore(dataPath string) *SessionStore {
	cm, err := crypto.NewCryptoManagerAt(dataPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize crypto manager for session store")
	}

	store := &SessionStore{
		sessions:   make(map[string]*SessionData),
		dataPath:   dataPath,
		stopChan:   make(chan bool),
		workerDone: make(chan struct{}),
		crypto:     cm,
	}

	// Load existing sessions from disk
	store.load()

	// Start periodic save and cleanup
	store.saveTicker = time.NewTicker(5 * time.Minute)
	go store.backgroundWorker()

	return store
}

func (s *SessionStore) Shutdown() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.saveTicker != nil {
			s.saveTicker.Stop()
		}
		if s.stopChan != nil {
			close(s.stopChan)
		}
	})
	if s.workerDone != nil {
		<-s.workerDone
	}
}

// backgroundWorker handles periodic saves and cleanup
func (s *SessionStore) backgroundWorker() {
	defer close(s.workerDone)
	for {
		select {
		case <-s.saveTicker.C:
			s.cleanup()
			s.save()
		case <-s.stopChan:
			s.save()
			return
		}
	}
}

// CreateSession creates a new session
func (s *SessionStore) CreateSession(token string, duration time.Duration, userAgent, ip, username string) {
	s.mu.Lock()

	key := sessionHash(token)
	s.sessions[key] = &SessionData{
		Username:         username,
		DisplayUsername:  username,
		RecoveryBypass:   false,
		ExpiresAt:        time.Now().Add(duration),
		CreatedAt:        time.Now(),
		UserAgent:        userAgent,
		IP:               ip,
		OriginalDuration: duration,
	}

	// Save immediately for important operations
	s.mu.Unlock()
	s.save()
}

// CreateRecoverySession creates a localhost-bound recovery session for the
// configured browser client without disabling auth globally.
func (s *SessionStore) CreateRecoverySession(token string, duration time.Duration, userAgent, ip, username string) {
	s.mu.Lock()

	key := sessionHash(token)
	s.sessions[key] = &SessionData{
		Username:         username,
		DisplayUsername:  username,
		RecoveryBypass:   true,
		ExpiresAt:        time.Now().Add(duration),
		CreatedAt:        time.Now(),
		UserAgent:        userAgent,
		IP:               ip,
		OriginalDuration: duration,
	}

	s.mu.Unlock()
	s.save()
}

// OIDCTokenInfo contains OAuth2 token information from the IdP
type OIDCTokenInfo struct {
	RefreshToken   string
	AccessTokenExp time.Time
	IssuedAt       time.Time
	Issuer         string
	ClientID       string
}

// CreateOIDCSession creates a new session with OIDC token information
func (s *SessionStore) CreateOIDCSession(token string, duration time.Duration, userAgent, ip, username string, oidc *OIDCTokenInfo) {
	s.CreateOIDCSessionWithDisplayName(token, duration, userAgent, ip, username, username, oidc)
}

// CreateOIDCSessionWithDisplayName creates an OIDC session with a stable
// principal and a separate human-readable label for UI display.
func (s *SessionStore) CreateOIDCSessionWithDisplayName(token string, duration time.Duration, userAgent, ip, username, displayUsername string, oidc *OIDCTokenInfo) {
	s.mu.Lock()

	if displayUsername == "" {
		displayUsername = username
	}
	now := time.Now()
	key := sessionHash(token)
	session := &SessionData{
		Username:         username,
		DisplayUsername:  displayUsername,
		RecoveryBypass:   false,
		ExpiresAt:        now.Add(duration),
		CreatedAt:        now,
		UserAgent:        userAgent,
		IP:               ip,
		OriginalDuration: duration,
	}

	if oidc != nil {
		issuedAt := oidc.IssuedAt
		if issuedAt.IsZero() {
			issuedAt = now
		}
		session.OIDCRefreshToken = oidc.RefreshToken
		session.OIDCAccessTokenExp = oidc.AccessTokenExp
		session.OIDCAccessTokenIssuedAt = issuedAt
		session.OIDCIssuer = oidc.Issuer
		session.OIDCClientID = oidc.ClientID
	}

	s.sessions[key] = session

	// Save immediately for important operations
	s.mu.Unlock()
	s.save()
}

// SAMLTokenInfo contains SAML session information for Single Logout support
type SAMLTokenInfo struct {
	ProviderID   string // SAML IdP provider ID
	NameID       string // SAML NameID from assertion
	SessionIndex string // SAML SessionIndex for SLO
}

// CreateSAMLSession creates a new session with SAML session information
func (s *SessionStore) CreateSAMLSession(token string, duration time.Duration, userAgent, ip, username string, saml *SAMLTokenInfo) {
	s.CreateSAMLSessionWithDisplayName(token, duration, userAgent, ip, username, username, saml)
}

// CreateSAMLSessionWithDisplayName creates a SAML session with a stable
// principal and a separate human-readable label for UI display.
func (s *SessionStore) CreateSAMLSessionWithDisplayName(token string, duration time.Duration, userAgent, ip, username, displayUsername string, saml *SAMLTokenInfo) {
	s.mu.Lock()

	if displayUsername == "" {
		displayUsername = username
	}
	key := sessionHash(token)
	now := time.Now()
	session := &SessionData{
		Username:         username,
		DisplayUsername:  displayUsername,
		RecoveryBypass:   false,
		ExpiresAt:        now.Add(duration),
		CreatedAt:        now,
		UserAgent:        userAgent,
		IP:               ip,
		OriginalDuration: duration,
	}

	if saml != nil {
		session.SAMLProviderID = saml.ProviderID
		session.SAMLNameID = saml.NameID
		session.SAMLSessionIndex = saml.SessionIndex
	}

	s.sessions[key] = session

	// Save immediately for important operations
	s.mu.Unlock()
	s.save()
}

// GetSAMLSessionInfo returns SAML-specific session info for the given token
func (s *SessionStore) GetSAMLSessionInfo(token string) *SAMLTokenInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionHash(token)]
	if !exists || session.SAMLProviderID == "" {
		return nil
	}

	return &SAMLTokenInfo{
		ProviderID:   session.SAMLProviderID,
		NameID:       session.SAMLNameID,
		SessionIndex: session.SAMLSessionIndex,
	}
}

// GetSession returns a copy of the session data for the given token
func (s *SessionStore) GetSession(token string) *SessionData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionHash(token)]
	if !exists {
		return nil
	}

	// Return a copy to avoid race conditions
	copy := *session
	return &copy
}

// sameOIDCRefreshSource ignores sliding session expiry and the transient busy
// flag, but not the principal, session generation or token version that was
// observed before background work was scheduled.
func sameOIDCRefreshSource(a, b *SessionData) bool {
	return a != nil && b != nil && a.CreatedAt.Equal(b.CreatedAt) &&
		a.Username == b.Username && a.RecoveryBypass == b.RecoveryBypass &&
		a.OIDCIssuer == b.OIDCIssuer && a.OIDCClientID == b.OIDCClientID &&
		a.OIDCRefreshToken == b.OIDCRefreshToken &&
		a.OIDCAccessTokenIssuedAt.Equal(b.OIDCAccessTokenIssuedAt) &&
		a.OIDCAccessTokenExp.Equal(b.OIDCAccessTokenExp)
}

// beginOIDCRefresh atomically revalidates a queued snapshot and claims the live
// session. A caller queued behind a completed rotation must not send the old
// refresh token to the IdP after the previous claim has been released.
func (s *SessionStore) beginOIDCRefresh(token string, observed *SessionData) *oidcSessionRefresh {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionHash(token)
	session := s.sessions[key]
	now := time.Now()
	if !sameOIDCRefreshSource(session, observed) || !now.Before(session.ExpiresAt) ||
		!shouldRefreshOIDCSessionToken(now, session) || s.oidcRefreshes[key] != nil {
		return nil
	}
	if s.oidcRefreshes == nil {
		s.oidcRefreshes = make(map[string]*oidcSessionRefresh)
	}
	attempt := &oidcSessionRefresh{session: session, snapshot: *session}
	s.oidcRefreshes[key] = attempt
	session.OIDCTokenRefreshing = true
	return attempt
}

func (s *SessionStore) endOIDCRefresh(token string, attempt *oidcSessionRefresh) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionHash(token)
	if s.oidcRefreshes[key] != attempt {
		return
	}
	delete(s.oidcRefreshes, key)
	if session := s.sessions[key]; session == attempt.session {
		session.OIDCTokenRefreshing = false
	}
}

// completeOIDCRefresh applies an IdP outcome only to its still-live source
// session and token version. A nil result invalidates that session; it must
// never delete a replacement or a newer independently refreshed credential.
func (s *SessionStore) completeOIDCRefresh(token string, attempt *oidcSessionRefresh, result *OIDCRefreshResult) bool {
	s.mu.Lock()

	key := sessionHash(token)
	session := s.sessions[key]
	now := time.Now()
	if s.oidcRefreshes[key] != attempt || session != attempt.session ||
		!sameOIDCRefreshSource(session, &attempt.snapshot) || !now.Before(session.ExpiresAt) {
		s.mu.Unlock()
		return false
	}
	if result == nil {
		delete(s.sessions, key)
	} else {
		session.OIDCRefreshToken = result.RefreshToken
		session.OIDCAccessTokenExp = result.Expiry
		session.OIDCAccessTokenIssuedAt = now
		session.OIDCTokenRefreshing = false
		if session.OriginalDuration > 0 {
			session.ExpiresAt = now.Add(session.OriginalDuration)
		}
	}
	// Keep persistence outside the authentication mutex. save selects its
	// snapshot after acquiring persistMu, so a logout queued during this write
	// cannot later be overwritten by an older refresh snapshot.
	s.mu.Unlock()
	s.save()
	return true
}

// UpdateOIDCTokens updates the OIDC tokens for a session after a successful refresh
func (s *SessionStore) UpdateOIDCTokens(token string, refreshToken string, accessTokenExp time.Time) {
	s.mu.Lock()

	now := time.Now()
	key := sessionHash(token)
	session, exists := s.sessions[key]
	if !exists {
		s.mu.Unlock()
		return
	}

	session.OIDCRefreshToken = refreshToken
	session.OIDCAccessTokenExp = accessTokenExp
	session.OIDCAccessTokenIssuedAt = now
	session.OIDCTokenRefreshing = false

	// Also extend the session expiry since the token is still valid
	if session.OriginalDuration > 0 {
		session.ExpiresAt = now.Add(session.OriginalDuration)
	}

	// Save immediately after token refresh
	s.mu.Unlock()
	s.save()
}

// InvalidateSession removes a session (used when OIDC refresh fails)
func (s *SessionStore) InvalidateSession(token string) {
	s.DeleteSession(token)
}

// SetTokenRefreshing marks a session as currently refreshing tokens
func (s *SessionStore) SetTokenRefreshing(token string, refreshing bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionHash(token)
	if session, exists := s.sessions[key]; exists {
		session.OIDCTokenRefreshing = refreshing
	}
}

// ValidateSession checks if a session is valid
func (s *SessionStore) ValidateSession(token string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionHash(token)]
	if !exists {
		return false
	}

	return time.Now().Before(session.ExpiresAt)
}

// ValidateAndExtendSession checks if a session is valid and extends it (sliding expiration)
func (s *SessionStore) ValidateAndExtendSession(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionHash(token)
	session, exists := s.sessions[key]
	if !exists {
		return false
	}

	now := time.Now()
	if now.After(session.ExpiresAt) {
		return false
	}

	// Extend session using the original duration (sliding window)
	if session.OriginalDuration > 0 {
		session.ExpiresAt = now.Add(session.OriginalDuration)
		// Note: We don't save immediately for performance, background worker will save periodically
	}

	return true
}

// DeleteSession removes a session
func (s *SessionStore) DeleteSession(token string) {
	s.removeSessionKeys([]string{sessionHash(token)})
	s.save()
}

// removeSessionKeys withdraws live authority without disk I/O. Coordinated
// callers must save after releasing the session-tracking lock, before returning.
func (s *SessionStore) removeSessionKeys(keys []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		delete(s.sessions, key)
	}
}

// DeleteSessionsForUser removes every persisted session for a stable
// principal and returns their already-hashed storage keys so the matching
// CSRF records can be removed without access to raw session tokens.
func (s *SessionStore) DeleteSessionsForUser(username string) []string {
	deletedKeys := s.removeSessionsForUser(username)
	// An unread startup inventory cannot establish that this user has no saved
	// sessions. Even with no live match, retire that unknown authority through
	// the same preservation-before-publication path as every other revocation.
	s.save()
	return deletedKeys
}

// removeSessionsForUser is the live half of account-wide withdrawal. The caller
// must still reach a save boundary, including when no live sessions matched.
func (s *SessionStore) removeSessionsForUser(username string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	deletedKeys := make([]string, 0)
	for key, session := range s.sessions {
		if session != nil && session.Username == username {
			delete(s.sessions, key)
			deletedKeys = append(deletedKeys, key)
		}
	}
	return deletedKeys
}

// cleanup removes expired sessions
func (s *SessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, key)
			log.Debug().Str("sessionKey", safePrefixForLog(key, 8)+"...").Msg("Cleaned up expired session")
		}
	}
}

// save selects and persists a detached snapshot. It must be called without mu:
// slow storage or encryption must not stall authentication, sliding expiry or
// live withdrawal. Mutations still wait for their save before returning.
//
// Select the snapshot AFTER acquiring persistMu, not before. Otherwise a save
// queued behind newer state could later overwrite it and resurrect a session
// deleted by another caller. No path acquires persistMu while holding mu.
func (s *SessionStore) save() {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()

	if s.preserveBeforeSave {
		preserved, err := s.preserveUnreadSessions()
		if err != nil {
			if !s.preserveFailureLogged {
				log.Error().Err(err).Msg("Cannot preserve unread session file; disk saves are disabled until it can be preserved. New sign-ins are memory-only and session revocations are not durable. Preserve or move sessions.json before restarting; do not restore stale session authority")
				s.preserveFailureLogged = true
			}
			return
		}
		s.preserveBeforeSave = false
		if preserved != "" {
			log.Warn().Str("preserved_file", preserved).Msg("Unread session file preserved outside active storage; it will not be loaded automatically. Sign in again rather than restoring stale session authority")
		} else {
			log.Warn().Msg("Unread session file is no longer in active storage; saving only the current authenticated sessions")
		}
	}

	persisted := s.persistenceSnapshot()
	s.persistSnapshot(persisted)
}

// preserveUnreadSessions retains the original inode before any fresh snapshot
// can replace it. An exclusive owner-private directory protects even permissive
// legacy files without opening, copying or changing their contents. Retained
// files become owner-readable so ordinary data backup readers can retain them.
// Only canonical sessions.json is loaded; retained bytes are not saved authority.
// Called under persistMu, never the authentication mutex. Failure keeps the
// original path and write hold, so a later save can recover after storage does.
func (s *SessionStore) preserveUnreadSessions() (string, error) {
	return preserveUnreadSessionFile(s.dataPath, os.Rename)
}

func preserveUnreadSessionFile(dataPath string, rename func(string, string) error) (string, error) {
	path := filepath.Join(dataPath, "sessions.json")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		// An operator may have already moved the unknown file out of active
		// storage. Its absence grants no old sessions and need not hold saves.
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect unread session file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("unread session path is not a regular file; leaving it untouched")
	}
	dir, err := os.MkdirTemp(dataPath, ".sessions-unread-*")
	if err != nil {
		return "", fmt.Errorf("prepare private session preservation directory: %w", err)
	}
	preserved := filepath.Join(dir, "sessions.json")
	if err := rename(path, preserved); err != nil {
		return "", errors.Join(fmt.Errorf("preserve unread session file: %w", err), os.Remove(dir))
	}
	if err := os.Chmod(preserved, 0600); err != nil {
		// The private directory still protects the original, and it is no
		// longer live authority. Do not restore it or block the fresh inventory
		// because a differently owned retained file needs filesystem repair.
		log.Warn().Err(err).Str("preserved_file", preserved).Msg("Session file retained in a private directory but its permissions could not be normalized; backup readers may need filesystem repair")
	}
	return preserved, nil
}

// persistenceSnapshot owns every mutable field before mu is released. Token
// encryption, marshaling, filesystem operations and logging happen afterwards.
func (s *SessionStore) persistenceSnapshot() []sessionPersisted {
	s.mu.RLock()
	defer s.mu.RUnlock()
	persisted := make([]sessionPersisted, 0, len(s.sessions))
	for key, session := range s.sessions {
		persisted = append(persisted, sessionPersisted{
			Key:                     key,
			Username:                session.Username,
			DisplayUsername:         session.DisplayUsername,
			RecoveryBypass:          session.RecoveryBypass,
			ExpiresAt:               session.ExpiresAt,
			CreatedAt:               session.CreatedAt,
			UserAgent:               session.UserAgent,
			IP:                      session.IP,
			OriginalDuration:        session.OriginalDuration,
			OIDCRefreshToken:        session.OIDCRefreshToken,
			OIDCAccessTokenExp:      session.OIDCAccessTokenExp,
			OIDCAccessTokenIssuedAt: session.OIDCAccessTokenIssuedAt,
			OIDCIssuer:              session.OIDCIssuer,
			OIDCClientID:            session.OIDCClientID,
			SAMLProviderID:          session.SAMLProviderID,
			SAMLNameID:              session.SAMLNameID,
			SAMLSessionIndex:        session.SAMLSessionIndex,
		})
	}
	return persisted
}

// persistSnapshot is called only under persistMu, with no session mutex held.
func (s *SessionStore) persistSnapshot(persisted []sessionPersisted) {
	sessionsFile := filepath.Join(s.dataPath, "sessions.json")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(s.dataPath, 0700); err != nil {
		log.Error().Err(err).Msg("Failed to create sessions directory")
		return
	}

	for i := range persisted {
		// Refresh tokens must not be persisted unless they can be encrypted.
		if persisted[i].OIDCRefreshToken != "" {
			if s.crypto != nil {
				if encrypted, err := s.crypto.EncryptString(persisted[i].OIDCRefreshToken); err == nil {
					persisted[i].OIDCRefreshToken = encrypted
				} else {
					log.Error().Err(err).Msg("Failed to encrypt refresh token")
					// Don't persist if encryption fails to prevent leak
					persisted[i].OIDCRefreshToken = ""
				}
			} else {
				log.Warn().Msg("Skipping OIDC refresh token persistence because session-store crypto is unavailable")
				persisted[i].OIDCRefreshToken = ""
			}
		}
	}

	data, err := json.Marshal(persisted)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal sessions")
		return
	}

	// Write to temporary file first
	tmpFile := sessionsFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		log.Error().Err(err).Msg("Failed to write sessions file")
		return
	}

	// Atomic rename
	if err := os.Rename(tmpFile, sessionsFile); err != nil {
		log.Error().Err(err).Msg("Failed to rename sessions file")
		return
	}

	log.Debug().Int("count", len(persisted)).Msg("Sessions saved to disk")
}

// load reads sessions from disk
func (s *SessionStore) load() {
	sessionsFile := filepath.Join(s.dataPath, "sessions.json")

	data, err := os.ReadFile(sessionsFile)
	if err != nil {
		if !os.IsNotExist(err) {
			s.preserveBeforeSave = true
			log.Error().Err(err).Msg("Failed to read sessions file; existing sessions were not loaded. The next save or shutdown will preserve it outside active storage, not restore it; sign in again")
		}
		return
	}

	now := time.Now()
	s.sessions = make(map[string]*SessionData)

	var persisted []sessionPersisted
	if err := json.Unmarshal(data, &persisted); err == nil {
		loaded := s.loadHashedSessions(persisted, now)
		log.Info().Int("loaded", loaded).Int("total", len(persisted)).Msg("Sessions loaded from disk (hashed format)")
		return
	}

	if migrated, err := s.migrateLegacySessions(data, now); err == nil && migrated {
		return
	}
	s.preserveBeforeSave = true
	log.Error().Msg("Failed to decode sessions file; existing sessions were not loaded. The next save or shutdown will preserve it outside active storage, not restore it; sign in again")
}
