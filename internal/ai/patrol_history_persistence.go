// Package ai provides AI-powered infrastructure monitoring and investigation.
package ai

import (
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// PatrolHistoryPersistence interface for saving/loading patrol run history
type PatrolHistoryPersistence interface {
	SavePatrolRunHistory(runs []PatrolRunRecord) error
	LoadPatrolRunHistory() ([]PatrolRunRecord, error)
}

// PatrolHistoryPersistenceAdapter bridges ConfigPersistence to PatrolHistoryPersistence interface
type PatrolHistoryPersistenceAdapter struct {
	config *config.ConfigPersistence
}

// NewPatrolHistoryPersistenceAdapter creates a new adapter
func NewPatrolHistoryPersistenceAdapter(cfg *config.ConfigPersistence) *PatrolHistoryPersistenceAdapter {
	return &PatrolHistoryPersistenceAdapter{config: cfg}
}

// SavePatrolRunHistory saves patrol run history to disk via ConfigPersistence
func (a *PatrolHistoryPersistenceAdapter) SavePatrolRunHistory(runs []PatrolRunRecord) error {
	// Convert from ai.PatrolRunRecord to config.PatrolRunRecord
	records := make([]config.PatrolRunRecord, len(runs))
	for i, r := range runs {
		durationMs := r.DurationMs
		if durationMs == 0 && r.Duration > 0 {
			durationMs = int64(r.Duration / time.Millisecond)
		}
		records[i] = config.PatrolRunRecord{
			ID:                 r.ID,
			StartedAt:          r.StartedAt,
			CompletedAt:        r.CompletedAt,
			DurationMs:         durationMs,
			Type:               r.Type,
			TriggerReason:      r.TriggerReason,
			ScopeResourceIDs:   r.ScopeResourceIDs,
			ScopeResourceTypes: r.ScopeResourceTypes,
			ScopeDepth:         r.ScopeDepth,
			ScopeContext:       r.ScopeContext,
			AlertID:            r.AlertID,
			FindingID:          r.FindingID,
			ResourcesChecked:   r.ResourcesChecked,
			NodesChecked:       r.NodesChecked,
			GuestsChecked:      r.GuestsChecked,
			DockerChecked:      r.DockerChecked,
			StorageChecked:     r.StorageChecked,
			HostsChecked:       r.HostsChecked,
			PBSChecked:         r.PBSChecked,
			KubernetesChecked:  r.KubernetesChecked,
			NewFindings:        r.NewFindings,
			ExistingFindings:   r.ExistingFindings,
			ResolvedFindings:   r.ResolvedFindings,
			AutoFixCount:       r.AutoFixCount,
			FindingsSummary:    r.FindingsSummary,
			FindingIDs:         r.FindingIDs,
			ErrorCount:         r.ErrorCount,
			Status:             r.Status,
			AIAnalysis:         r.AIAnalysis,
			InputTokens:        r.InputTokens,
			OutputTokens:       r.OutputTokens,
		}
	}
	return a.config.SavePatrolRunHistory(records)
}

// LoadPatrolRunHistory loads patrol run history from disk via ConfigPersistence
func (a *PatrolHistoryPersistenceAdapter) LoadPatrolRunHistory() ([]PatrolRunRecord, error) {
	data, err := a.config.LoadPatrolRunHistory()
	if err != nil {
		return nil, err
	}

	// Convert from config.PatrolRunRecord to ai.PatrolRunRecord
	runs := make([]PatrolRunRecord, len(data.Runs))
	for i, r := range data.Runs {
		runs[i] = PatrolRunRecord{
			ID:                 r.ID,
			StartedAt:          r.StartedAt,
			CompletedAt:        r.CompletedAt,
			Duration:           time.Duration(r.DurationMs) * time.Millisecond, // Convert milliseconds to nanoseconds
			DurationMs:         r.DurationMs,
			Type:               r.Type,
			TriggerReason:      r.TriggerReason,
			ScopeResourceIDs:   r.ScopeResourceIDs,
			ScopeResourceTypes: r.ScopeResourceTypes,
			ScopeDepth:         r.ScopeDepth,
			ScopeContext:       r.ScopeContext,
			AlertID:            r.AlertID,
			FindingID:          r.FindingID,
			ResourcesChecked:   r.ResourcesChecked,
			NodesChecked:       r.NodesChecked,
			GuestsChecked:      r.GuestsChecked,
			DockerChecked:      r.DockerChecked,
			StorageChecked:     r.StorageChecked,
			HostsChecked:       r.HostsChecked,
			PBSChecked:         r.PBSChecked,
			KubernetesChecked:  r.KubernetesChecked,
			NewFindings:        r.NewFindings,
			ExistingFindings:   r.ExistingFindings,
			ResolvedFindings:   r.ResolvedFindings,
			AutoFixCount:       r.AutoFixCount,
			FindingsSummary:    r.FindingsSummary,
			FindingIDs:         r.FindingIDs,
			ErrorCount:         r.ErrorCount,
			Status:             r.Status,
			AIAnalysis:         r.AIAnalysis,
			InputTokens:        r.InputTokens,
			OutputTokens:       r.OutputTokens,
		}
	}
	return runs, nil
}

// PatrolRunHistoryStore provides thread-safe storage for patrol run history with optional persistence
type PatrolRunHistoryStore struct {
	mu          sync.RWMutex
	runs        []PatrolRunRecord
	maxRuns     int
	persistence PatrolHistoryPersistence
	// Debounce save operations
	saveTimer    *time.Timer
	savePending  bool
	saveDebounce time.Duration

	// Error tracking for persistence
	lastSaveError error
	lastSaveTime  time.Time
	onSaveError   func(err error)
}

// NewPatrolRunHistoryStore creates a new patrol run history store
func NewPatrolRunHistoryStore(maxRuns int) *PatrolRunHistoryStore {
	if maxRuns <= 0 {
		maxRuns = MaxPatrolRunHistory
	}
	return &PatrolRunHistoryStore{
		runs:         make([]PatrolRunRecord, 0, maxRuns),
		maxRuns:      maxRuns,
		saveDebounce: 5 * time.Second,
	}
}

// SetPersistence sets the persistence layer and loads existing history
func (s *PatrolRunHistoryStore) SetPersistence(p PatrolHistoryPersistence) error {
	s.mu.Lock()
	s.persistence = p
	s.mu.Unlock()

	// Load existing history from disk
	if p != nil {
		runs, err := p.LoadPatrolRunHistory()
		if err != nil {
			return err
		}
		if len(runs) > 0 {
			s.mu.Lock()
			s.runs = runs
			// Trim to max if loaded more than maxRuns
			if len(s.runs) > s.maxRuns {
				s.runs = s.runs[:s.maxRuns]
			}
			s.mu.Unlock()
			log.Info().Int("count", len(runs)).Msg("Loaded patrol run history from disk")
		}
	}
	return nil
}

// Add adds a new patrol run to the history
func (s *PatrolRunHistoryStore) Add(run PatrolRunRecord) {
	if run.DurationMs == 0 && run.Duration > 0 {
		run.DurationMs = run.Duration.Milliseconds()
	}
	if run.Duration == 0 && run.DurationMs > 0 {
		run.Duration = time.Duration(run.DurationMs) * time.Millisecond
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Prepend (newest first)
	s.runs = append([]PatrolRunRecord{run}, s.runs...)

	// Trim to max
	if len(s.runs) > s.maxRuns {
		s.runs = s.runs[:s.maxRuns]
	}

	// Schedule save
	s.scheduleSaveLocked()
}

// GetAll returns all runs (newest first)
func (s *PatrolRunHistoryStore) GetAll() []PatrolRunRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]PatrolRunRecord, len(s.runs))
	copy(result, s.runs)
	return result
}

