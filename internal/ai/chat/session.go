package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// SessionStore manages chat sessions persisted as JSON files
type SessionStore struct {
	mu      sync.RWMutex
	dataDir string
}

// sessionData is the on-disk format for a session
type sessionData struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewSessionStore creates a new session store
func NewSessionStore(dataDir string) (*SessionStore, error) {
	sessionsDir := filepath.Join(dataDir, "ai_sessions")
	if err := os.MkdirAll(sessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &SessionStore{
		dataDir: sessionsDir,
	}, nil
}

// sessionPath returns the file path for a session
func (s *SessionStore) sessionPath(id string) string {
	return filepath.Join(s.dataDir, id+".json")
}

// List returns all sessions, sorted by updated_at descending
func (s *SessionStore) List() ([]Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := s.readSession(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to read session file")
			continue
		}

		sessions = append(sessions, Session{
			ID:           data.ID,
			Title:        data.Title,
			CreatedAt:    data.CreatedAt,
			UpdatedAt:    data.UpdatedAt,
			MessageCount: len(data.Messages),
		})
	}

	// Sort by updated_at descending (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Create creates a new session
func (s *SessionStore) Create() (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	data := sessionData{
		ID:        uuid.New().String(),
		Title:     "",
		Messages:  []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.writeSession(data); err != nil {
		return nil, err
	}

	return &Session{
		ID:        data.ID,
		Title:     data.Title,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}, nil
}

// Get retrieves a session by ID
func (s *SessionStore) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.readSession(id)
	if err != nil {
		return nil, err
	}

	return &Session{
		ID:           data.ID,
		Title:        data.Title,
		CreatedAt:    data.CreatedAt,
		UpdatedAt:    data.UpdatedAt,
		MessageCount: len(data.Messages),
	}, nil
}

// Delete deletes a session
func (s *SessionStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.sessionPath(id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session not found: %s", id)
		}
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// GetMessages retrieves all messages for a session
func (s *SessionStore) GetMessages(id string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.readSession(id)
	if err != nil {
		return nil, err
	}

	return data.Messages, nil
}

// AddMessage adds a message to a session
func (s *SessionStore) AddMessage(id string, msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return err
	}

	// Generate message ID if not set
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	data.Messages = append(data.Messages, msg)
	data.UpdatedAt = time.Now()

	// Auto-generate title from first user message if not set
	if data.Title == "" && msg.Role == "user" && msg.Content != "" {
		data.Title = generateTitle(msg.Content)
	}

	return s.writeSession(*data)
}

// UpdateLastMessage updates the last message in a session (for streaming updates)
func (s *SessionStore) UpdateLastMessage(id string, msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return err
	}

	if len(data.Messages) == 0 {
		return fmt.Errorf("no messages to update")
	}

	data.Messages[len(data.Messages)-1] = msg
	data.UpdatedAt = time.Now()

	return s.writeSession(*data)
}

// readSession reads a session from disk (caller must hold lock)
func (s *SessionStore) readSession(id string) (*sessionData, error) {
	path := s.sessionPath(id)
	file, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read session: %w", err)
	}

	var data sessionData
	if err := json.Unmarshal(file, &data); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}

	return &data, nil
}

// writeSession writes a session to disk (caller must hold lock)
func (s *SessionStore) writeSession(data sessionData) error {
	file, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	path := s.sessionPath(data.ID)
	if err := os.WriteFile(path, file, 0600); err != nil {
		return fmt.Errorf("failed to write session: %w", err)
	}

	return nil
}

// generateTitle creates a session title from the first user message
func generateTitle(content string) string {
	// Clean up the content
	content = strings.TrimSpace(content)
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.ReplaceAll(content, "\r", " ")

	// Collapse multiple spaces
	for strings.Contains(content, "  ") {
		content = strings.ReplaceAll(content, "  ", " ")
	}

	const maxLen = 50

	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}

	// Find a good break point
	truncated := string(runes[:maxLen])
	lastSpace := strings.LastIndex(truncated, " ")

	if lastSpace > 20 {
		return truncated[:lastSpace] + "..."
	}

	return truncated + "..."
}

// EnsureSession ensures a session exists, creating one if needed
func (s *SessionStore) EnsureSession(id string) (*Session, error) {
	if id == "" {
		return s.Create()
	}

	session, err := s.Get(id)
	if err != nil {
		// Session doesn't exist, create it with the specified ID
		s.mu.Lock()
		defer s.mu.Unlock()

		now := time.Now()
		data := sessionData{
			ID:        id,
			Title:     "",
			Messages:  []Message{},
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := s.writeSession(data); err != nil {
			return nil, err
		}

		return &Session{
			ID:        data.ID,
			CreatedAt: data.CreatedAt,
			UpdatedAt: data.UpdatedAt,
		}, nil
	}

	return session, nil
}
