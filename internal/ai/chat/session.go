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

	// resolvedContexts holds per-session resolved resource contexts (in-memory only)
	// These are NOT persisted - resources should be re-resolved after restart
	// because infrastructure state may have changed
	resolvedContexts map[string]*ResolvedContext

	// sessionFSMs holds per-session workflow state machines (in-memory only)
	// These track the RESOLVING -> READING -> WRITING -> VERIFYING workflow
	// to ensure structural guarantees (must discover before write, verify after write)
	sessionFSMs map[string]*SessionFSM

	// sessionToolSets holds per-session tool allowlists (in-memory only).
	// These keep tool availability stable across turns while allowing additive expansion.
	sessionToolSets map[string]map[string]bool

	// knowledgeAccumulators holds per-session knowledge accumulators (in-memory only).
	// These extract and preserve key facts from tool results to prevent amnesia
	// when old tool results are compacted from the conversation context.
	knowledgeAccumulators map[string]*KnowledgeAccumulator
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
		dataDir:               sessionsDir,
		resolvedContexts:      make(map[string]*ResolvedContext),
		sessionFSMs:           make(map[string]*SessionFSM),
		sessionToolSets:       make(map[string]map[string]bool),
		knowledgeAccumulators: make(map[string]*KnowledgeAccumulator),
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

	// Also clean up resolved context, FSM, and knowledge accumulator
	delete(s.resolvedContexts, id)
	delete(s.sessionFSMs, id)
	delete(s.sessionToolSets, id)
	delete(s.knowledgeAccumulators, id)

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

// GetResolvedContext returns the resolved context for a session, creating one if needed
func (s *SessionStore) GetResolvedContext(sessionID string) *ResolvedContext {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, ok := s.resolvedContexts[sessionID]
	if !ok {
		ctx = NewResolvedContext(sessionID)
		s.resolvedContexts[sessionID] = ctx
	}
	return ctx
}

// GetSessionFSM returns the workflow FSM for a session, creating one if needed
func (s *SessionStore) GetSessionFSM(sessionID string) *SessionFSM {
	s.mu.Lock()
	defer s.mu.Unlock()

	fsm, ok := s.sessionFSMs[sessionID]
	if !ok {
		fsm = NewSessionFSM()
		s.sessionFSMs[sessionID] = fsm
	}
	return fsm
}

// GetKnowledgeAccumulator returns the knowledge accumulator for a session, creating one if needed.
// For user chat sessions, this persists across messages (facts accumulate during a conversation).
func (s *SessionStore) GetKnowledgeAccumulator(sessionID string) *KnowledgeAccumulator {
	s.mu.Lock()
	defer s.mu.Unlock()

	ka, ok := s.knowledgeAccumulators[sessionID]
	if !ok {
		ka = NewKnowledgeAccumulator()
		s.knowledgeAccumulators[sessionID] = ka
	}
	return ka
}

// NewKnowledgeAccumulatorForRun creates a fresh KA for a patrol run.
// Unlike GetKnowledgeAccumulator (which reuses a session-scoped KA),
// this always returns a new instance to avoid stale facts from prior runs.
func (s *SessionStore) NewKnowledgeAccumulatorForRun(sessionID string) *KnowledgeAccumulator {
	s.mu.Lock()
	defer s.mu.Unlock()

	ka := NewKnowledgeAccumulator()
	s.knowledgeAccumulators[sessionID] = ka
	return ka
}

// ResetSessionFSM resets the FSM for a session (e.g., after context clear)
func (s *SessionStore) ResetSessionFSM(sessionID string, keepProgress bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fsm, ok := s.sessionFSMs[sessionID]
	if ok {
		if keepProgress {
			fsm.ResetKeepProgress()
		} else {
			fsm.Reset()
		}
	}
}

