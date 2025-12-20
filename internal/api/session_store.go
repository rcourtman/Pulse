package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// SessionStore handles persistent session storage
type SessionStore struct {
	sessions   map[string]*SessionData
	mu         sync.RWMutex
	dataPath   string
	saveTicker *time.Ticker
	stopChan   chan bool
}

func sessionHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type sessionPersisted struct {
	Key              string        `json:"key"`
	ExpiresAt        time.Time     `json:"expires_at"`
	CreatedAt        time.Time     `json:"created_at"`
	UserAgent        string        `json:"user_agent,omitempty"`
	IP               string        `json:"ip,omitempty"`
	OriginalDuration time.Duration `json:"original_duration,omitempty"`
	// OIDC token fields for refresh token support
	OIDCRefreshToken    string    `json:"oidc_refresh_token,omitempty"`
	OIDCAccessTokenExp  time.Time `json:"oidc_access_token_exp,omitempty"`
	OIDCIssuer          string    `json:"oidc_issuer,omitempty"`
	OIDCClientID        string    `json:"oidc_client_id,omitempty"`
	OIDCTokenRefreshing bool      `json:"-"` // transient, not persisted
}

// SessionData represents a user session
type SessionData struct {
	ExpiresAt        time.Time     `json:"expires_at"`
	CreatedAt        time.Time     `json:"created_at"`
	UserAgent        string        `json:"user_agent,omitempty"`
	IP               string        `json:"ip,omitempty"`
	OriginalDuration time.Duration `json:"original_duration,omitempty"` // Track original duration for sliding expiration
	// OIDC token fields for refresh token support
	OIDCRefreshToken    string    `json:"oidc_refresh_token,omitempty"`    // Encrypted at rest
	OIDCAccessTokenExp  time.Time `json:"oidc_access_token_exp,omitempty"` // When the access token expires
	OIDCIssuer          string    `json:"oidc_issuer,omitempty"`           // IdP issuer URL
	OIDCClientID        string    `json:"oidc_client_id,omitempty"`        // OIDC client ID
	OIDCTokenRefreshing bool      `json:"-"`                               // Prevents concurrent refresh attempts
}

// NewSessionStore creates a new persistent session store
func NewSessionStore(dataPath string) *SessionStore {
	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: dataPath,
		stopChan: make(chan bool),
	}

	// Load existing sessions from disk
	store.load()

	// Start periodic save and cleanup
	store.saveTicker = time.NewTicker(5 * time.Minute)
	go store.backgroundWorker()

	return store
}

