package api

import (
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

// SessionData represents a user session
type SessionData struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UserAgent string    `json:"user_agent,omitempty"`
	IP        string    `json:"ip,omitempty"`
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

// Stop gracefully stops the session store
func (s *SessionStore) Stop() {
	s.saveTicker.Stop()
	s.stopChan <- true
	s.save()
}

// CreateSession creates a new session
func (s *SessionStore) CreateSession(token string, duration time.Duration, userAgent, ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[token] = &SessionData{
		Token:     token,
		ExpiresAt: time.Now().Add(duration),
		CreatedAt: time.Now(),
		UserAgent: userAgent,
		IP:        ip,
	}

	// Save immediately for important operations
	s.saveUnsafe()
}

// ValidateSession checks if a session is valid
func (s *SessionStore) ValidateSession(token string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[token]
	if !exists {
		return false
	}

	return time.Now().Before(session.ExpiresAt)
}

// ExtendSession extends the expiration of a session
func (s *SessionStore) ExtendSession(token string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session, exists := s.sessions[token]; exists {
		session.ExpiresAt = time.Now().Add(duration)
		s.saveUnsafe()
	}
}

// DeleteSession removes a session
func (s *SessionStore) DeleteSession(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, token)
	s.saveUnsafe()
}

// GetSession returns session data if it exists and is valid
func (s *SessionStore) GetSession(token string) *SessionData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[token]
	if !exists || time.Now().After(session.ExpiresAt) {
		return nil
	}

	return session
}

// cleanup removes expired sessions
func (s *SessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for token, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, token)
			log.Debug().Str("token", token[:8]+"...").Msg("Cleaned up expired session")
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
	data, err := json.Marshal(s.sessions)
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

	var sessions map[string]*SessionData
	if err := json.Unmarshal(data, &sessions); err != nil {
		log.Error().Err(err).Msg("Failed to unmarshal sessions")
		return
	}

	// Filter out expired sessions
	now := time.Now()
	loaded := 0
	for token, session := range sessions {
		if now.Before(session.ExpiresAt) {
			s.sessions[token] = session
			loaded++
		}
	}

	log.Info().Int("loaded", loaded).Int("total", len(sessions)).Msg("Sessions loaded from disk")
}

// ClearAll removes all sessions (use carefully)
func (s *SessionStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions = make(map[string]*SessionData)
	s.saveUnsafe()
	log.Info().Msg("All sessions cleared")
}