// GetRecent returns at most n recent runs
func (s *PatrolRunHistoryStore) GetRecent(n int) []PatrolRunRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n <= 0 || n > len(s.runs) {
		n = len(s.runs)
	}

	result := make([]PatrolRunRecord, n)
	copy(result, s.runs[:n])
	return result
}

// Count returns the number of runs in history
func (s *PatrolRunHistoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.runs)
}

// scheduleSaveLocked schedules a debounced save operation.
// MUST be called with s.mu held (either Lock or is deferred with unlock).
func (s *PatrolRunHistoryStore) scheduleSaveLocked() {
	if s.persistence == nil {
		return
	}

	// Cancel existing timer if pending
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}

	s.savePending = true
	s.saveTimer = time.AfterFunc(s.saveDebounce, func() {
		s.mu.Lock()
		if !s.savePending {
			s.mu.Unlock()
			return
		}
		s.savePending = false
		// Copy runs while locked
		runs := make([]PatrolRunRecord, len(s.runs))
		copy(runs, s.runs)
		persistence := s.persistence
		s.mu.Unlock()

		// Save outside lock
		if persistence != nil {
			if err := persistence.SavePatrolRunHistory(runs); err != nil {
				log.Error().Err(err).Msg("Failed to save patrol run history")
				s.mu.Lock()
				s.lastSaveError = err
				s.mu.Unlock()
				if onErr := s.onSaveError; onErr != nil {
					onErr(err)
				}
			} else {
				s.mu.Lock()
				s.lastSaveError = nil
				s.lastSaveTime = time.Now()
				s.mu.Unlock()
			}
		}
	})
}

// SetOnSaveError sets a callback that is called when persistence fails.
func (s *PatrolRunHistoryStore) SetOnSaveError(fn func(err error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSaveError = fn
}

// GetPersistenceStatus returns the last save error, last save time, and whether persistence is configured.
func (s *PatrolRunHistoryStore) GetPersistenceStatus() (lastError error, lastSaveTime time.Time, hasPersistence bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSaveError, s.lastSaveTime, s.persistence != nil
}

// FlushPersistence immediately saves any pending changes
func (s *PatrolRunHistoryStore) FlushPersistence() error {
	s.mu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.savePending = false
	runs := make([]PatrolRunRecord, len(s.runs))
	copy(runs, s.runs)
	persistence := s.persistence
	s.mu.Unlock()

	if persistence != nil {
		return persistence.SavePatrolRunHistory(runs)
	}
	return nil
}