// backgroundWorker handles periodic saves and cleanup
func (s *SessionStore) backgroundWorker() {
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
func (s *SessionStore) CreateSession(token string, duration time.Duration, userAgent, ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionHash(token)
	s.sessions[key] = &SessionData{
		ExpiresAt:        time.Now().Add(duration),
		CreatedAt:        time.Now(),
		UserAgent:        userAgent,
		IP:               ip,
		OriginalDuration: duration,
	}

	// Save immediately for important operations
	s.saveUnsafe()
}

// OIDCTokenInfo contains OAuth2 token information from the IdP
type OIDCTokenInfo struct {
	RefreshToken   string
	AccessTokenExp time.Time
	Issuer         string
	ClientID       string
}

// CreateOIDCSession creates a new session with OIDC token information
func (s *SessionStore) CreateOIDCSession(token string, duration time.Duration, userAgent, ip string, oidc *OIDCTokenInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionHash(token)
	session := &SessionData{
		ExpiresAt:        time.Now().Add(duration),
		CreatedAt:        time.Now(),
		UserAgent:        userAgent,
		IP:               ip,
		OriginalDuration: duration,
	}

	if oidc != nil {
		session.OIDCRefreshToken = oidc.RefreshToken
		session.OIDCAccessTokenExp = oidc.AccessTokenExp
		session.OIDCIssuer = oidc.Issuer
		session.OIDCClientID = oidc.ClientID
	}

	s.sessions[key] = session

	// Save immediately for important operations
	s.saveUnsafe()
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

// UpdateOIDCTokens updates the OIDC tokens for a session after a successful refresh
func (s *SessionStore) UpdateOIDCTokens(token string, refreshToken string, accessTokenExp time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionHash(token)
	session, exists := s.sessions[key]
	if !exists {
		return
	}

	session.OIDCRefreshToken = refreshToken
	session.OIDCAccessTokenExp = accessTokenExp
	session.OIDCTokenRefreshing = false

	// Also extend the session expiry since the token is still valid
	if session.OriginalDuration > 0 {
		session.ExpiresAt = time.Now().Add(session.OriginalDuration)
	}

	// Save immediately after token refresh
	s.saveUnsafe()
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
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionHash(token))
	s.saveUnsafe()
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

// save persists sessions to disk
func (s *SessionStore) save() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.saveUnsafe()
}

// saveUnsafe saves without locking (caller must hold lock)
func (s *SessionStore) saveUnsafe() {
	sessionsFile := filepath.Join(s.dataPath, "sessions.json")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(s.dataPath, 0700); err != nil {
		log.Error().Err(err).Msg("Failed to create sessions directory")
		return
	}

	// Marshal sessions
	persisted := make([]sessionPersisted, 0, len(s.sessions))
	for key, session := range s.sessions {
		persisted = append(persisted, sessionPersisted{
			Key:                key,
			ExpiresAt:          session.ExpiresAt,
			CreatedAt:          session.CreatedAt,
			UserAgent:          session.UserAgent,
			IP:                 session.IP,
			OriginalDuration:   session.OriginalDuration,
			OIDCRefreshToken:   session.OIDCRefreshToken,
			OIDCAccessTokenExp: session.OIDCAccessTokenExp,
			OIDCIssuer:         session.OIDCIssuer,
			OIDCClientID:       session.OIDCClientID,
		})
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

	log.Debug().Int("count", len(s.sessions)).Msg("Sessions saved to disk")
}

// load reads sessions from disk
func (s *SessionStore) load() {
	sessionsFile := filepath.Join(s.dataPath, "sessions.json")

	data, err := os.ReadFile(sessionsFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Error().Err(err).Msg("Failed to read sessions file")
		}
		return
	}

	now := time.Now()
	s.sessions = make(map[string]*SessionData)

	var persisted []sessionPersisted
	if err := json.Unmarshal(data, &persisted); err == nil {
		for _, entry := range persisted {
			if now.After(entry.ExpiresAt) {
				continue
			}
			s.sessions[entry.Key] = &SessionData{
				ExpiresAt:          entry.ExpiresAt,
				CreatedAt:          entry.CreatedAt,
				UserAgent:          entry.UserAgent,
				IP:                 entry.IP,
				OriginalDuration:   entry.OriginalDuration,
				OIDCRefreshToken:   entry.OIDCRefreshToken,
				OIDCAccessTokenExp: entry.OIDCAccessTokenExp,
				OIDCIssuer:         entry.OIDCIssuer,
				OIDCClientID:       entry.OIDCClientID,
			}
		}
		log.Info().Int("loaded", len(s.sessions)).Int("total", len(persisted)).Msg("Sessions loaded from disk (hashed format)")
		return
	}

	// Legacy map format fallback (keys stored as raw tokens)
	var legacy map[string]*SessionData
	if err := json.Unmarshal(data, &legacy); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal legacy sessions")
		return
	}

	loaded := 0
	for token, session := range legacy {
		if now.After(session.ExpiresAt) {
			continue
		}
		s.sessions[sessionHash(token)] = session
		loaded++
	}

	log.Info().
		Int("loaded", loaded).
		Int("total", len(legacy)).
		Msg("Sessions loaded from disk (legacy format migrated)")
}
