package investigation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Store manages investigation sessions with persistence
type Store struct {
	mu           sync.RWMutex
	sessions     map[string]*InvestigationSession // keyed by ID
	byFinding    map[string][]string              // finding_id -> []session_id
	dataDir      string
	saveTimer    *time.Timer
	savePending  bool
	saveDebounce time.Duration
}

// NewStore creates a new investigation store
func NewStore(dataDir string) *Store {
	s := &Store{
		sessions:     make(map[string]*InvestigationSession),
		byFinding:    make(map[string][]string),
		dataDir:      dataDir,
		saveDebounce: 5 * time.Second,
	}
	return s
}

// LoadFromDisk loads investigation sessions from disk
func (s *Store) LoadFromDisk() error {
	if s.dataDir == "" {
		return nil
	}

	filePath := filepath.Join(s.dataDir, "investigations.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No file yet, that's ok
		}
		return fmt.Errorf("read investigation store file %q: %w", filePath, err)
	}

	var sessions []*InvestigationSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return fmt.Errorf("decode investigation store file %q: %w", filePath, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, session := range sessions {
		s.sessions[session.ID] = session
		s.byFinding[session.FindingID] = append(s.byFinding[session.FindingID], session.ID)
	}

	return nil
}

// scheduleSave schedules a debounced save operation
// IMPORTANT: Caller must NOT hold s.mu - this function acquires its own lock
func (s *Store) scheduleSave() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduleSaveLocked()
}

// scheduleSaveLocked schedules a debounced save operation
// IMPORTANT: Caller MUST hold s.mu
func (s *Store) scheduleSaveLocked() {
	if s.dataDir == "" || s.savePending {
		return
	}

	s.savePending = true
	saveDebounce := s.saveDebounce
	s.saveTimer = time.AfterFunc(saveDebounce, func() {
		s.mu.Lock()
		s.savePending = false
		sessions := make([]*InvestigationSession, 0, len(s.sessions))
		for _, session := range s.sessions {
			sessions = append(sessions, session)
		}
		dataDir := s.dataDir
		s.mu.Unlock()

		if dataDir != "" {
			if err := s.saveToDisk(sessions); err != nil {
				log.Warn().Err(err).Str("data_dir", dataDir).Msg("Failed to persist investigation sessions")
			}
		}
	})
}

// saveToDisk writes sessions to disk
func (s *Store) saveToDisk(sessions []*InvestigationSession) error {
	if s.dataDir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("create investigation data directory %q: %w", s.dataDir, err)
	}

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal investigation sessions: %w", err)
	}

	filePath := filepath.Join(s.dataDir, "investigations.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write investigation store file %q: %w", filePath, err)
	}
	return nil
}

// ForceSave immediately saves to disk
func (s *Store) ForceSave() error {
	s.mu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.savePending = false

	sessions := make([]*InvestigationSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	s.mu.Unlock()

	if err := s.saveToDisk(sessions); err != nil {
		return fmt.Errorf("force-save investigation store: %w", err)
	}
	return nil
}

// Create creates a new investigation session
func (s *Store) Create(findingID, chatSessionID string) *InvestigationSession {
	session := &InvestigationSession{
		ID:        uuid.New().String(),
		FindingID: findingID,
		SessionID: chatSessionID,
		Status:    StatusPending,
		StartedAt: time.Now(),
	}

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.byFinding[findingID] = append(s.byFinding[findingID], session.ID)
	s.mu.Unlock()

	s.scheduleSave()
	return session
}

// Get retrieves an investigation session by ID
func (s *Store) Get(id string) *InvestigationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if session, exists := s.sessions[id]; exists {
		// Return a copy
		copy := *session
		if session.ProposedFix != nil {
			fixCopy := *session.ProposedFix
			copy.ProposedFix = &fixCopy
		}
		return &copy
	}
	return nil
}

// GetByFinding returns all investigation sessions for a finding
func (s *Store) GetByFinding(findingID string) []*InvestigationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionIDs := s.byFinding[findingID]
	result := make([]*InvestigationSession, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		if session, exists := s.sessions[id]; exists {
			copy := *session
			if session.ProposedFix != nil {
				fixCopy := *session.ProposedFix
				copy.ProposedFix = &fixCopy
			}
			result = append(result, &copy)
		}
	}
	return result
}

// GetLatestByFinding returns the most recent investigation for a finding
func (s *Store) GetLatestByFinding(findingID string) *InvestigationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionIDs := s.byFinding[findingID]
	if len(sessionIDs) == 0 {
		return nil
	}

	var latest *InvestigationSession
	for _, id := range sessionIDs {
		if session, exists := s.sessions[id]; exists {
			if latest == nil || session.StartedAt.After(latest.StartedAt) {
				latest = session
			}
		}
	}

	if latest != nil {
		copy := *latest
		if latest.ProposedFix != nil {
			fixCopy := *latest.ProposedFix
			copy.ProposedFix = &fixCopy
		}
		return &copy
	}
	return nil
}

// GetRunning returns all currently running investigations
func (s *Store) GetRunning() []*InvestigationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*InvestigationSession
	for _, session := range s.sessions {
		if session.Status == StatusRunning {
			copy := *session
			if session.ProposedFix != nil {
				fixCopy := *session.ProposedFix
				copy.ProposedFix = &fixCopy
			}
			result = append(result, &copy)
		}
	}
	return result
}

// CountRunning returns the number of currently running investigations
func (s *Store) CountRunning() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, session := range s.sessions {
		if session.Status == StatusRunning {
			count++
		}
	}
	return count
}

