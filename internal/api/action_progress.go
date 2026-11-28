package api

import (
	"strings"
	"sync"
	"time"
)

const (
	maxActionProgressEntriesPerAction = 64
	maxActionProgressTrackedActions   = 256
	maxActionProgressMessageChars     = 512
	actionProgressRetentionTTL        = 30 * time.Minute
)

// ActionProgressEntry records one bounded, non-secret milestone emitted while
// a governed action executes on Pulse or on a connected agent.
type ActionProgressEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Phase     string    `json:"phase,omitempty"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
}

type actionProgressBucket struct {
	entries   []ActionProgressEntry
	updatedAt time.Time
}

// actionProgressStore keeps bounded, tenant-scoped execution logs in memory so
// review dialogs can display live step feedback during long-running actions
// (such as container image pulls) and retain the step log on the outcome view.
type actionProgressStore struct {
	mu      sync.RWMutex
	buckets map[string]*actionProgressBucket
	now     func() time.Time
}

func newActionProgressStore() *actionProgressStore {
	return &actionProgressStore{
		buckets: make(map[string]*actionProgressBucket),
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func actionProgressKey(orgID, actionID string) string {
	org := strings.TrimSpace(orgID)
	if org == "" {
		org = "default"
	}
	return org + "\x00" + strings.TrimSpace(actionID)
}

func sanitizeProgressMessage(value string, maxLen int) string {
	var b strings.Builder
	b.Grow(len(value))
	prevSpace := false
	for _, r := range value {
		if r == '\n' || r == '\r' || r == '\t' {
			r = ' '
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		if r == ' ' {
			if prevSpace || b.Len() == 0 {
				continue
			}
			prevSpace = true
		} else {
			prevSpace = false
		}
		if maxLen > 0 && b.Len() >= maxLen {
			break
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func (s *actionProgressStore) Record(orgID, actionID, source, phase, message string, at time.Time) {
	if s == nil {
		return
	}
	actionID = strings.TrimSpace(actionID)
	msg := sanitizeProgressMessage(message, maxActionProgressMessageChars)
	if actionID == "" || msg == "" {
		return
	}
	source = strings.TrimSpace(source)
	if source != "server" && source != "agent" {
		source = "server"
	}
	phase = sanitizeProgressMessage(phase, 64)
	now := s.now()
	if at.IsZero() {
		at = now
	} else {
		at = at.UTC()
	}

	key := actionProgressKey(orgID, actionID)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)

	bucket := s.buckets[key]
	if bucket == nil {
		if len(s.buckets) >= maxActionProgressTrackedActions {
			s.evictOldestLocked()
		}
		bucket = &actionProgressBucket{}
		s.buckets[key] = bucket
	}
	bucket.updatedAt = now

	// Avoid consecutive duplicate lines when throttled progress repeats a status.
	if n := len(bucket.entries); n > 0 {
		last := bucket.entries[n-1]
		if last.Message == msg && last.Source == source {
			return
		}
	}

	entry := ActionProgressEntry{
		Timestamp: at,
		Phase:     phase,
		Message:   msg,
		Source:    source,
	}
	if len(bucket.entries) >= maxActionProgressEntriesPerAction {
		// Preserve the first entry and the most recent tail.
		copy(bucket.entries[1:], bucket.entries[2:])
		bucket.entries[len(bucket.entries)-1] = entry
		return
	}
	bucket.entries = append(bucket.entries, entry)
}

func (s *actionProgressStore) List(orgID, actionID string) []ActionProgressEntry {
	if s == nil || strings.TrimSpace(actionID) == "" {
		return nil
	}
	key := actionProgressKey(orgID, actionID)
	now := s.now()
	s.mu.RLock()
	bucket := s.buckets[key]
	if bucket == nil || now.Sub(bucket.updatedAt) > actionProgressRetentionTTL || len(bucket.entries) == 0 {
		s.mu.RUnlock()
		return nil
	}
	out := make([]ActionProgressEntry, len(bucket.entries))
	copy(out, bucket.entries)
	s.mu.RUnlock()
	return out
}

func (s *actionProgressStore) pruneLocked(now time.Time) {
	for k, bucket := range s.buckets {
		if now.Sub(bucket.updatedAt) > actionProgressRetentionTTL {
			delete(s.buckets, k)
		}
	}
}

func (s *actionProgressStore) evictOldestLocked() {
	oldestKey := ""
	var oldestTime time.Time
	for k, bucket := range s.buckets {
		if oldestKey == "" || bucket.updatedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = bucket.updatedAt
		}
	}
	if oldestKey != "" {
		delete(s.buckets, oldestKey)
	}
}