// AddResolvedResource adds a resolved resource to a session's context
func (s *SessionStore) AddResolvedResource(sessionID, name string, res *ResolvedResource) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, ok := s.resolvedContexts[sessionID]
	if !ok {
		ctx = NewResolvedContext(sessionID)
		s.resolvedContexts[sessionID] = ctx
	}
	ctx.AddResource(name, res)

	log.Debug().
		Str("session_id", sessionID).
		Str("name", name).
		Str("resource_id", res.ResourceID).
		Str("resource_type", res.ResourceType).
		Str("target_host", res.TargetHost).
		Msg("[SessionStore] Added resolved resource to context")
}

// ValidateResourceForAction validates that a resource can perform an action
// Returns the resolved resource if valid, error if not
func (s *SessionStore) ValidateResourceForAction(sessionID, resourceID, action string) (*ResolvedResource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ctx, ok := s.resolvedContexts[sessionID]
	if !ok {
		return nil, &ResourceNotResolvedError{ResourceID: resourceID}
	}

	if err := ctx.ValidateAction(resourceID, action); err != nil {
		return nil, err
	}

	res, _ := ctx.GetResourceByID(resourceID)
	return res, nil
}

// ClearResolvedContext removes the resolved context for a session
func (s *SessionStore) ClearResolvedContext(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.resolvedContexts, sessionID)
}

// ClearSessionState clears both resolved context and FSM coherently.
// This is the preferred method when clearing session state.
// - keepPinned=false: Full reset (RESOLVING state, no resources)
// - keepPinned=true: Keep pinned resources, FSM stays in READING if resources exist
func (s *SessionStore) ClearSessionState(sessionID string, keepPinned bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear resolved context
	ctx, hasCtx := s.resolvedContexts[sessionID]
	if hasCtx {
		ctx.Clear(keepPinned)
	}

	if !keepPinned {
		delete(s.sessionToolSets, sessionID)
		delete(s.knowledgeAccumulators, sessionID)
	}

	// Reset FSM coherently with context state
	fsm, hasFSM := s.sessionFSMs[sessionID]
	if hasFSM {
		if !keepPinned {
			// Full reset: back to RESOLVING (must discover again)
			fsm.Reset()
		} else if hasCtx && ctx.HasAnyResources() {
			// Pinned resources remain: keep progress (stay in READING if possible)
			fsm.ResetKeepProgress()
		} else {
			// keepPinned=true but no resources left: must rediscover
			fsm.Reset()
		}
	}

	log.Debug().
		Str("session_id", sessionID).
		Bool("keep_pinned", keepPinned).
		Bool("has_resources", hasCtx && ctx.HasAnyResources()).
		Str("fsm_state", func() string {
			if hasFSM {
				return string(fsm.State)
			}
			return "none"
		}()).
		Msg("[SessionStore] Cleared session state")
}

// cleanupResolvedContext is called when a session is deleted to also remove its context
func (s *SessionStore) cleanupResolvedContext(sessionID string) {
	// Note: caller must NOT hold the lock (or use a separate lock for contexts)
	delete(s.resolvedContexts, sessionID)
}

// GetToolSet returns a copy of the tool allowlist for a session, or nil if none set.
func (s *SessionStore) GetToolSet(sessionID string) map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	toolSet, ok := s.sessionToolSets[sessionID]
	if !ok {
		return nil
	}
	return copyToolSet(toolSet)
}

// SetToolSet stores a tool allowlist for a session.
func (s *SessionStore) SetToolSet(sessionID string, toolSet map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessionToolSets[sessionID] = copyToolSet(toolSet)
}

// AddToolSet merges tool allowlist entries into the session's tool set.
// Returns a copy of the updated tool set.
func (s *SessionStore) AddToolSet(sessionID string, additions map[string]bool) map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	toolSet, ok := s.sessionToolSets[sessionID]
	if !ok {
		toolSet = make(map[string]bool)
	}
	for name := range additions {
		toolSet[name] = true
	}
	s.sessionToolSets[sessionID] = toolSet
	return copyToolSet(toolSet)
}

func copyToolSet(source map[string]bool) map[string]bool {
	if source == nil {
		return nil
	}
	out := make(map[string]bool, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}