// Update updates an investigation session
func (s *Store) Update(session *InvestigationSession) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[session.ID]; !exists {
		return false
	}

	// Make a copy to store
	copy := *session
	if session.ProposedFix != nil {
		fixCopy := *session.ProposedFix
		copy.ProposedFix = &fixCopy
	}
	s.sessions[session.ID] = &copy

	s.scheduleSaveLocked()
	return true
}

// UpdateStatus updates just the status of an investigation
func (s *Store) UpdateStatus(id string, status Status) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[id]
	if !exists {
		return false
	}

	session.Status = status
	s.scheduleSaveLocked()
	return true
}

// Complete marks an investigation as completed
func (s *Store) Complete(id string, outcome Outcome, summary string, proposedFix *Fix) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[id]
	if !exists {
		return false
	}

	now := time.Now()
	session.Status = StatusCompleted
	session.CompletedAt = &now
	session.Outcome = outcome
	session.Summary = summary
	if proposedFix != nil {
		fixCopy := *proposedFix
		session.ProposedFix = &fixCopy
	}

	s.scheduleSaveLocked()
	return true
}

// Fail marks an investigation as failed
func (s *Store) Fail(id string, errorMsg string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[id]
	if !exists {
		return false
	}

	now := time.Now()
	session.Status = StatusFailed
	session.CompletedAt = &now
	session.Error = errorMsg

	s.scheduleSaveLocked()
	return true
}

// SetOutcome updates just the outcome of an investigation
func (s *Store) SetOutcome(id string, outcome Outcome) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[id]
	if !exists {
		return false
	}

	session.Outcome = outcome
	s.scheduleSaveLocked()
	return true
}

// IncrementTurnCount increments the turn count for an investigation
func (s *Store) IncrementTurnCount(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[id]
	if !exists {
		return 0
	}

	session.TurnCount++
	count := session.TurnCount

	s.scheduleSaveLocked()
	return count
}

// SetApprovalID sets the approval ID for a queued fix
func (s *Store) SetApprovalID(id, approvalID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[id]
	if !exists {
		return false
	}

	session.ApprovalID = approvalID
	s.scheduleSaveLocked()
	return true
}

// GetAll returns all investigation sessions
func (s *Store) GetAll() []*InvestigationSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*InvestigationSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		copy := *session
		if session.ProposedFix != nil {
			fixCopy := *session.ProposedFix
			copy.ProposedFix = &fixCopy
		}
		result = append(result, &copy)
	}
	return result
}

// CountFixed returns the number of investigations that resulted in a fix being executed
func (s *Store) CountFixed() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, session := range s.sessions {
		if session.Outcome == OutcomeFixExecuted {
			count++
		}
	}
	return count
}

// Cleanup removes old completed/failed investigations and stuck sessions.
// A session is considered stuck if it has no CompletedAt and StartedAt is
// older than 20 minutes (2x the 10-minute investigation timeout).
func (s *Store) Cleanup(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	stuckCutoff := time.Now().Add(-20 * time.Minute)
	removed := 0

	for id, session := range s.sessions {
		shouldRemove := false

		// Remove old completed/failed investigations
		if session.CompletedAt != nil && session.CompletedAt.Before(cutoff) {
			shouldRemove = true
		}

		// Remove stuck sessions (no CompletedAt and started more than 20min ago)
		if session.CompletedAt == nil && session.StartedAt.Before(stuckCutoff) {
			shouldRemove = true
		}

		if shouldRemove {
			delete(s.sessions, id)
			// Clean up byFinding index
			findingIDs := s.byFinding[session.FindingID]
			for i, sid := range findingIDs {
				if sid == id {
					s.byFinding[session.FindingID] = append(findingIDs[:i], findingIDs[i+1:]...)
					break
				}
			}
			removed++
		}
	}

	// Clean up orphaned empty slices in byFinding
	for findingID, sessionIDs := range s.byFinding {
		if len(sessionIDs) == 0 {
			delete(s.byFinding, findingID)
		}
	}

	if removed > 0 {
		s.scheduleSaveLocked()
	}

	return removed
}

// EnforceSizeLimit removes the oldest completed sessions when the store
// exceeds maxSessions. Running/pending sessions are never evicted.
func (s *Store) EnforceSizeLimit(maxSessions int) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.sessions) <= maxSessions {
		return 0
	}

	// Collect completed sessions sorted by CompletedAt (oldest first)
	type entry struct {
		id          string
		completedAt time.Time
	}
	var completed []entry
	for id, session := range s.sessions {
		if session.CompletedAt != nil {
			completed = append(completed, entry{id: id, completedAt: *session.CompletedAt})
		}
	}

	// Sort oldest first
	for i := 0; i < len(completed)-1; i++ {
		for j := i + 1; j < len(completed); j++ {
			if completed[j].completedAt.Before(completed[i].completedAt) {
				completed[i], completed[j] = completed[j], completed[i]
			}
		}
	}

	toRemove := len(s.sessions) - maxSessions
	removed := 0
	for _, e := range completed {
		if removed >= toRemove {
			break
		}
		session := s.sessions[e.id]
		delete(s.sessions, e.id)
		// Clean up byFinding index
		findingIDs := s.byFinding[session.FindingID]
		for i, sid := range findingIDs {
			if sid == e.id {
				s.byFinding[session.FindingID] = append(findingIDs[:i], findingIDs[i+1:]...)
				break
			}
		}
		removed++
	}

	if removed > 0 {
		s.scheduleSaveLocked()
	}

	return removed
}
